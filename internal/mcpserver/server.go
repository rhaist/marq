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
	s := mcp.NewServer(&mcp.Implementation{Name: "marq", Version: Version}, nil)

	for _, t := range registry.All() {
		addTool(s, t)
	}

	addResources(s)
	return s
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
