// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <bit>
#include <cstddef>
#include <rocksdb/slice.h>

#include "Assert.hpp"
#include "Bincode.hpp"
#include "Exception.hpp"
#include "MsgsGen.hpp"
#include "RocksDBUtils.hpp"
#include "Time.hpp"
#include "CDCDB.hpp"

enum class CDCMetadataKey : uint8_t {
    LAST_APPLIED_LOG_ENTRY = 0,
    LAST_TXN = 1,
    FIRST_TXN_IN_QUEUE = 2,
    LAST_TXN_IN_QUEUE = 3,
    EXECUTING_TXN = 4,
    EXECUTING_TXN_STATE = 6,
    LAST_DIRECTORY_ID = 7,
    VERSION = 8,
};
constexpr CDCMetadataKey LAST_APPLIED_LOG_ENTRY_KEY = CDCMetadataKey::LAST_APPLIED_LOG_ENTRY; // V0, V1
constexpr CDCMetadataKey LAST_TXN_KEY = CDCMetadataKey::LAST_TXN; // V0, V1
constexpr CDCMetadataKey FIRST_TXN_IN_QUEUE_KEY = CDCMetadataKey::FIRST_TXN_IN_QUEUE; // V0
constexpr CDCMetadataKey LAST_TXN_IN_QUEUE_KEY = CDCMetadataKey::LAST_TXN_IN_QUEUE; // V0
constexpr CDCMetadataKey EXECUTING_TXN_KEY = CDCMetadataKey::EXECUTING_TXN; // V0
constexpr CDCMetadataKey EXECUTING_TXN_STATE_KEY = CDCMetadataKey::EXECUTING_TXN_STATE; // V0
constexpr CDCMetadataKey NEXT_DIRECTORY_ID_KEY = CDCMetadataKey::LAST_DIRECTORY_ID; // V0, V1
constexpr CDCMetadataKey VERSION_KEY = CDCMetadataKey::VERSION; // V1

inline rocksdb::Slice cdcMetadataKey(const CDCMetadataKey* k) {
    return rocksdb::Slice((const char*)k, sizeof(*k));
}

struct CDCTxnIdKey {
    FIELDS(
        BE, CDCTxnId, id, setIdUnchecked,
        END_STATIC
    )

    void setId(CDCTxnId i) {
        ALWAYS_ASSERT(i.x != 0);
        setIdUnchecked(i);
    }

    static StaticValue<CDCTxnIdKey> Static(CDCTxnId id) {
        auto x = StaticValue<CDCTxnIdKey>();
        x().setId(id);
        return x;
    }
};

struct CDCTxnIdValue {
    FIELDS(
        LE, CDCTxnId, id, setIdUnchecked,
        END_STATIC
    )

    void setId(CDCTxnId i) {
        ALWAYS_ASSERT(i.x != 0);
        setIdUnchecked(i);
    }

    static StaticValue<CDCTxnIdValue> Static(CDCTxnId id) {
        auto x = StaticValue<CDCTxnIdValue>();
        x().setId(id);
        return x;
    }
};

// This data structure, conceptually, is std::unordered_map<InodeId, std::vector<CDCTxnId>>.
//
// To encode this in RocksDB, we store (InodeId, CDCTxnId) keys with no values. Txn ids are
// increasing so the order will be naturally what we want.
//
// We also store a sentinel with the head of the list. This is to avoid having to step on many
// deleted keys when checking if a dir is already locked.
//
// The functions adding/removing elements to the list are tasked with bookeeping the sentinel.
struct DirsToTxnsKey {
    FIELDS(
        BE, InodeId,  dirId, setDirId,
        BE, uint64_t, maybeTxnId, setMaybeTxnId, // if 0, it's a sentinel
        END_STATIC
    )

    bool isSentinel() const {
        return maybeTxnId() == 0;
    }

    CDCTxnId txnId() const {
        ALWAYS_ASSERT(maybeTxnId() != 0);
        return maybeTxnId();
    }

    void setTxnId(CDCTxnId txnId) {
        ALWAYS_ASSERT(txnId.x != 0);
        setMaybeTxnId(txnId.x);
    }

    void setSentinel() {
        setMaybeTxnId(0);
    }
};

struct MakeDirectoryState {
    FIELDS(
        LE, InodeId,   dirId, setDirId,
        LE, TernTime,  oldCreationTime, setOldCreationTime,
        LE, TernTime,  creationTime, setCreationTime,
        LE, TernError, exitError, setExitError, // error if we're rolling back
        END_STATIC
    )

    void start() {
        setDirId(NULL_INODE_ID);
        setOldCreationTime({});
        setCreationTime({});
        setExitError(TernError::NO_ERROR);
    }
};

struct RenameFileState {
    FIELDS(
        LE, TernTime,  newOldCreationTime, setNewOldCreationTime,
        LE, TernTime,  newCreationTime, setNewCreationTime,
        LE, TernError, exitError, setExitError,
        END_STATIC
    )

    void start() {
        setNewOldCreationTime({});
        setNewCreationTime({});
        setExitError(TernError::NO_ERROR);
    }
};

struct SoftUnlinkDirectoryState {
    FIELDS(
        LE, InodeId,   statDirId, setStatDirId,
        LE, TernError, exitError, setExitError,
        END_STATIC
    )

    void start() {
        setExitError(TernError::NO_ERROR);
    }
};

struct RenameDirectoryState {
    FIELDS(
        LE, TernTime,  newOldCreationTime, setNewOldCreationTime,
        LE, TernTime,  newCreationTime, setNewCreationTime,
        LE, TernError, exitError, setExitError,
        END_STATIC
    )

    void start() {
        setNewOldCreationTime({});
        setNewCreationTime({});
        setExitError(TernError::NO_ERROR);
    }
};

struct HardUnlinkDirectoryState {
    FIELDS(END_STATIC)
    void start() {}
};

struct CrossShardHardUnlinkFileState {
    FIELDS(END_STATIC)
    void start() {}
};

template<typename Type, typename ...Types>
constexpr size_t maxMaxSize() {
    if constexpr (sizeof...(Types) > 0) {
        return std::max<size_t>(Type::MAX_SIZE, maxMaxSize<Types...>());
    } else {
        return Type::MAX_SIZE;
    }
}

struct TxnState {
    FIELDS(
        LE, CDCMessageKind, reqKind, setReqKind,
        LE, uint8_t,        step,    setStep,
        EMIT_OFFSET, MIN_SIZE,
        END
    )
    static constexpr size_t MAX_SIZE =
        MIN_SIZE +
        maxMaxSize<MakeDirectoryState, RenameFileState, SoftUnlinkDirectoryState, RenameDirectoryState, HardUnlinkDirectoryState, CrossShardHardUnlinkFileState>();

    size_t size() const {
        size_t sz = MIN_SIZE;
        switch (reqKind()) {
        case CDCMessageKind::MAKE_DIRECTORY:
            sz += MakeDirectoryState::MAX_SIZE; break;
        case CDCMessageKind::RENAME_FILE:
            sz += RenameFileState::MAX_SIZE; break;
        case CDCMessageKind::SOFT_UNLINK_DIRECTORY:
            sz += SoftUnlinkDirectoryState::MAX_SIZE; break;
        case CDCMessageKind::RENAME_DIRECTORY:
            sz += RenameDirectoryState::MAX_SIZE; break;
        case CDCMessageKind::HARD_UNLINK_DIRECTORY:
            sz += RenameDirectoryState::MAX_SIZE; break;
        case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE:
            sz += CrossShardHardUnlinkFileState::MAX_SIZE; break;
        default:
            throw TERN_EXCEPTION("bad cdc message kind %s", reqKind());
        }
        return sz;
    }

    void checkSize(size_t sz) {
        ALWAYS_ASSERT(sz >= MIN_SIZE);
        ALWAYS_ASSERT(sz == size());
    }

    #define TXN_STATE(kind, type, getName, startName) \
        type getName() { \
            ALWAYS_ASSERT(reqKind() == CDCMessageKind::kind); \
            type v; \
            v._data = _data + MIN_SIZE; \
            return v; \
        } \
        type startName() { \
            setStep(0); \
            setReqKind(CDCMessageKind::kind); \
            type v; \
            v._data = _data + MIN_SIZE; \
            v.start(); \
            return v; \
        }

    TXN_STATE(MAKE_DIRECTORY,               MakeDirectoryState,            getMakeDirectory,            startMakeDirectory)
    TXN_STATE(RENAME_FILE,                  RenameFileState,               getRenameFile,               startRenameFile)
    TXN_STATE(SOFT_UNLINK_DIRECTORY,        SoftUnlinkDirectoryState,      getSoftUnlinkDirectory,      startSoftUnlinkDirectory)
    TXN_STATE(RENAME_DIRECTORY,             RenameDirectoryState,          getRenameDirectory,          startRenameDirectory)
    TXN_STATE(HARD_UNLINK_DIRECTORY,        HardUnlinkDirectoryState,      getHardUnlinkDirectory,      startHardUnlinkDirectory)
    TXN_STATE(CROSS_SHARD_HARD_UNLINK_FILE, CrossShardHardUnlinkFileState, getCrossShardHardUnlinkFile, startCrossShardHardUnlinkFile)

    #undef TXN_STATE

    void start(CDCMessageKind kind) {
        setReqKind(kind);
        switch (reqKind()) {
        case CDCMessageKind::MAKE_DIRECTORY: startMakeDirectory(); break;
        case CDCMessageKind::RENAME_FILE: startRenameFile(); break;
        case CDCMessageKind::SOFT_UNLINK_DIRECTORY: startSoftUnlinkDirectory(); break;
        case CDCMessageKind::RENAME_DIRECTORY: startRenameDirectory(); break;
        case CDCMessageKind::HARD_UNLINK_DIRECTORY: startHardUnlinkDirectory(); break;
        case CDCMessageKind::CROSS_SHARD_HARD_UNLINK_FILE: startCrossShardHardUnlinkFile(); break;
        default:
            throw TERN_EXCEPTION("bad cdc message kind %s", reqKind());
        }
        memset(_data+MIN_SIZE, 0, size()-MIN_SIZE);
    }
};
