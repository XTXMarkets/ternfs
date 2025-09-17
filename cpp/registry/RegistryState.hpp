// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <cstdint>
#include <string>
#include <memory>
#include <optional>

#include "Env.hpp"
#include "Msgs.hpp"
#include "Time.hpp"
#include "XmonAgent.hpp"
#include "Metrics.hpp"

struct RegistryReplicaInfo {
  ReplicaId replicaId;
  uint8_t location;
  std::string host;
  uint16_t port;
};

struct RegistryOptions {
  // Logging
  LogLevel logLevel = LogLevel::LOG_INFO;
  std::string logFile = "";
  bool syslog = false;
  std::string xmonAddr;
  std::optional<InfluxDB> influxDB;

  // LogsDB settings
  std::string dbDir;
  bool avoidBeingLeader = true;
  bool noReplication = false;

  // Registry replication and location
  std::string registryPrimaryHost = "";
  uint16_t registryPort = 0;
  // The second will be used if the port is non-null
  AddrsInfo registryAddrs;
  ReplicaId replicaId;
  uint8_t location;

  // Registry specific settings
  bool enforceStableIp = false;
  bool enforceStableLeader = false;
  uint32_t maxConnections = 4000;
  Duration staleDelay = 3_mins;
  Duration blockServiceUsageDelay = 0_mins;
  Duration minDecomInterval = 1_hours;
  uint8_t alertAfterUnavailableFailureDomains = 3;
  uint32_t maxFailureDomainsPerShard = 28;
  Duration writableBlockServiceUpdateInterval = 30_mins;
};

struct RegistryState;
class Registry {
public:
    explicit Registry(const RegistryOptions& options, Logger& logger, std::shared_ptr<XmonAgent> xmon);
    ~Registry();
    void start();
    void waitUntilStopped();
private:
    const RegistryOptions& _options;
    Logger& _logger;
    std::shared_ptr<XmonAgent> _xmon;
    Env _env;
    std::unique_ptr<RegistryState> _state;
};
