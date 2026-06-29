---
name: social-engineering-defense
description: Defend against phishing/vishing/smishing/pretexting/BEC — out-of-band verification for money and credentials, identity verification, reporting culture, and DMARC/SPF/DKIM/MFA backstops.
---

# Social-engineering defense

Social engineering attacks the human, so the defense is **process + technical
backstops**, not "be more careful." In the AI/deepfake era, content-based
detection ("does this look fake?") is losing — shift defense to **process
enforcement** that holds regardless of how convincing the lure is. This is the
defensive pair to marq's offensive social-eng/people-OSINT tooling; for the
program/culture side see `load_skill security-awareness`.

## Threat taxonomy (quick reference)

- **Phishing** — mass email lure → credentials/malware/payment.
- **Spear-phishing / whaling** — targeted, recon-driven, often impersonating an exec.
- **Vishing** — phone/voice, now with cloned-voice deepfakes; ~30% of orgs report incidents.
- **Smishing** — SMS/iMessage/WhatsApp lures, often "package"/"toll"/"boss needs you."
- **Pretexting** — fabricated scenario to extract info or access (IT support, auditor, new vendor).
- **BEC** — compromised or look-alike business email → fraudulent wire/payroll/invoice change.
- verify (AI-era trends): https://hoxhunt.com/blog/business-email-compromise-statistics

## The core control: out-of-band verification

The single highest-value defense for money and access. Make it mandatory policy,
not judgment:

- **Any payment instruction, new payee, bank-detail change, or payroll change**
  → verify by **calling a number you already have on file** (vendor master /
  internal directory), **never** a number from the email/text/caller. Callback,
  don't call-forward.
- **Credential/MFA/account changes** (reset, device enrollment, "I'm locked out")
  → verify the requester through a second channel before acting.
- **Dual authorization** above a defined threshold; **mandatory waiting period**
  for first-time or changed payees.
- Treat urgency + secrecy + channel-switch as the red flag — that's the BEC
  signature, regardless of how legitimate the message reads.
- verify: https://www.cisa.gov/ (BEC guidance) and FBI IC3 BEC advisories.

## Caller / identity verification (anti-vishing, anti-pretext)

- Help desk: verify identity with something **not publicly knowable and not on
  the badge** (e.g. ticket-driven callback to the number in HR records, manager
  confirmation) — never DOB/employee-ID/last-4 alone, all of which leak.
- Establish a **duress / verification phrase** or internal callback protocol for
  high-stakes requests; deepfake voice/video can now pass "it sounded like them."
- Default posture: a real exec under real pressure will tolerate a 2-minute
  verification. Anyone who won't is the tell.
- Vendors/IT impersonation: confirm via the relationship's known contact, not
  contact details supplied in the request.

## Reporting culture (the scalable detector)

- One person reporting protects everyone who already clicked — reporting is the
  human SOC sensor. Make it one-click (**Report Phish** button) and blameless.
- Measure **report rate** and **time-to-report**, not click rate alone
  (`load_skill security-awareness` for the metrics rationale).
- Close the loop: reports feed triage/IR; tell reporters what happened so the
  behavior reinforces. Silence kills reporting.
- Wire reports to the SOC → `load_skill incident-response-leadership` on confirmed compromise.

## Technical backstops

- **DMARC at p=reject** (with aligned SPF + DKIM) — stops domain spoofing and
  look-alike-from-your-domain. CISA BOD 18-01 mandates p=reject for US federal
  domains; PCI DSS v4.0 requires DMARC for card-data orgs (since Mar 2025).
  Limit: DMARC does **not** stop a _real compromised account_ or display-name
  spoofing from a different domain — layer the rest.
  verify: https://dmarcreport.com/blog/business-email-compromise-bec-scams-take-new-dimension-with-multi-stage-attacks/
- **Phishing-resistant MFA** (FIDO2/passkeys > push > SMS) on all email and
  remote access — defeats credential phish and limits BEC account takeover.
- **Mail hygiene**: external-sender banners, look-alike/cousin-domain detection,
  newly-registered-domain blocking, attachment sandboxing, link rewriting.
- **Lookalike-domain monitoring + takedown**; register obvious typo-squats.
- **Conditional access / impossible-travel** + session-token protection to catch
  post-phish logins.
- **Least privilege + segmentation** so a single compromised user isn't game-over.

## Decision rule (frontline)

Money or access requested + urgency or channel-switch or secrecy → **stop,
verify out-of-band on a known-good number, before acting.** No exceptions for
seniority. Process beats content detection every time.
