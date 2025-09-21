// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <immintrin.h>
#include <stdint.h>
#include <string.h>
#include <sys/types.h>

typedef uint8_t u8;
typedef uint32_t u32;
typedef uint64_t u64;

#define CRC32C_USE_PCLMUL 1
#define CRC32C_NAME(a) a##_pclmul
#include "crc32c_body.c"