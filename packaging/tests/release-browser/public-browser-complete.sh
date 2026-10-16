#!/usr/bin/env bash
set -Eeuo pipefail

# Only GitHub-hosted ephemeral runners own the disposable test resources.
test "${GITHUB_ACTIONS:-}" = true && test "${RUNNER_ENVIRONMENT:-}" = github-hosted
: "${RUNNER_TEMP:?}" "${EXPECTED_VERSION:?}" "${EXPECTED_SHA:?}" "${RELEASE_TEST_RUNNER:?}" "${RELEASE_BROWSER_RUNTIME:?}"
image=${1:?immutable public image}; mode=${2:?published or upgrade}; root=${3:?unique root}; digest=${4:?preview digest}
[[ "$image" == docker.io/kjlion/kejilion-panel@sha256:* ]]
[[ "$root" == "$RUNNER_TEMP"/kpanel-browser-* ]]
case "$mode" in published|upgrade) ;; *) exit 2;; esac
test ! -e "$root"
install -d -m 755 "$root" "$root/results"
tools=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
data=$root/data;out=$root/results
test "$(df -Pk "$root" | awk 'NR==2 {print $4}')" -gt 2097152
test "$(awk '/MemAvailable/ {print $2}' /proc/meminfo)" -gt 1572864
port=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
install -d -m 750 "$data" "$data/ipc" "$data/agent" "$data/update" "$data/panel" "$data/web"
printf 'kejilion-panel:x:65532:\n' > "$data/group"
openssl rand -hex 32 > "$data/ipc/agent.token"
chown 0:65532 "$data/ipc/agent.token" "$data/ipc";chmod 640 "$data/ipc/agent.token"
chown 65532:65532 "$data/panel" "$out";chmod 700 "$data/panel" "$out"
printf '{"listen":":8080","dataDir":"/data/panel","agentSocket":"/ipc/agent.sock","agentTokenFile":"/ipc/agent.token","updateFreezeFile":"/data/update-freeze","webRoot":"/candidate-web","secureCookie":false,"allowIpHosts":true}\n' > "$data/panel.json"
chmod 644 "$data/panel.json" "$data/group"
name=$(basename "$root");agent=$name-agent;panel=$name-panel;browser=$name-browser;extract=$name-extract;watchdog_pid=
cleanup(){
 test -z "$watchdog_pid" || kill "$watchdog_pid" 2>/dev/null || true
 docker logs "$panel" > "$out/panel.log" 2>&1 || true
 docker logs "$agent" > "$out/agent.log" 2>&1 || true
 docker inspect "$panel" "$agent" > "$out/container-final.json" 2>/dev/null || true
 docker rm -f "$panel" "$agent" "$browser" "$extract" >/dev/null 2>&1 || true
 printf 'removed owned containers only\n' > "$out/cleanup.txt"
}
trap cleanup EXIT
docker pull "$image"
test "$(docker image inspect "$image" --format '{{index .Config.Labels "org.opencontainers.image.version"}}')" = "$EXPECTED_VERSION"
test "$(docker image inspect "$image" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')" = "$EXPECTED_SHA"
docker inspect "$image" > "$out/frontend-image.json"
docker create --name "$extract" "$image" >/dev/null
docker cp "$extract:/app/web/." "$data/web/"
docker cp "$extract:/release/kejilion-agent" "$data/agent-binary"
chmod 755 "$data/agent-binary"
docker rm "$extract" >/dev/null
chmod -R a+rX "$data/web"
backend=$image
if [ "$mode" = upgrade ]; then backend=$PREVIOUS_PREVIEW_IMAGE;docker pull "$backend";fi
docker inspect "$backend" > "$out/backend-image.json"
printf 'mode=%s\nfrontend_digest=%s\nbackend_image=%s\nport=%s\n' "$mode" "$digest" "$backend" "$port" > "$out/source-binding.txt"
(
 for i in $(seq 1 84); do
  disk=$(df -Pk "$root" | awk 'NR==2 {print $4}');memory=$(awk '/MemAvailable/ {print $2}' /proc/meminfo)
  printf '%s disk_kib=%s mem_available_kib=%s\n' "$(date -u +%FT%TZ)" "$disk" "$memory" >> "$out/watchdog.log"
  if [ "$disk" -lt 1048576 ] || [ "$memory" -lt 524288 ]; then touch "$out/watchdog.failed";docker rm -f "$panel" "$agent" "$browser" >/dev/null 2>&1 || true;exit 1;fi
  sleep 10
 done
) & watchdog_pid=$!
agent_image=$backend;agent_entry=/release/kejilion-agent;agent_mount=()
if [ "$mode" = published ]; then agent_image=$RELEASE_TEST_RUNNER;agent_entry=/candidate-agent;agent_mount=(-v "$data/agent-binary:/candidate-agent:ro");fi
docker run -d --name "$agent" --entrypoint "$agent_entry" "${agent_mount[@]}" --user 0:0 --read-only --cap-drop ALL --cap-add CHOWN --security-opt no-new-privileges --memory 512m --cpus 1 --pids-limit 256 --tmpfs /tmp:rw,size=67108864 -v "$data/group:/etc/group:ro" -v "$data/ipc:/ipc:rw" -v "$data/agent:/state:rw" -v "$data/update:/update:rw" "$agent_image" --socket /ipc/agent.sock --socket-group kejilion-panel --token-file /ipc/agent.token --state-dir /state --self-update-state-dir /update --web-root /absent-web --docker-socket /absent-docker.sock --docker-pid-file /absent-docker.pid --allow-docker-socket-activation=false --enable-system-writes=false --public-network-lookup=false
for i in $(seq 1 30);do test ! -S "$data/ipc/agent.sock" || break;sleep 1;done;test -S "$data/ipc/agent.sock"
docker run -d --name "$panel" --user 65532:65532 --read-only --cap-drop ALL --security-opt no-new-privileges --memory 512m --cpus 1 --pids-limit 256 --tmpfs /tmp:rw,size=67108864 -p 127.0.0.1:$port:8080 -v "$data/panel:/data/panel:rw" -v "$data/ipc:/ipc:ro" -v "$data/panel.json:/config/panel.json:ro" -v "$data/web:/candidate-web:ro" "$backend" --config /config/panel.json
for i in $(seq 1 60);do if test -f "$data/panel/bootstrap.token" && curl -fsS http://127.0.0.1:$port/api/v1/auth/bootstrap >/dev/null;then break;fi;sleep 1;done;test -f "$data/panel/bootstrap.token"
docker run --rm --name "$browser" --network host --user 65532:65532 --read-only --cap-drop ALL --security-opt no-new-privileges --memory 2g --cpus 1 --pids-limit 512 --tmpfs /tmp:rw,size=536870912 --shm-size 512m -v "$RELEASE_BROWSER_RUNTIME":/work/runtime:ro -v "$out:/work/evidence:rw" -v "$tools/public-browser-complete.cjs":/work/journey.cjs:ro -v "$data/panel/bootstrap.token:/work/bootstrap.token:ro" -e EXPECTED_VERSION -e EXPECTED_SHA -e PREVIOUS_PREVIEW_VERSION -e EXPECTED_NOTES -e EXPECTED_UPGRADE_NOTES -e EXPECTED_STABLE_VERSION mcr.microsoft.com/playwright@sha256:dcc5531e97840b9b5e794f2814476b21571c5124a3fca2267d73041f56e7580e node /work/journey.cjs "$port" "$mode" "$digest"
test ! -e "$out/watchdog.failed"
printf 'browser_gate=pass mode=%s\n' "$mode"
