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
#include "logsdb/LogMetadata.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

namespace {

struct LogMetadataTestDB {
    std::string dbDir{"temp-logs-metadata.XXXXXX"};
    Logger logger{LogLevel::LOG_ERROR, STDERR_FILENO, false, false};
    std::shared_ptr<XmonAgent> xmon;
    Env env{logger, xmon, "LogMetadataTest"};
    std::unique_ptr<SharedRocksDB> sharedDB;

    LogMetadataTestDB() {
        if (mkdtemp(dbDir.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }
        open();
    }

    ~LogMetadataTestDB() {
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

}

TEST_CASE("LogMetadata persists leader and release state") {
    _setCurrentTime(TernTime(10));
    LogMetadataTestDB testDB;
    LogsDBStats stats;

    {
        DataPartitions data(testDB.env, *testDB.sharedDB);
        LogMetadata metadata(
            testDB.env,
            stats,
            *testDB.sharedDB,
            ReplicaId(2),
            data);

        CHECK(metadata.isInitialStart());
        CHECK(data.isInitialStart());
        REQUIRE(metadata.init(true));
        REQUIRE(data.init(true));

        CHECK(metadata.getReplicaId() == ReplicaId(2));
        CHECK(metadata.getLeaderToken() == LeaderToken(0, 0));
        CHECK(metadata.getNomineeToken() == LeaderToken(0, 0));
        CHECK(metadata.getLastReleased() == LogIdx(0));
        CHECK(metadata.getLastReleasedTime() == TernTime(10));
        CHECK(stats.currentEpoch.load() == 0);

        data.writeLogEntries({
            makeEntry(1, "one"),
            makeEntry(2, "two"),
            makeEntry(3, "three"),
        });

        _setCurrentTime(TernTime(20));
        metadata.setLastReleased(LogIdx(2));
        CHECK(metadata.getLastReleased() == LogIdx(2));
        CHECK(metadata.getLastReleasedTime() == TernTime(20));
        CHECK(stats.entriesReleased.load() == doctest::Approx(0.1));

        auto token = metadata.generateNomineeToken();
        CHECK(token == LeaderToken(ReplicaId(2), LogIdx(1)));
        metadata.setNomineeToken(token);
        REQUIRE(metadata.updateLeaderToken(token) == TernError::NO_ERROR);

        CHECK(metadata.getLeaderToken() == token);
        CHECK(metadata.getNomineeToken() == LeaderToken(0, 0));
        CHECK(stats.currentEpoch.load() == 1);
        CHECK(metadata.assignLogIdx() == LogIdx(3));

        LogsDBLogEntry entry;
        CHECK(
            data.readLogEntry(LogIdx(3), entry) ==
            TernError::LOG_ENTRY_MISSING);
    }

    testDB.reopen();

    LogsDBStats reopenedStats;
    DataPartitions reopenedData(testDB.env, *testDB.sharedDB);
    LogMetadata reopenedMetadata(
        testDB.env,
        reopenedStats,
        *testDB.sharedDB,
        ReplicaId(2),
        reopenedData);
    CHECK_FALSE(reopenedMetadata.isInitialStart());
    REQUIRE(reopenedMetadata.init(false));
    REQUIRE(reopenedData.init(false));

    CHECK(
        reopenedMetadata.getLeaderToken() ==
        LeaderToken(ReplicaId(2), LogIdx(1)));
    CHECK(reopenedMetadata.getLastReleased() == LogIdx(2));
    CHECK(reopenedMetadata.getLastReleasedTime() == TernTime(20));
    CHECK(reopenedStats.currentEpoch.load() == 1);
}

TEST_CASE("LogMetadata rejects preempted leader tokens") {
    _setCurrentTime(TernTime(1));
    LogMetadataTestDB testDB;
    LogsDBStats stats;
    DataPartitions data(testDB.env, *testDB.sharedDB);
    LogMetadata metadata(
        testDB.env,
        stats,
        *testDB.sharedDB,
        ReplicaId(1),
        data);
    REQUIRE(metadata.init(true));
    REQUIRE(data.init(true));

    auto nominee = LeaderToken(ReplicaId(2), LogIdx(1));
    metadata.setNomineeToken(nominee);

    CHECK(
        metadata.updateLeaderToken(LeaderToken(ReplicaId(1), LogIdx(1))) ==
        TernError::LEADER_PREEMPTED);
    CHECK(metadata.getLeaderToken() == LeaderToken(0, 0));
    CHECK(metadata.getNomineeToken() == nominee);
    CHECK(metadata.isPreempting(LeaderToken(ReplicaId(3), LogIdx(1))));
    CHECK_FALSE(
        metadata.isPreempting(LeaderToken(ReplicaId(1), LogIdx(1))));

    REQUIRE(metadata.updateLeaderToken(nominee) == TernError::NO_ERROR);
    CHECK(metadata.getLeaderToken() == nominee);
    CHECK(metadata.getNomineeToken() == LeaderToken(0, 0));
    CHECK(
        metadata.updateLeaderToken(LeaderToken(ReplicaId(1), LogIdx(1))) ==
        TernError::LEADER_PREEMPTED);
}

TEST_CASE("LogMetadata drops unreleased entries after a skipped epoch") {
    _setCurrentTime(TernTime(1));
    LogMetadataTestDB testDB;
    LogsDBStats stats;
    DataPartitions data(testDB.env, *testDB.sharedDB);
    LogMetadata metadata(
        testDB.env,
        stats,
        *testDB.sharedDB,
        ReplicaId(0),
        data);
    REQUIRE(metadata.init(true));
    REQUIRE(data.init(true));

    data.writeLogEntries({
        makeEntry(1, "one"),
        makeEntry(2, "two"),
        makeEntry(3, "three"),
    });
    metadata.setLastReleased(LogIdx(2));

    LogsDBLogEntry entry;
    metadata.setNomineeToken(LeaderToken(ReplicaId(1), LogIdx(1)));
    CHECK(data.readLogEntry(LogIdx(3), entry) == TernError::NO_ERROR);

    metadata.setNomineeToken(LeaderToken(ReplicaId(1), LogIdx(2)));
    CHECK(
        data.readLogEntry(LogIdx(3), entry) ==
        TernError::LOG_ENTRY_MISSING);
}
