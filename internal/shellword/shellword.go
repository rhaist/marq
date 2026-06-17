// Package shellword provides shell word-splitting and quoting, the Go
// equivalents of Python's shlex.split / shlex.quote / shlex.join used by the
// original tool wrappers.
package shellword

import (
	"strings"

	"github.com/google/shlex"
)

// Split parses a string into shell words (like shlex.split). On a parse error
// it falls back to whitespace splitting so a malformed `options` string never
// hard-fails a tool call.
func Split(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts, err := shlex.Split(s)
	if err != nil {
		return strings.Fields(s)
	}
	return parts
}

// Quote returns a POSIX-shell-safe single token (like shlex.quote).
func Quote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if !(r == '_' || r == '-' || r == '.' || r == '/' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Join quotes each argument and joins them with spaces (like shlex.join).
func Join(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = Quote(a)
	}
	return strings.Join(parts, " ")
}
