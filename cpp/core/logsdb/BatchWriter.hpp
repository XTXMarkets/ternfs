// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <cstdint>
#include <vector>

#include "../LogsDB.hpp"
#include "LeaderElection.hpp"
#include "ReqResp.hpp"

class BatchWriter {
public:
    BatchWriter(
        Env& env,
        ReqResp& reqResp,
        LeaderElection& leaderElection);

    void proccessLogWriteRequest(LogsDBRequest& request);
    void proccessReleaseRequest(
        ReplicaId fromReplicaId,
        uint64_t requestId,
        const ReleaseReq& request);
    void writeBatch();

private:
    Env& _env;
    ReqResp& _reqResp;
    LeaderElection& _leaderElection;

    LeaderToken _token;
    LogIdx _lastReleased;
    std::vector<LogsDBRequest*> _requests;
    std::vector<LogsDBLogEntry> _entries;
};
