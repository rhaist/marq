#!/bin/bash
# Entrypoint for the pentest MCP container.
#
# With no args (the default), launch the MCP stdio server. Any other args are
# executed verbatim so the image doubles as an interactive pentest shell:
#   docker run --rm -it --entrypoint /bin/bash pentest-mcp
#   docker run --rm pentest-mcp nmap --version
set -euo pipefail

# Initialise the Metasploit database on first interactive use if requested.
if [[ "${1:-}" == "mcp" || "$#" -eq 0 ]]; then
    exec python3 -m server.main
fi

exec "$@"
