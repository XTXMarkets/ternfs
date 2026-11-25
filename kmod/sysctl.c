// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/sysctl.h>

#include "dir.h"
#include "span.h"
#include "log.h"
#include "super.h"
#include "sysctl.h"
#include "net.h"
#include "file.h"
#include "block.h"
#include "metadata.h"

int ternfs_debug_output = 0;

#define ternfs_do_sysctl(__callback) \
    int ret; \
    ret = proc_dointvec_minmax(table, write, buffer, len, ppos); \
    if (ret) { \
        return ret; \
    } \
    if (write) { \
        __callback(); \
    } \
    return 0;

static int drop_fetch_block_sockets;
static int ternfs_drop_fetch_block_sockets_sysctl(
    const struct ctl_table* table,
    int write, void __kernel* buffer, size_t* len, loff_t* ppos)
{
    ternfs_do_sysctl(ternfs_drop_fetch_block_sockets);
}

static int drop_write_block_sockets;
static int ternfs_drop_write_block_sockets_sysctl(
    const struct ctl_table* table,
    int write, void __kernel* buffer, size_t* len, loff_t* ppos)
{
    ternfs_do_sysctl(ternfs_drop_write_block_sockets);
}

#define TERNFS_CTL_ULONG(_name) \
    { \
        .procname = #_name, \
        .data = &ternfs_##_name, \
        .maxlen = sizeof(ternfs_##_name), \
        .mode = 0644, \
        .proc_handler = proc_doulongvec_minmax,  \
    }

#define TERNFS_CTL_UINT(_name) \
    { \
        .procname = #_name, \
        .data = &ternfs_##_name, \
        .maxlen = sizeof(ternfs_##_name), \
        .mode = 0644, \
        .proc_handler = proc_douintvec,  \
    }

static int bool_off = 0;
static int bool_on = 1;

#define TERNFS_CTL_BOOL(_name) \
    { \
        .procname = #_name, \
        .data = &ternfs_##_name, \
        .maxlen = sizeof(ternfs_##_name), \
        .mode = 0644, \
        .proc_handler = proc_dointvec_minmax,  \
        .extra1 = &bool_off,  \
        .extra2 = &bool_on,  \
    }

#define TERNFS_CTL_INT_JIFFIES(_name) \
    { \
        .procname = #_name "_ms", \
        .data = &ternfs_##_name##_jiffies, \
        .maxlen = sizeof(ternfs_##_name##_jiffies), \
        .mode = 0644, \
        .proc_handler = proc_dointvec_ms_jiffies,  \
    }

static struct ctl_table ternfs_cb_sysctls[] = {
    {
        .procname = "debug",
        .data = &ternfs_debug_output,
        .maxlen = sizeof(ternfs_debug_output),
        .mode = 0644,
        .proc_handler = proc_dointvec_minmax,
        .extra1 = &bool_off,
        .extra2 = &bool_on,
    },

    {
        .procname = "rs_cpu_level",
        .data = &ternfs_rs_cpu_level,
        .maxlen = sizeof(ternfs_rs_cpu_level),
        .mode = 0644,
        .proc_handler = proc_dointvec_minmax,
        .extra1 = &ternfs_rs_cpu_level_min,
        .extra2 = &ternfs_rs_cpu_level_max,
    },

    {
        .procname = "drop_fetch_block_sockets",
        .data = &drop_fetch_block_sockets,
        .maxlen = sizeof(int),
        .mode = 0200,
        .proc_handler = ternfs_drop_fetch_block_sockets_sysctl,
    },

    {
        .procname = "drop_write_block_sockets",
        .data = &drop_write_block_sockets,
        .maxlen = sizeof(int),
        .mode = 0200,
        .proc_handler = ternfs_drop_write_block_sockets_sysctl,
    },

    TERNFS_CTL_INT_JIFFIES(dir_getattr_refresh_time),
    TERNFS_CTL_INT_JIFFIES(dir_dentry_refresh_time),
    TERNFS_CTL_INT_JIFFIES(file_getattr_refresh_time),
    TERNFS_CTL_INT_JIFFIES(registry_refresh_time),
    TERNFS_CTL_INT_JIFFIES(initial_shard_timeout),
    TERNFS_CTL_INT_JIFFIES(max_shard_timeout),
    TERNFS_CTL_INT_JIFFIES(overall_shard_timeout),
    TERNFS_CTL_INT_JIFFIES(initial_cdc_timeout),
    TERNFS_CTL_INT_JIFFIES(max_cdc_timeout),
    TERNFS_CTL_INT_JIFFIES(overall_cdc_timeout),
    TERNFS_CTL_INT_JIFFIES(fetch_block_timeout),
    TERNFS_CTL_INT_JIFFIES(write_block_timeout),
    TERNFS_CTL_INT_JIFFIES(block_service_connect_timeout),
    TERNFS_CTL_INT_JIFFIES(span_cache_retention),

    TERNFS_CTL_UINT(readahead_pages),
    TERNFS_CTL_UINT(max_write_span_attempts),
    TERNFS_CTL_UINT(atime_update_interval_sec),
    TERNFS_CTL_BOOL(atime_on_open),
    TERNFS_CTL_UINT(file_io_timeout_sec),
    TERNFS_CTL_UINT(file_io_retry_refresh_span_interval_sec),
    TERNFS_CTL_UINT(disable_ftruncate),
    TERNFS_CTL_INT_JIFFIES(rename_idempotency_window),

    {
        .procname = "mtu",
        .data = &ternfs_mtu,
        .maxlen = sizeof(ternfs_mtu),
        .mode = 0644,
        .proc_handler = proc_douintvec_minmax,
        .extra1 = &ternfs_default_mtu,
        .extra2 = &ternfs_max_mtu,
    },

};

static struct ctl_table_header* ternfs_callback_sysctl_table;

int __init ternfs_sysctl_init(void) {
    ternfs_callback_sysctl_table = register_sysctl("fs/eggsfs", ternfs_cb_sysctls);
    if (ternfs_callback_sysctl_table == NULL) {
        return -ENOMEM;
    }
    return 0;
}

void __cold ternfs_sysctl_exit(void) {
    ternfs_debug("sysctl exit");

    unregister_sysctl_table(ternfs_callback_sysctl_table);
    ternfs_callback_sysctl_table = NULL;
}

