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

**~80 security tools + ~70 expert playbooks (current 2026 standards), in one
auditable binary.** You bring the model; marq brings the tools and the knowledge.

## Start

```bash
# 1. Get marq (prebuilt image — or build from source: docker build -t marq .)
docker pull ghcr.io/rhaist/marq && docker tag ghcr.io/rhaist/marq marq

# 2. Plug into Claude Code (any MCP client works)
claude mcp add marq -- docker run --rm -i \
  -v marq-audit:/var/log/marq -v marq-work:/work -e MARQ_OPERATOR=marq marq
```

Restart the client and ask: _"Load the nis2-dora skill — what's our breach
clock?"_ or _"Set scope to scanme.nmap.org, then run a quick nmap."_ (Scanning
tools also need `--cap-add NET_RAW --cap-add NET_ADMIN`; full hardened args in
[`mcp.json.example`](mcp.json.example).)

**Two ways to use it:**

- **Frontier brain** — Claude Code / Codex for reasoning-heavy work (architecture,
  GRC, threat modeling, IR, the report). → [`docs/CLIENTS.md`](docs/CLIENTS.md)
- **Local & uncensored** — a local model via Pi + llama.cpp for hands-on offensive
  ops, nothing leaving the box. → [`docs/SETUP.md`](docs/SETUP.md#4-run-with-a-local-model-pi--the-marq-skill)

> **Authorized use.** Advisory/knowledge is open; scanning & exploitation are
> authorized-only and audit-logged — [`docs/SECURITY.md`](docs/SECURITY.md).

---

## What you get

- **Execution** — ~80 wrapped Kali tools (recon, web, AD/internal, exploitation,
  cred cracking, malware static analysis, OSINT), each audit-logged and sandboxed;
  findings render to `findings.md` / `.csv` (with optional CVSS + CWE).
- **Knowledge** — ~70 `load_skill` playbooks across 14 domains, web-researched to
  current (2026) standards, telling the model _how_ to use the tools and _what_ to
  do where there's no tool: offensive, malware, threat-intel, sec-ops, architecture,
  GRC, standards & EU/US regulation, CISO, red/purple, resilience, human factors,
  DevSecOps/privacy.
- **Safe by construction** — every call funnels through one choke point: an
  append-only audit log, timeouts, output caps, a `/work`+`/tmp` file sandbox, and
  a non-root, capability-dropped container.

<details>
<summary><b>Full tool suite</b> — ~80 tools on a Kali base (click to expand)</summary>

| Category                              | Tools                                                                                                                                                                                                                                                                            |
| :------------------------------------ | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
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

</details>

<details>
<summary><b>How it works</b> — one registry, two front-ends (click to expand)</summary>

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

           load_skill ──▶ skills library (markdown, ~70 playbooks / 14 domains)
```

Every exec tool funnels through `internal/runner/runner.go::Run` — the single
point for audit logging, timeouts, and output truncation.

</details>

---

## Documentation

| Doc                                    | What's in it                                                       |
| :------------------------------------- | :---------------------------------------------------------------- |
| [`docs/SETUP.md`](docs/SETUP.md)       | Per-OS install (macOS & Debian), both run modes                    |
| [`docs/CLIENTS.md`](docs/CLIENTS.md)   | Pick a client/model — Claude Code, Codex, Pi + llama.cpp           |
| [`docs/USAGE.md`](docs/USAGE.md)       | Every tool, env vars, API keys, reading the audit log              |
| [`docs/SECURITY.md`](docs/SECURITY.md) | Legal/ethical baseline, the guardrail model, hardening             |

## License

**GNU AGPLv3** — see [`LICENSE`](LICENSE). © 2026 marq contributors. Free software;
run a modified version as a network service and you must offer users its source.
For authorized security testing and education, **no warranty** — use responsibly.
