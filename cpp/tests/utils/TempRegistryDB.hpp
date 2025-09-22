// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <filesystem>
#include <iostream>
#include <memory>
#include <ostream>

#include "Env.hpp"
#include "RegistryDB.hpp"
#include "SharedRocksDB.hpp"

struct TempRegistryDB {
    std::string dbDir;
    Logger logger;
    std::shared_ptr<XmonAgent> xmon;

    std::unique_ptr<SharedRocksDB> sharedDB;
    std::unique_ptr<RegistryDB> db;

    TempRegistryDB(
        LogLevel level): logger(level, STDERR_FILENO, false, false)
    {
        dbDir = std::string("temp-registry-db.XXXXXX");
        if (mkdtemp(dbDir.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }
    }


    void open(const RegistryOptions& options)
    {
        sharedDB = std::make_unique<SharedRocksDB>(logger, xmon, dbDir + "/db", dbDir + "/db-statistics.txt");
        initSharedDB();
        db = std::make_unique<RegistryDB>(logger, xmon, options, *sharedDB);
    }

    void close() {
        db->close();
        sharedDB->close();
        db.reset();
        sharedDB.reset();
    }

    ~TempRegistryDB() {
        std::error_code err;
        if (std::filesystem::remove_all(std::filesystem::path(dbDir), err) < 0) {
            std::cerr << "Could not remove " << dbDir << ": " << err << std::endl;
        }
    }

    std::unique_ptr<RegistryDB>& operator->() {
        return db;
    }

    void initSharedDB() {
        sharedDB->registerCFDescriptors({{rocksdb::kDefaultColumnFamilyName, {}}});
        sharedDB->registerCFDescriptors(RegistryDB::getColumnFamilyDescriptors());
        rocksdb::Options rocksDBOptions;
        rocksDBOptions.create_if_missing = true;
        rocksDBOptions.create_missing_column_families = true;
        rocksDBOptions.compression = rocksdb::kLZ4Compression;
        rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
        // 1000*256 = 256k open files at once, given that we currently run on a
        // single machine this is appropriate.
        rocksDBOptions.max_open_files = 1000;
        // We batch writes and flush manually.
        rocksDBOptions.manual_wal_flush = true;
        sharedDB->open(rocksDBOptions);
    }
};

