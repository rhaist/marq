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

	"pentest-mcp/internal/config"
	"pentest-mcp/internal/registry"
)

// Version is the server's reported implementation version.
const Version = "2.0.0"

// New builds the MCP server with every registered tool, the authorization +
// methodology resources, and the server_info tool.
func New() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "pentest-mcp", Version: Version}, nil)

	for _, t := range registry.All() {
		addTool(s, t)
	}

	addServerInfo(s)
	addResources(s)
	return s
}

// addTool bridges one registry.Tool into the MCP server with a raw-args handler.
func addTool(s *mcp.Server, t registry.Tool) {
	tool := t // capture per iteration
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

func addServerInfo(s *mcp.Server) {
	tool := &mcp.Tool{
		Name: "server_info",
		Description: "Return the authorization banner, configured scope and operator metadata. " +
			"Call this first to confirm you are authorized to test the intended targets.",
		InputSchema: map[string]any{"type": "object"},
	}
	s.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw := "disabled"
		if config.C.AllowRawShell {
			raw = "enabled"
		}
		return textResult(config.C.Banner() + "\n  raw shell  : " + raw), nil
	})
}

func addResources(s *mcp.Server) {
	s.AddResource(
		&mcp.Resource{
			URI:         "pentest://authorization",
			Name:        "authorization",
			Description: "Rules-of-engagement / authorization notice for this session. Surface to the operator before running tools.",
			MIMEType:    "text/plain",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return resourceText("pentest://authorization", config.C.Banner()), nil
		},
	)
	s.AddResource(
		&mcp.Resource{
			URI:         "pentest://methodology",
			Name:        "methodology",
			Description: "Engagement workflow: recon -> enumerate -> test -> exploit -> report. Read before driving the tools.",
			MIMEType:    "text/markdown",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return resourceText("pentest://methodology", registry.Methodology), nil
		},
	)
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
