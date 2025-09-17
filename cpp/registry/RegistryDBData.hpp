// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <cstdint>

#include <rocksdb/slice.h>

#include "Assert.hpp"
#include "LogsDB.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "RocksDBUtils.hpp"
#include "Time.hpp"

enum class RegistryMetadataKey : uint8_t {
    LAST_APPLIED_LOG_ENTRY = 0,
};

constexpr RegistryMetadataKey LAST_APPLIED_LOG_ENTRY_KEY = RegistryMetadataKey::LAST_APPLIED_LOG_ENTRY;

inline rocksdb::Slice registryMetadataKey(const RegistryMetadataKey* k) {
    return rocksdb::Slice((const char*)k, sizeof(*k));
}

static inline LogIdx readLastAppliedLogEntry(rocksdb::DB* db, rocksdb::ColumnFamilyHandle* cf) {
    std::string value;
    ROCKS_DB_CHECKED(db->Get({}, cf,
        registryMetadataKey(&LAST_APPLIED_LOG_ENTRY_KEY), &value));
    return LogIdx(ExternalValue<U64Value>(value)().u64());
}

static inline void writeLastAppliedLogEntry(rocksdb::WriteBatch& batch, rocksdb::ColumnFamilyHandle* cf, LogIdx lastAppliedLogEntry) {
    auto v = U64Value::Static(lastAppliedLogEntry.u64);
    ROCKS_DB_CHECKED(
        batch.Put(cf, registryMetadataKey(&LAST_APPLIED_LOG_ENTRY_KEY), v.toSlice())
    );
}

struct LocationInfoKey {
    FIELDS(
        LE, uint8_t, locationId, setLocationId,
        END_STATIC
    )
};

static inline void readLocationInfo(rocksdb::Slice key, rocksdb::Slice value, LocationInfo& locationInfo) {
    locationInfo.id = ExternalValue<LocationInfoKey>::FromSlice(key)().locationId();
    locationInfo.name.copy(value.data(), value.size());
}

static inline void writeLocationInfo(rocksdb::WriteBatch& batch, rocksdb::ColumnFamilyHandle* cf, const LocationInfo& locationInfo) {
    StaticValue<LocationInfoKey> key;
    key().setLocationId(locationInfo.id);
    ROCKS_DB_CHECKED(
        batch.Put(cf, key.toSlice(), rocksdb::Slice(locationInfo.name.data(), locationInfo.name.size()))
    );
}

struct RegistryReplicaInfoKey {
    FIELDS(
        LE, uint8_t, locationId, setLocationId,
        LE, ReplicaId, replicaId, setReplicaId,
        END_STATIC
    )
};

struct RegistryReplicaInfoBody {
    FIELDS(
        LE, uint8_t,  version, _setVersion,
        LE, bool, isLeader, setIsLeader,
        LE, TernTime, lastSeen, setLastSeen,
        FBYTES, 4, ip1, setIp1,
        BE, uint16_t, port1, setPort1,
        FBYTES, 4, ip2, setIp2,
        BE, uint16_t, port2, setPort2,
        END_STATIC
    )
};

static inline void readRegistryInfo(rocksdb::Slice key_, rocksdb::Slice value_, FullRegistryInfo& registryInfo) {
    auto key = ExternalValue<RegistryReplicaInfoKey>::FromSlice(key_);
    auto value = ExternalValue<RegistryReplicaInfoBody>::FromSlice(value_);
    registryInfo.id = key().replicaId();
    registryInfo.locationId = key().locationId();
    registryInfo.isLeader = value().isLeader();
    registryInfo.lastSeen = value().lastSeen();
    registryInfo.addrs.addrs[0].ip = value().ip1();
    registryInfo.addrs.addrs[0].port = value().port1();
    registryInfo.addrs.addrs[1].ip = value().ip2();
    registryInfo.addrs.addrs[1].port = value().port2();
}

static inline void writeRegistryInfo(rocksdb::WriteBatch& batch, rocksdb::ColumnFamilyHandle* cf, const FullRegistryInfo& registryInfo) {
    StaticValue<RegistryReplicaInfoKey> key;
    StaticValue<RegistryReplicaInfoBody> value;
    key().setReplicaId(registryInfo.id);
    key().setLocationId(registryInfo.locationId);
    value()._setVersion(0);
    value().setIsLeader(registryInfo.isLeader);
    value().setLastSeen(registryInfo.lastSeen);
    value().setIp1(registryInfo.addrs.addrs[0].ip.data);
    value().setPort1(registryInfo.addrs.addrs[0].port);
    value().setIp2(registryInfo.addrs.addrs[1].ip.data);
    value().setPort2(registryInfo.addrs.addrs[1].port);
    ROCKS_DB_CHECKED(
        batch.Put(cf, key.toSlice(), value.toSlice())
    );
}

static inline void initializeRegistries(rocksdb::WriteBatch& batch, rocksdb::ColumnFamilyHandle* cf) {
    FullRegistryInfo registryInfo;
    registryInfo.clear();
    for (ReplicaId id = 0; id.u8 < LogsDB::REPLICA_COUNT; ++id.u8) {
        registryInfo.id = id;
        writeRegistryInfo(batch, cf, registryInfo);
    }
}

struct ShardInfoKey {
    FIELDS(
        LE, uint8_t, locationId, setLocationId,
        LE, uint8_t, shardId, setShardId,
        LE, ReplicaId, replicaId, setReplicaId,
        END_STATIC
    )
};

struct ShardInfoValue {
    FIELDS(
        LE, uint8_t,  version, _setVersion,
        LE, bool, isLeader, setIsLeader,
        LE, TernTime, lastSeen, setLastSeen,
        FBYTES, 4, ip1, setIp1,
        BE, uint16_t, port1, setPort1,
        FBYTES, 4, ip2, setIp2,
        BE, uint16_t, port2, setPort2,
        END_STATIC
    )
};

static inline void readShardInfo(rocksdb::Slice key_, rocksdb::Slice value_, FullShardInfo& shardInfo) {
    auto key = ExternalValue<ShardInfoKey>::FromSlice(key_);
    auto value = ExternalValue<ShardInfoValue>::FromSlice(value_);
    shardInfo.id = ShardReplicaId(key().shardId(), key().replicaId());
    shardInfo.locationId = key().locationId();
    shardInfo.isLeader = value().isLeader();
    shardInfo.lastSeen = value().lastSeen();
    shardInfo.addrs.addrs[0].ip = value().ip1();
    shardInfo.addrs.addrs[0].port = value().port1();
    shardInfo.addrs.addrs[1].ip = value().ip2();
    shardInfo.addrs.addrs[1].port = value().port2();
}

static inline void writeShardInfo(rocksdb::WriteBatch& batch, rocksdb::ColumnFamilyHandle* cf, const FullShardInfo& shardInfo) {
    StaticValue<ShardInfoKey> key;
    StaticValue<ShardInfoValue> value;
    key().setLocationId(shardInfo.locationId);
    key().setShardId(shardInfo.id.shardId().u8);
    key().setReplicaId(shardInfo.id.replicaId().u8);
    
    value()._setVersion(0);
    value().setIsLeader(shardInfo.isLeader);
    value().setLastSeen(shardInfo.lastSeen);
    value().setIp1(shardInfo.addrs.addrs[0].ip.data);
    value().setPort1(shardInfo.addrs.addrs[0].port);
    value().setIp2(shardInfo.addrs.addrs[1].ip.data);
    value().setPort2(shardInfo.addrs.addrs[1].port);
    ROCKS_DB_CHECKED(
        batch.Put(cf, key.toSlice(), value.toSlice())
    );
}

static inline void initializeShardsForLocation(
    rocksdb::WriteBatch&  batch, rocksdb::ColumnFamilyHandle* cf, LocationId locationId) 
{
    FullShardInfo shardInfo;
    shardInfo.clear();
    shardInfo.locationId = locationId;
    for(uint16_t id = 0; id < ShardId::SHARD_COUNT; ++id) {
        for (ReplicaId replicaId = 0; replicaId.u8 < LogsDB::REPLICA_COUNT; ++replicaId.u8) {
            shardInfo.id = ShardReplicaId(ShardId(id), replicaId);
            writeShardInfo(batch, cf, shardInfo);
        }
    }
}

struct CdcInfoKey {
    FIELDS(
        LE, uint8_t, locationId, setLocationId,
        LE, ReplicaId, replicaId, setReplicaId,
        END_STATIC
    )
};

struct CdcInfoValue {
    FIELDS(
        LE, uint8_t,  version, _setVersion,
        LE, bool, isLeader, setIsLeader,
        LE, TernTime, lastSeen, setLastSeen,
        FBYTES, 4, ip1, setIp1,
        BE, uint16_t, port1, setPort1,
        FBYTES, 4, ip2, setIp2,
        BE, uint16_t, port2, setPort2,
        END_STATIC
    )
};

static inline void readCdcInfo(rocksdb::Slice key_, rocksdb::Slice value_, CdcInfo& cdcInfo) {
    auto key = ExternalValue<CdcInfoKey>::FromSlice(key_);
    auto value = ExternalValue<CdcInfoValue>::FromSlice(value_);
    cdcInfo.replicaId = key().replicaId();
    cdcInfo.locationId = key().locationId();
    cdcInfo.isLeader = value().isLeader();
    cdcInfo.lastSeen = value().lastSeen();
    cdcInfo.addrs.addrs[0].ip = value().ip1();
    cdcInfo.addrs.addrs[0].port = value().port1();
    cdcInfo.addrs.addrs[1].ip = value().ip2();
    cdcInfo.addrs.addrs[1].port = value().port2();
}

static inline void writeCdcInfo(rocksdb::WriteBatch& batch, rocksdb::ColumnFamilyHandle* cf, const CdcInfo& cdcInfo) {
    StaticValue<CdcInfoKey> key;
    StaticValue<CdcInfoValue> value;
    key().setLocationId(cdcInfo.locationId);
    key().setReplicaId(cdcInfo.replicaId.u8);

    value()._setVersion(0);
    value().setIsLeader(cdcInfo.isLeader);
    value().setLastSeen(cdcInfo.lastSeen);
    value().setIp1(cdcInfo.addrs.addrs[0].ip.data);
    value().setPort1(cdcInfo.addrs.addrs[0].port);
    value().setIp2(cdcInfo.addrs.addrs[1].ip.data);
    value().setPort2(cdcInfo.addrs.addrs[1].port);
    ROCKS_DB_CHECKED(
        batch.Put(cf, key.toSlice(), value.toSlice())
    );
}

static inline void initializeCdcForLocation(
    rocksdb::WriteBatch&  batch, rocksdb::ColumnFamilyHandle* cf, LocationId locationId) 
{
    CdcInfo cdcInfo;
    cdcInfo.clear();
    cdcInfo.locationId = locationId;
    for (ReplicaId replicaId = 0; replicaId.u8 < LogsDB::REPLICA_COUNT; ++replicaId.u8) {
        cdcInfo.replicaId = replicaId;
        writeCdcInfo(batch, cf, cdcInfo);
    }
}

struct BlockServiceInfoKey {
    FIELDS(
        LE, uint64_t, id, setId,
        END_STATIC
    )
};

struct BlockServiceInfoValue {
    FIELDS(
        LE,     uint8_t,    version, _setVersion,
        LE,     uint8_t,    locationId, setLocationId,
        LE,     uint8_t,    storageClass, setStorageClass,
        FBYTES, 16,      failureDomain, setFailureDomain,
        FBYTES, 16,      secretKey, setSecretKey,
        LE,     TernTime,   firstSeen, setFirstSeen,
        LE,     TernTime,   lastSeen, setLastSeen,
        LE,     TernTime,   lastInfoChange, setLastInfoChange,
        FBYTES, 4,       ip1, setIp1,
        BE,     uint16_t,   port1, setPort1,
        FBYTES, 4,       ip2, setIp2,
        BE,     uint16_t,   port2, setPort2,
        LE,     BlockServiceFlags,    flags, setFlags,
        LE,     uint64_t,   capacityBytes, setCapacityBytes,
        LE,     uint64_t,   availableBytes, setAvailableBytes,
        LE,     uint64_t,   blocks, setBlocks,
        LE,     bool,       hasFiles, setHasFiles,
        EMIT_OFFSET, STATIC_SIZE,
        
        BYTES, path, setPath,
        END
    )
    
    static constexpr size_t MIN_SIZE =
        STATIC_SIZE +
        sizeof(uint8_t);    // pathLen
    
    static constexpr size_t MAX_SIZE = MIN_SIZE + 255;

    size_t size() const {
        return  MIN_SIZE + path().size();
    }

    void checkSize(size_t sz) {
        ALWAYS_ASSERT(sz >= MIN_SIZE, "expected %s >= %s", sz, MIN_SIZE);
        ALWAYS_ASSERT(sz == size());
    }
};

static inline void readBlockServiceInfo(rocksdb::Slice key_, rocksdb::Slice value_, FullBlockServiceInfo& blockServiceInfo) {
    auto key = ExternalValue<BlockServiceInfoKey>::FromSlice(key_);
    auto value = ExternalValue<BlockServiceInfoValue>::FromSlice(value_);
    blockServiceInfo.id = key().id();
    
    blockServiceInfo.locationId = value().locationId();
    blockServiceInfo.storageClass = value().storageClass();
    blockServiceInfo.failureDomain.name = value().failureDomain();
    blockServiceInfo.secretKey = value().secretKey();
    blockServiceInfo.firstSeen = value().firstSeen();
    blockServiceInfo.lastSeen = value().lastSeen();
    blockServiceInfo.lastInfoChange = value().lastInfoChange();
    blockServiceInfo.addrs.addrs[0].ip = value().ip1();
    blockServiceInfo.addrs.addrs[0].port = value().port1();
    blockServiceInfo.addrs.addrs[1].ip = value().ip2();
    blockServiceInfo.addrs.addrs[1].port = value().port2();
    
    blockServiceInfo.flags = value().flags();
    blockServiceInfo.capacityBytes = value().capacityBytes();
    blockServiceInfo.availableBytes = value().availableBytes();
    blockServiceInfo.blocks = value().blocks();
    blockServiceInfo.hasFiles = value().hasFiles();
    blockServiceInfo.path = value().path();
}

static inline void writeBlockServiceInfo(
    rocksdb::WriteBatch& batch, rocksdb::ColumnFamilyHandle* cf, const FullBlockServiceInfo& blockServiceInfo)
{
    StaticValue<BlockServiceInfoKey> key;
    StaticValue<BlockServiceInfoValue> value;
    key().setId(blockServiceInfo.id.u64);

    value()._setVersion(0);
    value().setLocationId(blockServiceInfo.locationId);
    value().setStorageClass(blockServiceInfo.storageClass);
    value().setFailureDomain(blockServiceInfo.failureDomain.name.data);
    value().setSecretKey(blockServiceInfo.secretKey.data);
    value().setFirstSeen(blockServiceInfo.firstSeen);
    value().setLastSeen(blockServiceInfo.lastSeen);
    value().setLastInfoChange(blockServiceInfo.lastInfoChange);
    value().setIp1(blockServiceInfo.addrs.addrs[0].ip.data);
    value().setPort1(blockServiceInfo.addrs.addrs[0].port);
    value().setIp2(blockServiceInfo.addrs.addrs[1].ip.data);
    value().setPort2(blockServiceInfo.addrs.addrs[1].port);
    
    value().setFlags(blockServiceInfo.flags);
    value().setCapacityBytes(blockServiceInfo.capacityBytes);
    value().setAvailableBytes(blockServiceInfo.availableBytes);
    value().setBlocks(blockServiceInfo.blocks);
    value().setHasFiles(blockServiceInfo.hasFiles);
    value().setPath(blockServiceInfo.path.ref());
    ROCKS_DB_CHECKED(
        batch.Put(cf, key.toSlice(), value.toSlice())
    );
}

static constexpr auto MAX_SERVICE_KEY_SIZE = 
    std::max(std::max(std::max(
        RegistryReplicaInfoKey::MAX_SIZE, ShardInfoKey::MAX_SIZE), CdcInfoKey::MAX_SIZE),BlockServiceInfoKey::MAX_SIZE);

enum class ServiceType : uint8_t {
    REGISTRY = 0,
    SHARD = 1,
    CDC = 2,
    BLOCK_SERVICE = 3
};
struct LastHeartBeatKey {
    FIELDS(
        BE, uint64_t, lastSeen, setLastSeen,
        LE, ServiceType, _serviceType, setServiceType,
        FBYTES, MAX_SERVICE_KEY_SIZE, _serviceBytes, _setServiceBytes,
        END_STATIC
    )
};

static inline const ExternalValue<LastHeartBeatKey> readLastHeartBeatInfo(rocksdb::Slice key) {
    return ExternalValue<LastHeartBeatKey>::FromSlice(key);
}

static inline void registryToLastHeartBeat(const FullRegistryInfo& registry, LastHeartBeatKey key) {
    key.setLastSeen(registry.lastSeen.ns);
    key.setServiceType(ServiceType::REGISTRY);
    FBytesArr<MAX_SERVICE_KEY_SIZE> serviceBytes;
    auto registryKey = ExternalValue<RegistryReplicaInfoKey>::FromSlice({(char*)serviceBytes.data(), RegistryReplicaInfoKey::MAX_SIZE});
    registryKey().setLocationId(registry.locationId);
    registryKey().setReplicaId(registry.id.u8);
    key._setServiceBytes(serviceBytes);
}

static inline void shardToLastHeartBeat(const FullShardInfo& shard, LastHeartBeatKey key) {
    key.setLastSeen(shard.lastSeen.ns);
    key.setServiceType(ServiceType::SHARD);
    FBytesArr<MAX_SERVICE_KEY_SIZE> serviceBytes;
    auto shardKey = ExternalValue<ShardInfoKey>::FromSlice({(char*)serviceBytes.data(), ShardInfoKey::MAX_SIZE});
    shardKey().setLocationId(shard.locationId);
    shardKey().setShardId(shard.id.shardId().u8);
    shardKey().setReplicaId(shard.id.replicaId().u8);
    key._setServiceBytes(serviceBytes);
}

static inline void cdcToLastHeartBeat(const CdcInfo& cdc, LastHeartBeatKey key) {
    key.setLastSeen(cdc.lastSeen.ns);
    key.setServiceType(ServiceType::CDC);
    FBytesArr<MAX_SERVICE_KEY_SIZE> serviceBytes;
    auto cdcKey = ExternalValue<CdcInfoKey>::FromSlice({(char*)serviceBytes.data(), CdcInfoKey::MAX_SIZE});
    cdcKey().setLocationId(cdc.locationId);
    cdcKey().setReplicaId(cdc.replicaId.u8);
    key._setServiceBytes(serviceBytes);
}

static inline void blockServiceToLastHeartBeat(const FullBlockServiceInfo& bs, LastHeartBeatKey key) {
    key.setLastSeen(bs.lastSeen.ns);
    key.setServiceType(ServiceType::BLOCK_SERVICE);
    FBytesArr<MAX_SERVICE_KEY_SIZE> serviceBytes;
    auto bsKey = ExternalValue<BlockServiceInfoKey>::FromSlice({(char*)serviceBytes.data(), BlockServiceInfoKey::MAX_SIZE});
    bsKey().setId(bs.id.u64);
    key._setServiceBytes(serviceBytes);
}


struct WritableBlockServiceKey {
    FIELDS(
        LE,     uint8_t,    locationId, setLocationId,
        LE,     uint8_t,    storageClass, setStorageClass,
        FBYTES, 16,      failureDomain, setFailureDomain,
        BE,     uint64_t,   availableBytes, setAvailableBytes,
        LE,     uint64_t,   id, setId,
        END_STATIC
    )
};

static inline void blockServiceToWritableBlockServiceKey(const FullBlockServiceInfo& bs, WritableBlockServiceKey key) {
    ALWAYS_ASSERT(bs.availableBytes > 0);
    ALWAYS_ASSERT(isWritable(bs.flags));
    key.setLocationId(bs.locationId);
    key.setStorageClass(bs.storageClass);
    key.setFailureDomain(bs.failureDomain.name.data);
    key.setAvailableBytes(bs.availableBytes);
    key.setId(bs.id.u64);
}

