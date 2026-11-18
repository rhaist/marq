package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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

// TestTruncatedOutputIsSaved: output over the budget is cut for the model but
// kept whole under WorkDir/out, and Render points at that file; the echoed
// command is the first line only, capped.
func TestTruncatedOutputIsSaved(t *testing.T) {
	origLog, origDir, origMax := config.C.AuditLog, config.C.WorkDir, config.C.MaxOutputChars
	t.Cleanup(func() { config.C.AuditLog, config.C.WorkDir, config.C.MaxOutputChars = origLog, origDir, origMax })
	config.C.AuditLog = filepath.Join(t.TempDir(), "audit.log")
	config.C.WorkDir = t.TempDir()
	config.C.MaxOutputChars = 1000

	res := Run("big", []string{"/bin/sh", "-c", "seq 1 5000 # " + strings.Repeat("x", 300) + "\necho done"}, Opts{})
	if !res.Truncated || res.FullOut == "" {
		t.Fatalf("want a truncated result with a saved file, got truncated=%v file=%q", res.Truncated, res.FullOut)
	}
	data, err := os.ReadFile(res.FullOut)
	if err != nil || !strings.HasSuffix(string(data), "\n5000\ndone\n") {
		t.Fatalf("saved file must hold the full output (err %v, %d bytes)", err, len(data))
	}
	out := res.Render()
	if !strings.Contains(out, res.FullOut) || !strings.Contains(out, "offset") || !strings.Contains(out, "capped at 500 chars each") {
		t.Errorf("Render must name the saved file and how to page it: %s", out[len(out)-200:])
	}
	if first := strings.SplitN(out, "\n", 2)[0]; len(first) > 170 || !strings.HasSuffix(first, " …") || strings.Contains(first, "echo done") {
		t.Errorf("command echo must be the capped first line: %q", first)
	}
}

// TestTimeoutKillsPipeline: a `bash -c 'tool | filter'` wrapper that times out
// must not hang on the orphaned tool still holding the output pipe.
func TestTimeoutKillsPipeline(t *testing.T) {
	origLog, origT := config.C.AuditLog, config.C.CommandTimeout
	t.Cleanup(func() { config.C.AuditLog, config.C.CommandTimeout = origLog, origT })
	config.C.AuditLog = filepath.Join(t.TempDir(), "audit.log")
	config.C.CommandTimeout = 1

	start := time.Now()
	res := Run("slow", []string{"/bin/sh", "-c", "sleep 30 | cat"}, Opts{})
	if !res.TimedOut || time.Since(start) > 10*time.Second {
		t.Fatalf("want a timeout within seconds, got timedOut=%v after %s", res.TimedOut, time.Since(start))
	}
}
