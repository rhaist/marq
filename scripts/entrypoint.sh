#!/bin/bash
# Entrypoint for the pentest MCP container.
#
# With no args (the default) or `mcp`/`serve`, launch the MCP stdio server. Any
# other args are executed verbatim so the image doubles as an interactive
# pentest shell:
#   docker run --rm -it --entrypoint /bin/bash pentest-mcp
#   docker run --rm pentest-mcp nmap --version
set -euo pipefail

case "${1:-}" in
    "" | mcp | serve)
        exec pentest serve
        ;;
    tui)
        exec pentest tui
        ;;
esac

exec "$@"
