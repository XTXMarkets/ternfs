// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <array>
#include <cstdint>
#include <limits>
#include <unordered_map>
#include <vector>

#include "../Protocol.hpp"
#include "LogsDBCommon.hpp"

// Forward declarations
struct LogsDBRequest;
struct LogsDBResponse;
struct LogsDBStats;

class ReqResp {
    public:
        static constexpr size_t UNUSED_REQ_ID = std::numeric_limits<size_t>::max();
        static constexpr size_t CONFIRMED_REQ_ID = 0;

        using QuorumTrackArray = std::array<uint64_t, LogsDBConsts::REPLICA_COUNT>;

        ReqResp(LogsDBStats& stats);

        LogsDBRequest& newRequest(ReplicaId targetReplicaId);
        LogsDBRequest* getRequest(uint64_t requestId);
        void eraseRequest(uint64_t requestId);
        void cleanupRequests(QuorumTrackArray& requestIds);
        void resendTimedOutRequests();
        void getRequestsToSend(std::vector<LogsDBRequest*>& requests);
        LogsDBResponse& newResponse(ReplicaId targetReplicaId, uint64_t requestId);
        void getResponsesToSend(std::vector<LogsDBResponse>& responses);
        Duration getNextTimeout() const;

        static bool isQuorum(const QuorumTrackArray& requestIds);

private:
    LogsDBStats& _stats;
    uint64_t _lastAssignedRequest;
    std::unordered_map<uint64_t, LogsDBRequest> _requests;
    std::vector<LogsDBRequest*> _requestsToSend;

    std::vector<LogsDBResponse> _responses;
};
