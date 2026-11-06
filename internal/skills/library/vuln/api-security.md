---
name: api-security
description: Test REST and GraphQL APIs against the OWASP API Security Top 10 (2023) — surface and spec discovery, object/property/function-level authorization with two accounts, mass assignment, rate-limit and resource-consumption checks, GraphQL introspection — and report with proof.
---

# API security (REST & GraphQL)

Active testing — in-scope APIs only (`server_info`; record with `set_engagement`). Baseline:
**OWASP API Security Top 10 2023**, still the current API edition (the 2025 list is the web-app
Top 10). verify: https://owasp.org/API-Security/editions/2023/en/0x11-t10/ Requests use
`run_shell` + curl, or your client's HTTP tool if raw shell isn't enabled.

## 1. Surface & specs (API9 Improper Inventory)

- `httpx_probe` the hosts, `katana` (`js_crawl` on) for API paths/versions in JS bundles,
  `gau_urls` (passive) for historical `/api/` routes.
- Specs: `nuclei` `templates="http/exposures/apis/"`, `ffuf` `url="https://host/FUZZ"` over
  `openapi.json`, `swagger.json`, `v2/api-docs`, `v3/api-docs`, `swagger-ui/`, `graphql`
  (`load_skill web-content-discovery`). A spec is your test list.
- Shadow versions (`/v1/` beside `/v3/`, beta/staging hosts) often miss newer auth checks.
  `arjun` `method="JSON"` per endpoint finds undocumented body fields.

## 2. Authentication (API2)

- Tokens/JWT → `load_skill auth-jwt`; OAuth/OIDC → `load_skill oauth-oidc`. Look for verbs or
  backend hosts that skip the gateway's auth, keys in JS/URLs, and unlimited login/OTP/reset.

## 3. Authorization with two accounts (API1, API3, API5)

- Users A and B (plus admin if possible), tokens captured; method in `load_skill idor-access-control`.
- **BOLA (API1):** A's token + B's id on every verb and id location (path, query, JSON body,
  nested objects, bulk `?ids=1,2`), cross-tenant too.
- **Property level (API3):** exposure = fields the UI hides (others' email, `role`, hashes).
  Mass assignment = add fields from GET/spec to PUT/PATCH on **A's own** object (`"role":"admin"`,
  `"is_admin":true`, `"account_id"`), then re-GET to prove it persisted.
- **BFLA (API5):** low-priv token on admin routes, GET→PUT/DELETE, `X-HTTP-Method-Override`.
  API6 business flows (sign-up, referral, checkout): show bot-abuse with a handful of requests.

## 4. Resource consumption (API4) — measure, never flood

- Prove the missing limit, not the outage: huge `limit=`/`page_size=`, large bodies, a ~20–30
  request burst (`ffuf`, numeric list via `write_file`, `options="-rate 5 -mc all"`) checking for
  429 / `Retry-After` / `RateLimit-*`. Load/DoS testing needs its own written approval.

## 5. GraphQL

- Find it: `nuclei` `templates="http/technologies/graphql-detect.yaml"`; confirm with POST
  `{"query":"{__schema{types{name}}}"}`. Introspection off? "Did you mean…" suggestions still leak
  the schema. verify: https://cheatsheetseries.owasp.org/cheatsheets/GraphQL_Cheat_Sheet.html
- Same authz tests per query/mutation: ids in arguments, mutations as low-priv, nested paths into
  others' objects (`me{orders{owner{email}}}`). Limits: depth, aliases (one field ×N in one request
  bypasses per-request rate limits; show ~10), array batching. Mutations over GET → CSRF.

## 6. Rest of the list

- API7 SSRF (webhooks, URL import) → `load_skill ssrf`. API8: CORS reflecting `Origin` with
  credentials, stack traces, `testssl`. API10 (unsafe third-party consumption) is a design review item.

## Report

- `report_finding` per root cause, titled with the category (`API1:2023 BOLA — GET /v2/orders/{id}`),
  `evidence` = paired requests (A's token, B's id → B's data; PII redacted). `cwe`: BOLA CWE-639,
  BFLA CWE-285, mass assignment CWE-915, exposure CWE-213, limits CWE-770. Recommend per-object
  authz in the data layer, allowlisted writable fields, response schemas, rate/size/depth limits,
  and an inventory that retires old versions.
