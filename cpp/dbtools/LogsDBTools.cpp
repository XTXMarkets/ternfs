// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "LogsDBTools.hpp"

#include <rocksdb/slice.h>

#include "Env.hpp"
#include "LogsDB.hpp"
#include "SharedRocksDB.hpp"

void LogsDBTools::getUnreleasedLogEntries(Env& env, SharedRocksDB& sharedDB, LogIdx& lastReleasedOut, std::vector<LogIdx>& unreleasedLogEntriesOut) {
    LogsDB::_getUnreleasedLogEntries(env, sharedDB, lastReleasedOut, unreleasedLogEntriesOut);
}

void LogsDBTools::getLogEntries(Env& env, SharedRocksDB& sharedDB, LogIdx start, size_t count, std::vector<LogsDBLogEntry>& logEntriesOut) {
    LogsDB::_getLogEntries(env, sharedDB, start, count, logEntriesOut);
}
