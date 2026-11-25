// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <vector>

#include "Env.hpp"
#include "LogsDB.hpp"
#include "Metrics.hpp"
#include "MsgsGen.hpp"
#include "PeriodicLoop.hpp"
#include "RegistryCommon.hpp"
#include "RegistryDB.hpp"


void decommissionMetrics(MetricsBuilder& metrics, const std::vector<FullBlockServiceInfo>& blockServices, uint8_t unreadableLimit, TernTime now);

struct RegistryMetricsInserter : PeriodicLoop {
private:
    const RegistryOptions& _options;
    const RegistryDB& _registryDB;
    const LogsDB& _logsDB;
    XmonNCAlert _sendMetricsAlert;
    MetricsBuilder _metricsBuilder;
    std::vector<FullBlockServiceInfo> _blockServices;
public:
    RegistryMetricsInserter(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const RegistryOptions& options, const RegistryDB& registryDB, const LogsDB& logsDB);
    virtual ~RegistryMetricsInserter() = default;

    virtual bool periodicStep() override;
};
