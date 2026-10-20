package registry

import (
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
