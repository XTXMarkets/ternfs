#!/usr/bin/env python3

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

import os
from pathlib import Path
import shutil
import subprocess
import argparse

parser = argparse.ArgumentParser()
parser.add_argument('build_variant')
parser.add_argument('--cmake-build-type', help='CMake build type (default: same as build variant)')
parser.add_argument('--cmake-install-prefix', help='CMake install prefix')
parser.add_argument('--cmake-install-libdir', help='CMake install library directory')
parser.add_argument('--static', action='store_true', help='Static build')
args, ninja_args = parser.parse_known_intermixed_args()

cmake_build_type = args.cmake_build_type or args.build_variant
ninja = shutil.which('ninja')
if ninja is None:
    parser.error('ninja is required')

cpp_dir = Path(__file__).resolve().parent

build_dir = cpp_dir / 'build' / args.build_variant
cache_file = build_dir / 'CMakeCache.txt'
if cache_file.exists():
    cache = cache_file.read_text()
    expected_paths = (
        f'CMAKE_CACHEFILE_DIR:INTERNAL={build_dir}',
        f'CMAKE_HOME_DIRECTORY:INTERNAL={cpp_dir}',
    )
    if not all(path in cache for path in expected_paths):
        print(
            f'Removing CMake cache created in a different location: {build_dir}',
            flush=True,
        )
        shutil.rmtree(build_dir)
build_dir.mkdir(parents=True, exist_ok=True)

os.chdir(str(build_dir))
cmake_args = [
    'cmake',
    '-G',
    'Ninja',
    f'-DCMAKE_MAKE_PROGRAM:FILEPATH={ninja}',
    f'-DCMAKE_BUILD_TYPE={cmake_build_type}',
    f'-DTERN_STATIC_BUILD={args.static}',
]
if args.cmake_install_prefix:
    cmake_args.append(f'-DCMAKE_INSTALL_PREFIX={args.cmake_install_prefix}')
if args.cmake_install_libdir:
    cmake_args.append(f'-DCMAKE_INSTALL_LIBDIR={args.cmake_install_libdir}')
subprocess.run(cmake_args + ['../..'], check=True)
subprocess.run([ninja] + ninja_args, check=True)
