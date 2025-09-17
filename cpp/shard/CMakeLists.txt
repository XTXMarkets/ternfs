# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

include_directories(${ternfs_SOURCE_DIR}/core ${ternfs_SOURCE_DIR}/crc32c)

add_library(shard Shard.cpp Shard.hpp ShardDB.cpp ShardDB.hpp ShardDBData.cpp ShardDBData.hpp BlockServicesCacheDB.hpp BlockServicesCacheDB.cpp)
target_link_libraries(shard PRIVATE core)

add_executable(ternshard ternshard.cpp)
target_link_libraries(ternshard PRIVATE core shard crc32c ${TERNFS_JEMALLOC_LIBS})
