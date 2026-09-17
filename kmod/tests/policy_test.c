// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/module.h>

// Exercise the production cache implementation with real kernel allocation,
// hash tables and RCU, without starting metadata or block servers.
#include "../policy.c"

MODULE_LICENSE("GPL");

static struct ternfs_policy* entries[257];

static int check_policy(unsigned index, u64 expected) {
    struct ternfs_policy_body body;
    ternfs_get_policy_body(entries[index], &body);
    if (body.len != sizeof(expected) ||
        memcmp(body.body, &expected, sizeof(expected)) != 0) {
        pr_err("ternfs-policy-test: inode %u has another inode's policy\n", index + 1);
        return -EINVAL;
    }
    return 0;
}

static int __init policy_test_init(void) {
    int err = ternfs_policy_init();
    if (err) { return err; }

    unsigned i;
    // More distinct inode keys than buckets guarantees a collision. Use
    // distinct opaque payloads so any alias changes observable cache contents.
    for (i = 0; i < ARRAY_SIZE(entries); i++) {
        u64 value = i + 1;
        entries[i] = ternfs_upsert_policy(value, BLOCK_POLICY_TAG, (char*)&value, sizeof(value));
        if (IS_ERR(entries[i])) {
            err = PTR_ERR(entries[i]);
            goto out;
        }
    }
    for (i = 0; i < ARRAY_SIZE(entries); i++) {
        err = check_policy(i, i + 1);
        if (err) { goto out; }
    }

    // Reusing/updating one key must retain its object identity and leave all
    // other inode keys untouched, including those sharing its bucket.
    u64 replacement = 1000;
    struct ternfs_policy* updated = ternfs_upsert_policy(
        1, BLOCK_POLICY_TAG, (char*)&replacement, sizeof(replacement)
    );
    if (IS_ERR(updated)) {
        err = PTR_ERR(updated);
        goto out;
    }
    if (updated != entries[0]) {
        pr_err("ternfs-policy-test: update replaced the policy object\n");
        err = -EINVAL;
        goto out;
    }
    err = check_policy(0, replacement);
    if (err) { goto out; }
    for (i = 1; i < ARRAY_SIZE(entries); i++) {
        err = check_policy(i, i + 1);
        if (err) { goto out; }
    }

    u64 other_tag_value = 2000;
    struct ternfs_policy* other_tag = ternfs_upsert_policy(
        1, STRIPE_POLICY_TAG, (char*)&other_tag_value, sizeof(other_tag_value)
    );
    if (IS_ERR(other_tag)) {
        err = PTR_ERR(other_tag);
        goto out;
    }
    if (other_tag == entries[0]) {
        pr_err("ternfs-policy-test: distinct tags share a policy object\n");
        err = -EINVAL;
        goto out;
    }
    err = check_policy(0, replacement);

out:
    ternfs_policy_exit();
    // Updated bodies have callbacks implemented by this test module.
    rcu_barrier();
    if (!err) {
        pr_info("ternfs-policy-test: PASS (257 inode keys, update isolation, distinct tags)\n");
    }
    return err;
}

static void __exit policy_test_exit(void) {}

module_init(policy_test_init);
module_exit(policy_test_exit);
