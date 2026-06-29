#!/usr/bin/env python3
"""marq eval harness — drive marq's MCP tools with an OpenAI-compatible model and
record the tool-call trajectory for scoring (scripts/eval/score.py).

Free + local by design: point --base-url at LM Studio (default :1234/v1), Ollama
(:11434/v1) or llama-server and sweep --models — nothing leaves the box and there
is no per-token cost. The skills on/off ablation measures the *lift* from marq's
knowledge layer (does loading skills make a small model pick the right tool?).

The harness owns the loop (model <-> marq MCP), so it sees every tool call the
model makes — including load_skill, which is in-process and never hits the audit
log. That trace.jsonl is the ground truth; /work artifacts are the cross-check.

    python3 scripts/eval/harness.py --models qwen2.5-7b-instruct,llama-3.1-8b \
        --skills both --tasks scripts/eval/tasks.jsonl --out scripts/eval/runs
    python3 scripts/eval/harness.py --list-tools        # smoke the marq MCP leg

Requires a built `marq` image (the default --marq-cmd docker-runs it). For a
quick local smoke without Kali: --marq-cmd 'go run ./cmd/marq serve' (in-process
tools only; exec tools like nmap won't be present).
"""
from __future__ import annotations
import argparse, json, os, select, shlex, subprocess, sys, time, urllib.request, pathlib

DEFAULT_MARQ_CMD = (
    "docker run --rm -i -v {work}:/work -e MARQ_OPERATOR=eval "
    "--cap-add NET_RAW --cap-add NET_ADMIN marq"
)


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
        resp = self.call("initialize", {
            "protocolVersion": "2025-11-25",
            "capabilities": {},
            "clientInfo": {"name": "marq-eval", "version": "1.0"},
        }, timeout=60)
        self._send("notifications/initialized", {}, notify=True)
        return resp.get("result", {})

    def list_tools(self):
        return self.call("tools/list", {}, timeout=60).get("result", {}).get("tools", [])

    def call_tool(self, name, args, timeout=300):
        resp = self.call("tools/call", {"name": name, "arguments": args}, timeout)
        result = resp.get("result", {})
        parts = [c.get("text", "") for c in result.get("content", []) if c.get("type") == "text"]
        return "\n".join(parts), bool(result.get("isError"))


def openai_tools(tools):
    return [{
        "type": "function",
        "function": {
            "name": t["name"],
            "description": t.get("description", ""),
            "parameters": t.get("inputSchema") or {"type": "object", "properties": {}},
        },
    } for t in tools]


def chat(base_url, api_key, model, messages, tools, timeout):
    body = {"model": model, "messages": messages, "temperature": 0.2, "stream": False}
    if tools:
        body["tools"] = tools
        body["tool_choice"] = "auto"
    req = urllib.request.Request(
        base_url.rstrip("/") + "/chat/completions",
        data=json.dumps(body).encode(),
        headers={"Content-Type": "application/json", "Authorization": f"Bearer {api_key}"},
    )
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.load(r)["choices"][0]["message"]


def spawn_marq(marq_cmd, work):
    env = dict(os.environ, MARQ_OPERATOR="eval", MARQ_WORK_DIR=str(work))
    return subprocess.Popen(
        shlex.split(marq_cmd.format(work=work)),
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        text=True, env=env,
    )


def run_task(args, model, skills_on, task, repeat):
    slug = model.replace("/", "_").replace(":", "_")
    rundir = (pathlib.Path(args.out)
              / f"{slug}__skills-{'on' if skills_on else 'off'}__{task['id']}__r{repeat}")
    work = rundir / "work"
    work.mkdir(parents=True, exist_ok=True)

    proc = spawn_marq(args.marq_cmd, work.resolve())
    try:
        mcp = MCP(proc)
        init = mcp.initialize()
        tools = mcp.list_tools()
        if not skills_on:
            tools = [t for t in tools if t["name"] != "load_skill"]
            system = ("You are a security operator with the following tools. Use them to "
                      "complete the task. Only act within authorized scope.")
        else:
            system = init.get("instructions") or "You are marq, a security assistant."

        messages = [{"role": "system", "content": system},
                    {"role": "user", "content": task["prompt"]}]
        tool_schemas = openai_tools(tools)
        trace = []
        final = ""
        for step in range(args.max_steps):
            try:
                msg = chat(args.base_url, args.api_key, model, messages, tool_schemas, args.timeout)
            except Exception as e:  # endpoint down / model missing — record and stop
                final = f"[harness error: {e}]"
                break
            messages.append({k: msg[k] for k in ("role", "content", "tool_calls") if msg.get(k) is not None})
            tcs = msg.get("tool_calls") or []
            if not tcs:
                final = msg.get("content") or ""
                break
            for tc in tcs:
                fn = tc.get("function", {})
                raw = fn.get("arguments") or "{}"
                a = raw if isinstance(raw, dict) else _loads(raw)
                out, is_err = mcp.call_tool(fn.get("name", ""), a)
                trace.append({"step": step, "name": fn.get("name", ""), "args": a,
                              "is_error": is_err, "result_head": out[:600]})
                messages.append({"role": "tool", "tool_call_id": tc.get("id", ""), "content": out})

        (rundir / "trace.jsonl").write_text("".join(json.dumps(t) + "\n" for t in trace))
        (rundir / "messages.json").write_text(json.dumps(messages, indent=2))
        (rundir / "meta.json").write_text(json.dumps({
            "model": model, "skills_on": skills_on, "task": task["id"], "repeat": repeat,
            "temperature": 0.2, "base_url": args.base_url,
            "steps": len(trace), "final": final,
        }, indent=2))
        print(f"  {rundir.name}: {len(trace)} tool calls")
        return rundir
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            proc.kill()


def _loads(s):
    try:
        return json.loads(s)
    except Exception:
        return {}


def main():
    p = argparse.ArgumentParser(description="marq eval harness")
    p.add_argument("--base-url", default="http://localhost:1234/v1", help="OpenAI-compatible endpoint")
    p.add_argument("--api-key", default="lm-studio", help="ignored by local servers; some require non-empty")
    p.add_argument("--models", default="", help="comma-separated model ids to sweep")
    p.add_argument("--skills", choices=["on", "off", "both"], default="both")
    p.add_argument("--tasks", default=str(pathlib.Path(__file__).with_name("tasks.jsonl")))
    p.add_argument("--out", default=str(pathlib.Path(__file__).with_name("runs")))
    p.add_argument("--marq-cmd", default=DEFAULT_MARQ_CMD, help="command to start `marq serve`; {work} is substituted")
    p.add_argument("--max-steps", type=int, default=12)
    p.add_argument("--repeats", type=int, default=1, help="runs per task (use 3+ for published numbers; LLMs are stochastic)")
    p.add_argument("--timeout", type=int, default=300, help="per model call (s)")
    p.add_argument("--list-tools", action="store_true", help="smoke the marq MCP leg and exit")
    args = p.parse_args()

    if args.list_tools:
        work = pathlib.Path(args.out) / "_smoke"
        work.mkdir(parents=True, exist_ok=True)
        proc = spawn_marq(args.marq_cmd, work.resolve())
        try:
            mcp = MCP(proc)
            init = mcp.initialize()
            tools = mcp.list_tools()
            print(f"server instructions: {len(init.get('instructions',''))} chars")
            print(f"tools: {len(tools)}")
            print("  " + ", ".join(t["name"] for t in tools[:12]) + " ...")
        finally:
            proc.terminate()
        return 0

    models = [m.strip() for m in args.models.split(",") if m.strip()]
    if not models:
        p.error("pass --models (comma-separated) or --list-tools")
    skills = [True, False] if args.skills == "both" else [args.skills == "on"]
    tasks = [json.loads(l) for l in open(args.tasks) if l.strip()]

    for model in models:
        for on in skills:
            print(f"# model={model} skills={'on' if on else 'off'}")
            for task in tasks:
                for r in range(args.repeats):
                    run_task(args, model, on, task, r)
    print(f"\nruns in {args.out}")
    print(f"look:    python3 scripts/eval/report.py {args.out} --no-publish")
    print(f"publish: python3 scripts/eval/report.py {args.out} --quant <Q> --runtime lm-studio")
    return 0


if __name__ == "__main__":
    sys.exit(main())
