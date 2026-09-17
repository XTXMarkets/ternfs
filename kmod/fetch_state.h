// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_FETCH_STATE_H
#define _TERNFS_FETCH_STATE_H

#include <linux/semaphore.h>
#include <linux/spinlock.h>
#include <linux/wait.h>

#include "span.h"

struct fetch_span_pages_state {
    // Borrowed from the caller, which must finish all callbacks before return.
    struct ternfs_block_span* block_span;
    struct address_space* mapping;
    struct semaphore sema;
    u32 start_offset;
    u32 size;
    atomic_t start_block;
    atomic_t end_block;
    struct list_head blocks_pages[TERNFS_MAX_BLOCKS];
    atomic_t refcount; // one caller reference plus submitted callbacks
    spinlock_t refs_lock;
    wait_queue_head_t idle;
    atomic_t err;
    atomic_t last_block_err;
    static_assert(TERNFS_MAX_BLOCKS <= 16);
    // Bits 0..15 downloading, 16..31 succeeded, 32..47 failed.
    atomic64_t blocks;
};

struct fetch_span_pages_state* ternfs_new_fetch_span_pages_state(struct ternfs_block_span* span);
// The caller may hold while submitting; callbacks may hold while retrying.
void ternfs_hold_fetch_span_pages(struct fetch_span_pages_state* st);
void ternfs_put_fetch_span_pages(struct fetch_span_pages_state* st);
// Called only by the original caller, after it has stopped submitting work.
// Drains callbacks, then releases the caller reference and remaining pages.
void ternfs_finish_fetch_span_pages(struct fetch_span_pages_state* st);

int __init ternfs_fetch_span_pages_init(void);
void __cold ternfs_fetch_span_pages_exit(void);

#endif
