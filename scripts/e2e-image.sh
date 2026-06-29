#!/usr/bin/env bash
#
# e2e-image.sh — end-to-end test of the promises that only hold for the running
# Kali image (so they can't be checked by scripts/e2e.sh against the bare binary):
#
#   1. /work bind-mount persistence — findings + engagement context written by
#      one container survive into a *fresh* container on the same mount.
#   2. jwt_tool keys are unique per container — the deliberate per-container
#      keypair regeneration (so marq-driven JWT tests carry a detectable, unique
#      IOC rather than a baked-in shared key).
#
# Needs a built image. Build it yourself (`docker build -t marq .`) or pass
# --build. The wrapped-binary plumbing is covered separately by verify_tools.sh.
#
# Usage:  scripts/e2e-image.sh [--build] [--image NAME]
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${IMAGE:-marq}"
BUILD=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --build) BUILD=1 ;;
    --image) IMAGE="$2"; shift ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
  shift
done

pass=0 fail=0
check() { if [[ "$2" == 0 ]]; then printf '  ok   %s\n' "$1"; ((pass++)); else printf '  FAIL %s\n' "$1"; ((fail++)); fi; }

HOSTWORK="$(mktemp -d)"
chmod 777 "$HOSTWORK"   # the unprivileged marq user (foreign uid) must write the bind mount
C1="marq-e2e-jwt-1" C2="marq-e2e-jwt-2"
cleanup() { docker rm -f "$C1" "$C2" >/dev/null 2>&1; rm -rf "$HOSTWORK"; }
trap cleanup EXIT

if [[ "$BUILD" == 1 ]]; then
  echo "# building $IMAGE ..."
  docker build -t "$IMAGE" "$ROOT" || exit 1
fi
docker image inspect "$IMAGE" >/dev/null 2>&1 || { echo "error: image '$IMAGE' not found — build it or pass --build" >&2; exit 1; }

echo "# 1. /work bind-mount persistence across containers"
# Container A records an engagement + a finding into the mounted /work, then exits.
docker run --rm -v "$HOSTWORK":/work "$IMAGE" bash -c '
  marq run set_engagement '"'"'{"engagement":"img-e2e","scope":"*.img.test"}'"'"' >/dev/null
  marq run report_finding '"'"'{"title":"IMG persistence finding","severity":"high","target":"app.img.test","evidence":"e","recommendation":"r"}'"'"' >/dev/null
  marq run render_report '"'"'{}'"'"' >/dev/null' >/dev/null 2>&1

[[ -f "$HOSTWORK/findings.md" ]]; check "container-written findings.md lands on the host mount" $?
[[ -f "$HOSTWORK/.marq-context" ]]; check "engagement context persists to the host mount" $?

# Container B (fresh) reads the same mount back — proves cross-container persistence.
readback="$(docker run --rm -v "$HOSTWORK":/work "$IMAGE" marq run read_file '{"path":"/work/findings.md"}' 2>/dev/null)"
grep -q 'IMG persistence finding' <<<"$readback"; check "a fresh container reads the prior finding" $?
si="$(docker run --rm -v "$HOSTWORK":/work "$IMAGE" marq run server_info '{}' 2>/dev/null)"
grep -q 'engagement : img-e2e' <<<"$si"; check "a fresh container inherits the persisted engagement" $?

echo "# 2. jwt_tool keys are unique per container"
# Serve mode runs init_jwt_keys (rm -rf ~/.jwt_tool; jwt_tool x) before serving.
docker run -d --rm -i --name "$C1" "$IMAGE" >/dev/null
docker run -d --rm -i --name "$C2" "$IMAGE" >/dev/null
keyhash() { # poll until the generated keypair exists, then hash the .pem files
  local c="$1" i
  for i in $(seq 1 30); do
    if docker exec "$c" bash -c 'ls "$HOME"/.jwt_tool/*.pem >/dev/null 2>&1'; then
      docker exec "$c" bash -c 'cat "$HOME"/.jwt_tool/*.pem 2>/dev/null | sha256sum | cut -d" " -f1'
      return 0
    fi
    sleep 1
  done
  echo "TIMEOUT-$c"
}
h1="$(keyhash "$C1")"; h2="$(keyhash "$C2")"
[[ "$h1" != TIMEOUT-* && "$h1" != "" ]]; check "container 1 generated jwt keys ($h1)" $?
[[ "$h1" != "$h2" ]]; check "two containers have different jwt key material" $?

echo
echo "# result: $pass passed, $fail failed"
[[ "$fail" == 0 ]]
