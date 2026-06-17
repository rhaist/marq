# Security & Safe-Use Guide

This project bundles **active, offensive security tooling** (metasploit, hydra,
sqlmap, hashcat, masscan, …) plus **OSINT / footprinting tooling** that profiles
people and organisations (theHarvester, spiderfoot, sherlock, holehe, h8mail,
phoneinfoga, …), and exposes it all to an LLM through MCP. That is powerful and
inherently dual-use. Read this before you run anything.

## Legal & ethical baseline

- **Only test systems you own or have explicit, written authorization to test.**
  Unauthorized scanning, exploitation or credential attacks are illegal in most
  jurisdictions (e.g. the US CFAA, UK Computer Misuse Act).
- Keep your rules of engagement / scope document handy and record it in
  `PENTEST_MCP_SCOPE` so it lands in every audit record.
- Online brute force (hydra), exploitation (metasploit) and aggressive scanning
  (masscan at high rates) can disrupt or lock out production systems. Use the
  least aggressive technique that answers the question.
- **OSINT on people is still in-scope work, not a free-for-all.** Profiling
  individuals (usernames, emails, breach data, phone numbers) implicates privacy
  law (e.g. GDPR) and your engagement's rules. Only footprint people and
  organisations covered by your authorization, and prefer passive sources.
- Several OSINT tools call **third-party APIs** (Shodan, Censys, GitHub, HIBP,
  numverify). Your queries leave the box and are logged by those providers — do
  not submit data you are not permitted to disclose to a third party.

## Guardrail model: logging-only

This build uses **logging-only** guardrails (an explicit operator choice). That
means: nothing is blocked, but **every invocation is audit-logged** as JSON
lines, before and after it runs, with:

- a correlation id, UTC timestamps and duration
- the logical tool name and the **full argument vector**
- the operator and engagement identifiers
- a best-effort `target`

The audit log is append-only and `fsync`'d per write. Default location:
`/var/log/pentest-mcp/audit.jsonl` (mount it to the host to persist it — see
`mcp.json.example` and `docker-compose.yml`).

> If you later want hard controls, the natural place to add them is
> `internal/runner/runner.go::Run` (e.g. a scope allowlist check before the
> subprocess is spawned) — auditing already funnels through that one function,
> so enforcement can too.

## Container hardening applied

- **Non-root**: the server runs as the unprivileged `pentester` user.
- **Capabilities dropped**: `cap_drop: ALL`, re-adding only `NET_RAW`,
  `NET_ADMIN`, `NET_BIND_SERVICE` (needed for SYN scans). `nmap`/`masscan`/`naabu`
  get exactly those file capabilities via `setcap`, so no root is required.
- **`no-new-privileges`** is set in the sample run configs.
- **No network port is opened.** Transport is stdio only; the MCP client spawns
  the container and talks over stdin/stdout. There is no listening service to
  attack. (The local TUI/agent mode makes only an outbound call to the model
  runtime you configure.)
- **Raw shell is enabled by default but audit-logged.** The image is a full
  offensive toolkit (hundreds of tools without dedicated wrappers), so the
  arbitrary-command tool ships on so the model can chain and stage them; every
  invocation is logged like any other. It is still the broadest capability the
  server grants — set `PENTEST_MCP_ALLOW_RAW_SHELL=false` to remove it for a
  more locked-down deployment.
- **File access is sandboxed.** The `read_file`/`write_file`/`list_dir` tools
  resolve real paths and refuse anything outside `/work` and `/tmp`, so the
  model can exchange working files without reading or clobbering the rest of the
  container. These ops are audit-logged like every tool run.
- Output is truncated to a token budget so a runaway scan can't flood the model.
- A per-command timeout (`PENTEST_MCP_TIMEOUT`, default 900s) bounds runaway tools.

## Recommended operational practices

- Run the container on an **isolated lab network / VLAN**, not a network with
  bystander hosts. Consider `--network none` for offline tasks (hash cracking).
- Treat the audit log as evidence: ship it off-box and protect it from tampering.
- Don't bake credentials, API tokens or scope files into the image — pass them
  at runtime via env or mounted files (they are git-ignored).
- Review the model's proposed commands. An LLM driving offensive tools can
  misjudge scope; the human operator is the real guardrail here.

## Reporting issues

Found a problem with the tooling itself (not a target system)? Open an issue.
Do not file vulnerabilities discovered in third-party targets here.
