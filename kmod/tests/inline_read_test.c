// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/highmem.h>
#include <linux/mm.h>
#include <linux/module.h>

#include "../inline_read.h"

MODULE_LICENSE("GPL");

static u8 body[PAGE_SIZE];

static void poison_page(struct page* page) {
    u8* data = kmap_atomic(page);
    memset(data, 0xa5, PAGE_SIZE);
    kunmap_atomic(data);
}

static int check_page(struct page* page, const u8* expected, size_t len, const char* test) {
    size_t offset;
    size_t bad_offset = PAGE_SIZE;
    u8 bad_actual = 0;
    u8 bad_expected = 0;
    u8* data = kmap_atomic(page);

    for (offset = 0; offset < PAGE_SIZE; offset++) {
        u8 value = offset < len ? expected[offset] : 0;
        if (data[offset] != value) {
            bad_offset = offset;
            bad_actual = data[offset];
            bad_expected = value;
            break;
        }
    }
    kunmap_atomic(data);

    if (bad_offset != PAGE_SIZE) {
        pr_err(
            "ternfs-inline-read-test: %s mismatch at %zu: got %#x expected %#x\n",
            test, bad_offset, bad_actual, bad_expected
        );
        return -EINVAL;
    }
    if (page_ref_count(page) != 1) {
        pr_err(
            "ternfs-inline-read-test: %s changed page refcount to %d\n",
            test, page_ref_count(page)
        );
        return -EINVAL;
    }
    return 0;
}

static int run_poisoned_case(struct page* page, size_t len, const char* test) {
    poison_page(page);
    ternfs_fill_inline_page(page, len ? body : NULL, len);
    return check_page(page, body, len, test);
}

static int __init inline_read_test_init(void) {
    struct page* page = alloc_page(GFP_KERNEL);
    int err;
    size_t i;

    if (!page) { return -ENOMEM; }
    for (i = 0; i < ARRAY_SIZE(body); i++) {
        body[i] = (u8)(i * 37 + 11);
    }

    err = run_poisoned_case(page, 0, "empty inline data");
    if (err) { goto out; }
    err = run_poisoned_case(page, 1, "one-byte inline data");
    if (err) { goto out; }
    err = run_poisoned_case(page, 255, "255-byte inline data");
    if (err) { goto out; }
    err = run_poisoned_case(page, PAGE_SIZE, "full-page inline data");
    if (err) { goto out; }

    // Reuse without poisoning between fills. A shorter replacement must clear
    // both the previous body and all remaining bytes in the page.
    ternfs_fill_inline_page(page, body, 255);
    body[0] ^= 0xff;
    ternfs_fill_inline_page(page, body, 1);
    err = check_page(page, body, 1, "shorter replacement");
    if (err) { goto out; }
    ternfs_fill_inline_page(page, NULL, 0);
    err = check_page(page, body, 0, "empty replacement");

out:
    put_page(page);
    if (!err) {
        pr_info(
            "ternfs-inline-read-test: PASS (0, 1, 255, full page, repeated reuse)\n"
        );
    }
    return err;
}

static void __exit inline_read_test_exit(void) {}

module_init(inline_read_test_init);
module_exit(inline_read_test_exit);
