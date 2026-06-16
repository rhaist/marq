"""Subprocess runner shared by every tool wrapper.

Centralizes: audit logging, timeouts, output truncation and a consistent
result envelope. Tool modules should never call subprocess directly — they go
through `run` so that auditing can never be bypassed.
"""
from __future__ import annotations

import shutil
import subprocess
import time
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
