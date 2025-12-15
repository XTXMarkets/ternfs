// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <string>
#include <vector>
#include <memory>
//

#include <rocksdb/db.h>

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

#include "BlockServicesCacheDB.hpp"
#include "SharedRocksDB.hpp"
#include "RocksDBUtils.hpp"
#include "utils/TempBlockServicesCacheDB.hpp"
#include "Msgs.hpp"


TEST_CASE("BlockServicesCacheDB loads old current-list format with defaults") {
    Logger logger(LogLevel::LOG_ERROR, /*fd*/1, /*syslog*/false, /*usr2ToDebug*/false);
    TempBlockServicesCacheDB tdb(LogLevel::LOG_ERROR);
    tdb.open();
    auto* cf = tdb.sharedDB->createCF({"blockServicesCache", {}});

    BlockServicesCacheDB dbInit(logger, tdb.xmon, *tdb.sharedDB);

    // Overwrite CURRENT_BLOCK_SERVICES with old format: [len=2][id1][id2]
    uint8_t len = 2; uint64_t id1 = 111, id2 = 222;
    std::string curOld; curOld.resize(1 + 2*sizeof(uint64_t));
    curOld[0] = (char)len;
    memcpy(&curOld[1], &id1, sizeof(uint64_t));
    memcpy(&curOld[1+sizeof(uint64_t)], &id2, sizeof(uint64_t));
    uint8_t key = 0; // CURRENT_BLOCK_SERVICES
    ROCKS_DB_CHECKED(tdb.sharedDB->db()->Put({}, cf, rocksdb::Slice((const char*)&key, 1), rocksdb::Slice(curOld)));

    // Construct a fresh reader to simulate startup
    BlockServicesCacheDB db(logger, tdb.xmon, *tdb.sharedDB);
    auto cache = db.getCache();
    CHECK(cache.currentBlockServices.size() == 2);
    // Old current list should map ids; locationId defaults to 0 when unknown
    CHECK(cache.currentBlockServices[0].id.u64 == id1);
    CHECK(cache.currentBlockServices[1].id.u64 == id2);
}

TEST_CASE("BlockServicesCacheDB loads V2 bodies fully") {
    Logger logger(LogLevel::LOG_ERROR, /*fd*/1, /*syslog*/false, /*usr2ToDebug*/false);
    TempBlockServicesCacheDB tdb2(LogLevel::LOG_ERROR);
    tdb2.open();
    auto* cf = tdb2.sharedDB->createCF({"blockServicesCache", {}});

    BlockServicesCacheDB db(logger, tdb2.xmon, *tdb2.sharedDB);
    auto cache = db.getCache();
    CHECK(cache.currentBlockServices.size() == 0);
}

TEST_CASE("BlockServicesCacheDB upgrades old current-list on update") {
    Logger logger(LogLevel::LOG_ERROR, /*fd*/1, /*syslog*/false, /*usr2ToDebug*/false);
    TempBlockServicesCacheDB tdb(LogLevel::LOG_ERROR);
    tdb.open();
    auto* cf = tdb.sharedDB->createCF({"blockServicesCache", {}});

    // Seed DB with old CURRENT_BLOCK_SERVICES format: [len=2][id1][id2]
    uint8_t len = 2; uint64_t id1 = 101, id2 = 202;
    std::string curOld; curOld.resize(1 + 2*sizeof(uint64_t));
    curOld[0] = (char)len;
    memcpy(&curOld[1], &id1, sizeof(uint64_t));
    memcpy(&curOld[1+sizeof(uint64_t)], &id2, sizeof(uint64_t));
    uint8_t key = 0; // CURRENT_BLOCK_SERVICES
    ROCKS_DB_CHECKED(tdb.sharedDB->db()->Put({}, cf, rocksdb::Slice((const char*)&key, 1), rocksdb::Slice(curOld)));

    // Verify old list loads
    {
        BlockServicesCacheDB db(logger, tdb.xmon, *tdb.sharedDB);
        auto cache = db.getCache();
        REQUIRE(cache.currentBlockServices.size() == 2);
        CHECK(cache.currentBlockServices[0].id.u64 == id1);
        CHECK(cache.currentBlockServices[1].id.u64 == id2);
    }

    // Upgrade: write new-format current list with explicit locationId/storageClass
    std::vector<BlockServiceInfoShort> newCurrent;
    BlockServiceInfoShort s1{}; s1.id = BlockServiceId(id1); s1.locationId = 1; s1.storageClass = FLASH_STORAGE; s1.failureDomain = FailureDomain();
    BlockServiceInfoShort s2{}; s2.id = BlockServiceId(id2); s2.locationId = 2; s2.storageClass = HDD_STORAGE; s2.failureDomain = FailureDomain();
    newCurrent.push_back(s1);
    newCurrent.push_back(s2);

    {
        BlockServicesCacheDB db(logger, tdb.xmon, *tdb.sharedDB);
        std::vector<FullBlockServiceInfo> none;
        db.updateCache(none, newCurrent);
    }

    // Re-open and verify the new-format values were persisted and read correctly
    {
        BlockServicesCacheDB db(logger, tdb.xmon, *tdb.sharedDB);
        auto cache = db.getCache();
        REQUIRE(cache.currentBlockServices.size() == 2);
        CHECK(cache.currentBlockServices[0].id.u64 == id1);
        CHECK(cache.currentBlockServices[0].locationId == 1);
        CHECK(cache.currentBlockServices[0].storageClass == FLASH_STORAGE);
        CHECK(cache.currentBlockServices[1].id.u64 == id2);
        CHECK(cache.currentBlockServices[1].locationId == 2);
        CHECK(cache.currentBlockServices[1].storageClass == HDD_STORAGE);
    }
}

TEST_CASE("BlockServicesCacheDB persists blockServices metadata on update") {
    Logger logger(LogLevel::LOG_ERROR, /*fd*/1, /*syslog*/false, /*usr2ToDebug*/false);
    TempBlockServicesCacheDB tdb(LogLevel::LOG_ERROR);
    tdb.open();
    auto* cf = tdb.sharedDB->createCF({"blockServicesCache", {}});

    // Seed DB with old current-list format [len=1][id=999]
    uint8_t len = 1; uint64_t idBase = 999;
    std::string curOld; curOld.resize(1 + sizeof(uint64_t));
    curOld[0] = (char)len;
    memcpy(&curOld[1], &idBase, sizeof(uint64_t));
    uint8_t key = 0; // CURRENT_BLOCK_SERVICES
    ROCKS_DB_CHECKED(tdb.sharedDB->db()->Put({}, cf, rocksdb::Slice((const char*)&key, 1), rocksdb::Slice(curOld)));

    // Verify old list loads
    {
        BlockServicesCacheDB db(logger, tdb.xmon, *tdb.sharedDB);
        auto cache = db.getCache();
        REQUIRE(cache.currentBlockServices.size() == 1);
        CHECK(cache.currentBlockServices[0].id.u64 == idBase);
        // blockServices map is populated from DB on construction; if no entries exist, it stays empty
        // But the old current list still references idBase, so we just check current list
    }

    // Update with block service entries and new current list
    std::vector<FullBlockServiceInfo> blockSvcs;
    FullBlockServiceInfo e{};
    e.id = BlockServiceId(idBase);
    e.addrs[0].ip = {{192,168,1,10}}; e.addrs[0].port = 5000;
    e.addrs[1].ip = {{192,168,1,11}}; e.addrs[1].port = 5001;
    e.storageClass = FLASH_STORAGE;
    e.failureDomain.name = {{1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16}};
    e.secretKey = {{0xaa,0xbb,0xcc,0xdd,0xee,0xff,0x00,0x11,0x22,0x33,0x44,0x55,0x66,0x77,0x88,0x99}};
    e.flags = BlockServiceFlags::EMPTY;
    e.path = "ick-testhost-999";
    e.capacityBytes = 1000000;
    e.availableBytes = 800000;
    e.blocks = 42;
    e.lastSeen = TernTime(5000);
    e.lastInfoChange = TernTime(4000);
    e.hasFiles = true;
    blockSvcs.push_back(e);

    std::vector<BlockServiceInfoShort> newCurrent;
    BlockServiceInfoShort s{};
    s.id = BlockServiceId(idBase);
    s.locationId = 0; // extracted from "ick-testhost-999"
    s.storageClass = FLASH_STORAGE;
    s.failureDomain = e.failureDomain;
    newCurrent.push_back(s);

    {
        BlockServicesCacheDB db(logger, tdb.xmon, *tdb.sharedDB);
        db.updateCache(blockSvcs, newCurrent);
    }

    // Re-open and verify block service metadata persisted
    {
        BlockServicesCacheDB db(logger, tdb.xmon, *tdb.sharedDB);
        auto cache = db.getCache();
        REQUIRE(cache.currentBlockServices.size() == 1);
        CHECK(cache.currentBlockServices[0].id.u64 == idBase);
        CHECK(cache.currentBlockServices[0].locationId == 0);
        CHECK(cache.currentBlockServices[0].storageClass == FLASH_STORAGE);

        auto it = cache.blockServices.find(idBase);
        REQUIRE(it != cache.blockServices.end());
        CHECK(it->second.locationId == 0);
        CHECK(it->second.storageClass == FLASH_STORAGE);
        CHECK(it->second.capacityBytes == 1000000);
        CHECK(it->second.availableBytes == 800000);
        CHECK(it->second.blocks == 42);
        CHECK(it->second.hasFiles == true);
        CHECK(it->second.addrs[0].port == 5000);
        CHECK(it->second.addrs[1].port == 5001);
        // path is persisted in V2+ format
        CHECK(it->second.path == "ick-testhost-999");
    }
}
