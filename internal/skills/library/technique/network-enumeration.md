---
name: network-enumeration
description: Port/service enumeration and TLS review on hosts in scope.
---

# Network & service enumeration

## Discover hosts and ports
- `naabu` — fast modern port scan (`-host`), explicit `ports` or `top_ports`.
  SYN-scans with NET_RAW caps, else connect scan.
- `masscan` — for very large ranges; keep `rate` conservative on shared networks.
- Then `nmap` `-sV` on just the open ports for service + version. Add scripts via
  `options` (`-sC`, or targeted `--script`) for deeper checks — but stay in scope.

## Per-service follow-up
- HTTP(S): hand off to `web-content-discovery` / vuln-class skills.
- TLS: `testssl` on `host:443` for protocols, ciphers, cert chain, and known TLS
  flaws (Heartbleed, ROBOT, weak/expired certs).
- Known-vuln sweep: `nuclei` against discovered services/URLs; `searchsploit`
  the product+version strings nmap reported for public exploits.

## DNS / zone
- `dnsrecon` (`axfr` for zone transfer, `std` for records) and `dnsx` for bulk
  resolution and record types.

## Report
- `report_finding` per concrete issue (exposed service, weak TLS, default creds,
  known-CVE service). `target` = host:port, `evidence` = the nmap/testssl/nuclei
  line. Recommend closing/segmenting the port, patching, or hardening config.
- Map findings to the relevant CWE/CVE.
