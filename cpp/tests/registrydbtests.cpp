// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <iostream>
#include <ostream>
#include <resolv.h>
#include <unordered_set>

#include "Bincode.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "Time.hpp"
#include "utils/TempRegistryDB.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

REGISTER_EXCEPTION_TRANSLATOR(AbstractException& ex) {
    std::stringstream ss;
    ss << std::endl << ex.what() << std::endl;
    return doctest::String(ss.str().c_str());
}

TEST_CASE("CreateReopen") {
    RegistryOptions options;
    TempRegistryDB db(LogLevel::LOG_ERROR);
    
    db.open(options);
    db.close();
    db.open(options);
}

TEST_CASE("Initialization") {
    RegistryOptions options;
    TempRegistryDB db(LogLevel::LOG_ERROR);
    
    db.open(options);
    
    std::vector<LocationInfo> locations;
    db->locations(locations);
    REQUIRE(locations.size() == 1);
    CHECK(locations[0].id == DEFAULT_LOCATION);
    CHECK(locations[0].name == "default");
    
    std::vector<FullRegistryInfo> registries;
    db->registries(registries);
    CHECK(registries.size() == LogsDB::REPLICA_COUNT);
    
    std::vector<FullShardInfo> shards;
    db->shards(shards);
    CHECK(!shards.empty());
    CHECK(shards.size() == LogsDB::REPLICA_COUNT * ShardId::SHARD_COUNT);
    
    std::vector<CdcInfo> cdcs;
    db->cdcs(cdcs);
    CHECK(!cdcs.empty());
    CHECK(cdcs.size() == LogsDB::REPLICA_COUNT);
}

TEST_CASE("CreateLocation") {
    RegistryOptions options;
    TempRegistryDB db(LogLevel::LOG_ERROR);
    db.open(options);
    
    std::vector<LogsDBLogEntry> logEntries;
    std::vector<RegistryDBWriteResult> writeResults;
    
    auto& entry = logEntries.emplace_back();
    entry.idx = db->lastAppliedLogEntry() + 1;
    RegistryReqContainer reqContainer;
    auto& createReq = reqContainer.setCreateLocation();
    createReq.id = DEFAULT_LOCATION;
    createReq.name = "test-location";

    SUBCASE("failureOnExists") {
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();
    
        db->processLogEntries(logEntries, writeResults);
    
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::LOCATION_EXISTS);
        CHECK(writeResults[0].kind == RegistryMessageKind::CREATE_LOCATION);
    }

    SUBCASE("createSucceeded") {
        createReq.id = LocationId(42);
        
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::CREATE_LOCATION);
        
        std::vector<LocationInfo> locations;
        db->locations(locations);
        REQUIRE(locations.size() == 2);
        
        bool found = false;
        for (const auto& loc : locations) {
            if (loc.id == LocationId(42)) {
                CHECK(loc.name == "test-location");
                found = true;
                break;
            }
        }
        CHECK(found);
        std::vector<FullRegistryInfo> registries;
        db->registries(registries);
        CHECK(registries.size() == LogsDB::REPLICA_COUNT);
        
        std::vector<FullShardInfo> shards;
        db->shards(shards);
        CHECK(!shards.empty());
        CHECK(shards.size() == LogsDB::REPLICA_COUNT * ShardId::SHARD_COUNT * locations.size());
        
        std::vector<CdcInfo> cdcs;
        db->cdcs(cdcs);
        CHECK(!cdcs.empty());
        CHECK(cdcs.size() == LogsDB::REPLICA_COUNT * locations.size());
    }
}

TEST_CASE("RenameLocation") {
    RegistryOptions options;
    TempRegistryDB db(LogLevel::LOG_ERROR);
    db.open(options);
    
    std::vector<LogsDBLogEntry> logEntries;
    std::vector<RegistryDBWriteResult> writeResults;

    auto& entry = logEntries.emplace_back();
    entry.idx = db->lastAppliedLogEntry() + 1;
    
    RegistryReqContainer reqContainer;
    auto& renameReq = reqContainer.setRenameLocation();
    renameReq.id = LocationId(10);
    renameReq.name = "new-name";
    SUBCASE("failNotFound") {        
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();

        db->processLogEntries(logEntries, writeResults);
    
        REQUIRE(writeResults.size() == logEntries.size());
        CHECK(writeResults[0].err == TernError::LOCATION_NOT_FOUND);
        CHECK(writeResults[0].kind == RegistryMessageKind::RENAME_LOCATION);
    }
    
    SUBCASE("renameSucceeded") {        
        renameReq.id = DEFAULT_LOCATION;
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();

        db->processLogEntries(logEntries, writeResults);
    
        REQUIRE(writeResults.size() == logEntries.size());
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::RENAME_LOCATION);
        std::vector<LocationInfo> locations;
        db->locations(locations);
        
        bool found = false;
        for (const auto& loc : locations) {
            if (loc.id == LocationId(DEFAULT_LOCATION)) {
                CHECK(loc.name == "new-name");
                found = true;
                break;
            }
        }
        CHECK(found);
    }
}

TEST_CASE("RegisterShard") {
    RegistryOptions options;
    TempRegistryDB db(LogLevel::LOG_ERROR);
    db.open(options);
    
    std::vector<LogsDBLogEntry> logEntries;
    std::vector<RegistryDBWriteResult> writeResults;
    AddrsInfo addrsInfo;
    parseIpv4Addr("1.2.3.4:8080",addrsInfo.addrs[0]);
    parseIpv4Addr("5.6.7.8:8081",addrsInfo.addrs[1]);
    auto& entry = logEntries.emplace_back();
    entry.idx = db->lastAppliedLogEntry() + 1;
    
    RegistryReqContainer reqContainer;
    auto& registerReq = reqContainer.setRegisterShard();
    registerReq.location = LocationId(10);
    registerReq.shrid = ShardReplicaId(ShardId(1), ReplicaId(0));
    registerReq.isLeader = true;
    registerReq.addrs = addrsInfo;
    SUBCASE("failInvalidReplica") {
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();

        db->processLogEntries(logEntries, writeResults);
    
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::INVALID_REPLICA);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_SHARD);
    }
    SUBCASE("registerNew") {
        registerReq.location = DEFAULT_LOCATION;
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_SHARD);

        std::vector<FullShardInfo> shards;
        db->shards(shards);
        
        bool found = false;
        for (const auto& shard : shards) {
            if (shard.locationId == LocationId(DEFAULT_LOCATION) && 
                shard.id.replicaId() == ReplicaId(0) && 
                shard.id.shardId() == ShardId(1)) {
                CHECK(shard.isLeader == true);
                CHECK(shard.addrs == addrsInfo);
                found = true;
                break;
            }
        }
        CHECK(found);
    }
    SUBCASE("updateFailLeaderPreempted") {
        registerReq.location = DEFAULT_LOCATION;
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_SHARD);
        db.close();
        options.enforceStableLeader = true;
        db.open(options);
        registerReq.isLeader = false;
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::LEADER_PREEMPTED);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_SHARD);

        db.close();
        options.enforceStableLeader = false;
        db.open(options);
        registerReq.isLeader = false;
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_SHARD);
    }
    SUBCASE("updateFailDifferentAddressInfo") {
        registerReq.location = DEFAULT_LOCATION;
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_SHARD);
        db.close();
        options.enforceStableIp = true;
        db.open(options);
        
        parseIpv4Addr("1.2.3.4:8080",registerReq.addrs.addrs[0]);
        parseIpv4Addr("5.6.7.9:8081",registerReq.addrs.addrs[1]);        
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_SHARD);

        registerReq.addrs.addrs[0].clear();        
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_SHARD);

        parseIpv4Addr("5.4.3.2:8080",registerReq.addrs.addrs[0]);
        parseIpv4Addr("9.8.7.6:8081",registerReq.addrs.addrs[1]);
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::DIFFERENT_ADDRS_INFO);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_SHARD);

        db.close();
        options.enforceStableIp = false;
        db.open(options);
        registerReq.isLeader = false;
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_SHARD);
    }
}

TEST_CASE("RegisterCDC") {
    RegistryOptions options;
    TempRegistryDB db(LogLevel::LOG_ERROR);
    db.open(options);
    
    std::vector<LogsDBLogEntry> logEntries;
    std::vector<RegistryDBWriteResult> writeResults;
    AddrsInfo addrsInfo;
    parseIpv4Addr("1.2.3.4:8080",addrsInfo.addrs[0]);
    parseIpv4Addr("5.6.7.8:8081",addrsInfo.addrs[1]);
    auto& entry = logEntries.emplace_back();
    entry.idx = db->lastAppliedLogEntry() + 1;
    
    RegistryReqContainer reqContainer;
    auto& registerReq = reqContainer.setRegisterCdc();
    registerReq.location = LocationId(10);
    registerReq.replica = ReplicaId(0);
    registerReq.isLeader = true;
    registerReq.addrs = addrsInfo;
    SUBCASE("failInvalidReplica") {
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();

        db->processLogEntries(logEntries, writeResults);
    
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::INVALID_REPLICA);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_CDC);
    }
    SUBCASE("registerNew") {
        registerReq.location = DEFAULT_LOCATION;
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_CDC);

        std::vector<CdcInfo> cdcs;
        db->cdcs(cdcs);
        
        bool found = false;
        for (const auto& cdc : cdcs) {
            if (cdc.locationId == LocationId(DEFAULT_LOCATION) && 
                cdc.replicaId == ReplicaId(0)) {
                CHECK(cdc.isLeader == true);
                CHECK(cdc.addrs == addrsInfo);
                found = true;
                break;
            }
        }
        CHECK(found);
    }
    SUBCASE("updateFailLeaderPreempted") {
        registerReq.location = DEFAULT_LOCATION;
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_CDC);
        db.close();
        options.enforceStableLeader = true;
        db.open(options);
        registerReq.isLeader = false;
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::LEADER_PREEMPTED);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_CDC);

        db.close();
        options.enforceStableLeader = false;
        db.open(options);
        registerReq.isLeader = false;
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_CDC);
    }
    SUBCASE("updateFailDifferentAddressInfo") {
        registerReq.location = DEFAULT_LOCATION;
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_CDC);
        db.close();
        options.enforceStableIp = true;
        db.open(options);
        
        parseIpv4Addr("1.2.3.4:8080",registerReq.addrs.addrs[0]);
        parseIpv4Addr("5.6.7.9:8081",registerReq.addrs.addrs[1]);        
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_CDC);

        registerReq.addrs.addrs[0].clear();        
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_CDC);

        parseIpv4Addr("5.4.3.2:8080",registerReq.addrs.addrs[0]);
        parseIpv4Addr("9.8.7.6:8081",registerReq.addrs.addrs[1]);
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::DIFFERENT_ADDRS_INFO);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_CDC);

        db.close();
        options.enforceStableIp = false;
        db.open(options);
        registerReq.isLeader = false;
        ++entry.idx;
        {
            BincodeBuf buf((char*)entry.value.data(), entry.value.size());
            buf.packScalar<uint64_t>(ternNow().ns);
            reqContainer.pack(buf);
            buf.ensureFinished();
        }
        writeResults.clear();
        
        db->processLogEntries(logEntries, writeResults);
        
        REQUIRE(writeResults.size() == 1);
        CHECK(writeResults[0].err == TernError::NO_ERROR);
        CHECK(writeResults[0].kind == RegistryMessageKind::REGISTER_CDC);
    }
}

TEST_CASE("RegisterBlockServices") {
    RegistryOptions options;
    TempRegistryDB db(LogLevel::LOG_ERROR);
    db.open(options);
    
    std::vector<LogsDBLogEntry> logEntries;
    std::vector<RegistryDBWriteResult> writeResults;
    
    RegisterBlockServiceInfo info;
    auto now = ternNow();
    {
        auto& entry = logEntries.emplace_back();
        entry.idx = db->lastAppliedLogEntry() + 1;
        
        RegistryReqContainer reqContainer;
        auto& registerReq = reqContainer.setRegisterBlockServices();
        info.id = BlockServiceId(100);
        info.locationId = DEFAULT_LOCATION;
        info.storageClass = 1;
        info.failureDomain.name = "test-fd";
        info.secretKey = "test-secret-key";
        info.capacityBytes = 1000000;
        info.availableBytes = 500000;
        info.blocks = 100;
        info.path = "/test/path";
        
        parseIpv4Addr("1.2.3.4:8080",info.addrs.addrs[0]);
        parseIpv4Addr("5.6.7.8:8081",info.addrs.addrs[1]);

        registerReq.blockServices.els.emplace_back(info);
        
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(now.ns);
        reqContainer.pack(buf);
        buf.ensureFinished();
    }
    
    db->processLogEntries(logEntries, writeResults);
    
    REQUIRE(writeResults.size() == 1);
    CHECK(writeResults[0].err == TernError::NO_ERROR);
   
    SUBCASE("registerNew") {
        std::vector<FullBlockServiceInfo> services;
        db->blockServices(services);
    
        REQUIRE(services.size() >= 1);
        bool found = false;
        for (const auto& service : services) {
            if (service.id == info.id) {
                CHECK(service.locationId == info.locationId);
                CHECK(service.storageClass == info.storageClass);
                CHECK(service.capacityBytes == info.capacityBytes);
                CHECK(service.availableBytes == info.availableBytes);
                CHECK(service.blocks == info.blocks);
                CHECK(service.path == info.path);
                CHECK(service.lastInfoChange == now);
                CHECK(service.lastSeen == now);
                CHECK(service.firstSeen == now);
                CHECK(service.secretKey == info.secretKey);
                CHECK(service.flags == info.flags);
                found = true;
                break;
            }
        }
        CHECK(found);
    }
}

TEST_CASE("ShardBlockServices") {
    RegistryOptions options;
    TempRegistryDB db(LogLevel::LOG_ERROR);
    db.open(options);
    
    std::vector<LogsDBLogEntry> logEntries;
    std::vector<RegistryDBWriteResult> writeResults;
    
    {
        auto& entry = logEntries.emplace_back();
        entry.idx = db->lastAppliedLogEntry() + 1;
        
        RegistryReqContainer reqContainer;
        auto& registerReq = reqContainer.setRegisterBlockServices();
        
        auto& service = registerReq.blockServices.els.emplace_back();
        service.id = BlockServiceId(200);
        service.locationId = DEFAULT_LOCATION;
        service.storageClass = 1;
        service.failureDomain.name = "test-fd";
        service.secretKey = "test-key";
        service.capacityBytes = 1000000;
        service.availableBytes = 500000; 
        service.blocks = 100;
        service.path = "/test/path";
        
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();
    }
    
    db->processLogEntries(logEntries, writeResults);
    
    REQUIRE(writeResults.size() == 1);
    CHECK(writeResults[0].err == TernError::NO_ERROR);
    

    std::vector<BlockServiceInfoShort> shardServices;
    db->shardBlockServices(ShardId(0), shardServices);
    
    CHECK(!shardServices.empty());
    
    bool found = false;
    for (const auto& service : shardServices) {
        if (service.id == 200) {
            CHECK(service.locationId == DEFAULT_LOCATION);
            CHECK(service.storageClass == 1);
            found = true;
            break;
        }
    }
    CHECK(found);
}

TEST_CASE("DecommissionBlockService") {
    RegistryOptions options;
    TempRegistryDB db(LogLevel::LOG_ERROR);
    db.open(options);
    
    std::vector<LogsDBLogEntry> logEntries;
    std::vector<RegistryDBWriteResult> writeResults;
    
    // First register a block service
    {
        auto& entry = logEntries.emplace_back();
        entry.idx = db->lastAppliedLogEntry() + 1;
        
        RegistryReqContainer reqContainer;
        auto& registerReq = reqContainer.setRegisterBlockServices();
        
        auto& service = registerReq.blockServices.els.emplace_back();
        service.id = BlockServiceId(300);
        service.locationId = DEFAULT_LOCATION;
        service.storageClass = 1;

        service.failureDomain.name = "test-fd";
        service.secretKey = "test-key";
        service.capacityBytes = 1000000;
        service.availableBytes = 500000;
        service.blocks = 100;
        service.path = "/test/path";
        
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();
    }
    
    {
        auto& entry = logEntries.emplace_back();
        entry.idx = db->lastAppliedLogEntry() + 2;
        
        RegistryReqContainer reqContainer;
        auto& decommissionReq = reqContainer.setDecommissionBlockService();
        decommissionReq.id = BlockServiceId(300);
        
        entry.value.resize(sizeof(uint64_t) + reqContainer.packedSize());
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        buf.packScalar<uint64_t>(ternNow().ns);
        reqContainer.pack(buf);
        buf.ensureFinished();
    }
    
    db->processLogEntries(logEntries, writeResults);
    
    REQUIRE(writeResults.size() == 2);
    CHECK(writeResults[0].err == TernError::NO_ERROR);
    CHECK(writeResults[1].err == TernError::NO_ERROR);
    
    std::vector<FullBlockServiceInfo> services;
    db->blockServices(services);
    
    bool found = false;
    for (const auto& service : services) {
        if (service.id == BlockServiceId(300)) {
            CHECK(service.flags == BlockServiceFlags::DECOMMISSIONED);
            found = true;
            break;
        }
    }
    CHECK(found);
}


