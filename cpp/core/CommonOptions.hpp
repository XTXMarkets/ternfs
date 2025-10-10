// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include "Env.hpp"
#include "LogsDB.hpp"
#include "Metrics.hpp"
#include "Msgs.hpp"
#include "RegistryClient.hpp"
#include "Xmon.hpp"
#include <arpa/inet.h>
#include <cstdio>
#include <netinet/in.h>
#include <sys/socket.h>

class CommandLineArgs {
public:
    CommandLineArgs(int argc, char** argv, void (*printUsage)(const char* binaryName)) :
      _currentArg(1), _argc(argc), _argv(argv), _printUsage(printUsage) {}

    std::string peekArg() {
        if (_currentArg == _argc) {
            fprintf(stderr, "Argument list ended too early.\n\n");
            dieWithUsage();
        }
        return _argv[_currentArg];
    }

    CommandLineArgs& next() {
        if (_currentArg == _argc) {
            fprintf(stderr, "Argument list ended too early.\n\n");
            dieWithUsage();
        }
        ++_currentArg;
        return *this;
    }

    std::string getArg() {
        auto res = peekArg();
        next();
        return res;
    }

    void dieWithUsage() {
        _printUsage(_argv[0]);
        exit(2);
    }

    bool done() const {
        return _currentArg == _argc;
    }
private:
    int _currentArg;
    int _argc;
    char **_argv;
    void (*_printUsage)(const char*);
};

// LogOptions
struct LogOptions {
    LogLevel logLevel = LogLevel::LOG_INFO;
    std::string logFile = "";
    bool syslog = false;
};

static inline bool parseLogOptions(CommandLineArgs& args, LogOptions& options) {
    std::string arg = args.peekArg();
    if (arg == "-verbose") {
        options.logLevel = std::min<LogLevel>(LogLevel::LOG_DEBUG, options.logLevel);
        args.next();
        return true;
    }
    if (arg == "-log-level") {
        std::string logLevel = args.next().getArg();
        if (logLevel == "trace") {
            options.logLevel = LogLevel::LOG_TRACE;
        } else if (logLevel == "debug") {
            options.logLevel = LogLevel::LOG_DEBUG;
        } else if (logLevel == "info") {
            options.logLevel = LogLevel::LOG_INFO;
        } else if (logLevel == "error") {
            options.logLevel = LogLevel::LOG_ERROR;
        } else {
            fprintf(stderr, "Bad log level `%s'\n", logLevel.c_str());
            args.dieWithUsage();
        }
        return true;
    }
    if (arg == "-log-file") {
        options.logFile = args.next().getArg();
        return true;
    }
    return false;
}

static inline void printLogOptionsUsage() {
    fprintf(stderr, "LogOptions:\n");
    fprintf(stderr, " -log-file string\n");
    fprintf(stderr, "    	If not provided, stdout.\n");
    fprintf(stderr, " -log-level trace|debug|info|error\n");
    fprintf(stderr, "    	Note that 'trace' will only work for debug builds.\n");
    fprintf(stderr, " -verbose\n");
    fprintf(stderr, "    	Same as '-log-level debug'.\n");
}

static inline bool validateLogOptions(const LogOptions& options) {
#ifndef TERN_DEBUG
    if (options.logLevel <= LogLevel::LOG_TRACE) {
        fprintf(stderr,"Cannot use trace for non-debug builds (it won't work).\n");
        return false;
    }
#endif
    return true;
}

// XmonOptions

typedef XmonConfig XmonOptions;

static inline bool parseXmonOptions(CommandLineArgs& args, XmonOptions& options) {
    std::string arg = args.peekArg();
    if (arg == "-xmon") {
        options.addr = args.next().getArg();
        return true;
    }
    return false;
}

static inline void printXmonOptionsUsage() {
    fprintf(stderr, "XmonOptions:\n");
    fprintf(stderr, " -xmon host:port\n");
    fprintf(stderr, "    	Enable Xmon alerts.\n");
}

static inline bool validateXmonOptions(const XmonOptions&) { return true; }

// MetricsOptions

typedef InfluxDB MetricsOptions;

static inline bool parseMetricsOptions(CommandLineArgs& args, MetricsOptions& options){
    std::string arg = args.peekArg();
    if (arg == "-influx-db-origin") {
        options.origin = args.next().getArg();
        return true;
    }
    if (arg == "-influx-db-org") {
        options.org = args.next().getArg();
        return true;
    }
    if (arg == "-influx-db-bucket") {
        options.bucket = args.next().getArg();
        return true;
    }
    return false;
}

static inline void printMetricsOptionsUsage() {
    fprintf(stderr, "MetricsOptions:\n");
    fprintf(stderr, " -influx-db-origin\n");
    fprintf(stderr, " -influx-db-org\n");
    fprintf(stderr, " -influx-db-bucket\n");
    fprintf(stderr, "    	Enable metrics.\n");
}

static inline bool validateMetricsOptions(const MetricsOptions& options) {
    if (options.origin.empty() != options.org.empty() ||
        options.origin.empty() != options.bucket.empty())
    {
        fprintf(stderr, "either all or none of the -influx-db flags must be provided\n");
        return false;
    }
    return true;
}

// RegistryClientOptions

struct RegistryClientOptions {
    std::string host;
    uint16_t port;
};

static inline bool parseRegistryClientOptions(CommandLineArgs& args, RegistryClientOptions& options) {
    std::string arg = args.peekArg();
    if (arg == "-registry") {
        if (!parseRegistryAddress(args.next().peekArg(), options.host, options.port)) {
            fprintf(stderr, "failed parsing registry address %s\n",args.peekArg().c_str());
            args.dieWithUsage();
        }
        args.next();
        return true;
    }
    return false;
}

static inline void printRegistryClientOptionsUsage() {
    fprintf(stderr, "RegistryClientOptions:\n");
    fprintf(stderr, " -registry host:port\n");
    fprintf(stderr, "    	How to reach registry\n");
}

static inline bool validateRegistryClientOptions(const RegistryClientOptions& options) {
    if (options.host.empty()) {
        fprintf(stderr, "-registry needs to be set\n");
        return false;
    }
    return true;
}

// LogsDB options

static inline uint8_t parseReplicaId(CommandLineArgs& args) {
    size_t idx;
    std::string arg = args.getArg();
    unsigned long replicaId = std::stoul(arg, &idx);
    if (idx != arg.size()) {
        fprintf(stderr,"Runoff character in number %s\n", arg.c_str());
    }
    if (replicaId > 4) {
        fprintf(stderr,"Bad replicaId %s\n", arg.c_str());
    }
    return replicaId;
}

static inline uint8_t parseUint8(CommandLineArgs& args) {
    size_t processed;
    auto arg = args.getArg();
    uint64_t x = std::stoull(arg, &processed);
    if (processed != arg.size() || x > std::numeric_limits<uint8_t>::max()) {
        fprintf(stderr,"Invalid argument '%s', expecting an unsigned integer less than 256\n", arg.c_str());
    }
    return static_cast<uint8_t>(x);
}

struct LogsDBOptions {
    std::string dbDir;
    bool avoidBeingLeader = true;
    bool noReplication = false;
    uint8_t replicaId = 5;
    uint8_t location = 0;
};

static inline bool parseLogsDBOptions(CommandLineArgs& args, LogsDBOptions& options) {
    std::string arg = args.peekArg();
    if (arg == "-db-dir") {
        options.dbDir = args.next().getArg();
        return true;
    }
    if (arg == "-replica") {
        options.replicaId = parseReplicaId(args.next());
        return true;
    }
    if (arg == "-logsdb-leader") {
        args.next();
        options.avoidBeingLeader = false;
        return true;
    }
    if (arg == "-logsdb-no-replication") {
        args.next();
        options.noReplication = true;
        return true;
    }
    if (arg == "-location") {
        options.location = parseUint8(args.next());
        return true;
    }
    return false;
}

static inline void printLogsDBOptionsUsage() {
    fprintf(stderr, "LogsDBOptions:\n");
    fprintf(stderr, " -db-dir\n");
    fprintf(stderr, "    	Base directory path, db will be created in db subdirectory\n");
    fprintf(stderr, " -logsdb-leader\n");
    fprintf(stderr, "    	Allow replica to become leader. Default is false\n");
    fprintf(stderr, " -logsdb-no-replication\n");
    fprintf(stderr, "    	Don't wait for acks from other replicas when becoming leader or replicating.\n");
    fprintf(stderr, "    	Can only be set if -logsdb-leader is also set. Default is false\n");
    fprintf(stderr, " -replica\n");
    fprintf(stderr, "    	Which replica are we running as [0-4]\n");
    fprintf(stderr, " -location\n");
    fprintf(stderr, "    	Which location we are running as [0-255]. Default is 0\n");
}

static inline bool validateLogsDBOptions(const LogsDBOptions& options) {
    if (options.dbDir.empty()) {
        fprintf(stderr, "-db-dir needs to be set\n");
        return false;
    }
    if (options.noReplication && options.avoidBeingLeader) {
        fprintf(stderr, "-logsdb-leader needs to be set if -logsdb-no-replication is set\n");
        return false;
    }
    if (options.replicaId > 4) {
        fprintf(stderr, "-replica needs to be set\n");
        return false;
    }
    return true;
}


// ServerOptions

static inline double parseDouble(CommandLineArgs& args) {
    size_t idx;
    std::string arg = args.getArg();
    double x = std::stod(arg, &idx);
    if (idx != arg.size()) {
        fprintf(stderr, "Runoff characters in number %s\n", arg.c_str());
        args.dieWithUsage();
    }
    return x;
}

static inline double parseProbability(CommandLineArgs& args) {
    double x = parseDouble(args);
    if (x < 0.0 || x >= 1.0) {
        fprintf(stderr,"Please specify a number in the interval [0.0, 1.0), rather than %f\n", x);
        args.dieWithUsage();
    }
    return x;
}

static inline bool parseIpv4(const std::string& arg, sockaddr_in& addrOut) {
    addrOut.sin_family = AF_INET;
    int res = inet_pton(AF_INET, arg.c_str(), &addrOut.sin_addr);
    if (res == 0) {
        fprintf(stderr, "Invalid ipv4 address '%s'\n\n", arg.c_str());
        return false;
    }
    return true;
}

static inline bool parsePort(const std::string& arg, uint16_t& portOut) {
    size_t idx;
    unsigned long port = std::stoul(arg, &idx);
    if (idx != arg.size()) {
        fprintf(stderr, "Runoff character in number %s\n", arg.c_str());
        return false;
    }
    if (port > 0xFFFFul) {
        fprintf(stderr, "Bad port %s\n", arg.c_str());
        return false;
    }
    portOut = port;
    return true;
}

static inline bool parseIpv4Addr(const std::string& arg, IpPort& addrOut) {
    size_t colon = arg.find(':');
    if (colon == std::string::npos) {
        fprintf(stderr, "Invalid ipv4 ip:port address '%s'\n\n", arg.c_str());
        return false;
    }
    sockaddr_in addr;
    if (!parseIpv4(arg.substr(0, colon), addr)) {
        return false;
    }
    uint16_t port;
    if (!parsePort(arg.substr(colon+1),port)) {
        return false;
    }
    addr.sin_port = htons(port);
    addrOut = IpPort::fromSockAddrIn(addr);
    return true;
}

struct ServerOptions {
    AddrsInfo addrs;
    int numAddressesFound = 0;
    // If non-zero, UDP packets will be dropped with this probability. Useful to test
    // resilience of the system.
    double simulateOutgoingPacketDrop = 0.0;
};

static inline bool parseServerOptions(CommandLineArgs& args, ServerOptions& options) {
    std::string arg = args.peekArg();
    if (arg == "-addr") {
        if (options.numAddressesFound == options.addrs.size()) {
                fprintf(stderr, "too many addresses defined\n");
                args.dieWithUsage();
            }
        if (!parseIpv4Addr(args.next().peekArg(), options.addrs[options.numAddressesFound++])) {
            fprintf(stderr, "failed parsing address %s\n",args.peekArg().c_str());
            args.dieWithUsage();
        }
        args.next();
        return true;
    }
    if (arg == "-outgoing-packet-drop") {
        options.simulateOutgoingPacketDrop = parseProbability(args.next());
        return true;
    }
    return false;
}

static inline void printServerOptionsUsage() {
    fprintf(stderr, "ServerOptions:\n");
    fprintf(stderr, " -addr ipv4 ip:port\n");
    fprintf(stderr, "    	Addresses we bind ourselves too and advertise to registry. At least one needs to be provided and at most 2\n");
    fprintf(stderr, " -outgoing-packet-drop [0, 1)\n");
    fprintf(stderr, "    	Drop given ratio of packets after processing them.\n");
}

static inline bool validateServerOptions(const ServerOptions& options) {
    if (options.numAddressesFound == 0) {
        fprintf(stderr, "at least one -addr needs to be defined\n");
        return false;
    }
    return true;
}

// Parsing Helpers

Duration parseDuration(CommandLineArgs& args);

static inline uint32_t parseUint32(CommandLineArgs& args) {
    size_t processed;
    auto arg = args.getArg();
    uint64_t x = std::stoull(arg, &processed);
    if (processed != arg.size() || x > std::numeric_limits<uint32_t>::max()) {
        fprintf(stderr, "Invalid argument '%s', expecting an unsigned integer\n", arg.c_str());
    }
    return static_cast<uint32_t>(x);
}

static inline uint32_t parseUint16(CommandLineArgs& args) {
    size_t processed;
    auto arg = args.getArg();
    uint64_t x = std::stoull(arg, &processed);
    if (processed != arg.size() || x > std::numeric_limits<uint16_t>::max()) {
        fprintf(stderr, "Invalid argument '%s', expecting an unsigned 16-bit integer\n", arg.c_str());
    }
    return static_cast<uint32_t>(x);
}
