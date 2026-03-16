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


TEST_CASE("BlockServicesCacheDB initializes empty") {
    Logger logger(LogLevel::LOG_ERROR, /*fd*/1, /*syslog*/false, /*usr2ToDebug*/false);
    TempBlockServicesCacheDB tdb(LogLevel::LOG_ERROR);
    tdb.open();
    auto* cf = tdb.sharedDB->createCF({"blockServicesCache", {}});

    BlockServicesCacheDB db(logger, tdb.xmon, *tdb.sharedDB);
    auto cache = db.getCache();
    CHECK(cache.blockServices.size() == 0);
}

TEST_CASE("BlockServicesCacheDB persists blockServices metadata on update") {
    Logger logger(LogLevel::LOG_ERROR, /*fd*/1, /*syslog*/false, /*usr2ToDebug*/false);
    TempBlockServicesCacheDB tdb(LogLevel::LOG_ERROR);
    tdb.open();
    auto* cf = tdb.sharedDB->createCF({"blockServicesCache", {}});

    uint64_t idBase = 999;

    // Update with block service entries
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

    {
        BlockServicesCacheDB db(logger, tdb.xmon, *tdb.sharedDB);
        db.updateCache(blockSvcs);
    }

    // Re-open and verify block service metadata persisted
    {
        BlockServicesCacheDB db(logger, tdb.xmon, *tdb.sharedDB);
        auto cache = db.getCache();

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
