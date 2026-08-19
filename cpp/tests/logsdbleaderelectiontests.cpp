// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <algorithm>
#include <filesystem>
#include <memory>
#include <string>
#include <vector>

#include <rocksdb/db.h>
#include <rocksdb/options.h>

#include "Env.hpp"
#include "LogsDB.hpp"
#include "SharedRocksDB.hpp"
#include "Time.hpp"
#include "logsdb/DataPartitions.hpp"
#include "logsdb/LeaderElection.hpp"
#include "logsdb/LogMetadata.hpp"
#include "logsdb/ReqResp.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

namespace {

struct LeaderElectionTestDB {
    std::string dbDir{"temp-logs-leader-election.XXXXXX"};
    Logger logger{LogLevel::LOG_ERROR, STDERR_FILENO, false, false};
    std::shared_ptr<XmonAgent> xmon;
    Env env{logger, xmon, "LeaderElectionTest"};
    std::unique_ptr<SharedRocksDB> sharedDB;

    LeaderElectionTestDB() {
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

    ~LeaderElectionTestDB() {
        sharedDB.reset();
        std::error_code error;
        std::filesystem::remove_all(dbDir, error);
    }
};

std::vector<LogsDBRequest*> dispatchRequests(ReqResp& reqResp) {
    reqResp.resendTimedOutRequests();
    std::vector<LogsDBRequest*> requests;
    reqResp.getRequestsToSend(requests);
    return requests;
}

LogsDBRequest* findRequest(
        const std::vector<LogsDBRequest*>& requests,
        ReplicaId replicaId,
        LogMessageKind kind,
        LogIdx idx = LogIdx(0)) {
    auto it = std::ranges::find_if(
        requests,
        [&](const auto* request) {
            if (request->replicaId != replicaId ||
                request->msg.body.kind() != kind) {
                return false;
            }
            if (kind == LogMessageKind::LOG_RECOVERY_READ) {
                return request->msg.body.getLogRecoveryRead().idx == idx;
            }
            return true;
        });
    return it == requests.end() ? nullptr : *it;
}

LogsDBLogEntry makeEntry(uint64_t idx, const std::string& value) {
    return {
        LogIdx(idx),
        std::vector<uint8_t>(value.begin(), value.end())
    };
}

}

TEST_CASE("LeaderElection completes an empty-log election") {
    _setCurrentTime(TernTime(1'000'000'000));
    LeaderElectionTestDB testDB;
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

    ReqResp reqResp(stats);
    LeaderElection election(
        testDB.env,
        stats,
        false,
        false,
        false,
        ReplicaId(0),
        metadata,
        data,
        reqResp);

    _setCurrentTime(
        ternNow() + LogsDB::LEADER_INACTIVE_TIMEOUT + 1_ns);
    election.maybeStartLeaderElection();
    CHECK_FALSE(election.isLeader());
    CHECK(metadata.getNomineeToken() == LeaderToken(ReplicaId(0), Epoch(1)));

    auto nominationRequests = dispatchRequests(reqResp);
    REQUIRE(nominationRequests.size() == LogsDB::REPLICA_COUNT - 1);
    auto* nomination1 = findRequest(
        nominationRequests,
        ReplicaId(1),
        LogMessageKind::NEW_LEADER);
    auto* nomination2 = findRequest(
        nominationRequests,
        ReplicaId(2),
        LogMessageKind::NEW_LEADER);
    REQUIRE(nomination1 != nullptr);
    REQUIRE(nomination2 != nullptr);

    NewLeaderResp nominationResponse;
    nominationResponse.result = TernError::NO_ERROR;
    nominationResponse.lastReleased = LogIdx(0);
    election.proccessNewLeaderResponse(
        ReplicaId(1),
        *nomination1,
        nominationResponse);
    CHECK_FALSE(election.isLeader());
    election.proccessNewLeaderResponse(
        ReplicaId(2),
        *nomination2,
        nominationResponse);

    auto digestRequests = dispatchRequests(reqResp);
    REQUIRE(
        digestRequests.size() ==
        2 * LogsDB::IN_FLIGHT_APPEND_WINDOW);
    auto* digest1 = findRequest(
        digestRequests,
        ReplicaId(1),
        LogMessageKind::LOG_RECOVERY_READ,
        LogIdx(1));
    auto* digest2 = findRequest(
        digestRequests,
        ReplicaId(2),
        LogMessageKind::LOG_RECOVERY_READ,
        LogIdx(1));
    REQUIRE(digest1 != nullptr);
    REQUIRE(digest2 != nullptr);

    LogRecoveryReadResp digestResponse;
    digestResponse.result = TernError::LOG_ENTRY_MISSING;
    election.proccessRecoveryReadResponse(
        ReplicaId(1),
        *digest1,
        digestResponse);
    CHECK_FALSE(election.isLeader());
    election.proccessRecoveryReadResponse(
        ReplicaId(2),
        *digest2,
        digestResponse);

    auto completionRequests = dispatchRequests(reqResp);
    REQUIRE(completionRequests.size() == 2);
    auto* completion1 = findRequest(
        completionRequests,
        ReplicaId(1),
        LogMessageKind::NEW_LEADER_CONFIRM);
    auto* completion2 = findRequest(
        completionRequests,
        ReplicaId(2),
        LogMessageKind::NEW_LEADER_CONFIRM);
    REQUIRE(completion1 != nullptr);
    REQUIRE(completion2 != nullptr);
    CHECK(
        completion1->msg.body.getNewLeaderConfirm().nomineeToken ==
        LeaderToken(ReplicaId(0), Epoch(1)));
    CHECK(
        completion1->msg.body.getNewLeaderConfirm().releasedIdx ==
        LogIdx(0));

    NewLeaderConfirmResp completionResponse;
    completionResponse.result = TernError::NO_ERROR;
    election.proccessNewLeaderConfirmResponse(
        ReplicaId(1),
        *completion1,
        completionResponse);
    CHECK_FALSE(election.isLeader());
    election.proccessNewLeaderConfirmResponse(
        ReplicaId(2),
        *completion2,
        completionResponse);

    CHECK(election.isLeader());
    CHECK(metadata.getLeaderToken() == LeaderToken(ReplicaId(0), Epoch(1)));
    CHECK(metadata.getNomineeToken() == LeaderToken(0, 0));
    CHECK(metadata.getLastReleased() == LogIdx(0));
}

TEST_CASE("LeaderElection participant handles recovery and completion") {
    _setCurrentTime(TernTime(1));
    LeaderElectionTestDB testDB;
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
    metadata.setLastReleased(LogIdx(1));

    ReqResp reqResp(stats);
    LeaderElection election(
        testDB.env,
        stats,
        false,
        false,
        false,
        ReplicaId(0),
        metadata,
        data,
        reqResp);
    auto nomineeToken = LeaderToken(ReplicaId(1), Epoch(1));

    NewLeaderReq nomination;
    nomination.nomineeToken = nomineeToken;
    election.proccessNewLeaderRequest(ReplicaId(1), 11, nomination);

    std::vector<LogsDBResponse> responses;
    reqResp.getResponsesToSend(responses);
    REQUIRE(responses.size() == 1);
    CHECK(responses[0].msg.id == 11);
    CHECK(
        responses[0].msg.body.getNewLeader().result ==
        TernError::NO_ERROR);
    CHECK(
        responses[0].msg.body.getNewLeader().lastReleased ==
        LogIdx(1));
    CHECK(metadata.getNomineeToken() == nomineeToken);

    LogRecoveryReadReq recoveryRead;
    recoveryRead.nomineeToken = nomineeToken;
    recoveryRead.idx = LogIdx(1);
    election.proccessRecoveryReadRequest(
        ReplicaId(1),
        12,
        recoveryRead);

    LogRecoveryWriteReq recoveryWrite;
    recoveryWrite.nomineeToken = nomineeToken;
    recoveryWrite.idx = LogIdx(2);
    recoveryWrite.value.els = makeEntry(2, "two").value;
    election.proccessRecoveryWriteRequest(
        ReplicaId(1),
        13,
        recoveryWrite);

    responses.clear();
    reqResp.getResponsesToSend(responses);
    REQUIRE(responses.size() == 2);
    CHECK(
        responses[0].msg.body.getLogRecoveryRead().result ==
        TernError::NO_ERROR);
    CHECK(
        std::string(
            responses[0].msg.body.getLogRecoveryRead().value.els.begin(),
            responses[0].msg.body.getLogRecoveryRead().value.els.end()) ==
        "one");
    CHECK(
        responses[1].msg.body.getLogRecoveryWrite().result ==
        TernError::NO_ERROR);

    NewLeaderConfirmReq staleCompletion;
    staleCompletion.nomineeToken = nomineeToken;
    staleCompletion.releasedIdx = LogIdx(0);
    election.proccessNewLeaderConfirmRequest(
        ReplicaId(1),
        14,
        staleCompletion);

    responses.clear();
    reqResp.getResponsesToSend(responses);
    REQUIRE(responses.size() == 1);
    CHECK(
        responses[0].msg.body.getNewLeaderConfirm().result ==
        TernError::MALFORMED_REQUEST);
    CHECK(metadata.getLastReleased() == LogIdx(1));
    CHECK(metadata.getNomineeToken() == nomineeToken);

    NewLeaderConfirmReq completion;
    completion.nomineeToken = nomineeToken;
    completion.releasedIdx = LogIdx(2);
    election.proccessNewLeaderConfirmRequest(
        ReplicaId(1),
        15,
        completion);

    responses.clear();
    reqResp.getResponsesToSend(responses);
    REQUIRE(responses.size() == 1);
    CHECK(
        responses[0].msg.body.getNewLeaderConfirm().result ==
        TernError::NO_ERROR);
    CHECK(metadata.getLeaderToken() == nomineeToken);
    CHECK(metadata.getNomineeToken() == LeaderToken(0, 0));
    CHECK(metadata.getLastReleased() == LogIdx(2));

    LogsDBLogEntry entry;
    REQUIRE(data.readLogEntry(LogIdx(2), entry) == TernError::NO_ERROR);
    CHECK(std::string(entry.value.begin(), entry.value.end()) == "two");

    nomination.nomineeToken = LeaderToken(ReplicaId(2), Epoch(1));
    election.proccessNewLeaderRequest(ReplicaId(2), 16, nomination);
    responses.clear();
    reqResp.getResponsesToSend(responses);
    REQUIRE(responses.size() == 1);
    CHECK(
        responses[0].msg.body.getNewLeader().result ==
        TernError::LEADER_PREEMPTED);
}
