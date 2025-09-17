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

struct ternfs_policy;

// Creates or update a specific policy. Very fast unless the policy is unseen so
// far, which is a rare occurrence.
struct ternfs_policy* ternfs_upsert_policy(u64 inode, u8 tag, char* body, int len);

struct ternfs_policy_body {
    u8 len;
    char body[255];
};

void ternfs_get_policy_body(struct ternfs_policy* policy, struct ternfs_policy_body* body);

int __init ternfs_policy_init(void);
void __cold ternfs_policy_exit(void);

#endif
