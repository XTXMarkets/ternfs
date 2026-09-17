// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "inode.h"
#include "policy_update.h"

int ternfs_update_dir_policies(
    struct ternfs_inode* enode,
    struct ternfs_policy_body* block,
    struct ternfs_policy_body* span,
    struct ternfs_policy_body* stripe,
    struct ternfs_policy_body* snapshot
) {
    struct {
        struct ternfs_policy_body* body;
        struct ternfs_policy** policy;
        u8 tag;
    } updates[] = {
        { block, &enode->block_policy, BLOCK_POLICY_TAG },
        { span, &enode->span_policy, SPAN_POLICY_TAG },
        { stripe, &enode->stripe_policy, STRIPE_POLICY_TAG },
        { snapshot, &enode->snapshot_policy, SNAPSHOT_POLICY_TAG },
    };
    int i;
    for (i = 0; i < ARRAY_SIZE(updates); i++) {
        if (!updates[i].body->len) { continue; }
        struct ternfs_policy* policy = ternfs_upsert_policy(
            enode->inode.i_ino, updates[i].tag,
            updates[i].body->body, updates[i].body->len
        );
        // Never publish an error pointer. Keep this slot's previous policy
        // and let the caller report failure without advancing its expiry.
        if (IS_ERR(policy)) { return PTR_ERR(policy); }
        WRITE_ONCE(*updates[i].policy, policy);
    }
    return 0;
}
