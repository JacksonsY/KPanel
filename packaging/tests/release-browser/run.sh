#!/usr/bin/env bash
set -Eeuo pipefail
test "${GITHUB_ACTIONS:-}" = true && test "${RUNNER_ENVIRONMENT:-}" = github-hosted
: "${RUNNER_TEMP:?}" "${EXPECTED_VERSION:?}" "${EXPECTED_SHA:?}" "${IMAGE_DIGEST:?}"
repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
collect_permissions() {
  for path in "$RUNNER_TEMP"/kpanel-{real,browser}-*/results "$PUBLIC_EVIDENCE"; do
    test ! -d "$path" || sudo chown -R "$(id -u):$(id -g)" "$path"
  done
}
trap collect_permissions EXIT
export RELEASE_TEST_RUNNER="kpanel-release-test:${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
export RELEASE_BROWSER_RUNTIME="$RUNNER_TEMP/kpanel-browser-runtime"
mkdir "$RELEASE_BROWSER_RUNTIME"
# Full package version and matching browser image are fixed together. The registry
# integrity in package-lock.json is retained in the evidence, with no global install.
cp "$repo/packaging/release-browser/package"*.json "$RELEASE_BROWSER_RUNTIME/"
npm ci --prefix "$RELEASE_BROWSER_RUNTIME" --ignore-scripts --no-audit --no-fund
npm audit --prefix "$RELEASE_BROWSER_RUNTIME" --audit-level=high
chmod -R a+rX "$RELEASE_BROWSER_RUNTIME"
docker build -f "$repo/packaging/release-runner/Dockerfile" -t "$RELEASE_TEST_RUNNER" "$repo"
docker image inspect "$RELEASE_TEST_RUNNER" > "$PUBLIC_EVIDENCE/test-runner.json"
export PREVIOUS_PREVIEW_VERSION=1.27.0-rc.2
export PREVIOUS_PREVIEW_IMAGE=docker.io/kjlion/kejilion-panel@sha256:207aa4614c703914df69dcd2e1a392ed55f4864ce31d971bd910e02ee667f659
image="docker.io/kjlion/kejilion-panel@$IMAGE_DIGEST"
sudo -E timeout --kill-after=30s 15m bash "$repo/packaging/tests/release-browser/real-feature-public.sh" "$image" public "$RUNNER_TEMP/kpanel-real-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT" "$IMAGE_DIGEST"
for mode in published upgrade; do
  sudo -E timeout --kill-after=30s 15m bash "$repo/packaging/tests/release-browser/public-browser-complete.sh" "$image" "$mode" "$RUNNER_TEMP/kpanel-browser-$mode-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT" "$IMAGE_DIGEST"
done
printf 'Browser and real-feature validation passed for %s at %s\n' "$EXPECTED_VERSION" "$EXPECTED_SHA" >> "$GITHUB_STEP_SUMMARY"
