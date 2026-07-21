// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <array>
#include <filesystem>
#include <memory>
#include <stdexcept>
#include <string>
#include <vector>

#include <rocksdb/db.h>
#include <rocksdb/options.h>

#include "Env.hpp"
#include "LogsDB.hpp"
#include "SharedRocksDB.hpp"
#include "Time.hpp"
#include "logsdb/BatchWriter.hpp"
#include "logsdb/DataPartitions.hpp"
#include "logsdb/LeaderElection.hpp"
#include "logsdb/LogMetadata.hpp"
#include "logsdb/ReqResp.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

namespace {

struct BatchWriterTest {
    std::string dbDir{"temp-logs-batch-writer.XXXXXX"};
    Logger logger{LogLevel::LOG_ERROR, STDERR_FILENO, false, false};
    std::shared_ptr<XmonAgent> xmon;
    Env env{logger, xmon, "BatchWriterTest"};
    std::unique_ptr<SharedRocksDB> sharedDB;
    LogsDBStats stats;
    std::unique_ptr<DataPartitions> data;
    std::unique_ptr<LogMetadata> metadata;
    std::unique_ptr<ReqResp> reqResp;
    std::unique_ptr<LeaderElection> leaderElection;
    std::unique_ptr<BatchWriter> writer;

    BatchWriterTest() {
        _setCurrentTime(TernTime(1));
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

        data = std::make_unique<DataPartitions>(env, *sharedDB);
        metadata = std::make_unique<LogMetadata>(
            env,
            stats,
            *sharedDB,
            ReplicaId(0),
            *data);
        if (!metadata->init(true) || !data->init(true)) {
            throw std::runtime_error("Failed to initialize BatchWriter test");
        }
        reqResp = std::make_unique<ReqResp>(stats);
        leaderElection = std::make_unique<LeaderElection>(
            env,
            stats,
            false,
            false,
            false,
            ReplicaId(0),
            *metadata,
            *data,
            *reqResp);
        writer = std::make_unique<BatchWriter>(
            env,
            *reqResp,
            *leaderElection);
    }

    ~BatchWriterTest() {
        writer.reset();
        leaderElection.reset();
        reqResp.reset();
        metadata.reset();
        data.reset();
        sharedDB.reset();
        std::error_code error;
        std::filesystem::remove_all(dbDir, error);
    }

    LogsDBRequest makeWriteRequest(
            uint64_t requestId,
            LeaderToken token,
            LogIdx idx,
            LogIdx lastReleased,
            const std::string& value) {
        LogsDBRequest request;
        request.replicaId = token.replica();
        request.msg.id = requestId;
        auto& write = request.msg.body.setLogWrite();
        write.token = token;
        write.idx = idx;
        write.lastReleased = lastReleased;
        write.value.els.assign(value.begin(), value.end());
        return request;
    }

    std::vector<LogsDBResponse> takeResponses() {
        std::vector<LogsDBResponse> responses;
        reqResp->getResponsesToSend(responses);
        return responses;
    }

    std::string readValue(LogIdx idx) {
        LogsDBLogEntry entry;
        if (data->readLogEntry(idx, entry) != TernError::NO_ERROR) {
            return {};
        }
        return std::string(entry.value.begin(), entry.value.end());
    }
};

}

TEST_CASE_FIXTURE(
        BatchWriterTest,
        "BatchWriter batches writes with the same token") {
    auto token = LeaderToken(ReplicaId(1), LogIdx(1));
    std::array requests{
        makeWriteRequest(11, token, LogIdx(1), LogIdx(1), "one"),
        makeWriteRequest(12, token, LogIdx(2), LogIdx(2), "two"),
    };

    writer->proccessLogWriteRequest(requests[0]);
    writer->proccessLogWriteRequest(requests[1]);
    CHECK(takeResponses().empty());

    writer->writeBatch();

    auto responses = takeResponses();
    REQUIRE(responses.size() == 2);
    CHECK(responses[0].msg.id == 11);
    CHECK(
        responses[0].msg.body.getLogWrite().result ==
        TernError::NO_ERROR);
    CHECK(responses[1].msg.id == 12);
    CHECK(
        responses[1].msg.body.getLogWrite().result ==
        TernError::NO_ERROR);
    CHECK(readValue(LogIdx(1)) == "one");
    CHECK(readValue(LogIdx(2)) == "two");
    CHECK(metadata->getLeaderToken() == token);
    CHECK(metadata->getLastReleased() == LogIdx(2));
}

TEST_CASE_FIXTURE(
        BatchWriterTest,
        "BatchWriter flushes when a higher token arrives") {
    auto firstToken = LeaderToken(ReplicaId(1), LogIdx(1));
    auto secondToken = LeaderToken(ReplicaId(1), LogIdx(2));
    auto first = makeWriteRequest(
        21,
        firstToken,
        LogIdx(1),
        LogIdx(1),
        "one");
    auto second = makeWriteRequest(
        22,
        secondToken,
        LogIdx(2),
        LogIdx(2),
        "two");

    writer->proccessLogWriteRequest(first);
    writer->proccessLogWriteRequest(second);

    auto responses = takeResponses();
    REQUIRE(responses.size() == 1);
    CHECK(responses[0].msg.id == 21);
    CHECK(readValue(LogIdx(1)) == "one");
    CHECK(readValue(LogIdx(2)).empty());
    CHECK(metadata->getLeaderToken() == firstToken);

    writer->writeBatch();

    responses = takeResponses();
    REQUIRE(responses.size() == 1);
    CHECK(responses[0].msg.id == 22);
    CHECK(readValue(LogIdx(2)) == "two");
    CHECK(metadata->getLeaderToken() == secondToken);
    CHECK(metadata->getLastReleased() == LogIdx(2));
}

TEST_CASE_FIXTURE(
        BatchWriterTest,
        "BatchWriter rejects a lower token while a batch is pending") {
    auto currentToken = LeaderToken(ReplicaId(1), LogIdx(2));
    auto staleToken = LeaderToken(ReplicaId(1), LogIdx(1));
    auto current = makeWriteRequest(
        31,
        currentToken,
        LogIdx(1),
        LogIdx(1),
        "current");
    auto stale = makeWriteRequest(
        32,
        staleToken,
        LogIdx(1),
        LogIdx(1),
        "stale");

    writer->proccessLogWriteRequest(current);
    writer->proccessLogWriteRequest(stale);

    auto responses = takeResponses();
    REQUIRE(responses.size() == 1);
    CHECK(responses[0].msg.id == 32);
    CHECK(
        responses[0].msg.body.getLogWrite().result ==
        TernError::LEADER_PREEMPTED);

    writer->writeBatch();

    responses = takeResponses();
    REQUIRE(responses.size() == 1);
    CHECK(responses[0].msg.id == 31);
    CHECK(
        responses[0].msg.body.getLogWrite().result ==
        TernError::NO_ERROR);
    CHECK(readValue(LogIdx(1)) == "current");
}

TEST_CASE_FIXTURE(
        BatchWriterTest,
        "BatchWriter applies release-only batches without responses") {
    auto token = LeaderToken(ReplicaId(2), LogIdx(1));
    ReleaseReq release;
    release.token = token;
    release.lastReleased = LogIdx(3);

    writer->proccessReleaseRequest(ReplicaId(2), 41, release);
    writer->writeBatch();

    CHECK(takeResponses().empty());
    CHECK(metadata->getLeaderToken() == token);
    CHECK(metadata->getLastReleased() == LogIdx(3));
}

TEST_CASE_FIXTURE(
        BatchWriterTest,
        "BatchWriter ignores replica and token mismatches") {
    auto token = LeaderToken(ReplicaId(1), LogIdx(1));
    auto write = makeWriteRequest(
        51,
        token,
        LogIdx(1),
        LogIdx(1),
        "ignored");
    write.replicaId = ReplicaId(2);

    ReleaseReq release;
    release.token = token;
    release.lastReleased = LogIdx(2);

    writer->proccessLogWriteRequest(write);
    writer->proccessReleaseRequest(ReplicaId(2), 52, release);
    writer->writeBatch();

    CHECK(takeResponses().empty());
    CHECK(metadata->getLeaderToken() == LeaderToken(0, 0));
    CHECK(metadata->getLastReleased() == LogIdx(0));
    CHECK(readValue(LogIdx(1)).empty());
}
