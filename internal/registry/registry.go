// Package registry is the single source of truth for the tool suite. Each tool
// is data — name, description, params and a builder — so both adapters (the MCP
// stdio server and the TUI agent loop) iterate the same []Tool. Most tools are
// exec tools (Build -> Invocation -> runner); a few (files, findings, jobs) are
// in-process Handler tools.
package registry

import (
	"fmt"
	"slices"
	"strings"

	"marq/internal/audit"
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
	// Active marks an active-testing tool (scanning, exploitation, credential
	// attacks, on-target probing) that must run only against authorized in-scope
	// targets. Surfaced at the point of action (catalog + Usage) so the scope
	// rule is reinforced where the model decides — not just at server_info.
	// Passive OSINT, offline cracking, and static analysis stay false.
	Active bool
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

// HasRequired reports whether the tool has at least one required parameter.
func (t Tool) HasRequired() bool {
	for _, p := range t.Params {
		if p.Required {
			return true
		}
	}
	return false
}

// missingRequired returns the names of required params absent (or blank) in the
// raw arguments. A required string left empty counts as missing, so a fumbled
// call surfaces a clear error instead of silently running with an empty value.
func (t Tool) missingRequired(raw map[string]any) []string {
	var missing []string
	for _, p := range t.Params {
		if !p.Required {
			continue
		}
		v, ok := raw[p.Name]
		if !ok || v == nil {
			missing = append(missing, p.Name)
			continue
		}
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
			missing = append(missing, p.Name)
		}
	}
	return missing
}

// Usage renders a compact, model-readable parameter schema — the `marq run`
// path's equivalent of the typed InputSchema an MCP client receives. Surfacing
// it is what lets a local model construct correct JSON args instead of guessing.
func (t Tool) Usage() string {
	var b strings.Builder
	desc, _, _ := strings.Cut(t.Desc, "\n")
	fmt.Fprintf(&b, "%s — %s\n", t.Name, desc)
	if len(t.Params) == 0 {
		fmt.Fprintf(&b, "parameters: none\ncall: marq run %s '{}'", t.Name)
		return b.String()
	}
	b.WriteString("parameters:\n")
	var reqExample []string
	for _, p := range t.Params {
		req := "optional"
		if p.Required {
			req = "required"
		}
		def := ""
		if p.Default != nil && p.Default != "" {
			def = fmt.Sprintf(", default %v", p.Default)
		}
		fmt.Fprintf(&b, "  %-14s %s, %s%s — %s\n", p.Name, p.Type, req, def, p.Desc)
		if p.Required {
			reqExample = append(reqExample, fmt.Sprintf("%q:%q", p.Name, "..."))
		}
	}
	ex := "{}"
	if len(reqExample) > 0 {
		ex = "{" + strings.Join(reqExample, ",") + "}"
	}
	fmt.Fprintf(&b, "call: marq run %s '%s'", t.Name, ex)
	if t.Active {
		b.WriteString("\n[active testing] run only against authorized, in-scope targets — confirm scope with server_info first; every call is audit-logged.")
	}
	return b.String()
}

// unknownArgs returns raw keys that aren't declared parameters — a misnamed
// param a model passed (e.g. `recordtype` for `record_type`) would otherwise be
// silently dropped and the tool run with defaults.
func (t Tool) unknownArgs(raw map[string]any) []string {
	valid := make(map[string]bool, len(t.Params))
	for _, p := range t.Params {
		valid[p.Name] = true
	}
	var unknown []string
	for k := range raw {
		if !valid[k] {
			unknown = append(unknown, k)
		}
	}
	slices.Sort(unknown)
	return unknown
}

// Call executes the tool with raw JSON arguments and returns the text envelope
// the model reads. This is the single execution entry shared by both adapters.
func (t Tool) Call(raw map[string]any) string {
	if missing := t.missingRequired(raw); len(missing) > 0 {
		return fmt.Sprintf("error: missing required parameter(s): %s\n\n%s",
			strings.Join(missing, ", "), t.Usage())
	}
	// Surface misnamed args instead of silently dropping them; still run with
	// what was understood so a typo'd optional doesn't waste the whole call.
	var warn string
	if unknown := t.unknownArgs(raw); len(unknown) > 0 {
		warn = fmt.Sprintf("note: ignored unknown argument(s): %s — not a parameter of %s. "+
			"Run `marq tools %s` for valid parameters.\n\n", strings.Join(unknown, ", "), t.Name, t.Name)
	}
	args := Resolve(t.Params, raw)
	if t.Handler != nil {
		return warn + t.Handler(args)
	}
	inv := t.Build(args)
	if inv.Background {
		jobDir, errMsg := runner.RunBackground(t.Name, inv.Argv, inv.Target)
		if errMsg != "" {
			return warn + "error: " + errMsg
		}
		return warn + backgroundMsg(t.Name, inv.Target, jobDir)
	}
	res := runner.Run(t.Name, inv.Argv, runner.Opts{
		Target:  inv.Target,
		Stdin:   inv.Stdin,
		Timeout: inv.Timeout,
	})
	return warn + res.Render()
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
			"(scanning/exploitation) confirm the targets are in the authorized scope it reports. " +
			"If no scope is set, record it with set_engagement.",
		Handler: func(a Args) string {
			raw := "disabled"
			if config.C.AllowRawShell {
				raw = "enabled"
			}
			return config.C.Banner() + "\n  raw shell  : " + raw
		},
	}
}

// setEngagementTool records the engagement label and authorized scope for the
// session. Operator stays the host/env-set audit anchor and is not settable here.
func setEngagementTool() Tool {
	return Tool{
		Name: "set_engagement",
		Desc: "Record the engagement label and the authorized testing scope for this session, from the " +
			"operator's written authorization. Persists under the working dir so every subsequent tool " +
			"call — across processes — is audit-logged under it; set it before any active testing. " +
			"Operator identity is fixed by the host (MARQ_OPERATOR) and is not settable here. `scope` " +
			"should name the exact in-scope targets, e.g. '*.acme.com, 203.0.113.0/24 — per SOW'.",
		Params: []Param{
			{Name: "scope", Type: StringParam, Desc: "authorized in-scope targets per the SOW", Required: true},
			{Name: "engagement", Type: StringParam, Desc: "engagement label, e.g. acme-webapp-2026-06", Default: ""},
		},
		Handler: func(a Args) string {
			id := audit.LogStart("set_engagement", a.S("scope"),
				[]string{"engagement=" + a.S("engagement"), "scope=" + a.S("scope")})
			err := config.SetEngagement(a.S("engagement"), a.S("scope"))
			ec, msg := 0, ""
			if err != nil {
				ec, msg = 1, err.Error()
			}
			audit.LogEnd(id, "set_engagement", &ec, 0, false, msg)
			if err != nil {
				return "failed to persist engagement context: " + msg
			}
			return "engagement context set.\n\n" + config.C.Banner()
		},
	}
}

// All returns every registered tool. shell is included only when raw shell is
// enabled (mirrors the Python conditional registration).
func All() []Tool {
	tools := slices.Concat(
		[]Tool{serverInfoTool(), setEngagementTool()},
		recon(), osint(), people(), web(),
		exploit(), creds(), internal(), malware(), fileTools(), reportTools(), knowledgeTools(),
	)
	if config.C.AllowRawShell {
		tools = append(tools, shell()...)
	}
	return tools
}
