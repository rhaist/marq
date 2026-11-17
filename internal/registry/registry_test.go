package registry

import (
	"slices"
	"strings"
	"testing"
)

// TestNoDuplicateToolNames is the project's core sanity gate (ported from the
// Python "register against a stub, assert no duplicate names" check).
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

// TestCensysAssetsAfterDoubleDash: a model-supplied "-i /path" must be an
// asset, not cencli's --input-file (which would upload the file to Censys).
func TestCensysAssetsAfterDoubleDash(t *testing.T) {
	cmd := toolNamed(t, "censys_view").Build(Args{"assets": "-i /work/creds.txt"}).Argv[2]
	if !strings.HasSuffix(cmd, "-- '-i /work/creds.txt'") {
		t.Errorf("assets not after --: %q", cmd)
	}
}

// TestPassiveOnly pins the OSINT-only promise in each tool's argv: nothing the
// model passes can turn a lookup into a probe of the target's own infrastructure
// (zone transfer, DNS brute force, active spiderfoot modules, credential
// verification). With the scope gate gone, these argv checks are the enforcement.
func TestPassiveOnly(t *testing.T) {
	refused := func(name string, a Args) {
		t.Helper()
		if inv := toolNamed(t, name).Build(a); inv.Refuse == "" {
			t.Errorf("%s %v should be refused, got argv %q", name, a, inv.Argv)
		}
	}
	refused("dns_lookup", Args{"domain": "acme.com", "record_type": "AXFR"})
	refused("dns_lookup", Args{"domain": "@ns1.acme.com", "record_type": "a"})
	refused("dnsx", Args{"hosts": "acme.com", "records": "a,w"}) // -w = wordlist brute force
	refused("trufflehog", Args{"source": "filesystem /etc"})
	refused("trufflehog", Args{"source": "git https://github.com/o/r --no-no-verification"})
	refused("trufflehog", Args{"source": "git https://git.acme.com/o/r"}) // target's own server
	refused("trufflehog", Args{"source": "git file:///work/repo"})
	refused("trufflehog", Args{"source": "gitlab --repo=https://gitlab.com/o/r --endpoint=https://gitlab.acme.com"})
	refused("trufflehog", Args{"source": "github"})
	refused("maigret_username", Args{"username": "--parse=https://acme.com/"})
	refused("sherlock", Args{"username": "--site"})
	refused("h8mail_breach", Args{"email": "a@acme.com", "options": "-u https://acme.com"})
	refused("h8mail_breach", Args{"email": "a@acme.com", "options": "-bc /etc --loose"})
	if inv := toolNamed(t, "h8mail_breach").Build(Args{"email": "a@acme.com", "options": "-bc /work/breach --loose -sk"}); inv.Refuse != "" {
		t.Errorf("h8mail local compilation refused: %s", inv.Refuse)
	}
	for _, ok := range []string{"git https://github.com/acme/verifier", "github --org=acme", "git https://gitlab.com/o/r"} {
		if inv := toolNamed(t, "trufflehog").Build(Args{"source": ok}); inv.Refuse != "" {
			t.Errorf("trufflehog %q refused: %s", ok, inv.Refuse)
		}
	}

	if argv := toolNamed(t, "dns_lookup").Build(Args{"domain": "acme.com", "record_type": "MX"}).Argv; !slices.Equal(argv[4:], []string{"acme.com", "mx"}) {
		t.Errorf("dns_lookup argv = %q", argv)
	}
	if argv := toolNamed(t, "spiderfoot").Build(Args{"target": "acme.com"}).Argv; !slices.Contains(argv, "passive") {
		t.Errorf("spiderfoot not forced passive: %q", argv)
	}
	if argv := toolNamed(t, "trufflehog").Build(Args{"source": "github --org=acme"}).Argv; !slices.Contains(argv, "--no-verification") {
		t.Errorf("trufflehog verifies secrets: %q", argv)
	}
}
