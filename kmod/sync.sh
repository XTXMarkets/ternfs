#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR

rsync -vaxAXL --include '*.py' --include '*.c' --include '*.h' --include 'Makefile' --exclude '*' ./ $1:eggs/kmod/
