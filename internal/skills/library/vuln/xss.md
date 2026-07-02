---
name: xss
description: Find reflected/stored/DOM XSS and confirm execution.
---

# Cross-site scripting (XSS)

**Fast path:** if you already have a URL with a parameter, run `dalfox` on it
directly — it tests and verifies execution. Only crawl first (below) when you
still need to discover endpoints/params.

## Find candidates

- `katana` to crawl endpoints and pull parameters; `arjun` to find hidden params.
- Anything reflected into HTML, attributes, JS context, or stored and rendered
  later is a candidate.

## Confirm

- `dalfox` is the workhorse: run it on a URL with a parameter. It tests
  reflected/stored/DOM vectors and verifies execution, so a positive is
  high-confidence. Pass headers/cookies via `options` for auth'd pages
  (e.g. `options="--cookie 'session=...'"`); add `--deep-domxss` for SPA/DOM sinks.
- Triage context: HTML body vs attribute vs JS string vs URL — the working
  payload and the fix differ by context.

## Manual checks

- `run_shell` + curl to see how a marker (e.g. `xss7331`) is reflected, then
  craft a context-appropriate payload (`"><svg onload=...>`, `';alert(1)//`).
- DOM XSS: review the JS `katana` found for sinks (`innerHTML`, `document.write`,
  `eval`, `location`) fed by `location`/`postMessage`/storage.

## Report

- `report_finding`: severity by impact (stored > reflected; admin context is
  worse), `target` = URL+param+context, `evidence` = the dalfox PoC or the
  reflected payload that executed. Recommend context-aware output encoding + CSP.
- CWE-79; OWASP A05:2025 Injection (XSS stays under Injection, now ranked A05).
