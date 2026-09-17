// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/highmem.h>
#include <linux/mm.h>
#include <linux/slab.h>

#include "file.h"
#include "log.h"
#include "page_compat.h"
#include "span.h"

void ternfs_link_destructor(void* buf) {
    kfree(buf);
}

char* ternfs_read_link(struct ternfs_inode* enode) {
    ternfs_debug("ino=%016lx", enode->inode.i_ino);

    BUG_ON(ternfs_inode_type(enode->inode.i_ino) != TERNFS_INODE_SYMLINK);
    int err = 0;
    LIST_HEAD(pages);
    LIST_HEAD(extra_pages);
    char* buf = NULL;
    struct ternfs_span* span = NULL;

    err = ternfs_do_getattr(enode, ATTR_CACHE_NORM_TIMEOUT);
    if (err) { goto out; }

    BUG_ON(enode->inode.i_size > PAGE_SIZE);
    size_t size = enode->inode.i_size;
    ternfs_debug("size=%lu", size);

    buf = kmalloc(size + 1, GFP_KERNEL);
    if (!buf) { err = -ENOMEM; goto out; }
    buf[size] = '\0';

    if (size == 0) { goto out; }
    span = ternfs_get_span((struct ternfs_fs_info*)enode->inode.i_sb->s_fs_info, &enode->file.spans, 0);
    BUG_ON(span == NULL);
    if (IS_ERR(span)) {
        err = PTR_ERR(span);
        span = NULL;
        goto out;
    }
    BUG_ON(span->end - span->start != size);

    if (span->storage_class == TERNFS_INLINE_STORAGE) {
        memcpy(buf, TERNFS_INLINE_SPAN(span)->body, size);
    } else {
        struct ternfs_block_span* block_span = TERNFS_BLOCK_SPAN(span);
        struct page* page = alloc_page(GFP_KERNEL);
        if (!page) {
            err = -ENOMEM;
            goto out;
        }
        page->index = 0;
        list_add_tail(&page->lru, &pages);
        err = ternfs_span_get_pages(block_span, enode->inode.i_mapping, &pages, 1, &extra_pages);
        if (err) { goto out; }
        char* page_buf = kmap(page);
        memcpy(buf, page_buf, size);
        kunmap(page);
    }

out:
    if (span) { ternfs_put_span(span); }
    put_pages_list(&pages);
    put_pages_list(&extra_pages);
    if (err) {
        kfree(buf);
        ternfs_debug("get_link err=%d", err);
        return ERR_PTR(err);
    }
    ternfs_debug("link %*pE", (int)size, buf);
    return buf;
}
