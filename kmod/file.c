// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "file.h"

#include <linux/uio.h>
#include <linux/sched/mm.h>
#include <linux/mm.h>
#include <linux/pagemap.h>

#include "bincode.h"
#include "inode.h"
#include "log.h"
#include "metadata.h"
#include "err.h"
#include "rs.h"
#include "block.h"
#include "crc.h"
#include "trace.h"
#include "span.h"
#include "wq.h"
#include "bincode.h"
#include "dir.h"
#include "inode_compat.h"

unsigned ternfs_atime_update_interval_sec = 0;

unsigned ternfs_file_io_timeout_sec = 86400;

unsigned ternfs_file_io_retry_refresh_span_interval_sec = 60; // 1 minute interval for refreshing spans during IO retries

unsigned ternfs_max_write_span_attempts = 5;

static struct kmem_cache* ternfs_transient_span_cachep;

struct ternfs_transient_span {
    // The transient span holds a reference to this. As long as the
    // transient span lives, the enode must live.
    struct ternfs_inode* enode;
    // Offset in the file for this span
    u64 offset;
    // Linear list of pages with the body of the span, used when we're still gathering
    // content for it. We use `index` in the page to track where we're writing to.
    struct list_head pages;
    u32 written; // how much we've written to this span
    atomic_t refcount;

    // These are finalized when we start flushing out a span.
    char failure_domains[TERNFS_MAX_BLOCKS][16]; // failure domains for current run so we can blacklist failures
    char blacklisted_failure_domains[TERNFS_MAX_BLACKLIST_LENGTH][16]; // blacklisted failure domains
    struct list_head blocks[TERNFS_MAX_BLOCKS]; // the pages for each block
    u64 block_ids[TERNFS_MAX_BLOCKS]; // the block ids assigned by add span initiate
    spinlock_t lock; // we use this for various modifications
    u64 blocks_proofs[TERNFS_MAX_BLOCKS]; // when completing blocks, one of proof or err is set.
    int blocks_errs[TERNFS_MAX_BLOCKS];
    u32 span_crc;
    u32 cell_crcs[TERNFS_MAX_BLOCKS*TERNFS_MAX_STRIPES];
    u32 block_size;
    u8 attempts;
    u8 stripes;
    u8 storage_class;
    u8 parity;
    u8 blacklist_length;
    bool started_flushing;
};
// just a sanity check, this is just two per page rn
static_assert(sizeof(struct ternfs_transient_span) < (2<<10));

// open_mutex held here
// really want atomic open for this
static int file_open(struct inode* inode, struct file* filp) {
    inode_lock(inode); // for the .status modification below

    struct ternfs_inode* enode = TERNFS_I(inode);

    ternfs_debug("enode=%p status=%d owner=%p", enode, enode->file.status, current->group_leader);
    int err = 0;

    if ((filp->f_mode&FMODE_WRITE) && (enode->file.status == TERNFS_FILE_STATUS_WRITING)) {
        // this is the "common" writing case, we've just created a file to write it.
        // note that we never change the file owner, which might lead to confusing behavior
        // but is probably the only sensible thing to do.
    } else {
        // otherwise, the file must be already there, we're very relaxed in what we allow
        // in f_mode here and we just fail when operations that can't be done (e.g. writing
        // to files) are attempted. the reason is that some workflows (such as open write +
        // setattr) _will_ work.
        enode->file.status = TERNFS_FILE_STATUS_READING;
        // also, set atime, if requested
        if (!(filp->f_flags&O_NOATIME)) {
            u64 atime_ns = ktime_get_real_ns();
            struct timespec64 atime_ts = ns_to_timespec64(atime_ns);
            u64 diff = atime_ts.tv_sec - min(inode_get_atime_sec(&enode->inode), atime_ts.tv_sec);
            if (diff < ternfs_atime_update_interval_sec) {
                // we don't think we should update
                goto out;
            }

            // https://internal-repo/issues/292
            // we might have cached data and another client updated atime.
            // ternfs_do_getattr is orders of magnitude cheaper than ternfs_shard_set_time,
            // so we might as well refresh and re-check
            int err = ternfs_do_getattr(enode, ATTR_CACHE_NO_TIMEOUT);
            if (err) {
                inode_unlock(inode);
                goto out;
            }
            diff = atime_ts.tv_sec - min(inode_get_atime_sec(&enode->inode), atime_ts.tv_sec);
            if (diff < ternfs_atime_update_interval_sec) {
                // out local time changed and we see we don't need to update
                goto out;
            }

            if ((inode_get_atime_sec(&enode->inode) > atime_ts.tv_sec) ||
                (inode_get_atime_sec(&enode->inode) == atime_ts.tv_sec &&
                 inode_get_atime_nsec(&enode->inode) == atime_ts.tv_nsec
                )
            ) {
                // we don't want atime to go into the past don't update
                goto out;
            }
            u64 atime = atime_ns | (1ull<<63);
            err = ternfs_shard_set_time((struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info, inode->i_ino, 0, atime);
            if (err) {
                goto out;
            }
            // we updated time. we don't need to refresh it now but allow refresh on next stat by getattr_expiry
            smp_store_release(&enode->getattr_expiry, 0);
        }
    }
out:
    inode_unlock(inode);
    return err;
}

static void init_transient_span(void* p) {
    struct ternfs_transient_span* span = (struct ternfs_transient_span*)p;
    INIT_LIST_HEAD(&span->pages);
    int i;
    for (i = 0; i < TERNFS_MAX_BLOCKS; i++) {
        INIT_LIST_HEAD(&span->blocks[i]);
    }
    // we set it to 0, we bug if not 0 when getting from cache
    atomic_set(&span->refcount, 0);
    spin_lock_init(&span->lock);
}

// starts out with refcount = 1
static struct ternfs_transient_span* new_transient_span(struct ternfs_inode* enode, u64 offset) {
    struct ternfs_transient_span* span = kmem_cache_alloc(ternfs_transient_span_cachep, GFP_KERNEL);
    if (span == NULL) { return span; }
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
    // We set the parity to zero here so that `put_transient_span`
    // is always safe to run, since it traverses the blocks to free
    // the pages, if any.
    span->parity = 0;
    span->blacklist_length = 0;
    span->started_flushing = false;
    return span;
}

static void hold_transient_span(struct ternfs_transient_span* span) {
    atomic_inc(&span->refcount);
}

static bool put_transient_span(struct ternfs_transient_span* span) {
    if (atomic_dec_return(&span->refcount) == 0) {
        BUG_ON(spin_is_locked(&span->lock));
        // free pages, adjust OOM score
        int num_pages = 0;
        // We are not necessarily holding the last reference to a page here.
        // We could have errored out before fully sending out the request.
        // In which case network stack could still hold a reference to it.
        // It is however fine to adjust the OOM score here as technically
        // the file system is now responsible for this memory and it's no
        // longer tied to the process lifetime.
#define FREE_PAGES(__pages) \
        while (!list_empty(__pages)) { \
            struct page* victim = lru_to_page(__pages); \
            list_del(&victim->lru); \
            put_page(victim); \
            num_pages++; \
        }
        FREE_PAGES(&span->pages);
        int b;
        for (b = 0; b < ternfs_blocks(span->parity); b++) {
            FREE_PAGES(&span->blocks[b]);
        }
#undef FREE_PAGES
#if (LINUX_VERSION_CODE < KERNEL_VERSION(6,2,0))
        atomic_long_add_return(-num_pages, &span->enode->file.mm->rss_stat.count[MM_FILEPAGES]);
#else
        percpu_counter_add(&span->enode->file.mm->rss_stat[MM_FILEPAGES], -num_pages);   
#endif 
        if (span->started_flushing) {
            up(&span->enode->file.flushing_span_sema);
        }
        // Free the span itself
        kmem_cache_free(ternfs_transient_span_cachep, span);
        return true;
    }
    return false;
}

static int add_span_initiate(struct ternfs_transient_span* span);

static int move_span_and_restart(struct ternfs_transient_span* span) {
    struct ternfs_inode* enode = span->enode;

    // create new file
    u64 ino;
    u64 cookie;
    const char* name = "move_span";
    int err = ternfs_error_to_linux(ternfs_shard_create_file(
        (struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info,
        ternfs_inode_shard(enode->inode.i_ino), TERNFS_INODE_FILE, name, strlen(name), &ino, &cookie
    ));
    if (err) { goto out_err; }

    // move span
    err = ternfs_error_to_linux(ternfs_shard_move_span(
        (struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info,
        enode->inode.i_ino, span->offset, enode->file.cookie, ino, 0, cookie, span->written
    ));
    if (err) { goto out_err; }

    // go for it
    err = add_span_initiate(span);
    if (err) { goto out_err; }

    return 0;

out_err:
    ternfs_info("moving span failed %d", err);
    return err;
}

static int retry_after_block_error(struct ternfs_transient_span* span) {
    int i;
    int B = ternfs_blocks(span->parity);
    int err = 0;

    for (i = 0; i < B; i++) {
        if (span->blocks_errs[i]) {
            if (err) {
                ternfs_info("dropping err %d, since we already returned %d", span->blocks_errs[i], err);
            } else {
                err = span->blocks_errs[i];
            }
        }
    }
    BUG_ON(err == 0);

    if (span->attempts < ternfs_max_write_span_attempts) {
        ternfs_info("writing span failed with err %d after %d attempts, will try again", err, span->attempts);
        return move_span_and_restart(span);
    } else {
        ternfs_warn("writing span failed with err %d after %d attempts, giving up", err, span->attempts);
        return err;
    }
}

static void write_block_finalize(struct ternfs_transient_span* span, int b, u64 proof, int block_write_err) {
    int i;
    int B = ternfs_blocks(span->parity);
    struct ternfs_inode* enode = span->enode;

    ternfs_debug("block=%d proof=%016llx err=%d", b, proof, block_write_err);

    bool finished_writing = true;
    bool any_block_errs = false;
    // Mark the current one as done/error out, and check if we're done with all block
    // writing requests.
    spin_lock_bh(&span->lock);
    for (i = 0; i < B; i++) {
        BUG_ON(span->block_ids[i] == 0);
        if (i == b) {
            if (block_write_err == 0) {
                span->blocks_proofs[i] = proof;
            } else {
                span->blocks_errs[i] = block_write_err;
            }
        } else if (span->blocks_proofs[i] == 0 && span->blocks_errs[i] == 0) {
            // we're waiting on some other block proof.
            finished_writing = false;
        }
        any_block_errs = any_block_errs || (span->blocks_errs[i] != 0);
    }
    spin_unlock_bh(&span->lock);

    int err = 0;
    if (finished_writing && likely(!any_block_errs)) {
        // TODO just like in the other case, would be nicer to have this async, since we're in a queue here.
        err = ternfs_error_to_linux(ternfs_shard_add_span_certify(
            (struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info,
            enode->inode.i_ino, enode->file.cookie, span->offset, span->parity,
            span->block_ids, span->blocks_proofs
        ));
    }

    if (finished_writing) {
        span->attempts++;
        if (unlikely(err == 0 && any_block_errs)) {
            // if we failed writing blocks, consider retrying
            err = retry_after_block_error(span);
        }
        atomic_cmpxchg(&enode->file.transient_err, 0, err);
    }
    // waiters will be woken up once span ref count reaches 0
    put_transient_span(span);
}

static void write_block_done(void* data, struct list_head* pages, u64 block_id, u64 proof, int block_write_err) {
    // We need to be careful with locking here: the writer waits on the
    // flushing semaphore while holding the inode lock.

    // we use 0 to mean "no proof yet"
    if (unlikely(block_write_err == 0 && proof == 0)) {
        block_write_err = -EIO; // we need to get _very_ unlucky for this to happen!
    }

    struct ternfs_transient_span* span = (struct ternfs_transient_span*)data;

    ternfs_debug("block_id=%016llx proof=%016llx err=%d", block_id, proof, block_write_err);

    int B = ternfs_blocks(span->parity);

    // find the index that concerns us now
    int b;
    for (b = 0; b < B; b++) {
        if (span->block_ids[b] == block_id) {
            break;
        }
    }
    BUG_ON(b == B); // we found the block
    BUG_ON(span->blocks_proofs[b] != 0); // no proof or err already
    BUG_ON(span->blocks_errs[b] != 0);

    // re-acquire the pages
    list_replace_init(pages, &span->blocks[b]);

    write_block_finalize(span, b, proof, block_write_err);
}

static int add_span_initiate(struct ternfs_transient_span* span) {
    int i;
    struct ternfs_inode* enode = span->enode;
    int B = ternfs_blocks(span->parity);

    int cell_size = span->block_size / span->stripes;

    ternfs_debug("add span initiate");

    // fill in blacklist
    for (i = 0; i < B; i++) {
        if (span->blocks_errs[i]) {
            if (span->blacklist_length == TERNFS_MAX_BLACKLIST_LENGTH) {
                // it's ok to try again, we have limit on number of attempts
                ternfs_warn("span initiate blacklist full, dropping other errored out blocks");
                break;
            }
            static_assert(sizeof(span->blacklisted_failure_domains[span->blacklist_length]) == sizeof(span->failure_domains[i]));
            memcpy(span->blacklisted_failure_domains[span->blacklist_length], span->failure_domains[i], sizeof(span->failure_domains[i]));
            span->blacklist_length++;
        }
    }

    // reset stuff (including bad failure domains themselves)
    memset(span->block_ids, 0, sizeof(span->block_ids));
    memset(span->failure_domains, 0, sizeof(span->failure_domains));
    memset(span->blocks_proofs, 0, sizeof(span->blocks_proofs));
    memset(span->blocks_errs, 0, sizeof(span->blocks_errs));

    struct ternfs_add_span_initiate_block blocks[TERNFS_MAX_BLOCKS];
    int err = ternfs_error_to_linux(ternfs_shard_add_span_initiate(
        (struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info,
        span, enode->inode.i_ino, enode->file.cookie, span->offset, span->written,
        span->span_crc, span->storage_class, span->parity, span->stripes, cell_size, span->cell_crcs,
        span->blacklist_length, span->blacklisted_failure_domains,
        blocks
    ));
    if (unlikely(err)) {
        // this means that we've failed to do the metadata (not the blocks), we can't recover from this
        return err;
    }

    // send all the requests. at this point we never fail, we instead defer
    // failure to the block requests. we intentionally let all the requests
    // reach completion (rather than erroring on the first one) to try to
    // clear out all bad failure domains in one go.
    for (i = 0; i < B; i++) {
        // first mark the downloading blocks. required to do this upfront
        // so that we'll mark everything as finished only once we've gone
        // through all the requests.
        struct ternfs_add_span_initiate_block* block = &blocks[i];
        span->block_ids[i] = block->block_id;
        static_assert(sizeof(span->failure_domains[i]) == sizeof(block->failure_domain));
        memcpy(span->failure_domains[i], block->failure_domain, sizeof(block->failure_domain));
    }
    for (i = 0; i < B; i++) { // then start uploading
        struct ternfs_add_span_initiate_block* block = &blocks[i];
        u32 block_crc = 0;
        int s;
        for (s = 0; s < span->stripes; s++) {
            block_crc = ternfs_crc32c_append(block_crc, span->cell_crcs[s*B + i], cell_size);
        }
        struct ternfs_block_service bs = {
            .id = block->block_service_id,
            .ip1 = block->ip1,
            .port1 = block->port1,
            .ip2 = block->ip2,
            .port2 = block->port2,
        };
        hold_transient_span(span); // for the callback when block is done
        int err = ternfs_write_block(
            &write_block_done,
            span,
            &bs,
            block->block_id,
            block->certificate,
            span->block_size,
            block_crc,
            &span->blocks[i]
        );
        if (err) { // "finish" immediately, we've errored out
            write_block_finalize(span, i, 0, err);
        }
    }

    // Now we wait.
    ternfs_debug("sent all block writing requests");
    return 0;
}

static struct page* alloc_write_page(struct ternfs_inode_file* file) {
    // The zeroing is to assume that in the blocks we have trailing zeros,
    // and we also use this to just append zeros to the blocks without
    // copying. The former is definitely not needed, but anyway, simpler
    // for now.
    struct page* p = alloc_page(GFP_KERNEL | __GFP_ZERO);
    if (p == NULL) { return p; }
    // This contributes to OOM score.
#if (LINUX_VERSION_CODE < KERNEL_VERSION(6,2,0))
    atomic_long_inc_return(&file->mm->rss_stat.count[MM_FILEPAGES]);
#else
    percpu_counter_inc(&file->mm->rss_stat[MM_FILEPAGES]);
#endif
    return p;
}

static int compute_span_parameters(
    struct ternfs_policy* block_policy_p,
    struct ternfs_policy* span_policy_p,
    struct ternfs_policy* stripe_policy_p,
    struct ternfs_transient_span* span
) {
    u32 span_size = span->written;
    ternfs_debug("span_size=%u", span_size);

    // Easy case: inline span (or empty, can happen for empty files)
    if (span_size < 256) {
        span->storage_class = span_size > 0 ? TERNFS_INLINE_STORAGE : TERNFS_EMPTY_STORAGE;
        span->block_size = 0;
        span->stripes = 0;
        span->parity = 0;
        return 0;
    }

    // fetch the bodies of the policies
    struct ternfs_policy_body block_policy;
    ternfs_get_policy_body(block_policy_p, &block_policy);
    struct ternfs_policy_body span_policy;
    ternfs_get_policy_body(span_policy_p, &span_policy);
    struct ternfs_policy_body stripe_policy;
    ternfs_get_policy_body(stripe_policy_p, &stripe_policy);

    // Pick parity
    int num_span_policies = ternfs_span_policy_len(span_policy.body, span_policy.len);
    BUG_ON(num_span_policies == 0);
    int i;
    u8 parity = 0;
    for (i = num_span_policies - 2; i >= 0; i--) {
        u32 max_size;
        u8 this_parity;
        ternfs_span_policy_get(span_policy.body, span_policy.len, i, &max_size, &this_parity);
        if (span_size > max_size) {
            i++;
            ternfs_span_policy_get(span_policy.body, span_policy.len, i, &max_size, &parity);
            break;
        }
    }
    if (parity == 0) {
        i = 0;
        u32 max_size;
        ternfs_span_policy_get(span_policy.body, span_policy.len, i, &max_size, &parity);
        BUG_ON(span_size > max_size);
    }
    if (unlikely(ternfs_data_blocks(parity) > TERNFS_MAX_DATA || ternfs_parity_blocks(parity) > TERNFS_MAX_PARITY)) {
        ternfs_info("got asked to write with parity (%d,%d), which is beyond max blocks (%d,%d)", ternfs_data_blocks(parity), ternfs_parity_blocks(parity), TERNFS_MAX_DATA, TERNFS_MAX_PARITY);
        return -EINVAL;
    }

    // For simplicity, we have all cells to be a multiple of PAGE_SIZE, which means that all
    // blocks/stripes/spans are multiples of PAGE_SIZE. We might also have some extra pages
    // at the end to have things to line up correctly. This should only happens for big spans
    // anyway (small spans will just use mirroring).
    u32 target_stripe_size = ternfs_stripe_policy(stripe_policy.body, stripe_policy.len);
    int S;
    if (ternfs_data_blocks(parity) == 1) {
        // If we only have one data block, things are also pretty simple (just mirroring).
        // It doesn't make sense to have stripes in this case.
        S = 1;
    } else {
        // Otherwise compute striping etc.
        // We use target_stripe_size as an upper bound, for now (i.e. the stripe will always be smaller
        // than TargetStripeSize)
        S = min(max((u32)1, span_size / target_stripe_size), (u32)15);
    }
    int D = ternfs_data_blocks(parity);
    int block_size = (span_size + D - 1) / D;
    int cell_size = (block_size + S - 1) / S;
    // Round up to cell to page size
    cell_size = (cell_size + PAGE_SIZE - 1) & PAGE_MASK;
    block_size = cell_size * S;

    // Pick storage class
    BUG_ON(span_size > TERNFS_MAX_BLOCK_SIZE);
    int num_block_policies = ternfs_block_policy_len(block_policy.body, block_policy.len);
    BUG_ON(num_block_policies == 0);
    u8 storage_class;
    for (i = num_block_policies-1; i >= 0; i--) {
        u32 min_size;
        ternfs_block_policy_get(block_policy.body, block_policy.len, i, &storage_class, &min_size);
        if (block_size > min_size) {
            break;
        }
    }

    span->storage_class = storage_class;
    span->block_size = block_size;
    span->stripes = S;
    span->parity = parity;

    return 0;
}

// To be called with the inode lock. Acquires the flushing sema.
static int wait_flushed(struct ternfs_inode* enode, bool non_blocking) {
    BUG_ON(!inode_is_locked(&enode->inode));

    // make sure we're free to go
    if (down_trylock(&enode->file.flushing_span_sema) == 1) {
        if (non_blocking) {
            return -EAGAIN;
        }
        down(&enode->file.flushing_span_sema);
    }

    return 0;
}

static int write_blocks(struct ternfs_transient_span* span) {
    int i, j;
    int err = 0;

    struct ternfs_inode* enode = span->enode;

    // transient file is already borked
    err = atomic_read(&enode->file.transient_err);
    if (err) { goto out; }

    int D = ternfs_data_blocks(span->parity);
    int B = ternfs_blocks(span->parity);
    int S = span->stripes;

    ternfs_debug("size=%d, D=%d, B=%d, S=%d", span->written, D, B, S);
    loff_t offset = enode->inode.i_size - span->written;
    trace_eggsfs_span_flush_enter(
        enode->inode.i_ino, offset, span->written, span->parity, span->stripes, span->storage_class
    );
    // Below, remember that cell size is always a multiple of PAGE_SIZE.
    u32 pages_per_block = span->block_size/PAGE_SIZE;
    u32 data_pages = pages_per_block*D;
    u32 current_data_pages = (span->written + PAGE_SIZE - 1) / PAGE_SIZE;
    ternfs_debug("allocating padding pages_per_block=%u total_pages=%u current_pages=%u", pages_per_block, data_pages, current_data_pages);
    // This can happen if we have trailing zeros because of how we arrange the
    // stripes.
    for (i = current_data_pages; i < data_pages; i++) {
        struct page* zpage = alloc_write_page(&enode->file);
        if (zpage == NULL) {
            err = -ENOMEM;
            // we'd have to back out the pages we allocated so far and i can't be bothered
            goto out;
        }
        list_add_tail(&zpage->lru, &span->pages);
    }
    int cell_size = span->block_size / S;
    ternfs_debug("arranging pages");
    // First, we arrange the data blocks appropriatedly.
    {
        int cell_offset = 0;
        int block = 0;
        struct page* page;
        struct page* tmp;
        list_for_each_entry_safe(page, tmp, &span->pages, lru) {
            list_del(&page->lru);
            list_add_tail(&page->lru, &span->blocks[block]);
            cell_offset += PAGE_SIZE;
            if (cell_offset >= cell_size) {
                block++;
                block = block % D;
                cell_offset = 0;
            }
        }
    }
    // Then we allocate the parity blocks
    for (i = D; i < B; i++) {
        for (j = 0; j < pages_per_block; j++) {
            struct page* ppage = alloc_write_page(&enode->file);
            if (ppage == NULL) { err = -ENOMEM; goto out; }
            list_add_tail(&ppage->lru, &span->blocks[i]);
        }
    }
    ternfs_debug("computing parity");
    // Then, we compute the parity blocks and the CRCs for each block.
    {
        char* pages_bufs[TERNFS_MAX_BLOCKS];
        kernel_fpu_begin();
        // traverse each block, page by page, rotating as we go along
        for (j = 0; j < pages_per_block; j++) {
            // get buffer pointers
            int s = (j*PAGE_SIZE)/cell_size;
            for (i = 0; i < B; i++) {
                pages_bufs[i] = kmap_atomic(list_first_entry(&span->blocks[i], struct page, lru));
            }
            // compute parity
            err = ternfs_compute_parity(span->parity, PAGE_SIZE, (const char**)&pages_bufs[0], &pages_bufs[D]);
            // compute CRCs
            for (i = 0; i < B; i++) {
                span->cell_crcs[s*B + i] = ternfs_crc32c(span->cell_crcs[s*B + i], pages_bufs[i], PAGE_SIZE);
            }
            for (i = 0; i < B; i++) {
                kunmap_atomic(pages_bufs[i]);
                list_rotate_left(&span->blocks[i]);
            }
            // unmap pages before exiting
            if (err) { goto out_err_fpu; }
        }
        // Compute overall span crc
        int s;
        for (s = 0; s < S; s++) {
            for (i = 0; i < D; i++) {
                span->span_crc = ternfs_crc32c_append(span->span_crc, span->cell_crcs[s*B + i], cell_size);
            }
        }
        span->span_crc = ternfs_crc32c_zero_extend(span->span_crc, (int)span->written - (int)(span->block_size*D));
        kernel_fpu_end();
    }
    // Start the first attempt
    // Now we need to init the span (the callback will actually send the requests)
    // TODO would be better to have this async, and free up the queue for other stuff,
    // and also not block in this call which might be in a non-blocking write.
    // TODO Isn't this called on file write, and therefore not taking up time in
    // the queue? I think the comment above is partially outdated.
    err = add_span_initiate(span);

out:
    if (err) { // we've failed before trying to write blocks, terminate immediately
        atomic_cmpxchg(&enode->file.transient_err, 0, err); // store error if we have one
    }
    return err;

out_err_fpu:
    kernel_fpu_end();
    goto out;
}

// To be called with the inode lock.
static int start_flushing(struct ternfs_inode* enode, bool non_blocking) {
    BUG_ON(!inode_is_locked(&enode->inode));

    // Wait for the existing thing to be flushed
    int err = wait_flushed(enode, non_blocking);
    if (err < 0) { return err; }

    // Now turn the writing span into a flushing span
    struct ternfs_transient_span* span = enode->file.writing_span;

    if (span == NULL) {
        up(&enode->file.flushing_span_sema);
        return 0;
    }
    enode->file.writing_span = NULL;
    // We should hold the only reference
    BUG_ON(atomic_read(&span->refcount) != 1);
    // We mark span as started flushin so we know to wake up waiters on last reference going away.
    // At that point operation is done in either failure or success.
    span->started_flushing = true;
    err = compute_span_parameters(
        enode->block_policy, enode->span_policy, enode->stripe_policy, span
    );
    if (err) {
        goto out;
    }

    // Here (and in `write_blocks`) we might block even if we're non
    // blocking, since we don't have async metadata requests yet, which is
    // a bit unfortunate.

    if (span->storage_class == TERNFS_INLINE_STORAGE) {
        // this is an easy one, just add the inline span
        struct page* page = list_first_entry(&span->pages, struct page, lru);
        char* data = kmap(page);
        ternfs_debug("adding inline span of length %d", span->written);
        err = ternfs_error_to_linux(ternfs_shard_add_inline_span(
            (struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info, enode->inode.i_ino, enode->file.cookie,
            span->offset, span->written, data, span->written
        ));
        kunmap(page);
    } else if (span->storage_class != TERNFS_EMPTY_STORAGE) {
        // The real deal, we need to write blocks. Note that
        // this guy is tasked with releasing the flushing semaphore
        // if things fail (otherwise we're not done yet).
        err = write_blocks(span);
    }

out:
    if (err) {
        ternfs_info("attempting to flush failed permanently, err=%d ino=%016lx", err, enode->inode.i_ino);
        atomic_cmpxchg(&enode->file.transient_err, 0, err);
    }
    // we don't need this anymore (the requests might though)
    put_transient_span(span);
    return err;
}

// To be called with the inode lock.
//
// This accepts a NULL `from`, in which case zeros will be written.
ssize_t ternfs_file_write_internal(struct ternfs_inode* enode, int flags, loff_t* ppos, struct iov_iter* from, size_t count) {
    BUG_ON(!inode_is_locked(&enode->inode));

    int err;
    if (flags & IOCB_DIRECT) { return -ENOSYS; }

    loff_t ppos_before = *ppos;

    ternfs_debug("enode=%p, ino=%lu, count=%lu, size=%lld, *ppos=%lld status=%d", enode, enode->inode.i_ino, count, enode->inode.i_size, *ppos, enode->file.status);

    if (enode->file.status != TERNFS_FILE_STATUS_WRITING) {
        err = -EROFS;
        goto out_err;
    }

    // permanent error
    err = atomic_read(&enode->file.transient_err);
    if (err) { goto out_err; }

    // we're writing at the right offset
    if (*ppos != enode->inode.i_size) {
        err = -EINVAL;
        goto out_err;
    }

    // Precompute the bound for spans
    u32 max_span_size;
    {
        struct ternfs_policy_body span_policy;
        ternfs_get_policy_body(enode->span_policy, &span_policy);
        u8 p;
        ternfs_span_policy_last(span_policy.body, span_policy.len, &max_span_size, &p);
    }
    BUG_ON(max_span_size%PAGE_SIZE != 0); // needed for "new span" logic paired with page copying below, we could avoid it

    // Get the span we're currently writing at
    if (enode->file.writing_span == NULL) {
        enode->file.writing_span = new_transient_span(enode, enode->inode.i_size);
        if (enode->file.writing_span == NULL) {
            err = -ENOMEM;
            goto out_err;
        }
    }
    struct ternfs_transient_span* span = enode->file.writing_span;

    // Update mtime
    struct timespec64 mtime;
    ktime_get_real_ts64(&mtime);
    inode_set_mtime_to_ts(&enode->inode, mtime);

    // We now start writing into the span, as much as we can anyway
    while (span->written < max_span_size && count) {
        // grab the page to write to
        struct page* page = list_empty(&span->pages) ? NULL : list_last_entry(&span->pages, struct page, lru);
        if (page == NULL || page->index == 0) { // we're the first ones to get here, or we need to switch to the next one
            BUG_ON(page != NULL && page->index > PAGE_SIZE);
            page = alloc_write_page(&enode->file);
            if (!page) {
                err = -ENOMEM;
                goto out_err; // we haven't corrupted anything, so no need to make it permanent
            }
            page->index = 0;
            list_add_tail(&page->lru, &span->pages);
        }

        // copy stuff into page
        int ret;
        if (likely(from)) {
            ret = copy_page_from_iter(page, page->index, PAGE_SIZE - page->index, from);
        } else {
            ret = min(count, PAGE_SIZE - page->index);
        }
        if (ret < 0) { err = ret; goto out_err_permanent; }
        ternfs_debug("written %d to page %p", ret, page);
        enode->inode.i_size += ret;
        span->written += ret;
        page->index = (page->index + ret) % PAGE_SIZE;
        *ppos += ret;
        count -= ret;
    }

    if (span->written >= max_span_size) { // we need to start flushing the span
        // this might block even if it says nonblock, which is unfortunate.
        err = start_flushing(enode, !!(flags&IOCB_NOWAIT));
        if (err < 0) { goto out_err_permanent; }
    }

    int written;
out:
    written = *ppos - ppos_before;
    ternfs_debug("written=%d", written);
    return written;

out_err_permanent:
    atomic_cmpxchg(&enode->file.transient_err, 0, err);
out_err:
    ternfs_debug("err=%d", err);
    if (err == -EAGAIN && *ppos > ppos_before) {
        // We can just return what we've already written. In fact, it's important
        // that we do so, otherwise repeated calls might fail because they'd be
        // with the wrong offset.
        goto out;
    }
    return err;
}

ssize_t ternfs_file_write(struct ternfs_inode* enode, int flags, loff_t* ppos, struct iov_iter* from) {
    return ternfs_file_write_internal(enode, flags, ppos, from, iov_iter_count(from));
}

static ssize_t file_write_iter(struct kiocb* iocb, struct iov_iter* from) {
    struct file* file = iocb->ki_filp;
    struct inode* inode = file->f_inode;
    struct ternfs_inode* enode = TERNFS_I(inode);

    if (!inode_trylock(inode)) {
        if (iocb->ki_flags & IOCB_NOWAIT) {
            return -EAGAIN;
        }
        inode_lock(inode);
    }
    ssize_t res = ternfs_file_write(enode, iocb->ki_flags, &iocb->ki_pos, from);
    inode_unlock(inode);

    return res;
}

int ternfs_file_flush(struct ternfs_inode* enode, struct dentry* dentry) {
    inode_lock(&enode->inode);

    int err = 0;

    // Not writing, there's nothing to do, there's nothing to do, files are immutable
    if (enode->file.status != TERNFS_FILE_STATUS_WRITING) {
        ternfs_debug("status=%d, won't flush", enode->file.status);
        goto out_early;
    }

    // We are in another process, skip
    if (enode->file.owner != current->group_leader) {
        ternfs_debug("owner=%p != group_leader=%p, won't flush", enode->file.owner, current->group_leader);
        goto out_early;
    }

    bool file_is_alive_and_flushing = false;

    // if we've errored out already, just exit
    err = atomic_read(&enode->file.transient_err);
    if (err < 0) { goto out; }

    // If the owner doesn't have an mm anymore, it means that the file
    // is being torn down after the process has been terminated. In that case
    // we shouldn't even link the file.
    if (current->mm == NULL) {
        // abort everything
        atomic_cmpxchg(&enode->file.transient_err, 0, -EIO);
        goto out;
    }

    // OK, in this case we are indeed linking it, we need to do two things: first wait
    // for the span which is being flushed to be flushed, and then flush the current span,
    // if we have one.
    err = start_flushing(enode, false);
    if (err < 0) { goto out; }

    down(&enode->file.flushing_span_sema);
    file_is_alive_and_flushing = true;

    // the requests might have failed
    err = atomic_read(&enode->file.transient_err);
    if (err < 0) { goto out; }

    // now we actually need to link
    ternfs_debug("linking file");
    err = ternfs_error_to_linux(ternfs_shard_link_file(
        (struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info, enode->inode.i_ino,
        enode->file.cookie, dentry->d_parent->d_inode->i_ino, dentry->d_name.name, dentry->d_name.len,
        &enode->edge_creation_time
    ));
    if (err < 0) { goto out; }

    // finalize mtime
    inode_set_mtime(&enode->inode, enode->edge_creation_time / 1000000000, enode->edge_creation_time % 1000000000);
    inode_set_ctime(&enode->inode, enode->edge_creation_time / 1000000000, enode->edge_creation_time % 1000000000);

    // Switch the file to a normal file
    enode->file.status = TERNFS_FILE_STATUS_READING;
    smp_store_release(&enode->getattr_expiry, 0);

    // expire the directory listing -- we know for a fact that it
    // is wrong, it now contains this file.
    {
        struct dentry* parent = dget_parent(dentry);
        WRITE_ONCE(TERNFS_I(d_inode(parent))->dir.mtime_expiry, 0);
        dput(parent);
    }

out:
    if (err) {
        atomic_cmpxchg(&enode->file.transient_err, 0, err);
    }
    if (!file_is_alive_and_flushing) {
        // The file is dead, we want to make sure that any in-flight flushings
        // are gone. We might enter this function multiple times with a dead
        // file: imagine for instance a process dying with many threads having
        // the reference to the same file. This is why we also release the
        // semaphore afterwards, so that after the first one that cleans up
        // the others will not get stuck. So this section acts as a flushing
        // barrier of sorts. Since the file is dead this should not cause problems
        // (e.g. we should not get genuine flushes due to writes which then conflict
        // with this logic).
        down(&enode->file.flushing_span_sema);
        up(&enode->file.flushing_span_sema);
    }
    // There are cases where we decide not to flush, we still need to free the writing span
    if (enode->file.writing_span != NULL) {
        // if writing_span is not NULL we should be the one holding last reference to it
        BUG_ON(!put_transient_span(enode->file.writing_span));
        enode->file.writing_span = NULL;
    }
    // after we last cleared it should have no longer be set

    BUG_ON(enode->file.writing_span != NULL);
    if (enode->file.mm) {
        mmdrop(enode->file.mm);
    }
    enode->file.mm = NULL;
    inode_unlock(&enode->inode);
    return err;

out_early:
    inode_unlock(&enode->inode);
    return err;
}

static int file_flush_internal(struct file* filp, fl_owner_t id) { // can we get write while this is in progress?
    struct ternfs_inode* enode = TERNFS_I(filp->f_inode);
    struct dentry* dentry = filp->f_path.dentry;
    return ternfs_file_flush(enode, dentry);
}

static ssize_t file_read_iter(struct kiocb* iocb, struct iov_iter* to) {
    struct file* file = iocb->ki_filp;
    struct inode* inode = file->f_inode;
    struct ternfs_inode* enode = TERNFS_I(inode);

    if (unlikely(iocb->ki_flags & IOCB_DIRECT)) { return -ENOSYS; }

    // make sure we have size information
    int err = ternfs_do_getattr(enode, ATTR_CACHE_NORM_TIMEOUT);
    if (err) { return err; }

    return generic_file_read_iter(iocb, to);
}

void ternfs_link_destructor(void* buf) {
    kfree(buf);
}

char* ternfs_read_link(struct ternfs_inode* enode) {
    ternfs_debug("ino=%016lx", enode->inode.i_ino);

    BUG_ON(ternfs_inode_type(enode->inode.i_ino) != TERNFS_INODE_SYMLINK);

    // make sure we have size information
    int err = ternfs_do_getattr(enode, ATTR_CACHE_NORM_TIMEOUT);
    if (err) { return ERR_PTR(err); }

    BUG_ON(enode->inode.i_size > PAGE_SIZE); // for simplicity...
    size_t size = enode->inode.i_size;

    ternfs_debug("size=%lu", size);

    struct ternfs_span* span = ternfs_get_span((struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info, &enode->file.spans, 0);
    if (span == NULL) {
        ternfs_debug("got no span, empty file?");
        return "";
    }
    if (IS_ERR(span)) { return ERR_CAST(span); }

    BUG_ON(span->end-span->start != size); // again, for simplicity

    char* buf = kmalloc(size+1, GFP_KERNEL);
    if (buf == NULL) { err = -ENOMEM; goto out_err; }

    if (span->storage_class == TERNFS_INLINE_STORAGE) {
        memcpy(buf, TERNFS_INLINE_SPAN(span)->body, size);
    } else {
        struct page* page;
        struct ternfs_block_span* block_span = TERNFS_BLOCK_SPAN(span);
        LIST_HEAD(pages);
        page = alloc_page(GFP_KERNEL);
        if(IS_ERR(page)) {
            err = PTR_ERR(page);
            goto out_err;
        }
        page->index = 0;
        list_add_tail(&page->lru, &pages);
        LIST_HEAD(extra_pages);
        err = ternfs_span_get_pages(block_span, enode->inode.i_mapping, &pages, 1, &extra_pages);
        if (err) goto out_err;
        list_del(&page->lru);
        put_pages_list(&pages);
        put_pages_list(&extra_pages);
        char* page_buf = kmap(page);
        memcpy(buf, page_buf, size);
        kunmap(page);
    }
    buf[size] = '\0';

    ternfs_debug("link %*pE", (int)size, buf);
    ternfs_put_span(span);
    return buf;

out_err:
    ternfs_debug("get_link err=%d", err);
    ternfs_put_span(span);
    return ERR_PTR(err);
}

static loff_t file_lseek(struct file *file, loff_t offset, int whence) {
    // When seeking a read file, we just do as normal.
    // When seeking a file we're writing, we only allow to seek forward,
    // which is the same as writing zeros past the end.
    struct inode* inode = file->f_inode;
    struct ternfs_inode* enode = TERNFS_I(inode);

    if (likely(smp_load_acquire(&enode->file.status) == TERNFS_FILE_STATUS_READING)) {
        // make sure we have size information
        int err = ternfs_do_getattr(enode, ATTR_CACHE_NORM_TIMEOUT);
        if (err) { return err; }
        return generic_file_llseek(file, offset, whence);
    }

    inode_lock(inode);

    loff_t ppos = file->f_pos;
    // We only support ppos < i_size when reading the current position.
    if (ppos != enode->inode.i_size && (offset != 0 || whence != SEEK_CUR)) {
        goto out_err;
    }

    switch (whence) {
    case SEEK_SET:
        if (offset < ppos) { goto out_err; }
        break;
    case SEEK_CUR:
    case SEEK_END:
        if (offset < 0) { goto out_err; }
        offset = ppos + offset;
        break;
    default:
        goto out_err;
    }
    while (ppos < offset) {
        ssize_t written = ternfs_file_write_internal(enode, 0, &ppos, NULL, offset - ppos);
        if (unlikely(written < 0)) {
            offset = written;
            goto out;
        }
        file->f_pos = ppos;
        file->f_version = 0; // what's this for?
    }

out:
    inode_unlock(inode);
    return offset;

out_err:
    offset = -EINVAL;
    goto out;
}

// Files are not visible until they're fully persisted, so fsync is unnecessary.
static int file_fsync(struct file* f, loff_t start, loff_t end, int datasync) {
    return 0;
}

const struct file_operations ternfs_file_operations = {
    .open = file_open,
    .read_iter = file_read_iter,
    .write_iter = file_write_iter,
    .flush = file_flush_internal,
    .llseek = file_lseek,
    .mmap = generic_file_readonly_mmap,
    .fsync = file_fsync,
};

static void process_file_pages(struct address_space *mapping, struct list_head *pages, unsigned nr_pages) {
    struct page* page;
    int err;
    for(;;) {
        page = list_first_entry_or_null(pages, struct page, lru);
        if (page == NULL) {
            break;
        }
        list_del(&page->lru);
        u32 refct = atomic_read(&page->_refcount);
        if(refct != 1) ternfs_warn("page refcount=%d, expected 1 at index %ld, pages=%d", refct, page->index, nr_pages);
        err = add_to_page_cache_lru(page,
                        mapping,
                        page->index,
                        readahead_gfp_mask(mapping));
        if (err != 0) {
            ternfs_debug("failed adding page at index=%ld, err=%d", page->index, err);
        } else {
            unlock_page(page);
            SetPageUptodate(page);
        }
        put_page(page);
    }
}

static int file_readpages(struct file *filp, struct address_space *mapping, struct list_head *pages, unsigned nr_pages) {
    int err = 0;

    struct inode* inode = file_inode(filp);
    struct ternfs_inode* enode = TERNFS_I(inode);
    loff_t off = page_offset(lru_to_page(pages));
    ternfs_debug("enode=%p, ino=%ld, offset=%lld, nr_pages=%u", enode, inode->i_ino, off, nr_pages);

    struct ternfs_span* span = NULL;
    struct page *page;
    LIST_HEAD(extra_pages);

    if (off >= inode->i_size) { // out of bounds
        list_for_each_entry(page, pages, lru) {
            zero_user_segment(page, 0, PAGE_SIZE);
        }
        goto out;
    }

    span = ternfs_get_span((struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info, &enode->file.spans, off);
    if (IS_ERR(span)) {
        ternfs_warn("ternfs_get_span_failed at pos %llu", off);
        err = PTR_ERR(span);
        span = NULL;
        goto out_err;
    }

    if (span->storage_class == TERNFS_INLINE_STORAGE) {
        struct ternfs_inline_span* inline_span = TERNFS_INLINE_SPAN(span);
        page = lru_to_page(pages);
        if (page == NULL) {
            err = -EIO;
            goto out_err;
        }
        list_del(&page->lru);
        // rest is past end of file, just drop it
        put_pages_list(pages);
        // return page back
        list_add_tail(&page->lru, pages);
        BUG_ON(page_offset(page) != span->start);
        char* dst = kmap_atomic(page);
        memcpy(dst, inline_span->body, inline_span->len);
        kunmap_atomic(dst);
    } else {
        struct ternfs_block_span* block_span = TERNFS_BLOCK_SPAN(span);
        err = ternfs_span_get_pages(block_span, mapping, pages, nr_pages, &extra_pages);
        if (err) {
            ternfs_warn("readahead of %d pages at off=%lld in file %016lx failed with error %d", nr_pages, off, enode->inode.i_ino, err);
            put_pages_list(&extra_pages);
            goto out_err;
        }
    }
out:
    ternfs_put_span(span);
    process_file_pages(mapping, pages, nr_pages);
    process_file_pages(mapping, &extra_pages, 0);
    return 0;

out_err:
    if (span != NULL) {
        ternfs_put_span(span);
    }
    put_pages_list(pages);
    return err;
}

static int file_readpage(struct file* filp, struct page* page) {
    struct inode* inode = file_inode(filp);
    struct ternfs_inode* enode = TERNFS_I(inode);
    u64 off = page_offset(page);
    int err = 0;

    struct ternfs_span* span = NULL;

    struct timespec64 start_ts = ns_to_timespec64(ktime_get_real_ns());
    struct timespec64 last_span_refresh_ts = ns_to_timespec64(0);

    if (off >= inode->i_size) { // out of bounds
        zero_user_segment(page, 0, PAGE_SIZE);
        goto out;
    }

retry:
    if (span != NULL) {
        ternfs_put_span(span);
    }
    span = ternfs_get_span((struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info, &enode->file.spans, off);
    if (IS_ERR(span)) {
        ternfs_debug("ternfs_get_span_failed at pos %llu", off);
        err = PTR_ERR(span);
        span = NULL;
        goto out;
    }

    if (span->start%PAGE_SIZE != 0) {
        ternfs_warn("span start is not a multiple of page size %llu", span->start);
        span = NULL;
        err = -EIO;
        goto out;
    }

    u64 span_offset = off - span->start;

    if (span->storage_class == TERNFS_INLINE_STORAGE) {
        struct ternfs_inline_span* inline_span = TERNFS_INLINE_SPAN(span);
        size_t to_copy = inline_span->len - span_offset;
        BUG_ON(to_copy > PAGE_SIZE);
        char* dst = kmap_atomic(page);
        memcpy(dst, inline_span->body + span_offset, to_copy);
        kunmap_atomic(dst);
    } else {
        struct ternfs_block_span* block_span = TERNFS_BLOCK_SPAN(span);
        if (block_span->cell_size%PAGE_SIZE != 0) {
            ternfs_warn("cell size not multiple of page size %u", block_span->cell_size);
            err = -EIO;
            goto out;
        }

        struct page* stripe_page = NULL;

        LIST_HEAD(pages);
        stripe_page = alloc_page(GFP_KERNEL);
        if(!stripe_page) {
            err = -ENOMEM;
            goto out;
        }
        stripe_page->index = page->index;
        list_add_tail(&stripe_page->lru, &pages);
        LIST_HEAD(extra_pages);
        err = ternfs_span_get_pages(block_span, filp->f_mapping, &pages, 1, &extra_pages);
        process_file_pages(filp->f_mapping, &extra_pages, 0);
        if (err) {
            put_pages_list(&pages);
            struct timespec64 end_ts = ns_to_timespec64(ktime_get_real_ns());
            ternfs_warn("reading page %lld in file %016lx failed with error %d", off, enode->inode.i_ino, err);
            if ((end_ts.tv_sec - start_ts.tv_sec) > ternfs_file_io_timeout_sec) {
                ternfs_warn("io timeout while reading page at off=%lld in file %016lx last error %d", off, enode->inode.i_ino, err);
                goto out;
            }
            if ((end_ts.tv_sec - last_span_refresh_ts.tv_sec) > ternfs_file_io_retry_refresh_span_interval_sec) {
                ternfs_info("refreshing span %lld of file %016lx as the span structure or block service flags could have changed in the meantime", off, enode->inode.i_ino);
                ternfs_unlink_span(&enode->file.spans, span);
                span = NULL; // unlink already reduced refcount
                last_span_refresh_ts = end_ts;
            }
            goto retry;
        }
        list_del(&stripe_page->lru);

        char* to_ptr = kmap_atomic(page);
        char* from_ptr = kmap_atomic(stripe_page);
        memcpy(to_ptr, from_ptr, PAGE_SIZE);
        kunmap_atomic(to_ptr);
        kunmap_atomic(from_ptr);

        BUG_ON(atomic_read(&stripe_page->_refcount) != 1);
        put_page(stripe_page);
    }

out:
    if (unlikely(err)) {
        SetPageError(page);
    } else {
        SetPageUptodate(page);
    }
    if (span != NULL) {
        ternfs_put_span(span);
    }
    unlock_page(page);
    return err;
}

#if (LINUX_VERSION_CODE >= KERNEL_VERSION(5, 18, 0))
static void file_readahead(struct readahead_control *rac)
{
    struct file *filp = rac->file;
    struct inode *inode = file_inode(filp);
    struct ternfs_inode *enode = TERNFS_I(inode);
    struct page *page;
    pgoff_t index;
    loff_t off;
    unsigned nr_pages = readahead_count(rac);
    
    if (nr_pages == 0)
        return;

    // make sure we have size information
    int err = ternfs_do_getattr(enode, ATTR_CACHE_NORM_TIMEOUT);
    if (err) { return; }
        
    index = readahead_index(rac);
    off = index << PAGE_SHIFT;
    
    ternfs_debug("enode=%p, ino=%ld, index=%lu, offset=%lld, nr_pages=%u", 
                 enode, inode->i_ino, index, off, nr_pages);

    struct ternfs_span *span = NULL;
    LIST_HEAD(pages);
    LIST_HEAD(extra_pages);
    unsigned pages_allocated = 0;

    // Check if we're out of bounds
    if (off >= inode->i_size) {
        // Allocate and zero-fill pages for out-of-bounds requests
        unsigned i;
        for (i = 0; i < nr_pages; i++) {
                
            page = alloc_page(readahead_gfp_mask(rac->mapping));
            if (!page)
                break;
                
            page->index = index + i;
            zero_user_segment(page, 0, PAGE_SIZE);
            list_add(&page->lru, &pages);
            pages_allocated++;
        }
        goto out_process;
    }

    // Get the span for this offset
    span = ternfs_get_span((struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info, &enode->file.spans, off);
    if (IS_ERR(span)) {
        ternfs_warn("ternfs_get_span_failed at pos %llu", off);
        err = PTR_ERR(span);
        span = NULL;
        goto out_err;
    }

    // Allocate pages for readahead
    unsigned i;
    for (i = 0; i < nr_pages; i++) {
        page = alloc_page(readahead_gfp_mask(rac->mapping));
        if (!page)
            break;
            
        page->index = index + i;
        list_add(&page->lru, &pages);
        pages_allocated++;
    }

    if (pages_allocated == 0)
        return;

    // Process based on span type
    if (span->storage_class == TERNFS_INLINE_STORAGE) {
        struct ternfs_inline_span *inline_span = TERNFS_INLINE_SPAN(span);
        u64 span_start = span->start;
        
        list_for_each_entry(page, &pages, lru) {
            u64 page_off = page_offset(page);
            if (page_off >= inode->i_size) {
                zero_user_segment(page, 0, PAGE_SIZE);
                continue;
            }
            
            u64 span_offset = page_off - span_start;
            if (span_offset >= inline_span->len) {
                zero_user_segment(page, 0, PAGE_SIZE);
                continue;
            }
            
            size_t to_copy = min((size_t)(inline_span->len - span_offset), (size_t)PAGE_SIZE);
            char *dst = kmap_atomic(page);
            memcpy(dst, inline_span->body + span_offset, to_copy);
            if (to_copy < PAGE_SIZE)
                memset(dst + to_copy, 0, PAGE_SIZE - to_copy);
            kunmap_atomic(dst);
        }
    } else {
        struct ternfs_block_span *block_span = TERNFS_BLOCK_SPAN(span);
        err = ternfs_span_get_pages(block_span, rac->mapping, &pages, pages_allocated, &extra_pages);
        if (err) {
            ternfs_warn("readahead of %u pages at off=%lld in file %016lx failed with error %d", 
                       pages_allocated, off, enode->inode.i_ino, err);
            put_pages_list(&extra_pages);
            goto out_err;
        }
    }
    ternfs_put_span(span);

out_process:
    // Process the main pages
    list_for_each_entry(page, &pages, lru) {
        struct page* rac_page = readahead_page(rac);
        void* dest = kmap_atomic(rac_page);
        void* src = kmap_atomic(page);
        memcpy(dest, src, PAGE_SIZE);
        kunmap_atomic(src);
        kunmap_atomic(dest);
        SetPageUptodate(rac_page);
        unlock_page(rac_page);
        put_page(rac_page);
    }
    // Process any extra pages
    process_file_pages(rac->mapping, &extra_pages, 0);
    put_pages_list(&pages);
    return;

out_err:
    if (span != NULL) {
        ternfs_put_span(span);
    }
    put_pages_list(&pages);
    return;
}
#endif

#if (LINUX_VERSION_CODE >= KERNEL_VERSION(5, 18, 0))
static int file_readfolio(struct file *filp, struct folio *folio) {
    BUG_ON(folio_nr_pages(folio) != 1);
    return file_readpage(filp, folio_page(folio, 0));
}
#endif 

const struct address_space_operations ternfs_mmap_operations = {
#if (LINUX_VERSION_CODE < KERNEL_VERSION(5, 18, 0))
    .readpages = file_readpages,
#else
    .readahead = file_readahead,
#endif
#if (LINUX_VERSION_CODE < KERNEL_VERSION(5, 19, 0))
    .readpage = file_readpage,
#else
    .read_folio = file_readfolio,
#endif
};

int __init ternfs_file_init(void) {
    ternfs_transient_span_cachep = kmem_cache_create(
        "ternfs_transient_span_cache",
        sizeof(struct ternfs_transient_span),
        0,
        SLAB_RECLAIM_ACCOUNT | SLAB_MEM_SPREAD,
        &init_transient_span
    );
    if (!ternfs_transient_span_cachep) { return -ENOMEM; }
    return 0;
}

void __cold ternfs_file_exit(void) {
    ternfs_debug("file exit");
    // TODO: handle case where there still are requests in flight.
    kmem_cache_destroy(ternfs_transient_span_cachep);
}
