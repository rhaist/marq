package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDirectOsExec enforces the one architectural invariant that matters:
// every external command must funnel through internal/runner (the single choke
// point for audit, timeout, and truncation). Only that package may import
// os/exec; anything else shelling out directly bypasses all three guarantees.
// This walks the internal/ tree and fails if the invariant is ever broken.
func TestNoDirectOsExec(t *testing.T) {
	needle := `"os/` + `exec"` // split so this test file isn't a self-match
	err := filepath.WalkDir("..", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		if strings.Contains(filepath.ToSlash(p), "/runner/") {
			return nil // the single allowed home for os/exec
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(b), needle) {
			t.Errorf("%s imports os/exec directly — route it through internal/runner instead", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
