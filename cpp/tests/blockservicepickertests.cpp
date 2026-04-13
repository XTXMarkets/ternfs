// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <thread>
#include <atomic>
#include <vector>
#include <unordered_map>

#include "BlockServicePicker.hpp"
#include "BlockServicesCacheDB.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

static Logger testLogger(LogLevel::LOG_ERROR, STDERR_FILENO, false, false);
static std::shared_ptr<XmonAgent> testXmon;

static BlockServicePicker makePicker(uint8_t maxBlocksToPick = 15,
                                     Duration writableDelay = 0_sec,
                                     uint64_t hddDriveThroughput = 0,
                                     uint64_t flashDriveThroughput = 0) {
    return BlockServicePicker(testLogger, testXmon, maxBlocksToPick, writableDelay,
                              hddDriveThroughput, flashDriveThroughput);
}

static FailureDomain fdWith(uint8_t v) {
    FailureDomain fd;
    for (int i = 0; i < 16; ++i) fd.name.data[i] = v;
    return fd;
}

static BlockServiceInfoShort bs(uint64_t id, uint8_t loc, uint8_t sc, uint8_t fdByte) {
    BlockServiceInfoShort x;
    x.id = BlockServiceId(id);
    x.locationId = loc;
    x.storageClass = sc;
    x.failureDomain = fdWith(fdByte);
    return x;
}

static std::unordered_map<uint64_t, BlockServiceCache> makeCatalog(const std::vector<BlockServiceInfoShort>& services) {
    std::unordered_map<uint64_t, BlockServiceCache> cache;
    for (const auto& svc : services) {
        BlockServiceCache entry;
        entry.locationId = svc.locationId;
        entry.storageClass = svc.storageClass;
        entry.failureDomain = svc.failureDomain.name.data;
        entry.flags = BlockServiceFlags::EMPTY;
        entry.availableBytes = 1000000;
        entry.capacityBytes = 10000000;
        entry.blocks = 0;
        entry.hasFiles = false;
        cache[svc.id.u64] = entry;
    }
    return cache;
}

TEST_CASE("picker basic selection") {
    auto p = makePicker();
    std::vector<BlockServiceInfoShort> catalog{
        bs(1, 1, FLASH_STORAGE, 1),
        bs(2, 1, FLASH_STORAGE, 2),
        bs(3, 1, FLASH_STORAGE, 3),
        bs(4, 2, HDD_STORAGE, 4)
    };
    auto cache = makeCatalog(catalog);
    p.update(cache);

    std::vector<BlockServiceId> out;
    auto err = p.pick(1, FLASH_STORAGE, 2, {}, out);
    CHECK(err == TernError::NO_ERROR);
    CHECK(out.size() == 2);
    CHECK(out[0] != out[1]);
}

TEST_CASE("picker blacklist by id and failure domain") {
    auto p = makePicker();
    std::vector<BlockServiceInfoShort> catalog{
        bs(10, 1, FLASH_STORAGE, 7),
        bs(11, 1, FLASH_STORAGE, 8),
        bs(12, 1, FLASH_STORAGE, 9)
    };
    auto cache = makeCatalog(catalog);
    p.update(cache);

    std::vector<BlacklistEntry> bl;
    BlacklistEntry e1; e1.blockService = BlockServiceId(10); bl.push_back(e1);
    BlacklistEntry e2; e2.failureDomain = fdWith(8); bl.push_back(e2);

    std::vector<BlockServiceId> out;
    auto err = p.pick(1, FLASH_STORAGE, 2, bl, out);
    // Only bs 12 remains available
    CHECK(err == TernError::COULD_NOT_PICK_BLOCK_SERVICES);
    CHECK(out.size() == 0);
}

TEST_CASE("picker insufficient candidates") {
    auto p = makePicker();
    std::vector<BlockServiceInfoShort> catalog{
        bs(20, 1, HDD_STORAGE, 1)
    };
    auto cache = makeCatalog(catalog);
    p.update(cache);

    std::vector<BlockServiceId> out;
    auto err = p.pick(1, HDD_STORAGE, 2, {}, out);
    CHECK(err == TernError::COULD_NOT_PICK_BLOCK_SERVICES);
    CHECK(out.size() == 0);
}

TEST_CASE("picker concurrency update while picks") {
    auto p = makePicker();
    std::atomic<bool> stop{false};

    // Start with many candidates
    std::vector<BlockServiceInfoShort> catalog;
    for (uint64_t i = 0; i < 64; ++i) { catalog.push_back(bs(100+i, 1, FLASH_STORAGE, (uint8_t)i)); }
    auto cache = makeCatalog(catalog);
    p.update(cache);

    std::thread t1([&]{
        std::vector<BlockServiceId> out;
        while (!stop.load()) {
            auto err = p.pick(1, FLASH_STORAGE, 8, {}, out);
            CHECK(err == TernError::NO_ERROR);
            CHECK(out.size() == 8);
        }
    });

    // Rebuild state repeatedly
    for (int round = 0; round < 100; ++round) {
        std::vector<BlockServiceInfoShort> cat2;
        for (uint64_t i = 0; i < 64; ++i) { cat2.push_back(bs(200+i+round, 1, FLASH_STORAGE, (uint8_t)i)); }
        auto cache2 = makeCatalog(cat2);
        p.update(cache2);
    }

    stop = true;
    t1.join();
}

TEST_CASE("picker weighted distribution") {
    // Use maxBlocksToPick=1 to avoid scaling effects on small clusters
    auto p = makePicker(1);

    // Create services with different weights
    // Service 1000: 10 MB available (weight 10M)
    // Service 2000: 20 MB available (weight 20M)
    // Service 3000: 30 MB available (weight 30M)
    // Service 4000: 40 MB available (weight 40M)
    // Total: 100M, so expected ratios are 10%, 20%, 30%, 40%
    std::vector<BlockServiceInfoShort> catalog{
        bs(1000, 1, FLASH_STORAGE, 1),
        bs(2000, 1, FLASH_STORAGE, 2),
        bs(3000, 1, FLASH_STORAGE, 3),
        bs(4000, 1, FLASH_STORAGE, 4)
    };

    std::unordered_map<uint64_t, BlockServiceCache> cache;
    for (const auto& svc : catalog) {
        BlockServiceCache entry;
        entry.locationId = svc.locationId;
        entry.storageClass = svc.storageClass;
        entry.failureDomain = svc.failureDomain.name.data;
        entry.flags = BlockServiceFlags::EMPTY; // writable
        entry.capacityBytes = 100000000;
        entry.blocks = 0;
        entry.hasFiles = false;

        // Set different availableBytes for each service to create weights
        if (svc.id.u64 == 1000) entry.availableBytes = 10000000;
        else if (svc.id.u64 == 2000) entry.availableBytes = 20000000;
        else if (svc.id.u64 == 3000) entry.availableBytes = 30000000;
        else if (svc.id.u64 == 4000) entry.availableBytes = 40000000;

        cache[svc.id.u64] = entry;
    }

    p.update(cache);

    // Perform many picks to measure distribution
    const int NUM_ITERATIONS = 10000;
    std::unordered_map<uint64_t, int> pickCounts;

    for (int i = 0; i < NUM_ITERATIONS; ++i) {
        std::vector<BlockServiceId> out;
        auto err = p.pick(1, FLASH_STORAGE, 1, {}, out);
        REQUIRE(err == TernError::NO_ERROR);
        REQUIRE(out.size() == 1);
        pickCounts[out[0].u64]++;
    }

    // Check that all services were picked
    CHECK(pickCounts.size() == 4);
    CHECK(pickCounts.count(1000) > 0);
    CHECK(pickCounts.count(2000) > 0);
    CHECK(pickCounts.count(3000) > 0);
    CHECK(pickCounts.count(4000) > 0);

    // Verify distribution is roughly proportional to weights
    // Expected: 1000->1000, 2000->2000, 3000->3000, 4000->4000
    // Allow 20% deviation from expected
    CHECK(pickCounts[1000] > 800);
    CHECK(pickCounts[1000] < 1200);
    CHECK(pickCounts[2000] > 1600);
    CHECK(pickCounts[2000] < 2400);
    CHECK(pickCounts[3000] > 2400);
    CHECK(pickCounts[3000] < 3600);
    CHECK(pickCounts[4000] > 3200);
    CHECK(pickCounts[4000] < 4800);
}

TEST_CASE("picker blacklist enforcement") {
    auto p = makePicker();

    // Create 6 services in 6 different failure domains
    std::vector<BlockServiceInfoShort> catalog{
        bs(1001, 1, FLASH_STORAGE, 1),  // FD 1
        bs(1002, 1, FLASH_STORAGE, 2),  // FD 2 - blacklist this FD
        bs(1003, 1, FLASH_STORAGE, 3),  // FD 3
        bs(1004, 1, FLASH_STORAGE, 4),  // FD 4 - blacklist service 1004
        bs(1005, 1, FLASH_STORAGE, 5),  // FD 5
        bs(1006, 1, FLASH_STORAGE, 6)   // FD 6
    };

    auto cache = makeCatalog(catalog);
    p.update(cache);

    // Blacklist: entire FD 2, and service 1004 from FD 4
    std::vector<BlacklistEntry> blacklist;
    BlacklistEntry fdBlacklist;
    fdBlacklist.failureDomain = fdWith(2);
    blacklist.push_back(fdBlacklist);

    BlacklistEntry serviceBlacklist;
    serviceBlacklist.blockService = BlockServiceId(1004);
    blacklist.push_back(serviceBlacklist);

    // Perform many picks to verify blacklisted items never appear
    const int NUM_ITERATIONS = 5000;
    std::unordered_set<uint64_t> pickedServices;

    for (int i = 0; i < NUM_ITERATIONS; ++i) {
        std::vector<BlockServiceId> out;
        auto err = p.pick(1, FLASH_STORAGE, 1, blacklist, out);
        REQUIRE(err == TernError::NO_ERROR);
        REQUIRE(out.size() == 1);
        pickedServices.insert(out[0].u64);

        // Verify blacklisted services never picked
        CHECK(out[0].u64 != 1002);  // FD 2 is blacklisted
        CHECK(out[0].u64 != 1004);  // Service 1004 is blacklisted
    }

    // Verify only valid services were picked
    CHECK(pickedServices.count(1001) > 0);  // Should be picked
    CHECK(pickedServices.count(1002) == 0); // Blacklisted FD
    CHECK(pickedServices.count(1003) > 0);  // Should be picked
    CHECK(pickedServices.count(1004) == 0); // Blacklisted service
    CHECK(pickedServices.count(1005) > 0);  // Should be picked
    CHECK(pickedServices.count(1006) > 0);  // Should be picked
}

TEST_CASE("picker weighted distribution with blacklist") {
    // Use maxBlocksToPick=1 to avoid scaling effects on small clusters
    auto p = makePicker(1);

    // Create services with varying weights across multiple failure domains
    // FD 1: service 100 (10MB), service 101 (10MB) - total 20MB
    // FD 2: service 200 (40MB) - BLACKLIST THIS
    // FD 3: service 300 (20MB), service 301 (20MB) - total 40MB
    // Without blacklist: FD1=20MB (20%), FD2=40MB (40%), FD3=40MB (40%)
    // With blacklist: FD1=20MB (33%), FD3=40MB (67%)

    std::vector<BlockServiceInfoShort> catalog{
        bs(100, 1, FLASH_STORAGE, 1),
        bs(101, 1, FLASH_STORAGE, 1),
        bs(200, 1, FLASH_STORAGE, 2),
        bs(300, 1, FLASH_STORAGE, 3),
        bs(301, 1, FLASH_STORAGE, 3)
    };

    std::unordered_map<uint64_t, BlockServiceCache> cache;
    for (const auto& svc : catalog) {
        BlockServiceCache entry;
        entry.locationId = svc.locationId;
        entry.storageClass = svc.storageClass;
        entry.failureDomain = svc.failureDomain.name.data;
        entry.flags = BlockServiceFlags::EMPTY;
        entry.capacityBytes = 100000000;
        entry.blocks = 0;
        entry.hasFiles = false;

        if (svc.id.u64 == 100 || svc.id.u64 == 101) entry.availableBytes = 10000000;
        else if (svc.id.u64 == 200) entry.availableBytes = 40000000;
        else if (svc.id.u64 == 300 || svc.id.u64 == 301) entry.availableBytes = 20000000;

        cache[svc.id.u64] = entry;
    }

    p.update(cache);

    // Blacklist entire FD 2
    std::vector<BlacklistEntry> blacklist;
    BlacklistEntry fdBlacklist;
    fdBlacklist.failureDomain = fdWith(2);
    blacklist.push_back(fdBlacklist);

    // Perform many picks
    const int NUM_ITERATIONS = 6000;
    std::unordered_map<uint64_t, int> pickCounts;

    for (int i = 0; i < NUM_ITERATIONS; ++i) {
        std::vector<BlockServiceId> out;
        auto err = p.pick(1, FLASH_STORAGE, 1, blacklist, out);
        REQUIRE(err == TernError::NO_ERROR);
        REQUIRE(out.size() == 1);

        // Verify blacklisted service never picked
        CHECK(out[0].u64 != 200);
        pickCounts[out[0].u64]++;
    }

    // Verify FD 2 was never picked
    CHECK(pickCounts.count(200) == 0);

    // Verify FD 1 and FD 3 services were picked
    CHECK(pickCounts.count(100) > 0);
    CHECK(pickCounts.count(101) > 0);
    CHECK(pickCounts.count(300) > 0);
    CHECK(pickCounts.count(301) > 0);

    // Count picks by FD
    int fd1Picks = pickCounts[100] + pickCounts[101];
    int fd3Picks = pickCounts[300] + pickCounts[301];

    // FD1 (20MB) vs FD3 (40MB) => expect roughly 1:2 ratio
    // FD1 should get ~2000 picks, FD3 should get ~4000 picks
    // Allow 20% deviation
    CHECK(fd1Picks > 1600);
    CHECK(fd1Picks < 2400);
    CHECK(fd3Picks > 3200);
    CHECK(fd3Picks < 4800);

    // Within each FD, services should be roughly equal
    // FD1: 100 and 101 should each get ~1000 (allow 25% deviation)
    CHECK(pickCounts[100] > 750);
    CHECK(pickCounts[100] < 1250);
    CHECK(pickCounts[101] > 750);
    CHECK(pickCounts[101] < 1250);

    // FD3: 300 and 301 should each get ~2000 (allow 20% deviation)
    CHECK(pickCounts[300] > 1600);
    CHECK(pickCounts[300] < 2400);
    CHECK(pickCounts[301] > 1600);
    CHECK(pickCounts[301] < 2400);
}
TEST_CASE("picker multi-service weighted distribution") {
    auto p = makePicker();

    // Create 20 failure domains, each with 20-25 services with varying weights
    // This creates a large heterogeneous pool to test distribution
    std::vector<BlockServiceInfoShort> catalog;
    std::unordered_map<uint64_t, BlockServiceCache> cache;

    std::unordered_map<uint8_t, uint64_t> fdTotalWeights;  // FD byte -> total weight
    std::unordered_map<uint64_t, uint64_t> serviceWeights;  // service id -> weight
    uint64_t totalSystemWeight = 0;

    uint64_t serviceId = 10000;
    for (uint8_t fdByte = 1; fdByte <= 20; ++fdByte) {
        // Each FD gets 20-25 services
        int numServices = 20 + (fdByte % 6);
        uint64_t fdWeight = 0;

        for (int svcIdx = 0; svcIdx < numServices; ++svcIdx) {
            // Vary weights within FD: some small (5MB), some medium (10-20MB), some large (30-50MB)
            uint64_t weight;
            if (svcIdx % 5 == 0) {
                weight = 5000000;  // 5MB - small
            } else if (svcIdx % 3 == 0) {
                weight = (10 + (svcIdx % 10)) * 1000000;  // 10-20MB - medium
            } else {
                weight = (30 + (svcIdx % 20)) * 1000000;  // 30-50MB - large
            }

            catalog.push_back(bs(serviceId, 1, FLASH_STORAGE, fdByte));

            BlockServiceCache entry;
            entry.locationId = 1;
            entry.storageClass = FLASH_STORAGE;
            entry.failureDomain = fdWith(fdByte).name.data;
            entry.flags = BlockServiceFlags::EMPTY;
            entry.capacityBytes = 100000000;
            entry.availableBytes = weight;
            entry.blocks = 0;
            entry.hasFiles = false;
            cache[serviceId] = entry;

            serviceWeights[serviceId] = weight;
            fdWeight += weight;
            totalSystemWeight += weight;
            serviceId++;
        }
        fdTotalWeights[fdByte] = fdWeight;
    }

    p.update(cache);

    // Test picking different numbers of services: 1, 3, 5, 10, 15
    for (int needed : {1, 3, 5, 10, 15}) {
        std::unordered_map<uint64_t, int> pickCounts;
        std::unordered_map<uint8_t, int> fdPickCounts;

        // Run many iterations to gather statistics - 1M for good distribution
        const int NUM_ITERATIONS = 1000000;
        for (int i = 0; i < NUM_ITERATIONS; ++i) {
            std::vector<BlockServiceId> out;
            auto err = p.pick(1, FLASH_STORAGE, needed, {}, out);
            REQUIRE(err == TernError::NO_ERROR);
            REQUIRE(out.size() == needed);

            // Count each picked service
            for (const auto& svc : out) {
                pickCounts[svc.u64]++;

                // Find which FD this service belongs to
                for (uint8_t fdByte = 1; fdByte <= 20; ++fdByte) {
                    if (cache[svc.u64].failureDomain == fdWith(fdByte).name.data) {
                        fdPickCounts[fdByte]++;
                        break;
                    }
                }
            }
        }

        // Verify that picks are distributed according to weights
        uint64_t totalPicks = (uint64_t)NUM_ITERATIONS * needed;

        // Check failure domain distribution
        for (uint8_t fdByte = 1; fdByte <= 20; ++fdByte) {
            double expectedRatio = (double)fdTotalWeights[fdByte] / totalSystemWeight;
            double expectedPicks = totalPicks * expectedRatio;
            int actualPicks = fdPickCounts[fdByte];

            // Allow 5% deviation for FD distribution
            double tolerance = 0.05;
            if (expectedPicks > 1000) {  // Only check FDs with significant expected picks
                CHECK(actualPicks > expectedPicks * (1.0 - tolerance));
                CHECK(actualPicks < expectedPicks * (1.0 + tolerance));
            }
        }

        // Check individual service distribution for services with significant weight
        // Focus on services that should get a reasonable number of picks
        for (const auto& [svcId, weight] : serviceWeights) {
            double expectedRatio = (double)weight / totalSystemWeight;
            double expectedPicks = totalPicks * expectedRatio;
            int actualPicks = pickCounts[svcId];

            // Only validate services we expect to see picked frequently enough
            if (expectedPicks > 500) {
                // Allow 15% deviation for individual services (tight with 10M iterations)
                double tolerance = 0.15;
                if (needed >= 10) {
                    tolerance = 0.1;
                }
                CHECK(actualPicks > expectedPicks * (1.0 - tolerance));
                CHECK(actualPicks < expectedPicks * (1.0 + tolerance));
            }
        }
    }

    // Verify that the total number of unique services picked is reasonable
    // We should see most services picked at least once across all iterations
    std::unordered_set<uint64_t> allPickedServices;
    const int FINAL_ITERATIONS = 1000000;
    for (int i = 0; i < FINAL_ITERATIONS; ++i) {
        std::vector<BlockServiceId> out;
        auto err = p.pick(1, FLASH_STORAGE, 10, {}, out);
        REQUIRE(err == TernError::NO_ERROR);
        for (const auto& svc : out) {
            allPickedServices.insert(svc.u64);
        }
    }

    // With weighted selection, we should see a good portion of services
    // (not necessarily all, since low-weight services may be picked rarely)
    CHECK(allPickedServices.size() > serviceWeights.size() * 0.9);
}

TEST_CASE("picker weight clamping for dominant failure domain") {
    auto p = makePicker();
    std::vector<BlockServiceInfoShort> catalog;

    // Scenario:
    // FD 1: Dominant (10000)
    // FD 2: Large (5000)
    // FD 3-52: Small (100 each) - 50 FDs
    // Total FDs: 52
    //
    // With clamping (maxBlocksToPick=15):
    // FD1 and FD2 are both above the cap and get clamped to the same weight.
    // Small FDs keep their exact weights (100).
    // cap = S_unclamped / (15 - 2) = 5000 / 13 = 384
    // newTotal = 2*384 + 50*100 = 5768

    // Add FD 1
    catalog.push_back(bs(1, 1, FLASH_STORAGE, 1));
    // Add FD 2
    catalog.push_back(bs(2, 1, FLASH_STORAGE, 2));
    // Add FD 3-52
    for (int i = 3; i <= 52; ++i) {
        catalog.push_back(bs(i, 1, FLASH_STORAGE, i));
    }

    auto cache = makeCatalog(catalog);

    // Set weights
    cache[1].availableBytes = 10000;
    cache[2].availableBytes = 5000;
    for (int i = 3; i <= 52; ++i) {
        cache[i].availableBytes = 100;
    }

    p.update(cache);

    double cap = 384.0;
    double total = 2 * cap + 50 * 100.0;
    double probClamped = cap / total;       // ~6.66% for FD1 and FD2
    double probSmall = 100.0 / total;       // ~1.73% for each small FD

    // Run many iterations
    const int NUM_ITERATIONS = 1000000;
    std::unordered_map<uint64_t, int> picks;

    for (int i = 0; i < NUM_ITERATIONS; ++i) {
        std::vector<BlockServiceId> out;
        auto err = p.pick(1, FLASH_STORAGE, 1, {}, out);
        REQUIRE(err == TernError::NO_ERROR);
        REQUIRE(out.size() == 1);
        picks[out[0].u64]++;
    }

    // FD 1 and FD 2: both clamped to same weight, so approximately equal
    double actualProb1 = (double)picks[1] / NUM_ITERATIONS;
    double actualProb2 = (double)picks[2] / NUM_ITERATIONS;
    CHECK(actualProb1 > probClamped * 0.85);
    CHECK(actualProb1 < probClamped * 1.15);
    CHECK(actualProb2 > probClamped * 0.85);
    CHECK(actualProb2 < probClamped * 1.15);

    // Small FDs: keep exact weights
    double totalSmallPicks = 0;
    for (int i = 3; i <= 52; ++i) {
        totalSmallPicks += picks[i];
    }
    double actualProbSmallAvg = (totalSmallPicks / 50.0) / NUM_ITERATIONS;
    CHECK(actualProbSmallAvg > probSmall * 0.85);
    CHECK(actualProbSmallAvg < probSmall * 1.15);

    // Clamped FDs > small FDs
    CHECK(picks[1] > (totalSmallPicks / 50.0));
    CHECK(picks[2] > (totalSmallPicks / 50.0));
}

TEST_CASE("picker never picks same failure domain twice") {
    // Test various configurations: different FD counts, service counts, and needed values
    struct TestConfig {
        int numFDs;
        int servicesPerFD;
        int needed;
        uint64_t baseWeight;
    };

    std::vector<TestConfig> configs = {
        {3,  1, 2,  1000000},   // minimal: 3 FDs, 1 service each, pick 2
        {3,  1, 3,  1000000},   // pick all 3
        {5,  3, 4,  1000000},   // 5 FDs with 3 services each, pick 4
        {15, 1, 15, 1000000},   // exactly maxBlocksToPick FDs
        {20, 5, 10, 1000000},   // 20 FDs, pick 10
        {3,  1, 2,  1},         // tiny weights (post-scaling edge case)
        {52, 1, 15, 100},       // many FDs, small weights
    };

    for (const auto& cfg : configs) {
        auto p = makePicker();
        std::unordered_map<uint64_t, BlockServiceCache> cache;
        std::unordered_map<uint64_t, uint8_t> serviceToFD;  // service id -> FD byte

        uint64_t id = 1;
        for (int fd = 1; fd <= cfg.numFDs; fd++) {
            for (int s = 0; s < cfg.servicesPerFD; s++) {
                BlockServiceCache entry;
                entry.locationId = 1;
                entry.storageClass = FLASH_STORAGE;
                entry.failureDomain = fdWith(fd).name.data;
                entry.flags = BlockServiceFlags::EMPTY;
                entry.availableBytes = cfg.baseWeight;
                entry.capacityBytes = cfg.baseWeight * 10;
                entry.blocks = 0;
                entry.hasFiles = false;
                cache[id] = entry;
                serviceToFD[id] = fd;
                id++;
            }
        }

        p.update(cache);

        const int NUM_ITERATIONS = 10000;
        for (int i = 0; i < NUM_ITERATIONS; i++) {
            std::vector<BlockServiceId> out;
            auto err = p.pick(1, FLASH_STORAGE, cfg.needed, {}, out);
            REQUIRE(err == TernError::NO_ERROR);
            REQUIRE(out.size() == cfg.needed);

            std::unordered_set<uint8_t> pickedFDs;
            for (const auto& svc : out) {
                uint8_t fd = serviceToFD[svc.u64];
                CHECK(pickedFDs.insert(fd).second);  // fails if FD already picked
            }
        }
    }
}

TEST_CASE("picker blacklist rescale metric is bumped") {
    // Create a scenario where blacklisting a *service* (not FD) breaks the stride
    // invariant, forcing the slow path (rescale).
    // 3 FDs each with weight 100, maxBlocksToPick=3. step=100, maxFdWeight=100 → OK.
    // Blacklisting a service worth 50 from FD3 → totalWeight=250, maxFdWeight=100,
    // step=83 < 100 → invariant broken → slow path with rescale.
    auto p = makePicker(3);

    std::unordered_map<uint64_t, BlockServiceCache> cache;
    auto makeEntry = [](uint8_t fd, uint64_t avail) {
        BlockServiceCache entry;
        entry.locationId = 1;
        entry.storageClass = FLASH_STORAGE;
        entry.failureDomain = fdWith(fd).name.data;
        entry.flags = BlockServiceFlags::EMPTY;
        entry.availableBytes = avail;
        entry.capacityBytes = avail * 10;
        entry.blocks = 0;
        entry.hasFiles = false;
        return entry;
    };

    cache[1] = makeEntry(1, 100);
    cache[2] = makeEntry(2, 100);
    cache[3] = makeEntry(3, 50);
    cache[4] = makeEntry(3, 50);

    p.update(cache);
    p.resetStats();

    // Blacklist service 3 (one of FD3's services, weight 50)
    std::vector<BlacklistEntry> blacklist;
    BlacklistEntry bl;
    bl.blockService = BlockServiceId(3);
    blacklist.push_back(bl);

    for (int i = 0; i < 100; i++) {
        std::vector<BlockServiceId> out;
        auto err = p.pick(1, FLASH_STORAGE, 3, blacklist, out);
        REQUIRE(err == TernError::NO_ERROR);
        REQUIRE(out.size() == 3);
    }

    auto stats = p.getStats();
    bool foundRescales = false;
    for (const auto& ls : stats.locStorage) {
        if (ls.blacklistRepicks > 0) {
            foundRescales = true;
            CHECK(ls.blacklistRepicks == 100);
        }
    }
    CHECK(foundRescales);
}

TEST_CASE("picker extreme weight disparity uniform at max load") {
    // Scenario: 200 existing FDs (nearly full, 5GB/disk × 100 disks = 500GB/FD)
    // plus 5 brand-new FDs (empty, 20TB/disk × 100 disks = 2000TB/FD).
    // Weight ratio per FD: 2000TB / 500GB = 4000x.
    // At max load (default), ratio=1.0 clamps all FDs to minFdWeight, so distribution
    // should be near-uniform across all FDs regardless of capacity disparity.
    auto p = makePicker(15, 0_sec, 600'000'000, 600'000'000);

    const uint64_t EXISTING_BYTES_PER_DISK = 5000000000ULL;       // 5 GB
    const uint64_t NEW_BYTES_PER_DISK = 20000000000000ULL;         // 20 TB
    const int DISKS_PER_FD = 100;
    const int EXISTING_FDS = 200;
    const int NEW_FDS = 5;
    const int TOTAL_FDS = EXISTING_FDS + NEW_FDS;
    const int NEEDED = 15;

    std::unordered_map<uint64_t, BlockServiceCache> cache;
    std::unordered_map<uint64_t, uint8_t> serviceToFd;

    uint64_t id = 1;
    for (int fd = 1; fd <= EXISTING_FDS; fd++) {
        for (int d = 0; d < DISKS_PER_FD; d++) {
            BlockServiceCache entry;
            entry.locationId = 1;
            entry.storageClass = FLASH_STORAGE;
            entry.failureDomain = fdWith(fd).name.data;
            entry.flags = BlockServiceFlags::EMPTY;
            entry.availableBytes = EXISTING_BYTES_PER_DISK;
            entry.capacityBytes = EXISTING_BYTES_PER_DISK * 10;
            entry.blocks = 0;
            entry.hasFiles = false;
            cache[id] = entry;
            serviceToFd[id] = fd;
            id++;
        }
    }
    for (int fd = 0; fd < NEW_FDS; fd++) {
        uint8_t fdByte = EXISTING_FDS + 1 + fd;
        for (int d = 0; d < DISKS_PER_FD; d++) {
            BlockServiceCache entry;
            entry.locationId = 1;
            entry.storageClass = FLASH_STORAGE;
            entry.failureDomain = fdWith(fdByte).name.data;
            entry.flags = BlockServiceFlags::EMPTY;
            entry.availableBytes = NEW_BYTES_PER_DISK;
            entry.capacityBytes = NEW_BYTES_PER_DISK;
            entry.blocks = 0;
            entry.hasFiles = false;
            cache[id] = entry;
            serviceToFd[id] = fdByte;
            id++;
        }
    }

    p.update(cache);

    const int NUM_ITERATIONS = 100000;
    std::unordered_map<uint8_t, int> fdPickCounts;

    for (int i = 0; i < NUM_ITERATIONS; i++) {
        std::vector<BlockServiceId> out;
        auto err = p.pick(1, FLASH_STORAGE, NEEDED, {}, out);
        REQUIRE(err == TernError::NO_ERROR);
        REQUIRE(out.size() == NEEDED);

        for (const auto& svc : out) {
            fdPickCounts[serviceToFd[svc.u64]]++;
        }
    }

    uint64_t totalPicks = (uint64_t)NUM_ITERATIONS * NEEDED;
    double expectedPerFd = (double)totalPicks / TOTAL_FDS;

    for (const auto& [fd, count] : fdPickCounts) {
        double deviation = std::abs((double)count - expectedPerFd) / expectedPerFd;
        CHECK_MESSAGE(deviation < 0.15,
            "FD ", (int)fd, " got ", count, " picks, expected ~", (int)expectedPerFd,
            " (deviation ", deviation * 100, "%)");
    }
}

TEST_CASE("picker throughput adaptation increases ratio at low load") {
    // Low observed throughput → high effectiveMaxRatio → capacity-proportional picks.
    // Use maxBlocksToPick=1 so stride clamping doesn't interfere (only ratio clamping matters).
    _setCurrentTime(ternNow());

    auto p = makePicker(1, 0_sec, 600'000'000, 600'000'000);

    const int EXISTING_FDS = 4;
    const int NEW_FDS = 1;

    std::unordered_map<uint64_t, BlockServiceCache> cache;
    std::unordered_map<uint64_t, uint8_t> serviceToFd;

    uint64_t id = 1;
    for (int fd = 1; fd <= EXISTING_FDS; fd++) {
        BlockServiceCache entry;
        entry.locationId = 1;
        entry.storageClass = FLASH_STORAGE;
        entry.failureDomain = fdWith(fd).name.data;
        entry.flags = BlockServiceFlags::EMPTY;
        entry.availableBytes = 1'000'000'000ULL;  // 1GB
        entry.capacityBytes = 10'000'000'000ULL;
        entry.blocks = 0;
        entry.hasFiles = false;
        cache[id] = entry;
        serviceToFd[id] = fd;
        id++;
    }
    {
        uint8_t fdByte = EXISTING_FDS + 1;
        BlockServiceCache entry;
        entry.locationId = 1;
        entry.storageClass = FLASH_STORAGE;
        entry.failureDomain = fdWith(fdByte).name.data;
        entry.flags = BlockServiceFlags::EMPTY;
        entry.availableBytes = 100'000'000'000ULL;  // 100GB (100x more)
        entry.capacityBytes = 100'000'000'000ULL;
        entry.blocks = 0;
        entry.hasFiles = false;
        cache[id] = entry;
        serviceToFd[id] = fdByte;
        id++;
    }

    p.update(cache);

    // Simulate low throughput: 1000 picks of 100 bytes each over 2 seconds.
    // Per-shard total = 100KB. Cluster-wide = 25.6MB in 2s = 12.8MB/s.
    // Max cluster throughput = 600MB/s * 5 drives = 3GB/s.
    // Ratio = 3GB/s / 12.8MB/s ≈ 234 → effectively uncapped (raw weight ratio is 100x).
    for (int i = 0; i < 1000; i++) {
        std::vector<BlockServiceId> out;
        p.pick(1, FLASH_STORAGE, 1, {}, out, 100);
    }

    _setCurrentTime(ternNow() + 2_sec);
    p.update(cache);

    std::unordered_map<uint8_t, int> fdPickCounts;
    const int NUM_ITERATIONS = 50000;
    for (int i = 0; i < NUM_ITERATIONS; i++) {
        std::vector<BlockServiceId> out;
        auto err = p.pick(1, FLASH_STORAGE, 1, {}, out);
        REQUIRE(err == TernError::NO_ERROR);
        REQUIRE(out.size() == 1);
        fdPickCounts[serviceToFd[out[0].u64]]++;
    }

    double avgExisting = 0;
    for (int fd = 1; fd <= EXISTING_FDS; fd++) avgExisting += fdPickCounts[fd];
    avgExisting /= EXISTING_FDS;

    double avgNew = fdPickCounts[EXISTING_FDS + 1];

    double lowLoadRatio = avgNew / avgExisting;
    CHECK(lowLoadRatio > 80.0);

    _setCurrentTime(TernTime());
}

TEST_CASE("picker clamping ratio varies with load") {
    // 2 FDs: FD1 = 1GB (10 drives), FD2 = 100GB (10 drives). 100x weight difference.
    // maxBlocksToPick=1 so stride clamping doesn't interfere.
    // At max load (initial): ratio=1.0 → all FDs clamped to minFdWeight → near-uniform.
    // At low load (after recalc): high ratio → capacity-proportional → FD2 gets ~100x more.
    _setCurrentTime(ternNow());

    auto p = makePicker(1, 0_sec, 600'000'000, 600'000'000);

    std::unordered_map<uint64_t, BlockServiceCache> cache;
    std::unordered_map<uint64_t, uint8_t> serviceToFd;

    uint64_t id = 1;
    for (int d = 0; d < 10; d++) {
        BlockServiceCache entry;
        entry.locationId = 1;
        entry.storageClass = FLASH_STORAGE;
        entry.failureDomain = fdWith(1).name.data;
        entry.flags = BlockServiceFlags::EMPTY;
        entry.availableBytes = 1'000'000'000ULL;   // 1GB per drive
        entry.capacityBytes = 10'000'000'000ULL;
        entry.blocks = 0;
        entry.hasFiles = false;
        cache[id] = entry;
        serviceToFd[id] = 1;
        id++;
    }
    for (int d = 0; d < 10; d++) {
        BlockServiceCache entry;
        entry.locationId = 1;
        entry.storageClass = FLASH_STORAGE;
        entry.failureDomain = fdWith(2).name.data;
        entry.flags = BlockServiceFlags::EMPTY;
        entry.availableBytes = 100'000'000'000ULL;  // 100GB per drive
        entry.capacityBytes = 100'000'000'000ULL;
        entry.blocks = 0;
        entry.hasFiles = false;
        cache[id] = entry;
        serviceToFd[id] = 2;
        id++;
    }

    p.update(cache);

    auto measureDistribution = [&]() {
        std::unordered_map<uint8_t, int> fdPicks;
        const int N = 50000;
        for (int i = 0; i < N; i++) {
            std::vector<BlockServiceId> out;
            auto err = p.pick(1, FLASH_STORAGE, 1, {}, out);
            REQUIRE(err == TernError::NO_ERROR);
            fdPicks[serviceToFd[out[0].u64]]++;
        }
        return std::make_pair(fdPicks[1], fdPicks[2]);
    };

    // Phase 1: max load (initial state) — distribution should be near-uniform
    auto [fd1High, fd2High] = measureDistribution();
    double highLoadRatio = (double)fd2High / fd1High;
    CHECK(highLoadRatio > 0.8);
    CHECK(highLoadRatio < 1.25);

    // Phase 2: simulate intermediate load targeting ratio ≈ 5.
    // Max cluster throughput = 600MB/s * 20 drives = 12GB/s.
    // For ratio=5: cluster throughput = 12GB/s / 5 = 2.4GB/s.
    // Per-shard over 2s = 2.4GB/s * 2 / 256 ≈ 18.75MB → 1000 picks of ~18750 bytes.
    // With ratio=5: ratioCap = minFdWeight(10GB) * 5 = 50GB.
    // FD1 stays at 10GB, FD2 clamped from 1TB to 50GB → pick ratio ≈ 5x.
    for (int i = 0; i < 1000; i++) {
        std::vector<BlockServiceId> out;
        p.pick(1, FLASH_STORAGE, 1, {}, out, 18750);
    }
    _setCurrentTime(ternNow() + 2_sec);
    p.update(cache);

    auto [fd1Mid, fd2Mid] = measureDistribution();
    double midLoadRatio = (double)fd2Mid / fd1Mid;
    // Intermediate load: FD2 gets more but not fully proportional (100x)
    CHECK(midLoadRatio > 3.0);
    CHECK(midLoadRatio < 8.0);

    _setCurrentTime(TernTime(0));
}

TEST_CASE("picker insufficient live FDs returns error fast") {
    // If blacklist leaves fewer live FDs than `needed`, we cannot satisfy the pick.
    auto p = makePicker();
    std::vector<BlockServiceInfoShort> catalog{
        bs(1, 1, FLASH_STORAGE, 1),
        bs(2, 1, FLASH_STORAGE, 2),
        bs(3, 1, FLASH_STORAGE, 3),
        bs(4, 1, FLASH_STORAGE, 4),
    };
    auto cache = makeCatalog(catalog);
    p.update(cache);

    // Blacklist two whole FDs → only 2 live FDs left, ask for 3.
    std::vector<BlacklistEntry> blacklist;
    BlacklistEntry b1; b1.failureDomain = fdWith(1); blacklist.push_back(b1);
    BlacklistEntry b2; b2.failureDomain = fdWith(2); blacklist.push_back(b2);

    std::vector<BlockServiceId> out;
    auto err = p.pick(1, FLASH_STORAGE, 3, blacklist, out);
    CHECK(err == TernError::COULD_NOT_PICK_BLOCK_SERVICES);
    CHECK(out.size() == 0);
}

TEST_CASE("picker slow path picks distinct FDs with service blacklist") {
    // Force the slow path: 3 FDs, each with two services. Blacklist one service
    // in the heaviest FD so stride invariant (maxFdWeight <= step) breaks.
    auto p = makePicker(3);

    std::unordered_map<uint64_t, BlockServiceCache> cache;
    auto mk = [](uint8_t fd, uint64_t avail) {
        BlockServiceCache e;
        e.locationId = 1;
        e.storageClass = FLASH_STORAGE;
        e.failureDomain = fdWith(fd).name.data;
        e.flags = BlockServiceFlags::EMPTY;
        e.availableBytes = avail;
        e.capacityBytes = avail * 10;
        e.blocks = 0;
        e.hasFiles = false;
        return e;
    };
    // FD1: svcs 10, 11 (both weight 100)  → FD total 200
    // FD2: svcs 20, 21 (both weight 100)  → FD total 200
    // FD3: svcs 30, 31 (both weight 100)  → FD total 200
    cache[10] = mk(1, 100); cache[11] = mk(1, 100);
    cache[20] = mk(2, 100); cache[21] = mk(2, 100);
    cache[30] = mk(3, 100); cache[31] = mk(3, 100);

    p.update(cache);
    p.resetStats();

    // Blacklist svc 30 from FD3 (weight 100): totalWeight=500, maxFdWeight=200,
    // step = 500/3 = 166 < 200 → stride invariant broken, slow path taken.
    std::vector<BlacklistEntry> bl;
    BlacklistEntry e; e.blockService = BlockServiceId(30); bl.push_back(e);

    const int ITER = 2000;
    for (int i = 0; i < ITER; i++) {
        std::vector<BlockServiceId> out;
        auto err = p.pick(1, FLASH_STORAGE, 3, bl, out);
        REQUIRE(err == TernError::NO_ERROR);
        REQUIRE(out.size() == 3);

        std::unordered_set<uint8_t> seenFds;
        for (const auto& id : out) {
            CHECK(id.u64 != 30);  // blacklisted
            uint8_t fd = (id.u64 < 20) ? 1 : (id.u64 < 30) ? 2 : 3;
            CHECK(seenFds.insert(fd).second);  // distinct FDs
        }
    }

    auto stats = p.getStats();
    bool sawRescale = false;
    for (const auto& ls : stats.locStorage) {
        if (ls.blacklistRepicks > 0) {
            sawRescale = true;
            // We took the slow path every iteration.
            CHECK(ls.blacklistRepicks == (uint64_t)ITER);
        }
    }
    CHECK(sawRescale);
}

TEST_CASE("picker drained lcKey resets throughput stats") {
    // if an lcKey loses all services, stale numDrives /
    // lastThroughputEstimate must be cleared so the spike path can't read them.
    _setCurrentTime(ternNow());
    auto p = makePicker(1, 0_sec, 600'000'000, 600'000'000);

    std::vector<BlockServiceInfoShort> catalog{
        bs(1, 1, FLASH_STORAGE, 1),
        bs(2, 1, FLASH_STORAGE, 2),
    };
    p.update(makeCatalog(catalog));

    // Warm the throughput estimate with a few picks.
    for (int i = 0; i < 10; i++) {
        std::vector<BlockServiceId> out;
        p.pick(1, FLASH_STORAGE, 1, {}, out, 1000);
    }
    _setCurrentTime(ternNow() + 2_sec);
    p.update(makeCatalog(catalog));

    // Now drop all services for this lcKey.
    p.update({});

    auto stats = p.getStats();
    for (const auto& ls : stats.locStorage) {
        if ((ls.key & 0xFF) == FLASH_STORAGE) {
            CHECK(ls.writableBlockServices == 0);
            CHECK(ls.writableFailureDomains == 0);
            CHECK(ls.throughputEstimate == 0);
        }
    }
    _setCurrentTime(TernTime(0));
}

TEST_CASE("picker stale service id from dropped lcKey does not corrupt weights") {
    // a service that existed under key K, was evicted,
    // but still appears in a blacklist must not subtract bogus weight from
    // the current fdWeights.
    _setCurrentTime(ternNow());
    auto p = makePicker(1, 0_sec, 600'000'000, 600'000'000);

    // Phase 1: service 99 lives in location=1.
    std::vector<BlockServiceInfoShort> catalog1{
        bs(50, 1, FLASH_STORAGE, 1),
        bs(51, 1, FLASH_STORAGE, 2),
        bs(99, 1, FLASH_STORAGE, 3),
    };
    p.update(makeCatalog(catalog1));

    // Trigger spike recalc so serviceToFdInfo for key=(1, FLASH) is rebuilt.
    for (int i = 0; i < 100; i++) {
        std::vector<BlockServiceId> out;
        p.pick(1, FLASH_STORAGE, 1, {}, out, 1'000'000);
    }
    _setCurrentTime(ternNow() + 2_sec);

    // Phase 2: drop service 99 (remove its FD entirely).
    std::vector<BlockServiceInfoShort> catalog2{
        bs(50, 1, FLASH_STORAGE, 1),
        bs(51, 1, FLASH_STORAGE, 2),
    };
    p.update(makeCatalog(catalog2));

    // Pick with a blacklist still referencing 99. Should succeed with no
    // corruption of the live (50, 51) FDs' weights.
    std::vector<BlacklistEntry> bl;
    BlacklistEntry e; e.blockService = BlockServiceId(99); bl.push_back(e);
    for (int i = 0; i < 50; i++) {
        std::vector<BlockServiceId> out;
        auto err = p.pick(1, FLASH_STORAGE, 1, bl, out);
        REQUIRE(err == TernError::NO_ERROR);
        REQUIRE(out.size() == 1);
        CHECK(out[0].u64 != 99);
    }
    _setCurrentTime(TernTime(0));
}
