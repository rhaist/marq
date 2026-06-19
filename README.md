# marq

> ⚠️ **Authorized testing only.** This image bundles live offensive tooling.
> Read [`docs/SECURITY.md`](docs/SECURITY.md) first and have written
> authorization for every target you touch.

A Kali-based **penetration-testing & OSINT toolkit**, driven by an LLM through a
single, auditable **Go** binary. About 50 industry-standard tools — recon, OSINT,
web testing, exploitation and hash cracking — are exposed as typed tools with
**audit logging on every call**, **sandboxed file access**, and a **findings
report** as the deliverable.

**Two ways to run:**

- 🔌 **MCP server** — plug into any MCP client (Claude Desktop, LM Studio); the
  client brings the model. No network port is opened; it talks over stdio.
- 🖥️ **TUI agent host** — a self-contained terminal UI with its own tool-calling
  loop, driving a **local** model runtime (Ollama / llama.cpp).

---

## Contents

- [Features](#features)
- [Quick start](#quick-start)
- [The two run modes](#the-two-run-modes)
- [How it works](#how-it-works)
- [Documentation](#documentation)
- [Repository layout](#repository-layout)
- [License](#license)

## Features

- **Full tool suite** on a Kali base, grouped by phase:
  | Category | Tools |
  |----------|-------|
  | Recon / network | `nmap` `masscan` `naabu` `dnsx` `dnsrecon` `subfinder` `httpx` `whois` |
  | OSINT — org/domain | `theHarvester` `spiderfoot` `shodan` `gitleaks` `trufflehog` `gau` `waybackurls` `exiftool` |
  | OSINT — people | `sherlock` `maigret` `holehe` `h8mail` `phoneinfoga` |
  | Web app | `nuclei` `nikto` `feroxbuster` `katana` `ffuf` `gobuster` `arjun` `whatweb` `wafw00f` `cmseek` `wpscan` `testssl.sh` `dalfox` `sqlmap` |
  | Exploitation / creds | `metasploit` `hydra` `searchsploit` `john` `hashcat` `hashid` |
- **Findings deliverable** — `report_finding` / `render_report` capture validated
  issues (optional CVSS 3.1 vector → computed base score, plus CWE & references)
  and write a severity-sorted `findings.md` + `findings.csv`.
- **Skills library** — `load_skill` pulls technique / vuln-class playbooks (SQLi,
  XSS, SSRF, IDOR, recon, …) tied to these tool names, so even a local model
  chains the tools competently.
- **Background-job visibility** — `list_jobs` / `job_status` for long scans
  (e.g. spiderfoot) instead of polling files by hand.
- **Sandboxed working files** (`/work`, `/tmp`) — the model stages inputs and
  reads tool output back from disk, and nothing else.
- **Audit logging on every invocation** — append-only JSON lines with operator,
  engagement, target and the full argument vector.
- **Hardened container** — non-root, dropped capabilities (only what SYN scans
  need), `no-new-privileges`, and an opt-out raw-shell escape hatch.

## Quick start

```bash
# 1. Build (large image — Kali base + full tool suite; first build is slow)
docker build -t marq .

# 2. Sanity check
#    nmap/masscan/naabu carry file capabilities, so add the caps for those;
#    other tools run with a plain `docker run`.
docker run --rm --cap-add NET_RAW --cap-add NET_ADMIN marq nmap --version

# 3a. As an MCP server — merge mcp.json.example into your client's config,
#     set MARQ_SCOPE, then ask the model to call `server_info` first.
#
# 3b. As a local TUI agent — see docs/SETUP.md (needs Ollama on the host).
```

Full per-OS install (macOS + Debian Testing): **[`docs/SETUP.md`](docs/SETUP.md)**.
Tool reference, env vars & API keys: **[`docs/USAGE.md`](docs/USAGE.md)**.

## The two run modes

| | MCP server (`serve`) | TUI / agent host (`tui`, `agent`) |
|---|---|---|
| Who drives the model | An external MCP client | This binary's own loop |
| Needs a local model | No | Yes — Ollama / llama.cpp on the host |
| Transport | stdio JSON-RPC | OpenAI-compatible HTTP to the host |
| Use it when | You already have an MCP client | You want a self-contained tool |

Both consume the **same tool registry**, so every tool, resource and the audit
trail behave identically in either mode.

## How it works

```
MCP client ──stdio JSON-RPC──▶ docker run -i marq   (marq serve)
                                 └─ internal/mcpserver (go-sdk)
                                      └─ registry.All() ─ one Tool list, shared by both front-ends
                                           ├─ recon    nmap, naabu, dnsx, masscan…
                                           ├─ osint    theHarvester, spiderfoot, shodan, gitleaks…
                                           ├─ people   sherlock, maigret, holehe, phoneinfoga…
                                           ├─ web      nuclei, katana, feroxbuster, sqlmap…
                                           ├─ exploit  msf, hydra, searchsploit
                                           ├─ creds    john, hashcat, hashid
                                           ├─ files    read/write/list (sandboxed /work)
                                           ├─ extras   report_finding, render_report, list_jobs, job_status, load_skill
                                           └─ runner.Run ──▶ audit.jsonl (every call)
```

Every exec tool funnels through `internal/runner/runner.go::Run` — the single
point where audit logging, timeouts and output truncation happen (and where
you'd add hard scope-enforcement to move beyond logging-only guardrails).

## Documentation

| Doc | What's in it |
|-----|--------------|
| [`docs/SETUP.md`](docs/SETUP.md) | Per-OS install for macOS & Debian Testing, both run modes |
| [`docs/USAGE.md`](docs/USAGE.md) | Every tool, env vars, API keys, reading the audit log |
| [`docs/SECURITY.md`](docs/SECURITY.md) | Legal/ethical baseline, the guardrail model, hardening |

## Repository layout

```
Dockerfile            golang builder + Kali full-suite image, hardened, non-root
docker-compose.yml    Build / interactive-shell convenience + hardening flags
mcp.json.example      Drop-in MCP client config
cmd/marq/          CLI entry: serve | tui | agent | run
internal/
  config/             Env-driven configuration
  audit/              Append-only JSON-lines audit log
  runner/             Shared exec runner (audit + timeout + truncation + background)
  registry/           The tool suite as data (recon, osint, people, web, …)
  mcpserver/          MCP stdio adapter (modelcontextprotocol/go-sdk)
  agent/  tui/        Tool-calling loop + Bubble Tea front-end (local-model mode)
  files/              Sandboxed /work + /tmp access
  findings/  jobs/    Findings deliverable + background-job status
  skills/             Embedded technique / vuln-class playbooks
docs/                 SETUP.md, USAGE.md, SECURITY.md
```

## License

MIT — see [`LICENSE`](LICENSE). Provided for authorized security testing and
education. No warranty; use responsibly and legally.
