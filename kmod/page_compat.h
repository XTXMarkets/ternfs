// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_PAGE_COMPAT_H
#define _TERNFS_PAGE_COMPAT_H
#include <linux/version.h>

#if LINUX_VERSION_CODE >= KERNEL_VERSION(6, 13, 0)

static inline void put_pages_list(struct list_head *pages)
{
	struct page *page, *tmp;

  list_for_each_entry_safe(page, tmp, pages, lru) {
      list_del_init(&page->lru);
      put_page(page);
  }
	INIT_LIST_HEAD(pages);
}
#endif

#endif /* _TERNFS_PAGE_COMPAT_H */
