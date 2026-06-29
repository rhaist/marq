# Pentesting suite on a Kali rolling base, exposed as an MCP server over stdio.
#
# Build:  docker build -t marq .
#         docker build --build-arg WARMUP=0 -t marq .   (smaller image,
#         skips pre-fetching nuclei templates / wpscan DB; downloaded on first use)
# Run  :  docker run --rm -i marq            (stdio MCP server)
#
# Metasploit was removed: its 1.8 GB mingw/postgres/networkx subtree was not
# justified for a non-interactive MCP wrapper. Payload generation is covered
# by donut (4.5 MB). The Go build toolchain is kept out of the runtime image
# (built in a separate stage; golang-go purged in-layer).
# See docs/SECURITY.md before running.

# --- Stage 1: build the Go MCP server / agent-host binary ------------------
# Built in an isolated golang stage and copied in as a static binary, so the
# Kali image carries no Go build toolchain for the server itself. Runs on the
# native BUILDPLATFORM and cross-compiles to TARGETARCH (CGO off) — fast on both
# Apple Silicon (arm64) and amd64 hosts, no emulation for the Go build.
FROM --platform=$BUILDPLATFORM golang:1.26.4 AS gobuild
ARG TARGETOS TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/marq ./cmd/marq

# --- Stage 2: the Kali marq image ---------------------------------------
FROM kalilinux/kali-rolling

ENV DEBIAN_FRONTEND=noninteractive \
    PIP_NO_CACHE_DIR=1 \
    PYTHONUNBUFFERED=1 \
    VIRTUAL_ENV=/opt/venv \
    PATH=/opt/venv/bin:$PATH

# --- Marq tooling -------------------------------------------------------
# Grouped roughly by category. Kept explicit (rather than kali-linux-everything)
# so the image is auditable and reproducible. Metasploit was removed: its
# 1.8 GB mingw/postgres/networkx subtree was not justified for a non-interactive
# MCP wrapper. donut (4.5 MB) replaces msfvenom for payload/shellcode generation.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl wget git libcap2-bin unzip \
        python3 python3-pip python3-venv \
        # recon / network
        nmap masscan bind9-dnsutils whois subfinder nuclei httpx-toolkit \
        naabu dnsx dnsrecon fierce \
        fping ssh-audit \
        snmp snmpcheck onesixtyone braa \
        smtp-user-enum swaks \
        smbmap smbclient enum4linux-ng nbtscan \
        # web app  (dalfox is not in apt — installed via go below)
        nikto ffuf gobuster whatweb wpscan sqlmap \
        feroxbuster arjun wafw00f cmseek testssl.sh sslscan gospider \
        subjack paramspider sstimap \
        trivy \
        # osint / information gathering — company & domain footprint
        # (python3-pkg-resources: shodan CLI imports pkg_resources at runtime)
        theharvester spiderfoot recon-ng libimage-exiftool-perl \
        python3-shodan python3-censys python3-pkg-resources gitleaks trufflehog \
        # osint — people footprint
        sherlock h8mail \
        # exploitation (metasploit removed — searchsploit + hydra + donut cover the gap)
        exploitdb hydra \
        # AD / internal network (replaces metasploit)
        python3-impacket impacket-scripts netexec certipy-ad \
        bloodhound.py evil-winrm ldap-utils responder python3-netifaces \
        # payload generation (replaces msfvenom — 4.5 MB vs 1.8 GB)
        golang-github-binject-go-donut \
        # malware research — static analysis (capa/floss/oletools come via pip below)
        yara radare2 \
        # credentials / hashes (mesa-opencl-icd gives hashcat a CPU OpenCL device)
        john hashcat hashid mesa-opencl-icd ocl-icd-libopencl1 \
        # wordlists
        wordlists \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Go-built tools not packaged in Kali apt: active crawler + archive harvesters
# + XSS scanner. Installed system-wide (GOBIN -> /usr/local/bin) so the
# dropped-privilege user can run them. golang-go is a *build-time only* dep, so
# it is installed, used, and purged within this single layer — it never persists
# in the image (a separate layer can't shrink an earlier one).
ENV GOBIN=/usr/local/bin GOPATH=/root/go
RUN apt-get update \
    && apt-get install -y --no-install-recommends golang-go \
    && go install github.com/projectdiscovery/katana/cmd/katana@latest \
    && go install github.com/lc/gau/v2/cmd/gau@latest \
    && go install github.com/tomnomnom/waybackurls@latest \
    && go install github.com/hahwul/dalfox/v2@latest \
    && go install github.com/projectdiscovery/interactsh/cmd/interactsh-client@latest \
    && go install github.com/projectdiscovery/asnmap/cmd/asnmap@latest \
    && go install github.com/projectdiscovery/cdncheck/cmd/cdncheck@latest \
    && apt-get purge -y --auto-remove golang-go \
    && apt-get clean \
    && rm -rf /root/go /root/.cache/go-build /var/lib/apt/lists/*

# phoneinfoga can't be `go install`ed (its web client go:embeds built frontend
# assets that aren't in the module), so use the pinned prebuilt release binary.
# Arch-aware so the image builds on amd64 (Debian) and arm64 (Apple Silicon).
ARG TARGETARCH
RUN case "${TARGETARCH:-amd64}" in \
        amd64) PA=x86_64 ;; \
        arm64) PA=arm64 ;; \
        *)     PA=x86_64 ;; \
    esac \
    && curl -sSL "https://github.com/sundowndev/phoneinfoga/releases/download/v2.11.0/phoneinfoga_Linux_${PA}.tar.gz" \
        | tar -xz -C /usr/local/bin phoneinfoga \
    && chmod +x /usr/local/bin/phoneinfoga

# Allow unprivileged SYN scans (nmap/masscan/naabu) without running as root.
RUN setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip /usr/bin/nmap || true \
    && setcap cap_net_raw,cap_net_admin+eip /usr/bin/masscan || true \
    && setcap cap_net_raw,cap_net_admin+eip /usr/bin/naabu || true

# Kali ships rockyou gzipped; decompress so the canonical path that john/hashcat/
# hydra/wordlist tooling expect (/usr/share/wordlists/rockyou.txt) exists.
RUN gunzip -f /usr/share/wordlists/rockyou.txt.gz 2>/dev/null || true

# --- Python OSINT + malware tools (no MCP SDK — the server is the Go binary) -
# venv holds pip-only tools: OSINT (maigret, holehe) and maldoc analysis
# (oletools → olevba). capa is installed as a standalone binary below instead of
# pip — flare-capa hard-pins PyYAML==6.0.1, which has no Python-3.13 wheel and
# won't build from sdist (Cython-3 bug). floss is dropped: no linux-arm64 build,
# and it pulls flare-capa transitively (capa + `bin_headers -z` cover strings).
RUN python3 -m venv --system-site-packages "$VIRTUAL_ENV" \
    && pip install --no-cache-dir maigret holehe oletools

# capa (FLARE) static-capability analysis — official standalone binary, picked by
# arch (ships linux-arm64 + linux x86_64). The standalone bundles its rule set.
ARG TARGETARCH
RUN CAPA_VER=v9.4.0 \
    && case "$TARGETARCH" in \
         arm64) CAPA_ZIP="capa-${CAPA_VER}-linux-arm64.zip" ;; \
         *)     CAPA_ZIP="capa-${CAPA_VER}-linux.zip" ;; \
       esac \
    && curl -fsSL -o /tmp/capa.zip "https://github.com/mandiant/capa/releases/download/${CAPA_VER}/${CAPA_ZIP}" \
    && unzip -o /tmp/capa.zip -d /usr/local/bin/ \
    && chmod +x /usr/local/bin/capa \
    && rm /tmp/capa.zip

# jwt_tool: not in Kali apt or on PyPI — it's a standalone script. Clone it, pull
# its deps into the venv, and drop a `jwt_tool` launcher on PATH.
# ponytail: pinned to latest (no upstream release tags); pin a commit if repro matters.
RUN git clone --depth 1 https://github.com/ticarpi/jwt_tool /opt/jwt_tool \
    && pip install --no-cache-dir -r /opt/jwt_tool/requirements.txt \
    && printf '#!/bin/sh\nexec python3 /opt/jwt_tool/jwt_tool.py "$@"\n' > /usr/local/bin/jwt_tool \
    && chmod +x /usr/local/bin/jwt_tool

# --- Go MCP server / agent-host binary -------------------------------------
COPY --from=gobuild /out/marq /usr/local/bin/marq
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# --- Hardening: drop to a non-root user -----------------------------------
RUN useradd --create-home --shell /bin/bash marq \
    && mkdir -p /var/log/marq /work \
    && chown -R marq:marq /var/log/marq /work
USER marq
# jwt_tool bails on its very first run to write ~/.jwt_tool/jwtconf.ini (exit 1,
# token unprocessed). Seed it now (as marq, so it lands in /home/marq) — required
# init, not an optional cache, so it runs regardless of WARMUP.
RUN jwt_tool eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.x >/dev/null 2>&1 || true

# --- Build-time tool warm-up (runs AS marq) ---------------------------
# A few tools fetch data or build a cache on first use, stored in the user's
# HOME. Doing it here (as the runtime user, so it lands in /home/marq)
# moves that cost off the first live scan and removes a runtime network
# dependency. Steps, in order below:
#   1. nuclei      pre-fetch the template repository (~thousands of templates)
#   2. wpscan      pre-download the WordPress vulnerability database
#   3. trivy       pre-download the vulnerability database
# Each is best-effort (|| true): a build-host network hiccup must not fail the
# image — the tool falls back to its own first-run download.
#
# This warm-up (nuclei templates especially) is the largest variable bloat in the
# image. Build with `--build-arg WARMUP=0` to skip it for a much smaller image, at
# the cost of a slower first run that downloads these on demand (needs network).
ARG WARMUP=1
RUN if [ "$WARMUP" = "1" ]; then \
        (nuclei -update-templates -silent 2>/dev/null || nuclei -update-templates 2>/dev/null || true) ; \
        (wpscan --update 2>/dev/null || true) ; \
        (trivy image --download-db-only 2>/dev/null || true) ; \
    fi

ENV MARQ_AUDIT_LOG=/var/log/marq/audit.jsonl \
    MARQ_OPERATOR=unknown \
    MARQ_ENGAGEMENT=unspecified

# Tools that drop output in the cwd (bloodhound-python, certipy, paramspider,
# ntlmrelayx loot) must land in the writable engagement dir, not "/" — the
# runner inherits this cwd and "/" is not writable by the marq user.
WORKDIR /work

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
