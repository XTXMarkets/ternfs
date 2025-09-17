# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

include_directories(${ternfs_SOURCE_DIR}/core ${ternfs_SOURCE_DIR}/shard ${ternfs_SOURCE_DIR}/cdc)

add_library(sharddbtools ShardDBTools.hpp ShardDBTools.cpp LogsDBTools.hpp LogsDBTools.cpp CDCDBTools.hpp CDCDBTools.cpp)
target_link_libraries(sharddbtools PRIVATE core shard cdc)

add_executable(terndbtools terndbtools.cpp)
target_link_libraries(terndbtools PRIVATE core shard sharddbtools ${TERNFS_JEMALLOC_LIBS})
