// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <unistd.h>

#include "ReplicaTracker.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

static Logger logger(LogLevel::LOG_ERROR, STDERR_FILENO, false, false);
static std::shared_ptr<XmonAgent> xmon;

static IpPort address(uint8_t lastOctet, uint16_t port) {
    IpPort result;
    result.ip.data = {127, 0, 0, lastOctet};
    result.port = port;
    return result;
}

static AddrsInfo addresses(uint8_t lastOctet, uint16_t port) {
    AddrsInfo result;
    result[0] = address(lastOctet, port);
    return result;
}

static ShardReplicaAnnouncement announcement(ShardReplicaId id, uint8_t location, bool isLeader, const AddrsInfo& addrs) {
    return ShardReplicaAnnouncement(id, location, isLeader, addrs, LogIdx(), TernTime());
}

TEST_CASE("registry seeds local replicas and remote leaders") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));

    std::vector<FullShardInfo> replicas{
        FullShardInfo(self, true, addresses(1, 10001), 0, 2),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(1)), false, addresses(2, 10002), 0, 2),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(0)), true, addresses(3, 10003), 0, 3),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(2)), false, addresses(4, 10004), 0, 3),
        FullShardInfo(ShardReplicaId(ShardId(8), ReplicaId(1)), true, addresses(5, 10005), 0, 2),
        // A leader at another location whose address the registry has wiped.
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(0)), true, AddrsInfo(), 0, 4),
    };
    tracker.seedRegistry(replicas, TernTime(10));

    auto local = tracker.localReplicas();
    CHECK((*local)[0] == addresses(1, 10001));
    CHECK((*local)[1] == addresses(2, 10002));
    CHECK((*local)[2][0].port == 0);

    // Only leaders with an address, as the registry path always did.
    auto remote = tracker.remoteLeaders();
    REQUIRE(remote->size() == 1);
    CHECK(remote->at(0).id == ShardReplicaId(ShardId(7), ReplicaId(0)));
    CHECK(remote->at(0).locationId == 3);
    CHECK(remote->at(0).addrs == addresses(3, 10003));

    // A leader announces to every replica it knows of with an address.
    CHECK(tracker.announcementTargets()->size() == 3);
}

TEST_CASE("followers announce to their own location only") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(1));
    ReplicaTracker tracker(logger, xmon, self, 2, false, addresses(1, 10001));

    std::vector<FullShardInfo> replicas{
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(0)), true, addresses(2, 10002), 0, 2),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(2)), false, addresses(3, 10003), 0, 2),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(0)), true, addresses(4, 10004), 0, 3),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(1)), false, addresses(5, 10005), 0, 3),
    };
    tracker.seedRegistry(replicas, TernTime(10));

    auto targets = tracker.announcementTargets();
    REQUIRE(targets->size() == 2);
    CHECK(targets->at(0) != addresses(4, 10004));
    CHECK(targets->at(1) != addresses(4, 10004));
    // Only leaders talk across locations, but the other location's leader is
    // still known for routing.
    CHECK(tracker.remoteLeaders()->size() == 1);
}

TEST_CASE("a fresh leader announcement takes precedence over stale registry polls") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(4));
    ReplicaTracker tracker(logger, xmon, self, 0, true, addresses(1, 10001));

    // The registry says replica 4 leads location 1.
    std::vector<FullShardInfo> registry{
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(4)), true, addresses(2, 10002), TernTime(5), 1),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(3)), false, addresses(3, 10003), TernTime(5), 1),
    };
    tracker.seedRegistry(registry, TernTime(10));
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).id == ShardReplicaId(ShardId(7), ReplicaId(4)));

    // Replica 3 announces that it leads now: exactly one leader for location 1.
    auto newLeader = announcement(ShardReplicaId(ShardId(7), ReplicaId(3)), 1, true, addresses(3, 10003));
    tracker.recordAnnouncement(address(3, 10003), newLeader, TernTime(20));
    auto remote = tracker.remoteLeaders();
    REQUIRE(remote->size() == 1);
    CHECK(remote->at(0).id == ShardReplicaId(ShardId(7), ReplicaId(3)));
    CHECK(tracker.peerState(ShardReplicaId(ShardId(7), ReplicaId(4)), 1)->registry->isLeader);

    tracker.seedRegistry(registry, TernTime(30));
    remote = tracker.remoteLeaders();
    REQUIRE(remote->size() == 1);
    CHECK(remote->at(0).id == newLeader.shrid);
    CHECK(tracker.remoteProgress()->size() == 1);

    tracker.expire(TernTime(20) + ReplicaTracker::PEER_EXPIRY);
    remote = tracker.remoteLeaders();
    REQUIRE(remote->size() == 1);
    CHECK(remote->at(0).id == registry[0].id);
    CHECK(tracker.remoteProgress()->empty());
}

TEST_CASE("the most recently heard leader wins without changing other peers' records") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(4));
    ReplicaTracker tracker(logger, xmon, self, 0, true, addresses(1, 10001));
    auto first = announcement(ShardReplicaId(ShardId(7), ReplicaId(4)), 1, true, addresses(2, 10002));
    auto second = announcement(ShardReplicaId(ShardId(7), ReplicaId(3)), 1, true, addresses(3, 10003));
    first.lastApplied = LogIdx(40);
    second.lastApplied = LogIdx(42);

    tracker.recordAnnouncement(address(2, 10002), first, TernTime(10));
    tracker.recordAnnouncement(address(3, 10003), second, TernTime(20));
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).id == second.shrid);
    REQUIRE(tracker.remoteProgress()->size() == 1);
    CHECK(tracker.remoteProgress()->at(0).lastApplied == LogIdx(42));
    CHECK(tracker.peerState(first.shrid, 1)->announced->isLeader);

    // A delayed packet can temporarily change the winner; the next announcement
    // corrects it. Neither packet creates a second published leader.
    tracker.recordAnnouncement(address(2, 10002), first, TernTime(30));
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).id == first.shrid);
    tracker.recordAnnouncement(address(3, 10003), second, TernTime(40));
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).id == second.shrid);

    second.isLeader = false;
    tracker.recordAnnouncement(address(3, 10003), second, TernTime(50));
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).id == first.shrid);
}

TEST_CASE("leader selection notices expiry even when a peer's registry and announcement agree") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(4));
    ReplicaTracker tracker(logger, xmon, self, 0, true, addresses(1, 10001));
    auto leader = announcement(ShardReplicaId(ShardId(7), ReplicaId(4)), 1, true, addresses(2, 10002));
    std::vector<FullShardInfo> registry{
        FullShardInfo(leader.shrid, true, leader.addrs, TernTime(10), 1),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(3)), true, addresses(3, 10003), TernTime(20), 1),
    };
    tracker.recordAnnouncement(address(2, 10002), leader, TernTime(10));
    tracker.seedRegistry(registry, TernTime(30));
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).id == leader.shrid);

    tracker.expire(TernTime(10) + ReplicaTracker::PEER_EXPIRY);
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).id == registry[1].id);
}

TEST_CASE("equal leader timestamps have a stable winner") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(4));
    ReplicaTracker tracker(logger, xmon, self, 0, true, addresses(1, 10001));
    auto first = announcement(ShardReplicaId(ShardId(7), ReplicaId(4)), 1, true, addresses(2, 10002));
    auto second = announcement(ShardReplicaId(ShardId(7), ReplicaId(3)), 1, true, addresses(3, 10003));
    tracker.recordAnnouncement(address(2, 10002), first, TernTime(10));
    tracker.recordAnnouncement(address(3, 10003), second, TernTime(10));
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).id == second.shrid);
    tracker.recordAnnouncement(address(2, 10002), first, TernTime(10));
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).id == second.shrid);
}

TEST_CASE("announcements that change nothing do not republish") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    const auto peer = ShardReplicaId(ShardId(7), ReplicaId(1));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));

    auto body = announcement(peer, 2, false, addresses(2, 10002));
    tracker.recordAnnouncement(address(2, 10002), body, TernTime(20));
    auto local = tracker.localReplicas();
    auto targets = tracker.announcementTargets();

    // Same announcement again: the peer's last-heard time moves, the
    // published snapshots are the same objects.
    tracker.recordAnnouncement(address(2, 10002), body, TernTime(21));
    CHECK(tracker.peerState(peer, 2)->announced->lastSeen == TernTime(21));
    CHECK(tracker.localReplicas() == local);
    CHECK(tracker.announcementTargets() == targets);

    // A new address republishes.
    auto moved = announcement(peer, 2, false, addresses(3, 10003));
    tracker.recordAnnouncement(address(3, 10003), moved, TernTime(22));
    CHECK(tracker.localReplicas() != local);
    CHECK((*tracker.localReplicas())[1] == moved.addrs);
}

TEST_CASE("later registry polls update peers") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    const auto peer = ShardReplicaId(ShardId(7), ReplicaId(1));
    const auto remoteLeader = ShardReplicaId(ShardId(7), ReplicaId(4));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));

    std::vector<FullShardInfo> registry{
        FullShardInfo(peer, false, addresses(2, 10002), TernTime(1'000), 2),
        FullShardInfo(remoteLeader, true, addresses(3, 10003), TernTime(1'000), 3),
    };
    tracker.seedRegistry(registry, TernTime(10));
    CHECK((*tracker.localReplicas())[1] == addresses(2, 10002));
    REQUIRE(tracker.remoteLeaders()->size() == 1);

    // The peer moved host, the remote leader was heard from again.
    registry[0].addrs = addresses(3, 10003);
    registry[1].lastSeen = TernTime(2'000);
    tracker.seedRegistry(registry, TernTime(20));
    CHECK((*tracker.localReplicas())[1] == addresses(3, 10003));
    CHECK(tracker.remoteLeaders()->at(0).lastSeen == TernTime(2'000));

    // The remote leader stepped down and the peer's address was wiped: the
    // leader list empties, the peer keeps its last address.
    registry[0].addrs = AddrsInfo();
    registry[1].isLeader = false;
    tracker.seedRegistry(registry, TernTime(30));
    CHECK((*tracker.localReplicas())[1] == addresses(3, 10003));
    CHECK(tracker.remoteLeaders()->empty());
    auto state = tracker.peerState(peer, 2);
    REQUIRE(state.has_value());
    CHECK(state->registry->lastSeen == TernTime(1'000));
}

TEST_CASE("direct announcements validate identity and source") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));
    const auto peer = ShardReplicaId(ShardId(7), ReplicaId(1));

    auto body = announcement(peer, 2, false, addresses(2, 10002));
    tracker.recordAnnouncement(address(2, 10002), body, TernTime(20));
    CHECK((*tracker.localReplicas())[1] == body.addrs);

    // Not sent from one of the announced addresses.
    auto mismatched = announcement(ShardReplicaId(ShardId(7), ReplicaId(2)), 2, false, addresses(3, 10003));
    tracker.recordAnnouncement(address(9, 10009), mismatched, TernTime(21));
    CHECK(!tracker.peerState(mismatched.shrid, mismatched.location).has_value());

    // Another shard.
    auto wrongShard = announcement(ShardReplicaId(ShardId(8), ReplicaId(1)), 2, false, addresses(4, 10004));
    tracker.recordAnnouncement(address(4, 10004), wrongShard, TernTime(22));
    CHECK(!tracker.peerState(wrongShard.shrid, wrongShard.location).has_value());

    // An invalid replica id.
    auto invalid = body;
    invalid.shrid.u16 = (uint16_t(LogsDB::REPLICA_COUNT) << 8) | 7;
    tracker.recordAnnouncement(address(2, 10002), invalid, TernTime(23));
    CHECK(!tracker.peerState(invalid.shrid, invalid.location).has_value());

    // Ourselves: our own entry is never updated from the wire.
    auto selfBody = announcement(self, 2, true, addresses(5, 10005));
    tracker.recordAnnouncement(address(5, 10005), selfBody, TernTime(24));
    CHECK(tracker.peerState(self, 2)->announced->addrs == addresses(1, 10001));
    CHECK((*tracker.localReplicas())[0] == addresses(1, 10001));
}

TEST_CASE("direct announcements replace addresses and leadership") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    const auto peer = ShardReplicaId(ShardId(7), ReplicaId(1));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));

    std::vector<FullShardInfo> seed{
        FullShardInfo(peer, false, addresses(2, 10002), 0, 2),
    };
    tracker.seedRegistry(seed, TernTime(10));

    auto first = announcement(peer, 2, false, addresses(3, 10003));
    tracker.recordAnnouncement(address(3, 10003), first, TernTime(20));
    auto state = tracker.peerState(peer, 2);
    REQUIRE(state.has_value());
    CHECK(state->current(TernTime(20)).addrs == first.addrs);
    CHECK(!state->current(TernTime(20)).isLeader);
    CHECK(state->announced->lastSeen == TernTime(20));
    CHECK(state->current(TernTime(20)).lastSeen == TernTime(20));

    auto restarted = announcement(peer, 2, true, addresses(4, 10004));
    tracker.recordAnnouncement(address(4, 10004), restarted, TernTime(30));
    state = tracker.peerState(peer, 2);
    REQUIRE(state.has_value());
    CHECK(state->current(TernTime(30)).addrs == restarted.addrs);
    CHECK(state->current(TernTime(30)).isLeader);
    CHECK(state->announced->lastSeen == TernTime(30));

    // Registry polling cannot roll a directly announced address back.
    tracker.seedRegistry(seed, TernTime(40));
    CHECK((*tracker.localReplicas())[1] == restarted.addrs);
}

TEST_CASE("quiet peers fall back to the registry") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    const auto peer = ShardReplicaId(ShardId(7), ReplicaId(1));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));

    auto body = announcement(peer, 2, false, addresses(2, 10002));
    const TernTime heardAt(1'000'000'000);
    tracker.recordAnnouncement(address(2, 10002), body, heardAt);

    tracker.expire(heardAt + ReplicaTracker::PEER_EXPIRY - 1_ns);
    auto state = tracker.peerState(peer, 2);
    REQUIRE(state.has_value());
    CHECK(state->current(heardAt + ReplicaTracker::PEER_EXPIRY - 1_ns).lastSeen == heardAt);

    // Quiet for PEER_EXPIRY: the registry would describe the peer now, but it
    // never has, so the peer stays published at its last announced address.
    tracker.expire(heardAt + ReplicaTracker::PEER_EXPIRY);
    CHECK((*tracker.localReplicas())[1] == body.addrs);
    state = tracker.peerState(peer, 2);
    REQUIRE(state.has_value());
    CHECK(!state->registry.has_value());
    CHECK(state->current(heardAt + ReplicaTracker::PEER_EXPIRY).lastSeen == heardAt);
    REQUIRE(tracker.announcementTargets()->size() == 1);

    // Restarted without announcements at a new address: the registry knows.
    std::vector<FullShardInfo> registry{
        FullShardInfo(peer, false, addresses(3, 10003), TernTime(5), 2),
    };
    tracker.seedRegistry(registry, heardAt + ReplicaTracker::PEER_EXPIRY);
    CHECK((*tracker.localReplicas())[1] == addresses(3, 10003));
    CHECK(tracker.announcementTargets()->at(0) == addresses(3, 10003));

    // Announcing again makes it authoritative again.
    auto restarted = announcement(peer, 2, false, addresses(4, 10004));
    tracker.recordAnnouncement(
                address(4, 10004),
                restarted,
                heardAt + ReplicaTracker::PEER_EXPIRY + 1_ns);
    CHECK((*tracker.localReplicas())[1] == restarted.addrs);
    tracker.seedRegistry(registry, heardAt + ReplicaTracker::PEER_EXPIRY + 2_ns);
    CHECK((*tracker.localReplicas())[1] == restarted.addrs);
}

TEST_CASE("a quiet peer is published at the address the registry learnt meanwhile") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    const auto peer = ShardReplicaId(ShardId(7), ReplicaId(1));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));

    std::vector<FullShardInfo> registry{
        FullShardInfo(peer, false, addresses(2, 10002), TernTime(5), 2),
    };
    tracker.seedRegistry(registry, TernTime(10));
    auto body = announcement(peer, 2, false, addresses(2, 10002));
    const TernTime heardAt(1'000'000'000);
    tracker.recordAnnouncement(address(2, 10002), body, heardAt);

    // The peer restarts at a new address without announcing, and registers
    // it. The poll that reports it arrives while its last announcement is
    // still fresh: we keep publishing what it announced, but remember what
    // the registry said.
    registry[0].addrs = addresses(3, 10003);
    registry[0].lastSeen = TernTime(6);
    tracker.seedRegistry(registry, heardAt + 1_ns);
    CHECK((*tracker.localReplicas())[1] == addresses(2, 10002));
    CHECK(tracker.peerState(peer, 2)->registry->addrs == addresses(3, 10003));

    // Once it has been quiet for PEER_EXPIRY, the registry's address is
    // published at once, without waiting for another poll.
    tracker.expire(heardAt + ReplicaTracker::PEER_EXPIRY);
    CHECK((*tracker.localReplicas())[1] == addresses(3, 10003));
    CHECK(tracker.announcementTargets()->at(0) == addresses(3, 10003));
}

TEST_CASE("announcements after expiry update the published local address") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    const auto peer = ShardReplicaId(ShardId(7), ReplicaId(1));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));
    const TernTime heardAt(1'000'000'000);
    auto body = announcement(peer, 2, false, addresses(2, 10002));
    tracker.recordAnnouncement(body.addrs[0], body, heardAt);

    std::vector<FullShardInfo> registry{
        FullShardInfo(peer, false, addresses(3, 10003), heardAt + 1_sec, 2),
    };
    tracker.seedRegistry(registry, heardAt + 1_sec);
    REQUIRE(tracker.localReplicas()->at(1) == body.addrs);

    const TernTime resumedAt = heardAt + ReplicaTracker::PEER_EXPIRY;
    SUBCASE("announcement arrives before the expiry pass") {}
    SUBCASE("announcement arrives after the expiry pass") {
        tracker.expire(resumedAt);
    }
    SUBCASE("registry poll arrives before the expiry pass") {
        tracker.seedRegistry(registry, resumedAt);
        CHECK(tracker.localReplicas()->at(1) == registry[0].addrs);
    }

    body.addrs = registry[0].addrs;
    tracker.recordAnnouncement(body.addrs[0], body, resumedAt);
    CHECK(tracker.localReplicas()->at(1) == body.addrs);
    REQUIRE(tracker.announcementTargets()->size() == 1);
    CHECK(tracker.announcementTargets()->at(0) == body.addrs);

    registry[0].lastSeen = resumedAt + 1_sec;
    tracker.seedRegistry(registry, resumedAt + 1_sec);
    tracker.recordAnnouncement(body.addrs[0], body, resumedAt + 1_sec);
    tracker.expire(resumedAt + 1_sec);
    CHECK(tracker.localReplicas()->at(1) == body.addrs);
}

TEST_CASE("wiped registry ports preserve the last announced address") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    const auto peer = ShardReplicaId(ShardId(7), ReplicaId(1));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));
    const TernTime heardAt(1'000'000'000);
    auto body = announcement(peer, 2, false, addresses(2, 10002));
    std::vector<FullShardInfo> registry{
        FullShardInfo(peer, false, addresses(2, 0), heardAt, 2),
    };
    SUBCASE("registry record arrives first") {
        tracker.seedRegistry(registry, heardAt);
        tracker.recordAnnouncement(body.addrs[0], body, heardAt + 1_sec);
    }
    SUBCASE("announcement arrives first") {
        tracker.recordAnnouncement(body.addrs[0], body, heardAt);
        tracker.seedRegistry(registry, heardAt + 1_sec);
    }

    const TernTime expiredAt = heardAt + 1_sec + ReplicaTracker::PEER_EXPIRY;
    tracker.expire(expiredAt);
    CHECK(tracker.localReplicas()->at(1) == body.addrs);
    REQUIRE(tracker.announcementTargets()->size() == 1);
    CHECK(tracker.announcementTargets()->at(0) == body.addrs);
    CHECK(tracker.peerState(peer, 2)->registry->addrs == registry[0].addrs);

    // A later announcement supplies the new fallback address, even if the
    // registry still has no usable address after it expires.
    body.addrs = addresses(3, 10003);
    tracker.recordAnnouncement(body.addrs[0], body, expiredAt);
    registry[0].lastSeen = expiredAt + ReplicaTracker::PEER_EXPIRY;
    tracker.seedRegistry(registry, registry[0].lastSeen);
    tracker.expire(registry[0].lastSeen);
    CHECK(tracker.localReplicas()->at(1) == body.addrs);
    REQUIRE(tracker.announcementTargets()->size() == 1);
    CHECK(tracker.announcementTargets()->at(0) == body.addrs);

    // A usable registry address still takes precedence over a stale announcement.
    registry[0].addrs = addresses(4, 10004);
    tracker.seedRegistry(registry, registry[0].lastSeen + 1_ns);
    CHECK(tracker.localReplicas()->at(1) == registry[0].addrs);
    CHECK(tracker.announcementTargets()->at(0) == registry[0].addrs);
}

TEST_CASE("an expired announcement supplies only the missing registry address") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    const auto peer = ShardReplicaId(ShardId(7), ReplicaId(1));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));
    const TernTime heardAt(1'000'000'000);
    auto body = announcement(peer, 3, true, addresses(2, 10002));
    tracker.recordAnnouncement(body.addrs[0], body, heardAt);
    std::vector<FullShardInfo> registry{
        FullShardInfo(peer, false, addresses(2, 0), heardAt + 1_sec, 3),
    };
    tracker.seedRegistry(registry, heardAt + 1_sec);
    tracker.expire(heardAt + ReplicaTracker::PEER_EXPIRY);
    CHECK(tracker.remoteLeaders()->empty());
    REQUIRE(tracker.announcementTargets()->size() == 1);
    CHECK(tracker.announcementTargets()->at(0) == body.addrs);

    registry[0].isLeader = true;
    tracker.seedRegistry(registry, heardAt + ReplicaTracker::PEER_EXPIRY + 1_ns);
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).addrs == body.addrs);
    CHECK(tracker.remoteLeaders()->at(0).lastSeen == registry[0].lastSeen);
}

TEST_CASE("peers known only from the registry never expire") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    const auto localPeer = ShardReplicaId(ShardId(7), ReplicaId(1));
    const auto remoteLeader = ShardReplicaId(ShardId(7), ReplicaId(0));
    ReplicaTracker tracker(logger, xmon, self, 2, true, addresses(1, 10001));

    // For example replicas running without -replica-announcements.
    std::vector<FullShardInfo> registry{
        FullShardInfo(localPeer, false, addresses(2, 10002), TernTime(1'000), 2),
        FullShardInfo(remoteLeader, true, addresses(3, 10003), TernTime(2'000), 3),
    };
    tracker.seedRegistry(registry, TernTime(1'000));

    tracker.expire(TernTime(1'000) + ReplicaTracker::PEER_EXPIRY * 10);
    CHECK((*tracker.localReplicas())[1] == addresses(2, 10002));
    auto remote = tracker.remoteLeaders();
    REQUIRE(remote->size() == 1);
    CHECK(remote->at(0).addrs == addresses(3, 10003));
    // Liveness of a peer that doesn't announce is what the registry last saw.
    CHECK(remote->at(0).lastSeen == TernTime(2'000));

    registry[1].lastSeen = TernTime(3'000);
    tracker.seedRegistry(registry, TernTime(3'000));
    CHECK(tracker.remoteLeaders()->at(0).lastSeen == TernTime(3'000));
}

TEST_CASE("remote leaders publish the progress they announce") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(4));
    ReplicaTracker tracker(logger, xmon, self, 0, true, addresses(1, 10001));

    // Known only from the registry: no progress yet.
    std::vector<FullShardInfo> seed{
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(4)), true, addresses(3, 10003), 0, 1),
    };
    tracker.seedRegistry(seed, TernTime(10));
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteProgress()->empty());

    auto remoteLeader = announcement(ShardReplicaId(ShardId(7), ReplicaId(4)), 1, true, addresses(3, 10003));
    remoteLeader.lastApplied = LogIdx(42);
    remoteLeader.lastAppliedTime = TernTime(15);
    tracker.recordAnnouncement(address(3, 10003), remoteLeader, TernTime(20));

    // A follower at that location and a local replica don't count.
    auto remoteFollower = announcement(ShardReplicaId(ShardId(7), ReplicaId(1)), 1, false, addresses(4, 10004));
    remoteFollower.lastApplied = LogIdx(40);
    tracker.recordAnnouncement(address(4, 10004), remoteFollower, TernTime(21));
    auto localFollower = announcement(ShardReplicaId(ShardId(7), ReplicaId(0)), 0, false, addresses(5, 10005));
    tracker.recordAnnouncement(address(5, 10005), localFollower, TernTime(22));

    auto progress = tracker.remoteProgress();
    REQUIRE(progress->size() == 1);
    CHECK(progress->at(0).locationId == 1);
    CHECK(progress->at(0).lastHeard == TernTime(20));
    CHECK(progress->at(0).lastApplied == LogIdx(42));
    CHECK(progress->at(0).lastAppliedTime == TernTime(15));

    // Registration and announcements run independently. A registration between
    // announcements must not discard the progress we just received.
    seed[0].lastSeen = TernTime(20) + 10_ms;
    tracker.seedRegistry(seed, TernTime(20) + 20_ms);
    progress = tracker.remoteProgress();
    REQUIRE(progress->size() == 1);
    CHECK(progress->at(0).lastHeard == TernTime(20));
    CHECK(progress->at(0).lastApplied == LogIdx(42));

    // A quiet leader stays in the progress, with the time we last heard
    // from it, so that the primary can see it has gone silent.
    tracker.expire(TernTime(20) + ReplicaTracker::PEER_EXPIRY);
    progress = tracker.remoteProgress();
    REQUIRE(progress->size() == 1);
    CHECK(progress->at(0).lastHeard == TernTime(20));

    // Another poll with unchanged registration data must preserve silence too.
    tracker.seedRegistry(seed, TernTime(20) + ReplicaTracker::PEER_EXPIRY);
    CHECK(tracker.remoteProgress()->size() == 1);

    // Registry access can remain healthy while inter-location traffic fails.
    seed[0].lastSeen = TernTime(20) + ReplicaTracker::PEER_EXPIRY;
    tracker.seedRegistry(seed, seed[0].lastSeen + 1_ns);
    progress = tracker.remoteProgress();
    REQUIRE(progress->size() == 1);
    CHECK(progress->at(0).lastHeard == TernTime(20));
    CHECK(tracker.remoteLeaders()->size() == 1);

    remoteLeader.lastApplied = LogIdx(100);
    remoteLeader.lastAppliedTime = seed[0].lastSeen;
    tracker.recordAnnouncement(address(3, 10003), remoteLeader, seed[0].lastSeen + 100_ms);
    progress = tracker.remoteProgress();
    REQUIRE(progress->size() == 1);
    CHECK(progress->at(0).lastHeard == seed[0].lastSeen + 100_ms);
    CHECK(progress->at(0).lastApplied == LogIdx(100));
}

TEST_CASE("progress is not transferred to an address learned from the registry") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(4));
    ReplicaTracker tracker(logger, xmon, self, 0, true, addresses(1, 10001));
    auto leader = announcement(ShardReplicaId(ShardId(7), ReplicaId(4)), 1, true, addresses(2, 10002));
    leader.lastApplied = LogIdx(42);
    tracker.recordAnnouncement(address(2, 10002), leader, TernTime(10));

    std::vector<FullShardInfo> registry{
        FullShardInfo(leader.shrid, true, addresses(3, 10003), TernTime(20), 1),
    };
    tracker.seedRegistry(registry, TernTime(30));
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).addrs == leader.addrs);
    REQUIRE(tracker.remoteProgress()->size() == 1);
    CHECK(tracker.remoteProgress()->at(0).lastApplied == LogIdx(42));

    tracker.expire(TernTime(10) + ReplicaTracker::PEER_EXPIRY);
    REQUIRE(tracker.remoteLeaders()->size() == 1);
    CHECK(tracker.remoteLeaders()->at(0).addrs == registry[0].addrs);
    CHECK(tracker.remoteProgress()->empty());

    leader.addrs = registry[0].addrs;
    leader.lastApplied = LogIdx(100);
    tracker.recordAnnouncement(address(3, 10003), leader, TernTime(10) + ReplicaTracker::PEER_EXPIRY + 1_ns);
    REQUIRE(tracker.remoteProgress()->size() == 1);
    CHECK(tracker.remoteProgress()->at(0).lastApplied == LogIdx(100));
}
