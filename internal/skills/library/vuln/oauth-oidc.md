---
name: oauth-oidc
description: Test OAuth 2.0 / OpenID Connect flows — redirect_uri validation, state/PKCE/nonce, response-mode and token leakage, ID-token validation (iss/aud/nonce/signature) — against the RFC 9700 Security BCP baseline, and report with proof.
---

# OAuth 2.0 / OpenID Connect

Active testing — in-scope clients and authorization servers (AS) only (`server_info`; record with
`set_engagement`), with your own test accounts. A third-party IdP (Google, Entra ID, Okta) is out
of scope unless the authorization names it: test the _client's_ handling. Requests via `run_shell`

- curl (or your client's HTTP tool); capture full logins with an off-box intercepting proxy.

## Baseline

- **RFC 9700** (BCP 240, Jan 2025): code + PKCE (`S256`), exact-string redirect_uri match
  (localhost port excepted for native apps), no implicit or password grant, no tokens in URL
  query, sender-constrained (DPoP RFC 9449 / mTLS RFC 8705) or rotated refresh tokens for public
  clients, mix-up defence (RFC 9207 `iss`). verify: https://www.rfc-editor.org/rfc/rfc9700.html
- **OAuth 2.1** is still an Internet-Draft (draft-ietf-oauth-v2-1-16, Sep 2026), not an RFC; cite
  RFC 9700 as normative. verify: https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/ SPAs:
  RFC 10017 (Aug 2026) prefers a backend-for-frontend. verify: https://www.rfc-editor.org/info/rfc10017
  Follow-on draft-ietf-oauth-security-topics-update adds audience injection (client-assertion `aud`
  must be the AS issuer). verify: https://datatracker.ietf.org/doc/draft-ietf-oauth-security-topics-update/

## 1. Map the flow

- `/.well-known/openid-configuration`, `/.well-known/oauth-authorization-server`. Red flags:
  `token`/`id_token` response types (implicit on), `plain` PKCE, `password` grant.
- Record `client_id`, `redirect_uri`, `response_type`, `response_mode`, `scope`, `state`,
  `nonce`, `code_challenge(_method)` from a real authorization request.

## 2. redirect_uri validation

- Change one thing at a time: extra path (`/callback/x`, `/callback/..%2f`), added query, other
  subdomain/port/scheme, `@`/`#`/`%2f` parser tricks. A code/token delivered off the registered
  URI = takeover primitive (high/critical). Prove delivery to a host you control (`interactsh`
  domain), or chain an open redirect on the allowed domain (prefix matching).

## 3. state, PKCE, nonce, code

- Drop/reuse `state` — not bound to the browser session → login CSRF.
- PKCE: omit `code_challenge`, try `plain`, redeem with no/wrong `code_verifier`; any success =
  not enforced. Redeem a code twice: the second must fail.
- `nonce`: the client must reject an ID token with a missing or different nonce.

## 4. Response mode & token leakage

- `response_type=token` accepted for a code client → implicit still on. Forced
  `response_mode=query` puts tokens in logs. Codes in `Referer` to third-party assets on the
  callback page; `postMessage` to `*`. Refresh tokens: reuse after rotation, cross-client use,
  scope upgrade at the token endpoint.

## 5. ID-token / access-token validation

- `jwt_tool` (no options) to decode. OIDC Core §3.1.3.7: signature via `jwks_uri`, `iss` exact,
  `aud` contains the client_id (`azp` if several), `exp`, `nonce`.
- Try an ID token minted for a _different_ client of the same IdP (aud), from another issuer,
  expired, or alg-tampered (`load_skill auth-jwt`) — accepted = account takeover. Accounts keyed on
  `email` rather than `iss`+`sub` → takeover. Access token for API A accepted by API B = no
  audience check (`load_skill api-security`).

## Report

- `report_finding`: `target` = client_id + endpoint, `evidence` = the request/response showing the
  code/token going astray or the forged token accepted (truncate live tokens). `cwe`: CWE-601
  (redirect_uri), CWE-352 (state), CWE-347 (unverified ID token), CWE-287. Recommend the missing
  RFC 9700 item by section. OWASP A07:2025 Authentication Failures (A01:2025 if it grants another
  user's access).
