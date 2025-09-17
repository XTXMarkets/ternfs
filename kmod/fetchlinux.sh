#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

version=linux-5.4.237

# Download or resume
curl -C - -O "https://cdn.kernel.org/pub/linux/kernel/v5.x/${version}.tar.gz"

# Check
sha512sum -c "${version}.tar.gz.sha512"

# Extract
tar xf "${version}.tar.gz"

# Copy config
cp config-kasan ${version}/.config

# Create symlink
ln -sf ${version} linux
