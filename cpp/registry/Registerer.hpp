// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <atomic>

#include "Msgs.hpp"
#include "PeriodicLoop.hpp"

#include "RegistryCommon.hpp"


class Registerer : public PeriodicLoop {
public:
    Registerer(Logger &logger, std::shared_ptr<XmonAgent> &xmon,
                const RegistryOptions &options, const AddrsInfo &boundAddresses,
                const std::vector<FullRegistryInfo>& cachedReplicas)
        : PeriodicLoop(logger, xmon, "registerer", {1_sec, 1, 2_mins, 1}),
        _logsDBOptions(options.logsDBOptions),
        _clientOptions(options.registryClientOptions),
        _boundAddresses(boundAddresses),
        _hasEnoughReplicas(false),
        _replicas(std::make_shared<std::array<AddrsInfo, LogsDB::REPLICA_COUNT>>()) {
            _init(cachedReplicas);
    }

    virtual ~Registerer() = default;

    inline bool hasEnoughReplicas() const {
        return _hasEnoughReplicas.load(std::memory_order_relaxed);
    }

    inline std::shared_ptr<std::array<AddrsInfo, LogsDB::REPLICA_COUNT>> replicas() {
        return _replicas;
    }

    virtual bool periodicStep() override;

private:
    const LogsDBOptions _logsDBOptions;
    const RegistryClientOptions _clientOptions;
    const AddrsInfo _boundAddresses;
    XmonNCAlert _alert;

    std::atomic<bool> _hasEnoughReplicas;
    std::shared_ptr<std::array<AddrsInfo, LogsDB::REPLICA_COUNT>> _replicas;

    void _init(const std::vector<FullRegistryInfo> &cachedReplicas);
    bool _updateReplicas(const std::vector<FullRegistryInfo> &allReplicas);
};
