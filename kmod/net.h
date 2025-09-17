// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_NET_H
#define _TERNFS_NET_H

#include <linux/net.h>
#include <net/udp.h>
#include <net/sock.h>
#include <net/inet_common.h>

#define TERNFS_UDP_MTU 1472

extern unsigned ternfs_initial_shard_timeout_jiffies;
extern unsigned ternfs_max_shard_timeout_jiffies;
extern unsigned ternfs_overall_shard_timeout_jiffies;
extern unsigned ternfs_initial_cdc_timeout_jiffies;
extern unsigned ternfs_max_cdc_timeout_jiffies;
extern unsigned ternfs_overall_cdc_timeout_jiffies;

struct ternfs_metadata_socket {
    struct socket* sock;
    struct rb_root requests;
    spinlock_t lock;
    void (*original_data_ready)(struct sock *sk);
};

int ternfs_init_shard_socket(struct ternfs_metadata_socket* sock);
void ternfs_net_shard_free_socket(struct ternfs_metadata_socket* sock);

// TODO must remember to ihold inode while the request
// is in flight.
#define TERNFS_METADATA_REQUEST_ASYNC_GETATTR 1

struct ternfs_metadata_request {
    struct rb_node node;
    u64 request_id;
    struct sk_buff* skb;
    s16 shard; // used for logging, -1 = cdc
    u8 flags;
};

struct ternfs_metadata_sync_request {
    struct ternfs_metadata_request req;
    struct completion comp;
};

// shard is just used for debugging/event tracing, and should be -1 if we're going to the CDC
struct ternfs_metadata_request_state {
    u64 start_t; // in jiffies
    unsigned next_timeout; // in jiffies
    u32 attempts;
    u8 which_addr;
};

// After this function returns the request is in the tree.
void ternfs_metadata_request_init(
    struct ternfs_metadata_socket* sock,
    struct ternfs_metadata_request* req, // request_id/shard/flags are filled in by caller
    struct ternfs_metadata_request_state* state // everything filled in for you
);

// ternfs_metadata_init must obviously been called before this.
// If this fails, the request is not in the tree anymore (i.e we're done).
int ternfs_metadata_send_request(
    struct ternfs_metadata_socket* sock,
    const atomic64_t* addr_data1,
    const atomic64_t* addr_data2,
    struct ternfs_metadata_request* req,
    void* data,
    u32 len,
    struct ternfs_metadata_request_state* state
);

void ternfs_metadata_remove_request(struct ternfs_metadata_socket* sock, u64 request_id);

struct sk_buff* ternfs_metadata_request(
    struct ternfs_metadata_socket* sock, s16 shard_id, u64 req_id, void* p, u32 len, const atomic64_t* ip_data1, const atomic64_t* ip_data2, u32* attempts
);

int ternfs_metadata_request_nowait(struct ternfs_metadata_socket *sock, u64 req_id, void *p, u32 len, const atomic64_t* addr_data1, const atomic64_t* addr_data2);

#if 0

void ternfs_shard_async_request(struct ternfs_shard_async_request* req, struct ternfs_metadata_socket* sock, struct msghdr* msg, u64 req_id, u32 len);
void ternfs_shard_async_cleanup(struct ternfs_shard_async_request* req);

#endif

#endif
