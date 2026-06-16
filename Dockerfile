# Full pentesting suite on a Kali base, exposed as an MCP server over stdio.
#
# Build:  docker build -t pentest-mcp .
# Run  :  docker run --rm -i pentest-mcp            (stdio MCP server)
#
# The image is large (multi-GB) because it bundles the full tool suite
# (metasploit, hashcat, sqlmap, etc.). See docs/SECURITY.md before running.

FROM kalilinux/kali-rolling

ENV DEBIAN_FRONTEND=noninteractive \
    PIP_NO_CACHE_DIR=1 \
    PYTHONUNBUFFERED=1 \
    VIRTUAL_ENV=/opt/venv \
    PATH=/opt/venv/bin:$PATH

# --- Pentest tooling -------------------------------------------------------
# Grouped roughly by category. Kept explicit (rather than kali-linux-everything)
# so the image is auditable and reproducible.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl wget git libcap2-bin \
        python3 python3-pip python3-venv \
        # recon / network
        nmap masscan dnsutils whois subfinder nuclei httpx-toolkit \
        # web app
        nikto ffuf gobuster whatweb wpscan sqlmap \
        # exploitation
        metasploit-framework hydra exploitdb \
        # credentials / hashes
        john hashcat hashid \
        # wordlists
        wordlists \
    && rm -rf /var/lib/apt/lists/*

# Allow unprivileged SYN scans (nmap/masscan) without running as root.
RUN setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip /usr/bin/nmap || true \
    && setcap cap_net_raw,cap_net_admin+eip /usr/bin/masscan || true

# --- MCP server ------------------------------------------------------------
RUN python3 -m venv "$VIRTUAL_ENV"
COPY requirements.txt /tmp/requirements.txt
RUN pip install -r /tmp/requirements.txt

WORKDIR /app
COPY server/ /app/server/
COPY pyproject.toml /app/pyproject.toml
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# --- Hardening: drop to a non-root user -----------------------------------
RUN useradd --create-home --shell /bin/bash pentester \
    && mkdir -p /var/log/pentest-mcp /work \
    && chown -R pentester:pentester /var/log/pentest-mcp /work /app
USER pentester
WORKDIR /work

ENV PENTEST_MCP_AUDIT_LOG=/var/log/pentest-mcp/audit.jsonl \
    PENTEST_MCP_OPERATOR=unknown \
    PENTEST_MCP_ENGAGEMENT=unspecified

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
