#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

base_img=""

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        *)
            if [[ -n "$base_img" ]]; then
                echo "Bad usage -- base image specified twice"
                exit 2
            else
                base_img=$(realpath ${1})
                shift
            fi
            ;;
    esac
done

if [[ -z "$base_img" ]]; then
    echo "Bad usage -- no base image"
    exit 2
fi

echo "Preparing kmod CI environment with base image $base_img"

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR

# prepare linux sources
./fetchlinux.sh

# build linux kernel
(cd linux && make oldconfig && make prepare && make -j)

# create vm image
./createimg.sh "$base_img"
