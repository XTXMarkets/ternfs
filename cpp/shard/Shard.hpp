// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include "CommonOptions.hpp"
#include "ShardDB.hpp"

struct ShardOptions {
    LogOptions logOptions;
    XmonOptions xmonOptions;
    MetricsOptions metricsOptions;
    RegistryClientOptions registryClientOptions;
    LogsDBOptions logsDBOptions;
    ServerOptions serverOptions;
    
    Duration transientDeadlineInterval = DEFAULT_DEADLINE_INTERVAL;
    ShardId shardId;
    bool shardIdSet = false;

    // implicit options
    bool isLeader() const { return !logsDBOptions.avoidBeingLeader; }
    bool isProxyLocation() const { return logsDBOptions.location != 0; }
    ShardReplicaId shrid() const { return ShardReplicaId(shardId, logsDBOptions.replicaId); }
};

void runShard(ShardOptions& options);
