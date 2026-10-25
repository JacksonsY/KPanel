#!/usr/bin/env bash
set -Eeuo pipefail

# Only GitHub-hosted ephemeral runners own the disposable test resources.
test "${GITHUB_ACTIONS:-}" = true && test "${RUNNER_ENVIRONMENT:-}" = github-hosted
: "${RUNNER_TEMP:?}" "${EXPECTED_VERSION:?}" "${EXPECTED_SHA:?}" "${RELEASE_TEST_RUNNER:?}" "${RELEASE_BROWSER_RUNTIME:?}"
image=${1:?immutable image};publication=${2:?candidate or public};root=${3:?unique owned root};digest=${4:?image identity}
repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
[[ "$root" == "$RUNNER_TEMP"/kpanel-real-* ]]
case "$publication:$image" in candidate:sha256:*|public:docker.io/kjlion/kejilion-panel@sha256:*) ;; *) exit 2;; esac
test ! -e "$root"
for tool in docker python3 openssl curl git sha256sum;do command -v "$tool" >/dev/null;done
test "$(df -Pk "$RUNNER_TEMP" | awk 'NR==2 {print $4}')" -gt 4194304
test "$(awk '/MemAvailable/ {print $2}' /proc/meminfo)" -gt 3145728
mkdir -p "$root/results" "$root/data" "$root/fixture"
data=$root/data;fixture=$root/fixture;out=$root/results
name=$(basename "$root");agent=$name-agent;panel=$name-panel;engine=$name-engine;proxy=$name-proxy;browser=$name-browser;extract=$name-extract;network=$name-network;watchdog=
cleanup(){
 test -z "$watchdog" || kill "$watchdog" 2>/dev/null || true
 for c in "$agent" "$panel" "$engine" "$proxy";do docker logs "$c" > "$out/$c.log" 2>&1 || true;docker inspect "$c" --format '{{json .State}}' > "$out/$c-state.json" 2>/dev/null || true;done
 docker rm -f "$agent" "$panel" "$engine" "$proxy" "$browser" "$extract" >/dev/null 2>&1 || true
 docker network rm "$network" >/dev/null 2>&1 || true
 printf 'Removed only the named test containers and owned test network; no host Docker socket was mounted into tested Panel/Agent.\n' > "$out/cleanup.txt"
}
trap cleanup EXIT
if [ "$publication" = public ];then docker pull "$image";fi
test "$(docker image inspect "$image" --format '{{index .Config.Labels "org.opencontainers.image.version"}}')" = "$EXPECTED_VERSION"
if [ "$publication" = public ];then test "$(docker image inspect "$image" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')" = "$EXPECTED_SHA";fi
script_identity=$(KPANEL_REPO_ROOT="$repo" bash "$repo/scripts/check-managed-script-contract.sh" "$image")
script_sha256=${script_identity##* sha256=}
[[ "$script_sha256" =~ ^[0-9a-f]{64}$ ]]
printf '%s\n' "$script_identity"
docker inspect "$image" > "$out/image.json"
mkdir -p "$data/ipc" "$data/agent" "$data/update" "$data/panel" "$data/private" "$data/app-home/kpanel/bin" "$data/apps-local" "$data/apps-official" "$out/initial" "$out/restart"
chmod 755 "$data/apps-local" "$data/apps-official"
chown 65532:65532 "$data/panel" "$data/private" "$out/initial" "$out/restart";chmod 700 "$data/panel" "$data/private"
printf 'kejilion-panel:x:65532:\n' > "$data/group"
openssl rand -hex 32 > "$data/ipc/agent.token";chown 0:65532 "$data/ipc" "$data/ipc/agent.token";chmod 750 "$data/ipc";chmod 640 "$data/ipc/agent.token"
docker create --name "$extract" "$image" >/dev/null
docker cp "$extract:/release/kejilion-agent" "$data/agent-binary"
docker cp "$extract:/release/kejilion.sh" "$data/app-home/kpanel/bin/kejilion.sh"
docker cp "$extract:/etc/ssl/certs/ca-certificates.crt" "$fixture/system-ca.crt"
docker rm "$extract" >/dev/null
chmod 755 "$data/agent-binary";chmod 700 "$data/app-home/kpanel/bin/kejilion.sh"
printf '%s  %s\n' "$script_sha256" "$data/app-home/kpanel/bin/kejilion.sh" | sha256sum -c -
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -keyout "$fixture/ca.key" -out "$fixture/ca.crt" -days 2 -subj /CN=KPanel-RC2-Isolated-Test-CA -addext basicConstraints=critical,CA:TRUE >/dev/null 2>&1
openssl req -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -keyout "$fixture/server.key" -out "$fixture/server.csr" -subj /CN=rc2-panel.test >/dev/null 2>&1
printf 'subjectAltName=DNS:rc2-panel.test,DNS:rc2-other.test,DNS:rc2-wrong.test,DNS:rc2-redirect.test\nextendedKeyUsage=serverAuth\n' > "$fixture/server.ext"
openssl x509 -req -in "$fixture/server.csr" -CA "$fixture/ca.crt" -CAkey "$fixture/ca.key" -CAcreateserial -out "$fixture/server.crt" -days 2 -extfile "$fixture/server.ext" >/dev/null 2>&1
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -keyout "$fixture/untrusted.key" -out "$fixture/untrusted.crt" -days 2 -subj /CN=rc2-untrusted.test -addext subjectAltName=DNS:rc2-untrusted.test >/dev/null 2>&1
chmod 644 "$fixture/ca.crt"
cat "$fixture/system-ca.crt" "$fixture/ca.crt" > "$fixture/ca-bundle.crt"
port=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
# This journey starts in explicit HTTP mode and enables real TLS through the UI.
# Declaring an HTTPS PublicURL here would correctly reject plaintext bootstrap.
printf '{"listen":":8080","dataDir":"/data/panel","agentSocket":"/ipc/agent.sock","agentTokenFile":"/ipc/agent.token","updateFreezeFile":"/data/update-freeze","webRoot":"/app/web","secureCookie":false,"allowIpHosts":true,"publicUrl":"http://rc2-panel.test"}\n' > "$data/panel.json"
docker network create "$network" > "$out/network.id"
docker network inspect "$network" > "$out/network.json"
runner=$RELEASE_TEST_RUNNER
tools=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
docker run -d --name "$engine" --network none --read-only --cap-drop ALL --security-opt no-new-privileges --memory 128m --cpus .25 --pids-limit 64 -v "$data/ipc:/ipc" -v "$tools/real-docker-fixture.mjs:/fixture.mjs:ro" --entrypoint node "$runner" /fixture.mjs >/dev/null
for i in $(seq 1 30);do test ! -S "$data/ipc/docker-fixture.sock" || break;sleep 1;done;test -S "$data/ipc/docker-fixture.sock"
docker run -d --name "$proxy" --network "$network" --network-alias rc2-panel.test --network-alias rc2-other.test --network-alias rc2-wrong.test --network-alias rc2-redirect.test --network-alias rc2-untrusted.test --read-only --cap-drop ALL --cap-add NET_BIND_SERVICE --security-opt no-new-privileges --memory 128m --cpus .25 --pids-limit 64 -v "$fixture:/fixture:ro" -v "$tools/real-domain-fixture.mjs:/fixture.mjs:ro" --entrypoint node "$runner" /fixture.mjs >/dev/null
docker run -d --name "$agent" --network "$network" --entrypoint /candidate-agent --user 0:0 --read-only --cap-drop ALL --cap-add CHOWN --security-opt no-new-privileges --memory 512m --cpus 1 --pids-limit 256 --tmpfs /tmp:rw,size=67108864 -v "$data/agent-binary:/candidate-agent:ro" -v "$data/group:/etc/group:ro" -v "$data/ipc:/ipc" -v "$data/agent:/state" -v "$data/update:/update" -v "$data/app-home:/home/docker" -v "$data/apps-local:/root/apps-local" -v "$data/apps-official:/root/apps:ro" "$runner" --socket /ipc/agent.sock --socket-group kejilion-panel --token-file /ipc/agent.token --state-dir /state --self-update-state-dir /update --web-root /absent-web --docker-socket /ipc/docker-fixture.sock --docker-pid-file /absent-docker.pid --allow-docker-socket-activation=true --enable-system-writes=false --public-network-lookup=false >/dev/null
for i in $(seq 1 30);do test ! -S "$data/ipc/agent.sock" || break;sleep 1;done;test -S "$data/ipc/agent.sock"
docker run -d --name "$panel" --network "$network" --network-alias panel --add-host rc2-loopback.test:127.0.0.1 --user 65532:65532 --read-only --cap-drop ALL --security-opt no-new-privileges --memory 512m --cpus 1 --pids-limit 256 --tmpfs /tmp:rw,size=67108864 -e SSL_CERT_FILE=/fixture/ca-bundle.crt -p 127.0.0.1:$port:8080 -v "$fixture/ca-bundle.crt:/fixture/ca-bundle.crt:ro" -v "$data/panel:/data/panel" -v "$data/ipc:/ipc:ro" -v "$data/panel.json:/config/panel.json:ro" "$image" --config /config/panel.json >/dev/null
for i in $(seq 1 60);do if test -f "$data/panel/bootstrap.token" && curl -fsS http://127.0.0.1:$port/api/v1/auth/bootstrap >/dev/null;then break;fi;sleep 1;done;test -f "$data/panel/bootstrap.token"
(
 for i in $(seq 1 180);do mem=$(awk '/MemAvailable/ {print $2}' /proc/meminfo);disk=$(df -Pk "$root" | awk 'NR==2 {print $4}');printf '%s mem_available_kib=%s disk_kib=%s\n' "$(date -u +%FT%TZ)" "$mem" "$disk" >> "$out/resources.log";if [ "$mem" -lt 524288 ] || [ "$disk" -lt 1048576 ];then touch "$out/resource-guard.failed";docker stop -t 2 "$browser" "$panel" "$agent" >/dev/null 2>&1 || true;exit 1;fi;sleep 3;done
) & watchdog=$!
run_browser(){
 local stage=$1
 local bootstrap=()
 if [ "$stage" = initial ];then bootstrap=(-v "$data/panel/bootstrap.token:/work/bootstrap.token:ro");fi
 docker run --rm --name "$browser" --network host --user 65532:65532 --read-only --cap-drop ALL --security-opt no-new-privileges --memory 2g --cpus 1 --pids-limit 512 --tmpfs /tmp:rw,size=536870912 --shm-size 512m -v "$RELEASE_BROWSER_RUNTIME":/work/runtime:ro -v "$out/$stage:/work/evidence" -v "$data/private:/work/private" -v "$tools/real-feature-browser-public.cjs:/work/journey.cjs:ro" "${bootstrap[@]}" -e EXPECTED_VERSION -e EXPECTED_SHA -e PREVIOUS_PREVIEW_VERSION -e EXPECTED_NOTES -e EXPECTED_UPGRADE_NOTES -e EXPECTED_STABLE_VERSION mcr.microsoft.com/playwright@sha256:dcc5531e97840b9b5e794f2814476b21571c5124a3fca2267d73041f56e7580e node /work/journey.cjs "$port" "$stage" "$publication" "$digest"
}
run_browser initial
test ! -e "$data/app-home/rc2-proof-executed"
test -f "$data/apps-local/rc2-proof.conf"
printf '\n# CLI boundary edit\n' >> "$data/apps-local/rc2-proof.conf"
docker restart "$agent" "$panel" >/dev/null
for i in $(seq 1 60);do curl -fkSs https://127.0.0.1:$port/api/v1/health >/dev/null 2>&1 && break;sleep 1;done
run_browser restart
test ! -e "$data/apps-local/rc2-proof.conf"
test ! -e "$data/app-home/rc2-proof-executed"
docker stop "$panel" >/dev/null
docker run --rm --user 65532:65532 --read-only --cap-drop ALL --security-opt no-new-privileges --memory 256m --cpus 1 --pids-limit 128 -v "$data/panel:/data/panel" -v "$data/panel.json:/config/panel.json:ro" "$image" self-signed-tls -config /config/panel.json disable > "$out/offline-recovery.txt"
test ! -e "$data/panel/self-signed-tls/enabled"
cp -p "$data/panel/self-signed-tls/cert.pem" "$data/private/cert-backup.pem"
touch "$data/panel/self-signed-tls/enabled"
chown 65532:65532 "$data/panel/self-signed-tls/enabled"
chmod 600 "$data/panel/self-signed-tls/enabled"
printf 'Simulated corrupted certificate while the persisted HTTPS enabled marker is present.\n' > "$out/certificate-fault-scenario.txt"
printf 'corrupt-test-certificate\n' > "$data/panel/self-signed-tls/cert.pem"
docker start "$panel" >/dev/null
for i in $(seq 1 60);do curl -fsS http://127.0.0.1:$port/api/v1/health > "$out/health-after-certificate-fallback.json" 2>/dev/null && break;sleep 1;done
test -s "$out/health-after-certificate-fallback.json"
docker logs "$panel" 2>&1 | grep -F 'self-signed certificate could not be loaded; secure transport policy retained' > "$out/certificate-fail-closed.txt"
test "$(curl -sS -o "$out/plain-session-after-certificate-fault.json" -w '%{http_code}' http://127.0.0.1:$port/api/v1/auth/session)" = 426
test "$(curl -sS -o "$out/plain-login-after-certificate-fault.json" -w '%{http_code}' -H 'Content-Type: application/json' -d '{}' http://127.0.0.1:$port/api/v1/auth/login)" = 426
python3 - "$out" <<'PY'
import json, pathlib, sys
root = pathlib.Path(sys.argv[1])
for name in ['plain-session-after-certificate-fault.json', 'plain-login-after-certificate-fault.json']:
    assert json.loads((root / name).read_text())['code'] == 'https_required'
PY
docker stop "$panel" >/dev/null
docker run --rm --user 65532:65532 --read-only --cap-drop ALL --security-opt no-new-privileges --memory 256m --cpus 1 --pids-limit 128 -v "$data/panel:/data/panel" -v "$data/panel.json:/config/panel.json:ro" "$image" self-signed-tls -config /config/panel.json disable > "$out/offline-recovery-after-certificate-fault.txt"
test ! -e "$data/panel/self-signed-tls/enabled"
docker start "$panel" >/dev/null
for i in $(seq 1 60);do curl -fsS http://127.0.0.1:$port/api/v1/health >/dev/null 2>&1 && break;sleep 1;done
test "$(curl -sS -o "$out/session-after-explicit-offline-recovery.json" -w '%{http_code}' http://127.0.0.1:$port/api/v1/auth/session)" = 401
docker stop "$panel" >/dev/null
cp -p "$data/private/cert-backup.pem" "$data/panel/self-signed-tls/cert.pem"
test ! -e "$out/resource-guard.failed"
printf 'passed\n' > "$out/status.txt"
printf 'real_feature_gate=pass publication=%s image=%s\n' "$publication" "$image"
