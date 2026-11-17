package registry

import (
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/rhaist/marq/internal/shellword"
)

// harvesterKeyless are theHarvester sources that work with zero configuration.
// (theHarvester 4.11 dropped anubis, bing and threatminer; an unknown source fails the whole run.)
const harvesterKeyless = "crtsh,duckduckgo,otx,rapiddns,hackertarget,certspotter,urlscan,subdomaincenter"

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
				"or directory with exiftool. `path` must be inside the container — stage documents in the " +
				"mounted /work dir. Surfaces internal usernames and software versions.",
			Params: []Param{
				{Name: "path", Type: StringParam, Desc: "file or directory inside the container", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"exiftool", "-r", a.S("path")}, Target: a.S("path")}
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
			Desc: "Find leaked secrets in PUBLIC code with trufflehog. `source` must start with the " +
				"subcommand: 'git <https-url>', 'github --org=<org>', 'github --repo=<url>' or " +
				"'gitlab --repo=<url>'. Provide GITHUB_TOKEN for GitHub org scans (rate limits). " +
				"Secrets are reported unverified — marq never tests found credentials against the " +
				"service they belong to.",
			Params: []Param{
				{Name: "source", Type: StringParam, Desc: "subcommand + target, e.g. 'git https://github.com/org/repo'", Required: true},
			},
			Build: func(a Args) Invocation {
				src := shellword.Split(a.S("source"))
				if len(src) == 0 || !slices.Contains([]string{"git", "github", "gitlab"}, src[0]) {
					return Invocation{Target: a.S("source"), Refuse: "source must start with git, github or gitlab"}
				}
				// Verification logs in with the leaked credential — that's using it, not OSINT.
				if strings.Contains(strings.ToLower(a.S("source")), "verif") {
					return Invocation{Target: a.S("source"), Refuse: "verification flags are not allowed; secrets stay unverified"}
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
