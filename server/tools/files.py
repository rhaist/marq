"""Sandboxed file access for the engagement working area.

The MCP transport only carries strings, so without these tools the model can
neither stage an input file (a captured hash, a target list, a wordlist) nor
read back output a tool wrote to disk (cmseek's JSON, nuclei `-o`, downloaded
documents). These three tools close that gap, scoped to a small allowlist of
directories so the model can never read or clobber the rest of the container.

Access is confined to:
  * /work  — the engagement working dir (mount it from the host; see compose)
  * /tmp   — scratch space some wrapped tools write to

Every operation is audit-logged through the same `audit` module as tool runs,
so the "nothing bypasses the audit trail" guarantee still holds.
"""
from __future__ import annotations

import os

from .. import audit
from ..config import CONFIG

# Directories the model is allowed to touch. Resolved to real paths so symlink
# tricks (e.g. /work/escape -> /) can't break out.
ALLOWED_ROOTS = ("/work", "/tmp")


def _resolve(path: str) -> str:
    """Resolve `path` and confirm it sits inside an allowed root, else raise."""
    real = os.path.realpath(path)
    for root in ALLOWED_ROOTS:
        if real == root or real.startswith(root + os.sep):
            return real
    raise PermissionError(
        f"path {path!r} is outside the allowed area ({', '.join(ALLOWED_ROOTS)})"
    )


def _audited(op: str, path: str, fn):
    """Run a file op with the same start/end audit envelope as tool runs."""
    inv = audit.log_start(op, path, [op, path])
    error: str | None = None
    try:
        return fn()
    except Exception as exc:  # noqa: BLE001 - surface the failure to the model
        error = repr(exc)
        return f"error: {exc}"
    finally:
        audit.log_end(inv, op, exit_code=0 if error is None else 1, duration_s=0.0, error=error)


def register(mcp) -> None:
    @mcp.tool()
    def list_dir(path: str = "/work") -> str:
        """List a directory inside the working area (/work or /tmp). Shows entry
        names, type (dir/file) and size. Use this to find files other tools
        wrote, or to confirm a staged input is present."""
        def _do() -> str:
            real = _resolve(path)
            if not os.path.isdir(real):
                return f"not a directory: {path}"
            rows = []
            for name in sorted(os.listdir(real)):
                full = os.path.join(real, name)
                if os.path.isdir(full):
                    rows.append(f"d        {name}/")
                else:
                    rows.append(f"f {os.path.getsize(full):>9}  {name}")
            return "\n".join(rows) if rows else "(empty)"
        return _audited("list_dir", path, _do)

    @mcp.tool()
    def read_file(path: str, max_bytes: int = 0) -> str:
        """Read a text file from the working area (/work or /tmp). Returns the
        contents (truncated to the server output budget, or to `max_bytes` if
        smaller). Use it to retrieve results tools wrote to disk."""
        def _do() -> str:
            real = _resolve(path)
            if not os.path.isfile(real):
                return f"not a file: {path}"
            budget = CONFIG.max_output_chars
            if max_bytes > 0:
                budget = min(budget, max_bytes)
            with open(real, "r", encoding="utf-8", errors="replace") as fh:
                data = fh.read(budget + 1)
            if len(data) > budget:
                return data[:budget] + "\n…[truncated — narrow with max_bytes or read in parts]…"
            return data
        return _audited("read_file", path, _do)

    @mcp.tool()
    def write_file(path: str, content: str) -> str:
        """Write a text file into the working area (/work or /tmp), creating
        parent dirs as needed. Use it to stage inputs for other tools — e.g. a
        captured hash for `john`, a list of targets for `httpx_probe`/`dnsx`, or
        a custom wordlist. Overwrites an existing file at `path`."""
        def _do() -> str:
            real = _resolve(path)
            os.makedirs(os.path.dirname(real) or "/", exist_ok=True)
            with open(real, "w", encoding="utf-8") as fh:
                fh.write(content)
            return f"wrote {len(content)} bytes to {real}"
        return _audited("write_file", path, _do)
