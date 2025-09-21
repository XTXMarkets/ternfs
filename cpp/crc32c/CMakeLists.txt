# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

include_directories(${ternfs_SOURCE_DIR}/core)

add_library(crc32c crc32c.h crc32c.c crc32c_pclmul.c)

add_executable(crc32c-tests tests.cpp)
target_link_libraries(crc32c-tests PRIVATE crc32c core)
