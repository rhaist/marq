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
