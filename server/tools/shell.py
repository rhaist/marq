"""Optional raw-shell escape hatch.

Disabled unless PENTEST_MCP_ALLOW_RAW_SHELL=true. When enabled it lets the
model run any of the image's hundreds of tools that don't have a dedicated
wrapper. Still fully audit-logged. This is deliberately opt-in because it is
the broadest capability the server can grant.
"""
from __future__ import annotations

from ..runner import run


def register(mcp) -> None:
    @mcp.tool()
    def run_shell(command: str, target: str = "(raw shell)") -> str:
        """Run an arbitrary shell command inside the pentest container. Use for
        tools without a dedicated wrapper. `target` should name the host/URL
        under test for the audit record. Audit-logged; authorized use only."""
        argv = ["/bin/bash", "-c", command]
        return run("run_shell", argv, target=target).render()
