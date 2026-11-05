// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <filesystem>

// Exercise the request loop with real LogsDB/CDCDB and loopback UDP sockets,
// without registry threads or shard processes.
#include "../cdc/CDC.cpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

REGISTER_EXCEPTION_TRANSLATOR(AbstractException& ex) {
    return doctest::String(ex.what());
}

namespace {

struct TestDB {
    std::string dir = "temp-cdc-server.XXXXXX";
    Logger logger{LogLevel::LOG_ERROR, STDERR_FILENO, false, false};
    std::shared_ptr<XmonAgent> xmon;
    Env env{logger, xmon, "cdc_server_test"};
    std::unique_ptr<SharedRocksDB> shared;
    std::unique_ptr<CDCDB> cdc;

    TestDB() {
        _setCurrentTime(TernTime(1));
        if (mkdtemp(dir.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }
        shared = std::make_unique<SharedRocksDB>(logger, xmon, dir + "/db", dir + "/statistics");
        shared->registerCFDescriptors(CDCDB::getColumnFamilyDescriptors());
        shared->registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
        rocksdb::Options options;
        options.create_if_missing = true;
        options.create_missing_column_families = true;
        shared->openTransactionDB(options);
        cdc = std::make_unique<CDCDB>(logger, xmon, *shared);
    }

    ~TestDB() {
        cdc.reset();
        shared.reset();
        std::error_code error;
        std::filesystem::remove_all(dir, error);
        _setCurrentTime(TernTime(0));
    }
};

AddrsInfo loopback() {
    AddrsInfo addr{};
    addr[0].ip = Ip({127, 0, 0, 1});
    return addr;
}

CDCLogEntry entry(std::vector<CDCReqContainer> reqs, std::vector<CDCShardResp> resps) {
    std::vector<CDCLogEntry> entries;
    CDCLogEntry::prepareLogEntries(reqs, resps, 1 << 20, entries);
    REQUIRE(entries.size() == 1);
    return std::move(entries[0]);
}

// A rename whose final outstanding log entry answers the new edge's unlock.
// Applying that entry advances it to unlocking the old edge.
std::vector<LogsDBLogEntry> renameLog(TestDB& db, bool applyPrefix, int oldBootstraps = 0) {
    std::vector<LogsDBLogEntry> entries;
    CDCStep step;
    std::vector<CDCTxnId> txnIds;
    auto add = [&](CDCLogEntry e) {
        e.logIdx(entries.size() + 1);
        auto& logEntry = entries.emplace_back();
        logEntry.idx = e.logIdx();
        logEntry.value.resize(e.packedSize());
        BincodeBuf buf((char*)logEntry.value.data(), logEntry.value.size());
        e.pack(buf);
        if (applyPrefix && e.logIdx() <= 4) {
            db.cdc->applyLogEntry(true, e, step, txnIds);
        }
    };
    CDCReqContainer req;
    auto& rename = req.setRenameFile();
    rename.targetId = InodeId(InodeType::FILE, ShardId(3), 1);
    rename.oldOwnerId = InodeId(InodeType::DIRECTORY, ShardId(1), 1);
    rename.oldName = BincodeBytes("old");
    rename.oldCreationTime = TernTime(1);
    rename.newOwnerId = InodeId(InodeType::DIRECTORY, ShardId(2), 1);
    rename.newName = BincodeBytes("new");
    add(entry({req}, {}));
    auto answer = [&](const ShardRespContainer& resp) {
        CDCShardResp shardResp;
        shardResp.txnId = CDCTxnId(1);
        shardResp.checkPoint = 0;
        shardResp.resp = resp;
        add(entry({}, {shardResp}));
    };
    ShardRespContainer resp;
    resp.setLockCurrentEdge();
    answer(resp);
    resp.setFullReadDir();
    answer(resp);
    resp.setCreateLockedCurrentEdge().creationTime = TernTime(2);
    answer(resp);
    resp.setUnlockCurrentEdge();
    answer(resp);
    for (int i = 0; i < oldBootstraps; ++i) {
        add(CDCLogEntry::prepareBootstrapEntry());
    }
    return entries;
}

void seedLog(TestDB& db, const std::vector<LogsDBLogEntry>& entries) {
    LogsDB logs(db.logger, db.xmon, *db.shared, ReplicaId(0), 0, true, true, false);
    std::vector<LogsDBRequest> requests;
    std::vector<LogsDBResponse> responses;
    _setCurrentTime(ternNow() + LogsDB::LEADER_INACTIVE_TIMEOUT + 1_ns);
    logs.processIncomingMessages(requests, responses);
    REQUIRE(logs.isLeader());
    for (size_t i = 0; i < entries.size(); i += LogsDB::IN_FLIGHT_APPEND_WINDOW) {
        auto end = std::min(i + LogsDB::IN_FLIGHT_APPEND_WINDOW, entries.size());
        std::vector<LogsDBLogEntry> batch(entries.begin() + i, entries.begin() + end);
        REQUIRE(logs.appendEntries(batch) == TernError::NO_ERROR);
        REQUIRE(batch.back().idx == entries[end - 1].idx);
    }
    logs.flush(true);
}

struct TestServer {
    TestDB& db;
    LogsDB logs;
    CDCOptions options{};
    UDPSocketPair shard;
    AES128Key key;
    std::unique_ptr<CDCShared> shared;
    std::unique_ptr<CDCServer> server;

    TestServer(TestDB& db, bool noReplication = true) :
        db(db),
        logs(db.logger, db.xmon, *db.shared, ReplicaId(0), db.cdc->lastAppliedLogEntry(), noReplication, true, false),
        shard(db.env, loopback())
    {
        options.shardTimeout = 1_hours;
        options.registryClientOptions.host = "127.0.0.1";
        expandKey(CDCKey, key);
        std::array<UDPSocketPair, 2> sockets{
            UDPSocketPair(db.env, loopback()), UDPSocketPair(db.env, loopback())};
        shared = std::make_unique<CDCShared>(
            *db.shared, *db.cdc, logs, std::move(sockets), db.logger, db.xmon, options.registryClientOptions);
        for (auto& info : shared->shards) {
            info.addrs = shard.addr();
        }
        server = std::make_unique<CDCServer>(db.logger, db.xmon, options, *shared);
    }

    template<typename Message>
    void send(const Message& msg) {
        UDPSender sender;
        sender.prepareOutgoingMessage(db.env, shard.addr(), 0, shared->socks[0].addr()[0],
            [&](BincodeBuf& buf) { msg.pack(buf, key); });
        sender.sendMessages(db.env, shard);
    }

    void tick() {
        // A valid log message from an unregistered replica wakes the UDP poll
        // and is ignored. No sleeps or background threads are needed.
        LogReqMsg wake;
        wake.id = 0;
        auto& release = wake.body.setRelease();
        release.token = LeaderToken(0, 0);
        release.lastReleased = 0;
        send(wake);
        _setCurrentTime(ternNow() + 1_ns);
        server->step();
    }

    std::vector<CdcToShardReqMsg> takeShardRequests() {
        std::vector<CdcToShardReqMsg> requests;
        char data[MAX_UDP_MTU];
        for (;;) {
            auto len = recv(shard.socks()[0].get(), data, sizeof(data), MSG_DONTWAIT);
            if (len < 0) {
                REQUIRE((errno == EAGAIN || errno == EWOULDBLOCK));
                break;
            }
            REQUIRE(len > 0);
            BincodeBuf buf(data, len);
            requests.emplace_back().unpack(buf, key);
        }
        return requests;
    }

    CdcToShardReqMsg bootstrap(uint64_t bootstrapIdx) {
        std::vector<CdcToShardReqMsg> sent;
        for (int i = 0; i < 8; ++i) {
            tick();
            auto requests = takeShardRequests();
            if (db.cdc->lastAppliedLogEntry() < bootstrapIdx) {
                CHECK(requests.empty());
            }
            for (auto& request : requests) {
                sent.emplace_back(std::move(request));
            }
        }
        REQUIRE(logs.isLeader());
        REQUIRE(db.cdc->lastAppliedLogEntry() == bootstrapIdx);
        REQUIRE(sent.size() == 1);
        CHECK(sent[0].body.kind() == ShardMessageKind::UNLOCK_CURRENT_EDGE);
        const auto& unlock = sent[0].body.getUnlockCurrentEdge();
        CHECK(unlock.wasMoved);
        CHECK(unlock.dirId == InodeId(InodeType::DIRECTORY, ShardId(1), 1));
        CHECK(unlock.creationTime == TernTime(1));
        return std::move(sent[0]);
    }

    void finishRename(const CdcToShardReqMsg& req, uint64_t bootstrapIdx) {
        CdcToShardRespMsg resp;
        resp.id = req.id;
        resp.body.checkPointIdx = 0;
        resp.body.resp.setUnlockCurrentEdge();
        send(resp);
        for (int i = 0; i < 3; ++i) {
            tick();
            CHECK(takeShardRequests().empty());
        }
        CHECK(db.cdc->lastAppliedLogEntry() == bootstrapIdx + 1);
    }
};

}

TEST_CASE("CDC restart drives shards only from its own bootstrap") {
    int oldBootstraps = 0;
    SUBCASE("one unapplied response") {}
    SUBCASE("old bootstraps span multiple read batches") { oldBootstraps = 300; }

    TestDB db;
    auto entries = renameLog(db, true, oldBootstraps);
    seedLog(db, entries);
    TestServer server(db);
    auto bootstrapIdx = entries.size() + 1;
    auto request = server.bootstrap(bootstrapIdx);
    server.finishRename(request, bootstrapIdx);
}

TEST_CASE("CDC takeover refreshes the log index after following another leader") {
    bool applyBeforeElection = false;
    SUBCASE("released entries still need applying") {}
    SUBCASE("released entries already applied as follower") { applyBeforeElection = true; }

    TestDB db;
    TestServer server(db); // Captures index zero before receiving the old leader's entries.
    auto entries = renameLog(db, false);
    std::vector<LogsDBRequest> requests;
    std::vector<LogsDBResponse> responses;
    for (const auto& entry : entries) {
        auto& req = requests.emplace_back();
        req.replicaId = ReplicaId(1);
        req.msg.id = entry.idx.u64;
        auto& write = req.msg.body.setLogWrite();
        write.token = LeaderToken(ReplicaId(1), Epoch(1));
        write.idx = entry.idx;
        write.lastReleased = entries.back().idx;
        write.value.els = entry.value;
    }
    server.logs.processIncomingMessages(requests, responses);
    REQUIRE_FALSE(server.logs.isLeader());
    if (applyBeforeElection) {
        server.tick();
        CHECK(db.cdc->lastAppliedLogEntry() == entries.back().idx.u64);
        CHECK(server.takeShardRequests().empty());
    }
    _setCurrentTime(ternNow() + LogsDB::LEADER_INACTIVE_TIMEOUT + 1_ns);
    auto bootstrapIdx = entries.size() + 1;
    auto request = server.bootstrap(bootstrapIdx);
    server.finishRename(request, bootstrapIdx);
}

TEST_CASE("CDC re-election reuses indices discarded from its previous term") {
    TestDB db;
    TestServer server(db, false);
    _setCurrentTime(ternNow() + LogsDB::LEADER_INACTIVE_TIMEOUT + 1_ns);
    server.tick();
    REQUIRE(server.logs.isLeader());
    // The first bootstrap has index one, but no replica has acknowledged it.
    REQUIRE(server.logs.getLastReleased() == LogIdx(0));
    REQUIRE(db.cdc->lastAppliedLogEntry() == 0);

    std::vector<LogsDBRequest> requests(1);
    std::vector<LogsDBResponse> responses;
    requests[0].replicaId = ReplicaId(1);
    requests[0].msg.id = 100;
    auto& confirm = requests[0].msg.body.setNewLeaderConfirm();
    confirm.nomineeToken = LeaderToken(ReplicaId(1), Epoch(2));
    confirm.releasedIdx = 0;
    server.logs.processIncomingMessages(requests, responses);
    REQUIRE_FALSE(server.logs.isLeader());
    server.tick();
    REQUIRE_FALSE(server.shared->isLeader.load());

    _setCurrentTime(ternNow() + LogsDB::LEADER_INACTIVE_TIMEOUT + 1_ns);
    server.tick();
    REQUIRE(server.logs.isLeader());
    // Collect the retransmitted writes to verify that the new bootstrap
    // reuses index one. The old predicted index would instead have been two.
    _setCurrentTime(ternNow() + LogsDB::RESPONSE_TIMEOUT + 1_ns);
    server.logs.processIncomingMessages(requests, responses);
    std::vector<LogsDBRequest*> outgoing;
    server.logs.getOutgoingMessages(outgoing, responses);
    int writes = 0;
    for (const auto* req : outgoing) {
        if (req->msg.body.kind() == LogMessageKind::LOG_WRITE) {
            CHECK(req->msg.body.getLogWrite().idx == LogIdx(1));
            ++writes;
        }
    }
    CHECK(writes == LogsDB::REPLICA_COUNT - 1);
    CHECK(server.takeShardRequests().empty());
}
