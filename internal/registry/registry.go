// Package registry is the single source of truth for the tool suite. Each tool
// is data — name, description, params and a builder — so both adapters (the MCP
// stdio server and the TUI agent loop) iterate the same []Tool. Most tools are
// exec tools (Build -> Invocation -> runner); a few (files, findings, jobs) are
// in-process Handler tools.
package registry

import (
	"fmt"
	"slices"

	"marq/internal/config"
	"marq/internal/runner"
)

// ParamType is the JSON-schema scalar type of a tool parameter.
type ParamType string

const (
	StringParam ParamType = "string"
	IntParam    ParamType = "integer"
	BoolParam   ParamType = "boolean"
)

// Param describes one tool argument (drives schema generation and defaulting).
type Param struct {
	Name     string
	Type     ParamType
	Desc     string
	Required bool
	Default  any // applied when the caller omits the argument
}

// Invocation is what an exec tool's Build produces.
type Invocation struct {
	Argv       []string
	Target     string
	Stdin      string
	Timeout    int // seconds; 0 = default
	Background bool
}

// Tool is one capability. Exactly one of Build (exec) or Handler (in-process)
// is set.
type Tool struct {
	Name    string
	Desc    string
	Params  []Param
	Build   func(a Args) Invocation
	Handler func(a Args) string
}

// Args holds resolved argument values, already defaulted and coerced to Go types.
type Args map[string]any

// S returns a string argument.
func (a Args) S(name string) string {
	if v, ok := a[name].(string); ok {
		return v
	}
	return ""
}

// I returns an integer argument.
func (a Args) I(name string) int {
	switch v := a[name].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

// B returns a boolean argument.
func (a Args) B(name string) bool {
	v, _ := a[name].(bool)
	return v
}

// Resolve fills in defaults and coerces raw JSON argument values to the types
// declared by params, so Build/Handler can read them without re-checking.
func Resolve(params []Param, raw map[string]any) Args {
	out := Args{}
	for _, p := range params {
		v, ok := raw[p.Name]
		if !ok || v == nil {
			out[p.Name] = p.Default
			continue
		}
		switch p.Type {
		case IntParam:
			switch n := v.(type) {
			case float64:
				out[p.Name] = int(n)
			case int:
				out[p.Name] = n
			default:
				out[p.Name] = p.Default
			}
		case BoolParam:
			if b, ok := v.(bool); ok {
				out[p.Name] = b
			} else {
				out[p.Name] = p.Default
			}
		default: // string
			if s, ok := v.(string); ok {
				out[p.Name] = s
			} else {
				out[p.Name] = fmt.Sprint(v)
			}
		}
	}
	return out
}

// InputSchema builds the JSON-schema object describing this tool's parameters.
func (t Tool) InputSchema() map[string]any {
	props := map[string]any{}
	var required []string
	for _, p := range t.Params {
		prop := map[string]any{
			"type":        string(p.Type),
			"description": p.Desc,
		}
		if p.Default != nil {
			prop["default"] = p.Default
		}
		props[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// Call executes the tool with raw JSON arguments and returns the text envelope
// the model reads. This is the single execution entry shared by both adapters.
func (t Tool) Call(raw map[string]any) string {
	args := Resolve(t.Params, raw)
	if t.Handler != nil {
		return t.Handler(args)
	}
	inv := t.Build(args)
	if inv.Background {
		jobDir, errMsg := runner.RunBackground(t.Name, inv.Argv, inv.Target)
		if errMsg != "" {
			return "error: " + errMsg
		}
		return backgroundMsg(t.Name, inv.Target, jobDir)
	}
	res := runner.Run(t.Name, inv.Argv, runner.Opts{
		Target:  inv.Target,
		Stdin:   inv.Stdin,
		Timeout: inv.Timeout,
	})
	return res.Render()
}

func backgroundMsg(tool, target, jobDir string) string {
	return fmt.Sprintf(
		"%s launched in the background for %s.\n"+
			"  results     : %s/stdout.log\n"+
			"  diagnostics : %s/stderr.log\n"+
			"  done-signal : %s/status  (appears with 'exit=<code>' when finished)\n"+
			"Poll with job_status('%s'), read_file('%s/stdout.log') or list_dir('%s'); give it a few minutes.",
		tool, target, jobDir, jobDir, jobDir, jobDir, jobDir, jobDir,
	)
}

// serverInfoTool reports the authorization banner + configured scope. Lives in
// the registry so both front-ends expose it (the model is told to call it first
// to confirm scope); previously it was an MCP-only tool, so the agent/TUI loop
// errored on it.
func serverInfoTool() Tool {
	return Tool{
		Name: "server_info",
		Desc: "Return the scope/operator metadata and the domains marq covers (offensive, " +
			"malware, threat-intel, governance). Call this first; before any active testing " +
			"(scanning/exploitation) confirm the targets are in the authorized scope it reports.",
		Handler: func(a Args) string {
			raw := "disabled"
			if config.C.AllowRawShell {
				raw = "enabled"
			}
			return config.C.Banner() + "\n  raw shell  : " + raw
		},
	}
}

// All returns every registered tool. shell is included only when raw shell is
// enabled (mirrors the Python conditional registration).
func All() []Tool {
	tools := slices.Concat(
		[]Tool{serverInfoTool()},
		recon(), osint(), people(), web(),
		exploit(), creds(), internal(), malware(), fileTools(), reportTools(), knowledgeTools(),
	)
	if config.C.AllowRawShell {
		tools = append(tools, shell()...)
	}
	return tools
}
