// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "file.h"
#include "inode_compat.h"
#include "metadata.h"
#include "trace.h"

int ternfs_atime_on_open = 0;

static void update_atime_on_open(struct ternfs_inode* enode) {
    struct inode* inode = &enode->inode;
    u64 now_ns = ktime_get_real_ns();
    struct timespec64 now = ns_to_timespec64(now_ns);
    unsigned interval = READ_ONCE(ternfs_atime_update_interval_sec);
    time64_t cur_sec = inode_get_atime_sec(inode);
    u64 diff = now.tv_sec - min(cur_sec, now.tv_sec);

    // Unlike the VFS callback, do not advance cached atime when throttled:
    // repeated opens on a read-only mount must not postpone the next RPC.
    if (diff < interval) { return; }

    // Another client may already have updated atime. Refresh and recheck the
    // interval to avoid an unnecessary metadata write.
    int err = ternfs_do_getattr(enode, ATTR_CACHE_NO_TIMEOUT);
    if (err) {
        ternfs_warn("file=%016lx open atime getattr failed err=%d", inode->i_ino, err);
        return;
    }
    cur_sec = inode_get_atime_sec(inode);
    diff = now.tv_sec - min(cur_sec, now.tv_sec);
    if (diff < interval) { return; }
    if (cur_sec > now.tv_sec ||
        (cur_sec == now.tv_sec && inode_get_atime_nsec(inode) >= now.tv_nsec)) {
        return;
    }

    err = ternfs_shard_set_time(
        (struct ternfs_fs_info*)inode->i_sb->s_fs_info,
        inode->i_ino, 0, now_ns | (1ull << 63));
    if (err) {
        ternfs_warn("file=%016lx open atime shard_set_time failed err=%d", inode->i_ino, err);
        return;
    }
    inode_set_atime_to_ts(inode, now);
    smp_store_release(&enode->getattr_expiry, 0);
}

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
        if (READ_ONCE(ternfs_atime_on_open) && !(filp->f_flags & O_NOATIME)) {
            // Intentionally bypass VFS mount atime policy, including read-only
            // mounts. This opt-in keeps retention atimes fresh for RO clients.
            // Failures are best-effort and must not prevent opening the file.
            update_atime_on_open(enode);
        }
    }

    inode_unlock(inode);
    trace_eggsfs_inode_lock(inode, TERNFS_INODE_UNLOCK, "file_open");
    return err;
}
