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

	"pentest-mcp/internal/config"
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

// write appends one JSON record to the audit log, flushing and fsync'ing so the
// trail survives a crash. Guarded by a mutex so concurrent calls never interleave.
func write(record map[string]any) {
	line, err := json.Marshal(record)
	if err != nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	path := config.C.AuditLog
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer fh.Close()
	_, _ = fh.Write(append(line, '\n'))
	_ = fh.Sync()
}

// LogStart records the start of an invocation and returns a correlation id.
func LogStart(tool, target string, argv []string) string {
	id := newID()
	write(map[string]any{
		"event":      "invocation.start",
		"id":         id,
		"ts":         now(),
		"operator":   config.C.Operator,
		"engagement": config.C.Engagement,
		"tool":       tool,
		"target":     target,
		"argv":       argv,
	})
	return id
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
	write(rec)
}

func round3(f float64) float64 {
	return math.Round(f*1000) / 1000
}
