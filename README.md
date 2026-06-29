<div align="center">

<img src="docs/logo.svg" alt="marq" width="320">

### Your universal cyber assistant

Ask anything across security and get the right tools and current expertise to
answer it — on whatever model you choose, local or frontier.

[![CI](https://github.com/rhaist/marq/actions/workflows/ci.yml/badge.svg)](https://github.com/rhaist/marq/actions/workflows/ci.yml)

</div>

```text
"Threat-model this payment API."                       → architecture
"Map our pentest findings to ISO 27001 and SOC 2."     → GRC + standards
"What does this malware sample actually do?"           → malware
"Run an authorized pentest of *.acme.com."             → offensive
"Our breach-notification clock under GDPR + US law?"   → regulation
"Build the ransomware incident runbook."               → IR + resilience
```

**~80 security tools + ~70 expert playbooks across 14 domains, in one auditable
binary.** You bring the brain — [Claude Code or Codex](docs/CLIENTS.md) for deep
reasoning, a local uncensored model in [Pi](https://pi.dev/) for hands-on
offensive work, LM Studio to test.

> **Authorized use.** Advisory and knowledge work is open; scanning and
> exploitation are authorized-only and audit-logged — see
> [`docs/SECURITY.md`](docs/SECURITY.md).

---

## Why

marq is two layers your model drives:

- **Execution** — ~80 wrapped Kali tools (recon, web, AD/internal, exploitation,
  hash cracking, malware static analysis, OSINT) called as typed tools. Every
  call is audit-logged, file access sandboxed, findings render to a report.
- **Knowledge** — ~70 on-demand skill playbooks across 14 domains (offensive,
  malware, threat-intel, sec-ops, architecture, GRC, standards & regulation
  (EU + US), CISO, red/purple teaming, resilience, human factors,
  DevSecOps/privacy), web-researched against current (2026) standards. Loaded
  with `load_skill`, they tell the model _how_ to use the tools and _what_ to do
  where there's no tool to run.

One binary, two ways in, one registry.

| Way in                        | What it is                                                          | Drives the model                       |
| :---------------------------- | :------------------------------------------------------------------ | :------------------------------------- |
| **MCP server** (`marq serve`) | Stdio JSON-RPC; plug into Claude Desktop, LM Studio, any MCP client | The client                             |
| **Direct run** (`marq run`)   | Run one tool by name; a terminal agent calls it from bash via `pi/` | A local model in [Pi](https://pi.dev/) |

marq runs the tools. The model and the agent loop live in the client, so
every tool, resource, and audit record behaves the same either way.

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

**Pick the brain for the job** — marq brings the tools and knowledge; your client
brings the model:

- **Claude Code / Codex** (frontier model = the expert) — `claude mcp add` /
  `codex mcp add` and point it at the [`mcp.json.example`](mcp.json.example) args.
  Best for the reasoning-heavy domains: architecture, GRC, threat modeling, IR
  leadership, writing the report.
- **Pi** (local/abliterated model = standalone, uncensored) — install the
  `pi/marq` shim, `marq up <dir>`, load [`pi/SKILL.md`](pi/SKILL.md). Best for
  hands-on offensive ops with nothing leaving the box.
- **LM Studio** (local model = testing) — paste the `marq` entry into `mcp.json`.

Full guidance — which client and model for which work — in
**[`docs/CLIENTS.md`](docs/CLIENTS.md)**. Per-OS install:
**[`docs/SETUP.md`](docs/SETUP.md)**. Tools, env vars & API keys:
**[`docs/USAGE.md`](docs/USAGE.md)**.

---

## Features

### Full tool suite on a Kali base

| Category                              | Tools                                                                                                                                                                                                                                                                            |
| :------------------------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Recon / network**                   | `nmap` · `masscan` · `naabu` · `dnsx` · `dnsrecon` · `subfinder` · `httpx_probe` · `dns_lookup` · `whois_lookup` · `ssh_audit` · `fping_sweep` · `snmp_walk` · `snmp_check` · `snmp_brute` · `smtp_user_enum` · `smtp_test` · `asnmap` · `cdncheck` · `censys_search`            |
| **OSINT — org/domain**                | `theharvester` · `spiderfoot` · `shodan_host` · `shodan_search` · `gitleaks` · `trufflehog` · `gau_urls` · `wayback_urls` · `exif_metadata`                                                                                                                                      |
| **OSINT — people**                    | `sherlock` · `maigret_username` · `holehe_email` · `h8mail_breach` · `phoneinfoga`                                                                                                                                                                                               |
| **Web app**                           | `nuclei` · `nikto` · `feroxbuster` · `katana` · `ffuf` · `gobuster_dir` · `arjun` · `whatweb` · `wafw00f` · `cmseek` · `wpscan` · `testssl` · `dalfox` · `sqlmap` · `jwt_tool` · `trivy` · `interactsh` · `subjack` · `paramspider` · `sstimap`                                  |
| **Exploitation**                      | `donut` · `hydra` · `searchsploit`                                                                                                                                                                                                                                               |
| **AD / internal network**             | `impacket_secretsdump` · `impacket_kerberoast` · `impacket_asreproast` · `impacket_psexec` · `impacket_wmiexec` · `impacket_ntlmrelayx` · `netexec` · `certipy_find` · `bloodhound_collect` · `evil_winrm` · `enum4linux` · `smb_enum` · `ldap_search` · `responder` · `nbtscan` |
| **Credentials**                       | `john` · `hashcat` · `hash_identify`                                                                                                                                                                                                                                             |
| **Malware research** (static)         | `capa` · `yara_scan` · `olevba` · `bin_headers`                                                                                                                                                                                                                                  |
| **Files** (sandboxed `/work`, `/tmp`) | `list_dir` · `read_file` · `write_file`                                                                                                                                                                                                                                          |
| **Findings & jobs**                   | `report_finding` · `render_report` · `list_jobs` · `job_status`                                                                                                                                                                                                                  |
| **Knowledge**                         | `load_skill`                                                                                                                                                                                                                                                                     |
| **Escape hatch** (opt-in)             | `run_shell`                                                                                                                                                                                                                                                                      |

### Built for engagements

- **Findings deliverable** — `report_finding` / `render_report` capture
  validated issues (optional CVSS 3.1 vector → computed base score, plus CWE
  & references) and write a severity-sorted `findings.md` + `findings.csv`.
- **~70-skill library across 14 domains** — `load_skill` pulls playbooks for
  offensive, malware, threat-intel, sec-ops, architecture, GRC, standards &
  regulation (EU + US), CISO, red/purple teaming, resilience, physical/human, and
  DevSecOps/privacy — web-researched against current standards, tied to the tool
  names and findings output, so a small model knows what to do where there's no tool.
- **Background-job visibility** — `list_jobs` / `job_status` for long scans
  (e.g. spiderfoot) instead of polling files by hand.
- **Sandboxed working files** (`/work`, `/tmp`) — the model stages inputs and
  reads output back from disk, nowhere else.
- **Audit log on every call** — append-only JSON lines with operator,
  engagement, target, and the full argument vector. `fsync`'d per write.
- **Hardened container** — non-root, capabilities dropped (only what SYN
  scans need), `no-new-privileges`, opt-out raw-shell escape hatch.

---

## How it works

```
MCP client  ──stdio JSON-RPC──▶  marq serve  ┐
terminal agent (Pi) ──bash──▶  pi/marq shim ─┤   both hit the same registry
                              (docker exec)  │
                                             ▼
                              registry.All() ── one Tool list
                                ├─ recon     nmap, naabu, dnsx, masscan…
                                ├─ osint     theHarvester, spiderfoot, shodan, gitleaks…
                                ├─ people    sherlock, maigret, holehe, phoneinfoga…
                                ├─ web       nuclei, katana, feroxbuster, sqlmap…
                                ├─ exploit   donut, hydra, searchsploit
                                ├─ internal  impacket, netexec, certipy, bloodhound, enum4linux…
                                ├─ creds     john, hashcat, hashid
                                ├─ malware   capa, yara_scan, olevba, bin_headers
                                ├─ files     read/write/list (sandboxed /work)
                                └─ extras    report_finding, render_report, list_jobs, load_skill
                                             │
                                  runner.Run ──▶ audit.jsonl (every call)

           load_skill ──▶ skills library (markdown, ~70 playbooks / 14 domains:
                          offensive, malware, threat-intel, sec-ops, architecture,
                          GRC, standards, CISO, resilience, human factors, …)
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

## License

**GNU AGPLv3** — see [`LICENSE`](LICENSE). Copyright © 2026 marq contributors.
marq is free software: you may use, study, modify and share it under the AGPLv3;
if you run a modified version as a network service, you must offer your users its
source. Provided for authorized security testing and education, **with no
warranty** — use responsibly and legally.
