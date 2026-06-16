"""Company / domain footprint & information-gathering (OSINT) tools.

Passive reconnaissance of an *organisation's* externally-observable footprint:
employees and email addresses, subdomains and hosts, ASNs / IP ranges, exposed
assets, document metadata, leaked secrets and historical URLs. These query
public sources and third-party datasets rather than actively touching the
target, which makes them the right first phase of an engagement.

Tools that depend on third-party APIs note the relevant environment variables;
without keys they degrade to keyless sources or return nothing.
"""
from __future__ import annotations

import shlex

from ..runner import run, run_background

# Keyless theHarvester sources — work with zero configuration. The model can
# override with `sources` (e.g. add shodan/hunter once keys are configured).
_HARVESTER_KEYLESS = "crtsh,duckduckgo,bing,otx,rapiddns,hackertarget,certspotter,anubis,threatminer"


def register(mcp) -> None:
    # --- aggregated frameworks --------------------------------------------
    @mcp.tool()
    def theharvester(domain: str, sources: str = "", limit: int = 500) -> str:
        """Footprint a DOMAIN with theHarvester: emails, employee names, hosts
        and subdomains gathered from public search/cert sources. Defaults to
        keyless sources; pass `sources` (comma list) to use configured ones
        (shodan, hunter, securitytrails, …). The single best opening recon move."""
        argv = ["theHarvester", "-d", domain, "-b", sources or _HARVESTER_KEYLESS, "-l", str(limit)]
        return run("theharvester", argv, target=domain).render()

    @mcp.tool()
    def spiderfoot(target: str, use_case: str = "passive") -> str:
        """Broad automated OSINT footprint of a target (spiderfoot, 200+
        modules) — domains, IPs, netblocks, ASN, emails, names, breach data.
        `target` may be a domain, IP, email, name or username. `use_case` is one
        of all/footprint/investigate/passive (`passive` is safe + fastest;
        `footprint` also does active DNS work — scope-sensitive).

        SpiderFoot is too slow for a synchronous call, so this launches it in the
        BACKGROUND and returns immediately with a job directory. Read results as
        they stream in with `read_file('<job>/stdout.log')`; the scan is finished
        once `<job>/status` exists. Many modules use optional API keys."""
        argv = ["spiderfoot", "-s", target, "-u", use_case, "-o", "json"]
        job_dir, err = run_background("spiderfoot", argv, target=target)
        if err:
            return f"error: {err}"
        return (
            f"SpiderFoot ({use_case}) launched in the background for {target}.\n"
            f"  results (JSON) : {job_dir}/stdout.log\n"
            f"  diagnostics    : {job_dir}/stderr.log\n"
            f"  done-signal    : {job_dir}/status  (appears with 'exit=<code>' when finished)\n"
            f"Poll with read_file('{job_dir}/stdout.log') or list_dir('{job_dir}'); "
            "give it a few minutes. Note: subfinder/dnsx/theharvester are faster "
            "for quick subdomain/email recon."
        )

    # --- document metadata ------------------------------------------------
    @mcp.tool()
    def exif_metadata(path: str) -> str:
        """Extract embedded metadata (author, software, GPS, internal paths,
        emails) from a file or directory with exiftool. `path` must be inside
        the container — stage documents in the mounted `/work` dir (or pass a
        path produced by another tool), then read findings here. Surfaces
        internal usernames and software versions leaked in document properties."""
        argv = ["exiftool", "-r", path]
        return run("exiftool", argv, target=path).render()

    # --- internet-exposed assets (API-key) --------------------------------
    @mcp.tool()
    def shodan_host(ip: str) -> str:
        """Look up an IP on Shodan: open ports, services, banners, vulns.
        Requires SHODAN_API_KEY in the container environment."""
        cmd = (
            'if [ -n "$SHODAN_API_KEY" ]; then shodan init "$SHODAN_API_KEY" >/dev/null 2>&1; fi; '
            f"shodan host {shlex.quote(ip)}"
        )
        return run("shodan-host", ["/bin/bash", "-c", cmd], target=ip).render()

    @mcp.tool()
    def shodan_search(query: str, limit: int = 100) -> str:
        """Search Shodan for exposed assets, e.g. 'org:"Acme Corp"' or
        'ssl.cert.subject.cn:example.com'. Returns ip/port/org/product/hostnames.
        Requires SHODAN_API_KEY in the container environment."""
        fields = "ip_str,port,org,hostnames,product"
        cmd = (
            'if [ -n "$SHODAN_API_KEY" ]; then shodan init "$SHODAN_API_KEY" >/dev/null 2>&1; fi; '
            f"shodan search --fields {fields} --limit {int(limit)} {shlex.quote(query)}"
        )
        return run("shodan-search", ["/bin/bash", "-c", cmd], target=query).render()

    # --- leaked secrets / code exposure -----------------------------------
    @mcp.tool()
    def gitleaks(path: str) -> str:
        """Scan a local git repo or directory for committed secrets (gitleaks).
        `path` is a checked-out repo/dir inside the container. Reports findings
        as JSON to stdout."""
        argv = ["gitleaks", "dir", path, "--report-format", "json", "--report-path", "/dev/stdout"]
        return run("gitleaks", argv, target=path).render()

    @mcp.tool()
    def trufflehog(source: str, options: str = "--results=verified") -> str:
        """Find (and verify) leaked secrets with trufflehog. `source` is a git
        URL or 'github --org=<org>'-style target passed through as raw args.
        Provide a GITHUB_TOKEN env for GitHub org/repo scans. Defaults to only
        verified secrets to cut noise."""
        argv = ["trufflehog", *shlex.split(source), *shlex.split(options), "--json"]
        return run("trufflehog", argv, target=source).render()

    # --- historical / archived footprint ----------------------------------
    @mcp.tool()
    def wayback_urls(domain: str, include_subs: bool = True) -> str:
        """Pull historical URLs for a DOMAIN from the Wayback Machine
        (waybackurls). Reveals old endpoints, parameters and forgotten assets
        without touching the live target. `include_subs` also fetches subdomains."""
        flag = "" if include_subs else "-no-subs"
        cmd = f"echo {shlex.quote(domain)} | waybackurls {flag}".strip()
        return run("waybackurls", ["/bin/bash", "-c", cmd], target=domain).render()

    @mcp.tool()
    def gau_urls(domain: str) -> str:
        """Fetch known URLs for a DOMAIN from Wayback, Common Crawl, OTX and
        URLScan (gau) — broader historical coverage than waybackurls alone.
        Passive: queries archives, not the target."""
        cmd = f"echo {shlex.quote(domain)} | gau --subs --providers wayback,commoncrawl,otx,urlscan"
        return run("gau", ["/bin/bash", "-c", cmd], target=domain).render()
