// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/debugfs.h>
#include <linux/slab.h>
#include <linux/init.h>
#include <linux/vmalloc.h>

#include "debugfs.h"

#define TERNFS_ROOT_DEBUGFS_NAME "eggsfs"

#define TERNFS_SHARD_COUNTERS_SIZE (256 * TERNFS_SHARD_KIND_MAX * TERNFS_COUNTERS_NUM_COUNTERS * 8)
#define TERNFS_CDC_COUNTERS_SIZE (TERNFS_CDC_KIND_MAX * TERNFS_COUNTERS_NUM_COUNTERS * 8)

#define TERNFS_SHARD_LATENCIES_SIZE (256 * TERNFS_SHARD_KIND_MAX * TERNFS_LATENCIES_NUM_BUCKETS * 8)
#define TERNFS_CDC_LATENCIES_SIZE (TERNFS_CDC_KIND_MAX * TERNFS_LATENCIES_NUM_BUCKETS * 8)

static struct dentry* ternfs_debugfs_root;

static struct debugfs_blob_wrapper* debug_wrapper_shard_stats_header = NULL;
static struct debugfs_blob_wrapper* debug_wrapper_cdc_stats_header = NULL;

static struct debugfs_blob_wrapper* debug_wrapper_shard_counters = NULL;
static struct debugfs_blob_wrapper* debug_wrapper_cdc_counters = NULL;

static struct debugfs_blob_wrapper* debug_wrapper_shard_latencies = NULL;
static struct debugfs_blob_wrapper* debug_wrapper_cdc_latencies = NULL;

static struct ternfs_stats_header* shard_stats_header;
static struct ternfs_stats_header* cdc_stats_header;

u64* shard_counters;
u64* cdc_counters;

u64* shard_latencies;
u64* cdc_latencies;

int __init ternfs_debugfs_init(void) {
    int err;

    // Headers
    shard_stats_header = kzalloc(sizeof(struct ternfs_stats_header), GFP_KERNEL);
    if (IS_ERR(shard_stats_header)) {
        err = PTR_ERR(shard_stats_header);
        goto out_shard_stats_header;
    }

    cdc_stats_header = kzalloc(sizeof(struct ternfs_stats_header), GFP_KERNEL);
    if (IS_ERR(cdc_stats_header)) {
        err = PTR_ERR(cdc_stats_header);
        goto out_cdc_stats_header;
    }

    int j;
    shard_stats_header->version = (u8)TERNFS_COUNTERS_DATA_VERSION;
    static_assert(sizeof(__ternfs_shard_kind_index_mappings) == sizeof(shard_stats_header->kind_index_mapping));
    memcpy(shard_stats_header->kind_index_mapping, __ternfs_shard_kind_index_mappings, sizeof(__ternfs_shard_kind_index_mappings));
    shard_stats_header->n_buckets = (u8)TERNFS_LATENCIES_NUM_BUCKETS;
    for (j = TERNFS_LATENCIES_NUM_BUCKETS - 1; j >= 0; j--) {
        shard_stats_header->upper_bound_values[j] = 1ull << (TERNFS_LATENCIES_NUM_BUCKETS - 1 - j);
    }

    cdc_stats_header->version = (u8)TERNFS_COUNTERS_DATA_VERSION;
    static_assert(sizeof(__ternfs_cdc_kind_index_mappings) == sizeof(cdc_stats_header->kind_index_mapping));
    memcpy(cdc_stats_header->kind_index_mapping, __ternfs_cdc_kind_index_mappings, sizeof(__ternfs_cdc_kind_index_mappings));
    cdc_stats_header->n_buckets = (u8)TERNFS_LATENCIES_NUM_BUCKETS;
    for (j = TERNFS_LATENCIES_NUM_BUCKETS - 1; j >= 0; j--) {
        cdc_stats_header->upper_bound_values[j] = 1ull << (TERNFS_LATENCIES_NUM_BUCKETS - 1 - j);
    }

    // Metrics

    shard_counters = (u64 *)vzalloc(TERNFS_SHARD_COUNTERS_SIZE);
    if (IS_ERR(shard_counters)) {
        err = PTR_ERR(shard_counters);
        goto out_shard_counters;
    }

    cdc_counters = (u64 *)vzalloc(TERNFS_CDC_COUNTERS_SIZE);
    if (IS_ERR(cdc_counters)) {
        err = PTR_ERR(cdc_counters);
        goto out_cdc_counters;
    }

    // Timings
    shard_latencies = vzalloc(TERNFS_SHARD_LATENCIES_SIZE);
    if (IS_ERR(shard_latencies)) {
        err = PTR_ERR(shard_latencies);
        goto out_shard_latencies;
    }

    cdc_latencies = vzalloc(TERNFS_CDC_LATENCIES_SIZE);
    if (IS_ERR(cdc_latencies)) {
        err = PTR_ERR(cdc_latencies);
        goto out_cdc_latencies;
    }

    ternfs_debugfs_root = debugfs_create_dir(TERNFS_ROOT_DEBUGFS_NAME, NULL);
    if (IS_ERR(ternfs_debugfs_root)) {
        err = PTR_ERR(ternfs_debugfs_root);
        goto out_err; // debugfs_remove_recursive handles NULL parameter value
    }

    struct dentry *d;
#define TERNFS_DEBUGFS_FILE(_name, _size, _stat_name) ({ \
        debug_wrapper_##_name = kzalloc(sizeof(struct debugfs_blob_wrapper), GFP_KERNEL); \
        if (IS_ERR(debug_wrapper_##_name)) { \
            return PTR_ERR(debug_wrapper_##_name); \
        } \
        debug_wrapper_##_name->data = _name; \
        debug_wrapper_##_name->size = _size; \
        d = debugfs_create_blob(_stat_name, 0644, ternfs_debugfs_root, debug_wrapper_##_name); \
        if (IS_ERR(d)){ \
            err = PTR_ERR(d); \
            goto out_err; \
        } \
    })

    TERNFS_DEBUGFS_FILE(shard_stats_header, sizeof(struct ternfs_stats_header), "shard_stats_header");
    TERNFS_DEBUGFS_FILE(cdc_stats_header, sizeof(struct ternfs_stats_header), "cdc_stats_header");
    TERNFS_DEBUGFS_FILE(shard_counters, TERNFS_SHARD_COUNTERS_SIZE, "shard_counters");
    TERNFS_DEBUGFS_FILE(cdc_counters, TERNFS_CDC_COUNTERS_SIZE, "cdc_counters");
    TERNFS_DEBUGFS_FILE(shard_latencies, TERNFS_SHARD_LATENCIES_SIZE, "shard_latencies");
    TERNFS_DEBUGFS_FILE(cdc_latencies, TERNFS_CDC_LATENCIES_SIZE, "cdc_latencies");

    return 0;

out_cdc_latencies:
    vfree(shard_latencies);
out_shard_latencies:
    vfree(cdc_counters);
out_cdc_counters:
    vfree(shard_counters);
out_shard_counters:
    kfree(cdc_stats_header);
out_cdc_stats_header:
    kfree(shard_stats_header);
out_shard_stats_header:
    return err;

out_err:
    ternfs_debugfs_exit();
    return err;

#undef TERNFS_DEBUGFS_FILE
}

void __cold ternfs_debugfs_exit(void) {
    debugfs_remove_recursive(ternfs_debugfs_root);

    kfree(shard_stats_header);
    kfree(cdc_stats_header);

    vfree(shard_counters);
    vfree(cdc_counters);

    vfree(shard_latencies);
    vfree(cdc_latencies);

    kfree(debug_wrapper_shard_counters);
    kfree(debug_wrapper_shard_stats_header);
    kfree(debug_wrapper_cdc_counters);
    kfree(debug_wrapper_cdc_stats_header);
}
