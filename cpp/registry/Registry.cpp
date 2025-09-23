// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <atomic>
#include <cstring>
#include <fcntl.h>
#include <memory>
#include <netinet/in.h>
#include <sys/epoll.h>
#include <sys/socket.h>
#include <unistd.h>
#include <unordered_map>

#include "Assert.hpp"
#include "Bincode.hpp"
#include "Crypto.hpp"
#include "Env.hpp"
#include "ErrorCount.hpp"
#include "LogsDB.hpp"
#include "Loop.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"

#include "Registry.hpp"
#include "Registerer.hpp"
#include "RegistryDB.hpp"
#include "RegistryServer.hpp"

#include "SharedRocksDB.hpp"
#include "Time.hpp"
#include "Timings.hpp"

#include "XmonAgent.hpp"

struct RegistryState {
    explicit RegistryState() :         
        logEntriesQueueSize(0), readingRequests(0), writingRequests(0),
        activeConnections(0), requestsInProgress(0)
    {
        for (RegistryMessageKind kind : allRegistryMessageKind) {
        timings[(int)kind] = Timings::Standard();
        }
        for (auto &x : receivedRequests) {
        x = 0;
        }
    }

    std::unique_ptr<RegistryServer> server;

    // databases
    std::unique_ptr<SharedRocksDB> sharedDB;
    std::unique_ptr<LogsDB> logsDB;
    std::unique_ptr<RegistryDB> registryDB;  

    // statistics
    std::array<Timings, maxRegistryMessageKind + 1> timings;
    std::array<ErrorCount, maxRegistryMessageKind + 1> errors;
    std::atomic<double> logEntriesQueueSize;
    std::array<std::atomic<double>, 2> receivedRequests; // how many requests we got at once from each socket
    std::atomic<double> readingRequests; // how many requests we got from write queue
    std::atomic<double> writingRequests; // how many requests we got from read queue
    std::atomic<double> activeConnections;
    std::atomic<double> requestsInProgress;
};

class RegistryLoop : public Loop {
public:
    RegistryLoop(Logger &logger, std::shared_ptr<XmonAgent> xmon, const RegistryOptions& options, Registerer& registerer, RegistryServer& server, LogsDB& logsDB, RegistryDB& registryDB) :
        Loop(logger, xmon, "server"),
        _options(options),
        _registerer(registerer),
        _server(server),
        _logsDB(logsDB),
        _registryDB(registryDB),
        _replicas({}),
        _replicaFinishedBootstrap({}),
        _boostrapFinished(false)
        {}

    virtual ~RegistryLoop() {}

    virtual void step() override {
        _server.setReplicas(_registerer.replicas());
        if (!_server.receiveMessages(_logsDB.getNextTimeout())){
            return;
        }

        auto now = ternNow();
        
        _logsDB.processIncomingMessages(_server.receivedLogsDBRequests(), _server.receivedLogsDBResponses());

        auto& receivedRequests = _server.receivedRegistryRequests();
    
        for (auto &req : receivedRequests) {
            if (unlikely(!_boostrapFinished)) {
                _processBootstrapRequest(req);
                continue;
            }
            auto& resp = _registryResponses.emplace_back();
            resp.requestId = req.requestId;
            if (unlikely(!_logsDB.isLeader())) {
                LOG_DEBUG(_env, "not leader. dropping request from client %s", req.requestId);
                continue;
            }            
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
            case RegistryMessageKind::ALL_REGISTRY_REPLICAS: {
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
            case RegistryMessageKind::ALL_SHARDS: {
                auto& registryResp = resp.resp.setAllShards();
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
            case RegistryMessageKind::ALL_CDC: {
                auto& registryResp = resp.resp.setAllCdc();
                _populateCdcCache();
                registryResp.replicas.els = _cachedCdc;
                break;
            }
            case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK: {
                auto& eraseReq = req.req.getEraseDecommissionedBlock();
                BincodeFixedBytes<8> proof;
                if (_eraseBlock(eraseReq, proof)) {
                     auto& registryResp = resp.resp.setEraseDecommissionedBlock();
                     registryResp.proof = proof;
                } else {
                    resp.resp.setError() = TernError::INTERNAL_ERROR;
                }    
                break;
            }
            case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED: {
                auto &registryResp = resp.resp.setAllBlockServicesDeprecated();
                registryResp.blockServices.els = _allBlockServices();
                break;
            }
            case RegistryMessageKind::REGISTER_BLOCK_SERVICES: {
                if (_logEntries.size() >= LogsDB::IN_FLIGHT_APPEND_WINDOW) {
                    break;
                }
                bool failed = true;
                //happy path. request fits in udp package and we don't want to copy
                if (sizeof(now) + req.req.packedSize() <= LogsDB::DEFAULT_UDP_ENTRY_SIZE) {
                    try {
                        auto &entry = _logEntries.emplace_back();
                        entry.value.resize(sizeof(now) + req.req.packedSize());
                        ALWAYS_ASSERT(entry.value.size() <= LogsDB::DEFAULT_UDP_ENTRY_SIZE);
                        BincodeBuf buf((char *)(&entry.value[0]), entry.value.size());
                        buf.packScalar(now.ns);
                        req.req.pack(buf);
                        buf.ensureFinished();
                        _entriesRequestIds.emplace_back(req.requestId);
                        failed = false;
                    } catch (BincodeException e) {
                        LOG_ERROR(_env,"failed packing log entry request %s", e.what());
                        _logEntries.pop_back();
                    }
                } else {
                    std::vector<RegisterBlockServiceInfo> blockservices;
                    blockservices = req.req.getRegisterBlockServices().blockServices.els;
                    RegistryReqContainer tmpReqContainer;
                    auto& tmpBsReq = tmpReqContainer.setRegisterBlockServices();
                    while (!blockservices.empty()) {
                        size_t currentSize = sizeof(now) + RegistryReqContainer::STATIC_SIZE;
                        size_t toCopy = 0;
                        for (auto bsIt = blockservices.rbegin(); bsIt != blockservices.rend(); ++bsIt) {
                            currentSize += bsIt->packedSize();
                            if (currentSize > LogsDB::DEFAULT_UDP_ENTRY_SIZE) {
                                break;
                            }
                            ++toCopy;
                        }
                        tmpBsReq.blockServices.els.clear();
                        tmpBsReq.blockServices.els.insert(
                            tmpBsReq.blockServices.els.end(),blockservices.end() - toCopy,blockservices.end());
                        blockservices.resize(blockservices.size() - toCopy);
                        try {
                            auto &entry = _logEntries.emplace_back();
                            entry.value.resize(sizeof(now) + tmpReqContainer.packedSize());
                            ALWAYS_ASSERT(entry.value.size() <= LogsDB::DEFAULT_UDP_ENTRY_SIZE);
                            BincodeBuf buf((char *)(&entry.value[0]), entry.value.size());
                            buf.packScalar(now.ns);
                            tmpReqContainer.pack(buf);
                            buf.ensureFinished();
                            _entriesRequestIds.emplace_back(req.requestId);
                            failed = false;
                        } catch (BincodeException e) {
                            LOG_ERROR(_env,"failed packing log entry request %s", e.what());
                            _logEntries.pop_back();
                            break;
                        }
                    }    
                }
                
                if (!failed) {
                    // we are not sending response yet. we can't leave empty response otherwise
                    // server will drop connection
                    _registryResponses.pop_back();
                } else {
                    LOG_ERROR(_env,"FAILED REGISTERING");
                }
                break;
            }
            case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
            case RegistryMessageKind::CREATE_LOCATION:
            case RegistryMessageKind::RENAME_LOCATION:
            case RegistryMessageKind::REGISTER_SHARD:
            case RegistryMessageKind::REGISTER_REGISTRY:
            case RegistryMessageKind::REGISTER_CDC:
            case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
            case RegistryMessageKind::MOVE_SHARD_LEADER:
            case RegistryMessageKind::CLEAR_SHARD_INFO:
            case RegistryMessageKind::MOVE_CDC_LEADER:
            case RegistryMessageKind::CLEAR_CDC_INFO:
            case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH: {
                if (_logEntries.size() >= LogsDB::IN_FLIGHT_APPEND_WINDOW) {
                    break;
                }
                try {
                    auto &entry = _logEntries.emplace_back();
                    entry.value.resize(sizeof(now) + req.req.packedSize());
                    ALWAYS_ASSERT(entry.value.size() <= LogsDB::MAX_UDP_ENTRY_SIZE);
                    BincodeBuf buf((char *)(&entry.value[0]), entry.value.size());
                    buf.packScalar(now.ns);
                    req.req.pack(buf);
                    buf.ensureFinished();
                    _entriesRequestIds.emplace_back(req.requestId);
                    // we are not sending response yet. we can't leave empty response otherwise
                    // server will drop connection
                    _registryResponses.pop_back();
                } catch (BincodeException e) {
                    LOG_ERROR(_env,"failed packing log entry request %s", e.what());
                    _logEntries.pop_back();
                }
                break;
            } 
            case RegistryMessageKind::EMPTY:
            case RegistryMessageKind::ERROR:
                break;
            }
        }
        receivedRequests.clear();

        if (_logsDB.isLeader()) {
            _logsDB.appendEntries(_logEntries);
            for (int i = 0; i < _logEntries.size(); ++i) {
                auto &entry = _logEntries[i];
                auto requestId = _entriesRequestIds[i];
                if (entry.idx == 0) {
                    LOG_DEBUG(_env, "could not append entry for request %s", requestId);
                    // empty response drops request
                    _registryResponses.emplace_back().requestId = requestId;
                    continue;
                }
                _logIdxToRequestId.emplace(entry.idx.u64, requestId);
            }
            _logEntries.clear();
            _entriesRequestIds.clear();
        }
        ALWAYS_ASSERT(_logEntries.empty());
        ALWAYS_ASSERT(_entriesRequestIds.empty());
             
        do {
            _logEntries.clear();
            _logsDB.readEntries(_logEntries);
            _registryDB.processLogEntries(_logEntries, _writeResults);
        } while (!_logEntries.empty());

        _logsDB.flush(true);

        _logsDB.getOutgoingMessages(_logsDBOutRequests, _logsDBOutResponses);
        LOG_TRACE(_env, "Sending %s log requests and %s log responses", _logsDBOutRequests.size(), _logsDBOutResponses.size());
        _server.sendLogsDBMessages(_logsDBOutRequests, _logsDBOutResponses);
        

        for (auto &writeResult : _writeResults) {
            auto& regiResp = _registryResponses.emplace_back();
            auto requestIdIt = _logIdxToRequestId.find(writeResult.idx.u64);
            if (requestIdIt == _logIdxToRequestId.end()) {
                _registryResponses.pop_back();
                continue;
            }

            regiResp.requestId = requestIdIt->second;         
            RegistryRespContainer& resp = regiResp.resp;

            if (writeResult.err != TernError::NO_ERROR) {
                resp.setError() = writeResult.err;
                continue;
            }

            switch (writeResult.kind) {
            case RegistryMessageKind::ERROR:
                resp.setError() = writeResult.err;
                break;
            case RegistryMessageKind::CREATE_LOCATION:
                resp.setCreateLocation();
                break;
            case RegistryMessageKind::RENAME_LOCATION:
                resp.setRenameLocation();
                break;
            case RegistryMessageKind::REGISTER_SHARD:
                resp.setRegisterShard();
                break;
            case RegistryMessageKind::REGISTER_CDC:
                resp.setRegisterCdc();
                break;
            case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
                resp.setSetBlockServiceFlags();
                break;
            case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
                resp.setRegisterBlockServices();
                break;
            case RegistryMessageKind::REGISTER_REGISTRY:
                resp.setRegisterRegistry();
                break;
            case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
                resp.setDecommissionBlockService();
                break;
            case RegistryMessageKind::MOVE_SHARD_LEADER:
                resp.setMoveShardLeader();
                break;
            case RegistryMessageKind::CLEAR_SHARD_INFO:
                resp.setClearShardInfo();
                break;
            case RegistryMessageKind::MOVE_CDC_LEADER:
                resp.setMoveCdcLeader();
                break;
            case RegistryMessageKind::CLEAR_CDC_INFO:
                resp.setClearCdcInfo();
                break;
            case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
                resp.setUpdateBlockServicePath();
                break;
            default:
                ALWAYS_ASSERT(false);
                break;
            }
        }
        _writeResults.clear();
        _server.sendRegistryResponses(_registryResponses);
        _registryResponses.clear();
        _clearCaches();
    }
private:
    const RegistryOptions& _options;
    Registerer& _registerer;
    RegistryServer& _server;
    LogsDB& _logsDB;
    RegistryDB& _registryDB;

    std::array<AddrsInfo, LogsDB::REPLICA_COUNT> _replicas;
    std::array<bool, LogsDB::REPLICA_COUNT> _replicaFinishedBootstrap;
    bool _boostrapFinished;

    std::vector<RegistryResponse> _registryResponses;

    // buffer for reading/writing log entries
    std::vector<LogsDBLogEntry> _logEntries;
    std::vector<uint64_t> _entriesRequestIds;

    std::vector<LogsDBRequest*> _logsDBOutRequests;
    std::vector<LogsDBResponse> _logsDBOutResponses;

    // keeping track which log entry corresponds to which request
    std::unordered_map<uint64_t, uint64_t> _logIdxToRequestId;

    // buffer for RegistryDB processLogEntries result
    std::vector<RegistryDBWriteResult> _writeResults;

    // local cache for common data, read once per step

    std::vector<FullRegistryInfo> _cachedRegistries;

    std::vector<FullShardInfo> _cachedShardInfo;
    std::unordered_map<LocationId, std::vector<ShardInfo>> _cachedShardInfoPerLocation;

    std::vector<FullBlockServiceInfo> _cachedBlockServices;
    std::vector<BlockServiceDeprecatedInfo> _cachedAllBlockServices;
    std::unordered_map<BlockServiceId, AES128Key> _decommissionedServices;

    std::vector<CdcInfo> _cachedCdc;

    InfoResp _cachedInfo;

    void _clearCaches() {
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

    void _processBootstrapRequest(RegistryRequest &req) {
        auto& resp = _registryResponses.emplace_back();
        resp.requestId = req.requestId;
        switch (req.req.kind()) {
        case RegistryMessageKind::REGISTER_REGISTRY: {
            const auto &regiReq = req.req.getRegisterRegistry();
            LOG_TRACE(_env, "Received register request in bootstrap %s", regiReq);
            if (regiReq.location != _options.logsDBOptions.location) {
                LOG_TRACE(_env, "Dropping register request in bootstrap for other location");
                break;
            }
            _replicas[regiReq.replicaId.u8] = regiReq.addrs;
            _replicaFinishedBootstrap[regiReq.replicaId.u8] = !regiReq.bootstrap;
            resp.resp.setRegisterRegistry();
            size_t quorumSize = _replicaFinishedBootstrap.size() / 2 + 1;
            if (_options.logsDBOptions.noReplication) {
                quorumSize = 1;
            }

            _boostrapFinished = std::count(
                std::begin(_replicaFinishedBootstrap), std::end(_replicaFinishedBootstrap), true) >= quorumSize;
            if (_boostrapFinished) {
                LOG_TRACE(_env, "Bootstrap finished!");
                _server.setReplicas(_registerer.replicas());
            }
            break;
        }
        case RegistryMessageKind::ALL_REGISTRY_REPLICAS: {
            auto &allRegiResp = resp.resp.setAllRegistryReplicas();
            for (uint8_t i = 0; i < _replicas.size(); ++i) {
                auto &addrs = _replicas[i];
                if (addrs.addrs[0].port == 0 && addrs.addrs[1].port == 0) {
                    continue;
                }
                auto &replica = allRegiResp.replicas.els.emplace_back();
                replica.id = i;
                replica.locationId = _options.logsDBOptions.location;
                replica.addrs = addrs;
                replica.isLeader = !_options.logsDBOptions.avoidBeingLeader;
                replica.lastSeen = ternNow();
            }
            break;
        }
        default:
            // empty response removes client
            break;
        }
    }

    void _populateShardCache() {
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

    const std::vector<FullRegistryInfo>& _registries() {
        if (_cachedRegistries.empty()) {
            _registryDB.registries(_cachedRegistries);
        }
        return _cachedRegistries;
    }

    const std::vector<ShardInfo>& _shardsAtLocation(LocationId location) {
        _populateShardCache();
        return _cachedShardInfoPerLocation[location];
    }

    void _populateCdcCache() {
        if (!_cachedCdc.empty()) {
            return;
        }
        _registryDB.cdcs(_cachedCdc);
    }

    CdcInfo _cdcAtLocation(LocationId location) {
        _populateCdcCache();
        for(const auto& cdc : _cachedCdc) {
            if (cdc.isLeader && cdc.locationId == location) {
                return cdc;
            }
        }
        return CdcInfo{};
    }

    std::vector<AddrsInfo> _cdcReplicas() {
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

    void _populateBlockServiceCache() {
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

    InfoResp _info() {
        _populateBlockServiceCache();
        return _cachedInfo;
    }

    std::vector<BlockServiceDeprecatedInfo> _allBlockServices() {
        _populateBlockServiceCache();
        return _cachedAllBlockServices;
    }

    std::vector<BlockService> _changedBlockServices(LocationId location, TernTime changedSince) {
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

    bool _eraseBlock(const EraseDecommissionedBlockReq& req, BincodeFixedBytes<8>& proof) {
        _populateBlockServiceCache();
        auto bsIt = _decommissionedServices.find(req.blockServiceId);
        if (bsIt == _decommissionedServices.end()) {
            return false;
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
                return false;
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
        return true;
    }
};


Registry::Registry(Logger &logger, std::shared_ptr<XmonAgent> xmon)
    : _env(logger, xmon, "registry") {

      };

Registry::~Registry() {}

void Registry::start(const RegistryOptions& options, LoopThreads& threads) {
    {
        LOG_INFO(_env, "Running registry %s at location %s, base directory %s:", (int)options.logsDBOptions.replicaId, (int)options.logsDBOptions.location, options.logsDBOptions.dbDir);
        LOG_INFO(_env, "  Logging options:");
        LOG_INFO(_env, "    logFile = '%s'", options.logOptions.logFile);
        LOG_INFO(_env, "    xmon = %s", options.xmonOptions.addr);
        LOG_INFO(_env, "    metrics = '%s'", (int)!options.metricsOptions.origin.empty());
        LOG_INFO(_env, "  LogsDB options:");
        LOG_INFO(_env, "    noReplication = '%s'", (int)options.logsDBOptions.noReplication);
        LOG_INFO(_env, "    avoidBeingLeader = '%s'", (int)options.logsDBOptions.avoidBeingLeader);
        LOG_INFO(_env, "  Registry options:");
        LOG_INFO(_env, "    registryHost = '%s'", options.registryClientOptions.host);
        LOG_INFO(_env, "    registryPort = %s", options.registryClientOptions.port);
        LOG_INFO(_env, "    ownAddres = %s", options.serverOptions.addrs);
        LOG_INFO(_env, "    enforceStableIp = '%s'", (int)options.enforceStableIp);
        LOG_INFO(_env, "    enforceStableLeader = '%s'", (int)options.enforceStableLeader);
        LOG_INFO(_env, "    maxConnections = '%s'", options.maxConnections);
        LOG_INFO(_env, "    minAutoDecomInterval = '%s'", options.minDecomInterval);
        LOG_INFO(_env, "    alertAtUnavailableFailureDomains = '%s'", (int)options.alertAfterUnavailableFailureDomains);
        LOG_INFO(_env, "    stalenessDelay = '%s'", options.staleDelay);
        LOG_INFO(_env, "    blockServiceUseDelay = '%s'", options.blockServiceUsageDelay);
        LOG_INFO(_env, "    maxWritableBlockServicePerShard = '%s'", options.maxFailureDomainsPerShard);
        LOG_INFO(_env, "    writableBlockServiceUpdateInterval = '%s'", options.writableBlockServiceUpdateInterval);
    }
    
    _state = std::make_unique<RegistryState>();
    std::string dbDir = options.logsDBOptions.dbDir;
    XmonNCAlert dbInitAlert{1_mins};
    _env.updateAlert(dbInitAlert, "initializing database %s", options.xmonOptions.appInstance);
    auto xmon = _env.xmon();
    auto& logger = _env.logger();
    _state->sharedDB = std::make_unique<SharedRocksDB>(logger, xmon, dbDir + "/db", dbDir + "/db-statistics.txt");
    _state->sharedDB->registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
    _state->sharedDB->registerCFDescriptors(RegistryDB::getColumnFamilyDescriptors());

    rocksdb::Options rocksDBOptions;
    rocksDBOptions.create_if_missing = true;
    rocksDBOptions.create_missing_column_families = true;
    rocksDBOptions.compression = rocksdb::kLZ4Compression;
    rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
    rocksDBOptions.max_open_files = 1000;
    // We batch writes and flush manually.
    rocksDBOptions.manual_wal_flush = true;
    _state->sharedDB->open(rocksDBOptions);
    _state->registryDB = std::make_unique<RegistryDB>(logger, xmon, options, *_state->sharedDB);
    _state->logsDB = std::make_unique<LogsDB>(
        logger, xmon, *_state->sharedDB, options.logsDBOptions.replicaId,
        _state->registryDB->lastAppliedLogEntry(), options.logsDBOptions.noReplication,
        options.logsDBOptions.avoidBeingLeader);
    _env.clearAlert(dbInitAlert);

    _state->server.reset(new RegistryServer(options, _env));
    ALWAYS_ASSERT(_state->server->init());
    std::vector<FullRegistryInfo> cachedRegistries;
    _state->registryDB->registries(cachedRegistries);

    auto registerer = std::make_unique<Registerer>(logger, xmon, options, _state->server->boundAddresses(), cachedRegistries);
    auto registryLoop = std::make_unique<RegistryLoop>(logger, xmon, options, *registerer, *_state->server, *_state->logsDB, *_state->registryDB);
    
    threads.emplace_back(LoopThread::Spawn(std::move((registerer))));
    threads.emplace_back(LoopThread::Spawn(std::move((registryLoop))));
}

void Registry::close() {
  if (_state) {
    if (_state->registryDB) {
      _state->registryDB->close();
      _state->registryDB.reset();
    }
    if (_state->logsDB) {
      _state->logsDB->close();
      _state->logsDB.reset();
    }
    if (_state->sharedDB) {
      _state->sharedDB->close();
      _state->sharedDB.reset();
    }

    LOG_INFO(_env, "registry terminating gracefully, bye.");
  }
  _state.reset();
}
