// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// Automatically generated with go run bincodegen.
// Run `go generate ./...` from the go/ directory to regenerate it.
#pragma once
#include "Msgs.hpp"
#include <algorithm>
#include <utility>
#include <variant>
#include "Time.hpp"

enum class TernError : uint16_t {
    NO_ERROR = 0,
    INTERNAL_ERROR = 10,
    FATAL_ERROR = 11,
    TIMEOUT = 12,
    MALFORMED_REQUEST = 13,
    MALFORMED_RESPONSE = 14,
    NOT_AUTHORISED = 15,
    UNRECOGNIZED_REQUEST = 16,
    FILE_NOT_FOUND = 17,
    DIRECTORY_NOT_FOUND = 18,
    NAME_NOT_FOUND = 19,
    EDGE_NOT_FOUND = 20,
    EDGE_IS_LOCKED = 21,
    TYPE_IS_DIRECTORY = 22,
    TYPE_IS_NOT_DIRECTORY = 23,
    BAD_COOKIE = 24,
    INCONSISTENT_STORAGE_CLASS_PARITY = 25,
    LAST_SPAN_STATE_NOT_CLEAN = 26,
    COULD_NOT_PICK_BLOCK_SERVICES = 27,
    BAD_SPAN_BODY = 28,
    SPAN_NOT_FOUND = 29,
    BLOCK_SERVICE_NOT_FOUND = 30,
    CANNOT_CERTIFY_BLOCKLESS_SPAN = 31,
    BAD_NUMBER_OF_BLOCKS_PROOFS = 32,
    BAD_BLOCK_PROOF = 33,
    CANNOT_OVERRIDE_NAME = 34,
    NAME_IS_LOCKED = 35,
    MTIME_IS_TOO_RECENT = 36,
    MISMATCHING_TARGET = 37,
    MISMATCHING_OWNER = 38,
    MISMATCHING_CREATION_TIME = 39,
    DIRECTORY_NOT_EMPTY = 40,
    FILE_IS_TRANSIENT = 41,
    OLD_DIRECTORY_NOT_FOUND = 42,
    NEW_DIRECTORY_NOT_FOUND = 43,
    LOOP_IN_DIRECTORY_RENAME = 44,
    DIRECTORY_HAS_OWNER = 45,
    FILE_IS_NOT_TRANSIENT = 46,
    FILE_NOT_EMPTY = 47,
    CANNOT_REMOVE_ROOT_DIRECTORY = 48,
    FILE_EMPTY = 49,
    CANNOT_REMOVE_DIRTY_SPAN = 50,
    BAD_SHARD = 51,
    BAD_NAME = 52,
    MORE_RECENT_SNAPSHOT_EDGE = 53,
    MORE_RECENT_CURRENT_EDGE = 54,
    BAD_DIRECTORY_INFO = 55,
    DEADLINE_NOT_PASSED = 56,
    SAME_SOURCE_AND_DESTINATION = 57,
    SAME_DIRECTORIES = 58,
    SAME_SHARD = 59,
    BAD_PROTOCOL_VERSION = 60,
    BAD_CERTIFICATE = 61,
    BLOCK_TOO_RECENT_FOR_DELETION = 62,
    BLOCK_FETCH_OUT_OF_BOUNDS = 63,
    BAD_BLOCK_CRC = 64,
    BLOCK_TOO_BIG = 65,
    BLOCK_NOT_FOUND = 66,
    CANNOT_UNSET_DECOMMISSIONED = 67,
    CANNOT_REGISTER_DECOMMISSIONED_OR_STALE = 68,
    BLOCK_TOO_OLD_FOR_WRITE = 69,
    BLOCK_IO_ERROR_DEVICE = 70,
    BLOCK_IO_ERROR_FILE = 71,
    INVALID_REPLICA = 72,
    DIFFERENT_ADDRS_INFO = 73,
    LEADER_PREEMPTED = 74,
    LOG_ENTRY_MISSING = 75,
    LOG_ENTRY_TRIMMED = 76,
    LOG_ENTRY_UNRELEASED = 77,
    LOG_ENTRY_RELEASED = 78,
    AUTO_DECOMMISSION_FORBIDDEN = 79,
    INCONSISTENT_BLOCK_SERVICE_REGISTRATION = 80,
    SWAP_BLOCKS_INLINE_STORAGE = 81,
    SWAP_BLOCKS_MISMATCHING_SIZE = 82,
    SWAP_BLOCKS_MISMATCHING_STATE = 83,
    SWAP_BLOCKS_MISMATCHING_CRC = 84,
    SWAP_BLOCKS_DUPLICATE_BLOCK_SERVICE = 85,
    SWAP_SPANS_INLINE_STORAGE = 86,
    SWAP_SPANS_MISMATCHING_SIZE = 87,
    SWAP_SPANS_NOT_CLEAN = 88,
    SWAP_SPANS_MISMATCHING_CRC = 89,
    SWAP_SPANS_MISMATCHING_BLOCKS = 90,
    EDGE_NOT_OWNED = 91,
    CANNOT_CREATE_DB_SNAPSHOT = 92,
    BLOCK_SIZE_NOT_MULTIPLE_OF_PAGE_SIZE = 93,
    SWAP_BLOCKS_DUPLICATE_FAILURE_DOMAIN = 94,
    TRANSIENT_LOCATION_COUNT = 95,
    ADD_SPAN_LOCATION_INLINE_STORAGE = 96,
    ADD_SPAN_LOCATION_MISMATCHING_SIZE = 97,
    ADD_SPAN_LOCATION_NOT_CLEAN = 98,
    ADD_SPAN_LOCATION_MISMATCHING_CRC = 99,
    ADD_SPAN_LOCATION_EXISTS = 100,
    SWAP_BLOCKS_MISMATCHING_LOCATION = 101,
    LOCATION_EXISTS = 102,
    LOCATION_NOT_FOUND = 103,
};

std::ostream& operator<<(std::ostream& out, TernError err);

const std::vector<TernError> allTernErrors {
    TernError::INTERNAL_ERROR,
    TernError::FATAL_ERROR,
    TernError::TIMEOUT,
    TernError::MALFORMED_REQUEST,
    TernError::MALFORMED_RESPONSE,
    TernError::NOT_AUTHORISED,
    TernError::UNRECOGNIZED_REQUEST,
    TernError::FILE_NOT_FOUND,
    TernError::DIRECTORY_NOT_FOUND,
    TernError::NAME_NOT_FOUND,
    TernError::EDGE_NOT_FOUND,
    TernError::EDGE_IS_LOCKED,
    TernError::TYPE_IS_DIRECTORY,
    TernError::TYPE_IS_NOT_DIRECTORY,
    TernError::BAD_COOKIE,
    TernError::INCONSISTENT_STORAGE_CLASS_PARITY,
    TernError::LAST_SPAN_STATE_NOT_CLEAN,
    TernError::COULD_NOT_PICK_BLOCK_SERVICES,
    TernError::BAD_SPAN_BODY,
    TernError::SPAN_NOT_FOUND,
    TernError::BLOCK_SERVICE_NOT_FOUND,
    TernError::CANNOT_CERTIFY_BLOCKLESS_SPAN,
    TernError::BAD_NUMBER_OF_BLOCKS_PROOFS,
    TernError::BAD_BLOCK_PROOF,
    TernError::CANNOT_OVERRIDE_NAME,
    TernError::NAME_IS_LOCKED,
    TernError::MTIME_IS_TOO_RECENT,
    TernError::MISMATCHING_TARGET,
    TernError::MISMATCHING_OWNER,
    TernError::MISMATCHING_CREATION_TIME,
    TernError::DIRECTORY_NOT_EMPTY,
    TernError::FILE_IS_TRANSIENT,
    TernError::OLD_DIRECTORY_NOT_FOUND,
    TernError::NEW_DIRECTORY_NOT_FOUND,
    TernError::LOOP_IN_DIRECTORY_RENAME,
    TernError::DIRECTORY_HAS_OWNER,
    TernError::FILE_IS_NOT_TRANSIENT,
    TernError::FILE_NOT_EMPTY,
    TernError::CANNOT_REMOVE_ROOT_DIRECTORY,
    TernError::FILE_EMPTY,
    TernError::CANNOT_REMOVE_DIRTY_SPAN,
    TernError::BAD_SHARD,
    TernError::BAD_NAME,
    TernError::MORE_RECENT_SNAPSHOT_EDGE,
    TernError::MORE_RECENT_CURRENT_EDGE,
    TernError::BAD_DIRECTORY_INFO,
    TernError::DEADLINE_NOT_PASSED,
    TernError::SAME_SOURCE_AND_DESTINATION,
    TernError::SAME_DIRECTORIES,
    TernError::SAME_SHARD,
    TernError::BAD_PROTOCOL_VERSION,
    TernError::BAD_CERTIFICATE,
    TernError::BLOCK_TOO_RECENT_FOR_DELETION,
    TernError::BLOCK_FETCH_OUT_OF_BOUNDS,
    TernError::BAD_BLOCK_CRC,
    TernError::BLOCK_TOO_BIG,
    TernError::BLOCK_NOT_FOUND,
    TernError::CANNOT_UNSET_DECOMMISSIONED,
    TernError::CANNOT_REGISTER_DECOMMISSIONED_OR_STALE,
    TernError::BLOCK_TOO_OLD_FOR_WRITE,
    TernError::BLOCK_IO_ERROR_DEVICE,
    TernError::BLOCK_IO_ERROR_FILE,
    TernError::INVALID_REPLICA,
    TernError::DIFFERENT_ADDRS_INFO,
    TernError::LEADER_PREEMPTED,
    TernError::LOG_ENTRY_MISSING,
    TernError::LOG_ENTRY_TRIMMED,
    TernError::LOG_ENTRY_UNRELEASED,
    TernError::LOG_ENTRY_RELEASED,
    TernError::AUTO_DECOMMISSION_FORBIDDEN,
    TernError::INCONSISTENT_BLOCK_SERVICE_REGISTRATION,
    TernError::SWAP_BLOCKS_INLINE_STORAGE,
    TernError::SWAP_BLOCKS_MISMATCHING_SIZE,
    TernError::SWAP_BLOCKS_MISMATCHING_STATE,
    TernError::SWAP_BLOCKS_MISMATCHING_CRC,
    TernError::SWAP_BLOCKS_DUPLICATE_BLOCK_SERVICE,
    TernError::SWAP_SPANS_INLINE_STORAGE,
    TernError::SWAP_SPANS_MISMATCHING_SIZE,
    TernError::SWAP_SPANS_NOT_CLEAN,
    TernError::SWAP_SPANS_MISMATCHING_CRC,
    TernError::SWAP_SPANS_MISMATCHING_BLOCKS,
    TernError::EDGE_NOT_OWNED,
    TernError::CANNOT_CREATE_DB_SNAPSHOT,
    TernError::BLOCK_SIZE_NOT_MULTIPLE_OF_PAGE_SIZE,
    TernError::SWAP_BLOCKS_DUPLICATE_FAILURE_DOMAIN,
    TernError::TRANSIENT_LOCATION_COUNT,
    TernError::ADD_SPAN_LOCATION_INLINE_STORAGE,
    TernError::ADD_SPAN_LOCATION_MISMATCHING_SIZE,
    TernError::ADD_SPAN_LOCATION_NOT_CLEAN,
    TernError::ADD_SPAN_LOCATION_MISMATCHING_CRC,
    TernError::ADD_SPAN_LOCATION_EXISTS,
    TernError::SWAP_BLOCKS_MISMATCHING_LOCATION,
    TernError::LOCATION_EXISTS,
    TernError::LOCATION_NOT_FOUND,
};

constexpr int maxTernError = 104;

enum class ShardMessageKind : uint8_t {
    ERROR = 0,
    LOOKUP = 1,
    STAT_FILE = 2,
    STAT_DIRECTORY = 4,
    READ_DIR = 5,
    CONSTRUCT_FILE = 6,
    ADD_SPAN_INITIATE = 7,
    ADD_SPAN_CERTIFY = 8,
    LINK_FILE = 9,
    SOFT_UNLINK_FILE = 10,
    LOCAL_FILE_SPANS = 11,
    SAME_DIRECTORY_RENAME = 12,
    ADD_INLINE_SPAN = 16,
    SET_TIME = 17,
    FULL_READ_DIR = 115,
    MOVE_SPAN = 123,
    REMOVE_NON_OWNED_EDGE = 116,
    SAME_SHARD_HARD_FILE_UNLINK = 117,
    STAT_TRANSIENT_FILE = 3,
    SHARD_SNAPSHOT = 18,
    FILE_SPANS = 20,
    ADD_SPAN_LOCATION = 21,
    SCRAP_TRANSIENT_FILE = 22,
    SET_DIRECTORY_INFO = 13,
    GET_LINK_ENTRIES = 14,
    WAIT_STATE_APPLIED = 15,
    VISIT_DIRECTORIES = 112,
    VISIT_FILES = 113,
    VISIT_TRANSIENT_FILES = 114,
    REMOVE_SPAN_INITIATE = 118,
    REMOVE_SPAN_CERTIFY = 119,
    SWAP_BLOCKS = 120,
    BLOCK_SERVICE_FILES = 121,
    REMOVE_INODE = 122,
    ADD_SPAN_INITIATE_WITH_REFERENCE = 124,
    REMOVE_ZERO_BLOCK_SERVICE_FILES = 125,
    SWAP_SPANS = 126,
    SAME_DIRECTORY_RENAME_SNAPSHOT = 127,
    ADD_SPAN_AT_LOCATION_INITIATE = 19,
    CREATE_DIRECTORY_INODE = 128,
    SET_DIRECTORY_OWNER = 129,
    REMOVE_DIRECTORY_OWNER = 137,
    CREATE_LOCKED_CURRENT_EDGE = 130,
    LOCK_CURRENT_EDGE = 131,
    UNLOCK_CURRENT_EDGE = 132,
    REMOVE_OWNED_SNAPSHOT_FILE_EDGE = 134,
    MAKE_FILE_TRANSIENT = 135,
    EMPTY = 255,
};

const std::vector<ShardMessageKind> allShardMessageKind {
    ShardMessageKind::LOOKUP,
    ShardMessageKind::STAT_FILE,
    ShardMessageKind::STAT_DIRECTORY,
    ShardMessageKind::READ_DIR,
    ShardMessageKind::CONSTRUCT_FILE,
    ShardMessageKind::ADD_SPAN_INITIATE,
    ShardMessageKind::ADD_SPAN_CERTIFY,
    ShardMessageKind::LINK_FILE,
    ShardMessageKind::SOFT_UNLINK_FILE,
    ShardMessageKind::LOCAL_FILE_SPANS,
    ShardMessageKind::SAME_DIRECTORY_RENAME,
    ShardMessageKind::ADD_INLINE_SPAN,
    ShardMessageKind::SET_TIME,
    ShardMessageKind::FULL_READ_DIR,
    ShardMessageKind::MOVE_SPAN,
    ShardMessageKind::REMOVE_NON_OWNED_EDGE,
    ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK,
    ShardMessageKind::STAT_TRANSIENT_FILE,
    ShardMessageKind::SHARD_SNAPSHOT,
    ShardMessageKind::FILE_SPANS,
    ShardMessageKind::ADD_SPAN_LOCATION,
    ShardMessageKind::SCRAP_TRANSIENT_FILE,
    ShardMessageKind::SET_DIRECTORY_INFO,
    ShardMessageKind::GET_LINK_ENTRIES,
    ShardMessageKind::WAIT_STATE_APPLIED,
    ShardMessageKind::VISIT_DIRECTORIES,
    ShardMessageKind::VISIT_FILES,
    ShardMessageKind::VISIT_TRANSIENT_FILES,
    ShardMessageKind::REMOVE_SPAN_INITIATE,
    ShardMessageKind::REMOVE_SPAN_CERTIFY,
    ShardMessageKind::SWAP_BLOCKS,
    ShardMessageKind::BLOCK_SERVICE_FILES,
    ShardMessageKind::REMOVE_INODE,
    ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE,
    ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES,
    ShardMessageKind::SWAP_SPANS,
    ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT,
    ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE,
    ShardMessageKind::CREATE_DIRECTORY_INODE,
    ShardMessageKind::SET_DIRECTORY_OWNER,
    ShardMessageKind::REMOVE_DIRECTORY_OWNER,
    ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE,
    ShardMessageKind::LOCK_CURRENT_EDGE,
    ShardMessageKind::UNLOCK_CURRENT_EDGE,
    ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE,
    ShardMessageKind::MAKE_FILE_TRANSIENT,
};

constexpr int maxShardMessageKind = 137;

std::ostream& operator<<(std::ostream& out, ShardMessageKind kind);

enum class CDCMessageKind : uint8_t {
    ERROR = 0,
    MAKE_DIRECTORY = 1,
    RENAME_FILE = 2,
    SOFT_UNLINK_DIRECTORY = 3,
    RENAME_DIRECTORY = 4,
    CROSS_SHARD_HARD_UNLINK_FILE = 6,
    HARD_UNLINK_DIRECTORY = 5,
    CDC_SNAPSHOT = 7,
    EMPTY = 255,
};

const std::vector<CDCMessageKind> allCDCMessageKind {
    CDCMessageKind::MAKE_DIRECTORY,
    CDCMessageKind::RENAME_FILE,
    CDCMessageKind::SOFT_UNLINK_DIRECTORY,
    CDCMessageKind::RENAME_DIRECTORY,
    CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE,
    CDCMessageKind::HARD_UNLINK_DIRECTORY,
    CDCMessageKind::CDC_SNAPSHOT,
};

constexpr int maxCDCMessageKind = 7;

std::ostream& operator<<(std::ostream& out, CDCMessageKind kind);

enum class RegistryMessageKind : uint8_t {
    ERROR = 0,
    LOCAL_SHARDS = 3,
    LOCAL_CDC = 7,
    INFO = 8,
    REGISTRY = 15,
    LOCAL_CHANGED_BLOCK_SERVICES = 34,
    CREATE_LOCATION = 1,
    RENAME_LOCATION = 2,
    REGISTER_SHARD = 4,
    LOCATIONS = 5,
    REGISTER_CDC = 6,
    SET_BLOCK_SERVICE_FLAGS = 9,
    REGISTER_BLOCK_SERVICES = 10,
    CHANGED_BLOCK_SERVICES_AT_LOCATION = 11,
    SHARDS_AT_LOCATION = 12,
    CDC_AT_LOCATION = 13,
    REGISTER_REGISTRY = 14,
    ALL_REGISTRY_REPLICAS = 16,
    SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED = 17,
    CDC_REPLICAS_DE_PR_EC_AT_ED = 19,
    ALL_SHARDS = 20,
    DECOMMISSION_BLOCK_SERVICE = 21,
    MOVE_SHARD_LEADER = 22,
    CLEAR_SHARD_INFO = 23,
    SHARD_BLOCK_SERVICES = 24,
    ALL_CDC = 25,
    ERASE_DECOMMISSIONED_BLOCK = 32,
    ALL_BLOCK_SERVICES_DEPRECATED = 33,
    ALL_BLOCK_SERVICES = 40,
    MOVE_CDC_LEADER = 35,
    CLEAR_CDC_INFO = 36,
    UPDATE_BLOCK_SERVICE_PATH = 37,
    SET_BLOCK_SERVICE_HAS_FILES = 38,
    BLOCK_SERVICES_NEEDING_MIGRATION = 39,
    EMPTY = 255,
};

const std::vector<RegistryMessageKind> allRegistryMessageKind {
    RegistryMessageKind::LOCAL_SHARDS,
    RegistryMessageKind::LOCAL_CDC,
    RegistryMessageKind::INFO,
    RegistryMessageKind::REGISTRY,
    RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES,
    RegistryMessageKind::CREATE_LOCATION,
    RegistryMessageKind::RENAME_LOCATION,
    RegistryMessageKind::REGISTER_SHARD,
    RegistryMessageKind::LOCATIONS,
    RegistryMessageKind::REGISTER_CDC,
    RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS,
    RegistryMessageKind::REGISTER_BLOCK_SERVICES,
    RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION,
    RegistryMessageKind::SHARDS_AT_LOCATION,
    RegistryMessageKind::CDC_AT_LOCATION,
    RegistryMessageKind::REGISTER_REGISTRY,
    RegistryMessageKind::ALL_REGISTRY_REPLICAS,
    RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED,
    RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED,
    RegistryMessageKind::ALL_SHARDS,
    RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE,
    RegistryMessageKind::MOVE_SHARD_LEADER,
    RegistryMessageKind::CLEAR_SHARD_INFO,
    RegistryMessageKind::SHARD_BLOCK_SERVICES,
    RegistryMessageKind::ALL_CDC,
    RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK,
    RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED,
    RegistryMessageKind::ALL_BLOCK_SERVICES,
    RegistryMessageKind::MOVE_CDC_LEADER,
    RegistryMessageKind::CLEAR_CDC_INFO,
    RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH,
    RegistryMessageKind::SET_BLOCK_SERVICE_HAS_FILES,
    RegistryMessageKind::BLOCK_SERVICES_NEEDING_MIGRATION,
};

constexpr int maxRegistryMessageKind = 40;

std::ostream& operator<<(std::ostream& out, RegistryMessageKind kind);

enum class BlocksMessageKind : uint8_t {
    ERROR = 0,
    FETCH_BLOCK = 2,
    WRITE_BLOCK = 3,
    FETCH_BLOCK_WITH_CRC = 4,
    ERASE_BLOCK = 1,
    TEST_WRITE = 5,
    CHECK_BLOCK = 6,
    EMPTY = 255,
};

const std::vector<BlocksMessageKind> allBlocksMessageKind {
    BlocksMessageKind::FETCH_BLOCK,
    BlocksMessageKind::WRITE_BLOCK,
    BlocksMessageKind::FETCH_BLOCK_WITH_CRC,
    BlocksMessageKind::ERASE_BLOCK,
    BlocksMessageKind::TEST_WRITE,
    BlocksMessageKind::CHECK_BLOCK,
};

constexpr int maxBlocksMessageKind = 6;

std::ostream& operator<<(std::ostream& out, BlocksMessageKind kind);

enum class LogMessageKind : uint8_t {
    ERROR = 0,
    LOG_WRITE = 1,
    RELEASE = 2,
    LOG_READ = 3,
    NEW_LEADER = 4,
    NEW_LEADER_CONFIRM = 5,
    LOG_RECOVERY_READ = 6,
    LOG_RECOVERY_WRITE = 7,
    EMPTY = 255,
};

const std::vector<LogMessageKind> allLogMessageKind {
    LogMessageKind::LOG_WRITE,
    LogMessageKind::RELEASE,
    LogMessageKind::LOG_READ,
    LogMessageKind::NEW_LEADER,
    LogMessageKind::NEW_LEADER_CONFIRM,
    LogMessageKind::LOG_RECOVERY_READ,
    LogMessageKind::LOG_RECOVERY_WRITE,
};

constexpr int maxLogMessageKind = 7;

std::ostream& operator<<(std::ostream& out, LogMessageKind kind);

struct FailureDomain {
    BincodeFixedBytes<16> name;

    static constexpr uint16_t STATIC_SIZE = BincodeFixedBytes<16>::STATIC_SIZE; // name

    FailureDomain() { clear(); }
    explicit FailureDomain(BincodeFixedBytes<16>&& name_) : name(std::move(name_)) {}
    FailureDomain(const FailureDomain&) = default;
    FailureDomain& operator=(const FailureDomain&) = default;
    FailureDomain(FailureDomain&&) = default;
    FailureDomain& operator=(FailureDomain&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += BincodeFixedBytes<16>::STATIC_SIZE; // name
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FailureDomain&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FailureDomain& x);

struct DirectoryInfoEntry {
    uint8_t tag;
    BincodeBytes body;

    static constexpr uint16_t STATIC_SIZE = 1 + BincodeBytes::STATIC_SIZE; // tag + body

    DirectoryInfoEntry() { clear(); }
    explicit DirectoryInfoEntry(uint8_t tag_, BincodeBytes&& body_) : tag(tag_), body(std::move(body_)) {}
    DirectoryInfoEntry(const DirectoryInfoEntry&) = default;
    DirectoryInfoEntry& operator=(const DirectoryInfoEntry&) = default;
    DirectoryInfoEntry(DirectoryInfoEntry&&) = default;
    DirectoryInfoEntry& operator=(DirectoryInfoEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // tag
        _size += body.packedSize(); // body
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const DirectoryInfoEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const DirectoryInfoEntry& x);

struct DirectoryInfo {
    BincodeList<DirectoryInfoEntry> entries;

    static constexpr uint16_t STATIC_SIZE = BincodeList<DirectoryInfoEntry>::STATIC_SIZE; // entries

    DirectoryInfo() { clear(); }
    explicit DirectoryInfo(BincodeList<DirectoryInfoEntry>&& entries_) : entries(std::move(entries_)) {}
    DirectoryInfo(const DirectoryInfo&) = default;
    DirectoryInfo& operator=(const DirectoryInfo&) = default;
    DirectoryInfo(DirectoryInfo&&) = default;
    DirectoryInfo& operator=(DirectoryInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += entries.packedSize(); // entries
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const DirectoryInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const DirectoryInfo& x);

struct CurrentEdge {
    InodeId targetId;
    uint64_t nameHash;
    BincodeBytes name;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8; // targetId + nameHash + name + creationTime

    CurrentEdge() { clear(); }
    explicit CurrentEdge(InodeId targetId_, uint64_t nameHash_, BincodeBytes&& name_, TernTime creationTime_) : targetId(targetId_), nameHash(nameHash_), name(std::move(name_)), creationTime(creationTime_) {}
    CurrentEdge(const CurrentEdge&) = default;
    CurrentEdge& operator=(const CurrentEdge&) = default;
    CurrentEdge(CurrentEdge&&) = default;
    CurrentEdge& operator=(CurrentEdge&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // targetId
        _size += 8; // nameHash
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CurrentEdge&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CurrentEdge& x);

struct AddSpanInitiateBlockInfo {
    AddrsInfo blockServiceAddrs;
    BlockServiceId blockServiceId;
    FailureDomain blockServiceFailureDomain;
    uint64_t blockId;
    BincodeFixedBytes<8> certificate;

    static constexpr uint16_t STATIC_SIZE = AddrsInfo::STATIC_SIZE + 8 + FailureDomain::STATIC_SIZE + 8 + BincodeFixedBytes<8>::STATIC_SIZE; // blockServiceAddrs + blockServiceId + blockServiceFailureDomain + blockId + certificate

    AddSpanInitiateBlockInfo() { clear(); }
    explicit AddSpanInitiateBlockInfo(AddrsInfo blockServiceAddrs_, BlockServiceId blockServiceId_, FailureDomain&& blockServiceFailureDomain_, uint64_t blockId_, BincodeFixedBytes<8>&& certificate_) : blockServiceAddrs(blockServiceAddrs_), blockServiceId(blockServiceId_), blockServiceFailureDomain(std::move(blockServiceFailureDomain_)), blockId(blockId_), certificate(std::move(certificate_)) {}
    AddSpanInitiateBlockInfo(const AddSpanInitiateBlockInfo&) = default;
    AddSpanInitiateBlockInfo& operator=(const AddSpanInitiateBlockInfo&) = default;
    AddSpanInitiateBlockInfo(AddSpanInitiateBlockInfo&&) = default;
    AddSpanInitiateBlockInfo& operator=(AddSpanInitiateBlockInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += blockServiceAddrs.packedSize(); // blockServiceAddrs
        _size += 8; // blockServiceId
        _size += blockServiceFailureDomain.packedSize(); // blockServiceFailureDomain
        _size += 8; // blockId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // certificate
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanInitiateBlockInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanInitiateBlockInfo& x);

struct RemoveSpanInitiateBlockInfo {
    AddrsInfo blockServiceAddrs;
    BlockServiceId blockServiceId;
    FailureDomain blockServiceFailureDomain;
    BlockServiceFlags blockServiceFlags;
    uint64_t blockId;
    BincodeFixedBytes<8> certificate;

    static constexpr uint16_t STATIC_SIZE = AddrsInfo::STATIC_SIZE + 8 + FailureDomain::STATIC_SIZE + 1 + 8 + BincodeFixedBytes<8>::STATIC_SIZE; // blockServiceAddrs + blockServiceId + blockServiceFailureDomain + blockServiceFlags + blockId + certificate

    RemoveSpanInitiateBlockInfo() { clear(); }
    explicit RemoveSpanInitiateBlockInfo(AddrsInfo blockServiceAddrs_, BlockServiceId blockServiceId_, FailureDomain&& blockServiceFailureDomain_, BlockServiceFlags blockServiceFlags_, uint64_t blockId_, BincodeFixedBytes<8>&& certificate_) : blockServiceAddrs(blockServiceAddrs_), blockServiceId(blockServiceId_), blockServiceFailureDomain(std::move(blockServiceFailureDomain_)), blockServiceFlags(blockServiceFlags_), blockId(blockId_), certificate(std::move(certificate_)) {}
    RemoveSpanInitiateBlockInfo(const RemoveSpanInitiateBlockInfo&) = default;
    RemoveSpanInitiateBlockInfo& operator=(const RemoveSpanInitiateBlockInfo&) = default;
    RemoveSpanInitiateBlockInfo(RemoveSpanInitiateBlockInfo&&) = default;
    RemoveSpanInitiateBlockInfo& operator=(RemoveSpanInitiateBlockInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += blockServiceAddrs.packedSize(); // blockServiceAddrs
        _size += 8; // blockServiceId
        _size += blockServiceFailureDomain.packedSize(); // blockServiceFailureDomain
        _size += 1; // blockServiceFlags
        _size += 8; // blockId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // certificate
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveSpanInitiateBlockInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveSpanInitiateBlockInfo& x);

struct BlockProof {
    uint64_t blockId;
    BincodeFixedBytes<8> proof;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE; // blockId + proof

    BlockProof() { clear(); }
    explicit BlockProof(uint64_t blockId_, BincodeFixedBytes<8>&& proof_) : blockId(blockId_), proof(std::move(proof_)) {}
    BlockProof(const BlockProof&) = default;
    BlockProof& operator=(const BlockProof&) = default;
    BlockProof(BlockProof&&) = default;
    BlockProof& operator=(BlockProof&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // blockId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // proof
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlockProof&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlockProof& x);

struct BlockService {
    AddrsInfo addrs;
    BlockServiceId id;
    BlockServiceFlags flags;

    static constexpr uint16_t STATIC_SIZE = AddrsInfo::STATIC_SIZE + 8 + 1; // addrs + id + flags

    BlockService() { clear(); }
    explicit BlockService(AddrsInfo addrs_, BlockServiceId id_, BlockServiceFlags flags_) : addrs(addrs_), id(id_), flags(flags_) {}
    BlockService(const BlockService&) = default;
    BlockService& operator=(const BlockService&) = default;
    BlockService(BlockService&&) = default;
    BlockService& operator=(BlockService&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += addrs.packedSize(); // addrs
        _size += 8; // id
        _size += 1; // flags
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlockService&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlockService& x);

struct ShardInfo {
    AddrsInfo addrs;
    TernTime lastSeen;

    static constexpr uint16_t STATIC_SIZE = AddrsInfo::STATIC_SIZE + 8; // addrs + lastSeen

    ShardInfo() { clear(); }
    explicit ShardInfo(AddrsInfo addrs_, TernTime lastSeen_) : addrs(addrs_), lastSeen(lastSeen_) {}
    ShardInfo(const ShardInfo&) = default;
    ShardInfo& operator=(const ShardInfo&) = default;
    ShardInfo(ShardInfo&&) = default;
    ShardInfo& operator=(ShardInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += addrs.packedSize(); // addrs
        _size += 8; // lastSeen
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ShardInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ShardInfo& x);

struct BlockPolicyEntry {
    uint8_t storageClass;
    uint32_t minSize;

    static constexpr uint16_t STATIC_SIZE = 1 + 4; // storageClass + minSize

    BlockPolicyEntry() { clear(); }
    explicit BlockPolicyEntry(uint8_t storageClass_, uint32_t minSize_) : storageClass(storageClass_), minSize(minSize_) {}
    BlockPolicyEntry(const BlockPolicyEntry&) = default;
    BlockPolicyEntry& operator=(const BlockPolicyEntry&) = default;
    BlockPolicyEntry(BlockPolicyEntry&&) = default;
    BlockPolicyEntry& operator=(BlockPolicyEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // storageClass
        _size += 4; // minSize
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlockPolicyEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlockPolicyEntry& x);

struct SpanPolicyEntry {
    uint32_t maxSize;
    Parity parity;

    static constexpr uint16_t STATIC_SIZE = 4 + 1; // maxSize + parity

    SpanPolicyEntry() { clear(); }
    explicit SpanPolicyEntry(uint32_t maxSize_, Parity parity_) : maxSize(maxSize_), parity(parity_) {}
    SpanPolicyEntry(const SpanPolicyEntry&) = default;
    SpanPolicyEntry& operator=(const SpanPolicyEntry&) = default;
    SpanPolicyEntry(SpanPolicyEntry&&) = default;
    SpanPolicyEntry& operator=(SpanPolicyEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 4; // maxSize
        _size += 1; // parity
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SpanPolicyEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SpanPolicyEntry& x);

struct StripePolicy {
    uint32_t targetStripeSize;

    static constexpr uint16_t STATIC_SIZE = 4; // targetStripeSize

    StripePolicy() { clear(); }
    explicit StripePolicy(uint32_t targetStripeSize_) : targetStripeSize(targetStripeSize_) {}
    StripePolicy(const StripePolicy&) = default;
    StripePolicy& operator=(const StripePolicy&) = default;
    StripePolicy(StripePolicy&&) = default;
    StripePolicy& operator=(StripePolicy&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 4; // targetStripeSize
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const StripePolicy&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const StripePolicy& x);

struct FetchedBlock {
    uint8_t blockServiceIx;
    uint64_t blockId;
    Crc crc;

    static constexpr uint16_t STATIC_SIZE = 1 + 8 + 4; // blockServiceIx + blockId + crc

    FetchedBlock() { clear(); }
    explicit FetchedBlock(uint8_t blockServiceIx_, uint64_t blockId_, Crc crc_) : blockServiceIx(blockServiceIx_), blockId(blockId_), crc(crc_) {}
    FetchedBlock(const FetchedBlock&) = default;
    FetchedBlock& operator=(const FetchedBlock&) = default;
    FetchedBlock(FetchedBlock&&) = default;
    FetchedBlock& operator=(FetchedBlock&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // blockServiceIx
        _size += 8; // blockId
        _size += 4; // crc
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchedBlock&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchedBlock& x);

struct FetchedSpanHeader {
    uint64_t byteOffset;
    uint32_t size;
    Crc crc;
    uint8_t storageClass;

    static constexpr uint16_t STATIC_SIZE = 8 + 4 + 4 + 1; // byteOffset + size + crc + storageClass

    FetchedSpanHeader() { clear(); }
    explicit FetchedSpanHeader(uint64_t byteOffset_, uint32_t size_, Crc crc_, uint8_t storageClass_) : byteOffset(byteOffset_), size(size_), crc(crc_), storageClass(storageClass_) {}
    FetchedSpanHeader(const FetchedSpanHeader&) = default;
    FetchedSpanHeader& operator=(const FetchedSpanHeader&) = default;
    FetchedSpanHeader(FetchedSpanHeader&&) = default;
    FetchedSpanHeader& operator=(FetchedSpanHeader&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // byteOffset
        _size += 4; // size
        _size += 4; // crc
        _size += 1; // storageClass
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchedSpanHeader&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchedSpanHeader& x);

struct FetchedInlineSpan {
    BincodeBytes body;

    static constexpr uint16_t STATIC_SIZE = BincodeBytes::STATIC_SIZE; // body

    FetchedInlineSpan() { clear(); }
    explicit FetchedInlineSpan(BincodeBytes&& body_) : body(std::move(body_)) {}
    FetchedInlineSpan(const FetchedInlineSpan&) = default;
    FetchedInlineSpan& operator=(const FetchedInlineSpan&) = default;
    FetchedInlineSpan(FetchedInlineSpan&&) = default;
    FetchedInlineSpan& operator=(FetchedInlineSpan&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += body.packedSize(); // body
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchedInlineSpan&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchedInlineSpan& x);

struct FetchedBlocksSpan {
    Parity parity;
    uint8_t stripes;
    uint32_t cellSize;
    BincodeList<FetchedBlock> blocks;
    BincodeList<Crc> stripesCrc;

    static constexpr uint16_t STATIC_SIZE = 1 + 1 + 4 + BincodeList<FetchedBlock>::STATIC_SIZE + BincodeList<Crc>::STATIC_SIZE; // parity + stripes + cellSize + blocks + stripesCrc

    FetchedBlocksSpan() { clear(); }
    explicit FetchedBlocksSpan(Parity parity_, uint8_t stripes_, uint32_t cellSize_, BincodeList<FetchedBlock>&& blocks_, BincodeList<Crc>&& stripesCrc_) : parity(parity_), stripes(stripes_), cellSize(cellSize_), blocks(std::move(blocks_)), stripesCrc(std::move(stripesCrc_)) {}
    FetchedBlocksSpan(const FetchedBlocksSpan&) = default;
    FetchedBlocksSpan& operator=(const FetchedBlocksSpan&) = default;
    FetchedBlocksSpan(FetchedBlocksSpan&&) = default;
    FetchedBlocksSpan& operator=(FetchedBlocksSpan&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // parity
        _size += 1; // stripes
        _size += 4; // cellSize
        _size += blocks.packedSize(); // blocks
        _size += stripesCrc.packedSize(); // stripesCrc
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchedBlocksSpan&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchedBlocksSpan& x);


struct FetchedSpan {
public:
    FetchedSpanHeader header;
private:
    std::variant<FetchedInlineSpan, FetchedBlocksSpan> body;

public:
    static constexpr uint16_t STATIC_SIZE = FetchedSpanHeader::STATIC_SIZE;

    FetchedSpan() { clear(); }

    const FetchedInlineSpan& getInlineSpan() const {
        ALWAYS_ASSERT(header.storageClass == INLINE_STORAGE);
        return std::get<0>(body);
    }
    FetchedInlineSpan& setInlineSpan() {
        header.storageClass = INLINE_STORAGE;
        return body.emplace<0>();
    }

    const FetchedBlocksSpan& getBlocksSpan() const {
        ALWAYS_ASSERT(header.storageClass > INLINE_STORAGE);
        return std::get<1>(body);
    }
    FetchedBlocksSpan& setBlocksSpan(uint8_t s) {
        ALWAYS_ASSERT(s > INLINE_STORAGE);
        header.storageClass = s;
        return body.emplace<1>();
    }

    void clear() {
        header.clear();
    }

    size_t packedSize() const {
        ALWAYS_ASSERT(header.storageClass != 0);
        size_t size = STATIC_SIZE;
        if (header.storageClass == INLINE_STORAGE) {
            size += getInlineSpan().packedSize();
        } else if (header.storageClass > INLINE_STORAGE) {
            size += getBlocksSpan().packedSize();
        }
        return size;
    }

    void pack(BincodeBuf& buf) const {
        ALWAYS_ASSERT(header.storageClass != 0);
        header.pack(buf);
        if (header.storageClass == INLINE_STORAGE) {
            getInlineSpan().pack(buf);
        } else if (header.storageClass > INLINE_STORAGE) {
            getBlocksSpan().pack(buf);
        }
    }

    void unpack(BincodeBuf& buf) {
        header.unpack(buf);
        if (header.storageClass == 0) {
            throw BINCODE_EXCEPTION("Unexpected EMPTY storage class");
        }
        if (header.storageClass == INLINE_STORAGE) {
            setInlineSpan().unpack(buf);
        } else if (header.storageClass > INLINE_STORAGE) {
            setBlocksSpan(header.storageClass).unpack(buf);
        }
    }

    bool operator==(const FetchedSpan& other) const {
        if (header != other.header) {
            return false;
        }
        ALWAYS_ASSERT(header.storageClass != 0);
        if (header.storageClass != other.header.storageClass) {
            return false;
        }
        if (header.storageClass == INLINE_STORAGE) {
            return getInlineSpan() == other.getInlineSpan();
        } else if (header.storageClass > INLINE_STORAGE) {
            return getBlocksSpan() == other.getBlocksSpan();
        }
        return true;
    }
};

UNUSED
static std::ostream& operator<<(std::ostream& out, const FetchedSpan& span) {
    ALWAYS_ASSERT(span.header.storageClass != 0);
    out << "FetchedSpan(" << "Header=" << span.header;
    if (span.header.storageClass == INLINE_STORAGE) {
        out << ", Body=" << span.getInlineSpan();
    } else if (span.header.storageClass > INLINE_STORAGE) {
        out << ", Body=" << span.getBlocksSpan();
    }
    out << ")";
    return out;
}
struct FetchedBlockServices {
    uint8_t locationId;
    uint8_t storageClass;
    Parity parity;
    uint8_t stripes;
    uint32_t cellSize;
    BincodeList<FetchedBlock> blocks;
    BincodeList<Crc> stripesCrc;

    static constexpr uint16_t STATIC_SIZE = 1 + 1 + 1 + 1 + 4 + BincodeList<FetchedBlock>::STATIC_SIZE + BincodeList<Crc>::STATIC_SIZE; // locationId + storageClass + parity + stripes + cellSize + blocks + stripesCrc

    FetchedBlockServices() { clear(); }
    explicit FetchedBlockServices(uint8_t locationId_, uint8_t storageClass_, Parity parity_, uint8_t stripes_, uint32_t cellSize_, BincodeList<FetchedBlock>&& blocks_, BincodeList<Crc>&& stripesCrc_) : locationId(locationId_), storageClass(storageClass_), parity(parity_), stripes(stripes_), cellSize(cellSize_), blocks(std::move(blocks_)), stripesCrc(std::move(stripesCrc_)) {}
    FetchedBlockServices(const FetchedBlockServices&) = default;
    FetchedBlockServices& operator=(const FetchedBlockServices&) = default;
    FetchedBlockServices(FetchedBlockServices&&) = default;
    FetchedBlockServices& operator=(FetchedBlockServices&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // locationId
        _size += 1; // storageClass
        _size += 1; // parity
        _size += 1; // stripes
        _size += 4; // cellSize
        _size += blocks.packedSize(); // blocks
        _size += stripesCrc.packedSize(); // stripesCrc
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchedBlockServices&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchedBlockServices& x);

struct FetchedLocations {
    BincodeList<FetchedBlockServices> locations;

    static constexpr uint16_t STATIC_SIZE = BincodeList<FetchedBlockServices>::STATIC_SIZE; // locations

    FetchedLocations() { clear(); }
    explicit FetchedLocations(BincodeList<FetchedBlockServices>&& locations_) : locations(std::move(locations_)) {}
    FetchedLocations(const FetchedLocations&) = default;
    FetchedLocations& operator=(const FetchedLocations&) = default;
    FetchedLocations(FetchedLocations&&) = default;
    FetchedLocations& operator=(FetchedLocations&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += locations.packedSize(); // locations
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchedLocations&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchedLocations& x);

struct FetchedSpanHeaderFull {
    uint64_t byteOffset;
    uint32_t size;
    Crc crc;
    bool isInline;

    static constexpr uint16_t STATIC_SIZE = 8 + 4 + 4 + 1; // byteOffset + size + crc + isInline

    FetchedSpanHeaderFull() { clear(); }
    explicit FetchedSpanHeaderFull(uint64_t byteOffset_, uint32_t size_, Crc crc_, bool isInline_) : byteOffset(byteOffset_), size(size_), crc(crc_), isInline(isInline_) {}
    FetchedSpanHeaderFull(const FetchedSpanHeaderFull&) = default;
    FetchedSpanHeaderFull& operator=(const FetchedSpanHeaderFull&) = default;
    FetchedSpanHeaderFull(FetchedSpanHeaderFull&&) = default;
    FetchedSpanHeaderFull& operator=(FetchedSpanHeaderFull&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // byteOffset
        _size += 4; // size
        _size += 4; // crc
        _size += 1; // isInline
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchedSpanHeaderFull&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchedSpanHeaderFull& x);


struct FetchedFullSpan {
public:
    FetchedSpanHeaderFull header;
private:
    std::variant<FetchedInlineSpan, FetchedLocations> body;

public:
    static constexpr uint16_t STATIC_SIZE = FetchedSpanHeaderFull::STATIC_SIZE;

    FetchedFullSpan() { clear(); }

    const FetchedInlineSpan& getInlineSpan() const {
        ALWAYS_ASSERT(header.isInline);
        return std::get<0>(body);
    }
    FetchedInlineSpan& setInlineSpan() {
        header.isInline = true;
        return body.emplace<0>();
    }

    const FetchedLocations& getLocations() const {
        ALWAYS_ASSERT(!header.isInline);
        return std::get<1>(body);
    }
    FetchedLocations& setLocations() {
        header.isInline = false;
        return body.emplace<1>();
    }

    void clear() {
        header.clear();
    }

    size_t packedSize() const {
        size_t size = STATIC_SIZE;
        if (header.isInline) {
            size += getInlineSpan().packedSize();
        } else {
            size += getLocations().packedSize();
        }
        return size;
    }

    void pack(BincodeBuf& buf) const {
        header.pack(buf);
        if (header.isInline) {
            getInlineSpan().pack(buf);
        } else {
            getLocations().pack(buf);
        }
    }

    void unpack(BincodeBuf& buf) {
        header.unpack(buf);
        if (header.isInline) {
            setInlineSpan().unpack(buf);
        } else {
            setLocations().unpack(buf);
        }
    }

    bool operator==(const FetchedFullSpan& other) const {
        if (header != other.header) {
            return false;
        }

        if (header.isInline) {
            return getInlineSpan() == other.getInlineSpan();
        } else {
            return getLocations() == other.getLocations();
        }
        return true;
    }
};

UNUSED
static std::ostream& operator<<(std::ostream& out, const FetchedFullSpan& span) {
    out << "FetchedFullSpan(" << "Header=" << span.header;
    if (span.header.isInline) {
        out << ", Body=" << span.getInlineSpan();
    } else {
        out << ", Body=" << span.getLocations();
    }
    out << ")";
    return out;
}
struct BlacklistEntry {
    FailureDomain failureDomain;
    BlockServiceId blockService;

    static constexpr uint16_t STATIC_SIZE = FailureDomain::STATIC_SIZE + 8; // failureDomain + blockService

    BlacklistEntry() { clear(); }
    explicit BlacklistEntry(FailureDomain&& failureDomain_, BlockServiceId blockService_) : failureDomain(std::move(failureDomain_)), blockService(blockService_) {}
    BlacklistEntry(const BlacklistEntry&) = default;
    BlacklistEntry& operator=(const BlacklistEntry&) = default;
    BlacklistEntry(BlacklistEntry&&) = default;
    BlacklistEntry& operator=(BlacklistEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += failureDomain.packedSize(); // failureDomain
        _size += 8; // blockService
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlacklistEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlacklistEntry& x);

struct Edge {
    bool current;
    InodeIdExtra targetId;
    uint64_t nameHash;
    BincodeBytes name;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 1 + 8 + 8 + BincodeBytes::STATIC_SIZE + 8; // current + targetId + nameHash + name + creationTime

    Edge() { clear(); }
    explicit Edge(bool current_, InodeIdExtra targetId_, uint64_t nameHash_, BincodeBytes&& name_, TernTime creationTime_) : current(current_), targetId(targetId_), nameHash(nameHash_), name(std::move(name_)), creationTime(creationTime_) {}
    Edge(const Edge&) = default;
    Edge& operator=(const Edge&) = default;
    Edge(Edge&&) = default;
    Edge& operator=(Edge&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // current
        _size += 8; // targetId
        _size += 8; // nameHash
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const Edge&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const Edge& x);

struct FullReadDirCursor {
    bool current;
    BincodeBytes startName;
    TernTime startTime;

    static constexpr uint16_t STATIC_SIZE = 1 + BincodeBytes::STATIC_SIZE + 8; // current + startName + startTime

    FullReadDirCursor() { clear(); }
    explicit FullReadDirCursor(bool current_, BincodeBytes&& startName_, TernTime startTime_) : current(current_), startName(std::move(startName_)), startTime(startTime_) {}
    FullReadDirCursor(const FullReadDirCursor&) = default;
    FullReadDirCursor& operator=(const FullReadDirCursor&) = default;
    FullReadDirCursor(FullReadDirCursor&&) = default;
    FullReadDirCursor& operator=(FullReadDirCursor&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // current
        _size += startName.packedSize(); // startName
        _size += 8; // startTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FullReadDirCursor&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FullReadDirCursor& x);

struct LinkEntry {
    InodeId id;
    InodeId ownerId;
    TernTime time;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 8; // id + ownerId + time

    LinkEntry() { clear(); }
    explicit LinkEntry(InodeId id_, InodeId ownerId_, TernTime time_) : id(id_), ownerId(ownerId_), time(time_) {}
    LinkEntry(const LinkEntry&) = default;
    LinkEntry& operator=(const LinkEntry&) = default;
    LinkEntry(LinkEntry&&) = default;
    LinkEntry& operator=(LinkEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 8; // ownerId
        _size += 8; // time
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LinkEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LinkEntry& x);

struct FullRegistryInfo {
    ReplicaId id;
    uint8_t locationId;
    bool isLeader;
    AddrsInfo addrs;
    TernTime lastSeen;

    static constexpr uint16_t STATIC_SIZE = 1 + 1 + 1 + AddrsInfo::STATIC_SIZE + 8; // id + locationId + isLeader + addrs + lastSeen

    FullRegistryInfo() { clear(); }
    explicit FullRegistryInfo(ReplicaId id_, uint8_t locationId_, bool isLeader_, AddrsInfo addrs_, TernTime lastSeen_) : id(id_), locationId(locationId_), isLeader(isLeader_), addrs(addrs_), lastSeen(lastSeen_) {}
    FullRegistryInfo(const FullRegistryInfo&) = default;
    FullRegistryInfo& operator=(const FullRegistryInfo&) = default;
    FullRegistryInfo(FullRegistryInfo&&) = default;
    FullRegistryInfo& operator=(FullRegistryInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // id
        _size += 1; // locationId
        _size += 1; // isLeader
        _size += addrs.packedSize(); // addrs
        _size += 8; // lastSeen
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FullRegistryInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FullRegistryInfo& x);

struct TransientFile {
    InodeId id;
    BincodeFixedBytes<8> cookie;
    TernTime deadlineTime;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE + 8; // id + cookie + deadlineTime

    TransientFile() { clear(); }
    explicit TransientFile(InodeId id_, BincodeFixedBytes<8>&& cookie_, TernTime deadlineTime_) : id(id_), cookie(std::move(cookie_)), deadlineTime(deadlineTime_) {}
    TransientFile(const TransientFile&) = default;
    TransientFile& operator=(const TransientFile&) = default;
    TransientFile(TransientFile&&) = default;
    TransientFile& operator=(TransientFile&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie
        _size += 8; // deadlineTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const TransientFile&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const TransientFile& x);

struct EntryNewBlockInfo {
    BlockServiceId blockServiceId;
    Crc crc;

    static constexpr uint16_t STATIC_SIZE = 8 + 4; // blockServiceId + crc

    EntryNewBlockInfo() { clear(); }
    explicit EntryNewBlockInfo(BlockServiceId blockServiceId_, Crc crc_) : blockServiceId(blockServiceId_), crc(crc_) {}
    EntryNewBlockInfo(const EntryNewBlockInfo&) = default;
    EntryNewBlockInfo& operator=(const EntryNewBlockInfo&) = default;
    EntryNewBlockInfo(EntryNewBlockInfo&&) = default;
    EntryNewBlockInfo& operator=(EntryNewBlockInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // blockServiceId
        _size += 4; // crc
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const EntryNewBlockInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const EntryNewBlockInfo& x);

struct BlockServiceDeprecatedInfo {
    BlockServiceId id;
    AddrsInfo addrs;
    uint8_t storageClass;
    FailureDomain failureDomain;
    BincodeFixedBytes<16> secretKey;
    BlockServiceFlags flags;
    uint64_t capacityBytes;
    uint64_t availableBytes;
    uint64_t blocks;
    BincodeBytes path;
    TernTime lastSeen;
    bool hasFiles;
    TernTime flagsLastChanged;

    static constexpr uint16_t STATIC_SIZE = 8 + AddrsInfo::STATIC_SIZE + 1 + FailureDomain::STATIC_SIZE + BincodeFixedBytes<16>::STATIC_SIZE + 1 + 8 + 8 + 8 + BincodeBytes::STATIC_SIZE + 8 + 1 + 8; // id + addrs + storageClass + failureDomain + secretKey + flags + capacityBytes + availableBytes + blocks + path + lastSeen + hasFiles + flagsLastChanged

    BlockServiceDeprecatedInfo() { clear(); }
    explicit BlockServiceDeprecatedInfo(BlockServiceId id_, AddrsInfo addrs_, uint8_t storageClass_, FailureDomain&& failureDomain_, BincodeFixedBytes<16>&& secretKey_, BlockServiceFlags flags_, uint64_t capacityBytes_, uint64_t availableBytes_, uint64_t blocks_, BincodeBytes&& path_, TernTime lastSeen_, bool hasFiles_, TernTime flagsLastChanged_) : id(id_), addrs(addrs_), storageClass(storageClass_), failureDomain(std::move(failureDomain_)), secretKey(std::move(secretKey_)), flags(flags_), capacityBytes(capacityBytes_), availableBytes(availableBytes_), blocks(blocks_), path(std::move(path_)), lastSeen(lastSeen_), hasFiles(hasFiles_), flagsLastChanged(flagsLastChanged_) {}
    BlockServiceDeprecatedInfo(const BlockServiceDeprecatedInfo&) = default;
    BlockServiceDeprecatedInfo& operator=(const BlockServiceDeprecatedInfo&) = default;
    BlockServiceDeprecatedInfo(BlockServiceDeprecatedInfo&&) = default;
    BlockServiceDeprecatedInfo& operator=(BlockServiceDeprecatedInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += addrs.packedSize(); // addrs
        _size += 1; // storageClass
        _size += failureDomain.packedSize(); // failureDomain
        _size += BincodeFixedBytes<16>::STATIC_SIZE; // secretKey
        _size += 1; // flags
        _size += 8; // capacityBytes
        _size += 8; // availableBytes
        _size += 8; // blocks
        _size += path.packedSize(); // path
        _size += 8; // lastSeen
        _size += 1; // hasFiles
        _size += 8; // flagsLastChanged
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlockServiceDeprecatedInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlockServiceDeprecatedInfo& x);

struct BlockServiceInfoShort {
    uint8_t locationId;
    FailureDomain failureDomain;
    BlockServiceId id;
    uint8_t storageClass;

    static constexpr uint16_t STATIC_SIZE = 1 + FailureDomain::STATIC_SIZE + 8 + 1; // locationId + failureDomain + id + storageClass

    BlockServiceInfoShort() { clear(); }
    explicit BlockServiceInfoShort(uint8_t locationId_, FailureDomain&& failureDomain_, BlockServiceId id_, uint8_t storageClass_) : locationId(locationId_), failureDomain(std::move(failureDomain_)), id(id_), storageClass(storageClass_) {}
    BlockServiceInfoShort(const BlockServiceInfoShort&) = default;
    BlockServiceInfoShort& operator=(const BlockServiceInfoShort&) = default;
    BlockServiceInfoShort(BlockServiceInfoShort&&) = default;
    BlockServiceInfoShort& operator=(BlockServiceInfoShort&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // locationId
        _size += failureDomain.packedSize(); // failureDomain
        _size += 8; // id
        _size += 1; // storageClass
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlockServiceInfoShort&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlockServiceInfoShort& x);

struct SpanPolicy {
    BincodeList<SpanPolicyEntry> entries;

    static constexpr uint16_t STATIC_SIZE = BincodeList<SpanPolicyEntry>::STATIC_SIZE; // entries

    SpanPolicy() { clear(); }
    explicit SpanPolicy(BincodeList<SpanPolicyEntry>&& entries_) : entries(std::move(entries_)) {}
    SpanPolicy(const SpanPolicy&) = default;
    SpanPolicy& operator=(const SpanPolicy&) = default;
    SpanPolicy(SpanPolicy&&) = default;
    SpanPolicy& operator=(SpanPolicy&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += entries.packedSize(); // entries
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SpanPolicy&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SpanPolicy& x);

struct BlockPolicy {
    BincodeList<BlockPolicyEntry> entries;

    static constexpr uint16_t STATIC_SIZE = BincodeList<BlockPolicyEntry>::STATIC_SIZE; // entries

    BlockPolicy() { clear(); }
    explicit BlockPolicy(BincodeList<BlockPolicyEntry>&& entries_) : entries(std::move(entries_)) {}
    BlockPolicy(const BlockPolicy&) = default;
    BlockPolicy& operator=(const BlockPolicy&) = default;
    BlockPolicy(BlockPolicy&&) = default;
    BlockPolicy& operator=(BlockPolicy&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += entries.packedSize(); // entries
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlockPolicy&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlockPolicy& x);

struct SnapshotPolicy {
    uint64_t deleteAfterTime;
    uint16_t deleteAfterVersions;

    static constexpr uint16_t STATIC_SIZE = 8 + 2; // deleteAfterTime + deleteAfterVersions

    SnapshotPolicy() { clear(); }
    explicit SnapshotPolicy(uint64_t deleteAfterTime_, uint16_t deleteAfterVersions_) : deleteAfterTime(deleteAfterTime_), deleteAfterVersions(deleteAfterVersions_) {}
    SnapshotPolicy(const SnapshotPolicy&) = default;
    SnapshotPolicy& operator=(const SnapshotPolicy&) = default;
    SnapshotPolicy(SnapshotPolicy&&) = default;
    SnapshotPolicy& operator=(SnapshotPolicy&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // deleteAfterTime
        _size += 2; // deleteAfterVersions
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SnapshotPolicy&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SnapshotPolicy& x);

struct FullShardInfo {
    ShardReplicaId id;
    bool isLeader;
    AddrsInfo addrs;
    TernTime lastSeen;
    uint8_t locationId;

    static constexpr uint16_t STATIC_SIZE = 2 + 1 + AddrsInfo::STATIC_SIZE + 8 + 1; // id + isLeader + addrs + lastSeen + locationId

    FullShardInfo() { clear(); }
    explicit FullShardInfo(ShardReplicaId id_, bool isLeader_, AddrsInfo addrs_, TernTime lastSeen_, uint8_t locationId_) : id(id_), isLeader(isLeader_), addrs(addrs_), lastSeen(lastSeen_), locationId(locationId_) {}
    FullShardInfo(const FullShardInfo&) = default;
    FullShardInfo& operator=(const FullShardInfo&) = default;
    FullShardInfo(FullShardInfo&&) = default;
    FullShardInfo& operator=(FullShardInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // id
        _size += 1; // isLeader
        _size += addrs.packedSize(); // addrs
        _size += 8; // lastSeen
        _size += 1; // locationId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FullShardInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FullShardInfo& x);

struct RegisterBlockServiceInfo {
    BlockServiceId id;
    uint8_t locationId;
    AddrsInfo addrs;
    uint8_t storageClass;
    FailureDomain failureDomain;
    BincodeFixedBytes<16> secretKey;
    BlockServiceFlags flags;
    uint8_t flagsMask;
    uint64_t capacityBytes;
    uint64_t availableBytes;
    uint64_t blocks;
    BincodeBytes path;

    static constexpr uint16_t STATIC_SIZE = 8 + 1 + AddrsInfo::STATIC_SIZE + 1 + FailureDomain::STATIC_SIZE + BincodeFixedBytes<16>::STATIC_SIZE + 1 + 1 + 8 + 8 + 8 + BincodeBytes::STATIC_SIZE; // id + locationId + addrs + storageClass + failureDomain + secretKey + flags + flagsMask + capacityBytes + availableBytes + blocks + path

    RegisterBlockServiceInfo() { clear(); }
    explicit RegisterBlockServiceInfo(BlockServiceId id_, uint8_t locationId_, AddrsInfo addrs_, uint8_t storageClass_, FailureDomain&& failureDomain_, BincodeFixedBytes<16>&& secretKey_, BlockServiceFlags flags_, uint8_t flagsMask_, uint64_t capacityBytes_, uint64_t availableBytes_, uint64_t blocks_, BincodeBytes&& path_) : id(id_), locationId(locationId_), addrs(addrs_), storageClass(storageClass_), failureDomain(std::move(failureDomain_)), secretKey(std::move(secretKey_)), flags(flags_), flagsMask(flagsMask_), capacityBytes(capacityBytes_), availableBytes(availableBytes_), blocks(blocks_), path(std::move(path_)) {}
    RegisterBlockServiceInfo(const RegisterBlockServiceInfo&) = default;
    RegisterBlockServiceInfo& operator=(const RegisterBlockServiceInfo&) = default;
    RegisterBlockServiceInfo(RegisterBlockServiceInfo&&) = default;
    RegisterBlockServiceInfo& operator=(RegisterBlockServiceInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 1; // locationId
        _size += addrs.packedSize(); // addrs
        _size += 1; // storageClass
        _size += failureDomain.packedSize(); // failureDomain
        _size += BincodeFixedBytes<16>::STATIC_SIZE; // secretKey
        _size += 1; // flags
        _size += 1; // flagsMask
        _size += 8; // capacityBytes
        _size += 8; // availableBytes
        _size += 8; // blocks
        _size += path.packedSize(); // path
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegisterBlockServiceInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegisterBlockServiceInfo& x);

struct FullBlockServiceInfo {
    BlockServiceId id;
    uint8_t locationId;
    AddrsInfo addrs;
    uint8_t storageClass;
    FailureDomain failureDomain;
    BincodeFixedBytes<16> secretKey;
    BlockServiceFlags flags;
    uint64_t capacityBytes;
    uint64_t availableBytes;
    uint64_t blocks;
    TernTime firstSeen;
    TernTime lastSeen;
    TernTime lastInfoChange;
    bool hasFiles;
    BincodeBytes path;

    static constexpr uint16_t STATIC_SIZE = 8 + 1 + AddrsInfo::STATIC_SIZE + 1 + FailureDomain::STATIC_SIZE + BincodeFixedBytes<16>::STATIC_SIZE + 1 + 8 + 8 + 8 + 8 + 8 + 8 + 1 + BincodeBytes::STATIC_SIZE; // id + locationId + addrs + storageClass + failureDomain + secretKey + flags + capacityBytes + availableBytes + blocks + firstSeen + lastSeen + lastInfoChange + hasFiles + path

    FullBlockServiceInfo() { clear(); }
    explicit FullBlockServiceInfo(BlockServiceId id_, uint8_t locationId_, AddrsInfo addrs_, uint8_t storageClass_, FailureDomain&& failureDomain_, BincodeFixedBytes<16>&& secretKey_, BlockServiceFlags flags_, uint64_t capacityBytes_, uint64_t availableBytes_, uint64_t blocks_, TernTime firstSeen_, TernTime lastSeen_, TernTime lastInfoChange_, bool hasFiles_, BincodeBytes&& path_) : id(id_), locationId(locationId_), addrs(addrs_), storageClass(storageClass_), failureDomain(std::move(failureDomain_)), secretKey(std::move(secretKey_)), flags(flags_), capacityBytes(capacityBytes_), availableBytes(availableBytes_), blocks(blocks_), firstSeen(firstSeen_), lastSeen(lastSeen_), lastInfoChange(lastInfoChange_), hasFiles(hasFiles_), path(std::move(path_)) {}
    FullBlockServiceInfo(const FullBlockServiceInfo&) = default;
    FullBlockServiceInfo& operator=(const FullBlockServiceInfo&) = default;
    FullBlockServiceInfo(FullBlockServiceInfo&&) = default;
    FullBlockServiceInfo& operator=(FullBlockServiceInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 1; // locationId
        _size += addrs.packedSize(); // addrs
        _size += 1; // storageClass
        _size += failureDomain.packedSize(); // failureDomain
        _size += BincodeFixedBytes<16>::STATIC_SIZE; // secretKey
        _size += 1; // flags
        _size += 8; // capacityBytes
        _size += 8; // availableBytes
        _size += 8; // blocks
        _size += 8; // firstSeen
        _size += 8; // lastSeen
        _size += 8; // lastInfoChange
        _size += 1; // hasFiles
        _size += path.packedSize(); // path
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FullBlockServiceInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FullBlockServiceInfo& x);

struct CdcInfo {
    ReplicaId replicaId;
    uint8_t locationId;
    bool isLeader;
    AddrsInfo addrs;
    TernTime lastSeen;

    static constexpr uint16_t STATIC_SIZE = 1 + 1 + 1 + AddrsInfo::STATIC_SIZE + 8; // replicaId + locationId + isLeader + addrs + lastSeen

    CdcInfo() { clear(); }
    explicit CdcInfo(ReplicaId replicaId_, uint8_t locationId_, bool isLeader_, AddrsInfo addrs_, TernTime lastSeen_) : replicaId(replicaId_), locationId(locationId_), isLeader(isLeader_), addrs(addrs_), lastSeen(lastSeen_) {}
    CdcInfo(const CdcInfo&) = default;
    CdcInfo& operator=(const CdcInfo&) = default;
    CdcInfo(CdcInfo&&) = default;
    CdcInfo& operator=(CdcInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // replicaId
        _size += 1; // locationId
        _size += 1; // isLeader
        _size += addrs.packedSize(); // addrs
        _size += 8; // lastSeen
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CdcInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CdcInfo& x);

struct LocationInfo {
    uint8_t id;
    BincodeBytes name;

    static constexpr uint16_t STATIC_SIZE = 1 + BincodeBytes::STATIC_SIZE; // id + name

    LocationInfo() { clear(); }
    explicit LocationInfo(uint8_t id_, BincodeBytes&& name_) : id(id_), name(std::move(name_)) {}
    LocationInfo(const LocationInfo&) = default;
    LocationInfo& operator=(const LocationInfo&) = default;
    LocationInfo(LocationInfo&&) = default;
    LocationInfo& operator=(LocationInfo&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // id
        _size += name.packedSize(); // name
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocationInfo&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocationInfo& x);

struct LookupReq {
    InodeId dirId;
    BincodeBytes name;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeBytes::STATIC_SIZE; // dirId + name

    LookupReq() { clear(); }
    explicit LookupReq(InodeId dirId_, BincodeBytes&& name_) : dirId(dirId_), name(std::move(name_)) {}
    LookupReq(const LookupReq&) = default;
    LookupReq& operator=(const LookupReq&) = default;
    LookupReq(LookupReq&&) = default;
    LookupReq& operator=(LookupReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += name.packedSize(); // name
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LookupReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LookupReq& x);

struct LookupResp {
    InodeId targetId;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // targetId + creationTime

    LookupResp() { clear(); }
    explicit LookupResp(InodeId targetId_, TernTime creationTime_) : targetId(targetId_), creationTime(creationTime_) {}
    LookupResp(const LookupResp&) = default;
    LookupResp& operator=(const LookupResp&) = default;
    LookupResp(LookupResp&&) = default;
    LookupResp& operator=(LookupResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // targetId
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LookupResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LookupResp& x);

struct StatFileReq {
    InodeId id;

    static constexpr uint16_t STATIC_SIZE = 8; // id

    StatFileReq() { clear(); }
    explicit StatFileReq(InodeId id_) : id(id_) {}
    StatFileReq(const StatFileReq&) = default;
    StatFileReq& operator=(const StatFileReq&) = default;
    StatFileReq(StatFileReq&&) = default;
    StatFileReq& operator=(StatFileReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const StatFileReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const StatFileReq& x);

struct StatFileResp {
    TernTime mtime;
    TernTime atime;
    uint64_t size;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 8; // mtime + atime + size

    StatFileResp() { clear(); }
    explicit StatFileResp(TernTime mtime_, TernTime atime_, uint64_t size_) : mtime(mtime_), atime(atime_), size(size_) {}
    StatFileResp(const StatFileResp&) = default;
    StatFileResp& operator=(const StatFileResp&) = default;
    StatFileResp(StatFileResp&&) = default;
    StatFileResp& operator=(StatFileResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // mtime
        _size += 8; // atime
        _size += 8; // size
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const StatFileResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const StatFileResp& x);

struct StatDirectoryReq {
    InodeId id;

    static constexpr uint16_t STATIC_SIZE = 8; // id

    StatDirectoryReq() { clear(); }
    explicit StatDirectoryReq(InodeId id_) : id(id_) {}
    StatDirectoryReq(const StatDirectoryReq&) = default;
    StatDirectoryReq& operator=(const StatDirectoryReq&) = default;
    StatDirectoryReq(StatDirectoryReq&&) = default;
    StatDirectoryReq& operator=(StatDirectoryReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const StatDirectoryReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const StatDirectoryReq& x);

struct StatDirectoryResp {
    TernTime mtime;
    InodeId owner;
    DirectoryInfo info;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + DirectoryInfo::STATIC_SIZE; // mtime + owner + info

    StatDirectoryResp() { clear(); }
    explicit StatDirectoryResp(TernTime mtime_, InodeId owner_, DirectoryInfo&& info_) : mtime(mtime_), owner(owner_), info(std::move(info_)) {}
    StatDirectoryResp(const StatDirectoryResp&) = default;
    StatDirectoryResp& operator=(const StatDirectoryResp&) = default;
    StatDirectoryResp(StatDirectoryResp&&) = default;
    StatDirectoryResp& operator=(StatDirectoryResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // mtime
        _size += 8; // owner
        _size += info.packedSize(); // info
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const StatDirectoryResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const StatDirectoryResp& x);

struct ReadDirReq {
    InodeId dirId;
    uint64_t startHash;
    uint16_t mtu;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 2; // dirId + startHash + mtu

    ReadDirReq() { clear(); }
    explicit ReadDirReq(InodeId dirId_, uint64_t startHash_, uint16_t mtu_) : dirId(dirId_), startHash(startHash_), mtu(mtu_) {}
    ReadDirReq(const ReadDirReq&) = default;
    ReadDirReq& operator=(const ReadDirReq&) = default;
    ReadDirReq(ReadDirReq&&) = default;
    ReadDirReq& operator=(ReadDirReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += 8; // startHash
        _size += 2; // mtu
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ReadDirReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ReadDirReq& x);

struct ReadDirResp {
    uint64_t nextHash;
    BincodeList<CurrentEdge> results;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeList<CurrentEdge>::STATIC_SIZE; // nextHash + results

    ReadDirResp() { clear(); }
    explicit ReadDirResp(uint64_t nextHash_, BincodeList<CurrentEdge>&& results_) : nextHash(nextHash_), results(std::move(results_)) {}
    ReadDirResp(const ReadDirResp&) = default;
    ReadDirResp& operator=(const ReadDirResp&) = default;
    ReadDirResp(ReadDirResp&&) = default;
    ReadDirResp& operator=(ReadDirResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // nextHash
        _size += results.packedSize(); // results
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ReadDirResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ReadDirResp& x);

struct ConstructFileReq {
    uint8_t type;
    BincodeBytes note;

    static constexpr uint16_t STATIC_SIZE = 1 + BincodeBytes::STATIC_SIZE; // type + note

    ConstructFileReq() { clear(); }
    explicit ConstructFileReq(uint8_t type_, BincodeBytes&& note_) : type(type_), note(std::move(note_)) {}
    ConstructFileReq(const ConstructFileReq&) = default;
    ConstructFileReq& operator=(const ConstructFileReq&) = default;
    ConstructFileReq(ConstructFileReq&&) = default;
    ConstructFileReq& operator=(ConstructFileReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // type
        _size += note.packedSize(); // note
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ConstructFileReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ConstructFileReq& x);

struct ConstructFileResp {
    InodeId id;
    BincodeFixedBytes<8> cookie;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE; // id + cookie

    ConstructFileResp() { clear(); }
    explicit ConstructFileResp(InodeId id_, BincodeFixedBytes<8>&& cookie_) : id(id_), cookie(std::move(cookie_)) {}
    ConstructFileResp(const ConstructFileResp&) = default;
    ConstructFileResp& operator=(const ConstructFileResp&) = default;
    ConstructFileResp(ConstructFileResp&&) = default;
    ConstructFileResp& operator=(ConstructFileResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ConstructFileResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ConstructFileResp& x);

struct AddSpanInitiateReq {
    InodeId fileId;
    BincodeFixedBytes<8> cookie;
    uint64_t byteOffset;
    uint32_t size;
    Crc crc;
    uint8_t storageClass;
    BincodeList<BlacklistEntry> blacklist;
    Parity parity;
    uint8_t stripes;
    uint32_t cellSize;
    BincodeList<Crc> crcs;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE + 8 + 4 + 4 + 1 + BincodeList<BlacklistEntry>::STATIC_SIZE + 1 + 1 + 4 + BincodeList<Crc>::STATIC_SIZE; // fileId + cookie + byteOffset + size + crc + storageClass + blacklist + parity + stripes + cellSize + crcs

    AddSpanInitiateReq() { clear(); }
    explicit AddSpanInitiateReq(InodeId fileId_, BincodeFixedBytes<8>&& cookie_, uint64_t byteOffset_, uint32_t size_, Crc crc_, uint8_t storageClass_, BincodeList<BlacklistEntry>&& blacklist_, Parity parity_, uint8_t stripes_, uint32_t cellSize_, BincodeList<Crc>&& crcs_) : fileId(fileId_), cookie(std::move(cookie_)), byteOffset(byteOffset_), size(size_), crc(crc_), storageClass(storageClass_), blacklist(std::move(blacklist_)), parity(parity_), stripes(stripes_), cellSize(cellSize_), crcs(std::move(crcs_)) {}
    AddSpanInitiateReq(const AddSpanInitiateReq&) = default;
    AddSpanInitiateReq& operator=(const AddSpanInitiateReq&) = default;
    AddSpanInitiateReq(AddSpanInitiateReq&&) = default;
    AddSpanInitiateReq& operator=(AddSpanInitiateReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie
        _size += 8; // byteOffset
        _size += 4; // size
        _size += 4; // crc
        _size += 1; // storageClass
        _size += blacklist.packedSize(); // blacklist
        _size += 1; // parity
        _size += 1; // stripes
        _size += 4; // cellSize
        _size += crcs.packedSize(); // crcs
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanInitiateReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanInitiateReq& x);

struct AddSpanInitiateResp {
    BincodeList<AddSpanInitiateBlockInfo> blocks;

    static constexpr uint16_t STATIC_SIZE = BincodeList<AddSpanInitiateBlockInfo>::STATIC_SIZE; // blocks

    AddSpanInitiateResp() { clear(); }
    explicit AddSpanInitiateResp(BincodeList<AddSpanInitiateBlockInfo>&& blocks_) : blocks(std::move(blocks_)) {}
    AddSpanInitiateResp(const AddSpanInitiateResp&) = default;
    AddSpanInitiateResp& operator=(const AddSpanInitiateResp&) = default;
    AddSpanInitiateResp(AddSpanInitiateResp&&) = default;
    AddSpanInitiateResp& operator=(AddSpanInitiateResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += blocks.packedSize(); // blocks
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanInitiateResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanInitiateResp& x);

struct AddSpanCertifyReq {
    InodeId fileId;
    BincodeFixedBytes<8> cookie;
    uint64_t byteOffset;
    BincodeList<BlockProof> proofs;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE + 8 + BincodeList<BlockProof>::STATIC_SIZE; // fileId + cookie + byteOffset + proofs

    AddSpanCertifyReq() { clear(); }
    explicit AddSpanCertifyReq(InodeId fileId_, BincodeFixedBytes<8>&& cookie_, uint64_t byteOffset_, BincodeList<BlockProof>&& proofs_) : fileId(fileId_), cookie(std::move(cookie_)), byteOffset(byteOffset_), proofs(std::move(proofs_)) {}
    AddSpanCertifyReq(const AddSpanCertifyReq&) = default;
    AddSpanCertifyReq& operator=(const AddSpanCertifyReq&) = default;
    AddSpanCertifyReq(AddSpanCertifyReq&&) = default;
    AddSpanCertifyReq& operator=(AddSpanCertifyReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie
        _size += 8; // byteOffset
        _size += proofs.packedSize(); // proofs
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanCertifyReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanCertifyReq& x);

struct AddSpanCertifyResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    AddSpanCertifyResp() { clear(); }
    AddSpanCertifyResp(const AddSpanCertifyResp&) = default;
    AddSpanCertifyResp& operator=(const AddSpanCertifyResp&) = default;
    AddSpanCertifyResp(AddSpanCertifyResp&&) = default;
    AddSpanCertifyResp& operator=(AddSpanCertifyResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanCertifyResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanCertifyResp& x);

struct LinkFileReq {
    InodeId fileId;
    BincodeFixedBytes<8> cookie;
    InodeId ownerId;
    BincodeBytes name;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE + 8 + BincodeBytes::STATIC_SIZE; // fileId + cookie + ownerId + name

    LinkFileReq() { clear(); }
    explicit LinkFileReq(InodeId fileId_, BincodeFixedBytes<8>&& cookie_, InodeId ownerId_, BincodeBytes&& name_) : fileId(fileId_), cookie(std::move(cookie_)), ownerId(ownerId_), name(std::move(name_)) {}
    LinkFileReq(const LinkFileReq&) = default;
    LinkFileReq& operator=(const LinkFileReq&) = default;
    LinkFileReq(LinkFileReq&&) = default;
    LinkFileReq& operator=(LinkFileReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie
        _size += 8; // ownerId
        _size += name.packedSize(); // name
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LinkFileReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LinkFileReq& x);

struct LinkFileResp {
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8; // creationTime

    LinkFileResp() { clear(); }
    explicit LinkFileResp(TernTime creationTime_) : creationTime(creationTime_) {}
    LinkFileResp(const LinkFileResp&) = default;
    LinkFileResp& operator=(const LinkFileResp&) = default;
    LinkFileResp(LinkFileResp&&) = default;
    LinkFileResp& operator=(LinkFileResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LinkFileResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LinkFileResp& x);

struct SoftUnlinkFileReq {
    InodeId ownerId;
    InodeId fileId;
    BincodeBytes name;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8; // ownerId + fileId + name + creationTime

    SoftUnlinkFileReq() { clear(); }
    explicit SoftUnlinkFileReq(InodeId ownerId_, InodeId fileId_, BincodeBytes&& name_, TernTime creationTime_) : ownerId(ownerId_), fileId(fileId_), name(std::move(name_)), creationTime(creationTime_) {}
    SoftUnlinkFileReq(const SoftUnlinkFileReq&) = default;
    SoftUnlinkFileReq& operator=(const SoftUnlinkFileReq&) = default;
    SoftUnlinkFileReq(SoftUnlinkFileReq&&) = default;
    SoftUnlinkFileReq& operator=(SoftUnlinkFileReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // ownerId
        _size += 8; // fileId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SoftUnlinkFileReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SoftUnlinkFileReq& x);

struct SoftUnlinkFileResp {
    TernTime deleteCreationTime;

    static constexpr uint16_t STATIC_SIZE = 8; // deleteCreationTime

    SoftUnlinkFileResp() { clear(); }
    explicit SoftUnlinkFileResp(TernTime deleteCreationTime_) : deleteCreationTime(deleteCreationTime_) {}
    SoftUnlinkFileResp(const SoftUnlinkFileResp&) = default;
    SoftUnlinkFileResp& operator=(const SoftUnlinkFileResp&) = default;
    SoftUnlinkFileResp(SoftUnlinkFileResp&&) = default;
    SoftUnlinkFileResp& operator=(SoftUnlinkFileResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // deleteCreationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SoftUnlinkFileResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SoftUnlinkFileResp& x);

struct LocalFileSpansReq {
    InodeId fileId;
    uint64_t byteOffset;
    uint32_t limit;
    uint16_t mtu;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 4 + 2; // fileId + byteOffset + limit + mtu

    LocalFileSpansReq() { clear(); }
    explicit LocalFileSpansReq(InodeId fileId_, uint64_t byteOffset_, uint32_t limit_, uint16_t mtu_) : fileId(fileId_), byteOffset(byteOffset_), limit(limit_), mtu(mtu_) {}
    LocalFileSpansReq(const LocalFileSpansReq&) = default;
    LocalFileSpansReq& operator=(const LocalFileSpansReq&) = default;
    LocalFileSpansReq(LocalFileSpansReq&&) = default;
    LocalFileSpansReq& operator=(LocalFileSpansReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += 8; // byteOffset
        _size += 4; // limit
        _size += 2; // mtu
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocalFileSpansReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocalFileSpansReq& x);

struct LocalFileSpansResp {
    uint64_t nextOffset;
    BincodeList<BlockService> blockServices;
    BincodeList<FetchedSpan> spans;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeList<BlockService>::STATIC_SIZE + BincodeList<FetchedSpan>::STATIC_SIZE; // nextOffset + blockServices + spans

    LocalFileSpansResp() { clear(); }
    explicit LocalFileSpansResp(uint64_t nextOffset_, BincodeList<BlockService>&& blockServices_, BincodeList<FetchedSpan>&& spans_) : nextOffset(nextOffset_), blockServices(std::move(blockServices_)), spans(std::move(spans_)) {}
    LocalFileSpansResp(const LocalFileSpansResp&) = default;
    LocalFileSpansResp& operator=(const LocalFileSpansResp&) = default;
    LocalFileSpansResp(LocalFileSpansResp&&) = default;
    LocalFileSpansResp& operator=(LocalFileSpansResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // nextOffset
        _size += blockServices.packedSize(); // blockServices
        _size += spans.packedSize(); // spans
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocalFileSpansResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocalFileSpansResp& x);

struct SameDirectoryRenameReq {
    InodeId targetId;
    InodeId dirId;
    BincodeBytes oldName;
    TernTime oldCreationTime;
    BincodeBytes newName;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8 + BincodeBytes::STATIC_SIZE; // targetId + dirId + oldName + oldCreationTime + newName

    SameDirectoryRenameReq() { clear(); }
    explicit SameDirectoryRenameReq(InodeId targetId_, InodeId dirId_, BincodeBytes&& oldName_, TernTime oldCreationTime_, BincodeBytes&& newName_) : targetId(targetId_), dirId(dirId_), oldName(std::move(oldName_)), oldCreationTime(oldCreationTime_), newName(std::move(newName_)) {}
    SameDirectoryRenameReq(const SameDirectoryRenameReq&) = default;
    SameDirectoryRenameReq& operator=(const SameDirectoryRenameReq&) = default;
    SameDirectoryRenameReq(SameDirectoryRenameReq&&) = default;
    SameDirectoryRenameReq& operator=(SameDirectoryRenameReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // targetId
        _size += 8; // dirId
        _size += oldName.packedSize(); // oldName
        _size += 8; // oldCreationTime
        _size += newName.packedSize(); // newName
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SameDirectoryRenameReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameReq& x);

struct SameDirectoryRenameResp {
    TernTime newCreationTime;

    static constexpr uint16_t STATIC_SIZE = 8; // newCreationTime

    SameDirectoryRenameResp() { clear(); }
    explicit SameDirectoryRenameResp(TernTime newCreationTime_) : newCreationTime(newCreationTime_) {}
    SameDirectoryRenameResp(const SameDirectoryRenameResp&) = default;
    SameDirectoryRenameResp& operator=(const SameDirectoryRenameResp&) = default;
    SameDirectoryRenameResp(SameDirectoryRenameResp&&) = default;
    SameDirectoryRenameResp& operator=(SameDirectoryRenameResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // newCreationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SameDirectoryRenameResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameResp& x);

struct AddInlineSpanReq {
    InodeId fileId;
    BincodeFixedBytes<8> cookie;
    uint8_t storageClass;
    uint64_t byteOffset;
    uint32_t size;
    Crc crc;
    BincodeBytes body;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE + 1 + 8 + 4 + 4 + BincodeBytes::STATIC_SIZE; // fileId + cookie + storageClass + byteOffset + size + crc + body

    AddInlineSpanReq() { clear(); }
    explicit AddInlineSpanReq(InodeId fileId_, BincodeFixedBytes<8>&& cookie_, uint8_t storageClass_, uint64_t byteOffset_, uint32_t size_, Crc crc_, BincodeBytes&& body_) : fileId(fileId_), cookie(std::move(cookie_)), storageClass(storageClass_), byteOffset(byteOffset_), size(size_), crc(crc_), body(std::move(body_)) {}
    AddInlineSpanReq(const AddInlineSpanReq&) = default;
    AddInlineSpanReq& operator=(const AddInlineSpanReq&) = default;
    AddInlineSpanReq(AddInlineSpanReq&&) = default;
    AddInlineSpanReq& operator=(AddInlineSpanReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie
        _size += 1; // storageClass
        _size += 8; // byteOffset
        _size += 4; // size
        _size += 4; // crc
        _size += body.packedSize(); // body
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddInlineSpanReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddInlineSpanReq& x);

struct AddInlineSpanResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    AddInlineSpanResp() { clear(); }
    AddInlineSpanResp(const AddInlineSpanResp&) = default;
    AddInlineSpanResp& operator=(const AddInlineSpanResp&) = default;
    AddInlineSpanResp(AddInlineSpanResp&&) = default;
    AddInlineSpanResp& operator=(AddInlineSpanResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddInlineSpanResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddInlineSpanResp& x);

struct SetTimeReq {
    InodeId id;
    uint64_t mtime;
    uint64_t atime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 8; // id + mtime + atime

    SetTimeReq() { clear(); }
    explicit SetTimeReq(InodeId id_, uint64_t mtime_, uint64_t atime_) : id(id_), mtime(mtime_), atime(atime_) {}
    SetTimeReq(const SetTimeReq&) = default;
    SetTimeReq& operator=(const SetTimeReq&) = default;
    SetTimeReq(SetTimeReq&&) = default;
    SetTimeReq& operator=(SetTimeReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 8; // mtime
        _size += 8; // atime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetTimeReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetTimeReq& x);

struct SetTimeResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    SetTimeResp() { clear(); }
    SetTimeResp(const SetTimeResp&) = default;
    SetTimeResp& operator=(const SetTimeResp&) = default;
    SetTimeResp(SetTimeResp&&) = default;
    SetTimeResp& operator=(SetTimeResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetTimeResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetTimeResp& x);

struct FullReadDirReq {
    InodeId dirId;
    uint8_t flags;
    BincodeBytes startName;
    TernTime startTime;
    uint16_t limit;
    uint16_t mtu;

    static constexpr uint16_t STATIC_SIZE = 8 + 1 + BincodeBytes::STATIC_SIZE + 8 + 2 + 2; // dirId + flags + startName + startTime + limit + mtu

    FullReadDirReq() { clear(); }
    explicit FullReadDirReq(InodeId dirId_, uint8_t flags_, BincodeBytes&& startName_, TernTime startTime_, uint16_t limit_, uint16_t mtu_) : dirId(dirId_), flags(flags_), startName(std::move(startName_)), startTime(startTime_), limit(limit_), mtu(mtu_) {}
    FullReadDirReq(const FullReadDirReq&) = default;
    FullReadDirReq& operator=(const FullReadDirReq&) = default;
    FullReadDirReq(FullReadDirReq&&) = default;
    FullReadDirReq& operator=(FullReadDirReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += 1; // flags
        _size += startName.packedSize(); // startName
        _size += 8; // startTime
        _size += 2; // limit
        _size += 2; // mtu
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FullReadDirReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FullReadDirReq& x);

struct FullReadDirResp {
    FullReadDirCursor next;
    BincodeList<Edge> results;

    static constexpr uint16_t STATIC_SIZE = FullReadDirCursor::STATIC_SIZE + BincodeList<Edge>::STATIC_SIZE; // next + results

    FullReadDirResp() { clear(); }
    explicit FullReadDirResp(FullReadDirCursor&& next_, BincodeList<Edge>&& results_) : next(std::move(next_)), results(std::move(results_)) {}
    FullReadDirResp(const FullReadDirResp&) = default;
    FullReadDirResp& operator=(const FullReadDirResp&) = default;
    FullReadDirResp(FullReadDirResp&&) = default;
    FullReadDirResp& operator=(FullReadDirResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += next.packedSize(); // next
        _size += results.packedSize(); // results
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FullReadDirResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FullReadDirResp& x);

struct MoveSpanReq {
    uint32_t spanSize;
    InodeId fileId1;
    uint64_t byteOffset1;
    BincodeFixedBytes<8> cookie1;
    InodeId fileId2;
    uint64_t byteOffset2;
    BincodeFixedBytes<8> cookie2;

    static constexpr uint16_t STATIC_SIZE = 4 + 8 + 8 + BincodeFixedBytes<8>::STATIC_SIZE + 8 + 8 + BincodeFixedBytes<8>::STATIC_SIZE; // spanSize + fileId1 + byteOffset1 + cookie1 + fileId2 + byteOffset2 + cookie2

    MoveSpanReq() { clear(); }
    explicit MoveSpanReq(uint32_t spanSize_, InodeId fileId1_, uint64_t byteOffset1_, BincodeFixedBytes<8>&& cookie1_, InodeId fileId2_, uint64_t byteOffset2_, BincodeFixedBytes<8>&& cookie2_) : spanSize(spanSize_), fileId1(fileId1_), byteOffset1(byteOffset1_), cookie1(std::move(cookie1_)), fileId2(fileId2_), byteOffset2(byteOffset2_), cookie2(std::move(cookie2_)) {}
    MoveSpanReq(const MoveSpanReq&) = default;
    MoveSpanReq& operator=(const MoveSpanReq&) = default;
    MoveSpanReq(MoveSpanReq&&) = default;
    MoveSpanReq& operator=(MoveSpanReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 4; // spanSize
        _size += 8; // fileId1
        _size += 8; // byteOffset1
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie1
        _size += 8; // fileId2
        _size += 8; // byteOffset2
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie2
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MoveSpanReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MoveSpanReq& x);

struct MoveSpanResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    MoveSpanResp() { clear(); }
    MoveSpanResp(const MoveSpanResp&) = default;
    MoveSpanResp& operator=(const MoveSpanResp&) = default;
    MoveSpanResp(MoveSpanResp&&) = default;
    MoveSpanResp& operator=(MoveSpanResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MoveSpanResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MoveSpanResp& x);

struct RemoveNonOwnedEdgeReq {
    InodeId dirId;
    InodeId targetId;
    BincodeBytes name;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8; // dirId + targetId + name + creationTime

    RemoveNonOwnedEdgeReq() { clear(); }
    explicit RemoveNonOwnedEdgeReq(InodeId dirId_, InodeId targetId_, BincodeBytes&& name_, TernTime creationTime_) : dirId(dirId_), targetId(targetId_), name(std::move(name_)), creationTime(creationTime_) {}
    RemoveNonOwnedEdgeReq(const RemoveNonOwnedEdgeReq&) = default;
    RemoveNonOwnedEdgeReq& operator=(const RemoveNonOwnedEdgeReq&) = default;
    RemoveNonOwnedEdgeReq(RemoveNonOwnedEdgeReq&&) = default;
    RemoveNonOwnedEdgeReq& operator=(RemoveNonOwnedEdgeReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += 8; // targetId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveNonOwnedEdgeReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveNonOwnedEdgeReq& x);

struct RemoveNonOwnedEdgeResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RemoveNonOwnedEdgeResp() { clear(); }
    RemoveNonOwnedEdgeResp(const RemoveNonOwnedEdgeResp&) = default;
    RemoveNonOwnedEdgeResp& operator=(const RemoveNonOwnedEdgeResp&) = default;
    RemoveNonOwnedEdgeResp(RemoveNonOwnedEdgeResp&&) = default;
    RemoveNonOwnedEdgeResp& operator=(RemoveNonOwnedEdgeResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveNonOwnedEdgeResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveNonOwnedEdgeResp& x);

struct SameShardHardFileUnlinkReq {
    InodeId ownerId;
    InodeId targetId;
    BincodeBytes name;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8; // ownerId + targetId + name + creationTime

    SameShardHardFileUnlinkReq() { clear(); }
    explicit SameShardHardFileUnlinkReq(InodeId ownerId_, InodeId targetId_, BincodeBytes&& name_, TernTime creationTime_) : ownerId(ownerId_), targetId(targetId_), name(std::move(name_)), creationTime(creationTime_) {}
    SameShardHardFileUnlinkReq(const SameShardHardFileUnlinkReq&) = default;
    SameShardHardFileUnlinkReq& operator=(const SameShardHardFileUnlinkReq&) = default;
    SameShardHardFileUnlinkReq(SameShardHardFileUnlinkReq&&) = default;
    SameShardHardFileUnlinkReq& operator=(SameShardHardFileUnlinkReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // ownerId
        _size += 8; // targetId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SameShardHardFileUnlinkReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SameShardHardFileUnlinkReq& x);

struct SameShardHardFileUnlinkResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    SameShardHardFileUnlinkResp() { clear(); }
    SameShardHardFileUnlinkResp(const SameShardHardFileUnlinkResp&) = default;
    SameShardHardFileUnlinkResp& operator=(const SameShardHardFileUnlinkResp&) = default;
    SameShardHardFileUnlinkResp(SameShardHardFileUnlinkResp&&) = default;
    SameShardHardFileUnlinkResp& operator=(SameShardHardFileUnlinkResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SameShardHardFileUnlinkResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SameShardHardFileUnlinkResp& x);

struct StatTransientFileReq {
    InodeId id;

    static constexpr uint16_t STATIC_SIZE = 8; // id

    StatTransientFileReq() { clear(); }
    explicit StatTransientFileReq(InodeId id_) : id(id_) {}
    StatTransientFileReq(const StatTransientFileReq&) = default;
    StatTransientFileReq& operator=(const StatTransientFileReq&) = default;
    StatTransientFileReq(StatTransientFileReq&&) = default;
    StatTransientFileReq& operator=(StatTransientFileReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const StatTransientFileReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const StatTransientFileReq& x);

struct StatTransientFileResp {
    TernTime mtime;
    uint64_t size;
    BincodeBytes note;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE; // mtime + size + note

    StatTransientFileResp() { clear(); }
    explicit StatTransientFileResp(TernTime mtime_, uint64_t size_, BincodeBytes&& note_) : mtime(mtime_), size(size_), note(std::move(note_)) {}
    StatTransientFileResp(const StatTransientFileResp&) = default;
    StatTransientFileResp& operator=(const StatTransientFileResp&) = default;
    StatTransientFileResp(StatTransientFileResp&&) = default;
    StatTransientFileResp& operator=(StatTransientFileResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // mtime
        _size += 8; // size
        _size += note.packedSize(); // note
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const StatTransientFileResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const StatTransientFileResp& x);

struct ShardSnapshotReq {
    uint64_t snapshotId;

    static constexpr uint16_t STATIC_SIZE = 8; // snapshotId

    ShardSnapshotReq() { clear(); }
    explicit ShardSnapshotReq(uint64_t snapshotId_) : snapshotId(snapshotId_) {}
    ShardSnapshotReq(const ShardSnapshotReq&) = default;
    ShardSnapshotReq& operator=(const ShardSnapshotReq&) = default;
    ShardSnapshotReq(ShardSnapshotReq&&) = default;
    ShardSnapshotReq& operator=(ShardSnapshotReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // snapshotId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ShardSnapshotReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ShardSnapshotReq& x);

struct ShardSnapshotResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    ShardSnapshotResp() { clear(); }
    ShardSnapshotResp(const ShardSnapshotResp&) = default;
    ShardSnapshotResp& operator=(const ShardSnapshotResp&) = default;
    ShardSnapshotResp(ShardSnapshotResp&&) = default;
    ShardSnapshotResp& operator=(ShardSnapshotResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ShardSnapshotResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ShardSnapshotResp& x);

struct FileSpansReq {
    InodeId fileId;
    uint64_t byteOffset;
    uint32_t limit;
    uint16_t mtu;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 4 + 2; // fileId + byteOffset + limit + mtu

    FileSpansReq() { clear(); }
    explicit FileSpansReq(InodeId fileId_, uint64_t byteOffset_, uint32_t limit_, uint16_t mtu_) : fileId(fileId_), byteOffset(byteOffset_), limit(limit_), mtu(mtu_) {}
    FileSpansReq(const FileSpansReq&) = default;
    FileSpansReq& operator=(const FileSpansReq&) = default;
    FileSpansReq(FileSpansReq&&) = default;
    FileSpansReq& operator=(FileSpansReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += 8; // byteOffset
        _size += 4; // limit
        _size += 2; // mtu
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FileSpansReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FileSpansReq& x);

struct FileSpansResp {
    uint64_t nextOffset;
    BincodeList<BlockService> blockServices;
    BincodeList<FetchedFullSpan> spans;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeList<BlockService>::STATIC_SIZE + BincodeList<FetchedFullSpan>::STATIC_SIZE; // nextOffset + blockServices + spans

    FileSpansResp() { clear(); }
    explicit FileSpansResp(uint64_t nextOffset_, BincodeList<BlockService>&& blockServices_, BincodeList<FetchedFullSpan>&& spans_) : nextOffset(nextOffset_), blockServices(std::move(blockServices_)), spans(std::move(spans_)) {}
    FileSpansResp(const FileSpansResp&) = default;
    FileSpansResp& operator=(const FileSpansResp&) = default;
    FileSpansResp(FileSpansResp&&) = default;
    FileSpansResp& operator=(FileSpansResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // nextOffset
        _size += blockServices.packedSize(); // blockServices
        _size += spans.packedSize(); // spans
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FileSpansResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FileSpansResp& x);

struct AddSpanLocationReq {
    InodeId fileId1;
    uint64_t byteOffset1;
    BincodeList<uint64_t> blocks1;
    InodeId fileId2;
    uint64_t byteOffset2;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeList<uint64_t>::STATIC_SIZE + 8 + 8; // fileId1 + byteOffset1 + blocks1 + fileId2 + byteOffset2

    AddSpanLocationReq() { clear(); }
    explicit AddSpanLocationReq(InodeId fileId1_, uint64_t byteOffset1_, BincodeList<uint64_t>&& blocks1_, InodeId fileId2_, uint64_t byteOffset2_) : fileId1(fileId1_), byteOffset1(byteOffset1_), blocks1(std::move(blocks1_)), fileId2(fileId2_), byteOffset2(byteOffset2_) {}
    AddSpanLocationReq(const AddSpanLocationReq&) = default;
    AddSpanLocationReq& operator=(const AddSpanLocationReq&) = default;
    AddSpanLocationReq(AddSpanLocationReq&&) = default;
    AddSpanLocationReq& operator=(AddSpanLocationReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId1
        _size += 8; // byteOffset1
        _size += blocks1.packedSize(); // blocks1
        _size += 8; // fileId2
        _size += 8; // byteOffset2
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanLocationReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanLocationReq& x);

struct AddSpanLocationResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    AddSpanLocationResp() { clear(); }
    AddSpanLocationResp(const AddSpanLocationResp&) = default;
    AddSpanLocationResp& operator=(const AddSpanLocationResp&) = default;
    AddSpanLocationResp(AddSpanLocationResp&&) = default;
    AddSpanLocationResp& operator=(AddSpanLocationResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanLocationResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanLocationResp& x);

struct ScrapTransientFileReq {
    InodeId id;
    BincodeFixedBytes<8> cookie;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE; // id + cookie

    ScrapTransientFileReq() { clear(); }
    explicit ScrapTransientFileReq(InodeId id_, BincodeFixedBytes<8>&& cookie_) : id(id_), cookie(std::move(cookie_)) {}
    ScrapTransientFileReq(const ScrapTransientFileReq&) = default;
    ScrapTransientFileReq& operator=(const ScrapTransientFileReq&) = default;
    ScrapTransientFileReq(ScrapTransientFileReq&&) = default;
    ScrapTransientFileReq& operator=(ScrapTransientFileReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ScrapTransientFileReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ScrapTransientFileReq& x);

struct ScrapTransientFileResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    ScrapTransientFileResp() { clear(); }
    ScrapTransientFileResp(const ScrapTransientFileResp&) = default;
    ScrapTransientFileResp& operator=(const ScrapTransientFileResp&) = default;
    ScrapTransientFileResp(ScrapTransientFileResp&&) = default;
    ScrapTransientFileResp& operator=(ScrapTransientFileResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ScrapTransientFileResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ScrapTransientFileResp& x);

struct SetDirectoryInfoReq {
    InodeId id;
    DirectoryInfo info;

    static constexpr uint16_t STATIC_SIZE = 8 + DirectoryInfo::STATIC_SIZE; // id + info

    SetDirectoryInfoReq() { clear(); }
    explicit SetDirectoryInfoReq(InodeId id_, DirectoryInfo&& info_) : id(id_), info(std::move(info_)) {}
    SetDirectoryInfoReq(const SetDirectoryInfoReq&) = default;
    SetDirectoryInfoReq& operator=(const SetDirectoryInfoReq&) = default;
    SetDirectoryInfoReq(SetDirectoryInfoReq&&) = default;
    SetDirectoryInfoReq& operator=(SetDirectoryInfoReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += info.packedSize(); // info
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetDirectoryInfoReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetDirectoryInfoReq& x);

struct SetDirectoryInfoResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    SetDirectoryInfoResp() { clear(); }
    SetDirectoryInfoResp(const SetDirectoryInfoResp&) = default;
    SetDirectoryInfoResp& operator=(const SetDirectoryInfoResp&) = default;
    SetDirectoryInfoResp(SetDirectoryInfoResp&&) = default;
    SetDirectoryInfoResp& operator=(SetDirectoryInfoResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetDirectoryInfoResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetDirectoryInfoResp& x);

struct GetLinkEntriesReq {
    LogIdx fromIdx;
    uint16_t mtu;

    static constexpr uint16_t STATIC_SIZE = 8 + 2; // fromIdx + mtu

    GetLinkEntriesReq() { clear(); }
    explicit GetLinkEntriesReq(LogIdx fromIdx_, uint16_t mtu_) : fromIdx(fromIdx_), mtu(mtu_) {}
    GetLinkEntriesReq(const GetLinkEntriesReq&) = default;
    GetLinkEntriesReq& operator=(const GetLinkEntriesReq&) = default;
    GetLinkEntriesReq(GetLinkEntriesReq&&) = default;
    GetLinkEntriesReq& operator=(GetLinkEntriesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fromIdx
        _size += 2; // mtu
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const GetLinkEntriesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const GetLinkEntriesReq& x);

struct GetLinkEntriesResp {
    BincodeList<LinkEntry> entries;
    LogIdx nextIdx;

    static constexpr uint16_t STATIC_SIZE = BincodeList<LinkEntry>::STATIC_SIZE + 8; // entries + nextIdx

    GetLinkEntriesResp() { clear(); }
    explicit GetLinkEntriesResp(BincodeList<LinkEntry>&& entries_, LogIdx nextIdx_) : entries(std::move(entries_)), nextIdx(nextIdx_) {}
    GetLinkEntriesResp(const GetLinkEntriesResp&) = default;
    GetLinkEntriesResp& operator=(const GetLinkEntriesResp&) = default;
    GetLinkEntriesResp(GetLinkEntriesResp&&) = default;
    GetLinkEntriesResp& operator=(GetLinkEntriesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += entries.packedSize(); // entries
        _size += 8; // nextIdx
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const GetLinkEntriesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const GetLinkEntriesResp& x);

struct WaitStateAppliedReq {
    LogIdx idx;

    static constexpr uint16_t STATIC_SIZE = 8; // idx

    WaitStateAppliedReq() { clear(); }
    explicit WaitStateAppliedReq(LogIdx idx_) : idx(idx_) {}
    WaitStateAppliedReq(const WaitStateAppliedReq&) = default;
    WaitStateAppliedReq& operator=(const WaitStateAppliedReq&) = default;
    WaitStateAppliedReq(WaitStateAppliedReq&&) = default;
    WaitStateAppliedReq& operator=(WaitStateAppliedReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // idx
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const WaitStateAppliedReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const WaitStateAppliedReq& x);

struct WaitStateAppliedResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    WaitStateAppliedResp() { clear(); }
    WaitStateAppliedResp(const WaitStateAppliedResp&) = default;
    WaitStateAppliedResp& operator=(const WaitStateAppliedResp&) = default;
    WaitStateAppliedResp(WaitStateAppliedResp&&) = default;
    WaitStateAppliedResp& operator=(WaitStateAppliedResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const WaitStateAppliedResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const WaitStateAppliedResp& x);

struct VisitDirectoriesReq {
    InodeId beginId;
    uint16_t mtu;

    static constexpr uint16_t STATIC_SIZE = 8 + 2; // beginId + mtu

    VisitDirectoriesReq() { clear(); }
    explicit VisitDirectoriesReq(InodeId beginId_, uint16_t mtu_) : beginId(beginId_), mtu(mtu_) {}
    VisitDirectoriesReq(const VisitDirectoriesReq&) = default;
    VisitDirectoriesReq& operator=(const VisitDirectoriesReq&) = default;
    VisitDirectoriesReq(VisitDirectoriesReq&&) = default;
    VisitDirectoriesReq& operator=(VisitDirectoriesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // beginId
        _size += 2; // mtu
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const VisitDirectoriesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const VisitDirectoriesReq& x);

struct VisitDirectoriesResp {
    InodeId nextId;
    BincodeList<InodeId> ids;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeList<InodeId>::STATIC_SIZE; // nextId + ids

    VisitDirectoriesResp() { clear(); }
    explicit VisitDirectoriesResp(InodeId nextId_, BincodeList<InodeId>&& ids_) : nextId(nextId_), ids(std::move(ids_)) {}
    VisitDirectoriesResp(const VisitDirectoriesResp&) = default;
    VisitDirectoriesResp& operator=(const VisitDirectoriesResp&) = default;
    VisitDirectoriesResp(VisitDirectoriesResp&&) = default;
    VisitDirectoriesResp& operator=(VisitDirectoriesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // nextId
        _size += ids.packedSize(); // ids
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const VisitDirectoriesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const VisitDirectoriesResp& x);

struct VisitFilesReq {
    InodeId beginId;
    uint16_t mtu;

    static constexpr uint16_t STATIC_SIZE = 8 + 2; // beginId + mtu

    VisitFilesReq() { clear(); }
    explicit VisitFilesReq(InodeId beginId_, uint16_t mtu_) : beginId(beginId_), mtu(mtu_) {}
    VisitFilesReq(const VisitFilesReq&) = default;
    VisitFilesReq& operator=(const VisitFilesReq&) = default;
    VisitFilesReq(VisitFilesReq&&) = default;
    VisitFilesReq& operator=(VisitFilesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // beginId
        _size += 2; // mtu
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const VisitFilesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const VisitFilesReq& x);

struct VisitFilesResp {
    InodeId nextId;
    BincodeList<InodeId> ids;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeList<InodeId>::STATIC_SIZE; // nextId + ids

    VisitFilesResp() { clear(); }
    explicit VisitFilesResp(InodeId nextId_, BincodeList<InodeId>&& ids_) : nextId(nextId_), ids(std::move(ids_)) {}
    VisitFilesResp(const VisitFilesResp&) = default;
    VisitFilesResp& operator=(const VisitFilesResp&) = default;
    VisitFilesResp(VisitFilesResp&&) = default;
    VisitFilesResp& operator=(VisitFilesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // nextId
        _size += ids.packedSize(); // ids
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const VisitFilesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const VisitFilesResp& x);

struct VisitTransientFilesReq {
    InodeId beginId;
    uint16_t mtu;

    static constexpr uint16_t STATIC_SIZE = 8 + 2; // beginId + mtu

    VisitTransientFilesReq() { clear(); }
    explicit VisitTransientFilesReq(InodeId beginId_, uint16_t mtu_) : beginId(beginId_), mtu(mtu_) {}
    VisitTransientFilesReq(const VisitTransientFilesReq&) = default;
    VisitTransientFilesReq& operator=(const VisitTransientFilesReq&) = default;
    VisitTransientFilesReq(VisitTransientFilesReq&&) = default;
    VisitTransientFilesReq& operator=(VisitTransientFilesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // beginId
        _size += 2; // mtu
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const VisitTransientFilesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const VisitTransientFilesReq& x);

struct VisitTransientFilesResp {
    InodeId nextId;
    BincodeList<TransientFile> files;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeList<TransientFile>::STATIC_SIZE; // nextId + files

    VisitTransientFilesResp() { clear(); }
    explicit VisitTransientFilesResp(InodeId nextId_, BincodeList<TransientFile>&& files_) : nextId(nextId_), files(std::move(files_)) {}
    VisitTransientFilesResp(const VisitTransientFilesResp&) = default;
    VisitTransientFilesResp& operator=(const VisitTransientFilesResp&) = default;
    VisitTransientFilesResp(VisitTransientFilesResp&&) = default;
    VisitTransientFilesResp& operator=(VisitTransientFilesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // nextId
        _size += files.packedSize(); // files
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const VisitTransientFilesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const VisitTransientFilesResp& x);

struct RemoveSpanInitiateReq {
    InodeId fileId;
    BincodeFixedBytes<8> cookie;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE; // fileId + cookie

    RemoveSpanInitiateReq() { clear(); }
    explicit RemoveSpanInitiateReq(InodeId fileId_, BincodeFixedBytes<8>&& cookie_) : fileId(fileId_), cookie(std::move(cookie_)) {}
    RemoveSpanInitiateReq(const RemoveSpanInitiateReq&) = default;
    RemoveSpanInitiateReq& operator=(const RemoveSpanInitiateReq&) = default;
    RemoveSpanInitiateReq(RemoveSpanInitiateReq&&) = default;
    RemoveSpanInitiateReq& operator=(RemoveSpanInitiateReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveSpanInitiateReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveSpanInitiateReq& x);

struct RemoveSpanInitiateResp {
    uint64_t byteOffset;
    BincodeList<RemoveSpanInitiateBlockInfo> blocks;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeList<RemoveSpanInitiateBlockInfo>::STATIC_SIZE; // byteOffset + blocks

    RemoveSpanInitiateResp() { clear(); }
    explicit RemoveSpanInitiateResp(uint64_t byteOffset_, BincodeList<RemoveSpanInitiateBlockInfo>&& blocks_) : byteOffset(byteOffset_), blocks(std::move(blocks_)) {}
    RemoveSpanInitiateResp(const RemoveSpanInitiateResp&) = default;
    RemoveSpanInitiateResp& operator=(const RemoveSpanInitiateResp&) = default;
    RemoveSpanInitiateResp(RemoveSpanInitiateResp&&) = default;
    RemoveSpanInitiateResp& operator=(RemoveSpanInitiateResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // byteOffset
        _size += blocks.packedSize(); // blocks
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveSpanInitiateResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveSpanInitiateResp& x);

struct RemoveSpanCertifyReq {
    InodeId fileId;
    BincodeFixedBytes<8> cookie;
    uint64_t byteOffset;
    BincodeList<BlockProof> proofs;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE + 8 + BincodeList<BlockProof>::STATIC_SIZE; // fileId + cookie + byteOffset + proofs

    RemoveSpanCertifyReq() { clear(); }
    explicit RemoveSpanCertifyReq(InodeId fileId_, BincodeFixedBytes<8>&& cookie_, uint64_t byteOffset_, BincodeList<BlockProof>&& proofs_) : fileId(fileId_), cookie(std::move(cookie_)), byteOffset(byteOffset_), proofs(std::move(proofs_)) {}
    RemoveSpanCertifyReq(const RemoveSpanCertifyReq&) = default;
    RemoveSpanCertifyReq& operator=(const RemoveSpanCertifyReq&) = default;
    RemoveSpanCertifyReq(RemoveSpanCertifyReq&&) = default;
    RemoveSpanCertifyReq& operator=(RemoveSpanCertifyReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie
        _size += 8; // byteOffset
        _size += proofs.packedSize(); // proofs
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveSpanCertifyReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveSpanCertifyReq& x);

struct RemoveSpanCertifyResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RemoveSpanCertifyResp() { clear(); }
    RemoveSpanCertifyResp(const RemoveSpanCertifyResp&) = default;
    RemoveSpanCertifyResp& operator=(const RemoveSpanCertifyResp&) = default;
    RemoveSpanCertifyResp(RemoveSpanCertifyResp&&) = default;
    RemoveSpanCertifyResp& operator=(RemoveSpanCertifyResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveSpanCertifyResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveSpanCertifyResp& x);

struct SwapBlocksReq {
    InodeId fileId1;
    uint64_t byteOffset1;
    uint64_t blockId1;
    InodeId fileId2;
    uint64_t byteOffset2;
    uint64_t blockId2;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 8 + 8 + 8 + 8; // fileId1 + byteOffset1 + blockId1 + fileId2 + byteOffset2 + blockId2

    SwapBlocksReq() { clear(); }
    explicit SwapBlocksReq(InodeId fileId1_, uint64_t byteOffset1_, uint64_t blockId1_, InodeId fileId2_, uint64_t byteOffset2_, uint64_t blockId2_) : fileId1(fileId1_), byteOffset1(byteOffset1_), blockId1(blockId1_), fileId2(fileId2_), byteOffset2(byteOffset2_), blockId2(blockId2_) {}
    SwapBlocksReq(const SwapBlocksReq&) = default;
    SwapBlocksReq& operator=(const SwapBlocksReq&) = default;
    SwapBlocksReq(SwapBlocksReq&&) = default;
    SwapBlocksReq& operator=(SwapBlocksReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId1
        _size += 8; // byteOffset1
        _size += 8; // blockId1
        _size += 8; // fileId2
        _size += 8; // byteOffset2
        _size += 8; // blockId2
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SwapBlocksReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SwapBlocksReq& x);

struct SwapBlocksResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    SwapBlocksResp() { clear(); }
    SwapBlocksResp(const SwapBlocksResp&) = default;
    SwapBlocksResp& operator=(const SwapBlocksResp&) = default;
    SwapBlocksResp(SwapBlocksResp&&) = default;
    SwapBlocksResp& operator=(SwapBlocksResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SwapBlocksResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SwapBlocksResp& x);

struct BlockServiceFilesReq {
    BlockServiceId blockServiceId;
    InodeId startFrom;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // blockServiceId + startFrom

    BlockServiceFilesReq() { clear(); }
    explicit BlockServiceFilesReq(BlockServiceId blockServiceId_, InodeId startFrom_) : blockServiceId(blockServiceId_), startFrom(startFrom_) {}
    BlockServiceFilesReq(const BlockServiceFilesReq&) = default;
    BlockServiceFilesReq& operator=(const BlockServiceFilesReq&) = default;
    BlockServiceFilesReq(BlockServiceFilesReq&&) = default;
    BlockServiceFilesReq& operator=(BlockServiceFilesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // blockServiceId
        _size += 8; // startFrom
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlockServiceFilesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlockServiceFilesReq& x);

struct BlockServiceFilesResp {
    BincodeList<InodeId> fileIds;

    static constexpr uint16_t STATIC_SIZE = BincodeList<InodeId>::STATIC_SIZE; // fileIds

    BlockServiceFilesResp() { clear(); }
    explicit BlockServiceFilesResp(BincodeList<InodeId>&& fileIds_) : fileIds(std::move(fileIds_)) {}
    BlockServiceFilesResp(const BlockServiceFilesResp&) = default;
    BlockServiceFilesResp& operator=(const BlockServiceFilesResp&) = default;
    BlockServiceFilesResp(BlockServiceFilesResp&&) = default;
    BlockServiceFilesResp& operator=(BlockServiceFilesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += fileIds.packedSize(); // fileIds
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlockServiceFilesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlockServiceFilesResp& x);

struct RemoveInodeReq {
    InodeId id;

    static constexpr uint16_t STATIC_SIZE = 8; // id

    RemoveInodeReq() { clear(); }
    explicit RemoveInodeReq(InodeId id_) : id(id_) {}
    RemoveInodeReq(const RemoveInodeReq&) = default;
    RemoveInodeReq& operator=(const RemoveInodeReq&) = default;
    RemoveInodeReq(RemoveInodeReq&&) = default;
    RemoveInodeReq& operator=(RemoveInodeReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveInodeReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveInodeReq& x);

struct RemoveInodeResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RemoveInodeResp() { clear(); }
    RemoveInodeResp(const RemoveInodeResp&) = default;
    RemoveInodeResp& operator=(const RemoveInodeResp&) = default;
    RemoveInodeResp(RemoveInodeResp&&) = default;
    RemoveInodeResp& operator=(RemoveInodeResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveInodeResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveInodeResp& x);

struct AddSpanInitiateWithReferenceReq {
    AddSpanInitiateReq req;
    InodeId reference;

    static constexpr uint16_t STATIC_SIZE = AddSpanInitiateReq::STATIC_SIZE + 8; // req + reference

    AddSpanInitiateWithReferenceReq() { clear(); }
    explicit AddSpanInitiateWithReferenceReq(AddSpanInitiateReq&& req_, InodeId reference_) : req(std::move(req_)), reference(reference_) {}
    AddSpanInitiateWithReferenceReq(const AddSpanInitiateWithReferenceReq&) = default;
    AddSpanInitiateWithReferenceReq& operator=(const AddSpanInitiateWithReferenceReq&) = default;
    AddSpanInitiateWithReferenceReq(AddSpanInitiateWithReferenceReq&&) = default;
    AddSpanInitiateWithReferenceReq& operator=(AddSpanInitiateWithReferenceReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += req.packedSize(); // req
        _size += 8; // reference
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanInitiateWithReferenceReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanInitiateWithReferenceReq& x);

struct AddSpanInitiateWithReferenceResp {
    AddSpanInitiateResp resp;

    static constexpr uint16_t STATIC_SIZE = AddSpanInitiateResp::STATIC_SIZE; // resp

    AddSpanInitiateWithReferenceResp() { clear(); }
    explicit AddSpanInitiateWithReferenceResp(AddSpanInitiateResp&& resp_) : resp(std::move(resp_)) {}
    AddSpanInitiateWithReferenceResp(const AddSpanInitiateWithReferenceResp&) = default;
    AddSpanInitiateWithReferenceResp& operator=(const AddSpanInitiateWithReferenceResp&) = default;
    AddSpanInitiateWithReferenceResp(AddSpanInitiateWithReferenceResp&&) = default;
    AddSpanInitiateWithReferenceResp& operator=(AddSpanInitiateWithReferenceResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += resp.packedSize(); // resp
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanInitiateWithReferenceResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanInitiateWithReferenceResp& x);

struct RemoveZeroBlockServiceFilesReq {
    BlockServiceId startBlockService;
    InodeId startFile;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // startBlockService + startFile

    RemoveZeroBlockServiceFilesReq() { clear(); }
    explicit RemoveZeroBlockServiceFilesReq(BlockServiceId startBlockService_, InodeId startFile_) : startBlockService(startBlockService_), startFile(startFile_) {}
    RemoveZeroBlockServiceFilesReq(const RemoveZeroBlockServiceFilesReq&) = default;
    RemoveZeroBlockServiceFilesReq& operator=(const RemoveZeroBlockServiceFilesReq&) = default;
    RemoveZeroBlockServiceFilesReq(RemoveZeroBlockServiceFilesReq&&) = default;
    RemoveZeroBlockServiceFilesReq& operator=(RemoveZeroBlockServiceFilesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // startBlockService
        _size += 8; // startFile
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveZeroBlockServiceFilesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveZeroBlockServiceFilesReq& x);

struct RemoveZeroBlockServiceFilesResp {
    uint64_t removed;
    BlockServiceId nextBlockService;
    InodeId nextFile;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 8; // removed + nextBlockService + nextFile

    RemoveZeroBlockServiceFilesResp() { clear(); }
    explicit RemoveZeroBlockServiceFilesResp(uint64_t removed_, BlockServiceId nextBlockService_, InodeId nextFile_) : removed(removed_), nextBlockService(nextBlockService_), nextFile(nextFile_) {}
    RemoveZeroBlockServiceFilesResp(const RemoveZeroBlockServiceFilesResp&) = default;
    RemoveZeroBlockServiceFilesResp& operator=(const RemoveZeroBlockServiceFilesResp&) = default;
    RemoveZeroBlockServiceFilesResp(RemoveZeroBlockServiceFilesResp&&) = default;
    RemoveZeroBlockServiceFilesResp& operator=(RemoveZeroBlockServiceFilesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // removed
        _size += 8; // nextBlockService
        _size += 8; // nextFile
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveZeroBlockServiceFilesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveZeroBlockServiceFilesResp& x);

struct SwapSpansReq {
    InodeId fileId1;
    uint64_t byteOffset1;
    BincodeList<uint64_t> blocks1;
    InodeId fileId2;
    uint64_t byteOffset2;
    BincodeList<uint64_t> blocks2;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeList<uint64_t>::STATIC_SIZE + 8 + 8 + BincodeList<uint64_t>::STATIC_SIZE; // fileId1 + byteOffset1 + blocks1 + fileId2 + byteOffset2 + blocks2

    SwapSpansReq() { clear(); }
    explicit SwapSpansReq(InodeId fileId1_, uint64_t byteOffset1_, BincodeList<uint64_t>&& blocks1_, InodeId fileId2_, uint64_t byteOffset2_, BincodeList<uint64_t>&& blocks2_) : fileId1(fileId1_), byteOffset1(byteOffset1_), blocks1(std::move(blocks1_)), fileId2(fileId2_), byteOffset2(byteOffset2_), blocks2(std::move(blocks2_)) {}
    SwapSpansReq(const SwapSpansReq&) = default;
    SwapSpansReq& operator=(const SwapSpansReq&) = default;
    SwapSpansReq(SwapSpansReq&&) = default;
    SwapSpansReq& operator=(SwapSpansReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId1
        _size += 8; // byteOffset1
        _size += blocks1.packedSize(); // blocks1
        _size += 8; // fileId2
        _size += 8; // byteOffset2
        _size += blocks2.packedSize(); // blocks2
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SwapSpansReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SwapSpansReq& x);

struct SwapSpansResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    SwapSpansResp() { clear(); }
    SwapSpansResp(const SwapSpansResp&) = default;
    SwapSpansResp& operator=(const SwapSpansResp&) = default;
    SwapSpansResp(SwapSpansResp&&) = default;
    SwapSpansResp& operator=(SwapSpansResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SwapSpansResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SwapSpansResp& x);

struct SameDirectoryRenameSnapshotReq {
    InodeId targetId;
    InodeId dirId;
    BincodeBytes oldName;
    TernTime oldCreationTime;
    BincodeBytes newName;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8 + BincodeBytes::STATIC_SIZE; // targetId + dirId + oldName + oldCreationTime + newName

    SameDirectoryRenameSnapshotReq() { clear(); }
    explicit SameDirectoryRenameSnapshotReq(InodeId targetId_, InodeId dirId_, BincodeBytes&& oldName_, TernTime oldCreationTime_, BincodeBytes&& newName_) : targetId(targetId_), dirId(dirId_), oldName(std::move(oldName_)), oldCreationTime(oldCreationTime_), newName(std::move(newName_)) {}
    SameDirectoryRenameSnapshotReq(const SameDirectoryRenameSnapshotReq&) = default;
    SameDirectoryRenameSnapshotReq& operator=(const SameDirectoryRenameSnapshotReq&) = default;
    SameDirectoryRenameSnapshotReq(SameDirectoryRenameSnapshotReq&&) = default;
    SameDirectoryRenameSnapshotReq& operator=(SameDirectoryRenameSnapshotReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // targetId
        _size += 8; // dirId
        _size += oldName.packedSize(); // oldName
        _size += 8; // oldCreationTime
        _size += newName.packedSize(); // newName
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SameDirectoryRenameSnapshotReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameSnapshotReq& x);

struct SameDirectoryRenameSnapshotResp {
    TernTime newCreationTime;

    static constexpr uint16_t STATIC_SIZE = 8; // newCreationTime

    SameDirectoryRenameSnapshotResp() { clear(); }
    explicit SameDirectoryRenameSnapshotResp(TernTime newCreationTime_) : newCreationTime(newCreationTime_) {}
    SameDirectoryRenameSnapshotResp(const SameDirectoryRenameSnapshotResp&) = default;
    SameDirectoryRenameSnapshotResp& operator=(const SameDirectoryRenameSnapshotResp&) = default;
    SameDirectoryRenameSnapshotResp(SameDirectoryRenameSnapshotResp&&) = default;
    SameDirectoryRenameSnapshotResp& operator=(SameDirectoryRenameSnapshotResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // newCreationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SameDirectoryRenameSnapshotResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameSnapshotResp& x);

struct AddSpanAtLocationInitiateReq {
    uint8_t locationId;
    AddSpanInitiateWithReferenceReq req;

    static constexpr uint16_t STATIC_SIZE = 1 + AddSpanInitiateWithReferenceReq::STATIC_SIZE; // locationId + req

    AddSpanAtLocationInitiateReq() { clear(); }
    explicit AddSpanAtLocationInitiateReq(uint8_t locationId_, AddSpanInitiateWithReferenceReq&& req_) : locationId(locationId_), req(std::move(req_)) {}
    AddSpanAtLocationInitiateReq(const AddSpanAtLocationInitiateReq&) = default;
    AddSpanAtLocationInitiateReq& operator=(const AddSpanAtLocationInitiateReq&) = default;
    AddSpanAtLocationInitiateReq(AddSpanAtLocationInitiateReq&&) = default;
    AddSpanAtLocationInitiateReq& operator=(AddSpanAtLocationInitiateReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // locationId
        _size += req.packedSize(); // req
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanAtLocationInitiateReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanAtLocationInitiateReq& x);

struct AddSpanAtLocationInitiateResp {
    AddSpanInitiateResp resp;

    static constexpr uint16_t STATIC_SIZE = AddSpanInitiateResp::STATIC_SIZE; // resp

    AddSpanAtLocationInitiateResp() { clear(); }
    explicit AddSpanAtLocationInitiateResp(AddSpanInitiateResp&& resp_) : resp(std::move(resp_)) {}
    AddSpanAtLocationInitiateResp(const AddSpanAtLocationInitiateResp&) = default;
    AddSpanAtLocationInitiateResp& operator=(const AddSpanAtLocationInitiateResp&) = default;
    AddSpanAtLocationInitiateResp(AddSpanAtLocationInitiateResp&&) = default;
    AddSpanAtLocationInitiateResp& operator=(AddSpanAtLocationInitiateResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += resp.packedSize(); // resp
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanAtLocationInitiateResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanAtLocationInitiateResp& x);

struct CreateDirectoryInodeReq {
    InodeId id;
    InodeId ownerId;
    DirectoryInfo info;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + DirectoryInfo::STATIC_SIZE; // id + ownerId + info

    CreateDirectoryInodeReq() { clear(); }
    explicit CreateDirectoryInodeReq(InodeId id_, InodeId ownerId_, DirectoryInfo&& info_) : id(id_), ownerId(ownerId_), info(std::move(info_)) {}
    CreateDirectoryInodeReq(const CreateDirectoryInodeReq&) = default;
    CreateDirectoryInodeReq& operator=(const CreateDirectoryInodeReq&) = default;
    CreateDirectoryInodeReq(CreateDirectoryInodeReq&&) = default;
    CreateDirectoryInodeReq& operator=(CreateDirectoryInodeReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 8; // ownerId
        _size += info.packedSize(); // info
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CreateDirectoryInodeReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CreateDirectoryInodeReq& x);

struct CreateDirectoryInodeResp {
    TernTime mtime;

    static constexpr uint16_t STATIC_SIZE = 8; // mtime

    CreateDirectoryInodeResp() { clear(); }
    explicit CreateDirectoryInodeResp(TernTime mtime_) : mtime(mtime_) {}
    CreateDirectoryInodeResp(const CreateDirectoryInodeResp&) = default;
    CreateDirectoryInodeResp& operator=(const CreateDirectoryInodeResp&) = default;
    CreateDirectoryInodeResp(CreateDirectoryInodeResp&&) = default;
    CreateDirectoryInodeResp& operator=(CreateDirectoryInodeResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // mtime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CreateDirectoryInodeResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CreateDirectoryInodeResp& x);

struct SetDirectoryOwnerReq {
    InodeId dirId;
    InodeId ownerId;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // dirId + ownerId

    SetDirectoryOwnerReq() { clear(); }
    explicit SetDirectoryOwnerReq(InodeId dirId_, InodeId ownerId_) : dirId(dirId_), ownerId(ownerId_) {}
    SetDirectoryOwnerReq(const SetDirectoryOwnerReq&) = default;
    SetDirectoryOwnerReq& operator=(const SetDirectoryOwnerReq&) = default;
    SetDirectoryOwnerReq(SetDirectoryOwnerReq&&) = default;
    SetDirectoryOwnerReq& operator=(SetDirectoryOwnerReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += 8; // ownerId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetDirectoryOwnerReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetDirectoryOwnerReq& x);

struct SetDirectoryOwnerResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    SetDirectoryOwnerResp() { clear(); }
    SetDirectoryOwnerResp(const SetDirectoryOwnerResp&) = default;
    SetDirectoryOwnerResp& operator=(const SetDirectoryOwnerResp&) = default;
    SetDirectoryOwnerResp(SetDirectoryOwnerResp&&) = default;
    SetDirectoryOwnerResp& operator=(SetDirectoryOwnerResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetDirectoryOwnerResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetDirectoryOwnerResp& x);

struct RemoveDirectoryOwnerReq {
    InodeId dirId;
    DirectoryInfo info;

    static constexpr uint16_t STATIC_SIZE = 8 + DirectoryInfo::STATIC_SIZE; // dirId + info

    RemoveDirectoryOwnerReq() { clear(); }
    explicit RemoveDirectoryOwnerReq(InodeId dirId_, DirectoryInfo&& info_) : dirId(dirId_), info(std::move(info_)) {}
    RemoveDirectoryOwnerReq(const RemoveDirectoryOwnerReq&) = default;
    RemoveDirectoryOwnerReq& operator=(const RemoveDirectoryOwnerReq&) = default;
    RemoveDirectoryOwnerReq(RemoveDirectoryOwnerReq&&) = default;
    RemoveDirectoryOwnerReq& operator=(RemoveDirectoryOwnerReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += info.packedSize(); // info
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveDirectoryOwnerReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveDirectoryOwnerReq& x);

struct RemoveDirectoryOwnerResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RemoveDirectoryOwnerResp() { clear(); }
    RemoveDirectoryOwnerResp(const RemoveDirectoryOwnerResp&) = default;
    RemoveDirectoryOwnerResp& operator=(const RemoveDirectoryOwnerResp&) = default;
    RemoveDirectoryOwnerResp(RemoveDirectoryOwnerResp&&) = default;
    RemoveDirectoryOwnerResp& operator=(RemoveDirectoryOwnerResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveDirectoryOwnerResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveDirectoryOwnerResp& x);

struct CreateLockedCurrentEdgeReq {
    InodeId dirId;
    BincodeBytes name;
    InodeId targetId;
    TernTime oldCreationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeBytes::STATIC_SIZE + 8 + 8; // dirId + name + targetId + oldCreationTime

    CreateLockedCurrentEdgeReq() { clear(); }
    explicit CreateLockedCurrentEdgeReq(InodeId dirId_, BincodeBytes&& name_, InodeId targetId_, TernTime oldCreationTime_) : dirId(dirId_), name(std::move(name_)), targetId(targetId_), oldCreationTime(oldCreationTime_) {}
    CreateLockedCurrentEdgeReq(const CreateLockedCurrentEdgeReq&) = default;
    CreateLockedCurrentEdgeReq& operator=(const CreateLockedCurrentEdgeReq&) = default;
    CreateLockedCurrentEdgeReq(CreateLockedCurrentEdgeReq&&) = default;
    CreateLockedCurrentEdgeReq& operator=(CreateLockedCurrentEdgeReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += name.packedSize(); // name
        _size += 8; // targetId
        _size += 8; // oldCreationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CreateLockedCurrentEdgeReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CreateLockedCurrentEdgeReq& x);

struct CreateLockedCurrentEdgeResp {
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8; // creationTime

    CreateLockedCurrentEdgeResp() { clear(); }
    explicit CreateLockedCurrentEdgeResp(TernTime creationTime_) : creationTime(creationTime_) {}
    CreateLockedCurrentEdgeResp(const CreateLockedCurrentEdgeResp&) = default;
    CreateLockedCurrentEdgeResp& operator=(const CreateLockedCurrentEdgeResp&) = default;
    CreateLockedCurrentEdgeResp(CreateLockedCurrentEdgeResp&&) = default;
    CreateLockedCurrentEdgeResp& operator=(CreateLockedCurrentEdgeResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CreateLockedCurrentEdgeResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CreateLockedCurrentEdgeResp& x);

struct LockCurrentEdgeReq {
    InodeId dirId;
    InodeId targetId;
    TernTime creationTime;
    BincodeBytes name;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 8 + BincodeBytes::STATIC_SIZE; // dirId + targetId + creationTime + name

    LockCurrentEdgeReq() { clear(); }
    explicit LockCurrentEdgeReq(InodeId dirId_, InodeId targetId_, TernTime creationTime_, BincodeBytes&& name_) : dirId(dirId_), targetId(targetId_), creationTime(creationTime_), name(std::move(name_)) {}
    LockCurrentEdgeReq(const LockCurrentEdgeReq&) = default;
    LockCurrentEdgeReq& operator=(const LockCurrentEdgeReq&) = default;
    LockCurrentEdgeReq(LockCurrentEdgeReq&&) = default;
    LockCurrentEdgeReq& operator=(LockCurrentEdgeReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += 8; // targetId
        _size += 8; // creationTime
        _size += name.packedSize(); // name
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LockCurrentEdgeReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LockCurrentEdgeReq& x);

struct LockCurrentEdgeResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    LockCurrentEdgeResp() { clear(); }
    LockCurrentEdgeResp(const LockCurrentEdgeResp&) = default;
    LockCurrentEdgeResp& operator=(const LockCurrentEdgeResp&) = default;
    LockCurrentEdgeResp(LockCurrentEdgeResp&&) = default;
    LockCurrentEdgeResp& operator=(LockCurrentEdgeResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LockCurrentEdgeResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LockCurrentEdgeResp& x);

struct UnlockCurrentEdgeReq {
    InodeId dirId;
    BincodeBytes name;
    TernTime creationTime;
    InodeId targetId;
    bool wasMoved;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeBytes::STATIC_SIZE + 8 + 8 + 1; // dirId + name + creationTime + targetId + wasMoved

    UnlockCurrentEdgeReq() { clear(); }
    explicit UnlockCurrentEdgeReq(InodeId dirId_, BincodeBytes&& name_, TernTime creationTime_, InodeId targetId_, bool wasMoved_) : dirId(dirId_), name(std::move(name_)), creationTime(creationTime_), targetId(targetId_), wasMoved(wasMoved_) {}
    UnlockCurrentEdgeReq(const UnlockCurrentEdgeReq&) = default;
    UnlockCurrentEdgeReq& operator=(const UnlockCurrentEdgeReq&) = default;
    UnlockCurrentEdgeReq(UnlockCurrentEdgeReq&&) = default;
    UnlockCurrentEdgeReq& operator=(UnlockCurrentEdgeReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        _size += 8; // targetId
        _size += 1; // wasMoved
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const UnlockCurrentEdgeReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const UnlockCurrentEdgeReq& x);

struct UnlockCurrentEdgeResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    UnlockCurrentEdgeResp() { clear(); }
    UnlockCurrentEdgeResp(const UnlockCurrentEdgeResp&) = default;
    UnlockCurrentEdgeResp& operator=(const UnlockCurrentEdgeResp&) = default;
    UnlockCurrentEdgeResp(UnlockCurrentEdgeResp&&) = default;
    UnlockCurrentEdgeResp& operator=(UnlockCurrentEdgeResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const UnlockCurrentEdgeResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const UnlockCurrentEdgeResp& x);

struct RemoveOwnedSnapshotFileEdgeReq {
    InodeId ownerId;
    InodeId targetId;
    BincodeBytes name;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8; // ownerId + targetId + name + creationTime

    RemoveOwnedSnapshotFileEdgeReq() { clear(); }
    explicit RemoveOwnedSnapshotFileEdgeReq(InodeId ownerId_, InodeId targetId_, BincodeBytes&& name_, TernTime creationTime_) : ownerId(ownerId_), targetId(targetId_), name(std::move(name_)), creationTime(creationTime_) {}
    RemoveOwnedSnapshotFileEdgeReq(const RemoveOwnedSnapshotFileEdgeReq&) = default;
    RemoveOwnedSnapshotFileEdgeReq& operator=(const RemoveOwnedSnapshotFileEdgeReq&) = default;
    RemoveOwnedSnapshotFileEdgeReq(RemoveOwnedSnapshotFileEdgeReq&&) = default;
    RemoveOwnedSnapshotFileEdgeReq& operator=(RemoveOwnedSnapshotFileEdgeReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // ownerId
        _size += 8; // targetId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveOwnedSnapshotFileEdgeReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveOwnedSnapshotFileEdgeReq& x);

struct RemoveOwnedSnapshotFileEdgeResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RemoveOwnedSnapshotFileEdgeResp() { clear(); }
    RemoveOwnedSnapshotFileEdgeResp(const RemoveOwnedSnapshotFileEdgeResp&) = default;
    RemoveOwnedSnapshotFileEdgeResp& operator=(const RemoveOwnedSnapshotFileEdgeResp&) = default;
    RemoveOwnedSnapshotFileEdgeResp(RemoveOwnedSnapshotFileEdgeResp&&) = default;
    RemoveOwnedSnapshotFileEdgeResp& operator=(RemoveOwnedSnapshotFileEdgeResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveOwnedSnapshotFileEdgeResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveOwnedSnapshotFileEdgeResp& x);

struct MakeFileTransientReq {
    InodeId id;
    BincodeBytes note;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeBytes::STATIC_SIZE; // id + note

    MakeFileTransientReq() { clear(); }
    explicit MakeFileTransientReq(InodeId id_, BincodeBytes&& note_) : id(id_), note(std::move(note_)) {}
    MakeFileTransientReq(const MakeFileTransientReq&) = default;
    MakeFileTransientReq& operator=(const MakeFileTransientReq&) = default;
    MakeFileTransientReq(MakeFileTransientReq&&) = default;
    MakeFileTransientReq& operator=(MakeFileTransientReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += note.packedSize(); // note
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MakeFileTransientReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MakeFileTransientReq& x);

struct MakeFileTransientResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    MakeFileTransientResp() { clear(); }
    MakeFileTransientResp(const MakeFileTransientResp&) = default;
    MakeFileTransientResp& operator=(const MakeFileTransientResp&) = default;
    MakeFileTransientResp(MakeFileTransientResp&&) = default;
    MakeFileTransientResp& operator=(MakeFileTransientResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MakeFileTransientResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MakeFileTransientResp& x);

struct MakeDirectoryReq {
    InodeId ownerId;
    BincodeBytes name;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeBytes::STATIC_SIZE; // ownerId + name

    MakeDirectoryReq() { clear(); }
    explicit MakeDirectoryReq(InodeId ownerId_, BincodeBytes&& name_) : ownerId(ownerId_), name(std::move(name_)) {}
    MakeDirectoryReq(const MakeDirectoryReq&) = default;
    MakeDirectoryReq& operator=(const MakeDirectoryReq&) = default;
    MakeDirectoryReq(MakeDirectoryReq&&) = default;
    MakeDirectoryReq& operator=(MakeDirectoryReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // ownerId
        _size += name.packedSize(); // name
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MakeDirectoryReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MakeDirectoryReq& x);

struct MakeDirectoryResp {
    InodeId id;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // id + creationTime

    MakeDirectoryResp() { clear(); }
    explicit MakeDirectoryResp(InodeId id_, TernTime creationTime_) : id(id_), creationTime(creationTime_) {}
    MakeDirectoryResp(const MakeDirectoryResp&) = default;
    MakeDirectoryResp& operator=(const MakeDirectoryResp&) = default;
    MakeDirectoryResp(MakeDirectoryResp&&) = default;
    MakeDirectoryResp& operator=(MakeDirectoryResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MakeDirectoryResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MakeDirectoryResp& x);

struct RenameFileReq {
    InodeId targetId;
    InodeId oldOwnerId;
    BincodeBytes oldName;
    TernTime oldCreationTime;
    InodeId newOwnerId;
    BincodeBytes newName;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8 + 8 + BincodeBytes::STATIC_SIZE; // targetId + oldOwnerId + oldName + oldCreationTime + newOwnerId + newName

    RenameFileReq() { clear(); }
    explicit RenameFileReq(InodeId targetId_, InodeId oldOwnerId_, BincodeBytes&& oldName_, TernTime oldCreationTime_, InodeId newOwnerId_, BincodeBytes&& newName_) : targetId(targetId_), oldOwnerId(oldOwnerId_), oldName(std::move(oldName_)), oldCreationTime(oldCreationTime_), newOwnerId(newOwnerId_), newName(std::move(newName_)) {}
    RenameFileReq(const RenameFileReq&) = default;
    RenameFileReq& operator=(const RenameFileReq&) = default;
    RenameFileReq(RenameFileReq&&) = default;
    RenameFileReq& operator=(RenameFileReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // targetId
        _size += 8; // oldOwnerId
        _size += oldName.packedSize(); // oldName
        _size += 8; // oldCreationTime
        _size += 8; // newOwnerId
        _size += newName.packedSize(); // newName
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RenameFileReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RenameFileReq& x);

struct RenameFileResp {
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8; // creationTime

    RenameFileResp() { clear(); }
    explicit RenameFileResp(TernTime creationTime_) : creationTime(creationTime_) {}
    RenameFileResp(const RenameFileResp&) = default;
    RenameFileResp& operator=(const RenameFileResp&) = default;
    RenameFileResp(RenameFileResp&&) = default;
    RenameFileResp& operator=(RenameFileResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RenameFileResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RenameFileResp& x);

struct SoftUnlinkDirectoryReq {
    InodeId ownerId;
    InodeId targetId;
    TernTime creationTime;
    BincodeBytes name;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 8 + BincodeBytes::STATIC_SIZE; // ownerId + targetId + creationTime + name

    SoftUnlinkDirectoryReq() { clear(); }
    explicit SoftUnlinkDirectoryReq(InodeId ownerId_, InodeId targetId_, TernTime creationTime_, BincodeBytes&& name_) : ownerId(ownerId_), targetId(targetId_), creationTime(creationTime_), name(std::move(name_)) {}
    SoftUnlinkDirectoryReq(const SoftUnlinkDirectoryReq&) = default;
    SoftUnlinkDirectoryReq& operator=(const SoftUnlinkDirectoryReq&) = default;
    SoftUnlinkDirectoryReq(SoftUnlinkDirectoryReq&&) = default;
    SoftUnlinkDirectoryReq& operator=(SoftUnlinkDirectoryReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // ownerId
        _size += 8; // targetId
        _size += 8; // creationTime
        _size += name.packedSize(); // name
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SoftUnlinkDirectoryReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SoftUnlinkDirectoryReq& x);

struct SoftUnlinkDirectoryResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    SoftUnlinkDirectoryResp() { clear(); }
    SoftUnlinkDirectoryResp(const SoftUnlinkDirectoryResp&) = default;
    SoftUnlinkDirectoryResp& operator=(const SoftUnlinkDirectoryResp&) = default;
    SoftUnlinkDirectoryResp(SoftUnlinkDirectoryResp&&) = default;
    SoftUnlinkDirectoryResp& operator=(SoftUnlinkDirectoryResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SoftUnlinkDirectoryResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SoftUnlinkDirectoryResp& x);

struct RenameDirectoryReq {
    InodeId targetId;
    InodeId oldOwnerId;
    BincodeBytes oldName;
    TernTime oldCreationTime;
    InodeId newOwnerId;
    BincodeBytes newName;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8 + 8 + BincodeBytes::STATIC_SIZE; // targetId + oldOwnerId + oldName + oldCreationTime + newOwnerId + newName

    RenameDirectoryReq() { clear(); }
    explicit RenameDirectoryReq(InodeId targetId_, InodeId oldOwnerId_, BincodeBytes&& oldName_, TernTime oldCreationTime_, InodeId newOwnerId_, BincodeBytes&& newName_) : targetId(targetId_), oldOwnerId(oldOwnerId_), oldName(std::move(oldName_)), oldCreationTime(oldCreationTime_), newOwnerId(newOwnerId_), newName(std::move(newName_)) {}
    RenameDirectoryReq(const RenameDirectoryReq&) = default;
    RenameDirectoryReq& operator=(const RenameDirectoryReq&) = default;
    RenameDirectoryReq(RenameDirectoryReq&&) = default;
    RenameDirectoryReq& operator=(RenameDirectoryReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // targetId
        _size += 8; // oldOwnerId
        _size += oldName.packedSize(); // oldName
        _size += 8; // oldCreationTime
        _size += 8; // newOwnerId
        _size += newName.packedSize(); // newName
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RenameDirectoryReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RenameDirectoryReq& x);

struct RenameDirectoryResp {
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8; // creationTime

    RenameDirectoryResp() { clear(); }
    explicit RenameDirectoryResp(TernTime creationTime_) : creationTime(creationTime_) {}
    RenameDirectoryResp(const RenameDirectoryResp&) = default;
    RenameDirectoryResp& operator=(const RenameDirectoryResp&) = default;
    RenameDirectoryResp(RenameDirectoryResp&&) = default;
    RenameDirectoryResp& operator=(RenameDirectoryResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RenameDirectoryResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RenameDirectoryResp& x);

struct CrossShardHardUnlinkFileReq {
    InodeId ownerId;
    InodeId targetId;
    BincodeBytes name;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8; // ownerId + targetId + name + creationTime

    CrossShardHardUnlinkFileReq() { clear(); }
    explicit CrossShardHardUnlinkFileReq(InodeId ownerId_, InodeId targetId_, BincodeBytes&& name_, TernTime creationTime_) : ownerId(ownerId_), targetId(targetId_), name(std::move(name_)), creationTime(creationTime_) {}
    CrossShardHardUnlinkFileReq(const CrossShardHardUnlinkFileReq&) = default;
    CrossShardHardUnlinkFileReq& operator=(const CrossShardHardUnlinkFileReq&) = default;
    CrossShardHardUnlinkFileReq(CrossShardHardUnlinkFileReq&&) = default;
    CrossShardHardUnlinkFileReq& operator=(CrossShardHardUnlinkFileReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // ownerId
        _size += 8; // targetId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CrossShardHardUnlinkFileReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CrossShardHardUnlinkFileReq& x);

struct CrossShardHardUnlinkFileResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    CrossShardHardUnlinkFileResp() { clear(); }
    CrossShardHardUnlinkFileResp(const CrossShardHardUnlinkFileResp&) = default;
    CrossShardHardUnlinkFileResp& operator=(const CrossShardHardUnlinkFileResp&) = default;
    CrossShardHardUnlinkFileResp(CrossShardHardUnlinkFileResp&&) = default;
    CrossShardHardUnlinkFileResp& operator=(CrossShardHardUnlinkFileResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CrossShardHardUnlinkFileResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CrossShardHardUnlinkFileResp& x);

struct HardUnlinkDirectoryReq {
    InodeId dirId;

    static constexpr uint16_t STATIC_SIZE = 8; // dirId

    HardUnlinkDirectoryReq() { clear(); }
    explicit HardUnlinkDirectoryReq(InodeId dirId_) : dirId(dirId_) {}
    HardUnlinkDirectoryReq(const HardUnlinkDirectoryReq&) = default;
    HardUnlinkDirectoryReq& operator=(const HardUnlinkDirectoryReq&) = default;
    HardUnlinkDirectoryReq(HardUnlinkDirectoryReq&&) = default;
    HardUnlinkDirectoryReq& operator=(HardUnlinkDirectoryReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const HardUnlinkDirectoryReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const HardUnlinkDirectoryReq& x);

struct HardUnlinkDirectoryResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    HardUnlinkDirectoryResp() { clear(); }
    HardUnlinkDirectoryResp(const HardUnlinkDirectoryResp&) = default;
    HardUnlinkDirectoryResp& operator=(const HardUnlinkDirectoryResp&) = default;
    HardUnlinkDirectoryResp(HardUnlinkDirectoryResp&&) = default;
    HardUnlinkDirectoryResp& operator=(HardUnlinkDirectoryResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const HardUnlinkDirectoryResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const HardUnlinkDirectoryResp& x);

struct CdcSnapshotReq {
    uint64_t snapshotId;

    static constexpr uint16_t STATIC_SIZE = 8; // snapshotId

    CdcSnapshotReq() { clear(); }
    explicit CdcSnapshotReq(uint64_t snapshotId_) : snapshotId(snapshotId_) {}
    CdcSnapshotReq(const CdcSnapshotReq&) = default;
    CdcSnapshotReq& operator=(const CdcSnapshotReq&) = default;
    CdcSnapshotReq(CdcSnapshotReq&&) = default;
    CdcSnapshotReq& operator=(CdcSnapshotReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // snapshotId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CdcSnapshotReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CdcSnapshotReq& x);

struct CdcSnapshotResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    CdcSnapshotResp() { clear(); }
    CdcSnapshotResp(const CdcSnapshotResp&) = default;
    CdcSnapshotResp& operator=(const CdcSnapshotResp&) = default;
    CdcSnapshotResp(CdcSnapshotResp&&) = default;
    CdcSnapshotResp& operator=(CdcSnapshotResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CdcSnapshotResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CdcSnapshotResp& x);

struct LocalShardsReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    LocalShardsReq() { clear(); }
    LocalShardsReq(const LocalShardsReq&) = default;
    LocalShardsReq& operator=(const LocalShardsReq&) = default;
    LocalShardsReq(LocalShardsReq&&) = default;
    LocalShardsReq& operator=(LocalShardsReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocalShardsReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocalShardsReq& x);

struct LocalShardsResp {
    BincodeList<ShardInfo> shards;

    static constexpr uint16_t STATIC_SIZE = BincodeList<ShardInfo>::STATIC_SIZE; // shards

    LocalShardsResp() { clear(); }
    explicit LocalShardsResp(BincodeList<ShardInfo>&& shards_) : shards(std::move(shards_)) {}
    LocalShardsResp(const LocalShardsResp&) = default;
    LocalShardsResp& operator=(const LocalShardsResp&) = default;
    LocalShardsResp(LocalShardsResp&&) = default;
    LocalShardsResp& operator=(LocalShardsResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += shards.packedSize(); // shards
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocalShardsResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocalShardsResp& x);

struct LocalCdcReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    LocalCdcReq() { clear(); }
    LocalCdcReq(const LocalCdcReq&) = default;
    LocalCdcReq& operator=(const LocalCdcReq&) = default;
    LocalCdcReq(LocalCdcReq&&) = default;
    LocalCdcReq& operator=(LocalCdcReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocalCdcReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocalCdcReq& x);

struct LocalCdcResp {
    AddrsInfo addrs;
    TernTime lastSeen;

    static constexpr uint16_t STATIC_SIZE = AddrsInfo::STATIC_SIZE + 8; // addrs + lastSeen

    LocalCdcResp() { clear(); }
    explicit LocalCdcResp(AddrsInfo addrs_, TernTime lastSeen_) : addrs(addrs_), lastSeen(lastSeen_) {}
    LocalCdcResp(const LocalCdcResp&) = default;
    LocalCdcResp& operator=(const LocalCdcResp&) = default;
    LocalCdcResp(LocalCdcResp&&) = default;
    LocalCdcResp& operator=(LocalCdcResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += addrs.packedSize(); // addrs
        _size += 8; // lastSeen
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocalCdcResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocalCdcResp& x);

struct InfoReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    InfoReq() { clear(); }
    InfoReq(const InfoReq&) = default;
    InfoReq& operator=(const InfoReq&) = default;
    InfoReq(InfoReq&&) = default;
    InfoReq& operator=(InfoReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const InfoReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const InfoReq& x);

struct InfoResp {
    uint32_t numBlockServices;
    uint32_t numFailureDomains;
    uint64_t capacity;
    uint64_t available;
    uint64_t blocks;

    static constexpr uint16_t STATIC_SIZE = 4 + 4 + 8 + 8 + 8; // numBlockServices + numFailureDomains + capacity + available + blocks

    InfoResp() { clear(); }
    explicit InfoResp(uint32_t numBlockServices_, uint32_t numFailureDomains_, uint64_t capacity_, uint64_t available_, uint64_t blocks_) : numBlockServices(numBlockServices_), numFailureDomains(numFailureDomains_), capacity(capacity_), available(available_), blocks(blocks_) {}
    InfoResp(const InfoResp&) = default;
    InfoResp& operator=(const InfoResp&) = default;
    InfoResp(InfoResp&&) = default;
    InfoResp& operator=(InfoResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 4; // numBlockServices
        _size += 4; // numFailureDomains
        _size += 8; // capacity
        _size += 8; // available
        _size += 8; // blocks
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const InfoResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const InfoResp& x);

struct RegistryReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RegistryReq() { clear(); }
    RegistryReq(const RegistryReq&) = default;
    RegistryReq& operator=(const RegistryReq&) = default;
    RegistryReq(RegistryReq&&) = default;
    RegistryReq& operator=(RegistryReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegistryReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegistryReq& x);

struct RegistryResp {
    AddrsInfo addrs;

    static constexpr uint16_t STATIC_SIZE = AddrsInfo::STATIC_SIZE; // addrs

    RegistryResp() { clear(); }
    explicit RegistryResp(AddrsInfo addrs_) : addrs(addrs_) {}
    RegistryResp(const RegistryResp&) = default;
    RegistryResp& operator=(const RegistryResp&) = default;
    RegistryResp(RegistryResp&&) = default;
    RegistryResp& operator=(RegistryResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += addrs.packedSize(); // addrs
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegistryResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegistryResp& x);

struct LocalChangedBlockServicesReq {
    TernTime changedSince;

    static constexpr uint16_t STATIC_SIZE = 8; // changedSince

    LocalChangedBlockServicesReq() { clear(); }
    explicit LocalChangedBlockServicesReq(TernTime changedSince_) : changedSince(changedSince_) {}
    LocalChangedBlockServicesReq(const LocalChangedBlockServicesReq&) = default;
    LocalChangedBlockServicesReq& operator=(const LocalChangedBlockServicesReq&) = default;
    LocalChangedBlockServicesReq(LocalChangedBlockServicesReq&&) = default;
    LocalChangedBlockServicesReq& operator=(LocalChangedBlockServicesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // changedSince
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocalChangedBlockServicesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocalChangedBlockServicesReq& x);

struct LocalChangedBlockServicesResp {
    TernTime lastChange;
    BincodeList<BlockService> blockServices;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeList<BlockService>::STATIC_SIZE; // lastChange + blockServices

    LocalChangedBlockServicesResp() { clear(); }
    explicit LocalChangedBlockServicesResp(TernTime lastChange_, BincodeList<BlockService>&& blockServices_) : lastChange(lastChange_), blockServices(std::move(blockServices_)) {}
    LocalChangedBlockServicesResp(const LocalChangedBlockServicesResp&) = default;
    LocalChangedBlockServicesResp& operator=(const LocalChangedBlockServicesResp&) = default;
    LocalChangedBlockServicesResp(LocalChangedBlockServicesResp&&) = default;
    LocalChangedBlockServicesResp& operator=(LocalChangedBlockServicesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // lastChange
        _size += blockServices.packedSize(); // blockServices
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocalChangedBlockServicesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocalChangedBlockServicesResp& x);

struct CreateLocationReq {
    uint8_t id;
    BincodeBytes name;

    static constexpr uint16_t STATIC_SIZE = 1 + BincodeBytes::STATIC_SIZE; // id + name

    CreateLocationReq() { clear(); }
    explicit CreateLocationReq(uint8_t id_, BincodeBytes&& name_) : id(id_), name(std::move(name_)) {}
    CreateLocationReq(const CreateLocationReq&) = default;
    CreateLocationReq& operator=(const CreateLocationReq&) = default;
    CreateLocationReq(CreateLocationReq&&) = default;
    CreateLocationReq& operator=(CreateLocationReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // id
        _size += name.packedSize(); // name
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CreateLocationReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CreateLocationReq& x);

struct CreateLocationResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    CreateLocationResp() { clear(); }
    CreateLocationResp(const CreateLocationResp&) = default;
    CreateLocationResp& operator=(const CreateLocationResp&) = default;
    CreateLocationResp(CreateLocationResp&&) = default;
    CreateLocationResp& operator=(CreateLocationResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CreateLocationResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CreateLocationResp& x);

struct RenameLocationReq {
    uint8_t id;
    BincodeBytes name;

    static constexpr uint16_t STATIC_SIZE = 1 + BincodeBytes::STATIC_SIZE; // id + name

    RenameLocationReq() { clear(); }
    explicit RenameLocationReq(uint8_t id_, BincodeBytes&& name_) : id(id_), name(std::move(name_)) {}
    RenameLocationReq(const RenameLocationReq&) = default;
    RenameLocationReq& operator=(const RenameLocationReq&) = default;
    RenameLocationReq(RenameLocationReq&&) = default;
    RenameLocationReq& operator=(RenameLocationReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // id
        _size += name.packedSize(); // name
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RenameLocationReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RenameLocationReq& x);

struct RenameLocationResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RenameLocationResp() { clear(); }
    RenameLocationResp(const RenameLocationResp&) = default;
    RenameLocationResp& operator=(const RenameLocationResp&) = default;
    RenameLocationResp(RenameLocationResp&&) = default;
    RenameLocationResp& operator=(RenameLocationResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RenameLocationResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RenameLocationResp& x);

struct RegisterShardReq {
    ShardReplicaId shrid;
    bool isLeader;
    AddrsInfo addrs;
    uint8_t location;

    static constexpr uint16_t STATIC_SIZE = 2 + 1 + AddrsInfo::STATIC_SIZE + 1; // shrid + isLeader + addrs + location

    RegisterShardReq() { clear(); }
    explicit RegisterShardReq(ShardReplicaId shrid_, bool isLeader_, AddrsInfo addrs_, uint8_t location_) : shrid(shrid_), isLeader(isLeader_), addrs(addrs_), location(location_) {}
    RegisterShardReq(const RegisterShardReq&) = default;
    RegisterShardReq& operator=(const RegisterShardReq&) = default;
    RegisterShardReq(RegisterShardReq&&) = default;
    RegisterShardReq& operator=(RegisterShardReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // shrid
        _size += 1; // isLeader
        _size += addrs.packedSize(); // addrs
        _size += 1; // location
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegisterShardReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegisterShardReq& x);

struct RegisterShardResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RegisterShardResp() { clear(); }
    RegisterShardResp(const RegisterShardResp&) = default;
    RegisterShardResp& operator=(const RegisterShardResp&) = default;
    RegisterShardResp(RegisterShardResp&&) = default;
    RegisterShardResp& operator=(RegisterShardResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegisterShardResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegisterShardResp& x);

struct LocationsReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    LocationsReq() { clear(); }
    LocationsReq(const LocationsReq&) = default;
    LocationsReq& operator=(const LocationsReq&) = default;
    LocationsReq(LocationsReq&&) = default;
    LocationsReq& operator=(LocationsReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocationsReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocationsReq& x);

struct LocationsResp {
    BincodeList<LocationInfo> locations;

    static constexpr uint16_t STATIC_SIZE = BincodeList<LocationInfo>::STATIC_SIZE; // locations

    LocationsResp() { clear(); }
    explicit LocationsResp(BincodeList<LocationInfo>&& locations_) : locations(std::move(locations_)) {}
    LocationsResp(const LocationsResp&) = default;
    LocationsResp& operator=(const LocationsResp&) = default;
    LocationsResp(LocationsResp&&) = default;
    LocationsResp& operator=(LocationsResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += locations.packedSize(); // locations
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LocationsResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LocationsResp& x);

struct RegisterCdcReq {
    ReplicaId replica;
    uint8_t location;
    bool isLeader;
    AddrsInfo addrs;

    static constexpr uint16_t STATIC_SIZE = 1 + 1 + 1 + AddrsInfo::STATIC_SIZE; // replica + location + isLeader + addrs

    RegisterCdcReq() { clear(); }
    explicit RegisterCdcReq(ReplicaId replica_, uint8_t location_, bool isLeader_, AddrsInfo addrs_) : replica(replica_), location(location_), isLeader(isLeader_), addrs(addrs_) {}
    RegisterCdcReq(const RegisterCdcReq&) = default;
    RegisterCdcReq& operator=(const RegisterCdcReq&) = default;
    RegisterCdcReq(RegisterCdcReq&&) = default;
    RegisterCdcReq& operator=(RegisterCdcReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // replica
        _size += 1; // location
        _size += 1; // isLeader
        _size += addrs.packedSize(); // addrs
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegisterCdcReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegisterCdcReq& x);

struct RegisterCdcResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RegisterCdcResp() { clear(); }
    RegisterCdcResp(const RegisterCdcResp&) = default;
    RegisterCdcResp& operator=(const RegisterCdcResp&) = default;
    RegisterCdcResp(RegisterCdcResp&&) = default;
    RegisterCdcResp& operator=(RegisterCdcResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegisterCdcResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegisterCdcResp& x);

struct SetBlockServiceFlagsReq {
    BlockServiceId id;
    BlockServiceFlags flags;
    uint8_t flagsMask;

    static constexpr uint16_t STATIC_SIZE = 8 + 1 + 1; // id + flags + flagsMask

    SetBlockServiceFlagsReq() { clear(); }
    explicit SetBlockServiceFlagsReq(BlockServiceId id_, BlockServiceFlags flags_, uint8_t flagsMask_) : id(id_), flags(flags_), flagsMask(flagsMask_) {}
    SetBlockServiceFlagsReq(const SetBlockServiceFlagsReq&) = default;
    SetBlockServiceFlagsReq& operator=(const SetBlockServiceFlagsReq&) = default;
    SetBlockServiceFlagsReq(SetBlockServiceFlagsReq&&) = default;
    SetBlockServiceFlagsReq& operator=(SetBlockServiceFlagsReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 1; // flags
        _size += 1; // flagsMask
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetBlockServiceFlagsReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetBlockServiceFlagsReq& x);

struct SetBlockServiceFlagsResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    SetBlockServiceFlagsResp() { clear(); }
    SetBlockServiceFlagsResp(const SetBlockServiceFlagsResp&) = default;
    SetBlockServiceFlagsResp& operator=(const SetBlockServiceFlagsResp&) = default;
    SetBlockServiceFlagsResp(SetBlockServiceFlagsResp&&) = default;
    SetBlockServiceFlagsResp& operator=(SetBlockServiceFlagsResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetBlockServiceFlagsResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetBlockServiceFlagsResp& x);

struct RegisterBlockServicesReq {
    BincodeList<RegisterBlockServiceInfo> blockServices;

    static constexpr uint16_t STATIC_SIZE = BincodeList<RegisterBlockServiceInfo>::STATIC_SIZE; // blockServices

    RegisterBlockServicesReq() { clear(); }
    explicit RegisterBlockServicesReq(BincodeList<RegisterBlockServiceInfo>&& blockServices_) : blockServices(std::move(blockServices_)) {}
    RegisterBlockServicesReq(const RegisterBlockServicesReq&) = default;
    RegisterBlockServicesReq& operator=(const RegisterBlockServicesReq&) = default;
    RegisterBlockServicesReq(RegisterBlockServicesReq&&) = default;
    RegisterBlockServicesReq& operator=(RegisterBlockServicesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += blockServices.packedSize(); // blockServices
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegisterBlockServicesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegisterBlockServicesReq& x);

struct RegisterBlockServicesResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RegisterBlockServicesResp() { clear(); }
    RegisterBlockServicesResp(const RegisterBlockServicesResp&) = default;
    RegisterBlockServicesResp& operator=(const RegisterBlockServicesResp&) = default;
    RegisterBlockServicesResp(RegisterBlockServicesResp&&) = default;
    RegisterBlockServicesResp& operator=(RegisterBlockServicesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegisterBlockServicesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegisterBlockServicesResp& x);

struct ChangedBlockServicesAtLocationReq {
    uint8_t locationId;
    TernTime changedSince;

    static constexpr uint16_t STATIC_SIZE = 1 + 8; // locationId + changedSince

    ChangedBlockServicesAtLocationReq() { clear(); }
    explicit ChangedBlockServicesAtLocationReq(uint8_t locationId_, TernTime changedSince_) : locationId(locationId_), changedSince(changedSince_) {}
    ChangedBlockServicesAtLocationReq(const ChangedBlockServicesAtLocationReq&) = default;
    ChangedBlockServicesAtLocationReq& operator=(const ChangedBlockServicesAtLocationReq&) = default;
    ChangedBlockServicesAtLocationReq(ChangedBlockServicesAtLocationReq&&) = default;
    ChangedBlockServicesAtLocationReq& operator=(ChangedBlockServicesAtLocationReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // locationId
        _size += 8; // changedSince
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ChangedBlockServicesAtLocationReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ChangedBlockServicesAtLocationReq& x);

struct ChangedBlockServicesAtLocationResp {
    TernTime lastChange;
    BincodeList<BlockService> blockServices;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeList<BlockService>::STATIC_SIZE; // lastChange + blockServices

    ChangedBlockServicesAtLocationResp() { clear(); }
    explicit ChangedBlockServicesAtLocationResp(TernTime lastChange_, BincodeList<BlockService>&& blockServices_) : lastChange(lastChange_), blockServices(std::move(blockServices_)) {}
    ChangedBlockServicesAtLocationResp(const ChangedBlockServicesAtLocationResp&) = default;
    ChangedBlockServicesAtLocationResp& operator=(const ChangedBlockServicesAtLocationResp&) = default;
    ChangedBlockServicesAtLocationResp(ChangedBlockServicesAtLocationResp&&) = default;
    ChangedBlockServicesAtLocationResp& operator=(ChangedBlockServicesAtLocationResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // lastChange
        _size += blockServices.packedSize(); // blockServices
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ChangedBlockServicesAtLocationResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ChangedBlockServicesAtLocationResp& x);

struct ShardsAtLocationReq {
    uint8_t locationId;

    static constexpr uint16_t STATIC_SIZE = 1; // locationId

    ShardsAtLocationReq() { clear(); }
    explicit ShardsAtLocationReq(uint8_t locationId_) : locationId(locationId_) {}
    ShardsAtLocationReq(const ShardsAtLocationReq&) = default;
    ShardsAtLocationReq& operator=(const ShardsAtLocationReq&) = default;
    ShardsAtLocationReq(ShardsAtLocationReq&&) = default;
    ShardsAtLocationReq& operator=(ShardsAtLocationReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // locationId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ShardsAtLocationReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ShardsAtLocationReq& x);

struct ShardsAtLocationResp {
    BincodeList<ShardInfo> shards;

    static constexpr uint16_t STATIC_SIZE = BincodeList<ShardInfo>::STATIC_SIZE; // shards

    ShardsAtLocationResp() { clear(); }
    explicit ShardsAtLocationResp(BincodeList<ShardInfo>&& shards_) : shards(std::move(shards_)) {}
    ShardsAtLocationResp(const ShardsAtLocationResp&) = default;
    ShardsAtLocationResp& operator=(const ShardsAtLocationResp&) = default;
    ShardsAtLocationResp(ShardsAtLocationResp&&) = default;
    ShardsAtLocationResp& operator=(ShardsAtLocationResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += shards.packedSize(); // shards
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ShardsAtLocationResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ShardsAtLocationResp& x);

struct CdcAtLocationReq {
    uint8_t locationId;

    static constexpr uint16_t STATIC_SIZE = 1; // locationId

    CdcAtLocationReq() { clear(); }
    explicit CdcAtLocationReq(uint8_t locationId_) : locationId(locationId_) {}
    CdcAtLocationReq(const CdcAtLocationReq&) = default;
    CdcAtLocationReq& operator=(const CdcAtLocationReq&) = default;
    CdcAtLocationReq(CdcAtLocationReq&&) = default;
    CdcAtLocationReq& operator=(CdcAtLocationReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // locationId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CdcAtLocationReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CdcAtLocationReq& x);

struct CdcAtLocationResp {
    AddrsInfo addrs;
    TernTime lastSeen;

    static constexpr uint16_t STATIC_SIZE = AddrsInfo::STATIC_SIZE + 8; // addrs + lastSeen

    CdcAtLocationResp() { clear(); }
    explicit CdcAtLocationResp(AddrsInfo addrs_, TernTime lastSeen_) : addrs(addrs_), lastSeen(lastSeen_) {}
    CdcAtLocationResp(const CdcAtLocationResp&) = default;
    CdcAtLocationResp& operator=(const CdcAtLocationResp&) = default;
    CdcAtLocationResp(CdcAtLocationResp&&) = default;
    CdcAtLocationResp& operator=(CdcAtLocationResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += addrs.packedSize(); // addrs
        _size += 8; // lastSeen
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CdcAtLocationResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CdcAtLocationResp& x);

struct RegisterRegistryReq {
    ReplicaId replicaId;
    uint8_t location;
    bool isLeader;
    AddrsInfo addrs;
    bool bootstrap;

    static constexpr uint16_t STATIC_SIZE = 1 + 1 + 1 + AddrsInfo::STATIC_SIZE + 1; // replicaId + location + isLeader + addrs + bootstrap

    RegisterRegistryReq() { clear(); }
    explicit RegisterRegistryReq(ReplicaId replicaId_, uint8_t location_, bool isLeader_, AddrsInfo addrs_, bool bootstrap_) : replicaId(replicaId_), location(location_), isLeader(isLeader_), addrs(addrs_), bootstrap(bootstrap_) {}
    RegisterRegistryReq(const RegisterRegistryReq&) = default;
    RegisterRegistryReq& operator=(const RegisterRegistryReq&) = default;
    RegisterRegistryReq(RegisterRegistryReq&&) = default;
    RegisterRegistryReq& operator=(RegisterRegistryReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // replicaId
        _size += 1; // location
        _size += 1; // isLeader
        _size += addrs.packedSize(); // addrs
        _size += 1; // bootstrap
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegisterRegistryReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegisterRegistryReq& x);

struct RegisterRegistryResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    RegisterRegistryResp() { clear(); }
    RegisterRegistryResp(const RegisterRegistryResp&) = default;
    RegisterRegistryResp& operator=(const RegisterRegistryResp&) = default;
    RegisterRegistryResp(RegisterRegistryResp&&) = default;
    RegisterRegistryResp& operator=(RegisterRegistryResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RegisterRegistryResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RegisterRegistryResp& x);

struct AllRegistryReplicasReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    AllRegistryReplicasReq() { clear(); }
    AllRegistryReplicasReq(const AllRegistryReplicasReq&) = default;
    AllRegistryReplicasReq& operator=(const AllRegistryReplicasReq&) = default;
    AllRegistryReplicasReq(AllRegistryReplicasReq&&) = default;
    AllRegistryReplicasReq& operator=(AllRegistryReplicasReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AllRegistryReplicasReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AllRegistryReplicasReq& x);

struct AllRegistryReplicasResp {
    BincodeList<FullRegistryInfo> replicas;

    static constexpr uint16_t STATIC_SIZE = BincodeList<FullRegistryInfo>::STATIC_SIZE; // replicas

    AllRegistryReplicasResp() { clear(); }
    explicit AllRegistryReplicasResp(BincodeList<FullRegistryInfo>&& replicas_) : replicas(std::move(replicas_)) {}
    AllRegistryReplicasResp(const AllRegistryReplicasResp&) = default;
    AllRegistryReplicasResp& operator=(const AllRegistryReplicasResp&) = default;
    AllRegistryReplicasResp(AllRegistryReplicasResp&&) = default;
    AllRegistryReplicasResp& operator=(AllRegistryReplicasResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += replicas.packedSize(); // replicas
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AllRegistryReplicasResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AllRegistryReplicasResp& x);

struct ShardBlockServicesDEPRECATEDReq {
    ShardId shardId;

    static constexpr uint16_t STATIC_SIZE = 1; // shardId

    ShardBlockServicesDEPRECATEDReq() { clear(); }
    explicit ShardBlockServicesDEPRECATEDReq(ShardId shardId_) : shardId(shardId_) {}
    ShardBlockServicesDEPRECATEDReq(const ShardBlockServicesDEPRECATEDReq&) = default;
    ShardBlockServicesDEPRECATEDReq& operator=(const ShardBlockServicesDEPRECATEDReq&) = default;
    ShardBlockServicesDEPRECATEDReq(ShardBlockServicesDEPRECATEDReq&&) = default;
    ShardBlockServicesDEPRECATEDReq& operator=(ShardBlockServicesDEPRECATEDReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // shardId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ShardBlockServicesDEPRECATEDReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ShardBlockServicesDEPRECATEDReq& x);

struct ShardBlockServicesDEPRECATEDResp {
    BincodeList<BlockServiceId> blockServices;

    static constexpr uint16_t STATIC_SIZE = BincodeList<BlockServiceId>::STATIC_SIZE; // blockServices

    ShardBlockServicesDEPRECATEDResp() { clear(); }
    explicit ShardBlockServicesDEPRECATEDResp(BincodeList<BlockServiceId>&& blockServices_) : blockServices(std::move(blockServices_)) {}
    ShardBlockServicesDEPRECATEDResp(const ShardBlockServicesDEPRECATEDResp&) = default;
    ShardBlockServicesDEPRECATEDResp& operator=(const ShardBlockServicesDEPRECATEDResp&) = default;
    ShardBlockServicesDEPRECATEDResp(ShardBlockServicesDEPRECATEDResp&&) = default;
    ShardBlockServicesDEPRECATEDResp& operator=(ShardBlockServicesDEPRECATEDResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += blockServices.packedSize(); // blockServices
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ShardBlockServicesDEPRECATEDResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ShardBlockServicesDEPRECATEDResp& x);

struct CdcReplicasDEPRECATEDReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    CdcReplicasDEPRECATEDReq() { clear(); }
    CdcReplicasDEPRECATEDReq(const CdcReplicasDEPRECATEDReq&) = default;
    CdcReplicasDEPRECATEDReq& operator=(const CdcReplicasDEPRECATEDReq&) = default;
    CdcReplicasDEPRECATEDReq(CdcReplicasDEPRECATEDReq&&) = default;
    CdcReplicasDEPRECATEDReq& operator=(CdcReplicasDEPRECATEDReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CdcReplicasDEPRECATEDReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CdcReplicasDEPRECATEDReq& x);

struct CdcReplicasDEPRECATEDResp {
    BincodeList<AddrsInfo> replicas;

    static constexpr uint16_t STATIC_SIZE = BincodeList<AddrsInfo>::STATIC_SIZE; // replicas

    CdcReplicasDEPRECATEDResp() { clear(); }
    explicit CdcReplicasDEPRECATEDResp(BincodeList<AddrsInfo>&& replicas_) : replicas(std::move(replicas_)) {}
    CdcReplicasDEPRECATEDResp(const CdcReplicasDEPRECATEDResp&) = default;
    CdcReplicasDEPRECATEDResp& operator=(const CdcReplicasDEPRECATEDResp&) = default;
    CdcReplicasDEPRECATEDResp(CdcReplicasDEPRECATEDResp&&) = default;
    CdcReplicasDEPRECATEDResp& operator=(CdcReplicasDEPRECATEDResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += replicas.packedSize(); // replicas
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CdcReplicasDEPRECATEDResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CdcReplicasDEPRECATEDResp& x);

struct AllShardsReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    AllShardsReq() { clear(); }
    AllShardsReq(const AllShardsReq&) = default;
    AllShardsReq& operator=(const AllShardsReq&) = default;
    AllShardsReq(AllShardsReq&&) = default;
    AllShardsReq& operator=(AllShardsReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AllShardsReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AllShardsReq& x);

struct AllShardsResp {
    BincodeList<FullShardInfo> shards;

    static constexpr uint16_t STATIC_SIZE = BincodeList<FullShardInfo>::STATIC_SIZE; // shards

    AllShardsResp() { clear(); }
    explicit AllShardsResp(BincodeList<FullShardInfo>&& shards_) : shards(std::move(shards_)) {}
    AllShardsResp(const AllShardsResp&) = default;
    AllShardsResp& operator=(const AllShardsResp&) = default;
    AllShardsResp(AllShardsResp&&) = default;
    AllShardsResp& operator=(AllShardsResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += shards.packedSize(); // shards
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AllShardsResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AllShardsResp& x);

struct DecommissionBlockServiceReq {
    BlockServiceId id;

    static constexpr uint16_t STATIC_SIZE = 8; // id

    DecommissionBlockServiceReq() { clear(); }
    explicit DecommissionBlockServiceReq(BlockServiceId id_) : id(id_) {}
    DecommissionBlockServiceReq(const DecommissionBlockServiceReq&) = default;
    DecommissionBlockServiceReq& operator=(const DecommissionBlockServiceReq&) = default;
    DecommissionBlockServiceReq(DecommissionBlockServiceReq&&) = default;
    DecommissionBlockServiceReq& operator=(DecommissionBlockServiceReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const DecommissionBlockServiceReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const DecommissionBlockServiceReq& x);

struct DecommissionBlockServiceResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    DecommissionBlockServiceResp() { clear(); }
    DecommissionBlockServiceResp(const DecommissionBlockServiceResp&) = default;
    DecommissionBlockServiceResp& operator=(const DecommissionBlockServiceResp&) = default;
    DecommissionBlockServiceResp(DecommissionBlockServiceResp&&) = default;
    DecommissionBlockServiceResp& operator=(DecommissionBlockServiceResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const DecommissionBlockServiceResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const DecommissionBlockServiceResp& x);

struct MoveShardLeaderReq {
    ShardReplicaId shrid;
    uint8_t location;

    static constexpr uint16_t STATIC_SIZE = 2 + 1; // shrid + location

    MoveShardLeaderReq() { clear(); }
    explicit MoveShardLeaderReq(ShardReplicaId shrid_, uint8_t location_) : shrid(shrid_), location(location_) {}
    MoveShardLeaderReq(const MoveShardLeaderReq&) = default;
    MoveShardLeaderReq& operator=(const MoveShardLeaderReq&) = default;
    MoveShardLeaderReq(MoveShardLeaderReq&&) = default;
    MoveShardLeaderReq& operator=(MoveShardLeaderReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // shrid
        _size += 1; // location
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MoveShardLeaderReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MoveShardLeaderReq& x);

struct MoveShardLeaderResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    MoveShardLeaderResp() { clear(); }
    MoveShardLeaderResp(const MoveShardLeaderResp&) = default;
    MoveShardLeaderResp& operator=(const MoveShardLeaderResp&) = default;
    MoveShardLeaderResp(MoveShardLeaderResp&&) = default;
    MoveShardLeaderResp& operator=(MoveShardLeaderResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MoveShardLeaderResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MoveShardLeaderResp& x);

struct ClearShardInfoReq {
    ShardReplicaId shrid;
    uint8_t location;

    static constexpr uint16_t STATIC_SIZE = 2 + 1; // shrid + location

    ClearShardInfoReq() { clear(); }
    explicit ClearShardInfoReq(ShardReplicaId shrid_, uint8_t location_) : shrid(shrid_), location(location_) {}
    ClearShardInfoReq(const ClearShardInfoReq&) = default;
    ClearShardInfoReq& operator=(const ClearShardInfoReq&) = default;
    ClearShardInfoReq(ClearShardInfoReq&&) = default;
    ClearShardInfoReq& operator=(ClearShardInfoReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // shrid
        _size += 1; // location
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ClearShardInfoReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ClearShardInfoReq& x);

struct ClearShardInfoResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    ClearShardInfoResp() { clear(); }
    ClearShardInfoResp(const ClearShardInfoResp&) = default;
    ClearShardInfoResp& operator=(const ClearShardInfoResp&) = default;
    ClearShardInfoResp(ClearShardInfoResp&&) = default;
    ClearShardInfoResp& operator=(ClearShardInfoResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ClearShardInfoResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ClearShardInfoResp& x);

struct ShardBlockServicesReq {
    ShardId shardId;

    static constexpr uint16_t STATIC_SIZE = 1; // shardId

    ShardBlockServicesReq() { clear(); }
    explicit ShardBlockServicesReq(ShardId shardId_) : shardId(shardId_) {}
    ShardBlockServicesReq(const ShardBlockServicesReq&) = default;
    ShardBlockServicesReq& operator=(const ShardBlockServicesReq&) = default;
    ShardBlockServicesReq(ShardBlockServicesReq&&) = default;
    ShardBlockServicesReq& operator=(ShardBlockServicesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // shardId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ShardBlockServicesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ShardBlockServicesReq& x);

struct ShardBlockServicesResp {
    BincodeList<BlockServiceInfoShort> blockServices;

    static constexpr uint16_t STATIC_SIZE = BincodeList<BlockServiceInfoShort>::STATIC_SIZE; // blockServices

    ShardBlockServicesResp() { clear(); }
    explicit ShardBlockServicesResp(BincodeList<BlockServiceInfoShort>&& blockServices_) : blockServices(std::move(blockServices_)) {}
    ShardBlockServicesResp(const ShardBlockServicesResp&) = default;
    ShardBlockServicesResp& operator=(const ShardBlockServicesResp&) = default;
    ShardBlockServicesResp(ShardBlockServicesResp&&) = default;
    ShardBlockServicesResp& operator=(ShardBlockServicesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += blockServices.packedSize(); // blockServices
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ShardBlockServicesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ShardBlockServicesResp& x);

struct AllCdcReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    AllCdcReq() { clear(); }
    AllCdcReq(const AllCdcReq&) = default;
    AllCdcReq& operator=(const AllCdcReq&) = default;
    AllCdcReq(AllCdcReq&&) = default;
    AllCdcReq& operator=(AllCdcReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AllCdcReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AllCdcReq& x);

struct AllCdcResp {
    BincodeList<CdcInfo> replicas;

    static constexpr uint16_t STATIC_SIZE = BincodeList<CdcInfo>::STATIC_SIZE; // replicas

    AllCdcResp() { clear(); }
    explicit AllCdcResp(BincodeList<CdcInfo>&& replicas_) : replicas(std::move(replicas_)) {}
    AllCdcResp(const AllCdcResp&) = default;
    AllCdcResp& operator=(const AllCdcResp&) = default;
    AllCdcResp(AllCdcResp&&) = default;
    AllCdcResp& operator=(AllCdcResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += replicas.packedSize(); // replicas
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AllCdcResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AllCdcResp& x);

struct EraseDecommissionedBlockReq {
    BlockServiceId blockServiceId;
    uint64_t blockId;
    BincodeFixedBytes<8> certificate;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeFixedBytes<8>::STATIC_SIZE; // blockServiceId + blockId + certificate

    EraseDecommissionedBlockReq() { clear(); }
    explicit EraseDecommissionedBlockReq(BlockServiceId blockServiceId_, uint64_t blockId_, BincodeFixedBytes<8>&& certificate_) : blockServiceId(blockServiceId_), blockId(blockId_), certificate(std::move(certificate_)) {}
    EraseDecommissionedBlockReq(const EraseDecommissionedBlockReq&) = default;
    EraseDecommissionedBlockReq& operator=(const EraseDecommissionedBlockReq&) = default;
    EraseDecommissionedBlockReq(EraseDecommissionedBlockReq&&) = default;
    EraseDecommissionedBlockReq& operator=(EraseDecommissionedBlockReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // blockServiceId
        _size += 8; // blockId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // certificate
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const EraseDecommissionedBlockReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const EraseDecommissionedBlockReq& x);

struct EraseDecommissionedBlockResp {
    BincodeFixedBytes<8> proof;

    static constexpr uint16_t STATIC_SIZE = BincodeFixedBytes<8>::STATIC_SIZE; // proof

    EraseDecommissionedBlockResp() { clear(); }
    explicit EraseDecommissionedBlockResp(BincodeFixedBytes<8>&& proof_) : proof(std::move(proof_)) {}
    EraseDecommissionedBlockResp(const EraseDecommissionedBlockResp&) = default;
    EraseDecommissionedBlockResp& operator=(const EraseDecommissionedBlockResp&) = default;
    EraseDecommissionedBlockResp(EraseDecommissionedBlockResp&&) = default;
    EraseDecommissionedBlockResp& operator=(EraseDecommissionedBlockResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // proof
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const EraseDecommissionedBlockResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const EraseDecommissionedBlockResp& x);

struct AllBlockServicesDeprecatedReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    AllBlockServicesDeprecatedReq() { clear(); }
    AllBlockServicesDeprecatedReq(const AllBlockServicesDeprecatedReq&) = default;
    AllBlockServicesDeprecatedReq& operator=(const AllBlockServicesDeprecatedReq&) = default;
    AllBlockServicesDeprecatedReq(AllBlockServicesDeprecatedReq&&) = default;
    AllBlockServicesDeprecatedReq& operator=(AllBlockServicesDeprecatedReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AllBlockServicesDeprecatedReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AllBlockServicesDeprecatedReq& x);

struct AllBlockServicesDeprecatedResp {
    BincodeList<BlockServiceDeprecatedInfo> blockServices;

    static constexpr uint16_t STATIC_SIZE = BincodeList<BlockServiceDeprecatedInfo>::STATIC_SIZE; // blockServices

    AllBlockServicesDeprecatedResp() { clear(); }
    explicit AllBlockServicesDeprecatedResp(BincodeList<BlockServiceDeprecatedInfo>&& blockServices_) : blockServices(std::move(blockServices_)) {}
    AllBlockServicesDeprecatedResp(const AllBlockServicesDeprecatedResp&) = default;
    AllBlockServicesDeprecatedResp& operator=(const AllBlockServicesDeprecatedResp&) = default;
    AllBlockServicesDeprecatedResp(AllBlockServicesDeprecatedResp&&) = default;
    AllBlockServicesDeprecatedResp& operator=(AllBlockServicesDeprecatedResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += blockServices.packedSize(); // blockServices
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AllBlockServicesDeprecatedResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AllBlockServicesDeprecatedResp& x);

struct AllBlockServicesReq {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    AllBlockServicesReq() { clear(); }
    AllBlockServicesReq(const AllBlockServicesReq&) = default;
    AllBlockServicesReq& operator=(const AllBlockServicesReq&) = default;
    AllBlockServicesReq(AllBlockServicesReq&&) = default;
    AllBlockServicesReq& operator=(AllBlockServicesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AllBlockServicesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AllBlockServicesReq& x);

struct AllBlockServicesResp {
    BincodeList<FullBlockServiceInfo> blockServices;

    static constexpr uint16_t STATIC_SIZE = BincodeList<FullBlockServiceInfo>::STATIC_SIZE; // blockServices

    AllBlockServicesResp() { clear(); }
    explicit AllBlockServicesResp(BincodeList<FullBlockServiceInfo>&& blockServices_) : blockServices(std::move(blockServices_)) {}
    AllBlockServicesResp(const AllBlockServicesResp&) = default;
    AllBlockServicesResp& operator=(const AllBlockServicesResp&) = default;
    AllBlockServicesResp(AllBlockServicesResp&&) = default;
    AllBlockServicesResp& operator=(AllBlockServicesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += blockServices.packedSize(); // blockServices
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AllBlockServicesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AllBlockServicesResp& x);

struct MoveCdcLeaderReq {
    ReplicaId replica;
    uint8_t location;

    static constexpr uint16_t STATIC_SIZE = 1 + 1; // replica + location

    MoveCdcLeaderReq() { clear(); }
    explicit MoveCdcLeaderReq(ReplicaId replica_, uint8_t location_) : replica(replica_), location(location_) {}
    MoveCdcLeaderReq(const MoveCdcLeaderReq&) = default;
    MoveCdcLeaderReq& operator=(const MoveCdcLeaderReq&) = default;
    MoveCdcLeaderReq(MoveCdcLeaderReq&&) = default;
    MoveCdcLeaderReq& operator=(MoveCdcLeaderReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // replica
        _size += 1; // location
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MoveCdcLeaderReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MoveCdcLeaderReq& x);

struct MoveCdcLeaderResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    MoveCdcLeaderResp() { clear(); }
    MoveCdcLeaderResp(const MoveCdcLeaderResp&) = default;
    MoveCdcLeaderResp& operator=(const MoveCdcLeaderResp&) = default;
    MoveCdcLeaderResp(MoveCdcLeaderResp&&) = default;
    MoveCdcLeaderResp& operator=(MoveCdcLeaderResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MoveCdcLeaderResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MoveCdcLeaderResp& x);

struct ClearCdcInfoReq {
    ReplicaId replica;
    uint8_t location;

    static constexpr uint16_t STATIC_SIZE = 1 + 1; // replica + location

    ClearCdcInfoReq() { clear(); }
    explicit ClearCdcInfoReq(ReplicaId replica_, uint8_t location_) : replica(replica_), location(location_) {}
    ClearCdcInfoReq(const ClearCdcInfoReq&) = default;
    ClearCdcInfoReq& operator=(const ClearCdcInfoReq&) = default;
    ClearCdcInfoReq(ClearCdcInfoReq&&) = default;
    ClearCdcInfoReq& operator=(ClearCdcInfoReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // replica
        _size += 1; // location
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ClearCdcInfoReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ClearCdcInfoReq& x);

struct ClearCdcInfoResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    ClearCdcInfoResp() { clear(); }
    ClearCdcInfoResp(const ClearCdcInfoResp&) = default;
    ClearCdcInfoResp& operator=(const ClearCdcInfoResp&) = default;
    ClearCdcInfoResp(ClearCdcInfoResp&&) = default;
    ClearCdcInfoResp& operator=(ClearCdcInfoResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ClearCdcInfoResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ClearCdcInfoResp& x);

struct UpdateBlockServicePathReq {
    BlockServiceId id;
    BincodeBytes newPath;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeBytes::STATIC_SIZE; // id + newPath

    UpdateBlockServicePathReq() { clear(); }
    explicit UpdateBlockServicePathReq(BlockServiceId id_, BincodeBytes&& newPath_) : id(id_), newPath(std::move(newPath_)) {}
    UpdateBlockServicePathReq(const UpdateBlockServicePathReq&) = default;
    UpdateBlockServicePathReq& operator=(const UpdateBlockServicePathReq&) = default;
    UpdateBlockServicePathReq(UpdateBlockServicePathReq&&) = default;
    UpdateBlockServicePathReq& operator=(UpdateBlockServicePathReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += newPath.packedSize(); // newPath
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const UpdateBlockServicePathReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const UpdateBlockServicePathReq& x);

struct UpdateBlockServicePathResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    UpdateBlockServicePathResp() { clear(); }
    UpdateBlockServicePathResp(const UpdateBlockServicePathResp&) = default;
    UpdateBlockServicePathResp& operator=(const UpdateBlockServicePathResp&) = default;
    UpdateBlockServicePathResp(UpdateBlockServicePathResp&&) = default;
    UpdateBlockServicePathResp& operator=(UpdateBlockServicePathResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const UpdateBlockServicePathResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const UpdateBlockServicePathResp& x);

struct SetBlockServiceHasFilesReq {
    BlockServiceId id;
    bool hasFiles;

    static constexpr uint16_t STATIC_SIZE = 8 + 1; // id + hasFiles

    SetBlockServiceHasFilesReq() { clear(); }
    explicit SetBlockServiceHasFilesReq(BlockServiceId id_, bool hasFiles_) : id(id_), hasFiles(hasFiles_) {}
    SetBlockServiceHasFilesReq(const SetBlockServiceHasFilesReq&) = default;
    SetBlockServiceHasFilesReq& operator=(const SetBlockServiceHasFilesReq&) = default;
    SetBlockServiceHasFilesReq(SetBlockServiceHasFilesReq&&) = default;
    SetBlockServiceHasFilesReq& operator=(SetBlockServiceHasFilesReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 1; // hasFiles
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetBlockServiceHasFilesReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetBlockServiceHasFilesReq& x);

struct SetBlockServiceHasFilesResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    SetBlockServiceHasFilesResp() { clear(); }
    SetBlockServiceHasFilesResp(const SetBlockServiceHasFilesResp&) = default;
    SetBlockServiceHasFilesResp& operator=(const SetBlockServiceHasFilesResp&) = default;
    SetBlockServiceHasFilesResp(SetBlockServiceHasFilesResp&&) = default;
    SetBlockServiceHasFilesResp& operator=(SetBlockServiceHasFilesResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetBlockServiceHasFilesResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetBlockServiceHasFilesResp& x);

struct BlockServicesNeedingMigrationReq {
    uint8_t locationId;

    static constexpr uint16_t STATIC_SIZE = 1; // locationId

    BlockServicesNeedingMigrationReq() { clear(); }
    explicit BlockServicesNeedingMigrationReq(uint8_t locationId_) : locationId(locationId_) {}
    BlockServicesNeedingMigrationReq(const BlockServicesNeedingMigrationReq&) = default;
    BlockServicesNeedingMigrationReq& operator=(const BlockServicesNeedingMigrationReq&) = default;
    BlockServicesNeedingMigrationReq(BlockServicesNeedingMigrationReq&&) = default;
    BlockServicesNeedingMigrationReq& operator=(BlockServicesNeedingMigrationReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // locationId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlockServicesNeedingMigrationReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlockServicesNeedingMigrationReq& x);

struct BlockServicesNeedingMigrationResp {
    BincodeList<BlockServiceId> blockServices;

    static constexpr uint16_t STATIC_SIZE = BincodeList<BlockServiceId>::STATIC_SIZE; // blockServices

    BlockServicesNeedingMigrationResp() { clear(); }
    explicit BlockServicesNeedingMigrationResp(BincodeList<BlockServiceId>&& blockServices_) : blockServices(std::move(blockServices_)) {}
    BlockServicesNeedingMigrationResp(const BlockServicesNeedingMigrationResp&) = default;
    BlockServicesNeedingMigrationResp& operator=(const BlockServicesNeedingMigrationResp&) = default;
    BlockServicesNeedingMigrationResp(BlockServicesNeedingMigrationResp&&) = default;
    BlockServicesNeedingMigrationResp& operator=(BlockServicesNeedingMigrationResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += blockServices.packedSize(); // blockServices
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const BlockServicesNeedingMigrationResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const BlockServicesNeedingMigrationResp& x);

struct FetchBlockReq {
    uint64_t blockId;
    uint32_t offset;
    uint32_t count;

    static constexpr uint16_t STATIC_SIZE = 8 + 4 + 4; // blockId + offset + count

    FetchBlockReq() { clear(); }
    explicit FetchBlockReq(uint64_t blockId_, uint32_t offset_, uint32_t count_) : blockId(blockId_), offset(offset_), count(count_) {}
    FetchBlockReq(const FetchBlockReq&) = default;
    FetchBlockReq& operator=(const FetchBlockReq&) = default;
    FetchBlockReq(FetchBlockReq&&) = default;
    FetchBlockReq& operator=(FetchBlockReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // blockId
        _size += 4; // offset
        _size += 4; // count
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchBlockReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchBlockReq& x);

struct FetchBlockResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    FetchBlockResp() { clear(); }
    FetchBlockResp(const FetchBlockResp&) = default;
    FetchBlockResp& operator=(const FetchBlockResp&) = default;
    FetchBlockResp(FetchBlockResp&&) = default;
    FetchBlockResp& operator=(FetchBlockResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchBlockResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchBlockResp& x);

struct WriteBlockReq {
    uint64_t blockId;
    Crc crc;
    uint32_t size;
    BincodeFixedBytes<8> certificate;

    static constexpr uint16_t STATIC_SIZE = 8 + 4 + 4 + BincodeFixedBytes<8>::STATIC_SIZE; // blockId + crc + size + certificate

    WriteBlockReq() { clear(); }
    explicit WriteBlockReq(uint64_t blockId_, Crc crc_, uint32_t size_, BincodeFixedBytes<8>&& certificate_) : blockId(blockId_), crc(crc_), size(size_), certificate(std::move(certificate_)) {}
    WriteBlockReq(const WriteBlockReq&) = default;
    WriteBlockReq& operator=(const WriteBlockReq&) = default;
    WriteBlockReq(WriteBlockReq&&) = default;
    WriteBlockReq& operator=(WriteBlockReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // blockId
        _size += 4; // crc
        _size += 4; // size
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // certificate
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const WriteBlockReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const WriteBlockReq& x);

struct WriteBlockResp {
    BincodeFixedBytes<8> proof;

    static constexpr uint16_t STATIC_SIZE = BincodeFixedBytes<8>::STATIC_SIZE; // proof

    WriteBlockResp() { clear(); }
    explicit WriteBlockResp(BincodeFixedBytes<8>&& proof_) : proof(std::move(proof_)) {}
    WriteBlockResp(const WriteBlockResp&) = default;
    WriteBlockResp& operator=(const WriteBlockResp&) = default;
    WriteBlockResp(WriteBlockResp&&) = default;
    WriteBlockResp& operator=(WriteBlockResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // proof
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const WriteBlockResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const WriteBlockResp& x);

struct FetchBlockWithCrcReq {
    InodeId fileIdUnused;
    uint64_t blockId;
    Crc blockCrcUnused;
    uint32_t offset;
    uint32_t count;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 4 + 4 + 4; // fileIdUnused + blockId + blockCrcUnused + offset + count

    FetchBlockWithCrcReq() { clear(); }
    explicit FetchBlockWithCrcReq(InodeId fileIdUnused_, uint64_t blockId_, Crc blockCrcUnused_, uint32_t offset_, uint32_t count_) : fileIdUnused(fileIdUnused_), blockId(blockId_), blockCrcUnused(blockCrcUnused_), offset(offset_), count(count_) {}
    FetchBlockWithCrcReq(const FetchBlockWithCrcReq&) = default;
    FetchBlockWithCrcReq& operator=(const FetchBlockWithCrcReq&) = default;
    FetchBlockWithCrcReq(FetchBlockWithCrcReq&&) = default;
    FetchBlockWithCrcReq& operator=(FetchBlockWithCrcReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileIdUnused
        _size += 8; // blockId
        _size += 4; // blockCrcUnused
        _size += 4; // offset
        _size += 4; // count
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchBlockWithCrcReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchBlockWithCrcReq& x);

struct FetchBlockWithCrcResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    FetchBlockWithCrcResp() { clear(); }
    FetchBlockWithCrcResp(const FetchBlockWithCrcResp&) = default;
    FetchBlockWithCrcResp& operator=(const FetchBlockWithCrcResp&) = default;
    FetchBlockWithCrcResp(FetchBlockWithCrcResp&&) = default;
    FetchBlockWithCrcResp& operator=(FetchBlockWithCrcResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const FetchBlockWithCrcResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const FetchBlockWithCrcResp& x);

struct EraseBlockReq {
    uint64_t blockId;
    BincodeFixedBytes<8> certificate;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeFixedBytes<8>::STATIC_SIZE; // blockId + certificate

    EraseBlockReq() { clear(); }
    explicit EraseBlockReq(uint64_t blockId_, BincodeFixedBytes<8>&& certificate_) : blockId(blockId_), certificate(std::move(certificate_)) {}
    EraseBlockReq(const EraseBlockReq&) = default;
    EraseBlockReq& operator=(const EraseBlockReq&) = default;
    EraseBlockReq(EraseBlockReq&&) = default;
    EraseBlockReq& operator=(EraseBlockReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // blockId
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // certificate
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const EraseBlockReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const EraseBlockReq& x);

struct EraseBlockResp {
    BincodeFixedBytes<8> proof;

    static constexpr uint16_t STATIC_SIZE = BincodeFixedBytes<8>::STATIC_SIZE; // proof

    EraseBlockResp() { clear(); }
    explicit EraseBlockResp(BincodeFixedBytes<8>&& proof_) : proof(std::move(proof_)) {}
    EraseBlockResp(const EraseBlockResp&) = default;
    EraseBlockResp& operator=(const EraseBlockResp&) = default;
    EraseBlockResp(EraseBlockResp&&) = default;
    EraseBlockResp& operator=(EraseBlockResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // proof
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const EraseBlockResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const EraseBlockResp& x);

struct TestWriteReq {
    uint64_t size;

    static constexpr uint16_t STATIC_SIZE = 8; // size

    TestWriteReq() { clear(); }
    explicit TestWriteReq(uint64_t size_) : size(size_) {}
    TestWriteReq(const TestWriteReq&) = default;
    TestWriteReq& operator=(const TestWriteReq&) = default;
    TestWriteReq(TestWriteReq&&) = default;
    TestWriteReq& operator=(TestWriteReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // size
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const TestWriteReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const TestWriteReq& x);

struct TestWriteResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    TestWriteResp() { clear(); }
    TestWriteResp(const TestWriteResp&) = default;
    TestWriteResp& operator=(const TestWriteResp&) = default;
    TestWriteResp(TestWriteResp&&) = default;
    TestWriteResp& operator=(TestWriteResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const TestWriteResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const TestWriteResp& x);

struct CheckBlockReq {
    uint64_t blockId;
    uint32_t size;
    Crc crc;

    static constexpr uint16_t STATIC_SIZE = 8 + 4 + 4; // blockId + size + crc

    CheckBlockReq() { clear(); }
    explicit CheckBlockReq(uint64_t blockId_, uint32_t size_, Crc crc_) : blockId(blockId_), size(size_), crc(crc_) {}
    CheckBlockReq(const CheckBlockReq&) = default;
    CheckBlockReq& operator=(const CheckBlockReq&) = default;
    CheckBlockReq(CheckBlockReq&&) = default;
    CheckBlockReq& operator=(CheckBlockReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // blockId
        _size += 4; // size
        _size += 4; // crc
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CheckBlockReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CheckBlockReq& x);

struct CheckBlockResp {

    static constexpr uint16_t STATIC_SIZE = 0; // 

    CheckBlockResp() { clear(); }
    CheckBlockResp(const CheckBlockResp&) = default;
    CheckBlockResp& operator=(const CheckBlockResp&) = default;
    CheckBlockResp(CheckBlockResp&&) = default;
    CheckBlockResp& operator=(CheckBlockResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CheckBlockResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CheckBlockResp& x);

struct LogWriteReq {
    LeaderToken token;
    LogIdx lastReleased;
    LogIdx idx;
    BincodeList<uint8_t> value;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 8 + BincodeList<uint8_t>::STATIC_SIZE; // token + lastReleased + idx + value

    LogWriteReq() { clear(); }
    explicit LogWriteReq(LeaderToken token_, LogIdx lastReleased_, LogIdx idx_, BincodeList<uint8_t>&& value_) : token(token_), lastReleased(lastReleased_), idx(idx_), value(std::move(value_)) {}
    LogWriteReq(const LogWriteReq&) = default;
    LogWriteReq& operator=(const LogWriteReq&) = default;
    LogWriteReq(LogWriteReq&&) = default;
    LogWriteReq& operator=(LogWriteReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // token
        _size += 8; // lastReleased
        _size += 8; // idx
        _size += value.packedSize(); // value
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LogWriteReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LogWriteReq& x);

struct LogWriteResp {
    TernError result;

    static constexpr uint16_t STATIC_SIZE = 2; // result

    LogWriteResp() { clear(); }
    explicit LogWriteResp(TernError result_) : result(result_) {}
    LogWriteResp(const LogWriteResp&) = default;
    LogWriteResp& operator=(const LogWriteResp&) = default;
    LogWriteResp(LogWriteResp&&) = default;
    LogWriteResp& operator=(LogWriteResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // result
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LogWriteResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LogWriteResp& x);

struct ReleaseReq {
    LeaderToken token;
    LogIdx lastReleased;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // token + lastReleased

    ReleaseReq() { clear(); }
    explicit ReleaseReq(LeaderToken token_, LogIdx lastReleased_) : token(token_), lastReleased(lastReleased_) {}
    ReleaseReq(const ReleaseReq&) = default;
    ReleaseReq& operator=(const ReleaseReq&) = default;
    ReleaseReq(ReleaseReq&&) = default;
    ReleaseReq& operator=(ReleaseReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // token
        _size += 8; // lastReleased
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ReleaseReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ReleaseReq& x);

struct ReleaseResp {
    TernError result;

    static constexpr uint16_t STATIC_SIZE = 2; // result

    ReleaseResp() { clear(); }
    explicit ReleaseResp(TernError result_) : result(result_) {}
    ReleaseResp(const ReleaseResp&) = default;
    ReleaseResp& operator=(const ReleaseResp&) = default;
    ReleaseResp(ReleaseResp&&) = default;
    ReleaseResp& operator=(ReleaseResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // result
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ReleaseResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ReleaseResp& x);

struct LogReadReq {
    LogIdx idx;

    static constexpr uint16_t STATIC_SIZE = 8; // idx

    LogReadReq() { clear(); }
    explicit LogReadReq(LogIdx idx_) : idx(idx_) {}
    LogReadReq(const LogReadReq&) = default;
    LogReadReq& operator=(const LogReadReq&) = default;
    LogReadReq(LogReadReq&&) = default;
    LogReadReq& operator=(LogReadReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // idx
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LogReadReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LogReadReq& x);

struct LogReadResp {
    TernError result;
    BincodeList<uint8_t> value;

    static constexpr uint16_t STATIC_SIZE = 2 + BincodeList<uint8_t>::STATIC_SIZE; // result + value

    LogReadResp() { clear(); }
    explicit LogReadResp(TernError result_, BincodeList<uint8_t>&& value_) : result(result_), value(std::move(value_)) {}
    LogReadResp(const LogReadResp&) = default;
    LogReadResp& operator=(const LogReadResp&) = default;
    LogReadResp(LogReadResp&&) = default;
    LogReadResp& operator=(LogReadResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // result
        _size += value.packedSize(); // value
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LogReadResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LogReadResp& x);

struct NewLeaderReq {
    LeaderToken nomineeToken;

    static constexpr uint16_t STATIC_SIZE = 8; // nomineeToken

    NewLeaderReq() { clear(); }
    explicit NewLeaderReq(LeaderToken nomineeToken_) : nomineeToken(nomineeToken_) {}
    NewLeaderReq(const NewLeaderReq&) = default;
    NewLeaderReq& operator=(const NewLeaderReq&) = default;
    NewLeaderReq(NewLeaderReq&&) = default;
    NewLeaderReq& operator=(NewLeaderReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // nomineeToken
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const NewLeaderReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const NewLeaderReq& x);

struct NewLeaderResp {
    TernError result;
    LogIdx lastReleased;

    static constexpr uint16_t STATIC_SIZE = 2 + 8; // result + lastReleased

    NewLeaderResp() { clear(); }
    explicit NewLeaderResp(TernError result_, LogIdx lastReleased_) : result(result_), lastReleased(lastReleased_) {}
    NewLeaderResp(const NewLeaderResp&) = default;
    NewLeaderResp& operator=(const NewLeaderResp&) = default;
    NewLeaderResp(NewLeaderResp&&) = default;
    NewLeaderResp& operator=(NewLeaderResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // result
        _size += 8; // lastReleased
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const NewLeaderResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const NewLeaderResp& x);

struct NewLeaderConfirmReq {
    LeaderToken nomineeToken;
    LogIdx releasedIdx;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // nomineeToken + releasedIdx

    NewLeaderConfirmReq() { clear(); }
    explicit NewLeaderConfirmReq(LeaderToken nomineeToken_, LogIdx releasedIdx_) : nomineeToken(nomineeToken_), releasedIdx(releasedIdx_) {}
    NewLeaderConfirmReq(const NewLeaderConfirmReq&) = default;
    NewLeaderConfirmReq& operator=(const NewLeaderConfirmReq&) = default;
    NewLeaderConfirmReq(NewLeaderConfirmReq&&) = default;
    NewLeaderConfirmReq& operator=(NewLeaderConfirmReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // nomineeToken
        _size += 8; // releasedIdx
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const NewLeaderConfirmReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const NewLeaderConfirmReq& x);

struct NewLeaderConfirmResp {
    TernError result;

    static constexpr uint16_t STATIC_SIZE = 2; // result

    NewLeaderConfirmResp() { clear(); }
    explicit NewLeaderConfirmResp(TernError result_) : result(result_) {}
    NewLeaderConfirmResp(const NewLeaderConfirmResp&) = default;
    NewLeaderConfirmResp& operator=(const NewLeaderConfirmResp&) = default;
    NewLeaderConfirmResp(NewLeaderConfirmResp&&) = default;
    NewLeaderConfirmResp& operator=(NewLeaderConfirmResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // result
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const NewLeaderConfirmResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const NewLeaderConfirmResp& x);

struct LogRecoveryReadReq {
    LeaderToken nomineeToken;
    LogIdx idx;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // nomineeToken + idx

    LogRecoveryReadReq() { clear(); }
    explicit LogRecoveryReadReq(LeaderToken nomineeToken_, LogIdx idx_) : nomineeToken(nomineeToken_), idx(idx_) {}
    LogRecoveryReadReq(const LogRecoveryReadReq&) = default;
    LogRecoveryReadReq& operator=(const LogRecoveryReadReq&) = default;
    LogRecoveryReadReq(LogRecoveryReadReq&&) = default;
    LogRecoveryReadReq& operator=(LogRecoveryReadReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // nomineeToken
        _size += 8; // idx
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LogRecoveryReadReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LogRecoveryReadReq& x);

struct LogRecoveryReadResp {
    TernError result;
    BincodeList<uint8_t> value;

    static constexpr uint16_t STATIC_SIZE = 2 + BincodeList<uint8_t>::STATIC_SIZE; // result + value

    LogRecoveryReadResp() { clear(); }
    explicit LogRecoveryReadResp(TernError result_, BincodeList<uint8_t>&& value_) : result(result_), value(std::move(value_)) {}
    LogRecoveryReadResp(const LogRecoveryReadResp&) = default;
    LogRecoveryReadResp& operator=(const LogRecoveryReadResp&) = default;
    LogRecoveryReadResp(LogRecoveryReadResp&&) = default;
    LogRecoveryReadResp& operator=(LogRecoveryReadResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // result
        _size += value.packedSize(); // value
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LogRecoveryReadResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LogRecoveryReadResp& x);

struct LogRecoveryWriteReq {
    LeaderToken nomineeToken;
    LogIdx idx;
    BincodeList<uint8_t> value;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeList<uint8_t>::STATIC_SIZE; // nomineeToken + idx + value

    LogRecoveryWriteReq() { clear(); }
    explicit LogRecoveryWriteReq(LeaderToken nomineeToken_, LogIdx idx_, BincodeList<uint8_t>&& value_) : nomineeToken(nomineeToken_), idx(idx_), value(std::move(value_)) {}
    LogRecoveryWriteReq(const LogRecoveryWriteReq&) = default;
    LogRecoveryWriteReq& operator=(const LogRecoveryWriteReq&) = default;
    LogRecoveryWriteReq(LogRecoveryWriteReq&&) = default;
    LogRecoveryWriteReq& operator=(LogRecoveryWriteReq&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // nomineeToken
        _size += 8; // idx
        _size += value.packedSize(); // value
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LogRecoveryWriteReq&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LogRecoveryWriteReq& x);

struct LogRecoveryWriteResp {
    TernError result;

    static constexpr uint16_t STATIC_SIZE = 2; // result

    LogRecoveryWriteResp() { clear(); }
    explicit LogRecoveryWriteResp(TernError result_) : result(result_) {}
    LogRecoveryWriteResp(const LogRecoveryWriteResp&) = default;
    LogRecoveryWriteResp& operator=(const LogRecoveryWriteResp&) = default;
    LogRecoveryWriteResp(LogRecoveryWriteResp&&) = default;
    LogRecoveryWriteResp& operator=(LogRecoveryWriteResp&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 2; // result
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LogRecoveryWriteResp&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LogRecoveryWriteResp& x);

struct ShardReqContainer {
private:
    static constexpr std::array<size_t,46> _staticSizes = {LookupReq::STATIC_SIZE, StatFileReq::STATIC_SIZE, StatDirectoryReq::STATIC_SIZE, ReadDirReq::STATIC_SIZE, ConstructFileReq::STATIC_SIZE, AddSpanInitiateReq::STATIC_SIZE, AddSpanCertifyReq::STATIC_SIZE, LinkFileReq::STATIC_SIZE, SoftUnlinkFileReq::STATIC_SIZE, LocalFileSpansReq::STATIC_SIZE, SameDirectoryRenameReq::STATIC_SIZE, AddInlineSpanReq::STATIC_SIZE, SetTimeReq::STATIC_SIZE, FullReadDirReq::STATIC_SIZE, MoveSpanReq::STATIC_SIZE, RemoveNonOwnedEdgeReq::STATIC_SIZE, SameShardHardFileUnlinkReq::STATIC_SIZE, StatTransientFileReq::STATIC_SIZE, ShardSnapshotReq::STATIC_SIZE, FileSpansReq::STATIC_SIZE, AddSpanLocationReq::STATIC_SIZE, ScrapTransientFileReq::STATIC_SIZE, SetDirectoryInfoReq::STATIC_SIZE, GetLinkEntriesReq::STATIC_SIZE, WaitStateAppliedReq::STATIC_SIZE, VisitDirectoriesReq::STATIC_SIZE, VisitFilesReq::STATIC_SIZE, VisitTransientFilesReq::STATIC_SIZE, RemoveSpanInitiateReq::STATIC_SIZE, RemoveSpanCertifyReq::STATIC_SIZE, SwapBlocksReq::STATIC_SIZE, BlockServiceFilesReq::STATIC_SIZE, RemoveInodeReq::STATIC_SIZE, AddSpanInitiateWithReferenceReq::STATIC_SIZE, RemoveZeroBlockServiceFilesReq::STATIC_SIZE, SwapSpansReq::STATIC_SIZE, SameDirectoryRenameSnapshotReq::STATIC_SIZE, AddSpanAtLocationInitiateReq::STATIC_SIZE, CreateDirectoryInodeReq::STATIC_SIZE, SetDirectoryOwnerReq::STATIC_SIZE, RemoveDirectoryOwnerReq::STATIC_SIZE, CreateLockedCurrentEdgeReq::STATIC_SIZE, LockCurrentEdgeReq::STATIC_SIZE, UnlockCurrentEdgeReq::STATIC_SIZE, RemoveOwnedSnapshotFileEdgeReq::STATIC_SIZE, MakeFileTransientReq::STATIC_SIZE};
    ShardMessageKind _kind = ShardMessageKind::EMPTY;
    std::variant<LookupReq, StatFileReq, StatDirectoryReq, ReadDirReq, ConstructFileReq, AddSpanInitiateReq, AddSpanCertifyReq, LinkFileReq, SoftUnlinkFileReq, LocalFileSpansReq, SameDirectoryRenameReq, AddInlineSpanReq, SetTimeReq, FullReadDirReq, MoveSpanReq, RemoveNonOwnedEdgeReq, SameShardHardFileUnlinkReq, StatTransientFileReq, ShardSnapshotReq, FileSpansReq, AddSpanLocationReq, ScrapTransientFileReq, SetDirectoryInfoReq, GetLinkEntriesReq, WaitStateAppliedReq, VisitDirectoriesReq, VisitFilesReq, VisitTransientFilesReq, RemoveSpanInitiateReq, RemoveSpanCertifyReq, SwapBlocksReq, BlockServiceFilesReq, RemoveInodeReq, AddSpanInitiateWithReferenceReq, RemoveZeroBlockServiceFilesReq, SwapSpansReq, SameDirectoryRenameSnapshotReq, AddSpanAtLocationInitiateReq, CreateDirectoryInodeReq, SetDirectoryOwnerReq, RemoveDirectoryOwnerReq, CreateLockedCurrentEdgeReq, LockCurrentEdgeReq, UnlockCurrentEdgeReq, RemoveOwnedSnapshotFileEdgeReq, MakeFileTransientReq> _data;
public:
    ShardReqContainer();
    ShardReqContainer(const ShardReqContainer& other);
    ShardReqContainer(ShardReqContainer&& other);
    void operator=(const ShardReqContainer& other);
    void operator=(ShardReqContainer&& other);

    ShardMessageKind kind() const { return _kind; }

    const LookupReq& getLookup() const;
    LookupReq& setLookup();
    const StatFileReq& getStatFile() const;
    StatFileReq& setStatFile();
    const StatDirectoryReq& getStatDirectory() const;
    StatDirectoryReq& setStatDirectory();
    const ReadDirReq& getReadDir() const;
    ReadDirReq& setReadDir();
    const ConstructFileReq& getConstructFile() const;
    ConstructFileReq& setConstructFile();
    const AddSpanInitiateReq& getAddSpanInitiate() const;
    AddSpanInitiateReq& setAddSpanInitiate();
    const AddSpanCertifyReq& getAddSpanCertify() const;
    AddSpanCertifyReq& setAddSpanCertify();
    const LinkFileReq& getLinkFile() const;
    LinkFileReq& setLinkFile();
    const SoftUnlinkFileReq& getSoftUnlinkFile() const;
    SoftUnlinkFileReq& setSoftUnlinkFile();
    const LocalFileSpansReq& getLocalFileSpans() const;
    LocalFileSpansReq& setLocalFileSpans();
    const SameDirectoryRenameReq& getSameDirectoryRename() const;
    SameDirectoryRenameReq& setSameDirectoryRename();
    const AddInlineSpanReq& getAddInlineSpan() const;
    AddInlineSpanReq& setAddInlineSpan();
    const SetTimeReq& getSetTime() const;
    SetTimeReq& setSetTime();
    const FullReadDirReq& getFullReadDir() const;
    FullReadDirReq& setFullReadDir();
    const MoveSpanReq& getMoveSpan() const;
    MoveSpanReq& setMoveSpan();
    const RemoveNonOwnedEdgeReq& getRemoveNonOwnedEdge() const;
    RemoveNonOwnedEdgeReq& setRemoveNonOwnedEdge();
    const SameShardHardFileUnlinkReq& getSameShardHardFileUnlink() const;
    SameShardHardFileUnlinkReq& setSameShardHardFileUnlink();
    const StatTransientFileReq& getStatTransientFile() const;
    StatTransientFileReq& setStatTransientFile();
    const ShardSnapshotReq& getShardSnapshot() const;
    ShardSnapshotReq& setShardSnapshot();
    const FileSpansReq& getFileSpans() const;
    FileSpansReq& setFileSpans();
    const AddSpanLocationReq& getAddSpanLocation() const;
    AddSpanLocationReq& setAddSpanLocation();
    const ScrapTransientFileReq& getScrapTransientFile() const;
    ScrapTransientFileReq& setScrapTransientFile();
    const SetDirectoryInfoReq& getSetDirectoryInfo() const;
    SetDirectoryInfoReq& setSetDirectoryInfo();
    const GetLinkEntriesReq& getGetLinkEntries() const;
    GetLinkEntriesReq& setGetLinkEntries();
    const WaitStateAppliedReq& getWaitStateApplied() const;
    WaitStateAppliedReq& setWaitStateApplied();
    const VisitDirectoriesReq& getVisitDirectories() const;
    VisitDirectoriesReq& setVisitDirectories();
    const VisitFilesReq& getVisitFiles() const;
    VisitFilesReq& setVisitFiles();
    const VisitTransientFilesReq& getVisitTransientFiles() const;
    VisitTransientFilesReq& setVisitTransientFiles();
    const RemoveSpanInitiateReq& getRemoveSpanInitiate() const;
    RemoveSpanInitiateReq& setRemoveSpanInitiate();
    const RemoveSpanCertifyReq& getRemoveSpanCertify() const;
    RemoveSpanCertifyReq& setRemoveSpanCertify();
    const SwapBlocksReq& getSwapBlocks() const;
    SwapBlocksReq& setSwapBlocks();
    const BlockServiceFilesReq& getBlockServiceFiles() const;
    BlockServiceFilesReq& setBlockServiceFiles();
    const RemoveInodeReq& getRemoveInode() const;
    RemoveInodeReq& setRemoveInode();
    const AddSpanInitiateWithReferenceReq& getAddSpanInitiateWithReference() const;
    AddSpanInitiateWithReferenceReq& setAddSpanInitiateWithReference();
    const RemoveZeroBlockServiceFilesReq& getRemoveZeroBlockServiceFiles() const;
    RemoveZeroBlockServiceFilesReq& setRemoveZeroBlockServiceFiles();
    const SwapSpansReq& getSwapSpans() const;
    SwapSpansReq& setSwapSpans();
    const SameDirectoryRenameSnapshotReq& getSameDirectoryRenameSnapshot() const;
    SameDirectoryRenameSnapshotReq& setSameDirectoryRenameSnapshot();
    const AddSpanAtLocationInitiateReq& getAddSpanAtLocationInitiate() const;
    AddSpanAtLocationInitiateReq& setAddSpanAtLocationInitiate();
    const CreateDirectoryInodeReq& getCreateDirectoryInode() const;
    CreateDirectoryInodeReq& setCreateDirectoryInode();
    const SetDirectoryOwnerReq& getSetDirectoryOwner() const;
    SetDirectoryOwnerReq& setSetDirectoryOwner();
    const RemoveDirectoryOwnerReq& getRemoveDirectoryOwner() const;
    RemoveDirectoryOwnerReq& setRemoveDirectoryOwner();
    const CreateLockedCurrentEdgeReq& getCreateLockedCurrentEdge() const;
    CreateLockedCurrentEdgeReq& setCreateLockedCurrentEdge();
    const LockCurrentEdgeReq& getLockCurrentEdge() const;
    LockCurrentEdgeReq& setLockCurrentEdge();
    const UnlockCurrentEdgeReq& getUnlockCurrentEdge() const;
    UnlockCurrentEdgeReq& setUnlockCurrentEdge();
    const RemoveOwnedSnapshotFileEdgeReq& getRemoveOwnedSnapshotFileEdge() const;
    RemoveOwnedSnapshotFileEdgeReq& setRemoveOwnedSnapshotFileEdge();
    const MakeFileTransientReq& getMakeFileTransient() const;
    MakeFileTransientReq& setMakeFileTransient();

    void clear() { _kind = ShardMessageKind::EMPTY; };

    static constexpr size_t STATIC_SIZE = sizeof(ShardMessageKind) + *std::max_element(_staticSizes.begin(), _staticSizes.end());
    size_t packedSize() const;
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    bool operator==(const ShardReqContainer& other) const;
};

std::ostream& operator<<(std::ostream& out, const ShardReqContainer& x);

struct ShardRespContainer {
private:
    static constexpr std::array<size_t,47> _staticSizes = {sizeof(TernError), LookupResp::STATIC_SIZE, StatFileResp::STATIC_SIZE, StatDirectoryResp::STATIC_SIZE, ReadDirResp::STATIC_SIZE, ConstructFileResp::STATIC_SIZE, AddSpanInitiateResp::STATIC_SIZE, AddSpanCertifyResp::STATIC_SIZE, LinkFileResp::STATIC_SIZE, SoftUnlinkFileResp::STATIC_SIZE, LocalFileSpansResp::STATIC_SIZE, SameDirectoryRenameResp::STATIC_SIZE, AddInlineSpanResp::STATIC_SIZE, SetTimeResp::STATIC_SIZE, FullReadDirResp::STATIC_SIZE, MoveSpanResp::STATIC_SIZE, RemoveNonOwnedEdgeResp::STATIC_SIZE, SameShardHardFileUnlinkResp::STATIC_SIZE, StatTransientFileResp::STATIC_SIZE, ShardSnapshotResp::STATIC_SIZE, FileSpansResp::STATIC_SIZE, AddSpanLocationResp::STATIC_SIZE, ScrapTransientFileResp::STATIC_SIZE, SetDirectoryInfoResp::STATIC_SIZE, GetLinkEntriesResp::STATIC_SIZE, WaitStateAppliedResp::STATIC_SIZE, VisitDirectoriesResp::STATIC_SIZE, VisitFilesResp::STATIC_SIZE, VisitTransientFilesResp::STATIC_SIZE, RemoveSpanInitiateResp::STATIC_SIZE, RemoveSpanCertifyResp::STATIC_SIZE, SwapBlocksResp::STATIC_SIZE, BlockServiceFilesResp::STATIC_SIZE, RemoveInodeResp::STATIC_SIZE, AddSpanInitiateWithReferenceResp::STATIC_SIZE, RemoveZeroBlockServiceFilesResp::STATIC_SIZE, SwapSpansResp::STATIC_SIZE, SameDirectoryRenameSnapshotResp::STATIC_SIZE, AddSpanAtLocationInitiateResp::STATIC_SIZE, CreateDirectoryInodeResp::STATIC_SIZE, SetDirectoryOwnerResp::STATIC_SIZE, RemoveDirectoryOwnerResp::STATIC_SIZE, CreateLockedCurrentEdgeResp::STATIC_SIZE, LockCurrentEdgeResp::STATIC_SIZE, UnlockCurrentEdgeResp::STATIC_SIZE, RemoveOwnedSnapshotFileEdgeResp::STATIC_SIZE, MakeFileTransientResp::STATIC_SIZE};
    ShardMessageKind _kind = ShardMessageKind::EMPTY;
    std::variant<TernError, LookupResp, StatFileResp, StatDirectoryResp, ReadDirResp, ConstructFileResp, AddSpanInitiateResp, AddSpanCertifyResp, LinkFileResp, SoftUnlinkFileResp, LocalFileSpansResp, SameDirectoryRenameResp, AddInlineSpanResp, SetTimeResp, FullReadDirResp, MoveSpanResp, RemoveNonOwnedEdgeResp, SameShardHardFileUnlinkResp, StatTransientFileResp, ShardSnapshotResp, FileSpansResp, AddSpanLocationResp, ScrapTransientFileResp, SetDirectoryInfoResp, GetLinkEntriesResp, WaitStateAppliedResp, VisitDirectoriesResp, VisitFilesResp, VisitTransientFilesResp, RemoveSpanInitiateResp, RemoveSpanCertifyResp, SwapBlocksResp, BlockServiceFilesResp, RemoveInodeResp, AddSpanInitiateWithReferenceResp, RemoveZeroBlockServiceFilesResp, SwapSpansResp, SameDirectoryRenameSnapshotResp, AddSpanAtLocationInitiateResp, CreateDirectoryInodeResp, SetDirectoryOwnerResp, RemoveDirectoryOwnerResp, CreateLockedCurrentEdgeResp, LockCurrentEdgeResp, UnlockCurrentEdgeResp, RemoveOwnedSnapshotFileEdgeResp, MakeFileTransientResp> _data;
public:
    ShardRespContainer();
    ShardRespContainer(const ShardRespContainer& other);
    ShardRespContainer(ShardRespContainer&& other);
    void operator=(const ShardRespContainer& other);
    void operator=(ShardRespContainer&& other);

    ShardMessageKind kind() const { return _kind; }

    const TernError& getError() const;
    TernError& setError();
    const LookupResp& getLookup() const;
    LookupResp& setLookup();
    const StatFileResp& getStatFile() const;
    StatFileResp& setStatFile();
    const StatDirectoryResp& getStatDirectory() const;
    StatDirectoryResp& setStatDirectory();
    const ReadDirResp& getReadDir() const;
    ReadDirResp& setReadDir();
    const ConstructFileResp& getConstructFile() const;
    ConstructFileResp& setConstructFile();
    const AddSpanInitiateResp& getAddSpanInitiate() const;
    AddSpanInitiateResp& setAddSpanInitiate();
    const AddSpanCertifyResp& getAddSpanCertify() const;
    AddSpanCertifyResp& setAddSpanCertify();
    const LinkFileResp& getLinkFile() const;
    LinkFileResp& setLinkFile();
    const SoftUnlinkFileResp& getSoftUnlinkFile() const;
    SoftUnlinkFileResp& setSoftUnlinkFile();
    const LocalFileSpansResp& getLocalFileSpans() const;
    LocalFileSpansResp& setLocalFileSpans();
    const SameDirectoryRenameResp& getSameDirectoryRename() const;
    SameDirectoryRenameResp& setSameDirectoryRename();
    const AddInlineSpanResp& getAddInlineSpan() const;
    AddInlineSpanResp& setAddInlineSpan();
    const SetTimeResp& getSetTime() const;
    SetTimeResp& setSetTime();
    const FullReadDirResp& getFullReadDir() const;
    FullReadDirResp& setFullReadDir();
    const MoveSpanResp& getMoveSpan() const;
    MoveSpanResp& setMoveSpan();
    const RemoveNonOwnedEdgeResp& getRemoveNonOwnedEdge() const;
    RemoveNonOwnedEdgeResp& setRemoveNonOwnedEdge();
    const SameShardHardFileUnlinkResp& getSameShardHardFileUnlink() const;
    SameShardHardFileUnlinkResp& setSameShardHardFileUnlink();
    const StatTransientFileResp& getStatTransientFile() const;
    StatTransientFileResp& setStatTransientFile();
    const ShardSnapshotResp& getShardSnapshot() const;
    ShardSnapshotResp& setShardSnapshot();
    const FileSpansResp& getFileSpans() const;
    FileSpansResp& setFileSpans();
    const AddSpanLocationResp& getAddSpanLocation() const;
    AddSpanLocationResp& setAddSpanLocation();
    const ScrapTransientFileResp& getScrapTransientFile() const;
    ScrapTransientFileResp& setScrapTransientFile();
    const SetDirectoryInfoResp& getSetDirectoryInfo() const;
    SetDirectoryInfoResp& setSetDirectoryInfo();
    const GetLinkEntriesResp& getGetLinkEntries() const;
    GetLinkEntriesResp& setGetLinkEntries();
    const WaitStateAppliedResp& getWaitStateApplied() const;
    WaitStateAppliedResp& setWaitStateApplied();
    const VisitDirectoriesResp& getVisitDirectories() const;
    VisitDirectoriesResp& setVisitDirectories();
    const VisitFilesResp& getVisitFiles() const;
    VisitFilesResp& setVisitFiles();
    const VisitTransientFilesResp& getVisitTransientFiles() const;
    VisitTransientFilesResp& setVisitTransientFiles();
    const RemoveSpanInitiateResp& getRemoveSpanInitiate() const;
    RemoveSpanInitiateResp& setRemoveSpanInitiate();
    const RemoveSpanCertifyResp& getRemoveSpanCertify() const;
    RemoveSpanCertifyResp& setRemoveSpanCertify();
    const SwapBlocksResp& getSwapBlocks() const;
    SwapBlocksResp& setSwapBlocks();
    const BlockServiceFilesResp& getBlockServiceFiles() const;
    BlockServiceFilesResp& setBlockServiceFiles();
    const RemoveInodeResp& getRemoveInode() const;
    RemoveInodeResp& setRemoveInode();
    const AddSpanInitiateWithReferenceResp& getAddSpanInitiateWithReference() const;
    AddSpanInitiateWithReferenceResp& setAddSpanInitiateWithReference();
    const RemoveZeroBlockServiceFilesResp& getRemoveZeroBlockServiceFiles() const;
    RemoveZeroBlockServiceFilesResp& setRemoveZeroBlockServiceFiles();
    const SwapSpansResp& getSwapSpans() const;
    SwapSpansResp& setSwapSpans();
    const SameDirectoryRenameSnapshotResp& getSameDirectoryRenameSnapshot() const;
    SameDirectoryRenameSnapshotResp& setSameDirectoryRenameSnapshot();
    const AddSpanAtLocationInitiateResp& getAddSpanAtLocationInitiate() const;
    AddSpanAtLocationInitiateResp& setAddSpanAtLocationInitiate();
    const CreateDirectoryInodeResp& getCreateDirectoryInode() const;
    CreateDirectoryInodeResp& setCreateDirectoryInode();
    const SetDirectoryOwnerResp& getSetDirectoryOwner() const;
    SetDirectoryOwnerResp& setSetDirectoryOwner();
    const RemoveDirectoryOwnerResp& getRemoveDirectoryOwner() const;
    RemoveDirectoryOwnerResp& setRemoveDirectoryOwner();
    const CreateLockedCurrentEdgeResp& getCreateLockedCurrentEdge() const;
    CreateLockedCurrentEdgeResp& setCreateLockedCurrentEdge();
    const LockCurrentEdgeResp& getLockCurrentEdge() const;
    LockCurrentEdgeResp& setLockCurrentEdge();
    const UnlockCurrentEdgeResp& getUnlockCurrentEdge() const;
    UnlockCurrentEdgeResp& setUnlockCurrentEdge();
    const RemoveOwnedSnapshotFileEdgeResp& getRemoveOwnedSnapshotFileEdge() const;
    RemoveOwnedSnapshotFileEdgeResp& setRemoveOwnedSnapshotFileEdge();
    const MakeFileTransientResp& getMakeFileTransient() const;
    MakeFileTransientResp& setMakeFileTransient();

    void clear() { _kind = ShardMessageKind::EMPTY; };

    static constexpr size_t STATIC_SIZE = sizeof(ShardMessageKind) + *std::max_element(_staticSizes.begin(), _staticSizes.end());
    size_t packedSize() const;
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    bool operator==(const ShardRespContainer& other) const;
};

std::ostream& operator<<(std::ostream& out, const ShardRespContainer& x);

struct CDCReqContainer {
private:
    static constexpr std::array<size_t,7> _staticSizes = {MakeDirectoryReq::STATIC_SIZE, RenameFileReq::STATIC_SIZE, SoftUnlinkDirectoryReq::STATIC_SIZE, RenameDirectoryReq::STATIC_SIZE, CrossShardHardUnlinkFileReq::STATIC_SIZE, HardUnlinkDirectoryReq::STATIC_SIZE, CdcSnapshotReq::STATIC_SIZE};
    CDCMessageKind _kind = CDCMessageKind::EMPTY;
    std::variant<MakeDirectoryReq, RenameFileReq, SoftUnlinkDirectoryReq, RenameDirectoryReq, CrossShardHardUnlinkFileReq, HardUnlinkDirectoryReq, CdcSnapshotReq> _data;
public:
    CDCReqContainer();
    CDCReqContainer(const CDCReqContainer& other);
    CDCReqContainer(CDCReqContainer&& other);
    void operator=(const CDCReqContainer& other);
    void operator=(CDCReqContainer&& other);

    CDCMessageKind kind() const { return _kind; }

    const MakeDirectoryReq& getMakeDirectory() const;
    MakeDirectoryReq& setMakeDirectory();
    const RenameFileReq& getRenameFile() const;
    RenameFileReq& setRenameFile();
    const SoftUnlinkDirectoryReq& getSoftUnlinkDirectory() const;
    SoftUnlinkDirectoryReq& setSoftUnlinkDirectory();
    const RenameDirectoryReq& getRenameDirectory() const;
    RenameDirectoryReq& setRenameDirectory();
    const CrossShardHardUnlinkFileReq& getCrossShardHardUnlinkFile() const;
    CrossShardHardUnlinkFileReq& setCrossShardHardUnlinkFile();
    const HardUnlinkDirectoryReq& getHardUnlinkDirectory() const;
    HardUnlinkDirectoryReq& setHardUnlinkDirectory();
    const CdcSnapshotReq& getCdcSnapshot() const;
    CdcSnapshotReq& setCdcSnapshot();

    void clear() { _kind = CDCMessageKind::EMPTY; };

    static constexpr size_t STATIC_SIZE = sizeof(CDCMessageKind) + *std::max_element(_staticSizes.begin(), _staticSizes.end());
    size_t packedSize() const;
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    bool operator==(const CDCReqContainer& other) const;
};

std::ostream& operator<<(std::ostream& out, const CDCReqContainer& x);

struct CDCRespContainer {
private:
    static constexpr std::array<size_t,8> _staticSizes = {sizeof(TernError), MakeDirectoryResp::STATIC_SIZE, RenameFileResp::STATIC_SIZE, SoftUnlinkDirectoryResp::STATIC_SIZE, RenameDirectoryResp::STATIC_SIZE, CrossShardHardUnlinkFileResp::STATIC_SIZE, HardUnlinkDirectoryResp::STATIC_SIZE, CdcSnapshotResp::STATIC_SIZE};
    CDCMessageKind _kind = CDCMessageKind::EMPTY;
    std::variant<TernError, MakeDirectoryResp, RenameFileResp, SoftUnlinkDirectoryResp, RenameDirectoryResp, CrossShardHardUnlinkFileResp, HardUnlinkDirectoryResp, CdcSnapshotResp> _data;
public:
    CDCRespContainer();
    CDCRespContainer(const CDCRespContainer& other);
    CDCRespContainer(CDCRespContainer&& other);
    void operator=(const CDCRespContainer& other);
    void operator=(CDCRespContainer&& other);

    CDCMessageKind kind() const { return _kind; }

    const TernError& getError() const;
    TernError& setError();
    const MakeDirectoryResp& getMakeDirectory() const;
    MakeDirectoryResp& setMakeDirectory();
    const RenameFileResp& getRenameFile() const;
    RenameFileResp& setRenameFile();
    const SoftUnlinkDirectoryResp& getSoftUnlinkDirectory() const;
    SoftUnlinkDirectoryResp& setSoftUnlinkDirectory();
    const RenameDirectoryResp& getRenameDirectory() const;
    RenameDirectoryResp& setRenameDirectory();
    const CrossShardHardUnlinkFileResp& getCrossShardHardUnlinkFile() const;
    CrossShardHardUnlinkFileResp& setCrossShardHardUnlinkFile();
    const HardUnlinkDirectoryResp& getHardUnlinkDirectory() const;
    HardUnlinkDirectoryResp& setHardUnlinkDirectory();
    const CdcSnapshotResp& getCdcSnapshot() const;
    CdcSnapshotResp& setCdcSnapshot();

    void clear() { _kind = CDCMessageKind::EMPTY; };

    static constexpr size_t STATIC_SIZE = sizeof(CDCMessageKind) + *std::max_element(_staticSizes.begin(), _staticSizes.end());
    size_t packedSize() const;
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    bool operator==(const CDCRespContainer& other) const;
};

std::ostream& operator<<(std::ostream& out, const CDCRespContainer& x);

struct RegistryReqContainer {
private:
    static constexpr std::array<size_t,33> _staticSizes = {LocalShardsReq::STATIC_SIZE, LocalCdcReq::STATIC_SIZE, InfoReq::STATIC_SIZE, RegistryReq::STATIC_SIZE, LocalChangedBlockServicesReq::STATIC_SIZE, CreateLocationReq::STATIC_SIZE, RenameLocationReq::STATIC_SIZE, RegisterShardReq::STATIC_SIZE, LocationsReq::STATIC_SIZE, RegisterCdcReq::STATIC_SIZE, SetBlockServiceFlagsReq::STATIC_SIZE, RegisterBlockServicesReq::STATIC_SIZE, ChangedBlockServicesAtLocationReq::STATIC_SIZE, ShardsAtLocationReq::STATIC_SIZE, CdcAtLocationReq::STATIC_SIZE, RegisterRegistryReq::STATIC_SIZE, AllRegistryReplicasReq::STATIC_SIZE, ShardBlockServicesDEPRECATEDReq::STATIC_SIZE, CdcReplicasDEPRECATEDReq::STATIC_SIZE, AllShardsReq::STATIC_SIZE, DecommissionBlockServiceReq::STATIC_SIZE, MoveShardLeaderReq::STATIC_SIZE, ClearShardInfoReq::STATIC_SIZE, ShardBlockServicesReq::STATIC_SIZE, AllCdcReq::STATIC_SIZE, EraseDecommissionedBlockReq::STATIC_SIZE, AllBlockServicesDeprecatedReq::STATIC_SIZE, AllBlockServicesReq::STATIC_SIZE, MoveCdcLeaderReq::STATIC_SIZE, ClearCdcInfoReq::STATIC_SIZE, UpdateBlockServicePathReq::STATIC_SIZE, SetBlockServiceHasFilesReq::STATIC_SIZE, BlockServicesNeedingMigrationReq::STATIC_SIZE};
    RegistryMessageKind _kind = RegistryMessageKind::EMPTY;
    std::variant<LocalShardsReq, LocalCdcReq, InfoReq, RegistryReq, LocalChangedBlockServicesReq, CreateLocationReq, RenameLocationReq, RegisterShardReq, LocationsReq, RegisterCdcReq, SetBlockServiceFlagsReq, RegisterBlockServicesReq, ChangedBlockServicesAtLocationReq, ShardsAtLocationReq, CdcAtLocationReq, RegisterRegistryReq, AllRegistryReplicasReq, ShardBlockServicesDEPRECATEDReq, CdcReplicasDEPRECATEDReq, AllShardsReq, DecommissionBlockServiceReq, MoveShardLeaderReq, ClearShardInfoReq, ShardBlockServicesReq, AllCdcReq, EraseDecommissionedBlockReq, AllBlockServicesDeprecatedReq, AllBlockServicesReq, MoveCdcLeaderReq, ClearCdcInfoReq, UpdateBlockServicePathReq, SetBlockServiceHasFilesReq, BlockServicesNeedingMigrationReq> _data;
public:
    RegistryReqContainer();
    RegistryReqContainer(const RegistryReqContainer& other);
    RegistryReqContainer(RegistryReqContainer&& other);
    void operator=(const RegistryReqContainer& other);
    void operator=(RegistryReqContainer&& other);

    RegistryMessageKind kind() const { return _kind; }

    const LocalShardsReq& getLocalShards() const;
    LocalShardsReq& setLocalShards();
    const LocalCdcReq& getLocalCdc() const;
    LocalCdcReq& setLocalCdc();
    const InfoReq& getInfo() const;
    InfoReq& setInfo();
    const RegistryReq& getRegistry() const;
    RegistryReq& setRegistry();
    const LocalChangedBlockServicesReq& getLocalChangedBlockServices() const;
    LocalChangedBlockServicesReq& setLocalChangedBlockServices();
    const CreateLocationReq& getCreateLocation() const;
    CreateLocationReq& setCreateLocation();
    const RenameLocationReq& getRenameLocation() const;
    RenameLocationReq& setRenameLocation();
    const RegisterShardReq& getRegisterShard() const;
    RegisterShardReq& setRegisterShard();
    const LocationsReq& getLocations() const;
    LocationsReq& setLocations();
    const RegisterCdcReq& getRegisterCdc() const;
    RegisterCdcReq& setRegisterCdc();
    const SetBlockServiceFlagsReq& getSetBlockServiceFlags() const;
    SetBlockServiceFlagsReq& setSetBlockServiceFlags();
    const RegisterBlockServicesReq& getRegisterBlockServices() const;
    RegisterBlockServicesReq& setRegisterBlockServices();
    const ChangedBlockServicesAtLocationReq& getChangedBlockServicesAtLocation() const;
    ChangedBlockServicesAtLocationReq& setChangedBlockServicesAtLocation();
    const ShardsAtLocationReq& getShardsAtLocation() const;
    ShardsAtLocationReq& setShardsAtLocation();
    const CdcAtLocationReq& getCdcAtLocation() const;
    CdcAtLocationReq& setCdcAtLocation();
    const RegisterRegistryReq& getRegisterRegistry() const;
    RegisterRegistryReq& setRegisterRegistry();
    const AllRegistryReplicasReq& getAllRegistryReplicas() const;
    AllRegistryReplicasReq& setAllRegistryReplicas();
    const ShardBlockServicesDEPRECATEDReq& getShardBlockServicesDEPRECATED() const;
    ShardBlockServicesDEPRECATEDReq& setShardBlockServicesDEPRECATED();
    const CdcReplicasDEPRECATEDReq& getCdcReplicasDEPRECATED() const;
    CdcReplicasDEPRECATEDReq& setCdcReplicasDEPRECATED();
    const AllShardsReq& getAllShards() const;
    AllShardsReq& setAllShards();
    const DecommissionBlockServiceReq& getDecommissionBlockService() const;
    DecommissionBlockServiceReq& setDecommissionBlockService();
    const MoveShardLeaderReq& getMoveShardLeader() const;
    MoveShardLeaderReq& setMoveShardLeader();
    const ClearShardInfoReq& getClearShardInfo() const;
    ClearShardInfoReq& setClearShardInfo();
    const ShardBlockServicesReq& getShardBlockServices() const;
    ShardBlockServicesReq& setShardBlockServices();
    const AllCdcReq& getAllCdc() const;
    AllCdcReq& setAllCdc();
    const EraseDecommissionedBlockReq& getEraseDecommissionedBlock() const;
    EraseDecommissionedBlockReq& setEraseDecommissionedBlock();
    const AllBlockServicesDeprecatedReq& getAllBlockServicesDeprecated() const;
    AllBlockServicesDeprecatedReq& setAllBlockServicesDeprecated();
    const AllBlockServicesReq& getAllBlockServices() const;
    AllBlockServicesReq& setAllBlockServices();
    const MoveCdcLeaderReq& getMoveCdcLeader() const;
    MoveCdcLeaderReq& setMoveCdcLeader();
    const ClearCdcInfoReq& getClearCdcInfo() const;
    ClearCdcInfoReq& setClearCdcInfo();
    const UpdateBlockServicePathReq& getUpdateBlockServicePath() const;
    UpdateBlockServicePathReq& setUpdateBlockServicePath();
    const SetBlockServiceHasFilesReq& getSetBlockServiceHasFiles() const;
    SetBlockServiceHasFilesReq& setSetBlockServiceHasFiles();
    const BlockServicesNeedingMigrationReq& getBlockServicesNeedingMigration() const;
    BlockServicesNeedingMigrationReq& setBlockServicesNeedingMigration();

    void clear() { _kind = RegistryMessageKind::EMPTY; };

    static constexpr size_t STATIC_SIZE = sizeof(RegistryMessageKind) + *std::max_element(_staticSizes.begin(), _staticSizes.end());
    size_t packedSize() const;
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    bool operator==(const RegistryReqContainer& other) const;
};

std::ostream& operator<<(std::ostream& out, const RegistryReqContainer& x);

struct RegistryRespContainer {
private:
    static constexpr std::array<size_t,34> _staticSizes = {sizeof(TernError), LocalShardsResp::STATIC_SIZE, LocalCdcResp::STATIC_SIZE, InfoResp::STATIC_SIZE, RegistryResp::STATIC_SIZE, LocalChangedBlockServicesResp::STATIC_SIZE, CreateLocationResp::STATIC_SIZE, RenameLocationResp::STATIC_SIZE, RegisterShardResp::STATIC_SIZE, LocationsResp::STATIC_SIZE, RegisterCdcResp::STATIC_SIZE, SetBlockServiceFlagsResp::STATIC_SIZE, RegisterBlockServicesResp::STATIC_SIZE, ChangedBlockServicesAtLocationResp::STATIC_SIZE, ShardsAtLocationResp::STATIC_SIZE, CdcAtLocationResp::STATIC_SIZE, RegisterRegistryResp::STATIC_SIZE, AllRegistryReplicasResp::STATIC_SIZE, ShardBlockServicesDEPRECATEDResp::STATIC_SIZE, CdcReplicasDEPRECATEDResp::STATIC_SIZE, AllShardsResp::STATIC_SIZE, DecommissionBlockServiceResp::STATIC_SIZE, MoveShardLeaderResp::STATIC_SIZE, ClearShardInfoResp::STATIC_SIZE, ShardBlockServicesResp::STATIC_SIZE, AllCdcResp::STATIC_SIZE, EraseDecommissionedBlockResp::STATIC_SIZE, AllBlockServicesDeprecatedResp::STATIC_SIZE, AllBlockServicesResp::STATIC_SIZE, MoveCdcLeaderResp::STATIC_SIZE, ClearCdcInfoResp::STATIC_SIZE, UpdateBlockServicePathResp::STATIC_SIZE, SetBlockServiceHasFilesResp::STATIC_SIZE, BlockServicesNeedingMigrationResp::STATIC_SIZE};
    RegistryMessageKind _kind = RegistryMessageKind::EMPTY;
    std::variant<TernError, LocalShardsResp, LocalCdcResp, InfoResp, RegistryResp, LocalChangedBlockServicesResp, CreateLocationResp, RenameLocationResp, RegisterShardResp, LocationsResp, RegisterCdcResp, SetBlockServiceFlagsResp, RegisterBlockServicesResp, ChangedBlockServicesAtLocationResp, ShardsAtLocationResp, CdcAtLocationResp, RegisterRegistryResp, AllRegistryReplicasResp, ShardBlockServicesDEPRECATEDResp, CdcReplicasDEPRECATEDResp, AllShardsResp, DecommissionBlockServiceResp, MoveShardLeaderResp, ClearShardInfoResp, ShardBlockServicesResp, AllCdcResp, EraseDecommissionedBlockResp, AllBlockServicesDeprecatedResp, AllBlockServicesResp, MoveCdcLeaderResp, ClearCdcInfoResp, UpdateBlockServicePathResp, SetBlockServiceHasFilesResp, BlockServicesNeedingMigrationResp> _data;
public:
    RegistryRespContainer();
    RegistryRespContainer(const RegistryRespContainer& other);
    RegistryRespContainer(RegistryRespContainer&& other);
    void operator=(const RegistryRespContainer& other);
    void operator=(RegistryRespContainer&& other);

    RegistryMessageKind kind() const { return _kind; }

    const TernError& getError() const;
    TernError& setError();
    const LocalShardsResp& getLocalShards() const;
    LocalShardsResp& setLocalShards();
    const LocalCdcResp& getLocalCdc() const;
    LocalCdcResp& setLocalCdc();
    const InfoResp& getInfo() const;
    InfoResp& setInfo();
    const RegistryResp& getRegistry() const;
    RegistryResp& setRegistry();
    const LocalChangedBlockServicesResp& getLocalChangedBlockServices() const;
    LocalChangedBlockServicesResp& setLocalChangedBlockServices();
    const CreateLocationResp& getCreateLocation() const;
    CreateLocationResp& setCreateLocation();
    const RenameLocationResp& getRenameLocation() const;
    RenameLocationResp& setRenameLocation();
    const RegisterShardResp& getRegisterShard() const;
    RegisterShardResp& setRegisterShard();
    const LocationsResp& getLocations() const;
    LocationsResp& setLocations();
    const RegisterCdcResp& getRegisterCdc() const;
    RegisterCdcResp& setRegisterCdc();
    const SetBlockServiceFlagsResp& getSetBlockServiceFlags() const;
    SetBlockServiceFlagsResp& setSetBlockServiceFlags();
    const RegisterBlockServicesResp& getRegisterBlockServices() const;
    RegisterBlockServicesResp& setRegisterBlockServices();
    const ChangedBlockServicesAtLocationResp& getChangedBlockServicesAtLocation() const;
    ChangedBlockServicesAtLocationResp& setChangedBlockServicesAtLocation();
    const ShardsAtLocationResp& getShardsAtLocation() const;
    ShardsAtLocationResp& setShardsAtLocation();
    const CdcAtLocationResp& getCdcAtLocation() const;
    CdcAtLocationResp& setCdcAtLocation();
    const RegisterRegistryResp& getRegisterRegistry() const;
    RegisterRegistryResp& setRegisterRegistry();
    const AllRegistryReplicasResp& getAllRegistryReplicas() const;
    AllRegistryReplicasResp& setAllRegistryReplicas();
    const ShardBlockServicesDEPRECATEDResp& getShardBlockServicesDEPRECATED() const;
    ShardBlockServicesDEPRECATEDResp& setShardBlockServicesDEPRECATED();
    const CdcReplicasDEPRECATEDResp& getCdcReplicasDEPRECATED() const;
    CdcReplicasDEPRECATEDResp& setCdcReplicasDEPRECATED();
    const AllShardsResp& getAllShards() const;
    AllShardsResp& setAllShards();
    const DecommissionBlockServiceResp& getDecommissionBlockService() const;
    DecommissionBlockServiceResp& setDecommissionBlockService();
    const MoveShardLeaderResp& getMoveShardLeader() const;
    MoveShardLeaderResp& setMoveShardLeader();
    const ClearShardInfoResp& getClearShardInfo() const;
    ClearShardInfoResp& setClearShardInfo();
    const ShardBlockServicesResp& getShardBlockServices() const;
    ShardBlockServicesResp& setShardBlockServices();
    const AllCdcResp& getAllCdc() const;
    AllCdcResp& setAllCdc();
    const EraseDecommissionedBlockResp& getEraseDecommissionedBlock() const;
    EraseDecommissionedBlockResp& setEraseDecommissionedBlock();
    const AllBlockServicesDeprecatedResp& getAllBlockServicesDeprecated() const;
    AllBlockServicesDeprecatedResp& setAllBlockServicesDeprecated();
    const AllBlockServicesResp& getAllBlockServices() const;
    AllBlockServicesResp& setAllBlockServices();
    const MoveCdcLeaderResp& getMoveCdcLeader() const;
    MoveCdcLeaderResp& setMoveCdcLeader();
    const ClearCdcInfoResp& getClearCdcInfo() const;
    ClearCdcInfoResp& setClearCdcInfo();
    const UpdateBlockServicePathResp& getUpdateBlockServicePath() const;
    UpdateBlockServicePathResp& setUpdateBlockServicePath();
    const SetBlockServiceHasFilesResp& getSetBlockServiceHasFiles() const;
    SetBlockServiceHasFilesResp& setSetBlockServiceHasFiles();
    const BlockServicesNeedingMigrationResp& getBlockServicesNeedingMigration() const;
    BlockServicesNeedingMigrationResp& setBlockServicesNeedingMigration();

    void clear() { _kind = RegistryMessageKind::EMPTY; };

    static constexpr size_t STATIC_SIZE = sizeof(RegistryMessageKind) + *std::max_element(_staticSizes.begin(), _staticSizes.end());
    size_t packedSize() const;
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    bool operator==(const RegistryRespContainer& other) const;
};

std::ostream& operator<<(std::ostream& out, const RegistryRespContainer& x);

struct LogReqContainer {
private:
    static constexpr std::array<size_t,7> _staticSizes = {LogWriteReq::STATIC_SIZE, ReleaseReq::STATIC_SIZE, LogReadReq::STATIC_SIZE, NewLeaderReq::STATIC_SIZE, NewLeaderConfirmReq::STATIC_SIZE, LogRecoveryReadReq::STATIC_SIZE, LogRecoveryWriteReq::STATIC_SIZE};
    LogMessageKind _kind = LogMessageKind::EMPTY;
    std::variant<LogWriteReq, ReleaseReq, LogReadReq, NewLeaderReq, NewLeaderConfirmReq, LogRecoveryReadReq, LogRecoveryWriteReq> _data;
public:
    LogReqContainer();
    LogReqContainer(const LogReqContainer& other);
    LogReqContainer(LogReqContainer&& other);
    void operator=(const LogReqContainer& other);
    void operator=(LogReqContainer&& other);

    LogMessageKind kind() const { return _kind; }

    const LogWriteReq& getLogWrite() const;
    LogWriteReq& setLogWrite();
    const ReleaseReq& getRelease() const;
    ReleaseReq& setRelease();
    const LogReadReq& getLogRead() const;
    LogReadReq& setLogRead();
    const NewLeaderReq& getNewLeader() const;
    NewLeaderReq& setNewLeader();
    const NewLeaderConfirmReq& getNewLeaderConfirm() const;
    NewLeaderConfirmReq& setNewLeaderConfirm();
    const LogRecoveryReadReq& getLogRecoveryRead() const;
    LogRecoveryReadReq& setLogRecoveryRead();
    const LogRecoveryWriteReq& getLogRecoveryWrite() const;
    LogRecoveryWriteReq& setLogRecoveryWrite();

    void clear() { _kind = LogMessageKind::EMPTY; };

    static constexpr size_t STATIC_SIZE = sizeof(LogMessageKind) + *std::max_element(_staticSizes.begin(), _staticSizes.end());
    size_t packedSize() const;
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    bool operator==(const LogReqContainer& other) const;
};

std::ostream& operator<<(std::ostream& out, const LogReqContainer& x);

struct LogRespContainer {
private:
    static constexpr std::array<size_t,8> _staticSizes = {sizeof(TernError), LogWriteResp::STATIC_SIZE, ReleaseResp::STATIC_SIZE, LogReadResp::STATIC_SIZE, NewLeaderResp::STATIC_SIZE, NewLeaderConfirmResp::STATIC_SIZE, LogRecoveryReadResp::STATIC_SIZE, LogRecoveryWriteResp::STATIC_SIZE};
    LogMessageKind _kind = LogMessageKind::EMPTY;
    std::variant<TernError, LogWriteResp, ReleaseResp, LogReadResp, NewLeaderResp, NewLeaderConfirmResp, LogRecoveryReadResp, LogRecoveryWriteResp> _data;
public:
    LogRespContainer();
    LogRespContainer(const LogRespContainer& other);
    LogRespContainer(LogRespContainer&& other);
    void operator=(const LogRespContainer& other);
    void operator=(LogRespContainer&& other);

    LogMessageKind kind() const { return _kind; }

    const TernError& getError() const;
    TernError& setError();
    const LogWriteResp& getLogWrite() const;
    LogWriteResp& setLogWrite();
    const ReleaseResp& getRelease() const;
    ReleaseResp& setRelease();
    const LogReadResp& getLogRead() const;
    LogReadResp& setLogRead();
    const NewLeaderResp& getNewLeader() const;
    NewLeaderResp& setNewLeader();
    const NewLeaderConfirmResp& getNewLeaderConfirm() const;
    NewLeaderConfirmResp& setNewLeaderConfirm();
    const LogRecoveryReadResp& getLogRecoveryRead() const;
    LogRecoveryReadResp& setLogRecoveryRead();
    const LogRecoveryWriteResp& getLogRecoveryWrite() const;
    LogRecoveryWriteResp& setLogRecoveryWrite();

    void clear() { _kind = LogMessageKind::EMPTY; };

    static constexpr size_t STATIC_SIZE = sizeof(LogMessageKind) + *std::max_element(_staticSizes.begin(), _staticSizes.end());
    size_t packedSize() const;
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    bool operator==(const LogRespContainer& other) const;
};

std::ostream& operator<<(std::ostream& out, const LogRespContainer& x);

enum class ShardLogEntryKind : uint16_t {
    CONSTRUCT_FILE = 1,
    LINK_FILE = 2,
    SAME_DIRECTORY_RENAME = 3,
    SOFT_UNLINK_FILE = 4,
    CREATE_DIRECTORY_INODE = 5,
    CREATE_LOCKED_CURRENT_EDGE = 6,
    UNLOCK_CURRENT_EDGE = 7,
    LOCK_CURRENT_EDGE = 8,
    REMOVE_DIRECTORY_OWNER = 9,
    REMOVE_INODE = 10,
    SET_DIRECTORY_OWNER = 11,
    SET_DIRECTORY_INFO = 12,
    REMOVE_NON_OWNED_EDGE = 13,
    SCRAP_TRANSIENT_FILE = 14,
    REMOVE_SPAN_INITIATE = 15,
    ADD_SPAN_INITIATE = 16,
    ADD_SPAN_CERTIFY = 17,
    ADD_INLINE_SPAN = 18,
    MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED = 19,
    REMOVE_SPAN_CERTIFY = 20,
    REMOVE_OWNED_SNAPSHOT_FILE_EDGE = 21,
    SWAP_BLOCKS = 22,
    MOVE_SPAN = 23,
    SET_TIME = 24,
    REMOVE_ZERO_BLOCK_SERVICE_FILES = 25,
    SWAP_SPANS = 26,
    SAME_DIRECTORY_RENAME_SNAPSHOT = 27,
    ADD_SPAN_AT_LOCATION_INITIATE = 28,
    ADD_SPAN_LOCATION = 29,
    SAME_SHARD_HARD_FILE_UNLINK = 30,
    MAKE_FILE_TRANSIENT = 31,
    EMPTY = 255,
};

std::ostream& operator<<(std::ostream& out, ShardLogEntryKind err);

struct ConstructFileEntry {
    uint8_t type;
    TernTime deadlineTime;
    BincodeBytes note;

    static constexpr uint16_t STATIC_SIZE = 1 + 8 + BincodeBytes::STATIC_SIZE; // type + deadlineTime + note

    ConstructFileEntry() { clear(); }
    explicit ConstructFileEntry(uint8_t type_, TernTime deadlineTime_, BincodeBytes&& note_) : type(type_), deadlineTime(deadlineTime_), note(std::move(note_)) {}
    ConstructFileEntry(const ConstructFileEntry&) = default;
    ConstructFileEntry& operator=(const ConstructFileEntry&) = default;
    ConstructFileEntry(ConstructFileEntry&&) = default;
    ConstructFileEntry& operator=(ConstructFileEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // type
        _size += 8; // deadlineTime
        _size += note.packedSize(); // note
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ConstructFileEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ConstructFileEntry& x);

struct LinkFileEntry {
    InodeId fileId;
    InodeId ownerId;
    BincodeBytes name;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE; // fileId + ownerId + name

    LinkFileEntry() { clear(); }
    explicit LinkFileEntry(InodeId fileId_, InodeId ownerId_, BincodeBytes&& name_) : fileId(fileId_), ownerId(ownerId_), name(std::move(name_)) {}
    LinkFileEntry(const LinkFileEntry&) = default;
    LinkFileEntry& operator=(const LinkFileEntry&) = default;
    LinkFileEntry(LinkFileEntry&&) = default;
    LinkFileEntry& operator=(LinkFileEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += 8; // ownerId
        _size += name.packedSize(); // name
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LinkFileEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LinkFileEntry& x);

struct SameDirectoryRenameEntry {
    InodeId dirId;
    InodeId targetId;
    BincodeBytes oldName;
    TernTime oldCreationTime;
    BincodeBytes newName;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8 + BincodeBytes::STATIC_SIZE; // dirId + targetId + oldName + oldCreationTime + newName

    SameDirectoryRenameEntry() { clear(); }
    explicit SameDirectoryRenameEntry(InodeId dirId_, InodeId targetId_, BincodeBytes&& oldName_, TernTime oldCreationTime_, BincodeBytes&& newName_) : dirId(dirId_), targetId(targetId_), oldName(std::move(oldName_)), oldCreationTime(oldCreationTime_), newName(std::move(newName_)) {}
    SameDirectoryRenameEntry(const SameDirectoryRenameEntry&) = default;
    SameDirectoryRenameEntry& operator=(const SameDirectoryRenameEntry&) = default;
    SameDirectoryRenameEntry(SameDirectoryRenameEntry&&) = default;
    SameDirectoryRenameEntry& operator=(SameDirectoryRenameEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += 8; // targetId
        _size += oldName.packedSize(); // oldName
        _size += 8; // oldCreationTime
        _size += newName.packedSize(); // newName
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SameDirectoryRenameEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameEntry& x);

struct SoftUnlinkFileEntry {
    InodeId ownerId;
    InodeId fileId;
    BincodeBytes name;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8; // ownerId + fileId + name + creationTime

    SoftUnlinkFileEntry() { clear(); }
    explicit SoftUnlinkFileEntry(InodeId ownerId_, InodeId fileId_, BincodeBytes&& name_, TernTime creationTime_) : ownerId(ownerId_), fileId(fileId_), name(std::move(name_)), creationTime(creationTime_) {}
    SoftUnlinkFileEntry(const SoftUnlinkFileEntry&) = default;
    SoftUnlinkFileEntry& operator=(const SoftUnlinkFileEntry&) = default;
    SoftUnlinkFileEntry(SoftUnlinkFileEntry&&) = default;
    SoftUnlinkFileEntry& operator=(SoftUnlinkFileEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // ownerId
        _size += 8; // fileId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SoftUnlinkFileEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SoftUnlinkFileEntry& x);

struct CreateDirectoryInodeEntry {
    InodeId id;
    InodeId ownerId;
    DirectoryInfo info;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + DirectoryInfo::STATIC_SIZE; // id + ownerId + info

    CreateDirectoryInodeEntry() { clear(); }
    explicit CreateDirectoryInodeEntry(InodeId id_, InodeId ownerId_, DirectoryInfo&& info_) : id(id_), ownerId(ownerId_), info(std::move(info_)) {}
    CreateDirectoryInodeEntry(const CreateDirectoryInodeEntry&) = default;
    CreateDirectoryInodeEntry& operator=(const CreateDirectoryInodeEntry&) = default;
    CreateDirectoryInodeEntry(CreateDirectoryInodeEntry&&) = default;
    CreateDirectoryInodeEntry& operator=(CreateDirectoryInodeEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 8; // ownerId
        _size += info.packedSize(); // info
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CreateDirectoryInodeEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CreateDirectoryInodeEntry& x);

struct CreateLockedCurrentEdgeEntry {
    InodeId dirId;
    BincodeBytes name;
    InodeId targetId;
    TernTime oldCreationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeBytes::STATIC_SIZE + 8 + 8; // dirId + name + targetId + oldCreationTime

    CreateLockedCurrentEdgeEntry() { clear(); }
    explicit CreateLockedCurrentEdgeEntry(InodeId dirId_, BincodeBytes&& name_, InodeId targetId_, TernTime oldCreationTime_) : dirId(dirId_), name(std::move(name_)), targetId(targetId_), oldCreationTime(oldCreationTime_) {}
    CreateLockedCurrentEdgeEntry(const CreateLockedCurrentEdgeEntry&) = default;
    CreateLockedCurrentEdgeEntry& operator=(const CreateLockedCurrentEdgeEntry&) = default;
    CreateLockedCurrentEdgeEntry(CreateLockedCurrentEdgeEntry&&) = default;
    CreateLockedCurrentEdgeEntry& operator=(CreateLockedCurrentEdgeEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += name.packedSize(); // name
        _size += 8; // targetId
        _size += 8; // oldCreationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const CreateLockedCurrentEdgeEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const CreateLockedCurrentEdgeEntry& x);

struct UnlockCurrentEdgeEntry {
    InodeId dirId;
    BincodeBytes name;
    TernTime creationTime;
    InodeId targetId;
    bool wasMoved;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeBytes::STATIC_SIZE + 8 + 8 + 1; // dirId + name + creationTime + targetId + wasMoved

    UnlockCurrentEdgeEntry() { clear(); }
    explicit UnlockCurrentEdgeEntry(InodeId dirId_, BincodeBytes&& name_, TernTime creationTime_, InodeId targetId_, bool wasMoved_) : dirId(dirId_), name(std::move(name_)), creationTime(creationTime_), targetId(targetId_), wasMoved(wasMoved_) {}
    UnlockCurrentEdgeEntry(const UnlockCurrentEdgeEntry&) = default;
    UnlockCurrentEdgeEntry& operator=(const UnlockCurrentEdgeEntry&) = default;
    UnlockCurrentEdgeEntry(UnlockCurrentEdgeEntry&&) = default;
    UnlockCurrentEdgeEntry& operator=(UnlockCurrentEdgeEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        _size += 8; // targetId
        _size += 1; // wasMoved
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const UnlockCurrentEdgeEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const UnlockCurrentEdgeEntry& x);

struct LockCurrentEdgeEntry {
    InodeId dirId;
    BincodeBytes name;
    TernTime creationTime;
    InodeId targetId;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeBytes::STATIC_SIZE + 8 + 8; // dirId + name + creationTime + targetId

    LockCurrentEdgeEntry() { clear(); }
    explicit LockCurrentEdgeEntry(InodeId dirId_, BincodeBytes&& name_, TernTime creationTime_, InodeId targetId_) : dirId(dirId_), name(std::move(name_)), creationTime(creationTime_), targetId(targetId_) {}
    LockCurrentEdgeEntry(const LockCurrentEdgeEntry&) = default;
    LockCurrentEdgeEntry& operator=(const LockCurrentEdgeEntry&) = default;
    LockCurrentEdgeEntry(LockCurrentEdgeEntry&&) = default;
    LockCurrentEdgeEntry& operator=(LockCurrentEdgeEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        _size += 8; // targetId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const LockCurrentEdgeEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const LockCurrentEdgeEntry& x);

struct RemoveDirectoryOwnerEntry {
    InodeId dirId;
    DirectoryInfo info;

    static constexpr uint16_t STATIC_SIZE = 8 + DirectoryInfo::STATIC_SIZE; // dirId + info

    RemoveDirectoryOwnerEntry() { clear(); }
    explicit RemoveDirectoryOwnerEntry(InodeId dirId_, DirectoryInfo&& info_) : dirId(dirId_), info(std::move(info_)) {}
    RemoveDirectoryOwnerEntry(const RemoveDirectoryOwnerEntry&) = default;
    RemoveDirectoryOwnerEntry& operator=(const RemoveDirectoryOwnerEntry&) = default;
    RemoveDirectoryOwnerEntry(RemoveDirectoryOwnerEntry&&) = default;
    RemoveDirectoryOwnerEntry& operator=(RemoveDirectoryOwnerEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += info.packedSize(); // info
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveDirectoryOwnerEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveDirectoryOwnerEntry& x);

struct RemoveInodeEntry {
    InodeId id;

    static constexpr uint16_t STATIC_SIZE = 8; // id

    RemoveInodeEntry() { clear(); }
    explicit RemoveInodeEntry(InodeId id_) : id(id_) {}
    RemoveInodeEntry(const RemoveInodeEntry&) = default;
    RemoveInodeEntry& operator=(const RemoveInodeEntry&) = default;
    RemoveInodeEntry(RemoveInodeEntry&&) = default;
    RemoveInodeEntry& operator=(RemoveInodeEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveInodeEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveInodeEntry& x);

struct SetDirectoryOwnerEntry {
    InodeId dirId;
    InodeId ownerId;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // dirId + ownerId

    SetDirectoryOwnerEntry() { clear(); }
    explicit SetDirectoryOwnerEntry(InodeId dirId_, InodeId ownerId_) : dirId(dirId_), ownerId(ownerId_) {}
    SetDirectoryOwnerEntry(const SetDirectoryOwnerEntry&) = default;
    SetDirectoryOwnerEntry& operator=(const SetDirectoryOwnerEntry&) = default;
    SetDirectoryOwnerEntry(SetDirectoryOwnerEntry&&) = default;
    SetDirectoryOwnerEntry& operator=(SetDirectoryOwnerEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += 8; // ownerId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetDirectoryOwnerEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetDirectoryOwnerEntry& x);

struct SetDirectoryInfoEntry {
    InodeId dirId;
    DirectoryInfo info;

    static constexpr uint16_t STATIC_SIZE = 8 + DirectoryInfo::STATIC_SIZE; // dirId + info

    SetDirectoryInfoEntry() { clear(); }
    explicit SetDirectoryInfoEntry(InodeId dirId_, DirectoryInfo&& info_) : dirId(dirId_), info(std::move(info_)) {}
    SetDirectoryInfoEntry(const SetDirectoryInfoEntry&) = default;
    SetDirectoryInfoEntry& operator=(const SetDirectoryInfoEntry&) = default;
    SetDirectoryInfoEntry(SetDirectoryInfoEntry&&) = default;
    SetDirectoryInfoEntry& operator=(SetDirectoryInfoEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += info.packedSize(); // info
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetDirectoryInfoEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetDirectoryInfoEntry& x);

struct RemoveNonOwnedEdgeEntry {
    InodeId dirId;
    InodeId targetId;
    BincodeBytes name;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8; // dirId + targetId + name + creationTime

    RemoveNonOwnedEdgeEntry() { clear(); }
    explicit RemoveNonOwnedEdgeEntry(InodeId dirId_, InodeId targetId_, BincodeBytes&& name_, TernTime creationTime_) : dirId(dirId_), targetId(targetId_), name(std::move(name_)), creationTime(creationTime_) {}
    RemoveNonOwnedEdgeEntry(const RemoveNonOwnedEdgeEntry&) = default;
    RemoveNonOwnedEdgeEntry& operator=(const RemoveNonOwnedEdgeEntry&) = default;
    RemoveNonOwnedEdgeEntry(RemoveNonOwnedEdgeEntry&&) = default;
    RemoveNonOwnedEdgeEntry& operator=(RemoveNonOwnedEdgeEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += 8; // targetId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveNonOwnedEdgeEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveNonOwnedEdgeEntry& x);

struct ScrapTransientFileEntry {
    InodeId id;
    TernTime deadlineTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // id + deadlineTime

    ScrapTransientFileEntry() { clear(); }
    explicit ScrapTransientFileEntry(InodeId id_, TernTime deadlineTime_) : id(id_), deadlineTime(deadlineTime_) {}
    ScrapTransientFileEntry(const ScrapTransientFileEntry&) = default;
    ScrapTransientFileEntry& operator=(const ScrapTransientFileEntry&) = default;
    ScrapTransientFileEntry(ScrapTransientFileEntry&&) = default;
    ScrapTransientFileEntry& operator=(ScrapTransientFileEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 8; // deadlineTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const ScrapTransientFileEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const ScrapTransientFileEntry& x);

struct RemoveSpanInitiateEntry {
    InodeId fileId;

    static constexpr uint16_t STATIC_SIZE = 8; // fileId

    RemoveSpanInitiateEntry() { clear(); }
    explicit RemoveSpanInitiateEntry(InodeId fileId_) : fileId(fileId_) {}
    RemoveSpanInitiateEntry(const RemoveSpanInitiateEntry&) = default;
    RemoveSpanInitiateEntry& operator=(const RemoveSpanInitiateEntry&) = default;
    RemoveSpanInitiateEntry(RemoveSpanInitiateEntry&&) = default;
    RemoveSpanInitiateEntry& operator=(RemoveSpanInitiateEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveSpanInitiateEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveSpanInitiateEntry& x);

struct AddSpanInitiateEntry {
    bool withReference;
    InodeId fileId;
    uint64_t byteOffset;
    uint32_t size;
    Crc crc;
    uint8_t storageClass;
    Parity parity;
    uint8_t stripes;
    uint32_t cellSize;
    BincodeList<EntryNewBlockInfo> bodyBlocks;
    BincodeList<Crc> bodyStripes;

    static constexpr uint16_t STATIC_SIZE = 1 + 8 + 8 + 4 + 4 + 1 + 1 + 1 + 4 + BincodeList<EntryNewBlockInfo>::STATIC_SIZE + BincodeList<Crc>::STATIC_SIZE; // withReference + fileId + byteOffset + size + crc + storageClass + parity + stripes + cellSize + bodyBlocks + bodyStripes

    AddSpanInitiateEntry() { clear(); }
    explicit AddSpanInitiateEntry(bool withReference_, InodeId fileId_, uint64_t byteOffset_, uint32_t size_, Crc crc_, uint8_t storageClass_, Parity parity_, uint8_t stripes_, uint32_t cellSize_, BincodeList<EntryNewBlockInfo>&& bodyBlocks_, BincodeList<Crc>&& bodyStripes_) : withReference(withReference_), fileId(fileId_), byteOffset(byteOffset_), size(size_), crc(crc_), storageClass(storageClass_), parity(parity_), stripes(stripes_), cellSize(cellSize_), bodyBlocks(std::move(bodyBlocks_)), bodyStripes(std::move(bodyStripes_)) {}
    AddSpanInitiateEntry(const AddSpanInitiateEntry&) = default;
    AddSpanInitiateEntry& operator=(const AddSpanInitiateEntry&) = default;
    AddSpanInitiateEntry(AddSpanInitiateEntry&&) = default;
    AddSpanInitiateEntry& operator=(AddSpanInitiateEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // withReference
        _size += 8; // fileId
        _size += 8; // byteOffset
        _size += 4; // size
        _size += 4; // crc
        _size += 1; // storageClass
        _size += 1; // parity
        _size += 1; // stripes
        _size += 4; // cellSize
        _size += bodyBlocks.packedSize(); // bodyBlocks
        _size += bodyStripes.packedSize(); // bodyStripes
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanInitiateEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanInitiateEntry& x);

struct AddSpanCertifyEntry {
    InodeId fileId;
    uint64_t byteOffset;
    BincodeList<BlockProof> proofs;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeList<BlockProof>::STATIC_SIZE; // fileId + byteOffset + proofs

    AddSpanCertifyEntry() { clear(); }
    explicit AddSpanCertifyEntry(InodeId fileId_, uint64_t byteOffset_, BincodeList<BlockProof>&& proofs_) : fileId(fileId_), byteOffset(byteOffset_), proofs(std::move(proofs_)) {}
    AddSpanCertifyEntry(const AddSpanCertifyEntry&) = default;
    AddSpanCertifyEntry& operator=(const AddSpanCertifyEntry&) = default;
    AddSpanCertifyEntry(AddSpanCertifyEntry&&) = default;
    AddSpanCertifyEntry& operator=(AddSpanCertifyEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += 8; // byteOffset
        _size += proofs.packedSize(); // proofs
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanCertifyEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanCertifyEntry& x);

struct AddInlineSpanEntry {
    InodeId fileId;
    uint8_t storageClass;
    uint64_t byteOffset;
    uint32_t size;
    Crc crc;
    BincodeBytes body;

    static constexpr uint16_t STATIC_SIZE = 8 + 1 + 8 + 4 + 4 + BincodeBytes::STATIC_SIZE; // fileId + storageClass + byteOffset + size + crc + body

    AddInlineSpanEntry() { clear(); }
    explicit AddInlineSpanEntry(InodeId fileId_, uint8_t storageClass_, uint64_t byteOffset_, uint32_t size_, Crc crc_, BincodeBytes&& body_) : fileId(fileId_), storageClass(storageClass_), byteOffset(byteOffset_), size(size_), crc(crc_), body(std::move(body_)) {}
    AddInlineSpanEntry(const AddInlineSpanEntry&) = default;
    AddInlineSpanEntry& operator=(const AddInlineSpanEntry&) = default;
    AddInlineSpanEntry(AddInlineSpanEntry&&) = default;
    AddInlineSpanEntry& operator=(AddInlineSpanEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += 1; // storageClass
        _size += 8; // byteOffset
        _size += 4; // size
        _size += 4; // crc
        _size += body.packedSize(); // body
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddInlineSpanEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddInlineSpanEntry& x);

struct MakeFileTransientDEPRECATEDEntry {
    InodeId id;
    BincodeBytes note;

    static constexpr uint16_t STATIC_SIZE = 8 + BincodeBytes::STATIC_SIZE; // id + note

    MakeFileTransientDEPRECATEDEntry() { clear(); }
    explicit MakeFileTransientDEPRECATEDEntry(InodeId id_, BincodeBytes&& note_) : id(id_), note(std::move(note_)) {}
    MakeFileTransientDEPRECATEDEntry(const MakeFileTransientDEPRECATEDEntry&) = default;
    MakeFileTransientDEPRECATEDEntry& operator=(const MakeFileTransientDEPRECATEDEntry&) = default;
    MakeFileTransientDEPRECATEDEntry(MakeFileTransientDEPRECATEDEntry&&) = default;
    MakeFileTransientDEPRECATEDEntry& operator=(MakeFileTransientDEPRECATEDEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += note.packedSize(); // note
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MakeFileTransientDEPRECATEDEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MakeFileTransientDEPRECATEDEntry& x);

struct RemoveSpanCertifyEntry {
    InodeId fileId;
    uint64_t byteOffset;
    BincodeList<BlockProof> proofs;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeList<BlockProof>::STATIC_SIZE; // fileId + byteOffset + proofs

    RemoveSpanCertifyEntry() { clear(); }
    explicit RemoveSpanCertifyEntry(InodeId fileId_, uint64_t byteOffset_, BincodeList<BlockProof>&& proofs_) : fileId(fileId_), byteOffset(byteOffset_), proofs(std::move(proofs_)) {}
    RemoveSpanCertifyEntry(const RemoveSpanCertifyEntry&) = default;
    RemoveSpanCertifyEntry& operator=(const RemoveSpanCertifyEntry&) = default;
    RemoveSpanCertifyEntry(RemoveSpanCertifyEntry&&) = default;
    RemoveSpanCertifyEntry& operator=(RemoveSpanCertifyEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId
        _size += 8; // byteOffset
        _size += proofs.packedSize(); // proofs
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveSpanCertifyEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveSpanCertifyEntry& x);

struct RemoveOwnedSnapshotFileEdgeEntry {
    InodeId ownerId;
    InodeId targetId;
    BincodeBytes name;
    TernTime creationTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8; // ownerId + targetId + name + creationTime

    RemoveOwnedSnapshotFileEdgeEntry() { clear(); }
    explicit RemoveOwnedSnapshotFileEdgeEntry(InodeId ownerId_, InodeId targetId_, BincodeBytes&& name_, TernTime creationTime_) : ownerId(ownerId_), targetId(targetId_), name(std::move(name_)), creationTime(creationTime_) {}
    RemoveOwnedSnapshotFileEdgeEntry(const RemoveOwnedSnapshotFileEdgeEntry&) = default;
    RemoveOwnedSnapshotFileEdgeEntry& operator=(const RemoveOwnedSnapshotFileEdgeEntry&) = default;
    RemoveOwnedSnapshotFileEdgeEntry(RemoveOwnedSnapshotFileEdgeEntry&&) = default;
    RemoveOwnedSnapshotFileEdgeEntry& operator=(RemoveOwnedSnapshotFileEdgeEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // ownerId
        _size += 8; // targetId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveOwnedSnapshotFileEdgeEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveOwnedSnapshotFileEdgeEntry& x);

struct SwapBlocksEntry {
    InodeId fileId1;
    uint64_t byteOffset1;
    uint64_t blockId1;
    InodeId fileId2;
    uint64_t byteOffset2;
    uint64_t blockId2;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 8 + 8 + 8 + 8; // fileId1 + byteOffset1 + blockId1 + fileId2 + byteOffset2 + blockId2

    SwapBlocksEntry() { clear(); }
    explicit SwapBlocksEntry(InodeId fileId1_, uint64_t byteOffset1_, uint64_t blockId1_, InodeId fileId2_, uint64_t byteOffset2_, uint64_t blockId2_) : fileId1(fileId1_), byteOffset1(byteOffset1_), blockId1(blockId1_), fileId2(fileId2_), byteOffset2(byteOffset2_), blockId2(blockId2_) {}
    SwapBlocksEntry(const SwapBlocksEntry&) = default;
    SwapBlocksEntry& operator=(const SwapBlocksEntry&) = default;
    SwapBlocksEntry(SwapBlocksEntry&&) = default;
    SwapBlocksEntry& operator=(SwapBlocksEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId1
        _size += 8; // byteOffset1
        _size += 8; // blockId1
        _size += 8; // fileId2
        _size += 8; // byteOffset2
        _size += 8; // blockId2
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SwapBlocksEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SwapBlocksEntry& x);

struct MoveSpanEntry {
    uint32_t spanSize;
    InodeId fileId1;
    uint64_t byteOffset1;
    BincodeFixedBytes<8> cookie1;
    InodeId fileId2;
    uint64_t byteOffset2;
    BincodeFixedBytes<8> cookie2;

    static constexpr uint16_t STATIC_SIZE = 4 + 8 + 8 + BincodeFixedBytes<8>::STATIC_SIZE + 8 + 8 + BincodeFixedBytes<8>::STATIC_SIZE; // spanSize + fileId1 + byteOffset1 + cookie1 + fileId2 + byteOffset2 + cookie2

    MoveSpanEntry() { clear(); }
    explicit MoveSpanEntry(uint32_t spanSize_, InodeId fileId1_, uint64_t byteOffset1_, BincodeFixedBytes<8>&& cookie1_, InodeId fileId2_, uint64_t byteOffset2_, BincodeFixedBytes<8>&& cookie2_) : spanSize(spanSize_), fileId1(fileId1_), byteOffset1(byteOffset1_), cookie1(std::move(cookie1_)), fileId2(fileId2_), byteOffset2(byteOffset2_), cookie2(std::move(cookie2_)) {}
    MoveSpanEntry(const MoveSpanEntry&) = default;
    MoveSpanEntry& operator=(const MoveSpanEntry&) = default;
    MoveSpanEntry(MoveSpanEntry&&) = default;
    MoveSpanEntry& operator=(MoveSpanEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 4; // spanSize
        _size += 8; // fileId1
        _size += 8; // byteOffset1
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie1
        _size += 8; // fileId2
        _size += 8; // byteOffset2
        _size += BincodeFixedBytes<8>::STATIC_SIZE; // cookie2
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MoveSpanEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MoveSpanEntry& x);

struct SetTimeEntry {
    InodeId id;
    uint64_t mtime;
    uint64_t atime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + 8; // id + mtime + atime

    SetTimeEntry() { clear(); }
    explicit SetTimeEntry(InodeId id_, uint64_t mtime_, uint64_t atime_) : id(id_), mtime(mtime_), atime(atime_) {}
    SetTimeEntry(const SetTimeEntry&) = default;
    SetTimeEntry& operator=(const SetTimeEntry&) = default;
    SetTimeEntry(SetTimeEntry&&) = default;
    SetTimeEntry& operator=(SetTimeEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 8; // mtime
        _size += 8; // atime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SetTimeEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SetTimeEntry& x);

struct RemoveZeroBlockServiceFilesEntry {
    BlockServiceId startBlockService;
    InodeId startFile;

    static constexpr uint16_t STATIC_SIZE = 8 + 8; // startBlockService + startFile

    RemoveZeroBlockServiceFilesEntry() { clear(); }
    explicit RemoveZeroBlockServiceFilesEntry(BlockServiceId startBlockService_, InodeId startFile_) : startBlockService(startBlockService_), startFile(startFile_) {}
    RemoveZeroBlockServiceFilesEntry(const RemoveZeroBlockServiceFilesEntry&) = default;
    RemoveZeroBlockServiceFilesEntry& operator=(const RemoveZeroBlockServiceFilesEntry&) = default;
    RemoveZeroBlockServiceFilesEntry(RemoveZeroBlockServiceFilesEntry&&) = default;
    RemoveZeroBlockServiceFilesEntry& operator=(RemoveZeroBlockServiceFilesEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // startBlockService
        _size += 8; // startFile
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const RemoveZeroBlockServiceFilesEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const RemoveZeroBlockServiceFilesEntry& x);

struct SwapSpansEntry {
    InodeId fileId1;
    uint64_t byteOffset1;
    BincodeList<uint64_t> blocks1;
    InodeId fileId2;
    uint64_t byteOffset2;
    BincodeList<uint64_t> blocks2;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeList<uint64_t>::STATIC_SIZE + 8 + 8 + BincodeList<uint64_t>::STATIC_SIZE; // fileId1 + byteOffset1 + blocks1 + fileId2 + byteOffset2 + blocks2

    SwapSpansEntry() { clear(); }
    explicit SwapSpansEntry(InodeId fileId1_, uint64_t byteOffset1_, BincodeList<uint64_t>&& blocks1_, InodeId fileId2_, uint64_t byteOffset2_, BincodeList<uint64_t>&& blocks2_) : fileId1(fileId1_), byteOffset1(byteOffset1_), blocks1(std::move(blocks1_)), fileId2(fileId2_), byteOffset2(byteOffset2_), blocks2(std::move(blocks2_)) {}
    SwapSpansEntry(const SwapSpansEntry&) = default;
    SwapSpansEntry& operator=(const SwapSpansEntry&) = default;
    SwapSpansEntry(SwapSpansEntry&&) = default;
    SwapSpansEntry& operator=(SwapSpansEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId1
        _size += 8; // byteOffset1
        _size += blocks1.packedSize(); // blocks1
        _size += 8; // fileId2
        _size += 8; // byteOffset2
        _size += blocks2.packedSize(); // blocks2
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SwapSpansEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SwapSpansEntry& x);

struct SameDirectoryRenameSnapshotEntry {
    InodeId dirId;
    InodeId targetId;
    BincodeBytes oldName;
    TernTime oldCreationTime;
    BincodeBytes newName;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8 + BincodeBytes::STATIC_SIZE; // dirId + targetId + oldName + oldCreationTime + newName

    SameDirectoryRenameSnapshotEntry() { clear(); }
    explicit SameDirectoryRenameSnapshotEntry(InodeId dirId_, InodeId targetId_, BincodeBytes&& oldName_, TernTime oldCreationTime_, BincodeBytes&& newName_) : dirId(dirId_), targetId(targetId_), oldName(std::move(oldName_)), oldCreationTime(oldCreationTime_), newName(std::move(newName_)) {}
    SameDirectoryRenameSnapshotEntry(const SameDirectoryRenameSnapshotEntry&) = default;
    SameDirectoryRenameSnapshotEntry& operator=(const SameDirectoryRenameSnapshotEntry&) = default;
    SameDirectoryRenameSnapshotEntry(SameDirectoryRenameSnapshotEntry&&) = default;
    SameDirectoryRenameSnapshotEntry& operator=(SameDirectoryRenameSnapshotEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // dirId
        _size += 8; // targetId
        _size += oldName.packedSize(); // oldName
        _size += 8; // oldCreationTime
        _size += newName.packedSize(); // newName
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SameDirectoryRenameSnapshotEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameSnapshotEntry& x);

struct AddSpanAtLocationInitiateEntry {
    uint8_t locationId;
    bool withReference;
    InodeId fileId;
    uint64_t byteOffset;
    uint32_t size;
    Crc crc;
    uint8_t storageClass;
    Parity parity;
    uint8_t stripes;
    uint32_t cellSize;
    BincodeList<EntryNewBlockInfo> bodyBlocks;
    BincodeList<Crc> bodyStripes;

    static constexpr uint16_t STATIC_SIZE = 1 + 1 + 8 + 8 + 4 + 4 + 1 + 1 + 1 + 4 + BincodeList<EntryNewBlockInfo>::STATIC_SIZE + BincodeList<Crc>::STATIC_SIZE; // locationId + withReference + fileId + byteOffset + size + crc + storageClass + parity + stripes + cellSize + bodyBlocks + bodyStripes

    AddSpanAtLocationInitiateEntry() { clear(); }
    explicit AddSpanAtLocationInitiateEntry(uint8_t locationId_, bool withReference_, InodeId fileId_, uint64_t byteOffset_, uint32_t size_, Crc crc_, uint8_t storageClass_, Parity parity_, uint8_t stripes_, uint32_t cellSize_, BincodeList<EntryNewBlockInfo>&& bodyBlocks_, BincodeList<Crc>&& bodyStripes_) : locationId(locationId_), withReference(withReference_), fileId(fileId_), byteOffset(byteOffset_), size(size_), crc(crc_), storageClass(storageClass_), parity(parity_), stripes(stripes_), cellSize(cellSize_), bodyBlocks(std::move(bodyBlocks_)), bodyStripes(std::move(bodyStripes_)) {}
    AddSpanAtLocationInitiateEntry(const AddSpanAtLocationInitiateEntry&) = default;
    AddSpanAtLocationInitiateEntry& operator=(const AddSpanAtLocationInitiateEntry&) = default;
    AddSpanAtLocationInitiateEntry(AddSpanAtLocationInitiateEntry&&) = default;
    AddSpanAtLocationInitiateEntry& operator=(AddSpanAtLocationInitiateEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 1; // locationId
        _size += 1; // withReference
        _size += 8; // fileId
        _size += 8; // byteOffset
        _size += 4; // size
        _size += 4; // crc
        _size += 1; // storageClass
        _size += 1; // parity
        _size += 1; // stripes
        _size += 4; // cellSize
        _size += bodyBlocks.packedSize(); // bodyBlocks
        _size += bodyStripes.packedSize(); // bodyStripes
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanAtLocationInitiateEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanAtLocationInitiateEntry& x);

struct AddSpanLocationEntry {
    InodeId fileId1;
    uint64_t byteOffset1;
    BincodeList<uint64_t> blocks1;
    InodeId fileId2;
    uint64_t byteOffset2;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeList<uint64_t>::STATIC_SIZE + 8 + 8; // fileId1 + byteOffset1 + blocks1 + fileId2 + byteOffset2

    AddSpanLocationEntry() { clear(); }
    explicit AddSpanLocationEntry(InodeId fileId1_, uint64_t byteOffset1_, BincodeList<uint64_t>&& blocks1_, InodeId fileId2_, uint64_t byteOffset2_) : fileId1(fileId1_), byteOffset1(byteOffset1_), blocks1(std::move(blocks1_)), fileId2(fileId2_), byteOffset2(byteOffset2_) {}
    AddSpanLocationEntry(const AddSpanLocationEntry&) = default;
    AddSpanLocationEntry& operator=(const AddSpanLocationEntry&) = default;
    AddSpanLocationEntry(AddSpanLocationEntry&&) = default;
    AddSpanLocationEntry& operator=(AddSpanLocationEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // fileId1
        _size += 8; // byteOffset1
        _size += blocks1.packedSize(); // blocks1
        _size += 8; // fileId2
        _size += 8; // byteOffset2
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const AddSpanLocationEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const AddSpanLocationEntry& x);

struct SameShardHardFileUnlinkEntry {
    InodeId ownerId;
    InodeId targetId;
    BincodeBytes name;
    TernTime creationTime;
    TernTime deadlineTime;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE + 8 + 8; // ownerId + targetId + name + creationTime + deadlineTime

    SameShardHardFileUnlinkEntry() { clear(); }
    explicit SameShardHardFileUnlinkEntry(InodeId ownerId_, InodeId targetId_, BincodeBytes&& name_, TernTime creationTime_, TernTime deadlineTime_) : ownerId(ownerId_), targetId(targetId_), name(std::move(name_)), creationTime(creationTime_), deadlineTime(deadlineTime_) {}
    SameShardHardFileUnlinkEntry(const SameShardHardFileUnlinkEntry&) = default;
    SameShardHardFileUnlinkEntry& operator=(const SameShardHardFileUnlinkEntry&) = default;
    SameShardHardFileUnlinkEntry(SameShardHardFileUnlinkEntry&&) = default;
    SameShardHardFileUnlinkEntry& operator=(SameShardHardFileUnlinkEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // ownerId
        _size += 8; // targetId
        _size += name.packedSize(); // name
        _size += 8; // creationTime
        _size += 8; // deadlineTime
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const SameShardHardFileUnlinkEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const SameShardHardFileUnlinkEntry& x);

struct MakeFileTransientEntry {
    InodeId id;
    TernTime deadlineTime;
    BincodeBytes note;

    static constexpr uint16_t STATIC_SIZE = 8 + 8 + BincodeBytes::STATIC_SIZE; // id + deadlineTime + note

    MakeFileTransientEntry() { clear(); }
    explicit MakeFileTransientEntry(InodeId id_, TernTime deadlineTime_, BincodeBytes&& note_) : id(id_), deadlineTime(deadlineTime_), note(std::move(note_)) {}
    MakeFileTransientEntry(const MakeFileTransientEntry&) = default;
    MakeFileTransientEntry& operator=(const MakeFileTransientEntry&) = default;
    MakeFileTransientEntry(MakeFileTransientEntry&&) = default;
    MakeFileTransientEntry& operator=(MakeFileTransientEntry&&) = default;
    size_t packedSize() const {
        size_t _size = 0;
        _size += 8; // id
        _size += 8; // deadlineTime
        _size += note.packedSize(); // note
        return _size;
    }
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    void clear();
    bool operator==(const MakeFileTransientEntry&rhs) const;
};

std::ostream& operator<<(std::ostream& out, const MakeFileTransientEntry& x);

struct ShardLogEntryContainer {
private:
    static constexpr std::array<size_t,31> _staticSizes = {ConstructFileEntry::STATIC_SIZE, LinkFileEntry::STATIC_SIZE, SameDirectoryRenameEntry::STATIC_SIZE, SoftUnlinkFileEntry::STATIC_SIZE, CreateDirectoryInodeEntry::STATIC_SIZE, CreateLockedCurrentEdgeEntry::STATIC_SIZE, UnlockCurrentEdgeEntry::STATIC_SIZE, LockCurrentEdgeEntry::STATIC_SIZE, RemoveDirectoryOwnerEntry::STATIC_SIZE, RemoveInodeEntry::STATIC_SIZE, SetDirectoryOwnerEntry::STATIC_SIZE, SetDirectoryInfoEntry::STATIC_SIZE, RemoveNonOwnedEdgeEntry::STATIC_SIZE, ScrapTransientFileEntry::STATIC_SIZE, RemoveSpanInitiateEntry::STATIC_SIZE, AddSpanInitiateEntry::STATIC_SIZE, AddSpanCertifyEntry::STATIC_SIZE, AddInlineSpanEntry::STATIC_SIZE, MakeFileTransientDEPRECATEDEntry::STATIC_SIZE, RemoveSpanCertifyEntry::STATIC_SIZE, RemoveOwnedSnapshotFileEdgeEntry::STATIC_SIZE, SwapBlocksEntry::STATIC_SIZE, MoveSpanEntry::STATIC_SIZE, SetTimeEntry::STATIC_SIZE, RemoveZeroBlockServiceFilesEntry::STATIC_SIZE, SwapSpansEntry::STATIC_SIZE, SameDirectoryRenameSnapshotEntry::STATIC_SIZE, AddSpanAtLocationInitiateEntry::STATIC_SIZE, AddSpanLocationEntry::STATIC_SIZE, SameShardHardFileUnlinkEntry::STATIC_SIZE, MakeFileTransientEntry::STATIC_SIZE};
    ShardLogEntryKind _kind = ShardLogEntryKind::EMPTY;
    std::variant<ConstructFileEntry, LinkFileEntry, SameDirectoryRenameEntry, SoftUnlinkFileEntry, CreateDirectoryInodeEntry, CreateLockedCurrentEdgeEntry, UnlockCurrentEdgeEntry, LockCurrentEdgeEntry, RemoveDirectoryOwnerEntry, RemoveInodeEntry, SetDirectoryOwnerEntry, SetDirectoryInfoEntry, RemoveNonOwnedEdgeEntry, ScrapTransientFileEntry, RemoveSpanInitiateEntry, AddSpanInitiateEntry, AddSpanCertifyEntry, AddInlineSpanEntry, MakeFileTransientDEPRECATEDEntry, RemoveSpanCertifyEntry, RemoveOwnedSnapshotFileEdgeEntry, SwapBlocksEntry, MoveSpanEntry, SetTimeEntry, RemoveZeroBlockServiceFilesEntry, SwapSpansEntry, SameDirectoryRenameSnapshotEntry, AddSpanAtLocationInitiateEntry, AddSpanLocationEntry, SameShardHardFileUnlinkEntry, MakeFileTransientEntry> _data;
public:
    ShardLogEntryContainer();
    ShardLogEntryContainer(const ShardLogEntryContainer& other);
    ShardLogEntryContainer(ShardLogEntryContainer&& other);
    void operator=(const ShardLogEntryContainer& other);
    void operator=(ShardLogEntryContainer&& other);

    ShardLogEntryKind kind() const { return _kind; }

    const ConstructFileEntry& getConstructFile() const;
    ConstructFileEntry& setConstructFile();
    const LinkFileEntry& getLinkFile() const;
    LinkFileEntry& setLinkFile();
    const SameDirectoryRenameEntry& getSameDirectoryRename() const;
    SameDirectoryRenameEntry& setSameDirectoryRename();
    const SoftUnlinkFileEntry& getSoftUnlinkFile() const;
    SoftUnlinkFileEntry& setSoftUnlinkFile();
    const CreateDirectoryInodeEntry& getCreateDirectoryInode() const;
    CreateDirectoryInodeEntry& setCreateDirectoryInode();
    const CreateLockedCurrentEdgeEntry& getCreateLockedCurrentEdge() const;
    CreateLockedCurrentEdgeEntry& setCreateLockedCurrentEdge();
    const UnlockCurrentEdgeEntry& getUnlockCurrentEdge() const;
    UnlockCurrentEdgeEntry& setUnlockCurrentEdge();
    const LockCurrentEdgeEntry& getLockCurrentEdge() const;
    LockCurrentEdgeEntry& setLockCurrentEdge();
    const RemoveDirectoryOwnerEntry& getRemoveDirectoryOwner() const;
    RemoveDirectoryOwnerEntry& setRemoveDirectoryOwner();
    const RemoveInodeEntry& getRemoveInode() const;
    RemoveInodeEntry& setRemoveInode();
    const SetDirectoryOwnerEntry& getSetDirectoryOwner() const;
    SetDirectoryOwnerEntry& setSetDirectoryOwner();
    const SetDirectoryInfoEntry& getSetDirectoryInfo() const;
    SetDirectoryInfoEntry& setSetDirectoryInfo();
    const RemoveNonOwnedEdgeEntry& getRemoveNonOwnedEdge() const;
    RemoveNonOwnedEdgeEntry& setRemoveNonOwnedEdge();
    const ScrapTransientFileEntry& getScrapTransientFile() const;
    ScrapTransientFileEntry& setScrapTransientFile();
    const RemoveSpanInitiateEntry& getRemoveSpanInitiate() const;
    RemoveSpanInitiateEntry& setRemoveSpanInitiate();
    const AddSpanInitiateEntry& getAddSpanInitiate() const;
    AddSpanInitiateEntry& setAddSpanInitiate();
    const AddSpanCertifyEntry& getAddSpanCertify() const;
    AddSpanCertifyEntry& setAddSpanCertify();
    const AddInlineSpanEntry& getAddInlineSpan() const;
    AddInlineSpanEntry& setAddInlineSpan();
    const MakeFileTransientDEPRECATEDEntry& getMakeFileTransientDEPRECATED() const;
    MakeFileTransientDEPRECATEDEntry& setMakeFileTransientDEPRECATED();
    const RemoveSpanCertifyEntry& getRemoveSpanCertify() const;
    RemoveSpanCertifyEntry& setRemoveSpanCertify();
    const RemoveOwnedSnapshotFileEdgeEntry& getRemoveOwnedSnapshotFileEdge() const;
    RemoveOwnedSnapshotFileEdgeEntry& setRemoveOwnedSnapshotFileEdge();
    const SwapBlocksEntry& getSwapBlocks() const;
    SwapBlocksEntry& setSwapBlocks();
    const MoveSpanEntry& getMoveSpan() const;
    MoveSpanEntry& setMoveSpan();
    const SetTimeEntry& getSetTime() const;
    SetTimeEntry& setSetTime();
    const RemoveZeroBlockServiceFilesEntry& getRemoveZeroBlockServiceFiles() const;
    RemoveZeroBlockServiceFilesEntry& setRemoveZeroBlockServiceFiles();
    const SwapSpansEntry& getSwapSpans() const;
    SwapSpansEntry& setSwapSpans();
    const SameDirectoryRenameSnapshotEntry& getSameDirectoryRenameSnapshot() const;
    SameDirectoryRenameSnapshotEntry& setSameDirectoryRenameSnapshot();
    const AddSpanAtLocationInitiateEntry& getAddSpanAtLocationInitiate() const;
    AddSpanAtLocationInitiateEntry& setAddSpanAtLocationInitiate();
    const AddSpanLocationEntry& getAddSpanLocation() const;
    AddSpanLocationEntry& setAddSpanLocation();
    const SameShardHardFileUnlinkEntry& getSameShardHardFileUnlink() const;
    SameShardHardFileUnlinkEntry& setSameShardHardFileUnlink();
    const MakeFileTransientEntry& getMakeFileTransient() const;
    MakeFileTransientEntry& setMakeFileTransient();

    void clear() { _kind = ShardLogEntryKind::EMPTY; };

    static constexpr size_t STATIC_SIZE = sizeof(ShardLogEntryKind) + *std::max_element(_staticSizes.begin(), _staticSizes.end());
    size_t packedSize() const;
    void pack(BincodeBuf& buf) const;
    void unpack(BincodeBuf& buf);
    bool operator==(const ShardLogEntryContainer& other) const;
};

std::ostream& operator<<(std::ostream& out, const ShardLogEntryContainer& x);

