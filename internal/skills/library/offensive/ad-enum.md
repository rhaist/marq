---
name: ad-enum
description: Active Directory & internal-network enumeration and attack path — from unauthenticated discovery through credential attacks to lateral movement, using marq's AD tool suite.
---

# Active Directory enumeration & attack paths

Authorized internal engagements only — confirm the scope `server_info` reports
(domain, subnets, hosts) before scanning. Work in order; each phase feeds the next.

`secretsdump`/BloodHound loot is real credential material and often personal data
— keep it inside the client's authorized boundary. A common cross-border pitfall
(EU/US tester on an APAC estate): pulling domain hashes back to a home-region host
can breach local data-residency/computer-misuse law even with owner authorization.
Store and process loot in-region unless the ROE says otherwise.

## 1. Discover hosts & services

- `nmap` / `naabu` for live hosts and open ports; `nbtscan` to sweep NetBIOS
  names across a subnet. DCs expose 88 (Kerberos), 389/636 (LDAP), 445 (SMB).

## 2. Unauthenticated enumeration (no creds yet)

- `smb_enum` and `enum4linux` against 445 for shares, users, password policy,
  and null/guest sessions.
- `netexec` (protocol `smb`) to spray null/guest and map signing — flags hosts
  where SMB signing is off (relay targets). Defaults are tightening: Win11 24H2
  Pro/Enterprise require inbound SMB signing (Server 2025 only outbound), and new
  Server 2025 domains enforce LDAP signing (channel binding stays "When supported").
- `ldap_search` for anonymous binds; `certipy_find` to enumerate AD CS templates
  (look for ESC1-ESC17 misconfigurations — Certipy v5 covers ESC1-16, incl.
  ESC16's global szOID_NTDS_CA_SECURITY_EXT bypass; 5.1+ adds ESC17: a
  server-auth template with enrollee-supplied SAN, abusable to spoof WSUS).

## 3. Get a first credential

- `responder` to poison LLMNR/NBT-NS and capture NetNTLMv2 hashes; if SMB signing
  is off, `impacket_ntlmrelayx` to relay instead of crack.
- `impacket_asreproast` for accounts without Kerberos pre-auth (crack the AS-REP
  offline with `hashcat -m 18200`/`john`). Pass `-request` (with creds) or
  `-no-pass -usersfile <file>` (without), plus `-dc-ip <DC>`, via `options`.
- Captured/relayed hashes → crack with `hashcat -m 5600` (NetNTLMv2), or pass them.

## 4. Authenticated enumeration (with a credential)

- `netexec` with creds to validate access across hosts and find local-admin reach;
  on Server 2025 domains, protocol `ldap` with `-M badsuccessor` checks the dMSA
  (BadSuccessor) path.
- `bloodhound_collect` to map attack paths (who can reach Domain Admin, ACL abuse,
  delegation). This is the map — load it before deciding the next hop. It runs the
  legacy `bloodhound-python`: ingest into BloodHound 4.2/4.3, not CE.
- `impacket_kerberoast` for service-account SPNs → crack the TGS offline. Pass
  `options="-request -dc-ip <DC>"` with target `domain/user:pass` (the roasting
  scripts don't parse `@host`; without `-request` they only list SPNs). RC4 tickets
  are `hashcat -m 13100`; DCs updated for the 2026 RC4 deprecation (CVE-2026-20833)
  issue AES tickets (`-m 19700`, far slower) unless the account's
  `msDS-SupportedEncryptionTypes` still allows RC4.

## 5. Loot & move laterally (in scope, authorized)

- `impacket_secretsdump` on hosts you have admin on → local SAM + domain creds
  (DCSync with the right rights).
- Execute via `impacket_psexec` / `impacket_wmiexec`, or `evil_winrm` for an
  interactive shell on 5985/5986.

## Report

- `report_finding` per issue (weak policy, SMB signing off, AS-REP/Kerberoastable
  accounts, ADCS ESC, reused local-admin creds). `target` = host/identity,
  `evidence` = the tool output proving it. Map findings to the BloodHound path.
- Common CWEs: CWE-287 (auth), CWE-522 (weak credential protection), CWE-269
  (privilege management).
