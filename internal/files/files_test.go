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

// TestSandboxAllowsWorkDir: a bare install sets MARQ_WORK_DIR outside /work, and
// render_report writes there — read_file must reach it, but a symlink out of it
// must still be refused.
func TestSandboxAllowsWorkDir(t *testing.T) {
	config.C.AuditLog = filepath.Join(t.TempDir(), "audit.log")
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	origRoots, origWork := allowedRoots, config.C.WorkDir
	allowedRoots, config.C.WorkDir = []string{"/nonexistent-root"}, work
	t.Cleanup(func() { allowedRoots, config.C.WorkDir = origRoots, origWork })

	if err := os.WriteFile(filepath.Join(work, "findings.md"), []byte("REPORT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := ReadFile(filepath.Join(work, "findings.md"), 0); !strings.Contains(out, "REPORT") {
		t.Errorf("read inside MARQ_WORK_DIR refused: %q", out)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(work, "escape")); err != nil {
		t.Fatal(err)
	}
	if out := ReadFile(filepath.Join(work, "escape", "secret"), 0); strings.Contains(out, "TOPSECRET") {
		t.Fatal("symlink escape out of MARQ_WORK_DIR read a file outside the sandbox")
	}
	// a WorkDir of / or one containing $HOME must not widen the sandbox
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, "secret"), []byte("HOMESECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, wd := range []string{"/", home, filepath.Dir(home)} {
		config.C.WorkDir = wd
		if out := ReadFile(filepath.Join(home, "secret"), 0); strings.Contains(out, "HOMESECRET") {
			t.Errorf("MARQ_WORK_DIR=%s exposed $HOME", wd)
		}
	}
}
