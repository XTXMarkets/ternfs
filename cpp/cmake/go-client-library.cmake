# Copyright 2027 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

function(ternfs_install_go_client_library target header package_name description)
    set(TERNFS_PC_LIBRARY "${target}")
    set(TERNFS_PC_NAME "${package_name}")
    set(TERNFS_PC_DESCRIPTION "${description}")

    configure_file(
        "${ternfs_SOURCE_DIR}/cmake/ternfs-library-uninstalled.pc.in"
        "${CMAKE_CURRENT_BINARY_DIR}/${package_name}.pc"
        @ONLY
    )
    configure_file(
        "${ternfs_SOURCE_DIR}/cmake/ternfs-library.pc.in"
        "${CMAKE_CURRENT_BINARY_DIR}/${package_name}-installed.pc"
        @ONLY
    )

    install(
        TARGETS "${target}"
        ARCHIVE DESTINATION "${CMAKE_INSTALL_LIBDIR}"
    )
    install(
        FILES "${header}"
        DESTINATION "${CMAKE_INSTALL_INCLUDEDIR}/ternfs"
    )
    install(
        FILES "${CMAKE_CURRENT_BINARY_DIR}/${package_name}-installed.pc"
        DESTINATION "${CMAKE_INSTALL_LIBDIR}/pkgconfig"
        RENAME "${package_name}.pc"
    )
endfunction()
