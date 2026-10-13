// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "ReplicaTracker.hpp"

ReplicaTracker::ReplicaTracker(ShardReplicaId self, uint8_t location, bool isLeader, const AddrsInfo& addrs)
    : _self(self),
      _location(location),
      _localReplicas(std::make_shared<const LocalReplicas>()),
      _remoteLeaders(std::make_shared<const RemoteLeaders>())
{
    _peers.emplace(
        ShardReplicaLocationKey(_self, _location),
        PeerState{
            .addrs = addrs,
            .isLeader = isLeader,
            .registryLastSeen = {},
        });
    _publishLocked();
}

bool ReplicaTracker::_validReplicaId(ShardReplicaId id) {
    return (id.u16 >> 8) < LogsDB::REPLICA_COUNT;
}

void ReplicaTracker::seedRegistry(const std::vector<FullShardInfo>& replicas) {
    std::lock_guard lock(_mutex);
    bool changed = false;

    for (const auto& replica : replicas) {
        if (!_validReplicaId(replica.id) ||
            replica.id.shardId() != _self.shardId() ||
            (replica.id == _self && replica.locationId == _location)) {
            continue;
        }

        const auto key = ShardReplicaLocationKey(replica.id, replica.locationId);
        auto [it, inserted] = _peers.try_emplace(
            key,
            PeerState{
                .addrs = replica.addrs,
                .isLeader = replica.isLeader,
                .registryLastSeen = replica.lastSeen,
            });
        if (inserted) {
            changed = true;
            continue;
        }

        it->second.registryLastSeen = replica.lastSeen;
        if (replica.addrs[0].port != 0 || replica.addrs[1].port != 0) {
            it->second.addrs = replica.addrs;
        }
        it->second.isLeader = replica.isLeader;
        changed = true;
    }

    if (changed) {
        _publishLocked();
    }
}

std::shared_ptr<const ReplicaTracker::LocalReplicas>
ReplicaTracker::localReplicas() const {
    return _localReplicas.load();
}

std::shared_ptr<const ReplicaTracker::RemoteLeaders>
ReplicaTracker::remoteLeaders() const {
    return _remoteLeaders.load();
}

std::optional<ReplicaTracker::PeerState>
ReplicaTracker::peerState(ShardReplicaId id, uint8_t location) const {
    std::lock_guard lock(_mutex);
    auto it = _peers.find(ShardReplicaLocationKey(id, location));
    if (it == _peers.end()) {
        return std::nullopt;
    }
    return it->second;
}

void ReplicaTracker::_publishLocked() {
    auto local = std::make_shared<LocalReplicas>();
    auto remote = std::make_shared<RemoteLeaders>();

    for (const auto& [key, peer] : _peers) {
        const auto id = key.shardReplicaId();
        const auto location = key.locationId();

        if (location == _location) {
            (*local)[id.replicaId().u8] = peer.addrs;
        } else if (peer.isLeader && peer.addrs[0].port != 0) {
            // A leader without an address, which the registry reports after
            // wiping dynamic ports, cannot be waited for or replicated to.
            remote->emplace_back(id, peer.isLeader, peer.addrs, peer.registryLastSeen, location);
        }
    }

    _localReplicas.store(std::move(local));
    _remoteLeaders.store(std::move(remote));
}
