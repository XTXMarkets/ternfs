// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "LogsDB.hpp"

#include <algorithm>
#include <cstdint>
#include <memory>
#include <rocksdb/db.h>
#include <rocksdb/options.h>
#include <vector>

#include "Assert.hpp"
#include "logsdb/Appender.hpp"
#include "logsdb/BatchWriter.hpp"
#include "logsdb/CatchupReader.hpp"
#include "logsdb/DataPartitions.hpp"
#include "logsdb/LeaderElection.hpp"
#include "logsdb/LogMetadata.hpp"
#include "logsdb/ReqResp.hpp"
#include "Msgs.hpp"
#include "RocksDBUtils.hpp"
#include "Time.hpp"

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

static void update_atomic_stat_ema(std::atomic<double>& stat, double newValue) {
    stat.store((stat.load(std::memory_order_relaxed)* 0.95 + newValue * 0.05), std::memory_order_relaxed);
}

static void update_atomic_stat_ema(std::atomic<Duration>& stat, Duration newValue) {
    stat.store((Duration)((double)stat.load(std::memory_order_relaxed).ns * 0.95 + (double)newValue.ns * 0.05), std::memory_order_relaxed);
}

std::vector<rocksdb::ColumnFamilyDescriptor> LogsDB::getColumnFamilyDescriptors() {
    return {
        {DataPartitions::METADATA_CF_NAME,{}},
        {DataPartitions::DATA_PARTITION_0_NAME,{}},
        {DataPartitions::DATA_PARTITION_1_NAME,{}},
    };
}

void LogsDB::clearAllData(SharedRocksDB &shardDB) {
    shardDB.deleteCF(DataPartitions::METADATA_CF_NAME);
    shardDB.deleteCF(DataPartitions::DATA_PARTITION_0_NAME);
    shardDB.deleteCF(DataPartitions::DATA_PARTITION_1_NAME);
    shardDB.db()->FlushWAL(true);
}

class LogsDBImpl {
public:
    LogsDBImpl(
        Logger& logger,
        std::shared_ptr<XmonAgent>& xmon,
        SharedRocksDB& sharedDB,
        ReplicaId replicaId,
        LogIdx lastRead,
        bool noReplication,
        bool skipLeaderElection,
        bool avoidBeingLeader)
    :
        _env(logger, xmon, "LogsDB"),
        _db(sharedDB.db()),
        _replicaId(replicaId),
        _stats(),
        _partitions(_env,sharedDB),
        _metadata(_env,_stats, sharedDB, replicaId, _partitions),
        _reqResp(_stats),
        _leaderElection(_env, _stats, noReplication, skipLeaderElection, avoidBeingLeader, replicaId, _metadata, _partitions, _reqResp),
        _batchWriter(_env,_reqResp, _leaderElection),
        _catchupReader(_stats, _reqResp, _metadata, _partitions, replicaId, lastRead),
        _appender(_env, _stats, _reqResp, _metadata, _leaderElection, noReplication)
    {
        LOG_INFO(_env, "Initializing LogsDB");
        auto initialStart = _metadata.isInitialStart() && _partitions.isInitialStart();
        if (initialStart) {
            LOG_INFO(_env, "Initial start of LogsDB");
        }

        auto initSuccess = _metadata.init(initialStart);
        initSuccess = _partitions.init(initialStart) && initSuccess;

        ALWAYS_ASSERT(initSuccess, "Failed to init LogsDB, check if you need to run with \"initialStart\" flag!");
        ALWAYS_ASSERT(lastRead <= _metadata.getLastReleased());
        flush(true);
        _catchupReader.init();

        LOG_INFO(_env,"LogsDB opened, leaderToken(%s), lastReleased(%s), lastRead(%s)",_metadata.getLeaderToken(), _metadata.getLastReleased(), _catchupReader.lastRead());
        _infoLoggedTime = ternNow();
        _lastLoopFinished = ternNow();
    }

    ~LogsDBImpl() {
        close();
    }

    void close() {
        LOG_INFO(_env,"closing LogsDB, leaderToken(%s), lastReleased(%s), lastRead(%s)", _metadata.getLeaderToken(), _metadata.getLastReleased(), _catchupReader.lastRead());
    }

    LogIdx appendLogEntries(std::vector<LogsDBLogEntry>& entries) {
        ALWAYS_ASSERT(_metadata.getLeaderToken().replica() == _replicaId);
        if (unlikely(entries.size() == 0)) {
            return 0;
        }

        for (auto& entry : entries) {
            entry.idx = _metadata.assignLogIdx();
        }
        auto firstAssigned = entries.front().idx;
        ALWAYS_ASSERT(_metadata.getLastReleased() < firstAssigned);
        _partitions.writeLogEntries(entries);
        return firstAssigned;
    }

    void flush(bool sync) {
        ROCKS_DB_CHECKED(_db->FlushWAL(sync));
    }

    void processIncomingMessages(std::vector<LogsDBRequest>& requests, std::vector<LogsDBResponse>& responses) {
        auto processingStarted = ternNow();
        _maybeLogStatus(processingStarted);
        for(auto& resp : responses) {
            auto request = _reqResp.getRequest(resp.msg.id);
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
                _appender.proccessLogWriteResponse(request->replicaId, *request, resp.msg.body.getLogWrite());
                break;
            case LogMessageKind::LOG_READ:
                _catchupReader.proccessLogReadResponse(request->replicaId, *request, resp.msg.body.getLogRead());
                break;
            case LogMessageKind::NEW_LEADER:
                _leaderElection.proccessNewLeaderResponse(request->replicaId, *request, resp.msg.body.getNewLeader());
                break;
            case LogMessageKind::NEW_LEADER_CONFIRM:
                _leaderElection.proccessNewLeaderConfirmResponse(request->replicaId, *request, resp.msg.body.getNewLeaderConfirm());
                break;
            case LogMessageKind::LOG_RECOVERY_READ:
                _leaderElection.proccessRecoveryReadResponse(request->replicaId, *request, resp.msg.body.getLogRecoveryRead());
                break;
            case LogMessageKind::LOG_RECOVERY_WRITE:
                _leaderElection.proccessRecoveryWriteResponse(request->replicaId, *request, resp.msg.body.getLogRecoveryWrite());
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
                _batchWriter.proccessLogWriteRequest(req);
                break;
            case LogMessageKind::RELEASE:
                _batchWriter.proccessReleaseRequest(req.replicaId, req.msg.id, req.msg.body.getRelease());
                break;
            case LogMessageKind::LOG_READ:
                _catchupReader.proccessLogReadRequest(req.replicaId, req.msg.id, req.msg.body.getLogRead());
                break;
            case LogMessageKind::NEW_LEADER:
                _leaderElection.proccessNewLeaderRequest(req.replicaId, req.msg.id, req.msg.body.getNewLeader());
                break;
            case LogMessageKind::NEW_LEADER_CONFIRM:
                _leaderElection.proccessNewLeaderConfirmRequest(req.replicaId, req.msg.id, req.msg.body.getNewLeaderConfirm());
                break;
            case LogMessageKind::LOG_RECOVERY_READ:
                _leaderElection.proccessRecoveryReadRequest(req.replicaId, req.msg.id, req.msg.body.getLogRecoveryRead());
                break;
            case LogMessageKind::LOG_RECOVERY_WRITE:
                _leaderElection.proccessRecoveryWriteRequest(req.replicaId, req.msg.id, req.msg.body.getLogRecoveryWrite());
                break;
            case LogMessageKind::EMPTY:
                ALWAYS_ASSERT("LogMessageKind::EMPTY should not happen");
              break;
            }
        }
        _leaderElection.maybeStartLeaderElection();
        _batchWriter.writeBatch();
        _appender.maybeMoveRelease();
        _catchupReader.maybeCatchUp();
        _reqResp.resendTimedOutRequests();
        update_atomic_stat_ema(_stats.requestsReceived, requests.size());
        update_atomic_stat_ema(_stats.responsesReceived, responses.size());
        update_atomic_stat_ema(_stats.appendWindow, _appender.entriesInFlight());
        _stats.isLeader.store(_leaderElection.isLeader(), std::memory_order_relaxed);
        responses.clear();
        requests.clear();
        update_atomic_stat_ema(_stats.idleTime, processingStarted - _lastLoopFinished);
        _lastLoopFinished = ternNow();
        update_atomic_stat_ema(_stats.processingTime, _lastLoopFinished - processingStarted);
    }

    void getOutgoingMessages(std::vector<LogsDBRequest*>& requests, std::vector<LogsDBResponse>& responses) {
        _reqResp.getResponsesToSend(responses);
        _reqResp.getRequestsToSend(requests);
    }

    bool isLeader() const {
        return _leaderElection.isLeader();
    }

    TernError appendEntries(std::vector<LogsDBLogEntry>& entries) {
        return _appender.appendEntries(entries);
    }

    LogIdx getLastContinuous() const {
        return _catchupReader.getLastContinuous();
    }

    void readEntries(std::vector<LogsDBLogEntry>& entries, size_t maxEntries) {
        _catchupReader.readEntries(entries, maxEntries);
    }

    void readIndexedEntries(const std::vector<LogIdx> &indices, std::vector<LogsDBLogEntry> &entries) const {
        _partitions.readIndexedEntries(indices, entries);
    }

    Duration getNextTimeout() const {
        return _reqResp.getNextTimeout();
    }

    LogIdx getLastReleased() const {
        return _metadata.getLastReleased();
    }

    LogIdx getHeadIdx() const {
        return _partitions.getLowestKey();
    }

    const LogsDBStats& getStats() const {
        return _stats;
    }

private:

    void _maybeLogStatus(TernTime now) {
        if (now - _infoLoggedTime > 1_mins) {
            LOG_INFO(_env,"LogsDB status: leaderToken(%s), lastReleased(%s), lastRead(%s)",_metadata.getLeaderToken(), _metadata.getLastReleased(), _catchupReader.lastRead());
            _infoLoggedTime = now;
        }
    }

    Env _env;
    rocksdb::DB* _db;
    const ReplicaId _replicaId;
    LogsDBStats _stats;
    DataPartitions _partitions;
    LogMetadata _metadata;
    ReqResp _reqResp;
    LeaderElection _leaderElection;
    BatchWriter _batchWriter;
    CatchupReader _catchupReader;
    Appender _appender;
    TernTime _infoLoggedTime;
    TernTime _lastLoopFinished;
};

LogsDB::LogsDB(
        Logger& logger,
        std::shared_ptr<XmonAgent>& xmon,
        SharedRocksDB& sharedDB,
        ReplicaId replicaId,
        LogIdx lastRead,
        bool noReplication,
        bool skipLeaderElection,
        bool avoidBeingLeader)
{
    _impl = new LogsDBImpl(logger, xmon, sharedDB, replicaId, lastRead, noReplication, skipLeaderElection, avoidBeingLeader);
}

LogsDB::~LogsDB() {
    delete _impl;
    _impl = nullptr;
}

void LogsDB::close() {
    _impl->close();
}

void LogsDB::flush(bool sync) {
    _impl->flush(sync);
}

void LogsDB::processIncomingMessages(std::vector<LogsDBRequest>& requests, std::vector<LogsDBResponse>& responses) {
    _impl->processIncomingMessages(requests, responses);
}

void LogsDB::getOutgoingMessages(std::vector<LogsDBRequest*>& requests, std::vector<LogsDBResponse>& responses) {
    _impl->getOutgoingMessages(requests, responses);
}

bool LogsDB::isLeader() const {
    return _impl->isLeader();
}

TernError LogsDB::appendEntries(std::vector<LogsDBLogEntry>& entries) {
    return _impl->appendEntries(entries);
}

LogIdx LogsDB::getLastContinuous() const {
    return _impl->getLastContinuous();
}

void LogsDB::readEntries(std::vector<LogsDBLogEntry>& entries, size_t maxEntries) {
    _impl->readEntries(entries, maxEntries);
}

void LogsDB::readIndexedEntries(const std::vector<LogIdx> &indices, std::vector<LogsDBLogEntry> &entries) const {
    _impl->readIndexedEntries(indices, entries);
}

Duration LogsDB::getNextTimeout() const {
    return _impl->getNextTimeout();
}

LogIdx LogsDB::getLastReleased() const {
    return _impl->getLastReleased();
}

LogIdx LogsDB::getHeadIdx() const {
    return _impl->getHeadIdx();
}

const LogsDBStats& LogsDB::getStats() const {
    return _impl->getStats();
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
