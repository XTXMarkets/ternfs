#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR

build_variant=$1
out_dir=build/$build_variant
mkdir -p build/$build_variant

${PWD}/cpp/build.py alpine rs crc32c # build C libs we need for for go
${PWD}/go/build.py --generate # generate C++ files

# build C++
${PWD}/cpp/build.py $build_variant

# build go
${PWD}/go/build.py

# copy binaries
binaries=(
    cpp/build/$build_variant/registry/ternregistry
    cpp/build/$build_variant/shard/ternshard
    cpp/build/$build_variant/dbtools/terndbtools
    cpp/build/$build_variant/cdc/terncdc
    cpp/build/$build_variant/ktools/ternktools
    go/ternweb/ternweb
    go/ternrun/ternrun
    go/ternblocks/ternblocks
    go/ternfuse/ternfuse
    go/terncli/terncli
    go/terngc/terngc
    go/terntests/terntests
    go/ternregistryproxy/ternregistryproxy
)

for binary in "${binaries[@]}"; do
    cp $binary $out_dir
done
