# Driving marq — pick the brain for the job

marq is **tools + knowledge**: ~80 wrapped Kali tools and ~70 skill playbooks
across 14 domains (pentest, malware, threat-intel, sec-ops, architecture, GRC,
standards, CISO, resilience, human factors, …). It does **not** bring a model.
Your client does. So the question is which **brain** you point at it — and that
depends on the work.

## Match the brain to the work

| You want to…                                                                                                             | Use                                                           | Why                                                                                                         |
| :----------------------------------------------------------------------------------------------------------------------- | :------------------------------------------------------------ | :---------------------------------------------------------------------------------------------------------- |
| Reason hard — architecture review, threat modeling, GRC/compliance, IR leadership, write the report, app/code-sec review | **Claude Code** or **Codex** (frontier model) + marq over MCP | Strongest reasoning. marq hands it the current-standards skills and the tools; the model supplies judgment. |
| Run fully local & uncensored — hands-on offensive ops, on-box execution, nothing leaves the host, no model refusals      | **Pi** + a local/abliterated model + the `pi/marq` shim       | Terminal-native, no cloud, no refusals; the model gets bash and calls `marq run` directly.                  |
| Test marq, try a model, or chat in a GUI                                                                                 | **LM Studio** + marq over MCP                                 | Easiest local MCP client; load any tool-capable GGUF and go.                                                |

Rule of thumb: **frontier model for judgment** (the governance/architecture/
advisory domains), **local uncensored model for hands-on offensive and
privacy-sensitive work**. You can keep both configured and switch per task.

## Two ways marq connects

- **MCP server** (`marq serve`) — for Claude Code, Codex, LM Studio, Claude
  Desktop, and any MCP client. The client's model sees marq's tools as typed MCP
  tools and calls them directly. The same `mcp.json.example` works for all of
  the JSON-config clients.
- **Direct run** (`pi/marq` shim) — for Pi (or any agent with a shell). The
  model calls `marq run <tool> '<json>'` from bash; the shim forwards it into a
  long-lived container.

Either way it's the **same registry and the same audit log** — behavior is
identical.

---

## Claude Code (frontier expert)

Quickest path — register the server (use the hardened docker args from
[`../mcp.json.example`](../mcp.json.example)):

```bash
claude mcp add marq -- docker run --rm -i \
  --security-opt no-new-privileges:true --cap-drop ALL \
  --cap-add NET_RAW --cap-add NET_ADMIN --cap-add NET_BIND_SERVICE \
  -v marq-audit:/var/log/marq -v marq-work:/work -e MARQ_OPERATOR marq
```

Or commit a project [`.mcp.json`](https://docs.anthropic.com/en/docs/claude-code/mcp)
with the `mcpServers` block from `mcp.json.example` (same schema). Then in the
session: ask it to call `server_info`, record scope with `set_engagement`, then
`load_skill` the domain you're in.

**Best for:** "review this architecture against zero-trust", "map our findings
to ISO 27001 and SOC 2", "build me an incident runbook", "threat-model this
service", "turn `/work/findings.md` into a board report". The model does the
thinking; marq supplies current frameworks and the evidence tools.

## Codex CLI (frontier expert)

```bash
codex mcp add marq -- docker run --rm -i \
  --cap-drop ALL --cap-add NET_RAW --cap-add NET_ADMIN \
  -v marq-audit:/var/log/marq -v marq-work:/work -e MARQ_OPERATOR marq
```

Or add it to `~/.codex/config.toml` (TOML, not JSON):

```toml
[mcp_servers.marq]
command = "docker"
args = ["run", "--rm", "-i", "--cap-drop", "ALL",
        "--cap-add", "NET_RAW", "--cap-add", "NET_ADMIN",
        "-v", "marq-audit:/var/log/marq", "-v", "marq-work:/work",
        "-e", "MARQ_OPERATOR", "marq"]
env_vars = { MARQ_OPERATOR = "marq" }
```

Same uses as Claude Code. Docs: [Codex MCP](https://developers.openai.com/codex/mcp).

## LM Studio (local model, for testing)

`Program` tab → `Install` → `Edit mcp.json`, paste the `marq` entry from
[`../mcp.json.example`](../mcp.json.example) (LM Studio uses the same
`mcpServers` schema). Load a **tool-capable** GGUF (e.g. a recent Qwen/Llama
that supports function calling), then chat. Docs: [LM Studio MCP](https://lmstudio.ai/docs/app/mcp).

**Best for:** trying marq locally, lighter Q&A, and checking whether a given
local model is good enough at tool-calling before you wire it into Pi.

## Pi (local, uncensored, standalone)

The fully-local path — no cloud, no refusals, on-box execution. See
[`SETUP.md`](SETUP.md#4-run-with-a-local-model-pi--the-marq-skill):

```bash
install -m 0755 pi/marq /opt/homebrew/bin/marq   # or /usr/local/bin
marq up ~/engagements/acme                        # long-lived container
# add an LM Studio provider extension + register the marq skill in
# ~/.pi/agent/settings.json, then run `pi` — full steps in SETUP.md
```

**Best for:** hands-on offensive engagements with an abliterated model — recon,
exploitation, AD, malware triage — where you want everything local and nothing
filtered.

---

## Using it effectively (any client)

- **Always start with `server_info`**, then **`load_skill`** the domain playbook
  before diving in. The skills carry the current-standards detail and the right
  tool order — they're what make a smaller model competent and keep a frontier
  model from guessing at stale facts.
- **Load only the skill you need.** There are ~70; pulling the one relevant
  playbook keeps context tight. `marq run load_skill '{}'` (or the `load_skill`
  tool) lists the index.
- **Scope gates active testing, not advice.** GRC/architecture/standards
  conversations are unrestricted; scanning and exploitation are authorized-only
  and audit-logged. Record scope with `set_engagement` before any active testing.
- **Pick the model to the job.** A big reasoning model for analysis and writing;
  a fast tool-capable local model for chaining scans; an abliterated local model
  for offensive work that a hosted model would refuse.
- **Weak local model?** Lean on the skills, keep it to one tool call at a time,
  and let it read each result before the next step — that discipline beats a
  bigger prompt.
- **Long scans run in the background.** `list_jobs` / `job_status` instead of
  blocking; works the same in every client.
