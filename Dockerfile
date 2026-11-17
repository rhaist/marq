# Passive-OSINT suite on a Kali rolling base, exposed as an MCP server over stdio.
# Every wrapped tool queries public third-party sources only — no scanners,
# exploitation or cracking tools are installed.
#
# Build:  docker build -t marq .
# Run  :  docker run --rm -i marq            (stdio MCP server)
#
# The Go build toolchain is kept out of the runtime image (built in a separate
# stage; golang-go purged in-layer). See docs/SECURITY.md before running.

# --- Stage 1: build the Go MCP server / agent-host binary ------------------
# Built in an isolated golang stage and copied in as a static binary, so the
# Kali image carries no Go build toolchain for the server itself. Runs on the
# native BUILDPLATFORM and cross-compiles to TARGETARCH (CGO off) — fast on both
# Apple Silicon (arm64) and amd64 hosts, no emulation for the Go build.
FROM --platform=$BUILDPLATFORM golang:1.27.3 AS gobuild
ARG TARGETOS TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY skills.go ./
COPY skills/ ./skills/
COPY pi/ ./pi/
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
# Kept explicit (rather than a kali-linux-* metapackage) so the image is
# auditable and reproducible. git is for trufflehog's `git` source.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl git \
        python3 python3-pip python3-venv \
        # DNS / registry
        bind9-dnsutils whois subfinder dnsx \
        # company & domain footprint
        theharvester spiderfoot libimage-exiftool-perl trufflehog \
        # people footprint
        sherlock h8mail \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Go-built tools not packaged in Kali apt: the archive harvester + CDN
# classifier. Installed system-wide (GOBIN -> /usr/local/bin) so the
# dropped-privilege user can run them. golang-go is a *build-time only* dep, so
# it is installed, used, and purged within this single layer — it never persists
# in the image (a separate layer can't shrink an earlier one).
ENV GOBIN=/usr/local/bin GOPATH=/root/go
RUN apt-get update \
    && apt-get install -y --no-install-recommends golang-go \
    && go install github.com/lc/gau/v2/cmd/gau@latest \
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
    && curl -fsSL "https://github.com/sundowndev/phoneinfoga/releases/download/v2.11.0/phoneinfoga_Linux_${PA}.tar.gz" \
        | tar -xz -C /usr/local/bin phoneinfoga \
    && chmod +x /usr/local/bin/phoneinfoga

# cencli (the `censys` CLI for the Censys Platform API; Legacy Search, which the
# python3-censys CLI used, was decommissioned 30 Sep 2026). Asset names match
# TARGETARCH directly.
RUN curl -fsSL "https://github.com/censys/cencli/releases/download/v1.2.1/cencli_1.2.1_linux_${TARGETARCH:-amd64}.tar.gz" \
        | tar -xz -C /usr/local/bin censys \
    && chmod +x /usr/local/bin/censys

# --- Python OSINT tools (no MCP SDK — the server is the Go binary) ---------
# venv holds pip-only tools: maigret, user-scanner, shodan. user-scanner is
# pinned — a fast-moving single-maintainer project.
RUN python3 -m venv --system-site-packages "$VIRTUAL_ENV" \
    && pip install --no-cache-dir maigret user-scanner==1.5.2.5 \
    # user-scanner would check PyPI and prompt for updates on every call
    && sed -i 's/"auto_update_status": true/"auto_update_status": false/' \
        "$VIRTUAL_ENV"/lib/python3*/site-packages/user_scanner/config.json \
    # shodan CLI imports pkg_resources, removed in setuptools 82 (Kali dropped
    # python3-pkg-resources), so it lives here with an older setuptools.
    # --ignore-installed: apt pulls python3-shodan in as a dependency, and pip
    # would otherwise skip it and leave the broken /usr/bin/shodan on PATH.
    && pip install --no-cache-dir --ignore-installed shodan 'setuptools<82'

# --- Go MCP server / agent-host binary -------------------------------------
COPY --from=gobuild /out/marq /usr/local/bin/marq
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# --- Hardening: drop to a non-root user -----------------------------------
RUN useradd --create-home --shell /bin/bash marq \
    && mkdir -p /var/log/marq /work \
    && chown -R marq:marq /var/log/marq /work
USER marq
ENV MARQ_AUDIT_LOG=/var/log/marq/audit.jsonl \
    MARQ_OPERATOR=unknown \
    MARQ_ENGAGEMENT=unspecified

# Tools that drop output in the cwd must land in the writable engagement dir,
# not "/" — the runner inherits this cwd and "/" is not writable by the marq user.
WORKDIR /work

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
