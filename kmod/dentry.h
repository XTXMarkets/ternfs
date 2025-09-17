// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_DENTRY_H
#define _TERNFS_DENTRY_H

#include <linux/fs.h>

#include "counter.h"
#include "inode_compat.h"

TERNFS_DECLARE_COUNTER(ternfs_stat_dir_revalidations);

extern struct dentry_operations ternfs_dentry_ops;

struct dentry* ternfs_lookup(struct inode* dir, struct dentry* dentry, unsigned int flags);
int ternfs_rmdir(struct inode* dir, struct dentry* dentry);
int ternfs_unlink(struct inode* dir, struct dentry* dentry);
int COMPAT_FUNC_UNS(ternfs_mkdir, struct inode* dir, struct dentry* dentry, umode_t mode);
int COMPAT_FUNC_UNS(
    ternfs_rename,
    struct inode* old_dir, struct dentry* old_dentry,
    struct inode* new_dir, struct dentry* new_dentry,
    unsigned int flags
);

#endif /* _TERNFS_DENTRY_H */
