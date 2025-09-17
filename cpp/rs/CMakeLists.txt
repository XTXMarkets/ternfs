# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

include_directories(${ternfs_SOURCE_DIR}/core)

add_library(rs rs.h rs.cpp gf_tables.c)

add_executable(rs-tests tests.cpp)
target_link_libraries(rs-tests PRIVATE rs core)

add_executable(rs-bench bench.cpp)
target_link_libraries(rs-bench PRIVATE rs core)
