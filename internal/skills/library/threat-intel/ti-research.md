---
name: ti-research
description: Proactive threat-intel research and current awareness — where to look (feeds, RSS, KEV, advisories, ORKL) and how to turn scattered reporting into a tracked, cited picture of an actor, campaign, or sector threat.
---

# TI research & current awareness

The other threat-intel skills are reactive — you already have an IOC or a
behaviour and you pivot/attribute/enrich it (`ioc-pivoting`,
`actor-ttp-attribution`, `indicator-enrichment`). This one is the opposite
direction: **you have a question, not an indicator** — "what's this actor doing
lately", "what's actively exploited this week", "what's hitting my sector" — and
you need to go find the reporting. Fetch the sources, don't answer from memory:
threat reporting is perishable and your training data lags.

## Sources — fetch these, don't recall them

Query with the client's web-fetch, or keep it audited on the container network
with `run_shell` → `curl -s '<url>'`. All are free / no-login. Rate-limited —
query what you need, don't bulk-loop.

### Report libraries & search (start here)

- **ORKL** — the APT/threat-report library. Full-text search across thousands of
  vendor reports by keyword, actor, tool, or IOC:
  `curl -s 'https://orkl.eu/api/v1/library/search?query=<term>'`. List a specific
  actor's reports, get report text/refs by id (`/library/entry/<id>`). This is
  the single highest-value TI research endpoint.
- **ORKL news** — a curated security-news firehose (breaches, CVEs, campaigns),
  one aggregated feed instead of scraping the RSS table below:
  `curl -s 'https://orkl.eu/api/v1/news/entries?limit=50'` (add `&offset=N` to
  page). Each entry carries `published_at`, a `base_score` (4–10 importance),
  `feed_group` (e.g. "ORKL Top Sources", "National CERTs/CSIRTs") and any tagged
  `threat_actors` — sort by `base_score` to triage "what matters this week".
- **CISA KEV** — the authoritative "actively exploited" list, updated most
  weekdays: `curl -s https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json`.
  Cross with FIRST EPSS (see `security-apis`) to rank what to care about.

### RSS / Atom feeds (current awareness)

Fetch the feed XML and read the latest `<item>`/`<entry>` titles + links; open
the ones on-topic. Good, stable, free feeds:

| Source                                                  | Feed URL                                                                        |
| :------------------------------------------------------ | :------------------------------------------------------------------------------ |
| CISA advisories (all)                                   | `https://www.cisa.gov/cybersecurity-advisories/all.xml`                         |
| The DFIR Report (intrusion walk-throughs, full TTP+IOC) | `https://thedfirreport.com/feed/`                                               |
| Google TAG / Mandiant / GTIG blog                       | `https://blog.google/threat-analysis-group/rss/`                                |
| Microsoft Threat Intelligence                           | `https://www.microsoft.com/en-us/security/blog/topic/threat-intelligence/feed/` |
| Cisco Talos                                             | `https://blog.talosintelligence.com/rss/`                                       |
| Unit 42 (Palo Alto)                                     | `https://unit42.paloaltonetworks.com/feed/`                                     |
| Volexity                                                | `https://www.volexity.com/blog/feed/`                                           |
| SANS ISC daily diary                                    | `https://isc.sans.edu/rssfeed_full.xml`                                         |
| Krebs on Security                                       | `https://krebsonsecurity.com/feed/`                                             |
| BleepingComputer                                        | `https://www.bleepingcomputer.com/feed/`                                        |

For an actor/campaign deep-dive, ORKL library search beats scraping feeds. For
"what's new this week", start with ORKL news (`base_score`-ranked, one call);
drop to the raw vendor feeds below when you need a source ORKL doesn't aggregate
or the item is fresher than the news index.

### Sector / regional CERTs (coverage gaps)

Western vendors under-cover APAC-targeting activity (see the source-bias note in
`actor-ttp-attribution`). When the target is JP/KR/IN/SEA, add: JPCERT/CC
(`https://blogs.jpcert.or.jp/en/atom.xml`), KrCERT/KISA, CERT-In, AusCERT, and
regional vendors (AhnLab, NSFOCUS, QiAnXin). Expect heavier alias sprawl.

## Workflow

1. **Frame the question** — actor, campaign, CVE, or sector? Pick the axis. Note
   the time window ("lately" = be explicit: last 30/90 days).
2. **Sweep** — ORKL search for the actor/tool/CVE; pull the last N days from 2–3
   relevant feeds. Skim titles, open the on-topic items. Breadth first.
3. **Read for substance** — from each source extract: TTPs (map to ATT&CK IDs —
   hand off to `actor-ttp-attribution`), IOCs (hand to `ioc-pivoting` /
   `indicator-enrichment` to verify — never assert a report's IOC as live fact
   without checking), victimology (sector/geo), and timeline.
4. **Corroborate** — one vendor blog is a lead, not a fact. Require a second
   independent source before asserting attribution or a campaign link. Note when
   vendors name the same group differently (alias sprawl).
5. **Synthesise** — build the picture: who, what TTPs, what infra, what targets,
   over what window, with **every claim cited to a URL**. Confidence explicitly
   (default low; raise only when independent axes converge).

## Output

- `write_file` a research brief: summary → TTPs (ATT&CK table) → IOCs (with
  verification status) → victimology → timeline → sources. Cite every claim.
- `report_finding` only the durable, defensible items tied to the engagement
  (TTPs, confirmed-malicious infra), with report URLs as references. Attribution
  and "assessed intent" go in the narrative as hypotheses, not as finding facts.
- Freshness matters more here than anywhere: date-stamp the brief and note the
  window it covers — TI goes stale fast.
