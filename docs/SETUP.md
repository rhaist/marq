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
git clone https://github.com/rhaist/marq.git && cd marq
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
gives the model bash. This is the fully-local way to drive marq as an all-round
cyber agent — research, malware triage, threat-intel, GRC and standards work, and
authorized testing — through the `pi/marq` host shim, which forwards each call
into a long-lived container over `docker exec`.

```bash
# Put the shim on your PATH in a user-owned dir (don't pollute Homebrew's prefix
# or need sudo). ~/.local/bin is ideal — ensure it's on PATH:
mkdir -p ~/.local/bin
install -m 0755 pi/marq ~/.local/bin/marq     # copy; or, to track the repo:
# ln -sf "$PWD/pi/marq" ~/.local/bin/marq      # symlink — updates with the repo

# Start ONE long-lived container, bound to a workspace dir (whatever you're
# working on — a research folder, a sample dir, a compliance project, a test):
marq up ~/marq/work               # docker run -d … sleep infinity
marq tools                        # list every tool (sanity check)
marq run server_info '{}'         # what marq covers + current authorization state
marq down                         # tear down when finished
```

The model reaches marq through **bash** (`marq run <tool> '<json>'`), not MCP —
so all Pi needs is the `marq` skill loaded and a model. Configure Pi once
(verified against Pi 0.80):

**1. Point Pi at your local model** — Pi configures providers via an extension.
For LM Studio, drop this at `~/.pi/agent/extensions/lm-studio.ts` (auto-discovered):

```typescript
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
const BASE = "http://localhost:1234";
export default async function (pi: ExtensionAPI) {
  let models: any[] = [];
  try {
    const data =
      ((await (await fetch(`${BASE}/api/v0/models`)).json()) as { data: any[] })
        .data ?? [];
    models = data
      .filter((m) => m.type === "llm" || m.type === "vlm")
      .map((m) => ({
        id: m.id,
        name: m.id,
        reasoning: false,
        input: ["text"],
        cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
        contextWindow: m.loaded_context_length ?? m.max_context_length ?? 32768,
        maxTokens: 4096,
      }));
  } catch {}
  pi.registerProvider("lm-studio", {
    name: "LM Studio (local)",
    baseUrl: `${BASE}/v1`,
    apiKey: "lm-studio",
    api: "openai-completions",
    models,
  });
}
```

LM Studio's native `/api/v0/models` (vs the OpenAI `/v1/models`) reports the
real loaded context length, so Pi sizes the window correctly. For Ollama/
llama-server, change `BASE` and the discovery call.

**2. Register the marq skill + model defaults** in `~/.pi/agent/settings.json`:

```json
{
  "defaultProvider": "lm-studio",
  "defaultModel": "lm-studio/gemma4-12b-qat-uncensored-hauhaucs-balanced",
  "skills": ["/path/to/marq/pi"]
}
```

Pi discovers any directory containing a `SKILL.md` (recursively), so pointing
`skills` at the repo's [`pi/`](../pi/) dir registers [`pi/SKILL.md`](../pi/SKILL.md)
as the `marq` skill — kept in sync with the repo, no copy.

**Recommended model:** [`Gemma4-12B-QAT-Uncensored-HauhauCS-Balanced`](https://huggingface.co/HauhauCS/Gemma4-12B-QAT-Uncensored-HauhauCS-Balanced)
(Q4_K_M) — the default above. It's uncensored (no refusals on offensive work),
tool-capable, and fits a 12–16 GB GPU at a long context. It needs the LM Studio
settings below to behave; without them it leaks reasoning tokens into its
answers and (without SYSTEM.md) bypasses marq.

**2a. LM Studio settings for Gemma** (in the model's right-sidebar config):

- **Reasoning parsing** — Gemma emits its thinking inside `<|channel>thought …
<channel|>` markers; unset, LM Studio leaks those into the reply. Enable
  reasoning parsing and set **Start String** `<|channel>thought`, **End String**
  `<channel|>` so the block is split out of the final answer.
  (Background: [enabling Gemma thinking mode in LM Studio](https://antonioleiva.com/enable-gemma-thinking-mode-lm-studio-opencode).)
- **Sampling** (Gemma's recommended, what marq's evals run under): temperature
  `0.6`, top_p `0.9`, top_k `64`, min_p `0.05`, repeat_penalty `1.1`. These also
  live in [`scripts/eval/profiles.json`](../scripts/eval/profiles.json).
- **Context length** — load it as high as VRAM allows (the model is 262 K
  native). Tool outputs (nuclei/katana dumps) are large; a short window truncates
  them. Pi reads the loaded length from LM Studio's `/api/v0/models` and sizes its
  window to match.

Impact order if it misbehaves: **SYSTEM.md (2b) ≫ context length ≫ reasoning
parsing ≫ sampling.** SYSTEM.md decides whether it uses marq at all; the rest is
output quality.

**2b. Replace Pi's system prompt with marq's** (important for smaller models):

```bash
ln -sf "$PWD/pi/SYSTEM.md" ~/.pi/agent/SYSTEM.md   # tracks the repo
```

Pi's default prompt frames the model as a general coding assistant, so a small
model reaches for raw `curl`/`grep` and bypasses marq entirely (no audit, no
scope, no skills). [`pi/SYSTEM.md`](../pi/SYSTEM.md) replaces that framing —
"drive everything through `marq run`, never raw bash" — which in testing flipped
a 12B model from 300+ `curl` calls and zero marq use to clean, audited marq tool
calls. (`~/.pi/agent/SYSTEM.md` is global; use a per-project `.pi/SYSTEM.md` if
you also run Pi for non-marq work.) The `pi/SKILL.md` catalog is still appended.

**3. Verify** (no model call, so it won't disturb anything):

```bash
pi --list-models                  # → lists `lm-studio  qwen3-…  41.0K …`
```

Then just run `pi` in an engagement dir. The model loads the marq skill and
drives `marq run` from bash; the shim runs it inside the container.

> **Gotchas learned the hard way:** settings live in `~/.pi/agent/` (not
> `~/.pi/`); extensions auto-load only from `~/.pi/agent/extensions/*.ts` (the
> settings `extensions` array needs full paths, not bare names); Pi's bundled
> docs are at `…/pi-coding-agent/<ver>/libexec/.../docs/`.

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
git clone https://github.com/rhaist/marq.git && cd marq
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
