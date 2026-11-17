# Security & Safe-Use Guide

marq bundles **passive OSINT / footprinting tooling** that profiles organisations
and people (theHarvester, spiderfoot, sherlock, user-scanner, h8mail, phoneinfoga,
…) and exposes it to an LLM through MCP. The tools query public third-party
sources and never scan, crawl or exploit a target's hosts (the only traffic that can reach a target's own infrastructure is ordinary DNS resolution hitting its authoritative nameservers). Username and email tools send lookup requests to third-party sites (`email_accounts` uses silent registration/login checks). Even so,
people OSINT is personal-data processing. Read this before you run anything.

## Legal & ethical baseline

- **People OSINT is personal-data processing.** Profiling individuals
  (usernames, emails, breach data, phone numbers) implicates privacy law (e.g.
  GDPR): you need a lawful basis and a stated purpose. Record the purpose with
  `set_engagement` (`engagement` = label + purpose, e.g.
  `acme-footprint-2026-11 — pre-acquisition exposure review`) so it lands in every
  audit record, collect only what that purpose needs, and prefer organisation-level
  sources over individual profiling.
- marq never tests credentials. `trufflehog` reports found secrets unverified;
  do not try them against the service they belong to.
- Several tools call **third-party APIs** (Shodan, Censys, GitHub, HIBP,
  numverify). Your queries leave the box and are logged by those providers — do
  not submit data you are not permitted to disclose to a third party.

## Guardrail model: passive tools + fail-closed audit

There is no active-testing capability to gate: the tool set is passive by
construction (spiderfoot runs only its `passive` use case, trufflehog accepts only
public git hosts (GitHub, GitLab, Bitbucket, Codeberg) and never verifies secrets, DNS tools refuse zone
transfers), and there is no generic command runner or raw shell.

**Every OSINT-tool, file and findings call is audit-logged** as JSON lines, before and after
it runs — and an exec or file tool whose start record can't be persisted is
refused (fail-closed), so the log can't be silently skipped. In-process tools
(`server_info`, `load_skill`, `list_jobs`, `job_status`) are not logged;
`set_engagement` and the findings tools log but don't fail closed. Each record carries:

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

## Skills-only mode has a smaller surface

The container hardening below is for the **full** Docker tool suite. The
**skills-only** build (`MARQ_SKILLS_ONLY=1`, run as a bare `go install` binary)
registers none of the exec tools — it runs **no external binaries at all**, only
in-process handlers over embedded markdown and the `/work` file tools. So the
external-tool threat model simply doesn't apply, and it needs no container to be safe. File, findings and engagement calls
are still audit-logged, and the file sandbox still bounds `read_file`/`write_file`
to `/work`, `/tmp` and `MARQ_WORK_DIR` (ignored if it is `/` or contains `$HOME`). The
one thing it doesn't get natively is the container's process/network isolation —
run it where you'd run any small local service.

## Container hardening applied

Applies to the full Docker tool suite (skills-only above needs none of it):

- **Non-root**: the server runs as the unprivileged `marq` user.
- **Capabilities dropped**: run with `cap_drop: ALL` and `no-new-privileges` (as
  in the sample MCP run configs). No added capabilities are needed — nothing in the
  image opens raw sockets. The `pi/marq` shim's `up` sets neither.
- **No network port is opened.** Transport is stdio only; the MCP client spawns
  the container and talks over stdin/stdout. There is no listening service to
  attack. With the `pi/marq` shim, the agent reaches tools via `docker exec` into
  a local container — the model runtime is the client's concern (e.g. Pi), not
  marq's, so marq itself opens no outbound model connection.
- **No shell or generic executor.** The model is bounded to the wrapped tool set.
- **File access is sandboxed — for the file tools and `exif_metadata`.** The
  `read_file`/`write_file`/`list_dir` tools and `exif_metadata` resolve real paths (defeating symlink
  and `..` escapes) and refuse anything outside `/work`, `/tmp` and `MARQ_WORK_DIR` (the audit log itself is refused), so the model
  can exchange working files without reading or clobbering the rest of the
  container. The other tools aren't path-sandboxed; they are bounded by fixed or
  allowlisted argv and the container (no wrapper exposes an output-file option; h8mail `-o`/`-j` are refused). These ops are
  audit-logged.
- Output is truncated to a token budget so a runaway query can't flood the model.
- A per-command timeout (`MARQ_TIMEOUT`, default 900s) bounds runaway tools.
- **Audit completeness depends on the driver.** Every OSINT-tool call through `marq run` /
  `marq serve` is logged at `runner.Run`. The `pi/marq` shim, however, hands the
  agent raw host bash, so a model _could_ `docker exec` a tool directly and skip
  the audit trail — the skill instructs "always use `marq run`," but that's
  guidance, not enforcement. For a hard, unbypassable audit boundary use
  `marq serve` (no shell exposed to the model).

## Recommended operational practices

- Run the container where you'd run any service that makes outbound requests;
  it needs outbound internet only (no inbound).
- Treat the audit log as evidence: ship it off-box and protect it from tampering.
- Don't bake credentials or API tokens into the image — pass them
  at runtime via env or mounted files (they are git-ignored).
- Review what the model collects. An LLM driving people-OSINT tools can
  over-collect; the human operator is the real guardrail on purpose and data
  minimisation.

## Reporting issues

Found a problem with the tooling itself (not a target system)? Open an issue.
Do not file vulnerabilities discovered in third-party targets here.
