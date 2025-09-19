// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// Automatically generated with go run bincodegen.
// Run `go generate ./...` from the go/ directory to regenerate it.

#define TERNFS_ERR_INTERNAL_ERROR 10
#define TERNFS_ERR_FATAL_ERROR 11
#define TERNFS_ERR_TIMEOUT 12
#define TERNFS_ERR_MALFORMED_REQUEST 13
#define TERNFS_ERR_MALFORMED_RESPONSE 14
#define TERNFS_ERR_NOT_AUTHORISED 15
#define TERNFS_ERR_UNRECOGNIZED_REQUEST 16
#define TERNFS_ERR_FILE_NOT_FOUND 17
#define TERNFS_ERR_DIRECTORY_NOT_FOUND 18
#define TERNFS_ERR_NAME_NOT_FOUND 19
#define TERNFS_ERR_EDGE_NOT_FOUND 20
#define TERNFS_ERR_EDGE_IS_LOCKED 21
#define TERNFS_ERR_TYPE_IS_DIRECTORY 22
#define TERNFS_ERR_TYPE_IS_NOT_DIRECTORY 23
#define TERNFS_ERR_BAD_COOKIE 24
#define TERNFS_ERR_INCONSISTENT_STORAGE_CLASS_PARITY 25
#define TERNFS_ERR_LAST_SPAN_STATE_NOT_CLEAN 26
#define TERNFS_ERR_COULD_NOT_PICK_BLOCK_SERVICES 27
#define TERNFS_ERR_BAD_SPAN_BODY 28
#define TERNFS_ERR_SPAN_NOT_FOUND 29
#define TERNFS_ERR_BLOCK_SERVICE_NOT_FOUND 30
#define TERNFS_ERR_CANNOT_CERTIFY_BLOCKLESS_SPAN 31
#define TERNFS_ERR_BAD_NUMBER_OF_BLOCKS_PROOFS 32
#define TERNFS_ERR_BAD_BLOCK_PROOF 33
#define TERNFS_ERR_CANNOT_OVERRIDE_NAME 34
#define TERNFS_ERR_NAME_IS_LOCKED 35
#define TERNFS_ERR_MTIME_IS_TOO_RECENT 36
#define TERNFS_ERR_MISMATCHING_TARGET 37
#define TERNFS_ERR_MISMATCHING_OWNER 38
#define TERNFS_ERR_MISMATCHING_CREATION_TIME 39
#define TERNFS_ERR_DIRECTORY_NOT_EMPTY 40
#define TERNFS_ERR_FILE_IS_TRANSIENT 41
#define TERNFS_ERR_OLD_DIRECTORY_NOT_FOUND 42
#define TERNFS_ERR_NEW_DIRECTORY_NOT_FOUND 43
#define TERNFS_ERR_LOOP_IN_DIRECTORY_RENAME 44
#define TERNFS_ERR_DIRECTORY_HAS_OWNER 45
#define TERNFS_ERR_FILE_IS_NOT_TRANSIENT 46
#define TERNFS_ERR_FILE_NOT_EMPTY 47
#define TERNFS_ERR_CANNOT_REMOVE_ROOT_DIRECTORY 48
#define TERNFS_ERR_FILE_EMPTY 49
#define TERNFS_ERR_CANNOT_REMOVE_DIRTY_SPAN 50
#define TERNFS_ERR_BAD_SHARD 51
#define TERNFS_ERR_BAD_NAME 52
#define TERNFS_ERR_MORE_RECENT_SNAPSHOT_EDGE 53
#define TERNFS_ERR_MORE_RECENT_CURRENT_EDGE 54
#define TERNFS_ERR_BAD_DIRECTORY_INFO 55
#define TERNFS_ERR_DEADLINE_NOT_PASSED 56
#define TERNFS_ERR_SAME_SOURCE_AND_DESTINATION 57
#define TERNFS_ERR_SAME_DIRECTORIES 58
#define TERNFS_ERR_SAME_SHARD 59
#define TERNFS_ERR_BAD_PROTOCOL_VERSION 60
#define TERNFS_ERR_BAD_CERTIFICATE 61
#define TERNFS_ERR_BLOCK_TOO_RECENT_FOR_DELETION 62
#define TERNFS_ERR_BLOCK_FETCH_OUT_OF_BOUNDS 63
#define TERNFS_ERR_BAD_BLOCK_CRC 64
#define TERNFS_ERR_BLOCK_TOO_BIG 65
#define TERNFS_ERR_BLOCK_NOT_FOUND 66
#define TERNFS_ERR_CANNOT_UNSET_DECOMMISSIONED 67
#define TERNFS_ERR_CANNOT_REGISTER_DECOMMISSIONED_OR_STALE 68
#define TERNFS_ERR_BLOCK_TOO_OLD_FOR_WRITE 69
#define TERNFS_ERR_BLOCK_IO_ERROR_DEVICE 70
#define TERNFS_ERR_BLOCK_IO_ERROR_FILE 71
#define TERNFS_ERR_INVALID_REPLICA 72
#define TERNFS_ERR_DIFFERENT_ADDRS_INFO 73
#define TERNFS_ERR_LEADER_PREEMPTED 74
#define TERNFS_ERR_LOG_ENTRY_MISSING 75
#define TERNFS_ERR_LOG_ENTRY_TRIMMED 76
#define TERNFS_ERR_LOG_ENTRY_UNRELEASED 77
#define TERNFS_ERR_LOG_ENTRY_RELEASED 78
#define TERNFS_ERR_AUTO_DECOMMISSION_FORBIDDEN 79
#define TERNFS_ERR_INCONSISTENT_BLOCK_SERVICE_REGISTRATION 80
#define TERNFS_ERR_SWAP_BLOCKS_INLINE_STORAGE 81
#define TERNFS_ERR_SWAP_BLOCKS_MISMATCHING_SIZE 82
#define TERNFS_ERR_SWAP_BLOCKS_MISMATCHING_STATE 83
#define TERNFS_ERR_SWAP_BLOCKS_MISMATCHING_CRC 84
#define TERNFS_ERR_SWAP_BLOCKS_DUPLICATE_BLOCK_SERVICE 85
#define TERNFS_ERR_SWAP_SPANS_INLINE_STORAGE 86
#define TERNFS_ERR_SWAP_SPANS_MISMATCHING_SIZE 87
#define TERNFS_ERR_SWAP_SPANS_NOT_CLEAN 88
#define TERNFS_ERR_SWAP_SPANS_MISMATCHING_CRC 89
#define TERNFS_ERR_SWAP_SPANS_MISMATCHING_BLOCKS 90
#define TERNFS_ERR_EDGE_NOT_OWNED 91
#define TERNFS_ERR_CANNOT_CREATE_DB_SNAPSHOT 92
#define TERNFS_ERR_BLOCK_SIZE_NOT_MULTIPLE_OF_PAGE_SIZE 93
#define TERNFS_ERR_SWAP_BLOCKS_DUPLICATE_FAILURE_DOMAIN 94
#define TERNFS_ERR_TRANSIENT_LOCATION_COUNT 95
#define TERNFS_ERR_ADD_SPAN_LOCATION_INLINE_STORAGE 96
#define TERNFS_ERR_ADD_SPAN_LOCATION_MISMATCHING_SIZE 97
#define TERNFS_ERR_ADD_SPAN_LOCATION_NOT_CLEAN 98
#define TERNFS_ERR_ADD_SPAN_LOCATION_MISMATCHING_CRC 99
#define TERNFS_ERR_ADD_SPAN_LOCATION_EXISTS 100
#define TERNFS_ERR_SWAP_BLOCKS_MISMATCHING_LOCATION 101
#define TERNFS_ERR_LOCATION_EXISTS 102
#define TERNFS_ERR_LOCATION_NOT_FOUND 103

#define __print_ternfs_err(i) __print_symbolic(i, { 10, "INTERNAL_ERROR" }, { 11, "FATAL_ERROR" }, { 12, "TIMEOUT" }, { 13, "MALFORMED_REQUEST" }, { 14, "MALFORMED_RESPONSE" }, { 15, "NOT_AUTHORISED" }, { 16, "UNRECOGNIZED_REQUEST" }, { 17, "FILE_NOT_FOUND" }, { 18, "DIRECTORY_NOT_FOUND" }, { 19, "NAME_NOT_FOUND" }, { 20, "EDGE_NOT_FOUND" }, { 21, "EDGE_IS_LOCKED" }, { 22, "TYPE_IS_DIRECTORY" }, { 23, "TYPE_IS_NOT_DIRECTORY" }, { 24, "BAD_COOKIE" }, { 25, "INCONSISTENT_STORAGE_CLASS_PARITY" }, { 26, "LAST_SPAN_STATE_NOT_CLEAN" }, { 27, "COULD_NOT_PICK_BLOCK_SERVICES" }, { 28, "BAD_SPAN_BODY" }, { 29, "SPAN_NOT_FOUND" }, { 30, "BLOCK_SERVICE_NOT_FOUND" }, { 31, "CANNOT_CERTIFY_BLOCKLESS_SPAN" }, { 32, "BAD_NUMBER_OF_BLOCKS_PROOFS" }, { 33, "BAD_BLOCK_PROOF" }, { 34, "CANNOT_OVERRIDE_NAME" }, { 35, "NAME_IS_LOCKED" }, { 36, "MTIME_IS_TOO_RECENT" }, { 37, "MISMATCHING_TARGET" }, { 38, "MISMATCHING_OWNER" }, { 39, "MISMATCHING_CREATION_TIME" }, { 40, "DIRECTORY_NOT_EMPTY" }, { 41, "FILE_IS_TRANSIENT" }, { 42, "OLD_DIRECTORY_NOT_FOUND" }, { 43, "NEW_DIRECTORY_NOT_FOUND" }, { 44, "LOOP_IN_DIRECTORY_RENAME" }, { 45, "DIRECTORY_HAS_OWNER" }, { 46, "FILE_IS_NOT_TRANSIENT" }, { 47, "FILE_NOT_EMPTY" }, { 48, "CANNOT_REMOVE_ROOT_DIRECTORY" }, { 49, "FILE_EMPTY" }, { 50, "CANNOT_REMOVE_DIRTY_SPAN" }, { 51, "BAD_SHARD" }, { 52, "BAD_NAME" }, { 53, "MORE_RECENT_SNAPSHOT_EDGE" }, { 54, "MORE_RECENT_CURRENT_EDGE" }, { 55, "BAD_DIRECTORY_INFO" }, { 56, "DEADLINE_NOT_PASSED" }, { 57, "SAME_SOURCE_AND_DESTINATION" }, { 58, "SAME_DIRECTORIES" }, { 59, "SAME_SHARD" }, { 60, "BAD_PROTOCOL_VERSION" }, { 61, "BAD_CERTIFICATE" }, { 62, "BLOCK_TOO_RECENT_FOR_DELETION" }, { 63, "BLOCK_FETCH_OUT_OF_BOUNDS" }, { 64, "BAD_BLOCK_CRC" }, { 65, "BLOCK_TOO_BIG" }, { 66, "BLOCK_NOT_FOUND" }, { 67, "CANNOT_UNSET_DECOMMISSIONED" }, { 68, "CANNOT_REGISTER_DECOMMISSIONED_OR_STALE" }, { 69, "BLOCK_TOO_OLD_FOR_WRITE" }, { 70, "BLOCK_IO_ERROR_DEVICE" }, { 71, "BLOCK_IO_ERROR_FILE" }, { 72, "INVALID_REPLICA" }, { 73, "DIFFERENT_ADDRS_INFO" }, { 74, "LEADER_PREEMPTED" }, { 75, "LOG_ENTRY_MISSING" }, { 76, "LOG_ENTRY_TRIMMED" }, { 77, "LOG_ENTRY_UNRELEASED" }, { 78, "LOG_ENTRY_RELEASED" }, { 79, "AUTO_DECOMMISSION_FORBIDDEN" }, { 80, "INCONSISTENT_BLOCK_SERVICE_REGISTRATION" }, { 81, "SWAP_BLOCKS_INLINE_STORAGE" }, { 82, "SWAP_BLOCKS_MISMATCHING_SIZE" }, { 83, "SWAP_BLOCKS_MISMATCHING_STATE" }, { 84, "SWAP_BLOCKS_MISMATCHING_CRC" }, { 85, "SWAP_BLOCKS_DUPLICATE_BLOCK_SERVICE" }, { 86, "SWAP_SPANS_INLINE_STORAGE" }, { 87, "SWAP_SPANS_MISMATCHING_SIZE" }, { 88, "SWAP_SPANS_NOT_CLEAN" }, { 89, "SWAP_SPANS_MISMATCHING_CRC" }, { 90, "SWAP_SPANS_MISMATCHING_BLOCKS" }, { 91, "EDGE_NOT_OWNED" }, { 92, "CANNOT_CREATE_DB_SNAPSHOT" }, { 93, "BLOCK_SIZE_NOT_MULTIPLE_OF_PAGE_SIZE" }, { 94, "SWAP_BLOCKS_DUPLICATE_FAILURE_DOMAIN" }, { 95, "TRANSIENT_LOCATION_COUNT" }, { 96, "ADD_SPAN_LOCATION_INLINE_STORAGE" }, { 97, "ADD_SPAN_LOCATION_MISMATCHING_SIZE" }, { 98, "ADD_SPAN_LOCATION_NOT_CLEAN" }, { 99, "ADD_SPAN_LOCATION_MISMATCHING_CRC" }, { 100, "ADD_SPAN_LOCATION_EXISTS" }, { 101, "SWAP_BLOCKS_MISMATCHING_LOCATION" }, { 102, "LOCATION_EXISTS" }, { 103, "LOCATION_NOT_FOUND" })
const char* ternfs_err_str(int err);

#define TERNFS_SHARD_LOOKUP 0x1
#define TERNFS_SHARD_STAT_FILE 0x2
#define TERNFS_SHARD_STAT_DIRECTORY 0x4
#define TERNFS_SHARD_READ_DIR 0x5
#define TERNFS_SHARD_CONSTRUCT_FILE 0x6
#define TERNFS_SHARD_ADD_SPAN_INITIATE 0x7
#define TERNFS_SHARD_ADD_SPAN_CERTIFY 0x8
#define TERNFS_SHARD_LINK_FILE 0x9
#define TERNFS_SHARD_SOFT_UNLINK_FILE 0xA
#define TERNFS_SHARD_LOCAL_FILE_SPANS 0xB
#define TERNFS_SHARD_SAME_DIRECTORY_RENAME 0xC
#define TERNFS_SHARD_ADD_INLINE_SPAN 0x10
#define TERNFS_SHARD_SET_TIME 0x11
#define TERNFS_SHARD_FULL_READ_DIR 0x73
#define TERNFS_SHARD_MOVE_SPAN 0x7B
#define TERNFS_SHARD_REMOVE_NON_OWNED_EDGE 0x74
#define TERNFS_SHARD_SAME_SHARD_HARD_FILE_UNLINK 0x75
#define __print_ternfs_shard_kind(k) __print_symbolic(k, { 1, "LOOKUP" }, { 2, "STAT_FILE" }, { 4, "STAT_DIRECTORY" }, { 5, "READ_DIR" }, { 6, "CONSTRUCT_FILE" }, { 7, "ADD_SPAN_INITIATE" }, { 8, "ADD_SPAN_CERTIFY" }, { 9, "LINK_FILE" }, { 10, "SOFT_UNLINK_FILE" }, { 11, "LOCAL_FILE_SPANS" }, { 12, "SAME_DIRECTORY_RENAME" }, { 16, "ADD_INLINE_SPAN" }, { 17, "SET_TIME" }, { 115, "FULL_READ_DIR" }, { 123, "MOVE_SPAN" }, { 116, "REMOVE_NON_OWNED_EDGE" }, { 117, "SAME_SHARD_HARD_FILE_UNLINK" })
#define TERNFS_SHARD_KIND_MAX 17
static const u8 __ternfs_shard_kind_index_mappings[256] = {255, 0, 1, 255, 2, 3, 4, 5, 6, 7, 8, 9, 10, 255, 255, 255, 11, 12, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 13, 15, 16, 255, 255, 255, 255, 255, 14, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255};
const char* ternfs_shard_kind_str(int kind);

#define TERNFS_CDC_MAKE_DIRECTORY 0x1
#define TERNFS_CDC_RENAME_FILE 0x2
#define TERNFS_CDC_SOFT_UNLINK_DIRECTORY 0x3
#define TERNFS_CDC_RENAME_DIRECTORY 0x4
#define __print_ternfs_cdc_kind(k) __print_symbolic(k, { 1, "MAKE_DIRECTORY" }, { 2, "RENAME_FILE" }, { 3, "SOFT_UNLINK_DIRECTORY" }, { 4, "RENAME_DIRECTORY" })
#define TERNFS_CDC_KIND_MAX 4
static const u8 __ternfs_cdc_kind_index_mappings[256] = {255, 0, 1, 2, 3, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255};
const char* ternfs_cdc_kind_str(int kind);

#define TERNFS_REGISTRY_LOCAL_SHARDS 0x3
#define TERNFS_REGISTRY_LOCAL_CDC 0x7
#define TERNFS_REGISTRY_INFO 0x8
#define TERNFS_REGISTRY_REGISTRY 0xF
#define TERNFS_REGISTRY_LOCAL_CHANGED_BLOCK_SERVICES 0x22
#define __print_ternfs_registry_kind(k) __print_symbolic(k, { 3, "LOCAL_SHARDS" }, { 7, "LOCAL_CDC" }, { 8, "INFO" }, { 15, "REGISTRY" }, { 34, "LOCAL_CHANGED_BLOCK_SERVICES" })
#define TERNFS_REGISTRY_KIND_MAX 5
static const u8 __ternfs_registry_kind_index_mappings[256] = {255, 255, 255, 0, 255, 255, 255, 1, 2, 255, 255, 255, 255, 255, 255, 3, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 4, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255};
const char* ternfs_registry_kind_str(int kind);

#define TERNFS_BLOCKS_FETCH_BLOCK 0x2
#define TERNFS_BLOCKS_WRITE_BLOCK 0x3
#define TERNFS_BLOCKS_FETCH_BLOCK_WITH_CRC 0x4
#define __print_ternfs_blocks_kind(k) __print_symbolic(k, { 2, "FETCH_BLOCK" }, { 3, "WRITE_BLOCK" }, { 4, "FETCH_BLOCK_WITH_CRC" })
#define TERNFS_BLOCKS_KIND_MAX 3
static const u8 __ternfs_blocks_kind_index_mappings[256] = {255, 255, 0, 1, 2, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255};
const char* ternfs_blocks_kind_str(int kind);


#define TERNFS_IP_PORT_SIZE 6
struct ternfs_ip_port_start;
#define ternfs_ip_port_get_start(ctx, start) struct ternfs_ip_port_start* start = NULL

struct ternfs_ip_port_addrs { u32 x; };
static inline void _ternfs_ip_port_get_addrs(struct ternfs_bincode_get_ctx* ctx, struct ternfs_ip_port_start** prev, struct ternfs_ip_port_addrs* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_ip_port_get_addrs(ctx, prev, next) \
    struct ternfs_ip_port_addrs next; \
    _ternfs_ip_port_get_addrs(ctx, &(prev), &(next))

struct ternfs_ip_port_port { u16 x; };
static inline void _ternfs_ip_port_get_port(struct ternfs_bincode_get_ctx* ctx, struct ternfs_ip_port_addrs* prev, struct ternfs_ip_port_port* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    }
}
#define ternfs_ip_port_get_port(ctx, prev, next) \
    struct ternfs_ip_port_port next; \
    _ternfs_ip_port_get_port(ctx, &(prev), &(next))

struct ternfs_ip_port_end;
#define ternfs_ip_port_get_end(ctx, prev, next) \
    { struct ternfs_ip_port_port* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_ip_port_end* next = NULL

static inline void ternfs_ip_port_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_ip_port_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_ip_port_put_start(ctx, start) struct ternfs_ip_port_start* start = NULL

static inline void _ternfs_ip_port_put_addrs(struct ternfs_bincode_put_ctx* ctx, struct ternfs_ip_port_start** prev, struct ternfs_ip_port_addrs* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_be32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_ip_port_put_addrs(ctx, prev, next, x) \
    struct ternfs_ip_port_addrs next; \
    _ternfs_ip_port_put_addrs(ctx, &(prev), &(next), x)

static inline void _ternfs_ip_port_put_port(struct ternfs_bincode_put_ctx* ctx, struct ternfs_ip_port_addrs* prev, struct ternfs_ip_port_port* next, u16 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(x, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_ip_port_put_port(ctx, prev, next, x) \
    struct ternfs_ip_port_port next; \
    _ternfs_ip_port_put_port(ctx, &(prev), &(next), x)

#define ternfs_ip_port_put_end(ctx, prev, next) \
    { struct ternfs_ip_port_port* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_ip_port_end* next __attribute__((unused)) = NULL

#define TERNFS_ADDRS_INFO_SIZE 12
struct ternfs_addrs_info_start;
#define ternfs_addrs_info_get_start(ctx, start) struct ternfs_addrs_info_start* start = NULL

#define ternfs_addrs_info_get_addr1(ctx, prev, next) \
    { struct ternfs_addrs_info_start** __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_ip_port_start* next = NULL

#define ternfs_addrs_info_get_addr2(ctx, prev, next) \
    { struct ternfs_ip_port_end** __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_ip_port_start* next = NULL

struct ternfs_addrs_info_end;
#define ternfs_addrs_info_get_end(ctx, prev, next) \
    { struct ternfs_ip_port_end** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_addrs_info_end* next = NULL

static inline void ternfs_addrs_info_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_addrs_info_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_addrs_info_put_start(ctx, start) struct ternfs_addrs_info_start* start = NULL

#define ternfs_addrs_info_put_end(ctx, prev, next) \
    { struct ternfs_addrs_info_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_addrs_info_end* next __attribute__((unused)) = NULL

#define TERNFS_DIRECTORY_INFO_ENTRY_MAX_SIZE 257
struct ternfs_directory_info_entry_start;
#define ternfs_directory_info_entry_get_start(ctx, start) struct ternfs_directory_info_entry_start* start = NULL

struct ternfs_directory_info_entry_tag { u8 x; };
static inline void _ternfs_directory_info_entry_get_tag(struct ternfs_bincode_get_ctx* ctx, struct ternfs_directory_info_entry_start** prev, struct ternfs_directory_info_entry_tag* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_directory_info_entry_get_tag(ctx, prev, next) \
    struct ternfs_directory_info_entry_tag next; \
    _ternfs_directory_info_entry_get_tag(ctx, &(prev), &(next))

struct ternfs_directory_info_entry_body { struct ternfs_bincode_bytes str; };
static inline void _ternfs_directory_info_entry_get_body(struct ternfs_bincode_get_ctx* ctx, struct ternfs_directory_info_entry_tag* prev, struct ternfs_directory_info_entry_body* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_directory_info_entry_get_body(ctx, prev, next) \
    struct ternfs_directory_info_entry_body next; \
    _ternfs_directory_info_entry_get_body(ctx, &(prev), &(next))

struct ternfs_directory_info_entry_end;
#define ternfs_directory_info_entry_get_end(ctx, prev, next) \
    { struct ternfs_directory_info_entry_body* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_directory_info_entry_end* next = NULL

static inline void ternfs_directory_info_entry_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_directory_info_entry_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_directory_info_entry_put_start(ctx, start) struct ternfs_directory_info_entry_start* start = NULL

static inline void _ternfs_directory_info_entry_put_tag(struct ternfs_bincode_put_ctx* ctx, struct ternfs_directory_info_entry_start** prev, struct ternfs_directory_info_entry_tag* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_directory_info_entry_put_tag(ctx, prev, next, x) \
    struct ternfs_directory_info_entry_tag next; \
    _ternfs_directory_info_entry_put_tag(ctx, &(prev), &(next), x)

static inline void _ternfs_directory_info_entry_put_body(struct ternfs_bincode_put_ctx* ctx, struct ternfs_directory_info_entry_tag* prev, struct ternfs_directory_info_entry_body* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_directory_info_entry_put_body(ctx, prev, next, str, str_len) \
    struct ternfs_directory_info_entry_body next; \
    _ternfs_directory_info_entry_put_body(ctx, &(prev), &(next), str, str_len)

#define ternfs_directory_info_entry_put_end(ctx, prev, next) \
    { struct ternfs_directory_info_entry_body* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_directory_info_entry_end* next __attribute__((unused)) = NULL

struct ternfs_directory_info_start;
#define ternfs_directory_info_get_start(ctx, start) struct ternfs_directory_info_start* start = NULL

struct ternfs_directory_info_entries { u16 len; };
static inline void _ternfs_directory_info_get_entries(struct ternfs_bincode_get_ctx* ctx, struct ternfs_directory_info_start** prev, struct ternfs_directory_info_entries* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_directory_info_get_entries(ctx, prev, next) \
    struct ternfs_directory_info_entries next; \
    _ternfs_directory_info_get_entries(ctx, &(prev), &(next))

struct ternfs_directory_info_end;
#define ternfs_directory_info_get_end(ctx, prev, next) \
    { struct ternfs_directory_info_entries* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_directory_info_end* next = NULL

static inline void ternfs_directory_info_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_directory_info_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_directory_info_put_start(ctx, start) struct ternfs_directory_info_start* start = NULL

static inline void _ternfs_directory_info_put_entries(struct ternfs_bincode_put_ctx* ctx, struct ternfs_directory_info_start** prev, struct ternfs_directory_info_entries* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_directory_info_put_entries(ctx, prev, next, len) \
    struct ternfs_directory_info_entries next; \
    _ternfs_directory_info_put_entries(ctx, &(prev), &(next), len)

#define ternfs_directory_info_put_end(ctx, prev, next) \
    { struct ternfs_directory_info_entries* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_directory_info_end* next __attribute__((unused)) = NULL

#define TERNFS_CURRENT_EDGE_MAX_SIZE 280
struct ternfs_current_edge_start;
#define ternfs_current_edge_get_start(ctx, start) struct ternfs_current_edge_start* start = NULL

struct ternfs_current_edge_target_id { u64 x; };
static inline void _ternfs_current_edge_get_target_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_current_edge_start** prev, struct ternfs_current_edge_target_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_current_edge_get_target_id(ctx, prev, next) \
    struct ternfs_current_edge_target_id next; \
    _ternfs_current_edge_get_target_id(ctx, &(prev), &(next))

struct ternfs_current_edge_name_hash { u64 x; };
static inline void _ternfs_current_edge_get_name_hash(struct ternfs_bincode_get_ctx* ctx, struct ternfs_current_edge_target_id* prev, struct ternfs_current_edge_name_hash* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_current_edge_get_name_hash(ctx, prev, next) \
    struct ternfs_current_edge_name_hash next; \
    _ternfs_current_edge_get_name_hash(ctx, &(prev), &(next))

struct ternfs_current_edge_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_current_edge_get_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_current_edge_name_hash* prev, struct ternfs_current_edge_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_current_edge_get_name(ctx, prev, next) \
    struct ternfs_current_edge_name next; \
    _ternfs_current_edge_get_name(ctx, &(prev), &(next))

struct ternfs_current_edge_creation_time { u64 x; };
static inline void _ternfs_current_edge_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_current_edge_name* prev, struct ternfs_current_edge_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_current_edge_get_creation_time(ctx, prev, next) \
    struct ternfs_current_edge_creation_time next; \
    _ternfs_current_edge_get_creation_time(ctx, &(prev), &(next))

struct ternfs_current_edge_end;
#define ternfs_current_edge_get_end(ctx, prev, next) \
    { struct ternfs_current_edge_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_current_edge_end* next = NULL

static inline void ternfs_current_edge_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_current_edge_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_current_edge_put_start(ctx, start) struct ternfs_current_edge_start* start = NULL

static inline void _ternfs_current_edge_put_target_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_current_edge_start** prev, struct ternfs_current_edge_target_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_current_edge_put_target_id(ctx, prev, next, x) \
    struct ternfs_current_edge_target_id next; \
    _ternfs_current_edge_put_target_id(ctx, &(prev), &(next), x)

static inline void _ternfs_current_edge_put_name_hash(struct ternfs_bincode_put_ctx* ctx, struct ternfs_current_edge_target_id* prev, struct ternfs_current_edge_name_hash* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_current_edge_put_name_hash(ctx, prev, next, x) \
    struct ternfs_current_edge_name_hash next; \
    _ternfs_current_edge_put_name_hash(ctx, &(prev), &(next), x)

static inline void _ternfs_current_edge_put_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_current_edge_name_hash* prev, struct ternfs_current_edge_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_current_edge_put_name(ctx, prev, next, str, str_len) \
    struct ternfs_current_edge_name next; \
    _ternfs_current_edge_put_name(ctx, &(prev), &(next), str, str_len)

static inline void _ternfs_current_edge_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_current_edge_name* prev, struct ternfs_current_edge_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_current_edge_put_creation_time(ctx, prev, next, x) \
    struct ternfs_current_edge_creation_time next; \
    _ternfs_current_edge_put_creation_time(ctx, &(prev), &(next), x)

#define ternfs_current_edge_put_end(ctx, prev, next) \
    { struct ternfs_current_edge_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_current_edge_end* next __attribute__((unused)) = NULL

struct ternfs_add_span_initiate_block_info_start;
#define ternfs_add_span_initiate_block_info_get_start(ctx, start) struct ternfs_add_span_initiate_block_info_start* start = NULL

#define ternfs_add_span_initiate_block_info_get_block_service_addrs(ctx, prev, next) \
    { struct ternfs_add_span_initiate_block_info_start** __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_addrs_info_start* next = NULL

struct ternfs_add_span_initiate_block_info_block_service_id { u64 x; };
static inline void _ternfs_add_span_initiate_block_info_get_block_service_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_addrs_info_end** prev, struct ternfs_add_span_initiate_block_info_block_service_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_span_initiate_block_info_get_block_service_id(ctx, prev, next) \
    struct ternfs_add_span_initiate_block_info_block_service_id next; \
    _ternfs_add_span_initiate_block_info_get_block_service_id(ctx, &(prev), &(next))

#define ternfs_add_span_initiate_block_info_get_block_service_failure_domain(ctx, prev, next) \
    { struct ternfs_add_span_initiate_block_info_block_service_id* __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_failure_domain_start* next = NULL

struct ternfs_add_span_initiate_block_info_block_id { u64 x; };
static inline void _ternfs_add_span_initiate_block_info_get_block_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_failure_domain_end** prev, struct ternfs_add_span_initiate_block_info_block_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_span_initiate_block_info_get_block_id(ctx, prev, next) \
    struct ternfs_add_span_initiate_block_info_block_id next; \
    _ternfs_add_span_initiate_block_info_get_block_id(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_block_info_certificate { u64 x; };
static inline void _ternfs_add_span_initiate_block_info_get_certificate(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_block_info_block_id* prev, struct ternfs_add_span_initiate_block_info_certificate* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_span_initiate_block_info_get_certificate(ctx, prev, next) \
    struct ternfs_add_span_initiate_block_info_certificate next; \
    _ternfs_add_span_initiate_block_info_get_certificate(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_block_info_end;
#define ternfs_add_span_initiate_block_info_get_end(ctx, prev, next) \
    { struct ternfs_add_span_initiate_block_info_certificate* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_span_initiate_block_info_end* next = NULL

static inline void ternfs_add_span_initiate_block_info_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_block_info_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_add_span_initiate_block_info_put_start(ctx, start) struct ternfs_add_span_initiate_block_info_start* start = NULL

static inline void _ternfs_add_span_initiate_block_info_put_block_service_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_block_info_start** prev, struct ternfs_add_span_initiate_block_info_block_service_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_span_initiate_block_info_put_block_service_id(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_block_info_block_service_id next; \
    _ternfs_add_span_initiate_block_info_put_block_service_id(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_block_info_put_block_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_block_info_block_service_id* prev, struct ternfs_add_span_initiate_block_info_block_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_span_initiate_block_info_put_block_id(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_block_info_block_id next; \
    _ternfs_add_span_initiate_block_info_put_block_id(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_block_info_put_certificate(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_block_info_block_id* prev, struct ternfs_add_span_initiate_block_info_certificate* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_span_initiate_block_info_put_certificate(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_block_info_certificate next; \
    _ternfs_add_span_initiate_block_info_put_certificate(ctx, &(prev), &(next), x)

#define ternfs_add_span_initiate_block_info_put_end(ctx, prev, next) \
    { struct ternfs_add_span_initiate_block_info_certificate* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_span_initiate_block_info_end* next __attribute__((unused)) = NULL

struct ternfs_remove_span_initiate_block_info_start;
#define ternfs_remove_span_initiate_block_info_get_start(ctx, start) struct ternfs_remove_span_initiate_block_info_start* start = NULL

#define ternfs_remove_span_initiate_block_info_get_block_service_addrs(ctx, prev, next) \
    { struct ternfs_remove_span_initiate_block_info_start** __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_addrs_info_start* next = NULL

struct ternfs_remove_span_initiate_block_info_block_service_id { u64 x; };
static inline void _ternfs_remove_span_initiate_block_info_get_block_service_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_addrs_info_end** prev, struct ternfs_remove_span_initiate_block_info_block_service_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_remove_span_initiate_block_info_get_block_service_id(ctx, prev, next) \
    struct ternfs_remove_span_initiate_block_info_block_service_id next; \
    _ternfs_remove_span_initiate_block_info_get_block_service_id(ctx, &(prev), &(next))

#define ternfs_remove_span_initiate_block_info_get_block_service_failure_domain(ctx, prev, next) \
    { struct ternfs_remove_span_initiate_block_info_block_service_id* __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_failure_domain_start* next = NULL

struct ternfs_remove_span_initiate_block_info_block_service_flags { u8 x; };
static inline void _ternfs_remove_span_initiate_block_info_get_block_service_flags(struct ternfs_bincode_get_ctx* ctx, struct ternfs_failure_domain_end** prev, struct ternfs_remove_span_initiate_block_info_block_service_flags* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_remove_span_initiate_block_info_get_block_service_flags(ctx, prev, next) \
    struct ternfs_remove_span_initiate_block_info_block_service_flags next; \
    _ternfs_remove_span_initiate_block_info_get_block_service_flags(ctx, &(prev), &(next))

struct ternfs_remove_span_initiate_block_info_block_id { u64 x; };
static inline void _ternfs_remove_span_initiate_block_info_get_block_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_remove_span_initiate_block_info_block_service_flags* prev, struct ternfs_remove_span_initiate_block_info_block_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_remove_span_initiate_block_info_get_block_id(ctx, prev, next) \
    struct ternfs_remove_span_initiate_block_info_block_id next; \
    _ternfs_remove_span_initiate_block_info_get_block_id(ctx, &(prev), &(next))

struct ternfs_remove_span_initiate_block_info_certificate { u64 x; };
static inline void _ternfs_remove_span_initiate_block_info_get_certificate(struct ternfs_bincode_get_ctx* ctx, struct ternfs_remove_span_initiate_block_info_block_id* prev, struct ternfs_remove_span_initiate_block_info_certificate* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_remove_span_initiate_block_info_get_certificate(ctx, prev, next) \
    struct ternfs_remove_span_initiate_block_info_certificate next; \
    _ternfs_remove_span_initiate_block_info_get_certificate(ctx, &(prev), &(next))

struct ternfs_remove_span_initiate_block_info_end;
#define ternfs_remove_span_initiate_block_info_get_end(ctx, prev, next) \
    { struct ternfs_remove_span_initiate_block_info_certificate* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_remove_span_initiate_block_info_end* next = NULL

static inline void ternfs_remove_span_initiate_block_info_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_remove_span_initiate_block_info_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_remove_span_initiate_block_info_put_start(ctx, start) struct ternfs_remove_span_initiate_block_info_start* start = NULL

static inline void _ternfs_remove_span_initiate_block_info_put_block_service_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_remove_span_initiate_block_info_start** prev, struct ternfs_remove_span_initiate_block_info_block_service_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_remove_span_initiate_block_info_put_block_service_id(ctx, prev, next, x) \
    struct ternfs_remove_span_initiate_block_info_block_service_id next; \
    _ternfs_remove_span_initiate_block_info_put_block_service_id(ctx, &(prev), &(next), x)

static inline void _ternfs_remove_span_initiate_block_info_put_block_service_flags(struct ternfs_bincode_put_ctx* ctx, struct ternfs_remove_span_initiate_block_info_block_service_id* prev, struct ternfs_remove_span_initiate_block_info_block_service_flags* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_remove_span_initiate_block_info_put_block_service_flags(ctx, prev, next, x) \
    struct ternfs_remove_span_initiate_block_info_block_service_flags next; \
    _ternfs_remove_span_initiate_block_info_put_block_service_flags(ctx, &(prev), &(next), x)

static inline void _ternfs_remove_span_initiate_block_info_put_block_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_remove_span_initiate_block_info_block_service_flags* prev, struct ternfs_remove_span_initiate_block_info_block_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_remove_span_initiate_block_info_put_block_id(ctx, prev, next, x) \
    struct ternfs_remove_span_initiate_block_info_block_id next; \
    _ternfs_remove_span_initiate_block_info_put_block_id(ctx, &(prev), &(next), x)

static inline void _ternfs_remove_span_initiate_block_info_put_certificate(struct ternfs_bincode_put_ctx* ctx, struct ternfs_remove_span_initiate_block_info_block_id* prev, struct ternfs_remove_span_initiate_block_info_certificate* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_remove_span_initiate_block_info_put_certificate(ctx, prev, next, x) \
    struct ternfs_remove_span_initiate_block_info_certificate next; \
    _ternfs_remove_span_initiate_block_info_put_certificate(ctx, &(prev), &(next), x)

#define ternfs_remove_span_initiate_block_info_put_end(ctx, prev, next) \
    { struct ternfs_remove_span_initiate_block_info_certificate* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_remove_span_initiate_block_info_end* next __attribute__((unused)) = NULL

#define TERNFS_BLOCK_PROOF_SIZE 16
struct ternfs_block_proof_start;
#define ternfs_block_proof_get_start(ctx, start) struct ternfs_block_proof_start* start = NULL

struct ternfs_block_proof_block_id { u64 x; };
static inline void _ternfs_block_proof_get_block_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_block_proof_start** prev, struct ternfs_block_proof_block_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_block_proof_get_block_id(ctx, prev, next) \
    struct ternfs_block_proof_block_id next; \
    _ternfs_block_proof_get_block_id(ctx, &(prev), &(next))

struct ternfs_block_proof_proof { u64 x; };
static inline void _ternfs_block_proof_get_proof(struct ternfs_bincode_get_ctx* ctx, struct ternfs_block_proof_block_id* prev, struct ternfs_block_proof_proof* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_block_proof_get_proof(ctx, prev, next) \
    struct ternfs_block_proof_proof next; \
    _ternfs_block_proof_get_proof(ctx, &(prev), &(next))

struct ternfs_block_proof_end;
#define ternfs_block_proof_get_end(ctx, prev, next) \
    { struct ternfs_block_proof_proof* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_block_proof_end* next = NULL

static inline void ternfs_block_proof_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_block_proof_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_block_proof_put_start(ctx, start) struct ternfs_block_proof_start* start = NULL

static inline void _ternfs_block_proof_put_block_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_block_proof_start** prev, struct ternfs_block_proof_block_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_block_proof_put_block_id(ctx, prev, next, x) \
    struct ternfs_block_proof_block_id next; \
    _ternfs_block_proof_put_block_id(ctx, &(prev), &(next), x)

static inline void _ternfs_block_proof_put_proof(struct ternfs_bincode_put_ctx* ctx, struct ternfs_block_proof_block_id* prev, struct ternfs_block_proof_proof* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_block_proof_put_proof(ctx, prev, next, x) \
    struct ternfs_block_proof_proof next; \
    _ternfs_block_proof_put_proof(ctx, &(prev), &(next), x)

#define ternfs_block_proof_put_end(ctx, prev, next) \
    { struct ternfs_block_proof_proof* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_block_proof_end* next __attribute__((unused)) = NULL

#define TERNFS_BLOCK_SERVICE_SIZE 21
struct ternfs_block_service_start;
#define ternfs_block_service_get_start(ctx, start) struct ternfs_block_service_start* start = NULL

#define ternfs_block_service_get_addrs(ctx, prev, next) \
    { struct ternfs_block_service_start** __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_addrs_info_start* next = NULL

struct ternfs_block_service_id { u64 x; };
static inline void _ternfs_block_service_get_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_addrs_info_end** prev, struct ternfs_block_service_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_block_service_get_id(ctx, prev, next) \
    struct ternfs_block_service_id next; \
    _ternfs_block_service_get_id(ctx, &(prev), &(next))

struct ternfs_block_service_flags { u8 x; };
static inline void _ternfs_block_service_get_flags(struct ternfs_bincode_get_ctx* ctx, struct ternfs_block_service_id* prev, struct ternfs_block_service_flags* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_block_service_get_flags(ctx, prev, next) \
    struct ternfs_block_service_flags next; \
    _ternfs_block_service_get_flags(ctx, &(prev), &(next))

struct ternfs_block_service_end;
#define ternfs_block_service_get_end(ctx, prev, next) \
    { struct ternfs_block_service_flags* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_block_service_end* next = NULL

static inline void ternfs_block_service_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_block_service_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_block_service_put_start(ctx, start) struct ternfs_block_service_start* start = NULL

static inline void _ternfs_block_service_put_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_block_service_start** prev, struct ternfs_block_service_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_block_service_put_id(ctx, prev, next, x) \
    struct ternfs_block_service_id next; \
    _ternfs_block_service_put_id(ctx, &(prev), &(next), x)

static inline void _ternfs_block_service_put_flags(struct ternfs_bincode_put_ctx* ctx, struct ternfs_block_service_id* prev, struct ternfs_block_service_flags* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_block_service_put_flags(ctx, prev, next, x) \
    struct ternfs_block_service_flags next; \
    _ternfs_block_service_put_flags(ctx, &(prev), &(next), x)

#define ternfs_block_service_put_end(ctx, prev, next) \
    { struct ternfs_block_service_flags* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_block_service_end* next __attribute__((unused)) = NULL

#define TERNFS_SHARD_INFO_SIZE 20
struct ternfs_shard_info_start;
#define ternfs_shard_info_get_start(ctx, start) struct ternfs_shard_info_start* start = NULL

#define ternfs_shard_info_get_addrs(ctx, prev, next) \
    { struct ternfs_shard_info_start** __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_addrs_info_start* next = NULL

struct ternfs_shard_info_last_seen { u64 x; };
static inline void _ternfs_shard_info_get_last_seen(struct ternfs_bincode_get_ctx* ctx, struct ternfs_addrs_info_end** prev, struct ternfs_shard_info_last_seen* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_shard_info_get_last_seen(ctx, prev, next) \
    struct ternfs_shard_info_last_seen next; \
    _ternfs_shard_info_get_last_seen(ctx, &(prev), &(next))

struct ternfs_shard_info_end;
#define ternfs_shard_info_get_end(ctx, prev, next) \
    { struct ternfs_shard_info_last_seen* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_shard_info_end* next = NULL

static inline void ternfs_shard_info_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_shard_info_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_shard_info_put_start(ctx, start) struct ternfs_shard_info_start* start = NULL

static inline void _ternfs_shard_info_put_last_seen(struct ternfs_bincode_put_ctx* ctx, struct ternfs_shard_info_start** prev, struct ternfs_shard_info_last_seen* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_shard_info_put_last_seen(ctx, prev, next, x) \
    struct ternfs_shard_info_last_seen next; \
    _ternfs_shard_info_put_last_seen(ctx, &(prev), &(next), x)

#define ternfs_shard_info_put_end(ctx, prev, next) \
    { struct ternfs_shard_info_last_seen* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_shard_info_end* next __attribute__((unused)) = NULL

#define TERNFS_BLOCK_POLICY_ENTRY_SIZE 5
struct ternfs_block_policy_entry_start;
#define ternfs_block_policy_entry_get_start(ctx, start) struct ternfs_block_policy_entry_start* start = NULL

struct ternfs_block_policy_entry_storage_class { u8 x; };
static inline void _ternfs_block_policy_entry_get_storage_class(struct ternfs_bincode_get_ctx* ctx, struct ternfs_block_policy_entry_start** prev, struct ternfs_block_policy_entry_storage_class* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_block_policy_entry_get_storage_class(ctx, prev, next) \
    struct ternfs_block_policy_entry_storage_class next; \
    _ternfs_block_policy_entry_get_storage_class(ctx, &(prev), &(next))

struct ternfs_block_policy_entry_min_size { u32 x; };
static inline void _ternfs_block_policy_entry_get_min_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_block_policy_entry_storage_class* prev, struct ternfs_block_policy_entry_min_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_block_policy_entry_get_min_size(ctx, prev, next) \
    struct ternfs_block_policy_entry_min_size next; \
    _ternfs_block_policy_entry_get_min_size(ctx, &(prev), &(next))

struct ternfs_block_policy_entry_end;
#define ternfs_block_policy_entry_get_end(ctx, prev, next) \
    { struct ternfs_block_policy_entry_min_size* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_block_policy_entry_end* next = NULL

static inline void ternfs_block_policy_entry_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_block_policy_entry_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_block_policy_entry_put_start(ctx, start) struct ternfs_block_policy_entry_start* start = NULL

static inline void _ternfs_block_policy_entry_put_storage_class(struct ternfs_bincode_put_ctx* ctx, struct ternfs_block_policy_entry_start** prev, struct ternfs_block_policy_entry_storage_class* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_block_policy_entry_put_storage_class(ctx, prev, next, x) \
    struct ternfs_block_policy_entry_storage_class next; \
    _ternfs_block_policy_entry_put_storage_class(ctx, &(prev), &(next), x)

static inline void _ternfs_block_policy_entry_put_min_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_block_policy_entry_storage_class* prev, struct ternfs_block_policy_entry_min_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_block_policy_entry_put_min_size(ctx, prev, next, x) \
    struct ternfs_block_policy_entry_min_size next; \
    _ternfs_block_policy_entry_put_min_size(ctx, &(prev), &(next), x)

#define ternfs_block_policy_entry_put_end(ctx, prev, next) \
    { struct ternfs_block_policy_entry_min_size* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_block_policy_entry_end* next __attribute__((unused)) = NULL

#define TERNFS_SPAN_POLICY_ENTRY_SIZE 5
struct ternfs_span_policy_entry_start;
#define ternfs_span_policy_entry_get_start(ctx, start) struct ternfs_span_policy_entry_start* start = NULL

struct ternfs_span_policy_entry_max_size { u32 x; };
static inline void _ternfs_span_policy_entry_get_max_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_span_policy_entry_start** prev, struct ternfs_span_policy_entry_max_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_span_policy_entry_get_max_size(ctx, prev, next) \
    struct ternfs_span_policy_entry_max_size next; \
    _ternfs_span_policy_entry_get_max_size(ctx, &(prev), &(next))

struct ternfs_span_policy_entry_parity { u8 x; };
static inline void _ternfs_span_policy_entry_get_parity(struct ternfs_bincode_get_ctx* ctx, struct ternfs_span_policy_entry_max_size* prev, struct ternfs_span_policy_entry_parity* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_span_policy_entry_get_parity(ctx, prev, next) \
    struct ternfs_span_policy_entry_parity next; \
    _ternfs_span_policy_entry_get_parity(ctx, &(prev), &(next))

struct ternfs_span_policy_entry_end;
#define ternfs_span_policy_entry_get_end(ctx, prev, next) \
    { struct ternfs_span_policy_entry_parity* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_span_policy_entry_end* next = NULL

static inline void ternfs_span_policy_entry_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_span_policy_entry_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_span_policy_entry_put_start(ctx, start) struct ternfs_span_policy_entry_start* start = NULL

static inline void _ternfs_span_policy_entry_put_max_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_span_policy_entry_start** prev, struct ternfs_span_policy_entry_max_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_span_policy_entry_put_max_size(ctx, prev, next, x) \
    struct ternfs_span_policy_entry_max_size next; \
    _ternfs_span_policy_entry_put_max_size(ctx, &(prev), &(next), x)

static inline void _ternfs_span_policy_entry_put_parity(struct ternfs_bincode_put_ctx* ctx, struct ternfs_span_policy_entry_max_size* prev, struct ternfs_span_policy_entry_parity* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_span_policy_entry_put_parity(ctx, prev, next, x) \
    struct ternfs_span_policy_entry_parity next; \
    _ternfs_span_policy_entry_put_parity(ctx, &(prev), &(next), x)

#define ternfs_span_policy_entry_put_end(ctx, prev, next) \
    { struct ternfs_span_policy_entry_parity* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_span_policy_entry_end* next __attribute__((unused)) = NULL

#define TERNFS_STRIPE_POLICY_SIZE 4
struct ternfs_stripe_policy_start;
#define ternfs_stripe_policy_get_start(ctx, start) struct ternfs_stripe_policy_start* start = NULL

struct ternfs_stripe_policy_target_stripe_size { u32 x; };
static inline void _ternfs_stripe_policy_get_target_stripe_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stripe_policy_start** prev, struct ternfs_stripe_policy_target_stripe_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_stripe_policy_get_target_stripe_size(ctx, prev, next) \
    struct ternfs_stripe_policy_target_stripe_size next; \
    _ternfs_stripe_policy_get_target_stripe_size(ctx, &(prev), &(next))

struct ternfs_stripe_policy_end;
#define ternfs_stripe_policy_get_end(ctx, prev, next) \
    { struct ternfs_stripe_policy_target_stripe_size* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_stripe_policy_end* next = NULL

static inline void ternfs_stripe_policy_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stripe_policy_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_stripe_policy_put_start(ctx, start) struct ternfs_stripe_policy_start* start = NULL

static inline void _ternfs_stripe_policy_put_target_stripe_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_stripe_policy_start** prev, struct ternfs_stripe_policy_target_stripe_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_stripe_policy_put_target_stripe_size(ctx, prev, next, x) \
    struct ternfs_stripe_policy_target_stripe_size next; \
    _ternfs_stripe_policy_put_target_stripe_size(ctx, &(prev), &(next), x)

#define ternfs_stripe_policy_put_end(ctx, prev, next) \
    { struct ternfs_stripe_policy_target_stripe_size* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_stripe_policy_end* next __attribute__((unused)) = NULL

#define TERNFS_FETCHED_BLOCK_SIZE 13
struct ternfs_fetched_block_start;
#define ternfs_fetched_block_get_start(ctx, start) struct ternfs_fetched_block_start* start = NULL

struct ternfs_fetched_block_block_service_ix { u8 x; };
static inline void _ternfs_fetched_block_get_block_service_ix(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_start** prev, struct ternfs_fetched_block_block_service_ix* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_fetched_block_get_block_service_ix(ctx, prev, next) \
    struct ternfs_fetched_block_block_service_ix next; \
    _ternfs_fetched_block_get_block_service_ix(ctx, &(prev), &(next))

struct ternfs_fetched_block_block_id { u64 x; };
static inline void _ternfs_fetched_block_get_block_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_block_service_ix* prev, struct ternfs_fetched_block_block_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_fetched_block_get_block_id(ctx, prev, next) \
    struct ternfs_fetched_block_block_id next; \
    _ternfs_fetched_block_get_block_id(ctx, &(prev), &(next))

struct ternfs_fetched_block_crc { u32 x; };
static inline void _ternfs_fetched_block_get_crc(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_block_id* prev, struct ternfs_fetched_block_crc* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetched_block_get_crc(ctx, prev, next) \
    struct ternfs_fetched_block_crc next; \
    _ternfs_fetched_block_get_crc(ctx, &(prev), &(next))

struct ternfs_fetched_block_end;
#define ternfs_fetched_block_get_end(ctx, prev, next) \
    { struct ternfs_fetched_block_crc* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_block_end* next = NULL

static inline void ternfs_fetched_block_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetched_block_put_start(ctx, start) struct ternfs_fetched_block_start* start = NULL

static inline void _ternfs_fetched_block_put_block_service_ix(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_block_start** prev, struct ternfs_fetched_block_block_service_ix* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_fetched_block_put_block_service_ix(ctx, prev, next, x) \
    struct ternfs_fetched_block_block_service_ix next; \
    _ternfs_fetched_block_put_block_service_ix(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_block_put_block_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_block_block_service_ix* prev, struct ternfs_fetched_block_block_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_fetched_block_put_block_id(ctx, prev, next, x) \
    struct ternfs_fetched_block_block_id next; \
    _ternfs_fetched_block_put_block_id(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_block_put_crc(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_block_block_id* prev, struct ternfs_fetched_block_crc* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetched_block_put_crc(ctx, prev, next, x) \
    struct ternfs_fetched_block_crc next; \
    _ternfs_fetched_block_put_crc(ctx, &(prev), &(next), x)

#define ternfs_fetched_block_put_end(ctx, prev, next) \
    { struct ternfs_fetched_block_crc* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_block_end* next __attribute__((unused)) = NULL

#define TERNFS_FETCHED_SPAN_HEADER_SIZE 17
struct ternfs_fetched_span_header_start;
#define ternfs_fetched_span_header_get_start(ctx, start) struct ternfs_fetched_span_header_start* start = NULL

struct ternfs_fetched_span_header_byte_offset { u64 x; };
static inline void _ternfs_fetched_span_header_get_byte_offset(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_span_header_start** prev, struct ternfs_fetched_span_header_byte_offset* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_fetched_span_header_get_byte_offset(ctx, prev, next) \
    struct ternfs_fetched_span_header_byte_offset next; \
    _ternfs_fetched_span_header_get_byte_offset(ctx, &(prev), &(next))

struct ternfs_fetched_span_header_size { u32 x; };
static inline void _ternfs_fetched_span_header_get_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_span_header_byte_offset* prev, struct ternfs_fetched_span_header_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetched_span_header_get_size(ctx, prev, next) \
    struct ternfs_fetched_span_header_size next; \
    _ternfs_fetched_span_header_get_size(ctx, &(prev), &(next))

struct ternfs_fetched_span_header_crc { u32 x; };
static inline void _ternfs_fetched_span_header_get_crc(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_span_header_size* prev, struct ternfs_fetched_span_header_crc* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetched_span_header_get_crc(ctx, prev, next) \
    struct ternfs_fetched_span_header_crc next; \
    _ternfs_fetched_span_header_get_crc(ctx, &(prev), &(next))

struct ternfs_fetched_span_header_storage_class { u8 x; };
static inline void _ternfs_fetched_span_header_get_storage_class(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_span_header_crc* prev, struct ternfs_fetched_span_header_storage_class* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_fetched_span_header_get_storage_class(ctx, prev, next) \
    struct ternfs_fetched_span_header_storage_class next; \
    _ternfs_fetched_span_header_get_storage_class(ctx, &(prev), &(next))

struct ternfs_fetched_span_header_end;
#define ternfs_fetched_span_header_get_end(ctx, prev, next) \
    { struct ternfs_fetched_span_header_storage_class* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_span_header_end* next = NULL

static inline void ternfs_fetched_span_header_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_span_header_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetched_span_header_put_start(ctx, start) struct ternfs_fetched_span_header_start* start = NULL

static inline void _ternfs_fetched_span_header_put_byte_offset(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_span_header_start** prev, struct ternfs_fetched_span_header_byte_offset* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_fetched_span_header_put_byte_offset(ctx, prev, next, x) \
    struct ternfs_fetched_span_header_byte_offset next; \
    _ternfs_fetched_span_header_put_byte_offset(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_span_header_put_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_span_header_byte_offset* prev, struct ternfs_fetched_span_header_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetched_span_header_put_size(ctx, prev, next, x) \
    struct ternfs_fetched_span_header_size next; \
    _ternfs_fetched_span_header_put_size(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_span_header_put_crc(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_span_header_size* prev, struct ternfs_fetched_span_header_crc* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetched_span_header_put_crc(ctx, prev, next, x) \
    struct ternfs_fetched_span_header_crc next; \
    _ternfs_fetched_span_header_put_crc(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_span_header_put_storage_class(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_span_header_crc* prev, struct ternfs_fetched_span_header_storage_class* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_fetched_span_header_put_storage_class(ctx, prev, next, x) \
    struct ternfs_fetched_span_header_storage_class next; \
    _ternfs_fetched_span_header_put_storage_class(ctx, &(prev), &(next), x)

#define ternfs_fetched_span_header_put_end(ctx, prev, next) \
    { struct ternfs_fetched_span_header_storage_class* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_span_header_end* next __attribute__((unused)) = NULL

#define TERNFS_FETCHED_INLINE_SPAN_MAX_SIZE 256
struct ternfs_fetched_inline_span_start;
#define ternfs_fetched_inline_span_get_start(ctx, start) struct ternfs_fetched_inline_span_start* start = NULL

struct ternfs_fetched_inline_span_body { struct ternfs_bincode_bytes str; };
static inline void _ternfs_fetched_inline_span_get_body(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_inline_span_start** prev, struct ternfs_fetched_inline_span_body* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_fetched_inline_span_get_body(ctx, prev, next) \
    struct ternfs_fetched_inline_span_body next; \
    _ternfs_fetched_inline_span_get_body(ctx, &(prev), &(next))

struct ternfs_fetched_inline_span_end;
#define ternfs_fetched_inline_span_get_end(ctx, prev, next) \
    { struct ternfs_fetched_inline_span_body* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_inline_span_end* next = NULL

static inline void ternfs_fetched_inline_span_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_inline_span_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetched_inline_span_put_start(ctx, start) struct ternfs_fetched_inline_span_start* start = NULL

static inline void _ternfs_fetched_inline_span_put_body(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_inline_span_start** prev, struct ternfs_fetched_inline_span_body* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_fetched_inline_span_put_body(ctx, prev, next, str, str_len) \
    struct ternfs_fetched_inline_span_body next; \
    _ternfs_fetched_inline_span_put_body(ctx, &(prev), &(next), str, str_len)

#define ternfs_fetched_inline_span_put_end(ctx, prev, next) \
    { struct ternfs_fetched_inline_span_body* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_inline_span_end* next __attribute__((unused)) = NULL

struct ternfs_fetched_blocks_span_start;
#define ternfs_fetched_blocks_span_get_start(ctx, start) struct ternfs_fetched_blocks_span_start* start = NULL

struct ternfs_fetched_blocks_span_parity { u8 x; };
static inline void _ternfs_fetched_blocks_span_get_parity(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_blocks_span_start** prev, struct ternfs_fetched_blocks_span_parity* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_fetched_blocks_span_get_parity(ctx, prev, next) \
    struct ternfs_fetched_blocks_span_parity next; \
    _ternfs_fetched_blocks_span_get_parity(ctx, &(prev), &(next))

struct ternfs_fetched_blocks_span_stripes { u8 x; };
static inline void _ternfs_fetched_blocks_span_get_stripes(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_blocks_span_parity* prev, struct ternfs_fetched_blocks_span_stripes* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_fetched_blocks_span_get_stripes(ctx, prev, next) \
    struct ternfs_fetched_blocks_span_stripes next; \
    _ternfs_fetched_blocks_span_get_stripes(ctx, &(prev), &(next))

struct ternfs_fetched_blocks_span_cell_size { u32 x; };
static inline void _ternfs_fetched_blocks_span_get_cell_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_blocks_span_stripes* prev, struct ternfs_fetched_blocks_span_cell_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetched_blocks_span_get_cell_size(ctx, prev, next) \
    struct ternfs_fetched_blocks_span_cell_size next; \
    _ternfs_fetched_blocks_span_get_cell_size(ctx, &(prev), &(next))

struct ternfs_fetched_blocks_span_blocks { u16 len; };
static inline void _ternfs_fetched_blocks_span_get_blocks(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_blocks_span_cell_size* prev, struct ternfs_fetched_blocks_span_blocks* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_fetched_blocks_span_get_blocks(ctx, prev, next) \
    struct ternfs_fetched_blocks_span_blocks next; \
    _ternfs_fetched_blocks_span_get_blocks(ctx, &(prev), &(next))

struct ternfs_fetched_blocks_span_stripes_crc { u16 len; };
static inline void _ternfs_fetched_blocks_span_get_stripes_crc(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_blocks_span_blocks* prev, struct ternfs_fetched_blocks_span_stripes_crc* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_fetched_blocks_span_get_stripes_crc(ctx, prev, next) \
    struct ternfs_fetched_blocks_span_stripes_crc next; \
    _ternfs_fetched_blocks_span_get_stripes_crc(ctx, &(prev), &(next))

struct ternfs_fetched_blocks_span_end;
#define ternfs_fetched_blocks_span_get_end(ctx, prev, next) \
    { struct ternfs_fetched_blocks_span_stripes_crc* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_blocks_span_end* next = NULL

static inline void ternfs_fetched_blocks_span_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_blocks_span_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetched_blocks_span_put_start(ctx, start) struct ternfs_fetched_blocks_span_start* start = NULL

static inline void _ternfs_fetched_blocks_span_put_parity(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_blocks_span_start** prev, struct ternfs_fetched_blocks_span_parity* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_fetched_blocks_span_put_parity(ctx, prev, next, x) \
    struct ternfs_fetched_blocks_span_parity next; \
    _ternfs_fetched_blocks_span_put_parity(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_blocks_span_put_stripes(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_blocks_span_parity* prev, struct ternfs_fetched_blocks_span_stripes* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_fetched_blocks_span_put_stripes(ctx, prev, next, x) \
    struct ternfs_fetched_blocks_span_stripes next; \
    _ternfs_fetched_blocks_span_put_stripes(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_blocks_span_put_cell_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_blocks_span_stripes* prev, struct ternfs_fetched_blocks_span_cell_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetched_blocks_span_put_cell_size(ctx, prev, next, x) \
    struct ternfs_fetched_blocks_span_cell_size next; \
    _ternfs_fetched_blocks_span_put_cell_size(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_blocks_span_put_blocks(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_blocks_span_cell_size* prev, struct ternfs_fetched_blocks_span_blocks* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_fetched_blocks_span_put_blocks(ctx, prev, next, len) \
    struct ternfs_fetched_blocks_span_blocks next; \
    _ternfs_fetched_blocks_span_put_blocks(ctx, &(prev), &(next), len)

static inline void _ternfs_fetched_blocks_span_put_stripes_crc(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_blocks_span_blocks* prev, struct ternfs_fetched_blocks_span_stripes_crc* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_fetched_blocks_span_put_stripes_crc(ctx, prev, next, len) \
    struct ternfs_fetched_blocks_span_stripes_crc next; \
    _ternfs_fetched_blocks_span_put_stripes_crc(ctx, &(prev), &(next), len)

#define ternfs_fetched_blocks_span_put_end(ctx, prev, next) \
    { struct ternfs_fetched_blocks_span_stripes_crc* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_blocks_span_end* next __attribute__((unused)) = NULL

struct ternfs_fetched_block_services_start;
#define ternfs_fetched_block_services_get_start(ctx, start) struct ternfs_fetched_block_services_start* start = NULL

struct ternfs_fetched_block_services_location_id { u8 x; };
static inline void _ternfs_fetched_block_services_get_location_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_services_start** prev, struct ternfs_fetched_block_services_location_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_fetched_block_services_get_location_id(ctx, prev, next) \
    struct ternfs_fetched_block_services_location_id next; \
    _ternfs_fetched_block_services_get_location_id(ctx, &(prev), &(next))

struct ternfs_fetched_block_services_storage_class { u8 x; };
static inline void _ternfs_fetched_block_services_get_storage_class(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_services_location_id* prev, struct ternfs_fetched_block_services_storage_class* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_fetched_block_services_get_storage_class(ctx, prev, next) \
    struct ternfs_fetched_block_services_storage_class next; \
    _ternfs_fetched_block_services_get_storage_class(ctx, &(prev), &(next))

struct ternfs_fetched_block_services_parity { u8 x; };
static inline void _ternfs_fetched_block_services_get_parity(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_services_storage_class* prev, struct ternfs_fetched_block_services_parity* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_fetched_block_services_get_parity(ctx, prev, next) \
    struct ternfs_fetched_block_services_parity next; \
    _ternfs_fetched_block_services_get_parity(ctx, &(prev), &(next))

struct ternfs_fetched_block_services_stripes { u8 x; };
static inline void _ternfs_fetched_block_services_get_stripes(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_services_parity* prev, struct ternfs_fetched_block_services_stripes* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_fetched_block_services_get_stripes(ctx, prev, next) \
    struct ternfs_fetched_block_services_stripes next; \
    _ternfs_fetched_block_services_get_stripes(ctx, &(prev), &(next))

struct ternfs_fetched_block_services_cell_size { u32 x; };
static inline void _ternfs_fetched_block_services_get_cell_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_services_stripes* prev, struct ternfs_fetched_block_services_cell_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetched_block_services_get_cell_size(ctx, prev, next) \
    struct ternfs_fetched_block_services_cell_size next; \
    _ternfs_fetched_block_services_get_cell_size(ctx, &(prev), &(next))

struct ternfs_fetched_block_services_blocks { u16 len; };
static inline void _ternfs_fetched_block_services_get_blocks(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_services_cell_size* prev, struct ternfs_fetched_block_services_blocks* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_fetched_block_services_get_blocks(ctx, prev, next) \
    struct ternfs_fetched_block_services_blocks next; \
    _ternfs_fetched_block_services_get_blocks(ctx, &(prev), &(next))

struct ternfs_fetched_block_services_stripes_crc { u16 len; };
static inline void _ternfs_fetched_block_services_get_stripes_crc(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_services_blocks* prev, struct ternfs_fetched_block_services_stripes_crc* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_fetched_block_services_get_stripes_crc(ctx, prev, next) \
    struct ternfs_fetched_block_services_stripes_crc next; \
    _ternfs_fetched_block_services_get_stripes_crc(ctx, &(prev), &(next))

struct ternfs_fetched_block_services_end;
#define ternfs_fetched_block_services_get_end(ctx, prev, next) \
    { struct ternfs_fetched_block_services_stripes_crc* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_block_services_end* next = NULL

static inline void ternfs_fetched_block_services_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_block_services_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetched_block_services_put_start(ctx, start) struct ternfs_fetched_block_services_start* start = NULL

static inline void _ternfs_fetched_block_services_put_location_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_block_services_start** prev, struct ternfs_fetched_block_services_location_id* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_fetched_block_services_put_location_id(ctx, prev, next, x) \
    struct ternfs_fetched_block_services_location_id next; \
    _ternfs_fetched_block_services_put_location_id(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_block_services_put_storage_class(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_block_services_location_id* prev, struct ternfs_fetched_block_services_storage_class* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_fetched_block_services_put_storage_class(ctx, prev, next, x) \
    struct ternfs_fetched_block_services_storage_class next; \
    _ternfs_fetched_block_services_put_storage_class(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_block_services_put_parity(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_block_services_storage_class* prev, struct ternfs_fetched_block_services_parity* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_fetched_block_services_put_parity(ctx, prev, next, x) \
    struct ternfs_fetched_block_services_parity next; \
    _ternfs_fetched_block_services_put_parity(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_block_services_put_stripes(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_block_services_parity* prev, struct ternfs_fetched_block_services_stripes* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_fetched_block_services_put_stripes(ctx, prev, next, x) \
    struct ternfs_fetched_block_services_stripes next; \
    _ternfs_fetched_block_services_put_stripes(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_block_services_put_cell_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_block_services_stripes* prev, struct ternfs_fetched_block_services_cell_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetched_block_services_put_cell_size(ctx, prev, next, x) \
    struct ternfs_fetched_block_services_cell_size next; \
    _ternfs_fetched_block_services_put_cell_size(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_block_services_put_blocks(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_block_services_cell_size* prev, struct ternfs_fetched_block_services_blocks* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_fetched_block_services_put_blocks(ctx, prev, next, len) \
    struct ternfs_fetched_block_services_blocks next; \
    _ternfs_fetched_block_services_put_blocks(ctx, &(prev), &(next), len)

static inline void _ternfs_fetched_block_services_put_stripes_crc(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_block_services_blocks* prev, struct ternfs_fetched_block_services_stripes_crc* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_fetched_block_services_put_stripes_crc(ctx, prev, next, len) \
    struct ternfs_fetched_block_services_stripes_crc next; \
    _ternfs_fetched_block_services_put_stripes_crc(ctx, &(prev), &(next), len)

#define ternfs_fetched_block_services_put_end(ctx, prev, next) \
    { struct ternfs_fetched_block_services_stripes_crc* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_block_services_end* next __attribute__((unused)) = NULL

struct ternfs_fetched_locations_start;
#define ternfs_fetched_locations_get_start(ctx, start) struct ternfs_fetched_locations_start* start = NULL

struct ternfs_fetched_locations_locations { u16 len; };
static inline void _ternfs_fetched_locations_get_locations(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_locations_start** prev, struct ternfs_fetched_locations_locations* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_fetched_locations_get_locations(ctx, prev, next) \
    struct ternfs_fetched_locations_locations next; \
    _ternfs_fetched_locations_get_locations(ctx, &(prev), &(next))

struct ternfs_fetched_locations_end;
#define ternfs_fetched_locations_get_end(ctx, prev, next) \
    { struct ternfs_fetched_locations_locations* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_locations_end* next = NULL

static inline void ternfs_fetched_locations_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_locations_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetched_locations_put_start(ctx, start) struct ternfs_fetched_locations_start* start = NULL

static inline void _ternfs_fetched_locations_put_locations(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_locations_start** prev, struct ternfs_fetched_locations_locations* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_fetched_locations_put_locations(ctx, prev, next, len) \
    struct ternfs_fetched_locations_locations next; \
    _ternfs_fetched_locations_put_locations(ctx, &(prev), &(next), len)

#define ternfs_fetched_locations_put_end(ctx, prev, next) \
    { struct ternfs_fetched_locations_locations* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_locations_end* next __attribute__((unused)) = NULL

#define TERNFS_FETCHED_SPAN_HEADER_FULL_SIZE 17
struct ternfs_fetched_span_header_full_start;
#define ternfs_fetched_span_header_full_get_start(ctx, start) struct ternfs_fetched_span_header_full_start* start = NULL

struct ternfs_fetched_span_header_full_byte_offset { u64 x; };
static inline void _ternfs_fetched_span_header_full_get_byte_offset(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_span_header_full_start** prev, struct ternfs_fetched_span_header_full_byte_offset* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_fetched_span_header_full_get_byte_offset(ctx, prev, next) \
    struct ternfs_fetched_span_header_full_byte_offset next; \
    _ternfs_fetched_span_header_full_get_byte_offset(ctx, &(prev), &(next))

struct ternfs_fetched_span_header_full_size { u32 x; };
static inline void _ternfs_fetched_span_header_full_get_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_span_header_full_byte_offset* prev, struct ternfs_fetched_span_header_full_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetched_span_header_full_get_size(ctx, prev, next) \
    struct ternfs_fetched_span_header_full_size next; \
    _ternfs_fetched_span_header_full_get_size(ctx, &(prev), &(next))

struct ternfs_fetched_span_header_full_crc { u32 x; };
static inline void _ternfs_fetched_span_header_full_get_crc(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_span_header_full_size* prev, struct ternfs_fetched_span_header_full_crc* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetched_span_header_full_get_crc(ctx, prev, next) \
    struct ternfs_fetched_span_header_full_crc next; \
    _ternfs_fetched_span_header_full_get_crc(ctx, &(prev), &(next))

struct ternfs_fetched_span_header_full_is_inline { bool x; };
static inline void _ternfs_fetched_span_header_full_get_is_inline(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_span_header_full_crc* prev, struct ternfs_fetched_span_header_full_is_inline* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(bool*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_fetched_span_header_full_get_is_inline(ctx, prev, next) \
    struct ternfs_fetched_span_header_full_is_inline next; \
    _ternfs_fetched_span_header_full_get_is_inline(ctx, &(prev), &(next))

struct ternfs_fetched_span_header_full_end;
#define ternfs_fetched_span_header_full_get_end(ctx, prev, next) \
    { struct ternfs_fetched_span_header_full_is_inline* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_span_header_full_end* next = NULL

static inline void ternfs_fetched_span_header_full_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetched_span_header_full_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetched_span_header_full_put_start(ctx, start) struct ternfs_fetched_span_header_full_start* start = NULL

static inline void _ternfs_fetched_span_header_full_put_byte_offset(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_span_header_full_start** prev, struct ternfs_fetched_span_header_full_byte_offset* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_fetched_span_header_full_put_byte_offset(ctx, prev, next, x) \
    struct ternfs_fetched_span_header_full_byte_offset next; \
    _ternfs_fetched_span_header_full_put_byte_offset(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_span_header_full_put_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_span_header_full_byte_offset* prev, struct ternfs_fetched_span_header_full_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetched_span_header_full_put_size(ctx, prev, next, x) \
    struct ternfs_fetched_span_header_full_size next; \
    _ternfs_fetched_span_header_full_put_size(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_span_header_full_put_crc(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_span_header_full_size* prev, struct ternfs_fetched_span_header_full_crc* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetched_span_header_full_put_crc(ctx, prev, next, x) \
    struct ternfs_fetched_span_header_full_crc next; \
    _ternfs_fetched_span_header_full_put_crc(ctx, &(prev), &(next), x)

static inline void _ternfs_fetched_span_header_full_put_is_inline(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetched_span_header_full_crc* prev, struct ternfs_fetched_span_header_full_is_inline* next, bool x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(bool*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_fetched_span_header_full_put_is_inline(ctx, prev, next, x) \
    struct ternfs_fetched_span_header_full_is_inline next; \
    _ternfs_fetched_span_header_full_put_is_inline(ctx, &(prev), &(next), x)

#define ternfs_fetched_span_header_full_put_end(ctx, prev, next) \
    { struct ternfs_fetched_span_header_full_is_inline* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetched_span_header_full_end* next __attribute__((unused)) = NULL

struct ternfs_blacklist_entry_start;
#define ternfs_blacklist_entry_get_start(ctx, start) struct ternfs_blacklist_entry_start* start = NULL

#define ternfs_blacklist_entry_get_failure_domain(ctx, prev, next) \
    { struct ternfs_blacklist_entry_start** __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_failure_domain_start* next = NULL

struct ternfs_blacklist_entry_block_service { u64 x; };
static inline void _ternfs_blacklist_entry_get_block_service(struct ternfs_bincode_get_ctx* ctx, struct ternfs_failure_domain_end** prev, struct ternfs_blacklist_entry_block_service* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_blacklist_entry_get_block_service(ctx, prev, next) \
    struct ternfs_blacklist_entry_block_service next; \
    _ternfs_blacklist_entry_get_block_service(ctx, &(prev), &(next))

struct ternfs_blacklist_entry_end;
#define ternfs_blacklist_entry_get_end(ctx, prev, next) \
    { struct ternfs_blacklist_entry_block_service* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_blacklist_entry_end* next = NULL

static inline void ternfs_blacklist_entry_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_blacklist_entry_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_blacklist_entry_put_start(ctx, start) struct ternfs_blacklist_entry_start* start = NULL

static inline void _ternfs_blacklist_entry_put_block_service(struct ternfs_bincode_put_ctx* ctx, struct ternfs_blacklist_entry_start** prev, struct ternfs_blacklist_entry_block_service* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_blacklist_entry_put_block_service(ctx, prev, next, x) \
    struct ternfs_blacklist_entry_block_service next; \
    _ternfs_blacklist_entry_put_block_service(ctx, &(prev), &(next), x)

#define ternfs_blacklist_entry_put_end(ctx, prev, next) \
    { struct ternfs_blacklist_entry_block_service* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_blacklist_entry_end* next __attribute__((unused)) = NULL

#define TERNFS_EDGE_MAX_SIZE 281
struct ternfs_edge_start;
#define ternfs_edge_get_start(ctx, start) struct ternfs_edge_start* start = NULL

struct ternfs_edge_current { bool x; };
static inline void _ternfs_edge_get_current(struct ternfs_bincode_get_ctx* ctx, struct ternfs_edge_start** prev, struct ternfs_edge_current* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(bool*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_edge_get_current(ctx, prev, next) \
    struct ternfs_edge_current next; \
    _ternfs_edge_get_current(ctx, &(prev), &(next))

struct ternfs_edge_target_id { u64 x; };
static inline void _ternfs_edge_get_target_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_edge_current* prev, struct ternfs_edge_target_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_edge_get_target_id(ctx, prev, next) \
    struct ternfs_edge_target_id next; \
    _ternfs_edge_get_target_id(ctx, &(prev), &(next))

struct ternfs_edge_name_hash { u64 x; };
static inline void _ternfs_edge_get_name_hash(struct ternfs_bincode_get_ctx* ctx, struct ternfs_edge_target_id* prev, struct ternfs_edge_name_hash* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_edge_get_name_hash(ctx, prev, next) \
    struct ternfs_edge_name_hash next; \
    _ternfs_edge_get_name_hash(ctx, &(prev), &(next))

struct ternfs_edge_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_edge_get_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_edge_name_hash* prev, struct ternfs_edge_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_edge_get_name(ctx, prev, next) \
    struct ternfs_edge_name next; \
    _ternfs_edge_get_name(ctx, &(prev), &(next))

struct ternfs_edge_creation_time { u64 x; };
static inline void _ternfs_edge_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_edge_name* prev, struct ternfs_edge_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_edge_get_creation_time(ctx, prev, next) \
    struct ternfs_edge_creation_time next; \
    _ternfs_edge_get_creation_time(ctx, &(prev), &(next))

struct ternfs_edge_end;
#define ternfs_edge_get_end(ctx, prev, next) \
    { struct ternfs_edge_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_edge_end* next = NULL

static inline void ternfs_edge_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_edge_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_edge_put_start(ctx, start) struct ternfs_edge_start* start = NULL

static inline void _ternfs_edge_put_current(struct ternfs_bincode_put_ctx* ctx, struct ternfs_edge_start** prev, struct ternfs_edge_current* next, bool x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(bool*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_edge_put_current(ctx, prev, next, x) \
    struct ternfs_edge_current next; \
    _ternfs_edge_put_current(ctx, &(prev), &(next), x)

static inline void _ternfs_edge_put_target_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_edge_current* prev, struct ternfs_edge_target_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_edge_put_target_id(ctx, prev, next, x) \
    struct ternfs_edge_target_id next; \
    _ternfs_edge_put_target_id(ctx, &(prev), &(next), x)

static inline void _ternfs_edge_put_name_hash(struct ternfs_bincode_put_ctx* ctx, struct ternfs_edge_target_id* prev, struct ternfs_edge_name_hash* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_edge_put_name_hash(ctx, prev, next, x) \
    struct ternfs_edge_name_hash next; \
    _ternfs_edge_put_name_hash(ctx, &(prev), &(next), x)

static inline void _ternfs_edge_put_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_edge_name_hash* prev, struct ternfs_edge_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_edge_put_name(ctx, prev, next, str, str_len) \
    struct ternfs_edge_name next; \
    _ternfs_edge_put_name(ctx, &(prev), &(next), str, str_len)

static inline void _ternfs_edge_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_edge_name* prev, struct ternfs_edge_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_edge_put_creation_time(ctx, prev, next, x) \
    struct ternfs_edge_creation_time next; \
    _ternfs_edge_put_creation_time(ctx, &(prev), &(next), x)

#define ternfs_edge_put_end(ctx, prev, next) \
    { struct ternfs_edge_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_edge_end* next __attribute__((unused)) = NULL

#define TERNFS_FULL_READ_DIR_CURSOR_MAX_SIZE 265
struct ternfs_full_read_dir_cursor_start;
#define ternfs_full_read_dir_cursor_get_start(ctx, start) struct ternfs_full_read_dir_cursor_start* start = NULL

struct ternfs_full_read_dir_cursor_current { bool x; };
static inline void _ternfs_full_read_dir_cursor_get_current(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_cursor_start** prev, struct ternfs_full_read_dir_cursor_current* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(bool*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_full_read_dir_cursor_get_current(ctx, prev, next) \
    struct ternfs_full_read_dir_cursor_current next; \
    _ternfs_full_read_dir_cursor_get_current(ctx, &(prev), &(next))

struct ternfs_full_read_dir_cursor_start_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_full_read_dir_cursor_get_start_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_cursor_current* prev, struct ternfs_full_read_dir_cursor_start_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_full_read_dir_cursor_get_start_name(ctx, prev, next) \
    struct ternfs_full_read_dir_cursor_start_name next; \
    _ternfs_full_read_dir_cursor_get_start_name(ctx, &(prev), &(next))

struct ternfs_full_read_dir_cursor_start_time { u64 x; };
static inline void _ternfs_full_read_dir_cursor_get_start_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_cursor_start_name* prev, struct ternfs_full_read_dir_cursor_start_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_full_read_dir_cursor_get_start_time(ctx, prev, next) \
    struct ternfs_full_read_dir_cursor_start_time next; \
    _ternfs_full_read_dir_cursor_get_start_time(ctx, &(prev), &(next))

struct ternfs_full_read_dir_cursor_end;
#define ternfs_full_read_dir_cursor_get_end(ctx, prev, next) \
    { struct ternfs_full_read_dir_cursor_start_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_full_read_dir_cursor_end* next = NULL

static inline void ternfs_full_read_dir_cursor_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_cursor_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_full_read_dir_cursor_put_start(ctx, start) struct ternfs_full_read_dir_cursor_start* start = NULL

static inline void _ternfs_full_read_dir_cursor_put_current(struct ternfs_bincode_put_ctx* ctx, struct ternfs_full_read_dir_cursor_start** prev, struct ternfs_full_read_dir_cursor_current* next, bool x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(bool*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_full_read_dir_cursor_put_current(ctx, prev, next, x) \
    struct ternfs_full_read_dir_cursor_current next; \
    _ternfs_full_read_dir_cursor_put_current(ctx, &(prev), &(next), x)

static inline void _ternfs_full_read_dir_cursor_put_start_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_full_read_dir_cursor_current* prev, struct ternfs_full_read_dir_cursor_start_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_full_read_dir_cursor_put_start_name(ctx, prev, next, str, str_len) \
    struct ternfs_full_read_dir_cursor_start_name next; \
    _ternfs_full_read_dir_cursor_put_start_name(ctx, &(prev), &(next), str, str_len)

static inline void _ternfs_full_read_dir_cursor_put_start_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_full_read_dir_cursor_start_name* prev, struct ternfs_full_read_dir_cursor_start_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_full_read_dir_cursor_put_start_time(ctx, prev, next, x) \
    struct ternfs_full_read_dir_cursor_start_time next; \
    _ternfs_full_read_dir_cursor_put_start_time(ctx, &(prev), &(next), x)

#define ternfs_full_read_dir_cursor_put_end(ctx, prev, next) \
    { struct ternfs_full_read_dir_cursor_start_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_full_read_dir_cursor_end* next __attribute__((unused)) = NULL

#define TERNFS_LOOKUP_REQ_MAX_SIZE 264
struct ternfs_lookup_req_start;
#define ternfs_lookup_req_get_start(ctx, start) struct ternfs_lookup_req_start* start = NULL

struct ternfs_lookup_req_dir_id { u64 x; };
static inline void _ternfs_lookup_req_get_dir_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_lookup_req_start** prev, struct ternfs_lookup_req_dir_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_lookup_req_get_dir_id(ctx, prev, next) \
    struct ternfs_lookup_req_dir_id next; \
    _ternfs_lookup_req_get_dir_id(ctx, &(prev), &(next))

struct ternfs_lookup_req_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_lookup_req_get_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_lookup_req_dir_id* prev, struct ternfs_lookup_req_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_lookup_req_get_name(ctx, prev, next) \
    struct ternfs_lookup_req_name next; \
    _ternfs_lookup_req_get_name(ctx, &(prev), &(next))

struct ternfs_lookup_req_end;
#define ternfs_lookup_req_get_end(ctx, prev, next) \
    { struct ternfs_lookup_req_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_lookup_req_end* next = NULL

static inline void ternfs_lookup_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_lookup_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_lookup_req_put_start(ctx, start) struct ternfs_lookup_req_start* start = NULL

static inline void _ternfs_lookup_req_put_dir_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_lookup_req_start** prev, struct ternfs_lookup_req_dir_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_lookup_req_put_dir_id(ctx, prev, next, x) \
    struct ternfs_lookup_req_dir_id next; \
    _ternfs_lookup_req_put_dir_id(ctx, &(prev), &(next), x)

static inline void _ternfs_lookup_req_put_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_lookup_req_dir_id* prev, struct ternfs_lookup_req_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_lookup_req_put_name(ctx, prev, next, str, str_len) \
    struct ternfs_lookup_req_name next; \
    _ternfs_lookup_req_put_name(ctx, &(prev), &(next), str, str_len)

#define ternfs_lookup_req_put_end(ctx, prev, next) \
    { struct ternfs_lookup_req_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_lookup_req_end* next __attribute__((unused)) = NULL

#define TERNFS_LOOKUP_RESP_SIZE 16
struct ternfs_lookup_resp_start;
#define ternfs_lookup_resp_get_start(ctx, start) struct ternfs_lookup_resp_start* start = NULL

struct ternfs_lookup_resp_target_id { u64 x; };
static inline void _ternfs_lookup_resp_get_target_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_lookup_resp_start** prev, struct ternfs_lookup_resp_target_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_lookup_resp_get_target_id(ctx, prev, next) \
    struct ternfs_lookup_resp_target_id next; \
    _ternfs_lookup_resp_get_target_id(ctx, &(prev), &(next))

struct ternfs_lookup_resp_creation_time { u64 x; };
static inline void _ternfs_lookup_resp_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_lookup_resp_target_id* prev, struct ternfs_lookup_resp_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_lookup_resp_get_creation_time(ctx, prev, next) \
    struct ternfs_lookup_resp_creation_time next; \
    _ternfs_lookup_resp_get_creation_time(ctx, &(prev), &(next))

struct ternfs_lookup_resp_end;
#define ternfs_lookup_resp_get_end(ctx, prev, next) \
    { struct ternfs_lookup_resp_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_lookup_resp_end* next = NULL

static inline void ternfs_lookup_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_lookup_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_lookup_resp_put_start(ctx, start) struct ternfs_lookup_resp_start* start = NULL

static inline void _ternfs_lookup_resp_put_target_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_lookup_resp_start** prev, struct ternfs_lookup_resp_target_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_lookup_resp_put_target_id(ctx, prev, next, x) \
    struct ternfs_lookup_resp_target_id next; \
    _ternfs_lookup_resp_put_target_id(ctx, &(prev), &(next), x)

static inline void _ternfs_lookup_resp_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_lookup_resp_target_id* prev, struct ternfs_lookup_resp_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_lookup_resp_put_creation_time(ctx, prev, next, x) \
    struct ternfs_lookup_resp_creation_time next; \
    _ternfs_lookup_resp_put_creation_time(ctx, &(prev), &(next), x)

#define ternfs_lookup_resp_put_end(ctx, prev, next) \
    { struct ternfs_lookup_resp_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_lookup_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_STAT_FILE_REQ_SIZE 8
struct ternfs_stat_file_req_start;
#define ternfs_stat_file_req_get_start(ctx, start) struct ternfs_stat_file_req_start* start = NULL

struct ternfs_stat_file_req_id { u64 x; };
static inline void _ternfs_stat_file_req_get_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_file_req_start** prev, struct ternfs_stat_file_req_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_stat_file_req_get_id(ctx, prev, next) \
    struct ternfs_stat_file_req_id next; \
    _ternfs_stat_file_req_get_id(ctx, &(prev), &(next))

struct ternfs_stat_file_req_end;
#define ternfs_stat_file_req_get_end(ctx, prev, next) \
    { struct ternfs_stat_file_req_id* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_stat_file_req_end* next = NULL

static inline void ternfs_stat_file_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_file_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_stat_file_req_put_start(ctx, start) struct ternfs_stat_file_req_start* start = NULL

static inline void _ternfs_stat_file_req_put_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_stat_file_req_start** prev, struct ternfs_stat_file_req_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_stat_file_req_put_id(ctx, prev, next, x) \
    struct ternfs_stat_file_req_id next; \
    _ternfs_stat_file_req_put_id(ctx, &(prev), &(next), x)

#define ternfs_stat_file_req_put_end(ctx, prev, next) \
    { struct ternfs_stat_file_req_id* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_stat_file_req_end* next __attribute__((unused)) = NULL

#define TERNFS_STAT_FILE_RESP_SIZE 24
struct ternfs_stat_file_resp_start;
#define ternfs_stat_file_resp_get_start(ctx, start) struct ternfs_stat_file_resp_start* start = NULL

struct ternfs_stat_file_resp_mtime { u64 x; };
static inline void _ternfs_stat_file_resp_get_mtime(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_file_resp_start** prev, struct ternfs_stat_file_resp_mtime* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_stat_file_resp_get_mtime(ctx, prev, next) \
    struct ternfs_stat_file_resp_mtime next; \
    _ternfs_stat_file_resp_get_mtime(ctx, &(prev), &(next))

struct ternfs_stat_file_resp_atime { u64 x; };
static inline void _ternfs_stat_file_resp_get_atime(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_file_resp_mtime* prev, struct ternfs_stat_file_resp_atime* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_stat_file_resp_get_atime(ctx, prev, next) \
    struct ternfs_stat_file_resp_atime next; \
    _ternfs_stat_file_resp_get_atime(ctx, &(prev), &(next))

struct ternfs_stat_file_resp_size { u64 x; };
static inline void _ternfs_stat_file_resp_get_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_file_resp_atime* prev, struct ternfs_stat_file_resp_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_stat_file_resp_get_size(ctx, prev, next) \
    struct ternfs_stat_file_resp_size next; \
    _ternfs_stat_file_resp_get_size(ctx, &(prev), &(next))

struct ternfs_stat_file_resp_end;
#define ternfs_stat_file_resp_get_end(ctx, prev, next) \
    { struct ternfs_stat_file_resp_size* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_stat_file_resp_end* next = NULL

static inline void ternfs_stat_file_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_file_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_stat_file_resp_put_start(ctx, start) struct ternfs_stat_file_resp_start* start = NULL

static inline void _ternfs_stat_file_resp_put_mtime(struct ternfs_bincode_put_ctx* ctx, struct ternfs_stat_file_resp_start** prev, struct ternfs_stat_file_resp_mtime* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_stat_file_resp_put_mtime(ctx, prev, next, x) \
    struct ternfs_stat_file_resp_mtime next; \
    _ternfs_stat_file_resp_put_mtime(ctx, &(prev), &(next), x)

static inline void _ternfs_stat_file_resp_put_atime(struct ternfs_bincode_put_ctx* ctx, struct ternfs_stat_file_resp_mtime* prev, struct ternfs_stat_file_resp_atime* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_stat_file_resp_put_atime(ctx, prev, next, x) \
    struct ternfs_stat_file_resp_atime next; \
    _ternfs_stat_file_resp_put_atime(ctx, &(prev), &(next), x)

static inline void _ternfs_stat_file_resp_put_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_stat_file_resp_atime* prev, struct ternfs_stat_file_resp_size* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_stat_file_resp_put_size(ctx, prev, next, x) \
    struct ternfs_stat_file_resp_size next; \
    _ternfs_stat_file_resp_put_size(ctx, &(prev), &(next), x)

#define ternfs_stat_file_resp_put_end(ctx, prev, next) \
    { struct ternfs_stat_file_resp_size* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_stat_file_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_STAT_DIRECTORY_REQ_SIZE 8
struct ternfs_stat_directory_req_start;
#define ternfs_stat_directory_req_get_start(ctx, start) struct ternfs_stat_directory_req_start* start = NULL

struct ternfs_stat_directory_req_id { u64 x; };
static inline void _ternfs_stat_directory_req_get_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_directory_req_start** prev, struct ternfs_stat_directory_req_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_stat_directory_req_get_id(ctx, prev, next) \
    struct ternfs_stat_directory_req_id next; \
    _ternfs_stat_directory_req_get_id(ctx, &(prev), &(next))

struct ternfs_stat_directory_req_end;
#define ternfs_stat_directory_req_get_end(ctx, prev, next) \
    { struct ternfs_stat_directory_req_id* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_stat_directory_req_end* next = NULL

static inline void ternfs_stat_directory_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_directory_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_stat_directory_req_put_start(ctx, start) struct ternfs_stat_directory_req_start* start = NULL

static inline void _ternfs_stat_directory_req_put_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_stat_directory_req_start** prev, struct ternfs_stat_directory_req_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_stat_directory_req_put_id(ctx, prev, next, x) \
    struct ternfs_stat_directory_req_id next; \
    _ternfs_stat_directory_req_put_id(ctx, &(prev), &(next), x)

#define ternfs_stat_directory_req_put_end(ctx, prev, next) \
    { struct ternfs_stat_directory_req_id* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_stat_directory_req_end* next __attribute__((unused)) = NULL

struct ternfs_stat_directory_resp_start;
#define ternfs_stat_directory_resp_get_start(ctx, start) struct ternfs_stat_directory_resp_start* start = NULL

struct ternfs_stat_directory_resp_mtime { u64 x; };
static inline void _ternfs_stat_directory_resp_get_mtime(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_directory_resp_start** prev, struct ternfs_stat_directory_resp_mtime* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_stat_directory_resp_get_mtime(ctx, prev, next) \
    struct ternfs_stat_directory_resp_mtime next; \
    _ternfs_stat_directory_resp_get_mtime(ctx, &(prev), &(next))

struct ternfs_stat_directory_resp_owner { u64 x; };
static inline void _ternfs_stat_directory_resp_get_owner(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_directory_resp_mtime* prev, struct ternfs_stat_directory_resp_owner* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_stat_directory_resp_get_owner(ctx, prev, next) \
    struct ternfs_stat_directory_resp_owner next; \
    _ternfs_stat_directory_resp_get_owner(ctx, &(prev), &(next))

#define ternfs_stat_directory_resp_get_info(ctx, prev, next) \
    { struct ternfs_stat_directory_resp_owner* __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_directory_info_start* next = NULL

struct ternfs_stat_directory_resp_end;
#define ternfs_stat_directory_resp_get_end(ctx, prev, next) \
    { struct ternfs_directory_info_end** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_stat_directory_resp_end* next = NULL

static inline void ternfs_stat_directory_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_stat_directory_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_stat_directory_resp_put_start(ctx, start) struct ternfs_stat_directory_resp_start* start = NULL

static inline void _ternfs_stat_directory_resp_put_mtime(struct ternfs_bincode_put_ctx* ctx, struct ternfs_stat_directory_resp_start** prev, struct ternfs_stat_directory_resp_mtime* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_stat_directory_resp_put_mtime(ctx, prev, next, x) \
    struct ternfs_stat_directory_resp_mtime next; \
    _ternfs_stat_directory_resp_put_mtime(ctx, &(prev), &(next), x)

static inline void _ternfs_stat_directory_resp_put_owner(struct ternfs_bincode_put_ctx* ctx, struct ternfs_stat_directory_resp_mtime* prev, struct ternfs_stat_directory_resp_owner* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_stat_directory_resp_put_owner(ctx, prev, next, x) \
    struct ternfs_stat_directory_resp_owner next; \
    _ternfs_stat_directory_resp_put_owner(ctx, &(prev), &(next), x)

#define ternfs_stat_directory_resp_put_end(ctx, prev, next) \
    { struct ternfs_stat_directory_resp_owner* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_stat_directory_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_READ_DIR_REQ_SIZE 18
struct ternfs_read_dir_req_start;
#define ternfs_read_dir_req_get_start(ctx, start) struct ternfs_read_dir_req_start* start = NULL

struct ternfs_read_dir_req_dir_id { u64 x; };
static inline void _ternfs_read_dir_req_get_dir_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_read_dir_req_start** prev, struct ternfs_read_dir_req_dir_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_read_dir_req_get_dir_id(ctx, prev, next) \
    struct ternfs_read_dir_req_dir_id next; \
    _ternfs_read_dir_req_get_dir_id(ctx, &(prev), &(next))

struct ternfs_read_dir_req_start_hash { u64 x; };
static inline void _ternfs_read_dir_req_get_start_hash(struct ternfs_bincode_get_ctx* ctx, struct ternfs_read_dir_req_dir_id* prev, struct ternfs_read_dir_req_start_hash* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_read_dir_req_get_start_hash(ctx, prev, next) \
    struct ternfs_read_dir_req_start_hash next; \
    _ternfs_read_dir_req_get_start_hash(ctx, &(prev), &(next))

struct ternfs_read_dir_req_mtu { u16 x; };
static inline void _ternfs_read_dir_req_get_mtu(struct ternfs_bincode_get_ctx* ctx, struct ternfs_read_dir_req_start_hash* prev, struct ternfs_read_dir_req_mtu* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    }
}
#define ternfs_read_dir_req_get_mtu(ctx, prev, next) \
    struct ternfs_read_dir_req_mtu next; \
    _ternfs_read_dir_req_get_mtu(ctx, &(prev), &(next))

struct ternfs_read_dir_req_end;
#define ternfs_read_dir_req_get_end(ctx, prev, next) \
    { struct ternfs_read_dir_req_mtu* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_read_dir_req_end* next = NULL

static inline void ternfs_read_dir_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_read_dir_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_read_dir_req_put_start(ctx, start) struct ternfs_read_dir_req_start* start = NULL

static inline void _ternfs_read_dir_req_put_dir_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_read_dir_req_start** prev, struct ternfs_read_dir_req_dir_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_read_dir_req_put_dir_id(ctx, prev, next, x) \
    struct ternfs_read_dir_req_dir_id next; \
    _ternfs_read_dir_req_put_dir_id(ctx, &(prev), &(next), x)

static inline void _ternfs_read_dir_req_put_start_hash(struct ternfs_bincode_put_ctx* ctx, struct ternfs_read_dir_req_dir_id* prev, struct ternfs_read_dir_req_start_hash* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_read_dir_req_put_start_hash(ctx, prev, next, x) \
    struct ternfs_read_dir_req_start_hash next; \
    _ternfs_read_dir_req_put_start_hash(ctx, &(prev), &(next), x)

static inline void _ternfs_read_dir_req_put_mtu(struct ternfs_bincode_put_ctx* ctx, struct ternfs_read_dir_req_start_hash* prev, struct ternfs_read_dir_req_mtu* next, u16 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(x, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_read_dir_req_put_mtu(ctx, prev, next, x) \
    struct ternfs_read_dir_req_mtu next; \
    _ternfs_read_dir_req_put_mtu(ctx, &(prev), &(next), x)

#define ternfs_read_dir_req_put_end(ctx, prev, next) \
    { struct ternfs_read_dir_req_mtu* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_read_dir_req_end* next __attribute__((unused)) = NULL

struct ternfs_read_dir_resp_start;
#define ternfs_read_dir_resp_get_start(ctx, start) struct ternfs_read_dir_resp_start* start = NULL

struct ternfs_read_dir_resp_next_hash { u64 x; };
static inline void _ternfs_read_dir_resp_get_next_hash(struct ternfs_bincode_get_ctx* ctx, struct ternfs_read_dir_resp_start** prev, struct ternfs_read_dir_resp_next_hash* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_read_dir_resp_get_next_hash(ctx, prev, next) \
    struct ternfs_read_dir_resp_next_hash next; \
    _ternfs_read_dir_resp_get_next_hash(ctx, &(prev), &(next))

struct ternfs_read_dir_resp_results { u16 len; };
static inline void _ternfs_read_dir_resp_get_results(struct ternfs_bincode_get_ctx* ctx, struct ternfs_read_dir_resp_next_hash* prev, struct ternfs_read_dir_resp_results* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_read_dir_resp_get_results(ctx, prev, next) \
    struct ternfs_read_dir_resp_results next; \
    _ternfs_read_dir_resp_get_results(ctx, &(prev), &(next))

struct ternfs_read_dir_resp_end;
#define ternfs_read_dir_resp_get_end(ctx, prev, next) \
    { struct ternfs_read_dir_resp_results* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_read_dir_resp_end* next = NULL

static inline void ternfs_read_dir_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_read_dir_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_read_dir_resp_put_start(ctx, start) struct ternfs_read_dir_resp_start* start = NULL

static inline void _ternfs_read_dir_resp_put_next_hash(struct ternfs_bincode_put_ctx* ctx, struct ternfs_read_dir_resp_start** prev, struct ternfs_read_dir_resp_next_hash* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_read_dir_resp_put_next_hash(ctx, prev, next, x) \
    struct ternfs_read_dir_resp_next_hash next; \
    _ternfs_read_dir_resp_put_next_hash(ctx, &(prev), &(next), x)

static inline void _ternfs_read_dir_resp_put_results(struct ternfs_bincode_put_ctx* ctx, struct ternfs_read_dir_resp_next_hash* prev, struct ternfs_read_dir_resp_results* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_read_dir_resp_put_results(ctx, prev, next, len) \
    struct ternfs_read_dir_resp_results next; \
    _ternfs_read_dir_resp_put_results(ctx, &(prev), &(next), len)

#define ternfs_read_dir_resp_put_end(ctx, prev, next) \
    { struct ternfs_read_dir_resp_results* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_read_dir_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_CONSTRUCT_FILE_REQ_MAX_SIZE 257
struct ternfs_construct_file_req_start;
#define ternfs_construct_file_req_get_start(ctx, start) struct ternfs_construct_file_req_start* start = NULL

struct ternfs_construct_file_req_type { u8 x; };
static inline void _ternfs_construct_file_req_get_type(struct ternfs_bincode_get_ctx* ctx, struct ternfs_construct_file_req_start** prev, struct ternfs_construct_file_req_type* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_construct_file_req_get_type(ctx, prev, next) \
    struct ternfs_construct_file_req_type next; \
    _ternfs_construct_file_req_get_type(ctx, &(prev), &(next))

struct ternfs_construct_file_req_note { struct ternfs_bincode_bytes str; };
static inline void _ternfs_construct_file_req_get_note(struct ternfs_bincode_get_ctx* ctx, struct ternfs_construct_file_req_type* prev, struct ternfs_construct_file_req_note* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_construct_file_req_get_note(ctx, prev, next) \
    struct ternfs_construct_file_req_note next; \
    _ternfs_construct_file_req_get_note(ctx, &(prev), &(next))

struct ternfs_construct_file_req_end;
#define ternfs_construct_file_req_get_end(ctx, prev, next) \
    { struct ternfs_construct_file_req_note* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_construct_file_req_end* next = NULL

static inline void ternfs_construct_file_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_construct_file_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_construct_file_req_put_start(ctx, start) struct ternfs_construct_file_req_start* start = NULL

static inline void _ternfs_construct_file_req_put_type(struct ternfs_bincode_put_ctx* ctx, struct ternfs_construct_file_req_start** prev, struct ternfs_construct_file_req_type* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_construct_file_req_put_type(ctx, prev, next, x) \
    struct ternfs_construct_file_req_type next; \
    _ternfs_construct_file_req_put_type(ctx, &(prev), &(next), x)

static inline void _ternfs_construct_file_req_put_note(struct ternfs_bincode_put_ctx* ctx, struct ternfs_construct_file_req_type* prev, struct ternfs_construct_file_req_note* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_construct_file_req_put_note(ctx, prev, next, str, str_len) \
    struct ternfs_construct_file_req_note next; \
    _ternfs_construct_file_req_put_note(ctx, &(prev), &(next), str, str_len)

#define ternfs_construct_file_req_put_end(ctx, prev, next) \
    { struct ternfs_construct_file_req_note* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_construct_file_req_end* next __attribute__((unused)) = NULL

#define TERNFS_CONSTRUCT_FILE_RESP_SIZE 16
struct ternfs_construct_file_resp_start;
#define ternfs_construct_file_resp_get_start(ctx, start) struct ternfs_construct_file_resp_start* start = NULL

struct ternfs_construct_file_resp_id { u64 x; };
static inline void _ternfs_construct_file_resp_get_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_construct_file_resp_start** prev, struct ternfs_construct_file_resp_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_construct_file_resp_get_id(ctx, prev, next) \
    struct ternfs_construct_file_resp_id next; \
    _ternfs_construct_file_resp_get_id(ctx, &(prev), &(next))

struct ternfs_construct_file_resp_cookie { u64 x; };
static inline void _ternfs_construct_file_resp_get_cookie(struct ternfs_bincode_get_ctx* ctx, struct ternfs_construct_file_resp_id* prev, struct ternfs_construct_file_resp_cookie* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_construct_file_resp_get_cookie(ctx, prev, next) \
    struct ternfs_construct_file_resp_cookie next; \
    _ternfs_construct_file_resp_get_cookie(ctx, &(prev), &(next))

struct ternfs_construct_file_resp_end;
#define ternfs_construct_file_resp_get_end(ctx, prev, next) \
    { struct ternfs_construct_file_resp_cookie* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_construct_file_resp_end* next = NULL

static inline void ternfs_construct_file_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_construct_file_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_construct_file_resp_put_start(ctx, start) struct ternfs_construct_file_resp_start* start = NULL

static inline void _ternfs_construct_file_resp_put_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_construct_file_resp_start** prev, struct ternfs_construct_file_resp_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_construct_file_resp_put_id(ctx, prev, next, x) \
    struct ternfs_construct_file_resp_id next; \
    _ternfs_construct_file_resp_put_id(ctx, &(prev), &(next), x)

static inline void _ternfs_construct_file_resp_put_cookie(struct ternfs_bincode_put_ctx* ctx, struct ternfs_construct_file_resp_id* prev, struct ternfs_construct_file_resp_cookie* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_construct_file_resp_put_cookie(ctx, prev, next, x) \
    struct ternfs_construct_file_resp_cookie next; \
    _ternfs_construct_file_resp_put_cookie(ctx, &(prev), &(next), x)

#define ternfs_construct_file_resp_put_end(ctx, prev, next) \
    { struct ternfs_construct_file_resp_cookie* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_construct_file_resp_end* next __attribute__((unused)) = NULL

struct ternfs_add_span_initiate_req_start;
#define ternfs_add_span_initiate_req_get_start(ctx, start) struct ternfs_add_span_initiate_req_start* start = NULL

struct ternfs_add_span_initiate_req_file_id { u64 x; };
static inline void _ternfs_add_span_initiate_req_get_file_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_start** prev, struct ternfs_add_span_initiate_req_file_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_span_initiate_req_get_file_id(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_file_id next; \
    _ternfs_add_span_initiate_req_get_file_id(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_cookie { u64 x; };
static inline void _ternfs_add_span_initiate_req_get_cookie(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_file_id* prev, struct ternfs_add_span_initiate_req_cookie* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_span_initiate_req_get_cookie(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_cookie next; \
    _ternfs_add_span_initiate_req_get_cookie(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_byte_offset { u64 x; };
static inline void _ternfs_add_span_initiate_req_get_byte_offset(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_cookie* prev, struct ternfs_add_span_initiate_req_byte_offset* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_span_initiate_req_get_byte_offset(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_byte_offset next; \
    _ternfs_add_span_initiate_req_get_byte_offset(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_size { u32 x; };
static inline void _ternfs_add_span_initiate_req_get_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_byte_offset* prev, struct ternfs_add_span_initiate_req_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_add_span_initiate_req_get_size(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_size next; \
    _ternfs_add_span_initiate_req_get_size(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_crc { u32 x; };
static inline void _ternfs_add_span_initiate_req_get_crc(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_size* prev, struct ternfs_add_span_initiate_req_crc* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_add_span_initiate_req_get_crc(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_crc next; \
    _ternfs_add_span_initiate_req_get_crc(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_storage_class { u8 x; };
static inline void _ternfs_add_span_initiate_req_get_storage_class(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_crc* prev, struct ternfs_add_span_initiate_req_storage_class* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_add_span_initiate_req_get_storage_class(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_storage_class next; \
    _ternfs_add_span_initiate_req_get_storage_class(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_blacklist { u16 len; };
static inline void _ternfs_add_span_initiate_req_get_blacklist(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_storage_class* prev, struct ternfs_add_span_initiate_req_blacklist* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_add_span_initiate_req_get_blacklist(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_blacklist next; \
    _ternfs_add_span_initiate_req_get_blacklist(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_parity { u8 x; };
static inline void _ternfs_add_span_initiate_req_get_parity(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_blacklist* prev, struct ternfs_add_span_initiate_req_parity* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_add_span_initiate_req_get_parity(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_parity next; \
    _ternfs_add_span_initiate_req_get_parity(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_stripes { u8 x; };
static inline void _ternfs_add_span_initiate_req_get_stripes(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_parity* prev, struct ternfs_add_span_initiate_req_stripes* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_add_span_initiate_req_get_stripes(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_stripes next; \
    _ternfs_add_span_initiate_req_get_stripes(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_cell_size { u32 x; };
static inline void _ternfs_add_span_initiate_req_get_cell_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_stripes* prev, struct ternfs_add_span_initiate_req_cell_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_add_span_initiate_req_get_cell_size(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_cell_size next; \
    _ternfs_add_span_initiate_req_get_cell_size(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_crcs { u16 len; };
static inline void _ternfs_add_span_initiate_req_get_crcs(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_cell_size* prev, struct ternfs_add_span_initiate_req_crcs* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_add_span_initiate_req_get_crcs(ctx, prev, next) \
    struct ternfs_add_span_initiate_req_crcs next; \
    _ternfs_add_span_initiate_req_get_crcs(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_req_end;
#define ternfs_add_span_initiate_req_get_end(ctx, prev, next) \
    { struct ternfs_add_span_initiate_req_crcs* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_span_initiate_req_end* next = NULL

static inline void ternfs_add_span_initiate_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_add_span_initiate_req_put_start(ctx, start) struct ternfs_add_span_initiate_req_start* start = NULL

static inline void _ternfs_add_span_initiate_req_put_file_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_start** prev, struct ternfs_add_span_initiate_req_file_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_span_initiate_req_put_file_id(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_req_file_id next; \
    _ternfs_add_span_initiate_req_put_file_id(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_req_put_cookie(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_file_id* prev, struct ternfs_add_span_initiate_req_cookie* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_span_initiate_req_put_cookie(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_req_cookie next; \
    _ternfs_add_span_initiate_req_put_cookie(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_req_put_byte_offset(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_cookie* prev, struct ternfs_add_span_initiate_req_byte_offset* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_span_initiate_req_put_byte_offset(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_req_byte_offset next; \
    _ternfs_add_span_initiate_req_put_byte_offset(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_req_put_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_byte_offset* prev, struct ternfs_add_span_initiate_req_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_add_span_initiate_req_put_size(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_req_size next; \
    _ternfs_add_span_initiate_req_put_size(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_req_put_crc(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_size* prev, struct ternfs_add_span_initiate_req_crc* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_add_span_initiate_req_put_crc(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_req_crc next; \
    _ternfs_add_span_initiate_req_put_crc(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_req_put_storage_class(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_crc* prev, struct ternfs_add_span_initiate_req_storage_class* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_add_span_initiate_req_put_storage_class(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_req_storage_class next; \
    _ternfs_add_span_initiate_req_put_storage_class(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_req_put_blacklist(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_storage_class* prev, struct ternfs_add_span_initiate_req_blacklist* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_add_span_initiate_req_put_blacklist(ctx, prev, next, len) \
    struct ternfs_add_span_initiate_req_blacklist next; \
    _ternfs_add_span_initiate_req_put_blacklist(ctx, &(prev), &(next), len)

static inline void _ternfs_add_span_initiate_req_put_parity(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_blacklist* prev, struct ternfs_add_span_initiate_req_parity* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_add_span_initiate_req_put_parity(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_req_parity next; \
    _ternfs_add_span_initiate_req_put_parity(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_req_put_stripes(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_parity* prev, struct ternfs_add_span_initiate_req_stripes* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_add_span_initiate_req_put_stripes(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_req_stripes next; \
    _ternfs_add_span_initiate_req_put_stripes(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_req_put_cell_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_stripes* prev, struct ternfs_add_span_initiate_req_cell_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_add_span_initiate_req_put_cell_size(ctx, prev, next, x) \
    struct ternfs_add_span_initiate_req_cell_size next; \
    _ternfs_add_span_initiate_req_put_cell_size(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_initiate_req_put_crcs(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_req_cell_size* prev, struct ternfs_add_span_initiate_req_crcs* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_add_span_initiate_req_put_crcs(ctx, prev, next, len) \
    struct ternfs_add_span_initiate_req_crcs next; \
    _ternfs_add_span_initiate_req_put_crcs(ctx, &(prev), &(next), len)

#define ternfs_add_span_initiate_req_put_end(ctx, prev, next) \
    { struct ternfs_add_span_initiate_req_crcs* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_span_initiate_req_end* next __attribute__((unused)) = NULL

struct ternfs_add_span_initiate_resp_start;
#define ternfs_add_span_initiate_resp_get_start(ctx, start) struct ternfs_add_span_initiate_resp_start* start = NULL

struct ternfs_add_span_initiate_resp_blocks { u16 len; };
static inline void _ternfs_add_span_initiate_resp_get_blocks(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_resp_start** prev, struct ternfs_add_span_initiate_resp_blocks* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_add_span_initiate_resp_get_blocks(ctx, prev, next) \
    struct ternfs_add_span_initiate_resp_blocks next; \
    _ternfs_add_span_initiate_resp_get_blocks(ctx, &(prev), &(next))

struct ternfs_add_span_initiate_resp_end;
#define ternfs_add_span_initiate_resp_get_end(ctx, prev, next) \
    { struct ternfs_add_span_initiate_resp_blocks* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_span_initiate_resp_end* next = NULL

static inline void ternfs_add_span_initiate_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_initiate_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_add_span_initiate_resp_put_start(ctx, start) struct ternfs_add_span_initiate_resp_start* start = NULL

static inline void _ternfs_add_span_initiate_resp_put_blocks(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_initiate_resp_start** prev, struct ternfs_add_span_initiate_resp_blocks* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_add_span_initiate_resp_put_blocks(ctx, prev, next, len) \
    struct ternfs_add_span_initiate_resp_blocks next; \
    _ternfs_add_span_initiate_resp_put_blocks(ctx, &(prev), &(next), len)

#define ternfs_add_span_initiate_resp_put_end(ctx, prev, next) \
    { struct ternfs_add_span_initiate_resp_blocks* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_span_initiate_resp_end* next __attribute__((unused)) = NULL

struct ternfs_add_span_certify_req_start;
#define ternfs_add_span_certify_req_get_start(ctx, start) struct ternfs_add_span_certify_req_start* start = NULL

struct ternfs_add_span_certify_req_file_id { u64 x; };
static inline void _ternfs_add_span_certify_req_get_file_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_certify_req_start** prev, struct ternfs_add_span_certify_req_file_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_span_certify_req_get_file_id(ctx, prev, next) \
    struct ternfs_add_span_certify_req_file_id next; \
    _ternfs_add_span_certify_req_get_file_id(ctx, &(prev), &(next))

struct ternfs_add_span_certify_req_cookie { u64 x; };
static inline void _ternfs_add_span_certify_req_get_cookie(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_certify_req_file_id* prev, struct ternfs_add_span_certify_req_cookie* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_span_certify_req_get_cookie(ctx, prev, next) \
    struct ternfs_add_span_certify_req_cookie next; \
    _ternfs_add_span_certify_req_get_cookie(ctx, &(prev), &(next))

struct ternfs_add_span_certify_req_byte_offset { u64 x; };
static inline void _ternfs_add_span_certify_req_get_byte_offset(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_certify_req_cookie* prev, struct ternfs_add_span_certify_req_byte_offset* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_span_certify_req_get_byte_offset(ctx, prev, next) \
    struct ternfs_add_span_certify_req_byte_offset next; \
    _ternfs_add_span_certify_req_get_byte_offset(ctx, &(prev), &(next))

struct ternfs_add_span_certify_req_proofs { u16 len; };
static inline void _ternfs_add_span_certify_req_get_proofs(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_certify_req_byte_offset* prev, struct ternfs_add_span_certify_req_proofs* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_add_span_certify_req_get_proofs(ctx, prev, next) \
    struct ternfs_add_span_certify_req_proofs next; \
    _ternfs_add_span_certify_req_get_proofs(ctx, &(prev), &(next))

struct ternfs_add_span_certify_req_end;
#define ternfs_add_span_certify_req_get_end(ctx, prev, next) \
    { struct ternfs_add_span_certify_req_proofs* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_span_certify_req_end* next = NULL

static inline void ternfs_add_span_certify_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_certify_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_add_span_certify_req_put_start(ctx, start) struct ternfs_add_span_certify_req_start* start = NULL

static inline void _ternfs_add_span_certify_req_put_file_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_certify_req_start** prev, struct ternfs_add_span_certify_req_file_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_span_certify_req_put_file_id(ctx, prev, next, x) \
    struct ternfs_add_span_certify_req_file_id next; \
    _ternfs_add_span_certify_req_put_file_id(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_certify_req_put_cookie(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_certify_req_file_id* prev, struct ternfs_add_span_certify_req_cookie* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_span_certify_req_put_cookie(ctx, prev, next, x) \
    struct ternfs_add_span_certify_req_cookie next; \
    _ternfs_add_span_certify_req_put_cookie(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_certify_req_put_byte_offset(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_certify_req_cookie* prev, struct ternfs_add_span_certify_req_byte_offset* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_span_certify_req_put_byte_offset(ctx, prev, next, x) \
    struct ternfs_add_span_certify_req_byte_offset next; \
    _ternfs_add_span_certify_req_put_byte_offset(ctx, &(prev), &(next), x)

static inline void _ternfs_add_span_certify_req_put_proofs(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_span_certify_req_byte_offset* prev, struct ternfs_add_span_certify_req_proofs* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_add_span_certify_req_put_proofs(ctx, prev, next, len) \
    struct ternfs_add_span_certify_req_proofs next; \
    _ternfs_add_span_certify_req_put_proofs(ctx, &(prev), &(next), len)

#define ternfs_add_span_certify_req_put_end(ctx, prev, next) \
    { struct ternfs_add_span_certify_req_proofs* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_span_certify_req_end* next __attribute__((unused)) = NULL

#define TERNFS_ADD_SPAN_CERTIFY_RESP_SIZE 0
struct ternfs_add_span_certify_resp_start;
#define ternfs_add_span_certify_resp_get_start(ctx, start) struct ternfs_add_span_certify_resp_start* start = NULL

struct ternfs_add_span_certify_resp_end;
#define ternfs_add_span_certify_resp_get_end(ctx, prev, next) \
    { struct ternfs_add_span_certify_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_span_certify_resp_end* next = NULL

static inline void ternfs_add_span_certify_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_span_certify_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_add_span_certify_resp_put_start(ctx, start) struct ternfs_add_span_certify_resp_start* start = NULL

#define ternfs_add_span_certify_resp_put_end(ctx, prev, next) \
    { struct ternfs_add_span_certify_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_span_certify_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_LINK_FILE_REQ_MAX_SIZE 280
struct ternfs_link_file_req_start;
#define ternfs_link_file_req_get_start(ctx, start) struct ternfs_link_file_req_start* start = NULL

struct ternfs_link_file_req_file_id { u64 x; };
static inline void _ternfs_link_file_req_get_file_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_link_file_req_start** prev, struct ternfs_link_file_req_file_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_link_file_req_get_file_id(ctx, prev, next) \
    struct ternfs_link_file_req_file_id next; \
    _ternfs_link_file_req_get_file_id(ctx, &(prev), &(next))

struct ternfs_link_file_req_cookie { u64 x; };
static inline void _ternfs_link_file_req_get_cookie(struct ternfs_bincode_get_ctx* ctx, struct ternfs_link_file_req_file_id* prev, struct ternfs_link_file_req_cookie* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_link_file_req_get_cookie(ctx, prev, next) \
    struct ternfs_link_file_req_cookie next; \
    _ternfs_link_file_req_get_cookie(ctx, &(prev), &(next))

struct ternfs_link_file_req_owner_id { u64 x; };
static inline void _ternfs_link_file_req_get_owner_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_link_file_req_cookie* prev, struct ternfs_link_file_req_owner_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_link_file_req_get_owner_id(ctx, prev, next) \
    struct ternfs_link_file_req_owner_id next; \
    _ternfs_link_file_req_get_owner_id(ctx, &(prev), &(next))

struct ternfs_link_file_req_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_link_file_req_get_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_link_file_req_owner_id* prev, struct ternfs_link_file_req_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_link_file_req_get_name(ctx, prev, next) \
    struct ternfs_link_file_req_name next; \
    _ternfs_link_file_req_get_name(ctx, &(prev), &(next))

struct ternfs_link_file_req_end;
#define ternfs_link_file_req_get_end(ctx, prev, next) \
    { struct ternfs_link_file_req_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_link_file_req_end* next = NULL

static inline void ternfs_link_file_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_link_file_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_link_file_req_put_start(ctx, start) struct ternfs_link_file_req_start* start = NULL

static inline void _ternfs_link_file_req_put_file_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_link_file_req_start** prev, struct ternfs_link_file_req_file_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_link_file_req_put_file_id(ctx, prev, next, x) \
    struct ternfs_link_file_req_file_id next; \
    _ternfs_link_file_req_put_file_id(ctx, &(prev), &(next), x)

static inline void _ternfs_link_file_req_put_cookie(struct ternfs_bincode_put_ctx* ctx, struct ternfs_link_file_req_file_id* prev, struct ternfs_link_file_req_cookie* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_link_file_req_put_cookie(ctx, prev, next, x) \
    struct ternfs_link_file_req_cookie next; \
    _ternfs_link_file_req_put_cookie(ctx, &(prev), &(next), x)

static inline void _ternfs_link_file_req_put_owner_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_link_file_req_cookie* prev, struct ternfs_link_file_req_owner_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_link_file_req_put_owner_id(ctx, prev, next, x) \
    struct ternfs_link_file_req_owner_id next; \
    _ternfs_link_file_req_put_owner_id(ctx, &(prev), &(next), x)

static inline void _ternfs_link_file_req_put_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_link_file_req_owner_id* prev, struct ternfs_link_file_req_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_link_file_req_put_name(ctx, prev, next, str, str_len) \
    struct ternfs_link_file_req_name next; \
    _ternfs_link_file_req_put_name(ctx, &(prev), &(next), str, str_len)

#define ternfs_link_file_req_put_end(ctx, prev, next) \
    { struct ternfs_link_file_req_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_link_file_req_end* next __attribute__((unused)) = NULL

#define TERNFS_LINK_FILE_RESP_SIZE 8
struct ternfs_link_file_resp_start;
#define ternfs_link_file_resp_get_start(ctx, start) struct ternfs_link_file_resp_start* start = NULL

struct ternfs_link_file_resp_creation_time { u64 x; };
static inline void _ternfs_link_file_resp_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_link_file_resp_start** prev, struct ternfs_link_file_resp_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_link_file_resp_get_creation_time(ctx, prev, next) \
    struct ternfs_link_file_resp_creation_time next; \
    _ternfs_link_file_resp_get_creation_time(ctx, &(prev), &(next))

struct ternfs_link_file_resp_end;
#define ternfs_link_file_resp_get_end(ctx, prev, next) \
    { struct ternfs_link_file_resp_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_link_file_resp_end* next = NULL

static inline void ternfs_link_file_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_link_file_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_link_file_resp_put_start(ctx, start) struct ternfs_link_file_resp_start* start = NULL

static inline void _ternfs_link_file_resp_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_link_file_resp_start** prev, struct ternfs_link_file_resp_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_link_file_resp_put_creation_time(ctx, prev, next, x) \
    struct ternfs_link_file_resp_creation_time next; \
    _ternfs_link_file_resp_put_creation_time(ctx, &(prev), &(next), x)

#define ternfs_link_file_resp_put_end(ctx, prev, next) \
    { struct ternfs_link_file_resp_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_link_file_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_SOFT_UNLINK_FILE_REQ_MAX_SIZE 280
struct ternfs_soft_unlink_file_req_start;
#define ternfs_soft_unlink_file_req_get_start(ctx, start) struct ternfs_soft_unlink_file_req_start* start = NULL

struct ternfs_soft_unlink_file_req_owner_id { u64 x; };
static inline void _ternfs_soft_unlink_file_req_get_owner_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_file_req_start** prev, struct ternfs_soft_unlink_file_req_owner_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_soft_unlink_file_req_get_owner_id(ctx, prev, next) \
    struct ternfs_soft_unlink_file_req_owner_id next; \
    _ternfs_soft_unlink_file_req_get_owner_id(ctx, &(prev), &(next))

struct ternfs_soft_unlink_file_req_file_id { u64 x; };
static inline void _ternfs_soft_unlink_file_req_get_file_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_file_req_owner_id* prev, struct ternfs_soft_unlink_file_req_file_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_soft_unlink_file_req_get_file_id(ctx, prev, next) \
    struct ternfs_soft_unlink_file_req_file_id next; \
    _ternfs_soft_unlink_file_req_get_file_id(ctx, &(prev), &(next))

struct ternfs_soft_unlink_file_req_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_soft_unlink_file_req_get_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_file_req_file_id* prev, struct ternfs_soft_unlink_file_req_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_soft_unlink_file_req_get_name(ctx, prev, next) \
    struct ternfs_soft_unlink_file_req_name next; \
    _ternfs_soft_unlink_file_req_get_name(ctx, &(prev), &(next))

struct ternfs_soft_unlink_file_req_creation_time { u64 x; };
static inline void _ternfs_soft_unlink_file_req_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_file_req_name* prev, struct ternfs_soft_unlink_file_req_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_soft_unlink_file_req_get_creation_time(ctx, prev, next) \
    struct ternfs_soft_unlink_file_req_creation_time next; \
    _ternfs_soft_unlink_file_req_get_creation_time(ctx, &(prev), &(next))

struct ternfs_soft_unlink_file_req_end;
#define ternfs_soft_unlink_file_req_get_end(ctx, prev, next) \
    { struct ternfs_soft_unlink_file_req_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_soft_unlink_file_req_end* next = NULL

static inline void ternfs_soft_unlink_file_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_file_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_soft_unlink_file_req_put_start(ctx, start) struct ternfs_soft_unlink_file_req_start* start = NULL

static inline void _ternfs_soft_unlink_file_req_put_owner_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_soft_unlink_file_req_start** prev, struct ternfs_soft_unlink_file_req_owner_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_soft_unlink_file_req_put_owner_id(ctx, prev, next, x) \
    struct ternfs_soft_unlink_file_req_owner_id next; \
    _ternfs_soft_unlink_file_req_put_owner_id(ctx, &(prev), &(next), x)

static inline void _ternfs_soft_unlink_file_req_put_file_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_soft_unlink_file_req_owner_id* prev, struct ternfs_soft_unlink_file_req_file_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_soft_unlink_file_req_put_file_id(ctx, prev, next, x) \
    struct ternfs_soft_unlink_file_req_file_id next; \
    _ternfs_soft_unlink_file_req_put_file_id(ctx, &(prev), &(next), x)

static inline void _ternfs_soft_unlink_file_req_put_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_soft_unlink_file_req_file_id* prev, struct ternfs_soft_unlink_file_req_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_soft_unlink_file_req_put_name(ctx, prev, next, str, str_len) \
    struct ternfs_soft_unlink_file_req_name next; \
    _ternfs_soft_unlink_file_req_put_name(ctx, &(prev), &(next), str, str_len)

static inline void _ternfs_soft_unlink_file_req_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_soft_unlink_file_req_name* prev, struct ternfs_soft_unlink_file_req_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_soft_unlink_file_req_put_creation_time(ctx, prev, next, x) \
    struct ternfs_soft_unlink_file_req_creation_time next; \
    _ternfs_soft_unlink_file_req_put_creation_time(ctx, &(prev), &(next), x)

#define ternfs_soft_unlink_file_req_put_end(ctx, prev, next) \
    { struct ternfs_soft_unlink_file_req_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_soft_unlink_file_req_end* next __attribute__((unused)) = NULL

#define TERNFS_SOFT_UNLINK_FILE_RESP_SIZE 8
struct ternfs_soft_unlink_file_resp_start;
#define ternfs_soft_unlink_file_resp_get_start(ctx, start) struct ternfs_soft_unlink_file_resp_start* start = NULL

struct ternfs_soft_unlink_file_resp_delete_creation_time { u64 x; };
static inline void _ternfs_soft_unlink_file_resp_get_delete_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_file_resp_start** prev, struct ternfs_soft_unlink_file_resp_delete_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_soft_unlink_file_resp_get_delete_creation_time(ctx, prev, next) \
    struct ternfs_soft_unlink_file_resp_delete_creation_time next; \
    _ternfs_soft_unlink_file_resp_get_delete_creation_time(ctx, &(prev), &(next))

struct ternfs_soft_unlink_file_resp_end;
#define ternfs_soft_unlink_file_resp_get_end(ctx, prev, next) \
    { struct ternfs_soft_unlink_file_resp_delete_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_soft_unlink_file_resp_end* next = NULL

static inline void ternfs_soft_unlink_file_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_file_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_soft_unlink_file_resp_put_start(ctx, start) struct ternfs_soft_unlink_file_resp_start* start = NULL

static inline void _ternfs_soft_unlink_file_resp_put_delete_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_soft_unlink_file_resp_start** prev, struct ternfs_soft_unlink_file_resp_delete_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_soft_unlink_file_resp_put_delete_creation_time(ctx, prev, next, x) \
    struct ternfs_soft_unlink_file_resp_delete_creation_time next; \
    _ternfs_soft_unlink_file_resp_put_delete_creation_time(ctx, &(prev), &(next), x)

#define ternfs_soft_unlink_file_resp_put_end(ctx, prev, next) \
    { struct ternfs_soft_unlink_file_resp_delete_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_soft_unlink_file_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_LOCAL_FILE_SPANS_REQ_SIZE 22
struct ternfs_local_file_spans_req_start;
#define ternfs_local_file_spans_req_get_start(ctx, start) struct ternfs_local_file_spans_req_start* start = NULL

struct ternfs_local_file_spans_req_file_id { u64 x; };
static inline void _ternfs_local_file_spans_req_get_file_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_file_spans_req_start** prev, struct ternfs_local_file_spans_req_file_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_local_file_spans_req_get_file_id(ctx, prev, next) \
    struct ternfs_local_file_spans_req_file_id next; \
    _ternfs_local_file_spans_req_get_file_id(ctx, &(prev), &(next))

struct ternfs_local_file_spans_req_byte_offset { u64 x; };
static inline void _ternfs_local_file_spans_req_get_byte_offset(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_file_spans_req_file_id* prev, struct ternfs_local_file_spans_req_byte_offset* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_local_file_spans_req_get_byte_offset(ctx, prev, next) \
    struct ternfs_local_file_spans_req_byte_offset next; \
    _ternfs_local_file_spans_req_get_byte_offset(ctx, &(prev), &(next))

struct ternfs_local_file_spans_req_limit { u32 x; };
static inline void _ternfs_local_file_spans_req_get_limit(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_file_spans_req_byte_offset* prev, struct ternfs_local_file_spans_req_limit* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_local_file_spans_req_get_limit(ctx, prev, next) \
    struct ternfs_local_file_spans_req_limit next; \
    _ternfs_local_file_spans_req_get_limit(ctx, &(prev), &(next))

struct ternfs_local_file_spans_req_mtu { u16 x; };
static inline void _ternfs_local_file_spans_req_get_mtu(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_file_spans_req_limit* prev, struct ternfs_local_file_spans_req_mtu* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    }
}
#define ternfs_local_file_spans_req_get_mtu(ctx, prev, next) \
    struct ternfs_local_file_spans_req_mtu next; \
    _ternfs_local_file_spans_req_get_mtu(ctx, &(prev), &(next))

struct ternfs_local_file_spans_req_end;
#define ternfs_local_file_spans_req_get_end(ctx, prev, next) \
    { struct ternfs_local_file_spans_req_mtu* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_file_spans_req_end* next = NULL

static inline void ternfs_local_file_spans_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_file_spans_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_local_file_spans_req_put_start(ctx, start) struct ternfs_local_file_spans_req_start* start = NULL

static inline void _ternfs_local_file_spans_req_put_file_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_file_spans_req_start** prev, struct ternfs_local_file_spans_req_file_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_local_file_spans_req_put_file_id(ctx, prev, next, x) \
    struct ternfs_local_file_spans_req_file_id next; \
    _ternfs_local_file_spans_req_put_file_id(ctx, &(prev), &(next), x)

static inline void _ternfs_local_file_spans_req_put_byte_offset(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_file_spans_req_file_id* prev, struct ternfs_local_file_spans_req_byte_offset* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_local_file_spans_req_put_byte_offset(ctx, prev, next, x) \
    struct ternfs_local_file_spans_req_byte_offset next; \
    _ternfs_local_file_spans_req_put_byte_offset(ctx, &(prev), &(next), x)

static inline void _ternfs_local_file_spans_req_put_limit(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_file_spans_req_byte_offset* prev, struct ternfs_local_file_spans_req_limit* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_local_file_spans_req_put_limit(ctx, prev, next, x) \
    struct ternfs_local_file_spans_req_limit next; \
    _ternfs_local_file_spans_req_put_limit(ctx, &(prev), &(next), x)

static inline void _ternfs_local_file_spans_req_put_mtu(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_file_spans_req_limit* prev, struct ternfs_local_file_spans_req_mtu* next, u16 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(x, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_local_file_spans_req_put_mtu(ctx, prev, next, x) \
    struct ternfs_local_file_spans_req_mtu next; \
    _ternfs_local_file_spans_req_put_mtu(ctx, &(prev), &(next), x)

#define ternfs_local_file_spans_req_put_end(ctx, prev, next) \
    { struct ternfs_local_file_spans_req_mtu* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_file_spans_req_end* next __attribute__((unused)) = NULL

struct ternfs_local_file_spans_resp_start;
#define ternfs_local_file_spans_resp_get_start(ctx, start) struct ternfs_local_file_spans_resp_start* start = NULL

struct ternfs_local_file_spans_resp_next_offset { u64 x; };
static inline void _ternfs_local_file_spans_resp_get_next_offset(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_file_spans_resp_start** prev, struct ternfs_local_file_spans_resp_next_offset* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_local_file_spans_resp_get_next_offset(ctx, prev, next) \
    struct ternfs_local_file_spans_resp_next_offset next; \
    _ternfs_local_file_spans_resp_get_next_offset(ctx, &(prev), &(next))

struct ternfs_local_file_spans_resp_block_services { u16 len; };
static inline void _ternfs_local_file_spans_resp_get_block_services(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_file_spans_resp_next_offset* prev, struct ternfs_local_file_spans_resp_block_services* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_local_file_spans_resp_get_block_services(ctx, prev, next) \
    struct ternfs_local_file_spans_resp_block_services next; \
    _ternfs_local_file_spans_resp_get_block_services(ctx, &(prev), &(next))

struct ternfs_local_file_spans_resp_spans { u16 len; };
static inline void _ternfs_local_file_spans_resp_get_spans(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_file_spans_resp_block_services* prev, struct ternfs_local_file_spans_resp_spans* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_local_file_spans_resp_get_spans(ctx, prev, next) \
    struct ternfs_local_file_spans_resp_spans next; \
    _ternfs_local_file_spans_resp_get_spans(ctx, &(prev), &(next))

struct ternfs_local_file_spans_resp_end;
#define ternfs_local_file_spans_resp_get_end(ctx, prev, next) \
    { struct ternfs_local_file_spans_resp_spans* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_file_spans_resp_end* next = NULL

static inline void ternfs_local_file_spans_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_file_spans_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_local_file_spans_resp_put_start(ctx, start) struct ternfs_local_file_spans_resp_start* start = NULL

static inline void _ternfs_local_file_spans_resp_put_next_offset(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_file_spans_resp_start** prev, struct ternfs_local_file_spans_resp_next_offset* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_local_file_spans_resp_put_next_offset(ctx, prev, next, x) \
    struct ternfs_local_file_spans_resp_next_offset next; \
    _ternfs_local_file_spans_resp_put_next_offset(ctx, &(prev), &(next), x)

static inline void _ternfs_local_file_spans_resp_put_block_services(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_file_spans_resp_next_offset* prev, struct ternfs_local_file_spans_resp_block_services* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_local_file_spans_resp_put_block_services(ctx, prev, next, len) \
    struct ternfs_local_file_spans_resp_block_services next; \
    _ternfs_local_file_spans_resp_put_block_services(ctx, &(prev), &(next), len)

static inline void _ternfs_local_file_spans_resp_put_spans(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_file_spans_resp_block_services* prev, struct ternfs_local_file_spans_resp_spans* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_local_file_spans_resp_put_spans(ctx, prev, next, len) \
    struct ternfs_local_file_spans_resp_spans next; \
    _ternfs_local_file_spans_resp_put_spans(ctx, &(prev), &(next), len)

#define ternfs_local_file_spans_resp_put_end(ctx, prev, next) \
    { struct ternfs_local_file_spans_resp_spans* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_file_spans_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_SAME_DIRECTORY_RENAME_REQ_MAX_SIZE 536
struct ternfs_same_directory_rename_req_start;
#define ternfs_same_directory_rename_req_get_start(ctx, start) struct ternfs_same_directory_rename_req_start* start = NULL

struct ternfs_same_directory_rename_req_target_id { u64 x; };
static inline void _ternfs_same_directory_rename_req_get_target_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_directory_rename_req_start** prev, struct ternfs_same_directory_rename_req_target_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_same_directory_rename_req_get_target_id(ctx, prev, next) \
    struct ternfs_same_directory_rename_req_target_id next; \
    _ternfs_same_directory_rename_req_get_target_id(ctx, &(prev), &(next))

struct ternfs_same_directory_rename_req_dir_id { u64 x; };
static inline void _ternfs_same_directory_rename_req_get_dir_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_directory_rename_req_target_id* prev, struct ternfs_same_directory_rename_req_dir_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_same_directory_rename_req_get_dir_id(ctx, prev, next) \
    struct ternfs_same_directory_rename_req_dir_id next; \
    _ternfs_same_directory_rename_req_get_dir_id(ctx, &(prev), &(next))

struct ternfs_same_directory_rename_req_old_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_same_directory_rename_req_get_old_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_directory_rename_req_dir_id* prev, struct ternfs_same_directory_rename_req_old_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_same_directory_rename_req_get_old_name(ctx, prev, next) \
    struct ternfs_same_directory_rename_req_old_name next; \
    _ternfs_same_directory_rename_req_get_old_name(ctx, &(prev), &(next))

struct ternfs_same_directory_rename_req_old_creation_time { u64 x; };
static inline void _ternfs_same_directory_rename_req_get_old_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_directory_rename_req_old_name* prev, struct ternfs_same_directory_rename_req_old_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_same_directory_rename_req_get_old_creation_time(ctx, prev, next) \
    struct ternfs_same_directory_rename_req_old_creation_time next; \
    _ternfs_same_directory_rename_req_get_old_creation_time(ctx, &(prev), &(next))

struct ternfs_same_directory_rename_req_new_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_same_directory_rename_req_get_new_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_directory_rename_req_old_creation_time* prev, struct ternfs_same_directory_rename_req_new_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_same_directory_rename_req_get_new_name(ctx, prev, next) \
    struct ternfs_same_directory_rename_req_new_name next; \
    _ternfs_same_directory_rename_req_get_new_name(ctx, &(prev), &(next))

struct ternfs_same_directory_rename_req_end;
#define ternfs_same_directory_rename_req_get_end(ctx, prev, next) \
    { struct ternfs_same_directory_rename_req_new_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_same_directory_rename_req_end* next = NULL

static inline void ternfs_same_directory_rename_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_directory_rename_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_same_directory_rename_req_put_start(ctx, start) struct ternfs_same_directory_rename_req_start* start = NULL

static inline void _ternfs_same_directory_rename_req_put_target_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_same_directory_rename_req_start** prev, struct ternfs_same_directory_rename_req_target_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_same_directory_rename_req_put_target_id(ctx, prev, next, x) \
    struct ternfs_same_directory_rename_req_target_id next; \
    _ternfs_same_directory_rename_req_put_target_id(ctx, &(prev), &(next), x)

static inline void _ternfs_same_directory_rename_req_put_dir_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_same_directory_rename_req_target_id* prev, struct ternfs_same_directory_rename_req_dir_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_same_directory_rename_req_put_dir_id(ctx, prev, next, x) \
    struct ternfs_same_directory_rename_req_dir_id next; \
    _ternfs_same_directory_rename_req_put_dir_id(ctx, &(prev), &(next), x)

static inline void _ternfs_same_directory_rename_req_put_old_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_same_directory_rename_req_dir_id* prev, struct ternfs_same_directory_rename_req_old_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_same_directory_rename_req_put_old_name(ctx, prev, next, str, str_len) \
    struct ternfs_same_directory_rename_req_old_name next; \
    _ternfs_same_directory_rename_req_put_old_name(ctx, &(prev), &(next), str, str_len)

static inline void _ternfs_same_directory_rename_req_put_old_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_same_directory_rename_req_old_name* prev, struct ternfs_same_directory_rename_req_old_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_same_directory_rename_req_put_old_creation_time(ctx, prev, next, x) \
    struct ternfs_same_directory_rename_req_old_creation_time next; \
    _ternfs_same_directory_rename_req_put_old_creation_time(ctx, &(prev), &(next), x)

static inline void _ternfs_same_directory_rename_req_put_new_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_same_directory_rename_req_old_creation_time* prev, struct ternfs_same_directory_rename_req_new_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_same_directory_rename_req_put_new_name(ctx, prev, next, str, str_len) \
    struct ternfs_same_directory_rename_req_new_name next; \
    _ternfs_same_directory_rename_req_put_new_name(ctx, &(prev), &(next), str, str_len)

#define ternfs_same_directory_rename_req_put_end(ctx, prev, next) \
    { struct ternfs_same_directory_rename_req_new_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_same_directory_rename_req_end* next __attribute__((unused)) = NULL

#define TERNFS_SAME_DIRECTORY_RENAME_RESP_SIZE 8
struct ternfs_same_directory_rename_resp_start;
#define ternfs_same_directory_rename_resp_get_start(ctx, start) struct ternfs_same_directory_rename_resp_start* start = NULL

struct ternfs_same_directory_rename_resp_new_creation_time { u64 x; };
static inline void _ternfs_same_directory_rename_resp_get_new_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_directory_rename_resp_start** prev, struct ternfs_same_directory_rename_resp_new_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_same_directory_rename_resp_get_new_creation_time(ctx, prev, next) \
    struct ternfs_same_directory_rename_resp_new_creation_time next; \
    _ternfs_same_directory_rename_resp_get_new_creation_time(ctx, &(prev), &(next))

struct ternfs_same_directory_rename_resp_end;
#define ternfs_same_directory_rename_resp_get_end(ctx, prev, next) \
    { struct ternfs_same_directory_rename_resp_new_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_same_directory_rename_resp_end* next = NULL

static inline void ternfs_same_directory_rename_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_directory_rename_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_same_directory_rename_resp_put_start(ctx, start) struct ternfs_same_directory_rename_resp_start* start = NULL

static inline void _ternfs_same_directory_rename_resp_put_new_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_same_directory_rename_resp_start** prev, struct ternfs_same_directory_rename_resp_new_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_same_directory_rename_resp_put_new_creation_time(ctx, prev, next, x) \
    struct ternfs_same_directory_rename_resp_new_creation_time next; \
    _ternfs_same_directory_rename_resp_put_new_creation_time(ctx, &(prev), &(next), x)

#define ternfs_same_directory_rename_resp_put_end(ctx, prev, next) \
    { struct ternfs_same_directory_rename_resp_new_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_same_directory_rename_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_ADD_INLINE_SPAN_REQ_MAX_SIZE 289
struct ternfs_add_inline_span_req_start;
#define ternfs_add_inline_span_req_get_start(ctx, start) struct ternfs_add_inline_span_req_start* start = NULL

struct ternfs_add_inline_span_req_file_id { u64 x; };
static inline void _ternfs_add_inline_span_req_get_file_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_inline_span_req_start** prev, struct ternfs_add_inline_span_req_file_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_inline_span_req_get_file_id(ctx, prev, next) \
    struct ternfs_add_inline_span_req_file_id next; \
    _ternfs_add_inline_span_req_get_file_id(ctx, &(prev), &(next))

struct ternfs_add_inline_span_req_cookie { u64 x; };
static inline void _ternfs_add_inline_span_req_get_cookie(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_inline_span_req_file_id* prev, struct ternfs_add_inline_span_req_cookie* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_inline_span_req_get_cookie(ctx, prev, next) \
    struct ternfs_add_inline_span_req_cookie next; \
    _ternfs_add_inline_span_req_get_cookie(ctx, &(prev), &(next))

struct ternfs_add_inline_span_req_storage_class { u8 x; };
static inline void _ternfs_add_inline_span_req_get_storage_class(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_inline_span_req_cookie* prev, struct ternfs_add_inline_span_req_storage_class* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_add_inline_span_req_get_storage_class(ctx, prev, next) \
    struct ternfs_add_inline_span_req_storage_class next; \
    _ternfs_add_inline_span_req_get_storage_class(ctx, &(prev), &(next))

struct ternfs_add_inline_span_req_byte_offset { u64 x; };
static inline void _ternfs_add_inline_span_req_get_byte_offset(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_inline_span_req_storage_class* prev, struct ternfs_add_inline_span_req_byte_offset* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_add_inline_span_req_get_byte_offset(ctx, prev, next) \
    struct ternfs_add_inline_span_req_byte_offset next; \
    _ternfs_add_inline_span_req_get_byte_offset(ctx, &(prev), &(next))

struct ternfs_add_inline_span_req_size { u32 x; };
static inline void _ternfs_add_inline_span_req_get_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_inline_span_req_byte_offset* prev, struct ternfs_add_inline_span_req_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_add_inline_span_req_get_size(ctx, prev, next) \
    struct ternfs_add_inline_span_req_size next; \
    _ternfs_add_inline_span_req_get_size(ctx, &(prev), &(next))

struct ternfs_add_inline_span_req_crc { u32 x; };
static inline void _ternfs_add_inline_span_req_get_crc(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_inline_span_req_size* prev, struct ternfs_add_inline_span_req_crc* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_add_inline_span_req_get_crc(ctx, prev, next) \
    struct ternfs_add_inline_span_req_crc next; \
    _ternfs_add_inline_span_req_get_crc(ctx, &(prev), &(next))

struct ternfs_add_inline_span_req_body { struct ternfs_bincode_bytes str; };
static inline void _ternfs_add_inline_span_req_get_body(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_inline_span_req_crc* prev, struct ternfs_add_inline_span_req_body* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_add_inline_span_req_get_body(ctx, prev, next) \
    struct ternfs_add_inline_span_req_body next; \
    _ternfs_add_inline_span_req_get_body(ctx, &(prev), &(next))

struct ternfs_add_inline_span_req_end;
#define ternfs_add_inline_span_req_get_end(ctx, prev, next) \
    { struct ternfs_add_inline_span_req_body* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_inline_span_req_end* next = NULL

static inline void ternfs_add_inline_span_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_inline_span_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_add_inline_span_req_put_start(ctx, start) struct ternfs_add_inline_span_req_start* start = NULL

static inline void _ternfs_add_inline_span_req_put_file_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_inline_span_req_start** prev, struct ternfs_add_inline_span_req_file_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_inline_span_req_put_file_id(ctx, prev, next, x) \
    struct ternfs_add_inline_span_req_file_id next; \
    _ternfs_add_inline_span_req_put_file_id(ctx, &(prev), &(next), x)

static inline void _ternfs_add_inline_span_req_put_cookie(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_inline_span_req_file_id* prev, struct ternfs_add_inline_span_req_cookie* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_inline_span_req_put_cookie(ctx, prev, next, x) \
    struct ternfs_add_inline_span_req_cookie next; \
    _ternfs_add_inline_span_req_put_cookie(ctx, &(prev), &(next), x)

static inline void _ternfs_add_inline_span_req_put_storage_class(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_inline_span_req_cookie* prev, struct ternfs_add_inline_span_req_storage_class* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_add_inline_span_req_put_storage_class(ctx, prev, next, x) \
    struct ternfs_add_inline_span_req_storage_class next; \
    _ternfs_add_inline_span_req_put_storage_class(ctx, &(prev), &(next), x)

static inline void _ternfs_add_inline_span_req_put_byte_offset(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_inline_span_req_storage_class* prev, struct ternfs_add_inline_span_req_byte_offset* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_add_inline_span_req_put_byte_offset(ctx, prev, next, x) \
    struct ternfs_add_inline_span_req_byte_offset next; \
    _ternfs_add_inline_span_req_put_byte_offset(ctx, &(prev), &(next), x)

static inline void _ternfs_add_inline_span_req_put_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_inline_span_req_byte_offset* prev, struct ternfs_add_inline_span_req_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_add_inline_span_req_put_size(ctx, prev, next, x) \
    struct ternfs_add_inline_span_req_size next; \
    _ternfs_add_inline_span_req_put_size(ctx, &(prev), &(next), x)

static inline void _ternfs_add_inline_span_req_put_crc(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_inline_span_req_size* prev, struct ternfs_add_inline_span_req_crc* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_add_inline_span_req_put_crc(ctx, prev, next, x) \
    struct ternfs_add_inline_span_req_crc next; \
    _ternfs_add_inline_span_req_put_crc(ctx, &(prev), &(next), x)

static inline void _ternfs_add_inline_span_req_put_body(struct ternfs_bincode_put_ctx* ctx, struct ternfs_add_inline_span_req_crc* prev, struct ternfs_add_inline_span_req_body* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_add_inline_span_req_put_body(ctx, prev, next, str, str_len) \
    struct ternfs_add_inline_span_req_body next; \
    _ternfs_add_inline_span_req_put_body(ctx, &(prev), &(next), str, str_len)

#define ternfs_add_inline_span_req_put_end(ctx, prev, next) \
    { struct ternfs_add_inline_span_req_body* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_inline_span_req_end* next __attribute__((unused)) = NULL

#define TERNFS_ADD_INLINE_SPAN_RESP_SIZE 0
struct ternfs_add_inline_span_resp_start;
#define ternfs_add_inline_span_resp_get_start(ctx, start) struct ternfs_add_inline_span_resp_start* start = NULL

struct ternfs_add_inline_span_resp_end;
#define ternfs_add_inline_span_resp_get_end(ctx, prev, next) \
    { struct ternfs_add_inline_span_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_inline_span_resp_end* next = NULL

static inline void ternfs_add_inline_span_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_add_inline_span_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_add_inline_span_resp_put_start(ctx, start) struct ternfs_add_inline_span_resp_start* start = NULL

#define ternfs_add_inline_span_resp_put_end(ctx, prev, next) \
    { struct ternfs_add_inline_span_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_add_inline_span_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_SET_TIME_REQ_SIZE 24
struct ternfs_set_time_req_start;
#define ternfs_set_time_req_get_start(ctx, start) struct ternfs_set_time_req_start* start = NULL

struct ternfs_set_time_req_id { u64 x; };
static inline void _ternfs_set_time_req_get_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_set_time_req_start** prev, struct ternfs_set_time_req_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_set_time_req_get_id(ctx, prev, next) \
    struct ternfs_set_time_req_id next; \
    _ternfs_set_time_req_get_id(ctx, &(prev), &(next))

struct ternfs_set_time_req_mtime { u64 x; };
static inline void _ternfs_set_time_req_get_mtime(struct ternfs_bincode_get_ctx* ctx, struct ternfs_set_time_req_id* prev, struct ternfs_set_time_req_mtime* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_set_time_req_get_mtime(ctx, prev, next) \
    struct ternfs_set_time_req_mtime next; \
    _ternfs_set_time_req_get_mtime(ctx, &(prev), &(next))

struct ternfs_set_time_req_atime { u64 x; };
static inline void _ternfs_set_time_req_get_atime(struct ternfs_bincode_get_ctx* ctx, struct ternfs_set_time_req_mtime* prev, struct ternfs_set_time_req_atime* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_set_time_req_get_atime(ctx, prev, next) \
    struct ternfs_set_time_req_atime next; \
    _ternfs_set_time_req_get_atime(ctx, &(prev), &(next))

struct ternfs_set_time_req_end;
#define ternfs_set_time_req_get_end(ctx, prev, next) \
    { struct ternfs_set_time_req_atime* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_set_time_req_end* next = NULL

static inline void ternfs_set_time_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_set_time_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_set_time_req_put_start(ctx, start) struct ternfs_set_time_req_start* start = NULL

static inline void _ternfs_set_time_req_put_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_set_time_req_start** prev, struct ternfs_set_time_req_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_set_time_req_put_id(ctx, prev, next, x) \
    struct ternfs_set_time_req_id next; \
    _ternfs_set_time_req_put_id(ctx, &(prev), &(next), x)

static inline void _ternfs_set_time_req_put_mtime(struct ternfs_bincode_put_ctx* ctx, struct ternfs_set_time_req_id* prev, struct ternfs_set_time_req_mtime* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_set_time_req_put_mtime(ctx, prev, next, x) \
    struct ternfs_set_time_req_mtime next; \
    _ternfs_set_time_req_put_mtime(ctx, &(prev), &(next), x)

static inline void _ternfs_set_time_req_put_atime(struct ternfs_bincode_put_ctx* ctx, struct ternfs_set_time_req_mtime* prev, struct ternfs_set_time_req_atime* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_set_time_req_put_atime(ctx, prev, next, x) \
    struct ternfs_set_time_req_atime next; \
    _ternfs_set_time_req_put_atime(ctx, &(prev), &(next), x)

#define ternfs_set_time_req_put_end(ctx, prev, next) \
    { struct ternfs_set_time_req_atime* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_set_time_req_end* next __attribute__((unused)) = NULL

#define TERNFS_SET_TIME_RESP_SIZE 0
struct ternfs_set_time_resp_start;
#define ternfs_set_time_resp_get_start(ctx, start) struct ternfs_set_time_resp_start* start = NULL

struct ternfs_set_time_resp_end;
#define ternfs_set_time_resp_get_end(ctx, prev, next) \
    { struct ternfs_set_time_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_set_time_resp_end* next = NULL

static inline void ternfs_set_time_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_set_time_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_set_time_resp_put_start(ctx, start) struct ternfs_set_time_resp_start* start = NULL

#define ternfs_set_time_resp_put_end(ctx, prev, next) \
    { struct ternfs_set_time_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_set_time_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_FULL_READ_DIR_REQ_MAX_SIZE 277
struct ternfs_full_read_dir_req_start;
#define ternfs_full_read_dir_req_get_start(ctx, start) struct ternfs_full_read_dir_req_start* start = NULL

struct ternfs_full_read_dir_req_dir_id { u64 x; };
static inline void _ternfs_full_read_dir_req_get_dir_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_req_start** prev, struct ternfs_full_read_dir_req_dir_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_full_read_dir_req_get_dir_id(ctx, prev, next) \
    struct ternfs_full_read_dir_req_dir_id next; \
    _ternfs_full_read_dir_req_get_dir_id(ctx, &(prev), &(next))

struct ternfs_full_read_dir_req_flags { u8 x; };
static inline void _ternfs_full_read_dir_req_get_flags(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_req_dir_id* prev, struct ternfs_full_read_dir_req_flags* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = *(u8*)(ctx->buf);
            ctx->buf += 1;
        }
    }
}
#define ternfs_full_read_dir_req_get_flags(ctx, prev, next) \
    struct ternfs_full_read_dir_req_flags next; \
    _ternfs_full_read_dir_req_get_flags(ctx, &(prev), &(next))

struct ternfs_full_read_dir_req_start_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_full_read_dir_req_get_start_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_req_flags* prev, struct ternfs_full_read_dir_req_start_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_full_read_dir_req_get_start_name(ctx, prev, next) \
    struct ternfs_full_read_dir_req_start_name next; \
    _ternfs_full_read_dir_req_get_start_name(ctx, &(prev), &(next))

struct ternfs_full_read_dir_req_start_time { u64 x; };
static inline void _ternfs_full_read_dir_req_get_start_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_req_start_name* prev, struct ternfs_full_read_dir_req_start_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_full_read_dir_req_get_start_time(ctx, prev, next) \
    struct ternfs_full_read_dir_req_start_time next; \
    _ternfs_full_read_dir_req_get_start_time(ctx, &(prev), &(next))

struct ternfs_full_read_dir_req_limit { u16 x; };
static inline void _ternfs_full_read_dir_req_get_limit(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_req_start_time* prev, struct ternfs_full_read_dir_req_limit* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    }
}
#define ternfs_full_read_dir_req_get_limit(ctx, prev, next) \
    struct ternfs_full_read_dir_req_limit next; \
    _ternfs_full_read_dir_req_get_limit(ctx, &(prev), &(next))

struct ternfs_full_read_dir_req_mtu { u16 x; };
static inline void _ternfs_full_read_dir_req_get_mtu(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_req_limit* prev, struct ternfs_full_read_dir_req_mtu* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    }
}
#define ternfs_full_read_dir_req_get_mtu(ctx, prev, next) \
    struct ternfs_full_read_dir_req_mtu next; \
    _ternfs_full_read_dir_req_get_mtu(ctx, &(prev), &(next))

struct ternfs_full_read_dir_req_end;
#define ternfs_full_read_dir_req_get_end(ctx, prev, next) \
    { struct ternfs_full_read_dir_req_mtu* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_full_read_dir_req_end* next = NULL

static inline void ternfs_full_read_dir_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_full_read_dir_req_put_start(ctx, start) struct ternfs_full_read_dir_req_start* start = NULL

static inline void _ternfs_full_read_dir_req_put_dir_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_full_read_dir_req_start** prev, struct ternfs_full_read_dir_req_dir_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_full_read_dir_req_put_dir_id(ctx, prev, next, x) \
    struct ternfs_full_read_dir_req_dir_id next; \
    _ternfs_full_read_dir_req_put_dir_id(ctx, &(prev), &(next), x)

static inline void _ternfs_full_read_dir_req_put_flags(struct ternfs_bincode_put_ctx* ctx, struct ternfs_full_read_dir_req_dir_id* prev, struct ternfs_full_read_dir_req_flags* next, u8 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 1);
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += 1;
}
#define ternfs_full_read_dir_req_put_flags(ctx, prev, next, x) \
    struct ternfs_full_read_dir_req_flags next; \
    _ternfs_full_read_dir_req_put_flags(ctx, &(prev), &(next), x)

static inline void _ternfs_full_read_dir_req_put_start_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_full_read_dir_req_flags* prev, struct ternfs_full_read_dir_req_start_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_full_read_dir_req_put_start_name(ctx, prev, next, str, str_len) \
    struct ternfs_full_read_dir_req_start_name next; \
    _ternfs_full_read_dir_req_put_start_name(ctx, &(prev), &(next), str, str_len)

static inline void _ternfs_full_read_dir_req_put_start_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_full_read_dir_req_start_name* prev, struct ternfs_full_read_dir_req_start_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_full_read_dir_req_put_start_time(ctx, prev, next, x) \
    struct ternfs_full_read_dir_req_start_time next; \
    _ternfs_full_read_dir_req_put_start_time(ctx, &(prev), &(next), x)

static inline void _ternfs_full_read_dir_req_put_limit(struct ternfs_bincode_put_ctx* ctx, struct ternfs_full_read_dir_req_start_time* prev, struct ternfs_full_read_dir_req_limit* next, u16 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(x, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_full_read_dir_req_put_limit(ctx, prev, next, x) \
    struct ternfs_full_read_dir_req_limit next; \
    _ternfs_full_read_dir_req_put_limit(ctx, &(prev), &(next), x)

static inline void _ternfs_full_read_dir_req_put_mtu(struct ternfs_bincode_put_ctx* ctx, struct ternfs_full_read_dir_req_limit* prev, struct ternfs_full_read_dir_req_mtu* next, u16 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(x, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_full_read_dir_req_put_mtu(ctx, prev, next, x) \
    struct ternfs_full_read_dir_req_mtu next; \
    _ternfs_full_read_dir_req_put_mtu(ctx, &(prev), &(next), x)

#define ternfs_full_read_dir_req_put_end(ctx, prev, next) \
    { struct ternfs_full_read_dir_req_mtu* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_full_read_dir_req_end* next __attribute__((unused)) = NULL

struct ternfs_full_read_dir_resp_start;
#define ternfs_full_read_dir_resp_get_start(ctx, start) struct ternfs_full_read_dir_resp_start* start = NULL

#define ternfs_full_read_dir_resp_get_next(ctx, prev, next) \
    { struct ternfs_full_read_dir_resp_start** __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_full_read_dir_cursor_start* next = NULL

struct ternfs_full_read_dir_resp_results { u16 len; };
static inline void _ternfs_full_read_dir_resp_get_results(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_cursor_end** prev, struct ternfs_full_read_dir_resp_results* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_full_read_dir_resp_get_results(ctx, prev, next) \
    struct ternfs_full_read_dir_resp_results next; \
    _ternfs_full_read_dir_resp_get_results(ctx, &(prev), &(next))

struct ternfs_full_read_dir_resp_end;
#define ternfs_full_read_dir_resp_get_end(ctx, prev, next) \
    { struct ternfs_full_read_dir_resp_results* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_full_read_dir_resp_end* next = NULL

static inline void ternfs_full_read_dir_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_full_read_dir_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_full_read_dir_resp_put_start(ctx, start) struct ternfs_full_read_dir_resp_start* start = NULL

static inline void _ternfs_full_read_dir_resp_put_results(struct ternfs_bincode_put_ctx* ctx, struct ternfs_full_read_dir_resp_start** prev, struct ternfs_full_read_dir_resp_results* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_full_read_dir_resp_put_results(ctx, prev, next, len) \
    struct ternfs_full_read_dir_resp_results next; \
    _ternfs_full_read_dir_resp_put_results(ctx, &(prev), &(next), len)

#define ternfs_full_read_dir_resp_put_end(ctx, prev, next) \
    { struct ternfs_full_read_dir_resp_results* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_full_read_dir_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_MOVE_SPAN_REQ_SIZE 52
struct ternfs_move_span_req_start;
#define ternfs_move_span_req_get_start(ctx, start) struct ternfs_move_span_req_start* start = NULL

struct ternfs_move_span_req_span_size { u32 x; };
static inline void _ternfs_move_span_req_get_span_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_move_span_req_start** prev, struct ternfs_move_span_req_span_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_move_span_req_get_span_size(ctx, prev, next) \
    struct ternfs_move_span_req_span_size next; \
    _ternfs_move_span_req_get_span_size(ctx, &(prev), &(next))

struct ternfs_move_span_req_file_id1 { u64 x; };
static inline void _ternfs_move_span_req_get_file_id1(struct ternfs_bincode_get_ctx* ctx, struct ternfs_move_span_req_span_size* prev, struct ternfs_move_span_req_file_id1* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_move_span_req_get_file_id1(ctx, prev, next) \
    struct ternfs_move_span_req_file_id1 next; \
    _ternfs_move_span_req_get_file_id1(ctx, &(prev), &(next))

struct ternfs_move_span_req_byte_offset1 { u64 x; };
static inline void _ternfs_move_span_req_get_byte_offset1(struct ternfs_bincode_get_ctx* ctx, struct ternfs_move_span_req_file_id1* prev, struct ternfs_move_span_req_byte_offset1* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_move_span_req_get_byte_offset1(ctx, prev, next) \
    struct ternfs_move_span_req_byte_offset1 next; \
    _ternfs_move_span_req_get_byte_offset1(ctx, &(prev), &(next))

struct ternfs_move_span_req_cookie1 { u64 x; };
static inline void _ternfs_move_span_req_get_cookie1(struct ternfs_bincode_get_ctx* ctx, struct ternfs_move_span_req_byte_offset1* prev, struct ternfs_move_span_req_cookie1* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_move_span_req_get_cookie1(ctx, prev, next) \
    struct ternfs_move_span_req_cookie1 next; \
    _ternfs_move_span_req_get_cookie1(ctx, &(prev), &(next))

struct ternfs_move_span_req_file_id2 { u64 x; };
static inline void _ternfs_move_span_req_get_file_id2(struct ternfs_bincode_get_ctx* ctx, struct ternfs_move_span_req_cookie1* prev, struct ternfs_move_span_req_file_id2* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_move_span_req_get_file_id2(ctx, prev, next) \
    struct ternfs_move_span_req_file_id2 next; \
    _ternfs_move_span_req_get_file_id2(ctx, &(prev), &(next))

struct ternfs_move_span_req_byte_offset2 { u64 x; };
static inline void _ternfs_move_span_req_get_byte_offset2(struct ternfs_bincode_get_ctx* ctx, struct ternfs_move_span_req_file_id2* prev, struct ternfs_move_span_req_byte_offset2* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_move_span_req_get_byte_offset2(ctx, prev, next) \
    struct ternfs_move_span_req_byte_offset2 next; \
    _ternfs_move_span_req_get_byte_offset2(ctx, &(prev), &(next))

struct ternfs_move_span_req_cookie2 { u64 x; };
static inline void _ternfs_move_span_req_get_cookie2(struct ternfs_bincode_get_ctx* ctx, struct ternfs_move_span_req_byte_offset2* prev, struct ternfs_move_span_req_cookie2* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_move_span_req_get_cookie2(ctx, prev, next) \
    struct ternfs_move_span_req_cookie2 next; \
    _ternfs_move_span_req_get_cookie2(ctx, &(prev), &(next))

struct ternfs_move_span_req_end;
#define ternfs_move_span_req_get_end(ctx, prev, next) \
    { struct ternfs_move_span_req_cookie2* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_move_span_req_end* next = NULL

static inline void ternfs_move_span_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_move_span_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_move_span_req_put_start(ctx, start) struct ternfs_move_span_req_start* start = NULL

static inline void _ternfs_move_span_req_put_span_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_move_span_req_start** prev, struct ternfs_move_span_req_span_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_move_span_req_put_span_size(ctx, prev, next, x) \
    struct ternfs_move_span_req_span_size next; \
    _ternfs_move_span_req_put_span_size(ctx, &(prev), &(next), x)

static inline void _ternfs_move_span_req_put_file_id1(struct ternfs_bincode_put_ctx* ctx, struct ternfs_move_span_req_span_size* prev, struct ternfs_move_span_req_file_id1* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_move_span_req_put_file_id1(ctx, prev, next, x) \
    struct ternfs_move_span_req_file_id1 next; \
    _ternfs_move_span_req_put_file_id1(ctx, &(prev), &(next), x)

static inline void _ternfs_move_span_req_put_byte_offset1(struct ternfs_bincode_put_ctx* ctx, struct ternfs_move_span_req_file_id1* prev, struct ternfs_move_span_req_byte_offset1* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_move_span_req_put_byte_offset1(ctx, prev, next, x) \
    struct ternfs_move_span_req_byte_offset1 next; \
    _ternfs_move_span_req_put_byte_offset1(ctx, &(prev), &(next), x)

static inline void _ternfs_move_span_req_put_cookie1(struct ternfs_bincode_put_ctx* ctx, struct ternfs_move_span_req_byte_offset1* prev, struct ternfs_move_span_req_cookie1* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_move_span_req_put_cookie1(ctx, prev, next, x) \
    struct ternfs_move_span_req_cookie1 next; \
    _ternfs_move_span_req_put_cookie1(ctx, &(prev), &(next), x)

static inline void _ternfs_move_span_req_put_file_id2(struct ternfs_bincode_put_ctx* ctx, struct ternfs_move_span_req_cookie1* prev, struct ternfs_move_span_req_file_id2* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_move_span_req_put_file_id2(ctx, prev, next, x) \
    struct ternfs_move_span_req_file_id2 next; \
    _ternfs_move_span_req_put_file_id2(ctx, &(prev), &(next), x)

static inline void _ternfs_move_span_req_put_byte_offset2(struct ternfs_bincode_put_ctx* ctx, struct ternfs_move_span_req_file_id2* prev, struct ternfs_move_span_req_byte_offset2* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_move_span_req_put_byte_offset2(ctx, prev, next, x) \
    struct ternfs_move_span_req_byte_offset2 next; \
    _ternfs_move_span_req_put_byte_offset2(ctx, &(prev), &(next), x)

static inline void _ternfs_move_span_req_put_cookie2(struct ternfs_bincode_put_ctx* ctx, struct ternfs_move_span_req_byte_offset2* prev, struct ternfs_move_span_req_cookie2* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_move_span_req_put_cookie2(ctx, prev, next, x) \
    struct ternfs_move_span_req_cookie2 next; \
    _ternfs_move_span_req_put_cookie2(ctx, &(prev), &(next), x)

#define ternfs_move_span_req_put_end(ctx, prev, next) \
    { struct ternfs_move_span_req_cookie2* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_move_span_req_end* next __attribute__((unused)) = NULL

#define TERNFS_MOVE_SPAN_RESP_SIZE 0
struct ternfs_move_span_resp_start;
#define ternfs_move_span_resp_get_start(ctx, start) struct ternfs_move_span_resp_start* start = NULL

struct ternfs_move_span_resp_end;
#define ternfs_move_span_resp_get_end(ctx, prev, next) \
    { struct ternfs_move_span_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_move_span_resp_end* next = NULL

static inline void ternfs_move_span_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_move_span_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_move_span_resp_put_start(ctx, start) struct ternfs_move_span_resp_start* start = NULL

#define ternfs_move_span_resp_put_end(ctx, prev, next) \
    { struct ternfs_move_span_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_move_span_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_REMOVE_NON_OWNED_EDGE_REQ_MAX_SIZE 280
struct ternfs_remove_non_owned_edge_req_start;
#define ternfs_remove_non_owned_edge_req_get_start(ctx, start) struct ternfs_remove_non_owned_edge_req_start* start = NULL

struct ternfs_remove_non_owned_edge_req_dir_id { u64 x; };
static inline void _ternfs_remove_non_owned_edge_req_get_dir_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_remove_non_owned_edge_req_start** prev, struct ternfs_remove_non_owned_edge_req_dir_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_remove_non_owned_edge_req_get_dir_id(ctx, prev, next) \
    struct ternfs_remove_non_owned_edge_req_dir_id next; \
    _ternfs_remove_non_owned_edge_req_get_dir_id(ctx, &(prev), &(next))

struct ternfs_remove_non_owned_edge_req_target_id { u64 x; };
static inline void _ternfs_remove_non_owned_edge_req_get_target_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_remove_non_owned_edge_req_dir_id* prev, struct ternfs_remove_non_owned_edge_req_target_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_remove_non_owned_edge_req_get_target_id(ctx, prev, next) \
    struct ternfs_remove_non_owned_edge_req_target_id next; \
    _ternfs_remove_non_owned_edge_req_get_target_id(ctx, &(prev), &(next))

struct ternfs_remove_non_owned_edge_req_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_remove_non_owned_edge_req_get_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_remove_non_owned_edge_req_target_id* prev, struct ternfs_remove_non_owned_edge_req_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_remove_non_owned_edge_req_get_name(ctx, prev, next) \
    struct ternfs_remove_non_owned_edge_req_name next; \
    _ternfs_remove_non_owned_edge_req_get_name(ctx, &(prev), &(next))

struct ternfs_remove_non_owned_edge_req_creation_time { u64 x; };
static inline void _ternfs_remove_non_owned_edge_req_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_remove_non_owned_edge_req_name* prev, struct ternfs_remove_non_owned_edge_req_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_remove_non_owned_edge_req_get_creation_time(ctx, prev, next) \
    struct ternfs_remove_non_owned_edge_req_creation_time next; \
    _ternfs_remove_non_owned_edge_req_get_creation_time(ctx, &(prev), &(next))

struct ternfs_remove_non_owned_edge_req_end;
#define ternfs_remove_non_owned_edge_req_get_end(ctx, prev, next) \
    { struct ternfs_remove_non_owned_edge_req_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_remove_non_owned_edge_req_end* next = NULL

static inline void ternfs_remove_non_owned_edge_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_remove_non_owned_edge_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_remove_non_owned_edge_req_put_start(ctx, start) struct ternfs_remove_non_owned_edge_req_start* start = NULL

static inline void _ternfs_remove_non_owned_edge_req_put_dir_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_remove_non_owned_edge_req_start** prev, struct ternfs_remove_non_owned_edge_req_dir_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_remove_non_owned_edge_req_put_dir_id(ctx, prev, next, x) \
    struct ternfs_remove_non_owned_edge_req_dir_id next; \
    _ternfs_remove_non_owned_edge_req_put_dir_id(ctx, &(prev), &(next), x)

static inline void _ternfs_remove_non_owned_edge_req_put_target_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_remove_non_owned_edge_req_dir_id* prev, struct ternfs_remove_non_owned_edge_req_target_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_remove_non_owned_edge_req_put_target_id(ctx, prev, next, x) \
    struct ternfs_remove_non_owned_edge_req_target_id next; \
    _ternfs_remove_non_owned_edge_req_put_target_id(ctx, &(prev), &(next), x)

static inline void _ternfs_remove_non_owned_edge_req_put_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_remove_non_owned_edge_req_target_id* prev, struct ternfs_remove_non_owned_edge_req_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_remove_non_owned_edge_req_put_name(ctx, prev, next, str, str_len) \
    struct ternfs_remove_non_owned_edge_req_name next; \
    _ternfs_remove_non_owned_edge_req_put_name(ctx, &(prev), &(next), str, str_len)

static inline void _ternfs_remove_non_owned_edge_req_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_remove_non_owned_edge_req_name* prev, struct ternfs_remove_non_owned_edge_req_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_remove_non_owned_edge_req_put_creation_time(ctx, prev, next, x) \
    struct ternfs_remove_non_owned_edge_req_creation_time next; \
    _ternfs_remove_non_owned_edge_req_put_creation_time(ctx, &(prev), &(next), x)

#define ternfs_remove_non_owned_edge_req_put_end(ctx, prev, next) \
    { struct ternfs_remove_non_owned_edge_req_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_remove_non_owned_edge_req_end* next __attribute__((unused)) = NULL

#define TERNFS_REMOVE_NON_OWNED_EDGE_RESP_SIZE 0
struct ternfs_remove_non_owned_edge_resp_start;
#define ternfs_remove_non_owned_edge_resp_get_start(ctx, start) struct ternfs_remove_non_owned_edge_resp_start* start = NULL

struct ternfs_remove_non_owned_edge_resp_end;
#define ternfs_remove_non_owned_edge_resp_get_end(ctx, prev, next) \
    { struct ternfs_remove_non_owned_edge_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_remove_non_owned_edge_resp_end* next = NULL

static inline void ternfs_remove_non_owned_edge_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_remove_non_owned_edge_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_remove_non_owned_edge_resp_put_start(ctx, start) struct ternfs_remove_non_owned_edge_resp_start* start = NULL

#define ternfs_remove_non_owned_edge_resp_put_end(ctx, prev, next) \
    { struct ternfs_remove_non_owned_edge_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_remove_non_owned_edge_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_SAME_SHARD_HARD_FILE_UNLINK_REQ_MAX_SIZE 280
struct ternfs_same_shard_hard_file_unlink_req_start;
#define ternfs_same_shard_hard_file_unlink_req_get_start(ctx, start) struct ternfs_same_shard_hard_file_unlink_req_start* start = NULL

struct ternfs_same_shard_hard_file_unlink_req_owner_id { u64 x; };
static inline void _ternfs_same_shard_hard_file_unlink_req_get_owner_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_shard_hard_file_unlink_req_start** prev, struct ternfs_same_shard_hard_file_unlink_req_owner_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_same_shard_hard_file_unlink_req_get_owner_id(ctx, prev, next) \
    struct ternfs_same_shard_hard_file_unlink_req_owner_id next; \
    _ternfs_same_shard_hard_file_unlink_req_get_owner_id(ctx, &(prev), &(next))

struct ternfs_same_shard_hard_file_unlink_req_target_id { u64 x; };
static inline void _ternfs_same_shard_hard_file_unlink_req_get_target_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_shard_hard_file_unlink_req_owner_id* prev, struct ternfs_same_shard_hard_file_unlink_req_target_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_same_shard_hard_file_unlink_req_get_target_id(ctx, prev, next) \
    struct ternfs_same_shard_hard_file_unlink_req_target_id next; \
    _ternfs_same_shard_hard_file_unlink_req_get_target_id(ctx, &(prev), &(next))

struct ternfs_same_shard_hard_file_unlink_req_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_same_shard_hard_file_unlink_req_get_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_shard_hard_file_unlink_req_target_id* prev, struct ternfs_same_shard_hard_file_unlink_req_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_same_shard_hard_file_unlink_req_get_name(ctx, prev, next) \
    struct ternfs_same_shard_hard_file_unlink_req_name next; \
    _ternfs_same_shard_hard_file_unlink_req_get_name(ctx, &(prev), &(next))

struct ternfs_same_shard_hard_file_unlink_req_creation_time { u64 x; };
static inline void _ternfs_same_shard_hard_file_unlink_req_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_shard_hard_file_unlink_req_name* prev, struct ternfs_same_shard_hard_file_unlink_req_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_same_shard_hard_file_unlink_req_get_creation_time(ctx, prev, next) \
    struct ternfs_same_shard_hard_file_unlink_req_creation_time next; \
    _ternfs_same_shard_hard_file_unlink_req_get_creation_time(ctx, &(prev), &(next))

struct ternfs_same_shard_hard_file_unlink_req_end;
#define ternfs_same_shard_hard_file_unlink_req_get_end(ctx, prev, next) \
    { struct ternfs_same_shard_hard_file_unlink_req_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_same_shard_hard_file_unlink_req_end* next = NULL

static inline void ternfs_same_shard_hard_file_unlink_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_shard_hard_file_unlink_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_same_shard_hard_file_unlink_req_put_start(ctx, start) struct ternfs_same_shard_hard_file_unlink_req_start* start = NULL

static inline void _ternfs_same_shard_hard_file_unlink_req_put_owner_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_same_shard_hard_file_unlink_req_start** prev, struct ternfs_same_shard_hard_file_unlink_req_owner_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_same_shard_hard_file_unlink_req_put_owner_id(ctx, prev, next, x) \
    struct ternfs_same_shard_hard_file_unlink_req_owner_id next; \
    _ternfs_same_shard_hard_file_unlink_req_put_owner_id(ctx, &(prev), &(next), x)

static inline void _ternfs_same_shard_hard_file_unlink_req_put_target_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_same_shard_hard_file_unlink_req_owner_id* prev, struct ternfs_same_shard_hard_file_unlink_req_target_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_same_shard_hard_file_unlink_req_put_target_id(ctx, prev, next, x) \
    struct ternfs_same_shard_hard_file_unlink_req_target_id next; \
    _ternfs_same_shard_hard_file_unlink_req_put_target_id(ctx, &(prev), &(next), x)

static inline void _ternfs_same_shard_hard_file_unlink_req_put_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_same_shard_hard_file_unlink_req_target_id* prev, struct ternfs_same_shard_hard_file_unlink_req_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_same_shard_hard_file_unlink_req_put_name(ctx, prev, next, str, str_len) \
    struct ternfs_same_shard_hard_file_unlink_req_name next; \
    _ternfs_same_shard_hard_file_unlink_req_put_name(ctx, &(prev), &(next), str, str_len)

static inline void _ternfs_same_shard_hard_file_unlink_req_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_same_shard_hard_file_unlink_req_name* prev, struct ternfs_same_shard_hard_file_unlink_req_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_same_shard_hard_file_unlink_req_put_creation_time(ctx, prev, next, x) \
    struct ternfs_same_shard_hard_file_unlink_req_creation_time next; \
    _ternfs_same_shard_hard_file_unlink_req_put_creation_time(ctx, &(prev), &(next), x)

#define ternfs_same_shard_hard_file_unlink_req_put_end(ctx, prev, next) \
    { struct ternfs_same_shard_hard_file_unlink_req_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_same_shard_hard_file_unlink_req_end* next __attribute__((unused)) = NULL

#define TERNFS_SAME_SHARD_HARD_FILE_UNLINK_RESP_SIZE 0
struct ternfs_same_shard_hard_file_unlink_resp_start;
#define ternfs_same_shard_hard_file_unlink_resp_get_start(ctx, start) struct ternfs_same_shard_hard_file_unlink_resp_start* start = NULL

struct ternfs_same_shard_hard_file_unlink_resp_end;
#define ternfs_same_shard_hard_file_unlink_resp_get_end(ctx, prev, next) \
    { struct ternfs_same_shard_hard_file_unlink_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_same_shard_hard_file_unlink_resp_end* next = NULL

static inline void ternfs_same_shard_hard_file_unlink_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_same_shard_hard_file_unlink_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_same_shard_hard_file_unlink_resp_put_start(ctx, start) struct ternfs_same_shard_hard_file_unlink_resp_start* start = NULL

#define ternfs_same_shard_hard_file_unlink_resp_put_end(ctx, prev, next) \
    { struct ternfs_same_shard_hard_file_unlink_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_same_shard_hard_file_unlink_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_MAKE_DIRECTORY_REQ_MAX_SIZE 264
struct ternfs_make_directory_req_start;
#define ternfs_make_directory_req_get_start(ctx, start) struct ternfs_make_directory_req_start* start = NULL

struct ternfs_make_directory_req_owner_id { u64 x; };
static inline void _ternfs_make_directory_req_get_owner_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_make_directory_req_start** prev, struct ternfs_make_directory_req_owner_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_make_directory_req_get_owner_id(ctx, prev, next) \
    struct ternfs_make_directory_req_owner_id next; \
    _ternfs_make_directory_req_get_owner_id(ctx, &(prev), &(next))

struct ternfs_make_directory_req_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_make_directory_req_get_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_make_directory_req_owner_id* prev, struct ternfs_make_directory_req_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_make_directory_req_get_name(ctx, prev, next) \
    struct ternfs_make_directory_req_name next; \
    _ternfs_make_directory_req_get_name(ctx, &(prev), &(next))

struct ternfs_make_directory_req_end;
#define ternfs_make_directory_req_get_end(ctx, prev, next) \
    { struct ternfs_make_directory_req_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_make_directory_req_end* next = NULL

static inline void ternfs_make_directory_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_make_directory_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_make_directory_req_put_start(ctx, start) struct ternfs_make_directory_req_start* start = NULL

static inline void _ternfs_make_directory_req_put_owner_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_make_directory_req_start** prev, struct ternfs_make_directory_req_owner_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_make_directory_req_put_owner_id(ctx, prev, next, x) \
    struct ternfs_make_directory_req_owner_id next; \
    _ternfs_make_directory_req_put_owner_id(ctx, &(prev), &(next), x)

static inline void _ternfs_make_directory_req_put_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_make_directory_req_owner_id* prev, struct ternfs_make_directory_req_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_make_directory_req_put_name(ctx, prev, next, str, str_len) \
    struct ternfs_make_directory_req_name next; \
    _ternfs_make_directory_req_put_name(ctx, &(prev), &(next), str, str_len)

#define ternfs_make_directory_req_put_end(ctx, prev, next) \
    { struct ternfs_make_directory_req_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_make_directory_req_end* next __attribute__((unused)) = NULL

#define TERNFS_MAKE_DIRECTORY_RESP_SIZE 16
struct ternfs_make_directory_resp_start;
#define ternfs_make_directory_resp_get_start(ctx, start) struct ternfs_make_directory_resp_start* start = NULL

struct ternfs_make_directory_resp_id { u64 x; };
static inline void _ternfs_make_directory_resp_get_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_make_directory_resp_start** prev, struct ternfs_make_directory_resp_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_make_directory_resp_get_id(ctx, prev, next) \
    struct ternfs_make_directory_resp_id next; \
    _ternfs_make_directory_resp_get_id(ctx, &(prev), &(next))

struct ternfs_make_directory_resp_creation_time { u64 x; };
static inline void _ternfs_make_directory_resp_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_make_directory_resp_id* prev, struct ternfs_make_directory_resp_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_make_directory_resp_get_creation_time(ctx, prev, next) \
    struct ternfs_make_directory_resp_creation_time next; \
    _ternfs_make_directory_resp_get_creation_time(ctx, &(prev), &(next))

struct ternfs_make_directory_resp_end;
#define ternfs_make_directory_resp_get_end(ctx, prev, next) \
    { struct ternfs_make_directory_resp_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_make_directory_resp_end* next = NULL

static inline void ternfs_make_directory_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_make_directory_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_make_directory_resp_put_start(ctx, start) struct ternfs_make_directory_resp_start* start = NULL

static inline void _ternfs_make_directory_resp_put_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_make_directory_resp_start** prev, struct ternfs_make_directory_resp_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_make_directory_resp_put_id(ctx, prev, next, x) \
    struct ternfs_make_directory_resp_id next; \
    _ternfs_make_directory_resp_put_id(ctx, &(prev), &(next), x)

static inline void _ternfs_make_directory_resp_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_make_directory_resp_id* prev, struct ternfs_make_directory_resp_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_make_directory_resp_put_creation_time(ctx, prev, next, x) \
    struct ternfs_make_directory_resp_creation_time next; \
    _ternfs_make_directory_resp_put_creation_time(ctx, &(prev), &(next), x)

#define ternfs_make_directory_resp_put_end(ctx, prev, next) \
    { struct ternfs_make_directory_resp_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_make_directory_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_RENAME_FILE_REQ_MAX_SIZE 544
struct ternfs_rename_file_req_start;
#define ternfs_rename_file_req_get_start(ctx, start) struct ternfs_rename_file_req_start* start = NULL

struct ternfs_rename_file_req_target_id { u64 x; };
static inline void _ternfs_rename_file_req_get_target_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_file_req_start** prev, struct ternfs_rename_file_req_target_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_rename_file_req_get_target_id(ctx, prev, next) \
    struct ternfs_rename_file_req_target_id next; \
    _ternfs_rename_file_req_get_target_id(ctx, &(prev), &(next))

struct ternfs_rename_file_req_old_owner_id { u64 x; };
static inline void _ternfs_rename_file_req_get_old_owner_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_file_req_target_id* prev, struct ternfs_rename_file_req_old_owner_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_rename_file_req_get_old_owner_id(ctx, prev, next) \
    struct ternfs_rename_file_req_old_owner_id next; \
    _ternfs_rename_file_req_get_old_owner_id(ctx, &(prev), &(next))

struct ternfs_rename_file_req_old_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_rename_file_req_get_old_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_file_req_old_owner_id* prev, struct ternfs_rename_file_req_old_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_rename_file_req_get_old_name(ctx, prev, next) \
    struct ternfs_rename_file_req_old_name next; \
    _ternfs_rename_file_req_get_old_name(ctx, &(prev), &(next))

struct ternfs_rename_file_req_old_creation_time { u64 x; };
static inline void _ternfs_rename_file_req_get_old_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_file_req_old_name* prev, struct ternfs_rename_file_req_old_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_rename_file_req_get_old_creation_time(ctx, prev, next) \
    struct ternfs_rename_file_req_old_creation_time next; \
    _ternfs_rename_file_req_get_old_creation_time(ctx, &(prev), &(next))

struct ternfs_rename_file_req_new_owner_id { u64 x; };
static inline void _ternfs_rename_file_req_get_new_owner_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_file_req_old_creation_time* prev, struct ternfs_rename_file_req_new_owner_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_rename_file_req_get_new_owner_id(ctx, prev, next) \
    struct ternfs_rename_file_req_new_owner_id next; \
    _ternfs_rename_file_req_get_new_owner_id(ctx, &(prev), &(next))

struct ternfs_rename_file_req_new_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_rename_file_req_get_new_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_file_req_new_owner_id* prev, struct ternfs_rename_file_req_new_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_rename_file_req_get_new_name(ctx, prev, next) \
    struct ternfs_rename_file_req_new_name next; \
    _ternfs_rename_file_req_get_new_name(ctx, &(prev), &(next))

struct ternfs_rename_file_req_end;
#define ternfs_rename_file_req_get_end(ctx, prev, next) \
    { struct ternfs_rename_file_req_new_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_rename_file_req_end* next = NULL

static inline void ternfs_rename_file_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_file_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_rename_file_req_put_start(ctx, start) struct ternfs_rename_file_req_start* start = NULL

static inline void _ternfs_rename_file_req_put_target_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_file_req_start** prev, struct ternfs_rename_file_req_target_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_rename_file_req_put_target_id(ctx, prev, next, x) \
    struct ternfs_rename_file_req_target_id next; \
    _ternfs_rename_file_req_put_target_id(ctx, &(prev), &(next), x)

static inline void _ternfs_rename_file_req_put_old_owner_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_file_req_target_id* prev, struct ternfs_rename_file_req_old_owner_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_rename_file_req_put_old_owner_id(ctx, prev, next, x) \
    struct ternfs_rename_file_req_old_owner_id next; \
    _ternfs_rename_file_req_put_old_owner_id(ctx, &(prev), &(next), x)

static inline void _ternfs_rename_file_req_put_old_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_file_req_old_owner_id* prev, struct ternfs_rename_file_req_old_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_rename_file_req_put_old_name(ctx, prev, next, str, str_len) \
    struct ternfs_rename_file_req_old_name next; \
    _ternfs_rename_file_req_put_old_name(ctx, &(prev), &(next), str, str_len)

static inline void _ternfs_rename_file_req_put_old_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_file_req_old_name* prev, struct ternfs_rename_file_req_old_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_rename_file_req_put_old_creation_time(ctx, prev, next, x) \
    struct ternfs_rename_file_req_old_creation_time next; \
    _ternfs_rename_file_req_put_old_creation_time(ctx, &(prev), &(next), x)

static inline void _ternfs_rename_file_req_put_new_owner_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_file_req_old_creation_time* prev, struct ternfs_rename_file_req_new_owner_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_rename_file_req_put_new_owner_id(ctx, prev, next, x) \
    struct ternfs_rename_file_req_new_owner_id next; \
    _ternfs_rename_file_req_put_new_owner_id(ctx, &(prev), &(next), x)

static inline void _ternfs_rename_file_req_put_new_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_file_req_new_owner_id* prev, struct ternfs_rename_file_req_new_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_rename_file_req_put_new_name(ctx, prev, next, str, str_len) \
    struct ternfs_rename_file_req_new_name next; \
    _ternfs_rename_file_req_put_new_name(ctx, &(prev), &(next), str, str_len)

#define ternfs_rename_file_req_put_end(ctx, prev, next) \
    { struct ternfs_rename_file_req_new_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_rename_file_req_end* next __attribute__((unused)) = NULL

#define TERNFS_RENAME_FILE_RESP_SIZE 8
struct ternfs_rename_file_resp_start;
#define ternfs_rename_file_resp_get_start(ctx, start) struct ternfs_rename_file_resp_start* start = NULL

struct ternfs_rename_file_resp_creation_time { u64 x; };
static inline void _ternfs_rename_file_resp_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_file_resp_start** prev, struct ternfs_rename_file_resp_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_rename_file_resp_get_creation_time(ctx, prev, next) \
    struct ternfs_rename_file_resp_creation_time next; \
    _ternfs_rename_file_resp_get_creation_time(ctx, &(prev), &(next))

struct ternfs_rename_file_resp_end;
#define ternfs_rename_file_resp_get_end(ctx, prev, next) \
    { struct ternfs_rename_file_resp_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_rename_file_resp_end* next = NULL

static inline void ternfs_rename_file_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_file_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_rename_file_resp_put_start(ctx, start) struct ternfs_rename_file_resp_start* start = NULL

static inline void _ternfs_rename_file_resp_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_file_resp_start** prev, struct ternfs_rename_file_resp_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_rename_file_resp_put_creation_time(ctx, prev, next, x) \
    struct ternfs_rename_file_resp_creation_time next; \
    _ternfs_rename_file_resp_put_creation_time(ctx, &(prev), &(next), x)

#define ternfs_rename_file_resp_put_end(ctx, prev, next) \
    { struct ternfs_rename_file_resp_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_rename_file_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_SOFT_UNLINK_DIRECTORY_REQ_MAX_SIZE 280
struct ternfs_soft_unlink_directory_req_start;
#define ternfs_soft_unlink_directory_req_get_start(ctx, start) struct ternfs_soft_unlink_directory_req_start* start = NULL

struct ternfs_soft_unlink_directory_req_owner_id { u64 x; };
static inline void _ternfs_soft_unlink_directory_req_get_owner_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_directory_req_start** prev, struct ternfs_soft_unlink_directory_req_owner_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_soft_unlink_directory_req_get_owner_id(ctx, prev, next) \
    struct ternfs_soft_unlink_directory_req_owner_id next; \
    _ternfs_soft_unlink_directory_req_get_owner_id(ctx, &(prev), &(next))

struct ternfs_soft_unlink_directory_req_target_id { u64 x; };
static inline void _ternfs_soft_unlink_directory_req_get_target_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_directory_req_owner_id* prev, struct ternfs_soft_unlink_directory_req_target_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_soft_unlink_directory_req_get_target_id(ctx, prev, next) \
    struct ternfs_soft_unlink_directory_req_target_id next; \
    _ternfs_soft_unlink_directory_req_get_target_id(ctx, &(prev), &(next))

struct ternfs_soft_unlink_directory_req_creation_time { u64 x; };
static inline void _ternfs_soft_unlink_directory_req_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_directory_req_target_id* prev, struct ternfs_soft_unlink_directory_req_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_soft_unlink_directory_req_get_creation_time(ctx, prev, next) \
    struct ternfs_soft_unlink_directory_req_creation_time next; \
    _ternfs_soft_unlink_directory_req_get_creation_time(ctx, &(prev), &(next))

struct ternfs_soft_unlink_directory_req_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_soft_unlink_directory_req_get_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_directory_req_creation_time* prev, struct ternfs_soft_unlink_directory_req_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_soft_unlink_directory_req_get_name(ctx, prev, next) \
    struct ternfs_soft_unlink_directory_req_name next; \
    _ternfs_soft_unlink_directory_req_get_name(ctx, &(prev), &(next))

struct ternfs_soft_unlink_directory_req_end;
#define ternfs_soft_unlink_directory_req_get_end(ctx, prev, next) \
    { struct ternfs_soft_unlink_directory_req_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_soft_unlink_directory_req_end* next = NULL

static inline void ternfs_soft_unlink_directory_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_directory_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_soft_unlink_directory_req_put_start(ctx, start) struct ternfs_soft_unlink_directory_req_start* start = NULL

static inline void _ternfs_soft_unlink_directory_req_put_owner_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_soft_unlink_directory_req_start** prev, struct ternfs_soft_unlink_directory_req_owner_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_soft_unlink_directory_req_put_owner_id(ctx, prev, next, x) \
    struct ternfs_soft_unlink_directory_req_owner_id next; \
    _ternfs_soft_unlink_directory_req_put_owner_id(ctx, &(prev), &(next), x)

static inline void _ternfs_soft_unlink_directory_req_put_target_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_soft_unlink_directory_req_owner_id* prev, struct ternfs_soft_unlink_directory_req_target_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_soft_unlink_directory_req_put_target_id(ctx, prev, next, x) \
    struct ternfs_soft_unlink_directory_req_target_id next; \
    _ternfs_soft_unlink_directory_req_put_target_id(ctx, &(prev), &(next), x)

static inline void _ternfs_soft_unlink_directory_req_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_soft_unlink_directory_req_target_id* prev, struct ternfs_soft_unlink_directory_req_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_soft_unlink_directory_req_put_creation_time(ctx, prev, next, x) \
    struct ternfs_soft_unlink_directory_req_creation_time next; \
    _ternfs_soft_unlink_directory_req_put_creation_time(ctx, &(prev), &(next), x)

static inline void _ternfs_soft_unlink_directory_req_put_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_soft_unlink_directory_req_creation_time* prev, struct ternfs_soft_unlink_directory_req_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_soft_unlink_directory_req_put_name(ctx, prev, next, str, str_len) \
    struct ternfs_soft_unlink_directory_req_name next; \
    _ternfs_soft_unlink_directory_req_put_name(ctx, &(prev), &(next), str, str_len)

#define ternfs_soft_unlink_directory_req_put_end(ctx, prev, next) \
    { struct ternfs_soft_unlink_directory_req_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_soft_unlink_directory_req_end* next __attribute__((unused)) = NULL

#define TERNFS_SOFT_UNLINK_DIRECTORY_RESP_SIZE 0
struct ternfs_soft_unlink_directory_resp_start;
#define ternfs_soft_unlink_directory_resp_get_start(ctx, start) struct ternfs_soft_unlink_directory_resp_start* start = NULL

struct ternfs_soft_unlink_directory_resp_end;
#define ternfs_soft_unlink_directory_resp_get_end(ctx, prev, next) \
    { struct ternfs_soft_unlink_directory_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_soft_unlink_directory_resp_end* next = NULL

static inline void ternfs_soft_unlink_directory_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_soft_unlink_directory_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_soft_unlink_directory_resp_put_start(ctx, start) struct ternfs_soft_unlink_directory_resp_start* start = NULL

#define ternfs_soft_unlink_directory_resp_put_end(ctx, prev, next) \
    { struct ternfs_soft_unlink_directory_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_soft_unlink_directory_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_RENAME_DIRECTORY_REQ_MAX_SIZE 544
struct ternfs_rename_directory_req_start;
#define ternfs_rename_directory_req_get_start(ctx, start) struct ternfs_rename_directory_req_start* start = NULL

struct ternfs_rename_directory_req_target_id { u64 x; };
static inline void _ternfs_rename_directory_req_get_target_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_directory_req_start** prev, struct ternfs_rename_directory_req_target_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_rename_directory_req_get_target_id(ctx, prev, next) \
    struct ternfs_rename_directory_req_target_id next; \
    _ternfs_rename_directory_req_get_target_id(ctx, &(prev), &(next))

struct ternfs_rename_directory_req_old_owner_id { u64 x; };
static inline void _ternfs_rename_directory_req_get_old_owner_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_directory_req_target_id* prev, struct ternfs_rename_directory_req_old_owner_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_rename_directory_req_get_old_owner_id(ctx, prev, next) \
    struct ternfs_rename_directory_req_old_owner_id next; \
    _ternfs_rename_directory_req_get_old_owner_id(ctx, &(prev), &(next))

struct ternfs_rename_directory_req_old_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_rename_directory_req_get_old_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_directory_req_old_owner_id* prev, struct ternfs_rename_directory_req_old_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_rename_directory_req_get_old_name(ctx, prev, next) \
    struct ternfs_rename_directory_req_old_name next; \
    _ternfs_rename_directory_req_get_old_name(ctx, &(prev), &(next))

struct ternfs_rename_directory_req_old_creation_time { u64 x; };
static inline void _ternfs_rename_directory_req_get_old_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_directory_req_old_name* prev, struct ternfs_rename_directory_req_old_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_rename_directory_req_get_old_creation_time(ctx, prev, next) \
    struct ternfs_rename_directory_req_old_creation_time next; \
    _ternfs_rename_directory_req_get_old_creation_time(ctx, &(prev), &(next))

struct ternfs_rename_directory_req_new_owner_id { u64 x; };
static inline void _ternfs_rename_directory_req_get_new_owner_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_directory_req_old_creation_time* prev, struct ternfs_rename_directory_req_new_owner_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_rename_directory_req_get_new_owner_id(ctx, prev, next) \
    struct ternfs_rename_directory_req_new_owner_id next; \
    _ternfs_rename_directory_req_get_new_owner_id(ctx, &(prev), &(next))

struct ternfs_rename_directory_req_new_name { struct ternfs_bincode_bytes str; };
static inline void _ternfs_rename_directory_req_get_new_name(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_directory_req_new_owner_id* prev, struct ternfs_rename_directory_req_new_name* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 1)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->str.len = *(u8*)(ctx->buf);
            ctx->buf++;
            if (unlikely(ctx->end - ctx->buf < next->str.len)) {
                ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            } else {
                next->str.buf = ctx->buf;
                ctx->buf += next->str.len;
            }
        }
    }
}
#define ternfs_rename_directory_req_get_new_name(ctx, prev, next) \
    struct ternfs_rename_directory_req_new_name next; \
    _ternfs_rename_directory_req_get_new_name(ctx, &(prev), &(next))

struct ternfs_rename_directory_req_end;
#define ternfs_rename_directory_req_get_end(ctx, prev, next) \
    { struct ternfs_rename_directory_req_new_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_rename_directory_req_end* next = NULL

static inline void ternfs_rename_directory_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_directory_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_rename_directory_req_put_start(ctx, start) struct ternfs_rename_directory_req_start* start = NULL

static inline void _ternfs_rename_directory_req_put_target_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_directory_req_start** prev, struct ternfs_rename_directory_req_target_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_rename_directory_req_put_target_id(ctx, prev, next, x) \
    struct ternfs_rename_directory_req_target_id next; \
    _ternfs_rename_directory_req_put_target_id(ctx, &(prev), &(next), x)

static inline void _ternfs_rename_directory_req_put_old_owner_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_directory_req_target_id* prev, struct ternfs_rename_directory_req_old_owner_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_rename_directory_req_put_old_owner_id(ctx, prev, next, x) \
    struct ternfs_rename_directory_req_old_owner_id next; \
    _ternfs_rename_directory_req_put_old_owner_id(ctx, &(prev), &(next), x)

static inline void _ternfs_rename_directory_req_put_old_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_directory_req_old_owner_id* prev, struct ternfs_rename_directory_req_old_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_rename_directory_req_put_old_name(ctx, prev, next, str, str_len) \
    struct ternfs_rename_directory_req_old_name next; \
    _ternfs_rename_directory_req_put_old_name(ctx, &(prev), &(next), str, str_len)

static inline void _ternfs_rename_directory_req_put_old_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_directory_req_old_name* prev, struct ternfs_rename_directory_req_old_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_rename_directory_req_put_old_creation_time(ctx, prev, next, x) \
    struct ternfs_rename_directory_req_old_creation_time next; \
    _ternfs_rename_directory_req_put_old_creation_time(ctx, &(prev), &(next), x)

static inline void _ternfs_rename_directory_req_put_new_owner_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_directory_req_old_creation_time* prev, struct ternfs_rename_directory_req_new_owner_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_rename_directory_req_put_new_owner_id(ctx, prev, next, x) \
    struct ternfs_rename_directory_req_new_owner_id next; \
    _ternfs_rename_directory_req_put_new_owner_id(ctx, &(prev), &(next), x)

static inline void _ternfs_rename_directory_req_put_new_name(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_directory_req_new_owner_id* prev, struct ternfs_rename_directory_req_new_name* next, const char* str, int str_len) {
    next = NULL;
    BUG_ON(str_len < 0 || str_len > 255);
    BUG_ON(ctx->end - ctx->cursor < (1 + str_len));
    *(u8*)(ctx->cursor) = str_len;
    memcpy(ctx->cursor + 1, str, str_len);
    ctx->cursor += 1 + str_len;
}
#define ternfs_rename_directory_req_put_new_name(ctx, prev, next, str, str_len) \
    struct ternfs_rename_directory_req_new_name next; \
    _ternfs_rename_directory_req_put_new_name(ctx, &(prev), &(next), str, str_len)

#define ternfs_rename_directory_req_put_end(ctx, prev, next) \
    { struct ternfs_rename_directory_req_new_name* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_rename_directory_req_end* next __attribute__((unused)) = NULL

#define TERNFS_RENAME_DIRECTORY_RESP_SIZE 8
struct ternfs_rename_directory_resp_start;
#define ternfs_rename_directory_resp_get_start(ctx, start) struct ternfs_rename_directory_resp_start* start = NULL

struct ternfs_rename_directory_resp_creation_time { u64 x; };
static inline void _ternfs_rename_directory_resp_get_creation_time(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_directory_resp_start** prev, struct ternfs_rename_directory_resp_creation_time* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_rename_directory_resp_get_creation_time(ctx, prev, next) \
    struct ternfs_rename_directory_resp_creation_time next; \
    _ternfs_rename_directory_resp_get_creation_time(ctx, &(prev), &(next))

struct ternfs_rename_directory_resp_end;
#define ternfs_rename_directory_resp_get_end(ctx, prev, next) \
    { struct ternfs_rename_directory_resp_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_rename_directory_resp_end* next = NULL

static inline void ternfs_rename_directory_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_rename_directory_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_rename_directory_resp_put_start(ctx, start) struct ternfs_rename_directory_resp_start* start = NULL

static inline void _ternfs_rename_directory_resp_put_creation_time(struct ternfs_bincode_put_ctx* ctx, struct ternfs_rename_directory_resp_start** prev, struct ternfs_rename_directory_resp_creation_time* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_rename_directory_resp_put_creation_time(ctx, prev, next, x) \
    struct ternfs_rename_directory_resp_creation_time next; \
    _ternfs_rename_directory_resp_put_creation_time(ctx, &(prev), &(next), x)

#define ternfs_rename_directory_resp_put_end(ctx, prev, next) \
    { struct ternfs_rename_directory_resp_creation_time* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_rename_directory_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_LOCAL_SHARDS_REQ_SIZE 0
struct ternfs_local_shards_req_start;
#define ternfs_local_shards_req_get_start(ctx, start) struct ternfs_local_shards_req_start* start = NULL

struct ternfs_local_shards_req_end;
#define ternfs_local_shards_req_get_end(ctx, prev, next) \
    { struct ternfs_local_shards_req_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_shards_req_end* next = NULL

static inline void ternfs_local_shards_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_shards_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_local_shards_req_put_start(ctx, start) struct ternfs_local_shards_req_start* start = NULL

#define ternfs_local_shards_req_put_end(ctx, prev, next) \
    { struct ternfs_local_shards_req_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_shards_req_end* next __attribute__((unused)) = NULL

struct ternfs_local_shards_resp_start;
#define ternfs_local_shards_resp_get_start(ctx, start) struct ternfs_local_shards_resp_start* start = NULL

struct ternfs_local_shards_resp_shards { u16 len; };
static inline void _ternfs_local_shards_resp_get_shards(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_shards_resp_start** prev, struct ternfs_local_shards_resp_shards* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_local_shards_resp_get_shards(ctx, prev, next) \
    struct ternfs_local_shards_resp_shards next; \
    _ternfs_local_shards_resp_get_shards(ctx, &(prev), &(next))

struct ternfs_local_shards_resp_end;
#define ternfs_local_shards_resp_get_end(ctx, prev, next) \
    { struct ternfs_local_shards_resp_shards* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_shards_resp_end* next = NULL

static inline void ternfs_local_shards_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_shards_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_local_shards_resp_put_start(ctx, start) struct ternfs_local_shards_resp_start* start = NULL

static inline void _ternfs_local_shards_resp_put_shards(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_shards_resp_start** prev, struct ternfs_local_shards_resp_shards* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_local_shards_resp_put_shards(ctx, prev, next, len) \
    struct ternfs_local_shards_resp_shards next; \
    _ternfs_local_shards_resp_put_shards(ctx, &(prev), &(next), len)

#define ternfs_local_shards_resp_put_end(ctx, prev, next) \
    { struct ternfs_local_shards_resp_shards* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_shards_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_LOCAL_CDC_REQ_SIZE 0
struct ternfs_local_cdc_req_start;
#define ternfs_local_cdc_req_get_start(ctx, start) struct ternfs_local_cdc_req_start* start = NULL

struct ternfs_local_cdc_req_end;
#define ternfs_local_cdc_req_get_end(ctx, prev, next) \
    { struct ternfs_local_cdc_req_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_cdc_req_end* next = NULL

static inline void ternfs_local_cdc_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_cdc_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_local_cdc_req_put_start(ctx, start) struct ternfs_local_cdc_req_start* start = NULL

#define ternfs_local_cdc_req_put_end(ctx, prev, next) \
    { struct ternfs_local_cdc_req_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_cdc_req_end* next __attribute__((unused)) = NULL

#define TERNFS_LOCAL_CDC_RESP_SIZE 20
struct ternfs_local_cdc_resp_start;
#define ternfs_local_cdc_resp_get_start(ctx, start) struct ternfs_local_cdc_resp_start* start = NULL

#define ternfs_local_cdc_resp_get_addrs(ctx, prev, next) \
    { struct ternfs_local_cdc_resp_start** __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_addrs_info_start* next = NULL

struct ternfs_local_cdc_resp_last_seen { u64 x; };
static inline void _ternfs_local_cdc_resp_get_last_seen(struct ternfs_bincode_get_ctx* ctx, struct ternfs_addrs_info_end** prev, struct ternfs_local_cdc_resp_last_seen* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_local_cdc_resp_get_last_seen(ctx, prev, next) \
    struct ternfs_local_cdc_resp_last_seen next; \
    _ternfs_local_cdc_resp_get_last_seen(ctx, &(prev), &(next))

struct ternfs_local_cdc_resp_end;
#define ternfs_local_cdc_resp_get_end(ctx, prev, next) \
    { struct ternfs_local_cdc_resp_last_seen* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_cdc_resp_end* next = NULL

static inline void ternfs_local_cdc_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_cdc_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_local_cdc_resp_put_start(ctx, start) struct ternfs_local_cdc_resp_start* start = NULL

static inline void _ternfs_local_cdc_resp_put_last_seen(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_cdc_resp_start** prev, struct ternfs_local_cdc_resp_last_seen* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_local_cdc_resp_put_last_seen(ctx, prev, next, x) \
    struct ternfs_local_cdc_resp_last_seen next; \
    _ternfs_local_cdc_resp_put_last_seen(ctx, &(prev), &(next), x)

#define ternfs_local_cdc_resp_put_end(ctx, prev, next) \
    { struct ternfs_local_cdc_resp_last_seen* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_cdc_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_INFO_REQ_SIZE 0
struct ternfs_info_req_start;
#define ternfs_info_req_get_start(ctx, start) struct ternfs_info_req_start* start = NULL

struct ternfs_info_req_end;
#define ternfs_info_req_get_end(ctx, prev, next) \
    { struct ternfs_info_req_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_info_req_end* next = NULL

static inline void ternfs_info_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_info_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_info_req_put_start(ctx, start) struct ternfs_info_req_start* start = NULL

#define ternfs_info_req_put_end(ctx, prev, next) \
    { struct ternfs_info_req_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_info_req_end* next __attribute__((unused)) = NULL

#define TERNFS_INFO_RESP_SIZE 32
struct ternfs_info_resp_start;
#define ternfs_info_resp_get_start(ctx, start) struct ternfs_info_resp_start* start = NULL

struct ternfs_info_resp_num_block_services { u32 x; };
static inline void _ternfs_info_resp_get_num_block_services(struct ternfs_bincode_get_ctx* ctx, struct ternfs_info_resp_start** prev, struct ternfs_info_resp_num_block_services* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_info_resp_get_num_block_services(ctx, prev, next) \
    struct ternfs_info_resp_num_block_services next; \
    _ternfs_info_resp_get_num_block_services(ctx, &(prev), &(next))

struct ternfs_info_resp_num_failure_domains { u32 x; };
static inline void _ternfs_info_resp_get_num_failure_domains(struct ternfs_bincode_get_ctx* ctx, struct ternfs_info_resp_num_block_services* prev, struct ternfs_info_resp_num_failure_domains* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_info_resp_get_num_failure_domains(ctx, prev, next) \
    struct ternfs_info_resp_num_failure_domains next; \
    _ternfs_info_resp_get_num_failure_domains(ctx, &(prev), &(next))

struct ternfs_info_resp_capacity { u64 x; };
static inline void _ternfs_info_resp_get_capacity(struct ternfs_bincode_get_ctx* ctx, struct ternfs_info_resp_num_failure_domains* prev, struct ternfs_info_resp_capacity* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_info_resp_get_capacity(ctx, prev, next) \
    struct ternfs_info_resp_capacity next; \
    _ternfs_info_resp_get_capacity(ctx, &(prev), &(next))

struct ternfs_info_resp_available { u64 x; };
static inline void _ternfs_info_resp_get_available(struct ternfs_bincode_get_ctx* ctx, struct ternfs_info_resp_capacity* prev, struct ternfs_info_resp_available* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_info_resp_get_available(ctx, prev, next) \
    struct ternfs_info_resp_available next; \
    _ternfs_info_resp_get_available(ctx, &(prev), &(next))

struct ternfs_info_resp_blocks { u64 x; };
static inline void _ternfs_info_resp_get_blocks(struct ternfs_bincode_get_ctx* ctx, struct ternfs_info_resp_available* prev, struct ternfs_info_resp_blocks* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_info_resp_get_blocks(ctx, prev, next) \
    struct ternfs_info_resp_blocks next; \
    _ternfs_info_resp_get_blocks(ctx, &(prev), &(next))

struct ternfs_info_resp_end;
#define ternfs_info_resp_get_end(ctx, prev, next) \
    { struct ternfs_info_resp_blocks* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_info_resp_end* next = NULL

static inline void ternfs_info_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_info_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_info_resp_put_start(ctx, start) struct ternfs_info_resp_start* start = NULL

static inline void _ternfs_info_resp_put_num_block_services(struct ternfs_bincode_put_ctx* ctx, struct ternfs_info_resp_start** prev, struct ternfs_info_resp_num_block_services* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_info_resp_put_num_block_services(ctx, prev, next, x) \
    struct ternfs_info_resp_num_block_services next; \
    _ternfs_info_resp_put_num_block_services(ctx, &(prev), &(next), x)

static inline void _ternfs_info_resp_put_num_failure_domains(struct ternfs_bincode_put_ctx* ctx, struct ternfs_info_resp_num_block_services* prev, struct ternfs_info_resp_num_failure_domains* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_info_resp_put_num_failure_domains(ctx, prev, next, x) \
    struct ternfs_info_resp_num_failure_domains next; \
    _ternfs_info_resp_put_num_failure_domains(ctx, &(prev), &(next), x)

static inline void _ternfs_info_resp_put_capacity(struct ternfs_bincode_put_ctx* ctx, struct ternfs_info_resp_num_failure_domains* prev, struct ternfs_info_resp_capacity* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_info_resp_put_capacity(ctx, prev, next, x) \
    struct ternfs_info_resp_capacity next; \
    _ternfs_info_resp_put_capacity(ctx, &(prev), &(next), x)

static inline void _ternfs_info_resp_put_available(struct ternfs_bincode_put_ctx* ctx, struct ternfs_info_resp_capacity* prev, struct ternfs_info_resp_available* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_info_resp_put_available(ctx, prev, next, x) \
    struct ternfs_info_resp_available next; \
    _ternfs_info_resp_put_available(ctx, &(prev), &(next), x)

static inline void _ternfs_info_resp_put_blocks(struct ternfs_bincode_put_ctx* ctx, struct ternfs_info_resp_available* prev, struct ternfs_info_resp_blocks* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_info_resp_put_blocks(ctx, prev, next, x) \
    struct ternfs_info_resp_blocks next; \
    _ternfs_info_resp_put_blocks(ctx, &(prev), &(next), x)

#define ternfs_info_resp_put_end(ctx, prev, next) \
    { struct ternfs_info_resp_blocks* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_info_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_REGISTRY_REQ_SIZE 0
struct ternfs_registry_req_start;
#define ternfs_registry_req_get_start(ctx, start) struct ternfs_registry_req_start* start = NULL

struct ternfs_registry_req_end;
#define ternfs_registry_req_get_end(ctx, prev, next) \
    { struct ternfs_registry_req_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_registry_req_end* next = NULL

static inline void ternfs_registry_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_registry_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_registry_req_put_start(ctx, start) struct ternfs_registry_req_start* start = NULL

#define ternfs_registry_req_put_end(ctx, prev, next) \
    { struct ternfs_registry_req_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_registry_req_end* next __attribute__((unused)) = NULL

#define TERNFS_REGISTRY_RESP_SIZE 12
struct ternfs_registry_resp_start;
#define ternfs_registry_resp_get_start(ctx, start) struct ternfs_registry_resp_start* start = NULL

#define ternfs_registry_resp_get_addrs(ctx, prev, next) \
    { struct ternfs_registry_resp_start** __dummy __attribute__((unused)) = &(prev); }; \
    struct ternfs_addrs_info_start* next = NULL

struct ternfs_registry_resp_end;
#define ternfs_registry_resp_get_end(ctx, prev, next) \
    { struct ternfs_addrs_info_end** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_registry_resp_end* next = NULL

static inline void ternfs_registry_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_registry_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_registry_resp_put_start(ctx, start) struct ternfs_registry_resp_start* start = NULL

#define ternfs_registry_resp_put_end(ctx, prev, next) \
    { struct ternfs_registry_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_registry_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_LOCAL_CHANGED_BLOCK_SERVICES_REQ_SIZE 8
struct ternfs_local_changed_block_services_req_start;
#define ternfs_local_changed_block_services_req_get_start(ctx, start) struct ternfs_local_changed_block_services_req_start* start = NULL

struct ternfs_local_changed_block_services_req_changed_since { u64 x; };
static inline void _ternfs_local_changed_block_services_req_get_changed_since(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_changed_block_services_req_start** prev, struct ternfs_local_changed_block_services_req_changed_since* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_local_changed_block_services_req_get_changed_since(ctx, prev, next) \
    struct ternfs_local_changed_block_services_req_changed_since next; \
    _ternfs_local_changed_block_services_req_get_changed_since(ctx, &(prev), &(next))

struct ternfs_local_changed_block_services_req_end;
#define ternfs_local_changed_block_services_req_get_end(ctx, prev, next) \
    { struct ternfs_local_changed_block_services_req_changed_since* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_changed_block_services_req_end* next = NULL

static inline void ternfs_local_changed_block_services_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_changed_block_services_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_local_changed_block_services_req_put_start(ctx, start) struct ternfs_local_changed_block_services_req_start* start = NULL

static inline void _ternfs_local_changed_block_services_req_put_changed_since(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_changed_block_services_req_start** prev, struct ternfs_local_changed_block_services_req_changed_since* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_local_changed_block_services_req_put_changed_since(ctx, prev, next, x) \
    struct ternfs_local_changed_block_services_req_changed_since next; \
    _ternfs_local_changed_block_services_req_put_changed_since(ctx, &(prev), &(next), x)

#define ternfs_local_changed_block_services_req_put_end(ctx, prev, next) \
    { struct ternfs_local_changed_block_services_req_changed_since* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_changed_block_services_req_end* next __attribute__((unused)) = NULL

struct ternfs_local_changed_block_services_resp_start;
#define ternfs_local_changed_block_services_resp_get_start(ctx, start) struct ternfs_local_changed_block_services_resp_start* start = NULL

struct ternfs_local_changed_block_services_resp_last_change { u64 x; };
static inline void _ternfs_local_changed_block_services_resp_get_last_change(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_changed_block_services_resp_start** prev, struct ternfs_local_changed_block_services_resp_last_change* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_local_changed_block_services_resp_get_last_change(ctx, prev, next) \
    struct ternfs_local_changed_block_services_resp_last_change next; \
    _ternfs_local_changed_block_services_resp_get_last_change(ctx, &(prev), &(next))

struct ternfs_local_changed_block_services_resp_block_services { u16 len; };
static inline void _ternfs_local_changed_block_services_resp_get_block_services(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_changed_block_services_resp_last_change* prev, struct ternfs_local_changed_block_services_resp_block_services* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 2)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->len = get_unaligned_le16(ctx->buf);
            ctx->buf += 2;
        }
    } else {
        next->len = 0;
    }
}
#define ternfs_local_changed_block_services_resp_get_block_services(ctx, prev, next) \
    struct ternfs_local_changed_block_services_resp_block_services next; \
    _ternfs_local_changed_block_services_resp_get_block_services(ctx, &(prev), &(next))

struct ternfs_local_changed_block_services_resp_end;
#define ternfs_local_changed_block_services_resp_get_end(ctx, prev, next) \
    { struct ternfs_local_changed_block_services_resp_block_services* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_changed_block_services_resp_end* next = NULL

static inline void ternfs_local_changed_block_services_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_local_changed_block_services_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_local_changed_block_services_resp_put_start(ctx, start) struct ternfs_local_changed_block_services_resp_start* start = NULL

static inline void _ternfs_local_changed_block_services_resp_put_last_change(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_changed_block_services_resp_start** prev, struct ternfs_local_changed_block_services_resp_last_change* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_local_changed_block_services_resp_put_last_change(ctx, prev, next, x) \
    struct ternfs_local_changed_block_services_resp_last_change next; \
    _ternfs_local_changed_block_services_resp_put_last_change(ctx, &(prev), &(next), x)

static inline void _ternfs_local_changed_block_services_resp_put_block_services(struct ternfs_bincode_put_ctx* ctx, struct ternfs_local_changed_block_services_resp_last_change* prev, struct ternfs_local_changed_block_services_resp_block_services* next, int len) {
    next = NULL;
    BUG_ON(len < 0 || len >= 1<<16);
    BUG_ON(ctx->end - ctx->cursor < 2);
    put_unaligned_le16(len, ctx->cursor);
    ctx->cursor += 2;
}
#define ternfs_local_changed_block_services_resp_put_block_services(ctx, prev, next, len) \
    struct ternfs_local_changed_block_services_resp_block_services next; \
    _ternfs_local_changed_block_services_resp_put_block_services(ctx, &(prev), &(next), len)

#define ternfs_local_changed_block_services_resp_put_end(ctx, prev, next) \
    { struct ternfs_local_changed_block_services_resp_block_services* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_local_changed_block_services_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_FETCH_BLOCK_REQ_SIZE 16
struct ternfs_fetch_block_req_start;
#define ternfs_fetch_block_req_get_start(ctx, start) struct ternfs_fetch_block_req_start* start = NULL

struct ternfs_fetch_block_req_block_id { u64 x; };
static inline void _ternfs_fetch_block_req_get_block_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_req_start** prev, struct ternfs_fetch_block_req_block_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_fetch_block_req_get_block_id(ctx, prev, next) \
    struct ternfs_fetch_block_req_block_id next; \
    _ternfs_fetch_block_req_get_block_id(ctx, &(prev), &(next))

struct ternfs_fetch_block_req_offset { u32 x; };
static inline void _ternfs_fetch_block_req_get_offset(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_req_block_id* prev, struct ternfs_fetch_block_req_offset* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetch_block_req_get_offset(ctx, prev, next) \
    struct ternfs_fetch_block_req_offset next; \
    _ternfs_fetch_block_req_get_offset(ctx, &(prev), &(next))

struct ternfs_fetch_block_req_count { u32 x; };
static inline void _ternfs_fetch_block_req_get_count(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_req_offset* prev, struct ternfs_fetch_block_req_count* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetch_block_req_get_count(ctx, prev, next) \
    struct ternfs_fetch_block_req_count next; \
    _ternfs_fetch_block_req_get_count(ctx, &(prev), &(next))

struct ternfs_fetch_block_req_end;
#define ternfs_fetch_block_req_get_end(ctx, prev, next) \
    { struct ternfs_fetch_block_req_count* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetch_block_req_end* next = NULL

static inline void ternfs_fetch_block_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetch_block_req_put_start(ctx, start) struct ternfs_fetch_block_req_start* start = NULL

static inline void _ternfs_fetch_block_req_put_block_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetch_block_req_start** prev, struct ternfs_fetch_block_req_block_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_fetch_block_req_put_block_id(ctx, prev, next, x) \
    struct ternfs_fetch_block_req_block_id next; \
    _ternfs_fetch_block_req_put_block_id(ctx, &(prev), &(next), x)

static inline void _ternfs_fetch_block_req_put_offset(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetch_block_req_block_id* prev, struct ternfs_fetch_block_req_offset* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetch_block_req_put_offset(ctx, prev, next, x) \
    struct ternfs_fetch_block_req_offset next; \
    _ternfs_fetch_block_req_put_offset(ctx, &(prev), &(next), x)

static inline void _ternfs_fetch_block_req_put_count(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetch_block_req_offset* prev, struct ternfs_fetch_block_req_count* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetch_block_req_put_count(ctx, prev, next, x) \
    struct ternfs_fetch_block_req_count next; \
    _ternfs_fetch_block_req_put_count(ctx, &(prev), &(next), x)

#define ternfs_fetch_block_req_put_end(ctx, prev, next) \
    { struct ternfs_fetch_block_req_count* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetch_block_req_end* next __attribute__((unused)) = NULL

#define TERNFS_FETCH_BLOCK_RESP_SIZE 0
struct ternfs_fetch_block_resp_start;
#define ternfs_fetch_block_resp_get_start(ctx, start) struct ternfs_fetch_block_resp_start* start = NULL

struct ternfs_fetch_block_resp_end;
#define ternfs_fetch_block_resp_get_end(ctx, prev, next) \
    { struct ternfs_fetch_block_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetch_block_resp_end* next = NULL

static inline void ternfs_fetch_block_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetch_block_resp_put_start(ctx, start) struct ternfs_fetch_block_resp_start* start = NULL

#define ternfs_fetch_block_resp_put_end(ctx, prev, next) \
    { struct ternfs_fetch_block_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetch_block_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_WRITE_BLOCK_REQ_SIZE 24
struct ternfs_write_block_req_start;
#define ternfs_write_block_req_get_start(ctx, start) struct ternfs_write_block_req_start* start = NULL

struct ternfs_write_block_req_block_id { u64 x; };
static inline void _ternfs_write_block_req_get_block_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_write_block_req_start** prev, struct ternfs_write_block_req_block_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_write_block_req_get_block_id(ctx, prev, next) \
    struct ternfs_write_block_req_block_id next; \
    _ternfs_write_block_req_get_block_id(ctx, &(prev), &(next))

struct ternfs_write_block_req_crc { u32 x; };
static inline void _ternfs_write_block_req_get_crc(struct ternfs_bincode_get_ctx* ctx, struct ternfs_write_block_req_block_id* prev, struct ternfs_write_block_req_crc* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_write_block_req_get_crc(ctx, prev, next) \
    struct ternfs_write_block_req_crc next; \
    _ternfs_write_block_req_get_crc(ctx, &(prev), &(next))

struct ternfs_write_block_req_size { u32 x; };
static inline void _ternfs_write_block_req_get_size(struct ternfs_bincode_get_ctx* ctx, struct ternfs_write_block_req_crc* prev, struct ternfs_write_block_req_size* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_write_block_req_get_size(ctx, prev, next) \
    struct ternfs_write_block_req_size next; \
    _ternfs_write_block_req_get_size(ctx, &(prev), &(next))

struct ternfs_write_block_req_certificate { u64 x; };
static inline void _ternfs_write_block_req_get_certificate(struct ternfs_bincode_get_ctx* ctx, struct ternfs_write_block_req_size* prev, struct ternfs_write_block_req_certificate* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_write_block_req_get_certificate(ctx, prev, next) \
    struct ternfs_write_block_req_certificate next; \
    _ternfs_write_block_req_get_certificate(ctx, &(prev), &(next))

struct ternfs_write_block_req_end;
#define ternfs_write_block_req_get_end(ctx, prev, next) \
    { struct ternfs_write_block_req_certificate* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_write_block_req_end* next = NULL

static inline void ternfs_write_block_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_write_block_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_write_block_req_put_start(ctx, start) struct ternfs_write_block_req_start* start = NULL

static inline void _ternfs_write_block_req_put_block_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_write_block_req_start** prev, struct ternfs_write_block_req_block_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_write_block_req_put_block_id(ctx, prev, next, x) \
    struct ternfs_write_block_req_block_id next; \
    _ternfs_write_block_req_put_block_id(ctx, &(prev), &(next), x)

static inline void _ternfs_write_block_req_put_crc(struct ternfs_bincode_put_ctx* ctx, struct ternfs_write_block_req_block_id* prev, struct ternfs_write_block_req_crc* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_write_block_req_put_crc(ctx, prev, next, x) \
    struct ternfs_write_block_req_crc next; \
    _ternfs_write_block_req_put_crc(ctx, &(prev), &(next), x)

static inline void _ternfs_write_block_req_put_size(struct ternfs_bincode_put_ctx* ctx, struct ternfs_write_block_req_crc* prev, struct ternfs_write_block_req_size* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_write_block_req_put_size(ctx, prev, next, x) \
    struct ternfs_write_block_req_size next; \
    _ternfs_write_block_req_put_size(ctx, &(prev), &(next), x)

static inline void _ternfs_write_block_req_put_certificate(struct ternfs_bincode_put_ctx* ctx, struct ternfs_write_block_req_size* prev, struct ternfs_write_block_req_certificate* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_write_block_req_put_certificate(ctx, prev, next, x) \
    struct ternfs_write_block_req_certificate next; \
    _ternfs_write_block_req_put_certificate(ctx, &(prev), &(next), x)

#define ternfs_write_block_req_put_end(ctx, prev, next) \
    { struct ternfs_write_block_req_certificate* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_write_block_req_end* next __attribute__((unused)) = NULL

#define TERNFS_WRITE_BLOCK_RESP_SIZE 8
struct ternfs_write_block_resp_start;
#define ternfs_write_block_resp_get_start(ctx, start) struct ternfs_write_block_resp_start* start = NULL

struct ternfs_write_block_resp_proof { u64 x; };
static inline void _ternfs_write_block_resp_get_proof(struct ternfs_bincode_get_ctx* ctx, struct ternfs_write_block_resp_start** prev, struct ternfs_write_block_resp_proof* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_be64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_write_block_resp_get_proof(ctx, prev, next) \
    struct ternfs_write_block_resp_proof next; \
    _ternfs_write_block_resp_get_proof(ctx, &(prev), &(next))

struct ternfs_write_block_resp_end;
#define ternfs_write_block_resp_get_end(ctx, prev, next) \
    { struct ternfs_write_block_resp_proof* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_write_block_resp_end* next = NULL

static inline void ternfs_write_block_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_write_block_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_write_block_resp_put_start(ctx, start) struct ternfs_write_block_resp_start* start = NULL

static inline void _ternfs_write_block_resp_put_proof(struct ternfs_bincode_put_ctx* ctx, struct ternfs_write_block_resp_start** prev, struct ternfs_write_block_resp_proof* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_be64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_write_block_resp_put_proof(ctx, prev, next, x) \
    struct ternfs_write_block_resp_proof next; \
    _ternfs_write_block_resp_put_proof(ctx, &(prev), &(next), x)

#define ternfs_write_block_resp_put_end(ctx, prev, next) \
    { struct ternfs_write_block_resp_proof* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_write_block_resp_end* next __attribute__((unused)) = NULL

#define TERNFS_FETCH_BLOCK_WITH_CRC_REQ_SIZE 28
struct ternfs_fetch_block_with_crc_req_start;
#define ternfs_fetch_block_with_crc_req_get_start(ctx, start) struct ternfs_fetch_block_with_crc_req_start* start = NULL

struct ternfs_fetch_block_with_crc_req_file_id_unused { u64 x; };
static inline void _ternfs_fetch_block_with_crc_req_get_file_id_unused(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_with_crc_req_start** prev, struct ternfs_fetch_block_with_crc_req_file_id_unused* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_fetch_block_with_crc_req_get_file_id_unused(ctx, prev, next) \
    struct ternfs_fetch_block_with_crc_req_file_id_unused next; \
    _ternfs_fetch_block_with_crc_req_get_file_id_unused(ctx, &(prev), &(next))

struct ternfs_fetch_block_with_crc_req_block_id { u64 x; };
static inline void _ternfs_fetch_block_with_crc_req_get_block_id(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_with_crc_req_file_id_unused* prev, struct ternfs_fetch_block_with_crc_req_block_id* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 8)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le64(ctx->buf);
            ctx->buf += 8;
        }
    }
}
#define ternfs_fetch_block_with_crc_req_get_block_id(ctx, prev, next) \
    struct ternfs_fetch_block_with_crc_req_block_id next; \
    _ternfs_fetch_block_with_crc_req_get_block_id(ctx, &(prev), &(next))

struct ternfs_fetch_block_with_crc_req_block_crc_unused { u32 x; };
static inline void _ternfs_fetch_block_with_crc_req_get_block_crc_unused(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_with_crc_req_block_id* prev, struct ternfs_fetch_block_with_crc_req_block_crc_unused* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetch_block_with_crc_req_get_block_crc_unused(ctx, prev, next) \
    struct ternfs_fetch_block_with_crc_req_block_crc_unused next; \
    _ternfs_fetch_block_with_crc_req_get_block_crc_unused(ctx, &(prev), &(next))

struct ternfs_fetch_block_with_crc_req_offset { u32 x; };
static inline void _ternfs_fetch_block_with_crc_req_get_offset(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_with_crc_req_block_crc_unused* prev, struct ternfs_fetch_block_with_crc_req_offset* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetch_block_with_crc_req_get_offset(ctx, prev, next) \
    struct ternfs_fetch_block_with_crc_req_offset next; \
    _ternfs_fetch_block_with_crc_req_get_offset(ctx, &(prev), &(next))

struct ternfs_fetch_block_with_crc_req_count { u32 x; };
static inline void _ternfs_fetch_block_with_crc_req_get_count(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_with_crc_req_offset* prev, struct ternfs_fetch_block_with_crc_req_count* next) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 4)) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
        } else {
            next->x = get_unaligned_le32(ctx->buf);
            ctx->buf += 4;
        }
    }
}
#define ternfs_fetch_block_with_crc_req_get_count(ctx, prev, next) \
    struct ternfs_fetch_block_with_crc_req_count next; \
    _ternfs_fetch_block_with_crc_req_get_count(ctx, &(prev), &(next))

struct ternfs_fetch_block_with_crc_req_end;
#define ternfs_fetch_block_with_crc_req_get_end(ctx, prev, next) \
    { struct ternfs_fetch_block_with_crc_req_count* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetch_block_with_crc_req_end* next = NULL

static inline void ternfs_fetch_block_with_crc_req_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_with_crc_req_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetch_block_with_crc_req_put_start(ctx, start) struct ternfs_fetch_block_with_crc_req_start* start = NULL

static inline void _ternfs_fetch_block_with_crc_req_put_file_id_unused(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetch_block_with_crc_req_start** prev, struct ternfs_fetch_block_with_crc_req_file_id_unused* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_fetch_block_with_crc_req_put_file_id_unused(ctx, prev, next, x) \
    struct ternfs_fetch_block_with_crc_req_file_id_unused next; \
    _ternfs_fetch_block_with_crc_req_put_file_id_unused(ctx, &(prev), &(next), x)

static inline void _ternfs_fetch_block_with_crc_req_put_block_id(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetch_block_with_crc_req_file_id_unused* prev, struct ternfs_fetch_block_with_crc_req_block_id* next, u64 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 8);
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += 8;
}
#define ternfs_fetch_block_with_crc_req_put_block_id(ctx, prev, next, x) \
    struct ternfs_fetch_block_with_crc_req_block_id next; \
    _ternfs_fetch_block_with_crc_req_put_block_id(ctx, &(prev), &(next), x)

static inline void _ternfs_fetch_block_with_crc_req_put_block_crc_unused(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetch_block_with_crc_req_block_id* prev, struct ternfs_fetch_block_with_crc_req_block_crc_unused* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetch_block_with_crc_req_put_block_crc_unused(ctx, prev, next, x) \
    struct ternfs_fetch_block_with_crc_req_block_crc_unused next; \
    _ternfs_fetch_block_with_crc_req_put_block_crc_unused(ctx, &(prev), &(next), x)

static inline void _ternfs_fetch_block_with_crc_req_put_offset(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetch_block_with_crc_req_block_crc_unused* prev, struct ternfs_fetch_block_with_crc_req_offset* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetch_block_with_crc_req_put_offset(ctx, prev, next, x) \
    struct ternfs_fetch_block_with_crc_req_offset next; \
    _ternfs_fetch_block_with_crc_req_put_offset(ctx, &(prev), &(next), x)

static inline void _ternfs_fetch_block_with_crc_req_put_count(struct ternfs_bincode_put_ctx* ctx, struct ternfs_fetch_block_with_crc_req_offset* prev, struct ternfs_fetch_block_with_crc_req_count* next, u32 x) {
    next = NULL;
    BUG_ON(ctx->end - ctx->cursor < 4);
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += 4;
}
#define ternfs_fetch_block_with_crc_req_put_count(ctx, prev, next, x) \
    struct ternfs_fetch_block_with_crc_req_count next; \
    _ternfs_fetch_block_with_crc_req_put_count(ctx, &(prev), &(next), x)

#define ternfs_fetch_block_with_crc_req_put_end(ctx, prev, next) \
    { struct ternfs_fetch_block_with_crc_req_count* __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetch_block_with_crc_req_end* next __attribute__((unused)) = NULL

#define TERNFS_FETCH_BLOCK_WITH_CRC_RESP_SIZE 0
struct ternfs_fetch_block_with_crc_resp_start;
#define ternfs_fetch_block_with_crc_resp_get_start(ctx, start) struct ternfs_fetch_block_with_crc_resp_start* start = NULL

struct ternfs_fetch_block_with_crc_resp_end;
#define ternfs_fetch_block_with_crc_resp_get_end(ctx, prev, next) \
    { struct ternfs_fetch_block_with_crc_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetch_block_with_crc_resp_end* next = NULL

static inline void ternfs_fetch_block_with_crc_resp_get_finish(struct ternfs_bincode_get_ctx* ctx, struct ternfs_fetch_block_with_crc_resp_end* end) {
    if (unlikely(ctx->buf != ctx->end)) {
        ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
    }
}

#define ternfs_fetch_block_with_crc_resp_put_start(ctx, start) struct ternfs_fetch_block_with_crc_resp_start* start = NULL

#define ternfs_fetch_block_with_crc_resp_put_end(ctx, prev, next) \
    { struct ternfs_fetch_block_with_crc_resp_start** __dummy __attribute__((unused)) = &(prev); }\
    struct ternfs_fetch_block_with_crc_resp_end* next __attribute__((unused)) = NULL

