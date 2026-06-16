"""Entry point for the pentest MCP server (stdio transport).

LM Studio (or any MCP client) launches this over stdio. We register every
tool group, surface an authorization banner + scope as MCP resources, and then
hand control to FastMCP's stdio loop.
"""
from __future__ import annotations

import sys

from mcp.server.fastmcp import FastMCP

from .config import CONFIG
from .tools import creds, exploit, recon, shell, web

mcp = FastMCP("pentest-mcp")


def _register_all() -> None:
    recon.register(mcp)
    web.register(mcp)
    exploit.register(mcp)
    creds.register(mcp)
    if CONFIG.allow_raw_shell:
        shell.register(mcp)


@mcp.resource("pentest://authorization")
def authorization_banner() -> str:
    """The rules-of-engagement / authorization notice for this session.
    Clients should surface this to the operator before running tools."""
    return CONFIG.banner()


@mcp.tool()
def server_info() -> str:
    """Return the authorization banner, configured scope and operator metadata.
    Call this first to confirm you are authorized to test the intended targets."""
    raw = "enabled" if CONFIG.allow_raw_shell else "disabled"
    return CONFIG.banner() + f"\n  raw shell  : {raw}"


def main() -> None:
    _register_all()
    # Banner goes to stderr so it never corrupts the stdio JSON-RPC stream.
    print(CONFIG.banner(), file=sys.stderr, flush=True)
    mcp.run(transport="stdio")


if __name__ == "__main__":
    main()
