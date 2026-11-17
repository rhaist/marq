#!/usr/bin/env python3
"""marq eval harness — drive marq's MCP tools with an OpenAI-compatible model and
record the tool-call trajectory for scoring (scripts/eval/report.py).

Free + local by design: point --base-url at llama.cpp's `llama-server` (default
:8080/v1) — or any OpenAI-compatible endpoint — and sweep --models: nothing
leaves the box and there is no per-token cost. The skills on/off ablation measures
the *lift* from marq's knowledge layer (does loading skills make a small model
pick the right tool?).

The harness owns the loop (model <-> marq MCP), so it sees every tool call the
model makes — including load_skill, which is in-process and never hits the audit
log. That trace.jsonl is the ground truth; /work artifacts are the cross-check.

    python3 scripts/eval/harness.py --models qwen2.5-7b-instruct,llama-3.1-8b \
        --skills both --tasks scripts/eval/tasks.jsonl --out scripts/eval/runs
    python3 scripts/eval/harness.py --list-tools        # smoke the marq MCP leg

Requires a built `marq` image (the default --marq-cmd docker-runs it). For a
quick local smoke without Kali: --marq-cmd 'go run ./cmd/marq serve' (in-process
tools only; exec tools like subfinder are listed but their Kali binaries are missing).
"""

from __future__ import annotations
import argparse, hashlib, json, os, re, select, shlex, subprocess, sys, time, urllib.request, pathlib

DEFAULT_MARQ_CMD = (
    "docker run --rm -i -v {work}:/work -e MARQ_OPERATOR=eval "
    "-e MARQ_TIMEOUT "  # inherit the per-tool runtime cap set in spawn_marq's env
    "marq"
)

# --smoke Tier-1 go/no-go: "is this model usable — follows directions?" Runs this
# small capability sample, skills-on. Answers the cheap question in minutes instead of the full sweep.
SMOKE_CAPABILITY = ["subdomain-enum", "username-osint", "grc-mapping"]


class MCP:
    """Minimal newline-delimited JSON-RPC MCP client over a subprocess (the same
    shape scripts/test_tools.py uses)."""

    def __init__(self, proc: subprocess.Popen):
        self.proc, self._id = proc, 0

    def _send(self, method, params=None, *, notify=False):
        msg = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            msg["params"] = params
        if not notify:
            self._id += 1
            msg["id"] = self._id
        self.proc.stdin.write(json.dumps(msg) + "\n")
        self.proc.stdin.flush()
        return None if notify else self._id

    def _recv(self, want_id, timeout):
        deadline = time.monotonic() + timeout
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError(f"no response to id={want_id} within {timeout}s")
            r, _, _ = select.select([self.proc.stdout], [], [], remaining)
            if not r:
                continue
            line = self.proc.stdout.readline()
            if line == "":
                raise EOFError("marq server closed the stream")
            line = line.strip()
            if not line:
                continue
            try:
                obj = json.loads(line)
            except json.JSONDecodeError:
                continue  # stray banner/log line on stdout
            if obj.get("id") == want_id:
                return obj

    def call(self, method, params, timeout=120):
        return self._recv(self._send(method, params), timeout)

    def initialize(self):
        resp = self.call(
            "initialize",
            {
                "protocolVersion": "2025-11-25",
                "capabilities": {},
                "clientInfo": {"name": "marq-eval", "version": "1.0"},
            },
            timeout=120,
        )
        self._send("notifications/initialized", {}, notify=True)
        return resp.get("result", {})

    def list_tools(self):
        return (
            self.call("tools/list", {}, timeout=120).get("result", {}).get("tools", [])
        )

    def call_tool(self, name, args, timeout=60):
        resp = self.call("tools/call", {"name": name, "arguments": args}, timeout)
        result = resp.get("result", {})
        parts = [
            c.get("text", "")
            for c in result.get("content", [])
            if c.get("type") == "text"
        ]
        return "\n".join(parts), bool(result.get("isError"))


_TOOLCALL_RE = re.compile(r"<tool_call>\s*(\{.*?\})\s*</tool_call>", re.S)


def text_tool_calls(msg):
    """Fallback: many local models (qwen/Hermes templates) emit tool calls as
    `<tool_call>{...}</tool_call>` text in content/reasoning_content instead of
    the structured tool_calls field. Parse those so we don't silently drop the
    model's actions. Returns a list of {name, arguments}."""
    out = []
    for field in ("content", "reasoning_content", "reasoning"):
        for m in _TOOLCALL_RE.finditer(msg.get(field) or ""):
            try:
                c = json.loads(m.group(1))
                if isinstance(c.get("name"), str):
                    out.append(c)
            except Exception:
                pass
    return out


def openai_tools(tools):
    return [
        {
            "type": "function",
            "function": {
                "name": t["name"],
                "description": t.get("description", ""),
                "parameters": t.get("inputSchema")
                or {"type": "object", "properties": {}},
            },
        }
        for t in tools
    ]


def native_model_info(base_url, model):
    """Load-time metadata from llama.cpp's `/props` endpoint — the loaded context
    length (default_generation_settings.n_ctx) and the model id/path. Best-effort:
    empty dict for non-llama.cpp servers or on any failure, so callers fall back to
    the --quant / --note flags. Temperature is deliberately absent — it's a request
    param, not a load property, and the harness pins it.

    The keys are normalized to the same shape the rest of the harness expects
    (loaded_context_length) so downstream code (meta.json, report.py) is unchanged.
    """
    root = base_url.rstrip("/")
    root = root[:-3] if root.endswith("/v1") else root
    try:
        with urllib.request.urlopen(root.rstrip("/") + "/props", timeout=10) as r:
            props = json.load(r)
        info = {}
        gen = props.get("default_generation_settings") or {}
        n_ctx = gen.get("n_ctx") or props.get("n_ctx")
        if n_ctx:
            info["loaded_context_length"] = n_ctx
        model_id = props.get("model_path") or props.get("model")
        if model_id:
            info["model_path"] = model_id
        return info
    except Exception:
        pass
    return {}


def chat(base_url, api_key, model, messages, tools, timeout, sampling):
    body = {"model": model, "messages": messages, "stream": False, **sampling}
    if tools:
        body["tools"] = tools
        body["tool_choice"] = "auto"
    req = urllib.request.Request(
        base_url.rstrip("/") + "/chat/completions",
        data=json.dumps(body).encode(),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {api_key}",
        },
    )
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.load(r)["choices"][0]["message"]


def spawn_marq(marq_cmd, work):
    env = dict(os.environ, MARQ_OPERATOR="eval", MARQ_WORK_DIR=str(work))
    # The eval scores tool *selection*, not execution — cap tool runtime so a
    # model that picks a full-range scan can't block the sweep for minutes when
    # the real binary is installed locally. Override by exporting MARQ_TIMEOUT.
    env.setdefault("MARQ_TIMEOUT", "30")
    # Bare-binary sweeps (--marq-cmd '<bin> serve') have no writable /var/log,
    # so keep each run's audit log inside its work dir — else tool calls are
    # refused fail-closed and the model reacts to spurious audit errors. The
    # docker cmd only forwards -e MARQ_OPERATOR/-e MARQ_TIMEOUT, so the
    # container ignores this and keeps its own /var/log path.
    env.setdefault("MARQ_AUDIT_LOG", str(pathlib.Path(work) / "audit.jsonl"))
    return subprocess.Popen(
        shlex.split(marq_cmd.format(work=work)),
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        env=env,
    )


def shutdown_marq(proc):
    """Close stdin first: `marq serve` exits on EOF, so a `docker run -i` container
    stops itself and `--rm` reaps it. terminate()-ing only the docker *client* can
    orphan the container; orphans starve later runs until the sweep wedges at init."""
    try:
        if proc.stdin:
            proc.stdin.close()
    except Exception:
        pass
    proc.terminate()
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        proc.kill()


# A wedged init is transient (a prior run's container may still be reaping), but an
# aborted run is unusable data — and report.py refuses to publish a sweep whose
# abort fraction exceeds --max-abort-frac, so a cluster of them sinks the whole
# sweep. Retrying with a fresh process costs seconds; not retrying costs a re-run.
INIT_ATTEMPTS = 3
INIT_BACKOFF = 5  # seconds, multiplied by the attempt number


def priming_fingerprint(marq_cmd, system=None):
    """Provenance for the system prompt the model actually received.

    Sampling and model path were already stamped into meta.json; the priming was
    not, and that is the blind spot. `marq_cmd` decides where the prompt comes
    from: the default starts a *prebuilt image*, so editing the Go source and
    re-running silently measures the old prompt — two runs can be labelled as
    different arms and be byte-identical here. Comparing `sha256` across runs
    settles "did these two actually see different priming?" from the data
    instead of from memory.
    """
    fp = {"marq_cmd": marq_cmd, "chars": None, "sha256": None}
    if system is not None:
        fp["chars"] = len(system)
        fp["sha256"] = hashlib.sha256(system.encode()).hexdigest()[:12]
    return fp


def run_task(args, model, skills_on, task, repeat, model_info, cfg):
    slug = model.replace("/", "_").replace(":", "_")
    rundir = (
        pathlib.Path(args.out)
        / f"{slug}__skills-{'on' if skills_on else 'off'}__{task['id']}__r{repeat}"
    )
    # Resume: a long sweep can be killed midway (background-task time caps), so
    # skip runs that already completed cleanly — a real trajectory, not an
    # init-aborted stub. Re-run with --fresh to force every run.
    if not getattr(args, "fresh", False):
        meta = rundir / "meta.json"
        if meta.is_file():
            try:
                d = json.loads(meta.read_text())
                fin = str(d.get("final", ""))
                crashed = fin.startswith("[harness error") or fin.startswith("[aborted")
                # Done = a real trajectory (a text-only refusal has steps==0 but a
                # real final — still done). Aborted runs are re-run on resume.
                if not crashed and (d.get("steps", 0) > 0 or fin):
                    print(f"  {rundir.name}: already done ({d['steps']} steps) — skip")
                    return rundir
            except Exception:
                pass
    work = rundir / "work"
    work.mkdir(parents=True, exist_ok=True)

    # A wedged handshake is usually transient, so retry with a fresh process; only a
    # persistent one aborts THIS run, and never the sweep.
    for attempt in range(1, INIT_ATTEMPTS + 1):
        proc = spawn_marq(args.marq_cmd, work.resolve())
        mcp = MCP(proc)
        try:
            init = mcp.initialize()
            tools = mcp.list_tools()
            break
        except Exception as e:
            shutdown_marq(proc)
            if attempt < INIT_ATTEMPTS:
                print(
                    f"  {rundir.name}: init failed ({e}) — retrying ({attempt}/{INIT_ATTEMPTS - 1})"
                )
                time.sleep(INIT_BACKOFF * attempt)
                continue
            (rundir / "trace.jsonl").write_text("")
            (rundir / "responses.jsonl").write_text("")
            (rundir / "meta.json").write_text(
                json.dumps(
                    {
                        "model": model,
                        "skills_on": skills_on,
                        "task": task["id"],
                        "repeat": repeat,
                        "sampling": {
                            k: v
                            for k, v in cfg.items()
                            if k not in ("no_think", "max_steps")
                        },
                        "no_think": cfg["no_think"],
                        "max_steps": cfg["max_steps"],
                        "base_url": args.base_url,
                        "model_info": model_info,
                        "priming": priming_fingerprint(
                            args.marq_cmd
                        ),  # no prompt built yet
                        "steps": 0,
                        "final": f"[harness error: init: {e}]",
                    },
                    indent=2,
                )
            )
            print(
                f"  {rundir.name}: init failed after {INIT_ATTEMPTS} attempts ({e}) — aborted, continuing"
            )
            return rundir

    try:
        if not skills_on:
            tools = [t for t in tools if t["name"] != "load_skill"]
            # NOTE (known confound): this bare prompt differs from the skills-on
            # arm's full server instructions (methodology + engagement/
            # skill-loading/reporting briefing), so the measured "lift" reflects briefing+skills
            # combined, not the skill content alone. Kept as-is to preserve
            # comparability with the committed LEADERBOARD; revisit as a
            # methodology change (hold this prompt identical to the on-arm, minus
            # load_skill) if isolating the knowledge-layer effect becomes the goal.
            system = (
                "You are a security operator with the following tools. Use them to "
                "complete the task."
            )
        else:
            system = init.get("instructions") or "You are marq, a security assistant."
        if cfg["no_think"]:
            system += (
                "\n\n/no_think"  # qwen3 et al.: skip the reasoning block (much faster)
            )

        messages = [
            {"role": "system", "content": system},
            {"role": "user", "content": task["prompt"]},
        ]
        tool_schemas = openai_tools(tools)
        # Everything in cfg except the harness-control keys is sampling, so a
        # profile can carry a model's full recommended set (top_k/min_p/
        # repeat_penalty etc.) — llama.cpp's OpenAI server accepts the sampling
        # extras (top_k, min_p, repeat_penalty).
        sampling = {k: v for k, v in cfg.items() if k not in ("no_think", "max_steps")}
        trace = []  # tool calls (scored)
        responses = []  # raw model turns incl. reasoning (for later semantic analysis)
        final = ""
        for step in range(cfg["max_steps"]):
            try:
                msg = chat(
                    args.base_url,
                    args.api_key,
                    model,
                    messages,
                    tool_schemas,
                    args.timeout,
                    sampling,
                )
            except Exception as e:  # endpoint down / model missing — record and stop
                final = f"[harness error: {e}]"
                break
            responses.append(
                {"step": step, "message": msg}
            )  # full raw response, untrimmed
            structured = msg.get("tool_calls") or []
            text_calls = [] if structured else text_tool_calls(msg)
            if (
                not structured and not text_calls
            ):  # final answer (content, or reasoning if content empty)
                final = msg.get("content") or msg.get("reasoning_content") or ""
                break
            if structured:
                messages.append(
                    {
                        k: msg[k]
                        for k in ("role", "content", "tool_calls")
                        if msg.get(k) is not None
                    }
                )
            else:  # text-form calls: record the turn as plain assistant text
                messages.append(
                    {
                        "role": "assistant",
                        "content": (
                            msg.get("content") or msg.get("reasoning_content") or ""
                        )[:4000],
                    }
                )
            text_results = []
            aborted = False
            for tc in structured or text_calls:
                if structured:
                    fn = tc.get("function", {})
                    name, raw = fn.get("name", ""), fn.get("arguments") or "{}"
                    a = raw if isinstance(raw, dict) else _loads(raw)
                else:
                    name, a = tc.get("name", ""), tc.get("arguments") or {}
                    if isinstance(a, str):
                        a = _loads(a)
                via = "structured" if structured else "text"
                try:  # a hung/dead tool fails THIS run, never the whole sweep
                    out, is_err = mcp.call_tool(name, a)
                except Exception as e:
                    trace.append(
                        {
                            "step": step,
                            "name": name,
                            "args": a,
                            "is_error": True,
                            "result_head": f"[tool call failed: {e}]"[:600],
                            "via": via,
                        }
                    )
                    final = final or f"[aborted: {name} {e}]"
                    aborted = True
                    break
                trace.append(
                    {
                        "step": step,
                        "name": name,
                        "args": a,
                        "is_error": is_err,
                        "result_head": out[:600],
                        "via": via,
                    }
                )
                if structured:
                    messages.append(
                        {
                            "role": "tool",
                            "tool_call_id": tc.get("id", ""),
                            "content": out,
                        }
                    )
                else:
                    text_results.append(f"{name} -> {out}")
            if aborted:  # marq server likely wedged; end this run, next spawns fresh
                break
            if text_results:  # feed text-call results back as a user turn (no tool_call_id to bind to)
                messages.append(
                    {
                        "role": "user",
                        "content": "[tool results]\n" + "\n".join(text_results),
                    }
                )

        (rundir / "trace.jsonl").write_text(
            "".join(json.dumps(t) + "\n" for t in trace)
        )
        (rundir / "responses.jsonl").write_text(
            "".join(json.dumps(r) + "\n" for r in responses)
        )
        (rundir / "messages.json").write_text(json.dumps(messages, indent=2))
        (rundir / "meta.json").write_text(
            json.dumps(
                {
                    "model": model,
                    "skills_on": skills_on,
                    "task": task["id"],
                    "repeat": repeat,
                    "sampling": sampling,
                    "no_think": cfg["no_think"],
                    "max_steps": cfg["max_steps"],
                    "base_url": args.base_url,
                    "model_info": model_info,
                    "priming": priming_fingerprint(args.marq_cmd, system),
                    "steps": len(trace),
                    "final": final,
                },
                indent=2,
            )
        )
        print(f"  {rundir.name}: {len(trace)} tool calls")
        return rundir
    finally:
        shutdown_marq(proc)


def smoke_verdict(out_dir, model, tasks):
    """Tier-1 smoke: print per-task pass counts (skills-on, completed runs only)
    for the capability sample. Indicative only, no pass/fail gate; returns False
    when no run completed, so a crashed or empty sweep never looks like a result."""
    import report  # sibling module; sys.path[0] is this script's dir when run directly

    out = pathlib.Path(out_dir)
    slug = model.replace("/", "_").replace(":", "_")
    bar = "=" * 60
    print(
        f"\n{bar}\nSMOKE — {model}  (capability sample, skills-on, completed runs only)\n{bar}"
    )
    completed = 0
    for t in tasks:
        dirs = [
            d
            for d in sorted(out.glob(f"{slug}__skills-on__{t['id']}__r*"))
            if (d / "meta.json").exists()
            and not report.is_aborted(json.loads((d / "meta.json").read_text()))
        ]
        print(f"  {sum(report.passed(t, d) for d in dirs)}/{len(dirs)}  {t['id']}")
        completed += len(dirs)
    print(bar)
    if not completed:
        print("  no run completed: fix the environment and re-run")
    return completed > 0


def _loads(s):
    try:
        return json.loads(s)
    except Exception:
        return {}


def main():
    p = argparse.ArgumentParser(description="marq eval harness")
    p.add_argument(
        "--base-url",
        default="http://localhost:8080/v1",
        help="OpenAI-compatible endpoint (llama.cpp llama-server default)",
    )
    p.add_argument(
        "--api-key",
        default="llama",
        help="ignored by most local servers; some require non-empty",
    )
    p.add_argument("--models", default="", help="comma-separated model ids to sweep")
    p.add_argument("--skills", choices=["on", "off", "both"], default="both")
    p.add_argument(
        "--tasks", default=str(pathlib.Path(__file__).with_name("tasks.jsonl"))
    )
    p.add_argument("--out", default=str(pathlib.Path(__file__).with_name("runs")))
    p.add_argument(
        "--marq-cmd",
        default=DEFAULT_MARQ_CMD,
        help="command to start `marq serve`; {work} is substituted",
    )
    p.add_argument("--max-steps", type=int, default=12)
    p.add_argument(
        "--repeats",
        type=int,
        default=1,
        help="runs per task (use 3+ for published numbers; LLMs are stochastic)",
    )
    p.add_argument(
        "--fresh",
        action="store_true",
        help="re-run every task even if a completed result exists (default: resume, skipping done runs)",
    )
    p.add_argument(
        "--temperature",
        type=float,
        default=0.2,
        help="pinned + recorded for reproducibility",
    )
    p.add_argument(
        "--top-p", type=float, default=1.0, help="pinned + recorded for reproducibility"
    )
    p.add_argument(
        "--no-think",
        action="store_true",
        help="default; append /no_think to skip reasoning (qwen3 et al.) — override per model in profiles.json",
    )
    p.add_argument(
        "--profiles",
        default=str(pathlib.Path(__file__).with_name("profiles.json")),
        help="per-model config overrides (temperature/top_p/no_think/max_steps)",
    )
    p.add_argument("--timeout", type=int, default=300, help="per model call (s)")
    p.add_argument(
        "--list-tools", action="store_true", help="smoke the marq MCP leg and exit"
    )
    p.add_argument(
        "--smoke",
        action="store_true",
        help="Tier-1 go/no-go: a small capability sample, skills-on, 3x — prints per-task pass counts instead of the leaderboard sweep",
    )
    args = p.parse_args()

    if args.list_tools:
        work = pathlib.Path(args.out) / "_smoke"
        work.mkdir(parents=True, exist_ok=True)
        proc = spawn_marq(args.marq_cmd, work.resolve())
        try:
            mcp = MCP(proc)
            init = mcp.initialize()
            tools = mcp.list_tools()
            print(f"server instructions: {len(init.get('instructions', ''))} chars")
            print(f"tools: {len(tools)}")
            print("  " + ", ".join(t["name"] for t in tools[:12]) + " ...")
        finally:
            proc.terminate()
        return 0

    # Keep the two tiers' run dirs apart. Smoke uses the same naming as a sweep
    # (model__skills-on__task__rN), so sharing --out would let the sweep's resume
    # logic adopt smoke runs as sweep data and report.py pool them into a
    # published row — whose recorded `repeats` would then not match its own n.
    # An explicit --out still wins, so you can point both tiers anywhere.
    if args.smoke and args.out == p.get_default("out"):
        args.out = str(pathlib.Path(args.out) / "_smoke")

    profiles = (
        json.loads(pathlib.Path(args.profiles).read_text())
        if pathlib.Path(args.profiles).is_file()
        else {}
    )
    models = [m.strip() for m in args.models.split(",") if m.strip()]
    if not models:
        p.error("pass --models (comma-separated) or --list-tools")
    skills = [True, False] if args.skills == "both" else [args.skills == "on"]
    tasks = [json.loads(l) for l in open(args.tasks) if l.strip()]

    if args.smoke:  # Tier-1: small capability sample, skills-on only
        tasks = [t for t in tasks if t["id"] in SMOKE_CAPABILITY]
        skills = [True]
        if args.repeats == 1:
            args.repeats = 3

    for model in models:
        # CLI flags are the defaults; the model's profile (else "default") overrides
        # them, so each model competes under its own recorded config.
        cfg = {
            "temperature": args.temperature,
            "top_p": args.top_p,
            "no_think": args.no_think,
            "max_steps": args.max_steps,
        }
        cfg.update(profiles.get(model) or profiles.get("default") or {})
        info = native_model_info(args.base_url, model)
        print(
            f"# {model}: quant={info.get('quantization', '?')} ctx={info.get('loaded_context_length', '?')} "
            f"| temp={cfg['temperature']} top_p={cfg['top_p']} no_think={cfg['no_think']} max_steps={cfg['max_steps']}"
        )
        for on in skills:
            print(f"# model={model} skills={'on' if on else 'off'}")
            for task in tasks:
                for r in range(args.repeats):
                    run_task(args, model, on, task, r, info, cfg)

    if args.smoke:
        ok = [smoke_verdict(args.out, m, tasks) for m in models]
        return 0 if all(ok) else 1
    print(f"\nruns in {args.out}")
    print(f"look:    python3 scripts/eval/report.py {args.out} --no-publish")
    print(
        f"publish: python3 scripts/eval/report.py {args.out} --quant <Q> --runtime llama.cpp"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
