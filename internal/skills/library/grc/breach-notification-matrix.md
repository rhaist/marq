---
name: breach-notification-matrix
description: Every incident/breach notification clock a security team meets (EU, UK, US federal and state, APAC, contractual) in one table with trigger, recipient, deadline, owner and source, plus how to build the per-incident matrix in the first hour.
---

# Breach notification matrix

One incident can start a dozen clocks, each with its own **start event**, recipient and
owner. This is practitioner orientation, not legal advice: counsel decides whether and
what to notify. Security's job is to surface a _plausible_ notifiable event early and
supply the facts and timestamps. Owners below are typical roles; set yours in advance.
Clocks and statuses were checked Nov 2026. Re-verify a row before filing under it.

## EU / UK

| Regime                      | Trigger                                                                                | Recipient                                   | Deadline (clock start)                                                                           | Owner                 | Source                                                                                                                                            |
| --------------------------- | -------------------------------------------------------------------------------------- | ------------------------------------------- | ------------------------------------------------------------------------------------------------ | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| GDPR Art. 33                | Personal-data breach (C, I **or** A; ransomware counts) unless unlikely to risk rights | Lead/competent DPA                          | **72h from awareness**; information may be phased                                                | DPO / privacy counsel | `load_skill privacy-eu`                                                                                                                           |
| GDPR Art. 34                | High risk to individuals                                                               | Data subjects                               | Without undue delay                                                                              | DPO + comms           | same                                                                                                                                              |
| GDPR Art. 33(2)             | Processor suffers any breach                                                           | Controller                                  | Without undue delay (the DPA should pin hours)                                                   | Vendor owner          | `load_skill legal-contractual`                                                                                                                    |
| NIS2 Art. 23 (national law) | Significant incident                                                                   | National CSIRT / authority                  | **24h** early warning → **72h** notification → **1 month** final                                 | CISO / compliance     | `load_skill nis2-dora`                                                                                                                            |
| DORA                        | Major ICT-related incident (classify within 24h of detection)                          | Financial supervisor                        | **4h from classification and ≤24h from awareness** → 72h → 1 month                               | CISO + risk           | CDR (EU) 2025/301                                                                                                                                 |
| CRA Art. 14                 | Actively exploited vuln in your product, or severe incident affecting its security     | CSIRT + ENISA via Single Reporting Platform | **24h** → 72h → 14 days after fix (vuln) / 1 month (incident); also inform users                 | PSIRT                 | `load_skill eu-cra` (live since 11 Sep 2026)                                                                                                      |
| AI Act Art. 73              | Serious incident from a high-risk AI system                                            | Market-surveillance authority               | ≤15 days; ≤2 days for widespread infringement or critical-infra disruption; ≤10 days for a death | AI system provider    | `load_skill eu-ai-act` (from 2 Dec 2027 for Annex III)                                                                                            |
| UK GDPR                     | Personal-data breach                                                                   | ICO                                         | 72h from awareness                                                                               | DPO                   | verify: https://ico.org.uk/for-organisations/report-a-breach/                                                                                     |
| UK NIS Regs 2018            | NIS incident (OES / RDSP)                                                              | Competent authority                         | 72h                                                                                              | CISO                  | Pending Cyber Security and Resilience (NIS) Bill → 24h initial + 72h full report; **still in Parliament in Nov 2026**. `load_skill uk-cyber-regs` |

## United States

| Regime                                                   | Trigger                                                                                                        | Recipient                                  | Deadline (clock start)                                                                                             | Owner                           | Source                                                                                                           |
| -------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------ | ------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| SEC 8-K Item 1.05                                        | **Material** cyber incident                                                                                    | SEC / market                               | **4 business days from the materiality determination** (which must be made without unreasonable delay)             | CFO / GC / disclosure committee | `load_skill us-sectoral-regs`                                                                                    |
| SEC Reg S-P                                              | Sensitive customer info accessed, or reasonably likely accessed (broker-dealers, RIAs, funds, transfer agents) | Affected individuals                       | ≤30 days from awareness; service provider → covered institution ≤72h                                               | Compliance                      | Live: large firms 3 Dec 2025, small firms 3 Jun 2026. verify: https://www.sec.gov/files/34-100155-fact-sheet.pdf |
| NYDFS 500.17                                             | Reportable cybersecurity incident                                                                              | NY DFS                                     | **72h from determination**; extortion payment **24h**, written justification in 30 days                            | CISO                            | `load_skill us-sectoral-regs`                                                                                    |
| Banks (OCC/FDIC/FRB)                                     | Notification incident                                                                                          | Primary regulator                          | **36h** from determination; bank service provider → bank customers asap after a ≥4h disruption                     | CISO / compliance               | same                                                                                                             |
| NCUA (credit unions)                                     | Reportable cyber incident                                                                                      | NCUA                                       | 72h                                                                                                                | CISO                            | verify: 12 CFR 748.1(c)                                                                                          |
| HIPAA Breach Rule                                        | Breach of unsecured PHI (ransomware presumed unless LoProCo shown)                                             | Individuals; HHS; media if ≥500 in a state | **≤60 days from discovery**; HHS at the same time if ≥500, else annual log; BA → CE ≤60 days (contract it shorter) | Privacy officer                 | same                                                                                                             |
| GLBA Safeguards (FTC)                                    | Unencrypted info of ≥500 consumers (nonbank FIs)                                                               | FTC                                        | **≤30 days from discovery**                                                                                        | Qualified individual            | same                                                                                                             |
| State breach laws                                        | Unencrypted PI elements per state, by residency of each person                                                 | Residents, AGs above thresholds, CRAs      | Strictest is **30 days** (CA, CO, FL, WA, ME); CA AG 15 days after consumers                                       | Privacy counsel                 | `load_skill privacy-us`                                                                                          |
| DFARS 252.204-7012                                       | Cyber incident affecting covered defense info / CDI systems                                                    | DoD via DIBNet (DC3)                       | **72h from discovery**; keep images 90 days; submit malware to DC3                                                 | FSO / contracts                 | `load_skill us-sectoral-regs`                                                                                    |
| FedRAMP / agency contracts                               | Incident on a federal cloud system                                                                             | Agency + CISA                              | Typically **1h** from identification (contract/ICP)                                                                | CSP security                    | verify: https://www.fedramp.gov/                                                                                 |
| CIRCIA (**pending**)                                     | Covered incident / ransom payment                                                                              | CISA                                       | 72h / 24h, **not in force**; final rule at OIRA since 1 Oct 2026                                                   | (n/a yet)                       | `load_skill us-sectoral-regs`                                                                                    |
| TSA SD Pipeline-2021-01G; rail SD 1580/1582-21-01 series | Cybersecurity incident                                                                                         | CISA                                       | **72h from identification** (cut from 24h in SD 01F, Oct 2025); supplemental info within 24h of becoming available | Cyber coordinator               | 01G effective 16 Jan 2026–15 Jan 2027; check the current letter. `load_skill ot-compliance`                      |
| NERC CIP-008                                             | Reportable cyber security incident / attempt                                                                   | E-ISAC + CISA                              | **1h** from determination (reportable); end of next calendar day (attempt)                                         | CIP senior manager              | verify: https://www.nerc.com/                                                                                    |

## APAC

| Regime                              | Trigger                                                             | Recipient                               | Deadline (clock start)                                                                                               | Owner               | Source                                                                                                            |
| ----------------------------------- | ------------------------------------------------------------------- | --------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ------------------- | ----------------------------------------------------------------------------------------------------------------- |
| India CERT-In Directions (Apr 2022) | Listed incident types on systems in India                           | CERT-In                                 | **6h from noticing**                                                                                                 | CISO / India entity | Still in force; restated in the Feb 2026 space framework. verify: https://www.cert-in.org.in/                     |
| India DPDP Rules r.7                | **Any** personal-data breach (no risk threshold)                    | Data Protection Board + data principals | Without delay, then a detailed report in **72h**                                                                     | DPO                 | **Not yet applicable**: applies ~May 2027 (18 months after the 13 Nov 2025 notification); verify the Gazette date |
| Australia SOCI Pt 2B                | Critical cyber incident: significant impact / relevant impact       | ASD's ACSC                              | **12h** (a verbal report needs a written one within 84h) / **72h** (verbal → written within 48h)                     | Responsible entity  | verify: https://www.cyber.gov.au/                                                                                 |
| Australia NDB (Privacy Act)         | Eligible data breach (likely serious harm)                          | OAIC + individuals                      | Assess within 30 days of suspicion, then notify as soon as practicable                                               | Privacy officer     | verify: https://www.oaic.gov.au/                                                                                  |
| Australia Cyber Security Act Pt 3   | Ransomware/extortion **payment** (turnover ≥ AUD 3M or SOCI entity) | ASD                                     | **72h from payment**                                                                                                 | CFO / GC            | `load_skill ransomware-response`                                                                                  |
| Singapore PDPA / Japan APPI         | Notifiable breach                                                   | PDPC / PPC                              | SG: 3 days after the assessment (assess within 30 days). JP: prompt report, final in 30 days (60 for malicious acts) | DPO                 | `load_skill privacy-apac`: also China CAC 1h/4h, Singapore CSA (CII) 2h, MAS 1h, Korea 72h, APRA 72h              |

## Contractual (often the tightest clock)

| Source                                                | Trigger                                    | Recipient                          | Typical deadline                                              | Owner                                         |
| ----------------------------------------------------- | ------------------------------------------ | ---------------------------------- | ------------------------------------------------------------- | --------------------------------------------- |
| Customer DPAs / MSAs / security addenda               | As defined: often "suspected" incident     | Each customer                      | 24–72h from discovery                                         | Legal + account owners                        |
| Card brands / acquirer (PCI DSS)                      | Suspected cardholder-data compromise       | Acquirer → brands                  | Immediately, per brand rules; forensic investigation by a PFI | Payments owner (`load_skill pci-dss`)         |
| Cyber insurance policy                                | Any circumstance that may become a claim   | Carrier / breach coach             | "As soon as practicable"; late notice can void cover          | Risk manager (`load_skill legal-contractual`) |
| Upstream regulated customers (DORA/NIS2 supply chain) | Incident affecting the service you provide | Customer, who files its own report | Hours: sized to their 4h/24h clocks                           | Service owner                                 |

## Clock-start events (where teams get it wrong)

- **Awareness** (GDPR, NIS2): reasonable certainty that an incident hit personal data or
  service. A short investigation is allowed, not an indefinite one (EDPB Guidelines 9/2022).
- **Determination** (SEC materiality, NYDFS, banks): starts at the decision, but the
  decision cannot be slow-walked. Minute when and why it was made.
- **Classification** (DORA) / **identification** (TSA, NERC): a technical call. Pre-agree who makes it.
- **Discovery** (HIPAA, GLBA, DFARS): known, or would have been known with reasonable
  diligence, including by your agents and BAs.
- **Payment** (Australia, NYDFS, CIRCIA when live): starts when you pay, not when you are attacked.

## Build the per-incident matrix in the first hour

1. **Fact sheet** (15 min): affected legal entities and licences, sectors, products
   shipped, data types, where the data subjects live, systems hit, whether exfiltration
   is plausible, whether payment is on the table. Pull from a pre-built regulatory
   register; don't research the law mid-incident.
2. **Filter the master tables above.** Mark each row Y / N / **? (treat as Y until ruled
   out)**. Add the contract rows for your top customers and the insurer.
3. **Timestamp each start event** in UTC and local time, compute the deadline, and put
   every deadline in the war-room timeline (`load_skill incident-response-leadership`).
4. **Sort by deadline.** The usual front of the queue: NERC/FedRAMP 1h, DORA 4h,
   CERT-In 6h, SOCI 12h, then the 24h clocks (NIS2, NYDFS payment, CRA, contracts).
5. **Name an owner and a backup** per row, with the template, portal and credentials located.
6. **Re-run the filter on every new fact**: confirmed exfiltration (GDPR Art. 34, state
   laws, HIPAA), a materiality call (SEC), a payment (AU, NYDFS), a new country.
7. **Log decisions not to notify** with the reasoning. GDPR Art. 33(5) requires a record of
   every breach, reported or not.

Write it with `write_file /work/notification-matrix.md`:

| #   | Regime | Applies (Y/N/?) | Trigger met? | Start event + UTC | Deadline UTC | Recipient + channel | Owner / backup | Status | Ref |
| --- | ------ | --------------- | ------------ | ----------------- | ------------ | ------------------- | -------------- | ------ | --- |

## Pitfalls

- File staged reports with what you know. GDPR, NIS2, DORA and CRA all expect updates;
  waiting for complete facts is how deadlines get missed.
- One story everywhere: SEC, regulators, customers and insurer compare filings, so keep
  them consistent.
- A law-enforcement delay exists only where the regime provides one (state laws; SEC via
  the US Attorney General). GDPR and NIS2 have no such pause.
- NIS2 is national law: a multi-country group may owe several national reports.
- Don't file under regimes that aren't live (CIRCIA, the UK Bill, India DPDP r.7 before
  ~May 2027, the EU Digital Omnibus single entry point), but put their dates in the diary.
- Ransomware with no exfiltration is still a GDPR breach and is presumed a HIPAA breach.
