---
name: ot-compliance
description: Which regimes actually bind a plant or machine builder — NIS2 for manufacturing (NACE C26–C30), EU CRA + Machinery Regulation for what you ship, NERC CIP and TSA directives for US infrastructure, and how they stack.
---

# OT / ICS compliance

For an international manufacturer the obligations split cleanly in two, and
conflating them is the usual mistake:

- **Your organisation and its plants** → NIS2 (EU), NERC CIP / TSA (US sectors).
- **The products and machines you ship** → EU CRA + Machinery Regulation.

Engineering detail lives in `load_skill ot-ics`. This skill is about who can fine
you and when.

## NIS2 — does it catch a factory?

- **Annex II sector 5 "Manufacturing" → _important_ entity**, covering: medical
  devices and IVDs; **NACE C26** computer/electronic/optical; **C27** electrical
  equipment; **C28** machinery and equipment n.e.c.; **C29** motor vehicles and
  trailers; **C30** other transport equipment. Chemicals and food are _separate_
  Annex II sectors, not part of "Manufacturing" — check which row you are in.
- **Threshold**: medium-sized or larger, i.e. **≥50 staff OR >€10m** turnover /
  balance sheet. Below that you are out of scope absent a national carve-in.
- **The load-bearing subtlety:** the words "operational technology", "industrial
  control" and "SCADA" appear **nowhere** in the Directive. OT is in scope by
  implication, through the Art. 6(1)(b) definition of a network and information
  system — _devices which, pursuant to a programme, carry out automatic
  processing of digital data_. That is what pulls PLCs, DCS and HMIs under the
  Art. 21 measures. **Argue coverage from Art. 6(1)(b)**; do not go looking for
  an OT-specific hook, because there isn't one, and an auditor who expects one
  will wrongly conclude the plant is exempt.
- **Art. 21(2)** measures apply to the plant as much as the office: risk
  analysis, incident handling, business continuity, **supply-chain security**,
  secure acquisition/development with **vulnerability handling**, effectiveness
  assessment, cyber hygiene and training, cryptography, HR/access/asset
  management, and MFA plus secured emergency communications.
- Obligations come from **national transposition law**, which varies. See
  `load_skill nis2-dora` for transposition status, the incident clocks and the
  DORA overlap.

## What you ship — CRA and the Machinery Regulation both apply

- **EU CRA** (Reg. (EU) 2024/2847) — product cybersecurity. **Art. 14 reporting
  of actively exploited vulnerabilities starts 11 Sep 2026**; full application
  **11 Dec 2027**. `load_skill eu-cra`.
- **EU Machinery Regulation** (Reg. (EU) 2023/1230) — **applies 14 Jan 2027**,
  repealing Directive 2006/42/EC. Cyber-relevant essential requirements:
  - **Annex III 1.1.9 Protection against corruption** — connecting a device must
    not create a hazard; hardware carrying safety signals protected against
    **accidental _and intentional_** corruption; evidence of legitimate and
    illegitimate intervention collected; the machine must be able to show the
    list of installed safety-relevant software at any time; changes logged.
  - **Annex III 1.2.1 Safety and reliability of control systems** — control
    systems must withstand **"reasonably foreseeable malicious attempts from
    third parties"**; safety-function limits must not be modifiable;
    **intervention and software-version logs retained 5 years**. Self-evolving
    systems are constrained to a defined task/movement space, with
    safety-decision data kept 1 year.
  - **Annex I Part A.5/A.6** — safety components with fully or partially
    **self-evolving (ML) behaviour**, and machinery embedding them, take the
    mandatory **notified-body** route. If you are putting ML in a safety
    function, that is a conformity-assessment decision, not a product decision
    (`load_skill eu-ai-act` for the parallel AI Act analysis).
- **They do not substitute for each other.** A machine builder in scope of both
  meets **both** requirement sets and runs **both** conformity assessments; CRA
  compliance may _facilitate_ 1.1.9/1.2.1 but the synergy must be **demonstrated,
  not assumed**. The only statutory shortcut is **MR Art. 20(9)**: a certificate
  under an EU cybersecurity certification scheme (Reg. (EU) 2019/881) cited in
  the OJ gives presumption of conformity with 1.1.9 and 1.2.1 — a **CRA CE mark
  does not**.
- **Do not claim "62443 certified = CRA compliant."** Presumption of conformity
  requires a harmonised standard **cited in the OJEU**, and **none has been cited
  yet**. Standardisation request **M/606** (Feb 2025, 41 deliverables) has 62443
  feeding only the **OT vertical** drafts (the prEN 50770 series); the CRA
  horizontals are being written on EN 18031 and EN ISO/IEC 29147/30111 instead.
  Drafting deadlines run into late 2027 — plan to demonstrate conformity against
  the Regulation's text.

## US — only if you own the assets

- **NERC CIP** (bulk electric system). Currently enforceable includes CIP-002
  categorisation, CIP-003 security management, CIP-005 electronic security
  perimeters, CIP-007 system security, CIP-008 incident reporting, CIP-010
  config/vuln management, **CIP-013-2 supply-chain risk** (since Oct 2022), and
  **CIP-012-2 control-centre communications (new, 1 Jul 2026)**.
  - **CIP-015-1 internal network security monitoring is NOT in effect.** FERC
    Order No. 907 (Jun 2025); compliance lands **1 Oct 2028** for control
    centres, with medium-impact systems with external routable connectivity at
    non-control-centre assets getting a further two years (**2030**). CIP-015-2
    is filed and pending. Treat INSM as a budget-cycle item, not a 2026 problem.
  - A large virtualization package (CIP-002-7/-8, CIP-005-8, CIP-010-5 and
    others) was FERC-approved Mar 2026 and is **effective 1 Jul 2028**.
- **TSA Security Directives** — surface transport cyber runs through the SD
  series for **pipeline, freight rail and passenger rail** (aviation cyber is
  handled through separate security-programme instruments, so don't cite the
  surface SDs at an airline). They are **renewed annually**, so always check the
  current letter suffix and expiry; the pipeline and rail directives currently
  run on Jan and May renewal cycles into 2027. The surface-cyber rulemaking
  (RIN 1652-AA74) was proposed Nov 2024 but has been **demoted to long-term
  actions with no final-rule date** — the directives, not a rule, bind you.
- **CIRCIA** will add incident reporting once final; **it is not yet
  enforceable**. `load_skill us-sectoral-regs`.

## Rules

- Separate "we operate plants" from "we ship machines" before quoting any
  deadline — they are different regimes with different regulators.
- **The first CRA-adjacent date is 11 Sep 2026** (vulnerability reporting), not
  the 2027 full-application date most planning decks show.
- For NIS2 in a factory, build the Art. 6(1)(b) coverage argument into the scope
  document up front.
- Certification claims are about products; regulators ask about your plant. Map
  once with `load_skill control-mapping`, then run
  `load_skill gap-analysis` per regime.

## Pairs with

- `load_skill ot-ics` (the engineering), `load_skill nis2-dora`,
  `load_skill eu-cra`, `load_skill us-sectoral-regs`,
  `load_skill choosing-a-framework`.
