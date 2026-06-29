#!/usr/bin/env python3
"""Smoke-test every tool exposed by the marq server.

Spawns a *fresh* MCP server (`docker run --rm -i <image>`) — it does NOT touch a
container already bound to LM Studio — speaks the MCP stdio protocol, lists the
tools the server actually registered, and calls each one with benign arguments
to confirm the wrapper runs and its underlying binary is present.

What "works" means here: the wrapper executed and the binary was found. A tool
that runs but exits non-zero because it couldn't connect to localhost still
PASSES the binary check — we're testing the plumbing, not finding vulns.

Safety
------
By default every call targets only loopback (127.0.0.1), the reserved
`example.com`, or files staged locally in the container's /work. Tools that
reach third-party services (subdomain/OSINT/people/archive lookups) are skipped
unless you pass --external. Nothing scans a target you didn't authorise.

The test container is launched with a short MARQ_TIMEOUT so slow tools
return a "[timed out]" envelope instead of hanging (which keeps the JSON-RPC
stream in sync). API keys present in your environment are forwarded so key-gated
tools (shodan, …) can be exercised too.

Usage
-----
    python3 scripts/test_tools.py                  # local + passive-safe tools
    python3 scripts/test_tools.py --external       # also hit internet OSINT
    python3 scripts/test_tools.py --slow           # also run the slow crackers
    python3 scripts/test_tools.py --only nmap,dnsx # a subset
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
LOOPBACK = "127.0.0.1"
LOOPBACK_URL = "http://127.0.0.1/"
EX_DOMAIN = "example.com"
STAGE = "/work/smoke"  # created in-container via write_file before file tools

# Categories drive which tools run in which mode.
LOCAL = "local"        # loopback / staged / local DB — always run
EXTERNAL = "external"  # reaches third-party services — needs --external
SLOW = "slow"          # heavy crackers — needs --slow
KEY = "key"            # needs an API key; run if reachable, "needs key" is fine

# tool name -> (mode, arguments dict). Tools the server registers but that are
# missing here are reported as UNTESTED (and fail the run) so coverage stays honest.
TESTS: dict[str, tuple[str, dict]] = {
    # meta
    "server_info":   (LOCAL, {}),
    "run_shell":     (LOCAL, {"command": "echo SMOKE_OK && command -v nmap && command -v nuclei"}),
    # files (staging happens first, see stage_files)
    "write_file":    (LOCAL, {"path": f"{STAGE}/ping.txt", "content": "smoke"}),
    "read_file":     (LOCAL, {"path": f"{STAGE}/ping.txt"}),
    "list_dir":      (LOCAL, {"path": STAGE}),
    # recon / network — loopback
    "nmap":          (LOCAL, {"target": LOOPBACK, "options": "-p 22,80 -T4 -Pn"}),
    "masscan":       (LOCAL, {"target": LOOPBACK, "ports": "80", "rate": 100}),
    "naabu":         (LOCAL, {"target": LOOPBACK, "ports": "80"}),
    "dns_lookup":    (LOCAL, {"domain": EX_DOMAIN, "record_type": "A"}),
    "whois_lookup":  (LOCAL, {"target": EX_DOMAIN}),
    "dnsx":          (LOCAL, {"hosts": EX_DOMAIN, "records": "a"}),
    "dnsrecon":      (LOCAL, {"domain": EX_DOMAIN, "scan_type": "std"}),
    "httpx_probe":   (LOCAL, {"targets": LOOPBACK}),
    "ssh_audit":      (LOCAL, {"host": LOOPBACK, "port": 22}),
    "fping_sweep":    (LOCAL, {"target": "127.0.0.1/30"}),
    "nbtscan":        (LOCAL, {"target": LOOPBACK}),
    "snmp_check":     (LOCAL, {"host": LOOPBACK, "community": "public"}),
    "snmp_brute":     (SLOW,  {"host": LOOPBACK, "wordlist": f"{STAGE}/snmp_comm.txt"}),
    "snmp_walk":      (LOCAL, {"host": LOOPBACK, "community": "public"}),
    "smtp_user_enum": (LOCAL, {"host": LOOPBACK, "method": "VRFY", "users": f"{STAGE}/users.txt"}),
    "smtp_test":      (LOCAL, {"target": "127.0.0.1:25"}),
    "asnmap":         (EXTERNAL, {"target": "1.1.1.1"}),
    "cdncheck":       (LOCAL, {"targets": "127.0.0.1"}),
    "censys_search":  (KEY, {"query": "8.8.8.8", "index": "hosts"}),
    # web — loopback (connection refused is fine; the binary ran)
    "nuclei":        (LOCAL, {"target": LOOPBACK_URL, "severity": "info"}),
    "nikto":         (LOCAL, {"target": LOOPBACK_URL}),
    "ffuf":          (LOCAL, {"url": "http://127.0.0.1/FUZZ"}),
    "gobuster_dir":  (LOCAL, {"url": LOOPBACK_URL}),
    "feroxbuster":   (LOCAL, {"url": LOOPBACK_URL}),
    "whatweb":       (LOCAL, {"target": LOOPBACK_URL}),
    "wafw00f":       (LOCAL, {"target": LOOPBACK_URL}),
    "cmseek":        (LOCAL, {"url": LOOPBACK_URL}),
    "testssl":       (LOCAL, {"host": "127.0.0.1:443"}),
    "katana":        (LOCAL, {"url": LOOPBACK_URL}),
    "arjun":         (LOCAL, {"url": LOOPBACK_URL}),
    "dalfox":        (LOCAL, {"url": "http://127.0.0.1/?q=1"}),
    "sqlmap":        (LOCAL, {"url": "http://127.0.0.1/?id=1", "options": "--batch --flush-session --disable-coloring"}),
    "wpscan":        (LOCAL, {"url": LOOPBACK_URL}),
    "subjack":        (LOCAL, {"target": "http://127.0.0.1", "options": "-t 1"}),
    "paramspider":    (EXTERNAL, {"domain": EX_DOMAIN}),
    "sstimap":        (LOCAL, {"url": LOOPBACK_URL}),
    "donut":          (LOCAL, {"input": f"{STAGE}/donut_test.py", "output": f"{STAGE}/donut.bin"}),
    # exploitation / creds — local DBs / staged files
    "searchsploit":  (LOCAL, {"query": "apache 2.4"}),
    "impacket_secretsdump": (LOCAL, {"target": f"{STAGE}/ntds.txt"}),
    "impacket_kerberoast":  (KEY, {"target": "test/test:test@127.0.0.1"}),
    "impacket_asreproast":  (KEY, {"target": "test/test:test@127.0.0.1"}),
    "impacket_psexec":      (KEY, {"target": "test/test:test@127.0.0.1", "command": "whoami"}),
    "impacket_wmiexec":     (KEY, {"target": "test/test:test@127.0.0.1", "command": "whoami"}),
    "impacket_ntlmrelayx":  (LOCAL, {"options": ""}),
    "netexec":              (LOCAL, {"protocol": "smb", "target": LOOPBACK}),
    "certipy_find":         (KEY, {"target": "test/test:test@127.0.0.1"}),
    "bloodhound_collect":   (KEY, {"target": "test/test:test@127.0.0.1"}),
    "evil_winrm":           (KEY, {"target": "127.0.0.1:5985", "options": "-u test -p test"}),
    "enum4linux":           (LOCAL, {"target": LOOPBACK}),
    "smb_enum":             (LOCAL, {"target": LOOPBACK}),
    "ldap_search":          (LOCAL, {"target": "ldap://127.0.0.1", "filter": "(objectclass=*)"}),
    "hash_identify": (LOCAL, {"hash_value": "5f4dcc3b5aa765d61d8327deb882cf99"}),
    "exif_metadata": (LOCAL, {"path": f"{STAGE}/meta.txt"}),
    "gitleaks":      (LOCAL, {"path": STAGE}),
    "john":          (SLOW,  {"hash_file": f"{STAGE}/hash_md5.txt",
                              "options": "--format=raw-md5 --wordlist=" + f"{STAGE}/wordlist.txt"}),
    "hashcat":       (SLOW,  {"hash_file": f"{STAGE}/hash_md5.txt", "mode": 0,
                              "wordlist": f"{STAGE}/wordlist.txt", "options": "--potfile-disable -D 1"}),
    "hydra":         (SLOW,  {"target": LOOPBACK, "service": "ssh",
                              "options": "-l root -p root -t 1 -f"}),
    # osint — these reach the internet, gated behind --external
    "subfinder":     (EXTERNAL, {"domain": EX_DOMAIN}),
    "theharvester":  (EXTERNAL, {"domain": EX_DOMAIN, "limit": 50}),
    "gau_urls":      (EXTERNAL, {"domain": EX_DOMAIN}),
    "wayback_urls":  (EXTERNAL, {"domain": EX_DOMAIN}),
    "spiderfoot":    (EXTERNAL, {"target": EX_DOMAIN, "use_case": "passive"}),
    "sherlock":      (EXTERNAL, {"username": "octocat", "timeout": 15}),
    "maigret_username": (EXTERNAL, {"username": "octocat", "timeout": 15, "top_sites": 20}),
    "holehe_email":  (EXTERNAL, {"email": "test@example.com"}),
    "phoneinfoga":   (EXTERNAL, {"number": "+12025550123"}),
    "trufflehog":    (EXTERNAL, {"source": "git https://github.com/trufflesecurity/test_keys"}),
    # key-gated — run if reachable; "needs key" is an acceptable outcome
    "h8mail_breach": (KEY, {"email": "test@example.com"}),
    "shodan_host":   (KEY, {"ip": "1.1.1.1"}),
    "shodan_search": (KEY, {"query": "org:\"Example\"", "limit": 5}),
}

FORWARD_KEYS = ["SHODAN_API_KEY", "CENSYS_API_ID", "CENSYS_API_SECRET", "GITHUB_TOKEN", "NUMVERIFY_API_KEY"]


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
    staged = {
        f"{STAGE}/hash_md5.txt": "5f4dcc3b5aa765d61d8327deb882cf99\n",  # md5('password')
        f"{STAGE}/wordlist.txt": "password\n123456\nletmein\n",
        f"{STAGE}/meta.txt": "smoke-test metadata sample\n",
        f"{STAGE}/users.txt": "root\nadmin\ntest\n",
        f"{STAGE}/snmp_comm.txt": "public\nprivate\ncommunity\n",
        f"{STAGE}/donut_test.py": "import socket\nprint('donut smoke test')\n",
        f"{STAGE}/ntds.txt": ":::\n",
    }
    for path, content in staged.items():
        mcp.call("tools/call", {"name": "write_file", "arguments": {"path": path, "content": content}}, timeout)


def main() -> int:
    ap = argparse.ArgumentParser(description="Smoke-test every marq tool.")
    ap.add_argument("--image", default="marq", help="docker image to test (default: marq)")
    ap.add_argument("--timeout", type=int, default=25, help="server per-command timeout, seconds (default 25)")
    ap.add_argument("--external", action="store_true", help="also test tools that reach the internet")
    ap.add_argument("--slow", action="store_true", help="also test the slow crackers/brute tools")
    ap.add_argument("--only", default="", help="comma-separated subset of tool names to test")
    ap.add_argument("--json", action="store_true", help="emit machine-readable JSON results")
    args = ap.parse_args()

    only = {s.strip() for s in args.only.split(",") if s.strip()}
    read_timeout = args.timeout + 20

    docker_cmd = [
        "docker", "run", "--rm", "-i",
        # nmap/masscan/naabu carry cap_net_admin file-caps and won't exec without it.
        "--cap-add", "NET_RAW", "--cap-add", "NET_ADMIN", "--cap-add", "NET_BIND_SERVICE",
        "-e", f"MARQ_TIMEOUT={args.timeout}",
        "-e", "MARQ_OPERATOR=smoke-test",
        "-e", "MARQ_ENGAGEMENT=tool-smoke-test",
        "-e", "MARQ_SCOPE=loopback + example.com + staged files (smoke test)",
        "-e", "MARQ_ALLOW_RAW_SHELL=true",
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
            if mode == SLOW and not args.slow:
                results.append({"tool": name, "status": "SKIP", "note": "needs --slow"})
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
