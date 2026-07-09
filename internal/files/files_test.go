package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhaist/marq/internal/config"
)

// TestSandboxRejectsSymlinkEscape exercises the whole reason realpath() exists:
// a symlink INSIDE an allowed root that points OUT must not let a read/write
// escape the sandbox. A naive HasPrefix(abs, root) check (no symlink resolution)
// would pass the e2e absolute-path test but fail this one.
func TestSandboxRejectsSymlinkEscape(t *testing.T) {
	// Keep audit happy so audited() doesn't fail-closed before resolve() runs.
	config.C.AuditLog = filepath.Join(t.TempDir(), "audit.log")

	// EvalSymlinks so the roots match what realpath() produces (macOS /tmp etc).
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	orig := allowedRoots
	allowedRoots = []string{root}
	t.Cleanup(func() { allowedRoots = orig })

	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	// symlink inside the sandbox -> outside
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}

	// (1) read through the escaping symlink must be refused
	out := ReadFile(filepath.Join(root, "escape", "secret"), 0)
	if strings.Contains(out, "TOPSECRET") {
		t.Fatal("symlink escape read a file outside the sandbox — resolve() failed")
	}
	if !strings.Contains(out, "outside the allowed area") {
		t.Errorf("want sandbox-refusal message, got %q", out)
	}

	// (2) `..` traversal must be refused
	if out := ReadFile(filepath.Join(root, "..", "etc-ish"), 0); !strings.Contains(out, "outside the allowed area") {
		// a non-existent target is fine; what matters is it never resolves outside
		if !strings.Contains(out, "not a file") {
			t.Errorf("`..` traversal not contained: %q", out)
		}
	}

	// (3) a legit file directly inside the sandbox still works
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("HELLO"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := ReadFile(filepath.Join(root, "ok.txt"), 0); !strings.Contains(out, "HELLO") {
		t.Errorf("legit in-sandbox read failed: %q", out)
	}
}
