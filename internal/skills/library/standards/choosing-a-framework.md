---
name: choosing-a-framework
description: Decision guide mapping a company's drivers (sector, customers, region, data type) to the security/privacy frameworks that actually apply — ISO 27001, SOC 2, NIST CSF/800-53, CIS, PCI DSS, HIPAA, GDPR, NIS2, DORA.
---

# Choosing a framework

Frameworks are not interchangeable. They split into three kinds — pick by **why**:

- **Certifiable / attestable** (you prove it to others): ISO 27001, SOC 2, PCI DSS.
- **Voluntary frameworks** (you structure a program): NIST CSF 2.0, CIS Controls.
- **Law / regulation** (non-optional if in scope): GDPR, NIS2, DORA, HIPAA. PCI DSS is contractual, not law, but enforced like one by card brands.

Most companies end up with a **stack**: one program framework (CSF or ISO) + whatever laws/contracts their data and customers force on them. Don't pick one and ignore the rest.

## Driver → framework

| Driver / situation                                                                                          | Reach for                   | Why                                                                      |
| :---------------------------------------------------------------------------------------------------------- | :-------------------------- | :----------------------------------------------------------------------- |
| Enterprise/EU customers demand a cert in procurement                                                        | **ISO 27001**               | Globally recognized certificate; auditable ISMS                          |
| US SaaS customers demand assurance                                                                          | **SOC 2**                   | The US B2B SaaS default; attestation report, not a cert                  |
| You take card payments (store/process/transmit PAN)                                                         | **PCI DSS v4.0.1**          | Mandatory via card-brand contracts; scope-driven                         |
| US health data (PHI)                                                                                        | **HIPAA**                   | Law for covered entities + business associates                           |
| Any personal data of people in the EU/UK                                                                    | **GDPR** (UK GDPR)          | Extraterritorial; applies by data subject location, not company location |
| EU operator in critical/important sector (energy, transport, health, digital infra, ICT, food, waste, mfg…) | **NIS2**                    | EU directive; broad sector sweep, management accountable                 |
| EU financial entity (bank, insurer, payment, crypto, fund) or critical ICT vendor to one                    | **DORA**                    | EU regulation; ICT operational resilience                                |
| US federal data / want a control catalog under CSF                                                          | **NIST 800-53**             | The control set CSF subcategories map into                               |
| Small/mid org, no mandate, "just get secure" cheaply                                                        | **CIS Controls v8.1 (IG1)** | Prioritized, concrete, free; good starting baseline                      |
| Need a common language across exec + technical, gap assessment                                              | **NIST CSF 2.0**            | Outcome-based lens; maps to ISO/800-53/CIS                               |

## How they relate (avoid double-work)

- **CSF 2.0 = the lens; ISO 27001 / 800-53 / CIS = the controls.** CSF subcategories carry Informative References that map to the others — one control set can satisfy several frameworks. See `load_skill control-mapping`.
- **ISO 27001 ≈ SOC 2 overlap is large** (access control, change mgmt, IR, vendor risk). Companies often run both off one control set; SOC 2 is broader on customer-facing assurance, ISO is a cleaner international cert.
- **CIS IG1 is a fast on-ramp** to ISO/CSF — implement it, then map upward.
- **PCI/HIPAA/GDPR/NIS2/DORA are obligations, not programs** — they tell you _what you must do_, an ISO/CSF program tells you _how to run it_. Run a program framework and treat the laws as additional required controls layered on.

## Region quick-cut

- **EU/UK**: GDPR always (if personal data). + NIS2 if critical sector, + DORA if finance. ISO 27001 is the procurement cert of choice.
- **US**: SOC 2 for B2B SaaS; HIPAA for health; sector laws (GLBA finance, state privacy e.g. CCPA — verify current). PCI everywhere cards are taken.
- **Global SaaS**: SOC 2 (US buyers) + ISO 27001 (everyone else) + GDPR (EU users) is the common trio.
- **APAC**: no single regime — ISO 27001 is again the procurement cert of choice, but privacy/transfer law is per-country and a common EU/US blind spot: **China PIPL** (data localization + a mandatory cross-border transfer route), **India DPDP Act** (rules notified Nov 2025, phasing in), **Singapore PDPA**, **Japan APPI**, **Australia Privacy Act** (+ **APRA CPS 234** info-security for regulated finance). GDPR compliance does not cover them — verify per country.

## Next steps

- Deep-dive a specific regime: `load_skill iso27001`, `soc2`, `pci-dss`, `nist-csf`, `privacy-eu` (GDPR), `nis2-dora`, `eu-cra`, `privacy-us`, `us-sectoral-regs`.
- Confirmed which apply → run a **gap analysis** against the chosen framework: `load_skill gap-analysis`.
- Then map controls once, satisfy many: `load_skill control-mapping`.
- For the canonical current text of any framework below, web-fetch the source named in its skill — versions and dates move.
