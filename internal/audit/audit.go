// Package audit is the append-only audit log: every tool invocation is recorded
// as one JSON line before it runs and a second when it completes. This is the
// core safety control of the "logging only" guardrail model — nothing is
// blocked, but everything is attributable and reconstructable after the fact.
package audit

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"marq/internal/config"
)

var mu sync.Mutex

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// newID returns a random UUIDv4 string (no external dependency).
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// write appends one JSON record to the audit log and mirrors it to stderr,
// returning an error if the durable on-disk write fails. The stderr copy goes to
// the container's log stream — outside the unprivileged `marq` user's reach and
// separate from the MCP stdout protocol channel — so a record survives even if
// the on-disk log is unwritable or later truncated by the audited process.
// Callers at the exec choke point use the returned error to fail closed rather
// than run a tool unlogged. Guarded by a mutex so concurrent calls never interleave.
func write(record map[string]any) error {
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	// Integrity mirror first, so it lands even if the file write below fails.
	fmt.Fprintf(os.Stderr, "%s\n", line)
	path := config.C.AuditLog
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fh.Close()
	if _, err := fh.Write(append(line, '\n')); err != nil {
		return err
	}
	return fh.Sync()
}

// LogStart records the start of an invocation and returns a correlation id plus
// any error persisting the record. Exec callers should fail closed when the
// error is non-nil rather than run an unlogged tool.
func LogStart(tool, target string, argv []string) (string, error) {
	id := newID()
	err := write(map[string]any{
		"event":      "invocation.start",
		"id":         id,
		"ts":         now(),
		"operator":   config.C.Operator,
		"engagement": config.C.Engagement,
		"tool":       tool,
		"target":     target,
		"argv":       argv,
	})
	return id, err
}

// LogEnd records the completion of an invocation. exitCode is nil when the tool
// timed out or failed to spawn (mirrors the Python None).
func LogEnd(id, tool string, exitCode *int, durationS float64, timedOut bool, errMsg string) {
	rec := map[string]any{
		"event":      "invocation.end",
		"id":         id,
		"ts":         now(),
		"tool":       tool,
		"exit_code":  exitCode,
		"duration_s": round3(durationS),
		"timed_out":  timedOut,
	}
	if errMsg != "" {
		rec["error"] = errMsg
	} else {
		rec["error"] = nil
	}
	// Best-effort: the tool has already run, so a failure here can't be prevented
	// — but the record is still mirrored to stderr inside write().
	_ = write(rec)
}

func round3(f float64) float64 {
	return math.Round(f*1000) / 1000
}
