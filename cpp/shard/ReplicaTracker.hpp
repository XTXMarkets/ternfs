// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <array>
#include <atomic>
#include <memory>
#include <mutex>
#include <optional>
#include <unordered_map>
#include <vector>

#include "LogsDB.hpp"
#include "MsgsGen.hpp"
#include "Time.hpp"

// Tracks the replicas of our shard, at every location, as the registry
// describes them. It publishes the addresses of the replicas at our location
// and the leaders at other locations as snapshots that the receive, writer
// and metrics threads read without locking.
class ReplicaTracker {
public:
    struct PeerState {
        AddrsInfo addrs;
        bool isLeader;
        TernTime registryLastSeen;  // when the registry last heard from the peer
    };

    using LocalReplicas = std::array<AddrsInfo, LogsDB::REPLICA_COUNT>;
    using RemoteLeaders = std::vector<FullShardInfo>;

    ReplicaTracker(ShardReplicaId self, uint8_t location, bool isLeader, const AddrsInfo& addrs);

    // Takes addresses, leadership and last-seen times from the registry.
    void seedRegistry(const std::vector<FullShardInfo>& replicas);

    std::shared_ptr<const LocalReplicas> localReplicas() const;
    std::shared_ptr<const RemoteLeaders> remoteLeaders() const;
    std::optional<PeerState> peerState(ShardReplicaId id, uint8_t location) const;

private:
    const ShardReplicaId _self;
    const uint8_t _location;
    mutable std::mutex _mutex;
    std::unordered_map<ShardReplicaLocationKey, PeerState> _peers;
    std::atomic<std::shared_ptr<const LocalReplicas>> _localReplicas;
    std::atomic<std::shared_ptr<const RemoteLeaders>> _remoteLeaders;

    static bool _validReplicaId(ShardReplicaId id);
    void _publishLocked();
};
