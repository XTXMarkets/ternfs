// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// Automatically generated with go run bincodegen.
// Run `go generate ./...` from the go/ directory to regenerate it.
#include "MsgsGen.hpp"

std::ostream& operator<<(std::ostream& out, TernError err) {
    switch (err) {
    case TernError::NO_ERROR:
        out << "NO_ERROR";
        break;
    case TernError::INTERNAL_ERROR:
        out << "INTERNAL_ERROR";
        break;
    case TernError::FATAL_ERROR:
        out << "FATAL_ERROR";
        break;
    case TernError::TIMEOUT:
        out << "TIMEOUT";
        break;
    case TernError::MALFORMED_REQUEST:
        out << "MALFORMED_REQUEST";
        break;
    case TernError::MALFORMED_RESPONSE:
        out << "MALFORMED_RESPONSE";
        break;
    case TernError::NOT_AUTHORISED:
        out << "NOT_AUTHORISED";
        break;
    case TernError::UNRECOGNIZED_REQUEST:
        out << "UNRECOGNIZED_REQUEST";
        break;
    case TernError::FILE_NOT_FOUND:
        out << "FILE_NOT_FOUND";
        break;
    case TernError::DIRECTORY_NOT_FOUND:
        out << "DIRECTORY_NOT_FOUND";
        break;
    case TernError::NAME_NOT_FOUND:
        out << "NAME_NOT_FOUND";
        break;
    case TernError::EDGE_NOT_FOUND:
        out << "EDGE_NOT_FOUND";
        break;
    case TernError::EDGE_IS_LOCKED:
        out << "EDGE_IS_LOCKED";
        break;
    case TernError::TYPE_IS_DIRECTORY:
        out << "TYPE_IS_DIRECTORY";
        break;
    case TernError::TYPE_IS_NOT_DIRECTORY:
        out << "TYPE_IS_NOT_DIRECTORY";
        break;
    case TernError::BAD_COOKIE:
        out << "BAD_COOKIE";
        break;
    case TernError::INCONSISTENT_STORAGE_CLASS_PARITY:
        out << "INCONSISTENT_STORAGE_CLASS_PARITY";
        break;
    case TernError::LAST_SPAN_STATE_NOT_CLEAN:
        out << "LAST_SPAN_STATE_NOT_CLEAN";
        break;
    case TernError::COULD_NOT_PICK_BLOCK_SERVICES:
        out << "COULD_NOT_PICK_BLOCK_SERVICES";
        break;
    case TernError::BAD_SPAN_BODY:
        out << "BAD_SPAN_BODY";
        break;
    case TernError::SPAN_NOT_FOUND:
        out << "SPAN_NOT_FOUND";
        break;
    case TernError::BLOCK_SERVICE_NOT_FOUND:
        out << "BLOCK_SERVICE_NOT_FOUND";
        break;
    case TernError::CANNOT_CERTIFY_BLOCKLESS_SPAN:
        out << "CANNOT_CERTIFY_BLOCKLESS_SPAN";
        break;
    case TernError::BAD_NUMBER_OF_BLOCKS_PROOFS:
        out << "BAD_NUMBER_OF_BLOCKS_PROOFS";
        break;
    case TernError::BAD_BLOCK_PROOF:
        out << "BAD_BLOCK_PROOF";
        break;
    case TernError::CANNOT_OVERRIDE_NAME:
        out << "CANNOT_OVERRIDE_NAME";
        break;
    case TernError::NAME_IS_LOCKED:
        out << "NAME_IS_LOCKED";
        break;
    case TernError::MTIME_IS_TOO_RECENT:
        out << "MTIME_IS_TOO_RECENT";
        break;
    case TernError::MISMATCHING_TARGET:
        out << "MISMATCHING_TARGET";
        break;
    case TernError::MISMATCHING_OWNER:
        out << "MISMATCHING_OWNER";
        break;
    case TernError::MISMATCHING_CREATION_TIME:
        out << "MISMATCHING_CREATION_TIME";
        break;
    case TernError::DIRECTORY_NOT_EMPTY:
        out << "DIRECTORY_NOT_EMPTY";
        break;
    case TernError::FILE_IS_TRANSIENT:
        out << "FILE_IS_TRANSIENT";
        break;
    case TernError::OLD_DIRECTORY_NOT_FOUND:
        out << "OLD_DIRECTORY_NOT_FOUND";
        break;
    case TernError::NEW_DIRECTORY_NOT_FOUND:
        out << "NEW_DIRECTORY_NOT_FOUND";
        break;
    case TernError::LOOP_IN_DIRECTORY_RENAME:
        out << "LOOP_IN_DIRECTORY_RENAME";
        break;
    case TernError::DIRECTORY_HAS_OWNER:
        out << "DIRECTORY_HAS_OWNER";
        break;
    case TernError::FILE_IS_NOT_TRANSIENT:
        out << "FILE_IS_NOT_TRANSIENT";
        break;
    case TernError::FILE_NOT_EMPTY:
        out << "FILE_NOT_EMPTY";
        break;
    case TernError::CANNOT_REMOVE_ROOT_DIRECTORY:
        out << "CANNOT_REMOVE_ROOT_DIRECTORY";
        break;
    case TernError::FILE_EMPTY:
        out << "FILE_EMPTY";
        break;
    case TernError::CANNOT_REMOVE_DIRTY_SPAN:
        out << "CANNOT_REMOVE_DIRTY_SPAN";
        break;
    case TernError::BAD_SHARD:
        out << "BAD_SHARD";
        break;
    case TernError::BAD_NAME:
        out << "BAD_NAME";
        break;
    case TernError::MORE_RECENT_SNAPSHOT_EDGE:
        out << "MORE_RECENT_SNAPSHOT_EDGE";
        break;
    case TernError::MORE_RECENT_CURRENT_EDGE:
        out << "MORE_RECENT_CURRENT_EDGE";
        break;
    case TernError::BAD_DIRECTORY_INFO:
        out << "BAD_DIRECTORY_INFO";
        break;
    case TernError::DEADLINE_NOT_PASSED:
        out << "DEADLINE_NOT_PASSED";
        break;
    case TernError::SAME_SOURCE_AND_DESTINATION:
        out << "SAME_SOURCE_AND_DESTINATION";
        break;
    case TernError::SAME_DIRECTORIES:
        out << "SAME_DIRECTORIES";
        break;
    case TernError::SAME_SHARD:
        out << "SAME_SHARD";
        break;
    case TernError::BAD_PROTOCOL_VERSION:
        out << "BAD_PROTOCOL_VERSION";
        break;
    case TernError::BAD_CERTIFICATE:
        out << "BAD_CERTIFICATE";
        break;
    case TernError::BLOCK_TOO_RECENT_FOR_DELETION:
        out << "BLOCK_TOO_RECENT_FOR_DELETION";
        break;
    case TernError::BLOCK_FETCH_OUT_OF_BOUNDS:
        out << "BLOCK_FETCH_OUT_OF_BOUNDS";
        break;
    case TernError::BAD_BLOCK_CRC:
        out << "BAD_BLOCK_CRC";
        break;
    case TernError::BLOCK_TOO_BIG:
        out << "BLOCK_TOO_BIG";
        break;
    case TernError::BLOCK_NOT_FOUND:
        out << "BLOCK_NOT_FOUND";
        break;
    case TernError::CANNOT_UNSET_DECOMMISSIONED:
        out << "CANNOT_UNSET_DECOMMISSIONED";
        break;
    case TernError::CANNOT_REGISTER_DECOMMISSIONED_OR_STALE:
        out << "CANNOT_REGISTER_DECOMMISSIONED_OR_STALE";
        break;
    case TernError::BLOCK_TOO_OLD_FOR_WRITE:
        out << "BLOCK_TOO_OLD_FOR_WRITE";
        break;
    case TernError::BLOCK_IO_ERROR_DEVICE:
        out << "BLOCK_IO_ERROR_DEVICE";
        break;
    case TernError::BLOCK_IO_ERROR_FILE:
        out << "BLOCK_IO_ERROR_FILE";
        break;
    case TernError::INVALID_REPLICA:
        out << "INVALID_REPLICA";
        break;
    case TernError::DIFFERENT_ADDRS_INFO:
        out << "DIFFERENT_ADDRS_INFO";
        break;
    case TernError::LEADER_PREEMPTED:
        out << "LEADER_PREEMPTED";
        break;
    case TernError::LOG_ENTRY_MISSING:
        out << "LOG_ENTRY_MISSING";
        break;
    case TernError::LOG_ENTRY_TRIMMED:
        out << "LOG_ENTRY_TRIMMED";
        break;
    case TernError::LOG_ENTRY_UNRELEASED:
        out << "LOG_ENTRY_UNRELEASED";
        break;
    case TernError::LOG_ENTRY_RELEASED:
        out << "LOG_ENTRY_RELEASED";
        break;
    case TernError::AUTO_DECOMMISSION_FORBIDDEN:
        out << "AUTO_DECOMMISSION_FORBIDDEN";
        break;
    case TernError::INCONSISTENT_BLOCK_SERVICE_REGISTRATION:
        out << "INCONSISTENT_BLOCK_SERVICE_REGISTRATION";
        break;
    case TernError::SWAP_BLOCKS_INLINE_STORAGE:
        out << "SWAP_BLOCKS_INLINE_STORAGE";
        break;
    case TernError::SWAP_BLOCKS_MISMATCHING_SIZE:
        out << "SWAP_BLOCKS_MISMATCHING_SIZE";
        break;
    case TernError::SWAP_BLOCKS_MISMATCHING_STATE:
        out << "SWAP_BLOCKS_MISMATCHING_STATE";
        break;
    case TernError::SWAP_BLOCKS_MISMATCHING_CRC:
        out << "SWAP_BLOCKS_MISMATCHING_CRC";
        break;
    case TernError::SWAP_BLOCKS_DUPLICATE_BLOCK_SERVICE:
        out << "SWAP_BLOCKS_DUPLICATE_BLOCK_SERVICE";
        break;
    case TernError::SWAP_SPANS_INLINE_STORAGE:
        out << "SWAP_SPANS_INLINE_STORAGE";
        break;
    case TernError::SWAP_SPANS_MISMATCHING_SIZE:
        out << "SWAP_SPANS_MISMATCHING_SIZE";
        break;
    case TernError::SWAP_SPANS_NOT_CLEAN:
        out << "SWAP_SPANS_NOT_CLEAN";
        break;
    case TernError::SWAP_SPANS_MISMATCHING_CRC:
        out << "SWAP_SPANS_MISMATCHING_CRC";
        break;
    case TernError::SWAP_SPANS_MISMATCHING_BLOCKS:
        out << "SWAP_SPANS_MISMATCHING_BLOCKS";
        break;
    case TernError::EDGE_NOT_OWNED:
        out << "EDGE_NOT_OWNED";
        break;
    case TernError::CANNOT_CREATE_DB_SNAPSHOT:
        out << "CANNOT_CREATE_DB_SNAPSHOT";
        break;
    case TernError::BLOCK_SIZE_NOT_MULTIPLE_OF_PAGE_SIZE:
        out << "BLOCK_SIZE_NOT_MULTIPLE_OF_PAGE_SIZE";
        break;
    case TernError::SWAP_BLOCKS_DUPLICATE_FAILURE_DOMAIN:
        out << "SWAP_BLOCKS_DUPLICATE_FAILURE_DOMAIN";
        break;
    case TernError::TRANSIENT_LOCATION_COUNT:
        out << "TRANSIENT_LOCATION_COUNT";
        break;
    case TernError::ADD_SPAN_LOCATION_INLINE_STORAGE:
        out << "ADD_SPAN_LOCATION_INLINE_STORAGE";
        break;
    case TernError::ADD_SPAN_LOCATION_MISMATCHING_SIZE:
        out << "ADD_SPAN_LOCATION_MISMATCHING_SIZE";
        break;
    case TernError::ADD_SPAN_LOCATION_NOT_CLEAN:
        out << "ADD_SPAN_LOCATION_NOT_CLEAN";
        break;
    case TernError::ADD_SPAN_LOCATION_MISMATCHING_CRC:
        out << "ADD_SPAN_LOCATION_MISMATCHING_CRC";
        break;
    case TernError::ADD_SPAN_LOCATION_EXISTS:
        out << "ADD_SPAN_LOCATION_EXISTS";
        break;
    case TernError::SWAP_BLOCKS_MISMATCHING_LOCATION:
        out << "SWAP_BLOCKS_MISMATCHING_LOCATION";
        break;
    case TernError::LOCATION_EXISTS:
        out << "LOCATION_EXISTS";
        break;
    case TernError::LOCATION_NOT_FOUND:
        out << "LOCATION_NOT_FOUND";
        break;
    default:
        out << "TernError(" << ((int)err) << ")";
        break;
    }
    return out;
}

std::ostream& operator<<(std::ostream& out, ShardMessageKind kind) {
    switch (kind) {
    case ShardMessageKind::ERROR:
        out << "ERROR";
        break;
    case ShardMessageKind::LOOKUP:
        out << "LOOKUP";
        break;
    case ShardMessageKind::STAT_FILE:
        out << "STAT_FILE";
        break;
    case ShardMessageKind::STAT_DIRECTORY:
        out << "STAT_DIRECTORY";
        break;
    case ShardMessageKind::READ_DIR:
        out << "READ_DIR";
        break;
    case ShardMessageKind::CONSTRUCT_FILE:
        out << "CONSTRUCT_FILE";
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE:
        out << "ADD_SPAN_INITIATE";
        break;
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        out << "ADD_SPAN_CERTIFY";
        break;
    case ShardMessageKind::LINK_FILE:
        out << "LINK_FILE";
        break;
    case ShardMessageKind::SOFT_UNLINK_FILE:
        out << "SOFT_UNLINK_FILE";
        break;
    case ShardMessageKind::LOCAL_FILE_SPANS:
        out << "LOCAL_FILE_SPANS";
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        out << "SAME_DIRECTORY_RENAME";
        break;
    case ShardMessageKind::ADD_INLINE_SPAN:
        out << "ADD_INLINE_SPAN";
        break;
    case ShardMessageKind::SET_TIME:
        out << "SET_TIME";
        break;
    case ShardMessageKind::FULL_READ_DIR:
        out << "FULL_READ_DIR";
        break;
    case ShardMessageKind::MOVE_SPAN:
        out << "MOVE_SPAN";
        break;
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        out << "REMOVE_NON_OWNED_EDGE";
        break;
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        out << "SAME_SHARD_HARD_FILE_UNLINK";
        break;
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        out << "STAT_TRANSIENT_FILE";
        break;
    case ShardMessageKind::SHARD_SNAPSHOT:
        out << "SHARD_SNAPSHOT";
        break;
    case ShardMessageKind::FILE_SPANS:
        out << "FILE_SPANS";
        break;
    case ShardMessageKind::ADD_SPAN_LOCATION:
        out << "ADD_SPAN_LOCATION";
        break;
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        out << "SCRAP_TRANSIENT_FILE";
        break;
    case ShardMessageKind::SET_DIRECTORY_INFO:
        out << "SET_DIRECTORY_INFO";
        break;
    case ShardMessageKind::VISIT_DIRECTORIES:
        out << "VISIT_DIRECTORIES";
        break;
    case ShardMessageKind::VISIT_FILES:
        out << "VISIT_FILES";
        break;
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        out << "VISIT_TRANSIENT_FILES";
        break;
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        out << "REMOVE_SPAN_INITIATE";
        break;
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        out << "REMOVE_SPAN_CERTIFY";
        break;
    case ShardMessageKind::SWAP_BLOCKS:
        out << "SWAP_BLOCKS";
        break;
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        out << "BLOCK_SERVICE_FILES";
        break;
    case ShardMessageKind::REMOVE_INODE:
        out << "REMOVE_INODE";
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        out << "ADD_SPAN_INITIATE_WITH_REFERENCE";
        break;
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        out << "REMOVE_ZERO_BLOCK_SERVICE_FILES";
        break;
    case ShardMessageKind::SWAP_SPANS:
        out << "SWAP_SPANS";
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        out << "SAME_DIRECTORY_RENAME_SNAPSHOT";
        break;
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        out << "ADD_SPAN_AT_LOCATION_INITIATE";
        break;
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        out << "CREATE_DIRECTORY_INODE";
        break;
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        out << "SET_DIRECTORY_OWNER";
        break;
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        out << "REMOVE_DIRECTORY_OWNER";
        break;
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        out << "CREATE_LOCKED_CURRENT_EDGE";
        break;
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        out << "LOCK_CURRENT_EDGE";
        break;
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        out << "UNLOCK_CURRENT_EDGE";
        break;
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        out << "REMOVE_OWNED_SNAPSHOT_FILE_EDGE";
        break;
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        out << "MAKE_FILE_TRANSIENT";
        break;
    case ShardMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        out << "ShardMessageKind(" << ((int)kind) << ")";
        break;
    }
    return out;
}

std::ostream& operator<<(std::ostream& out, CDCMessageKind kind) {
    switch (kind) {
    case CDCMessageKind::ERROR:
        out << "ERROR";
        break;
    case CDCMessageKind::MAKE_DIRECTORY:
        out << "MAKE_DIRECTORY";
        break;
    case CDCMessageKind::RENAME_FILE:
        out << "RENAME_FILE";
        break;
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        out << "SOFT_UNLINK_DIRECTORY";
        break;
    case CDCMessageKind::RENAME_DIRECTORY:
        out << "RENAME_DIRECTORY";
        break;
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        out << "HARD_UNLINK_DIRECTORY";
        break;
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        out << "CROSS_SHARD_HARD_UNLINK_FILE";
        break;
    case CDCMessageKind::CDC_SNAPSHOT:
        out << "CDC_SNAPSHOT";
        break;
    case CDCMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        out << "CDCMessageKind(" << ((int)kind) << ")";
        break;
    }
    return out;
}

std::ostream& operator<<(std::ostream& out, RegistryMessageKind kind) {
    switch (kind) {
    case RegistryMessageKind::ERROR:
        out << "ERROR";
        break;
    case RegistryMessageKind::LOCAL_SHARDS:
        out << "LOCAL_SHARDS";
        break;
    case RegistryMessageKind::LOCAL_CDC:
        out << "LOCAL_CDC";
        break;
    case RegistryMessageKind::INFO:
        out << "INFO";
        break;
    case RegistryMessageKind::REGISTRY:
        out << "REGISTRY";
        break;
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        out << "LOCAL_CHANGED_BLOCK_SERVICES";
        break;
    case RegistryMessageKind::CREATE_LOCATION:
        out << "CREATE_LOCATION";
        break;
    case RegistryMessageKind::RENAME_LOCATION:
        out << "RENAME_LOCATION";
        break;
    case RegistryMessageKind::REGISTER_SHARD:
        out << "REGISTER_SHARD";
        break;
    case RegistryMessageKind::LOCATIONS:
        out << "LOCATIONS";
        break;
    case RegistryMessageKind::REGISTER_CDC:
        out << "REGISTER_CDC";
        break;
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        out << "SET_BLOCK_SERVICE_FLAGS";
        break;
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        out << "REGISTER_BLOCK_SERVICES";
        break;
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        out << "CHANGED_BLOCK_SERVICES_AT_LOCATION";
        break;
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        out << "SHARDS_AT_LOCATION";
        break;
    case RegistryMessageKind::CDC_AT_LOCATION:
        out << "CDC_AT_LOCATION";
        break;
    case RegistryMessageKind::REGISTER_REGISTRY:
        out << "REGISTER_REGISTRY";
        break;
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        out << "ALL_REGISTRY_REPLICAS";
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        out << "SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED";
        break;
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        out << "CDC_REPLICAS_DE_PR_EC_AT_ED";
        break;
    case RegistryMessageKind::ALL_SHARDS:
        out << "ALL_SHARDS";
        break;
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        out << "DECOMMISSION_BLOCK_SERVICE";
        break;
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        out << "MOVE_SHARD_LEADER";
        break;
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        out << "CLEAR_SHARD_INFO";
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        out << "SHARD_BLOCK_SERVICES";
        break;
    case RegistryMessageKind::ALL_CDC:
        out << "ALL_CDC";
        break;
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        out << "ERASE_DECOMMISSIONED_BLOCK";
        break;
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        out << "ALL_BLOCK_SERVICES_DEPRECATED";
        break;
    case RegistryMessageKind::MOVE_CDC_LEADER:
        out << "MOVE_CDC_LEADER";
        break;
    case RegistryMessageKind::CLEAR_CDC_INFO:
        out << "CLEAR_CDC_INFO";
        break;
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        out << "UPDATE_BLOCK_SERVICE_PATH";
        break;
    case RegistryMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        out << "RegistryMessageKind(" << ((int)kind) << ")";
        break;
    }
    return out;
}

std::ostream& operator<<(std::ostream& out, BlocksMessageKind kind) {
    switch (kind) {
    case BlocksMessageKind::ERROR:
        out << "ERROR";
        break;
    case BlocksMessageKind::FETCH_BLOCK:
        out << "FETCH_BLOCK";
        break;
    case BlocksMessageKind::WRITE_BLOCK:
        out << "WRITE_BLOCK";
        break;
    case BlocksMessageKind::FETCH_BLOCK_WITH_CRC:
        out << "FETCH_BLOCK_WITH_CRC";
        break;
    case BlocksMessageKind::ERASE_BLOCK:
        out << "ERASE_BLOCK";
        break;
    case BlocksMessageKind::TEST_WRITE:
        out << "TEST_WRITE";
        break;
    case BlocksMessageKind::CHECK_BLOCK:
        out << "CHECK_BLOCK";
        break;
    case BlocksMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        out << "BlocksMessageKind(" << ((int)kind) << ")";
        break;
    }
    return out;
}

std::ostream& operator<<(std::ostream& out, LogMessageKind kind) {
    switch (kind) {
    case LogMessageKind::ERROR:
        out << "ERROR";
        break;
    case LogMessageKind::LOG_WRITE:
        out << "LOG_WRITE";
        break;
    case LogMessageKind::RELEASE:
        out << "RELEASE";
        break;
    case LogMessageKind::LOG_READ:
        out << "LOG_READ";
        break;
    case LogMessageKind::NEW_LEADER:
        out << "NEW_LEADER";
        break;
    case LogMessageKind::NEW_LEADER_CONFIRM:
        out << "NEW_LEADER_CONFIRM";
        break;
    case LogMessageKind::LOG_RECOVERY_READ:
        out << "LOG_RECOVERY_READ";
        break;
    case LogMessageKind::LOG_RECOVERY_WRITE:
        out << "LOG_RECOVERY_WRITE";
        break;
    case LogMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        out << "LogMessageKind(" << ((int)kind) << ")";
        break;
    }
    return out;
}

void FailureDomain::pack(BincodeBuf& buf) const {
    buf.packFixedBytes<16>(name);
}
void FailureDomain::unpack(BincodeBuf& buf) {
    buf.unpackFixedBytes<16>(name);
}
void FailureDomain::clear() {
    name.clear();
}
bool FailureDomain::operator==(const FailureDomain& rhs) const {
    if (name != rhs.name) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FailureDomain& x) {
    out << "FailureDomain(" << "Name=" << x.name << ")";
    return out;
}

void DirectoryInfoEntry::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(tag);
    buf.packBytes(body);
}
void DirectoryInfoEntry::unpack(BincodeBuf& buf) {
    tag = buf.unpackScalar<uint8_t>();
    buf.unpackBytes(body);
}
void DirectoryInfoEntry::clear() {
    tag = uint8_t(0);
    body.clear();
}
bool DirectoryInfoEntry::operator==(const DirectoryInfoEntry& rhs) const {
    if ((uint8_t)this->tag != (uint8_t)rhs.tag) { return false; };
    if (body != rhs.body) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const DirectoryInfoEntry& x) {
    out << "DirectoryInfoEntry(" << "Tag=" << (int)x.tag << ", " << "Body=" << x.body << ")";
    return out;
}

void DirectoryInfo::pack(BincodeBuf& buf) const {
    buf.packList<DirectoryInfoEntry>(entries);
}
void DirectoryInfo::unpack(BincodeBuf& buf) {
    buf.unpackList<DirectoryInfoEntry>(entries);
}
void DirectoryInfo::clear() {
    entries.clear();
}
bool DirectoryInfo::operator==(const DirectoryInfo& rhs) const {
    if (entries != rhs.entries) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const DirectoryInfo& x) {
    out << "DirectoryInfo(" << "Entries=" << x.entries << ")";
    return out;
}

void CurrentEdge::pack(BincodeBuf& buf) const {
    targetId.pack(buf);
    buf.packScalar<uint64_t>(nameHash);
    buf.packBytes(name);
    creationTime.pack(buf);
}
void CurrentEdge::unpack(BincodeBuf& buf) {
    targetId.unpack(buf);
    nameHash = buf.unpackScalar<uint64_t>();
    buf.unpackBytes(name);
    creationTime.unpack(buf);
}
void CurrentEdge::clear() {
    targetId = InodeId();
    nameHash = uint64_t(0);
    name.clear();
    creationTime = TernTime();
}
bool CurrentEdge::operator==(const CurrentEdge& rhs) const {
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((uint64_t)this->nameHash != (uint64_t)rhs.nameHash) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CurrentEdge& x) {
    out << "CurrentEdge(" << "TargetId=" << x.targetId << ", " << "NameHash=" << x.nameHash << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void AddSpanInitiateBlockInfo::pack(BincodeBuf& buf) const {
    blockServiceAddrs.pack(buf);
    blockServiceId.pack(buf);
    blockServiceFailureDomain.pack(buf);
    buf.packScalar<uint64_t>(blockId);
    buf.packFixedBytes<8>(certificate);
}
void AddSpanInitiateBlockInfo::unpack(BincodeBuf& buf) {
    blockServiceAddrs.unpack(buf);
    blockServiceId.unpack(buf);
    blockServiceFailureDomain.unpack(buf);
    blockId = buf.unpackScalar<uint64_t>();
    buf.unpackFixedBytes<8>(certificate);
}
void AddSpanInitiateBlockInfo::clear() {
    blockServiceAddrs.clear();
    blockServiceId = BlockServiceId(0);
    blockServiceFailureDomain.clear();
    blockId = uint64_t(0);
    certificate.clear();
}
bool AddSpanInitiateBlockInfo::operator==(const AddSpanInitiateBlockInfo& rhs) const {
    if (blockServiceAddrs != rhs.blockServiceAddrs) { return false; };
    if ((BlockServiceId)this->blockServiceId != (BlockServiceId)rhs.blockServiceId) { return false; };
    if (blockServiceFailureDomain != rhs.blockServiceFailureDomain) { return false; };
    if ((uint64_t)this->blockId != (uint64_t)rhs.blockId) { return false; };
    if (certificate != rhs.certificate) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanInitiateBlockInfo& x) {
    out << "AddSpanInitiateBlockInfo(" << "BlockServiceAddrs=" << x.blockServiceAddrs << ", " << "BlockServiceId=" << x.blockServiceId << ", " << "BlockServiceFailureDomain=" << x.blockServiceFailureDomain << ", " << "BlockId=" << x.blockId << ", " << "Certificate=" << x.certificate << ")";
    return out;
}

void RemoveSpanInitiateBlockInfo::pack(BincodeBuf& buf) const {
    blockServiceAddrs.pack(buf);
    blockServiceId.pack(buf);
    blockServiceFailureDomain.pack(buf);
    buf.packScalar<BlockServiceFlags>(blockServiceFlags);
    buf.packScalar<uint64_t>(blockId);
    buf.packFixedBytes<8>(certificate);
}
void RemoveSpanInitiateBlockInfo::unpack(BincodeBuf& buf) {
    blockServiceAddrs.unpack(buf);
    blockServiceId.unpack(buf);
    blockServiceFailureDomain.unpack(buf);
    blockServiceFlags = buf.unpackScalar<BlockServiceFlags>();
    blockId = buf.unpackScalar<uint64_t>();
    buf.unpackFixedBytes<8>(certificate);
}
void RemoveSpanInitiateBlockInfo::clear() {
    blockServiceAddrs.clear();
    blockServiceId = BlockServiceId(0);
    blockServiceFailureDomain.clear();
    blockServiceFlags = BlockServiceFlags(0);
    blockId = uint64_t(0);
    certificate.clear();
}
bool RemoveSpanInitiateBlockInfo::operator==(const RemoveSpanInitiateBlockInfo& rhs) const {
    if (blockServiceAddrs != rhs.blockServiceAddrs) { return false; };
    if ((BlockServiceId)this->blockServiceId != (BlockServiceId)rhs.blockServiceId) { return false; };
    if (blockServiceFailureDomain != rhs.blockServiceFailureDomain) { return false; };
    if ((BlockServiceFlags)this->blockServiceFlags != (BlockServiceFlags)rhs.blockServiceFlags) { return false; };
    if ((uint64_t)this->blockId != (uint64_t)rhs.blockId) { return false; };
    if (certificate != rhs.certificate) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveSpanInitiateBlockInfo& x) {
    out << "RemoveSpanInitiateBlockInfo(" << "BlockServiceAddrs=" << x.blockServiceAddrs << ", " << "BlockServiceId=" << x.blockServiceId << ", " << "BlockServiceFailureDomain=" << x.blockServiceFailureDomain << ", " << "BlockServiceFlags=" << x.blockServiceFlags << ", " << "BlockId=" << x.blockId << ", " << "Certificate=" << x.certificate << ")";
    return out;
}

void BlockProof::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(blockId);
    buf.packFixedBytes<8>(proof);
}
void BlockProof::unpack(BincodeBuf& buf) {
    blockId = buf.unpackScalar<uint64_t>();
    buf.unpackFixedBytes<8>(proof);
}
void BlockProof::clear() {
    blockId = uint64_t(0);
    proof.clear();
}
bool BlockProof::operator==(const BlockProof& rhs) const {
    if ((uint64_t)this->blockId != (uint64_t)rhs.blockId) { return false; };
    if (proof != rhs.proof) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const BlockProof& x) {
    out << "BlockProof(" << "BlockId=" << x.blockId << ", " << "Proof=" << x.proof << ")";
    return out;
}

void BlockService::pack(BincodeBuf& buf) const {
    addrs.pack(buf);
    id.pack(buf);
    buf.packScalar<BlockServiceFlags>(flags);
}
void BlockService::unpack(BincodeBuf& buf) {
    addrs.unpack(buf);
    id.unpack(buf);
    flags = buf.unpackScalar<BlockServiceFlags>();
}
void BlockService::clear() {
    addrs.clear();
    id = BlockServiceId(0);
    flags = BlockServiceFlags(0);
}
bool BlockService::operator==(const BlockService& rhs) const {
    if (addrs != rhs.addrs) { return false; };
    if ((BlockServiceId)this->id != (BlockServiceId)rhs.id) { return false; };
    if ((BlockServiceFlags)this->flags != (BlockServiceFlags)rhs.flags) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const BlockService& x) {
    out << "BlockService(" << "Addrs=" << x.addrs << ", " << "Id=" << x.id << ", " << "Flags=" << x.flags << ")";
    return out;
}

void ShardInfo::pack(BincodeBuf& buf) const {
    addrs.pack(buf);
    lastSeen.pack(buf);
}
void ShardInfo::unpack(BincodeBuf& buf) {
    addrs.unpack(buf);
    lastSeen.unpack(buf);
}
void ShardInfo::clear() {
    addrs.clear();
    lastSeen = TernTime();
}
bool ShardInfo::operator==(const ShardInfo& rhs) const {
    if (addrs != rhs.addrs) { return false; };
    if ((TernTime)this->lastSeen != (TernTime)rhs.lastSeen) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ShardInfo& x) {
    out << "ShardInfo(" << "Addrs=" << x.addrs << ", " << "LastSeen=" << x.lastSeen << ")";
    return out;
}

void BlockPolicyEntry::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(storageClass);
    buf.packScalar<uint32_t>(minSize);
}
void BlockPolicyEntry::unpack(BincodeBuf& buf) {
    storageClass = buf.unpackScalar<uint8_t>();
    minSize = buf.unpackScalar<uint32_t>();
}
void BlockPolicyEntry::clear() {
    storageClass = uint8_t(0);
    minSize = uint32_t(0);
}
bool BlockPolicyEntry::operator==(const BlockPolicyEntry& rhs) const {
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    if ((uint32_t)this->minSize != (uint32_t)rhs.minSize) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const BlockPolicyEntry& x) {
    out << "BlockPolicyEntry(" << "StorageClass=" << (int)x.storageClass << ", " << "MinSize=" << x.minSize << ")";
    return out;
}

void SpanPolicyEntry::pack(BincodeBuf& buf) const {
    buf.packScalar<uint32_t>(maxSize);
    parity.pack(buf);
}
void SpanPolicyEntry::unpack(BincodeBuf& buf) {
    maxSize = buf.unpackScalar<uint32_t>();
    parity.unpack(buf);
}
void SpanPolicyEntry::clear() {
    maxSize = uint32_t(0);
    parity = Parity();
}
bool SpanPolicyEntry::operator==(const SpanPolicyEntry& rhs) const {
    if ((uint32_t)this->maxSize != (uint32_t)rhs.maxSize) { return false; };
    if ((Parity)this->parity != (Parity)rhs.parity) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SpanPolicyEntry& x) {
    out << "SpanPolicyEntry(" << "MaxSize=" << x.maxSize << ", " << "Parity=" << x.parity << ")";
    return out;
}

void StripePolicy::pack(BincodeBuf& buf) const {
    buf.packScalar<uint32_t>(targetStripeSize);
}
void StripePolicy::unpack(BincodeBuf& buf) {
    targetStripeSize = buf.unpackScalar<uint32_t>();
}
void StripePolicy::clear() {
    targetStripeSize = uint32_t(0);
}
bool StripePolicy::operator==(const StripePolicy& rhs) const {
    if ((uint32_t)this->targetStripeSize != (uint32_t)rhs.targetStripeSize) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const StripePolicy& x) {
    out << "StripePolicy(" << "TargetStripeSize=" << x.targetStripeSize << ")";
    return out;
}

void FetchedBlock::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(blockServiceIx);
    buf.packScalar<uint64_t>(blockId);
    crc.pack(buf);
}
void FetchedBlock::unpack(BincodeBuf& buf) {
    blockServiceIx = buf.unpackScalar<uint8_t>();
    blockId = buf.unpackScalar<uint64_t>();
    crc.unpack(buf);
}
void FetchedBlock::clear() {
    blockServiceIx = uint8_t(0);
    blockId = uint64_t(0);
    crc = Crc(0);
}
bool FetchedBlock::operator==(const FetchedBlock& rhs) const {
    if ((uint8_t)this->blockServiceIx != (uint8_t)rhs.blockServiceIx) { return false; };
    if ((uint64_t)this->blockId != (uint64_t)rhs.blockId) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchedBlock& x) {
    out << "FetchedBlock(" << "BlockServiceIx=" << (int)x.blockServiceIx << ", " << "BlockId=" << x.blockId << ", " << "Crc=" << x.crc << ")";
    return out;
}

void FetchedSpanHeader::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(byteOffset);
    buf.packScalar<uint32_t>(size);
    crc.pack(buf);
    buf.packScalar<uint8_t>(storageClass);
}
void FetchedSpanHeader::unpack(BincodeBuf& buf) {
    byteOffset = buf.unpackScalar<uint64_t>();
    size = buf.unpackScalar<uint32_t>();
    crc.unpack(buf);
    storageClass = buf.unpackScalar<uint8_t>();
}
void FetchedSpanHeader::clear() {
    byteOffset = uint64_t(0);
    size = uint32_t(0);
    crc = Crc(0);
    storageClass = uint8_t(0);
}
bool FetchedSpanHeader::operator==(const FetchedSpanHeader& rhs) const {
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if ((uint32_t)this->size != (uint32_t)rhs.size) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchedSpanHeader& x) {
    out << "FetchedSpanHeader(" << "ByteOffset=" << x.byteOffset << ", " << "Size=" << x.size << ", " << "Crc=" << x.crc << ", " << "StorageClass=" << (int)x.storageClass << ")";
    return out;
}

void FetchedInlineSpan::pack(BincodeBuf& buf) const {
    buf.packBytes(body);
}
void FetchedInlineSpan::unpack(BincodeBuf& buf) {
    buf.unpackBytes(body);
}
void FetchedInlineSpan::clear() {
    body.clear();
}
bool FetchedInlineSpan::operator==(const FetchedInlineSpan& rhs) const {
    if (body != rhs.body) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchedInlineSpan& x) {
    out << "FetchedInlineSpan(" << "Body=" << x.body << ")";
    return out;
}

void FetchedBlocksSpan::pack(BincodeBuf& buf) const {
    parity.pack(buf);
    buf.packScalar<uint8_t>(stripes);
    buf.packScalar<uint32_t>(cellSize);
    buf.packList<FetchedBlock>(blocks);
    buf.packList<Crc>(stripesCrc);
}
void FetchedBlocksSpan::unpack(BincodeBuf& buf) {
    parity.unpack(buf);
    stripes = buf.unpackScalar<uint8_t>();
    cellSize = buf.unpackScalar<uint32_t>();
    buf.unpackList<FetchedBlock>(blocks);
    buf.unpackList<Crc>(stripesCrc);
}
void FetchedBlocksSpan::clear() {
    parity = Parity();
    stripes = uint8_t(0);
    cellSize = uint32_t(0);
    blocks.clear();
    stripesCrc.clear();
}
bool FetchedBlocksSpan::operator==(const FetchedBlocksSpan& rhs) const {
    if ((Parity)this->parity != (Parity)rhs.parity) { return false; };
    if ((uint8_t)this->stripes != (uint8_t)rhs.stripes) { return false; };
    if ((uint32_t)this->cellSize != (uint32_t)rhs.cellSize) { return false; };
    if (blocks != rhs.blocks) { return false; };
    if (stripesCrc != rhs.stripesCrc) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchedBlocksSpan& x) {
    out << "FetchedBlocksSpan(" << "Parity=" << x.parity << ", " << "Stripes=" << (int)x.stripes << ", " << "CellSize=" << x.cellSize << ", " << "Blocks=" << x.blocks << ", " << "StripesCrc=" << x.stripesCrc << ")";
    return out;
}

void FetchedBlockServices::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(locationId);
    buf.packScalar<uint8_t>(storageClass);
    parity.pack(buf);
    buf.packScalar<uint8_t>(stripes);
    buf.packScalar<uint32_t>(cellSize);
    buf.packList<FetchedBlock>(blocks);
    buf.packList<Crc>(stripesCrc);
}
void FetchedBlockServices::unpack(BincodeBuf& buf) {
    locationId = buf.unpackScalar<uint8_t>();
    storageClass = buf.unpackScalar<uint8_t>();
    parity.unpack(buf);
    stripes = buf.unpackScalar<uint8_t>();
    cellSize = buf.unpackScalar<uint32_t>();
    buf.unpackList<FetchedBlock>(blocks);
    buf.unpackList<Crc>(stripesCrc);
}
void FetchedBlockServices::clear() {
    locationId = uint8_t(0);
    storageClass = uint8_t(0);
    parity = Parity();
    stripes = uint8_t(0);
    cellSize = uint32_t(0);
    blocks.clear();
    stripesCrc.clear();
}
bool FetchedBlockServices::operator==(const FetchedBlockServices& rhs) const {
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    if ((Parity)this->parity != (Parity)rhs.parity) { return false; };
    if ((uint8_t)this->stripes != (uint8_t)rhs.stripes) { return false; };
    if ((uint32_t)this->cellSize != (uint32_t)rhs.cellSize) { return false; };
    if (blocks != rhs.blocks) { return false; };
    if (stripesCrc != rhs.stripesCrc) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchedBlockServices& x) {
    out << "FetchedBlockServices(" << "LocationId=" << (int)x.locationId << ", " << "StorageClass=" << (int)x.storageClass << ", " << "Parity=" << x.parity << ", " << "Stripes=" << (int)x.stripes << ", " << "CellSize=" << x.cellSize << ", " << "Blocks=" << x.blocks << ", " << "StripesCrc=" << x.stripesCrc << ")";
    return out;
}

void FetchedLocations::pack(BincodeBuf& buf) const {
    buf.packList<FetchedBlockServices>(locations);
}
void FetchedLocations::unpack(BincodeBuf& buf) {
    buf.unpackList<FetchedBlockServices>(locations);
}
void FetchedLocations::clear() {
    locations.clear();
}
bool FetchedLocations::operator==(const FetchedLocations& rhs) const {
    if (locations != rhs.locations) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchedLocations& x) {
    out << "FetchedLocations(" << "Locations=" << x.locations << ")";
    return out;
}

void FetchedSpanHeaderFull::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(byteOffset);
    buf.packScalar<uint32_t>(size);
    crc.pack(buf);
    buf.packScalar<bool>(isInline);
}
void FetchedSpanHeaderFull::unpack(BincodeBuf& buf) {
    byteOffset = buf.unpackScalar<uint64_t>();
    size = buf.unpackScalar<uint32_t>();
    crc.unpack(buf);
    isInline = buf.unpackScalar<bool>();
}
void FetchedSpanHeaderFull::clear() {
    byteOffset = uint64_t(0);
    size = uint32_t(0);
    crc = Crc(0);
    isInline = bool(0);
}
bool FetchedSpanHeaderFull::operator==(const FetchedSpanHeaderFull& rhs) const {
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if ((uint32_t)this->size != (uint32_t)rhs.size) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    if ((bool)this->isInline != (bool)rhs.isInline) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchedSpanHeaderFull& x) {
    out << "FetchedSpanHeaderFull(" << "ByteOffset=" << x.byteOffset << ", " << "Size=" << x.size << ", " << "Crc=" << x.crc << ", " << "IsInline=" << x.isInline << ")";
    return out;
}

void BlacklistEntry::pack(BincodeBuf& buf) const {
    failureDomain.pack(buf);
    blockService.pack(buf);
}
void BlacklistEntry::unpack(BincodeBuf& buf) {
    failureDomain.unpack(buf);
    blockService.unpack(buf);
}
void BlacklistEntry::clear() {
    failureDomain.clear();
    blockService = BlockServiceId(0);
}
bool BlacklistEntry::operator==(const BlacklistEntry& rhs) const {
    if (failureDomain != rhs.failureDomain) { return false; };
    if ((BlockServiceId)this->blockService != (BlockServiceId)rhs.blockService) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const BlacklistEntry& x) {
    out << "BlacklistEntry(" << "FailureDomain=" << x.failureDomain << ", " << "BlockService=" << x.blockService << ")";
    return out;
}

void Edge::pack(BincodeBuf& buf) const {
    buf.packScalar<bool>(current);
    targetId.pack(buf);
    buf.packScalar<uint64_t>(nameHash);
    buf.packBytes(name);
    creationTime.pack(buf);
}
void Edge::unpack(BincodeBuf& buf) {
    current = buf.unpackScalar<bool>();
    targetId.unpack(buf);
    nameHash = buf.unpackScalar<uint64_t>();
    buf.unpackBytes(name);
    creationTime.unpack(buf);
}
void Edge::clear() {
    current = bool(0);
    targetId = InodeIdExtra();
    nameHash = uint64_t(0);
    name.clear();
    creationTime = TernTime();
}
bool Edge::operator==(const Edge& rhs) const {
    if ((bool)this->current != (bool)rhs.current) { return false; };
    if ((InodeIdExtra)this->targetId != (InodeIdExtra)rhs.targetId) { return false; };
    if ((uint64_t)this->nameHash != (uint64_t)rhs.nameHash) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const Edge& x) {
    out << "Edge(" << "Current=" << x.current << ", " << "TargetId=" << x.targetId << ", " << "NameHash=" << x.nameHash << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void FullReadDirCursor::pack(BincodeBuf& buf) const {
    buf.packScalar<bool>(current);
    buf.packBytes(startName);
    startTime.pack(buf);
}
void FullReadDirCursor::unpack(BincodeBuf& buf) {
    current = buf.unpackScalar<bool>();
    buf.unpackBytes(startName);
    startTime.unpack(buf);
}
void FullReadDirCursor::clear() {
    current = bool(0);
    startName.clear();
    startTime = TernTime();
}
bool FullReadDirCursor::operator==(const FullReadDirCursor& rhs) const {
    if ((bool)this->current != (bool)rhs.current) { return false; };
    if (startName != rhs.startName) { return false; };
    if ((TernTime)this->startTime != (TernTime)rhs.startTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FullReadDirCursor& x) {
    out << "FullReadDirCursor(" << "Current=" << x.current << ", " << "StartName=" << GoLangQuotedStringFmt(x.startName.data(), x.startName.size()) << ", " << "StartTime=" << x.startTime << ")";
    return out;
}

void FullRegistryInfo::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packScalar<uint8_t>(locationId);
    buf.packScalar<bool>(isLeader);
    addrs.pack(buf);
    lastSeen.pack(buf);
}
void FullRegistryInfo::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    locationId = buf.unpackScalar<uint8_t>();
    isLeader = buf.unpackScalar<bool>();
    addrs.unpack(buf);
    lastSeen.unpack(buf);
}
void FullRegistryInfo::clear() {
    id = ReplicaId();
    locationId = uint8_t(0);
    isLeader = bool(0);
    addrs.clear();
    lastSeen = TernTime();
}
bool FullRegistryInfo::operator==(const FullRegistryInfo& rhs) const {
    if ((ReplicaId)this->id != (ReplicaId)rhs.id) { return false; };
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    if ((bool)this->isLeader != (bool)rhs.isLeader) { return false; };
    if (addrs != rhs.addrs) { return false; };
    if ((TernTime)this->lastSeen != (TernTime)rhs.lastSeen) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FullRegistryInfo& x) {
    out << "FullRegistryInfo(" << "Id=" << x.id << ", " << "LocationId=" << (int)x.locationId << ", " << "IsLeader=" << x.isLeader << ", " << "Addrs=" << x.addrs << ", " << "LastSeen=" << x.lastSeen << ")";
    return out;
}

void TransientFile::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packFixedBytes<8>(cookie);
    deadlineTime.pack(buf);
}
void TransientFile::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    buf.unpackFixedBytes<8>(cookie);
    deadlineTime.unpack(buf);
}
void TransientFile::clear() {
    id = InodeId();
    cookie.clear();
    deadlineTime = TernTime();
}
bool TransientFile::operator==(const TransientFile& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if (cookie != rhs.cookie) { return false; };
    if ((TernTime)this->deadlineTime != (TernTime)rhs.deadlineTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const TransientFile& x) {
    out << "TransientFile(" << "Id=" << x.id << ", " << "Cookie=" << x.cookie << ", " << "DeadlineTime=" << x.deadlineTime << ")";
    return out;
}

void EntryNewBlockInfo::pack(BincodeBuf& buf) const {
    blockServiceId.pack(buf);
    crc.pack(buf);
}
void EntryNewBlockInfo::unpack(BincodeBuf& buf) {
    blockServiceId.unpack(buf);
    crc.unpack(buf);
}
void EntryNewBlockInfo::clear() {
    blockServiceId = BlockServiceId(0);
    crc = Crc(0);
}
bool EntryNewBlockInfo::operator==(const EntryNewBlockInfo& rhs) const {
    if ((BlockServiceId)this->blockServiceId != (BlockServiceId)rhs.blockServiceId) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const EntryNewBlockInfo& x) {
    out << "EntryNewBlockInfo(" << "BlockServiceId=" << x.blockServiceId << ", " << "Crc=" << x.crc << ")";
    return out;
}

void BlockServiceDeprecatedInfo::pack(BincodeBuf& buf) const {
    id.pack(buf);
    addrs.pack(buf);
    buf.packScalar<uint8_t>(storageClass);
    failureDomain.pack(buf);
    buf.packFixedBytes<16>(secretKey);
    buf.packScalar<BlockServiceFlags>(flags);
    buf.packScalar<uint64_t>(capacityBytes);
    buf.packScalar<uint64_t>(availableBytes);
    buf.packScalar<uint64_t>(blocks);
    buf.packBytes(path);
    lastSeen.pack(buf);
    buf.packScalar<bool>(hasFiles);
    flagsLastChanged.pack(buf);
}
void BlockServiceDeprecatedInfo::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    addrs.unpack(buf);
    storageClass = buf.unpackScalar<uint8_t>();
    failureDomain.unpack(buf);
    buf.unpackFixedBytes<16>(secretKey);
    flags = buf.unpackScalar<BlockServiceFlags>();
    capacityBytes = buf.unpackScalar<uint64_t>();
    availableBytes = buf.unpackScalar<uint64_t>();
    blocks = buf.unpackScalar<uint64_t>();
    buf.unpackBytes(path);
    lastSeen.unpack(buf);
    hasFiles = buf.unpackScalar<bool>();
    flagsLastChanged.unpack(buf);
}
void BlockServiceDeprecatedInfo::clear() {
    id = BlockServiceId(0);
    addrs.clear();
    storageClass = uint8_t(0);
    failureDomain.clear();
    secretKey.clear();
    flags = BlockServiceFlags(0);
    capacityBytes = uint64_t(0);
    availableBytes = uint64_t(0);
    blocks = uint64_t(0);
    path.clear();
    lastSeen = TernTime();
    hasFiles = bool(0);
    flagsLastChanged = TernTime();
}
bool BlockServiceDeprecatedInfo::operator==(const BlockServiceDeprecatedInfo& rhs) const {
    if ((BlockServiceId)this->id != (BlockServiceId)rhs.id) { return false; };
    if (addrs != rhs.addrs) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    if (failureDomain != rhs.failureDomain) { return false; };
    if (secretKey != rhs.secretKey) { return false; };
    if ((BlockServiceFlags)this->flags != (BlockServiceFlags)rhs.flags) { return false; };
    if ((uint64_t)this->capacityBytes != (uint64_t)rhs.capacityBytes) { return false; };
    if ((uint64_t)this->availableBytes != (uint64_t)rhs.availableBytes) { return false; };
    if ((uint64_t)this->blocks != (uint64_t)rhs.blocks) { return false; };
    if (path != rhs.path) { return false; };
    if ((TernTime)this->lastSeen != (TernTime)rhs.lastSeen) { return false; };
    if ((bool)this->hasFiles != (bool)rhs.hasFiles) { return false; };
    if ((TernTime)this->flagsLastChanged != (TernTime)rhs.flagsLastChanged) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const BlockServiceDeprecatedInfo& x) {
    out << "BlockServiceDeprecatedInfo(" << "Id=" << x.id << ", " << "Addrs=" << x.addrs << ", " << "StorageClass=" << (int)x.storageClass << ", " << "FailureDomain=" << x.failureDomain << ", " << "SecretKey=" << x.secretKey << ", " << "Flags=" << x.flags << ", " << "CapacityBytes=" << x.capacityBytes << ", " << "AvailableBytes=" << x.availableBytes << ", " << "Blocks=" << x.blocks << ", " << "Path=" << GoLangQuotedStringFmt(x.path.data(), x.path.size()) << ", " << "LastSeen=" << x.lastSeen << ", " << "HasFiles=" << x.hasFiles << ", " << "FlagsLastChanged=" << x.flagsLastChanged << ")";
    return out;
}

void BlockServiceInfoShort::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(locationId);
    failureDomain.pack(buf);
    id.pack(buf);
    buf.packScalar<uint8_t>(storageClass);
}
void BlockServiceInfoShort::unpack(BincodeBuf& buf) {
    locationId = buf.unpackScalar<uint8_t>();
    failureDomain.unpack(buf);
    id.unpack(buf);
    storageClass = buf.unpackScalar<uint8_t>();
}
void BlockServiceInfoShort::clear() {
    locationId = uint8_t(0);
    failureDomain.clear();
    id = BlockServiceId(0);
    storageClass = uint8_t(0);
}
bool BlockServiceInfoShort::operator==(const BlockServiceInfoShort& rhs) const {
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    if (failureDomain != rhs.failureDomain) { return false; };
    if ((BlockServiceId)this->id != (BlockServiceId)rhs.id) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const BlockServiceInfoShort& x) {
    out << "BlockServiceInfoShort(" << "LocationId=" << (int)x.locationId << ", " << "FailureDomain=" << x.failureDomain << ", " << "Id=" << x.id << ", " << "StorageClass=" << (int)x.storageClass << ")";
    return out;
}

void SpanPolicy::pack(BincodeBuf& buf) const {
    buf.packList<SpanPolicyEntry>(entries);
}
void SpanPolicy::unpack(BincodeBuf& buf) {
    buf.unpackList<SpanPolicyEntry>(entries);
}
void SpanPolicy::clear() {
    entries.clear();
}
bool SpanPolicy::operator==(const SpanPolicy& rhs) const {
    if (entries != rhs.entries) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SpanPolicy& x) {
    out << "SpanPolicy(" << "Entries=" << x.entries << ")";
    return out;
}

void BlockPolicy::pack(BincodeBuf& buf) const {
    buf.packList<BlockPolicyEntry>(entries);
}
void BlockPolicy::unpack(BincodeBuf& buf) {
    buf.unpackList<BlockPolicyEntry>(entries);
}
void BlockPolicy::clear() {
    entries.clear();
}
bool BlockPolicy::operator==(const BlockPolicy& rhs) const {
    if (entries != rhs.entries) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const BlockPolicy& x) {
    out << "BlockPolicy(" << "Entries=" << x.entries << ")";
    return out;
}

void SnapshotPolicy::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(deleteAfterTime);
    buf.packScalar<uint16_t>(deleteAfterVersions);
}
void SnapshotPolicy::unpack(BincodeBuf& buf) {
    deleteAfterTime = buf.unpackScalar<uint64_t>();
    deleteAfterVersions = buf.unpackScalar<uint16_t>();
}
void SnapshotPolicy::clear() {
    deleteAfterTime = uint64_t(0);
    deleteAfterVersions = uint16_t(0);
}
bool SnapshotPolicy::operator==(const SnapshotPolicy& rhs) const {
    if ((uint64_t)this->deleteAfterTime != (uint64_t)rhs.deleteAfterTime) { return false; };
    if ((uint16_t)this->deleteAfterVersions != (uint16_t)rhs.deleteAfterVersions) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SnapshotPolicy& x) {
    out << "SnapshotPolicy(" << "DeleteAfterTime=" << x.deleteAfterTime << ", " << "DeleteAfterVersions=" << x.deleteAfterVersions << ")";
    return out;
}

void FullShardInfo::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packScalar<bool>(isLeader);
    addrs.pack(buf);
    lastSeen.pack(buf);
    buf.packScalar<uint8_t>(locationId);
}
void FullShardInfo::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    isLeader = buf.unpackScalar<bool>();
    addrs.unpack(buf);
    lastSeen.unpack(buf);
    locationId = buf.unpackScalar<uint8_t>();
}
void FullShardInfo::clear() {
    id = ShardReplicaId();
    isLeader = bool(0);
    addrs.clear();
    lastSeen = TernTime();
    locationId = uint8_t(0);
}
bool FullShardInfo::operator==(const FullShardInfo& rhs) const {
    if ((ShardReplicaId)this->id != (ShardReplicaId)rhs.id) { return false; };
    if ((bool)this->isLeader != (bool)rhs.isLeader) { return false; };
    if (addrs != rhs.addrs) { return false; };
    if ((TernTime)this->lastSeen != (TernTime)rhs.lastSeen) { return false; };
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FullShardInfo& x) {
    out << "FullShardInfo(" << "Id=" << x.id << ", " << "IsLeader=" << x.isLeader << ", " << "Addrs=" << x.addrs << ", " << "LastSeen=" << x.lastSeen << ", " << "LocationId=" << (int)x.locationId << ")";
    return out;
}

void RegisterBlockServiceInfo::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packScalar<uint8_t>(locationId);
    addrs.pack(buf);
    buf.packScalar<uint8_t>(storageClass);
    failureDomain.pack(buf);
    buf.packFixedBytes<16>(secretKey);
    buf.packScalar<BlockServiceFlags>(flags);
    buf.packScalar<uint8_t>(flagsMask);
    buf.packScalar<uint64_t>(capacityBytes);
    buf.packScalar<uint64_t>(availableBytes);
    buf.packScalar<uint64_t>(blocks);
    buf.packBytes(path);
}
void RegisterBlockServiceInfo::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    locationId = buf.unpackScalar<uint8_t>();
    addrs.unpack(buf);
    storageClass = buf.unpackScalar<uint8_t>();
    failureDomain.unpack(buf);
    buf.unpackFixedBytes<16>(secretKey);
    flags = buf.unpackScalar<BlockServiceFlags>();
    flagsMask = buf.unpackScalar<uint8_t>();
    capacityBytes = buf.unpackScalar<uint64_t>();
    availableBytes = buf.unpackScalar<uint64_t>();
    blocks = buf.unpackScalar<uint64_t>();
    buf.unpackBytes(path);
}
void RegisterBlockServiceInfo::clear() {
    id = BlockServiceId(0);
    locationId = uint8_t(0);
    addrs.clear();
    storageClass = uint8_t(0);
    failureDomain.clear();
    secretKey.clear();
    flags = BlockServiceFlags(0);
    flagsMask = uint8_t(0);
    capacityBytes = uint64_t(0);
    availableBytes = uint64_t(0);
    blocks = uint64_t(0);
    path.clear();
}
bool RegisterBlockServiceInfo::operator==(const RegisterBlockServiceInfo& rhs) const {
    if ((BlockServiceId)this->id != (BlockServiceId)rhs.id) { return false; };
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    if (addrs != rhs.addrs) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    if (failureDomain != rhs.failureDomain) { return false; };
    if (secretKey != rhs.secretKey) { return false; };
    if ((BlockServiceFlags)this->flags != (BlockServiceFlags)rhs.flags) { return false; };
    if ((uint8_t)this->flagsMask != (uint8_t)rhs.flagsMask) { return false; };
    if ((uint64_t)this->capacityBytes != (uint64_t)rhs.capacityBytes) { return false; };
    if ((uint64_t)this->availableBytes != (uint64_t)rhs.availableBytes) { return false; };
    if ((uint64_t)this->blocks != (uint64_t)rhs.blocks) { return false; };
    if (path != rhs.path) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegisterBlockServiceInfo& x) {
    out << "RegisterBlockServiceInfo(" << "Id=" << x.id << ", " << "LocationId=" << (int)x.locationId << ", " << "Addrs=" << x.addrs << ", " << "StorageClass=" << (int)x.storageClass << ", " << "FailureDomain=" << x.failureDomain << ", " << "SecretKey=" << x.secretKey << ", " << "Flags=" << x.flags << ", " << "FlagsMask=" << (int)x.flagsMask << ", " << "CapacityBytes=" << x.capacityBytes << ", " << "AvailableBytes=" << x.availableBytes << ", " << "Blocks=" << x.blocks << ", " << "Path=" << GoLangQuotedStringFmt(x.path.data(), x.path.size()) << ")";
    return out;
}

void FullBlockServiceInfo::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packScalar<uint8_t>(locationId);
    addrs.pack(buf);
    buf.packScalar<uint8_t>(storageClass);
    failureDomain.pack(buf);
    buf.packFixedBytes<16>(secretKey);
    buf.packScalar<BlockServiceFlags>(flags);
    buf.packScalar<uint64_t>(capacityBytes);
    buf.packScalar<uint64_t>(availableBytes);
    buf.packScalar<uint64_t>(blocks);
    firstSeen.pack(buf);
    lastSeen.pack(buf);
    lastInfoChange.pack(buf);
    buf.packScalar<bool>(hasFiles);
    buf.packBytes(path);
}
void FullBlockServiceInfo::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    locationId = buf.unpackScalar<uint8_t>();
    addrs.unpack(buf);
    storageClass = buf.unpackScalar<uint8_t>();
    failureDomain.unpack(buf);
    buf.unpackFixedBytes<16>(secretKey);
    flags = buf.unpackScalar<BlockServiceFlags>();
    capacityBytes = buf.unpackScalar<uint64_t>();
    availableBytes = buf.unpackScalar<uint64_t>();
    blocks = buf.unpackScalar<uint64_t>();
    firstSeen.unpack(buf);
    lastSeen.unpack(buf);
    lastInfoChange.unpack(buf);
    hasFiles = buf.unpackScalar<bool>();
    buf.unpackBytes(path);
}
void FullBlockServiceInfo::clear() {
    id = BlockServiceId(0);
    locationId = uint8_t(0);
    addrs.clear();
    storageClass = uint8_t(0);
    failureDomain.clear();
    secretKey.clear();
    flags = BlockServiceFlags(0);
    capacityBytes = uint64_t(0);
    availableBytes = uint64_t(0);
    blocks = uint64_t(0);
    firstSeen = TernTime();
    lastSeen = TernTime();
    lastInfoChange = TernTime();
    hasFiles = bool(0);
    path.clear();
}
bool FullBlockServiceInfo::operator==(const FullBlockServiceInfo& rhs) const {
    if ((BlockServiceId)this->id != (BlockServiceId)rhs.id) { return false; };
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    if (addrs != rhs.addrs) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    if (failureDomain != rhs.failureDomain) { return false; };
    if (secretKey != rhs.secretKey) { return false; };
    if ((BlockServiceFlags)this->flags != (BlockServiceFlags)rhs.flags) { return false; };
    if ((uint64_t)this->capacityBytes != (uint64_t)rhs.capacityBytes) { return false; };
    if ((uint64_t)this->availableBytes != (uint64_t)rhs.availableBytes) { return false; };
    if ((uint64_t)this->blocks != (uint64_t)rhs.blocks) { return false; };
    if ((TernTime)this->firstSeen != (TernTime)rhs.firstSeen) { return false; };
    if ((TernTime)this->lastSeen != (TernTime)rhs.lastSeen) { return false; };
    if ((TernTime)this->lastInfoChange != (TernTime)rhs.lastInfoChange) { return false; };
    if ((bool)this->hasFiles != (bool)rhs.hasFiles) { return false; };
    if (path != rhs.path) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FullBlockServiceInfo& x) {
    out << "FullBlockServiceInfo(" << "Id=" << x.id << ", " << "LocationId=" << (int)x.locationId << ", " << "Addrs=" << x.addrs << ", " << "StorageClass=" << (int)x.storageClass << ", " << "FailureDomain=" << x.failureDomain << ", " << "SecretKey=" << x.secretKey << ", " << "Flags=" << x.flags << ", " << "CapacityBytes=" << x.capacityBytes << ", " << "AvailableBytes=" << x.availableBytes << ", " << "Blocks=" << x.blocks << ", " << "FirstSeen=" << x.firstSeen << ", " << "LastSeen=" << x.lastSeen << ", " << "LastInfoChange=" << x.lastInfoChange << ", " << "HasFiles=" << x.hasFiles << ", " << "Path=" << GoLangQuotedStringFmt(x.path.data(), x.path.size()) << ")";
    return out;
}

void CdcInfo::pack(BincodeBuf& buf) const {
    replicaId.pack(buf);
    buf.packScalar<uint8_t>(locationId);
    buf.packScalar<bool>(isLeader);
    addrs.pack(buf);
    lastSeen.pack(buf);
}
void CdcInfo::unpack(BincodeBuf& buf) {
    replicaId.unpack(buf);
    locationId = buf.unpackScalar<uint8_t>();
    isLeader = buf.unpackScalar<bool>();
    addrs.unpack(buf);
    lastSeen.unpack(buf);
}
void CdcInfo::clear() {
    replicaId = ReplicaId();
    locationId = uint8_t(0);
    isLeader = bool(0);
    addrs.clear();
    lastSeen = TernTime();
}
bool CdcInfo::operator==(const CdcInfo& rhs) const {
    if ((ReplicaId)this->replicaId != (ReplicaId)rhs.replicaId) { return false; };
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    if ((bool)this->isLeader != (bool)rhs.isLeader) { return false; };
    if (addrs != rhs.addrs) { return false; };
    if ((TernTime)this->lastSeen != (TernTime)rhs.lastSeen) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CdcInfo& x) {
    out << "CdcInfo(" << "ReplicaId=" << x.replicaId << ", " << "LocationId=" << (int)x.locationId << ", " << "IsLeader=" << x.isLeader << ", " << "Addrs=" << x.addrs << ", " << "LastSeen=" << x.lastSeen << ")";
    return out;
}

void LocationInfo::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(id);
    buf.packBytes(name);
}
void LocationInfo::unpack(BincodeBuf& buf) {
    id = buf.unpackScalar<uint8_t>();
    buf.unpackBytes(name);
}
void LocationInfo::clear() {
    id = uint8_t(0);
    name.clear();
}
bool LocationInfo::operator==(const LocationInfo& rhs) const {
    if ((uint8_t)this->id != (uint8_t)rhs.id) { return false; };
    if (name != rhs.name) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocationInfo& x) {
    out << "LocationInfo(" << "Id=" << (int)x.id << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ")";
    return out;
}

void LookupReq::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    buf.packBytes(name);
}
void LookupReq::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    buf.unpackBytes(name);
}
void LookupReq::clear() {
    dirId = InodeId();
    name.clear();
}
bool LookupReq::operator==(const LookupReq& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (name != rhs.name) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LookupReq& x) {
    out << "LookupReq(" << "DirId=" << x.dirId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ")";
    return out;
}

void LookupResp::pack(BincodeBuf& buf) const {
    targetId.pack(buf);
    creationTime.pack(buf);
}
void LookupResp::unpack(BincodeBuf& buf) {
    targetId.unpack(buf);
    creationTime.unpack(buf);
}
void LookupResp::clear() {
    targetId = InodeId();
    creationTime = TernTime();
}
bool LookupResp::operator==(const LookupResp& rhs) const {
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LookupResp& x) {
    out << "LookupResp(" << "TargetId=" << x.targetId << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void StatFileReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
}
void StatFileReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
}
void StatFileReq::clear() {
    id = InodeId();
}
bool StatFileReq::operator==(const StatFileReq& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const StatFileReq& x) {
    out << "StatFileReq(" << "Id=" << x.id << ")";
    return out;
}

void StatFileResp::pack(BincodeBuf& buf) const {
    mtime.pack(buf);
    atime.pack(buf);
    buf.packScalar<uint64_t>(size);
}
void StatFileResp::unpack(BincodeBuf& buf) {
    mtime.unpack(buf);
    atime.unpack(buf);
    size = buf.unpackScalar<uint64_t>();
}
void StatFileResp::clear() {
    mtime = TernTime();
    atime = TernTime();
    size = uint64_t(0);
}
bool StatFileResp::operator==(const StatFileResp& rhs) const {
    if ((TernTime)this->mtime != (TernTime)rhs.mtime) { return false; };
    if ((TernTime)this->atime != (TernTime)rhs.atime) { return false; };
    if ((uint64_t)this->size != (uint64_t)rhs.size) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const StatFileResp& x) {
    out << "StatFileResp(" << "Mtime=" << x.mtime << ", " << "Atime=" << x.atime << ", " << "Size=" << x.size << ")";
    return out;
}

void StatDirectoryReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
}
void StatDirectoryReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
}
void StatDirectoryReq::clear() {
    id = InodeId();
}
bool StatDirectoryReq::operator==(const StatDirectoryReq& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const StatDirectoryReq& x) {
    out << "StatDirectoryReq(" << "Id=" << x.id << ")";
    return out;
}

void StatDirectoryResp::pack(BincodeBuf& buf) const {
    mtime.pack(buf);
    owner.pack(buf);
    info.pack(buf);
}
void StatDirectoryResp::unpack(BincodeBuf& buf) {
    mtime.unpack(buf);
    owner.unpack(buf);
    info.unpack(buf);
}
void StatDirectoryResp::clear() {
    mtime = TernTime();
    owner = InodeId();
    info.clear();
}
bool StatDirectoryResp::operator==(const StatDirectoryResp& rhs) const {
    if ((TernTime)this->mtime != (TernTime)rhs.mtime) { return false; };
    if ((InodeId)this->owner != (InodeId)rhs.owner) { return false; };
    if (info != rhs.info) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const StatDirectoryResp& x) {
    out << "StatDirectoryResp(" << "Mtime=" << x.mtime << ", " << "Owner=" << x.owner << ", " << "Info=" << x.info << ")";
    return out;
}

void ReadDirReq::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    buf.packScalar<uint64_t>(startHash);
    buf.packScalar<uint16_t>(mtu);
}
void ReadDirReq::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    startHash = buf.unpackScalar<uint64_t>();
    mtu = buf.unpackScalar<uint16_t>();
}
void ReadDirReq::clear() {
    dirId = InodeId();
    startHash = uint64_t(0);
    mtu = uint16_t(0);
}
bool ReadDirReq::operator==(const ReadDirReq& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if ((uint64_t)this->startHash != (uint64_t)rhs.startHash) { return false; };
    if ((uint16_t)this->mtu != (uint16_t)rhs.mtu) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ReadDirReq& x) {
    out << "ReadDirReq(" << "DirId=" << x.dirId << ", " << "StartHash=" << x.startHash << ", " << "Mtu=" << x.mtu << ")";
    return out;
}

void ReadDirResp::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(nextHash);
    buf.packList<CurrentEdge>(results);
}
void ReadDirResp::unpack(BincodeBuf& buf) {
    nextHash = buf.unpackScalar<uint64_t>();
    buf.unpackList<CurrentEdge>(results);
}
void ReadDirResp::clear() {
    nextHash = uint64_t(0);
    results.clear();
}
bool ReadDirResp::operator==(const ReadDirResp& rhs) const {
    if ((uint64_t)this->nextHash != (uint64_t)rhs.nextHash) { return false; };
    if (results != rhs.results) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ReadDirResp& x) {
    out << "ReadDirResp(" << "NextHash=" << x.nextHash << ", " << "Results=" << x.results << ")";
    return out;
}

void ConstructFileReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(type);
    buf.packBytes(note);
}
void ConstructFileReq::unpack(BincodeBuf& buf) {
    type = buf.unpackScalar<uint8_t>();
    buf.unpackBytes(note);
}
void ConstructFileReq::clear() {
    type = uint8_t(0);
    note.clear();
}
bool ConstructFileReq::operator==(const ConstructFileReq& rhs) const {
    if ((uint8_t)this->type != (uint8_t)rhs.type) { return false; };
    if (note != rhs.note) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ConstructFileReq& x) {
    out << "ConstructFileReq(" << "Type=" << (int)x.type << ", " << "Note=" << GoLangQuotedStringFmt(x.note.data(), x.note.size()) << ")";
    return out;
}

void ConstructFileResp::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packFixedBytes<8>(cookie);
}
void ConstructFileResp::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    buf.unpackFixedBytes<8>(cookie);
}
void ConstructFileResp::clear() {
    id = InodeId();
    cookie.clear();
}
bool ConstructFileResp::operator==(const ConstructFileResp& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if (cookie != rhs.cookie) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ConstructFileResp& x) {
    out << "ConstructFileResp(" << "Id=" << x.id << ", " << "Cookie=" << x.cookie << ")";
    return out;
}

void AddSpanInitiateReq::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packFixedBytes<8>(cookie);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packScalar<uint32_t>(size);
    crc.pack(buf);
    buf.packScalar<uint8_t>(storageClass);
    buf.packList<BlacklistEntry>(blacklist);
    parity.pack(buf);
    buf.packScalar<uint8_t>(stripes);
    buf.packScalar<uint32_t>(cellSize);
    buf.packList<Crc>(crcs);
}
void AddSpanInitiateReq::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    buf.unpackFixedBytes<8>(cookie);
    byteOffset = buf.unpackScalar<uint64_t>();
    size = buf.unpackScalar<uint32_t>();
    crc.unpack(buf);
    storageClass = buf.unpackScalar<uint8_t>();
    buf.unpackList<BlacklistEntry>(blacklist);
    parity.unpack(buf);
    stripes = buf.unpackScalar<uint8_t>();
    cellSize = buf.unpackScalar<uint32_t>();
    buf.unpackList<Crc>(crcs);
}
void AddSpanInitiateReq::clear() {
    fileId = InodeId();
    cookie.clear();
    byteOffset = uint64_t(0);
    size = uint32_t(0);
    crc = Crc(0);
    storageClass = uint8_t(0);
    blacklist.clear();
    parity = Parity();
    stripes = uint8_t(0);
    cellSize = uint32_t(0);
    crcs.clear();
}
bool AddSpanInitiateReq::operator==(const AddSpanInitiateReq& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if (cookie != rhs.cookie) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if ((uint32_t)this->size != (uint32_t)rhs.size) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    if (blacklist != rhs.blacklist) { return false; };
    if ((Parity)this->parity != (Parity)rhs.parity) { return false; };
    if ((uint8_t)this->stripes != (uint8_t)rhs.stripes) { return false; };
    if ((uint32_t)this->cellSize != (uint32_t)rhs.cellSize) { return false; };
    if (crcs != rhs.crcs) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanInitiateReq& x) {
    out << "AddSpanInitiateReq(" << "FileId=" << x.fileId << ", " << "Cookie=" << x.cookie << ", " << "ByteOffset=" << x.byteOffset << ", " << "Size=" << x.size << ", " << "Crc=" << x.crc << ", " << "StorageClass=" << (int)x.storageClass << ", " << "Blacklist=" << x.blacklist << ", " << "Parity=" << x.parity << ", " << "Stripes=" << (int)x.stripes << ", " << "CellSize=" << x.cellSize << ", " << "Crcs=" << x.crcs << ")";
    return out;
}

void AddSpanInitiateResp::pack(BincodeBuf& buf) const {
    buf.packList<AddSpanInitiateBlockInfo>(blocks);
}
void AddSpanInitiateResp::unpack(BincodeBuf& buf) {
    buf.unpackList<AddSpanInitiateBlockInfo>(blocks);
}
void AddSpanInitiateResp::clear() {
    blocks.clear();
}
bool AddSpanInitiateResp::operator==(const AddSpanInitiateResp& rhs) const {
    if (blocks != rhs.blocks) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanInitiateResp& x) {
    out << "AddSpanInitiateResp(" << "Blocks=" << x.blocks << ")";
    return out;
}

void AddSpanCertifyReq::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packFixedBytes<8>(cookie);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packList<BlockProof>(proofs);
}
void AddSpanCertifyReq::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    buf.unpackFixedBytes<8>(cookie);
    byteOffset = buf.unpackScalar<uint64_t>();
    buf.unpackList<BlockProof>(proofs);
}
void AddSpanCertifyReq::clear() {
    fileId = InodeId();
    cookie.clear();
    byteOffset = uint64_t(0);
    proofs.clear();
}
bool AddSpanCertifyReq::operator==(const AddSpanCertifyReq& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if (cookie != rhs.cookie) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if (proofs != rhs.proofs) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanCertifyReq& x) {
    out << "AddSpanCertifyReq(" << "FileId=" << x.fileId << ", " << "Cookie=" << x.cookie << ", " << "ByteOffset=" << x.byteOffset << ", " << "Proofs=" << x.proofs << ")";
    return out;
}

void AddSpanCertifyResp::pack(BincodeBuf& buf) const {
}
void AddSpanCertifyResp::unpack(BincodeBuf& buf) {
}
void AddSpanCertifyResp::clear() {
}
bool AddSpanCertifyResp::operator==(const AddSpanCertifyResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanCertifyResp& x) {
    out << "AddSpanCertifyResp(" << ")";
    return out;
}

void LinkFileReq::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packFixedBytes<8>(cookie);
    ownerId.pack(buf);
    buf.packBytes(name);
}
void LinkFileReq::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    buf.unpackFixedBytes<8>(cookie);
    ownerId.unpack(buf);
    buf.unpackBytes(name);
}
void LinkFileReq::clear() {
    fileId = InodeId();
    cookie.clear();
    ownerId = InodeId();
    name.clear();
}
bool LinkFileReq::operator==(const LinkFileReq& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if (cookie != rhs.cookie) { return false; };
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if (name != rhs.name) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LinkFileReq& x) {
    out << "LinkFileReq(" << "FileId=" << x.fileId << ", " << "Cookie=" << x.cookie << ", " << "OwnerId=" << x.ownerId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ")";
    return out;
}

void LinkFileResp::pack(BincodeBuf& buf) const {
    creationTime.pack(buf);
}
void LinkFileResp::unpack(BincodeBuf& buf) {
    creationTime.unpack(buf);
}
void LinkFileResp::clear() {
    creationTime = TernTime();
}
bool LinkFileResp::operator==(const LinkFileResp& rhs) const {
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LinkFileResp& x) {
    out << "LinkFileResp(" << "CreationTime=" << x.creationTime << ")";
    return out;
}

void SoftUnlinkFileReq::pack(BincodeBuf& buf) const {
    ownerId.pack(buf);
    fileId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
}
void SoftUnlinkFileReq::unpack(BincodeBuf& buf) {
    ownerId.unpack(buf);
    fileId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
}
void SoftUnlinkFileReq::clear() {
    ownerId = InodeId();
    fileId = InodeId();
    name.clear();
    creationTime = TernTime();
}
bool SoftUnlinkFileReq::operator==(const SoftUnlinkFileReq& rhs) const {
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SoftUnlinkFileReq& x) {
    out << "SoftUnlinkFileReq(" << "OwnerId=" << x.ownerId << ", " << "FileId=" << x.fileId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void SoftUnlinkFileResp::pack(BincodeBuf& buf) const {
    deleteCreationTime.pack(buf);
}
void SoftUnlinkFileResp::unpack(BincodeBuf& buf) {
    deleteCreationTime.unpack(buf);
}
void SoftUnlinkFileResp::clear() {
    deleteCreationTime = TernTime();
}
bool SoftUnlinkFileResp::operator==(const SoftUnlinkFileResp& rhs) const {
    if ((TernTime)this->deleteCreationTime != (TernTime)rhs.deleteCreationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SoftUnlinkFileResp& x) {
    out << "SoftUnlinkFileResp(" << "DeleteCreationTime=" << x.deleteCreationTime << ")";
    return out;
}

void LocalFileSpansReq::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packScalar<uint32_t>(limit);
    buf.packScalar<uint16_t>(mtu);
}
void LocalFileSpansReq::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    byteOffset = buf.unpackScalar<uint64_t>();
    limit = buf.unpackScalar<uint32_t>();
    mtu = buf.unpackScalar<uint16_t>();
}
void LocalFileSpansReq::clear() {
    fileId = InodeId();
    byteOffset = uint64_t(0);
    limit = uint32_t(0);
    mtu = uint16_t(0);
}
bool LocalFileSpansReq::operator==(const LocalFileSpansReq& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if ((uint32_t)this->limit != (uint32_t)rhs.limit) { return false; };
    if ((uint16_t)this->mtu != (uint16_t)rhs.mtu) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocalFileSpansReq& x) {
    out << "LocalFileSpansReq(" << "FileId=" << x.fileId << ", " << "ByteOffset=" << x.byteOffset << ", " << "Limit=" << x.limit << ", " << "Mtu=" << x.mtu << ")";
    return out;
}

void LocalFileSpansResp::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(nextOffset);
    buf.packList<BlockService>(blockServices);
    buf.packList<FetchedSpan>(spans);
}
void LocalFileSpansResp::unpack(BincodeBuf& buf) {
    nextOffset = buf.unpackScalar<uint64_t>();
    buf.unpackList<BlockService>(blockServices);
    buf.unpackList<FetchedSpan>(spans);
}
void LocalFileSpansResp::clear() {
    nextOffset = uint64_t(0);
    blockServices.clear();
    spans.clear();
}
bool LocalFileSpansResp::operator==(const LocalFileSpansResp& rhs) const {
    if ((uint64_t)this->nextOffset != (uint64_t)rhs.nextOffset) { return false; };
    if (blockServices != rhs.blockServices) { return false; };
    if (spans != rhs.spans) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocalFileSpansResp& x) {
    out << "LocalFileSpansResp(" << "NextOffset=" << x.nextOffset << ", " << "BlockServices=" << x.blockServices << ", " << "Spans=" << x.spans << ")";
    return out;
}

void SameDirectoryRenameReq::pack(BincodeBuf& buf) const {
    targetId.pack(buf);
    dirId.pack(buf);
    buf.packBytes(oldName);
    oldCreationTime.pack(buf);
    buf.packBytes(newName);
}
void SameDirectoryRenameReq::unpack(BincodeBuf& buf) {
    targetId.unpack(buf);
    dirId.unpack(buf);
    buf.unpackBytes(oldName);
    oldCreationTime.unpack(buf);
    buf.unpackBytes(newName);
}
void SameDirectoryRenameReq::clear() {
    targetId = InodeId();
    dirId = InodeId();
    oldName.clear();
    oldCreationTime = TernTime();
    newName.clear();
}
bool SameDirectoryRenameReq::operator==(const SameDirectoryRenameReq& rhs) const {
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (oldName != rhs.oldName) { return false; };
    if ((TernTime)this->oldCreationTime != (TernTime)rhs.oldCreationTime) { return false; };
    if (newName != rhs.newName) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameReq& x) {
    out << "SameDirectoryRenameReq(" << "TargetId=" << x.targetId << ", " << "DirId=" << x.dirId << ", " << "OldName=" << GoLangQuotedStringFmt(x.oldName.data(), x.oldName.size()) << ", " << "OldCreationTime=" << x.oldCreationTime << ", " << "NewName=" << GoLangQuotedStringFmt(x.newName.data(), x.newName.size()) << ")";
    return out;
}

void SameDirectoryRenameResp::pack(BincodeBuf& buf) const {
    newCreationTime.pack(buf);
}
void SameDirectoryRenameResp::unpack(BincodeBuf& buf) {
    newCreationTime.unpack(buf);
}
void SameDirectoryRenameResp::clear() {
    newCreationTime = TernTime();
}
bool SameDirectoryRenameResp::operator==(const SameDirectoryRenameResp& rhs) const {
    if ((TernTime)this->newCreationTime != (TernTime)rhs.newCreationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameResp& x) {
    out << "SameDirectoryRenameResp(" << "NewCreationTime=" << x.newCreationTime << ")";
    return out;
}

void AddInlineSpanReq::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packFixedBytes<8>(cookie);
    buf.packScalar<uint8_t>(storageClass);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packScalar<uint32_t>(size);
    crc.pack(buf);
    buf.packBytes(body);
}
void AddInlineSpanReq::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    buf.unpackFixedBytes<8>(cookie);
    storageClass = buf.unpackScalar<uint8_t>();
    byteOffset = buf.unpackScalar<uint64_t>();
    size = buf.unpackScalar<uint32_t>();
    crc.unpack(buf);
    buf.unpackBytes(body);
}
void AddInlineSpanReq::clear() {
    fileId = InodeId();
    cookie.clear();
    storageClass = uint8_t(0);
    byteOffset = uint64_t(0);
    size = uint32_t(0);
    crc = Crc(0);
    body.clear();
}
bool AddInlineSpanReq::operator==(const AddInlineSpanReq& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if (cookie != rhs.cookie) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if ((uint32_t)this->size != (uint32_t)rhs.size) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    if (body != rhs.body) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddInlineSpanReq& x) {
    out << "AddInlineSpanReq(" << "FileId=" << x.fileId << ", " << "Cookie=" << x.cookie << ", " << "StorageClass=" << (int)x.storageClass << ", " << "ByteOffset=" << x.byteOffset << ", " << "Size=" << x.size << ", " << "Crc=" << x.crc << ", " << "Body=" << x.body << ")";
    return out;
}

void AddInlineSpanResp::pack(BincodeBuf& buf) const {
}
void AddInlineSpanResp::unpack(BincodeBuf& buf) {
}
void AddInlineSpanResp::clear() {
}
bool AddInlineSpanResp::operator==(const AddInlineSpanResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddInlineSpanResp& x) {
    out << "AddInlineSpanResp(" << ")";
    return out;
}

void SetTimeReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packScalar<uint64_t>(mtime);
    buf.packScalar<uint64_t>(atime);
}
void SetTimeReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    mtime = buf.unpackScalar<uint64_t>();
    atime = buf.unpackScalar<uint64_t>();
}
void SetTimeReq::clear() {
    id = InodeId();
    mtime = uint64_t(0);
    atime = uint64_t(0);
}
bool SetTimeReq::operator==(const SetTimeReq& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if ((uint64_t)this->mtime != (uint64_t)rhs.mtime) { return false; };
    if ((uint64_t)this->atime != (uint64_t)rhs.atime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetTimeReq& x) {
    out << "SetTimeReq(" << "Id=" << x.id << ", " << "Mtime=" << x.mtime << ", " << "Atime=" << x.atime << ")";
    return out;
}

void SetTimeResp::pack(BincodeBuf& buf) const {
}
void SetTimeResp::unpack(BincodeBuf& buf) {
}
void SetTimeResp::clear() {
}
bool SetTimeResp::operator==(const SetTimeResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetTimeResp& x) {
    out << "SetTimeResp(" << ")";
    return out;
}

void FullReadDirReq::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    buf.packScalar<uint8_t>(flags);
    buf.packBytes(startName);
    startTime.pack(buf);
    buf.packScalar<uint16_t>(limit);
    buf.packScalar<uint16_t>(mtu);
}
void FullReadDirReq::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    flags = buf.unpackScalar<uint8_t>();
    buf.unpackBytes(startName);
    startTime.unpack(buf);
    limit = buf.unpackScalar<uint16_t>();
    mtu = buf.unpackScalar<uint16_t>();
}
void FullReadDirReq::clear() {
    dirId = InodeId();
    flags = uint8_t(0);
    startName.clear();
    startTime = TernTime();
    limit = uint16_t(0);
    mtu = uint16_t(0);
}
bool FullReadDirReq::operator==(const FullReadDirReq& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if ((uint8_t)this->flags != (uint8_t)rhs.flags) { return false; };
    if (startName != rhs.startName) { return false; };
    if ((TernTime)this->startTime != (TernTime)rhs.startTime) { return false; };
    if ((uint16_t)this->limit != (uint16_t)rhs.limit) { return false; };
    if ((uint16_t)this->mtu != (uint16_t)rhs.mtu) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FullReadDirReq& x) {
    out << "FullReadDirReq(" << "DirId=" << x.dirId << ", " << "Flags=" << (int)x.flags << ", " << "StartName=" << GoLangQuotedStringFmt(x.startName.data(), x.startName.size()) << ", " << "StartTime=" << x.startTime << ", " << "Limit=" << x.limit << ", " << "Mtu=" << x.mtu << ")";
    return out;
}

void FullReadDirResp::pack(BincodeBuf& buf) const {
    next.pack(buf);
    buf.packList<Edge>(results);
}
void FullReadDirResp::unpack(BincodeBuf& buf) {
    next.unpack(buf);
    buf.unpackList<Edge>(results);
}
void FullReadDirResp::clear() {
    next.clear();
    results.clear();
}
bool FullReadDirResp::operator==(const FullReadDirResp& rhs) const {
    if (next != rhs.next) { return false; };
    if (results != rhs.results) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FullReadDirResp& x) {
    out << "FullReadDirResp(" << "Next=" << x.next << ", " << "Results=" << x.results << ")";
    return out;
}

void MoveSpanReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint32_t>(spanSize);
    fileId1.pack(buf);
    buf.packScalar<uint64_t>(byteOffset1);
    buf.packFixedBytes<8>(cookie1);
    fileId2.pack(buf);
    buf.packScalar<uint64_t>(byteOffset2);
    buf.packFixedBytes<8>(cookie2);
}
void MoveSpanReq::unpack(BincodeBuf& buf) {
    spanSize = buf.unpackScalar<uint32_t>();
    fileId1.unpack(buf);
    byteOffset1 = buf.unpackScalar<uint64_t>();
    buf.unpackFixedBytes<8>(cookie1);
    fileId2.unpack(buf);
    byteOffset2 = buf.unpackScalar<uint64_t>();
    buf.unpackFixedBytes<8>(cookie2);
}
void MoveSpanReq::clear() {
    spanSize = uint32_t(0);
    fileId1 = InodeId();
    byteOffset1 = uint64_t(0);
    cookie1.clear();
    fileId2 = InodeId();
    byteOffset2 = uint64_t(0);
    cookie2.clear();
}
bool MoveSpanReq::operator==(const MoveSpanReq& rhs) const {
    if ((uint32_t)this->spanSize != (uint32_t)rhs.spanSize) { return false; };
    if ((InodeId)this->fileId1 != (InodeId)rhs.fileId1) { return false; };
    if ((uint64_t)this->byteOffset1 != (uint64_t)rhs.byteOffset1) { return false; };
    if (cookie1 != rhs.cookie1) { return false; };
    if ((InodeId)this->fileId2 != (InodeId)rhs.fileId2) { return false; };
    if ((uint64_t)this->byteOffset2 != (uint64_t)rhs.byteOffset2) { return false; };
    if (cookie2 != rhs.cookie2) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const MoveSpanReq& x) {
    out << "MoveSpanReq(" << "SpanSize=" << x.spanSize << ", " << "FileId1=" << x.fileId1 << ", " << "ByteOffset1=" << x.byteOffset1 << ", " << "Cookie1=" << x.cookie1 << ", " << "FileId2=" << x.fileId2 << ", " << "ByteOffset2=" << x.byteOffset2 << ", " << "Cookie2=" << x.cookie2 << ")";
    return out;
}

void MoveSpanResp::pack(BincodeBuf& buf) const {
}
void MoveSpanResp::unpack(BincodeBuf& buf) {
}
void MoveSpanResp::clear() {
}
bool MoveSpanResp::operator==(const MoveSpanResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const MoveSpanResp& x) {
    out << "MoveSpanResp(" << ")";
    return out;
}

void RemoveNonOwnedEdgeReq::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    targetId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
}
void RemoveNonOwnedEdgeReq::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    targetId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
}
void RemoveNonOwnedEdgeReq::clear() {
    dirId = InodeId();
    targetId = InodeId();
    name.clear();
    creationTime = TernTime();
}
bool RemoveNonOwnedEdgeReq::operator==(const RemoveNonOwnedEdgeReq& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveNonOwnedEdgeReq& x) {
    out << "RemoveNonOwnedEdgeReq(" << "DirId=" << x.dirId << ", " << "TargetId=" << x.targetId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void RemoveNonOwnedEdgeResp::pack(BincodeBuf& buf) const {
}
void RemoveNonOwnedEdgeResp::unpack(BincodeBuf& buf) {
}
void RemoveNonOwnedEdgeResp::clear() {
}
bool RemoveNonOwnedEdgeResp::operator==(const RemoveNonOwnedEdgeResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveNonOwnedEdgeResp& x) {
    out << "RemoveNonOwnedEdgeResp(" << ")";
    return out;
}

void SameShardHardFileUnlinkReq::pack(BincodeBuf& buf) const {
    ownerId.pack(buf);
    targetId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
}
void SameShardHardFileUnlinkReq::unpack(BincodeBuf& buf) {
    ownerId.unpack(buf);
    targetId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
}
void SameShardHardFileUnlinkReq::clear() {
    ownerId = InodeId();
    targetId = InodeId();
    name.clear();
    creationTime = TernTime();
}
bool SameShardHardFileUnlinkReq::operator==(const SameShardHardFileUnlinkReq& rhs) const {
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SameShardHardFileUnlinkReq& x) {
    out << "SameShardHardFileUnlinkReq(" << "OwnerId=" << x.ownerId << ", " << "TargetId=" << x.targetId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void SameShardHardFileUnlinkResp::pack(BincodeBuf& buf) const {
}
void SameShardHardFileUnlinkResp::unpack(BincodeBuf& buf) {
}
void SameShardHardFileUnlinkResp::clear() {
}
bool SameShardHardFileUnlinkResp::operator==(const SameShardHardFileUnlinkResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const SameShardHardFileUnlinkResp& x) {
    out << "SameShardHardFileUnlinkResp(" << ")";
    return out;
}

void StatTransientFileReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
}
void StatTransientFileReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
}
void StatTransientFileReq::clear() {
    id = InodeId();
}
bool StatTransientFileReq::operator==(const StatTransientFileReq& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const StatTransientFileReq& x) {
    out << "StatTransientFileReq(" << "Id=" << x.id << ")";
    return out;
}

void StatTransientFileResp::pack(BincodeBuf& buf) const {
    mtime.pack(buf);
    buf.packScalar<uint64_t>(size);
    buf.packBytes(note);
}
void StatTransientFileResp::unpack(BincodeBuf& buf) {
    mtime.unpack(buf);
    size = buf.unpackScalar<uint64_t>();
    buf.unpackBytes(note);
}
void StatTransientFileResp::clear() {
    mtime = TernTime();
    size = uint64_t(0);
    note.clear();
}
bool StatTransientFileResp::operator==(const StatTransientFileResp& rhs) const {
    if ((TernTime)this->mtime != (TernTime)rhs.mtime) { return false; };
    if ((uint64_t)this->size != (uint64_t)rhs.size) { return false; };
    if (note != rhs.note) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const StatTransientFileResp& x) {
    out << "StatTransientFileResp(" << "Mtime=" << x.mtime << ", " << "Size=" << x.size << ", " << "Note=" << GoLangQuotedStringFmt(x.note.data(), x.note.size()) << ")";
    return out;
}

void ShardSnapshotReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(snapshotId);
}
void ShardSnapshotReq::unpack(BincodeBuf& buf) {
    snapshotId = buf.unpackScalar<uint64_t>();
}
void ShardSnapshotReq::clear() {
    snapshotId = uint64_t(0);
}
bool ShardSnapshotReq::operator==(const ShardSnapshotReq& rhs) const {
    if ((uint64_t)this->snapshotId != (uint64_t)rhs.snapshotId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ShardSnapshotReq& x) {
    out << "ShardSnapshotReq(" << "SnapshotId=" << x.snapshotId << ")";
    return out;
}

void ShardSnapshotResp::pack(BincodeBuf& buf) const {
}
void ShardSnapshotResp::unpack(BincodeBuf& buf) {
}
void ShardSnapshotResp::clear() {
}
bool ShardSnapshotResp::operator==(const ShardSnapshotResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const ShardSnapshotResp& x) {
    out << "ShardSnapshotResp(" << ")";
    return out;
}

void FileSpansReq::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packScalar<uint32_t>(limit);
    buf.packScalar<uint16_t>(mtu);
}
void FileSpansReq::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    byteOffset = buf.unpackScalar<uint64_t>();
    limit = buf.unpackScalar<uint32_t>();
    mtu = buf.unpackScalar<uint16_t>();
}
void FileSpansReq::clear() {
    fileId = InodeId();
    byteOffset = uint64_t(0);
    limit = uint32_t(0);
    mtu = uint16_t(0);
}
bool FileSpansReq::operator==(const FileSpansReq& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if ((uint32_t)this->limit != (uint32_t)rhs.limit) { return false; };
    if ((uint16_t)this->mtu != (uint16_t)rhs.mtu) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FileSpansReq& x) {
    out << "FileSpansReq(" << "FileId=" << x.fileId << ", " << "ByteOffset=" << x.byteOffset << ", " << "Limit=" << x.limit << ", " << "Mtu=" << x.mtu << ")";
    return out;
}

void FileSpansResp::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(nextOffset);
    buf.packList<BlockService>(blockServices);
    buf.packList<FetchedFullSpan>(spans);
}
void FileSpansResp::unpack(BincodeBuf& buf) {
    nextOffset = buf.unpackScalar<uint64_t>();
    buf.unpackList<BlockService>(blockServices);
    buf.unpackList<FetchedFullSpan>(spans);
}
void FileSpansResp::clear() {
    nextOffset = uint64_t(0);
    blockServices.clear();
    spans.clear();
}
bool FileSpansResp::operator==(const FileSpansResp& rhs) const {
    if ((uint64_t)this->nextOffset != (uint64_t)rhs.nextOffset) { return false; };
    if (blockServices != rhs.blockServices) { return false; };
    if (spans != rhs.spans) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FileSpansResp& x) {
    out << "FileSpansResp(" << "NextOffset=" << x.nextOffset << ", " << "BlockServices=" << x.blockServices << ", " << "Spans=" << x.spans << ")";
    return out;
}

void AddSpanLocationReq::pack(BincodeBuf& buf) const {
    fileId1.pack(buf);
    buf.packScalar<uint64_t>(byteOffset1);
    buf.packList<uint64_t>(blocks1);
    fileId2.pack(buf);
    buf.packScalar<uint64_t>(byteOffset2);
}
void AddSpanLocationReq::unpack(BincodeBuf& buf) {
    fileId1.unpack(buf);
    byteOffset1 = buf.unpackScalar<uint64_t>();
    buf.unpackList<uint64_t>(blocks1);
    fileId2.unpack(buf);
    byteOffset2 = buf.unpackScalar<uint64_t>();
}
void AddSpanLocationReq::clear() {
    fileId1 = InodeId();
    byteOffset1 = uint64_t(0);
    blocks1.clear();
    fileId2 = InodeId();
    byteOffset2 = uint64_t(0);
}
bool AddSpanLocationReq::operator==(const AddSpanLocationReq& rhs) const {
    if ((InodeId)this->fileId1 != (InodeId)rhs.fileId1) { return false; };
    if ((uint64_t)this->byteOffset1 != (uint64_t)rhs.byteOffset1) { return false; };
    if (blocks1 != rhs.blocks1) { return false; };
    if ((InodeId)this->fileId2 != (InodeId)rhs.fileId2) { return false; };
    if ((uint64_t)this->byteOffset2 != (uint64_t)rhs.byteOffset2) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanLocationReq& x) {
    out << "AddSpanLocationReq(" << "FileId1=" << x.fileId1 << ", " << "ByteOffset1=" << x.byteOffset1 << ", " << "Blocks1=" << x.blocks1 << ", " << "FileId2=" << x.fileId2 << ", " << "ByteOffset2=" << x.byteOffset2 << ")";
    return out;
}

void AddSpanLocationResp::pack(BincodeBuf& buf) const {
}
void AddSpanLocationResp::unpack(BincodeBuf& buf) {
}
void AddSpanLocationResp::clear() {
}
bool AddSpanLocationResp::operator==(const AddSpanLocationResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanLocationResp& x) {
    out << "AddSpanLocationResp(" << ")";
    return out;
}

void ScrapTransientFileReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packFixedBytes<8>(cookie);
}
void ScrapTransientFileReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    buf.unpackFixedBytes<8>(cookie);
}
void ScrapTransientFileReq::clear() {
    id = InodeId();
    cookie.clear();
}
bool ScrapTransientFileReq::operator==(const ScrapTransientFileReq& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if (cookie != rhs.cookie) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ScrapTransientFileReq& x) {
    out << "ScrapTransientFileReq(" << "Id=" << x.id << ", " << "Cookie=" << x.cookie << ")";
    return out;
}

void ScrapTransientFileResp::pack(BincodeBuf& buf) const {
}
void ScrapTransientFileResp::unpack(BincodeBuf& buf) {
}
void ScrapTransientFileResp::clear() {
}
bool ScrapTransientFileResp::operator==(const ScrapTransientFileResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const ScrapTransientFileResp& x) {
    out << "ScrapTransientFileResp(" << ")";
    return out;
}

void SetDirectoryInfoReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
    info.pack(buf);
}
void SetDirectoryInfoReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    info.unpack(buf);
}
void SetDirectoryInfoReq::clear() {
    id = InodeId();
    info.clear();
}
bool SetDirectoryInfoReq::operator==(const SetDirectoryInfoReq& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if (info != rhs.info) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetDirectoryInfoReq& x) {
    out << "SetDirectoryInfoReq(" << "Id=" << x.id << ", " << "Info=" << x.info << ")";
    return out;
}

void SetDirectoryInfoResp::pack(BincodeBuf& buf) const {
}
void SetDirectoryInfoResp::unpack(BincodeBuf& buf) {
}
void SetDirectoryInfoResp::clear() {
}
bool SetDirectoryInfoResp::operator==(const SetDirectoryInfoResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetDirectoryInfoResp& x) {
    out << "SetDirectoryInfoResp(" << ")";
    return out;
}

void VisitDirectoriesReq::pack(BincodeBuf& buf) const {
    beginId.pack(buf);
    buf.packScalar<uint16_t>(mtu);
}
void VisitDirectoriesReq::unpack(BincodeBuf& buf) {
    beginId.unpack(buf);
    mtu = buf.unpackScalar<uint16_t>();
}
void VisitDirectoriesReq::clear() {
    beginId = InodeId();
    mtu = uint16_t(0);
}
bool VisitDirectoriesReq::operator==(const VisitDirectoriesReq& rhs) const {
    if ((InodeId)this->beginId != (InodeId)rhs.beginId) { return false; };
    if ((uint16_t)this->mtu != (uint16_t)rhs.mtu) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const VisitDirectoriesReq& x) {
    out << "VisitDirectoriesReq(" << "BeginId=" << x.beginId << ", " << "Mtu=" << x.mtu << ")";
    return out;
}

void VisitDirectoriesResp::pack(BincodeBuf& buf) const {
    nextId.pack(buf);
    buf.packList<InodeId>(ids);
}
void VisitDirectoriesResp::unpack(BincodeBuf& buf) {
    nextId.unpack(buf);
    buf.unpackList<InodeId>(ids);
}
void VisitDirectoriesResp::clear() {
    nextId = InodeId();
    ids.clear();
}
bool VisitDirectoriesResp::operator==(const VisitDirectoriesResp& rhs) const {
    if ((InodeId)this->nextId != (InodeId)rhs.nextId) { return false; };
    if (ids != rhs.ids) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const VisitDirectoriesResp& x) {
    out << "VisitDirectoriesResp(" << "NextId=" << x.nextId << ", " << "Ids=" << x.ids << ")";
    return out;
}

void VisitFilesReq::pack(BincodeBuf& buf) const {
    beginId.pack(buf);
    buf.packScalar<uint16_t>(mtu);
}
void VisitFilesReq::unpack(BincodeBuf& buf) {
    beginId.unpack(buf);
    mtu = buf.unpackScalar<uint16_t>();
}
void VisitFilesReq::clear() {
    beginId = InodeId();
    mtu = uint16_t(0);
}
bool VisitFilesReq::operator==(const VisitFilesReq& rhs) const {
    if ((InodeId)this->beginId != (InodeId)rhs.beginId) { return false; };
    if ((uint16_t)this->mtu != (uint16_t)rhs.mtu) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const VisitFilesReq& x) {
    out << "VisitFilesReq(" << "BeginId=" << x.beginId << ", " << "Mtu=" << x.mtu << ")";
    return out;
}

void VisitFilesResp::pack(BincodeBuf& buf) const {
    nextId.pack(buf);
    buf.packList<InodeId>(ids);
}
void VisitFilesResp::unpack(BincodeBuf& buf) {
    nextId.unpack(buf);
    buf.unpackList<InodeId>(ids);
}
void VisitFilesResp::clear() {
    nextId = InodeId();
    ids.clear();
}
bool VisitFilesResp::operator==(const VisitFilesResp& rhs) const {
    if ((InodeId)this->nextId != (InodeId)rhs.nextId) { return false; };
    if (ids != rhs.ids) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const VisitFilesResp& x) {
    out << "VisitFilesResp(" << "NextId=" << x.nextId << ", " << "Ids=" << x.ids << ")";
    return out;
}

void VisitTransientFilesReq::pack(BincodeBuf& buf) const {
    beginId.pack(buf);
    buf.packScalar<uint16_t>(mtu);
}
void VisitTransientFilesReq::unpack(BincodeBuf& buf) {
    beginId.unpack(buf);
    mtu = buf.unpackScalar<uint16_t>();
}
void VisitTransientFilesReq::clear() {
    beginId = InodeId();
    mtu = uint16_t(0);
}
bool VisitTransientFilesReq::operator==(const VisitTransientFilesReq& rhs) const {
    if ((InodeId)this->beginId != (InodeId)rhs.beginId) { return false; };
    if ((uint16_t)this->mtu != (uint16_t)rhs.mtu) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const VisitTransientFilesReq& x) {
    out << "VisitTransientFilesReq(" << "BeginId=" << x.beginId << ", " << "Mtu=" << x.mtu << ")";
    return out;
}

void VisitTransientFilesResp::pack(BincodeBuf& buf) const {
    nextId.pack(buf);
    buf.packList<TransientFile>(files);
}
void VisitTransientFilesResp::unpack(BincodeBuf& buf) {
    nextId.unpack(buf);
    buf.unpackList<TransientFile>(files);
}
void VisitTransientFilesResp::clear() {
    nextId = InodeId();
    files.clear();
}
bool VisitTransientFilesResp::operator==(const VisitTransientFilesResp& rhs) const {
    if ((InodeId)this->nextId != (InodeId)rhs.nextId) { return false; };
    if (files != rhs.files) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const VisitTransientFilesResp& x) {
    out << "VisitTransientFilesResp(" << "NextId=" << x.nextId << ", " << "Files=" << x.files << ")";
    return out;
}

void RemoveSpanInitiateReq::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packFixedBytes<8>(cookie);
}
void RemoveSpanInitiateReq::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    buf.unpackFixedBytes<8>(cookie);
}
void RemoveSpanInitiateReq::clear() {
    fileId = InodeId();
    cookie.clear();
}
bool RemoveSpanInitiateReq::operator==(const RemoveSpanInitiateReq& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if (cookie != rhs.cookie) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveSpanInitiateReq& x) {
    out << "RemoveSpanInitiateReq(" << "FileId=" << x.fileId << ", " << "Cookie=" << x.cookie << ")";
    return out;
}

void RemoveSpanInitiateResp::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(byteOffset);
    buf.packList<RemoveSpanInitiateBlockInfo>(blocks);
}
void RemoveSpanInitiateResp::unpack(BincodeBuf& buf) {
    byteOffset = buf.unpackScalar<uint64_t>();
    buf.unpackList<RemoveSpanInitiateBlockInfo>(blocks);
}
void RemoveSpanInitiateResp::clear() {
    byteOffset = uint64_t(0);
    blocks.clear();
}
bool RemoveSpanInitiateResp::operator==(const RemoveSpanInitiateResp& rhs) const {
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if (blocks != rhs.blocks) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveSpanInitiateResp& x) {
    out << "RemoveSpanInitiateResp(" << "ByteOffset=" << x.byteOffset << ", " << "Blocks=" << x.blocks << ")";
    return out;
}

void RemoveSpanCertifyReq::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packFixedBytes<8>(cookie);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packList<BlockProof>(proofs);
}
void RemoveSpanCertifyReq::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    buf.unpackFixedBytes<8>(cookie);
    byteOffset = buf.unpackScalar<uint64_t>();
    buf.unpackList<BlockProof>(proofs);
}
void RemoveSpanCertifyReq::clear() {
    fileId = InodeId();
    cookie.clear();
    byteOffset = uint64_t(0);
    proofs.clear();
}
bool RemoveSpanCertifyReq::operator==(const RemoveSpanCertifyReq& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if (cookie != rhs.cookie) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if (proofs != rhs.proofs) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveSpanCertifyReq& x) {
    out << "RemoveSpanCertifyReq(" << "FileId=" << x.fileId << ", " << "Cookie=" << x.cookie << ", " << "ByteOffset=" << x.byteOffset << ", " << "Proofs=" << x.proofs << ")";
    return out;
}

void RemoveSpanCertifyResp::pack(BincodeBuf& buf) const {
}
void RemoveSpanCertifyResp::unpack(BincodeBuf& buf) {
}
void RemoveSpanCertifyResp::clear() {
}
bool RemoveSpanCertifyResp::operator==(const RemoveSpanCertifyResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveSpanCertifyResp& x) {
    out << "RemoveSpanCertifyResp(" << ")";
    return out;
}

void SwapBlocksReq::pack(BincodeBuf& buf) const {
    fileId1.pack(buf);
    buf.packScalar<uint64_t>(byteOffset1);
    buf.packScalar<uint64_t>(blockId1);
    fileId2.pack(buf);
    buf.packScalar<uint64_t>(byteOffset2);
    buf.packScalar<uint64_t>(blockId2);
}
void SwapBlocksReq::unpack(BincodeBuf& buf) {
    fileId1.unpack(buf);
    byteOffset1 = buf.unpackScalar<uint64_t>();
    blockId1 = buf.unpackScalar<uint64_t>();
    fileId2.unpack(buf);
    byteOffset2 = buf.unpackScalar<uint64_t>();
    blockId2 = buf.unpackScalar<uint64_t>();
}
void SwapBlocksReq::clear() {
    fileId1 = InodeId();
    byteOffset1 = uint64_t(0);
    blockId1 = uint64_t(0);
    fileId2 = InodeId();
    byteOffset2 = uint64_t(0);
    blockId2 = uint64_t(0);
}
bool SwapBlocksReq::operator==(const SwapBlocksReq& rhs) const {
    if ((InodeId)this->fileId1 != (InodeId)rhs.fileId1) { return false; };
    if ((uint64_t)this->byteOffset1 != (uint64_t)rhs.byteOffset1) { return false; };
    if ((uint64_t)this->blockId1 != (uint64_t)rhs.blockId1) { return false; };
    if ((InodeId)this->fileId2 != (InodeId)rhs.fileId2) { return false; };
    if ((uint64_t)this->byteOffset2 != (uint64_t)rhs.byteOffset2) { return false; };
    if ((uint64_t)this->blockId2 != (uint64_t)rhs.blockId2) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SwapBlocksReq& x) {
    out << "SwapBlocksReq(" << "FileId1=" << x.fileId1 << ", " << "ByteOffset1=" << x.byteOffset1 << ", " << "BlockId1=" << x.blockId1 << ", " << "FileId2=" << x.fileId2 << ", " << "ByteOffset2=" << x.byteOffset2 << ", " << "BlockId2=" << x.blockId2 << ")";
    return out;
}

void SwapBlocksResp::pack(BincodeBuf& buf) const {
}
void SwapBlocksResp::unpack(BincodeBuf& buf) {
}
void SwapBlocksResp::clear() {
}
bool SwapBlocksResp::operator==(const SwapBlocksResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const SwapBlocksResp& x) {
    out << "SwapBlocksResp(" << ")";
    return out;
}

void BlockServiceFilesReq::pack(BincodeBuf& buf) const {
    blockServiceId.pack(buf);
    startFrom.pack(buf);
}
void BlockServiceFilesReq::unpack(BincodeBuf& buf) {
    blockServiceId.unpack(buf);
    startFrom.unpack(buf);
}
void BlockServiceFilesReq::clear() {
    blockServiceId = BlockServiceId(0);
    startFrom = InodeId();
}
bool BlockServiceFilesReq::operator==(const BlockServiceFilesReq& rhs) const {
    if ((BlockServiceId)this->blockServiceId != (BlockServiceId)rhs.blockServiceId) { return false; };
    if ((InodeId)this->startFrom != (InodeId)rhs.startFrom) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const BlockServiceFilesReq& x) {
    out << "BlockServiceFilesReq(" << "BlockServiceId=" << x.blockServiceId << ", " << "StartFrom=" << x.startFrom << ")";
    return out;
}

void BlockServiceFilesResp::pack(BincodeBuf& buf) const {
    buf.packList<InodeId>(fileIds);
}
void BlockServiceFilesResp::unpack(BincodeBuf& buf) {
    buf.unpackList<InodeId>(fileIds);
}
void BlockServiceFilesResp::clear() {
    fileIds.clear();
}
bool BlockServiceFilesResp::operator==(const BlockServiceFilesResp& rhs) const {
    if (fileIds != rhs.fileIds) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const BlockServiceFilesResp& x) {
    out << "BlockServiceFilesResp(" << "FileIds=" << x.fileIds << ")";
    return out;
}

void RemoveInodeReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
}
void RemoveInodeReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
}
void RemoveInodeReq::clear() {
    id = InodeId();
}
bool RemoveInodeReq::operator==(const RemoveInodeReq& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveInodeReq& x) {
    out << "RemoveInodeReq(" << "Id=" << x.id << ")";
    return out;
}

void RemoveInodeResp::pack(BincodeBuf& buf) const {
}
void RemoveInodeResp::unpack(BincodeBuf& buf) {
}
void RemoveInodeResp::clear() {
}
bool RemoveInodeResp::operator==(const RemoveInodeResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveInodeResp& x) {
    out << "RemoveInodeResp(" << ")";
    return out;
}

void AddSpanInitiateWithReferenceReq::pack(BincodeBuf& buf) const {
    req.pack(buf);
    reference.pack(buf);
}
void AddSpanInitiateWithReferenceReq::unpack(BincodeBuf& buf) {
    req.unpack(buf);
    reference.unpack(buf);
}
void AddSpanInitiateWithReferenceReq::clear() {
    req.clear();
    reference = InodeId();
}
bool AddSpanInitiateWithReferenceReq::operator==(const AddSpanInitiateWithReferenceReq& rhs) const {
    if (req != rhs.req) { return false; };
    if ((InodeId)this->reference != (InodeId)rhs.reference) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanInitiateWithReferenceReq& x) {
    out << "AddSpanInitiateWithReferenceReq(" << "Req=" << x.req << ", " << "Reference=" << x.reference << ")";
    return out;
}

void AddSpanInitiateWithReferenceResp::pack(BincodeBuf& buf) const {
    resp.pack(buf);
}
void AddSpanInitiateWithReferenceResp::unpack(BincodeBuf& buf) {
    resp.unpack(buf);
}
void AddSpanInitiateWithReferenceResp::clear() {
    resp.clear();
}
bool AddSpanInitiateWithReferenceResp::operator==(const AddSpanInitiateWithReferenceResp& rhs) const {
    if (resp != rhs.resp) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanInitiateWithReferenceResp& x) {
    out << "AddSpanInitiateWithReferenceResp(" << "Resp=" << x.resp << ")";
    return out;
}

void RemoveZeroBlockServiceFilesReq::pack(BincodeBuf& buf) const {
    startBlockService.pack(buf);
    startFile.pack(buf);
}
void RemoveZeroBlockServiceFilesReq::unpack(BincodeBuf& buf) {
    startBlockService.unpack(buf);
    startFile.unpack(buf);
}
void RemoveZeroBlockServiceFilesReq::clear() {
    startBlockService = BlockServiceId(0);
    startFile = InodeId();
}
bool RemoveZeroBlockServiceFilesReq::operator==(const RemoveZeroBlockServiceFilesReq& rhs) const {
    if ((BlockServiceId)this->startBlockService != (BlockServiceId)rhs.startBlockService) { return false; };
    if ((InodeId)this->startFile != (InodeId)rhs.startFile) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveZeroBlockServiceFilesReq& x) {
    out << "RemoveZeroBlockServiceFilesReq(" << "StartBlockService=" << x.startBlockService << ", " << "StartFile=" << x.startFile << ")";
    return out;
}

void RemoveZeroBlockServiceFilesResp::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(removed);
    nextBlockService.pack(buf);
    nextFile.pack(buf);
}
void RemoveZeroBlockServiceFilesResp::unpack(BincodeBuf& buf) {
    removed = buf.unpackScalar<uint64_t>();
    nextBlockService.unpack(buf);
    nextFile.unpack(buf);
}
void RemoveZeroBlockServiceFilesResp::clear() {
    removed = uint64_t(0);
    nextBlockService = BlockServiceId(0);
    nextFile = InodeId();
}
bool RemoveZeroBlockServiceFilesResp::operator==(const RemoveZeroBlockServiceFilesResp& rhs) const {
    if ((uint64_t)this->removed != (uint64_t)rhs.removed) { return false; };
    if ((BlockServiceId)this->nextBlockService != (BlockServiceId)rhs.nextBlockService) { return false; };
    if ((InodeId)this->nextFile != (InodeId)rhs.nextFile) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveZeroBlockServiceFilesResp& x) {
    out << "RemoveZeroBlockServiceFilesResp(" << "Removed=" << x.removed << ", " << "NextBlockService=" << x.nextBlockService << ", " << "NextFile=" << x.nextFile << ")";
    return out;
}

void SwapSpansReq::pack(BincodeBuf& buf) const {
    fileId1.pack(buf);
    buf.packScalar<uint64_t>(byteOffset1);
    buf.packList<uint64_t>(blocks1);
    fileId2.pack(buf);
    buf.packScalar<uint64_t>(byteOffset2);
    buf.packList<uint64_t>(blocks2);
}
void SwapSpansReq::unpack(BincodeBuf& buf) {
    fileId1.unpack(buf);
    byteOffset1 = buf.unpackScalar<uint64_t>();
    buf.unpackList<uint64_t>(blocks1);
    fileId2.unpack(buf);
    byteOffset2 = buf.unpackScalar<uint64_t>();
    buf.unpackList<uint64_t>(blocks2);
}
void SwapSpansReq::clear() {
    fileId1 = InodeId();
    byteOffset1 = uint64_t(0);
    blocks1.clear();
    fileId2 = InodeId();
    byteOffset2 = uint64_t(0);
    blocks2.clear();
}
bool SwapSpansReq::operator==(const SwapSpansReq& rhs) const {
    if ((InodeId)this->fileId1 != (InodeId)rhs.fileId1) { return false; };
    if ((uint64_t)this->byteOffset1 != (uint64_t)rhs.byteOffset1) { return false; };
    if (blocks1 != rhs.blocks1) { return false; };
    if ((InodeId)this->fileId2 != (InodeId)rhs.fileId2) { return false; };
    if ((uint64_t)this->byteOffset2 != (uint64_t)rhs.byteOffset2) { return false; };
    if (blocks2 != rhs.blocks2) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SwapSpansReq& x) {
    out << "SwapSpansReq(" << "FileId1=" << x.fileId1 << ", " << "ByteOffset1=" << x.byteOffset1 << ", " << "Blocks1=" << x.blocks1 << ", " << "FileId2=" << x.fileId2 << ", " << "ByteOffset2=" << x.byteOffset2 << ", " << "Blocks2=" << x.blocks2 << ")";
    return out;
}

void SwapSpansResp::pack(BincodeBuf& buf) const {
}
void SwapSpansResp::unpack(BincodeBuf& buf) {
}
void SwapSpansResp::clear() {
}
bool SwapSpansResp::operator==(const SwapSpansResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const SwapSpansResp& x) {
    out << "SwapSpansResp(" << ")";
    return out;
}

void SameDirectoryRenameSnapshotReq::pack(BincodeBuf& buf) const {
    targetId.pack(buf);
    dirId.pack(buf);
    buf.packBytes(oldName);
    oldCreationTime.pack(buf);
    buf.packBytes(newName);
}
void SameDirectoryRenameSnapshotReq::unpack(BincodeBuf& buf) {
    targetId.unpack(buf);
    dirId.unpack(buf);
    buf.unpackBytes(oldName);
    oldCreationTime.unpack(buf);
    buf.unpackBytes(newName);
}
void SameDirectoryRenameSnapshotReq::clear() {
    targetId = InodeId();
    dirId = InodeId();
    oldName.clear();
    oldCreationTime = TernTime();
    newName.clear();
}
bool SameDirectoryRenameSnapshotReq::operator==(const SameDirectoryRenameSnapshotReq& rhs) const {
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (oldName != rhs.oldName) { return false; };
    if ((TernTime)this->oldCreationTime != (TernTime)rhs.oldCreationTime) { return false; };
    if (newName != rhs.newName) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameSnapshotReq& x) {
    out << "SameDirectoryRenameSnapshotReq(" << "TargetId=" << x.targetId << ", " << "DirId=" << x.dirId << ", " << "OldName=" << GoLangQuotedStringFmt(x.oldName.data(), x.oldName.size()) << ", " << "OldCreationTime=" << x.oldCreationTime << ", " << "NewName=" << GoLangQuotedStringFmt(x.newName.data(), x.newName.size()) << ")";
    return out;
}

void SameDirectoryRenameSnapshotResp::pack(BincodeBuf& buf) const {
    newCreationTime.pack(buf);
}
void SameDirectoryRenameSnapshotResp::unpack(BincodeBuf& buf) {
    newCreationTime.unpack(buf);
}
void SameDirectoryRenameSnapshotResp::clear() {
    newCreationTime = TernTime();
}
bool SameDirectoryRenameSnapshotResp::operator==(const SameDirectoryRenameSnapshotResp& rhs) const {
    if ((TernTime)this->newCreationTime != (TernTime)rhs.newCreationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameSnapshotResp& x) {
    out << "SameDirectoryRenameSnapshotResp(" << "NewCreationTime=" << x.newCreationTime << ")";
    return out;
}

void AddSpanAtLocationInitiateReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(locationId);
    req.pack(buf);
}
void AddSpanAtLocationInitiateReq::unpack(BincodeBuf& buf) {
    locationId = buf.unpackScalar<uint8_t>();
    req.unpack(buf);
}
void AddSpanAtLocationInitiateReq::clear() {
    locationId = uint8_t(0);
    req.clear();
}
bool AddSpanAtLocationInitiateReq::operator==(const AddSpanAtLocationInitiateReq& rhs) const {
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    if (req != rhs.req) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanAtLocationInitiateReq& x) {
    out << "AddSpanAtLocationInitiateReq(" << "LocationId=" << (int)x.locationId << ", " << "Req=" << x.req << ")";
    return out;
}

void AddSpanAtLocationInitiateResp::pack(BincodeBuf& buf) const {
    resp.pack(buf);
}
void AddSpanAtLocationInitiateResp::unpack(BincodeBuf& buf) {
    resp.unpack(buf);
}
void AddSpanAtLocationInitiateResp::clear() {
    resp.clear();
}
bool AddSpanAtLocationInitiateResp::operator==(const AddSpanAtLocationInitiateResp& rhs) const {
    if (resp != rhs.resp) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanAtLocationInitiateResp& x) {
    out << "AddSpanAtLocationInitiateResp(" << "Resp=" << x.resp << ")";
    return out;
}

void CreateDirectoryInodeReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
    ownerId.pack(buf);
    info.pack(buf);
}
void CreateDirectoryInodeReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    ownerId.unpack(buf);
    info.unpack(buf);
}
void CreateDirectoryInodeReq::clear() {
    id = InodeId();
    ownerId = InodeId();
    info.clear();
}
bool CreateDirectoryInodeReq::operator==(const CreateDirectoryInodeReq& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if (info != rhs.info) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CreateDirectoryInodeReq& x) {
    out << "CreateDirectoryInodeReq(" << "Id=" << x.id << ", " << "OwnerId=" << x.ownerId << ", " << "Info=" << x.info << ")";
    return out;
}

void CreateDirectoryInodeResp::pack(BincodeBuf& buf) const {
    mtime.pack(buf);
}
void CreateDirectoryInodeResp::unpack(BincodeBuf& buf) {
    mtime.unpack(buf);
}
void CreateDirectoryInodeResp::clear() {
    mtime = TernTime();
}
bool CreateDirectoryInodeResp::operator==(const CreateDirectoryInodeResp& rhs) const {
    if ((TernTime)this->mtime != (TernTime)rhs.mtime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CreateDirectoryInodeResp& x) {
    out << "CreateDirectoryInodeResp(" << "Mtime=" << x.mtime << ")";
    return out;
}

void SetDirectoryOwnerReq::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    ownerId.pack(buf);
}
void SetDirectoryOwnerReq::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    ownerId.unpack(buf);
}
void SetDirectoryOwnerReq::clear() {
    dirId = InodeId();
    ownerId = InodeId();
}
bool SetDirectoryOwnerReq::operator==(const SetDirectoryOwnerReq& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetDirectoryOwnerReq& x) {
    out << "SetDirectoryOwnerReq(" << "DirId=" << x.dirId << ", " << "OwnerId=" << x.ownerId << ")";
    return out;
}

void SetDirectoryOwnerResp::pack(BincodeBuf& buf) const {
}
void SetDirectoryOwnerResp::unpack(BincodeBuf& buf) {
}
void SetDirectoryOwnerResp::clear() {
}
bool SetDirectoryOwnerResp::operator==(const SetDirectoryOwnerResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetDirectoryOwnerResp& x) {
    out << "SetDirectoryOwnerResp(" << ")";
    return out;
}

void RemoveDirectoryOwnerReq::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    info.pack(buf);
}
void RemoveDirectoryOwnerReq::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    info.unpack(buf);
}
void RemoveDirectoryOwnerReq::clear() {
    dirId = InodeId();
    info.clear();
}
bool RemoveDirectoryOwnerReq::operator==(const RemoveDirectoryOwnerReq& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (info != rhs.info) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveDirectoryOwnerReq& x) {
    out << "RemoveDirectoryOwnerReq(" << "DirId=" << x.dirId << ", " << "Info=" << x.info << ")";
    return out;
}

void RemoveDirectoryOwnerResp::pack(BincodeBuf& buf) const {
}
void RemoveDirectoryOwnerResp::unpack(BincodeBuf& buf) {
}
void RemoveDirectoryOwnerResp::clear() {
}
bool RemoveDirectoryOwnerResp::operator==(const RemoveDirectoryOwnerResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveDirectoryOwnerResp& x) {
    out << "RemoveDirectoryOwnerResp(" << ")";
    return out;
}

void CreateLockedCurrentEdgeReq::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    buf.packBytes(name);
    targetId.pack(buf);
    oldCreationTime.pack(buf);
}
void CreateLockedCurrentEdgeReq::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    buf.unpackBytes(name);
    targetId.unpack(buf);
    oldCreationTime.unpack(buf);
}
void CreateLockedCurrentEdgeReq::clear() {
    dirId = InodeId();
    name.clear();
    targetId = InodeId();
    oldCreationTime = TernTime();
}
bool CreateLockedCurrentEdgeReq::operator==(const CreateLockedCurrentEdgeReq& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (name != rhs.name) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((TernTime)this->oldCreationTime != (TernTime)rhs.oldCreationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CreateLockedCurrentEdgeReq& x) {
    out << "CreateLockedCurrentEdgeReq(" << "DirId=" << x.dirId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "TargetId=" << x.targetId << ", " << "OldCreationTime=" << x.oldCreationTime << ")";
    return out;
}

void CreateLockedCurrentEdgeResp::pack(BincodeBuf& buf) const {
    creationTime.pack(buf);
}
void CreateLockedCurrentEdgeResp::unpack(BincodeBuf& buf) {
    creationTime.unpack(buf);
}
void CreateLockedCurrentEdgeResp::clear() {
    creationTime = TernTime();
}
bool CreateLockedCurrentEdgeResp::operator==(const CreateLockedCurrentEdgeResp& rhs) const {
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CreateLockedCurrentEdgeResp& x) {
    out << "CreateLockedCurrentEdgeResp(" << "CreationTime=" << x.creationTime << ")";
    return out;
}

void LockCurrentEdgeReq::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    targetId.pack(buf);
    creationTime.pack(buf);
    buf.packBytes(name);
}
void LockCurrentEdgeReq::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    targetId.unpack(buf);
    creationTime.unpack(buf);
    buf.unpackBytes(name);
}
void LockCurrentEdgeReq::clear() {
    dirId = InodeId();
    targetId = InodeId();
    creationTime = TernTime();
    name.clear();
}
bool LockCurrentEdgeReq::operator==(const LockCurrentEdgeReq& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    if (name != rhs.name) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LockCurrentEdgeReq& x) {
    out << "LockCurrentEdgeReq(" << "DirId=" << x.dirId << ", " << "TargetId=" << x.targetId << ", " << "CreationTime=" << x.creationTime << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ")";
    return out;
}

void LockCurrentEdgeResp::pack(BincodeBuf& buf) const {
}
void LockCurrentEdgeResp::unpack(BincodeBuf& buf) {
}
void LockCurrentEdgeResp::clear() {
}
bool LockCurrentEdgeResp::operator==(const LockCurrentEdgeResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const LockCurrentEdgeResp& x) {
    out << "LockCurrentEdgeResp(" << ")";
    return out;
}

void UnlockCurrentEdgeReq::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
    targetId.pack(buf);
    buf.packScalar<bool>(wasMoved);
}
void UnlockCurrentEdgeReq::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
    targetId.unpack(buf);
    wasMoved = buf.unpackScalar<bool>();
}
void UnlockCurrentEdgeReq::clear() {
    dirId = InodeId();
    name.clear();
    creationTime = TernTime();
    targetId = InodeId();
    wasMoved = bool(0);
}
bool UnlockCurrentEdgeReq::operator==(const UnlockCurrentEdgeReq& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((bool)this->wasMoved != (bool)rhs.wasMoved) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const UnlockCurrentEdgeReq& x) {
    out << "UnlockCurrentEdgeReq(" << "DirId=" << x.dirId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ", " << "TargetId=" << x.targetId << ", " << "WasMoved=" << x.wasMoved << ")";
    return out;
}

void UnlockCurrentEdgeResp::pack(BincodeBuf& buf) const {
}
void UnlockCurrentEdgeResp::unpack(BincodeBuf& buf) {
}
void UnlockCurrentEdgeResp::clear() {
}
bool UnlockCurrentEdgeResp::operator==(const UnlockCurrentEdgeResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const UnlockCurrentEdgeResp& x) {
    out << "UnlockCurrentEdgeResp(" << ")";
    return out;
}

void RemoveOwnedSnapshotFileEdgeReq::pack(BincodeBuf& buf) const {
    ownerId.pack(buf);
    targetId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
}
void RemoveOwnedSnapshotFileEdgeReq::unpack(BincodeBuf& buf) {
    ownerId.unpack(buf);
    targetId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
}
void RemoveOwnedSnapshotFileEdgeReq::clear() {
    ownerId = InodeId();
    targetId = InodeId();
    name.clear();
    creationTime = TernTime();
}
bool RemoveOwnedSnapshotFileEdgeReq::operator==(const RemoveOwnedSnapshotFileEdgeReq& rhs) const {
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveOwnedSnapshotFileEdgeReq& x) {
    out << "RemoveOwnedSnapshotFileEdgeReq(" << "OwnerId=" << x.ownerId << ", " << "TargetId=" << x.targetId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void RemoveOwnedSnapshotFileEdgeResp::pack(BincodeBuf& buf) const {
}
void RemoveOwnedSnapshotFileEdgeResp::unpack(BincodeBuf& buf) {
}
void RemoveOwnedSnapshotFileEdgeResp::clear() {
}
bool RemoveOwnedSnapshotFileEdgeResp::operator==(const RemoveOwnedSnapshotFileEdgeResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveOwnedSnapshotFileEdgeResp& x) {
    out << "RemoveOwnedSnapshotFileEdgeResp(" << ")";
    return out;
}

void MakeFileTransientReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packBytes(note);
}
void MakeFileTransientReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    buf.unpackBytes(note);
}
void MakeFileTransientReq::clear() {
    id = InodeId();
    note.clear();
}
bool MakeFileTransientReq::operator==(const MakeFileTransientReq& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if (note != rhs.note) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const MakeFileTransientReq& x) {
    out << "MakeFileTransientReq(" << "Id=" << x.id << ", " << "Note=" << GoLangQuotedStringFmt(x.note.data(), x.note.size()) << ")";
    return out;
}

void MakeFileTransientResp::pack(BincodeBuf& buf) const {
}
void MakeFileTransientResp::unpack(BincodeBuf& buf) {
}
void MakeFileTransientResp::clear() {
}
bool MakeFileTransientResp::operator==(const MakeFileTransientResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const MakeFileTransientResp& x) {
    out << "MakeFileTransientResp(" << ")";
    return out;
}

void MakeDirectoryReq::pack(BincodeBuf& buf) const {
    ownerId.pack(buf);
    buf.packBytes(name);
}
void MakeDirectoryReq::unpack(BincodeBuf& buf) {
    ownerId.unpack(buf);
    buf.unpackBytes(name);
}
void MakeDirectoryReq::clear() {
    ownerId = InodeId();
    name.clear();
}
bool MakeDirectoryReq::operator==(const MakeDirectoryReq& rhs) const {
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if (name != rhs.name) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const MakeDirectoryReq& x) {
    out << "MakeDirectoryReq(" << "OwnerId=" << x.ownerId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ")";
    return out;
}

void MakeDirectoryResp::pack(BincodeBuf& buf) const {
    id.pack(buf);
    creationTime.pack(buf);
}
void MakeDirectoryResp::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    creationTime.unpack(buf);
}
void MakeDirectoryResp::clear() {
    id = InodeId();
    creationTime = TernTime();
}
bool MakeDirectoryResp::operator==(const MakeDirectoryResp& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const MakeDirectoryResp& x) {
    out << "MakeDirectoryResp(" << "Id=" << x.id << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void RenameFileReq::pack(BincodeBuf& buf) const {
    targetId.pack(buf);
    oldOwnerId.pack(buf);
    buf.packBytes(oldName);
    oldCreationTime.pack(buf);
    newOwnerId.pack(buf);
    buf.packBytes(newName);
}
void RenameFileReq::unpack(BincodeBuf& buf) {
    targetId.unpack(buf);
    oldOwnerId.unpack(buf);
    buf.unpackBytes(oldName);
    oldCreationTime.unpack(buf);
    newOwnerId.unpack(buf);
    buf.unpackBytes(newName);
}
void RenameFileReq::clear() {
    targetId = InodeId();
    oldOwnerId = InodeId();
    oldName.clear();
    oldCreationTime = TernTime();
    newOwnerId = InodeId();
    newName.clear();
}
bool RenameFileReq::operator==(const RenameFileReq& rhs) const {
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((InodeId)this->oldOwnerId != (InodeId)rhs.oldOwnerId) { return false; };
    if (oldName != rhs.oldName) { return false; };
    if ((TernTime)this->oldCreationTime != (TernTime)rhs.oldCreationTime) { return false; };
    if ((InodeId)this->newOwnerId != (InodeId)rhs.newOwnerId) { return false; };
    if (newName != rhs.newName) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RenameFileReq& x) {
    out << "RenameFileReq(" << "TargetId=" << x.targetId << ", " << "OldOwnerId=" << x.oldOwnerId << ", " << "OldName=" << GoLangQuotedStringFmt(x.oldName.data(), x.oldName.size()) << ", " << "OldCreationTime=" << x.oldCreationTime << ", " << "NewOwnerId=" << x.newOwnerId << ", " << "NewName=" << GoLangQuotedStringFmt(x.newName.data(), x.newName.size()) << ")";
    return out;
}

void RenameFileResp::pack(BincodeBuf& buf) const {
    creationTime.pack(buf);
}
void RenameFileResp::unpack(BincodeBuf& buf) {
    creationTime.unpack(buf);
}
void RenameFileResp::clear() {
    creationTime = TernTime();
}
bool RenameFileResp::operator==(const RenameFileResp& rhs) const {
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RenameFileResp& x) {
    out << "RenameFileResp(" << "CreationTime=" << x.creationTime << ")";
    return out;
}

void SoftUnlinkDirectoryReq::pack(BincodeBuf& buf) const {
    ownerId.pack(buf);
    targetId.pack(buf);
    creationTime.pack(buf);
    buf.packBytes(name);
}
void SoftUnlinkDirectoryReq::unpack(BincodeBuf& buf) {
    ownerId.unpack(buf);
    targetId.unpack(buf);
    creationTime.unpack(buf);
    buf.unpackBytes(name);
}
void SoftUnlinkDirectoryReq::clear() {
    ownerId = InodeId();
    targetId = InodeId();
    creationTime = TernTime();
    name.clear();
}
bool SoftUnlinkDirectoryReq::operator==(const SoftUnlinkDirectoryReq& rhs) const {
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    if (name != rhs.name) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SoftUnlinkDirectoryReq& x) {
    out << "SoftUnlinkDirectoryReq(" << "OwnerId=" << x.ownerId << ", " << "TargetId=" << x.targetId << ", " << "CreationTime=" << x.creationTime << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ")";
    return out;
}

void SoftUnlinkDirectoryResp::pack(BincodeBuf& buf) const {
}
void SoftUnlinkDirectoryResp::unpack(BincodeBuf& buf) {
}
void SoftUnlinkDirectoryResp::clear() {
}
bool SoftUnlinkDirectoryResp::operator==(const SoftUnlinkDirectoryResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const SoftUnlinkDirectoryResp& x) {
    out << "SoftUnlinkDirectoryResp(" << ")";
    return out;
}

void RenameDirectoryReq::pack(BincodeBuf& buf) const {
    targetId.pack(buf);
    oldOwnerId.pack(buf);
    buf.packBytes(oldName);
    oldCreationTime.pack(buf);
    newOwnerId.pack(buf);
    buf.packBytes(newName);
}
void RenameDirectoryReq::unpack(BincodeBuf& buf) {
    targetId.unpack(buf);
    oldOwnerId.unpack(buf);
    buf.unpackBytes(oldName);
    oldCreationTime.unpack(buf);
    newOwnerId.unpack(buf);
    buf.unpackBytes(newName);
}
void RenameDirectoryReq::clear() {
    targetId = InodeId();
    oldOwnerId = InodeId();
    oldName.clear();
    oldCreationTime = TernTime();
    newOwnerId = InodeId();
    newName.clear();
}
bool RenameDirectoryReq::operator==(const RenameDirectoryReq& rhs) const {
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((InodeId)this->oldOwnerId != (InodeId)rhs.oldOwnerId) { return false; };
    if (oldName != rhs.oldName) { return false; };
    if ((TernTime)this->oldCreationTime != (TernTime)rhs.oldCreationTime) { return false; };
    if ((InodeId)this->newOwnerId != (InodeId)rhs.newOwnerId) { return false; };
    if (newName != rhs.newName) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RenameDirectoryReq& x) {
    out << "RenameDirectoryReq(" << "TargetId=" << x.targetId << ", " << "OldOwnerId=" << x.oldOwnerId << ", " << "OldName=" << GoLangQuotedStringFmt(x.oldName.data(), x.oldName.size()) << ", " << "OldCreationTime=" << x.oldCreationTime << ", " << "NewOwnerId=" << x.newOwnerId << ", " << "NewName=" << GoLangQuotedStringFmt(x.newName.data(), x.newName.size()) << ")";
    return out;
}

void RenameDirectoryResp::pack(BincodeBuf& buf) const {
    creationTime.pack(buf);
}
void RenameDirectoryResp::unpack(BincodeBuf& buf) {
    creationTime.unpack(buf);
}
void RenameDirectoryResp::clear() {
    creationTime = TernTime();
}
bool RenameDirectoryResp::operator==(const RenameDirectoryResp& rhs) const {
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RenameDirectoryResp& x) {
    out << "RenameDirectoryResp(" << "CreationTime=" << x.creationTime << ")";
    return out;
}

void HardUnlinkDirectoryReq::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
}
void HardUnlinkDirectoryReq::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
}
void HardUnlinkDirectoryReq::clear() {
    dirId = InodeId();
}
bool HardUnlinkDirectoryReq::operator==(const HardUnlinkDirectoryReq& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const HardUnlinkDirectoryReq& x) {
    out << "HardUnlinkDirectoryReq(" << "DirId=" << x.dirId << ")";
    return out;
}

void HardUnlinkDirectoryResp::pack(BincodeBuf& buf) const {
}
void HardUnlinkDirectoryResp::unpack(BincodeBuf& buf) {
}
void HardUnlinkDirectoryResp::clear() {
}
bool HardUnlinkDirectoryResp::operator==(const HardUnlinkDirectoryResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const HardUnlinkDirectoryResp& x) {
    out << "HardUnlinkDirectoryResp(" << ")";
    return out;
}

void CrossShardHardUnlinkFileReq::pack(BincodeBuf& buf) const {
    ownerId.pack(buf);
    targetId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
}
void CrossShardHardUnlinkFileReq::unpack(BincodeBuf& buf) {
    ownerId.unpack(buf);
    targetId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
}
void CrossShardHardUnlinkFileReq::clear() {
    ownerId = InodeId();
    targetId = InodeId();
    name.clear();
    creationTime = TernTime();
}
bool CrossShardHardUnlinkFileReq::operator==(const CrossShardHardUnlinkFileReq& rhs) const {
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CrossShardHardUnlinkFileReq& x) {
    out << "CrossShardHardUnlinkFileReq(" << "OwnerId=" << x.ownerId << ", " << "TargetId=" << x.targetId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void CrossShardHardUnlinkFileResp::pack(BincodeBuf& buf) const {
}
void CrossShardHardUnlinkFileResp::unpack(BincodeBuf& buf) {
}
void CrossShardHardUnlinkFileResp::clear() {
}
bool CrossShardHardUnlinkFileResp::operator==(const CrossShardHardUnlinkFileResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const CrossShardHardUnlinkFileResp& x) {
    out << "CrossShardHardUnlinkFileResp(" << ")";
    return out;
}

void CdcSnapshotReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(snapshotId);
}
void CdcSnapshotReq::unpack(BincodeBuf& buf) {
    snapshotId = buf.unpackScalar<uint64_t>();
}
void CdcSnapshotReq::clear() {
    snapshotId = uint64_t(0);
}
bool CdcSnapshotReq::operator==(const CdcSnapshotReq& rhs) const {
    if ((uint64_t)this->snapshotId != (uint64_t)rhs.snapshotId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CdcSnapshotReq& x) {
    out << "CdcSnapshotReq(" << "SnapshotId=" << x.snapshotId << ")";
    return out;
}

void CdcSnapshotResp::pack(BincodeBuf& buf) const {
}
void CdcSnapshotResp::unpack(BincodeBuf& buf) {
}
void CdcSnapshotResp::clear() {
}
bool CdcSnapshotResp::operator==(const CdcSnapshotResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const CdcSnapshotResp& x) {
    out << "CdcSnapshotResp(" << ")";
    return out;
}

void LocalShardsReq::pack(BincodeBuf& buf) const {
}
void LocalShardsReq::unpack(BincodeBuf& buf) {
}
void LocalShardsReq::clear() {
}
bool LocalShardsReq::operator==(const LocalShardsReq& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocalShardsReq& x) {
    out << "LocalShardsReq(" << ")";
    return out;
}

void LocalShardsResp::pack(BincodeBuf& buf) const {
    buf.packList<ShardInfo>(shards);
}
void LocalShardsResp::unpack(BincodeBuf& buf) {
    buf.unpackList<ShardInfo>(shards);
}
void LocalShardsResp::clear() {
    shards.clear();
}
bool LocalShardsResp::operator==(const LocalShardsResp& rhs) const {
    if (shards != rhs.shards) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocalShardsResp& x) {
    out << "LocalShardsResp(" << "Shards=" << x.shards << ")";
    return out;
}

void LocalCdcReq::pack(BincodeBuf& buf) const {
}
void LocalCdcReq::unpack(BincodeBuf& buf) {
}
void LocalCdcReq::clear() {
}
bool LocalCdcReq::operator==(const LocalCdcReq& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocalCdcReq& x) {
    out << "LocalCdcReq(" << ")";
    return out;
}

void LocalCdcResp::pack(BincodeBuf& buf) const {
    addrs.pack(buf);
    lastSeen.pack(buf);
}
void LocalCdcResp::unpack(BincodeBuf& buf) {
    addrs.unpack(buf);
    lastSeen.unpack(buf);
}
void LocalCdcResp::clear() {
    addrs.clear();
    lastSeen = TernTime();
}
bool LocalCdcResp::operator==(const LocalCdcResp& rhs) const {
    if (addrs != rhs.addrs) { return false; };
    if ((TernTime)this->lastSeen != (TernTime)rhs.lastSeen) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocalCdcResp& x) {
    out << "LocalCdcResp(" << "Addrs=" << x.addrs << ", " << "LastSeen=" << x.lastSeen << ")";
    return out;
}

void InfoReq::pack(BincodeBuf& buf) const {
}
void InfoReq::unpack(BincodeBuf& buf) {
}
void InfoReq::clear() {
}
bool InfoReq::operator==(const InfoReq& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const InfoReq& x) {
    out << "InfoReq(" << ")";
    return out;
}

void InfoResp::pack(BincodeBuf& buf) const {
    buf.packScalar<uint32_t>(numBlockServices);
    buf.packScalar<uint32_t>(numFailureDomains);
    buf.packScalar<uint64_t>(capacity);
    buf.packScalar<uint64_t>(available);
    buf.packScalar<uint64_t>(blocks);
}
void InfoResp::unpack(BincodeBuf& buf) {
    numBlockServices = buf.unpackScalar<uint32_t>();
    numFailureDomains = buf.unpackScalar<uint32_t>();
    capacity = buf.unpackScalar<uint64_t>();
    available = buf.unpackScalar<uint64_t>();
    blocks = buf.unpackScalar<uint64_t>();
}
void InfoResp::clear() {
    numBlockServices = uint32_t(0);
    numFailureDomains = uint32_t(0);
    capacity = uint64_t(0);
    available = uint64_t(0);
    blocks = uint64_t(0);
}
bool InfoResp::operator==(const InfoResp& rhs) const {
    if ((uint32_t)this->numBlockServices != (uint32_t)rhs.numBlockServices) { return false; };
    if ((uint32_t)this->numFailureDomains != (uint32_t)rhs.numFailureDomains) { return false; };
    if ((uint64_t)this->capacity != (uint64_t)rhs.capacity) { return false; };
    if ((uint64_t)this->available != (uint64_t)rhs.available) { return false; };
    if ((uint64_t)this->blocks != (uint64_t)rhs.blocks) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const InfoResp& x) {
    out << "InfoResp(" << "NumBlockServices=" << x.numBlockServices << ", " << "NumFailureDomains=" << x.numFailureDomains << ", " << "Capacity=" << x.capacity << ", " << "Available=" << x.available << ", " << "Blocks=" << x.blocks << ")";
    return out;
}

void RegistryReq::pack(BincodeBuf& buf) const {
}
void RegistryReq::unpack(BincodeBuf& buf) {
}
void RegistryReq::clear() {
}
bool RegistryReq::operator==(const RegistryReq& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegistryReq& x) {
    out << "RegistryReq(" << ")";
    return out;
}

void RegistryResp::pack(BincodeBuf& buf) const {
    addrs.pack(buf);
}
void RegistryResp::unpack(BincodeBuf& buf) {
    addrs.unpack(buf);
}
void RegistryResp::clear() {
    addrs.clear();
}
bool RegistryResp::operator==(const RegistryResp& rhs) const {
    if (addrs != rhs.addrs) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegistryResp& x) {
    out << "RegistryResp(" << "Addrs=" << x.addrs << ")";
    return out;
}

void LocalChangedBlockServicesReq::pack(BincodeBuf& buf) const {
    changedSince.pack(buf);
}
void LocalChangedBlockServicesReq::unpack(BincodeBuf& buf) {
    changedSince.unpack(buf);
}
void LocalChangedBlockServicesReq::clear() {
    changedSince = TernTime();
}
bool LocalChangedBlockServicesReq::operator==(const LocalChangedBlockServicesReq& rhs) const {
    if ((TernTime)this->changedSince != (TernTime)rhs.changedSince) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocalChangedBlockServicesReq& x) {
    out << "LocalChangedBlockServicesReq(" << "ChangedSince=" << x.changedSince << ")";
    return out;
}

void LocalChangedBlockServicesResp::pack(BincodeBuf& buf) const {
    lastChange.pack(buf);
    buf.packList<BlockService>(blockServices);
}
void LocalChangedBlockServicesResp::unpack(BincodeBuf& buf) {
    lastChange.unpack(buf);
    buf.unpackList<BlockService>(blockServices);
}
void LocalChangedBlockServicesResp::clear() {
    lastChange = TernTime();
    blockServices.clear();
}
bool LocalChangedBlockServicesResp::operator==(const LocalChangedBlockServicesResp& rhs) const {
    if ((TernTime)this->lastChange != (TernTime)rhs.lastChange) { return false; };
    if (blockServices != rhs.blockServices) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocalChangedBlockServicesResp& x) {
    out << "LocalChangedBlockServicesResp(" << "LastChange=" << x.lastChange << ", " << "BlockServices=" << x.blockServices << ")";
    return out;
}

void CreateLocationReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(id);
    buf.packBytes(name);
}
void CreateLocationReq::unpack(BincodeBuf& buf) {
    id = buf.unpackScalar<uint8_t>();
    buf.unpackBytes(name);
}
void CreateLocationReq::clear() {
    id = uint8_t(0);
    name.clear();
}
bool CreateLocationReq::operator==(const CreateLocationReq& rhs) const {
    if ((uint8_t)this->id != (uint8_t)rhs.id) { return false; };
    if (name != rhs.name) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CreateLocationReq& x) {
    out << "CreateLocationReq(" << "Id=" << (int)x.id << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ")";
    return out;
}

void CreateLocationResp::pack(BincodeBuf& buf) const {
}
void CreateLocationResp::unpack(BincodeBuf& buf) {
}
void CreateLocationResp::clear() {
}
bool CreateLocationResp::operator==(const CreateLocationResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const CreateLocationResp& x) {
    out << "CreateLocationResp(" << ")";
    return out;
}

void RenameLocationReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(id);
    buf.packBytes(name);
}
void RenameLocationReq::unpack(BincodeBuf& buf) {
    id = buf.unpackScalar<uint8_t>();
    buf.unpackBytes(name);
}
void RenameLocationReq::clear() {
    id = uint8_t(0);
    name.clear();
}
bool RenameLocationReq::operator==(const RenameLocationReq& rhs) const {
    if ((uint8_t)this->id != (uint8_t)rhs.id) { return false; };
    if (name != rhs.name) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RenameLocationReq& x) {
    out << "RenameLocationReq(" << "Id=" << (int)x.id << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ")";
    return out;
}

void RenameLocationResp::pack(BincodeBuf& buf) const {
}
void RenameLocationResp::unpack(BincodeBuf& buf) {
}
void RenameLocationResp::clear() {
}
bool RenameLocationResp::operator==(const RenameLocationResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RenameLocationResp& x) {
    out << "RenameLocationResp(" << ")";
    return out;
}

void RegisterShardReq::pack(BincodeBuf& buf) const {
    shrid.pack(buf);
    buf.packScalar<bool>(isLeader);
    addrs.pack(buf);
    buf.packScalar<uint8_t>(location);
}
void RegisterShardReq::unpack(BincodeBuf& buf) {
    shrid.unpack(buf);
    isLeader = buf.unpackScalar<bool>();
    addrs.unpack(buf);
    location = buf.unpackScalar<uint8_t>();
}
void RegisterShardReq::clear() {
    shrid = ShardReplicaId();
    isLeader = bool(0);
    addrs.clear();
    location = uint8_t(0);
}
bool RegisterShardReq::operator==(const RegisterShardReq& rhs) const {
    if ((ShardReplicaId)this->shrid != (ShardReplicaId)rhs.shrid) { return false; };
    if ((bool)this->isLeader != (bool)rhs.isLeader) { return false; };
    if (addrs != rhs.addrs) { return false; };
    if ((uint8_t)this->location != (uint8_t)rhs.location) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegisterShardReq& x) {
    out << "RegisterShardReq(" << "Shrid=" << x.shrid << ", " << "IsLeader=" << x.isLeader << ", " << "Addrs=" << x.addrs << ", " << "Location=" << (int)x.location << ")";
    return out;
}

void RegisterShardResp::pack(BincodeBuf& buf) const {
}
void RegisterShardResp::unpack(BincodeBuf& buf) {
}
void RegisterShardResp::clear() {
}
bool RegisterShardResp::operator==(const RegisterShardResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegisterShardResp& x) {
    out << "RegisterShardResp(" << ")";
    return out;
}

void LocationsReq::pack(BincodeBuf& buf) const {
}
void LocationsReq::unpack(BincodeBuf& buf) {
}
void LocationsReq::clear() {
}
bool LocationsReq::operator==(const LocationsReq& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocationsReq& x) {
    out << "LocationsReq(" << ")";
    return out;
}

void LocationsResp::pack(BincodeBuf& buf) const {
    buf.packList<LocationInfo>(locations);
}
void LocationsResp::unpack(BincodeBuf& buf) {
    buf.unpackList<LocationInfo>(locations);
}
void LocationsResp::clear() {
    locations.clear();
}
bool LocationsResp::operator==(const LocationsResp& rhs) const {
    if (locations != rhs.locations) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LocationsResp& x) {
    out << "LocationsResp(" << "Locations=" << x.locations << ")";
    return out;
}

void RegisterCdcReq::pack(BincodeBuf& buf) const {
    replica.pack(buf);
    buf.packScalar<uint8_t>(location);
    buf.packScalar<bool>(isLeader);
    addrs.pack(buf);
}
void RegisterCdcReq::unpack(BincodeBuf& buf) {
    replica.unpack(buf);
    location = buf.unpackScalar<uint8_t>();
    isLeader = buf.unpackScalar<bool>();
    addrs.unpack(buf);
}
void RegisterCdcReq::clear() {
    replica = ReplicaId();
    location = uint8_t(0);
    isLeader = bool(0);
    addrs.clear();
}
bool RegisterCdcReq::operator==(const RegisterCdcReq& rhs) const {
    if ((ReplicaId)this->replica != (ReplicaId)rhs.replica) { return false; };
    if ((uint8_t)this->location != (uint8_t)rhs.location) { return false; };
    if ((bool)this->isLeader != (bool)rhs.isLeader) { return false; };
    if (addrs != rhs.addrs) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegisterCdcReq& x) {
    out << "RegisterCdcReq(" << "Replica=" << x.replica << ", " << "Location=" << (int)x.location << ", " << "IsLeader=" << x.isLeader << ", " << "Addrs=" << x.addrs << ")";
    return out;
}

void RegisterCdcResp::pack(BincodeBuf& buf) const {
}
void RegisterCdcResp::unpack(BincodeBuf& buf) {
}
void RegisterCdcResp::clear() {
}
bool RegisterCdcResp::operator==(const RegisterCdcResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegisterCdcResp& x) {
    out << "RegisterCdcResp(" << ")";
    return out;
}

void SetBlockServiceFlagsReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packScalar<BlockServiceFlags>(flags);
    buf.packScalar<uint8_t>(flagsMask);
}
void SetBlockServiceFlagsReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    flags = buf.unpackScalar<BlockServiceFlags>();
    flagsMask = buf.unpackScalar<uint8_t>();
}
void SetBlockServiceFlagsReq::clear() {
    id = BlockServiceId(0);
    flags = BlockServiceFlags(0);
    flagsMask = uint8_t(0);
}
bool SetBlockServiceFlagsReq::operator==(const SetBlockServiceFlagsReq& rhs) const {
    if ((BlockServiceId)this->id != (BlockServiceId)rhs.id) { return false; };
    if ((BlockServiceFlags)this->flags != (BlockServiceFlags)rhs.flags) { return false; };
    if ((uint8_t)this->flagsMask != (uint8_t)rhs.flagsMask) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetBlockServiceFlagsReq& x) {
    out << "SetBlockServiceFlagsReq(" << "Id=" << x.id << ", " << "Flags=" << x.flags << ", " << "FlagsMask=" << (int)x.flagsMask << ")";
    return out;
}

void SetBlockServiceFlagsResp::pack(BincodeBuf& buf) const {
}
void SetBlockServiceFlagsResp::unpack(BincodeBuf& buf) {
}
void SetBlockServiceFlagsResp::clear() {
}
bool SetBlockServiceFlagsResp::operator==(const SetBlockServiceFlagsResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetBlockServiceFlagsResp& x) {
    out << "SetBlockServiceFlagsResp(" << ")";
    return out;
}

void RegisterBlockServicesReq::pack(BincodeBuf& buf) const {
    buf.packList<RegisterBlockServiceInfo>(blockServices);
}
void RegisterBlockServicesReq::unpack(BincodeBuf& buf) {
    buf.unpackList<RegisterBlockServiceInfo>(blockServices);
}
void RegisterBlockServicesReq::clear() {
    blockServices.clear();
}
bool RegisterBlockServicesReq::operator==(const RegisterBlockServicesReq& rhs) const {
    if (blockServices != rhs.blockServices) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegisterBlockServicesReq& x) {
    out << "RegisterBlockServicesReq(" << "BlockServices=" << x.blockServices << ")";
    return out;
}

void RegisterBlockServicesResp::pack(BincodeBuf& buf) const {
}
void RegisterBlockServicesResp::unpack(BincodeBuf& buf) {
}
void RegisterBlockServicesResp::clear() {
}
bool RegisterBlockServicesResp::operator==(const RegisterBlockServicesResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegisterBlockServicesResp& x) {
    out << "RegisterBlockServicesResp(" << ")";
    return out;
}

void ChangedBlockServicesAtLocationReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(locationId);
    changedSince.pack(buf);
}
void ChangedBlockServicesAtLocationReq::unpack(BincodeBuf& buf) {
    locationId = buf.unpackScalar<uint8_t>();
    changedSince.unpack(buf);
}
void ChangedBlockServicesAtLocationReq::clear() {
    locationId = uint8_t(0);
    changedSince = TernTime();
}
bool ChangedBlockServicesAtLocationReq::operator==(const ChangedBlockServicesAtLocationReq& rhs) const {
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    if ((TernTime)this->changedSince != (TernTime)rhs.changedSince) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ChangedBlockServicesAtLocationReq& x) {
    out << "ChangedBlockServicesAtLocationReq(" << "LocationId=" << (int)x.locationId << ", " << "ChangedSince=" << x.changedSince << ")";
    return out;
}

void ChangedBlockServicesAtLocationResp::pack(BincodeBuf& buf) const {
    lastChange.pack(buf);
    buf.packList<BlockService>(blockServices);
}
void ChangedBlockServicesAtLocationResp::unpack(BincodeBuf& buf) {
    lastChange.unpack(buf);
    buf.unpackList<BlockService>(blockServices);
}
void ChangedBlockServicesAtLocationResp::clear() {
    lastChange = TernTime();
    blockServices.clear();
}
bool ChangedBlockServicesAtLocationResp::operator==(const ChangedBlockServicesAtLocationResp& rhs) const {
    if ((TernTime)this->lastChange != (TernTime)rhs.lastChange) { return false; };
    if (blockServices != rhs.blockServices) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ChangedBlockServicesAtLocationResp& x) {
    out << "ChangedBlockServicesAtLocationResp(" << "LastChange=" << x.lastChange << ", " << "BlockServices=" << x.blockServices << ")";
    return out;
}

void ShardsAtLocationReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(locationId);
}
void ShardsAtLocationReq::unpack(BincodeBuf& buf) {
    locationId = buf.unpackScalar<uint8_t>();
}
void ShardsAtLocationReq::clear() {
    locationId = uint8_t(0);
}
bool ShardsAtLocationReq::operator==(const ShardsAtLocationReq& rhs) const {
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ShardsAtLocationReq& x) {
    out << "ShardsAtLocationReq(" << "LocationId=" << (int)x.locationId << ")";
    return out;
}

void ShardsAtLocationResp::pack(BincodeBuf& buf) const {
    buf.packList<ShardInfo>(shards);
}
void ShardsAtLocationResp::unpack(BincodeBuf& buf) {
    buf.unpackList<ShardInfo>(shards);
}
void ShardsAtLocationResp::clear() {
    shards.clear();
}
bool ShardsAtLocationResp::operator==(const ShardsAtLocationResp& rhs) const {
    if (shards != rhs.shards) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ShardsAtLocationResp& x) {
    out << "ShardsAtLocationResp(" << "Shards=" << x.shards << ")";
    return out;
}

void CdcAtLocationReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(locationId);
}
void CdcAtLocationReq::unpack(BincodeBuf& buf) {
    locationId = buf.unpackScalar<uint8_t>();
}
void CdcAtLocationReq::clear() {
    locationId = uint8_t(0);
}
bool CdcAtLocationReq::operator==(const CdcAtLocationReq& rhs) const {
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CdcAtLocationReq& x) {
    out << "CdcAtLocationReq(" << "LocationId=" << (int)x.locationId << ")";
    return out;
}

void CdcAtLocationResp::pack(BincodeBuf& buf) const {
    addrs.pack(buf);
    lastSeen.pack(buf);
}
void CdcAtLocationResp::unpack(BincodeBuf& buf) {
    addrs.unpack(buf);
    lastSeen.unpack(buf);
}
void CdcAtLocationResp::clear() {
    addrs.clear();
    lastSeen = TernTime();
}
bool CdcAtLocationResp::operator==(const CdcAtLocationResp& rhs) const {
    if (addrs != rhs.addrs) { return false; };
    if ((TernTime)this->lastSeen != (TernTime)rhs.lastSeen) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CdcAtLocationResp& x) {
    out << "CdcAtLocationResp(" << "Addrs=" << x.addrs << ", " << "LastSeen=" << x.lastSeen << ")";
    return out;
}

void RegisterRegistryReq::pack(BincodeBuf& buf) const {
    replicaId.pack(buf);
    buf.packScalar<uint8_t>(location);
    buf.packScalar<bool>(isLeader);
    addrs.pack(buf);
    buf.packScalar<bool>(bootstrap);
}
void RegisterRegistryReq::unpack(BincodeBuf& buf) {
    replicaId.unpack(buf);
    location = buf.unpackScalar<uint8_t>();
    isLeader = buf.unpackScalar<bool>();
    addrs.unpack(buf);
    bootstrap = buf.unpackScalar<bool>();
}
void RegisterRegistryReq::clear() {
    replicaId = ReplicaId();
    location = uint8_t(0);
    isLeader = bool(0);
    addrs.clear();
    bootstrap = bool(0);
}
bool RegisterRegistryReq::operator==(const RegisterRegistryReq& rhs) const {
    if ((ReplicaId)this->replicaId != (ReplicaId)rhs.replicaId) { return false; };
    if ((uint8_t)this->location != (uint8_t)rhs.location) { return false; };
    if ((bool)this->isLeader != (bool)rhs.isLeader) { return false; };
    if (addrs != rhs.addrs) { return false; };
    if ((bool)this->bootstrap != (bool)rhs.bootstrap) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegisterRegistryReq& x) {
    out << "RegisterRegistryReq(" << "ReplicaId=" << x.replicaId << ", " << "Location=" << (int)x.location << ", " << "IsLeader=" << x.isLeader << ", " << "Addrs=" << x.addrs << ", " << "Bootstrap=" << x.bootstrap << ")";
    return out;
}

void RegisterRegistryResp::pack(BincodeBuf& buf) const {
}
void RegisterRegistryResp::unpack(BincodeBuf& buf) {
}
void RegisterRegistryResp::clear() {
}
bool RegisterRegistryResp::operator==(const RegisterRegistryResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const RegisterRegistryResp& x) {
    out << "RegisterRegistryResp(" << ")";
    return out;
}

void AllRegistryReplicasReq::pack(BincodeBuf& buf) const {
}
void AllRegistryReplicasReq::unpack(BincodeBuf& buf) {
}
void AllRegistryReplicasReq::clear() {
}
bool AllRegistryReplicasReq::operator==(const AllRegistryReplicasReq& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const AllRegistryReplicasReq& x) {
    out << "AllRegistryReplicasReq(" << ")";
    return out;
}

void AllRegistryReplicasResp::pack(BincodeBuf& buf) const {
    buf.packList<FullRegistryInfo>(replicas);
}
void AllRegistryReplicasResp::unpack(BincodeBuf& buf) {
    buf.unpackList<FullRegistryInfo>(replicas);
}
void AllRegistryReplicasResp::clear() {
    replicas.clear();
}
bool AllRegistryReplicasResp::operator==(const AllRegistryReplicasResp& rhs) const {
    if (replicas != rhs.replicas) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AllRegistryReplicasResp& x) {
    out << "AllRegistryReplicasResp(" << "Replicas=" << x.replicas << ")";
    return out;
}

void ShardBlockServicesDEPRECATEDReq::pack(BincodeBuf& buf) const {
    shardId.pack(buf);
}
void ShardBlockServicesDEPRECATEDReq::unpack(BincodeBuf& buf) {
    shardId.unpack(buf);
}
void ShardBlockServicesDEPRECATEDReq::clear() {
    shardId = ShardId();
}
bool ShardBlockServicesDEPRECATEDReq::operator==(const ShardBlockServicesDEPRECATEDReq& rhs) const {
    if ((ShardId)this->shardId != (ShardId)rhs.shardId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ShardBlockServicesDEPRECATEDReq& x) {
    out << "ShardBlockServicesDEPRECATEDReq(" << "ShardId=" << x.shardId << ")";
    return out;
}

void ShardBlockServicesDEPRECATEDResp::pack(BincodeBuf& buf) const {
    buf.packList<BlockServiceId>(blockServices);
}
void ShardBlockServicesDEPRECATEDResp::unpack(BincodeBuf& buf) {
    buf.unpackList<BlockServiceId>(blockServices);
}
void ShardBlockServicesDEPRECATEDResp::clear() {
    blockServices.clear();
}
bool ShardBlockServicesDEPRECATEDResp::operator==(const ShardBlockServicesDEPRECATEDResp& rhs) const {
    if (blockServices != rhs.blockServices) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ShardBlockServicesDEPRECATEDResp& x) {
    out << "ShardBlockServicesDEPRECATEDResp(" << "BlockServices=" << x.blockServices << ")";
    return out;
}

void CdcReplicasDEPRECATEDReq::pack(BincodeBuf& buf) const {
}
void CdcReplicasDEPRECATEDReq::unpack(BincodeBuf& buf) {
}
void CdcReplicasDEPRECATEDReq::clear() {
}
bool CdcReplicasDEPRECATEDReq::operator==(const CdcReplicasDEPRECATEDReq& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const CdcReplicasDEPRECATEDReq& x) {
    out << "CdcReplicasDEPRECATEDReq(" << ")";
    return out;
}

void CdcReplicasDEPRECATEDResp::pack(BincodeBuf& buf) const {
    buf.packList<AddrsInfo>(replicas);
}
void CdcReplicasDEPRECATEDResp::unpack(BincodeBuf& buf) {
    buf.unpackList<AddrsInfo>(replicas);
}
void CdcReplicasDEPRECATEDResp::clear() {
    replicas.clear();
}
bool CdcReplicasDEPRECATEDResp::operator==(const CdcReplicasDEPRECATEDResp& rhs) const {
    if (replicas != rhs.replicas) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CdcReplicasDEPRECATEDResp& x) {
    out << "CdcReplicasDEPRECATEDResp(" << "Replicas=" << x.replicas << ")";
    return out;
}

void AllShardsReq::pack(BincodeBuf& buf) const {
}
void AllShardsReq::unpack(BincodeBuf& buf) {
}
void AllShardsReq::clear() {
}
bool AllShardsReq::operator==(const AllShardsReq& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const AllShardsReq& x) {
    out << "AllShardsReq(" << ")";
    return out;
}

void AllShardsResp::pack(BincodeBuf& buf) const {
    buf.packList<FullShardInfo>(shards);
}
void AllShardsResp::unpack(BincodeBuf& buf) {
    buf.unpackList<FullShardInfo>(shards);
}
void AllShardsResp::clear() {
    shards.clear();
}
bool AllShardsResp::operator==(const AllShardsResp& rhs) const {
    if (shards != rhs.shards) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AllShardsResp& x) {
    out << "AllShardsResp(" << "Shards=" << x.shards << ")";
    return out;
}

void DecommissionBlockServiceReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
}
void DecommissionBlockServiceReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
}
void DecommissionBlockServiceReq::clear() {
    id = BlockServiceId(0);
}
bool DecommissionBlockServiceReq::operator==(const DecommissionBlockServiceReq& rhs) const {
    if ((BlockServiceId)this->id != (BlockServiceId)rhs.id) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const DecommissionBlockServiceReq& x) {
    out << "DecommissionBlockServiceReq(" << "Id=" << x.id << ")";
    return out;
}

void DecommissionBlockServiceResp::pack(BincodeBuf& buf) const {
}
void DecommissionBlockServiceResp::unpack(BincodeBuf& buf) {
}
void DecommissionBlockServiceResp::clear() {
}
bool DecommissionBlockServiceResp::operator==(const DecommissionBlockServiceResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const DecommissionBlockServiceResp& x) {
    out << "DecommissionBlockServiceResp(" << ")";
    return out;
}

void MoveShardLeaderReq::pack(BincodeBuf& buf) const {
    shrid.pack(buf);
    buf.packScalar<uint8_t>(location);
}
void MoveShardLeaderReq::unpack(BincodeBuf& buf) {
    shrid.unpack(buf);
    location = buf.unpackScalar<uint8_t>();
}
void MoveShardLeaderReq::clear() {
    shrid = ShardReplicaId();
    location = uint8_t(0);
}
bool MoveShardLeaderReq::operator==(const MoveShardLeaderReq& rhs) const {
    if ((ShardReplicaId)this->shrid != (ShardReplicaId)rhs.shrid) { return false; };
    if ((uint8_t)this->location != (uint8_t)rhs.location) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const MoveShardLeaderReq& x) {
    out << "MoveShardLeaderReq(" << "Shrid=" << x.shrid << ", " << "Location=" << (int)x.location << ")";
    return out;
}

void MoveShardLeaderResp::pack(BincodeBuf& buf) const {
}
void MoveShardLeaderResp::unpack(BincodeBuf& buf) {
}
void MoveShardLeaderResp::clear() {
}
bool MoveShardLeaderResp::operator==(const MoveShardLeaderResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const MoveShardLeaderResp& x) {
    out << "MoveShardLeaderResp(" << ")";
    return out;
}

void ClearShardInfoReq::pack(BincodeBuf& buf) const {
    shrid.pack(buf);
    buf.packScalar<uint8_t>(location);
}
void ClearShardInfoReq::unpack(BincodeBuf& buf) {
    shrid.unpack(buf);
    location = buf.unpackScalar<uint8_t>();
}
void ClearShardInfoReq::clear() {
    shrid = ShardReplicaId();
    location = uint8_t(0);
}
bool ClearShardInfoReq::operator==(const ClearShardInfoReq& rhs) const {
    if ((ShardReplicaId)this->shrid != (ShardReplicaId)rhs.shrid) { return false; };
    if ((uint8_t)this->location != (uint8_t)rhs.location) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ClearShardInfoReq& x) {
    out << "ClearShardInfoReq(" << "Shrid=" << x.shrid << ", " << "Location=" << (int)x.location << ")";
    return out;
}

void ClearShardInfoResp::pack(BincodeBuf& buf) const {
}
void ClearShardInfoResp::unpack(BincodeBuf& buf) {
}
void ClearShardInfoResp::clear() {
}
bool ClearShardInfoResp::operator==(const ClearShardInfoResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const ClearShardInfoResp& x) {
    out << "ClearShardInfoResp(" << ")";
    return out;
}

void ShardBlockServicesReq::pack(BincodeBuf& buf) const {
    shardId.pack(buf);
}
void ShardBlockServicesReq::unpack(BincodeBuf& buf) {
    shardId.unpack(buf);
}
void ShardBlockServicesReq::clear() {
    shardId = ShardId();
}
bool ShardBlockServicesReq::operator==(const ShardBlockServicesReq& rhs) const {
    if ((ShardId)this->shardId != (ShardId)rhs.shardId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ShardBlockServicesReq& x) {
    out << "ShardBlockServicesReq(" << "ShardId=" << x.shardId << ")";
    return out;
}

void ShardBlockServicesResp::pack(BincodeBuf& buf) const {
    buf.packList<BlockServiceInfoShort>(blockServices);
}
void ShardBlockServicesResp::unpack(BincodeBuf& buf) {
    buf.unpackList<BlockServiceInfoShort>(blockServices);
}
void ShardBlockServicesResp::clear() {
    blockServices.clear();
}
bool ShardBlockServicesResp::operator==(const ShardBlockServicesResp& rhs) const {
    if (blockServices != rhs.blockServices) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ShardBlockServicesResp& x) {
    out << "ShardBlockServicesResp(" << "BlockServices=" << x.blockServices << ")";
    return out;
}

void AllCdcReq::pack(BincodeBuf& buf) const {
}
void AllCdcReq::unpack(BincodeBuf& buf) {
}
void AllCdcReq::clear() {
}
bool AllCdcReq::operator==(const AllCdcReq& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const AllCdcReq& x) {
    out << "AllCdcReq(" << ")";
    return out;
}

void AllCdcResp::pack(BincodeBuf& buf) const {
    buf.packList<CdcInfo>(replicas);
}
void AllCdcResp::unpack(BincodeBuf& buf) {
    buf.unpackList<CdcInfo>(replicas);
}
void AllCdcResp::clear() {
    replicas.clear();
}
bool AllCdcResp::operator==(const AllCdcResp& rhs) const {
    if (replicas != rhs.replicas) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AllCdcResp& x) {
    out << "AllCdcResp(" << "Replicas=" << x.replicas << ")";
    return out;
}

void EraseDecommissionedBlockReq::pack(BincodeBuf& buf) const {
    blockServiceId.pack(buf);
    buf.packScalar<uint64_t>(blockId);
    buf.packFixedBytes<8>(certificate);
}
void EraseDecommissionedBlockReq::unpack(BincodeBuf& buf) {
    blockServiceId.unpack(buf);
    blockId = buf.unpackScalar<uint64_t>();
    buf.unpackFixedBytes<8>(certificate);
}
void EraseDecommissionedBlockReq::clear() {
    blockServiceId = BlockServiceId(0);
    blockId = uint64_t(0);
    certificate.clear();
}
bool EraseDecommissionedBlockReq::operator==(const EraseDecommissionedBlockReq& rhs) const {
    if ((BlockServiceId)this->blockServiceId != (BlockServiceId)rhs.blockServiceId) { return false; };
    if ((uint64_t)this->blockId != (uint64_t)rhs.blockId) { return false; };
    if (certificate != rhs.certificate) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const EraseDecommissionedBlockReq& x) {
    out << "EraseDecommissionedBlockReq(" << "BlockServiceId=" << x.blockServiceId << ", " << "BlockId=" << x.blockId << ", " << "Certificate=" << x.certificate << ")";
    return out;
}

void EraseDecommissionedBlockResp::pack(BincodeBuf& buf) const {
    buf.packFixedBytes<8>(proof);
}
void EraseDecommissionedBlockResp::unpack(BincodeBuf& buf) {
    buf.unpackFixedBytes<8>(proof);
}
void EraseDecommissionedBlockResp::clear() {
    proof.clear();
}
bool EraseDecommissionedBlockResp::operator==(const EraseDecommissionedBlockResp& rhs) const {
    if (proof != rhs.proof) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const EraseDecommissionedBlockResp& x) {
    out << "EraseDecommissionedBlockResp(" << "Proof=" << x.proof << ")";
    return out;
}

void AllBlockServicesDeprecatedReq::pack(BincodeBuf& buf) const {
}
void AllBlockServicesDeprecatedReq::unpack(BincodeBuf& buf) {
}
void AllBlockServicesDeprecatedReq::clear() {
}
bool AllBlockServicesDeprecatedReq::operator==(const AllBlockServicesDeprecatedReq& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const AllBlockServicesDeprecatedReq& x) {
    out << "AllBlockServicesDeprecatedReq(" << ")";
    return out;
}

void AllBlockServicesDeprecatedResp::pack(BincodeBuf& buf) const {
    buf.packList<BlockServiceDeprecatedInfo>(blockServices);
}
void AllBlockServicesDeprecatedResp::unpack(BincodeBuf& buf) {
    buf.unpackList<BlockServiceDeprecatedInfo>(blockServices);
}
void AllBlockServicesDeprecatedResp::clear() {
    blockServices.clear();
}
bool AllBlockServicesDeprecatedResp::operator==(const AllBlockServicesDeprecatedResp& rhs) const {
    if (blockServices != rhs.blockServices) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AllBlockServicesDeprecatedResp& x) {
    out << "AllBlockServicesDeprecatedResp(" << "BlockServices=" << x.blockServices << ")";
    return out;
}

void MoveCdcLeaderReq::pack(BincodeBuf& buf) const {
    replica.pack(buf);
    buf.packScalar<uint8_t>(location);
}
void MoveCdcLeaderReq::unpack(BincodeBuf& buf) {
    replica.unpack(buf);
    location = buf.unpackScalar<uint8_t>();
}
void MoveCdcLeaderReq::clear() {
    replica = ReplicaId();
    location = uint8_t(0);
}
bool MoveCdcLeaderReq::operator==(const MoveCdcLeaderReq& rhs) const {
    if ((ReplicaId)this->replica != (ReplicaId)rhs.replica) { return false; };
    if ((uint8_t)this->location != (uint8_t)rhs.location) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const MoveCdcLeaderReq& x) {
    out << "MoveCdcLeaderReq(" << "Replica=" << x.replica << ", " << "Location=" << (int)x.location << ")";
    return out;
}

void MoveCdcLeaderResp::pack(BincodeBuf& buf) const {
}
void MoveCdcLeaderResp::unpack(BincodeBuf& buf) {
}
void MoveCdcLeaderResp::clear() {
}
bool MoveCdcLeaderResp::operator==(const MoveCdcLeaderResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const MoveCdcLeaderResp& x) {
    out << "MoveCdcLeaderResp(" << ")";
    return out;
}

void ClearCdcInfoReq::pack(BincodeBuf& buf) const {
    replica.pack(buf);
    buf.packScalar<uint8_t>(location);
}
void ClearCdcInfoReq::unpack(BincodeBuf& buf) {
    replica.unpack(buf);
    location = buf.unpackScalar<uint8_t>();
}
void ClearCdcInfoReq::clear() {
    replica = ReplicaId();
    location = uint8_t(0);
}
bool ClearCdcInfoReq::operator==(const ClearCdcInfoReq& rhs) const {
    if ((ReplicaId)this->replica != (ReplicaId)rhs.replica) { return false; };
    if ((uint8_t)this->location != (uint8_t)rhs.location) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ClearCdcInfoReq& x) {
    out << "ClearCdcInfoReq(" << "Replica=" << x.replica << ", " << "Location=" << (int)x.location << ")";
    return out;
}

void ClearCdcInfoResp::pack(BincodeBuf& buf) const {
}
void ClearCdcInfoResp::unpack(BincodeBuf& buf) {
}
void ClearCdcInfoResp::clear() {
}
bool ClearCdcInfoResp::operator==(const ClearCdcInfoResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const ClearCdcInfoResp& x) {
    out << "ClearCdcInfoResp(" << ")";
    return out;
}

void UpdateBlockServicePathReq::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packBytes(newPath);
}
void UpdateBlockServicePathReq::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    buf.unpackBytes(newPath);
}
void UpdateBlockServicePathReq::clear() {
    id = BlockServiceId(0);
    newPath.clear();
}
bool UpdateBlockServicePathReq::operator==(const UpdateBlockServicePathReq& rhs) const {
    if ((BlockServiceId)this->id != (BlockServiceId)rhs.id) { return false; };
    if (newPath != rhs.newPath) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const UpdateBlockServicePathReq& x) {
    out << "UpdateBlockServicePathReq(" << "Id=" << x.id << ", " << "NewPath=" << GoLangQuotedStringFmt(x.newPath.data(), x.newPath.size()) << ")";
    return out;
}

void UpdateBlockServicePathResp::pack(BincodeBuf& buf) const {
}
void UpdateBlockServicePathResp::unpack(BincodeBuf& buf) {
}
void UpdateBlockServicePathResp::clear() {
}
bool UpdateBlockServicePathResp::operator==(const UpdateBlockServicePathResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const UpdateBlockServicePathResp& x) {
    out << "UpdateBlockServicePathResp(" << ")";
    return out;
}

void FetchBlockReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(blockId);
    buf.packScalar<uint32_t>(offset);
    buf.packScalar<uint32_t>(count);
}
void FetchBlockReq::unpack(BincodeBuf& buf) {
    blockId = buf.unpackScalar<uint64_t>();
    offset = buf.unpackScalar<uint32_t>();
    count = buf.unpackScalar<uint32_t>();
}
void FetchBlockReq::clear() {
    blockId = uint64_t(0);
    offset = uint32_t(0);
    count = uint32_t(0);
}
bool FetchBlockReq::operator==(const FetchBlockReq& rhs) const {
    if ((uint64_t)this->blockId != (uint64_t)rhs.blockId) { return false; };
    if ((uint32_t)this->offset != (uint32_t)rhs.offset) { return false; };
    if ((uint32_t)this->count != (uint32_t)rhs.count) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchBlockReq& x) {
    out << "FetchBlockReq(" << "BlockId=" << x.blockId << ", " << "Offset=" << x.offset << ", " << "Count=" << x.count << ")";
    return out;
}

void FetchBlockResp::pack(BincodeBuf& buf) const {
}
void FetchBlockResp::unpack(BincodeBuf& buf) {
}
void FetchBlockResp::clear() {
}
bool FetchBlockResp::operator==(const FetchBlockResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchBlockResp& x) {
    out << "FetchBlockResp(" << ")";
    return out;
}

void WriteBlockReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(blockId);
    crc.pack(buf);
    buf.packScalar<uint32_t>(size);
    buf.packFixedBytes<8>(certificate);
}
void WriteBlockReq::unpack(BincodeBuf& buf) {
    blockId = buf.unpackScalar<uint64_t>();
    crc.unpack(buf);
    size = buf.unpackScalar<uint32_t>();
    buf.unpackFixedBytes<8>(certificate);
}
void WriteBlockReq::clear() {
    blockId = uint64_t(0);
    crc = Crc(0);
    size = uint32_t(0);
    certificate.clear();
}
bool WriteBlockReq::operator==(const WriteBlockReq& rhs) const {
    if ((uint64_t)this->blockId != (uint64_t)rhs.blockId) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    if ((uint32_t)this->size != (uint32_t)rhs.size) { return false; };
    if (certificate != rhs.certificate) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const WriteBlockReq& x) {
    out << "WriteBlockReq(" << "BlockId=" << x.blockId << ", " << "Crc=" << x.crc << ", " << "Size=" << x.size << ", " << "Certificate=" << x.certificate << ")";
    return out;
}

void WriteBlockResp::pack(BincodeBuf& buf) const {
    buf.packFixedBytes<8>(proof);
}
void WriteBlockResp::unpack(BincodeBuf& buf) {
    buf.unpackFixedBytes<8>(proof);
}
void WriteBlockResp::clear() {
    proof.clear();
}
bool WriteBlockResp::operator==(const WriteBlockResp& rhs) const {
    if (proof != rhs.proof) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const WriteBlockResp& x) {
    out << "WriteBlockResp(" << "Proof=" << x.proof << ")";
    return out;
}

void FetchBlockWithCrcReq::pack(BincodeBuf& buf) const {
    fileIdUnused.pack(buf);
    buf.packScalar<uint64_t>(blockId);
    blockCrcUnused.pack(buf);
    buf.packScalar<uint32_t>(offset);
    buf.packScalar<uint32_t>(count);
}
void FetchBlockWithCrcReq::unpack(BincodeBuf& buf) {
    fileIdUnused.unpack(buf);
    blockId = buf.unpackScalar<uint64_t>();
    blockCrcUnused.unpack(buf);
    offset = buf.unpackScalar<uint32_t>();
    count = buf.unpackScalar<uint32_t>();
}
void FetchBlockWithCrcReq::clear() {
    fileIdUnused = InodeId();
    blockId = uint64_t(0);
    blockCrcUnused = Crc(0);
    offset = uint32_t(0);
    count = uint32_t(0);
}
bool FetchBlockWithCrcReq::operator==(const FetchBlockWithCrcReq& rhs) const {
    if ((InodeId)this->fileIdUnused != (InodeId)rhs.fileIdUnused) { return false; };
    if ((uint64_t)this->blockId != (uint64_t)rhs.blockId) { return false; };
    if ((Crc)this->blockCrcUnused != (Crc)rhs.blockCrcUnused) { return false; };
    if ((uint32_t)this->offset != (uint32_t)rhs.offset) { return false; };
    if ((uint32_t)this->count != (uint32_t)rhs.count) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchBlockWithCrcReq& x) {
    out << "FetchBlockWithCrcReq(" << "FileIdUnused=" << x.fileIdUnused << ", " << "BlockId=" << x.blockId << ", " << "BlockCrcUnused=" << x.blockCrcUnused << ", " << "Offset=" << x.offset << ", " << "Count=" << x.count << ")";
    return out;
}

void FetchBlockWithCrcResp::pack(BincodeBuf& buf) const {
}
void FetchBlockWithCrcResp::unpack(BincodeBuf& buf) {
}
void FetchBlockWithCrcResp::clear() {
}
bool FetchBlockWithCrcResp::operator==(const FetchBlockWithCrcResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const FetchBlockWithCrcResp& x) {
    out << "FetchBlockWithCrcResp(" << ")";
    return out;
}

void EraseBlockReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(blockId);
    buf.packFixedBytes<8>(certificate);
}
void EraseBlockReq::unpack(BincodeBuf& buf) {
    blockId = buf.unpackScalar<uint64_t>();
    buf.unpackFixedBytes<8>(certificate);
}
void EraseBlockReq::clear() {
    blockId = uint64_t(0);
    certificate.clear();
}
bool EraseBlockReq::operator==(const EraseBlockReq& rhs) const {
    if ((uint64_t)this->blockId != (uint64_t)rhs.blockId) { return false; };
    if (certificate != rhs.certificate) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const EraseBlockReq& x) {
    out << "EraseBlockReq(" << "BlockId=" << x.blockId << ", " << "Certificate=" << x.certificate << ")";
    return out;
}

void EraseBlockResp::pack(BincodeBuf& buf) const {
    buf.packFixedBytes<8>(proof);
}
void EraseBlockResp::unpack(BincodeBuf& buf) {
    buf.unpackFixedBytes<8>(proof);
}
void EraseBlockResp::clear() {
    proof.clear();
}
bool EraseBlockResp::operator==(const EraseBlockResp& rhs) const {
    if (proof != rhs.proof) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const EraseBlockResp& x) {
    out << "EraseBlockResp(" << "Proof=" << x.proof << ")";
    return out;
}

void TestWriteReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(size);
}
void TestWriteReq::unpack(BincodeBuf& buf) {
    size = buf.unpackScalar<uint64_t>();
}
void TestWriteReq::clear() {
    size = uint64_t(0);
}
bool TestWriteReq::operator==(const TestWriteReq& rhs) const {
    if ((uint64_t)this->size != (uint64_t)rhs.size) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const TestWriteReq& x) {
    out << "TestWriteReq(" << "Size=" << x.size << ")";
    return out;
}

void TestWriteResp::pack(BincodeBuf& buf) const {
}
void TestWriteResp::unpack(BincodeBuf& buf) {
}
void TestWriteResp::clear() {
}
bool TestWriteResp::operator==(const TestWriteResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const TestWriteResp& x) {
    out << "TestWriteResp(" << ")";
    return out;
}

void CheckBlockReq::pack(BincodeBuf& buf) const {
    buf.packScalar<uint64_t>(blockId);
    buf.packScalar<uint32_t>(size);
    crc.pack(buf);
}
void CheckBlockReq::unpack(BincodeBuf& buf) {
    blockId = buf.unpackScalar<uint64_t>();
    size = buf.unpackScalar<uint32_t>();
    crc.unpack(buf);
}
void CheckBlockReq::clear() {
    blockId = uint64_t(0);
    size = uint32_t(0);
    crc = Crc(0);
}
bool CheckBlockReq::operator==(const CheckBlockReq& rhs) const {
    if ((uint64_t)this->blockId != (uint64_t)rhs.blockId) { return false; };
    if ((uint32_t)this->size != (uint32_t)rhs.size) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CheckBlockReq& x) {
    out << "CheckBlockReq(" << "BlockId=" << x.blockId << ", " << "Size=" << x.size << ", " << "Crc=" << x.crc << ")";
    return out;
}

void CheckBlockResp::pack(BincodeBuf& buf) const {
}
void CheckBlockResp::unpack(BincodeBuf& buf) {
}
void CheckBlockResp::clear() {
}
bool CheckBlockResp::operator==(const CheckBlockResp& rhs) const {
    return true;
}
std::ostream& operator<<(std::ostream& out, const CheckBlockResp& x) {
    out << "CheckBlockResp(" << ")";
    return out;
}

void LogWriteReq::pack(BincodeBuf& buf) const {
    token.pack(buf);
    lastReleased.pack(buf);
    idx.pack(buf);
    buf.packList<uint8_t>(value);
}
void LogWriteReq::unpack(BincodeBuf& buf) {
    token.unpack(buf);
    lastReleased.unpack(buf);
    idx.unpack(buf);
    buf.unpackList<uint8_t>(value);
}
void LogWriteReq::clear() {
    token = LeaderToken();
    lastReleased = LogIdx();
    idx = LogIdx();
    value.clear();
}
bool LogWriteReq::operator==(const LogWriteReq& rhs) const {
    if ((LeaderToken)this->token != (LeaderToken)rhs.token) { return false; };
    if ((LogIdx)this->lastReleased != (LogIdx)rhs.lastReleased) { return false; };
    if ((LogIdx)this->idx != (LogIdx)rhs.idx) { return false; };
    if (value != rhs.value) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LogWriteReq& x) {
    out << "LogWriteReq(" << "Token=" << x.token << ", " << "LastReleased=" << x.lastReleased << ", " << "Idx=" << x.idx << ", " << "Value=" << x.value << ")";
    return out;
}

void LogWriteResp::pack(BincodeBuf& buf) const {
    buf.packScalar<TernError>(result);
}
void LogWriteResp::unpack(BincodeBuf& buf) {
    result = buf.unpackScalar<TernError>();
}
void LogWriteResp::clear() {
    result = TernError(0);
}
bool LogWriteResp::operator==(const LogWriteResp& rhs) const {
    if ((TernError)this->result != (TernError)rhs.result) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LogWriteResp& x) {
    out << "LogWriteResp(" << "Result=" << x.result << ")";
    return out;
}

void ReleaseReq::pack(BincodeBuf& buf) const {
    token.pack(buf);
    lastReleased.pack(buf);
}
void ReleaseReq::unpack(BincodeBuf& buf) {
    token.unpack(buf);
    lastReleased.unpack(buf);
}
void ReleaseReq::clear() {
    token = LeaderToken();
    lastReleased = LogIdx();
}
bool ReleaseReq::operator==(const ReleaseReq& rhs) const {
    if ((LeaderToken)this->token != (LeaderToken)rhs.token) { return false; };
    if ((LogIdx)this->lastReleased != (LogIdx)rhs.lastReleased) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ReleaseReq& x) {
    out << "ReleaseReq(" << "Token=" << x.token << ", " << "LastReleased=" << x.lastReleased << ")";
    return out;
}

void ReleaseResp::pack(BincodeBuf& buf) const {
    buf.packScalar<TernError>(result);
}
void ReleaseResp::unpack(BincodeBuf& buf) {
    result = buf.unpackScalar<TernError>();
}
void ReleaseResp::clear() {
    result = TernError(0);
}
bool ReleaseResp::operator==(const ReleaseResp& rhs) const {
    if ((TernError)this->result != (TernError)rhs.result) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ReleaseResp& x) {
    out << "ReleaseResp(" << "Result=" << x.result << ")";
    return out;
}

void LogReadReq::pack(BincodeBuf& buf) const {
    idx.pack(buf);
}
void LogReadReq::unpack(BincodeBuf& buf) {
    idx.unpack(buf);
}
void LogReadReq::clear() {
    idx = LogIdx();
}
bool LogReadReq::operator==(const LogReadReq& rhs) const {
    if ((LogIdx)this->idx != (LogIdx)rhs.idx) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LogReadReq& x) {
    out << "LogReadReq(" << "Idx=" << x.idx << ")";
    return out;
}

void LogReadResp::pack(BincodeBuf& buf) const {
    buf.packScalar<TernError>(result);
    buf.packList<uint8_t>(value);
}
void LogReadResp::unpack(BincodeBuf& buf) {
    result = buf.unpackScalar<TernError>();
    buf.unpackList<uint8_t>(value);
}
void LogReadResp::clear() {
    result = TernError(0);
    value.clear();
}
bool LogReadResp::operator==(const LogReadResp& rhs) const {
    if ((TernError)this->result != (TernError)rhs.result) { return false; };
    if (value != rhs.value) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LogReadResp& x) {
    out << "LogReadResp(" << "Result=" << x.result << ", " << "Value=" << x.value << ")";
    return out;
}

void NewLeaderReq::pack(BincodeBuf& buf) const {
    nomineeToken.pack(buf);
}
void NewLeaderReq::unpack(BincodeBuf& buf) {
    nomineeToken.unpack(buf);
}
void NewLeaderReq::clear() {
    nomineeToken = LeaderToken();
}
bool NewLeaderReq::operator==(const NewLeaderReq& rhs) const {
    if ((LeaderToken)this->nomineeToken != (LeaderToken)rhs.nomineeToken) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const NewLeaderReq& x) {
    out << "NewLeaderReq(" << "NomineeToken=" << x.nomineeToken << ")";
    return out;
}

void NewLeaderResp::pack(BincodeBuf& buf) const {
    buf.packScalar<TernError>(result);
    lastReleased.pack(buf);
}
void NewLeaderResp::unpack(BincodeBuf& buf) {
    result = buf.unpackScalar<TernError>();
    lastReleased.unpack(buf);
}
void NewLeaderResp::clear() {
    result = TernError(0);
    lastReleased = LogIdx();
}
bool NewLeaderResp::operator==(const NewLeaderResp& rhs) const {
    if ((TernError)this->result != (TernError)rhs.result) { return false; };
    if ((LogIdx)this->lastReleased != (LogIdx)rhs.lastReleased) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const NewLeaderResp& x) {
    out << "NewLeaderResp(" << "Result=" << x.result << ", " << "LastReleased=" << x.lastReleased << ")";
    return out;
}

void NewLeaderConfirmReq::pack(BincodeBuf& buf) const {
    nomineeToken.pack(buf);
    releasedIdx.pack(buf);
}
void NewLeaderConfirmReq::unpack(BincodeBuf& buf) {
    nomineeToken.unpack(buf);
    releasedIdx.unpack(buf);
}
void NewLeaderConfirmReq::clear() {
    nomineeToken = LeaderToken();
    releasedIdx = LogIdx();
}
bool NewLeaderConfirmReq::operator==(const NewLeaderConfirmReq& rhs) const {
    if ((LeaderToken)this->nomineeToken != (LeaderToken)rhs.nomineeToken) { return false; };
    if ((LogIdx)this->releasedIdx != (LogIdx)rhs.releasedIdx) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const NewLeaderConfirmReq& x) {
    out << "NewLeaderConfirmReq(" << "NomineeToken=" << x.nomineeToken << ", " << "ReleasedIdx=" << x.releasedIdx << ")";
    return out;
}

void NewLeaderConfirmResp::pack(BincodeBuf& buf) const {
    buf.packScalar<TernError>(result);
}
void NewLeaderConfirmResp::unpack(BincodeBuf& buf) {
    result = buf.unpackScalar<TernError>();
}
void NewLeaderConfirmResp::clear() {
    result = TernError(0);
}
bool NewLeaderConfirmResp::operator==(const NewLeaderConfirmResp& rhs) const {
    if ((TernError)this->result != (TernError)rhs.result) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const NewLeaderConfirmResp& x) {
    out << "NewLeaderConfirmResp(" << "Result=" << x.result << ")";
    return out;
}

void LogRecoveryReadReq::pack(BincodeBuf& buf) const {
    nomineeToken.pack(buf);
    idx.pack(buf);
}
void LogRecoveryReadReq::unpack(BincodeBuf& buf) {
    nomineeToken.unpack(buf);
    idx.unpack(buf);
}
void LogRecoveryReadReq::clear() {
    nomineeToken = LeaderToken();
    idx = LogIdx();
}
bool LogRecoveryReadReq::operator==(const LogRecoveryReadReq& rhs) const {
    if ((LeaderToken)this->nomineeToken != (LeaderToken)rhs.nomineeToken) { return false; };
    if ((LogIdx)this->idx != (LogIdx)rhs.idx) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LogRecoveryReadReq& x) {
    out << "LogRecoveryReadReq(" << "NomineeToken=" << x.nomineeToken << ", " << "Idx=" << x.idx << ")";
    return out;
}

void LogRecoveryReadResp::pack(BincodeBuf& buf) const {
    buf.packScalar<TernError>(result);
    buf.packList<uint8_t>(value);
}
void LogRecoveryReadResp::unpack(BincodeBuf& buf) {
    result = buf.unpackScalar<TernError>();
    buf.unpackList<uint8_t>(value);
}
void LogRecoveryReadResp::clear() {
    result = TernError(0);
    value.clear();
}
bool LogRecoveryReadResp::operator==(const LogRecoveryReadResp& rhs) const {
    if ((TernError)this->result != (TernError)rhs.result) { return false; };
    if (value != rhs.value) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LogRecoveryReadResp& x) {
    out << "LogRecoveryReadResp(" << "Result=" << x.result << ", " << "Value=" << x.value << ")";
    return out;
}

void LogRecoveryWriteReq::pack(BincodeBuf& buf) const {
    nomineeToken.pack(buf);
    idx.pack(buf);
    buf.packList<uint8_t>(value);
}
void LogRecoveryWriteReq::unpack(BincodeBuf& buf) {
    nomineeToken.unpack(buf);
    idx.unpack(buf);
    buf.unpackList<uint8_t>(value);
}
void LogRecoveryWriteReq::clear() {
    nomineeToken = LeaderToken();
    idx = LogIdx();
    value.clear();
}
bool LogRecoveryWriteReq::operator==(const LogRecoveryWriteReq& rhs) const {
    if ((LeaderToken)this->nomineeToken != (LeaderToken)rhs.nomineeToken) { return false; };
    if ((LogIdx)this->idx != (LogIdx)rhs.idx) { return false; };
    if (value != rhs.value) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LogRecoveryWriteReq& x) {
    out << "LogRecoveryWriteReq(" << "NomineeToken=" << x.nomineeToken << ", " << "Idx=" << x.idx << ", " << "Value=" << x.value << ")";
    return out;
}

void LogRecoveryWriteResp::pack(BincodeBuf& buf) const {
    buf.packScalar<TernError>(result);
}
void LogRecoveryWriteResp::unpack(BincodeBuf& buf) {
    result = buf.unpackScalar<TernError>();
}
void LogRecoveryWriteResp::clear() {
    result = TernError(0);
}
bool LogRecoveryWriteResp::operator==(const LogRecoveryWriteResp& rhs) const {
    if ((TernError)this->result != (TernError)rhs.result) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LogRecoveryWriteResp& x) {
    out << "LogRecoveryWriteResp(" << "Result=" << x.result << ")";
    return out;
}

const LookupReq& ShardReqContainer::getLookup() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::LOOKUP, "%s != %s", _kind, ShardMessageKind::LOOKUP);
    return std::get<0>(_data);
}
LookupReq& ShardReqContainer::setLookup() {
    _kind = ShardMessageKind::LOOKUP;
    auto& x = _data.emplace<0>();
    return x;
}
const StatFileReq& ShardReqContainer::getStatFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::STAT_FILE, "%s != %s", _kind, ShardMessageKind::STAT_FILE);
    return std::get<1>(_data);
}
StatFileReq& ShardReqContainer::setStatFile() {
    _kind = ShardMessageKind::STAT_FILE;
    auto& x = _data.emplace<1>();
    return x;
}
const StatDirectoryReq& ShardReqContainer::getStatDirectory() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::STAT_DIRECTORY, "%s != %s", _kind, ShardMessageKind::STAT_DIRECTORY);
    return std::get<2>(_data);
}
StatDirectoryReq& ShardReqContainer::setStatDirectory() {
    _kind = ShardMessageKind::STAT_DIRECTORY;
    auto& x = _data.emplace<2>();
    return x;
}
const ReadDirReq& ShardReqContainer::getReadDir() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::READ_DIR, "%s != %s", _kind, ShardMessageKind::READ_DIR);
    return std::get<3>(_data);
}
ReadDirReq& ShardReqContainer::setReadDir() {
    _kind = ShardMessageKind::READ_DIR;
    auto& x = _data.emplace<3>();
    return x;
}
const ConstructFileReq& ShardReqContainer::getConstructFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::CONSTRUCT_FILE, "%s != %s", _kind, ShardMessageKind::CONSTRUCT_FILE);
    return std::get<4>(_data);
}
ConstructFileReq& ShardReqContainer::setConstructFile() {
    _kind = ShardMessageKind::CONSTRUCT_FILE;
    auto& x = _data.emplace<4>();
    return x;
}
const AddSpanInitiateReq& ShardReqContainer::getAddSpanInitiate() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_SPAN_INITIATE, "%s != %s", _kind, ShardMessageKind::ADD_SPAN_INITIATE);
    return std::get<5>(_data);
}
AddSpanInitiateReq& ShardReqContainer::setAddSpanInitiate() {
    _kind = ShardMessageKind::ADD_SPAN_INITIATE;
    auto& x = _data.emplace<5>();
    return x;
}
const AddSpanCertifyReq& ShardReqContainer::getAddSpanCertify() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_SPAN_CERTIFY, "%s != %s", _kind, ShardMessageKind::ADD_SPAN_CERTIFY);
    return std::get<6>(_data);
}
AddSpanCertifyReq& ShardReqContainer::setAddSpanCertify() {
    _kind = ShardMessageKind::ADD_SPAN_CERTIFY;
    auto& x = _data.emplace<6>();
    return x;
}
const LinkFileReq& ShardReqContainer::getLinkFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::LINK_FILE, "%s != %s", _kind, ShardMessageKind::LINK_FILE);
    return std::get<7>(_data);
}
LinkFileReq& ShardReqContainer::setLinkFile() {
    _kind = ShardMessageKind::LINK_FILE;
    auto& x = _data.emplace<7>();
    return x;
}
const SoftUnlinkFileReq& ShardReqContainer::getSoftUnlinkFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SOFT_UNLINK_FILE, "%s != %s", _kind, ShardMessageKind::SOFT_UNLINK_FILE);
    return std::get<8>(_data);
}
SoftUnlinkFileReq& ShardReqContainer::setSoftUnlinkFile() {
    _kind = ShardMessageKind::SOFT_UNLINK_FILE;
    auto& x = _data.emplace<8>();
    return x;
}
const LocalFileSpansReq& ShardReqContainer::getLocalFileSpans() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::LOCAL_FILE_SPANS, "%s != %s", _kind, ShardMessageKind::LOCAL_FILE_SPANS);
    return std::get<9>(_data);
}
LocalFileSpansReq& ShardReqContainer::setLocalFileSpans() {
    _kind = ShardMessageKind::LOCAL_FILE_SPANS;
    auto& x = _data.emplace<9>();
    return x;
}
const SameDirectoryRenameReq& ShardReqContainer::getSameDirectoryRename() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SAME_DIRECTORY_RENAME, "%s != %s", _kind, ShardMessageKind::SAME_DIRECTORY_RENAME);
    return std::get<10>(_data);
}
SameDirectoryRenameReq& ShardReqContainer::setSameDirectoryRename() {
    _kind = ShardMessageKind::SAME_DIRECTORY_RENAME;
    auto& x = _data.emplace<10>();
    return x;
}
const AddInlineSpanReq& ShardReqContainer::getAddInlineSpan() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_INLINE_SPAN, "%s != %s", _kind, ShardMessageKind::ADD_INLINE_SPAN);
    return std::get<11>(_data);
}
AddInlineSpanReq& ShardReqContainer::setAddInlineSpan() {
    _kind = ShardMessageKind::ADD_INLINE_SPAN;
    auto& x = _data.emplace<11>();
    return x;
}
const SetTimeReq& ShardReqContainer::getSetTime() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SET_TIME, "%s != %s", _kind, ShardMessageKind::SET_TIME);
    return std::get<12>(_data);
}
SetTimeReq& ShardReqContainer::setSetTime() {
    _kind = ShardMessageKind::SET_TIME;
    auto& x = _data.emplace<12>();
    return x;
}
const FullReadDirReq& ShardReqContainer::getFullReadDir() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::FULL_READ_DIR, "%s != %s", _kind, ShardMessageKind::FULL_READ_DIR);
    return std::get<13>(_data);
}
FullReadDirReq& ShardReqContainer::setFullReadDir() {
    _kind = ShardMessageKind::FULL_READ_DIR;
    auto& x = _data.emplace<13>();
    return x;
}
const MoveSpanReq& ShardReqContainer::getMoveSpan() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::MOVE_SPAN, "%s != %s", _kind, ShardMessageKind::MOVE_SPAN);
    return std::get<14>(_data);
}
MoveSpanReq& ShardReqContainer::setMoveSpan() {
    _kind = ShardMessageKind::MOVE_SPAN;
    auto& x = _data.emplace<14>();
    return x;
}
const RemoveNonOwnedEdgeReq& ShardReqContainer::getRemoveNonOwnedEdge() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_NON_OWNED_EDGE, "%s != %s", _kind, ShardMessageKind::REMOVE_NON_OWNED_EDGE);
    return std::get<15>(_data);
}
RemoveNonOwnedEdgeReq& ShardReqContainer::setRemoveNonOwnedEdge() {
    _kind = ShardMessageKind::REMOVE_NON_OWNED_EDGE;
    auto& x = _data.emplace<15>();
    return x;
}
const SameShardHardFileUnlinkReq& ShardReqContainer::getSameShardHardFileUnlink() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK, "%s != %s", _kind, ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK);
    return std::get<16>(_data);
}
SameShardHardFileUnlinkReq& ShardReqContainer::setSameShardHardFileUnlink() {
    _kind = ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK;
    auto& x = _data.emplace<16>();
    return x;
}
const StatTransientFileReq& ShardReqContainer::getStatTransientFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::STAT_TRANSIENT_FILE, "%s != %s", _kind, ShardMessageKind::STAT_TRANSIENT_FILE);
    return std::get<17>(_data);
}
StatTransientFileReq& ShardReqContainer::setStatTransientFile() {
    _kind = ShardMessageKind::STAT_TRANSIENT_FILE;
    auto& x = _data.emplace<17>();
    return x;
}
const ShardSnapshotReq& ShardReqContainer::getShardSnapshot() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SHARD_SNAPSHOT, "%s != %s", _kind, ShardMessageKind::SHARD_SNAPSHOT);
    return std::get<18>(_data);
}
ShardSnapshotReq& ShardReqContainer::setShardSnapshot() {
    _kind = ShardMessageKind::SHARD_SNAPSHOT;
    auto& x = _data.emplace<18>();
    return x;
}
const FileSpansReq& ShardReqContainer::getFileSpans() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::FILE_SPANS, "%s != %s", _kind, ShardMessageKind::FILE_SPANS);
    return std::get<19>(_data);
}
FileSpansReq& ShardReqContainer::setFileSpans() {
    _kind = ShardMessageKind::FILE_SPANS;
    auto& x = _data.emplace<19>();
    return x;
}
const AddSpanLocationReq& ShardReqContainer::getAddSpanLocation() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_SPAN_LOCATION, "%s != %s", _kind, ShardMessageKind::ADD_SPAN_LOCATION);
    return std::get<20>(_data);
}
AddSpanLocationReq& ShardReqContainer::setAddSpanLocation() {
    _kind = ShardMessageKind::ADD_SPAN_LOCATION;
    auto& x = _data.emplace<20>();
    return x;
}
const ScrapTransientFileReq& ShardReqContainer::getScrapTransientFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SCRAP_TRANSIENT_FILE, "%s != %s", _kind, ShardMessageKind::SCRAP_TRANSIENT_FILE);
    return std::get<21>(_data);
}
ScrapTransientFileReq& ShardReqContainer::setScrapTransientFile() {
    _kind = ShardMessageKind::SCRAP_TRANSIENT_FILE;
    auto& x = _data.emplace<21>();
    return x;
}
const SetDirectoryInfoReq& ShardReqContainer::getSetDirectoryInfo() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SET_DIRECTORY_INFO, "%s != %s", _kind, ShardMessageKind::SET_DIRECTORY_INFO);
    return std::get<22>(_data);
}
SetDirectoryInfoReq& ShardReqContainer::setSetDirectoryInfo() {
    _kind = ShardMessageKind::SET_DIRECTORY_INFO;
    auto& x = _data.emplace<22>();
    return x;
}
const VisitDirectoriesReq& ShardReqContainer::getVisitDirectories() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::VISIT_DIRECTORIES, "%s != %s", _kind, ShardMessageKind::VISIT_DIRECTORIES);
    return std::get<23>(_data);
}
VisitDirectoriesReq& ShardReqContainer::setVisitDirectories() {
    _kind = ShardMessageKind::VISIT_DIRECTORIES;
    auto& x = _data.emplace<23>();
    return x;
}
const VisitFilesReq& ShardReqContainer::getVisitFiles() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::VISIT_FILES, "%s != %s", _kind, ShardMessageKind::VISIT_FILES);
    return std::get<24>(_data);
}
VisitFilesReq& ShardReqContainer::setVisitFiles() {
    _kind = ShardMessageKind::VISIT_FILES;
    auto& x = _data.emplace<24>();
    return x;
}
const VisitTransientFilesReq& ShardReqContainer::getVisitTransientFiles() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::VISIT_TRANSIENT_FILES, "%s != %s", _kind, ShardMessageKind::VISIT_TRANSIENT_FILES);
    return std::get<25>(_data);
}
VisitTransientFilesReq& ShardReqContainer::setVisitTransientFiles() {
    _kind = ShardMessageKind::VISIT_TRANSIENT_FILES;
    auto& x = _data.emplace<25>();
    return x;
}
const RemoveSpanInitiateReq& ShardReqContainer::getRemoveSpanInitiate() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_SPAN_INITIATE, "%s != %s", _kind, ShardMessageKind::REMOVE_SPAN_INITIATE);
    return std::get<26>(_data);
}
RemoveSpanInitiateReq& ShardReqContainer::setRemoveSpanInitiate() {
    _kind = ShardMessageKind::REMOVE_SPAN_INITIATE;
    auto& x = _data.emplace<26>();
    return x;
}
const RemoveSpanCertifyReq& ShardReqContainer::getRemoveSpanCertify() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_SPAN_CERTIFY, "%s != %s", _kind, ShardMessageKind::REMOVE_SPAN_CERTIFY);
    return std::get<27>(_data);
}
RemoveSpanCertifyReq& ShardReqContainer::setRemoveSpanCertify() {
    _kind = ShardMessageKind::REMOVE_SPAN_CERTIFY;
    auto& x = _data.emplace<27>();
    return x;
}
const SwapBlocksReq& ShardReqContainer::getSwapBlocks() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SWAP_BLOCKS, "%s != %s", _kind, ShardMessageKind::SWAP_BLOCKS);
    return std::get<28>(_data);
}
SwapBlocksReq& ShardReqContainer::setSwapBlocks() {
    _kind = ShardMessageKind::SWAP_BLOCKS;
    auto& x = _data.emplace<28>();
    return x;
}
const BlockServiceFilesReq& ShardReqContainer::getBlockServiceFiles() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::BLOCK_SERVICE_FILES, "%s != %s", _kind, ShardMessageKind::BLOCK_SERVICE_FILES);
    return std::get<29>(_data);
}
BlockServiceFilesReq& ShardReqContainer::setBlockServiceFiles() {
    _kind = ShardMessageKind::BLOCK_SERVICE_FILES;
    auto& x = _data.emplace<29>();
    return x;
}
const RemoveInodeReq& ShardReqContainer::getRemoveInode() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_INODE, "%s != %s", _kind, ShardMessageKind::REMOVE_INODE);
    return std::get<30>(_data);
}
RemoveInodeReq& ShardReqContainer::setRemoveInode() {
    _kind = ShardMessageKind::REMOVE_INODE;
    auto& x = _data.emplace<30>();
    return x;
}
const AddSpanInitiateWithReferenceReq& ShardReqContainer::getAddSpanInitiateWithReference() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE, "%s != %s", _kind, ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE);
    return std::get<31>(_data);
}
AddSpanInitiateWithReferenceReq& ShardReqContainer::setAddSpanInitiateWithReference() {
    _kind = ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE;
    auto& x = _data.emplace<31>();
    return x;
}
const RemoveZeroBlockServiceFilesReq& ShardReqContainer::getRemoveZeroBlockServiceFiles() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES, "%s != %s", _kind, ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES);
    return std::get<32>(_data);
}
RemoveZeroBlockServiceFilesReq& ShardReqContainer::setRemoveZeroBlockServiceFiles() {
    _kind = ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES;
    auto& x = _data.emplace<32>();
    return x;
}
const SwapSpansReq& ShardReqContainer::getSwapSpans() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SWAP_SPANS, "%s != %s", _kind, ShardMessageKind::SWAP_SPANS);
    return std::get<33>(_data);
}
SwapSpansReq& ShardReqContainer::setSwapSpans() {
    _kind = ShardMessageKind::SWAP_SPANS;
    auto& x = _data.emplace<33>();
    return x;
}
const SameDirectoryRenameSnapshotReq& ShardReqContainer::getSameDirectoryRenameSnapshot() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT, "%s != %s", _kind, ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT);
    return std::get<34>(_data);
}
SameDirectoryRenameSnapshotReq& ShardReqContainer::setSameDirectoryRenameSnapshot() {
    _kind = ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT;
    auto& x = _data.emplace<34>();
    return x;
}
const AddSpanAtLocationInitiateReq& ShardReqContainer::getAddSpanAtLocationInitiate() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE, "%s != %s", _kind, ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE);
    return std::get<35>(_data);
}
AddSpanAtLocationInitiateReq& ShardReqContainer::setAddSpanAtLocationInitiate() {
    _kind = ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE;
    auto& x = _data.emplace<35>();
    return x;
}
const CreateDirectoryInodeReq& ShardReqContainer::getCreateDirectoryInode() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::CREATE_DIRECTORY_INODE, "%s != %s", _kind, ShardMessageKind::CREATE_DIRECTORY_INODE);
    return std::get<36>(_data);
}
CreateDirectoryInodeReq& ShardReqContainer::setCreateDirectoryInode() {
    _kind = ShardMessageKind::CREATE_DIRECTORY_INODE;
    auto& x = _data.emplace<36>();
    return x;
}
const SetDirectoryOwnerReq& ShardReqContainer::getSetDirectoryOwner() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SET_DIRECTORY_OWNER, "%s != %s", _kind, ShardMessageKind::SET_DIRECTORY_OWNER);
    return std::get<37>(_data);
}
SetDirectoryOwnerReq& ShardReqContainer::setSetDirectoryOwner() {
    _kind = ShardMessageKind::SET_DIRECTORY_OWNER;
    auto& x = _data.emplace<37>();
    return x;
}
const RemoveDirectoryOwnerReq& ShardReqContainer::getRemoveDirectoryOwner() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_DIRECTORY_OWNER, "%s != %s", _kind, ShardMessageKind::REMOVE_DIRECTORY_OWNER);
    return std::get<38>(_data);
}
RemoveDirectoryOwnerReq& ShardReqContainer::setRemoveDirectoryOwner() {
    _kind = ShardMessageKind::REMOVE_DIRECTORY_OWNER;
    auto& x = _data.emplace<38>();
    return x;
}
const CreateLockedCurrentEdgeReq& ShardReqContainer::getCreateLockedCurrentEdge() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE, "%s != %s", _kind, ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE);
    return std::get<39>(_data);
}
CreateLockedCurrentEdgeReq& ShardReqContainer::setCreateLockedCurrentEdge() {
    _kind = ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE;
    auto& x = _data.emplace<39>();
    return x;
}
const LockCurrentEdgeReq& ShardReqContainer::getLockCurrentEdge() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::LOCK_CURRENT_EDGE, "%s != %s", _kind, ShardMessageKind::LOCK_CURRENT_EDGE);
    return std::get<40>(_data);
}
LockCurrentEdgeReq& ShardReqContainer::setLockCurrentEdge() {
    _kind = ShardMessageKind::LOCK_CURRENT_EDGE;
    auto& x = _data.emplace<40>();
    return x;
}
const UnlockCurrentEdgeReq& ShardReqContainer::getUnlockCurrentEdge() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::UNLOCK_CURRENT_EDGE, "%s != %s", _kind, ShardMessageKind::UNLOCK_CURRENT_EDGE);
    return std::get<41>(_data);
}
UnlockCurrentEdgeReq& ShardReqContainer::setUnlockCurrentEdge() {
    _kind = ShardMessageKind::UNLOCK_CURRENT_EDGE;
    auto& x = _data.emplace<41>();
    return x;
}
const RemoveOwnedSnapshotFileEdgeReq& ShardReqContainer::getRemoveOwnedSnapshotFileEdge() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE, "%s != %s", _kind, ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE);
    return std::get<42>(_data);
}
RemoveOwnedSnapshotFileEdgeReq& ShardReqContainer::setRemoveOwnedSnapshotFileEdge() {
    _kind = ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE;
    auto& x = _data.emplace<42>();
    return x;
}
const MakeFileTransientReq& ShardReqContainer::getMakeFileTransient() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::MAKE_FILE_TRANSIENT, "%s != %s", _kind, ShardMessageKind::MAKE_FILE_TRANSIENT);
    return std::get<43>(_data);
}
MakeFileTransientReq& ShardReqContainer::setMakeFileTransient() {
    _kind = ShardMessageKind::MAKE_FILE_TRANSIENT;
    auto& x = _data.emplace<43>();
    return x;
}
ShardReqContainer::ShardReqContainer() {
    clear();
}

ShardReqContainer::ShardReqContainer(const ShardReqContainer& other) {
    *this = other;
}

ShardReqContainer::ShardReqContainer(ShardReqContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = ShardMessageKind::EMPTY;
}

void ShardReqContainer::operator=(const ShardReqContainer& other) {
    if (other.kind() == ShardMessageKind::EMPTY) { clear(); return; }
    switch (other.kind()) {
    case ShardMessageKind::LOOKUP:
        setLookup() = other.getLookup();
        break;
    case ShardMessageKind::STAT_FILE:
        setStatFile() = other.getStatFile();
        break;
    case ShardMessageKind::STAT_DIRECTORY:
        setStatDirectory() = other.getStatDirectory();
        break;
    case ShardMessageKind::READ_DIR:
        setReadDir() = other.getReadDir();
        break;
    case ShardMessageKind::CONSTRUCT_FILE:
        setConstructFile() = other.getConstructFile();
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE:
        setAddSpanInitiate() = other.getAddSpanInitiate();
        break;
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        setAddSpanCertify() = other.getAddSpanCertify();
        break;
    case ShardMessageKind::LINK_FILE:
        setLinkFile() = other.getLinkFile();
        break;
    case ShardMessageKind::SOFT_UNLINK_FILE:
        setSoftUnlinkFile() = other.getSoftUnlinkFile();
        break;
    case ShardMessageKind::LOCAL_FILE_SPANS:
        setLocalFileSpans() = other.getLocalFileSpans();
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        setSameDirectoryRename() = other.getSameDirectoryRename();
        break;
    case ShardMessageKind::ADD_INLINE_SPAN:
        setAddInlineSpan() = other.getAddInlineSpan();
        break;
    case ShardMessageKind::SET_TIME:
        setSetTime() = other.getSetTime();
        break;
    case ShardMessageKind::FULL_READ_DIR:
        setFullReadDir() = other.getFullReadDir();
        break;
    case ShardMessageKind::MOVE_SPAN:
        setMoveSpan() = other.getMoveSpan();
        break;
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        setRemoveNonOwnedEdge() = other.getRemoveNonOwnedEdge();
        break;
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        setSameShardHardFileUnlink() = other.getSameShardHardFileUnlink();
        break;
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        setStatTransientFile() = other.getStatTransientFile();
        break;
    case ShardMessageKind::SHARD_SNAPSHOT:
        setShardSnapshot() = other.getShardSnapshot();
        break;
    case ShardMessageKind::FILE_SPANS:
        setFileSpans() = other.getFileSpans();
        break;
    case ShardMessageKind::ADD_SPAN_LOCATION:
        setAddSpanLocation() = other.getAddSpanLocation();
        break;
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        setScrapTransientFile() = other.getScrapTransientFile();
        break;
    case ShardMessageKind::SET_DIRECTORY_INFO:
        setSetDirectoryInfo() = other.getSetDirectoryInfo();
        break;
    case ShardMessageKind::VISIT_DIRECTORIES:
        setVisitDirectories() = other.getVisitDirectories();
        break;
    case ShardMessageKind::VISIT_FILES:
        setVisitFiles() = other.getVisitFiles();
        break;
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        setVisitTransientFiles() = other.getVisitTransientFiles();
        break;
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        setRemoveSpanInitiate() = other.getRemoveSpanInitiate();
        break;
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        setRemoveSpanCertify() = other.getRemoveSpanCertify();
        break;
    case ShardMessageKind::SWAP_BLOCKS:
        setSwapBlocks() = other.getSwapBlocks();
        break;
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        setBlockServiceFiles() = other.getBlockServiceFiles();
        break;
    case ShardMessageKind::REMOVE_INODE:
        setRemoveInode() = other.getRemoveInode();
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        setAddSpanInitiateWithReference() = other.getAddSpanInitiateWithReference();
        break;
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        setRemoveZeroBlockServiceFiles() = other.getRemoveZeroBlockServiceFiles();
        break;
    case ShardMessageKind::SWAP_SPANS:
        setSwapSpans() = other.getSwapSpans();
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        setSameDirectoryRenameSnapshot() = other.getSameDirectoryRenameSnapshot();
        break;
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        setAddSpanAtLocationInitiate() = other.getAddSpanAtLocationInitiate();
        break;
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        setCreateDirectoryInode() = other.getCreateDirectoryInode();
        break;
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        setSetDirectoryOwner() = other.getSetDirectoryOwner();
        break;
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        setRemoveDirectoryOwner() = other.getRemoveDirectoryOwner();
        break;
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        setCreateLockedCurrentEdge() = other.getCreateLockedCurrentEdge();
        break;
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        setLockCurrentEdge() = other.getLockCurrentEdge();
        break;
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        setUnlockCurrentEdge() = other.getUnlockCurrentEdge();
        break;
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        setRemoveOwnedSnapshotFileEdge() = other.getRemoveOwnedSnapshotFileEdge();
        break;
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        setMakeFileTransient() = other.getMakeFileTransient();
        break;
    default:
        throw TERN_EXCEPTION("bad ShardMessageKind kind %s", other.kind());
    }
}

void ShardReqContainer::operator=(ShardReqContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = ShardMessageKind::EMPTY;
}

size_t ShardReqContainer::packedSize() const {
    switch (_kind) {
    case ShardMessageKind::LOOKUP:
        return sizeof(ShardMessageKind) + std::get<0>(_data).packedSize();
    case ShardMessageKind::STAT_FILE:
        return sizeof(ShardMessageKind) + std::get<1>(_data).packedSize();
    case ShardMessageKind::STAT_DIRECTORY:
        return sizeof(ShardMessageKind) + std::get<2>(_data).packedSize();
    case ShardMessageKind::READ_DIR:
        return sizeof(ShardMessageKind) + std::get<3>(_data).packedSize();
    case ShardMessageKind::CONSTRUCT_FILE:
        return sizeof(ShardMessageKind) + std::get<4>(_data).packedSize();
    case ShardMessageKind::ADD_SPAN_INITIATE:
        return sizeof(ShardMessageKind) + std::get<5>(_data).packedSize();
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        return sizeof(ShardMessageKind) + std::get<6>(_data).packedSize();
    case ShardMessageKind::LINK_FILE:
        return sizeof(ShardMessageKind) + std::get<7>(_data).packedSize();
    case ShardMessageKind::SOFT_UNLINK_FILE:
        return sizeof(ShardMessageKind) + std::get<8>(_data).packedSize();
    case ShardMessageKind::LOCAL_FILE_SPANS:
        return sizeof(ShardMessageKind) + std::get<9>(_data).packedSize();
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        return sizeof(ShardMessageKind) + std::get<10>(_data).packedSize();
    case ShardMessageKind::ADD_INLINE_SPAN:
        return sizeof(ShardMessageKind) + std::get<11>(_data).packedSize();
    case ShardMessageKind::SET_TIME:
        return sizeof(ShardMessageKind) + std::get<12>(_data).packedSize();
    case ShardMessageKind::FULL_READ_DIR:
        return sizeof(ShardMessageKind) + std::get<13>(_data).packedSize();
    case ShardMessageKind::MOVE_SPAN:
        return sizeof(ShardMessageKind) + std::get<14>(_data).packedSize();
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        return sizeof(ShardMessageKind) + std::get<15>(_data).packedSize();
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        return sizeof(ShardMessageKind) + std::get<16>(_data).packedSize();
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        return sizeof(ShardMessageKind) + std::get<17>(_data).packedSize();
    case ShardMessageKind::SHARD_SNAPSHOT:
        return sizeof(ShardMessageKind) + std::get<18>(_data).packedSize();
    case ShardMessageKind::FILE_SPANS:
        return sizeof(ShardMessageKind) + std::get<19>(_data).packedSize();
    case ShardMessageKind::ADD_SPAN_LOCATION:
        return sizeof(ShardMessageKind) + std::get<20>(_data).packedSize();
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        return sizeof(ShardMessageKind) + std::get<21>(_data).packedSize();
    case ShardMessageKind::SET_DIRECTORY_INFO:
        return sizeof(ShardMessageKind) + std::get<22>(_data).packedSize();
    case ShardMessageKind::VISIT_DIRECTORIES:
        return sizeof(ShardMessageKind) + std::get<23>(_data).packedSize();
    case ShardMessageKind::VISIT_FILES:
        return sizeof(ShardMessageKind) + std::get<24>(_data).packedSize();
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        return sizeof(ShardMessageKind) + std::get<25>(_data).packedSize();
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        return sizeof(ShardMessageKind) + std::get<26>(_data).packedSize();
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        return sizeof(ShardMessageKind) + std::get<27>(_data).packedSize();
    case ShardMessageKind::SWAP_BLOCKS:
        return sizeof(ShardMessageKind) + std::get<28>(_data).packedSize();
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        return sizeof(ShardMessageKind) + std::get<29>(_data).packedSize();
    case ShardMessageKind::REMOVE_INODE:
        return sizeof(ShardMessageKind) + std::get<30>(_data).packedSize();
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        return sizeof(ShardMessageKind) + std::get<31>(_data).packedSize();
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        return sizeof(ShardMessageKind) + std::get<32>(_data).packedSize();
    case ShardMessageKind::SWAP_SPANS:
        return sizeof(ShardMessageKind) + std::get<33>(_data).packedSize();
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        return sizeof(ShardMessageKind) + std::get<34>(_data).packedSize();
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        return sizeof(ShardMessageKind) + std::get<35>(_data).packedSize();
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        return sizeof(ShardMessageKind) + std::get<36>(_data).packedSize();
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        return sizeof(ShardMessageKind) + std::get<37>(_data).packedSize();
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        return sizeof(ShardMessageKind) + std::get<38>(_data).packedSize();
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        return sizeof(ShardMessageKind) + std::get<39>(_data).packedSize();
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        return sizeof(ShardMessageKind) + std::get<40>(_data).packedSize();
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        return sizeof(ShardMessageKind) + std::get<41>(_data).packedSize();
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        return sizeof(ShardMessageKind) + std::get<42>(_data).packedSize();
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        return sizeof(ShardMessageKind) + std::get<43>(_data).packedSize();
    default:
        throw TERN_EXCEPTION("bad ShardMessageKind kind %s", _kind);
    }
}

void ShardReqContainer::pack(BincodeBuf& buf) const {
    buf.packScalar<ShardMessageKind>(_kind);
    switch (_kind) {
    case ShardMessageKind::LOOKUP:
        std::get<0>(_data).pack(buf);
        break;
    case ShardMessageKind::STAT_FILE:
        std::get<1>(_data).pack(buf);
        break;
    case ShardMessageKind::STAT_DIRECTORY:
        std::get<2>(_data).pack(buf);
        break;
    case ShardMessageKind::READ_DIR:
        std::get<3>(_data).pack(buf);
        break;
    case ShardMessageKind::CONSTRUCT_FILE:
        std::get<4>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE:
        std::get<5>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        std::get<6>(_data).pack(buf);
        break;
    case ShardMessageKind::LINK_FILE:
        std::get<7>(_data).pack(buf);
        break;
    case ShardMessageKind::SOFT_UNLINK_FILE:
        std::get<8>(_data).pack(buf);
        break;
    case ShardMessageKind::LOCAL_FILE_SPANS:
        std::get<9>(_data).pack(buf);
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        std::get<10>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_INLINE_SPAN:
        std::get<11>(_data).pack(buf);
        break;
    case ShardMessageKind::SET_TIME:
        std::get<12>(_data).pack(buf);
        break;
    case ShardMessageKind::FULL_READ_DIR:
        std::get<13>(_data).pack(buf);
        break;
    case ShardMessageKind::MOVE_SPAN:
        std::get<14>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        std::get<15>(_data).pack(buf);
        break;
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        std::get<16>(_data).pack(buf);
        break;
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        std::get<17>(_data).pack(buf);
        break;
    case ShardMessageKind::SHARD_SNAPSHOT:
        std::get<18>(_data).pack(buf);
        break;
    case ShardMessageKind::FILE_SPANS:
        std::get<19>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_LOCATION:
        std::get<20>(_data).pack(buf);
        break;
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        std::get<21>(_data).pack(buf);
        break;
    case ShardMessageKind::SET_DIRECTORY_INFO:
        std::get<22>(_data).pack(buf);
        break;
    case ShardMessageKind::VISIT_DIRECTORIES:
        std::get<23>(_data).pack(buf);
        break;
    case ShardMessageKind::VISIT_FILES:
        std::get<24>(_data).pack(buf);
        break;
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        std::get<25>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        std::get<26>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        std::get<27>(_data).pack(buf);
        break;
    case ShardMessageKind::SWAP_BLOCKS:
        std::get<28>(_data).pack(buf);
        break;
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        std::get<29>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_INODE:
        std::get<30>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        std::get<31>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        std::get<32>(_data).pack(buf);
        break;
    case ShardMessageKind::SWAP_SPANS:
        std::get<33>(_data).pack(buf);
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        std::get<34>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        std::get<35>(_data).pack(buf);
        break;
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        std::get<36>(_data).pack(buf);
        break;
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        std::get<37>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        std::get<38>(_data).pack(buf);
        break;
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        std::get<39>(_data).pack(buf);
        break;
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        std::get<40>(_data).pack(buf);
        break;
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        std::get<41>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        std::get<42>(_data).pack(buf);
        break;
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        std::get<43>(_data).pack(buf);
        break;
    default:
        throw TERN_EXCEPTION("bad ShardMessageKind kind %s", _kind);
    }
}

void ShardReqContainer::unpack(BincodeBuf& buf) {
    _kind = buf.unpackScalar<ShardMessageKind>();
    switch (_kind) {
    case ShardMessageKind::LOOKUP:
        _data.emplace<0>().unpack(buf);
        break;
    case ShardMessageKind::STAT_FILE:
        _data.emplace<1>().unpack(buf);
        break;
    case ShardMessageKind::STAT_DIRECTORY:
        _data.emplace<2>().unpack(buf);
        break;
    case ShardMessageKind::READ_DIR:
        _data.emplace<3>().unpack(buf);
        break;
    case ShardMessageKind::CONSTRUCT_FILE:
        _data.emplace<4>().unpack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE:
        _data.emplace<5>().unpack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        _data.emplace<6>().unpack(buf);
        break;
    case ShardMessageKind::LINK_FILE:
        _data.emplace<7>().unpack(buf);
        break;
    case ShardMessageKind::SOFT_UNLINK_FILE:
        _data.emplace<8>().unpack(buf);
        break;
    case ShardMessageKind::LOCAL_FILE_SPANS:
        _data.emplace<9>().unpack(buf);
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        _data.emplace<10>().unpack(buf);
        break;
    case ShardMessageKind::ADD_INLINE_SPAN:
        _data.emplace<11>().unpack(buf);
        break;
    case ShardMessageKind::SET_TIME:
        _data.emplace<12>().unpack(buf);
        break;
    case ShardMessageKind::FULL_READ_DIR:
        _data.emplace<13>().unpack(buf);
        break;
    case ShardMessageKind::MOVE_SPAN:
        _data.emplace<14>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        _data.emplace<15>().unpack(buf);
        break;
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        _data.emplace<16>().unpack(buf);
        break;
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        _data.emplace<17>().unpack(buf);
        break;
    case ShardMessageKind::SHARD_SNAPSHOT:
        _data.emplace<18>().unpack(buf);
        break;
    case ShardMessageKind::FILE_SPANS:
        _data.emplace<19>().unpack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_LOCATION:
        _data.emplace<20>().unpack(buf);
        break;
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        _data.emplace<21>().unpack(buf);
        break;
    case ShardMessageKind::SET_DIRECTORY_INFO:
        _data.emplace<22>().unpack(buf);
        break;
    case ShardMessageKind::VISIT_DIRECTORIES:
        _data.emplace<23>().unpack(buf);
        break;
    case ShardMessageKind::VISIT_FILES:
        _data.emplace<24>().unpack(buf);
        break;
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        _data.emplace<25>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        _data.emplace<26>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        _data.emplace<27>().unpack(buf);
        break;
    case ShardMessageKind::SWAP_BLOCKS:
        _data.emplace<28>().unpack(buf);
        break;
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        _data.emplace<29>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_INODE:
        _data.emplace<30>().unpack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        _data.emplace<31>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        _data.emplace<32>().unpack(buf);
        break;
    case ShardMessageKind::SWAP_SPANS:
        _data.emplace<33>().unpack(buf);
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        _data.emplace<34>().unpack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        _data.emplace<35>().unpack(buf);
        break;
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        _data.emplace<36>().unpack(buf);
        break;
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        _data.emplace<37>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        _data.emplace<38>().unpack(buf);
        break;
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        _data.emplace<39>().unpack(buf);
        break;
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        _data.emplace<40>().unpack(buf);
        break;
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        _data.emplace<41>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        _data.emplace<42>().unpack(buf);
        break;
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        _data.emplace<43>().unpack(buf);
        break;
    default:
        throw BINCODE_EXCEPTION("bad ShardMessageKind kind %s", _kind);
    }
}

bool ShardReqContainer::operator==(const ShardReqContainer& other) const {
    if (_kind != other.kind()) { return false; }
    if (_kind == ShardMessageKind::EMPTY) { return true; }
    switch (_kind) {
    case ShardMessageKind::LOOKUP:
        return getLookup() == other.getLookup();
    case ShardMessageKind::STAT_FILE:
        return getStatFile() == other.getStatFile();
    case ShardMessageKind::STAT_DIRECTORY:
        return getStatDirectory() == other.getStatDirectory();
    case ShardMessageKind::READ_DIR:
        return getReadDir() == other.getReadDir();
    case ShardMessageKind::CONSTRUCT_FILE:
        return getConstructFile() == other.getConstructFile();
    case ShardMessageKind::ADD_SPAN_INITIATE:
        return getAddSpanInitiate() == other.getAddSpanInitiate();
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        return getAddSpanCertify() == other.getAddSpanCertify();
    case ShardMessageKind::LINK_FILE:
        return getLinkFile() == other.getLinkFile();
    case ShardMessageKind::SOFT_UNLINK_FILE:
        return getSoftUnlinkFile() == other.getSoftUnlinkFile();
    case ShardMessageKind::LOCAL_FILE_SPANS:
        return getLocalFileSpans() == other.getLocalFileSpans();
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        return getSameDirectoryRename() == other.getSameDirectoryRename();
    case ShardMessageKind::ADD_INLINE_SPAN:
        return getAddInlineSpan() == other.getAddInlineSpan();
    case ShardMessageKind::SET_TIME:
        return getSetTime() == other.getSetTime();
    case ShardMessageKind::FULL_READ_DIR:
        return getFullReadDir() == other.getFullReadDir();
    case ShardMessageKind::MOVE_SPAN:
        return getMoveSpan() == other.getMoveSpan();
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        return getRemoveNonOwnedEdge() == other.getRemoveNonOwnedEdge();
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        return getSameShardHardFileUnlink() == other.getSameShardHardFileUnlink();
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        return getStatTransientFile() == other.getStatTransientFile();
    case ShardMessageKind::SHARD_SNAPSHOT:
        return getShardSnapshot() == other.getShardSnapshot();
    case ShardMessageKind::FILE_SPANS:
        return getFileSpans() == other.getFileSpans();
    case ShardMessageKind::ADD_SPAN_LOCATION:
        return getAddSpanLocation() == other.getAddSpanLocation();
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        return getScrapTransientFile() == other.getScrapTransientFile();
    case ShardMessageKind::SET_DIRECTORY_INFO:
        return getSetDirectoryInfo() == other.getSetDirectoryInfo();
    case ShardMessageKind::VISIT_DIRECTORIES:
        return getVisitDirectories() == other.getVisitDirectories();
    case ShardMessageKind::VISIT_FILES:
        return getVisitFiles() == other.getVisitFiles();
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        return getVisitTransientFiles() == other.getVisitTransientFiles();
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        return getRemoveSpanInitiate() == other.getRemoveSpanInitiate();
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        return getRemoveSpanCertify() == other.getRemoveSpanCertify();
    case ShardMessageKind::SWAP_BLOCKS:
        return getSwapBlocks() == other.getSwapBlocks();
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        return getBlockServiceFiles() == other.getBlockServiceFiles();
    case ShardMessageKind::REMOVE_INODE:
        return getRemoveInode() == other.getRemoveInode();
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        return getAddSpanInitiateWithReference() == other.getAddSpanInitiateWithReference();
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        return getRemoveZeroBlockServiceFiles() == other.getRemoveZeroBlockServiceFiles();
    case ShardMessageKind::SWAP_SPANS:
        return getSwapSpans() == other.getSwapSpans();
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        return getSameDirectoryRenameSnapshot() == other.getSameDirectoryRenameSnapshot();
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        return getAddSpanAtLocationInitiate() == other.getAddSpanAtLocationInitiate();
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        return getCreateDirectoryInode() == other.getCreateDirectoryInode();
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        return getSetDirectoryOwner() == other.getSetDirectoryOwner();
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        return getRemoveDirectoryOwner() == other.getRemoveDirectoryOwner();
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        return getCreateLockedCurrentEdge() == other.getCreateLockedCurrentEdge();
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        return getLockCurrentEdge() == other.getLockCurrentEdge();
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        return getUnlockCurrentEdge() == other.getUnlockCurrentEdge();
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        return getRemoveOwnedSnapshotFileEdge() == other.getRemoveOwnedSnapshotFileEdge();
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        return getMakeFileTransient() == other.getMakeFileTransient();
    default:
        throw BINCODE_EXCEPTION("bad ShardMessageKind kind %s", _kind);
    }
}

std::ostream& operator<<(std::ostream& out, const ShardReqContainer& x) {
    switch (x.kind()) {
    case ShardMessageKind::LOOKUP:
        out << x.getLookup();
        break;
    case ShardMessageKind::STAT_FILE:
        out << x.getStatFile();
        break;
    case ShardMessageKind::STAT_DIRECTORY:
        out << x.getStatDirectory();
        break;
    case ShardMessageKind::READ_DIR:
        out << x.getReadDir();
        break;
    case ShardMessageKind::CONSTRUCT_FILE:
        out << x.getConstructFile();
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE:
        out << x.getAddSpanInitiate();
        break;
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        out << x.getAddSpanCertify();
        break;
    case ShardMessageKind::LINK_FILE:
        out << x.getLinkFile();
        break;
    case ShardMessageKind::SOFT_UNLINK_FILE:
        out << x.getSoftUnlinkFile();
        break;
    case ShardMessageKind::LOCAL_FILE_SPANS:
        out << x.getLocalFileSpans();
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        out << x.getSameDirectoryRename();
        break;
    case ShardMessageKind::ADD_INLINE_SPAN:
        out << x.getAddInlineSpan();
        break;
    case ShardMessageKind::SET_TIME:
        out << x.getSetTime();
        break;
    case ShardMessageKind::FULL_READ_DIR:
        out << x.getFullReadDir();
        break;
    case ShardMessageKind::MOVE_SPAN:
        out << x.getMoveSpan();
        break;
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        out << x.getRemoveNonOwnedEdge();
        break;
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        out << x.getSameShardHardFileUnlink();
        break;
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        out << x.getStatTransientFile();
        break;
    case ShardMessageKind::SHARD_SNAPSHOT:
        out << x.getShardSnapshot();
        break;
    case ShardMessageKind::FILE_SPANS:
        out << x.getFileSpans();
        break;
    case ShardMessageKind::ADD_SPAN_LOCATION:
        out << x.getAddSpanLocation();
        break;
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        out << x.getScrapTransientFile();
        break;
    case ShardMessageKind::SET_DIRECTORY_INFO:
        out << x.getSetDirectoryInfo();
        break;
    case ShardMessageKind::VISIT_DIRECTORIES:
        out << x.getVisitDirectories();
        break;
    case ShardMessageKind::VISIT_FILES:
        out << x.getVisitFiles();
        break;
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        out << x.getVisitTransientFiles();
        break;
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        out << x.getRemoveSpanInitiate();
        break;
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        out << x.getRemoveSpanCertify();
        break;
    case ShardMessageKind::SWAP_BLOCKS:
        out << x.getSwapBlocks();
        break;
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        out << x.getBlockServiceFiles();
        break;
    case ShardMessageKind::REMOVE_INODE:
        out << x.getRemoveInode();
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        out << x.getAddSpanInitiateWithReference();
        break;
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        out << x.getRemoveZeroBlockServiceFiles();
        break;
    case ShardMessageKind::SWAP_SPANS:
        out << x.getSwapSpans();
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        out << x.getSameDirectoryRenameSnapshot();
        break;
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        out << x.getAddSpanAtLocationInitiate();
        break;
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        out << x.getCreateDirectoryInode();
        break;
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        out << x.getSetDirectoryOwner();
        break;
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        out << x.getRemoveDirectoryOwner();
        break;
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        out << x.getCreateLockedCurrentEdge();
        break;
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        out << x.getLockCurrentEdge();
        break;
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        out << x.getUnlockCurrentEdge();
        break;
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        out << x.getRemoveOwnedSnapshotFileEdge();
        break;
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        out << x.getMakeFileTransient();
        break;
    case ShardMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        throw TERN_EXCEPTION("bad ShardMessageKind kind %s", x.kind());
    }
    return out;
}

const TernError& ShardRespContainer::getError() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ERROR, "%s != %s", _kind, ShardMessageKind::ERROR);
    return std::get<0>(_data);
}
TernError& ShardRespContainer::setError() {
    _kind = ShardMessageKind::ERROR;
    auto& x = _data.emplace<0>();
    return x;
}
const LookupResp& ShardRespContainer::getLookup() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::LOOKUP, "%s != %s", _kind, ShardMessageKind::LOOKUP);
    return std::get<1>(_data);
}
LookupResp& ShardRespContainer::setLookup() {
    _kind = ShardMessageKind::LOOKUP;
    auto& x = _data.emplace<1>();
    return x;
}
const StatFileResp& ShardRespContainer::getStatFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::STAT_FILE, "%s != %s", _kind, ShardMessageKind::STAT_FILE);
    return std::get<2>(_data);
}
StatFileResp& ShardRespContainer::setStatFile() {
    _kind = ShardMessageKind::STAT_FILE;
    auto& x = _data.emplace<2>();
    return x;
}
const StatDirectoryResp& ShardRespContainer::getStatDirectory() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::STAT_DIRECTORY, "%s != %s", _kind, ShardMessageKind::STAT_DIRECTORY);
    return std::get<3>(_data);
}
StatDirectoryResp& ShardRespContainer::setStatDirectory() {
    _kind = ShardMessageKind::STAT_DIRECTORY;
    auto& x = _data.emplace<3>();
    return x;
}
const ReadDirResp& ShardRespContainer::getReadDir() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::READ_DIR, "%s != %s", _kind, ShardMessageKind::READ_DIR);
    return std::get<4>(_data);
}
ReadDirResp& ShardRespContainer::setReadDir() {
    _kind = ShardMessageKind::READ_DIR;
    auto& x = _data.emplace<4>();
    return x;
}
const ConstructFileResp& ShardRespContainer::getConstructFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::CONSTRUCT_FILE, "%s != %s", _kind, ShardMessageKind::CONSTRUCT_FILE);
    return std::get<5>(_data);
}
ConstructFileResp& ShardRespContainer::setConstructFile() {
    _kind = ShardMessageKind::CONSTRUCT_FILE;
    auto& x = _data.emplace<5>();
    return x;
}
const AddSpanInitiateResp& ShardRespContainer::getAddSpanInitiate() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_SPAN_INITIATE, "%s != %s", _kind, ShardMessageKind::ADD_SPAN_INITIATE);
    return std::get<6>(_data);
}
AddSpanInitiateResp& ShardRespContainer::setAddSpanInitiate() {
    _kind = ShardMessageKind::ADD_SPAN_INITIATE;
    auto& x = _data.emplace<6>();
    return x;
}
const AddSpanCertifyResp& ShardRespContainer::getAddSpanCertify() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_SPAN_CERTIFY, "%s != %s", _kind, ShardMessageKind::ADD_SPAN_CERTIFY);
    return std::get<7>(_data);
}
AddSpanCertifyResp& ShardRespContainer::setAddSpanCertify() {
    _kind = ShardMessageKind::ADD_SPAN_CERTIFY;
    auto& x = _data.emplace<7>();
    return x;
}
const LinkFileResp& ShardRespContainer::getLinkFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::LINK_FILE, "%s != %s", _kind, ShardMessageKind::LINK_FILE);
    return std::get<8>(_data);
}
LinkFileResp& ShardRespContainer::setLinkFile() {
    _kind = ShardMessageKind::LINK_FILE;
    auto& x = _data.emplace<8>();
    return x;
}
const SoftUnlinkFileResp& ShardRespContainer::getSoftUnlinkFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SOFT_UNLINK_FILE, "%s != %s", _kind, ShardMessageKind::SOFT_UNLINK_FILE);
    return std::get<9>(_data);
}
SoftUnlinkFileResp& ShardRespContainer::setSoftUnlinkFile() {
    _kind = ShardMessageKind::SOFT_UNLINK_FILE;
    auto& x = _data.emplace<9>();
    return x;
}
const LocalFileSpansResp& ShardRespContainer::getLocalFileSpans() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::LOCAL_FILE_SPANS, "%s != %s", _kind, ShardMessageKind::LOCAL_FILE_SPANS);
    return std::get<10>(_data);
}
LocalFileSpansResp& ShardRespContainer::setLocalFileSpans() {
    _kind = ShardMessageKind::LOCAL_FILE_SPANS;
    auto& x = _data.emplace<10>();
    return x;
}
const SameDirectoryRenameResp& ShardRespContainer::getSameDirectoryRename() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SAME_DIRECTORY_RENAME, "%s != %s", _kind, ShardMessageKind::SAME_DIRECTORY_RENAME);
    return std::get<11>(_data);
}
SameDirectoryRenameResp& ShardRespContainer::setSameDirectoryRename() {
    _kind = ShardMessageKind::SAME_DIRECTORY_RENAME;
    auto& x = _data.emplace<11>();
    return x;
}
const AddInlineSpanResp& ShardRespContainer::getAddInlineSpan() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_INLINE_SPAN, "%s != %s", _kind, ShardMessageKind::ADD_INLINE_SPAN);
    return std::get<12>(_data);
}
AddInlineSpanResp& ShardRespContainer::setAddInlineSpan() {
    _kind = ShardMessageKind::ADD_INLINE_SPAN;
    auto& x = _data.emplace<12>();
    return x;
}
const SetTimeResp& ShardRespContainer::getSetTime() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SET_TIME, "%s != %s", _kind, ShardMessageKind::SET_TIME);
    return std::get<13>(_data);
}
SetTimeResp& ShardRespContainer::setSetTime() {
    _kind = ShardMessageKind::SET_TIME;
    auto& x = _data.emplace<13>();
    return x;
}
const FullReadDirResp& ShardRespContainer::getFullReadDir() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::FULL_READ_DIR, "%s != %s", _kind, ShardMessageKind::FULL_READ_DIR);
    return std::get<14>(_data);
}
FullReadDirResp& ShardRespContainer::setFullReadDir() {
    _kind = ShardMessageKind::FULL_READ_DIR;
    auto& x = _data.emplace<14>();
    return x;
}
const MoveSpanResp& ShardRespContainer::getMoveSpan() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::MOVE_SPAN, "%s != %s", _kind, ShardMessageKind::MOVE_SPAN);
    return std::get<15>(_data);
}
MoveSpanResp& ShardRespContainer::setMoveSpan() {
    _kind = ShardMessageKind::MOVE_SPAN;
    auto& x = _data.emplace<15>();
    return x;
}
const RemoveNonOwnedEdgeResp& ShardRespContainer::getRemoveNonOwnedEdge() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_NON_OWNED_EDGE, "%s != %s", _kind, ShardMessageKind::REMOVE_NON_OWNED_EDGE);
    return std::get<16>(_data);
}
RemoveNonOwnedEdgeResp& ShardRespContainer::setRemoveNonOwnedEdge() {
    _kind = ShardMessageKind::REMOVE_NON_OWNED_EDGE;
    auto& x = _data.emplace<16>();
    return x;
}
const SameShardHardFileUnlinkResp& ShardRespContainer::getSameShardHardFileUnlink() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK, "%s != %s", _kind, ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK);
    return std::get<17>(_data);
}
SameShardHardFileUnlinkResp& ShardRespContainer::setSameShardHardFileUnlink() {
    _kind = ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK;
    auto& x = _data.emplace<17>();
    return x;
}
const StatTransientFileResp& ShardRespContainer::getStatTransientFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::STAT_TRANSIENT_FILE, "%s != %s", _kind, ShardMessageKind::STAT_TRANSIENT_FILE);
    return std::get<18>(_data);
}
StatTransientFileResp& ShardRespContainer::setStatTransientFile() {
    _kind = ShardMessageKind::STAT_TRANSIENT_FILE;
    auto& x = _data.emplace<18>();
    return x;
}
const ShardSnapshotResp& ShardRespContainer::getShardSnapshot() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SHARD_SNAPSHOT, "%s != %s", _kind, ShardMessageKind::SHARD_SNAPSHOT);
    return std::get<19>(_data);
}
ShardSnapshotResp& ShardRespContainer::setShardSnapshot() {
    _kind = ShardMessageKind::SHARD_SNAPSHOT;
    auto& x = _data.emplace<19>();
    return x;
}
const FileSpansResp& ShardRespContainer::getFileSpans() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::FILE_SPANS, "%s != %s", _kind, ShardMessageKind::FILE_SPANS);
    return std::get<20>(_data);
}
FileSpansResp& ShardRespContainer::setFileSpans() {
    _kind = ShardMessageKind::FILE_SPANS;
    auto& x = _data.emplace<20>();
    return x;
}
const AddSpanLocationResp& ShardRespContainer::getAddSpanLocation() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_SPAN_LOCATION, "%s != %s", _kind, ShardMessageKind::ADD_SPAN_LOCATION);
    return std::get<21>(_data);
}
AddSpanLocationResp& ShardRespContainer::setAddSpanLocation() {
    _kind = ShardMessageKind::ADD_SPAN_LOCATION;
    auto& x = _data.emplace<21>();
    return x;
}
const ScrapTransientFileResp& ShardRespContainer::getScrapTransientFile() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SCRAP_TRANSIENT_FILE, "%s != %s", _kind, ShardMessageKind::SCRAP_TRANSIENT_FILE);
    return std::get<22>(_data);
}
ScrapTransientFileResp& ShardRespContainer::setScrapTransientFile() {
    _kind = ShardMessageKind::SCRAP_TRANSIENT_FILE;
    auto& x = _data.emplace<22>();
    return x;
}
const SetDirectoryInfoResp& ShardRespContainer::getSetDirectoryInfo() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SET_DIRECTORY_INFO, "%s != %s", _kind, ShardMessageKind::SET_DIRECTORY_INFO);
    return std::get<23>(_data);
}
SetDirectoryInfoResp& ShardRespContainer::setSetDirectoryInfo() {
    _kind = ShardMessageKind::SET_DIRECTORY_INFO;
    auto& x = _data.emplace<23>();
    return x;
}
const VisitDirectoriesResp& ShardRespContainer::getVisitDirectories() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::VISIT_DIRECTORIES, "%s != %s", _kind, ShardMessageKind::VISIT_DIRECTORIES);
    return std::get<24>(_data);
}
VisitDirectoriesResp& ShardRespContainer::setVisitDirectories() {
    _kind = ShardMessageKind::VISIT_DIRECTORIES;
    auto& x = _data.emplace<24>();
    return x;
}
const VisitFilesResp& ShardRespContainer::getVisitFiles() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::VISIT_FILES, "%s != %s", _kind, ShardMessageKind::VISIT_FILES);
    return std::get<25>(_data);
}
VisitFilesResp& ShardRespContainer::setVisitFiles() {
    _kind = ShardMessageKind::VISIT_FILES;
    auto& x = _data.emplace<25>();
    return x;
}
const VisitTransientFilesResp& ShardRespContainer::getVisitTransientFiles() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::VISIT_TRANSIENT_FILES, "%s != %s", _kind, ShardMessageKind::VISIT_TRANSIENT_FILES);
    return std::get<26>(_data);
}
VisitTransientFilesResp& ShardRespContainer::setVisitTransientFiles() {
    _kind = ShardMessageKind::VISIT_TRANSIENT_FILES;
    auto& x = _data.emplace<26>();
    return x;
}
const RemoveSpanInitiateResp& ShardRespContainer::getRemoveSpanInitiate() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_SPAN_INITIATE, "%s != %s", _kind, ShardMessageKind::REMOVE_SPAN_INITIATE);
    return std::get<27>(_data);
}
RemoveSpanInitiateResp& ShardRespContainer::setRemoveSpanInitiate() {
    _kind = ShardMessageKind::REMOVE_SPAN_INITIATE;
    auto& x = _data.emplace<27>();
    return x;
}
const RemoveSpanCertifyResp& ShardRespContainer::getRemoveSpanCertify() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_SPAN_CERTIFY, "%s != %s", _kind, ShardMessageKind::REMOVE_SPAN_CERTIFY);
    return std::get<28>(_data);
}
RemoveSpanCertifyResp& ShardRespContainer::setRemoveSpanCertify() {
    _kind = ShardMessageKind::REMOVE_SPAN_CERTIFY;
    auto& x = _data.emplace<28>();
    return x;
}
const SwapBlocksResp& ShardRespContainer::getSwapBlocks() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SWAP_BLOCKS, "%s != %s", _kind, ShardMessageKind::SWAP_BLOCKS);
    return std::get<29>(_data);
}
SwapBlocksResp& ShardRespContainer::setSwapBlocks() {
    _kind = ShardMessageKind::SWAP_BLOCKS;
    auto& x = _data.emplace<29>();
    return x;
}
const BlockServiceFilesResp& ShardRespContainer::getBlockServiceFiles() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::BLOCK_SERVICE_FILES, "%s != %s", _kind, ShardMessageKind::BLOCK_SERVICE_FILES);
    return std::get<30>(_data);
}
BlockServiceFilesResp& ShardRespContainer::setBlockServiceFiles() {
    _kind = ShardMessageKind::BLOCK_SERVICE_FILES;
    auto& x = _data.emplace<30>();
    return x;
}
const RemoveInodeResp& ShardRespContainer::getRemoveInode() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_INODE, "%s != %s", _kind, ShardMessageKind::REMOVE_INODE);
    return std::get<31>(_data);
}
RemoveInodeResp& ShardRespContainer::setRemoveInode() {
    _kind = ShardMessageKind::REMOVE_INODE;
    auto& x = _data.emplace<31>();
    return x;
}
const AddSpanInitiateWithReferenceResp& ShardRespContainer::getAddSpanInitiateWithReference() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE, "%s != %s", _kind, ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE);
    return std::get<32>(_data);
}
AddSpanInitiateWithReferenceResp& ShardRespContainer::setAddSpanInitiateWithReference() {
    _kind = ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE;
    auto& x = _data.emplace<32>();
    return x;
}
const RemoveZeroBlockServiceFilesResp& ShardRespContainer::getRemoveZeroBlockServiceFiles() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES, "%s != %s", _kind, ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES);
    return std::get<33>(_data);
}
RemoveZeroBlockServiceFilesResp& ShardRespContainer::setRemoveZeroBlockServiceFiles() {
    _kind = ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES;
    auto& x = _data.emplace<33>();
    return x;
}
const SwapSpansResp& ShardRespContainer::getSwapSpans() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SWAP_SPANS, "%s != %s", _kind, ShardMessageKind::SWAP_SPANS);
    return std::get<34>(_data);
}
SwapSpansResp& ShardRespContainer::setSwapSpans() {
    _kind = ShardMessageKind::SWAP_SPANS;
    auto& x = _data.emplace<34>();
    return x;
}
const SameDirectoryRenameSnapshotResp& ShardRespContainer::getSameDirectoryRenameSnapshot() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT, "%s != %s", _kind, ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT);
    return std::get<35>(_data);
}
SameDirectoryRenameSnapshotResp& ShardRespContainer::setSameDirectoryRenameSnapshot() {
    _kind = ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT;
    auto& x = _data.emplace<35>();
    return x;
}
const AddSpanAtLocationInitiateResp& ShardRespContainer::getAddSpanAtLocationInitiate() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE, "%s != %s", _kind, ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE);
    return std::get<36>(_data);
}
AddSpanAtLocationInitiateResp& ShardRespContainer::setAddSpanAtLocationInitiate() {
    _kind = ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE;
    auto& x = _data.emplace<36>();
    return x;
}
const CreateDirectoryInodeResp& ShardRespContainer::getCreateDirectoryInode() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::CREATE_DIRECTORY_INODE, "%s != %s", _kind, ShardMessageKind::CREATE_DIRECTORY_INODE);
    return std::get<37>(_data);
}
CreateDirectoryInodeResp& ShardRespContainer::setCreateDirectoryInode() {
    _kind = ShardMessageKind::CREATE_DIRECTORY_INODE;
    auto& x = _data.emplace<37>();
    return x;
}
const SetDirectoryOwnerResp& ShardRespContainer::getSetDirectoryOwner() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::SET_DIRECTORY_OWNER, "%s != %s", _kind, ShardMessageKind::SET_DIRECTORY_OWNER);
    return std::get<38>(_data);
}
SetDirectoryOwnerResp& ShardRespContainer::setSetDirectoryOwner() {
    _kind = ShardMessageKind::SET_DIRECTORY_OWNER;
    auto& x = _data.emplace<38>();
    return x;
}
const RemoveDirectoryOwnerResp& ShardRespContainer::getRemoveDirectoryOwner() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_DIRECTORY_OWNER, "%s != %s", _kind, ShardMessageKind::REMOVE_DIRECTORY_OWNER);
    return std::get<39>(_data);
}
RemoveDirectoryOwnerResp& ShardRespContainer::setRemoveDirectoryOwner() {
    _kind = ShardMessageKind::REMOVE_DIRECTORY_OWNER;
    auto& x = _data.emplace<39>();
    return x;
}
const CreateLockedCurrentEdgeResp& ShardRespContainer::getCreateLockedCurrentEdge() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE, "%s != %s", _kind, ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE);
    return std::get<40>(_data);
}
CreateLockedCurrentEdgeResp& ShardRespContainer::setCreateLockedCurrentEdge() {
    _kind = ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE;
    auto& x = _data.emplace<40>();
    return x;
}
const LockCurrentEdgeResp& ShardRespContainer::getLockCurrentEdge() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::LOCK_CURRENT_EDGE, "%s != %s", _kind, ShardMessageKind::LOCK_CURRENT_EDGE);
    return std::get<41>(_data);
}
LockCurrentEdgeResp& ShardRespContainer::setLockCurrentEdge() {
    _kind = ShardMessageKind::LOCK_CURRENT_EDGE;
    auto& x = _data.emplace<41>();
    return x;
}
const UnlockCurrentEdgeResp& ShardRespContainer::getUnlockCurrentEdge() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::UNLOCK_CURRENT_EDGE, "%s != %s", _kind, ShardMessageKind::UNLOCK_CURRENT_EDGE);
    return std::get<42>(_data);
}
UnlockCurrentEdgeResp& ShardRespContainer::setUnlockCurrentEdge() {
    _kind = ShardMessageKind::UNLOCK_CURRENT_EDGE;
    auto& x = _data.emplace<42>();
    return x;
}
const RemoveOwnedSnapshotFileEdgeResp& ShardRespContainer::getRemoveOwnedSnapshotFileEdge() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE, "%s != %s", _kind, ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE);
    return std::get<43>(_data);
}
RemoveOwnedSnapshotFileEdgeResp& ShardRespContainer::setRemoveOwnedSnapshotFileEdge() {
    _kind = ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE;
    auto& x = _data.emplace<43>();
    return x;
}
const MakeFileTransientResp& ShardRespContainer::getMakeFileTransient() const {
    ALWAYS_ASSERT(_kind == ShardMessageKind::MAKE_FILE_TRANSIENT, "%s != %s", _kind, ShardMessageKind::MAKE_FILE_TRANSIENT);
    return std::get<44>(_data);
}
MakeFileTransientResp& ShardRespContainer::setMakeFileTransient() {
    _kind = ShardMessageKind::MAKE_FILE_TRANSIENT;
    auto& x = _data.emplace<44>();
    return x;
}
ShardRespContainer::ShardRespContainer() {
    clear();
}

ShardRespContainer::ShardRespContainer(const ShardRespContainer& other) {
    *this = other;
}

ShardRespContainer::ShardRespContainer(ShardRespContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = ShardMessageKind::EMPTY;
}

void ShardRespContainer::operator=(const ShardRespContainer& other) {
    if (other.kind() == ShardMessageKind::EMPTY) { clear(); return; }
    switch (other.kind()) {
    case ShardMessageKind::ERROR:
        setError() = other.getError();
        break;
    case ShardMessageKind::LOOKUP:
        setLookup() = other.getLookup();
        break;
    case ShardMessageKind::STAT_FILE:
        setStatFile() = other.getStatFile();
        break;
    case ShardMessageKind::STAT_DIRECTORY:
        setStatDirectory() = other.getStatDirectory();
        break;
    case ShardMessageKind::READ_DIR:
        setReadDir() = other.getReadDir();
        break;
    case ShardMessageKind::CONSTRUCT_FILE:
        setConstructFile() = other.getConstructFile();
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE:
        setAddSpanInitiate() = other.getAddSpanInitiate();
        break;
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        setAddSpanCertify() = other.getAddSpanCertify();
        break;
    case ShardMessageKind::LINK_FILE:
        setLinkFile() = other.getLinkFile();
        break;
    case ShardMessageKind::SOFT_UNLINK_FILE:
        setSoftUnlinkFile() = other.getSoftUnlinkFile();
        break;
    case ShardMessageKind::LOCAL_FILE_SPANS:
        setLocalFileSpans() = other.getLocalFileSpans();
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        setSameDirectoryRename() = other.getSameDirectoryRename();
        break;
    case ShardMessageKind::ADD_INLINE_SPAN:
        setAddInlineSpan() = other.getAddInlineSpan();
        break;
    case ShardMessageKind::SET_TIME:
        setSetTime() = other.getSetTime();
        break;
    case ShardMessageKind::FULL_READ_DIR:
        setFullReadDir() = other.getFullReadDir();
        break;
    case ShardMessageKind::MOVE_SPAN:
        setMoveSpan() = other.getMoveSpan();
        break;
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        setRemoveNonOwnedEdge() = other.getRemoveNonOwnedEdge();
        break;
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        setSameShardHardFileUnlink() = other.getSameShardHardFileUnlink();
        break;
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        setStatTransientFile() = other.getStatTransientFile();
        break;
    case ShardMessageKind::SHARD_SNAPSHOT:
        setShardSnapshot() = other.getShardSnapshot();
        break;
    case ShardMessageKind::FILE_SPANS:
        setFileSpans() = other.getFileSpans();
        break;
    case ShardMessageKind::ADD_SPAN_LOCATION:
        setAddSpanLocation() = other.getAddSpanLocation();
        break;
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        setScrapTransientFile() = other.getScrapTransientFile();
        break;
    case ShardMessageKind::SET_DIRECTORY_INFO:
        setSetDirectoryInfo() = other.getSetDirectoryInfo();
        break;
    case ShardMessageKind::VISIT_DIRECTORIES:
        setVisitDirectories() = other.getVisitDirectories();
        break;
    case ShardMessageKind::VISIT_FILES:
        setVisitFiles() = other.getVisitFiles();
        break;
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        setVisitTransientFiles() = other.getVisitTransientFiles();
        break;
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        setRemoveSpanInitiate() = other.getRemoveSpanInitiate();
        break;
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        setRemoveSpanCertify() = other.getRemoveSpanCertify();
        break;
    case ShardMessageKind::SWAP_BLOCKS:
        setSwapBlocks() = other.getSwapBlocks();
        break;
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        setBlockServiceFiles() = other.getBlockServiceFiles();
        break;
    case ShardMessageKind::REMOVE_INODE:
        setRemoveInode() = other.getRemoveInode();
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        setAddSpanInitiateWithReference() = other.getAddSpanInitiateWithReference();
        break;
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        setRemoveZeroBlockServiceFiles() = other.getRemoveZeroBlockServiceFiles();
        break;
    case ShardMessageKind::SWAP_SPANS:
        setSwapSpans() = other.getSwapSpans();
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        setSameDirectoryRenameSnapshot() = other.getSameDirectoryRenameSnapshot();
        break;
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        setAddSpanAtLocationInitiate() = other.getAddSpanAtLocationInitiate();
        break;
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        setCreateDirectoryInode() = other.getCreateDirectoryInode();
        break;
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        setSetDirectoryOwner() = other.getSetDirectoryOwner();
        break;
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        setRemoveDirectoryOwner() = other.getRemoveDirectoryOwner();
        break;
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        setCreateLockedCurrentEdge() = other.getCreateLockedCurrentEdge();
        break;
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        setLockCurrentEdge() = other.getLockCurrentEdge();
        break;
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        setUnlockCurrentEdge() = other.getUnlockCurrentEdge();
        break;
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        setRemoveOwnedSnapshotFileEdge() = other.getRemoveOwnedSnapshotFileEdge();
        break;
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        setMakeFileTransient() = other.getMakeFileTransient();
        break;
    default:
        throw TERN_EXCEPTION("bad ShardMessageKind kind %s", other.kind());
    }
}

void ShardRespContainer::operator=(ShardRespContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = ShardMessageKind::EMPTY;
}

size_t ShardRespContainer::packedSize() const {
    switch (_kind) {
    case ShardMessageKind::ERROR:
        return sizeof(ShardMessageKind) + sizeof(TernError);
    case ShardMessageKind::LOOKUP:
        return sizeof(ShardMessageKind) + std::get<1>(_data).packedSize();
    case ShardMessageKind::STAT_FILE:
        return sizeof(ShardMessageKind) + std::get<2>(_data).packedSize();
    case ShardMessageKind::STAT_DIRECTORY:
        return sizeof(ShardMessageKind) + std::get<3>(_data).packedSize();
    case ShardMessageKind::READ_DIR:
        return sizeof(ShardMessageKind) + std::get<4>(_data).packedSize();
    case ShardMessageKind::CONSTRUCT_FILE:
        return sizeof(ShardMessageKind) + std::get<5>(_data).packedSize();
    case ShardMessageKind::ADD_SPAN_INITIATE:
        return sizeof(ShardMessageKind) + std::get<6>(_data).packedSize();
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        return sizeof(ShardMessageKind) + std::get<7>(_data).packedSize();
    case ShardMessageKind::LINK_FILE:
        return sizeof(ShardMessageKind) + std::get<8>(_data).packedSize();
    case ShardMessageKind::SOFT_UNLINK_FILE:
        return sizeof(ShardMessageKind) + std::get<9>(_data).packedSize();
    case ShardMessageKind::LOCAL_FILE_SPANS:
        return sizeof(ShardMessageKind) + std::get<10>(_data).packedSize();
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        return sizeof(ShardMessageKind) + std::get<11>(_data).packedSize();
    case ShardMessageKind::ADD_INLINE_SPAN:
        return sizeof(ShardMessageKind) + std::get<12>(_data).packedSize();
    case ShardMessageKind::SET_TIME:
        return sizeof(ShardMessageKind) + std::get<13>(_data).packedSize();
    case ShardMessageKind::FULL_READ_DIR:
        return sizeof(ShardMessageKind) + std::get<14>(_data).packedSize();
    case ShardMessageKind::MOVE_SPAN:
        return sizeof(ShardMessageKind) + std::get<15>(_data).packedSize();
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        return sizeof(ShardMessageKind) + std::get<16>(_data).packedSize();
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        return sizeof(ShardMessageKind) + std::get<17>(_data).packedSize();
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        return sizeof(ShardMessageKind) + std::get<18>(_data).packedSize();
    case ShardMessageKind::SHARD_SNAPSHOT:
        return sizeof(ShardMessageKind) + std::get<19>(_data).packedSize();
    case ShardMessageKind::FILE_SPANS:
        return sizeof(ShardMessageKind) + std::get<20>(_data).packedSize();
    case ShardMessageKind::ADD_SPAN_LOCATION:
        return sizeof(ShardMessageKind) + std::get<21>(_data).packedSize();
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        return sizeof(ShardMessageKind) + std::get<22>(_data).packedSize();
    case ShardMessageKind::SET_DIRECTORY_INFO:
        return sizeof(ShardMessageKind) + std::get<23>(_data).packedSize();
    case ShardMessageKind::VISIT_DIRECTORIES:
        return sizeof(ShardMessageKind) + std::get<24>(_data).packedSize();
    case ShardMessageKind::VISIT_FILES:
        return sizeof(ShardMessageKind) + std::get<25>(_data).packedSize();
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        return sizeof(ShardMessageKind) + std::get<26>(_data).packedSize();
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        return sizeof(ShardMessageKind) + std::get<27>(_data).packedSize();
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        return sizeof(ShardMessageKind) + std::get<28>(_data).packedSize();
    case ShardMessageKind::SWAP_BLOCKS:
        return sizeof(ShardMessageKind) + std::get<29>(_data).packedSize();
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        return sizeof(ShardMessageKind) + std::get<30>(_data).packedSize();
    case ShardMessageKind::REMOVE_INODE:
        return sizeof(ShardMessageKind) + std::get<31>(_data).packedSize();
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        return sizeof(ShardMessageKind) + std::get<32>(_data).packedSize();
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        return sizeof(ShardMessageKind) + std::get<33>(_data).packedSize();
    case ShardMessageKind::SWAP_SPANS:
        return sizeof(ShardMessageKind) + std::get<34>(_data).packedSize();
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        return sizeof(ShardMessageKind) + std::get<35>(_data).packedSize();
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        return sizeof(ShardMessageKind) + std::get<36>(_data).packedSize();
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        return sizeof(ShardMessageKind) + std::get<37>(_data).packedSize();
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        return sizeof(ShardMessageKind) + std::get<38>(_data).packedSize();
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        return sizeof(ShardMessageKind) + std::get<39>(_data).packedSize();
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        return sizeof(ShardMessageKind) + std::get<40>(_data).packedSize();
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        return sizeof(ShardMessageKind) + std::get<41>(_data).packedSize();
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        return sizeof(ShardMessageKind) + std::get<42>(_data).packedSize();
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        return sizeof(ShardMessageKind) + std::get<43>(_data).packedSize();
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        return sizeof(ShardMessageKind) + std::get<44>(_data).packedSize();
    default:
        throw TERN_EXCEPTION("bad ShardMessageKind kind %s", _kind);
    }
}

void ShardRespContainer::pack(BincodeBuf& buf) const {
    buf.packScalar<ShardMessageKind>(_kind);
    switch (_kind) {
    case ShardMessageKind::ERROR:
        buf.packScalar<TernError>(std::get<0>(_data));
        break;
    case ShardMessageKind::LOOKUP:
        std::get<1>(_data).pack(buf);
        break;
    case ShardMessageKind::STAT_FILE:
        std::get<2>(_data).pack(buf);
        break;
    case ShardMessageKind::STAT_DIRECTORY:
        std::get<3>(_data).pack(buf);
        break;
    case ShardMessageKind::READ_DIR:
        std::get<4>(_data).pack(buf);
        break;
    case ShardMessageKind::CONSTRUCT_FILE:
        std::get<5>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE:
        std::get<6>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        std::get<7>(_data).pack(buf);
        break;
    case ShardMessageKind::LINK_FILE:
        std::get<8>(_data).pack(buf);
        break;
    case ShardMessageKind::SOFT_UNLINK_FILE:
        std::get<9>(_data).pack(buf);
        break;
    case ShardMessageKind::LOCAL_FILE_SPANS:
        std::get<10>(_data).pack(buf);
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        std::get<11>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_INLINE_SPAN:
        std::get<12>(_data).pack(buf);
        break;
    case ShardMessageKind::SET_TIME:
        std::get<13>(_data).pack(buf);
        break;
    case ShardMessageKind::FULL_READ_DIR:
        std::get<14>(_data).pack(buf);
        break;
    case ShardMessageKind::MOVE_SPAN:
        std::get<15>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        std::get<16>(_data).pack(buf);
        break;
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        std::get<17>(_data).pack(buf);
        break;
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        std::get<18>(_data).pack(buf);
        break;
    case ShardMessageKind::SHARD_SNAPSHOT:
        std::get<19>(_data).pack(buf);
        break;
    case ShardMessageKind::FILE_SPANS:
        std::get<20>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_LOCATION:
        std::get<21>(_data).pack(buf);
        break;
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        std::get<22>(_data).pack(buf);
        break;
    case ShardMessageKind::SET_DIRECTORY_INFO:
        std::get<23>(_data).pack(buf);
        break;
    case ShardMessageKind::VISIT_DIRECTORIES:
        std::get<24>(_data).pack(buf);
        break;
    case ShardMessageKind::VISIT_FILES:
        std::get<25>(_data).pack(buf);
        break;
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        std::get<26>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        std::get<27>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        std::get<28>(_data).pack(buf);
        break;
    case ShardMessageKind::SWAP_BLOCKS:
        std::get<29>(_data).pack(buf);
        break;
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        std::get<30>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_INODE:
        std::get<31>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        std::get<32>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        std::get<33>(_data).pack(buf);
        break;
    case ShardMessageKind::SWAP_SPANS:
        std::get<34>(_data).pack(buf);
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        std::get<35>(_data).pack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        std::get<36>(_data).pack(buf);
        break;
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        std::get<37>(_data).pack(buf);
        break;
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        std::get<38>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        std::get<39>(_data).pack(buf);
        break;
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        std::get<40>(_data).pack(buf);
        break;
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        std::get<41>(_data).pack(buf);
        break;
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        std::get<42>(_data).pack(buf);
        break;
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        std::get<43>(_data).pack(buf);
        break;
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        std::get<44>(_data).pack(buf);
        break;
    default:
        throw TERN_EXCEPTION("bad ShardMessageKind kind %s", _kind);
    }
}

void ShardRespContainer::unpack(BincodeBuf& buf) {
    _kind = buf.unpackScalar<ShardMessageKind>();
    switch (_kind) {
    case ShardMessageKind::ERROR:
        _data.emplace<0>(buf.unpackScalar<TernError>());
        break;
    case ShardMessageKind::LOOKUP:
        _data.emplace<1>().unpack(buf);
        break;
    case ShardMessageKind::STAT_FILE:
        _data.emplace<2>().unpack(buf);
        break;
    case ShardMessageKind::STAT_DIRECTORY:
        _data.emplace<3>().unpack(buf);
        break;
    case ShardMessageKind::READ_DIR:
        _data.emplace<4>().unpack(buf);
        break;
    case ShardMessageKind::CONSTRUCT_FILE:
        _data.emplace<5>().unpack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE:
        _data.emplace<6>().unpack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        _data.emplace<7>().unpack(buf);
        break;
    case ShardMessageKind::LINK_FILE:
        _data.emplace<8>().unpack(buf);
        break;
    case ShardMessageKind::SOFT_UNLINK_FILE:
        _data.emplace<9>().unpack(buf);
        break;
    case ShardMessageKind::LOCAL_FILE_SPANS:
        _data.emplace<10>().unpack(buf);
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        _data.emplace<11>().unpack(buf);
        break;
    case ShardMessageKind::ADD_INLINE_SPAN:
        _data.emplace<12>().unpack(buf);
        break;
    case ShardMessageKind::SET_TIME:
        _data.emplace<13>().unpack(buf);
        break;
    case ShardMessageKind::FULL_READ_DIR:
        _data.emplace<14>().unpack(buf);
        break;
    case ShardMessageKind::MOVE_SPAN:
        _data.emplace<15>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        _data.emplace<16>().unpack(buf);
        break;
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        _data.emplace<17>().unpack(buf);
        break;
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        _data.emplace<18>().unpack(buf);
        break;
    case ShardMessageKind::SHARD_SNAPSHOT:
        _data.emplace<19>().unpack(buf);
        break;
    case ShardMessageKind::FILE_SPANS:
        _data.emplace<20>().unpack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_LOCATION:
        _data.emplace<21>().unpack(buf);
        break;
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        _data.emplace<22>().unpack(buf);
        break;
    case ShardMessageKind::SET_DIRECTORY_INFO:
        _data.emplace<23>().unpack(buf);
        break;
    case ShardMessageKind::VISIT_DIRECTORIES:
        _data.emplace<24>().unpack(buf);
        break;
    case ShardMessageKind::VISIT_FILES:
        _data.emplace<25>().unpack(buf);
        break;
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        _data.emplace<26>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        _data.emplace<27>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        _data.emplace<28>().unpack(buf);
        break;
    case ShardMessageKind::SWAP_BLOCKS:
        _data.emplace<29>().unpack(buf);
        break;
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        _data.emplace<30>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_INODE:
        _data.emplace<31>().unpack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        _data.emplace<32>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        _data.emplace<33>().unpack(buf);
        break;
    case ShardMessageKind::SWAP_SPANS:
        _data.emplace<34>().unpack(buf);
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        _data.emplace<35>().unpack(buf);
        break;
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        _data.emplace<36>().unpack(buf);
        break;
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        _data.emplace<37>().unpack(buf);
        break;
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        _data.emplace<38>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        _data.emplace<39>().unpack(buf);
        break;
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        _data.emplace<40>().unpack(buf);
        break;
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        _data.emplace<41>().unpack(buf);
        break;
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        _data.emplace<42>().unpack(buf);
        break;
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        _data.emplace<43>().unpack(buf);
        break;
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        _data.emplace<44>().unpack(buf);
        break;
    default:
        throw BINCODE_EXCEPTION("bad ShardMessageKind kind %s", _kind);
    }
}

bool ShardRespContainer::operator==(const ShardRespContainer& other) const {
    if (_kind != other.kind()) { return false; }
    if (_kind == ShardMessageKind::EMPTY) { return true; }
    switch (_kind) {
    case ShardMessageKind::ERROR:
        return getError() == other.getError();
    case ShardMessageKind::LOOKUP:
        return getLookup() == other.getLookup();
    case ShardMessageKind::STAT_FILE:
        return getStatFile() == other.getStatFile();
    case ShardMessageKind::STAT_DIRECTORY:
        return getStatDirectory() == other.getStatDirectory();
    case ShardMessageKind::READ_DIR:
        return getReadDir() == other.getReadDir();
    case ShardMessageKind::CONSTRUCT_FILE:
        return getConstructFile() == other.getConstructFile();
    case ShardMessageKind::ADD_SPAN_INITIATE:
        return getAddSpanInitiate() == other.getAddSpanInitiate();
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        return getAddSpanCertify() == other.getAddSpanCertify();
    case ShardMessageKind::LINK_FILE:
        return getLinkFile() == other.getLinkFile();
    case ShardMessageKind::SOFT_UNLINK_FILE:
        return getSoftUnlinkFile() == other.getSoftUnlinkFile();
    case ShardMessageKind::LOCAL_FILE_SPANS:
        return getLocalFileSpans() == other.getLocalFileSpans();
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        return getSameDirectoryRename() == other.getSameDirectoryRename();
    case ShardMessageKind::ADD_INLINE_SPAN:
        return getAddInlineSpan() == other.getAddInlineSpan();
    case ShardMessageKind::SET_TIME:
        return getSetTime() == other.getSetTime();
    case ShardMessageKind::FULL_READ_DIR:
        return getFullReadDir() == other.getFullReadDir();
    case ShardMessageKind::MOVE_SPAN:
        return getMoveSpan() == other.getMoveSpan();
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        return getRemoveNonOwnedEdge() == other.getRemoveNonOwnedEdge();
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        return getSameShardHardFileUnlink() == other.getSameShardHardFileUnlink();
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        return getStatTransientFile() == other.getStatTransientFile();
    case ShardMessageKind::SHARD_SNAPSHOT:
        return getShardSnapshot() == other.getShardSnapshot();
    case ShardMessageKind::FILE_SPANS:
        return getFileSpans() == other.getFileSpans();
    case ShardMessageKind::ADD_SPAN_LOCATION:
        return getAddSpanLocation() == other.getAddSpanLocation();
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        return getScrapTransientFile() == other.getScrapTransientFile();
    case ShardMessageKind::SET_DIRECTORY_INFO:
        return getSetDirectoryInfo() == other.getSetDirectoryInfo();
    case ShardMessageKind::VISIT_DIRECTORIES:
        return getVisitDirectories() == other.getVisitDirectories();
    case ShardMessageKind::VISIT_FILES:
        return getVisitFiles() == other.getVisitFiles();
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        return getVisitTransientFiles() == other.getVisitTransientFiles();
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        return getRemoveSpanInitiate() == other.getRemoveSpanInitiate();
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        return getRemoveSpanCertify() == other.getRemoveSpanCertify();
    case ShardMessageKind::SWAP_BLOCKS:
        return getSwapBlocks() == other.getSwapBlocks();
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        return getBlockServiceFiles() == other.getBlockServiceFiles();
    case ShardMessageKind::REMOVE_INODE:
        return getRemoveInode() == other.getRemoveInode();
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        return getAddSpanInitiateWithReference() == other.getAddSpanInitiateWithReference();
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        return getRemoveZeroBlockServiceFiles() == other.getRemoveZeroBlockServiceFiles();
    case ShardMessageKind::SWAP_SPANS:
        return getSwapSpans() == other.getSwapSpans();
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        return getSameDirectoryRenameSnapshot() == other.getSameDirectoryRenameSnapshot();
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        return getAddSpanAtLocationInitiate() == other.getAddSpanAtLocationInitiate();
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        return getCreateDirectoryInode() == other.getCreateDirectoryInode();
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        return getSetDirectoryOwner() == other.getSetDirectoryOwner();
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        return getRemoveDirectoryOwner() == other.getRemoveDirectoryOwner();
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        return getCreateLockedCurrentEdge() == other.getCreateLockedCurrentEdge();
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        return getLockCurrentEdge() == other.getLockCurrentEdge();
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        return getUnlockCurrentEdge() == other.getUnlockCurrentEdge();
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        return getRemoveOwnedSnapshotFileEdge() == other.getRemoveOwnedSnapshotFileEdge();
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        return getMakeFileTransient() == other.getMakeFileTransient();
    default:
        throw BINCODE_EXCEPTION("bad ShardMessageKind kind %s", _kind);
    }
}

std::ostream& operator<<(std::ostream& out, const ShardRespContainer& x) {
    switch (x.kind()) {
    case ShardMessageKind::ERROR:
        out << x.getError();
        break;
    case ShardMessageKind::LOOKUP:
        out << x.getLookup();
        break;
    case ShardMessageKind::STAT_FILE:
        out << x.getStatFile();
        break;
    case ShardMessageKind::STAT_DIRECTORY:
        out << x.getStatDirectory();
        break;
    case ShardMessageKind::READ_DIR:
        out << x.getReadDir();
        break;
    case ShardMessageKind::CONSTRUCT_FILE:
        out << x.getConstructFile();
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE:
        out << x.getAddSpanInitiate();
        break;
    case ShardMessageKind::ADD_SPAN_CERTIFY:
        out << x.getAddSpanCertify();
        break;
    case ShardMessageKind::LINK_FILE:
        out << x.getLinkFile();
        break;
    case ShardMessageKind::SOFT_UNLINK_FILE:
        out << x.getSoftUnlinkFile();
        break;
    case ShardMessageKind::LOCAL_FILE_SPANS:
        out << x.getLocalFileSpans();
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME:
        out << x.getSameDirectoryRename();
        break;
    case ShardMessageKind::ADD_INLINE_SPAN:
        out << x.getAddInlineSpan();
        break;
    case ShardMessageKind::SET_TIME:
        out << x.getSetTime();
        break;
    case ShardMessageKind::FULL_READ_DIR:
        out << x.getFullReadDir();
        break;
    case ShardMessageKind::MOVE_SPAN:
        out << x.getMoveSpan();
        break;
    case ShardMessageKind::REMOVE_NON_OWNED_EDGE:
        out << x.getRemoveNonOwnedEdge();
        break;
    case ShardMessageKind::SAME_SHARD_HARD_FILE_UNLINK:
        out << x.getSameShardHardFileUnlink();
        break;
    case ShardMessageKind::STAT_TRANSIENT_FILE:
        out << x.getStatTransientFile();
        break;
    case ShardMessageKind::SHARD_SNAPSHOT:
        out << x.getShardSnapshot();
        break;
    case ShardMessageKind::FILE_SPANS:
        out << x.getFileSpans();
        break;
    case ShardMessageKind::ADD_SPAN_LOCATION:
        out << x.getAddSpanLocation();
        break;
    case ShardMessageKind::SCRAP_TRANSIENT_FILE:
        out << x.getScrapTransientFile();
        break;
    case ShardMessageKind::SET_DIRECTORY_INFO:
        out << x.getSetDirectoryInfo();
        break;
    case ShardMessageKind::VISIT_DIRECTORIES:
        out << x.getVisitDirectories();
        break;
    case ShardMessageKind::VISIT_FILES:
        out << x.getVisitFiles();
        break;
    case ShardMessageKind::VISIT_TRANSIENT_FILES:
        out << x.getVisitTransientFiles();
        break;
    case ShardMessageKind::REMOVE_SPAN_INITIATE:
        out << x.getRemoveSpanInitiate();
        break;
    case ShardMessageKind::REMOVE_SPAN_CERTIFY:
        out << x.getRemoveSpanCertify();
        break;
    case ShardMessageKind::SWAP_BLOCKS:
        out << x.getSwapBlocks();
        break;
    case ShardMessageKind::BLOCK_SERVICE_FILES:
        out << x.getBlockServiceFiles();
        break;
    case ShardMessageKind::REMOVE_INODE:
        out << x.getRemoveInode();
        break;
    case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
        out << x.getAddSpanInitiateWithReference();
        break;
    case ShardMessageKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        out << x.getRemoveZeroBlockServiceFiles();
        break;
    case ShardMessageKind::SWAP_SPANS:
        out << x.getSwapSpans();
        break;
    case ShardMessageKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        out << x.getSameDirectoryRenameSnapshot();
        break;
    case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
        out << x.getAddSpanAtLocationInitiate();
        break;
    case ShardMessageKind::CREATE_DIRECTORY_INODE:
        out << x.getCreateDirectoryInode();
        break;
    case ShardMessageKind::SET_DIRECTORY_OWNER:
        out << x.getSetDirectoryOwner();
        break;
    case ShardMessageKind::REMOVE_DIRECTORY_OWNER:
        out << x.getRemoveDirectoryOwner();
        break;
    case ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE:
        out << x.getCreateLockedCurrentEdge();
        break;
    case ShardMessageKind::LOCK_CURRENT_EDGE:
        out << x.getLockCurrentEdge();
        break;
    case ShardMessageKind::UNLOCK_CURRENT_EDGE:
        out << x.getUnlockCurrentEdge();
        break;
    case ShardMessageKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        out << x.getRemoveOwnedSnapshotFileEdge();
        break;
    case ShardMessageKind::MAKE_FILE_TRANSIENT:
        out << x.getMakeFileTransient();
        break;
    case ShardMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        throw TERN_EXCEPTION("bad ShardMessageKind kind %s", x.kind());
    }
    return out;
}

const MakeDirectoryReq& CDCReqContainer::getMakeDirectory() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::MAKE_DIRECTORY, "%s != %s", _kind, CDCMessageKind::MAKE_DIRECTORY);
    return std::get<0>(_data);
}
MakeDirectoryReq& CDCReqContainer::setMakeDirectory() {
    _kind = CDCMessageKind::MAKE_DIRECTORY;
    auto& x = _data.emplace<0>();
    return x;
}
const RenameFileReq& CDCReqContainer::getRenameFile() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::RENAME_FILE, "%s != %s", _kind, CDCMessageKind::RENAME_FILE);
    return std::get<1>(_data);
}
RenameFileReq& CDCReqContainer::setRenameFile() {
    _kind = CDCMessageKind::RENAME_FILE;
    auto& x = _data.emplace<1>();
    return x;
}
const SoftUnlinkDirectoryReq& CDCReqContainer::getSoftUnlinkDirectory() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::SOFT_UNLINK_DIRECTORY, "%s != %s", _kind, CDCMessageKind::SOFT_UNLINK_DIRECTORY);
    return std::get<2>(_data);
}
SoftUnlinkDirectoryReq& CDCReqContainer::setSoftUnlinkDirectory() {
    _kind = CDCMessageKind::SOFT_UNLINK_DIRECTORY;
    auto& x = _data.emplace<2>();
    return x;
}
const RenameDirectoryReq& CDCReqContainer::getRenameDirectory() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::RENAME_DIRECTORY, "%s != %s", _kind, CDCMessageKind::RENAME_DIRECTORY);
    return std::get<3>(_data);
}
RenameDirectoryReq& CDCReqContainer::setRenameDirectory() {
    _kind = CDCMessageKind::RENAME_DIRECTORY;
    auto& x = _data.emplace<3>();
    return x;
}
const HardUnlinkDirectoryReq& CDCReqContainer::getHardUnlinkDirectory() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::HARD_UNLINK_DIRECTORY, "%s != %s", _kind, CDCMessageKind::HARD_UNLINK_DIRECTORY);
    return std::get<4>(_data);
}
HardUnlinkDirectoryReq& CDCReqContainer::setHardUnlinkDirectory() {
    _kind = CDCMessageKind::HARD_UNLINK_DIRECTORY;
    auto& x = _data.emplace<4>();
    return x;
}
const CrossShardHardUnlinkFileReq& CDCReqContainer::getCrossShardHardUnlinkFile() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE, "%s != %s", _kind, CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE);
    return std::get<5>(_data);
}
CrossShardHardUnlinkFileReq& CDCReqContainer::setCrossShardHardUnlinkFile() {
    _kind = CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE;
    auto& x = _data.emplace<5>();
    return x;
}
const CdcSnapshotReq& CDCReqContainer::getCdcSnapshot() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::CDC_SNAPSHOT, "%s != %s", _kind, CDCMessageKind::CDC_SNAPSHOT);
    return std::get<6>(_data);
}
CdcSnapshotReq& CDCReqContainer::setCdcSnapshot() {
    _kind = CDCMessageKind::CDC_SNAPSHOT;
    auto& x = _data.emplace<6>();
    return x;
}
CDCReqContainer::CDCReqContainer() {
    clear();
}

CDCReqContainer::CDCReqContainer(const CDCReqContainer& other) {
    *this = other;
}

CDCReqContainer::CDCReqContainer(CDCReqContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = CDCMessageKind::EMPTY;
}

void CDCReqContainer::operator=(const CDCReqContainer& other) {
    if (other.kind() == CDCMessageKind::EMPTY) { clear(); return; }
    switch (other.kind()) {
    case CDCMessageKind::MAKE_DIRECTORY:
        setMakeDirectory() = other.getMakeDirectory();
        break;
    case CDCMessageKind::RENAME_FILE:
        setRenameFile() = other.getRenameFile();
        break;
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        setSoftUnlinkDirectory() = other.getSoftUnlinkDirectory();
        break;
    case CDCMessageKind::RENAME_DIRECTORY:
        setRenameDirectory() = other.getRenameDirectory();
        break;
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        setHardUnlinkDirectory() = other.getHardUnlinkDirectory();
        break;
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        setCrossShardHardUnlinkFile() = other.getCrossShardHardUnlinkFile();
        break;
    case CDCMessageKind::CDC_SNAPSHOT:
        setCdcSnapshot() = other.getCdcSnapshot();
        break;
    default:
        throw TERN_EXCEPTION("bad CDCMessageKind kind %s", other.kind());
    }
}

void CDCReqContainer::operator=(CDCReqContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = CDCMessageKind::EMPTY;
}

size_t CDCReqContainer::packedSize() const {
    switch (_kind) {
    case CDCMessageKind::MAKE_DIRECTORY:
        return sizeof(CDCMessageKind) + std::get<0>(_data).packedSize();
    case CDCMessageKind::RENAME_FILE:
        return sizeof(CDCMessageKind) + std::get<1>(_data).packedSize();
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        return sizeof(CDCMessageKind) + std::get<2>(_data).packedSize();
    case CDCMessageKind::RENAME_DIRECTORY:
        return sizeof(CDCMessageKind) + std::get<3>(_data).packedSize();
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        return sizeof(CDCMessageKind) + std::get<4>(_data).packedSize();
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        return sizeof(CDCMessageKind) + std::get<5>(_data).packedSize();
    case CDCMessageKind::CDC_SNAPSHOT:
        return sizeof(CDCMessageKind) + std::get<6>(_data).packedSize();
    default:
        throw TERN_EXCEPTION("bad CDCMessageKind kind %s", _kind);
    }
}

void CDCReqContainer::pack(BincodeBuf& buf) const {
    buf.packScalar<CDCMessageKind>(_kind);
    switch (_kind) {
    case CDCMessageKind::MAKE_DIRECTORY:
        std::get<0>(_data).pack(buf);
        break;
    case CDCMessageKind::RENAME_FILE:
        std::get<1>(_data).pack(buf);
        break;
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        std::get<2>(_data).pack(buf);
        break;
    case CDCMessageKind::RENAME_DIRECTORY:
        std::get<3>(_data).pack(buf);
        break;
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        std::get<4>(_data).pack(buf);
        break;
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        std::get<5>(_data).pack(buf);
        break;
    case CDCMessageKind::CDC_SNAPSHOT:
        std::get<6>(_data).pack(buf);
        break;
    default:
        throw TERN_EXCEPTION("bad CDCMessageKind kind %s", _kind);
    }
}

void CDCReqContainer::unpack(BincodeBuf& buf) {
    _kind = buf.unpackScalar<CDCMessageKind>();
    switch (_kind) {
    case CDCMessageKind::MAKE_DIRECTORY:
        _data.emplace<0>().unpack(buf);
        break;
    case CDCMessageKind::RENAME_FILE:
        _data.emplace<1>().unpack(buf);
        break;
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        _data.emplace<2>().unpack(buf);
        break;
    case CDCMessageKind::RENAME_DIRECTORY:
        _data.emplace<3>().unpack(buf);
        break;
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        _data.emplace<4>().unpack(buf);
        break;
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        _data.emplace<5>().unpack(buf);
        break;
    case CDCMessageKind::CDC_SNAPSHOT:
        _data.emplace<6>().unpack(buf);
        break;
    default:
        throw BINCODE_EXCEPTION("bad CDCMessageKind kind %s", _kind);
    }
}

bool CDCReqContainer::operator==(const CDCReqContainer& other) const {
    if (_kind != other.kind()) { return false; }
    if (_kind == CDCMessageKind::EMPTY) { return true; }
    switch (_kind) {
    case CDCMessageKind::MAKE_DIRECTORY:
        return getMakeDirectory() == other.getMakeDirectory();
    case CDCMessageKind::RENAME_FILE:
        return getRenameFile() == other.getRenameFile();
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        return getSoftUnlinkDirectory() == other.getSoftUnlinkDirectory();
    case CDCMessageKind::RENAME_DIRECTORY:
        return getRenameDirectory() == other.getRenameDirectory();
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        return getHardUnlinkDirectory() == other.getHardUnlinkDirectory();
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        return getCrossShardHardUnlinkFile() == other.getCrossShardHardUnlinkFile();
    case CDCMessageKind::CDC_SNAPSHOT:
        return getCdcSnapshot() == other.getCdcSnapshot();
    default:
        throw BINCODE_EXCEPTION("bad CDCMessageKind kind %s", _kind);
    }
}

std::ostream& operator<<(std::ostream& out, const CDCReqContainer& x) {
    switch (x.kind()) {
    case CDCMessageKind::MAKE_DIRECTORY:
        out << x.getMakeDirectory();
        break;
    case CDCMessageKind::RENAME_FILE:
        out << x.getRenameFile();
        break;
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        out << x.getSoftUnlinkDirectory();
        break;
    case CDCMessageKind::RENAME_DIRECTORY:
        out << x.getRenameDirectory();
        break;
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        out << x.getHardUnlinkDirectory();
        break;
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        out << x.getCrossShardHardUnlinkFile();
        break;
    case CDCMessageKind::CDC_SNAPSHOT:
        out << x.getCdcSnapshot();
        break;
    case CDCMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        throw TERN_EXCEPTION("bad CDCMessageKind kind %s", x.kind());
    }
    return out;
}

const TernError& CDCRespContainer::getError() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::ERROR, "%s != %s", _kind, CDCMessageKind::ERROR);
    return std::get<0>(_data);
}
TernError& CDCRespContainer::setError() {
    _kind = CDCMessageKind::ERROR;
    auto& x = _data.emplace<0>();
    return x;
}
const MakeDirectoryResp& CDCRespContainer::getMakeDirectory() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::MAKE_DIRECTORY, "%s != %s", _kind, CDCMessageKind::MAKE_DIRECTORY);
    return std::get<1>(_data);
}
MakeDirectoryResp& CDCRespContainer::setMakeDirectory() {
    _kind = CDCMessageKind::MAKE_DIRECTORY;
    auto& x = _data.emplace<1>();
    return x;
}
const RenameFileResp& CDCRespContainer::getRenameFile() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::RENAME_FILE, "%s != %s", _kind, CDCMessageKind::RENAME_FILE);
    return std::get<2>(_data);
}
RenameFileResp& CDCRespContainer::setRenameFile() {
    _kind = CDCMessageKind::RENAME_FILE;
    auto& x = _data.emplace<2>();
    return x;
}
const SoftUnlinkDirectoryResp& CDCRespContainer::getSoftUnlinkDirectory() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::SOFT_UNLINK_DIRECTORY, "%s != %s", _kind, CDCMessageKind::SOFT_UNLINK_DIRECTORY);
    return std::get<3>(_data);
}
SoftUnlinkDirectoryResp& CDCRespContainer::setSoftUnlinkDirectory() {
    _kind = CDCMessageKind::SOFT_UNLINK_DIRECTORY;
    auto& x = _data.emplace<3>();
    return x;
}
const RenameDirectoryResp& CDCRespContainer::getRenameDirectory() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::RENAME_DIRECTORY, "%s != %s", _kind, CDCMessageKind::RENAME_DIRECTORY);
    return std::get<4>(_data);
}
RenameDirectoryResp& CDCRespContainer::setRenameDirectory() {
    _kind = CDCMessageKind::RENAME_DIRECTORY;
    auto& x = _data.emplace<4>();
    return x;
}
const HardUnlinkDirectoryResp& CDCRespContainer::getHardUnlinkDirectory() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::HARD_UNLINK_DIRECTORY, "%s != %s", _kind, CDCMessageKind::HARD_UNLINK_DIRECTORY);
    return std::get<5>(_data);
}
HardUnlinkDirectoryResp& CDCRespContainer::setHardUnlinkDirectory() {
    _kind = CDCMessageKind::HARD_UNLINK_DIRECTORY;
    auto& x = _data.emplace<5>();
    return x;
}
const CrossShardHardUnlinkFileResp& CDCRespContainer::getCrossShardHardUnlinkFile() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE, "%s != %s", _kind, CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE);
    return std::get<6>(_data);
}
CrossShardHardUnlinkFileResp& CDCRespContainer::setCrossShardHardUnlinkFile() {
    _kind = CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE;
    auto& x = _data.emplace<6>();
    return x;
}
const CdcSnapshotResp& CDCRespContainer::getCdcSnapshot() const {
    ALWAYS_ASSERT(_kind == CDCMessageKind::CDC_SNAPSHOT, "%s != %s", _kind, CDCMessageKind::CDC_SNAPSHOT);
    return std::get<7>(_data);
}
CdcSnapshotResp& CDCRespContainer::setCdcSnapshot() {
    _kind = CDCMessageKind::CDC_SNAPSHOT;
    auto& x = _data.emplace<7>();
    return x;
}
CDCRespContainer::CDCRespContainer() {
    clear();
}

CDCRespContainer::CDCRespContainer(const CDCRespContainer& other) {
    *this = other;
}

CDCRespContainer::CDCRespContainer(CDCRespContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = CDCMessageKind::EMPTY;
}

void CDCRespContainer::operator=(const CDCRespContainer& other) {
    if (other.kind() == CDCMessageKind::EMPTY) { clear(); return; }
    switch (other.kind()) {
    case CDCMessageKind::ERROR:
        setError() = other.getError();
        break;
    case CDCMessageKind::MAKE_DIRECTORY:
        setMakeDirectory() = other.getMakeDirectory();
        break;
    case CDCMessageKind::RENAME_FILE:
        setRenameFile() = other.getRenameFile();
        break;
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        setSoftUnlinkDirectory() = other.getSoftUnlinkDirectory();
        break;
    case CDCMessageKind::RENAME_DIRECTORY:
        setRenameDirectory() = other.getRenameDirectory();
        break;
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        setHardUnlinkDirectory() = other.getHardUnlinkDirectory();
        break;
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        setCrossShardHardUnlinkFile() = other.getCrossShardHardUnlinkFile();
        break;
    case CDCMessageKind::CDC_SNAPSHOT:
        setCdcSnapshot() = other.getCdcSnapshot();
        break;
    default:
        throw TERN_EXCEPTION("bad CDCMessageKind kind %s", other.kind());
    }
}

void CDCRespContainer::operator=(CDCRespContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = CDCMessageKind::EMPTY;
}

size_t CDCRespContainer::packedSize() const {
    switch (_kind) {
    case CDCMessageKind::ERROR:
        return sizeof(CDCMessageKind) + sizeof(TernError);
    case CDCMessageKind::MAKE_DIRECTORY:
        return sizeof(CDCMessageKind) + std::get<1>(_data).packedSize();
    case CDCMessageKind::RENAME_FILE:
        return sizeof(CDCMessageKind) + std::get<2>(_data).packedSize();
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        return sizeof(CDCMessageKind) + std::get<3>(_data).packedSize();
    case CDCMessageKind::RENAME_DIRECTORY:
        return sizeof(CDCMessageKind) + std::get<4>(_data).packedSize();
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        return sizeof(CDCMessageKind) + std::get<5>(_data).packedSize();
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        return sizeof(CDCMessageKind) + std::get<6>(_data).packedSize();
    case CDCMessageKind::CDC_SNAPSHOT:
        return sizeof(CDCMessageKind) + std::get<7>(_data).packedSize();
    default:
        throw TERN_EXCEPTION("bad CDCMessageKind kind %s", _kind);
    }
}

void CDCRespContainer::pack(BincodeBuf& buf) const {
    buf.packScalar<CDCMessageKind>(_kind);
    switch (_kind) {
    case CDCMessageKind::ERROR:
        buf.packScalar<TernError>(std::get<0>(_data));
        break;
    case CDCMessageKind::MAKE_DIRECTORY:
        std::get<1>(_data).pack(buf);
        break;
    case CDCMessageKind::RENAME_FILE:
        std::get<2>(_data).pack(buf);
        break;
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        std::get<3>(_data).pack(buf);
        break;
    case CDCMessageKind::RENAME_DIRECTORY:
        std::get<4>(_data).pack(buf);
        break;
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        std::get<5>(_data).pack(buf);
        break;
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        std::get<6>(_data).pack(buf);
        break;
    case CDCMessageKind::CDC_SNAPSHOT:
        std::get<7>(_data).pack(buf);
        break;
    default:
        throw TERN_EXCEPTION("bad CDCMessageKind kind %s", _kind);
    }
}

void CDCRespContainer::unpack(BincodeBuf& buf) {
    _kind = buf.unpackScalar<CDCMessageKind>();
    switch (_kind) {
    case CDCMessageKind::ERROR:
        _data.emplace<0>(buf.unpackScalar<TernError>());
        break;
    case CDCMessageKind::MAKE_DIRECTORY:
        _data.emplace<1>().unpack(buf);
        break;
    case CDCMessageKind::RENAME_FILE:
        _data.emplace<2>().unpack(buf);
        break;
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        _data.emplace<3>().unpack(buf);
        break;
    case CDCMessageKind::RENAME_DIRECTORY:
        _data.emplace<4>().unpack(buf);
        break;
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        _data.emplace<5>().unpack(buf);
        break;
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        _data.emplace<6>().unpack(buf);
        break;
    case CDCMessageKind::CDC_SNAPSHOT:
        _data.emplace<7>().unpack(buf);
        break;
    default:
        throw BINCODE_EXCEPTION("bad CDCMessageKind kind %s", _kind);
    }
}

bool CDCRespContainer::operator==(const CDCRespContainer& other) const {
    if (_kind != other.kind()) { return false; }
    if (_kind == CDCMessageKind::EMPTY) { return true; }
    switch (_kind) {
    case CDCMessageKind::ERROR:
        return getError() == other.getError();
    case CDCMessageKind::MAKE_DIRECTORY:
        return getMakeDirectory() == other.getMakeDirectory();
    case CDCMessageKind::RENAME_FILE:
        return getRenameFile() == other.getRenameFile();
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        return getSoftUnlinkDirectory() == other.getSoftUnlinkDirectory();
    case CDCMessageKind::RENAME_DIRECTORY:
        return getRenameDirectory() == other.getRenameDirectory();
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        return getHardUnlinkDirectory() == other.getHardUnlinkDirectory();
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        return getCrossShardHardUnlinkFile() == other.getCrossShardHardUnlinkFile();
    case CDCMessageKind::CDC_SNAPSHOT:
        return getCdcSnapshot() == other.getCdcSnapshot();
    default:
        throw BINCODE_EXCEPTION("bad CDCMessageKind kind %s", _kind);
    }
}

std::ostream& operator<<(std::ostream& out, const CDCRespContainer& x) {
    switch (x.kind()) {
    case CDCMessageKind::ERROR:
        out << x.getError();
        break;
    case CDCMessageKind::MAKE_DIRECTORY:
        out << x.getMakeDirectory();
        break;
    case CDCMessageKind::RENAME_FILE:
        out << x.getRenameFile();
        break;
    case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
        out << x.getSoftUnlinkDirectory();
        break;
    case CDCMessageKind::RENAME_DIRECTORY:
        out << x.getRenameDirectory();
        break;
    case CDCMessageKind::HARD_UNLINK_DIRECTORY:
        out << x.getHardUnlinkDirectory();
        break;
    case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
        out << x.getCrossShardHardUnlinkFile();
        break;
    case CDCMessageKind::CDC_SNAPSHOT:
        out << x.getCdcSnapshot();
        break;
    case CDCMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        throw TERN_EXCEPTION("bad CDCMessageKind kind %s", x.kind());
    }
    return out;
}

const LocalShardsReq& RegistryReqContainer::getLocalShards() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::LOCAL_SHARDS, "%s != %s", _kind, RegistryMessageKind::LOCAL_SHARDS);
    return std::get<0>(_data);
}
LocalShardsReq& RegistryReqContainer::setLocalShards() {
    _kind = RegistryMessageKind::LOCAL_SHARDS;
    auto& x = _data.emplace<0>();
    return x;
}
const LocalCdcReq& RegistryReqContainer::getLocalCdc() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::LOCAL_CDC, "%s != %s", _kind, RegistryMessageKind::LOCAL_CDC);
    return std::get<1>(_data);
}
LocalCdcReq& RegistryReqContainer::setLocalCdc() {
    _kind = RegistryMessageKind::LOCAL_CDC;
    auto& x = _data.emplace<1>();
    return x;
}
const InfoReq& RegistryReqContainer::getInfo() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::INFO, "%s != %s", _kind, RegistryMessageKind::INFO);
    return std::get<2>(_data);
}
InfoReq& RegistryReqContainer::setInfo() {
    _kind = RegistryMessageKind::INFO;
    auto& x = _data.emplace<2>();
    return x;
}
const RegistryReq& RegistryReqContainer::getRegistry() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::REGISTRY, "%s != %s", _kind, RegistryMessageKind::REGISTRY);
    return std::get<3>(_data);
}
RegistryReq& RegistryReqContainer::setRegistry() {
    _kind = RegistryMessageKind::REGISTRY;
    auto& x = _data.emplace<3>();
    return x;
}
const LocalChangedBlockServicesReq& RegistryReqContainer::getLocalChangedBlockServices() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES, "%s != %s", _kind, RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES);
    return std::get<4>(_data);
}
LocalChangedBlockServicesReq& RegistryReqContainer::setLocalChangedBlockServices() {
    _kind = RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES;
    auto& x = _data.emplace<4>();
    return x;
}
const CreateLocationReq& RegistryReqContainer::getCreateLocation() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CREATE_LOCATION, "%s != %s", _kind, RegistryMessageKind::CREATE_LOCATION);
    return std::get<5>(_data);
}
CreateLocationReq& RegistryReqContainer::setCreateLocation() {
    _kind = RegistryMessageKind::CREATE_LOCATION;
    auto& x = _data.emplace<5>();
    return x;
}
const RenameLocationReq& RegistryReqContainer::getRenameLocation() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::RENAME_LOCATION, "%s != %s", _kind, RegistryMessageKind::RENAME_LOCATION);
    return std::get<6>(_data);
}
RenameLocationReq& RegistryReqContainer::setRenameLocation() {
    _kind = RegistryMessageKind::RENAME_LOCATION;
    auto& x = _data.emplace<6>();
    return x;
}
const RegisterShardReq& RegistryReqContainer::getRegisterShard() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::REGISTER_SHARD, "%s != %s", _kind, RegistryMessageKind::REGISTER_SHARD);
    return std::get<7>(_data);
}
RegisterShardReq& RegistryReqContainer::setRegisterShard() {
    _kind = RegistryMessageKind::REGISTER_SHARD;
    auto& x = _data.emplace<7>();
    return x;
}
const LocationsReq& RegistryReqContainer::getLocations() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::LOCATIONS, "%s != %s", _kind, RegistryMessageKind::LOCATIONS);
    return std::get<8>(_data);
}
LocationsReq& RegistryReqContainer::setLocations() {
    _kind = RegistryMessageKind::LOCATIONS;
    auto& x = _data.emplace<8>();
    return x;
}
const RegisterCdcReq& RegistryReqContainer::getRegisterCdc() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::REGISTER_CDC, "%s != %s", _kind, RegistryMessageKind::REGISTER_CDC);
    return std::get<9>(_data);
}
RegisterCdcReq& RegistryReqContainer::setRegisterCdc() {
    _kind = RegistryMessageKind::REGISTER_CDC;
    auto& x = _data.emplace<9>();
    return x;
}
const SetBlockServiceFlagsReq& RegistryReqContainer::getSetBlockServiceFlags() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS, "%s != %s", _kind, RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS);
    return std::get<10>(_data);
}
SetBlockServiceFlagsReq& RegistryReqContainer::setSetBlockServiceFlags() {
    _kind = RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS;
    auto& x = _data.emplace<10>();
    return x;
}
const RegisterBlockServicesReq& RegistryReqContainer::getRegisterBlockServices() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::REGISTER_BLOCK_SERVICES, "%s != %s", _kind, RegistryMessageKind::REGISTER_BLOCK_SERVICES);
    return std::get<11>(_data);
}
RegisterBlockServicesReq& RegistryReqContainer::setRegisterBlockServices() {
    _kind = RegistryMessageKind::REGISTER_BLOCK_SERVICES;
    auto& x = _data.emplace<11>();
    return x;
}
const ChangedBlockServicesAtLocationReq& RegistryReqContainer::getChangedBlockServicesAtLocation() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION, "%s != %s", _kind, RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION);
    return std::get<12>(_data);
}
ChangedBlockServicesAtLocationReq& RegistryReqContainer::setChangedBlockServicesAtLocation() {
    _kind = RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION;
    auto& x = _data.emplace<12>();
    return x;
}
const ShardsAtLocationReq& RegistryReqContainer::getShardsAtLocation() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::SHARDS_AT_LOCATION, "%s != %s", _kind, RegistryMessageKind::SHARDS_AT_LOCATION);
    return std::get<13>(_data);
}
ShardsAtLocationReq& RegistryReqContainer::setShardsAtLocation() {
    _kind = RegistryMessageKind::SHARDS_AT_LOCATION;
    auto& x = _data.emplace<13>();
    return x;
}
const CdcAtLocationReq& RegistryReqContainer::getCdcAtLocation() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CDC_AT_LOCATION, "%s != %s", _kind, RegistryMessageKind::CDC_AT_LOCATION);
    return std::get<14>(_data);
}
CdcAtLocationReq& RegistryReqContainer::setCdcAtLocation() {
    _kind = RegistryMessageKind::CDC_AT_LOCATION;
    auto& x = _data.emplace<14>();
    return x;
}
const RegisterRegistryReq& RegistryReqContainer::getRegisterRegistry() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::REGISTER_REGISTRY, "%s != %s", _kind, RegistryMessageKind::REGISTER_REGISTRY);
    return std::get<15>(_data);
}
RegisterRegistryReq& RegistryReqContainer::setRegisterRegistry() {
    _kind = RegistryMessageKind::REGISTER_REGISTRY;
    auto& x = _data.emplace<15>();
    return x;
}
const AllRegistryReplicasReq& RegistryReqContainer::getAllRegistryReplicas() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ALL_REGISTRY_REPLICAS, "%s != %s", _kind, RegistryMessageKind::ALL_REGISTRY_REPLICAS);
    return std::get<16>(_data);
}
AllRegistryReplicasReq& RegistryReqContainer::setAllRegistryReplicas() {
    _kind = RegistryMessageKind::ALL_REGISTRY_REPLICAS;
    auto& x = _data.emplace<16>();
    return x;
}
const ShardBlockServicesDEPRECATEDReq& RegistryReqContainer::getShardBlockServicesDEPRECATED() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED, "%s != %s", _kind, RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED);
    return std::get<17>(_data);
}
ShardBlockServicesDEPRECATEDReq& RegistryReqContainer::setShardBlockServicesDEPRECATED() {
    _kind = RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED;
    auto& x = _data.emplace<17>();
    return x;
}
const CdcReplicasDEPRECATEDReq& RegistryReqContainer::getCdcReplicasDEPRECATED() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED, "%s != %s", _kind, RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED);
    return std::get<18>(_data);
}
CdcReplicasDEPRECATEDReq& RegistryReqContainer::setCdcReplicasDEPRECATED() {
    _kind = RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED;
    auto& x = _data.emplace<18>();
    return x;
}
const AllShardsReq& RegistryReqContainer::getAllShards() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ALL_SHARDS, "%s != %s", _kind, RegistryMessageKind::ALL_SHARDS);
    return std::get<19>(_data);
}
AllShardsReq& RegistryReqContainer::setAllShards() {
    _kind = RegistryMessageKind::ALL_SHARDS;
    auto& x = _data.emplace<19>();
    return x;
}
const DecommissionBlockServiceReq& RegistryReqContainer::getDecommissionBlockService() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE, "%s != %s", _kind, RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE);
    return std::get<20>(_data);
}
DecommissionBlockServiceReq& RegistryReqContainer::setDecommissionBlockService() {
    _kind = RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE;
    auto& x = _data.emplace<20>();
    return x;
}
const MoveShardLeaderReq& RegistryReqContainer::getMoveShardLeader() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::MOVE_SHARD_LEADER, "%s != %s", _kind, RegistryMessageKind::MOVE_SHARD_LEADER);
    return std::get<21>(_data);
}
MoveShardLeaderReq& RegistryReqContainer::setMoveShardLeader() {
    _kind = RegistryMessageKind::MOVE_SHARD_LEADER;
    auto& x = _data.emplace<21>();
    return x;
}
const ClearShardInfoReq& RegistryReqContainer::getClearShardInfo() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CLEAR_SHARD_INFO, "%s != %s", _kind, RegistryMessageKind::CLEAR_SHARD_INFO);
    return std::get<22>(_data);
}
ClearShardInfoReq& RegistryReqContainer::setClearShardInfo() {
    _kind = RegistryMessageKind::CLEAR_SHARD_INFO;
    auto& x = _data.emplace<22>();
    return x;
}
const ShardBlockServicesReq& RegistryReqContainer::getShardBlockServices() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::SHARD_BLOCK_SERVICES, "%s != %s", _kind, RegistryMessageKind::SHARD_BLOCK_SERVICES);
    return std::get<23>(_data);
}
ShardBlockServicesReq& RegistryReqContainer::setShardBlockServices() {
    _kind = RegistryMessageKind::SHARD_BLOCK_SERVICES;
    auto& x = _data.emplace<23>();
    return x;
}
const AllCdcReq& RegistryReqContainer::getAllCdc() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ALL_CDC, "%s != %s", _kind, RegistryMessageKind::ALL_CDC);
    return std::get<24>(_data);
}
AllCdcReq& RegistryReqContainer::setAllCdc() {
    _kind = RegistryMessageKind::ALL_CDC;
    auto& x = _data.emplace<24>();
    return x;
}
const EraseDecommissionedBlockReq& RegistryReqContainer::getEraseDecommissionedBlock() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK, "%s != %s", _kind, RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK);
    return std::get<25>(_data);
}
EraseDecommissionedBlockReq& RegistryReqContainer::setEraseDecommissionedBlock() {
    _kind = RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK;
    auto& x = _data.emplace<25>();
    return x;
}
const AllBlockServicesDeprecatedReq& RegistryReqContainer::getAllBlockServicesDeprecated() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED, "%s != %s", _kind, RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED);
    return std::get<26>(_data);
}
AllBlockServicesDeprecatedReq& RegistryReqContainer::setAllBlockServicesDeprecated() {
    _kind = RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED;
    auto& x = _data.emplace<26>();
    return x;
}
const MoveCdcLeaderReq& RegistryReqContainer::getMoveCdcLeader() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::MOVE_CDC_LEADER, "%s != %s", _kind, RegistryMessageKind::MOVE_CDC_LEADER);
    return std::get<27>(_data);
}
MoveCdcLeaderReq& RegistryReqContainer::setMoveCdcLeader() {
    _kind = RegistryMessageKind::MOVE_CDC_LEADER;
    auto& x = _data.emplace<27>();
    return x;
}
const ClearCdcInfoReq& RegistryReqContainer::getClearCdcInfo() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CLEAR_CDC_INFO, "%s != %s", _kind, RegistryMessageKind::CLEAR_CDC_INFO);
    return std::get<28>(_data);
}
ClearCdcInfoReq& RegistryReqContainer::setClearCdcInfo() {
    _kind = RegistryMessageKind::CLEAR_CDC_INFO;
    auto& x = _data.emplace<28>();
    return x;
}
const UpdateBlockServicePathReq& RegistryReqContainer::getUpdateBlockServicePath() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH, "%s != %s", _kind, RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH);
    return std::get<29>(_data);
}
UpdateBlockServicePathReq& RegistryReqContainer::setUpdateBlockServicePath() {
    _kind = RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH;
    auto& x = _data.emplace<29>();
    return x;
}
RegistryReqContainer::RegistryReqContainer() {
    clear();
}

RegistryReqContainer::RegistryReqContainer(const RegistryReqContainer& other) {
    *this = other;
}

RegistryReqContainer::RegistryReqContainer(RegistryReqContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = RegistryMessageKind::EMPTY;
}

void RegistryReqContainer::operator=(const RegistryReqContainer& other) {
    if (other.kind() == RegistryMessageKind::EMPTY) { clear(); return; }
    switch (other.kind()) {
    case RegistryMessageKind::LOCAL_SHARDS:
        setLocalShards() = other.getLocalShards();
        break;
    case RegistryMessageKind::LOCAL_CDC:
        setLocalCdc() = other.getLocalCdc();
        break;
    case RegistryMessageKind::INFO:
        setInfo() = other.getInfo();
        break;
    case RegistryMessageKind::REGISTRY:
        setRegistry() = other.getRegistry();
        break;
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        setLocalChangedBlockServices() = other.getLocalChangedBlockServices();
        break;
    case RegistryMessageKind::CREATE_LOCATION:
        setCreateLocation() = other.getCreateLocation();
        break;
    case RegistryMessageKind::RENAME_LOCATION:
        setRenameLocation() = other.getRenameLocation();
        break;
    case RegistryMessageKind::REGISTER_SHARD:
        setRegisterShard() = other.getRegisterShard();
        break;
    case RegistryMessageKind::LOCATIONS:
        setLocations() = other.getLocations();
        break;
    case RegistryMessageKind::REGISTER_CDC:
        setRegisterCdc() = other.getRegisterCdc();
        break;
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        setSetBlockServiceFlags() = other.getSetBlockServiceFlags();
        break;
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        setRegisterBlockServices() = other.getRegisterBlockServices();
        break;
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        setChangedBlockServicesAtLocation() = other.getChangedBlockServicesAtLocation();
        break;
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        setShardsAtLocation() = other.getShardsAtLocation();
        break;
    case RegistryMessageKind::CDC_AT_LOCATION:
        setCdcAtLocation() = other.getCdcAtLocation();
        break;
    case RegistryMessageKind::REGISTER_REGISTRY:
        setRegisterRegistry() = other.getRegisterRegistry();
        break;
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        setAllRegistryReplicas() = other.getAllRegistryReplicas();
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        setShardBlockServicesDEPRECATED() = other.getShardBlockServicesDEPRECATED();
        break;
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        setCdcReplicasDEPRECATED() = other.getCdcReplicasDEPRECATED();
        break;
    case RegistryMessageKind::ALL_SHARDS:
        setAllShards() = other.getAllShards();
        break;
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        setDecommissionBlockService() = other.getDecommissionBlockService();
        break;
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        setMoveShardLeader() = other.getMoveShardLeader();
        break;
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        setClearShardInfo() = other.getClearShardInfo();
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        setShardBlockServices() = other.getShardBlockServices();
        break;
    case RegistryMessageKind::ALL_CDC:
        setAllCdc() = other.getAllCdc();
        break;
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        setEraseDecommissionedBlock() = other.getEraseDecommissionedBlock();
        break;
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        setAllBlockServicesDeprecated() = other.getAllBlockServicesDeprecated();
        break;
    case RegistryMessageKind::MOVE_CDC_LEADER:
        setMoveCdcLeader() = other.getMoveCdcLeader();
        break;
    case RegistryMessageKind::CLEAR_CDC_INFO:
        setClearCdcInfo() = other.getClearCdcInfo();
        break;
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        setUpdateBlockServicePath() = other.getUpdateBlockServicePath();
        break;
    default:
        throw TERN_EXCEPTION("bad RegistryMessageKind kind %s", other.kind());
    }
}

void RegistryReqContainer::operator=(RegistryReqContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = RegistryMessageKind::EMPTY;
}

size_t RegistryReqContainer::packedSize() const {
    switch (_kind) {
    case RegistryMessageKind::LOCAL_SHARDS:
        return sizeof(RegistryMessageKind) + std::get<0>(_data).packedSize();
    case RegistryMessageKind::LOCAL_CDC:
        return sizeof(RegistryMessageKind) + std::get<1>(_data).packedSize();
    case RegistryMessageKind::INFO:
        return sizeof(RegistryMessageKind) + std::get<2>(_data).packedSize();
    case RegistryMessageKind::REGISTRY:
        return sizeof(RegistryMessageKind) + std::get<3>(_data).packedSize();
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        return sizeof(RegistryMessageKind) + std::get<4>(_data).packedSize();
    case RegistryMessageKind::CREATE_LOCATION:
        return sizeof(RegistryMessageKind) + std::get<5>(_data).packedSize();
    case RegistryMessageKind::RENAME_LOCATION:
        return sizeof(RegistryMessageKind) + std::get<6>(_data).packedSize();
    case RegistryMessageKind::REGISTER_SHARD:
        return sizeof(RegistryMessageKind) + std::get<7>(_data).packedSize();
    case RegistryMessageKind::LOCATIONS:
        return sizeof(RegistryMessageKind) + std::get<8>(_data).packedSize();
    case RegistryMessageKind::REGISTER_CDC:
        return sizeof(RegistryMessageKind) + std::get<9>(_data).packedSize();
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        return sizeof(RegistryMessageKind) + std::get<10>(_data).packedSize();
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        return sizeof(RegistryMessageKind) + std::get<11>(_data).packedSize();
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        return sizeof(RegistryMessageKind) + std::get<12>(_data).packedSize();
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        return sizeof(RegistryMessageKind) + std::get<13>(_data).packedSize();
    case RegistryMessageKind::CDC_AT_LOCATION:
        return sizeof(RegistryMessageKind) + std::get<14>(_data).packedSize();
    case RegistryMessageKind::REGISTER_REGISTRY:
        return sizeof(RegistryMessageKind) + std::get<15>(_data).packedSize();
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        return sizeof(RegistryMessageKind) + std::get<16>(_data).packedSize();
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        return sizeof(RegistryMessageKind) + std::get<17>(_data).packedSize();
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        return sizeof(RegistryMessageKind) + std::get<18>(_data).packedSize();
    case RegistryMessageKind::ALL_SHARDS:
        return sizeof(RegistryMessageKind) + std::get<19>(_data).packedSize();
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        return sizeof(RegistryMessageKind) + std::get<20>(_data).packedSize();
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        return sizeof(RegistryMessageKind) + std::get<21>(_data).packedSize();
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        return sizeof(RegistryMessageKind) + std::get<22>(_data).packedSize();
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        return sizeof(RegistryMessageKind) + std::get<23>(_data).packedSize();
    case RegistryMessageKind::ALL_CDC:
        return sizeof(RegistryMessageKind) + std::get<24>(_data).packedSize();
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        return sizeof(RegistryMessageKind) + std::get<25>(_data).packedSize();
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        return sizeof(RegistryMessageKind) + std::get<26>(_data).packedSize();
    case RegistryMessageKind::MOVE_CDC_LEADER:
        return sizeof(RegistryMessageKind) + std::get<27>(_data).packedSize();
    case RegistryMessageKind::CLEAR_CDC_INFO:
        return sizeof(RegistryMessageKind) + std::get<28>(_data).packedSize();
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        return sizeof(RegistryMessageKind) + std::get<29>(_data).packedSize();
    default:
        throw TERN_EXCEPTION("bad RegistryMessageKind kind %s", _kind);
    }
}

void RegistryReqContainer::pack(BincodeBuf& buf) const {
    buf.packScalar<RegistryMessageKind>(_kind);
    switch (_kind) {
    case RegistryMessageKind::LOCAL_SHARDS:
        std::get<0>(_data).pack(buf);
        break;
    case RegistryMessageKind::LOCAL_CDC:
        std::get<1>(_data).pack(buf);
        break;
    case RegistryMessageKind::INFO:
        std::get<2>(_data).pack(buf);
        break;
    case RegistryMessageKind::REGISTRY:
        std::get<3>(_data).pack(buf);
        break;
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        std::get<4>(_data).pack(buf);
        break;
    case RegistryMessageKind::CREATE_LOCATION:
        std::get<5>(_data).pack(buf);
        break;
    case RegistryMessageKind::RENAME_LOCATION:
        std::get<6>(_data).pack(buf);
        break;
    case RegistryMessageKind::REGISTER_SHARD:
        std::get<7>(_data).pack(buf);
        break;
    case RegistryMessageKind::LOCATIONS:
        std::get<8>(_data).pack(buf);
        break;
    case RegistryMessageKind::REGISTER_CDC:
        std::get<9>(_data).pack(buf);
        break;
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        std::get<10>(_data).pack(buf);
        break;
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        std::get<11>(_data).pack(buf);
        break;
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        std::get<12>(_data).pack(buf);
        break;
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        std::get<13>(_data).pack(buf);
        break;
    case RegistryMessageKind::CDC_AT_LOCATION:
        std::get<14>(_data).pack(buf);
        break;
    case RegistryMessageKind::REGISTER_REGISTRY:
        std::get<15>(_data).pack(buf);
        break;
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        std::get<16>(_data).pack(buf);
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        std::get<17>(_data).pack(buf);
        break;
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        std::get<18>(_data).pack(buf);
        break;
    case RegistryMessageKind::ALL_SHARDS:
        std::get<19>(_data).pack(buf);
        break;
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        std::get<20>(_data).pack(buf);
        break;
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        std::get<21>(_data).pack(buf);
        break;
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        std::get<22>(_data).pack(buf);
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        std::get<23>(_data).pack(buf);
        break;
    case RegistryMessageKind::ALL_CDC:
        std::get<24>(_data).pack(buf);
        break;
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        std::get<25>(_data).pack(buf);
        break;
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        std::get<26>(_data).pack(buf);
        break;
    case RegistryMessageKind::MOVE_CDC_LEADER:
        std::get<27>(_data).pack(buf);
        break;
    case RegistryMessageKind::CLEAR_CDC_INFO:
        std::get<28>(_data).pack(buf);
        break;
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        std::get<29>(_data).pack(buf);
        break;
    default:
        throw TERN_EXCEPTION("bad RegistryMessageKind kind %s", _kind);
    }
}

void RegistryReqContainer::unpack(BincodeBuf& buf) {
    _kind = buf.unpackScalar<RegistryMessageKind>();
    switch (_kind) {
    case RegistryMessageKind::LOCAL_SHARDS:
        _data.emplace<0>().unpack(buf);
        break;
    case RegistryMessageKind::LOCAL_CDC:
        _data.emplace<1>().unpack(buf);
        break;
    case RegistryMessageKind::INFO:
        _data.emplace<2>().unpack(buf);
        break;
    case RegistryMessageKind::REGISTRY:
        _data.emplace<3>().unpack(buf);
        break;
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        _data.emplace<4>().unpack(buf);
        break;
    case RegistryMessageKind::CREATE_LOCATION:
        _data.emplace<5>().unpack(buf);
        break;
    case RegistryMessageKind::RENAME_LOCATION:
        _data.emplace<6>().unpack(buf);
        break;
    case RegistryMessageKind::REGISTER_SHARD:
        _data.emplace<7>().unpack(buf);
        break;
    case RegistryMessageKind::LOCATIONS:
        _data.emplace<8>().unpack(buf);
        break;
    case RegistryMessageKind::REGISTER_CDC:
        _data.emplace<9>().unpack(buf);
        break;
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        _data.emplace<10>().unpack(buf);
        break;
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        _data.emplace<11>().unpack(buf);
        break;
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        _data.emplace<12>().unpack(buf);
        break;
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        _data.emplace<13>().unpack(buf);
        break;
    case RegistryMessageKind::CDC_AT_LOCATION:
        _data.emplace<14>().unpack(buf);
        break;
    case RegistryMessageKind::REGISTER_REGISTRY:
        _data.emplace<15>().unpack(buf);
        break;
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        _data.emplace<16>().unpack(buf);
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        _data.emplace<17>().unpack(buf);
        break;
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        _data.emplace<18>().unpack(buf);
        break;
    case RegistryMessageKind::ALL_SHARDS:
        _data.emplace<19>().unpack(buf);
        break;
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        _data.emplace<20>().unpack(buf);
        break;
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        _data.emplace<21>().unpack(buf);
        break;
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        _data.emplace<22>().unpack(buf);
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        _data.emplace<23>().unpack(buf);
        break;
    case RegistryMessageKind::ALL_CDC:
        _data.emplace<24>().unpack(buf);
        break;
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        _data.emplace<25>().unpack(buf);
        break;
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        _data.emplace<26>().unpack(buf);
        break;
    case RegistryMessageKind::MOVE_CDC_LEADER:
        _data.emplace<27>().unpack(buf);
        break;
    case RegistryMessageKind::CLEAR_CDC_INFO:
        _data.emplace<28>().unpack(buf);
        break;
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        _data.emplace<29>().unpack(buf);
        break;
    default:
        throw BINCODE_EXCEPTION("bad RegistryMessageKind kind %s", _kind);
    }
}

bool RegistryReqContainer::operator==(const RegistryReqContainer& other) const {
    if (_kind != other.kind()) { return false; }
    if (_kind == RegistryMessageKind::EMPTY) { return true; }
    switch (_kind) {
    case RegistryMessageKind::LOCAL_SHARDS:
        return getLocalShards() == other.getLocalShards();
    case RegistryMessageKind::LOCAL_CDC:
        return getLocalCdc() == other.getLocalCdc();
    case RegistryMessageKind::INFO:
        return getInfo() == other.getInfo();
    case RegistryMessageKind::REGISTRY:
        return getRegistry() == other.getRegistry();
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        return getLocalChangedBlockServices() == other.getLocalChangedBlockServices();
    case RegistryMessageKind::CREATE_LOCATION:
        return getCreateLocation() == other.getCreateLocation();
    case RegistryMessageKind::RENAME_LOCATION:
        return getRenameLocation() == other.getRenameLocation();
    case RegistryMessageKind::REGISTER_SHARD:
        return getRegisterShard() == other.getRegisterShard();
    case RegistryMessageKind::LOCATIONS:
        return getLocations() == other.getLocations();
    case RegistryMessageKind::REGISTER_CDC:
        return getRegisterCdc() == other.getRegisterCdc();
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        return getSetBlockServiceFlags() == other.getSetBlockServiceFlags();
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        return getRegisterBlockServices() == other.getRegisterBlockServices();
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        return getChangedBlockServicesAtLocation() == other.getChangedBlockServicesAtLocation();
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        return getShardsAtLocation() == other.getShardsAtLocation();
    case RegistryMessageKind::CDC_AT_LOCATION:
        return getCdcAtLocation() == other.getCdcAtLocation();
    case RegistryMessageKind::REGISTER_REGISTRY:
        return getRegisterRegistry() == other.getRegisterRegistry();
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        return getAllRegistryReplicas() == other.getAllRegistryReplicas();
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        return getShardBlockServicesDEPRECATED() == other.getShardBlockServicesDEPRECATED();
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        return getCdcReplicasDEPRECATED() == other.getCdcReplicasDEPRECATED();
    case RegistryMessageKind::ALL_SHARDS:
        return getAllShards() == other.getAllShards();
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        return getDecommissionBlockService() == other.getDecommissionBlockService();
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        return getMoveShardLeader() == other.getMoveShardLeader();
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        return getClearShardInfo() == other.getClearShardInfo();
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        return getShardBlockServices() == other.getShardBlockServices();
    case RegistryMessageKind::ALL_CDC:
        return getAllCdc() == other.getAllCdc();
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        return getEraseDecommissionedBlock() == other.getEraseDecommissionedBlock();
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        return getAllBlockServicesDeprecated() == other.getAllBlockServicesDeprecated();
    case RegistryMessageKind::MOVE_CDC_LEADER:
        return getMoveCdcLeader() == other.getMoveCdcLeader();
    case RegistryMessageKind::CLEAR_CDC_INFO:
        return getClearCdcInfo() == other.getClearCdcInfo();
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        return getUpdateBlockServicePath() == other.getUpdateBlockServicePath();
    default:
        throw BINCODE_EXCEPTION("bad RegistryMessageKind kind %s", _kind);
    }
}

std::ostream& operator<<(std::ostream& out, const RegistryReqContainer& x) {
    switch (x.kind()) {
    case RegistryMessageKind::LOCAL_SHARDS:
        out << x.getLocalShards();
        break;
    case RegistryMessageKind::LOCAL_CDC:
        out << x.getLocalCdc();
        break;
    case RegistryMessageKind::INFO:
        out << x.getInfo();
        break;
    case RegistryMessageKind::REGISTRY:
        out << x.getRegistry();
        break;
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        out << x.getLocalChangedBlockServices();
        break;
    case RegistryMessageKind::CREATE_LOCATION:
        out << x.getCreateLocation();
        break;
    case RegistryMessageKind::RENAME_LOCATION:
        out << x.getRenameLocation();
        break;
    case RegistryMessageKind::REGISTER_SHARD:
        out << x.getRegisterShard();
        break;
    case RegistryMessageKind::LOCATIONS:
        out << x.getLocations();
        break;
    case RegistryMessageKind::REGISTER_CDC:
        out << x.getRegisterCdc();
        break;
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        out << x.getSetBlockServiceFlags();
        break;
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        out << x.getRegisterBlockServices();
        break;
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        out << x.getChangedBlockServicesAtLocation();
        break;
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        out << x.getShardsAtLocation();
        break;
    case RegistryMessageKind::CDC_AT_LOCATION:
        out << x.getCdcAtLocation();
        break;
    case RegistryMessageKind::REGISTER_REGISTRY:
        out << x.getRegisterRegistry();
        break;
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        out << x.getAllRegistryReplicas();
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        out << x.getShardBlockServicesDEPRECATED();
        break;
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        out << x.getCdcReplicasDEPRECATED();
        break;
    case RegistryMessageKind::ALL_SHARDS:
        out << x.getAllShards();
        break;
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        out << x.getDecommissionBlockService();
        break;
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        out << x.getMoveShardLeader();
        break;
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        out << x.getClearShardInfo();
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        out << x.getShardBlockServices();
        break;
    case RegistryMessageKind::ALL_CDC:
        out << x.getAllCdc();
        break;
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        out << x.getEraseDecommissionedBlock();
        break;
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        out << x.getAllBlockServicesDeprecated();
        break;
    case RegistryMessageKind::MOVE_CDC_LEADER:
        out << x.getMoveCdcLeader();
        break;
    case RegistryMessageKind::CLEAR_CDC_INFO:
        out << x.getClearCdcInfo();
        break;
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        out << x.getUpdateBlockServicePath();
        break;
    case RegistryMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        throw TERN_EXCEPTION("bad RegistryMessageKind kind %s", x.kind());
    }
    return out;
}

const TernError& RegistryRespContainer::getError() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ERROR, "%s != %s", _kind, RegistryMessageKind::ERROR);
    return std::get<0>(_data);
}
TernError& RegistryRespContainer::setError() {
    _kind = RegistryMessageKind::ERROR;
    auto& x = _data.emplace<0>();
    return x;
}
const LocalShardsResp& RegistryRespContainer::getLocalShards() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::LOCAL_SHARDS, "%s != %s", _kind, RegistryMessageKind::LOCAL_SHARDS);
    return std::get<1>(_data);
}
LocalShardsResp& RegistryRespContainer::setLocalShards() {
    _kind = RegistryMessageKind::LOCAL_SHARDS;
    auto& x = _data.emplace<1>();
    return x;
}
const LocalCdcResp& RegistryRespContainer::getLocalCdc() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::LOCAL_CDC, "%s != %s", _kind, RegistryMessageKind::LOCAL_CDC);
    return std::get<2>(_data);
}
LocalCdcResp& RegistryRespContainer::setLocalCdc() {
    _kind = RegistryMessageKind::LOCAL_CDC;
    auto& x = _data.emplace<2>();
    return x;
}
const InfoResp& RegistryRespContainer::getInfo() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::INFO, "%s != %s", _kind, RegistryMessageKind::INFO);
    return std::get<3>(_data);
}
InfoResp& RegistryRespContainer::setInfo() {
    _kind = RegistryMessageKind::INFO;
    auto& x = _data.emplace<3>();
    return x;
}
const RegistryResp& RegistryRespContainer::getRegistry() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::REGISTRY, "%s != %s", _kind, RegistryMessageKind::REGISTRY);
    return std::get<4>(_data);
}
RegistryResp& RegistryRespContainer::setRegistry() {
    _kind = RegistryMessageKind::REGISTRY;
    auto& x = _data.emplace<4>();
    return x;
}
const LocalChangedBlockServicesResp& RegistryRespContainer::getLocalChangedBlockServices() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES, "%s != %s", _kind, RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES);
    return std::get<5>(_data);
}
LocalChangedBlockServicesResp& RegistryRespContainer::setLocalChangedBlockServices() {
    _kind = RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES;
    auto& x = _data.emplace<5>();
    return x;
}
const CreateLocationResp& RegistryRespContainer::getCreateLocation() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CREATE_LOCATION, "%s != %s", _kind, RegistryMessageKind::CREATE_LOCATION);
    return std::get<6>(_data);
}
CreateLocationResp& RegistryRespContainer::setCreateLocation() {
    _kind = RegistryMessageKind::CREATE_LOCATION;
    auto& x = _data.emplace<6>();
    return x;
}
const RenameLocationResp& RegistryRespContainer::getRenameLocation() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::RENAME_LOCATION, "%s != %s", _kind, RegistryMessageKind::RENAME_LOCATION);
    return std::get<7>(_data);
}
RenameLocationResp& RegistryRespContainer::setRenameLocation() {
    _kind = RegistryMessageKind::RENAME_LOCATION;
    auto& x = _data.emplace<7>();
    return x;
}
const RegisterShardResp& RegistryRespContainer::getRegisterShard() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::REGISTER_SHARD, "%s != %s", _kind, RegistryMessageKind::REGISTER_SHARD);
    return std::get<8>(_data);
}
RegisterShardResp& RegistryRespContainer::setRegisterShard() {
    _kind = RegistryMessageKind::REGISTER_SHARD;
    auto& x = _data.emplace<8>();
    return x;
}
const LocationsResp& RegistryRespContainer::getLocations() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::LOCATIONS, "%s != %s", _kind, RegistryMessageKind::LOCATIONS);
    return std::get<9>(_data);
}
LocationsResp& RegistryRespContainer::setLocations() {
    _kind = RegistryMessageKind::LOCATIONS;
    auto& x = _data.emplace<9>();
    return x;
}
const RegisterCdcResp& RegistryRespContainer::getRegisterCdc() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::REGISTER_CDC, "%s != %s", _kind, RegistryMessageKind::REGISTER_CDC);
    return std::get<10>(_data);
}
RegisterCdcResp& RegistryRespContainer::setRegisterCdc() {
    _kind = RegistryMessageKind::REGISTER_CDC;
    auto& x = _data.emplace<10>();
    return x;
}
const SetBlockServiceFlagsResp& RegistryRespContainer::getSetBlockServiceFlags() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS, "%s != %s", _kind, RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS);
    return std::get<11>(_data);
}
SetBlockServiceFlagsResp& RegistryRespContainer::setSetBlockServiceFlags() {
    _kind = RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS;
    auto& x = _data.emplace<11>();
    return x;
}
const RegisterBlockServicesResp& RegistryRespContainer::getRegisterBlockServices() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::REGISTER_BLOCK_SERVICES, "%s != %s", _kind, RegistryMessageKind::REGISTER_BLOCK_SERVICES);
    return std::get<12>(_data);
}
RegisterBlockServicesResp& RegistryRespContainer::setRegisterBlockServices() {
    _kind = RegistryMessageKind::REGISTER_BLOCK_SERVICES;
    auto& x = _data.emplace<12>();
    return x;
}
const ChangedBlockServicesAtLocationResp& RegistryRespContainer::getChangedBlockServicesAtLocation() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION, "%s != %s", _kind, RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION);
    return std::get<13>(_data);
}
ChangedBlockServicesAtLocationResp& RegistryRespContainer::setChangedBlockServicesAtLocation() {
    _kind = RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION;
    auto& x = _data.emplace<13>();
    return x;
}
const ShardsAtLocationResp& RegistryRespContainer::getShardsAtLocation() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::SHARDS_AT_LOCATION, "%s != %s", _kind, RegistryMessageKind::SHARDS_AT_LOCATION);
    return std::get<14>(_data);
}
ShardsAtLocationResp& RegistryRespContainer::setShardsAtLocation() {
    _kind = RegistryMessageKind::SHARDS_AT_LOCATION;
    auto& x = _data.emplace<14>();
    return x;
}
const CdcAtLocationResp& RegistryRespContainer::getCdcAtLocation() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CDC_AT_LOCATION, "%s != %s", _kind, RegistryMessageKind::CDC_AT_LOCATION);
    return std::get<15>(_data);
}
CdcAtLocationResp& RegistryRespContainer::setCdcAtLocation() {
    _kind = RegistryMessageKind::CDC_AT_LOCATION;
    auto& x = _data.emplace<15>();
    return x;
}
const RegisterRegistryResp& RegistryRespContainer::getRegisterRegistry() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::REGISTER_REGISTRY, "%s != %s", _kind, RegistryMessageKind::REGISTER_REGISTRY);
    return std::get<16>(_data);
}
RegisterRegistryResp& RegistryRespContainer::setRegisterRegistry() {
    _kind = RegistryMessageKind::REGISTER_REGISTRY;
    auto& x = _data.emplace<16>();
    return x;
}
const AllRegistryReplicasResp& RegistryRespContainer::getAllRegistryReplicas() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ALL_REGISTRY_REPLICAS, "%s != %s", _kind, RegistryMessageKind::ALL_REGISTRY_REPLICAS);
    return std::get<17>(_data);
}
AllRegistryReplicasResp& RegistryRespContainer::setAllRegistryReplicas() {
    _kind = RegistryMessageKind::ALL_REGISTRY_REPLICAS;
    auto& x = _data.emplace<17>();
    return x;
}
const ShardBlockServicesDEPRECATEDResp& RegistryRespContainer::getShardBlockServicesDEPRECATED() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED, "%s != %s", _kind, RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED);
    return std::get<18>(_data);
}
ShardBlockServicesDEPRECATEDResp& RegistryRespContainer::setShardBlockServicesDEPRECATED() {
    _kind = RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED;
    auto& x = _data.emplace<18>();
    return x;
}
const CdcReplicasDEPRECATEDResp& RegistryRespContainer::getCdcReplicasDEPRECATED() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED, "%s != %s", _kind, RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED);
    return std::get<19>(_data);
}
CdcReplicasDEPRECATEDResp& RegistryRespContainer::setCdcReplicasDEPRECATED() {
    _kind = RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED;
    auto& x = _data.emplace<19>();
    return x;
}
const AllShardsResp& RegistryRespContainer::getAllShards() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ALL_SHARDS, "%s != %s", _kind, RegistryMessageKind::ALL_SHARDS);
    return std::get<20>(_data);
}
AllShardsResp& RegistryRespContainer::setAllShards() {
    _kind = RegistryMessageKind::ALL_SHARDS;
    auto& x = _data.emplace<20>();
    return x;
}
const DecommissionBlockServiceResp& RegistryRespContainer::getDecommissionBlockService() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE, "%s != %s", _kind, RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE);
    return std::get<21>(_data);
}
DecommissionBlockServiceResp& RegistryRespContainer::setDecommissionBlockService() {
    _kind = RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE;
    auto& x = _data.emplace<21>();
    return x;
}
const MoveShardLeaderResp& RegistryRespContainer::getMoveShardLeader() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::MOVE_SHARD_LEADER, "%s != %s", _kind, RegistryMessageKind::MOVE_SHARD_LEADER);
    return std::get<22>(_data);
}
MoveShardLeaderResp& RegistryRespContainer::setMoveShardLeader() {
    _kind = RegistryMessageKind::MOVE_SHARD_LEADER;
    auto& x = _data.emplace<22>();
    return x;
}
const ClearShardInfoResp& RegistryRespContainer::getClearShardInfo() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CLEAR_SHARD_INFO, "%s != %s", _kind, RegistryMessageKind::CLEAR_SHARD_INFO);
    return std::get<23>(_data);
}
ClearShardInfoResp& RegistryRespContainer::setClearShardInfo() {
    _kind = RegistryMessageKind::CLEAR_SHARD_INFO;
    auto& x = _data.emplace<23>();
    return x;
}
const ShardBlockServicesResp& RegistryRespContainer::getShardBlockServices() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::SHARD_BLOCK_SERVICES, "%s != %s", _kind, RegistryMessageKind::SHARD_BLOCK_SERVICES);
    return std::get<24>(_data);
}
ShardBlockServicesResp& RegistryRespContainer::setShardBlockServices() {
    _kind = RegistryMessageKind::SHARD_BLOCK_SERVICES;
    auto& x = _data.emplace<24>();
    return x;
}
const AllCdcResp& RegistryRespContainer::getAllCdc() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ALL_CDC, "%s != %s", _kind, RegistryMessageKind::ALL_CDC);
    return std::get<25>(_data);
}
AllCdcResp& RegistryRespContainer::setAllCdc() {
    _kind = RegistryMessageKind::ALL_CDC;
    auto& x = _data.emplace<25>();
    return x;
}
const EraseDecommissionedBlockResp& RegistryRespContainer::getEraseDecommissionedBlock() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK, "%s != %s", _kind, RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK);
    return std::get<26>(_data);
}
EraseDecommissionedBlockResp& RegistryRespContainer::setEraseDecommissionedBlock() {
    _kind = RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK;
    auto& x = _data.emplace<26>();
    return x;
}
const AllBlockServicesDeprecatedResp& RegistryRespContainer::getAllBlockServicesDeprecated() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED, "%s != %s", _kind, RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED);
    return std::get<27>(_data);
}
AllBlockServicesDeprecatedResp& RegistryRespContainer::setAllBlockServicesDeprecated() {
    _kind = RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED;
    auto& x = _data.emplace<27>();
    return x;
}
const MoveCdcLeaderResp& RegistryRespContainer::getMoveCdcLeader() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::MOVE_CDC_LEADER, "%s != %s", _kind, RegistryMessageKind::MOVE_CDC_LEADER);
    return std::get<28>(_data);
}
MoveCdcLeaderResp& RegistryRespContainer::setMoveCdcLeader() {
    _kind = RegistryMessageKind::MOVE_CDC_LEADER;
    auto& x = _data.emplace<28>();
    return x;
}
const ClearCdcInfoResp& RegistryRespContainer::getClearCdcInfo() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::CLEAR_CDC_INFO, "%s != %s", _kind, RegistryMessageKind::CLEAR_CDC_INFO);
    return std::get<29>(_data);
}
ClearCdcInfoResp& RegistryRespContainer::setClearCdcInfo() {
    _kind = RegistryMessageKind::CLEAR_CDC_INFO;
    auto& x = _data.emplace<29>();
    return x;
}
const UpdateBlockServicePathResp& RegistryRespContainer::getUpdateBlockServicePath() const {
    ALWAYS_ASSERT(_kind == RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH, "%s != %s", _kind, RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH);
    return std::get<30>(_data);
}
UpdateBlockServicePathResp& RegistryRespContainer::setUpdateBlockServicePath() {
    _kind = RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH;
    auto& x = _data.emplace<30>();
    return x;
}
RegistryRespContainer::RegistryRespContainer() {
    clear();
}

RegistryRespContainer::RegistryRespContainer(const RegistryRespContainer& other) {
    *this = other;
}

RegistryRespContainer::RegistryRespContainer(RegistryRespContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = RegistryMessageKind::EMPTY;
}

void RegistryRespContainer::operator=(const RegistryRespContainer& other) {
    if (other.kind() == RegistryMessageKind::EMPTY) { clear(); return; }
    switch (other.kind()) {
    case RegistryMessageKind::ERROR:
        setError() = other.getError();
        break;
    case RegistryMessageKind::LOCAL_SHARDS:
        setLocalShards() = other.getLocalShards();
        break;
    case RegistryMessageKind::LOCAL_CDC:
        setLocalCdc() = other.getLocalCdc();
        break;
    case RegistryMessageKind::INFO:
        setInfo() = other.getInfo();
        break;
    case RegistryMessageKind::REGISTRY:
        setRegistry() = other.getRegistry();
        break;
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        setLocalChangedBlockServices() = other.getLocalChangedBlockServices();
        break;
    case RegistryMessageKind::CREATE_LOCATION:
        setCreateLocation() = other.getCreateLocation();
        break;
    case RegistryMessageKind::RENAME_LOCATION:
        setRenameLocation() = other.getRenameLocation();
        break;
    case RegistryMessageKind::REGISTER_SHARD:
        setRegisterShard() = other.getRegisterShard();
        break;
    case RegistryMessageKind::LOCATIONS:
        setLocations() = other.getLocations();
        break;
    case RegistryMessageKind::REGISTER_CDC:
        setRegisterCdc() = other.getRegisterCdc();
        break;
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        setSetBlockServiceFlags() = other.getSetBlockServiceFlags();
        break;
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        setRegisterBlockServices() = other.getRegisterBlockServices();
        break;
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        setChangedBlockServicesAtLocation() = other.getChangedBlockServicesAtLocation();
        break;
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        setShardsAtLocation() = other.getShardsAtLocation();
        break;
    case RegistryMessageKind::CDC_AT_LOCATION:
        setCdcAtLocation() = other.getCdcAtLocation();
        break;
    case RegistryMessageKind::REGISTER_REGISTRY:
        setRegisterRegistry() = other.getRegisterRegistry();
        break;
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        setAllRegistryReplicas() = other.getAllRegistryReplicas();
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        setShardBlockServicesDEPRECATED() = other.getShardBlockServicesDEPRECATED();
        break;
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        setCdcReplicasDEPRECATED() = other.getCdcReplicasDEPRECATED();
        break;
    case RegistryMessageKind::ALL_SHARDS:
        setAllShards() = other.getAllShards();
        break;
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        setDecommissionBlockService() = other.getDecommissionBlockService();
        break;
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        setMoveShardLeader() = other.getMoveShardLeader();
        break;
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        setClearShardInfo() = other.getClearShardInfo();
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        setShardBlockServices() = other.getShardBlockServices();
        break;
    case RegistryMessageKind::ALL_CDC:
        setAllCdc() = other.getAllCdc();
        break;
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        setEraseDecommissionedBlock() = other.getEraseDecommissionedBlock();
        break;
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        setAllBlockServicesDeprecated() = other.getAllBlockServicesDeprecated();
        break;
    case RegistryMessageKind::MOVE_CDC_LEADER:
        setMoveCdcLeader() = other.getMoveCdcLeader();
        break;
    case RegistryMessageKind::CLEAR_CDC_INFO:
        setClearCdcInfo() = other.getClearCdcInfo();
        break;
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        setUpdateBlockServicePath() = other.getUpdateBlockServicePath();
        break;
    default:
        throw TERN_EXCEPTION("bad RegistryMessageKind kind %s", other.kind());
    }
}

void RegistryRespContainer::operator=(RegistryRespContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = RegistryMessageKind::EMPTY;
}

size_t RegistryRespContainer::packedSize() const {
    switch (_kind) {
    case RegistryMessageKind::ERROR:
        return sizeof(RegistryMessageKind) + sizeof(TernError);
    case RegistryMessageKind::LOCAL_SHARDS:
        return sizeof(RegistryMessageKind) + std::get<1>(_data).packedSize();
    case RegistryMessageKind::LOCAL_CDC:
        return sizeof(RegistryMessageKind) + std::get<2>(_data).packedSize();
    case RegistryMessageKind::INFO:
        return sizeof(RegistryMessageKind) + std::get<3>(_data).packedSize();
    case RegistryMessageKind::REGISTRY:
        return sizeof(RegistryMessageKind) + std::get<4>(_data).packedSize();
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        return sizeof(RegistryMessageKind) + std::get<5>(_data).packedSize();
    case RegistryMessageKind::CREATE_LOCATION:
        return sizeof(RegistryMessageKind) + std::get<6>(_data).packedSize();
    case RegistryMessageKind::RENAME_LOCATION:
        return sizeof(RegistryMessageKind) + std::get<7>(_data).packedSize();
    case RegistryMessageKind::REGISTER_SHARD:
        return sizeof(RegistryMessageKind) + std::get<8>(_data).packedSize();
    case RegistryMessageKind::LOCATIONS:
        return sizeof(RegistryMessageKind) + std::get<9>(_data).packedSize();
    case RegistryMessageKind::REGISTER_CDC:
        return sizeof(RegistryMessageKind) + std::get<10>(_data).packedSize();
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        return sizeof(RegistryMessageKind) + std::get<11>(_data).packedSize();
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        return sizeof(RegistryMessageKind) + std::get<12>(_data).packedSize();
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        return sizeof(RegistryMessageKind) + std::get<13>(_data).packedSize();
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        return sizeof(RegistryMessageKind) + std::get<14>(_data).packedSize();
    case RegistryMessageKind::CDC_AT_LOCATION:
        return sizeof(RegistryMessageKind) + std::get<15>(_data).packedSize();
    case RegistryMessageKind::REGISTER_REGISTRY:
        return sizeof(RegistryMessageKind) + std::get<16>(_data).packedSize();
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        return sizeof(RegistryMessageKind) + std::get<17>(_data).packedSize();
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        return sizeof(RegistryMessageKind) + std::get<18>(_data).packedSize();
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        return sizeof(RegistryMessageKind) + std::get<19>(_data).packedSize();
    case RegistryMessageKind::ALL_SHARDS:
        return sizeof(RegistryMessageKind) + std::get<20>(_data).packedSize();
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        return sizeof(RegistryMessageKind) + std::get<21>(_data).packedSize();
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        return sizeof(RegistryMessageKind) + std::get<22>(_data).packedSize();
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        return sizeof(RegistryMessageKind) + std::get<23>(_data).packedSize();
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        return sizeof(RegistryMessageKind) + std::get<24>(_data).packedSize();
    case RegistryMessageKind::ALL_CDC:
        return sizeof(RegistryMessageKind) + std::get<25>(_data).packedSize();
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        return sizeof(RegistryMessageKind) + std::get<26>(_data).packedSize();
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        return sizeof(RegistryMessageKind) + std::get<27>(_data).packedSize();
    case RegistryMessageKind::MOVE_CDC_LEADER:
        return sizeof(RegistryMessageKind) + std::get<28>(_data).packedSize();
    case RegistryMessageKind::CLEAR_CDC_INFO:
        return sizeof(RegistryMessageKind) + std::get<29>(_data).packedSize();
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        return sizeof(RegistryMessageKind) + std::get<30>(_data).packedSize();
    default:
        throw TERN_EXCEPTION("bad RegistryMessageKind kind %s", _kind);
    }
}

void RegistryRespContainer::pack(BincodeBuf& buf) const {
    buf.packScalar<RegistryMessageKind>(_kind);
    switch (_kind) {
    case RegistryMessageKind::ERROR:
        buf.packScalar<TernError>(std::get<0>(_data));
        break;
    case RegistryMessageKind::LOCAL_SHARDS:
        std::get<1>(_data).pack(buf);
        break;
    case RegistryMessageKind::LOCAL_CDC:
        std::get<2>(_data).pack(buf);
        break;
    case RegistryMessageKind::INFO:
        std::get<3>(_data).pack(buf);
        break;
    case RegistryMessageKind::REGISTRY:
        std::get<4>(_data).pack(buf);
        break;
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        std::get<5>(_data).pack(buf);
        break;
    case RegistryMessageKind::CREATE_LOCATION:
        std::get<6>(_data).pack(buf);
        break;
    case RegistryMessageKind::RENAME_LOCATION:
        std::get<7>(_data).pack(buf);
        break;
    case RegistryMessageKind::REGISTER_SHARD:
        std::get<8>(_data).pack(buf);
        break;
    case RegistryMessageKind::LOCATIONS:
        std::get<9>(_data).pack(buf);
        break;
    case RegistryMessageKind::REGISTER_CDC:
        std::get<10>(_data).pack(buf);
        break;
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        std::get<11>(_data).pack(buf);
        break;
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        std::get<12>(_data).pack(buf);
        break;
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        std::get<13>(_data).pack(buf);
        break;
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        std::get<14>(_data).pack(buf);
        break;
    case RegistryMessageKind::CDC_AT_LOCATION:
        std::get<15>(_data).pack(buf);
        break;
    case RegistryMessageKind::REGISTER_REGISTRY:
        std::get<16>(_data).pack(buf);
        break;
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        std::get<17>(_data).pack(buf);
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        std::get<18>(_data).pack(buf);
        break;
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        std::get<19>(_data).pack(buf);
        break;
    case RegistryMessageKind::ALL_SHARDS:
        std::get<20>(_data).pack(buf);
        break;
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        std::get<21>(_data).pack(buf);
        break;
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        std::get<22>(_data).pack(buf);
        break;
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        std::get<23>(_data).pack(buf);
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        std::get<24>(_data).pack(buf);
        break;
    case RegistryMessageKind::ALL_CDC:
        std::get<25>(_data).pack(buf);
        break;
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        std::get<26>(_data).pack(buf);
        break;
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        std::get<27>(_data).pack(buf);
        break;
    case RegistryMessageKind::MOVE_CDC_LEADER:
        std::get<28>(_data).pack(buf);
        break;
    case RegistryMessageKind::CLEAR_CDC_INFO:
        std::get<29>(_data).pack(buf);
        break;
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        std::get<30>(_data).pack(buf);
        break;
    default:
        throw TERN_EXCEPTION("bad RegistryMessageKind kind %s", _kind);
    }
}

void RegistryRespContainer::unpack(BincodeBuf& buf) {
    _kind = buf.unpackScalar<RegistryMessageKind>();
    switch (_kind) {
    case RegistryMessageKind::ERROR:
        _data.emplace<0>(buf.unpackScalar<TernError>());
        break;
    case RegistryMessageKind::LOCAL_SHARDS:
        _data.emplace<1>().unpack(buf);
        break;
    case RegistryMessageKind::LOCAL_CDC:
        _data.emplace<2>().unpack(buf);
        break;
    case RegistryMessageKind::INFO:
        _data.emplace<3>().unpack(buf);
        break;
    case RegistryMessageKind::REGISTRY:
        _data.emplace<4>().unpack(buf);
        break;
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        _data.emplace<5>().unpack(buf);
        break;
    case RegistryMessageKind::CREATE_LOCATION:
        _data.emplace<6>().unpack(buf);
        break;
    case RegistryMessageKind::RENAME_LOCATION:
        _data.emplace<7>().unpack(buf);
        break;
    case RegistryMessageKind::REGISTER_SHARD:
        _data.emplace<8>().unpack(buf);
        break;
    case RegistryMessageKind::LOCATIONS:
        _data.emplace<9>().unpack(buf);
        break;
    case RegistryMessageKind::REGISTER_CDC:
        _data.emplace<10>().unpack(buf);
        break;
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        _data.emplace<11>().unpack(buf);
        break;
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        _data.emplace<12>().unpack(buf);
        break;
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        _data.emplace<13>().unpack(buf);
        break;
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        _data.emplace<14>().unpack(buf);
        break;
    case RegistryMessageKind::CDC_AT_LOCATION:
        _data.emplace<15>().unpack(buf);
        break;
    case RegistryMessageKind::REGISTER_REGISTRY:
        _data.emplace<16>().unpack(buf);
        break;
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        _data.emplace<17>().unpack(buf);
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        _data.emplace<18>().unpack(buf);
        break;
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        _data.emplace<19>().unpack(buf);
        break;
    case RegistryMessageKind::ALL_SHARDS:
        _data.emplace<20>().unpack(buf);
        break;
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        _data.emplace<21>().unpack(buf);
        break;
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        _data.emplace<22>().unpack(buf);
        break;
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        _data.emplace<23>().unpack(buf);
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        _data.emplace<24>().unpack(buf);
        break;
    case RegistryMessageKind::ALL_CDC:
        _data.emplace<25>().unpack(buf);
        break;
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        _data.emplace<26>().unpack(buf);
        break;
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        _data.emplace<27>().unpack(buf);
        break;
    case RegistryMessageKind::MOVE_CDC_LEADER:
        _data.emplace<28>().unpack(buf);
        break;
    case RegistryMessageKind::CLEAR_CDC_INFO:
        _data.emplace<29>().unpack(buf);
        break;
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        _data.emplace<30>().unpack(buf);
        break;
    default:
        throw BINCODE_EXCEPTION("bad RegistryMessageKind kind %s", _kind);
    }
}

bool RegistryRespContainer::operator==(const RegistryRespContainer& other) const {
    if (_kind != other.kind()) { return false; }
    if (_kind == RegistryMessageKind::EMPTY) { return true; }
    switch (_kind) {
    case RegistryMessageKind::ERROR:
        return getError() == other.getError();
    case RegistryMessageKind::LOCAL_SHARDS:
        return getLocalShards() == other.getLocalShards();
    case RegistryMessageKind::LOCAL_CDC:
        return getLocalCdc() == other.getLocalCdc();
    case RegistryMessageKind::INFO:
        return getInfo() == other.getInfo();
    case RegistryMessageKind::REGISTRY:
        return getRegistry() == other.getRegistry();
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        return getLocalChangedBlockServices() == other.getLocalChangedBlockServices();
    case RegistryMessageKind::CREATE_LOCATION:
        return getCreateLocation() == other.getCreateLocation();
    case RegistryMessageKind::RENAME_LOCATION:
        return getRenameLocation() == other.getRenameLocation();
    case RegistryMessageKind::REGISTER_SHARD:
        return getRegisterShard() == other.getRegisterShard();
    case RegistryMessageKind::LOCATIONS:
        return getLocations() == other.getLocations();
    case RegistryMessageKind::REGISTER_CDC:
        return getRegisterCdc() == other.getRegisterCdc();
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        return getSetBlockServiceFlags() == other.getSetBlockServiceFlags();
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        return getRegisterBlockServices() == other.getRegisterBlockServices();
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        return getChangedBlockServicesAtLocation() == other.getChangedBlockServicesAtLocation();
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        return getShardsAtLocation() == other.getShardsAtLocation();
    case RegistryMessageKind::CDC_AT_LOCATION:
        return getCdcAtLocation() == other.getCdcAtLocation();
    case RegistryMessageKind::REGISTER_REGISTRY:
        return getRegisterRegistry() == other.getRegisterRegistry();
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        return getAllRegistryReplicas() == other.getAllRegistryReplicas();
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        return getShardBlockServicesDEPRECATED() == other.getShardBlockServicesDEPRECATED();
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        return getCdcReplicasDEPRECATED() == other.getCdcReplicasDEPRECATED();
    case RegistryMessageKind::ALL_SHARDS:
        return getAllShards() == other.getAllShards();
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        return getDecommissionBlockService() == other.getDecommissionBlockService();
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        return getMoveShardLeader() == other.getMoveShardLeader();
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        return getClearShardInfo() == other.getClearShardInfo();
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        return getShardBlockServices() == other.getShardBlockServices();
    case RegistryMessageKind::ALL_CDC:
        return getAllCdc() == other.getAllCdc();
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        return getEraseDecommissionedBlock() == other.getEraseDecommissionedBlock();
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        return getAllBlockServicesDeprecated() == other.getAllBlockServicesDeprecated();
    case RegistryMessageKind::MOVE_CDC_LEADER:
        return getMoveCdcLeader() == other.getMoveCdcLeader();
    case RegistryMessageKind::CLEAR_CDC_INFO:
        return getClearCdcInfo() == other.getClearCdcInfo();
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        return getUpdateBlockServicePath() == other.getUpdateBlockServicePath();
    default:
        throw BINCODE_EXCEPTION("bad RegistryMessageKind kind %s", _kind);
    }
}

std::ostream& operator<<(std::ostream& out, const RegistryRespContainer& x) {
    switch (x.kind()) {
    case RegistryMessageKind::ERROR:
        out << x.getError();
        break;
    case RegistryMessageKind::LOCAL_SHARDS:
        out << x.getLocalShards();
        break;
    case RegistryMessageKind::LOCAL_CDC:
        out << x.getLocalCdc();
        break;
    case RegistryMessageKind::INFO:
        out << x.getInfo();
        break;
    case RegistryMessageKind::REGISTRY:
        out << x.getRegistry();
        break;
    case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
        out << x.getLocalChangedBlockServices();
        break;
    case RegistryMessageKind::CREATE_LOCATION:
        out << x.getCreateLocation();
        break;
    case RegistryMessageKind::RENAME_LOCATION:
        out << x.getRenameLocation();
        break;
    case RegistryMessageKind::REGISTER_SHARD:
        out << x.getRegisterShard();
        break;
    case RegistryMessageKind::LOCATIONS:
        out << x.getLocations();
        break;
    case RegistryMessageKind::REGISTER_CDC:
        out << x.getRegisterCdc();
        break;
    case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
        out << x.getSetBlockServiceFlags();
        break;
    case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
        out << x.getRegisterBlockServices();
        break;
    case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
        out << x.getChangedBlockServicesAtLocation();
        break;
    case RegistryMessageKind::SHARDS_AT_LOCATION:
        out << x.getShardsAtLocation();
        break;
    case RegistryMessageKind::CDC_AT_LOCATION:
        out << x.getCdcAtLocation();
        break;
    case RegistryMessageKind::REGISTER_REGISTRY:
        out << x.getRegisterRegistry();
        break;
    case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
        out << x.getAllRegistryReplicas();
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
        out << x.getShardBlockServicesDEPRECATED();
        break;
    case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
        out << x.getCdcReplicasDEPRECATED();
        break;
    case RegistryMessageKind::ALL_SHARDS:
        out << x.getAllShards();
        break;
    case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
        out << x.getDecommissionBlockService();
        break;
    case RegistryMessageKind::MOVE_SHARD_LEADER:
        out << x.getMoveShardLeader();
        break;
    case RegistryMessageKind::CLEAR_SHARD_INFO:
        out << x.getClearShardInfo();
        break;
    case RegistryMessageKind::SHARD_BLOCK_SERVICES:
        out << x.getShardBlockServices();
        break;
    case RegistryMessageKind::ALL_CDC:
        out << x.getAllCdc();
        break;
    case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
        out << x.getEraseDecommissionedBlock();
        break;
    case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
        out << x.getAllBlockServicesDeprecated();
        break;
    case RegistryMessageKind::MOVE_CDC_LEADER:
        out << x.getMoveCdcLeader();
        break;
    case RegistryMessageKind::CLEAR_CDC_INFO:
        out << x.getClearCdcInfo();
        break;
    case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
        out << x.getUpdateBlockServicePath();
        break;
    case RegistryMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        throw TERN_EXCEPTION("bad RegistryMessageKind kind %s", x.kind());
    }
    return out;
}

const LogWriteReq& LogReqContainer::getLogWrite() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::LOG_WRITE, "%s != %s", _kind, LogMessageKind::LOG_WRITE);
    return std::get<0>(_data);
}
LogWriteReq& LogReqContainer::setLogWrite() {
    _kind = LogMessageKind::LOG_WRITE;
    auto& x = _data.emplace<0>();
    return x;
}
const ReleaseReq& LogReqContainer::getRelease() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::RELEASE, "%s != %s", _kind, LogMessageKind::RELEASE);
    return std::get<1>(_data);
}
ReleaseReq& LogReqContainer::setRelease() {
    _kind = LogMessageKind::RELEASE;
    auto& x = _data.emplace<1>();
    return x;
}
const LogReadReq& LogReqContainer::getLogRead() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::LOG_READ, "%s != %s", _kind, LogMessageKind::LOG_READ);
    return std::get<2>(_data);
}
LogReadReq& LogReqContainer::setLogRead() {
    _kind = LogMessageKind::LOG_READ;
    auto& x = _data.emplace<2>();
    return x;
}
const NewLeaderReq& LogReqContainer::getNewLeader() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::NEW_LEADER, "%s != %s", _kind, LogMessageKind::NEW_LEADER);
    return std::get<3>(_data);
}
NewLeaderReq& LogReqContainer::setNewLeader() {
    _kind = LogMessageKind::NEW_LEADER;
    auto& x = _data.emplace<3>();
    return x;
}
const NewLeaderConfirmReq& LogReqContainer::getNewLeaderConfirm() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::NEW_LEADER_CONFIRM, "%s != %s", _kind, LogMessageKind::NEW_LEADER_CONFIRM);
    return std::get<4>(_data);
}
NewLeaderConfirmReq& LogReqContainer::setNewLeaderConfirm() {
    _kind = LogMessageKind::NEW_LEADER_CONFIRM;
    auto& x = _data.emplace<4>();
    return x;
}
const LogRecoveryReadReq& LogReqContainer::getLogRecoveryRead() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::LOG_RECOVERY_READ, "%s != %s", _kind, LogMessageKind::LOG_RECOVERY_READ);
    return std::get<5>(_data);
}
LogRecoveryReadReq& LogReqContainer::setLogRecoveryRead() {
    _kind = LogMessageKind::LOG_RECOVERY_READ;
    auto& x = _data.emplace<5>();
    return x;
}
const LogRecoveryWriteReq& LogReqContainer::getLogRecoveryWrite() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::LOG_RECOVERY_WRITE, "%s != %s", _kind, LogMessageKind::LOG_RECOVERY_WRITE);
    return std::get<6>(_data);
}
LogRecoveryWriteReq& LogReqContainer::setLogRecoveryWrite() {
    _kind = LogMessageKind::LOG_RECOVERY_WRITE;
    auto& x = _data.emplace<6>();
    return x;
}
LogReqContainer::LogReqContainer() {
    clear();
}

LogReqContainer::LogReqContainer(const LogReqContainer& other) {
    *this = other;
}

LogReqContainer::LogReqContainer(LogReqContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = LogMessageKind::EMPTY;
}

void LogReqContainer::operator=(const LogReqContainer& other) {
    if (other.kind() == LogMessageKind::EMPTY) { clear(); return; }
    switch (other.kind()) {
    case LogMessageKind::LOG_WRITE:
        setLogWrite() = other.getLogWrite();
        break;
    case LogMessageKind::RELEASE:
        setRelease() = other.getRelease();
        break;
    case LogMessageKind::LOG_READ:
        setLogRead() = other.getLogRead();
        break;
    case LogMessageKind::NEW_LEADER:
        setNewLeader() = other.getNewLeader();
        break;
    case LogMessageKind::NEW_LEADER_CONFIRM:
        setNewLeaderConfirm() = other.getNewLeaderConfirm();
        break;
    case LogMessageKind::LOG_RECOVERY_READ:
        setLogRecoveryRead() = other.getLogRecoveryRead();
        break;
    case LogMessageKind::LOG_RECOVERY_WRITE:
        setLogRecoveryWrite() = other.getLogRecoveryWrite();
        break;
    default:
        throw TERN_EXCEPTION("bad LogMessageKind kind %s", other.kind());
    }
}

void LogReqContainer::operator=(LogReqContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = LogMessageKind::EMPTY;
}

size_t LogReqContainer::packedSize() const {
    switch (_kind) {
    case LogMessageKind::LOG_WRITE:
        return sizeof(LogMessageKind) + std::get<0>(_data).packedSize();
    case LogMessageKind::RELEASE:
        return sizeof(LogMessageKind) + std::get<1>(_data).packedSize();
    case LogMessageKind::LOG_READ:
        return sizeof(LogMessageKind) + std::get<2>(_data).packedSize();
    case LogMessageKind::NEW_LEADER:
        return sizeof(LogMessageKind) + std::get<3>(_data).packedSize();
    case LogMessageKind::NEW_LEADER_CONFIRM:
        return sizeof(LogMessageKind) + std::get<4>(_data).packedSize();
    case LogMessageKind::LOG_RECOVERY_READ:
        return sizeof(LogMessageKind) + std::get<5>(_data).packedSize();
    case LogMessageKind::LOG_RECOVERY_WRITE:
        return sizeof(LogMessageKind) + std::get<6>(_data).packedSize();
    default:
        throw TERN_EXCEPTION("bad LogMessageKind kind %s", _kind);
    }
}

void LogReqContainer::pack(BincodeBuf& buf) const {
    buf.packScalar<LogMessageKind>(_kind);
    switch (_kind) {
    case LogMessageKind::LOG_WRITE:
        std::get<0>(_data).pack(buf);
        break;
    case LogMessageKind::RELEASE:
        std::get<1>(_data).pack(buf);
        break;
    case LogMessageKind::LOG_READ:
        std::get<2>(_data).pack(buf);
        break;
    case LogMessageKind::NEW_LEADER:
        std::get<3>(_data).pack(buf);
        break;
    case LogMessageKind::NEW_LEADER_CONFIRM:
        std::get<4>(_data).pack(buf);
        break;
    case LogMessageKind::LOG_RECOVERY_READ:
        std::get<5>(_data).pack(buf);
        break;
    case LogMessageKind::LOG_RECOVERY_WRITE:
        std::get<6>(_data).pack(buf);
        break;
    default:
        throw TERN_EXCEPTION("bad LogMessageKind kind %s", _kind);
    }
}

void LogReqContainer::unpack(BincodeBuf& buf) {
    _kind = buf.unpackScalar<LogMessageKind>();
    switch (_kind) {
    case LogMessageKind::LOG_WRITE:
        _data.emplace<0>().unpack(buf);
        break;
    case LogMessageKind::RELEASE:
        _data.emplace<1>().unpack(buf);
        break;
    case LogMessageKind::LOG_READ:
        _data.emplace<2>().unpack(buf);
        break;
    case LogMessageKind::NEW_LEADER:
        _data.emplace<3>().unpack(buf);
        break;
    case LogMessageKind::NEW_LEADER_CONFIRM:
        _data.emplace<4>().unpack(buf);
        break;
    case LogMessageKind::LOG_RECOVERY_READ:
        _data.emplace<5>().unpack(buf);
        break;
    case LogMessageKind::LOG_RECOVERY_WRITE:
        _data.emplace<6>().unpack(buf);
        break;
    default:
        throw BINCODE_EXCEPTION("bad LogMessageKind kind %s", _kind);
    }
}

bool LogReqContainer::operator==(const LogReqContainer& other) const {
    if (_kind != other.kind()) { return false; }
    if (_kind == LogMessageKind::EMPTY) { return true; }
    switch (_kind) {
    case LogMessageKind::LOG_WRITE:
        return getLogWrite() == other.getLogWrite();
    case LogMessageKind::RELEASE:
        return getRelease() == other.getRelease();
    case LogMessageKind::LOG_READ:
        return getLogRead() == other.getLogRead();
    case LogMessageKind::NEW_LEADER:
        return getNewLeader() == other.getNewLeader();
    case LogMessageKind::NEW_LEADER_CONFIRM:
        return getNewLeaderConfirm() == other.getNewLeaderConfirm();
    case LogMessageKind::LOG_RECOVERY_READ:
        return getLogRecoveryRead() == other.getLogRecoveryRead();
    case LogMessageKind::LOG_RECOVERY_WRITE:
        return getLogRecoveryWrite() == other.getLogRecoveryWrite();
    default:
        throw BINCODE_EXCEPTION("bad LogMessageKind kind %s", _kind);
    }
}

std::ostream& operator<<(std::ostream& out, const LogReqContainer& x) {
    switch (x.kind()) {
    case LogMessageKind::LOG_WRITE:
        out << x.getLogWrite();
        break;
    case LogMessageKind::RELEASE:
        out << x.getRelease();
        break;
    case LogMessageKind::LOG_READ:
        out << x.getLogRead();
        break;
    case LogMessageKind::NEW_LEADER:
        out << x.getNewLeader();
        break;
    case LogMessageKind::NEW_LEADER_CONFIRM:
        out << x.getNewLeaderConfirm();
        break;
    case LogMessageKind::LOG_RECOVERY_READ:
        out << x.getLogRecoveryRead();
        break;
    case LogMessageKind::LOG_RECOVERY_WRITE:
        out << x.getLogRecoveryWrite();
        break;
    case LogMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        throw TERN_EXCEPTION("bad LogMessageKind kind %s", x.kind());
    }
    return out;
}

const TernError& LogRespContainer::getError() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::ERROR, "%s != %s", _kind, LogMessageKind::ERROR);
    return std::get<0>(_data);
}
TernError& LogRespContainer::setError() {
    _kind = LogMessageKind::ERROR;
    auto& x = _data.emplace<0>();
    return x;
}
const LogWriteResp& LogRespContainer::getLogWrite() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::LOG_WRITE, "%s != %s", _kind, LogMessageKind::LOG_WRITE);
    return std::get<1>(_data);
}
LogWriteResp& LogRespContainer::setLogWrite() {
    _kind = LogMessageKind::LOG_WRITE;
    auto& x = _data.emplace<1>();
    return x;
}
const ReleaseResp& LogRespContainer::getRelease() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::RELEASE, "%s != %s", _kind, LogMessageKind::RELEASE);
    return std::get<2>(_data);
}
ReleaseResp& LogRespContainer::setRelease() {
    _kind = LogMessageKind::RELEASE;
    auto& x = _data.emplace<2>();
    return x;
}
const LogReadResp& LogRespContainer::getLogRead() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::LOG_READ, "%s != %s", _kind, LogMessageKind::LOG_READ);
    return std::get<3>(_data);
}
LogReadResp& LogRespContainer::setLogRead() {
    _kind = LogMessageKind::LOG_READ;
    auto& x = _data.emplace<3>();
    return x;
}
const NewLeaderResp& LogRespContainer::getNewLeader() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::NEW_LEADER, "%s != %s", _kind, LogMessageKind::NEW_LEADER);
    return std::get<4>(_data);
}
NewLeaderResp& LogRespContainer::setNewLeader() {
    _kind = LogMessageKind::NEW_LEADER;
    auto& x = _data.emplace<4>();
    return x;
}
const NewLeaderConfirmResp& LogRespContainer::getNewLeaderConfirm() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::NEW_LEADER_CONFIRM, "%s != %s", _kind, LogMessageKind::NEW_LEADER_CONFIRM);
    return std::get<5>(_data);
}
NewLeaderConfirmResp& LogRespContainer::setNewLeaderConfirm() {
    _kind = LogMessageKind::NEW_LEADER_CONFIRM;
    auto& x = _data.emplace<5>();
    return x;
}
const LogRecoveryReadResp& LogRespContainer::getLogRecoveryRead() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::LOG_RECOVERY_READ, "%s != %s", _kind, LogMessageKind::LOG_RECOVERY_READ);
    return std::get<6>(_data);
}
LogRecoveryReadResp& LogRespContainer::setLogRecoveryRead() {
    _kind = LogMessageKind::LOG_RECOVERY_READ;
    auto& x = _data.emplace<6>();
    return x;
}
const LogRecoveryWriteResp& LogRespContainer::getLogRecoveryWrite() const {
    ALWAYS_ASSERT(_kind == LogMessageKind::LOG_RECOVERY_WRITE, "%s != %s", _kind, LogMessageKind::LOG_RECOVERY_WRITE);
    return std::get<7>(_data);
}
LogRecoveryWriteResp& LogRespContainer::setLogRecoveryWrite() {
    _kind = LogMessageKind::LOG_RECOVERY_WRITE;
    auto& x = _data.emplace<7>();
    return x;
}
LogRespContainer::LogRespContainer() {
    clear();
}

LogRespContainer::LogRespContainer(const LogRespContainer& other) {
    *this = other;
}

LogRespContainer::LogRespContainer(LogRespContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = LogMessageKind::EMPTY;
}

void LogRespContainer::operator=(const LogRespContainer& other) {
    if (other.kind() == LogMessageKind::EMPTY) { clear(); return; }
    switch (other.kind()) {
    case LogMessageKind::ERROR:
        setError() = other.getError();
        break;
    case LogMessageKind::LOG_WRITE:
        setLogWrite() = other.getLogWrite();
        break;
    case LogMessageKind::RELEASE:
        setRelease() = other.getRelease();
        break;
    case LogMessageKind::LOG_READ:
        setLogRead() = other.getLogRead();
        break;
    case LogMessageKind::NEW_LEADER:
        setNewLeader() = other.getNewLeader();
        break;
    case LogMessageKind::NEW_LEADER_CONFIRM:
        setNewLeaderConfirm() = other.getNewLeaderConfirm();
        break;
    case LogMessageKind::LOG_RECOVERY_READ:
        setLogRecoveryRead() = other.getLogRecoveryRead();
        break;
    case LogMessageKind::LOG_RECOVERY_WRITE:
        setLogRecoveryWrite() = other.getLogRecoveryWrite();
        break;
    default:
        throw TERN_EXCEPTION("bad LogMessageKind kind %s", other.kind());
    }
}

void LogRespContainer::operator=(LogRespContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = LogMessageKind::EMPTY;
}

size_t LogRespContainer::packedSize() const {
    switch (_kind) {
    case LogMessageKind::ERROR:
        return sizeof(LogMessageKind) + sizeof(TernError);
    case LogMessageKind::LOG_WRITE:
        return sizeof(LogMessageKind) + std::get<1>(_data).packedSize();
    case LogMessageKind::RELEASE:
        return sizeof(LogMessageKind) + std::get<2>(_data).packedSize();
    case LogMessageKind::LOG_READ:
        return sizeof(LogMessageKind) + std::get<3>(_data).packedSize();
    case LogMessageKind::NEW_LEADER:
        return sizeof(LogMessageKind) + std::get<4>(_data).packedSize();
    case LogMessageKind::NEW_LEADER_CONFIRM:
        return sizeof(LogMessageKind) + std::get<5>(_data).packedSize();
    case LogMessageKind::LOG_RECOVERY_READ:
        return sizeof(LogMessageKind) + std::get<6>(_data).packedSize();
    case LogMessageKind::LOG_RECOVERY_WRITE:
        return sizeof(LogMessageKind) + std::get<7>(_data).packedSize();
    default:
        throw TERN_EXCEPTION("bad LogMessageKind kind %s", _kind);
    }
}

void LogRespContainer::pack(BincodeBuf& buf) const {
    buf.packScalar<LogMessageKind>(_kind);
    switch (_kind) {
    case LogMessageKind::ERROR:
        buf.packScalar<TernError>(std::get<0>(_data));
        break;
    case LogMessageKind::LOG_WRITE:
        std::get<1>(_data).pack(buf);
        break;
    case LogMessageKind::RELEASE:
        std::get<2>(_data).pack(buf);
        break;
    case LogMessageKind::LOG_READ:
        std::get<3>(_data).pack(buf);
        break;
    case LogMessageKind::NEW_LEADER:
        std::get<4>(_data).pack(buf);
        break;
    case LogMessageKind::NEW_LEADER_CONFIRM:
        std::get<5>(_data).pack(buf);
        break;
    case LogMessageKind::LOG_RECOVERY_READ:
        std::get<6>(_data).pack(buf);
        break;
    case LogMessageKind::LOG_RECOVERY_WRITE:
        std::get<7>(_data).pack(buf);
        break;
    default:
        throw TERN_EXCEPTION("bad LogMessageKind kind %s", _kind);
    }
}

void LogRespContainer::unpack(BincodeBuf& buf) {
    _kind = buf.unpackScalar<LogMessageKind>();
    switch (_kind) {
    case LogMessageKind::ERROR:
        _data.emplace<0>(buf.unpackScalar<TernError>());
        break;
    case LogMessageKind::LOG_WRITE:
        _data.emplace<1>().unpack(buf);
        break;
    case LogMessageKind::RELEASE:
        _data.emplace<2>().unpack(buf);
        break;
    case LogMessageKind::LOG_READ:
        _data.emplace<3>().unpack(buf);
        break;
    case LogMessageKind::NEW_LEADER:
        _data.emplace<4>().unpack(buf);
        break;
    case LogMessageKind::NEW_LEADER_CONFIRM:
        _data.emplace<5>().unpack(buf);
        break;
    case LogMessageKind::LOG_RECOVERY_READ:
        _data.emplace<6>().unpack(buf);
        break;
    case LogMessageKind::LOG_RECOVERY_WRITE:
        _data.emplace<7>().unpack(buf);
        break;
    default:
        throw BINCODE_EXCEPTION("bad LogMessageKind kind %s", _kind);
    }
}

bool LogRespContainer::operator==(const LogRespContainer& other) const {
    if (_kind != other.kind()) { return false; }
    if (_kind == LogMessageKind::EMPTY) { return true; }
    switch (_kind) {
    case LogMessageKind::ERROR:
        return getError() == other.getError();
    case LogMessageKind::LOG_WRITE:
        return getLogWrite() == other.getLogWrite();
    case LogMessageKind::RELEASE:
        return getRelease() == other.getRelease();
    case LogMessageKind::LOG_READ:
        return getLogRead() == other.getLogRead();
    case LogMessageKind::NEW_LEADER:
        return getNewLeader() == other.getNewLeader();
    case LogMessageKind::NEW_LEADER_CONFIRM:
        return getNewLeaderConfirm() == other.getNewLeaderConfirm();
    case LogMessageKind::LOG_RECOVERY_READ:
        return getLogRecoveryRead() == other.getLogRecoveryRead();
    case LogMessageKind::LOG_RECOVERY_WRITE:
        return getLogRecoveryWrite() == other.getLogRecoveryWrite();
    default:
        throw BINCODE_EXCEPTION("bad LogMessageKind kind %s", _kind);
    }
}

std::ostream& operator<<(std::ostream& out, const LogRespContainer& x) {
    switch (x.kind()) {
    case LogMessageKind::ERROR:
        out << x.getError();
        break;
    case LogMessageKind::LOG_WRITE:
        out << x.getLogWrite();
        break;
    case LogMessageKind::RELEASE:
        out << x.getRelease();
        break;
    case LogMessageKind::LOG_READ:
        out << x.getLogRead();
        break;
    case LogMessageKind::NEW_LEADER:
        out << x.getNewLeader();
        break;
    case LogMessageKind::NEW_LEADER_CONFIRM:
        out << x.getNewLeaderConfirm();
        break;
    case LogMessageKind::LOG_RECOVERY_READ:
        out << x.getLogRecoveryRead();
        break;
    case LogMessageKind::LOG_RECOVERY_WRITE:
        out << x.getLogRecoveryWrite();
        break;
    case LogMessageKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        throw TERN_EXCEPTION("bad LogMessageKind kind %s", x.kind());
    }
    return out;
}

std::ostream& operator<<(std::ostream& out, ShardLogEntryKind err) {
    switch (err) {
    case ShardLogEntryKind::CONSTRUCT_FILE:
        out << "CONSTRUCT_FILE";
        break;
    case ShardLogEntryKind::LINK_FILE:
        out << "LINK_FILE";
        break;
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME:
        out << "SAME_DIRECTORY_RENAME";
        break;
    case ShardLogEntryKind::SOFT_UNLINK_FILE:
        out << "SOFT_UNLINK_FILE";
        break;
    case ShardLogEntryKind::CREATE_DIRECTORY_INODE:
        out << "CREATE_DIRECTORY_INODE";
        break;
    case ShardLogEntryKind::CREATE_LOCKED_CURRENT_EDGE:
        out << "CREATE_LOCKED_CURRENT_EDGE";
        break;
    case ShardLogEntryKind::UNLOCK_CURRENT_EDGE:
        out << "UNLOCK_CURRENT_EDGE";
        break;
    case ShardLogEntryKind::LOCK_CURRENT_EDGE:
        out << "LOCK_CURRENT_EDGE";
        break;
    case ShardLogEntryKind::REMOVE_DIRECTORY_OWNER:
        out << "REMOVE_DIRECTORY_OWNER";
        break;
    case ShardLogEntryKind::REMOVE_INODE:
        out << "REMOVE_INODE";
        break;
    case ShardLogEntryKind::SET_DIRECTORY_OWNER:
        out << "SET_DIRECTORY_OWNER";
        break;
    case ShardLogEntryKind::SET_DIRECTORY_INFO:
        out << "SET_DIRECTORY_INFO";
        break;
    case ShardLogEntryKind::REMOVE_NON_OWNED_EDGE:
        out << "REMOVE_NON_OWNED_EDGE";
        break;
    case ShardLogEntryKind::SCRAP_TRANSIENT_FILE:
        out << "SCRAP_TRANSIENT_FILE";
        break;
    case ShardLogEntryKind::REMOVE_SPAN_INITIATE:
        out << "REMOVE_SPAN_INITIATE";
        break;
    case ShardLogEntryKind::ADD_SPAN_INITIATE:
        out << "ADD_SPAN_INITIATE";
        break;
    case ShardLogEntryKind::ADD_SPAN_CERTIFY:
        out << "ADD_SPAN_CERTIFY";
        break;
    case ShardLogEntryKind::ADD_INLINE_SPAN:
        out << "ADD_INLINE_SPAN";
        break;
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED:
        out << "MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED";
        break;
    case ShardLogEntryKind::REMOVE_SPAN_CERTIFY:
        out << "REMOVE_SPAN_CERTIFY";
        break;
    case ShardLogEntryKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        out << "REMOVE_OWNED_SNAPSHOT_FILE_EDGE";
        break;
    case ShardLogEntryKind::SWAP_BLOCKS:
        out << "SWAP_BLOCKS";
        break;
    case ShardLogEntryKind::MOVE_SPAN:
        out << "MOVE_SPAN";
        break;
    case ShardLogEntryKind::SET_TIME:
        out << "SET_TIME";
        break;
    case ShardLogEntryKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        out << "REMOVE_ZERO_BLOCK_SERVICE_FILES";
        break;
    case ShardLogEntryKind::SWAP_SPANS:
        out << "SWAP_SPANS";
        break;
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        out << "SAME_DIRECTORY_RENAME_SNAPSHOT";
        break;
    case ShardLogEntryKind::ADD_SPAN_AT_LOCATION_INITIATE:
        out << "ADD_SPAN_AT_LOCATION_INITIATE";
        break;
    case ShardLogEntryKind::ADD_SPAN_LOCATION:
        out << "ADD_SPAN_LOCATION";
        break;
    case ShardLogEntryKind::SAME_SHARD_HARD_FILE_UNLINK:
        out << "SAME_SHARD_HARD_FILE_UNLINK";
        break;
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT:
        out << "MAKE_FILE_TRANSIENT";
        break;
    case ShardLogEntryKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        out << "ShardLogEntryKind(" << ((int)err) << ")";
        break;
    }
    return out;
}

void ConstructFileEntry::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(type);
    deadlineTime.pack(buf);
    buf.packBytes(note);
}
void ConstructFileEntry::unpack(BincodeBuf& buf) {
    type = buf.unpackScalar<uint8_t>();
    deadlineTime.unpack(buf);
    buf.unpackBytes(note);
}
void ConstructFileEntry::clear() {
    type = uint8_t(0);
    deadlineTime = TernTime();
    note.clear();
}
bool ConstructFileEntry::operator==(const ConstructFileEntry& rhs) const {
    if ((uint8_t)this->type != (uint8_t)rhs.type) { return false; };
    if ((TernTime)this->deadlineTime != (TernTime)rhs.deadlineTime) { return false; };
    if (note != rhs.note) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ConstructFileEntry& x) {
    out << "ConstructFileEntry(" << "Type=" << (int)x.type << ", " << "DeadlineTime=" << x.deadlineTime << ", " << "Note=" << GoLangQuotedStringFmt(x.note.data(), x.note.size()) << ")";
    return out;
}

void LinkFileEntry::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    ownerId.pack(buf);
    buf.packBytes(name);
}
void LinkFileEntry::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    ownerId.unpack(buf);
    buf.unpackBytes(name);
}
void LinkFileEntry::clear() {
    fileId = InodeId();
    ownerId = InodeId();
    name.clear();
}
bool LinkFileEntry::operator==(const LinkFileEntry& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if (name != rhs.name) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LinkFileEntry& x) {
    out << "LinkFileEntry(" << "FileId=" << x.fileId << ", " << "OwnerId=" << x.ownerId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ")";
    return out;
}

void SameDirectoryRenameEntry::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    targetId.pack(buf);
    buf.packBytes(oldName);
    oldCreationTime.pack(buf);
    buf.packBytes(newName);
}
void SameDirectoryRenameEntry::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    targetId.unpack(buf);
    buf.unpackBytes(oldName);
    oldCreationTime.unpack(buf);
    buf.unpackBytes(newName);
}
void SameDirectoryRenameEntry::clear() {
    dirId = InodeId();
    targetId = InodeId();
    oldName.clear();
    oldCreationTime = TernTime();
    newName.clear();
}
bool SameDirectoryRenameEntry::operator==(const SameDirectoryRenameEntry& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if (oldName != rhs.oldName) { return false; };
    if ((TernTime)this->oldCreationTime != (TernTime)rhs.oldCreationTime) { return false; };
    if (newName != rhs.newName) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameEntry& x) {
    out << "SameDirectoryRenameEntry(" << "DirId=" << x.dirId << ", " << "TargetId=" << x.targetId << ", " << "OldName=" << GoLangQuotedStringFmt(x.oldName.data(), x.oldName.size()) << ", " << "OldCreationTime=" << x.oldCreationTime << ", " << "NewName=" << GoLangQuotedStringFmt(x.newName.data(), x.newName.size()) << ")";
    return out;
}

void SoftUnlinkFileEntry::pack(BincodeBuf& buf) const {
    ownerId.pack(buf);
    fileId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
}
void SoftUnlinkFileEntry::unpack(BincodeBuf& buf) {
    ownerId.unpack(buf);
    fileId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
}
void SoftUnlinkFileEntry::clear() {
    ownerId = InodeId();
    fileId = InodeId();
    name.clear();
    creationTime = TernTime();
}
bool SoftUnlinkFileEntry::operator==(const SoftUnlinkFileEntry& rhs) const {
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SoftUnlinkFileEntry& x) {
    out << "SoftUnlinkFileEntry(" << "OwnerId=" << x.ownerId << ", " << "FileId=" << x.fileId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void CreateDirectoryInodeEntry::pack(BincodeBuf& buf) const {
    id.pack(buf);
    ownerId.pack(buf);
    info.pack(buf);
}
void CreateDirectoryInodeEntry::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    ownerId.unpack(buf);
    info.unpack(buf);
}
void CreateDirectoryInodeEntry::clear() {
    id = InodeId();
    ownerId = InodeId();
    info.clear();
}
bool CreateDirectoryInodeEntry::operator==(const CreateDirectoryInodeEntry& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if (info != rhs.info) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CreateDirectoryInodeEntry& x) {
    out << "CreateDirectoryInodeEntry(" << "Id=" << x.id << ", " << "OwnerId=" << x.ownerId << ", " << "Info=" << x.info << ")";
    return out;
}

void CreateLockedCurrentEdgeEntry::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    buf.packBytes(name);
    targetId.pack(buf);
    oldCreationTime.pack(buf);
}
void CreateLockedCurrentEdgeEntry::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    buf.unpackBytes(name);
    targetId.unpack(buf);
    oldCreationTime.unpack(buf);
}
void CreateLockedCurrentEdgeEntry::clear() {
    dirId = InodeId();
    name.clear();
    targetId = InodeId();
    oldCreationTime = TernTime();
}
bool CreateLockedCurrentEdgeEntry::operator==(const CreateLockedCurrentEdgeEntry& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (name != rhs.name) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((TernTime)this->oldCreationTime != (TernTime)rhs.oldCreationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const CreateLockedCurrentEdgeEntry& x) {
    out << "CreateLockedCurrentEdgeEntry(" << "DirId=" << x.dirId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "TargetId=" << x.targetId << ", " << "OldCreationTime=" << x.oldCreationTime << ")";
    return out;
}

void UnlockCurrentEdgeEntry::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
    targetId.pack(buf);
    buf.packScalar<bool>(wasMoved);
}
void UnlockCurrentEdgeEntry::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
    targetId.unpack(buf);
    wasMoved = buf.unpackScalar<bool>();
}
void UnlockCurrentEdgeEntry::clear() {
    dirId = InodeId();
    name.clear();
    creationTime = TernTime();
    targetId = InodeId();
    wasMoved = bool(0);
}
bool UnlockCurrentEdgeEntry::operator==(const UnlockCurrentEdgeEntry& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if ((bool)this->wasMoved != (bool)rhs.wasMoved) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const UnlockCurrentEdgeEntry& x) {
    out << "UnlockCurrentEdgeEntry(" << "DirId=" << x.dirId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ", " << "TargetId=" << x.targetId << ", " << "WasMoved=" << x.wasMoved << ")";
    return out;
}

void LockCurrentEdgeEntry::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
    targetId.pack(buf);
}
void LockCurrentEdgeEntry::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
    targetId.unpack(buf);
}
void LockCurrentEdgeEntry::clear() {
    dirId = InodeId();
    name.clear();
    creationTime = TernTime();
    targetId = InodeId();
}
bool LockCurrentEdgeEntry::operator==(const LockCurrentEdgeEntry& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const LockCurrentEdgeEntry& x) {
    out << "LockCurrentEdgeEntry(" << "DirId=" << x.dirId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ", " << "TargetId=" << x.targetId << ")";
    return out;
}

void RemoveDirectoryOwnerEntry::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    info.pack(buf);
}
void RemoveDirectoryOwnerEntry::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    info.unpack(buf);
}
void RemoveDirectoryOwnerEntry::clear() {
    dirId = InodeId();
    info.clear();
}
bool RemoveDirectoryOwnerEntry::operator==(const RemoveDirectoryOwnerEntry& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (info != rhs.info) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveDirectoryOwnerEntry& x) {
    out << "RemoveDirectoryOwnerEntry(" << "DirId=" << x.dirId << ", " << "Info=" << x.info << ")";
    return out;
}

void RemoveInodeEntry::pack(BincodeBuf& buf) const {
    id.pack(buf);
}
void RemoveInodeEntry::unpack(BincodeBuf& buf) {
    id.unpack(buf);
}
void RemoveInodeEntry::clear() {
    id = InodeId();
}
bool RemoveInodeEntry::operator==(const RemoveInodeEntry& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveInodeEntry& x) {
    out << "RemoveInodeEntry(" << "Id=" << x.id << ")";
    return out;
}

void SetDirectoryOwnerEntry::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    ownerId.pack(buf);
}
void SetDirectoryOwnerEntry::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    ownerId.unpack(buf);
}
void SetDirectoryOwnerEntry::clear() {
    dirId = InodeId();
    ownerId = InodeId();
}
bool SetDirectoryOwnerEntry::operator==(const SetDirectoryOwnerEntry& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetDirectoryOwnerEntry& x) {
    out << "SetDirectoryOwnerEntry(" << "DirId=" << x.dirId << ", " << "OwnerId=" << x.ownerId << ")";
    return out;
}

void SetDirectoryInfoEntry::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    info.pack(buf);
}
void SetDirectoryInfoEntry::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    info.unpack(buf);
}
void SetDirectoryInfoEntry::clear() {
    dirId = InodeId();
    info.clear();
}
bool SetDirectoryInfoEntry::operator==(const SetDirectoryInfoEntry& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if (info != rhs.info) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetDirectoryInfoEntry& x) {
    out << "SetDirectoryInfoEntry(" << "DirId=" << x.dirId << ", " << "Info=" << x.info << ")";
    return out;
}

void RemoveNonOwnedEdgeEntry::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    targetId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
}
void RemoveNonOwnedEdgeEntry::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    targetId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
}
void RemoveNonOwnedEdgeEntry::clear() {
    dirId = InodeId();
    targetId = InodeId();
    name.clear();
    creationTime = TernTime();
}
bool RemoveNonOwnedEdgeEntry::operator==(const RemoveNonOwnedEdgeEntry& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveNonOwnedEdgeEntry& x) {
    out << "RemoveNonOwnedEdgeEntry(" << "DirId=" << x.dirId << ", " << "TargetId=" << x.targetId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void ScrapTransientFileEntry::pack(BincodeBuf& buf) const {
    id.pack(buf);
    deadlineTime.pack(buf);
}
void ScrapTransientFileEntry::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    deadlineTime.unpack(buf);
}
void ScrapTransientFileEntry::clear() {
    id = InodeId();
    deadlineTime = TernTime();
}
bool ScrapTransientFileEntry::operator==(const ScrapTransientFileEntry& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if ((TernTime)this->deadlineTime != (TernTime)rhs.deadlineTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const ScrapTransientFileEntry& x) {
    out << "ScrapTransientFileEntry(" << "Id=" << x.id << ", " << "DeadlineTime=" << x.deadlineTime << ")";
    return out;
}

void RemoveSpanInitiateEntry::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
}
void RemoveSpanInitiateEntry::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
}
void RemoveSpanInitiateEntry::clear() {
    fileId = InodeId();
}
bool RemoveSpanInitiateEntry::operator==(const RemoveSpanInitiateEntry& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveSpanInitiateEntry& x) {
    out << "RemoveSpanInitiateEntry(" << "FileId=" << x.fileId << ")";
    return out;
}

void AddSpanInitiateEntry::pack(BincodeBuf& buf) const {
    buf.packScalar<bool>(withReference);
    fileId.pack(buf);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packScalar<uint32_t>(size);
    crc.pack(buf);
    buf.packScalar<uint8_t>(storageClass);
    parity.pack(buf);
    buf.packScalar<uint8_t>(stripes);
    buf.packScalar<uint32_t>(cellSize);
    buf.packList<EntryNewBlockInfo>(bodyBlocks);
    buf.packList<Crc>(bodyStripes);
}
void AddSpanInitiateEntry::unpack(BincodeBuf& buf) {
    withReference = buf.unpackScalar<bool>();
    fileId.unpack(buf);
    byteOffset = buf.unpackScalar<uint64_t>();
    size = buf.unpackScalar<uint32_t>();
    crc.unpack(buf);
    storageClass = buf.unpackScalar<uint8_t>();
    parity.unpack(buf);
    stripes = buf.unpackScalar<uint8_t>();
    cellSize = buf.unpackScalar<uint32_t>();
    buf.unpackList<EntryNewBlockInfo>(bodyBlocks);
    buf.unpackList<Crc>(bodyStripes);
}
void AddSpanInitiateEntry::clear() {
    withReference = bool(0);
    fileId = InodeId();
    byteOffset = uint64_t(0);
    size = uint32_t(0);
    crc = Crc(0);
    storageClass = uint8_t(0);
    parity = Parity();
    stripes = uint8_t(0);
    cellSize = uint32_t(0);
    bodyBlocks.clear();
    bodyStripes.clear();
}
bool AddSpanInitiateEntry::operator==(const AddSpanInitiateEntry& rhs) const {
    if ((bool)this->withReference != (bool)rhs.withReference) { return false; };
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if ((uint32_t)this->size != (uint32_t)rhs.size) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    if ((Parity)this->parity != (Parity)rhs.parity) { return false; };
    if ((uint8_t)this->stripes != (uint8_t)rhs.stripes) { return false; };
    if ((uint32_t)this->cellSize != (uint32_t)rhs.cellSize) { return false; };
    if (bodyBlocks != rhs.bodyBlocks) { return false; };
    if (bodyStripes != rhs.bodyStripes) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanInitiateEntry& x) {
    out << "AddSpanInitiateEntry(" << "WithReference=" << x.withReference << ", " << "FileId=" << x.fileId << ", " << "ByteOffset=" << x.byteOffset << ", " << "Size=" << x.size << ", " << "Crc=" << x.crc << ", " << "StorageClass=" << (int)x.storageClass << ", " << "Parity=" << x.parity << ", " << "Stripes=" << (int)x.stripes << ", " << "CellSize=" << x.cellSize << ", " << "BodyBlocks=" << x.bodyBlocks << ", " << "BodyStripes=" << x.bodyStripes << ")";
    return out;
}

void AddSpanCertifyEntry::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packList<BlockProof>(proofs);
}
void AddSpanCertifyEntry::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    byteOffset = buf.unpackScalar<uint64_t>();
    buf.unpackList<BlockProof>(proofs);
}
void AddSpanCertifyEntry::clear() {
    fileId = InodeId();
    byteOffset = uint64_t(0);
    proofs.clear();
}
bool AddSpanCertifyEntry::operator==(const AddSpanCertifyEntry& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if (proofs != rhs.proofs) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanCertifyEntry& x) {
    out << "AddSpanCertifyEntry(" << "FileId=" << x.fileId << ", " << "ByteOffset=" << x.byteOffset << ", " << "Proofs=" << x.proofs << ")";
    return out;
}

void AddInlineSpanEntry::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packScalar<uint8_t>(storageClass);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packScalar<uint32_t>(size);
    crc.pack(buf);
    buf.packBytes(body);
}
void AddInlineSpanEntry::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    storageClass = buf.unpackScalar<uint8_t>();
    byteOffset = buf.unpackScalar<uint64_t>();
    size = buf.unpackScalar<uint32_t>();
    crc.unpack(buf);
    buf.unpackBytes(body);
}
void AddInlineSpanEntry::clear() {
    fileId = InodeId();
    storageClass = uint8_t(0);
    byteOffset = uint64_t(0);
    size = uint32_t(0);
    crc = Crc(0);
    body.clear();
}
bool AddInlineSpanEntry::operator==(const AddInlineSpanEntry& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if ((uint32_t)this->size != (uint32_t)rhs.size) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    if (body != rhs.body) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddInlineSpanEntry& x) {
    out << "AddInlineSpanEntry(" << "FileId=" << x.fileId << ", " << "StorageClass=" << (int)x.storageClass << ", " << "ByteOffset=" << x.byteOffset << ", " << "Size=" << x.size << ", " << "Crc=" << x.crc << ", " << "Body=" << x.body << ")";
    return out;
}

void MakeFileTransientDEPRECATEDEntry::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packBytes(note);
}
void MakeFileTransientDEPRECATEDEntry::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    buf.unpackBytes(note);
}
void MakeFileTransientDEPRECATEDEntry::clear() {
    id = InodeId();
    note.clear();
}
bool MakeFileTransientDEPRECATEDEntry::operator==(const MakeFileTransientDEPRECATEDEntry& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if (note != rhs.note) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const MakeFileTransientDEPRECATEDEntry& x) {
    out << "MakeFileTransientDEPRECATEDEntry(" << "Id=" << x.id << ", " << "Note=" << GoLangQuotedStringFmt(x.note.data(), x.note.size()) << ")";
    return out;
}

void RemoveSpanCertifyEntry::pack(BincodeBuf& buf) const {
    fileId.pack(buf);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packList<BlockProof>(proofs);
}
void RemoveSpanCertifyEntry::unpack(BincodeBuf& buf) {
    fileId.unpack(buf);
    byteOffset = buf.unpackScalar<uint64_t>();
    buf.unpackList<BlockProof>(proofs);
}
void RemoveSpanCertifyEntry::clear() {
    fileId = InodeId();
    byteOffset = uint64_t(0);
    proofs.clear();
}
bool RemoveSpanCertifyEntry::operator==(const RemoveSpanCertifyEntry& rhs) const {
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if (proofs != rhs.proofs) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveSpanCertifyEntry& x) {
    out << "RemoveSpanCertifyEntry(" << "FileId=" << x.fileId << ", " << "ByteOffset=" << x.byteOffset << ", " << "Proofs=" << x.proofs << ")";
    return out;
}

void RemoveOwnedSnapshotFileEdgeEntry::pack(BincodeBuf& buf) const {
    ownerId.pack(buf);
    targetId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
}
void RemoveOwnedSnapshotFileEdgeEntry::unpack(BincodeBuf& buf) {
    ownerId.unpack(buf);
    targetId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
}
void RemoveOwnedSnapshotFileEdgeEntry::clear() {
    ownerId = InodeId();
    targetId = InodeId();
    name.clear();
    creationTime = TernTime();
}
bool RemoveOwnedSnapshotFileEdgeEntry::operator==(const RemoveOwnedSnapshotFileEdgeEntry& rhs) const {
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveOwnedSnapshotFileEdgeEntry& x) {
    out << "RemoveOwnedSnapshotFileEdgeEntry(" << "OwnerId=" << x.ownerId << ", " << "TargetId=" << x.targetId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ")";
    return out;
}

void SwapBlocksEntry::pack(BincodeBuf& buf) const {
    fileId1.pack(buf);
    buf.packScalar<uint64_t>(byteOffset1);
    buf.packScalar<uint64_t>(blockId1);
    fileId2.pack(buf);
    buf.packScalar<uint64_t>(byteOffset2);
    buf.packScalar<uint64_t>(blockId2);
}
void SwapBlocksEntry::unpack(BincodeBuf& buf) {
    fileId1.unpack(buf);
    byteOffset1 = buf.unpackScalar<uint64_t>();
    blockId1 = buf.unpackScalar<uint64_t>();
    fileId2.unpack(buf);
    byteOffset2 = buf.unpackScalar<uint64_t>();
    blockId2 = buf.unpackScalar<uint64_t>();
}
void SwapBlocksEntry::clear() {
    fileId1 = InodeId();
    byteOffset1 = uint64_t(0);
    blockId1 = uint64_t(0);
    fileId2 = InodeId();
    byteOffset2 = uint64_t(0);
    blockId2 = uint64_t(0);
}
bool SwapBlocksEntry::operator==(const SwapBlocksEntry& rhs) const {
    if ((InodeId)this->fileId1 != (InodeId)rhs.fileId1) { return false; };
    if ((uint64_t)this->byteOffset1 != (uint64_t)rhs.byteOffset1) { return false; };
    if ((uint64_t)this->blockId1 != (uint64_t)rhs.blockId1) { return false; };
    if ((InodeId)this->fileId2 != (InodeId)rhs.fileId2) { return false; };
    if ((uint64_t)this->byteOffset2 != (uint64_t)rhs.byteOffset2) { return false; };
    if ((uint64_t)this->blockId2 != (uint64_t)rhs.blockId2) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SwapBlocksEntry& x) {
    out << "SwapBlocksEntry(" << "FileId1=" << x.fileId1 << ", " << "ByteOffset1=" << x.byteOffset1 << ", " << "BlockId1=" << x.blockId1 << ", " << "FileId2=" << x.fileId2 << ", " << "ByteOffset2=" << x.byteOffset2 << ", " << "BlockId2=" << x.blockId2 << ")";
    return out;
}

void MoveSpanEntry::pack(BincodeBuf& buf) const {
    buf.packScalar<uint32_t>(spanSize);
    fileId1.pack(buf);
    buf.packScalar<uint64_t>(byteOffset1);
    buf.packFixedBytes<8>(cookie1);
    fileId2.pack(buf);
    buf.packScalar<uint64_t>(byteOffset2);
    buf.packFixedBytes<8>(cookie2);
}
void MoveSpanEntry::unpack(BincodeBuf& buf) {
    spanSize = buf.unpackScalar<uint32_t>();
    fileId1.unpack(buf);
    byteOffset1 = buf.unpackScalar<uint64_t>();
    buf.unpackFixedBytes<8>(cookie1);
    fileId2.unpack(buf);
    byteOffset2 = buf.unpackScalar<uint64_t>();
    buf.unpackFixedBytes<8>(cookie2);
}
void MoveSpanEntry::clear() {
    spanSize = uint32_t(0);
    fileId1 = InodeId();
    byteOffset1 = uint64_t(0);
    cookie1.clear();
    fileId2 = InodeId();
    byteOffset2 = uint64_t(0);
    cookie2.clear();
}
bool MoveSpanEntry::operator==(const MoveSpanEntry& rhs) const {
    if ((uint32_t)this->spanSize != (uint32_t)rhs.spanSize) { return false; };
    if ((InodeId)this->fileId1 != (InodeId)rhs.fileId1) { return false; };
    if ((uint64_t)this->byteOffset1 != (uint64_t)rhs.byteOffset1) { return false; };
    if (cookie1 != rhs.cookie1) { return false; };
    if ((InodeId)this->fileId2 != (InodeId)rhs.fileId2) { return false; };
    if ((uint64_t)this->byteOffset2 != (uint64_t)rhs.byteOffset2) { return false; };
    if (cookie2 != rhs.cookie2) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const MoveSpanEntry& x) {
    out << "MoveSpanEntry(" << "SpanSize=" << x.spanSize << ", " << "FileId1=" << x.fileId1 << ", " << "ByteOffset1=" << x.byteOffset1 << ", " << "Cookie1=" << x.cookie1 << ", " << "FileId2=" << x.fileId2 << ", " << "ByteOffset2=" << x.byteOffset2 << ", " << "Cookie2=" << x.cookie2 << ")";
    return out;
}

void SetTimeEntry::pack(BincodeBuf& buf) const {
    id.pack(buf);
    buf.packScalar<uint64_t>(mtime);
    buf.packScalar<uint64_t>(atime);
}
void SetTimeEntry::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    mtime = buf.unpackScalar<uint64_t>();
    atime = buf.unpackScalar<uint64_t>();
}
void SetTimeEntry::clear() {
    id = InodeId();
    mtime = uint64_t(0);
    atime = uint64_t(0);
}
bool SetTimeEntry::operator==(const SetTimeEntry& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if ((uint64_t)this->mtime != (uint64_t)rhs.mtime) { return false; };
    if ((uint64_t)this->atime != (uint64_t)rhs.atime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SetTimeEntry& x) {
    out << "SetTimeEntry(" << "Id=" << x.id << ", " << "Mtime=" << x.mtime << ", " << "Atime=" << x.atime << ")";
    return out;
}

void RemoveZeroBlockServiceFilesEntry::pack(BincodeBuf& buf) const {
    startBlockService.pack(buf);
    startFile.pack(buf);
}
void RemoveZeroBlockServiceFilesEntry::unpack(BincodeBuf& buf) {
    startBlockService.unpack(buf);
    startFile.unpack(buf);
}
void RemoveZeroBlockServiceFilesEntry::clear() {
    startBlockService = BlockServiceId(0);
    startFile = InodeId();
}
bool RemoveZeroBlockServiceFilesEntry::operator==(const RemoveZeroBlockServiceFilesEntry& rhs) const {
    if ((BlockServiceId)this->startBlockService != (BlockServiceId)rhs.startBlockService) { return false; };
    if ((InodeId)this->startFile != (InodeId)rhs.startFile) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const RemoveZeroBlockServiceFilesEntry& x) {
    out << "RemoveZeroBlockServiceFilesEntry(" << "StartBlockService=" << x.startBlockService << ", " << "StartFile=" << x.startFile << ")";
    return out;
}

void SwapSpansEntry::pack(BincodeBuf& buf) const {
    fileId1.pack(buf);
    buf.packScalar<uint64_t>(byteOffset1);
    buf.packList<uint64_t>(blocks1);
    fileId2.pack(buf);
    buf.packScalar<uint64_t>(byteOffset2);
    buf.packList<uint64_t>(blocks2);
}
void SwapSpansEntry::unpack(BincodeBuf& buf) {
    fileId1.unpack(buf);
    byteOffset1 = buf.unpackScalar<uint64_t>();
    buf.unpackList<uint64_t>(blocks1);
    fileId2.unpack(buf);
    byteOffset2 = buf.unpackScalar<uint64_t>();
    buf.unpackList<uint64_t>(blocks2);
}
void SwapSpansEntry::clear() {
    fileId1 = InodeId();
    byteOffset1 = uint64_t(0);
    blocks1.clear();
    fileId2 = InodeId();
    byteOffset2 = uint64_t(0);
    blocks2.clear();
}
bool SwapSpansEntry::operator==(const SwapSpansEntry& rhs) const {
    if ((InodeId)this->fileId1 != (InodeId)rhs.fileId1) { return false; };
    if ((uint64_t)this->byteOffset1 != (uint64_t)rhs.byteOffset1) { return false; };
    if (blocks1 != rhs.blocks1) { return false; };
    if ((InodeId)this->fileId2 != (InodeId)rhs.fileId2) { return false; };
    if ((uint64_t)this->byteOffset2 != (uint64_t)rhs.byteOffset2) { return false; };
    if (blocks2 != rhs.blocks2) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SwapSpansEntry& x) {
    out << "SwapSpansEntry(" << "FileId1=" << x.fileId1 << ", " << "ByteOffset1=" << x.byteOffset1 << ", " << "Blocks1=" << x.blocks1 << ", " << "FileId2=" << x.fileId2 << ", " << "ByteOffset2=" << x.byteOffset2 << ", " << "Blocks2=" << x.blocks2 << ")";
    return out;
}

void SameDirectoryRenameSnapshotEntry::pack(BincodeBuf& buf) const {
    dirId.pack(buf);
    targetId.pack(buf);
    buf.packBytes(oldName);
    oldCreationTime.pack(buf);
    buf.packBytes(newName);
}
void SameDirectoryRenameSnapshotEntry::unpack(BincodeBuf& buf) {
    dirId.unpack(buf);
    targetId.unpack(buf);
    buf.unpackBytes(oldName);
    oldCreationTime.unpack(buf);
    buf.unpackBytes(newName);
}
void SameDirectoryRenameSnapshotEntry::clear() {
    dirId = InodeId();
    targetId = InodeId();
    oldName.clear();
    oldCreationTime = TernTime();
    newName.clear();
}
bool SameDirectoryRenameSnapshotEntry::operator==(const SameDirectoryRenameSnapshotEntry& rhs) const {
    if ((InodeId)this->dirId != (InodeId)rhs.dirId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if (oldName != rhs.oldName) { return false; };
    if ((TernTime)this->oldCreationTime != (TernTime)rhs.oldCreationTime) { return false; };
    if (newName != rhs.newName) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SameDirectoryRenameSnapshotEntry& x) {
    out << "SameDirectoryRenameSnapshotEntry(" << "DirId=" << x.dirId << ", " << "TargetId=" << x.targetId << ", " << "OldName=" << GoLangQuotedStringFmt(x.oldName.data(), x.oldName.size()) << ", " << "OldCreationTime=" << x.oldCreationTime << ", " << "NewName=" << GoLangQuotedStringFmt(x.newName.data(), x.newName.size()) << ")";
    return out;
}

void AddSpanAtLocationInitiateEntry::pack(BincodeBuf& buf) const {
    buf.packScalar<uint8_t>(locationId);
    buf.packScalar<bool>(withReference);
    fileId.pack(buf);
    buf.packScalar<uint64_t>(byteOffset);
    buf.packScalar<uint32_t>(size);
    crc.pack(buf);
    buf.packScalar<uint8_t>(storageClass);
    parity.pack(buf);
    buf.packScalar<uint8_t>(stripes);
    buf.packScalar<uint32_t>(cellSize);
    buf.packList<EntryNewBlockInfo>(bodyBlocks);
    buf.packList<Crc>(bodyStripes);
}
void AddSpanAtLocationInitiateEntry::unpack(BincodeBuf& buf) {
    locationId = buf.unpackScalar<uint8_t>();
    withReference = buf.unpackScalar<bool>();
    fileId.unpack(buf);
    byteOffset = buf.unpackScalar<uint64_t>();
    size = buf.unpackScalar<uint32_t>();
    crc.unpack(buf);
    storageClass = buf.unpackScalar<uint8_t>();
    parity.unpack(buf);
    stripes = buf.unpackScalar<uint8_t>();
    cellSize = buf.unpackScalar<uint32_t>();
    buf.unpackList<EntryNewBlockInfo>(bodyBlocks);
    buf.unpackList<Crc>(bodyStripes);
}
void AddSpanAtLocationInitiateEntry::clear() {
    locationId = uint8_t(0);
    withReference = bool(0);
    fileId = InodeId();
    byteOffset = uint64_t(0);
    size = uint32_t(0);
    crc = Crc(0);
    storageClass = uint8_t(0);
    parity = Parity();
    stripes = uint8_t(0);
    cellSize = uint32_t(0);
    bodyBlocks.clear();
    bodyStripes.clear();
}
bool AddSpanAtLocationInitiateEntry::operator==(const AddSpanAtLocationInitiateEntry& rhs) const {
    if ((uint8_t)this->locationId != (uint8_t)rhs.locationId) { return false; };
    if ((bool)this->withReference != (bool)rhs.withReference) { return false; };
    if ((InodeId)this->fileId != (InodeId)rhs.fileId) { return false; };
    if ((uint64_t)this->byteOffset != (uint64_t)rhs.byteOffset) { return false; };
    if ((uint32_t)this->size != (uint32_t)rhs.size) { return false; };
    if ((Crc)this->crc != (Crc)rhs.crc) { return false; };
    if ((uint8_t)this->storageClass != (uint8_t)rhs.storageClass) { return false; };
    if ((Parity)this->parity != (Parity)rhs.parity) { return false; };
    if ((uint8_t)this->stripes != (uint8_t)rhs.stripes) { return false; };
    if ((uint32_t)this->cellSize != (uint32_t)rhs.cellSize) { return false; };
    if (bodyBlocks != rhs.bodyBlocks) { return false; };
    if (bodyStripes != rhs.bodyStripes) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanAtLocationInitiateEntry& x) {
    out << "AddSpanAtLocationInitiateEntry(" << "LocationId=" << (int)x.locationId << ", " << "WithReference=" << x.withReference << ", " << "FileId=" << x.fileId << ", " << "ByteOffset=" << x.byteOffset << ", " << "Size=" << x.size << ", " << "Crc=" << x.crc << ", " << "StorageClass=" << (int)x.storageClass << ", " << "Parity=" << x.parity << ", " << "Stripes=" << (int)x.stripes << ", " << "CellSize=" << x.cellSize << ", " << "BodyBlocks=" << x.bodyBlocks << ", " << "BodyStripes=" << x.bodyStripes << ")";
    return out;
}

void AddSpanLocationEntry::pack(BincodeBuf& buf) const {
    fileId1.pack(buf);
    buf.packScalar<uint64_t>(byteOffset1);
    buf.packList<uint64_t>(blocks1);
    fileId2.pack(buf);
    buf.packScalar<uint64_t>(byteOffset2);
}
void AddSpanLocationEntry::unpack(BincodeBuf& buf) {
    fileId1.unpack(buf);
    byteOffset1 = buf.unpackScalar<uint64_t>();
    buf.unpackList<uint64_t>(blocks1);
    fileId2.unpack(buf);
    byteOffset2 = buf.unpackScalar<uint64_t>();
}
void AddSpanLocationEntry::clear() {
    fileId1 = InodeId();
    byteOffset1 = uint64_t(0);
    blocks1.clear();
    fileId2 = InodeId();
    byteOffset2 = uint64_t(0);
}
bool AddSpanLocationEntry::operator==(const AddSpanLocationEntry& rhs) const {
    if ((InodeId)this->fileId1 != (InodeId)rhs.fileId1) { return false; };
    if ((uint64_t)this->byteOffset1 != (uint64_t)rhs.byteOffset1) { return false; };
    if (blocks1 != rhs.blocks1) { return false; };
    if ((InodeId)this->fileId2 != (InodeId)rhs.fileId2) { return false; };
    if ((uint64_t)this->byteOffset2 != (uint64_t)rhs.byteOffset2) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const AddSpanLocationEntry& x) {
    out << "AddSpanLocationEntry(" << "FileId1=" << x.fileId1 << ", " << "ByteOffset1=" << x.byteOffset1 << ", " << "Blocks1=" << x.blocks1 << ", " << "FileId2=" << x.fileId2 << ", " << "ByteOffset2=" << x.byteOffset2 << ")";
    return out;
}

void SameShardHardFileUnlinkEntry::pack(BincodeBuf& buf) const {
    ownerId.pack(buf);
    targetId.pack(buf);
    buf.packBytes(name);
    creationTime.pack(buf);
    deadlineTime.pack(buf);
}
void SameShardHardFileUnlinkEntry::unpack(BincodeBuf& buf) {
    ownerId.unpack(buf);
    targetId.unpack(buf);
    buf.unpackBytes(name);
    creationTime.unpack(buf);
    deadlineTime.unpack(buf);
}
void SameShardHardFileUnlinkEntry::clear() {
    ownerId = InodeId();
    targetId = InodeId();
    name.clear();
    creationTime = TernTime();
    deadlineTime = TernTime();
}
bool SameShardHardFileUnlinkEntry::operator==(const SameShardHardFileUnlinkEntry& rhs) const {
    if ((InodeId)this->ownerId != (InodeId)rhs.ownerId) { return false; };
    if ((InodeId)this->targetId != (InodeId)rhs.targetId) { return false; };
    if (name != rhs.name) { return false; };
    if ((TernTime)this->creationTime != (TernTime)rhs.creationTime) { return false; };
    if ((TernTime)this->deadlineTime != (TernTime)rhs.deadlineTime) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const SameShardHardFileUnlinkEntry& x) {
    out << "SameShardHardFileUnlinkEntry(" << "OwnerId=" << x.ownerId << ", " << "TargetId=" << x.targetId << ", " << "Name=" << GoLangQuotedStringFmt(x.name.data(), x.name.size()) << ", " << "CreationTime=" << x.creationTime << ", " << "DeadlineTime=" << x.deadlineTime << ")";
    return out;
}

void MakeFileTransientEntry::pack(BincodeBuf& buf) const {
    id.pack(buf);
    deadlineTime.pack(buf);
    buf.packBytes(note);
}
void MakeFileTransientEntry::unpack(BincodeBuf& buf) {
    id.unpack(buf);
    deadlineTime.unpack(buf);
    buf.unpackBytes(note);
}
void MakeFileTransientEntry::clear() {
    id = InodeId();
    deadlineTime = TernTime();
    note.clear();
}
bool MakeFileTransientEntry::operator==(const MakeFileTransientEntry& rhs) const {
    if ((InodeId)this->id != (InodeId)rhs.id) { return false; };
    if ((TernTime)this->deadlineTime != (TernTime)rhs.deadlineTime) { return false; };
    if (note != rhs.note) { return false; };
    return true;
}
std::ostream& operator<<(std::ostream& out, const MakeFileTransientEntry& x) {
    out << "MakeFileTransientEntry(" << "Id=" << x.id << ", " << "DeadlineTime=" << x.deadlineTime << ", " << "Note=" << GoLangQuotedStringFmt(x.note.data(), x.note.size()) << ")";
    return out;
}

const ConstructFileEntry& ShardLogEntryContainer::getConstructFile() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::CONSTRUCT_FILE, "%s != %s", _kind, ShardLogEntryKind::CONSTRUCT_FILE);
    return std::get<0>(_data);
}
ConstructFileEntry& ShardLogEntryContainer::setConstructFile() {
    _kind = ShardLogEntryKind::CONSTRUCT_FILE;
    auto& x = _data.emplace<0>();
    return x;
}
const LinkFileEntry& ShardLogEntryContainer::getLinkFile() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::LINK_FILE, "%s != %s", _kind, ShardLogEntryKind::LINK_FILE);
    return std::get<1>(_data);
}
LinkFileEntry& ShardLogEntryContainer::setLinkFile() {
    _kind = ShardLogEntryKind::LINK_FILE;
    auto& x = _data.emplace<1>();
    return x;
}
const SameDirectoryRenameEntry& ShardLogEntryContainer::getSameDirectoryRename() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::SAME_DIRECTORY_RENAME, "%s != %s", _kind, ShardLogEntryKind::SAME_DIRECTORY_RENAME);
    return std::get<2>(_data);
}
SameDirectoryRenameEntry& ShardLogEntryContainer::setSameDirectoryRename() {
    _kind = ShardLogEntryKind::SAME_DIRECTORY_RENAME;
    auto& x = _data.emplace<2>();
    return x;
}
const SoftUnlinkFileEntry& ShardLogEntryContainer::getSoftUnlinkFile() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::SOFT_UNLINK_FILE, "%s != %s", _kind, ShardLogEntryKind::SOFT_UNLINK_FILE);
    return std::get<3>(_data);
}
SoftUnlinkFileEntry& ShardLogEntryContainer::setSoftUnlinkFile() {
    _kind = ShardLogEntryKind::SOFT_UNLINK_FILE;
    auto& x = _data.emplace<3>();
    return x;
}
const CreateDirectoryInodeEntry& ShardLogEntryContainer::getCreateDirectoryInode() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::CREATE_DIRECTORY_INODE, "%s != %s", _kind, ShardLogEntryKind::CREATE_DIRECTORY_INODE);
    return std::get<4>(_data);
}
CreateDirectoryInodeEntry& ShardLogEntryContainer::setCreateDirectoryInode() {
    _kind = ShardLogEntryKind::CREATE_DIRECTORY_INODE;
    auto& x = _data.emplace<4>();
    return x;
}
const CreateLockedCurrentEdgeEntry& ShardLogEntryContainer::getCreateLockedCurrentEdge() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::CREATE_LOCKED_CURRENT_EDGE, "%s != %s", _kind, ShardLogEntryKind::CREATE_LOCKED_CURRENT_EDGE);
    return std::get<5>(_data);
}
CreateLockedCurrentEdgeEntry& ShardLogEntryContainer::setCreateLockedCurrentEdge() {
    _kind = ShardLogEntryKind::CREATE_LOCKED_CURRENT_EDGE;
    auto& x = _data.emplace<5>();
    return x;
}
const UnlockCurrentEdgeEntry& ShardLogEntryContainer::getUnlockCurrentEdge() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::UNLOCK_CURRENT_EDGE, "%s != %s", _kind, ShardLogEntryKind::UNLOCK_CURRENT_EDGE);
    return std::get<6>(_data);
}
UnlockCurrentEdgeEntry& ShardLogEntryContainer::setUnlockCurrentEdge() {
    _kind = ShardLogEntryKind::UNLOCK_CURRENT_EDGE;
    auto& x = _data.emplace<6>();
    return x;
}
const LockCurrentEdgeEntry& ShardLogEntryContainer::getLockCurrentEdge() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::LOCK_CURRENT_EDGE, "%s != %s", _kind, ShardLogEntryKind::LOCK_CURRENT_EDGE);
    return std::get<7>(_data);
}
LockCurrentEdgeEntry& ShardLogEntryContainer::setLockCurrentEdge() {
    _kind = ShardLogEntryKind::LOCK_CURRENT_EDGE;
    auto& x = _data.emplace<7>();
    return x;
}
const RemoveDirectoryOwnerEntry& ShardLogEntryContainer::getRemoveDirectoryOwner() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::REMOVE_DIRECTORY_OWNER, "%s != %s", _kind, ShardLogEntryKind::REMOVE_DIRECTORY_OWNER);
    return std::get<8>(_data);
}
RemoveDirectoryOwnerEntry& ShardLogEntryContainer::setRemoveDirectoryOwner() {
    _kind = ShardLogEntryKind::REMOVE_DIRECTORY_OWNER;
    auto& x = _data.emplace<8>();
    return x;
}
const RemoveInodeEntry& ShardLogEntryContainer::getRemoveInode() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::REMOVE_INODE, "%s != %s", _kind, ShardLogEntryKind::REMOVE_INODE);
    return std::get<9>(_data);
}
RemoveInodeEntry& ShardLogEntryContainer::setRemoveInode() {
    _kind = ShardLogEntryKind::REMOVE_INODE;
    auto& x = _data.emplace<9>();
    return x;
}
const SetDirectoryOwnerEntry& ShardLogEntryContainer::getSetDirectoryOwner() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::SET_DIRECTORY_OWNER, "%s != %s", _kind, ShardLogEntryKind::SET_DIRECTORY_OWNER);
    return std::get<10>(_data);
}
SetDirectoryOwnerEntry& ShardLogEntryContainer::setSetDirectoryOwner() {
    _kind = ShardLogEntryKind::SET_DIRECTORY_OWNER;
    auto& x = _data.emplace<10>();
    return x;
}
const SetDirectoryInfoEntry& ShardLogEntryContainer::getSetDirectoryInfo() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::SET_DIRECTORY_INFO, "%s != %s", _kind, ShardLogEntryKind::SET_DIRECTORY_INFO);
    return std::get<11>(_data);
}
SetDirectoryInfoEntry& ShardLogEntryContainer::setSetDirectoryInfo() {
    _kind = ShardLogEntryKind::SET_DIRECTORY_INFO;
    auto& x = _data.emplace<11>();
    return x;
}
const RemoveNonOwnedEdgeEntry& ShardLogEntryContainer::getRemoveNonOwnedEdge() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::REMOVE_NON_OWNED_EDGE, "%s != %s", _kind, ShardLogEntryKind::REMOVE_NON_OWNED_EDGE);
    return std::get<12>(_data);
}
RemoveNonOwnedEdgeEntry& ShardLogEntryContainer::setRemoveNonOwnedEdge() {
    _kind = ShardLogEntryKind::REMOVE_NON_OWNED_EDGE;
    auto& x = _data.emplace<12>();
    return x;
}
const ScrapTransientFileEntry& ShardLogEntryContainer::getScrapTransientFile() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::SCRAP_TRANSIENT_FILE, "%s != %s", _kind, ShardLogEntryKind::SCRAP_TRANSIENT_FILE);
    return std::get<13>(_data);
}
ScrapTransientFileEntry& ShardLogEntryContainer::setScrapTransientFile() {
    _kind = ShardLogEntryKind::SCRAP_TRANSIENT_FILE;
    auto& x = _data.emplace<13>();
    return x;
}
const RemoveSpanInitiateEntry& ShardLogEntryContainer::getRemoveSpanInitiate() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::REMOVE_SPAN_INITIATE, "%s != %s", _kind, ShardLogEntryKind::REMOVE_SPAN_INITIATE);
    return std::get<14>(_data);
}
RemoveSpanInitiateEntry& ShardLogEntryContainer::setRemoveSpanInitiate() {
    _kind = ShardLogEntryKind::REMOVE_SPAN_INITIATE;
    auto& x = _data.emplace<14>();
    return x;
}
const AddSpanInitiateEntry& ShardLogEntryContainer::getAddSpanInitiate() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::ADD_SPAN_INITIATE, "%s != %s", _kind, ShardLogEntryKind::ADD_SPAN_INITIATE);
    return std::get<15>(_data);
}
AddSpanInitiateEntry& ShardLogEntryContainer::setAddSpanInitiate() {
    _kind = ShardLogEntryKind::ADD_SPAN_INITIATE;
    auto& x = _data.emplace<15>();
    return x;
}
const AddSpanCertifyEntry& ShardLogEntryContainer::getAddSpanCertify() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::ADD_SPAN_CERTIFY, "%s != %s", _kind, ShardLogEntryKind::ADD_SPAN_CERTIFY);
    return std::get<16>(_data);
}
AddSpanCertifyEntry& ShardLogEntryContainer::setAddSpanCertify() {
    _kind = ShardLogEntryKind::ADD_SPAN_CERTIFY;
    auto& x = _data.emplace<16>();
    return x;
}
const AddInlineSpanEntry& ShardLogEntryContainer::getAddInlineSpan() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::ADD_INLINE_SPAN, "%s != %s", _kind, ShardLogEntryKind::ADD_INLINE_SPAN);
    return std::get<17>(_data);
}
AddInlineSpanEntry& ShardLogEntryContainer::setAddInlineSpan() {
    _kind = ShardLogEntryKind::ADD_INLINE_SPAN;
    auto& x = _data.emplace<17>();
    return x;
}
const MakeFileTransientDEPRECATEDEntry& ShardLogEntryContainer::getMakeFileTransientDEPRECATED() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED, "%s != %s", _kind, ShardLogEntryKind::MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED);
    return std::get<18>(_data);
}
MakeFileTransientDEPRECATEDEntry& ShardLogEntryContainer::setMakeFileTransientDEPRECATED() {
    _kind = ShardLogEntryKind::MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED;
    auto& x = _data.emplace<18>();
    return x;
}
const RemoveSpanCertifyEntry& ShardLogEntryContainer::getRemoveSpanCertify() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::REMOVE_SPAN_CERTIFY, "%s != %s", _kind, ShardLogEntryKind::REMOVE_SPAN_CERTIFY);
    return std::get<19>(_data);
}
RemoveSpanCertifyEntry& ShardLogEntryContainer::setRemoveSpanCertify() {
    _kind = ShardLogEntryKind::REMOVE_SPAN_CERTIFY;
    auto& x = _data.emplace<19>();
    return x;
}
const RemoveOwnedSnapshotFileEdgeEntry& ShardLogEntryContainer::getRemoveOwnedSnapshotFileEdge() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE, "%s != %s", _kind, ShardLogEntryKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE);
    return std::get<20>(_data);
}
RemoveOwnedSnapshotFileEdgeEntry& ShardLogEntryContainer::setRemoveOwnedSnapshotFileEdge() {
    _kind = ShardLogEntryKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE;
    auto& x = _data.emplace<20>();
    return x;
}
const SwapBlocksEntry& ShardLogEntryContainer::getSwapBlocks() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::SWAP_BLOCKS, "%s != %s", _kind, ShardLogEntryKind::SWAP_BLOCKS);
    return std::get<21>(_data);
}
SwapBlocksEntry& ShardLogEntryContainer::setSwapBlocks() {
    _kind = ShardLogEntryKind::SWAP_BLOCKS;
    auto& x = _data.emplace<21>();
    return x;
}
const MoveSpanEntry& ShardLogEntryContainer::getMoveSpan() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::MOVE_SPAN, "%s != %s", _kind, ShardLogEntryKind::MOVE_SPAN);
    return std::get<22>(_data);
}
MoveSpanEntry& ShardLogEntryContainer::setMoveSpan() {
    _kind = ShardLogEntryKind::MOVE_SPAN;
    auto& x = _data.emplace<22>();
    return x;
}
const SetTimeEntry& ShardLogEntryContainer::getSetTime() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::SET_TIME, "%s != %s", _kind, ShardLogEntryKind::SET_TIME);
    return std::get<23>(_data);
}
SetTimeEntry& ShardLogEntryContainer::setSetTime() {
    _kind = ShardLogEntryKind::SET_TIME;
    auto& x = _data.emplace<23>();
    return x;
}
const RemoveZeroBlockServiceFilesEntry& ShardLogEntryContainer::getRemoveZeroBlockServiceFiles() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::REMOVE_ZERO_BLOCK_SERVICE_FILES, "%s != %s", _kind, ShardLogEntryKind::REMOVE_ZERO_BLOCK_SERVICE_FILES);
    return std::get<24>(_data);
}
RemoveZeroBlockServiceFilesEntry& ShardLogEntryContainer::setRemoveZeroBlockServiceFiles() {
    _kind = ShardLogEntryKind::REMOVE_ZERO_BLOCK_SERVICE_FILES;
    auto& x = _data.emplace<24>();
    return x;
}
const SwapSpansEntry& ShardLogEntryContainer::getSwapSpans() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::SWAP_SPANS, "%s != %s", _kind, ShardLogEntryKind::SWAP_SPANS);
    return std::get<25>(_data);
}
SwapSpansEntry& ShardLogEntryContainer::setSwapSpans() {
    _kind = ShardLogEntryKind::SWAP_SPANS;
    auto& x = _data.emplace<25>();
    return x;
}
const SameDirectoryRenameSnapshotEntry& ShardLogEntryContainer::getSameDirectoryRenameSnapshot() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::SAME_DIRECTORY_RENAME_SNAPSHOT, "%s != %s", _kind, ShardLogEntryKind::SAME_DIRECTORY_RENAME_SNAPSHOT);
    return std::get<26>(_data);
}
SameDirectoryRenameSnapshotEntry& ShardLogEntryContainer::setSameDirectoryRenameSnapshot() {
    _kind = ShardLogEntryKind::SAME_DIRECTORY_RENAME_SNAPSHOT;
    auto& x = _data.emplace<26>();
    return x;
}
const AddSpanAtLocationInitiateEntry& ShardLogEntryContainer::getAddSpanAtLocationInitiate() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::ADD_SPAN_AT_LOCATION_INITIATE, "%s != %s", _kind, ShardLogEntryKind::ADD_SPAN_AT_LOCATION_INITIATE);
    return std::get<27>(_data);
}
AddSpanAtLocationInitiateEntry& ShardLogEntryContainer::setAddSpanAtLocationInitiate() {
    _kind = ShardLogEntryKind::ADD_SPAN_AT_LOCATION_INITIATE;
    auto& x = _data.emplace<27>();
    return x;
}
const AddSpanLocationEntry& ShardLogEntryContainer::getAddSpanLocation() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::ADD_SPAN_LOCATION, "%s != %s", _kind, ShardLogEntryKind::ADD_SPAN_LOCATION);
    return std::get<28>(_data);
}
AddSpanLocationEntry& ShardLogEntryContainer::setAddSpanLocation() {
    _kind = ShardLogEntryKind::ADD_SPAN_LOCATION;
    auto& x = _data.emplace<28>();
    return x;
}
const SameShardHardFileUnlinkEntry& ShardLogEntryContainer::getSameShardHardFileUnlink() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::SAME_SHARD_HARD_FILE_UNLINK, "%s != %s", _kind, ShardLogEntryKind::SAME_SHARD_HARD_FILE_UNLINK);
    return std::get<29>(_data);
}
SameShardHardFileUnlinkEntry& ShardLogEntryContainer::setSameShardHardFileUnlink() {
    _kind = ShardLogEntryKind::SAME_SHARD_HARD_FILE_UNLINK;
    auto& x = _data.emplace<29>();
    return x;
}
const MakeFileTransientEntry& ShardLogEntryContainer::getMakeFileTransient() const {
    ALWAYS_ASSERT(_kind == ShardLogEntryKind::MAKE_FILE_TRANSIENT, "%s != %s", _kind, ShardLogEntryKind::MAKE_FILE_TRANSIENT);
    return std::get<30>(_data);
}
MakeFileTransientEntry& ShardLogEntryContainer::setMakeFileTransient() {
    _kind = ShardLogEntryKind::MAKE_FILE_TRANSIENT;
    auto& x = _data.emplace<30>();
    return x;
}
ShardLogEntryContainer::ShardLogEntryContainer() {
    clear();
}

ShardLogEntryContainer::ShardLogEntryContainer(const ShardLogEntryContainer& other) {
    *this = other;
}

ShardLogEntryContainer::ShardLogEntryContainer(ShardLogEntryContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = ShardLogEntryKind::EMPTY;
}

void ShardLogEntryContainer::operator=(const ShardLogEntryContainer& other) {
    if (other.kind() == ShardLogEntryKind::EMPTY) { clear(); return; }
    switch (other.kind()) {
    case ShardLogEntryKind::CONSTRUCT_FILE:
        setConstructFile() = other.getConstructFile();
        break;
    case ShardLogEntryKind::LINK_FILE:
        setLinkFile() = other.getLinkFile();
        break;
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME:
        setSameDirectoryRename() = other.getSameDirectoryRename();
        break;
    case ShardLogEntryKind::SOFT_UNLINK_FILE:
        setSoftUnlinkFile() = other.getSoftUnlinkFile();
        break;
    case ShardLogEntryKind::CREATE_DIRECTORY_INODE:
        setCreateDirectoryInode() = other.getCreateDirectoryInode();
        break;
    case ShardLogEntryKind::CREATE_LOCKED_CURRENT_EDGE:
        setCreateLockedCurrentEdge() = other.getCreateLockedCurrentEdge();
        break;
    case ShardLogEntryKind::UNLOCK_CURRENT_EDGE:
        setUnlockCurrentEdge() = other.getUnlockCurrentEdge();
        break;
    case ShardLogEntryKind::LOCK_CURRENT_EDGE:
        setLockCurrentEdge() = other.getLockCurrentEdge();
        break;
    case ShardLogEntryKind::REMOVE_DIRECTORY_OWNER:
        setRemoveDirectoryOwner() = other.getRemoveDirectoryOwner();
        break;
    case ShardLogEntryKind::REMOVE_INODE:
        setRemoveInode() = other.getRemoveInode();
        break;
    case ShardLogEntryKind::SET_DIRECTORY_OWNER:
        setSetDirectoryOwner() = other.getSetDirectoryOwner();
        break;
    case ShardLogEntryKind::SET_DIRECTORY_INFO:
        setSetDirectoryInfo() = other.getSetDirectoryInfo();
        break;
    case ShardLogEntryKind::REMOVE_NON_OWNED_EDGE:
        setRemoveNonOwnedEdge() = other.getRemoveNonOwnedEdge();
        break;
    case ShardLogEntryKind::SCRAP_TRANSIENT_FILE:
        setScrapTransientFile() = other.getScrapTransientFile();
        break;
    case ShardLogEntryKind::REMOVE_SPAN_INITIATE:
        setRemoveSpanInitiate() = other.getRemoveSpanInitiate();
        break;
    case ShardLogEntryKind::ADD_SPAN_INITIATE:
        setAddSpanInitiate() = other.getAddSpanInitiate();
        break;
    case ShardLogEntryKind::ADD_SPAN_CERTIFY:
        setAddSpanCertify() = other.getAddSpanCertify();
        break;
    case ShardLogEntryKind::ADD_INLINE_SPAN:
        setAddInlineSpan() = other.getAddInlineSpan();
        break;
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED:
        setMakeFileTransientDEPRECATED() = other.getMakeFileTransientDEPRECATED();
        break;
    case ShardLogEntryKind::REMOVE_SPAN_CERTIFY:
        setRemoveSpanCertify() = other.getRemoveSpanCertify();
        break;
    case ShardLogEntryKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        setRemoveOwnedSnapshotFileEdge() = other.getRemoveOwnedSnapshotFileEdge();
        break;
    case ShardLogEntryKind::SWAP_BLOCKS:
        setSwapBlocks() = other.getSwapBlocks();
        break;
    case ShardLogEntryKind::MOVE_SPAN:
        setMoveSpan() = other.getMoveSpan();
        break;
    case ShardLogEntryKind::SET_TIME:
        setSetTime() = other.getSetTime();
        break;
    case ShardLogEntryKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        setRemoveZeroBlockServiceFiles() = other.getRemoveZeroBlockServiceFiles();
        break;
    case ShardLogEntryKind::SWAP_SPANS:
        setSwapSpans() = other.getSwapSpans();
        break;
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        setSameDirectoryRenameSnapshot() = other.getSameDirectoryRenameSnapshot();
        break;
    case ShardLogEntryKind::ADD_SPAN_AT_LOCATION_INITIATE:
        setAddSpanAtLocationInitiate() = other.getAddSpanAtLocationInitiate();
        break;
    case ShardLogEntryKind::ADD_SPAN_LOCATION:
        setAddSpanLocation() = other.getAddSpanLocation();
        break;
    case ShardLogEntryKind::SAME_SHARD_HARD_FILE_UNLINK:
        setSameShardHardFileUnlink() = other.getSameShardHardFileUnlink();
        break;
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT:
        setMakeFileTransient() = other.getMakeFileTransient();
        break;
    default:
        throw TERN_EXCEPTION("bad ShardLogEntryKind kind %s", other.kind());
    }
}

void ShardLogEntryContainer::operator=(ShardLogEntryContainer&& other) {
    _data = std::move(other._data);
    _kind = other._kind;
    other._kind = ShardLogEntryKind::EMPTY;
}

size_t ShardLogEntryContainer::packedSize() const {
    switch (_kind) {
    case ShardLogEntryKind::CONSTRUCT_FILE:
        return sizeof(ShardLogEntryKind) + std::get<0>(_data).packedSize();
    case ShardLogEntryKind::LINK_FILE:
        return sizeof(ShardLogEntryKind) + std::get<1>(_data).packedSize();
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME:
        return sizeof(ShardLogEntryKind) + std::get<2>(_data).packedSize();
    case ShardLogEntryKind::SOFT_UNLINK_FILE:
        return sizeof(ShardLogEntryKind) + std::get<3>(_data).packedSize();
    case ShardLogEntryKind::CREATE_DIRECTORY_INODE:
        return sizeof(ShardLogEntryKind) + std::get<4>(_data).packedSize();
    case ShardLogEntryKind::CREATE_LOCKED_CURRENT_EDGE:
        return sizeof(ShardLogEntryKind) + std::get<5>(_data).packedSize();
    case ShardLogEntryKind::UNLOCK_CURRENT_EDGE:
        return sizeof(ShardLogEntryKind) + std::get<6>(_data).packedSize();
    case ShardLogEntryKind::LOCK_CURRENT_EDGE:
        return sizeof(ShardLogEntryKind) + std::get<7>(_data).packedSize();
    case ShardLogEntryKind::REMOVE_DIRECTORY_OWNER:
        return sizeof(ShardLogEntryKind) + std::get<8>(_data).packedSize();
    case ShardLogEntryKind::REMOVE_INODE:
        return sizeof(ShardLogEntryKind) + std::get<9>(_data).packedSize();
    case ShardLogEntryKind::SET_DIRECTORY_OWNER:
        return sizeof(ShardLogEntryKind) + std::get<10>(_data).packedSize();
    case ShardLogEntryKind::SET_DIRECTORY_INFO:
        return sizeof(ShardLogEntryKind) + std::get<11>(_data).packedSize();
    case ShardLogEntryKind::REMOVE_NON_OWNED_EDGE:
        return sizeof(ShardLogEntryKind) + std::get<12>(_data).packedSize();
    case ShardLogEntryKind::SCRAP_TRANSIENT_FILE:
        return sizeof(ShardLogEntryKind) + std::get<13>(_data).packedSize();
    case ShardLogEntryKind::REMOVE_SPAN_INITIATE:
        return sizeof(ShardLogEntryKind) + std::get<14>(_data).packedSize();
    case ShardLogEntryKind::ADD_SPAN_INITIATE:
        return sizeof(ShardLogEntryKind) + std::get<15>(_data).packedSize();
    case ShardLogEntryKind::ADD_SPAN_CERTIFY:
        return sizeof(ShardLogEntryKind) + std::get<16>(_data).packedSize();
    case ShardLogEntryKind::ADD_INLINE_SPAN:
        return sizeof(ShardLogEntryKind) + std::get<17>(_data).packedSize();
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED:
        return sizeof(ShardLogEntryKind) + std::get<18>(_data).packedSize();
    case ShardLogEntryKind::REMOVE_SPAN_CERTIFY:
        return sizeof(ShardLogEntryKind) + std::get<19>(_data).packedSize();
    case ShardLogEntryKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        return sizeof(ShardLogEntryKind) + std::get<20>(_data).packedSize();
    case ShardLogEntryKind::SWAP_BLOCKS:
        return sizeof(ShardLogEntryKind) + std::get<21>(_data).packedSize();
    case ShardLogEntryKind::MOVE_SPAN:
        return sizeof(ShardLogEntryKind) + std::get<22>(_data).packedSize();
    case ShardLogEntryKind::SET_TIME:
        return sizeof(ShardLogEntryKind) + std::get<23>(_data).packedSize();
    case ShardLogEntryKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        return sizeof(ShardLogEntryKind) + std::get<24>(_data).packedSize();
    case ShardLogEntryKind::SWAP_SPANS:
        return sizeof(ShardLogEntryKind) + std::get<25>(_data).packedSize();
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        return sizeof(ShardLogEntryKind) + std::get<26>(_data).packedSize();
    case ShardLogEntryKind::ADD_SPAN_AT_LOCATION_INITIATE:
        return sizeof(ShardLogEntryKind) + std::get<27>(_data).packedSize();
    case ShardLogEntryKind::ADD_SPAN_LOCATION:
        return sizeof(ShardLogEntryKind) + std::get<28>(_data).packedSize();
    case ShardLogEntryKind::SAME_SHARD_HARD_FILE_UNLINK:
        return sizeof(ShardLogEntryKind) + std::get<29>(_data).packedSize();
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT:
        return sizeof(ShardLogEntryKind) + std::get<30>(_data).packedSize();
    default:
        throw TERN_EXCEPTION("bad ShardLogEntryKind kind %s", _kind);
    }
}

void ShardLogEntryContainer::pack(BincodeBuf& buf) const {
    buf.packScalar<ShardLogEntryKind>(_kind);
    switch (_kind) {
    case ShardLogEntryKind::CONSTRUCT_FILE:
        std::get<0>(_data).pack(buf);
        break;
    case ShardLogEntryKind::LINK_FILE:
        std::get<1>(_data).pack(buf);
        break;
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME:
        std::get<2>(_data).pack(buf);
        break;
    case ShardLogEntryKind::SOFT_UNLINK_FILE:
        std::get<3>(_data).pack(buf);
        break;
    case ShardLogEntryKind::CREATE_DIRECTORY_INODE:
        std::get<4>(_data).pack(buf);
        break;
    case ShardLogEntryKind::CREATE_LOCKED_CURRENT_EDGE:
        std::get<5>(_data).pack(buf);
        break;
    case ShardLogEntryKind::UNLOCK_CURRENT_EDGE:
        std::get<6>(_data).pack(buf);
        break;
    case ShardLogEntryKind::LOCK_CURRENT_EDGE:
        std::get<7>(_data).pack(buf);
        break;
    case ShardLogEntryKind::REMOVE_DIRECTORY_OWNER:
        std::get<8>(_data).pack(buf);
        break;
    case ShardLogEntryKind::REMOVE_INODE:
        std::get<9>(_data).pack(buf);
        break;
    case ShardLogEntryKind::SET_DIRECTORY_OWNER:
        std::get<10>(_data).pack(buf);
        break;
    case ShardLogEntryKind::SET_DIRECTORY_INFO:
        std::get<11>(_data).pack(buf);
        break;
    case ShardLogEntryKind::REMOVE_NON_OWNED_EDGE:
        std::get<12>(_data).pack(buf);
        break;
    case ShardLogEntryKind::SCRAP_TRANSIENT_FILE:
        std::get<13>(_data).pack(buf);
        break;
    case ShardLogEntryKind::REMOVE_SPAN_INITIATE:
        std::get<14>(_data).pack(buf);
        break;
    case ShardLogEntryKind::ADD_SPAN_INITIATE:
        std::get<15>(_data).pack(buf);
        break;
    case ShardLogEntryKind::ADD_SPAN_CERTIFY:
        std::get<16>(_data).pack(buf);
        break;
    case ShardLogEntryKind::ADD_INLINE_SPAN:
        std::get<17>(_data).pack(buf);
        break;
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED:
        std::get<18>(_data).pack(buf);
        break;
    case ShardLogEntryKind::REMOVE_SPAN_CERTIFY:
        std::get<19>(_data).pack(buf);
        break;
    case ShardLogEntryKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        std::get<20>(_data).pack(buf);
        break;
    case ShardLogEntryKind::SWAP_BLOCKS:
        std::get<21>(_data).pack(buf);
        break;
    case ShardLogEntryKind::MOVE_SPAN:
        std::get<22>(_data).pack(buf);
        break;
    case ShardLogEntryKind::SET_TIME:
        std::get<23>(_data).pack(buf);
        break;
    case ShardLogEntryKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        std::get<24>(_data).pack(buf);
        break;
    case ShardLogEntryKind::SWAP_SPANS:
        std::get<25>(_data).pack(buf);
        break;
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        std::get<26>(_data).pack(buf);
        break;
    case ShardLogEntryKind::ADD_SPAN_AT_LOCATION_INITIATE:
        std::get<27>(_data).pack(buf);
        break;
    case ShardLogEntryKind::ADD_SPAN_LOCATION:
        std::get<28>(_data).pack(buf);
        break;
    case ShardLogEntryKind::SAME_SHARD_HARD_FILE_UNLINK:
        std::get<29>(_data).pack(buf);
        break;
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT:
        std::get<30>(_data).pack(buf);
        break;
    default:
        throw TERN_EXCEPTION("bad ShardLogEntryKind kind %s", _kind);
    }
}

void ShardLogEntryContainer::unpack(BincodeBuf& buf) {
    _kind = buf.unpackScalar<ShardLogEntryKind>();
    switch (_kind) {
    case ShardLogEntryKind::CONSTRUCT_FILE:
        _data.emplace<0>().unpack(buf);
        break;
    case ShardLogEntryKind::LINK_FILE:
        _data.emplace<1>().unpack(buf);
        break;
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME:
        _data.emplace<2>().unpack(buf);
        break;
    case ShardLogEntryKind::SOFT_UNLINK_FILE:
        _data.emplace<3>().unpack(buf);
        break;
    case ShardLogEntryKind::CREATE_DIRECTORY_INODE:
        _data.emplace<4>().unpack(buf);
        break;
    case ShardLogEntryKind::CREATE_LOCKED_CURRENT_EDGE:
        _data.emplace<5>().unpack(buf);
        break;
    case ShardLogEntryKind::UNLOCK_CURRENT_EDGE:
        _data.emplace<6>().unpack(buf);
        break;
    case ShardLogEntryKind::LOCK_CURRENT_EDGE:
        _data.emplace<7>().unpack(buf);
        break;
    case ShardLogEntryKind::REMOVE_DIRECTORY_OWNER:
        _data.emplace<8>().unpack(buf);
        break;
    case ShardLogEntryKind::REMOVE_INODE:
        _data.emplace<9>().unpack(buf);
        break;
    case ShardLogEntryKind::SET_DIRECTORY_OWNER:
        _data.emplace<10>().unpack(buf);
        break;
    case ShardLogEntryKind::SET_DIRECTORY_INFO:
        _data.emplace<11>().unpack(buf);
        break;
    case ShardLogEntryKind::REMOVE_NON_OWNED_EDGE:
        _data.emplace<12>().unpack(buf);
        break;
    case ShardLogEntryKind::SCRAP_TRANSIENT_FILE:
        _data.emplace<13>().unpack(buf);
        break;
    case ShardLogEntryKind::REMOVE_SPAN_INITIATE:
        _data.emplace<14>().unpack(buf);
        break;
    case ShardLogEntryKind::ADD_SPAN_INITIATE:
        _data.emplace<15>().unpack(buf);
        break;
    case ShardLogEntryKind::ADD_SPAN_CERTIFY:
        _data.emplace<16>().unpack(buf);
        break;
    case ShardLogEntryKind::ADD_INLINE_SPAN:
        _data.emplace<17>().unpack(buf);
        break;
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED:
        _data.emplace<18>().unpack(buf);
        break;
    case ShardLogEntryKind::REMOVE_SPAN_CERTIFY:
        _data.emplace<19>().unpack(buf);
        break;
    case ShardLogEntryKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        _data.emplace<20>().unpack(buf);
        break;
    case ShardLogEntryKind::SWAP_BLOCKS:
        _data.emplace<21>().unpack(buf);
        break;
    case ShardLogEntryKind::MOVE_SPAN:
        _data.emplace<22>().unpack(buf);
        break;
    case ShardLogEntryKind::SET_TIME:
        _data.emplace<23>().unpack(buf);
        break;
    case ShardLogEntryKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        _data.emplace<24>().unpack(buf);
        break;
    case ShardLogEntryKind::SWAP_SPANS:
        _data.emplace<25>().unpack(buf);
        break;
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        _data.emplace<26>().unpack(buf);
        break;
    case ShardLogEntryKind::ADD_SPAN_AT_LOCATION_INITIATE:
        _data.emplace<27>().unpack(buf);
        break;
    case ShardLogEntryKind::ADD_SPAN_LOCATION:
        _data.emplace<28>().unpack(buf);
        break;
    case ShardLogEntryKind::SAME_SHARD_HARD_FILE_UNLINK:
        _data.emplace<29>().unpack(buf);
        break;
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT:
        _data.emplace<30>().unpack(buf);
        break;
    default:
        throw BINCODE_EXCEPTION("bad ShardLogEntryKind kind %s", _kind);
    }
}

bool ShardLogEntryContainer::operator==(const ShardLogEntryContainer& other) const {
    if (_kind != other.kind()) { return false; }
    if (_kind == ShardLogEntryKind::EMPTY) { return true; }
    switch (_kind) {
    case ShardLogEntryKind::CONSTRUCT_FILE:
        return getConstructFile() == other.getConstructFile();
    case ShardLogEntryKind::LINK_FILE:
        return getLinkFile() == other.getLinkFile();
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME:
        return getSameDirectoryRename() == other.getSameDirectoryRename();
    case ShardLogEntryKind::SOFT_UNLINK_FILE:
        return getSoftUnlinkFile() == other.getSoftUnlinkFile();
    case ShardLogEntryKind::CREATE_DIRECTORY_INODE:
        return getCreateDirectoryInode() == other.getCreateDirectoryInode();
    case ShardLogEntryKind::CREATE_LOCKED_CURRENT_EDGE:
        return getCreateLockedCurrentEdge() == other.getCreateLockedCurrentEdge();
    case ShardLogEntryKind::UNLOCK_CURRENT_EDGE:
        return getUnlockCurrentEdge() == other.getUnlockCurrentEdge();
    case ShardLogEntryKind::LOCK_CURRENT_EDGE:
        return getLockCurrentEdge() == other.getLockCurrentEdge();
    case ShardLogEntryKind::REMOVE_DIRECTORY_OWNER:
        return getRemoveDirectoryOwner() == other.getRemoveDirectoryOwner();
    case ShardLogEntryKind::REMOVE_INODE:
        return getRemoveInode() == other.getRemoveInode();
    case ShardLogEntryKind::SET_DIRECTORY_OWNER:
        return getSetDirectoryOwner() == other.getSetDirectoryOwner();
    case ShardLogEntryKind::SET_DIRECTORY_INFO:
        return getSetDirectoryInfo() == other.getSetDirectoryInfo();
    case ShardLogEntryKind::REMOVE_NON_OWNED_EDGE:
        return getRemoveNonOwnedEdge() == other.getRemoveNonOwnedEdge();
    case ShardLogEntryKind::SCRAP_TRANSIENT_FILE:
        return getScrapTransientFile() == other.getScrapTransientFile();
    case ShardLogEntryKind::REMOVE_SPAN_INITIATE:
        return getRemoveSpanInitiate() == other.getRemoveSpanInitiate();
    case ShardLogEntryKind::ADD_SPAN_INITIATE:
        return getAddSpanInitiate() == other.getAddSpanInitiate();
    case ShardLogEntryKind::ADD_SPAN_CERTIFY:
        return getAddSpanCertify() == other.getAddSpanCertify();
    case ShardLogEntryKind::ADD_INLINE_SPAN:
        return getAddInlineSpan() == other.getAddInlineSpan();
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED:
        return getMakeFileTransientDEPRECATED() == other.getMakeFileTransientDEPRECATED();
    case ShardLogEntryKind::REMOVE_SPAN_CERTIFY:
        return getRemoveSpanCertify() == other.getRemoveSpanCertify();
    case ShardLogEntryKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        return getRemoveOwnedSnapshotFileEdge() == other.getRemoveOwnedSnapshotFileEdge();
    case ShardLogEntryKind::SWAP_BLOCKS:
        return getSwapBlocks() == other.getSwapBlocks();
    case ShardLogEntryKind::MOVE_SPAN:
        return getMoveSpan() == other.getMoveSpan();
    case ShardLogEntryKind::SET_TIME:
        return getSetTime() == other.getSetTime();
    case ShardLogEntryKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        return getRemoveZeroBlockServiceFiles() == other.getRemoveZeroBlockServiceFiles();
    case ShardLogEntryKind::SWAP_SPANS:
        return getSwapSpans() == other.getSwapSpans();
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        return getSameDirectoryRenameSnapshot() == other.getSameDirectoryRenameSnapshot();
    case ShardLogEntryKind::ADD_SPAN_AT_LOCATION_INITIATE:
        return getAddSpanAtLocationInitiate() == other.getAddSpanAtLocationInitiate();
    case ShardLogEntryKind::ADD_SPAN_LOCATION:
        return getAddSpanLocation() == other.getAddSpanLocation();
    case ShardLogEntryKind::SAME_SHARD_HARD_FILE_UNLINK:
        return getSameShardHardFileUnlink() == other.getSameShardHardFileUnlink();
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT:
        return getMakeFileTransient() == other.getMakeFileTransient();
    default:
        throw BINCODE_EXCEPTION("bad ShardLogEntryKind kind %s", _kind);
    }
}

std::ostream& operator<<(std::ostream& out, const ShardLogEntryContainer& x) {
    switch (x.kind()) {
    case ShardLogEntryKind::CONSTRUCT_FILE:
        out << x.getConstructFile();
        break;
    case ShardLogEntryKind::LINK_FILE:
        out << x.getLinkFile();
        break;
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME:
        out << x.getSameDirectoryRename();
        break;
    case ShardLogEntryKind::SOFT_UNLINK_FILE:
        out << x.getSoftUnlinkFile();
        break;
    case ShardLogEntryKind::CREATE_DIRECTORY_INODE:
        out << x.getCreateDirectoryInode();
        break;
    case ShardLogEntryKind::CREATE_LOCKED_CURRENT_EDGE:
        out << x.getCreateLockedCurrentEdge();
        break;
    case ShardLogEntryKind::UNLOCK_CURRENT_EDGE:
        out << x.getUnlockCurrentEdge();
        break;
    case ShardLogEntryKind::LOCK_CURRENT_EDGE:
        out << x.getLockCurrentEdge();
        break;
    case ShardLogEntryKind::REMOVE_DIRECTORY_OWNER:
        out << x.getRemoveDirectoryOwner();
        break;
    case ShardLogEntryKind::REMOVE_INODE:
        out << x.getRemoveInode();
        break;
    case ShardLogEntryKind::SET_DIRECTORY_OWNER:
        out << x.getSetDirectoryOwner();
        break;
    case ShardLogEntryKind::SET_DIRECTORY_INFO:
        out << x.getSetDirectoryInfo();
        break;
    case ShardLogEntryKind::REMOVE_NON_OWNED_EDGE:
        out << x.getRemoveNonOwnedEdge();
        break;
    case ShardLogEntryKind::SCRAP_TRANSIENT_FILE:
        out << x.getScrapTransientFile();
        break;
    case ShardLogEntryKind::REMOVE_SPAN_INITIATE:
        out << x.getRemoveSpanInitiate();
        break;
    case ShardLogEntryKind::ADD_SPAN_INITIATE:
        out << x.getAddSpanInitiate();
        break;
    case ShardLogEntryKind::ADD_SPAN_CERTIFY:
        out << x.getAddSpanCertify();
        break;
    case ShardLogEntryKind::ADD_INLINE_SPAN:
        out << x.getAddInlineSpan();
        break;
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT_DE_PR_EC_AT_ED:
        out << x.getMakeFileTransientDEPRECATED();
        break;
    case ShardLogEntryKind::REMOVE_SPAN_CERTIFY:
        out << x.getRemoveSpanCertify();
        break;
    case ShardLogEntryKind::REMOVE_OWNED_SNAPSHOT_FILE_EDGE:
        out << x.getRemoveOwnedSnapshotFileEdge();
        break;
    case ShardLogEntryKind::SWAP_BLOCKS:
        out << x.getSwapBlocks();
        break;
    case ShardLogEntryKind::MOVE_SPAN:
        out << x.getMoveSpan();
        break;
    case ShardLogEntryKind::SET_TIME:
        out << x.getSetTime();
        break;
    case ShardLogEntryKind::REMOVE_ZERO_BLOCK_SERVICE_FILES:
        out << x.getRemoveZeroBlockServiceFiles();
        break;
    case ShardLogEntryKind::SWAP_SPANS:
        out << x.getSwapSpans();
        break;
    case ShardLogEntryKind::SAME_DIRECTORY_RENAME_SNAPSHOT:
        out << x.getSameDirectoryRenameSnapshot();
        break;
    case ShardLogEntryKind::ADD_SPAN_AT_LOCATION_INITIATE:
        out << x.getAddSpanAtLocationInitiate();
        break;
    case ShardLogEntryKind::ADD_SPAN_LOCATION:
        out << x.getAddSpanLocation();
        break;
    case ShardLogEntryKind::SAME_SHARD_HARD_FILE_UNLINK:
        out << x.getSameShardHardFileUnlink();
        break;
    case ShardLogEntryKind::MAKE_FILE_TRANSIENT:
        out << x.getMakeFileTransient();
        break;
    case ShardLogEntryKind::EMPTY:
        out << "EMPTY";
        break;
    default:
        throw TERN_EXCEPTION("bad ShardLogEntryKind kind %s", x.kind());
    }
    return out;
}

