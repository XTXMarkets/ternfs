// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <cstdint>

#include <rocksdb/slice.h>

enum class LogsDBMetadataKey : uint8_t {
    PARTITION_0_FIRST_WRITE_TIME = 0,
    PARTITION_1_FIRST_WRITE_TIME = 1,
    LEADER_TOKEN = 2,
    LAST_RELEASED_IDX = 3,
    LAST_RELEASED_TIME = 4,
};

constexpr LogsDBMetadataKey PARTITION_0_FIRST_WRITE_TIME_KEY = LogsDBMetadataKey::PARTITION_0_FIRST_WRITE_TIME;
constexpr LogsDBMetadataKey PARTITION_1_FIRST_WRITE_TIME_KEY = LogsDBMetadataKey::PARTITION_1_FIRST_WRITE_TIME;
constexpr LogsDBMetadataKey LEADER_TOKEN_KEY = LogsDBMetadataKey::LEADER_TOKEN;
constexpr LogsDBMetadataKey LAST_RELEASED_IDX_KEY = LogsDBMetadataKey::LAST_RELEASED_IDX;
constexpr LogsDBMetadataKey LAST_RELEASED_TIME_KEY = LogsDBMetadataKey::LAST_RELEASED_TIME;

inline rocksdb::Slice logsDBMetadataKey(const LogsDBMetadataKey& k) {
    return rocksdb::Slice((const char*)&k, sizeof(LogsDBMetadataKey));
}
