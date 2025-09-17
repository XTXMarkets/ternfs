// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_CRC_H
#define _TERNFS_CRC_H

#include <linux/kernel.h>

// You _must_ wrap this with kernel_fpu_begin/kernel_fpu_end!
// It's not required for the other functions.
u32 ternfs_crc32c(u32 crc, const char* buf, size_t len);

// The only difference with `ternfs_crc32c` is that kernel_fpu_begin/kernel_fpu_end
// are not required for this one.
u32 ternfs_crc32c_simple(u32 crc, const char* buf, size_t len);

u32 ternfs_crc32c_xor(u32 crc1, u32 crc2, size_t len);

u32 ternfs_crc32c_append(u32 crc1, u32 crc2, size_t len2);

u32 ternfs_crc32c_zero_extend(u32 crc, ssize_t zeros);

#endif
