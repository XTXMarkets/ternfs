// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "CatchupReader.hpp"

#include "../Assert.hpp"
#include "../LogsDB.hpp"
#include "../Msgs.hpp"
#include "LogsDBCommon.hpp"

CatchupReader::CatchupReader(LogsDBStats& stats, ReqResp& reqResp, LogMetadata& metadata, DataPartitions& data, ReplicaId replicaId, LogIdx lastRead) :
    _stats(stats),
    _reqResp(reqResp),
    _metadata(metadata),
    _data(data),
    _replicaId(replicaId),
    _lastRead(lastRead),
    _lastContinuousIdx(lastRead),
    _lastMissingIdx(lastRead) {}

LogIdx CatchupReader::getLastContinuous() const {
    return _lastContinuousIdx;
}

void CatchupReader::readEntries(std::vector<LogsDBLogEntry>& entries, size_t maxEntries) {
    if (_lastRead == _lastContinuousIdx) {
        update_atomic_stat_ema(_stats.entriesRead, (uint64_t)0);
        return;
    }
    auto lastReleased = _metadata.getLastReleased();
    auto startIndex = _lastRead;
    ++startIndex;

    auto it = _data.getIterator();
    for (it.seek(startIndex); it.valid(); it.next(), ++startIndex) {
        if (_lastContinuousIdx < it.key() || entries.size() >= maxEntries) {
            break;
        }
        ALWAYS_ASSERT(startIndex == it.key());
        entries.emplace_back(it.entry());
        _lastRead = startIndex;
    }
    update_atomic_stat_ema(_stats.entriesRead, entries.size());
    update_atomic_stat_ema(_stats.readerLag, lastReleased.u64 - _lastRead.u64);
}

void CatchupReader::init() {
    _missingEntries.reserve(LogsDBConsts::CATCHUP_WINDOW);
    _requestIds.reserve(LogsDBConsts::CATCHUP_WINDOW);
    _findMissingEntries();
}

void CatchupReader::maybeCatchUp() {
    for (auto idx : _missingEntries) {
        if (idx != 0) {
            _populateStats();
            return;
        }
    }
    _lastContinuousIdx = _lastMissingIdx;
    _missingEntries.clear();
    _requestIds.clear();
    _findMissingEntries();
    _populateStats();
}


void CatchupReader::proccessLogReadRequest(ReplicaId fromReplicaId, uint64_t requestId, const LogReadReq& request) {
    auto& response = _reqResp.newResponse(fromReplicaId, requestId);
    auto& readResponse = response.msg.body.setLogRead();
    if (_metadata.getLastReleased() < request.idx) {
        readResponse.result = TernError::LOG_ENTRY_UNRELEASED;
        return;
    }
    LogsDBLogEntry entry;
    auto err =_data.readLogEntry(request.idx, entry);
    readResponse.result = err;
    if (err == TernError::NO_ERROR) {
        readResponse.value.els = entry.value;
    }
}

void CatchupReader::proccessLogReadResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogReadResp& response) {
    if (response.result != TernError::NO_ERROR) {
        return;
    }

    auto idx = request.msg.body.getLogRead().idx;

    size_t i = 0;
    for (; i < _missingEntries.size(); ++i) {
        if (_missingEntries[i] == idx) {
            _missingEntries[i] = 0;
            break;
        }
    }

    if (i == _missingEntries.size()) {
        return;
    }
    _reqResp.cleanupRequests(_requestIds[i]);
    LogsDBLogEntry entry;
    entry.idx = idx;
    entry.value = response.value.els;
    _data.writeLogEntry(entry);
}

LogIdx CatchupReader::lastRead() const {
    return _lastRead;
}

void CatchupReader::_populateStats() {
    update_atomic_stat_ema(_stats.followerLag, _metadata.getLastReleased().u64 - _lastContinuousIdx.u64);
    update_atomic_stat_ema(_stats.catchupWindow, _missingEntries.size());
}

void CatchupReader::_findMissingEntries() {
    if (!_missingEntries.empty()) {
        return;
    }
    auto lastReleased = _metadata.getLastReleased();
    if (unlikely(_metadata.getLastReleased() <= _lastRead)) {
        return;
    }
    auto it = _data.getIterator();
    auto startIdx = _lastContinuousIdx;
    it.seek(++startIdx);
    while (startIdx <= lastReleased && _missingEntries.size() < LogsDBConsts::CATCHUP_WINDOW) {
        if(!it.valid() || startIdx < it.key() ) {
            _missingEntries.emplace_back(startIdx);
        } else {
            ++it;
        }
        ++startIdx;
    }

    if (_missingEntries.empty()) {
        _lastContinuousIdx = _lastMissingIdx = lastReleased;
        return;
    }

    _lastContinuousIdx = _missingEntries.front().u64 - 1;
    _lastMissingIdx = _missingEntries.back();

    for(auto logIdx : _missingEntries) {
        _requestIds.emplace_back();
        auto& requests = _requestIds.back();
        for (ReplicaId replicaId = 0; replicaId.u8 < LogsDBConsts::REPLICA_COUNT; ++replicaId.u8 ) {
            if (replicaId == _replicaId) {
                requests[replicaId.u8] = 0;
                continue;
            }
            auto& request = _reqResp.newRequest(replicaId);
            auto& readRequest  = request.msg.body.setLogRead();
            readRequest.idx = logIdx;
            requests[replicaId.u8] = request.msg.id;
        }
    }
}
