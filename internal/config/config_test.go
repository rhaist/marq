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
	if got := Load(); got.Banner() == "" || got.ScopeNote != "" {
		t.Fatalf("empty scope should stay empty (banner adds the placeholder): %+v", got)
	}
}
