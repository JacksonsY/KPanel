#!/usr/bin/env bash
set -Eeuo pipefail
[[ ${GITHUB_ACTIONS:-} == true && ${RUNNER_ENVIRONMENT:-} == github-hosted && -n ${RUNNER_TEMP:-} ]]
repo=$(git rev-parse --show-toplevel)
evidence="$repo/internal/browsercomponent/testdata"
network=kejilion-panel-browser-control
if docker container inspect kpanel-browser >/dev/null 2>&1 || docker network inspect "$network" >/dev/null 2>&1; then
  echo 'preflight failed: fixed browser resources already exist' >&2; exit 1
fi
runtime=$(mktemp -d "$RUNNER_TEMP/kpanel-component-test.XXXXXXXX")
mkdir -m 0700 "$runtime/state"
umask 0077
label="hosted-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT"
cleanup() {
  status=$?
  trap - EXIT
  actual=$(docker network inspect --format '{{index .Labels "kpanel.test"}}' "$network" 2>/dev/null || true)
  if [[ "$actual" == "$label" ]]; then
    if docker container inspect kpanel-browser >/dev/null 2>&1; then
      attached=$(docker inspect --format '{{with index .NetworkSettings.Networks "kejilion-panel-browser-control"}}{{.NetworkID}}{{end}}' kpanel-browser)
      owned=$(docker network inspect --format '{{.Id}}' "$network")
      if [[ "$attached" == "$owned" ]]; then docker rm -f kpanel-browser >/dev/null || status=1; else status=1; fi
    fi
    docker network rm "$network" >/dev/null || status=1
  fi
  case "$runtime" in "$RUNNER_TEMP"/kpanel-component-test.*) rm -rf -- "$runtime" ;; *) status=1 ;; esac
  exit "$status"
}
trap cleanup EXIT
docker network create --internal --label "kpanel.test=$label" "$network" >/dev/null
cd "$repo"
go build -o "$runtime/agent" ./cmd/kejilion-agent
CGO_ENABLED=0 go build -o "$runtime/probe" "$evidence/live-browser-probe.go"
chmod 0555 "$runtime/probe"
for action in install stop start restart uninstall install uninstall; do
  id=$(node "$evidence/live-component-task.mjs" "$action" "$runtime/state")
  "$runtime/agent" browser-component-run --state-dir "$runtime/state" --runtime-dir "$runtime/runtime" --docker-socket /var/run/docker.sock --job "$id"
  node "$evidence/live-component-task.mjs" result "$runtime/state"
  if [[ "$action" == install ]]; then
    docker inspect --format 'limits memory={{.HostConfig.Memory}} pids={{.HostConfig.PidsLimit}} readonly={{.HostConfig.ReadonlyRootfs}} ports={{json .HostConfig.PortBindings}}' kpanel-browser
    [[ $(docker inspect --format '{{.HostConfig.Memory}}' kpanel-browser) == 1610612736 ]]
    [[ $(docker inspect --format '{{.HostConfig.PidsLimit}}' kpanel-browser) == 512 ]]
    [[ $(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' kpanel-browser) == true ]]
    [[ $(docker inspect --format '{{json .HostConfig.PortBindings}}' kpanel-browser) == '{}' ]]
    docker run --rm --network "$network" --user 65532:65532 --read-only --cap-drop ALL --security-opt no-new-privileges \
      --mount "type=bind,src=$runtime/probe,dst=/test/probe,readonly" \
      --mount "type=bind,src=$runtime/runtime,dst=/run/secrets,readonly" \
      --entrypoint /test/probe docker.io/kjlion/kpanel-browser@sha256:fbc00e5b7e5bb39c117507b1995cc0e4f85976eb6818db23322d46968f7d5697 browser /run/secrets/browser-token
  fi
done
echo 'component_lifecycle=pass actions=7 navigation=pass cleanup=pending'
