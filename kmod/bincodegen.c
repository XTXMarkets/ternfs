// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// Automatically generated with go run bincodegen.
// Run `go generate ./...` from the go/ directory to regenerate it.

const char* ternfs_err_str(int err) {
    switch (err) {
    case 10: return "INTERNAL_ERROR";
    case 11: return "FATAL_ERROR";
    case 12: return "TIMEOUT";
    case 13: return "MALFORMED_REQUEST";
    case 14: return "MALFORMED_RESPONSE";
    case 15: return "NOT_AUTHORISED";
    case 16: return "UNRECOGNIZED_REQUEST";
    case 17: return "FILE_NOT_FOUND";
    case 18: return "DIRECTORY_NOT_FOUND";
    case 19: return "NAME_NOT_FOUND";
    case 20: return "EDGE_NOT_FOUND";
    case 21: return "EDGE_IS_LOCKED";
    case 22: return "TYPE_IS_DIRECTORY";
    case 23: return "TYPE_IS_NOT_DIRECTORY";
    case 24: return "BAD_COOKIE";
    case 25: return "INCONSISTENT_STORAGE_CLASS_PARITY";
    case 26: return "LAST_SPAN_STATE_NOT_CLEAN";
    case 27: return "COULD_NOT_PICK_BLOCK_SERVICES";
    case 28: return "BAD_SPAN_BODY";
    case 29: return "SPAN_NOT_FOUND";
    case 30: return "BLOCK_SERVICE_NOT_FOUND";
    case 31: return "CANNOT_CERTIFY_BLOCKLESS_SPAN";
    case 32: return "BAD_NUMBER_OF_BLOCKS_PROOFS";
    case 33: return "BAD_BLOCK_PROOF";
    case 34: return "CANNOT_OVERRIDE_NAME";
    case 35: return "NAME_IS_LOCKED";
    case 36: return "MTIME_IS_TOO_RECENT";
    case 37: return "MISMATCHING_TARGET";
    case 38: return "MISMATCHING_OWNER";
    case 39: return "MISMATCHING_CREATION_TIME";
    case 40: return "DIRECTORY_NOT_EMPTY";
    case 41: return "FILE_IS_TRANSIENT";
    case 42: return "OLD_DIRECTORY_NOT_FOUND";
    case 43: return "NEW_DIRECTORY_NOT_FOUND";
    case 44: return "LOOP_IN_DIRECTORY_RENAME";
    case 45: return "DIRECTORY_HAS_OWNER";
    case 46: return "FILE_IS_NOT_TRANSIENT";
    case 47: return "FILE_NOT_EMPTY";
    case 48: return "CANNOT_REMOVE_ROOT_DIRECTORY";
    case 49: return "FILE_EMPTY";
    case 50: return "CANNOT_REMOVE_DIRTY_SPAN";
    case 51: return "BAD_SHARD";
    case 52: return "BAD_NAME";
    case 53: return "MORE_RECENT_SNAPSHOT_EDGE";
    case 54: return "MORE_RECENT_CURRENT_EDGE";
    case 55: return "BAD_DIRECTORY_INFO";
    case 56: return "DEADLINE_NOT_PASSED";
    case 57: return "SAME_SOURCE_AND_DESTINATION";
    case 58: return "SAME_DIRECTORIES";
    case 59: return "SAME_SHARD";
    case 60: return "BAD_PROTOCOL_VERSION";
    case 61: return "BAD_CERTIFICATE";
    case 62: return "BLOCK_TOO_RECENT_FOR_DELETION";
    case 63: return "BLOCK_FETCH_OUT_OF_BOUNDS";
    case 64: return "BAD_BLOCK_CRC";
    case 65: return "BLOCK_TOO_BIG";
    case 66: return "BLOCK_NOT_FOUND";
    case 67: return "CANNOT_UNSET_DECOMMISSIONED";
    case 68: return "CANNOT_REGISTER_DECOMMISSIONED_OR_STALE";
    case 69: return "BLOCK_TOO_OLD_FOR_WRITE";
    case 70: return "BLOCK_IO_ERROR_DEVICE";
    case 71: return "BLOCK_IO_ERROR_FILE";
    case 72: return "INVALID_REPLICA";
    case 73: return "DIFFERENT_ADDRS_INFO";
    case 74: return "LEADER_PREEMPTED";
    case 75: return "LOG_ENTRY_MISSING";
    case 76: return "LOG_ENTRY_TRIMMED";
    case 77: return "LOG_ENTRY_UNRELEASED";
    case 78: return "LOG_ENTRY_RELEASED";
    case 79: return "AUTO_DECOMMISSION_FORBIDDEN";
    case 80: return "INCONSISTENT_BLOCK_SERVICE_REGISTRATION";
    case 81: return "SWAP_BLOCKS_INLINE_STORAGE";
    case 82: return "SWAP_BLOCKS_MISMATCHING_SIZE";
    case 83: return "SWAP_BLOCKS_MISMATCHING_STATE";
    case 84: return "SWAP_BLOCKS_MISMATCHING_CRC";
    case 85: return "SWAP_BLOCKS_DUPLICATE_BLOCK_SERVICE";
    case 86: return "SWAP_SPANS_INLINE_STORAGE";
    case 87: return "SWAP_SPANS_MISMATCHING_SIZE";
    case 88: return "SWAP_SPANS_NOT_CLEAN";
    case 89: return "SWAP_SPANS_MISMATCHING_CRC";
    case 90: return "SWAP_SPANS_MISMATCHING_BLOCKS";
    case 91: return "EDGE_NOT_OWNED";
    case 92: return "CANNOT_CREATE_DB_SNAPSHOT";
    case 93: return "BLOCK_SIZE_NOT_MULTIPLE_OF_PAGE_SIZE";
    case 94: return "SWAP_BLOCKS_DUPLICATE_FAILURE_DOMAIN";
    case 95: return "TRANSIENT_LOCATION_COUNT";
    case 96: return "ADD_SPAN_LOCATION_INLINE_STORAGE";
    case 97: return "ADD_SPAN_LOCATION_MISMATCHING_SIZE";
    case 98: return "ADD_SPAN_LOCATION_NOT_CLEAN";
    case 99: return "ADD_SPAN_LOCATION_MISMATCHING_CRC";
    case 100: return "ADD_SPAN_LOCATION_EXISTS";
    case 101: return "SWAP_BLOCKS_MISMATCHING_LOCATION";
    case 102: return "LOCATION_EXISTS";
    case 103: return "LOCATION_NOT_FOUND";
    default: return "UNKNOWN";
    }
}

const char* ternfs_shard_kind_str(int kind) {
    switch (kind) {
    case 1: return "LOOKUP";
    case 2: return "STAT_FILE";
    case 4: return "STAT_DIRECTORY";
    case 5: return "READ_DIR";
    case 6: return "CONSTRUCT_FILE";
    case 7: return "ADD_SPAN_INITIATE";
    case 8: return "ADD_SPAN_CERTIFY";
    case 9: return "LINK_FILE";
    case 10: return "SOFT_UNLINK_FILE";
    case 11: return "LOCAL_FILE_SPANS";
    case 12: return "SAME_DIRECTORY_RENAME";
    case 16: return "ADD_INLINE_SPAN";
    case 17: return "SET_TIME";
    case 115: return "FULL_READ_DIR";
    case 123: return "MOVE_SPAN";
    case 116: return "REMOVE_NON_OWNED_EDGE";
    case 117: return "SAME_SHARD_HARD_FILE_UNLINK";
    default: return "UNKNOWN";
    }
}

const char* ternfs_cdc_kind_str(int kind) {
    switch (kind) {
    case 1: return "MAKE_DIRECTORY";
    case 2: return "RENAME_FILE";
    case 3: return "SOFT_UNLINK_DIRECTORY";
    case 4: return "RENAME_DIRECTORY";
    case 6: return "CROSS_SHARD_HARD_UNLINK_FILE";
    default: return "UNKNOWN";
    }
}

const char* ternfs_registry_kind_str(int kind) {
    switch (kind) {
    case 3: return "LOCAL_SHARDS";
    case 7: return "LOCAL_CDC";
    case 8: return "INFO";
    case 15: return "REGISTRY";
    case 34: return "LOCAL_CHANGED_BLOCK_SERVICES";
    default: return "UNKNOWN";
    }
}

const char* ternfs_blocks_kind_str(int kind) {
    switch (kind) {
    case 2: return "FETCH_BLOCK";
    case 3: return "WRITE_BLOCK";
    case 4: return "FETCH_BLOCK_WITH_CRC";
    default: return "UNKNOWN";
    }
}

