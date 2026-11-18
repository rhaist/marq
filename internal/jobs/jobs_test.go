package jobs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhaist/marq/internal/config"
)

// TestState: a job is done once status exists; otherwise its pid file decides
// between running, stopped (dead pid on this host) and unknown (another host,
// e.g. a container that has since stopped).
func TestState(t *testing.T) {
	orig := config.C.WorkDir
	t.Cleanup(func() { config.C.WorkDir = orig })
	config.C.WorkDir = t.TempDir()
	host, _ := os.Hostname()
	dead := exec.Command("true")
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}
	job := func(name, pid, status string) {
		dir := filepath.Join(jobsRoot(), name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if pid != "" {
			_ = os.WriteFile(filepath.Join(dir, "pid"), []byte(pid), 0o644)
		}
		if status != "" {
			_ = os.WriteFile(filepath.Join(dir, "status"), []byte(status), 0o644)
		}
	}
	job("a-done", fmt.Sprintf("%s %d\n", host, dead.Process.Pid), "exit=0\n")
	job("b-live", fmt.Sprintf("%s %d\n", host, os.Getpid()), "")
	job("c-dead", fmt.Sprintf("%s %d\n", host, dead.Process.Pid), "")
	job("d-other", "f00dcafe1234 7\n", "")
	job("e-legacy", "", "")

	list := List()
	for _, want := range []string{"a-done             done (exit=0)", "b-live             running", "c-dead             stopped without an exit status", "d-other            unknown (started in container f00dcafe1234", "e-legacy           running"} {
		if !strings.Contains(list, want) {
			t.Errorf("List() missing %q:\n%s", want, list)
		}
	}
	if st := Status("c-dead"); !strings.Contains(st, "status : stopped") {
		t.Errorf("Status(c-dead) = %q", st)
	}
}
