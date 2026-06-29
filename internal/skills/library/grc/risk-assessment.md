---
name: risk-assessment
description: Run a structured asset-threat-likelihood-impact risk assessment and emit a risk register.
---

# Risk assessment workflow

Turn loose findings into a scored, treatable register. Consume pentest output
first: `read_file /work/findings.md` (or `findings.jsonl`) — each confirmed
finding is a candidate risk. Don't re-discover what the engagement already proved.

## 1. Scope the assets

- List assets in scope: systems, data stores, processes, third parties. One row each.
- Tag each with a value driver: confidentiality / integrity / availability it serves,
  and a rough business value (regulated data? revenue path? safety?).

## 2. Pair asset x threat

- For each asset name the realistic threats (not every STRIDE box — what actually
  applies). A finding from `findings.md` IS a threat with proof: cite its id.
- Skip theoretical threats with no exposure; note why (compensating control exists).

## 3. Score likelihood x impact (5x5)

Likelihood — how plausible in the next 12 months:

| L                | Meaning                                            |
| ---------------- | -------------------------------------------------- |
| 1 Rare           | No known path; needs insider + luck.               |
| 2 Unlikely       | Theoretical, hard, no evidence seen.               |
| 3 Possible       | Plausible; skilled attacker, some barrier.         |
| 4 Likely         | Easy/known technique; exposure confirmed in recon. |
| 5 Almost certain | Trivial / actively exploited / already happening.  |

Impact — worst credible business outcome:

| I            | Meaning                                                   |
| ------------ | --------------------------------------------------------- |
| 1 Negligible | Cosmetic; no data/availability loss.                      |
| 2 Minor      | Single user/record; quick recovery.                       |
| 3 Moderate   | Limited breach, SLA breach, modest fine.                  |
| 4 Major      | Sensitive data loss, multi-day outage, regulatory report. |
| 5 Severe     | Mass breach, safety, existential fine, license loss.      |

Risk = L x I (1-25). Bands: 1-4 Low, 5-9 Medium, 10-15 High, 16-25 Critical.
A pentest finding's severity maps in as a sanity check — if a Critical finding
scores Low here, your likelihood is probably wrong; reconcile, don't average.

## 4. Choose treatment

| Option   | Pick when                                                                                                   |
| -------- | ----------------------------------------------------------------------------------------------------------- |
| Mitigate | Cost-effective control exists and risk > appetite. Default.                                                 |
| Transfer | Loss is insurable / outsourceable; you can't reduce likelihood.                                             |
| Avoid    | Risk has no business upside; kill the feature/asset/integration.                                            |
| Accept   | Residual <= appetite, or mitigation costs more than the loss. Needs a named owner + sign-off + review date. |

## 5. Emit the register

Write `/work/risk-register.md` with `write_file`, one row per risk:

| ID  | Asset | Threat (finding ref) | L   | I   | Score | Band | Treatment | Owner | Target date | Residual |
| --- | ----- | -------------------- | --- | --- | ----- | ---- | --------- | ----- | ----------- | -------- |

- Residual = re-score L x I assuming the treatment lands.
- For each Critical/High, also `report_finding` so it flows into `render_report`
  alongside the technical findings — one deliverable, not two.

## Notes

- Inherent (no controls) vs residual (with controls) — state which you scored; be consistent.
- Framework-specific risk methods (ISO 27005, NIST 800-30): `load_skill` the standards
  domain and web-fetch the current clause rather than guessing the steps.
