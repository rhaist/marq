package registry

import (
	"strconv"
	"strings"

	"github.com/rhaist/marq/internal/shellword"
)

// people returns the people-footprint OSINT tools.
func people() []Tool {
	return []Tool{
		{
			Name: "maigret_username",
			Desc: "Find a USERNAME across ~2500 sites with profile-metadata extraction (maigret). " +
				"Slower on the full site list, so cap breadth with `top_sites` for a quick pass. Keyless.",
			Params: []Param{
				{Name: "username", Type: StringParam, Desc: "username", Required: true},
				{Name: "timeout", Type: IntParam, Desc: "per-site timeout seconds", Default: 30},
				{Name: "top_sites", Type: IntParam, Desc: "limit number of sites", Default: 500},
			},
			Build: func(a Args) Invocation {
				if strings.HasPrefix(a.S("username"), "-") {
					return Invocation{Target: a.S("username"), Refuse: "username must not start with '-'"}
				}
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
				// Drop skipped-module lines and the upstream proxy-affiliate and web-app ads.
				cmd := "set -o pipefail; user-scanner -e " + shellword.Quote(a.S("email")) +
					` </dev/null | grep -v -e '\[~\]' -e 'partner=' -e 'Tip: Route' -e 'user-scanner\.app'`
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("email")}
			},
		},
	}
}
