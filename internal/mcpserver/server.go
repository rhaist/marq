// Package mcpserver adapts the shared tool registry onto an MCP stdio server
// using the official modelcontextprotocol/go-sdk. It is the drop-in replacement
// for the original Python server: an external MCP client (Claude Desktop, etc.)
// launches it over stdio and brings its own model.
package mcpserver

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rhaist/marq/internal/audit"
	"github.com/rhaist/marq/internal/config"
	"github.com/rhaist/marq/internal/registry"
	"github.com/rhaist/marq/internal/scope"
	"github.com/rhaist/marq/internal/skills"
)

// Version is the server's reported implementation version.
const Version = "2.0.0"

// New builds the MCP server with every registered tool, the authorization +
// methodology resources, and the server_info tool.
func New() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "marq", Version: Version},
		&mcp.ServerOptions{Instructions: instructions()})

	for _, t := range registry.All() {
		addTool(s, t)
	}

	addResources(s)
	return s
}

// instructions is the server-level briefing surfaced into the model's context by
// MCP clients (Claude Code, Codex, …). Resources aren't auto-loaded, so this is
// how a client-driven model learns to drive marq across all domains: call
// server_info first, then load_skill the right playbook. Mirrors pi/SKILL.md's
// role for the Pi flow.
func instructions() string {
	return "marq is a universal cyber assistant: ~80 security tools plus a skills " +
		"library of expert playbooks across 14 domains (offensive, malware, " +
		"threat-intel, sec-ops, architecture, GRC, standards & regulation, CISO, " +
		"resilience, human factors, DevSecOps/privacy).\n\n" +
		"How to work:\n" +
		"1. Call the `server_info` tool first — it reports scope and the domains.\n" +
		"2. Load the playbook for the task with the `load_skill` tool — including " +
		"advisory, standards and regulation questions: load the skill first, don't " +
		"answer those from memory. (Call it with no arguments to list every skill, " +
		"then load the relevant one by name.) The skills carry current-standards " +
		"detail and the right tool order.\n" +
		"3. Record results with `report_finding`; render the deliverable with " +
		"`render_report`. Long scans run in the background — poll with `list_jobs` / " +
		"`job_status`.\n\n" +
		"Work efficiently (matters most for smaller models):\n" +
		"- Prefer the dedicated tool over `run_shell` — check `marq tools` and use " +
		"the wrapper if one exists (it's scoped, structured and audited). Use " +
		"`exec` (allowlisted binaries, no shell) for options or tools a wrapper doesn't cover, " +
		"and `run_shell` only when it's enabled and nothing else fits.\n" +
		"- When you have what you need, stop calling tools and give a concise final " +
		"answer — lead with the key facts/numbers, then the supporting detail.\n" +
		"- Report only what a tool actually returned this session — never open ports, " +
		"hosts, subdomains or findings from prior knowledge (even for well-known " +
		"targets). No tool output means no finding; run the tool first.\n\n" +
		"Advisory and knowledge work is unrestricted. Active testing (scanning, " +
		"exploitation, credential attacks) is authorized-only: run active tools only " +
		"against targets inside the recorded scope, and refuse — don't call the tool — " +
		"for anything outside it, even when a request tacks an extra target onto an " +
		"in-scope one. marq also enforces this: out-of-scope active calls are refused, and " +
		"widening scope asks the operator when the client supports it — otherwise ask them yourself first. Every call is audit-logged. (Scope handling is spelled out in " +
		"the methodology below.)\n\n" +
		"--- methodology ---\n\n" + registry.Methodology
}

// addTool bridges one registry.Tool into the MCP server with a raw-args handler.
func addTool(s *mcp.Server, tool registry.Tool) {
	mcpTool := &mcp.Tool{
		Name:        tool.Name,
		Description: tool.Desc,
		InputSchema: tool.InputSchema(),
	}
	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}
		}
		if res := confirmScope(req, tool, args); res != nil {
			return res, nil
		}
		out := tool.Call(args)
		return textResult(out), nil
	}
	s.AddTool(mcpTool, handler)
}

// confirmScope makes widening scope a human decision on clients that support
// elicitation: an out-of-scope active call, or a set_engagement that adds
// targets, asks the operator first. It uses the multi-round-trip pattern
// (SEP-2322): the first pass returns the question as InputRequests, the client
// retries with the answer, and the SDK bridges legacy clients via Elicit.
// Accepting widens scope (audit-logged) and the call proceeds; declining returns
// a refusal. Without elicitation support it returns nil, and registry.Call's own
// gate refuses out-of-scope calls as on the `marq run` path.
func confirmScope(req *mcp.CallToolRequest, tool registry.Tool, args map[string]any) *mcp.CallToolResult {
	if caps := req.ClientCapabilities(); caps == nil || caps.Elicitation == nil {
		return nil
	}
	var added []string
	var question string
	if tool.Name == "set_engagement" {
		_, old := config.Attribution()
		newNote, _ := args["scope"].(string)
		if added = scope.Added(old, newNote); len(added) == 0 {
			return nil
		}
		question = fmt.Sprintf("The agent wants to set the engagement scope to %q, which authorizes %s. Confirm?",
			newNote, strings.Join(added, ", "))
	} else {
		if added = tool.ScopeGap(args); len(added) == 0 {
			return nil
		}
		question = fmt.Sprintf("%s wants to touch %s, which is outside the engagement scope. Authorize and add to scope?",
			tool.Name, strings.Join(added, ", "))
	}
	// The answer only counts on the retry of the question we asked: the state is
	// an HMAC over the tool and the exact targets, so an answer can't be
	// pre-supplied or carried over to a retry with different targets.
	state := scopeState(tool.Name, added)
	answer, answered := req.Params.InputResponses[scopeInput].(*mcp.ElicitResult)
	if !answered || !hmac.Equal([]byte(req.Params.RequestState), []byte(state)) {
		return &mcp.CallToolResult{RequestState: state, InputRequests: mcp.InputRequestMap{scopeInput: &mcp.ElicitParams{
			Message: "marq: " + question,
			RequestedSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"approve": map[string]any{"type": "boolean", "title": "Authorize these targets for this engagement"},
				},
				"required": []string{"approve"},
			},
		}}}
	}
	approved := answer.Action == "accept" && answer.Content["approve"] == true
	id, _ := audit.LogStart("scope.confirm", strings.Join(added, ", "),
		[]string{"tool=" + tool.Name, "action=" + answer.Action, fmt.Sprintf("approved=%t", approved)})
	code := 0
	if !approved {
		code = 3
	}
	audit.LogEnd(id, "scope.confirm", &code, 0, false, "")
	if !approved {
		return textResult(fmt.Sprintf("refused: the operator did not authorize %s. Don't retry or work around "+
			"it; continue within the current scope or ask the operator.", strings.Join(added, ", ")))
	}
	if tool.Name != "set_engagement" {
		if err := config.WidenScope(added...); err != nil {
			return textResult("refused: could not persist the widened scope: " + err.Error())
		}
	}
	return nil
}

// scopeInput is the InputRequests key for the scope confirmation.
const scopeInput = "scope_confirmation"

// stateKey signs scope-confirmation request state; per process, so a state is
// only valid against the server that issued it.
var stateKey = func() []byte {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		panic(err)
	}
	return k
}()

func scopeState(tool string, targets []string) string {
	m := hmac.New(sha256.New, stateKey)
	m.Write([]byte(tool + "\x00" + strings.Join(targets, ",")))
	return hex.EncodeToString(m.Sum(nil))
}

func addResources(s *mcp.Server) {
	s.AddResource(
		&mcp.Resource{
			URI:         "marq://authorization",
			Name:        "authorization",
			Description: "Rules-of-engagement / authorization notice for this session. Surface to the operator before running tools.",
			MIMEType:    "text/plain",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return resourceText("marq://authorization", config.C.Banner()), nil
		},
	)
	s.AddResource(
		&mcp.Resource{
			URI:         "marq://methodology",
			Name:        "methodology",
			Description: "Cross-domain methodology + the offensive engagement phases (recon -> enumerate -> test -> exploit -> report). Read before driving the tools.",
			MIMEType:    "text/markdown",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return resourceText("marq://methodology", registry.Methodology), nil
		},
	)
	addSkillResources(s)
}

// addSkillResources exposes the skills library: an index plus one resource per
// skill (marq://skills and marq://skills/<name>).
func addSkillResources(s *mcp.Server) {
	s.AddResource(
		&mcp.Resource{
			URI:         "marq://skills",
			Name:        "skills",
			Description: "Index of expert playbooks (offensive, malware, threat-intel, governance/standards/CISO). Load a body with the load_skill tool or read marq://skills/<name>.",
			MIMEType:    "text/markdown",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return resourceText("marq://skills", "# Skills\n\n"+skills.IndexText()), nil
		},
	)
	for _, meta := range skills.List() {
		name := meta.Name
		uri := "marq://skills/" + name
		s.AddResource(
			&mcp.Resource{URI: uri, Name: name, Description: meta.Description, MIMEType: "text/markdown"},
			func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				body, err := skills.Load(name)
				if err != nil {
					return nil, err
				}
				return resourceText(uri, body), nil
			},
		)
	}
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func errorResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: true}
}

func resourceText(uri, text string) *mcp.ReadResourceResult {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/plain", Text: text}},
	}
}
