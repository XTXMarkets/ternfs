// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/module.h>
#include <linux/string.h>
#include <linux/workqueue.h>

#include "../wq.h"

static unsigned allocation_attempts;
static unsigned fail_allocation;
static unsigned live_queues;
static unsigned destroy_count;
static struct workqueue_struct* destroyed[8];
static char allocation_names[8][32];
static unsigned allocation_flags[8];
static int allocation_max_active[8];

static struct workqueue_struct* test_alloc_workqueue(
    const char* name, unsigned int flags, int max_active
) {
    struct workqueue_struct* wq;
    unsigned index = allocation_attempts++;
    if (index < ARRAY_SIZE(allocation_names)) {
        strscpy(allocation_names[index], name, sizeof(allocation_names[index]));
        allocation_flags[index] = flags;
        allocation_max_active[index] = max_active;
    }
    if (allocation_attempts == fail_allocation) { return NULL; }
    wq = alloc_workqueue(name, flags, max_active);
    if (wq) { live_queues++; }
    return wq;
}

static void test_destroy_workqueue(struct workqueue_struct* wq) {
    destroy_workqueue(wq);
    if (destroy_count < ARRAY_SIZE(destroyed)) {
        destroyed[destroy_count] = wq;
    }
    destroy_count++;
    if (live_queues) { live_queues--; }
}

// Limit failure injection and lifecycle accounting to this copy of the
// production helper. Successful queues and their workers remain real.
#undef alloc_workqueue
#define alloc_workqueue(name, flags, max_active) \
    test_alloc_workqueue(name, flags, max_active)
#undef destroy_workqueue
#define destroy_workqueue(wq) test_destroy_workqueue(wq)
#include "../wq.c"
#undef destroy_workqueue
#undef alloc_workqueue

MODULE_LICENSE("GPL");

static atomic_t general_work_runs = ATOMIC_INIT(0);
static atomic_t fast_work_runs = ATOMIC_INIT(0);

static void run_general_work(struct work_struct* work) {
    atomic_inc(&general_work_runs);
}

static void run_fast_work(struct work_struct* work) {
    atomic_inc(&fast_work_runs);
}

static DECLARE_WORK(general_work, run_general_work);
static DECLARE_WORK(fast_work, run_fast_work);

#define CHECK(condition, description) do { \
    if (!(condition)) { \
        pr_err("ternfs-wq-test: FAIL: %s\n", description); \
        err = -EINVAL; \
        goto out; \
    } \
} while (0)

static int __init wq_test_init(void) {
    struct workqueue_struct* general;
    struct workqueue_struct* fast;
    unsigned attempts_before;
    unsigned destroys_before;
    int err = 0;

    CHECK(!ternfs_wq && !ternfs_fast_wq, "globals not initially clear");

    attempts_before = allocation_attempts;
    destroys_before = destroy_count;
    fail_allocation = attempts_before + 1;
    CHECK(ternfs_workqueues_init() == -ENOMEM, "first allocation failure errno");
    CHECK(allocation_attempts == attempts_before + 1, "first failure attempts");
    CHECK(destroy_count == destroys_before, "first failure destroyed a queue");
    CHECK(!ternfs_wq && !ternfs_fast_wq, "first failure left queue pointer");
    CHECK(live_queues == 0, "first failure left live queue");

    attempts_before = allocation_attempts;
    destroys_before = destroy_count;
    fail_allocation = attempts_before + 2;
    CHECK(ternfs_workqueues_init() == -ENOMEM, "second allocation failure errno");
    CHECK(allocation_attempts == attempts_before + 2, "second failure attempts");
    CHECK(destroy_count == destroys_before + 1, "second failure cleanup count");
    CHECK(!ternfs_wq && !ternfs_fast_wq, "second failure left queue pointer");
    CHECK(live_queues == 0, "second failure left live queue");

    fail_allocation = 0;
    attempts_before = allocation_attempts;
    CHECK(ternfs_workqueues_init() == 0, "successful initialization");
    CHECK(ternfs_wq && ternfs_fast_wq && ternfs_wq != ternfs_fast_wq,
          "successful queue pointers");
    CHECK(live_queues == 2, "successful live queue count");
    CHECK(strcmp(allocation_names[attempts_before], "ternfs-wq") == 0,
          "general queue name");
    CHECK(strcmp(allocation_names[attempts_before + 1], "ternfs-fast-wq") == 0,
          "fast queue name");
    CHECK(allocation_flags[attempts_before] == 0 &&
          allocation_flags[attempts_before + 1] == 0,
          "queue flags changed");
    CHECK(allocation_max_active[attempts_before] == 0 &&
          allocation_max_active[attempts_before + 1] == 0,
          "queue max-active changed");

    general = ternfs_wq;
    fast = ternfs_fast_wq;
    destroys_before = destroy_count;
    CHECK(queue_work(general, &general_work), "queue general work");
    CHECK(queue_work(fast, &fast_work), "queue fast work");
    ternfs_workqueues_exit();
    CHECK(atomic_read(&general_work_runs) == 1 &&
          atomic_read(&fast_work_runs) == 1,
          "queued work not drained by destruction");
    CHECK(destroy_count == destroys_before + 2, "successful destroy count");
    CHECK(destroyed[destroys_before] == fast &&
          destroyed[destroys_before + 1] == general,
          "queue destruction order");
    CHECK(!ternfs_wq && !ternfs_fast_wq, "cleanup did not clear pointers");
    CHECK(live_queues == 0, "cleanup left live queues");

    ternfs_workqueues_exit();
    CHECK(destroy_count == destroys_before + 2, "repeated cleanup destroyed queue");
    CHECK(live_queues == 0, "repeated cleanup changed live count");

    CHECK(ternfs_workqueues_init() == 0, "reinitialization");
    CHECK(live_queues == 2, "reinitialization live count");
    ternfs_workqueues_exit();
    CHECK(!ternfs_wq && !ternfs_fast_wq && live_queues == 0,
          "reinitialized cleanup");

out:
    fail_allocation = 0;
    ternfs_workqueues_exit();
    if (!err) {
        pr_info("ternfs-wq-test: PASS (allocation failures, drain, order, repeat)\n");
    }
    return err;
}

static void __exit wq_test_exit(void) {}

module_init(wq_test_init);
module_exit(wq_test_exit);
