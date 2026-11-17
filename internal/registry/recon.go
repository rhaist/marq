package registry

import (
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/rhaist/marq/internal/shellword"
)

// splitHosts turns a comma/newline-separated list into trimmed host entries.
func splitHosts(s string) []string {
	s = strings.ReplaceAll(s, ",", "\n")
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		if h := strings.TrimSpace(line); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// dnsTypes are the record types the DNS tools may query: normal resolution
// only — no AXFR/IXFR (zone transfer against the target's nameservers) and no
// way to smuggle a tool flag in as a "record type".
var dnsTypes = []string{"a", "aaaa", "cname", "mx", "ns", "txt", "soa", "caa", "srv", "ptr", "any"}

// recon returns the passive DNS / network-registry lookup tools.
func recon() []Tool {
	return []Tool{
		{
			Name: "dns_lookup",
			Desc: "Query DNS records for a domain using dig, via the system resolver. `record_type` is " +
				"one of A, AAAA, CNAME, MX, NS, TXT, SOA, CAA, SRV, PTR, ANY (no zone transfers).",
			Params: []Param{
				{Name: "domain", Type: StringParam, Desc: "domain to query", Required: true},
				{Name: "record_type", Type: StringParam, Desc: "DNS record type", Default: "ANY"},
			},
			Build: func(a Args) Invocation {
				d, rt := a.S("domain"), strings.ToLower(strings.TrimSpace(a.S("record_type")))
				if !slices.Contains(dnsTypes, rt) {
					return Invocation{Target: d, Refuse: "record_type must be one of " + strings.Join(dnsTypes, ", ")}
				}
				if strings.ContainsAny(d[:1], "-+@") { // a dig option or @server, not a name
					return Invocation{Target: d, Refuse: "domain must be a DNS name"}
				}
				argv := []string{"dig", "+nocmd", "+noall", "+answer", d, rt}
				return Invocation{Argv: argv, Target: d}
			},
		},
		{
			Name: "whois_lookup",
			Desc: "WHOIS registration lookup for a domain or IP.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "domain or IP", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"whois", a.S("target")}, Target: a.S("target")}
			},
		},
		{
			Name: "subfinder",
			Desc: "Passive subdomain enumeration for a domain via subfinder.",
			Params: []Param{
				{Name: "domain", Type: StringParam, Desc: "domain", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"subfinder", "-silent", "-d", a.S("domain")}, Target: a.S("domain")}
			},
		},
		{
			Name: "dnsx",
			Desc: "Resolve and enumerate DNS for hosts (comma/newline-separated) with dnsx. " +
				"`records` is a comma list of types to query (a,aaaa,cname,mx,ns,txt,soa,caa,srv,ptr). " +
				"Returns the records inline.",
			Params: []Param{
				{Name: "hosts", Type: StringParam, Desc: "comma/newline-separated hosts", Required: true},
				{Name: "records", Type: StringParam, Desc: "comma list of record types", Default: "a"},
			},
			Build: func(a Args) Invocation {
				argv := []string{"dnsx", "-silent", "-resp"}
				for rec := range strings.SplitSeq(a.S("records"), ",") {
					rec = strings.ToLower(strings.TrimSpace(rec))
					if rec == "" {
						continue
					}
					if rec == "any" || !slices.Contains(dnsTypes, rec) { // dnsx has no -any
						return Invocation{Target: a.S("hosts"), Refuse: "records must be a comma list of a,aaaa,cname,mx,ns,txt,soa,caa,srv,ptr"}
					}
					argv = append(argv, "-"+rec)
				}
				return Invocation{Argv: argv, Target: a.S("hosts"), Stdin: strings.Join(splitHosts(a.S("hosts")), "\n")}
			},
		},
		{
			Name: "asnmap",
			Desc: "Map IPs and domains to their ASN, and an ASN to its announced CIDR ranges — keyless " +
				"(Team Cymru whois + RIPEstat). `target` is an IP (→ ASN, BGP prefix, country, AS " +
				"name), a domain (resolved, then the same per IP) or an ASN like AS13335 (→ AS " +
				"name + announced prefixes). Org names aren't supported: search " +
				"https://bgp.he.net/search?search[search]=<org> with your web fetch to find the ASN first.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "IP, domain or ASN (AS12345)", Required: true},
			},
			Build: func(a Args) Invocation {
				// asnmap itself now needs a ProjectDiscovery cloud key and prompts
				// on a TTY, so this keeps the name with keyless sources instead.
				t := strings.TrimSpace(a.S("target"))
				cymru := func(q string) string { return "whois -h whois.cymru.com " + shellword.Quote(" -v "+q) }
				var cmd string
				if n, err := strconv.ParseUint(strings.TrimPrefix(strings.ToUpper(t), "AS"), 10, 32); err == nil {
					asn := "AS" + strconv.FormatUint(n, 10)
					cmd = cymru(asn) + "; echo; echo '# announced prefixes (RIPEstat)'; " +
						"curl -sS --max-time 30 " + shellword.Quote("https://stat.ripe.net/data/announced-prefixes/data.json?resource="+asn) +
						` | python3 -I -c 'import json,sys
try: ps=[p["prefix"] for p in json.load(sys.stdin)["data"]["prefixes"]]
except Exception as e: sys.exit("RIPEstat lookup failed: %r" % e)
print("\n".join(ps) or "(no announced prefixes)")'`
				} else if net.ParseIP(t) != nil {
					cmd = cymru(t)
				} else if strings.Contains(t, ".") && !strings.ContainsAny(t, " /") && !strings.ContainsAny(t[:1], "-+@") { // no dig options
					cmd = "ips=$({ dig +short A " + shellword.Quote(t) + "; dig +short AAAA " + shellword.Quote(t) + "; } | grep -v '\\.$' | grep -E '^[0-9a-fA-F:.]+$'); " +
						`[ -n "$ips" ] || { echo "no A/AAAA records" >&2; exit 1; }; ` +
						`echo 'AS | IP | BGP Prefix | CC | Registry | Allocated | AS Name'; ` +
						`for ip in $ips; do whois -h whois.cymru.com " -v $ip" | tail -n +2; done`
				} else {
					cmd = `echo "asnmap: target must be an IP, domain or ASN (AS12345); for an org name, search bgp.he.net with your web fetch" >&2; exit 2`
				}
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: t}
			},
		},
		{
			Name: "cdncheck",
			Desc: "Identify whether IPs belong to a CDN, cloud, or WAF provider with cdncheck (ProjectDiscovery). " +
				"`target` is a comma/newline-separated IP list. Useful to exclude third-party infra from scans.",
			Params: []Param{
				{Name: "targets", Type: StringParam, Desc: "comma/newline-separated IPs", Required: true},
			},
			Build: func(a Args) Invocation {
				argv := []string{"cdncheck", "-silent"}
				return Invocation{Argv: argv, Target: a.S("targets"), Stdin: strings.Join(splitHosts(a.S("targets")), "\n")}
			},
		},
		{
			Name: "censys_view",
			Desc: "Look up hosts, web properties or certificates on Censys (Platform API via cencli): " +
				"services, software, TLS, location. `assets` is comma-separated IPs, host:port, " +
				"domains (port 443 default) or cert SHA-256s. Requires CENSYS_API_TOKEN (a Personal " +
				"Access Token); CENSYS_ORG_ID is optional. Free accounts get lookups only, 100 " +
				"credits/month at 1 per asset — Censys *search* is paid-only, use shodan_search.",
			Params: []Param{
				{Name: "assets", Type: StringParam, Desc: "comma-separated IPs/host:port/domains/cert hashes", Required: true},
			},
			Build: func(a Args) Invocation {
				// cencli has no token env var, only a stored config; a per-call
				// throwaway CENCLI_DATA_DIR keeps tokens from piling up and picks
				// up a rotated token every time. It lives in $HOME, outside the
				// read_file roots, so a SIGKILLed call can't leave the token readable.
				// `--` keeps a model-supplied "-i <file>" from uploading a local file.
				cmd := `[ -n "$CENSYS_API_TOKEN" ] || { echo "CENSYS_API_TOKEN not set" >&2; exit 2; }; ` +
					`export CENCLI_DATA_DIR="$(mktemp -d -p "$HOME")"; trap 'rm -rf "$CENCLI_DATA_DIR"' EXIT; ` +
					`printf %s "$CENSYS_API_TOKEN" | censys config auth add --value-file - --name marq -q >/dev/null || exit 1; ` +
					`censys view -O json --no-spinner --no-color ${CENSYS_ORG_ID:+-o "$CENSYS_ORG_ID"} -- ` + shellword.Quote(a.S("assets"))
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("assets")}
			},
		},
	}
}
