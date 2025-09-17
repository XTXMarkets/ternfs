// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "crc.h"

#ifndef __CHECKER__ // sparse doesn't like this code at all.

#include "intrshims.h"

#include "log.h"
#include "crc32c.c"

u32 ternfs_crc32c(u32 crc, const char* buf, size_t len) {
    return crc32c(crc, buf, len);
}

u32 ternfs_crc32c_simple(u32 crc, const char* buf, size_t len) {
    return crc32c_simple(crc, buf, len);
}

u32 ternfs_crc32c_xor(u32 crc1, u32 crc2, size_t len) {
    return crc32c_xor(crc1, crc2, len);
}

u32 ternfs_crc32c_append(u32 crc1, u32 crc2, size_t len2) {
    return crc32c_append(crc1, crc2, len2);
}

u32 ternfs_crc32c_zero_extend(u32 crc, ssize_t zeros) {
    return crc32c_zero_extend(crc, zeros);
}

#endif
