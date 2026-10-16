// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/module.h>
#include <linux/mm.h>
#include <linux/highmem.h>
#include <linux/sched/mm.h>
#include <linux/uio.h>
#include <linux/version.h>

static unsigned allocation_count;
static unsigned fail_allocation;

static struct page* test_alloc_page(gfp_t flags) {
    if (++allocation_count == fail_allocation) { return NULL; }
    return alloc_page(flags);
}

// Limit fault injection to this copy of the production helper. All successful
// allocations and all iterators, page lists and RSS counters are real.
#undef alloc_page
#define alloc_page(flags) test_alloc_page(flags)
#include "../write.c"
#undef alloc_page

MODULE_LICENSE("GPL");

static long file_pages(struct mm_struct* mm) {
#if (LINUX_VERSION_CODE < KERNEL_VERSION(6,2,0))
    return atomic_long_read(&mm->rss_stat.count[MM_FILEPAGES]);
#else
    return percpu_counter_sum(&mm->rss_stat[MM_FILEPAGES]);
#endif
}

static void release_test_pages(struct list_head* pages, struct mm_struct* mm) {
    struct page* page;
    struct page* next;
    list_for_each_entry_safe(page, next, pages, lru) {
        list_del(&page->lru);
        put_page(page);
        ternfs_account_write_pages(mm, -1);
    }
}

static ssize_t copy_bytes(
    struct list_head* pages, struct mm_struct* mm,
    char* data, size_t available, size_t requested
) {
    struct kvec vec = { .iov_base = data, .iov_len = available };
    struct iov_iter from;
    iov_iter_kvec(&from, WRITE, &vec, 1, available);
    return ternfs_copy_write_pages(pages, mm, &from, requested);
}

static ssize_t copy_faulting_user_bytes(
    struct list_head* pages, struct mm_struct* mm, size_t requested
) {
    struct iovec vec = {
        .iov_base = (void __user*)1UL,
        .iov_len = requested,
    };
    struct iov_iter from;
    iov_iter_init(&from, WRITE, &vec, 1, requested);
    return ternfs_copy_write_pages(pages, mm, &from, requested);
}

static bool check_pages(
    struct list_head* pages, struct mm_struct* mm, long baseline,
    const char* expected, size_t size
) {
    size_t offset = 0;
    unsigned count = 0;
    struct page* page;
    list_for_each_entry(page, pages, lru) {
        if (offset >= size) { return false; }
        size_t take = min(size - offset, (size_t)PAGE_SIZE);
        char* data = kmap(page);
        bool matches = memcmp(data, expected + offset, take) == 0;
        kunmap(page);
        if (!matches || page->index != take % PAGE_SIZE ||
            page_ref_count(page) != 1) {
            return false;
        }
        offset += take;
        count++;
    }
    return offset == size && file_pages(mm) == baseline + count;
}

#define CHECK(condition, description) do { \
    if (!(condition)) { \
        pr_err("ternfs-write-test: FAIL: %s\n", description); \
        err = -EINVAL; \
        goto out; \
    } \
} while (0)

static int __init write_test_init(void) {
    struct mm_struct* mm = current->mm;
    if (!mm) { return -EINVAL; }
    mmgrab(mm);
    long baseline = file_pages(mm);
    LIST_HEAD(pages);
    size_t size = 2 * PAGE_SIZE + 7;
    char* data = kmalloc(size, GFP_KERNEL);
    int err = 0;
    if (!data) {
        err = -ENOMEM;
        goto out;
    }
    memset(data, 0x5c, size);

    CHECK(copy_bytes(&pages, mm, data, 0, PAGE_SIZE) == -EFAULT, "empty iterator");
    CHECK(check_pages(&pages, mm, baseline, NULL, 0), "empty allocation released");
    CHECK(copy_faulting_user_bytes(&pages, mm, PAGE_SIZE) == -EFAULT,
          "faulting user iterator");
    CHECK(check_pages(&pages, mm, baseline, NULL, 0), "faulting allocation released");

    CHECK(copy_bytes(&pages, mm, data, 3, 3) == 3, "partial page write");
    CHECK(copy_faulting_user_bytes(&pages, mm, 7) == -EFAULT,
          "faulting user iterator on partial page");
    CHECK(check_pages(&pages, mm, baseline, data, 3),
          "previous bytes and RSS retained after fault");

    CHECK(ternfs_copy_write_pages(&pages, mm, NULL, PAGE_SIZE - 3) == PAGE_SIZE - 3,
          "zero-fill remainder");
    memset(data + 3, 0, PAGE_SIZE - 3);
    CHECK(check_pages(&pages, mm, baseline, data, PAGE_SIZE), "zero-filled data");
    CHECK(copy_bytes(&pages, mm, data, 0, 7) == -EFAULT, "empty iterator after full page");
    CHECK(check_pages(&pages, mm, baseline, data, PAGE_SIZE), "empty next page removed");
    release_test_pages(&pages, mm);

    memset(data, 0x5c, size);
    CHECK(copy_bytes(&pages, mm, data, 7, PAGE_SIZE) == 7, "short iterator returns progress");
    CHECK(check_pages(&pages, mm, baseline, data, 7), "short iterator preserves data");
    CHECK(copy_bytes(&pages, mm, data + 7, size - 7, size - 7) == size - 7,
          "continue across page boundaries");
    CHECK(check_pages(&pages, mm, baseline, data, size), "multi-page contents and RSS");
    release_test_pages(&pages, mm);

    CHECK(copy_bytes(&pages, mm, data, size, 17) == 17, "respect requested count");
    CHECK(check_pages(&pages, mm, baseline, data, 17), "bounded copy");
    release_test_pages(&pages, mm);

    fail_allocation = allocation_count + 1;
    CHECK(copy_bytes(&pages, mm, data, size, size) == -ENOMEM, "first allocation failure");
    CHECK(check_pages(&pages, mm, baseline, NULL, 0), "no pages or RSS after failure");

    fail_allocation = allocation_count + 2;
    CHECK(copy_bytes(&pages, mm, data, size, size) == PAGE_SIZE, "later failure returns progress");
    CHECK(check_pages(&pages, mm, baseline, data, PAGE_SIZE), "retain completed page");
    fail_allocation = 0;
    CHECK(copy_bytes(&pages, mm, data + PAGE_SIZE, size - PAGE_SIZE, size - PAGE_SIZE)
          == size - PAGE_SIZE, "resume after allocation failure");
    CHECK(check_pages(&pages, mm, baseline, data, size), "resumed contents and RSS");

out:
    release_test_pages(&pages, mm);
    kfree(data);
    if (file_pages(mm) != baseline) {
        pr_err("ternfs-write-test: RSS accounting did not return to baseline\n");
        err = -EINVAL;
    }
    mmdrop(mm);
    if (!err) {
        pr_info("ternfs-write-test: PASS (zero progress, partial writes, zero fill, allocation failures, RSS)\n");
    }
    return err;
}

static void __exit write_test_exit(void) {}

module_init(write_test_init);
module_exit(write_test_exit);
