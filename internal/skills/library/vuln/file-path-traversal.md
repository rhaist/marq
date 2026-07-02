---
name: file-path-traversal
description: Path traversal, local file include, and unsafe upload.
---

# Path traversal / LFI / file upload

## Path traversal & LFI

- Candidates: any param naming a file/path/template/page/lang — `download?file=`,
  `?page=`, `include`, `template`, image/report generators. `arjun` to find them.
- Test with `run_shell` + curl: `../../../../etc/passwd`, encoded variants
  (`%2e%2e%2f`, double-encoding, `....//`), null byte / extension tricks on old
  stacks, and absolute paths. A read of `/etc/passwd` (or `win.ini`) confirms.
- PHP LFI: wrappers (`php://filter/convert.base64-encode/resource=...`) to read
  source; log poisoning / session files for RCE if includable.
- `nuclei` has LFI templates — run it to catch the easy ones fast.

## File upload

- If the app accepts uploads, test: dangerous extensions (.php/.jsp/.svg/.html),
  content-type vs extension mismatch, double extensions, path traversal in the
  filename, and whether the upload lands in a web-served, executable directory.
- After upload, fetch the file back (curl) to see if it executes or runs JS (SVG/HTML → stored XSS).

## Report

- `report_finding`: severity high/critical (file read of secrets, or RCE via
  upload/LFI), `target` = endpoint+param, `evidence` = the retrieved file
  contents or the executed payload. Recommend canonicalize + allowlist paths,
  store uploads outside webroot, validate content not just extension.
- CWE-22 / CWE-98 / CWE-434; path traversal/LFI now map under OWASP A01:2025
  Broken Access Control (a sub-pattern in the 2025 Top 10).
