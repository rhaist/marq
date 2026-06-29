# Setup

Install + run steps for **macOS** (Apple Silicon or Intel) and **Debian Testing**
(rolling). The container (Kali tools + the `marq` Go binary) is identical on both;
the only OS-specific part is installing Docker.

marq is your **universal cyber assistant** — it brings the tools and the
knowledge; your client brings the model. After building the image, see
[`CLIENTS.md`](CLIENTS.md) to pick a client (Claude Code / Codex as the expert
brain, Pi for fully-local uncensored work, LM Studio for testing). This page is
just install + the two connection methods.

> Advisory and knowledge work is open; **active testing is authorized-only** —
> read [`SECURITY.md`](SECURITY.md) first.

**Contents**

- [Two ways to connect](#two-ways-to-connect)
- [macOS](#macos)
- [Debian Testing (rolling)](#debian-testing-rolling)
- [Local Go development](#local-go-development-both-oses)
- [Verify](#verify)

## Two ways to connect

| Method              | Command                       | Clients                                         | Who drives the model                               |
| ------------------- | ----------------------------- | ----------------------------------------------- | -------------------------------------------------- |
| **MCP server**      | `marq serve` (default)        | Claude Code, Codex, LM Studio, Claude Desktop   | The client brings its own model                    |
| **Direct run (Pi)** | `pi/marq` shim + `marq run …` | [Pi](https://pi.dev/) (local/abliterated model) | Pi drives the model; it calls `marq run` from bash |

The image is the same for both. Do the **build** once, then use the **MCP server**
command below with any client (per-client setup is in [`CLIENTS.md`](CLIENTS.md)),
and/or the **local model (Pi)** subsection.

---

## macOS

### 1. Prerequisites

```bash
# Docker Desktop
brew install --cask docker        # then launch Docker.app once and let it start

# Optional, only for local Go dev (not needed to run the image)
brew install go
```

### 2. Build the image

```bash
# Pull the prebuilt image (published from CI) and tag it `marq` so the rest of
# these docs work unchanged. NOTE: it's amd64 — on M-series it runs under
# emulation (slower); build from source below for a native arm64 image.
docker pull ghcr.io/rhaist/marq && docker tag ghcr.io/rhaist/marq marq

# …or build from source (native arch):
git clone <this-repo> marq && cd marq
docker build -t marq .     # builds natively for your arch (arm64 on M-series)
# Smaller image (skips warm-up; nuclei templates / wpscan DB / trivy DB fetched on first use):
docker build --build-arg WARMUP=0 -t marq .
```

### 3. Run as an MCP server (external client brings the model)

```bash
docker run --rm -i \
  --security-opt no-new-privileges:true --cap-drop ALL \
  --cap-add NET_RAW --cap-add NET_ADMIN --cap-add NET_BIND_SERVICE \
  -v marq-audit:/var/log/marq \
  -v marq-work:/work \
  -e MARQ_OPERATOR=marq \
  marq
```

Engagement and scope aren't passed here — the model records them at runtime with
the `set_engagement` tool (from the operator's written authorization). The
`marq-work` volume persists findings/reports across restarts; swap it for a bind
mount (`-v ~/engagements/acme:/work`) to read them straight off the host.

Wire it into a client with [`mcp.json.example`](../mcp.json.example) — per-client
steps (Claude Code, Codex, LM Studio) are in [`CLIENTS.md`](CLIENTS.md).

### 4. Run with a local model (Pi + the marq skill)

[Pi](https://pi.dev/) is a minimal terminal agent that runs a local/abliterated
model over an OpenAI-compatible endpoint (LM Studio / Ollama / llama-server) and
gives the model bash. The model reaches marq's tools through the `pi/marq` host
shim, which forwards each call into a long-lived container over `docker exec`.

```bash
# Install the shim
cp pi/marq /usr/local/bin/marq && chmod +x /usr/local/bin/marq

# Start ONE long-lived container, bound to your engagement dir
marq up ~/engagements/acme        # docker run -d … sleep infinity
marq tools                        # list every tool (sanity check)
marq run server_info '{}'         # confirm scope
marq down                         # tear down when finished
```

Then **configure your model runtime in Pi** (LM Studio / Ollama / llama-server —
Pi owns the endpoint, that's no longer marq's concern) and load
[`pi/SKILL.md`](../pi/SKILL.md) into Pi as a skill so the model knows the calling
convention and scope rules. The model then calls `marq run <tool> '<json>'` from
bash and the shim runs it inside the container.

A long-lived container is **required**: background tools (spiderfoot, responder,
ntlmrelayx) detach inside it and are polled later, so a per-call `docker run`
would kill them.

Shim env vars: `MARQ_CONTAINER` (default `marq`), `MARQ_IMAGE` (default
`marq:latest`), `MARQ_ENV_FILE` (optional env-file for `MARQ_OPERATOR` plus API
keys, passed as `--env-file` — copy `.env.example` to `.env` for a template;
engagement + scope are set at runtime via `set_engagement`).

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
git clone <this-repo> marq && cd marq
docker build -t marq .          # native amd64 (or arm64 on ARM boards)
```

### 3. Run as an MCP server (external client brings the model)

Same as macOS step 3 above — identical command.

### 4. Run with a local model (Pi + the marq skill)

Same as macOS step 4 — install the `pi/marq` shim, `marq up <dir>`, and drive it
from Pi with [`pi/SKILL.md`](../pi/SKILL.md) loaded. Configure the model runtime
(Ollama is the practical choice on a headless box) in Pi, not in marq.

> **Bind-mount permissions (Linux).** The container runs as the non-root `marq`
> user, so the engagement dir you pass to `marq up` must be writable by it —
> `chmod a+rwx <dir>` is the simplest. macOS Docker Desktop handles this
> automatically.

---

## Local Go development (both OSes)

Iterate on the server/tool layer without a Docker build (tool _calls_ still need
the Kali binaries, so most only fully work in-container):

```bash
go build ./... && go vet ./... && go test ./...   # build + sanity gate
go run ./cmd/marq serve                         # stdio MCP server locally
go run ./cmd/marq run nmap '{"target":"scanme.nmap.org"}'   # invoke one tool
```

## Verify

```bash
# Tool present in the built image. nmap/masscan/naabu have file capabilities
# (cap_net_admin) a bare container won't exec — pass the caps, or smoke a
# non-capped tool:
docker run --rm --cap-add NET_RAW --cap-add NET_ADMIN marq nmap --version
docker run --rm marq nuclei -version

# MCP server lists its tools (expects ~80)
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"c","version":"0"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
 | docker run --rm -i marq | grep -o '"name":"[a-z_]*"' | wc -l

# List every tool with a one-line description (also how the model discovers them):
docker run --rm marq tools
```

The full end-to-end harness is `scripts/verify_tools.sh` (drives the built image
over MCP).
