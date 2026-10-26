#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "$0")/../.."
image=${1:?usage: verify.sh IMAGE [TEST_IMAGE] [EVIDENCE_DIR]}
test_image=${2:-kpanel-browser:test}
evidence=${3:-$(mktemp -d /tmp/kpanel-browser-evidence.XXXXXXXX)}
mkdir -p "$evidence"
evidence=$(cd "$evidence" && pwd -P)
work=$(mktemp -d /tmp/kpanel-browser-check.XXXXXXXX)
container=
test_container=
cleanup() {
  result=$?
  trap - EXIT
  if [[ -n "$container" ]]; then
    docker logs "$container" > "$evidence/service.log" 2>&1 || result=1
    docker inspect "$container" > "$evidence/service-inspect.json" || result=1
    [[ $(docker inspect --format '{{.State.OOMKilled}}' "$container") == false ]] || result=1
    docker rm -f "$container" > /dev/null || result=1
  fi
  [[ -z "$test_container" ]] || docker rm -f "$test_container" > /dev/null || result=1
  # Only files created by this script; preserve evidence even on failure.
  rm -f -- "$work/token" "$work/browser-tests"
  rmdir -- "$work" || result=1
  printf 'browser_container_verification_exit=%s cleanup=completed\n' "$result" | tee "$evidence/result.txt"
  exit "$result"
}
trap cleanup EXIT
test "$(docker image inspect --format '{{.Config.User}}' "$image")" = 65532:65532
docker image inspect "$image" > "$evidence/image-inspect.json"
test_container=$(docker create "$test_image")
docker cp "$test_container:/usr/local/bin/browser-tests" "$work/browser-tests"
docker rm "$test_container" > /dev/null
test_container=
chmod 0555 "$work/browser-tests"
head -c 48 /dev/urandom | base64 > "$work/token"
chown 65532:65532 "$work/token" "$evidence"
chmod 0400 "$work/token"
chmod 0700 "$evidence"
chmod 0755 "$work"
runtime=(--user 65532:65532 --read-only --cap-drop ALL --security-opt no-new-privileges:true
  --security-opt "seccomp=$PWD/packaging/browser/browser-seccomp.json"
  --pids-limit 512 --memory 1536m --memory-swap 1536m --cpus 1.5
  --shm-size 256m --tmpfs /tmp:rw,size=256m,mode=1777)
timeout --kill-after=10s 210s docker run --rm "${runtime[@]}" "$test_image" -test.v -test.timeout=180s > "$evidence/browser-tests.log" 2>&1
container=$(docker run -d "${runtime[@]}" \
  --mount "type=bind,src=$work/token,dst=/run/secrets/browser-token,readonly" \
  --mount "type=bind,src=$work/browser-tests,dst=/test/browser-tests,readonly" \
  --mount "type=bind,src=$evidence,dst=/evidence" \
  "$image" -listen=0.0.0.0:8092 -browser=/usr/bin/chromium \
  -token-file=/run/secrets/browser-token -check-runtime -private-cidrs=127.0.0.1/32)
ready=false
for ((i=0;i<35;i++)); do
  if docker exec "$container" /usr/local/bin/kpanel-browser -browser=/usr/bin/chromium \
      -token-file=/run/secrets/browser-token -healthcheck >/dev/null 2>&1; then
    ready=true
    break
  fi
  [[ $(docker inspect --format '{{.State.Running}}' "$container") == true ]] || break
  sleep 1
done
test "$ready" = true
docker exec "$container" /usr/bin/chromium --version | tee "$evidence/chromium-version.txt"
docker exec "$container" cat /usr/share/kpanel-browser-packages.txt > "$evidence/packages.txt"
timeout --kill-after=10s 140s docker exec \
  -e KPANEL_TEST_SERVICE=http://127.0.0.1:8092 -e KPANEL_TEST_EVIDENCE=/evidence \
  "$container" /test/browser-tests -test.v -test.run '^TestContainerServiceRoundTrip$' -test.timeout=120s \
  > "$evidence/service-tests.log" 2>&1
# Preserve container configuration and positive test evidence, never the token.
test "$(docker inspect --format '{{.HostConfig.Privileged}}' "$container")" = false
test "$(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "$container")" = true
test "$(docker inspect --format '{{len .HostConfig.PortBindings}}' "$container")" = 0
echo 'browser_container=pass sandbox=enabled external_port=none'
