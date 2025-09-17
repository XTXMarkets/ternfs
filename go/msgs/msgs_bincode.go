// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// Automatically generated with go run bincodegen.
// Run `go generate ./...` from the go/ directory to regenerate it.
package msgs

import (
	"fmt"
	"io"
	"xtx/ternfs/core/bincode"
)

// This file specifies
type ShardRequest interface {
	bincode.Packable
	bincode.Unpackable
	ShardRequestKind() ShardMessageKind
}

type ShardResponse interface {
	bincode.Packable
	bincode.Unpackable
	ShardResponseKind() ShardMessageKind
}

type CDCRequest interface {
	bincode.Packable
	bincode.Unpackable
	CDCRequestKind() CDCMessageKind
}

type CDCResponse interface {
	bincode.Packable
	bincode.Unpackable
	CDCResponseKind() CDCMessageKind
}

type IsDirectoryInfoEntry interface {
	bincode.Packable
	bincode.Unpackable
	Tag() DirectoryInfoTag
}

func TagToDirInfoEntry(tag DirectoryInfoTag) IsDirectoryInfoEntry {
	switch tag {
	case SNAPSHOT_POLICY_TAG:
		return &SnapshotPolicy{}
	case SPAN_POLICY_TAG:
		return &SpanPolicy{}
	case BLOCK_POLICY_TAG:
		return &BlockPolicy{}
	case STRIPE_POLICY_TAG:
		return &StripePolicy{}
	default:
		panic(fmt.Errorf("unknown policy tag %v", tag))
	}
}

func (err TernError) Error() string {
	return err.String()
}

func (err *TernError) Pack(w io.Writer) error {
	return bincode.PackScalar(w, uint16(*err))
}

func (errCode *TernError) Unpack(r io.Reader) error {
	var c uint16
	if err := bincode.UnpackScalar(r, &c); err != nil {
		return err
	}
	*errCode = TernError(c)
	return nil
}

func (fs *FetchedSpan) Pack(w io.Writer) error {
	if fs.Header.StorageClass == EMPTY_STORAGE {
		return fmt.Errorf("cannot have EMPTY_STORAGE in fetched span")
	}
	if err := fs.Header.Pack(w); err != nil {
		return err
	}
	switch b := fs.Body.(type) {
	case *FetchedBlocksSpan:
		if fs.Header.StorageClass == INLINE_STORAGE {
			return fmt.Errorf("got INLINE storage class with blocks body")
		}
		if err := b.Pack(w); err != nil {
			return err
		}
	case *FetchedInlineSpan:
		if fs.Header.StorageClass != INLINE_STORAGE {
			return fmt.Errorf("got non-INLINE storage (%v) with inline body", fs.Header.StorageClass)
		}
		if err := b.Pack(w); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unexpected FetchedSpan body of type %T", b)
	}
	return nil
}

func (fs *FetchedSpan) Unpack(r io.Reader) error {
	if err := fs.Header.Unpack(r); err != nil {
		return err
	}
	if fs.Header.StorageClass == EMPTY_STORAGE {
		return fmt.Errorf("unexpected EMPTY_STORAGE in unpacked FetchedSpan")
	}
	if fs.Header.StorageClass == INLINE_STORAGE {
		fs.Body = &FetchedInlineSpan{}
	} else {
		fs.Body = &FetchedBlocksSpan{}
	}
	if err := fs.Body.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (fs *FetchedFullSpan) Pack(w io.Writer) error {
	if err := fs.Header.Pack(w); err != nil {
		return err
	}
	switch b := fs.Body.(type) {
	case *FetchedLocations:
		if fs.Header.IsInline {
			return fmt.Errorf("got INLINE storage class with blocks body")
		}
		if err := b.Pack(w); err != nil {
			return err
		}
	case *FetchedInlineSpan:
		if !fs.Header.IsInline {
			return fmt.Errorf("got non-INLINE storage (%v) with inline body", fs.Header)
		}
		if err := b.Pack(w); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unexpected FetchedFullSpan body of type %T", b)
	}
	return nil
}

func (fs *FetchedFullSpan) Unpack(r io.Reader) error {
	if err := fs.Header.Unpack(r); err != nil {
		return err
	}
	if fs.Header.IsInline {
		fs.Body = &FetchedInlineSpan{}
	} else {
		fs.Body = &FetchedLocations{}
	}
	if err := fs.Body.Unpack(r); err != nil {
		return err
	}
	return nil
}

const (
	INTERNAL_ERROR                          TernError = 10
	FATAL_ERROR                             TernError = 11
	TIMEOUT                                 TernError = 12
	MALFORMED_REQUEST                       TernError = 13
	MALFORMED_RESPONSE                      TernError = 14
	NOT_AUTHORISED                          TernError = 15
	UNRECOGNIZED_REQUEST                    TernError = 16
	FILE_NOT_FOUND                          TernError = 17
	DIRECTORY_NOT_FOUND                     TernError = 18
	NAME_NOT_FOUND                          TernError = 19
	EDGE_NOT_FOUND                          TernError = 20
	EDGE_IS_LOCKED                          TernError = 21
	TYPE_IS_DIRECTORY                       TernError = 22
	TYPE_IS_NOT_DIRECTORY                   TernError = 23
	BAD_COOKIE                              TernError = 24
	INCONSISTENT_STORAGE_CLASS_PARITY       TernError = 25
	LAST_SPAN_STATE_NOT_CLEAN               TernError = 26
	COULD_NOT_PICK_BLOCK_SERVICES           TernError = 27
	BAD_SPAN_BODY                           TernError = 28
	SPAN_NOT_FOUND                          TernError = 29
	BLOCK_SERVICE_NOT_FOUND                 TernError = 30
	CANNOT_CERTIFY_BLOCKLESS_SPAN           TernError = 31
	BAD_NUMBER_OF_BLOCKS_PROOFS             TernError = 32
	BAD_BLOCK_PROOF                         TernError = 33
	CANNOT_OVERRIDE_NAME                    TernError = 34
	NAME_IS_LOCKED                          TernError = 35
	MTIME_IS_TOO_RECENT                     TernError = 36
	MISMATCHING_TARGET                      TernError = 37
	MISMATCHING_OWNER                       TernError = 38
	MISMATCHING_CREATION_TIME               TernError = 39
	DIRECTORY_NOT_EMPTY                     TernError = 40
	FILE_IS_TRANSIENT                       TernError = 41
	OLD_DIRECTORY_NOT_FOUND                 TernError = 42
	NEW_DIRECTORY_NOT_FOUND                 TernError = 43
	LOOP_IN_DIRECTORY_RENAME                TernError = 44
	DIRECTORY_HAS_OWNER                     TernError = 45
	FILE_IS_NOT_TRANSIENT                   TernError = 46
	FILE_NOT_EMPTY                          TernError = 47
	CANNOT_REMOVE_ROOT_DIRECTORY            TernError = 48
	FILE_EMPTY                              TernError = 49
	CANNOT_REMOVE_DIRTY_SPAN                TernError = 50
	BAD_SHARD                               TernError = 51
	BAD_NAME                                TernError = 52
	MORE_RECENT_SNAPSHOT_EDGE               TernError = 53
	MORE_RECENT_CURRENT_EDGE                TernError = 54
	BAD_DIRECTORY_INFO                      TernError = 55
	DEADLINE_NOT_PASSED                     TernError = 56
	SAME_SOURCE_AND_DESTINATION             TernError = 57
	SAME_DIRECTORIES                        TernError = 58
	SAME_SHARD                              TernError = 59
	BAD_PROTOCOL_VERSION                    TernError = 60
	BAD_CERTIFICATE                         TernError = 61
	BLOCK_TOO_RECENT_FOR_DELETION           TernError = 62
	BLOCK_FETCH_OUT_OF_BOUNDS               TernError = 63
	BAD_BLOCK_CRC                           TernError = 64
	BLOCK_TOO_BIG                           TernError = 65
	BLOCK_NOT_FOUND                         TernError = 66
	CANNOT_UNSET_DECOMMISSIONED             TernError = 67
	CANNOT_REGISTER_DECOMMISSIONED_OR_STALE TernError = 68
	BLOCK_TOO_OLD_FOR_WRITE                 TernError = 69
	BLOCK_IO_ERROR_DEVICE                   TernError = 70
	BLOCK_IO_ERROR_FILE                     TernError = 71
	INVALID_REPLICA                         TernError = 72
	DIFFERENT_ADDRS_INFO                    TernError = 73
	LEADER_PREEMPTED                        TernError = 74
	LOG_ENTRY_MISSING                       TernError = 75
	LOG_ENTRY_TRIMMED                       TernError = 76
	LOG_ENTRY_UNRELEASED                    TernError = 77
	LOG_ENTRY_RELEASED                      TernError = 78
	AUTO_DECOMMISSION_FORBIDDEN             TernError = 79
	INCONSISTENT_BLOCK_SERVICE_REGISTRATION TernError = 80
	SWAP_BLOCKS_INLINE_STORAGE              TernError = 81
	SWAP_BLOCKS_MISMATCHING_SIZE            TernError = 82
	SWAP_BLOCKS_MISMATCHING_STATE           TernError = 83
	SWAP_BLOCKS_MISMATCHING_CRC             TernError = 84
	SWAP_BLOCKS_DUPLICATE_BLOCK_SERVICE     TernError = 85
	SWAP_SPANS_INLINE_STORAGE               TernError = 86
	SWAP_SPANS_MISMATCHING_SIZE             TernError = 87
	SWAP_SPANS_NOT_CLEAN                    TernError = 88
	SWAP_SPANS_MISMATCHING_CRC              TernError = 89
	SWAP_SPANS_MISMATCHING_BLOCKS           TernError = 90
	EDGE_NOT_OWNED                          TernError = 91
	CANNOT_CREATE_DB_SNAPSHOT               TernError = 92
	BLOCK_SIZE_NOT_MULTIPLE_OF_PAGE_SIZE    TernError = 93
	SWAP_BLOCKS_DUPLICATE_FAILURE_DOMAIN    TernError = 94
	TRANSIENT_LOCATION_COUNT                TernError = 95
	ADD_SPAN_LOCATION_INLINE_STORAGE        TernError = 96
	ADD_SPAN_LOCATION_MISMATCHING_SIZE      TernError = 97
	ADD_SPAN_LOCATION_NOT_CLEAN             TernError = 98
	ADD_SPAN_LOCATION_MISMATCHING_CRC       TernError = 99
	ADD_SPAN_LOCATION_EXISTS                TernError = 100
	SWAP_BLOCKS_MISMATCHING_LOCATION        TernError = 101
	LOCATION_EXISTS                         TernError = 102
	LOCATION_NOT_FOUND                      TernError = 103
)

func (err TernError) String() string {
	switch err {
	case 10:
		return "INTERNAL_ERROR"
	case 11:
		return "FATAL_ERROR"
	case 12:
		return "TIMEOUT"
	case 13:
		return "MALFORMED_REQUEST"
	case 14:
		return "MALFORMED_RESPONSE"
	case 15:
		return "NOT_AUTHORISED"
	case 16:
		return "UNRECOGNIZED_REQUEST"
	case 17:
		return "FILE_NOT_FOUND"
	case 18:
		return "DIRECTORY_NOT_FOUND"
	case 19:
		return "NAME_NOT_FOUND"
	case 20:
		return "EDGE_NOT_FOUND"
	case 21:
		return "EDGE_IS_LOCKED"
	case 22:
		return "TYPE_IS_DIRECTORY"
	case 23:
		return "TYPE_IS_NOT_DIRECTORY"
	case 24:
		return "BAD_COOKIE"
	case 25:
		return "INCONSISTENT_STORAGE_CLASS_PARITY"
	case 26:
		return "LAST_SPAN_STATE_NOT_CLEAN"
	case 27:
		return "COULD_NOT_PICK_BLOCK_SERVICES"
	case 28:
		return "BAD_SPAN_BODY"
	case 29:
		return "SPAN_NOT_FOUND"
	case 30:
		return "BLOCK_SERVICE_NOT_FOUND"
	case 31:
		return "CANNOT_CERTIFY_BLOCKLESS_SPAN"
	case 32:
		return "BAD_NUMBER_OF_BLOCKS_PROOFS"
	case 33:
		return "BAD_BLOCK_PROOF"
	case 34:
		return "CANNOT_OVERRIDE_NAME"
	case 35:
		return "NAME_IS_LOCKED"
	case 36:
		return "MTIME_IS_TOO_RECENT"
	case 37:
		return "MISMATCHING_TARGET"
	case 38:
		return "MISMATCHING_OWNER"
	case 39:
		return "MISMATCHING_CREATION_TIME"
	case 40:
		return "DIRECTORY_NOT_EMPTY"
	case 41:
		return "FILE_IS_TRANSIENT"
	case 42:
		return "OLD_DIRECTORY_NOT_FOUND"
	case 43:
		return "NEW_DIRECTORY_NOT_FOUND"
	case 44:
		return "LOOP_IN_DIRECTORY_RENAME"
	case 45:
		return "DIRECTORY_HAS_OWNER"
	case 46:
		return "FILE_IS_NOT_TRANSIENT"
	case 47:
		return "FILE_NOT_EMPTY"
	case 48:
		return "CANNOT_REMOVE_ROOT_DIRECTORY"
	case 49:
		return "FILE_EMPTY"
	case 50:
		return "CANNOT_REMOVE_DIRTY_SPAN"
	case 51:
		return "BAD_SHARD"
	case 52:
		return "BAD_NAME"
	case 53:
		return "MORE_RECENT_SNAPSHOT_EDGE"
	case 54:
		return "MORE_RECENT_CURRENT_EDGE"
	case 55:
		return "BAD_DIRECTORY_INFO"
	case 56:
		return "DEADLINE_NOT_PASSED"
	case 57:
		return "SAME_SOURCE_AND_DESTINATION"
	case 58:
		return "SAME_DIRECTORIES"
	case 59:
		return "SAME_SHARD"
	case 60:
		return "BAD_PROTOCOL_VERSION"
	case 61:
		return "BAD_CERTIFICATE"
	case 62:
		return "BLOCK_TOO_RECENT_FOR_DELETION"
	case 63:
		return "BLOCK_FETCH_OUT_OF_BOUNDS"
	case 64:
		return "BAD_BLOCK_CRC"
	case 65:
		return "BLOCK_TOO_BIG"
	case 66:
		return "BLOCK_NOT_FOUND"
	case 67:
		return "CANNOT_UNSET_DECOMMISSIONED"
	case 68:
		return "CANNOT_REGISTER_DECOMMISSIONED_OR_STALE"
	case 69:
		return "BLOCK_TOO_OLD_FOR_WRITE"
	case 70:
		return "BLOCK_IO_ERROR_DEVICE"
	case 71:
		return "BLOCK_IO_ERROR_FILE"
	case 72:
		return "INVALID_REPLICA"
	case 73:
		return "DIFFERENT_ADDRS_INFO"
	case 74:
		return "LEADER_PREEMPTED"
	case 75:
		return "LOG_ENTRY_MISSING"
	case 76:
		return "LOG_ENTRY_TRIMMED"
	case 77:
		return "LOG_ENTRY_UNRELEASED"
	case 78:
		return "LOG_ENTRY_RELEASED"
	case 79:
		return "AUTO_DECOMMISSION_FORBIDDEN"
	case 80:
		return "INCONSISTENT_BLOCK_SERVICE_REGISTRATION"
	case 81:
		return "SWAP_BLOCKS_INLINE_STORAGE"
	case 82:
		return "SWAP_BLOCKS_MISMATCHING_SIZE"
	case 83:
		return "SWAP_BLOCKS_MISMATCHING_STATE"
	case 84:
		return "SWAP_BLOCKS_MISMATCHING_CRC"
	case 85:
		return "SWAP_BLOCKS_DUPLICATE_BLOCK_SERVICE"
	case 86:
		return "SWAP_SPANS_INLINE_STORAGE"
	case 87:
		return "SWAP_SPANS_MISMATCHING_SIZE"
	case 88:
		return "SWAP_SPANS_NOT_CLEAN"
	case 89:
		return "SWAP_SPANS_MISMATCHING_CRC"
	case 90:
		return "SWAP_SPANS_MISMATCHING_BLOCKS"
	case 91:
		return "EDGE_NOT_OWNED"
	case 92:
		return "CANNOT_CREATE_DB_SNAPSHOT"
	case 93:
		return "BLOCK_SIZE_NOT_MULTIPLE_OF_PAGE_SIZE"
	case 94:
		return "SWAP_BLOCKS_DUPLICATE_FAILURE_DOMAIN"
	case 95:
		return "TRANSIENT_LOCATION_COUNT"
	case 96:
		return "ADD_SPAN_LOCATION_INLINE_STORAGE"
	case 97:
		return "ADD_SPAN_LOCATION_MISMATCHING_SIZE"
	case 98:
		return "ADD_SPAN_LOCATION_NOT_CLEAN"
	case 99:
		return "ADD_SPAN_LOCATION_MISMATCHING_CRC"
	case 100:
		return "ADD_SPAN_LOCATION_EXISTS"
	case 101:
		return "SWAP_BLOCKS_MISMATCHING_LOCATION"
	case 102:
		return "LOCATION_EXISTS"
	case 103:
		return "LOCATION_NOT_FOUND"
	default:
		return fmt.Sprintf("TernError(%d)", err)
	}
}

func (k ShardMessageKind) String() string {
	switch k {
	case 1:
		return "LOOKUP"
	case 2:
		return "STAT_FILE"
	case 4:
		return "STAT_DIRECTORY"
	case 5:
		return "READ_DIR"
	case 6:
		return "CONSTRUCT_FILE"
	case 7:
		return "ADD_SPAN_INITIATE"
	case 8:
		return "ADD_SPAN_CERTIFY"
	case 9:
		return "LINK_FILE"
	case 10:
		return "SOFT_UNLINK_FILE"
	case 11:
		return "LOCAL_FILE_SPANS"
	case 12:
		return "SAME_DIRECTORY_RENAME"
	case 16:
		return "ADD_INLINE_SPAN"
	case 17:
		return "SET_TIME"
	case 115:
		return "FULL_READ_DIR"
	case 123:
		return "MOVE_SPAN"
	case 116:
		return "REMOVE_NON_OWNED_EDGE"
	case 117:
		return "SAME_SHARD_HARD_FILE_UNLINK"
	case 3:
		return "STAT_TRANSIENT_FILE"
	case 18:
		return "SHARD_SNAPSHOT"
	case 20:
		return "FILE_SPANS"
	case 21:
		return "ADD_SPAN_LOCATION"
	case 22:
		return "SCRAP_TRANSIENT_FILE"
	case 13:
		return "SET_DIRECTORY_INFO"
	case 112:
		return "VISIT_DIRECTORIES"
	case 113:
		return "VISIT_FILES"
	case 114:
		return "VISIT_TRANSIENT_FILES"
	case 118:
		return "REMOVE_SPAN_INITIATE"
	case 119:
		return "REMOVE_SPAN_CERTIFY"
	case 120:
		return "SWAP_BLOCKS"
	case 121:
		return "BLOCK_SERVICE_FILES"
	case 122:
		return "REMOVE_INODE"
	case 124:
		return "ADD_SPAN_INITIATE_WITH_REFERENCE"
	case 125:
		return "REMOVE_ZERO_BLOCK_SERVICE_FILES"
	case 126:
		return "SWAP_SPANS"
	case 127:
		return "SAME_DIRECTORY_RENAME_SNAPSHOT"
	case 19:
		return "ADD_SPAN_AT_LOCATION_INITIATE"
	case 128:
		return "CREATE_DIRECTORY_INODE"
	case 129:
		return "SET_DIRECTORY_OWNER"
	case 137:
		return "REMOVE_DIRECTORY_OWNER"
	case 130:
		return "CREATE_LOCKED_CURRENT_EDGE"
	case 131:
		return "LOCK_CURRENT_EDGE"
	case 132:
		return "UNLOCK_CURRENT_EDGE"
	case 134:
		return "REMOVE_OWNED_SNAPSHOT_FILE_EDGE"
	case 135:
		return "MAKE_FILE_TRANSIENT"
	default:
		return fmt.Sprintf("ShardMessageKind(%d)", k)
	}
}

const (
	LOOKUP                           ShardMessageKind = 0x1
	STAT_FILE                        ShardMessageKind = 0x2
	STAT_DIRECTORY                   ShardMessageKind = 0x4
	READ_DIR                         ShardMessageKind = 0x5
	CONSTRUCT_FILE                   ShardMessageKind = 0x6
	ADD_SPAN_INITIATE                ShardMessageKind = 0x7
	ADD_SPAN_CERTIFY                 ShardMessageKind = 0x8
	LINK_FILE                        ShardMessageKind = 0x9
	SOFT_UNLINK_FILE                 ShardMessageKind = 0xA
	LOCAL_FILE_SPANS                 ShardMessageKind = 0xB
	SAME_DIRECTORY_RENAME            ShardMessageKind = 0xC
	ADD_INLINE_SPAN                  ShardMessageKind = 0x10
	SET_TIME                         ShardMessageKind = 0x11
	FULL_READ_DIR                    ShardMessageKind = 0x73
	MOVE_SPAN                        ShardMessageKind = 0x7B
	REMOVE_NON_OWNED_EDGE            ShardMessageKind = 0x74
	SAME_SHARD_HARD_FILE_UNLINK      ShardMessageKind = 0x75
	STAT_TRANSIENT_FILE              ShardMessageKind = 0x3
	SHARD_SNAPSHOT                   ShardMessageKind = 0x12
	FILE_SPANS                       ShardMessageKind = 0x14
	ADD_SPAN_LOCATION                ShardMessageKind = 0x15
	SCRAP_TRANSIENT_FILE             ShardMessageKind = 0x16
	SET_DIRECTORY_INFO               ShardMessageKind = 0xD
	VISIT_DIRECTORIES                ShardMessageKind = 0x70
	VISIT_FILES                      ShardMessageKind = 0x71
	VISIT_TRANSIENT_FILES            ShardMessageKind = 0x72
	REMOVE_SPAN_INITIATE             ShardMessageKind = 0x76
	REMOVE_SPAN_CERTIFY              ShardMessageKind = 0x77
	SWAP_BLOCKS                      ShardMessageKind = 0x78
	BLOCK_SERVICE_FILES              ShardMessageKind = 0x79
	REMOVE_INODE                     ShardMessageKind = 0x7A
	ADD_SPAN_INITIATE_WITH_REFERENCE ShardMessageKind = 0x7C
	REMOVE_ZERO_BLOCK_SERVICE_FILES  ShardMessageKind = 0x7D
	SWAP_SPANS                       ShardMessageKind = 0x7E
	SAME_DIRECTORY_RENAME_SNAPSHOT   ShardMessageKind = 0x7F
	ADD_SPAN_AT_LOCATION_INITIATE    ShardMessageKind = 0x13
	CREATE_DIRECTORY_INODE           ShardMessageKind = 0x80
	SET_DIRECTORY_OWNER              ShardMessageKind = 0x81
	REMOVE_DIRECTORY_OWNER           ShardMessageKind = 0x89
	CREATE_LOCKED_CURRENT_EDGE       ShardMessageKind = 0x82
	LOCK_CURRENT_EDGE                ShardMessageKind = 0x83
	UNLOCK_CURRENT_EDGE              ShardMessageKind = 0x84
	REMOVE_OWNED_SNAPSHOT_FILE_EDGE  ShardMessageKind = 0x86
	MAKE_FILE_TRANSIENT              ShardMessageKind = 0x87
)

var AllShardMessageKind = [...]ShardMessageKind{
	LOOKUP,
	STAT_FILE,
	STAT_DIRECTORY,
	READ_DIR,
	CONSTRUCT_FILE,
	ADD_SPAN_INITIATE,
	ADD_SPAN_CERTIFY,
	LINK_FILE,
	SOFT_UNLINK_FILE,
	LOCAL_FILE_SPANS,
	SAME_DIRECTORY_RENAME,
	ADD_INLINE_SPAN,
	SET_TIME,
	FULL_READ_DIR,
	MOVE_SPAN,
	REMOVE_NON_OWNED_EDGE,
	SAME_SHARD_HARD_FILE_UNLINK,
	STAT_TRANSIENT_FILE,
	SHARD_SNAPSHOT,
	FILE_SPANS,
	ADD_SPAN_LOCATION,
	SCRAP_TRANSIENT_FILE,
	SET_DIRECTORY_INFO,
	VISIT_DIRECTORIES,
	VISIT_FILES,
	VISIT_TRANSIENT_FILES,
	REMOVE_SPAN_INITIATE,
	REMOVE_SPAN_CERTIFY,
	SWAP_BLOCKS,
	BLOCK_SERVICE_FILES,
	REMOVE_INODE,
	ADD_SPAN_INITIATE_WITH_REFERENCE,
	REMOVE_ZERO_BLOCK_SERVICE_FILES,
	SWAP_SPANS,
	SAME_DIRECTORY_RENAME_SNAPSHOT,
	ADD_SPAN_AT_LOCATION_INITIATE,
	CREATE_DIRECTORY_INODE,
	SET_DIRECTORY_OWNER,
	REMOVE_DIRECTORY_OWNER,
	CREATE_LOCKED_CURRENT_EDGE,
	LOCK_CURRENT_EDGE,
	UNLOCK_CURRENT_EDGE,
	REMOVE_OWNED_SNAPSHOT_FILE_EDGE,
	MAKE_FILE_TRANSIENT,
}

const MaxShardMessageKind ShardMessageKind = 137

func MkShardMessage(k string) (ShardRequest, ShardResponse, error) {
	switch {
	case k == "LOOKUP":
		return &LookupReq{}, &LookupResp{}, nil
	case k == "STAT_FILE":
		return &StatFileReq{}, &StatFileResp{}, nil
	case k == "STAT_DIRECTORY":
		return &StatDirectoryReq{}, &StatDirectoryResp{}, nil
	case k == "READ_DIR":
		return &ReadDirReq{}, &ReadDirResp{}, nil
	case k == "CONSTRUCT_FILE":
		return &ConstructFileReq{}, &ConstructFileResp{}, nil
	case k == "ADD_SPAN_INITIATE":
		return &AddSpanInitiateReq{}, &AddSpanInitiateResp{}, nil
	case k == "ADD_SPAN_CERTIFY":
		return &AddSpanCertifyReq{}, &AddSpanCertifyResp{}, nil
	case k == "LINK_FILE":
		return &LinkFileReq{}, &LinkFileResp{}, nil
	case k == "SOFT_UNLINK_FILE":
		return &SoftUnlinkFileReq{}, &SoftUnlinkFileResp{}, nil
	case k == "LOCAL_FILE_SPANS":
		return &LocalFileSpansReq{}, &LocalFileSpansResp{}, nil
	case k == "SAME_DIRECTORY_RENAME":
		return &SameDirectoryRenameReq{}, &SameDirectoryRenameResp{}, nil
	case k == "ADD_INLINE_SPAN":
		return &AddInlineSpanReq{}, &AddInlineSpanResp{}, nil
	case k == "SET_TIME":
		return &SetTimeReq{}, &SetTimeResp{}, nil
	case k == "FULL_READ_DIR":
		return &FullReadDirReq{}, &FullReadDirResp{}, nil
	case k == "MOVE_SPAN":
		return &MoveSpanReq{}, &MoveSpanResp{}, nil
	case k == "REMOVE_NON_OWNED_EDGE":
		return &RemoveNonOwnedEdgeReq{}, &RemoveNonOwnedEdgeResp{}, nil
	case k == "SAME_SHARD_HARD_FILE_UNLINK":
		return &SameShardHardFileUnlinkReq{}, &SameShardHardFileUnlinkResp{}, nil
	case k == "STAT_TRANSIENT_FILE":
		return &StatTransientFileReq{}, &StatTransientFileResp{}, nil
	case k == "SHARD_SNAPSHOT":
		return &ShardSnapshotReq{}, &ShardSnapshotResp{}, nil
	case k == "FILE_SPANS":
		return &FileSpansReq{}, &FileSpansResp{}, nil
	case k == "ADD_SPAN_LOCATION":
		return &AddSpanLocationReq{}, &AddSpanLocationResp{}, nil
	case k == "SCRAP_TRANSIENT_FILE":
		return &ScrapTransientFileReq{}, &ScrapTransientFileResp{}, nil
	case k == "SET_DIRECTORY_INFO":
		return &SetDirectoryInfoReq{}, &SetDirectoryInfoResp{}, nil
	case k == "VISIT_DIRECTORIES":
		return &VisitDirectoriesReq{}, &VisitDirectoriesResp{}, nil
	case k == "VISIT_FILES":
		return &VisitFilesReq{}, &VisitFilesResp{}, nil
	case k == "VISIT_TRANSIENT_FILES":
		return &VisitTransientFilesReq{}, &VisitTransientFilesResp{}, nil
	case k == "REMOVE_SPAN_INITIATE":
		return &RemoveSpanInitiateReq{}, &RemoveSpanInitiateResp{}, nil
	case k == "REMOVE_SPAN_CERTIFY":
		return &RemoveSpanCertifyReq{}, &RemoveSpanCertifyResp{}, nil
	case k == "SWAP_BLOCKS":
		return &SwapBlocksReq{}, &SwapBlocksResp{}, nil
	case k == "BLOCK_SERVICE_FILES":
		return &BlockServiceFilesReq{}, &BlockServiceFilesResp{}, nil
	case k == "REMOVE_INODE":
		return &RemoveInodeReq{}, &RemoveInodeResp{}, nil
	case k == "ADD_SPAN_INITIATE_WITH_REFERENCE":
		return &AddSpanInitiateWithReferenceReq{}, &AddSpanInitiateWithReferenceResp{}, nil
	case k == "REMOVE_ZERO_BLOCK_SERVICE_FILES":
		return &RemoveZeroBlockServiceFilesReq{}, &RemoveZeroBlockServiceFilesResp{}, nil
	case k == "SWAP_SPANS":
		return &SwapSpansReq{}, &SwapSpansResp{}, nil
	case k == "SAME_DIRECTORY_RENAME_SNAPSHOT":
		return &SameDirectoryRenameSnapshotReq{}, &SameDirectoryRenameSnapshotResp{}, nil
	case k == "ADD_SPAN_AT_LOCATION_INITIATE":
		return &AddSpanAtLocationInitiateReq{}, &AddSpanAtLocationInitiateResp{}, nil
	case k == "CREATE_DIRECTORY_INODE":
		return &CreateDirectoryInodeReq{}, &CreateDirectoryInodeResp{}, nil
	case k == "SET_DIRECTORY_OWNER":
		return &SetDirectoryOwnerReq{}, &SetDirectoryOwnerResp{}, nil
	case k == "REMOVE_DIRECTORY_OWNER":
		return &RemoveDirectoryOwnerReq{}, &RemoveDirectoryOwnerResp{}, nil
	case k == "CREATE_LOCKED_CURRENT_EDGE":
		return &CreateLockedCurrentEdgeReq{}, &CreateLockedCurrentEdgeResp{}, nil
	case k == "LOCK_CURRENT_EDGE":
		return &LockCurrentEdgeReq{}, &LockCurrentEdgeResp{}, nil
	case k == "UNLOCK_CURRENT_EDGE":
		return &UnlockCurrentEdgeReq{}, &UnlockCurrentEdgeResp{}, nil
	case k == "REMOVE_OWNED_SNAPSHOT_FILE_EDGE":
		return &RemoveOwnedSnapshotFileEdgeReq{}, &RemoveOwnedSnapshotFileEdgeResp{}, nil
	case k == "MAKE_FILE_TRANSIENT":
		return &MakeFileTransientReq{}, &MakeFileTransientResp{}, nil
	default:
		return nil, nil, fmt.Errorf("bad kind string %s", k)
	}
}

func (k CDCMessageKind) String() string {
	switch k {
	case 1:
		return "MAKE_DIRECTORY"
	case 2:
		return "RENAME_FILE"
	case 3:
		return "SOFT_UNLINK_DIRECTORY"
	case 4:
		return "RENAME_DIRECTORY"
	case 5:
		return "HARD_UNLINK_DIRECTORY"
	case 6:
		return "CROSS_SHARD_HARD_UNLINK_FILE"
	case 7:
		return "CDC_SNAPSHOT"
	default:
		return fmt.Sprintf("CDCMessageKind(%d)", k)
	}
}

const (
	MAKE_DIRECTORY               CDCMessageKind = 0x1
	RENAME_FILE                  CDCMessageKind = 0x2
	SOFT_UNLINK_DIRECTORY        CDCMessageKind = 0x3
	RENAME_DIRECTORY             CDCMessageKind = 0x4
	HARD_UNLINK_DIRECTORY        CDCMessageKind = 0x5
	CROSS_SHARD_HARD_UNLINK_FILE CDCMessageKind = 0x6
	CDC_SNAPSHOT                 CDCMessageKind = 0x7
)

var AllCDCMessageKind = [...]CDCMessageKind{
	MAKE_DIRECTORY,
	RENAME_FILE,
	SOFT_UNLINK_DIRECTORY,
	RENAME_DIRECTORY,
	HARD_UNLINK_DIRECTORY,
	CROSS_SHARD_HARD_UNLINK_FILE,
	CDC_SNAPSHOT,
}

const MaxCDCMessageKind CDCMessageKind = 7

func MkCDCMessage(k string) (CDCRequest, CDCResponse, error) {
	switch {
	case k == "MAKE_DIRECTORY":
		return &MakeDirectoryReq{}, &MakeDirectoryResp{}, nil
	case k == "RENAME_FILE":
		return &RenameFileReq{}, &RenameFileResp{}, nil
	case k == "SOFT_UNLINK_DIRECTORY":
		return &SoftUnlinkDirectoryReq{}, &SoftUnlinkDirectoryResp{}, nil
	case k == "RENAME_DIRECTORY":
		return &RenameDirectoryReq{}, &RenameDirectoryResp{}, nil
	case k == "HARD_UNLINK_DIRECTORY":
		return &HardUnlinkDirectoryReq{}, &HardUnlinkDirectoryResp{}, nil
	case k == "CROSS_SHARD_HARD_UNLINK_FILE":
		return &CrossShardHardUnlinkFileReq{}, &CrossShardHardUnlinkFileResp{}, nil
	case k == "CDC_SNAPSHOT":
		return &CdcSnapshotReq{}, &CdcSnapshotResp{}, nil
	default:
		return nil, nil, fmt.Errorf("bad kind string %s", k)
	}
}

func (k RegistryMessageKind) String() string {
	switch k {
	case 3:
		return "LOCAL_SHARDS"
	case 7:
		return "LOCAL_CDC"
	case 8:
		return "INFO"
	case 15:
		return "REGISTRY"
	case 34:
		return "LOCAL_CHANGED_BLOCK_SERVICES"
	case 1:
		return "CREATE_LOCATION"
	case 2:
		return "RENAME_LOCATION"
	case 4:
		return "REGISTER_SHARD"
	case 5:
		return "LOCATIONS"
	case 6:
		return "REGISTER_CDC"
	case 9:
		return "SET_BLOCK_SERVICE_FLAGS"
	case 10:
		return "REGISTER_BLOCK_SERVICES"
	case 11:
		return "CHANGED_BLOCK_SERVICES_AT_LOCATION"
	case 12:
		return "SHARDS_AT_LOCATION"
	case 13:
		return "CDC_AT_LOCATION"
	case 14:
		return "REGISTER_REGISTRY"
	case 16:
		return "ALL_REGISTRY_REPLICAS"
	case 17:
		return "SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED"
	case 19:
		return "CDC_REPLICAS_DE_PR_EC_AT_ED"
	case 20:
		return "ALL_SHARDS"
	case 21:
		return "DECOMMISSION_BLOCK_SERVICE"
	case 22:
		return "MOVE_SHARD_LEADER"
	case 23:
		return "CLEAR_SHARD_INFO"
	case 24:
		return "SHARD_BLOCK_SERVICES"
	case 25:
		return "ALL_CDC"
	case 32:
		return "ERASE_DECOMMISSIONED_BLOCK"
	case 33:
		return "ALL_BLOCK_SERVICES_DEPRECATED"
	case 35:
		return "MOVE_CDC_LEADER"
	case 36:
		return "CLEAR_CDC_INFO"
	case 37:
		return "UPDATE_BLOCK_SERVICE_PATH"
	default:
		return fmt.Sprintf("RegistryMessageKind(%d)", k)
	}
}

const (
	LOCAL_SHARDS                        RegistryMessageKind = 0x3
	LOCAL_CDC                           RegistryMessageKind = 0x7
	INFO                                RegistryMessageKind = 0x8
	REGISTRY                            RegistryMessageKind = 0xF
	LOCAL_CHANGED_BLOCK_SERVICES        RegistryMessageKind = 0x22
	CREATE_LOCATION                     RegistryMessageKind = 0x1
	RENAME_LOCATION                     RegistryMessageKind = 0x2
	REGISTER_SHARD                      RegistryMessageKind = 0x4
	LOCATIONS                           RegistryMessageKind = 0x5
	REGISTER_CDC                        RegistryMessageKind = 0x6
	SET_BLOCK_SERVICE_FLAGS             RegistryMessageKind = 0x9
	REGISTER_BLOCK_SERVICES             RegistryMessageKind = 0xA
	CHANGED_BLOCK_SERVICES_AT_LOCATION  RegistryMessageKind = 0xB
	SHARDS_AT_LOCATION                  RegistryMessageKind = 0xC
	CDC_AT_LOCATION                     RegistryMessageKind = 0xD
	REGISTER_REGISTRY                   RegistryMessageKind = 0xE
	ALL_REGISTRY_REPLICAS               RegistryMessageKind = 0x10
	SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED RegistryMessageKind = 0x11
	CDC_REPLICAS_DE_PR_EC_AT_ED         RegistryMessageKind = 0x13
	ALL_SHARDS                          RegistryMessageKind = 0x14
	DECOMMISSION_BLOCK_SERVICE          RegistryMessageKind = 0x15
	MOVE_SHARD_LEADER                   RegistryMessageKind = 0x16
	CLEAR_SHARD_INFO                    RegistryMessageKind = 0x17
	SHARD_BLOCK_SERVICES                RegistryMessageKind = 0x18
	ALL_CDC                             RegistryMessageKind = 0x19
	ERASE_DECOMMISSIONED_BLOCK          RegistryMessageKind = 0x20
	ALL_BLOCK_SERVICES_DEPRECATED       RegistryMessageKind = 0x21
	MOVE_CDC_LEADER                     RegistryMessageKind = 0x23
	CLEAR_CDC_INFO                      RegistryMessageKind = 0x24
	UPDATE_BLOCK_SERVICE_PATH           RegistryMessageKind = 0x25
)

var AllRegistryMessageKind = [...]RegistryMessageKind{
	LOCAL_SHARDS,
	LOCAL_CDC,
	INFO,
	REGISTRY,
	LOCAL_CHANGED_BLOCK_SERVICES,
	CREATE_LOCATION,
	RENAME_LOCATION,
	REGISTER_SHARD,
	LOCATIONS,
	REGISTER_CDC,
	SET_BLOCK_SERVICE_FLAGS,
	REGISTER_BLOCK_SERVICES,
	CHANGED_BLOCK_SERVICES_AT_LOCATION,
	SHARDS_AT_LOCATION,
	CDC_AT_LOCATION,
	REGISTER_REGISTRY,
	ALL_REGISTRY_REPLICAS,
	SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED,
	CDC_REPLICAS_DE_PR_EC_AT_ED,
	ALL_SHARDS,
	DECOMMISSION_BLOCK_SERVICE,
	MOVE_SHARD_LEADER,
	CLEAR_SHARD_INFO,
	SHARD_BLOCK_SERVICES,
	ALL_CDC,
	ERASE_DECOMMISSIONED_BLOCK,
	ALL_BLOCK_SERVICES_DEPRECATED,
	MOVE_CDC_LEADER,
	CLEAR_CDC_INFO,
	UPDATE_BLOCK_SERVICE_PATH,
}

const MaxRegistryMessageKind RegistryMessageKind = 37

func MkRegistryMessage(k string) (RegistryRequest, RegistryResponse, error) {
	switch {
	case k == "LOCAL_SHARDS":
		return &LocalShardsReq{}, &LocalShardsResp{}, nil
	case k == "LOCAL_CDC":
		return &LocalCdcReq{}, &LocalCdcResp{}, nil
	case k == "INFO":
		return &InfoReq{}, &InfoResp{}, nil
	case k == "REGISTRY":
		return &RegistryReq{}, &RegistryResp{}, nil
	case k == "LOCAL_CHANGED_BLOCK_SERVICES":
		return &LocalChangedBlockServicesReq{}, &LocalChangedBlockServicesResp{}, nil
	case k == "CREATE_LOCATION":
		return &CreateLocationReq{}, &CreateLocationResp{}, nil
	case k == "RENAME_LOCATION":
		return &RenameLocationReq{}, &RenameLocationResp{}, nil
	case k == "REGISTER_SHARD":
		return &RegisterShardReq{}, &RegisterShardResp{}, nil
	case k == "LOCATIONS":
		return &LocationsReq{}, &LocationsResp{}, nil
	case k == "REGISTER_CDC":
		return &RegisterCdcReq{}, &RegisterCdcResp{}, nil
	case k == "SET_BLOCK_SERVICE_FLAGS":
		return &SetBlockServiceFlagsReq{}, &SetBlockServiceFlagsResp{}, nil
	case k == "REGISTER_BLOCK_SERVICES":
		return &RegisterBlockServicesReq{}, &RegisterBlockServicesResp{}, nil
	case k == "CHANGED_BLOCK_SERVICES_AT_LOCATION":
		return &ChangedBlockServicesAtLocationReq{}, &ChangedBlockServicesAtLocationResp{}, nil
	case k == "SHARDS_AT_LOCATION":
		return &ShardsAtLocationReq{}, &ShardsAtLocationResp{}, nil
	case k == "CDC_AT_LOCATION":
		return &CdcAtLocationReq{}, &CdcAtLocationResp{}, nil
	case k == "REGISTER_REGISTRY":
		return &RegisterRegistryReq{}, &RegisterRegistryResp{}, nil
	case k == "ALL_REGISTRY_REPLICAS":
		return &AllRegistryReplicasReq{}, &AllRegistryReplicasResp{}, nil
	case k == "SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED":
		return &ShardBlockServicesDEPRECATEDReq{}, &ShardBlockServicesDEPRECATEDResp{}, nil
	case k == "CDC_REPLICAS_DE_PR_EC_AT_ED":
		return &CdcReplicasDEPRECATEDReq{}, &CdcReplicasDEPRECATEDResp{}, nil
	case k == "ALL_SHARDS":
		return &AllShardsReq{}, &AllShardsResp{}, nil
	case k == "DECOMMISSION_BLOCK_SERVICE":
		return &DecommissionBlockServiceReq{}, &DecommissionBlockServiceResp{}, nil
	case k == "MOVE_SHARD_LEADER":
		return &MoveShardLeaderReq{}, &MoveShardLeaderResp{}, nil
	case k == "CLEAR_SHARD_INFO":
		return &ClearShardInfoReq{}, &ClearShardInfoResp{}, nil
	case k == "SHARD_BLOCK_SERVICES":
		return &ShardBlockServicesReq{}, &ShardBlockServicesResp{}, nil
	case k == "ALL_CDC":
		return &AllCdcReq{}, &AllCdcResp{}, nil
	case k == "ERASE_DECOMMISSIONED_BLOCK":
		return &EraseDecommissionedBlockReq{}, &EraseDecommissionedBlockResp{}, nil
	case k == "ALL_BLOCK_SERVICES_DEPRECATED":
		return &AllBlockServicesDeprecatedReq{}, &AllBlockServicesDeprecatedResp{}, nil
	case k == "MOVE_CDC_LEADER":
		return &MoveCdcLeaderReq{}, &MoveCdcLeaderResp{}, nil
	case k == "CLEAR_CDC_INFO":
		return &ClearCdcInfoReq{}, &ClearCdcInfoResp{}, nil
	case k == "UPDATE_BLOCK_SERVICE_PATH":
		return &UpdateBlockServicePathReq{}, &UpdateBlockServicePathResp{}, nil
	default:
		return nil, nil, fmt.Errorf("bad kind string %s", k)
	}
}

func (k BlocksMessageKind) String() string {
	switch k {
	case 2:
		return "FETCH_BLOCK"
	case 3:
		return "WRITE_BLOCK"
	case 4:
		return "FETCH_BLOCK_WITH_CRC"
	case 1:
		return "ERASE_BLOCK"
	case 5:
		return "TEST_WRITE"
	case 6:
		return "CHECK_BLOCK"
	default:
		return fmt.Sprintf("BlocksMessageKind(%d)", k)
	}
}

const (
	FETCH_BLOCK          BlocksMessageKind = 0x2
	WRITE_BLOCK          BlocksMessageKind = 0x3
	FETCH_BLOCK_WITH_CRC BlocksMessageKind = 0x4
	ERASE_BLOCK          BlocksMessageKind = 0x1
	TEST_WRITE           BlocksMessageKind = 0x5
	CHECK_BLOCK          BlocksMessageKind = 0x6
)

var AllBlocksMessageKind = [...]BlocksMessageKind{
	FETCH_BLOCK,
	WRITE_BLOCK,
	FETCH_BLOCK_WITH_CRC,
	ERASE_BLOCK,
	TEST_WRITE,
	CHECK_BLOCK,
}

const MaxBlocksMessageKind BlocksMessageKind = 6

func MkBlocksMessage(k string) (BlocksRequest, BlocksResponse, error) {
	switch {
	case k == "FETCH_BLOCK":
		return &FetchBlockReq{}, &FetchBlockResp{}, nil
	case k == "WRITE_BLOCK":
		return &WriteBlockReq{}, &WriteBlockResp{}, nil
	case k == "FETCH_BLOCK_WITH_CRC":
		return &FetchBlockWithCrcReq{}, &FetchBlockWithCrcResp{}, nil
	case k == "ERASE_BLOCK":
		return &EraseBlockReq{}, &EraseBlockResp{}, nil
	case k == "TEST_WRITE":
		return &TestWriteReq{}, &TestWriteResp{}, nil
	case k == "CHECK_BLOCK":
		return &CheckBlockReq{}, &CheckBlockResp{}, nil
	default:
		return nil, nil, fmt.Errorf("bad kind string %s", k)
	}
}

func (k LogMessageKind) String() string {
	switch k {
	case 1:
		return "LOG_WRITE"
	case 2:
		return "RELEASE"
	case 3:
		return "LOG_READ"
	case 4:
		return "NEW_LEADER"
	case 5:
		return "NEW_LEADER_CONFIRM"
	case 6:
		return "LOG_RECOVERY_READ"
	case 7:
		return "LOG_RECOVERY_WRITE"
	default:
		return fmt.Sprintf("LogMessageKind(%d)", k)
	}
}

const (
	LOG_WRITE          LogMessageKind = 0x1
	RELEASE            LogMessageKind = 0x2
	LOG_READ           LogMessageKind = 0x3
	NEW_LEADER         LogMessageKind = 0x4
	NEW_LEADER_CONFIRM LogMessageKind = 0x5
	LOG_RECOVERY_READ  LogMessageKind = 0x6
	LOG_RECOVERY_WRITE LogMessageKind = 0x7
)

var AllLogMessageKind = [...]LogMessageKind{
	LOG_WRITE,
	RELEASE,
	LOG_READ,
	NEW_LEADER,
	NEW_LEADER_CONFIRM,
	LOG_RECOVERY_READ,
	LOG_RECOVERY_WRITE,
}

const MaxLogMessageKind LogMessageKind = 7

func MkLogMessage(k string) (LogRequest, LogResponse, error) {
	switch {
	case k == "LOG_WRITE":
		return &LogWriteReq{}, &LogWriteResp{}, nil
	case k == "RELEASE":
		return &ReleaseReq{}, &ReleaseResp{}, nil
	case k == "LOG_READ":
		return &LogReadReq{}, &LogReadResp{}, nil
	case k == "NEW_LEADER":
		return &NewLeaderReq{}, &NewLeaderResp{}, nil
	case k == "NEW_LEADER_CONFIRM":
		return &NewLeaderConfirmReq{}, &NewLeaderConfirmResp{}, nil
	case k == "LOG_RECOVERY_READ":
		return &LogRecoveryReadReq{}, &LogRecoveryReadResp{}, nil
	case k == "LOG_RECOVERY_WRITE":
		return &LogRecoveryWriteReq{}, &LogRecoveryWriteResp{}, nil
	default:
		return nil, nil, fmt.Errorf("bad kind string %s", k)
	}
}

func (v *LookupReq) ShardRequestKind() ShardMessageKind {
	return LOOKUP
}

func (v *LookupReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	return nil
}

func (v *LookupReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	return nil
}

func (v *LookupResp) ShardResponseKind() ShardMessageKind {
	return LOOKUP
}

func (v *LookupResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *LookupResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *StatFileReq) ShardRequestKind() ShardMessageKind {
	return STAT_FILE
}

func (v *StatFileReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	return nil
}

func (v *StatFileReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	return nil
}

func (v *StatFileResp) ShardResponseKind() ShardMessageKind {
	return STAT_FILE
}

func (v *StatFileResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Mtime)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Atime)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Size)); err != nil {
		return err
	}
	return nil
}

func (v *StatFileResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Mtime)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Atime)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Size)); err != nil {
		return err
	}
	return nil
}

func (v *StatDirectoryReq) ShardRequestKind() ShardMessageKind {
	return STAT_DIRECTORY
}

func (v *StatDirectoryReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	return nil
}

func (v *StatDirectoryReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	return nil
}

func (v *StatDirectoryResp) ShardResponseKind() ShardMessageKind {
	return STAT_DIRECTORY
}

func (v *StatDirectoryResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Mtime)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Owner)); err != nil {
		return err
	}
	if err := v.Info.Pack(w); err != nil {
		return err
	}
	return nil
}

func (v *StatDirectoryResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Mtime)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Owner)); err != nil {
		return err
	}
	if err := v.Info.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (v *ReadDirReq) ShardRequestKind() ShardMessageKind {
	return READ_DIR
}

func (v *ReadDirReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.StartHash)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint16(v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *ReadDirReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.StartHash)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *ReadDirResp) ShardResponseKind() ShardMessageKind {
	return READ_DIR
}

func (v *ReadDirResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NextHash)); err != nil {
		return err
	}
	len1 := len(v.Results)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Results[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *ReadDirResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NextHash)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Results, len1)
	for i := 0; i < len1; i++ {
		if err := v.Results[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *ConstructFileReq) ShardRequestKind() ShardMessageKind {
	return CONSTRUCT_FILE
}

func (v *ConstructFileReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.Type)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Note)); err != nil {
		return err
	}
	return nil
}

func (v *ConstructFileReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Type)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Note); err != nil {
		return err
	}
	return nil
}

func (v *ConstructFileResp) ShardResponseKind() ShardMessageKind {
	return CONSTRUCT_FILE
}

func (v *ConstructFileResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie[:]); err != nil {
		return err
	}
	return nil
}

func (v *ConstructFileResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie[:]); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanInitiateReq) ShardRequestKind() ShardMessageKind {
	return ADD_SPAN_INITIATE
}

func (v *AddSpanInitiateReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Size)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Crc)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.StorageClass)); err != nil {
		return err
	}
	len1 := len(v.Blacklist)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Blacklist[i].Pack(w); err != nil {
			return err
		}
	}
	if err := bincode.PackScalar(w, uint8(v.Parity)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Stripes)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.CellSize)); err != nil {
		return err
	}
	len2 := len(v.Crcs)
	if err := bincode.PackLength(w, len2); err != nil {
		return err
	}
	for i := 0; i < len2; i++ {
		if err := bincode.PackScalar(w, uint32(v.Crcs[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *AddSpanInitiateReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Size)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Crc)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.StorageClass)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Blacklist, len1)
	for i := 0; i < len1; i++ {
		if err := v.Blacklist[i].Unpack(r); err != nil {
			return err
		}
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Parity)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Stripes)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.CellSize)); err != nil {
		return err
	}
	var len2 int
	if err := bincode.UnpackLength(r, &len2); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Crcs, len2)
	for i := 0; i < len2; i++ {
		if err := bincode.UnpackScalar(r, (*uint32)(&v.Crcs[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *AddSpanInitiateResp) ShardResponseKind() ShardMessageKind {
	return ADD_SPAN_INITIATE
}

func (v *AddSpanInitiateResp) Pack(w io.Writer) error {
	len1 := len(v.Blocks)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Blocks[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *AddSpanInitiateResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Blocks, len1)
	for i := 0; i < len1; i++ {
		if err := v.Blocks[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *AddSpanCertifyReq) ShardRequestKind() ShardMessageKind {
	return ADD_SPAN_CERTIFY
}

func (v *AddSpanCertifyReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset)); err != nil {
		return err
	}
	len1 := len(v.Proofs)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Proofs[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *AddSpanCertifyReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Proofs, len1)
	for i := 0; i < len1; i++ {
		if err := v.Proofs[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *AddSpanCertifyResp) ShardResponseKind() ShardMessageKind {
	return ADD_SPAN_CERTIFY
}

func (v *AddSpanCertifyResp) Pack(w io.Writer) error {
	return nil
}

func (v *AddSpanCertifyResp) Unpack(r io.Reader) error {
	return nil
}

func (v *LinkFileReq) ShardRequestKind() ShardMessageKind {
	return LINK_FILE
}

func (v *LinkFileReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	return nil
}

func (v *LinkFileReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	return nil
}

func (v *LinkFileResp) ShardResponseKind() ShardMessageKind {
	return LINK_FILE
}

func (v *LinkFileResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *LinkFileResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *SoftUnlinkFileReq) ShardRequestKind() ShardMessageKind {
	return SOFT_UNLINK_FILE
}

func (v *SoftUnlinkFileReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.FileId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *SoftUnlinkFileReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *SoftUnlinkFileResp) ShardResponseKind() ShardMessageKind {
	return SOFT_UNLINK_FILE
}

func (v *SoftUnlinkFileResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DeleteCreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *SoftUnlinkFileResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DeleteCreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *LocalFileSpansReq) ShardRequestKind() ShardMessageKind {
	return LOCAL_FILE_SPANS
}

func (v *LocalFileSpansReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Limit)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint16(v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *LocalFileSpansReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Limit)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *LocalFileSpansResp) ShardResponseKind() ShardMessageKind {
	return LOCAL_FILE_SPANS
}

func (v *LocalFileSpansResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NextOffset)); err != nil {
		return err
	}
	len1 := len(v.BlockServices)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Pack(w); err != nil {
			return err
		}
	}
	len2 := len(v.Spans)
	if err := bincode.PackLength(w, len2); err != nil {
		return err
	}
	for i := 0; i < len2; i++ {
		if err := v.Spans[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *LocalFileSpansResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NextOffset)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.BlockServices, len1)
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Unpack(r); err != nil {
			return err
		}
	}
	var len2 int
	if err := bincode.UnpackLength(r, &len2); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Spans, len2)
	for i := 0; i < len2; i++ {
		if err := v.Spans[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *SameDirectoryRenameReq) ShardRequestKind() ShardMessageKind {
	return SAME_DIRECTORY_RENAME
}

func (v *SameDirectoryRenameReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.OldName)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.OldCreationTime)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.NewName)); err != nil {
		return err
	}
	return nil
}

func (v *SameDirectoryRenameReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.OldName); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OldCreationTime)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.NewName); err != nil {
		return err
	}
	return nil
}

func (v *SameDirectoryRenameResp) ShardResponseKind() ShardMessageKind {
	return SAME_DIRECTORY_RENAME
}

func (v *SameDirectoryRenameResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NewCreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *SameDirectoryRenameResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NewCreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *AddInlineSpanReq) ShardRequestKind() ShardMessageKind {
	return ADD_INLINE_SPAN
}

func (v *AddInlineSpanReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.StorageClass)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Size)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Crc)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Body)); err != nil {
		return err
	}
	return nil
}

func (v *AddInlineSpanReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.StorageClass)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Size)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Crc)); err != nil {
		return err
	}
	if err := bincode.UnpackBytes(r, (*[]byte)(&v.Body)); err != nil {
		return err
	}
	return nil
}

func (v *AddInlineSpanResp) ShardResponseKind() ShardMessageKind {
	return ADD_INLINE_SPAN
}

func (v *AddInlineSpanResp) Pack(w io.Writer) error {
	return nil
}

func (v *AddInlineSpanResp) Unpack(r io.Reader) error {
	return nil
}

func (v *SetTimeReq) ShardRequestKind() ShardMessageKind {
	return SET_TIME
}

func (v *SetTimeReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Mtime)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Atime)); err != nil {
		return err
	}
	return nil
}

func (v *SetTimeReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Mtime)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Atime)); err != nil {
		return err
	}
	return nil
}

func (v *SetTimeResp) ShardResponseKind() ShardMessageKind {
	return SET_TIME
}

func (v *SetTimeResp) Pack(w io.Writer) error {
	return nil
}

func (v *SetTimeResp) Unpack(r io.Reader) error {
	return nil
}

func (v *FullReadDirReq) ShardRequestKind() ShardMessageKind {
	return FULL_READ_DIR
}

func (v *FullReadDirReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Flags)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.StartName)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.StartTime)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint16(v.Limit)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint16(v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *FullReadDirReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Flags)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.StartName); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.StartTime)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Limit)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *FullReadDirResp) ShardResponseKind() ShardMessageKind {
	return FULL_READ_DIR
}

func (v *FullReadDirResp) Pack(w io.Writer) error {
	if err := v.Next.Pack(w); err != nil {
		return err
	}
	len1 := len(v.Results)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Results[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *FullReadDirResp) Unpack(r io.Reader) error {
	if err := v.Next.Unpack(r); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Results, len1)
	for i := 0; i < len1; i++ {
		if err := v.Results[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *MoveSpanReq) ShardRequestKind() ShardMessageKind {
	return MOVE_SPAN
}

func (v *MoveSpanReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint32(v.SpanSize)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.FileId1)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset1)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie1[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.FileId2)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset2)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie2[:]); err != nil {
		return err
	}
	return nil
}

func (v *MoveSpanReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint32)(&v.SpanSize)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId1)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset1)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie1[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId2)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset2)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie2[:]); err != nil {
		return err
	}
	return nil
}

func (v *MoveSpanResp) ShardResponseKind() ShardMessageKind {
	return MOVE_SPAN
}

func (v *MoveSpanResp) Pack(w io.Writer) error {
	return nil
}

func (v *MoveSpanResp) Unpack(r io.Reader) error {
	return nil
}

func (v *RemoveNonOwnedEdgeReq) ShardRequestKind() ShardMessageKind {
	return REMOVE_NON_OWNED_EDGE
}

func (v *RemoveNonOwnedEdgeReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *RemoveNonOwnedEdgeReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *RemoveNonOwnedEdgeResp) ShardResponseKind() ShardMessageKind {
	return REMOVE_NON_OWNED_EDGE
}

func (v *RemoveNonOwnedEdgeResp) Pack(w io.Writer) error {
	return nil
}

func (v *RemoveNonOwnedEdgeResp) Unpack(r io.Reader) error {
	return nil
}

func (v *SameShardHardFileUnlinkReq) ShardRequestKind() ShardMessageKind {
	return SAME_SHARD_HARD_FILE_UNLINK
}

func (v *SameShardHardFileUnlinkReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *SameShardHardFileUnlinkReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *SameShardHardFileUnlinkResp) ShardResponseKind() ShardMessageKind {
	return SAME_SHARD_HARD_FILE_UNLINK
}

func (v *SameShardHardFileUnlinkResp) Pack(w io.Writer) error {
	return nil
}

func (v *SameShardHardFileUnlinkResp) Unpack(r io.Reader) error {
	return nil
}

func (v *StatTransientFileReq) ShardRequestKind() ShardMessageKind {
	return STAT_TRANSIENT_FILE
}

func (v *StatTransientFileReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	return nil
}

func (v *StatTransientFileReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	return nil
}

func (v *StatTransientFileResp) ShardResponseKind() ShardMessageKind {
	return STAT_TRANSIENT_FILE
}

func (v *StatTransientFileResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Mtime)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Size)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Note)); err != nil {
		return err
	}
	return nil
}

func (v *StatTransientFileResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Mtime)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Size)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Note); err != nil {
		return err
	}
	return nil
}

func (v *ShardSnapshotReq) ShardRequestKind() ShardMessageKind {
	return SHARD_SNAPSHOT
}

func (v *ShardSnapshotReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.SnapshotId)); err != nil {
		return err
	}
	return nil
}

func (v *ShardSnapshotReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.SnapshotId)); err != nil {
		return err
	}
	return nil
}

func (v *ShardSnapshotResp) ShardResponseKind() ShardMessageKind {
	return SHARD_SNAPSHOT
}

func (v *ShardSnapshotResp) Pack(w io.Writer) error {
	return nil
}

func (v *ShardSnapshotResp) Unpack(r io.Reader) error {
	return nil
}

func (v *FileSpansReq) ShardRequestKind() ShardMessageKind {
	return FILE_SPANS
}

func (v *FileSpansReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Limit)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint16(v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *FileSpansReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Limit)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *FileSpansResp) ShardResponseKind() ShardMessageKind {
	return FILE_SPANS
}

func (v *FileSpansResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NextOffset)); err != nil {
		return err
	}
	len1 := len(v.BlockServices)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Pack(w); err != nil {
			return err
		}
	}
	len2 := len(v.Spans)
	if err := bincode.PackLength(w, len2); err != nil {
		return err
	}
	for i := 0; i < len2; i++ {
		if err := v.Spans[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *FileSpansResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NextOffset)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.BlockServices, len1)
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Unpack(r); err != nil {
			return err
		}
	}
	var len2 int
	if err := bincode.UnpackLength(r, &len2); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Spans, len2)
	for i := 0; i < len2; i++ {
		if err := v.Spans[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *AddSpanLocationReq) ShardRequestKind() ShardMessageKind {
	return ADD_SPAN_LOCATION
}

func (v *AddSpanLocationReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId1)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset1)); err != nil {
		return err
	}
	len1 := len(v.Blocks1)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := bincode.PackScalar(w, uint64(v.Blocks1[i])); err != nil {
			return err
		}
	}
	if err := bincode.PackScalar(w, uint64(v.FileId2)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset2)); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanLocationReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId1)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset1)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Blocks1, len1)
	for i := 0; i < len1; i++ {
		if err := bincode.UnpackScalar(r, (*uint64)(&v.Blocks1[i])); err != nil {
			return err
		}
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId2)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset2)); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanLocationResp) ShardResponseKind() ShardMessageKind {
	return ADD_SPAN_LOCATION
}

func (v *AddSpanLocationResp) Pack(w io.Writer) error {
	return nil
}

func (v *AddSpanLocationResp) Unpack(r io.Reader) error {
	return nil
}

func (v *ScrapTransientFileReq) ShardRequestKind() ShardMessageKind {
	return SCRAP_TRANSIENT_FILE
}

func (v *ScrapTransientFileReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie[:]); err != nil {
		return err
	}
	return nil
}

func (v *ScrapTransientFileReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie[:]); err != nil {
		return err
	}
	return nil
}

func (v *ScrapTransientFileResp) ShardResponseKind() ShardMessageKind {
	return SCRAP_TRANSIENT_FILE
}

func (v *ScrapTransientFileResp) Pack(w io.Writer) error {
	return nil
}

func (v *ScrapTransientFileResp) Unpack(r io.Reader) error {
	return nil
}

func (v *SetDirectoryInfoReq) ShardRequestKind() ShardMessageKind {
	return SET_DIRECTORY_INFO
}

func (v *SetDirectoryInfoReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := v.Info.Pack(w); err != nil {
		return err
	}
	return nil
}

func (v *SetDirectoryInfoReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := v.Info.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (v *SetDirectoryInfoResp) ShardResponseKind() ShardMessageKind {
	return SET_DIRECTORY_INFO
}

func (v *SetDirectoryInfoResp) Pack(w io.Writer) error {
	return nil
}

func (v *SetDirectoryInfoResp) Unpack(r io.Reader) error {
	return nil
}

func (v *VisitDirectoriesReq) ShardRequestKind() ShardMessageKind {
	return VISIT_DIRECTORIES
}

func (v *VisitDirectoriesReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BeginId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint16(v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *VisitDirectoriesReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BeginId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *VisitDirectoriesResp) ShardResponseKind() ShardMessageKind {
	return VISIT_DIRECTORIES
}

func (v *VisitDirectoriesResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NextId)); err != nil {
		return err
	}
	len1 := len(v.Ids)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := bincode.PackScalar(w, uint64(v.Ids[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *VisitDirectoriesResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NextId)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Ids, len1)
	for i := 0; i < len1; i++ {
		if err := bincode.UnpackScalar(r, (*uint64)(&v.Ids[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *VisitFilesReq) ShardRequestKind() ShardMessageKind {
	return VISIT_FILES
}

func (v *VisitFilesReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BeginId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint16(v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *VisitFilesReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BeginId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *VisitFilesResp) ShardResponseKind() ShardMessageKind {
	return VISIT_FILES
}

func (v *VisitFilesResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NextId)); err != nil {
		return err
	}
	len1 := len(v.Ids)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := bincode.PackScalar(w, uint64(v.Ids[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *VisitFilesResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NextId)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Ids, len1)
	for i := 0; i < len1; i++ {
		if err := bincode.UnpackScalar(r, (*uint64)(&v.Ids[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *VisitTransientFilesReq) ShardRequestKind() ShardMessageKind {
	return VISIT_TRANSIENT_FILES
}

func (v *VisitTransientFilesReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BeginId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint16(v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *VisitTransientFilesReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BeginId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Mtu)); err != nil {
		return err
	}
	return nil
}

func (v *VisitTransientFilesResp) ShardResponseKind() ShardMessageKind {
	return VISIT_TRANSIENT_FILES
}

func (v *VisitTransientFilesResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NextId)); err != nil {
		return err
	}
	len1 := len(v.Files)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Files[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *VisitTransientFilesResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NextId)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Files, len1)
	for i := 0; i < len1; i++ {
		if err := v.Files[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *RemoveSpanInitiateReq) ShardRequestKind() ShardMessageKind {
	return REMOVE_SPAN_INITIATE
}

func (v *RemoveSpanInitiateReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie[:]); err != nil {
		return err
	}
	return nil
}

func (v *RemoveSpanInitiateReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie[:]); err != nil {
		return err
	}
	return nil
}

func (v *RemoveSpanInitiateResp) ShardResponseKind() ShardMessageKind {
	return REMOVE_SPAN_INITIATE
}

func (v *RemoveSpanInitiateResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.ByteOffset)); err != nil {
		return err
	}
	len1 := len(v.Blocks)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Blocks[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *RemoveSpanInitiateResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Blocks, len1)
	for i := 0; i < len1; i++ {
		if err := v.Blocks[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *RemoveSpanCertifyReq) ShardRequestKind() ShardMessageKind {
	return REMOVE_SPAN_CERTIFY
}

func (v *RemoveSpanCertifyReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset)); err != nil {
		return err
	}
	len1 := len(v.Proofs)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Proofs[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *RemoveSpanCertifyReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Proofs, len1)
	for i := 0; i < len1; i++ {
		if err := v.Proofs[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *RemoveSpanCertifyResp) ShardResponseKind() ShardMessageKind {
	return REMOVE_SPAN_CERTIFY
}

func (v *RemoveSpanCertifyResp) Pack(w io.Writer) error {
	return nil
}

func (v *RemoveSpanCertifyResp) Unpack(r io.Reader) error {
	return nil
}

func (v *SwapBlocksReq) ShardRequestKind() ShardMessageKind {
	return SWAP_BLOCKS
}

func (v *SwapBlocksReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId1)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset1)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.BlockId1)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.FileId2)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset2)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.BlockId2)); err != nil {
		return err
	}
	return nil
}

func (v *SwapBlocksReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId1)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset1)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId1)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId2)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset2)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId2)); err != nil {
		return err
	}
	return nil
}

func (v *SwapBlocksResp) ShardResponseKind() ShardMessageKind {
	return SWAP_BLOCKS
}

func (v *SwapBlocksResp) Pack(w io.Writer) error {
	return nil
}

func (v *SwapBlocksResp) Unpack(r io.Reader) error {
	return nil
}

func (v *BlockServiceFilesReq) ShardRequestKind() ShardMessageKind {
	return BLOCK_SERVICE_FILES
}

func (v *BlockServiceFilesReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BlockServiceId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.StartFrom)); err != nil {
		return err
	}
	return nil
}

func (v *BlockServiceFilesReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockServiceId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.StartFrom)); err != nil {
		return err
	}
	return nil
}

func (v *BlockServiceFilesResp) ShardResponseKind() ShardMessageKind {
	return BLOCK_SERVICE_FILES
}

func (v *BlockServiceFilesResp) Pack(w io.Writer) error {
	len1 := len(v.FileIds)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := bincode.PackScalar(w, uint64(v.FileIds[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *BlockServiceFilesResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.FileIds, len1)
	for i := 0; i < len1; i++ {
		if err := bincode.UnpackScalar(r, (*uint64)(&v.FileIds[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *RemoveInodeReq) ShardRequestKind() ShardMessageKind {
	return REMOVE_INODE
}

func (v *RemoveInodeReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	return nil
}

func (v *RemoveInodeReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	return nil
}

func (v *RemoveInodeResp) ShardResponseKind() ShardMessageKind {
	return REMOVE_INODE
}

func (v *RemoveInodeResp) Pack(w io.Writer) error {
	return nil
}

func (v *RemoveInodeResp) Unpack(r io.Reader) error {
	return nil
}

func (v *AddSpanInitiateWithReferenceReq) ShardRequestKind() ShardMessageKind {
	return ADD_SPAN_INITIATE_WITH_REFERENCE
}

func (v *AddSpanInitiateWithReferenceReq) Pack(w io.Writer) error {
	if err := v.Req.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Reference)); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanInitiateWithReferenceReq) Unpack(r io.Reader) error {
	if err := v.Req.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Reference)); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanInitiateWithReferenceResp) ShardResponseKind() ShardMessageKind {
	return ADD_SPAN_INITIATE_WITH_REFERENCE
}

func (v *AddSpanInitiateWithReferenceResp) Pack(w io.Writer) error {
	if err := v.Resp.Pack(w); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanInitiateWithReferenceResp) Unpack(r io.Reader) error {
	if err := v.Resp.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (v *RemoveZeroBlockServiceFilesReq) ShardRequestKind() ShardMessageKind {
	return REMOVE_ZERO_BLOCK_SERVICE_FILES
}

func (v *RemoveZeroBlockServiceFilesReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.StartBlockService)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.StartFile)); err != nil {
		return err
	}
	return nil
}

func (v *RemoveZeroBlockServiceFilesReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.StartBlockService)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.StartFile)); err != nil {
		return err
	}
	return nil
}

func (v *RemoveZeroBlockServiceFilesResp) ShardResponseKind() ShardMessageKind {
	return REMOVE_ZERO_BLOCK_SERVICE_FILES
}

func (v *RemoveZeroBlockServiceFilesResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Removed)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.NextBlockService)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.NextFile)); err != nil {
		return err
	}
	return nil
}

func (v *RemoveZeroBlockServiceFilesResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Removed)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NextBlockService)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NextFile)); err != nil {
		return err
	}
	return nil
}

func (v *SwapSpansReq) ShardRequestKind() ShardMessageKind {
	return SWAP_SPANS
}

func (v *SwapSpansReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId1)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset1)); err != nil {
		return err
	}
	len1 := len(v.Blocks1)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := bincode.PackScalar(w, uint64(v.Blocks1[i])); err != nil {
			return err
		}
	}
	if err := bincode.PackScalar(w, uint64(v.FileId2)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ByteOffset2)); err != nil {
		return err
	}
	len2 := len(v.Blocks2)
	if err := bincode.PackLength(w, len2); err != nil {
		return err
	}
	for i := 0; i < len2; i++ {
		if err := bincode.PackScalar(w, uint64(v.Blocks2[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *SwapSpansReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId1)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset1)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Blocks1, len1)
	for i := 0; i < len1; i++ {
		if err := bincode.UnpackScalar(r, (*uint64)(&v.Blocks1[i])); err != nil {
			return err
		}
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId2)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset2)); err != nil {
		return err
	}
	var len2 int
	if err := bincode.UnpackLength(r, &len2); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Blocks2, len2)
	for i := 0; i < len2; i++ {
		if err := bincode.UnpackScalar(r, (*uint64)(&v.Blocks2[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *SwapSpansResp) ShardResponseKind() ShardMessageKind {
	return SWAP_SPANS
}

func (v *SwapSpansResp) Pack(w io.Writer) error {
	return nil
}

func (v *SwapSpansResp) Unpack(r io.Reader) error {
	return nil
}

func (v *SameDirectoryRenameSnapshotReq) ShardRequestKind() ShardMessageKind {
	return SAME_DIRECTORY_RENAME_SNAPSHOT
}

func (v *SameDirectoryRenameSnapshotReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.OldName)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.OldCreationTime)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.NewName)); err != nil {
		return err
	}
	return nil
}

func (v *SameDirectoryRenameSnapshotReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.OldName); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OldCreationTime)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.NewName); err != nil {
		return err
	}
	return nil
}

func (v *SameDirectoryRenameSnapshotResp) ShardResponseKind() ShardMessageKind {
	return SAME_DIRECTORY_RENAME_SNAPSHOT
}

func (v *SameDirectoryRenameSnapshotResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NewCreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *SameDirectoryRenameSnapshotResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NewCreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanAtLocationInitiateReq) ShardRequestKind() ShardMessageKind {
	return ADD_SPAN_AT_LOCATION_INITIATE
}

func (v *AddSpanAtLocationInitiateReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	if err := v.Req.Pack(w); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanAtLocationInitiateReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	if err := v.Req.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanAtLocationInitiateResp) ShardResponseKind() ShardMessageKind {
	return ADD_SPAN_AT_LOCATION_INITIATE
}

func (v *AddSpanAtLocationInitiateResp) Pack(w io.Writer) error {
	if err := v.Resp.Pack(w); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanAtLocationInitiateResp) Unpack(r io.Reader) error {
	if err := v.Resp.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (v *CreateDirectoryInodeReq) ShardRequestKind() ShardMessageKind {
	return CREATE_DIRECTORY_INODE
}

func (v *CreateDirectoryInodeReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.OwnerId)); err != nil {
		return err
	}
	if err := v.Info.Pack(w); err != nil {
		return err
	}
	return nil
}

func (v *CreateDirectoryInodeReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OwnerId)); err != nil {
		return err
	}
	if err := v.Info.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (v *CreateDirectoryInodeResp) ShardResponseKind() ShardMessageKind {
	return CREATE_DIRECTORY_INODE
}

func (v *CreateDirectoryInodeResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Mtime)); err != nil {
		return err
	}
	return nil
}

func (v *CreateDirectoryInodeResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Mtime)); err != nil {
		return err
	}
	return nil
}

func (v *SetDirectoryOwnerReq) ShardRequestKind() ShardMessageKind {
	return SET_DIRECTORY_OWNER
}

func (v *SetDirectoryOwnerReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.OwnerId)); err != nil {
		return err
	}
	return nil
}

func (v *SetDirectoryOwnerReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OwnerId)); err != nil {
		return err
	}
	return nil
}

func (v *SetDirectoryOwnerResp) ShardResponseKind() ShardMessageKind {
	return SET_DIRECTORY_OWNER
}

func (v *SetDirectoryOwnerResp) Pack(w io.Writer) error {
	return nil
}

func (v *SetDirectoryOwnerResp) Unpack(r io.Reader) error {
	return nil
}

func (v *RemoveDirectoryOwnerReq) ShardRequestKind() ShardMessageKind {
	return REMOVE_DIRECTORY_OWNER
}

func (v *RemoveDirectoryOwnerReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := v.Info.Pack(w); err != nil {
		return err
	}
	return nil
}

func (v *RemoveDirectoryOwnerReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := v.Info.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (v *RemoveDirectoryOwnerResp) ShardResponseKind() ShardMessageKind {
	return REMOVE_DIRECTORY_OWNER
}

func (v *RemoveDirectoryOwnerResp) Pack(w io.Writer) error {
	return nil
}

func (v *RemoveDirectoryOwnerResp) Unpack(r io.Reader) error {
	return nil
}

func (v *CreateLockedCurrentEdgeReq) ShardRequestKind() ShardMessageKind {
	return CREATE_LOCKED_CURRENT_EDGE
}

func (v *CreateLockedCurrentEdgeReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.OldCreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *CreateLockedCurrentEdgeReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OldCreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *CreateLockedCurrentEdgeResp) ShardResponseKind() ShardMessageKind {
	return CREATE_LOCKED_CURRENT_EDGE
}

func (v *CreateLockedCurrentEdgeResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *CreateLockedCurrentEdgeResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *LockCurrentEdgeReq) ShardRequestKind() ShardMessageKind {
	return LOCK_CURRENT_EDGE
}

func (v *LockCurrentEdgeReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	return nil
}

func (v *LockCurrentEdgeReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	return nil
}

func (v *LockCurrentEdgeResp) ShardResponseKind() ShardMessageKind {
	return LOCK_CURRENT_EDGE
}

func (v *LockCurrentEdgeResp) Pack(w io.Writer) error {
	return nil
}

func (v *LockCurrentEdgeResp) Unpack(r io.Reader) error {
	return nil
}

func (v *UnlockCurrentEdgeReq) ShardRequestKind() ShardMessageKind {
	return UNLOCK_CURRENT_EDGE
}

func (v *UnlockCurrentEdgeReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.WasMoved)); err != nil {
		return err
	}
	return nil
}

func (v *UnlockCurrentEdgeReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.WasMoved)); err != nil {
		return err
	}
	return nil
}

func (v *UnlockCurrentEdgeResp) ShardResponseKind() ShardMessageKind {
	return UNLOCK_CURRENT_EDGE
}

func (v *UnlockCurrentEdgeResp) Pack(w io.Writer) error {
	return nil
}

func (v *UnlockCurrentEdgeResp) Unpack(r io.Reader) error {
	return nil
}

func (v *RemoveOwnedSnapshotFileEdgeReq) ShardRequestKind() ShardMessageKind {
	return REMOVE_OWNED_SNAPSHOT_FILE_EDGE
}

func (v *RemoveOwnedSnapshotFileEdgeReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *RemoveOwnedSnapshotFileEdgeReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *RemoveOwnedSnapshotFileEdgeResp) ShardResponseKind() ShardMessageKind {
	return REMOVE_OWNED_SNAPSHOT_FILE_EDGE
}

func (v *RemoveOwnedSnapshotFileEdgeResp) Pack(w io.Writer) error {
	return nil
}

func (v *RemoveOwnedSnapshotFileEdgeResp) Unpack(r io.Reader) error {
	return nil
}

func (v *MakeFileTransientReq) ShardRequestKind() ShardMessageKind {
	return MAKE_FILE_TRANSIENT
}

func (v *MakeFileTransientReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Note)); err != nil {
		return err
	}
	return nil
}

func (v *MakeFileTransientReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Note); err != nil {
		return err
	}
	return nil
}

func (v *MakeFileTransientResp) ShardResponseKind() ShardMessageKind {
	return MAKE_FILE_TRANSIENT
}

func (v *MakeFileTransientResp) Pack(w io.Writer) error {
	return nil
}

func (v *MakeFileTransientResp) Unpack(r io.Reader) error {
	return nil
}

func (v *MakeDirectoryReq) CDCRequestKind() CDCMessageKind {
	return MAKE_DIRECTORY
}

func (v *MakeDirectoryReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	return nil
}

func (v *MakeDirectoryReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	return nil
}

func (v *MakeDirectoryResp) CDCResponseKind() CDCMessageKind {
	return MAKE_DIRECTORY
}

func (v *MakeDirectoryResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *MakeDirectoryResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *RenameFileReq) CDCRequestKind() CDCMessageKind {
	return RENAME_FILE
}

func (v *RenameFileReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.OldOwnerId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.OldName)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.OldCreationTime)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.NewOwnerId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.NewName)); err != nil {
		return err
	}
	return nil
}

func (v *RenameFileReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OldOwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.OldName); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OldCreationTime)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NewOwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.NewName); err != nil {
		return err
	}
	return nil
}

func (v *RenameFileResp) CDCResponseKind() CDCMessageKind {
	return RENAME_FILE
}

func (v *RenameFileResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *RenameFileResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *SoftUnlinkDirectoryReq) CDCRequestKind() CDCMessageKind {
	return SOFT_UNLINK_DIRECTORY
}

func (v *SoftUnlinkDirectoryReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	return nil
}

func (v *SoftUnlinkDirectoryReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	return nil
}

func (v *SoftUnlinkDirectoryResp) CDCResponseKind() CDCMessageKind {
	return SOFT_UNLINK_DIRECTORY
}

func (v *SoftUnlinkDirectoryResp) Pack(w io.Writer) error {
	return nil
}

func (v *SoftUnlinkDirectoryResp) Unpack(r io.Reader) error {
	return nil
}

func (v *RenameDirectoryReq) CDCRequestKind() CDCMessageKind {
	return RENAME_DIRECTORY
}

func (v *RenameDirectoryReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.OldOwnerId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.OldName)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.OldCreationTime)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.NewOwnerId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.NewName)); err != nil {
		return err
	}
	return nil
}

func (v *RenameDirectoryReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OldOwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.OldName); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OldCreationTime)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NewOwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.NewName); err != nil {
		return err
	}
	return nil
}

func (v *RenameDirectoryResp) CDCResponseKind() CDCMessageKind {
	return RENAME_DIRECTORY
}

func (v *RenameDirectoryResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *RenameDirectoryResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *HardUnlinkDirectoryReq) CDCRequestKind() CDCMessageKind {
	return HARD_UNLINK_DIRECTORY
}

func (v *HardUnlinkDirectoryReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DirId)); err != nil {
		return err
	}
	return nil
}

func (v *HardUnlinkDirectoryReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DirId)); err != nil {
		return err
	}
	return nil
}

func (v *HardUnlinkDirectoryResp) CDCResponseKind() CDCMessageKind {
	return HARD_UNLINK_DIRECTORY
}

func (v *HardUnlinkDirectoryResp) Pack(w io.Writer) error {
	return nil
}

func (v *HardUnlinkDirectoryResp) Unpack(r io.Reader) error {
	return nil
}

func (v *CrossShardHardUnlinkFileReq) CDCRequestKind() CDCMessageKind {
	return CROSS_SHARD_HARD_UNLINK_FILE
}

func (v *CrossShardHardUnlinkFileReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *CrossShardHardUnlinkFileReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.OwnerId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *CrossShardHardUnlinkFileResp) CDCResponseKind() CDCMessageKind {
	return CROSS_SHARD_HARD_UNLINK_FILE
}

func (v *CrossShardHardUnlinkFileResp) Pack(w io.Writer) error {
	return nil
}

func (v *CrossShardHardUnlinkFileResp) Unpack(r io.Reader) error {
	return nil
}

func (v *CdcSnapshotReq) CDCRequestKind() CDCMessageKind {
	return CDC_SNAPSHOT
}

func (v *CdcSnapshotReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.SnapshotId)); err != nil {
		return err
	}
	return nil
}

func (v *CdcSnapshotReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.SnapshotId)); err != nil {
		return err
	}
	return nil
}

func (v *CdcSnapshotResp) CDCResponseKind() CDCMessageKind {
	return CDC_SNAPSHOT
}

func (v *CdcSnapshotResp) Pack(w io.Writer) error {
	return nil
}

func (v *CdcSnapshotResp) Unpack(r io.Reader) error {
	return nil
}

func (v *FailureDomain) Pack(w io.Writer) error {
	if err := bincode.PackFixedBytes(w, 16, v.Name[:]); err != nil {
		return err
	}
	return nil
}

func (v *FailureDomain) Unpack(r io.Reader) error {
	if err := bincode.UnpackFixedBytes(r, 16, v.Name[:]); err != nil {
		return err
	}
	return nil
}

func (v *DirectoryInfoEntry) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.Tag)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Body)); err != nil {
		return err
	}
	return nil
}

func (v *DirectoryInfoEntry) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Tag)); err != nil {
		return err
	}
	if err := bincode.UnpackBytes(r, (*[]byte)(&v.Body)); err != nil {
		return err
	}
	return nil
}

func (v *DirectoryInfo) Pack(w io.Writer) error {
	len1 := len(v.Entries)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Entries[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *DirectoryInfo) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Entries, len1)
	for i := 0; i < len1; i++ {
		if err := v.Entries[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *CurrentEdge) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.NameHash)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *CurrentEdge) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NameHash)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanInitiateBlockInfo) Pack(w io.Writer) error {
	if err := v.BlockServiceAddrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.BlockServiceId)); err != nil {
		return err
	}
	if err := v.BlockServiceFailureDomain.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.BlockId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Certificate[:]); err != nil {
		return err
	}
	return nil
}

func (v *AddSpanInitiateBlockInfo) Unpack(r io.Reader) error {
	if err := v.BlockServiceAddrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockServiceId)); err != nil {
		return err
	}
	if err := v.BlockServiceFailureDomain.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Certificate[:]); err != nil {
		return err
	}
	return nil
}

func (v *RemoveSpanInitiateBlockInfo) Pack(w io.Writer) error {
	if err := v.BlockServiceAddrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.BlockServiceId)); err != nil {
		return err
	}
	if err := v.BlockServiceFailureDomain.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.BlockServiceFlags)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.BlockId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Certificate[:]); err != nil {
		return err
	}
	return nil
}

func (v *RemoveSpanInitiateBlockInfo) Unpack(r io.Reader) error {
	if err := v.BlockServiceAddrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockServiceId)); err != nil {
		return err
	}
	if err := v.BlockServiceFailureDomain.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.BlockServiceFlags)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Certificate[:]); err != nil {
		return err
	}
	return nil
}

func (v *BlockProof) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BlockId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Proof[:]); err != nil {
		return err
	}
	return nil
}

func (v *BlockProof) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Proof[:]); err != nil {
		return err
	}
	return nil
}

func (v *BlockService) Pack(w io.Writer) error {
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Flags)); err != nil {
		return err
	}
	return nil
}

func (v *BlockService) Unpack(r io.Reader) error {
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Flags)); err != nil {
		return err
	}
	return nil
}

func (v *ShardInfo) Pack(w io.Writer) error {
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastSeen)); err != nil {
		return err
	}
	return nil
}

func (v *ShardInfo) Unpack(r io.Reader) error {
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastSeen)); err != nil {
		return err
	}
	return nil
}

func (v *BlockPolicyEntry) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.StorageClass)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.MinSize)); err != nil {
		return err
	}
	return nil
}

func (v *BlockPolicyEntry) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.StorageClass)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.MinSize)); err != nil {
		return err
	}
	return nil
}

func (v *SpanPolicyEntry) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint32(v.MaxSize)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Parity)); err != nil {
		return err
	}
	return nil
}

func (v *SpanPolicyEntry) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint32)(&v.MaxSize)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Parity)); err != nil {
		return err
	}
	return nil
}

func (v *StripePolicy) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint32(v.TargetStripeSize)); err != nil {
		return err
	}
	return nil
}

func (v *StripePolicy) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint32)(&v.TargetStripeSize)); err != nil {
		return err
	}
	return nil
}

func (v *FetchedBlock) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.BlockServiceIx)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.BlockId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Crc)); err != nil {
		return err
	}
	return nil
}

func (v *FetchedBlock) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.BlockServiceIx)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Crc)); err != nil {
		return err
	}
	return nil
}

func (v *FetchedSpanHeader) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Size)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Crc)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.StorageClass)); err != nil {
		return err
	}
	return nil
}

func (v *FetchedSpanHeader) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Size)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Crc)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.StorageClass)); err != nil {
		return err
	}
	return nil
}

func (v *FetchedInlineSpan) Pack(w io.Writer) error {
	if err := bincode.PackBytes(w, []byte(v.Body)); err != nil {
		return err
	}
	return nil
}

func (v *FetchedInlineSpan) Unpack(r io.Reader) error {
	if err := bincode.UnpackBytes(r, (*[]byte)(&v.Body)); err != nil {
		return err
	}
	return nil
}

func (v *FetchedBlocksSpan) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.Parity)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Stripes)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.CellSize)); err != nil {
		return err
	}
	len1 := len(v.Blocks)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Blocks[i].Pack(w); err != nil {
			return err
		}
	}
	len2 := len(v.StripesCrc)
	if err := bincode.PackLength(w, len2); err != nil {
		return err
	}
	for i := 0; i < len2; i++ {
		if err := bincode.PackScalar(w, uint32(v.StripesCrc[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *FetchedBlocksSpan) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Parity)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Stripes)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.CellSize)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Blocks, len1)
	for i := 0; i < len1; i++ {
		if err := v.Blocks[i].Unpack(r); err != nil {
			return err
		}
	}
	var len2 int
	if err := bincode.UnpackLength(r, &len2); err != nil {
		return err
	}
	bincode.EnsureLength(&v.StripesCrc, len2)
	for i := 0; i < len2; i++ {
		if err := bincode.UnpackScalar(r, (*uint32)(&v.StripesCrc[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *FetchedBlockServices) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.StorageClass)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Parity)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Stripes)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.CellSize)); err != nil {
		return err
	}
	len1 := len(v.Blocks)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Blocks[i].Pack(w); err != nil {
			return err
		}
	}
	len2 := len(v.StripesCrc)
	if err := bincode.PackLength(w, len2); err != nil {
		return err
	}
	for i := 0; i < len2; i++ {
		if err := bincode.PackScalar(w, uint32(v.StripesCrc[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *FetchedBlockServices) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.StorageClass)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Parity)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Stripes)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.CellSize)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Blocks, len1)
	for i := 0; i < len1; i++ {
		if err := v.Blocks[i].Unpack(r); err != nil {
			return err
		}
	}
	var len2 int
	if err := bincode.UnpackLength(r, &len2); err != nil {
		return err
	}
	bincode.EnsureLength(&v.StripesCrc, len2)
	for i := 0; i < len2; i++ {
		if err := bincode.UnpackScalar(r, (*uint32)(&v.StripesCrc[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *FetchedLocations) Pack(w io.Writer) error {
	len1 := len(v.Locations)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Locations[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *FetchedLocations) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Locations, len1)
	for i := 0; i < len1; i++ {
		if err := v.Locations[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *FetchedSpanHeaderFull) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Size)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Crc)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.IsInline)); err != nil {
		return err
	}
	return nil
}

func (v *FetchedSpanHeaderFull) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ByteOffset)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Size)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Crc)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.IsInline)); err != nil {
		return err
	}
	return nil
}

func (v *BlacklistEntry) Pack(w io.Writer) error {
	if err := v.FailureDomain.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.BlockService)); err != nil {
		return err
	}
	return nil
}

func (v *BlacklistEntry) Unpack(r io.Reader) error {
	if err := v.FailureDomain.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockService)); err != nil {
		return err
	}
	return nil
}

func (v *Edge) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, bool(v.Current)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.TargetId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.NameHash)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *Edge) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*bool)(&v.Current)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.TargetId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NameHash)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CreationTime)); err != nil {
		return err
	}
	return nil
}

func (v *FullReadDirCursor) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, bool(v.Current)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.StartName)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.StartTime)); err != nil {
		return err
	}
	return nil
}

func (v *FullReadDirCursor) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*bool)(&v.Current)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.StartName); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.StartTime)); err != nil {
		return err
	}
	return nil
}

func (v *FullRegistryInfo) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastSeen)); err != nil {
		return err
	}
	return nil
}

func (v *FullRegistryInfo) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastSeen)); err != nil {
		return err
	}
	return nil
}

func (v *TransientFile) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.DeadlineTime)); err != nil {
		return err
	}
	return nil
}

func (v *TransientFile) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Cookie[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DeadlineTime)); err != nil {
		return err
	}
	return nil
}

func (v *EntryNewBlockInfo) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BlockServiceId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Crc)); err != nil {
		return err
	}
	return nil
}

func (v *EntryNewBlockInfo) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockServiceId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Crc)); err != nil {
		return err
	}
	return nil
}

func (v *BlockServiceDeprecatedInfo) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.StorageClass)); err != nil {
		return err
	}
	if err := v.FailureDomain.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 16, v.SecretKey[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Flags)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CapacityBytes)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.AvailableBytes)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Blocks)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Path)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastSeen)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.HasFiles)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.FlagsLastChanged)); err != nil {
		return err
	}
	return nil
}

func (v *BlockServiceDeprecatedInfo) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.StorageClass)); err != nil {
		return err
	}
	if err := v.FailureDomain.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 16, v.SecretKey[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Flags)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CapacityBytes)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.AvailableBytes)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Blocks)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Path); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastSeen)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.HasFiles)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FlagsLastChanged)); err != nil {
		return err
	}
	return nil
}

func (v *BlockServiceInfoShort) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	if err := v.FailureDomain.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.StorageClass)); err != nil {
		return err
	}
	return nil
}

func (v *BlockServiceInfoShort) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	if err := v.FailureDomain.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.StorageClass)); err != nil {
		return err
	}
	return nil
}

func (v *SpanPolicy) Pack(w io.Writer) error {
	len1 := len(v.Entries)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Entries[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *SpanPolicy) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Entries, len1)
	for i := 0; i < len1; i++ {
		if err := v.Entries[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *BlockPolicy) Pack(w io.Writer) error {
	len1 := len(v.Entries)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Entries[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *BlockPolicy) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Entries, len1)
	for i := 0; i < len1; i++ {
		if err := v.Entries[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *SnapshotPolicy) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.DeleteAfterTime)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint16(v.DeleteAfterVersions)); err != nil {
		return err
	}
	return nil
}

func (v *SnapshotPolicy) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.DeleteAfterTime)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint16)(&v.DeleteAfterVersions)); err != nil {
		return err
	}
	return nil
}

func (v *FullShardInfo) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastSeen)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	return nil
}

func (v *FullShardInfo) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastSeen)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	return nil
}

func (v *RegisterBlockServiceInfo) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.StorageClass)); err != nil {
		return err
	}
	if err := v.FailureDomain.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 16, v.SecretKey[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Flags)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.FlagsMask)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CapacityBytes)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.AvailableBytes)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Blocks)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Path)); err != nil {
		return err
	}
	return nil
}

func (v *RegisterBlockServiceInfo) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.StorageClass)); err != nil {
		return err
	}
	if err := v.FailureDomain.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 16, v.SecretKey[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Flags)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.FlagsMask)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CapacityBytes)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.AvailableBytes)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Blocks)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Path); err != nil {
		return err
	}
	return nil
}

func (v *FullBlockServiceInfo) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.StorageClass)); err != nil {
		return err
	}
	if err := v.FailureDomain.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 16, v.SecretKey[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Flags)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.CapacityBytes)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.AvailableBytes)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Blocks)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.FirstSeen)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastSeen)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastInfoChange)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.HasFiles)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Path)); err != nil {
		return err
	}
	return nil
}

func (v *FullBlockServiceInfo) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.StorageClass)); err != nil {
		return err
	}
	if err := v.FailureDomain.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 16, v.SecretKey[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Flags)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.CapacityBytes)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.AvailableBytes)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Blocks)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FirstSeen)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastSeen)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastInfoChange)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.HasFiles)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Path); err != nil {
		return err
	}
	return nil
}

func (v *CdcInfo) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.ReplicaId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastSeen)); err != nil {
		return err
	}
	return nil
}

func (v *CdcInfo) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.ReplicaId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastSeen)); err != nil {
		return err
	}
	return nil
}

func (v *LocationInfo) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	return nil
}

func (v *LocationInfo) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	return nil
}

func (v *IpPort) Pack(w io.Writer) error {
	if err := bincode.PackFixedBytes(w, 4, v.Addrs[:]); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint16(v.Port)); err != nil {
		return err
	}
	return nil
}

func (v *IpPort) Unpack(r io.Reader) error {
	if err := bincode.UnpackFixedBytes(r, 4, v.Addrs[:]); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Port)); err != nil {
		return err
	}
	return nil
}

func (v *AddrsInfo) Pack(w io.Writer) error {
	if err := v.Addr1.Pack(w); err != nil {
		return err
	}
	if err := v.Addr2.Pack(w); err != nil {
		return err
	}
	return nil
}

func (v *AddrsInfo) Unpack(r io.Reader) error {
	if err := v.Addr1.Unpack(r); err != nil {
		return err
	}
	if err := v.Addr2.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (v *LocalShardsReq) RegistryRequestKind() RegistryMessageKind {
	return LOCAL_SHARDS
}

func (v *LocalShardsReq) Pack(w io.Writer) error {
	return nil
}

func (v *LocalShardsReq) Unpack(r io.Reader) error {
	return nil
}

func (v *LocalShardsResp) RegistryResponseKind() RegistryMessageKind {
	return LOCAL_SHARDS
}

func (v *LocalShardsResp) Pack(w io.Writer) error {
	len1 := len(v.Shards)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Shards[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *LocalShardsResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Shards, len1)
	for i := 0; i < len1; i++ {
		if err := v.Shards[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *LocalCdcReq) RegistryRequestKind() RegistryMessageKind {
	return LOCAL_CDC
}

func (v *LocalCdcReq) Pack(w io.Writer) error {
	return nil
}

func (v *LocalCdcReq) Unpack(r io.Reader) error {
	return nil
}

func (v *LocalCdcResp) RegistryResponseKind() RegistryMessageKind {
	return LOCAL_CDC
}

func (v *LocalCdcResp) Pack(w io.Writer) error {
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastSeen)); err != nil {
		return err
	}
	return nil
}

func (v *LocalCdcResp) Unpack(r io.Reader) error {
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastSeen)); err != nil {
		return err
	}
	return nil
}

func (v *InfoReq) RegistryRequestKind() RegistryMessageKind {
	return INFO
}

func (v *InfoReq) Pack(w io.Writer) error {
	return nil
}

func (v *InfoReq) Unpack(r io.Reader) error {
	return nil
}

func (v *InfoResp) RegistryResponseKind() RegistryMessageKind {
	return INFO
}

func (v *InfoResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint32(v.NumBlockServices)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.NumFailureDomains)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Capacity)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Available)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Blocks)); err != nil {
		return err
	}
	return nil
}

func (v *InfoResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint32)(&v.NumBlockServices)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.NumFailureDomains)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Capacity)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Available)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Blocks)); err != nil {
		return err
	}
	return nil
}

func (v *RegistryReq) RegistryRequestKind() RegistryMessageKind {
	return REGISTRY
}

func (v *RegistryReq) Pack(w io.Writer) error {
	return nil
}

func (v *RegistryReq) Unpack(r io.Reader) error {
	return nil
}

func (v *RegistryResp) RegistryResponseKind() RegistryMessageKind {
	return REGISTRY
}

func (v *RegistryResp) Pack(w io.Writer) error {
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	return nil
}

func (v *RegistryResp) Unpack(r io.Reader) error {
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (v *LocalChangedBlockServicesReq) RegistryRequestKind() RegistryMessageKind {
	return LOCAL_CHANGED_BLOCK_SERVICES
}

func (v *LocalChangedBlockServicesReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.ChangedSince)); err != nil {
		return err
	}
	return nil
}

func (v *LocalChangedBlockServicesReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ChangedSince)); err != nil {
		return err
	}
	return nil
}

func (v *LocalChangedBlockServicesResp) RegistryResponseKind() RegistryMessageKind {
	return LOCAL_CHANGED_BLOCK_SERVICES
}

func (v *LocalChangedBlockServicesResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.LastChange)); err != nil {
		return err
	}
	len1 := len(v.BlockServices)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *LocalChangedBlockServicesResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastChange)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.BlockServices, len1)
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *CreateLocationReq) RegistryRequestKind() RegistryMessageKind {
	return CREATE_LOCATION
}

func (v *CreateLocationReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	return nil
}

func (v *CreateLocationReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	return nil
}

func (v *CreateLocationResp) RegistryResponseKind() RegistryMessageKind {
	return CREATE_LOCATION
}

func (v *CreateLocationResp) Pack(w io.Writer) error {
	return nil
}

func (v *CreateLocationResp) Unpack(r io.Reader) error {
	return nil
}

func (v *RenameLocationReq) RegistryRequestKind() RegistryMessageKind {
	return RENAME_LOCATION
}

func (v *RenameLocationReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.Name)); err != nil {
		return err
	}
	return nil
}

func (v *RenameLocationReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.Name); err != nil {
		return err
	}
	return nil
}

func (v *RenameLocationResp) RegistryResponseKind() RegistryMessageKind {
	return RENAME_LOCATION
}

func (v *RenameLocationResp) Pack(w io.Writer) error {
	return nil
}

func (v *RenameLocationResp) Unpack(r io.Reader) error {
	return nil
}

func (v *RegisterShardReq) RegistryRequestKind() RegistryMessageKind {
	return REGISTER_SHARD
}

func (v *RegisterShardReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Shrid)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Location)); err != nil {
		return err
	}
	return nil
}

func (v *RegisterShardReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Shrid)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Location)); err != nil {
		return err
	}
	return nil
}

func (v *RegisterShardResp) RegistryResponseKind() RegistryMessageKind {
	return REGISTER_SHARD
}

func (v *RegisterShardResp) Pack(w io.Writer) error {
	return nil
}

func (v *RegisterShardResp) Unpack(r io.Reader) error {
	return nil
}

func (v *LocationsReq) RegistryRequestKind() RegistryMessageKind {
	return LOCATIONS
}

func (v *LocationsReq) Pack(w io.Writer) error {
	return nil
}

func (v *LocationsReq) Unpack(r io.Reader) error {
	return nil
}

func (v *LocationsResp) RegistryResponseKind() RegistryMessageKind {
	return LOCATIONS
}

func (v *LocationsResp) Pack(w io.Writer) error {
	len1 := len(v.Locations)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Locations[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *LocationsResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Locations, len1)
	for i := 0; i < len1; i++ {
		if err := v.Locations[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *RegisterCdcReq) RegistryRequestKind() RegistryMessageKind {
	return REGISTER_CDC
}

func (v *RegisterCdcReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.Replica)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Location)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	return nil
}

func (v *RegisterCdcReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Replica)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Location)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	return nil
}

func (v *RegisterCdcResp) RegistryResponseKind() RegistryMessageKind {
	return REGISTER_CDC
}

func (v *RegisterCdcResp) Pack(w io.Writer) error {
	return nil
}

func (v *RegisterCdcResp) Unpack(r io.Reader) error {
	return nil
}

func (v *SetBlockServiceFlagsReq) RegistryRequestKind() RegistryMessageKind {
	return SET_BLOCK_SERVICE_FLAGS
}

func (v *SetBlockServiceFlagsReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Flags)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.FlagsMask)); err != nil {
		return err
	}
	return nil
}

func (v *SetBlockServiceFlagsReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Flags)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.FlagsMask)); err != nil {
		return err
	}
	return nil
}

func (v *SetBlockServiceFlagsResp) RegistryResponseKind() RegistryMessageKind {
	return SET_BLOCK_SERVICE_FLAGS
}

func (v *SetBlockServiceFlagsResp) Pack(w io.Writer) error {
	return nil
}

func (v *SetBlockServiceFlagsResp) Unpack(r io.Reader) error {
	return nil
}

func (v *RegisterBlockServicesReq) RegistryRequestKind() RegistryMessageKind {
	return REGISTER_BLOCK_SERVICES
}

func (v *RegisterBlockServicesReq) Pack(w io.Writer) error {
	len1 := len(v.BlockServices)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *RegisterBlockServicesReq) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.BlockServices, len1)
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *RegisterBlockServicesResp) RegistryResponseKind() RegistryMessageKind {
	return REGISTER_BLOCK_SERVICES
}

func (v *RegisterBlockServicesResp) Pack(w io.Writer) error {
	return nil
}

func (v *RegisterBlockServicesResp) Unpack(r io.Reader) error {
	return nil
}

func (v *ChangedBlockServicesAtLocationReq) RegistryRequestKind() RegistryMessageKind {
	return CHANGED_BLOCK_SERVICES_AT_LOCATION
}

func (v *ChangedBlockServicesAtLocationReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ChangedSince)); err != nil {
		return err
	}
	return nil
}

func (v *ChangedBlockServicesAtLocationReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ChangedSince)); err != nil {
		return err
	}
	return nil
}

func (v *ChangedBlockServicesAtLocationResp) RegistryResponseKind() RegistryMessageKind {
	return CHANGED_BLOCK_SERVICES_AT_LOCATION
}

func (v *ChangedBlockServicesAtLocationResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.LastChange)); err != nil {
		return err
	}
	len1 := len(v.BlockServices)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *ChangedBlockServicesAtLocationResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastChange)); err != nil {
		return err
	}
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.BlockServices, len1)
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *ShardsAtLocationReq) RegistryRequestKind() RegistryMessageKind {
	return SHARDS_AT_LOCATION
}

func (v *ShardsAtLocationReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	return nil
}

func (v *ShardsAtLocationReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	return nil
}

func (v *ShardsAtLocationResp) RegistryResponseKind() RegistryMessageKind {
	return SHARDS_AT_LOCATION
}

func (v *ShardsAtLocationResp) Pack(w io.Writer) error {
	len1 := len(v.Shards)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Shards[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *ShardsAtLocationResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Shards, len1)
	for i := 0; i < len1; i++ {
		if err := v.Shards[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *CdcAtLocationReq) RegistryRequestKind() RegistryMessageKind {
	return CDC_AT_LOCATION
}

func (v *CdcAtLocationReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.LocationId)); err != nil {
		return err
	}
	return nil
}

func (v *CdcAtLocationReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.LocationId)); err != nil {
		return err
	}
	return nil
}

func (v *CdcAtLocationResp) RegistryResponseKind() RegistryMessageKind {
	return CDC_AT_LOCATION
}

func (v *CdcAtLocationResp) Pack(w io.Writer) error {
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastSeen)); err != nil {
		return err
	}
	return nil
}

func (v *CdcAtLocationResp) Unpack(r io.Reader) error {
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastSeen)); err != nil {
		return err
	}
	return nil
}

func (v *RegisterRegistryReq) RegistryRequestKind() RegistryMessageKind {
	return REGISTER_REGISTRY
}

func (v *RegisterRegistryReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.ReplicaId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Location)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Pack(w); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, bool(v.Bootstrap)); err != nil {
		return err
	}
	return nil
}

func (v *RegisterRegistryReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.ReplicaId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Location)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.IsLeader)); err != nil {
		return err
	}
	if err := v.Addrs.Unpack(r); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*bool)(&v.Bootstrap)); err != nil {
		return err
	}
	return nil
}

func (v *RegisterRegistryResp) RegistryResponseKind() RegistryMessageKind {
	return REGISTER_REGISTRY
}

func (v *RegisterRegistryResp) Pack(w io.Writer) error {
	return nil
}

func (v *RegisterRegistryResp) Unpack(r io.Reader) error {
	return nil
}

func (v *AllRegistryReplicasReq) RegistryRequestKind() RegistryMessageKind {
	return ALL_REGISTRY_REPLICAS
}

func (v *AllRegistryReplicasReq) Pack(w io.Writer) error {
	return nil
}

func (v *AllRegistryReplicasReq) Unpack(r io.Reader) error {
	return nil
}

func (v *AllRegistryReplicasResp) RegistryResponseKind() RegistryMessageKind {
	return ALL_REGISTRY_REPLICAS
}

func (v *AllRegistryReplicasResp) Pack(w io.Writer) error {
	len1 := len(v.Replicas)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Replicas[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *AllRegistryReplicasResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Replicas, len1)
	for i := 0; i < len1; i++ {
		if err := v.Replicas[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *ShardBlockServicesDEPRECATEDReq) RegistryRequestKind() RegistryMessageKind {
	return SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED
}

func (v *ShardBlockServicesDEPRECATEDReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.ShardId)); err != nil {
		return err
	}
	return nil
}

func (v *ShardBlockServicesDEPRECATEDReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.ShardId)); err != nil {
		return err
	}
	return nil
}

func (v *ShardBlockServicesDEPRECATEDResp) RegistryResponseKind() RegistryMessageKind {
	return SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED
}

func (v *ShardBlockServicesDEPRECATEDResp) Pack(w io.Writer) error {
	len1 := len(v.BlockServices)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := bincode.PackScalar(w, uint64(v.BlockServices[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *ShardBlockServicesDEPRECATEDResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.BlockServices, len1)
	for i := 0; i < len1; i++ {
		if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockServices[i])); err != nil {
			return err
		}
	}
	return nil
}

func (v *CdcReplicasDEPRECATEDReq) RegistryRequestKind() RegistryMessageKind {
	return CDC_REPLICAS_DE_PR_EC_AT_ED
}

func (v *CdcReplicasDEPRECATEDReq) Pack(w io.Writer) error {
	return nil
}

func (v *CdcReplicasDEPRECATEDReq) Unpack(r io.Reader) error {
	return nil
}

func (v *CdcReplicasDEPRECATEDResp) RegistryResponseKind() RegistryMessageKind {
	return CDC_REPLICAS_DE_PR_EC_AT_ED
}

func (v *CdcReplicasDEPRECATEDResp) Pack(w io.Writer) error {
	len1 := len(v.Replicas)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Replicas[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *CdcReplicasDEPRECATEDResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Replicas, len1)
	for i := 0; i < len1; i++ {
		if err := v.Replicas[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *AllShardsReq) RegistryRequestKind() RegistryMessageKind {
	return ALL_SHARDS
}

func (v *AllShardsReq) Pack(w io.Writer) error {
	return nil
}

func (v *AllShardsReq) Unpack(r io.Reader) error {
	return nil
}

func (v *AllShardsResp) RegistryResponseKind() RegistryMessageKind {
	return ALL_SHARDS
}

func (v *AllShardsResp) Pack(w io.Writer) error {
	len1 := len(v.Shards)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Shards[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *AllShardsResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Shards, len1)
	for i := 0; i < len1; i++ {
		if err := v.Shards[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *DecommissionBlockServiceReq) RegistryRequestKind() RegistryMessageKind {
	return DECOMMISSION_BLOCK_SERVICE
}

func (v *DecommissionBlockServiceReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	return nil
}

func (v *DecommissionBlockServiceReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	return nil
}

func (v *DecommissionBlockServiceResp) RegistryResponseKind() RegistryMessageKind {
	return DECOMMISSION_BLOCK_SERVICE
}

func (v *DecommissionBlockServiceResp) Pack(w io.Writer) error {
	return nil
}

func (v *DecommissionBlockServiceResp) Unpack(r io.Reader) error {
	return nil
}

func (v *MoveShardLeaderReq) RegistryRequestKind() RegistryMessageKind {
	return MOVE_SHARD_LEADER
}

func (v *MoveShardLeaderReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Shrid)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Location)); err != nil {
		return err
	}
	return nil
}

func (v *MoveShardLeaderReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Shrid)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Location)); err != nil {
		return err
	}
	return nil
}

func (v *MoveShardLeaderResp) RegistryResponseKind() RegistryMessageKind {
	return MOVE_SHARD_LEADER
}

func (v *MoveShardLeaderResp) Pack(w io.Writer) error {
	return nil
}

func (v *MoveShardLeaderResp) Unpack(r io.Reader) error {
	return nil
}

func (v *ClearShardInfoReq) RegistryRequestKind() RegistryMessageKind {
	return CLEAR_SHARD_INFO
}

func (v *ClearShardInfoReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Shrid)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Location)); err != nil {
		return err
	}
	return nil
}

func (v *ClearShardInfoReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Shrid)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Location)); err != nil {
		return err
	}
	return nil
}

func (v *ClearShardInfoResp) RegistryResponseKind() RegistryMessageKind {
	return CLEAR_SHARD_INFO
}

func (v *ClearShardInfoResp) Pack(w io.Writer) error {
	return nil
}

func (v *ClearShardInfoResp) Unpack(r io.Reader) error {
	return nil
}

func (v *ShardBlockServicesReq) RegistryRequestKind() RegistryMessageKind {
	return SHARD_BLOCK_SERVICES
}

func (v *ShardBlockServicesReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.ShardId)); err != nil {
		return err
	}
	return nil
}

func (v *ShardBlockServicesReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.ShardId)); err != nil {
		return err
	}
	return nil
}

func (v *ShardBlockServicesResp) RegistryResponseKind() RegistryMessageKind {
	return SHARD_BLOCK_SERVICES
}

func (v *ShardBlockServicesResp) Pack(w io.Writer) error {
	len1 := len(v.BlockServices)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *ShardBlockServicesResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.BlockServices, len1)
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *AllCdcReq) RegistryRequestKind() RegistryMessageKind {
	return ALL_CDC
}

func (v *AllCdcReq) Pack(w io.Writer) error {
	return nil
}

func (v *AllCdcReq) Unpack(r io.Reader) error {
	return nil
}

func (v *AllCdcResp) RegistryResponseKind() RegistryMessageKind {
	return ALL_CDC
}

func (v *AllCdcResp) Pack(w io.Writer) error {
	len1 := len(v.Replicas)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.Replicas[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *AllCdcResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.Replicas, len1)
	for i := 0; i < len1; i++ {
		if err := v.Replicas[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *EraseDecommissionedBlockReq) RegistryRequestKind() RegistryMessageKind {
	return ERASE_DECOMMISSIONED_BLOCK
}

func (v *EraseDecommissionedBlockReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BlockServiceId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.BlockId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Certificate[:]); err != nil {
		return err
	}
	return nil
}

func (v *EraseDecommissionedBlockReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockServiceId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Certificate[:]); err != nil {
		return err
	}
	return nil
}

func (v *EraseDecommissionedBlockResp) RegistryResponseKind() RegistryMessageKind {
	return ERASE_DECOMMISSIONED_BLOCK
}

func (v *EraseDecommissionedBlockResp) Pack(w io.Writer) error {
	if err := bincode.PackFixedBytes(w, 8, v.Proof[:]); err != nil {
		return err
	}
	return nil
}

func (v *EraseDecommissionedBlockResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackFixedBytes(r, 8, v.Proof[:]); err != nil {
		return err
	}
	return nil
}

func (v *AllBlockServicesDeprecatedReq) RegistryRequestKind() RegistryMessageKind {
	return ALL_BLOCK_SERVICES_DEPRECATED
}

func (v *AllBlockServicesDeprecatedReq) Pack(w io.Writer) error {
	return nil
}

func (v *AllBlockServicesDeprecatedReq) Unpack(r io.Reader) error {
	return nil
}

func (v *AllBlockServicesDeprecatedResp) RegistryResponseKind() RegistryMessageKind {
	return ALL_BLOCK_SERVICES_DEPRECATED
}

func (v *AllBlockServicesDeprecatedResp) Pack(w io.Writer) error {
	len1 := len(v.BlockServices)
	if err := bincode.PackLength(w, len1); err != nil {
		return err
	}
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Pack(w); err != nil {
			return err
		}
	}
	return nil
}

func (v *AllBlockServicesDeprecatedResp) Unpack(r io.Reader) error {
	var len1 int
	if err := bincode.UnpackLength(r, &len1); err != nil {
		return err
	}
	bincode.EnsureLength(&v.BlockServices, len1)
	for i := 0; i < len1; i++ {
		if err := v.BlockServices[i].Unpack(r); err != nil {
			return err
		}
	}
	return nil
}

func (v *MoveCdcLeaderReq) RegistryRequestKind() RegistryMessageKind {
	return MOVE_CDC_LEADER
}

func (v *MoveCdcLeaderReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.Replica)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Location)); err != nil {
		return err
	}
	return nil
}

func (v *MoveCdcLeaderReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Replica)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Location)); err != nil {
		return err
	}
	return nil
}

func (v *MoveCdcLeaderResp) RegistryResponseKind() RegistryMessageKind {
	return MOVE_CDC_LEADER
}

func (v *MoveCdcLeaderResp) Pack(w io.Writer) error {
	return nil
}

func (v *MoveCdcLeaderResp) Unpack(r io.Reader) error {
	return nil
}

func (v *ClearCdcInfoReq) RegistryRequestKind() RegistryMessageKind {
	return CLEAR_CDC_INFO
}

func (v *ClearCdcInfoReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint8(v.Replica)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint8(v.Location)); err != nil {
		return err
	}
	return nil
}

func (v *ClearCdcInfoReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Replica)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint8)(&v.Location)); err != nil {
		return err
	}
	return nil
}

func (v *ClearCdcInfoResp) RegistryResponseKind() RegistryMessageKind {
	return CLEAR_CDC_INFO
}

func (v *ClearCdcInfoResp) Pack(w io.Writer) error {
	return nil
}

func (v *ClearCdcInfoResp) Unpack(r io.Reader) error {
	return nil
}

func (v *UpdateBlockServicePathReq) RegistryRequestKind() RegistryMessageKind {
	return UPDATE_BLOCK_SERVICE_PATH
}

func (v *UpdateBlockServicePathReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Id)); err != nil {
		return err
	}
	if err := bincode.PackBytes(w, []byte(v.NewPath)); err != nil {
		return err
	}
	return nil
}

func (v *UpdateBlockServicePathReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Id)); err != nil {
		return err
	}
	if err := bincode.UnpackString(r, &v.NewPath); err != nil {
		return err
	}
	return nil
}

func (v *UpdateBlockServicePathResp) RegistryResponseKind() RegistryMessageKind {
	return UPDATE_BLOCK_SERVICE_PATH
}

func (v *UpdateBlockServicePathResp) Pack(w io.Writer) error {
	return nil
}

func (v *UpdateBlockServicePathResp) Unpack(r io.Reader) error {
	return nil
}

func (v *FetchBlockReq) BlocksRequestKind() BlocksMessageKind {
	return FETCH_BLOCK
}

func (v *FetchBlockReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BlockId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Offset)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Count)); err != nil {
		return err
	}
	return nil
}

func (v *FetchBlockReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Offset)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Count)); err != nil {
		return err
	}
	return nil
}

func (v *FetchBlockResp) BlocksResponseKind() BlocksMessageKind {
	return FETCH_BLOCK
}

func (v *FetchBlockResp) Pack(w io.Writer) error {
	return nil
}

func (v *FetchBlockResp) Unpack(r io.Reader) error {
	return nil
}

func (v *WriteBlockReq) BlocksRequestKind() BlocksMessageKind {
	return WRITE_BLOCK
}

func (v *WriteBlockReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BlockId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Crc)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Size)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Certificate[:]); err != nil {
		return err
	}
	return nil
}

func (v *WriteBlockReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Crc)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Size)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Certificate[:]); err != nil {
		return err
	}
	return nil
}

func (v *WriteBlockResp) BlocksResponseKind() BlocksMessageKind {
	return WRITE_BLOCK
}

func (v *WriteBlockResp) Pack(w io.Writer) error {
	if err := bincode.PackFixedBytes(w, 8, v.Proof[:]); err != nil {
		return err
	}
	return nil
}

func (v *WriteBlockResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackFixedBytes(r, 8, v.Proof[:]); err != nil {
		return err
	}
	return nil
}

func (v *FetchBlockWithCrcReq) BlocksRequestKind() BlocksMessageKind {
	return FETCH_BLOCK_WITH_CRC
}

func (v *FetchBlockWithCrcReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.FileId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.BlockId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.BlockCrc)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Offset)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Count)); err != nil {
		return err
	}
	return nil
}

func (v *FetchBlockWithCrcReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.FileId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.BlockCrc)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Offset)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Count)); err != nil {
		return err
	}
	return nil
}

func (v *FetchBlockWithCrcResp) BlocksResponseKind() BlocksMessageKind {
	return FETCH_BLOCK_WITH_CRC
}

func (v *FetchBlockWithCrcResp) Pack(w io.Writer) error {
	return nil
}

func (v *FetchBlockWithCrcResp) Unpack(r io.Reader) error {
	return nil
}

func (v *EraseBlockReq) BlocksRequestKind() BlocksMessageKind {
	return ERASE_BLOCK
}

func (v *EraseBlockReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BlockId)); err != nil {
		return err
	}
	if err := bincode.PackFixedBytes(w, 8, v.Certificate[:]); err != nil {
		return err
	}
	return nil
}

func (v *EraseBlockReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId)); err != nil {
		return err
	}
	if err := bincode.UnpackFixedBytes(r, 8, v.Certificate[:]); err != nil {
		return err
	}
	return nil
}

func (v *EraseBlockResp) BlocksResponseKind() BlocksMessageKind {
	return ERASE_BLOCK
}

func (v *EraseBlockResp) Pack(w io.Writer) error {
	if err := bincode.PackFixedBytes(w, 8, v.Proof[:]); err != nil {
		return err
	}
	return nil
}

func (v *EraseBlockResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackFixedBytes(r, 8, v.Proof[:]); err != nil {
		return err
	}
	return nil
}

func (v *TestWriteReq) BlocksRequestKind() BlocksMessageKind {
	return TEST_WRITE
}

func (v *TestWriteReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Size)); err != nil {
		return err
	}
	return nil
}

func (v *TestWriteReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Size)); err != nil {
		return err
	}
	return nil
}

func (v *TestWriteResp) BlocksResponseKind() BlocksMessageKind {
	return TEST_WRITE
}

func (v *TestWriteResp) Pack(w io.Writer) error {
	return nil
}

func (v *TestWriteResp) Unpack(r io.Reader) error {
	return nil
}

func (v *CheckBlockReq) BlocksRequestKind() BlocksMessageKind {
	return CHECK_BLOCK
}

func (v *CheckBlockReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.BlockId)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Size)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint32(v.Crc)); err != nil {
		return err
	}
	return nil
}

func (v *CheckBlockReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.BlockId)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Size)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint32)(&v.Crc)); err != nil {
		return err
	}
	return nil
}

func (v *CheckBlockResp) BlocksResponseKind() BlocksMessageKind {
	return CHECK_BLOCK
}

func (v *CheckBlockResp) Pack(w io.Writer) error {
	return nil
}

func (v *CheckBlockResp) Unpack(r io.Reader) error {
	return nil
}

func (v *LogWriteReq) LogRequestKind() LogMessageKind {
	return LOG_WRITE
}

func (v *LogWriteReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Token)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastReleased)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Idx)); err != nil {
		return err
	}
	if err := bincode.PackBlob(w, v.Value); err != nil {
		return err
	}
	return nil
}

func (v *LogWriteReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Token)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastReleased)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Idx)); err != nil {
		return err
	}
	if err := bincode.UnpackBlob(r, &v.Value); err != nil {
		return err
	}
	return nil
}

func (v *LogWriteResp) LogResponseKind() LogMessageKind {
	return LOG_WRITE
}

func (v *LogWriteResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Result)); err != nil {
		return err
	}
	return nil
}

func (v *LogWriteResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Result)); err != nil {
		return err
	}
	return nil
}

func (v *ReleaseReq) LogRequestKind() LogMessageKind {
	return RELEASE
}

func (v *ReleaseReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Token)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastReleased)); err != nil {
		return err
	}
	return nil
}

func (v *ReleaseReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Token)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastReleased)); err != nil {
		return err
	}
	return nil
}

func (v *ReleaseResp) LogResponseKind() LogMessageKind {
	return RELEASE
}

func (v *ReleaseResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Result)); err != nil {
		return err
	}
	return nil
}

func (v *ReleaseResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Result)); err != nil {
		return err
	}
	return nil
}

func (v *LogReadReq) LogRequestKind() LogMessageKind {
	return LOG_READ
}

func (v *LogReadReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.Idx)); err != nil {
		return err
	}
	return nil
}

func (v *LogReadReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Idx)); err != nil {
		return err
	}
	return nil
}

func (v *LogReadResp) LogResponseKind() LogMessageKind {
	return LOG_READ
}

func (v *LogReadResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Result)); err != nil {
		return err
	}
	if err := bincode.PackBlob(w, v.Value); err != nil {
		return err
	}
	return nil
}

func (v *LogReadResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Result)); err != nil {
		return err
	}
	if err := bincode.UnpackBlob(r, &v.Value); err != nil {
		return err
	}
	return nil
}

func (v *NewLeaderReq) LogRequestKind() LogMessageKind {
	return NEW_LEADER
}

func (v *NewLeaderReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NomineeToken)); err != nil {
		return err
	}
	return nil
}

func (v *NewLeaderReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NomineeToken)); err != nil {
		return err
	}
	return nil
}

func (v *NewLeaderResp) LogResponseKind() LogMessageKind {
	return NEW_LEADER
}

func (v *NewLeaderResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Result)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.LastReleased)); err != nil {
		return err
	}
	return nil
}

func (v *NewLeaderResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Result)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.LastReleased)); err != nil {
		return err
	}
	return nil
}

func (v *NewLeaderConfirmReq) LogRequestKind() LogMessageKind {
	return NEW_LEADER_CONFIRM
}

func (v *NewLeaderConfirmReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NomineeToken)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.ReleasedIdx)); err != nil {
		return err
	}
	return nil
}

func (v *NewLeaderConfirmReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NomineeToken)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.ReleasedIdx)); err != nil {
		return err
	}
	return nil
}

func (v *NewLeaderConfirmResp) LogResponseKind() LogMessageKind {
	return NEW_LEADER_CONFIRM
}

func (v *NewLeaderConfirmResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Result)); err != nil {
		return err
	}
	return nil
}

func (v *NewLeaderConfirmResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Result)); err != nil {
		return err
	}
	return nil
}

func (v *LogRecoveryReadReq) LogRequestKind() LogMessageKind {
	return LOG_RECOVERY_READ
}

func (v *LogRecoveryReadReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NomineeToken)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Idx)); err != nil {
		return err
	}
	return nil
}

func (v *LogRecoveryReadReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NomineeToken)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Idx)); err != nil {
		return err
	}
	return nil
}

func (v *LogRecoveryReadResp) LogResponseKind() LogMessageKind {
	return LOG_RECOVERY_READ
}

func (v *LogRecoveryReadResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Result)); err != nil {
		return err
	}
	if err := bincode.PackBlob(w, v.Value); err != nil {
		return err
	}
	return nil
}

func (v *LogRecoveryReadResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Result)); err != nil {
		return err
	}
	if err := bincode.UnpackBlob(r, &v.Value); err != nil {
		return err
	}
	return nil
}

func (v *LogRecoveryWriteReq) LogRequestKind() LogMessageKind {
	return LOG_RECOVERY_WRITE
}

func (v *LogRecoveryWriteReq) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint64(v.NomineeToken)); err != nil {
		return err
	}
	if err := bincode.PackScalar(w, uint64(v.Idx)); err != nil {
		return err
	}
	if err := bincode.PackBlob(w, v.Value); err != nil {
		return err
	}
	return nil
}

func (v *LogRecoveryWriteReq) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint64)(&v.NomineeToken)); err != nil {
		return err
	}
	if err := bincode.UnpackScalar(r, (*uint64)(&v.Idx)); err != nil {
		return err
	}
	if err := bincode.UnpackBlob(r, &v.Value); err != nil {
		return err
	}
	return nil
}

func (v *LogRecoveryWriteResp) LogResponseKind() LogMessageKind {
	return LOG_RECOVERY_WRITE
}

func (v *LogRecoveryWriteResp) Pack(w io.Writer) error {
	if err := bincode.PackScalar(w, uint16(v.Result)); err != nil {
		return err
	}
	return nil
}

func (v *LogRecoveryWriteResp) Unpack(r io.Reader) error {
	if err := bincode.UnpackScalar(r, (*uint16)(&v.Result)); err != nil {
		return err
	}
	return nil
}
