// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <memory>
#include <shared_mutex>
#include <string>
#include <unordered_map>
#include <vector>

#include <rocksdb/db.h>
#include <rocksdb/options.h>
#include <rocksdb/utilities/optimistic_transaction_db.h>

#include "Env.hpp"
#include "MsgsGen.hpp"

class SharedRocksDB {
public:
    SharedRocksDB(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const std::string& path, const std::string& statisticsPath);
    ~SharedRocksDB();
    void open(rocksdb::Options options, const std::string& path);
    void openTransactionDB(rocksdb::Options options);
    void open(rocksdb::Options options);
    void openForReadOnly(rocksdb::Options options);
    void close();

    void registerCFDescriptors(const std::vector<rocksdb::ColumnFamilyDescriptor>& _cfDescriptors);

    rocksdb::ColumnFamilyHandle* getCF(const std::string& name) const;
    void deleteCF(const std::string& name);
    rocksdb::ColumnFamilyHandle* createCF(const rocksdb::ColumnFamilyDescriptor& descriptor);


    rocksdb::DB* db() const;
    rocksdb::OptimisticTransactionDB* transactionDB() const;
    void rocksDBMetrics(std::unordered_map<std::string, uint64_t>& stats);
    void dumpRocksDBStatistics();

    TernError snapshot(const std::string& path);

private:
    Env _env;
    bool _transactionDB;
    const std::string _path;
    const std::string _statisticsFilePath;
    std::unique_ptr<rocksdb::DB, void(*)(rocksdb::DB*)> _db;
    std::shared_ptr<rocksdb::Statistics> _dbStatistics;
    std::vector<rocksdb::ColumnFamilyDescriptor> _cfDescriptors;
    mutable std::shared_mutex _stateMutex;
    std::unordered_map<std::string, rocksdb::ColumnFamilyHandle*> _cfs;
};
