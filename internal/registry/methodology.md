# marq methodology

marq is a cyber assistant across disciplines — ~70 skill playbooks in 14
domains. Call `server_info` first for scope, run `load_skill '{}'` for the full
index, then load the playbook(s) for the work in front of you:

| Domain                      | When                                              | Example skills                                                                                                                                                 |
| --------------------------- | ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **offensive**               | pentest, recon, web, AD                           | the phases below + `vuln/*`, `technique/*`                                                                                                                     |
| **malware**                 | analyze a sample / maldoc                         | `malware-triage`, `malware-static`, `malware-maldoc`, `malware-yara`                                                                                           |
| **threat-intel**            | IOC pivot, attribution, enrichment                | `ioc-pivoting`, `actor-ttp-attribution`, `indicator-enrichment`                                                                                                |
| **secops**                  | SOC, hunting, IR, forensics, vuln mgmt, detection | `siem-soar`, `threat-hunting`, `incident-response`, `digital-forensics`, `vulnerability-management`, `detection-engineering`                                   |
| **architecture**            | design & engineering                              | `zero-trust`, `network-segmentation`, `cloud-security`, `appsec-sdlc`, `threat-modeling`, `iam`, `data-security`, `endpoint-security`, `crypto-key-management` |
| **grc**                     | risk, governance, audit, metrics, vendor          | `risk-assessment`, `security-governance`, `compliance-audit`, `security-metrics`, `gap-analysis`, `third-party-risk`, `control-mapping`                        |
| **standards**               | frameworks & regulations (EU + US)                | `choosing-a-framework`, `iso27001`, `nist-csf`, `pci-dss`, `soc2`, `control-frameworks`, `privacy-eu`, `nis2-dora`, `eu-cra`, `privacy-us`, `us-sectoral-regs` |
| **ciso**                    | leadership decisions                              | `vulnerability-prioritization`, `incident-response-leadership`, `security-program-roadmap`, `board-metrics-reporting`                                          |
| **offensive (adversarial)** | red/purple/bug-bounty/exploit                     | `red-teaming`, `purple-teaming`, `bug-bounty-disclosure`, `exploit-development`                                                                                |
| **resilience**              | BCP/DR/crisis/backup                              | `business-continuity`, `disaster-recovery`, `crisis-management`, `backup-recovery`                                                                             |
| **physical-human**          | physical, awareness, social-eng, insider          | `physical-security`, `security-awareness`, `social-engineering-defense`, `insider-threat`                                                                      |
| **enablers**                | GRC automation, privacy eng, DevSecOps, legal     | `grc-automation`, `privacy-engineering`, `devsecops`, `legal-contractual`                                                                                      |
| **reference**               | free no-login enrichment APIs                     | `security-apis`                                                                                                                                                |

Advisory and knowledge work is unrestricted. **Active testing** (scanning,
exploitation, credential attacks) is authorized-only — confirm targets are in
the scope `server_info` reports before you touch anything.

The phases below are the **offensive engagement** playbook. Work in phases;
record results with `report_finding`, then `render_report` at the end.

## 1. Scope & recon (passive first)

- `server_info` — confirm operator, engagement and scope.
- Company/domain footprint: `theharvester`, `subfinder`, `dnsx`, `wayback_urls`,
  `gau_urls`, `whois_lookup`, `dns_lookup`, `asnmap` (ASN mapping), `censys_search`.
  People: `sherlock`, `holehe_email`.
- Broad automated sweep: `spiderfoot` (runs in the background — poll with
  `job_status` / `list_jobs`, do not block waiting on it).
- Subdomain bruteforce: `dnsx` (-w wordlist). Takeover check: `subjack`.
- SSH config audit: `ssh_audit`. SNMP enum: `snmp_walk` / `snmp_check`. SMTP: `smtp_user_enum` / `smtp_test`.

## 2. Enumerate (active, scope-sensitive)

- Hosts/ports: `naabu` or `masscan` to find open ports, then `nmap` `-sV` on the
  hits for service/version detail. `fping_sweep` for host discovery across CIDRs.
- Live web: `httpx_probe` to find live HTTP, `whatweb` / `wafw00f` to fingerprint,
  `katana` to crawl endpoints. `cdncheck` to identify CDN/cloud IPs.
- Windows/SMB: `enum4linux` / `smb_enum` for shares & null sessions, `nbtscan`
  for NetBIOS discovery, `ldap_search` for directory queries.
- Params: `arjun` / `paramspider` for hidden parameters, `sstimap` for SSTI.

## 3. Test for vulnerabilities

- Web: `nuclei` (templates), `nikto`, content discovery (`ffuf` / `feroxbuster` /
  `gobuster_dir`), params (`arjun`), injection (`sqlmap`), XSS (`dalfox`), TLS
  (`testssl`), CMS (`wpscan` / `cmseek`).
- Known exploits: `searchsploit` against discovered product/versions.
- Secrets: `gitleaks` / `trufflehog` on retrieved code.

## 4. Exploit / validate (authorized, highest impact)

- AD/internal: `netexec` for mass auth/spray across SMB/WinRM/SSH, `impacket`
  lateral movement (psexec/wmiexec) + cred extraction (secretsdump/kerberoast/asreproast),
  `certipy_find` for AD CS vulnerabilities, `bloodhound_collect` for attack-path mapping,
  `evil_winrm` for WinRM access, `responder` for credential capture (background).
- Payloads: `donut` for shellcode generation. Online creds: `hydra`. Offline: `john` / `hashcat`.
  Validate a finding with a concrete proof before reporting it.

## 5. Report

- Record each validated issue with `report_finding` (title, severity, target,
  evidence, recommendation). Run `render_report` to write the Markdown + CSV
  deliverable into the working area.

## Operating notes

- Stage inputs and read tool output via `write_file` / `read_file` / `list_dir`
  (sandboxed to /work and /tmp).
- Prefer narrow scans; large output is truncated. Write big results to a file and
  read it back in parts.
- Every invocation is audit-logged (a call that can't be logged is refused). Scope
  is recorded, not enforced — you are accountable for staying in scope.
