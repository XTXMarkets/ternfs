// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <cstdint>
#include <ostream>
#include <vector>

#include "../Protocol.hpp"
#include "../Time.hpp"

// Core LogsDB types that need to be fully defined for helper classes

struct LogsDBLogEntry {
    LogIdx idx;
    std::vector<uint8_t> value;
    bool operator==(const LogsDBLogEntry& oth) const {
        return idx == oth.idx && value == oth.value;
    }
};

std::ostream& operator<<(std::ostream& out, const LogsDBLogEntry& entry);

struct LogsDBRequest {
    ReplicaId replicaId;
    TernTime sentTime;
    LogReqMsg msg;
};

std::ostream& operator<<(std::ostream& out, const LogsDBRequest& entry);

struct LogsDBResponse {
    ReplicaId replicaId;
    LogRespMsg msg;
};

std::ostream& operator<<(std::ostream& out, const LogsDBResponse& entry);
