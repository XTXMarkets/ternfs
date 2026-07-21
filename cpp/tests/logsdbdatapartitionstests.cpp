// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <filesystem>
#include <memory>
#include <string>
#include <vector>

#include <rocksdb/db.h>
#include <rocksdb/options.h>

#include "Env.hpp"
#include "LogsDB.hpp"
#include "SharedRocksDB.hpp"
#include "Time.hpp"
#include "logsdb/DataPartitions.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

namespace {

struct DataPartitionsTestDB {
    std::string dbDir{"temp-logs-data-partitions.XXXXXX"};
    Logger logger{LogLevel::LOG_ERROR, STDERR_FILENO, false, false};
    std::shared_ptr<XmonAgent> xmon;
    Env env{logger, xmon, "DataPartitionsTest"};
    std::unique_ptr<SharedRocksDB> sharedDB;

    DataPartitionsTestDB() {
        if (mkdtemp(dbDir.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }
        open();
    }

    ~DataPartitionsTestDB() {
        sharedDB.reset();
        std::error_code error;
        std::filesystem::remove_all(dbDir, error);
    }

    void reopen() {
        sharedDB.reset();
        open();
    }

private:
    void open() {
        sharedDB = std::make_unique<SharedRocksDB>(
            logger,
            xmon,
            dbDir + "/db",
            dbDir + "/db-statistics.txt");
        sharedDB->registerCFDescriptors(
            {{rocksdb::kDefaultColumnFamilyName, {}}});
        sharedDB->registerCFDescriptors(
            LogsDB::getColumnFamilyDescriptors());

        rocksdb::Options options;
        options.create_if_missing = true;
        options.create_missing_column_families = true;
        options.compression = rocksdb::kLZ4Compression;
        options.bottommost_compression = rocksdb::kZSTD;
        options.manual_wal_flush = true;
        sharedDB->open(options);
    }
};

LogsDBLogEntry makeEntry(uint64_t idx, const std::string& value) {
    return {
        LogIdx(idx),
        std::vector<uint8_t>(value.begin(), value.end())
    };
}

std::string entryValue(const LogsDBLogEntry& entry) {
    return std::string(entry.value.begin(), entry.value.end());
}

}

TEST_CASE("DataPartitions reads and iterates entries in log order") {
    _setCurrentTime(TernTime(1));
    DataPartitionsTestDB testDB;
    DataPartitions partitions(testDB.env, *testDB.sharedDB);

    CHECK(partitions.isInitialStart());
    REQUIRE(partitions.init(true));

    std::vector<LogsDBLogEntry> entries{
        makeEntry(1, "one"),
        makeEntry(2, "two"),
        makeEntry(3, "three"),
    };
    partitions.writeLogEntries(entries);
    entries.emplace_back(makeEntry(4, "four"));
    partitions.writeLogEntry(entries.back());

    CHECK_FALSE(partitions.isInitialStart());
    CHECK(partitions.getLowestKey() == LogIdx(1));

    LogsDBLogEntry entry;
    CHECK(partitions.readLogEntry(LogIdx(2), entry) == TernError::NO_ERROR);
    CHECK(entry.idx == LogIdx(2));
    CHECK(entryValue(entry) == "two");
    CHECK(
        partitions.readLogEntry(LogIdx(5), entry) ==
        TernError::LOG_ENTRY_MISSING);

    std::vector<LogsDBLogEntry> indexedEntries;
    partitions.readIndexedEntries(
        {LogIdx(4), LogIdx(5), LogIdx(1)},
        indexedEntries);
    REQUIRE(indexedEntries.size() == 3);
    CHECK(indexedEntries[0] == entries[3]);
    CHECK(indexedEntries[1].idx == LogIdx(0));
    CHECK(indexedEntries[2] == entries[0]);

    std::vector<LogsDBLogEntry> iteratedEntries;
    auto iterator = partitions.getIterator();
    for (iterator.seek(LogIdx(1)); iterator.valid(); ++iterator) {
        iteratedEntries.emplace_back(iterator.entry());
    }
    CHECK(iteratedEntries == entries);
}

TEST_CASE("DataPartitions truncation and metadata survive reopen") {
    _setCurrentTime(TernTime(1));
    DataPartitionsTestDB testDB;

    {
        DataPartitions partitions(testDB.env, *testDB.sharedDB);
        REQUIRE(partitions.init(true));
        partitions.writeLogEntries({
            makeEntry(1, "one"),
            makeEntry(2, "two"),
            makeEntry(3, "three"),
        });
        partitions.dropEntriesAfterIdx(LogIdx(2));

        LogsDBLogEntry entry;
        CHECK(
            partitions.readLogEntry(LogIdx(1), entry) ==
            TernError::NO_ERROR);
        CHECK(
            partitions.readLogEntry(LogIdx(2), entry) ==
            TernError::NO_ERROR);
        CHECK(
            partitions.readLogEntry(LogIdx(3), entry) ==
            TernError::LOG_ENTRY_MISSING);
    }

    testDB.reopen();

    DataPartitions reopened(testDB.env, *testDB.sharedDB);
    CHECK_FALSE(reopened.isInitialStart());
    REQUIRE(reopened.init(false));
    CHECK(reopened.getLowestKey() == LogIdx(1));

    LogsDBLogEntry entry;
    CHECK(reopened.readLogEntry(LogIdx(1), entry) == TernError::NO_ERROR);
    CHECK(entryValue(entry) == "one");
    CHECK(reopened.readLogEntry(LogIdx(2), entry) == TernError::NO_ERROR);
    CHECK(entryValue(entry) == "two");
    CHECK(
        reopened.readLogEntry(LogIdx(3), entry) ==
        TernError::LOG_ENTRY_MISSING);
}
