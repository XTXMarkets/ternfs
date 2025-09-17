#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

base_img=$(realpath ${1})

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR

set -x

qemu-img create -f qcow2 -F qcow2 -b $base_img ubuntu.img 100G
cloud-localds init.img image-init.yml

# remove existing key if any
if [[ -f "~/.ssh/known_hosts" ]]; then
    ssh-keygen -R '[localhost]:2223' >/dev/null
fi
