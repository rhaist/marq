# Contributing to marq

Thanks for your interest in improving marq. It is a Kali-based cyber assistant:
wrapped passive-OSINT tools plus an embedded skills library, exposed from a single Go
binary over MCP (`marq serve`) and direct invocation (`marq run`). This guide
covers how to build it, the quality gates, and how to add a tool or a skill.

> **Passive and attributable is the contract.** marq's tools query public sources
> only and never scan, crawl or exploit a target's hosts (only ordinary DNS resolution can reach its nameservers). Its safety model is _attribution_: every OSINT-tool, file and findings call is audit-logged, and an OSINT tool funnels through one audited choke point
> (`runner.Run`) that fails closed if it can't log. Any contribution must preserve
> that. See [Design invariants](#design-invariants-and-adding-a-tool-or-skill)
> below and [`docs/SECURITY.md`](docs/SECURITY.md). Contributions that add
> active scanning, exploitation, evasion, anti-forensics, or that bypass the audit
> log will not be merged.

## Getting started

```bash
# Local Go iteration needs no Docker build for the server/tool-wrapper layer
# (most tool *calls* only fully work in-container, since they shell out to Kali
# binaries). Run the fast gate below first.
go run ./cmd/marq serve                  # stdio MCP server
go run ./cmd/marq run <tool> '<json>'    # invoke one tool

# Full image (multi-GB: Go builder stage + Kali tool suite):
docker build -t marq .
```

## Quality gates

Run these before opening a PR. CI enforces the first three on every push/PR, and the image e2e when image paths change:

| Gate              | Command                                                           | What it checks                                                                                                                                                        |
| ----------------- | ----------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Format**        | `gofmt -l . && prettier --check . && ruff format --check scripts` | Go, Markdown/JSON/YAML and Python formatting (CI pins prettier 3.10.1, ruff 0.17.5).                                                                                  |
| **Fast gate**     | `go build ./... && go vet ./... && go test ./...`                 | Compiles, vets, and the registry unit test: no duplicate tool names, every tool has exactly one of `Build`/`Handler`, generated input schemas are well-formed.        |
| **End-to-end**    | `scripts/e2e.sh`                                                  | User-facing promises through the real `marq run` CLI (engagement persistence, skills load, findings + CVSS, audit, file sandbox, catalog). No image; runs in seconds. |
| **Image e2e**     | `scripts/e2e-image.sh`                                            | Promises that only hold for the running image (`/work` persistence across containers). Needs a built image.                                                           |
| **Tool plumbing** | `scripts/test_tools.py`                                           | Drives the built image over MCP for wrapped-binary coverage.                                                                                                          |

`go test ./...` is the sanity gate; keep it green.

## Design invariants and adding a tool or skill

The design invariants (the `runner.Run` choke point, no interactive tools, no file
upload, background jobs, passive-only wrappers and `TestPassiveOnly`), the steps to
add a tool, and the skill frontmatter rules are in [`CLAUDE.md`](CLAUDE.md). Rules
that are only here:

- **API keys degrade quietly.** Tools needing keys must still run (with reduced
  capability) when the key env var is absent.
- If a tool isn't a Kali apt package, install it in the Go-built block of the `Dockerfile`.
- Set `Invocation.Target` to the lookup target so the call is audited.
- A new skill category also needs adding to `skills` in
  `skills/.claude-plugin/plugin.json`. The periodic review sweep is the
  `/refresh-skills` skill (`.claude/skills/refresh-skills/`). Spec:
  [Agent Skills](https://agentskills.io/specification).

**Regional review wanted.** The regional skills (`apac-cyber-regs`,
`middle-east-regs`, `latam-regs`, `privacy-apac`) were researched mostly from English
sources, and the official-language text is what counts. If you work with one of these
jurisdictions, a review of its section is the most useful contribution you can make:
confirm or correct a claim against the official text and cite it. Where a claim carries
a tag, change it from U (unconfirmed) or S (summary) to V (official text read).
Corrections from local practitioners outrank our sources.

## Commits and PRs

- This repo uses **Conventional Commits** (`feat:`, `fix:`, `docs:`,
  `feat(eval):`, …). Keep the subject imperative and scoped.
- Keep PRs focused. Describe what changed and how you verified it (which gate(s)
  you ran).
- New or changed tool behavior should come with a doc update and, where it fits,
  an assertion in `scripts/e2e.sh`.

## Reporting security issues

See [`SECURITY.md`](SECURITY.md); do not open a public issue.

## License

By contributing, you agree that your contributions are licensed under the
project's [AGPL-3.0](LICENSE).
