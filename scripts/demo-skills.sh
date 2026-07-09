#!/usr/bin/env bash
#
# demo-skills.sh — the "cyber skills for your LLM" story in one run, for
# recording (asciinema / GIF). Drives marq in MARQ_SKILLS_ONLY mode: the skills
# library + advisory tools only, NO Kali binaries, so it runs anywhere Go builds.
#
# It plays the knowledge loop a marketing demo should show:
#   server_info -> load_skill -> report_finding -> render_report -> the deliverable
#
# Usage:  scripts/demo-skills.sh          (record with: asciinema rec -c scripts/demo-skills.sh)
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)"
BIN="$WORK/marq"
export MARQ_SKILLS_ONLY=1 MARQ_WORK_DIR="$WORK" MARQ_AUDIT_LOG="$WORK/audit.jsonl"
trap 'rm -rf "$WORK"' EXIT

step() { printf '\n\033[1;36m$ %s\033[0m\n' "$*"; }        # cyan prompt line
run()  { "$BIN" run "$@" 2>/dev/null; }

( cd "$ROOT" && go build -o "$BIN" ./cmd/marq ) || { echo "build failed"; exit 1; }

step "marq run server_info   # knowledge-only mode — no tool suite loaded"
run server_info '{}'

step "marq run load_skill control-mapping   # 1 of ~73 expert playbooks"
run load_skill '{"name":"control-mapping"}' | head -18

step 'marq run report_finding "Missing MFA on admin console"   # advisory, no scan needed'
run report_finding '{"title":"Missing MFA on admin console","severity":"high","target":"admin.example.com","evidence":"Admin console reachable with a password alone","recommendation":"Enforce phishing-resistant MFA (FIDO2)","cwe":"CWE-308"}'

step "marq run render_report   # the engagement deliverable"
run render_report '{}'

step "cat findings.md"
cat "$WORK/findings.md"
printf '\n\033[1;32m# add MARQ_SKILLS_ONLY=1 to any MCP client — that is the whole setup.\033[0m\n'
