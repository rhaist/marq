package registry

import (
	"net"
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

// recon returns the reconnaissance and network-discovery tools.
func recon() []Tool {
	return []Tool{
		{
			Name:   "nmap",
			Active: true,
			Desc: "Run an nmap scan. `target` is a host, CIDR or hostname; `options` are raw nmap " +
				"flags (default a service/version scan). Use only against authorized targets — " +
				"every scan is audit-logged.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "host, CIDR or hostname", Required: true},
				{Name: "options", Type: StringParam, Desc: "raw nmap flags", Default: "-sV -T4"},
			},
			Build: func(a Args) Invocation {
				argv := append([]string{"nmap"}, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("target"))
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "masscan",
			Active: true,
			Desc: "Fast port sweep with masscan across `target` (CIDR/host). `rate` is packets/sec — " +
				"keep it conservative on shared networks.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "CIDR or host", Required: true},
				{Name: "ports", Type: StringParam, Desc: "port range", Default: "1-1000"},
				{Name: "rate", Type: IntParam, Desc: "packets per second", Default: 1000},
			},
			Build: func(a Args) Invocation {
				argv := []string{"masscan", a.S("target"), "-p", a.S("ports"), "--rate", strconv.Itoa(a.I("rate"))}
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name: "dns_lookup",
			Desc: "Query DNS records for a domain using dig.",
			Params: []Param{
				{Name: "domain", Type: StringParam, Desc: "domain to query", Required: true},
				{Name: "record_type", Type: StringParam, Desc: "DNS record type", Default: "ANY"},
			},
			Build: func(a Args) Invocation {
				argv := []string{"dig", "+nocmd", "+noall", "+answer", a.S("domain"), a.S("record_type")}
				return Invocation{Argv: argv, Target: a.S("domain")}
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
			Name:   "httpx_probe",
			Active: true,
			Desc: "Probe one or more hosts (comma- or newline-separated) for live HTTP services, " +
				"returning status, title and tech. Wraps projectdiscovery httpx.",
			Params: []Param{
				{Name: "targets", Type: StringParam, Desc: "comma/newline-separated hosts", Required: true},
			},
			Build: func(a Args) Invocation {
				// Kali ships ProjectDiscovery httpx as httpx-toolkit (plain httpx is
				// the unrelated python HTTP client).
				argv := []string{"httpx-toolkit", "-silent", "-status-code", "-title", "-tech-detect"}
				return Invocation{Argv: argv, Target: a.S("targets"), Stdin: strings.Join(splitHosts(a.S("targets")), "\n")}
			},
		},
		{
			Name:   "naabu",
			Active: true,
			Desc: "Fast modern port scan with naabu (projectdiscovery). Give explicit `ports` " +
				"(e.g. \"80,443,8000-9000\") or rely on `top_ports`. SYN-scans with NET_RAW, else " +
				"falls back to a connect scan. Pairs well with nmap -sV on the discovered ports.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "host", Required: true},
				{Name: "ports", Type: StringParam, Desc: "explicit ports", Default: ""},
				{Name: "top_ports", Type: IntParam, Desc: "top N ports if ports empty", Default: 100},
			},
			Build: func(a Args) Invocation {
				argv := []string{"naabu", "-host", a.S("target"), "-silent"}
				if a.S("ports") != "" {
					argv = append(argv, "-p", a.S("ports"))
				} else {
					argv = append(argv, "-top-ports", strconv.Itoa(a.I("top_ports")))
				}
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name: "dnsx",
			Desc: "Resolve and enumerate DNS for hosts (comma/newline-separated) with dnsx. " +
				"`records` is a comma list of types to query (a,aaaa,cname,mx,ns,txt,ptr,srv). " +
				"Returns the records inline.",
			Params: []Param{
				{Name: "hosts", Type: StringParam, Desc: "comma/newline-separated hosts", Required: true},
				{Name: "records", Type: StringParam, Desc: "comma list of record types", Default: "a"},
			},
			Build: func(a Args) Invocation {
				argv := []string{"dnsx", "-silent", "-resp"}
				for rec := range strings.SplitSeq(a.S("records"), ",") {
					rec = strings.ToLower(strings.TrimSpace(rec))
					if rec != "" {
						argv = append(argv, "-"+rec)
					}
				}
				return Invocation{Argv: argv, Target: a.S("hosts"), Stdin: strings.Join(splitHosts(a.S("hosts")), "\n")}
			},
		},
		{
			Name: "dnsrecon",
			Desc: "DNS reconnaissance with dnsrecon. `scan_type` selects the technique: std (records), " +
				"brt (bruteforce), axfr (zone transfer), crt (crt.sh certs), zonewalk. Good for " +
				"zone-transfer checks and record sweeps.",
			Params: []Param{
				{Name: "domain", Type: StringParam, Desc: "domain", Required: true},
				{Name: "scan_type", Type: StringParam, Desc: "std/brt/axfr/crt/zonewalk", Default: "std"},
			},
			Build: func(a Args) Invocation {
				argv := []string{"dnsrecon", "-d", a.S("domain"), "-t", a.S("scan_type")}
				return Invocation{Argv: argv, Target: a.S("domain")}
			},
		},
		{
			Name:   "ssh_audit",
			Active: true,
			Desc:   "Audit an SSH server's algorithms and configuration with ssh-audit.",
			Params: []Param{
				{Name: "host", Type: StringParam, Desc: "SSH host", Required: true},
				{Name: "port", Type: IntParam, Desc: "SSH port", Default: 22},
			},
			Build: func(a Args) Invocation {
				argv := []string{"ssh-audit", "-p", strconv.Itoa(a.I("port")), a.S("host")}
				return Invocation{Argv: argv, Target: a.S("host")}
			},
		},
		{
			Name:   "fping_sweep",
			Active: true,
			Desc:   "Ping sweep a CIDR or host list with fping (-a -q -g).",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "CIDR or host list", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"fping", "-a", "-q", "-g", a.S("target")}, Target: a.S("target")}
			},
		},
		{
			Name:   "snmp_walk",
			Active: true,
			Desc:   "Walk an SNMP MIB tree with snmpwalk (v2c).",
			Params: []Param{
				{Name: "host", Type: StringParam, Desc: "SNMP host", Required: true},
				{Name: "community", Type: StringParam, Desc: "community string", Default: "public"},
				{Name: "options", Type: StringParam, Desc: "extra snmpwalk flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"snmpwalk", "-v", "2c", "-c", a.S("community")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("host"))
				return Invocation{Argv: argv, Target: a.S("host")}
			},
		},
		{
			Name:   "snmp_check",
			Active: true,
			Desc:   "Quick SNMP enumeration with snmp-check.",
			Params: []Param{
				{Name: "host", Type: StringParam, Desc: "SNMP host", Required: true},
				{Name: "community", Type: StringParam, Desc: "community string", Default: "public"},
			},
			Build: func(a Args) Invocation {
				// snmp-check (Cantoni's CLI enumerator) takes the target positionally;
				// -c sets the community. NB: the hyphen-less `snmpcheck` is an unrelated
				// Perl/Tk GUI tool that crashes headless — do not use it.
				return Invocation{Argv: []string{"snmp-check", "-c", a.S("community"), a.S("host")}, Target: a.S("host")}
			},
		},
		{
			Name:   "snmp_brute",
			Active: true,
			Desc:   "Brute-force SNMP community strings with onesixtyone.",
			Params: []Param{
				{Name: "host", Type: StringParam, Desc: "SNMP host", Required: true},
				{Name: "wordlist", Type: StringParam, Desc: "community string wordlist path", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"onesixtyone", "-c", a.S("wordlist"), a.S("host")}, Target: a.S("host")}
			},
		},
		{
			Name:   "smtp_user_enum",
			Active: true,
			Desc:   "Enumerate SMTP users with smtp-user-enum.",
			Params: []Param{
				{Name: "host", Type: StringParam, Desc: "SMTP host", Required: true},
				{Name: "method", Type: StringParam, Desc: "VRFY/EXPN/RCPT", Default: "VRFY"},
				{Name: "users", Type: StringParam, Desc: "user list file path", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra smtp-user-enum flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"smtp-user-enum", "-M", a.S("method"), "-U", a.S("users"), "-t", a.S("host")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("host")}
			},
		},
		{
			Name:   "smtp_test",
			Active: true,
			Desc:   "SMTP testing with swaks — test relay, injection, auth. Flexible Swiss-army knife for SMTP.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "target host", Required: true},
				{Name: "options", Type: StringParam, Desc: "raw swaks flags (--to --from --body ...)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"swaks", "--server", a.S("target")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("target")}
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
						` | python3 -I -c 'import json,sys; [print(p["prefix"]) for p in json.load(sys.stdin)["data"]["prefixes"]]'`
				} else if net.ParseIP(t) != nil {
					cmd = cymru(t)
				} else if strings.Contains(t, ".") && !strings.ContainsAny(t, " /") {
					cmd = "ips=$({ dig +short A " + shellword.Quote(t) + "; dig +short AAAA " + shellword.Quote(t) + "; } | grep -E '^[0-9a-fA-F:.]+$'); " +
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
				// up a rotated token every time.
				cmd := `[ -n "$CENSYS_API_TOKEN" ] || { echo "CENSYS_API_TOKEN not set" >&2; exit 2; }; ` +
					`export CENCLI_DATA_DIR="$(mktemp -d)"; trap 'rm -rf "$CENCLI_DATA_DIR"' EXIT; ` +
					`printf %s "$CENSYS_API_TOKEN" | censys config auth add --value-file - --name marq -q >/dev/null || exit 1; ` +
					`censys view ` + shellword.Quote(a.S("assets")) + ` -O json --no-spinner --no-color ${CENSYS_ORG_ID:+-o "$CENSYS_ORG_ID"}`
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("assets")}
			},
		},
	}
}
