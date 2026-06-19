package registry

import (
	"strconv"

	"marq/internal/shellword"
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
			Name: "holehe_email",
			Desc: "Discover which sites have an account registered to an EMAIL address (holehe), via " +
				"silent password-reset/registration probes — it does not alert the account owner. Only " +
				"prints sites where the email is in use.",
			Params: []Param{
				{Name: "email", Type: StringParam, Desc: "email address", Required: true},
			},
			Build: func(a Args) Invocation {
				argv := []string{"holehe", "--only-used", "--no-color", a.S("email")}
				return Invocation{Argv: argv, Target: a.S("email")}
			},
		},
		{
			Name: "h8mail_breach",
			Desc: "Check an EMAIL against breach/leak databases (h8mail). Largely keyless-limited — " +
				"configure HIBP/Hunter/Snusbase/Dehashed/IntelX keys via a config file passed in `options` " +
				"(e.g. '-c config.ini'), or point at a local breach compilation with '-bc /path --loose'.",
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
			Desc: "OSINT on a PHONE NUMBER (phoneinfoga): carrier, line type, country and footprint " +
				"search links. Pass an E.164 number, e.g. '+15554441212'. Optional NUMVERIFY_API_KEY / " +
				"GOOGLE_API_KEY+GOOGLECSE_CX enrich results.",
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
