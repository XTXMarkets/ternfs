// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <rocksdb/db.h>
#include <vector>

#include "Bincode.hpp"
#include "BlockServicesCacheDB.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "RocksDBUtils.hpp"
#include "Time.hpp"

enum class BlockServicesCacheKey : uint8_t {
    CURRENT_BLOCK_SERVICES = 0,
    BLOCK_SERVICE = 1, // postfixed with the block service id
};

constexpr BlockServicesCacheKey CURRENT_BLOCK_SERVICES_KEY = BlockServicesCacheKey::CURRENT_BLOCK_SERVICES;
constexpr BlockServicesCacheKey BLOCK_SERVICE_KEY = BlockServicesCacheKey::BLOCK_SERVICE;

inline rocksdb::Slice blockServicesCacheKey(const BlockServicesCacheKey* k) {
    ALWAYS_ASSERT(*k != BLOCK_SERVICE_KEY);
    return rocksdb::Slice((const char*)k, sizeof(*k));
}

struct BlockServiceKey {
    FIELDS(
        BE, BlockServicesCacheKey, key, setKey, // always BLOCK_SERVICE_KEY
        BE, uint64_t,              blockServiceId, setBlockServiceId,
        END_STATIC
    )
};

struct BlockServiceBody {
    FIELDS(
        LE, uint8_t,  version,          setVersion,
        LE, uint64_t, id,               setId,
        FBYTES, 4,    ip1,              setIp1,
        LE, uint16_t, port1,            setPort1,
        FBYTES, 4,    ip2,              setIp2,
        LE, uint16_t, port2,            setPort2,
        LE, uint8_t,  storageClass,     setStorageClass,
        FBYTES, 16,   failureDomain,    setFailureDomain,
        FBYTES, 16,   secretKey,        setSecretKey,
        EMIT_OFFSET, V0_OFFSET,
        LE, BlockServiceFlags, flagsV1, setFlagsV1,
        EMIT_OFFSET, V1_OFFSET,
        LE, uint8_t,  locationIdV2,     setLocationIdV2,
        LE, uint64_t, capacityBytesV2,  setCapacityBytesV2,
        LE, uint64_t, availableBytesV2, setAvailableBytesV2,
        LE, uint64_t, blocksV2,         setBlocksV2,
        LE, uint64_t, firstSeenV2,      setFirstSeenV2,
        LE, uint64_t, lastSeenV2,       setLastSeenV2,
        LE, uint64_t, lastInfoChangeV2, setLastInfoChangeV2,
        LE, uint8_t,  hasFilesV2,       setHasFilesV2,
        BYTES, pathV2, setPathV2,
        END
    )

    static constexpr size_t V2_OFFSET = V1_OFFSET + 1 + 8*6 + 1;  // locationId + 6 uint64_t + hasFiles

    static constexpr size_t MIN_SIZE = V2_OFFSET + sizeof(uint8_t);  // +1 for path length byte
    static constexpr size_t MAX_SIZE = MIN_SIZE + 255;  // max path size

    size_t size() const {
        switch (version()) {
        case 0: return V0_OFFSET;
        case 1: return V1_OFFSET;
        case 2: return MIN_SIZE + pathV2().size();
        default: throw TERN_EXCEPTION("bad version %s", version());
        }
    }

    void checkSize(size_t sz) {
        ALWAYS_ASSERT(sz >= MIN_SIZE || version() < 2, "expected %s >= %s", sz, MIN_SIZE);
        ALWAYS_ASSERT(sz == size());
    }

    BlockServiceFlags flags() const {
        if (unlikely(version() == 0)) { return BlockServiceFlags::EMPTY; }
        return flagsV1();
    }

    void setFlags(BlockServiceFlags f) {
        ALWAYS_ASSERT(version() > 0);
        setFlagsV1(f);
    }

    uint8_t locationId() const {
        ALWAYS_ASSERT(version() >= 2);
        return locationIdV2();
    }

    void setLocationId(uint8_t v) {
        ALWAYS_ASSERT(version() >= 2);
        setLocationIdV2(v);
    }

    uint64_t capacityBytes() const {
        ALWAYS_ASSERT(version() >= 2);
        return capacityBytesV2();
    }

    void setCapacityBytes(uint64_t v) {
        ALWAYS_ASSERT(version() >= 2);
        setCapacityBytesV2(v);
    }

    uint64_t availableBytes() const {
        ALWAYS_ASSERT(version() >= 2);
        return availableBytesV2();
    }

    void setAvailableBytes(uint64_t v) {
        ALWAYS_ASSERT(version() >= 2);
        setAvailableBytesV2(v);
    }

    uint64_t blocks() const {
        ALWAYS_ASSERT(version() >= 2);
        return blocksV2();
    }

    void setBlocks(uint64_t v) {
        ALWAYS_ASSERT(version() >= 2);
        setBlocksV2(v);
    }

    uint64_t firstSeen() const {
        ALWAYS_ASSERT(version() >= 2);
        return firstSeenV2();
    }

    void setFirstSeen(uint64_t v) {
        ALWAYS_ASSERT(version() >= 2);
        setFirstSeenV2(v);
    }

    uint64_t lastSeen() const {
        ALWAYS_ASSERT(version() >= 2);
        return lastSeenV2();
    }

    void setLastSeen(uint64_t v) {
        ALWAYS_ASSERT(version() >= 2);
        setLastSeenV2(v);
    }

    uint64_t lastInfoChange() const {
        ALWAYS_ASSERT(version() >= 2);
        return lastInfoChangeV2();
    }

    void setLastInfoChange(uint64_t v) {
        ALWAYS_ASSERT(version() >= 2);
        setLastInfoChangeV2(v);
    }

    bool hasFiles() const {
        ALWAYS_ASSERT(version() >= 2);
        return hasFilesV2() != 0;
    }

    void setHasFiles(bool v) {
        ALWAYS_ASSERT(version() >= 2);
        setHasFilesV2(v ? 1 : 0);
    }

    std::string_view path() const {
        if (version() < 2) { return ""; }
        auto ref = pathV2();
        return std::string_view(ref.data(), ref.size());
    }

    void setPath(const std::string& p) {
        ALWAYS_ASSERT(version() >= 2);
        setPathV2(BincodeBytesRef(p.data(), p.size()));
    }
};

std::vector<rocksdb::ColumnFamilyDescriptor> BlockServicesCacheDB::getColumnFamilyDescriptors() {
    return std::vector<rocksdb::ColumnFamilyDescriptor> {{"blockServicesCache", {}}};
}

BlockServicesCacheDB::BlockServicesCacheDB(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const SharedRocksDB& sharedDB, Duration blockServiceWritableDelay) :
    _env(logger, xmon, "bs_cache_db"),
    _db(sharedDB.db()),
    _blockServicesCF(sharedDB.getCF("blockServicesCache")),
    _picker(logger, xmon, 15, blockServiceWritableDelay)
{
    LOG_INFO(_env, "Initializing block services cache DB");

    std::unique_lock _(_mutex);

    const auto keyExists = [this](rocksdb::ColumnFamilyHandle* cf, const rocksdb::Slice& key) -> bool {
        std::string value;
        auto status = _db->Get({}, cf, key, &value);
        if (status.IsNotFound()) {
            return false;
        } else {
            ROCKS_DB_CHECKED(status);
            return true;
        }
    };

    // Drop legacy current block services key if present
    if (keyExists(_blockServicesCF, blockServicesCacheKey(&CURRENT_BLOCK_SERVICES_KEY))) {
        ROCKS_DB_CHECKED(_db->Delete({}, _blockServicesCF, blockServicesCacheKey(&CURRENT_BLOCK_SERVICES_KEY)));
    }

    {
        LOG_INFO(_env, "initializing block services cache (from db)");
        rocksdb::ReadOptions options;
        static_assert(sizeof(BlockServicesCacheKey) == sizeof(uint8_t));
        auto upperBound = (BlockServicesCacheKey)((uint8_t)BLOCK_SERVICE_KEY + 1);
        auto upperBoundSlice = blockServicesCacheKey(&upperBound);
        options.iterate_upper_bound = &upperBoundSlice;
        std::unique_ptr<rocksdb::Iterator> it(_db->NewIterator(options, _blockServicesCF));
        StaticValue<BlockServiceKey> beginKey;
        beginKey().setKey(BLOCK_SERVICE_KEY);
        beginKey().setBlockServiceId(0);
        for (it->Seek(beginKey.toSlice()); it->Valid(); it->Next()) {
            _haveBlockServices = true;
            auto k = ExternalValue<BlockServiceKey>::FromSlice(it->key());
            ALWAYS_ASSERT(k().key() == BLOCK_SERVICE_KEY);
            auto v = ExternalValue<BlockServiceBody>::FromSlice(it->value());
            auto& cache = _blockServices[k().blockServiceId()];
            cache.addrs[0].ip = v().ip1();
            cache.addrs[0].port = v().port1();
            cache.addrs[1].ip = v().ip2();
            cache.addrs[1].port = v().port2();
            expandKey(v().secretKey(), cache.secretKey);
            cache.storageClass = v().storageClass();
            cache.flags = v().flags();
            cache.failureDomain = v().failureDomain();

            if (v().version() >= 2) {
                cache.locationId = v().locationId();
                cache.capacityBytes = v().capacityBytes();
                cache.availableBytes = v().availableBytes();
                cache.blocks = v().blocks();
                cache.firstSeen = TernTime(v().firstSeen());
                cache.lastSeen = TernTime(v().lastSeen());
                cache.lastInfoChange = TernTime(v().lastInfoChange());
                cache.hasFiles = v().hasFiles();
                cache.path = (v().version() >= 2) ? std::string(v().path()) : "";
            } else {
                // Set defaults for v1 format
                cache.locationId = 0;
                cache.capacityBytes = 0;
                cache.availableBytes = 0;
                cache.blocks = 0;
                cache.firstSeen = TernTime();
                cache.lastSeen = TernTime();
                cache.lastInfoChange = TernTime();
                cache.hasFiles = false;
                cache.path = "";
            }
        }
        ROCKS_DB_CHECKED(it->status());
        if (!_haveBlockServices) {
            LOG_INFO(_env, "no block services cache in db yet");
        }
    }
}

BlockServicesCache BlockServicesCacheDB::getCache() const {
    return BlockServicesCache(_mutex, _blockServices);
}

void BlockServicesCacheDB::updateCache(const std::vector<FullBlockServiceInfo>& blockServices) {
    LOG_INFO(_env, "Updating %s changed block services", blockServices.size());

    std::unique_lock _(_mutex);

    rocksdb::WriteBatch batch;
    StaticValue<BlockServiceKey> blockKey;
    blockKey().setKey(BLOCK_SERVICE_KEY);
    StaticValue<BlockServiceBody> blockBody;
    for (const auto& entryBlock : blockServices) {
        blockKey().setBlockServiceId(entryBlock.id.u64);
        blockBody().setVersion(2);
        blockBody().setId(entryBlock.id.u64);
        blockBody().setIp1(entryBlock.addrs[0].ip.data);
        blockBody().setPort1(entryBlock.addrs[0].port);
        blockBody().setIp2(entryBlock.addrs[1].ip.data);
        blockBody().setPort2(entryBlock.addrs[1].port);
        blockBody().setStorageClass(entryBlock.storageClass);
        blockBody().setFailureDomain(entryBlock.failureDomain.name.data);
        blockBody().setSecretKey(entryBlock.secretKey.data);
        blockBody().setFlags(entryBlock.flags);
        blockBody().setLocationId(entryBlock.locationId);
        blockBody().setCapacityBytes(entryBlock.capacityBytes);
        blockBody().setAvailableBytes(entryBlock.availableBytes);
        blockBody().setBlocks(entryBlock.blocks);
        blockBody().setLastSeen(entryBlock.lastSeen.ns);
        blockBody().setLastInfoChange(entryBlock.lastInfoChange.ns);
        blockBody().setHasFiles(entryBlock.hasFiles);
        blockBody().setFirstSeen(entryBlock.firstSeen.ns);
        blockBody().setPath(std::string(entryBlock.path.data(), entryBlock.path.size()));

        ROCKS_DB_CHECKED(batch.Put(_blockServicesCF, blockKey.toSlice(), blockBody.toSlice()));

        auto& cache = _blockServices[entryBlock.id.u64];
        expandKey(entryBlock.secretKey.data, cache.secretKey);
        cache.addrs = entryBlock.addrs;
        cache.storageClass = entryBlock.storageClass;
        cache.failureDomain = entryBlock.failureDomain.name.data;
        cache.flags = entryBlock.flags;
        cache.locationId = entryBlock.locationId;
        cache.capacityBytes = entryBlock.capacityBytes;
        cache.availableBytes = entryBlock.availableBytes;
        cache.blocks = entryBlock.blocks;
        cache.lastSeen = entryBlock.lastSeen;
        cache.lastInfoChange = entryBlock.lastInfoChange;
        cache.hasFiles = entryBlock.hasFiles;
        cache.firstSeen = entryBlock.firstSeen;
        cache.path = std::string(entryBlock.path.data(), entryBlock.path.size());
    }

    ROCKS_DB_CHECKED(_db->Write({}, &batch));

    _haveBlockServices = true;
    _picker.update(_blockServices);
}

void BlockServicesCacheDB::updateAvailableSpace(const std::vector<BlockServiceSpace>& blockServices) {
    LOG_INFO(_env, "Updating available space for %s block services", blockServices.size());

    std::unique_lock _(_mutex);

    rocksdb::WriteBatch batch;
    StaticValue<BlockServiceKey> blockKey;
    blockKey().setKey(BLOCK_SERVICE_KEY);
    for (const auto& entry : blockServices) {
        auto it = _blockServices.find(entry.id.u64);
        if (it == _blockServices.end()) {
            continue;
        }

        it->second.capacityBytes = entry.capacityBytes;
        it->second.availableBytes = entry.availableBytes;
        it->second.blocks = entry.blocks;

        // Update RocksDB - read existing entry and update space fields
        blockKey().setBlockServiceId(entry.id.u64);
        std::string existingValue;
        auto status = _db->Get({}, _blockServicesCF, blockKey.toSlice(), &existingValue);
        if (status.IsNotFound()) {
            continue;
        }
        ROCKS_DB_CHECKED(status);
        auto v = ExternalValue<BlockServiceBody>::FromSlice(rocksdb::Slice(existingValue));
        if (v().version() < 2) {
            continue;
        }
        v().setCapacityBytes(entry.capacityBytes);
        v().setAvailableBytes(entry.availableBytes);
        v().setBlocks(entry.blocks);
        ROCKS_DB_CHECKED(batch.Put(_blockServicesCF, blockKey.toSlice(), rocksdb::Slice(existingValue)));
    }

    ROCKS_DB_CHECKED(_db->Write({}, &batch));

    _picker.update(_blockServices);
}

TernError BlockServicesCacheDB::pickBlockServices(
    uint8_t locationId,
    uint8_t storageClass,
    int needed,
    const std::vector<BlacklistEntry>& blacklist,
    std::vector<BlockServiceId>& out
) const {
    return _picker.pick(locationId, storageClass, needed, blacklist, out);
}

bool BlockServicesCacheDB::haveBlockServices() const {
    return _haveBlockServices;
}
