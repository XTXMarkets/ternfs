// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "CDCDBTools.hpp"

#include <optional>
#include <memory>
#include <rocksdb/db.h>
#include <vector>

#include "Bincode.hpp"
#include "Common.hpp"
#include "Env.hpp"
#include "LogsDB.hpp"
#include "LogsDBTools.hpp"
#include "Msgs.hpp"
#include "RocksDBUtils.hpp"
#include "CDCDB.hpp"
#include "SharedRocksDB.hpp"

void CDCDBTools::outputLogEntries(const std::string& dbPath, LogIdx startIdx, size_t count) {
    Logger logger(LogLevel::LOG_INFO, STDERR_FILENO, false, false);
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "CDCDBTools");
    SharedRocksDB sharedDb(logger, xmon, dbPath, "");
    auto cdcCFdescriptors = CDCDB::getColumnFamilyDescriptors();
    for (auto it = cdcCFdescriptors.begin(); it != cdcCFdescriptors.end(); ++it) {
        if (it->name == "reqQueue") {
            cdcCFdescriptors.erase(it);
            break;
        }
    }
    sharedDb.registerCFDescriptors(cdcCFdescriptors);
    sharedDb.registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
    rocksdb::Options rocksDBOptions;
    rocksDBOptions.compression = rocksdb::kLZ4Compression;
    rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
    sharedDb.openForReadOnly(rocksDBOptions);
    size_t entriesAtOnce = std::min<size_t>(count, 1 << 20);
    std::vector<LogsDBLogEntry> logEntries;
    logEntries.reserve(entriesAtOnce);
    while (count > 0) {
        entriesAtOnce = std::min(count, entriesAtOnce);
        LogsDBTools::getLogEntries(env, sharedDb, startIdx, entriesAtOnce, logEntries);
        for (const auto& entry : logEntries) {
            BincodeBuf buf((char*)&entry.value.front(), entry.value.size());
            CDCLogEntry cdcLogEntry;
            cdcLogEntry.unpack(buf);
            cdcLogEntry.logIdx(entry.idx.u64);
            LOG_INFO(env, "%s", cdcLogEntry);
        }
        if (logEntries.size() == entriesAtOnce) {
            startIdx = logEntries.back().idx + 1;
            count -= entriesAtOnce;
        } else {
            break;
        }
    }
}

