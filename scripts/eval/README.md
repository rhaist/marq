# marq eval harness

The harness measures how well a model drives marq's knowledge and tools: whether it loads the
right skill, picks the right tool, records the purpose and produces the deliverable. It also
measures the lift the knowledge layer gives a small local model (skills
on vs off).

It is not a benchmark. Published security benchmarks (CyBench, NYU
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
2. Start `llama-server` with a tool-capable model (ready-to-run command in
   [`llama.cpp/README.md`](llama.cpp/README.md)).
3. Smoke the marq side (no model needed):
   ```bash
   python3 scripts/eval/harness.py --list-tools
   # → server instructions: <chars> / tools: <count>
   ```

## Run: two tiers, cheap one first

A full sweep is 15 tasks × 2 arms per repeat (150 runs at `--repeats 5`) and takes hours on a local 12B. Most models that fail
fail immediately: they ignore directions or can't emit a tool call at all.
Tier 1 answers "is this model worth hours of GPU?" in ~9 runs, so
never open with the sweep.

Tier 1 (`--smoke`) is the go/no-go check (~9 runs, skills-on only).

```bash
python3 scripts/eval/harness.py --marq-cmd '/tmp/marq-eval serve' \
    --models gemma-4-12b-it-qat --smoke
```

Runs a 3-task capability sample (`subdomain-enum`, `username-osint`,
`grc-mapping`) at 3 repeats and prints per-task pass counts.

Tier 2 is the full sweep (30 runs per model per repeat), run only once Tier 1 looks promising.

```bash
# both arms, all tasks — this is the multi-hour one
python3 scripts/eval/harness.py \
    --marq-cmd '/tmp/marq-eval serve' \
    --models qwen2.5-7b-instruct,llama-3.1-8b-instruct,mistral-nemo \
    --skills both

# Pass-rates with intervals, lift and per-task rates:
python3 scripts/eval/report.py scripts/eval/runs
```

The two tiers ask different questions, and their verdicts are not interchangeable.
Tier 1 is an indicative go/no-go on one model. Tier 2 produces the measurement
with Wilson intervals. A good Tier-1 count is permission to spend the GPU. See
[Reading the numbers](#reading-the-numbers).

The tiers keep separate run dirs on purpose. `--smoke` defaults to
`runs/_smoke/`, not `runs/`. Both tiers name runs identically
(`model__skills-on__task__rN`), so a shared dir would let the sweep's resume
logic silently adopt smoke runs as sweep data and `report.py` pool them into one
score. Passing an
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
then `--marq-cmd '/tmp/marq-eval serve'`).

The deterministic scorer can't judge quality, such as whether the GRC mapping was
right or the reasoning was sound. That needs a later semantic pass (an LLM
judge or human) over `responses.jsonl` + `meta.final`, which is why the raw
responses are recorded in full. `runs/` is ephemeral (gitignored,
overwritten each sweep), so archive a `runs/` dir somewhere durable if you want
to analyse it later.

## Per-model config (`profiles.json`)

Every model has preferences. qwen3 needs `/no_think` (a mode, not a luxury),
and others want a different temperature or step budget. `profiles.json` maps a
model id to config overrides (`temperature`, `top_p`, `top_k`, `min_p`,
`repeat_penalty`, `no_think`, `max_steps`) merged over the CLI defaults, and the
effective config is recorded in each run's `meta.json`:

```json
{
  "default": {},
  "qwen3-14b-uncensored-i1": { "no_think": true, "max_steps": 5 }
}
```

Each model runs at the config it best drives marq with. For strict
apples-to-apples runs, leave `profiles.json` at `{"default": {}}`. Thinking
models are the clearest case: reasoning before each tool call is costly over
marq's tool schemas, so `no_think` plus a modest `max_steps` keeps them from
crawling or flailing on a tool that isn't present.

## Reading the numbers

`report.py` pools repeats into pass-rates with 95% Wilson intervals over
task×repeat observations. With 15 tasks the intervals are wide, so two models
whose intervals overlap are not distinguishable. The scores describe a model as
driven by this harness at its recorded config, quant and runtime, and are only
comparable for the same marq commit and task set. Treat them as a smoke test:
the weakest check is `answer_contains` keyword matching, which stands in for
semantic correctness until a rubric'd judge replaces it. `report.py` flags a
sweep where more than 10% of runs aborted; those are environment failures, and
the surviving runs don't measure the model.

## Tasks (`tasks.jsonl`)

The set has fifteen seeds spanning passive OSINT, people OSINT, threat-intel, malware
knowledge (skills only, no malware tools), GRC, standards and secops. Add your own as one
JSON object per line. Each task declares an `expect` block, and the scorer
only runs the checks present:

| check             | meaning                                                       |
| :---------------- | :------------------------------------------------------------ |
| `skills`          | the named skills were `load_skill`'d                          |
| `tools_any`       | at least one of these tools was called                        |
| `tools_all`       | every one of these tools was called (chained tasks)           |
| `engagement_set`  | `set_engagement` preceded the first third-party-querying tool |
| `artifact`        | the named file landed in `work/` (e.g. `findings.md`)         |
| `answer_contains` | the final answer states the required facts (knowledge tasks)  |

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
  marq's tool schemas. `/no_think` (set per model in `profiles.json`) speeds it but
  can route the answer into `reasoning_content` (the harness falls back to it for
  the final answer). It is a real behaviour change, recorded in the measurement, so
  you can A/B `no_think` on and off.
- Cap tool execution. If the OSINT binaries are installed on the eval host (running
  marq as the bare binary, not the image), a model that picks a slow tool (e.g. `gau_urls`)
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
