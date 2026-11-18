package registry

import (
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/rhaist/marq/internal/files"
	"github.com/rhaist/marq/internal/shellword"
)

// harvesterKeyless are theHarvester sources that work with zero configuration.
// (theHarvester 4.11 dropped anubis, bing and threatminer; an unknown source fails the whole run.)
const harvesterKeyless = "crtsh,duckduckgo,otx,rapiddns,hackertarget,certspotter,urlscan,subdomaincenter"

// publicHosts are the public code-hosting services trufflehog may read from. A
// target's self-hosted git/GitLab server is its own infrastructure, so it's out.
var publicHosts = []string{"github.com", "gitlab.com", "bitbucket.org", "codeberg.org"}

// publicCodeSource allowlists the whole trufflehog source, token by token, and
// returns a refusal message for anything else: another flag could re-enable
// verification (which logs in with the leaked secret) or point --endpoint at
// the target's own server.
func publicCodeSource(src []string) string {
	const usage = "source must be 'git <https-url>', 'github --org=<org>' or 'github --repo=<url>' " +
		"on github.com, gitlab.com, bitbucket.org or codeberg.org, with no other flags"
	publicURL := func(raw string) bool {
		u, err := url.Parse(raw)
		return err == nil && u.Scheme == "https" && u.User == nil &&
			slices.Contains(publicHosts, strings.ToLower(strings.TrimPrefix(u.Host, "www.")))
	}
	if len(src) < 2 {
		return usage
	}
	for _, tok := range src[1:] {
		repo, isRepo := strings.CutPrefix(tok, "--repo=")
		org, isOrg := strings.CutPrefix(tok, "--org=")
		ok := false
		switch src[0] {
		case "git":
			ok = len(src) == 2 && publicURL(tok)
		case "github":
			ok = (isRepo && publicURL(repo)) || (isOrg && org != "" && !strings.ContainsAny(org, "/:"))
		}
		if !ok {
			return usage
		}
	}
	return ""
}

// osint returns company/domain footprint & information-gathering tools.
func osint() []Tool {
	return []Tool{
		{
			Name: "theharvester",
			Desc: "Footprint a DOMAIN with theHarvester: emails, employee names, hosts and subdomains " +
				"gathered from public search/cert sources. Defaults to keyless sources; pass `sources` " +
				"(comma list) to use configured ones. The single best opening recon move.",
			Params: []Param{
				{Name: "domain", Type: StringParam, Desc: "domain", Required: true},
				{Name: "sources", Type: StringParam, Desc: "comma list of sources", Default: ""},
				{Name: "limit", Type: IntParam, Desc: "result limit", Default: 500},
			},
			Build: func(a Args) Invocation {
				sources := a.S("sources")
				if sources == "" {
					sources = harvesterKeyless
				}
				argv := []string{"theHarvester", "-d", a.S("domain"), "-b", sources, "-l", strconv.Itoa(a.I("limit"))}
				return Invocation{Argv: argv, Target: a.S("domain")}
			},
		},
		{
			Name: "exif_metadata",
			Desc: "Extract embedded metadata (author, software, GPS, internal paths, emails) from a file " +
				"or directory with exiftool. `path` must be inside the working area (/work or /tmp) — stage " +
				"downloaded public documents there. Surfaces internal usernames and software versions.",
			Params: []Param{
				{Name: "path", Type: StringParam, Desc: "file or directory inside the container", Required: true},
			},
			Build: func(a Args) Invocation {
				real, err := files.Resolve(a.S("path")) // same sandbox as read_file
				if err != nil {
					return Invocation{Target: a.S("path"), Refuse: err.Error()}
				}
				return Invocation{Argv: []string{"exiftool", "-r", real}, Target: a.S("path")}
			},
		},
		{
			Name: "shodan_host",
			Desc: "Look up an IP on Shodan: open ports, services, banners, vulns. Works without a key " +
				"via Shodan's free InternetDB (ports, CPEs, known CVEs, hostnames, tags as JSON); " +
				"SHODAN_API_KEY adds full service banners.",
			Params: []Param{
				{Name: "ip", Type: StringParam, Desc: "IP address", Required: true},
			},
			Build: func(a Args) Invocation {
				if net.ParseIP(a.S("ip")) == nil {
					return Invocation{Target: a.S("ip"), Refuse: "ip must be an IP address"}
				}
				ip := url.PathEscape(a.S("ip"))
				// The key reaches curl on stdin, so it is in neither the process list nor the audit argv.
				// The reply is captured whole and then capped, so curl's own exit status is reported.
				cmd := `if [ -n "$SHODAN_API_KEY" ]; then r=$(printf %s "$SHODAN_API_KEY" | curl -sS --max-time 30 -G --data-urlencode key@- ` +
					shellword.Quote("https://api.shodan.io/shodan/host/"+ip) + "); else " +
					"echo '(keyless InternetDB lookup; set SHODAN_API_KEY for full banners)'; " +
					"r=$(curl -sS --max-time 30 " + shellword.Quote("https://internetdb.shodan.io/"+ip) + "); fi; rc=$?; " +
					`if [ ${#r} -gt 30000 ]; then printf '%s\n... (truncated at 30 kB)\n' "${r:0:30000}"; else printf '%s\n' "$r"; fi; exit $rc`
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("ip")}
			},
		},
		{
			Name: "shodan_search",
			Desc: "Search Shodan for exposed assets, e.g. 'org:\"Acme Corp\"' or " +
				"'ssl.cert.subject.cn:example.com'. Returns ip/port/org/product/hostnames. Requires " +
				"SHODAN_API_KEY with query credits; for a single IP use shodan_host, which works keyless.",
			Params: []Param{
				{Name: "query", Type: StringParam, Desc: "Shodan search query", Required: true},
				{Name: "limit", Type: IntParam, Desc: "result limit", Default: 100},
			},
			Build: func(a Args) Invocation {
				limit := min(max(a.I("limit"), 1), 100) // one API page holds 100 matches
				cmd := `[ -n "$SHODAN_API_KEY" ] || { echo "shodan_search needs SHODAN_API_KEY; for a single IP use shodan_host (keyless)" >&2; exit 2; }; ` +
					`set -o pipefail; printf %s "$SHODAN_API_KEY" | curl -sS --max-time 60 -G --data-urlencode key@- --data-urlencode minify=true ` +
					"--data-urlencode " + shellword.Quote("query="+a.S("query")) + " https://api.shodan.io/shodan/host/search | " +
					"python3 -c " + shellword.Quote(shodanCompact) + " " + strconv.Itoa(limit)
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("query")}
			},
		},
		{
			Name: "trufflehog",
			Desc: "Find leaked secrets in PUBLIC code with trufflehog. `source` is 'git <https-url>', " +
				"'github --org=<org>' or 'github --repo=<url>', on github.com, gitlab.com, bitbucket.org " +
				"or codeberg.org only (GitLab repos via 'git <url>') — no self-hosted servers, no extra " +
				"flags. Provide GITHUB_TOKEN for GitHub org scans (rate limits). Secrets are reported " +
				"unverified — marq never tests found credentials against the service they belong to.",
			Params: []Param{
				{Name: "source", Type: StringParam, Desc: "subcommand + target, e.g. 'git https://github.com/org/repo'", Required: true},
			},
			Build: func(a Args) Invocation {
				src := strings.Fields(a.S("source")) // a quoted token fails publicCodeSource: refused, not unquoted
				if msg := publicCodeSource(src); msg != "" {
					return Invocation{Target: a.S("source"), Refuse: msg}
				}
				argv := append([]string{"trufflehog"}, src...)
				argv = append(argv, "--no-verification", "--json")
				return Invocation{Argv: argv, Target: a.S("source")}
			},
		},
		{
			Name: "gau_urls",
			Desc: "Fetch known URLs for a DOMAIN from Wayback, Common Crawl, OTX and URLScan (gau) — " +
				"broad historical coverage of old endpoints, parameters and forgotten assets. Passive: " +
				"queries archives, not the target. Runs in the background (archive volume varies wildly " +
				"by domain); poll with list_jobs / job_status and read the job's stdout.log.",
			Params: []Param{
				{Name: "domain", Type: StringParam, Desc: "domain", Required: true},
			},
			Build: func(a Args) Invocation {
				// Background: gau's runtime is set by the target's archive size (minutes for
				// heavily-archived domains), which the caller can't scope down — so a
				// synchronous run blows the client call timeout. See CLAUDE.md.
				cmd := "echo " + shellword.Quote(a.S("domain")) + " | gau --subs --providers wayback,commoncrawl,otx,urlscan"
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("domain"), Background: true}
			},
		},
	}
}

// shodanCompact prints one line per Shodan search match (ip, port, org,
// hostnames, product), the fields the old CLI's --fields gave; argv[1] is the
// already-clamped limit.
const shodanCompact = `import json,sys
raw=sys.stdin.read()
try: d=json.loads(raw)
except ValueError: sys.exit("shodan: unexpected response (bad key or plan?): "+raw[:300])
if "error" in d: sys.exit("shodan: "+d["error"])
print("total:", d.get("total"))
for m in d.get("matches", [])[:int(sys.argv[1])]:
    print(m.get("ip_str"), m.get("port"), m.get("org") or "", ",".join(m.get("hostnames") or []), m.get("product") or "", sep="\t")`
