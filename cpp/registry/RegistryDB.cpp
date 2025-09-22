// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "RegistryDB.hpp"
#include "Assert.hpp"
#include "Bincode.hpp"
#include "Env.hpp"
#include "LogsDB.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "RegistryDBData.hpp"
#include "RocksDBUtils.hpp"
#include "Time.hpp"
#include <rocksdb/write_batch.h>

static constexpr auto REGISTRY_CF_NAME = "registry";
static constexpr auto LOCATIONS_CF_NAME = "locations";
static constexpr auto SHARDS_CF_NAME = "shards";
static constexpr auto CDC_CF_NAME = "cdc";
static constexpr auto BLOCKSERVICES_CF_NAME = "blockservices";
static constexpr auto LAST_SERVICE_HEARTBEAT_CF_NAME = "last_heartbeat";
static constexpr auto WRITABLE_BLOCKSERVICES_CF_NAME = "writable_blockservices";

std::vector<rocksdb::ColumnFamilyDescriptor>
RegistryDB::getColumnFamilyDescriptors() {
    return {
        {rocksdb::kDefaultColumnFamilyName, {}},
        {REGISTRY_CF_NAME, {}},
        {LOCATIONS_CF_NAME, {}},
        {SHARDS_CF_NAME, {}},
        {CDC_CF_NAME, {}},
        {BLOCKSERVICES_CF_NAME, {}},
        {LAST_SERVICE_HEARTBEAT_CF_NAME, {}},
        {WRITABLE_BLOCKSERVICES_CF_NAME, {}},
    };
}

RegistryDB::RegistryDB(Logger &logger, std::shared_ptr<XmonAgent> &xmon, const RegistryOptions& options, const SharedRocksDB &sharedDB) :
    _options(options), _env(logger, xmon, "RegistryDB"),
    _lastCalculatedShardBlockServices(0),
     _db(sharedDB.db()),
    _defaultCf(sharedDB.getCF(rocksdb::kDefaultColumnFamilyName)),
    _registryCf(sharedDB.getCF(REGISTRY_CF_NAME)),
    _locationsCf(sharedDB.getCF(LOCATIONS_CF_NAME)),
    _shardsCf(sharedDB.getCF(SHARDS_CF_NAME)),
    _cdcCf(sharedDB.getCF(CDC_CF_NAME)),
    _blockServicesCf(sharedDB.getCF(BLOCKSERVICES_CF_NAME)),
    _lastHeartBeatCf(sharedDB.getCF(LAST_SERVICE_HEARTBEAT_CF_NAME)),
    _writableBlockServicesCf(sharedDB.getCF(WRITABLE_BLOCKSERVICES_CF_NAME))
{
    LOG_INFO(_env, "opening Registry RocksDB");
    _initDb();
}

void RegistryDB::close() {
    LOG_INFO(_env, "closing RegistryDB, lastAppliedLogEntry(%s)", lastAppliedLogEntry());
}

struct ReloadState {
    bool locations;
    bool registry;
    bool shards;
    bool cdc;
    bool blockServices;
    ReloadState() : locations(false), registry(false), shards(false), cdc(false), blockServices(false) {}
};

static bool addressesIntersect(const AddrsInfo& currentAddr, const AddrsInfo& newAddr) {
    if (currentAddr.addrs[0].ip.data[0] == 0 && currentAddr.addrs[1].ip.data[0] == 0) {
        // we allow address to be set if current is empty
        return true;
    }
    for (auto& cur : currentAddr.addrs) {
        if (cur.ip.data[0] == 0) {
            continue;
        }
        for(auto& n : newAddr.addrs) {
            if (cur.ip == n.ip) {
                return true;
            }
        }
    }
    return false;
}

void RegistryDB::processLogEntries(std::vector<LogsDBLogEntry>& logEntries, std::vector<RegistryDBWriteResult>& writeResults) {
    auto expectedLogEntry = lastAppliedLogEntry();
    std::unordered_map<uint64_t, FullBlockServiceInfo> updatedBlocks;
    bool writableChanged = false;
    rocksdb::WriteBatch writeBatch;
    ReloadState toReload{};
    TernTime lastRequestTime{};
    for (auto &entry : logEntries) {
        ALWAYS_ASSERT(entry.idx == ++expectedLogEntry, "log entry index mismatch");
        BincodeBuf buf((char*)entry.value.data(), entry.value.size());
        TernTime requestTime = buf.unpackScalar<uint64_t>();
        lastRequestTime = std::max(lastRequestTime, requestTime);
        RegistryReqContainer reqContainer;
        reqContainer.unpack(buf);
        buf.ensureFinished();
        auto& res = writeResults.emplace_back();
        res.idx = entry.idx;
        res.kind = reqContainer.kind();
        res.err = TernError::NO_ERROR;
        switch (res.kind) {
        case RegistryMessageKind::CREATE_LOCATION: {
            auto& req = reqContainer.getCreateLocation();
            StaticValue<LocationInfoKey> key;
            std::string value;
            key().setLocationId(req.id);
            auto status = _db->Get({}, _locationsCf, key.toSlice(), &value);
            if (status != rocksdb::Status::NotFound()) {
                ROCKS_DB_CHECKED(status);
                res.err = TernError::LOCATION_EXISTS;
                break;
            }
            LocationInfo newLocation;
            newLocation.id = req.id;
            newLocation.name = req.name;
            writeLocationInfo(writeBatch, _locationsCf, newLocation);
            initializeShardsForLocation(writeBatch, _shardsCf, req.id);
            initializeCdcForLocation(writeBatch, _cdcCf, req.id);
            toReload.locations = toReload.registry = toReload.shards = toReload.cdc = true;
            break;
        }
        case RegistryMessageKind::RENAME_LOCATION: {
            auto& req = reqContainer.getRenameLocation();
            StaticValue<LocationInfoKey> key;
            std::string value;
            key().setLocationId(req.id);
            auto status = _db->Get({}, _locationsCf, key.toSlice(), &value);
            if (status == rocksdb::Status::NotFound()) {
                LOG_DEBUG(_env, "unknown location in req %s", req);
                res.err = TernError::LOCATION_NOT_FOUND;
                break;
            }
            ROCKS_DB_CHECKED(status);
            LocationInfo newLocation;
            newLocation.id = req.id;
            newLocation.name = req.name;
            writeLocationInfo(writeBatch, _locationsCf, newLocation);
            toReload.locations = true;
            break;
        }
        case RegistryMessageKind::REGISTER_SHARD: {
            auto& req = reqContainer.getRegisterShard();
            StaticValue<ShardInfoKey> key;
            std::string value;
            key().setLocationId(req.location);
            key().setReplicaId(req.shrid.replicaId());
            key().setShardId(req.shrid.shardId().u8);
            auto status = _db->Get({}, _shardsCf, key.toSlice(), &value);
            if (status == rocksdb::Status::NotFound()) {
                LOG_DEBUG(_env, "unknown shard replica in req %s", req);
                res.err = TernError::INVALID_REPLICA;
                break;
            }
            ROCKS_DB_CHECKED(status);
            FullShardInfo info;
            readShardInfo(key.toSlice(), value, info);
            if (_options.enforceStableIp && (!addressesIntersect(info.addrs, req.addrs))) {
                res.err = TernError::DIFFERENT_ADDRS_INFO;
                break;
            }
            if (_options.enforceStableLeader && info.isLeader != req.isLeader) {
                res.err = TernError::LEADER_PREEMPTED;
                break;
            }
            StaticValue<LastHeartBeatKey> lastHeartBeat;
            shardToLastHeartBeat(info, lastHeartBeat());
            writeBatch.Delete(_lastHeartBeatCf, lastHeartBeat.toSlice());
            info.isLeader = req.isLeader;
            info.addrs = req.addrs;
            info.lastSeen = requestTime;
            shardToLastHeartBeat(info, lastHeartBeat());
            writeBatch.Put(_lastHeartBeatCf, lastHeartBeat.toSlice(),{});
            writeShardInfo(writeBatch, _shardsCf, info);
            break;
        }
        case RegistryMessageKind::REGISTER_CDC:  {
            auto& req = reqContainer.getRegisterCdc();
            StaticValue<CdcInfoKey> key;
            std::string value;
            key().setLocationId(req.location);
            key().setReplicaId(req.replica);
            auto status = _db->Get({}, _cdcCf, key.toSlice(), &value);
            if (status == rocksdb::Status::NotFound()) {
                LOG_DEBUG(_env, "unknown cdc replica in req %s", req);
                res.err = TernError::INVALID_REPLICA;
                break;
            }
            ROCKS_DB_CHECKED(status);
            CdcInfo info;
            readCdcInfo(key.toSlice(), value, info);
            if (_options.enforceStableIp && (!addressesIntersect(info.addrs, req.addrs))) {
                res.err = TernError::DIFFERENT_ADDRS_INFO;
                break;
            }
            if (_options.enforceStableLeader && info.isLeader != req.isLeader) {
                res.err = TernError::LEADER_PREEMPTED;
                break;
            }
            StaticValue<LastHeartBeatKey> lastHeartBeat;
            cdcToLastHeartBeat(info, lastHeartBeat());
            writeBatch.Delete(_lastHeartBeatCf, lastHeartBeat.toSlice());
            info.isLeader = req.isLeader;
            info.addrs = req.addrs;
            info.lastSeen = requestTime;
            cdcToLastHeartBeat(info, lastHeartBeat());
            writeBatch.Put(_lastHeartBeatCf, lastHeartBeat.toSlice(),{});
            writeCdcInfo(writeBatch, _cdcCf, info);
            break;
        }
        case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS: {
            auto& req = reqContainer.getSetBlockServiceFlags();
            auto bsIt = updatedBlocks.find(req.id.u64);
            FullBlockServiceInfo info;
            StaticValue<BlockServiceInfoKey> key;
            key().setId(req.id.u64);
            if (bsIt == updatedBlocks.end()) {
                std::string value;
                auto status = _db->Get({}, _blockServicesCf, key.toSlice(), &value);
                if (status == rocksdb::Status::NotFound()) {
                    LOG_DEBUG(_env, "unknown block service in req %s", req);
                    res.err = TernError::BLOCK_SERVICE_NOT_FOUND;
                    break;
                }
                ROCKS_DB_CHECKED(status);
                readBlockServiceInfo(key.toSlice(), value, info);
            } else {
                info = bsIt->second;
            }
            if ((info.flags & BlockServiceFlags::DECOMMISSIONED) != BlockServiceFlags::EMPTY) {
                // no updates allowed to decomissioned services do nothing
                break;
            }
            if ((req.flags & BlockServiceFlags::DECOMMISSIONED & (BlockServiceFlags) req.flagsMask) != BlockServiceFlags::EMPTY) {
                // no longer track staleness
                StaticValue<LastHeartBeatKey> lastHeartBeat;
                blockServiceToLastHeartBeat(info, lastHeartBeat());
                writeBatch.Delete(_lastHeartBeatCf, lastHeartBeat.toSlice());
                // DECOMMISSIONED service looses other flags
                info.flags = BlockServiceFlags::DECOMMISSIONED;
            } else {
                info.flags = (info.flags & ~(BlockServiceFlags)req.flagsMask) | (req.flags & (BlockServiceFlags) req.flagsMask);
            }
            info.lastInfoChange = requestTime;
            writeBlockServiceInfo(writeBatch, _blockServicesCf, info);
            updatedBlocks[info.id.u64] = info;
            break;
        }
        case RegistryMessageKind::REGISTER_BLOCK_SERVICES: {
            auto& req = reqContainer.getRegisterBlockServices();
            for(auto& newInfo : req.blockServices.els) {
                auto bsIt = updatedBlocks.find(newInfo.id.u64);
                FullBlockServiceInfo info;
                bool newService = false;
                StaticValue<BlockServiceInfoKey> key;
                key().setId(newInfo.id.u64);
                if (bsIt == updatedBlocks.end()) {
                    std::string value;
                    auto status = _db->Get({}, _blockServicesCf, key.toSlice(), &value);
                    if (status == rocksdb::Status::NotFound()) {
                        info.id = newInfo.id;
                        info.locationId = newInfo.locationId;
                        info.addrs = newInfo.addrs;
                        info.storageClass = newInfo.storageClass;
                        info.failureDomain = newInfo.failureDomain;
                        info.secretKey = newInfo.secretKey;
                        info.flags = BlockServiceFlags::EMPTY;
                        info.firstSeen = info.lastInfoChange = requestTime;
                        info.hasFiles = false;
                        info.path = newInfo.path;
                        newService = true;
                    } else {
                        ROCKS_DB_CHECKED(status);
                        readBlockServiceInfo(key.toSlice(), value, info);
                    }
                } else {
                    info = bsIt->second;
                }
                if (info.flags == BlockServiceFlags::DECOMMISSIONED) {
                    // no updates to decommissioned services
                    continue;
                }
                StaticValue<WritableBlockServiceKey> writableKey;
                StaticValue<LastHeartBeatKey> lastHeartBeat;
                bool wasWritable = false;
                if (!newService) {
                    if (isWritable(info.flags) && info.availableBytes > 0) {
                        wasWritable = true;
                        blockServiceToWritableBlockServiceKey(info, writableKey());
                        writeBatch.Delete(_writableBlockServicesCf, writableKey.toSlice());
                    }
                    blockServiceToLastHeartBeat(info, lastHeartBeat());
                    writeBatch.Delete(_lastHeartBeatCf, lastHeartBeat.toSlice());
                }
                if(info.addrs != newInfo.addrs) {
                    info.addrs = newInfo.addrs;
                    info.lastInfoChange = requestTime;
                }
                if ((info.flags & BlockServiceFlags::STALE) != BlockServiceFlags::EMPTY ) {
                    info.flags = info.flags & ~BlockServiceFlags::STALE;
                    info.lastInfoChange = requestTime;
                }
                info.lastSeen = requestTime;
                info.capacityBytes = newInfo.capacityBytes;
                info.availableBytes = newInfo.availableBytes;
                info.blocks = newInfo.blocks;
                bool nowWritable = false;
                if (isWritable(info.flags) && info.availableBytes > 0 && (requestTime - info.firstSeen >= _options.blockServiceUsageDelay)) {
                    nowWritable = true;
                    blockServiceToWritableBlockServiceKey(info, writableKey());
                    writeBatch.Put(_writableBlockServicesCf, writableKey.toSlice(), {});
                }
                if (wasWritable != nowWritable) {
                    info.lastInfoChange = requestTime;
                    writableChanged = true;
                }
                blockServiceToLastHeartBeat(info, lastHeartBeat());
                writeBatch.Put(_lastHeartBeatCf, lastHeartBeat.toSlice(),{});
                writeBlockServiceInfo(writeBatch, _blockServicesCf, info);
                updatedBlocks[info.id.u64] = info;
            }
            break;
        }
        case RegistryMessageKind::REGISTER_REGISTRY: {
            auto& req = reqContainer.getRegisterRegistry();

            StaticValue<RegistryReplicaInfoKey> key;
            std::string value;
            key().setReplicaId(req.replicaId);
            key().setLocationId(req.location);
            auto status = _db->Get({}, _registryCf, key.toSlice(), &value);
            if (status == rocksdb::Status::NotFound()) {
                LOG_ERROR(_env, "unknown registry replica in req %s", req);
                res.err = TernError::INVALID_REPLICA;
                break;
            }
            ROCKS_DB_CHECKED(status);
            FullRegistryInfo info;
            readRegistryInfo(key.toSlice(), value, info);
            if (_options.enforceStableIp && (!addressesIntersect(info.addrs, req.addrs))) {
                res.err = TernError::DIFFERENT_ADDRS_INFO;
                break;
            }
            if (_options.enforceStableLeader && info.isLeader != req.isLeader) {
                res.err = TernError::LEADER_PREEMPTED;
                break;
            }
            StaticValue<LastHeartBeatKey> lastHeartBeat;
            registryToLastHeartBeat(info, lastHeartBeat());
            writeBatch.Delete(_lastHeartBeatCf, lastHeartBeat.toSlice());
            info.isLeader = req.isLeader;
            info.addrs = req.addrs;
            info.lastSeen = requestTime;
            registryToLastHeartBeat(info, lastHeartBeat());
            writeBatch.Put(_lastHeartBeatCf, lastHeartBeat.toSlice(),{});
            writeRegistryInfo(writeBatch, _registryCf, info);
            break;
        }
        case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE: {
            auto& req = reqContainer.getDecommissionBlockService();
            auto bsIt = updatedBlocks.find(req.id.u64);
            FullBlockServiceInfo info;
            StaticValue<BlockServiceInfoKey> key;
            key().setId(req.id.u64);
            if (bsIt == updatedBlocks.end()) {
                std::string value;
                auto status = _db->Get({}, _blockServicesCf, key.toSlice(), &value);
                if (status == rocksdb::Status::NotFound()) {
                    LOG_ERROR(_env, "unknown block service in req %s", req);
                    res.err = TernError::BLOCK_SERVICE_NOT_FOUND;
                    break;
                }
                ROCKS_DB_CHECKED(status);
                readBlockServiceInfo(key.toSlice(), value, info);
            } else {
                info = bsIt->second;
            }
            if ((info.flags & BlockServiceFlags::DECOMMISSIONED) != BlockServiceFlags::EMPTY) {
                // no updates allowed to decomissioned services do nothing
                break;
            }
            if (isWritable(info.flags) && info.availableBytes > 0) {
                StaticValue<WritableBlockServiceKey> writableKey;
                writableChanged = true;
                blockServiceToWritableBlockServiceKey(info, writableKey());
                writeBatch.Delete(_writableBlockServicesCf, writableKey.toSlice());
            }
            // no longer track staleness
            StaticValue<LastHeartBeatKey> lastHeartBeat;
            blockServiceToLastHeartBeat(info, lastHeartBeat());
            writeBatch.Delete(_lastHeartBeatCf, lastHeartBeat.toSlice());

            // DECOMMISSIONED service looses other flags
            info.flags = BlockServiceFlags::DECOMMISSIONED;
            info.lastInfoChange = requestTime;
            writeBlockServiceInfo(writeBatch, _blockServicesCf, info);
            updatedBlocks[info.id.u64] = info;
            break;
        }
        case RegistryMessageKind::MOVE_SHARD_LEADER: {
            auto& req = reqContainer.getMoveShardLeader();
            StaticValue<ShardInfoKey> key;
            std::string value;
            key().setLocationId(req.location);
            key().setReplicaId(req.shrid.replicaId());
            key().setShardId(req.shrid.shardId().u8);
            auto status = _db->Get({}, _shardsCf, key.toSlice(), &value);
            if (status == rocksdb::Status::NotFound()) {
                LOG_ERROR(_env, "unknown shard replica in req %s", req);
                res.err = TernError::INVALID_REPLICA;
                break;
            }
            ROCKS_DB_CHECKED(status);
            FullShardInfo info;
            readShardInfo(key.toSlice(), value, info);
            info.isLeader = true;
            writeShardInfo(writeBatch, _shardsCf, info);
            break;
        }
        case RegistryMessageKind::CLEAR_SHARD_INFO: {
            auto& req = reqContainer.getClearShardInfo();
            StaticValue<ShardInfoKey> key;
            std::string value;
            key().setLocationId(req.location);
            key().setReplicaId(req.shrid.replicaId());
            key().setShardId(req.shrid.shardId().u8);
            auto status = _db->Get({}, _shardsCf, key.toSlice(), &value);
            if (status == rocksdb::Status::NotFound()) {
                LOG_ERROR(_env, "unknown shard replica in req %s", req);
                res.err = TernError::INVALID_REPLICA;
                break;
            }
            ROCKS_DB_CHECKED(status);
            FullShardInfo info;
            readShardInfo(key.toSlice(), value, info);
            info.isLeader = false;
            info.addrs.clear();
            writeShardInfo(writeBatch, _shardsCf, info);
            break;
        }
        case RegistryMessageKind::MOVE_CDC_LEADER: {
            auto& req = reqContainer.getMoveCdcLeader();
            StaticValue<CdcInfoKey> key;
            std::string value;
            key().setLocationId(req.location);
            key().setReplicaId(req.replica);
            auto status = _db->Get({}, _cdcCf, key.toSlice(), &value);
            if (status == rocksdb::Status::NotFound()) {
                LOG_ERROR(_env, "unknown cdc replica in req %s", req);
                res.err = TernError::INVALID_REPLICA;
                break;
            }
            ROCKS_DB_CHECKED(status);
            CdcInfo info;
            readCdcInfo(key.toSlice(), value, info);
            info.isLeader = true;
            writeCdcInfo(writeBatch, _cdcCf, info);
            break;
        }
        case RegistryMessageKind::CLEAR_CDC_INFO: {
            auto& req = reqContainer.getClearCdcInfo();
            StaticValue<CdcInfoKey> key;
            std::string value;
            key().setLocationId(req.location);
            key().setReplicaId(req.replica);
            auto status = _db->Get({}, _cdcCf, key.toSlice(), &value);
            if (status == rocksdb::Status::NotFound()) {
                LOG_ERROR(_env, "unknown cdc replica in req %s", req);
                res.err = TernError::INVALID_REPLICA;
                break;
            }
            ROCKS_DB_CHECKED(status);
            CdcInfo info;
            readCdcInfo(key.toSlice(), value, info);
            info.isLeader = false;
            info.addrs.clear();
            writeCdcInfo(writeBatch, _cdcCf, info);
            break;
        }
        case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH: {
            auto& req = reqContainer.getUpdateBlockServicePath();
            auto bsIt = updatedBlocks.find(req.id.u64);
            FullBlockServiceInfo info;
            StaticValue<BlockServiceInfoKey> key;
            key().setId(req.id.u64);
            if (bsIt == updatedBlocks.end()) {
                std::string value;
                auto status = _db->Get({}, _blockServicesCf, key.toSlice(), &value);
                if (status == rocksdb::Status::NotFound()) {
                    LOG_ERROR(_env, "unknown block service in req %s", req);
                    res.err = TernError::BLOCK_SERVICE_NOT_FOUND;
                    break;
                }
                ROCKS_DB_CHECKED(status);
                readBlockServiceInfo(key.toSlice(), value, info);
            } else {
                info = bsIt->second;
            }
            info.path = req.newPath;
            writeBlockServiceInfo(writeBatch, _blockServicesCf, info);
            updatedBlocks[info.id.u64] = info;
            break;
        }
        default:
            ALWAYS_ASSERT(false);
        }
    }
    writeLastAppliedLogEntry(writeBatch, _defaultCf, expectedLogEntry);
    ROCKS_DB_CHECKED(_db->Write({}, &writeBatch));
    writableChanged = _updateStaleBlockServices(lastRequestTime) || writableChanged;
    _recalcualteShardBlockServices(writableChanged);
}

void RegistryDB::_initDb() {
    const auto keyExists =
        [this](rocksdb::ColumnFamilyHandle *cf, const rocksdb::Slice &key) -> bool {
            std::string value;
            auto status = _db->Get({}, cf, key, &value);
            if (status.IsNotFound()) {
            return false;
            } else {
            ROCKS_DB_CHECKED(status);
            return true;
            }
        };

    if (!keyExists(_defaultCf, registryMetadataKey(&LAST_APPLIED_LOG_ENTRY_KEY))) {
        LOG_INFO(_env, "initializing Registry RocksDB");
        rocksdb::WriteBatch batch;
        LOG_INFO(_env, "initializing last applied log entry");
        writeLastAppliedLogEntry(batch, _defaultCf, LogIdx(0));
        
        LOG_INFO(_env, "initializing default location");
        LocationInfo defaultLocation;
        defaultLocation.id = DEFAULT_LOCATION;
        defaultLocation.name = "default";
        writeLocationInfo(batch, _locationsCf, defaultLocation);
        
        LOG_INFO(_env, "initializing registries");
        initializeRegistries(batch, _registryCf);
        
        LOG_INFO(_env, "initializing shards for default location");
        initializeShardsForLocation(batch, _shardsCf, defaultLocation.id);

        LOG_INFO(_env, "initializing cdc for default location");
        initializeCdcForLocation(batch, _cdcCf, defaultLocation.id);

        ROCKS_DB_CHECKED(_db->Write({}, &batch));
        LOG_INFO(_env, "initialized Registry RocksDB");
    }
}

void RegistryDB::_recalcualteShardBlockServices(bool writableChanged) {
    if (!writableChanged && (ternNow() - _lastCalculatedShardBlockServices < _options.writableBlockServiceUpdateInterval)) {
        return;
    }
    _lastCalculatedShardBlockServices = ternNow();
    for(int i = 0; i < 256; ++i) {
        _shardBlockServices[(uint8_t) i].clear();
    }

    auto *it = _db->NewIterator(rocksdb::ReadOptions(), _writableBlockServicesCf);
    BincodeFixedBytes<16> lastFd;
    lastFd.clear();
    uint8_t lastStorageClass = 0;
    for (it->SeekToFirst(); it->Valid(); it->Next()) {
        auto writableKey = ExternalValue<WritableBlockServiceKey>::FromSlice(it->key());
        if (lastFd == writableKey().failureDomain() && lastStorageClass == writableKey().storageClass()) {
            continue;
        }
        lastFd = writableKey().failureDomain();
        lastStorageClass = writableKey().storageClass();
        auto id = writableKey().id();
        StaticValue<BlockServiceInfoKey> key;
        key().setId(id);
        std::string value;
        auto status = _db->Get({}, _blockServicesCf, key.toSlice(), &value);
        ROCKS_DB_CHECKED(status);
        FullBlockServiceInfo info;
        readBlockServiceInfo(key.toSlice(), value, info);
        BlockServiceInfoShort bsShort;
        bsShort.id = id;
        bsShort.locationId = info.locationId;
        bsShort.failureDomain = info.failureDomain;
        bsShort.storageClass = info.storageClass;
        for(int i = 0; i < 256; ++i) {
            _shardBlockServices[(uint8_t) i].push_back(bsShort);
        }
    }
    ROCKS_DB_CHECKED(it->status());
    delete it;
}

bool RegistryDB::_updateStaleBlockServices(TernTime now) {
    rocksdb::WriteBatch writeBatch;
    bool writableChanged = false;
    auto *it = _db->NewIterator(rocksdb::ReadOptions(), _lastHeartBeatCf);
    for (it->SeekToFirst(); it->Valid(); it->Next()) {
        auto lastHeartBeat = ExternalValue<LastHeartBeatKey>::FromSlice(it->key());
        if (TernTime(lastHeartBeat().lastSeen()) + _options.staleDelay > now) {
            break;
        }
        if (lastHeartBeat()._serviceType() != ServiceType::BLOCK_SERVICE) {
            continue;
        }
        auto serviceBytes = lastHeartBeat()._serviceBytes();
        auto bsKey = ExternalValue<BlockServiceInfoKey>::FromSlice({(char*)serviceBytes.data(), BlockServiceInfoKey::MAX_SIZE});
        writeBatch.Delete(_lastHeartBeatCf, it->key());
        std::string value;
        auto status = _db->Get({}, _blockServicesCf, bsKey.toSlice(), &value);
        ROCKS_DB_CHECKED(status);
        FullBlockServiceInfo info;
        readBlockServiceInfo(bsKey.toSlice(), value, info);
        StaticValue<WritableBlockServiceKey> writableKey;
        if (isWritable(info.flags) && info.availableBytes > 0) {
            writableChanged = true;
            blockServiceToWritableBlockServiceKey(info, writableKey());
            writeBatch.Delete(_writableBlockServicesCf, writableKey.toSlice());
        }
        info.flags = info.flags | BlockServiceFlags::STALE;
        writeBlockServiceInfo(writeBatch, _blockServicesCf, info); 
    }
    ROCKS_DB_CHECKED(it->status());
    delete it;
    ROCKS_DB_CHECKED(_db->Write({}, &writeBatch));
    return writableChanged;
}

LogIdx RegistryDB::lastAppliedLogEntry() const {
    return readLastAppliedLogEntry(_db, _defaultCf);
}

void RegistryDB::registries(std::vector<FullRegistryInfo>& out) const {
    out.clear();
    auto *it = _db->NewIterator(rocksdb::ReadOptions(), _registryCf);
    for (it->SeekToFirst(); it->Valid(); it->Next()) {
        readRegistryInfo(it->key(), it->value(), out.emplace_back());
    }
    ROCKS_DB_CHECKED(it->status());
    delete it;
    ALWAYS_ASSERT(out.size() == LogsDB::REPLICA_COUNT);
}

void RegistryDB::locations(std::vector<LocationInfo>& out) const {
    out.clear();
    auto *it = _db->NewIterator(rocksdb::ReadOptions(), _locationsCf);
    for (it->SeekToFirst(); it->Valid(); it->Next()) {
        readLocationInfo(it->key(), it->value(), out.emplace_back());
    }
    ROCKS_DB_CHECKED(it->status());
    delete it;
    ALWAYS_ASSERT(!out.empty());
}

void RegistryDB::shards(std::vector<FullShardInfo>& out) const {
    out.clear();
    auto *it = _db->NewIterator(rocksdb::ReadOptions(), _shardsCf);
    for (it->SeekToFirst(); it->Valid(); it->Next()) {
        auto& shard = out.emplace_back();
        readShardInfo(it->key(), it->value(), shard);
    }
    ROCKS_DB_CHECKED(it->status());
    delete it;
    ALWAYS_ASSERT((!out.empty()) && (out.size() % LogsDB::REPLICA_COUNT * ShardId::SHARD_COUNT == 0));
}

void RegistryDB::cdcs(std::vector<CdcInfo>& out) const {
    out.clear();
    auto *it = _db->NewIterator(rocksdb::ReadOptions(), _cdcCf);
    for (it->SeekToFirst(); it->Valid(); it->Next()) {
        readCdcInfo(it->key(), it->value(), out.emplace_back());
    }
    ROCKS_DB_CHECKED(it->status());
    delete it;
    ALWAYS_ASSERT((!out.empty()) && (out.size() % LogsDB::REPLICA_COUNT == 0));
}

void RegistryDB::blockServices(std::vector<FullBlockServiceInfo>& out) const{
    out.clear();
    auto *it = _db->NewIterator(rocksdb::ReadOptions(), _blockServicesCf);
    for (it->SeekToFirst(); it->Valid(); it->Next()) {
        readBlockServiceInfo(it->key(), it->value(), out.emplace_back());
    }
    ROCKS_DB_CHECKED(it->status());
    delete it;
}
