---
name: incident-response-leadership
description: Run a security incident as the decision-maker — phases, the notify/contain/escalate decisions, comms, and evidence preservation.
---

# Incident response leadership

You are the incident commander, not the analyst. Your job is decisions, tempo,
and who-gets-told — not running the forensic tools yourself. Follow the NIST
SP 800-61 lifecycle: Prepare → Detect & Analyze → Contain, Eradicate & Recover
→ Post-incident. Map adversary actions to MITRE ATT&CK so handoffs are unambiguous.

## First 60 minutes (commander checklist)

1. Declare or don't — set a severity now (see table), it drives everyone's urgency.
2. Open a single source of truth (war-room channel + running timeline doc). One
   scribe. Every action and time gets logged — this is the legal record later.
3. Assign roles: commander (you, decisions), tech lead (containment), comms lead,
   scribe. Don't let one person hold two.
4. Establish out-of-band comms (Signal/phone) — assume the attacker reads
   corporate email/Slack until proven otherwise.
5. Decide the prime directive for this incident: **preserve evidence** vs **stop
   the bleeding**. State it explicitly — it resolves later conflicts (see below).

## Severity → who wakes up

| Sev | Trigger                                                                      | Activates                          |
| --- | ---------------------------------------------------------------------------- | ---------------------------------- |
| 1   | Active attacker in crown-jewels / ransomware spreading / confirmed PII exfil | Exec, Legal, on-call IR, CEO brief |
| 2   | Confirmed compromise, contained scope, no mass data loss yet                 | IR team, CISO, Legal on standby    |
| 3   | Single host, malware caught, no lateral movement evident                     | IR team, CISO informed             |
| 4   | Suspicious, unconfirmed                                                      | Analyst triages, log it            |

## The hard decisions (decision rules)

**Pull the plug / isolate now?**

- Isolate the host (network-quarantine, _don't_ power off — RAM/volatile evidence
  dies with power) when: active lateral movement, ransomware encrypting, or active
  exfil. Containing > evidence when data is actively leaving.
- Watch-and-learn instead when: scope unknown, attacker dormant, and isolating one
  node tips them off to burn the rest. Time-box it and get exec air-cover for the risk.

**Notify regulators / customers?** — run the clock, it's the trap people miss.

- Personal data breach likely → GDPR Art. 33: **72 hours** to the supervisory
  authority from _awareness_, not from resolution. The clock has started already.
- US: state breach laws + sector rules; **SEC** public companies — material cyber
  incident → 8-K within **4 business days** of materiality determination.
- HIPAA, PCI-DSS, contractual notify clauses — Legal owns the matrix; your job is
  to surface "we may have a notifiable event" _early_, not when you're certain.
- Decision: if _plausible_ that regulated data was accessed → tell Legal now and
  let them run the clock. Under-notifying is the expensive mistake.

**Call law enforcement (FBI/IC3, NCA, local CERT)?**

- Yes when: ransomware/extortion, nation-state indicators, ongoing fraud/wire
  theft (speed can claw back funds), or you'll file a cyber-insurance claim that
  requires it. Loop Legal _before_ the call.
- Understand the trade: LE may ask you to preserve and not disclose; they rarely
  accelerate _your_ recovery. Engage for attribution, takedown, and the record.

**Pay a ransom?** Not your call alone — Legal + exec + insurer. Check OFAC
sanctions exposure (paying a sanctioned actor is itself illegal). Decryptors
often fail; recovery from backups is the real plan.

## Evidence preservation (do this before eradicating)

- Order of volatility: capture RAM and live network state _before_ disk, before
  reboot, before reimage. Reimaging destroys the case.
- Hash everything on collection (sha256), record chain of custody (who/when/where)
  in the timeline. Untracked evidence is inadmissible.
- For triage of a captured sample or suspicious binary, `load_skill` the malware
  domain — marq wraps forensic/triage tooling; analyse the _copy_, never the only
  copy, and on an isolated host. Pivot IOCs with `load_skill ioc-pivoting`.
- Store artifacts and the timeline under `/work`; `report_finding` each confirmed
  TTP with its ATT&CK id so the post-incident report writes itself via `render_report`.

## Comms discipline

- Internal: facts only, on a cadence (e.g. hourly exec update). "We don't know yet"
  beats a guess that ages badly into a lie.
- External: Legal/PR approve every word; an early over-promise ("no data taken")
  that reverses is the reputational wound, not the breach.
- Hold a blameless post-incident review within 2 weeks: timeline, what detection
  missed, dwell time, and 3 concrete control changes. Feed them into the roadmap.
