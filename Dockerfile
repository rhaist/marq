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
        ca-certificates curl wget git libcap2-bin golang-go \
        python3 python3-pip python3-venv \
        # recon / network
        nmap masscan dnsutils whois subfinder nuclei httpx-toolkit \
        naabu dnsx amass dnsrecon fierce \
        # web app
        nikto ffuf gobuster whatweb wpscan sqlmap \
        feroxbuster arjun dalfox wafw00f cmseek testssl.sh sslscan gospider \
        # osint / information gathering — company & domain footprint
        theharvester spiderfoot recon-ng libimage-exiftool-perl \
        python3-shodan python3-censys gitleaks trufflehog \
        # osint — people footprint
        sherlock h8mail \
        # exploitation
        metasploit-framework hydra exploitdb \
        # credentials / hashes (pocl gives hashcat a CPU OpenCL device)
        john hashcat hashid pocl-opencl-icd \
        # wordlists
        wordlists \
    && rm -rf /var/lib/apt/lists/*

# Go-built tools not packaged in Kali apt: active crawler + archive harvesters
# + phone OSINT. Installed system-wide so the dropped-privilege user can run them.
ENV GOBIN=/usr/local/bin GOPATH=/root/go
RUN go install github.com/projectdiscovery/katana/cmd/katana@latest \
    && go install github.com/lc/gau/v2/cmd/gau@latest \
    && go install github.com/tomnomnom/waybackurls@latest \
    && go install github.com/sundowndev/phoneinfoga/v2@latest \
    && rm -rf /root/go /root/.cache/go-build

# Allow unprivileged SYN scans (nmap/masscan/naabu) without running as root.
RUN setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip /usr/bin/nmap || true \
    && setcap cap_net_raw,cap_net_admin+eip /usr/bin/masscan || true \
    && setcap cap_net_raw,cap_net_admin+eip /usr/bin/naabu || true

# Kali ships rockyou gzipped; decompress so the canonical path that john/hashcat/
# hydra/wordlist tooling expect (/usr/share/wordlists/rockyou.txt) exists.
RUN gunzip -f /usr/share/wordlists/rockyou.txt.gz 2>/dev/null || true

# --- MCP server + venv-installed OSINT tools -------------------------------
RUN python3 -m venv "$VIRTUAL_ENV"
COPY requirements.txt /tmp/requirements.txt
# Server SDK plus Python OSINT tools not in apt (deep username sweep,
# email-account discovery) — into the venv, on PATH.
RUN pip install -r /tmp/requirements.txt \
    && pip install --no-cache-dir maigret holehe

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

# --- Build-time tool warm-up (runs AS pentester) ---------------------------
# A few tools fetch data or build a cache on first use, stored in the user's
# HOME. Doing it here (as the runtime user, so it lands in /home/pentester)
# moves that cost off the first live scan and removes a runtime network
# dependency. Steps, in order below:
#   1. nuclei      pre-fetch the template repository (~thousands of templates)
#   2. wpscan      pre-download the WordPress vulnerability database
#   3. metasploit  build the module cache (first msfconsole is otherwise slow)
# Each is best-effort (|| true): a build-host network hiccup must not fail the
# image — the tool falls back to its own first-run download.
RUN (nuclei -update-templates -silent 2>/dev/null || nuclei -update-templates 2>/dev/null || true) \
    && (wpscan --update 2>/dev/null || true) \
    && (msfconsole -q -x "version; exit" 2>/dev/null || true)

ENV PENTEST_MCP_AUDIT_LOG=/var/log/pentest-mcp/audit.jsonl \
    PENTEST_MCP_OPERATOR=unknown \
    PENTEST_MCP_ENGAGEMENT=unspecified

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
