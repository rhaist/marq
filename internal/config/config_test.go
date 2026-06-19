package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTUIConfigRoundTrip verifies the persist -> reload path used by the TUI
// setup screen: saved values come back, and an explicit env var still wins.
func TestTUIConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tui.json")
	t.Setenv("MARQ_TUI_CONFIG", path)

	for _, k := range []string{"MARQ_OPERATOR", "MARQ_ENGAGEMENT", "MARQ_SCOPE",
		"MARQ_MODEL_URL", "MARQ_MODEL", "MARQ_MODEL_KEY"} {
		os.Unsetenv(k)
	}

	if TUIConfigExists(path) {
		t.Fatal("file should not exist yet")
	}

	c := Load()
	c.Operator = "alice"
	c.Engagement = "acme-2026"
	c.ScopeNote = "*.example.com"
	c.ModelBaseURL = "http://host.docker.internal:1234/v1"
	c.ModelName = "qwen2.5-7b"
	c.ModelAPIKey = "lm-studio"
	if err := SaveTUIConfig(c); err != nil {
		t.Fatalf("SaveTUIConfig: %v", err)
	}
	if !TUIConfigExists(path) {
		t.Fatal("file should exist after save")
	}

	c2 := Load()
	if c2.Operator != "alice" || c2.ModelBaseURL != "http://host.docker.internal:1234/v1" ||
		c2.ModelName != "qwen2.5-7b" || c2.ModelAPIKey != "lm-studio" ||
		c2.ScopeNote != "*.example.com" || c2.Engagement != "acme-2026" {
		t.Fatalf("reload did not restore saved values: %+v", c2)
	}

	// Env var must override the file.
	t.Setenv("MARQ_OPERATOR", "bob")
	c3 := Load()
	if c3.Operator != "bob" {
		t.Fatalf("env should override file: got operator=%q want %q", c3.Operator, "bob")
	}
	// Unrelated field still comes from the file.
	if c3.ModelName != "qwen2.5-7b" {
		t.Fatalf("model should still come from file: got %q", c3.ModelName)
	}
}

// TestTUIConfigMissingIgnored verifies a missing or malformed file is silently
// ignored (the TUI just re-runs setup) rather than panicking.
func TestTUIConfigMissingIgnored(t *testing.T) {
	if loadTUIFile("/no/such/path/tui.json") != nil {
		t.Fatal("missing file should return nil")
	}
	if loadTUIFile("/dev/null") != nil {
		t.Fatal("empty/unreadable file should return nil")
	}
}
