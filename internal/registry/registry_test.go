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
