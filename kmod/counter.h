// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_COUNTER_H
#define _TERNFS_COUNTER_H

#include <linux/compiler.h>
#include <linux/percpu.h>

#define TERNFS_DECLARE_COUNTER(_name) DECLARE_PER_CPU(u64, _name)
#define TERNFS_DEFINE_COUNTER(_name) DEFINE_PER_CPU(u64, _name)

#define ternfs_counter_inc(_counter) raw_cpu_inc(_counter)
#define ternfs_counter_dec(_counter) raw_cpu_dec(_counter)

#define ternfs_counter_get(_counter) ({ \
        u64 total = 0; \
        int cpu; \
        for_each_possible_cpu(cpu) { \
            total += *per_cpu_ptr(_counter, cpu); \
        } \
        total; \
    })

#endif

