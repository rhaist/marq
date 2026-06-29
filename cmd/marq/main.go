// Command marq is the single binary for the marq toolkit. It exposes the
// shared tool registry two ways:
//
//	marq serve               run the MCP stdio server (external client brings the model)
//	marq run <tool> [json]   invoke one tool directly (the host shim / Pi skill uses this)
//	marq tools               list every tool with a one-line description
//
// With no arguments it defaults to `serve` (so `docker run -i` starts the server).
// Local-model agent UX lives outside the binary now — drive `marq run` from a
// terminal agent (Pi) or any MCP client against `marq serve`. See pi/SKILL.md.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"marq/internal/config"
	"marq/internal/mcpserver"
	"marq/internal/registry"
)

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
	case "tools":
		listTools()
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

// listTools prints every registered tool with the first line of its
// description, so a terminal agent (or the operator) can browse the catalog on
// demand instead of carrying 70+ tool specs in the system prompt.
func listTools() {
	for _, t := range registry.All() {
		desc, _, _ := strings.Cut(t.Desc, "\n")
		fmt.Printf("%-24s %s\n", t.Name, desc)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `marq — Kali pentest/OSINT toolkit (MCP server + direct tool runner)

usage:
  marq serve              run the MCP stdio server (default)
  marq run <tool> [json]  invoke one tool directly
  marq tools              list every tool with a one-line description

Interactive local-model use: drive `+"`marq run`"+` from a terminal agent such as
Pi (see pi/SKILL.md), or point any MCP client at `+"`marq serve`"+`.
`)
}
