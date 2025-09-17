# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

include_directories(${ternfs_SOURCE_DIR}/core ${ternfs_SOURCE_DIR}/shard)

add_library(cdc CDC.cpp CDC.hpp CDCDB.cpp CDCDB.hpp CDCDBData.hpp)
target_link_libraries(cdc PRIVATE core)

add_executable(terncdc terncdc.cpp)
target_link_libraries(terncdc PRIVATE core shard cdc ${TERNFS_JEMALLOC_LIBS})
