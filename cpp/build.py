#!/usr/bin/env python3

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

import os
from pathlib import Path
import subprocess
import argparse

parser = argparse.ArgumentParser()
parser.add_argument('build_variant')
parser.add_argument('--cmake-build-type', help='CMake build type (default: same as build variant)')
parser.add_argument('--static', action='store_true', help='Static build')
args, ninja_args = parser.parse_known_intermixed_args()

cmake_build_type = args.cmake_build_type or args.build_variant

cpp_dir = Path(__file__).resolve().parent

build_dir = cpp_dir / 'build' / args.build_variant
build_dir.mkdir(parents=True, exist_ok=True)

os.chdir(str(build_dir))
subprocess.run(['cmake', '-G', 'Ninja', f'-DCMAKE_BUILD_TYPE={cmake_build_type}', f'-DTERN_STATIC_BUILD={args.static}', '../..'], check=True)
subprocess.run(['ninja'] + ninja_args, check=True)
