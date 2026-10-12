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

    uint16_t numReaders = 1;
    int32_t rocksdbMaxBackgroundJobs = 4;
    int32_t rocksdbMaxSubcompactions = 4;
    Duration regionStalenessThreshold = 10_mins;
    // These limits let CDC proceed without confirmation from every location.
    // Read gating is independent: a secondary can still serve an older namespace,
    // including a mkdir or cross-shard rename that has only partly replicated.
    // Set all three to 0 to disable these early-release limits.

    // Maximum duration of an individual wait. 0 disables only this timeout.
    Duration regionWaitTimeout = 15_sec;
    // Skip new waits after this long without an announcement. Requires
    // -replica-announcements; 0 disables.
    Duration regionSilenceThreshold = 5_sec;
    // Skip new waits when the last announcement showed this much apply lag.
    // Requires -replica-announcements; 0 disables.
    Duration regionLagThreshold = 15_sec;
    // Stop serving reads when our newest applied log entry is older than this. 0 disables read gating.
    Duration readStalenessThreshold = 10_sec;
    // How often the primary leader emits a heartbeat log entry while otherwise idle. 0 disables emission.
    Duration heartbeatEmissionInterval = 1_sec;
    Duration blockServiceWritableDelay = 5_mins;  // delay before new block service becomes writable
    uint64_t hddDriveThroughput = 35'000'000;      // bytes/sec per HDD drive
    uint64_t flashDriveThroughput = 350'000'000;    // bytes/sec per flash drive
    uint64_t minSpaceRequiredForWrite = uint64_t(MAXIMUM_SPAN_SIZE);  // min available bytes for a block service to be considered writable
    bool replicaAnnouncements = false;

    // implicit options
    bool isLeader() const { return !logsDBOptions.avoidBeingLeader; }
    bool isProxyLocation() const { return logsDBOptions.location != 0; }
    ShardReplicaId shrid() const { return ShardReplicaId(shardId, logsDBOptions.replicaId); }
};

void runShard(ShardOptions& options);
