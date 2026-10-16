// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/completion.h>
#include <linux/fs.h>
#include <linux/jiffies.h>
#include <linux/kthread.h>
#include <linux/module.h>

#include "../inode.h"

MODULE_LICENSE("GPL");

static struct ternfs_inode test_enode;
static unsigned wake_calls;
static unsigned iput_calls;
static bool iput_order_ok;
static bool reacquire_on_wake;
static bool reacquired;
static u64 reacquire_seqno;
static u64 expected_counter_at_iput;

struct waiter_context {
    struct ternfs_latch* latch;
    u64 seqno;
    long result;
    struct completion started;
    struct completion done;
};

static int test_wake_function(
    wait_queue_entry_t* entry, unsigned mode, int sync, void* key
) {
    wake_calls++;
    if (reacquire_on_wake) {
        reacquired = ternfs_latch_try_acquire(
            &test_enode.getattr_update_latch, reacquire_seqno
        );
        if (reacquired) {
            // Model the next async getattr taking ownership during wakeup.
            test_enode.getattr_async_seqno = reacquire_seqno;
        }
    }
    return 1;
}

static void test_iput(struct inode* inode) {
    iput_calls++;
    iput_order_ok =
        inode == &test_enode.inode && wake_calls == 1 &&
        atomic64_read(&test_enode.getattr_update_latch.counter) ==
            expected_counter_at_iput &&
        atomic_read(&inode->i_count) == 2;
    iput(inode);
}

#define iput test_iput
#include "../getattr_completion.h"
#undef iput

static int waiter_thread_fn(void* data) {
    struct waiter_context* context = data;

    complete(&context->started);
    ternfs_latch_wait(context->latch, context->seqno);
    context->result = 0;
    complete(&context->done);
    while (!kthread_should_stop()) {
        set_current_state(TASK_INTERRUPTIBLE);
        schedule();
    }
    __set_current_state(TASK_RUNNING);
    return 0;
}

static int wait_until_queued(struct ternfs_latch* latch) {
    unsigned long deadline = jiffies + 2 * HZ;

    while (!waitqueue_active(&latch->wq)) {
        if (time_after(jiffies, deadline)) { return -ETIMEDOUT; }
        cond_resched();
    }
    return 0;
}

static int run_completion_case(bool reacquire) {
    struct waiter_context waiter;
    wait_queue_entry_t observer;
    struct task_struct* task;
    u64 seqno;
    u64 next_seqno;
    u64 mtime = 0x1122334455667788ULL;
    u64 edge_time = 0x8877665544332211ULL;
    int status = TERNFS_FILE_STATUS_READING;
    int err = 0;

    memset(&test_enode.file, 0, sizeof(test_enode.file));
    test_enode.mtime = mtime;
    test_enode.edge_creation_time = edge_time;
    test_enode.file.status = status;
    ternfs_latch_init(&test_enode.getattr_update_latch);
    if (!ternfs_latch_try_acquire(&test_enode.getattr_update_latch, seqno)) {
        return -EINVAL;
    }
    test_enode.getattr_async_seqno = seqno;
    atomic_set(&test_enode.inode.i_count, 1);
    ihold(&test_enode.inode);

    waiter.latch = &test_enode.getattr_update_latch;
    waiter.seqno = seqno;
    waiter.result = -1;
    init_completion(&waiter.started);
    init_completion(&waiter.done);
    task = kthread_run(
        waiter_thread_fn, &waiter, "ternfs-getattr-completion-test"
    );
    if (IS_ERR(task)) { return PTR_ERR(task); }
    if (!wait_for_completion_timeout(&waiter.started, 2 * HZ)) {
        err = -ETIMEDOUT;
        goto stop;
    }
    err = wait_until_queued(&test_enode.getattr_update_latch);
    if (err) { goto stop; }

    init_waitqueue_func_entry(&observer, test_wake_function);
    add_wait_queue(&test_enode.getattr_update_latch.wq, &observer);
    wake_calls = 0;
    iput_calls = 0;
    iput_order_ok = false;
    reacquire_on_wake = reacquire;
    reacquired = false;
    reacquire_seqno = 0;
    expected_counter_at_iput = reacquire ? seqno + 3 : seqno + 2;

    ternfs_finish_async_getattr(&test_enode);
    remove_wait_queue(&test_enode.getattr_update_latch.wq, &observer);

    if (!wait_for_completion_timeout(&waiter.done, 2 * HZ)) {
        err = -ETIMEDOUT;
        goto stop;
    }
    if (waiter.result != 0 || wake_calls != 1 || iput_calls != 1 ||
        !iput_order_ok || atomic_read(&test_enode.inode.i_count) != 1 ||
        test_enode.mtime != mtime ||
        test_enode.edge_creation_time != edge_time ||
        test_enode.file.status != status) {
        pr_err(
            "ternfs-getattr-completion-test: completion ordering mismatch\n"
        );
        err = -EINVAL;
        goto stop;
    }
    if (!reacquire && test_enode.getattr_async_seqno != seqno) {
        pr_err(
            "ternfs-getattr-completion-test: completion changed async seqno\n"
        );
        err = -EINVAL;
        goto stop;
    }

    if (reacquire) {
        if (!reacquired || reacquire_seqno != seqno + 2 ||
            test_enode.getattr_async_seqno != reacquire_seqno) {
            pr_err(
                "ternfs-getattr-completion-test: wake reacquisition failed\n"
            );
            err = -EINVAL;
            goto stop;
        }
        ternfs_latch_release(
            &test_enode.getattr_update_latch, reacquire_seqno
        );
    } else {
        if (!ternfs_latch_try_acquire(
                &test_enode.getattr_update_latch, next_seqno) ||
            next_seqno != seqno + 2) {
            pr_err(
                "ternfs-getattr-completion-test: subsequent acquire failed\n"
            );
            err = -EINVAL;
            goto stop;
        }
        ternfs_latch_release(&test_enode.getattr_update_latch, next_seqno);
    }
    if (atomic64_read(&test_enode.getattr_update_latch.counter) !=
        seqno + 4) {
        pr_err("ternfs-getattr-completion-test: final counter mismatch\n");
        err = -EINVAL;
    }

stop:
    if (!completion_done(&waiter.done)) {
        // Ensure a fixture failure cannot strand the waiter.
        u64 counter = atomic64_read(&test_enode.getattr_update_latch.counter);
        if (counter & 1) {
            u64 held_seqno = counter - 1;
            ternfs_latch_release(
                &test_enode.getattr_update_latch, held_seqno
            );
        }
        wait_for_completion_timeout(&waiter.done, 2 * HZ);
    }
    kthread_stop(task);
    atomic_set(&test_enode.inode.i_count, 0);
    return err;
}

static int __init getattr_completion_test_init(void) {
    int err;

    memset(&test_enode, 0, sizeof(test_enode));
    inode_init_once(&test_enode.inode);

    err = run_completion_case(false);
    if (err) { return err; }
    err = run_completion_case(true);
    if (err) { return err; }

    pr_info(
        "ternfs-getattr-completion-test: PASS "
        "(counter, wake, iput, waiter, reacquire, state)\n"
    );
    return 0;
}

static void __exit getattr_completion_test_exit(void) {}

module_init(getattr_completion_test_init);
module_exit(getattr_completion_test_exit);
