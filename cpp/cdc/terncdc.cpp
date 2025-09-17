// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <stdio.h>
#include <filesystem>

#include "CDC.hpp"
#include "CommonOptions.hpp"

static bool parseCDCOptions(CommandLineArgs& args, CDCOptions& options) {
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
        if (arg == "-shard-timeout") {
            options.shardTimeout = parseDuration(args.next());
            continue;
        }
        fprintf(stderr, "unknown argument %s\n", args.peekArg().c_str());
        return false;
    }
    return true;
}

static void printCDCOptionsUsage() {
    printLogOptionsUsage();
    printXmonOptionsUsage();
    printMetricsOptionsUsage();
    printRegistryClientOptionsUsage();
    printLogsDBOptionsUsage();
    printServerOptionsUsage();
    fprintf(stderr, "CDCOptions:\n");
    fprintf(stderr, " -shard-timeout\n");
    fprintf(stderr, "    	How much to wait for shard responses. Right now this is a simple loop.\n");
}

static bool validateCDCOptions(const CDCOptions& options) {
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
    printCDCOptionsUsage();
}

int main(int argc, char** argv) {
    namespace fs = std::filesystem;

    CDCOptions options;
    CommandLineArgs args(argc, argv, usage);
    if (!(parseCDCOptions(args, options) && validateCDCOptions(options))) {
        args.dieWithUsage();
    }

    fs::path dbDir(options.logsDBOptions.dbDir);
    {
        std::error_code err;
        if (!fs::create_directory(dbDir, err) && err.value() != 0) {
            throw EXPLICIT_SYSCALL_EXCEPTION(err.value(), "mkdir");
        }
    }

    options.cdcToShardAddress = options.serverOptions.addrs;
    options.cdcToShardAddress[0].port = 0;
    options.cdcToShardAddress[1].port = 0;

    runCDC(options);

    return 0;
}
