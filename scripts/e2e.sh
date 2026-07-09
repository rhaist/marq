#!/usr/bin/env bash
#
# e2e.sh — end-to-end test of marq's user-facing promises through the real
# `marq run` CLI (the same interface the Pi shim and docs use). Builds the
# binary once and drives the full engagement journey against a temp workdir.
#
# These are the promises that live in the Go binary and the in-process tools —
# scope/engagement, skills, findings, audit, file sandbox, catalog. They need
# NO Kali image, so this runs in seconds. The image-dependent promises (the ~80
# wrapped binaries actually executing, /work bind-mount + jwt-key uniqueness
# across containers) are covered by scripts/verify_tools.sh against a built image.
#
# Usage:  scripts/e2e.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="${MARQ_E2E_WORK:-$(mktemp -d)}"   # override to a pre-authorized dir if needed
BIN="$WORK/marq"
export MARQ_WORK_DIR="$WORK" MARQ_AUDIT_LOG="$WORK/audit.jsonl"
unset MARQ_OPERATOR MARQ_ENGAGEMENT MARQ_SCOPE   # exercise the defaults
trap 'rm -rf "$WORK"' EXIT

pass=0 fail=0 skip=0
check() { # check <name> <condition-already-evaluated:0/1>
  if [[ "$2" == 0 ]]; then printf '  ok   %s\n' "$1"; ((pass++))
  else printf '  FAIL %s\n' "$1"; ((fail++)); fi
}
skip() { printf '  skip %s (%s)\n' "$1" "$2"; ((skip++)); }
run() { "$BIN" run "$@" 2>/dev/null; }   # marq run <tool> '<json>'

echo "# building marq -> $BIN"
( cd "$ROOT" && go build -o "$BIN" ./cmd/marq ) || { echo "build failed"; exit 1; }

echo "# 1. scope & engagement"
out="$(run server_info '{}')"
grep -q 'operator   : marq' <<<"$out"; check "server_info shows default operator 'marq'" $?
grep -qi 'AUTHORIZED USE ONLY' <<<"$out"; check "server_info carries the authorization notice" $?

run set_engagement '{"engagement":"e2e-2026","scope":"*.e2e.test, 10.0.0.0/8"}' >/dev/null
[[ -f "$WORK/.marq-context" ]]; check "set_engagement persists .marq-context" $?
grep -q '10.0.0.0/8' "$WORK/.marq-context" 2>/dev/null; check ".marq-context holds the scope" $?
# fresh process must re-read the persisted context (the Pi process-per-call promise)
fresh="$(run server_info '{}')"
grep -q 'engagement : e2e-2026' <<<"$fresh"; check "engagement survives across processes" $?
grep -q 'scope note : \*.e2e.test' <<<"$fresh"; check "scope survives across processes" $?

echo "# 2. skills library (load_skill)"
names="$(grep -rh '^name:' "$ROOT/internal/skills/library" --include='*.md' | sed 's/name: *//' | tr -d '\r')"
total="$(wc -l <<<"$names" | tr -d ' ')"
bad=0
while IFS= read -r n; do
  [[ -z "$n" ]] && continue
  o="$(run load_skill "{\"name\":\"$n\"}")"
  grep -q "## skill: $n" <<<"$o" || { echo "    - $n did not load"; bad=1; }
done <<<"$names"
check "all $total skills load via load_skill" "$bad"
grep -qi 'unknown skill' <<<"$(run load_skill '{"name":"definitely-not-a-skill"}')"; check "unknown skill fails gracefully" $?

echo "# 3. findings deliverable (report_finding -> render_report)"
# No severity given + a critical CVSS vector → severity must be derived as critical.
run report_finding '{"title":"E2E SQLi","target":"https://app.e2e.test","evidence":"id=1'"'"' UNION ...","recommendation":"parameterize","cvss":"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"}' >/dev/null
run render_report '{}' >/dev/null
[[ -f "$WORK/findings.md" && -f "$WORK/findings.csv" ]]; check "render_report writes findings.md + findings.csv" $?
grep -q 'E2E SQLi' "$WORK/findings.md" 2>/dev/null; check "findings.md contains the finding" $?
grep -qi 'critical' "$WORK/findings.md" 2>/dev/null; check "CVSS 9.8 vector derives Critical severity" $?
[[ "$(wc -l <"$WORK/findings.csv")" -ge 2 ]]; check "findings.csv has a header + row" $?

echo "# 4. file sandbox (/work, /tmp only)"
# The sandbox resolves symlinks before the root check (anti-escape). On macOS
# /tmp -> /private/tmp, so a host /tmp path is (correctly) rejected; the positive
# round-trip is only meaningful where /tmp is a real dir (the Linux container).
if [[ "$(cd /tmp && pwd -P)" == "/tmp" ]]; then
  probe="/tmp/marq-e2e-$$.txt"
  run write_file "{\"path\":\"$probe\",\"content\":\"hello-e2e\"}" >/dev/null
  grep -q 'hello-e2e' <<<"$(run read_file "{\"path\":\"$probe\"}")"; check "write_file/read_file round-trip inside /tmp" $?
  rm -f "$probe"
else
  skip "write_file/read_file round-trip inside /tmp" "host /tmp is a symlink — container-only"
fi
esc="$(run write_file '{"path":"/etc/marq-e2e-escape","content":"x"}')"
grep -q 'outside the allowed area' <<<"$esc"; check "write_file outside /work,/tmp is refused" $?
[[ ! -f /etc/marq-e2e-escape ]]; check "sandbox escape created no file" $?

echo "# 5. audit log (one start+end per audited call)"
[[ -f "$WORK/audit.jsonl" ]]; check "audit.jsonl exists" $?
python3 -c 'import json,sys; [json.loads(l) for l in open(sys.argv[1]) if l.strip()]' "$WORK/audit.jsonl"; check "audit.jsonl is valid JSON lines" $?
s=$(grep -c '"invocation.start"' "$WORK/audit.jsonl"); e=$(grep -c '"invocation.end"' "$WORK/audit.jsonl")
[[ "$s" -ge 3 && "$s" == "$e" ]]; check "audit start/end records balance ($s/$e)" $?
grep -q 'set_engagement' "$WORK/audit.jsonl"; check "set_engagement was audited" $?

echo "# 6. tool catalog"
catalog="$("$BIN" tools)"
n=$(wc -l <<<"$catalog" | tr -d ' ')
[[ "$n" -ge 80 ]]; check "catalog lists >= 80 tools ($n)" $?
for t in server_info set_engagement load_skill report_finding render_report; do
  grep -q "^$t " <<<"$catalog"; check "catalog includes $t" $?
done
run no_such_tool '{}' >/dev/null; [[ $? -ne 0 ]]; check "unknown tool exits non-zero" $?

echo "# 7. skills-only mode (MARQ_SKILLS_ONLY — knowledge server, no Kali binaries)"
lite="$(MARQ_SKILLS_ONLY=1 "$BIN" tools)"
grep -q '^load_skill ' <<<"$lite"; check "skills-only keeps load_skill" $?
grep -q '^report_finding ' <<<"$lite"; check "skills-only keeps report_finding" $?
! grep -q '^nmap ' <<<"$lite"; check "skills-only drops exec tools (nmap absent)" $?
grep -qi 'knowledge-only mode' <<<"$(MARQ_SKILLS_ONLY=1 run server_info '{}')"; check "server_info announces knowledge-only mode" $?

echo
echo "# result: $pass passed, $fail failed, $skip skipped"
[[ "$fail" == 0 ]]
