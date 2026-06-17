# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Kali-based penetration-testing + OSINT toolkit. A single **Go** binary (`pentest`) exposes ~51 wrapped CLI tools two ways from one shared tool registry:

- **`pentest serve`** — an **MCP stdio server** (JSON-RPC over stdin/stdout) for an external MCP client (Claude Desktop, LM Studio, etc.) that brings its own model. No network port is opened.
- **`pentest tui`** — *(phases 3–4, in progress)* a TUI agent host with its own tool-calling loop driving a host-native local model runtime (Ollama/llama.cpp). The model runs on the **host** (Apple-Silicon Metal has no Docker GPU passthrough), reached over `host.docker.internal`.

> Active offensive + people/company-footprint tooling. Authorized testing only. See `docs/SECURITY.md`.

## Commands

```bash
# Build the (multi-GB) image — golang builder stage compiles `pentest`, then the
# Kali stage installs the full tool suite + warm-up
docker build -t pentest-mcp .

# Smoke-test a wrapped binary without starting the server
docker run --rm pentest-mcp nmap --version

# Run the MCP server (waits for JSON-RPC on stdin; banner -> stderr)
docker run --rm -i pentest-mcp

# Interactive shell inside the image
docker run --rm -it --entrypoint /bin/bash pentest-mcp

# Iterate on the Go server WITHOUT a Docker build (tools shell out to Kali
# binaries, so most tool *calls* only fully work in-container):
go build ./... && go vet ./... && go test ./...     # build + sanity gate
go run ./cmd/pentest serve                            # stdio MCP server locally
go run ./cmd/pentest run <tool> '<json-args>'        # invoke one tool directly
```

The sanity gate is **`go test ./...`** — `internal/registry/registry_test.go` asserts no duplicate tool names, that every tool has exactly one of Build/Handler, and that the generated input schemas are well-formed. `go vet ./...` + `go build ./...` are the quick syntax/type gate. There is no separate linter or CI. `scripts/test_tools.py` / `scripts/verify_tools.sh` drive the built **image** over MCP (`docker run -i`) for end-to-end coverage.

`entrypoint.sh`: bare `docker run` (or `... mcp` / `... serve`) starts `pentest serve`; `... tui` starts the agent host; any other args are exec'd directly (used for smoke tests).

## Architecture

**Tool call flow (the one invariant that matters):** every exec tool funnels through `internal/runner/runner.go::Run(tool, argv, Opts{...})`. That function is the single choke point for **audit logging, timeout, and output truncation** — tool code must never call `os/exec` directly, or those guarantees are bypassed. The shared **registry** (`internal/registry`) holds every tool as data; one `Tool.Call` dispatch routes exec tools through `Run`/`RunBackground` and in-process tools through their `Handler`. Both front-ends iterate the same `registry.All()`.

```
serve: external MCP client --stdio--> cmd/pentest (serve)
                                         internal/mcpserver (go-sdk) -- bridges each registry.Tool via Server.AddTool
tui  : internal/tui + internal/agent --HTTP--> host model runtime   (phases 3–4)
both : registry.Tool.Call -> runner.Run -> os/exec
                                  |-> internal/audit (JSON-lines, fsync'd, start+end per call)
```

- **`cmd/pentest/main.go`** — CLI entry: subcommands `serve` (default) / `tui` / `run <tool> [json]`.
- **`internal/mcpserver`** — adapts `registry.All()` onto an MCP stdio server using the official `github.com/modelcontextprotocol/go-sdk`. Each tool is added via the low-level `Server.AddTool(&mcp.Tool{InputSchema: t.InputSchema()}, handler)` with a raw-args handler that calls `t.Call`. Exposes the `pentest://authorization` + `pentest://methodology` resources and the `server_info` tool (the model should call `server_info` first to confirm scope).
- **`internal/config`** — all config is env-driven (`PENTEST_MCP_*`), resolved once into the package-level `config.C`. Adding a knob = add a field + env read in `Load()`.
- **`internal/audit`** — append-only JSON-lines log, `fsync` per write, one `invocation.start` + one `invocation.end` record per call. This is the project's core safety control ("logging-only" guardrails: nothing is blocked, everything is attributable). If hard enforcement (e.g. a scope allowlist) is ever wanted, the place to add it is `runner.Run` before `os/exec`, since everything funnels there.
- **`internal/registry`** — the tool suite as data. `registry.go` defines `Tool`/`Param`/`Invocation`/`Args`, `InputSchema()` (params -> JSON schema), and `Call` (the single execution entry). One file per category returns `[]Tool`: `recon`, `osint` (company/domain footprint), `people` (people footprint), `web`, `exploit`, `creds`, `files`, `extras` (findings + jobs), `shell` (added only when `config.C.AllowRawShell`). `Tool.Desc` is the model's only guidance for a tool — keep it informative (usage, defaults, API-key needs, caveats).
- **`internal/files`** — `ListDir`/`ReadFile`/`WriteFile` sandboxed via a `realpath` check to `/work` and `/tmp` only (`resolve()` enforces it, defeating symlink escapes). Makes file-driven tools (john, hashcat, gitleaks, exif_metadata) and read-back-from-disk tools (cmseek) usable, since the transport carries only strings. Audit-logged directly (not via `runner`).
- **`internal/findings`** (feature: engagement deliverable) — `report_finding` appends a structured finding to `/work/findings.jsonl`; `render_report` writes severity-sorted `findings.md` + `findings.csv`.
- **`internal/jobs`** (feature: background-job visibility) — `list_jobs` / `job_status` read the `status` file + tail `stdout.log` of `/work/jobs/*` dirs, so the model gets a clean running|done answer instead of polling files by hand.
- **`internal/shellword`** — `Split`/`Quote`/`Join`, the Go equivalents of Python `shlex.split`/`quote`/`join` used by the wrappers and the background-job redirection.

### Design constraints that shape the tool wrappers

The serve transport is **synchronous, one-shot, stateless, no interactivity** (`os/exec`, `Stdin` fixed or empty). When adding/editing tools, respect:
- **No interactive tools** — anything that prompts or needs a TTY will hang until timeout. Always pass non-interactive flags (`--batch`, `-x "...; exit"`, `--silence`, etc.).
- **15-min default timeout** (`PENTEST_MCP_TIMEOUT`), truncation at `PENTEST_MCP_MAX_OUTPUT` chars. An `Invocation.Timeout` override is clamped to `PENTEST_MCP_MAX_TIMEOUT` (default 3600) for moderately slow tools.
- **Tools that can't finish in an interactive window run in the background** by setting `Invocation.Background = true` (e.g. `spiderfoot`): `RunBackground` spawns the process detached (`Setsid`), streams `stdout.log`/`stderr.log` to a job dir under `/work/jobs/`, writes a `status` file when done, and returns the path immediately. The model polls with `list_jobs` / `job_status` (or the file tools). Reach for this instead of a long synchronous `Run` — the MCP client has its own call timeout that a 15-min scan will blow regardless of the server-side limit.
- **Watch for Kali `sudo`-wrapper binaries under `no-new-privileges`.** Some packages ship `/usr/bin/<tool>` as a shell wrapper that calls `sudo` (e.g. amass → libpostal); the container's `no-new-privileges` makes `sudo` fail instantly (exit 1). Call the real binary directly or pick a tool that doesn't wrap sudo.
- **No file upload** — the model passes strings only. File inputs/outputs go through the `/work` mount + the `files` tools.
- Tools needing **API keys** (shodan, censys, h8mail breaches, trufflehog GitHub, phoneinfoga enrichment) degrade quietly without them; the key env vars are passed through in `mcp.json.example` / `docker-compose.yml` and documented in `docs/USAGE.md`.

### Dockerfile structure (order is load-bearing)

1. **Stage 1 (`golang` builder)** compiles the static `pentest` binary (`CGO_ENABLED=0`).
2. apt installs the bulk of the suite (explicit package list, not `kali-linux-everything`, for auditability).
3. Go-built tools not in Kali apt (`katana`, `gau`, `waybackurls`, `dalfox`, `phoneinfoga`) — installed system-wide to `/usr/local/bin` (these still use the apt `golang-go`).
4. `setcap` grants `nmap`/`masscan`/`naabu` SYN-scan caps so they run non-root.
5. venv at `/opt/venv` (on PATH) holds the pip-only OSINT tools (`maigret`, `holehe`) — **no MCP SDK** (the server is the Go binary). The `pentest` binary is copied from stage 1.
6. Drops to the unprivileged `pentester` user, **then** runs the build-time warm-up.

**Build-time warm-up runs AS `pentester`** (after the `USER` switch) on purpose: tools that cache data in `$HOME` (nuclei templates, wpscan DB, metasploit module cache) must populate `/home/pentester`, not `/root`. Each warm-up step is `|| true` so a build-host network hiccup falls back to the tool's first-run download instead of failing the image. rockyou is gunzipped to the canonical `/usr/share/wordlists/rockyou.txt`.

When adding a tool: prefer a Kali apt package (verify on pkg.kali.org); if it caches/downloads on first use, add a warm-up line in the post-`USER` block; add the `Tool` to the relevant `internal/registry/*.go`; document it in `docs/USAGE.md`.
