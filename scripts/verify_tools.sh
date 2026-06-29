#!/usr/bin/env bash
#
# verify_tools.sh — one-stop verification for the marq toolkit.
#
# Two phases, both against a throwaway container (never touches the LM Studio
# instance):
#   1. HELP   — for every wrapped tool, print the exact invocation the MCP
#               wrapper uses next to the tool's own --help/usage, so the flags
#               can be verified by eye (this is how the httpx-toolkit / amass /
#               dalfox mismatches were caught).
#   2. TEST   — run scripts/test_tools.py, which drives the live MCP server and
#               calls every tool with benign args.
#
# Usage:
#   scripts/verify_tools.sh                 # help dump + local/passive smoke test
#   scripts/verify_tools.sh --build         # docker build the image first
#   scripts/verify_tools.sh --help-only     # just the help/invocation dump
#   scripts/verify_tools.sh --test-only     # just the smoke test
#   scripts/verify_tools.sh --external --slow   # forwarded to test_tools.py
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
    *)            PASSTHROUGH+=("$1") ;;   # e.g. --external --slow --only ...
  esac
  shift
done

# Capabilities the scanners' file-caps require in order to exec.
CAPS=(--cap-add NET_RAW --cap-add NET_ADMIN --cap-add NET_BIND_SERVICE)

# name @@ wrapper-invocation (flags the wrapper passes) @@ help command
# Keep this in sync with server/tools/*.py — the point is to diff the two.
TOOLS=(
  "nmap@@nmap <options:-sV -T4> <target>@@nmap -h"
  "masscan@@masscan <target> -p <ports> --rate <rate>@@masscan --help"
  "naabu@@naabu -host <t> -silent (-p <ports> | -top-ports <n>)@@naabu -h"
  "dns_lookup@@dig +nocmd +noall +answer <domain> <type>@@dig -h"
  "whois_lookup@@whois <target>@@whois --help"
  "subfinder@@subfinder -silent -d <domain>@@subfinder -h"
  "httpx_probe@@httpx-toolkit -silent -status-code -title -tech-detect@@httpx-toolkit -h"
  "dnsx@@dnsx -silent -resp -<record>@@dnsx -h"
  "dnsrecon@@dnsrecon -d <domain> -t <std|brt|axfr|crt|zonewalk>@@dnsrecon -h"
  "theharvester@@theHarvester -d <domain> -b <sources> -l <limit>@@theHarvester -h"
  "spiderfoot@@spiderfoot -s <target> -u <use_case> -o json   (run in background)@@spiderfoot -h"
  "exif_metadata@@exiftool -r <path>@@exiftool -ver"
  "shodan@@shodan host <ip>  /  shodan search --fields ... --limit <n> <query>@@shodan --help"
  "gitleaks@@gitleaks dir <path> --report-format json --report-path /dev/stdout@@gitleaks dir --help"
  "trufflehog@@trufflehog <source...> --results=verified --json@@trufflehog --help"
  "wayback_urls@@echo <domain> | waybackurls [-no-subs]@@waybackurls -h"
  "gau_urls@@echo <domain> | gau --subs --providers wayback,commoncrawl,otx,urlscan@@gau --help"
  "sherlock@@sherlock --print-found --no-color --timeout <t> <username>@@sherlock --help"
  "maigret_username@@maigret <username> --no-color --no-progressbar --timeout <t> --top-sites <n>@@maigret --help"
  "holehe_email@@holehe --only-used --no-color <email>@@holehe --help"
  "h8mail_breach@@h8mail -t <email> <options>@@h8mail --help"
  "phoneinfoga@@phoneinfoga scan -n <number>@@phoneinfoga scan --help"
  "nuclei@@nuclei -silent -u <target> [-t <tpl>] [-severity <sev>]@@nuclei -h"
  "nikto@@nikto -h <target>@@nikto -Help"
  "ffuf@@ffuf -w <wordlist> -u <url-with-FUZZ> [options]@@ffuf -h"
  "gobuster_dir@@gobuster dir -q -u <url> -w <wordlist>@@gobuster dir --help"
  "whatweb@@whatweb <target>@@whatweb --help"
  "wpscan@@wpscan --url <url> <options>@@wpscan --help"
  "sqlmap@@sqlmap -u <url> <options:--batch>@@sqlmap -h"
  "katana@@katana -u <url> -silent -d <depth> [-jc]@@katana -h"
  "feroxbuster@@feroxbuster -u <url> -w <wordlist> --silent --no-state@@feroxbuster --help"
  "arjun@@arjun -u <url> -m <GET|POST|JSON|XML>@@arjun -h"
  "dalfox@@dalfox url <url> --silence --no-color --no-spinner@@dalfox url --help"
  "wafw00f@@wafw00f -a <target>@@wafw00f -h"
  "testssl@@testssl --quiet --color 0 <host>@@testssl --help"
  "cmseek@@cmseek --batch --follow-redirect -u <url>@@cmseek --help"
  "searchsploit@@searchsploit <query>@@searchsploit -h"
  "hydra@@hydra <options> <target> <service>@@hydra -h"
  "john@@john <options> <hash_file>@@john --help"
  "hashcat@@hashcat -m <mode> --force <options> <hash_file> <wordlist>@@hashcat --version"
  "hash_identify@@hashid <hash>@@hashid -h"
  "ssh_audit@@ssh-audit -p <port> <host>@@ssh-audit -h"
  "fping_sweep@@fping -a -q -g <target>@@fping -h"
  "snmp_walk@@snmpwalk -v 2c -c <community> <host>@@snmpwalk -h"
  "snmp_check@@snmpcheck -t <host> -c <community>@@snmpcheck -h"
  "snmp_brute@@onesixtyone -c <wordlist> <host>@@onesixtyone -h"
  "smtp_user_enum@@smtp-user-enum -M <method> -U <users> -t <host>@@smtp-user-enum -h"
  "smtp_test@@swaks --server <target> <options>@@swaks --help"
  "asnmap@@asnmap -silent <target>@@asnmap -h"
  "cdncheck@@cdncheck -silent (stdin IPs)@@cdncheck -h"
  "censys_search@@censys search <query> --type <hosts|certs>@@censys search --help"
  "impacket_secretsdump@@impacket-secretsdump <options> <target>@@impacket-secretsdump -h"
  "impacket_kerberoast@@impacket-GetUserSPNs <options> <target>@@impacket-GetUserSPNs -h"
  "impacket_asreproast@@impacket-GetNPUsers <options> <target>@@impacket-GetNPUsers -h"
  "impacket_psexec@@impacket-psexec <options> <target> <command>@@impacket-psexec -h"
  "impacket_wmiexec@@impacket-wmiexec <options> <target> <command>@@impacket-wmiexec -h"
  "impacket_ntlmrelayx@@impacket-ntlmrelayx <options> (background)@@impacket-ntlmrelayx -h"
  "netexec@@netexec <protocol> <options> <target>@@netexec -h"
  "certipy_find@@certipy-ad find <options> -u <user> -p <pass>@@certipy-ad find --help"
  "bloodhound_collect@@bloodhound-python -u <u> -p <p> -d <domain> -ns <DC> -c All@@bloodhound-python -h"
  "evil_winrm@@evil-winrm -i <host:port> <options>@@evil-winrm -h"
  "enum4linux@@enum4linux-ng <options> <target>@@enum4linux-ng -h"
  "smb_enum@@smbmap -H <target> <options>@@smbmap -h"
  "ldap_search@@ldapsearch -H <target> <options> <filter>@@ldapsearch -h"
  "responder@@responder <options> (background)@@responder -h"
  "nbtscan@@nbtscan <target>@@nbtscan -h"
  "subjack@@subjack <options> <target>@@subjack -h"
  "paramspider@@paramspider -d <domain> <options>@@paramspider -h"
  "sstimap@@sstimap -u <url> <options>@@sstimap -h"
  "donut@@go-donut -i <input> -o <output> <options>@@go-donut -h"
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
  docker run --rm "${CAPS[@]}" "$IMAGE" bash -c "$script"
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
