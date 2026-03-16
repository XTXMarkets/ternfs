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
    BlockServicePicker p(testLogger, testXmon, 15, 0_sec);
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
    BlockServicePicker p(testLogger, testXmon, 15, 0_sec);
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
    BlockServicePicker p(testLogger, testXmon, 15, 0_sec);
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
    BlockServicePicker p(testLogger, testXmon, 15, 0_sec);
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
    BlockServicePicker p(testLogger, testXmon, 1, 0_sec);

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
    BlockServicePicker p(testLogger, testXmon, 15, 0_sec);

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
    BlockServicePicker p(testLogger, testXmon, 1, 0_sec);

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
    BlockServicePicker p(testLogger, testXmon, 15, 0_sec);

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

TEST_CASE("picker weight scaling for dominant failure domain") {
    BlockServicePicker p(testLogger, testXmon, 15, 0_sec);
    std::vector<BlockServiceInfoShort> catalog;

    // Scenario:
    // FD 1: Dominant (10000)
    // FD 2: Large (5000)
    // FD 3-52: Small (100 each) - 50 FDs
    // Total FDs: 52
    // We use more small FDs to make the scaling less extreme (p will be larger),
    // allowing us to distinguish between weights more reliably.

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

    // Calculate expected scaling
    // We replicate the logic to find expected p
    double p_exp = 1.0;
    double min_p = 0.0, max_p = 1.0;

    for(int i=0; i<100; ++i) {
        p_exp = (min_p + max_p) / 2.0;
        double w1 = std::pow(10000.0, p_exp);
        double w2 = std::pow(5000.0, p_exp);
        double w_small = std::pow(100.0, p_exp);
        double total = w1 + w2 + 50 * w_small;

        if (w1 * 15 <= total) {
            min_p = p_exp; // Valid, try higher
        } else {
            max_p = p_exp; // Invalid, try lower
        }
    }
    // Use the best valid p found (min_p is the highest valid p)
    p_exp = min_p;

    double w1 = std::pow(10000.0, p_exp);
    double w2 = std::pow(5000.0, p_exp);
    double w_small = std::pow(100.0, p_exp);
    double total = w1 + w2 + 50 * w_small;

    double prob1 = w1 / total;
    double prob2 = w2 / total;
    double prob_small = w_small / total;

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

    // Verify ratios
    // FD 1
    double actualProb1 = (double)picks[1] / NUM_ITERATIONS;
    CHECK(actualProb1 > prob1 * 0.85);
    CHECK(actualProb1 < prob1 * 1.15);

    // FD 2
    double actualProb2 = (double)picks[2] / NUM_ITERATIONS;
    CHECK(actualProb2 > prob2 * 0.85);
    CHECK(actualProb2 < prob2 * 1.15);

    // Small FDs
    double totalSmallPicks = 0;
    for (int i = 3; i <= 52; ++i) {
        totalSmallPicks += picks[i];
    }
    double actualProbSmallAvg = (totalSmallPicks / 50.0) / NUM_ITERATIONS;
    CHECK(actualProbSmallAvg > prob_small * 0.85);
    CHECK(actualProbSmallAvg < prob_small * 1.15);

    // Verify relative ratios are preserved (FD1 > FD2 > Small)
    CHECK(picks[1] > picks[2]);
    CHECK(picks[2] > (totalSmallPicks / 50.0));
}

TEST_CASE("picker with blacklisted FD after scaling") {
    // Regression test for applyScaling rounding error.
    // With 18 FDs and needed=14, blacklisting 1 FD should still allow picking.
    // The scaling invariant maxFdWeight*maxBlocksToPick <= totalWeight guarantees
    // this, but integer rounding in applyScaling can violate the invariant.
    //
    // We sweep weight ratios between "large" and "small" FD groups to find
    // configurations where the rounding error triggers a false pick failure.

    const int NUM_FDS = 18;
    const int LARGE_FDS = 8;
    const int SMALL_FDS = 10;
    const int SVCS_PER_FD = 10;
    const int NEEDED = 14;

    // Try many weight ratios to catch the rounding edge case
    for (int ratio100 = 150; ratio100 <= 500; ratio100 += 5) {
        double weightRatio = ratio100 / 100.0;
        uint64_t smallWeight = 1000000;
        uint64_t largeWeight = static_cast<uint64_t>(smallWeight * weightRatio);

        BlockServicePicker p(testLogger, testXmon, 15, 0_sec);
        std::unordered_map<uint64_t, BlockServiceCache> cache;

        uint64_t id = 1;
        for (int fd = 1; fd <= NUM_FDS; fd++) {
            uint64_t svcWeight = (fd <= LARGE_FDS) ? largeWeight : smallWeight;
            for (int s = 0; s < SVCS_PER_FD; s++) {
                BlockServiceCache entry;
                entry.locationId = 0;
                entry.storageClass = FLASH_STORAGE;
                entry.failureDomain = fdWith(fd).name.data;
                entry.flags = BlockServiceFlags::EMPTY;
                entry.availableBytes = svcWeight;
                entry.capacityBytes = svcWeight * 10;
                entry.blocks = 0;
                entry.hasFiles = false;
                cache[id] = entry;
                id++;
            }
        }

        p.update(cache);

        // Try blacklisting each large FD and picking needed=14
        for (int blFd = 1; blFd <= LARGE_FDS; blFd++) {
            std::vector<BlacklistEntry> blacklist;
            BlacklistEntry bl;
            bl.failureDomain = fdWith(blFd);
            blacklist.push_back(bl);

            std::vector<BlockServiceId> out;
            auto err = p.pick(0, FLASH_STORAGE, NEEDED, blacklist, out);
            CHECK_MESSAGE(err == TernError::NO_ERROR,
                "pick failed with ratio=", weightRatio, " blacklisted FD=", blFd);
        }
    }
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
        BlockServicePicker p(testLogger, testXmon, 15, 0_sec);
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
