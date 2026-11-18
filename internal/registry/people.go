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
				// --no-recursion: by default maigret re-searches usernames it extracts from found
				// profiles, which pulls in unrelated people (a search for one handle returned others').
				cmd := "set -o pipefail; " + shellword.Join([]string{
					"maigret", a.S("username"),
					"--no-color", "--no-progressbar", "--no-recursion",
					"--timeout", strconv.Itoa(a.I("timeout")),
					"--top-sites", strconv.Itoa(a.I("top_sites")),
				}) + " | " + fromLine("Checking username", "rapidproxy|Donate on|Support Maigret")
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("username")}
			},
		},
		{
			Name: "email_accounts",
			Desc: "Discover which sites have an account registered to an EMAIL address (user-scanner), " +
				"via silent registration/login probes. Modules that could send the owner a " +
				"password-reset email are skipped, so it does not alert the account owner. Prints " +
				"sites where the email is registered; a few sites may be unreachable (rate limits). Hits " +
				"are unconfirmed and some modules report false positives (Letterboxd flagged a random " +
				"address as registered), so corroborate a hit before reporting it about a person.",
			Params: []Param{
				{Name: "email", Type: StringParam, Desc: "email address", Required: true},
			},
			Build: func(a Args) Invocation {
				// Drop the banner, skipped-module lines and the upstream proxy-affiliate and web-app ads.
				cmd := "set -o pipefail; user-scanner -e " + shellword.Quote(a.S("email")) + " </dev/null | " +
					fromLine("Checking email", `\[~\]|partner=|Tip: Route|user-scanner\.app`)
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("email")}
			},
		},
	}
}

// fromLine is an awk filter that drops a tool's banner (everything before the
// first line matching start) and any line matching drop. If start never
// matches, nothing before it is dropped, so an early error still shows. The
// patterns become awk regex literals, so they must not contain '/'.
func fromLine(start, drop string) string {
	return "awk " + shellword.Quote("/"+drop+"/{next} f{print;next} /"+start+`/{f=1;print;next} {b=b $0 ORS} END{if(!f)printf "%s",b}`)
}
