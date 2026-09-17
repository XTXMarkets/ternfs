// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/mm.h>
#include <linux/sched/mm.h>
#include <linux/slab.h>

#include "file.h"
#include "transient.h"
#include "write.h"

static struct kmem_cache* ternfs_transient_span_cachep;

static void init_transient_span(void* p) {
    struct ternfs_transient_span* span = p;
    INIT_LIST_HEAD(&span->pages);
    int i;
    for (i = 0; i < TERNFS_MAX_BLOCKS; i++) {
        INIT_LIST_HEAD(&span->blocks[i]);
    }
    atomic_set(&span->refcount, 0);
    spin_lock_init(&span->lock);
}

struct ternfs_transient_span* ternfs_new_transient_span(struct ternfs_inode* enode, u64 offset) {
    struct ternfs_transient_span* span = kmem_cache_alloc(ternfs_transient_span_cachep, GFP_KERNEL);
    if (!span) { return NULL; }
    span->enode = enode;
    BUG_ON(atomic_read(&span->refcount) != 0);
    atomic_set(&span->refcount, 1);
    span->offset = offset;
    span->written = 0;
    memset(span->failure_domains, 0, sizeof(span->failure_domains));
    memset(span->blacklisted_failure_domains, 0, sizeof(span->blacklisted_failure_domains));
    memset(span->blocks_proofs, 0, sizeof(span->blocks_proofs));
    memset(span->blocks_errs, 0, sizeof(span->blocks_errs));
    span->span_crc = 0;
    memset(span->cell_crcs, 0, sizeof(span->cell_crcs));
    span->attempts = 0;
    // Destruction is safe even before a parity layout has been chosen.
    span->parity = 0;
    span->blacklist_length = 0;
    span->started_flushing = false;
    return span;
}

void ternfs_hold_transient_span(struct ternfs_transient_span* span) {
    atomic_inc(&span->refcount);
}

static unsigned release_write_pages(struct list_head* pages) {
    unsigned count = 0;
    struct page* page;
    struct page* next;
    list_for_each_entry_safe(page, next, pages, lru) {
        list_del(&page->lru);
        put_page(page);
        count++;
    }
    return count;
}

bool ternfs_put_transient_span(struct ternfs_transient_span* span) {
    if (atomic_dec_return(&span->refcount) != 0) { return false; }
    BUG_ON(spin_is_locked(&span->lock));
    unsigned pages = release_write_pages(&span->pages);
    int b;
    for (b = 0; b < ternfs_blocks(span->parity); b++) {
        pages += release_write_pages(&span->blocks[b]);
    }
    // A socket may still own page references after a send error. The span
    // no longer owns them, so remove its charge from the writer's RSS.
    ternfs_account_write_pages(span->enode->file.mm, -(long)pages);
    if (span->started_flushing) {
        up(&span->enode->file.flushing_span_sema);
    }
    kmem_cache_free(ternfs_transient_span_cachep, span);
    return true;
}

void ternfs_release_transient_file(struct ternfs_inode* enode) {
    BUG_ON(!inode_is_locked(&enode->inode));
    // A barrier, not a permanent acquisition: close/error cleanup can run
    // more than once. The final flushing-span put releases this semaphore.
    down(&enode->file.flushing_span_sema);
    up(&enode->file.flushing_span_sema);
    if (enode->file.writing_span) {
        BUG_ON(!ternfs_put_transient_span(enode->file.writing_span));
        enode->file.writing_span = NULL;
    }
    if (enode->file.mm) {
        mmdrop(enode->file.mm);
        enode->file.mm = NULL;
    }
}

void ternfs_file_discard(struct ternfs_inode* enode) {
    inode_lock(&enode->inode);
    if (enode->file.status == TERNFS_FILE_STATUS_WRITING) {
        // Eviction is the final owner; it must also clean files whose open
        // failed or whose last close did not run in the creating process.
        atomic_cmpxchg(&enode->file.transient_err, 0, -EIO);
        ternfs_release_transient_file(enode);
    }
    inode_unlock(&enode->inode);
}

int __init ternfs_file_init(void) {
    ternfs_transient_span_cachep = kmem_cache_create(
        "ternfs_transient_span_cache", sizeof(struct ternfs_transient_span),
        0, SLAB_RECLAIM_ACCOUNT, init_transient_span
    );
    return ternfs_transient_span_cachep ? 0 : -ENOMEM;
}

void __cold ternfs_file_exit(void) {
    kmem_cache_destroy(ternfs_transient_span_cachep);
}
