// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <array>
#include <vector>

#include "../Env.hpp"
#include "../Protocol.hpp"
#include "LeaderElection.hpp"
#include "LogMetadata.hpp"
#include "LogsDBCommon.hpp"
#include "LogsDBTypes.hpp"
#include "ReqResp.hpp"

// Forward declarations
struct LogsDBRequest;
struct LogsDBStats;
struct LogWriteResp;

class Appender {
    static constexpr size_t IN_FLIGHT_MASK = LogsDBConsts::IN_FLIGHT_APPEND_WINDOW - 1;
    static_assert((IN_FLIGHT_MASK & LogsDBConsts::IN_FLIGHT_APPEND_WINDOW) == 0);
public:
    Appender(Env& env, LogsDBStats& stats, ReqResp& reqResp, LogMetadata& metadata, LeaderElection& leaderElection, bool noReplication);

    void maybeMoveRelease();
    TernError appendEntries(std::vector<LogsDBLogEntry>& entries);
    void proccessLogWriteResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogWriteResp& response);
    uint64_t entriesInFlight() const;

private:
    void _init();
    void _cleanup();

    Env& _env;
    ReqResp& _reqResp;
    LogMetadata& _metadata;
    LeaderElection& _leaderElection;

    const bool _noReplication;
    bool _currentIsLeader;
    uint64_t _entriesStart;
    uint64_t _entriesEnd;

    std::array<LogsDBLogEntry, LogsDBConsts::IN_FLIGHT_APPEND_WINDOW> _entries;
    std::array<ReqResp::QuorumTrackArray, LogsDBConsts::IN_FLIGHT_APPEND_WINDOW> _requestIds;
    ReqResp::QuorumTrackArray _releaseRequests;
};
