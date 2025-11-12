// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "DataPartitions.hpp"

#include <algorithm>
#include <rocksdb/comparator.h>

#include "../Assert.hpp"
#include "../Bincode.hpp"
#include "../LogsDB.hpp"
#include "../RocksDBUtils.hpp"
#include "LogsDBCommon.hpp"

void LogPartition::reset(rocksdb::ColumnFamilyHandle* cf_, LogIdx minMaxKey, TernTime firstWriteTime_) {
    cf = cf_;
    minKey = maxKey = minMaxKey;
    firstWriteTime = firstWriteTime_;
}

DataPartitions::Iterator::Iterator(const DataPartitions& partitions) 
    : _partitions(partitions), _rotationCount(_partitions._rotationCount), _smaller(nullptr) {
    _iterators = _partitions._getPartitionIterators();
}

void DataPartitions::Iterator::seek(LogIdx idx) {
    if (unlikely(_rotationCount != _partitions._rotationCount)) {
        _iterators = _partitions._getPartitionIterators();
    }
    auto key = U64Key::Static(idx.u64);
    for (auto& it : _iterators) {
        it->Seek(key.toSlice());
    }
    _updateSmaller();
}

bool DataPartitions::Iterator::valid() const {
    return _smaller != nullptr;
}

void DataPartitions::Iterator::next() {
    if (_smaller != nullptr) {
        _smaller->Next();
    }
    _updateSmaller();
}

DataPartitions::Iterator& DataPartitions::Iterator::operator++() {
    this->next();
    return *this;
}

LogIdx DataPartitions::Iterator::key() const {
    return LogIdx(ExternalValue<U64Key>::FromSlice(_smaller->key())().u64());
}

LogsDBLogEntry DataPartitions::Iterator::entry() const {
    auto value = _smaller->value();
    return LogsDBLogEntry{key(), {(const uint8_t*)value.data(), (const uint8_t*)value.data() + value.size()}};
}

void DataPartitions::Iterator::dropEntry() {
    ALWAYS_ASSERT(_rotationCount == _partitions._rotationCount);
    auto cfIdx = _cfIndexForCurrentIterator();
    ROCKS_DB_CHECKED(_partitions._sharedDb.db()->Delete({}, _partitions._partitions[cfIdx].cf, _smaller->key()));
}

void DataPartitions::Iterator::_updateSmaller() {
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

size_t DataPartitions::Iterator::_cfIndexForCurrentIterator() const {
    for (size_t i = 0; i < _iterators.size(); ++i) {
        if (_smaller == _iterators[i].get()) {
            return i;
        }
    }
    return -1;
}

DataPartitions::DataPartitions(Env& env, SharedRocksDB& sharedDB)
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

bool DataPartitions::isInitialStart() {
    auto it1 = std::unique_ptr<rocksdb::Iterator>(_sharedDb.db()->NewIterator({},_partitions[0].cf));
    auto it2 = std::unique_ptr<rocksdb::Iterator>(_sharedDb.db()->NewIterator({},_partitions[1].cf));
    it1->SeekToFirst();
    it2->SeekToFirst();
    return !(it1->Valid() || it2->Valid());
}

bool DataPartitions::init(bool initialStart) {
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

DataPartitions::Iterator DataPartitions::getIterator() const {
    return Iterator(*this);
}

TernError DataPartitions::readLogEntry(LogIdx logIdx, LogsDBLogEntry& entry) const {
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

void DataPartitions::readIndexedEntries(const std::vector<LogIdx>& indices, std::vector<LogsDBLogEntry>& entries) const {
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

void DataPartitions::writeLogEntries(const std::vector<LogsDBLogEntry>& entries) {
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

void DataPartitions::writeLogEntry(const LogsDBLogEntry& entry) {
    _maybeRotate();

    auto& partition = _getPartitionForIdx(entry.idx);
    _sharedDb.db()->Put({}, partition.cf, U64Key::Static(entry.idx.u64).toSlice(), rocksdb::Slice((const char*)entry.value.data(), entry.value.size()));
    _partitionKeyInserted(partition, entry.idx);

}

void DataPartitions::dropEntriesAfterIdx(LogIdx start) {
    auto iterator = getIterator();
    size_t droppedEntriesCount = 0;
    for (iterator.seek(start), iterator.next(); iterator.valid(); ++iterator) {
        iterator.dropEntry();
        ++droppedEntriesCount;
    }
    LOG_INFO(_env,"Dropped %s entries after %s", droppedEntriesCount, start);
}

LogIdx DataPartitions::getLowestKey() const {
    return std::min(_partitions[0].firstWriteTime == 0 ? MAX_LOG_IDX : _partitions[0].minKey, _partitions[1].firstWriteTime == 0 ? MAX_LOG_IDX : _partitions[1].minKey);
}

void DataPartitions::_updatePartitionFirstWriteTime(LogPartition& partition, TernTime time) {
    ROCKS_DB_CHECKED(_sharedDb.db()->Put({}, _sharedDb.getCF(METADATA_CF_NAME), logsDBMetadataKey(partition.firstWriteKey), U64Value::Static(time.ns).toSlice()));
    partition.firstWriteTime = time;
}

std::vector<std::unique_ptr<rocksdb::Iterator>> DataPartitions::_getPartitionIterators() const {
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

void DataPartitions::_maybeRotate() {
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

LogPartition& DataPartitions::_getPartitionForIdx(LogIdx key) {
    return const_cast<LogPartition&>(static_cast<const DataPartitions*>(this)->_getPartitionForIdx(key));
}

const LogPartition& DataPartitions::_getPartitionForIdx(LogIdx key) const {
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

void DataPartitions::_partitionKeyInserted(LogPartition& partition, LogIdx idx) {
    if (unlikely(partition.minKey == 0)) {
        partition.minKey = idx;
        _updatePartitionFirstWriteTime(partition, ternNow());
    }
    partition.minKey = std::min(partition.minKey, idx);
    partition.maxKey = std::max(partition.maxKey, idx);
}
