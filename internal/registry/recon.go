package registry

import (
	"strconv"
	"strings"

	"marq/internal/shellword"
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
			Name: "nmap",
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
			Name: "masscan",
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
			Name: "httpx_probe",
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
			Name: "naabu",
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
			Name: "ssh_audit",
			Desc: "Audit an SSH server's algorithms and configuration with ssh-audit.",
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
			Name: "fping_sweep",
			Desc: "Ping sweep a CIDR or host list with fping (-a -q -g).",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "CIDR or host list", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"fping", "-a", "-q", "-g", a.S("target")}, Target: a.S("target")}
			},
		},
		{
			Name: "snmp_walk",
			Desc: "Walk an SNMP MIB tree with snmpwalk (v2c).",
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
			Name: "snmp_check",
			Desc: "Quick SNMP enumeration with snmpcheck.",
			Params: []Param{
				{Name: "host", Type: StringParam, Desc: "SNMP host", Required: true},
				{Name: "community", Type: StringParam, Desc: "community string", Default: "public"},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"snmpcheck", "-t", a.S("host"), "-c", a.S("community")}, Target: a.S("host")}
			},
		},
		{
			Name: "snmp_brute",
			Desc: "Brute-force SNMP community strings with onesixtyone.",
			Params: []Param{
				{Name: "host", Type: StringParam, Desc: "SNMP host", Required: true},
				{Name: "wordlist", Type: StringParam, Desc: "community string wordlist path", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"onesixtyone", "-c", a.S("wordlist"), a.S("host")}, Target: a.S("host")}
			},
		},
		{
			Name: "smtp_user_enum",
			Desc: "Enumerate SMTP users with smtp-user-enum.",
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
			Name: "smtp_test",
			Desc: "SMTP testing with swaks — test relay, injection, auth. Flexible Swiss-army knife for SMTP.",
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
			Desc: "Map ASN to CIDR ranges / IP to ASN / organization to network ranges with asnmap (ProjectDiscovery). " +
				"`target` is an IP, ASN (e.g. AS12345), or org name. Returns CIDR blocks.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "IP, ASN or org name", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra asnmap flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"asnmap", "-silent"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("target"))
				return Invocation{Argv: argv, Target: a.S("target")}
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
			Name: "censys_search",
			Desc: "Search Censys for exposed assets. Requires CENSYS_API_ID and CENSYS_API_SECRET env vars.",
			Params: []Param{
				{Name: "query", Type: StringParam, Desc: "Censys query", Required: true},
				{Name: "index", Type: StringParam, Desc: "hosts/certs/v2", Default: "hosts"},
				{Name: "options", Type: StringParam, Desc: "extra censys flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				cmd := "if [ -n \"$CENSYS_API_ID\" ]; then export CENSYS_API_ID; fi; if [ -n \"$CENSYS_API_SECRET\" ]; then export CENSYS_API_SECRET; fi; censys search " + shellword.Quote(a.S("query")) + " --type " + a.S("index") + " " + a.S("options")
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: a.S("query")}
			},
		},
	}
}
