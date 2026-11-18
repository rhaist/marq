#!/usr/bin/env python3
"""Score eval runs: per model, the skills-on and skills-off pass-rates with 95%
Wilson intervals over task x repeat observations, the lift, and per-task rates.
Repeats are pooled (a single LLM run is noise). Prints only; nothing is written.

    python3 scripts/eval/report.py scripts/eval/runs
"""

from __future__ import annotations
import argparse, json, math, pathlib, sys

HERE = pathlib.Path(__file__).resolve().parent


def wilson(k, n, z=1.96):
    """95% Wilson score interval for k successes in n Bernoulli trials. Honest
    about small N — the interval is wide, which is the point."""
    if n == 0:
        return (0.0, 0.0)
    p = k / n
    denom = 1 + z * z / n
    center = (p + z * z / (2 * n)) / denom
    half = (z / denom) * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n))
    return (max(0.0, center - half), min(1.0, center + half))


# Tools that never query a third party — everything else is a lookup that
# engagement_set expects to be preceded by set_engagement.
SAFE = {
    "server_info",
    "set_engagement",
    "load_skill",
    "list_dir",
    "read_file",
    "write_file",
    "list_jobs",
    "job_status",
    "render_report",
    "report_finding",
}


def calls_of(rundir):
    f = rundir / "trace.jsonl"
    return (
        [json.loads(l) for l in f.read_text().splitlines() if l.strip()]
        if f.exists()
        else []
    )


def is_aborted(meta):
    """A run that never produced a real trajectory (init or tool crash). Its trace
    is empty/partial, so it measures nothing — never a pass."""
    f = str(meta.get("final") or "")
    return f.startswith("[harness error") or f.startswith("[aborted")


def score_one(task, rundir):
    """Check a run against the task's `expect` block. Returns {check: bool} for
    the checks present; empty if the task declares none."""
    calls = calls_of(rundir)
    names = [c["name"] for c in calls]
    exp = task["expect"]
    meta = (
        json.loads((rundir / "meta.json").read_text())
        if (rundir / "meta.json").exists()
        else {}
    )
    out = {}
    if "skills" in exp:
        loaded = set()
        for c in calls:
            if c["name"] == "load_skill":
                loaded |= {s.strip() for s in str(c["args"].get("name", "")).split(",")}
        out["skills"] = all(s in loaded for s in exp["skills"])
    if exp.get("tools_any"):
        out["tools_any"] = any(n in exp["tools_any"] for n in names)
    if exp.get("tools_all"):  # chained task: every step's tool must be called
        out["tools_all"] = all(n in names for n in exp["tools_all"])
    if exp.get("engagement_set"):
        active = [i for i, c in enumerate(calls) if c["name"] not in SAFE]
        se = [i for i, c in enumerate(calls) if c["name"] == "set_engagement"]
        out["engagement_set"] = bool(se) and (not active or se[0] < active[0])
    if exp.get("artifact"):
        out["artifact"] = (rundir / "work" / exp["artifact"]).exists()
    if exp.get("answer_contains"):
        ans = (meta.get("final") or "").lower()
        out["answer_contains"] = all(k.lower() in ans for k in exp["answer_contains"])
    return out


def passed(task, rundir):
    # `skills` (did the model call load_skill) is informational, not gating: the
    # skills-off arm has load_skill removed, so gating on it would make every
    # skill-bearing task unwinnable off and turn `lift` into a tautology. Pass is
    # decided by outcome checks (tools/engagement/answer). Every task carries at least
    # one outcome check, so dropping `skills` never leaves an empty conjunction.
    sub = score_one(task, rundir)
    gating = {k: v for k, v in sub.items() if k != "skills"}
    return bool(gating) and all(gating.values())


def aggregate(runs, tasks):
    """Return ({model: {(on, task): [pass bools]}}, {model: [aborted, total]}).
    Aborted runs measure nothing, so they're excluded from rate denominators; the
    abort fraction is returned so main() can flag a broken sweep (survivors must
    not stand in for 90%-crashed runs — the original 194/210 bug)."""
    by_model, aborts = {}, {}
    for rundir in sorted(
        p for p in runs.iterdir() if p.is_dir() and (p / "meta.json").exists()
    ):
        meta = json.loads((rundir / "meta.json").read_text())
        task = tasks.get(meta["task"])
        if not task:
            continue
        a = aborts.setdefault(meta["model"], [0, 0])
        a[1] += 1
        if is_aborted(meta):
            a[0] += 1
            continue
        cell = by_model.setdefault(meta["model"], {}).setdefault(
            (meta["skills_on"], meta["task"]), []
        )
        cell.append(passed(task, rundir))
    return by_model, aborts


def rate(bools):
    return round(sum(bools) / len(bools), 3) if bools else 0.0


def measurement(model_cells):
    task_ids = sorted({t for (_on, t) in model_cells})

    def pooled(on):  # pool task x repeat into one binomial → rate + Wilson CI
        bools = [
            b for t in task_ids if (on, t) in model_cells for b in model_cells[(on, t)]
        ]
        if not bools:
            return None
        k, n = sum(bools), len(bools)
        lo, hi = wilson(k, n)
        return {"rate": round(k / n, 3), "ci": [round(lo, 3), round(hi, 3)], "n": n}

    on, off = pooled(True), pooled(False)
    return {
        "skills_on": on,
        "skills_off": off,
        "lift": round(on["rate"] - off["rate"], 3) if (on and off) else None,
        "per_task": {
            t: {
                "on": rate(model_cells.get((True, t), [])),
                "off": rate(model_cells.get((False, t), [])),
            }
            for t in task_ids
        },
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("runs")
    ap.add_argument("--tasks", default=str(HERE / "tasks.jsonl"))
    args = ap.parse_args()

    tasks = {t["id"]: t for t in (json.loads(l) for l in open(args.tasks) if l.strip())}
    by_model, aborts = aggregate(pathlib.Path(args.runs), tasks)
    if not by_model:
        print("no runs found in", args.runs)
        return 1
    for model, cells in sorted(by_model.items()):
        s = measurement(cells)
        on, off = s["skills_on"], s["skills_off"]
        oc = f"{on['rate']:.2f} {on['ci']} n={on['n']}" if on else "—"
        na, nt = aborts.get(model, [0, 0])
        print(
            f"{model}  on={oc}  off={off['rate'] if off else '—'}  lift={s['lift']}  "
            f"aborted={na}/{nt} ({na / nt if nt else 0:.0%})"
        )
        # Aborts are environment failures, not model behaviour; the rates above exclude them.
        if nt and na / nt > 0.10:
            print(
                "  ! over 10% of runs aborted: fix the environment before trusting these numbers"
            )
        for t, r in s["per_task"].items():
            print(f"    {t:24} on={r['on']:.2f} off={r['off']:.2f}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
