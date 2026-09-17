// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_TRANSIENT_H
#define _TERNFS_TRANSIENT_H

#include "inode.h"
#include "metadata.h"

struct ternfs_transient_span {
    // The file must outlive its spans: cleanup waits for the flushing span.
    struct ternfs_inode* enode;
    u64 offset;
    // Private write pages; the last page's index is its next write offset.
    struct list_head pages;
    u32 written;
    atomic_t refcount;

    char failure_domains[TERNFS_MAX_BLOCKS][16];
    char blacklisted_failure_domains[TERNFS_MAX_BLACKLIST_LENGTH][16];
    // Once flushing starts, data and parity pages are arranged per block.
    struct list_head blocks[TERNFS_MAX_BLOCKS];
    u64 block_ids[TERNFS_MAX_BLOCKS];
    spinlock_t lock;
    u64 blocks_proofs[TERNFS_MAX_BLOCKS];
    int blocks_errs[TERNFS_MAX_BLOCKS];
    u32 span_crc;
    u32 cell_crcs[TERNFS_MAX_BLOCKS * TERNFS_MAX_STRIPES];
    u32 block_size;
    u8 attempts;
    u8 stripes;
    u8 storage_class;
    u8 parity;
    u8 blacklist_length;
    bool started_flushing;
};

static_assert(sizeof(struct ternfs_transient_span) < (2 << 10));

struct ternfs_transient_span* ternfs_new_transient_span(struct ternfs_inode* enode, u64 offset);
void ternfs_hold_transient_span(struct ternfs_transient_span* span);
bool ternfs_put_transient_span(struct ternfs_transient_span* span);

// Requires the inode lock and initialized transient state. Waits for any
// flushing span, frees the current writing span, then drops the saved mm.
void ternfs_release_transient_file(struct ternfs_inode* enode);

#endif
