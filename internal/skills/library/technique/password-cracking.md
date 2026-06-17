---
name: password-cracking
description: Offline hash identification and cracking workflow.
---

# Offline password / hash cracking

For hashes you are authorized to assess (dumped DB, captured handshake, JWT
secret, /etc/shadow on an owned box).

## 1. Identify
- `hash_identify` to guess the hash type from a sample. This drives the format
  for john and the `-m` mode for hashcat.

## 2. Stage the input
- `write_file` the hashes into `/work/hashes.txt` (one per line, in the format
  the cracker expects — e.g. `user:hash` or raw hash).

## 3. Crack
- `john` — flexible, good defaults. Pass `options="--format=<fmt> --wordlist=/usr/share/wordlists/rockyou.txt"`.
  Without a wordlist it runs its default modes.
- `hashcat` — faster, mode-driven. Provide `mode=<-m>`, `wordlist=...`. A CPU
  OpenCL device is bundled (`--force` is auto-added) — fine for weak/fast hashes;
  real cracking wants `--gpus all` at `docker run`.
- Common modes: 0=MD5, 100=SHA1, 1400=SHA256, 1800=sha512crypt, 3200=bcrypt,
  16500=JWT(HS256), 22000=WPA. Rules (`-r best64.rule`) and masks for tougher sets.

## 4. Report
- `report_finding`: severity by what the cracked creds unlock and password
  policy weakness. `evidence` = counts cracked / weak examples (do NOT paste
  real plaintext credentials in full). Recommend strong hashing (bcrypt/argon2),
  salting, and password policy / MFA.
- CWE-916 / CWE-521.
