// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/init.h>
#include <linux/kernel.h>
#include <linux/module.h>
#include <linux/version.h>
#include <linux/tracepoint.h>

#include "wq.h"
#include "inode.h"
#include "metadata.h"
#include "block.h"
#include "super.h"
#include "sysctl.h"
#include "sysfs.h"
#include "block.h"
#include "rs.h"
#include "log.h"
#include "span.h"
#include "dir.h"
#include "file.h"
#include "debugfs.h"
#include "policy.h"
#include "block_services.h"

MODULE_LICENSE("GPL");

// general purpose wq this is where various async completion function run
struct workqueue_struct* ternfs_wq;

// fast wq, we don't want delays in these operations
struct workqueue_struct* ternfs_fast_wq;

static int __init ternfs_init(void) {
    int err;

    ternfs_debug("initializing module");
    ternfs_info("revision %s", ternfs_revision);

    ternfs_shard_init();

    ternfs_wq = alloc_workqueue("ternfs-wq", 0, 0);
    if (!ternfs_wq) { return -ENOMEM; }

    ternfs_fast_wq = alloc_workqueue("ternfs-fast-wq", 0, 0);
    if (!ternfs_fast_wq) { goto out_fast_wq; }

    err = ternfs_rs_init();
    if (err) { goto out_rs; }

    err = ternfs_policy_init();
    if (err) { goto out_policy; }

    err = ternfs_block_service_init();
    if (err) { goto out_block_service; }

    err = ternfs_sysfs_init();
    if (err) { goto out_sysfs; }

    err = ternfs_sysctl_init();
    if (err) { goto out_sysctl; }

    err = ternfs_block_init();
    if (err) { goto out_block; }

    err = ternfs_span_init();
    if (err) { goto out_span; }

    err = ternfs_inode_init();
    if (err) { goto out_inode; }

    err = ternfs_dir_init();
    if (err) { goto out_dir; }

    err = ternfs_file_init();
    if (err) { goto out_file; }

    err = ternfs_fs_init();
    if (err) { goto out_fs; }

    err = ternfs_debugfs_init();
    if (err) { goto out_debugfs; }

    return 0;

out_debugfs:
    ternfs_fs_exit();
out_fs:
    ternfs_file_exit();
out_file:
    ternfs_dir_exit();
out_dir:
    ternfs_inode_exit();
out_inode:
    ternfs_span_exit();
out_span:
    ternfs_block_exit();
out_block:
    ternfs_sysctl_exit();
out_sysctl:
    ternfs_sysfs_exit();
out_sysfs:
    ternfs_block_service_exit();
out_block_service:
    ternfs_policy_exit();
out_policy:
    ternfs_rs_exit();
out_rs:
    destroy_workqueue(ternfs_fast_wq);
out_fast_wq:
    destroy_workqueue(ternfs_wq);
    return err;
}

static void __exit ternfs_exit(void) {
    ternfs_debugfs_exit();
    ternfs_fs_exit();
    ternfs_file_exit();
    ternfs_dir_exit();
    ternfs_inode_exit();
    ternfs_span_exit();
    ternfs_block_exit();
    ternfs_sysctl_exit();
    ternfs_sysfs_exit();
    ternfs_block_service_exit();
    ternfs_policy_exit();
    ternfs_rs_exit();

    // tracepoint_synchronize_unregister() must be called before the end of
    // the module exit function to make sure there is no caller left using
    // the probe. This, and the fact that preemption is disabled around the
    // probe call, make sure that probe removal and module unload are safe.
    tracepoint_synchronize_unregister();

    ternfs_debug("destroying workqueues");
    destroy_workqueue(ternfs_fast_wq);
    destroy_workqueue(ternfs_wq);
}

module_init(ternfs_init);
module_exit(ternfs_exit);

