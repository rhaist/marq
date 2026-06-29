---
name: threat-hunting
description: Hypothesis-driven, ATT&CK-anchored threat hunting — the hunt loop, pyramid-of-pain hunt value, data prerequisites, and turning a hunt into a detection.
---

# Threat hunting

Hunting is the proactive search for adversary activity that evaded existing
detections. It assumes compromise and tries to disprove it. Not alert-chasing,
not running a scanner — a _hypothesis_ tested against data. Use the PEAK loop
(Plan → Execute → Analyze → Knowledge/Communicate).
verify: https://huntbook.predefender.com/part-1/frameworks/threathunting/peak/

## The hunt loop

1. **Form a hypothesis** — specific, testable, falsifiable, ATT&CK-mapped.
   Good: "An adversary is using T1053.005 scheduled tasks for persistence on
   workstations." Bad: "Look for bad stuff." Sources of hypotheses:
   - Threat intel relevant to _your_ sector/crown jewels (`load_skill
actor-ttp-attribution` to turn an actor into testable TTPs).
   - A fresh ATT&CK technique / public exploit / news intrusion.
   - A known detection gap (from `siem-soar` coverage map) or an anomaly baseline.
2. **Scope the data** — name the exact log source + fields needed _before_
   querying (see prerequisites). No data = the hunt is impossible, not negative.
3. **Execute** — query the data. Three PEAK hunt types:
   - Hypothesis-driven (intel → TTP → search).
   - Baseline/anomaly (build "normal", flag deviation — new parent-child process
     pairs, rare LOLBins, first-seen autoruns).
   - Model-assisted (clustering/ML over a feature, e.g. beacon jitter).
4. **Analyze** — investigate hits. Stack-count (least-frequency-of-occurrence:
   rare = interesting), pivot on the survivors, separate benign-rare from
   malicious-rare. Confirm or refute the hypothesis explicitly.
5. **Conclude + hand off** — every outcome is valuable:
   - Found evil → declare an incident: `load_skill incident-response`.
   - Found a gap → write a detection: `load_skill detection-engineering`.
   - Found nothing → record the hunt, data coverage, and queries so it's
     repeatable and counts as assurance.

## Anchor to ATT&CK (v18)

- Map every hypothesis to a technique id; it makes hunts comparable, communicable,
  and feeds a coverage heatmap. v18 (Oct 2025) replaced Data Sources with
  Detection Strategies + Analytics — read the technique's Analytics for the exact
  log sources/data components to query.
  verify: https://attack.mitre.org/matrices/enterprise/
- Prioritize techniques in your threat model (sector, actors, exposed tech), not
  the whole matrix.

## Pyramid of pain — hunt where it hurts the adversary

Hunt up the pyramid: hash < IP < domain < network/host artifact < tool < **TTP**.
Hashes/IPs rotate for free and your hunt expires the next day. Behaviours (TTPs)
cost the adversary the most to change, so a TTP hunt yields a _durable_ detection.
Prefer "how they persist/move" over "this one C2 IP." Pull indicators to pivot
from `load_skill ioc-pivoting` / `indicator-enrichment` (threat-intel domain).

## Data prerequisites (no telemetry, no hunt)

- Endpoint: process creation w/ command line + hashes + parent (Sysmon 1 / EDR),
  network, image-load, registry, scheduled-task, service install, named-pipe.
- Identity: authN success/fail, token/grant, privileged-group changes.
- Cloud: control-plane audit (CloudTrail / Azure Activity / Graph).
- Network: DNS query logs, proxy, NetFlow/Zeek.
  Check coverage _first_. A "clean" hunt over data you don't actually collect is a
  false negative — log it as a visibility gap, not an all-clear.

## Turn a hunt into a detection

The hunt loop only pays off if findings feed back. For each confirmed-evil or
high-value pattern:

- Generalize the one-off query into a behaviour rule (Sigma), not an IOC match.
- Define expected FP sources + a triage runbook, then ship via
  `load_skill detection-engineering`.
- Each new detection retires that manual hunt and frees you to hunt the next gap —
  that feedback loop is the whole point.

## marq tie-in

This is an offensive box, not a hunt platform — but use it to _validate_ a
detection hypothesis end-to-end: emulate the TTP (run the technique with marq
tooling), then confirm the data source/rule would have caught it. `report_finding`
any technique you execute that produced no telemetry — a proven blind spot is a
finding. `run_shell` for ad-hoc log greps over artifacts staged in `/work`.
