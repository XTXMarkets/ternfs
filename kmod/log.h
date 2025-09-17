// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_LOG_H
#define _TERNFS_LOG_H

#include "sysctl.h"

#define ternfs_error(fmt, args...) printk(KERN_ERR "ternfs: %s: " fmt "\n", __func__, ##args)
#define ternfs_warn(fmt, args...) printk(KERN_WARNING "ternfs: %s: " fmt "\n", __func__, ##args)
#define ternfs_info(fmt, args...) printk(KERN_INFO "ternfs: %s: " fmt "\n", __func__, ##args)
#define ternfs_debug(fmt, args...) \
    if (unlikely(ternfs_debug_output)) { \
        printk(KERN_DEBUG "ternfs: %s: " fmt "\n", __func__, ##args); \
    }

#endif
