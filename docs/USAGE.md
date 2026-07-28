# Usage

How to build, smoke-test and drive marq — your universal cyber assistant (~80
tools + ~80 skill playbooks across 14 domains). For choosing a client and model
for the job (Claude Code / Codex as the expert, Pi for local work, LM
Studio for testing) see [`CLIENTS.md`](CLIENTS.md); for a guided per-OS install
see [`SETUP.md`](SETUP.md); for the safety model see [`SECURITY.md`](SECURITY.md).

**Contents**

- [1. Build the image](#1-build-the-image)
- [2. Smoke-test the image](#2-smoke-test-the-image)
- [3. Wire it into an MCP client](#3-wire-it-into-an-mcp-client)
- [4. Run with a local model (Pi)](#4-run-with-a-local-model-pi)
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
- `trivy` — the vulnerability database (`trivy image --download-db-only`)
- `rockyou` — decompressed to `/usr/share/wordlists/rockyou.txt`

These steps are best-effort: a network hiccup during build won't fail the
image — the tool just downloads on first run instead. To refresh the data in a
running container later, re-run e.g. `nuclei -update-templates` or
`wpscan --update`. All other tools ship their data bundled and need no setup.

## 2. Smoke-test the image

Start with the binary's own self-report — no target, no capabilities, no client:

```bash
# build revision + tool/skill counts
docker run --rm marq version

# the catalog — add a name (`marq tools nmap`) for that tool's schema
docker run --rm marq tools

# the playbook library, grouped by domain — add a name to print one
docker run --rm marq skills
```

A wrong count here is the fastest way to spot a stale image or an unintended
`MARQ_SKILLS_ONLY=1` (which drops the suite to the ~10 in-process tools).

Then run a one-off tool to confirm the wrapped binaries work:

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

Any MCP client works — **Claude Code** and **Codex** (frontier model as the
expert), **Claude Desktop**, and any other MCP client. Per-client setup and
which-brain-for-which-job guidance is in [`CLIENTS.md`](CLIENTS.md). The
shared shape:

1. Add the `marq` server from [`mcp.json.example`](../mcp.json.example) — drop the
   `mcpServers` block into Claude Code's `.mcp.json` or Claude Desktop's config.
   Codex uses TOML (`codex mcp add marq -- …`, see CLIENTS.md).
2. Set `MARQ_OPERATOR` (the audit anchor). Engagement and scope are set at
   runtime — have the model call `set_engagement` before any active testing
   (advisory/knowledge use needs no scope).
3. Use a tool-capable model. Have it call `server_info` first, record scope with
   `set_engagement`, then `load_skill` the domain it's working in.

## 4. Run with a local model (Pi)

Instead of an external MCP client, drive marq from [Pi](https://pi.dev/) — a
minimal terminal agent that runs a local model on **llama.cpp's
`llama-server`** (any OpenAI-compatible endpoint works) and gives the model
bash. The model invokes `marq run <tool> '<json>'`; the `pi/marq` host shim
forwards each call into a long-lived container over `docker exec`.

```bash
# Install the shim
docker run --rm marq shim > ~/.local/bin/marq && chmod +x ~/.local/bin/marq

# Start ONE long-lived container, bound to your engagement dir
marq up ~/engagements/acme        # docker run -d … sleep infinity

# Sanity checks through the shim
marq tools                        # list every tool
marq run server_info '{}'         # confirm scope

marq down                         # tear down when finished
```

Configure your model runtime **in Pi** (it owns the endpoint — that's no longer
marq's concern), then load [`pi/SKILL.md`](../pi/SKILL.md) into Pi as a skill so
the model knows the calling convention and scope rules. From there the model
calls `server_info` first, works the tools, records findings with
`report_finding`, and writes the report with `render_report` into `/work`.

A long-lived container is **required**: background tools (spiderfoot, responder,
ntlmrelayx) detach inside it and are polled later, so a per-call `docker run`
would kill them.

Shim env vars: `MARQ_CONTAINER` (default `marq`), `MARQ_IMAGE` (default
`marq:latest`), `MARQ_ENV_FILE` (optional env-file for `MARQ_OPERATOR` plus API
keys, passed as `--env-file` — copy `.env.example` to `.env` for a template;
engagement + scope are set at runtime via `set_engagement`).

## Available tools

### Recon / network

| Tool             | Wraps          | Purpose                           |
| ---------------- | -------------- | --------------------------------- |
| `server_info`    | —              | Show authorization banner + scope |
| `nmap`           | nmap           | Port/service scanning             |
| `masscan`        | masscan        | Fast port sweeps                  |
| `naabu`          | naabu          | Fast modern port scan (top-ports) |
| `dns_lookup`     | dig            | DNS records                       |
| `dnsx`           | dnsx           | Bulk DNS resolution / record enum |
| `dnsrecon`       | dnsrecon       | DNS recon, zone transfer, brute   |
| `whois_lookup`   | whois          | Registration data                 |
| `subfinder`      | subfinder      | Passive subdomain enum            |
| `httpx_probe`    | httpx          | Live HTTP probing / tech detect   |
| `ssh_audit`      | ssh-audit      | SSH server algorithm/config audit |
| `fping_sweep`    | fping          | Fast parallel ping sweep          |
| `snmp_walk`      | snmpwalk       | SNMP MIB tree walk                |
| `snmp_check`     | snmpcheck      | SNMP service enumeration          |
| `snmp_brute`     | onesixtyone    | SNMP community-string bruteforce  |
| `smtp_user_enum` | smtp-user-enum | SMTP VRFY/EXPN user enum          |
| `smtp_test`      | swaks          | SMTP relay/injection testing      |
| `asnmap`         | asnmap         | ASN ↔ CIDR ↔ IP mapping           |
| `cdncheck`       | cdncheck       | CDN/cloud/WAF IP detection        |

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
| `gau_urls`      | gau          | Known URLs from Wayback/CommonCrawl/OTX/URLScan (background)        |
| `censys_search` | censys       | Search Censys for exposed assets †                                  |

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
| `jwt_tool`     | jwt_tool    | JWT analysis / attacks           |
| `trivy`        | Trivy       | Vuln/secret/misconfig scanning   |
| `interactsh`   | interactsh  | OOB blind-vuln listener (bg)     |
| `paramspider`  | paramspider | Hidden parameter mining          |
| `sstimap`      | sstimap     | SSTI detection / exploitation    |

### Exploitation / credentials

| Tool            | Wraps          | Purpose                      |
| --------------- | -------------- | ---------------------------- |
| `searchsploit`  | exploitdb      | Local exploit DB search      |
| `donut`         | go-donut       | Payload/shellcode generation |
| `hydra`         | hydra          | Online credential testing    |
| `john`          | john           | Offline hash cracking        |
| `hashcat`       | hashcat        | GPU/CPU hash cracking        |
| `hash_identify` | name-that-hash | Identify hash type (nth)     |
| `run_shell`     | bash           | Arbitrary command (opt-in)   |

### AD / internal network

| Tool                   | Wraps                | Purpose                                   |
| ---------------------- | -------------------- | ----------------------------------------- |
| `impacket_secretsdump` | impacket-secretsdump | Dump NTDS / SAM / LSA secrets             |
| `impacket_kerberoast`  | impacket-kerberoast  | Kerberoasting (TGS-REQ crack offline)     |
| `impacket_asreproast`  | impacket-GetNPUsers  | AS-REP roasting (pre-auth accounts)       |
| `impacket_psexec`      | impacket-psexec      | PsExec-style remote exec (SMB)            |
| `impacket_wmiexec`     | impacket-wmiexec     | WMI-based remote exec                     |
| `impacket_ntlmrelayx`  | impacket-ntlmrelayx  | NTLM relay server                         |
| `netexec`              | netexec (nxc)        | Mass auth / spray / exec across hosts     |
| `certipy_find`         | certipy-ad           | AD CS vulnerability enumeration (ESC1-17) |
| `bloodhound_collect`   | bloodhound-python    | AD attack-path data collection            |
| `evil_winrm`           | evil-winrm           | PowerShell remoting over WinRM            |
| `enum4linux`           | enum4linux-ng        | SMB/RPC/NetBIOS enumeration               |
| `smb_enum`             | smbmap               | Share & permission enumeration            |
| `ldap_search`          | ldapsearch           | LDAP directory queries                    |
| `responder`            | responder            | LLMNR/NBT-NS/mDNS poisoner (background)   |
| `nbtscan`              | nbtscan              | NetBIOS host discovery                    |

### Malware — static analysis

Operate on a sample staged in `/work` (pass a container path; the model never
uploads bytes). All static/offline — safe to run on untrusted samples.

| Tool          | Wraps        | Purpose                                                 |
| ------------- | ------------ | ------------------------------------------------------- |
| `capa`        | capa (FLARE) | Identify capabilities / ATT&CK + MBC behaviors          |
| `yara_scan`   | yara         | Match a sample against YARA rules                       |
| `olevba`      | oletools     | Extract & analyse VBA macros from Office documents      |
| `bin_headers` | rabin2       | Binary headers/imports/strings (format, sections, libs) |

### Working files (`/work`, `/tmp`)

| Tool         | Purpose                                                          |
| ------------ | ---------------------------------------------------------------- |
| `list_dir`   | List a directory in the working area                             |
| `read_file`  | Read back output a tool wrote to disk (cmseek JSON, `nuclei -o`) |
| `write_file` | Stage an input file (a hash for `john`, a target list, …)        |

These are sandboxed to `/work` and `/tmp` — the model cannot read or write
anywhere else. They are what make the file-driven tools usable: the model can
`write_file` a captured hash then `john` it, or `read_file` a result another
tool dropped on disk. Mount `/work` from the host (`-v ./work:/work`, as the
`pi/marq` shim does) to exchange files with the operator.

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
| `CENSYS_API_ID` / `CENSYS_API_SECRET` | `censys_search`                |
| `GITHUB_TOKEN`                        | `trufflehog` (GitHub scans)    |
| `NUMVERIFY_API_KEY`                   | `phoneinfoga`                  |
| `GOOGLE_API_KEY` / `GOOGLECSE_CX`     | `phoneinfoga` (Google CSE)     |
| `WPSCAN_API_TOKEN`                    | `wpscan` (vuln data)           |

theHarvester and h8mail read their own config files (`~/.theHarvester/api-keys.yaml`,
an h8mail config passed via `options`) for Hunter, SecurityTrails, HIBP, etc.

## Environment variables

| Variable               | Default                     | Meaning                                                                                                           |
| ---------------------- | --------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `MARQ_OPERATOR`        | `marq`                      | Recorded in every audit record — the accountability anchor; env-set only                                          |
| `MARQ_ENGAGEMENT`      | `unspecified`               | Engagement / SOW identifier (initial default; overridable by `set_engagement`)                                    |
| `MARQ_SCOPE`           | `""`                        | Free-text authorized scope (initial default; overridable by `set_engagement`)                                     |
| `MARQ_AUDIT_LOG`       | `/var/log/marq/audit.jsonl` | Audit log path                                                                                                    |
| `MARQ_TIMEOUT`         | `900`                       | Default per-command timeout (seconds)                                                                             |
| `MARQ_MAX_TIMEOUT`     | `3600`                      | Ceiling for a tool's per-call timeout override                                                                    |
| `MARQ_MAX_OUTPUT`      | `60000`                     | Max output chars returned to the model                                                                            |
| `MARQ_ALLOW_RAW_SHELL` | `false`                     | Expose the arbitrary-shell `run_shell` tool (opt-in; set `true` to enable)                                        |
| `MARQ_SKILLS_ONLY`     | `false`                     | Knowledge-only mode: serve just the skills + findings/files tools, drop every Kali exec tool (runs with no image) |
| `MARQ_WORK_DIR`        | `/work`                     | Working area (findings, job dirs)                                                                                 |

Engagement and scope are per-task, so the model sets them at session start with
the **`set_engagement`** tool (from the operator's written authorization). It
persists them to `$MARQ_WORK_DIR/.marq-context`, so they survive across the
process-per-call `marq run` path and show up in `server_info` and every audit
record. Operator stays env-set — it's the accountability anchor, not something
the model should assert.

## Reading the audit log

```bash
# Persisted to ./audit/ when you bind-mount it (-v ./audit:/var/log/marq).
cat audit/audit.jsonl | jq 'select(.event=="invocation.start") | {ts, tool, target, argv}'
```

## GPU hash cracking (optional)

`hashcat` benefits from a GPU. Pass it through at run time, e.g. with the
NVIDIA container toolkit: add `--gpus all` to the `docker run` args.
