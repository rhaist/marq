// Package jobs surfaces background-job status (feature #2). Long scans launched
// via runner.RunBackground stream to /work/jobs/<id>/; today the model has to
// manually read_file/list_dir-poll those dirs. list_jobs/job_status read the
// `status` file (written last by the background wrapper) and tail stdout so the
// model gets a clean running|done answer.
package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"marq/internal/config"
)

func jobsRoot() string { return filepath.Join(config.C.WorkDir, "jobs") }

// statusOf reads a job dir's status file. ok=false means still running.
func statusOf(dir string) (exitCode string, done bool) {
	data, err := os.ReadFile(filepath.Join(dir, "status"))
	if err != nil {
		return "", false
	}
	// status file contains: exit=<code>
	s := strings.TrimSpace(string(data))
	s = strings.TrimPrefix(s, "exit=")
	return s, true
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
		code, done := statusOf(filepath.Join(jobsRoot(), name))
		if done {
			rows = append(rows, fmt.Sprintf("done    (exit=%s)  %s", code, name))
		} else {
			rows = append(rows, fmt.Sprintf("running           %s", name))
		}
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
	code, done := statusOf(dir)
	state := "running"
	if done {
		state = fmt.Sprintf("done (exit=%s)", code)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "job    : %s\nstatus : %s\n", filepath.Base(dir), state)
	fmt.Fprintf(&b, "results: %s/stdout.log\n", dir)
	if tail := tailFile(filepath.Join(dir, "stdout.log"), 40); tail != "" {
		fmt.Fprintf(&b, "--- stdout (last 40 lines) ---\n%s", tail)
	}
	if !done {
		if tail := tailFile(filepath.Join(dir, "stderr.log"), 10); tail != "" {
			fmt.Fprintf(&b, "\n--- stderr (last 10 lines) ---\n%s", tail)
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
