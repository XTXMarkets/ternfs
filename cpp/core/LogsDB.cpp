// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "LogsDB.hpp"

#include <algorithm>
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
#include "LogsDBData.hpp"
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

static bool tryGet(rocksdb::DB* db, rocksdb::ColumnFamilyHandle* cf, const rocksdb::Slice& key, std::string& value) {
    auto status = db->Get({}, cf, key, &value);
    if (status.IsNotFound()) {
        return false;
    }
    ROCKS_DB_CHECKED(status);
    return true;
};

static constexpr auto METADATA_CF_NAME = "logMetadata";
static constexpr auto DATA_PARTITION_0_NAME = "logTimePartition0";
static constexpr auto DATA_PARTITION_1_NAME = "logTimePartition1";


static void update_atomic_stat_ema(std::atomic<double>& stat, double newValue) {
    stat.store((stat.load(std::memory_order_relaxed)* 0.95 + newValue * 0.05), std::memory_order_relaxed);
}

static void update_atomic_stat_ema(std::atomic<Duration>& stat, Duration newValue) {
    stat.store((Duration)((double)stat.load(std::memory_order_relaxed).ns * 0.95 + (double)newValue.ns * 0.05), std::memory_order_relaxed);
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

struct LogPartition {
    std::string name;
    LogsDBMetadataKey firstWriteKey;
    rocksdb::ColumnFamilyHandle* cf;
    TernTime firstWriteTime{0};
    LogIdx minKey{0};
    LogIdx maxKey{0};

    void reset(rocksdb::ColumnFamilyHandle* cf_, LogIdx minMaxKey, TernTime firstWriteTime_) {
        cf = cf_;
        minKey = maxKey = minMaxKey;
        firstWriteTime = firstWriteTime_;
    }
};

class DataPartitions {
public:
    class Iterator {
        public:
            Iterator(const DataPartitions& partitions) : _partitions(partitions), _rotationCount(_partitions._rotationCount), _smaller(nullptr) {
                _iterators = _partitions._getPartitionIterators();
            }

            void seek(LogIdx idx) {
                if (unlikely(_rotationCount != _partitions._rotationCount)) {
                    _iterators = _partitions._getPartitionIterators();
                }
                auto key = U64Key::Static(idx.u64);
                for (auto& it : _iterators) {
                    it->Seek(key.toSlice());
                }
                _updateSmaller();
            }

            bool valid() const {
                return _smaller != nullptr;
            }

            void next() {
                if (_smaller != nullptr) {
                    _smaller->Next();
                }
                _updateSmaller();
            }
            Iterator& operator++() {
                this->next();
                return *this;
            }

            LogIdx key() const {
                return LogIdx(ExternalValue<U64Key>::FromSlice(_smaller->key())().u64());
            }

            LogsDBLogEntry entry() const {
                auto value = _smaller->value();
                return LogsDBLogEntry{key(), {(const uint8_t*)value.data(), (const uint8_t*)value.data() + value.size()}};
            }

            void dropEntry() {
                ALWAYS_ASSERT(_rotationCount == _partitions._rotationCount);
                auto cfIdx = _cfIndexForCurrentIterator();
                ROCKS_DB_CHECKED(_partitions._sharedDb.db()->Delete({}, _partitions._partitions[cfIdx].cf, _smaller->key()));
            }

        private:
            void _updateSmaller() {
                _smaller = nullptr;
                for (auto& it : _iterators) {
                    if (!it->Valid()) {
                        continue;
                    }
                    if (_smaller == nullptr || (rocksdb::BytewiseComparator()->Compare(it->key(),_smaller->key()) < 0)) {
                        _smaller = it.get();
                    }
                }
            }
            size_t _cfIndexForCurrentIterator() const {
                for (size_t i = 0; i < _iterators.size(); ++i) {
                    if (_smaller == _iterators[i].get()) {
                        return i;
                    }
                }
                return -1;
            }
            const DataPartitions& _partitions;
            size_t _rotationCount;
            rocksdb::Iterator* _smaller;
            std::vector<std::unique_ptr<rocksdb::Iterator>> _iterators;
    };

    DataPartitions(Env& env, SharedRocksDB& sharedDB)
    :
        _env(env),
        _sharedDb(sharedDB),
        _rotationCount(0),
        _partitions({
            LogPartition{
                    DATA_PARTITION_0_NAME,
                    LogsDBMetadataKey::PARTITION_0_FIRST_WRITE_TIME,
                    sharedDB.getCF(DATA_PARTITION_0_NAME),
                    0,
                    0,
                    0
                },
            LogPartition{
                    DATA_PARTITION_1_NAME,
                    LogsDBMetadataKey::PARTITION_1_FIRST_WRITE_TIME,
                    sharedDB.getCF(DATA_PARTITION_1_NAME),
                    0,
                    0,
                    0
                }
            })
    {}

    bool isInitialStart() {
        auto it1 = std::unique_ptr<rocksdb::Iterator>(_sharedDb.db()->NewIterator({},_partitions[0].cf));
        auto it2 = std::unique_ptr<rocksdb::Iterator>(_sharedDb.db()->NewIterator({},_partitions[1].cf));
        it1->SeekToFirst();
        it2->SeekToFirst();
        return !(it1->Valid() || it2->Valid());
    }

    bool init(bool initialStart) {
        bool initSuccess = true;
        auto metadataCF = _sharedDb.getCF(METADATA_CF_NAME);
        auto db = _sharedDb.db();
        std::string value;
        for (auto& partition : _partitions) {
            if (tryGet(db, metadataCF, logsDBMetadataKey(partition.firstWriteKey), value)) {
                partition.firstWriteTime = ExternalValue<U64Value>::FromSlice(value)().u64();
                LOG_INFO(_env, "Loaded partition %s first write time %s", partition.name, partition.firstWriteTime);
            } else if (initialStart) {
                LOG_INFO(_env, "Partition %s first write time not found. Using %s", partition.name, partition.firstWriteTime);
                _updatePartitionFirstWriteTime(partition, partition.firstWriteTime);
            } else {
                initSuccess = false;
                LOG_ERROR(_env, "Partition %s first write time not found. Possible DB corruption!", partition.name);
            }
        }
        {
            auto partitionIterators = _getPartitionIterators();
            for (size_t i = 0; i < partitionIterators.size(); ++i) {
                auto it = partitionIterators[i].get();
                auto& partition = _partitions[i];
                it->SeekToFirst();
                if (!it->Valid()) {
                    if (partition.firstWriteTime != 0) {
                        LOG_ERROR(_env, "No keys found in partition %s, but first write time is %s. DB Corruption!", partition.name, partition.firstWriteTime);
                        initSuccess = false;
                    } else {
                        LOG_INFO(_env, "Partition %s empty.", partition.name);
                    }
                    continue;
                }
                partition.minKey = ExternalValue<U64Key>::FromSlice(it->key())().u64();
                it->SeekToLast();
                // If at least one key exists seeking to last should never fail.
                ROCKS_DB_CHECKED(it->status());
                partition.maxKey = ExternalValue<U64Key>::FromSlice(it->key())().u64();
            }
        }
        return initSuccess;
    }

    Iterator getIterator() const {
        return Iterator(*this);
    }

    TernError readLogEntry(LogIdx logIdx, LogsDBLogEntry& entry) const {
        auto& partition = _getPartitionForIdx(logIdx);
        if (unlikely(logIdx < partition.minKey)) {
            return TernError::LOG_ENTRY_TRIMMED;
        }

        auto key = U64Key::Static(logIdx.u64);
        rocksdb::PinnableSlice value;
        auto status = _sharedDb.db()->Get({}, partition.cf, key.toSlice(), &value);
        if (status.IsNotFound()) {
            return TernError::LOG_ENTRY_MISSING;
        }
        ROCKS_DB_CHECKED(status);
        entry.idx = logIdx;
        entry.value.assign((const uint8_t*)value.data(), (const uint8_t*)value.data() + value.size());
        return TernError::NO_ERROR;
    }

    void readIndexedEntries(const std::vector<LogIdx>& indices, std::vector<LogsDBLogEntry>& entries) const {
        entries.clear();
        if (indices.empty()) {
            return;
        }
        // TODO: This is not very efficient as we're doing a lookup for each index.
        entries.reserve(indices.size());
        for (auto idx : indices) {
            LogsDBLogEntry& entry = entries.emplace_back();
            if (readLogEntry(idx, entry) != TernError::NO_ERROR) {
                entry.idx = 0;
            }
        }
    }

    void writeLogEntries(const std::vector<LogsDBLogEntry>& entries) {
        _maybeRotate();

        rocksdb::WriteBatch batch;
        std::vector<StaticValue<U64Key>> keys;
        keys.reserve(entries.size());
        for (const auto& entry : entries) {
            auto& partition = _getPartitionForIdx(entry.idx);
            keys.emplace_back(U64Key::Static(entry.idx.u64));
            batch.Put(partition.cf, keys.back().toSlice(), rocksdb::Slice((const char*)entry.value.data(), entry.value.size()));
            _partitionKeyInserted(partition, entry.idx);
        }
        ROCKS_DB_CHECKED(_sharedDb.db()->Write({}, &batch));
    }

    void writeLogEntry(const LogsDBLogEntry& entry) {
        _maybeRotate();

        auto& partition = _getPartitionForIdx(entry.idx);
        _sharedDb.db()->Put({}, partition.cf, U64Key::Static(entry.idx.u64).toSlice(), rocksdb::Slice((const char*)entry.value.data(), entry.value.size()));
        _partitionKeyInserted(partition, entry.idx);

    }

    void dropEntriesAfterIdx(LogIdx start) {
        auto iterator = getIterator();
        size_t droppedEntriesCount = 0;
        for (iterator.seek(start), iterator.next(); iterator.valid(); ++iterator) {
            iterator.dropEntry();
            ++droppedEntriesCount;
        }
        LOG_INFO(_env,"Dropped %s entries after %s", droppedEntriesCount, start);
    }

    LogIdx getHighestKey() const {
        return std::max(_partitions[0].maxKey, _partitions[1].maxKey);
    }

private:
    void _updatePartitionFirstWriteTime(LogPartition& partition, TernTime time) {
        ROCKS_DB_CHECKED(_sharedDb.db()->Put({}, _sharedDb.getCF(METADATA_CF_NAME), logsDBMetadataKey(partition.firstWriteKey), U64Value::Static(time.ns).toSlice()));
        partition.firstWriteTime = time;
    }

    std::vector<std::unique_ptr<rocksdb::Iterator>> _getPartitionIterators() const {
        std::vector<rocksdb::ColumnFamilyHandle*> cfHandles;
        cfHandles.reserve(_partitions.size());
        for (const auto& partition : _partitions) {
            cfHandles.emplace_back(partition.cf);
        }
        std::vector<std::unique_ptr<rocksdb::Iterator>> iterators;
        iterators.reserve(_partitions.size());
        ROCKS_DB_CHECKED(_sharedDb.db()->NewIterators({}, cfHandles, (std::vector<rocksdb::Iterator*>*)(&iterators)));
        return iterators;
    }

    void _maybeRotate() {
        auto& partition = _getPartitionForIdx(MAX_LOG_IDX);
        if (likely(partition.firstWriteTime == 0 || (partition.firstWriteTime + LogsDB::PARTITION_TIME_SPAN > ternNow()))) {
            return;
        }
        // we only need to drop older partition and reset it's info.
        // picking partition for writes/reads takes care of rest
        auto& olderPartition = _partitions[0].minKey < _partitions[1].minKey ? _partitions[0] : _partitions[1];
        LOG_INFO(_env, "Rotating partions. Dropping partition %s, firstWriteTime: %s, minKey: %s, maxKey: %s", olderPartition.name, olderPartition.firstWriteTime, olderPartition.minKey, olderPartition.maxKey);

        _sharedDb.deleteCF(olderPartition.name);
        olderPartition.reset(_sharedDb.createCF({olderPartition.name,{}}),0,0);
        _updatePartitionFirstWriteTime(olderPartition, 0);
        ++_rotationCount;
    }

    LogPartition& _getPartitionForIdx(LogIdx key) {
        return const_cast<LogPartition&>(static_cast<const DataPartitions*>(this)->_getPartitionForIdx(key));
    }

    const LogPartition& _getPartitionForIdx(LogIdx key) const {
        // This is a bit of a mess of ifs but I (mcrnic) am unsure how to do it better at this point.
        // Logic is roughly:
        // 1. If both are empty we return partition 0.
        // 2. If only 1 is empty then it's likely we just rotated and key will be larger than range of old partition so we return new one,
        // if it fits in old partition (we are backfilling missed data) we returned the old one
        // 3. Both contain data, likely the key is in newer partition (newerPartition.minKey) <= key
        // Note that there is inefficiency in case of empty DB where first key will be written in partition 0 and second one will immediately go to partition 1
        // This is irrelevant from correctness of rotation/retention perspective and will be ignored.
        if (unlikely(_partitions[0].firstWriteTime == 0 && _partitions[1].firstWriteTime == 0)) {
            return _partitions[0];
        }
        if (unlikely(_partitions[0].firstWriteTime == 0)) {
            if (likely(_partitions[1].maxKey < key)) {
                return _partitions[0];
            }
            return _partitions[1];
        }
        if (unlikely(_partitions[1].firstWriteTime == 0)) {
            if (likely(_partitions[0].maxKey < key)) {
                return _partitions[1];
            }
            return _partitions[0];
        }
        int newerPartitionIdx = _partitions[0].minKey < _partitions[1].minKey ? 1 : 0;
        if (likely(_partitions[newerPartitionIdx].minKey <= key)) {
            return _partitions[newerPartitionIdx];
        }

        return _partitions[newerPartitionIdx ^ 1];
    }

    void _partitionKeyInserted(LogPartition& partition, LogIdx idx) {
        if (unlikely(partition.minKey == 0)) {
            partition.minKey = idx;
            _updatePartitionFirstWriteTime(partition, ternNow());
        }
        partition.minKey = std::min(partition.minKey, idx);
        partition.maxKey = std::max(partition.maxKey, idx);
    }

    Env& _env;
    SharedRocksDB& _sharedDb;
    size_t _rotationCount;
    std::array<LogPartition, 2> _partitions;
};

class LogMetadata {
public:
    LogMetadata(Env& env, LogsDBStats& stats, SharedRocksDB& sharedDb, ReplicaId replicaId, DataPartitions& data) :
        _env(env),
        _stats(stats),
        _sharedDb(sharedDb),
        _cf(sharedDb.getCF(METADATA_CF_NAME)),
        _replicaId(replicaId),
        _data(data),
        _nomineeToken(LeaderToken(0,0))
    {}

    bool isInitialStart() {
        auto it = std::unique_ptr<rocksdb::Iterator>(_sharedDb.db()->NewIterator({},_cf));
        it->SeekToFirst();
        return !it->Valid();
    }

    bool init(bool initialStart) {
        bool initSuccess = true;
        std::string value;
        if (tryGet(_sharedDb.db(), _cf, logsDBMetadataKey(LEADER_TOKEN_KEY), value)) {
            _leaderToken.u64 = ExternalValue<U64Value>::FromSlice(value)().u64();
            LOG_INFO(_env, "Loaded leader token %s", _leaderToken);
        } else if (initialStart) {
            _leaderToken = LeaderToken(0,0);
            LOG_INFO(_env, "Leader token not found. Using %s", _leaderToken);
            ROCKS_DB_CHECKED(_sharedDb.db()->Put({}, _cf, logsDBMetadataKey(LEADER_TOKEN_KEY), U64Value::Static(_leaderToken.u64).toSlice()));
        } else {
            initSuccess = false;
            LOG_ERROR(_env, "Leader token not found! Possible DB corruption!");
        }

        if (tryGet(_sharedDb.db(), _cf, logsDBMetadataKey(LAST_RELEASED_IDX_KEY), value)) {
            _lastReleased = ExternalValue<U64Value>::FromSlice(value)().u64();
            LOG_INFO(_env, "Loaded last released %s", _lastReleased);
        } else if (initialStart) {
            LOG_INFO(_env, "Last released not found. Using %s", 0);
            setLastReleased(0);
        } else {
            initSuccess = false;
            LOG_ERROR(_env, "Last released not found! Possible DB corruption!");
        }

        if (tryGet(_sharedDb.db(),_cf, logsDBMetadataKey(LAST_RELEASED_TIME_KEY), value)) {
            _lastReleasedTime = ExternalValue<U64Value>::FromSlice(value)().u64();
            LOG_INFO(_env, "Loaded last released time %s", _lastReleasedTime);
        } else {
            initSuccess = false;
            LOG_ERROR(_env, "Last released time not found! Possible DB corruption!");
        }
        _stats.currentEpoch.store(_leaderToken.idx().u64, std::memory_order_relaxed);
        return initSuccess;
    }

    ReplicaId getReplicaId() const {
        return _replicaId;
    }

    LogIdx assignLogIdx() {
        ALWAYS_ASSERT(_leaderToken.replica() ==_replicaId);
        return ++_lastAssigned;
    }

    LeaderToken getLeaderToken() const {
        return _leaderToken;
    }

    TernError updateLeaderToken(LeaderToken token) {
        if (unlikely(token < _leaderToken || token < _nomineeToken)) {
            return TernError::LEADER_PREEMPTED;
        }
        if (likely(token == _leaderToken)) {
            return TernError::NO_ERROR;
        }
        _data.dropEntriesAfterIdx(_lastReleased);
        ROCKS_DB_CHECKED(_sharedDb.db()->Put({}, _cf, logsDBMetadataKey(LEADER_TOKEN_KEY), U64Value::Static(token.u64).toSlice()));
        if (_leaderToken != token && token.replica() == _replicaId) {
            // We just became leader, at this point last released should be the last known entry
            _lastAssigned = _lastReleased;
        }
        _leaderToken = token;
        _stats.currentEpoch.store(_leaderToken.idx().u64, std::memory_order_relaxed);
        _nomineeToken = LeaderToken(0,0);
        return TernError::NO_ERROR;
    }

    LeaderToken getNomineeToken() const {
        return _nomineeToken;
    }

    void setNomineeToken(LeaderToken token) {
        if (++_leaderToken.idx() < _nomineeToken.idx()) {
            LOG_INFO(_env, "Got a nominee token for epoch %s, last leader epoch is %s, we must have skipped leader election.", _nomineeToken.idx(), _leaderToken.idx());
            _data.dropEntriesAfterIdx(_lastReleased);
        }
        _nomineeToken = token;
    }

    LeaderToken generateNomineeToken() const {
        auto lastEpoch = _leaderToken.idx();
        return LeaderToken(_replicaId, ++lastEpoch);
    }

    LogIdx getLastReleased() const {
        return _lastReleased;
    }

    TernTime getLastReleasedTime() const {
        return _lastReleasedTime;
    }

    void setLastReleased(LogIdx lastReleased) {
        ALWAYS_ASSERT(_lastReleased <= lastReleased, "Moving release point backwards is not possible. It would cause data inconsistency");
        auto now = ternNow();
        rocksdb::WriteBatch batch;
        batch.Put(_cf, logsDBMetadataKey(LAST_RELEASED_IDX_KEY), U64Value::Static(lastReleased.u64).toSlice());
        batch.Put(_cf, logsDBMetadataKey(LAST_RELEASED_TIME_KEY),U64Value::Static(now.ns).toSlice());
        ROCKS_DB_CHECKED(_sharedDb.db()->Write({}, &batch));
        update_atomic_stat_ema(_stats.entriesReleased, lastReleased.u64 - _lastReleased.u64);
        _lastReleased = lastReleased;
        _lastReleasedTime = now;
    }

    bool isPreempting(LeaderToken token) const {
        return _leaderToken < token && _nomineeToken < token;
    }

private:
    Env& _env;
    LogsDBStats& _stats;
    SharedRocksDB& _sharedDb;
    rocksdb::ColumnFamilyHandle* _cf;
    const ReplicaId _replicaId;
    DataPartitions& _data;

    LogIdx _lastAssigned;
    LogIdx _lastReleased;
    TernTime _lastReleasedTime;
    LeaderToken _leaderToken;
    LeaderToken _nomineeToken;
};

class ReqResp {
    public:
        static constexpr size_t UNUSED_REQ_ID = std::numeric_limits<size_t>::max();
        static constexpr size_t CONFIRMED_REQ_ID = 0;

        using QuorumTrackArray = std::array<uint64_t, LogsDB::REPLICA_COUNT>;

        ReqResp(LogsDBStats& stats) : _stats(stats), _lastAssignedRequest(CONFIRMED_REQ_ID) {}

        LogsDBRequest& newRequest(ReplicaId targetReplicaId) {
            auto& request = _requests[++_lastAssignedRequest];
            request.replicaId = targetReplicaId;
            request.msg.id = _lastAssignedRequest;
            return request;
        }

        LogsDBRequest* getRequest(uint64_t requestId) {
            auto it = _requests.find(requestId);
            if (it == _requests.end()) {
                return nullptr;
            }
            return &it->second;
        }

        void eraseRequest(uint64_t requestId) {
            _requests.erase(requestId);
        }

        void cleanupRequests(QuorumTrackArray& requestIds) {
            for (auto& reqId : requestIds) {
                if (reqId == CONFIRMED_REQ_ID || reqId == UNUSED_REQ_ID) {
                    continue;
                }
                eraseRequest(reqId);
                reqId = ReqResp::UNUSED_REQ_ID;
            }
        }

        void resendTimedOutRequests() {
            auto now = ternNow();
            auto defaultCutoffTime = now - LogsDB::RESPONSE_TIMEOUT;
            auto releaseCutoffTime = now - LogsDB::SEND_RELEASE_INTERVAL;
            auto readCutoffTime = now - LogsDB::READ_TIMEOUT;
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

        void getRequestsToSend(std::vector<LogsDBRequest*>& requests) {
            requests.swap(_requestsToSend);
            update_atomic_stat_ema(_stats.requestsSent, requests.size());
            _requestsToSend.clear();
        }

        LogsDBResponse& newResponse(ReplicaId targetReplicaId, uint64_t requestId) {
            _responses.emplace_back();
            auto& response = _responses.back();
            response.replicaId = targetReplicaId;
            response.msg.id = requestId;
            return response;
        }

        void getResponsesToSend(std::vector<LogsDBResponse>& responses) {
            responses.swap(_responses);
            update_atomic_stat_ema(_stats.responsesSent, responses.size());
            _responses.clear();
        }

        Duration getNextTimeout() const {
            if (_requests.empty()) {
                return LogsDB::LEADER_INACTIVE_TIMEOUT;
            }
            return LogsDB::RESPONSE_TIMEOUT;
        }

        static bool isQuorum(const QuorumTrackArray& requestIds) {
            size_t numResponses = 0;
            for (auto reqId : requestIds) {
                if (reqId == CONFIRMED_REQ_ID) {
                    ++numResponses;
                }
            }
            return numResponses > requestIds.size() / 2;
        }

private:
    LogsDBStats& _stats;
    uint64_t _lastAssignedRequest;
    std::unordered_map<uint64_t, LogsDBRequest> _requests;
    std::vector<LogsDBRequest*> _requestsToSend;

    std::vector<LogsDBResponse> _responses;
};

enum class LeadershipState : uint8_t {
    FOLLOWER,
    BECOMING_NOMINEE,
    DIGESTING_ENTRIES,
    CONFIRMING_REPLICATION,
    CONFIRMING_LEADERSHIP,
    LEADER
};

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

struct LeaderElectionState {
    ReqResp::QuorumTrackArray requestIds;
    LogIdx lastReleased;
    std::array<ReqResp::QuorumTrackArray, LogsDB::IN_FLIGHT_APPEND_WINDOW> recoveryRequests;
    std::array<LogsDBLogEntry, LogsDB::IN_FLIGHT_APPEND_WINDOW> recoveryEntries;
};

class LeaderElection {
public:
    LeaderElection(Env& env, LogsDBStats& stats, bool noReplication, bool avoidBeingLeader, ReplicaId replicaId, LogMetadata& metadata, DataPartitions& data, ReqResp& reqResp) :
        _env(env),
        _stats(stats),
        _noReplication(noReplication),
        _avoidBeingLeader(avoidBeingLeader),
        _replicaId(replicaId),
        _metadata(metadata),
        _data(data),
        _reqResp(reqResp),
        _state(LeadershipState::FOLLOWER),
        _leaderLastActive(_noReplication ? 0 :ternNow()) {}

    bool isLeader() const {
        return _state == LeadershipState::LEADER;
    }

    void maybeStartLeaderElection() {
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

        //if (unlikely(_noReplication)) {
        {
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

    void proccessNewLeaderResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const NewLeaderResp& response) {
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

    void proccessNewLeaderConfirmResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const NewLeaderConfirmResp& response) {
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

    void proccessRecoveryReadResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogRecoveryReadResp& response) {
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

    void proccessRecoveryWriteResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogRecoveryWriteResp& response) {
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

    void proccessNewLeaderRequest(ReplicaId fromReplicaId, uint64_t requestId, const NewLeaderReq& request) {
        if (unlikely(fromReplicaId != request.nomineeToken.replica())) {
            LOG_ERROR(_env, "Nominee token from replica id %s does not have matching replica id. Token: %s", fromReplicaId, request.nomineeToken);
            return;
        }
        auto& response = _reqResp.newResponse( fromReplicaId, requestId);
        auto& newLeaderResponse = response.msg.body.setNewLeader();

        if (request.nomineeToken.idx() <= _metadata.getLeaderToken().idx() || request.nomineeToken < _metadata.getNomineeToken()) {
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

    void proccessNewLeaderConfirmRequest(ReplicaId fromReplicaId, uint64_t requestId, const NewLeaderConfirmReq& request) {
        if (unlikely(fromReplicaId != request.nomineeToken.replica())) {
            LOG_ERROR(_env, "Nominee token from replica id %s does not have matching replica id. Token: %s", fromReplicaId, request.nomineeToken);
            return;
        }
        auto& response = _reqResp.newResponse(fromReplicaId, requestId);
        auto& newLeaderConfirmResponse = response.msg.body.setNewLeaderConfirm();
        if (_metadata.getNomineeToken() == request.nomineeToken) {
            _metadata.setLastReleased(request.releasedIdx);
        }

        auto err = _metadata.updateLeaderToken(request.nomineeToken);
        newLeaderConfirmResponse.result = err;
        if (err == TernError::NO_ERROR) {
            _leaderLastActive = ternNow();
            resetLeaderElection();
        }
    }

    void proccessRecoveryReadRequest(ReplicaId fromReplicaId, uint64_t requestId, const LogRecoveryReadReq& request) {
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

    void proccessRecoveryWriteRequest(ReplicaId fromReplicaId, uint64_t requestId, const LogRecoveryWriteReq& request) {
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

    TernError writeLogEntries(LeaderToken token, LogIdx newlastReleased, std::vector<LogsDBLogEntry>& entries) {
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

    void resetLeaderElection() {
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

private:

    void _tryProgressToDigest() {
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

    void _tryProgressToReplication() {
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
                if (replicaId == replicaId) {
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

    void _tryProgressToLeaderConfirm() {
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

    void _tryBecomeLeader() {
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

    void _clearElectionState() {
        _leaderLastActive = ternNow();
        if (!_electionState) {
            return;
        }
        _reqResp.cleanupRequests(_electionState->requestIds);
        _clearRecoveryRequests();
        _electionState.reset();
    }

    void _clearRecoveryRequests() {
        for(auto& requestIds : _electionState->recoveryRequests) {
            _reqResp.cleanupRequests(requestIds);
        }
    }

    Env& _env;
    LogsDBStats& _stats;
    const bool _noReplication;
    const bool _avoidBeingLeader;
    const ReplicaId _replicaId;
    LogMetadata& _metadata;
    DataPartitions& _data;
    ReqResp& _reqResp;

    LeadershipState _state;
    std::unique_ptr<LeaderElectionState> _electionState;
    TernTime _leaderLastActive;
};

class BatchWriter {
public:
    BatchWriter(Env& env, ReqResp& reqResp, LeaderElection& leaderElection) :
        _env(env),
        _reqResp(reqResp),
        _leaderElection(leaderElection),
        _token(LeaderToken(0,0)),
        _lastReleased(0) {}

    void proccessLogWriteRequest(LogsDBRequest& request) {
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

    void proccessReleaseRequest(ReplicaId fromReplicaId, uint64_t requestId, const ReleaseReq& request) {
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

    void writeBatch() {
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

private:
    Env& _env;
    ReqResp& _reqResp;
    LeaderElection& _leaderElection;

    LeaderToken _token;
    LogIdx _lastReleased;
    std::vector<LogsDBRequest*> _requests;
    std::vector<LogsDBLogEntry> _entries;
};

class CatchupReader {
public:
    CatchupReader(LogsDBStats& stats, ReqResp& reqResp, LogMetadata& metadata, DataPartitions& data, ReplicaId replicaId, LogIdx lastRead) :
        _stats(stats),
        _reqResp(reqResp),
        _metadata(metadata),
        _data(data),
        _replicaId(replicaId),
        _lastRead(lastRead),
        _lastContinuousIdx(lastRead),
        _lastMissingIdx(lastRead) {}

    LogIdx getLastContinuous() const {
        return _lastContinuousIdx;
    }

    void readEntries(std::vector<LogsDBLogEntry>& entries, size_t maxEntries) {
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

    void init() {
        _missingEntries.reserve(LogsDB::CATCHUP_WINDOW);
        _requestIds.reserve(LogsDB::CATCHUP_WINDOW);
        _findMissingEntries();
    }

    void maybeCatchUp() {
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


    void proccessLogReadRequest(ReplicaId fromReplicaId, uint64_t requestId, const LogReadReq& request) {
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

    void proccessLogReadResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogReadResp& response) {
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

    LogIdx lastRead() const {
        return _lastRead;
    }

private:

    void _populateStats() {
        update_atomic_stat_ema(_stats.followerLag, _metadata.getLastReleased().u64 - _lastContinuousIdx.u64);
        update_atomic_stat_ema(_stats.catchupWindow, _missingEntries.size());
    }

    void _findMissingEntries() {
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
        while (startIdx <= lastReleased && _missingEntries.size() < LogsDB::CATCHUP_WINDOW) {
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
            for (ReplicaId replicaId = 0; replicaId.u8 < LogsDB::REPLICA_COUNT; ++replicaId.u8 ) {
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
    LogsDBStats& _stats;
    ReqResp& _reqResp;
    LogMetadata& _metadata;
    DataPartitions& _data;

    const ReplicaId _replicaId;
    LogIdx _lastRead;
    LogIdx _lastContinuousIdx;
    LogIdx _lastMissingIdx;

    std::vector<LogIdx> _missingEntries;
    std::vector<ReqResp::QuorumTrackArray> _requestIds;
};

class Appender {
    static constexpr size_t IN_FLIGHT_MASK = LogsDB::IN_FLIGHT_APPEND_WINDOW - 1;
    static_assert((IN_FLIGHT_MASK & LogsDB::IN_FLIGHT_APPEND_WINDOW) == 0);
public:
    Appender(Env& env, LogsDBStats& stats, ReqResp& reqResp, LogMetadata& metadata, LeaderElection& leaderElection, bool noReplication) :
        _env(env),
        _reqResp(reqResp),
        _metadata(metadata),
        _leaderElection(leaderElection),
        _noReplication(noReplication),
        _currentIsLeader(false),
        _entriesStart(0),
        _entriesEnd(0) { }

    void maybeMoveRelease() {
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

    TernError appendEntries(std::vector<LogsDBLogEntry>& entries) {
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

    void proccessLogWriteResponse(ReplicaId fromReplicaId, LogsDBRequest& request, const LogWriteResp& response) {
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

    uint64_t entriesInFlight() const {
        return _entriesEnd - _entriesStart;
    }

private:

    void _init() {
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

    void _cleanup() {
        for (; _entriesStart < _entriesEnd; ++_entriesStart) {
            auto offset = _entriesStart & IN_FLIGHT_MASK;
            _entries[offset].value.clear();
            _reqResp.cleanupRequests(_requestIds[offset]);
        }
        _reqResp.cleanupRequests(_releaseRequests);
        _currentIsLeader = false;
    }
    Env& _env;
    ReqResp& _reqResp;
    LogMetadata& _metadata;
    LeaderElection& _leaderElection;

    const bool _noReplication;
    bool _currentIsLeader;
    uint64_t _entriesStart;
    uint64_t _entriesEnd;

    std::array<LogsDBLogEntry, LogsDB::IN_FLIGHT_APPEND_WINDOW> _entries;
    std::array<ReqResp::QuorumTrackArray, LogsDB::IN_FLIGHT_APPEND_WINDOW> _requestIds;
    ReqResp::QuorumTrackArray _releaseRequests;


};

class LogsDBImpl {
public:
    LogsDBImpl(
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
        _stats(),
        _partitions(_env,sharedDB),
        _metadata(_env,_stats, sharedDB, replicaId, _partitions),
        _reqResp(_stats),
        _leaderElection(_env, _stats, noReplication, avoidBeingLeader, replicaId, _metadata, _partitions, _reqResp),
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
        bool avoidBeingLeader)
{
    _impl = new LogsDBImpl(logger, xmon, sharedDB, replicaId, lastRead, noReplication, avoidBeingLeader);
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
