#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR

set -x

./linux/scripts/clang-tools/gen_compile_commands.py
sed -i 's:-I.:-I./linux:g' ./compile_commands.json
sed -i 's:-include .:-include ./linux:g' ./compile_commands.json