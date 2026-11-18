# Security & Safe-Use Guide

marq bundles passive OSINT and footprinting tooling that profiles organisations
and people (theHarvester, maigret, user-scanner, …) and exposes it to an LLM through MCP. The tools query public third-party
sources and never scan, crawl or exploit a target's hosts. The only traffic that can reach a target's own infrastructure is ordinary DNS resolution hitting its authoritative nameservers. `dnstwist` resolves lookalike domains through the normal resolver; those are not the target's infrastructure. Username and email tools send lookup requests to third-party sites (`email_accounts` uses silent registration/login checks). People OSINT is still personal-data processing, so read this before you run anything.

## Legal & ethical baseline

- People OSINT is personal-data processing. Profiling individuals
  (usernames, emails, breach data, phone numbers) implicates privacy law (e.g.
  GDPR): you need a lawful basis and a stated purpose. Record the purpose with
  `set_engagement` (`engagement` = label + purpose, e.g.
  `acme-footprint-2026-11 — pre-acquisition exposure review`) so it lands in every
  audit record. Collect only what that purpose needs, and prefer organisation-level
  sources over individual profiling.
- marq never tests credentials. `trufflehog` reports found secrets unverified;
  do not try them against the service they belong to.
- Several tools call third-party services (Shodan, GitHub, crt.sh, the RDAP registries, GreyNoise, AlienVault OTX, urlscan.io,
  CIRCL, ORKL). Your queries leave the box and are logged by those providers, so
  do not submit data you are not permitted to disclose to a third party.

## Guardrail model: passive tools + fail-closed audit

There is no active-testing capability to gate. The tool set is passive by
construction: trufflehog accepts only
public git hosts (GitHub, GitLab, Bitbucket, Codeberg) and never verifies secrets, and DNS tools refuse zone
transfers. There is no generic command runner or raw shell.

Every OSINT-tool, file and findings call is audit-logged as JSON lines, before and after
it runs. An exec or file tool whose start record can't be persisted is
refused (fail-closed), so the log can't be silently skipped. In-process tools
(`server_info`, `load_skill`, `list_jobs`, `job_status`) are not logged;
`set_engagement` and the findings tools log but don't fail closed. Each record carries:

- a correlation id, UTC timestamps and duration
- the logical tool name and the full argument vector
- the operator and engagement identifiers
- a best-effort `target`

The audit log is append-only and `fsync`'d per write, and each record is also
mirrored to stderr, the container's log stream. That stream is outside the unprivileged
`marq` user's reach and separate from the MCP stdout protocol channel, so a copy
survives even if the on-disk log is truncated. At the exec choke point
(`runner.Run`) marq fails closed: if a start record cannot be persisted, the
tool does not run, rather than executing unlogged. Default location:
`/var/log/marq/audit.jsonl` (mount it to the host to persist it, see
`mcp.json.example`). The on-disk log is owned by the same `marq` user that runs
the tools, so for a tamper-resistant trail rely on the stderr mirror, ship it
off-box (below), or make it append-only at the filesystem level.

## Skills-only mode has a smaller surface

The container hardening below is for the full Docker tool suite. The
skills-only build (`MARQ_SKILLS_ONLY=1`, run as a bare `go install` binary)
registers none of the exec tools. It runs no external binaries at all, only
in-process handlers over embedded markdown and the `/work` file tools. The
external-tool threat model therefore doesn't apply, and it needs no container to be safe. File, findings and engagement calls
are still audit-logged, and the file sandbox still bounds `read_file`/`write_file`
to `/work`, `/tmp` and `MARQ_WORK_DIR` (ignored if it is `/` or contains `$HOME`). It does not get the container's process/network isolation natively, so
run it where you'd run any small local service.

## Container hardening applied

This applies to the full Docker tool suite (skills-only above needs none of it).

- Non-root: the server runs as the unprivileged `marq` user.
- Capabilities dropped: run with `cap_drop: ALL` and `no-new-privileges` (as
  in the sample MCP run configs). No added capabilities are needed, since nothing in the
  image opens raw sockets. The `pi/marq` shim's `up` sets neither.
- No network port is opened. Transport is stdio only; the MCP client spawns
  the container and talks over stdin/stdout. There is no listening service to
  attack. With the `pi/marq` shim, the agent reaches tools via `docker exec` into
  a local container. The model runtime is the client's concern (e.g. Pi), not
  marq's, so marq itself opens no outbound model connection.
- No shell or generic executor. The model is bounded to the wrapped tool set.
- File access is sandboxed for the file tools and `exif_metadata`. The
  `read_file`/`write_file`/`list_dir` tools and `exif_metadata` resolve real paths (defeating symlink
  and `..` escapes) and refuse anything outside `/work`, `/tmp` and `MARQ_WORK_DIR` (the audit log itself is refused), so the model
  can exchange working files without reading or clobbering the rest of the
  container. The other tools aren't path-sandboxed; they are bounded by fixed or
  allowlisted argv and the container (no wrapper exposes an output-file option). These ops are
  audit-logged.
- Output is truncated to a token budget so a runaway query can't flood the model.
- A per-command timeout (`MARQ_TIMEOUT`, default 900s) bounds runaway tools.
- Audit completeness depends on the driver. Every OSINT-tool call through `marq run` /
  `marq serve` is logged at `runner.Run`. The `pi/marq` shim, however, hands the
  agent raw host bash, so a model _could_ `docker exec` a tool directly and skip
  the audit trail. The skill instructs "always use `marq run`," but that's
  guidance, not enforcement. For a hard, unbypassable audit boundary use
  `marq serve` (no shell exposed to the model).

## Prompt injection in tool output

Everything marq's tools return was written by someone else: WHOIS remarks, DNS TXT
records, certificate SANs, social profiles, threat-intel pulses, scan titles. Any
of it can carry instructions aimed at the model (indirect prompt injection). marq
applies two deterministic layers to all OSINT-tool output, `read_file`, `list_dir`
and `job_status` (`internal/untrusted`):

- It strips characters that hide text from the person reading the transcript:
  Unicode tag characters (ASCII smuggling), variation-selector smuggling, bidi
  overrides, zero-width characters, ANSI/OSC terminal escapes and other control
  codes. When it removes any, the output starts with a `[marq removed …]` note.
- It fences the text between `[untrusted-data <id> begin …]` and
  `[untrusted-data <id> end]` markers with a random id per call, so a payload
  can't forge the end of the block. The MCP instructions and the Pi prompts tell
  the model to treat fenced content as data and to report instructions it finds.

The OSINT tools also carry the standard MCP `readOnlyHint` and `openWorldHint`
annotations, so clients can tell they read untrusted external content.

Content the model fetches with the client's own web or shell tools never passes
through marq and isn't fenced, so the client's tool approval and the Rule of Two
matter most there.

These layers reduce the risk; they don't remove it. Published measurements show
delimiting lowers attack success by a few points, and adaptive attacks get past
every published prompt-level defense. The controls that hold sit in the client:

- **Rule of Two** (Meta, 2025): in one session, combine at most two of untrusted
  input, access to sensitive data or systems, and the ability to change state or
  communicate externally. marq's tools are the untrusted input, so don't give the
  same session unattended shell, email or write access to production.
- Keep tool approval on for anything outside marq, and review the model's next
  call after it reads third-party content.
- The full research behind this design is in
  [`reports/MCP tool output injection defenses.md`](../reports/MCP%20tool%20output%20injection%20defenses.md).

Not adopted: bundling a classifier such as Llama Prompt Guard 2 (independent
tests show low recall on tool-output injection, the licence and Hugging Face
gating complicate redistribution, and Go has no clean DeBERTa tokenizer) and
cloud scanners (data would leave the host). If you want a classifier anyway, put
it in front of marq in your own gateway, for example a Docker MCP Gateway `after`
interceptor (experimental).

## Recommended operational practices

- Run the container where you'd run any service that makes outbound requests;
  it needs outbound internet only (no inbound).
- Treat the audit log as evidence: ship it off-box and protect it from tampering.
- Don't bake credentials or API tokens into the image. Pass them
  at runtime via env or mounted files (they are git-ignored).
- Review what the model collects. An LLM driving people-OSINT tools can
  over-collect, and the human operator is the real guardrail on purpose and data
  minimisation.

## Reporting issues

For a vulnerability in marq itself, report it privately as described in
[`SECURITY.md`](../SECURITY.md); do not open a public issue. Other bugs can go in a normal
issue. Do not file vulnerabilities discovered in third-party targets here.
