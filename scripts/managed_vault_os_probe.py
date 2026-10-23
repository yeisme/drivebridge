#!/usr/bin/env python3
"""Probe Linux DAC boundaries in a disposable, offline container only.

Run through scripts/check.py for standard evidence. This is design evidence,
not a managed-vault implementation or a host permission migration.
"""
import argparse
import json
import re
import subprocess
import tempfile
from pathlib import Path


PROBE = r'''
import errno
import json
import os

WRITER, BROKER = 2001, 2002
results = {}

def as_user(uid, operation):
    pid = os.fork()
    if pid == 0:
        try:
            os.setgroups([])
            os.setgid(uid)
            os.setuid(uid)
            operation()
        except PermissionError as exc:
            os._exit(10 if exc.errno in (errno.EACCES, errno.EPERM) else 11)
        except Exception:
            os._exit(12)
        os._exit(0)
    _, status = os.waitpid(pid, 0)
    return os.waitstatus_to_exitcode(status)

def vault(path, uid):
    os.mkdir(path, 0o700)
    with open(path + '/note.md', 'w') as stream:
        stream.write('disposable fixture\n')
    os.chmod(path + '/note.md', 0o600)
    os.chown(path + '/note.md', uid, uid)
    os.chown(path, uid, uid)

def append(path):
    with open(path, 'a') as stream:
        stream.write('fixture mutation\n')

def verify(name, actual, expected):
    if actual != expected:
        raise RuntimeError(name + ': unexpected probe result ' + str(actual))
    results[name] = True

# 0700/0600 still permits an independent process with the same UID.
vault('/proof/same-user', WRITER)
verify('same_uid_writer_can_write', as_user(WRITER, lambda: append('/proof/same-user/note.md')), 0)

# Ownership changes deny new opens, but do not revoke an existing descriptor.
vault('/proof/cutover', WRITER)
ready_r, ready_w = os.pipe()
resume_r, resume_w = os.pipe()
pid = os.fork()
if pid == 0:
    try:
        os.close(ready_r)
        os.close(resume_w)
        os.setgroups([])
        os.setgid(WRITER)
        os.setuid(WRITER)
        fd = os.open('/proof/cutover/note.md', os.O_WRONLY | os.O_APPEND)
        os.write(ready_w, b'1')
        os.close(ready_w)
        if os.read(resume_r, 1) != b'1':
            os._exit(13)
        written = os.write(fd, b'held-descriptor\n')
        os.close(fd)
        os._exit(0 if written == len(b'held-descriptor\n') else 14)
    except Exception:
        os._exit(15)
os.close(ready_w)
os.close(resume_r)
verify('descriptor_opened_before_cutover', os.read(ready_r, 1), b'1')
os.close(ready_r)
# The privileged migrator takes the directory before traversing it. The old
# writer's already-open descriptor survives this transition too.
os.chown('/proof/cutover', 0, 0)
os.chown('/proof/cutover/note.md', BROKER, BROKER)
os.chown('/proof/cutover', BROKER, BROKER)
os.write(resume_w, b'1')
os.close(resume_w)
_, status = os.waitpid(pid, 0)
verify('retained_descriptor_still_writes', os.waitstatus_to_exitcode(status), 0)
verify('cutover_denies_new_open', as_user(WRITER, lambda: append('/proof/cutover/note.md')), 10)

# Protecting the leaf does not prevent rename through a writable parent.
os.mkdir('/proof/user-parent', 0o700)
vault('/proof/user-parent/vault', BROKER)
os.chown('/proof/user-parent', WRITER, WRITER)
verify('writable_parent_can_rename_vault', as_user(WRITER, lambda: os.rename('/proof/user-parent/vault', '/proof/user-parent/moved')), 0)

# A new protected root under a protected parent rejects new untrusted opens
# and aliases; the designated owner can still write the same original file.
vault('/proof/protected', BROKER)
os.symlink('/proof/protected', '/proof/alias')
verify('separate_uid_direct_write_denied', as_user(WRITER, lambda: append('/proof/protected/note.md')), 10)
verify('separate_uid_alias_write_denied', as_user(WRITER, lambda: append('/proof/alias/note.md')), 10)
verify('protected_parent_rename_denied', as_user(WRITER, lambda: os.rename('/proof/protected', '/proof/moved')), 10)
verify('broker_uid_can_write_original', as_user(BROKER, lambda: append('/proof/protected/note.md')), 0)
# An ordinary external editor has the same access as the legacy writer.
verify('ordinary_editor_also_denied', as_user(WRITER, lambda: append('/proof/protected/note.md')), 10)
print(json.dumps({'platform': 'linux-container', 'checks': results,
                  'host_permissions_changed': False, 'runtime_guard_ready': False,
                  'pinax_electron_integration': False}, sort_keys=True))
'''


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--image', default='python:3.12-slim-bookworm', help='Already cached Linux Python image; never pulled')
    args = parser.parse_args()
    endpoint = subprocess.check_output(['docker', 'context', 'inspect', '--format', '{{.Endpoints.docker.Host}}'], text=True).strip()
    if not endpoint.startswith('unix:///'):
        parser.error('A local Unix-socket Docker engine is required')
    docker = ['docker', '--host', endpoint]
    image = subprocess.check_output([*docker, 'image', 'inspect', args.image, '--format', '{{.Id}}'], text=True).strip()
    if not re.fullmatch(r'sha256:[a-f0-9]{64}', image):
        parser.error('A cached image digest is required')
    command = [*docker, 'run', '--rm', '--pull=never', '--network=none', '--read-only',
               '--tmpfs', '/proof:rw,noexec,nosuid,size=16m,mode=0755',
               '--cap-drop=ALL', '--cap-add=CHOWN', '--cap-add=SETUID', '--cap-add=SETGID',
               '--security-opt=no-new-privileges', '--pids-limit=32', '--memory=128m',
               '--user=0:0', '-i', '--entrypoint=python3', image, '-']
    print(json.dumps({'image_digest': image, 'host_mounts': False, 'network': 'none'}), flush=True)
    with tempfile.TemporaryDirectory(prefix='drivebridge-guard-probe-') as temporary:
        cidfile = Path(temporary) / 'container-id'
        command[command.index('--rm'):command.index('--rm')] = ['--cidfile', str(cidfile)]
        try:
            subprocess.run(command, input=PROBE, text=True, check=True, timeout=60)
        finally:
            container_id = cidfile.read_text().strip() if cidfile.exists() else ''
            if re.fullmatch(r'[a-f0-9]{64}', container_id):
                # Only this invocation's disposable container; --rm normally
                # already removed it. Also clean up after a client timeout.
                subprocess.run([*docker, 'rm', '-f', container_id], capture_output=True, timeout=10, check=False)


if __name__ == '__main__':
    main()
