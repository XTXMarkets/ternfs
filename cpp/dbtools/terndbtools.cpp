// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>

#include <rocksdb/db.h>
#include <rocksdb/options.h>
#include <rocksdb/iterator.h>
#include <rocksdb/write_batch.h>
#include <rocksdb/utilities/checkpoint.h>

#include "BlockServicesCacheDB.hpp"
#include "CDCDB.hpp"
#include "CDCDBTools.hpp"
#include "Env.hpp"
#include "LogsDB.hpp"
#include "RegistryDB.hpp"
#include "ShardDB.hpp"
#include "ShardDBTools.hpp"

#define die(...) do { fprintf(stderr, __VA_ARGS__); exit(1); } while(false)

static void createCheckpoint(const std::string& dbPath, const std::string& checkpointPath) {
    Logger logger(LogLevel::LOG_INFO, STDERR_FILENO, false, false);
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "createCheckpoint");

    rocksdb::Options listOptions;
    std::vector<std::string> cfNames;
    auto s = rocksdb::DB::ListColumnFamilies(listOptions, dbPath, &cfNames);
    if (!s.ok()) {
        die("Failed to list column families in %s: %s\n", dbPath.c_str(), s.ToString().c_str());
    }

    std::unordered_map<std::string, rocksdb::ColumnFamilyOptions> knownCFOptions;
    auto addKnown = [&](const std::vector<rocksdb::ColumnFamilyDescriptor>& descriptors) {
        for (const auto& d : descriptors) {
            knownCFOptions[d.name] = d.options;
        }
    };
    addKnown(ShardDB::getColumnFamilyDescriptors());
    addKnown(LogsDB::getColumnFamilyDescriptors());
    addKnown(CDCDB::getColumnFamilyDescriptors());
    addKnown(BlockServicesCacheDB::getColumnFamilyDescriptors());
    addKnown(RegistryDB::getColumnFamilyDescriptors());

    std::vector<rocksdb::ColumnFamilyDescriptor> cfDescriptors;
    for (const auto& name : cfNames) {
        auto it = knownCFOptions.find(name);
        if (it != knownCFOptions.end()) {
            cfDescriptors.emplace_back(name, it->second);
        } else {
            cfDescriptors.emplace_back(name, rocksdb::ColumnFamilyOptions());
        }
    }

    rocksdb::Options dbOptions;
    dbOptions.compression = rocksdb::kLZ4Compression;
    dbOptions.bottommost_compression = rocksdb::kZSTD;
    dbOptions.create_if_missing = false;
    dbOptions.create_missing_column_families = false;
    dbOptions.max_open_files = 1000;

    rocksdb::DB* db = nullptr;
    std::vector<rocksdb::ColumnFamilyHandle*> handles;
    s = rocksdb::DB::OpenForReadOnly(dbOptions, dbPath, cfDescriptors, &handles, &db);
    if (!s.ok()) {
        die("Failed to open DB %s: %s\n", dbPath.c_str(), s.ToString().c_str());
    }

    LOG_INFO(env, "Creating checkpoint %s -> %s", dbPath, checkpointPath);
    rocksdb::Checkpoint* checkpoint = nullptr;
    s = rocksdb::Checkpoint::Create(db, &checkpoint);
    if (!s.ok()) {
        die("Failed to create checkpoint object: %s\n", s.ToString().c_str());
    }
    std::unique_ptr<rocksdb::Checkpoint> checkpointGuard(checkpoint);

    s = checkpoint->CreateCheckpoint(checkpointPath);
    if (!s.ok()) {
        die("Failed to create checkpoint at %s: %s\n", checkpointPath.c_str(), s.ToString().c_str());
    }

    for (auto* h : handles) db->DestroyColumnFamilyHandle(h);
    delete db;

    LOG_INFO(env, "Checkpoint created: %s", checkpointPath);
}

static void copyDB(const std::string& srcPath, const std::string& dstPath) {
    Logger logger(LogLevel::LOG_INFO, STDERR_FILENO, false, false);
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "copyDB");

    rocksdb::Options listOptions;
    std::vector<std::string> cfNames;
    auto s = rocksdb::DB::ListColumnFamilies(listOptions, srcPath, &cfNames);
    if (!s.ok()) {
        die("Failed to list column families in %s: %s\n", srcPath.c_str(), s.ToString().c_str());
    }

    LOG_INFO(env, "Found %s column families in %s", cfNames.size(), srcPath);
    for (const auto& name : cfNames) {
        LOG_INFO(env, "  %s", name);
    }

    std::unordered_map<std::string, rocksdb::ColumnFamilyOptions> knownCFOptions;
    auto addKnown = [&](const std::vector<rocksdb::ColumnFamilyDescriptor>& descriptors) {
        for (const auto& d : descriptors) {
            knownCFOptions[d.name] = d.options;
        }
    };
    addKnown(ShardDB::getColumnFamilyDescriptors());
    addKnown(LogsDB::getColumnFamilyDescriptors());
    addKnown(CDCDB::getColumnFamilyDescriptors());
    addKnown(BlockServicesCacheDB::getColumnFamilyDescriptors());
    addKnown(RegistryDB::getColumnFamilyDescriptors());

    std::vector<rocksdb::ColumnFamilyDescriptor> cfDescriptors;
    for (const auto& name : cfNames) {
        auto it = knownCFOptions.find(name);
        if (it != knownCFOptions.end()) {
            cfDescriptors.emplace_back(name, it->second);
        } else {
            LOG_INFO(env, "  Warning: CF '%s' not in known descriptors, using default options", name);
            cfDescriptors.emplace_back(name, rocksdb::ColumnFamilyOptions());
        }
    }

    rocksdb::Options srcOptions;
    srcOptions.compression = rocksdb::kLZ4Compression;
    srcOptions.bottommost_compression = rocksdb::kZSTD;
    srcOptions.create_if_missing = false;
    srcOptions.create_missing_column_families = false;
    srcOptions.max_open_files = 1000;

    rocksdb::DB* srcDB = nullptr;
    std::vector<rocksdb::ColumnFamilyHandle*> srcHandles;
    s = rocksdb::DB::OpenForReadOnly(srcOptions, srcPath, cfDescriptors, &srcHandles, &srcDB);
    if (!s.ok()) {
        die("Failed to open source DB %s: %s\n", srcPath.c_str(), s.ToString().c_str());
    }

    rocksdb::Options dstOptions;
    dstOptions.compression = rocksdb::kLZ4Compression;
    dstOptions.bottommost_compression = rocksdb::kZSTD;
    dstOptions.create_if_missing = true;
    dstOptions.create_missing_column_families = true;
    dstOptions.max_open_files = 1000;

    rocksdb::DB* dstDB = nullptr;
    std::vector<rocksdb::ColumnFamilyHandle*> dstHandles;
    s = rocksdb::DB::Open(dstOptions, dstPath, cfDescriptors, &dstHandles, &dstDB);
    if (!s.ok()) {
        die("Failed to open destination DB %s: %s\n", dstPath.c_str(), s.ToString().c_str());
    }

    static constexpr size_t BATCH_SIZE = 1024;

    for (size_t i = 0; i < cfNames.size(); i++) {
        bool hasMergeOperator = cfDescriptors[i].options.merge_operator != nullptr;

        uint64_t estimatedKeys = 0;
        srcDB->GetIntProperty(srcHandles[i], "rocksdb.estimate-num-keys", &estimatedKeys);
        uint64_t nextProgressThreshold = estimatedKeys > 0 ? estimatedKeys / 10 : 0;
        unsigned progressPct = 0;

        LOG_INFO(env, "Copying column family '%s'%s (estimated %s keys)...",
                 cfNames[i], hasMergeOperator ? " (has merge operator, dropping zero values)" : "", estimatedKeys);

        size_t keysCopied = 0;
        size_t keysDropped = 0;
        size_t totalKeyBytes = 0;
        size_t totalValueBytes = 0;

        auto* it = srcDB->NewIterator(rocksdb::ReadOptions(), srcHandles[i]);
        rocksdb::WriteBatch batch;

        for (it->SeekToFirst(); it->Valid(); it->Next()) {
            // For CFs with merge operators, the iterator resolves all merge
            // operands. If the final value is all-zero (e.g. +1/-1 that
            // cancelled out), skip it — it's dead weight.
            if (hasMergeOperator) {
                auto val = it->value();
                bool allZero = true;
                for (size_t b = 0; b < val.size(); b++) {
                    if (val.data()[b] != 0) { allZero = false; break; }
                }
                if (allZero) {
                    keysDropped++;
                    continue;
                }
            }

            batch.Put(dstHandles[i], it->key(), it->value());
            keysCopied++;
            totalKeyBytes += it->key().size();
            totalValueBytes += it->value().size();

            if (keysCopied % BATCH_SIZE == 0) {
                s = dstDB->Write(rocksdb::WriteOptions(), &batch);
                if (!s.ok()) {
                    die("Failed to write batch to CF '%s': %s\n", cfNames[i].c_str(), s.ToString().c_str());
                }
                batch.Clear();
            }

            if (nextProgressThreshold > 0 && (keysCopied + keysDropped) >= nextProgressThreshold) {
                progressPct += 10;
                LOG_INFO(env, "  %s%% (%s keys copied, %s dropped)", progressPct, keysCopied, keysDropped);
                uint64_t pctThreshold = estimatedKeys > 0 ? (uint64_t)(estimatedKeys * (progressPct + 10) / 100) : 0;
                nextProgressThreshold = std::max<uint64_t>(pctThreshold, keysCopied + keysDropped + 100000000ull);
            }
        }
        if (!it->status().ok()) {
            die("Iterator error on CF '%s': %s\n", cfNames[i].c_str(), it->status().ToString().c_str());
        }

        if (batch.Count() > 0) {
            s = dstDB->Write(rocksdb::WriteOptions(), &batch);
            if (!s.ok()) {
                die("Failed to write final batch to CF '%s': %s\n", cfNames[i].c_str(), s.ToString().c_str());
            }
        }

        delete it;
        LOG_INFO(env, "  Done: %s keys copied, %s keys dropped, %s key bytes, %s value bytes",
                 keysCopied, keysDropped, totalKeyBytes, totalValueBytes);
    }

    for (auto* h : srcHandles) srcDB->DestroyColumnFamilyHandle(h);
    for (auto* h : dstHandles) dstDB->DestroyColumnFamilyHandle(h);
    delete srcDB;
    delete dstDB;

    LOG_INFO(env, "Copy complete: %s -> %s", srcPath, dstPath);
}

static void usage(const char* binary) {
    fprintf(stderr, "Usage: %s COMMAND ARGS\n\n", binary);
    fprintf(stderr, "Commands:\n");
    fprintf(stderr, "  verify-equal DB1_PATH DB2_PATH\n");
    fprintf(stderr, "       Verifies two databases are the same.\n");
    fprintf(stderr, "  unreleased-state DB_PATH\n");
    fprintf(stderr, "       Outputs state of unreleased entries in DB.\n");
    fprintf(stderr, "  fsck DB_PATH\n");
    fprintf(stderr, "       Performs various integrity checks on the RocksDB state. The RocksDB database will be opened as read only.\n");
    fprintf(stderr, "  shard-log-entries DB_PATH START_LOG_IDX ENTRY_COUNT\n");
    fprintf(stderr, "       Outputs entries from distributed log. The RocksDB database will be opened as read only.\n");
    fprintf(stderr, "  cdc-log-entries DB_PATH START_LOG_IDX ENTRY_COUNT\n");
    fprintf(stderr, "       Outputs entries from distributed log. The RocksDB database will be opened as read only.\n");
    fprintf(stderr, "  sample-files DB_PATH OUTPUT_FILE_PATH\n");
    fprintf(stderr, "       Outputs per directory usage statistics in binary format. The RocksDB database will be opened as read only.\n");
    fprintf(stderr, "  find-failure-domain-duplicates DB_PATH OUTPUT_FILE_PATH\n");
    fprintf(stderr, "       Outputs files with spans that have duplicate failure domain, along with how many failure domain failures they can tolerate.\n");
    fprintf(stderr, "  block-service-usage DB_PATH OUTPUT_FILE_PATH\n");
    fprintf(stderr, "       Aggregates expected block usage per block service\n");
    fprintf(stderr, "  copy-db OLD_DB_PATH NEW_DB_PATH\n");
    fprintf(stderr, "       Copies all column families from OLD_DB_PATH to NEW_DB_PATH by iterating all keys.\n");
    fprintf(stderr, "  create-checkpoint DB_PATH CHECKPOINT_PATH\n");
    fprintf(stderr, "       Creates a RocksDB checkpoint (hardlink snapshot) of DB_PATH at CHECKPOINT_PATH.\n");
    fprintf(stderr, "  rebuild-block-services-to-files DB_PATH\n");
    fprintf(stderr, "       Drops and recreates the blockServicesToFiles CF by scanning all spans.\n");
}

int main(int argc, char** argv) {
    const auto dieWithUsage = [&argv]() {
        usage(argv[0]);
        exit(2);
    };
    if (argc == 1) {
        dieWithUsage();
    }

    for (int i = 1; i < argc; i++) {
        const auto getNextArg = [argc, &argv, &dieWithUsage, &i]() {
            if (i+1 >= argc) {
                fprintf(stderr, "Argument list ended too early.\n\n");
                dieWithUsage();
            }
            std::string arg(argv[i+1]);
            i++;
            return arg;
        };
        std::string arg = argv[i];
        if (arg == "verify-equal") {
            std::string db1Path = getNextArg();
            std::string db2Path = getNextArg();
            ShardDBTools::verifyEqual(db1Path, db2Path);
        } else if (arg == "unreleased-state") {
            std::string db1Path = getNextArg();
            ShardDBTools::outputUnreleasedState(db1Path);
        } else if (arg == "shard-log-entries") {
            std::string dbPath = getNextArg();
            LogIdx startIdx = std::stoull(getNextArg());
            size_t count = std::stoull(getNextArg());
            ShardDBTools::outputLogEntries(dbPath, startIdx, count);
        } else if (arg == "cdc-log-entries") {
            std::string dbPath = getNextArg();
            LogIdx startIdx = std::stoull(getNextArg());
            size_t count = std::stoull(getNextArg());
            CDCDBTools::outputLogEntries(dbPath, startIdx, count);
        } else if (arg == "cdc-verify-dirs-to-txn") {
            std::string dbPath = getNextArg();
            CDCDBTools::verifyDirsToTxnCf(dbPath);
        } else if (arg == "fsck") {
            std::string dbPath = getNextArg();
            ShardDBTools::fsck(dbPath);
        } else if (arg == "sample-files") {
            std::string dbPath = getNextArg();
            std::string outputFilePath = getNextArg();
            ShardDBTools::sampleFiles(dbPath, outputFilePath);
        } else if (arg == "find-failure-domain-duplicates") {
            std::string dbPath = getNextArg();
            std::string outputFilePath = getNextArg();
            ShardDBTools::outputFilesWithDuplicateFailureDomains(dbPath, outputFilePath);
        } else if (arg == "block-service-usage") {
            std::string dbPath = getNextArg();
            std::string outputFilePath = getNextArg();
            ShardDBTools::outputBlockServiceUsage(dbPath, outputFilePath);
        } else if (arg == "copy-db") {
            std::string oldPath = getNextArg();
            std::string newPath = getNextArg();
            copyDB(oldPath, newPath);
        } else if (arg == "create-checkpoint") {
            std::string dbPath = getNextArg();
            std::string checkpointPath = getNextArg();
            createCheckpoint(dbPath, checkpointPath);
        } else if (arg == "rebuild-block-services-to-files") {
            std::string dbPath = getNextArg();
            ShardDBTools::rebuildBlockServicesToFiles(dbPath);
        } else {
            dieWithUsage();
        }
    }

    return 0;
}
