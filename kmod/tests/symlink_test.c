// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/highmem.h>
#include <linux/mm.h>
#include <linux/module.h>
#include <linux/slab.h>
#include <linux/string.h>

#include "../file.h"
#include "../page_compat.h"
#include "../span.h"

MODULE_LICENSE("GPL");

enum test_mode {
    TEST_EMPTY,
    TEST_INLINE,
    TEST_BLOCK,
    TEST_GETATTR_ERROR,
    TEST_SPAN_ERROR,
    TEST_BUFFER_ERROR,
    TEST_PAGE_ERROR,
    TEST_FETCH_ERROR,
};

static enum test_mode mode;
static struct ternfs_inline_span inline_span;
static struct ternfs_block_span block_span;
static const char link_body[] = "target/path";

static unsigned buffer_allocations;
static unsigned buffer_releases;
static unsigned page_allocations;
static unsigned page_releases;
static unsigned span_gets;
static unsigned span_puts;
static unsigned error_span_puts;
static unsigned fetches;

int ternfs_debug_output;

static void reset_counters(void) {
    buffer_allocations = 0;
    buffer_releases = 0;
    page_allocations = 0;
    page_releases = 0;
    span_gets = 0;
    span_puts = 0;
    error_span_puts = 0;
    fetches = 0;
}

static void* test_kmalloc(size_t size, gfp_t flags) {
    void* buffer;

    if (mode == TEST_BUFFER_ERROR) { return NULL; }
    buffer = kmalloc(size, flags);
    if (buffer) { buffer_allocations++; }
    return buffer;
}

static void test_kfree(const void* buffer) {
    if (buffer) { buffer_releases++; }
    kfree(buffer);
}

static struct page* test_alloc_page(gfp_t flags) {
    struct page* page;

    if (mode == TEST_PAGE_ERROR) { return NULL; }
    page = alloc_page(flags);
    if (page) { page_allocations++; }
    return page;
}

static void test_put_pages_list(struct list_head* pages) {
    struct page* page;
    struct page* next;

    list_for_each_entry_safe(page, next, pages, lru) {
        list_del(&page->lru);
        put_page(page);
        page_releases++;
    }
}

static int test_do_getattr(struct ternfs_inode* enode, int cache_timeout_type) {
    return mode == TEST_GETATTR_ERROR ? -ETIMEDOUT : 0;
}

static struct ternfs_span* test_get_span(
    struct ternfs_fs_info* fs_info, struct ternfs_file_spans* spans, u64 offset
) {
    span_gets++;
    if (mode == TEST_SPAN_ERROR) { return ERR_PTR(-EIO); }
    if (mode == TEST_INLINE) { return &inline_span.span; }
    return &block_span.span;
}

static void test_put_span(struct ternfs_span* span) {
    if (IS_ERR(span)) {
        error_span_puts++;
        return;
    }
    span_puts++;
}

static int test_span_get_pages(
    struct ternfs_block_span* span, struct address_space* mapping,
    struct list_head* pages, unsigned nr_pages, struct list_head* extra_pages
) {
    struct page* page;
    char* data;

    fetches++;
    if (mode == TEST_FETCH_ERROR) {
        page = alloc_page(GFP_KERNEL);
        if (!page) { return -ENOMEM; }
        page_allocations++;
        list_add_tail(&page->lru, extra_pages);
        return -EIO;
    }

    page = list_first_entry(pages, struct page, lru);
    data = kmap(page);
    memcpy(data, link_body, sizeof(link_body) - 1);
    kunmap(page);
    return 0;
}

#define ternfs_do_getattr test_do_getattr
#define ternfs_get_span test_get_span
#define ternfs_put_span test_put_span
#define ternfs_span_get_pages test_span_get_pages
#define put_pages_list test_put_pages_list
#undef kmalloc
#define kmalloc(size, flags) test_kmalloc(size, flags)
#define kfree(buffer) test_kfree(buffer)
#undef alloc_page
#define alloc_page(flags) test_alloc_page(flags)
#include "../symlink.c"
#undef alloc_page
#undef kfree
#undef kmalloc
#undef put_pages_list
#undef ternfs_span_get_pages
#undef ternfs_put_span
#undef ternfs_get_span
#undef ternfs_do_getattr

static int check_ownership(
    unsigned expected_buffers, unsigned expected_pages,
    unsigned expected_gets, unsigned expected_puts, unsigned expected_fetches,
    const char* test
) {
    if (buffer_allocations != buffer_releases + expected_buffers ||
        page_allocations != page_releases + expected_pages ||
        span_gets != expected_gets || span_puts != expected_puts ||
        error_span_puts != 0 || fetches != expected_fetches) {
        pr_err(
            "ternfs-symlink-test: %s ownership mismatch: "
            "buffers=%u/%u pages=%u/%u spans=%u/%u bad_put=%u fetches=%u\n",
            test, buffer_allocations, buffer_releases,
            page_allocations, page_releases, span_gets, span_puts,
            error_span_puts, fetches
        );
        return -EINVAL;
    }
    return 0;
}

static void prepare_inode(struct ternfs_inode* enode, size_t size) {
    static struct super_block super;
    static struct address_space mapping;

    memset(enode, 0, sizeof(*enode));
    enode->inode.i_ino = ((u64)TERNFS_INODE_SYMLINK << 61) | 1;
    enode->inode.i_size = size;
    enode->inode.i_sb = &super;
    enode->inode.i_mapping = &mapping;
}

static void prepare_spans(size_t size) {
    memset(&inline_span, 0, sizeof(inline_span));
    inline_span.span.start = 0;
    inline_span.span.end = size;
    inline_span.span.storage_class = TERNFS_INLINE_STORAGE;
    inline_span.len = size;
    memcpy(inline_span.body, link_body, size);

    memset(&block_span, 0, sizeof(block_span));
    block_span.span.start = 0;
    block_span.span.end = size;
    block_span.span.storage_class = 2;
}

static int expect_error(
    struct ternfs_inode* enode, enum test_mode test_mode, int expected,
    unsigned expected_gets, unsigned expected_puts, unsigned expected_fetches,
    const char* test
) {
    char* result;
    int err;

    mode = test_mode;
    reset_counters();
    result = ternfs_read_link(enode);
    if (!IS_ERR(result) || PTR_ERR(result) != expected) {
        pr_err("ternfs-symlink-test: %s returned %pe\n", test, result);
        if (!IS_ERR(result)) { ternfs_link_destructor(result); }
        return -EINVAL;
    }
    err = check_ownership(0, 0, expected_gets, expected_puts, expected_fetches, test);
    return err;
}

static int expect_success(
    struct ternfs_inode* enode, enum test_mode test_mode,
    const char* expected, unsigned expected_gets, unsigned expected_puts,
    unsigned expected_fetches, const char* test
) {
    char* result;
    int err;

    mode = test_mode;
    reset_counters();
    result = ternfs_read_link(enode);
    if (IS_ERR(result)) {
        pr_err("ternfs-symlink-test: %s returned %pe\n", test, result);
        return PTR_ERR(result);
    }
    if (strcmp(result, expected) != 0 ||
        result[enode->inode.i_size] != '\0') {
        pr_err("ternfs-symlink-test: %s returned bad contents\n", test);
        ternfs_link_destructor(result);
        return -EINVAL;
    }
    err = check_ownership(1, 0, expected_gets, expected_puts, expected_fetches, test);
    ternfs_link_destructor(result);
    if (err) { return err; }
    return check_ownership(0, 0, expected_gets, expected_puts, expected_fetches, test);
}

static int __init symlink_test_init(void) {
    struct ternfs_inode enode;
    size_t size = sizeof(link_body) - 1;
    int err;

    prepare_spans(size);
    prepare_inode(&enode, size);

    err = expect_error(
        &enode, TEST_GETATTR_ERROR, -ETIMEDOUT, 0, 0, 0, "metadata error"
    );
    if (err) { return err; }
    err = expect_error(
        &enode, TEST_BUFFER_ERROR, -ENOMEM, 0, 0, 0, "buffer allocation error"
    );
    if (err) { return err; }
    err = expect_error(
        &enode, TEST_SPAN_ERROR, -EIO, 1, 0, 0, "span lookup error"
    );
    if (err) { return err; }
    err = expect_error(
        &enode, TEST_PAGE_ERROR, -ENOMEM, 1, 1, 0, "page allocation error"
    );
    if (err) { return err; }
    err = expect_error(
        &enode, TEST_FETCH_ERROR, -EIO, 1, 1, 1, "page fetch error"
    );
    if (err) { return err; }

    prepare_inode(&enode, 0);
    err = expect_success(&enode, TEST_EMPTY, "", 0, 0, 0, "empty link");
    if (err) { return err; }
    prepare_inode(&enode, size);
    err = expect_success(
        &enode, TEST_INLINE, link_body, 1, 1, 0, "inline link"
    );
    if (err) { return err; }
    err = expect_success(
        &enode, TEST_BLOCK, link_body, 1, 1, 1, "block link"
    );
    if (err) { return err; }

    pr_info(
        "ternfs-symlink-test: PASS (metadata, allocation, span, fetch, empty, inline, block)\n"
    );
    return 0;
}

static void __exit symlink_test_exit(void) {}

module_init(symlink_test_init);
module_exit(symlink_test_exit);
