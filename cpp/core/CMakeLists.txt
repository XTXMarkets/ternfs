# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

file(GLOB core_sources CONFIGURE_DEPENDS "*.cpp")
file(GLOB core_headers CONFIGURE_DEPENDS "*.hpp")
file(GLOB logsdb_sources CONFIGURE_DEPENDS "logsdb/*.cpp")
file(GLOB logsdb_headers CONFIGURE_DEPENDS "logsdb/*.hpp")

add_library(core ${core_sources} ${core_headers} ${logsdb_sources} ${logsdb_headers})
add_dependencies(core thirdparty)
target_link_libraries(core PRIVATE rocksdb lz4 zstd uring xxhash rs crc32c)
