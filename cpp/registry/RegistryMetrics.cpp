// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "RegistryMetrics.hpp"

#include <algorithm>
#include <cstring>
#include <unordered_map>

// Tag values may only have the characters validMetricsName allows.
static std::string tagValue(const char* data, size_t len) {
    std::string out(data, len);
    for (auto& ch : out) {
        if (!(
            (ch >= 'a' && ch <= 'z') ||
            (ch >= 'A' && ch <= 'Z') ||
            (ch >= '0' && ch <= '9') ||
            ch == '-' || ch == '_' || ch == '.' || ch == ':'
        )) {
            ch = '-';
        }
    }
    return out;
}

void decommissionMetrics(MetricsBuilder& metrics, const std::vector<FullBlockServiceInfo>& blockServices, uint8_t unreadableLimit, TernTime now) {
    // Every location and storage class gets its counts, even when they're 0.
    struct Group {
        uint32_t unreadable = 0;
        uint64_t decommissioned = 0;
        uint64_t decommissionedWithFiles = 0;
    };
    std::unordered_map<uint16_t, Group> groups;
    auto group = [&](uint8_t locationId, uint8_t storageClass) -> Group& {
        return groups[(uint16_t(locationId) << 8) | storageClass];
    };
    for (const auto& u : unreadableFailureDomains(blockServices)) {
        group(u.locationId, u.storageClass).unreadable = u.count;
    }

    for (const auto& bs : blockServices) {
        auto& g = group(bs.locationId, bs.storageClass);
        if (bs.flags != BlockServiceFlags::DECOMMISSIONED) {
            continue;
        }
        g.decommissioned++;
        if (!bs.hasFiles) {
            continue;
        }
        g.decommissionedWithFiles++;
        // Nothing changes a block service once it is decommissioned, so
        // lastInfoChange is when that happened.
        metrics.measurement("eggsfs_registry_decommissioned_block_service");
        metrics.tag("blockservice", bs.id);
        metrics.tag("location", int(bs.locationId));
        metrics.tagRaw("storageclass", storageClassName(bs.storageClass));
        const char* fd = (const char*)bs.failureDomain.name.data.data();
        metrics.tagRaw("failuredomain", tagValue(fd, strnlen(fd, bs.failureDomain.name.data.size())));
        const char* colon = (const char*)memchr(bs.path.data(), ':', bs.path.size());
        if (colon != nullptr && colon != bs.path.data()) {
            metrics.tagRaw("pathprefix", tagValue(bs.path.data(), colon - bs.path.data()));
        }
        metrics.fieldU64("decommissioned_at", bs.lastInfoChange.ns / 1'000'000'000ull);
        metrics.fieldU64("age", std::max<int64_t>(0, (now - bs.lastInfoChange).ns) / 1'000'000'000ll);
        metrics.timestamp(now);
    }

    for (const auto& [key, g] : groups) {
        uint8_t locationId = key >> 8;
        uint8_t storageClass = key & 0xFF;
        metrics.measurement("eggsfs_registry_unreadable_failure_domains");
        metrics.tag("location", int(locationId));
        metrics.tagRaw("storageclass", storageClassName(storageClass));
        metrics.fieldU64("count", g.unreadable);
        metrics.fieldU64("limit", unreadableLimit);
        metrics.timestamp(now);
        metrics.measurement("eggsfs_registry_decommissioned_block_services");
        metrics.tag("location", int(locationId));
        metrics.tagRaw("storageclass", storageClassName(storageClass));
        metrics.fieldU64("count", g.decommissioned);
        metrics.fieldU64("with_files", g.decommissionedWithFiles);
        metrics.timestamp(now);
    }
}

RegistryMetricsInserter::RegistryMetricsInserter(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const RegistryOptions& options, const RegistryDB& registryDB, const LogsDB& logsDB) :
    PeriodicLoop(logger, xmon, "metrics", {1_sec, 1.0, 1_mins, 0.1}),
    _options(options),
    _registryDB(registryDB),
    _logsDB(logsDB),
    _sendMetricsAlert(XmonAppType::DAYTIME, 1_mins)
{}

bool RegistryMetricsInserter::periodicStep() {
    if (!_logsDB.getStats().isLeader.load(std::memory_order_relaxed)) {
        _env.clearAlert(_sendMetricsAlert);
        return true;
    }
    _registryDB.blockServices(_blockServices);
    decommissionMetrics(_metricsBuilder, _blockServices, _options.alertAfterUnavailableFailureDomains, ternNow());
    if (_metricsBuilder.payload().empty()) {
        return true;
    }
    std::string err = sendMetrics(_options.metricsOptions, 10_sec, _metricsBuilder.payload());
    _metricsBuilder.reset();
    if (err.empty()) {
        LOG_INFO(_env, "Sent metrics to influxdb");
        _env.clearAlert(_sendMetricsAlert);
        return true;
    } else {
        _env.updateAlert(_sendMetricsAlert, "Could not insert metrics: %s", err);
        return false;
    }
}
