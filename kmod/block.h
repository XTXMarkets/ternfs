// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_BLOCK_H
#define _TERNFS_BLOCK_H

#include <linux/kernel.h>
#include <linux/completion.h>
#include <linux/net.h>
#include <net/tcp.h>

#include "bincode.h"

#define TERNFS_MAX_BLOCK_SIZE (100 << 20) // 100MiB

extern int ternfs_fetch_block_timeout_jiffies;
extern int ternfs_write_block_timeout_jiffies;
extern int ternfs_block_service_connect_timeout_jiffies;

struct ternfs_block_service {
    u64 id;
    u32 ip1;
    u32 ip2;
    u16 port1;
    u16 port2;
    u8 flags;
};

// Returns an error immediately if it can't connect to the block service or anyway
// if it thinks the block service is no good.
//
// It's _very_ important that you remember to get a reference to whatever
// the callback needs! E.g. ihold on the inode.
// The crc of the fetched page is stored in the page->private field. This is ok
// because the field only has meaning when PagePrivate bit is set.
// `pages` is the list of pages to be filled when fetching blocks. Pages from
// the supplied list are transferred to block request and then returned back
// via the callback in `pages `parameter in the callback itself.
int ternfs_fetch_block_pages_with_crc(
    void (*callback)(void* data, u64 block_id, struct list_head* pages, int err),
    void* data,
    struct ternfs_block_service* bs,
    struct list_head* pages,
    u64 file_id,
    u64 block_id,
    u32 block_crc,
    u32 offset,
    u32 count
);

// Returns how many sockets were dropped. Note that the sockets won't be
// dropped immediately, they will just be scheduled for deletion (but dropping
// them _will_ fail the requests within them).
int ternfs_drop_fetch_block_sockets(void);

// Returns an error immediately if it can't connect to the block service or anyway
// if it thinks the block service is no good.
int ternfs_write_block(
    // This callback _must_ take ownership of the pages (and free them if necessary)
    void (*callback)(void* data, struct list_head* pages, u64 block_id, u64 proof, int err),
    void* data,
    struct ternfs_block_service* bs,
    u64 block_id,
    u64 certificate,
    u32 size,
    u32 crc,
    // There must be enough pages to write everything.
    // After this call, ownership of these pages is passed onto the block
    // writing until control is returned to the callback.
    struct list_head* pages
);

int ternfs_drop_write_block_sockets(void);

int __init ternfs_block_init(void);
void __cold ternfs_block_exit(void);

#endif
