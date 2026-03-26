// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <shared_mutex>
#include <vector>

#include "LogsDB.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "RegistryCommon.hpp"
#include "RegistryDBLogEntry.hpp"
#include "SharedRocksDB.hpp"

struct RegistryDBWriteResult {
    EntryReqIdx idx;
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

    void flush(bool sync = true) { _db->FlushWAL(sync); }

    void updateReplicaInfo(const RegisterRegistryReq& req);

private:
    void _initDb();

    bool _updateStaleBlockServices(TernTime now);
    void _recalculateShardBlockServices(bool writableChanged);

    const RegistryOptions& _options;
    Env _env;


    TernTime _lastCalculatedShardBlockServices;
    std::unordered_map<uint8_t, std::vector<BlockServiceInfoShort>> _shardBlockServices;

    mutable std::shared_mutex _replicaInfosMutex;
    std::array<FullRegistryInfo, LogsDB::REPLICA_COUNT> _replicaInfos{};

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
