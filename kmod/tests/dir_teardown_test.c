// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/completion.h>
#include <linux/highmem.h>
#include <linux/kthread.h>
#include <linux/module.h>
#include <linux/slab.h>

#include "../dir.h"
#include "../err.h"
#include "../metadata.h"
#include "../page_compat.h"

static struct kmem_cache* tracked_dirents_cache;
static struct ternfs_dirents* tracked_dirents;
static struct page* tracked_pages[2];
static atomic_t tracked_dirents_frees = ATOMIC_INIT(0);
static atomic_t release_reader = ATOMIC_INIT(0);
static atomic_t teardown_order_bad = ATOMIC_INIT(0);
static unsigned cache_destroys;

static bool pages_have_external_ref(void) {
    unsigned i;
    for (i = 0; i < ARRAY_SIZE(tracked_pages); i++) {
        if (!tracked_pages[i] || page_ref_count(tracked_pages[i]) != 1) {
            return false;
        }
    }
    return true;
}

static void test_kmem_cache_free(struct kmem_cache* cache, void* object) {
    if (cache == tracked_dirents_cache && object == tracked_dirents) {
        if (!pages_have_external_ref()) {
            atomic_set(&teardown_order_bad, 1);
        }
        atomic_inc(&tracked_dirents_frees);
    }
    kmem_cache_free(cache, object);
}

static void test_kmem_cache_destroy(struct kmem_cache* cache) {
    if (cache == tracked_dirents_cache &&
        (atomic_read(&tracked_dirents_frees) != 1 ||
         !pages_have_external_ref())) {
        atomic_set(&teardown_order_bad, 1);
    }
    cache_destroys++;
    kmem_cache_destroy(cache);
}

// dir.c only needs these trace boundaries. Avoid instantiating a second
// tracepoint provider in this standalone module.
#define _TRACE_EGGFS_H
#define TERNFS_INODE_LOCK 0
#define TERNFS_INODE_UNLOCK 3
#define trace_eggsfs_vfs_opendir_enter(...) do {} while (0)
#define trace_eggsfs_vfs_opendir_exit(...) do {} while (0)
#define trace_eggsfs_vfs_closedir_enter(...) do {} while (0)
#define trace_eggsfs_vfs_closedir_exit(...) do {} while (0)
#define trace_eggsfs_inode_lock(...) do {} while (0)

// Observe the production callback and exit ordering while retaining real slab
// allocation/free behavior.
#undef kmem_cache_free
#define kmem_cache_free(cache, object) test_kmem_cache_free(cache, object)
#undef kmem_cache_destroy
#define kmem_cache_destroy(cache) test_kmem_cache_destroy(cache)
#include "../dir.c"
#undef kmem_cache_destroy
#undef kmem_cache_free

MODULE_LICENSE("GPL");

int ternfs_debug_output;

int ternfs_do_getattr(struct ternfs_inode* enode, int cache_timeout_type) {
    return -EIO;
}

int ternfs_error_to_linux(int err) {
    return err <= 0 ? err : -EIO;
}

struct inode* ternfs_get_inode(
    struct super_block* sb, bool allow_no_parent,
    struct ternfs_inode* parent, u64 ino
) {
    return ERR_PTR(-EIO);
}

int ternfs_shard_readdir(
    struct ternfs_fs_info* info, u64 dir, u64 start_pos,
    void* data, u64* next_hash
) {
    return -EIO;
}

int ternfs_start_async_getattr(struct ternfs_inode* enode) {
    return -EIO;
}

static DECLARE_COMPLETION(reader_entered);
static DECLARE_COMPLETION(reader_finished);
static DECLARE_COMPLETION(exit_finished);

static int hold_rcu_reader(void* unused) {
    rcu_read_lock();
    complete(&reader_entered);
    while (!atomic_read(&release_reader)) {
        cpu_relax();
    }
    rcu_read_unlock();
    complete(&reader_finished);
    for (;;) {
        set_current_state(TASK_INTERRUPTIBLE);
        if (kthread_should_stop()) { break; }
        schedule();
    }
    __set_current_state(TASK_RUNNING);
    return 0;
}

static int run_dir_exit(void* unused) {
    ternfs_dir_exit();
    complete(&exit_finished);
    for (;;) {
        set_current_state(TASK_INTERRUPTIBLE);
        if (kthread_should_stop()) { break; }
        schedule();
    }
    __set_current_state(TASK_RUNNING);
    return 0;
}

static bool page_contents_survive(void) {
    unsigned i;
    for (i = 0; i < ARRAY_SIZE(tracked_pages); i++) {
        char* data = kmap(tracked_pages[i]);
        bool matches = data[0] == (char)(0x51 + i) &&
            data[PAGE_SIZE - 1] == (char)(0x61 + i);
        kunmap(tracked_pages[i]);
        if (!matches) { return false; }
    }
    return true;
}

#define CHECK(condition, description) do { \
    if (!(condition)) { \
        pr_err("ternfs-dir-teardown-test: FAIL: %s\n", description); \
        err = -EINVAL; \
        goto out; \
    } \
} while (0)

static int __init dir_teardown_test_init(void) {
    static struct ternfs_inode enode;
    struct ternfs_dirents* held;
    struct task_struct* reader_task = NULL;
    struct task_struct* exit_task = NULL;
    bool cache_assigned = false;
    bool dir_active = false;
    unsigned i;
    int err;

    err = ternfs_dir_init();
    if (err) { return err; }
    dir_active = true;
    tracked_dirents_cache = ternfs_dirents_cachep;

    tracked_dirents = kmem_cache_alloc(ternfs_dirents_cachep, GFP_KERNEL);
    CHECK(tracked_dirents != NULL, "allocate dirents");
    INIT_LIST_HEAD(&tracked_dirents->pages);
    atomic_set(&tracked_dirents->refcount, 1);
    tracked_dirents->mtime = 123;
    tracked_dirents->max_hash = 456;
    rcu_assign_pointer(enode.dir.dirents, tracked_dirents);
    cache_assigned = true;

    for (i = 0; i < ARRAY_SIZE(tracked_pages); i++) {
        char* data;
        tracked_pages[i] = alloc_page(GFP_KERNEL);
        CHECK(tracked_pages[i] != NULL, "allocate directory page");
        data = kmap(tracked_pages[i]);
        memset(data, 0, PAGE_SIZE);
        data[0] = (char)(0x51 + i);
        data[PAGE_SIZE - 1] = (char)(0x61 + i);
        kunmap(tracked_pages[i]);
        get_page(tracked_pages[i]);
        list_add_tail(&tracked_pages[i]->lru, &tracked_dirents->pages);
    }

    held = ternfs_dir_get_cache(&enode);
    CHECK(held == tracked_dirents, "get assigned cache");
    CHECK(atomic_read(&tracked_dirents->refcount) == 2, "get cache reference");
    ternfs_dir_put_cache(held);
    CHECK(atomic_read(&tracked_dirents->refcount) == 1, "put cache reference");

    reader_task = kthread_run(hold_rcu_reader, NULL, "ternfs-dir-rcu-reader");
    CHECK(!IS_ERR(reader_task), "start RCU reader");
    CHECK(wait_for_completion_timeout(&reader_entered, 5 * HZ) > 0,
          "RCU reader did not enter");

    ternfs_dir_drop_cache(&enode);
    cache_assigned = false;
    CHECK(rcu_access_pointer(enode.dir.dirents) == NULL, "drop did not clear pointer");
    ternfs_dir_drop_cache(&enode);
    ternfs_dir_drop_cache(&enode);
    CHECK(atomic_read(&tracked_dirents_frees) == 0,
          "callback ran while prior RCU reader remained");

    exit_task = kthread_run(run_dir_exit, NULL, "ternfs-dir-exit");
    CHECK(!IS_ERR(exit_task), "start directory exit");
    CHECK(wait_for_completion_timeout(&exit_finished, msecs_to_jiffies(25)) == 0,
          "directory exit did not wait for RCU callback");
    CHECK(cache_destroys == 0, "slab destroyed before grace period");

    atomic_set(&release_reader, 1);
    CHECK(wait_for_completion_timeout(&reader_finished, 5 * HZ) > 0,
          "RCU reader did not finish");
    CHECK(wait_for_completion_timeout(&exit_finished, 5 * HZ) > 0,
          "directory exit did not finish");
    dir_active = false;

    CHECK(atomic_read(&tracked_dirents_frees) == 1,
          "dirents not freed exactly once");
    CHECK(cache_destroys == 2, "directory slabs not destroyed exactly once");
    CHECK(!atomic_read(&teardown_order_bad),
          "directory slab destroyed before callback/page release");
    CHECK(pages_have_external_ref(), "directory page ownership not released");
    CHECK(page_contents_survive(), "external page owners did not survive");
    ternfs_dir_drop_cache(&enode);

out:
    atomic_set(&release_reader, 1);
    if (reader_task && !IS_ERR(reader_task)) {
        kthread_stop(reader_task);
    }
    if (exit_task && !IS_ERR(exit_task)) {
        kthread_stop(exit_task);
        dir_active = false;
    }
    if (dir_active) {
        if (cache_assigned) {
            ternfs_dir_drop_cache(&enode);
            cache_assigned = false;
        } else if (tracked_dirents &&
                   atomic_read(&tracked_dirents_frees) == 0) {
            rcu_barrier();
        }
        ternfs_dir_exit();
    }
    for (i = 0; i < ARRAY_SIZE(tracked_pages); i++) {
        if (tracked_pages[i]) {
            put_page(tracked_pages[i]);
            tracked_pages[i] = NULL;
        }
    }
    if (!err) {
        pr_info("ternfs-dir-teardown-test: PASS (barrier, slab, pages, repeated drop)\n");
    }
    return err;
}

static void __exit dir_teardown_test_exit(void) {}

module_init(dir_teardown_test_init);
module_exit(dir_teardown_test_exit);
