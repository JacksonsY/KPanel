#!/bin/sh

# BEGIN KPanel IPC permissions (also embedded in the standalone app config).
kpanel_prepare_ipc_permissions() (
    # Only callers' private IPC directories are touched, never their parents or
    # descendants. Validate the complete set before changing any permissions.
    ipc_gid=$1
    ipc_run=$2
    ipc_secrets=$3
    case "$ipc_gid" in ''|*[!0-9]*) return 1 ;; esac
    for ipc_path in "$ipc_run" "$ipc_secrets"; do
        [ ! -L "$ipc_path" ] && [ -d "$ipc_path" ] &&
            [ "$(stat -c %u -- "$ipc_path")" = 0 ] || {
            echo "KPanel IPC directory must be a root-owned real directory: $ipc_path" >&2
            return 1
        }
    done
    set -- "$ipc_run" "$ipc_secrets"
    for ipc_path in "$ipc_run/agent.sock" "$ipc_secrets/agent.token"; do
        [ ! -L "$ipc_path" ] || return 1
        if [ -e "$ipc_path" ]; then
            case "$ipc_path" in
                "$ipc_run/agent.sock") [ -S "$ipc_path" ] || return 1 ;;
                *) [ -f "$ipc_path" ] || return 1 ;;
            esac
            [ "$(stat -c '%u:%h' -- "$ipc_path")" = 0:1 ] || return 1
            set -- "$@" "$ipc_path"
        fi
    done
    if command -v setfacl >/dev/null 2>&1; then
        # chmod alone updates the ACL mask, not the owning group's ACL entry.
        # Remove default ACLs too, so recreated sockets/tokens stay accessible.
        setfacl -b -- "$@" && setfacl -k -- "$ipc_run" "$ipc_secrets" || {
            echo 'KPanel could not normalize private IPC ACLs; installation/update stopped.' >&2
            return 1
        }
    else
        ipc_modes=$(LC_ALL=C ls -ld -- "$@") || return 1
        if printf '%s\n' "$ipc_modes" | awk '$1 ~ /\+/ {found=1} END {exit !found}'; then
            echo 'KPanel IPC has POSIX ACLs; install the acl package (setfacl), then retry.' >&2
            return 1
        fi
    fi
    chown root:"$ipc_gid" -- "$@" && chmod 750 -- "$ipc_run" "$ipc_secrets" || return 1
    [ ! -f "$ipc_secrets/agent.token" ] || chmod 640 -- "$ipc_secrets/agent.token" || return 1
    [ ! -S "$ipc_run/agent.sock" ] || chmod 660 -- "$ipc_run/agent.sock" || return 1
)
# END KPanel IPC permissions.
