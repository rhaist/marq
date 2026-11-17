---
name: recon-footprint
description: "Guides passive OSINT footprinting of a domain or organization: whois, DNS, subfinder, theHarvester, spiderfoot, certificate transparency, historical URLs, leaked secrets in public repos, internet-scan data (Shodan/Censys) and people footprint. Use when the user asks to recon a target, map an external footprint, find subdomains, or start an OSINT investigation."
license: Apache-2.0
metadata:
  verified: "2026-11-17"
---

# Recon / footprint workflow

Everything here queries public third-party sources, with no scanning, probing or
crawling of the target. DNS lookups may reach the domain's own nameservers.

Passive doesn't mean lawful-by-default: OSINT on named people collects personal
data governed by GDPR and APAC privacy regimes (Singapore PDPA, Japan APPI, etc.).
Collect only what the investigation needs, and keep findings data in the client's
region. Shipping a dossier of employee data to a home-region host is a common
cross-border misstep.

## 1. Passive footprint (company/domain)

- `theharvester`: emails, names, hosts, subdomains from public sources. Best
  single opening move.
- `subfinder`: passive subdomain enumeration. Feed results onward.
- `whois_lookup` + `dns_lookup`: registration, name servers, records.
- `ct_search`: certificates in CT logs (crt.sh) for a domain and subdomains.
- `rdap`: structured registration data for a domain, IP or ASN.
- `dnstwist`: registered lookalike domains (background job, poll with `job_status`).
- `gau_urls`: historical URLs (Wayback/CommonCrawl/OTX/URLScan), forgotten
  endpoints and params. Runs as a background job: poll with `list_jobs` /
  `job_status` and read the job's `stdout.log`.
- `spiderfoot`: broad passive sweep (passive modules only; background job, poll
  with `job_status`).
- People: `sherlock` / `maigret_username` (a username across sites),
  `email_accounts` (email → accounts), `h8mail_breach` (email → breaches),
  `phoneinfoga` (phone numbers), `exif_metadata` (author/device data in
  retrieved documents, staged under `/work` or `/tmp`).
- Secrets/exposure: `trufflehog` on public repos at github.com, gitlab.com,
  bitbucket.org or codeberg.org. `source` is `git <https-url>`, `github --org=<org>`
  or `github --repo=<url>`. It never verifies, so it doesn't use any key it finds.

## 2. Resolve & classify

- Pass the collected subdomains to `dnsx` as `hosts` (comma or newline separated)
  to resolve them (no `write_file` staging needed); `records` defaults to `a` (records via public resolvers).
- `asnmap` for the ASN/CIDR ranges an org owns (`target` is an IP, domain or ASN);
  `cdncheck` (`targets` are IPs) to flag hosts behind a CDN/WAF/cloud (so an IP is the edge, not the origin).
- Dangling CNAMEs (subdomain-takeover candidates) show up in `dnsx` CNAME output;
  report with the DNS evidence only.

## 3. Third-party scan data

- `shodan_host` / `shodan_search` and `censys_view`: ports, banners, TLS certs and
  CVEs as already indexed by those services (e.g. `ssl.cert.subject.cn:acme.com`).
  `shodan_host` works keyless; `shodan_search` needs `SHODAN_API_KEY` and
  `censys_view` needs `CENSYS_API_TOKEN`.
- Cert transparency, per-IP ports/CVEs and other keyless APIs: `load_skill security-apis`.

## Hand-off

- Record interesting assets as you go with `report_finding`, then `render_report`.
  For prioritising what you found, `load_skill indicator-enrichment` and
  `load_skill ioc-pivoting`.
