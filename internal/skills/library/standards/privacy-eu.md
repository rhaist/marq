---
name: privacy-eu
description: EU GDPR essentials for a security team — what creates the obligation, the 72-hour breach clock, fines, and how it differs from US privacy.
---

# EU privacy: GDPR

GDPR is _personal-data_ protection (Regulation (EU) 2016/679). Regulator: national
DPAs. For the EU _cybersecurity/resilience_ regimes (NIS2, DORA) see
`load_skill nis2-dora`; for the EU product-security regime see `load_skill eu-cra`;
for the US side and an EU/US comparison see `load_skill privacy-us`.

Canonical text (web-fetch for current wording): eur-lex Regulation (EU) 2016/679.

## What creates the obligation

- Processing **personal data** of people in the EU/EEA — **extraterritorial**:
  triggered by the data subject's location, not where the company sits. Offering
  goods/services to, or monitoring, EU people pulls a non-EU company in.
- Roles matter: **controller** decides purposes/means; **processor** acts on the
  controller's instructions. Obligations and liability differ.
- Lawful basis is required for every processing activity (consent, contract,
  legal obligation, vital/public interest, or legitimate interest).

## Breach notification clock

- A **personal-data breach** = breach of confidentiality, integrity, **or**
  availability — **ransomware counts** even with no exfiltration.
- To the **supervisory authority: within 72 hours** of becoming _aware_ (Art. 33),
  unless unlikely to risk rights/freedoms. Late = document and explain the delay.
- To **affected data subjects: "without undue delay"** when **high risk** to their
  rights (Art. 34). Strong encryption of the lost data can remove this duty.
- **Processor → controller: "without undue delay"** — push this into your DPAs
  (see `load_skill legal-contractual`).

## Data-subject rights & DPIAs

- Rights: access, rectification, erasure ("right to be forgotten"), restriction,
  portability, objection — with statutory response deadlines (generally 1 month).
- **DPIA** required for high-risk processing (Art. 35); the privacy-engineering
  process for running one is in `load_skill privacy-engineering`.

## Penalties

- Up to **€20M or 4%** of global annual turnover — whichever is higher.

## vs the US

- GDPR is **opt-in / omnibus**: one law, all personal data, consent-forward. The
  US is **opt-out / sectoral patchwork** (`load_skill privacy-us`). Don't assume
  a US privacy posture satisfies GDPR, or vice versa.

## Pairs with

- EU cyber/resilience law: `load_skill nis2-dora`; product security: `load_skill eu-cra`.
- Breach-response handling & reporting decisions: `load_skill incident-response-leadership`.
- Mapping obligations onto your control set: `load_skill control-mapping`, `load_skill gap-analysis`.
