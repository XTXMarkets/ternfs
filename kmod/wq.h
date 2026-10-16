// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_WQ_H
#define _TERNFS_WQ_H

#include <linux/init.h>

extern struct workqueue_struct* ternfs_wq;

extern struct workqueue_struct* ternfs_fast_wq;

int __init ternfs_workqueues_init(void);
// Call after all work producers and their deferred callbacks are stopped.
void ternfs_workqueues_exit(void);

#endif
