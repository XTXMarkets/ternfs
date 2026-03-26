// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "RegistryReader.hpp"
#include "Assert.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "RegistryCommon.hpp"
#include "RegistryDB.hpp"
#include "RegistryServer.hpp"
#include "SPSC.hpp"


std::vector<RegistryRequest>::iterator RegistryReader::pushRequests(std::vector<RegistryRequest>::iterator begin, std::vector<RegistryRequest>::iterator end) {
    return _queue.push(begin, end);
}

void RegistryReader::step() {
    ALWAYS_ASSERT(_reqs.empty());
    ALWAYS_ASSERT(_resps.empty());
    _clearCaches();
    _queue.pull(_reqs, MAX_REQUESTS, _pendingWaits.empty() ? Duration(-1) : 1_sec);
    for (auto& pw : _pendingWaits) {
        auto& req = _reqs.emplace_back();
        req.requestId = pw.requestId;
        req.req = std::move(pw.req);
    }
    _pendingWaits.clear();
    for (auto &req : _reqs) {
        auto& resp = _resps.emplace_back();
        resp.requestId = req.requestId;
        switch (req.req.kind()) {
        case RegistryMessageKind::LOCAL_SHARDS: {
            auto& registryResp = resp.resp.setLocalShards();
            registryResp.shards.els = _shardsAtLocation(_options.logsDBOptions.location);
            break;
        }
        case RegistryMessageKind::LOCAL_CDC: {
            auto& registryResp = resp.resp.setLocalCdc();
            auto cdcInfo = _cdcAtLocation(_options.logsDBOptions.location);
            registryResp.addrs = cdcInfo.addrs;
            registryResp.lastSeen = cdcInfo.lastSeen;
            break;
        }
        case RegistryMessageKind::INFO: {
            auto& registryResp = resp.resp.setInfo();
            auto cachedInfo = _info();
            registryResp.capacity = cachedInfo.capacity;
            registryResp.available = cachedInfo.available;
            registryResp.blocks = cachedInfo.blocks;
            registryResp.numBlockServices = cachedInfo.numBlockServices;
            registryResp.numFailureDomains = cachedInfo.numFailureDomains;
            break;
        }
        case RegistryMessageKind::REGISTRY: {
            auto &registryResp = resp.resp.setRegistry();
            registryResp.addrs = _server.boundAddresses();
            break;
        }
        case RegistryMessageKind::LOCATIONS: {
            auto& registryResp = resp.resp.setLocations();
            _registryDB.locations(registryResp.locations.els);
            break;
        }
        case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES: {
            auto& registryResp = resp.resp.setLocalChangedBlockServices();
            auto& changedReq = req.req.getLocalChangedBlockServices();
            registryResp.blockServices.els = _changedBlockServices(_options.logsDBOptions.location, changedReq.changedSince);
            break;
        }
        case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION: {
            auto& registryResp = resp.resp.setChangedBlockServicesAtLocation();
            auto& changedReq = req.req.getChangedBlockServicesAtLocation();
            registryResp.blockServices.els = _changedBlockServices(changedReq.locationId, changedReq.changedSince);
            break;
        }
        case RegistryMessageKind::SHARDS_AT_LOCATION: {
            auto& registryResp = resp.resp.setShardsAtLocation();
            registryResp.shards.els = _shardsAtLocation(req.req.getShardsAtLocation().locationId);
            break;
        }
        case RegistryMessageKind::CDC_AT_LOCATION: {
            auto& registryResp = resp.resp.setCdcAtLocation();
            auto cdcInfo = _cdcAtLocation(req.req.getCdcAtLocation().locationId);
            registryResp.addrs = cdcInfo.addrs;
            registryResp.lastSeen = cdcInfo.lastSeen;
            break;
        }
        case RegistryMessageKind::ALL_REGISTRY_REPLICAS_DE_PR_EC_AT_ED: {
            auto &allRegiResp = resp.resp.setAllRegistryReplicasDEPRECATED();
            allRegiResp.replicas.els = _registries();
            break;
        }
        case RegistryMessageKind::ALL_REGISTRY_REPLICAS: {
            const auto& allRegiReq = req.req.getAllRegistryReplicas();
            if (allRegiReq.minKnownReplicas > 0) {
                auto& regs = _registries();
                auto count = std::count_if(regs.begin(), regs.end(), [&](const auto& r) {
                    return r.locationId == allRegiReq.location && (r.addrs.addrs[0].port != 0 || r.addrs.addrs[1].port != 0);
                });
                if (count < allRegiReq.minKnownReplicas) {
                    _resps.pop_back();
                    _pendingWaits.emplace_back(PendingWait{req.requestId, std::move(req.req)});
                    break;
                }
            }
            auto &allRegiResp = resp.resp.setAllRegistryReplicas();
            allRegiResp.replicas.els = _registries();
            break;
        }
        case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED: {
            auto& registryResp = resp.resp.setShardBlockServicesDEPRECATED();
            auto& bsReq = req.req.getShardBlockServicesDEPRECATED();
            std::vector<BlockServiceInfoShort> blockservices;
            _registryDB.shardBlockServices(bsReq.shardId, blockservices);
            for (auto& bs : blockservices) {
                registryResp.blockServices.els.push_back(bs.id);
            }
            break;
        }
        case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED: {
            auto& registryResp = resp.resp.setCdcReplicasDEPRECATED();
            registryResp.replicas.els = _cdcReplicas();
            break;
        }
        case RegistryMessageKind::ALL_SHARDS_DE_PR_EC_AT_ED: {
            auto& registryResp = resp.resp.setAllShardsDEPRECATED();
            _populateShardCache();
            registryResp.shards.els = _cachedShardInfo;
            break;
        }

        case RegistryMessageKind::SHARD_BLOCK_SERVICES: {
            auto& registryResp = resp.resp.setShardBlockServices();
            auto& bsReq = req.req.getShardBlockServices();
            _registryDB.shardBlockServices(bsReq.shardId, registryResp.blockServices.els);
            break;
        }
        case RegistryMessageKind::ALL_CDC_DE_PR_EC_AT_ED: {
            auto& registryResp = resp.resp.setAllCdcDEPRECATED();
            _populateCdcCache();
            registryResp.replicas.els = _cachedCdc;
            break;
        }
        case RegistryMessageKind::ALL_SHARDS: {
            _populateShardCache();
            const auto& allShardsReq = req.req.getAllShards();
            if (allShardsReq.minKnownReplicas > 0) {
                auto count = std::count_if(_cachedShardInfo.begin(), _cachedShardInfo.end(), [&](const auto& s) {
                    return s.id.shardId() == ShardId{allShardsReq.shardId} && s.locationId == allShardsReq.location && (s.addrs.addrs[0].port != 0 || s.addrs.addrs[1].port != 0);
                });
                if (count < allShardsReq.minKnownReplicas) {
                    _resps.pop_back();
                    _pendingWaits.emplace_back(PendingWait{req.requestId, std::move(req.req)});
                    break;
                }
            }
            auto& registryResp = resp.resp.setAllShards();
            registryResp.shards.els = _cachedShardInfo;
            break;
        }
        case RegistryMessageKind::ALL_CDC: {
            _populateCdcCache();
            const auto& allCdcReq = req.req.getAllCdc();
            if (allCdcReq.minKnownReplicas > 0) {
                auto count = std::count_if(_cachedCdc.begin(), _cachedCdc.end(), [&](const auto& c) {
                    return c.locationId == allCdcReq.location && (c.addrs.addrs[0].port != 0 || c.addrs.addrs[1].port != 0);
                });
                if (count < allCdcReq.minKnownReplicas) {
                    _resps.pop_back();
                    _pendingWaits.emplace_back(PendingWait{req.requestId, std::move(req.req)});
                    break;
                }
            }
            auto& registryResp = resp.resp.setAllCdc();
            registryResp.replicas.els = _cachedCdc;
            break;
        }
        case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK: {
            auto& eraseReq = req.req.getEraseDecommissionedBlock();
            BincodeFixedBytes<8> proof;
            auto err = _eraseBlock(eraseReq, proof);
            if (err == TernError::NO_ERROR) {
                 auto& registryResp = resp.resp.setEraseDecommissionedBlock();
                 registryResp.proof = proof;
            } else {
                resp.resp.setError() = err;
            }
            break;
        }
        case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED: {
            auto &registryResp = resp.resp.setAllBlockServicesDeprecated();
            registryResp.blockServices.els = _allBlockServices();
            break;
        }
        case RegistryMessageKind::ALL_BLOCK_SERVICES: {
            auto &registryResp = resp.resp.setAllBlockServices();
            registryResp.blockServices.els = _allBlockServicesFull();
            break;
        }
        case RegistryMessageKind::BLOCK_SERVICES_NEEDING_MIGRATION: {
            auto &registryResp = resp.resp.setBlockServicesNeedingMigration();
            auto &migrationReq = req.req.getBlockServicesNeedingMigration();
            registryResp.blockServices.els = _blockServicesNeedingMigration(migrationReq.locationId);
            break;
        }
        default:
            ALWAYS_ASSERT(false, "unexpected request kind %s", req.req.kind());
        }
    }
    _reqs.clear();
    _server.sendRegistryResponses(_resps);
}

void RegistryReader::sendStop() {
    _queue.close();
}

void RegistryReader::_clearCaches() {
    _cachedRegistries.clear();
    _cachedShardInfo.clear();
    for(auto& loc : _cachedShardInfoPerLocation) {
        loc.second.clear();
        loc.second.resize(256);
    }
    _cachedBlockServices.clear();
    _cachedAllBlockServices.clear();
    _cachedInfo.clear();
    _cachedCdc.clear();
}

void RegistryReader::_populateShardCache() {
    if (!_cachedShardInfo.empty()) {
        return;
    }
    _registryDB.shards(_cachedShardInfo);
    for(auto& shard : _cachedShardInfo) {
        if (!shard.isLeader) {
            continue;
        }
        auto& locShards = _cachedShardInfoPerLocation[shard.locationId];
        if (locShards.empty()) {
            locShards.resize(256);
        }
        auto& infoShort = locShards[shard.id.shardId().u8];
        infoShort.addrs = shard.addrs;
        infoShort.lastSeen = shard.lastSeen;
    }
    if (_cachedShardInfoPerLocation.empty()) {
        _cachedShardInfoPerLocation[_options.logsDBOptions.location].resize(256);
    }
}

const std::vector<FullRegistryInfo>& RegistryReader::_registries() {
    if (_cachedRegistries.empty()) {
        _registryDB.registries(_cachedRegistries);
    }
    return _cachedRegistries;
}

const std::vector<ShardInfo>& RegistryReader::_shardsAtLocation(LocationId location) {
    _populateShardCache();
    return _cachedShardInfoPerLocation[location];
}

void RegistryReader::_populateCdcCache() {
    if (!_cachedCdc.empty()) {
        return;
    }
    _registryDB.cdcs(_cachedCdc);
}

CdcInfo RegistryReader::_cdcAtLocation(LocationId location) {
    _populateCdcCache();
    for(const auto& cdc : _cachedCdc) {
        if (cdc.isLeader && cdc.locationId == location) {
            return cdc;
        }
    }
    return CdcInfo{};
}

std::vector<AddrsInfo> RegistryReader::_cdcReplicas() {
    _populateCdcCache();
    std::vector<AddrsInfo> res;
    res.resize(LogsDB::REPLICA_COUNT);
    for(const auto& cdc : _cachedCdc) {
        if (cdc.locationId != 0) {
            continue;
        }
        res[cdc.replicaId.u8] = cdc.addrs;
    }
    return res;
}

void RegistryReader::_populateBlockServiceCache() {
    if (!_cachedBlockServices.empty()) {
        return;
    }
    _registryDB.blockServices(_cachedBlockServices);
    std::sort(_cachedBlockServices.begin(), _cachedBlockServices.end(),
    [](const FullBlockServiceInfo& a, const FullBlockServiceInfo& b) {
                return a.lastInfoChange > b.lastInfoChange;
    });

    for (auto& bs : _cachedBlockServices) {
        auto& infoDeprecated = _cachedAllBlockServices.emplace_back();
        infoDeprecated.id = bs.id;
        infoDeprecated.addrs = bs.addrs;
        infoDeprecated.storageClass = bs.storageClass;
        infoDeprecated.failureDomain = bs.failureDomain;
        infoDeprecated.secretKey = bs.secretKey;
        infoDeprecated.flags = bs.flags;
        infoDeprecated.capacityBytes = bs.capacityBytes;
        infoDeprecated.availableBytes = bs.availableBytes;
        infoDeprecated.blocks = bs.blocks;
        infoDeprecated.path = bs.path;
        infoDeprecated.lastSeen = bs.lastSeen;
        infoDeprecated.hasFiles = bs.hasFiles;
        infoDeprecated.flagsLastChanged = bs.lastInfoChange;
        if (bs.flags == BlockServiceFlags::DECOMMISSIONED && !_decommissionedServices.contains(bs.id)) {
            auto it = _decommissionedServices.emplace(bs.id, AES128Key{}).first;
            expandKey(bs.secretKey.data, it->second);
        }
        if (bs.flags != BlockServiceFlags::DECOMMISSIONED) {
            ++_cachedInfo.numBlockServices;
            _cachedInfo.blocks += bs.blocks;
            _cachedInfo.available += bs.availableBytes;
            _cachedInfo.capacity += bs.capacityBytes;
        }
    }
}

InfoResp RegistryReader::_info() {
    _populateBlockServiceCache();
    return _cachedInfo;
}

std::vector<BlockServiceDeprecatedInfo> RegistryReader::_allBlockServices() {
    _populateBlockServiceCache();
    return _cachedAllBlockServices;
}

std::vector<FullBlockServiceInfo> RegistryReader::_allBlockServicesFull() {
    _populateBlockServiceCache();
    return _cachedBlockServices;
}

std::vector<BlockServiceId> RegistryReader::_blockServicesNeedingMigration(LocationId location) {
    _populateBlockServiceCache();
    std::vector<BlockServiceId> res;
    for (auto& bs : _cachedBlockServices) {
        if (bs.locationId == location && bs.flags == BlockServiceFlags::DECOMMISSIONED && bs.hasFiles) {
            res.push_back(bs.id);
        }
    }
    return res;
}

std::vector<BlockService> RegistryReader::_changedBlockServices(LocationId location, TernTime changedSince) {
    _populateBlockServiceCache();
    std::vector<BlockService> res;
    for (auto& bs : _cachedBlockServices) {
        if (bs.lastInfoChange < changedSince) {
            break;
        }
        if (bs.locationId != location) {
            continue;
        }
        auto& bsOut = res.emplace_back();
        bsOut.id = bs.id;
        bsOut.addrs = bs.addrs;
        bsOut.flags = bsOut.flags;
    }
    return res;
}

TernError RegistryReader::_eraseBlock(const EraseDecommissionedBlockReq& req, BincodeFixedBytes<8>& proof) {
    _populateBlockServiceCache();
    auto bsIt = _decommissionedServices.find(req.blockServiceId);
    if (bsIt == _decommissionedServices.end()) {
        return TernError::BLOCK_SERVICE_NOT_FOUND;
    }

    // validate certificate
    {
        char buf[32];
        memset(buf, 0, sizeof(buf));
        BincodeBuf bbuf(buf, sizeof(buf));
        // struct.pack_into('<QcQ', b, 0, block['block_service_id'], b'e', block['block_id'])
        bbuf.packScalar<uint64_t>(req.blockServiceId.u64);
        bbuf.packScalar<char>('e');
        bbuf.packScalar<uint64_t>(req.blockId);
        if (cbcmac(bsIt->second, (uint8_t*)buf, sizeof(buf)) != req.certificate) {
            return TernError::BAD_CERTIFICATE;
        }
    }
    // generate proof
    {
        char buf[32];
        memset(buf, 0, sizeof(buf));
        BincodeBuf bbuf(buf, sizeof(buf));
        // struct.pack_into('<QcQ', b, 0, block['block_service_id'], b'E', block['block_id'])
        bbuf.packScalar<uint64_t>(req.blockServiceId.u64);
        bbuf.packScalar<char>('E');
        bbuf.packScalar<uint64_t>(req.blockId);
        proof.data = cbcmac(bsIt->second, (uint8_t*)buf, sizeof(buf));
    }
    return TernError::NO_ERROR;
}
