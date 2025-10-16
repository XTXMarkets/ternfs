# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

find_program(MAKE_EXE NAMES gmake nmake make)

if(${CMAKE_VERSION} VERSION_GREATER "3.23")
    cmake_policy(SET CMP0135 NEW)
endif()

include(ProcessorCount)
ProcessorCount(MAKE_PARALLELISM)
if(DEFINED ENV{MAKE_PARALLELISM})
    set(MAKE_PARALLELISM "$ENV{MAKE_PARALLELISM}")
endif()

# Depends on: nothing
# Dependecy of: rocksdb
# License: dual MIT and GPLv2
# We build this manually because alpine doesn't have liburing-static
ExternalProject_Add(make_uring
    DOWNLOAD_DIR ${CMAKE_CURRENT_BINARY_DIR}
    URL https://github.com/axboe/liburing/archive/refs/tags/liburing-2.3.tar.gz
    URL_HASH SHA256=60b367dbdc6f2b0418a6e0cd203ee0049d9d629a36706fcf91dfb9428bae23c8
    PREFIX thirdparty/uring
    UPDATE_COMMAND ""
    SOURCE_DIR ${make_uring_SOURCE_DIR}
    CONFIGURE_COMMAND ./configure --prefix=<INSTALL_DIR>
    BUILD_IN_SOURCE 1
    BUILD_COMMAND ""
    BUILD_BYPRODUCTS <INSTALL_DIR>/lib/liburing.a
    # We don't `make` in BUILD_COMMAND because building the tests is currently broken with musl
    # due to their dependency on non-standard `error.h`.
    INSTALL_COMMAND ${MAKE_EXE} -j ${MAKE_PARALLELISM} install
    LOG_DOWNLOAD ON
    LOG_CONFIGURE ON
    LOG_INSTALL ON
    LOG_BUILD ON
    LOG_OUTPUT_ON_FAILURE ON
)
add_library(uring STATIC IMPORTED)
ExternalProject_Get_property(make_uring INSTALL_DIR)
include_directories(SYSTEM ${INSTALL_DIR}/include)
set_target_properties(uring PROPERTIES IMPORTED_LOCATION ${INSTALL_DIR}/lib/liburing.a)

# Depends on: none
# Dependency of: rocksdb
# License: BSD
ExternalProject_Add(make_lz4
    DOWNLOAD_DIR ${CMAKE_CURRENT_BINARY_DIR}
    URL https://github.com/lz4/lz4/archive/refs/tags/v1.9.4.tar.gz
    URL_HASH SHA256=0b0e3aa07c8c063ddf40b082bdf7e37a1562bda40a0ff5272957f3e987e0e54b
    PREFIX thirdparty/lz4
    UPDATE_COMMAND ""
    SOURCE_DIR ${make_lz4_SOURCE_DIR}
    CONFIGURE_COMMAND ""
    BUILD_IN_SOURCE 1
    BUILD_COMMAND ${MAKE_EXE} -j ${MAKE_PARALLELISM}
    BUILD_BYPRODUCTS <INSTALL_DIR>/lib/liblz4.a
    INSTALL_COMMAND ${MAKE_EXE} install PREFIX=<INSTALL_DIR>
    LOG_DOWNLOAD ON
    LOG_CONFIGURE ON
    LOG_INSTALL ON
    LOG_BUILD ON
    LOG_OUTPUT_ON_FAILURE ON
)
add_library(lz4 STATIC IMPORTED)
ExternalProject_Get_property(make_lz4 INSTALL_DIR)
set(lz4_INSTALL_DIR ${INSTALL_DIR})
include_directories(SYSTEM ${INSTALL_DIR}/include)
set_target_properties(lz4 PROPERTIES IMPORTED_LOCATION ${INSTALL_DIR}/lib/liblz4.a)

# Depends on: none
# Dependency of: rocksdb
# License: dual BSD and GPLv2
ExternalProject_Add(make_zstd
    DOWNLOAD_DIR ${CMAKE_CURRENT_BINARY_DIR}
    URL https://github.com/facebook/zstd/archive/refs/tags/v1.5.2.tar.gz
    URL_HASH SHA256=f7de13462f7a82c29ab865820149e778cbfe01087b3a55b5332707abf9db4a6e
    PREFIX thirdparty/zstd
    UPDATE_COMMAND ""
    SOURCE_DIR ${make_zstd_SOURCE_DIR}
    CONFIGURE_COMMAND ""
    BUILD_IN_SOURCE 1
    BUILD_COMMAND ${MAKE_EXE} -j ${MAKE_PARALLELISM}
    BUILD_BYPRODUCTS <INSTALL_DIR>/lib/libzstd.a
    INSTALL_COMMAND ${MAKE_EXE} install PREFIX=<INSTALL_DIR>
    LOG_DOWNLOAD ON
    LOG_CONFIGURE ON
    LOG_INSTALL ON
    LOG_BUILD ON
    LOG_OUTPUT_ON_FAILURE ON
)
add_library(zstd STATIC IMPORTED)
ExternalProject_Get_property(make_zstd INSTALL_DIR)
include_directories(SYSTEM ${INSTALL_DIR}/include)
set_target_properties(zstd PROPERTIES IMPORTED_LOCATION ${INSTALL_DIR}/lib/libzstd.a)

# Depends on: lz4, zstd
# Dependency of: eggs
# License: dual Apache 2.0 and GPLv2
if(${CMAKE_BUILD_TYPE} STREQUAL "valgrind")
    set(ROCKS_DB_MARCH "-march=haswell") # Valgind can't support current -march=native instructions
endif()
# musl doesn't seem to like AVX512, at least for now.
separate_arguments(
    rocksdb_build UNIX_COMMAND
    "env ROCKSDB_DISABLE_ZLIB=y ROCKSDB_DISABLE_BZIP=1 ROCKSDB_DISABLE_SNAPPY=1 ${MAKE_EXE} USE_RTTI=1 EXTRA_CXXFLAGS='${ROCKS_DB_MARCH} -mno-avx512f -DROCKSDB_NO_DYNAMIC_EXTENSION' EXTRA_CFLAGS='${ROCKS_DB_MARCH} -mno-avx512f' -j ${MAKE_PARALLELISM} static_lib"
)
ExternalProject_Add(make_rocksdb
    DOWNLOAD_DIR ${CMAKE_CURRENT_BINARY_DIR}
    URL https://github.com/facebook/rocksdb/archive/refs/tags/v7.9.2.tar.gz
    URL_HASH SHA256=886378093098a1b2521b824782db7f7dd86224c232cf9652fcaf88222420b292
    # When we upgraded dev boxes to newer arch and therefore newer clang this was
    # needed. New RocksDB (e.g. 8.10.0) compiles out of the box, but we don't have
    # a great way to test this upgrade on the live cluster.
    #
    # The || true is so that the patch application is idempotent (there's no way to have
    # patch return 0 when -N skips patches, which is what we want here). This is safe
    # since the code is hashed anyway, but if you change the RocksDB release (which
    # you probably shouldn't without some thorough testing) you should keep this in mind.
    PATCH_COMMAND sh -c "patch -N -p1 < ${CMAKE_CURRENT_SOURCE_DIR}/rocksdb-stdint.diff || true"
    PREFIX thirdparty/rocksdb
    UPDATE_COMMAND ""
    SOURCE_DIR ${make_rocksdb_SOURCE_DIR}
    CONFIGURE_COMMAND ""
    BUILD_IN_SOURCE 1
    BUILD_COMMAND ${rocksdb_build}
    BUILD_BYPRODUCTS <INSTALL_DIR>/lib/librocksdb.a
    INSTALL_COMMAND ${MAKE_EXE} install-static PREFIX=<INSTALL_DIR>
    LOG_DOWNLOAD ON
    LOG_CONFIGURE ON
    LOG_INSTALL ON
    LOG_BUILD ON
    LOG_OUTPUT_ON_FAILURE ON
)
add_library(rocksdb STATIC IMPORTED)
ExternalProject_Get_property(make_rocksdb INSTALL_DIR)
include_directories(SYSTEM ${INSTALL_DIR}/include)
set_target_properties(rocksdb PROPERTIES IMPORTED_LOCATION ${INSTALL_DIR}/lib/librocksdb.a)

# Depends on: none
# Dependency of: eggs
# License: BSD
ExternalProject_Add(make_xxhash
    DOWNLOAD_DIR ${CMAKE_CURRENT_BINARY_DIR}
    URL https://github.com/Cyan4973/xxHash/archive/refs/tags/v0.8.1.tar.gz
    URL_HASH SHA256=3bb6b7d6f30c591dd65aaaff1c8b7a5b94d81687998ca9400082c739a690436c
    PREFIX thirdparty/xxhash
    UPDATE_COMMAND ""
    SOURCE_DIR ${make_xxhash_SOURCE_DIR}
    CONFIGURE_COMMAND ""
    BUILD_IN_SOURCE 1
    BUILD_COMMAND ${MAKE_EXE} -j ${MAKE_PARALLELISM}
    BUILD_BYPRODUCTS <INSTALL_DIR>/lib/libxxhash.a
    INSTALL_COMMAND ${MAKE_EXE} install PREFIX=<INSTALL_DIR>
    LOG_DOWNLOAD ON
    LOG_CONFIGURE ON
    LOG_INSTALL ON
    LOG_BUILD ON
    LOG_OUTPUT_ON_FAILURE ON
)
add_library(xxhash STATIC IMPORTED)
ExternalProject_Get_property(make_xxhash INSTALL_DIR)
include_directories(SYSTEM ${INSTALL_DIR}/include)
set_target_properties(xxhash PROPERTIES IMPORTED_LOCATION ${INSTALL_DIR}/lib/libxxhash.a)

set(DEP_MIMALLOC_CMAKE_ARGS
    -DCMAKE_INSTALL_PREFIX:PATH=<INSTALL_DIR>
    -DCMAKE_BUILD_TYPE=Release
    -DMI_BUILD_SHARED:BOOL=OFF
    -DMI_BUILD_TESTS:BOOL=OFF
    -DMI_OPT_ARCH:BOOL=ON
)
if("${CMAKE_BUILD_TYPE}" MATCHES "^alpine")
    list(APPEND DEP_MIMALLOC_CMAKE_ARGS -DMI_LIBC_MUSL:BOOL=ON)
endif()

# License: BSD
ExternalProject_Add(make_mimalloc
    DOWNLOAD_DIR ${CMAKE_CURRENT_BINARY_DIR}
    URL https://github.com/microsoft/mimalloc/archive/refs/tags/v3.0.10.tar.gz
    URL_HASH SHA256=ee5556a31060f2289497f00126e90bf871e90933f98e21ea13dca3578e9ccfb5
    PREFIX thirdparty/mimalloc
    UPDATE_COMMAND ""
    SOURCE_DIR ${make_mimalloc_SOURCE_DIR}
    CMAKE_ARGS ${DEP_MIMALLOC_CMAKE_ARGS}
    BUILD_BYPRODUCTS <INSTALL_DIR>/lib/mimalloc-3.0/libmimalloc.a
    LOG_DOWNLOAD ON
    LOG_CONFIGURE ON
    LOG_INSTALL ON
    LOG_BUILD ON
    LOG_OUTPUT_ON_FAILURE ON
)
add_library(mimalloc STATIC IMPORTED)
ExternalProject_Get_property(make_mimalloc INSTALL_DIR)
include_directories(SYSTEM ${INSTALL_DIR}/include)
set_target_properties(mimalloc PROPERTIES IMPORTED_LOCATION ${INSTALL_DIR}/lib/mimalloc-3.0/libmimalloc.a)

# This explicit dependency tracking is needed for ninja, which is blind to the
# include dependencies from our code into the above, apparently.
add_custom_target(thirdparty)
add_dependencies(thirdparty make_uring make_lz4 make_zstd make_rocksdb make_xxhash make_mimalloc)
