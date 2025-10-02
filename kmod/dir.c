// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "dir.h"

#include <linux/mm.h>
#include <linux/slab.h>

#include "log.h"
#include "err.h"
#include "inode.h"
#include "metadata.h"
#include "trace.h"

static struct kmem_cache* ternfs_dirents_cachep;

static inline int ternfs_dir_entry_size(int name_len) {
    return 8 + 8 + 1 + name_len; // inode + name_hash + len + name
}

static inline void ternfs_dir_entry_parse(const char* buf, u64* ino, u64* name_hash, u8* name_len, const char** name) {
    *ino = get_unaligned_le64(buf); buf += 8;
    *name_hash = get_unaligned_le64(buf); buf += 8;
    *name_len = *(u8*)buf; buf++;
    *name = buf; buf += *name_len;
}

static inline u64 ternfs_dir_entry_hash(const char* buf) {
    return get_unaligned_le64(buf + 8);
}

static struct kmem_cache* ternfs_readdir_ctx_cachep;

struct ternfs_readdir_ctx {
    struct ternfs_dirents* dirents;
    struct page* current_page; // never NULL
    u32 page_offset;
    u16 page_n;
};

// Increments the refcount of the cache under RCU.
// After this we know the dirents won't be deallocated until we
// decrease the refcount.
static struct ternfs_dirents* ternfs_dir_get_cache(struct ternfs_inode* enode) {
    struct ternfs_dirents* dirents;
    rcu_read_lock();
    dirents = rcu_dereference(enode->dir.dirents);
    if (dirents) { atomic_inc(&dirents->refcount); }
    rcu_read_unlock();
    return dirents;
}

// Decrements the refcount, and frees up all the pages in the
// cache if we were the last ones. Must be called by
// somebody who called ternfs_dir_get_cache before.
//
// Note that ->dirents and ->dir.pages are both holding a reference, so
// once you clear those you should call `ternfs_dir_put_cache`.
static void ternfs_dir_put_cache(struct ternfs_dirents* dirents) {
    int refs = atomic_dec_return(&dirents->refcount);
    BUG_ON(refs < 0);
    if (refs == 0) {
        put_pages_list(&dirents->pages);
        kmem_cache_free(ternfs_dirents_cachep, dirents);
    }
}

static void __ternfs_dir_put_cache_rcu(struct rcu_head* rcu) {
    struct ternfs_dirents* dirents = container_of(rcu, struct ternfs_dirents, rcu_head);
    ternfs_dir_put_cache(dirents);
}

// Removes the cache from ->dir.dirents, and also calls `ternfs_dir_put_cache` to
// clear the reference. So calls to this too should be paired to a `ternfs_dir_get_cache`.
void ternfs_dir_drop_cache(struct ternfs_inode* enode) {
    if (!atomic64_read((atomic64_t*)&enode->dir.dirents)) { return; } // early atomicless exit
    // zero out dirents, and free it up after the grace period.
    struct ternfs_dirents* dirents = (struct ternfs_dirents*)atomic64_xchg((atomic64_t*)&enode->dir.dirents, (u64)0);
    if (likely(dirents)) {
        call_rcu(&dirents->rcu_head, __ternfs_dir_put_cache_rcu);
    }
}

static struct ternfs_dirents* ternfs_dir_read_all(struct dentry* dentry, u64 dir_seqno);

static int ternfs_dir_open(struct inode* inode, struct file* filp) {
    struct ternfs_inode* enode = TERNFS_I(inode);
    struct ternfs_readdir_ctx* ctx;
    struct ternfs_dirents* dirents;
    int err;

    trace_eggsfs_vfs_opendir_enter(inode);

    if (get_jiffies_64() >= enode->dir.mtime_expiry) {
        err = ternfs_dir_revalidate(enode);
        if (err) { return err; }
    }

    ctx = kmem_cache_alloc(ternfs_readdir_ctx_cachep, GFP_KERNEL);
    if (!ctx) { return -ENOMEM; }

again: // progress: whoever wins the lock either get pages or fails
    dirents = ternfs_dir_get_cache(enode);
    if (dirents && dirents->mtime != READ_ONCE(enode->mtime)) {
        ternfs_dir_put_cache(dirents);
        ternfs_dir_drop_cache(enode);
        dirents = NULL;
    }
    if (!dirents) {
        int seqno;
        if (!ternfs_latch_try_acquire(&enode->dir.dirents_latch, seqno)) {
            ternfs_latch_wait(&enode->dir.dirents_latch, seqno);
            goto again;
        }

        // If somebody else got to creating the page already, drop it,
        // otherwise we'll leak.
        dirents = ternfs_dir_get_cache(enode);
        if (dirents && dirents->mtime != READ_ONCE(enode->mtime)) {
            ternfs_dir_put_cache(dirents);
            ternfs_dir_drop_cache(enode);
            dirents = NULL;
        }

        if (!dirents) {
            u64 dir_mtime = READ_ONCE(enode->mtime);
            dirents = ternfs_dir_read_all(filp->f_path.dentry, dir_mtime);
            if (!IS_ERR(dirents)) {
                // If the mtime matches (which will be the case unless somebody
                // else revalidated between the READ_ONCE above), then the refcount
                // starts at two: one for the ->dir.dirents, and one for the
                // ->dirents below. Otherwise only for ->dirents (basically
                // we have these pages to be private to this read).
                if (likely(READ_ONCE(enode->mtime) == dir_mtime)) {
                    atomic_set(&dirents->refcount, 2);
                    rcu_assign_pointer(enode->dir.dirents, dirents);
                } else {
                    atomic_set(&dirents->refcount, 1);
                }
            }
        }
        ternfs_latch_release(&enode->dir.dirents_latch, seqno);
    }
    if (IS_ERR(dirents)) { err = PTR_ERR(dirents); goto out_err; }

    ctx->dirents = dirents;
    ctx->current_page = list_first_entry(&dirents->pages, struct page, lru);
    ctx->page_offset = 0;
    ctx->page_n = 0;

    filp->private_data = ctx;

    trace_eggsfs_vfs_opendir_exit(inode, 0);
    return 0;

out_err:
    kmem_cache_free(ternfs_readdir_ctx_cachep, ctx);
    trace_eggsfs_vfs_opendir_exit(inode, err);
    ternfs_debug("err=%d", err);
    return err;
}

static void update_dcache(struct dentry* parent, u64 dir_seqno, const char* name, int name_len, u64 edge_creation_time, u64 ino) {
    struct qstr filename = QSTR_INIT(name, name_len);
    struct dentry* dentry;
    struct dentry* alias;
    DECLARE_WAIT_QUEUE_HEAD_ONSTACK(wq);
    struct inode* parent_inode = parent->d_inode;
    if (unlikely(parent_inode == NULL)) { // TODO can this ever happen?
        ternfs_warn("got NULL in dentry parent (ino %016llx)", ino);
        return;
    }

    ternfs_debug("parent=%pd name=%*pE ino=%llx", parent, name_len, name, ino);

    filename.hash = full_name_hash(parent, filename.name, filename.len);

    dentry = d_lookup(parent, &filename);
    if (!dentry) {
again:
        dentry = d_alloc_parallel(parent, &filename, &wq);
        if (IS_ERR(dentry)) { return; }
    }
    struct ternfs_inode* enode = NULL;
    if (!d_in_lookup(dentry)) { // d_alloc_parallel sets in_lookup
        // pre-existing entry
        struct inode* old_inode = d_inode(dentry);
        if (old_inode && old_inode->i_ino == ino) {
            WRITE_ONCE(dentry->d_time, dir_seqno);
            enode = TERNFS_I(old_inode);
            goto out;
        }
        d_invalidate(dentry);
        dput(dentry);
        goto again;
    } else {
        // new entry
        struct inode* inode = ternfs_get_inode_normal(parent->d_sb, TERNFS_I(parent_inode), ino);
        if (!IS_ERR(inode)) {
            enode = TERNFS_I(inode);
            enode->edge_creation_time = edge_creation_time;
        }
        ternfs_debug("inode=%p is_err=%d", inode, IS_ERR(inode));
        // d_splice_alias propagates error in inode
        WRITE_ONCE(dentry->d_time, dir_seqno);
        alias = d_splice_alias(inode, dentry);
        ternfs_debug("alias=%p is_err=%d", alias, IS_ERR(alias));
        d_lookup_done(dentry);
        if (alias) {
            dput(dentry);
            dentry = alias;
        }
        if (IS_ERR(dentry)) {
            ternfs_debug("dentry err=%ld", PTR_ERR(dentry));
            return;
        }
    }

out:
    // Speculatively fetch stat info. Note that this will never
    // block.
    if (enode) {
        int err = ternfs_start_async_getattr(enode);
        if (err < 0 && err != -ERESTARTSYS) {
            ternfs_warn("could not send async getattr: %d", err);
        }
    }
    dput(dentry);
};

struct ternfs_dir_fill_ctx {
    struct dentry* parent;

    struct ternfs_dirents* dirents;
    // kmap'd address of last page
    char* page_addr;
    // current offset inside last page
    u32 page_off;
    // Number of ternfs dir entries in the last page
    u16 page_n;
};

static struct page* dir_fill_ctx_last_page(struct ternfs_dir_fill_ctx* ctx) {
    return list_last_entry(&ctx->dirents->pages, struct page, lru);
}

static int ternfs_dir_add_entry(struct ternfs_dir_fill_ctx* ctx, u64 name_hash, const char* name, int name_len, u64 ino) {
    if (name_len > TERNFS_MAX_FILENAME) { return -ENAMETOOLONG; }
    // Flush current page, and start a new one
    if (ctx->page_off + ternfs_dir_entry_size(name_len) > PAGE_SIZE) {
        struct page* page = alloc_page(GFP_KERNEL);
        if (!page) { return -ENOMEM; }

        // unmap and store number of entries
        kunmap(dir_fill_ctx_last_page(ctx));
        dir_fill_ctx_last_page(ctx)->index = ctx->page_n;

        // add new page, reset ctx indices
        list_add_tail(&page->lru, &ctx->dirents->pages);
        ctx->page_addr = kmap(page);
        ctx->page_off = 0;
        ctx->page_n = 0;
        static_assert(sizeof(page->private) == sizeof(name_hash));
        page->private = name_hash;
    }
    if (ctx->page_off + ternfs_dir_entry_size(name_len) > PAGE_SIZE) {
        WARN_ON(ctx->page_off + ternfs_dir_entry_size(name_len) > PAGE_SIZE);
        return -EIO;
    }

    char* entry = ctx->page_addr + ctx->page_off;
    put_unaligned_le64(ino, entry); entry += 8;
    put_unaligned_le64(name_hash, entry); entry += 8;
    *(u8*)entry = (u8)name_len; entry++;
    memcpy(entry, name, name_len);
    ctx->page_off += ternfs_dir_entry_size(name_len);
    ctx->page_n++;
    ctx->dirents->max_hash = name_hash;
    return 0;
}

int ternfs_dir_readdir_entry_cb(void* ptr, u64 hash, const char* name, int name_len, u64 edge_creation_time, u64 ino) {
    struct ternfs_dir_fill_ctx* ctx = ptr;
    ternfs_debug("hash=%llx ino=%llx, name_len=%d, name=%*pE", hash, ino, name_len, name_len, name);
    update_dcache(ctx->parent, ctx->dirents->mtime, name, name_len, edge_creation_time, ino);
    return ternfs_dir_add_entry(ctx, hash, name, name_len, ino);
}

// mut hold
static struct ternfs_dirents* ternfs_dir_read_all(struct dentry* dentry, u64 dir_mtime) {
    struct inode* inode = d_inode(dentry);
    struct ternfs_inode* enode = TERNFS_I(inode);

    struct ternfs_dirents* dirents = kmem_cache_alloc(ternfs_dirents_cachep, GFP_KERNEL);
    if (!dirents) { return ERR_PTR(-ENOMEM); }

    dirents->mtime = dir_mtime;
    INIT_LIST_HEAD(&dirents->pages);
    atomic_set(&dirents->refcount, 0); // the caller sets this up
    dirents->max_hash = 0;

    int err;
    struct page* first_page = alloc_page(GFP_KERNEL);
    if (!first_page) {
        err = -ENOMEM;
        goto out_err_dirents;
    }
    list_add_tail(&first_page->lru, &dirents->pages);
    first_page->private = 0; // first hash = 0

    bool eof = false;
    u64 continuation_key = 0;
    struct ternfs_dir_fill_ctx ctx = {
        .parent = dentry,
        .dirents = dirents,
        .page_addr = kmap(first_page),
        .page_off = 0,
        .page_n = 0,
    };

    while (!eof) {
        ternfs_debug("cont_key=%llx", continuation_key);
        err = ternfs_shard_readdir((struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info, enode->inode.i_ino, continuation_key, &ctx, &continuation_key);
        if (err) {
            err = ternfs_error_to_linux(err);
            goto out_err;
        }
        eof = continuation_key == 0;
    }

    kunmap(dir_fill_ctx_last_page(&ctx));
    dir_fill_ctx_last_page(&ctx)->index = ctx.page_n;

    return dirents;

out_err:
    kunmap(dir_fill_ctx_last_page(&ctx));
    put_pages_list(&ctx.dirents->pages);
out_err_dirents:
    kmem_cache_free(ternfs_dirents_cachep, dirents);
    ternfs_info("err=%d", err);
    return ERR_PTR(err);
};

// vfs: shared inode.i_rwsem
// vfs: mutex file.f_pos_lock
static int ternfs_dir_read(struct file* filp, struct dir_context* dctx) {
    struct ternfs_readdir_ctx* ctx = filp->private_data;

    WARN_ON(!ctx);
    if (!ctx) { return -EINVAL; }

    ternfs_debug("ctx=%p ctx.current_page=%p ctx.dirents=%p ctx.page_offset=%u ctx.page_n=%u pos=%lld", ctx, ctx->current_page, ctx->dirents, ctx->page_offset, ctx->page_n, dctx->pos);

    // For our offsets, we just use the edge hash. This is technically not guaranteed
    // to be correct: there might be collisions which mean that multiple elements will
    // have the same offset. However this is very unlikely -- we need to get to roughly
    // 5 million elements in a directory for the probability of a collision happening
    // being more than 1 in a million (assuming that the hash that we use is a good
    // hashing function). And seeking directories is somewhat uncommon -- the code below
    // will just work for normal directory listings (not skipping over duplicate offsets,
    // if they're present). So we accept it.

    if (dctx->pos < 0) {
        ternfs_warn("got negative pos %lld, this should never happen", dctx->pos);
        return -EIO;
    }

    char* page_addr;
    // The first thing we do is restore coherency between the dctx pos and the
    // ctx pos, by making sure current_page is the correct one, and that we're
    // positioned _before_ the entry we're looking for. The code below will then
    // skip forward to the right entry.
    if (dctx->pos == 0 || dctx->pos == 1) {
        // If the context position is zero (.) or one (..), we've just started. We reset the
        // context to the beginning and we're good to go.
        ternfs_debug("we're at the beginning, will start with dots");
        ctx->current_page = list_first_entry(&ctx->dirents->pages, struct page, lru);
        ctx->page_offset = 0;
        ctx->page_n = 0;
        page_addr = kmap(ctx->current_page);
    } else {
        // Otherwise, we make sure we're in the right page, if there is such a page.
        // In the very common case where we're already at the right entry (from where
        // we left off the last time), we just leave things as they are, so no skipping
        // forward will happen.
        page_addr = kmap(ctx->current_page);
        if (likely(
            ctx->page_n < ctx->current_page->index && // we're not out of bounds
            ternfs_dir_entry_hash(page_addr + ctx->page_offset) == dctx->pos // we're at the right place
        )) {
            // we're good, nothing to do
            ternfs_debug("resuming readdir");
        } else if (likely(dctx->pos > ctx->dirents->max_hash)) {
            // we're done for good
            ternfs_debug("pos out of bounds (%lld > %llu), returning", dctx->pos, ctx->dirents->max_hash);
            return 0;
        } else {
            // First try to rewind if we're at an earlier position. We know we'll stop
            // when we hit zero.
            for (; ctx->current_page->private > dctx->pos; ctx->current_page = list_prev_entry(ctx->current_page, lru)) {
                ternfs_debug("rewinding");
            }
            // Then check if we need to go forward, by checking if we have a next page,
            // and if we do, checking if the position is <= than its first hash.
            for (;;) {
                if (list_is_last(&ctx->current_page->lru, &ctx->dirents->pages)) {
                    ternfs_debug("ran out of pages to forward");
                    break;
                }
                struct page* next_page = list_next_entry(ctx->current_page, lru);
                if (next_page->private > dctx->pos) {
                    ternfs_debug("found right page to forward to");
                    break;
                }
                ternfs_debug("forwarding");
                ctx->current_page = next_page;
            }
            // Finally, reset position inside page. The code below will forward to
            // the right hash.
            page_addr = kmap(ctx->current_page);
            ctx->page_offset = 0;
            ctx->page_n = 0;
        }
    }

    // emit dots first
    if (!dir_emit_dots(filp, dctx)) {
        ternfs_debug("exiting after dots");
        return 0;
    }

    // move along, nothing to look here
    if (ctx->page_n >= ctx->current_page->index) {
        ternfs_debug("we're out of elements before even starting");
        return 0;
    }

    for (;;) {
        const char* entry = page_addr + ctx->page_offset;
        ternfs_debug("next page_addr=%p ctx.page_offset=%u entry=%p", page_addr, ctx->page_offset, entry);

        u64 ino;
        u64 hash;
        u8 name_len;
        const char* name;
        ternfs_dir_entry_parse(entry, &ino, &hash, &name_len, &name);
        ternfs_debug("name=%*pE ino=0x%016llx", name_len, name, ino);

        if (likely(hash >= dctx->pos)) {
            dctx->pos = hash;

            int dtype;

            switch (ternfs_inode_type(ino)) {
            case TERNFS_INODE_DIRECTORY: dtype = DT_DIR; break;
            case TERNFS_INODE_FILE: dtype = DT_REG; break;
            case TERNFS_INODE_SYMLINK: dtype = DT_LNK; break;
            default: dtype = DT_UNKNOWN; break;
            }

            ternfs_debug("emitting name=%*pE ino=%016llx dtype=%d hash=%llu", name_len, name, ino, dtype, hash);
            if (!dir_emit(dctx, name, name_len, ino, dtype)) {
                ternfs_debug("break");
                kunmap(ctx->current_page);
                break;
            }
        } else {
            ternfs_debug("skipping hash %llu < dctx->pos %lld", hash, dctx->pos);
        }

        ctx->page_offset += ternfs_dir_entry_size(name_len);
        ctx->page_n++;

        ternfs_debug(
            "next ctx=%p ctx.current_page=%p ctx.dirents=%p ctx.page_offset=%u ctx.page_n=%u page_n=%ld", ctx,
            ctx->current_page, ctx->dirents, ctx->page_offset, ctx->page_n, ctx->current_page->index
        );

        if (ctx->page_n >= ctx->current_page->index) {
            kunmap(ctx->current_page);
            ctx->page_n = 0;
            ctx->page_offset = 0;
            if (list_is_last(&ctx->current_page->lru, &ctx->dirents->pages)) {
                // we've run out of pages
                ternfs_debug("ran out of pages, exiting");
                ctx->current_page = list_first_entry(&ctx->dirents->pages, struct page, lru);
                dctx->pos++; // otherwise we'll loop forever
                break;
            } else {
                ctx->current_page = list_next_entry(ctx->current_page, lru);
                page_addr = kmap(ctx->current_page);
            }
        }
    }

    ternfs_debug("done, exit pos %lld", dctx->pos);
    return 0;
}

static int ternfs_dir_close(struct inode* inode, struct file* filp) {
    struct ternfs_readdir_ctx* ctx = (struct ternfs_readdir_ctx*)filp->private_data;

    WARN_ON(!ctx);
    if (!ctx) { return -EINVAL; }

    trace_eggsfs_vfs_closedir_enter(inode);
    ternfs_dir_put_cache(ctx->dirents);
    kmem_cache_free(ternfs_readdir_ctx_cachep, ctx);
    trace_eggsfs_vfs_closedir_exit(inode, 0);
	return 0;
}

static loff_t ternfs_dir_seek(struct file* file, loff_t offset, int whence) {
    struct inode* inode = file_inode(file);

    inode_lock(inode);

    switch (whence) {
    case SEEK_SET:
        break;
    case SEEK_CUR:
        offset += file->f_pos;
        break;
    case SEEK_END:
        offset = -EOPNOTSUPP;
        goto out_err;
    default:
        offset = -EINVAL;
        goto out_err;
    }

    if (offset < 0) {
        offset = -EINVAL;
        goto out_err;
    }

    ternfs_debug("setting dir position to %lld", offset);

    vfs_setpos(file, offset, MAX_LFS_FILESIZE);

out_err:
    inode_unlock(inode);
    return offset;
}

static int ternfs_dir_fsync(struct file* f, loff_t start, loff_t end, int datasync) {
    return 0;
}

struct file_operations ternfs_dir_operations = {
    .open = ternfs_dir_open,
    .iterate_shared = ternfs_dir_read,
    .release = ternfs_dir_close,
    .llseek = ternfs_dir_seek,
    .fsync = ternfs_dir_fsync,
};

int __init ternfs_dir_init(void) {
    ternfs_dirents_cachep = kmem_cache_create(
        "ternfs_dirents_cache",
        sizeof(struct ternfs_dirents),
        0,
        SLAB_RECLAIM_ACCOUNT,
        NULL
    );
    if (!ternfs_dirents_cachep) {
        return -ENOMEM;
    }

    ternfs_readdir_ctx_cachep = kmem_cache_create(
        "ternfs_readdir_ctx_cache",
        sizeof(struct ternfs_readdir_ctx),
        0,
        SLAB_RECLAIM_ACCOUNT,
        NULL
    );
    if (!ternfs_readdir_ctx_cachep) {
        kmem_cache_destroy(ternfs_dirents_cachep);
        return -ENOMEM;
    }
    return 0;
}

void __cold ternfs_dir_exit(void) {
    ternfs_debug("dir exit");
    kmem_cache_destroy(ternfs_readdir_ctx_cachep);
    kmem_cache_destroy(ternfs_dirents_cachep);
}
