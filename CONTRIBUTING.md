# Contributing to marq

Thanks for your interest in improving marq. It's a Kali-based cyber assistant —
~80 wrapped tools plus an embedded skills library — exposed from a single Go
binary over MCP (`marq serve`) and direct invocation (`marq run`). This guide
covers how to build it, the quality gates, and how to add a tool or a skill.

> **Authorized use is the contract.** marq's safety model is _attribution, not
> prevention_: every tool call funnels through one audited choke point. Any
> contribution must preserve that — see [Design invariants](#design-invariants)
> below and [`docs/SECURITY.md`](docs/SECURITY.md). Contributions that add
> evasion, anti-forensics, or that bypass the audit log will not be merged.

## Getting started

```bash
# Local Go iteration — no Docker build needed for the server/tool-wrapper layer.
# (Most tool *calls* only fully work in-container, since they shell out to Kali
# binaries, but build/vet/test and the MCP plumbing all run locally.)
go build ./... && go vet ./... && go test ./...
go run ./cmd/marq serve                  # stdio MCP server
go run ./cmd/marq run <tool> '<json>'    # invoke one tool

# Full image (multi-GB: Go builder stage + Kali tool suite):
docker build -t marq .
```

## Quality gates

Run these before opening a PR — CI enforces the first two on every push/PR:

| Gate              | Command                                             | What it checks                                                                                                                                                   |
| ----------------- | --------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Fast gate**     | `go build ./... && go vet ./... && go test ./...`   | Compiles, vets, and the registry unit test: no duplicate tool names, every tool has exactly one of `Build`/`Handler`, generated input schemas are well-formed.   |
| **End-to-end**    | `scripts/e2e.sh`                                    | User-facing promises through the real `marq run` CLI (scope persistence, skills load, findings + CVSS, audit, file sandbox, catalog). No image; runs in seconds. |
| **Image e2e**     | `scripts/e2e-image.sh`                              | Promises that only hold for the running image (`/work` persistence, per-container jwt keys). Needs a built image.                                                |
| **Tool plumbing** | `scripts/test_tools.py` / `scripts/verify_tools.sh` | Drives the built image over MCP for wrapped-binary coverage.                                                                                                     |

`go test ./...` is the sanity gate; keep it green.

## Design invariants

These are load-bearing — breaking them silently defeats marq's safety model:

- **Never call `os/exec` directly.** Every exec tool funnels through
  `internal/runner/runner.go::Run` (or `RunBackground`). That is the single choke
  point for audit logging, timeout, and output truncation.
- **No interactive tools.** The transport is synchronous, one-shot, stateless,
  no TTY. Always pass non-interactive flags (`--batch`, `-x "...; exit"`, etc.) —
  anything that prompts will hang until timeout.
- **No file upload.** The model passes strings only. File inputs/outputs go
  through the `/work` mount and the `files` tools, which are sandboxed to
  `/work` and `/tmp`.
- **Long-running tools go to the background** (`Invocation.Background = true`),
  polled via `list_jobs` / `job_status`.
- **API keys degrade quietly.** Tools needing keys must still run (with reduced
  capability) when the key env var is absent.

## Adding a tool

1. Prefer a **Kali apt package** (verify on [pkg.kali.org](https://pkg.kali.org)).
   If it isn't packaged, install it in the Go-built block of the `Dockerfile`.
2. If it caches/downloads on first use, add a warm-up line in the post-`USER`
   block of the `Dockerfile` (so the cache lands in `/home/marq`, not `/root`).
3. Add the `Tool` to the relevant `internal/registry/*.go` (recon, web, osint,
   exploit, creds, internal, files, …). `Tool.Desc` is the model's only guidance
   — make it informative (usage, defaults, API-key needs, caveats).
4. Set `Invocation.Target` to the network/file target so it's audited (and
   eligible for any scope enforcement at the runner).
5. Document it in [`docs/USAGE.md`](docs/USAGE.md).

## Adding a skill

Drop a markdown file with `name` / `description` frontmatter into
`internal/skills/library/<category>/`. It's embedded at build time and exposed
via the `load_skill` tool and the `marq://skills/<name>` MCP resource.

## Commits and PRs

- This repo uses **Conventional Commits** (`feat:`, `fix:`, `docs:`,
  `feat(eval):`, …). Keep the subject imperative and scoped.
- Keep PRs focused. Describe what changed and how you verified it (which gate(s)
  you ran).
- New or changed tool behavior should come with a doc update and, where it fits,
  an assertion in `scripts/e2e.sh`.

## Reporting security issues

Do **not** open a public issue for a vulnerability in marq itself. See
[`SECURITY.md`](SECURITY.md).

## License

By contributing, you agree that your contributions are licensed under the
project's [AGPL-3.0](LICENSE).
