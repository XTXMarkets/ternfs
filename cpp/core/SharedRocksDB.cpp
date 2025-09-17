// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "SharedRocksDB.hpp"

#include <fstream>
#include <filesystem>
#include <memory>
#include <mutex>
#include <rocksdb/db.h>
#include <rocksdb/statistics.h>
#include <rocksdb/utilities/checkpoint.h>
#include <shared_mutex>
#include <string>
#include <utility>

#include "Assert.hpp"
#include "MsgsGen.hpp"
#include "RocksDBUtils.hpp"
#include "Time.hpp"

static void closeDB(rocksdb::DB* db) {
    ROCKS_DB_CHECKED(db->Close());
    delete db;
}

SharedRocksDB::SharedRocksDB(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const std::string& path, const std::string& statisticsFilePath) :
    _env(logger, xmon, "shared_rocksdb"),
    _transactionDB(false),
    _path(path),
    _statisticsFilePath(statisticsFilePath),
    _db(nullptr, closeDB)
    {}

SharedRocksDB::~SharedRocksDB() {
    close();
}

void SharedRocksDB::open(rocksdb::Options options) {
    std::unique_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() == nullptr);
    ALWAYS_ASSERT(options.statistics.get() == nullptr);
    _dbStatistics = rocksdb::CreateDBStatistics();
    options.statistics = _dbStatistics;


    std::vector<rocksdb::ColumnFamilyHandle*> cfHandles;
    cfHandles.reserve(_cfDescriptors.size());
    rocksdb::DB* db;
    LOG_INFO(_env, "Opening RocksDB in %s", _path);
    ROCKS_DB_CHECKED_MSG(
            rocksdb::DB::Open(options, _path, _cfDescriptors, &cfHandles, &db),
            "could not open RocksDB %s", _path
    );

    _db = std::unique_ptr<rocksdb::DB, void (*)(rocksdb::DB*)>(db,closeDB);
    ALWAYS_ASSERT(_cfDescriptors.size() == cfHandles.size());

    for (auto i = 0; i < _cfDescriptors.size(); ++i) {
        _cfs.insert(std::make_pair(_cfDescriptors[i].name, cfHandles[i]));
    }

    _cfDescriptors.clear();
}

void SharedRocksDB::openTransactionDB(rocksdb::Options options) {
    std::unique_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() == nullptr);
    ALWAYS_ASSERT(options.statistics.get() == nullptr);
    _dbStatistics = rocksdb::CreateDBStatistics();
    options.statistics = _dbStatistics;

    std::vector<rocksdb::ColumnFamilyHandle*> cfHandles;
    cfHandles.reserve(_cfDescriptors.size());
    rocksdb::OptimisticTransactionDB* db;
    LOG_INFO(_env, "Opening RocksDB in %s", _path);
    ROCKS_DB_CHECKED_MSG(
            rocksdb::OptimisticTransactionDB::Open(options, _path, _cfDescriptors, &cfHandles, &db),
            "could not open RocksDB %s", _path
    );

    _db = std::unique_ptr<rocksdb::DB, void (*)(rocksdb::DB*)>(db,closeDB);
    _transactionDB = true;
    ALWAYS_ASSERT(_cfDescriptors.size() == cfHandles.size());

    for (auto i = 0; i < _cfDescriptors.size(); ++i) {
        _cfs.insert(std::make_pair(_cfDescriptors[i].name, cfHandles[i]));
    }

    _cfDescriptors.clear();
}

void SharedRocksDB::openForReadOnly(rocksdb::Options options) {
    std::unique_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() == nullptr);
    options.create_if_missing = false;
    options.create_missing_column_families = false;

    std::vector<rocksdb::ColumnFamilyHandle*> cfHandles;
    cfHandles.reserve(_cfDescriptors.size());
    rocksdb::DB* db;
    LOG_INFO(_env, "Opening RocksDB as readonly in %s", _path);
    ROCKS_DB_CHECKED_MSG(
            rocksdb::DB::OpenForReadOnly(options, _path, _cfDescriptors, &cfHandles, &db),
            "could not open RocksDB %s", _path
    );

    _db = std::unique_ptr<rocksdb::DB, void (*)(rocksdb::DB*)>(db,closeDB);
    ALWAYS_ASSERT(_cfDescriptors.size() == cfHandles.size());

    for (auto i = 0; i < _cfDescriptors.size(); ++i) {
        _cfs.insert(std::make_pair(_cfDescriptors[i].name, cfHandles[i]));
    }

    _cfDescriptors.clear();
}

void SharedRocksDB::close() {
    std::unique_lock<std::shared_mutex> _(_stateMutex);
    if (_db.get() == nullptr) {
        return;
    }
    LOG_INFO(_env, "Destroying column families and closing database");
    for( auto& cf : _cfs ) {
        ROCKS_DB_CHECKED(_db->DestroyColumnFamilyHandle(cf.second));
    }
    _cfs.clear();
    _db.reset(nullptr);
    LOG_INFO(_env, "database closed");
}

void SharedRocksDB::registerCFDescriptors(const std::vector<rocksdb::ColumnFamilyDescriptor>& cfDescriptors) {
    std::unique_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() == nullptr);
    _cfDescriptors.insert(_cfDescriptors.end(), cfDescriptors.begin(), cfDescriptors.end());
}

rocksdb::ColumnFamilyHandle* SharedRocksDB::getCF(const std::string& name) const {
    std::shared_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() != nullptr);
    auto it = _cfs.find(name);
    if (it == _cfs.end()) {
        return nullptr;
    }
    return it->second;
}

void SharedRocksDB::deleteCF(const std::string& name) {
    std::unique_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() != nullptr);
    auto it = _cfs.find(name);
    if (it == _cfs.end()) {
        return;
    }
    ROCKS_DB_CHECKED(_db->DropColumnFamily(it->second));
    ROCKS_DB_CHECKED(_db->DestroyColumnFamilyHandle(it->second));
    _cfs.erase(it);
}

rocksdb::ColumnFamilyHandle* SharedRocksDB::createCF(const rocksdb::ColumnFamilyDescriptor& descriptor) {
    std::unique_lock<std::shared_mutex> _(_stateMutex);
    ASSERT(_db.get() != nullptr);
    auto it = _cfs.find(descriptor.name);
    if (it != _cfs.end()) {
        return it->second;
    }
    rocksdb::ColumnFamilyHandle* handle;
    ROCKS_DB_CHECKED(_db->CreateColumnFamily(descriptor.options, descriptor.name, &handle));
    _cfs.emplace(descriptor.name, handle);
    return handle;
}

rocksdb::DB* SharedRocksDB::db() const {
    std::shared_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() != nullptr);
    return _db.get();
}

rocksdb::OptimisticTransactionDB* SharedRocksDB::transactionDB() const {
    std::shared_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() != nullptr);
    ALWAYS_ASSERT(_transactionDB);
    return (rocksdb::OptimisticTransactionDB*)_db.get();
}

void SharedRocksDB::rocksDBMetrics(std::unordered_map<std::string, uint64_t>& stats) {
    std::shared_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() != nullptr);
    ::rocksDBMetrics(_env, _db.get(), *_dbStatistics, stats);
}

void SharedRocksDB::dumpRocksDBStatistics() {
    std::shared_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() != nullptr);
    LOG_INFO(_env, "Dumping statistics to %s", _statisticsFilePath);
    std::ofstream file(_statisticsFilePath);
    file << _dbStatistics->ToString();
}

namespace fs = std::filesystem;

TernError SharedRocksDB::snapshot(const std::string& path) {
    std::shared_lock<std::shared_mutex> _(_stateMutex);
    ALWAYS_ASSERT(_db.get() != nullptr);
    LOG_INFO(_env, "Creating snapshot in  %s", path);
    std::error_code ec;
    if (fs::is_directory(path, ec)) {
        LOG_INFO(_env, "Snapshot exists in  %s", path);
        return TernError::NO_ERROR;
    }
    if (fs::exists(path, ec)) {
        LOG_ERROR(_env, "Provided path exists and is not an existing snapshot  %s", path);
        return TernError::CANNOT_CREATE_DB_SNAPSHOT;
    }
    std::string tmpPath;
    {
        fs::path p{path};
        if (!p.has_parent_path()) {
            LOG_ERROR(_env, "Path %s does not have parent", path);
            return TernError::CANNOT_CREATE_DB_SNAPSHOT;
        }
        p = p.parent_path();
        if (!fs::is_directory(p, ec)) {
            LOG_ERROR(_env, "Parent path of %s is not a directory", path);
            return TernError::CANNOT_CREATE_DB_SNAPSHOT;
        }
        p /= "tmp-snapshot-" + std::to_string(ternNow().ns);
        if (fs::exists(p, ec)) {
            LOG_ERROR(_env, "Tmp path exists %s", p.generic_string());
            return TernError::CANNOT_CREATE_DB_SNAPSHOT;
        }
        tmpPath = p.generic_string();
    }

    std::unique_ptr<rocksdb::Checkpoint> checkpoint;
    auto status = rocksdb::Checkpoint::Create(_db.get(), (rocksdb::Checkpoint**)(&checkpoint));
    if (!status.ok()) {
        LOG_ERROR(_env, "Failed creating checkpint (%s)", status.ToString());
        return TernError::CANNOT_CREATE_DB_SNAPSHOT;
    }

    status = checkpoint->CreateCheckpoint(tmpPath);
    if (!status.ok()) {
        LOG_ERROR(_env, "Failed storing checkpint (%s)", status.ToString());
        // try to cleanup tmpPath
        fs::remove_all(tmpPath, ec);
        return TernError::CANNOT_CREATE_DB_SNAPSHOT;
    }
    fs::rename(tmpPath, path, ec);
    if (ec) {
        LOG_ERROR(_env, "Failed moving temp dir to requested path error (%s)", ec.message());
        // try to cleanup tmpPath
        fs::remove_all(tmpPath, ec);
        return TernError::CANNOT_CREATE_DB_SNAPSHOT;
    }
    return TernError::NO_ERROR;
}
