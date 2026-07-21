// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include "../LogsDB.hpp"
#include "DataPartitions.hpp"

class LogMetadata {
public:
    LogMetadata(
        Env& env,
        LogsDBStats& stats,
        SharedRocksDB& sharedDb,
        ReplicaId replicaId,
        DataPartitions& data);

    bool isInitialStart();
    bool init(bool initialStart);

    ReplicaId getReplicaId() const;
    LogIdx assignLogIdx();

    LeaderToken getLeaderToken() const;
    TernError updateLeaderToken(LeaderToken token);

    LeaderToken getNomineeToken() const;
    void setNomineeToken(LeaderToken token);
    LeaderToken generateNomineeToken() const;

    LogIdx getLastReleased() const;
    TernTime getLastReleasedTime() const;
    void setLastReleased(LogIdx lastReleased);

    bool isPreempting(LeaderToken token) const;

private:
    Env& _env;
    LogsDBStats& _stats;
    SharedRocksDB& _sharedDb;
    rocksdb::ColumnFamilyHandle* _cf;
    const ReplicaId _replicaId;
    DataPartitions& _data;

    LogIdx _lastAssigned;
    LogIdx _lastReleased;
    TernTime _lastReleasedTime;
    LeaderToken _leaderToken;
    LeaderToken _nomineeToken;
};
