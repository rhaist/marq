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
	}
}
