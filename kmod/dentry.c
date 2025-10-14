// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "dentry.h"

#include "bincode.h"
#include "inode_compat.h"
#include "log.h"
#include "dir.h"
#include "err.h"
#include "inode.h"
#include "metadata.h"
#include "policy.h"
#include "trace.h"

TERNFS_DEFINE_COUNTER(ternfs_stat_dir_revalidations);

static inline int ternfs_dir_needs_reval(struct ternfs_inode* dir, struct dentry* dentry, bool in_rcu) {
    // 0 => invalid
    // -ECHILD => revalidate parent dir
    // 1 => valid
    u64 dentry_dir_mtime = dentry->d_time;
    int ret;
    u64 t = get_jiffies_64();
    if (t >= dir->dir.mtime_expiry) { ret = -ECHILD; }
    else if (dentry_dir_mtime != dir->mtime) {
        struct inode* inode;
        if (in_rcu) {
            inode = d_inode_rcu(dentry);
        } else {
            inode = dentry->d_inode;
        }
        // We are fine invalidating negative or dentries for non-directories
        // as this is not expensive
        if (inode == NULL || !S_ISDIR(inode->i_mode)) { ret = 0; }
        // we want to confirm we are still valid but we need  to do lookup
        // so we need to drop into non-rcu mode
        else if (in_rcu) { ret = -ECHILD; }
        else {
            u64 ino, creation_time;
            if (ternfs_shard_lookup(
                    inode->i_sb->s_fs_info, dir->inode.i_ino,
                    dentry->d_name.name, dentry->d_name.len,
                    &ino, &creation_time) < 0)
            {
                ret = 0;
            } else {
                if (inode->i_ino != ino) {
                    ret = 0;
                } else {
                    dentry->d_time = dir->mtime;
                    ret = 1;
                }
            }
        }
    }
    else { ret = 1; }

    ternfs_debug(
        "t=%llu dir=%p dir_id=0x%016lx dir_mtime=%llu dir_mtime_expiry=%llu dentry=%pd dentry_dir_mtime=%llu -> ret=%d",
        t, dir, dir->inode.i_ino, dir->mtime, dir->dir.mtime_expiry, dentry, dentry_dir_mtime, ret
    );

    return ret;
}

static int ternfs_d_revalidate(struct dentry* dentry, unsigned int flags) {
    struct dentry* parent;
    struct inode* dir;
    int ret;

    ternfs_debug("dentry=%pd flags=%x[rcu=%d]", dentry, flags, !!(flags & LOOKUP_RCU));

    if (IS_ROOT(dentry)) { return 1; }

    if (dentry->d_name.len > TERNFS_MAX_FILENAME) {
        return -ENAMETOOLONG;
    }

    if (flags & LOOKUP_RCU) {
        parent = READ_ONCE(dentry->d_parent);
        dir = d_inode_rcu(parent);
        if (!dir) { return -ECHILD; }
        // we are either asked for validiti of our root dentry or we are bound mounted
        // either way we can't return invalid
        if (parent->d_sb != dentry->d_sb) { return 1; }
        ret = ternfs_dir_needs_reval(TERNFS_I(dir), dentry, true);
        if (parent != READ_ONCE(dentry->d_parent)) { return -ECHILD; }
        return ret;
    } else {
        parent = dget_parent(dentry);
        dir = d_inode(parent);
        ret = ternfs_dir_needs_reval(TERNFS_I(dir), dentry, false);
        if (ret == -ECHILD) {
            ternfs_counter_inc(ternfs_stat_dir_revalidations);
            ret = ternfs_dir_revalidate(TERNFS_I(dir));
            if (ret) { goto out; }
            ret = ternfs_dir_needs_reval(TERNFS_I(dir), dentry, false);
            if (ret == -ECHILD) { ret = 1; }
        }
out:
        dput(parent);
    }
    return ret;
}

struct dentry_operations ternfs_dentry_ops = {
    .d_revalidate = ternfs_d_revalidate,
};


static void ternfs_dcache_handle_error(struct inode* dir, struct dentry* dentry, int err) {
    ternfs_debug("err=%d", err);
    switch (err) {
    case TERNFS_ERR_CANNOT_OVERRIDE_NAME:
        WARN_ON(dentry->d_inode);
        trace_eggsfs_dcache_invalidate_neg_entry(dir, dentry);
        WRITE_ONCE(TERNFS_I(dir)->dir.mtime_expiry, 0);
        d_invalidate(dentry);
        break;
    // TODO is this correct? verify, I've changed the code from an earlier error
    // which does not exist anymore.
    case TERNFS_ERR_DIRECTORY_NOT_FOUND:
        WARN_ON(dentry->d_inode);
        trace_eggsfs_dcache_delete_inode(dir);
        WRITE_ONCE(TERNFS_I(dir)->dir.mtime_expiry, 0);
        clear_nlink(dir);
        d_invalidate(dentry->d_parent);
        break;
    case TERNFS_ERR_DIRECTORY_NOT_EMPTY:
        WARN_ON(!dentry->d_inode);
        if (dentry->d_inode) {
            trace_eggsfs_dcache_invalidate_dir(dentry->d_inode);
            WRITE_ONCE(TERNFS_I(dentry->d_inode)->dir.mtime_expiry, 0);
        }
        break;
    case TERNFS_ERR_NAME_NOT_FOUND:
        WARN_ON(!dentry->d_inode);
        if (dentry->d_inode) {
            trace_eggsfs_dcache_delete_entry(dir, dentry, dentry->d_inode);
            WRITE_ONCE(TERNFS_I(dentry->d_inode)->dir.mtime_expiry, 0);
            clear_nlink(dentry->d_inode);
            d_invalidate(dentry);
        }
        break;
    case TERNFS_ERR_MISMATCHING_TARGET:
    case TERNFS_ERR_MISMATCHING_CREATION_TIME:
        WARN_ON(!dentry->d_inode);
        if (dentry->d_inode) {
            trace_eggsfs_dcache_invalidate_entry(dir, dentry, dentry->d_inode);
            WRITE_ONCE(TERNFS_I(dentry->d_inode)->dir.mtime_expiry, 0);
            d_invalidate(dentry);
        }
        break;
    }
}

// vfs: shared dir.i_rwsem
struct dentry* ternfs_lookup(struct inode* dir, struct dentry* dentry, unsigned int flags) {
    ternfs_debug("dir=0x%016lx, name=%*pE", dir->i_ino, dentry->d_name.len, dentry->d_name.name);
    int err;

    trace_eggsfs_vfs_lookup_enter(dir, dentry);

    if (dentry->d_name.len > TERNFS_MAX_FILENAME) {
        err = -ENAMETOOLONG;
        goto out_err;
    }

    u64 ino, creation_time;
    // need to keep the tern error around for the TERNFS_ERR_NAME_NOT_FOUND
    err = ternfs_shard_lookup(
        dir->i_sb->s_fs_info, dir->i_ino, dentry->d_name.name, dentry->d_name.len, &ino, &creation_time
    );
    if (unlikely(err < 0)) {
        goto out_err;
    }
    if (err) { // not unlikely since vfs will do lookups for entries likely to not exist
        if (err == TERNFS_ERR_NAME_NOT_FOUND) {
            dentry->d_time = TERNFS_I(dir)->mtime;
            trace_eggsfs_vfs_lookup_exit(dir, dentry, NULL, 0);
            return d_splice_alias(NULL, dentry);
        }
        err = ternfs_error_to_linux(err);
        goto out_err;
    }

    struct inode* inode = ternfs_get_inode_normal(dentry->d_sb, TERNFS_I(dir), ino);
    if (IS_ERR(inode)) {
        err = PTR_ERR(inode);
        goto out_err;
    }
    inode->i_ino = ino;
    TERNFS_I(inode)->edge_creation_time = creation_time;

    dentry->d_time = TERNFS_I(dir)->mtime;

    trace_eggsfs_vfs_lookup_exit(dir, dentry, inode, 0);
    return d_splice_alias(inode, dentry);

out_err:
    trace_eggsfs_vfs_lookup_exit(dir, dentry, NULL, err);
    return ERR_PTR(err);
}

// vfs: exclusive dir.i_rwsem, lockref dentry
int COMPAT_FUNC_UNS_IMP(ternfs_mkdir, struct inode* dir, struct dentry* dentry, umode_t mode) {
    int err;

    struct ternfs_inode* dir_enode = TERNFS_I(dir);

    trace_eggsfs_vfs_mkdir_enter(dir, dentry);
    BUG_ON(dentry->d_inode);

    if (dentry->d_name.len > TERNFS_MAX_FILENAME) {
        err = -ENAMETOOLONG;
        goto out_err;
    }

    u64 ino, creation_time;
    err = ternfs_cdc_mkdir(
        dir->i_sb->s_fs_info,
        dir->i_ino,
        dentry->d_name.name,
        dentry->d_name.len,
        &ino,
        &creation_time
    );
    if (unlikely(err)) {
        if (err < 0) { goto out_err; }
        ternfs_dcache_handle_error(dir, dentry, err);
        err = ternfs_error_to_linux(err);
        goto out_err;
    }
    ternfs_dir_drop_cache(TERNFS_I(dir));

    struct inode* inode = ternfs_get_inode_normal(dentry->d_sb, TERNFS_I(dir), ino);
    if (IS_ERR(inode)) { err = PTR_ERR(inode); goto out_err; }
    struct ternfs_inode* enode = TERNFS_I(inode);
    enode->edge_creation_time = creation_time;
    dentry->d_time = dir_enode->mtime;
    d_instantiate(dentry, inode);

    trace_eggsfs_vfs_mkdir_exit(dir, dentry, inode, 0);
    return 0;

out_err:
    trace_eggsfs_vfs_mkdir_exit(dir, dentry, NULL, err);
    return err;
}

// vfs: exclusive dir,d_entry->d_inode.i_rwsem
int ternfs_rmdir(struct inode* dir, struct dentry* dentry) {
    int err;

    trace_eggsfs_vfs_rmdir_enter(dir, dentry, dentry->d_inode);
    BUG_ON(!dentry->d_inode);

    if (unlikely(dentry->d_name.len > TERNFS_MAX_FILENAME)) {
        pr_err("ternfs: ternfs_rmdir dir=%lu dentry=%pd exceed max filename length %d\n", dir->i_ino, dentry, TERNFS_MAX_FILENAME);
        err = -ENAMETOOLONG;
        goto out_err;
    }

    err = ternfs_cdc_rmdir((struct ternfs_fs_info*)dir->i_sb->s_fs_info, dir->i_ino, dentry->d_inode->i_ino, TERNFS_I(dentry->d_inode)->edge_creation_time, dentry->d_name.name, dentry->d_name.len);
    if (unlikely(err)) {
        if (err < 0) { goto out_err; }
        ternfs_dcache_handle_error(dir, dentry, err);
        err = ternfs_error_to_linux(err);
        goto out_err;
    } else {
        ternfs_dir_drop_cache(TERNFS_I(dir));
        clear_nlink(dentry->d_inode);
    }

out_err:
    trace_eggsfs_vfs_rmdir_exit(dir, dentry, err);
    return err;
}

#define POLICY_DELETE_AFTER_TIME_ACTIVE_AND_ZERO (1ull << 63)
#define POLICY_DELETE_AFTER_VERSIONS_ACTIVE_AND_ZERO (1 << 15)

static int ternfs_delete_file(
    struct ternfs_fs_info* info, u64 dir, u64 file, const char* name, int name_len, u64 creation_time,
    struct ternfs_policy* snapshot_policy
) {
    int err;

    u64 delete_creation_time;
    err = ternfs_shard_soft_unlink_file(
        info,
        dir,
        file,
        name,
        name_len,
        creation_time,
        &delete_creation_time
    );
    struct ternfs_policy_body snapshot_policy_body;
    ternfs_get_policy_body(snapshot_policy, &snapshot_policy_body);
    struct ternfs_snapshot_policy policy;
    ternfs_snapshot_policy_get(&snapshot_policy_body, &policy);
    if (likely(policy.delete_after_time != POLICY_DELETE_AFTER_TIME_ACTIVE_AND_ZERO &&
        policy.delete_after_versions != POLICY_DELETE_AFTER_VERSIONS_ACTIVE_AND_ZERO)) {
        goto out;
    }

    if (unlikely(err)) {
        goto out;
    }

    if (unlikely(ternfs_inode_shard(dir) != ternfs_inode_shard(file))) {
        err = ternfs_cdc_cross_shard_hard_unlink_file(
            info,
            dir,
            file,
            name,
            name_len,
            creation_time
        );
    } else {
        err = ternfs_shard_hard_unlink_file(
            info,
            dir,
            file,
            name,
            name_len,
            creation_time
        );
    }
out:
    return err;
}

// vfs: exclusive dir,d_entry->d_inode.i_rwsem
int ternfs_unlink(struct inode* dir, struct dentry* dentry) {
    int err;

    trace_eggsfs_vfs_unlink_enter(dir, dentry, dentry->d_inode);
    BUG_ON(!dentry->d_inode);

    if (unlikely(dentry->d_name.len > TERNFS_MAX_FILENAME)) {
        pr_err("ternfs: ternfs_unlink dir=%lu dentry=%pd exceed max filename length %d\n", dir->i_ino, dentry, TERNFS_MAX_FILENAME);
        err = -ENAMETOOLONG;
        goto out_err;
    }

    struct ternfs_inode* enode = TERNFS_I(dentry->d_inode);
    err = ternfs_delete_file(
        (struct ternfs_fs_info*)dir->i_sb->s_fs_info,
        dir->i_ino,
        dentry->d_inode->i_ino,
        dentry->d_name.name,
        dentry->d_name.len,
        enode->edge_creation_time,
        enode->snapshot_policy
    );
    if (unlikely(err)) {
        if (err < 0) { goto out_err; }
        ternfs_dcache_handle_error(dir, dentry, err);
        err = ternfs_error_to_linux(err);
        goto out_err;
    } else {
        ternfs_dir_drop_cache(TERNFS_I(dir));
        // no hard links, inode is gone
        clear_nlink(dentry->d_inode);
    }

out_err:
    trace_eggsfs_vfs_unlink_exit(dir, dentry, err);
    return err;
}

// vfs: exclusive i_rwsem for all
int COMPAT_FUNC_UNS_IMP(ternfs_rename, struct inode* old_dir, struct dentry* old_dentry, struct inode* new_dir, struct dentry* new_dentry, unsigned int flags) {
    int err;

    trace_eggsfs_vfs_rename_enter(old_dir, old_dentry, new_dir, new_dentry);

    if (!old_dentry->d_inode) { // TODO can this ever happen?
        ternfs_warn("got no inodes for old dentry!");
        return -EIO;
    }
    struct ternfs_inode* enode = TERNFS_I(old_dentry->d_inode);
    u64 ino = enode->inode.i_ino;

    if (flags) { err = -EINVAL; goto out; }

    u64 old_creation_time = enode->edge_creation_time;
    u64 new_creation_time;
    struct ternfs_fs_info* info = old_dir->i_sb->s_fs_info;
    if (old_dir == new_dir) {
        err = ternfs_error_to_linux(ternfs_shard_rename(
            info,
            old_dir->i_ino,
            ino, old_dentry->d_name.name, old_dentry->d_name.len,
            old_creation_time,
            new_dentry->d_name.name, new_dentry->d_name.len,
            &new_creation_time
        ));
    } else if (S_ISDIR(old_dentry->d_inode->i_mode)) {
        err = ternfs_error_to_linux(ternfs_cdc_rename_directory(
            info,
            ino,
            old_dir->i_ino, new_dir->i_ino,
            old_dentry->d_name.name, old_dentry->d_name.len, old_creation_time,
            new_dentry->d_name.name, new_dentry->d_name.len,
            &new_creation_time
        ));
    } else {
        err = ternfs_error_to_linux(ternfs_cdc_rename_file(
            info,
            ino,
            old_dir->i_ino, new_dir->i_ino,
            old_dentry->d_name.name, old_dentry->d_name.len, old_creation_time,
            new_dentry->d_name.name, new_dentry->d_name.len,
            &new_creation_time
        ));
    }
    // TODO do we need to drop the dentries in case of error? The old code
    // thought so, but I'm not sure.
    if (!err) {
        old_dentry->d_time = new_creation_time; // used by d_revalidate
        enode->edge_creation_time = new_creation_time;
        struct ternfs_inode* new_edir = TERNFS_I(new_dir);
        enode->block_policy = new_edir->block_policy;
        enode->span_policy = new_edir->span_policy;
        enode->stripe_policy = new_edir->stripe_policy;
        d_move(old_dentry, new_dentry);
    }

out:
    trace_eggsfs_vfs_rename_exit(old_dir, old_dentry, new_dir, new_dentry, err);
    ternfs_debug("err=%d", err);
    return err;
}
