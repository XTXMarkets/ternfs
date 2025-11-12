// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "Appender.hpp"

#include <algorithm>

#include "../Assert.hpp"
#include "../LogsDB.hpp"
#include "../Msgs.hpp"

Appender::Appender(Env& env, LogsDBStats& stats, ReqResp& reqResp, LogMetadata& metadata, LeaderElection& leaderElection, bool noReplication) :
    _env(env),
    _reqResp(reqResp),
    _metadata(metadata),
    _leaderElection(leaderElection),
    _noReplication(noReplication),
    _currentIsLeader(false),
    _entriesStart(0),
    _entriesEnd(0) { }

void Appender::maybeMoveRelease() {
    if (!_currentIsLeader && _leaderElection.isLeader()) {
        _init();
        return;
    }
    if (!_leaderElection.isLeader() && _currentIsLeader) {
        _cleanup();
        return;
    }

    if (!_currentIsLeader) {
        return;
    }

    auto newRelease = _metadata.getLastReleased();
    std::vector<LogsDBLogEntry> entriesToWrite;
    for (; _entriesStart < _entriesEnd; ++_entriesStart) {
        auto offset = _entriesStart & IN_FLIGHT_MASK;
        auto& requestIds = _requestIds[offset];
        if (_noReplication || ReqResp::isQuorum(requestIds)) {
            ++newRelease;
            entriesToWrite.emplace_back(std::move(_entries[offset]));
            ALWAYS_ASSERT(newRelease == entriesToWrite.back().idx);
            _reqResp.cleanupRequests(requestIds);
            continue;
        }
        break;
    }
    if (entriesToWrite.empty()) {
        return;
    }

    auto err = _leaderElection.writeLogEntries(_metadata.getLeaderToken(), newRelease, entriesToWrite);
    ALWAYS_ASSERT(err == TernError::NO_ERROR);
    for (auto reqId : _releaseRequests) {
        if (reqId == 0) {
            continue;
        }
        auto request = _reqResp.getRequest(reqId);
        ALWAYS_ASSERT(request->msg.body.kind() == LogMessageKind::RELEASE);
        auto& releaseReq = request->msg.body.setRelease();
        releaseReq.token = _metadata.getLeaderToken();
        releaseReq.lastReleased = _metadata.getLastReleased();
    }
}

TernError Appender::appendEntries(std::vector<LogsDBLogEntry>& entries) {
    if (!_leaderElection.isLeader()) {
        return TernError::LEADER_PREEMPTED;
    }
    auto availableSpace = LogsDB::IN_FLIGHT_APPEND_WINDOW - entriesInFlight();
    auto countToAppend = std::min(entries.size(), availableSpace);
    for(size_t i = 0; i < countToAppend; ++i) {
        entries[i].idx = _metadata.assignLogIdx();
        auto offset = (_entriesEnd + i) & IN_FLIGHT_MASK;
        _entries[offset] = entries[i];
        auto& requestIds = _requestIds[offset];
        for(ReplicaId replicaId = 0; replicaId.u8 < LogsDB::REPLICA_COUNT; ++replicaId.u8) {
            if (replicaId == _metadata.getReplicaId()) {
                requestIds[replicaId.u8] = 0;
                continue;
            }
            if (unlikely(_noReplication)) {
                requestIds[replicaId.u8] = 0;
                continue;
            }
            auto& req = _reqResp.newRequest(replicaId);
            auto& writeReq = req.msg.body.setLogWrite();
            writeReq.token = _metadata.getLeaderToken();
            writeReq.lastReleased = _metadata.getLastReleased();
            writeReq.idx = _entries[offset].idx;
            writeReq.value.els = _entries[offset].value;
            requestIds[replicaId.u8] = req.msg.id;
        }
    }
    for (size_t i = countToAppend; i < entries.size(); ++i) {
        entries[i].idx = 0;
    }
    _entriesEnd += countToAppend;
    if (unlikely(_noReplication)) {
        maybeMoveRelease();
    }
    return TernError::NO_ERROR;
}

void Appender::proccessLogWriteResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogWriteResp& response) {
    if (!_leaderElection.isLeader()) {
        return;
    }
    switch ((TernError)response.result) {
        case TernError::NO_ERROR:
            break;
        case TernError::LEADER_PREEMPTED:
            _leaderElection.resetLeaderElection();
            return;
        default:
            LOG_ERROR(_env, "Unexpected result from LOG_WRITE response %s", response.result);
            return;
    }

    auto logIdx = request.msg.body.getLogWrite().idx;
    ALWAYS_ASSERT(_metadata.getLastReleased() < logIdx);
    auto offset = _entriesStart + (logIdx.u64 - _metadata.getLastReleased().u64  - 1);
    ALWAYS_ASSERT(offset < _entriesEnd);
    offset &= IN_FLIGHT_MASK;
    ALWAYS_ASSERT(_entries[offset].idx == logIdx);
    auto& requestIds = _requestIds[offset];
    if (requestIds[fromReplicaId.u8] != request.msg.id) {
        LOG_ERROR(_env, "Mismatch in expected requestId in LOG_WRITE response %s", response);
        return;
    }
    requestIds[fromReplicaId.u8] = 0;
    _reqResp.eraseRequest(request.msg.id);
}

uint64_t Appender::entriesInFlight() const {
    return _entriesEnd - _entriesStart;
}

void Appender::_init() {
    for(ReplicaId replicaId = 0; replicaId.u8 < LogsDB::REPLICA_COUNT; ++replicaId.u8) {
        if (replicaId == _metadata.getReplicaId()) {
            _releaseRequests[replicaId.u8] = 0;
            continue;
        }
        auto& req = _reqResp.newRequest(replicaId);
        auto& releaseReq = req.msg.body.setRelease();
        releaseReq.token = _metadata.getLeaderToken();
        releaseReq.lastReleased = _metadata.getLastReleased();
        _releaseRequests[replicaId.u8] = req.msg.id;
    }
    _currentIsLeader = true;
}

void Appender::_cleanup() {
    for (; _entriesStart < _entriesEnd; ++_entriesStart) {
        auto offset = _entriesStart & IN_FLIGHT_MASK;
        _entries[offset].value.clear();
        _reqResp.cleanupRequests(_requestIds[offset]);
    }
    _reqResp.cleanupRequests(_releaseRequests);
    _currentIsLeader = false;
}
