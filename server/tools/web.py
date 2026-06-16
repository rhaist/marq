"""Web application testing tools."""
from __future__ import annotations

import shlex

from ..runner import run


def register(mcp) -> None:
    @mcp.tool()
    def nuclei(target: str, templates: str = "", severity: str = "") -> str:
        """Run nuclei vulnerability templates against a URL/host. Optionally
        restrict to `templates` (a tag or path) and/or `severity`
        (e.g. "medium,high,critical")."""
        argv = ["nuclei", "-silent", "-u", target]
        if templates:
            argv += ["-t", templates]
        if severity:
            argv += ["-severity", severity]
        return run("nuclei", argv, target=target).render()

    @mcp.tool()
    def nikto(target: str) -> str:
        """Run a Nikto web server scan against a URL or host."""
        argv = ["nikto", "-h", target]
        return run("nikto", argv, target=target).render()

    @mcp.tool()
    def ffuf(url: str, wordlist: str = "/usr/share/wordlists/dirb/common.txt", options: str = "") -> str:
        """Content/directory fuzzing with ffuf. `url` must contain the FUZZ
        keyword (e.g. https://host/FUZZ). Extra raw flags via `options`."""
        argv = ["ffuf", "-w", wordlist, "-u", url]
        if options:
            argv += shlex.split(options)
        return run("ffuf", argv, target=url).render()

    @mcp.tool()
    def gobuster_dir(url: str, wordlist: str = "/usr/share/wordlists/dirb/common.txt") -> str:
        """Directory brute force with gobuster against a base URL."""
        argv = ["gobuster", "dir", "-q", "-u", url, "-w", wordlist]
        return run("gobuster", argv, target=url).render()

    @mcp.tool()
    def whatweb(target: str) -> str:
        """Fingerprint web technologies on a URL/host with WhatWeb."""
        return run("whatweb", ["whatweb", target], target=target).render()

    @mcp.tool()
    def wpscan(url: str, options: str = "--enumerate vp") -> str:
        """Scan a WordPress site with WPScan. Default enumerates vulnerable
        plugins. Provide an API token via WPSCAN_API_TOKEN for vuln data."""
        argv = ["wpscan", "--url", url, *shlex.split(options)]
        return run("wpscan", argv, target=url).render()

    @mcp.tool()
    def sqlmap(url: str, options: str = "--batch") -> str:
        """Test a URL for SQL injection with sqlmap. `--batch` runs
        non-interactively with defaults. Add raw flags via `options`."""
        argv = ["sqlmap", "-u", url, *shlex.split(options)]
        return run("sqlmap", argv, target=url).render()

    @mcp.tool()
    def katana(url: str, depth: int = 2, js_crawl: bool = True) -> str:
        """Crawl a site for endpoints with katana (projectdiscovery). Discovers
        links, forms and (with `js_crawl`) endpoints embedded in JavaScript.
        Actively requests the target. `depth` bounds crawl recursion."""
        argv = ["katana", "-u", url, "-silent", "-d", str(depth)]
        if js_crawl:
            argv.append("-jc")
        return run("katana", argv, target=url).render()

    @mcp.tool()
    def feroxbuster(url: str, wordlist: str = "/usr/share/wordlists/dirb/common.txt", options: str = "") -> str:
        """Fast recursive content/directory discovery with feroxbuster — a
        modern alternative to gobuster/ffuf with automatic recursion. Extra raw
        flags via `options` (e.g. '-x php,txt -d 2')."""
        argv = ["feroxbuster", "-u", url, "-w", wordlist, "--silent", "--no-state"]
        if options:
            argv += shlex.split(options)
        return run("feroxbuster", argv, target=url).render()

    @mcp.tool()
    def arjun(url: str, method: str = "GET") -> str:
        """Discover hidden HTTP parameters on an endpoint with arjun. `method`
        is GET/POST/JSON/XML. Useful before fuzzing for injection on params the
        app accepts but doesn't document."""
        argv = ["arjun", "-u", url, "-m", method]
        return run("arjun", argv, target=url).render()

    @mcp.tool()
    def dalfox(url: str, options: str = "") -> str:
        """Scan a URL for XSS with dalfox. Tests reflected/stored/DOM vectors and
        verifies findings. Add raw flags via `options` (e.g. a custom header or
        '--deep-domxss')."""
        argv = ["dalfox", "url", url, "--silence", "--no-color", "--no-spinner", *shlex.split(options)]
        return run("dalfox", argv, target=url).render()

    @mcp.tool()
    def wafw00f(target: str) -> str:
        """Detect and fingerprint a Web Application Firewall in front of a
        URL/host with wafw00f. `-a` reports all matching WAF signatures."""
        argv = ["wafw00f", "-a", target]
        return run("wafw00f", argv, target=target).render()

    @mcp.tool()
    def testssl(host: str, options: str = "") -> str:
        """Analyse a host's SSL/TLS configuration with testssl.sh: protocols,
        ciphers, cert chain and known TLS vulnerabilities (Heartbleed, ROBOT,
        etc.). `host` is host:port (port defaults to 443). Extra flags via
        `options`. Thorough — can take a while."""
        argv = ["testssl", "--quiet", "--color", "0", *shlex.split(options), host]
        return run("testssl", argv, target=host).render()

    @mcp.tool()
    def cmseek(url: str) -> str:
        """Detect the CMS behind a site and known issues with CMSeeK (180+ CMSs,
        broader than wpscan). Runs non-interactively in batch mode."""
        argv = ["cmseek", "--batch", "--follow-redirect", "-u", url]
        return run("cmseek", argv, target=url).render()
