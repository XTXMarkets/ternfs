// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_DEBUGFS_H
#define _TERNFS_DEBUGFS_H

#include <linux/init.h>

#include "bincode.h"

#define TERNFS_COUNTERS_DATA_VERSION 0
#define TERNFS_LATENCIES_DATA_VERSION 0

#define TERNFS_COUNTERS_IDX_COMPLETED 0
#define TERNFS_COUNTERS_IDX_ATTEMPTED 1
#define TERNFS_COUNTERS_IDX_TIMEOUTS 2
#define TERNFS_COUNTERS_IDX_FAILURES 3
#define TERNFS_COUNTERS_IDX_NET_FAILURES 3

#define TERNFS_COUNTERS_NUM_COUNTERS 5

#define TERNFS_LATENCIES_NUM_BUCKETS 64

struct ternfs_stats_header {
    u8 version;
    u8 n_buckets;
    u8 kind_index_mapping[256];
    u64 upper_bound_values[TERNFS_LATENCIES_NUM_BUCKETS];
};

extern u64* shard_counters;
extern u64* cdc_counters;

extern u64* shard_latencies;
extern u64* cdc_latencies;

int __init ternfs_debugfs_init(void);
void __cold ternfs_debugfs_exit(void);

#endif
