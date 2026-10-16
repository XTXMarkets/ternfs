// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/module.h>
#include <linux/slab.h>

#include "../inode.h"
#include "../policy_update.h"

static unsigned allocation_count;
static unsigned fail_allocation;

static void* test_policy_kmalloc(size_t size, gfp_t flags) {
    allocation_count++;
    if (allocation_count == fail_allocation) { return NULL; }
    return kmalloc(size, flags);
}

// Inject failures only into this copy of the production policy cache. The
// helper under test and all successful allocations, locks and RCU callbacks
// remain production code.
#undef kmalloc
#define kmalloc(size, flags) test_policy_kmalloc(size, flags)
#include "../policy.c"
#undef kmalloc
#include "../policy_update.c"

MODULE_LICENSE("GPL");

enum policy_slot {
    TEST_BLOCK,
    TEST_SPAN,
    TEST_STRIPE,
    TEST_SNAPSHOT,
    TEST_POLICY_COUNT,
};

static struct ternfs_inode test_inode;
static struct ternfs_policy_body test_bodies[TEST_POLICY_COUNT];
static struct ternfs_policy* inherited[TEST_POLICY_COUNT];

static struct ternfs_policy** policy_slot(unsigned index) {
    switch (index) {
    case TEST_BLOCK: return &test_inode.block_policy;
    case TEST_SPAN: return &test_inode.span_policy;
    case TEST_STRIPE: return &test_inode.stripe_policy;
    case TEST_SNAPSHOT: return &test_inode.snapshot_policy;
    default: BUG();
    }
}

static u8 policy_tag(unsigned index) {
    switch (index) {
    case TEST_BLOCK: return BLOCK_POLICY_TAG;
    case TEST_SPAN: return SPAN_POLICY_TAG;
    case TEST_STRIPE: return STRIPE_POLICY_TAG;
    case TEST_SNAPSHOT: return SNAPSHOT_POLICY_TAG;
    default: BUG();
    }
}

static void set_body(unsigned index, u8 marker) {
    test_bodies[index].len = 3;
    test_bodies[index].body[0] = marker;
    test_bodies[index].body[1] = marker + 1;
    test_bodies[index].body[2] = marker + 2;
}

static bool policy_matches(
    struct ternfs_policy* policy, const struct ternfs_policy_body* expected
) {
    struct ternfs_policy_body actual;
    if (!policy || IS_ERR(policy)) { return false; }
    ternfs_get_policy_body(policy, &actual);
    return actual.len == expected->len &&
        memcmp(actual.body, expected->body, expected->len) == 0;
}

static bool slots_are_valid(void) {
    unsigned i;
    for (i = 0; i < TEST_POLICY_COUNT; i++) {
        struct ternfs_policy* policy = READ_ONCE(*policy_slot(i));
        if (!policy || IS_ERR(policy)) { return false; }
    }
    return true;
}

static bool other_slots_inherited(unsigned changed) {
    unsigned i;
    for (i = 0; i < TEST_POLICY_COUNT; i++) {
        if (i != changed && READ_ONCE(*policy_slot(i)) != inherited[i]) {
            return false;
        }
    }
    return true;
}

static void prepare_inode(u64 ino) {
    unsigned i;
    memset(&test_inode, 0, sizeof(test_inode));
    memset(test_bodies, 0, sizeof(test_bodies));
    test_inode.inode.i_ino = ino;
    test_inode.inode.i_mode = S_IFDIR | 0750;
    test_inode.inode.i_size = 12345;
    test_inode.inode.i_blocks = 27;
    test_inode.mtime = 0x1111222233334444ull;
    test_inode.edge_creation_time = 0x5555666677778888ull;
    test_inode.dir.mtime_expiry = 0x12345678;
    test_inode.getattr_expiry = 0x87654321;
    test_inode.getattr_async_seqno = -73;
    for (i = 0; i < TEST_POLICY_COUNT; i++) {
        *policy_slot(i) = inherited[i];
    }
}

static bool unrelated_state_unchanged(u64 ino) {
    return test_inode.inode.i_ino == ino &&
        test_inode.inode.i_mode == (S_IFDIR | 0750) &&
        test_inode.inode.i_size == 12345 &&
        test_inode.inode.i_blocks == 27 &&
        test_inode.mtime == 0x1111222233334444ull &&
        test_inode.edge_creation_time == 0x5555666677778888ull &&
        test_inode.dir.mtime_expiry == 0x12345678 &&
        test_inode.getattr_expiry == 0x87654321 &&
        test_inode.getattr_async_seqno == -73;
}

static int update_policies(void) {
    return ternfs_update_dir_policies(
        &test_inode,
        &test_bodies[TEST_BLOCK],
        &test_bodies[TEST_SPAN],
        &test_bodies[TEST_STRIPE],
        &test_bodies[TEST_SNAPSHOT]
    );
}

#define CHECK(condition, description) do { \
    if (!(condition)) { \
        pr_err("ternfs-policy-update-test: FAIL: %s\n", description); \
        return -EINVAL; \
    } \
} while (0)

static int seed_inherited_policies(void) {
    unsigned i;
    for (i = 0; i < TEST_POLICY_COUNT; i++) {
        char body[] = { (char)(0xa0 + i), (char)(0xb0 + i) };
        inherited[i] = ternfs_upsert_policy(
            0x100 + i, policy_tag(i), body, sizeof(body)
        );
        if (IS_ERR(inherited[i])) { return PTR_ERR(inherited[i]); }
    }
    return 0;
}

static int test_empty_update(void) {
    u64 ino = 0x200;
    unsigned before;
    unsigned i;
    prepare_inode(ino);
    before = allocation_count;
    fail_allocation = before + 1;

    CHECK(update_policies() == 0, "empty policies are a no-op");
    CHECK(allocation_count == before, "empty policies allocate");
    for (i = 0; i < TEST_POLICY_COUNT; i++) {
        CHECK(*policy_slot(i) == inherited[i], "empty policy replaced inherited pointer");
    }
    CHECK(slots_are_valid(), "empty update published an invalid pointer");
    CHECK(unrelated_state_unchanged(ino), "empty update changed inode metadata");
    fail_allocation = 0;
    return 0;
}

static int test_new_object_failure(void) {
    u64 ino = 0x201;
    unsigned before;
    prepare_inode(ino);
    set_body(TEST_BLOCK, 0x10);
    before = allocation_count;
    fail_allocation = before + 1;

    CHECK(update_policies() == -ENOMEM, "new object failure errno");
    CHECK(allocation_count == before + 1, "new object failure position");
    CHECK(*policy_slot(TEST_BLOCK) == inherited[TEST_BLOCK],
          "new object failure replaced block pointer");
    CHECK(other_slots_inherited(TEST_BLOCK), "new object failure changed another slot");
    CHECK(slots_are_valid(), "new object failure published ERR_PTR");
    CHECK(unrelated_state_unchanged(ino), "new object failure changed inode metadata");

    fail_allocation = 0;
    CHECK(update_policies() == 0, "block retry failed");
    CHECK(*policy_slot(TEST_BLOCK) != inherited[TEST_BLOCK], "block retry not published");
    CHECK(policy_matches(*policy_slot(TEST_BLOCK), &test_bodies[TEST_BLOCK]),
          "block retry body");
    CHECK(other_slots_inherited(TEST_BLOCK), "block retry changed another slot");
    CHECK(slots_are_valid(), "block retry published invalid pointer");
    return 0;
}

static int test_new_body_failure(unsigned target, u64 ino, u8 marker) {
    unsigned before;
    prepare_inode(ino);
    set_body(target, marker);
    before = allocation_count;
    fail_allocation = before + 2;

    CHECK(update_policies() == -ENOMEM, "new body failure errno");
    CHECK(allocation_count == before + 2, "new body failure position");
    CHECK(*policy_slot(target) == inherited[target],
          "new body failure replaced inherited pointer");
    CHECK(other_slots_inherited(target), "new body failure changed another slot");
    CHECK(slots_are_valid(), "new body failure published ERR_PTR");
    CHECK(unrelated_state_unchanged(ino), "new body failure changed inode metadata");

    fail_allocation = 0;
    CHECK(update_policies() == 0, "new body retry failed");
    CHECK(*policy_slot(target) != inherited[target], "new body retry not published");
    CHECK(policy_matches(*policy_slot(target), &test_bodies[target]),
          "new body retry contents");
    CHECK(other_slots_inherited(target), "new body retry changed another slot");
    CHECK(slots_are_valid(), "new body retry published invalid pointer");
    return 0;
}

static int test_existing_body_failure(void) {
    u64 ino = 0x204;
    struct ternfs_policy* existing;
    struct ternfs_policy_body old_body;
    prepare_inode(ino);
    set_body(TEST_STRIPE, 0x40);
    CHECK(update_policies() == 0, "seed existing stripe policy");
    existing = *policy_slot(TEST_STRIPE);
    old_body = test_bodies[TEST_STRIPE];
    set_body(TEST_STRIPE, 0x50);
    fail_allocation = allocation_count + 1;

    CHECK(update_policies() == -ENOMEM, "existing body failure errno");
    CHECK(*policy_slot(TEST_STRIPE) == existing,
          "existing body failure replaced policy object");
    CHECK(policy_matches(existing, &old_body), "existing body failure changed contents");
    CHECK(other_slots_inherited(TEST_STRIPE), "existing body failure changed another slot");
    CHECK(slots_are_valid(), "existing body failure published ERR_PTR");
    CHECK(unrelated_state_unchanged(ino), "existing body failure changed inode metadata");

    fail_allocation = 0;
    CHECK(update_policies() == 0, "existing body retry failed");
    CHECK(*policy_slot(TEST_STRIPE) == existing, "existing body retry replaced object");
    CHECK(policy_matches(existing, &test_bodies[TEST_STRIPE]),
          "existing body retry contents");
    CHECK(other_slots_inherited(TEST_STRIPE), "existing body retry changed another slot");
    return 0;
}

static int test_partial_update(void) {
    u64 ino = 0x205;
    prepare_inode(ino);
    set_body(TEST_BLOCK, 0x60);
    set_body(TEST_SPAN, 0x70);
    set_body(TEST_SNAPSHOT, 0x80);
    fail_allocation = allocation_count + 3;

    CHECK(update_policies() == -ENOMEM, "later slot failure errno");
    CHECK(*policy_slot(TEST_BLOCK) != inherited[TEST_BLOCK],
          "successful prefix was not published");
    CHECK(policy_matches(*policy_slot(TEST_BLOCK), &test_bodies[TEST_BLOCK]),
          "successful prefix body");
    CHECK(*policy_slot(TEST_SPAN) == inherited[TEST_SPAN],
          "failing later slot was published");
    CHECK(*policy_slot(TEST_STRIPE) == inherited[TEST_STRIPE],
          "empty later slot changed");
    CHECK(*policy_slot(TEST_SNAPSHOT) == inherited[TEST_SNAPSHOT],
          "unvisited later slot changed");
    CHECK(slots_are_valid(), "partial update published ERR_PTR");
    CHECK(unrelated_state_unchanged(ino), "partial update changed inode metadata");
    fail_allocation = 0;
    return 0;
}

static int __init policy_update_test_init(void) {
    int err = ternfs_policy_init();
    if (err) { return err; }

    err = seed_inherited_policies();
    if (err) { goto out; }
    err = test_empty_update();
    if (err) { goto out; }
    err = test_new_object_failure();
    if (err) { goto out; }
    err = test_new_body_failure(TEST_SPAN, 0x202, 0x20);
    if (err) { goto out; }
    err = test_existing_body_failure();
    if (err) { goto out; }
    err = test_new_body_failure(TEST_SNAPSHOT, 0x203, 0x30);
    if (err) { goto out; }
    err = test_partial_update();

out:
    fail_allocation = 0;
    ternfs_policy_exit();
    // Successful body replacements use callbacks defined in this module.
    rcu_barrier();
    if (!err) {
        pr_info("ternfs-policy-update-test: PASS (four slots, OOM preservation, retries, RCU)\n");
    }
    return err;
}

static void __exit policy_update_test_exit(void) {}

module_init(policy_update_test_init);
module_exit(policy_update_test_exit);
