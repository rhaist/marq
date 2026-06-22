<div align="center">

<img src="docs/logo.svg" alt="marq" width="320">

A Kali-based penetration-testing & OSINT toolkit, driven by an LLM
through a single, auditable Go binary.

[![CI](https://github.com/rhaist/marq/actions/workflows/ci.yml/badge.svg)](https://github.com/rhaist/marq/actions/workflows/ci.yml)

</div>

> **Authorized testing only.** This image bundles live offensive tooling.
> Read [`docs/SECURITY.md`](docs/SECURITY.md) first, and have written
> authorization for every target you touch.

---

## Why

50 industry-standard tools — recon, OSINT, web testing, exploitation and
hash cracking — exposed as **typed tools** an LLM can call, with **audit
logging on every invocation**, **sandboxed file access**, and a
**findings report** as the engagement deliverable. One binary, two run
modes, one tool registry.

| Mode                            | What it is                                                                     | Who drives the model                                                                   |
| :------------------------------ | :----------------------------------------------------------------------------- | :------------------------------------------------------------------------------------- |
| **MCP server** (`marq serve`)   | Stdio JSON-RPC server — plug into Claude Desktop, LM Studio, or any MCP client | The external client brings its own model                                               |
| **TUI agent host** (`marq tui`) | Self-contained terminal UI with its own tool-calling loop                      | A local model runtime on the host (LM Studio by default; Ollama / llama.cpp also work) |

Both consume the **same tool registry**, so every tool, resource, and
audit-record behaves identically in either mode.

---

## Quick start

```bash
# 1. Build the image (large — Kali base + full tool suite; first build is slow)
docker build -t marq .

# 2. Smoke-test a tool (nmap/masscan/naabu need file-capability passthrough;
#    other tools run with a plain `docker run`)
docker run --rm --cap-add NET_RAW --cap-add NET_ADMIN marq nmap --version

# 3. Run the MCP server (waits for JSON-RPC on stdin; banner → stderr)
docker run --rm -i marq
```

**Next steps:**

- **MCP server** — merge [`mcp.json.example`](mcp.json.example) into your
  client's config, set `MARQ_SCOPE`, and ask the model to call `server_info`
  first.
- **TUI agent** — see [`docs/SETUP.md`](docs/SETUP.md) (needs a local model
  runtime on the host — LM Studio by default, Ollama also supported).

Full per-OS install (macOS + Debian Testing): **[`docs/SETUP.md`](docs/SETUP.md)**.
Tool reference, env vars & API keys: **[`docs/USAGE.md`](docs/USAGE.md)**.

---

## Features

### Full tool suite on a Kali base

| Category                              | Tools                                                                                                                                                             |
| :------------------------------------ | :---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Recon / network**                   | `nmap` · `masscan` · `naabu` · `dnsx` · `dnsrecon` · `subfinder` · `httpx_probe` · `dns_lookup` · `whois_lookup`                                                  |
| **OSINT — org/domain**                | `theharvester` · `spiderfoot` · `shodan_host` · `shodan_search` · `gitleaks` · `trufflehog` · `gau_urls` · `wayback_urls` · `exif_metadata`                       |
| **OSINT — people**                    | `sherlock` · `maigret_username` · `holehe_email` · `h8mail_breach` · `phoneinfoga`                                                                                |
| **Web app**                           | `nuclei` · `nikto` · `feroxbuster` · `katana` · `ffuf` · `gobuster_dir` · `arjun` · `whatweb` · `wafw00f` · `cmseek` · `wpscan` · `testssl` · `dalfox` · `sqlmap` |
| **Exploitation**                      | `msfconsole` · `hydra` · `searchsploit`                                                                                                                           |
| **Credentials**                       | `john` · `hashcat` · `hash_identify`                                                                                                                              |
| **Files** (sandboxed `/work`, `/tmp`) | `list_dir` · `read_file` · `write_file`                                                                                                                           |
| **Findings & jobs**                   | `report_finding` · `render_report` · `list_jobs` · `job_status`                                                                                                   |
| **Knowledge**                         | `load_skill`                                                                                                                                                      |
| **Escape hatch** (opt-in)             | `run_shell`                                                                                                                                                       |

### Built for engagements

- **Findings deliverable** — `report_finding` / `render_report` capture
  validated issues (optional CVSS 3.1 vector → computed base score, plus CWE
  & references) and write a severity-sorted `findings.md` + `findings.csv`.
- **Skills library** — `load_skill` pulls technique / vuln-class playbooks
  (SQLi, XSS, SSRF, IDOR, recon, …) tied to these tool names, so even a local
  model chains the tools competently.
- **Background-job visibility** — `list_jobs` / `job_status` for long scans
  (e.g. spiderfoot) instead of polling files by hand.
- **Sandboxed working files** (`/work`, `/tmp`) — the model stages inputs and
  reads tool output back from disk, and nothing else.
- **Audit logging on every invocation** — append-only JSON lines with
  operator, engagement, target, and the full argument vector. `fsync`'d per
  write; survives a crash.
- **Hardened container** — non-root, capabilities dropped (only what SYN
  scans need), `no-new-privileges`, opt-out raw-shell escape hatch.

---

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
point where audit logging, timeouts, and output truncation happen (and where
hard scope-enforcement would go to move beyond logging-only guardrails).

---

## Documentation

| Doc                                    | What's in it                                              |
| :------------------------------------- | :-------------------------------------------------------- |
| [`docs/SETUP.md`](docs/SETUP.md)       | Per-OS install for macOS & Debian Testing, both run modes |
| [`docs/USAGE.md`](docs/USAGE.md)       | Every tool, env vars, API keys, reading the audit log     |
| [`docs/SECURITY.md`](docs/SECURITY.md) | Legal/ethical baseline, the guardrail model, hardening    |

---

## Repository layout

```
Dockerfile            golang builder + Kali full-suite image, hardened, non-root
docker-compose.yml    Build / interactive-shell convenience + hardening flags
mcp.json.example      Drop-in MCP client config
cmd/marq/             CLI entry: serve | tui | agent | run
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

---

## License

MIT — see [`LICENSE`](LICENSE). Provided for authorized security testing and
education. No warranty; use responsibly and legally.
