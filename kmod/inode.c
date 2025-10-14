// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "inode.h"

#include <linux/sched/mm.h>

#include "latch.h"
#include "log.h"
#include "dir.h"
#include "metadata.h"
#include "dentry.h"
#include "net.h"
#include "trace.h"
#include "err.h"
#include "file.h"
#include "wq.h"
#include "inode_compat.h"

// Some services (samba) try to preallocate larger files, but can handle
// the failure or absence of ftruncate().
unsigned ternfs_disable_ftruncate = 0;

static struct kmem_cache* ternfs_inode_cachep;

#define MSECS_TO_JIFFIES(_ms) (((u64)_ms * HZ) / 1000ull)

#define POSIX_BLK_SIZE 512

// This is very rarely needed to be up to date, set it to 1hr
int ternfs_dir_getattr_refresh_time_jiffies = MSECS_TO_JIFFIES(3600000);
int ternfs_file_getattr_refresh_time_jiffies = MSECS_TO_JIFFIES(3600000);

int ternfs_dir_dentry_refresh_time_jiffies = MSECS_TO_JIFFIES(250);

static void getattr_async_complete(struct work_struct* work);

struct inode* ternfs_inode_alloc(struct super_block* sb) {
    struct ternfs_inode* enode;

    ternfs_debug("sb=%p", sb);

    enode = (struct ternfs_inode*)kmem_cache_alloc(ternfs_inode_cachep, GFP_NOFS);
    if (!enode) {
        return NULL;
    }

    ternfs_latch_init(&enode->getattr_update_latch);
    ternfs_latch_init(&enode->getattr_update_init_latch);
    INIT_DELAYED_WORK(&enode->getattr_async_work, &getattr_async_complete);

    ternfs_debug("done enode=%p", enode);
    return &enode->inode;
}

void ternfs_inode_evict(struct inode* inode) {
    struct ternfs_inode* enode = TERNFS_I(inode);
    ternfs_debug("evict enode=%p", enode);
    if (S_ISDIR(inode->i_mode)) {
        ternfs_dir_drop_cache(enode);
    } else if (S_ISREG(inode->i_mode)) {
        // We need to clear the spans
        ternfs_free_file_spans(&enode->file.spans);
    }
    truncate_inode_pages(&inode->i_data, 0);
    clear_inode(inode);
}

void ternfs_inode_free(struct inode* inode) {

    struct ternfs_inode* enode = TERNFS_I(inode);
    ternfs_debug("enode=%p", enode);
    kmem_cache_free(ternfs_inode_cachep, enode);
}

static void ternfs_inode_init_once(void* ptr) {
    struct ternfs_inode* enode = (struct ternfs_inode*)ptr;

    inode_init_once(&enode->inode);
}

int __init ternfs_inode_init(void) {
    ternfs_inode_cachep = kmem_cache_create(
        "ternfs_inode_cache", sizeof(struct ternfs_inode),
        0, SLAB_RECLAIM_ACCOUNT, ternfs_inode_init_once
    );
    if (!ternfs_inode_cachep) { return -ENOMEM; }

    return 0;
}

void __cold ternfs_inode_exit(void) {
    ternfs_debug("inode exit");
    rcu_barrier();
    kmem_cache_destroy(ternfs_inode_cachep);
}

// Returns:
// *  0: the async getattr was not sent, because of contention
// *  1: the async getattr was sent
// * -n: error code
int ternfs_start_async_getattr(struct ternfs_inode* enode) {
    ternfs_debug("enode=%p id=0x%016lx mtime=%lld getattr_expiry=%lld", enode, enode->inode.i_ino, enode->mtime, enode->getattr_expiry);

    int ret = 0;
    u64 seqno = 0;
    u64 init_seqno = 0;
    if (ternfs_latch_try_acquire(&enode->getattr_update_latch, seqno)) {
        // it is not safe to pass enode->getattr_async_seqno directly to ternfs_latch_try_acquire as subsequent calls while lock is held
        // overwrite the seqno we need for release causing a deadlock
        enode->getattr_async_seqno = seqno;
        // This latch is guaranteeing completion does not finish and free update_latch before we are finished with it
        // As we didn't schedule completion yet no one could be holding it
        BUG_ON(!ternfs_latch_try_acquire(&enode->getattr_update_init_latch, init_seqno));
        ihold(&enode->inode); // we need the request until we're done
        // Schedule the work immediately, so that we can't end up in the situation where
        // the async completes before we schedule the work and we run the complete twice.
        BUG_ON(!schedule_delayed_work(&enode->getattr_async_work, ternfs_initial_shard_timeout_jiffies));


        u64 ts = get_jiffies_64();
        if (smp_load_acquire(&enode->getattr_expiry) > ts) {
            ternfs_debug("getattr fresh enough, skipping");
            goto out;
        }

        if (S_ISDIR(enode->inode.i_mode)) {
            ret = ternfs_shard_async_getattr_dir(
                (struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info,
                &enode->getattr_async_req,
                enode->inode.i_ino
            );
            if (ret) { goto out; }
        } else {
            if (enode->file.status == TERNFS_FILE_STATUS_READING || enode->file.status == TERNFS_FILE_STATUS_NONE) {
                ret = ternfs_shard_async_getattr_file(
                    (struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info,
                    &enode->getattr_async_req,
                    enode->inode.i_ino
                );
                if (ret) { goto out; }
            } else {
                ternfs_debug("skipping async getattr for non-reading file");
                goto out;
            }
        }
        ret = 1;
    } else {
        // no goto out -- we don't hold the latch
        return ret;
    }

out:
    //we are done with init, relase the init latch
    ternfs_latch_release(&enode->getattr_update_init_latch, init_seqno);
    if (ret <= 0) {
        // The timeout might have already ran, in which case it'll be the one
        // releasing the latch.
        bool was_pending = cancel_delayed_work_sync(&enode->getattr_async_work);
        if (was_pending) {
            iput(&enode->inode);
            ternfs_latch_release(&enode->getattr_update_latch, enode->getattr_async_seqno);
        }
    }
    return ret;
}

static void getattr_async_complete(struct work_struct* work) {
    struct ternfs_inode* enode = container_of(to_delayed_work(work), struct ternfs_inode, getattr_async_work);
    ternfs_debug("enode=%p id=0x%016lx mtime=%lld getattr_expiry=%lld", enode, enode->inode.i_ino, enode->mtime, enode->getattr_expiry);

    u64 seqno = 0;
    if(!ternfs_latch_try_acquire(&enode->getattr_update_init_latch, seqno)) {
        // wait until ternfs_do_getattr is finished with init
        ternfs_latch_wait(&enode->getattr_update_init_latch, seqno);
        // we are holding getattr_update_latch no one will acquire init latch before we release it
        BUG_ON(!ternfs_latch_try_acquire(&enode->getattr_update_init_latch, seqno));
    }
    // we might as well release it now, we are holding getattr_update_latch
    ternfs_latch_release(&enode->getattr_update_init_latch, seqno);

    // we need to remove the request first before checking if we have buffer or not otherwise buffer could be set after we check and we would leak buffer
    ternfs_metadata_remove_request(&((struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info)->sock, enode->getattr_async_req.request_id);

    // if we have a buffer, we're done, otherwise it's a timeout
    if (enode->getattr_async_req.skb) {
        int err;
        // This is consumed by the parse functions below. You need to make sure that those
        // functions run or consume it yourself.
        struct sk_buff* skb = enode->getattr_async_req.skb;
        enode->getattr_async_req.skb = NULL;
        u64 mtime;
        u64 expiry;
        bool has_atime = false;
        u64 atime;
        if (S_ISDIR(enode->inode.i_mode)) {
            u64 owner;
            struct ternfs_policy_body block_policy;
            struct ternfs_policy_body span_policy;
            struct ternfs_policy_body stripe_policy;
            struct ternfs_policy_body snapshot_policy;
            err = ternfs_error_to_linux(ternfs_shard_parse_getattr_dir(skb, &mtime, &owner, &block_policy, &span_policy, &stripe_policy, &snapshot_policy));
            if (err == 0) {
                if (block_policy.len) {
                    enode->block_policy = ternfs_upsert_policy(enode->inode.i_ino, BLOCK_POLICY_TAG, block_policy.body, block_policy.len);
                }
                if (span_policy.len) {
                    enode->span_policy = ternfs_upsert_policy(enode->inode.i_ino, SPAN_POLICY_TAG, span_policy.body, span_policy.len);
                }
                if (stripe_policy.len) {
                    enode->stripe_policy = ternfs_upsert_policy(enode->inode.i_ino, STRIPE_POLICY_TAG, stripe_policy.body, stripe_policy.len);
                }
                if (snapshot_policy.len) {
                    enode->snapshot_policy = ternfs_upsert_policy(enode->inode.i_ino, SNAPSHOT_POLICY_TAG, snapshot_policy.body, snapshot_policy.len);
                }
                expiry = get_jiffies_64() + ternfs_dir_getattr_refresh_time_jiffies;
            }
        } else {
            u64 size;
            err = ternfs_shard_parse_getattr_file(skb, &mtime, &atime, &size);
            has_atime = true;
            if (err == TERNFS_ERR_FILE_NOT_FOUND && enode->file.status == TERNFS_FILE_STATUS_NONE) { // probably just created
                enode->inode.i_size = 0;
                enode->inode.i_blocks = 0;
                expiry = 0;
                mtime = 0;
            } else if (err == 0) {
                enode->inode.i_size = size;
                enode->inode.i_blocks = DIV_ROUND_UP(size, POSIX_BLK_SIZE);
                expiry = get_jiffies_64() + ternfs_file_getattr_refresh_time_jiffies;
            }
            err = ternfs_error_to_linux(err);
        }

        if (err) {
            ternfs_info("could not perform async stat to 0x%016lx: %d", enode->inode.i_ino, err);
        } else {
            WRITE_ONCE(enode->mtime, mtime);
            inode_set_mtime(&enode->inode, mtime / 1000000000, mtime % 1000000000);
            inode_set_ctime(&enode->inode, mtime / 1000000000, mtime % 1000000000);
            if (has_atime) {
                inode_set_atime(&enode->inode, atime / 1000000000, atime % 1000000000);
            }
            ternfs_debug("id=%016lx new_expiry=%llu", enode->inode.i_ino, expiry);
            smp_store_release(&enode->getattr_expiry, expiry);
        }
    }

    // And put inode, release latch ordering is not important in this case but it's good practice to release references/locks in reverse order of acquisition
    iput(&enode->inode);
    ternfs_latch_release(&enode->getattr_update_latch, enode->getattr_async_seqno);
}

int ternfs_do_getattr(struct ternfs_inode* enode, int cache_timeout_type) {
    int err;
    s64 seqno;

    // progress: whoever wins the lock won't try again
    for (;;) {
        ternfs_debug("enode=%p id=0x%016lx mtime=%lld getattr_expiry=%lld", enode, enode->inode.i_ino, enode->mtime, enode->getattr_expiry);

        // we're still managing this file, nothing to do
        if (!S_ISDIR(enode->inode.i_mode) && enode->file.status == TERNFS_FILE_STATUS_WRITING) {
            return 0;
        }

        if (ternfs_latch_try_acquire(&enode->getattr_update_latch, seqno)) {
            u64 ts = get_jiffies_64();
            switch (cache_timeout_type) {
            case ATTR_CACHE_NORM_TIMEOUT:
                if (smp_load_acquire(&enode->getattr_expiry) > ts) { err = 0; goto out; }
                break;
            case ATTR_CACHE_DIR_TIMEOUT:
                BUG_ON(!S_ISDIR(enode->inode.i_mode));
                if (smp_load_acquire(&enode->dir.mtime_expiry) > ts) { err = 0; goto out; }
                break;
            case ATTR_CACHE_NO_TIMEOUT:
                break;
            default:
                ternfs_error("unknown cache timeout type %d", cache_timeout_type);
                BUG();
            }

            u64 mtime;
            u64 expiry;
            bool has_atime = false;
            u64 atime;
            if (S_ISDIR(enode->inode.i_mode)) {
                u64 owner;
                struct ternfs_policy_body block_policy;
                struct ternfs_policy_body span_policy;
                struct ternfs_policy_body stripe_policy;
                struct ternfs_policy_body snapshot_policy;
                err = ternfs_shard_getattr_dir(
                    (struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info,
                    enode->inode.i_ino,
                    &mtime,
                    &owner,
                    &block_policy,
                    &span_policy,
                    &stripe_policy,
                    &snapshot_policy
                );
                if (err == 0) {
                    if (block_policy.len) {
                        enode->block_policy = ternfs_upsert_policy(enode->inode.i_ino, BLOCK_POLICY_TAG, block_policy.body, block_policy.len);
                    }
                    if (span_policy.len) {
                        enode->span_policy = ternfs_upsert_policy(enode->inode.i_ino, SPAN_POLICY_TAG, span_policy.body, span_policy.len);
                    }
                    if (stripe_policy.len) {
                        enode->stripe_policy = ternfs_upsert_policy(enode->inode.i_ino, STRIPE_POLICY_TAG, stripe_policy.body, stripe_policy.len);
                    }
                    if (snapshot_policy.len) {
                        enode->snapshot_policy = ternfs_upsert_policy(enode->inode.i_ino, SNAPSHOT_POLICY_TAG, snapshot_policy.body, snapshot_policy.len);
                    }
                    expiry = get_jiffies_64() + ternfs_dir_getattr_refresh_time_jiffies;
                }
            } else {
                if (enode->file.status == TERNFS_FILE_STATUS_READING || enode->file.status == TERNFS_FILE_STATUS_NONE) {
                    ternfs_debug("updating getattr for reading file");
                    u64 size;
                    err = ternfs_shard_getattr_file(
                        (struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info,
                        enode->inode.i_ino,
                        &mtime,
                        &atime,
                        &size
                    );
                    has_atime = true;
                    if (err == TERNFS_ERR_FILE_NOT_FOUND && enode->file.status == TERNFS_FILE_STATUS_NONE) { // probably just created
                        enode->inode.i_size = 0;
                        expiry = 0;
                        mtime = 0;
                    } else if (err == 0) {
                        enode->inode.i_size = size;
                        // This is not correct physical size, in order to calculate physical size we would need to get span information as
                        // our replication policy is per span. This is quite expensive to do for stat.
                        // We could keep this value calcualated and stored on shards but it's not worth it at this point.
                        enode->inode.i_blocks = DIV_ROUND_UP(size, POSIX_BLK_SIZE);
                        expiry = get_jiffies_64() + ternfs_file_getattr_refresh_time_jiffies;
                    }
                } else {
                    BUG_ON(enode->file.status != TERNFS_FILE_STATUS_WRITING);
                }
            }
            if (err) { err = ternfs_error_to_linux(err); goto out; }
            else {
                WRITE_ONCE(enode->mtime, mtime);
                inode_set_mtime(&enode->inode, mtime / 1000000000, mtime % 1000000000);
                inode_set_ctime(&enode->inode, mtime / 1000000000, mtime % 1000000000);
                if (has_atime) {
                    inode_set_atime(&enode->inode, atime / 1000000000, atime % 1000000000);
                }
            }

            if (S_ISDIR(enode->inode.i_mode)) {
                smp_store_release(&enode->dir.mtime_expiry, get_jiffies_64() + ternfs_dir_dentry_refresh_time_jiffies);
            }
            smp_store_release(&enode->getattr_expiry, expiry);

out:
            ternfs_latch_release(&enode->getattr_update_latch, seqno);
            ternfs_debug("out mtime=%llu getattr_expiry=%llu", enode->mtime, enode->getattr_expiry);
            return err;
        } else {
            long ret = ternfs_latch_wait_timeout(&enode->getattr_update_latch, seqno, 2 * ternfs_overall_shard_timeout_jiffies);
            if (unlikely(ret < 1)) {
                ternfs_warn("latch_wait timed out, this should never happen, getattr stuck? id=0x%016lx getattr_async_seqno=%lld counter=%lld seqno=%lld", enode->inode.i_ino, enode->getattr_async_seqno, atomic64_read(&enode->getattr_update_latch.counter), seqno);
                return -EDEADLK;
            }
        }
    }
}

static struct ternfs_inode* ternfs_create_internal(struct inode* parent, int itype, struct dentry* dentry) {
    int err;

    BUG_ON(dentry->d_inode);

    if (dentry->d_name.len > TERNFS_MAX_FILENAME) { return ERR_PTR(-ENAMETOOLONG); }

    // https://internal-repo/blob/main/docs/kmod-file-tracking.md
    // This (should) happen only from NFSd. Right now we only re-export on NFS for
    // this reason, so we should never hit this.
    preempt_disable();
    struct task_struct* owner = current->group_leader;
    struct mm_struct* mm = owner->mm;
    if (mm == NULL) {
        preempt_enable();
        ternfs_warn("current->group_leader->mm = NULL, called from kernel thread?");
        return ERR_PTR(-EIO);
    }
    // We might need the mm beyond the file lifetime, to clear up block write pages MM_FILEPAGES
    mmgrab(mm);
    preempt_enable();

    struct ternfs_inode* parent_enode = TERNFS_I(parent);

    ternfs_debug("creating %*s in %016lx", dentry->d_name.len, dentry->d_name.name, parent->i_ino);

    u64 ino, cookie;
    err = ternfs_error_to_linux(ternfs_shard_create_file(
        (struct ternfs_fs_info*)parent->i_sb->s_fs_info, ternfs_inode_shard(parent->i_ino),
        itype, dentry->d_name.name, dentry->d_name.len, &ino, &cookie
    ));
    if (err) {
        err = ternfs_error_to_linux(err);
        goto out_err;
    }
    // make this dentry stick around?

    // new_inode
    struct inode* inode = ternfs_get_inode_normal(parent->i_sb, parent_enode, ino);
    if (IS_ERR(inode)) {
        err = PTR_ERR(inode);
        goto out_err;
    }
    struct ternfs_inode* enode = TERNFS_I(inode);

    enode->file.status = TERNFS_FILE_STATUS_WRITING;
    enode->inode.i_size = 0;

    // Initialize all the transient specific fields
    enode->file.cookie = cookie;
    atomic_set(&enode->file.transient_err, 0);
    enode->file.writing_span = NULL;
    sema_init(&enode->file.flushing_span_sema, 1); // 1 = free to flush
    enode->file.owner = owner;
    enode->file.mm = mm;

    return enode;

out_err:
    mmdrop(mm);
    return ERR_PTR(err);
}

static int COMPAT_FUNC_UNS_IMP(ternfs_create, struct inode* parent, struct dentry* dentry, umode_t mode, bool excl) {
    struct ternfs_inode* enode = ternfs_create_internal(parent, TERNFS_INODE_FILE, dentry);
    if (IS_ERR(enode)) { return PTR_ERR(enode); }

    // This is not valid yet, we need to fill it in first
    d_instantiate(dentry, &enode->inode);
    d_invalidate(dentry);

    return 0;
}

// vfs: refcount of path->dentry
static int COMPAT_FUNC_UNS_IMP(ternfs_getattr, const struct path* path, struct kstat* stat, u32 request_mask, unsigned int query_flags) {
    struct inode* inode = d_inode(path->dentry);
    struct ternfs_inode* enode = TERNFS_I(inode);

    trace_eggsfs_vfs_getattr_enter(inode);

    // >= so that ternfs_dir_refresh_time=0 causes revalidation at every call to this function
    if (get_jiffies_64() >= smp_load_acquire(&enode->getattr_expiry)) {
        int err;

        // FIXME: symlinks
        // if requests_mask is 0 then this is originating from stat() call and not statx and we should fill in basic stats
        if (request_mask && !(request_mask & STATX_MTIME) && (S_ISDIR(inode->i_mode) || !(request_mask & (STATX_ATIME | STATX_SIZE | STATX_BLOCKS)))) {
            goto done;
        }

        trace_eggsfs_vfs_getattr_lock(inode);

        // dentry refcount also protects the inode (e.g. d_delete will not turn used dentry into a negative one),
        // so no need to grab anything before we start waiting for stuff
        err = ternfs_do_getattr(enode, ATTR_CACHE_NORM_TIMEOUT);
        if (err) {
            trace_eggsfs_vfs_getattr_exit(inode, err);
            return err;
        }
    }

done:
#if LINUX_VERSION_CODE < KERNEL_VERSION(6,6,0)
    COMPAT_FUNC_UNS_CALL(generic_fillattr, inode, stat);
#else
    COMPAT_FUNC_UNS_CALL(generic_fillattr, request_mask, inode, stat);
#endif
    trace_eggsfs_vfs_getattr_exit(inode, 0);
    return 0;
}

static int ternfs_do_ftruncate(struct dentry* dentry, struct iattr* attr) {
    struct inode* inode = dentry->d_inode;
    struct ternfs_inode* enode = TERNFS_I(inode);

    if (ternfs_disable_ftruncate) { return -ENOSYS; }

    BUG_ON(!inode_is_locked(inode));

    if (smp_load_acquire(&enode->file.status) != TERNFS_FILE_STATUS_WRITING) {
        return -EINVAL;
    }

    loff_t ppos = enode->inode.i_size;
    loff_t epos = attr->ia_size;

    if (epos < ppos) {
        ternfs_debug("refusing ftruncate on pos %lld smaller than file size %lld", epos, ppos);
        return -EINVAL;
    }
    if (epos == ppos) {
        return 0;
    }

    // any attempt to write after ftruncate() will still result in EINVAL
    while (ppos < epos) {
        ternfs_debug("calling file write with %lld %lld", ppos, epos);
        ssize_t written = ternfs_file_write_internal(enode, 0, &ppos, NULL, epos - ppos);
        ternfs_debug("wrriten %ld", written);
        if (unlikely(written < 0)) {
            return written;
        }
    }

    return 0;
}

static int COMPAT_FUNC_UNS_IMP(ternfs_setattr, struct dentry* dentry, struct iattr* attr) {
    // https://elixir.bootlin.com/linux/v5.4/source/fs/open.c#L49 is the only
    // place where ATTR_SIZE is set, which means only when truncating.
    if (attr->ia_valid & ATTR_SIZE) {
        // ftruncate() is only allowed to EXTEND the transient files.
        // We are called here through https://elixir.bootlin.com/linux/v5.4/source/fs/open.c#L64
        // which fills the `newattrs` variable from scratch, setting ctime in
        // https://elixir.bootlin.com/linux/v5.4/source/fs/attr.c#L268.
        // Ideally we would continue the execution here and send the SET_TIME
        // request, but it is not supported for transient files and instead
        // ternfs_file_write_internal will modify mtime/ctime if needed.
        return ternfs_do_ftruncate(dentry, attr);
    }

    // ATTR_TIMES_SET/ATTR_TOUCH is just to distinguish between a touch
    // and an explicit time set
    // Note that `utimes_common` unconditionally sets ATTR_CTIME, but we
    // can't do much with it.
    if (attr->ia_valid & ~(ATTR_ATIME|ATTR_ATIME_SET|ATTR_MTIME|ATTR_MTIME_SET|ATTR_TIMES_SET|ATTR_CTIME|ATTR_TOUCH)) {
        ternfs_debug("skipping setattr, ia_valid=%08x", attr->ia_valid);
        return -EPERM;
    }

    uint64_t now = ktime_get_real_ns();

    uint64_t atime = 0;
    if (attr->ia_valid & ATTR_ATIME) {
        if (attr->ia_valid & ATTR_ATIME_SET) {
            atime = (attr->ia_atime.tv_sec * 1000000000ll) + attr->ia_atime.tv_nsec;
        } else {
            atime = now;
        }
        ternfs_debug("setting atime to %llu", atime);
        atime |= 1ull << 63;
    }

    uint64_t mtime = 0;
    if (attr->ia_valid & ATTR_MTIME) {
        if (attr->ia_valid & ATTR_MTIME_SET) {
            mtime = (attr->ia_mtime.tv_sec * 1000000000ll) + attr->ia_mtime.tv_nsec;
        } else {
            mtime = now;
        }
        ternfs_debug("setting mtime to %llu", mtime);
        mtime |= 1ull << 63;
    }

    int err = ternfs_shard_set_time((struct ternfs_fs_info*)dentry->d_inode->i_sb->s_fs_info, dentry->d_inode->i_ino, mtime, atime);
    if (err) { return err; }

    // could copy out attributes instead?
    smp_store_release(&TERNFS_I(dentry->d_inode)->getattr_expiry, 0);

    return 0;
}

static int COMPAT_FUNC_UNS_IMP(ternfs_symlink, struct inode* dir, struct dentry* dentry, const char* path) {
    struct ternfs_inode* enode = ternfs_create_internal(dir, TERNFS_INODE_SYMLINK, dentry);
    if (IS_ERR(enode)) { return PTR_ERR(enode); }

    enode->file.status = TERNFS_FILE_STATUS_WRITING; // needed for flush to flush below

    size_t len = strlen(path);

    // We now need to write out the symlink contents...
    loff_t ppos = 0;
    struct kvec vec;
    struct iov_iter from;
    vec.iov_base = (void*)path;
    vec.iov_len = len;
    iov_iter_kvec(&from, WRITE, &vec, 1, vec.iov_len);
    inode_lock(&enode->inode);
    int err = ternfs_file_write(enode, 0, &ppos, &from);
    inode_unlock(&enode->inode);
    if (err < 0) { return err; }
    // ...and flush them
    err = ternfs_file_flush(enode, dentry);
    if (err < 0) { return err; }

    // Now we link the dentry in
    d_instantiate(dentry, &enode->inode);

    return 0;
}

static const char* ternfs_get_link(struct dentry* dentry, struct inode* inode, struct delayed_call* destructor) {
    // Can't be bothered to think about RCU
    if (dentry == NULL) { return ERR_PTR(-ECHILD); }

    struct ternfs_inode* enode = TERNFS_I(inode);
    char* buf = ternfs_read_link(enode);

    if (IS_ERR(buf)) { return buf; }

    destructor->fn = ternfs_link_destructor;
    destructor->arg = buf;

    return buf;
}

static const struct inode_operations ternfs_dir_inode_ops = {
    .create = ternfs_create,
    .lookup = ternfs_lookup,
    .unlink = ternfs_unlink,
    .mkdir = ternfs_mkdir,
    .rmdir = ternfs_rmdir,
    .rename = ternfs_rename,
    .getattr = ternfs_getattr,
    .symlink = ternfs_symlink,
};

static const struct inode_operations ternfs_file_inode_ops = {
    .getattr = ternfs_getattr,
    .setattr = ternfs_setattr,
};

static const struct inode_operations ternfs_symlink_inode_ops = {
    .getattr = ternfs_getattr,
    .setattr = ternfs_setattr,
    .get_link = ternfs_get_link,
};

extern struct file_operations ternfs_dir_operations;

struct inode* ternfs_get_inode(
    struct super_block* sb,
    bool allow_no_parent,
    struct ternfs_inode* parent,
    u64 ino
) {
    trace_eggsfs_get_inode_enter(ino);

    struct ternfs_fs_info* fs_info = (struct ternfs_fs_info*)sb->s_fs_info;

    struct inode* inode = iget_locked(sb, ino); // new_inode when possible?
    if (!inode) {
        trace_eggsfs_get_inode_exit(ino, NULL, false, 0);
        return ERR_PTR(-ENOMEM);
    }
    struct ternfs_inode* enode = TERNFS_I(inode);

    bool new = inode->i_state & I_NEW;
    if (new) {
        switch (ternfs_inode_type(ino)) {
        case TERNFS_INODE_DIRECTORY: inode->i_mode = S_IFDIR; break;
        case TERNFS_INODE_FILE: inode->i_mode = S_IFREG; break;
        case TERNFS_INODE_SYMLINK: inode->i_mode = S_IFLNK; break;
        default: BUG();
        }
        inode->i_mode |= 0777 & ~(S_ISDIR(inode->i_mode) ? fs_info->dmask : fs_info->fmask);
        inode->i_blocks = 0;
        inode->i_size = 0;
        inode_set_mtime(inode, 0, 0);
        inode_set_ctime(inode, 0, 0);
        inode_set_atime(inode, 0, 0);

        inode->i_op = NULL;
        inode->i_fop = NULL;
        enode->getattr_expiry = 0;
        enode->mtime = 0;
        enode->edge_creation_time = 0;
        if (S_ISDIR(inode->i_mode)) {
            inode->i_op = &ternfs_dir_inode_ops;
            inode->i_fop = &ternfs_dir_operations;

            rcu_assign_pointer(enode->dir.dirents, NULL);
            ternfs_latch_init(&enode->dir.dirents_latch);
            enode->dir.mtime_expiry = 0;
        } else if (S_ISREG(inode->i_mode) || S_ISLNK(inode->i_mode)) {
            if (unlikely(S_ISLNK(inode->i_mode))) {
                inode->i_op = &ternfs_symlink_inode_ops;
            } else {
                inode->i_op = &ternfs_file_inode_ops;
            }
            inode->i_fop = &ternfs_file_operations;
            inode->i_mapping->a_ops = &ternfs_mmap_operations;
            enode->file.status = TERNFS_FILE_STATUS_NONE;

            // Init normal file stuff -- that's always there. The transient
            // file stuff is only filled in if needed.
            ternfs_init_file_spans(&enode->file.spans, ino);
        }

        inode->i_uid = fs_info->uid;
        inode->i_gid = fs_info->gid;

        // Only == NULL when we're getting the root inode,
        // unless we allow no parent, which is only in the
        // case of NFS exports. In that case we never write
        // files anyway, so not having the right policy does
        // not matter. Ofc we could re-export a mount that
        // is also written to, but this is an edge case that
        // we do not cater to very well for now. The proper
        // solution is to store the correct policy on
        // `d_splice_alias`, but we can only do that if we
        // also remember if the current enode policy is its
        // own or not. In fact, I just realized that `ternfs_rename`
        // is wrong: it unconditionally sets the policy of the
        // child directory to that of the parent, but that's
        // not right, it should do that _only if the child
        // policy is inherited_. We should store a bit that tells
        // us whether it is inherited, and then use it
        // on `d_splice_alias` and `d_move`.
        if (parent) {
            enode->block_policy = parent->block_policy;
            enode->span_policy = parent->span_policy;
            enode->stripe_policy = parent->stripe_policy;
            enode->snapshot_policy = parent->snapshot_policy;
        } else {
            bool is_root = ino == TERNFS_ROOT_INODE;
            BUG_ON(!allow_no_parent && !is_root);
            if (is_root) { // we've just created the root inode, policy will be filled in later
                enode->block_policy = NULL;
                enode->span_policy = NULL;
                enode->stripe_policy = NULL;
                enode->snapshot_policy = NULL;
            } else {
                struct ternfs_inode* root = TERNFS_I(sb->s_root->d_inode);
                enode->block_policy = root->block_policy;
                enode->span_policy = root->span_policy;
                enode->stripe_policy = root->stripe_policy;
                enode->snapshot_policy = root->snapshot_policy;
            }
        }

        unlock_new_inode(inode);
    }

    trace_eggsfs_get_inode_exit(ino, inode, new, 0);

    return inode;
}
