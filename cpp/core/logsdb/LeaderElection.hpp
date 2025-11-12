// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <array>
#include <memory>
#include <vector>

#include "../Env.hpp"
#include "../Protocol.hpp"
#include "DataPartitions.hpp"
#include "LogMetadata.hpp"
#include "LogsDBCommon.hpp"
#include "LogsDBTypes.hpp"
#include "ReqResp.hpp"

// Forward declarations
struct LogsDBRequest;
struct LogsDBStats;
struct NewLeaderReq;
struct NewLeaderResp;
struct NewLeaderConfirmReq;
struct NewLeaderConfirmResp;
struct LogRecoveryReadReq;
struct LogRecoveryReadResp;
struct LogRecoveryWriteReq;
struct LogRecoveryWriteResp;
class LogsDB;

enum class LeadershipState : uint8_t {
    FOLLOWER,
    BECOMING_NOMINEE,
    DIGESTING_ENTRIES,
    CONFIRMING_REPLICATION,
    CONFIRMING_LEADERSHIP,
    LEADER
};

std::ostream& operator<<(std::ostream& out, LeadershipState state);

struct LeaderElectionState {
    ReqResp::QuorumTrackArray requestIds;
    LogIdx lastReleased;
    std::array<ReqResp::QuorumTrackArray, LogsDBConsts::IN_FLIGHT_APPEND_WINDOW> recoveryRequests;
    std::array<LogsDBLogEntry, LogsDBConsts::IN_FLIGHT_APPEND_WINDOW> recoveryEntries;
};

class LeaderElection {
public:
    LeaderElection(Env& env, LogsDBStats& stats, bool noReplication, bool avoidBeingLeader, ReplicaId replicaId, LogMetadata& metadata, DataPartitions& data, ReqResp& reqResp);

    bool isLeader() const;
    void maybeStartLeaderElection();
    void proccessNewLeaderResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const NewLeaderResp& response);
    void proccessNewLeaderConfirmResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const NewLeaderConfirmResp& response);
    void proccessRecoveryReadResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogRecoveryReadResp& response);
    void proccessRecoveryWriteResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogRecoveryWriteResp& response);
    void proccessNewLeaderRequest(ReplicaId fromReplicaId, uint64_t requestId, const NewLeaderReq& request);
    void proccessNewLeaderConfirmRequest(ReplicaId fromReplicaId, uint64_t requestId, const NewLeaderConfirmReq& request);
    void proccessRecoveryReadRequest(ReplicaId fromReplicaId, uint64_t requestId, const LogRecoveryReadReq& request);
    void proccessRecoveryWriteRequest(ReplicaId fromReplicaId, uint64_t requestId, const LogRecoveryWriteReq& request);
    TernError writeLogEntries(LeaderToken token, LogIdx newlastReleased, std::vector<LogsDBLogEntry>& entries);
    void resetLeaderElection();

private:
    void _tryProgressToDigest();
    void _tryProgressToReplication();
    void _tryProgressToLeaderConfirm();
    void _tryBecomeLeader();
    void _clearElectionState();
    void _clearRecoveryRequests();

    Env& _env;
    LogsDBStats& _stats;
    const bool _noReplication;
    const bool _avoidBeingLeader;
    const ReplicaId _replicaId;
    LogMetadata& _metadata;
    DataPartitions& _data;
    ReqResp& _reqResp;

    LeadershipState _state;
    std::unique_ptr<LeaderElectionState> _electionState;
    TernTime _leaderLastActive;
};
