// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_INLINE_READ_H
#define _TERNFS_INLINE_READ_H

#include <linux/highmem.h>
#include <linux/mm.h>
#include <linux/string.h>

// The whole page can become visible through mmap, including its bytes past
// EOF. Initialize the tail before the caller marks the page uptodate.
static inline void ternfs_fill_inline_page(struct page* page, const void* body, size_t len) {
    BUG_ON(len > PAGE_SIZE);
    char* dst = kmap_atomic(page);
    if (len) { memcpy(dst, body, len); }
    memset(dst + len, 0, PAGE_SIZE - len);
    kunmap_atomic(dst);
}

#endif
