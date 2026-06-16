"""Offline password / hash cracking tools.

These operate on local hash files inside the container — useful for validating
password policy strength on data you are authorized to assess.
"""
from __future__ import annotations

import shlex

from ..runner import run


def register(mcp) -> None:
    @mcp.tool()
    def john(hash_file: str, options: str = "") -> str:
        """Crack hashes in a local file with John the Ripper. Provide extra
        flags (e.g. --format=, --wordlist=) via `options`. `hash_file` must be a
        path inside the container."""
        argv = ["john", *shlex.split(options), hash_file]
        return run("john", argv, target=hash_file).render()

    @mcp.tool()
    def hashcat(hash_file: str, mode: int, wordlist: str, options: str = "") -> str:
        """Crack hashes with hashcat. `mode` is the -m hash type, `wordlist` an
        attack dictionary path inside the container. Extra flags via `options`.
        Note: GPU acceleration requires the container to expose a GPU."""
        argv = ["hashcat", "-m", str(mode), *shlex.split(options), hash_file, wordlist]
        return run("hashcat", argv, target=hash_file).render()

    @mcp.tool()
    def hash_identify(hash_value: str) -> str:
        """Identify the likely type of a hash string using hashid."""
        return run("hashid", ["hashid", hash_value], target="(hash)").render()
