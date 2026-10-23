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

#include "Env.hpp"
#include "LogsDB.hpp"
#include "MsgsGen.hpp"
#include "Time.hpp"

// Keeps registry records and peer announcements separately. Published snapshots
// use fresh announcements in preference to registry records and contain at most
// one leader per remote location.
class ReplicaTracker {
public:
    // How long a peer's announcements stay authoritative after we last heard one.
    static constexpr Duration PEER_EXPIRY = 1_mins;

    // One source's description of a peer.
    struct View {
        AddrsInfo addrs;
        bool isLeader = false;
        // Registry registration time, or local receipt time for an announcement.
        TernTime lastSeen;
    };

    struct PeerState {
        std::optional<View> registry;   // once a registry poll has described the peer
        std::optional<View> announced;  // once the peer has announced itself
        // Progress from `announced`, valid only for the address it advertised.
        // Registry registrations do not establish whether announcements stopped.
        LogIdx lastApplied;
        TernTime lastAppliedTime;

        bool hasFreshAnnouncement(TernTime now) const;
        View current(TernTime now) const;
    };

    // Last announced progress of a published remote leader, retained during silence.
    struct RemoteLeaderProgress {
        uint8_t locationId;
        TernTime lastHeard; // local receipt time
        LogIdx lastApplied;
        TernTime lastAppliedTime;
    };
    using RemoteProgress = std::vector<RemoteLeaderProgress>;

    using LocalReplicas = std::array<AddrsInfo, LogsDB::REPLICA_COUNT>;
    using RemoteLeaders = std::vector<FullShardInfo>;
    using AnnouncementTargets = std::vector<AddrsInfo>;

    ReplicaTracker(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardReplicaId self, uint8_t location, bool isLeader, const AddrsInfo& addrs);

    void seedRegistry(const std::vector<FullShardInfo>& replicas, TernTime now);
    void recordAnnouncement(const IpPort& source, const ShardReplicaAnnouncement& announcement, TernTime now);
    void expire(TernTime now);

    std::shared_ptr<const LocalReplicas> localReplicas() const;
    std::shared_ptr<const RemoteLeaders> remoteLeaders() const;
    std::shared_ptr<const RemoteProgress> remoteProgress() const;
    std::shared_ptr<const AnnouncementTargets> announcementTargets() const;
    std::optional<PeerState> peerState(ShardReplicaId id, uint8_t location) const;

private:
    Env _env;
    const ShardReplicaId _self;
    const uint8_t _location;
    const bool _isLeader;
    mutable std::mutex _mutex;
    std::unordered_map<ShardReplicaLocationKey, PeerState> _peers;
    std::atomic<std::shared_ptr<const LocalReplicas>> _localReplicas;
    std::atomic<std::shared_ptr<const RemoteLeaders>> _remoteLeaders;
    std::atomic<std::shared_ptr<const RemoteProgress>> _remoteProgress;
    std::atomic<std::shared_ptr<const AnnouncementTargets>> _announcementTargets;
    TernTime _publishedAt; // the `now` the snapshots were last built for

    static bool _validReplicaId(ShardReplicaId id);
    // Whether the snapshots would differ for a peer at `location` described by
    // `before` rather than `after`.
    bool _publishedDiffers(uint8_t location, const View& before, const View& after) const;
    void _publishLocked(TernTime now);
};
