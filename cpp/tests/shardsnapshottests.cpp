// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <filesystem>
#include <fstream>

// Exercise the server and writer loops without running the registry or reader threads.
#include "../shard/Shard.cpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

struct SnapshotTestDir {
    std::string path = "temp-shard-snapshot.XXXXXX";

    SnapshotTestDir() {
        if (mkdtemp(path.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }
    }

    ~SnapshotTestDir() {
        std::error_code ec;
        std::filesystem::remove_all(path, ec);
    }
};

TEST_CASE("Shard snapshots stay on the writer and are local to each replica") {
    bool avoidBeingLeader = true;
    bool noReplication = false;
    LocationId location = 0;
    SUBCASE("configured follower") {}
    SUBCASE("eligible follower") { avoidBeingLeader = false; }
    SUBCASE("leader") {
        avoidBeingLeader = false;
        noReplication = true;
    }
    SUBCASE("follower at a proxy location") { location = 1; }

    SnapshotTestDir dir;
    Logger logger(LogLevel::LOG_ERROR, STDERR_FILENO, false, false);
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "snapshot_test");
    ShardOptions options;
    options.shardId = 0;
    options.logsDBOptions.dbDir = dir.path;
    options.logsDBOptions.replicaId = 0;
    options.logsDBOptions.location = location;
    options.logsDBOptions.avoidBeingLeader = avoidBeingLeader;
    options.logsDBOptions.noReplication = noReplication;
    options.heartbeatEmissionInterval = 0;

    SharedRocksDB db(logger, xmon, dir.path + "/db", dir.path + "/statistics");
    auto registerCFs = [](SharedRocksDB& db) {
        db.registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());
        db.registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
        db.registerCFDescriptors(BlockServicesCacheDB::getColumnFamilyDescriptors());
    };
    registerCFs(db);
    rocksdb::Options dbOptions;
    dbOptions.create_if_missing = true;
    dbOptions.create_missing_column_families = true;
    dbOptions.manual_wal_flush = true;
    db.open(dbOptions);
    BlockServicesCacheDB cache(logger, xmon, db, 0_sec, 0, 0, 0);
    ShardDB shardDB(logger, xmon, options.shardId, location, DEFAULT_DEADLINE_INTERVAL, db, cache);
    LogsDB logsDB(logger, xmon, db, 0, shardDB.lastAppliedLogEntry(), noReplication, false, avoidBeingLeader);

    AddrsInfo loopback;
    loopback[0].ip = Ip({127, 0, 0, 1});
    ShardShared shared(options, db, cache, shardDB, logsDB, UDPSocketPair(env, loopback), logger, xmon);
    shared.isInitiated = true;
    shared.isBlockServiceCacheInitiated = true;
    ShardServer server(logger, xmon, shared);
    auto writer = std::make_unique<ShardWriter>(logger, xmon, shared);
    std::array<UDPSocketPair, 1> client{UDPSocketPair(env, loopback)};
    UDPSender sender;
    UDPReceiver<1> receiver;

    auto send = [&](const ShardReqMsg& req) {
        sender.prepareOutgoingMessage(env, client[0].addr(), 0, shared.sock().addr()[0],
            [&](BincodeBuf& buf) { req.pack(buf); });
        sender.sendMessages(env, client[0]);
    };
    auto receive = [&](uint64_t id, TernError expectedError) {
        REQUIRE(receiver.receiveMessages(env, client, -1, 1_sec));
        auto& responses = receiver.messages()[0];
        REQUIRE(responses.size() == 1);
        ShardRespMsg resp;
        resp.unpack(responses[0].buf);
        CHECK(resp.id == id);
        if (expectedError == TernError::NO_ERROR) {
            CHECK(resp.body.kind() == ShardMessageKind::SHARD_SNAPSHOT);
        } else {
            REQUIRE(resp.body.kind() == ShardMessageKind::ERROR);
            CHECK(resp.body.getError() == expectedError);
        }
    };

    REQUIRE_FALSE(readOnlyShardReq(ShardMessageKind::SHARD_SNAPSHOT));
    REQUIRE(db.db()->Put({}, "snapshot-test", "before").ok());
    std::vector<rocksdb::LiveFileMetaData> liveFiles;
    db.db()->GetLiveFilesMetaData(&liveFiles);
    REQUIRE(liveFiles.empty());
    auto snapshotPath = dir.path + "/snapshot-42";

    if (!noReplication) {
        // A normal write in the same batch must still be dropped by a follower.
        ShardReqMsg write;
        write.id = 100;
        write.body.setSetDirectoryInfo().id = ROOT_DIR_INODE_ID;
        send(write);
    }
    ShardReqMsg req;
    req.id = 101;
    req.body.setShardSnapshot().snapshotId = 42;
    send(req);
    server.step();
    CHECK_FALSE(std::filesystem::exists(snapshotPath));
    writer->step();
    CHECK(logsDB.isLeader() == noReplication);
    receive(req.id, TernError::NO_ERROR);
    REQUIRE(std::filesystem::is_directory(snapshotPath));
    db.db()->GetLiveFilesMetaData(&liveFiles);
    CHECK(liveFiles.empty());
    bool hasWAL = false;
    for (const auto& file : std::filesystem::directory_iterator(snapshotPath)) {
        hasWAL |= file.path().extension() == ".log";
    }
    CHECK(hasWAL);

    // Retry after further WAL writes; recovery must still see the original checkpoint.
    REQUIRE(db.db()->Put({}, "snapshot-test", "after").ok());
    req.id = 102;
    send(req);
    server.step();
    writer->step();
    receive(req.id, TernError::NO_ERROR);
    {
        SharedRocksDB checkpoint(logger, xmon, snapshotPath, dir.path + "/checkpoint-statistics");
        registerCFs(checkpoint);
        checkpoint.openForReadOnly(dbOptions);
        std::string value;
        REQUIRE(checkpoint.db()->Get({}, "snapshot-test", &value).ok());
        CHECK(value == "before");
    }

    std::ofstream(dir.path + "/snapshot-43") << "not a directory";
    req.id = 103;
    req.body.setShardSnapshot().snapshotId = 43;
    send(req);
    server.step();
    writer->step();
    receive(req.id, TernError::CANNOT_CREATE_DB_SNAPSHOT);
}
