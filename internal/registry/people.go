package registry

import (
	"strconv"

	"github.com/rhaist/marq/internal/shellword"
)

// people returns the people-footprint OSINT tools.
func people() []Tool {
	return []Tool{
		{
			Name: "sherlock",
			Desc: "Hunt a USERNAME across ~400 social networks and sites (sherlock). Fast, keyless. " +
				"Returns the sites where the username exists. Use `maigret_username` for a broader sweep.",
			Params: []Param{
				{Name: "username", Type: StringParam, Desc: "username", Required: true},
				{Name: "timeout", Type: IntParam, Desc: "per-site timeout seconds", Default: 30},
			},
			Build: func(a Args) Invocation {
				argv := []string{"sherlock", "--print-found", "--no-color", "--timeout", strconv.Itoa(a.I("timeout")), a.S("username")}
				return Invocation{Argv: argv, Target: a.S("username")}
			},
		},
		{
			Name: "maigret_username",
			Desc: "Deep USERNAME enumeration across ~2500 sites with profile-metadata extraction " +
				"(maigret). Broader and richer than sherlock but slower; cap breadth with `top_sites`. Keyless.",
			Params: []Param{
				{Name: "username", Type: StringParam, Desc: "username", Required: true},
				{Name: "timeout", Type: IntParam, Desc: "per-site timeout seconds", Default: 30},
				{Name: "top_sites", Type: IntParam, Desc: "limit number of sites", Default: 500},
			},
			Build: func(a Args) Invocation {
				argv := []string{
					"maigret", a.S("username"),
					"--no-color", "--no-progressbar",
					"--timeout", strconv.Itoa(a.I("timeout")),
					"--top-sites", strconv.Itoa(a.I("top_sites")),
				}
				return Invocation{Argv: argv, Target: a.S("username")}
			},
		},
		{
			Name: "email_accounts",
			Desc: "Discover which sites have an account registered to an EMAIL address (user-scanner), " +
				"via silent registration/login probes. Modules that could send the owner a " +
				"password-reset email are skipped, so it does not alert the account owner. Prints " +
				"sites where the email is registered; a few sites may be unreachable (rate limits).",
			Params: []Param{
				{Name: "email", Type: StringParam, Desc: "email address", Required: true},
			},
			Build: func(a Args) Invocation {
				// Drop skipped-module lines and the upstream proxy-affiliate ad.
				cmd := "set -o pipefail; user-scanner -e " + shellword.Quote(a.S("email")) +
					` </dev/null | grep -v -e '\[~\]' -e 'partner=' -e 'Tip: Route'`
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("email")}
			},
		},
		{
			Name: "h8mail_breach",
			Desc: "Check an EMAIL against breach/leak databases (h8mail). Largely keyless-limited — " +
				"configure HIBP/Hunter/Snusbase/Dehashed/IntelX keys via a config file passed in `options` " +
				"(e.g. '-c config.ini'), or point at a local breach compilation with '-bc /path --loose'. " +
				"(Note: h8mail is unmaintained upstream as of 2026 — some API integrations may have drifted.)",
			Params: []Param{
				{Name: "email", Type: StringParam, Desc: "email address", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra raw flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"h8mail", "-t", a.S("email")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("email")}
			},
		},
		{
			Name: "phoneinfoga",
			Desc: "OSINT on a PHONE NUMBER (phoneinfoga). Pass an E.164 number, e.g. '+15554441212'. " +
				"Keyless it gives an offline parse (country, E.164/national format) plus search dorks; " +
				"NUMVERIFY_API_KEY adds carrier/line type. marq runs no search engine: run the dorks " +
				"with your client's web search — if you have none, tell the user to add a web-search " +
				"skill or MCP server. (phoneinfoga upstream is unmaintained but functional.)",
			Params: []Param{
				{Name: "number", Type: StringParam, Desc: "E.164 phone number", Required: true},
			},
			Build: func(a Args) Invocation {
				argv := []string{"phoneinfoga", "scan", "-n", a.S("number")}
				return Invocation{Argv: argv, Target: a.S("number")}
			},
		},
	}
}
