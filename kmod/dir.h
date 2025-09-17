// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_DIR_H
#define _TERNFS_DIR_H

#include <linux/fs.h>

#include "inode.h"
#include "log.h"

extern struct file_operations ternfs_dir_operations;

extern int ternfs_dir_getattr_refresh_time_jiffies;
extern int ternfs_dir_dentry_refresh_time_jiffies;

int ternfs_dir_readdir_entry_cb(void* ptr, u64 name_hash, const char* name, int name_len, u64 edge_creation_time, u64 ino);

void ternfs_dir_drop_cache(struct ternfs_inode* enode);

static inline int ternfs_dir_revalidate(struct ternfs_inode* enode) {
    return ternfs_do_getattr(enode, ATTR_CACHE_DIR_TIMEOUT);
}

static inline int ternfs_dir_needs_reval(struct ternfs_inode* dir, struct dentry* dentry) {
    // 0 => invalid
    // -ECHILD => revalidate parent dir
    // 1 => valid
    u64 dentry_dir_mtime = dentry->d_time;
    int ret;
    u64 t = get_jiffies_64();
    if (dentry_dir_mtime != dir->mtime) { ret = 0; }
    else if (t >= dir->dir.mtime_expiry) { ret = -ECHILD; }
    else { ret = 1; }

    ternfs_debug(
        "t=%llu dir=%p dir_id=0x%016lx dir_mtime=%llu dir_mtime_expiry=%llu dentry=%pd dentry_dir_mtime=%llu -> ret=%d",
        t, dir, dir->inode.i_ino, dir->mtime, dir->dir.mtime_expiry, dentry, dentry_dir_mtime, ret
    );

    return ret;
}

int __init ternfs_dir_init(void);
void __cold ternfs_dir_exit(void);

#endif

