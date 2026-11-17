# Usage

How to build, smoke-test and drive marq, your cyber assistant: 23 passive-OSINT tools (33 tools in all)
and ~83 skill playbooks across 13 domains. To choose a client and model
for the job (Claude Code / Codex as the expert, Pi for local work, LM
Studio for testing), see [`CLIENTS.md`](CLIENTS.md); for a guided per-OS install,
see [`SETUP.md`](SETUP.md); for the safety model, see [`SECURITY.md`](SECURITY.md).

**Contents**

- [1. Build the image](#1-build-the-image)
- [2. Smoke-test the image](#2-smoke-test-the-image)
- [3. Wire it into an MCP client](#3-wire-it-into-an-mcp-client)
- [4. Run with a local model (Pi)](#4-run-with-a-local-model-pi)
- [Available tools](#available-tools)
- [API keys for OSINT sources](#api-keys-for-osint-sources)
- [Environment variables](#environment-variables)
- [Reading the audit log](#reading-the-audit-log)

## 1. Build the image

```bash
docker build -t marq .
```

This pulls the Kali base and installs the full tool suite, so it is large
(multi-GB) and the first build takes a while.

## 2. Smoke-test the image

Start with the binary's own self-report, which needs no target and no client:

```bash
# build revision + tool/skill counts
docker run --rm marq version

# the catalog — add a name (`marq tools subfinder`) for that tool's schema
docker run --rm marq tools

# the playbook library, grouped by domain — add a name to print one
docker run --rm marq skills
```

A wrong count here is the fastest way to spot a stale image or an unintended
`MARQ_SKILLS_ONLY=1` (which drops the suite to the ~10 in-process tools).

Then run a one-off tool to confirm the wrapped binaries work:

```bash
docker run --rm marq subfinder -version
```

Get an interactive shell:

```bash
docker run --rm -it --entrypoint /bin/bash marq
```

Confirm the MCP server starts (it will wait for JSON-RPC on stdin; Ctrl-C to exit):

```bash
docker run --rm -i marq        # prints the usage notice to stderr
```

## 3. Wire it into an MCP client

Any MCP client works: Claude Code and Codex (frontier model as the
expert), Claude Desktop, and any other MCP client. Per-client setup and
guidance on which model to use for which job is in [`CLIENTS.md`](CLIENTS.md). The
shared shape:

1. Add the `marq` server from [`mcp.json.example`](../mcp.json.example), and drop the
   `mcpServers` block into Claude Code's `.mcp.json` or Claude Desktop's config.
   Codex uses TOML (`codex mcp add marq -- …`, see CLIENTS.md).
2. Set `MARQ_OPERATOR` (the audit anchor). The engagement and its purpose are set at
   runtime: have the model call `set_engagement` before any people OSINT
   (GDPR: record why you are collecting).
3. Use a tool-capable model. Have it call `server_info` first, record the purpose with
   `set_engagement`, then `load_skill` the domain it's working in.

## 4. Run with a local model (Pi)

Instead of an external MCP client, drive marq from [Pi](https://pi.dev/), a
minimal terminal agent that runs a local model on llama.cpp's
`llama-server` (any OpenAI-compatible endpoint works) and gives the model
bash. The model invokes `marq run <tool> '<json>'`; the `pi/marq` host shim
forwards each call into a long-lived container over `docker exec`.

```bash
# Install the shim
docker run --rm marq shim > ~/.local/bin/marq && chmod +x ~/.local/bin/marq

# Start ONE long-lived container, bound to your working dir
marq up ~/investigations/acme        # docker run -d … sleep infinity

# Sanity checks through the shim
marq tools                        # list every tool
marq run server_info '{}'         # confirm purpose

marq down                         # tear down when finished
```

Configure your model runtime in Pi (it owns the endpoint, which is no longer
marq's concern), then load [`pi/SKILL.md`](../pi/SKILL.md) into Pi as a skill so
the model knows the calling convention. From there the model
calls `server_info` first, works the tools, records findings with
`report_finding`, and writes the report with `render_report` into `/work`.

A long-lived container is required: background tools (`spiderfoot`, `gau_urls`, `dnstwist`)
detach inside it and are polled later, so a per-call `docker run`
would kill them.

The shim reads these env vars: `MARQ_CONTAINER` (default `marq`), `MARQ_IMAGE` (default
`marq:latest`), `MARQ_ENGINE` (container CLI, default `docker`; or `podman`),
`MARQ_ENV_FILE` (default `~/.config/marq/env`, loaded if present). Nothing is
required: `MARQ_OPERATOR` defaults to your `$USER`, and any API key exported in
your shell is forwarded (see _API keys_ below); the engagement + purpose is set at
runtime via `set_engagement`.

## Available tools

All wrapped tools are passive: they query public third-party sources (DNS resolvers, WHOIS, certificate logs, Shodan/Censys, web archives, breach databases, social and code-hosting sites) and never scan, crawl or exploit a target's hosts. The only traffic that can reach a target's own infrastructure is ordinary DNS resolution hitting its authoritative nameservers. Username and email tools send lookup requests to third-party sites (`email_accounts` uses silent registration/login checks).

### Session

| Tool             | Purpose                                                      |
| ---------------- | ------------------------------------------------------------ |
| `server_info`    | Operator, recorded engagement/purpose, what marq covers      |
| `set_engagement` | Record the investigation label + purpose (GDPR: say why)     |

### Information gathering: company and domain footprint

| Tool            | Wraps        | Purpose                                                             |
| --------------- | ------------ | ------------------------------------------------------------------- |
| `theharvester`  | theHarvester | Emails, employees, hosts, subdomains                                |
| `spiderfoot`    | spiderfoot   | Broad automated OSINT footprint, passive use case (background)     |
| `subfinder`     | subfinder    | Passive subdomain enum                                              |
| `dns_lookup`    | dig          | DNS records (no zone transfers)                                     |
| `dnsx`          | dnsx         | Bulk DNS resolution / record enum                                   |
| `whois_lookup`  | whois        | Registration data                                                   |
| `asnmap`        | whois + curl | IP/domain → ASN, ASN → CIDRs                                        |
| `cdncheck`      | cdncheck     | CDN/cloud/WAF IP detection                                          |
| `ct_search`     | curl (crt.sh) | Certificates issued for a domain and subdomains (CT logs)          |
| `rdap`          | go-rdap      | Registration data over RDAP for a domain, IP or ASN                 |
| `dnstwist`      | dnstwist     | Registered lookalike domains, DNS-only (background)                 |
| `ioc_lookup`    | curl         | Hash, IP or domain enrichment: CIRCL hashlookup, GreyNoise community, OTX, urlscan.io |
| `exif_metadata` | exiftool     | Metadata from a staged file/dir                                     |
| `shodan_host`   | shodan       | Exposed ports/services/CVEs for an IP (keyless via InternetDB)      |
| `shodan_search` | shodan       | Search exposed assets (e.g. `org:`) †                               |
| `trufflehog`    | trufflehog   | Leaked secrets in public git hosts (GitHub, GitLab, Bitbucket, Codeberg; unverified) ‡           |
| `gau_urls`      | gau          | Known URLs from Wayback/CommonCrawl/OTX/URLScan (background)        |
| `censys_view`   | cencli       | Censys host/web/cert lookup (search is paid-only) †                 |

### Information gathering: people footprint

| Tool               | Wraps        | Purpose                             |
| ------------------ | ------------ | ----------------------------------- |
| `sherlock`         | sherlock     | Username across ~400 sites          |
| `maigret_username` | maigret      | Deep username sweep (~2500 sites)   |
| `email_accounts`   | user-scanner | Sites where an email has an account |
| `h8mail_breach`    | h8mail       | Email breach/leak exposure †        |
| `phoneinfoga`      | phoneinfoga  | Phone number OSINT                  |

### Working files (`/work`, `/tmp`)

| Tool         | Purpose                                                          |
| ------------ | ---------------------------------------------------------------- |
| `list_dir`   | List a directory in the working area                             |
| `read_file`  | Read back output a tool wrote to disk                             |
| `write_file` | Stage an input file (a host list for `dnsx`, …)                 |

These, and `exif_metadata`, are sandboxed to `/work`, `/tmp` and `MARQ_WORK_DIR` (realpath-checked; the audit log itself is refused). Other tools aren't path-sandboxed but are bounded by fixed/allowlisted argv and the container. Together they make the file-driven tools usable: the model can
`write_file` a host list then `dnsx` it, or `read_file` a result another
tool dropped on disk. Mount `/work` from the host (`-v ./work:/work`, as the
`pi/marq` shim does) to exchange files with the operator.

### Findings, jobs & knowledge

| Tool             | Purpose                                                                                                                                           |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `report_finding` | Record a validated issue (title, severity, target, evidence, recommendation; optional CVSS 3.1 vector, CWE, references) to `/work/findings.jsonl` |
| `render_report`  | Write the severity-sorted `findings.md` + `findings.csv` deliverable                                                                              |
| `list_jobs`      | List background jobs and whether each is running or done                                                                                          |
| `job_status`     | A background job's state (running/done + exit code) + output tail                                                                                 |
| `load_skill`     | Load a playbook (recon-footprint, ioc-pivoting, nis2-dora, …) tied to these tool names                                         |

A `report_finding` with a `cvss` vector (e.g. `CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H`)
gets its base score computed. Your explicit `severity` stays authoritative; the
vector only sets severity when you omit one, and a divergent vector is flagged in
the report rather than overriding your call. The skills library is also exposed as
MCP resources: `marq://skills` (index) and `marq://skills/<name>`.

Tools too slow for a synchronous call (`spiderfoot`,
`gau_urls`, `dnstwist`) launch in the background and return a job directory under
`/work/jobs/<tool>-<id>/`. Results stream to `stdout.log`, diagnostics to
`stderr.log`, and a `status` file (containing `exit=<code>`) appears when the
scan finishes. Check progress with `list_jobs` / `job_status` (or `read_file` /
`list_dir`). The work survives individual tool-call timeouts.

† Needs an API key (see _API keys_ below). ‡ GitHub org/repo scans need `GITHUB_TOKEN` (rate limits).

## API keys for OSINT sources

All optional: every tool runs without them (keyless sources or reduced detail),
so skip this on first run. To add one, export it in your shell (the `pi/marq`
shim forwards it, and the `-e VAR` args in `mcp.json.example` pass it to MCP
clients that inherit your environment) or put `KEY=value` lines in
`~/.config/marq/env` (shim) / an `"env": {...}` block (Claude Desktop).

| Variable                               | Used by                                                              |
| -------------------------------------- | -------------------------------------------------------------------- |
| `SHODAN_API_KEY`                       | `shodan_search`; full banners in `shodan_host` (keyless: InternetDB) |
| `CENSYS_API_TOKEN` (+ `CENSYS_ORG_ID`) | `censys_view`                                                        |
| `GITHUB_TOKEN`                         | `trufflehog` (GitHub scans)                                          |
| `NUMVERIFY_API_KEY`                    | `phoneinfoga`                                                        |

theHarvester and h8mail read their own config files (`~/.theHarvester/api-keys.yaml`,
an h8mail config passed via `options`) for Hunter, SecurityTrails, HIBP, etc.

## Environment variables

| Variable               | Default                     | Meaning                                                                                                           |
| ---------------------- | --------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `MARQ_OPERATOR`        | `marq` (bare binary), `unknown` (image); `pi/marq` shim: host `$USER` | Recorded in every audit record; the accountability anchor, env-set only                |
| `MARQ_ENGAGEMENT`      | `unspecified`               | Investigation label + purpose (initial default; overridable by `set_engagement`)                                  |
| `MARQ_AUDIT_LOG`       | `/var/log/marq/audit.jsonl` | Audit log path                                                                                                    |
| `MARQ_TIMEOUT`         | `900`                       | Default per-command timeout (seconds)                                                                             |
| `MARQ_MAX_TIMEOUT`     | `3600`                      | Ceiling for a tool's per-call timeout override                                                                    |
| `MARQ_MAX_OUTPUT`      | `60000`                     | Max output chars returned to the model                                                                            |
| `MARQ_SKILLS_ONLY`     | `false`                     | Knowledge-only mode: serve just the skills + findings/files tools, drop every OSINT exec tool (runs with no image) |
| `MARQ_WORK_DIR`        | `/work`                     | Working area (findings, job dirs)                                                                                 |

The engagement is per-task, so the model sets it at session start with the
`set_engagement` tool: a label plus the purpose, e.g.
`acme-footprint-2026-11 — pre-acquisition exposure review`. People OSINT is
GDPR personal-data processing, so always state why you are collecting. It persists
to `$MARQ_WORK_DIR/.marq-context`, so it survives across the process-per-call
`marq run` path and shows up in `server_info` and every audit record. Operator
stays env-set because it is the accountability anchor, and the model should not
assert it.

## Reading the audit log

```bash
# Persisted to ./audit/ when you bind-mount it (-v ./audit:/var/log/marq).
cat audit/audit.jsonl | jq 'select(.event=="invocation.start") | {ts, tool, target, argv}'
```
