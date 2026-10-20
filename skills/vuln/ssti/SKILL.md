---
name: ssti
description: Guides finding server-side template injection and confirming code execution — locating reflected inputs, math-probe confirmation, engine identification, sstimap, and careful in-scope escalation. Use when the user suspects template injection (Jinja2, Twig, Freemarker and others), sees {{7*7}} evaluate, or tests template-rendered inputs on an authorized target.
license: Apache-2.0
metadata:
  verified: "2026-10-20"
---

# Server-side template injection (SSTI)

## Find candidates

- Any input reflected into a server-rendered template: search results, error
  messages, name/profile fields, email/notification templates, filenames, custom
  themes. `arjun` / `katana` to surface params that echo back.

## Confirm

- `sstimap` is the workhorse: point it at a URL with a parameter — it detects the
  engine (Jinja2, Twig, Freemarker, Velocity, ERB, …) and escalates to code
  execution when possible, so a positive is high-confidence.
- Manual marker first if you prefer: with `run_shell` + curl, send a math probe
  (`${7*7}`, `{{7*7}}`, `<%= 7*7 %>`) and look for `49` in the response — that
  separates SSTI from plain reflected XSS, and which payload renders tells you the
  engine.

## Escalate (carefully, in scope)

- Engine-specific RCE gadgets (e.g. Jinja2 `{{ ''.__class__... }}`, Freemarker
  `freemarker.template.utility.Execute`). `sstimap` with `options="--os-cmd 'id'"`
  runs one command once confirmed (`--os-shell` is interactive and hangs here).
  Stay within authorized scope.

## Report

- `report_finding`: severity usually high/critical (RCE), `target` = URL+param,
  `evidence` = the rendered math result and/or the command output. Recommend
  sandboxed/logic-less templates, no user input as template source, allowlist.
- CWE-1336 (SSTI) / CWE-94 (code injection); OWASP A05:2025 Injection.
- To map this finding to IEC 62443 / CRA / NIS2 obligations, `load_skill finding-control-crosswalk`.
