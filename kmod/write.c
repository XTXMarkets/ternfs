// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/mm.h>
#include <linux/sched/mm.h>
#include <linux/uio.h>
#include <linux/version.h>

#include "write.h"

void ternfs_account_write_pages(struct mm_struct* mm, long count) {
#if (LINUX_VERSION_CODE < KERNEL_VERSION(6,2,0))
    atomic_long_add(count, &mm->rss_stat.count[MM_FILEPAGES]);
#else
    percpu_counter_add(&mm->rss_stat[MM_FILEPAGES], count);
#endif
}

struct page* ternfs_alloc_write_page(struct mm_struct* mm) {
    // Unwritten bytes stay zero, including padding and forward seeks.
    struct page* page = alloc_page(GFP_KERNEL | __GFP_ZERO);
    if (page) { ternfs_account_write_pages(mm, 1); }
    return page;
}

ssize_t ternfs_copy_write_pages(
    struct list_head* pages, struct mm_struct* mm,
    struct iov_iter* from, size_t count
) {
    size_t written = 0;
    int err;

    while (written < count) {
        struct page* page = list_empty(pages) ? NULL : list_last_entry(pages, struct page, lru);
        if (page == NULL || page->index == 0) {
            page = ternfs_alloc_write_page(mm);
            if (!page) {
                err = -ENOMEM;
                goto out_err;
            }
            page->index = 0;
            list_add_tail(&page->lru, pages);
        }

        size_t size = min(count - written, PAGE_SIZE - page->index);
        size_t copied = from ? copy_page_from_iter(page, page->index, size, from) : size;
        if (unlikely(copied == 0)) {
            // An exhausted or faulting iterator returns a byte count of
            // zero, not a negative errno. Do not retry without progress.
            if (page->index == 0) {
                list_del(&page->lru);
                put_page(page);
                ternfs_account_write_pages(mm, -1);
            }
            err = -EFAULT;
            goto out_err;
        }
        page->index = (page->index + copied) % PAGE_SIZE;
        written += copied;
    }
    return written;

out_err:
    return written ? (ssize_t)written : err;
}
