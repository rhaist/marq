---
name: ot-assessment
description: Safely assessing an OT/ICS environment — why standard scanning is a hazard, which marq tools are safe below the IT/OT boundary, passive-first method, per-activity authorization, and where to do active testing instead.
---

# Assessing OT / ICS safely

**The default marq toolkit is unsafe below the IT/OT boundary.** In IT a scan is
free; in OT it is a physical-risk action. Guidance going back to the CPNI/DHS
ICS assessment guide is blunt: _many field devices currently deployed will crash
or become unresponsive from a simple scan_, and such tools _should never be used
on a production system_. CISA repeated it in 2026: typical active scanning
should not be attempted without deep awareness of the system, as it may knock a
legacy device offline. The mechanism is mundane — improper input validation was
the top ICS vulnerability class in INL's lab testing.

Engineering context: `load_skill ot-ics`. Adversary context:
`load_skill ot-threat-landscape`.

## Rules of engagement — settle before any packet

- **Confirm authorization and scope** (`server_info`) — then narrow it further.
  OT needs **per-activity permission, not a blanket scope**: obtain specific
  permission for each testing activity _before it is initiated_. If a component
  cannot be temporarily isolated for a test, that attack vector waits until it
  can. Blanket "internal network" authorization does not authorize touching L1.
- **Write rules of engagement that name the target**: production, or a credible
  substitute — a backup/secondary control system, a test network, a stand-alone
  rig. Prefer the substitute.
- **Hands-on-keyboard belongs to the plant.** ICS-CERT's recommended practice is
  that local personnel perform tests on an active control system _at the
  direction of_ the assessment team. You advise; the engineer executes.
- **Define abort conditions and who calls them** in writing. Process upset,
  alarm floods and comms loss during a test are the signals — agree in advance
  what happens next.
- **Never fuzz production.** The goal of fuzzing is to cause a crash; that makes
  it categorically unacceptable against a live ICS.
- Record the whole thing — every marq call is audit-logged, which is exactly the
  evidence you want when a plant asks what you did at 14:03.

## marq tools — what is safe where

| Tool                                     | L4/IT & IDMZ       | Below the IDMZ (L0–L3)                                                            |
| :--------------------------------------- | :----------------- | :-------------------------------------------------------------------------------- |
| `shodan_search`, `censys_search`         | safe               | **safe — the preferred first move** (third-party data, zero packets to the plant) |
| `whois_lookup`, `dns_lookup`, `asnmap`   | safe               | safe (passive)                                                                    |
| `nmap`, `masscan`, `naabu`               | normal care        | **do not run** — crash / DoS of critical services                                 |
| `nuclei`, `nikto`, `ffuf`, `feroxbuster` | IT web apps only   | **do not run** against an HMI or embedded web server                              |
| `snmp_walk`, `snmp_check`                | care               | **do not run** unattended; SNMP stacks on field gear are fragile                  |
| `snmp_brute`, `hydra`, `netexec`         | authorized IT only | **never** — lockouts and crashes on control gear                                  |
| `sqlmap`, `dalfox`, `sstimap`            | IT web apps only   | **never**                                                                         |
| `searchsploit`, `capa`, `yara_scan`      | safe (offline)     | safe (offline analysis, no target contact)                                        |

Packet capture from a **SPAN/mirror port** is the accepted safe alternative for
live analysis — traditionally Wireshark/tcpdump, which marq does not wrap, so
capture on the plant side and analyse the artefacts through the `files` tools.

## Method — passive first, outside-in

1. **External exposure, without touching the plant.** `shodan_search` /
   `censys_search` against your own ranges and org. CISA's exposure-reduction
   guidance names exactly this class of platform for self-discovery. Useful
   pivots: `port:502` (Modbus), `port:102` (S7comm), `port:44818` (EtherNet/IP),
   `port:20000` (DNP3), `port:47808` (BACnet), `port:8001`/`9001`/`10001`
   (automatic tank gauges). Anything found here is a finding on its own — an
   internet-exposed HMI or PLC needs no exploitation to be critical.
2. **Document review before tooling.** P&IDs, network diagrams, asset registers,
   firewall rules, remote-access inventory. Ask specifically about **wireless and
   radio links** — an unsecured radio link inside an OT network often does not
   appear on diagrams yet forms part of the network edge.
3. **Passive collection** on the plant side: SPAN-port capture, historian and
   firewall logs, engineering-workstation inventory. This alone typically yields
   the asset list nobody had.
4. **Architecture review against the model** — Purdue levels and a real IDMZ,
   62443 zones/conduits, SL-T vs the deployed reality (`load_skill ot-ics`).
   Most high-severity findings are architectural, not exploitable-bug findings.
5. **Active testing only on a substitute** — vendor lab, digital twin, spare
   rack, or a planned-outage window with engineering present. For infrastructure
   operators, CISA's **CELR** platforms (with INL and PNNL) exist for exactly
   this: chemical processing, substations, compressor stations, water treatment,
   rail and dam environments.

## What to look for (highest yield first)

- **Internet-exposed OT** — HMIs, PLCs, gateways, tank gauges, BESS controllers.
- **IDMZ bypasses** — cellular modems, vendor VPNs terminating inside OT, dual-
  homed engineering workstations, cloud historian pushing through the boundary.
- **Remote access hygiene** — per-vendor VPN endpoints instead of one brokered
  DMZ gateway, shared privileged accounts, no session recording, break-glass
  accounts in routine use, standing rather than just-in-time access.
- **Flat OT networks** where a contractor's connection reaches every device.
- **Legacy services left on** — Telnet, FTP, RDP, VNC, unauthenticated web on
  controllers.
- **Default and view-only gaps** — restricting remotely accessible and default
  accounts to **view-only** removes impact without needing any vulnerability
  fixed; it is the cheapest high-value control in the corpus.

## Consequence-driven alternative

When the environment is too critical to test, **CCE** (Consequence-driven
Cyber-informed Engineering, INL) inverts the exercise: Consequence
Prioritization → System-of-Systems Analysis → Consequence-based Targeting →
Mitigations and Protections, aiming to engineer the physical effect out of
reach. Its working assumption is that a skilled, determined adversary _can and
will_ penetrate the network — so the control is engineering, not detection. Use
it when "prove it's exploitable" is the wrong question.

## Report

- `report_finding` with the **process consequence**, not just the CVE: "PLC
  controlling the reactor feed pump is reachable from the corporate VLAN" beats
  "outdated firmware". Severity in OT tracks safety and production impact.
- Note that CVSS is unreliable here (`load_skill ot-threat-landscape`) — justify
  severity by exposure and consequence, and say so in the finding.
- `render_report` for the deliverable; keep an explicit "not tested, and why"
  section — in OT that list is a finding about testability, not a gap in rigour.

## Pairs with

- `load_skill ot-ics`, `load_skill ot-threat-landscape`,
  `load_skill ot-compliance`.
- `load_skill network-enumeration` (the IT-side habits to _unlearn_ here),
  `load_skill red-teaming` (rules of engagement discipline).
