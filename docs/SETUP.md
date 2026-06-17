# Setup

Full install + run steps for **macOS** (Apple Silicon or Intel) and **Debian
Testing** (rolling). The container (Kali tools + the `pentest` Go binary) is
identical on both; the only OS-specific part is how the local model runtime is
installed and how the container reaches it.

> Authorized testing only. Read [`SECURITY.md`](SECURITY.md) first.

## Two ways to run

| Mode | Command | Needs a local model? | Who drives the model |
|------|---------|----------------------|----------------------|
| **MCP server** | `pentest serve` (default) | No | An external MCP client (Claude Desktop, LM Studio) brings its own |
| **Agent host / TUI** | `pentest tui` / `pentest agent "<task>"` | **Yes** (Ollama on the host) | The binary's own tool-calling loop |

The image is the same. Pick a section below for your OS; do the **common build**
once, then the **MCP server** and/or **agent/TUI** subsections.

---

## macOS

### 1. Prerequisites
```bash
# Docker Desktop (provides `host.docker.internal` automatically)
brew install --cask docker        # then launch Docker.app once and let it start

# Optional, only for local Go dev (not needed to run the image)
brew install go
```

### 2. Build the image
```bash
git clone <this-repo> pentest-mcp && cd pentest-mcp
docker build -t pentest-mcp .     # builds natively for your arch (arm64 on M-series)
# Smaller image (skips warm-up; nuclei templates / msf cache fetched on first use):
docker build --build-arg WARMUP=0 -t pentest-mcp .
```

### 3. Run as an MCP server (external client brings the model)
```bash
docker run --rm -i \
  --security-opt no-new-privileges:true --cap-drop ALL \
  --cap-add NET_RAW --cap-add NET_ADMIN --cap-add NET_BIND_SERVICE \
  -v pentest-mcp-audit:/var/log/pentest-mcp \
  -e PENTEST_MCP_OPERATOR=your-name -e PENTEST_MCP_ENGAGEMENT=acme-2026 \
  -e PENTEST_MCP_SCOPE="*.example.com — per SOW" \
  pentest-mcp
```
Or wire it into LM Studio / Claude Desktop with [`mcp.json.example`](../mcp.json.example).

### 4. Run the agent host / TUI (local model)
```bash
# Install + start Ollama, pull an uncensored tool-calling model
brew install ollama
ollama serve >/dev/null 2>&1 &                       # or run the Ollama app
ollama pull huihui_ai/Qwen3.6-abliterated:27b        # ~17 GB, fits 24 GB unified

mkdir -p work
docker run --rm -it \
  -e PENTEST_MCP_MODEL_URL=http://host.docker.internal:11434/v1 \
  -e PENTEST_MCP_MODEL=huihui_ai/Qwen3.6-abliterated:27b \
  -e PENTEST_MCP_OPERATOR=your-name -e PENTEST_MCP_ENGAGEMENT=acme-2026 \
  -e PENTEST_MCP_SCOPE="*.example.com — per SOW" \
  -v "$PWD/work:/work" \
  pentest-mcp tui
```
On Docker Desktop, `host.docker.internal` resolves to the host, so Ollama bound
to its default `127.0.0.1:11434` is reachable from the container as-is.

---

## Debian Testing (rolling)

### 1. Prerequisites
```bash
# Docker engine + buildx (Testing ships a recent docker.io with BuildKit)
sudo apt update && sudo apt install -y docker.io docker-buildx git curl
sudo usermod -aG docker "$USER"        # log out/in (or: newgrp docker) so this takes effect
sudo systemctl enable --now docker

# Optional, only for local Go dev
sudo apt install -y golang
```
(Alternatively use Docker's official CE repo; `docker.io` from Testing is fine.)

### 2. Build the image
```bash
git clone <this-repo> pentest-mcp && cd pentest-mcp
docker build -t pentest-mcp .          # native amd64 (or arm64 on ARM boards)
```

### 3. Run as an MCP server (external client brings the model)
Same as macOS step 3 above — identical command.

### 4. Run the agent host / TUI (local model)
```bash
# Install Ollama (installs a systemd service listening on 127.0.0.1:11434)
curl -fsSL https://ollama.com/install.sh | sh
ollama pull huihui_ai/Qwen3.6-abliterated:27b
```

**Networking — the one real difference from macOS.** Linux containers do *not*
get `host.docker.internal` for free and the host's `127.0.0.1` is not the
container's. Two clean options:

**Option A — `--network=host` (recommended for a pentest box).** The container
shares the host network namespace, so Ollama on `127.0.0.1:11434` is reached
directly *and* the scanning tools get unmediated network access to targets.
```bash
mkdir -p work && chmod a+rwx work      # so the container's non-root user can write findings
docker run --rm -it --network=host \
  -e PENTEST_MCP_MODEL_URL=http://localhost:11434/v1 \
  -e PENTEST_MCP_MODEL=huihui_ai/Qwen3.6-abliterated:27b \
  -e PENTEST_MCP_OPERATOR=your-name -e PENTEST_MCP_ENGAGEMENT=acme-2026 \
  -e PENTEST_MCP_SCOPE="*.example.com — per SOW" \
  -v "$PWD/work:/work" \
  pentest-mcp tui
```

**Option B — bridge networking with `host-gateway`.** Keep the container on its
own network and add a host alias. Ollama must then listen on an interface the
bridge can reach, so set `OLLAMA_HOST=0.0.0.0` (⚠️ this exposes Ollama on all
host interfaces — restrict with a firewall):
```bash
sudo systemctl edit ollama     # add:  [Service]  Environment="OLLAMA_HOST=0.0.0.0:11434"
sudo systemctl restart ollama

mkdir -p work && chmod a+rwx work
docker run --rm -it --add-host=host.docker.internal:host-gateway \
  -e PENTEST_MCP_MODEL_URL=http://host.docker.internal:11434/v1 \
  -e PENTEST_MCP_MODEL=huihui_ai/Qwen3.6-abliterated:27b \
  -v "$PWD/work:/work" \
  pentest-mcp tui
```

> **Bind-mount permissions (Linux).** The container runs as the non-root
> `pentester` user, so a bind-mounted `./work` must be writable by it —
> `chmod a+rwx work` (shown above) is the simplest. macOS Docker Desktop handles
> this automatically. Alternatively use a named volume (`-v pentest-work:/work`)
> and copy results out with `docker cp`.

---

## Local Go development (both OSes)

Iterate on the server/tool layer without a Docker build (tool *calls* still need
the Kali binaries, so most only fully work in-container):
```bash
go build ./... && go vet ./... && go test ./...   # build + sanity gate
go run ./cmd/pentest serve                         # stdio MCP server locally
go run ./cmd/pentest run nmap '{"target":"scanme.nmap.org"}'   # invoke one tool
```

## Verify

```bash
# Tool present in the built image
docker run --rm pentest-mcp nmap --version

# MCP server lists its tools (expects ~52)
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"c","version":"0"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
 | docker run --rm -i pentest-mcp | grep -o '"name":"[a-z_]*"' | wc -l

# Validate the model does reliable multi-tool calling (needs Ollama running):
docker run --rm -i <networking flags for your OS> \
  -e PENTEST_MCP_MODEL_URL=... -e PENTEST_MCP_MODEL=... \
  pentest-mcp agent "resolve and port-scan scanme.nmap.org, then summarize"
```
The full end-to-end harness is `scripts/verify_tools.sh` (drives the built image
over MCP).
```
