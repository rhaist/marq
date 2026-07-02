---
name: recon-footprint
description: Opening recon — map a domain's external footprint passively first.
---

# Recon / footprint workflow

Start passive (no packets at the target), then go active only in scope.

Passive doesn't mean lawful-by-default: OSINT on named people collects personal
data governed by GDPR and APAC privacy regimes (Singapore PDPA, Japan APPI, etc.).
Collect only what the engagement needs, and keep findings/PoC data in the client's
region — shipping a dossier of employee data to a home-region host is a common
cross-border misstep.

## 1. Passive footprint (company/domain)

- `theharvester` — emails, names, hosts, subdomains from public sources. Best
  single opening move.
- `subfinder` — passive subdomain enumeration. Feed results onward.
- `whois_lookup` + `dns_lookup` — registration, name servers, records.
- `gau_urls` — historical URLs (Wayback/CommonCrawl/OTX/URLScan), forgotten
  endpoints and params without touching the live target.
- `spiderfoot` — broad automated sweep (background job; poll with `job_status`).
- People: `sherlock` / `maigret_username` (a username across sites),
  `holehe_email` (email → accounts), `phoneinfoga` (phone numbers).
- Secrets/exposure: `gitleaks` / `trufflehog` on any retrieved repos.

## 2. Resolve & find what's live

- Collect subdomains into a file with `write_file`, then `dnsx` to resolve and
  `httpx_probe` to find live HTTP services (status, title, tech).
- `cdncheck` to flag hosts behind a CDN/WAF (you may be hitting the edge, not the
  origin). For subdomain takeover on resolved subdomains (dangling CNAMEs), run
  `nuclei` with the takeover templates (`--tags takeover`).

## 3. Enumerate hosts (active)

- `naabu` for a fast port sweep, then `nmap` `-sV` on the open ports for
  service/version detail. `masscan` for very large ranges.

## 4. Fingerprint web

- `whatweb` and `wafw00f` per live host; `katana` to crawl endpoints.

## Hand-off

- Record interesting assets/params as you go (notes in the report via
  `report_finding` for confirmed issues). Then pick a vuln-class skill
  (sqli/xss/ssrf/idor/...) for the testing phase.
