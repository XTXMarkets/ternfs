// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "BlockServicePicker.hpp"

#include "Bincode.hpp"
#include "MsgsGen.hpp"
#include "Time.hpp"
#include "Msgs.hpp"
#include "BlockServicesCacheDB.hpp"
#include <algorithm>
#include <cmath>
#include <unordered_set>

namespace {
    inline uint16_t lcKey(uint8_t locationId, uint8_t storageClass) {
        return (uint16_t(locationId) << 8) | uint16_t(storageClass);
    }

    inline bool blockServiceIsWritable(const BlockServiceCache& bs, Duration writableDelay) {
        return bs.availableBytes > 0 && blockServiceFlagsWritable(bs.flags) && ternNow() - bs.firstSeen > writableDelay;
    }

    // Apply power scaling to ensure max FD weight <= 1/N of total
    void applyScaling(
        BlockServicePicker::LocationStorageInfo& lsInfo,
        std::unordered_map<uint64_t, BlockServicePicker::State::ServiceLookup>& serviceToFdInfo,
        uint8_t maxBlocksToPick
    ) {
        auto& failureDomains = lsInfo.failureDomains;
        if (failureDomains.empty()) return;

        uint64_t maxFdWeight = 0;
        for (const auto& fd : failureDomains) {
            maxFdWeight = std::max(maxFdWeight, fd.totalWeight);
        }

        if (maxFdWeight * maxBlocksToPick <= lsInfo.totalWeight) {
            return;
        }

        double scaleFactor = 1.0;
        double minScale = 0.0;
        double maxScale = 1.0;
        double bestScaleFactor = 0.0;

        for (int iter = 0; iter < 50; iter++) {
            scaleFactor = (minScale + maxScale) / 2.0;

            uint64_t scaledTotalWeight = 0;
            uint64_t scaledMaxWeight = 0;
            for (const auto& fd : failureDomains) {
                uint64_t scaled = static_cast<uint64_t>(std::pow(static_cast<double>(fd.totalWeight), scaleFactor));
                if (fd.totalWeight > 0 && scaled == 0) {
                    scaled = 1;
                }
                scaledTotalWeight += scaled;
                scaledMaxWeight = std::max(scaledMaxWeight, scaled);
            }

            if (scaledMaxWeight * maxBlocksToPick <= scaledTotalWeight) {
                bestScaleFactor = scaleFactor;
                minScale = scaleFactor;
            } else {
                maxScale = scaleFactor;
            }
        }

        // Apply final scale factor to all failure domains and distribute to services
        lsInfo.totalWeight = 0;
        for (auto& fd : failureDomains) {
            if (fd.totalWeight == 0) continue;
            uint64_t targetFdWeight = static_cast<uint64_t>(std::pow(static_cast<double>(fd.totalWeight), bestScaleFactor));
            double ratio = static_cast<double>(targetFdWeight) / fd.totalWeight;
            fd.totalWeight = 0;
            for (auto& svc : fd.services) {
                if (svc.availableBytes == 0) continue;
                svc.availableBytes = std::max(static_cast<uint64_t>(svc.availableBytes * ratio), 1ul);
                serviceToFdInfo[svc.id.u64].weight = svc.availableBytes;
                fd.totalWeight += svc.availableBytes;
            }
            lsInfo.totalWeight += fd.totalWeight;
        }

        // Fix up rounding errors: ensure maxFdWeight * maxBlocksToPick <= totalWeight
        for (;;) {
            BlockServicePicker::FailureDomainInfo* maxFd = nullptr;
            uint64_t maxW = 0;
            for (auto& fd : failureDomains) {
                if (fd.totalWeight > maxW) {
                    maxW = fd.totalWeight;
                    maxFd = &fd;
                }
            }
            if (!maxFd || maxW * maxBlocksToPick <= lsInfo.totalWeight) break;

            // Find the largest service in the max FD and reduce it by 1
            BlockServicePicker::BlockServiceInfo* maxSvc = nullptr;
            uint64_t maxSvcW = 0;
            for (auto& svc : maxFd->services) {
                if (svc.availableBytes > maxSvcW) {
                    maxSvcW = svc.availableBytes;
                    maxSvc = &svc;
                }
            }
            if (!maxSvc || maxSvcW <= 1) break;
            maxSvc->availableBytes--;
            serviceToFdInfo[maxSvc->id.u64].weight--;
            maxFd->totalWeight--;
            lsInfo.totalWeight--;
        }
    }
}

BlockServicePicker::BlockServicePicker(Logger& logger, std::shared_ptr<XmonAgent>& xmon, uint8_t maxBlocksToPick, Duration writableDelay)
    : _state(nullptr), _rng(ternNow().ns), _env(logger, xmon, "block_service_picker"), _maxBlocksToPick(maxBlocksToPick), _writableDelay(writableDelay) {}

void BlockServicePicker::update(
    const std::unordered_map<uint64_t, BlockServiceCache>& allBlockServices
) {
    auto next = std::make_shared<State>();
    next->byLocClass.clear();
    next->serviceToFdInfo.clear();
    next->needsFallback.clear();

    // Build weighted structure: group by location/storageClass, then by failure domain
    // Map: (location, storageClass) -> map of (failure domain string) -> failure domain index
    std::unordered_map<uint16_t, std::unordered_map<std::string, size_t>> grouped;
    std::unordered_set<uint16_t> distinctBlockServiceTypeLoc;

    for (const auto& [id, bs] : allBlockServices) {
        distinctBlockServiceTypeLoc.insert(lcKey(bs.locationId, bs.storageClass));
        if (!blockServiceIsWritable(bs, _writableDelay)) continue;

        uint16_t key = lcKey(bs.locationId, bs.storageClass);
        std::string fdStr(reinterpret_cast<const char*>(bs.failureDomain.data()), bs.failureDomain.size());

        auto& lsInfo = next->byLocClass[key];

        auto& fdMap = grouped[key];
        auto [it, inserted] = fdMap.try_emplace(fdStr, lsInfo.failureDomains.size());
        if (inserted) {
            lsInfo.failureDomains.emplace_back(
                FailureDomainInfo{
                    FailureDomain{BincodeFixedBytes<16>{fdStr.data(), fdStr.size()}},
                    {},
                    0
                });
        }

        FailureDomainInfo& fdInfo = lsInfo.failureDomains[it->second];
        fdInfo.services.emplace_back(BlockServiceInfo{BlockServiceId(id), bs.availableBytes});
        fdInfo.totalWeight += bs.availableBytes;
        lsInfo.totalWeight += bs.availableBytes;

        next->serviceToFdInfo[id] = {key, it->second, bs.availableBytes};
    }

    // Apply weight scaling to each location/storageClass to avoid one failure domain being more than 1/N of total
    for (auto& [k, lsInfo] : next->byLocClass) {
        applyScaling(lsInfo, next->serviceToFdInfo, _maxBlocksToPick);
    }

    for (auto key : distinctBlockServiceTypeLoc) {
        auto otherKey = (key & 0xFF) == HDD_STORAGE
            ? (key & 0xFF00) | FLASH_STORAGE
            : (key & 0xFF00) | HDD_STORAGE;
        if (!distinctBlockServiceTypeLoc.contains(otherKey)) {
            next->needsFallback.insert(otherKey);
        }
    }

    {
        std::lock_guard lock(_statsMutex);
        for (auto& [key, lsInfo] : next->byLocClass) {
            auto& stats = _locStorageStats[key];
            stats.writableFailureDomains.store(lsInfo.failureDomains.size(), std::memory_order_relaxed);

            uint64_t totalServices = 0;
            uint64_t maxW = 0, minW = UINT64_MAX;
            for (const auto& fd : lsInfo.failureDomains) {
                totalServices += fd.services.size();
                maxW = std::max(maxW, fd.totalWeight);
                minW = std::min(minW, fd.totalWeight);
            }
            stats.writableBlockServices.store(totalServices, std::memory_order_relaxed);
            stats.maxWeight.store(maxW, std::memory_order_relaxed);
            stats.minWeight.store(lsInfo.failureDomains.empty() ? 0 : minW, std::memory_order_relaxed);
        }
    }

    _state.store(next, std::memory_order_release);
}

TernError BlockServicePicker::pick(
    uint8_t locationId,
    uint8_t storageClass,
    uint8_t needed,
    const std::vector<BlacklistEntry>& blacklist,
    std::vector<BlockServiceId>& out
) const {
    auto state = _state.load(std::memory_order_acquire);
    if (!state || needed == 0 || needed > _maxBlocksToPick) {
        LOG_DEBUG(_env, "pick failed: state=%s needed=%s maxBlocksToPick=%s",
            state != nullptr, (int)needed, (int)_maxBlocksToPick);
        return TernError::COULD_NOT_PICK_BLOCK_SERVICES;
    }

    uint16_t key = lcKey(locationId, storageClass);

    if (state->needsFallback.contains(key)) {
        storageClass = storageClass == HDD_STORAGE ? FLASH_STORAGE : HDD_STORAGE;
        key = lcKey(locationId, storageClass);
    }

    auto it = state->byLocClass.find(key);

    if (it != state->byLocClass.end()) {
        const auto& lsInfo = it->second;

        std::unordered_set<uint64_t> blacklistedServices;
        for (const auto& b : blacklist) {
            blacklistedServices.insert(b.blockService.u64);
        }

        // Build adjusted FD weights and lookup (copy and apply blacklist)
        std::vector<uint64_t> fdWeights;
        std::unordered_set<uint64_t> actuallyBlacklistedServices;
        fdWeights.reserve(lsInfo.failureDomains.size());
        uint64_t totalWeight = 0;
        uint64_t maxFdWeight = 0;

        for (const auto& fdInfo : lsInfo.failureDomains) {
            uint64_t adjustedWeight = fdInfo.totalWeight;
            for (const auto& b : blacklist) {
                if (b.failureDomain == fdInfo.failureDomain) {
                    adjustedWeight = 0;
                    break;
                }
            }

            fdWeights.emplace_back(adjustedWeight);
            totalWeight += adjustedWeight;
            maxFdWeight = std::max(maxFdWeight, adjustedWeight);
        }

        for(const auto& blacklistEntry : blacklist) {
            auto svcIt = state->serviceToFdInfo.find(blacklistEntry.blockService.u64);
            if (svcIt != state->serviceToFdInfo.end()) {
                const auto& svcInfo = svcIt->second;
                if (fdWeights[svcInfo.fdIndex] == 0) continue; // already blacklisted via FD
                if (svcInfo.lcKey == key) {
                    actuallyBlacklistedServices.insert(blacklistEntry.blockService.u64);
                    fdWeights[svcInfo.fdIndex] -= svcInfo.weight;
                    totalWeight -= svcInfo.weight;
                }
            }
        }

        if (totalWeight > 0 && needed > 0) {
            out.clear();
            out.reserve(needed);

            uint64_t step = totalWeight / needed;
            if (step >= maxFdWeight) {
                uint64_t offset = _rng.generate64() % totalWeight;

                for (uint8_t i = 0; i < needed; i++) {
                    uint64_t target = (offset + i * step) % totalWeight;

                    uint64_t cumulative = 0;
                    for (size_t fdIdx = 0; fdIdx < lsInfo.failureDomains.size(); fdIdx++) {
                        uint64_t fdWeight = fdWeights[fdIdx];
                        if (fdWeight == 0) continue;

                        if (target < cumulative + fdWeight) {
                            const auto& fdInfo = lsInfo.failureDomains[fdIdx];
                            uint64_t fdTarget = target - cumulative;
                            uint64_t svcCumulative = 0;

                            for (const auto& svc : fdInfo.services) {
                                if (actuallyBlacklistedServices.contains(svc.id.u64)) continue;

                                if (fdTarget < svcCumulative + svc.availableBytes) {
                                    out.push_back(svc.id);
                                    {
                                        std::lock_guard lock(_statsMutex);
                                        _blockServiceStats[svc.id.u64].fetch_add(1, std::memory_order_relaxed);
                                        std::string fdStr(reinterpret_cast<const char*>(fdInfo.failureDomain.name.data.data()), 16);
                                        _failureDomainStats[fdStr].fetch_add(1, std::memory_order_relaxed);
                                    }
                                    break;
                                }
                                svcCumulative += svc.availableBytes;
                            }
                            break;
                        }
                        cumulative += fdWeight;
                    }
                }

                if (out.size() == needed) {
                    {
                        std::lock_guard lock(_statsMutex);
                        _locStorageStats[key].totalPicks.fetch_add(needed, std::memory_order_relaxed);
                    }
                    return TernError::NO_ERROR;
                }
            }
        }
    }

    if (_env.shouldLog(LogLevel::LOG_DEBUG)) {
        auto it2 = state->byLocClass.find(key);
        if (it2 == state->byLocClass.end()) {
            LOG_DEBUG(_env, "pick failed: no entry for location=%s storageClass=%s (key=0x%s), byLocClass has %s entries, needsFallback has %s entries",
                (int)locationId, (int)storageClass, key, state->byLocClass.size(), state->needsFallback.size());
            for (const auto& [k, lsInfo] : state->byLocClass) {
                LOG_DEBUG(_env, "  byLocClass key=0x%s: failureDomains=%s totalWeight=%s",
                    k, lsInfo.failureDomains.size(), lsInfo.totalWeight);
            }
        } else {
            const auto& lsInfo = it2->second;
            size_t totalServices = 0;
            for (const auto& fd : lsInfo.failureDomains) {
                totalServices += fd.services.size();
            }
            LOG_DEBUG(_env, "pick failed: location=%s storageClass=%s needed=%s blacklist=%s failureDomains=%s totalServices=%s totalWeight=%s",
                (int)locationId, (int)storageClass, (int)needed, blacklist.size(), lsInfo.failureDomains.size(), totalServices, lsInfo.totalWeight);
            for (size_t fdIdx = 0; fdIdx < lsInfo.failureDomains.size(); fdIdx++) {
                const auto& fdInfo = lsInfo.failureDomains[fdIdx];
                LOG_DEBUG(_env, "  fd[%s]: services=%s totalWeight=%s",
                    fdIdx, fdInfo.services.size(), fdInfo.totalWeight);
            }
        }
    }
    out.clear();
    return TernError::COULD_NOT_PICK_BLOCK_SERVICES;
}

BlockServicePicker::StatsSnapshot BlockServicePicker::getStats() const {
    StatsSnapshot snapshot;
    std::lock_guard lock(_statsMutex);

    for (const auto& [key, stats] : _locStorageStats) {
        snapshot.locStorage.push_back({
            key,
            stats.totalPicks.load(std::memory_order_relaxed),
            stats.writableFailureDomains.load(std::memory_order_relaxed),
            stats.writableBlockServices.load(std::memory_order_relaxed),
            stats.maxWeight.load(std::memory_order_relaxed),
            stats.minWeight.load(std::memory_order_relaxed)
        });
    }

    uint64_t minPicks = UINT64_MAX, maxPicks = 0;
    for (const auto& [id, stats] : _blockServiceStats) {
        uint64_t picks = stats.load(std::memory_order_relaxed);
        snapshot.blockServices.push_back({id, picks});
        if (picks > 0) {
            minPicks = std::min(minPicks, picks);
            maxPicks = std::max(maxPicks, picks);
        }
    }
    snapshot.minServicePicks = (minPicks == UINT64_MAX) ? 0 : minPicks;
    snapshot.maxServicePicks = maxPicks;

    uint64_t minFdPicks = UINT64_MAX, maxFdPicks = 0;
    for (const auto& [fd, stats] : _failureDomainStats) {
        uint64_t picks = stats.load(std::memory_order_relaxed);
        snapshot.failureDomains.push_back({fd, picks});
        if (picks > 0) {
            minFdPicks = std::min(minFdPicks, picks);
            maxFdPicks = std::max(maxFdPicks, picks);
        }
    }
    snapshot.minFdPicks = (minFdPicks == UINT64_MAX) ? 0 : minFdPicks;
    snapshot.maxFdPicks = maxFdPicks;

    return snapshot;
}

void BlockServicePicker::resetStats() {
    std::lock_guard lock(_statsMutex);

    for (auto& [key, stats] : _locStorageStats) {
        stats.totalPicks.store(0, std::memory_order_relaxed);
    }

    for (auto& [id, stats] : _blockServiceStats) {
        stats.store(0, std::memory_order_relaxed);
    }

    for (auto& [fd, stats] : _failureDomainStats) {
        stats.store(0, std::memory_order_relaxed);
    }
}
