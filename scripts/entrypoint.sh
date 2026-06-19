#!/bin/bash
# Entrypoint for the marq MCP container.
#
# With no args (the default) or `mcp`/`serve`, launch the MCP stdio server. Any
# other args are executed verbatim so the image doubles as an interactive
# marq shell:
#   docker run --rm -it --entrypoint /bin/bash marq
#   docker run --rm marq nmap --version
set -euo pipefail

case "${1:-}" in
    "" | mcp | serve)
        exec marq serve
        ;;
    tui)
        exec marq tui
        ;;
esac

exec "$@"
