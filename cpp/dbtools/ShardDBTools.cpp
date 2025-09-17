// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "ShardDBTools.hpp"

#include <chrono>
#include <random>
#include <cstdint>
#include <optional>
#include <memory>
#include <regex>
#include <rocksdb/db.h>
#include <rocksdb/slice.h>
#include <rocksdb/write_batch.h>
#include <unordered_map>
#include <vector>
#include <iostream>
#include <fstream>

#include "Bincode.hpp"
#include "BlockServicesCacheDB.hpp"
#include "Common.hpp"
#include "Env.hpp"
#include "LogsDB.hpp"
#include "LogsDBTools.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "RocksDBUtils.hpp"
#include "ShardDB.hpp"
#include "SharedRocksDB.hpp"
#include "ShardDBData.hpp"
#include "Time.hpp"

namespace rocksdb {
std::ostream& operator<<(std::ostream& out, const rocksdb::Slice& slice) {
    return goLangBytesFmt(out, (const char*)slice.data(), slice.size());
}
}


void ShardDBTools::verifyEqual(const std::string &db1Path, const std::string &db2Path) {
    Logger logger(LogLevel::LOG_INFO, STDERR_FILENO, false, false);
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "ShardDBTools");
    SharedRocksDB sharedDb1(logger, xmon, db1Path, "");
    sharedDb1.registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());
    sharedDb1.registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
    rocksdb::Options rocksDBOptions;
    rocksDBOptions.compression = rocksdb::kLZ4Compression;
    rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
    sharedDb1.openForReadOnly(rocksDBOptions);
    SharedRocksDB sharedDb2(logger, xmon, db2Path, "");
    sharedDb2.registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());
    sharedDb2.registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
    sharedDb2.openForReadOnly(rocksDBOptions);
    auto db1 = sharedDb1.db();
    auto db2 = sharedDb2.db();

    size_t totalKeysCompared{0};
    size_t totalKeySize{0};
    size_t totalValueSize{0};

    for(const auto& cf : ShardDB::getColumnFamilyDescriptors()) {
        LOG_INFO(env, "Starting comparison on CF %s", cf.name);
        auto cf1 = sharedDb1.getCF(cf.name);
        auto cf2 = sharedDb2.getCF(cf.name);

        size_t keysCompared{0};
        size_t keySize{0};
        size_t valueSize{0};

        auto it1 = db1->NewIterator({}, cf1);
        auto it2 = db2->NewIterator({}, cf2);
        it1->SeekToFirst();
        it2->SeekToFirst();
        while (it1->Valid() && it2->Valid()) {
            if (it1->key() != it2->key()) {
                LOG_ERROR(env, "Found mismatch in key cf %s, key1: %s, key2: %s", cf.name, it1->key(), it2->key());
                return;
            }
            if (it1->value() != it2->value()) {
                LOG_ERROR(env, "Found mismatch in value cf %s", cf.name);
                return;
            }
            ++keysCompared;
            keySize += it1->key().size();
            valueSize += it1->value().size();
            if (keysCompared % 100000000ull == 0) {
                LOG_INFO(env, "Compared %s key/value pairs so far and they are identical", keysCompared);
            }

            it1->Next();
            it2->Next();
        }
        if (it1->Valid()) {
            LOG_ERROR(env, "Database %s has extra keys in cf %s", db1Path, cf.name);
            return;
        }
        if (it2->Valid()) {
            LOG_ERROR(env, "Database %s has extra keys in cf %s", db2Path, cf.name);
            return;
        }
        delete it1;
        delete it2;

        LOG_INFO(env, "CF %s identical. Compared %s key/value pairs. KeySize: %s, ValueSize: %s", cf.name, keysCompared, keySize, valueSize);
        totalKeysCompared += keysCompared;
        totalKeySize += keySize;
        totalValueSize += valueSize;
    }
    LOG_INFO(env, "Databases identical! Compared %s cfs, %s key/value pairs. Total key size: %s, Total value size: %s", ShardDB::getColumnFamilyDescriptors().size(), totalKeysCompared, totalKeySize, totalValueSize);
}

std::ostream& operator<<(std::ostream& out, const std::vector<LogIdx>& indices) {
    out << "[";
    for (auto idx : indices) {
        out << idx << ", ";
    }
    out << "]";
    return out;
}

void ShardDBTools::outputUnreleasedState(const std::string& dbPath) {
    Logger logger(LogLevel::LOG_INFO, STDERR_FILENO, false, false);
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "ShardDBTools");
    SharedRocksDB sharedDb(logger, xmon, dbPath, "");
    sharedDb.registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());
    sharedDb.registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
    rocksdb::Options rocksDBOptions;
    rocksDBOptions.compression = rocksdb::kLZ4Compression;
    rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
    sharedDb.openForReadOnly(rocksDBOptions);
    LogIdx lastReleased;
    std::vector<LogIdx> unreleasedLogEntries;

    LogsDBTools::getUnreleasedLogEntries(env, sharedDb, lastReleased, unreleasedLogEntries);

    LOG_INFO(env, "Last released: %s", lastReleased);
    LOG_INFO(env, "Unreleased entries: %s", unreleasedLogEntries);
}

void ShardDBTools::outputLogEntries(const std::string& dbPath, LogIdx startIdx, size_t count) {
    Logger logger(LogLevel::LOG_INFO, STDERR_FILENO, false, false);
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "ShardDBTools");
    SharedRocksDB sharedDb(logger, xmon, dbPath, "");
    sharedDb.registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());
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
            ShardLogEntry shardEntry;
            shardEntry.unpack(buf);
            LOG_INFO(env, "%s", shardEntry);
        }
        if (logEntries.size() == entriesAtOnce) {
            startIdx = logEntries.back().idx + 1;
            count -= entriesAtOnce;
        } else {
            break;
        }
    }
}

void ShardDBTools::fsck(const std::string& dbPath) {
    Logger logger(LogLevel::LOG_INFO, STDERR_FILENO, false, false);
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "ShardDBTools");
    SharedRocksDB sharedDb(logger, xmon, dbPath, "");
    sharedDb.registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());
    sharedDb.registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
    rocksdb::Options rocksDBOptions;
    rocksDBOptions.compression = rocksdb::kLZ4Compression;
    rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
    sharedDb.openForReadOnly(rocksDBOptions);
    auto db = sharedDb.db();
    bool anyErrors = false;

#define ERROR(...) do { \
        LOG_ERROR(env, __VA_ARGS__); \
        anyErrors = true; \
    } while (0)

    const rocksdb::Snapshot* snapshotPtr = db->GetSnapshot();
    ALWAYS_ASSERT(snapshotPtr != nullptr);
    const auto snapshotDeleter = [db](const rocksdb::Snapshot* ptr) { db->ReleaseSnapshot(ptr); };
    std::unique_ptr<const rocksdb::Snapshot, decltype(snapshotDeleter)> snapshot(snapshotPtr, snapshotDeleter);

    auto directoriesCf = sharedDb.getCF("directories");
    auto edgesCf = sharedDb.getCF("edges");
    auto filesCf = sharedDb.getCF("files");
    auto spansCf = sharedDb.getCF("spans");
    auto transientFilesCf = sharedDb.getCF("transientFiles");
    auto blockServicesToFilesCf = sharedDb.getCF("blockServicesToFiles");

    rocksdb::ReadOptions options;
    options.snapshot = snapshot.get();

    LOG_INFO(env, "This will take a while and will take 10s of gigabytes of RAM.");

    {
        LOG_INFO(env, "Directories pass");
        uint64_t analyzedDirs = 0;
        std::unique_ptr<rocksdb::Iterator> it(db->NewIterator(options, directoriesCf));
        for (it->SeekToFirst(); it->Valid(); it->Next()) {
            analyzedDirs++;
            auto dirK = ExternalValue<InodeIdKey>::FromSlice(it->key());
            if (dirK().id().type() != InodeType::DIRECTORY) {
                ERROR("Found key with bad directory inode %s", dirK().id());
                continue;
            }
            auto _ = ExternalValue<DirectoryBody>::FromSlice(it->value()); // just to check that this works
        }
        ROCKS_DB_CHECKED(it->status());
        LOG_INFO(env, "Analyzed %s directories", analyzedDirs);
    }

    {
        LOG_INFO(env, "Edges pass");
        const rocksdb::ReadOptions options;
        std::unordered_set<InodeId> ownedInodes;
        auto dummyCurrentDirectory = InodeId::FromU64Unchecked(1ull << 63);
        auto thisDir = dummyCurrentDirectory;
        bool thisDirHasCurrentEdges = false;
        TernTime thisDirMaxTime = 0;
        std::string thisDirMaxTimeEdge;
        // the last edge for a given name, in a given directory
        std::unordered_map<std::string, std::pair<StaticValue<EdgeKey>, StaticValue<SnapshotEdgeBody>>> thisDirLastEdges;
        uint64_t analyzedEdges = 0;
        uint64_t analyzedDirectories = 0;
        std::unique_ptr<rocksdb::Iterator> it(db->NewIterator(options, edgesCf));
        for (it->SeekToFirst(); it->Valid(); it->Next()) {
            analyzedEdges++;
            auto edgeK = ExternalValue<EdgeKey>::FromSlice(it->key());
            if (edgeK().dirId().type() != InodeType::DIRECTORY) {
                ERROR("Found edge with bad inode id %s", edgeK().dirId());
                continue;
            }
            if (edgeK().dirId() != thisDir) {
                analyzedDirectories++;
                if (thisDir != dummyCurrentDirectory) {
                    auto dirIdK = InodeIdKey::Static(thisDir);
                    std::string dirValue;
                    auto status = db->Get(options, directoriesCf, dirIdK.toSlice(), &dirValue);
                    if (status.IsNotFound()) {
                        // the directory must exist
                        ERROR("Directory %s is referenced in edges, but it does not exist.", thisDir);
                    } else {
                        ROCKS_DB_CHECKED(status);
                        auto dir = ExternalValue<DirectoryBody>(dirValue);
                        // the mtime must be greater or equal than all edges
                        if (dir().mtime() < thisDirMaxTime) {
                            ERROR("Directory %s has time %s, but max edge with name %s has greater time %s", thisDir, dir().mtime(), thisDirMaxTimeEdge, thisDirMaxTime);
                        }
                        // if we have current edges, the directory can't be snapshot
                        if (thisDirHasCurrentEdges && thisDir != ROOT_DIR_INODE_ID && dir().ownerId() == NULL_INODE_ID) {
                            ERROR("Directory %s has current edges, but is snaphsot (no owner)", thisDir);
                        }
                    }
                }
                thisDir = edgeK().dirId();
                thisDirLastEdges.clear();
                thisDirHasCurrentEdges = false;
                thisDirMaxTime = 0;
            }
            auto nameHash = edgeK().nameHash();
            std::string name(edgeK().name().data(), edgeK().name().size());
            if (nameHash != EdgeKey::computeNameHash(HashMode::XXH3_63, BincodeBytesRef(edgeK().name().data(), edgeK().name().size()))) {
                ERROR("Edge %s has mismatch between name and nameHash", edgeK());
            }
            InodeId ownedTargetId = NULL_INODE_ID;
            TernTime creationTime;
            std::optional<ExternalValue<CurrentEdgeBody>> currentEdge;
            std::optional<ExternalValue<SnapshotEdgeBody>> snapshotEdge;
            if (edgeK().current()) {
                currentEdge = ExternalValue<CurrentEdgeBody>::FromSlice(it->value());
                ownedTargetId = (*currentEdge)().targetId();
                creationTime = (*currentEdge)().creationTime();
                thisDirHasCurrentEdges = true;
            } else {
                snapshotEdge = ExternalValue<SnapshotEdgeBody>::FromSlice(it->value());
                creationTime = edgeK().creationTime();
                if ((*snapshotEdge)().targetIdWithOwned().extra()) {
                    ownedTargetId = (*snapshotEdge)().targetIdWithOwned().id();
                    // we can't have an owned directory snapshot edge
                    if (ownedTargetId.type() == InodeType::DIRECTORY) {
                        ERROR("Directory %s has a snapshot, owned edge to directory %s", thisDir, ownedTargetId);
                    }
                }
            }
            if (ownedTargetId != NULL_INODE_ID) {
                if (ownedInodes.contains(ownedTargetId)) {
                    ERROR("Inode %s is owned twice!", ownedTargetId);
                }
                ownedInodes.emplace(ownedTargetId);
            }
            if (creationTime > thisDirMaxTime) {
                thisDirMaxTime = creationTime;
                thisDirMaxTimeEdge = name;
            }
            // No intra directory dangling pointers
            if (ownedTargetId != NULL_INODE_ID && ownedTargetId.shard() == thisDir.shard()) {
                auto idK = InodeIdKey::Static(ownedTargetId);
                if (ownedTargetId.type() == InodeType::DIRECTORY) {
                    std::string tmp;
                    auto status = db->Get(options, directoriesCf, idK.toSlice(), &tmp);
                    if (status.IsNotFound()) {
                        ERROR("Directory %s is referenced in directory %s but I could not find it.", ownedTargetId, edgeK().dirId());
                    } else {
                        ROCKS_DB_CHECKED(status);
                    }
                } else {
                    std::string tmp;
                    auto status = db->Get(options, filesCf, idK.toSlice(), &tmp);
                    if (status.IsNotFound()) {
                        ERROR("File %s is referenced in directory %s but I could not find it.", ownedTargetId, edgeK().dirId());
                    } else {
                        ROCKS_DB_CHECKED(status);
                    }
                }
            }
            {
                auto prevEdge = thisDirLastEdges.find(name);
                if (prevEdge != thisDirLastEdges.end()) {
                    TernTime prevCreationTime = prevEdge->second.first().creationTime();
                    // The edge must be newer than every non-current edge before it, with the exception of deletion edges
                    // (when we override the deletion edge)
                    if (
                        (prevCreationTime > creationTime) ||
                        (prevCreationTime == creationTime && !(snapshotEdge && (*snapshotEdge)().targetIdWithOwned().id() == NULL_INODE_ID))
                    ) {
                        ERROR("Name %s in directory %s has overlapping edges (%s >= %s).", name, edgeK().dirId(), prevCreationTime, creationTime);
                    }
                    // Deletion edges are not preceded by another deletion edge
                    if (snapshotEdge && (*snapshotEdge)().targetIdWithOwned().id() == NULL_INODE_ID && prevEdge->second.second().targetIdWithOwned().id() == NULL_INODE_ID) {
                        ERROR("Deletion edge with name %s creation time %s is preceded by another deletion edge in directory %s", name, edgeK().creationTime(), thisDir);
                    }
                } else {
                    // Deletion edges are preceded by something not -- this is actually not enforced
                    // by the schema, but by GC. So it's not a huge deal if it's wrong.
                    if (snapshotEdge && (*snapshotEdge)().targetIdWithOwned().id() == NULL_INODE_ID) {
                        ERROR("Deletion edge with name %s creation time %s is not preceded by anything in directory %s", name, edgeK().creationTime(), thisDir);
                    }
                }
            }
            // Add last edge (only snapshot edges can have predecessors)
            if (snapshotEdge) {
                StaticValue<EdgeKey> staticEdgeK(edgeK());
                StaticValue<SnapshotEdgeBody> staticEdgeV((*snapshotEdge)());
                thisDirLastEdges[name] = {staticEdgeK, staticEdgeV};
            }
        }
        ROCKS_DB_CHECKED(it->status());
        LOG_INFO(env, "Analyzed %s edges in %s directories", analyzedEdges, analyzedDirectories);
    }

    struct HashBlockServiceInodeId {
        std::size_t operator () (const std::pair<BlockServiceId, InodeId>& k) const {
            return std::hash<uint64_t>{}(k.first.u64) ^ std::hash<uint64_t>{}(k.second.u64);
        }
    };

    std::unordered_map<std::pair<BlockServiceId, InodeId>, int64_t, HashBlockServiceInodeId> blockServicesToFiles;
    std::unordered_map<InodeId, uint64_t> filesWithSpans; // file to expected size
    {
        LOG_INFO(env, "Spans pass");
        std::unique_ptr<rocksdb::Iterator> it(db->NewIterator(options, spansCf));
        InodeId thisFile = NULL_INODE_ID;
        uint64_t expectedNextOffset = 0;
        uint64_t analyzedSpans = 0;
        uint64_t analyzedFiles = 0;
        for (it->SeekToFirst(); it->Valid(); it->Next()) {
            analyzedSpans++;
            auto spanK = ExternalValue<SpanKey>::FromSlice(it->key());
            if (spanK().fileId().type() != InodeType::FILE && spanK().fileId().type() != InodeType::SYMLINK) {
                ERROR("Found span with bad file id %s", spanK().fileId());
                continue;
            }
            if (spanK().fileId() != thisFile) {
                analyzedFiles++;
                expectedNextOffset = 0;
                thisFile = spanK().fileId();
            }
            // check that file exists (either transient or non)
            {
                auto idK = InodeIdKey::Static(thisFile);
                std::string tmp;
                auto status = db->Get(options, filesCf, idK.toSlice(), &tmp);
                if (status.IsNotFound()) {
                    auto status = db->Get(options, transientFilesCf, idK.toSlice(), &tmp);
                    if (status.IsNotFound()) {
                        ERROR("We have a span for non-esistant file %s", thisFile);
                    } else {
                        ROCKS_DB_CHECKED(status);
                    }
                } else {
                    ROCKS_DB_CHECKED(status);
                }
            }
            // Record blocks
            auto spanV = ExternalValue<SpanBody>::FromSlice(it->value());
            filesWithSpans[spanK().fileId()] = spanK().offset() + spanV().spanSize();
            if (spanK().offset() != expectedNextOffset) {
                ERROR("Expected offset %s, got %s in span for file %s", expectedNextOffset, spanK().offset(), thisFile);
            }
            expectedNextOffset = spanK().offset() + spanV().spanSize();
            if (!spanV().isInlineStorage()) {
                for (uint8_t i = 0; i < spanV().locationCount(); ++i) {
                    auto blocksBody = spanV().blocksBodyReadOnly(i);
                    if (blocksBody.storageClass() == EMPTY_STORAGE) {
                        ERROR("File %s has empty storage span at offset %s", thisFile, spanK().offset());
                    } else {
                        for (int i = 0; i < blocksBody.parity().blocks(); i++) {
                            auto block = blocksBody.block(i);
                            blockServicesToFiles[std::pair<BlockServiceId, InodeId>(block.blockService(), thisFile)] += 1;
                        }
                    }
                }
            }
        }
        ROCKS_DB_CHECKED(it->status());
        LOG_INFO(env, "Analyzed %s spans in %s files", analyzedSpans, analyzedFiles);
    }

    const auto filesPass = [&env, &anyErrors, db, &options, &filesWithSpans]<typename T>(rocksdb::ColumnFamilyHandle* cf, const std::string& what) {
        uint64_t analyzedFiles = 0;
        std::unique_ptr<rocksdb::Iterator> it(db->NewIterator(options, cf));
        for (it->SeekToFirst(); it->Valid(); it->Next()) {
            analyzedFiles++;
            auto fileK = ExternalValue<InodeIdKey>::FromSlice(it->key());
            InodeId fileId = fileK().id();
            if (fileId.type() != InodeType::FILE && fileId.type() != InodeType::SYMLINK) {
                ERROR("Found %s with bad file id %s", what, fileId);
                continue;
            }
            auto fileV = ExternalValue<T>::FromSlice(it->value());
            if (fileV().fileSize() == 0) { continue; } // nothing in this
            auto spansSize = filesWithSpans.find(fileId);
            if (spansSize == filesWithSpans.end()) {
                ERROR("Could not find spans for non-zero-sized file %s", fileId);
            } else if (spansSize->second != fileV().fileSize()) {
                ERROR("Spans tell us file %s should be of size %s, but it actually has size %s", fileId, spansSize->second, fileV().fileSize());
            }
        }
        ROCKS_DB_CHECKED(it->status());
        LOG_INFO(env, "Analyzed %s %s", analyzedFiles, what);
    };

    LOG_INFO(env, "Files pass");
    filesPass.template operator()<FileBody>(filesCf, "files");

    LOG_INFO(env, "Transient files pass");
    filesPass.template operator()<TransientFileBody>(transientFilesCf, "transient files");

    LOG_INFO(env, "Block services to files pass");
    {
        uint64_t foundItems = 0;
        std::unique_ptr<rocksdb::Iterator> it(db->NewIterator(options, blockServicesToFilesCf));
        for (it->SeekToFirst(); it->Valid(); it->Next()) {
            auto k = ExternalValue<BlockServiceToFileKey>::FromSlice(it->key());
            if (k().fileId().type() != InodeType::FILE && k().fileId().type() != InodeType::SYMLINK) {
                ERROR("Found block service to files with bad file id %s", k().fileId());
                continue;
            }
            auto v = ExternalValue<I64Value>::FromSlice(it->value());
            if (v().i64() != 0) { foundItems++; }
            auto expected = blockServicesToFiles.find(std::pair<BlockServiceId, InodeId>(k().blockServiceId(), k().fileId()));
            if (expected == blockServicesToFiles.end() && v().i64() != 0) {
                ERROR("Found spurious block services to file entry for block service %s, file %s, with count %s", k().blockServiceId(), k().fileId(), v().i64());
            }
            if (expected != blockServicesToFiles.end() && v().i64() != expected->second) {
                ERROR("Found wrong block services to file entry for block service %s, file %s, expected count %s, got %s", k().blockServiceId(), k().fileId(), expected->second, v().i64());
            }
        }
        ROCKS_DB_CHECKED(it->status());
        if (foundItems != blockServicesToFiles.size()) {
            ERROR("Expected %s block services to files, got %s", blockServicesToFiles.size(), foundItems);
        }
        LOG_INFO(env, "Analyzed %s block service to files entries", foundItems);
    }

    ALWAYS_ASSERT(!anyErrors, "Some errors detected, check log");

#undef ERROR
}

struct StorageClassSize {
    uint64_t flash{0};
    uint64_t hdd{0};
};

typedef std::array<StorageClassSize, 3> LocationSize;
struct SizePerStorageClass {
    uint64_t logical{0};
    LocationSize size{StorageClassSize{0,0},StorageClassSize{0,0}, StorageClassSize{0,0}};
    uint64_t inMetadata{0};
};

struct FileInfo {
    TernTime mTime;
    TernTime aTime;
    SizePerStorageClass size;
    uint64_t size_weight;
};

static constexpr uint64_t  SAMPLE_RATE_SIZE_LIMIT = 10ull << 30;

void ShardDBTools::sampleFiles(const std::string& dbPath, const std::string& outputFilePath) {
    Logger logger(LogLevel::LOG_INFO, STDERR_FILENO, false, false);
    std::ofstream outputStream(outputFilePath);
    ALWAYS_ASSERT(outputStream.is_open(), "Failed to open output file for sampling results");
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "ShardDBTools");
    SharedRocksDB sharedDb(logger, xmon, dbPath, "");
    sharedDb.registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());
    sharedDb.registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
    rocksdb::Options rocksDBOptions;
    rocksDBOptions.compression = rocksdb::kLZ4Compression;
    rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
    sharedDb.openForReadOnly(rocksDBOptions);
    auto db = sharedDb.db();
    rocksdb::ReadOptions options;
    auto edgesCf = sharedDb.getCF("edges");
    auto filesCf = sharedDb.getCF("files");
    auto spansCf = sharedDb.getCF("spans");
    std::unordered_map<InodeId, FileInfo> files;
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_real_distribution<double> realDis(0.0, 1.0);
    {
        std::unique_ptr<rocksdb::Iterator> it(db->NewIterator(options, filesCf));
        for (it->SeekToFirst(); it->Valid(); it->Next()) {
            auto fileK = ExternalValue<InodeIdKey>::FromSlice(it->key());
            InodeId fileId = fileK().id();
            if (fileId.type() != InodeType::FILE) {
                continue;
            }
            auto fileV = ExternalValue<FileBody>::FromSlice(it->value());
            auto logicalSize = fileV().fileSize();
            if ( logicalSize == 0) { continue; } // nothing in this
            auto probability = (double)logicalSize / SAMPLE_RATE_SIZE_LIMIT;
            if( probability > realDis(gen)) {
                files.insert(std::make_pair(fileId, FileInfo{fileV().mtime(), fileV().atime(), SizePerStorageClass{logicalSize,{StorageClassSize{0,0},StorageClassSize{0,0}},0}, std::max(logicalSize, SAMPLE_RATE_SIZE_LIMIT)}));
            }
        }
        ROCKS_DB_CHECKED(it->status());
    }
    {
        std::unique_ptr<rocksdb::Iterator> it(db->NewIterator(options, spansCf));
        InodeId thisFile = NULL_INODE_ID;
        LocationSize thisFileSize{StorageClassSize{0,0},StorageClassSize{0,0}, StorageClassSize{0,0}};
        uint64_t thisFileInlineSize{0};
        for (it->SeekToFirst(); it->Valid(); it->Next()) {
            auto spanK = ExternalValue<SpanKey>::FromSlice(it->key());
            if (spanK().fileId().type() != InodeType::FILE) {
                continue;
            }
            if (spanK().fileId() != thisFile) {
                auto file_it = files.find(thisFile);
                if (file_it != files.end()) {
                    file_it->second.size.size = thisFileSize;
                    file_it->second.size.inMetadata = thisFileInlineSize;
                }
                thisFile = spanK().fileId();
                thisFileSize = LocationSize{StorageClassSize{0,0},StorageClassSize{0,0}, StorageClassSize{0,0}};
                thisFileInlineSize = 0;
            }

            const auto spanV = ExternalValue<SpanBody>::FromSlice(it->value());
            if (spanV().isInlineStorage()) {
                thisFileInlineSize += spanV().size();
                continue;
            }
            for(uint8_t i = 0; i < spanV().locationCount(); ++i) {
                auto blocksBody = spanV().blocksBodyReadOnly(i);
                auto physicalSize = (uint64_t)blocksBody.parity().blocks() * blocksBody.stripes() * blocksBody.cellSize();
                switch (blocksBody.storageClass()){
                    case HDD_STORAGE:
                        thisFileSize[blocksBody.location()].hdd += physicalSize;
                        break;
                    case FLASH_STORAGE:
                        thisFileSize[blocksBody.location()].flash += physicalSize;
                        break;
                }
            }
        }
        auto file_it = files.find(thisFile);
        if (file_it != files.end()) {
            file_it->second.size.size = thisFileSize;
            file_it->second.size.inMetadata = thisFileInlineSize;
        }
        ROCKS_DB_CHECKED(it->status());
    }
    {
        const rocksdb::ReadOptions options;
        std::unordered_set<InodeId> ownedInodes;
        auto dummyCurrentDirectory = InodeId::FromU64Unchecked(1ull << 63);
        auto thisDir = dummyCurrentDirectory;
        bool thisDirHasCurrentEdges = false;
        TernTime thisDirMaxTime = 0;
        std::string thisDirMaxTimeEdge;
        std::unique_ptr<rocksdb::Iterator> it(db->NewIterator(options, edgesCf));
        for (it->SeekToFirst(); it->Valid(); it->Next())
        {
            auto keyValue = it->key().ToString(); // we move iterator within the loop, we need to copy key
            auto edgeK = ExternalValue<EdgeKey>::FromSlice(rocksdb::Slice(keyValue));
            InodeId ownedTargetId = NULL_INODE_ID;
            std::optional<ExternalValue<CurrentEdgeBody>> currentEdge;
            std::optional<ExternalValue<SnapshotEdgeBody>> snapshotEdge;
            bool current = false;
            TernTime creationTime = 0;
            TernTime deletionTime = 0;
            if (edgeK().current()) {
                currentEdge = ExternalValue<CurrentEdgeBody>::FromSlice(it->value());
                ownedTargetId = (*currentEdge)().targetId();
                current = true;
                creationTime = (*currentEdge)().creationTime();
            } else {
                snapshotEdge = ExternalValue<SnapshotEdgeBody>::FromSlice(it->value());
                if (!(*snapshotEdge)().targetIdWithOwned().extra()) {
                    // we don't care about non-owned edges
                    continue;
                }
                ownedTargetId = (*snapshotEdge)().targetIdWithOwned().id();
                creationTime = edgeK().creationTime();
                // If this is a snapshot edge, then the one immediately following it should
                // be a deletion marker that indicates that the file was removed or moved elsewhere.
                // The creation time of the deletion marker is the deletion time of the previous edge.
                it->Next(); // Advancing here is fine because we already skip deletion markers during sampling.
                ALWAYS_ASSERT(it->Valid());
                auto deletionEdgeK = ExternalValue<EdgeKey>::FromSlice(it->key());
                if (deletionEdgeK().snapshot() && deletionEdgeK().name() == edgeK().name()) {
                    ALWAYS_ASSERT(deletionEdgeK().dirId() == edgeK().dirId());
                    deletionTime = deletionEdgeK().creationTime();
                    snapshotEdge = ExternalValue<SnapshotEdgeBody>::FromSlice(it->value());
                    if ((*snapshotEdge)().targetIdWithOwned().id().u64 != 0) {
                        // this is not a deletion edge so we rewind the iterator and continue.
                        it->Prev();
                    }
                } else {
                    // this edge has nothing to do with snapshot one we are looking at.
                    // we need to rewind and find current edge which overwrote it
                    it->Prev();
                    StaticValue<EdgeKey> currentEdgeK;
                    currentEdgeK().setDirIdWithCurrent(edgeK().dirId(), true);
                    currentEdgeK().setName(edgeK().name());
                    currentEdgeK().setNameHash(edgeK().nameHash());
                    std::string value;
                    ROCKS_DB_CHECKED(db->Get(options, edgesCf, currentEdgeK.toSlice(), &value));

                    currentEdge = ExternalValue<CurrentEdgeBody>::FromSlice(value);
                    deletionTime = (*currentEdge)().creationTime();
                }
            }
            auto file_id = files.find(ownedTargetId);
            if (file_id == files.end()) {
                // not sampled skip
                continue;
            }
            auto name = std::string(edgeK().name().data(), edgeK().name().size());
            outputStream << "\"" << std::regex_replace(name, std::regex(R"(")"), R"("")") << "\",";
            outputStream << edgeK().dirId() << ",";
            outputStream << ownedTargetId << ",";
            outputStream << current << ",";
            outputStream << file_id->second.size.logical << ",";
            outputStream << file_id->second.size.size[0].hdd << ",";
            outputStream << file_id->second.size.size[1].hdd << ",";
            outputStream << file_id->second.size.size[2].hdd << ",";
            outputStream << file_id->second.size.size[0].flash << ",";
            outputStream << file_id->second.size.size[1].flash << ",";
            outputStream << file_id->second.size.size[2].flash << ",";
            outputStream << file_id->second.size.inMetadata << ",";
            outputStream << creationTime << ",";
            outputStream << deletionTime << ",";
            outputStream << file_id->second.mTime << ",";
            outputStream << file_id->second.aTime << ",";
            outputStream << file_id->second.size_weight << "\n";
        }
        ROCKS_DB_CHECKED(it->status());
    }
    outputStream.close();
    ALWAYS_ASSERT(!outputStream.bad(), "An error occurred while closing the sample output file");
}

void ShardDBTools::outputFilesWithDuplicateFailureDomains(const std::string& dbPath, const std::string& outputFilePath) {
    Logger logger(LogLevel::LOG_INFO, STDERR_FILENO, false, false);
    std::ofstream outputStream(outputFilePath);
    ALWAYS_ASSERT(outputStream.is_open(), "Failed to open output file");
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "ShardDBTools");
    SharedRocksDB sharedDb(logger, xmon, dbPath, "");
    sharedDb.registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());
    sharedDb.registerCFDescriptors(BlockServicesCacheDB::getColumnFamilyDescriptors());

    rocksdb::Options rocksDBOptions;
    rocksDBOptions.compression = rocksdb::kLZ4Compression;
    rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
    sharedDb.openForReadOnly(rocksDBOptions);
    auto db = sharedDb.db();
    BlockServicesCacheDB blockServiceDB{logger, xmon, sharedDb};
    auto blockServiceCache = blockServiceDB.getCache();

    rocksdb::ReadOptions options;
    auto spansCf = sharedDb.getCF("spans");
    std::unordered_map<InodeId, uint8_t> filesToFailureDomainTolerance;
    std::unordered_set<std::string> failureDomains;
    {
        std::unique_ptr<rocksdb::Iterator> it(db->NewIterator(options, spansCf));
        for (it->SeekToFirst(); it->Valid(); it->Next()) {
            auto spanK = ExternalValue<SpanKey>::FromSlice(it->key());
            if (spanK().fileId().type() != InodeType::FILE) {
                continue;
            }

            auto spanV = ExternalValue<SpanBody>::FromSlice(it->value());
            if (spanV().isInlineStorage() == INLINE_STORAGE) {
                continue;
            }

            for (uint8_t i = 0; i < spanV().locationCount(); ++i) {
                failureDomains.clear();
                auto blocksBody = spanV().blocksBodyReadOnly(i);
                uint8_t failureToleranceCount = blocksBody.parity().parityBlocks();

                for (uint8_t i = 0; i < blocksBody.parity().blocks(); ++i) {
                    auto blockServiceId = blocksBody.block(i).blockService();

                    auto it = blockServiceCache.blockServices.find(blockServiceId.u64);
                    ALWAYS_ASSERT(it != blockServiceCache.blockServices.end());
                    auto failureDomainString = std::string((char*)(&it->second.failureDomain[0]), it->second.failureDomain.size());
                    failureDomains.insert(failureDomainString);
                }
                uint8_t duplicateFailureDomains = blocksBody.parity().blocks() - failureDomains.size();
                if (duplicateFailureDomains == 0) {
                    continue;
                }
                failureToleranceCount -= std::min(failureToleranceCount, duplicateFailureDomains);
                auto fileId = filesToFailureDomainTolerance.find(spanK().fileId());
                if (fileId != filesToFailureDomainTolerance.end()) {
                    fileId->second = std::min<uint8_t>(fileId->second, failureToleranceCount);
                } else {
                    filesToFailureDomainTolerance[spanK().fileId()] = failureToleranceCount;
                }
            }
        }
        ROCKS_DB_CHECKED(it->status());
    }
    if (!filesToFailureDomainTolerance.empty()) {
        outputStream << "[" << std::endl;

        for (auto it = filesToFailureDomainTolerance.begin(); it != filesToFailureDomainTolerance.end();) {
            outputStream << "{\"fileId\": \"" << it->first << "\", \"failure_domain_tolerance\": " << (int)it->second << "\"}";
            if (++it != filesToFailureDomainTolerance.end())
                outputStream << ",";
            outputStream << std::endl;
        }
        outputStream << "]" << std::endl;
    }
    outputStream.close();
    ALWAYS_ASSERT(!outputStream.bad(), "An error occurred while closing the sample output file");
}


void ShardDBTools::outputBlockServiceUsage(const std::string& dbPath, const std::string& outputFilePath) {
    Logger logger(LogLevel::LOG_INFO, STDERR_FILENO, false, false);
    std::ofstream outputStream(outputFilePath);
    ALWAYS_ASSERT(outputStream.is_open(), "Failed to open output file");
    std::shared_ptr<XmonAgent> xmon;
    Env env(logger, xmon, "ShardDBTools");
    SharedRocksDB sharedDb(logger, xmon, dbPath, "");
    sharedDb.registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());

    rocksdb::Options rocksDBOptions;
    rocksDBOptions.compression = rocksdb::kLZ4Compression;
    rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
    sharedDb.openForReadOnly(rocksDBOptions);
    auto db = sharedDb.db();

    rocksdb::ReadOptions options;
    auto spansCf = sharedDb.getCF("spans");
    std::unordered_map<uint64_t, std::pair<uint64_t,uint64_t>> blockServices;
    {
        std::unique_ptr<rocksdb::Iterator> it(db->NewIterator(options, spansCf));
        for (it->SeekToFirst(); it->Valid(); it->Next()) {
            auto spanK = ExternalValue<SpanKey>::FromSlice(it->key());
            if (spanK().fileId().type() != InodeType::FILE) {
                continue;
            }

            auto spanV = ExternalValue<SpanBody>::FromSlice(it->value());
            if (spanV().isInlineStorage() == INLINE_STORAGE) {
                continue;
            }
            for (uint8_t i = 0; i < spanV().locationCount(); ++i) {
                auto blocksBody = spanV().blocksBodyReadOnly(i);
                uint8_t failureToleranceCount = blocksBody.parity().parityBlocks();
                auto blockSizePhysical = blocksBody.stripes() * blocksBody.cellSize();
                for (uint8_t i = 0; i < blocksBody.parity().blocks(); ++i) {
                    auto blockServiceId = blocksBody.block(i).blockService();
                    auto& data = blockServices[blockServiceId.u64];
                    ++data.first;
                    data.second += blockSizePhysical;
                }
            }
        }
        ROCKS_DB_CHECKED(it->status());
    }
    outputStream << "[" << std::endl;
    for (auto it = blockServices.begin(); it != blockServices.end();) {
        outputStream << "{\"blockServiceId\": " << it->first << ", \"blockCount\": " << it->second.first << ", \"totalSize\": " << it->second.second << "}";
            if (++it != blockServices.end())
                outputStream << ",";
            outputStream << std::endl;
    }
    outputStream << "]" << std::endl;
    outputStream.close();
    ALWAYS_ASSERT(!outputStream.bad(), "An error occurred while closing the sample output file");
}
