---
name: cloud-k8s-assessment
description: Authorized external assessment of cloud and Kubernetes exposure with marq — cloud asset footprint, exposed control-plane ports (API server, kubelet, etcd, Docker), nuclei misconfiguration templates, trivy image/IaC review, leaked cloud keys in repos, and how to rate and report each finding.
---

# Cloud & Kubernetes exposure assessment

Find what the client exposes, prove it with minimal contact, and report it. This is an exposure and configuration review, not an exploitation or post-exploitation playbook.

For what "good" looks like, see `load_skill container-kubernetes-security` and `load_skill cloud-security`.

## Authorization and scope (settle before any packet)

- **Record scope first.** Call `server_info`, then `set_engagement` with the written scope. Cloud scope must name **accounts, subscriptions or projects, and clusters**, not just IPs.
- **Cloud IPs get reassigned.** An IP that was in scope last month may belong to someone else now. Re-resolve and confirm ownership right before scanning (`dnsx`, `cdncheck`, `asnmap`).
- **The provider is a third party.** Its testing policy applies on top of the client's authorization.
  - No DoS or stress testing.
  - The provider's infrastructure and managed control planes are not yours to test.
  - verify: https://aws.amazon.com/security/penetration-testing/ (plus Azure's rules of engagement and Google Cloud's AUP)
- **Never without explicit written authorization for that specific action:**
  - Production cluster writes (`apply`, `exec`, `delete`).
  - Changes to IAM, policies, SCPs or Conditional Access.
  - Brute force or load against an API server.
  - **Using a discovered credential, even "just to check".** `trufflehog` verification counts.
  - Reading exposed bucket data beyond the minimum proof (a listing or one object name).
- **Authenticated config review is out of band.** Prowler, ScoutSuite and kube-bench need credentials marq doesn't hold. Have the client run them or hand over read-only exports, and review those here.

## 1. Cloud footprint (passive)

- **Spot cloud-hosted names.** Run `subfinder`, then `dnsx` with `records: "cname,a"`. Look for CNAMEs to `*.s3.amazonaws.com`, `*.blob.core.windows.net`, `storage.googleapis.com`, `*.azurewebsites.net`, `*.cloudfront.net` and `*.elb.amazonaws.com`.
- **Dangling CNAMEs.** One whose target no longer exists is a takeover candidate. Report it with the DNS evidence; don't claim the resource.
- **Classify IPs:** `cdncheck` (cloud/CDN/WAF) and `asnmap`.
- **Third-party scan data:** `shodan_search` (e.g. `ssl.cert.subject.cn:acme.com port:6443`), `shodan_host` (keyless) and `censys_view`. See `load_skill recon-footprint` and `load_skill security-apis`.

## 2. Exposed control-plane and cluster ports

Run `naabu` with `ports: "443,2375,2376,2379,2380,6443,8001,8080,9090,10250,10255,10256,10257,10259"`, then `nmap` `-sV` on whatever is open.

| Port                    | Expect                           | Finding when…                                                                  |
| :---------------------- | :------------------------------- | :----------------------------------------------------------------------------- |
| 6443/443 API server     | Often public on managed clusters | No allowlist (medium); anonymous resource access (critical)                    |
| 10250 kubelet           | Never internet-facing            | Reachable (high); `/pods` answers without auth (critical)                      |
| 10255 read-only kubelet | Shouldn't exist                  | Any response: pod specs and env leak (high)                                    |
| 2379-2380 etcd          | Control plane only               | Reachable (high); answers without a client cert (critical: holds every Secret) |
| 2375 Docker API         | Never                            | Any unauthenticated response (critical: host control)                          |
| 10257/10259             | Localhost only                   | Reachable at all                                                               |
| 8001/9090/8080          | Proxy, Prometheus, dashboards    | Unauthenticated UI or API                                                      |

- **Check anonymous API access with `nuclei`**, `templates: "http/misconfiguration/kubernetes/"`. It covers unauthenticated etcd, pod-listing APIs and metrics endpoints.
  - `/version` and `/healthz` are anonymously readable by default; treat them as info.
  - A 401/403 on resource paths is the healthy answer.
- **Dashboards and add-ons:** `httpx_probe` for titles and tech, then `nuclei` with `templates: "http/exposed-panels/"` and `"http/misconfiguration/prometheus/"` on in-scope URLs.
- **ingress-nginx.** If you see it (banner, default backend), report it as an unsupported component (retired 24 Mar 2026). Report it separately as CVE-2025-1974 if its admission webhook is reachable.

## 3. Cloud service misconfiguration (HTTP-visible)

- **Run `nuclei` templates on in-scope web endpoints.** Use `templates: "http/misconfiguration/aws/"` and `"http/exposures/"`, which catch leaked `.env` files, credential files, `tfstate` and backups. Add `severity: "medium,high,critical"` to cut noise.
- **Nuclei's `cloud/` templates won't run here.** They're `code`-protocol config reviews that need `-code` plus a local `kubectl`/`aws`/`az`/`gcloud` with credentials, and the marq `nuclei` wrapper passes neither.
- **Metadata reachability.** SSRF to `169.254.169.254` is how a web bug becomes a cloud compromise (`load_skill ssrf`). Prove reach; don't harvest role credentials.

## 4. Images and IaC (artefact review, no target contact)

- **Images:** `trivy` with `mode: image`, the target pinned by digest. Private registries won't authenticate as wired, because no trivy registry credentials are forwarded into the container. Ask for a public mirror or the client's own scan output.
- **Manifests, Helm, Terraform, Dockerfiles:** stage them under `/work` (or let `trivy` `mode: repo` clone them), then run `trivy` with `mode: fs` on the directory.
  - **Use `fs`, not `config`.** The wrapper always adds `--scanners`, which `trivy config` doesn't accept (verify against the installed build).
  - Helm renders with default values, so ask for `helm template` or `kubectl get -o yaml` exports; production values differ.
- **Read the results:**
  - `KSV-*`: privileged pods, hostPath, hostNetwork, root, added capabilities, `latest` tags.
  - `AVD-AWS/AZU/GCP-*`: public buckets, `0.0.0.0/0` on 22/3389/6443, `*:*` IAM, a public cluster endpoint without CIDRs, IMDSv1, no encryption, a CI OIDC role trust missing a `sub` condition.
  - Re-rate by exposure; trivy's severity is the check's generic one.
- **RBAC/policy exports:** stage them with `write_file` and review against `load_skill container-kubernetes-security`. Look for wildcard roles, `list secrets`, cluster-admin bindings, namespaces without Pod Security labels, and a missing default-deny NetworkPolicy.

## 5. Leaked cloud keys

- **`gitleaks`** with `path` on a checked-out repo under `/work`. Its `dir` mode scans **files only, not git history**.
- **`trufflehog`** with `source: "git https://…"` walks full history. Use `source: "github --org=acme"` for a whole org (needs `GITHUB_TOKEN`).
- **Verification is credential use.** The wrapper's default `--results=verified` calls the provider with each key, and it shows up in the client's CloudTrail. Unless that's authorized, pass `options: "--no-verification"`.
- **What to look for:** AWS `AKIA`/`ASIA` keys, Azure client secrets, GCP SA JSON (`private_key`), kubeconfigs with `client-key-data` or tokens, registry and CI tokens, secrets in `tfstate`.
- **A key that looks live is a stop-and-notify event.** Hand it to the client now rather than holding it for the report, and point them to `load_skill cloud-incident-response`.

## Report

- **Write each finding with `report_finding`.**
  - `target`: host:port, image digest, file path or account/cluster.
  - `evidence`: the trivy/nuclei/naabu line, or the HTTP status plus a body excerpt. Redact secrets to a prefix plus the last 4 characters.
- **Severity guide:**
  - **Critical:** unauthenticated kubelet/etcd/Docker API, anonymous API resource access, a verified key in a public repo.
  - **High:** control-plane ports open to the internet, privileged pods on an exposed cluster.
  - **Medium/low:** API with no allowlist, EOL minor or retired ingress-nginx, missing Pod Security labels or NetworkPolicy. Adjust for exposure.
- **CWE:**
  - CWE-306: missing authentication for a critical function.
  - CWE-798: hard-coded credentials.
  - CWE-269 / CWE-732: privilege or permission flaws.
  - CWE-1104: unmaintained third-party component.
- **Roll trivy CVEs up per image:** counts plus KEV/EPSS standouts (`load_skill vulnerability-prioritization`), not one finding per CVE.
- **Finish with `render_report`.** Keep a "not tested, and why" section: authenticated config review, private registries, managed control plane.

## Pairs with

- `load_skill container-kubernetes-security`, `load_skill cloud-security`, `load_skill cloud-incident-response`, `load_skill devsecops`.
- `load_skill network-enumeration`, `load_skill recon-footprint`, `load_skill ssrf`.
