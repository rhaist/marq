"""Subprocess runner shared by every tool wrapper.

Centralizes: audit logging, timeouts, output truncation and a consistent
result envelope. Tool modules should never call subprocess directly — they go
through `run` so that auditing can never be bypassed.
"""
from __future__ import annotations

import os
import shlex
import shutil
import subprocess
import time
import uuid
from dataclasses import dataclass

from . import audit
from .config import CONFIG


@dataclass
class Result:
    tool: str
    argv: list[str]
    exit_code: int | None
    stdout: str
    stderr: str
    duration_s: float
    timed_out: bool = False
    truncated: bool = False
    timeout_s: int = 0

    def render(self) -> str:
        parts = [f"$ {' '.join(self.argv)}"]
        if self.timed_out:
            parts.append(f"[timed out after {self.timeout_s}s]")
        parts.append(f"[exit code: {self.exit_code}, {self.duration_s:.1f}s]")
        if self.stdout:
            parts.append("--- stdout ---\n" + self.stdout)
        if self.stderr:
            parts.append("--- stderr ---\n" + self.stderr)
        if self.truncated:
            parts.append(
                f"\n[output truncated to {CONFIG.max_output_chars} chars — "
                "narrow the scan or write results to a file inside the container]"
            )
        return "\n".join(parts)


def _truncate(text: str, budget: int) -> tuple[str, bool]:
    if len(text) <= budget:
        return text, False
    head = text[: budget - 200]
    return head + "\n…[truncated]…", True


def run(
    tool: str,
    argv: list[str],
    *,
    target: str = "",
    stdin: str | None = None,
    timeout: int | None = None,
) -> Result:
    """Run a command with auditing, a timeout and output truncation.

    `tool` is the logical tool name (for audit + result), `argv` the full
    command vector, `target` a best-effort extraction of the host/URL under
    test (recorded in the audit log). `timeout` overrides the default
    per-command wall-clock limit for this call only, clamped to
    `CONFIG.max_command_timeout` — used by the slow OSINT/scan tools.
    """
    limit = CONFIG.command_timeout if timeout is None else max(1, min(timeout, CONFIG.max_command_timeout))
    binary = argv[0]
    if shutil.which(binary) is None:
        return Result(
            tool=tool,
            argv=argv,
            exit_code=127,
            stdout="",
            stderr=f"binary not found in image: {binary}",
            duration_s=0.0,
        )

    invocation_id = audit.log_start(tool, target or "(unspecified)", argv)
    start = time.monotonic()
    timed_out = False
    error: str | None = None
    try:
        proc = subprocess.run(
            argv,
            input=stdin,
            capture_output=True,
            text=True,
            timeout=limit,
        )
        exit_code = proc.returncode
        stdout, stderr = proc.stdout, proc.stderr
    except subprocess.TimeoutExpired as exc:
        timed_out = True
        exit_code = None
        stdout = exc.stdout.decode() if isinstance(exc.stdout, bytes) else (exc.stdout or "")
        stderr = exc.stderr.decode() if isinstance(exc.stderr, bytes) else (exc.stderr or "")
    except Exception as exc:  # noqa: BLE001 - report any spawn failure to the model
        timed_out = False
        exit_code = None
        stdout = ""
        stderr = ""
        error = repr(exc)

    duration = time.monotonic() - start
    audit.log_end(
        invocation_id,
        tool,
        exit_code=exit_code,
        duration_s=duration,
        timed_out=timed_out,
        error=error,
    )

    budget = CONFIG.max_output_chars
    out, out_trunc = _truncate(stdout, budget // 2)
    err, err_trunc = _truncate(stderr, budget // 2)
    return Result(
        tool=tool,
        argv=argv,
        exit_code=exit_code,
        stdout=out if not error else (error),
        stderr=err,
        duration_s=duration,
        timed_out=timed_out,
        truncated=out_trunc or err_trunc,
        timeout_s=limit,
    )


def run_background(tool: str, argv: list[str], *, target: str = "", job_root: str = "/work/jobs") -> tuple[str | None, str | None]:
    """Launch a long-running tool detached and return immediately.

    For tools that can't finish inside an interactive tool-call window (broad
    OSINT scans, etc.). Output streams to files under a per-job directory in the
    working area, which the operator/model reads back with the `read_file` /
    `list_dir` tools. Returns `(job_dir, None)` on success or `(None, error)`.

    Layout of the job dir: `stdout.log` (results), `stderr.log` (diagnostics),
    `status` (written last, contains `exit=<code>` once the tool finishes — its
    presence is the done-signal). The child is audit-logged at launch.
    """
    if shutil.which(argv[0]) is None:
        return None, f"binary not found in image: {argv[0]}"
    job_dir = os.path.join(job_root, f"{tool}-{uuid.uuid4().hex[:8]}")
    try:
        os.makedirs(job_dir, exist_ok=True)
    except OSError as exc:
        return None, f"could not create job dir {job_dir}: {exc}"

    out, err, status = (os.path.join(job_dir, n) for n in ("stdout.log", "stderr.log", "status"))
    inv = audit.log_start(f"{tool}:bg", target or "(unspecified)", argv)
    # Wrap so the child redirects its streams and records its own exit code.
    cmd = (
        f"( {shlex.join(argv)} ) >{shlex.quote(out)} 2>{shlex.quote(err)}; "
        f'echo "exit=$?" >{shlex.quote(status)}'
    )
    try:
        subprocess.Popen(  # noqa: S603 - detached background job, intentional
            ["/bin/bash", "-c", cmd],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
        )
    except Exception as exc:  # noqa: BLE001 - report any spawn failure
        audit.log_end(inv, f"{tool}:bg", exit_code=None, duration_s=0.0, error=repr(exc))
        return None, f"failed to launch: {exc}"
    audit.log_end(inv, f"{tool}:bg", exit_code=None, duration_s=0.0)
    return job_dir, None
