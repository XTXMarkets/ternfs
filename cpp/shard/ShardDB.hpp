// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <array>
#include <cstdint>
#include <vector>
#include <rocksdb/db.h>

#include "BlockServicesCacheDB.hpp"
#include "Msgs.hpp"
#include "Env.hpp"
#include "SharedRocksDB.hpp"


struct ShardLogEntry {
    LogIdx idx;
    TernTime time;
    ShardLogEntryContainer body;

    bool operator==(const ShardLogEntry& rhs) const {
        return time == rhs.time && body == rhs.body;
    }

    void clear() {
        time = 0;
        body.clear();
    }

    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
};

std::ostream& operator<<(std::ostream& out, const ShardLogEntry& entry);

bool readOnlyShardReq(const ShardMessageKind kind);

DirectoryInfo defaultDirectoryInfo();

// 100MiB. Important to enforce this since we often need to fetch the span upfront.
constexpr uint32_t MAXIMUM_SPAN_SIZE = 100 << 20;

constexpr Duration DEFAULT_DEADLINE_INTERVAL = 2_hours;

struct ShardDB {
private:
    void* _impl;

public:
    ShardDB() = delete;

    // init/teardown
    ShardDB(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardId shid, uint8_t location, Duration deadlineInterval, const SharedRocksDB& sharedDB, const BlockServicesCacheDB& blockServicesCache);
    ~ShardDB();
    void close();

    // Performs a read-only request, responding immediately.
    // Returns last applied log entry at the point of reading
    uint64_t read(const ShardReqContainer& req, ShardRespContainer& resp);

    // Prepares and persists a log entry to be applied.
    //
    // This function can be called concurrently. We only read from the database to
    // do this. Note that the reading is limited to data which is OK to be a bit
    // stale, such as block service information, and which is not used in a way
    // which relies on the log entries being successfully applied.
    //
    // The log entry is not persisted in any way -- the idea is that a consensus module
    // can be built on top of this to first persist the log entry across a consensus
    // of machines, and then the log entries can be applied.
    //
    // Morally this function always succeeds, the "real" error checking is all done at
    // log application time. The reasoning here is that log preparation is not reading
    // from the latest state anyway, since there might be many log entries in flight which
    // we have prepared but not applied, and therefore this function does not have the latest
    // view of the state. We are still allowed to read from the state at log preparation time
    // (e.g. to gather needed info about block services), but knowing that we might lag a bit
    // behind.
    //
    // However we still do some "type checking" here (e.g. we have ids in the right
    // shard and of the right type, good transient file cookies), and we might still
    // return errors for some corner cases (e.g. if we cannot allocate block services
    // for some span request or something like that).
    //
    // As usual, if an error is returned, the contents of `logEntry` should be ignored.
    TernError prepareLogEntry(const ShardReqContainer& req, ShardLogEntry& logEntry);

    // The index of the last log entry persisted to the DB
    uint64_t lastAppliedLogEntry();

    // Applies the log entry at the given index, and fills in the client response.
    // The log entry index should be the next in line to be applied to the state --
    // `lastAppliedLogEntry() + 1`.
    //
    // It is the job of the caller to keep track of indices, this state machine  only
    // remembers which index it last applied.
    //
    // This function will fail if called concurrently -- we rely on having a single
    // writer for easy of synchronization.
    //
    // The state is written atomically (you can never witness an half-applied log entry).
    //
    // The `sync` parameter determines whether `fsync` is called when persisting the
    // state. If the log entry is persisted before `applyLogEntry` is applied, this might
    // not be necessary. If it is not, then it is certainly necessary.
    //
    // This function does NOT persist the changes (in fact it doesn't even write
    // to the WAL). You need to call flush(). But this allows you to apply many
    // log entries without any write/fsync.
    void applyLogEntry(uint64_t logEntryIx, const ShardLogEntry& logEntry, ShardRespContainer& resp);

    // Flushes the changes to the WAL, and persists it if sync=true (won't be
    // required when we have a distributed log).
    //
    // Before calling this function the writes up to the previous flush() will
    // not be visible to reads (but they will be visible to writes).
    void flush(bool sync);

    // For internal testing
    const std::array<uint8_t, 16>& secretKey() const;

    static std::vector<rocksdb::ColumnFamilyDescriptor> getColumnFamilyDescriptors();
};
