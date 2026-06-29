---
name: nis2-dora
description: EU NIS2 + DORA — who's in scope, the incident-notification clocks, security obligations, supervision/penalties, and the DORA-supersedes-NIS2 overlap for financial entities.
---

# EU operational resilience: NIS2 & DORA

Two EU cybersecurity regimes (distinct from GDPR's _privacy_ focus — for that see
`load_skill privacy-eu`). Different scope, different regulators, different clocks
— and a financial-sector incident can implicate both plus GDPR at once.

- **NIS2** — cybersecurity of essential/important entities. Regulator: national
  NIS authority / CSIRT. Directive (EU) 2022/2555.
- **DORA** — ICT operational resilience of the financial sector. Regulator:
  financial supervisors (ESAs / national). Regulation (EU) 2022/2554 + RTS/ITS.

Canonical text (web-fetch for current detail): NIS2 = eur-lex Directive (EU)
2022/2555; DORA = eur-lex Regulation (EU) 2022/2554 and its RTS/ITS on ESMA/EBA/
EIOPA. These cite RTS that get revised — verify exact figures before relying.

## Who's in scope

- **NIS2 — essential or important entity** in a covered sector: energy,
  transport, banking, financial market infra, health, drinking/waste water,
  digital infrastructure, ICT service management, public admin, space (essential);
  - postal, waste, chemicals, food, manufacturing, digital providers, research
    (important). Generally medium+ enterprises (size-cap rule, with exceptions).
    It's a **Directive** → obligations come from _national transposition law_, so
    specifics vary by member state.
- **DORA — financial entity** (banks, payment/e-money, investment firms,
  insurers, crypto-asset providers, funds, etc.) **or a critical ICT third-party
  provider** to them. It's a **Regulation** — directly applicable, uniform EU-wide.

## Security obligations (build the control once)

- **NIS2 (Art. 21):** risk-management measures — policies, incident handling,
  business continuity/backup, supply-chain security, vuln handling/disclosure,
  crypto, access control/MFA, and effectiveness testing. Management bodies must
  approve and oversee them.
- **DORA — five pillars:** (1) ICT risk-management framework, (2) ICT-incident
  management & reporting, (3) digital operational-resilience **testing** (incl.
  threat-led penetration testing / TLPT for significant entities — see
  `load_skill red-teaming`), (4) **ICT third-party risk** (incl. an oversight
  regime for critical providers), (5) information sharing.

## Notification clocks (the load-bearing numbers — verify against the RTS)

**NIS2 — significant incident (three-step, Art. 23)**

- **Early warning: within 24 hours** of becoming aware.
- **Incident notification: within 72 hours** (assessment, severity, IoCs).
- **Final report: within 1 month** of the notification. (Intermediate update on request.)

**DORA — major ICT-related incident (per RTS — verify current figures)**

- **Initial: ~4 hours after classifying** as major, and **≤24 hours** from awareness.
- **Intermediate: ~72 hours** after the initial notification.
- **Final: ~1 month.** Significant cyber-_threat_ notification is voluntary;
  inform affected clients where relevant.

## Overlap & precedence

- **DORA is _lex specialis_ for finance** and generally supersedes NIS2's
  incident-reporting for in-scope financial entities — **verify the carve-out**
  for your entity type before assuming one or the other.
- A ransomware hit on an EU **bank** holding PII can fire **all three**: GDPR
  (personal-data breach, `load_skill privacy-eu`), DORA _or_ NIS2, and sector
  supervisor rules. Build the control once, report to the right regulator on the
  right clock.
- **Management is personally accountable** under both — board oversight is an
  explicit legal requirement, not a nicety.

## Key dates

- **NIS2:** member-state **transposition deadline 17 Oct 2024** (several states
  were late — check the specific national law that actually binds you).
- **DORA:** **applies from 17 Jan 2025.**

## Penalties (orders of magnitude — NIS2 set in national law, verify)

- **NIS2:** essential up to **€10M or 2%** of global turnover; important up to
  **€7M or 1.4%**.
- **DORA:** supervisory measures + penalties per national/ESA regime; periodic
  penalty payments for critical ICT third-party providers.

## Pairs with

- `load_skill privacy-eu` (GDPR), `load_skill eu-cra` (product cybersecurity),
  `load_skill choosing-a-framework` (which regimes apply to you).
- Implementing the controls: `load_skill control-mapping`, `load_skill gap-analysis`,
  `load_skill incident-response-leadership` (the reporting-clock decisions).
