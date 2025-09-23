#!/usr/bin/env python3

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

import os
import argparse

from common import *

parser = argparse.ArgumentParser()
parser.add_argument('--functional', action='store_true')
parser.add_argument('--integration', action='store_true')
parser.add_argument('--short', action='store_true')
parser.add_argument('--kmod', action='store_true')
parser.add_argument('--build', action='store_true')
parser.add_argument('--docker', action='store_true', help='Build and run in docker image')
parser.add_argument('--prepare-image', default=None, type=str, help='Build the kmod image given the provided base image')
parser.add_argument('--leader-only', action='store_true', help='Run only LogsDB leader with LEADER_NO_FOLLOWERS')
parser.add_argument('--close-tracker-object', default=None, type=str, help='Run fuse driver with the given close tracker object')
args = parser.parse_args()

script_dir = os.path.dirname(os.path.realpath(__file__))
os.chdir(script_dir)

def run_docker_unbuffered(docker_args, args):
    # See <https://groups.google.com/g/seastar-dev/c/r7W-Kqzy9O4>
    # for motivation for `--security-opt seccomp=unconfined`,
    # the `--pids-limit -1` is not something I hit but it seems
    # like a good idea.
    container = 'ghcr.io/xtxmarkets/ternfs-ubuntu-build:2025-09-18'
    run_cmd_unbuffered(
        ['docker', 'run', '--pids-limit', '-1', '--security-opt', 'seccomp=unconfined', '--mount', f'type=bind,src={script_dir},dst=/ternfs', '--cap-add', 'SYS_ADMIN', '--privileged', '--rm', '-i', '-e', f'UID={os.getuid()}', '-e', f'GID={os.getgid()}'] + docker_args + [container] + args
    )

if args.build:
    bold_print('building')
    for r in (['ubuntu', 'ubuntusanitized', 'ubuntuvalgrind'] if args.docker else ['release', 'sanitized', 'valgrind']):
        wait_cmd(run_cmd(['./build.sh', r]))
    if args.docker:
        run_docker_unbuffered(['-w', '/ternfs/kmod'], ['make', 'bincode_tests'])
    else:
        wait_cmd(run_cmd(['make', 'bincode_tests'], cwd='kmod'))

if args.functional:
    bold_print('functional tests')
    if args.docker:
        bold_print('starting functional tests in docker')
        container = 'ghcr.io/xtxmarkets/ternfs-ubuntu-build:2025-09-18'
        run_docker_unbuffered(
            ['-w', '/ternfs'], ['./cpp/tests.sh']
        )
        run_docker_unbuffered(
            ['-w', '/ternfs/kmod'], ['./bincode_tests']
        )
        run_docker_unbuffered(
            ['-w', '/ternfs/go'], ['go', 'test', './...']
        )
    else:
        wait_cmds([
            run_cmd(['./bincode_tests'], cwd='kmod'),
            run_cmd(['./cpp/tests.sh']),
            run_cmd(['go', 'test', './...'], cwd='go')
        ])

if args.integration:
    integration_args = (['--short'] if args.short else []) + (['--leader-only'] if args.leader_only else []) + (['--close-tracker-object', args.close_tracker_object] if args.close_tracker_object else [])
    if args.docker:
        bold_print('starting integration tests in docker')
        run_docker_unbuffered(
            [],
            ['/ternfs/integration.py', '--docker'] + integration_args
        )
    else:
        run_cmd_unbuffered(
            ['./integration.py'] + integration_args
        )

if args.prepare_image:
    bold_print('prepare kmod image')
    wait_cmd(run_cmd(['./kmod/ci_prepare.sh', args.prepare_image]))

if args.kmod:
    bold_print('kmod tests')
    wait_cmd(run_cmd(['./kmod/ci.sh'] + (['-short'] if args.short else []) + (['-leader-only'] if args.leader_only else [])))
