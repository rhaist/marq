package config

import "testing"

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
// per-call `marq run` path) reads the same engagement back.
func TestSetEngagementRoundTrip(t *testing.T) {
	dir := t.TempDir()
	C = Config{WorkDir: dir, Operator: "alice", Engagement: "unspecified"}

	if err := SetEngagement("acme-2026"); err != nil {
		t.Fatalf("SetEngagement: %v", err)
	}
	if C.Engagement != "acme-2026" || readContext(dir) != "acme-2026" {
		t.Fatalf("not persisted: live=%q file=%q", C.Engagement, readContext(dir))
	}
}
