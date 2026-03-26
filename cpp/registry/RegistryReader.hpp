// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include "Loop.hpp"
#include "Msgs.hpp"
#include "RegistryCommon.hpp"
#include "RegistryDB.hpp"
#include "RegistryServer.hpp"
#include "SPSC.hpp"
#include <vector>


class RegistryReader : public Loop {
public:
    static constexpr uint32_t MAX_REQUESTS = 128;
    RegistryReader(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const RegistryOptions& options, RegistryDB& db, RegistryServer& server) : Loop(logger, xmon, "registry_reader"),
        _options(options), _registryDB(db), _server(server), _queue(MAX_REQUESTS) {
            _reqs.reserve(MAX_REQUESTS);
            _resps.reserve(MAX_REQUESTS);
        }

    std::vector<RegistryRequest>::iterator pushRequests(std::vector<RegistryRequest>::iterator begin, std::vector<RegistryRequest>::iterator end);

    virtual void step() override;

    virtual void sendStop() override;

private:
    const RegistryOptions& _options;
    RegistryDB& _registryDB;
    RegistryServer& _server;

    SPSC<RegistryRequest> _queue;

    std::vector<RegistryRequest> _reqs;
    std::vector<RegistryResponse> _resps;

    struct PendingWait {
        uint64_t requestId;
        RegistryReqContainer req;
    };
    std::vector<PendingWait> _pendingWaits;

// local cache for common data, read once per step

    std::vector<FullRegistryInfo> _cachedRegistries;

    std::vector<FullShardInfo> _cachedShardInfo;
    std::unordered_map<LocationId, std::vector<ShardInfo>> _cachedShardInfoPerLocation;

    std::vector<FullBlockServiceInfo> _cachedBlockServices;
    std::vector<BlockServiceDeprecatedInfo> _cachedAllBlockServices;
    std::unordered_map<BlockServiceId, AES128Key> _decommissionedServices;

    std::vector<CdcInfo> _cachedCdc;

    InfoResp _cachedInfo;

    void _clearCaches();
    void _populateShardCache();
    const std::vector<FullRegistryInfo>& _registries();
    const std::vector<ShardInfo>& _shardsAtLocation(LocationId location);
    void _populateCdcCache();
    CdcInfo _cdcAtLocation(LocationId location);
    std::vector<AddrsInfo> _cdcReplicas();
    void _populateBlockServiceCache();
    InfoResp _info();
    std::vector<BlockServiceDeprecatedInfo> _allBlockServices();
    std::vector<FullBlockServiceInfo> _allBlockServicesFull();
    std::vector<BlockServiceId> _blockServicesNeedingMigration(LocationId location);
    std::vector<BlockService> _changedBlockServices(LocationId location, TernTime changedSince);
    TernError _eraseBlock(const EraseDecommissionedBlockReq& req, BincodeFixedBytes<8>& proof);
};
