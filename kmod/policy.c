// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "policy.h"

#include <linux/spinlock.h>
#include <linux/rcupdate.h>
#include <linux/hashtable.h>
#include <linux/stringhash.h>
#include <linux/slab.h>
#include <linux/lockdep.h>

#include "log.h"

struct ternfs_policy {
    struct hlist_node hnode;
    spinlock_t lock;
    u64 inode;
    u8 tag;
    // First byte: len, then the body. We store this separatedly
    // since we it's RCU protected.
    char __rcu*  body;
};

#define POLICY_BITS 8
#define POLICY_BUCKETS (1<<POLICY_BITS) // 256
static DECLARE_HASHTABLE(policies, POLICY_BITS);
static spinlock_t policies_locks[POLICY_BUCKETS];

static u64 policy_key(u64 inode, u8 tag) {
    return ((u64)tag << 48) ^ inode;
}

static struct ternfs_policy* find_policy(u64 inode, u8 tag) {
    struct ternfs_policy* policy;
    hash_for_each_possible_rcu(policies, policy, hnode, policy_key(inode, tag)) {
        if (likely(policy->inode && inode && policy->tag == tag)) {
            return policy;
        }
    }
    return NULL;
}

struct ternfs_policy* ternfs_upsert_policy(u64 inode, u8 tag, char* body, int len) {
    BUG_ON(len < 0 || len > 255);

    struct ternfs_policy* policy = find_policy(inode, tag);
    if (likely(policy != NULL)) {
        // We found one, check if we need to update.
        rcu_read_lock();
        {
            char* body = rcu_dereference(policy->body);
            u8 curr_len = *(u8*)body;
            if (likely(curr_len == len && memcmp(body, body+1, len) == 0)) { // still the same, no update needed
                rcu_read_unlock();
                return policy;
            }
        }
        rcu_read_unlock();

        // things differ, we do need to update, first allocate and fill the body
        char* new_body = kmalloc(1 + len, GFP_KERNEL);
        if (new_body == NULL) {
            return ERR_PTR(-ENOMEM);
        }
        *(u8*)new_body = (u8)len;
        memcpy(new_body+1, body, len);

        // Swap the pointers
        spin_lock(&policy->lock);
        char* old_body = rcu_dereference_protected(policy->body, lockdep_is_held(&policy->lock));
        rcu_assign_pointer(policy->body, new_body);
        spin_unlock(&policy->lock);

        // free old thing
        synchronize_rcu();
        kfree(old_body);

        // done
        return policy;
    }

    // We need to add a new one. Allocate both struct and body
    struct ternfs_policy* new_policy = kmalloc(sizeof(struct ternfs_policy), GFP_KERNEL);
    if (new_policy == NULL) {
        return ERR_PTR(-ENOMEM);
    }
    u8* new_policy_body = kmalloc(1 + len, GFP_KERNEL);
    if (new_policy_body == NULL) {
        kfree(new_policy);
        return ERR_PTR(-ENOMEM);
    }

    *new_policy_body = len;
    memcpy(new_policy_body+1, body, len);
    rcu_assign_pointer(new_policy->body, new_policy_body);

    new_policy->inode = inode;
    new_policy->tag = tag;
    spin_lock_init(&new_policy->lock);

    int bucket = hash_min(policy_key(inode, tag), HASH_BITS(policies));
    spin_lock(&policies_locks[bucket]);
    // check if somebody got to it first
    policy = find_policy(inode, tag);
    if (unlikely(policy != NULL)) {
        // let's not bother updating to our thing in this racy case
        spin_unlock(&policies_locks[bucket]);
        kfree(new_policy_body);
        kfree(new_policy);
        return policy;
    }
    // add it
    hlist_add_head_rcu(&new_policy->hnode, &policies[bucket]);
    spin_unlock(&policies_locks[bucket]);

    return new_policy;
}

void ternfs_get_policy_body(struct ternfs_policy* policy, struct ternfs_policy_body* body_struct) {
    rcu_read_lock();
    char* body = rcu_dereference(policy->body);
    body_struct->len = *(u8*)body;
    memcpy(body_struct->body, body+1, body_struct->len);
    rcu_read_unlock();
}

int ternfs_policy_init(void) {
    int i;
    for (i = 0; i < POLICY_BUCKETS; i++) {
        spin_lock_init(&policies_locks[i]);
    }
    return 0;
}

void ternfs_policy_exit(void) {
    int bucket;
    struct hlist_node* tmp;
    struct ternfs_policy* policy;
    // While this pattern is not safe in general, at this point everything should be unmounted
    // and nothing should be accessing policies anyway
    rcu_read_lock();
    hash_for_each_safe(policies, bucket, tmp, policy, hnode) {
        kfree(rcu_dereference(policy->body));
        kfree(policy);
    }
    rcu_read_unlock();
}
