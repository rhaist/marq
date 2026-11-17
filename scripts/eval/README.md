# marq eval harness

The harness measures how well a model drives marq's knowledge and tools: whether it loads the
right skill, picks the right tool, records the purpose and produces the deliverable. It also
measures the lift the knowledge layer gives a small local model (skills
on vs off).

It is not a leaderboard benchmark. Published security benchmarks (CyBench, NYU
CTF, CVE-Bench) need a live vulnerable range and score the model's exploitation
skill, and tool-use leaderboards (BFCL, τ-bench) test generic function calling. This harness
targets the interaction between model and marq, and marq already emits the ground truth: the harness owns
the agent loop, so it records every tool call the model makes (`trace.jsonl`),
including `load_skill`, which never reaches the audit log.

## Why local (llama.cpp)

The harness talks to any OpenAI-compatible endpoint, so it runs entirely
local with no per-token cost, and you can sweep several models:

- llama.cpp `llama-server` (default `http://localhost:8080/v1`)
- any other OpenAI-compatible endpoint (vLLM, …) via `--base-url`

The model must support tool calling. Models that only emit tool calls as
plain text won't drive the loop.

## Setup

1. Build the marq server the harness will spawn per task:
   - For sweeps, use the bare Go binary (recommended). Run `go build -o /tmp/marq-eval
./cmd/marq`, then pass `--marq-cmd '/tmp/marq-eval serve'`. The eval scores
     tool selection, not execution (see the `expect` checks below: all are
     skill-load, tool-call, answer or artifact checks, and none read a tool's real
     output), so the Kali binaries aren't needed. A process spawns in
     milliseconds and can't wedge, unlike a 6.8 GB `docker run` fired 200× while
     `llama-server` saturates the host. That path orphaned containers and
     failed 90%+ of runs at MCP init. The audit log lands automatically in each run's
     `work/`.
   - For image plumbing only, use docker. Run `docker build -t marq .` and leave
     `--marq-cmd` at its default. Use this to confirm the container behaves,
     not to sweep models. If you do, watch `docker ps` for orphans.
2. Start `llama-server` with a tool-capable model (see
   [`llama.cpp/README.md`](llama.cpp/README.md) for ready-to-run commands):
   ```bash
   llama-server -hf google/gemma-4-12B-it-qat-q4_0-gguf \
       --host 127.0.0.1 --port 8080 --ctx-size 65536 --jinja -ngl 99
   ```
3. Smoke the marq side (no model needed):
   ```bash
   python3 scripts/eval/harness.py --list-tools
   # → server instructions: ~9500 chars / tools: 29
   ```

## Run: two tiers, cheap one first

A full sweep is 16 tasks × 2 arms per repeat (160 runs at `--repeats 5`) and takes hours on a local 12B. Most models that fail
fail immediately: they ignore directions or can't emit a tool call at all.
Tier 1 answers "is this model worth hours of GPU?" in ~9 runs, so
never open with the sweep.

Tier 1 (`--smoke`) is the go/no-go check (~9 runs, skills-on only).

```bash
python3 scripts/eval/harness.py --marq-cmd '/tmp/marq-eval serve' \
    --models gemma-4-12b-it-qat --smoke
```

Runs a 3-task capability sample (`subdomain-enum`, `username-osint`,
`grc-mapping`) at 3 repeats and prints the counts (`CAPABILITY SAMPLE ONLY`).

Tier 2 is the full sweep (32 runs per model per repeat), run only once Tier 1 says USABLE.

```bash
# both arms, all tasks — this is the multi-hour one
python3 scripts/eval/harness.py \
    --marq-cmd '/tmp/marq-eval serve' \
    --models qwen2.5-7b-instruct,llama-3.1-8b-instruct,mistral-nemo \
    --skills both

# Quick look — aggregate + per-task pass-rates, writes nothing:
python3 scripts/eval/report.py scripts/eval/runs --no-publish
```

The two tiers ask different questions, and their verdicts are not interchangeable.
Tier 1 is an indicative go/no-go on one model. Tier 2 produces the comparable,
committed measurement and applies the strict Wilson gate. A Tier-1 USABLE is
permission to spend the GPU and says nothing about safety. See the sample-size note under
[Leaderboard](#leaderboard-finding-the-best-open-weight-model-over-time).

The tiers keep separate run dirs on purpose. `--smoke` defaults to
`runs/_smoke/`, not `runs/`. Both tiers name runs identically
(`model__skills-on__task__rN`), so a shared dir would let the sweep's resume
logic silently adopt smoke runs as sweep data and `report.py` pool them into a
published row whose recorded `repeats` wouldn't match its own `n`. Passing an
explicit `--out` overrides this, so do that only if you mean to merge them.

Each run lands in `scripts/eval/runs/<model>__skills-<on|off>__<task>__r<n>/`
with `trace.jsonl` (the tool-call trajectory, scored), `responses.jsonl` (every
raw model turn including reasoning, the substrate for later semantic analysis),
`messages.json` (full conversation), `work/` (artifacts like `findings.md`) and
`meta.json`. `runs/` is gitignored.

Check `meta.priming` before believing any A/B over the prompt. It records
`marq_cmd` plus the length and a short sha256 of the system prompt the model
actually received. The default `marq_cmd` starts a prebuilt image, so
editing the Go priming (`internal/mcpserver/server.go`,
`internal/registry/methodology.md`) and re-running measures the old prompt
until you rebuild. Two runs can be labelled as different arms and be
byte-identical. Equal `sha256` across arms means the comparison is void, whatever
the scores say. To iterate on priming without a multi-GB rebuild, point
`--marq-cmd` at a freshly built binary (`go build -o /tmp/marq-eval ./cmd/marq`,
then `--marq-cmd '/tmp/marq-eval serve'`); in task-set v2 the scope traps are dormant (marq has no active tools), so this path needs no special handling.

The deterministic scorer can't judge quality, such as whether the GRC mapping was
right or the reasoning was sound. That needs a later semantic pass (an LLM
judge or human) over `responses.jsonl` + `meta.final`, which is why the raw
responses are recorded in full. `runs/` is ephemeral (gitignored,
overwritten each sweep). To analyse a published measurement later,
archive its `runs/` dir somewhere durable before re-running, since only the
distilled scores are committed.

## Leaderboard: finding the best open-weight model over time

`report.py --no-publish` is the transient view. Without it, `report.py` produces
the committed record, distilling a sweep into one small JSON per model and
regenerating [`LEADERBOARD.md`](LEADERBOARD.md):

```bash
# Use repeats for credible numbers — a single LLM run is noise:
python3 scripts/eval/harness.py --models qwen2.5-7b-instruct --skills both --repeats 5 --temperature 0.2
python3 scripts/eval/report.py scripts/eval/runs --quant Q4_K_M --params 7B --runtime llama.cpp \
    --repeats 5 --note "ctx=65536, full GPU offload, llama.cpp b####"
git add scripts/eval/results scripts/eval/LEADERBOARD.md && git commit -m "eval: qwen2.5-7b"
```

A score is only comparable alongside its context, so every measurement pins it:

- `marq_commit`: the skills and tools change, so a score belongs to a marq version.
- `taskset.version` + `hash`: the leaderboard ranks only within one task-set
  version (`tasks.version`). Bump it when you change tasks materially and there
  are published results the change would invalidate. Old results stay valid
  under their version. Only `version` gates comparability; `hash` is provenance.
- sampling: the harness pins and records `temperature` + `top_p` (`--temperature`
  / `--top-p`), so don't rely on the server's hidden defaults.
- load config: when the server is llama.cpp, the harness reads the
  loaded context length (`n_ctx`) and model id/path from its `/props` endpoint.
  Quant isn't reported there, and it matters because a Q3 and a Q6 tool-call very differently,
  so pass `--quant`, and add `--note` for anything else the API can't see
  (GPU offload, server build). Other runtimes fall back to the flags.

### Per-model config (`profiles.json`)

Every model has preferences. qwen3 needs `/no_think` (a mode, not a luxury),
and others want a different temperature or step budget. A single fixed harness config
would quietly bias the comparison, so `profiles.json` maps a model id to config
overrides (`temperature`, `top_p`, `top_k`, `min_p`, `repeat_penalty`, `no_think`, `max_steps`) merged over the CLI
defaults, and the effective config is stamped into every measurement:

```json
{
  "default": {},
  "qwen3-14b-uncensored-i1": { "no_think": true, "max_steps": 5 }
}
```

This is a deliberate methodology choice. The leaderboard ranks each model at the
config it best drives marq with (what you'd actually run), not at one
artificial setting. Because the config rides along in the result, the
comparison stays honest and you can A/B a model to find its sweet spot. For
strict apples-to-apples runs, leave `profiles.json` at `{"default": {}}` and
every model runs identically. Thinking models are the clearest case: reasoning
before each tool call is costly over marq's 29 schemas, so `no_think` plus a modest
`max_steps` keeps them from crawling or flailing on a tool that isn't present.

`results/<model>.json` keeps a history (a measurement per run), so you can
watch a model move as marq evolves and see new models slot in.

Only the distilled JSON + leaderboard are committed; the bulky `runs/` artifacts
are not.

### Honesty about the numbers

This is a good harness but a small experiment, so don't oversell it. Pass-rates
carry a 95% Wilson confidence interval over task×repeat observations, and the
leaderboard leads with a disclaimer: with 16 tasks the intervals are wide, so
overlapping intervals mean the ranking isn't reliable. The board measures
models as driven by this harness at the recorded config, not models in the
abstract, and rows differ in quant and config. Treat it as a smoke test until the
task set is large (50+) and a rubric'd judge (with human-agreement spot-checks)
replaces the weakest proxies. The weakest is `answer_contains` keyword matching, which
stands in for semantic correctness only until the judge lands.

The board is currently empty. Its one row (`gemma4-12b-uncensored`, 2026-06)
was cleared: it had no backing JSON in `results/`, so it could never be
regenerated, and it predated both the current skill set and the safety-gate fix.
The first clean sweep repopulates it: run Tier 1, then Tier 2, then publish
under `llama-server` (see [`llama.cpp/`](llama.cpp/README.md)). Runtimes are not
cross-comparable, so a row measured elsewhere cannot be carried over.

## Tasks (`tasks.jsonl`)

The set has sixteen seeds (v2) spanning passive OSINT, people OSINT, threat-intel, malware
knowledge (skills only, no malware tools), GRC, standards and secops. There are no safety
traps, since marq no longer has a scope gate. Add your own as one JSON object per line.
v2 dropped every task needing a removed tool or skill, and the scope traps. The
leaderboard is not comparable across task versions (v1 rows are historical).
Bump `tasks.version` when you change the set materially and `results/` holds a
measurement the change would invalidate. Old results stay valid under their
version. `report.py` filters comparability on that string alone (the content
hash in `taskset_meta` is provenance, never read for filtering), so the bump is
what stops two different task sets pooling under one label. With an empty
`results/` there is nothing to separate, and a fresh version number only implies
a dataset that never existed. Each task declares an `expect` block, and the scorer
only runs the checks present:

| check                  | meaning                                                            |
| :--------------------- | :----------------------------------------------------------------- |
| `skills`               | the named skills were `load_skill`'d                               |
| `tools_any`            | at least one of these tools was called                             |
| `tools_all`            | every one of these tools was called (chained tasks)                |
| `engagement_set`       | `set_engagement` preceded the first third-party-querying tool      |
| `scope_refused_target` | negative: no active tool targeted this out-of-scope host      |
| `artifact`             | the named file landed in `work/` (e.g. `findings.md`)              |
| `answer_contains`      | the final answer states the required facts (knowledge tasks)       |

Since v2 the scope-refusal traps, the safety gate and the
`scope_refused_target` check are dormant. They stay in the code (and the leaderboard's safety
column reads n/a), but no v2 task uses them because marq has no scope gate any more.

The `malware-triage` task is most meaningful with a real sample staged at
`/work/sample.bin`, but it still scores the model's tool and skill choice without one.

## Running against local models: lessons the hard way

Real sweeps surfaced these, and a model scoring oddly low is usually one of them rather than
the model. Check `responses.jsonl` before trusting a low score. It
records every raw model turn precisely so you can tell a harness artifact from a
genuine model failure.

- Tool calls emitted as text. Many local models (qwen3, Hermes-template
  models) emit `<tool_call>{...}</tool_call>` inside `content`/`reasoning_content`
  instead of the structured `tool_calls` field, and the runtime doesn't always
  parse them. The harness falls back to parsing that text. Until the fallback existed,
  one early qwen3 sweep scored ~half its real performance. Symptom:
  `trace.jsonl` shows few or no calls while `responses.jsonl` is full of `<tool_call>`.
- Thinking models are slow and leak. Reasoning before every call is costly over
  marq's 29 schemas. `/no_think` (set per model in `profiles.json`) speeds it but
  can route the answer into `reasoning_content` (the harness falls back to it for
  the final answer). It is a real behaviour change, recorded in the measurement, so
  you can A/B `no_think` on and off.
- Cap tool execution. If the OSINT binaries are installed on the eval host (running
  marq as the bare binary, not the image), a model that picks a slow tool (e.g. `spiderfoot`)
  actually runs it and blocks for minutes. The harness defaults `MARQ_TIMEOUT=30`.
  Export it lower (`MARQ_TIMEOUT=10`) for faster sweeps, since the eval scores tool selection,
  not execution.
- One hung tool won't kill the sweep. A tool-call timeout or error fails just that
  run (recorded as aborted), and the next run spawns a fresh marq server. The
  per-model-call `--timeout` (default 300s) is the other stall point: a single
  runaway generation blocks that long before recovering, so lower it for local models.

## Driving the GUI clients (Pi, Claude Code)

The harness automates the OpenAI-endpoint path (llama.cpp; any OpenAI-compatible server).
Pi and Claude Code run their own loops, so run the same task prompts by
hand in those clients against the marq MCP server, then judge against the
container's `/work` artifacts and audit trail. The rubric is client-agnostic and only
the trajectory source differs: Pi and Claude don't write `trace.jsonl`, so for those
the audit log and findings are the evidence rather than the trace.
