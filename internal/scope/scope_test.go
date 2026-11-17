package scope

import (
	"errors"
	"slices"
	"testing"
)

func TestParse(t *testing.T) {
	s := Parse("*.acme.com, 203.0.113.0/24, app.example.org 198.51.100.7 — per SOW; loopback")
	if len(s.Domains) != 1 || s.Domains[0] != "acme.com" {
		t.Errorf("domains: %v", s.Domains)
	}
	if !s.Hosts["app.example.org"] || !s.Hosts["localhost"] {
		t.Errorf("hosts: %v", s.Hosts)
	}
	if len(s.Prefixes) != 4 { // /24, /32, 127/8, ::1
		t.Errorf("prefixes: %v", s.Prefixes)
	}
	if Parse("per the SOW, verbally agreed").Empty() == false {
		t.Error("prose-only note should parse as empty")
	}
}

func TestAllows(t *testing.T) {
	orig := lookup
	lookup = func(h string) ([]string, error) {
		if h == "in.example.net" {
			return []string{"203.0.113.9"}, nil
		}
		if h == "mixed.example.net" {
			return []string{"203.0.113.9", "8.8.8.8"}, nil
		}
		return nil, errors.New("nxdomain")
	}
	t.Cleanup(func() { lookup = orig })
	s := Parse("*.acme.com, 203.0.113.0/24, app.example.org, loopback")
	for target, want := range map[string]bool{
		"acme.com": true, "www.acme.com": true, "a.b.acme.com": true, "notacme.com": false,
		"app.example.org": true, "x.example.org": false,
		"203.0.113.50": true, "203.0.114.1": false, "203.0.113.0/25": true, "203.0.112.0/23": false,
		"127.0.0.1": true, "::1": true, "localhost": true,
		"in.example.net": true, "mixed.example.net": false, "nx.example.net": false,
	} {
		if got := s.Allows(target); got != want {
			t.Errorf("Allows(%q) = %v, want %v", target, got, want)
		}
	}
}

func TestTargets(t *testing.T) {
	got := Targets(
		"nmap -sV -p 80,443 10.0.0.5 scanme.nmap.org",
		"--url=https://www.acme.com/login?x=1",
		"test/test:test@192.0.2.10",
		"-w /usr/share/wordlists/dirb/common.txt -o out.json rules.yar",
		"203.0.113.0/24",
		"(raw shell)",
		"[2001:db8::1]:8443",
		"1.2.3",
	)
	want := []string{"10.0.0.5", "scanme.nmap.org", "www.acme.com", "192.0.2.10", "203.0.113.0/24", "2001:db8::1"}
	if !slices.Equal(got, want) {
		t.Errorf("Targets = %v\nwant      %v", got, want)
	}
}

func TestOutOfScope(t *testing.T) {
	s := Parse("*.acme.com, loopback")
	got := s.OutOfScope("nuclei -u https://www.acme.com -u https://evil.example", "127.0.0.1", "evil.example")
	if !slices.Equal(got, []string{"evil.example"}) {
		t.Errorf("OutOfScope = %v", got)
	}
}

func TestAdded(t *testing.T) {
	got := Added("*.acme.com, 10.0.0.0/8", "*.acme.com, 10.0.0.0/8, 192.0.2.0/24, *.acme.org — widened per SOW v2")
	if !slices.Equal(got, []string{"*.acme.org", "192.0.2.0/24"}) {
		t.Errorf("Added = %v", got)
	}
	if got := Added("*.acme.com", "*.acme.com — note only"); len(got) != 0 {
		t.Errorf("rewording only should add nothing: %v", got)
	}
}

// TestReviewBypasses: each command must yield an out-of-scope target under
// scope 10.0.0.0/24 (fail closed), per the 2026-10 code review.
func TestReviewBypasses(t *testing.T) {
	orig := lookup
	lookup = func(string) ([]string, error) { return nil, errors.New("nxdomain") }
	t.Cleanup(func() { lookup = orig })
	s := Parse("10.0.0.0/24")
	for _, cmd := range []string{
		"curl evil.com/index.html",            // schemeless host/path
		"smbclient //evil.com/share",          // UNC
		`smbclient \\evil.com\share`,          // UNC, backslashes
		"curl http://evil.com/%zz",            // malformed URL
		"curl http://8.8.8.8:80abc/",          // bad port
		"nmap 10.0.0.1-254 8.8.8.1-5",         // nmap octet range
		"masscan 1.0.0.0-1.255.255.255",       // masscan range
		"nmap 10.0.*.1",                       // wildcard octet beyond /24
		"curl http://0x08080808/",             // hex IP
		"curl http://134744072/",              // decimal IP
		"curl http://пример.рф/",              // IDN
		"sqlmap -uhttp://evil.com --batch",    // glued short option
		"curl -xhttp://evil-proxy.net:8080 x", // glued proxy
		"curl https://evil.sh/",               // TLD that is also a file extension
		"nuclei -u https://x.zip",             // ditto
		"--target=8.8.8.8",                    // glued IP
	} {
		if oos := s.OutOfScope(cmd); len(oos) == 0 {
			t.Errorf("%q: no out-of-scope target found (targets %v)", cmd, Targets(cmd))
		}
	}
	if oos := s.OutOfScope("nmap 10.0.0.1-254", "masscan 10.0.0.0-10.0.0.255"); len(oos) != 0 {
		t.Errorf("in-scope ranges refused: %v", oos)
	}
}

// TestNoFalseTargets: wordlists, output files, flags, ports, AD-style passwords
// and file paths are not targets.
func TestNoFalseTargets(t *testing.T) {
	for _, cmd := range []string{
		"-w /usr/share/wordlists/dirb/common.txt -o res.jsonl rules.yar web.config",
		"-p 80,443,8000-8100 --rate 1000 -T4 --script=http-title",
		"/work/acme.com-report.md ./scan.sh ../out/evil.com.txt ~/x.json",
		"--severity critical,high -timeout 10 1.5",
	} {
		if got := Targets(cmd); len(got) != 0 {
			t.Errorf("%q: unexpected targets %v", cmd, got)
		}
	}
}

func TestExclusions(t *testing.T) {
	s := Parse("*.acme.com except vpn.acme.com; 10.0.0.0/24 !10.0.0.5; (*.acme.org)")
	for target, want := range map[string]bool{
		"www.acme.com": true, "vpn.acme.com": false, "x.acme.org": true,
		"10.0.0.4": true, "10.0.0.5": false, "10.0.0.1-254": false, "10.0.0.6-10.0.0.9": true,
	} {
		if got := s.Allows(target); got != want {
			t.Errorf("Allows(%q) = %v, want %v", target, got, want)
		}
	}
	if !slices.Equal(Added("", "*.acme.com except vpn.acme.com"), []string{"*.acme.com"}) {
		t.Errorf("exclusions must not count as added scope")
	}
}
