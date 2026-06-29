---
name: eu-cra
description: EU Cyber Resilience Act (Regulation (EU) 2024/2847) — scope, manufacturer obligations, conformity assessment/CE marking, and the load-bearing phased application timeline (web-verify the dates).
---

# EU Cyber Resilience Act — Regulation (EU) 2024/2847

Horizontal EU regulation setting **mandatory cybersecurity requirements for products with digital elements (PDEs)** placed on the EU market, across their lifecycle. It's a **product-safety / CE-marking** regime (New Legislative Framework style), **not** an org-wide ISMS standard. Binding regulation — directly applicable in all member states, no transposition.

Authoritative text + dates (web-verify, these are load-bearing): EUR-Lex https://eur-lex.europa.eu/eli/reg/2024/2847/oj/eng • EC overview https://digital-strategy.ec.europa.eu/en/policies/cyber-resilience-act • ENISA https://www.enisa.europa.eu/

## Scope — "products with digital elements"

- Any **hardware or software** whose intended/foreseeable use includes a **direct or indirect logical/physical data connection** to a device or network, placed on the EU market in the course of commercial activity. Includes remote data-processing solutions integral to the product.
- **Risk classes** (drive the conformity route): **default** (self-assessment), **Important — Class I** and **Class II** (Annex III, e.g. password managers, network mgmt, OS, routers, hypervisors), and **Critical** (Annex IV, e.g. smartcards/secure elements) — the higher tiers require stricter assessment.
- **Out of scope / carve-outs**: products already covered by sector law — medical devices (MDR/IVDR), motor vehicles, aviation (EASA), marine equipment; **SaaS/cloud** as a pure service (covered by NIS2, unless a remote-processing component of a product); **non-commercial open-source** software (open-source **stewards** get a lighter, tailored regime). Verify carve-out edges against the text.

## Core obligations (manufacturers)

- **Security-by-design & by-default** (Annex I Part I) — ship with a secure default configuration, minimal attack surface, protection of confidentiality/integrity, no known exploitable vulnerabilities at release.
- **Vulnerability handling** (Annex I Part II) — a coordinated vulnerability disclosure (CVD) policy, a process to handle/remediate vulns, and **security updates** (free, separate from feature updates).
- **SBOM** — produce and maintain a software bill of materials (at least top-level dependencies), in a commonly used machine-readable format; keep it (provide to authorities on request).
- **Support period** — provide security updates for the expected product lifetime, **default minimum 5 years** (unless the product is reasonably expected to be in use for less).
- **Documentation** — technical documentation (Annex VII), risk assessment, EU declaration of conformity, user information/instructions.
- **Reporting** to **ENISA** (single reporting platform) **and the relevant CSIRT**: **actively exploited vulnerabilities** and **severe incidents** — **early warning within 24h**, notification within 72h, final report within 14 days/1 month (Article 14). Verify exact stage clocks against Art. 14.

## Conformity assessment & CE marking

- Demonstrate conformity, draw up the **EU Declaration of Conformity**, affix **CE marking** — then the product may be placed on the market.
- Route depends on class: **default/Important** → can use **internal control (self-assessment, Module A)**, optionally against **harmonised standards** (being developed by CEN/CENELEC); **Class II / Critical** → **third-party (notified body)** assessment or EU certification scheme.
- **Importers and distributors** carry verification duties (CE present, docs available, manufacturer compliant).

## Phased timeline (WEB-VERIFY each — dates are dynamic and load-bearing)

- **Entry into force: 10 December 2024.**
- **11 June 2026** — rules on **notification of conformity assessment bodies** (notified bodies) begin to apply. (Reported widely; confirm against Art. 71 — verify: EUR-Lex.)
- **11 September 2026 (~21 months)** — the **reporting obligations** (Art. 14: actively-exploited-vulnerability and severe-incident reporting to ENISA/CSIRT) start applying.
- **11 December 2027 (~36 months)** — **full application**: all essential requirements, vulnerability handling, conformity assessment, CE marking, and SBOM obligations apply. Products placed on the market before this date are generally only caught if substantially modified afterward — verify transition provisions (Art. 69) against the text.

Penalties: up to **€15M or 2.5% of worldwide annual turnover** for breach of essential requirements (lower tiers for other breaches) — verify Art. 64.

## Pitfalls

- It's **product-by-product**, not company-level — a portfolio means repeated conformity work and per-product support clocks.
- **SaaS isn't automatically out** — a remote data-processing component that's integral to a PDE is **in** scope.
- **"On the market before the date" is not a permanent exemption** — substantial modification re-triggers the obligations.
- Don't assume self-assessment — check **Annex III/IV** class first; misclassifying Class II as default skips a mandatory notified-body step.

## Pairs with

- Org-level security management that supports CRA processes: `load_skill iso27001`, `load_skill nist-csf`.
- CRA vs NIS2 vs other obligations when choosing where to invest: `load_skill choosing-a-framework`.
