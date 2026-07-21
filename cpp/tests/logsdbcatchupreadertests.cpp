// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <filesystem>
#include <memory>
#include <set>
#include <string>
#include <vector>

#include <rocksdb/db.h>
#include <rocksdb/options.h>

#include "Env.hpp"
#include "LogsDB.hpp"
#include "SharedRocksDB.hpp"
#include "Time.hpp"
#include "logsdb/CatchupReader.hpp"
#include "logsdb/DataPartitions.hpp"
#include "logsdb/LogMetadata.hpp"
#include "logsdb/ReqResp.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

namespace {

struct CatchupReaderTestDB {
    std::string dbDir{"temp-logs-catchup-reader.XXXXXX"};
    Logger logger{LogLevel::LOG_ERROR, STDERR_FILENO, false, false};
    std::shared_ptr<XmonAgent> xmon;
    Env env{logger, xmon, "CatchupReaderTest"};
    std::unique_ptr<SharedRocksDB> sharedDB;

    CatchupReaderTestDB() {
        if (mkdtemp(dbDir.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }

        sharedDB = std::make_unique<SharedRocksDB>(
            logger,
            xmon,
            dbDir + "/db",
            dbDir + "/db-statistics.txt");
        sharedDB->registerCFDescriptors(
            {{rocksdb::kDefaultColumnFamilyName, {}}});
        sharedDB->registerCFDescriptors(
            LogsDB::getColumnFamilyDescriptors());

        rocksdb::Options options;
        options.create_if_missing = true;
        options.create_missing_column_families = true;
        options.compression = rocksdb::kLZ4Compression;
        options.bottommost_compression = rocksdb::kZSTD;
        options.manual_wal_flush = true;
        sharedDB->open(options);
    }

    ~CatchupReaderTestDB() {
        sharedDB.reset();
        std::error_code error;
        std::filesystem::remove_all(dbDir, error);
    }
};

LogsDBLogEntry makeEntry(uint64_t idx, const std::string& value) {
    return {
        LogIdx(idx),
        std::vector<uint8_t>(value.begin(), value.end())
    };
}

std::string entryValue(const LogsDBLogEntry& entry) {
    return std::string(entry.value.begin(), entry.value.end());
}

}

TEST_CASE("CatchupReader repairs a released hole from one peer") {
    _setCurrentTime(TernTime(10'000'000'000));
    CatchupReaderTestDB testDB;
    LogsDBStats stats;
    DataPartitions data(testDB.env, *testDB.sharedDB);
    LogMetadata metadata(
        testDB.env,
        stats,
        *testDB.sharedDB,
        ReplicaId(0),
        data);
    REQUIRE(metadata.init(true));
    REQUIRE(data.init(true));

    data.writeLogEntries({
        makeEntry(1, "one"),
        makeEntry(3, "three"),
    });
    metadata.setLastReleased(LogIdx(3));

    ReqResp reqResp(stats);
    CatchupReader reader(
        stats,
        reqResp,
        metadata,
        data,
        ReplicaId(0),
        LogIdx(0));
    reader.init();

    CHECK(reader.getLastContinuous() == LogIdx(1));
    CHECK(reader.lastRead() == LogIdx(0));

    reqResp.resendTimedOutRequests();
    std::vector<LogsDBRequest*> requests;
    reqResp.getRequestsToSend(requests);
    REQUIRE(requests.size() == LogsDB::REPLICA_COUNT - 1);

    std::set<uint8_t> targetReplicas;
    std::vector<uint64_t> requestIds;
    for (const auto* request : requests) {
        targetReplicas.emplace(request->replicaId.u8);
        requestIds.emplace_back(request->msg.id);
        CHECK(request->msg.body.kind() == LogMessageKind::LOG_READ);
        CHECK(request->msg.body.getLogRead().idx == LogIdx(2));
    }
    CHECK(
        targetReplicas ==
        std::set<uint8_t>{1, 2, 3, 4});

    auto* request = requests.front();
    auto fromReplicaId = request->replicaId;
    LogReadResp response;
    response.result = TernError::NO_ERROR;
    response.value.els = makeEntry(2, "two").value;
    reader.proccessLogReadResponse(fromReplicaId, *request, response);

    for (auto requestId : requestIds) {
        CHECK(reqResp.getRequest(requestId) == nullptr);
    }

    reader.maybeCatchUp();
    CHECK(reader.getLastContinuous() == LogIdx(3));

    LogsDBLogEntry repaired;
    REQUIRE(data.readLogEntry(LogIdx(2), repaired) == TernError::NO_ERROR);
    CHECK(entryValue(repaired) == "two");

    std::vector<LogsDBLogEntry> entries;
    reader.readEntries(entries, 2);
    REQUIRE(entries.size() == 2);
    CHECK(entryValue(entries[0]) == "one");
    CHECK(entryValue(entries[1]) == "two");
    CHECK(reader.lastRead() == LogIdx(2));

    entries.clear();
    reader.readEntries(entries, 2);
    REQUIRE(entries.size() == 1);
    CHECK(entryValue(entries[0]) == "three");
    CHECK(reader.lastRead() == LogIdx(3));
}

TEST_CASE("CatchupReader serves released entries and read errors") {
    _setCurrentTime(TernTime(1));
    CatchupReaderTestDB testDB;
    LogsDBStats stats;
    DataPartitions data(testDB.env, *testDB.sharedDB);
    LogMetadata metadata(
        testDB.env,
        stats,
        *testDB.sharedDB,
        ReplicaId(0),
        data);
    REQUIRE(metadata.init(true));
    REQUIRE(data.init(true));

    data.writeLogEntry(makeEntry(1, "one"));
    metadata.setLastReleased(LogIdx(2));

    ReqResp reqResp(stats);
    CatchupReader reader(
        stats,
        reqResp,
        metadata,
        data,
        ReplicaId(0),
        LogIdx(0));

    LogReadReq request;
    request.idx = LogIdx(1);
    reader.proccessLogReadRequest(ReplicaId(4), 11, request);
    request.idx = LogIdx(2);
    reader.proccessLogReadRequest(ReplicaId(4), 12, request);
    request.idx = LogIdx(3);
    reader.proccessLogReadRequest(ReplicaId(4), 13, request);

    std::vector<LogsDBResponse> responses;
    reqResp.getResponsesToSend(responses);
    REQUIRE(responses.size() == 3);

    CHECK(responses[0].replicaId == ReplicaId(4));
    CHECK(responses[0].msg.id == 11);
    CHECK(
        responses[0].msg.body.getLogRead().result ==
        TernError::NO_ERROR);
    CHECK(
        std::string(
            responses[0].msg.body.getLogRead().value.els.begin(),
            responses[0].msg.body.getLogRead().value.els.end()) ==
        "one");

    CHECK(responses[1].msg.id == 12);
    CHECK(
        responses[1].msg.body.getLogRead().result ==
        TernError::LOG_ENTRY_MISSING);

    CHECK(responses[2].msg.id == 13);
    CHECK(
        responses[2].msg.body.getLogRead().result ==
        TernError::LOG_ENTRY_UNRELEASED);
}
