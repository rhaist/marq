package registry

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/rhaist/marq/internal/shellword"
)

// domainRE is a plain DNS name (no scheme, port, path, leading dash or @), so a
// model-supplied value can't become a flag or a URL.
var domainRE = regexp.MustCompile(`(?i)^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

var hashRE = regexp.MustCompile(`(?i)^([0-9a-f]{32}|[0-9a-f]{40}|[0-9a-f]{64})$`)

// fetch is a curl of one fixed third-party URL, capped so one oversized
// response (OTX runs to ~440 KB) can't crowd the rest out of the output budget.
func fetch(label, u string) string {
	return "echo " + shellword.Quote("# "+label) + "; curl -sS --max-time 30 " + shellword.Quote(u) + " | head -c 20000; echo; "
}

// intel returns the certificate, registration, lookalike-domain and
// threat-intel lookups. Every one queries a third-party service only.
func intel() []Tool {
	return []Tool{
		{
			Name: "ct_search",
			Desc: "Search Certificate Transparency logs (crt.sh) for certificates issued for a DOMAIN and " +
				"its subdomains: SANs, issuer, not-before/not-after. Shows when a host got its certificate " +
				"(e.g. a phishing domain certified just before the send) and names subfinder may miss. " +
				"Expired certificates are excluded. Keyless; crt.sh is slow on very large domains.",
			Params: []Param{
				{Name: "domain", Type: StringParam, Desc: "domain, e.g. acme.com", Required: true},
			},
			Build: func(a Args) Invocation {
				d := strings.ToLower(strings.TrimSpace(a.S("domain")))
				if !domainRE.MatchString(d) {
					return Invocation{Target: d, Refuse: "domain must be a DNS name like acme.com"}
				}
				u := "https://crt.sh/?q=" + url.QueryEscape("%."+d) + "&exclude=expired&deduplicate=Y&output=json"
				cmd := "set -o pipefail; curl -sS --max-time 90 " + shellword.Quote(u) + ` | python3 -I -c 'import json,sys
try: rows=json.load(sys.stdin)
except Exception as e: sys.exit("crt.sh returned no JSON (it times out on big domains): %r" % e)
for r in sorted(rows, key=lambda r: r.get("not_before",""), reverse=True)[:300]:
    print(r.get("not_before","")[:10], r.get("not_after","")[:10], r.get("issuer_name","").split("O=")[-1].split(",")[0], " ".join(sorted(set(r.get("name_value","").split()))), sep=" | ")
print("(%d certificates)" % len(rows))'`
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: d}
			},
		},
		{
			Name: "rdap",
			Desc: "Structured registration data over RDAP (the successor to WHOIS): registrar, " +
				"registration/expiry dates, status, nameservers and contacts for a domain; holder and " +
				"allocation for an IP or ASN (AS13335). Queries the registry servers from the IANA " +
				"bootstrap, never the target. ccTLD coverage varies — fall back to whois_lookup.",
			Params: []Param{
				{Name: "query", Type: StringParam, Desc: "domain, IP or ASN (AS12345)", Required: true},
			},
			Build: func(a Args) Invocation {
				q := strings.TrimSpace(a.S("query"))
				typ := "domain"
				if n, err := strconv.ParseUint(strings.TrimPrefix(strings.ToUpper(q), "AS"), 10, 32); err == nil {
					typ, q = "autnum", strconv.FormatUint(n, 10)
				} else if net.ParseIP(q) != nil {
					typ = "ip"
				} else if !domainRE.MatchString(q) {
					return Invocation{Target: q, Refuse: "query must be a domain, IP or ASN"}
				}
				// Kali ships openrdap as go-rdap. Type is always explicit: auto-detect would fetch a URL-shaped query.
				return Invocation{Argv: []string{"go-rdap", "-j", "-t", typ, q}, Target: q}
			},
		},
		{
			Name: "dnstwist",
			Desc: "Find registered lookalike domains (typosquats, homoglyphs, bitsquats, other TLDs) of a " +
				"DOMAIN with dnstwist, for phishing/BEC and brand-impersonation checks. Resolves the " +
				"permutations through the normal resolver only — no banner grabs, MX/SMTP checks, page " +
				"fetches or screenshots. Prints registered permutations as JSON (A/AAAA/MX/NS). Runs in " +
				"the background (thousands of DNS queries); poll with job_status.",
			Params: []Param{
				{Name: "domain", Type: StringParam, Desc: "domain, e.g. acme.com", Required: true},
			},
			Build: func(a Args) Invocation {
				d := strings.ToLower(strings.TrimSpace(a.S("domain")))
				if !domainRE.MatchString(d) {
					return Invocation{Target: d, Refuse: "domain must be a DNS name like acme.com"}
				}
				// Fixed flags: -b/-m/--lsh/--ssdeep/-p/--screenshots would contact hosts, --nameservers the target.
				argv := []string{"dnstwist", "--registered", "--format", "json", "--threads", "8", d}
				return Invocation{Argv: argv, Target: d, Background: true}
			},
		},
		{
			Name: "ioc_lookup",
			Desc: "Enrich an INDICATOR against keyless threat-intel sources. A file hash (MD5/SHA-1/" +
				"SHA-256) goes to CIRCL hashlookup (known-good/known-file database); an IP to GreyNoise " +
				"community (internet scanner / benign service?) and AlienVault OTX pulses; a domain to OTX " +
				"and urlscan.io's public scan history. Lookups only — nothing is submitted for scanning. " +
				"Note the services see what you query. Each response is capped at 20 kB.",
			Params: []Param{
				{Name: "indicator", Type: StringParam, Desc: "hash, IP or domain", Required: true},
			},
			Build: func(a Args) Invocation {
				ind := strings.ToLower(strings.TrimSpace(a.S("indicator")))
				var cmd string
				switch ip := net.ParseIP(ind); {
				case hashRE.MatchString(ind):
					algo := map[int]string{32: "md5", 40: "sha1", 64: "sha256"}[len(ind)]
					cmd = fetch("CIRCL hashlookup", "https://hashlookup.circl.lu/lookup/"+algo+"/"+ind)
				case ip != nil:
					fam := "IPv4"
					if ip.To4() == nil {
						fam = "IPv6"
					}
					cmd = fetch("GreyNoise community (404 = not seen scanning)", "https://api.greynoise.io/v3/community/"+ind) +
						fetch("AlienVault OTX", "https://otx.alienvault.com/api/v1/indicators/"+fam+"/"+ind+"/general")
				case domainRE.MatchString(ind):
					cmd = fetch("AlienVault OTX", "https://otx.alienvault.com/api/v1/indicators/domain/"+ind+"/general") +
						fetch("urlscan.io search", "https://urlscan.io/api/v1/search/?size=20&q="+url.QueryEscape("domain:"+ind))
				default:
					return Invocation{Target: ind, Refuse: "indicator must be an MD5/SHA-1/SHA-256 hash, an IP or a domain"}
				}
				return Invocation{Argv: []string{"/bin/bash", "-c", cmd}, Target: ind}
			},
		},
	}
}
