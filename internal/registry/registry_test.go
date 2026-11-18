package registry

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestNoDuplicateToolNames is the core sanity gate: tool names must be unique.
func TestNoDuplicateToolNames(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range All() {
		if seen[tool.Name] {
			t.Errorf("duplicate tool name: %s", tool.Name)
		}
		seen[tool.Name] = true
	}
}

// TestExactlyOneImplementation asserts every tool is either an exec tool (Build)
// or an in-process tool (Handler), never both or neither.
func TestExactlyOneImplementation(t *testing.T) {
	for _, tool := range All() {
		hasBuild := tool.Build != nil
		hasHandler := tool.Handler != nil
		if hasBuild == hasHandler {
			t.Errorf("tool %s must have exactly one of Build/Handler (build=%v handler=%v)",
				tool.Name, hasBuild, hasHandler)
		}
	}
}

// TestSchemaShape asserts each tool's generated input schema is an object and
// that required params have no default (defaults imply optional).
func TestSchemaShape(t *testing.T) {
	for _, tool := range All() {
		schema := tool.InputSchema()
		if schema["type"] != "object" {
			t.Errorf("tool %s schema type is not object", tool.Name)
		}
		for _, p := range tool.Params {
			if p.Required && p.Default != nil {
				t.Errorf("tool %s param %s is required but has a default", tool.Name, p.Name)
			}
		}
	}
}

// TestArgValidation covers the small-model guardrails on Tool.Call: a missing
// required arg errors with the schema, an unknown arg warns but still runs, and
// a clean call does neither.
func TestArgValidation(t *testing.T) {
	tool := Tool{
		Name:    "demo",
		Params:  []Param{{Name: "host", Type: StringParam, Required: true}},
		Handler: func(a Args) string { return "ran:" + a.S("host") },
	}
	if got := tool.Call(map[string]any{}); !strings.Contains(got, "missing required") {
		t.Errorf("missing required not reported: %q", got)
	}
	if got := tool.Call(map[string]any{"host": "x", "hostt": "y"}); !strings.Contains(got, "ignored unknown argument(s): hostt") || !strings.Contains(got, "ran:x") {
		t.Errorf("unknown arg should warn and still run: %q", got)
	}
	if got := tool.Call(map[string]any{"host": "x"}); got != "ran:x" {
		t.Errorf("clean call should not warn: %q", got)
	}
}

func toolNamed(t *testing.T, name string) Tool {
	for _, tool := range All() {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not registered", name)
	return Tool{}
}

// TestAsnmapRouting pins asnmap's input classification: each kind of target
// takes its own branch, and anything else (org names, dig-option lookalikes,
// shell metacharacters) never reaches a lookup.
func TestAsnmapRouting(t *testing.T) {
	asnmap := toolNamed(t, "asnmap")
	for target, want := range map[string]string{
		"AS13335":         "announced-prefixes",
		"as3320":          "resource=AS3320",
		"13335":           "resource=AS13335",
		"1.1.1.1":         "' -v 1.1.1.1'",
		"2606:4700::1111": "' -v 2606:4700::1111'",
		"example.com":     "dig +short A example.com",
		"Acme Corp":       "must be an IP, domain or ASN",
		"@evil.example":   "must be an IP, domain or ASN",
		"-f.x":            "must be an IP, domain or ASN",
		"1.1.1.1; id":     "must be an IP, domain or ASN",
	} {
		cmd := asnmap.Build(Args{"target": target}).Argv[2]
		if !strings.Contains(cmd, want) {
			t.Errorf("asnmap %q: want %q in %q", target, want, cmd)
		}
	}
	// metacharacters in a domain stay inside single quotes
	if cmd := asnmap.Build(Args{"target": "$(id).x"}).Argv[2]; !strings.Contains(cmd, "dig +short A '$(id).x'") {
		t.Errorf("domain not quoted: %q", cmd)
	}
}

// TestPassiveOnly pins the OSINT-only promise in each tool's argv: nothing the
// model passes can turn a lookup into a probe of the target's own infrastructure
// (zone transfer, DNS brute force, credential
// verification). These argv checks are the enforcement.
func TestPassiveOnly(t *testing.T) {
	refused := func(name string, a Args) {
		t.Helper()
		if inv := toolNamed(t, name).Build(a); inv.Refuse == "" {
			t.Errorf("%s %v should be refused, got argv %q", name, a, inv.Argv)
		}
	}
	refused("dns_lookup", Args{"domain": "acme.com", "record_type": "AXFR"})
	refused("dns_lookup", Args{"domain": "@ns1.acme.com", "record_type": "a"})
	refused("trufflehog", Args{"source": "filesystem /etc"})
	refused("trufflehog", Args{"source": "git https://github.com/o/r --no-no-verification"})
	refused("trufflehog", Args{"source": "git https://git.acme.com/o/r"}) // target's own server
	refused("trufflehog", Args{"source": "git file:///work/repo"})
	refused("trufflehog", Args{"source": "gitlab --repo=https://gitlab.com/o/r --endpoint=https://gitlab.acme.com"})
	refused("trufflehog", Args{"source": "github"})
	refused("trufflehog", Args{"source": `git "https://github.com/o/r"`}) // quotes are refused, never unquoted
	refused("exif_metadata", Args{"path": "/etc/passwd"})
	refused("whois_lookup", Args{"target": "-h whois.acme.com acme.com"})
	refused("shodan_host", Args{"ip": "--help"})
	for _, name := range []string{"ct_search", "dnstwist"} {
		refused(name, Args{"domain": "--nameservers=ns1.acme.com"})
		refused(name, Args{"domain": "https://acme.com/x"})
	}
	refused("rdap", Args{"query": "https://rdap.acme.com/x"})
	refused("ioc_lookup", Args{"indicator": "http://acme.com/payload"})
	if argv := toolNamed(t, "dnstwist").Build(Args{"domain": "Acme.com"}).Argv; !slices.Equal(argv, []string{"dnstwist", "--registered", "--format", "json", "--threads", "8", "acme.com"}) {
		t.Errorf("dnstwist argv = %q", argv)
	}
	if argv := toolNamed(t, "rdap").Build(Args{"query": "as13335"}).Argv; !slices.Equal(argv, []string{"go-rdap", "-j", "-t", "autnum", "13335"}) {
		t.Errorf("rdap argv = %q", argv)
	}
	refused("ioc_lookup", Args{"indicator": "10.0.0.5"})
	for ind, want := range map[string]string{
		"D41D8CD98F00B204E9800998ECF8427E": "hashlookup.circl.lu/lookup/md5/d41d8cd98f00b204e9800998ecf8427e",
		"::ffff:1.2.3.4":                   "/indicators/IPv4/1.2.3.4/general",
		"2606:4700::1111":                  "/indicators/IPv6/2606:4700::1111/general",
		"acme.com":                         "q=page.domain%3Aacme.com",
	} {
		if cmd := toolNamed(t, "ioc_lookup").Build(Args{"indicator": ind}).Argv[2]; !strings.Contains(cmd, want) {
			t.Errorf("ioc_lookup %q: want %q in %q", ind, want, cmd)
		}
	}
	if cmd := toolNamed(t, "ct_search").Build(Args{"domain": "acme.com"}).Argv[2]; !strings.Contains(cmd, "q=%25.acme.com&deduplicate=Y&output=json&exclude=expired") {
		t.Errorf("ct_search url: %q", cmd)
	}
	if cmd := toolNamed(t, "ct_search").Build(Args{"domain": "acme.com", "include_expired": true}).Argv[2]; strings.Contains(cmd, "exclude=expired") {
		t.Errorf("ct_search include_expired still excludes: %q", cmd)
	}
	refused("orkl", Args{"report_sha1": "../../etc/passwd"})
	if cmd := toolNamed(t, "orkl").Build(Args{"query": "APT29 & x"}).Argv[2]; !strings.Contains(cmd, "query=APT29+%26+x") {
		t.Errorf("orkl query not escaped: %q", cmd)
	}
	if cmd := toolNamed(t, "orkl").Build(Args{"report_sha1": "46758AA98959FDC1AE34DD3EF0AB1303879CC692"}).Argv[2]; !strings.Contains(cmd, "archive.orkl.eu/46758aa98959fdc1ae34dd3ef0ab1303879cc692.txt") {
		t.Errorf("orkl report url: %q", cmd)
	}
	refused("maigret_username", Args{"username": "--parse=https://acme.com/"})
	for _, ok := range []string{"git https://github.com/acme/verifier", "github --org=acme", "git https://gitlab.com/o/r"} {
		if inv := toolNamed(t, "trufflehog").Build(Args{"source": ok}); inv.Refuse != "" {
			t.Errorf("trufflehog %q refused: %s", ok, inv.Refuse)
		}
	}

	if argv := toolNamed(t, "dns_lookup").Build(Args{"domain": "acme.com", "record_type": "MX"}).Argv; !slices.Equal(argv[4:], []string{"acme.com", "mx"}) {
		t.Errorf("dns_lookup argv = %q", argv)
	}
	if cmd := toolNamed(t, "trufflehog").Build(Args{"source": "github --org=acme"}).Argv[2]; !strings.Contains(cmd, "trufflehog github '--org=acme' --no-verification --no-update --json 2>&1 | python3") {
		t.Errorf("trufflehog verifies secrets: %q", cmd)
	}
	if cmd := toolNamed(t, "maigret_username").Build(Args{"username": "jdoe", "timeout": 30, "top_sites": 500}).Argv[2]; !strings.Contains(cmd, "--no-recursion") {
		t.Errorf("maigret must not re-search usernames it extracts (other people): %q", cmd)
	}
}

// TestShodanWrappers: the key stays out of argv (it reaches curl on stdin), the
// limit is clamped, a failing curl isn't masked by the output cap, and the
// inline formatter handles a result, an API error and a non-JSON reply.
func TestShodanWrappers(t *testing.T) {
	for limit, want := range map[int]string{-1: " 1", 0: " 1", 7: " 7", 500: " 100"} {
		cmd := toolNamed(t, "shodan_search").Build(Args{"query": `org:"x"; rm -rf /`, "limit": limit}).Argv[2]
		if !strings.HasSuffix(cmd, want) {
			t.Errorf("limit %d: command ends %q, want %q", limit, cmd[len(cmd)-6:], want)
		}
		if !strings.Contains(cmd, `'query=org:"x"; rm -rf /'`) || !strings.Contains(cmd, `printf %s "$SHODAN_API_KEY" | curl`) {
			t.Errorf("query must be one quoted argument and the key must go to curl's stdin: %s", cmd)
		}
	}
	if cmd := toolNamed(t, "shodan_host").Build(Args{"ip": "8.8.8.8"}).Argv[2]; !strings.Contains(cmd, "rc=$?") || !strings.HasSuffix(cmd, "exit $rc") || strings.Contains(cmd, "| head") {
		t.Errorf("shodan_host must report curl's exit status and cap output without cutting the pipe: %s", cmd)
	}
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not installed")
	}
	for in, want := range map[string]string{
		`{"total": 2, "matches": [{"ip_str": "1.2.3.4", "port": 443, "org": "Acme", "hostnames": ["a.acme.com"], "product": "nginx"}, {"ip_str": "5.6.7.8", "port": 22}]}`: "total: 2\n1.2.3.4\t443\tAcme\ta.acme.com\tnginx\n",
		`{"error": "Access denied (403 Forbidden)"}`: "shodan: Access denied",
		`<html>401</html>`:                           "unexpected response",
	} {
		c := exec.Command(py, "-c", shodanCompact, "1")
		c.Stdin = strings.NewReader(in)
		out, _ := c.CombinedOutput()
		if !strings.Contains(string(out), want) {
			t.Errorf("formatter(%.30s) = %q, want %q", in, out, want)
		}
	}
}

// TestOutputFormatters runs the inline formatters on sample tool output: the
// trufflehog one must never print a raw secret, the banner filter must keep
// output that has no banner, and OTX/urlscan come back as one line per item.
func TestOutputFormatters(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not installed")
	}
	run := func(argv []string, in string) string {
		c := exec.Command(argv[0], argv[1:]...)
		c.Stdin = strings.NewReader(in)
		out, _ := c.CombinedOutput()
		return string(out)
	}
	truffle := `{"SourceMetadata":{"Data":{"Git":{"commit":"f832872a3b1ce389","file":"keys","line":3,"repository":"https://github.com/o/r"}}},"DetectorName":"AWS","Raw":"AKIA2UC3BSXMLSCLTUUS","RawV2":"AKIA2UC3BSXMLSCLTUUS:y1ZZUiLI0krfqmUIZ","Redacted":"AKIA2UC3BSXMLSCLTUUS","ExtraData":{"is_canary":"true"},"SecretParts":{"secret_access_key":"y1ZZUiLI0krfqmUIZ"}}
{"SourceMetadata":{"Data":{"Git":{"file":"k"}}},"DetectorName":"Slack","Raw":"xoxb-1234567890-secret","Redacted":"xoxb-1234567890-secret"}
{"DetectorName":"URI","Raw":"hunter2","Redacted":""}
{"level":"info-0","msg":"scanning repo"}
{"level":"info-0","msg":"finished scanning","chunks":9,"unverified_secrets":2}
{"level":"error","msg":"failed to clone","error":"repository not found"}`
	out := run([]string{py, "-I", "-c", truffleCompact}, truffle)
	for _, want := range []string{"AWS | https://github.com/o/r | keys:3 | commit f832872a3b1c", "canary token", "xoxb...(redacted, 22 chars)", "2 unverified results", "repository not found"} {
		if !strings.Contains(out, want) {
			t.Errorf("trufflehog formatter: want %q in\n%s", want, out)
		}
	}
	for _, leak := range []string{"y1ZZUiLI0krfqmUIZ", "xoxb-1234567890-secret", "hunter", "scanning repo"} {
		if strings.Contains(out, leak) {
			t.Errorf("trufflehog formatter printed %q:\n%s", leak, out)
		}
	}

	awk := []string{"/bin/sh", "-c", fromLine("Checking email", `\[~\]|partner=`)}
	if out := run(awk, "ART\nVersion 1\n Checking email: a@b.c\n  [~] skipped\n  [+] hit partner=x\n  [✔] Site: Registered\n"); out != " Checking email: a@b.c\n  [✔] Site: Registered\n" {
		t.Errorf("banner filter = %q", out)
	}
	if out := run(awk, "error: no such option\n"); out != "error: no such option\n" {
		t.Errorf("banner filter must keep output with no start line: %q", out)
	}

	otx := `{"validation":[{"message":"Whitelisted domain"}],"pulse_info":{"count":1,"pulses":[{"name":"Phish kit","modified":"2026-11-18T10:00:00","description":"LONG FREE TEXT","tags":["a","b"],"malware_families":[{"display_name":"Emotet"}],"TLP":"green"}]}}`
	if out := run([]string{py, "-I", "-c", otxCompact}, otx); !strings.Contains(out, "pulses: 1") || !strings.Contains(out, "2026-11-18 | Phish kit | adversary - | malware Emotet | TLP green") || strings.Contains(out, "LONG FREE TEXT") {
		t.Errorf("otx formatter = %q", out)
	}
	scan := `{"total":1,"results":[{"task":{"time":"2026-11-18T14:36:20Z","url":"https://acme.com/"},"page":{"url":"https://acme.com/","ip":"1.2.3.4","asnname":"ACME","title":"Acme","status":"200"},"result":"https://urlscan.io/api/v1/result/x/"}]}`
	if out := run([]string{py, "-I", "-c", urlscanCompact}, scan); !strings.Contains(out, "2026-11-18 | https://acme.com/ | 1.2.3.4 | ACME | Acme | HTTP 200 | https://urlscan.io/api/v1/result/x/") {
		t.Errorf("urlscan formatter = %q", out)
	}
	if out := run([]string{py, "-I", "-c", otxCompact}, "<html>502</html>"); !strings.Contains(out, "OTX returned no JSON") {
		t.Errorf("otx formatter on an error page = %q", out)
	}
	if out := run([]string{py, "-I", "-c", otxCompact}, `{"detail":"rate limited"}`); !strings.Contains(out, "no pulse data") || strings.Contains(out, "pulses: 0") {
		t.Errorf("otx formatter must flag an error body, not report 0 pulses: %q", out)
	}
	if out := run([]string{py, "-I", "-c", urlscanCompact}, `{"message":"Quota exceeded"}`); !strings.Contains(out, "Quota exceeded") || strings.Contains(out, "scans: 0") {
		t.Errorf("urlscan formatter must flag an error body: %q", out)
	}
}
