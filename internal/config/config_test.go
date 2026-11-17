package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadEnv verifies env vars drive the per-engagement fields.
func TestLoadEnv(t *testing.T) {
	t.Setenv("MARQ_OPERATOR", "alice")
	t.Setenv("MARQ_ENGAGEMENT", "acme-2026")
	t.Setenv("MARQ_WORK_DIR", t.TempDir()) // isolate from any real /work/.marq-context

	c := Load()
	if c.Operator != "alice" || c.Engagement != "acme-2026" || c.Banner() == "" {
		t.Fatalf("env not applied: %+v", c)
	}
}

// SetEngagement must update the live config and persist so a fresh Load (the
// per-call `marq run` path) reads the same engagement back — including from a
// context file written by the older scope-gated build.
func TestSetEngagementRoundTrip(t *testing.T) {
	dir := t.TempDir()
	C = Config{WorkDir: dir, Operator: "alice", Engagement: "unspecified"}

	if err := SetEngagement("acme-2026"); err != nil {
		t.Fatalf("SetEngagement: %v", err)
	}
	if C.Engagement != "acme-2026" || readContext(dir) != "acme-2026" {
		t.Fatalf("not persisted: live=%q file=%q", C.Engagement, readContext(dir))
	}

	legacy := `{"engagement":"old-2025","scope":"*.acme.com"}`
	if err := os.WriteFile(filepath.Join(dir, ".marq-context"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readContext(dir); got != "old-2025" {
		t.Fatalf("legacy context = %q; want old-2025", got)
	}
}
