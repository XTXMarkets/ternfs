// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_KERNEL_FLOOR_H
#define _TERNFS_KERNEL_FLOOR_H

#include <linux/version.h>

#if LINUX_VERSION_CODE < KERNEL_VERSION(6, 12, 0)
#error "TernFS requires Linux 6.12 or newer"
#endif

#endif
