// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "BatchWriter.hpp"

#include "../Assert.hpp"
#include "../LogsDB.hpp"
#include "../Msgs.hpp"

BatchWriter::BatchWriter(Env& env, ReqResp& reqResp, LeaderElection& leaderElection) :
    _env(env),
    _reqResp(reqResp),
    _leaderElection(leaderElection),
    _token(LeaderToken(0,0)),
    _lastReleased(0) {}

void BatchWriter::proccessLogWriteRequest(LogsDBRequest& request) {
    ALWAYS_ASSERT(request.msg.body.kind() == LogMessageKind::LOG_WRITE);
    const auto& writeRequest = request.msg.body.getLogWrite();
    if (unlikely(request.replicaId != writeRequest.token.replica())) {
        LOG_ERROR(_env, "Token from replica id %s does not have matching replica id. Token: %s", request.replicaId, writeRequest.token);
        return;
    }
    if (unlikely(writeRequest.token < _token)) {
        auto& resp = _reqResp.newResponse(request.replicaId, request.msg.id);
        auto& writeResponse = resp.msg.body.setLogWrite();
        writeResponse.result = TernError::LEADER_PREEMPTED;
        return;
    }
    if (unlikely(_token < writeRequest.token )) {
        writeBatch();
        _token = writeRequest.token;
    }
    _requests.emplace_back(&request);
    _entries.emplace_back();
    auto& entry = _entries.back();
    entry.idx = writeRequest.idx;
    entry.value = writeRequest.value.els;
    if (_lastReleased < writeRequest.lastReleased) {
        _lastReleased = writeRequest.lastReleased;
    }
}

void BatchWriter::proccessReleaseRequest(ReplicaId fromReplicaId, uint64_t requestId, const ReleaseReq& request) {
    if (unlikely(fromReplicaId != request.token.replica())) {
        LOG_ERROR(_env, "Token from replica id %s does not have matching replica id. Token: %s", fromReplicaId, request.token);
        return;
    }
    if (unlikely(request.token < _token)) {
        return;
    }
    if (unlikely(_token < request.token )) {
        writeBatch();
        _token = request.token;
    }

    if (_lastReleased < request.lastReleased) {
        _lastReleased = request.lastReleased;
    }

}

void BatchWriter::writeBatch() {
    if (_token == LeaderToken(0,0)) {
        return;
    }
    auto response = _leaderElection.writeLogEntries(_token, _lastReleased, _entries);
    for (auto req : _requests) {
        auto& resp = _reqResp.newResponse(req->replicaId, req->msg.id);
        auto& writeResponse = resp.msg.body.setLogWrite();
        writeResponse.result = response;
    }
    _requests.clear();
    _entries.clear();
    _lastReleased = 0;
    _token = LeaderToken(0,0);
}
