// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <cstdint>

#include "CommonOptions.hpp"
#include "Time.hpp"

struct RegistryOptions {
    LogOptions logOptions;
    XmonOptions xmonOptions;
    MetricsOptions metricsOptions;
    RegistryClientOptions registryClientOptions;
    LogsDBOptions logsDBOptions;
    ServerOptions serverOptions;
  
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
