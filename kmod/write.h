// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_WRITE_H
#define _TERNFS_WRITE_H

#include <linux/types.h>

struct iov_iter;
struct list_head;
struct mm_struct;
struct page;

void ternfs_account_write_pages(struct mm_struct* mm, long count);
struct page* ternfs_alloc_write_page(struct mm_struct* mm);

// Append up to count bytes to private write pages. page->index is the next
// offset within the last page; zero means that page is full. A NULL iterator
// appends zeros. On failure, return bytes already copied or a negative errno.
// The caller owns the list, serializes access, and keeps mm alive.
ssize_t ternfs_copy_write_pages(
    struct list_head* pages, struct mm_struct* mm,
    struct iov_iter* from, size_t count
);

#endif
