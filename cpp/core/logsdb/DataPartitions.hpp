// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <array>
#include <memory>
#include <string>
#include <vector>

#include <rocksdb/iterator.h>

#include "../LogsDB.hpp"
#include "../LogsDBData.hpp"

class DataPartitions {
public:
    static constexpr auto METADATA_CF_NAME = "logMetadata";
    static constexpr auto DATA_PARTITION_0_NAME = "logTimePartition0";
    static constexpr auto DATA_PARTITION_1_NAME = "logTimePartition1";

    class Iterator {
    public:
        Iterator(const DataPartitions& partitions);

        void seek(LogIdx idx);
        bool valid() const;
        void next();
        Iterator& operator++();
        LogIdx key() const;
        LogsDBLogEntry entry() const;
        void dropEntry();

    private:
        void _updateSmaller();
        size_t _cfIndexForCurrentIterator() const;

        const DataPartitions& _partitions;
        size_t _rotationCount;
        rocksdb::Iterator* _smaller;
        std::vector<std::unique_ptr<rocksdb::Iterator>> _iterators;
    };

    DataPartitions(Env& env, SharedRocksDB& sharedDB);

    bool isInitialStart();
    bool init(bool initialStart);

    Iterator getIterator() const;

    TernError readLogEntry(LogIdx logIdx, LogsDBLogEntry& entry) const;
    void readIndexedEntries(
        const std::vector<LogIdx>& indices,
        std::vector<LogsDBLogEntry>& entries) const;

    void writeLogEntries(const std::vector<LogsDBLogEntry>& entries);
    void writeLogEntry(const LogsDBLogEntry& entry);
    void dropEntriesAfterIdx(LogIdx start);

    LogIdx getLowestKey() const;

private:
    struct LogPartition {
        std::string name;
        LogsDBMetadataKey firstWriteKey;
        rocksdb::ColumnFamilyHandle* cf;
        TernTime firstWriteTime{0};
        LogIdx minKey{0};
        LogIdx maxKey{0};

        void reset(
            rocksdb::ColumnFamilyHandle* newCF,
            LogIdx minMaxKey,
            TernTime newFirstWriteTime);
    };

    void _updatePartitionFirstWriteTime(
        LogPartition& partition,
        TernTime time);
    std::vector<std::unique_ptr<rocksdb::Iterator>>
        _getPartitionIterators() const;
    void _maybeRotate();
    LogPartition& _getPartitionForIdx(LogIdx key);
    const LogPartition& _getPartitionForIdx(LogIdx key) const;
    void _partitionKeyInserted(LogPartition& partition, LogIdx idx);

    Env& _env;
    SharedRocksDB& _sharedDb;
    size_t _rotationCount;
    std::array<LogPartition, 2> _partitions;
};
