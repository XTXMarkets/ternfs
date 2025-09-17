// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <filesystem>
#include <ostream>

#include "Env.hpp"
#include "LogsDB.hpp"

struct TempLogsDB {
    std::string dbDir;
    Logger logger;
    std::shared_ptr<XmonAgent> xmon;

    std::unique_ptr<SharedRocksDB> sharedDB;
    std::unique_ptr<LogsDB> db;

    TempLogsDB(
        LogLevel level,
        ReplicaId replicaId = 0,
        LogIdx lastRead = 0,
        bool noReplication = false,
        bool avoidBeingLeader = false): logger(level, STDERR_FILENO, false, false)
    {
        dbDir = std::string("temp-logs-db.XXXXXX");
        if (mkdtemp(dbDir.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }

        sharedDB = std::make_unique<SharedRocksDB>(logger, xmon, dbDir + "/db", dbDir + "/db-statistics.txt");

        initSharedDB();
        db = std::make_unique<LogsDB>(logger, xmon, *sharedDB, replicaId, lastRead, noReplication, avoidBeingLeader);
    }

    // useful to test recovery
    void restart(
        ReplicaId replicaId = 0,
        LogIdx lastRead = 0,
        bool noReplication = false,
        bool avoidBeingLeader = false)
    {
        db->close();
        sharedDB = std::make_unique<SharedRocksDB>(logger, xmon, dbDir + "/db", dbDir + "/db-statistics.txt");
        initSharedDB();
        db = std::make_unique<LogsDB>(logger, xmon, *sharedDB, replicaId, lastRead, noReplication, avoidBeingLeader);
    }

    ~TempLogsDB() {
        std::error_code err;
        if (std::filesystem::remove_all(std::filesystem::path(dbDir), err) < 0) {
            std::cerr << "Could not remove " << dbDir << ": " << err << std::endl;
        }
    }

    std::unique_ptr<LogsDB>& operator->() {
        return db;
    }

    void initSharedDB() {
        sharedDB->registerCFDescriptors({{rocksdb::kDefaultColumnFamilyName, {}}});
        sharedDB->registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
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

inline LogsDBLogEntry initEntry(uint64_t idx, std::string data) {
    LogsDBLogEntry e;
    e.idx = idx;
    e.value.assign(data.begin(), data.end());
    return e;
}

inline std::ostream& operator<<(std::ostream& out, const std::vector<LogsDBLogEntry>& entries) {
    out << "{ ";
    for (auto& entry : entries) {
        out << "{" << entry << "}" << ",";
    }
    out << "} ";
    return out;
}
