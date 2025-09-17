// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

// This cache is never cleared -- but it is small. For 100k disks, which is
// what we're targeting, it'd be ~5MB.
#ifndef _TERNFS_BLOCK_SERVICE_H
#define _TERNFS_BLOCK_SERVICE_H

#include "block.h"

struct ternfs_stored_block_service;

// Creates or updates a specific block service. Very fast unless the block service is
// unseen so far, which should be rare.
struct ternfs_stored_block_service* ternfs_upsert_block_service(struct ternfs_block_service* bs);

// Gets block service.
void ternfs_get_block_service(struct ternfs_stored_block_service* bs_node, struct ternfs_block_service* bs);

int __init ternfs_block_service_init(void);
void __cold ternfs_block_service_exit(void);

#endif
