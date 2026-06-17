---
name: auth-jwt
description: Attack authentication and JWT handling.
---

# Authentication & JWT

## Credential attacks (online)
- `hydra` for online guessing against ssh/ftp/http-post-form/etc. Noisy and
  lockout-prone — confirm scope, throttle, prefer a tiny high-signal list.
- For login forms: identify the failure vs success marker first, then encode it
  in the http-post-form module string.

## Session & token review
- Capture a token (cookie/JWT) via `run_shell` + curl on login. Check: secure,
  httponly, samesite flags; token entropy; does logout invalidate server-side.

## JWT specifically
- Decode header/payload (base64url) with `run_shell`. Look for:
  - `alg: none` accepted → forge an unsigned token.
  - HS256 verified with a weak/guessable secret → crack the signature offline
    with `john` (format `HMAC-SHA256`) or `hashcat` (`mode=16500`) against a
    wordlist; if it cracks you can mint arbitrary claims.
  - RS256→HS256 confusion → sign with the public key as HMAC secret.
  - Unverified claims: tamper `sub`/`role`/`admin` and replay.

## Report
- `report_finding`: severity high/critical (auth bypass / privilege forge),
  `target` = the auth endpoint/token, `evidence` = the forged/cracked token and
  the request it authorized. Recommend strong server-side verification, pinned
  algorithm, rotated high-entropy secrets, short expiry.
- CWE-287 / CWE-345 / CWE-347.
