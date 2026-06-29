---
name: security-apis
description: Enrich recon with free, no-login security APIs (cert transparency, exposed ports/CVEs, DNS/host/ASN, EPSS, file-hash and threat-intel lookups) via web fetch, alongside the wrapped Kali tools.
---

# Security APIs

A few public APIs fill gaps the local tools don't — cert transparency, exposed
ports/CVEs per IP, exploit-likelihood scores, file-hash and threat-intel lookups
— without sending a packet at the target. Run them early in recon to seed hosts
for `httpx_probe` / `nuclei`, and to triage findings.

Every endpoint below was probed and confirmed to return data with **no key and
no login** (the source list's auth column is unreliable — several "apiKey"
entries are actually open, and several "free" ones now demand an account, so
each was tested). Login-gated services (VirusTotal, GreyNoise, OTX, URLhaus,
EmailRep, …) are deliberately excluded. Full catalogue if you bring your own
keys: https://github.com/jaegeral/security-apis

## How to query

- **Already wrapped — use the tool, not the raw API:** Shodan (`shodan_host`,
  `shodan_search`), Censys (`censys_search`), subdomains (`subfinder`), ASN
  (`asnmap`). The APIs below add what those don't.
- **Otherwise:** fetch the URL — the client's web-fetch, or keep it audited and
  on the container's network with `run_shell` → `curl -s '<url>'`.
- These are free _because_ they're rate-limited (HackerTarget ~50/day per IP,
  IPinfo / ORKL per-IP throttling). Query specific targets, don't bulk-loop them.

## Verified no-login APIs

| API                          | Gives you                                                                  | Example (works as-is)                                                     |
| :--------------------------- | :------------------------------------------------------------------------- | :------------------------------------------------------------------------ |
| **crt.sh**                   | subdomains from certificate transparency                                   | `curl -s 'https://crt.sh/?q=%25.example.com&output=json'`                 |
| **HackerTarget**             | DNS / hostsearch / reverse-IP / ASN lookups                                | `curl -s 'https://api.hackertarget.com/hostsearch/?q=example.com'`        |
| **Shodan InternetDB**        | open ports, CPEs, known CVEs for an IP                                     | `curl -s https://internetdb.shodan.io/1.1.1.1`                            |
| **IPinfo**                   | geo, ASN, org, hostname for an IP (tokenless)                              | `curl -s https://ipinfo.io/8.8.8.8/json`                                  |
| **CIRCL CVE**                | CVE detail, CVSS, references by id                                         | `curl -s https://vulnerability.circl.lu/api/vulnerability/CVE-2021-44228` |
| **FIRST EPSS**               | exploit-prediction score for a CVE (prioritise)                            | `curl -s 'https://api.first.org/data/v1/epss?cve=CVE-2021-44228'`         |
| **CIRCL hashlookup**         | is a file hash known-good (NSRL) / known                                   | `curl -s https://hashlookup.circl.lu/lookup/sha256/<sha256>`              |
| **ORKL**                     | search threat-intel reports by keyword, actor, or **IOC** (domain/IP/hash) | `curl -s 'https://orkl.eu/api/v1/library/search?query=avsvmcloud.com'`    |
| **urlscan.io** (search only) | historical scans, screenshots, linked domains                              | `curl -s 'https://urlscan.io/api/v1/search/?q=domain:example.com'`        |

Highest-value: `crt.sh` + `internetdb` to expand the surface; `CIRCL CVE` +
`EPSS` together to detail and prioritise a CVE; `hashlookup` to clear known-good
files off a host; `ORKL` to pivot an IOC (domain/IP/hash you found on a box)
into the threat reports that reference it — attribution and TTPs.

## Opsec — read before querying

These are third-party services. Querying one **discloses the target** to that
provider. Indicator look-ups (cert transparency, CVE, DNS, ASN) are low-risk.
But anything that _submits_ — notably a urlscan.io **submit** (not the search
above) — defaults to **public** and can leak the engagement.

- Never submit a client's URL/file/sample to a public scanner or sandbox without
  explicit authorization. When unsure, ask the operator.
- Confirm the asset is in scope (`server_info`) before sending it anywhere.
