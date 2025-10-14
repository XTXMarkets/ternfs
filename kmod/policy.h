// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

// We index policies by (inode, tag), so that the automatically get updated
// when the getattr for the inode runs. They never get freed, there should
// be very few of them.
#ifndef _TERNFS_POLICY_H
#define _TERNFS_POLICY_H

#include <linux/kernel.h>
#include <linux/spinlock.h>

#include "bincode.h"

struct ternfs_policy;

// Creates or update a specific policy. Very fast unless the policy is unseen so
// far, which is a rare occurrence.
struct ternfs_policy* ternfs_upsert_policy(u64 inode, u8 tag, char* body, int len);

struct ternfs_policy_body {
    u8 len;
    char body[255];
};

void ternfs_get_policy_body(struct ternfs_policy* policy, struct ternfs_policy_body* body);

static inline u8 ternfs_block_policy_len(const char* body, u8 len) {
    BUG_ON(len == 0);
    return *(u8*)body;
}

static inline void ternfs_block_policy_get(const struct ternfs_policy_body* policy, int ix, u8* storage_class, u32* min_size) {
    BUG_ON(ix < 0 || ix >= ternfs_block_policy_len(policy->body, policy->len));
    const char* b = policy->body + 2 + ix*TERNFS_BLOCK_POLICY_ENTRY_SIZE;
    *storage_class = *(u8*)b; b += 1;
    *min_size = get_unaligned_le32(b); b += 4;
}

static inline u8 ternfs_span_policy_len(const struct ternfs_policy_body* policy) {
    BUG_ON(policy->len == 0);
    return *(u8*)policy->body;
}

static inline void ternfs_span_policy_get(const struct ternfs_policy_body* policy, int ix, u32* max_size, u8* parity) {
    BUG_ON(ix < 0 || ix >= ternfs_span_policy_len(policy));
    const char* b = policy->body + 2 + ix*TERNFS_BLOCK_POLICY_ENTRY_SIZE;
    *max_size = get_unaligned_le32(b); b += 4;
    *parity = *(u8*)b; b += 1;
}

static inline void ternfs_span_policy_last(const struct ternfs_policy_body* policy, u32* max_size, u8* parity) {
    ternfs_span_policy_get(policy, ternfs_span_policy_len(policy)-1, max_size, parity);
}

static inline u32 ternfs_stripe_policy(const struct ternfs_policy_body* policy) {
    BUG_ON(policy->len != 4);
    return get_unaligned_le32(policy->body);
}

struct ternfs_snapshot_policy {
    u64 delete_after_time;
    u16 delete_after_versions;
};

static inline void ternfs_snapshot_policy_get(const struct ternfs_policy_body* policy, struct ternfs_snapshot_policy* snapshot_policy) {
    BUG_ON(policy->len != 10);
    const char* b = policy->body;
    snapshot_policy->delete_after_time = get_unaligned_le64(b); b += 8;
    snapshot_policy->delete_after_versions = get_unaligned_le16(b);
}

int __init ternfs_policy_init(void);
void __cold ternfs_policy_exit(void);

#endif
