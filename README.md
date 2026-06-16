# pentest-mcp

A Dockerized **penetration-testing toolkit exposed as an MCP server** for use
inside **LM Studio** (or any MCP client). A tool-use-capable local model can
drive industry-standard offensive tools — recon, web testing, exploitation and
hash cracking — through a small, auditable Python MCP layer.

> ⚠️ **Authorized testing only.** This image contains active attack tooling.
> Read [`docs/SECURITY.md`](docs/SECURITY.md) first. You are responsible for
> having written authorization for every target you touch.

## What you get

- **Full tool suite** on a Kali base: `nmap`, `masscan`, `nuclei`, `httpx`,
  `subfinder`, `nikto`, `ffuf`, `gobuster`, `whatweb`, `wpscan`, `sqlmap`,
  `metasploit`, `hydra`, `searchsploit`, `john`, `hashcat`, `hashid`, …
- **MCP over stdio** (Python, official `mcp` SDK / FastMCP). No network port is
  opened — LM Studio launches the container and talks over stdin/stdout.
- **Audit logging on every invocation** — JSON-lines, append-only, with
  operator, engagement, target and full argument vector.
- **Hardened container** — non-root user, dropped capabilities (only the few
  needed for SYN scans), `no-new-privileges`, opt-in raw shell.

## Quick start

```bash
# 1. Build (large image — pulls Kali + full tool suite)
docker build -t pentest-mcp .

# 2. Sanity check
docker run --rm pentest-mcp nmap --version

# 3. Add to LM Studio's mcp.json (see mcp.json.example), set your scope/env,
#    enable the server, then ask the model to call `server_info` first.
```

Full instructions: [`docs/USAGE.md`](docs/USAGE.md).

## How it fits together

```
LM Studio  ──stdio JSON-RPC──▶  docker run -i pentest-mcp
                                   └─ python -m server.main   (FastMCP)
                                        ├─ tools/recon.py   nmap, masscan, dns…
                                        ├─ tools/web.py     nuclei, sqlmap, ffuf…
                                        ├─ tools/exploit.py msf, hydra, searchsploit
                                        ├─ tools/creds.py   john, hashcat, hashid
                                        └─ runner.py ──▶ audit.jsonl (every call)
```

Every tool call funnels through `server/runner.py`, which is the single point
where auditing happens (and where you'd add hard scope-enforcement if you want
to move beyond logging-only guardrails).

## Repository layout

```
Dockerfile            Kali-based full-suite image, hardened, non-root
docker-compose.yml    Build + interactive-shell convenience, hardening flags
mcp.json.example      Drop-in LM Studio MCP config
server/               The MCP server
  main.py             FastMCP entrypoint (stdio), tool registration, banner
  config.py           Env-driven configuration
  audit.py            Append-only JSON-lines audit log
  runner.py           Shared subprocess runner (audit + timeout + truncation)
  tools/              recon, web, exploit, creds, shell tool groups
docs/                 SECURITY.md, USAGE.md
```

## License

MIT — see [`LICENSE`](LICENSE). Provided for authorized security testing and
education. No warranty; use responsibly and legally.
