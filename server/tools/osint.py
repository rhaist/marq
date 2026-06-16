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

from ..runner import run

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
    def spiderfoot(target: str, use_case: str = "footprint", minutes: int = 20) -> str:
        """Broad automated OSINT footprint of a target (spiderfoot, 200+
        modules) — domains, IPs, netblocks, ASN, emails, names, breach data.
        `target` may be a domain, IP, email, name or username. `use_case` is
        one of all/footprint/investigate/passive. Runs headless (no web UI) and
        emits JSON. It is slow: `minutes` raises this call's timeout above the
        default (clamped server-side) so it can finish; on timeout you still get
        the partial JSON gathered so far. Many modules use optional API keys."""
        argv = ["spiderfoot", "-s", target, "-u", use_case, "-o", "json", "-q"]
        return run("spiderfoot", argv, target=target, timeout=minutes * 60).render()

    # --- asset / netblock discovery ---------------------------------------
    @mcp.tool()
    def amass_intel(org: str = "", domain: str = "", asn: str = "", cidr: str = "") -> str:
        """Discover an organisation's attack surface with `amass intel`: map an
        org name, root domain, ASN or CIDR to related domains, ASNs and IP
        ranges. Supply exactly one of `org`, `domain`, `asn`, `cidr`. With a
        domain it does reverse-WHOIS to find sibling domains."""
        argv = ["amass", "intel"]
        if org:
            argv += ["-org", org]
        elif asn:
            argv += ["-asn", asn]
        elif cidr:
            argv += ["-cidr", cidr]
        elif domain:
            argv += ["-whois", "-d", domain]
        else:
            return "error: provide one of org, domain, asn or cidr"
        return run("amass-intel", argv, target=org or domain or asn or cidr).render()

    @mcp.tool()
    def amass_enum(domain: str, active: bool = False, minutes: int = 15) -> str:
        """In-depth subdomain/asset enumeration for a DOMAIN (OWASP amass).
        Passive by default (OSINT sources only); set `active=True` to also do
        DNS resolution and light probing of discovered names. Active runs are
        slow — `minutes` raises this call's timeout (clamped server-side)."""
        argv = ["amass", "enum", "-d", domain, "-silent"]
        if not active:
            argv.append("-passive")
        return run("amass-enum", argv, target=domain, timeout=minutes * 60).render()

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
        return run("shodan-host", ["/bin/bash", "-lc", cmd], target=ip).render()

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
        return run("shodan-search", ["/bin/bash", "-lc", cmd], target=query).render()

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
        return run("waybackurls", ["/bin/bash", "-lc", cmd], target=domain).render()

    @mcp.tool()
    def gau_urls(domain: str) -> str:
        """Fetch known URLs for a DOMAIN from Wayback, Common Crawl, OTX and
        URLScan (gau) — broader historical coverage than waybackurls alone.
        Passive: queries archives, not the target."""
        cmd = f"echo {shlex.quote(domain)} | gau --subs --providers wayback,commoncrawl,otx,urlscan"
        return run("gau", ["/bin/bash", "-lc", cmd], target=domain).render()
