// Package registry is the single source of truth for the tool suite. Each tool
// is data — name, description, params and a builder — so both adapters (the MCP
// stdio server and `marq run`) iterate the same []Tool. Most tools are
// exec tools (Build -> Invocation -> runner); a few (files, findings, jobs) are
// in-process Handler tools.
package registry

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/rhaist/marq/internal/audit"
	"github.com/rhaist/marq/internal/config"
	"github.com/rhaist/marq/internal/runner"
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
	Background bool
	Refuse     string // non-empty: don't run; Call returns this (audited) instead
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
			case string: // from the `--key value` form
				if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
					out[p.Name] = i
				} else {
					out[p.Name] = p.Default
				}
			default:
				out[p.Name] = p.Default
			}
		case BoolParam:
			switch b := v.(type) {
			case bool:
				out[p.Name] = b
			case string: // from the `--key value` / bare-flag form
				out[p.Name] = b == "true" || b == "1" || b == "yes" || b == "on"
			default:
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
		fmt.Fprintf(&b, "parameters: none\ncall: marq run %s", t.Name)
		return b.String()
	}
	b.WriteString("parameters:\n")
	// Example uses the `--key value` form: small models drive it far more reliably
	// than JSON-in-shell (no escaping), and angle brackets read as "replace me".
	var call strings.Builder
	call.WriteString("marq run " + t.Name)
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
			call.WriteString(fmt.Sprintf(" --%s <%s>", p.Name, p.Name))
		}
	}
	fmt.Fprintf(&b, "call: %s", call.String())
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
	if msg := inv.Refuse; msg != "" {
		target := cmp.Or(inv.Target, "(unspecified)")
		id, _ := audit.LogStart(t.Name, target, inv.Argv)
		code := 3
		audit.LogEnd(id, t.Name, &code, 0, false, "refused: "+msg)
		return warn + "refused: " + msg
	}
	if inv.Background {
		jobDir, errMsg := runner.RunBackground(t.Name, inv.Argv, inv.Target)
		if errMsg != "" {
			return warn + "error: " + errMsg
		}
		return warn + backgroundMsg(t.Name, inv.Target, jobDir)
	}
	res := runner.Run(t.Name, inv.Argv, runner.Opts{
		Target: inv.Target,
		Stdin:  inv.Stdin,
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

// serverInfoTool reports the usage notice (operator, purpose, audit log). Lives
// in the registry so both front-ends expose it; the model is told to call it first.
func serverInfoTool() Tool {
	return Tool{
		Name: "server_info",
		Desc: "Return the operator/purpose metadata and what marq covers (passive OSINT from public " +
			"third-party sources, plus the cyber skills library). Call this first; if no purpose is " +
			"recorded, record it with set_engagement.",
		Handler: func(a Args) string { return config.C.Banner() },
	}
}

// setEngagementTool records the investigation label/purpose for the session's
// audit trail. Operator stays the host/env-set audit anchor and is not settable here.
func setEngagementTool() Tool {
	return Tool{
		Name: "set_engagement",
		Desc: "Record the investigation label and its purpose, e.g. 'acme-footprint-2026-11 — " +
			"pre-acquisition exposure review'. Persists under the working dir so every subsequent tool " +
			"call — across processes — is audit-logged under it. People OSINT is personal-data " +
			"processing: name the purpose. Operator identity is fixed by the host (MARQ_OPERATOR).",
		Params: []Param{
			{Name: "engagement", Type: StringParam, Desc: "investigation label and purpose", Required: true},
		},
		Handler: func(a Args) string {
			id, _ := audit.LogStart("set_engagement", a.S("engagement"), []string{"engagement=" + a.S("engagement")})
			err := config.SetEngagement(a.S("engagement"))
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

// All returns every registered tool.
func All() []Tool {
	tools := slices.Concat(
		[]Tool{serverInfoTool(), setEngagementTool()},
		recon(), intel(), osint(), people(), fileTools(), reportTools(), knowledgeTools(),
	)
	if config.C.SkillsOnly {
		// Knowledge-only mode: drop every exec tool (those with a Build) so the
		// server needs no Kali binaries, and the job tools, since only exec
		// tools start jobs.
		tools = slices.DeleteFunc(tools, func(t Tool) bool {
			return t.Build != nil || t.Name == "list_jobs" || t.Name == "job_status"
		})
	}
	return tools
}
