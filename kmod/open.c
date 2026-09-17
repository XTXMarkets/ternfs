// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "file.h"
#include "trace.h"

int ternfs_file_open(struct inode* inode, struct file* filp) {
    int err = 0;
    struct ternfs_inode* enode = TERNFS_I(inode);

    trace_eggsfs_inode_lock(inode, TERNFS_INODE_LOCK, "file_open");
    inode_lock(inode);

    if (enode->file.status == TERNFS_FILE_STATUS_WRITING) {
        // A procfs fd link can reopen an unpublished file. Its inode still
        // owns write buffers: neither change its state nor let a read-only
        // descriptor flush the original writer's file on close.
        if (!(filp->f_mode & FMODE_WRITE)) {
            err = -EBUSY;
        }
    } else {
        // Existing files remain readable regardless of their open mode;
        // unsupported operations such as writing fail at the operation.
        enode->file.status = TERNFS_FILE_STATUS_READING;
    }

    inode_unlock(inode);
    trace_eggsfs_inode_lock(inode, TERNFS_INODE_UNLOCK, "file_open");
    return err;
}
