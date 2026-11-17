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
			Name: "spiderfoot",
			Desc: "Broad automated OSINT footprint of a target (spiderfoot, 200+ modules) — domains, " +
				"IPs, netblocks, ASN, emails, names, breach data. `target` may be a domain, IP, email, " +
				"name or username. Always runs spiderfoot's `passive` use case (no modules that touch " +
				"the target). Runs in the BACKGROUND and returns a job dir to poll. " +
				"(Note: the open-source spiderfoot is unmaintained upstream as of 2026 — still functional; " +
				"cross-check anything critical.)",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "domain/IP/email/name/username", Required: true},
			},
			Build: func(a Args) Invocation {
				argv := []string{"spiderfoot", "-s", a.S("target"), "-u", "passive", "-o", "json"}
				return Invocation{Argv: argv, Target: a.S("target"), Background: true}
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
				cmd := `if [ -n "$SHODAN_API_KEY" ]; then shodan init "$SHODAN_API_KEY" >/dev/null 2>&1; ` +
					"shodan host " + shellword.Quote(a.S("ip")) + "; else " +
					"echo '(keyless InternetDB lookup; set SHODAN_API_KEY for full banners)'; " +
					"curl -sS --max-time 30 " + shellword.Quote("https://internetdb.shodan.io/"+url.PathEscape(a.S("ip"))) + "; fi"
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
				fields := "ip_str,port,org,hostnames,product"
				cmd := `[ -n "$SHODAN_API_KEY" ] || { echo "shodan_search needs SHODAN_API_KEY; for a single IP use shodan_host (keyless)" >&2; exit 2; }; ` +
					`shodan init "$SHODAN_API_KEY" >/dev/null 2>&1; ` +
					"shodan search --fields " + fields + " --limit " + strconv.Itoa(a.I("limit")) + " " + shellword.Quote(a.S("query"))
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
				src := shellword.Split(a.S("source"))
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
