# Security & Safe-Use Guide

This project bundles **active, offensive security tooling** (netexec, impacket,
hydra, sqlmap, hashcat, masscan, …) plus **OSINT / footprinting tooling** that profiles
people and organisations (theHarvester, spiderfoot, sherlock, holehe, h8mail,
phoneinfoga, …), and exposes it all to an LLM through MCP. That is powerful and
inherently dual-use. Read this before you run anything.

## Legal & ethical baseline

- **Only test systems you own or have explicit, written authorization to test.**
  Unauthorized scanning, exploitation or credential attacks are illegal in most
  jurisdictions (e.g. the US CFAA, UK Computer Misuse Act).
- Keep your rules of engagement / scope document handy and record it with the
  `set_engagement` tool so it lands in every audit record.
- Online brute force (hydra), exploitation (impacket, evil-winrm) and aggressive scanning
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
means: nothing is blocked on **scope** grounds (scope is recorded, not enforced),
but **every invocation is audit-logged** as JSON lines, before and after it runs
— and a tool whose start record can't be persisted is refused (fail-closed), so
the log can't be silently skipped. Each record carries:

- a correlation id, UTC timestamps and duration
- the logical tool name and the **full argument vector**
- the operator and engagement identifiers
- a best-effort `target`

The audit log is append-only and `fsync`'d per write, and each record is also
mirrored to **stderr** — the container's log stream, outside the unprivileged
`marq` user's reach and separate from the MCP stdout protocol channel — so a copy
survives even if the on-disk log is truncated. At the exec choke point
(`runner.Run`) marq **fails closed**: if a start record cannot be persisted, the
tool does not run (rather than executing unlogged). Default location:
`/var/log/marq/audit.jsonl` (mount it to the host to persist it — see
`mcp.json.example`). The on-disk log is owned by the same `marq` user that runs
the tools, so for a tamper-resistant trail rely on the stderr mirror / ship it
off-box (below), or make it append-only at the filesystem level.

> If you later want hard controls, the natural place to add them is
> `internal/runner/runner.go::Run` (e.g. a scope allowlist check before the
> subprocess is spawned) — auditing already funnels through that one function,
> so enforcement can too.

## Container hardening applied

- **Non-root**: the server runs as the unprivileged `marq` user.
- **Capabilities dropped**: `cap_drop: ALL`, re-adding only `NET_RAW`,
  `NET_ADMIN`, `NET_BIND_SERVICE` (needed for SYN scans). `nmap`/`masscan`/`naabu`
  get exactly those file capabilities via `setcap`, so no root is required.
- **`no-new-privileges`** is set in the sample run configs.
- **No network port is opened.** Transport is stdio only; the MCP client spawns
  the container and talks over stdin/stdout. There is no listening service to
  attack. With the `pi/marq` shim, the agent reaches tools via `docker exec` into
  a local container — the model runtime is the client's concern (e.g. Pi), not
  marq's, so marq itself opens no outbound model connection.
- **Raw shell is OFF by default (opt-in).** `run_shell` executes an arbitrary
  in-container command — the broadest capability the server grants — so it is
  **not registered** unless you set `MARQ_ALLOW_RAW_SHELL=true`. Enable it when
  you need the model to chain the hundreds of tools that have no dedicated
  wrapper; every invocation is still audit-logged. Left off, the model is bounded
  to the wrapped tool set.
- **File access is sandboxed — for the file tools.** The
  `read_file`/`write_file`/`list_dir` tools resolve real paths (defeating symlink
  and `..` escapes) and refuse anything outside `/work` and `/tmp`, so the model
  can exchange working files without reading or clobbering the rest of the
  container. This bounds **those tools**, not tool _execution_: an exec tool with
  an output flag (e.g. `nmap -oN …`) can still write anywhere the `marq` user can,
  so the sandbox is a file-exchange boundary, not a jail. These ops are
  audit-logged like every tool run.
- Output is truncated to a token budget so a runaway scan can't flood the model.
- A per-command timeout (`MARQ_TIMEOUT`, default 900s) bounds runaway tools.
- **Audit completeness depends on the driver.** Every call through `marq run` /
  `marq serve` is logged at `runner.Run`. The `pi/marq` shim, however, hands the
  agent raw host bash, so a model _could_ `docker exec` a tool directly and skip
  the audit trail — the skill instructs "always use `marq run`," but that's
  guidance, not enforcement. For a hard, unbypassable audit boundary use
  `marq serve` (no shell exposed to the model).

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
