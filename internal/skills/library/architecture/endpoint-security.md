---
name: endpoint-security
description: Endpoint architecture decisions — EDR vs XDR vs MDR, CIS hardening, app allowlisting, mobile (MDM/MAM/BYOD), and IoT/OT with the Purdue model and IEC 62443.
---

# Endpoint & device security architecture

The endpoint is where the user, the data, and the attacker all meet. Antivirus
(signature-based) is dead as a primary control — modern detection is
behavior-based. The 2025-2026 reality: attackers actively **blind the sensor**
(BYOVD driver attacks, EDR-killers) or **skip the endpoint entirely** (SaaS/
identity-only intrusions like the Snowflake/UNC5537 campaign). Architect so you
survive a silenced or absent endpoint agent.

## EDR vs XDR vs MDR — what each actually is

|         | What it is                                                                                      | Scope                        | Buy when                                                 |
| ------- | ----------------------------------------------------------------------------------------------- | ---------------------------- | -------------------------------------------------------- |
| **EPP** | Endpoint Protection Platform — the prevention layer (NGAV, device control)                      | One endpoint                 | Table stakes; usually bundled with EDR.                  |
| **EDR** | Detection + response _on the endpoint_ — telemetry, behavioral detection, isolate/kill/rollback | Endpoints only (depth)       | You need host-level visibility + response. Baseline.     |
| **XDR** | Correlates telemetry **across** endpoint + identity + email + cloud + network                   | The attack surface (breadth) | Attacks move between domains or bypass the host.         |
| **MDR** | A **service**, not a product — a vendor SOC runs detection/response 24/7 for you                | Your tools + their humans    | You lack a 24/7 SOC (most orgs). Human-led is the point. |

**Decision logic (verify current market via web — it shifts yearly):**

- EDR = depth on the host; XDR = breadth across domains. EDR is right when the
  attack lives on a host; XDR when it moves between domains or never touches one.
- MDR is orthogonal: a delivery model layered on EDR/XDR. "MXDR" = MDR over XDR
  telemetry, but Gartner is firm it must stay **human-led** — automated detections
  plus human judgment + business context, not a pure automation play.
- Pragmatic 2026 default for a mid-size org: **EDR/XDR product + MDR service**. The
  tool without 24/7 humans watching it is alerts no one reads at 3am.
- Verify the current landscape/Gartner positioning before recommending a vendor:
  https://www.crowdstrike.com/en-us/cybersecurity-101/endpoint-security/edr-vs-mdr-vs-xdr/
  and Gartner MDR Market Guide / EPP Magic Quadrant (current year).
- Resilience requirements regardless of tier: **tamper protection on**, agent-health
  alerting (a sensor that goes silent is an event), driver allow/blocklist against
  BYOVD, and identity/cloud telemetry so you still see the intrusion that skips the host.

## Hardening via CIS Benchmarks

- Baseline against **CIS Benchmarks** (per-OS/app, consensus configs) — Windows,
  macOS, Linux, browsers, cloud. They ship with **L1** (safe, deploy broadly) and
  **L2** (defense-in-depth, may break workflows) profiles. Start L1 fleet-wide, L2
  on high-value/Restricted-data endpoints. https://www.cisecurity.org/cis-benchmarks
- Or DISA STIGs in gov/defense contexts. Microsoft Security Baselines for Windows/Entra.
- **Measure drift continuously** — a hardened gold image rots. Use CIS-CAT / cloud
  posture tooling to score against the benchmark and alert on regression. A one-time
  hardening is a snapshot, not a control.
- Map hardening to framework controls via `load_skill control-mapping` (CIS Control 4,
  NIST CM-6/CM-7 secure config).

## Application control / allowlisting

- **Allowlisting > blocklisting.** Default-deny execution beats chasing bad hashes.
  This is the single highest-leverage control against ransomware/LOLBins on servers
  and fixed-function hosts.
- Tools: Windows App Control for Business (WDAC) / AppLocker, macOS notarization +
  MDM, Linux fapolicyd. Pair with constrained PowerShell and LOLBin (LOLBAS) blocking.
- Deploy in **audit mode first**, build the allowlist from real usage, then enforce —
  same monitor-then-block discipline as DLP. Block-mode-on-day-one breaks the business.
- Easiest wins: locked-down kiosks, servers, OT/HMI hosts (stable software set).
  Hardest: developer laptops (constantly new binaries) — manage by exception/publisher rule.

## Mobile — MDM vs MAM, BYOD

- **MDM** (Mobile Device Management) — manages the **whole device**: enrollment,
  config, full wipe. Right for **corporate-owned** devices.
- **MAM** (Mobile App Management) — manages only the **corporate apps/data** (app
  protection policies, selective wipe of work data, container). Right for **BYOD** —
  you can't (legally/ethically) full-wipe an employee's personal phone or see their photos.
- **BYOD rule**: default to **MAM / app-protection + Conditional Access**, not full MDM,
  on personal devices. Enforce: managed app container, no copy/paste to personal apps,
  app-level PIN/biometric, block jailbroken/rooted, selective wipe on leaver.
- Tie device posture into access: a non-compliant device fails Conditional Access —
  the endpoint becomes an input to the identity decision (`load_skill iam`).

## IoT / OT — different physics, different rules

IT security instincts get people hurt in OT. Here **availability and safety
outrank confidentiality** (inverted CIA), uptime is measured in years, and you
can't just patch or reboot a turbine.

- **Purdue model** — the reference segmentation for ICS. Know the levels:
  - L0 field devices (sensors/actuators) · L1 controllers (PLC/RTU) · L2 supervisory
    (SCADA/HMI) · L3 site operations/historian · **L3.5 DMZ** (the IT/OT boundary —
    the most important zone you'll design) · L4/L5 enterprise IT.
  - The hard rule: **no direct L4->L1 path.** All IT/OT traffic brokered through the
    L3.5 DMZ (jump hosts, data diodes, replicated historian). Verify model details:
    https://www.sentinelone.com/cybersecurity-101/cybersecurity/what-is-the-purdue-model/
- **ICS constraints that change the playbook:**
  - Legacy/insecure-by-design protocols (Modbus, DNP3, no auth/encryption) — you
    can't fix the protocol; you isolate it.
  - Fragile devices — an active vuln scan can crash a PLC. Use **passive/OT-aware**
    monitoring (Nozomi, Dragos, Claroty), not your IT Nessus scan against L1.
  - Can't patch on IT cadence — **compensating controls** (segmentation, allowlisting,
    monitoring) are the primary defense, not patching.
- **IEC 62443 / ISA 62443** — the OT security standard. Purdue says _how to segment_;
  62443 says _what security each zone requires_ (verify current parts):
  - **Zones & conduits** — group assets by security need (zones), control the comms
    paths between them (conduits). This is segmentation as a formal model.
  - **Security Levels SL 1-4** — SL1 (casual/accidental) -> SL4 (nation-state). Assign
    a **target SL** per zone by consequence, then meet 62443-3-3 system requirements
    for that SL. Don't gold-plate L0 to SL4. https://www.dragos.com/blog/isa-iec-62443-concepts
  - 62443-2-1 = OT security program/management; 62443-4-2 = component requirements
    for procurement ("our PLC vendor must meet SL-C 2").
- Decision: for any OT engagement, segment to Purdue + L3.5 DMZ first, monitor
  passively, assign 62443 target SLs by safety consequence, and treat the OT
  incident plan as separate from IT — recovery means safe physical state, not just
  clean disks (`load_skill incident-response-leadership`).
