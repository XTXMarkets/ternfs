// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "ReplicaTracker.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

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

TEST_CASE("registry seeds local replicas and remote leaders") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    ReplicaTracker tracker(self, 2, true, addresses(1, 10001));

    std::vector<FullShardInfo> replicas{
        FullShardInfo(self, true, addresses(1, 10001), 0, 2),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(1)), false, addresses(2, 10002), 0, 2),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(0)), true, addresses(3, 10003), TernTime(1'000), 3),
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(2)), false, addresses(4, 10004), 0, 3),
        FullShardInfo(ShardReplicaId(ShardId(8), ReplicaId(1)), true, addresses(5, 10005), 0, 2),
        // A leader at another location whose address the registry has wiped.
        FullShardInfo(ShardReplicaId(ShardId(7), ReplicaId(0)), true, AddrsInfo(), 0, 4),
    };
    tracker.seedRegistry(replicas);

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
    CHECK(remote->at(0).lastSeen == TernTime(1'000));
}

TEST_CASE("later registry polls update peers") {
    const auto self = ShardReplicaId(ShardId(7), ReplicaId(0));
    const auto peer = ShardReplicaId(ShardId(7), ReplicaId(1));
    const auto remoteLeader = ShardReplicaId(ShardId(7), ReplicaId(4));
    ReplicaTracker tracker(self, 2, true, addresses(1, 10001));

    std::vector<FullShardInfo> registry{
        FullShardInfo(peer, false, addresses(2, 10002), TernTime(1'000), 2),
        FullShardInfo(remoteLeader, true, addresses(3, 10003), TernTime(1'000), 3),
    };
    tracker.seedRegistry(registry);
    CHECK((*tracker.localReplicas())[1] == addresses(2, 10002));
    REQUIRE(tracker.remoteLeaders()->size() == 1);

    // The peer moved host, the remote leader was heard from again.
    registry[0].addrs = addresses(3, 10003);
    registry[1].lastSeen = TernTime(2'000);
    tracker.seedRegistry(registry);
    CHECK((*tracker.localReplicas())[1] == addresses(3, 10003));
    CHECK(tracker.remoteLeaders()->at(0).lastSeen == TernTime(2'000));

    // The remote leader stepped down and the peer's address was wiped: the
    // leader list empties, the peer keeps its last address.
    registry[0].addrs = AddrsInfo();
    registry[1].isLeader = false;
    tracker.seedRegistry(registry);
    CHECK((*tracker.localReplicas())[1] == addresses(3, 10003));
    CHECK(tracker.remoteLeaders()->empty());
    auto state = tracker.peerState(peer, 2);
    REQUIRE(state.has_value());
    CHECK(state->registryLastSeen == TernTime(1'000));
}
