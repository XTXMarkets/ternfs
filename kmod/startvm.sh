#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

qemu-system-x86_64 \
    -machine accel=kvm,type=q35 \
    -enable-kvm \
    -cpu host \
    -kernel "${SCRIPT_DIR}/linux/arch/x86/boot/bzImage" \
    -append "root=/dev/sda1 single console=ttyS0 systemd.unit=graphical.target" \
    -drive file="${SCRIPT_DIR}/ubuntu.img,format=qcow2",index=0,media=disk,cache=unsafe \
    -drive file="${SCRIPT_DIR}/init.img,format=raw",index=1,media=disk,cache=unsafe \
    -m 128G \
    -smp $(nproc),cores=$(nproc) \
    -nographic \
    -nic user,hostfwd=tcp::2223-:22
