# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

cmake_minimum_required(VERSION 3.21)
include(ExternalProject)

# _must_ be done before everything else <https://gitlab.kitware.com/cmake/community/-/wikis/FAQ#method-3-avoid-use-set>
set(CMAKE_C_COMPILER clang)
set(CMAKE_CXX_COMPILER clang++)

project(ternfs)

# Yes, this means that it won't work for multi-configuration stuff, which is fine for now.
if(NOT CMAKE_BUILD_TYPE)
    set(
        CMAKE_BUILD_TYPE release CACHE STRING
        "Choose the type of build, options are: 'debug', 'release', 'alpine', 'valgrind', 'sanitized'."
        FORCE
    )
endif()

if (NOT (${CMAKE_BUILD_TYPE} MATCHES "^(debug|release|alpine|alpinedebug|sanitized|valgrind)$"))
    message(FATAL_ERROR "Build type must be one of debug, release, sanitized, alpine, alpinedebug got ${CMAKE_BUILD_TYPE}")
endif()

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_REQUIRED true)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)
option(BUILD_SHARED_LIBS OFF)
set(POSITION_INDEPENDENT_CODE off)
add_compile_options(-fno-stack-protector -fdiagnostics-color=always -fno-stack-protector -g -gdwarf-4 -Wall -Wextra -Wno-sign-compare -Wno-unused-parameter -Wno-unused-variable -Wno-gnu-string-literal-operator-template -Wno-format-security)
add_link_options(-fuse-ld=lld -pthread)

# valgrind we currently use cannot digest certain instructions
add_compile_options("$<$<CONFIG:valgrind>:-march=haswell;-maes;-mgfni>")
add_compile_options("$<$<NOT:$<CONFIG:valgrind>>:-march=skylake;-mgfni>")

# performance/debug stuff
add_compile_options("$<$<NOT:$<CONFIG:debug,alpinedebug>>:-O3>")
add_compile_options("$<$<CONFIG:debug,alpinedebug>:-Og;-DTERN_DEBUG>")

# We build the release build statically in Alpine
add_compile_options("$<$<CONFIG:alpine,alpinedebug>:-DTERN_ALPINE>")
add_link_options("$<$<CONFIG:alpine,alpinedebug>:-static>")
add_link_options("$<$<NOT:$<CONFIG:alpine,alpinedebug>>:-no-pie>")

# sanitizer options
set(SANITIZE_OPTIONS "-fsanitize=undefined,address,integer,function;-fno-sanitize-recover=all;-fsanitize-blacklist=${CMAKE_SOURCE_DIR}/ubsan-ignorelist")
add_compile_options("$<$<CONFIG:sanitized>:${SANITIZE_OPTIONS}>")
add_link_options("$<$<CONFIG:sanitized>:${SANITIZE_OPTIONS}>")

# we only use jemalloc in release builds, alpine doesn't seem to
# like jemalloc very much, and sanitizer etc works better without it
if ("${CMAKE_BUILD_TYPE}" STREQUAL "release")
    set(TERNFS_JEMALLOC_LIBS "jemalloc")
endif()

include(thirdparty.cmake)

add_subdirectory(rs)
add_subdirectory(crc32c)
add_subdirectory(core)
add_subdirectory(dbtools)
add_subdirectory(registry)
add_subdirectory(shard)
add_subdirectory(cdc)
add_subdirectory(tests)
add_subdirectory(ktools)
