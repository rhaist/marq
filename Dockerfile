# Full pentesting suite on a Kali base, exposed as an MCP server over stdio.
#
# Build:  docker build -t pentest-mcp .
# Run  :  docker run --rm -i pentest-mcp            (stdio MCP server)
#
# The image is large (multi-GB) because it bundles the full tool suite
# (metasploit, hashcat, sqlmap, etc.). See docs/SECURITY.md before running.

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
    go build -trimpath -ldflags="-s -w" -o /out/pentest ./cmd/pentest

# --- Stage 2: the Kali pentest image ---------------------------------------
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
        nmap masscan bind9-dnsutils whois subfinder nuclei httpx-toolkit \
        naabu dnsx dnsrecon fierce \
        # web app  (dalfox is not in apt — installed via go below)
        nikto ffuf gobuster whatweb wpscan sqlmap \
        feroxbuster arjun wafw00f cmseek testssl.sh sslscan gospider \
        # osint / information gathering — company & domain footprint
        # (python3-pkg-resources: shodan CLI imports pkg_resources at runtime)
        theharvester spiderfoot recon-ng libimage-exiftool-perl \
        python3-shodan python3-censys python3-pkg-resources gitleaks trufflehog \
        # osint — people footprint
        sherlock h8mail \
        # exploitation
        metasploit-framework hydra exploitdb \
        # credentials / hashes (mesa-opencl-icd gives hashcat a CPU OpenCL device)
        john hashcat hashid mesa-opencl-icd ocl-icd-libopencl1 \
        # wordlists
        wordlists \
    && rm -rf /var/lib/apt/lists/*

# Go-built tools not packaged in Kali apt: active crawler + archive harvesters
# + XSS scanner. Installed system-wide so the dropped-privilege user can run
# them (GOBIN puts the binaries in /usr/local/bin).
ENV GOBIN=/usr/local/bin GOPATH=/root/go
RUN go install github.com/projectdiscovery/katana/cmd/katana@latest \
    && go install github.com/lc/gau/v2/cmd/gau@latest \
    && go install github.com/tomnomnom/waybackurls@latest \
    && go install github.com/hahwul/dalfox/v2@latest \
    && rm -rf /root/go /root/.cache/go-build

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

# --- Python OSINT tools (no MCP SDK — the server is the Go binary) ----------
# The venv still holds the pip-only OSINT tools used by the people wrappers
# (maigret = deep username sweep, holehe = email-account discovery).
RUN python3 -m venv "$VIRTUAL_ENV" \
    && pip install --no-cache-dir maigret holehe

# --- Go MCP server / agent-host binary -------------------------------------
COPY --from=gobuild /out/pentest /usr/local/bin/pentest
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# --- Hardening: drop to a non-root user -----------------------------------
RUN useradd --create-home --shell /bin/bash pentester \
    && mkdir -p /var/log/pentest-mcp /work \
    && chown -R pentester:pentester /var/log/pentest-mcp /work
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
