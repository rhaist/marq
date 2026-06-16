"""Append-only audit logging.

Every tool invocation is recorded as a single JSON line *before* it runs and a
second line when it completes. This is the core safety control of the
"logging only" guardrail model: nothing is blocked, but everything is
attributable and reconstructable after the fact.
"""
from __future__ import annotations

import datetime as _dt
import json
import os
import threading
import uuid
from pathlib import Path
from typing import Any

from .config import CONFIG

_lock = threading.Lock()


def _now() -> str:
    return _dt.datetime.now(_dt.timezone.utc).isoformat()


def _ensure_log() -> Path:
    path = CONFIG.audit_log
    path.parent.mkdir(parents=True, exist_ok=True)
    return path


def _write(record: dict[str, Any]) -> None:
    line = json.dumps(record, default=str, ensure_ascii=False)
    with _lock:
        path = _ensure_log()
        with path.open("a", encoding="utf-8") as fh:
            fh.write(line + "\n")
            fh.flush()
            os.fsync(fh.fileno())


def log_start(tool: str, target: str, argv: list[str]) -> str:
    """Record the start of an invocation. Returns a correlation id."""
    invocation_id = str(uuid.uuid4())
    _write(
        {
            "event": "invocation.start",
            "id": invocation_id,
            "ts": _now(),
            "operator": CONFIG.operator,
            "engagement": CONFIG.engagement,
            "tool": tool,
            "target": target,
            "argv": argv,
        }
    )
    return invocation_id


def log_end(
    invocation_id: str,
    tool: str,
    *,
    exit_code: int | None,
    duration_s: float,
    timed_out: bool = False,
    error: str | None = None,
) -> None:
    _write(
        {
            "event": "invocation.end",
            "id": invocation_id,
            "ts": _now(),
            "tool": tool,
            "exit_code": exit_code,
            "duration_s": round(duration_s, 3),
            "timed_out": timed_out,
            "error": error,
        }
    )
