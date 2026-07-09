// marq — a universal cyber assistant.
// Copyright (C) 2026 marq contributors
//
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option) any
// later version. It is distributed WITHOUT ANY WARRANTY. See the LICENSE file or
// <https://www.gnu.org/licenses/> for details.

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

	"github.com/rhaist/marq/internal/config"
	"github.com/rhaist/marq/internal/mcpserver"
	"github.com/rhaist/marq/internal/registry"
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
		listTools(os.Args[2:])
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
	var tool *registry.Tool
	for _, t := range registry.All() {
		if t.Name == name {
			tt := t
			tool = &tt
			break
		}
	}
	if tool == nil {
		fmt.Fprintf(os.Stderr, "unknown tool %q\n", name)
		os.Exit(1)
	}
	rest := argv[1:]
	// No args given: if the tool needs required params, print its schema instead
	// of running empty — this is how a local model discovers a tool's arguments.
	if len(rest) == 0 || (len(rest) == 1 && strings.TrimSpace(rest[0]) == "") {
		if tool.HasRequired() {
			fmt.Println(tool.Usage())
			return
		}
	}
	args, err := parseToolArgs(rest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(tool.Call(args))
}

// parseToolArgs accepts either a single JSON object ('{"k":"v"}') OR
// `--key value` / `--key=value` flags — small models reach for CLI flags by
// reflex and stumble on JSON-in-shell escaping, so marq run takes both. A bare
// `--json '<obj>'` merges the object; a valueless flag is boolean true. Values
// stay strings; Resolve coerces them to each param's declared type.
func parseToolArgs(rest []string) (map[string]any, error) {
	args := map[string]any{}
	if len(rest) >= 1 && strings.HasPrefix(strings.TrimSpace(rest[0]), "{") {
		if err := json.Unmarshal([]byte(strings.TrimSpace(rest[0])), &args); err != nil {
			return nil, fmt.Errorf("invalid json args: %w", err)
		}
		return args, nil
	}
	for i := 0; i < len(rest); i++ {
		if !strings.HasPrefix(rest[i], "--") {
			continue // tolerate stray tokens
		}
		key := strings.TrimPrefix(rest[i], "--")
		val := "true" // bare flag → true
		if eq := strings.IndexByte(key, '='); eq >= 0 {
			key, val = key[:eq], key[eq+1:]
		} else if i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "--") {
			val = rest[i+1]
			i++
		}
		if key == "json" { // `--json '<obj>'`
			if err := json.Unmarshal([]byte(val), &args); err != nil {
				return nil, fmt.Errorf("invalid json args: %w", err)
			}
			continue
		}
		args[key] = val
	}
	return args, nil
}

// listTools prints every registered tool with the first line of its
// description, so a terminal agent (or the operator) can browse the catalog on
// demand instead of carrying 70+ tool specs in the system prompt. With a tool
// name, it prints that tool's full parameter schema (Usage) for arg discovery.
func listTools(argv []string) {
	if len(argv) > 0 {
		name := argv[0]
		for _, t := range registry.All() {
			if t.Name == name {
				fmt.Println(t.Usage())
				return
			}
		}
		fmt.Fprintf(os.Stderr, "unknown tool %q\n", name)
		os.Exit(1)
	}
	for _, t := range registry.All() {
		desc, _, _ := strings.Cut(t.Desc, "\n")
		if t.Active {
			desc += "  [active: in-scope only]"
		}
		fmt.Printf("%-24s %s\n", t.Name, desc)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `marq — Kali pentest/OSINT toolkit (MCP server + direct tool runner)

usage:
  marq serve              run the MCP stdio server (default)
  marq run <tool> [json]  invoke one tool directly (omit json to see its parameters)
  marq tools [name]       list every tool, or show one tool's parameter schema

Interactive local-model use: drive `+"`marq run`"+` from a terminal agent such as
Pi (see pi/SKILL.md), or point any MCP client at `+"`marq serve`"+`.
`)
}
