# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Kali-based **cyber assistant** spanning offensive (pentest/OSINT), malware research, threat intel, and governance (GRC, standards, CISO advisory). Two layers: **execution** — ~80 wrapped CLI tools (the offensive + malware-static suite); and **knowledge** — an embedded skills library of domain playbooks (`internal/skills/library/<domain>/`) the model loads on demand. Tools exist where there's something to run; the knowledge domains are skills + the model + web-fetch (see the `security-apis` skill). A single **Go** binary (`marq`) exposes the tools two ways from one shared registry:

- **`marq serve`** — an **MCP stdio server** (JSON-RPC over stdin/stdout) for an external MCP client (Claude Desktop, LM Studio, etc.) that brings its own model. No network port is opened.
- **`marq run <tool> '<json>'`** — invoke one tool directly. This is the interface a terminal agent uses: a host shim (`pi/marq`) `docker exec`s into a long-lived container, and a markdown skill (`pi/SKILL.md`) teaches a local/abliterated model to drive `marq run` from bash. marq owns the tools; the agent loop + model runtime are borrowed (e.g. [Pi](https://pi.dev/) on the host). `marq tools` lists the catalog.

> marq no longer ships its own TUI/agent loop — the model-runtime UX is delegated to whatever client drives it (Pi via the skill, or any MCP client via `serve`). Advisory/knowledge work is unrestricted; **active testing is authorized-only** and scope-gated at `server_info` + audit-logged at `runner.Run`. See `docs/SECURITY.md`.

## Commands

```bash
# Build the (multi-GB) image — golang builder stage compiles `marq`, then the
# Kali stage installs the full tool suite + warm-up
docker build -t marq .

# Smoke-test a wrapped binary without starting the server.
# nmap/masscan/naabu carry file capabilities (cap_net_admin), which a bare
# container won't exec — add the caps, or smoke a non-capped tool (e.g. nuclei).
docker run --rm --cap-add NET_RAW --cap-add NET_ADMIN marq nmap --version
docker run --rm marq nuclei -version

# Run the MCP server (waits for JSON-RPC on stdin; banner -> stderr)
docker run --rm -i marq

# Interactive shell inside the image
docker run --rm -it --entrypoint /bin/bash marq

# Iterate on the Go server WITHOUT a Docker build (tools shell out to Kali
# binaries, so most tool *calls* only fully work in-container):
go build ./... && go vet ./... && go test ./...     # build + sanity gate
go run ./cmd/marq serve                            # stdio MCP server locally
go run ./cmd/marq run <tool> '<json-args>'        # invoke one tool directly
```

The sanity gate is **`go test ./...`** — `internal/registry/registry_test.go` asserts no duplicate tool names, that every tool has exactly one of Build/Handler, and that the generated input schemas are well-formed. `go vet ./...` + `go build ./...` are the quick syntax/type gate. There is no separate linter or CI. `scripts/test_tools.py` / `scripts/verify_tools.sh` drive the built **image** over MCP (`docker run -i`) for end-to-end coverage.

`entrypoint.sh`: bare `docker run` (or `... mcp` / `... serve`) starts `marq serve` (after regenerating jwt_tool's per-container keypair); any other args are exec'd directly (the `pi/marq` shim runs the container as `... sleep infinity` and `docker exec`s `marq run`/`marq tools` into it).

## Architecture

**Tool call flow (the one invariant that matters):** every exec tool funnels through `internal/runner/runner.go::Run(tool, argv, Opts{...})`. That function is the single choke point for **audit logging, timeout, and output truncation** — tool code must never call `os/exec` directly, or those guarantees are bypassed. The shared **registry** (`internal/registry`) holds every tool as data; one `Tool.Call` dispatch routes exec tools through `Run`/`RunBackground` and in-process tools through their `Handler`. Both front-ends iterate the same `registry.All()`.

```
serve: external MCP client --stdio--> cmd/marq (serve)
                                         internal/mcpserver (go-sdk) -- bridges each registry.Tool via Server.AddTool
run  : terminal agent (Pi) --bash--> pi/marq shim --docker exec--> cmd/marq (run <tool> '<json>')
both : registry.Tool.Call -> runner.Run -> os/exec
                                  |-> internal/audit (JSON-lines, fsync'd, start+end per call)
```

- **`cmd/marq/main.go`** — CLI entry: subcommands `serve` (default) / `run <tool> [json]` / `tools` (catalog).
- **`internal/mcpserver`** — adapts `registry.All()` onto an MCP stdio server using the official `github.com/modelcontextprotocol/go-sdk`. Each tool is added via the low-level `Server.AddTool(&mcp.Tool{InputSchema: t.InputSchema()}, handler)` with a raw-args handler that calls `t.Call`. Exposes the `marq://authorization`, `marq://methodology` and `marq://skills[/<name>]` resources and the `server_info` tool (the model should call `server_info` first to confirm scope).
- **`pi/`** — the terminal-agent integration (not Go). `pi/SKILL.md` is a portable markdown skill: scope-first instructions, the `marq run <tool> '<json>'` calling convention, tool discovery via `marq tools`, and a small-model working style + few-shot. `pi/marq` is the host shim that `docker exec`s into a long-lived container so the model can call `marq` from bash. This replaces the old in-binary TUI/agent loop — Pi (or any client) owns the loop + model runtime; marq owns the audited tools.
- **`internal/config`** — all config is env-driven (`MARQ_*`), resolved once into the package-level `config.C`. Adding a knob = add a field + env read in `Load()`. `MARQ_OPERATOR` (the audit attribution anchor) is env-set only. Engagement/scope default from `MARQ_ENGAGEMENT`/`MARQ_SCOPE` but are meant to be set at runtime by the model via the `set_engagement` tool → `config.SetEngagement`, which persists `{engagement,scope}` to `$MARQ_WORK_DIR/.marq-context`; `Load()` overlays that file so the values survive across the process-per-call `marq run` path (each call re-`Load()`s). The `pi/marq` shim passes operator + keys through with `--env-file`.
- **`internal/audit`** — append-only JSON-lines log, `fsync` per write, one `invocation.start` + one `invocation.end` record per call. This is the project's core safety control ("logging-only" guardrails: nothing is blocked, everything is attributable). If hard enforcement (e.g. a scope allowlist) is ever wanted, the place to add it is `runner.Run` before `os/exec`, since everything funnels there.
- **`internal/registry`** — the tool suite as data. `registry.go` defines `Tool`/`Param`/`Invocation`/`Args`, `InputSchema()` (params -> JSON schema), and `Call` (the single execution entry). One file per category returns `[]Tool`: `recon`, `osint` (company/domain footprint), `people` (people footprint), `web`, `exploit`, `creds`, `internal` (AD/internal network), `files`, `extras` (findings + jobs), `shell` (added only when `config.C.AllowRawShell`). `Tool.Desc` is the model's only guidance for a tool — keep it informative (usage, defaults, API-key needs, caveats).
- **`internal/files`** — `ListDir`/`ReadFile`/`WriteFile` sandboxed via a `realpath` check to `/work` and `/tmp` only (`resolve()` enforces it, defeating symlink escapes). Makes file-driven tools (john, hashcat, gitleaks, exif_metadata) and read-back-from-disk tools (cmseek) usable, since the transport carries only strings. Audit-logged directly (not via `runner`).
- **`internal/findings`** (feature: engagement deliverable) — `report_finding` appends a structured finding to `/work/findings.jsonl`; `render_report` writes severity-sorted `findings.md` + `findings.csv`. Accepts an optional CVSS 3.1 vector — `cvss.go` computes the base score (pure arithmetic, no LLM). Severity precedence: the caller's explicit severity is authoritative; a CVSS vector only _sets_ severity when none was given, and a divergent vector is flagged (not silently applied) so a model-guessed vector can't mask the assigned risk. Also CWE + references fields.
- **`internal/jobs`** (feature: background-job visibility) — `list_jobs` / `job_status` read the `status` file + tail `stdout.log` of `/work/jobs/*` dirs, so the model gets a clean running|done answer instead of polling files by hand.
- **`internal/skills`** (feature: knowledge library) — embedded markdown playbooks (per-vuln-class + per-technique, under `library/`) tied to this server's tool names. Exposed via the `load_skill` tool (registry) and `marq://skills` + `marq://skills/<name>` MCP resources. Add a skill = drop a `.md` with `name`/`description` frontmatter in `library/<category>/`. (Distinct from `pi/SKILL.md`, which is the host-side Pi skill that teaches a model to drive `marq run`.)
- **`internal/shellword`** — `Split`/`Quote`/`Join`, the Go equivalents of Python `shlex.split`/`quote`/`join` used by the wrappers and the background-job redirection.

### Design constraints that shape the tool wrappers

The serve transport is **synchronous, one-shot, stateless, no interactivity** (`os/exec`, `Stdin` fixed or empty). When adding/editing tools, respect:

- **No interactive tools** — anything that prompts or needs a TTY will hang until timeout. Always pass non-interactive flags (`--batch`, `-x "...; exit"`, `--silence`, etc.).
- **15-min default timeout** (`MARQ_TIMEOUT`), truncation at `MARQ_MAX_OUTPUT` chars. An `Invocation.Timeout` override is clamped to `MARQ_MAX_TIMEOUT` (default 3600) for moderately slow tools.
- **Tools that can't finish in an interactive window run in the background** by setting `Invocation.Background = true` (e.g. `spiderfoot`): `RunBackground` spawns the process detached (`Setsid`), streams `stdout.log`/`stderr.log` to a job dir under `/work/jobs/`, writes a `status` file when done, and returns the path immediately. The model polls with `list_jobs` / `job_status` (or the file tools). Reach for this instead of a long synchronous `Run` — the MCP client has its own call timeout that a 15-min scan will blow regardless of the server-side limit.
- **Watch for Kali `sudo`-wrapper binaries under `no-new-privileges`.** Some packages ship `/usr/bin/<tool>` as a shell wrapper that calls `sudo` (e.g. amass → libpostal); the container's `no-new-privileges` makes `sudo` fail instantly (exit 1). Call the real binary directly or pick a tool that doesn't wrap sudo.
- **No file upload** — the model passes strings only. File inputs/outputs go through the `/work` mount + the `files` tools.
- Tools needing **API keys** (shodan, censys, h8mail breaches, trufflehog GitHub, phoneinfoga enrichment) degrade quietly without them; the key env vars are passed through in `mcp.json.example` and documented in `docs/USAGE.md`.

### Dockerfile structure (order is load-bearing)

1. **Stage 1 (`golang` builder)** compiles the static `marq` binary (`CGO_ENABLED=0`).
2. apt installs the bulk of the suite (explicit package list, not `kali-linux-everything`, for auditability).
3. Go-built tools not in Kali apt (`katana`, `gau`, `waybackurls`, `dalfox`, `interactsh-client`, `asnmap`, `cdncheck`, `phoneinfoga`) — installed system-wide to `/usr/local/bin` (these still use the apt `golang-go`).
4. `setcap` grants `nmap`/`masscan`/`naabu` SYN-scan caps so they run non-root.
5. venv at `/opt/venv` (on PATH) holds the pip-only OSINT tools (`maigret`, `holehe`) — **no MCP SDK** (the server is the Go binary). The `marq` binary is copied from stage 1.
6. Drops to the unprivileged `marq` user, **then** runs the build-time warm-up.

**Build-time warm-up runs AS `marq`** (after the `USER` switch) on purpose: tools that cache data in `$HOME` (nuclei templates, wpscan DB) must populate `/home/marq`, not `/root`. Each warm-up step is `|| true` so a build-host network hiccup falls back to the tool's first-run download instead of failing the image. rockyou is gunzipped to the canonical `/usr/share/wordlists/rockyou.txt`.

When adding a tool: prefer a Kali apt package (verify on pkg.kali.org); if it caches/downloads on first use, add a warm-up line in the post-`USER` block; add the `Tool` to the relevant `internal/registry/*.go`; document it in `docs/USAGE.md`.
