#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

set -x

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR

./sync.sh $1
ssh $1 'cd eggs/kmod && make -j kmod'