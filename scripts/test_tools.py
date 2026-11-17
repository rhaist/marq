#!/usr/bin/env python3
"""Smoke-test every tool exposed by the marq server.

Spawns a *fresh* MCP server (`docker run --rm -i <image>`) — it does NOT touch a
long-lived marq container in use by a client — speaks the MCP stdio protocol, lists the
tools the server actually registered, and calls each one with benign arguments
to confirm the wrapper runs and its underlying binary is present.

What "works" means here: the wrapper executed and the binary was found. A tool
that runs but exits non-zero because it couldn't connect to localhost still
PASSES the binary check — we're testing the plumbing, not finding vulns.

Safety
------
marq is passive OSINT only: calls query public data about the reserved
`example.com`, a well-known public IP, or files staged in /work. Tools that
reach third-party services are skipped unless you pass --external.

The test container is launched with a short MARQ_TIMEOUT so slow tools
return a "[timed out]" envelope instead of hanging (which keeps the JSON-RPC
stream in sync). API keys present in your environment are forwarded so key-gated
tools (shodan, …) can be exercised too.

Usage
-----
    python3 scripts/test_tools.py                  # local + passive-safe tools
    python3 scripts/test_tools.py --external       # also hit internet OSINT
        python3 scripts/test_tools.py --only dnsx,whois_lookup # a subset
    python3 scripts/test_tools.py --image marq --timeout 25

Exit code is non-zero if any tool FAILs the binary check or a tool the server
registered has no test defined (so new tools don't silently go untested).
"""
from __future__ import annotations

import argparse
import json
import os
import select
import subprocess
import sys
import time

# --- result staging targets (all benign) -----------------------------------
EX_DOMAIN = "example.com"
STAGE = "/work/smoke"  # created in-container via write_file before file tools

# Categories drive which tools run in which mode.
LOCAL = "local"        # loopback / staged / local DB — always run
EXTERNAL = "external"  # reaches third-party services — needs --external
KEY = "key"            # needs an API key; run if reachable, "needs key" is fine

# tool name -> (mode, arguments dict). Tools the server registers but that are
# missing here are reported as UNTESTED (and fail the run) so coverage stays honest.
TESTS: dict[str, tuple[str, dict]] = {
    # meta
    "server_info":   (LOCAL, {}),
    # files (staging happens first, see stage_files)
    "write_file":    (LOCAL, {"path": f"{STAGE}/ping.txt", "content": "smoke"}),
    "read_file":     (LOCAL, {"path": f"{STAGE}/ping.txt"}),
    "list_dir":      (LOCAL, {"path": STAGE}),
    # DNS / registration — resolver and WHOIS only
    "dns_lookup":    (LOCAL, {"domain": EX_DOMAIN, "record_type": "A"}),
    "whois_lookup":  (LOCAL, {"target": EX_DOMAIN}),
    "dnsx":          (LOCAL, {"hosts": EX_DOMAIN, "records": "a"}),
    "cdncheck":      (LOCAL, {"targets": "1.1.1.1"}),
    "asnmap":        (EXTERNAL, {"target": "1.1.1.1"}),
    "ct_search":     (EXTERNAL, {"domain": EX_DOMAIN}),
    "rdap":          (EXTERNAL, {"query": EX_DOMAIN}),
    "dnstwist":      (EXTERNAL, {"domain": EX_DOMAIN}),
    "ioc_lookup":    (EXTERNAL, {"indicator": "d41d8cd98f00b204e9800998ecf8427e"}),
    "censys_view":   (KEY, {"assets": "8.8.8.8"}),
    # in-process: engagement, knowledge, findings, jobs
    "set_engagement": (LOCAL, {"engagement": "smoke-test: tool plumbing check"}),
    "load_skill":    (LOCAL, {}),
    "report_finding": (LOCAL, {"title": "smoke", "severity": "info", "target": EX_DOMAIN}),
    "render_report": (LOCAL, {}),
    "list_jobs":     (LOCAL, {}),
    "job_status":    (LOCAL, {"job": "no-such-job"}),
    "exif_metadata": (LOCAL, {"path": f"{STAGE}/meta.txt"}),
    # osint — these reach the internet, gated behind --external
    "subfinder":     (EXTERNAL, {"domain": EX_DOMAIN}),
    "theharvester":  (EXTERNAL, {"domain": EX_DOMAIN, "limit": 50}),
    "gau_urls":      (EXTERNAL, {"domain": EX_DOMAIN}),
    "spiderfoot":    (EXTERNAL, {"target": EX_DOMAIN}),
    "sherlock":      (EXTERNAL, {"username": "octocat", "timeout": 15}),
    "maigret_username": (EXTERNAL, {"username": "octocat", "timeout": 15, "top_sites": 20}),
    "email_accounts": (EXTERNAL, {"email": "test@example.com"}),
    "phoneinfoga":   (EXTERNAL, {"number": "+12025550123"}),
    "trufflehog":    (EXTERNAL, {"source": "git https://github.com/trufflesecurity/test_keys"}),
    # key-gated — run if reachable; "needs key" is an acceptable outcome
    "h8mail_breach": (KEY, {"email": "test@example.com"}),
    "shodan_host":   (EXTERNAL, {"ip": "1.1.1.1"}),
    "shodan_search": (KEY, {"query": "org:\"Example\"", "limit": 5}),
}

FORWARD_KEYS = ["SHODAN_API_KEY", "CENSYS_API_TOKEN", "CENSYS_ORG_ID", "GITHUB_TOKEN", "NUMVERIFY_API_KEY"]


class MCP:
    """A minimal newline-delimited JSON-RPC MCP client over a subprocess."""

    def __init__(self, proc: subprocess.Popen):
        self.proc = proc
        self._id = 0

    def _send(self, method: str, params: dict | None = None, *, notify: bool = False) -> int | None:
        msg = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            msg["params"] = params
        if not notify:
            self._id += 1
            msg["id"] = self._id
        self.proc.stdin.write(json.dumps(msg) + "\n")
        self.proc.stdin.flush()
        return None if notify else self._id

    def _recv(self, want_id: int, timeout: float) -> dict:
        """Read JSON lines until the response with `want_id` arrives or timeout."""
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
                raise EOFError("server closed the stream")
            line = line.strip()
            if not line:
                continue
            try:
                obj = json.loads(line)
            except json.JSONDecodeError:
                continue  # stray log line on stdout — ignore
            if obj.get("id") == want_id:
                return obj

    def call(self, method: str, params: dict, timeout: float) -> dict:
        rid = self._send(method, params)
        return self._recv(rid, timeout)

    def initialize(self) -> dict:
        resp = self.call("initialize", {
            "protocolVersion": "2025-11-25",
            "capabilities": {},
            "clientInfo": {"name": "marq-smoke", "version": "1.0"},
        }, timeout=30)
        self._send("notifications/initialized", {}, notify=True)
        return resp


def render_result(resp: dict) -> tuple[str, bool]:
    """Extract the text body and the MCP-level isError flag from a tools/call result."""
    if "error" in resp:
        return json.dumps(resp["error"]), True
    result = resp.get("result", {})
    is_err = bool(result.get("isError"))
    sc = result.get("structuredContent")
    if isinstance(sc, dict) and "result" in sc:
        return str(sc["result"]), is_err
    parts = [c.get("text", "") for c in result.get("content", []) if c.get("type") == "text"]
    return "\n".join(parts), is_err


def classify(text: str, is_err: bool) -> tuple[str, str]:
    """Map a rendered tool result to (status, note)."""
    low = text.lower()
    if "binary not found in image" in low:
        return "FAIL", "missing binary"
    if is_err:
        return "ERROR", text.strip().splitlines()[0][:80] if text.strip() else "wrapper raised"
    if "launched in the background" in low:
        return "PASS", "backgrounded"
    if "[timed out" in low:
        return "TIMEOUT", "ran but exceeded the test timeout"
    # parse "[exit code: N, Xs]"
    code = None
    for tok in text.split("[exit code:")[1:2]:
        try:
            code = int(tok.split(",")[0].strip())
        except (ValueError, IndexError):
            pass
    if code == 0:
        return "PASS", "exit 0"
    if code is not None:
        return "PASS", f"ran (exit {code})"
    return "PASS", "ran"  # no parseable code but produced output and no error


def stage_files(mcp: MCP, timeout: float) -> None:
    """Create the benign input files the file-driven tools need."""
    staged = {f"{STAGE}/meta.txt": "smoke-test metadata sample\n"}
    for path, content in staged.items():
        mcp.call("tools/call", {"name": "write_file", "arguments": {"path": path, "content": content}}, timeout)


def main() -> int:
    ap = argparse.ArgumentParser(description="Smoke-test every marq tool.")
    ap.add_argument("--image", default="marq", help="docker image to test (default: marq)")
    ap.add_argument("--timeout", type=int, default=25, help="server per-command timeout, seconds (default 25)")
    ap.add_argument("--external", action="store_true", help="also test tools that reach the internet")
    ap.add_argument("--only", default="", help="comma-separated subset of tool names to test")
    ap.add_argument("--json", action="store_true", help="emit machine-readable JSON results")
    args = ap.parse_args()

    only = {s.strip() for s in args.only.split(",") if s.strip()}
    read_timeout = args.timeout + 20

    docker_cmd = [
        "docker", "run", "--rm", "-i",
        "-e", f"MARQ_TIMEOUT={args.timeout}",
        "-e", "MARQ_OPERATOR=smoke-test",
        "-e", "MARQ_ENGAGEMENT=tool-smoke-test",
    ]
    for k in FORWARD_KEYS:
        if os.environ.get(k):
            docker_cmd += ["-e", k]
    docker_cmd.append(args.image)

    print(f"# launching: {' '.join(docker_cmd)}", file=sys.stderr)
    proc = subprocess.Popen(
        docker_cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL, text=True, bufsize=1, env={**os.environ},
    )
    mcp = MCP(proc)
    results: list[dict] = []
    try:
        init = mcp.initialize()
        srv = init.get("result", {}).get("serverInfo", {})
        print(f"# connected: {srv.get('name')} v{srv.get('version')}", file=sys.stderr)

        listed = mcp.call("tools/list", {}, timeout=30)
        tools = sorted(t["name"] for t in listed.get("result", {}).get("tools", []))
        print(f"# server registered {len(tools)} tools\n", file=sys.stderr)

        stage_files(mcp, read_timeout)

        for name in tools:
            if only and name not in only:
                continue
            if name not in TESTS:
                results.append({"tool": name, "status": "UNTESTED", "note": "no test args defined"})
                continue
            mode, tool_args = TESTS[name]
            if mode == EXTERNAL and not args.external:
                results.append({"tool": name, "status": "SKIP", "note": "needs --external"})
                continue
            t0 = time.monotonic()
            try:
                resp = mcp.call("tools/call", {"name": name, "arguments": tool_args}, read_timeout)
                text, is_err = render_result(resp)
                status, note = classify(text, is_err)
            except (TimeoutError, EOFError) as exc:
                status, note = "HANG", str(exc)
            dt = time.monotonic() - t0
            results.append({"tool": name, "status": status, "note": note, "secs": round(dt, 1)})
            print(f"  {status:8} {name:18} {note} ({dt:.1f}s)", file=sys.stderr)
    finally:
        try:
            proc.stdin.close()
        except Exception:
            pass
        proc.terminate()
        try:
            proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            proc.kill()

    # --- summary ---
    counts: dict[str, int] = {}
    for r in results:
        counts[r["status"]] = counts.get(r["status"], 0) + 1
    print("\n" + "=" * 60, file=sys.stderr)
    print("SUMMARY: " + "  ".join(f"{k}={v}" for k, v in sorted(counts.items())), file=sys.stderr)
    bad = [r for r in results if r["status"] in ("FAIL", "ERROR", "UNTESTED", "HANG")]
    if bad:
        print("\nNeeds attention:", file=sys.stderr)
        for r in bad:
            print(f"  {r['status']:8} {r['tool']:18} {r['note']}", file=sys.stderr)

    if args.json:
        print(json.dumps({"results": results, "counts": counts}, indent=2))

    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
