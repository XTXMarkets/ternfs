// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "err.h"
#include "log.h"
#include "bincode.h"

bool ternfs_unexpected_error(int err) {
    switch (err) {
    case 0: return false;
    case TERNFS_ERR_INTERNAL_ERROR: return true;
    case TERNFS_ERR_FATAL_ERROR: return true;
    case TERNFS_ERR_TIMEOUT: return true;
    case TERNFS_ERR_MALFORMED_REQUEST: return true;
    case TERNFS_ERR_MALFORMED_RESPONSE: return true;
    case TERNFS_ERR_NOT_AUTHORISED: return true;
    case TERNFS_ERR_UNRECOGNIZED_REQUEST: return true;
    case TERNFS_ERR_FILE_NOT_FOUND: return false;
    case TERNFS_ERR_DIRECTORY_NOT_FOUND: return false;
    case TERNFS_ERR_NAME_NOT_FOUND: return false;
    case TERNFS_ERR_EDGE_NOT_FOUND: return false;
    case TERNFS_ERR_EDGE_IS_LOCKED: return true;
    case TERNFS_ERR_TYPE_IS_DIRECTORY: return true;
    case TERNFS_ERR_TYPE_IS_NOT_DIRECTORY: return true;
    case TERNFS_ERR_BAD_COOKIE: return true;
    case TERNFS_ERR_INCONSISTENT_STORAGE_CLASS_PARITY: return true;
    case TERNFS_ERR_LAST_SPAN_STATE_NOT_CLEAN: return true;
    case TERNFS_ERR_COULD_NOT_PICK_BLOCK_SERVICES: return true;
    case TERNFS_ERR_BAD_SPAN_BODY: return true;
    case TERNFS_ERR_SPAN_NOT_FOUND: return true;
    case TERNFS_ERR_BLOCK_SERVICE_NOT_FOUND: return true;
    case TERNFS_ERR_CANNOT_CERTIFY_BLOCKLESS_SPAN: return true;
    case TERNFS_ERR_BAD_NUMBER_OF_BLOCKS_PROOFS: return true;
    case TERNFS_ERR_BAD_BLOCK_PROOF: return true;
    case TERNFS_ERR_CANNOT_OVERRIDE_NAME: return true;
    case TERNFS_ERR_NAME_IS_LOCKED: return true;
    case TERNFS_ERR_MTIME_IS_TOO_RECENT: return true;
    case TERNFS_ERR_MISMATCHING_TARGET: return true;
    case TERNFS_ERR_MISMATCHING_OWNER: return true;
    case TERNFS_ERR_MISMATCHING_CREATION_TIME: return false;
    case TERNFS_ERR_DIRECTORY_NOT_EMPTY: return true;
    case TERNFS_ERR_FILE_IS_TRANSIENT: return true;
    case TERNFS_ERR_OLD_DIRECTORY_NOT_FOUND: return true;
    case TERNFS_ERR_NEW_DIRECTORY_NOT_FOUND: return true;
    case TERNFS_ERR_LOOP_IN_DIRECTORY_RENAME: return true;
    case TERNFS_ERR_DIRECTORY_HAS_OWNER: return true;
    case TERNFS_ERR_FILE_IS_NOT_TRANSIENT: return true;
    case TERNFS_ERR_FILE_NOT_EMPTY: return true;
    case TERNFS_ERR_CANNOT_REMOVE_ROOT_DIRECTORY: return true;
    case TERNFS_ERR_FILE_EMPTY: return true;
    case TERNFS_ERR_CANNOT_REMOVE_DIRTY_SPAN: return true;
    case TERNFS_ERR_BAD_SHARD: return true;
    case TERNFS_ERR_BAD_NAME: return true;
    case TERNFS_ERR_MORE_RECENT_SNAPSHOT_EDGE: return true;
    case TERNFS_ERR_MORE_RECENT_CURRENT_EDGE: return true;
    case TERNFS_ERR_BAD_DIRECTORY_INFO: return true;
    case TERNFS_ERR_DEADLINE_NOT_PASSED: return true;
    case TERNFS_ERR_SAME_SOURCE_AND_DESTINATION: return true;
    case TERNFS_ERR_SAME_DIRECTORIES: return true;
    case TERNFS_ERR_SAME_SHARD: return true;
    case TERNFS_ERR_BAD_PROTOCOL_VERSION: return true;
    case TERNFS_ERR_BAD_CERTIFICATE: return true;
    case TERNFS_ERR_BLOCK_TOO_RECENT_FOR_DELETION: return true;
    case TERNFS_ERR_BLOCK_FETCH_OUT_OF_BOUNDS: return true;
    case TERNFS_ERR_BAD_BLOCK_CRC: return true;
    case TERNFS_ERR_BLOCK_TOO_BIG: return true;
    case TERNFS_ERR_BLOCK_NOT_FOUND: return false;
    case TERNFS_ERR_BLOCK_IO_ERROR_FILE: return false;
    case TERNFS_ERR_BLOCK_IO_ERROR_DEVICE: return false;
    case -ERESTARTSYS: return false;
    case -ETIMEDOUT: return false;
    case -ECONNREFUSED: return false;
    case -ECONNABORTED: return false;
    case -ECONNRESET: return false;
    default: return true;
    }
}

// Safe to use with non-error `err`
int ternfs_error_to_linux(int err) {
    bool unexpected = ternfs_unexpected_error(err);
    if (unexpected) {
        ternfs_warn("unexpected ternsfs error %s (%d)", ternfs_err_str(err), err);
    }
    if (err <= 0) { return err; }
    switch (err) {
    case TERNFS_ERR_INTERNAL_ERROR: return -EIO;
    case TERNFS_ERR_FATAL_ERROR: return -EIO;
    case TERNFS_ERR_TIMEOUT: return -ETIMEDOUT;
    case TERNFS_ERR_NOT_AUTHORISED: return -EPERM;
    case TERNFS_ERR_UNRECOGNIZED_REQUEST: return -EIO;
    case TERNFS_ERR_FILE_NOT_FOUND: return -ENOENT;
    case TERNFS_ERR_DIRECTORY_NOT_FOUND: return -ENOENT;
    case TERNFS_ERR_NAME_NOT_FOUND: return -ENOENT;
    case TERNFS_ERR_EDGE_NOT_FOUND: return -ENOENT;
    case TERNFS_ERR_OLD_DIRECTORY_NOT_FOUND: return -ENOENT;
    case TERNFS_ERR_NEW_DIRECTORY_NOT_FOUND: return -ENOENT;
    case TERNFS_ERR_TYPE_IS_DIRECTORY: return -EISDIR;
    case TERNFS_ERR_TYPE_IS_NOT_DIRECTORY: return -ENOTDIR;
    case TERNFS_ERR_BAD_COOKIE: return -EBADCOOKIE;
    case TERNFS_ERR_INCONSISTENT_STORAGE_CLASS_PARITY: return -EIO;
    case TERNFS_ERR_LAST_SPAN_STATE_NOT_CLEAN: return -EIO;
    case TERNFS_ERR_COULD_NOT_PICK_BLOCK_SERVICES: return -EIO;
    case TERNFS_ERR_BAD_SPAN_BODY: return -EIO;
    case TERNFS_ERR_SPAN_NOT_FOUND: return -EIO;
    case TERNFS_ERR_BLOCK_SERVICE_NOT_FOUND: return -EIO;
    case TERNFS_ERR_CANNOT_CERTIFY_BLOCKLESS_SPAN: return -EIO;
    case TERNFS_ERR_BAD_NUMBER_OF_BLOCKS_PROOFS: return -EIO;
    case TERNFS_ERR_BAD_BLOCK_PROOF: return -EIO;
    case TERNFS_ERR_CANNOT_OVERRIDE_NAME: return -EEXIST;
    case TERNFS_ERR_NAME_IS_LOCKED: return -EIO;
    case TERNFS_ERR_MISMATCHING_TARGET: return -EIO;
    case TERNFS_ERR_MISMATCHING_OWNER: return -EIO;
    case TERNFS_ERR_DIRECTORY_NOT_EMPTY: return -ENOTEMPTY;
    case TERNFS_ERR_FILE_IS_TRANSIENT: return -EIO;
    case TERNFS_ERR_LOOP_IN_DIRECTORY_RENAME: return -ELOOP;
    case TERNFS_ERR_MALFORMED_REQUEST: return -EIO;
    case TERNFS_ERR_MALFORMED_RESPONSE: return -EIO;
    case TERNFS_ERR_BLOCK_IO_ERROR_FILE: return -EIO;
    case TERNFS_ERR_BLOCK_IO_ERROR_DEVICE: return -EIO;
    }
    return -EIO;
}
