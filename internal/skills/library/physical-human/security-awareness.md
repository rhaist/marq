---
name: security-awareness
description: Build a behavior-change awareness program (not an annual checkbox) — role-based training, ethical phishing simulation, report-rate-first metrics, positive reinforcement, and just-in-time nudges.
---

# Security awareness (behavior change, not compliance theater)

Goal is **measurable behavior change**, not completion certificates. An annual
e-learning module satisfies an auditor and changes nothing. Design for
continuous, role-relevant, in-the-moment learning, and measure outcomes you can
tie to risk. This is the human-layer defense; pair it with the technical
backstops in `load_skill social-engineering-defense`.

## Program model

- **Continuous > annual**: short, frequent touches (monthly micro-learning,
  in-context nudges) beat one long yearly dump. Twelve months of continuous
  training cut global phish-prone rate ~33% → ~4% in industry data.
  verify: https://www.knowbe4.com/resources/reports/phishing-by-industry-benchmarking-report
- **Role-based**: tailor content to risk surface. Finance/AP → BEC & payment
  fraud; execs/admins → spear-phish & credential theft; devs → secrets, supply
  chain; HR → data handling & lure-rich inbound. Generic content underperforms.
- **Risk-tiered**: high-risk roles and repeat-clickers get more frequent,
  harder, supported reps — not the same baseline as everyone.
- **Culture, not blame**: the program's job is to make reporting easy and
  reputationally safe. Shame produces hiding, which is worse than clicking.

## Phishing simulation — design & ethics

- **Adaptive difficulty**: skilled users get progressively harder lures; new or
  high-risk users get clearer indicators and support. Don't sandbag everyone to
  the same level. verify: https://www.brside.com/blog/7-phishing-simulation-best-practices-the-2025-guide
- **Ethical guardrails (non-negotiable)**:
  - Failure = a teachable moment (just-in-time landing page), never discipline.
  - Simulation results are **separate from HR performance evaluation**.
  - Never weaponize sensitive personal data; lures use only fair, plausible pretexts.
  - Avoid cruel themes (fake bonuses, layoff notices, bereavement) — NDSS 2025
    research shows severe-consequence/incentive lures cause backlash, not learning.
- **Privacy/legal**: in EU/works-council contexts, engage the works council
  early, run a DPIA / Legitimate Interests Assessment, pseudonymize, short
  retention, transparent notice. Track cohorts, not individuals, for reporting.
  verify: https://autophish.io/blog/privacy-friendly-phishing-training-works-councils-consent-and-gdpr-essentials
- **Announce the program exists** (not each test) — transparency builds trust;
  gotcha culture destroys it.

## Metrics that matter

- **Report rate is the headline metric, not click rate.** Click rate measures
  who failed; **report rate measures who actively defended** — the behavior you
  actually want at scale. Strong programs hit ≥ 60% report rate.
  verify: https://hoxhunt.com/blog/security-awareness-metrics
- **Why report > click**: one reporter triggers triage that protects everyone
  who _did_ click; a low click rate with zero reports means the real phish lands
  silently. Reporting is the scalable detection control humans provide.
- **Time-to-report**: median minutes from delivery to first report. Shorter =
  smaller attacker dwell window. Trend it down.
- **Repeat-offender / resilience rate**: who fails repeatedly despite training —
  route to coaching, not punishment. Track the _trend_ per cohort over time.
- **Avoid vanity metrics**: training completion %, quiz scores, and raw click
  rate alone don't predict breach reduction.
- Tie a north-star to incidents: % reduction in real phishing-related incidents
  over 12 months (behavior-based programs report ~50%).

## Positive reinforcement & just-in-time nudges

- Reward reporters: leaderboards, thank-the-reporter automation, "you caught a
  real one" callouts. Recognition outperforms fear.
- **Just-in-time nudges** — teach at the moment of risk, where retention is
  highest: external-sender email banners, "this link is newly registered"
  warnings, re-auth prompts on sensitive actions, paste-into-untrusted-site
  warnings. Context beats a classroom three months earlier.
- Make the right action one click: a prominent **Report Phish** button wired to
  the SOC (closes the loop with `load_skill incident-response-leadership`).

## Anti-patterns

- Annual-only training; identical content for all roles.
- Punishing clickers / publishing names — guarantees under-reporting.
- Optimizing click rate to zero while ignoring report rate.
- Lures using distress/financial-harm themes; running tests Legal/works-council
  hasn't cleared.
