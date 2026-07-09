// Package files provides sandboxed file access for the engagement working area.
// The MCP transport carries only strings, so these ops let the model stage tool
// inputs and read back tool outputs — confined to /work and /tmp so it can never
// read or clobber the rest of the container. Every op is audit-logged through
// the same audit module as tool runs.
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rhaist/marq/internal/audit"
	"github.com/rhaist/marq/internal/config"
)

// allowedRoots are the only directories the model may touch.
var allowedRoots = []string{"/work", "/tmp"}

// realpath resolves symlinks for the longest existing prefix of p, then appends
// the remainder — mirroring Python os.path.realpath, which works even when p (or
// its tail) does not exist yet (needed by write_file).
func realpath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	dir := filepath.Dir(p)
	if dir == p {
		return p
	}
	return filepath.Join(realpath(dir), filepath.Base(p))
}

// resolve confirms path sits inside an allowed root, else errors. Resolving real
// paths first defeats symlink escape tricks (e.g. /work/escape -> /).
func resolve(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real := realpath(abs)
	for _, root := range allowedRoots {
		if real == root || strings.HasPrefix(real, root+string(os.PathSeparator)) {
			return real, nil
		}
	}
	return "", fmt.Errorf("path %q is outside the allowed area (%s)", path, strings.Join(allowedRoots, ", "))
}

// audited runs fn with the same start/end audit envelope as tool runs, and
// fails closed like runner.Run: if the start record can't be persisted, the
// file op is refused rather than run unlogged.
func audited(op, path string, fn func() (string, error)) string {
	id, auditErr := audit.LogStart(op, path, []string{op, path})
	if auditErr != nil {
		return "blocked: could not persist audit record (" + auditErr.Error() +
			") — refusing to run unlogged. Set MARQ_AUDIT_LOG to a writable path."
	}
	out, err := fn()
	code := 0
	errMsg := ""
	if err != nil {
		code = 1
		errMsg = err.Error()
		out = "error: " + err.Error()
	}
	c := code
	audit.LogEnd(id, op, &c, 0, false, errMsg)
	return out
}

// ListDir lists a directory inside the working area.
func ListDir(path string) string {
	if path == "" {
		path = "/work"
	}
	return audited("list_dir", path, func() (string, error) {
		real, err := resolve(path)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(real)
		if err != nil || !info.IsDir() {
			return fmt.Sprintf("not a directory: %s", path), nil
		}
		entries, err := os.ReadDir(real)
		if err != nil {
			return "", err
		}
		slices.SortFunc(entries, func(a, b os.DirEntry) int {
			return strings.Compare(a.Name(), b.Name())
		})
		var rows []string
		for _, e := range entries {
			fi, err := e.Info()
			if err != nil {
				continue
			}
			if fi.IsDir() {
				rows = append(rows, fmt.Sprintf("d        %s/", e.Name()))
			} else {
				rows = append(rows, fmt.Sprintf("f %9d  %s", fi.Size(), e.Name()))
			}
		}
		if len(rows) == 0 {
			return "(empty)", nil
		}
		return strings.Join(rows, "\n"), nil
	})
}

// ReadFile reads a text file from the working area, truncated to the output
// budget (or maxBytes if smaller).
func ReadFile(path string, maxBytes int) string {
	return audited("read_file", path, func() (string, error) {
		real, err := resolve(path)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(real)
		if err != nil || info.IsDir() {
			return fmt.Sprintf("not a file: %s", path), nil
		}
		budget := config.C.MaxOutputChars
		if maxBytes > 0 && maxBytes < budget {
			budget = maxBytes
		}
		if budget < 1 { // guard a misconfigured MARQ_MAX_OUTPUT<=0 (s[:budget] would panic)
			budget = 1
		}
		data, err := os.ReadFile(real)
		if err != nil {
			return "", err
		}
		s := string(data)
		if len(s) > budget {
			return s[:budget] + "\n…[truncated — narrow with max_bytes or read in parts]…", nil
		}
		return s, nil
	})
}

// WriteFile writes a text file into the working area, creating parent dirs.
func WriteFile(path, content string) string {
	return audited("write_file", path, func() (string, error) {
		real, err := resolve(path)
		if err != nil {
			return "", err
		}
		if dir := filepath.Dir(real); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", err
			}
		}
		if err := os.WriteFile(real, []byte(content), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("wrote %d bytes to %s", len(content), real), nil
	})
}
