# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Kali-based penetration-testing + OSINT toolkit exposed to an MCP client (LM Studio) over **stdio JSON-RPC**. A tool-use-capable model drives ~49 wrapped CLI tools through a small Python MCP layer. No network port is opened — the client launches the container and talks over stdin/stdout.

> Active offensive + people/company-footprint tooling. Authorized testing only. See `docs/SECURITY.md`.

## Commands

```bash
# Build the (multi-GB) image — pulls Kali + the full tool suite + warm-up
docker build -t pentest-mcp .

# Smoke-test a wrapped binary without starting the server
docker run --rm pentest-mcp nmap --version

# Run the MCP server (waits for JSON-RPC on stdin; banner -> stderr)
docker run --rm -i pentest-mcp

# Interactive shell inside the image
docker run --rm -it --entrypoint /bin/bash pentest-mcp

# Iterate on the Python server WITHOUT rebuilding the image:
pip install -e .              # needs the `mcp` package; tools shell out to Kali
                              # binaries, so most tool calls only work in-container
python -m server.main         # starts the stdio server locally
```

There is **no test suite, linter, or CI**. The fast way to sanity-check changes to the server/tool layer without a Docker build is to register the tool modules against a stub MCP object (each `tools/*.py` exposes `register(mcp)` where `mcp.tool()` is a decorator factory) and assert there are no duplicate tool names — this catches signature/decorator errors and is what was used during development. `python3 -m py_compile server/*.py server/tools/*.py` is a quick syntax gate.

`entrypoint.sh`: bare `docker run` (or `... mcp`) starts the server; any other args are exec'd directly (used for smoke tests).

## Architecture

**Tool call flow (the one invariant that matters):** every tool wrapper MUST funnel through `server/runner.py::run(tool, argv, target=..., stdin=..., timeout=...)`. That function is the single choke point for **audit logging, timeout, and output truncation** — tool modules must never call `subprocess` directly, or those guarantees are bypassed. `run()` returns a `Result`; wrappers return `result.render()` (a text envelope the model reads).

```
client --stdio--> server/main.py (FastMCP)
                    _register_all() -> tools/*.py register(mcp)
                       each @mcp.tool() wrapper -> runner.run() -> subprocess
                                                      |-> audit.py (JSON-lines, fsync'd, start+end per call)
```

- **`server/main.py`** — FastMCP entrypoint; `_register_all()` registers each tool group. `shell.py` (raw command escape hatch) is registered **only** when `CONFIG.allow_raw_shell`. Also exposes the `pentest://authorization` resource + `server_info` tool (the model should call `server_info` first to confirm scope).
- **`server/config.py`** — all config is env-driven (`PENTEST_MCP_*`), read once into a module-level `CONFIG` dataclass. Adding a knob = add a field + env read here.
- **`server/audit.py`** — append-only JSON-lines log, `fsync` per write, one `invocation.start` + one `invocation.end` record per call. This is the project's core safety control ("logging-only" guardrails: nothing is blocked, everything is attributable). If hard enforcement (e.g. a scope allowlist) is ever wanted, the place to add it is `runner.run` before `subprocess.run`, since everything already funnels there.
- **`server/tools/`** — one module per category, each a `register(mcp)` containing thin `@mcp.tool()` wrappers: `recon`, `osint` (company/domain footprint), `people` (people footprint), `web`, `exploit`, `creds`, `files`, `shell`. Tool docstrings are the model's only guidance for a tool, so they encode usage, defaults, API-key needs, and caveats — keep them informative.
- **`server/tools/files.py`** — `read_file`/`write_file`/`list_dir` sandboxed via `os.path.realpath` to `/work` and `/tmp` only (`_resolve()` enforces it). This is what makes file-driven tools (john, hashcat, gitleaks, exif_metadata) and read-back-from-disk tools (cmseek) usable, since the MCP transport carries only strings. These ops are audit-logged through `audit` directly (not `runner`).

### Design constraints that shape the tool wrappers

The transport is **synchronous, one-shot, stateless, no interactivity** (`subprocess.run`, `stdin` fixed or `None`). When adding/editing tools, respect:
- **No interactive tools** — anything that prompts or needs a TTY will hang until timeout. Always pass non-interactive flags (`--batch`, `-x "...; exit"`, `--silence`, etc.).
- **15-min default timeout** (`PENTEST_MCP_TIMEOUT`), truncation at `PENTEST_MCP_MAX_OUTPUT` chars. `run()` takes an optional `timeout` override clamped to `PENTEST_MCP_MAX_TIMEOUT` (default 3600) for moderately slow tools.
- **Tools that can't finish in an interactive window run in the background** via `runner.run_background()` (e.g. `spiderfoot`): it spawns the process detached, streams `stdout.log`/`stderr.log` to a job dir under `/work/jobs/`, writes a `status` file when done, and returns the path immediately. The model polls results with the `read_file`/`list_dir` file tools. Reach for this instead of a long synchronous `run()` — the MCP client has its own call timeout that a 15-min scan will blow regardless of the server-side limit.
- **Watch for Kali `sudo`-wrapper binaries under `no-new-privileges`.** Some packages ship `/usr/bin/<tool>` as a shell wrapper that calls `sudo` (e.g. amass → libpostal); the container's `no-new-privileges` makes `sudo` fail instantly (exit 1). Call the real binary directly or pick a tool that doesn't wrap sudo.
- **No file upload** — the model passes strings only. File inputs/outputs go through the `/work` mount + the `files.py` tools.
- Tools needing **API keys** (shodan, censys, h8mail breaches, trufflehog GitHub, phoneinfoga enrichment) degrade quietly without them; the key env vars are passed through in `mcp.json.example` / `docker-compose.yml` and documented in `docs/USAGE.md`.

### Dockerfile structure (order is load-bearing)

1. apt installs the bulk of the suite (explicit package list, not `kali-linux-everything`, for auditability).
2. Go-built tools not in Kali apt (`katana`, `gau`, `waybackurls`, `phoneinfoga`) — installed system-wide to `/usr/local/bin`.
3. `setcap` grants `nmap`/`masscan`/`naabu` SYN-scan caps so they run non-root.
4. venv at `/opt/venv` (on PATH) holds the `mcp` SDK + pip-only OSINT tools (`maigret`, `holehe`).
5. Drops to the unprivileged `pentester` user, **then** runs the build-time warm-up.

**Build-time warm-up runs AS `pentester`** (after the `USER` switch) on purpose: tools that cache data in `$HOME` (nuclei templates, wpscan DB, metasploit module cache) must populate `/home/pentester`, not `/root`. Each warm-up step is `|| true` so a build-host network hiccup falls back to the tool's first-run download instead of failing the image. rockyou is gunzipped to the canonical `/usr/share/wordlists/rockyou.txt`.

When adding a tool: prefer a Kali apt package (verify on pkg.kali.org); if it caches/downloads on first use, add a warm-up line in the post-`USER` block; add the wrapper to the relevant `tools/*.py`; document it in `docs/USAGE.md`.
