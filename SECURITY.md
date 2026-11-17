# Security Policy

This policy covers vulnerabilities **in marq itself**: the Go binary, the MCP
server, the tool wrappers, the file sandbox, and the audit model.
For marq's operational threat model and safe-use posture (how to operate it
safely), see [`docs/SECURITY.md`](docs/SECURITY.md).

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

- **Audit bypass:** a tool path that reaches `os/exec` without funnelling
  through `internal/runner` (so the call isn't logged).
- **File-sandbox escape:** reading or writing outside `/work`, `/tmp` and `MARQ_WORK_DIR`
  (or reading the audit log itself) through the `files` tools (e.g. a symlink or path-traversal escape).
- **Command/argument injection** in a tool wrapper that lets crafted input run
  unintended commands.
- **Passive-only bypass:** a wrapper that can be made to actively probe,
  scan or authenticate to a target (argument injection past its fixed flags).
- **Container hardening regressions** in the shipped run configuration
  (capabilities, `no-new-privileges`, the unprivileged `marq` user).

## Out of scope

- **The OSINT tools doing what OSINT tools do.** Querying public sources about
  organisations and people is their intended functionality, not a vulnerability.
- **Using marq for unlawful collection of personal data.** That is misuse, not a
  flaw in marq. See [`docs/SECURITY.md`](docs/SECURITY.md).
- Vulnerabilities in the third-party tools themselves. Report those
  upstream.

## Supported versions

marq is developed on `main`; fixes land there. Please reproduce against the
latest `main` before reporting.
