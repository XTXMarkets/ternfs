// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "LogMetadata.hpp"

#include <memory>
#include <string>

#include <rocksdb/db.h>
#include <rocksdb/iterator.h>
#include <rocksdb/slice.h>
#include <rocksdb/write_batch.h>

#include "../Assert.hpp"
#include "../LogsDBData.hpp"
#include "../RocksDBUtils.hpp"

static bool tryGet(rocksdb::DB* db, rocksdb::ColumnFamilyHandle* cf, const rocksdb::Slice& key, std::string& value) {
    auto status = db->Get({}, cf, key, &value);
    if (status.IsNotFound()) {
        return false;
    }
    ROCKS_DB_CHECKED(status);
    return true;
};

static void update_atomic_stat_ema(std::atomic<double>& stat, double newValue) {
    stat.store((stat.load(std::memory_order_relaxed)* 0.95 + newValue * 0.05), std::memory_order_relaxed);
}

LogMetadata::LogMetadata(Env& env, LogsDBStats& stats, SharedRocksDB& sharedDb, ReplicaId replicaId, DataPartitions& data) :
    _env(env),
    _stats(stats),
    _sharedDb(sharedDb),
    _cf(sharedDb.getCF(DataPartitions::METADATA_CF_NAME)),
    _replicaId(replicaId),
    _data(data),
    _nomineeToken(LeaderToken(0,0))
{}

bool LogMetadata::isInitialStart() {
    auto it = std::unique_ptr<rocksdb::Iterator>(_sharedDb.db()->NewIterator({},_cf));
    it->SeekToFirst();
    return !it->Valid();
}

bool LogMetadata::init(bool initialStart) {
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

ReplicaId LogMetadata::getReplicaId() const {
    return _replicaId;
}

LogIdx LogMetadata::assignLogIdx() {
    ALWAYS_ASSERT(_leaderToken.replica() ==_replicaId);
    return ++_lastAssigned;
}

LeaderToken LogMetadata::getLeaderToken() const {
    return _leaderToken;
}

TernError LogMetadata::updateLeaderToken(LeaderToken token) {
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

LeaderToken LogMetadata::getNomineeToken() const {
    return _nomineeToken;
}

void LogMetadata::setNomineeToken(LeaderToken token) {
    if (++_leaderToken.idx() < token.idx()) {
        LOG_INFO(_env, "Got a nominee token for epoch %s, last leader epoch is %s, we must have skipped leader election.", token.idx(), _leaderToken.idx());
        _data.dropEntriesAfterIdx(_lastReleased);
    }
    _nomineeToken = token;
}

LeaderToken LogMetadata::generateNomineeToken() const {
    auto lastEpoch = _leaderToken.idx();
    return LeaderToken(_replicaId, ++lastEpoch);
}

LogIdx LogMetadata::getLastReleased() const {
    return _lastReleased;
}

TernTime LogMetadata::getLastReleasedTime() const {
    return _lastReleasedTime;
}

void LogMetadata::setLastReleased(LogIdx lastReleased) {
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

bool LogMetadata::isPreempting(LeaderToken token) const {
    return _leaderToken < token && _nomineeToken < token;
}
