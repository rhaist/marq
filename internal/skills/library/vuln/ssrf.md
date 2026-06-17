---
name: ssrf
description: Find server-side request forgery and prove out-of-band reach.
---

# Server-side request forgery (SSRF)

## Find candidates
- Any parameter that takes a URL, host, or file ref the server fetches:
  webhooks, "import from URL", PDF/image renderers, link previews, proxies,
  avatar-by-URL, XML/SVG processors. `arjun` + `katana` to surface params.

## Confirm (out-of-band is the proof)
- Stand up a listener you control and point the parameter at it; a callback
  proves the server made the request. Use `run_shell` to start a quick listener
  (e.g. `python3 -m http.server` on an authorized host) or an interactsh-style
  collaborator, then watch for the hit.
- Probe internal targets: `http://127.0.0.1:<port>`, `http://169.254.169.254/`
  (cloud metadata), internal hostnames. Compare responses/latency for blind SSRF.

## Escalate (carefully, in scope)
- Cloud metadata (IMDSv1) → credentials. Internal admin panels, Redis/Elastic on
  localhost. Note gopher/file/dict schemes if the fetcher allows them.

## Report
- `report_finding`: severity often high/critical (metadata creds = critical),
  `target` = endpoint+param, `evidence` = the OOB callback log or the internal
  response body returned. Recommend allowlist of destinations, block link-local
  and private ranges, disable unused URL schemes, IMDSv2.
- CWE-918.
