// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_SUPER_H
#define _TERNFS_SUPER_H

#include <linux/inet.h>

#include "net.h"
#include "registry.h"

extern int ternfs_registry_refresh_time_jiffies;
extern unsigned int ternfs_readahead_pages;

// We store addresses as atomics so that we can
// easily refresh them.
struct ternfs_fs_info {
    struct ternfs_registry_addr registry_addr;

    struct ternfs_metadata_socket sock;

    atomic64_t shard_addrs1[256];
    atomic64_t shard_addrs2[256];
    atomic64_t cdc_addr1;
    atomic64_t cdc_addr2;

    atomic64_t capacity;
    atomic64_t available;

    u64 block_services_last_changed_time;

    struct delayed_work registry_refresh_work;

    kuid_t uid;
    kgid_t gid;
    umode_t fmask;
    umode_t dmask;
};

int __init ternfs_fs_init(void);
void __cold ternfs_fs_exit(void);

static inline u64 ternfs_mk_addr(u32 ip, u16 port) {
    return ((u64)port << 32) | (u64)ip;
}

static inline __be32 ternfs_get_addr_ip(u64 v) {
    return htonl(v&((1ull<<32)-1));
}
static inline __be16 ternfs_get_addr_port(u64 v) {
    return htons(v >> 32);
}

#endif

