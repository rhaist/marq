# Driving marq: pick the model for the job

marq is tools plus knowledge: passive-OSINT tools and skill playbooks
across many domains (OSINT, malware, threat-intel, sec-ops, architecture, GRC,
standards, CISO, resilience, human factors, …). It does not bring a model.
Your client does, so you choose which model to point at it, and that
depends on the work.

## Match the brain to the work

| You want to…                                                                                                             | Use                                                                        | Why                                                                                                         |
| :----------------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------------------------- | :---------------------------------------------------------------------------------------------------------- |
| Run fully local: hands-on OSINT, on-box execution, the model and your data stay local _(recommended default)_ | Pi + a local model on llama.cpp + the `pi/marq` shim               | Terminal-native, no cloud model (OSINT lookups still query third-party sources); the model gets bash and calls `marq run` directly.                               |
| Reason hard: architecture review, threat modeling, GRC/compliance, IR leadership, write the report, app/code-sec review | Claude Code or Codex (frontier model) + marq over MCP _(fallback)_ | Strongest reasoning. marq hands it the current-standards skills and the tools, and the model supplies judgment. |

Rule of thumb: use a local model by default, so hands-on OSINT and
privacy-sensitive work stays on your box. Fall back to a frontier model
when a task is judgment-heavy (governance/architecture/advisory) and
the data isn't sensitive. Keep both configured and switch per task.

## How marq runs & connects

There are two builds. The skills-only build serves just the knowledge library +
findings/files tools, a ~10 MB native binary with no Docker (install:
[`SETUP.md`](SETUP.md#skills-only-no-docker)). Reach for it when the work is
advisory: architecture, GRC, standards, threat-intel research. The full build
is the Docker image that adds the OSINT tool suite for when you need to _run_ things.

There are two transports (both builds use the same `marq serve` stdio server):

- MCP server (`marq serve`): for Claude Code, Codex, Claude Desktop, and
  any MCP client. The client's model sees marq's tools as typed MCP tools and
  calls them directly. The same `mcp.json.example` works for all of the
  JSON-config clients.
- Direct run (`pi/marq` shim): for Pi (or any agent with a shell). The
  model calls `marq run <tool> '<json>'` from bash; the shim forwards it into a
  long-lived container.

Every combination uses the same registry and the same audit log, so behavior is
identical.

---

## Claude Code (frontier expert)

The quickest path is to register the server (`--scope user` makes it available in every
project; drop it for project-local). It uses the hardened docker args from
[`../mcp.json.example`](../mcp.json.example):

```bash
claude mcp add marq --scope user -- docker run --rm -i \
  --security-opt no-new-privileges:true --cap-drop ALL \
  -v marq-audit:/var/log/marq -v marq-work:/work -e MARQ_OPERATOR="$USER" marq

claude mcp list                    # confirm "marq … ✔ Connected"
```

The named volumes (`marq-audit`, `marq-work`) persist the audit log and
findings/reports across sessions. MCP servers load at session start, so
restart Claude Code before marq's tools/skills appear. Or commit a project
[`.mcp.json`](https://docs.anthropic.com/en/docs/claude-code/mcp) with the
`mcpServers` block from `mcp.json.example` (same schema). Then ask it to call
`server_info` first and `load_skill` the domain you're in; for people OSINT,
record the purpose with `set_engagement`. Remove with `claude mcp remove marq
-s user`.

Best for: "review this architecture against zero-trust", "map our findings
to ISO 27001 and SOC 2", "build me an incident runbook", "threat-model this
service", "turn `/work/findings.md` into a board report". The model does the
thinking, and marq supplies current frameworks and the evidence tools.

## Codex CLI (frontier expert)

```bash
codex mcp add marq -- docker run --rm -i \
  --security-opt no-new-privileges:true --cap-drop ALL \
  -v marq-audit:/var/log/marq -v marq-work:/work -e MARQ_OPERATOR="$USER" marq
```

Or add it to `~/.codex/config.toml` (TOML, not JSON):

```toml
[mcp_servers.marq]
command = "docker"
args = ["run", "--rm", "-i", "--cap-drop", "ALL",
        "-v", "marq-audit:/var/log/marq", "-v", "marq-work:/work",
        "-e", "MARQ_OPERATOR=you", "marq"]
```

Same uses as Claude Code. Docs: [Codex MCP](https://developers.openai.com/codex/mcp).

## Pi (local, standalone)

The fully-local path has no cloud model and runs on the box (OSINT lookups still query
third-party sources). Setup (shim, `llama-server`, Pi provider, skill, system prompt) is
in [`SETUP.md`](SETUP.md#4-run-with-a-local-model-pi--the-marq-skill).

Best for: any cyber work you want fully local (malware triage, threat-intel pivoting,
OSINT footprinting and research), where the model and your data stay on the box.

---

## Using it effectively (any client)

- Always start with `server_info`, then `load_skill` the domain playbook
  before diving in. The skills carry the current-standards detail and the right
  tool order. They make a smaller model competent and keep a frontier
  model from guessing at stale facts.
- Load only the skill you need. There are many, and pulling the one relevant
  playbook keeps context tight. `marq run load_skill '{}'` (or the `load_skill`
  tool) lists the index.
- Record a purpose with `set_engagement` before people OSINT (why:
  [`SECURITY.md`](SECURITY.md#legal--ethical-baseline)). GRC/architecture/standards
  conversations are unrestricted.
- Pick the model to the job. Use a big reasoning model for analysis and writing and
  a fast tool-capable local model for chaining lookups and hands-on OSINT
  you want to keep on-box.
- If a weak local model bypasses marq (raw `curl`/`grep`, ignoring the tools),
  install `pi/SYSTEM.md` ([`SETUP.md`](SETUP.md#4-run-with-a-local-model-pi--the-marq-skill), step 2b).
  The cause is the model's prompt, not your `llama-server` flags. Then
  lean on the skills and keep it to one tool call at a time.
- Slow lookups run in the background. Use `list_jobs` / `job_status` instead of
  blocking; this works the same in every client.
