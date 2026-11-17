# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Kali-based **passive-OSINT and cyber-knowledge assistant**. Two layers: **execution** — 24 wrapped OSINT tools (34 tools in all) that query public third-party sources only (DNS via resolver, WHOIS, certificate logs, Shodan/Censys, web archives, breach DBs, social sites, public code); and **knowledge** — a skills library of domain playbooks (threat intel, malware, sec-ops, architecture, GRC, standards, CISO; `skills/<domain>/<name>/SKILL.md`, a standard Agent Skills pack also installable natively as a Claude Code plugin or via `npx skills`) the model loads on demand. **marq never scans, crawls or exploits a target's hosts** (only ordinary DNS resolution reaches its nameservers; username/email tools query third-party sites): no scanners, exploitation, cracking or malware-analysis tools, and so no scope concept. A single **Go** binary (`marq`) exposes the tools two ways from one shared registry:

- **`marq serve`** — an **MCP stdio server** (JSON-RPC over stdin/stdout) for an external MCP client (Claude Code, Codex, Claude Desktop, etc.) that brings its own model. No network port is opened.
- **`marq run <tool> '<json>'`** — invoke one tool directly. This is the interface a terminal agent uses: a host shim (`pi/marq`) `docker exec`s into a long-lived container, and a markdown skill (`pi/SKILL.md`) teaches a local model to drive `marq run` from bash. marq owns the tools; the agent loop + model runtime are borrowed (e.g. [Pi](https://pi.dev/) on the host). `marq tools` lists the catalog.

> The model-runtime UX is delegated to whatever client drives marq (Pi via the skill, or any MCP client via `serve`). The passive-only promise is enforced in each wrapper's argv (fixed flags, allowlisted record types/subcommands, `Invocation.Refuse`), pinned by `TestPassiveOnly`; every OSINT-tool, file and findings call is audit-logged (OSINT tools at `runner.Run`, fail-closed; in-process lookups are not logged). People OSINT is personal-data processing — `set_engagement` records a label + purpose for the audit trail. See `docs/SECURITY.md`. The scope-gated active-testing design is in git history (commit 4114959) if it's ever needed again.

## Commands

```bash
# Build the image — golang builder stage compiles `marq`, then the Kali stage
# installs the OSINT tool suite
docker build -t marq .

# Smoke-test a wrapped binary without starting the server
docker run --rm marq subfinder -version

# Run the MCP server (waits for JSON-RPC on stdin; banner -> stderr)
docker run --rm -i marq

# Interactive shell inside the image
docker run --rm -it --entrypoint /bin/bash marq

# Iterate on the Go server WITHOUT a Docker build (tools shell out to Kali
# binaries, so most tool *calls* only fully work in-container):
go build ./... && go vet ./... && go test ./...     # build + sanity gate
go run ./cmd/marq serve                            # stdio MCP server locally
go run ./cmd/marq run <tool> '<json-args>'        # invoke one tool directly
go run ./cmd/marq skills [name]                   # browse the playbook library / print one
go run ./cmd/marq version                         # build rev + tool/skill counts
```

The sanity gate is **`go test ./...`** — `internal/registry/registry_test.go` asserts no duplicate tool names, that every tool has exactly one of Build/Handler, that the generated input schemas are well-formed, and (`TestPassiveOnly`) that no argument can turn a lookup into a probe of the target. `go vet ./...` + `go build ./...` are the quick syntax/type gate. Test surface beyond the unit gate:

- **`scripts/e2e.sh`** — end-to-end of the user-facing promises through the real `marq run` CLI (`set_engagement` persistence, passive-only refusals, all skills load, findings + CVSS, audit, file sandbox, catalog). No image, runs in seconds.
- **`scripts/e2e-image.sh`** — the promises that only hold for the running image (`/work` bind-mount persistence across containers). Needs a built image.
- **`scripts/test_tools.py`** / **`scripts/verify_tools.sh`** — drive the built **image** over MCP (`docker run -i`) for wrapped-binary plumbing coverage.
- **`scripts/eval/`** — measures how well a model drives marq's skills+tools (and the skills-on/off lift), producing a committed, comparable-over-time leaderboard. Its README captures the local-model gotchas (text-form tool calls, thinking models, tool-exec capping).
- **CI:** `.github/workflows/ci.yml` (fast gate + `e2e.sh` on every push/PR) and `image.yml` (path-gated image build + `e2e-image.sh`, publishes to GHCR on main).

`entrypoint.sh`: bare `docker run` (or `... mcp` / `... serve`) starts `marq serve`; a marq subcommand (`run`/`tools`/`skills`/`shim`/`prompt`/`version`) is passed to the binary, so `docker run --rm marq shim` works; any other args are exec'd directly (the `pi/marq` shim runs the container as `... sleep infinity` and `docker exec`s `marq run`/`marq tools` into it).

## Architecture

**Tool call flow (the one invariant that matters):** every exec tool funnels through `internal/runner/runner.go::Run(tool, argv, Opts{...})`. That function is the single choke point for **audit logging, timeout, and output truncation** — tool code must never call `os/exec` directly, or those guarantees are bypassed. The shared **registry** (`internal/registry`) holds every tool as data; one `Tool.Call` dispatch routes exec tools through `Run`/`RunBackground` (after any `Invocation.Refuse`, which is audit-logged) and in-process tools through their `Handler`. Both front-ends iterate the same `registry.All()`.

```
serve: external MCP client --stdio--> cmd/marq (serve)
                                         internal/mcpserver (go-sdk) -- bridges each registry.Tool via Server.AddTool
run  : terminal agent (Pi) --bash--> pi/marq shim --docker exec--> cmd/marq (run <tool> '<json>')
both : registry.Tool.Call -> runner.Run -> os/exec
                                  |-> internal/audit (JSON-lines, fsync'd, start+end per call)
```

- **`cmd/marq/main.go`** — CLI entry: subcommands `serve` (default) / `run <tool> [json]` / `tools` (catalog) / `skills [name]` (the knowledge-side twin of `tools`) / `shim` (prints the embedded `pi/marq` host shim) / `prompt <system|skill>` (prints the embedded `pi/SYSTEM.md` / `pi/SKILL.md`) / `version` (build rev + counts, which also surfaces an unintended `MARQ_SKILLS_ONLY`).
- **`internal/mcpserver`** — adapts `registry.All()` onto an MCP stdio server using the official `github.com/modelcontextprotocol/go-sdk`. Each tool is added via the low-level `Server.AddTool(&mcp.Tool{InputSchema: t.InputSchema()}, handler)` with a raw-args handler that calls `t.Call`. Exposes the `marq://notice` (usage notice), `marq://methodology` and `marq://skills[/<name>]` resources; the model is told to call `server_info` first.
- **`pi/`** — the terminal-agent integration (not Go). `pi/SKILL.md` is a portable markdown skill: purpose-first instructions, the `marq run <tool> '<json>'` calling convention, tool discovery via `marq tools`, and a small-model working style + few-shot. `pi/marq` is the host shim that `docker exec`s into a long-lived container so the model can call `marq` from bash.
- **`internal/config`** — all config is env-driven (`MARQ_*`), resolved once into the package-level `config.C`. Adding a knob = add a field + env read in `Load()`. `MARQ_OPERATOR` (the audit attribution anchor) is env-set only. The engagement label/purpose defaults from `MARQ_ENGAGEMENT` but is meant to be set at runtime by the model via the `set_engagement` tool → `config.SetEngagement`, which persists `{engagement}` to `$MARQ_WORK_DIR/.marq-context`; `Load()` overlays that file so the value survives across the process-per-call `marq run` path (each call re-`Load()`s). The `pi/marq` shim needs no config: it forwards known API keys exported in the host shell, loads `~/.config/marq/env` if present, and defaults `MARQ_OPERATOR` to the host `$USER`.
- **`internal/audit`** — append-only JSON-lines log, `fsync` per write, one `invocation.start` + one `invocation.end` record per call, and each record **mirrored to stderr** (a tamper-evident copy outside the `marq` user's reach). This is the project's core safety control: a call whose start record can't be persisted is **refused at `runner.Run`** (fail-closed, so the log can't be silently skipped; everything that runs is attributable).
- **`internal/registry`** — the tool suite as data. `registry.go` defines `Tool`/`Param`/`Invocation`/`Args`, `InputSchema()` (params -> JSON schema), and `Call` (the single execution entry). One file per category returns `[]Tool`: `recon` (DNS/registry lookups), `intel` (certificates, RDAP, lookalike domains, threat-intel lookups), `osint` (company/domain footprint), `people` (people footprint), `files`, `extras` (findings + jobs + skills). A wrapper that takes free-form input must not let it reach the target: fix the flags, allowlist values, and return `Invocation{Refuse: ...}` otherwise — and extend `TestPassiveOnly`. `Tool.Desc` is the model's only guidance for a tool — keep it informative (usage, defaults, API-key needs, caveats).
- **`internal/files`** — `ListDir`/`ReadFile`/`WriteFile` sandboxed via a `realpath` check to `/work`, `/tmp` and `MARQ_WORK_DIR` only (`Resolve()` enforces it, defeating symlink escapes). `exif_metadata` also uses `files.Resolve`. Makes file-driven tools (exif_metadata) and read-back-from-disk results (background job logs) usable, since the transport carries only strings. Audit-logged directly (not via `runner`).
- **`internal/findings`** (feature: engagement deliverable) — `report_finding` appends a structured finding to `/work/findings.jsonl`; `render_report` writes severity-sorted `findings.md` + `findings.csv`. Accepts an optional CVSS 3.1 vector — `cvss.go` computes the base score (pure arithmetic, no LLM). Severity precedence: the caller's explicit severity is authoritative; a CVSS vector only _sets_ severity when none was given, and a divergent vector is flagged (not silently applied) so a model-guessed vector can't mask the assigned risk. Also CWE + references fields.
- **`internal/jobs`** (feature: background-job visibility) — `list_jobs` / `job_status` read the `status` file + tail `stdout.log` of `/work/jobs/*` dirs, so the model gets a clean running|done answer instead of polling files by hand.
- **`internal/skills`** (feature: knowledge library) — loads the repo-root `skills/` pack (embedded via the root package `skills.go`, since `go:embed` can't reach parent dirs) and exposes it via the `load_skill` tool (registry), `marq://skills` + `marq://skills/<name>` MCP resources and `marq skills [--stale]`. The same tree is the distributable Agent Skills pack: `.claude-plugin/marketplace.json` + `skills/.claude-plugin/plugin.json` (Claude Code; lists each category dir, since plugins scan one level) and `npx skills` (discovers `skills/<category>/<name>/SKILL.md`). Add a skill = `skills/<category>/<name>/SKILL.md` with `name` (== dir), `description` (double-quote it if it contains `: `) and `metadata.verified`; `go test ./internal/skills/` enforces the spec. The 6-8 week review sweep is the `/refresh-skills` project skill (`.claude/skills/refresh-skills/SKILL.md`). (Distinct from `pi/SKILL.md`, which is the host-side Pi skill that teaches a model to drive `marq run`.)
- **`internal/untrusted`** — prompt-injection hardening for third-party text. `Wrap` strips hidden-text characters (Unicode tags, variation selectors, bidi, zero-width, ANSI/OSC, controls), prepends a `[marq removed …]` note when it did, and fences the text in `[untrusted-data <nonce> begin/end]` markers. `runner.Result.Render`, `files.ReadFile`, `files.ListDir` and `jobs.Status` apply it; `mcpserver.untrustedRule` (mirrored in pi/SKILL.md, pi/SYSTEM.md, methodology.md) tells the model what the fence means. Exec tools also carry MCP `readOnlyHint`/`openWorldHint` annotations. Any new path that returns third-party text to the model must go through `Wrap`.
- **`internal/shellword`** — `Split`/`Quote`/`Join`, the Go equivalents of Python `shlex.split`/`quote`/`join` used by the wrappers and the background-job redirection.

### Design constraints that shape the tool wrappers

The serve transport is **synchronous, one-shot, stateless, no interactivity** (`os/exec`, `Stdin` fixed or empty). When adding/editing tools, respect:

- **No interactive tools** — anything that prompts or needs a TTY will hang until timeout. Always pass non-interactive flags (`--batch`, `-x "...; exit"`, `--silence`, etc.).
- **15-min default timeout** (`MARQ_TIMEOUT`), truncation at `MARQ_MAX_OUTPUT` chars. `MARQ_MAX_TIMEOUT` (default 3600) caps how long a background job may run.
- **Tools that can't finish in an interactive window run in the background** by setting `Invocation.Background = true` (e.g. `spiderfoot`, `gau_urls`): `RunBackground` spawns the process detached (`Setsid`), streams `stdout.log`/`stderr.log` to a job dir under `/work/jobs/`, writes a `status` file when done, and returns the path immediately. The model polls with `list_jobs` / `job_status` (or the file tools). Reach for this instead of a long synchronous `Run` — the MCP client has its own call timeout that a 15-min scan will blow regardless of the server-side limit.
- **Watch for Kali `sudo`-wrapper binaries under `no-new-privileges`.** Some packages ship `/usr/bin/<tool>` as a shell wrapper that calls `sudo` (e.g. amass → libpostal, which is why amass isn't wrapped); the container's `no-new-privileges` makes `sudo` fail instantly (exit 1). Call the real binary directly or pick a tool that doesn't wrap sudo.
- **No file upload** — the model passes strings only. File inputs/outputs go through the `/work` mount + the `files` tools.
- Tools needing **API keys** (shodan, censys, h8mail breaches, trufflehog GitHub, phoneinfoga enrichment) degrade quietly without them; the key env vars are passed through in `mcp.json.example` and documented in `docs/USAGE.md`.

### Dockerfile structure (order is load-bearing)

1. **Stage 1 (`golang` builder)** compiles the static `marq` binary (`CGO_ENABLED=0`).
2. apt installs the OSINT suite (explicit package list, not a metapackage, for auditability).
3. Go-built tools not in Kali apt (`gau`, `cdncheck`) — `go install`ed system-wide to `/usr/local/bin` with a throwaway apt `golang-go` purged in the same layer; `phoneinfoga` and `censys` (cencli) are pinned prebuilt release binaries.
4. venv at `/opt/venv` (on PATH) holds the pip-only tools (`maigret`, `user-scanner` (pinned), `shodan` + `setuptools<82` for its `pkg_resources` import) — **no MCP SDK** (the server is the Go binary); the `marq` binary is copied from stage 1.
5. Drops to the unprivileged `marq` user. Nothing in the image scans, so the container runs fine with `--cap-drop ALL`.

When adding a tool: it must query public third-party sources only, never the target's own hosts. Prefer a Kali apt package (verify on pkg.kali.org); add the `Tool` to the relevant `internal/registry/*.go`; document it in `docs/USAGE.md`.
