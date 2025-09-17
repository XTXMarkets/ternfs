# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

include_directories(${ternfs_SOURCE_DIR}/core ${ternfs_SOURCE_DIR}/wyhash)

add_library(registry Registry.hpp Registry.cpp RegistryDB.hpp RegistryDB.cpp 
    RegistryDBData.hpp Registerer.hpp Registerer.cpp RegistryServer.hpp RegistryServer.cpp)
target_link_libraries(registry PRIVATE core)

add_executable(ternregistry ternregistry.cpp)
target_link_libraries(ternregistry PRIVATE core registry crc32c ${TERNFS_JEMALLOC_LIBS})
