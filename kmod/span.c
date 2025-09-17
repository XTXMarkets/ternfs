// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/module.h>
#include <linux/random.h>
#include <linux/err.h>
#include <linux/slab.h>

#include "bincode.h"
#include "block.h"
#include "inode.h"
#include "log.h"
#include "span.h"
#include "metadata.h"
#include "rs.h"
#include "latch.h"
#include "crc.h"
#include "err.h"
#include "counter.h"
#include "wq.h"
#include "trace.h"
#include "intrshims.h"
#include "sysctl.h"
#include "block_services.h"

static struct kmem_cache* ternfs_block_span_cachep;
static struct kmem_cache* ternfs_inline_span_cachep;
static struct kmem_cache* ternfs_fetch_span_pages_cachep;

#define DOWNLOADING_SHIFT 0
#define DOWNLOADING_MASK (0xFFFFull<<DOWNLOADING_SHIFT)
#define DOWNLOADING_GET(__b) ((__b>>DOWNLOADING_SHIFT) & 0xFFFFull)
#define DOWNLOADING_SET(__b, __i) (__b | 1ull<<(__i+DOWNLOADING_SHIFT))
#define DOWNLOADING_UNSET(__b, __i) (__b & ~(1ull<<(__i+DOWNLOADING_SHIFT)))

#define SUCCEEDED_SHIFT 16
#define SUCCEEDED_MASK (0xFFFFull<<SUCCEEDED_SHIFT)
#define SUCCEEDED_GET(__b) ((__b>>SUCCEEDED_SHIFT) & 0xFFFFull)
#define SUCCEEDED_SET(__b, __i) (__b | 1ull<<(__i+SUCCEEDED_SHIFT))
#define SUCCEEDED_UNSET(__b, __i) (__b & ~(1ull<<(__i+SUCCEEDED_SHIFT)))

#define FAILED_SHIFT 32
#define FAILED_MASK (0xFFFFull<<FAILED_SHIFT)
#define FAILED_GET(__b) ((__b>>FAILED_SHIFT) & 0xFFFFull)
#define FAILED_SET(__b, __i) (__b | 1ull<<(__i+FAILED_SHIFT))
#define FAILED_UNSET(__b, __i) (__b & ~(1ull<<(__i+FAILED_SHIFT)))

inline u32 ternfs_stripe_size(struct ternfs_block_span* span) {
    return span->cell_size * ternfs_data_blocks(span->parity);
}

void ternfs_init_file_spans(struct ternfs_file_spans* spans, u64 ino) {
    spans->__spans = RB_ROOT;
    init_rwsem(&spans->__lock);
    spans->__ino = ino;
}

struct fetch_span_pages_state {
     struct ternfs_block_span* block_span;
    // Used to wait on every block being finished. Could be done faster with a wait_queue, but don't want to
    // worry about smb subtleties.
    struct semaphore sema;
    struct address_space *mapping;
    u32 start_offset;
    u32 size;
    // will be set to 0 and D respectively when initiating fetch for RS recovery.
    atomic_t start_block;
    atomic_t end_block;
    // These will be filled in by the fetching (only D of them well be
    // filled by fetching the blocks, the others might be filled in by the RS
    // recovery)
    struct list_head blocks_pages[TERNFS_MAX_BLOCKS];
    atomic_t refcount;       // to garbage collect
    atomic_t err;            // the result of the stripe fetching
    atomic_t last_block_err; // to return something vaguely connected to what happened
    static_assert(TERNFS_MAX_BLOCKS <= 16);
    // 00-16: blocks which are downloading.
    // 16-32: blocks which have succeeded.
    // 32-48: blocks which have failed.
    atomic64_t blocks;
};

static void init_fetch_span_pages(void* p) {
    struct fetch_span_pages_state* st = (struct fetch_span_pages_state*)p;

    sema_init(&st->sema, 0);
    int i;
    for (i = 0; i < TERNFS_MAX_BLOCKS; i++) {
        INIT_LIST_HEAD(&st->blocks_pages[i]);
    }
}

static struct fetch_span_pages_state* new_fetch_span_pages_state(struct ternfs_block_span *block_span) {
    struct fetch_span_pages_state* st = (struct fetch_span_pages_state*)kmem_cache_alloc(ternfs_fetch_span_pages_cachep, GFP_KERNEL);
    if (!st) { return st; }

    st->block_span = block_span;
    atomic_set(&st->refcount, 1); // the caller
    atomic_set(&st->err, 0);
    atomic_set(&st->last_block_err, 0);
    atomic64_set(&st->blocks, 0);
    atomic_set(&st->start_block, 0);
    atomic_set(&st->end_block, 0);
    st->start_offset = 0;
    st->size = 0;

    return st;
}

inline u8 fetch_span_pages_state_stripe_ix(struct fetch_span_pages_state* st) {
    return st->start_offset/st->block_span->cell_size;
}

inline int fetch_span_pages_state_need_blocks(struct fetch_span_pages_state* st) {
    int D = ternfs_data_blocks(st->block_span->parity);
    return min(atomic_read(&st->end_block) - atomic_read(&st->start_block), D);
}

inline void fetch_span_pages_move_list(struct fetch_span_pages_state* st, int src, int dst) {
    BUG_ON(list_empty(&st->blocks_pages[src]));

    if (src == dst) return;

    BUG_ON(!list_empty(&st->blocks_pages[dst]));
    struct page* page;
    struct page* tmp;
    int num_pages = 0;
    list_for_each_entry_safe(page, tmp, &st->blocks_pages[src], lru) {
        list_del(&page->lru);
        list_add_tail(&page->lru, &st->blocks_pages[dst]);
        num_pages++;
    }
    BUG_ON(num_pages != st->size/PAGE_SIZE);
}

inline void fetch_span_pages_state_request_full_fetch(struct fetch_span_pages_state* st, int failed_block) {
        int D = ternfs_data_blocks(st->block_span->parity);
        // Keep the original pages with block 0 when D == 1.
        // They were transferred to the selected block for fetching and now
        // they need to be handed back to be passed to the next selected block.
        if (D == 1) fetch_span_pages_move_list(st, failed_block, 0);

        int B = ternfs_blocks(st->block_span->parity);
        atomic_set(&st->start_block, 0);
        atomic_set(&st->end_block, B);
}

static void free_fetch_span_pages(struct fetch_span_pages_state* st) {
    // we're the last ones here
    //fetch_stripe_trace(st, TERNFS_FETCH_STRIPE_FREE, -1, atomic_read(&st->err));

    while (down_trylock(&st->sema) == 0) {} // reset sema to zero for next usage

    // Free leftover blocks (also ensures the lists are all nice and empty for the
    // next user)
    int i;
    for (i = 0; i < TERNFS_MAX_BLOCKS; i++) {
        put_pages_list(&st->blocks_pages[i]);
    }
    kmem_cache_free(ternfs_fetch_span_pages_cachep, st);
}

static void put_fetch_span_pages(struct fetch_span_pages_state* st) {
    int remaining = atomic_dec_return(&st->refcount);
    ternfs_debug("st=%p remaining=%d", st, remaining);
    BUG_ON(remaining < 0);
    if (remaining > 0) { return; }
    free_fetch_span_pages(st);
}

static void hold_fetch_span_pages(struct fetch_span_pages_state* st) {
    int holding = atomic_inc_return(&st->refcount);
    ternfs_debug("st=%p holding=%d", st, holding)
    WARN_ON(holding < 2);
}

static void store_block_pages(struct fetch_span_pages_state* st) {
    struct ternfs_block_span* span = st->block_span;
    int D = ternfs_data_blocks(span->parity);
    int B = ternfs_blocks(span->parity);
    // During failure we kick off fetches for all D+P blocks,
    // but we return when we get at least D.
    int want_blocks = min(D, atomic_read(&st->end_block) - atomic_read(&st->start_block));
    int i;

    u32 pages_per_block = st->size/PAGE_SIZE;
    u16 blocks = SUCCEEDED_GET(atomic64_read(&st->blocks));
    u16 failed_blocks = FAILED_GET(atomic64_read(&st->blocks));
    u32 start = atomic_read(&st->start_block);
    u32 end = atomic_read(&st->end_block);

    if (failed_blocks > 0) {
        start = 0;
        end = B;
    }

    if (__builtin_popcountll(blocks) != want_blocks) {
        ternfs_warn("have %lld blocks, want %d, succeeded: %d, failed:%d, start:%d end:%d", __builtin_popcountll(blocks), want_blocks, blocks, failed_blocks, atomic_read(&st->start_block), atomic_read(&st->end_block));
        BUG();
    }
    if (unlikely(failed_blocks > 0) && D != 1) { // we need to recover data using RS
        for (i = 0; i < D; i++) { // fill in missing data blocks
            if ((1ull<<i) & blocks) { continue; } // we have this one already
            if (list_empty(&st->blocks_pages[i])) {
                ternfs_warn("list empty for recovery, block %d, start %d, end %d, blocks %d, failed %d", i, start, end, blocks, failed_blocks);
                BUG();
            }

            // compute
            ternfs_info("recovering block %d, stripe %d, D=%d, want_blocks=%d, blocks=%d, failed_blocks=%d", i, fetch_span_pages_state_stripe_ix(st), D, want_blocks, blocks, failed_blocks);
            int err = ternfs_recover(span->parity, blocks, 1ull<<i, pages_per_block, st->blocks_pages);
            if (err) {
                ternfs_warn("recovery for block %d, stripe %d failed. err=%d", i, fetch_span_pages_state_stripe_ix(st), err);
                atomic_set(&st->err, err);
            }
        }
    }
    if (D == 1) {
        for (i = 0; i < B; i++) {
            if ((1ull<<i) & blocks) break;
        }
        if (failed_blocks) ternfs_info("recovering block %d, stripe %d, D=%d, want_blocks=%d, blocks=%d, failed_blocks=%d", i, fetch_span_pages_state_stripe_ix(st), D, want_blocks, blocks, failed_blocks);
        // Copy pages over from any non 0 block regardless if it was selected
        // to spread the load or because any other block failed.
        fetch_span_pages_move_list(st, i, 0);
    }

    for(i = D; i < B; i++) {
        put_pages_list(&st->blocks_pages[i]);
    }
}

static void span_block_done(void* data, u64 block_id, struct list_head* pages, int err);

static int try_fill_from_page_cache(struct address_space *mapping, struct list_head* pages, unsigned long start_page_ix, unsigned int nr_pages) {
    int have_pages = 0;
    struct page* page;
    struct page* cached_page;
    list_for_each_entry(page, pages, lru) {
        cached_page = find_get_page(mapping, page->index);
        if (cached_page != NULL) {
            // from kernel 5.19 pages are added to page cache before calling readahead
            // since readahead does not need to read everything it's possible some pages
            // are in page cache but were never filled. 
            // we need to check if page is up to date before using it
            // otherwise we could reconstruct other pages using incorrect data 
            if (PageUptodate(cached_page)) {
                char* to_ptr = kmap_atomic(page);
                char* from_ptr = kmap_atomic(cached_page);
                memcpy(to_ptr, from_ptr, PAGE_SIZE);
                kunmap_atomic(to_ptr);
                kunmap_atomic(from_ptr);
                put_page(cached_page);
                break;
            }

            // page cache page is returned with an increased refcount
            put_page(cached_page);
        } else {
            break;
        }
        have_pages++;
    }
    if (have_pages == nr_pages) { return 1; }
    return 0;
}

#if (LINUX_VERSION_CODE >= KERNEL_VERSION(6, 1, 0))
static inline u32 prandom_u32(void)
{
	return get_random_u32();
}
#endif

// Starts loading up D blocks as necessary.
static int fetch_span_blocks(struct fetch_span_pages_state* st) {
    int i;

    struct ternfs_block_span* span = st->block_span;
    int D = ternfs_data_blocks(span->parity);
    int P = ternfs_parity_blocks(span->parity);
    int B = ternfs_blocks(span->parity);

#define LOG_STR "file=%016llx span=%llu start=%d size=%d D=%d P=%d downloading=%04x(%llu) succeeded=%04x(%llu) failed=%04x(%llu) last_block_err=%d"
#define LOG_ARGS span->span.ino, span->span.start, st->start_offset, st->size, D, P, downloading, __builtin_popcountll(downloading), succeeded, __builtin_popcountll(succeeded), failed, __builtin_popcountll(failed), atomic_read(&st->last_block_err)

    for (;;) {
        s64 blocks = atomic64_read(&st->blocks);
        u16 downloading = DOWNLOADING_GET(blocks);
        u16 succeeded = SUCCEEDED_GET(blocks);
        u16 failed = FAILED_GET(blocks);
        if (__builtin_popcountll(downloading)+__builtin_popcountll(succeeded) >= fetch_span_pages_state_need_blocks(st)) { // we managed to get out of the woods
            ternfs_debug("we have enough blocks, terminating " LOG_STR, LOG_ARGS);
            return 0;
        }
        if (__builtin_popcountll(failed) > P) { // nowhere to go from here but tears
            ternfs_info("we're out of blocks, giving up " LOG_STR, LOG_ARGS);
            int err = atomic_read(&st->last_block_err);
            BUG_ON(err == 0);
            return err;
        }

        // Find the next block to download. If we're mirrored, download
        // blocks at random, so that we can use mirrored files to avoid
        // hotspot for super busy files.
        if (D == 1) {
            int start = prandom_u32()%B;
            int j;
            for (j = start; j < start + B; j++) {
                i = j%B;
                if (!((1ull<<i) & (downloading|failed|succeeded))) { break; }
            }
        } else {
            for (i = atomic_read(&st->start_block); i < atomic_read(&st->end_block); i++) {
                if (!((1ull<<i) & (downloading|failed|succeeded))) { break; }
            }
        }
        BUG_ON(i == B); // something _must_ be there given the checks above

        ternfs_debug("next block to be fetched is %d " LOG_STR, i, LOG_ARGS);

        // try to set ourselves as selected
        if (atomic64_cmpxchg(&st->blocks, blocks, DOWNLOADING_SET(blocks, i)) != blocks) {
            ternfs_debug("skipping %d " LOG_STR, i, LOG_ARGS);
            // somebody else got here first (can happen if some blocks complete before this loop finishes) first, keep going
            continue;
        }

        // ok, we're responsible for this now, start
        struct ternfs_block* block = &span->blocks[i];
        loff_t curr_off = span->span.start + fetch_span_pages_state_stripe_ix(st)*ternfs_stripe_size(span) + i*span->cell_size + st->start_offset%span->cell_size;
        u16 failed_blocks = FAILED_GET(atomic64_read(&st->blocks));

        // We start with pages at index 0 when D=1.
        // If another block is selected above to spread the load, we need to
        // transfer pages to that block.
        if (D == 1) fetch_span_pages_move_list(st, 0, i);

        if(list_empty(&st->blocks_pages[i]) && failed_blocks == 0) {
            ternfs_warn("empty list at file=%016llx, block %d, stripe %d, offset=%d, size=%d, span_start=%lld, span_end=%lld. blocks=%lld, failed:%d", span->span.ino, i, fetch_span_pages_state_stripe_ix(st), st->start_offset, st->size, span->span.start, span->span.end, blocks, failed_blocks);
        }
        BUG_ON(list_empty(&st->blocks_pages[i]) && failed_blocks == 0);

        // we are likely kicking additional blocks due to failed block downloads
        if (list_empty(&st->blocks_pages[i])) {
            int j;
            for (j = 0; j < st->size/PAGE_SIZE; j++) {
                u32 page_ix = curr_off/PAGE_SIZE;
                struct page* page = __page_cache_alloc(readahead_gfp_mask(st->mapping));
                if (!page) {
                    ternfs_warn("couldn't alocate page at index %d", page_ix);
                    atomic_set(&st->last_block_err, -ENOMEM);
                    put_pages_list(&st->blocks_pages[i]);
                    return -ENOMEM;
                }
                if(curr_off < span->span.end) {
                    page->index = page_ix;
                    curr_off += PAGE_SIZE;
                } else {
                    // These pages will be dropped after RS recovery.
                    page->index = (unsigned long)-1;
                }
                page->private = 0;
                list_add_tail(&page->lru, &st->blocks_pages[i]);
                BUG_ON(atomic_read(&page->_refcount) < 1);
            }
            if (i < D && curr_off < span->span.end) {
                int have_pages = try_fill_from_page_cache(st->mapping, &st->blocks_pages[i], curr_off/PAGE_SIZE, st->size/PAGE_SIZE);
                if (have_pages == st->size/PAGE_SIZE && curr_off + st->size <= st->block_span->span.end) {
                    ternfs_info("found all pages in cache block %d, stripe %d", i, fetch_span_pages_state_stripe_ix(st));
                    while (unlikely(!atomic64_try_cmpxchg(&st->blocks, &blocks, SUCCEEDED_SET(DOWNLOADING_UNSET(blocks, i), i)))) {}
                    continue;
                }
            }
        }

        //fetch_stripe_trace(st, TERNFS_FETCH_STRIPE_BLOCK_START, i, 0);
        hold_fetch_span_pages(st);
        ternfs_debug("block start st=%p block_service=%016llx block_id=%016llx", st, block->id, block->id);
        struct ternfs_block_service bs;
        ternfs_get_block_service(block->bs, &bs);
        // Fetches a single cell from the block
        int block_err = ternfs_fetch_block_pages_with_crc(
            &span_block_done,
            (void*)st,
            &bs, &st->blocks_pages[i], span->span.ino, block->id, block->crc, st->start_offset, st->size
        );
        if (block_err) {
            BUG_ON(list_empty(&st->blocks_pages[i]));
            atomic_set(&st->last_block_err, block_err);
            put_fetch_span_pages(st);
            ternfs_info("loading block %d failed " LOG_STR, i, LOG_ARGS);

            // Downloading some block failed, all blocks + parity will be needed for RS recovery.
            fetch_span_pages_state_request_full_fetch(st, i);

            //fetch_stripe_trace(st, TERNFS_FETCH_STRIPE_BLOCK_DONE, i, block_err);
            // mark it as failed and not downloading
            blocks = atomic64_read(&st->blocks);
            while (unlikely(!atomic64_try_cmpxchg(&st->blocks, &blocks, FAILED_SET(DOWNLOADING_UNSET(blocks, i), i)))) {}
        } else {
            ternfs_debug("loading block %d " LOG_STR, i, LOG_ARGS);
        }
    }

#undef LOG_STR
#undef LOG_ARGS
}

static void span_block_done(void* data, u64 block_id, struct list_head* pages, int err) {
    int i;

    struct fetch_span_pages_state* st = (struct fetch_span_pages_state*)data;
    struct ternfs_block_span* span = st->block_span;

    int D = ternfs_data_blocks(span->parity);
    int P = ternfs_parity_blocks(span->parity);
    int B = ternfs_blocks(span->parity);
    u32 pages_per_block = st->size/PAGE_SIZE;

    for (i = 0; i < B; i++) {
        if (span->blocks[i].id == block_id) { break; }
    }

    BUG_ON(D != 1 && i == atomic_read(&st->end_block));

    ternfs_debug(
        "st=%p block=%d block_id=%016llx file=%016llx span=%llu offset=%d size=%d D=%d P=%d err=%d",
        st, i, block_id, span->span.ino, span->span.start, st->start_offset, st->size, D, P, err
    );

    //fetch_stripe_trace(st, TERNFS_FETCH_STRIPE_BLOCK_DONE, i, err);

    bool finished = false;
    // we reuse the supplied pages, move them back to reuse for fetching and
    // RS recovery if needed.
    BUG_ON(!list_empty(&st->blocks_pages[i]));
    BUG_ON(list_empty(pages));
    struct page* page;
    struct page* tmp;
    u32 fetched_pages = 0;
    list_for_each_entry_safe(page, tmp, pages, lru) {
        list_del(&page->lru);
        list_add_tail(&page->lru, &st->blocks_pages[i]);
        fetched_pages++;
    }
    BUG_ON(fetched_pages != pages_per_block);

retry:
    if (err) {
        s64 blocks = atomic64_read(&st->blocks);
        ternfs_info("fetching block %016llx (index:%d, D=%d, P=%d, blocks:%lld) in file %016llx failed: %d", block_id, i, D, P, blocks, span->span.ino, err);
        atomic_set(&st->last_block_err, err);
        // it failed, try to restore order
        // mark it as failed and not downloading
        while (unlikely(!atomic64_try_cmpxchg(&st->blocks, &blocks, FAILED_SET(DOWNLOADING_UNSET(blocks, i), i)))) {}

        // Kick off downloading all blocks + parity for RS recovery.
        fetch_span_pages_state_request_full_fetch(st, i);
        err = fetch_span_blocks(st);
        // If we failed to restore order, mark the whole thing as failed,
        // and if we were the first ones to notice the error, finish
        // the overall thing. Other failing requests will just drop the
        // reference.
        if (err) {
            int old_err = 0;
            finished = atomic_try_cmpxchg(&st->err, &old_err, err);
            ternfs_debug("failed finished=%d", finished);
        }
    } else {
        // mark as fetched, see if we're the last ones
        s64 blocks = atomic64_read(&st->blocks);

        u32 block_offset = 0;
        list_for_each_entry(page, &st->blocks_pages[i], lru) {
            char* page_buf = kmap_atomic(page);
            kernel_fpu_begin();
            u32 crc = ternfs_crc32c(0, page_buf, PAGE_SIZE);
            kernel_fpu_end();
            kunmap_atomic(page_buf);
            if (crc != (u32)page->private) {
                err = TERNFS_ERR_BAD_BLOCK_CRC;
                ternfs_warn("incorrect crc %d (expected %d) for %lld, block %d, offset %d, will try different block", crc, (u32)page->private, span->span.ino, i, block_offset);
                // mark this one as failed and kick off parity blocks for recovery.
                goto retry;
            }
            block_offset += PAGE_SIZE;
        }
        while (unlikely(!atomic64_try_cmpxchg(&st->blocks, &blocks, SUCCEEDED_SET(DOWNLOADING_UNSET(blocks, i), i)))) {}
        finished = __builtin_popcountll(SUCCEEDED_GET(SUCCEEDED_SET(blocks, i))) == fetch_span_pages_state_need_blocks(st); // blocks = last old one, we need to reapply
    }

    if (finished) {
        ternfs_debug("finished");
        // we're the last one to finish, we need to store the pages in the span,
        // if we have no errors
        err = atomic_read(&st->err);
        if (err == 0) { store_block_pages(st); }
        //fetch_stripe_trace(st, TERNFS_FETCH_STRIPE_END, -1, err);
        // Wake up waiters. The trace above needs to happen before because it
        // references the stripe. If the caller is allowed to continue,
        // the stripe may get reclaimed and it results in null pointer deref.
        up(&st->sema);
    }

    // drop our reference
    put_fetch_span_pages(st);
}

int ternfs_span_get_pages(struct ternfs_block_span* block_span, struct address_space * mapping, struct list_head *pages, unsigned nr_pages, struct list_head *extra_pages) {
    int err;

    if (list_empty(pages)) {
        ternfs_warn("called with empty list of pages for file %016llx", block_span->span.ino);
        return -EIO;
    }

    loff_t off_start = page_offset(lru_to_page(pages));
    unsigned long page_ix;
    unsigned long first_page_index = page_index(lru_to_page(pages));
    // Work out start and end index for each block we need to fetch.
    loff_t curr_off = off_start;
    u64 span_offset = off_start - block_span->span.start;
    u32 stripe_size = ternfs_stripe_size(block_span);

    int D = ternfs_data_blocks(block_span->parity);
    u64 span_physical_end = block_span->span.start + (u64)stripe_size*block_span->num_stripes;
    // span physical end and off_start are always at PAGE_SIZE boundary
    unsigned span_physical_pages = (span_physical_end - off_start)/PAGE_SIZE;
    unsigned span_logical_pages = (block_span->span.end - off_start + PAGE_SIZE -1)/PAGE_SIZE;
    unsigned available_span_pages = min(span_physical_pages, span_logical_pages);
    unsigned span_zero_pages = span_logical_pages - available_span_pages;
    unsigned span_pages = min(nr_pages, available_span_pages);

    // Pages that are used to store zeroes at the end of the file.
    // They are not stored in the block service and are filled by the client.
    LIST_HEAD(zero_pages);
    unsigned num_zero_pages = min(nr_pages - span_pages, span_zero_pages);
    if (num_zero_pages > 0)
        ternfs_info("zero_pages:%d, num_pages:%d, span_pages:%d, span->start=%lld, span->end=%lld, stripe_size=%d, num_stripes=%d, diff=%lld", num_zero_pages, nr_pages, span_pages, block_span->span.start, block_span->span.end, stripe_size, block_span->num_stripes, block_span->span.end - span_physical_end);

    unsigned remaining_pages = span_pages;
    struct fetch_span_pages_state *fetches[TERNFS_MAX_STRIPES];
    memset(fetches, 0, TERNFS_MAX_STRIPES*sizeof(struct fetch_span_pages_state *));

    u32 curr_block_ix;
    int i;

    u32 stripe_ix = (curr_off - block_span->span.start) / stripe_size;
    u64 stripe_offset = (span_offset - stripe_ix*ternfs_stripe_size(block_span));
    u32 start_block_offset = stripe_offset%block_span->cell_size + stripe_ix * block_span->cell_size;

    u32 next_block_offset = (u32)-1;


    for(;;) {
        BUG_ON(curr_off > block_span->span.end);

        struct fetch_span_pages_state *st = new_fetch_span_pages_state(block_span);
        if (IS_ERR(st)) {
            err = PTR_ERR(st);
            st = NULL;
            goto out;
        }
        stripe_ix = (curr_off - block_span->span.start) / stripe_size;
        BUG_ON(stripe_ix >= block_span->num_stripes);
        st->mapping = mapping;

        u32 start_block_ix = stripe_offset/block_span->cell_size;
        u32 last_block_ix = start_block_ix;
        u32 page_count;

        ternfs_debug("stripe_ix=%d, st=%p, off_start=%lld, curr_off=%lld, span_start=%lld, stripe_size=%d", stripe_ix, st, off_start, curr_off, block_span->span.start, stripe_size);
        for (curr_block_ix = start_block_ix; curr_block_ix < D && remaining_pages > 0; curr_block_ix++) {
            // Only the first iteration needs to use the block offset as calculated from the request.
            // The rest of the blocks in the stripe will start from 0.
            if (next_block_offset == (u32)-1) {
                next_block_offset = start_block_offset;
            } else {
                next_block_offset = block_span->cell_size*stripe_ix;
            }
            st->start_offset = next_block_offset;

            u32 remaining_cell = min((u32)(block_span->span.end - block_span->span.start - stripe_ix*ternfs_stripe_size(block_span) - curr_block_ix*block_span->cell_size), block_span->cell_size);
            page_count = min(remaining_pages, (unsigned)((remaining_cell - next_block_offset%block_span->cell_size + PAGE_SIZE - 1)/PAGE_SIZE));
            BUG_ON(page_count == 0);
            remaining_pages -= page_count;

            for(i = 0; i < page_count; i++) {
                page_ix = curr_off/PAGE_SIZE;
                struct page *page = lru_to_page(pages);
                if (page != NULL) {
                    if(page->index != page_ix) {
                        ternfs_warn("adding stripe_ix %d, block_ix=%d page_ix: have %ld, want %ld, curr_off=%lld, block_off=%d", stripe_ix, curr_block_ix, page->index, page_ix, curr_off, next_block_offset);
                        BUG();
                    }
                    list_del(&page->lru);
                } else {
                    page = __page_cache_alloc(readahead_gfp_mask(mapping));
                    if (!page) {
                        err = -ENOMEM;
                        goto out_pages;
                    }
                    page->index = page_ix;
                }
                list_add_tail(&page->lru, &st->blocks_pages[curr_block_ix]);
                BUG_ON(atomic_read(&page->_refcount) < 1);
                curr_off += PAGE_SIZE;
            }

            if(remaining_pages == 0) {
                // The blockservice itself will be padded to the end,
                // we need to fetch (some of) those pages to match other blocks.
                u32 curr_size = page_count * PAGE_SIZE;
                while(curr_size < st->size) {
                    struct page* page = __page_cache_alloc(readahead_gfp_mask(mapping));
                    if (!page) {
                        err = -ENOMEM;
                        goto out_pages;
                    }
                    if(curr_off < block_span->span.end) {
                        // This is a valid page that we want to add in page cache later
                        page->index = curr_off/PAGE_SIZE;
                        curr_off += PAGE_SIZE;
                    } else {
                        // padding page that we will want to discard.
                        page->index = (unsigned long)-1;
                    }
                    curr_size += PAGE_SIZE;
                    list_add_tail(&page->lru, &st->blocks_pages[curr_block_ix]);
                    page_count++;
                    BUG_ON(atomic_read(&page->_refcount) < 1);
                }
            }
            // add extra pages in the very first block to match other blocks.
            if (curr_block_ix == start_block_ix && stripe_ix == ((off_start - block_span->span.start) / stripe_size) && remaining_pages > 0) {
                u64 tmp_off_start = off_start;
                for(i = stripe_offset%block_span->cell_size/PAGE_SIZE; i > 0; i--) {
                    tmp_off_start -= PAGE_SIZE;
                    st->start_offset -= PAGE_SIZE;
                    page_ix = tmp_off_start/PAGE_SIZE;
                    struct page* page = __page_cache_alloc(readahead_gfp_mask(mapping));
                    if (!page) {
                        err = -ENOMEM;
                        goto out_pages;
                    }
                    page->index = page_ix;
                    ternfs_debug("inserting page %ld before first. pos=%lld, span->start=%lld sbo:%d, stripe_ix=%d", page->index, tmp_off_start, block_span->span.start, start_block_offset, stripe_ix);
                    list_add(&page->lru, &st->blocks_pages[curr_block_ix]);
                    page_count++;
                    BUG_ON(atomic_read(&page->_refcount) < 1);
                }
            }
            BUG_ON(remaining_pages < 0);
            if (st->size == 0) {
                st->size = page_count * PAGE_SIZE;
            } else {
                if (st->size != page_count * PAGE_SIZE) {
                    ternfs_warn("read_size: %d, new: %ld", st->size, page_count * PAGE_SIZE);
                    BUG();
                }
            }
            BUG_ON(st->size > block_span->cell_size);
            if (curr_block_ix > last_block_ix) last_block_ix = curr_block_ix;
            BUG_ON(list_empty(&st->blocks_pages[curr_block_ix]));
        }
        stripe_offset = 0;

        BUG_ON(st->size == 0);
        atomic_set(&st->start_block, start_block_ix);
        atomic_set(&st->end_block, last_block_ix + 1);

        // fetch blocks from start_block_ix to last_block_ix - these will be our blocks we need when assembling the pages back, the rest will go to extra_pages.
        err = fetch_span_blocks(st);
        if (err) { goto out; }
        fetches[stripe_ix] = st;
        // wait for the fetch to complete before kicking off next stripe
        down(&st->sema);
        if(remaining_pages == 0) {
            break;
        }

    }

    for (i = 0; i < num_zero_pages; i++) {
        struct page* page = lru_to_page(pages);
        BUG_ON(page == NULL);
        list_del(&page->lru);
        char* dst = kmap_atomic(page);
        memset(dst, 0, PAGE_SIZE);
        kunmap_atomic(dst);
        list_add_tail(&page->lru, &zero_pages);
    }
    // if span_pages < nr_pages, some pages at the end may be left not filled.
    // It is ok to just drop them as they will be requested in the next readahead request.
    put_pages_list(pages);

    // collect pages

    remaining_pages = span_pages;

    stripe_ix = (off_start - block_span->span.start) / stripe_size;
    stripe_offset = (span_offset - stripe_ix*ternfs_stripe_size(block_span));

    // starting offset is the start of the first block we ended up fetching.
    curr_off = block_span->span.start + stripe_ix*ternfs_stripe_size(block_span) + atomic_read(&fetches[stripe_ix]->start_block)*block_span->cell_size + fetches[stripe_ix]->start_offset%block_span->cell_size;

    for(;;) {
        stripe_ix = (curr_off - block_span->span.start) / stripe_size;
        BUG_ON(stripe_ix >= block_span->num_stripes);
        struct fetch_span_pages_state *st = fetches[stripe_ix];
        err = atomic_read(&st->err);
        if (err) goto out;

        BUG_ON(st == NULL);
        BUG_ON(stripe_ix != fetch_span_pages_state_stripe_ix(st));
        BUG_ON(curr_off > block_span->span.end);

        for(curr_block_ix = atomic_read(&st->start_block); curr_block_ix < atomic_read(&st->end_block) && remaining_pages > 0 && curr_block_ix < D; curr_block_ix++) {
            curr_off = block_span->span.start + stripe_ix*ternfs_stripe_size(block_span) + curr_block_ix*block_span->cell_size + st->start_offset%block_span->cell_size;
            ternfs_debug("collecting: block_ix=%d, stripe_ix=%d, read_size=%d, remaining_pages=%d, start_offset=%d, span_end:%lld", curr_block_ix, stripe_ix, st->size, remaining_pages, st->start_offset, block_span->span.end);

            u32 curr_size = 0;
            while(curr_size < st->size) {
                // work out if it needs to be added back to the main list
                if(curr_off < block_span->span.end) {
                    // This is a valid page that we want to add in page cache later
                    page_ix = curr_off/PAGE_SIZE;
                    curr_off += PAGE_SIZE;
                } else {
                    // padding page that we will want to discard.
                    page_ix = (unsigned long)-1;
                }


                struct page *page = list_first_entry_or_null(&st->blocks_pages[curr_block_ix], struct page, lru);
                BUG_ON(page == NULL);
                if (page->index != page_ix) {
                    ternfs_debug("first_page_index:%ld start_block=%d, end_block=%d, D=%d, stripe_offset=%lld, span_offset=%lld", first_page_index, atomic_read(&st->start_block), atomic_read(&st->end_block), D, stripe_offset, span_offset);
                    ternfs_info("ino:%016llx, curr_block_ix=%d, read_size=%d, curr_size=%d", block_span->span.ino, curr_block_ix, st->size, curr_size);
                    ternfs_warn("block_ix:%d curr_off:%lld want:%ld have:%ld remaining:%d", curr_block_ix, curr_off, page->index, page_ix, remaining_pages);
                    BUG();
                }
                list_del(&page->lru);
                page->private = 0;
                if(page->index == (unsigned long) -1) {
                    put_page(page);
                } else if(page->index >= first_page_index && remaining_pages > 0) {
                    list_add_tail(&page->lru, pages);
                    // Only count the pages we were asked to fetch.
                    remaining_pages--;
                } else {
                    list_add_tail(&page->lru, extra_pages);
                }
                curr_size += PAGE_SIZE;
            }
            // We go through all the requested pages, but there may be some
            // added to align fetch ranges for different blocks. At the end
            // all blocks falling in and out of requested range need to
            // balance. This is one of the checks to make sure they do.
            BUG_ON(remaining_pages < 0);
            if(!list_empty(&st->blocks_pages[curr_block_ix])) {
                ternfs_warn("list not empty at block:%d, stripe_ix:%d, remaining=%d", curr_block_ix, stripe_ix, remaining_pages);
                BUG();
            }
        }
        stripe_offset = 0;
        start_block_offset = 0;

        if (remaining_pages == 0) {
            break;
        }
    }

    for (i = 0; i < num_zero_pages; i++) {
        struct page *page = lru_to_page(&zero_pages);
        BUG_ON(page == NULL);
        list_del(&page->lru);
        list_add_tail(&page->lru, pages);
    }

out:
    for (i = 0; i < block_span->num_stripes; i++) {
        if (fetches[i] != NULL) put_fetch_span_pages(fetches[i]);
    }
    return err;
out_pages:
    put_pages_list(pages);
    goto out;
}

//
// SPANS PART
//

static struct ternfs_span* lookup_span(struct rb_root* spans, u64 offset) {
    struct rb_node* node = spans->rb_node;
    while (node) {
        struct ternfs_span* span = container_of(node, struct ternfs_span, node);
        if (offset < span->start) { node = node->rb_left; }
        else if (offset >= span->end) { node = node->rb_right; }
        else {
            ternfs_debug("off=%llu span=%p", offset, span);
            return span;
        }
    }
    ternfs_debug("off=%llu no_span", offset);
    return NULL;
}

static bool insert_span(struct rb_root* spans, struct ternfs_span* span) {
    struct rb_node** new = &(spans->rb_node);
    struct rb_node* parent = NULL;
    while (*new) {
        struct ternfs_span* this = container_of(*new, struct ternfs_span, node);
        parent = *new;
        if (span->end <= this->start) { new = &((*new)->rb_left); }
        else if (span->start >= this->end) { new = &((*new)->rb_right); }
        else {
            // TODO: be loud if there are overlaping spans
            ternfs_debug("span=%p (%llu-%llu) already present", span, span->start, span->end);
            return false;
        }
    }

    rb_link_node(&span->node, parent, new);
    rb_insert_color(&span->node, spans);
    ternfs_debug("span=%p (%llu-%llu) inserted", span, span->start, span->end);
    return true;
}

void ternfs_put_span(struct ternfs_span* span) {
    if (atomic_dec_return(&span->refcount) > 0) { return; }
    if (span->storage_class == TERNFS_INLINE_STORAGE) {
        kmem_cache_free(ternfs_inline_span_cachep, TERNFS_INLINE_SPAN(span));
    } else {
        kmem_cache_free(ternfs_block_span_cachep, TERNFS_BLOCK_SPAN(span));
    }
}

struct get_span_ctx {
    u64 ino;
    struct rb_root spans;
    int err;
};

static void file_spans_cb_span(void* data, u64 offset, u32 size, u32 crc, u8 storage_class, u8 parity, u8 stripes, u32 cell_size) {
    ternfs_debug("offset=%llu size=%u crc=%08x storage_class=%d parity=%d stripes=%d cell_size=%u", offset, size, crc, storage_class, parity, stripes, cell_size);

    struct get_span_ctx* ctx = (struct get_span_ctx*)data;
    if (ctx->err) { return; }
    if (offset % PAGE_SIZE) {
        ternfs_warn("span start is not a multiple of page size %llu", offset);
        ctx->err = -EIO;
        return;
    }
    if (cell_size % PAGE_SIZE) {
        ternfs_warn("cell size not multiple of page size %u", cell_size);
        ctx->err = -EIO;
        return;
    }

    struct ternfs_block_span* span = kmem_cache_alloc(ternfs_block_span_cachep, GFP_KERNEL);
    if (!span) { ctx->err = -ENOMEM; return; }
    atomic_set(&span->span.refcount, 1);

    if (ternfs_data_blocks(parity) > TERNFS_MAX_DATA || ternfs_parity_blocks(parity) > TERNFS_MAX_PARITY) {
        ternfs_warn("D=%d > %d || P=%d > %d", ternfs_data_blocks(parity), TERNFS_MAX_DATA, ternfs_data_blocks(parity), TERNFS_MAX_PARITY);
        ctx->err = -EIO;
        ternfs_put_span(&span->span);
        return;
    }

    span->span.ino = ctx->ino;
    span->span.start = offset;
    span->span.end = offset + size;
    span->span.storage_class = storage_class;

    span->cell_size = cell_size;
    span->num_stripes = stripes;
    span->parity = parity;
    ternfs_debug("adding normal span");
    insert_span(&ctx->spans, &span->span);
}

static void file_spans_cb_block(
    void* data, int block_ix,
    u64 bs_id, u32 ip1, u16 port1, u32 ip2, u16 port2, u8 flags,
    u64 block_id, u32 crc
) {
    struct get_span_ctx* ctx = (struct get_span_ctx*)data;
    if (ctx->err) { return; }

    struct ternfs_span* span = rb_entry(rb_last(&ctx->spans), struct ternfs_span, node);
    struct ternfs_block_span* block_span = TERNFS_BLOCK_SPAN(span);

    struct ternfs_block* block = &block_span->blocks[block_ix];
    block->id = block_id;
    block->crc = crc;

    // Populate bs cache
    struct ternfs_block_service bs;
    bs.id = bs_id;
    bs.ip1 = ip1;
    bs.port1 = port1;
    bs.ip2 = ip2;
    bs.port2 = port2;
    bs.flags = flags;
    block->bs = ternfs_upsert_block_service(&bs);
    if (IS_ERR(block->bs)) {
        ctx->err = PTR_ERR(block->bs);
        return;
    }
}

static void file_spans_cb_inline_span(void* data, u64 offset, u32 size, u8 len, const char* body) {
    ternfs_debug("offset=%llu size=%u len=%u body=%*pE", offset, size, len, len, body);

    struct get_span_ctx* ctx = (struct get_span_ctx*)data;
    if (ctx->err) { return; }

    if (offset % PAGE_SIZE) {
        ternfs_warn("span start is not a multiple of page size %llu", offset);
        ctx->err = -EIO;
        return;
    }

    struct ternfs_inline_span* span = kmem_cache_alloc(ternfs_inline_span_cachep, GFP_KERNEL);
    if (!span) { ctx->err = -ENOMEM; return; }
    atomic_set(&span->span.refcount, 1);

    span->span.ino = ctx->ino;

    span->span.start = offset;
    span->span.end = offset + size;
    span->span.storage_class = TERNFS_INLINE_STORAGE;
    span->len = len;
    memcpy(span->body, body, len);

    ternfs_debug("adding inline span");

    insert_span(&ctx->spans, &span->span);
}

struct ternfs_span* ternfs_get_span(struct ternfs_fs_info* fs_info, struct ternfs_file_spans* spans, u64 offset) {
    int err;

    ternfs_debug("ino=%016llx, pid=%d, off=%llu getting span", spans->__ino, get_current()->pid, offset);

    trace_eggsfs_get_span_enter(spans->__ino, offset);

#define GET_SPAN_EXIT(s) do { \
        trace_eggsfs_get_span_exit(spans->__ino, offset, IS_ERR(s) ? PTR_ERR(s) : 0); \
        return s; \
    } while(0)

retry:
    // Check if we already have the span
    {
        down_read(&spans->__lock);
        struct ternfs_span* span = lookup_span(&spans->__spans, offset);
        // we need to grab refcount while holding the lock
        if (likely(span != NULL)) {
            atomic_inc(&span->refcount);
        }
        up_read(&spans->__lock);
        if (likely(span != NULL)) {
            return span;
        }
        // While it looks it must be there it could be again removed already
        // BUG_ON(fetched); // If we've just fetched it must be here.
    }

    // We need to fetch the spans.
    down_write(&spans->__lock);

    u64 next_offset;
    struct get_span_ctx ctx = { .err = 0, .ino = spans->__ino };
    ctx.spans = RB_ROOT;
    err = ternfs_error_to_linux(ternfs_shard_file_spans(
        fs_info, spans->__ino, offset, &next_offset,
        file_spans_cb_inline_span,
        file_spans_cb_span,
        file_spans_cb_block,
        &ctx
    ));
    err = err ?: ctx.err;
    if (unlikely(err)) {
        ternfs_debug("failed to get file spans at %llu err=%d", offset, err);
        for (;;) { // free all the spans we just got
            struct rb_node* node = rb_first(&ctx.spans);
            if (node == NULL) { break; }
            rb_erase(node, &ctx.spans);
            ternfs_put_span(rb_entry(node, struct ternfs_span, node));
        }
        up_write(&spans->__lock);
        GET_SPAN_EXIT(ERR_PTR(err));
    }
    // add them to enode spans
    for (;;) {
        struct rb_node* node = rb_first(&ctx.spans);
        if (node == NULL) { break; }
        rb_erase(node, &ctx.spans);
        struct ternfs_span* span = rb_entry(node, struct ternfs_span, node);
        if (!insert_span(&spans->__spans, span)) {
            // Span is already cached
            ternfs_put_span(span);
        }
    }
    up_write(&spans->__lock);

    goto retry;

#undef GET_SPAN_EXIT
}

void ternfs_unlink_span(struct ternfs_file_spans* spans, struct ternfs_span* span) {
    down_write(&spans->__lock);
    rb_erase(&span->node, &spans->__spans);
    ternfs_put_span(span);
    up_write(&spans->__lock);
}

void ternfs_free_file_spans(struct ternfs_file_spans* spans) {
    down_write(&spans->__lock);

    for (;;) {
        struct rb_node* node = rb_first(&spans->__spans);
        if (node == NULL) { break; }

        struct ternfs_span* span = rb_entry(node, struct ternfs_span, node);
        rb_erase(&span->node, &spans->__spans);
        ternfs_put_span(span);
    }

    up_write(&spans->__lock);
}

//
// INIT PART
//

int ternfs_span_init(void) {
    int err;
    ternfs_block_span_cachep = kmem_cache_create(
        "ternfs_block_span_cache",
        sizeof(struct ternfs_block_span),
        0,
        SLAB_RECLAIM_ACCOUNT | SLAB_MEM_SPREAD,
        NULL
    );
    if (!ternfs_block_span_cachep) {
        err = -ENOMEM;
        return err;
    }

    ternfs_inline_span_cachep = kmem_cache_create_usercopy(
        "ternfs_inline_span_cache",
        sizeof(struct ternfs_inline_span),
        0,
        SLAB_RECLAIM_ACCOUNT | SLAB_MEM_SPREAD,
        offsetof(struct ternfs_inline_span, body),
        sizeof(((struct ternfs_inline_span*)NULL)->body),
        NULL
    );
    if (!ternfs_block_span_cachep) {
        err = -ENOMEM;
        goto out_block;
    }
    ternfs_fetch_span_pages_cachep = kmem_cache_create(
        "ternfs_fetch_span_pages_cache",
        sizeof(struct fetch_span_pages_state),
        0,
        SLAB_RECLAIM_ACCOUNT | SLAB_MEM_SPREAD,
        &init_fetch_span_pages
    );
    if (!ternfs_fetch_span_pages_cachep) {
        err = -ENOMEM;
        goto out_inline;
    }

    return 0;
out_inline:
    kmem_cache_destroy(ternfs_inline_span_cachep);
out_block:
    kmem_cache_destroy(ternfs_block_span_cachep);
    return err;
}

void ternfs_span_exit(void) {
    ternfs_debug("span exit");
    kmem_cache_destroy(ternfs_fetch_span_pages_cachep);
    kmem_cache_destroy(ternfs_inline_span_cachep);
    kmem_cache_destroy(ternfs_block_span_cachep);
}
