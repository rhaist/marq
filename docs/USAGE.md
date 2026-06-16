# Usage

## 1. Build the image

```bash
docker build -t pentest-mcp .
```

This pulls the Kali base and installs the full tool suite, so it is large
(multi-GB) and the first build takes a while.

## 2. Smoke-test outside LM Studio

Run a one-off tool to confirm the image works:

```bash
docker run --rm pentest-mcp nmap --version
docker run --rm pentest-mcp searchsploit --help
```

Get an interactive shell:

```bash
docker run --rm -it --entrypoint /bin/bash pentest-mcp
```

Confirm the MCP server starts (it will wait for JSON-RPC on stdin; Ctrl-C to exit):

```bash
docker run --rm -i pentest-mcp        # prints the authorization banner to stderr
```

## 3. Wire it into LM Studio

1. In LM Studio open the MCP config (**Program → Edit `mcp.json`**, or the
   "Integrations" panel).
2. Merge the `pentest-mcp` entry from [`mcp.json.example`](../mcp.json.example)
   into your `mcpServers`.
3. Edit the `env` block — set `PENTEST_MCP_OPERATOR`, `PENTEST_MCP_ENGAGEMENT`
   and especially `PENTEST_MCP_SCOPE` to your authorized targets.
4. Save and toggle the server on. Load a tool-use-capable model.
5. Ask the model to call `server_info` first — it returns the authorization
   banner and confirms scope before any scanning.

## Available tools

| Tool            | Wraps         | Purpose                              |
|-----------------|---------------|--------------------------------------|
| `server_info`   | —             | Show authorization banner + scope    |
| `nmap`          | nmap          | Port/service scanning                |
| `masscan`       | masscan       | Fast port sweeps                     |
| `dns_lookup`    | dig           | DNS records                          |
| `whois_lookup`  | whois         | Registration data                    |
| `subfinder`     | subfinder     | Passive subdomain enum               |
| `httpx_probe`   | httpx         | Live HTTP probing / tech detect      |
| `nuclei`        | nuclei        | Template-based vuln scanning         |
| `nikto`         | nikto         | Web server scanning                  |
| `ffuf`          | ffuf          | Content/dir fuzzing                  |
| `gobuster_dir`  | gobuster      | Directory brute force                |
| `whatweb`       | whatweb       | Web tech fingerprinting              |
| `wpscan`        | wpscan        | WordPress scanning                   |
| `sqlmap`        | sqlmap        | SQL injection testing                |
| `searchsploit`  | exploitdb     | Local exploit DB search              |
| `msfconsole`    | metasploit    | Run a resource script                |
| `hydra`         | hydra         | Online credential testing            |
| `john`          | john          | Offline hash cracking                |
| `hashcat`       | hashcat       | GPU/CPU hash cracking                |
| `hash_identify` | hashid        | Identify hash type                   |
| `run_shell`     | bash          | Arbitrary command (opt-in)           |

## Environment variables

| Variable                      | Default                              | Meaning                                   |
|-------------------------------|--------------------------------------|-------------------------------------------|
| `PENTEST_MCP_OPERATOR`        | `unknown`                            | Recorded in every audit record            |
| `PENTEST_MCP_ENGAGEMENT`      | `unspecified`                        | Engagement / SOW identifier               |
| `PENTEST_MCP_SCOPE`           | `""`                                 | Free-text authorized scope (banner + log) |
| `PENTEST_MCP_AUDIT_LOG`       | `/var/log/pentest-mcp/audit.jsonl`   | Audit log path                            |
| `PENTEST_MCP_TIMEOUT`         | `900`                                | Per-command timeout (seconds)             |
| `PENTEST_MCP_MAX_OUTPUT`      | `60000`                              | Max output chars returned to the model    |
| `PENTEST_MCP_ALLOW_RAW_SHELL` | `false`                              | Expose the arbitrary-shell tool           |

## Reading the audit log

```bash
# Persisted to ./audit/ via the compose/volume mount.
cat audit/audit.jsonl | jq 'select(.event=="invocation.start") | {ts, tool, target, argv}'
```

## GPU hash cracking (optional)

`hashcat` benefits from a GPU. Pass it through at run time, e.g. with the
NVIDIA container toolkit: add `--gpus all` to the `docker run` args.
