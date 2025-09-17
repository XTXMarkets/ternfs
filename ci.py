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
args = parser.parse_args()


os.environ['PATH'] = f'/opt/go1.18.4/bin:{os.environ["PATH"]}'

script_dir = os.path.dirname(os.path.realpath(__file__))
os.chdir(script_dir)

if args.build:
    bold_print('building')
    for r in (['ubuntu', 'ubuntusanitized', 'ubuntuvalgrind'] if args.docker else ['release', 'sanitized', 'valgrind']):
        wait_cmd(run_cmd(['./build.sh', r]))
    wait_cmd(run_cmd(['make', 'bincode_tests'], cwd='kmod'))

if args.functional:
    bold_print('functional tests')
    if args.docker:
        bold_print('starting functional tests in docker')
        container = 'ghcr.io/xtxmarkets/ternfs-ubuntu-build:2025-09-03'
        # See <https://groups.google.com/g/seastar-dev/c/r7W-Kqzy9O4>
        # for motivation for `--security-opt seccomp=unconfined`,
        # the `--pids-limit -1` is not something I hit but it seems
        # like a good idea.
        run_cmd_unbuffered(
            ['docker', 'run', '--pids-limit', '-1', '--security-opt', 'seccomp=unconfined', '--cap-add', 'SYS_ADMIN', '--privileged', '--rm', '-i', '--mount', f'type=bind,src={script_dir},dst={script_dir}', '-w', f'{script_dir}', '-e', f'UID={os.getuid()}', '-e', f'GID={os.getgid()}', container, './cpp/tests.sh']
        )
        run_cmd_unbuffered(
            ['docker', 'run', '--pids-limit', '-1', '--security-opt', 'seccomp=unconfined', '--cap-add', 'SYS_ADMIN', '--privileged', '--rm', '-i', '--mount', f'type=bind,src={script_dir},dst={script_dir}', '-w', f'{script_dir}/go', '-e', f'UID={os.getuid()}', '-e', f'GID={os.getgid()}', container, 'go', 'test', './...']
        )
        #run_cmd_unbuffered(
        #    ['docker', 'run', '--pids-limit', '-1', '--security-opt', 'seccomp=unconfined', '--cap-add', 'SYS_ADMIN', '-v', '/dev/fuse:/dev/fuse', '--privileged', '--rm', '-i', '--mount', f'type=bind,src={script_dir},dst={script_dir}', '-w', f'{script_dir}/kmod', '-e', f'UID={os.getuid()}', '-e', f'GID={os.getgid()}', container, './bincode_tests']
        #)
    # ToDo bincode_tests don't work in container mising libasan.so
    wait_cmds(
        [run_cmd(['./bincode_tests'], cwd='kmod')] + ([] if args.docker else [run_cmd(['./cpp/tests.sh']), run_cmd(['go', 'test', './...'], cwd='go')]),
    )

if args.integration:
    if args.docker:
        bold_print('starting integration tests in docker')
        container = 'ghcr.io/xtxmarkets/ternfs-ubuntu-build:2025-09-03'
        # See <https://groups.google.com/g/seastar-dev/c/r7W-Kqzy9O4>
        # for motivation for `--security-opt seccomp=unconfined`,
        # the `--pids-limit -1` is not something I hit but it seems
        # like a good idea.
        run_cmd_unbuffered(
            ['docker', 'run', '--pids-limit', '-1', '--security-opt', 'seccomp=unconfined', '--cap-add', 'SYS_ADMIN', '-v', '/dev/fuse:/dev/fuse', '--privileged', '--rm', '-i', '--mount', f'type=bind,src={script_dir},dst=/eggsfs', '-e', f'UID={os.getuid()}', '-e', f'GID={os.getgid()}', container, '/eggsfs/integration.py', '--docker'] + (['--short'] if args.short else []) + (['--leader-only'] if args.leader_only else [])
        )
    else:
        run_cmd_unbuffered(
            ['./integration.py'] + (['--short'] if args.short else []) + (['--leader-only'] if args.leader_only else [])
        )

if args.prepare_image:
    bold_print('prepare kmod image')
    wait_cmd(run_cmd(['./kmod/ci_prepare.sh', args.prepare_image]))

if args.kmod:
    bold_print('kmod tests')
    wait_cmd(run_cmd(['./kmod/ci.sh'] + (['-short'] if args.short else []) + (['-leader-only'] if args.leader_only else [])))
