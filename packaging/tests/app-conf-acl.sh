#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR=${1:-/src}
TEST_DIR=$(mktemp -d /tmp/kpanel-ipc-acl.XXXXXX)
trap 'rm -rf -- "$TEST_DIR"' EXIT

# The app configuration is standalone on hosts. Keep its embedded helper byte
# identical to the one used by the source installer.
sed -n '/^# BEGIN KPanel IPC permissions/,/^# END KPanel IPC permissions/p' \
  "$PROJECT_DIR/packaging/kejilion-app/kpanel.conf" >"$TEST_DIR/app-helper"
sed -n '/^# BEGIN KPanel IPC permissions/,/^# END KPanel IPC permissions/p' \
  "$PROJECT_DIR/deploy/ipc-permissions.sh" >"$TEST_DIR/deploy-helper"
test -s "$TEST_DIR/app-helper"
cmp "$TEST_DIR/app-helper" "$TEST_DIR/deploy-helper"
# shellcheck source=/dev/null
. "$PROJECT_DIR/deploy/ipc-permissions.sh"

mkdir "$TEST_DIR/run" "$TEST_DIR/secrets" "$TEST_DIR/unrelated"
printf 'test-only-token\n' >"$TEST_DIR/secrets/agent.token"
cp "$TEST_DIR/secrets/agent.token" "$TEST_DIR/original-token"
kpanel_prepare_ipc_permissions 987 "$TEST_DIR/run" "$TEST_DIR/secrets"
test "$(stat -c '%u:%g:%a' "$TEST_DIR/run")" = 0:987:750
test "$(stat -c '%u:%g:%a' "$TEST_DIR/secrets/agent.token")" = 0:987:640
kpanel_prepare_ipc_permissions 987 "$TEST_DIR/run" "$TEST_DIR/secrets"
cmp "$TEST_DIR/original-token" "$TEST_DIR/secrets/agent.token"

# Validate all objects before mutation; never follow a link or touch hard-linked
# credentials, even when the caller is root.
chmod 700 "$TEST_DIR/run"
mv "$TEST_DIR/secrets/agent.token" "$TEST_DIR/saved-token"
ln -s "$TEST_DIR/saved-token" "$TEST_DIR/secrets/agent.token"
if kpanel_prepare_ipc_permissions 987 "$TEST_DIR/run" "$TEST_DIR/secrets"; then exit 1; fi
test "$(stat -c %a "$TEST_DIR/run")" = 700
rm "$TEST_DIR/secrets/agent.token"
ln "$TEST_DIR/saved-token" "$TEST_DIR/secrets/agent.token"
if kpanel_prepare_ipc_permissions 987 "$TEST_DIR/run" "$TEST_DIR/secrets"; then exit 1; fi
rm "$TEST_DIR/secrets/agent.token"
mv "$TEST_DIR/saved-token" "$TEST_DIR/secrets/agent.token"
ln -s "$TEST_DIR/run" "$TEST_DIR/linked-run"
if kpanel_prepare_ipc_permissions 987 "$TEST_DIR/linked-run" "$TEST_DIR/secrets"; then exit 1; fi

# Test the ordinary host without ACL tools, and a detected ACL without setfacl.
mkdir "$TEST_DIR/bin"
for tool in stat ls awk chown chmod; do ln -s "$(command -v "$tool")" "$TEST_DIR/bin/$tool"; done
PATH="$TEST_DIR/bin" kpanel_prepare_ipc_permissions 987 "$TEST_DIR/run" "$TEST_DIR/secrets"
rm "$TEST_DIR/bin/ls"
printf '#!/bin/sh\nprintf "drwxr-x---+ root root directory\\n"\n' >"$TEST_DIR/bin/ls"
chmod 755 "$TEST_DIR/bin/ls"
if PATH="$TEST_DIR/bin" kpanel_prepare_ipc_permissions 987 "$TEST_DIR/run" "$TEST_DIR/secrets" \
  >"$TEST_DIR/missing-acl.out" 2>&1; then exit 1; fi
grep -F 'install the acl package' "$TEST_DIR/missing-acl.out"
printf '#!/bin/sh\nexit 1\n' >"$TEST_DIR/bin/setfacl"
chmod 755 "$TEST_DIR/bin/setfacl"
if PATH="$TEST_DIR/bin:$PATH" kpanel_prepare_ipc_permissions 987 "$TEST_DIR/run" "$TEST_DIR/secrets"; then exit 1; fi

# A root Agent probe and a healthy Panel alone must not allow install/update
# success if the actual container user cannot connect to the Agent.
check_container_probe() (
  docker_app_plus() { :; }
  # shellcheck source=/dev/null
  . "$PROJECT_DIR/packaging/kejilion-app/kpanel.conf"
  mkdir -p "$TEST_DIR/health/bin"
  printf '#!/bin/sh\nexit 0\n' >"$TEST_DIR/health/bin/kejilion-agent"
  chmod 755 "$TEST_DIR/health/bin/kejilion-agent"
  # Only relocate the readiness helper, leaving production paths untouched.
  eval "$(declare -f kpanel_wait_ready | sed "s@/home/docker/kpanel@$TEST_DIR/health@g")"
  docker() {
    case "$1" in
      inspect) echo healthy ;;
      exec) test "${probe_fails}" = false ;;
      *) return 1 ;;
    esac
  }
  sleep() { :; }
  probe_fails=true
  if kpanel_wait_ready; then exit 1; fi
  probe_fails=false
  kpanel_wait_ready
)
check_container_probe

if command -v setfacl >/dev/null && command -v getfacl >/dev/null && command -v python3 >/dev/null; then
  export KPANEL_ACL_TEST_DIR=$TEST_DIR
  python3 "$PROJECT_DIR/packaging/tests/ipc-acl-integration.py" "$PROJECT_DIR"
elif [ "${KPANEL_REQUIRE_REAL_ACL:-0}" = 1 ]; then
  echo 'real ACL tests require acl and python3' >&2
  exit 1
else
  echo 'ipc_real_acl=skipped (install acl and python3; use KPANEL_REQUIRE_REAL_ACL=1 to require)'
fi
echo 'app_conf_ipc_permissions=pass'
