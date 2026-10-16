// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/uio.h>

#include "file.h"
#include "trace.h"

int ternfs_finish_symlink(struct ternfs_inode* enode, struct dentry* dentry, const char* path) {
    enode->file.status = TERNFS_FILE_STATUS_WRITING;
    loff_t ppos = 0;
    struct kvec vec = { .iov_base = (void*)path, .iov_len = strlen(path) };
    struct iov_iter from;
    iov_iter_kvec(&from, WRITE, &vec, 1, vec.iov_len);

    trace_eggsfs_inode_lock(&enode->inode, TERNFS_INODE_LOCK, "symlink");
    inode_lock(&enode->inode);
    int err = ternfs_file_write(enode, 0, &ppos, &from);
    inode_unlock(&enode->inode);
    trace_eggsfs_inode_lock(&enode->inode, TERNFS_INODE_UNLOCK, "symlink");
    if (err < 0) { goto out_err; }

    err = ternfs_file_flush(enode, dentry);
    if (err < 0) { goto out_err; }

    d_instantiate(dentry, &enode->inode);
    return 0;

out_err:
    // No dentry took the new inode reference. Dropping it must evict this
    // unpublished inode so its transient pages and saved mm are released.
    clear_nlink(&enode->inode);
    iput(&enode->inode);
    return err;
}
