// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/completion.h>
#include <linux/highmem.h>
#include <linux/jiffies.h>
#include <linux/kthread.h>
#include <linux/mm.h>
#include <linux/module.h>
#include <linux/sched/mm.h>
#include <linux/version.h>

#include "../write.c"
#include "../transient.c"

MODULE_LICENSE("GPL");

static struct ternfs_inode test_enode;

struct cleanup_thread {
    struct ternfs_inode* enode;
    struct completion started;
    struct completion done;
};

static long file_pages(struct mm_struct* mm) {
#if (LINUX_VERSION_CODE < KERNEL_VERSION(6, 2, 0))
    return atomic_long_read(&mm->rss_stat.count[MM_FILEPAGES]);
#else
    return percpu_counter_sum(&mm->rss_stat[MM_FILEPAGES]);
#endif
}

static void prepare_file(int status, int error, int semaphore_count) {
    memset(&test_enode.file, 0, sizeof(test_enode.file));
    test_enode.file.status = status;
    atomic_set(&test_enode.file.transient_err, error);
    sema_init(&test_enode.file.flushing_span_sema, semaphore_count);
    test_enode.file.owner = (struct task_struct*)0x12340000UL;
}

static int check_inode_unlocked(const char* test) {
    if (!inode_trylock(&test_enode.inode)) {
        pr_err("ternfs-transient-test: %s left inode locked\n", test);
        return -EINVAL;
    }
    inode_unlock(&test_enode.inode);
    return 0;
}

static struct page* add_held_page(
    struct list_head* pages, struct mm_struct* mm, u8 marker
) {
    struct page* page = ternfs_alloc_write_page(mm);
    u8* data;

    if (!page) { return NULL; }
    data = kmap(page);
    data[0] = marker;
    kunmap(page);
    get_page(page);
    list_add_tail(&page->lru, pages);
    return page;
}

static int check_held_page(struct page* page, u8 marker, const char* test) {
    u8* data;
    u8 actual;

    if (page_ref_count(page) != 1) {
        pr_err(
            "ternfs-transient-test: %s page refcount=%d expected=1\n",
            test, page_ref_count(page)
        );
        return -EINVAL;
    }
    data = kmap(page);
    actual = data[0];
    kunmap(page);
    if (actual != marker) {
        pr_err(
            "ternfs-transient-test: %s page marker=%#x expected=%#x\n",
            test, actual, marker
        );
        return -EINVAL;
    }
    return 0;
}

static int test_untouched_states(void) {
    struct ternfs_inode_file expected;
    int statuses[] = {
        TERNFS_FILE_STATUS_NONE,
        TERNFS_FILE_STATUS_READING,
    };
    unsigned i;
    int err;

    for (i = 0; i < ARRAY_SIZE(statuses); i++) {
        prepare_file(statuses[i], -ENOSPC, 1);
        test_enode.file.writing_span =
            (struct ternfs_transient_span*)0x56780000UL;
        test_enode.file.mm = (struct mm_struct*)0x9abc0000UL;
        expected = test_enode.file;
        ternfs_file_discard(&test_enode);
        if (memcmp(&test_enode.file, &expected, sizeof(expected)) != 0) {
            pr_err(
                "ternfs-transient-test: status %d changed during discard\n",
                statuses[i]
            );
            return -EINVAL;
        }
        err = check_inode_unlocked("untouched state");
        if (err) { return err; }
    }
    return 0;
}

static int test_nonfinal_span_put(struct mm_struct* mm) {
    long rss = file_pages(mm);
    int mm_count = atomic_read(&mm->mm_count);
    struct ternfs_transient_span* span = NULL;
    struct page* page = NULL;
    bool extra_page_ref = false;
    int err = 0;

    prepare_file(TERNFS_FILE_STATUS_WRITING, 0, 1);
    mmgrab(mm);
    test_enode.file.mm = mm;
    span = ternfs_new_transient_span(&test_enode, 17);
    if (!span) {
        err = -ENOMEM;
        goto out;
    }
    page = add_held_page(&span->pages, mm, 0x31);
    if (!page) {
        err = -ENOMEM;
        goto out;
    }
    extra_page_ref = true;
    ternfs_hold_transient_span(span);
    if (ternfs_put_transient_span(span)) {
        span = NULL;
        pr_err("ternfs-transient-test: non-final put released span\n");
        err = -EINVAL;
        goto out;
    }
    if (atomic_read(&span->refcount) != 1 || list_empty(&span->pages) ||
        page_ref_count(page) != 2 || file_pages(mm) != rss + 1) {
        pr_err("ternfs-transient-test: non-final put lost ownership\n");
        err = -EINVAL;
        goto out;
    }
    if (!ternfs_put_transient_span(span)) {
        pr_err("ternfs-transient-test: final put retained span\n");
        err = -EINVAL;
        goto out;
    }
    span = NULL;
    err = check_held_page(page, 0x31, "non-final span put");
    if (err) { goto out; }
    if (file_pages(mm) != rss) {
        pr_err("ternfs-transient-test: non-final test RSS mismatch\n");
        err = -EINVAL;
    }

out:
    if (span) {
        while (!ternfs_put_transient_span(span)) {}
    }
    if (test_enode.file.mm) {
        mmdrop(test_enode.file.mm);
        test_enode.file.mm = NULL;
    }
    if (extra_page_ref) { put_page(page); }
    if (!err && atomic_read(&mm->mm_count) != mm_count) {
        pr_err("ternfs-transient-test: non-final test mm ref mismatch\n");
        err = -EINVAL;
    }
    return err;
}

static int test_discard_pages(struct mm_struct* mm) {
    long rss = file_pages(mm);
    int mm_count = atomic_read(&mm->mm_count);
    struct ternfs_transient_span* span;
    struct page* pages[TERNFS_MAX_BLOCKS + 1] = {};
    unsigned page_count = 0;
    unsigned i;
    int err = 0;

    prepare_file(TERNFS_FILE_STATUS_WRITING, 0, 1);
    mmgrab(mm);
    test_enode.file.mm = mm;
    span = ternfs_new_transient_span(&test_enode, 0);
    if (!span) {
        mmdrop(mm);
        test_enode.file.mm = NULL;
        return -ENOMEM;
    }
    test_enode.file.writing_span = span;
    span->parity = ternfs_mk_parity(2, 1);

    pages[page_count] = add_held_page(&span->pages, mm, 0x40);
    if (!pages[page_count++]) {
        err = -ENOMEM;
        goto out;
    }
    for (i = 0; i < ternfs_blocks(span->parity); i++) {
        pages[page_count] = add_held_page(
            &span->blocks[i], mm, (u8)(0x41 + i)
        );
        if (!pages[page_count++]) {
            err = -ENOMEM;
            goto out;
        }
    }
    if (file_pages(mm) != rss + page_count) {
        pr_err("ternfs-transient-test: discard setup RSS mismatch\n");
        err = -EINVAL;
        goto out;
    }

out:
    ternfs_file_discard(&test_enode);
    if (!err &&
        (test_enode.file.writing_span || test_enode.file.mm ||
         atomic_read(&test_enode.file.transient_err) != -EIO ||
         file_pages(mm) != rss || atomic_read(&mm->mm_count) != mm_count)) {
        pr_err("ternfs-transient-test: discard did not release file state\n");
        err = -EINVAL;
    }
    for (i = 0; i < page_count; i++) {
        if (!pages[i]) { continue; }
        if (!err) {
            err = check_held_page(
                pages[i], (u8)(0x40 + i), "discarded write page"
            );
        }
        put_page(pages[i]);
    }
    ternfs_file_discard(&test_enode);
    if (!err &&
        (test_enode.file.mm || test_enode.file.writing_span ||
         atomic_read(&mm->mm_count) != mm_count ||
         atomic_read(&test_enode.file.transient_err) != -EIO)) {
        pr_err("ternfs-transient-test: repeated discard changed ownership\n");
        err = -EINVAL;
    }
    if (!err) { err = check_inode_unlocked("discard"); }
    return err;
}

static int test_direct_release_idempotent(struct mm_struct* mm) {
    int mm_count = atomic_read(&mm->mm_count);
    int err = 0;

    prepare_file(TERNFS_FILE_STATUS_READING, 0, 1);
    mmgrab(mm);
    test_enode.file.mm = mm;
    inode_lock(&test_enode.inode);
    ternfs_release_transient_file(&test_enode);
    ternfs_release_transient_file(&test_enode);
    inode_unlock(&test_enode.inode);
    if (test_enode.file.mm || atomic_read(&mm->mm_count) != mm_count) {
        pr_err("ternfs-transient-test: direct release double-dropped mm\n");
        err = -EINVAL;
    }
    if (!err) { err = check_inode_unlocked("direct release"); }
    return err;
}

static int test_preexisting_error(struct mm_struct* mm) {
    int mm_count = atomic_read(&mm->mm_count);
    int err = 0;

    prepare_file(TERNFS_FILE_STATUS_WRITING, -ENOSPC, 1);
    mmgrab(mm);
    test_enode.file.mm = mm;
    ternfs_file_discard(&test_enode);
    if (test_enode.file.mm ||
        atomic_read(&test_enode.file.transient_err) != -ENOSPC ||
        atomic_read(&mm->mm_count) != mm_count) {
        pr_err("ternfs-transient-test: discard replaced existing error\n");
        err = -EINVAL;
    }
    if (!err) { err = check_inode_unlocked("preexisting error"); }
    return err;
}

static int cleanup_thread_fn(void* data) {
    struct cleanup_thread* cleanup = data;

    complete(&cleanup->started);
    ternfs_file_discard(cleanup->enode);
    complete(&cleanup->done);
    return 0;
}

static int wait_for_inode_lock(struct inode* inode) {
    unsigned long deadline = jiffies + 2 * HZ;

    while (!inode_is_locked(inode)) {
        if (time_after(jiffies, deadline)) { return -ETIMEDOUT; }
        cond_resched();
    }
    return 0;
}

static int test_flush_wait(struct mm_struct* mm) {
    long rss = file_pages(mm);
    int mm_count = atomic_read(&mm->mm_count);
    struct ternfs_transient_span* span = NULL;
    struct page* page = NULL;
    struct cleanup_thread cleanup;
    struct task_struct* task = NULL;
    bool extra_page_ref = false;
    bool task_started = false;
    int err = 0;

    prepare_file(TERNFS_FILE_STATUS_WRITING, 0, 0);
    mmgrab(mm);
    test_enode.file.mm = mm;
    span = ternfs_new_transient_span(&test_enode, 0);
    if (!span) {
        err = -ENOMEM;
        goto out;
    }
    span->started_flushing = true;
    page = add_held_page(&span->pages, mm, 0x7a);
    if (!page) {
        err = -ENOMEM;
        goto out;
    }
    extra_page_ref = true;

    cleanup.enode = &test_enode;
    init_completion(&cleanup.started);
    init_completion(&cleanup.done);
    task = kthread_run(cleanup_thread_fn, &cleanup, "ternfs-transient-test");
    if (IS_ERR(task)) {
        err = PTR_ERR(task);
        task = NULL;
        goto out;
    }
    task_started = true;
    if (!wait_for_completion_timeout(&cleanup.started, 2 * HZ)) {
        err = -ETIMEDOUT;
        goto out;
    }
    err = wait_for_inode_lock(&test_enode.inode);
    if (err) { goto out; }
    if (wait_for_completion_timeout(
            &cleanup.done, msecs_to_jiffies(20))) {
        pr_err("ternfs-transient-test: cleanup bypassed flush barrier\n");
        err = -EINVAL;
        goto out;
    }
    if (test_enode.file.mm != mm ||
        atomic_read(&mm->mm_count) != mm_count + 1 ||
        file_pages(mm) != rss + 1 || page_ref_count(page) != 2) {
        pr_err("ternfs-transient-test: mm/pages released before flush\n");
        err = -EINVAL;
        goto out;
    }

out:
    if (span) {
        if (!ternfs_put_transient_span(span)) {
            pr_err("ternfs-transient-test: flushing final put retained span\n");
            up(&test_enode.file.flushing_span_sema);
            err = -EINVAL;
        }
        span = NULL;
    }
    if (task_started) {
        if (!wait_for_completion_timeout(&cleanup.done, 2 * HZ)) {
            pr_err("ternfs-transient-test: cleanup did not finish\n");
            err = -ETIMEDOUT;
        }
        kthread_stop(task);
    } else if (test_enode.file.mm) {
        // Setup failed before a waiter existed; make its cleanup barrier free.
        up(&test_enode.file.flushing_span_sema);
        ternfs_file_discard(&test_enode);
    }
    if (extra_page_ref) {
        if (!err) { err = check_held_page(page, 0x7a, "flushing page"); }
        put_page(page);
    }
    if (!err &&
        (test_enode.file.mm || atomic_read(&mm->mm_count) != mm_count ||
         file_pages(mm) != rss ||
         atomic_read(&test_enode.file.transient_err) != -EIO)) {
        pr_err("ternfs-transient-test: post-flush cleanup mismatch\n");
        err = -EINVAL;
    }
    if (!err) { err = check_inode_unlocked("flush wait"); }
    return err;
}

static int __init transient_test_init(void) {
    struct mm_struct* mm = current->mm;
    int err;

    if (!mm) { return -EINVAL; }
    memset(&test_enode, 0, sizeof(test_enode));
    inode_init_once(&test_enode.inode);
    err = ternfs_file_init();
    if (err) { return err; }

    err = test_untouched_states();
    if (err) { goto out; }
    err = test_nonfinal_span_put(mm);
    if (err) { goto out; }
    err = test_discard_pages(mm);
    if (err) { goto out; }
    err = test_direct_release_idempotent(mm);
    if (err) { goto out; }
    err = test_preexisting_error(mm);
    if (err) { goto out; }
    err = test_flush_wait(mm);

out:
    ternfs_file_exit();
    if (!err) {
        pr_info(
            "ternfs-transient-test: PASS "
            "(span refs, pages, RSS, mm, idempotence, non-owner, flush wait)\n"
        );
    }
    return err;
}

static void __exit transient_test_exit(void) {}

module_init(transient_test_init);
module_exit(transient_test_exit);
