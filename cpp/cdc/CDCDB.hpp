// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <cstdint>
#include <rocksdb/db.h>
#include <unordered_map>
#include <vector>

#include "Bincode.hpp"
#include "Env.hpp"
#include "MsgsGen.hpp"
#include "SharedRocksDB.hpp"
#include "RegistryClient.hpp"

// This exists purely for type safety
struct CDCTxnId {
    uint64_t x;

    CDCTxnId() : x(0) {} // txn ids are never zeros, use it as a null value
    CDCTxnId(uint64_t x_) : x(x_) {}

    bool operator==(const CDCTxnId rhs) const {
        return x == rhs.x;
    }

    bool operator!=(const CDCTxnId rhs) const {
        return x != rhs.x;
    }
};

std::ostream& operator<<(std::ostream& out, CDCTxnId id);

template <>
struct std::hash<CDCTxnId> {
    std::size_t operator()(const CDCTxnId key) const {
        return std::hash<uint64_t>{}(key.x);
    }
};

struct CDCShardReq {
    ShardId shid;
    bool repeated; // This request is exactly the same as the previous one.
    ShardReqContainer req;

    void clear() {
        shid = ShardId(0);
        repeated = false;
        req.clear();
    }
};

std::ostream& operator<<(std::ostream& out, const CDCShardReq& x);

struct CDCStep {
    std::vector<std::pair<CDCTxnId, CDCRespContainer>> finishedTxns; // txns which have finished
    std::vector<std::pair<CDCTxnId, CDCShardReq>> runningTxns;  // txns which need a new shard request

    void clear() {
        finishedTxns.clear();
        runningTxns.clear();
    }
};

std::ostream& operator<<(std::ostream& out, const CDCStep& x);

struct CDCShardResp {
    CDCTxnId txnId; // the transaction id we're getting a response for
    LogIdx checkPoint;
    ShardRespContainer resp;
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    size_t packedSize() const;
    bool operator==(const CDCShardResp& rhs) const {
        return txnId == rhs.txnId && checkPoint == rhs.checkPoint && resp == rhs.resp;
    }
};

std::ostream& operator<<(std::ostream& out, const CDCShardResp& x);

class CDCLogEntry {
public:
    static void prepareLogEntries(std::vector<CDCReqContainer>& cdcReqs, std::vector<CDCShardResp>& shardResps, size_t maxPackedSize, std::vector<CDCLogEntry>& entriesOut);
    static CDCLogEntry prepareBootstrapEntry();

    CDCLogEntry() = default;
    CDCLogEntry(const CDCLogEntry&) = delete;
    CDCLogEntry(CDCLogEntry&&) = default;
    CDCLogEntry& operator=(CDCLogEntry&&) = default;

    void logIdx(uint64_t idx ) { _logIndex = idx; }
    bool bootstrapEntry() const { return _bootstrapEntry; }
    uint64_t logIdx() const { return _logIndex; }
    const std::vector<CDCReqContainer>& cdcReqs() const { return _cdcReqs; }
    const std::vector<CDCShardResp>& shardResps() const { return _shardResps; }

    bool operator==(const CDCLogEntry& rhs) const {
        return _logIndex == rhs._logIndex && _bootstrapEntry == rhs._bootstrapEntry &&
            _cdcReqs == rhs._cdcReqs && _shardResps == rhs._shardResps;
    }

    void clear() {
        _logIndex = 0;
        _bootstrapEntry = false;
        _cdcReqs.clear();
        _shardResps.clear();
    }

    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    size_t packedSize() const;
private:
    uint64_t _logIndex;
    bool _bootstrapEntry;
    std::vector<CDCReqContainer> _cdcReqs;
    std::vector<CDCShardResp> _shardResps;
};

std::ostream& operator<<(std::ostream& out, const CDCLogEntry& x);

struct CDCDB {
private:
    void* _impl;

public:
    CDCDB() = delete;
    CDCDB& operator=(const CDCDB&) = delete;

    CDCDB(Logger& env, std::shared_ptr<XmonAgent>& xmon, SharedRocksDB& sharedDb);
    ~CDCDB();


    // The functions below cannot be called concurrently.

    // Enqueues some CDC requests, and immediately starts it if possible.
    // Returns the txn id that was assigned to each request.
    //
    // Also, advances the CDC state using some shard responses.
    // When becoming a leader you need to pass a bootstrap log entry which will
    // instruct which requests to send to shards.
    //
    // This function crashes hard if the caller passes it a response it's not
    // expecting. So the caller should track responses and make sure only relevant
    // ones are passed in.
    void applyLogEntry(
        bool sync,
        const CDCLogEntry& entry,
        CDCStep& step,
        // Output txn ids for all the requests, same length as `cdcReqs`.
        std::vector<CDCTxnId>& cdcReqsTxnIds
    );

    // The index of the last log entry persisted to the DB
    uint64_t lastAppliedLogEntry();

    static std::vector<rocksdb::ColumnFamilyDescriptor> getColumnFamilyDescriptors();
};
