// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/errno.h>
#include <linux/workqueue.h>

#include "wq.h"

struct workqueue_struct* ternfs_wq;
struct workqueue_struct* ternfs_fast_wq;

int __init ternfs_workqueues_init(void) {
    ternfs_wq = alloc_workqueue("ternfs-wq", 0, 0);
    if (!ternfs_wq) { return -ENOMEM; }

    ternfs_fast_wq = alloc_workqueue("ternfs-fast-wq", 0, 0);
    if (!ternfs_fast_wq) {
        destroy_workqueue(ternfs_wq);
        ternfs_wq = NULL;
        return -ENOMEM;
    }
    return 0;
}

void ternfs_workqueues_exit(void) {
    if (ternfs_fast_wq) {
        destroy_workqueue(ternfs_fast_wq);
        ternfs_fast_wq = NULL;
    }
    if (ternfs_wq) {
        destroy_workqueue(ternfs_wq);
        ternfs_wq = NULL;
    }
}
