#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mode="${1:-source}"
trivy_image="${TRIVY_IMAGE:-aquasec/trivy@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969}"
cache_dir="${TRIVY_CACHE_DIR:-${TMPDIR:-/tmp}/kpanel-trivy-cache}"

command -v docker >/dev/null 2>&1 || {
  echo "Docker is required for the pinned Trivy security scan." >&2
  exit 1
}
mkdir -p "$cache_dir"

proxy_args=()
network_args=()
for proxy_name in HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy; do
  if [[ -n "${!proxy_name:-}" ]]; then
    proxy_args+=(--env "$proxy_name")
  fi
done
if [[ -n "${HTTP_PROXY:-}${HTTPS_PROXY:-}${http_proxy:-}${https_proxy:-}" ]]; then
  # Trivy runs as a nested container. Host networking is required for the
  # loopback proxy endpoints commonly supplied by WSL, and --env NAME keeps
  # the values out of command lines and persisted evidence.
  network_args+=(--network host)
fi

case "$mode" in
  source)
    docker run --rm \
      "${network_args[@]}" \
      "${proxy_args[@]}" \
      -v "$repo_root:/src:ro" \
      -v "$cache_dir:/root/.cache/trivy" \
      "$trivy_image" \
      fs \
      --scanners vuln,secret,misconfig \
      --severity HIGH,CRITICAL \
      --exit-code 1 \
      --ignorefile /src/.trivyignore.yaml \
      --skip-files /src/go.sum \
      /src
    # A local Go replacement has no upstream version in scanner databases.
    # Keep the actual scan above and require conservative original-version coverage too.
    # The release Runner and host Docker daemon share this checkout at the same path.
    # Runner-private /tmp cannot be used as a nested Docker bind source.
    baseline_root="$repo_root/.codex-tmp"
    if [[ -L "$baseline_root" || ( -e "$baseline_root" && ! -d "$baseline_root" ) ]]; then
      echo 'Refusing an unsafe baseline staging directory.' >&2
      exit 1
    fi
    [[ -d "$baseline_root" ]] || mkdir -m 700 -- "$baseline_root"
    baseline_dir="$(mktemp -d "$baseline_root/torrent-baseline.XXXXXXXX")"
    trap 'rm -rf -- "$baseline_dir"' EXIT
    node "$repo_root/scripts/torrent-backport-baseline.mjs" manifest "$baseline_dir"
    docker run --rm \
      "${network_args[@]}" \
      "${proxy_args[@]}" \
      -v "$baseline_dir:/baseline:ro" \
      -v "$cache_dir:/root/.cache/trivy" \
      "$trivy_image" \
      fs \
      --scanners vuln \
      --severity HIGH,CRITICAL \
      --exit-code 1 \
      --format json \
      --list-all-pkgs \
      --skip-files /baseline/go.sum \
      /baseline | tee "$baseline_dir/trivy.json"
    node "$repo_root/scripts/torrent-backport-baseline.mjs" check-trivy "$baseline_dir/trivy.json"
    ;;
  image)
    image_ref="${2:-}"
    [[ -n "$image_ref" ]] || {
      echo "Usage: $0 image IMAGE_REFERENCE" >&2
      exit 2
    }
    docker image inspect "$image_ref" >/dev/null
    docker run --rm \
      "${network_args[@]}" \
      "${proxy_args[@]}" \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -v "$cache_dir:/root/.cache/trivy" \
      "$trivy_image" \
      image \
      --scanners vuln,secret \
      --severity HIGH,CRITICAL \
      --exit-code 1 \
      "$image_ref"
    ;;
  *)
    echo "Unknown scan mode: $mode (expected source or image)" >&2
    exit 2
    ;;
esac
