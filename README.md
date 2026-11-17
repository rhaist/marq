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
"Does China's 4-hour rule or Saudi NCA apply to our plants?"      → regional law
"Threat-model this payment API."                                  → architecture
"What does this malware sample actually do?"                      → malware
"Footprint acme.com from public sources."                       → OSINT
```

**The knowledge pack.** About 86 playbooks for practitioners at EU and US
manufacturers with sites worldwide. They take you from a finding to the obligation it
touches (IEC 62443, EU CRA, NIS2, ISO 27001, NIST, US sector rules, and the national
cyber and privacy laws of APAC, the Middle East and Latin America) and on to the board
report. Every skill records the date it was last checked. The pack is Apache-2.0, so it
installs into any agent, including internal ones.

**The tool server (optional).** 24 passive OSINT tools (34 tools in all: domain,
infrastructure, people, leaked secrets in public code). They query public third-party sources (DNS resolvers, WHOIS, certificate logs, Shodan/Censys, web archives, breach databases, social and code-hosting sites) and never scan, crawl or exploit a target's hosts. The only traffic that can reach a target's own infrastructure is ordinary DNS resolution hitting its authoritative nameservers. Username and email tools send lookup requests to third-party sites (`email_accounts` uses silent registration/login checks). Every OSINT-tool, file and findings call lands in an operator-attributed audit log you can hand to a client or regulator, and an OSINT tool won't run if its start record can't be written (fail-closed).

## Start

Pick a path. Start simple and add a layer only when you need it:

| I want…                                    | Path            | Setup                       |
| :----------------------------------------- | :-------------- | :-------------------------- |
| **cyber expertise inside my agent**        | skills pack     | one install command         |
| **…the same, over MCP for any client**     | skills server   | one `go install`, no Docker |
| **…plus the 24-tool OSINT suite to _run_** | full MCP server | build/pull the image        |
| **…driven by a local model**               | full + Pi       | image + Pi + llama.cpp      |

### The skills pack: install into your agent (no server, no Docker)

The ~86 playbooks are a standard [Agent Skills](https://agentskills.io) pack under
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
marq skills                                          # browse all 86, by domain
marq skills nis2-dora                                # read one, no client needed

claude mcp add marq-skills -e MARQ_SKILLS_ONLY=1 -e MARQ_WORK_DIR=$HOME/.marq -- marq serve
```

Other clients and the audit-log variable: [`docs/SETUP.md`](docs/SETUP.md#skills-only-no-docker).

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
lookups still query third-party sources). This is the recommended setup when
investigating people or a client's footprint. Drive marq from
[Pi](https://pi.dev/) over [llama.cpp](https://github.com/ggml-org/llama.cpp):
the shim comes out of the image (`docker run --rm marq shim`), then `marq up
~/work` starts the container. The `llama-server` flags, Pi provider, skill and
system prompt are in [`docs/SETUP.md`](docs/SETUP.md#4-run-with-a-local-model-pi--the-marq-skill).
Then ask _"Footprint example.com: subdomains, emails, exposed hosts."_

### Or fall back to a frontier model

For the heaviest reasoning (architecture, GRC, IR write-ups), point a frontier MCP
client at marq instead. The hardened `docker run` command for Claude Code, Codex and Claude Desktop is in
[`docs/CLIENTS.md`](docs/CLIENTS.md) and [`mcp.json.example`](mcp.json.example).
No keys or config are required; optional
API keys are listed in [`docs/USAGE.md`](docs/USAGE.md#api-keys-for-osint-sources).

> **Passive and purpose-bound.** marq queries public sources only and audit-logs
> every OSINT-tool, file and findings call. People OSINT is personal-data processing, so record a purpose with
> `set_engagement`. See [`docs/SECURITY.md`](docs/SECURITY.md).

---

## What you get

- **Execution:** 24 wrapped passive-OSINT tools (34 tools in all: domain/infra footprint, people,
  leaked secrets in public code), audit-logged. File tools and `exif_metadata` are sandboxed to `/work` and `/tmp`. Findings render
  to `findings.md` / `.csv` (with optional CVSS and CWE).
- **Knowledge:** ~86 `load_skill` playbooks across 13 domains, web-researched to
  current (2026) standards. They tell the model how to use the tools and what to
  do where there is no tool: malware, threat-intel, sec-ops, architecture,
  GRC, standards and regulation (EU, US, APAC, Middle East, LATAM), CISO,
  purple-team, resilience, human factors, DevSecOps/privacy. Run them standalone with `MARQ_SKILLS_ONLY=1`, with no tool
  suite and no OSINT binaries.
- **Audit:** every external-tool call funnels through one choke
  point: an append-only, `fsync`'d audit log, a timeout, and an output cap. File
  access is sandboxed to `/work`+`/tmp`, and the image runs non-root (the
  recommended run args also drop all Linux capabilities and set `no-new-privileges`).

The full tool catalog, with what each tool wraps and which need API keys, is in
[`docs/USAGE.md`](docs/USAGE.md#available-tools).

<details>
<summary><b>How it works</b>: one registry, two front-ends (click to expand)</summary>

```
MCP client  ──stdio JSON-RPC──▶  marq serve  ┐
terminal agent (Pi) ──bash──▶  pi/marq shim ─┤   both hit the same registry
                              (docker exec)  │
                                             ▼
                              registry.All() ── one Tool list
                                ├─ recon     dns_lookup, whois_lookup, subfinder, dnsx (DNS/registry lookups)
                                ├─ intel     ct_search, rdap, dnstwist, ioc_lookup, orkl
                                ├─ osint     theHarvester, spiderfoot, shodan, trufflehog, gau…
                                ├─ people    sherlock, maigret, user-scanner, h8mail, phoneinfoga
                                ├─ files     read/write/list (sandboxed /work)
                                └─ extras    report_finding, render_report, list_jobs, load_skill
                                             │
                                  runner.Run ──▶ audit.jsonl (OSINT/file/findings calls)

           load_skill ──▶ skills library (markdown, ~86 playbooks / 13 domains)
```

Every tool funnels through `internal/runner/runner.go::Run`, the single
point for audit logging, timeouts, and output truncation.

</details>

---

## Documentation

| Doc                                    | What's in it                                                     |
| :------------------------------------- | :--------------------------------------------------------------- |
| [`docs/SETUP.md`](docs/SETUP.md)       | Per-OS install: skills-only (no Docker), full suite, local model |
| [`docs/CLIENTS.md`](docs/CLIENTS.md)   | Pick a client/model: Claude Code, Codex, Pi + llama.cpp          |
| [`docs/USAGE.md`](docs/USAGE.md)       | Every tool, env vars, API keys, reading the audit log            |
| [`docs/SECURITY.md`](docs/SECURITY.md) | Legal/ethical baseline, the guardrail model, hardening           |

## License

GNU AGPLv3, see [`LICENSE`](LICENSE). © 2026 marq contributors. This is free software:
if you run a modified version as a network service, you must offer users its source.
The skills pack under [`skills/`](skills/) is Apache-2.0 ([`skills/LICENSE`](skills/LICENSE)),
so you can install and adapt the playbooks in any agent, including internal ones.
marq is for lawful OSINT, security research and education. There is no warranty, so use it responsibly.
