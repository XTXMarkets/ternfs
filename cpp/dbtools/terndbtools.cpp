// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <cstdint>
#include <cstdio>
#include <string>

#include "ShardDB.hpp"
#include "ShardDBTools.hpp"
#include "CDCDBTools.hpp"

#define die(...) do { fprintf(stderr, __VA_ARGS__); exit(1); } while(false)

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
        } else {
            dieWithUsage();
        }
    }

    return 0;
}
