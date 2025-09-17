// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_SPAN_H
#define _TERNFS_SPAN_H

#include <linux/kernel.h>

#include "counter.h"
#include "block.h"
#include "rs.h"
#include "super.h"

// span cache for specific inode
struct ternfs_file_spans {
    u64 __ino;
    // Spans manipulation could be done in a lockless way on the read-side, but
    // as of now we use a read/write semaphore for simplicity (we'd need to
    // be careful when freeing spans).
    struct rb_root __spans;
    struct rw_semaphore __lock;    
};

void ternfs_init_file_spans(struct ternfs_file_spans* spans, u64 ino);

void ternfs_free_file_spans(struct ternfs_file_spans* spans);

struct ternfs_span {
    // All the spans operations are independent from `struct ternfs_inode`.
    // So we need the inode here.
    u64 ino;
    u64 start;
    u64 end;
    // To be in the inode tree. Note that we might _not_ be in the
    // tree once the inode releases us.
    struct rb_node node;
    // The inode and in progress reads hold references to us.
    // The reads can decide to unlink the span on error but other in 
    // progress reads for same span need to finish before this can be freed.
    atomic_t refcount;
    // Used to determine which type of span this is enclosed in.
    u8 storage_class;
};

struct ternfs_inline_span {
    struct ternfs_span span;
    u8 len;
    u8 body[255];
};

#define TERNFS_INLINE_SPAN(_span) ({ \
        BUG_ON((_span)->storage_class != TERNFS_INLINE_STORAGE); \
        container_of(_span, struct ternfs_inline_span, span); \
    })

struct ternfs_block {
    struct ternfs_stored_block_service* bs;
    u64 id;
    u32 crc;
};

struct ternfs_block_span {
    struct ternfs_span span;
    struct ternfs_block blocks[TERNFS_MAX_BLOCKS];
    u32 cell_size;
    u32 stripes_crc[TERNFS_MAX_STRIPES];
    u8 num_stripes;
    u8 parity;
};

#define TERNFS_BLOCK_SPAN(_span) ({ \
        BUG_ON((_span)->storage_class == TERNFS_EMPTY_STORAGE || (_span)->storage_class == TERNFS_INLINE_STORAGE); \
        container_of(_span, struct ternfs_block_span, span); \
    })

// Fetches and caches the span if necessary. `offset` need not be the actual
// span offset, the span containing the offset will be returned with incremented refcount
//
// If the offset is out of bounds, NULL will be returned. Errors might
// be returned if we fail to fetch the span.
struct ternfs_span* ternfs_get_span(struct ternfs_fs_info* fs_info, struct ternfs_file_spans* spans, u64 offset);

// decrements refcount
void ternfs_put_span(struct ternfs_span* span);

// Remove the span from the inode tree and decrement the refcount.
void ternfs_unlink_span(struct ternfs_file_spans* spans, struct ternfs_span* span);

// Fetches pages from block services to fill the supplied list of pages.
// @pages - original list of pages to be filled. The list is expected to
//          contain pages with page->index values increasing by 1.
//          If file offsets in the supplied list fall beyond the end of
//          the supplied block_span, pages for those offsets are dropped
//          and removed from the list.
// @extra_pages - empty list to add fetched pages that fall outside of the
//                requested range.
// Pages are taken out of the list, added to required blocks for fetching,
// filled with data fetched from block services and then assembled back
// maintaining the original order to match file offsets. Each supplied page
// has page->index prefilled as file_offset/PAGE_SIZE. Fetches are done in
// multiple blocks, but only up to the end of the stripe. If more pages are
// needed, separate fetch for the next stripe is kicked off. Fetch is kicked
// off with the same amount of pages in each block, so if any blocks end up
// having fewer pages, additional padding pages are allocated. If any blocks
// fail, the fetch for the rest of the blocks in the stripe and required parity
// blocks is kicked off. These pages and any padding pages have correct
// page->index set to match the file offset and they are added to the extra_pages
// list. It is up to the caller how to handle them.
// If the requested range is larger than span end, the remaining pages are
// dropped with put_page and not filled. The upstream code only really cares that
// the first page is filled in by the readahead code, so returning fewer pages or
// even returning error aggressively is fine.
int ternfs_span_get_pages(struct ternfs_block_span* block_span, struct address_space* mapping, struct list_head *pages, unsigned nr_pages, struct list_head *extra_pages);

int __init ternfs_span_init(void);
void __cold ternfs_span_exit(void);

#endif
