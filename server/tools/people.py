"""People-footprint OSINT tools.

Passive reconnaissance on an *individual's* digital footprint — usernames,
email accounts, breach exposure, phone numbers and social profiles. Everything
here queries third-party/public sources only; nothing touches the subject's own
infrastructure. Still scope-sensitive: only profile people in-scope for the
engagement. Every call is audit-logged.

Many of these tools rely on third-party APIs and silently return thin results
without keys. The relevant API-key environment variables are listed per tool.
"""
from __future__ import annotations

import shlex

from ..runner import run


def register(mcp) -> None:
    @mcp.tool()
    def sherlock(username: str, timeout: int = 30) -> str:
        """Hunt a USERNAME across ~400 social networks and sites (sherlock).
        Fast, keyless, in-repo. Returns the sites where the username exists.
        Use `maigret_username` for a far broader (~2500-site) deep sweep."""
        argv = ["sherlock", "--print-found", "--no-color", "--timeout", str(timeout), username]
        return run("sherlock", argv, target=username).render()

    @mcp.tool()
    def maigret_username(username: str, timeout: int = 30, top_sites: int = 500) -> str:
        """Deep USERNAME enumeration across ~2500 sites with profile-metadata
        extraction (maigret). Broader and richer than sherlock but slower; cap
        breadth with `top_sites`. Keyless."""
        argv = [
            "maigret", username,
            "--no-color", "--no-progressbar",
            "--timeout", str(timeout),
            "--top-sites", str(top_sites),
        ]
        return run("maigret", argv, target=username).render()

    @mcp.tool()
    def holehe_email(email: str) -> str:
        """Discover which sites have an account registered to an EMAIL address
        (holehe), via silent password-reset/registration probes — it does not
        alert the account owner. Only prints sites where the email is in use."""
        argv = ["holehe", "--only-used", "--no-color", email]
        return run("holehe", argv, target=email).render()

    @mcp.tool()
    def h8mail_breach(email: str, options: str = "") -> str:
        """Check an EMAIL against breach/leak databases (h8mail). Surfaces
        breach presence and, with paid sources, leaked credentials. Largely
        keyless-limited — configure HIBP/Hunter/Snusbase/Dehashed/IntelX keys
        via a config file passed in `options` (e.g. '-c config.ini'), or point
        at a local breach compilation with '-bc /path --loose'."""
        argv = ["h8mail", "-t", email, *shlex.split(options)]
        return run("h8mail", argv, target=email).render()

    @mcp.tool()
    def phoneinfoga(number: str) -> str:
        """OSINT on a PHONE NUMBER (phoneinfoga): carrier, line type, country
        and footprint search links. Pass an E.164 number, e.g. '+15554441212'.
        Optional NUMVERIFY_API_KEY / GOOGLE_API_KEY+GOOGLECSE_CX enrich results."""
        argv = ["phoneinfoga", "scan", "-n", number]
        return run("phoneinfoga", argv, target=number).render()
