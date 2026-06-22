// Command marq is the single binary for the marq toolkit. It exposes the
// shared tool registry two ways:
//
//	marq serve          run the MCP stdio server (external client brings the model)
//	marq tui            run the TUI agent host (local model runtime)  [phase 3/4]
//	marq run <tool> [json]   invoke one tool directly (smoke testing)
//
// With no arguments it defaults to `serve` (so `docker run -i` starts the server).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"marq/internal/agent"
	"marq/internal/config"
	"marq/internal/mcpserver"
	"marq/internal/registry"
	"marq/internal/tui"
)

func runTUI() {
	if err := tui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui error:", err)
		os.Exit(1)
	}
}

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "serve", "mcp":
		serve()
	case "run":
		runTool(os.Args[2:])
	case "agent":
		runAgent(os.Args[2:])
	case "tui":
		runTUI()
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func serve() {
	// Banner goes to stderr so it never corrupts the stdio JSON-RPC stream.
	fmt.Fprintln(os.Stderr, config.C.Banner())
	srv := mcpserver.New()
	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}

// runTool invokes a single tool by name with JSON arguments, for smoke testing
// outside an MCP client. Example: marq run nmap '{"target":"127.0.0.1"}'
func runTool(argv []string) {
	if len(argv) < 1 {
		fmt.Fprintln(os.Stderr, "usage: marq run <tool> ['<json-args>']")
		os.Exit(1)
	}
	name := argv[0]
	args := map[string]any{}
	if len(argv) > 1 && argv[1] != "" {
		if err := json.Unmarshal([]byte(argv[1]), &args); err != nil {
			fmt.Fprintln(os.Stderr, "invalid json args:", err)
			os.Exit(1)
		}
	}
	for _, t := range registry.All() {
		if t.Name == name {
			fmt.Println(t.Call(args))
			return
		}
	}
	fmt.Fprintf(os.Stderr, "unknown tool %q\n", name)
	os.Exit(1)
}

// runAgent runs the headless agent loop against the configured model runtime,
// printing each step. Example: marq agent "footprint example.com"
func runAgent(argv []string) {
	task := strings.TrimSpace(strings.Join(argv, " "))
	if task == "" {
		fmt.Fprintln(os.Stderr, `usage: marq agent "<task>"`)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "model: %s @ %s\n\n", config.C.ModelName, config.C.ModelBaseURL)
	loop := agent.NewLoop()
	loop.User(task)
	loop.Run(context.Background(), func(e agent.Event) {
		switch e.Kind {
		case agent.EventAssistant:
			fmt.Printf("\n🤖 %s\n", e.Text)
		case agent.EventToolCall:
			fmt.Printf("\n→ %s %s\n", e.Tool, e.Args)
		case agent.EventToolResult:
			fmt.Printf("%s\n", truncForLog(e.Text, 1500))
		case agent.EventError:
			fmt.Fprintf(os.Stderr, "\n✖ error: %s\n", e.Text)
		case agent.EventDone:
			fmt.Println("\n✓ done")
		}
	})
}

func truncForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…[truncated for log]…"
}

func usage() {
	fmt.Fprint(os.Stderr, `marq — Kali marq/OSINT toolkit (MCP server + TUI agent host)

usage:
  marq serve              run the MCP stdio server (default)
  marq tui                run the TUI agent host (local model runtime)
  marq agent "<task>"     run the agent host headless (prints each step)
  marq run <tool> [json]  invoke one tool directly (smoke testing)

The agent/tui modes need a local OpenAI-compatible model runtime (LM Studio by
default, or Ollama / llama.cpp). Configure it with MARQ_MODEL_URL / MARQ_MODEL.
`)
}
