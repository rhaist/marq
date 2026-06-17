# pentest-mcp

A Dockerized **penetration-testing toolkit** that drives industry-standard
offensive tools — recon, web testing, exploitation and hash cracking — from a
single, auditable **Go** binary. It runs either as an **MCP server** for any MCP
client (Claude Desktop, LM Studio) that brings its own model, or *(in progress)*
as a self-contained **TUI agent host** driving a local model runtime.

> ⚠️ **Authorized testing only.** This image contains active attack tooling.
> Read [`docs/SECURITY.md`](docs/SECURITY.md) first. You are responsible for
> having written authorization for every target you touch.

## What you get

- **Full tool suite** on a Kali base, best-in-class per category:
  - *Recon/network*: `nmap`, `masscan`, `naabu`, `dnsx`, `dnsrecon`, `subfinder`, `httpx`
  - *OSINT — company/domain footprint*: `theHarvester`, `spiderfoot`,
    `exiftool`, `shodan`, `gitleaks`, `trufflehog`, `gau`, `waybackurls`
  - *OSINT — people footprint*: `sherlock`, `maigret`, `holehe`, `h8mail`,
    `phoneinfoga`
  - *Web app*: `nuclei`, `nikto`, `feroxbuster`, `katana`, `ffuf`, `gobuster`,
    `arjun`, `whatweb`, `wafw00f`, `cmseek`, `wpscan`, `testssl.sh`, `dalfox`, `sqlmap`
  - *Exploitation/creds*: `metasploit`, `hydra`, `searchsploit`, `john`, `hashcat`, `hashid`
- **Sandboxed working-file access** (`/work`, `/tmp`) so the model can stage
  inputs (hashes, target lists) and read back outputs tools write to disk.
- **Findings deliverable** — `report_finding` / `render_report` record validated
  issues and write a severity-sorted `findings.md` + `findings.csv`.
- **Background-job visibility** — `list_jobs` / `job_status` for long scans
  (e.g. spiderfoot) instead of polling files by hand.
- **MCP over stdio** (Go, official `modelcontextprotocol/go-sdk`). No network
  port is opened — the client launches the container and talks over stdin/stdout.
- **Audit logging on every invocation** — JSON-lines, append-only, with
  operator, engagement, target and full argument vector.
- **Hardened container** — non-root user, dropped capabilities (only the few
  needed for SYN scans), `no-new-privileges`, audit-logged raw shell (default
  on; disable with `PENTEST_MCP_ALLOW_RAW_SHELL=false`).

## Quick start

```bash
# 1. Build (large image — pulls Kali + full tool suite)
docker build -t pentest-mcp .

# 2. Sanity check
docker run --rm pentest-mcp nmap --version

# 3. Add to LM Studio's mcp.json (see mcp.json.example), set your scope/env,
#    enable the server, then ask the model to call `server_info` first.
```

Full per-OS install (macOS + Debian Testing), including the local-model agent/TUI
mode: [`docs/SETUP.md`](docs/SETUP.md). Tool reference + env vars:
[`docs/USAGE.md`](docs/USAGE.md).

## How it fits together

```
MCP client  ──stdio JSON-RPC──▶  docker run -i pentest-mcp   (pentest serve)
                                   └─ internal/mcpserver (go-sdk)
                                        └─ registry.All() ── one Tool list, shared by both front-ends
                                             ├─ recon    nmap, naabu, dnsx, masscan…
                                             ├─ osint     theHarvester, spiderfoot, shodan, gitleaks…
                                             ├─ people    sherlock, maigret, holehe, phoneinfoga…
                                             ├─ web       nuclei, katana, feroxbuster, sqlmap…
                                             ├─ exploit   msf, hydra, searchsploit
                                             ├─ creds     john, hashcat, hashid
                                             ├─ files     read/write/list (sandboxed /work)
                                             ├─ extras    report_finding, render_report, list_jobs, job_status
                                             └─ runner.Run ──▶ audit.jsonl (every call)
```

Every exec tool funnels through `internal/runner/runner.go::Run`, the single
point where auditing happens (and where you'd add hard scope-enforcement to move
beyond logging-only guardrails).

## Repository layout

```
Dockerfile            golang builder + Kali full-suite image, hardened, non-root
docker-compose.yml    Build + interactive-shell convenience, hardening flags
mcp.json.example      Drop-in MCP client config
cmd/pentest/          CLI entry: serve | tui | run
internal/
  config/             Env-driven configuration (config.C)
  audit/              Append-only JSON-lines audit log
  runner/             Shared exec runner (audit + timeout + truncation + background)
  registry/           The tool suite as data; recon/osint/people/web/exploit/creds/files/extras/shell
  mcpserver/          MCP stdio adapter (modelcontextprotocol/go-sdk)
  files/              Sandboxed /work + /tmp file access
  findings/ jobs/     Findings deliverable + background-job status
docs/                 SECURITY.md, USAGE.md
```

## License

MIT — see [`LICENSE`](LICENSE). Provided for authorized security testing and
education. No warranty; use responsibly and legally.
