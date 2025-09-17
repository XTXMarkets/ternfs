#!/usr/bin/env python3

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

import sys
import os
from pathlib import Path
import subprocess
import argparse
import re

go_dir = Path(__file__).resolve().parent
repo_dir = go_dir.parent

parser = argparse.ArgumentParser()
parser.add_argument('--race', action='store_true', help='Build Go with -race')
parser.add_argument('--generate', action='store_true', help='Run generate rather than build')
parser.add_argument('paths', nargs='*')
args = parser.parse_args()

paths = args.paths

if args.generate and (args.race or paths):
    print('--generate only works as the only flag')
    os.exit(2)

if not args.generate and len(paths) == 0:
    vendor_dir = go_dir / 'vendor'
    pattern = re.compile(r'^package main', re.MULTILINE)
    paths = set()
    for root, _, files in os.walk(go_dir):
        for file in files:
            if file.endswith('.go'):
                file_path = os.path.join(root, file)
                if not file_path.startswith(str(vendor_dir)):
                    with open(file_path, 'r', encoding='utf-8') as f:
                        content = f.read()
                        if pattern.search(content):
                            paths.add(os.path.dirname(file_path))


if 'IN_TERN_BUILD_CONTAINER' not in os.environ:
    container = 'ghcr.io/xtxmarkets/ternfs-alpine-build:2025-09-03'
    # See <https://groups.google.com/g/seastar-dev/c/r7W-Kqzy9O4>
    # for motivation for `--security-opt seccomp=unconfined`,
    # the `--pids-limit -1` is not something I hit but it seems
    # like a good idea.
    subprocess.run(
        ['docker', 'run', '--pids-limit', '-1', '--security-opt', 'seccomp=unconfined', '--rm', '-i', '--mount', f'type=bind,src={repo_dir},dst=/ternfs', '-u', f'{os.getuid()}:{os.getgid()}', container, '/ternfs/go/build.py'] + sys.argv[1:],
        check=True,
    )
else:
    # Otherwise go will try to create the cache in /.cache, which won't work
    # since we're not running as root.
    os.environ['GOCACHE'] = '/ternfs/.cache'
    os.environ['GOMODCACHE'] = '/ternfs/.go-cache'
    if args.generate:
        subprocess.run(['go', 'generate', './...'], cwd=go_dir, check=True)
    else:
        for path_str in paths:
            path = go_dir / Path(path_str)
            print(f'Building {path_str}')
            subprocess.run(
                ['go', 'build', '-ldflags=-extldflags=-static'] + (["-race"] if args.race else []) + ['.'],
                cwd=str(path),
                check=True,
            )
