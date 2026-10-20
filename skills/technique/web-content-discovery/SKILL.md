---
name: web-content-discovery
description: Guides finding hidden paths, endpoints and parameters on a web app — crawling with katana, brute forcing with ffuf, feroxbuster or gobuster, parameter discovery with arjun, nikto known-issue scans, and CMS-aware scanning (wpscan, cmseek). Use when the user asks to enumerate directories or files, find API endpoints or Swagger specs, discover hidden parameters, or scan a WordPress or other CMS site.
license: Apache-2.0
metadata:
  verified: "2026-10-20"
---

# Web content & endpoint discovery

## Crawl what's linked

- `katana` — crawl the site for links, forms, and JS-embedded endpoints
  (`js_crawl` on). Bound depth on big sites. This finds the "known" surface.

## Brute force the unknown

- `feroxbuster` — fast recursive directory/content discovery (preferred default).
- `ffuf` — fuzz with the `FUZZ` keyword anywhere in the URL (paths, vhosts,
  extensions): `url="https://host/FUZZ"`. Add `-e .php,.txt,.bak` via `options`
  (in ffuf `-x` is the proxy flag).
- `gobuster_dir` — simple directory brute force alternative.
- Pick a wordlist appropriate to the stack; raise extensions for the tech
  `whatweb` reported. Watch for backup/config files (`.git`, `.env`, `.bak`,
  `swagger`/`openapi`, `/api`).

## Find hidden parameters

- `arjun` on interesting endpoints to discover undocumented GET/POST/JSON params
  — the inputs scanners miss and where injection often hides.
- `paramspider` mines parameter names from archived URLs (passive) — a fast seed
  list to feed `arjun` or the vuln-class tools.

## Known-issue scan

- `nikto` — quick scan for known web-server misconfigurations, dangerous/default
  files, and outdated software. Noisy; run once for the low-hanging fruit.

## CMS-aware

- `whatweb`/`cmseek` to detect the CMS; `wpscan` for WordPress (plugins/users).

## Hand-off

- Feed discovered endpoints+params into the relevant vuln-class skill. Stage big
  URL lists with `write_file` and pipe them into the next tool.
