// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <filesystem>
#include <iostream>
#include <memory>
#include <unistd.h>

#include "CDCDB.hpp"
#include "Env.hpp"
#include "SharedRocksDB.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

REGISTER_EXCEPTION_TRANSLATOR(AbstractException& ex) {
    std::stringstream ss;
    ss << std::endl << ex.what() << std::endl;
    return doctest::String(ss.str().c_str());
}

struct TempCDCDB {
    std::string dbDir;
    Logger logger;
    std::shared_ptr<XmonAgent> xmon;
    std::unique_ptr<SharedRocksDB> sharedDB;
    std::unique_ptr<CDCDB> db;

    TempCDCDB(LogLevel level) : logger(level, STDERR_FILENO, false, false) {
        dbDir = std::string("temp-cdc-db.XXXXXX");
        if (mkdtemp(dbDir.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }
        sharedDB = std::make_unique<SharedRocksDB>(logger, xmon, dbDir + "/db", dbDir + "/db-statistics.txt");
        sharedDB->registerCFDescriptors(CDCDB::getColumnFamilyDescriptors());
        rocksdb::Options options;
        options.create_if_missing = true;
        options.create_missing_column_families = true;
        sharedDB->openTransactionDB(options);
        db = std::make_unique<CDCDB>(logger, xmon, *sharedDB);
    }

    ~TempCDCDB() {
        db.reset();
        sharedDB->close();
        std::error_code err;
        if (std::filesystem::remove_all(std::filesystem::path(dbDir), err) < 0) {
            std::cerr << "Could not remove " << dbDir << ": " << err << std::endl;
        }
    }

    std::unique_ptr<CDCDB>& operator->() {
        return db;
    }
};

static CDCLogEntry logEntry(uint64_t idx, std::vector<CDCReqContainer> reqs, std::vector<CDCShardResp> resps) {
    std::vector<CDCLogEntry> entries;
    CDCLogEntry::prepareLogEntries(reqs, resps, 1 << 20, entries);
    REQUIRE(entries.size() == 1);
    entries[0].logIdx(idx);
    return std::move(entries[0]);
}

static CDCShardResp shardResp(CDCTxnId txnId, const ShardRespContainer& resp) {
    CDCShardResp r;
    r.txnId = txnId;
    r.checkPoint = 0;
    r.resp = resp;
    return r;
}

// The schedule from a CDC restart: the log entry released before the stop
// answers a rename's unlock of the new edge, so applying it sends the unlock
// of the old edge; the bootstrap entry then drives the executing rename once
// more and the old edge's unlock goes out twice. Both answers are logged: the
// first finishes the rename, the second (EDGE_NOT_FOUND, the edge has moved)
// answers a txn that no longer executes. Applying it used to throw
// NotFound, on every restart, since the entry stays in the log.
TEST_CASE("second answer to a step driven twice is dropped") {
    TempCDCDB db(LogLevel::LOG_ERROR);
    CDCStep step;
    std::vector<CDCTxnId> txnIds;
    uint64_t idx = 0;

    CDCReqContainer req;
    {
        auto& rename = req.setRenameFile();
        rename.targetId = InodeId(InodeType::FILE, ShardId(3), 1);
        rename.oldOwnerId = InodeId(InodeType::DIRECTORY, ShardId(1), 1);
        rename.oldName = BincodeBytes("f000.tmp1");
        rename.oldCreationTime = TernTime(1);
        rename.newOwnerId = InodeId(InodeType::DIRECTORY, ShardId(2), 1);
        rename.newName = BincodeBytes("f000");
    }
    db->applyLogEntry(true, logEntry(++idx, {req}, {}), step, txnIds);
    REQUIRE(txnIds.size() == 1);
    const CDCTxnId txn = txnIds[0];
    REQUIRE(step.runningTxns.size() == 1);

    auto answer = [&](const ShardRespContainer& resp) {
        db->applyLogEntry(true, logEntry(++idx, {}, {shardResp(txn, resp)}), step, txnIds);
    };
    ShardRespContainer resp;
    resp.setLockCurrentEdge();
    answer(resp);
    resp.setFullReadDir();
    answer(resp);
    resp.setCreateLockedCurrentEdge().creationTime = TernTime(2);
    answer(resp);
    resp.setUnlockCurrentEdge(); // the new edge
    answer(resp);
    REQUIRE(step.runningTxns.size() == 1);
    CHECK(step.runningTxns[0].second.req.getUnlockCurrentEdge().wasMoved);

    auto bootstrap = CDCLogEntry::prepareBootstrapEntry();
    bootstrap.logIdx(++idx);
    db->applyLogEntry(true, bootstrap, step, txnIds);
    REQUIRE(step.runningTxns.size() == 1);
    CHECK(step.runningTxns[0].first == txn);

    resp.setUnlockCurrentEdge(); // the old edge, first answer
    answer(resp);
    REQUIRE(step.finishedTxns.size() == 1);
    CHECK(step.finishedTxns[0].first == txn);

    resp.setError() = TernError::EDGE_NOT_FOUND; // the old edge, second answer
    answer(resp);
    CHECK(step.finishedTxns.empty());
    CHECK(step.runningTxns.empty());
    CHECK(db->lastAppliedLogEntry() == idx);
}
