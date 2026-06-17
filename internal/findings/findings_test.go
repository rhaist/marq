package findings

import (
	"path/filepath"
	"strings"
	"testing"

	"pentest-mcp/internal/config"
)

// TestSeverityPrecedence locks in: caller severity is authoritative; CVSS only
// fills in when severity is omitted; a divergent vector is flagged, not applied.
func TestSeverityPrecedence(t *testing.T) {
	config.C.WorkDir = t.TempDir()
	config.C.AuditLog = filepath.Join(t.TempDir(), "audit.jsonl")
	crit := "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H" // 9.8 critical

	// Explicit severity wins; mismatch with the vector is flagged, not applied.
	out := Report("t1", "low", "x", "", "", crit, "", "")
	if !strings.Contains(out, "[low]") {
		t.Errorf("explicit severity not kept: %s", out)
	}
	if !strings.Contains(out, "note:") {
		t.Errorf("severity/CVSS mismatch not flagged: %s", out)
	}

	// Omitted severity is derived from the CVSS rating.
	if out := Report("t2", "", "x", "", "", crit, "", ""); !strings.Contains(out, "[critical]") {
		t.Errorf("severity not derived from CVSS: %s", out)
	}

	// Omitted severity, no CVSS -> info.
	if out := Report("t3", "", "x", "", "", "", "", ""); !strings.Contains(out, "[info]") {
		t.Errorf("default severity not info: %s", out)
	}
}
