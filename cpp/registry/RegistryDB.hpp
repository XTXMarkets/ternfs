// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <vector>

#include "LogsDB.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "RegistryCommon.hpp"
#include "SharedRocksDB.hpp"

struct RegistryDBWriteResult {
    LogIdx idx;
    RegistryMessageKind kind;
    TernError err;
};

class RegistryDB {
public:
    static std::vector<rocksdb::ColumnFamilyDescriptor> getColumnFamilyDescriptors();
    
    RegistryDB(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const RegistryOptions& options, const SharedRocksDB& sharedDB);
    ~RegistryDB() {}
    void close();

    LogIdx lastAppliedLogEntry() const;

    void registries(std::vector<FullRegistryInfo>& out) const;
    void locations(std::vector<LocationInfo>& out) const;
    void shards(std::vector<FullShardInfo>& out) const;
    void cdcs(std::vector<CdcInfo>& out) const;
    void blockServices(std::vector<FullBlockServiceInfo>& out) const;
    void shardBlockServices(ShardId shardId, std::vector<BlockServiceInfoShort>& out) const {
        out = _shardBlockServices.at(shardId.u8);
    }

    void processLogEntries(std::vector<LogsDBLogEntry>& logEntries, std::vector<RegistryDBWriteResult>& writeResults);

private:
    void _initDb();

    bool _updateStaleBlockServices(TernTime now);
    void _recalcualteShardBlockServices(bool writableChanged);

    const RegistryOptions& _options;
    Env _env;


    TernTime _lastCalculatedShardBlockServices;
    std::unordered_map<uint8_t, std::vector<BlockServiceInfoShort>> _shardBlockServices;

    rocksdb::DB* _db;
    rocksdb::ColumnFamilyHandle* _defaultCf;
    rocksdb::ColumnFamilyHandle* _registryCf;
    rocksdb::ColumnFamilyHandle* _locationsCf;
    rocksdb::ColumnFamilyHandle* _shardsCf;
    rocksdb::ColumnFamilyHandle* _cdcCf;
    rocksdb::ColumnFamilyHandle* _blockServicesCf;
    rocksdb::ColumnFamilyHandle* _lastHeartBeatCf;
    rocksdb::ColumnFamilyHandle* _writableBlockServicesCf;
};
