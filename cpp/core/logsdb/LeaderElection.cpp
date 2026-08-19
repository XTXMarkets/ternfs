// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "LeaderElection.hpp"

#include <algorithm>

#include "../Assert.hpp"

static void update_atomic_stat_ema(std::atomic<Duration>& stat, Duration newValue) {
    stat.store((Duration)((double)stat.load(std::memory_order_relaxed).ns * 0.95 + (double)newValue.ns * 0.05), std::memory_order_relaxed);
}

std::ostream& operator<<(std::ostream& out, LeadershipState state) {
    switch (state) {
    case LeadershipState::FOLLOWER:
        out << "FOLLOWER";
        break;
    case LeadershipState::BECOMING_NOMINEE:
        out << "BECOMING_NOMINEE";
        break;
    case LeadershipState::DIGESTING_ENTRIES:
        out << "DIGESTING_ENTRIES";
        break;
    case LeadershipState::CONFIRMING_REPLICATION:
        out << "CONFIRMING_REPLICATION";
        break;
    case LeadershipState::CONFIRMING_LEADERSHIP:
        out << "CONFIRMING_LEADERSHIP";
        break;
    case LeadershipState::LEADER:
        out << "LEADER";
        break;
    }
    return out;
}

LeaderElection::LeaderElection(Env& env, LogsDBStats& stats, bool noReplication, bool skipLeaderElection, bool avoidBeingLeader, ReplicaId replicaId, LogMetadata& metadata, DataPartitions& data, ReqResp& reqResp) :
    _env(env),
    _stats(stats),
    _noReplication(noReplication),
    _skipLeaderElection(skipLeaderElection),
    _avoidBeingLeader(avoidBeingLeader),
    _replicaId(replicaId),
    _metadata(metadata),
    _data(data),
    _reqResp(reqResp),
    _state(LeadershipState::FOLLOWER),
    _leaderLastActive(_noReplication ? 0 :ternNow()) {}

bool LeaderElection::isLeader() const {
    return _state == LeadershipState::LEADER;
}

void LeaderElection::maybeStartLeaderElection() {
    if (unlikely(_avoidBeingLeader)) {
        return;
    }
    auto now = ternNow();
    if (_state != LeadershipState::FOLLOWER ||
        (_leaderLastActive + LogsDB::LEADER_INACTIVE_TIMEOUT > now)) {
        update_atomic_stat_ema(_stats.leaderLastActive, now - _leaderLastActive);
        return;
    }
    auto nomineeToken = _metadata.generateNomineeToken();
    LOG_INFO(_env,"Starting new leader election round with token %s", nomineeToken);
    _metadata.setNomineeToken(nomineeToken);
    _state = LeadershipState::BECOMING_NOMINEE;

    _electionState.reset(new LeaderElectionState());
    _electionState->lastReleased = _metadata.getLastReleased();
    _leaderLastActive = now;

    if (unlikely(_skipLeaderElection || _noReplication)) {
        LOG_INFO(_env,"ForceLeader set, skipping to confirming leader phase");
        _electionState->requestIds.fill(ReqResp::CONFIRMED_REQ_ID);
        _tryBecomeLeader();
        return;
    }
    auto& newLeaderRequestIds = _electionState->requestIds;
    for (ReplicaId replicaId = 0; replicaId.u8 < newLeaderRequestIds.size(); ++replicaId.u8) {
        if (replicaId == _replicaId) {
            newLeaderRequestIds[replicaId.u8] = ReqResp::CONFIRMED_REQ_ID;
            continue;
        }
        auto& request = _reqResp.newRequest(replicaId);
        newLeaderRequestIds[replicaId.u8] = request.msg.id;

        auto& newLeaderRequest = request.msg.body.setNewLeader();
        newLeaderRequest.nomineeToken = nomineeToken;
    }
}

void LeaderElection::proccessNewLeaderResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const NewLeaderResp& response) {
    LOG_DEBUG(_env, "Received NEW_LEADER response %s from replicaId %s", response, fromReplicaId);
    ALWAYS_ASSERT(_state == LeadershipState::BECOMING_NOMINEE, "In state %s Received NEW_LEADER response %s", _state, response);
    auto& state = *_electionState;
    ALWAYS_ASSERT(_electionState->requestIds[fromReplicaId.u8] == request.msg.id);
    auto result = TernError(response.result);
    switch (result) {
        case TernError::NO_ERROR:
            _electionState->requestIds[request.replicaId.u8] = ReqResp::CONFIRMED_REQ_ID;
            _electionState->lastReleased = std::max(_electionState->lastReleased, response.lastReleased);
            _reqResp.eraseRequest(request.msg.id);
            _tryProgressToDigest();
            break;
        case TernError::LEADER_PREEMPTED:
            resetLeaderElection();
            break;
        default:
            LOG_ERROR(_env, "Unexpected result %s in NEW_LEADER message, %s", result, response);
            break;
    }
}

void LeaderElection::proccessNewLeaderConfirmResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const NewLeaderConfirmResp& response) {
    ALWAYS_ASSERT(_state == LeadershipState::CONFIRMING_LEADERSHIP, "In state %s Received NEW_LEADER_CONFIRM response %s", _state, response);
    ALWAYS_ASSERT(_electionState->requestIds[fromReplicaId.u8] == request.msg.id);

    auto result = TernError(response.result);
    switch (result) {
    case TernError::NO_ERROR:
        _electionState->requestIds[request.replicaId.u8] = 0;
        _reqResp.eraseRequest(request.msg.id);
        LOG_DEBUG(_env,"trying to become leader");
        _tryBecomeLeader();
        break;
    case TernError::LEADER_PREEMPTED:
        resetLeaderElection();
        break;
    default:
        LOG_ERROR(_env, "Unexpected result %s in NEW_LEADER_CONFIRM message, %s", result, response);
        break;
    }
}

void LeaderElection::proccessRecoveryReadResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogRecoveryReadResp& response) {
    ALWAYS_ASSERT(_state == LeadershipState::DIGESTING_ENTRIES, "In state %s Received LOG_RECOVERY_READ response %s", _state, response);
    auto& state = *_electionState;
    auto result = TernError(response.result);
    switch (result) {
        case TernError::NO_ERROR:
        case TernError::LOG_ENTRY_MISSING:
        {
            ALWAYS_ASSERT(state.lastReleased < request.msg.body.getLogRecoveryRead().idx);
            auto entryOffset = request.msg.body.getLogRecoveryRead().idx.u64 - state.lastReleased.u64 - 1;
            ALWAYS_ASSERT(entryOffset < LogsDB::IN_FLIGHT_APPEND_WINDOW);
            ALWAYS_ASSERT(state.recoveryRequests[entryOffset][request.replicaId.u8] == request.msg.id);
            auto& entry = state.recoveryEntries[entryOffset];
            if (response.value.els.size() != 0) {
                // we found a record here, we don't care about other answers
                entry.value = response.value.els;
                _reqResp.cleanupRequests(state.recoveryRequests[entryOffset]);
            } else {
                state.recoveryRequests[entryOffset][request.replicaId.u8] = 0;
                _reqResp.eraseRequest(request.msg.id);
            }
            _tryProgressToReplication();
            break;
        }
        case TernError::LEADER_PREEMPTED:
            LOG_DEBUG(_env, "Got preempted during recovery by replica %s",fromReplicaId);
            resetLeaderElection();
            break;
        default:
            LOG_ERROR(_env, "Unexpected result %s in LOG_RECOVERY_READ message, %s", result, response);
            break;
    }
}

void LeaderElection::proccessRecoveryWriteResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogRecoveryWriteResp& response) {
    ALWAYS_ASSERT(_state == LeadershipState::CONFIRMING_REPLICATION, "In state %s Received LOG_RECOVERY_WRITE response %s", _state, response);
    auto& state = *_electionState;
    auto result = TernError(response.result);
    switch (result) {
        case TernError::NO_ERROR:
        {
            ALWAYS_ASSERT(state.lastReleased < request.msg.body.getLogRecoveryWrite().idx);
            auto entryOffset = request.msg.body.getLogRecoveryWrite().idx.u64 - state.lastReleased.u64 - 1;
            ALWAYS_ASSERT(entryOffset < LogsDB::IN_FLIGHT_APPEND_WINDOW);
            ALWAYS_ASSERT(state.recoveryRequests[entryOffset][request.replicaId.u8] == request.msg.id);
            state.recoveryRequests[entryOffset][request.replicaId.u8] = 0;
            _reqResp.eraseRequest(request.msg.id);
            _tryProgressToLeaderConfirm();
            break;
        }
        case TernError::LEADER_PREEMPTED:
            resetLeaderElection();
            break;
        default:
            LOG_ERROR(_env, "Unexpected result %s in LOG_RECOVERY_READ message, %s", result, response);
            break;
    }
}

void LeaderElection::proccessNewLeaderRequest(ReplicaId fromReplicaId, uint64_t requestId, const NewLeaderReq& request) {
    if (unlikely(fromReplicaId != request.nomineeToken.replica())) {
        LOG_ERROR(_env, "Nominee token from replica id %s does not have matching replica id. Token: %s", fromReplicaId, request.nomineeToken);
        return;
    }
    auto& response = _reqResp.newResponse( fromReplicaId, requestId);
    auto& newLeaderResponse = response.msg.body.setNewLeader();

    if (request.nomineeToken.epoch() <= _metadata.getLeaderToken().epoch() || request.nomineeToken < _metadata.getNomineeToken()) {
        newLeaderResponse.result = TernError::LEADER_PREEMPTED;
        return;
    }

    newLeaderResponse.result = TernError::NO_ERROR;
    newLeaderResponse.lastReleased = _metadata.getLastReleased();
    _leaderLastActive = ternNow();

    if (_metadata.getNomineeToken() == request.nomineeToken) {
        return;
    }

    resetLeaderElection();
    _metadata.setNomineeToken(request.nomineeToken);
}

void LeaderElection::proccessNewLeaderConfirmRequest(ReplicaId fromReplicaId, uint64_t requestId, const NewLeaderConfirmReq& request) {
    if (unlikely(fromReplicaId != request.nomineeToken.replica())) {
        LOG_ERROR(_env, "Nominee token from replica id %s does not have matching replica id. Token: %s", fromReplicaId, request.nomineeToken);
        return;
    }
    auto& response = _reqResp.newResponse(fromReplicaId, requestId);
    auto& newLeaderConfirmResponse = response.msg.body.setNewLeaderConfirm();
    if (_metadata.getNomineeToken() == request.nomineeToken) {
        if (unlikely(request.releasedIdx < _metadata.getLastReleased())) {
            LOG_ERROR(
                _env,
                "NEW_LEADER_CONFIRM from replica %s moves release point backwards from %s to %s",
                fromReplicaId,
                _metadata.getLastReleased(),
                request.releasedIdx);
            newLeaderConfirmResponse.result = TernError::MALFORMED_REQUEST;
            return;
        }
        _metadata.setLastReleased(request.releasedIdx);
    }

    auto err = _metadata.updateLeaderToken(request.nomineeToken);
    newLeaderConfirmResponse.result = err;
    if (err == TernError::NO_ERROR) {
        _leaderLastActive = ternNow();
        resetLeaderElection();
    }
}

void LeaderElection::proccessRecoveryReadRequest(ReplicaId fromReplicaId, uint64_t requestId, const LogRecoveryReadReq& request) {
    if (unlikely(fromReplicaId != request.nomineeToken.replica())) {
        LOG_ERROR(_env, "Nominee token from replica id %s does not have matching replica id. Token: %s", fromReplicaId, request.nomineeToken);
        return;
    }
    auto& response = _reqResp.newResponse(fromReplicaId, requestId);
    auto& recoveryReadResponse = response.msg.body.setLogRecoveryRead();
    if (request.nomineeToken != _metadata.getNomineeToken()) {
        recoveryReadResponse.result = TernError::LEADER_PREEMPTED;
        return;
    }
    _leaderLastActive = ternNow();
    LogsDBLogEntry entry;
    auto err = _data.readLogEntry(request.idx, entry);
    recoveryReadResponse.result = err;
    if (err == TernError::NO_ERROR) {
        recoveryReadResponse.value.els = entry.value;
    }
}

void LeaderElection::proccessRecoveryWriteRequest(ReplicaId fromReplicaId, uint64_t requestId, const LogRecoveryWriteReq& request) {
    if (unlikely(fromReplicaId != request.nomineeToken.replica())) {
        LOG_ERROR(_env, "Nominee token from replica id %s does not have matching replica id. Token: %s", fromReplicaId, request.nomineeToken);
        return;
    }
    auto& response = _reqResp.newResponse(fromReplicaId, requestId);
    auto& recoveryWriteResponse = response.msg.body.setLogRecoveryWrite();
    if (request.nomineeToken != _metadata.getNomineeToken()) {
        recoveryWriteResponse.result = TernError::LEADER_PREEMPTED;
        return;
    }
    _leaderLastActive = ternNow();
    LogsDBLogEntry entry;
    entry.idx = request.idx;
    entry.value = request.value.els;
    _data.writeLogEntry(entry);
    recoveryWriteResponse.result = TernError::NO_ERROR;
}

TernError LeaderElection::writeLogEntries(LeaderToken token, LogIdx newlastReleased, std::vector<LogsDBLogEntry>& entries) {
    auto err = _metadata.updateLeaderToken(token);
    if (err != TernError::NO_ERROR) {
        return err;
    }
    _clearElectionState();
    _data.writeLogEntries(entries);
    if (_metadata.getLastReleased() < newlastReleased) {
        _metadata.setLastReleased(newlastReleased);
    }
    return TernError::NO_ERROR;
}

void LeaderElection::resetLeaderElection() {
    if (isLeader()) {
        LOG_INFO(_env,"Preempted as leader. Reseting leader election. Becoming follower");
    } else {
        LOG_INFO(_env,"Reseting leader election. Becoming follower of leader with token %s", _metadata.getLeaderToken());
    }
    _state = LeadershipState::FOLLOWER;
    _leaderLastActive = ternNow();
    _metadata.setNomineeToken(LeaderToken(0,0));
    _clearElectionState();
}

void LeaderElection::_tryProgressToDigest() {
    ALWAYS_ASSERT(_state == LeadershipState::BECOMING_NOMINEE);
    LOG_DEBUG(_env, "trying to progress to digest");
    if (!ReqResp::isQuorum(_electionState->requestIds)) {
        return;
    }
    _reqResp.cleanupRequests(_electionState->requestIds);
    _state = LeadershipState::DIGESTING_ENTRIES;
    LOG_INFO(_env,"Became nominee with token: %s", _metadata.getNomineeToken());

    // We might have gotten a higher release point. We can safely update
    _metadata.setLastReleased(_electionState->lastReleased);

    // Populate entries we have and don't ask for them
    std::vector<LogsDBLogEntry> entries;
    entries.reserve(LogsDB::IN_FLIGHT_APPEND_WINDOW);
    auto it = _data.getIterator();
    it.seek(_electionState->lastReleased);
    it.next();
    for(; it.valid(); ++it) {
        entries.emplace_back(it.entry());
    }
    ALWAYS_ASSERT(entries.size() <= LogsDB::IN_FLIGHT_APPEND_WINDOW);
    for (auto& entry : entries) {
        auto offset = entry.idx.u64 - _electionState->lastReleased.u64 - 1;
        _electionState->recoveryEntries[offset] = entry;
    }

    // Ask for all non populated entries
    for(size_t i = 0; i < _electionState->recoveryEntries.size(); ++i) {
        auto& entry = _electionState->recoveryEntries[i];
        if (!entry.value.empty()) {
            continue;
        }
        entry.idx = _electionState->lastReleased + i + 1;
        auto& requestIds = _electionState->recoveryRequests[i];
        auto& participatingReplicas = _electionState->requestIds;
        for(ReplicaId replicaId = 0; replicaId.u8 < LogsDB::REPLICA_COUNT; ++replicaId.u8) {
            if (replicaId == _replicaId) {
                requestIds[replicaId.u8] = ReqResp::CONFIRMED_REQ_ID;
                continue;
            }
            if (participatingReplicas[replicaId.u8] != ReqResp::CONFIRMED_REQ_ID) {
                requestIds[replicaId.u8] = ReqResp::UNUSED_REQ_ID;
                continue;
            }
            auto& request = _reqResp.newRequest(replicaId);
            auto& recoveryRead = request.msg.body.setLogRecoveryRead();
            recoveryRead.idx = entry.idx;
            recoveryRead.nomineeToken = _metadata.getNomineeToken();
            requestIds[replicaId.u8] = request.msg.id;
        }
    }
}

void LeaderElection::_tryProgressToReplication() {
    ALWAYS_ASSERT(_state == LeadershipState::DIGESTING_ENTRIES);
    bool canMakeProgress{false};
    for(size_t i = 0; i < _electionState->recoveryEntries.size(); ++i) {
        if (_electionState->recoveryEntries[i].value.empty()) {
            auto& requestIds = _electionState->recoveryRequests[i];
            if (ReqResp::isQuorum(requestIds)) {
                canMakeProgress = true;
            }
            if (canMakeProgress) {
                _reqResp.cleanupRequests(requestIds);
                continue;
            }
            return;
        }
    }
    // If we came here it means whole array contains records
    // Send replication requests until first hole
    _state = LeadershipState::CONFIRMING_REPLICATION;
    std::vector<LogsDBLogEntry> entries;
    entries.reserve(_electionState->recoveryEntries.size());
    for(size_t i = 0; i < _electionState->recoveryEntries.size(); ++i) {
        auto& entry = _electionState->recoveryEntries[i];
        if (entry.value.empty()) {
            break;
        }
        auto& requestIds = _electionState->recoveryRequests[i];
        auto& participatingReplicas = _electionState->requestIds;
        for (ReplicaId replicaId = 0; replicaId.u8 < LogsDB::REPLICA_COUNT; ++replicaId.u8) {
            if (replicaId == _replicaId) {
                requestIds[replicaId.u8] = ReqResp::CONFIRMED_REQ_ID;
                continue;
            }
            if (participatingReplicas[replicaId.u8] != ReqResp::CONFIRMED_REQ_ID) {
                requestIds[replicaId.u8] = ReqResp::UNUSED_REQ_ID;
                continue;
            }
            entries.emplace_back(entry);
            auto& request = _reqResp.newRequest(replicaId);
            auto& recoveryWrite = request.msg.body.setLogRecoveryWrite();
            recoveryWrite.idx = entry.idx;
            recoveryWrite.nomineeToken = _metadata.getNomineeToken();
            recoveryWrite.value.els = entry.value;
            requestIds[replicaId.u8] = request.msg.id;
        }
    }
    LOG_INFO(_env,"Digesting complete progressing to replication of %s entries with token: %s", entries.size(), _metadata.getNomineeToken());
    if (entries.empty()) {
        _tryProgressToLeaderConfirm();
    } else {
        _data.writeLogEntries(entries);
    }
}

void LeaderElection::_tryProgressToLeaderConfirm() {
    ALWAYS_ASSERT(_state == LeadershipState::CONFIRMING_REPLICATION);
    LogIdx newLastReleased = _electionState->lastReleased;
    for(size_t i = 0; i < _electionState->recoveryEntries.size(); ++i) {
        if (_electionState->recoveryEntries[i].value.empty()) {
            break;
        }
        auto& requestIds = _electionState->recoveryRequests[i];
        if (!ReqResp::isQuorum(requestIds)) {
            // we just confirmed replication up to this point.
            // It is safe to move last released for us even if we don't become leader
            // while not necessary for correctness it somewhat helps making progress in multiple preemtion case
            _metadata.setLastReleased(newLastReleased);
            return;
        }
        newLastReleased = _electionState->recoveryEntries[i].idx;
        _reqResp.cleanupRequests(requestIds);
    }
    // we just confirmed replication up to this point.
    // It is safe to move last released for us even if we don't become leader
    // if we do become leader we guarantee state up here was readable
    _metadata.setLastReleased(newLastReleased);
    _state = LeadershipState::CONFIRMING_LEADERSHIP;
    LOG_INFO(_env,"Replication of extra records complete. Progressing to CONFIRMING_LEADERSHIP with token: %s, newLastReleased: %s", _metadata.getNomineeToken(), newLastReleased);

    auto& requestIds = _electionState->requestIds;
    for (ReplicaId replicaId = 0; replicaId.u8 < LogsDB::REPLICA_COUNT; ++replicaId.u8) {
        if (replicaId == _replicaId) {
            requestIds[replicaId.u8] = ReqResp::CONFIRMED_REQ_ID;
            continue;
        }
        if (requestIds[replicaId.u8] == ReqResp::UNUSED_REQ_ID) {
            continue;
        }
        auto& request = _reqResp.newRequest(replicaId);
        auto& recoveryConfirm = request.msg.body.setNewLeaderConfirm();
        recoveryConfirm.nomineeToken = _metadata.getNomineeToken();
        recoveryConfirm.releasedIdx = _metadata.getLastReleased();
        requestIds[replicaId.u8] = request.msg.id;
    }
}

void LeaderElection::_tryBecomeLeader() {
    if (!ReqResp::isQuorum(_electionState->requestIds)) {
        return;
    }
    auto nomineeToken = _metadata.getNomineeToken();
    ALWAYS_ASSERT(nomineeToken.replica() == _replicaId);
    LOG_INFO(_env,"Became leader with token %s", nomineeToken);
    _state = LeadershipState::LEADER;
    ALWAYS_ASSERT(_metadata.updateLeaderToken(nomineeToken) == TernError::NO_ERROR);
    _clearElectionState();
}

void LeaderElection::_clearElectionState() {
    _leaderLastActive = ternNow();
    if (!_electionState) {
        return;
    }
    _reqResp.cleanupRequests(_electionState->requestIds);
    _clearRecoveryRequests();
    _electionState.reset();
}

void LeaderElection::_clearRecoveryRequests() {
    for(auto& requestIds : _electionState->recoveryRequests) {
        _reqResp.cleanupRequests(requestIds);
    }
}
