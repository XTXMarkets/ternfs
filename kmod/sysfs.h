// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_SYSFS_H
#define _TERNFS_SYSFS_H

#include <linux/init.h>

extern const char* ternfs_revision;

int __init ternfs_sysfs_init(void);
void __cold ternfs_sysfs_exit(void);

#endif

