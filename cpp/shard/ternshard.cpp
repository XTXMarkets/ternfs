// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <pthread.h>
#include <stdio.h>
#include <filesystem>
#include <string>

#include "CommonOptions.hpp"
#include "Exception.hpp"
#include "Shard.hpp"

static bool parseShardOptions(CommandLineArgs& args, ShardOptions& options) {
    while(!args.done()) {
        if (parseLogOptions(args, options.logOptions) || 
            parseXmonOptions(args, options.xmonOptions) ||
            parseMetricsOptions(args, options.metricsOptions) ||
            parseRegistryClientOptions(args, options.registryClientOptions) ||
            parseLogsDBOptions(args, options.logsDBOptions) ||
            parseServerOptions(args, options.serverOptions)
        ) {
            continue;
        }
        std::string arg = args.peekArg();
        if (arg == "-transient-deadline-interval") {
            options.transientDeadlineInterval = parseDuration(args.next());
            continue;
        }
        if (arg == "-shard") {
            options.shardId = parseUint8(args.next());
            options.shardIdSet = true;
            continue;
        }
        fprintf(stderr, "unknown argument %s\n", args.peekArg().c_str());
        return false;
    }
    return true;
}

static void printShardOptionsUsage() {
    printLogOptionsUsage();
    printXmonOptionsUsage();
    printMetricsOptionsUsage();
    printRegistryClientOptionsUsage();
    printLogsDBOptionsUsage();
    printServerOptionsUsage();
    fprintf(stderr, "ShardOptions:\n");
    fprintf(stderr, " -shard\n");
    fprintf(stderr, "    	Which shard we are running as [0-255]\n");
    fprintf(stderr, " -transient-deadline-interval\n");
    fprintf(stderr, "    	Tweaks the interval with which the deadline for transient file gets bumped.\n");
}

static bool validateShardOptions(const ShardOptions& options) { 
    if (!options.shardIdSet) {
        fprintf(stderr, "-shard needs to be set\n");
        return false;
    }
    return (validateLogOptions(options.logOptions) && 
            validateXmonOptions(options.xmonOptions) &&
            validateMetricsOptions(options.metricsOptions) &&
            validateRegistryClientOptions(options.registryClientOptions) &&
            validateLogsDBOptions(options.logsDBOptions) &&
            validateServerOptions(options.serverOptions)
    );
}

static void usage(const char* binary) {
    fprintf(stderr, "Usage: %s \n\n", binary);
    printShardOptionsUsage();
}

int main(int argc, char** argv) {
    namespace fs = std::filesystem;
    ShardOptions options;
    CommandLineArgs args(argc, argv, usage);

    if (!(parseShardOptions(args, options) && validateShardOptions(options))) {
        args.dieWithUsage();
    }

    fs::path dbDir(options.logsDBOptions.dbDir);
    {
        std::error_code err;
        if (!fs::create_directory(dbDir, err) && err.value() != 0) {
            throw EXPLICIT_SYSCALL_EXCEPTION(err.value(), "mkdir");
        }
    }

    runShard(options);

    return 0;
}
