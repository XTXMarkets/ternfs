# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

include_directories(${ternfs_SOURCE_DIR}/core ${ternfs_SOURCE_DIR}/shard ${ternfs_SOURCE_DIR}/registry)

add_executable(tests tests.cpp doctest.h)
target_link_libraries(tests PRIVATE core shard cdc)

add_executable(logsdbtests logsdbtests.cpp doctest.h)
target_link_libraries(logsdbtests PRIVATE core)

add_executable(registrydbtests registrydbtests.cpp doctest.h)
target_link_libraries(registrydbtests PRIVATE core registry)
