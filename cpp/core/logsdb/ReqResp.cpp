// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "ReqResp.hpp"

#include "../LogsDB.hpp"
#include "../Time.hpp"
#include "LogsDBCommon.hpp"

ReqResp::ReqResp(LogsDBStats& stats) : _stats(stats), _lastAssignedRequest(CONFIRMED_REQ_ID) {}

LogsDBRequest& ReqResp::newRequest(ReplicaId targetReplicaId) {
    auto& request = _requests[++_lastAssignedRequest];
    request.replicaId = targetReplicaId;
    request.msg.id = _lastAssignedRequest;
    return request;
}

LogsDBRequest* ReqResp::getRequest(uint64_t requestId) {
    auto it = _requests.find(requestId);
    if (it == _requests.end()) {
        return nullptr;
    }
    return &it->second;
}

void ReqResp::eraseRequest(uint64_t requestId) {
    _requests.erase(requestId);
}

void ReqResp::cleanupRequests(QuorumTrackArray& requestIds) {
    for (auto& reqId : requestIds) {
        if (reqId == CONFIRMED_REQ_ID || reqId == UNUSED_REQ_ID) {
            continue;
        }
        eraseRequest(reqId);
        reqId = ReqResp::UNUSED_REQ_ID;
    }
}

void ReqResp::resendTimedOutRequests() {
    auto now = ternNow();
    auto defaultCutoffTime = now - LogsDBConsts::RESPONSE_TIMEOUT;
    auto releaseCutoffTime = now - LogsDBConsts::SEND_RELEASE_INTERVAL;
    auto readCutoffTime = now - LogsDBConsts::READ_TIMEOUT;
    auto cutoffTime = now;
    uint64_t timedOutCount{0};
    for (auto& r : _requests) {
        switch (r.second.msg.body.kind()) {
        case LogMessageKind::RELEASE:
            cutoffTime = releaseCutoffTime;
            break;
        case LogMessageKind::LOG_READ:
            cutoffTime = readCutoffTime;
            break;
        default:
            cutoffTime = defaultCutoffTime;
        }
        if (r.second.sentTime < cutoffTime) {
            r.second.sentTime = now;
            _requestsToSend.emplace_back(&r.second);
            if (r.second.msg.body.kind() != LogMessageKind::RELEASE) {
                ++timedOutCount;
            }
        }
    }
    update_atomic_stat_ema(_stats.requestsTimedOut, timedOutCount);
}

void ReqResp::getRequestsToSend(std::vector<LogsDBRequest*>& requests) {
    requests.swap(_requestsToSend);
    update_atomic_stat_ema(_stats.requestsSent, requests.size());
    _requestsToSend.clear();
}

LogsDBResponse& ReqResp::newResponse(ReplicaId targetReplicaId, uint64_t requestId) {
    _responses.emplace_back();
    auto& response = _responses.back();
    response.replicaId = targetReplicaId;
    response.msg.id = requestId;
    return response;
}

void ReqResp::getResponsesToSend(std::vector<LogsDBResponse>& responses) {
    responses.swap(_responses);
    update_atomic_stat_ema(_stats.responsesSent, responses.size());
    _responses.clear();
}

Duration ReqResp::getNextTimeout() const {
    if (_requests.empty()) {
        return LogsDBConsts::LEADER_INACTIVE_TIMEOUT;
    }
    return LogsDBConsts::RESPONSE_TIMEOUT;
}

bool ReqResp::isQuorum(const QuorumTrackArray& requestIds) {
    size_t numResponses = 0;
    for (auto reqId : requestIds) {
        if (reqId == CONFIRMED_REQ_ID) {
            ++numResponses;
        }
    }
    return numResponses > requestIds.size() / 2;
}
