package registry

import (
	"strconv"

	"marq/internal/shellword"
)

// harvesterKeyless are theHarvester sources that work with zero configuration.
const harvesterKeyless = "crtsh,duckduckgo,bing,otx,rapiddns,hackertarget,certspotter,anubis,threatminer"

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
				"name or username. `use_case` is one of all/footprint/investigate/passive (`passive` is " +
				"safe + fastest). Runs in the BACKGROUND and returns a job dir to poll. " +
				"(Note: the open-source spiderfoot is unmaintained upstream as of 2026 — still functional; " +
				"cross-check anything critical.)",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "domain/IP/email/name/username", Required: true},
				{Name: "use_case", Type: StringParam, Desc: "all/footprint/investigate/passive", Default: "passive"},
			},
			Build: func(a Args) Invocation {
				argv := []string{"spiderfoot", "-s", a.S("target"), "-u", a.S("use_case"), "-o", "json"}
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
			Desc: "Look up an IP on Shodan: open ports, services, banners, vulns. Requires SHODAN_API_KEY " +
				"in the container environment.",
			Params: []Param{
				{Name: "ip", Type: StringParam, Desc: "IP address", Required: true},
			},
			Build: func(a Args) Invocation {
				cmd := `if [ -n "$SHODAN_API_KEY" ]; then shodan init "$SHODAN_API_KEY" >/dev/null 2>&1; fi; ` +
					"shodan host " + shellword.Quote(a.S("ip"))
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("ip")}
			},
		},
		{
			Name: "shodan_search",
			Desc: "Search Shodan for exposed assets, e.g. 'org:\"Acme Corp\"' or " +
				"'ssl.cert.subject.cn:example.com'. Returns ip/port/org/product/hostnames. Requires " +
				"SHODAN_API_KEY in the container environment.",
			Params: []Param{
				{Name: "query", Type: StringParam, Desc: "Shodan search query", Required: true},
				{Name: "limit", Type: IntParam, Desc: "result limit", Default: 100},
			},
			Build: func(a Args) Invocation {
				fields := "ip_str,port,org,hostnames,product"
				cmd := `if [ -n "$SHODAN_API_KEY" ]; then shodan init "$SHODAN_API_KEY" >/dev/null 2>&1; fi; ` +
					"shodan search --fields " + fields + " --limit " + strconv.Itoa(a.I("limit")) + " " + shellword.Quote(a.S("query"))
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("query")}
			},
		},
		{
			Name: "gitleaks",
			Desc: "Scan a local git repo or directory for committed secrets (gitleaks). `path` is a " +
				"checked-out repo/dir inside the container. Reports findings as JSON to stdout.",
			Params: []Param{
				{Name: "path", Type: StringParam, Desc: "repo/dir inside the container", Required: true},
			},
			Build: func(a Args) Invocation {
				argv := []string{"gitleaks", "dir", a.S("path"), "--report-format", "json", "--report-path", "/dev/stdout"}
				return Invocation{Argv: argv, Target: a.S("path")}
			},
		},
		{
			Name: "trufflehog",
			Desc: "Find (and verify) leaked secrets with trufflehog. `source` is a git URL or " +
				"'github --org=<org>'-style target passed through as raw args. Provide a GITHUB_TOKEN env " +
				"for GitHub org/repo scans. Defaults to only verified secrets to cut noise.",
			Params: []Param{
				{Name: "source", Type: StringParam, Desc: "git URL or raw trufflehog target args", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra raw flags", Default: "--results=verified"},
			},
			Build: func(a Args) Invocation {
				argv := []string{"trufflehog"}
				argv = append(argv, shellword.Split(a.S("source"))...)
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, "--json")
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
