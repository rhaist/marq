package config

import "testing"

// TestLoadEnv verifies env vars drive the per-engagement fields and that an
// unset scope falls back to the placeholder shown in the banner.
func TestLoadEnv(t *testing.T) {
	t.Setenv("MARQ_OPERATOR", "alice")
	t.Setenv("MARQ_ENGAGEMENT", "acme-2026")
	t.Setenv("MARQ_SCOPE", "*.example.com")

	c := Load()
	if c.Operator != "alice" || c.Engagement != "acme-2026" || c.ScopeNote != "*.example.com" {
		t.Fatalf("env not applied: %+v", c)
	}

	t.Setenv("MARQ_SCOPE", "")
	t.Setenv("MARQ_WORK_DIR", t.TempDir()) // isolate from any real /work/.marq-context
	if got := Load(); got.Banner() == "" || got.ScopeNote != "" {
		t.Fatalf("empty scope should stay empty (banner adds the placeholder): %+v", got)
	}
}

// SetEngagement must update the live config and persist so a fresh Load (the
// per-call `marq run` path) reads the same engagement/scope back.
func TestSetEngagementRoundTrip(t *testing.T) {
	dir := t.TempDir()
	C = Config{WorkDir: dir, Operator: "alice", Engagement: "unspecified"}

	if err := SetEngagement("acme-2026", "*.acme.com"); err != nil {
		t.Fatalf("SetEngagement: %v", err)
	}
	if C.Engagement != "acme-2026" || C.ScopeNote != "*.acme.com" {
		t.Fatalf("live config not updated: %+v", C)
	}
	if e, s, ok := readContext(dir); !ok || e != "acme-2026" || s != "*.acme.com" {
		t.Fatalf("readContext = %q,%q,%v; want acme-2026,*.acme.com,true", e, s, ok)
	}

	// Empty engagement keeps the prior label; scope still updates.
	if err := SetEngagement("", "10.0.0.0/8"); err != nil {
		t.Fatalf("SetEngagement: %v", err)
	}
	if C.Engagement != "acme-2026" || C.ScopeNote != "10.0.0.0/8" {
		t.Fatalf("partial update wrong: %+v", C)
	}
}
