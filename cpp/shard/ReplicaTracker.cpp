// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "ReplicaTracker.hpp"

#include <algorithm>

#include "Assert.hpp"

static bool hasAddress(const AddrsInfo& addrs) {
    return addrs[0].port != 0 || addrs[1].port != 0;
}

bool ReplicaTracker::PeerState::hasFreshAnnouncement(TernTime now) const {
    return announced.has_value() && now < announced->lastSeen + PEER_EXPIRY;
}

ReplicaTracker::View ReplicaTracker::PeerState::current(TernTime now) const {
    if (hasFreshAnnouncement(now) || !registry.has_value()) {
        ALWAYS_ASSERT(announced.has_value());
        return *announced;
    }
    View view = *registry;
    // Wiped registry ports must not discard the only address we know. Keep
    // the registry's leadership and timestamp when falling back to it.
    if (!hasAddress(view.addrs) && announced.has_value()) {
        view.addrs = announced->addrs;
    }
    return view;
}

ReplicaTracker::ReplicaTracker(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardReplicaId self, uint8_t location, bool isLeader, const AddrsInfo& addrs)
    : _env(logger, xmon, "replica_tracker"),
      _self(self),
      _location(location),
      _isLeader(isLeader),
      _localReplicas(std::make_shared<const LocalReplicas>()),
      _remoteLeaders(std::make_shared<const RemoteLeaders>()),
      _remoteProgress(std::make_shared<const RemoteProgress>()),
      _announcementTargets(std::make_shared<const AnnouncementTargets>())
{
    _peers.emplace(
        ShardReplicaLocationKey(_self, _location),
        PeerState{
            .registry = std::nullopt,
            .announced = View{.addrs = addrs, .isLeader = isLeader, .lastSeen = {}},
            .lastApplied = {},
            .lastAppliedTime = {},
        });
    _publishLocked(TernTime{});
}

bool ReplicaTracker::_validReplicaId(ShardReplicaId id) {
    return (id.u16 >> 8) < LogsDB::REPLICA_COUNT;
}

bool ReplicaTracker::_publishedDiffers(uint8_t location, const View& before, const View& after) const {
    if (before.addrs != after.addrs || before.isLeader != after.isLeader) {
        return true;
    }
    // Remote leader timestamps affect leader selection, liveness and progress.
    const bool inRemoteLeaders = location != _location && before.isLeader;
    return inRemoteLeaders && before.lastSeen != after.lastSeen;
}

void ReplicaTracker::seedRegistry(const std::vector<FullShardInfo>& replicas, TernTime now) {
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
                .registry = View{.addrs = replica.addrs, .isLeader = replica.isLeader, .lastSeen = replica.lastSeen},
                .announced = std::nullopt,
                .lastApplied = {},
                .lastAppliedTime = {},
            });
        if (inserted) {
            changed = true;
            continue;
        }

        // The registry's view is kept current whichever view we publish, so
        // that a peer whose announcements go stale is published at whatever
        // address the registry has learnt in the meantime.
        auto& peer = it->second;
        const View before = peer.current(_publishedAt);
        if (!peer.registry.has_value()) {
            peer.registry = View{.addrs = replica.addrs, .isLeader = replica.isLeader, .lastSeen = replica.lastSeen};
            changed = true;
        } else {
            if (hasAddress(replica.addrs)) {
                // The registry wipes dynamic ports on restart; keep the last real address.
                peer.registry->addrs = replica.addrs;
            }
            peer.registry->isLeader = replica.isLeader;
            peer.registry->lastSeen = replica.lastSeen;
        }
        changed = changed || _publishedDiffers(replica.locationId, before, peer.current(now));
    }

    if (changed) {
        _publishLocked(now);
    }
}

void ReplicaTracker::recordAnnouncement(const IpPort& source, const ShardReplicaAnnouncement& announcement, TernTime now) {
    if (!_validReplicaId(announcement.shrid)) {
        LOG_DEBUG(_env, "Ignoring announcement from %s for invalid replica %s", source, announcement.shrid);
        return;
    }
    if (announcement.shrid.shardId() != _self.shardId()) {
        LOG_DEBUG(_env, "Ignoring announcement from %s for shard %s, we are shard %s", source, announcement.shrid.shardId(), _self.shardId());
        return;
    }
    if (announcement.shrid == _self && announcement.location == _location) {
        LOG_DEBUG(_env, "Ignoring announcement from %s claiming to be us", source);
        return;
    }
    if (!announcement.addrs.contains(source)) {
        LOG_DEBUG(_env, "Ignoring announcement from %s for %s at location %s, not one of its addresses %s", source, announcement.shrid, (int)announcement.location, announcement.addrs);
        return;
    }

    std::lock_guard lock(_mutex);
    bool changed = false;
    const View announced{.addrs = announcement.addrs, .isLeader = announcement.isLeader, .lastSeen = now};
    const auto key = ShardReplicaLocationKey(announcement.shrid, announcement.location);
    auto [it, inserted] = _peers.try_emplace(key, PeerState{
        .registry = std::nullopt,
        .announced = announced,
        .lastApplied = announcement.lastApplied,
        .lastAppliedTime = announcement.lastAppliedTime,
    });
    if (inserted) {
        changed = true;
    } else {
        auto& peer = it->second;
        // Expiry may have changed the selected source before the expiry pass
        // publishes it. Compare with the view that was actually published.
        const View before = peer.current(_publishedAt);
        peer.announced = announced;
        peer.lastApplied = announcement.lastApplied;
        peer.lastAppliedTime = announcement.lastAppliedTime;
        changed = changed || _publishedDiffers(announcement.location, before, peer.current(now));
    }
    if (changed || (announcement.location != _location && announcement.isLeader)) {
        _publishLocked(now);
    }
}

void ReplicaTracker::expire(TernTime now) {
    std::lock_guard lock(_mutex);
    bool changed = false;

    // A peer whose announcements have gone stale since the snapshots were
    // built, for example after a restart without -replica-announcements, is
    // now published as the registry describes it.
    for (const auto& [key, peer] : _peers) {
        if (key.shardReplicaId() == _self && key.locationId() == _location) {
            continue;
        }
        changed = changed || _publishedDiffers(key.locationId(), peer.current(_publishedAt), peer.current(now));
        if (key.locationId() != _location && peer.hasFreshAnnouncement(_publishedAt) != peer.hasFreshAnnouncement(now)) {
            changed = true;
        }
    }

    if (changed) {
        _publishLocked(now);
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

std::shared_ptr<const ReplicaTracker::RemoteProgress>
ReplicaTracker::remoteProgress() const {
    return _remoteProgress.load();
}

std::shared_ptr<const ReplicaTracker::AnnouncementTargets>
ReplicaTracker::announcementTargets() const {
    return _announcementTargets.load();
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

void ReplicaTracker::_publishLocked(TernTime now) {
    _publishedAt = now;
    auto local = std::make_shared<LocalReplicas>();
    auto remote = std::make_shared<RemoteLeaders>();
    auto progress = std::make_shared<RemoteProgress>();
    auto targets = std::make_shared<AnnouncementTargets>();

    // Prefer fresh announcements, then registry records, then announcements
    // from peers the registry has never described. Compare timestamps only
    // within a source, so registry clock skew cannot override a fresh peer.
    auto sourcePriority = [now](const PeerState& peer) {
        return peer.hasFreshAnnouncement(now) ? 2 : peer.registry.has_value() ? 1 : 0;
    };

    for (const auto& [key, peer] : _peers) {
        const auto id = key.shardReplicaId();
        const auto location = key.locationId();
        const bool self = id == _self && location == _location;
        const View view = peer.current(now);

        if (!self && hasAddress(view.addrs) && (location == _location || _isLeader)) {
            targets->push_back(view.addrs);
        }
        if (location == _location) {
            (*local)[id.replicaId().u8] = view.addrs;
        } else if (view.isLeader && view.addrs[0].port != 0) {
            // A leader without an address, which the registry reports after
            // wiping dynamic ports, cannot be waited for or replicated to.
            auto leader = std::find_if(remote->begin(), remote->end(), [location](const FullShardInfo& info) {
                return info.locationId == location;
            });
            if (leader == remote->end()) {
                remote->emplace_back(id, true, view.addrs, view.lastSeen, location);
                continue;
            }
            const auto& previous = _peers.at(ShardReplicaLocationKey(leader->id, location));
            const int priority = sourcePriority(peer);
            const int previousPriority = sourcePriority(previous);
            if (priority > previousPriority ||
                (priority == previousPriority &&
                    (view.lastSeen > leader->lastSeen || (view.lastSeen == leader->lastSeen && id.u16 < leader->id.u16)))) {
                *leader = FullShardInfo(id, true, view.addrs, view.lastSeen, location);
            }
        }
    }

    for (const auto& leader : *remote) {
        const auto& peer = _peers.at(ShardReplicaLocationKey(leader.id, leader.locationId));
        if (peer.announced.has_value() && peer.announced->addrs == leader.addrs) {
            progress->push_back(RemoteLeaderProgress{
                .locationId = leader.locationId,
                .lastHeard = peer.announced->lastSeen,
                .lastApplied = peer.lastApplied,
                .lastAppliedTime = peer.lastAppliedTime,
            });
        }
    }

    _localReplicas.store(std::move(local));
    _remoteLeaders.store(std::move(remote));
    _remoteProgress.store(std::move(progress));
    _announcementTargets.store(std::move(targets));
}
