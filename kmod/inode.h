// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_INODE_H
#define _TERNFS_INODE_H

#include <linux/fs.h>

#include "bincode.h"
#include "latch.h"
#include "log.h"
#include "net.h"
#include "policy.h"
#include "rs.h"
#include "span.h"


#define TERNFS_ROOT_INODE 0x2000000000000000ull

extern unsigned ternfs_disable_ftruncate;

struct ternfs_transient_span;

#define TERNFS_FILE_STATUS_NONE 0     // we have created the inode, we haven't opened it yet
#define TERNFS_FILE_STATUS_READING 1  // the file has been linked (we can reopen it at will)
#define TERNFS_FILE_STATUS_WRITING 2  // the file is transient, we're writing it

struct ternfs_inode_file {
    int status;

    // Normal file stuff

    struct ternfs_file_spans spans;

    // Transient file stuff. Only initialized on file creation (rather than opening),
    // otherwise it's garbage.
    // Could be factored out to separate data structure since it's completely useless
    // when reading.

    u64 cookie;
    // If we've encountered an error such that we want to stop writing to this file
    // forever.
    atomic_t transient_err;
    // Span we're currently writing to. Might be NULL.
    struct ternfs_transient_span* writing_span;
    // Whether we're currently flushing a span (block write + add span certify)
    struct semaphore flushing_span_sema;
    // We use this to track where we should close the file from.
    struct task_struct* owner;
    // We store these one separatedly from `owner` above because when a process exits
    // it frees the mm before it frees the files. And we need the mm to account the
    // MM_FILEPAGES in the file flushing logic.
    struct mm_struct* mm;
};

struct ternfs_dirents {
    // used to know when we need to refresh the dirents
    u64 mtime;
    // used to know when we're done quickly
    u64 max_hash;
    // to GC this structure
    atomic_t refcount;
    // to synchronize between modifications of the reference count
    struct rcu_head rcu_head;
    // the list of pages with the dirents in them. always at least
    // one page in it.
    //
    // we use fields of each page to store stuff:
    // * ->index: number of entries in the page
    // * ->private: first hash appearing in the page
    struct list_head pages;
};

struct ternfs_inode_dir {
    u64 mtime_expiry; // in jiffies

    // In `struct page`, we use:
    //
    // ->mapping to store the next page
    // ->private to store the number of entries stored in the page (high 32 bits),
    //     and a reference count with how many people need the cache (low 32 bits).
    //     The reference count is only stored in the "head" page.
    // ->rcu_head to synchronize between modifications to the reference count.
    // ->index in the first page to store the dir mtime we've tagged the dir with
    //     (used to invalidate the dir contents).
    struct ternfs_dirents __rcu * dirents;
    struct ternfs_latch dirents_latch;
};

struct ternfs_inode {
    struct inode inode;

    // We cache things based on the ternfs mtime, but we need to decide when the
    // mtime itself is stale for the purposes of dir lookups, which we do using
    // `mtime_expiry`.
    u64 mtime; // in ternfs time

    u64 edge_creation_time; // in ternfs time, used for operations (re)moving the edge

    // These are relevant for directoriese (obviously), but we also use them for transient
    // files when we create them, since we need it in many places.
    struct ternfs_policy* block_policy;
    struct ternfs_policy* span_policy;
    struct ternfs_policy* stripe_policy;
    // We use the snapshot policy to immediately declare file transient if policy
    // requests it. This is to reduce load on gc for high churn directories where we
    // don't care about snapshots.
    struct ternfs_policy* snapshot_policy;

    union {
        struct ternfs_inode_file file;
        struct ternfs_inode_dir dir;
    };

    // There is always at most one metadata request in flight for getattr.
    // This is regulated by `getattr_update_latch`. We do getattr in two ways:
    // 1. Synchronously, so we just take the `getattr_update_latch` and
    //    do the metadata request normally;
    // 2. Asynchronously, where we take the `getattr_update_latch` and then
    //    have `getattr_async_work` complete it, without retries.
    //    Due to async nature of completion it may race with init and relase the
    //    latch before init completes. To avoid it we use `getattr_update_init_latch`.
    // Method 2 is used when doing speculative getattrs when opening directories.
    u64 getattr_expiry;
    struct ternfs_latch getattr_update_latch;
    struct ternfs_latch getattr_update_init_latch;
    struct ternfs_metadata_request getattr_async_req;
    struct delayed_work getattr_async_work;
    s64 getattr_async_seqno;
};

#define TERNFS_I(ptr) container_of(ptr, struct ternfs_inode, inode)

#define TERNFS_INODE_DIRECTORY 1
#define TERNFS_INODE_FILE 2
#define TERNFS_INODE_SYMLINK 3

static inline u64 ternfs_inode_type(u64 ino) {
    return (ino >> 61) & 0x03;
}

static inline u32 ternfs_inode_shard(u64 ino) {
    return ino & 0xff;
}

struct inode* ternfs_get_inode(
    struct super_block* sb,
    // Are we OK with not having `parent`? This is currently only OK
    // in the context of NFS.
    bool allow_no_parent,
    struct ternfs_inode* parent,
    u64 ino
);

static inline struct inode* ternfs_get_inode_normal(
    struct super_block* sb,
    struct ternfs_inode* parent,
    u64 ino
) {
    return ternfs_get_inode(sb, false, parent, ino);
}

static inline struct inode* ternfs_get_inode_export(
    struct super_block* sb,
    struct ternfs_inode* parent,
    u64 ino
) {
    return ternfs_get_inode(sb, true, parent, ino);
}

// super ops
struct inode* ternfs_inode_alloc(struct super_block* sb);
void ternfs_inode_evict(struct inode* inode);
void ternfs_inode_free(struct inode* inode);

// inode ops
enum { ATTR_CACHE_NORM_TIMEOUT, ATTR_CACHE_DIR_TIMEOUT, ATTR_CACHE_NO_TIMEOUT };
int ternfs_do_getattr(struct ternfs_inode* enode, int cache_timeout_type);
// 0: not started
// 1: started
// -n: error
int ternfs_start_async_getattr(struct ternfs_inode* enode);

int __init ternfs_inode_init(void);
void __cold ternfs_inode_exit(void);

#endif
