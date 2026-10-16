// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/completion.h>
#include <linux/highmem.h>
#include <linux/kthread.h>
#include <linux/mm.h>
#include <linux/module.h>

#include "../fetch_state.c"

MODULE_LICENSE("GPL");

static struct ternfs_block_span test_span;
static struct address_space test_mapping;

struct finish_context {
    struct fetch_span_pages_state* state;
    struct completion started;
    struct completion done;
};

static struct page* add_held_page(
    struct fetch_span_pages_state* state, unsigned block, u8 marker
) {
    struct page* page = alloc_page(GFP_KERNEL);
    u8* data;

    if (!page) { return NULL; }
    data = kmap(page);
    data[0] = marker;
    kunmap(page);
    get_page(page);
    list_add_tail(&page->lru, &state->blocks_pages[block]);
    return page;
}

static int check_held_page(struct page* page, u8 marker, const char* test) {
    u8* data;
    u8 actual;

    if (page_ref_count(page) != 1) {
        pr_err(
            "ternfs-fetch-state-test: %s refcount=%d expected=1\n",
            test, page_ref_count(page)
        );
        return -EINVAL;
    }
    data = kmap(page);
    actual = data[0];
    kunmap(page);
    if (actual != marker) {
        pr_err(
            "ternfs-fetch-state-test: %s marker=%#x expected=%#x\n",
            test, actual, marker
        );
        return -EINVAL;
    }
    return 0;
}

static int finish_thread_fn(void* data) {
    struct finish_context* context = data;

    complete(&context->started);
    ternfs_finish_fetch_span_pages(context->state);
    complete(&context->done);
    return 0;
}

static struct task_struct* start_finish(
    struct finish_context* context, struct fetch_span_pages_state* state
) {
    struct task_struct* task;

    context->state = state;
    init_completion(&context->started);
    init_completion(&context->done);
    task = kthread_run(finish_thread_fn, context, "ternfs-fetch-state-test");
    if (IS_ERR(task)) { return task; }
    if (!wait_for_completion_timeout(&context->started, 2 * HZ)) {
        kthread_stop(task);
        return ERR_PTR(-ETIMEDOUT);
    }
    return task;
}

static int require_finish_blocked(
    struct finish_context* context, const char* test
) {
    if (wait_for_completion_timeout(
            &context->done, msecs_to_jiffies(20))) {
        pr_err("ternfs-fetch-state-test: %s returned before callbacks\n", test);
        return -EINVAL;
    }
    return 0;
}

static int test_immediate_cleanup(void) {
    struct fetch_span_pages_state* state;
    struct page* page;
    int err;

    state = ternfs_new_fetch_span_pages_state(&test_span);
    if (!state) { return -ENOMEM; }
    page = add_held_page(state, 0, 0x21);
    if (!page) {
        ternfs_finish_fetch_span_pages(state);
        return -ENOMEM;
    }
    atomic64_set(&state->blocks, 0xffff);
    ternfs_finish_fetch_span_pages(state);
    err = check_held_page(page, 0x21, "immediate cleanup");
    put_page(page);
    return err;
}

static int test_pending_callbacks(bool terminal_error, bool retry_hold) {
    struct fetch_span_pages_state* state;
    struct finish_context context;
    struct task_struct* task;
    struct page* page;
    unsigned callback_refs = 3;
    int err = 0;

    test_span.span.ino = 0x1122334455667788ULL;
    state = ternfs_new_fetch_span_pages_state(&test_span);
    if (!state) { return -ENOMEM; }
    state->mapping = &test_mapping;
    state->start_offset = 4096;
    state->size = PAGE_SIZE;
    if (terminal_error) { atomic_set(&state->err, -EIO); }
    page = add_held_page(state, 1, terminal_error ? 0x42 : 0x41);
    if (!page) {
        ternfs_finish_fetch_span_pages(state);
        return -ENOMEM;
    }

    while (callback_refs > 0) {
        ternfs_hold_fetch_span_pages(state);
        callback_refs--;
    }
    callback_refs = 3;
    task = start_finish(&context, state);
    if (IS_ERR(task)) {
        err = PTR_ERR(task);
        while (callback_refs > 0) {
            ternfs_put_fetch_span_pages(state);
            callback_refs--;
        }
        put_page(page);
        return err;
    }
    err = require_finish_blocked(
        &context, terminal_error ? "terminal-error finish" : "pending finish"
    );
    if (err) { goto release; }
    if (page_ref_count(page) != 2 ||
        state->block_span != &test_span ||
        state->mapping != &test_mapping ||
        state->block_span->span.ino != 0x1122334455667788ULL) {
        pr_err("ternfs-fetch-state-test: borrowed dependencies released early\n");
        err = -EINVAL;
        goto release;
    }

    if (retry_hold) {
        // A callback may submit replacement work before dropping its own ref.
        ternfs_hold_fetch_span_pages(state);
        callback_refs++;
        ternfs_put_fetch_span_pages(state);
        callback_refs--;
        if (completion_done(&context.done)) {
            pr_err("ternfs-fetch-state-test: retry hold did not retain caller\n");
            err = -EINVAL;
            goto release;
        }
    }

    while (callback_refs > 1) {
        ternfs_put_fetch_span_pages(state);
        callback_refs--;
    }
    if (completion_done(&context.done)) {
        pr_err("ternfs-fetch-state-test: finish returned with one callback\n");
        err = -EINVAL;
        goto release;
    }

release:
    while (callback_refs > 0) {
        ternfs_put_fetch_span_pages(state);
        callback_refs--;
    }
    if (!wait_for_completion_timeout(&context.done, 2 * HZ)) {
        pr_err("ternfs-fetch-state-test: finish did not observe last callback\n");
        err = -ETIMEDOUT;
    }
    kthread_stop(task);
    if (!err) {
        err = check_held_page(
            page, terminal_error ? 0x42 : 0x41,
            terminal_error ? "terminal-error cleanup" : "pending cleanup"
        );
    }
    put_page(page);
    return err;
}

static int check_reset_state(struct fetch_span_pages_state* state) {
    unsigned i;

    if (atomic_read(&state->refcount) != 1 ||
        atomic_read(&state->err) != 0 ||
        atomic_read(&state->last_block_err) != 0 ||
        atomic64_read(&state->blocks) != 0 ||
        atomic_read(&state->start_block) != 0 ||
        atomic_read(&state->end_block) != 0 ||
        state->mapping != NULL || state->start_offset != 0 ||
        state->size != 0 || waitqueue_active(&state->idle) ||
        down_trylock(&state->sema) == 0) {
        return -EINVAL;
    }
    for (i = 0; i < TERNFS_MAX_BLOCKS; i++) {
        if (!list_empty(&state->blocks_pages[i])) { return -EINVAL; }
    }
    return 0;
}

static int test_reuse_reset(void) {
    struct fetch_span_pages_state* first;
    struct fetch_span_pages_state* state;
    struct page* page;
    int err;

    first = ternfs_new_fetch_span_pages_state(&test_span);
    if (!first) { return -ENOMEM; }
    first->mapping = &test_mapping;
    first->start_offset = 123;
    first->size = 456;
    atomic_set(&first->err, -EIO);
    atomic_set(&first->last_block_err, -ENOMEM);
    atomic64_set(&first->blocks, ~0ULL);
    atomic_set(&first->start_block, 2);
    atomic_set(&first->end_block, 3);
    up(&first->sema);
    up(&first->sema);
    page = add_held_page(first, 2, 0x63);
    if (!page) {
        ternfs_finish_fetch_span_pages(first);
        return -ENOMEM;
    }
    ternfs_finish_fetch_span_pages(first);
    err = check_held_page(page, 0x63, "reuse cleanup");
    put_page(page);
    if (err) { return err; }

    state = ternfs_new_fetch_span_pages_state(&test_span);
    if (!state) { return -ENOMEM; }
    err = check_reset_state(state);
    if (err) {
        pr_err(
            "ternfs-fetch-state-test: cache state was not reset%s\n",
            state == first ? " on reused object" : ""
        );
    }
    ternfs_finish_fetch_span_pages(state);
    return err;
}

static int __init fetch_state_test_init(void) {
    int err;

    memset(&test_span, 0, sizeof(test_span));
    memset(&test_mapping, 0, sizeof(test_mapping));
    err = ternfs_fetch_span_pages_init();
    if (err) { return err; }

    err = test_immediate_cleanup();
    if (err) { goto out; }
    err = test_pending_callbacks(false, false);
    if (err) { goto out; }
    err = test_pending_callbacks(true, true);
    if (err) { goto out; }
    err = test_reuse_reset();

out:
    ternfs_fetch_span_pages_exit();
    if (!err) {
        pr_info(
            "ternfs-fetch-state-test: PASS "
            "(immediate, pending, terminal, retry, borrowed, wake/free, reuse)\n"
        );
    }
    return err;
}

static void __exit fetch_state_test_exit(void) {}

module_init(fetch_state_test_init);
module_exit(fetch_state_test_exit);
