// Package mcpserver adapts the shared tool registry onto an MCP stdio server
// using the official modelcontextprotocol/go-sdk. It is the drop-in replacement
// for the original Python server: an external MCP client (Claude Desktop, etc.)
// launches it over stdio and brings its own model.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"marq/internal/config"
	"marq/internal/registry"
	"marq/internal/skills"
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
		"`run_shell` only for actions with no dedicated tool.\n" +
		"- When you have what you need, stop calling tools and give a concise final " +
		"answer — lead with the key facts/numbers, then the supporting detail.\n\n" +
		"Advisory and knowledge work is unrestricted. Active testing (scanning, " +
		"exploitation, credential attacks) is authorized-only — confirm targets are " +
		"in the scope server_info reports before touching anything; every call is " +
		"audit-logged.\n\n" +
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
		out := tool.Call(args)
		return textResult(out), nil
	}
	s.AddTool(mcpTool, handler)
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
			Description: "Engagement workflow: recon -> enumerate -> test -> exploit -> report. Read before driving the tools.",
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
			Description: "Index of technique/vuln-class playbooks. Load a body with the load_skill tool or read marq://skills/<name>.",
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
