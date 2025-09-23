// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <cstdio>
#include <fcntl.h>
#include <filesystem>
#include <string>

#include "Loop.hpp"
#include "Registry.hpp"
#include "Xmon.hpp"

static bool parseRegistryOptions(CommandLineArgs& args, RegistryOptions& options) {
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
        if (arg == "-enforce-stable-ip") {
            args.next();
            options.enforceStableIp = true;
            continue;
        }
        if (arg == "-enforce-stable-leader") {
            args.next();
            options.enforceStableLeader = true;
            continue;
        }
        if (arg == "-max-connections") {
            options.maxConnections = parseUint32(args.next());
            continue;
        }
        if (arg == "-min-auto-decom-interval") {
            options.minDecomInterval = parseDuration(args.next());
            continue;
        }
        if (arg == "-alert-at-unavailable-failure-domains") {
            options.alertAfterUnavailableFailureDomains = parseUint8(args.next());
            continue;
        }
        if (arg == "-staleness-delay") {
            options.staleDelay = parseDuration(args.next());
            continue;
        }
        if (arg == "-block-service-use-delay") {
            options.blockServiceUsageDelay = parseDuration(args.next());
            continue;
        }
        if (arg == "-max-writable-block-service-per-shard") {
            options.maxFailureDomainsPerShard = parseUint32(args.next());
            continue;
        }
        if (arg == "-writable-block-service-update-interval") {
            options.writableBlockServiceUpdateInterval = parseDuration(args.next());
            continue;
        }
        if (arg == "-using-dynamic-ports") {
            args.next();
            options.usingDynamicPorts = true;
            continue;
        }
        fprintf(stderr, "unknown argument %s\n", args.peekArg().c_str());
        return false;
    }
    return true;
}

static void printRegistryOptionsUsage() {
    printLogOptionsUsage();
    printXmonOptionsUsage();
    printMetricsOptionsUsage();
    printRegistryClientOptionsUsage();
    printLogsDBOptionsUsage();
    printServerOptionsUsage();
    fprintf(stderr, "RegistryOptions:\n");
    fprintf(stderr, " -enforce-stable-ip\n");
    fprintf(stderr, "       Don't allow both ip addresses to change at once on any service hearbeat.\n");
    fprintf(stderr, " -enforce-stable-leader\n");
    fprintf(stderr, "       Don't allow leader to implicitly change on heartbeat but require LeaderMoveReq.\n");
    fprintf(stderr, " -max-connections\n");
    fprintf(stderr, "       Maximum number of connections to serve at the same time. Default is 4000\n");
    fprintf(stderr, " -min-auto-decom-interval\n");
    fprintf(stderr, "       Minimum time between auto-decomissions for same path-prefix. Default 1 hour\n");
    fprintf(stderr, " -alert-at-unavailable-failure-domains\n");
    fprintf(stderr, "       Alert and prevent auto-decomission after errors spread number of failure domains. Default 3\n");
    fprintf(stderr, " -staleness-delay\n");
    fprintf(stderr, "       How long it needs to pass without heartbeat before a service is declared stale. Default is 3 min\n");
    fprintf(stderr, " -block-service-use-delay\n");
    fprintf(stderr, "       How long to wait after seeing block services for the first time before using it. Default is 0\n");
    fprintf(stderr, " -max-writable-block-service-per-shard\n");
    fprintf(stderr, "       Maximum number of block services to assign to a shard for writting at any given time. Default is 28\n");
    fprintf(stderr, " -writable-block-service-update-interval\n");
    fprintf(stderr, "       Maximum interval at which to change writable services assigned to shards. Default 30 min\n");
    fprintf(stderr, " -using-dynamic-ports\n");
    fprintf(stderr, "       Inform registry services are using dynamic ports. Registry will wipe port information on startup to speed up bootstrap\n");
}

static bool validateRegistryOptions(const RegistryOptions& options) {
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
    printRegistryOptionsUsage();
}

int main(int argc, char** argv) {
    namespace fs = std::filesystem;

    RegistryOptions options;
    CommandLineArgs args(argc, argv, usage);
    if (!(parseRegistryOptions(args, options) && validateRegistryOptions(options))) {
        args.dieWithUsage();
    }

    fs::path dbDir(options.logsDBOptions.dbDir);
    {
        std::error_code err;
        if (!fs::create_directory(dbDir, err) && err.value() != 0) {
            throw EXPLICIT_SYSCALL_EXCEPTION(err.value(), "mkdir");
        }
    }
    
    int logOutFd = STDOUT_FILENO;
    if (!options.logOptions.logFile.empty()) {
        logOutFd = open(options.logOptions.logFile.c_str(), O_WRONLY|O_CREAT|O_APPEND, 0644);
        if (logOutFd < 0) {
            throw SYSCALL_EXCEPTION("open");
        }
    }
    Logger logger(options.logOptions.logLevel, logOutFd, options.logOptions.syslog, true);

    LoopThreads threads;
    // Immediately start xmon: we want the init alerts to be there
    std::shared_ptr<XmonAgent> xmon;
    if (!options.xmonOptions.addr.empty()) {
        xmon = std::make_shared<XmonAgent>();
        std::ostringstream ss;
        ss << "registry_" << options.logsDBOptions.replicaId;
        options.xmonOptions.appInstance = ss.str();
        options.xmonOptions.appType = XmonAppType::CRITICAL;
        threads.emplace_back(LoopThread::Spawn(std::make_unique<Xmon>(logger, xmon, options.xmonOptions)));
    }

    Env env(logger, xmon, "startup");

    Registry regi(logger, xmon);
    LOG_INFO(env, "starting Registry");
    regi.start(options, threads);
    LOG_INFO(env, "waiting for Registry to stop");
    // from this point on termination on SIGINT/SIGTERM will be graceful
    LoopThread::waitUntilStopped(threads);
    regi.close();
    LOG_INFO(env, "registry stopped. Exiting");

    return 0;
}
