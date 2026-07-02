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
# Get marq (prebuilt image — or build from source: docker build -t marq .)
docker pull ghcr.io/rhaist/marq && docker tag ghcr.io/rhaist/marq marq
```

No Docker Desktop needed — **OrbStack** (macOS) or **Podman / Colima** (Linux) run
the image just as well ([`docs/SETUP.md`](docs/SETUP.md)).

### Run it fully local — recommended

Your own uncensored model, on your box: no cloud, no refusals, nothing leaves the
host. Drive marq from [Pi](https://pi.dev/) over [llama.cpp](https://github.com/ggml-org/llama.cpp):

```bash
# 1. the model — full flags matter: --reasoning-format keeps chain-of-thought
#    out of the reply, --jinja makes tool calls parse (see docs/SETUP.md §2a)
llama-server -hf HauhauCS/Gemma4-12B-QAT-Uncensored-HauhauCS-Balanced:Q4_K_M \
  --host 127.0.0.1 --port 8080 -ngl 99 --ctx-size 65536 --jinja \
  --reasoning-format deepseek \
  -fa on -ctk q8_0 -ctv q8_0 \
  --temp 0.6 --top-p 0.9 --top-k 64 --min-p 0.05 --repeat-penalty 1.1
# 2. marq in a long-lived container bound to your workspace
install -m 0755 pi/marq ~/.local/bin/marq && marq up ~/work
# 3. one-time Pi config — llama.cpp provider + marq's system prompt + skill.
#    Without SYSTEM.md a small model narrates instead of driving marq. → docs/SETUP.md §4
ln -sf "$PWD/pi/SYSTEM.md" ~/.pi/agent/SYSTEM.md
# then run `pi`
```

Then just talk to it: _"Set scope to scanme.nmap.org and run a quick nmap,"_ or
_"Load the nis2-dora skill — what's our breach clock?"_ Full local walkthrough
(Pi provider, skill, tuning) → [`docs/SETUP.md`](docs/SETUP.md#4-run-with-a-local-model-pi--the-marq-skill).

### Or fall back to a frontier model

For the heaviest reasoning (architecture, GRC, IR write-ups), point a frontier MCP
client at marq instead:

```bash
claude mcp add marq -- docker run --rm -i \
  --security-opt no-new-privileges:true --cap-drop ALL \
  --cap-add NET_RAW --cap-add NET_ADMIN --cap-add NET_BIND_SERVICE \
  -v marq-audit:/var/log/marq -v marq-work:/work -e MARQ_OPERATOR=marq marq
```

Any MCP client works (Codex, Claude Desktop) → [`docs/CLIENTS.md`](docs/CLIENTS.md).
(The caps are only what SYN scans need; full config incl. API keys in
[`mcp.json.example`](mcp.json.example).)

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
- **Auditable by design** — every external-tool call funnels through one choke
  point: an append-only, `fsync`'d audit log, a timeout, and an output cap. File
  access is sandboxed to `/work`+`/tmp`, and the image runs non-root (the
  recommended run args also drop all Linux capabilities + `no-new-privileges`).

<details>
<summary><b>Full tool suite</b> — ~80 tools on a Kali base (click to expand)</summary>

| Category                              | Tools                                                                                                                                                                                                                                                                            |
| :------------------------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Recon / network**                   | `nmap` · `masscan` · `naabu` · `dnsx` · `dnsrecon` · `subfinder` · `httpx_probe` · `dns_lookup` · `whois_lookup` · `ssh_audit` · `fping_sweep` · `snmp_walk` · `snmp_check` · `snmp_brute` · `smtp_user_enum` · `smtp_test` · `asnmap` · `cdncheck` · `censys_search`            |
| **OSINT — org/domain**                | `theharvester` · `spiderfoot` · `shodan_host` · `shodan_search` · `gitleaks` · `trufflehog` · `gau_urls` · `exif_metadata`                                                                                                                                                       |
| **OSINT — people**                    | `sherlock` · `maigret_username` · `holehe_email` · `h8mail_breach` · `phoneinfoga`                                                                                                                                                                                               |
| **Web app**                           | `nuclei` · `nikto` · `feroxbuster` · `katana` · `ffuf` · `gobuster_dir` · `arjun` · `whatweb` · `wafw00f` · `cmseek` · `wpscan` · `testssl` · `dalfox` · `sqlmap` · `jwt_tool` · `trivy` · `interactsh` · `paramspider` · `sstimap`                                              |
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
                                ├─ creds     john, hashcat, name-that-hash
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

| Doc                                    | What's in it                                             |
| :------------------------------------- | :------------------------------------------------------- |
| [`docs/SETUP.md`](docs/SETUP.md)       | Per-OS install (macOS & Debian), both run modes          |
| [`docs/CLIENTS.md`](docs/CLIENTS.md)   | Pick a client/model — Claude Code, Codex, Pi + llama.cpp |
| [`docs/USAGE.md`](docs/USAGE.md)       | Every tool, env vars, API keys, reading the audit log    |
| [`docs/SECURITY.md`](docs/SECURITY.md) | Legal/ethical baseline, the guardrail model, hardening   |

## License

**GNU AGPLv3** — see [`LICENSE`](LICENSE). © 2026 marq contributors. Free software;
run a modified version as a network service and you must offer users its source.
For authorized security testing and education, **no warranty** — use responsibly.
