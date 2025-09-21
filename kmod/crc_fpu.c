// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "crc.h"

#ifndef __CHECKER__ // sparse doesn't like this code at all.

#include "intrshims.h"

#include "log.h"

#define CRC32C_USE_PCLMUL 1
#define CRC32C_NAME(a) ternfs_##a##_fpu
#include "crc32c_body.c"

#endif