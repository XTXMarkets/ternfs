// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <vector>

#include "DataPartitions.hpp"
#include "LogMetadata.hpp"
#include "ReqResp.hpp"

class CatchupReader {
public:
    CatchupReader(
        LogsDBStats& stats,
        ReqResp& reqResp,
        LogMetadata& metadata,
        DataPartitions& data,
        ReplicaId replicaId,
        LogIdx lastRead);

    LogIdx getLastContinuous() const;
    void readEntries(std::vector<LogsDBLogEntry>& entries, size_t maxEntries);

    void init();
    void maybeCatchUp();

    void proccessLogReadRequest(
        ReplicaId fromReplicaId,
        uint64_t requestId,
        const LogReadReq& request);
    void proccessLogReadResponse(
        ReplicaId fromReplicaId,
        LogsDBRequest& request,
        const LogReadResp& response);

    LogIdx lastRead() const;

private:
    void _populateStats();
    void _findMissingEntries();

    LogsDBStats& _stats;
    ReqResp& _reqResp;
    LogMetadata& _metadata;
    DataPartitions& _data;

    const ReplicaId _replicaId;
    LogIdx _lastRead;
    LogIdx _lastContinuousIdx;
    LogIdx _lastMissingIdx;

    std::vector<LogIdx> _missingEntries;
    std::vector<ReqResp::QuorumTrackArray> _requestIds;
};
