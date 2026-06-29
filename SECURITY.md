# Security Policy

This policy covers vulnerabilities **in marq itself** — the Go binary, the MCP
server, the tool wrappers, the file sandbox, and the audit/authorization model.
For marq's operational threat model and authorized-use posture (how the tool is
meant to be operated safely), see [`docs/SECURITY.md`](docs/SECURITY.md).

## Reporting a vulnerability

**Please do not open a public issue for security vulnerabilities.**

Report privately via GitHub's **private vulnerability reporting**:

1. Go to the repository's **Security** tab.
2. Click **Report a vulnerability**.
3. Provide a description, affected version/commit, reproduction steps, and impact.

We aim to acknowledge a report within a few days and to keep you updated as we
investigate. Please give us reasonable time to release a fix before any public
disclosure.

## In scope

Vulnerabilities in marq's own code and trust boundaries, for example:

- **Audit bypass** — a tool path that reaches `os/exec` without funnelling
  through `internal/runner` (so the call isn't logged).
- **File-sandbox escape** — reading or writing outside `/work` and `/tmp`
  through the `files` tools (e.g. a symlink or path-traversal escape).
- **Command/argument injection** in a tool wrapper that lets crafted input run
  unintended commands.
- **Scope / authorization bypass** in the engagement or scope handling.
- **Container hardening regressions** in the shipped run configuration
  (capabilities, `no-new-privileges`, the unprivileged `marq` user).

## Out of scope

- **The offensive tools doing what offensive tools do.** marq bundles scanners,
  exploitation frameworks, and credential tools. Their intended functionality is
  not a vulnerability.
- **Using marq against systems you are not authorized to test.** That is misuse,
  not a flaw in marq. Active testing is authorized-only and audit-logged by
  design — see [`docs/SECURITY.md`](docs/SECURITY.md).
- Vulnerabilities in the third-party Kali tools themselves — report those
  upstream.

## Supported versions

marq is developed on `main`; fixes land there. Please reproduce against the
latest `main` before reporting.
