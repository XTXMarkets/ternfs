// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <filesystem>
#include <memory>

#include "Env.hpp"
#include "BlockServicesCacheDB.hpp"
#include "SharedRocksDB.hpp"

struct TempBlockServicesCacheDB {
    std::string dbDir;
    Logger logger;
    std::shared_ptr<XmonAgent> xmon;

    std::unique_ptr<SharedRocksDB> sharedDB;

    TempBlockServicesCacheDB(LogLevel level)
        : logger(level, STDERR_FILENO, false, false) {
        dbDir = std::string("temp-bs-cache-db.XXXXXX");
        if (mkdtemp(dbDir.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }
    }

    ~TempBlockServicesCacheDB() {
        std::error_code err;
        std::filesystem::remove_all(std::filesystem::path(dbDir), err);
    }

    void open() {
        sharedDB = std::make_unique<SharedRocksDB>(logger, xmon, dbDir + "/db", dbDir + "/db-statistics.txt");
        initSharedDB();
    }

    void close() {
        if (sharedDB) { sharedDB->close(); sharedDB.reset(); }
    }

    void initSharedDB() {
        std::vector<rocksdb::ColumnFamilyDescriptor> descs;
        descs.push_back({rocksdb::kDefaultColumnFamilyName, {}});
        auto extra = BlockServicesCacheDB::getColumnFamilyDescriptors();
        descs.insert(descs.end(), extra.begin(), extra.end());
        sharedDB->registerCFDescriptors(descs);
        rocksdb::Options rocksDBOptions;
        rocksDBOptions.create_if_missing = true;
        rocksDBOptions.create_missing_column_families = true;
        rocksDBOptions.compression = rocksdb::kLZ4Compression;
        rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
        rocksDBOptions.max_open_files = 1000;
        rocksDBOptions.manual_wal_flush = true;
        sharedDB->open(rocksDBOptions);
    }
};
