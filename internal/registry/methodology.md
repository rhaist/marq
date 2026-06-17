# Engagement methodology

Authorized testing only. Call `server_info` first and confirm the targets are in
scope. Work in phases; record results as you go with `report_finding`, then
`render_report` at the end.

## 1. Scope & recon (passive first)
- `server_info` — confirm operator, engagement and scope.
- Company/domain footprint: `theharvester`, `subfinder`, `dnsx`, `wayback_urls`,
  `gau_urls`, `whois_lookup`, `dns_lookup`. People: `sherlock`, `holehe_email`.
- Broad automated sweep: `spiderfoot` (runs in the background — poll with
  `job_status` / `list_jobs`, do not block waiting on it).

## 2. Enumerate (active, scope-sensitive)
- Hosts/ports: `naabu` or `masscan` to find open ports, then `nmap` `-sV` on the
  hits for service/version detail.
- Live web: `httpx_probe` to find live HTTP, `whatweb` / `wafw00f` to fingerprint,
  `katana` to crawl endpoints.

## 3. Test for vulnerabilities
- Web: `nuclei` (templates), `nikto`, content discovery (`ffuf` / `feroxbuster` /
  `gobuster_dir`), params (`arjun`), injection (`sqlmap`), XSS (`dalfox`), TLS
  (`testssl`), CMS (`wpscan` / `cmseek`).
- Known exploits: `searchsploit` against discovered product/versions.
- Secrets: `gitleaks` / `trufflehog` on retrieved code.

## 4. Exploit / validate (authorized, highest impact)
- `msfconsole` resource scripts, `hydra` (online), `john` / `hashcat` (offline).
  Validate a finding with a concrete proof before reporting it.

## 5. Report
- Record each validated issue with `report_finding` (title, severity, target,
  evidence, recommendation). Run `render_report` to write the Markdown + CSV
  deliverable into the working area.

## Operating notes
- Stage inputs and read tool output via `write_file` / `read_file` / `list_dir`
  (sandboxed to /work and /tmp).
- Prefer narrow scans; large output is truncated. Write big results to a file and
  read it back in parts.
- Every invocation is audit-logged. Nothing is blocked — you are accountable for
  staying in scope.
