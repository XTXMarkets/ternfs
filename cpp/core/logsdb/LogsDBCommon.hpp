// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <algorithm>
#include <atomic>
#include <cstddef>
#include <rocksdb/db.h>
#include <rocksdb/slice.h>
#include <string>

#include "../Bincode.hpp"
#include "../Time.hpp"

// LogsDB constants - moved here to avoid circular dependencies
namespace LogsDBConsts {
    static constexpr size_t REPLICA_COUNT = 5;
    static constexpr Duration PARTITION_TIME_SPAN = 12_hours;
    static constexpr Duration RESPONSE_TIMEOUT = 10_ms;
    static constexpr Duration READ_TIMEOUT = 1_sec;
    static constexpr Duration SEND_RELEASE_INTERVAL = 300_ms;
    static constexpr Duration LEADER_INACTIVE_TIMEOUT = 1_sec;
    static constexpr size_t IN_FLIGHT_APPEND_WINDOW = 1 << 8;
    static constexpr size_t CATCHUP_WINDOW = 1 << 8;
}

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
