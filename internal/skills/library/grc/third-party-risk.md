---
name: third-party-risk
description: Score vendor/supply-chain risk — what to ask, how to tier, what evidence to demand.
---

# Third-party / vendor risk

Assess a vendor before/while you trust them with data or access. Effort scales
with the vendor's blast radius — tier first, then dig.

## 1. Tier the vendor (drives everything else)

| Tier      | Trigger (any one)                                                                        | Diligence                                                    |
| --------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| Critical  | Hosts/processes regulated or customer data; has prod access; outage stops your business. | Full review + evidence + annual reassess + contract clauses. |
| Important | Internal data, no prod access; meaningful outage impact.                                 | Questionnaire + attestation review; reassess every 1-2 yr.   |
| Low       | Public/no sensitive data; easily replaced.                                               | Lightweight check at onboarding; no recurring review.        |

Tier = data sensitivity x access level x dependency. When unsure, tier up.

## 2. Ask (map answers to evidence, not promises)

- **Certs**: current SOC 2 Type II / ISO 27001 cert + scope? Get the report, not the logo.
- **Data**: what data, where stored, which regions, retention, encryption at rest/in transit, sub-processors?
- **Access**: who in the vendor can reach your data? MFA, least privilege, offboarding?
- **Resilience**: RTO/RPO, last DR test, backup model, last availability figures.
- **Security program**: pen-test cadence (ask for exec summary), vuln SLA, patch cadence.
- **Incidents**: breach notification SLA (hours), breach history, contact path.
- **Supply chain**: their critical sub-processors (4th parties) — concentration risk (everyone on one cloud/region?).

## 3. Score each domain 1-5, weight, combine

Per domain rate residual risk: 1 strong evidence / 5 no control or refused to answer.

| Domain                       | Weight |
| ---------------------------- | ------ |
| Data protection & encryption | 0.25   |
| Access control & identity    | 0.20   |
| Certifications & audit       | 0.15   |
| Resilience / BCDR            | 0.15   |
| Vuln & patch management      | 0.15   |
| Incident response            | 0.10   |

Vendor score = sum(domain x weight). Bands: <2 Low, 2-3 Medium, >3 High.
Any single domain at 5 on a Critical-tier vendor = High overall regardless of average
(no compensating average for an unanswered breach-notification clause).

## 4. Evidence to collect & retain

- SOC 2 Type II report (read the exceptions section + CUECs — complementary user
  entity controls are YOUR homework).
- ISO 27001 cert + Statement of Applicability scope.
- Latest pen-test / vuln-scan summary; their cyber-insurance proof; DPA / sub-processor list.
- File everything in `/work` and reference paths in the output, so reassessment can diff.

## 5. Output

`write_file /work/vendor-<name>.md`:

| Domain                                                                                                                            | Score | Evidence | Notes/gap |
| --------------------------------------------------------------------------------------------------------------------------------- | ----- | -------- | --------- |
| ... plus header: Vendor, Tier, Overall score+band, Recommendation (approve / approve-with-conditions / reject), Reassess-by date. |

- For High-band findings, `report_finding` so vendor risk lands in the same `render_report` deliverable.
- Contract asks for Critical vendors: breach-notice SLA, audit right, sub-processor change notice, deletion-on-exit.

## Notes

- "We have SOC 2" is not evidence; the report's scope + exceptions + period are.
- Recheck on trigger events too: vendor breach, M&A, scope change — not just the calendar.
- **EU financial sector:** DORA (in force since 17 Jan 2025) mandates a **register of all ICT
  third-party arrangements**, forces you to flag provider concentration/substitutability, and
  puts _critical_ ICT providers under direct EU oversight. `load_skill nis2-dora`.
- **APAC pitfall (what EU/US teams get wrong):** a SOC 2 + DPA is not enough. APRA **CPS 230**
  (Australia, since 1 Jul 2025) and MAS's regime (revised Outsourcing Guidelines Dec 2024; a
  broader TPRM guideline proposed 2026) demand a **material-service-provider register**, named
  **fourth-party / sub-outsourcing** disclosure, and **data-location** clauses — and
  data-localization laws (China PIPL, India DPDP) can block the flows your vendor assumes.
  Tier and contract to the strictest in-scope jurisdiction, not your home one.
