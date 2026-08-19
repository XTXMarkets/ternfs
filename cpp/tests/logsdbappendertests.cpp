// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <array>
#include <cstdio>
#include <filesystem>
#include <memory>
#include <set>
#include <stdexcept>
#include <string>
#include <vector>

#include <rocksdb/db.h>
#include <rocksdb/options.h>

#include "Env.hpp"
#include "LogsDB.hpp"
#include "SharedRocksDB.hpp"
#include "Time.hpp"
#include "logsdb/Appender.hpp"
#include "logsdb/DataPartitions.hpp"
#include "logsdb/LeaderElection.hpp"
#include "logsdb/LogMetadata.hpp"
#include "logsdb/ReqResp.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

namespace {

struct AppenderTest {
    std::string dbDir{"temp-logs-appender.XXXXXX"};
    Logger logger;
    std::shared_ptr<XmonAgent> xmon;
    Env env{logger, xmon, "AppenderTest"};
    std::unique_ptr<SharedRocksDB> sharedDB;
    LogsDBStats stats;
    std::unique_ptr<DataPartitions> data;
    std::unique_ptr<LogMetadata> metadata;
    std::unique_ptr<ReqResp> reqResp;
    std::unique_ptr<LeaderElection> leaderElection;
    std::unique_ptr<Appender> appender;

    AppenderTest(
            bool noReplication = false,
            LogLevel logLevel = LogLevel::LOG_ERROR,
            int logFd = STDERR_FILENO) :
        logger(logLevel, logFd, false, false) {
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
            throw std::runtime_error("Failed to initialize Appender test");
        }
        reqResp = std::make_unique<ReqResp>(stats);
        leaderElection = std::make_unique<LeaderElection>(
            env,
            stats,
            noReplication,
            true,
            false,
            ReplicaId(0),
            *metadata,
            *data,
            *reqResp);
        appender = std::make_unique<Appender>(
            env,
            stats,
            *reqResp,
            *metadata,
            *leaderElection,
            noReplication);
    }

    ~AppenderTest() {
        appender.reset();
        leaderElection.reset();
        reqResp.reset();
        metadata.reset();
        data.reset();
        sharedDB.reset();
        std::error_code error;
        std::filesystem::remove_all(dbDir, error);
    }

    void becomeLeader() {
        _setCurrentTime(
            ternNow() + LogsDB::LEADER_INACTIVE_TIMEOUT + 1_ns);
        leaderElection->maybeStartLeaderElection();
        if (!leaderElection->isLeader()) {
            throw std::runtime_error("Failed to become leader");
        }
        appender->maybeMoveRelease();
    }

    std::vector<LogsDBRequest*> takeRequests() {
        reqResp->resendTimedOutRequests();
        std::vector<LogsDBRequest*> requests;
        reqResp->getRequestsToSend(requests);
        return requests;
    }

    void respond(uint64_t requestId, TernError result) {
        auto* request = reqResp->getRequest(requestId);
        if (request == nullptr) {
            throw std::runtime_error("Request not found");
        }
        auto replicaId = request->replicaId;
        LogWriteResp response;
        response.result = result;
        appender->proccessLogWriteResponse(
            replicaId,
            *request,
            response);
    }

    LogsDBLogEntry makeEntry(const std::string& value) {
        return {
            LogIdx(0),
            std::vector<uint8_t>(value.begin(), value.end())
        };
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

TEST_CASE_FIXTURE(AppenderTest, "Appender rejects writes as a follower") {
    std::vector<LogsDBLogEntry> entries{
        makeEntry("one"),
        makeEntry("two"),
    };

    CHECK(
        appender->appendEntries(entries) ==
        TernError::LEADER_PREEMPTED);
    CHECK(entries[0].idx == LogIdx(0));
    CHECK(entries[1].idx == LogIdx(0));
    CHECK(appender->entriesInFlight() == 0);
    CHECK(takeRequests().empty());
}

TEST_CASE_FIXTURE(
        AppenderTest,
        "Appender initializes release requests after becoming leader") {
    becomeLeader();

    auto requests = takeRequests();
    REQUIRE(requests.size() == LogsDB::REPLICA_COUNT - 1);
    std::set<uint8_t> replicas;
    for (const auto* request : requests) {
        replicas.emplace(request->replicaId.u8);
        REQUIRE(request->msg.body.kind() == LogMessageKind::RELEASE);
        const auto& release = request->msg.body.getRelease();
        CHECK(release.token == metadata->getLeaderToken());
        CHECK(release.lastReleased == LogIdx(0));
    }
    std::set<uint8_t> expectedReplicas{1, 2, 3, 4};
    CHECK(replicas == expectedReplicas);
}

TEST_CASE_FIXTURE(
        AppenderTest,
        "Appender releases replicated entries in order") {
    becomeLeader();
    auto releaseRequests = takeRequests();
    std::array<uint64_t, LogsDB::REPLICA_COUNT> releaseIds{};
    for (const auto* request : releaseRequests) {
        releaseIds[request->replicaId.u8] = request->msg.id;
    }

    std::vector<LogsDBLogEntry> entries{
        makeEntry("one"),
        makeEntry("two"),
    };
    REQUIRE(appender->appendEntries(entries) == TernError::NO_ERROR);
    CHECK(entries[0].idx == LogIdx(1));
    CHECK(entries[1].idx == LogIdx(2));
    CHECK(appender->entriesInFlight() == 2);

    auto requests = takeRequests();
    REQUIRE(
        requests.size() ==
        2 * (LogsDB::REPLICA_COUNT - 1));
    std::array<
        std::array<uint64_t, LogsDB::REPLICA_COUNT>,
        2> writeIds{};
    for (const auto* request : requests) {
        REQUIRE(request->msg.body.kind() == LogMessageKind::LOG_WRITE);
        const auto& write = request->msg.body.getLogWrite();
        auto expectedIdx =
            write.idx == LogIdx(1) || write.idx == LogIdx(2);
        REQUIRE(expectedIdx);
        auto entryOffset = write.idx.u64 - 1;
        writeIds[entryOffset][request->replicaId.u8] = request->msg.id;
        CHECK(write.token == metadata->getLeaderToken());
        CHECK(write.lastReleased == LogIdx(0));
        CHECK(
            write.value.els ==
            entries[entryOffset].value);
    }

    respond(writeIds[1][1], TernError::NO_ERROR);
    respond(writeIds[1][2], TernError::NO_ERROR);
    appender->maybeMoveRelease();
    CHECK(metadata->getLastReleased() == LogIdx(0));
    CHECK(appender->entriesInFlight() == 2);

    respond(writeIds[0][1], TernError::NO_ERROR);
    respond(writeIds[0][2], TernError::NO_ERROR);
    appender->maybeMoveRelease();
    CHECK(metadata->getLastReleased() == LogIdx(2));
    CHECK(appender->entriesInFlight() == 0);
    CHECK(readValue(LogIdx(1)) == "one");
    CHECK(readValue(LogIdx(2)) == "two");

    for (const auto& entryIds : writeIds) {
        for (ReplicaId replicaId = 1;
             replicaId.u8 < LogsDB::REPLICA_COUNT;
             ++replicaId.u8) {
            CHECK(reqResp->getRequest(entryIds[replicaId.u8]) == nullptr);
        }
    }
    for (ReplicaId replicaId = 1;
         replicaId.u8 < LogsDB::REPLICA_COUNT;
         ++replicaId.u8) {
        auto* request = reqResp->getRequest(releaseIds[replicaId.u8]);
        REQUIRE(request != nullptr);
        const auto& release = request->msg.body.getRelease();
        CHECK(release.token == metadata->getLeaderToken());
        CHECK(release.lastReleased == LogIdx(2));
    }
}

TEST_CASE_FIXTURE(AppenderTest, "Appender limits its in-flight window") {
    becomeLeader();
    takeRequests();

    std::vector<LogsDBLogEntry> entries;
    entries.reserve(LogsDB::IN_FLIGHT_APPEND_WINDOW + 1);
    for (size_t i = 0; i <= LogsDB::IN_FLIGHT_APPEND_WINDOW; ++i) {
        entries.emplace_back(makeEntry("entry"));
    }

    REQUIRE(appender->appendEntries(entries) == TernError::NO_ERROR);
    CHECK(entries.front().idx == LogIdx(1));
    CHECK(
        entries[LogsDB::IN_FLIGHT_APPEND_WINDOW - 1].idx ==
        LogIdx(LogsDB::IN_FLIGHT_APPEND_WINDOW));
    CHECK(entries.back().idx == LogIdx(0));
    CHECK(
        appender->entriesInFlight() ==
        LogsDB::IN_FLIGHT_APPEND_WINDOW);

    std::vector<LogsDBLogEntry> extra{makeEntry("extra")};
    REQUIRE(appender->appendEntries(extra) == TernError::NO_ERROR);
    CHECK(extra[0].idx == LogIdx(0));
    CHECK(
        appender->entriesInFlight() ==
        LogsDB::IN_FLIGHT_APPEND_WINDOW);
}

TEST_CASE("Appender persists immediately without replication") {
    AppenderTest test(true);
    test.becomeLeader();
    auto releaseRequests = test.takeRequests();

    std::vector<LogsDBLogEntry> entries{
        test.makeEntry("one"),
        test.makeEntry("two"),
    };
    REQUIRE(
        test.appender->appendEntries(entries) ==
        TernError::NO_ERROR);

    CHECK(entries[0].idx == LogIdx(1));
    CHECK(entries[1].idx == LogIdx(2));
    CHECK(test.appender->entriesInFlight() == 0);
    CHECK(test.metadata->getLastReleased() == LogIdx(2));
    CHECK(test.readValue(LogIdx(1)) == "one");
    CHECK(test.readValue(LogIdx(2)) == "two");
    CHECK(test.takeRequests().empty());

    for (const auto* releaseRequest : releaseRequests) {
        auto* request =
            test.reqResp->getRequest(releaseRequest->msg.id);
        REQUIRE(request != nullptr);
        CHECK(
            request->msg.body.getRelease().lastReleased ==
            LogIdx(2));
    }
}

TEST_CASE_FIXTURE(
        AppenderTest,
        "Appender cleans up pending writes after preemption") {
    becomeLeader();
    auto releaseRequests = takeRequests();
    std::vector<uint64_t> releaseIds;
    for (const auto* request : releaseRequests) {
        releaseIds.emplace_back(request->msg.id);
    }

    std::vector<LogsDBLogEntry> entries{makeEntry("pending")};
    REQUIRE(appender->appendEntries(entries) == TernError::NO_ERROR);
    auto writeRequests = takeRequests();
    REQUIRE(
        writeRequests.size() ==
        LogsDB::REPLICA_COUNT - 1);
    std::vector<uint64_t> writeIds;
    for (const auto* request : writeRequests) {
        writeIds.emplace_back(request->msg.id);
    }

    respond(writeIds.front(), TernError::LEADER_PREEMPTED);
    CHECK_FALSE(leaderElection->isLeader());
    CHECK(appender->entriesInFlight() == 1);

    appender->maybeMoveRelease();
    CHECK(appender->entriesInFlight() == 0);
    CHECK(readValue(LogIdx(1)).empty());
    for (auto requestId : releaseIds) {
        CHECK(reqResp->getRequest(requestId) == nullptr);
    }
    for (auto requestId : writeIds) {
        CHECK(reqResp->getRequest(requestId) == nullptr);
    }

    std::vector<LogsDBLogEntry> rejected{makeEntry("rejected")};
    CHECK(
        appender->appendEntries(rejected) ==
        TernError::LEADER_PREEMPTED);
}

TEST_CASE_FIXTURE(
        AppenderTest,
        "accepting a foreign leader token clears local leadership") {
    becomeLeader();
    takeRequests();

    std::vector<LogsDBLogEntry> pending{makeEntry("pending")};
    REQUIRE(appender->appendEntries(pending) == TernError::NO_ERROR);
    CHECK(appender->entriesInFlight() == 1);

    auto foreignToken = LeaderToken(ReplicaId(1), Epoch(2));
    std::vector<LogsDBLogEntry> foreignEntries;
    REQUIRE(
        leaderElection->writeLogEntries(
            foreignToken,
            metadata->getLastReleased(),
            foreignEntries) ==
        TernError::NO_ERROR);

    CHECK_FALSE(leaderElection->isLeader());
    CHECK(metadata->getLeaderToken() == foreignToken);

    appender->maybeMoveRelease();
    CHECK(appender->entriesInFlight() == 0);

    std::vector<LogsDBLogEntry> rejected{makeEntry("rejected")};
    CHECK(
        appender->appendEntries(rejected) ==
        TernError::LEADER_PREEMPTED);
}

TEST_CASE("accepting leader writes does not reset an existing follower") {
    auto* logFile = tmpfile();
    REQUIRE(logFile != nullptr);
    {
        AppenderTest test(
            false,
            LogLevel::LOG_INFO,
            fileno(logFile));
        auto foreignToken = LeaderToken(ReplicaId(1), Epoch(1));
        std::vector<LogsDBLogEntry> entries;

        REQUIRE(
            test.leaderElection->writeLogEntries(
                foreignToken,
                test.metadata->getLastReleased(),
                entries) ==
            TernError::NO_ERROR);
        CHECK_FALSE(test.leaderElection->isLeader());
        CHECK(test.metadata->getLeaderToken() == foreignToken);
    }

    rewind(logFile);
    std::string logs;
    std::array<char, 1024> buffer;
    while (fgets(buffer.data(), buffer.size(), logFile) != nullptr) {
        logs += buffer.data();
    }
    CHECK(logs.find("Reseting leader election") == std::string::npos);
    fclose(logFile);
}
