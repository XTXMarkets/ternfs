#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR

build_variant=$1

# Determine if we need a container
container=
case $build_variant in
    alpine|alpinedebug)
        container=ghcr.io/xtxmarkets/ternfs-alpine-build:2025-09-18-1
        ;;
    ubuntu|ubuntudebug|ubuntusanitized|ubuntuvalgrind)
        container=${TERN_UBUNTU_BUILD_CONTAINER:-ghcr.io/xtxmarkets/ternfs-ubuntu-build:2025-09-18}
        ;;
esac

# Re-exec inside container if needed
if [ -n "$container" ] && [ "${IN_TERN_BUILD_CONTAINER:-}" != "1" ]; then
    # See <https://groups.google.com/g/seastar-dev/c/r7W-Kqzy9O4>
    # for motivation for `--security-opt seccomp=unconfined`,
    # the `--pids-limit -1` is not something I hit but it seems
    # like a good idea.
    exec docker run \
        --network host \
        --pids-limit -1 \
        --security-opt seccomp=unconfined \
        --rm -i \
        --mount "type=bind,src=${SCRIPT_DIR},dst=/ternfs" \
        -u "$(id -u):$(id -g)" \
        -e IN_TERN_BUILD_CONTAINER=1 \
        -e MAKE_PARALLELISM -e http_proxy -e https_proxy -e no_proxy \
        -e GOCACHE=/ternfs/.cache \
        -e GOMODCACHE=/ternfs/.go-cache \
        "$container" \
        /ternfs/build.sh "$@"
fi

# Normalize build variant to cmake build type
is_static_build=false
case $build_variant in
    alpine*) is_static_build=true ;;
esac
case $build_variant in
    ubuntu|alpine) cmake_build_type=release ;;
    ubuntudebug|alpinedebug) cmake_build_type=debug ;;
    ubuntusanitized) cmake_build_type=sanitized ;;
    ubuntuvalgrind) cmake_build_type=valgrind ;;
    *) cmake_build_type=$build_variant ;;
esac
static_flag=""
if [ "$is_static_build" = "true" ]; then
    static_flag="--static"
fi

# ensure output directory exists
out_dir=build/$build_variant
mkdir -p $out_dir

# build C libs we need for go (always as a release build)
case $build_variant in
    ubuntu*) golibs_variant=ubuntu ;;
    alpine*) golibs_variant=alpine ;;
    *) golibs_variant=release ;;
esac
${PWD}/cpp/build.py $golibs_variant --cmake-build-type=release $static_flag rs crc32c
cp cpp/build/$golibs_variant/crc32c/libcrc32c.a go/core/crc32c/
cp cpp/build/$golibs_variant/rs/librs.a go/core/rs/

${PWD}/go/build.py --generate # generate C++ files

# build C++
${PWD}/cpp/build.py $build_variant --cmake-build-type=$cmake_build_type $static_flag

# build go
${PWD}/go/build.py $static_flag

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
