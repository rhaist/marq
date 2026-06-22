# Usage

How to build, smoke-test and drive the toolkit. For a guided per-OS install
(macOS / Debian Testing) see [`SETUP.md`](SETUP.md); for the safety model see
[`SECURITY.md`](SECURITY.md).

**Contents**

- [1. Build the image](#1-build-the-image)
- [2. Smoke-test the image](#2-smoke-test-the-image)
- [3. Wire it into an MCP client](#3-wire-it-into-an-mcp-client)
- [4. Agent host (TUI) with a local model](#4-agent-host-tui-with-a-local-model)
- [Available tools](#available-tools)
- [API keys for OSINT sources](#api-keys-for-osint-sources)
- [Environment variables](#environment-variables)
- [Reading the audit log](#reading-the-audit-log)
- [GPU hash cracking (optional)](#gpu-hash-cracking-optional)

## 1. Build the image

```bash
docker build -t marq .
```

This pulls the Kali base and installs the full tool suite, so it is large
(multi-GB) and the first build takes a while.

**Setup baked in at build time.** A few tools fetch data or build a cache on
first use; the Dockerfile does this during the build (as the runtime
`marq` user, so it lands in that user's home) to avoid a slow,
network-dependent first scan:

- `nuclei` — the template repository (`nuclei -update-templates`)
- `wpscan` — the WordPress vulnerability database (`wpscan --update`)
- `metasploit` — the module cache (a one-shot `msfconsole` run)
- `rockyou` — decompressed to `/usr/share/wordlists/rockyou.txt`

These steps are best-effort: a network hiccup during build won't fail the
image — the tool just downloads on first run instead. To refresh the data in a
running container later, re-run e.g. `nuclei -update-templates` or
`wpscan --update`. All other tools ship their data bundled and need no setup.

## 2. Smoke-test the image

Run a one-off tool to confirm the image works:

```bash
# nmap/masscan/naabu carry file capabilities (cap_net_admin) that a bare
# container won't exec — pass the caps for those:
docker run --rm --cap-add NET_RAW --cap-add NET_ADMIN marq nmap --version
# tools without special capabilities run with a plain docker run:
docker run --rm marq searchsploit --help
```

Get an interactive shell:

```bash
docker run --rm -it --entrypoint /bin/bash marq
```

Confirm the MCP server starts (it will wait for JSON-RPC on stdin; Ctrl-C to exit):

```bash
docker run --rm -i marq        # prints the authorization banner to stderr
```

## 3. Wire it into an MCP client

These steps use LM Studio; Claude Desktop and other MCP clients are equivalent
(point them at the same `docker run` command).

1. In LM Studio open the MCP config (**Program → Edit `mcp.json`**, or the
   "Integrations" panel).
2. Merge the `marq` entry from [`mcp.json.example`](../mcp.json.example)
   into your `mcpServers`.
3. Edit the `env` block — set `MARQ_OPERATOR`, `MARQ_ENGAGEMENT`
   and especially `MARQ_SCOPE` to your authorized targets.
4. Save and toggle the server on. Load a tool-use-capable model.
5. Ask the model to call `server_info` first — it returns the authorization
   banner and confirms scope before any scanning.

## 4. Agent host (TUI) with a local model

Instead of an external MCP client, the same binary can drive a **local** model
itself, with its own tool-calling loop and a terminal UI. The model runs on the
**host** (Docker on Apple Silicon has no GPU passthrough), reached over
`host.docker.internal`.

```bash
# On the host: load an uncensored, tool-calling model and start a local
# OpenAI-compatible server. marq defaults to LM Studio (port 1234); Ollama
# (port 11434) and llama.cpp's llama-server work too — pick either in the TUI.
#   - LM Studio (default): download the app, load a model, Developer → start server
#   - Ollama:  ollama pull huihui_ai/Qwen3.6-abliterated:27b   # ~17 GB; strong tool-calling
#   lighter / cyber-specialist option: WhiteRabbitNeo-V3-7B (~5 GB)

# Run the TUI agent host (model on host, tools in the container). With no
# MARQ_MODEL_* set, it defaults to LM Studio on host.docker.internal:1234 and
# opens the setup screen to pick the model. To pin it explicitly:
docker run --rm -it \
  -e MARQ_MODEL_URL=http://host.docker.internal:1234/v1 \
  -e MARQ_MODEL=<your-loaded-model-id> \
  -e MARQ_OPERATOR=your-name -e MARQ_ENGAGEMENT=acme-2026 \
  -e MARQ_SCOPE="*.example.com — per SOW" \
  -v "$PWD/work:/work" \
  marq tui

# Headless equivalent (prints each step) — also the way to validate that the
# chosen model does reliable multi-tool calling:
docker run --rm -i ... marq agent "footprint example.com, report findings"
```

The agent gets the authorization banner + methodology as its system prompt,
calls `server_info` first, works the tools, records findings with
`report_finding`, and writes the report with `render_report` into `/work`.

### In-TUI setup (interactive alternative to the env vars above)

On first launch (no saved config), `marq tui` opens a **setup screen** instead
of going straight to the chat:

1. **Backend** — cycle with ←/→ between `LM Studio` (the default), `Ollama`,
   and `Custom`. Picking one fills in its default Base URL + API key (LM Studio
   uses `http://host.docker.internal:1234/v1` and key `lm-studio`; Ollama uses
   `:11434/v1` and key `ollama`).
2. Edit **Base URL**, **Model**, **API key**, **Operator**, **Engagement**,
   and **Scope** (Tab/↑↓ to move between fields).
3. **Enter** probes the endpoint with `GET /v1/models` (using the same client
   the agent loop uses) and shows `✓ connected — N models available` or the
   error. **Ctrl+S** saves without probing.
4. From the result line: **Enter** to proceed, **e** to edit, **r** to retry.

Choices persist to `$MARQ_WORK_DIR/.marq/tui.json` (override with
`MARQ_TUI_CONFIG`), so the next launch skips setup and goes straight to chat.
**Ctrl+S from the chat view re-opens setup**, preloaded with the current
config. Env vars (`MARQ_MODEL_URL`, `MARQ_MODEL`, …) always override the saved
file when set, so automated runs stay reproducible.

> **Linux note:** under `--network=host` change the Base URL to
> `http://localhost:1234/v1` (or `:11434` for Ollama) in the setup screen —
> the container shares the host namespace so there's no `host.docker.internal`.

## Available tools

### Recon / network

| Tool           | Wraps     | Purpose                           |
| -------------- | --------- | --------------------------------- |
| `server_info`  | —         | Show authorization banner + scope |
| `nmap`         | nmap      | Port/service scanning             |
| `masscan`      | masscan   | Fast port sweeps                  |
| `naabu`        | naabu     | Fast modern port scan (top-ports) |
| `dns_lookup`   | dig       | DNS records                       |
| `dnsx`         | dnsx      | Bulk DNS resolution / record enum |
| `dnsrecon`     | dnsrecon  | DNS recon, zone transfer, brute   |
| `whois_lookup` | whois     | Registration data                 |
| `subfinder`    | subfinder | Passive subdomain enum            |
| `httpx_probe`  | httpx     | Live HTTP probing / tech detect   |

### Information gathering — company & domain footprint

| Tool            | Wraps        | Purpose                                                             |
| --------------- | ------------ | ------------------------------------------------------------------- |
| `theharvester`  | theHarvester | Emails, employees, hosts, subdomains                                |
| `spiderfoot`    | spiderfoot   | Broad automated OSINT footprint (background; poll with `read_file`) |
| `exif_metadata` | exiftool     | Metadata from a staged file/dir                                     |
| `shodan_host`   | shodan       | Exposed ports/services for an IP †                                  |
| `shodan_search` | shodan       | Search exposed assets (e.g. `org:`) †                               |
| `gitleaks`      | gitleaks     | Secrets in a local git repo/dir                                     |
| `trufflehog`    | trufflehog   | Verified leaked secrets (git/GitHub) ‡                              |
| `wayback_urls`  | waybackurls  | Historical URLs from the Wayback Machine                            |
| `gau_urls`      | gau          | Known URLs (Wayback/CommonCrawl/OTX)                                |

### Information gathering — people footprint

| Tool               | Wraps       | Purpose                             |
| ------------------ | ----------- | ----------------------------------- |
| `sherlock`         | sherlock    | Username across ~400 sites          |
| `maigret_username` | maigret     | Deep username sweep (~2500 sites)   |
| `holehe_email`     | holehe      | Sites where an email has an account |
| `h8mail_breach`    | h8mail      | Email breach/leak exposure †        |
| `phoneinfoga`      | phoneinfoga | Phone number OSINT                  |

### Web application

| Tool           | Wraps       | Purpose                          |
| -------------- | ----------- | -------------------------------- |
| `nuclei`       | nuclei      | Template-based vuln scanning     |
| `nikto`        | nikto       | Web server scanning              |
| `ffuf`         | ffuf        | Content/dir fuzzing              |
| `gobuster_dir` | gobuster    | Directory brute force            |
| `feroxbuster`  | feroxbuster | Fast recursive content discovery |
| `katana`       | katana      | Endpoint/JS crawler              |
| `arjun`        | arjun       | Hidden HTTP parameter discovery  |
| `whatweb`      | whatweb     | Web tech fingerprinting          |
| `wafw00f`      | wafw00f     | WAF detection / fingerprint      |
| `cmseek`       | CMSeeK      | CMS detection (180+ CMSs)        |
| `wpscan`       | wpscan      | WordPress scanning               |
| `testssl`      | testssl.sh  | SSL/TLS configuration analysis   |
| `dalfox`       | dalfox      | XSS scanning                     |
| `sqlmap`       | sqlmap      | SQL injection testing            |

### Exploitation / credentials

| Tool            | Wraps      | Purpose                    |
| --------------- | ---------- | -------------------------- |
| `searchsploit`  | exploitdb  | Local exploit DB search    |
| `msfconsole`    | metasploit | Run a resource script      |
| `hydra`         | hydra      | Online credential testing  |
| `john`          | john       | Offline hash cracking      |
| `hashcat`       | hashcat    | GPU/CPU hash cracking      |
| `hash_identify` | hashid     | Identify hash type         |
| `run_shell`     | bash       | Arbitrary command (opt-in) |

### Working files (`/work`, `/tmp`)

| Tool         | Purpose                                                          |
| ------------ | ---------------------------------------------------------------- |
| `list_dir`   | List a directory in the working area                             |
| `read_file`  | Read back output a tool wrote to disk (cmseek JSON, `nuclei -o`) |
| `write_file` | Stage an input file (a hash for `john`, a target list, …)        |

These are sandboxed to `/work` and `/tmp` — the model cannot read or write
anywhere else. They are what make the file-driven tools usable: the model can
`write_file` a captured hash then `john` it, or `read_file` a result another
tool dropped on disk. Mount `/work` from the host (see `docker-compose.yml`) to
exchange files with the operator.

### Findings, jobs & knowledge

| Tool             | Purpose                                                                                                                                           |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `report_finding` | Record a validated issue (title, severity, target, evidence, recommendation; optional CVSS 3.1 vector, CWE, references) to `/work/findings.jsonl` |
| `render_report`  | Write the severity-sorted `findings.md` + `findings.csv` deliverable                                                                              |
| `list_jobs`      | List background jobs and whether each is running or done                                                                                          |
| `job_status`     | A background job's state (running/done + exit code) + output tail                                                                                 |
| `load_skill`     | Load a technique/vuln-class playbook (sqli, xss, ssrf, idor, recon-footprint, …) tied to these tool names                                         |

A `report_finding` with a `cvss` vector (e.g. `CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H`)
gets its base score computed. Your explicit `severity` stays authoritative; the
vector only sets severity when you omit one, and a divergent vector is flagged in
the report rather than overriding your call. The skills library is also exposed as
MCP resources: `marq://skills` (index) and `marq://skills/<name>`.

**Background scans.** Tools too slow for a synchronous call (currently
`spiderfoot`) launch in the background and return a job directory under
`/work/jobs/<tool>-<id>/`. Results stream to `stdout.log`, diagnostics to
`stderr.log`, and a `status` file (containing `exit=<code>`) appears when the
scan finishes. Check progress with `list_jobs` / `job_status` (or `read_file` /
`list_dir`) — the work survives individual tool-call timeouts.

† Needs an API key (see _API keys_ below). ‡ GitHub org/repo scans need `GITHUB_TOKEN`.

## API keys for OSINT sources

Several information-gathering tools return far richer data with API keys. Pass
them into the container via the `env` block in `mcp.json` (and add matching
`-e VAR` passthrough args). All are optional — without them the tools fall back
to keyless sources or return limited results.

| Variable                              | Used by                        |
| ------------------------------------- | ------------------------------ |
| `SHODAN_API_KEY`                      | `shodan_host`, `shodan_search` |
| `CENSYS_API_ID` / `CENSYS_API_SECRET` | censys (via raw shell)         |
| `GITHUB_TOKEN`                        | `trufflehog` (GitHub scans)    |
| `NUMVERIFY_API_KEY`                   | `phoneinfoga`                  |
| `GOOGLE_API_KEY` / `GOOGLECSE_CX`     | `phoneinfoga` (Google CSE)     |

theHarvester and h8mail read their own config files (`~/.theHarvester/api-keys.yaml`,
an h8mail config passed via `options`) for Hunter, SecurityTrails, HIBP, etc.

## Environment variables

| Variable               | Default                                | Meaning                                                          |
| ---------------------- | -------------------------------------- | ---------------------------------------------------------------- |
| `MARQ_OPERATOR`        | `unknown`                              | Recorded in every audit record                                   |
| `MARQ_ENGAGEMENT`      | `unspecified`                          | Engagement / SOW identifier                                      |
| `MARQ_SCOPE`           | `""`                                   | Free-text authorized scope (banner + log)                        |
| `MARQ_AUDIT_LOG`       | `/var/log/marq/audit.jsonl`            | Audit log path                                                   |
| `MARQ_TIMEOUT`         | `900`                                  | Default per-command timeout (seconds)                            |
| `MARQ_MAX_TIMEOUT`     | `3600`                                 | Ceiling for a tool's per-call timeout override                   |
| `MARQ_MAX_OUTPUT`      | `60000`                                | Max output chars returned to the model                           |
| `MARQ_ALLOW_RAW_SHELL` | `true`                                 | Expose the arbitrary-shell tool (set `false` to disable)         |
| `MARQ_WORK_DIR`        | `/work`                                | Working area (findings, job dirs)                                |
| `MARQ_MODEL_URL`       | `http://host.docker.internal:1234/v1`  | Agent/TUI: OpenAI-compatible model endpoint (default: LM Studio) |
| `MARQ_MODEL`           | _(none — set to your loaded model id)_ | Agent/TUI: model name (pick in the TUI or set for headless)      |
| `MARQ_MODEL_KEY`       | `lm-studio`                            | Agent/TUI: API key (local runtimes ignore it)                    |
| `MARQ_TUI_CONFIG`      | `<MARQ_WORK_DIR>/.marq/tui.json`       | Where the TUI setup form persists; set to skip/force re-setup    |

## Reading the audit log

```bash
# Persisted to ./audit/ via the compose/volume mount.
cat audit/audit.jsonl | jq 'select(.event=="invocation.start") | {ts, tool, target, argv}'
```

## GPU hash cracking (optional)

`hashcat` benefits from a GPU. Pass it through at run time, e.g. with the
NVIDIA container toolkit: add `--gpus all` to the `docker run` args.
