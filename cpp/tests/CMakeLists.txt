# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

include_directories(${ternfs_SOURCE_DIR}/core ${ternfs_SOURCE_DIR}/shard ${ternfs_SOURCE_DIR}/registry)

add_executable(tests tests.cpp doctest.h)
target_link_libraries(tests PRIVATE core shard cdc)

add_executable(logsdbtests logsdbtests.cpp doctest.h)
target_link_libraries(logsdbtests PRIVATE core)

add_executable(logsdbreqresptests logsdbreqresptests.cpp doctest.h)
target_link_libraries(logsdbreqresptests PRIVATE core)

add_executable(logsdbdatapartitionstests logsdbdatapartitionstests.cpp doctest.h)
target_link_libraries(logsdbdatapartitionstests PRIVATE core)

add_executable(logsdbmetadatatests logsdbmetadatatests.cpp doctest.h)
target_link_libraries(logsdbmetadatatests PRIVATE core)

add_executable(logsdbcatchupreadertests logsdbcatchupreadertests.cpp doctest.h)
target_link_libraries(logsdbcatchupreadertests PRIVATE core)

add_executable(logsdbleaderelectiontests logsdbleaderelectiontests.cpp doctest.h)
target_link_libraries(logsdbleaderelectiontests PRIVATE core)

add_executable(registrydbtests registrydbtests.cpp doctest.h)
target_link_libraries(registrydbtests PRIVATE core registry)

add_executable(blockservicepickertests blockservicepickertests.cpp doctest.h)
target_link_libraries(blockservicepickertests PRIVATE core shard)

add_executable(blockservicescachedbtests blockservicescachedbtests.cpp doctest.h)
target_link_libraries(blockservicescachedbtests PRIVATE core shard)
