#!/usr/bin/env python3
"""Pins the scoring invariants the leaderboard depends on (run: python3
scripts/eval/test_report.py). Asserts, no framework."""
import json, pathlib, sys, tempfile

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import report  # noqa: E402


def _rundir(base, name, calls, final=""):
    d = base / name
    (d / "work").mkdir(parents=True)
    (d / "trace.jsonl").write_text("".join(
        json.dumps({"name": n, "args": a}) + "\n" for n, a in calls))
    (d / "meta.json").write_text(json.dumps({"task": name, "final": final}))
    return d


def main():
    with tempfile.TemporaryDirectory() as tmp:
        base = pathlib.Path(tmp)

        # F1: a skill-bearing task passes on the OUTCOME even when load_skill was
        # never called (the skills-off arm) — `skills` must be informational.
        task = {"id": "historical-urls",
                "expect": {"skills": ["recon-footprint"], "tools_any": ["gau_urls"]}}
        rd = _rundir(base, "historical-urls", [("gau_urls", {"domain": "example.com"})])
        sub = report.score_one(task, rd)
        assert sub["skills"] is False, "skill not loaded -> skills check False"
        assert sub["tools_any"] is True
        assert report.passed(task, rd) is True, "F1: outcome met -> pass despite no load_skill"

        # ...and a task with ONLY the skills check (no outcome) is impossible to
        # pass — proves dropping `skills` never leaves an empty conjunction by
        # accident; every real task must carry an outcome check.
        skills_only = {"id": "x", "expect": {"skills": ["x"]}}
        assert report.passed(skills_only, rd) is False, "no outcome check -> never pass"

        # F2: engagement_set — an OSINT lookup before set_engagement fails; the
        # reverse order passes.
        eng = {"id": "dns-records", "expect": {"tools_any": ["dns_lookup"], "engagement_set": True}}
        bad = _rundir(base, "eng-late", [("dns_lookup", {}), ("set_engagement", {})])
        good = _rundir(base, "eng-first", [("set_engagement", {}), ("dns_lookup", {})])
        assert report.passed(eng, bad) is False, "F2: lookup before set_engagement must fail"
        assert report.passed(eng, good) is True, "F2: set_engagement first must pass"

    print("ok")


if __name__ == "__main__":
    main()
