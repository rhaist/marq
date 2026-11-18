// Package jobs surfaces background-job status. Long lookups launched via
// runner.RunBackground stream to /work/jobs/<id>/; list_jobs/job_status read the
// `status` file (written last by the background wrapper) and the pid file, and
// tail stdout, so the model gets a clean running|done|stopped answer.
package jobs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/rhaist/marq/internal/config"
	"github.com/rhaist/marq/internal/untrusted"
)

func jobsRoot() string { return filepath.Join(config.C.WorkDir, "jobs") }

// state reads a job dir: done (the wrapper wrote status), running, or a job
// that ended without a status. The pid file holds "<host> <pid>"; in Docker
// the host is the container, whose processes all stop with it.
func state(dir string) string {
	if data, err := os.ReadFile(filepath.Join(dir, "status")); err == nil {
		return "done (" + strings.TrimSpace(string(data)) + ")"
	}
	var host string
	var pid int
	data, err := os.ReadFile(filepath.Join(dir, "pid"))
	if err != nil {
		return "running" // pre-pid job dirs, or the launch is still writing it
	}
	if _, err := fmt.Sscanf(string(data), "%s %d", &host, &pid); err != nil || pid <= 0 {
		return "running"
	}
	if me, _ := os.Hostname(); host != me {
		return "unknown (started in container " + host + "; it stopped if that container did)"
	}
	if err := syscall.Kill(pid, 0); (err == nil || errors.Is(err, syscall.EPERM)) && !zombie(pid) {
		return "running"
	}
	return "stopped without an exit status (killed, or its container stopped); results may be partial"
}

// zombie reports a process that has exited but was never reaped (it still
// answers kill -0), as happens under a PID 1 that doesn't reap, like `sleep`.
func zombie(pid int) bool {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false // no /proc (macOS): trust kill -0
	}
	i := bytes.LastIndexByte(b, ')') // comm may contain spaces; the state follows ") "
	return i >= 0 && i+2 < len(b) && b[i+2] == 'Z'
}

// List returns one line per background job with its running/done state.
func List() string {
	entries, err := os.ReadDir(jobsRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return "no background jobs yet"
		}
		return "error: " + err.Error()
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "no background jobs yet"
	}
	sort.Strings(names)
	var rows []string
	for _, name := range names {
		rows = append(rows, fmt.Sprintf("%-18s %s", name, state(filepath.Join(jobsRoot(), name))))
	}
	return strings.Join(rows, "\n")
}

// Status reports one job's state plus a tail of its output. job may be a job-dir
// name or a full path; it is confined to the jobs root.
func Status(job string) string {
	dir := resolveJobDir(job)
	if dir == "" {
		return fmt.Sprintf("error: job %q not found under %s", job, jobsRoot())
	}
	st := state(dir)
	done := strings.HasPrefix(st, "done")
	var b strings.Builder
	fmt.Fprintf(&b, "job    : %s\nstatus : %s\n", filepath.Base(dir), st)
	fmt.Fprintf(&b, "results: %s/stdout.log\n", dir)
	src := "job " + filepath.Base(dir)
	if tail := tailFile(filepath.Join(dir, "stdout.log"), 40); tail != "" {
		fmt.Fprintf(&b, "--- stdout (last 40 lines) ---\n%s", untrusted.Wrap(src, tail))
	}
	if !done {
		if tail := tailFile(filepath.Join(dir, "stderr.log"), 10); tail != "" {
			fmt.Fprintf(&b, "\n--- stderr (last 10 lines) ---\n%s", untrusted.Wrap(src, tail))
		}
	}
	return b.String()
}

// resolveJobDir maps a job name or path to a directory inside the jobs root,
// rejecting anything that escapes it.
func resolveJobDir(job string) string {
	root := jobsRoot()
	cand := job
	if !filepath.IsAbs(cand) {
		cand = filepath.Join(root, filepath.Base(job))
	}
	cand = filepath.Clean(cand)
	if cand != root && !strings.HasPrefix(cand, root+string(os.PathSeparator)) {
		return ""
	}
	if info, err := os.Stat(cand); err != nil || !info.IsDir() {
		return ""
	}
	return cand
}

// tailFile returns the last n lines of a file (best effort).
func tailFile(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
