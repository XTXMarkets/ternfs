# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

include_directories(${ternfs_SOURCE_DIR}/core)

set(GO_RS_DIR "${ternfs_SOURCE_DIR}/../go/core/rs")
set(RS_FILES rs.h rs.cpp rs_core.c gf_tables.c)
foreach(FILE IN LISTS RS_FILES)
    configure_file(
        "${GO_RS_DIR}/${FILE}"
        "${CMAKE_CURRENT_BINARY_DIR}/${FILE}"
        COPYONLY
    )
endforeach()

add_library(
    rs
    "${CMAKE_CURRENT_BINARY_DIR}/rs.h"
    "${CMAKE_CURRENT_BINARY_DIR}/rs.cpp"
    "${CMAKE_CURRENT_BINARY_DIR}/gf_tables.c"
)
target_include_directories(rs PUBLIC "${CMAKE_CURRENT_BINARY_DIR}")

add_executable(rs-tests tests.cpp)
target_link_libraries(rs-tests PRIVATE rs core)

add_executable(rs-bench bench.cpp)
target_link_libraries(rs-bench PRIVATE rs core)
