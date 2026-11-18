// Package files provides sandboxed file access for the engagement working area.
// The MCP transport carries only strings, so these ops let the model stage tool
// inputs and read back tool outputs — confined to /work, /tmp and MARQ_WORK_DIR so it can never
// read or clobber the rest of the container. Every op is audit-logged through
// the same audit module as tool runs.
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rhaist/marq/internal/audit"
	"github.com/rhaist/marq/internal/config"
	"github.com/rhaist/marq/internal/untrusted"
)

// allowedRoots are the only directories the model may touch, plus MARQ_WORK_DIR
// (see roots) so findings/jobs written there on a bare install stay readable.
var allowedRoots = []string{"/work", "/tmp"}

// Roots are realpath'd so they match Resolve()'s output (macOS /tmp -> /private/tmp).
// A MARQ_WORK_DIR that is / or contains $HOME is ignored: it would expose
// ~/.ssh, ~/.aws etc. to read_file/write_file.
func roots() []string {
	rs := make([]string, 0, len(allowedRoots)+1)
	for _, r := range allowedRoots {
		rs = append(rs, realpath(r))
	}
	if config.C.WorkDir == "" {
		return rs
	}
	w, err := filepath.Abs(config.C.WorkDir)
	if err != nil {
		return rs
	}
	w = realpath(w)
	if home, err := os.UserHomeDir(); w == "/" || (err == nil && (realpath(home) == w || strings.HasPrefix(realpath(home), w+string(os.PathSeparator)))) {
		return rs
	}
	if slices.Contains(rs, w) { // MARQ_WORK_DIR=/work (the image default) is already a root
		return rs
	}
	return append(rs, w)
}

// realpath resolves symlinks for the longest existing prefix of p, then appends
// the remainder, so it works even when p (or its tail) does not exist yet
// (needed by write_file).
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

// Resolve confirms path sits inside an allowed root, else errors. Resolving real
// paths first defeats symlink escape tricks (e.g. /work/escape -> /).
func Resolve(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real := realpath(abs)
	// The audit log is the tamper-evidence control; on a bare install it sits
	// inside MARQ_WORK_DIR, so refuse the file itself rather than the dir.
	if a, err := filepath.Abs(config.C.AuditLog); config.C.AuditLog != "" && err == nil && real == realpath(a) {
		return "", fmt.Errorf("path %q is the audit log, which is off-limits", path)
	}
	rs := roots()
	for _, root := range rs {
		if real == root || strings.HasPrefix(real, root+string(os.PathSeparator)) {
			return real, nil
		}
	}
	return "", fmt.Errorf("path %q is outside the allowed area (%s)", path, strings.Join(rs, ", "))
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
		real, err := Resolve(path)
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
		return untrusted.Wrap("dir "+path, strings.Join(rows, "\n")), nil // names of staged files are third-party too
	})
}

// ReadFile reads a text file from the working area, truncated to the output
// budget (or maxBytes if smaller).
func ReadFile(path string, maxBytes, offset int) string {
	return audited("read_file", path, func() (string, error) {
		real, err := Resolve(path)
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
		if offset < 0 {
			offset = 0
		}
		if offset >= len(data) && len(data) > 0 {
			return fmt.Sprintf("offset %d is past the end of %s (%d bytes)", offset, path, len(data)), nil
		}
		data = data[min(offset, len(data)):]
		s := string(data)
		if len(data) > budget {
			cut := budget
			for cut > 1 && !utf8.RuneStart(data[cut]) { // never split a character across two parts
				cut--
			}
			s = string(data[:cut]) + fmt.Sprintf("\n…[truncated: continue with offset=%d]…", offset+cut)
		}
		return untrusted.Wrap("file "+path, s), nil // files hold tool output: same fence as runner
	})
}

// WriteFile writes a text file into the working area, creating parent dirs.
func WriteFile(path, content string) string {
	return audited("write_file", path, func() (string, error) {
		real, err := Resolve(path)
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
