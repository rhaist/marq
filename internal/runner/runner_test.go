package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rhaist/marq/internal/config"
)

// TestFailClosedOnAuditError pins the core safety invariant: if the audit start
// record can't be persisted, Run must REFUSE to execute the tool (exit 126) and
// never spawn the binary. A regression to `id, _ := audit.LogStart(...)` would
// silently run tools unlogged — this test fails loudly if that happens.
func TestFailClosedOnAuditError(t *testing.T) {
	// Force the audit write to fail: put a regular FILE where the log's parent
	// dir would be, so audit.write's MkdirAll(dir) fails with ENOTDIR.
	blocker := filepath.Join(t.TempDir(), "iamafile")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := config.C.AuditLog
	config.C.AuditLog = filepath.Join(blocker, "audit.log")
	t.Cleanup(func() { config.C.AuditLog = orig })

	// Use a sentinel file the tool would create if it actually ran.
	marker := filepath.Join(t.TempDir(), "ran")
	res := Run("test", []string{"/bin/sh", "-c", "touch " + marker}, Opts{})

	if res.ExitCode == nil || *res.ExitCode != 126 {
		got := "nil"
		if res.ExitCode != nil {
			got = strconv.Itoa(*res.ExitCode)
		}
		t.Fatalf("want exit 126 (fail-closed), got %s (stderr: %q)", got, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "refusing to run unlogged") {
		t.Errorf("want fail-closed message, got %q", res.Stderr)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("tool executed despite audit failure — fail-closed invariant VIOLATED")
	}
}
