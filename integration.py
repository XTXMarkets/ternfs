#!/usr/bin/env -S python3 -u

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

import argparse
import socket
import os
import subprocess

from common import *

parser = argparse.ArgumentParser()
parser.add_argument('--short', action='store_true')
parser.add_argument('--docker', action='store_true')
parser.add_argument('--leader-only', action='store_true', help='Run only LogsDB leader with LEADER_NO_FOLLOWERS')
args = parser.parse_args()

script_dir = os.path.dirname(os.path.realpath(__file__))
os.chdir(script_dir)

if args.short:
    fuse_tests = 'history|direct|mounted|cp|parallel writes'
else:
    # The mounted ones are long, and we don't rely on the FUSE driver critically,
    # but we still want to test it in the short run.
    fuse_tests = 'history|direct|cp'

if args.docker:
    build_sanitized = 'build/ubuntusanitized'
    build_release = 'build/ubuntu'
    build_valgrind = 'build/ubuntuvalgrind'
    # setup right user -- we need it with a name because of fusermount
    gid = int(os.environ['GID'])
    uid = int(os.environ['UID'])
    subprocess.run(['groupadd', '-g', str(gid), 'restech'])
    subprocess.run(['useradd', '-u', str(uid), '-g', 'restech', 'restechprod'])
    os.setgid(gid)
    os.setuid(uid)
else:
    build_sanitized = 'build/sanitized'
    build_release = 'build/release'
    build_valgrind = 'build/valgrind'

bold_print('integration tests')
short = ['-short'] if args.short else []
leader_only = ['-leader-only'] if args.leader_only else []
# -block-service-killer does not work with FUSE driver (the duplicated FDs
# of the child processes confuse the FUSE driver).
tests = [
    ['./go/terntests/terntests', '-binaries-dir', build_sanitized, '-verbose', '-repo-dir', '.', '-tmp-dir', '.', '-filter', fuse_tests, '-outgoing-packet-drop', '0.02'] + short + leader_only,
    ['./go/terntests/terntests', '-binaries-dir', build_release, '-preserve-data-dir', '-verbose', '-block-service-killer', '-filter', 'direct', '-repo-dir', '.', '-tmp-dir', '.'] + short + leader_only,
]
if not args.short:
    # valgrind is super slow, it still surfaced bugs in the past but run the short
    # versions only in the long tests.
    tests.append(['./go/terntests/terntests', '-binaries-dir', build_valgrind, '-verbose', '-repo-dir', '.', '-tmp-dir', '.', '-short', '-filter', fuse_tests] + leader_only)
# we need three free ports, we get them here upfront rather than in registry to reduce
# the chance of races -- if we got it from the integration tests it'll be while
# tons of things are started in another integration test
ports = []
for _ in range(len(tests)):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(('', 0))
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)  # Ensure the port is immediately reusable
        ports.append(s.getsockname()[1])
wait_cmds(
    [run_cmd(test + ['-registry-port', str(port)]) for test, port in zip(tests, ports)],
)
