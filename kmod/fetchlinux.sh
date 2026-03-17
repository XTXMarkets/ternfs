#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

if [[ ${1-} = wsl ]]; then

version=msft-wsl-6.6.87.2
wsl_dir=WSL2-Linux-Kernel-linux-$version

curl --etag-save "etag-$version" --etag-compare "etag-$version" \
    -LO "https://github.com/microsoft/WSL2-Linux-Kernel/archive/refs/tags/linux-$version.tar.gz"

sha512sum -c "linux-$version.tar.gz.sha512"

tar xf "linux-$version.tar.gz"
cp "$wsl_dir/Microsoft/config-wsl" .
sed -i /CONFIG_DEBUG_PREEMPT=/s/y/n/ config-wsl
cp config-wsl "$wsl_dir/.config"
ln -sf "$wsl_dir" wsl-linux

else

# version=5.4.237
version=6.14.9

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

fi
