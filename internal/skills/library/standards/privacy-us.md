---
name: privacy-us
description: The US privacy patchwork — no single federal law, the growing set of state comprehensive laws, sectoral federal laws, FTC Act §5 as de-facto enforcer, and 50-state breach notification — plus how it differs from GDPR.
---

# US privacy: the patchwork (practitioner orientation, NOT legal advice)

There is **no single US federal privacy law** (no GDPR equivalent). Obligations stack from three independent layers, and the same data can trigger more than one. Engage counsel for any actual filing/decision — this is orientation.

- **State comprehensive privacy laws** — the closest thing to "GDPR-style" rights, but **state-by-state**, mostly **opt-out**.
- **Sectoral federal laws** — by data type / industry (health, financial, children, education).
- **FTC Act §5** — the catch-all enforcer for everyone else.
- **State breach-notification laws** — separate from all the above; **all 50 states + DC**, no federal floor.

Authoritative, fast-moving trackers (web-fetch for current state, do not trust memory):

- IAPP US State Privacy Legislation Tracker — https://iapp.org/resources/article/us-state-privacy-legislation-tracker
- IAPP US State Data Breach Notification Chart — https://iapp.org/resources/article/state-data-breach-notification-chart
- FTC privacy/security guidance — https://www.ftc.gov/business-guidance/privacy-security

## Layer 1 — State comprehensive privacy laws

- **~20 states have a comprehensive law in effect as of 2026** (CA, CO, CT, DE, IN, IA, KY, MD, MN, MT, NE, NH, NJ, OR, RI, TN, TX, UT, VA, WA); more are enacted-but-not-yet-effective. **verify current count + which are live:** https://www.multistate.us/insider/2026/2/4/all-of-the-comprehensive-privacy-laws-that-take-effect-in-2026 and the IAPP tracker. New for 2026 (Jan 1): Indiana, Kentucky, Rhode Island.
- **California is the outlier**: CCPA as amended by **CPRA** (2020) — has a dedicated regulator, the **California Privacy Protection Agency (CPPA)**, plus a private right of action for certain breaches. https://cppa.ca.gov/
- Most other states follow the **"Washington/Virginia model"**: enforced **only by the state Attorney General**, no private right of action, with a **cure period** (some sunsetting).
- **Applicability is threshold-gated, not universal** — typically by revenue and/or number of consumers' data processed (e.g. ~100k consumers, or ~25k + selling data). Thresholds vary per state; a small B2B SaaS may be out of scope in most. **verify per state.**
- Common consumer rights across the set: **access, delete, correct, portability, opt-out of sale/targeted advertising/profiling**; sensitive data needs opt-in-style consent or a right to limit. **Universal opt-out signals (e.g. Global Privacy Control) are mandatory in a growing number of states** — verify which.

## Layer 2 — Sectoral federal laws (by data type / industry)

- **HIPAA** — Protected Health Information held by covered entities (providers, plans, clearinghouses) + business associates. Regulator: **HHS Office for Civil Rights (OCR)**. (See `load_skill us-sectoral-regs` for the Security Rule / breach clock.)
- **GLBA** — "nonpublic personal information" held by financial institutions. Regulators: **FTC** (nonbank) + banking agencies. (Safeguards Rule detail in `us-sectoral-regs`.)
- **COPPA** — personal info of **children under 13** collected online. Regulator: **FTC**. Note the 2025 amended COPPA Rule tightened consent/retention — **verify effective dates:** https://www.ftc.gov/legal-library/browse/rules/childrens-online-privacy-protection-rule-coppa
- **FERPA** — student education records at federally-funded schools. Regulator: **US Dept of Education**.
- Others a security team meets: **CAN-SPAM** (email), **TCPA** (calls/texts), **VPPA** (video viewing — a live class-action magnet for web pixels/trackers), **BIPA** (Illinois biometric, private right of action, heavy litigation), **state genetic/health data laws** (e.g. Washington **My Health My Data**, consumer health data, broad).

## Layer 3 — FTC Act §5 (the de-facto national enforcer)

- **§5 bars "unfair or deceptive acts or practices."** No privacy statute needed: the FTC treats **broken privacy promises** (deceptive) and **unreasonable security** (unfair) as §5 violations. https://www.ftc.gov/legal-library/browse/statutes/federal-trade-commission-act
- This is why a privacy policy is a **binding representation** — say it, do it, or it's deceptive. Lax security causing consumer harm = unfair, even with no specific law on point.
- Consent orders run **~20 years** and bind future conduct. State AGs wield parallel "UDAP" authority under state law.

## State data-breach notification (separate regime, all 50 states + DC)

- **No single federal breach law for general PII.** You notify **per the residency of each affected individual** — a multi-state breach means multiple, differing obligations at once. There is **no preemption**; comply with the strictest in-scope state.
- **Most states: "without unreasonable delay."** A growing minority set a **hard clock**. **Strictest fixed deadline is now 30 days** (CA, CO, FL, WA, ME, NJ; others at 45/60). https://iapp.org/resources/article/state-data-breach-notification-chart
- **California SB 446 (effective Jan 1 2026)** replaced "without unreasonable delay" with a **hard 30 calendar days** to residents, and **15 days** to the **California AG** after notifying consumers. https://www.dataprotectionreport.com/2025/11/california-tightens-data-breach-notification-timelines-imposes-30-day-notice-requirement/
- **Rule of thumb for a national footprint:** plan to **notify individuals within 30 days** and **file with AGs ~15 days** after that. **~36 states also require AG/regulator notice**, often above a resident-count threshold (e.g. 500/1,000). Trigger is usually **unencrypted** PII (name + SSN/driver's license/financial/medical/biometric); definitions vary. **verify the exact element list + AG thresholds per state.**
- Sectoral breach clocks (HIPAA 60 days, GLBA 30 days, NYDFS 72 hours) run **on top of**, not instead of, state law — see `us-sectoral-regs`.

## US vs EU/GDPR — what a base LLM gets wrong

- **Scope of law**: GDPR = one regulation, all sectors, all of EU/EEA. US = patchwork; **no general federal statute**.
- **Consent default**: GDPR ≈ **opt-in** (lawful basis required up front). US state laws ≈ **opt-out** (collect, then honor opt-out of sale/targeted ads); only sensitive/children data leans opt-in.
- **What's protected**: GDPR "personal data" is **broad** and covers **employees**. Most US state laws **exclude B2B and employee data** and exempt data already covered by HIPAA/GLBA/FERPA (entity- or data-level carve-outs). **verify per state.**
- **Regulator/penalties**: GDPR = DPAs, up to €20M/4% turnover. US = AGs / FTC / sector regulators, **per-violation** civil penalties (e.g. CCPA ~$2,500, or ~$7,500 if intentional/minors — **verify current**).
- **Breach clock**: GDPR = 72h to the DPA. US = **no single clock** — state-by-state (30 days strictest) + sectoral overlays.
- EU resilience regimes (NIS2/DORA) have **no direct US analog**; the nearest US equivalents are sectoral (SEC, NYDFS, CMMC) — see `us-sectoral-regs`.

## Pairs with

- EU counterpart and the full comparison framing: `load_skill privacy-eu`.
- The US security/compliance regime map (SEC, HIPAA, GLBA, NYDFS, CMMC): `load_skill us-sectoral-regs`.
- Picking a control framework to satisfy these: `load_skill choosing-a-framework`, `load_skill control-mapping`.
- Vendor/processor obligations and DPAs: `load_skill third-party-risk`, `load_skill legal-contractual`.
