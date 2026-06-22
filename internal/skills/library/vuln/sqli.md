---
name: sqli
description: Find and confirm SQL injection, then report with proof.
---

# SQL injection

## Find candidates

- Map endpoints and parameters first: `katana` to crawl, `arjun` to discover
  hidden GET/POST params on each interesting endpoint.
- Any parameter that reaches a query is a candidate: ids, filters, search,
  sort, auth fields.

## Confirm

- Run `sqlmap` against a specific URL/parameter. Start safe and non-interactive:
  `sqlmap` with `options="--batch --level=2 --risk=1 -p <param>"`.
- For POST/JSON bodies pass `--data` / `--json` via `options`; for auth'd areas
  pass `--cookie` or `--headers`.
- Escalate only as needed: raise `--level`/`--risk`, add `--dbs`, `--tables`,
  `--dump` once injection is proven. Dumping data is high-impact — stay in scope.

## Manual checks (when sqlmap is blocked)

- Use `run_shell` with curl to test classic markers: a single quote causing a
  500/SQL error, boolean pairs (`' AND '1'='1` vs `' AND '1'='2`) changing the
  response, and time-based payloads (`'; SELECT pg_sleep(5)--`) changing latency.

## Report

- `report_finding` with severity (usually high/critical), `target` = the exact
  URL+param, `evidence` = the sqlmap line proving injection (DBMS, technique) or
  the differential responses, and a recommendation: parameterized queries /
  prepared statements, least-privilege DB user.
- CWE-89. Provide a CVSS vector if you can.
