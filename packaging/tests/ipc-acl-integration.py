#!/usr/bin/env python3
"""Exercise real Linux ACL inheritance and the unprivileged Panel identity."""
import os
from pathlib import Path
import socket
import subprocess
import sys

repo = Path(sys.argv[1])
root = Path(os.environ["KPANEL_ACL_TEST_DIR"])
os.chmod(root, 0o755)


def run(*args):
    return subprocess.check_output(args, text=True)


def probe(directory, uid, groups):
    def identity():
        os.setgroups(groups)
        os.setgid(65532)
        os.setuid(uid)

    code = """
import pathlib, socket, sys
base = pathlib.Path(sys.argv[1])
if sys.argv[2] == 'token':
    assert (base / 'secrets/agent.token').read_text() == 'test-only-token\\n'
else:
    client = socket.socket(socket.AF_UNIX)
    client.connect(str(base / 'run/agent.sock'))
"""
    return [subprocess.run([sys.executable, "-c", code, str(directory), kind],
                           preexec_fn=identity, capture_output=True).returncode == 0
            for kind in ("token", "socket")]


for name, helper in (("deploy", repo / "deploy/ipc-permissions.sh"),
                     ("standalone", root / "app-helper")):
    parent = root / name
    parent.mkdir()
    os.chmod(parent, 0o755)
    # Reproduce group::--x despite chmod's readable/writable ACL mask, plus
    # inherited named-user entries that must not grant access to credentials.
    run("setfacl", "-m", "d:u::rwx,d:g::--x,d:o::--x,d:m::rwx,"
        "d:u:65532:---,d:u:65533:rwx", str(parent))
    unrelated = parent / "unrelated"
    unrelated.mkdir()
    before = run("getfacl", "-cp", str(parent), str(unrelated))
    target = parent / "private"
    target.mkdir()
    # Container bind mounts bypass this host parent. Permit traversal for the
    # host UID probes while retaining its default ACL for child creation.
    run("setfacl", "-b", str(target))
    os.chmod(target, 0o755)
    run("setfacl", "-m", "d:u::rwx,d:g::--x,d:o::--x,d:m::rwx,"
        "d:u:65532:---,d:u:65533:rwx", str(target))
    # NAS /home/docker symlinks remain supported; private IPC dirs are real.
    alias = root / (name + "-docker")
    alias.symlink_to(parent, target_is_directory=True)
    dirs = [target / "run", target / "secrets"]
    for path in dirs:
        path.mkdir()
        os.chown(path, 0, 987)
        os.chmod(path, 0o750)
    token = dirs[1] / "agent.token"
    token.write_text("test-only-token\n")
    os.chown(token, 0, 987)
    os.chmod(token, 0o640)
    socket_path = dirs[0] / "agent.sock"
    server = socket.socket(socket.AF_UNIX)
    server.bind(str(socket_path))
    server.listen(16)
    os.chown(socket_path, 0, 987)
    os.chmod(socket_path, 0o660)
    original_probe = probe(target, 65532, [987])
    assert original_probe == [False, False], f"ACL fixture did not reproduce outage: {original_probe}"
    token_inode = token.stat().st_ino
    command = ['bash', '-c', '. "$1"; kpanel_prepare_ipc_permissions 987 "$2/run" "$2/secrets"',
               'test', str(helper), str(alias / "private")]
    subprocess.run(command, check=True)
    assert token.stat().st_ino == token_inode, "repair replaced bind-mounted token"
    assert probe(target, 65532, [987]) == [True, True]
    assert probe(target, 65533, []) == [False, False], "unrelated user gained access"
    for path in (*dirs, token, socket_path):
        acl = run("getfacl", "-cp", str(path))
        assert "default:" not in acl and "mask:" not in acl, acl
    subprocess.run(command, check=True)  # Existing-install repair is idempotent.
    assert run("getfacl", "-cp", str(parent), str(unrelated)) == before

    # Match Agent restart and atomic credential recreation; the clean defaults
    # must preserve access without running another permission repair.
    server.close()
    socket_path.unlink()
    server = socket.socket(socket.AF_UNIX)
    server.bind(str(socket_path))
    server.listen(16)
    os.chown(socket_path, 0, 987)
    os.chmod(socket_path, 0o660)
    temporary = dirs[1] / "new-token"
    temporary.write_text("test-only-token\n")
    os.chown(temporary, 0, 987)
    os.chmod(temporary, 0o640)
    temporary.replace(token)
    assert probe(target, 65532, [987]) == [True, True]
    assert probe(target, 65533, []) == [False, False]
    assert run("getfacl", "-cp", str(parent), str(unrelated)) == before
    server.close()
    print(f"ipc_real_acl={name}:pass (denied before, connected after, restart, token replacement, isolation)")
