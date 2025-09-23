// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "Registerer.hpp"
#include "LogsDB.hpp"
#include <atomic>

static std::ostream &operator<<(std::ostream &o, const std::vector<FullRegistryInfo> &replicas) {
  o << "[";
  for (const auto &rep : replicas) {
    o << rep << ',';
  }
  return o << "]";
}


void Registerer::_init(const std::vector<FullRegistryInfo> &cachedReplicas) {
    _env.updateAlert(_alert, "Waiting to register ourselves for the first time");
    _updateReplicas(cachedReplicas);
}

bool Registerer::periodicStep() {
    {
        LOG_DEBUG(
            _env,
            "Registering ourselves (replicaId %s, location %s, %s) with registry",
            _logsDBOptions.replicaId, (int)_logsDBOptions.location,
            _boundAddresses);

        const auto [err, errStr] =
            registerRegistry(_clientOptions.host, _clientOptions.port, 10_sec,
                            _logsDBOptions.replicaId, _logsDBOptions.location,
                            !_logsDBOptions.avoidBeingLeader, _boundAddresses, 
                            !_hasEnoughReplicas.load(std::memory_order_relaxed));
        if (err == EINTR) {
            return false;
        }
        if (err) {
            _env.updateAlert(_alert, "Couldn't register ourselves with registry: %s", errStr);
            return false;
        }
    }

    {
        std::vector<FullRegistryInfo> allReplicas;
        LOG_DEBUG(_env, "Fetching replicas from registry");
        const auto [err, errStr] = fetchRegistryReplicas(
            _clientOptions.host, _clientOptions.port, 10_sec, allReplicas);
        if (err == EINTR) {
            return false;
        }
        if (err) {
            _env.updateAlert(
                _alert, "Failed getting registry replicas from registry: %s", errStr);
            return false;
        }
        LOG_DEBUG(_env, "fetched all replicas %s", allReplicas);
        if (!_updateReplicas(allReplicas)) {
            return false;
        }
    }
    _env.clearAlert(_alert);
    return true;
}

bool Registerer::_updateReplicas(const std::vector<FullRegistryInfo> &allReplicas) {
    std::array<AddrsInfo, LogsDB::REPLICA_COUNT> localReplicas;
    bool foundLeader = false;
    for (auto &replica : allReplicas) {
        if (replica.locationId != _logsDBOptions.location) {            
            continue;
        }
        localReplicas[replica.id.u8] = replica.addrs;
        if (replica.isLeader) {
            foundLeader = true;
        }
    }
    if (!foundLeader) {
        _env.updateAlert(_alert, "Didn't get leader with known addresses from registry");
        return false;

    }
    if (_boundAddresses != localReplicas[_logsDBOptions.replicaId]) {
        _env.updateAlert(_alert, "AddrsInfo in registry: %s , not matching local AddrsInfo: %s",
            localReplicas[_logsDBOptions.replicaId], _boundAddresses);
        return false;
    }

    size_t knownReplicas{0};
    for (auto &replica : localReplicas) {
        if (replica.addrs[0].port != 0 || replica.addrs[1].port) {
            ++knownReplicas;
        }
    }
    size_t quorumSize = _logsDBOptions.noReplication ? 1 : LogsDB::REPLICA_COUNT / 2 + 1;
    if (knownReplicas < quorumSize ) {
        _env.updateAlert(_alert, "Didn't get enough replicas with known addresses from registry");
        return false;
    }

    if (unlikely(*_replicas != localReplicas)) {
        LOG_DEBUG(_env, "Updating replicas to %s %s %s %s %s", localReplicas[0],
                localReplicas[1], localReplicas[2], localReplicas[3],
                localReplicas[4]);
        std::atomic_exchange(&_replicas, std::make_shared<std::array<AddrsInfo, LogsDB::REPLICA_COUNT>>(localReplicas));
    }
    return _hasEnoughReplicas.exchange(true, std::memory_order_release);
}
