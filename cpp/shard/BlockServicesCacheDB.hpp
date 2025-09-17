// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <shared_mutex>
#include <array>
#include <unordered_map>
#include <vector>
#include <rocksdb/db.h>

#include "Crypto.hpp"
#include "Common.hpp"
#include "Env.hpp"
#include "SharedRocksDB.hpp"
#include "Msgs.hpp"

struct BlockServiceCache {
    AES128Key secretKey;
    std::array<uint8_t, 16> failureDomain;
    AddrsInfo addrs;
    uint8_t storageClass;
    BlockServiceFlags flags;
};

struct BlockServicesCache {
private:
    std::shared_mutex& _shared_mutex;
public:
    const std::unordered_map<uint64_t, BlockServiceCache>& blockServices;
    const std::vector<BlockServiceInfoShort>& currentBlockServices;

    BlockServicesCache(
        std::shared_mutex& mutex,
        const std::unordered_map<uint64_t, BlockServiceCache>& blockServices_,
        const std::vector<BlockServiceInfoShort>& currentBlockServices_
    ) :
        _shared_mutex(mutex), blockServices(blockServices_), currentBlockServices(currentBlockServices_)
    {
        _shared_mutex.lock_shared();
    }

    ~BlockServicesCache() {
        _shared_mutex.unlock();
    }

    BlockServicesCache(const BlockServicesCache&) = delete;
};

struct BlockServicesCacheDB {
private:
    Env _env;

    rocksdb::DB* _db;
    rocksdb::ColumnFamilyHandle* _blockServicesCF;

    bool _haveBlockServices = false;

    // In-memory version of the RocksDB data
    mutable std::shared_mutex _mutex;
    // Cache of all the block services as an in-memory map.
    std::unordered_map<uint64_t, BlockServiceCache> _blockServices;
    // The block services that we currently want to write to.
    std::vector<BlockServiceInfoShort> _currentBlockServices;

public:
    BlockServicesCacheDB(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const SharedRocksDB& sharedDB);
    static std::vector<rocksdb::ColumnFamilyDescriptor> getColumnFamilyDescriptors();

    void updateCache(const std::vector<BlockServiceDeprecatedInfo>& blockServices, const std::vector<BlockServiceInfoShort>& currentBlockServices);

    // We've seen at least one `updateCache()`, or we've loaded the
    // block services from the cache.
    bool haveBlockServices() const;

    // The reason for this state to live outside ShardDB is that we don't want the
    // block services updates, which are very big (we have 20k block services right
    // now and it'll probably be more soon) to be in the main log replication.
    // It also conceptually doesn't make much sense since the block services data
    // is never in the ShardDB write path.
    //
    // However to keep the ShardDB state machine coherent _you need to make sure
    // that you use `getCache()` judiciously_.
    //
    // Specifically, there are two instances where you can use `getCache()` in
    // ShardDB.cpp:
    //
    // 1. You're filling in a response (e.g. converting block service ids to
    //     addresses);
    // 2. You're preparing a log entry.
    //
    // You CANNOT use it to decide what to write to the state, or in other words
    // you cannot use it in the `applyLogEntry()` call tree _unless you're filling
    // in a response (point 1 above).
    BlockServicesCache getCache() const;
};
