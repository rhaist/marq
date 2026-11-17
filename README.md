<div align="center">

<img src="docs/logo.svg" alt="marq" width="320">

### Current security expertise for your AI agent

An open knowledge pack of security, GRC and OT/ICS playbooks, re-checked
against primary sources every 6 to 8 weeks, plus an optional passive-OSINT tool
server. It works with any model and agent.

[![CI](https://github.com/rhaist/marq/actions/workflows/ci.yml/badge.svg)](https://github.com/rhaist/marq/actions/workflows/ci.yml)

</div>

```text
"Is this CVE in our firmware 'actively exploited' under the CRA?"  → CRA triage
"Map these pentest findings to IEC 62443, CRA and NIS2."          → crosswalk
"Our breach-notification clock under NIS2, GDPR and US law?"      → regulation
"Threat-model this payment API."                                  → architecture
"What does this malware sample actually do?"                      → malware
"Footprint acme.com from public sources."                       → OSINT
```

**The knowledge pack.** About 83 playbooks for practitioners at EU and US
manufacturers. They take you from a finding to the obligation it touches (IEC 62443,
EU CRA, NIS2, ISO 27001, NIST, US sector rules) and on to the board report. Every skill
records the date it was last checked. The pack is Apache-2.0, so it installs into any
agent, including internal ones.

**The tool server (optional).** 23 passive OSINT tools (33 tools in all: domain,
infrastructure, people, leaked secrets in public code). They query public third-party sources (DNS resolvers, WHOIS, certificate logs, Shodan/Censys, web archives, breach databases, social and code-hosting sites) and never scan, crawl or exploit a target's hosts. The only traffic that can reach a target's own infrastructure is ordinary DNS resolution hitting its authoritative nameservers. Username and email tools send lookup requests to third-party sites (`email_accounts` uses silent registration/login checks). Every OSINT-tool, file and findings call lands in an operator-attributed audit log you can hand to a client or regulator, and an OSINT tool won't run if its start record can't be written (fail-closed).

## Start

Pick a path. Start simple and add a layer only when you need it:

| I want…                                      | Path            | Setup                       |
| :------------------------------------------- | :-------------- | :-------------------------- |
| **cyber expertise inside my agent**          | skills pack     | one install command         |
| **…the same, over MCP for any client**       | skills server   | one `go install`, no Docker |
| **…plus the 23-tool OSINT suite to _run_**   | full MCP server | build/pull the image        |
| **…driven by a local model**                 | full + Pi       | image + Pi + llama.cpp      |

### The skills pack: install into your agent (no server, no Docker)

The ~83 playbooks are a standard [Agent Skills](https://agentskills.io) pack under
[`skills/`](skills/), so agents that read `SKILL.md` load them natively:

```bash
# Claude Code (v2.1.292+; on older versions: claude plugin marketplace add
# rhaist/marq && claude plugin install marq@marq)
claude plugin install marq --marketplace rhaist/marq

# Codex, Cursor, Copilot, Gemini CLI, Pi and ~40 other agents
npx skills add rhaist/marq
```

Each skill carries `metadata.verified`, the date of its last review sweep. The
pack is re-verified against primary sources every 6 to 8 weeks; `marq skills --stale`
lists anything overdue.

### Just the skills over MCP, for any MCP client (no Docker)

The lowest-friction way in: add the skills library and advisory tools to any MCP
client and skip the OSINT tool suite entirely. `MARQ_SKILLS_ONLY=1` loads only
the in-process knowledge tools (skills, findings, files) and no OSINT
binaries, so it is a single static Go binary you run natively:

```bash
go install github.com/rhaist/marq/cmd/marq@latest   # ~10 MB, skills embedded
marq skills                                          # browse all 83, by domain
marq skills nis2-dora                                # read one, no client needed

claude mcp add marq-skills \
  -e MARQ_SKILLS_ONLY=1 -e MARQ_WORK_DIR=$HOME/.marq \
  -e MARQ_AUDIT_LOG=$HOME/.marq/audit.jsonl \
  -- marq serve
```

Now ask _"Map our audit findings to ISO 27001 and SOC 2"_ or _"What's our
breach-notification clock under GDPR and NIS2?"_ and the model loads the right
playbook (current 2026 standards) and drafts the deliverable. Scripted
end-to-end demo: [`scripts/demo-skills.sh`](scripts/demo-skills.sh).

Prefer containers, or already pulled the image? The same mode runs there too. Use
the `marq-skills` entry in [`mcp.json.example`](mcp.json.example). Add the full tool
suite below when you need to _run_ something, not just reason about it.

### The full tool suite

To _run_ the OSINT tools (domain, people, leaked-secret lookups), not just reason, get the image:

```bash
# prebuilt — or build from source: docker build -t marq .
docker pull ghcr.io/rhaist/marq && docker tag ghcr.io/rhaist/marq marq

# first check: which build, and how many tools/skills did it come with
docker run --rm marq version

# the catalog — add a name (`marq tools subfinder`) for one tool's schema
docker run --rm marq tools
```

You do not need Docker Desktop. OrbStack (macOS) or Podman / Colima (Linux) run
it just as well ([`docs/SETUP.md`](docs/SETUP.md)). Then drive it fully local (below)
or point a frontier MCP client at it.

### Run it fully local

Your own model, on your box: no cloud model, and your data stays local (OSINT
lookups still query third-party sources). This is the recommended setup when investigating people or a client's footprint. Drive marq from
[Pi](https://pi.dev/) over [llama.cpp](https://github.com/ggml-org/llama.cpp):

```bash
# 1. the model — full flags matter: --reasoning-format keeps chain-of-thought
#    out of the reply, --jinja makes tool calls parse (see docs/SETUP.md §2a)
llama-server -hf google/gemma-4-12B-it-qat-q4_0-gguf \
  --host 127.0.0.1 --port 8080 -ngl 99 --ctx-size 65536 --jinja \
  --reasoning-format deepseek \
  -fa on -ctk q8_0 -ctv q8_0 \
  --temp 1.0 --top-p 0.95 --top-k 64

# 2. marq in a long-lived container bound to your workspace. The shim ships
#    inside the image, so it always matches the marq it drives — no clone.
mkdir -p ~/.local/bin && docker run --rm marq shim > ~/.local/bin/marq
chmod +x ~/.local/bin/marq

marq up ~/work

# 3. one-time Pi config — marq's system prompt. Without it a small model
#    narrates instead of driving marq. → docs/SETUP.md §4
mkdir -p ~/.pi/agent && marq prompt system > ~/.pi/agent/SYSTEM.md

# then run `pi`
```

Then talk to it: _"Footprint example.com: subdomains, emails, exposed hosts,"_ or
_"Load the nis2-dora skill: what's our breach clock?"_ The full local walkthrough
(Pi provider, skill, tuning) is in [`docs/SETUP.md`](docs/SETUP.md#4-run-with-a-local-model-pi--the-marq-skill).

### Or fall back to a frontier model

For the heaviest reasoning (architecture, GRC, IR write-ups), point a frontier MCP
client at marq instead:

```bash
claude mcp add marq -- docker run --rm -i \
  --security-opt no-new-privileges:true --cap-drop ALL \
  -v marq-audit:/var/log/marq -v marq-work:/work -e MARQ_OPERATOR="$USER" marq
```

Any MCP client works (Codex, Claude Desktop); see [`docs/CLIENTS.md`](docs/CLIENTS.md).
No keys or config are required; optional
API keys are listed in [`docs/USAGE.md`](docs/USAGE.md#api-keys-for-osint-sources).

> **Passive and purpose-bound.** marq queries public sources only and audit-logs
> every OSINT-tool, file and findings call. People OSINT is personal-data processing, so record a purpose with
> `set_engagement`. See [`docs/SECURITY.md`](docs/SECURITY.md).

---

## What you get

- **Execution:** 23 wrapped passive-OSINT tools (33 tools in all: domain/infra footprint, people,
  leaked secrets in public code), audit-logged. File tools and `exif_metadata` are sandboxed to `/work` and `/tmp`. Findings render
  to `findings.md` / `.csv` (with optional CVSS and CWE).
- **Knowledge:** ~83 `load_skill` playbooks across 13 domains, web-researched to
  current (2026) standards. They tell the model how to use the tools and what to
  do where there is no tool: malware, threat-intel, sec-ops, architecture,
  GRC, standards and EU/US regulation, CISO, purple-team, resilience, human factors,
  DevSecOps/privacy. Run them standalone with `MARQ_SKILLS_ONLY=1`, with no tool
  suite and no OSINT binaries.
- **Audit:** every external-tool call funnels through one choke
  point: an append-only, `fsync`'d audit log, a timeout, and an output cap. File
  access is sandboxed to `/work`+`/tmp`, and the image runs non-root (the
  recommended run args also drop all Linux capabilities and set `no-new-privileges`).

<details>
<summary><b>Full tool suite</b>: 23 passive OSINT tools (click to expand)</summary>

| Category                              | Tools                                                                                                           |
| :------------------------------------ | :-------------------------------------------------------------------------------------------------------------- |
| **Domain / infrastructure**           | `theharvester` · `spiderfoot` · `subfinder` · `dnsx` · `dns_lookup` · `whois_lookup` · `asnmap` · `cdncheck` · `gau_urls` |
| **Certificates / registry / lookalikes** | `ct_search` · `rdap` · `dnstwist` (background) |
| **Threat-intel lookups**              | `ioc_lookup`                                                                                                    |
| **Exposure lookups**                  | `shodan_host` · `shodan_search` · `censys_view`                                                                 |
| **Leaked secrets / metadata**         | `trufflehog` · `exif_metadata`                                                                                  |
| **OSINT, people**                     | `sherlock` · `maigret_username` · `email_accounts` · `h8mail_breach` · `phoneinfoga`                            |
| **Files** (sandboxed `/work`, `/tmp`) | `list_dir` · `read_file` · `write_file`                                                                         |
| **Findings & jobs**                   | `report_finding` · `render_report` · `list_jobs` · `job_status`                                                 |
| **Session & knowledge**               | `server_info` · `set_engagement` · `load_skill`                                                                 |

</details>

<details>
<summary><b>How it works</b>: one registry, two front-ends (click to expand)</summary>

```
MCP client  ──stdio JSON-RPC──▶  marq serve  ┐
terminal agent (Pi) ──bash──▶  pi/marq shim ─┤   both hit the same registry
                              (docker exec)  │
                                             ▼
                              registry.All() ── one Tool list
                                ├─ intel     ct_search, rdap, dnstwist, ioc_lookup
                                ├─ osint     theHarvester, spiderfoot, subfinder, dnsx, shodan, trufflehog, gau…
                                ├─ people    sherlock, maigret, user-scanner, h8mail, phoneinfoga
                                ├─ files     read/write/list (sandboxed /work)
                                └─ extras    report_finding, render_report, list_jobs, load_skill
                                             │
                                  runner.Run ──▶ audit.jsonl (OSINT/file/findings calls)

           load_skill ──▶ skills library (markdown, ~83 playbooks / 13 domains)
```

Every tool funnels through `internal/runner/runner.go::Run`, the single
point for audit logging, timeouts, and output truncation.

</details>

---

## Documentation

| Doc                                    | What's in it                                                      |
| :------------------------------------- | :---------------------------------------------------------------- |
| [`docs/SETUP.md`](docs/SETUP.md)       | Per-OS install: skills-only (no Docker), full suite, local model |
| [`docs/CLIENTS.md`](docs/CLIENTS.md)   | Pick a client/model: Claude Code, Codex, Pi + llama.cpp           |
| [`docs/USAGE.md`](docs/USAGE.md)       | Every tool, env vars, API keys, reading the audit log             |
| [`docs/SECURITY.md`](docs/SECURITY.md) | Legal/ethical baseline, the guardrail model, hardening            |

## License

GNU AGPLv3, see [`LICENSE`](LICENSE). © 2026 marq contributors. This is free software:
if you run a modified version as a network service, you must offer users its source.
The skills pack under [`skills/`](skills/) is Apache-2.0 ([`skills/LICENSE`](skills/LICENSE)),
so you can install and adapt the playbooks in any agent, including internal ones.
marq is for lawful OSINT, security research and education. There is no warranty, so use it responsibly.
