package files

import (
	"fmt"
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
	// Keep audit happy so audited() doesn't fail-closed before Resolve() runs.
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
	out := ReadFile(filepath.Join(root, "escape", "secret"), 0, 0)
	if strings.Contains(out, "TOPSECRET") {
		t.Fatal("symlink escape read a file outside the sandbox — Resolve() failed")
	}
	if !strings.Contains(out, "outside the allowed area") {
		t.Errorf("want sandbox-refusal message, got %q", out)
	}

	// (2) `..` traversal must be refused
	if out := ReadFile(filepath.Join(root, "..", "etc-ish"), 0, 0); !strings.Contains(out, "outside the allowed area") {
		// a non-existent target is fine; what matters is it never resolves outside
		if !strings.Contains(out, "not a file") {
			t.Errorf("`..` traversal not contained: %q", out)
		}
	}

	// (3) a legit file directly inside the sandbox still works
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("HELLO"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := ReadFile(filepath.Join(root, "ok.txt"), 0, 0); !strings.Contains(out, "HELLO") {
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
	if out := ReadFile(filepath.Join(work, "findings.md"), 0, 0); !strings.Contains(out, "REPORT") {
		t.Errorf("read inside MARQ_WORK_DIR refused: %q", out)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(work, "escape")); err != nil {
		t.Fatal(err)
	}
	if out := ReadFile(filepath.Join(work, "escape", "secret"), 0, 0); strings.Contains(out, "TOPSECRET") {
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
		if out := ReadFile(filepath.Join(home, "secret"), 0, 0); strings.Contains(out, "HOMESECRET") {
			t.Errorf("MARQ_WORK_DIR=%s exposed $HOME", wd)
		}
	}
}

// TestSandboxRefusesAuditLog: the audit log may sit inside MARQ_WORK_DIR (bare
// install), but write_file/read_file must never touch it.
func TestSandboxRefusesAuditLog(t *testing.T) {
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	origRoots, origWork, origLog := allowedRoots, config.C.WorkDir, config.C.AuditLog
	allowedRoots, config.C.WorkDir, config.C.AuditLog = []string{"/nonexistent-root"}, work, filepath.Join(work, "audit.jsonl")
	t.Cleanup(func() { allowedRoots, config.C.WorkDir, config.C.AuditLog = origRoots, origWork, origLog })
	// audited() writes the start record before Resolve(), so the log always exists.
	if err := os.WriteFile(config.C.AuditLog, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Resolve(filepath.Join(work, "audit.jsonl")); err == nil {
		t.Fatal("audit log path was allowed")
	}
	if err := os.Symlink(filepath.Join(work, "audit.jsonl"), filepath.Join(work, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(filepath.Join(work, "alias")); err == nil {
		t.Fatal("symlink to the audit log was allowed")
	}
	if _, err := Resolve(filepath.Join(work, "findings.md")); err != nil {
		t.Errorf("ordinary work file refused: %v", err)
	}
}

// TestReadFileOffset: a file over the budget is read in parts, and the cut says
// where to continue; MARQ_WORK_DIR equal to a fixed root isn't listed twice.
func TestReadFileOffset(t *testing.T) {
	config.C.AuditLog = filepath.Join(t.TempDir(), "audit.log")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	orig, origDir, origMax := allowedRoots, config.C.WorkDir, config.C.MaxOutputChars
	t.Cleanup(func() { allowedRoots, config.C.WorkDir, config.C.MaxOutputChars = orig, origDir, origMax })
	allowedRoots, config.C.WorkDir, config.C.MaxOutputChars = []string{root}, root, 10

	if n := len(roots()); n != 1 {
		t.Errorf("roots() = %q, want the work dir once", roots())
	}
	p := filepath.Join(root, "big.txt")
	if err := os.WriteFile(p, []byte("0123456789abcdefghijKLMN"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := ReadFile(p, 0, 0); !strings.Contains(out, "0123456789") || !strings.Contains(out, "offset=10") || strings.Contains(out, "abc") {
		t.Errorf("first part: %q", out)
	}
	if out := ReadFile(p, 0, 20); !strings.Contains(out, "KLMN") || strings.Contains(out, "truncated") {
		t.Errorf("last part: %q", out)
	}
	if out := ReadFile(p, 0, 99); !strings.Contains(out, "past the end") {
		t.Errorf("offset past the end: %q", out)
	}
	// "ééééé…" in 10-byte parts: 2-byte runes must never straddle a cut.
	u := filepath.Join(root, "utf8.txt")
	if err := os.WriteFile(u, []byte("aéééééééééé"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for off, n := 0, 0; n < 5; n++ {
		out := ReadFile(u, 0, off)
		body := out[strings.Index(out, "\n")+1:]
		body = body[:strings.Index(body, "\n[untrusted-data")]
		if i := strings.Index(body, "\n…[truncated: continue with offset="); i >= 0 {
			fmt.Sscanf(body[i:], "\n…[truncated: continue with offset=%d", &off)
			got.WriteString(body[:i])
			continue
		}
		got.WriteString(body)
		break
	}
	if got.String() != "aéééééééééé" {
		t.Errorf("paged read = %q, want the file back intact", got.String())
	}
}
