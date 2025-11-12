// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "LogsDB.hpp"

#include <algorithm>
#include <cstdint>
#include <limits>
#include <memory>
#include <rocksdb/comparator.h>
#include <rocksdb/db.h>
#include <rocksdb/iterator.h>
#include <rocksdb/options.h>
#include <rocksdb/slice.h>
#include <rocksdb/write_batch.h>
#include <unordered_map>
#include <vector>

#include "Assert.hpp"
#include "Msgs.hpp"
#include "RocksDBUtils.hpp"
#include "Time.hpp"
#include "logsdb/Appender.hpp"
#include "logsdb/BatchWriter.hpp"
#include "logsdb/CatchupReader.hpp"
#include "logsdb/DataPartitions.hpp"
#include "logsdb/LeaderElection.hpp"
#include "logsdb/LogMetadata.hpp"
#include "logsdb/LogsDBCommon.hpp"
#include "logsdb/LogsDBData.hpp"
#include "logsdb/ReqResp.hpp"

std::ostream& operator<<(std::ostream& out, const LogsDBLogEntry& entry) {
    out << entry.idx << ":";
    return goLangBytesFmt(out, (const char*)entry.value.data(), entry.value.size());
}

std::ostream& operator<<(std::ostream& out, const LogsDBRequest& entry) {
    return out << "replicaId: " << entry.replicaId << "[ " << entry.msg << "]";
}

std::ostream& operator<<(std::ostream& out, const LogsDBResponse& entry) {
    return out << "replicaId: " << entry.replicaId << "[ " << entry.msg << "]";
}

std::vector<rocksdb::ColumnFamilyDescriptor> LogsDB::getColumnFamilyDescriptors() {
    return {
        {METADATA_CF_NAME,{}},
        {DATA_PARTITION_0_NAME,{}},
        {DATA_PARTITION_1_NAME,{}},
    };
}

void LogsDB::clearAllData(SharedRocksDB &shardDB) {
    shardDB.deleteCF(METADATA_CF_NAME);
    shardDB.deleteCF(DATA_PARTITION_0_NAME);
    shardDB.deleteCF(DATA_PARTITION_1_NAME);
    shardDB.db()->FlushWAL(true);
}

LogsDB::LogsDB(
        Logger& logger,
        std::shared_ptr<XmonAgent>& xmon,
        SharedRocksDB& sharedDB,
        ReplicaId replicaId,
        LogIdx lastRead,
        bool noReplication,
        bool avoidBeingLeader)
:
    _env(logger, xmon, "LogsDB"),
    _db(sharedDB.db()),
    _replicaId(replicaId),
    _stats()
{
    LOG_INFO(_env, "Initializing LogsDB");
    
    // Create components in order of dependencies
    _partitions = std::make_unique<DataPartitions>(_env, sharedDB);
    _metadata = std::make_unique<LogMetadata>(_env, _stats, sharedDB, replicaId, *_partitions);
    _reqResp = std::make_unique<ReqResp>(_stats);
    _leaderElection = std::make_unique<LeaderElection>(_env, _stats, noReplication, avoidBeingLeader, replicaId, *_metadata, *_partitions, *_reqResp);
    _batchWriter = std::make_unique<BatchWriter>(_env, *_reqResp, *_leaderElection);
    _catchupReader = std::make_unique<CatchupReader>(_stats, *_reqResp, *_metadata, *_partitions, replicaId, lastRead);
    _appender = std::make_unique<Appender>(_env, _stats, *_reqResp, *_metadata, *_leaderElection, noReplication);

    auto initialStart = _metadata->isInitialStart() && _partitions->isInitialStart();
    if (initialStart) {
        LOG_INFO(_env, "Initial start of LogsDB");
    }

    auto initSuccess = _metadata->init(initialStart);
    initSuccess = _partitions->init(initialStart) && initSuccess;

    ALWAYS_ASSERT(initSuccess, "Failed to init LogsDB, check if you need to run with \"initialStart\" flag!");
    ALWAYS_ASSERT(lastRead <= _metadata->getLastReleased());
    flush(true);
    _catchupReader->init();

    LOG_INFO(_env,"LogsDB opened, leaderToken(%s), lastReleased(%s), lastRead(%s)",_metadata->getLeaderToken(), _metadata->getLastReleased(), _catchupReader->lastRead());
    _infoLoggedTime = ternNow();
    _lastLoopFinished = ternNow();
}

LogsDB::~LogsDB() {
    close();
}

void LogsDB::close() {
    LOG_INFO(_env,"closing LogsDB, leaderToken(%s), lastReleased(%s), lastRead(%s)", _metadata->getLeaderToken(), _metadata->getLastReleased(), _catchupReader->lastRead());
}

void LogsDB::flush(bool sync) {
    ROCKS_DB_CHECKED(_db->FlushWAL(sync));
}

void LogsDB::processIncomingMessages(std::vector<LogsDBRequest>& requests, std::vector<LogsDBResponse>& responses) {
    auto processingStarted = ternNow();
    _maybeLogStatus(processingStarted);
    for(auto& resp : responses) {
        auto request = _reqResp->getRequest(resp.msg.id);
        if (request == nullptr) {
            // We often don't care about all responses and remove requests as soon as we can make progress
            continue;
        }

        // Mismatch in responses could be due to network issues we don't want to crash but we will ignore and retry
        // Mismatch in internal state is asserted on.
        if (unlikely(request->replicaId != resp.replicaId)) {
            LOG_ERROR(_env, "Expected response from replica %s, got it from replica %s. Response: %s", request->replicaId, resp.msg.id, resp);
            continue;
        }
        if (unlikely(request->msg.body.kind() != resp.msg.body.kind())) {
            LOG_ERROR(_env, "Expected response of type %s, got type %s. Response: %s", request->msg.body.kind(), resp.msg.body.kind(), resp);
            continue;
        }
        LOG_TRACE(_env, "processing %s", resp);

        switch(resp.msg.body.kind()) {
        case LogMessageKind::RELEASE:
            // We don't track release requests. This response is unexpected
        case LogMessageKind::ERROR:
            LOG_ERROR(_env, "Bad response %s", resp);
            break;
        case LogMessageKind::LOG_WRITE:
            _appender->proccessLogWriteResponse(request->replicaId, *request, resp.msg.body.getLogWrite());
            break;
        case LogMessageKind::LOG_READ:
            _catchupReader->proccessLogReadResponse(request->replicaId, *request, resp.msg.body.getLogRead());
            break;
        case LogMessageKind::NEW_LEADER:
            _leaderElection->proccessNewLeaderResponse(request->replicaId, *request, resp.msg.body.getNewLeader());
            break;
        case LogMessageKind::NEW_LEADER_CONFIRM:
            _leaderElection->proccessNewLeaderConfirmResponse(request->replicaId, *request, resp.msg.body.getNewLeaderConfirm());
            break;
        case LogMessageKind::LOG_RECOVERY_READ:
            _leaderElection->proccessRecoveryReadResponse(request->replicaId, *request, resp.msg.body.getLogRecoveryRead());
            break;
        case LogMessageKind::LOG_RECOVERY_WRITE:
            _leaderElection->proccessRecoveryWriteResponse(request->replicaId, *request, resp.msg.body.getLogRecoveryWrite());
            break;
        case LogMessageKind::EMPTY:
            ALWAYS_ASSERT("LogMessageKind::EMPTY should not happen");
          break;
        }
    }
    for(auto& req : requests) {
        switch (req.msg.body.kind()) {
        case LogMessageKind::ERROR:
            LOG_ERROR(_env, "Bad request %s", req);
            break;
        case LogMessageKind::LOG_WRITE:
            _batchWriter->proccessLogWriteRequest(req);
            break;
        case LogMessageKind::RELEASE:
            _batchWriter->proccessReleaseRequest(req.replicaId, req.msg.id, req.msg.body.getRelease());
            break;
        case LogMessageKind::LOG_READ:
            _catchupReader->proccessLogReadRequest(req.replicaId, req.msg.id, req.msg.body.getLogRead());
            break;
        case LogMessageKind::NEW_LEADER:
            _leaderElection->proccessNewLeaderRequest(req.replicaId, req.msg.id, req.msg.body.getNewLeader());
            break;
        case LogMessageKind::NEW_LEADER_CONFIRM:
            _leaderElection->proccessNewLeaderConfirmRequest(req.replicaId, req.msg.id, req.msg.body.getNewLeaderConfirm());
            break;
        case LogMessageKind::LOG_RECOVERY_READ:
            _leaderElection->proccessRecoveryReadRequest(req.replicaId, req.msg.id, req.msg.body.getLogRecoveryRead());
            break;
        case LogMessageKind::LOG_RECOVERY_WRITE:
            _leaderElection->proccessRecoveryWriteRequest(req.replicaId, req.msg.id, req.msg.body.getLogRecoveryWrite());
            break;
        case LogMessageKind::EMPTY:
            ALWAYS_ASSERT("LogMessageKind::EMPTY should not happen");
          break;
        }
    }
    _leaderElection->maybeStartLeaderElection();
    _batchWriter->writeBatch();
    _appender->maybeMoveRelease();
    _catchupReader->maybeCatchUp();
    _reqResp->resendTimedOutRequests();
    update_atomic_stat_ema(_stats.requestsReceived, requests.size());
    update_atomic_stat_ema(_stats.responsesReceived, responses.size());
    update_atomic_stat_ema(_stats.appendWindow, _appender->entriesInFlight());
    _stats.isLeader.store(_leaderElection->isLeader(), std::memory_order_relaxed);
    responses.clear();
    requests.clear();
    update_atomic_stat_ema(_stats.idleTime, processingStarted - _lastLoopFinished);
    _lastLoopFinished = ternNow();
    update_atomic_stat_ema(_stats.processingTime, _lastLoopFinished - processingStarted);
}

void LogsDB::getOutgoingMessages(std::vector<LogsDBRequest*>& requests, std::vector<LogsDBResponse>& responses) {
    _reqResp->getResponsesToSend(responses);
    _reqResp->getRequestsToSend(requests);
}

bool LogsDB::isLeader() const {
    return _leaderElection->isLeader();
}

TernError LogsDB::appendEntries(std::vector<LogsDBLogEntry>& entries) {
    return _appender->appendEntries(entries);
}

LogIdx LogsDB::getLastContinuous() const {
    return _catchupReader->getLastContinuous();
}

void LogsDB::readEntries(std::vector<LogsDBLogEntry>& entries, size_t maxEntries) {
    _catchupReader->readEntries(entries, maxEntries);
}

void LogsDB::readIndexedEntries(const std::vector<LogIdx> &indices, std::vector<LogsDBLogEntry> &entries) const {
    _partitions->readIndexedEntries(indices, entries);
}

Duration LogsDB::getNextTimeout() const {
    return _reqResp->getNextTimeout();
}

LogIdx LogsDB::getLastReleased() const {
    return _metadata->getLastReleased();
}

LogIdx LogsDB::getHeadIdx() const {
    return _partitions->getLowestKey();
}

const LogsDBStats& LogsDB::getStats() const {
    return _stats;
}

void LogsDB::_maybeLogStatus(TernTime now) {
    if (now - _infoLoggedTime > 1_mins) {
        LOG_INFO(_env,"LogsDB status: leaderToken(%s), lastReleased(%s), lastRead(%s)",_metadata->getLeaderToken(), _metadata->getLastReleased(), _catchupReader->lastRead());
        _infoLoggedTime = now;
    }
}

void LogsDB::_getUnreleasedLogEntries(Env& env, SharedRocksDB& sharedDB, LogIdx& lastReleasedOut, std::vector<LogIdx>& unreleasedLogEntriesOut)  {
    DataPartitions data(env, sharedDB);
    bool initSuccess = data.init(false);
    LogsDBStats stats;
    LogMetadata metadata(env, stats, sharedDB, 0, data);
    initSuccess = initSuccess && metadata.init(false);
    ALWAYS_ASSERT(initSuccess, "Failed to init LogsDB, check if you need to run with \"initialStart\" flag!");
    lastReleasedOut = metadata.getLastReleased();

    auto it = data.getIterator();
    it.seek(lastReleasedOut + 1);
    while(it.valid()) {
        unreleasedLogEntriesOut.emplace_back(it.key());
        ++it;
    }
}

void LogsDB::_getLogEntries(Env& env, SharedRocksDB& sharedDB, LogIdx start, size_t count, std::vector<LogsDBLogEntry>& logEntriesOut) {
    logEntriesOut.clear();
    DataPartitions data(env, sharedDB);
    bool initSuccess = data.init(false);
    ALWAYS_ASSERT(initSuccess, "Failed to init LogsDB, check if you need to run with \"initialStart\" flag!");
    auto it = data.getIterator();
    it.seek(start);
    while(it.valid() && logEntriesOut.size() < count) {
        logEntriesOut.emplace_back(it.entry());
        ++it;
    }
}
