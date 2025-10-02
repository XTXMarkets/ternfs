#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

# version=5.4.237
version=6.12.49

# Download or resume
curl -C - -O "https://cdn.kernel.org/pub/linux/kernel/v${version%%.*}.x/linux-${version}.tar.gz"

# Check
sha512sum -c "linux-${version}.tar.gz.sha512"

# Extract
tar xf "linux-${version}.tar.gz"

# Copy config
cp "config-kasan-${version}" "linux-${version}/.config"

# Create symlink
rm -f linux
ln -sf linux-${version} linux
