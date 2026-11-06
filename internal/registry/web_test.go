package registry

import (
	"slices"
	"testing"
)

// nuclei's `tags` arg must reach the CLI as `-tags`; `templates` stays `-t`.
// Regression guard for the wrapper fix — a model that passes tags should not
// have them silently dropped or mapped onto -t.
func TestNucleiTagsFlag(t *testing.T) {
	var nuclei Tool
	for _, tool := range web() {
		if tool.Name == "nuclei" {
			nuclei = tool
		}
	}
	if nuclei.Build == nil {
		t.Fatal("nuclei tool not found")
	}
	argv := nuclei.Build(Resolve(nuclei.Params, map[string]any{
		"target":    "https://example.com",
		"tags":      "cve,takeover",
		"templates": "http/takeovers/",
	})).Argv

	for _, want := range [][2]string{{"-tags", "cve,takeover"}, {"-t", "http/takeovers/"}} {
		i := slices.Index(argv, want[0])
		if i < 0 || i+1 >= len(argv) || argv[i+1] != want[1] {
			t.Errorf("argv %v missing %s %s", argv, want[0], want[1])
		}
	}
}
