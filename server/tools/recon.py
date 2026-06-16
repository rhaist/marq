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
        # Kali ships ProjectDiscovery httpx as `httpx-toolkit` (plain `httpx` is
        # the unrelated python HTTP client).
        argv = ["httpx-toolkit", "-silent", "-status-code", "-title", "-tech-detect"]
        stdin = "\n".join(hosts)
        return run("httpx", argv, target=targets, stdin=stdin).render()

    @mcp.tool()
    def naabu(target: str, ports: str = "", top_ports: int = 100) -> str:
        """Fast modern port scan with naabu (projectdiscovery). Give explicit
        `ports` (e.g. "80,443,8000-9000") or rely on `top_ports`. SYN-scans with
        NET_RAW, else falls back to a connect scan. Pairs well with nmap -sV on
        the discovered ports."""
        argv = ["naabu", "-host", target, "-silent"]
        if ports:
            argv += ["-p", ports]
        else:
            argv += ["-top-ports", str(top_ports)]
        return run("naabu", argv, target=target).render()

    @mcp.tool()
    def dnsx(hosts: str, records: str = "a") -> str:
        """Resolve and enumerate DNS for hosts (comma/newline-separated) with
        dnsx. `records` is a comma list of types to query
        (a,aaaa,cname,mx,ns,txt,ptr,srv). Returns the records inline."""
        names = [h.strip() for h in hosts.replace(",", "\n").splitlines() if h.strip()]
        argv = ["dnsx", "-silent", "-resp"]
        for rec in (r.strip().lower() for r in records.split(",") if r.strip()):
            argv.append(f"-{rec}")
        return run("dnsx", argv, target=hosts, stdin="\n".join(names)).render()

    @mcp.tool()
    def dnsrecon(domain: str, scan_type: str = "std") -> str:
        """DNS reconnaissance with dnsrecon. `scan_type` selects the technique:
        std (records), brt (bruteforce — needs a wordlist via options is not
        exposed here), axfr (zone transfer), crt (crt.sh certs), zonewalk.
        Good for zone-transfer checks and record sweeps."""
        argv = ["dnsrecon", "-d", domain, "-t", scan_type]
        return run("dnsrecon", argv, target=domain).render()
