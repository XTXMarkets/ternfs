// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <atomic>
#include <rocksdb/db.h>
#include <rocksdb/slice.h>
#include <string>

#include "Time.hpp"

// Helper functions for LogsDB

static inline bool tryGet(rocksdb::DB* db, rocksdb::ColumnFamilyHandle* cf, const rocksdb::Slice& key, std::string& value) {
    auto status = db->Get({}, cf, key, &value);
    if (status.IsNotFound()) {
        return false;
    }
    ROCKS_DB_CHECKED(status);
    return true;
}

static inline void update_atomic_stat_ema(std::atomic<double>& stat, double newValue) {
    stat.store((stat.load(std::memory_order_relaxed)* 0.95 + newValue * 0.05), std::memory_order_relaxed);
}

static inline void update_atomic_stat_ema(std::atomic<Duration>& stat, Duration newValue) {
    stat.store((Duration)((double)stat.load(std::memory_order_relaxed).ns * 0.95 + (double)newValue.ns * 0.05), std::memory_order_relaxed);
}

// Column family names
static constexpr auto METADATA_CF_NAME = "logMetadata";
static constexpr auto DATA_PARTITION_0_NAME = "logTimePartition0";
static constexpr auto DATA_PARTITION_1_NAME = "logTimePartition1";
