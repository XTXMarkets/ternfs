// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <iostream>
#include <ostream>
#include <resolv.h>
#include <unordered_set>
#include <vector>

#include "LogsDB.hpp"
#include "MsgsGen.hpp"
#include "Time.hpp"
#include "utils/TempLogsDB.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

REGISTER_EXCEPTION_TRANSLATOR(AbstractException& ex) {
    std::stringstream ss;
    // Before, we had stack traces and this was useful, now a bit less
    ss << std::endl << ex.what() << std::endl;
    return doctest::String(ss.str().c_str());
}

std::ostream& operator<<(std::ostream& out, const std::vector<LogsDBRequest*>& data) {
    for (auto& d : data) {
        out << *d;
    }
    return out;
}

TEST_CASE("EmptyLogsDBNoOverrides") {
    // init time control
    _setCurrentTime(ternNow());
    TempLogsDB db(LogLevel::LOG_ERROR);
    std::vector<LogsDBLogEntry> entries;
    std::vector<LogsDBRequest> inReq;
    std::vector<LogsDBResponse> inResp;

    std::vector<LogsDBRequest*> outReq;
    std::vector<LogsDBResponse> outResp;

    // verify empty db with no flags starts as follower and does not try to pass any messages
    {
        REQUIRE_FALSE(db->isLeader());
        std::vector<LogsDBLogEntry> entries{
            initEntry(1, "entry1"),
            initEntry(4, "entry4"),
            initEntry(5, "entry5"),
        };

        REQUIRE(db->appendEntries(entries) == TernError::LEADER_PREEMPTED);
        db->getOutgoingMessages(outReq, outResp);
        REQUIRE(outReq.empty());
        REQUIRE(outResp.empty());
        entries.clear();
        db->readEntries(entries);
        REQUIRE(entries.empty());
        db->processIncomingMessages(inReq, inResp);

        REQUIRE(db->getNextTimeout() == LogsDB::LEADER_INACTIVE_TIMEOUT);
    }

    // verify writting to follower succeeds
    {
        size_t requestId{0};
        LeaderToken token(1, 1);
        std::unordered_set<size_t> reqIds;
        entries = {initEntry(1, "entry1"), initEntry(3, "entry3"), initEntry(2, "entry2")};
        for (auto& entry : entries) {
            inReq.emplace_back();
            auto& req = inReq.back();
            req.replicaId = token.replica();
            req.msg.id = requestId++;
            reqIds.emplace(req.msg.id);
            auto& writeReq = req.msg.body.setLogWrite();
            writeReq.idx = entry.idx;
            writeReq.token = token;
            writeReq.value.els = entry.value;
            writeReq.lastReleased = 0;
        }
        db->processIncomingMessages(inReq, inResp);
        db->getOutgoingMessages(outReq, outResp);
        REQUIRE(outReq.empty());
        REQUIRE(outResp.size() == entries.size());
        for (auto& resp : outResp) {
            REQUIRE(resp.replicaId == token.replica());
            REQUIRE(resp.msg.body.kind() == LogMessageKind::LOG_WRITE);
            REQUIRE(resp.msg.body.getLogWrite().result == TernError::NO_ERROR);
            reqIds.erase(resp.msg.id);
        }
        REQUIRE(reqIds.empty());
        entries.clear();
        db->readEntries(entries);
        REQUIRE(entries.empty());
    }

    // Release written data verify it's readable and no catchup requests
    {
        size_t requestId{0};
        LeaderToken token(1, 1);
        std::unordered_set<size_t> reqIds;
        inReq.clear();
        auto& req = inReq.emplace_back();
        req.replicaId = token.replica();
        req.msg.id = requestId++;
        reqIds.emplace(req.msg.id);
        auto& releaseReq = req.msg.body.setRelease();;
        releaseReq.lastReleased = 3;
        releaseReq.token = token;
        db->processIncomingMessages(inReq, inResp);
        db->getOutgoingMessages(outReq, outResp);
        std::cerr << outReq << std::endl;
        REQUIRE(outReq.empty());
        REQUIRE(outResp.empty());
        db->readEntries(entries);
        REQUIRE(entries.size() == 3);
    }
}

TEST_CASE("LogsDBStandAloneLeader") {
    _setCurrentTime(ternNow());
    LogIdx readUpTo = 0;
    TempLogsDB db(LogLevel::LOG_ERROR, 0, readUpTo,true,false);

    std::vector<LogsDBRequest> inReq;
    std::vector<LogsDBResponse> inResp;

    std::vector<LogsDBRequest*> outReq;
    std::vector<LogsDBResponse> outResp;
    db->processIncomingMessages(inReq, inResp);
    _setCurrentTime(ternNow() + LogsDB::LEADER_INACTIVE_TIMEOUT + 1_ms);
    db->processIncomingMessages(inReq, inResp);
    REQUIRE(db->isLeader());

    std::vector<LogsDBLogEntry> entries{
        initEntry(1, "entry1"),
        initEntry(4, "entry4"),
        initEntry(5, "entry5"),
    };
    auto err = db->appendEntries(entries);
    db->processIncomingMessages(inReq, inResp);
    REQUIRE(err == TernError::NO_ERROR);
    for(size_t i = 0; i < entries.size(); ++i) {
        REQUIRE(entries[i].idx == readUpTo + i + 1);
    }
    std::vector<LogsDBLogEntry> readEntries;
    db->readEntries(readEntries);
    REQUIRE(entries == readEntries);

}

TEST_CASE("LogsDBAvoidBeingLeader") {
    _setCurrentTime(ternNow());
    TempLogsDB db(LogLevel::LOG_ERROR, 0, 0, true, true);
    REQUIRE_FALSE(db->isLeader());
    std::vector<LogsDBRequest> inReq;
    std::vector<LogsDBResponse> inResp;
    db->processIncomingMessages(inReq, inResp);
    std::vector<LogsDBRequest*> outReq;
    std::vector<LogsDBResponse> outResp;
    db->getOutgoingMessages(outReq, outResp);
    REQUIRE(outResp.empty());
    REQUIRE(outReq.empty());
    REQUIRE(db->getNextTimeout() == LogsDB::LEADER_INACTIVE_TIMEOUT);
    _setCurrentTime(ternNow() + LogsDB::LEADER_INACTIVE_TIMEOUT + 1_ms);

    // Tick
    db->processIncomingMessages(inReq, inResp);
    db->getOutgoingMessages(outReq, outResp);
    REQUIRE(outResp.empty());
    REQUIRE(outReq.empty());
    REQUIRE(db->getNextTimeout() == LogsDB::LEADER_INACTIVE_TIMEOUT);
}

TEST_CASE("EmptyLogsDBLeaderElection" * doctest::skip(true)) { // leader election temporarily disabled in code
    _setCurrentTime(ternNow());
    TempLogsDB db(LogLevel::LOG_ERROR);
    REQUIRE_FALSE(db->isLeader());
    std::vector<LogsDBRequest> inReq;
    std::vector<LogsDBResponse> inResp;
    db->processIncomingMessages(inReq, inResp);
    std::vector<LogsDBRequest*> outReq;
    std::vector<LogsDBResponse> outResp;
    db->getOutgoingMessages(outReq, outResp);
    REQUIRE(outResp.empty());
    REQUIRE(outReq.empty());
    REQUIRE(db->getNextTimeout() == LogsDB::LEADER_INACTIVE_TIMEOUT);
    _setCurrentTime(ternNow() + LogsDB::LEADER_INACTIVE_TIMEOUT + 1_ms);

    // Tick
    db->processIncomingMessages(inReq, inResp);
    db->getOutgoingMessages(outReq, outResp);
    REQUIRE(outResp.empty());
    REQUIRE(db->getNextTimeout() == LogsDB::RESPONSE_TIMEOUT);


    //expect leader election messages
    REQUIRE(outReq.size() == LogsDB::REPLICA_COUNT - 1);
    std::unordered_set<uint8_t> replicaIds{1,2,3,4};

    for (size_t replicaId = 1, reqIdx = 0; replicaId < LogsDB::REPLICA_COUNT; ++replicaId, ++reqIdx) {
        auto& req = *outReq[reqIdx];
        replicaIds.erase(req.replicaId.u8);
        REQUIRE(req.msg.body.kind() == LogMessageKind::NEW_LEADER);
        REQUIRE(req.msg.body.getNewLeader().nomineeToken == LeaderToken(0, 1));
    }
    REQUIRE(replicaIds.empty());
}
