// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <algorithm>

#include "LogsDB.hpp"
#include "logsdb/ReqResp.hpp"
#include "Time.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

TEST_CASE("ReqResp assigns and tracks request ids") {
    LogsDBStats stats;
    ReqResp reqResp(stats);

    auto& first = reqResp.newRequest(ReplicaId(2));
    auto firstId = first.msg.id;
    CHECK(firstId == 1);
    CHECK(first.replicaId == ReplicaId(2));
    CHECK(reqResp.getRequest(firstId) == &first);
    CHECK(reqResp.getNextTimeout() == LogsDB::RESPONSE_TIMEOUT);

    auto& second = reqResp.newRequest(ReplicaId(4));
    auto secondId = second.msg.id;
    CHECK(secondId == 2);
    CHECK(second.replicaId == ReplicaId(4));

    reqResp.eraseRequest(firstId);
    CHECK(reqResp.getRequest(firstId) == nullptr);
    CHECK(reqResp.getRequest(secondId) == &second);

    reqResp.eraseRequest(secondId);
    CHECK(reqResp.getNextTimeout() == LogsDB::LEADER_INACTIVE_TIMEOUT);
}

TEST_CASE("ReqResp cleanup preserves sentinels and removes pending requests") {
    LogsDBStats stats;
    ReqResp reqResp(stats);
    ReqResp::QuorumTrackArray requestIds;
    requestIds.fill(ReqResp::UNUSED_REQ_ID);

    auto& request = reqResp.newRequest(ReplicaId(1));
    auto requestId = request.msg.id;
    requestIds[0] = ReqResp::CONFIRMED_REQ_ID;
    requestIds[1] = requestId;

    reqResp.cleanupRequests(requestIds);

    CHECK(requestIds[0] == ReqResp::CONFIRMED_REQ_ID);
    CHECK(requestIds[1] == ReqResp::UNUSED_REQ_ID);
    CHECK(reqResp.getRequest(requestId) == nullptr);
}

TEST_CASE("ReqResp drains responses") {
    LogsDBStats stats;
    ReqResp reqResp(stats);

    auto& response = reqResp.newResponse(ReplicaId(3), 42);
    response.msg.body.setLogWrite().result = TernError::NO_ERROR;

    std::vector<LogsDBResponse> responses;
    reqResp.getResponsesToSend(responses);

    REQUIRE(responses.size() == 1);
    CHECK(responses[0].replicaId == ReplicaId(3));
    CHECK(responses[0].msg.id == 42);
    CHECK(responses[0].msg.body.getLogWrite().result == TernError::NO_ERROR);

    responses.clear();
    reqResp.getResponsesToSend(responses);
    CHECK(responses.empty());
}

TEST_CASE("ReqResp resends requests according to message timeout") {
    _setCurrentTime(TernTime(1));
    LogsDBStats stats;
    ReqResp reqResp(stats);

    auto& regular = reqResp.newRequest(ReplicaId(1));
    regular.msg.body.setNewLeader();
    regular.sentTime = ternNow();

    auto& release = reqResp.newRequest(ReplicaId(2));
    release.msg.body.setRelease();
    release.sentTime = ternNow();

    auto& read = reqResp.newRequest(ReplicaId(3));
    read.msg.body.setLogRead();
    read.sentTime = ternNow();

    _setCurrentTime(ternNow() + LogsDB::RESPONSE_TIMEOUT + 1_ns);
    reqResp.resendTimedOutRequests();
    std::vector<LogsDBRequest*> requests;
    reqResp.getRequestsToSend(requests);
    REQUIRE(requests.size() == 1);
    CHECK(requests[0]->msg.id == regular.msg.id);

    _setCurrentTime(ternNow() + LogsDB::SEND_RELEASE_INTERVAL + 1_ns);
    reqResp.resendTimedOutRequests();
    reqResp.getRequestsToSend(requests);
    REQUIRE(requests.size() == 2);
    CHECK(std::ranges::any_of(
        requests,
        [&](const auto* request) { return request->msg.id == regular.msg.id; }));
    CHECK(std::ranges::any_of(
        requests,
        [&](const auto* request) { return request->msg.id == release.msg.id; }));

    _setCurrentTime(ternNow() + LogsDB::READ_TIMEOUT + 1_ns);
    reqResp.resendTimedOutRequests();
    reqResp.getRequestsToSend(requests);
    REQUIRE(requests.size() == 3);
    CHECK(std::ranges::any_of(
        requests,
        [&](const auto* request) { return request->msg.id == read.msg.id; }));
}

TEST_CASE("ReqResp quorum requires a strict majority") {
    ReqResp::QuorumTrackArray requestIds;
    requestIds.fill(ReqResp::UNUSED_REQ_ID);

    requestIds[0] = ReqResp::CONFIRMED_REQ_ID;
    requestIds[1] = ReqResp::CONFIRMED_REQ_ID;
    CHECK_FALSE(ReqResp::isQuorum(requestIds));

    requestIds[2] = ReqResp::CONFIRMED_REQ_ID;
    CHECK(ReqResp::isQuorum(requestIds));
}
