# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

include_directories(${ternfs_SOURCE_DIR}/core)

set(GO_CRC32C_DIR "${ternfs_SOURCE_DIR}/../go/core/crc32c")
set(CRC32C_FILES crc32c.h crc32c.c crc32c_body.c crc32c_pclmul.c)
foreach(FILE IN LISTS CRC32C_FILES)
    configure_file(
        "${GO_CRC32C_DIR}/${FILE}"
        "${CMAKE_CURRENT_BINARY_DIR}/${FILE}"
        COPYONLY
    )
endforeach()

add_library(
    crc32c
    "${CMAKE_CURRENT_BINARY_DIR}/crc32c.h"
    "${CMAKE_CURRENT_BINARY_DIR}/crc32c.c"
    "${CMAKE_CURRENT_BINARY_DIR}/crc32c_pclmul.c"
)
target_include_directories(crc32c PUBLIC "${CMAKE_CURRENT_BINARY_DIR}")

add_executable(crc32c-tests tests.cpp)
target_link_libraries(crc32c-tests PRIVATE crc32c core)
