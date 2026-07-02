---
name: us-sectoral-regs
description: The US security/compliance regimes a security team actually meets — SEC cyber disclosure, HIPAA Security Rule, GLBA Safeguards, NYDFS 500, CMMC/FedRAMP/FISMA/800-171 — with a "who must comply with what" decision table.
---

# US sectoral cyber regulations (practitioner orientation, NOT legal advice)

There is no general US cyber law; obligations attach by **what you do** (public company, handle health/financial data, sell to NY-regulated finance, contract with DoD/federal). Each has its **own regulator and clock**. Hedge all dates — these shift; web-fetch the source. Engage counsel for filings.

## Who must comply with what (decision table)

| If you are…                                | Regime                                 | Regulator          | Headline obligation / clock                                                           |
| ------------------------------------------ | -------------------------------------- | ------------------ | ------------------------------------------------------------------------------------- |
| US public company (SEC registrant)         | SEC cyber disclosure rules             | SEC                | **8-K Item 1.05 within 4 business days** of materiality call; annual **10-K Item 1C** |
| Health provider/plan/clearinghouse + BAs   | HIPAA Security + Breach Rules          | HHS OCR            | Safeguards; breach notice **≤60 days** (≥500 indiv.)                                  |
| Nonbank financial institution              | GLBA **Safeguards Rule**               | FTC                | Written infosec program; **notify FTC ≤30 days** (≥500 consumers)                     |
| Bank / depository institution              | GLBA via banking agencies              | OCC/FDIC/FRB/NCUA  | **36-hour** computer-security incident notice to regulator                            |
| NY-licensed financial/insurance entity     | **23 NYCRR Part 500** (NYDFS)          | NY DFS             | **72-hour** incident notice; **24-hour** extortion-payment notice                     |
| DoD contractor handling **CUI**            | **CMMC 2.0** (+ DFARS 252.204-7012)    | DoD / C3PAO        | Level 1–3 assessment; **72-hour** incident report to DC3                              |
| Cloud service sold **to federal agencies** | **FedRAMP**                            | GSA / FedRAMP      | Authorization (Low/Mod/High) before federal use                                       |
| Federal agency / its IT contractors        | **FISMA / NIST 800-53**                | OMB / CISA / NIST  | Agency ATO program                                                                    |
| Any federal contractor handling CUI        | **NIST SP 800-171**                    | Contracting agency | 110 controls; SPRS score                                                              |
| Critical-infrastructure owner/operator     | **CIRCIA** (when final) + sector rules | CISA + sector      | Covered-incident reporting (rule pending — see below)                                 |

## SEC cybersecurity disclosure (public companies)

- In force since **Dec 2023** (large filers; smaller reporting companies phased in ~June 2024). Two pieces:
  - **Form 8-K Item 1.05** — disclose a **material** cybersecurity incident **within 4 business days** of the **materiality determination** (NOT discovery — clock starts when you decide it's material; you must make that call "without unreasonable delay"). Describe nature, scope, timing, and material impact.
  - **Form 10-K Item 1C** — **annual** disclosure of cyber **risk-management processes, strategy, and board/management governance.**
- **National-security/public-safety delay**: only if the **US Attorney General** notifies the SEC; initial delay up to 30 days (extendable). https://www.sec.gov/rules/final/2023/33-11216.pdf
- **Status hedge:** industry petitioned in 2025 to rescind/narrow Item 1.05 (joint petition filed May 2025) and the Commission majority shifted, but **the rule remains in effect as of mid-2026** — no rescission or amendment has been adopted. **Verify current status** before advising: https://www.sec.gov/rules-regulations/2025/05/joint-petition-rulemaking-amend-sec-cybersecurity-risk-management-strategy-governance-incident
- Practitioner trap: a **ransom payment or extortion** is not automatically a 1.05 trigger — the test is **materiality**, and many filers over-disclosed early. Document the materiality analysis.

## HIPAA Security Rule (health)

- Applies to **covered entities + business associates**. Requires **administrative, physical, technical safeguards** for ePHI (risk analysis is the keystone control).
- **Breach Notification Rule**: notify affected individuals **and HHS without unreasonable delay, ≤60 days**; **≥500 individuals** = notify HHS and **prominent media** promptly (≤60 days); **<500** = annual log to HHS. https://www.hhs.gov/hipaa/for-professionals/breach-notification/index.html
- **2025 Security Rule NPRM** proposes to **modernize/strengthen** safeguards (mandatory MFA, encryption, asset inventory, faster restoration objectives). Comment period closed Mar 2025; **still not finalized as of mid-2026** (OCR reviewing ~4,700 comments; industry has pressed for withdrawal) — **verify status/effective date:** https://www.hhs.gov/hipaa/for-professionals/security/hipaa-security-rule-nprm/index.html

## GLBA Safeguards Rule (nonbank financial — FTC)

- FTC-enforced version applies to **nonbanking financial institutions** (mortgage brokers, auto dealers/financing, payday lenders, tax preparers, etc. — broader than people expect).
- Requires a written infosec program with **named qualified individual, risk assessment, MFA, encryption, access controls, vendor oversight**.
- **Breach notification (effective May 13 2024):** notify the **FTC as soon as possible, no later than 30 days** after discovering an event involving **unencrypted info of ≥500 consumers**. https://www.ftc.gov/news-events/news/press-releases/2023/10/ftc-amends-safeguards-rule-require-non-banking-financial-institutions-report-data-security-breaches
- **Banks** follow GLBA via their prudential regulators, with the separate **36-hour** incident-notification rule — different clock, different regulator.

## NYDFS 23 NYCRR Part 500 (NY financial/insurance)

- Applies to entities with a **NY DFS license** (banks, insurers, mortgage lenders, etc.). Second Amendment (Nov 2023) **fully effective Nov 1 2025**.
- **72-hour** notice to the Superintendent for a reportable cybersecurity incident; **24-hour** notice of any **extortion/ransom payment** + written justification within 30 days. CISO + annual certification, MFA, asset inventory. https://www.dfs.ny.gov/industry_guidance/cybersecurity

## CMMC 2.0 / FedRAMP / FISMA / 800-171 (federal & defense)

- **CMMC 2.0** — DoD contractors handling **FCI/CUI**. Final **48 CFR** acquisition rule **effective Nov 10 2025**, triggering a **4-phase rollout** (verify, dates have moved before):
  - **Phase 1 — Nov 10 2025** (the current phase as of mid-2026): L1/L2 **self-assessment** required in new contracts.
  - **Phase 2 — Nov 10 2026**: L2 **third-party (C3PAO) certification** (assessor capacity is the known bottleneck).
  - **Phase 3 — Nov 10 2027**: L3.
  - **Phase 4 — Nov 10 2028**: full implementation across applicable contracts.
  - https://www.federalregister.gov / https://dodcio.defense.gov/cmmc/ — **verify current phase.** Underlying control set: **NIST SP 800-171** (Level 2). DFARS 252.204-7012 still requires **72-hour** incident reporting to DC3 + DIBNet.
- **FedRAMP** — standardized authorization for **cloud services sold to federal agencies** (Low/Moderate/High baselines, built on **NIST 800-53**). FedRAMP has been modernizing toward "**20x**" automation — verify program state: https://www.fedramp.gov/
- **FISMA** — governs **federal agencies** (and their contractors operating systems on their behalf); implemented via **NIST 800-53** controls and agency ATOs.
- **NIST SP 800-171** — the **110-control** CUI baseline (now **Rev 3**) for non-federal systems; contractors self-score into **SPRS**. This is the shared substrate under CMMC L2.

## Critical infrastructure / utilities (high level)

- **CIRCIA** (Cyber Incident Reporting for Critical Infrastructure Act) — will require **covered entities** to report covered incidents to **CISA within 72 hours** and ransom payments within 24 hours, **once the final rule is in effect — still NOT enforceable as of mid-2026**: CISA missed the statutory Oct 2025 deadline, targeted May 2026, and slipped again (rulemaking town halls restarted June 2026 after the DHS appropriations lapse). **Verify status:** https://www.cisa.gov/topics/cyber-threats-and-advisories/information-sharing/cyber-incident-reporting-critical-infrastructure-act-2022-circia
- Sector-specific: **NERC CIP** (bulk electric system), **TSA Security Directives** (pipelines, rail, aviation), **EPA/AWIA** (water), **NRC** (nuclear). Plus state PUC/utility-commission rules. Treat these as additional, not substitutes.

## US vs EU framing

- The US equivalent of NIS2/DORA is **not one law** — it is this **sectoral spread** (SEC ≈ disclosure governance, NYDFS/GLBA ≈ DORA-ish for finance, CMMC/FISMA ≈ federal supply chain, CIRCIA ≈ NIS2-ish but narrower and pending). See `load_skill privacy-eu` for the EU side.

## Pairs with

- US privacy patchwork + breach notification: `load_skill privacy-us`.
- Mapping these regimes onto one control set (don't build N programs): `load_skill control-mapping`, `load_skill choosing-a-framework`.
- Vendor/contract flow-downs (DFARS, BAAs, FedRAMP): `load_skill third-party-risk`, `load_skill legal-contractual`.
