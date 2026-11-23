#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

version=${1:-6.14.9}
fragment=${2:-}
case "$version" in
    6.12.44|6.14.9) ;;
    *) echo "Usage: $0 [6.12.44|6.14.9] [lockdep]" >&2; exit 2 ;;
esac

# Download or resume
curl -C - -O "https://cdn.kernel.org/pub/linux/kernel/v${version%%.*}.x/linux-${version}.tar.gz"

# Check
sha512sum -c "linux-${version}.tar.gz.sha512"

# Extract
tar xf "linux-${version}.tar.gz"

# Copy config
if [[ "$version" = 6.12.44 ]]; then
    cp config-kasan-6.12.49 "linux-${version}/.config"
    make -C "linux-${version}" olddefconfig
    make -C "linux-${version}" -j"$(nproc)" modules_prepare
else
    cp "config-kasan-${version}" "linux-${version}/.config"
    if [[ -n "$fragment" ]]; then
        "linux-${version}/scripts/kconfig/merge_config.sh" -m -O "linux-${version}" "linux-${version}/.config" "config-${fragment}.fragment"
        make -C "linux-${version}" olddefconfig
        # olddefconfig silently drops options with unmet dependencies
        if grep -vxFf "linux-${version}/.config" "config-${fragment}.fragment"; then
            echo "options above from config-${fragment}.fragment are not in .config" >&2
            exit 1
        fi
    fi

    # Keep the QEMU kernel on 6.14.9.
    rm -f linux
    ln -sf linux-${version} linux

fi
