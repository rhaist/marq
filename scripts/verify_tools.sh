#!/usr/bin/env bash
#
# verify_tools.sh — one-stop verification for the marq toolkit.
#
# Two phases, both against a throwaway container (never touches your local
# model runtime or a running marq container):
#   1. HELP   — for every wrapped exec tool, print the exact invocation the MCP
#               wrapper uses next to the tool's own --help/usage, so the flags
#               can be verified by eye (passive OSINT tools only; no capabilities needed).
#   2. TEST   — run scripts/test_tools.py, which drives the live MCP server and
#               calls every tool with benign args.
#
# Usage:
#   scripts/verify_tools.sh                 # help dump + in-image smoke test
#   scripts/verify_tools.sh --build         # docker build the image first
#   scripts/verify_tools.sh --help-only     # just the help/invocation dump
#   scripts/verify_tools.sh --test-only     # just the smoke test
#   scripts/verify_tools.sh --external   # forwarded to test_tools.py
#   IMAGE=marq:dev scripts/verify_tools.sh
#
set -uo pipefail

IMAGE="${IMAGE:-marq}"
LINES="${LINES:-35}"
HELP_ONLY=0 TEST_ONLY=0 BUILD=0
PASSTHROUGH=()
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --build)      BUILD=1 ;;
    --help-only)  HELP_ONLY=1 ;;
    --test-only)  TEST_ONLY=1 ;;
    --lines)      LINES="$2"; shift ;;
    --image)      IMAGE="$2"; shift ;;
    *)            PASSTHROUGH+=("$1") ;;   # e.g. --external --only ...
  esac
  shift
done

# name @@ wrapper-invocation (flags the wrapper passes) @@ help command
# Keep this in sync with internal/registry/{recon,osint,people}.go — the point is to diff the two.
TOOLS=(
  "dns_lookup@@dig +nocmd +noall +answer <domain> <type>@@dig -h"
  "whois_lookup@@whois <target>@@whois --help"
  "subfinder@@subfinder -silent -d <domain>@@subfinder -h"
  "dnsx@@dnsx -silent -resp -<record>@@dnsx -h"
  "asnmap@@whois -h whois.cymru.com \" -v <ip>\" + RIPEstat@@whois --version"
  "cdncheck@@cdncheck -silent (stdin IPs)@@cdncheck -h"
  "censys_view@@censys view <assets> -O json --no-spinner --no-color@@censys view --help"
  "theharvester@@theHarvester -d <domain> -b <sources> -l <limit>@@theHarvester -h"
  "spiderfoot@@spiderfoot -s <target> -u passive -o json   (run in background)@@spiderfoot -h"
  "exif_metadata@@exiftool -r <path>@@exiftool -ver"
  "shodan_host@@InternetDB (keyless) / shodan host <ip>@@shodan host --help"
  "shodan_search@@shodan search --fields ... --limit <n> <query>@@shodan search --help"
  "trufflehog@@trufflehog <source...> --no-verification --json@@trufflehog --help"
  "gau_urls@@echo <domain> | gau --subs --providers wayback,commoncrawl,otx,urlscan   (background)@@gau --help"
  "sherlock@@sherlock --print-found --no-color --timeout <t> <username>@@sherlock --help"
  "maigret_username@@maigret <username> --no-color --no-progressbar --timeout <t> --top-sites <n>@@maigret --help"
  "email_accounts@@user-scanner -e <email>@@user-scanner --help"
  "h8mail_breach@@h8mail -t <email> <options>@@h8mail --help"
  "phoneinfoga@@phoneinfoga scan -n <number>@@phoneinfoga scan --help"
)

dump_help() {
  echo "############################################################"
  echo "#  HELP / INVOCATION VERIFICATION  (image: $IMAGE)"
  echo "#  '\$ ...' is what the MCP wrapper runs; below it is the tool's help."
  echo "############################################################"
  # Build a single in-container script so we only spawn one container.
  local script=""
  for entry in "${TOOLS[@]}"; do
    local name="${entry%%@@*}"; local rest="${entry#*@@}"
    local inv="${rest%%@@*}"; local helpcmd="${rest#*@@}"
    script+="printf '\n═══════════════ %s ═══════════════\n' $(printf '%q' "$name");"
    script+="printf '\$ %s\n--- help ---\n' $(printf '%q' "$inv");"
    script+="{ ${helpcmd} ; } 2>&1 | head -n ${LINES};"
  done
  docker run --rm "$IMAGE" bash -c "$script"
}

run_tests() {
  echo
  echo "############################################################"
  echo "#  MCP SMOKE TEST"
  echo "############################################################"
  python3 "$REPO_ROOT/scripts/test_tools.py" --image "$IMAGE" "${PASSTHROUGH[@]}"
}

# --- main ---
if [[ "$BUILD" == 1 ]]; then
  echo "# building $IMAGE ..."
  docker build -t "$IMAGE" "$REPO_ROOT" || exit 1
fi

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "error: image '$IMAGE' not found — build it or pass --build" >&2
  exit 1
fi

rc=0
[[ "$TEST_ONLY" == 1 ]] || dump_help
[[ "$HELP_ONLY" == 1 ]] || { run_tests; rc=$?; }
exit "$rc"
