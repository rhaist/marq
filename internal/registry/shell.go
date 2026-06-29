package registry

// shell returns the optional raw-shell escape hatch. Registered only when
// config.C.AllowRawShell is true (see All). It lets the model run any of the
// image's hundreds of tools that lack a dedicated wrapper. Still audit-logged.
func shell() []Tool {
	return []Tool{
		{
			Name:   "run_shell",
			Active: true,
			Desc: "Run an arbitrary shell command inside the marq container. Use for tools without a " +
				"dedicated wrapper. `target` should name the host/URL under test for the audit record. " +
				"Audit-logged; authorized use only.",
			Params: []Param{
				{Name: "command", Type: StringParam, Desc: "shell command", Required: true},
				{Name: "target", Type: StringParam, Desc: "host/URL under test (for the audit record)", Default: "(raw shell)"},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"/bin/bash", "-c", a.S("command")}, Target: a.S("target")}
			},
		},
	}
}
