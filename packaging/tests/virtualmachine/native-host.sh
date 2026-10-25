#!/usr/bin/env bash
set -Eeuo pipefail
test "${GITHUB_ACTIONS:-}" = true
test "${RUNNER_ENVIRONMENT:-}" = github-hosted
test "${GITHUB_REPOSITORY:-}" = kejilion/KPanel
test "${GITHUB_REF:-}" = refs/heads/main
: "${RUNNER_TEMP:?}" "${GITHUB_WORKSPACE:?}" "${EXPECTED_CANDIDATE:?}" "${GITHUB_SHA:?}"
[[ "$GITHUB_RUN_ID" =~ ^[1-9][0-9]*$ && "$GITHUB_RUN_ATTEMPT" =~ ^[1-9][0-9]*$ ]]
[[ "$EXPECTED_CANDIDATE" =~ ^[a-f0-9]{40}$ ]]
test "$(git rev-parse HEAD)" = "$EXPECTED_CANDIDATE"
test -z "$(git status --porcelain)"
test -f internal/virtualmachine/jobs.go
key="$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT"
proof="$RUNNER_TEMP/kpanel-native-$key"
test ! -e "$proof"
mkdir -m 755 "$proof" "$proof/evidence" "$proof/build" "$proof/http"
evidence="$proof/evidence"
controller="$GITHUB_WORKSPACE/controller"
image='bash:5.2.37-alpine3.22@sha256:3bee76a96d86d5d2d5efc7c1c570e5a7c95db22348a26944e0e546fa174e3324'
network="kpanel-native-$key"; server="kpanel-native-http-$key"
install_dir="/usr/local/libexec/kpanel-release-qualification-$key"
state_dir="/var/lib/kpanel-release-qualification-$key"
name="kpanel-qa-$key"
cleanup() {
  docker logs "$server" > "$evidence/docker-http.log" 2>&1 || true
  docker rm -f "$server" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  # Root filesystem changes belong only to this ephemeral hosted runner. Native
  # test cleanup stops its exact task units/domain; failed evidence is retained.
  printf 'Attempted cleanup of exact owned Docker resources; command failures may remain. Hosted runner disposal is the final resource boundary.\n' > "$evidence/cleanup.txt"
}
trap cleanup EXIT
exec > >(tee "$evidence/native-host.log") 2>&1
test -c /dev/kvm
test "$(go env GOVERSION)" = go1.27.3
test "$(ps -p 1 -o comm=)" = systemd
git ls-files '*.go' | LC_ALL=C sort | while IFS= read -r file; do sha256sum "$file"; done > "$evidence/go-sources-before.sha256"
git rev-parse 'HEAD^{tree}' > "$evidence/source-tree.txt"
go version > "$evidence/go-version.txt"
cat /etc/os-release > "$evidence/os-release.txt"
sha256sum "$controller/.github/workflows/virtual-machine-native-verify.yml" "$controller/packaging/tests/virtualmachine/native-host.sh" "$controller/packaging/tests/virtualmachine/native-host_test.go.tmpl" > "$evidence/controller-sources.sha256"
cp "$controller/packaging/tests/virtualmachine/native-host_test.go.tmpl" "$proof/build/native-host_test.go"
node --input-type=module - "$PWD" "$proof" <<'NODE'
import { writeFileSync } from 'node:fs';import { join } from 'node:path';
const [repo,proof]=process.argv.slice(2);
writeFileSync(join(proof,'build/overlay.json'),JSON.stringify({Replace:{[join(repo,'internal/virtualmachine/native_release_qualification_test.go')]:join(proof,'build/native-host_test.go')}}));
NODE
go build -trimpath -buildvcs=false -o "$proof/build/kejilion-agent" ./cmd/kejilion-agent
go test -c -trimpath -buildvcs=false -overlay "$proof/build/overlay.json" -o "$proof/build/native-test" ./internal/virtualmachine
sha256sum "$proof/build/kejilion-agent" "$proof/build/native-test" "$proof/build/native-host_test.go" > "$evidence/build.sha256"
sudo test ! -e "$install_dir"
sudo test ! -e "$state_dir"
sudo install -d -m 755 "$install_dir"
sudo install -m 755 "$proof/build/kejilion-agent" "$install_dir/kejilion-agent"
sudo install -m 755 "$proof/build/native-test" "$install_dir/native-test"
sudo install -d -m 700 "$state_dir"
docker pull "$image"
docker network create "$network" > "$evidence/docker-network-id.txt"
printf 'KPanel-native-%s\n' "$key" > "$proof/http/index.html"
docker run -d --name "$server" --network "$network" --read-only --cap-drop ALL --security-opt no-new-privileges --memory 64m --cpus .25 --pids-limit 32 -p 127.0.0.1::8080 -v "$proof/http:/web:ro" --entrypoint /bin/busybox "$image" httpd -f -p 8080 -h /web
port=$(docker port "$server" 8080/tcp | sed -n 's/^127\.0\.0\.1://p')
[[ "$port" =~ ^[0-9]{1,5}$ ]]
url="http://127.0.0.1:$port/"
curl -fsS --max-time 10 "$url" | grep -Fx "KPanel-native-$key"
docker run --rm --network "$network" --read-only --cap-drop ALL --security-opt no-new-privileges --memory 64m --cpus .25 --pids-limit 32 --entrypoint /bin/busybox "$image" wget -qO- "http://$server:8080/" | grep -Fx "KPanel-native-$key"
sudo iptables -S FORWARD | head -n 1 > "$evidence/forward-before.txt"
sudo iptables-save > "$evidence/iptables-before.txt"
docker network inspect "$network" bridge > "$evidence/docker-networks-before.json"
sudo env GITHUB_ACTIONS=true RUNNER_ENVIRONMENT=github-hosted GITHUB_REPOSITORY=kejilion/KPanel "KPANEL_NATIVE_KEY=$key" "KPANEL_NATIVE_NAME=$name" "KPANEL_NATIVE_STATE=$state_dir" "KPANEL_NATIVE_AGENT=$install_dir/kejilion-agent" "KPANEL_NATIVE_EVIDENCE=$evidence" "KPANEL_NATIVE_DOCKER_URL=$url" "KPANEL_NATIVE_SHA=$EXPECTED_CANDIDATE" "$install_dir/native-test" -test.v -test.run '^TestNativeReleaseQualification$' -test.timeout 45m
curl -fsS --max-time 10 "$url" | grep -Fx "KPanel-native-$key"
docker run --rm --network "$network" --read-only --cap-drop ALL --security-opt no-new-privileges --memory 64m --cpus .25 --pids-limit 32 --entrypoint /bin/busybox "$image" wget -qO- "http://$server:8080/" | grep -Fx "KPanel-native-$key"
sudo iptables -S FORWARD | head -n 1 > "$evidence/forward-after.txt"
diff -u "$evidence/forward-before.txt" "$evidence/forward-after.txt"
sudo iptables-save > "$evidence/iptables-after.txt"
sudo nft list ruleset > "$evidence/nft-after.txt"
grep -qi libvirt "$evidence/iptables-after.txt" "$evidence/nft-after.txt"
grep -q DOCKER "$evidence/iptables-after.txt"
docker network inspect "$network" bridge > "$evidence/docker-networks-after.json"
dpkg-query -W 'libvirt*' 'qemu*' 'dnsmasq*' 'cloud-image-utils' > "$evidence/packages.txt"
sudo virsh --connect qemu:///system net-dumpxml default > "$evidence/default-network.xml"
network_source=/usr/share/libvirt/networks/default.xml
if ! test -f "$network_source"; then network_source=/usr/share/libvirt/networks/default.xml.in; fi
test -f "$network_source"
dpkg-query -S "$network_source" > "$evidence/default-network-package.txt"
sha256sum "$network_source" > "$evidence/default-network-source.sha256"
apt-cache policy libvirt-daemon-system libvirt-daemon-config-network > "$evidence/libvirt-package-policy.txt"
git ls-files '*.go' | LC_ALL=C sort | while IFS= read -r file; do sha256sum "$file"; done > "$evidence/go-sources-after.sha256"
diff -u "$evidence/go-sources-before.sha256" "$evidence/go-sources-after.sha256"
test "$(git rev-parse HEAD)" = "$EXPECTED_CANDIDATE"
test -z "$(git status --porcelain)"
node --input-type=module - "$evidence" "$EXPECTED_CANDIDATE" "$GITHUB_SHA" <<'NODE'
import{readFileSync,writeFileSync}from'node:fs';import{join}from'node:path';
const [dir,candidate,controller]=process.argv.slice(2);const native=JSON.parse(readFileSync(join(dir,'native-result.json')));
if(native.status!=='passed'||native.candidate!==candidate)throw Error('Native evidence did not pass');
writeFileSync(join(dir,'receipt.json'),JSON.stringify({status:'passed',candidate,controller,environment:'github-hosted',native,DockerHostAndPeerBeforeAfter:true,forwardPolicyUnchanged:true,defaultNetworkPackageRecorded:true,originalGoSourcesUnchanged:true,productionConnected:false,finishedAt:new Date().toISOString()},null,2)+'\n');
NODE
