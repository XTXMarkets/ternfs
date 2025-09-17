// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <atomic>
#include <cstdint>
#include <ostream>
#include <vector>
#include <rocksdb/db.h>

#include "Bincode.hpp"
#include "Env.hpp"
#include "Protocol.hpp"
#include "SharedRocksDB.hpp"
#include "Time.hpp"

// ** Releases **
// Released records are records which have been confirmed by the leader to have been at some point correctly replicated.
// Leader only confirms record with LogIdx X if all records up to and including X have been correctly replicated.
// This guarantee simplifies the message structure as we can move release point with a single message.
// Releases are not required from correctness perspective but they improve performance as we have a guarantee that any record
// before the release point can be read without having a quorum of replicas. It also allows followers to apply release records
// to a state machine wihout first checking if the record has been correctly replicated.
// It also simplifies cleanup during leader election. Imagine following scenario:
// Leader is replica 0, R indicates that a record is replicated to specific replica X that it's not
// Leader replication window
// ReplicaId   LogIdx(2) LogIdx(3) LogIdx(4)
//         0          R         R         R
//         1          X         R         X
//         2          R         X         R
//         3          R         X         R
//         4          R         X         R
// Replicas (0,1) suddenly go away
// Replicas (2,3,4) elect 2 as new leader
// During "recovery" (a process which cleans up after previous leader from LastReleasedIdx)
// They don't see Record with LogIdx(3). Because they got a majority of nodes saying there is no record
// They know it couldn't have been correctly replicated. It is safe to drop it and all records after it.
// Because we don't want holes or conflicts we rewind logIndex to 3 and next appended record will come with 3
// What happens when replicas (0,1) come back. They will receive a movement of release point to arbitraty point in the future
// However they will, based on the leader token, notice it comes from a different leader.
// Since they were not part of the leader election they know their records after last released point have not been taken into
// account and could have been overwriten. They at this point drop these records and catch up from lastReleased point.


struct LogsDBLogEntry {
    LogIdx idx;
    std::vector<uint8_t> value;
    bool operator==(const LogsDBLogEntry& oth) const {
        return idx == oth.idx && value == oth.value;
    }
};

std::ostream& operator<<(std::ostream& out, const LogsDBLogEntry& entry);

struct LogsDBRequest {
    ReplicaId replicaId;
    TernTime sentTime;
    LogReqMsg msg;
};

std::ostream& operator<<(std::ostream& out, const LogsDBRequest& entry);

struct LogsDBResponse {
    ReplicaId replicaId;
    LogRespMsg msg;
};

std::ostream& operator<<(std::ostream& out, const LogsDBResponse& entry);

struct LogsDBStats {
    std::atomic<Duration> idleTime{0};
    std::atomic<Duration> processingTime{0};
    std::atomic<Duration> leaderLastActive{0};
    std::atomic<double> appendWindow{0};
    std::atomic<double> entriesReleased{0};
    std::atomic<double> followerLag{0};
    std::atomic<double> readerLag{0};
    std::atomic<double> catchupWindow{0};
    std::atomic<double> entriesRead{0};
    std::atomic<double> requestsReceived{0};
    std::atomic<double> responsesReceived{0};
    std::atomic<double> requestsSent{0};
    std::atomic<double> responsesSent{0};
    std::atomic<double> requestsTimedOut{0};
    std::atomic<uint64_t> currentEpoch{0};
    std::atomic<bool> isLeader{false};
};

class LogsDBImpl;

class LogsDB {
public:
    static constexpr size_t REPLICA_COUNT = 5;
    static constexpr Duration PARTITION_TIME_SPAN = 12_hours;
    static constexpr Duration RESPONSE_TIMEOUT = 10_ms;
    static constexpr Duration READ_TIMEOUT = 1_sec;
    static constexpr Duration SEND_RELEASE_INTERVAL = 300_ms;
    static constexpr Duration LEADER_INACTIVE_TIMEOUT = 1_sec;
    static constexpr size_t IN_FLIGHT_APPEND_WINDOW = 1 << 8;
    static constexpr size_t CATCHUP_WINDOW = 1 << 8 ;

    static constexpr size_t MAX_UDP_ENTRY_SIZE = MAX_UDP_MTU - std::max(LogReqMsg::STATIC_SIZE, LogRespMsg::STATIC_SIZE);
    static constexpr size_t DEFAULT_UDP_ENTRY_SIZE = DEFAULT_UDP_MTU - std::max(LogReqMsg::STATIC_SIZE, LogRespMsg::STATIC_SIZE);

    LogsDB() = delete;

    // On start we verify last released data is less than 1.5 * PARTITION_TIME_SPAN old to guarantee we can catchup.
    LogsDB(
        Logger& logger, std::shared_ptr<XmonAgent>& xmon,
        SharedRocksDB& sharedDB,
        ReplicaId replicaId,
        LogIdx lastRead,
        bool noReplication,
        bool avoidBeingLeader);

    ~LogsDB();

    void close();

    void flush(bool sync);

    void processIncomingMessages(std::vector<LogsDBRequest>& requests, std::vector<LogsDBResponse>& responses);
    void getOutgoingMessages(std::vector<LogsDBRequest*>& requests, std::vector<LogsDBResponse>& responses);

    bool isLeader() const;

    TernError appendEntries(std::vector<LogsDBLogEntry>& entries);

    // returns index of last entry available for read
    LogIdx getLastContinuous() const;
    void readEntries(std::vector<LogsDBLogEntry>& entries, size_t maxEntries = IN_FLIGHT_APPEND_WINDOW);

    // Takes a sorted vector of log inxices and returns the corresponding entries
    // If an entry is missing and empty entry with LogIdx 0 is returned
    void readIndexedEntries(const std::vector<LogIdx>& indices, std::vector<LogsDBLogEntry>& entries) const;

    Duration getNextTimeout() const;

    LogIdx getLastReleased() const;


    const LogsDBStats& getStats() const;

    static std::vector<rocksdb::ColumnFamilyDescriptor> getColumnFamilyDescriptors();
    static void clearAllData(SharedRocksDB& shardDB);
private:
    friend class LogsDBTools;
    static void _getUnreleasedLogEntries(Env& env, SharedRocksDB& sharedDB, LogIdx& lastReleasedOut, std::vector<LogIdx>& unreleasedLogEntriesOut);
    static void _getLogEntries(Env& env, SharedRocksDB& sharedDB, LogIdx start, size_t count, std::vector<LogsDBLogEntry>& logEntriesOut);
    LogsDBImpl* _impl;
};
