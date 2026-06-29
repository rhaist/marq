#!/bin/bash
# Entrypoint for the marq MCP container.
#
# With no args (the default) or `mcp`/`serve`, launch the MCP stdio server. Any
# other args are executed verbatim so the image doubles as an interactive
# marq shell:
#   docker run --rm -it --entrypoint /bin/bash marq
#   docker run --rm marq nmap --version
set -euo pipefail

init_jwt_keys() {
    rm -rf "$HOME/.jwt_tool"
    jwt_tool x >/dev/null 2>&1 || true
}

case "${1:-}" in
    "" | mcp | serve)
        init_jwt_keys
        exec marq serve
        ;;
esac

exec "$@"
