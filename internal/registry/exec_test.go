package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhaist/marq/internal/config"
)

func withScopeNote(t *testing.T, note string) {
	t.Helper()
	dir := t.TempDir()
	orig := config.C
	config.C.WorkDir, config.C.AuditLog, config.C.ScopeNote = dir, filepath.Join(dir, "audit.jsonl"), note
	t.Cleanup(func() { config.C = orig })
}

func execRefusal(t *testing.T, command string) string {
	t.Helper()
	tool := toolNamed(t, "exec")
	return tool.refusal(tool.Build(Args{"command": command}))
}

func TestExecGate(t *testing.T) {
	withScopeNote(t, "10.0.0.0/24")
	list := filepath.Join(t.TempDir(), "targets.txt")
	if err := os.WriteFile(list, []byte("10.0.0.5\n8.8.8.8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for cmd, want := range map[string]string{
		"nmap -sV 10.0.0.5":                  "",
		"nmap -iL " + list:                   "8.8.8.8 is outside",
		"nxc smb " + list:                    "8.8.8.8 is outside",
		"bash -c id":                         "not on the exec allowlist",
		"radare2 -qc '!id' -":                "not on the exec allowlist",
		"nmap -iR 100":                       "can't see",
		"nmap --script /work/x.nse 10.0.0.5": "custom script files",
		"nmap --script=http-title 10.0.0.5":  "",
		"curl -K /work/cfg http://10.0.0.5/": "config files",
		"nuclei -code -u http://10.0.0.5":    "code templates",
		"sqlmap -uhttp://evil.com --batch":   "evil.com is outside",
		"capa /work/sample.exe":              "", // offline: no scope check
		"impacket-secretsdump u:p@10.0.0.5":  "",
	} {
		got := execRefusal(t, cmd)
		if want == "" && got != "" || want != "" && !strings.Contains(got, want) {
			t.Errorf("%q: refusal %q, want %q", cmd, got, want)
		}
	}
	withScopeNote(t, "")
	if got := execRefusal(t, "yara /work/r.yar /work/s.bin"); got != "" {
		t.Errorf("offline tool refused without scope: %q", got)
	}
	if got := execRefusal(t, "nmap 10.0.0.5"); !strings.Contains(got, "no engagement scope") {
		t.Errorf("active exec without scope: %q", got)
	}
}
