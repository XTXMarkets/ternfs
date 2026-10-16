// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_GETATTR_COMPLETION_H
#define _TERNFS_GETATTR_COMPLETION_H

#include "inode.h"

static inline void ternfs_finish_async_getattr(struct ternfs_inode* enode) {
    // This request may own the last inode reference. Finish all latch and
    // waitqueue accesses before iput can evict/reclaim the enclosing inode.
    u64 seqno = enode->getattr_async_seqno;
    ternfs_latch_release(&enode->getattr_update_latch, seqno);
    iput(&enode->inode);
}

#endif
