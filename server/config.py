"""Configuration for the pentest MCP server.

All settings are environment-driven so the container can be configured at
launch time (e.g. from LM Studio's mcp.json `env` block) without rebuilding.
"""
from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path


def _bool(name: str, default: bool = False) -> bool:
    val = os.environ.get(name)
    if val is None:
        return default
    return val.strip().lower() in {"1", "true", "yes", "on"}


def _int(name: str, default: int) -> int:
    try:
        return int(os.environ.get(name, default))
    except (TypeError, ValueError):
        return default


@dataclass
class Config:
    # Where the structured audit log is written (JSON lines).
    audit_log: Path = field(
        default_factory=lambda: Path(
            os.environ.get("PENTEST_MCP_AUDIT_LOG", "/var/log/pentest-mcp/audit.jsonl")
        )
    )
    # Per-command wall-clock timeout in seconds.
    command_timeout: int = field(default_factory=lambda: _int("PENTEST_MCP_TIMEOUT", 900))
    # Max characters of tool output returned to the model (keeps token use sane).
    max_output_chars: int = field(
        default_factory=lambda: _int("PENTEST_MCP_MAX_OUTPUT", 60_000)
    )
    # Operator / engagement metadata recorded in every audit record.
    operator: str = field(default_factory=lambda: os.environ.get("PENTEST_MCP_OPERATOR", "unknown"))
    engagement: str = field(
        default_factory=lambda: os.environ.get("PENTEST_MCP_ENGAGEMENT", "unspecified")
    )
    # Optional scope note shown in the authorization banner. Free text describing
    # the authorized targets / rules of engagement for this session.
    scope_note: str = field(default_factory=lambda: os.environ.get("PENTEST_MCP_SCOPE", ""))
    # When true, the generic `run_shell` escape hatch is exposed. Off by default.
    allow_raw_shell: bool = field(default_factory=lambda: _bool("PENTEST_MCP_ALLOW_RAW_SHELL", False))

    def banner(self) -> str:
        return (
            "AUTHORIZED USE ONLY. This MCP server runs active security testing "
            "tools. Only use it against systems you own or are explicitly "
            "authorized in writing to test. All invocations are audit-logged.\n"
            f"  operator   : {self.operator}\n"
            f"  engagement : {self.engagement}\n"
            f"  scope note : {self.scope_note or '(none provided — set PENTEST_MCP_SCOPE)'}\n"
            f"  audit log  : {self.audit_log}"
        )


CONFIG = Config()
