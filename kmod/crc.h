// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

// Functions postfixed with _fpu must be wrapped in
// kernel_fpu_begin/kernel_fpu_end. They're also generally
// faster.
#ifndef _TERNFS_CRC_H
#define _TERNFS_CRC_H

#include <linux/kernel.h>

u32 ternfs_crc32c(u32 crc, const char* buf, size_t len);
u32 ternfs_crc32c_xor(u32 crc1, u32 crc2, size_t len);
u32 ternfs_crc32c_append(u32 crc1, u32 crc2, size_t len2);
u32 ternfs_crc32c_zero_extend(u32 crc, ssize_t zeros);

u32 ternfs_crc32c_fpu(u32 crc, const char* buf, size_t len);
u32 ternfs_crc32c_xor_fpu(u32 crc1, u32 crc2, size_t len);
u32 ternfs_crc32c_append_fpu(u32 crc1, u32 crc2, size_t len2);
u32 ternfs_crc32c_zero_extend_fpu(u32 crc, ssize_t zeros);

#endif
