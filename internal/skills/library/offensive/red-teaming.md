---
name: red-teaming
description: Run an objective-driven adversary-emulation engagement — assumed-breach scoping, ATT&CK emulation plans, C2 OPSEC, deconfliction, and regulated TLPT frameworks (TIBER-EU/CBEST/FedRAMP).
---

# Red teaming

Authorized engagements only. Confirm scope first: call `server_info` and read
`marq://authorization`. A red team simulates a _named threat actor against
specific objectives_ ("flags") to test detection + response — not a pentest's
breadth-first vuln sweep. If the goal is coverage/patch-list, you want a
pentest; reach for `methodology` and the `vuln/*` skills instead.

## 1. Pentest vs red team — pick the right job

- **Pentest** = enumerate as many vulns as possible across a defined scope;
  success = findings list; detection is out of scope.
- **Red team** = achieve an objective (domain dominance, access crown-jewel
  data, fraudulent transaction) the way a real adversary would; success =
  did we reach the flag, and _did the blue team see us_. Stealth is a control
  under test.
- **Assumed breach** is now the default starting posture (full external-only
  red teams burn weeks on initial access the org already knows is possible).
  Negotiate a planted foothold / phished workstation so the exercise spends
  budget on the post-exploitation chain that actually exercises detection.

## 2. Engagement structure

1. **Scoping & ROE** — objectives/flags, in-scope networks+identities, hard
   no-go list (safety-critical OT, patient/payment systems, prod data
   destruction), test window, legal authorization signed _before any action_.
2. **Threat-intel-led scenario** — pick the actor(s) plausibly targeting this
   org's sector/geo; derive their TTPs (`load_skill actor-ttp-attribution`).
3. **Emulation plan** (see §3) — the actor's kill chain as concrete steps.
4. **Execution** — initial access → recon → priv-esc → lateral → objective,
   each step logged with timestamp for later purple replay.
5. **Deconfliction & reporting** — attack narrative + ATT&CK heatmap of what
   blue detected vs missed; hand straight into a purple loop
   (`load_skill purple-teaming`).

## 3. Build the emulation plan from ATT&CK

- Anchor on MITRE's adversary-emulation methodology and the open emulation
  library: https://attack.mitre.org/resources/adversary-emulation-plans/ and
  https://github.com/center-for-threat-informed-defense/adversary_emulation_library
- For each tactic, choose the _specific technique the target actor uses_, not a
  generic one — e.g. emulate their real persistence (T1053.005 vs T1547.001),
  their real C2 protocol, their real exfil path. Web-fetch the technique page
  `https://attack.mitre.org/techniques/T####/` to confirm current sub-techniques.
- Automate the repeatable atomic steps with CALDERA
  (https://github.com/mitre/caldera) or Atomic Red Team so they're replayable;
  keep the creative objective-seeking manual.
- Produce an ATT&CK Navigator layer of planned techniques — this becomes the
  coverage scorecard.

## 4. C2 & OPSEC

- **Infra**: redirectors in front of every team-server (never point an implant
  at the C2 directly); categorized domains; per-engagement TLS certs; separate
  channels for staging vs interactive vs exfil so one burned IOC doesn't
  collapse the op.
- **Beacon hygiene**: jittered, long sleep intervals; match callback timing to
  the actor being emulated; malleable profiles that blend with normal traffic.
- **On-host OPSEC**: prefer living-off-the-land (LOLBins, built-in admin tools)
  over dropping tooling; minimize disk artifacts; clean up created accounts/
  tasks at end-of-op. Assume EDR telemetry — the point is to test whether it
  _alerts_, so don't disable it, evade it and record what got through.
- Tag every implant/op with the engagement ID so blue can deconflict a real
  intrusion from your activity.
- **Payload staging**: `donut` turns a .NET assembly / EXE / DLL into
  position-independent shellcode for in-memory loaders (avoids dropping the PE to
  disk). Generate, then deliver via your loader of choice — authorized scope only.

## 5. Rules of engagement & deconfliction

- **Deconfliction line**: a named control on both sides + a code word. Blue
  calls it to ask "is this you?"; you answer truthfully and instantly so a
  _real_ attacker during your window isn't dismissed as the red team.
- **Stop conditions**: pre-agreed triggers (instability, safety risk, evidence
  of a genuine breach) that pause the op. Escalation contacts 24/7.
- **No collateral**: no DoS, no destroying data, no touching the no-go list,
  no exfil of real regulated data beyond an agreed canary token. Cross-border
  pitfall: even authorized "exfil" of loot/evidence to a home-region team server
  can breach data-residency law on an APAC engagement — keep captured data in the
  client's jurisdiction unless the ROE says otherwise.
- If you trip something fragile, **stop and report up** — don't improvise on
  production.

## 6. Regulated TLPT frameworks (verify current before citing)

- **TIBER-EU** (ECB) — threat-intel-based ethical red-teaming; updated
  **11 Feb 2025** to align with the DORA RTS on Threat-Led Penetration Testing.
  Notably now **makes purple-teaming mandatory** and harmonizes terminology
  with DORA. Phases: Preparation → Testing (TI + Red Team) → Closure.
  https://www.ecb.europa.eu/paym/cyber-resilience/tiber-eu/html/index.en.html
  (2025 SSM implementation guide:
  https://www.bankingsupervision.europa.eu/ecb/pub/pdf/ssm.supervisory_guide202511.en.pdf)
- **CBEST** (Bank of England / PRA / FCA) — intel-led assessment for UK
  financial firms+FMIs; TISP-derived scenarios of state actors, OCGs, insiders.
  STAR-FS is the lighter complementary scheme. 2025 thematic findings published
  Jan 2025. verify:
  https://www.bankofengland.co.uk/financial-stability/operational-resilience-of-the-financial-sector/cbest-threat-intelligence-led-assessments-implementation-guide
- **FedRAMP Rev 5** — control **CA-8(2)** mandates an **annual red team
  exercise** for CSPs (test plan + report, 3PAO-validated), distinct from the
  standard pentest: it tests people/process/tech detection+response, not just
  "can it be breached." Still current mid-2026, but the **FedRAMP 20x** transition
  is underway (consolidated rules ~June 2026, applications from July 2026) — check
  which regime the CSP is under before citing. verify:
  https://help.fedramp.gov/hc/en-us/articles/28907820003227-CA-8-2-requires-Red-Team-exercises
- Common thread: independent threat intel drives the scenario, a control group
  oversees, and the deliverable is resilience evidence — not a bug list.

## Anti-patterns

- Running a "red team" that's really a noisy pentest — if you never test
  whether blue detects you, it isn't one.
- Skipping deconfliction — guarantees a real breach gets ignored or your op
  gets escalated as an incident.
- Emulating a generic attacker when the framework (TIBER/CBEST) requires a
  _specific intel-led_ actor scenario.
- Treating EDR evasion as the goal rather than the measurement — disabling
  defenses destroys the data the exercise exists to produce.
