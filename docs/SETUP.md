# Setup

Install and run steps for macOS (Apple Silicon or Intel) and Debian Testing
(rolling). marq is a universal cyber assistant: it brings the tools and
the knowledge, and your client brings the model.

Start with the "Pick a path" table below. The simplest option, the skills
library alone, is a single Go binary and needs no Docker at all. The full tool
suite runs from one image that's identical on both OSes; the only OS-specific part
is installing Docker. Once you're running, [`CLIENTS.md`](CLIENTS.md) helps you
pick a client (Claude Code / Codex for the heaviest reasoning; Pi + llama.cpp for
fully-local work).

> marq's tools are passive OSINT, and people OSINT is personal-data processing.
> Read [`SECURITY.md`](SECURITY.md) first.

**Contents**

- [Pick a path](#pick-a-path)
- [Skills only, no Docker](#skills-only-no-docker)
- [macOS](#macos)
- [Debian Testing (rolling)](#debian-testing-rolling)
- [Local Go development](#local-go-development-both-oses)
- [Verify](#verify)

## Pick a path

| Path                   | You get                                      | Needs                   | Setup                                                             |
| ---------------------- | -------------------------------------------- | ----------------------- | ----------------------------------------------------------------- |
| Skills only            | the knowledge library + findings/files tools | Go, no Docker           | [below](#skills-only-no-docker), one `go install`                |
| Full MCP server        | skills and the 24-tool OSINT suite           | Docker                  | build the image, then any MCP client                              |
| Full + local model     | the above, driven by a local model           | Docker + Pi + llama.cpp | the [Pi subsection](#4-run-with-a-local-model-pi--the-marq-skill) |

Start at the top and add a layer only when you need it. The image is the same for
the two full paths, so build it once. Per-client setup is in [`CLIENTS.md`](CLIENTS.md).

## Skills only, no Docker

To use the cyber-skills library and advisory tools (skills, findings, files) with
your LLM and without the OSINT tool suite, set `MARQ_SKILLS_ONLY=1`. It drops every
external-binary tool, so marq is a single static Go binary with no image and no
network lookups:

```bash
go install github.com/rhaist/marq/cmd/marq@latest    # ~10 MB, skills embedded
claude mcp add marq-skills \
  -e MARQ_SKILLS_ONLY=1 -e MARQ_WORK_DIR=$HOME/.marq \
  -e MARQ_AUDIT_LOG=$HOME/.marq/audit.jsonl \
  -- marq serve
```

Any MCP client works (the same `marq serve` stdio server with a smaller tool
set); see [`CLIENTS.md`](CLIENTS.md). Add the full tool suite below when you need
to _run_ something as well as reason about it. To try it with no client at all:
`scripts/demo-skills.sh`.

---

## macOS

### 1. Prerequisites

You need a container engine that provides the `docker` CLI. Docker Desktop is
not required (its licence is commercial for larger orgs), and a lighter,
open alternative is preferable:

```bash
# Recommended: OrbStack — fast, light, native Apple-Silicon, drop-in `docker` CLI
brew install orbstack

# Fully-FOSS alternative (macOS + Linux):
#   brew install colima docker && colima start
# Docker Desktop still works if you already run it.

# Optional, only for local Go dev (not needed to run the image)
brew install go
```

All three expose the same `docker` command, so every command below is unchanged.
If you prefer Podman, see the Debian note and set `MARQ_ENGINE=podman` for the `pi/marq`
shim.

### 2. Build the image

```bash
# Pull the prebuilt image (published from CI) and tag it `marq` so the rest of
# these docs work unchanged. Multi-arch (amd64 + arm64), so it's native on
# M-series too.
docker pull ghcr.io/rhaist/marq && docker tag ghcr.io/rhaist/marq marq

# …or build from source (native arch):
git clone https://github.com/rhaist/marq.git && cd marq
docker build -t marq .     # builds natively for your arch (arm64 on M-series)
```

### 3. Run as an MCP server (external client brings the model)

```bash
docker run --rm -i \
  --security-opt no-new-privileges:true --cap-drop ALL \
  -v marq-audit:/var/log/marq \
  -v marq-work:/work \
  -e MARQ_OPERATOR="$USER" \
  marq
```

The engagement and its purpose aren't passed here. The model records them at runtime with
the `set_engagement` tool (people OSINT is GDPR personal-data processing, so say why). The
`marq-work` volume persists findings/reports across restarts; swap it for a bind
mount (`-v ~/investigations/acme:/work`) to read them straight off the host.

Wire it into a client with [`mcp.json.example`](../mcp.json.example). Per-client
steps (Claude Code, Codex, Claude Desktop) are in [`CLIENTS.md`](CLIENTS.md).

### 4. Run with a local model (Pi + the marq skill)

[Pi](https://pi.dev/) is a minimal terminal agent that runs a local
model and gives it bash. We drive it with [llama.cpp](https://github.com/ggml-org/llama.cpp)'s
`llama-server`, the OpenAI-compatible local runtime. It has no GUI, is fully scriptable,
and is the layer the GUI wrappers sit on anyway (going direct gets you grammar-constrained
tool calls and reproducible flags). Pi talks to any OpenAI-compatible endpoint, so
vLLM or another server works too. The steps below assume `llama-server`.

This is the fully-local way to drive marq as an all-round cyber agent for research,
malware triage, threat-intel, GRC and standards work, and OSINT footprinting,
through the `pi/marq` host shim, which forwards each call into a long-lived
container over `docker exec`.

```bash
# Put the shim on your PATH in a user-owned dir (don't pollute Homebrew's prefix
# or need sudo). ~/.local/bin is ideal — ensure it's on PATH:
mkdir -p ~/.local/bin

# From the image — no checkout needed, and the shim always matches the marq it
# drives (a shim cloned from main can outrun the image tag you pulled):
docker run --rm marq shim > ~/.local/bin/marq && chmod +x ~/.local/bin/marq

# From a checkout instead:
# install -m 0755 pi/marq ~/.local/bin/marq   # copy
# ln -sf "$PWD/pi/marq" ~/.local/bin/marq     # symlink — updates with the repo

# Start ONE long-lived container, bound to a workspace dir (whatever you're
# working on — a research folder, a sample dir, a compliance project, a test):
marq up ~/marq/work               # docker run -d … sleep infinity

# Sanity checks
marq tools                        # list every tool
marq run server_info '{}'         # what marq covers + current engagement/purpose

marq down                         # tear down when finished
```

The model reaches marq through bash (`marq run <tool> '<json>'`), not MCP,
so Pi needs only the `marq` skill loaded and a model. Configure Pi once
(verified against Pi 0.80).

Step 1, point Pi at your local model. Pi configures providers via an extension.
Drop this at `~/.pi/agent/extensions/llama-cpp.ts` (auto-discovered):

```typescript
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
const BASE = "http://127.0.0.1:8080"; // llama-server default port
export default async function (pi: ExtensionAPI) {
  let ctx = 65536; // fallback; matches --ctx-size below
  try {
    const props = (await (await fetch(`${BASE}/props`)).json()) as any;
    ctx = props?.default_generation_settings?.n_ctx ?? ctx;
  } catch {}
  pi.registerProvider("llama-cpp", {
    name: "llama.cpp (local)",
    baseUrl: `${BASE}/v1`,
    apiKey: "llama",
    api: "openai-completions",
    models: [
      {
        id: "gemma-local", // just a label (see note)
        name: "gemma-local",
        reasoning: false,
        input: ["text"],
        cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
        contextWindow: ctx,
        maxTokens: 4096,
      },
    ],
  });
}
```

`llama-server` serves a single loaded model and ignores the request's `model`
field, so the `id` above is just a label. Name it what you like and match it in
`defaultModel`. Pi reads the real context length from llama.cpp's `/props`
(`n_ctx`) and sizes its window to match. Any OpenAI-compatible server works if you
point `BASE` at it instead.

Step 2, register the marq skill and model defaults in `~/.pi/agent/settings.json`:

```json
{
  "defaultProvider": "llama-cpp",
  "defaultModel": "llama-cpp/gemma-local",
  "skills": ["~/.pi/skills"]
}
```

Pi discovers any directory containing a `SKILL.md` (recursively), so give it one
from the image. No checkout is required, and it tracks the marq you actually run:

```bash
mkdir -p ~/.pi/skills/marq
marq prompt skill > ~/.pi/skills/marq/SKILL.md
```

From a checkout instead, skip the copy and point `skills` straight at the repo's
[`pi/`](../pi/) dir, which registers [`pi/SKILL.md`](../pi/SKILL.md) in place.

Recommended model: [`google/gemma-4-12B-it-qat-q4_0-gguf`](https://huggingface.co/google/gemma-4-12B-it-qat-q4_0-gguf),
Google's official quantization-aware-trained release. It is tool-capable, natively
q4_0, and fits a 12 to 16 GB GPU at a long context. Launch it with the flags below;
without them it leaks reasoning tokens into its answers and (without SYSTEM.md)
bypasses marq.

> Not yet on the leaderboard. This is the default on provenance grounds
> (official weights, no third-party fine-tune in the trust path), not because it
> measured better. Its one sweep to date was voided by a harness fault (see
> `scripts/eval/`), so treat it as unmeasured until a clean run publishes.
> A stock instruction-tuned model may also refuse some legitimate people OSINT.
> If that blocks you, evaluate alternatives with the harness rather than
> swapping blind.

Step 2a, start `llama-server` for Gemma. Install it once. On macOS, `brew install
llama.cpp` builds with Metal on Apple Silicon and runs on CPU on Intel Macs. On Linux, use
Linuxbrew `brew install llama.cpp`, a prebuilt CPU binary from the
[llama.cpp releases](https://github.com/ggml-org/llama.cpp/releases), or a
source/container build with CUDA/ROCm/Vulkan for GPU (the prebuilt Linux
binaries are CPU-only). Then run one command with the model's recommended
settings baked into flags (the same values marq's eval pins in
[`scripts/eval/profiles.json`](../scripts/eval/profiles.json), and the quickstarts
in [`scripts/eval/llama.cpp/`](../scripts/eval/llama.cpp/)):

```bash
# "Balanced" build (the default above). -hf pulls the GGUF from HF.
llama-server -hf google/gemma-4-12B-it-qat-q4_0-gguf \
  --host 127.0.0.1 --port 8080 -ngl 99 --ctx-size 65536 --jinja \
  --reasoning-format deepseek \
  -fa on -ctk q8_0 -ctv q8_0 \
  --temp 1.0 --top-p 0.95 --top-k 64
```

- `--host 127.0.0.1` binds loopback only. The Pi extension connects there,
  so that's all you need. Don't use `--host 0.0.0.0` unless you deliberately want
  to serve the model to other machines: it exposes the model on an
  open completions endpoint to your whole network. If you must, firewall the port.
- `--jinja` applies the model's chat template so tool calls parse. It is the
  most important flag for driving marq.
- `--reasoning-format deepseek` routes the model's chain-of-thought into a
  separate `reasoning_content` field instead of the reply. This model exposes
  CoT, so without the flag the reasoning leaks into answers and can exhaust the output
  budget mid-thought. Keep it on.
- `-fa on` + `-ctk q8_0 -ctv q8_0` is the memory win that makes 64K context
  fit on a 12 to 16 GB GPU. `-fa on` is flash attention (exact, not lossy; faster
  long-context prefill, smaller attention footprint) and is a prerequisite for
  the KV-cache flags. `-ctk/-ctv q8_0` quantize the K/V cache (default `f16`),
  roughly halving its VRAM at negligible quality cost. Both work on Metal
  (Apple Silicon) and CUDA/ROCm/Vulkan (Linux). Keep K and V the same
  type (mixed quant fails on Metal). Drop all three for CPU-only.
- Platform: works on Linux and macOS. Apple Silicon uses unified memory, so
  "12 to 16 GB GPU" means a 16 GB+ Mac (a 12B Q4 is ~7 to 8 GB). Intel Macs and
  CPU-only Linux run but are slow for a 12B, so drop `-ngl -fa -ctk -ctv` there.
- `--ctx-size` should be as high as VRAM allows (Gemma is 262 K native). Tool outputs
  (spiderfoot/gau dumps) are large, and a short window truncates them.
- Sampling: the flags above are Google's official set for the `-it` model
  (`--temp 1.0 --top-p 0.95 --top-k 64`). Community fine-tunes usually ship
  their own preset on the model card; use theirs, not these. Either way it's the
  smallest lever (see impact order). `-ngl 99` offloads all layers to the GPU.
- If a flaky small model emits malformed tool JSON, llama.cpp can constrain
  decoding to a grammar/JSON schema (`--grammar-file`, or `json_schema` in the
  request). GUI wrappers don't expose this reliability lever.

Impact order if it misbehaves: SYSTEM.md (2b) ≫ context length ≫ `--jinja`
≫ sampling. SYSTEM.md decides whether it uses marq at all; the rest is
output quality.

Step 2b, replace Pi's system prompt with marq's (important for smaller models):

```bash
mkdir -p ~/.pi/agent
marq prompt system > ~/.pi/agent/SYSTEM.md         # from the image, via the shim
# ln -sf "$PWD/pi/SYSTEM.md" ~/.pi/agent/SYSTEM.md # from a checkout; tracks the repo
```

Pi's default prompt frames the model as a general coding assistant, so a small
model reaches for raw `curl`/`grep` and bypasses marq entirely (no audit, no
skills). [`pi/SYSTEM.md`](../pi/SYSTEM.md) replaces that framing with
"drive everything through `marq run`, never raw bash". In testing this flipped
a 12B model from 300+ `curl` calls and zero marq use to clean, audited marq tool
calls. (`~/.pi/agent/SYSTEM.md` is global; use a per-project `.pi/SYSTEM.md` if
you also run Pi for non-marq work.) The `pi/SKILL.md` catalog is still appended.

Step 3, verify (no model call, so it won't disturb anything):

```bash
pi --list-models                  # → lists `llama-cpp  gemma-local  64.0K …`
```

Then just run `pi` in a working dir. The model loads the marq skill and
drives `marq run` from bash; the shim runs it inside the container.

> Gotchas learned the hard way: settings live in `~/.pi/agent/` (not
> `~/.pi/`); extensions auto-load only from `~/.pi/agent/extensions/*.ts` (the
> settings `extensions` array needs full paths, not bare names); Pi's bundled
> docs are at `…/pi-coding-agent/<ver>/libexec/.../docs/`.

A long-lived container is required. Background tools (`spiderfoot`, `gau_urls`, `dnstwist`)
detach inside it and are polled later, so a per-call `docker run`
would kill them.

Shim env vars: `MARQ_CONTAINER` (default `marq`), `MARQ_IMAGE` (default
`marq:latest`), `MARQ_ENGINE` (container CLI, default `docker`; set `podman` to
run daemonless without Docker Desktop), `MARQ_ENV_FILE` (default
`~/.config/marq/env`, loaded if present). Nothing is required: `MARQ_OPERATOR`
defaults to your `$USER`, and any API key exported in your shell is forwarded
(keys: `docs/USAGE.md`); the engagement + purpose is set at runtime via `set_engagement`.

---

## Debian Testing (rolling)

### 1. Prerequisites

On Linux the Docker Engine is free and open-source (only Docker _Desktop_ is
the commercial GUI you don't need). You can also run Podman, which is rootless,
daemonless, and a drop-in for `docker`:

```bash
# Option A — Docker Engine (open-source; docker.io from Testing ships BuildKit)
sudo apt update && sudo apt install -y docker.io docker-buildx git curl
sudo usermod -aG docker "$USER"        # log out/in (or: newgrp docker) so this takes effect
sudo systemctl enable --now docker

# Option B — Podman (rootless/daemonless). `podman-docker` adds a `docker` alias:
sudo apt install -y podman podman-docker git curl
#   then either use `podman` directly, or set MARQ_ENGINE=podman for the pi/marq shim.

# Optional, only for local Go dev
sudo apt install -y golang
```

Podman note: no extra capabilities are needed. Docker's official CE repo is also
fine instead of `docker.io`.

### 2. Build the image

```bash
git clone https://github.com/rhaist/marq.git && cd marq
docker build -t marq .          # native amd64 (or arm64 on ARM boards)
```

### 3. Run as an MCP server (external client brings the model)

Same as macOS step 3 above: the command is identical.

### 4. Run with a local model (Pi + the marq skill)

Same as macOS step 4: install the `pi/marq` shim, run `marq up <dir>`, and drive it
from Pi with [`pi/SKILL.md`](../pi/SKILL.md) loaded. Run `llama-server` on the box
(see the install options in macOS step 2a; on a headless GPU box, a source or
container build with CUDA/ROCm/Vulkan) and point Pi at it. The runtime lives in
Pi, not in marq.

> Bind-mount permissions (Linux): the container runs as the non-root `marq`
> user, so the working dir you pass to `marq up` must be writable by it.
> `chmod a+rwx <dir>` is the simplest way. macOS Docker Desktop handles this
> automatically.

---

## Local Go development (both OSes)

Iterate on the server/tool layer without a Docker build (tool _calls_ still need
the OSINT binaries, so most only fully work in-container):

```bash
go build ./... && go vet ./... && go test ./...   # build + sanity gate
go run ./cmd/marq serve                         # stdio MCP server locally
go run ./cmd/marq run subfinder '{"domain":"example.com"}'   # invoke one tool
```

## Verify

```bash
# Tool present in the built image:
docker run --rm marq subfinder -version

# MCP server lists its tools (expects ~34)
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
