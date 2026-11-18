// Package runner is the single choke point every tool invocation funnels
// through. It centralizes audit logging, timeouts and output truncation behind
// a consistent Result envelope. Tool wrappers must never exec directly — they
// go through Run/RunBackground so auditing can never be bypassed.
package runner

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rhaist/marq/internal/audit"
	"github.com/rhaist/marq/internal/config"
	"github.com/rhaist/marq/internal/shellword"
	"github.com/rhaist/marq/internal/untrusted"
)

// Opts are per-call options for Run.
type Opts struct {
	Target string
	Stdin  string
}

// Result is the outcome of a tool invocation, rendered into a text envelope the
// model reads.
type Result struct {
	Tool      string
	Argv      []string
	ExitCode  *int // nil when the tool timed out or failed to spawn
	Stdout    string
	Stderr    string
	DurationS float64
	TimedOut  bool
	Truncated bool
	FullOut   string // when Truncated: file holding the untruncated output ("" if it couldn't be saved)
	TimeoutS  int
}

// Render formats the result for model consumption.
func (r Result) Render() string {
	var b strings.Builder
	// The first line of the command, capped: inline formatter scripts are noise to the
	// model, and the full argv is in the audit log.
	cmd, _, cut := strings.Cut(strings.Join(r.Argv, " "), "\n")
	if len(cmd) > 160 {
		cmd, cut = strings.ToValidUTF8(cmd[:160], ""), true
	}
	if cut {
		cmd += " …"
	}
	fmt.Fprintf(&b, "$ %s", cmd)
	if r.TimedOut {
		fmt.Fprintf(&b, "\n[timed out after %ds]", r.TimeoutS)
	}
	code := "nil"
	if r.ExitCode != nil {
		code = strconv.Itoa(*r.ExitCode)
	}
	fmt.Fprintf(&b, "\n[exit code: %s, %.1fs]", code, r.DurationS)
	// Tool output is third-party text (a prompt-injection channel): sanitize and fence it.
	if r.Stdout != "" {
		fmt.Fprintf(&b, "\n--- stdout ---\n%s", untrusted.Wrap(r.Tool, r.Stdout))
	}
	if r.Stderr != "" {
		fmt.Fprintf(&b, "\n--- stderr ---\n%s", untrusted.Wrap(r.Tool, r.Stderr))
	}
	// A non-zero exit with no output is opaque to a model (e.g. a missing input
	// file or an absent API key). Name the likely causes instead of leaving it bare.
	if !r.TimedOut && r.ExitCode != nil && *r.ExitCode != 0 && r.Stdout == "" && r.Stderr == "" {
		b.WriteString("\n[no output — the tool failed silently; check that input files/paths exist, " +
			"required flags are set, and any needed API key is present in the container env]")
	}
	if r.Truncated && r.FullOut != "" {
		fmt.Fprintf(&b, "\n\n[output truncated to %d chars. The full output is in %s: read it in parts "+
			"with read_file and `offset`, or narrow the query]", config.C.MaxOutputChars, r.FullOut)
	} else if r.Truncated {
		fmt.Fprintf(&b, "\n\n[output truncated to %d chars — narrow the query]", config.C.MaxOutputChars)
	}
	return b.String()
}

// saveFull writes untruncated output under WorkDir/out so a model can page
// through it with read_file. Returns the path, or "" if it couldn't be written.
// shortcut: never pruned, like jobs/ dirs; add a size or age cap if /work fills up.
func saveFull(tool string, stdout, stderr []byte) string {
	dir := filepath.Join(config.C.WorkDir, "out")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	p := filepath.Join(dir, fmt.Sprintf("%s-%s.log", tool, shortID()))
	data := stdout
	if len(stderr) > 0 {
		data = append(append(append([]byte{}, stdout...), "\n--- stderr ---\n"...), stderr...)
	}
	if os.WriteFile(p, data, 0o644) != nil {
		return ""
	}
	return p
}

// truncateTailBudget is reserved for the truncation marker so a truncated
// stream still ends in a legible "…[truncated]…" tail rather than a bare cut.
const truncateTailBudget = 200

func truncate(text string, budget int) (string, bool) {
	if budget < 1 {
		budget = 1
	}
	if len(text) <= budget {
		return text, false
	}
	head := max(budget-truncateTailBudget, 0)
	return text[:head] + "\n…[truncated]…", true
}

// Run executes argv with auditing, a timeout and output truncation.
func Run(tool string, argv []string, opts Opts) Result {
	limit := config.C.CommandTimeout

	target := opts.Target
	if target == "" {
		target = "(unspecified)"
	}
	// Audit first (fail closed) so even an attempt to run a missing/renamed binary
	// is attributable — the start record is the core control, never skip it.
	id, auditErr := audit.LogStart(tool, target, argv)
	if auditErr != nil {
		code := 126
		return Result{Tool: tool, Argv: argv, ExitCode: &code, TimeoutS: limit,
			Stderr: "blocked: could not persist audit record (" + auditErr.Error() +
				") — refusing to run unlogged. Set MARQ_AUDIT_LOG to a writable path."}
	}

	if _, err := exec.LookPath(argv[0]); err != nil {
		code := 127
		audit.LogEnd(id, tool, &code, 0, false, "binary not found: "+argv[0])
		return Result{Tool: tool, Argv: argv, ExitCode: &code,
			Stderr: "binary not found in image: " + argv[0], TimeoutS: limit}
	}
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(limit)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// Wrappers are often `bash -c 'tool | filter'`: on timeout kill the whole process
	// group, or the orphaned tool keeps the output pipe open and Wait never returns.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}
	runErr := cmd.Run()
	duration := time.Since(start).Seconds()

	var exitCode *int
	timedOut := false
	var errMsg string
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		timedOut = true
	case runErr == nil:
		c := cmd.ProcessState.ExitCode()
		exitCode = &c
	default:
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			c := ee.ExitCode()
			exitCode = &c
		} else {
			errMsg = runErr.Error()
		}
	}
	audit.LogEnd(id, tool, exitCode, duration, timedOut, errMsg)

	budget := config.C.MaxOutputChars
	out, outTrunc := truncate(outBuf.String(), budget/2)
	er, errTrunc := truncate(errBuf.String(), budget/2)
	if errMsg != "" && out == "" {
		out = errMsg
	}
	var full string
	if outTrunc || errTrunc {
		full = saveFull(tool, outBuf.Bytes(), errBuf.Bytes())
	}
	return Result{
		Tool: tool, Argv: argv, ExitCode: exitCode,
		Stdout: out, Stderr: er, DurationS: duration,
		TimedOut: timedOut, Truncated: outTrunc || errTrunc, FullOut: full, TimeoutS: limit,
	}
}

// RunBackground launches a long-running tool detached and returns its job dir
// immediately. Output streams to stdout.log/stderr.log under the job dir; a
// `status` file (containing exit=<code>) appears when the tool finishes — its
// presence is the done-signal the file tools poll for.
func RunBackground(tool string, argv []string, target string) (jobDir string, errMsg string) {
	if _, err := exec.LookPath(argv[0]); err != nil {
		return "", "binary not found in image: " + argv[0]
	}
	jobDir = filepath.Join(config.C.WorkDir, "jobs", fmt.Sprintf("%s-%s", tool, shortID()))
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return "", fmt.Sprintf("could not create job dir %s: %v", jobDir, err)
	}
	out := filepath.Join(jobDir, "stdout.log")
	errf := filepath.Join(jobDir, "stderr.log")
	status := filepath.Join(jobDir, "status")

	if target == "" {
		target = "(unspecified)"
	}
	id, auditErr := audit.LogStart(tool+":bg", target, argv)
	if auditErr != nil {
		return "", "blocked: could not persist audit record (" + auditErr.Error() +
			") — refusing to run unlogged. Set MARQ_AUDIT_LOG to a writable path."
	}
	// Wrap so the child redirects its streams and records its own exit code.
	// `timeout` caps the background job at the same ceiling the sync path clamps
	// to, so a detached tool can't run unbounded (a timed-out job records exit=124).
	cmdStr := fmt.Sprintf("timeout %d %s >%s 2>%s; echo \"exit=$?\" >%s",
		config.C.MaxCommandTimeout, shellword.Join(argv), shellword.Quote(out), shellword.Quote(errf), shellword.Quote(status))
	c := exec.Command("/bin/bash", "-c", cmdStr)
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach into its own session
	c.Stdin, c.Stdout, c.Stderr = nil, nil, nil
	if err := c.Start(); err != nil {
		audit.LogEnd(id, tool+":bg", nil, 0, false, err.Error())
		return "", fmt.Sprintf("failed to launch: %v", err)
	}
	go func() { _ = c.Wait() }() // reap the bash wrapper without blocking
	audit.LogEnd(id, tool+":bg", nil, 0, false, "")
	return jobDir, ""
}

func shortID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}
