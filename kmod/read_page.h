// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_READ_PAGE_H
#define _TERNFS_READ_PAGE_H

#include <linux/pagemap.h>

// The caller has filled a locked page. Publish its completed state before
// unlocking wakes another reader or lets it start a second read.
static inline void ternfs_complete_read_page(struct page* page) {
    SetPageUptodate(page);
    unlock_page(page);
}

#endif
