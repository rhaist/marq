"""Reconnaissance and network discovery tools."""
from __future__ import annotations

import shlex

from ..runner import run


def register(mcp) -> None:
    @mcp.tool()
    def nmap(target: str, options: str = "-sV -T4") -> str:
        """Run an nmap scan. `target` is a host, CIDR or hostname; `options`
        are raw nmap flags (default a service/version scan). Use only against
        authorized targets — every scan is audit-logged."""
        argv = ["nmap", *shlex.split(options), target]
        return run("nmap", argv, target=target).render()

    @mcp.tool()
    def masscan(target: str, ports: str = "1-1000", rate: int = 1000) -> str:
        """Fast port sweep with masscan across `target` (CIDR/host). `rate` is
        packets/sec — keep it conservative on shared networks."""
        argv = ["masscan", target, "-p", ports, "--rate", str(rate)]
        return run("masscan", argv, target=target).render()

    @mcp.tool()
    def dns_lookup(domain: str, record_type: str = "ANY") -> str:
        """Query DNS records for a domain using dig."""
        argv = ["dig", "+nocmd", "+noall", "+answer", domain, record_type]
        return run("dig", argv, target=domain).render()

    @mcp.tool()
    def whois_lookup(target: str) -> str:
        """WHOIS registration lookup for a domain or IP."""
        return run("whois", ["whois", target], target=target).render()

    @mcp.tool()
    def subfinder(domain: str) -> str:
        """Passive subdomain enumeration for a domain via subfinder."""
        argv = ["subfinder", "-silent", "-d", domain]
        return run("subfinder", argv, target=domain).render()

    @mcp.tool()
    def httpx_probe(targets: str) -> str:
        """Probe one or more hosts (comma- or newline-separated) for live HTTP
        services, returning status, title and tech. Wraps projectdiscovery httpx."""
        hosts = [h.strip() for h in targets.replace(",", "\n").splitlines() if h.strip()]
        argv = ["httpx", "-silent", "-status-code", "-title", "-tech-detect"]
        stdin = "\n".join(hosts)
        return run("httpx", argv, target=targets, stdin=stdin).render()
