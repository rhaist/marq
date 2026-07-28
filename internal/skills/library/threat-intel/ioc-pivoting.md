---
name: ioc-pivoting
description: Take one indicator (IP/domain/hash/URL) and pivot it into context, history, and related infrastructure using free sources.
---

# IOC pivoting

Turn a single indicator into a cluster: who/what it is, when it was active, and
what else shares its infrastructure. Pivot outward, then verdict (see
`indicator-enrichment`). Sources: `load_skill security-apis`.

## First move — always

- ORKL full-text search the raw indicator:
  `curl -s 'https://orkl.eu/api/v1/library/search?query=<ioc>'`. A hit means the
  indicator already appears in a published APT/threat report — read it for actor,
  campaign, and sibling IOCs. This is the highest-value single pivot.
- urlscan search (domain/IP/url) for historical scans, screenshots, page hashes,
  and linked domains: `q=domain:<d>` / `q=ip:<ip>` / `q=hash:<sha256>`.

## By indicator type

- **IP** → `shodan_host` (open ports, TLS certs, banners, hostnames), Shodan
  InternetDB (ports/CVEs, no key), IPinfo (ASN/org/geo). Pivot on: shared TLS
  cert serial/SHA1, JARM, favicon hash, ASN, reverse-DNS, hosting org. Then
  `nmap -sV` / `httpx_probe` the IP **only if in scope** (`server_info`).
- **Domain** → `subfinder` + crt.sh (`%25.<domain>`) for siblings sharing the
  cert; `whois_lookup` + `dns_lookup` for registrant, NS, MX, current A/AAAA;
  `dnsx` to resolve. Pivot on: registrant email/org, NS set, same A record
  (shared host), cert SAN entries, registration date clustering.
- **URL** → urlscan for redirect chain + resources; extract the host and pivot as
  domain/IP; `httpx_probe` for live status/title/tech (in scope only).
- **Hash** → CIRCL hashlookup (known-good NSRL → likely benign), ORKL search,
  urlscan `hash:` for samples seen serving it. No detonation here.

## Infra-clustering pivots (ranked by signal)

- Shared TLS cert (SHA1/serial) or self-signed CN — strong, often same operator.
- Shared JARM / JA3S / favicon (murmur) hash — same server stack/panel.
- Same registrant email, or NS pair, or hosting on one quiet ASN — moderate.
- Co-resolution: many domains → one IP, or one domain → rotating IPs (fast-flux).
- Naming/registration-time clustering (DGA, batch-registered same day) — weak
  alone, strong with another pivot.

## Durability (pyramid of pain)

Pivot toward the durable end: hash < IP < domain < TTP. A hash/IP rotates
cheaply; cert reuse, panel fingerprints, and TTPs cost the adversary more, so
note those as the trackable indicators. Hand actor/TTP work to
`actor-ttp-attribution`.

## Record

- `write_file` the pivot graph (indicator → relation → indicator + source).
- `report_finding` only confirmed malicious infra tied to the engagement, with
  the source URL/report as a reference. Don't log raw third-party verdicts as
  fact — cite them.
