package registry

import (
	"testing"

	"github.com/rhaist/marq/internal/config"
)

// TestSkillsOnlyDropsExecTools asserts the knowledge-only mode exposes only
// in-process tools (Handler, no Build) — so the server runs without any Kali
// binary — and that it's a strict subset of the full catalog.
func TestSkillsOnlyDropsExecTools(t *testing.T) {
	full := len(All())

	defer func() { config.C.SkillsOnly = false }()
	config.C.SkillsOnly = true

	lite := All()
	if len(lite) == 0 || len(lite) >= full {
		t.Fatalf("skills-only catalog = %d tools, full = %d; want a non-empty strict subset", len(lite), full)
	}
	for _, tool := range lite {
		if tool.Build != nil {
			t.Errorf("skills-only exposed exec tool %q (has Build) — it needs a Kali binary", tool.Name)
		}
	}
}
