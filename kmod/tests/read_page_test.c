// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/completion.h>
#include <linux/delay.h>
#include <linux/highmem.h>
#include <linux/kthread.h>
#include <linux/module.h>
#include <linux/pagemap.h>

static struct page* test_page;
static u8 expected_byte;
static unsigned expected_refcount;
static atomic_t unlock_calls = ATOMIC_INIT(0);
static atomic_t boundary_bad = ATOMIC_INIT(0);

static bool page_has_expected_data(struct page* page) {
    char* data = kmap(page);
    unsigned i;
    bool matches = true;
    for (i = 0; i < PAGE_SIZE; i++) {
        if ((u8)data[i] != expected_byte) {
            matches = false;
            break;
        }
    }
    kunmap(page);
    return matches;
}

static void test_unlock_page(struct page* page);

// Observe the exact publication boundary while retaining the real page unlock
// and wakeup implementation.
#undef unlock_page
#define unlock_page(page) test_unlock_page(page)
#include "../read_page.h"
#undef unlock_page

static void test_unlock_page(struct page* page) {
    if (page != test_page || !PageLocked(page) || !PageUptodate(page) ||
        page_ref_count(page) != expected_refcount ||
        !page_has_expected_data(page)) {
        atomic_set(&boundary_bad, 1);
    }
    atomic_inc(&unlock_calls);
    unlock_page(page);
    if (PageLocked(page)) {
        atomic_set(&boundary_bad, 1);
    }
}

MODULE_LICENSE("GPL");

static DECLARE_COMPLETION(waiter_started);
static DECLARE_COMPLETION(waiter_finished);
static atomic_t waiter_ok = ATOMIC_INIT(0);

static int page_waiter(void* unused) {
    complete(&waiter_started);
    wait_on_page_locked(test_page);
    if (!PageLocked(test_page) && PageUptodate(test_page) &&
        page_ref_count(test_page) == expected_refcount &&
        page_has_expected_data(test_page)) {
        atomic_set(&waiter_ok, 1);
    }
    complete(&waiter_finished);
    for (;;) {
        set_current_state(TASK_INTERRUPTIBLE);
        if (kthread_should_stop()) { break; }
        schedule();
    }
    __set_current_state(TASK_RUNNING);
    return 0;
}

static void fill_page(struct page* page, u8 byte) {
    char* data = kmap(page);
    memset(data, byte, PAGE_SIZE);
    kunmap(page);
    expected_byte = byte;
}

#define CHECK(condition, description) do { \
    if (!(condition)) { \
        pr_err("ternfs-read-page-test: FAIL: %s\n", description); \
        err = -EINVAL; \
        goto out; \
    } \
} while (0)

static int __init read_page_test_init(void) {
    struct task_struct* waiter = NULL;
    unsigned base_refcount;
    int err = 0;

    test_page = alloc_page(GFP_KERNEL);
    if (!test_page) { return -ENOMEM; }
    base_refcount = page_ref_count(test_page);
    expected_refcount = base_refcount;

    fill_page(test_page, 0x31);
    ClearPageUptodate(test_page);
    lock_page(test_page);
    ternfs_complete_read_page(test_page);
    CHECK(atomic_read(&unlock_calls) == 1, "first unlock boundary");
    CHECK(!atomic_read(&boundary_bad), "uptodate was not published while locked");
    CHECK(!PageLocked(test_page) && PageUptodate(test_page), "first final state");
    CHECK(page_ref_count(test_page) == base_refcount, "first reference changed");
    CHECK(page_has_expected_data(test_page), "first data changed");

    fill_page(test_page, 0x52);
    ClearPageUptodate(test_page);
    atomic_set(&waiter_ok, 0);
    reinit_completion(&waiter_started);
    reinit_completion(&waiter_finished);
    lock_page(test_page);
    waiter = kthread_run(page_waiter, NULL, "ternfs-read-page-waiter");
    CHECK(!IS_ERR(waiter), "start page waiter");
    CHECK(wait_for_completion_timeout(&waiter_started, 5 * HZ) > 0,
          "page waiter did not start");
    // Give the waiter a chance to enter wait_on_page_locked; correctness does
    // not depend on this delay because it validates state again after unlock.
    msleep(10);
    ternfs_complete_read_page(test_page);
    CHECK(wait_for_completion_timeout(&waiter_finished, 5 * HZ) > 0,
          "page waiter did not observe unlock");
    CHECK(atomic_read(&waiter_ok), "waiter observed incomplete state or data");
    CHECK(atomic_read(&unlock_calls) == 2, "waiter unlock boundary");
    CHECK(!atomic_read(&boundary_bad), "waiter publication boundary");
    CHECK(page_ref_count(test_page) == base_refcount, "waiter reference changed");
    kthread_stop(waiter);
    waiter = NULL;

    fill_page(test_page, 0x73);
    ClearPageUptodate(test_page);
    lock_page(test_page);
    ternfs_complete_read_page(test_page);
    CHECK(atomic_read(&unlock_calls) == 3, "reuse unlock boundary");
    CHECK(!atomic_read(&boundary_bad), "reuse publication boundary");
    CHECK(!PageLocked(test_page) && PageUptodate(test_page), "reuse final state");
    CHECK(page_ref_count(test_page) == base_refcount, "reuse reference changed");
    CHECK(page_has_expected_data(test_page), "reuse data changed");

out:
    if (test_page && PageLocked(test_page)) {
        unlock_page(test_page);
    }
    if (waiter && !IS_ERR(waiter)) {
        kthread_stop(waiter);
    }
    if (test_page) {
        put_page(test_page);
        test_page = NULL;
    }
    if (!err) {
        pr_info("ternfs-read-page-test: PASS (locked publication, waiter, reuse)\n");
    }
    return err;
}

static void __exit read_page_test_exit(void) {}

module_init(read_page_test_init);
module_exit(read_page_test_exit);
