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

int __init ternfs_dir_init(void);
void __cold ternfs_dir_exit(void);

#endif

