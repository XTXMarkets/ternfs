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

parser = argparse.ArgumentParser()
parser.add_argument('--race', action='store_true', help='Build Go with -race')
parser.add_argument('--static', action='store_true', help='Statically link')
parser.add_argument('--generate', action='store_true', help='Run generate rather than build')
parser.add_argument('--test', action='store_true', help='Run tests rather than build')
parser.add_argument(
    '--native-variant',
    default='go-release',
    help='CMake build directory for native Go dependencies (default: go-release)',
)
parser.add_argument('--close-tracker', action='store_true', help='Build the BPF object file for the ternfuse close tracker')
parser.add_argument('paths', nargs='*')
args = parser.parse_args()

paths = args.paths

if args.generate and (args.race or args.static or args.test or paths or args.close_tracker):
    print('--generate only works as the only flag')
    sys.exit(2)

if args.close_tracker and (args.race or args.static or args.test or paths or args.generate):
    print('--close-tracker only works as the only flag')
    sys.exit(2)

if args.close_tracker:
    print('Dumping vmlinux.h')
    with open(go_dir / 'closetracker' / 'vmlinux.h', 'w') as f:
        subprocess.run(['bpftool', 'btf', 'dump', 'file', '/sys/kernel/btf/vmlinux', 'format', 'c'], stdout=f, check=True)
    print('Building closetracker.bpf.o')
    subprocess.run(['clang', '-g', '-O2', '-target', 'bpf', '-c', go_dir / 'closetracker' / 'closetracker.bpf.c', '-o', go_dir / 'closetracker' / 'closetracker.bpf.o'], check=True)
    sys.exit(0)

if args.generate:
    subprocess.run(['go', 'generate', './...'], cwd=go_dir, check=True)
else:
    cpp_dir = go_dir.parent / 'cpp'
    native_build = [
        str(cpp_dir / 'build.py'),
        args.native_variant,
        '--cmake-build-type=release',
    ]
    if args.static:
        native_build.append('--static')
    subprocess.run(native_build + ['rs', 'crc32c'], check=True)

    env = os.environ.copy()
    pkg_config_dirs = [
        cpp_dir / 'build' / args.native_variant / 'crc32c',
        cpp_dir / 'build' / args.native_variant / 'rs',
    ]
    pkg_config_path = os.pathsep.join(str(path) for path in pkg_config_dirs)
    if env.get('PKG_CONFIG_PATH'):
        pkg_config_path += os.pathsep + env['PKG_CONFIG_PATH']
    env['PKG_CONFIG_PATH'] = pkg_config_path

    go_flags = (
        (['-ldflags=-extldflags=-static'] if args.static else [])
        + (['-race'] if args.race else [])
    )

    if args.test:
        subprocess.run(
            ['go', 'test'] + go_flags + (paths or ['./...']),
            cwd=go_dir,
            env=env,
            check=True,
        )
        sys.exit(0)

    if len(paths) == 0:
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

    for path_str in paths:
        path = go_dir / Path(path_str)
        print(f'Building {path_str}')
        subprocess.run(
            ['go', 'build'] + go_flags + ['.'],
            cwd=str(path),
            env=env,
            check=True,
        )
