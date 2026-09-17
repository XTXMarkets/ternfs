// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/mm.h>
#include <linux/slab.h>

#include "fetch_state.h"
#include "page_compat.h"

static struct kmem_cache* ternfs_fetch_span_pages_cachep;

static void init_fetch_span_pages(void* p) {
    struct fetch_span_pages_state* st = p;
    spin_lock_init(&st->refs_lock);
    init_waitqueue_head(&st->idle);
    int i;
    for (i = 0; i < TERNFS_MAX_BLOCKS; i++) {
        INIT_LIST_HEAD(&st->blocks_pages[i]);
    }
}

struct fetch_span_pages_state* ternfs_new_fetch_span_pages_state(struct ternfs_block_span* span) {
    struct fetch_span_pages_state* st = kmem_cache_alloc(ternfs_fetch_span_pages_cachep, GFP_KERNEL);
    if (!st) { return NULL; }
    // Reset completion signals on every allocation, including slab reuse.
    sema_init(&st->sema, 0);
    st->block_span = span;
    st->mapping = NULL;
    atomic_set(&st->refcount, 1);
    atomic_set(&st->err, 0);
    atomic_set(&st->last_block_err, 0);
    atomic64_set(&st->blocks, 0);
    atomic_set(&st->start_block, 0);
    atomic_set(&st->end_block, 0);
    st->start_offset = 0;
    st->size = 0;
    return st;
}

void ternfs_hold_fetch_span_pages(struct fetch_span_pages_state* st) {
    WARN_ON(atomic_inc_return(&st->refcount) < 2);
}

void ternfs_put_fetch_span_pages(struct fetch_span_pages_state* st) {
    // Serialize the final caller put with the last callback's wakeup. A
    // refcount-only wait could observe 1 and free idle before wake_up_all.
    spin_lock_bh(&st->refs_lock);
    int remaining = atomic_dec_return(&st->refcount);
    BUG_ON(remaining < 0);
    if (remaining == 1) { wake_up_all(&st->idle); }
    spin_unlock_bh(&st->refs_lock);
    if (remaining != 0) { return; }

    int i;
    for (i = 0; i < TERNFS_MAX_BLOCKS; i++) {
        put_pages_list(&st->blocks_pages[i]);
    }
    kmem_cache_free(ternfs_fetch_span_pages_cachep, st);
}

void ternfs_finish_fetch_span_pages(struct fetch_span_pages_state* st) {
    // Even after a terminal error, other block requests may still reference
    // the caller's span and mapping. Keep the caller alive until they stop.
    wait_event(st->idle, atomic_read(&st->refcount) == 1);
    ternfs_put_fetch_span_pages(st);
}

int __init ternfs_fetch_span_pages_init(void) {
    ternfs_fetch_span_pages_cachep = kmem_cache_create(
        "ternfs_fetch_span_pages_cache", sizeof(struct fetch_span_pages_state),
        0, SLAB_RECLAIM_ACCOUNT, init_fetch_span_pages
    );
    return ternfs_fetch_span_pages_cachep ? 0 : -ENOMEM;
}

void __cold ternfs_fetch_span_pages_exit(void) {
    kmem_cache_destroy(ternfs_fetch_span_pages_cachep);
}
