---
name: idor-access-control
description: Find broken object-level/function access control (IDOR, BOLA).
---

# Broken access control (IDOR / BOLA / privilege escalation)

The #1 web risk and mostly invisible to scanners — it needs two identities and
manual comparison.

## Setup

- Get two accounts (or two roles): low-priv A and another user/admin B. Capture
  a working request for one of A's objects (note the id: numeric, UUID, slug).

## Test object-level (IDOR/BOLA)

- Replay A's request but swap the object id to B's, keeping A's session. Use
  `run_shell` + curl with A's cookie/token. If A reads or mutates B's object →
  IDOR. Try id±1, other users' UUIDs, and predictable refs.
- Check every verb: GET leak, but also POST/PUT/PATCH/DELETE for write IDOR.

## Test function-level

- Call admin-only endpoints (from `katana`/JS) with a low-priv token. Try
  forced browsing to admin paths (`web-content-discovery` skill). Method
  tampering (GET vs POST), and removing/forging role params.

## Report

- `report_finding`: severity high/critical by data sensitivity and write vs read,
  `target` = endpoint + the id you swapped, `evidence` = paired requests showing
  A's session returning B's data (redact real PII). Recommend server-side
  authorization checks per object, deny-by-default, unpredictable ids are not a fix.
- CWE-639 / CWE-284; OWASP A01:2025 Broken Access Control — still #1, and in the
  2025 Top 10 it also absorbs SSRF and path traversal as sub-patterns.
