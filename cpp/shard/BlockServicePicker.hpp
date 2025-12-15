// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <atomic>
#include <cstdint>
#include <memory>
#include <mutex>
#include <unordered_map>
#include <unordered_set>
#include <vector>


#include "MsgsGen.hpp"
#include "Random.hpp"
#include "Time.hpp"

struct BlockServiceCache;

struct BlockServicePicker {
    struct BlockServiceInfo {
        BlockServiceId id;
        uint64_t availableBytes;
    };

    struct FailureDomainInfo {
        FailureDomain failureDomain;
        std::vector<BlockServiceInfo> services;  // writable services in this FD
        uint64_t totalWeight;
    };

    struct LocationStorageInfo {
        std::vector<FailureDomainInfo> failureDomains;
        uint64_t totalWeight;
    };

    struct State {
        // key: (location << 8) | storageClass
        std::unordered_map<uint16_t, LocationStorageInfo> byLocClass;
        struct ServiceLookup {
            uint16_t lcKey;
            size_t fdIndex;
            uint64_t weight;
        };
        std::unordered_map<uint64_t, ServiceLookup> serviceToFdInfo;
        // Fallback: current block services (original simple list)
        std::unordered_map<uint16_t, std::vector<BlockServiceInfoShort>> currentByLocClass;
        // Set of (location, storageClass) keys that should use fallback storage class
        // Populated when there are no block services (writable or unwritable) for that combination
        std::unordered_set<uint16_t> needsFallback;
    };

    struct LocationStorageStats {
        std::atomic<uint64_t> totalPicks{0};
        std::atomic<uint64_t> fallbackPicks{0};
        std::atomic<uint64_t> writableFailureDomains{0};
        std::atomic<uint64_t> writableBlockServices{0};
        std::atomic<uint64_t> maxWeight{0};
        std::atomic<uint64_t> minWeight{0};
    };

    std::atomic<std::shared_ptr<const State>> _state{nullptr};
    mutable RandomGenerator _rng;

    // mutable as pick is const
    mutable std::unordered_map<uint16_t, LocationStorageStats> _locStorageStats;
    mutable std::unordered_map<uint64_t, std::atomic<uint64_t>> _blockServiceStats;
    mutable std::unordered_map<std::string, std::atomic<uint64_t>> _failureDomainStats;
    mutable std::mutex _statsMutex;  // protects map structures (not atomic values)
    const uint8_t _maxBlocksToPick;
    const Duration _writableDelay;

    explicit BlockServicePicker(uint8_t maxBlocksToPick = 15, Duration writableDelay = 5_mins);

    void update(
        const std::unordered_map<uint64_t, BlockServiceCache>& allBlockServices,
        const std::vector<BlockServiceInfoShort>& currentBlockServices
    );

    // Pick "needed" services for (locationId, storageClass), honoring blacklist.
    // Returns TernError::COULD_NOT_PICK_BLOCK_SERVICES if not enough candidates.
    TernError pick(
        uint8_t locationId,
        uint8_t storageClass,
        uint8_t needed,
        const std::vector<BlacklistEntry>& blacklist,
        std::vector<BlockServiceId>& out
    ) const;

    struct StatsSnapshot {
        struct LocStorageSnap {
            uint16_t key;
            uint64_t totalPicks;
            uint64_t fallbackPicks;
            uint64_t writableFailureDomains;
            uint64_t writableBlockServices;
            uint64_t maxWeight;
            uint64_t minWeight;
        };
        struct BlockServiceSnap {
            uint64_t blockServiceId;
            uint64_t picks;
        };
        struct FailureDomainSnap {
            std::string failureDomain;
            uint64_t picks;
        };
        std::vector<LocStorageSnap> locStorage;
        std::vector<BlockServiceSnap> blockServices;
        std::vector<FailureDomainSnap> failureDomains;
        uint64_t minServicePicks;
        uint64_t maxServicePicks;
        uint64_t minFdPicks;
        uint64_t maxFdPicks;
    };

    StatsSnapshot getStats() const;
    void resetStats();
};
