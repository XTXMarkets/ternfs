// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// See bincodegen.go for more comments on how we split op these operations
package msgs

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"
	"xtx/ternfs/core/bincode"
	"xtx/ternfs/core/parity"
)

//go:generate go run ../bincodegen

// --------------------------------------------------------------------
// Common types and utilities

const DEFAULT_UDP_MTU = 1472
const MAX_UDP_MTU = 8972

type InodeType uint8
type InodeId uint64
type InodeIdExtra uint64 // 64th bit is used to mark whether the inode is owned.
type ShardId uint8
type ReplicaId uint8
type ShardReplicaId uint16
type StorageClass uint8
type TernTime uint64
type BlockId uint64
type BlockServiceId uint64
type BlockServiceFlags uint8
type Crc uint32
type NameHash uint64
type Cookie [8]byte
type LogIdx uint64
type LeaderToken uint64
type Ip [4]byte
type Location uint8

// These four below are the magic number to identify UDP packets. After a three-letter
// string identifying the service we have a version number. The idea is that when the
// version number increases you increase both req and resp. Different for req and resp
// so that middleware can understand packets just by looking at their contents.

// >>> format(struct.unpack('<I', b'SHA\0')[0], 'x')
// '414853'
const SHARD_REQ_PROTOCOL_VERSION uint32 = 0x414853

// >>> format(struct.unpack('<I', b'SHA\1')[0], 'x')
// '1414853'
const SHARD_RESP_PROTOCOL_VERSION uint32 = 0x1414853

// >>> format(struct.unpack('<I', b'CDC\0')[0], 'x')
// '434443'
const CDC_REQ_PROTOCOL_VERSION uint32 = 0x434443

// >>> format(struct.unpack('<I', b'CDC\1')[0], 'x')
// '1434443'
const CDC_RESP_PROTOCOL_VERSION uint32 = 0x1434443

// >>> format(struct.unpack('<I', b'SHU\0')[0], 'x')
// '554853'
const REGISTRY_REQ_PROTOCOL_VERSION uint32 = 0x554853

// >>> format(struct.unpack('<I', b'SHU\1')[0], 'x')
// '1554853'
const REGISTRY_RESP_PROTOCOL_VERSION uint32 = 0x1554853

// >>> format(struct.unpack('<I', b'BLO\0')[0], 'x')
// '4f4c42'
const BLOCKS_REQ_PROTOCOL_VERSION uint32 = 0x4f4c42

// >>> format(struct.unpack('<I', b'BLO\1')[0], 'x')
// '14f4c42'
const BLOCKS_RESP_PROTOCOL_VERSION uint32 = 0x14f4c42

// >>> format(struct.unpack('<I', b'LOG\0')[0], 'x')
// '474f4c'
const LOG_REQ_PROTOCOL_VERSION uint32 = 0x474f4c

// >>> format(struct.unpack('<I', b'LOG\1')[0], 'x')
// '1474f4c'
const LOG_RESP_PROTOCOL_VERSION uint32 = 0x1474f4c

// For CDC/SHARD we use 0 as an error kind
const ERROR_KIND uint8 = 0

const (
	TERNFS_BLOCK_SERVICE_EMPTY          BlockServiceFlags = 0x0
	TERNFS_BLOCK_SERVICE_STALE          BlockServiceFlags = 0x1
	TERNFS_BLOCK_SERVICE_NO_READ        BlockServiceFlags = 0x2
	TERNFS_BLOCK_SERVICE_NO_WRITE       BlockServiceFlags = 0x4
	TERNFS_BLOCK_SERVICE_DECOMMISSIONED BlockServiceFlags = 0x8
	TERNFS_BLOCK_SERVICE_MASK_ALL                         = 0xf
)

const (
	TERN_PAGE_SIZE          uint32 = 1 << 12 // 4KB
	TERN_PAGE_WITH_CRC_SIZE uint32 = TERN_PAGE_SIZE + 4
)

func BlockServiceFlagFromName(n string) (BlockServiceFlags, error) {
	switch n {
	case "0":
		return TERNFS_BLOCK_SERVICE_EMPTY, nil
	case "STALE":
		return TERNFS_BLOCK_SERVICE_STALE, nil
	case "NO_READ":
		return TERNFS_BLOCK_SERVICE_NO_READ, nil
	case "NO_WRITE":
		return TERNFS_BLOCK_SERVICE_NO_WRITE, nil
	case "DECOMMISSIONED":
		return TERNFS_BLOCK_SERVICE_DECOMMISSIONED, nil
	default:
		panic(fmt.Errorf("unknown blockservice flag %q", n))
	}
}

func BlockServiceFlagsFromUnion(n string) (BlockServiceFlags, error) {
	var flags BlockServiceFlags
	for _, fs := range strings.Split(n, "|") {
		f, err := BlockServiceFlagFromName(fs)
		if err != nil {
			return 0, err
		}
		flags |= f
	}
	return flags, nil
}

func (flags BlockServiceFlags) HasAny(f BlockServiceFlags) bool {
	return flags&f != 0
}

func (flags BlockServiceFlags) String() string {
	if flags == 0 {
		return "0"
	}
	var ret []string
	if flags&TERNFS_BLOCK_SERVICE_STALE != 0 {
		ret = append(ret, "STALE")
	}
	if flags&TERNFS_BLOCK_SERVICE_NO_READ != 0 {
		ret = append(ret, "NO_READ")
	}
	if flags&TERNFS_BLOCK_SERVICE_NO_WRITE != 0 {
		ret = append(ret, "NO_WRITE")
	}
	if flags&TERNFS_BLOCK_SERVICE_DECOMMISSIONED != 0 {
		ret = append(ret, "DECOMMISSIONED")
	}
	return strings.Join(ret, "|")
}

func (flags BlockServiceFlags) ShortString() string {
	if flags == 0 {
		return "0"
	}
	var ret []string
	if flags&TERNFS_BLOCK_SERVICE_STALE != 0 {
		ret = append(ret, "S")
	}
	if flags&TERNFS_BLOCK_SERVICE_NO_READ != 0 {
		ret = append(ret, "NR")
	}
	if flags&TERNFS_BLOCK_SERVICE_NO_WRITE != 0 {
		ret = append(ret, "NW")
	}
	if flags&TERNFS_BLOCK_SERVICE_DECOMMISSIONED != 0 {
		ret = append(ret, "D")
	}
	return strings.Join(ret, "|")
}

func (flags BlockServiceFlags) CanRead() bool {
	return (flags & (TERNFS_BLOCK_SERVICE_STALE | TERNFS_BLOCK_SERVICE_NO_READ | TERNFS_BLOCK_SERVICE_DECOMMISSIONED)) == 0
}

func (flags BlockServiceFlags) CanWrite() bool {
	return (flags & (TERNFS_BLOCK_SERVICE_STALE | TERNFS_BLOCK_SERVICE_NO_WRITE | TERNFS_BLOCK_SERVICE_DECOMMISSIONED)) == 0
}

const (
	DIRECTORY InodeType = 1
	FILE      InodeType = 2
	SYMLINK   InodeType = 3
)

func (typ InodeType) String() string {
	switch typ {
	case DIRECTORY:
		return "DIRECTORY"
	case FILE:
		return "FILE"
	case SYMLINK:
		return "SYMLINK"
	default:
		return fmt.Sprintf("InodeType(%d)", uint8(typ))
	}
}

func (id InodeId) Type() InodeType {
	typ := InodeType((id >> 61) & 0x03)
	if !(typ == DIRECTORY || typ == FILE || typ == SYMLINK) {
		panic(fmt.Errorf("bad inode type %v -- are you calling Type() on NULL_INODE_ID?", typ))
	}
	return typ
}

func (id InodeId) Shard() ShardId {
	return ShardId(id & 0xFF)
}

func (id InodeId) String() string {
	return fmt.Sprintf("0x%016x", uint64(id))
}

func marshalJSONId(id uint64) ([]byte, error) {
	return []byte(fmt.Sprintf("\"0x%016x\"", uint64(id))), nil
}

func unmarshalJSONId(b []byte) (uint64, error) {
	var ids string
	if err := json.Unmarshal(b, &ids); err != nil {
		return 0, err
	}
	idu, err := strconv.ParseUint(ids, 0, 63)
	if err != nil {
		return 0, err
	}
	return idu, nil
}

func (id InodeId) MarshalJSON() ([]byte, error) {
	return marshalJSONId(uint64(id))
}

func (id *InodeId) UnmarshalJSON(b []byte) error {
	idu, err := unmarshalJSONId(b)
	if err != nil {
		return err
	}
	*id = InodeId(idu)
	return nil
}

func MakeShardReplicaId(shard ShardId, replica ReplicaId) ShardReplicaId {
	return ShardReplicaId((uint16(replica) << 8) | uint16(shard))
}

func (id ShardReplicaId) Shard() ShardId {
	return ShardId(id & 0xFF)
}

func (id ShardReplicaId) Replica() ReplicaId {
	return ReplicaId(id >> 8)
}

func (id ShardReplicaId) String() string {
	return fmt.Sprintf("%v:%v", id.Shard(), id.Replica())
}

func (id ShardReplicaId) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", id.String())), nil
}

func (id *ShardReplicaId) UnmarshalJSON(b []byte) error {
	var shardReplica string
	err := json.Unmarshal(b, &shardReplica)
	if err != nil {
		return err
	}

	tokens := strings.Split(shardReplica, ":")
	if len(tokens) != 2 {
		return fmt.Errorf("can not parse ShardReplicaId(shardId:replicaId) %s", shardReplica)
	}

	shardId, err := strconv.ParseUint(tokens[0], 10, 8)
	if err != nil {
		return err
	}

	replicaId, err := strconv.ParseUint(tokens[1], 10, 8)
	if err != nil {
		return err
	}

	*id = MakeShardReplicaId(ShardId(shardId), ReplicaId(replicaId))
	return nil
}

func (id Cookie) MarshalJSON() ([]byte, error) {
	idB := [8]byte(id)
	return marshalJSONId(binary.LittleEndian.Uint64(idB[:]))
}

func (id *Cookie) UnmarshalJSON(b []byte) error {
	idu, err := unmarshalJSONId(b)
	if err != nil {
		return err
	}
	var idB [8]byte
	binary.LittleEndian.PutUint64(idB[:], idu)
	*id = Cookie(idB)
	return nil
}

func (ip Ip) String() string {
	return fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])
}

func (ip Ip) MarshalJSON() ([]byte, error) {
	return []byte(ip.String()), nil
}

func (ip *Ip) UnmarshalJSON(b []byte) error {
	var ipStr string
	err := json.Unmarshal(b, &ipStr)
	if err != nil {
		return err
	}

	tokens := strings.Split(ipStr, ".")
	if len(tokens) != 4 {
		return fmt.Errorf("can not parse ipv4 %s", ipStr)
	}
	for i, token := range tokens {
		val, err := strconv.ParseUint(token, 10, 8)
		if err != nil {
			return err
		}
		ip[i] = byte(val)
	}
	return nil
}

func marshalNameHash(id uint64) ([]byte, error) {
	return []byte(fmt.Sprintf("\"%016x\"", uint64(id))), nil
}

func unmarshalNameHash(b []byte) (uint64, error) {
	var ids string
	if err := json.Unmarshal(b, &ids); err != nil {
		return 0, err
	}
	idu, err := strconv.ParseUint(ids, 16, 64)
	if err != nil {
		return 0, err
	}
	return idu, nil
}

func (h NameHash) MarshalJSON() ([]byte, error) {
	return marshalNameHash(uint64(h))
}

func (h *NameHash) UnmarshalJSON(b []byte) error {
	idu, err := unmarshalNameHash(b)
	if err != nil {
		return err
	}
	*h = NameHash(idu)
	return nil
}

type inodeIdJson struct {
	Id    InodeId
	Extra bool
}

func (id InodeIdExtra) MarshalJSON() ([]byte, error) {
	return json.Marshal(&inodeIdJson{Id: id.Id(), Extra: id.Extra()})
}

func (id *InodeIdExtra) UnmarshalJSON(b []byte) error {
	idJ := &inodeIdJson{}
	if err := json.Unmarshal(b, idJ); err != nil {
		return err
	}
	*id = MakeInodeIdExtra(idJ.Id, idJ.Extra)
	return nil
}

func (id BlockId) MarshalJSON() ([]byte, error) {
	return marshalJSONId(uint64(id))
}

func (id *BlockId) UnmarshalJSON(b []byte) error {
	idu, err := unmarshalJSONId(b)
	if err != nil {
		return err
	}
	*id = BlockId(idu)
	return nil
}

func (id BlockServiceId) MarshalJSON() ([]byte, error) {
	return marshalJSONId(uint64(id))
}

func (id *BlockServiceId) UnmarshalJSON(b []byte) error {
	idu, err := unmarshalJSONId(b)
	if err != nil {
		return err
	}
	*id = BlockServiceId(idu)
	return nil
}

func (sclass *StorageClass) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*sclass = StorageClassFromString(s)
	return nil
}

func (id BlockServiceId) String() string {
	return fmt.Sprintf("0x%016x", uint64(id))
}

func (id BlockId) String() string {
	return fmt.Sprintf("0x%016x", uint64(id))
}

func (id BlockId) Path() string {
	hex := fmt.Sprintf("%016x", uint64(id))
	// We want to split the blocks in dirs to avoid trouble with high number of
	// files in single directory (e.g. birthday paradox stuff). However the block
	// id is very much _not_ uniformly distributed (it's the time). So we use
	// the first byte of the SHA1 of the filename of the block id.
	h := sha1.New()
	h.Write([]byte(hex))
	dir := fmt.Sprintf("%02x", h.Sum(nil)[0])
	return path.Join("with_crc", dir, hex)
}

func MakeInodeId(typ InodeType, shard ShardId, id uint64) InodeId {
	return (InodeId(typ) << 61) | (InodeId(id) << 8) | InodeId(shard)
}

func (id InodeIdExtra) Id() InodeId {
	return InodeId(uint64(id) & ^(uint64(1) << 63))
}

func (id InodeIdExtra) Extra() bool {
	if (uint64(id) >> 63) == 0 {
		return false
	} else {
		return true
	}
}

func (id InodeIdExtra) String() string {
	if id.Extra() {
		return fmt.Sprintf("[X]%v", id.Id())
	} else {
		return fmt.Sprintf("[ ]%v", id.Id())
	}
}

func MakeInodeIdExtra(id InodeId, owned bool) InodeIdExtra {
	x := uint64(id)
	if owned {
		x = x | (1 << 63)
	}
	return InodeIdExtra(x)
}

const (
	NULL_INODE_ID InodeId = 0
	// Can't call MakeInodeId in constant
	ROOT_DIR_INODE_ID = InodeId(DIRECTORY) << 61
)

func MakeTernTime(t time.Time) TernTime {
	return TernTime(uint64(t.UnixNano()))
}

func Now() TernTime {
	return MakeTernTime(time.Now())
}

func (t TernTime) Time() time.Time {
	return time.Unix(0, int64(uint64(t)))
}

func (t TernTime) String() string {
	return t.Time().Format(time.RFC3339Nano)
}

func (t TernTime) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", t.String())), nil
}

func (t *TernTime) UnmarshalJSON(b []byte) error {
	var ts string
	if err := json.Unmarshal(b, &ts); err != nil {
		return err
	}
	tt, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return err
	}
	*t = TernTime(tt.UnixNano())
	return nil
}

type IpPort struct {
	Addrs Ip
	Port  uint16
}

func (i IpPort) String() string {
	return fmt.Sprintf("%s:%d", i.Addrs.String(), i.Port)
}

func (i IpPort) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", i.String())), nil
}

func (i *IpPort) UnmarshalJSON(b []byte) error {
	var ts string
	if err := json.Unmarshal(b, &ts); err != nil {
		return err
	}
	tokens := strings.Split(ts, ":")
	if len(tokens) != 2 {
		return fmt.Errorf("can not parse ip:port (%s)", ts)
	}
	if err := i.Addrs.UnmarshalJSON([]byte(tokens[0])); err != nil {
		return err
	}
	val, err := strconv.ParseUint(tokens[1], 10, 16)
	if err != nil {
		return err
	}
	i.Port = uint16(val)
	return nil
}

type TernError uint16

type ShardMessageKind uint8

type CDCMessageKind uint8

type RegistryMessageKind uint8

type BlocksMessageKind uint8

type LogMessageKind uint8

type CtrlMessageKind uint8

const ERROR uint8 = 0

const (
	EMPTY_STORAGE  StorageClass = 0
	INLINE_STORAGE StorageClass = 1
	HDD_STORAGE    StorageClass = 2
	FLASH_STORAGE  StorageClass = 3
)

func StorageClassFromString(s string) StorageClass {
	switch s {
	case "EMPTY":
		return EMPTY_STORAGE
	case "INLINE":
		return INLINE_STORAGE
	case "HDD":
		return HDD_STORAGE
	case "FLASH":
		return FLASH_STORAGE
	default:
		panic(fmt.Errorf("bad storage class string '%v'", s))
	}
}

func (s StorageClass) String() string {
	switch s {
	case EMPTY_STORAGE:
		return "EMPTY"
	case INLINE_STORAGE:
		return "INLINE"
	case HDD_STORAGE:
		return "HDD"
	case FLASH_STORAGE:
		return "FLASH"
	default:
		return fmt.Sprintf("StorageClass(%v)", uint8(s))
	}
}

func (crc Crc) String() string {
	return fmt.Sprintf("%08x", uint32(crc))
}

func (crc *Crc) Unpack(r io.Reader) error {
	return bincode.UnpackScalar(r, (*uint32)(crc))
}

// --------------------------------------------------------------------
// Shard requests/responses

type ShardSnapshotReq struct {
	SnapshotId uint64
}

type ShardSnapshotResp struct {
}

// Given directory inode and name, returns inode from outgoing
// current edge. Does not consider non-current directories and
// non-current edges.
//
// Returns DIRECTORY_NOT_FOUND if the directory is snapshot.
type LookupReq struct {
	DirId InodeId
	Name  string
}

type LookupResp struct {
	TargetId     InodeId
	CreationTime TernTime
}

// Does not consider transient files. Might return snapshot files:
// we don't really have a way of knowing if a file is snapshot just by
// looking at it, unlike directories.
type StatFileReq struct {
	Id InodeId
}

type StatFileResp struct {
	Mtime TernTime
	Atime TernTime
	Size  uint64
}

type StatTransientFileReq struct {
	Id InodeId
}

type StatTransientFileResp struct {
	Mtime TernTime
	Size  uint64
	Note  string
}

// Considers all directories, also snapshot/transient ones. Remember that
// a transient directory is just a snapshot directory with no outgoing
// edges.
//
// The caller can detect if the directory is snapshot (unlike with files),
// and avoid calling `ReadDirReq`, which won't work.
type StatDirectoryReq struct {
	Id InodeId
}

type DirectoryInfoEntry struct {
	Tag  DirectoryInfoTag
	Body []byte
}

// The directory info is a bunch of tagged entries. If we want a tag
// for a directory, if we don't have it in the current directory we
// need to traverse upwards until we find it.
type DirectoryInfo struct {
	Entries []DirectoryInfoEntry
}

type StatDirectoryResp struct {
	Mtime TernTime
	Owner InodeId // if NULL_INODE_ID, the directory is currently snapshot
	Info  DirectoryInfo
}

// Does not consider snaphsot/transient directories, unlike `StatDirectoryReq`.
// This is since clients almost never want this, and when they do they're almost
// certainly better served by FullReadDirReq.
type ReadDirReq struct {
	DirId     InodeId
	StartHash NameHash
	// if 0, a conservative MTU will be used.
	Mtu uint16
}

type CurrentEdge struct {
	TargetId     InodeId
	NameHash     NameHash
	Name         string
	CreationTime TernTime
}

// Names with the same hash will never straddle two `ReadDirResp`s, assuming the
// directory contents don't change in the meantime.
type ReadDirResp struct {
	NextHash NameHash
	Results  []CurrentEdge
}

// create a new transient file.
type ConstructFileReq struct {
	Type InodeType // must not be DIRECTORY
	// this can be the future file name or anyway something that gives a pointer
	// to what this transient wile will be or was (if we're destructing).
	Note string
}

type ConstructFileResp struct {
	Id     InodeId
	Cookie Cookie
}

type AddInlineSpanReq struct {
	FileId       InodeId
	Cookie       Cookie
	StorageClass StorageClass // Either EMPTY_STORAGE or INLINE_STORAGE
	ByteOffset   uint64
	Size         uint32
	Crc          Crc
	Body         []byte
}

type AddInlineSpanResp struct{}

// in a separate type for the kernel to be able to define the decoding for this explicitly
type FailureDomain struct {
	Name [16]byte
}

func (fd *FailureDomain) String() string {
	return string(bytes.TrimRightFunc(fd.Name[:], func(r rune) bool {
		return r == 0
	}))
}

func (fd *FailureDomain) MarshalJSON() ([]byte, error) {
	return json.Marshal(fd.String())
}

func MkFailureDomain(str string) (*FailureDomain, error) {
	if len(str) > 16 {
		return nil, fmt.Errorf("failure domain string too long (%v)", len(str))
	}
	fd := &FailureDomain{Name: [16]byte{}}
	copy(fd.Name[:], []byte(str))
	return fd, nil
}

func (fd *FailureDomain) UnmarshalJSON(b []byte) error {
	var str string
	err := json.Unmarshal([]byte(b), &str)
	if err != nil {
		return err
	}
	parsed, err := MkFailureDomain(str)
	if err != nil {
		return err
	}
	fd.Name = parsed.Name
	return nil
}

// If any of these match, candidate block services will be excluded.
type BlacklistEntry struct {
	FailureDomain FailureDomain
	BlockService  BlockServiceId
}

// Add span. The file must be transient.
//
// Generally speaking, the num_data_blocks*BlockSize == Size. However, there are two
// exceptions.
//
//   - If the erasure code we use dictates the data to be a multiple of a certain number
//     of bytes, then the sum of the data block sizes might be larger than the span size
//     to ensure that, in which case the excess data must be zero and can be discarded.
//   - The span size can be greater than the sum of data blocks, in which case trailing
//     zeros are added. This is to support cheap creation of gaps in the file.
//
// The storage class must not be EMPTY_STORAGE or INLINE_STORAGE.
type AddSpanInitiateReq struct {
	FileId       InodeId
	Cookie       Cookie
	ByteOffset   uint64
	Size         uint32
	Crc          Crc
	StorageClass StorageClass
	// A single element of these matches if all the nonzero elements in it match.
	// This is useful when the kernel knows it cannot communicate with a certain block
	// service (because it noticed that it is broken before registry/shard did, or
	// because of some transient network problem, or...).
	Blacklist []BlacklistEntry
	Parity    parity.Parity
	Stripes   uint8 // [1, 15]
	CellSize  uint32
	// Stripes x Parity.Blocks() array with the CRCs for every cell -- row-major.
	Crcs []Crc
}

// Same as above but used internally to create span at specific location.
// Spans can not be created at multiple locations but can be merged to contain multiple locations.
type AddSpanAtLocationInitiateReq struct {
	LocationId Location
	Req        AddSpanInitiateWithReferenceReq
}

type AddSpanInitiateBlockInfo struct {
	BlockServiceAddrs         AddrsInfo
	BlockServiceId            BlockServiceId
	BlockServiceFailureDomain FailureDomain
	BlockId                   BlockId
	// certificate := MAC(b'w' + block_id + crc + size)[:8] (for creation)
	Certificate [8]byte
}

type AddSpanInitiateResp struct {
	// Left empty for inline/zero-filled spans
	Blocks []AddSpanInitiateBlockInfo
}

type AddSpanAtLocationInitiateResp struct {
	// Left empty for inline/zero-filled spans
	Resp AddSpanInitiateResp
}

// The only reason why this isn't the same as AddSpanInitiateReq is
// that we added it after we had deployed many clients.
type AddSpanInitiateWithReferenceReq struct {
	Req AddSpanInitiateReq
	// What inode to use to compute what block services to pick. If NULL, FileId
	// will be used. This is useful when migrating or defragmenting, where we're
	// adding a span to a file but we plan to merge it into another file.
	Reference InodeId
}

type AddSpanInitiateWithReferenceResp struct {
	Resp AddSpanInitiateResp
}

type BlockProof struct {
	BlockId BlockId
	Proof   [8]byte
}

// certify span. again, the file must be transient.
type AddSpanCertifyReq struct {
	FileId     InodeId
	Cookie     Cookie
	ByteOffset uint64
	Proofs     []BlockProof
}

type AddSpanCertifyResp struct{}

type RemoveSpanInitiateReq struct {
	FileId InodeId
	Cookie Cookie
}

type RemoveSpanInitiateBlockInfo struct {
	BlockServiceAddrs         AddrsInfo
	BlockServiceId            BlockServiceId
	BlockServiceFailureDomain FailureDomain
	BlockServiceFlags         BlockServiceFlags
	BlockId                   BlockId
	// certificate := MAC(b'w' + block_id + crc + size)[:8] (for creation)
	Certificate [8]byte
}

type RemoveSpanInitiateResp struct {
	ByteOffset uint64
	// If empty, this is a blockless span, and no certification is required
	Blocks []RemoveSpanInitiateBlockInfo
}

type RemoveSpanCertifyReq struct {
	FileId     InodeId
	Cookie     Cookie
	ByteOffset uint64
	Proofs     []BlockProof
}

type RemoveSpanCertifyResp struct{}

// Makes a transient file current. Requires the inode, the
// parent dir, and the filename.
//
// If the file is not transient, but does exist, this will return
// normally. This is not strictly necessary but simplifies writing
// the clients (since they) need to retry.
//
// An expired transient file is considered gone.
type LinkFileReq struct {
	FileId  InodeId
	Cookie  Cookie
	OwnerId InodeId
	Name    string
}

type LinkFileResp struct {
	CreationTime TernTime
}

// turns a current outgoing edge into a snapshot owning edge.
type SoftUnlinkFileReq struct {
	OwnerId InodeId
	FileId  InodeId
	Name    string
	// See comment in `SameDirectoryRenameReq` for an idication of why
	// we have this here even if it's not strictly needed.
	CreationTime TernTime
}

type SoftUnlinkFileResp struct {
	DeleteCreationTime TernTime // the creation time of the newly created delete edge
}

// Starts from the first span with byte offset <= than the provided
// ByteOffset (this is so that you can just start reading a file
// somewhere without hunting for the right span).
type LocalFileSpansReq struct {
	FileId     InodeId
	ByteOffset uint64
	// if 0, no limit.
	Limit uint32
	// if  0, a conservative MTU will be used
	Mtu uint16
}

type FetchedBlock struct {
	BlockServiceIx uint8 // Index into `BlockServices`
	BlockId        BlockId
	Crc            Crc
}

type FetchedInlineSpan struct {
	Body []byte
}

func (f *FetchedInlineSpan) String() string {
	return fmt.Sprintf("%v", f.Body)
}

type FetchedBlocksSpan struct {
	Parity     parity.Parity
	Stripes    uint8 // [0, 16)
	CellSize   uint32
	Blocks     []FetchedBlock
	StripesCrc []Crc
}

func (f *FetchedBlocksSpan) String() string {
	return fmt.Sprintf("%+v", *f)
}

type IsFetchedSpanBody interface {
	bincode.Packable
	bincode.Unpackable
}

type FetchedSpanHeader struct {
	ByteOffset   uint64
	Size         uint32
	Crc          Crc
	StorageClass StorageClass
}

// If StorageClass is `INLINE_STORAGE`, then the `Body` will be `FetchedInlineSpan`.
// Otherwise it'll be `FetchedBlockSpan`.
type FetchedSpan struct {
	Header FetchedSpanHeader
	Body   IsFetchedSpanBody
}

type BlockService struct {
	Addrs AddrsInfo
	// The BlockServiceId is stable (derived from the secret key, which is stored on the
	// block service id).
	//
	// The ip/port are not, and in fact might be shared by multiple block services.
	Id    BlockServiceId
	Flags BlockServiceFlags
}

type LocalFileSpansResp struct {
	NextOffset    uint64
	BlockServices []BlockService
	Spans         []FetchedSpan
}

type FileSpansReq struct {
	FileId     InodeId
	ByteOffset uint64
	// if 0, no limit.
	Limit uint32
	// if  0, a conservative MTU will be used
	Mtu uint16
}

type FetchedSpanHeaderFull struct {
	ByteOffset uint64
	Size       uint32
	Crc        Crc
	IsInline   bool
}

type FetchedBlockServices struct {
	LocationId   Location
	StorageClass StorageClass
	Parity       parity.Parity
	Stripes      uint8 // [0, 16)
	CellSize     uint32
	Blocks       []FetchedBlock
	StripesCrc   []Crc
}

type FetchedLocations struct {
	Locations []FetchedBlockServices
}

// if Header.IsInline is true, then the `Body` will be `FetchedInlineSpan`.
// Otherwise it'll be `FetchedLocations`.
type FetchedFullSpan struct {
	Header FetchedSpanHeaderFull
	Body   IsFetchedSpanBody
}

type FileSpansResp struct {
	NextOffset    uint64
	BlockServices []BlockService
	Spans         []FetchedFullSpan
}

type SameDirectoryRenameReq struct {
	TargetId InodeId
	DirId    InodeId
	OldName  string
	// This request is a bit annoying in the presence of packet
	// loss. Consider this scenario: a client performs a
	// `SameDirectoryRenameReq`, which goes through, but the
	// response is dropped.
	//
	// In this case the client must retry but genuine failures
	// (for example because the file does not exist) are indistinguishable
	// from failures due to the previous request going through.
	//
	// For this reason we include the creation time here (even if we
	// don't strictly needed because current edges are uniquely
	// identified by name) so that the shard can implement heuristics
	// to let likely repeated calls through in the name of idempotency.
	OldCreationTime TernTime
	NewName         string
}

type SameDirectoryRenameResp struct {
	NewCreationTime TernTime
}

// This is exactly like `SameDirectoryRenameReq`, but it expects
// an owned snapshot edge. Useful to resurrect mistakenly deleted
// files.
type SameDirectoryRenameSnapshotReq struct {
	TargetId        InodeId
	DirId           InodeId
	OldName         string
	OldCreationTime TernTime
	NewName         string
}

type SameDirectoryRenameSnapshotResp struct {
	NewCreationTime TernTime
}

type VisitDirectoriesReq struct {
	BeginId InodeId
	Mtu     uint16
}

type VisitDirectoriesResp struct {
	NextId InodeId
	Ids    []InodeId
}

type VisitFilesReq struct {
	BeginId InodeId
	Mtu     uint16
}

type VisitFilesResp struct {
	NextId InodeId
	Ids    []InodeId
}

// We intentionally do not allow to only visit the "past deadline" files, to not
// have possibly long-running requests on the server (because they need to traverse
// many files before finding the correct one). Instead we let the GC process spend
// time doing that (or we could just stop visiting when we encounter a not-expired
// file, since they are in increasing id order, and therefore the new one come last)
type VisitTransientFilesReq struct {
	BeginId InodeId
	Mtu     uint16
}

type TransientFile struct {
	Id           InodeId
	Cookie       Cookie
	DeadlineTime TernTime
}

// Shall this be unsafe/private? We can freely get the cookie.
type VisitTransientFilesResp struct {
	NextId InodeId
	Files  []TransientFile
}

type FullReadDirFlags uint8

const (
	// Whether the cursor is in current edges. Note that if you're traveling backwards
	// and want to see all edges, you need to set this in the first request.
	FULL_READ_DIR_CURRENT FullReadDirFlags = 1 << 0
	// Travel backwards
	FULL_READ_DIR_BACKWARDS FullReadDirFlags = 1 << 1
	// Only consider the name given in the req. This is needed because we'll need to
	// skip over the other current edges if we need to look only at one edge.
	FULL_READ_DIR_SAME_NAME FullReadDirFlags = 1 << 2
)

// This streams all the edges (snapshot and not), backwards or forwards depending
// on what was requested, and does so regardless whether the directory has been
// removed or not.
//
// Starts from the first edge >= (forwards) or <= (backwards) than the cursor. The
// hash is automatically generated from the name.
type FullReadDirReq struct {
	DirId     InodeId
	Flags     FullReadDirFlags
	StartName string
	StartTime TernTime
	Limit     uint16
	Mtu       uint16
}

type Edge struct {
	Current bool
	// if current, the extra bit tells us whether the edge is locked. this
	// shouldn't really be useful to any client, but FullReadDir is sort of
	// an internal request anyway.
	//
	// if not current, the extra bit tells us whether the edge is owned.
	// this is needed for GC.
	TargetId     InodeIdExtra
	NameHash     NameHash
	Name         string
	CreationTime TernTime
}

func (e *Edge) Owned() bool {
	return e.Current || e.TargetId.Extra()
}

type FullReadDirCursor struct {
	// Current and non-current edges are separated, so we must
	// remember in which section we are.
	Current   bool
	StartName string
	StartTime TernTime
}

type FullReadDirResp struct {
	Next    FullReadDirCursor // default value if we're done
	Results []Edge
}

// Creates a directory with a given parent and given inode id. Unsafe because
// we can create directories with a certain parent while the paren't isn't
// pointing at them (or isn't even a valid inode). We'd break the "no directory leaks"
// invariant or the "null dir owner <-> not current" invariant.
type CreateDirectoryInodeReq struct {
	Id      InodeId
	OwnerId InodeId
	Info    DirectoryInfo
}

type CreateDirectoryInodeResp struct {
	Mtime TernTime
}

// This is needed to move directories -- but it can break the invariants
// between edges pointing to the dir and the owner.
type SetDirectoryOwnerReq struct {
	DirId   InodeId
	OwnerId InodeId // must not be a directory (no NULL_INODE_ID!)
}

type SetDirectoryOwnerResp struct{}

// This is needed to remove directories -- but again, it can break invariants.
type RemoveDirectoryOwnerReq struct {
	DirId InodeId
	// We need the full directory info when removing a directory since we won't
	// be able to inherit it anymore.
	Info DirectoryInfo
}

type RemoveDirectoryOwnerResp struct{}

type RemoveEdgesReq struct {
	DirId InodeId
	Edges []Edge
}

// These is generally needed when we need to move/create things cross-shard,
// but is unsafe in generally since we need to make sure to not leave multiple
// edges pointing to the same target. Moreover we need to create it locked and
// matching the OldCreationTime.
//
// Consider the following scenarios:
//
//   - We might create an edge and not realize because of timeouts, and
//     then create a second, unwanted edge. Both the lock and the
//     OldCreationTime match protect against this.
//   - We might create an edge, lose track of the request because of
//     packet drop in the response, and then somebody else might overwrite
//     the edge. OldCreationTime will make the retry request fail,
//     however at that point we don't know if we have ever succeeded
//     or not. Locking fixes this, because no non-CDC operation will
//     be able to mess with the locked edge.
//   - We might send many create requests because of retries, have one succeed,
//     then unlock the edge, and then have one of the requests we sent
//     lock again because of packet reordering. So at that point we'd
//     leak a lock. This does not happen with OldCreationTime: only
//     one request will succeed.
//
// This means that we need both locking and OldCreationTime. This is quite
// unfortunate: it means that generally we need to first lookup the old
// creation time, then create the locked edge, then unlock it. Three requests
// instead of one.
//
// Also note that this "locking" is clearly only safe given that the CDC
// coordinates the locking. It's based on the assumption that nobody but
// the CDC locks, but internally the CDC needs to coordinate its own locking.
// We currently solve this by just having one transaction at a time in the
// CDC.
type CreateLockedCurrentEdgeReq struct {
	DirId           InodeId
	Name            string
	TargetId        InodeId
	OldCreationTime TernTime
}

type CreateLockedCurrentEdgeResp struct {
	CreationTime TernTime
}

type LockCurrentEdgeReq struct {
	DirId        InodeId
	TargetId     InodeId
	CreationTime TernTime
	Name         string
}

type LockCurrentEdgeResp struct{}

// This also lets us turn edges into snapshot, through `WasMoved`.
type UnlockCurrentEdgeReq struct {
	DirId        InodeId
	Name         string
	CreationTime TernTime
	TargetId     InodeId
	// Turn the current edge into a snapshot edge, and create a deletion
	// edge with the same name.
	WasMoved bool
}

type UnlockCurrentEdgeResp struct{}

type RemoveEdgesResp struct{}

type RemoveNonOwnedEdgeReq struct {
	DirId        InodeId
	TargetId     InodeId
	Name         string
	CreationTime TernTime
}

type RemoveNonOwnedEdgeResp struct{}

// Will remove the snapshot, owned edge; and move the file to transient in one
// go.
type SameShardHardFileUnlinkReq struct {
	OwnerId      InodeId
	TargetId     InodeId
	Name         string
	CreationTime TernTime
}

type SameShardHardFileUnlinkResp struct{}

// This is needed to implement inter-shard hard file unlinking, and it is unsafe, since
// we must make sure that the owned file is made transient in its shard.
type RemoveOwnedSnapshotFileEdgeReq struct {
	OwnerId      InodeId
	TargetId     InodeId
	Name         string
	CreationTime TernTime
}

type RemoveOwnedSnapshotFileEdgeResp struct{}

// This is required to implemented inter-shard had file unlinking, and it is unsafe
// since it lets us make a file which is owned by a directory transient.
type MakeFileTransientReq struct {
	Id   InodeId
	Note string
}

type MakeFileTransientResp struct{}

// Sets transient file deadling to now, allowing it to be garbage collected
type ScrapTransientFileReq struct {
	Id     InodeId
	Cookie Cookie
}

type ScrapTransientFileResp struct{}

type SetDirectoryInfoReq struct {
	Id   InodeId
	Info DirectoryInfo
}

type SetDirectoryInfoResp struct{}

// TODO this works with transient files, but doesn't require a cookie -- it's a bit
// inconsistent.
type SwapBlocksReq struct {
	FileId1     InodeId
	ByteOffset1 uint64
	BlockId1    BlockId
	FileId2     InodeId
	ByteOffset2 uint64
	BlockId2    BlockId
}

type SwapBlocksResp struct{}

// TODO this works with transient files, but doesn't require a cookie -- it's a bit
// inconsistent.
//
// Swaps two spans, both clean. CRCs etc must match.
//
// We need the list of block ids to make sure we're swapping the right thing,
// and to make the request idempotent. The block ids are enough since each block
// id is referenced from exactly one span.
type SwapSpansReq struct {
	FileId1     InodeId
	ByteOffset1 uint64
	Blocks1     []BlockId
	FileId2     InodeId
	ByteOffset2 uint64
	Blocks2     []BlockId
}

type SwapSpansResp struct{}

// Adds a span location from transient file to non transient file used for
// replicating to multiple locations

// We need the list of block ids to make the request idempotent.
// The block ids are enough since each block
// id is referenced from exactly one span.
type AddSpanLocationReq struct {
	FileId1     InodeId
	ByteOffset1 uint64
	Blocks1     []BlockId
	FileId2     InodeId
	ByteOffset2 uint64
}

type AddSpanLocationResp struct{}

type BlockServiceFilesReq struct {
	BlockServiceId BlockServiceId
	// Not strictly needed, since the migration process usually fetches some file ids
	// and then purges all blocks with that block service id. So we can just keep asking
	// from the beginning. However, it can be useful due to the problems listed
	// here <https://github.com/facebook/rocksdb/wiki/Implement-Queue-Service-Using-RocksDB>.
	StartFrom InodeId
}

type BlockServiceFilesResp struct {
	FileIds []InodeId
}

// Moves a span (possibly dirty) from the end of a file onto another file
// (last span must be clean). Useful to get rid of bad under construction spans.
type MoveSpanReq struct {
	SpanSize    uint32
	FileId1     InodeId
	ByteOffset1 uint64
	Cookie1     Cookie
	FileId2     InodeId
	ByteOffset2 uint64
	Cookie2     Cookie
}

type MoveSpanResp struct{}

// The most significant bit indicates whether we're setting
// the time.
type SetTimeReq struct {
	Id    InodeId
	Mtime uint64
	Atime uint64
}

type SetTimeResp struct{}

// --------------------------------------------------------------------
// CDC requests/responses

type CdcSnapshotReq struct {
	SnapshotId uint64
}

type CdcSnapshotResp struct {
}

type MakeDirectoryReq struct {
	OwnerId InodeId
	Name    string
}

type MakeDirectoryResp struct {
	Id           InodeId
	CreationTime TernTime
}

type RenameFileReq struct {
	TargetId        InodeId
	OldOwnerId      InodeId
	OldName         string
	OldCreationTime TernTime
	NewOwnerId      InodeId
	NewName         string
}

type RenameFileResp struct {
	CreationTime TernTime
}

type SoftUnlinkDirectoryReq struct {
	OwnerId      InodeId
	TargetId     InodeId
	CreationTime TernTime
	Name         string
}

type SoftUnlinkDirectoryResp struct{}

type RenameDirectoryReq struct {
	TargetId        InodeId
	OldOwnerId      InodeId
	OldName         string
	OldCreationTime TernTime
	NewOwnerId      InodeId
	NewName         string
}

type RenameDirectoryResp struct {
	CreationTime TernTime
}

// This operation is safe for files: we can check that it has no spans,
// and that it is transient.
//
// For directories however it is not safe. In theory, we could check
// if the directory had no edges (or only non-owning edges), but we might
// be in the middle of a CDC transaction that might be rolled back eventually.
type RemoveInodeReq struct {
	Id InodeId
}

type RemoveInodeResp struct{}

type HardUnlinkDirectoryReq struct {
	DirId InodeId
}

type HardUnlinkDirectoryResp struct{}

type CrossShardHardUnlinkFileReq struct {
	OwnerId      InodeId
	TargetId     InodeId
	Name         string
	CreationTime TernTime
}

type CrossShardHardUnlinkFileResp struct{}

type RemoveZeroBlockServiceFilesReq struct {
	StartBlockService BlockServiceId
	StartFile         InodeId
}

type RemoveZeroBlockServiceFilesResp struct {
	Removed          uint64
	NextBlockService BlockServiceId
	NextFile         InodeId
}

// --------------------------------------------------------------------
// directory info

type DirectoryInfoTag uint8

const SNAPSHOT_POLICY_TAG DirectoryInfoTag = 1
const SPAN_POLICY_TAG DirectoryInfoTag = 2
const BLOCK_POLICY_TAG DirectoryInfoTag = 3
const STRIPE_POLICY_TAG DirectoryInfoTag = 4

var DirectoryInfoTags = []DirectoryInfoTag{SNAPSHOT_POLICY_TAG, SPAN_POLICY_TAG, BLOCK_POLICY_TAG, STRIPE_POLICY_TAG}

func (d DirectoryInfoTag) String() string {
	switch d {
	case SNAPSHOT_POLICY_TAG:
		return "SNAPSHOT"
	case SPAN_POLICY_TAG:
		return "SPAN"
	case BLOCK_POLICY_TAG:
		return "BLOCK"
	case STRIPE_POLICY_TAG:
		return "STRIPE"
	default:
		return fmt.Sprintf("DirectoryInfoTag(%v)", uint8(d))
	}
}

func DirInfoTagFromName(s string) DirectoryInfoTag {
	switch s {
	case "SNAPSHOT":
		return SNAPSHOT_POLICY_TAG
	case "SPAN":
		return SPAN_POLICY_TAG
	case "BLOCK":
		return BLOCK_POLICY_TAG
	case "STRIPE":
		return STRIPE_POLICY_TAG
	default:
		panic(fmt.Errorf("bad dir info tag string '%v'", s))
	}
}

// MSB: whether this policy is active or not. After: nanoseconds.
type DeleteAfterTime uint64

func (dat DeleteAfterTime) Active() bool {
	return (uint64(dat) >> 63) != 0
}

func (dat DeleteAfterTime) String() string {
	if dat.Active() {
		return fmt.Sprintf("ActiveDeleteAfterTime(%v)", dat.Time())
	} else {
		return "InactiveDeleteAfterTime()"
	}
}

func (dat DeleteAfterTime) Time() time.Duration {
	return time.Duration(uint64(dat) & ^(uint64(1) << 63))
}

func InactiveDeleteAfterTime() DeleteAfterTime {
	return 0
}

func ActiveDeleteAfterTime(duration time.Duration) DeleteAfterTime {
	if duration.Nanoseconds() < 0 {
		panic(fmt.Errorf("negative duration in DeleteAfterTime: %v", duration))
	}
	return DeleteAfterTime((uint64(1) << 63) | uint64(duration.Nanoseconds()))
}

// MSB: whether this policy is active or not. After: number of versions.
type DeleteAfterVersions uint16

func (dav DeleteAfterVersions) Active() bool {
	return (uint16(dav) >> 15) != 0
}

func (dav DeleteAfterVersions) Versions() uint16 {
	return uint16(dav) & ^(uint16(1) << 15)
}

func (dav DeleteAfterVersions) String() string {
	if dav.Active() {
		return fmt.Sprintf("ActiveDeleteAfterVersions(%v)", dav.Versions())
	} else {
		return "InactiveDeleteAfterVersions()"
	}
}

func InactiveDeleteAfterVersions() DeleteAfterVersions {
	return 0
}

func ActiveDeleteAfterVersions(versions int16) DeleteAfterVersions {
	if versions < 0 {
		panic(fmt.Errorf("negative versions: %v", versions))
	}
	return DeleteAfterVersions((uint16(1) << 15) | uint16(versions))
}

// If multiple policies are present, the file will be deleted if
// any of the policies are not respected.
//
// If neither policies are active, then snapshots will be kept forever.
//
// Also note that you can use either policy to delete all snapshots, by
// setting either to zero.
type SnapshotPolicy struct {
	DeleteAfterTime     DeleteAfterTime
	DeleteAfterVersions DeleteAfterVersions
}

func (p *SnapshotPolicy) Tag() DirectoryInfoTag {
	return SNAPSHOT_POLICY_TAG
}

func (p *SnapshotPolicy) UnmarshalJSON(b []byte) error {
	var spec struct {
		DeleteAfterVersions *uint16
		DeleteAfterTime     *string
	}
	if err := json.Unmarshal(b, &spec); err != nil {
		return err
	}
	if spec.DeleteAfterVersions != nil {
		p.DeleteAfterVersions = ActiveDeleteAfterVersions(int16(*spec.DeleteAfterVersions))
	} else {
		p.DeleteAfterVersions = InactiveDeleteAfterVersions()
	}
	if spec.DeleteAfterTime != nil {
		dur, err := time.ParseDuration(*spec.DeleteAfterTime)
		if err != nil {
			return err
		}
		p.DeleteAfterTime = ActiveDeleteAfterTime(dur)
	} else {
		p.DeleteAfterTime = InactiveDeleteAfterTime()
	}
	return nil
}

// 100MiB
const MAX_BLOCK_SIZE uint32 = 100 << 20

type BlockPolicyEntry struct {
	StorageClass StorageClass
	// The minimum amount of block size we want in this storage class. For spinning
	// disks, a function of seek latency and sustained data rate.
	MinSize uint32
}

// Sorted by MinSize.
type BlockPolicy struct {
	Entries []BlockPolicyEntry
}

func (p *BlockPolicy) Tag() DirectoryInfoTag {
	return BLOCK_POLICY_TAG
}

func (p *BlockPolicy) Pick(size uint32) (entry *BlockPolicyEntry) {
	if size > MAX_BLOCK_SIZE {
		panic(fmt.Errorf("size %v is greater than overall maximum size %v", size, MAX_BLOCK_SIZE))
	}
	for i := len(p.Entries) - 1; i >= 0; i-- {
		if size > p.Entries[i].MinSize {
			return &p.Entries[i]
		}
	}
	return &p.Entries[0]
}

type SpanPolicyEntry struct {
	MaxSize uint32
	Parity  parity.Parity
}

type SpanPolicy struct {
	Entries []SpanPolicyEntry
}

func (p *SpanPolicy) Pick(size uint32) (entry *SpanPolicyEntry) {
	if p.Entries[len(p.Entries)-1].MaxSize < size {
		panic(fmt.Errorf("size %v is greater than overall maximum size %v", size, p.Entries[len(p.Entries)-1].MaxSize))
	}
	for i := len(p.Entries) - 2; i >= 0; i-- {
		if size > p.Entries[i].MaxSize {
			return &p.Entries[i+1]
		}
	}
	return &p.Entries[0]
}

func (p *SpanPolicy) Tag() DirectoryInfoTag {
	return SPAN_POLICY_TAG
}

// This will in essence determine how much data we need to read from the block
// services before being able to return to the client.
type StripePolicy struct {
	TargetStripeSize uint32
}

func (p *StripePolicy) Tag() DirectoryInfoTag {
	return STRIPE_POLICY_TAG
}

func (p *StripePolicy) Stripes(size uint32) uint8 {
	S := int(size) / int(p.TargetStripeSize)
	if S < 1 {
		S = 1
	}
	if S > 15 {
		S = 15
	}
	return uint8(S)
}

// --------------------------------------------------------------------
// shard log entries
//
// these are only used internally, but we define them here for codegen
// simplicity.
//
// We only define the individual entries here, the framing is specified
// in c++. The framing, amongst other things, includes the log entry time
// and the index, so we don't also store it here.

type ConstructFileEntry struct {
	Type         InodeType
	DeadlineTime TernTime
	Note         string
}

type LinkFileEntry struct {
	FileId  InodeId
	OwnerId InodeId
	Name    string
}

type SameDirectoryRenameEntry struct {
	DirId           InodeId
	TargetId        InodeId
	OldName         string
	OldCreationTime TernTime
	NewName         string
}

type SameDirectoryRenameSnapshotEntry struct {
	DirId           InodeId
	TargetId        InodeId
	OldName         string
	OldCreationTime TernTime
	NewName         string
}

type SoftUnlinkFileEntry struct {
	OwnerId      InodeId
	FileId       InodeId
	Name         string
	CreationTime TernTime
}

type CreateDirectoryInodeEntry struct {
	Id      InodeId
	OwnerId InodeId
	Info    DirectoryInfo
}

type CreateLockedCurrentEdgeEntry struct {
	DirId           InodeId
	Name            string
	TargetId        InodeId
	OldCreationTime TernTime
}

type UnlockCurrentEdgeEntry struct {
	DirId InodeId
	Name  string
	// Here the `CreationTime` is currently not strictly needed, since we have the
	// locking mechanism + CDC synchronization anyway, which offer stronger guarantees
	// which means we never need heuristics for this. But we include it for consistency
	// and to better detect bugs.
	CreationTime TernTime
	TargetId     InodeId
	WasMoved     bool
}

type LockCurrentEdgeEntry struct {
	DirId        InodeId
	Name         string
	CreationTime TernTime
	TargetId     InodeId
}

type RemoveDirectoryOwnerEntry struct {
	DirId InodeId
	Info  DirectoryInfo
}

type RemoveInodeEntry struct {
	Id InodeId
}

type SetDirectoryOwnerEntry struct {
	DirId   InodeId
	OwnerId InodeId // must be a directory (no NULL_INODE_ID!)
}

type SetDirectoryInfoEntry struct {
	DirId InodeId
	Info  DirectoryInfo
}

type RemoveNonOwnedEdgeEntry struct {
	DirId        InodeId
	TargetId     InodeId
	Name         string
	CreationTime TernTime
}

type SameShardHardFileUnlinkDEPRECATEDEntry struct {
	OwnerId      InodeId
	TargetId     InodeId
	Name         string
	CreationTime TernTime
}

type SameShardHardFileUnlinkEntry struct {
	OwnerId      InodeId
	TargetId     InodeId
	Name         string
	CreationTime TernTime
	DeadlineTime TernTime // Deadline for transient file
}

type RemoveSpanInitiateEntry struct {
	FileId InodeId
}

type AddInlineSpanEntry struct {
	FileId       InodeId
	StorageClass StorageClass // Either EMPTY_STORAGE or INLINE_STORAGE
	ByteOffset   uint64
	Size         uint32
	Crc          Crc
	Body         []byte
}

type BlockServiceDeprecatedInfo struct {
	Id               BlockServiceId
	Addrs            AddrsInfo
	StorageClass     StorageClass
	FailureDomain    FailureDomain
	SecretKey        [16]byte
	Flags            BlockServiceFlags
	CapacityBytes    uint64
	AvailableBytes   uint64
	Blocks           uint64 // how many blocks we have
	Path             string
	LastSeen         TernTime
	HasFiles         bool
	FlagsLastChanged TernTime
}

type EntryNewBlockInfo struct {
	BlockServiceId BlockServiceId
	Crc            Crc
}

type AddSpanAtLocationInitiateEntry struct {
	LocationId    Location
	WithReference bool
	FileId        InodeId
	ByteOffset    uint64
	Size          uint32
	Crc           Crc
	StorageClass  StorageClass
	Parity        parity.Parity
	Stripes       uint8 // [1, 16]
	CellSize      uint32
	BodyBlocks    []EntryNewBlockInfo
	BodyStripes   []Crc // the CRCs
}

type AddSpanInitiateEntry struct {
	WithReference bool
	FileId        InodeId
	ByteOffset    uint64
	Size          uint32
	Crc           Crc
	StorageClass  StorageClass
	Parity        parity.Parity
	Stripes       uint8 // [1, 16]
	CellSize      uint32
	BodyBlocks    []EntryNewBlockInfo
	BodyStripes   []Crc // the CRCs
}

type AddSpanCertifyEntry struct {
	FileId     InodeId
	ByteOffset uint64
	Proofs     []BlockProof
}

type MakeFileTransientDEPRECATEDEntry struct {
	Id   InodeId
	Note string
}

type MakeFileTransientEntry struct {
	Id           InodeId
	DeadlineTime TernTime
	Note         string
}

type ScrapTransientFileEntry struct {
	Id           InodeId
	DeadlineTime TernTime
}

type RemoveSpanCertifyEntry struct {
	FileId     InodeId
	ByteOffset uint64
	Proofs     []BlockProof
}

type RemoveOwnedSnapshotFileEdgeEntry struct {
	OwnerId      InodeId
	TargetId     InodeId
	Name         string
	CreationTime TernTime
}

type SwapBlocksEntry struct {
	FileId1     InodeId
	ByteOffset1 uint64
	BlockId1    BlockId
	FileId2     InodeId
	ByteOffset2 uint64
	BlockId2    BlockId
}

type SwapSpansEntry struct {
	FileId1     InodeId
	ByteOffset1 uint64
	Blocks1     []BlockId
	FileId2     InodeId
	ByteOffset2 uint64
	Blocks2     []BlockId
}

type AddSpanLocationEntry struct {
	FileId1     InodeId
	ByteOffset1 uint64
	Blocks1     []BlockId
	FileId2     InodeId
	ByteOffset2 uint64
}

type MoveSpanEntry struct {
	SpanSize    uint32
	FileId1     InodeId
	ByteOffset1 uint64
	Cookie1     Cookie
	FileId2     InodeId
	ByteOffset2 uint64
	Cookie2     Cookie
}

type SetTimeEntry struct {
	Id    InodeId
	Mtime uint64
	Atime uint64
}

type RemoveZeroBlockServiceFilesEntry struct {
	StartBlockService BlockServiceId
	StartFile         InodeId
}

// --------------------------------------------------------------------
// registry requests/responses

type RegistryRequest interface {
	bincode.Packable
	bincode.Unpackable
	RegistryRequestKind() RegistryMessageKind
}

type RegistryResponse interface {
	bincode.Packable
	bincode.Unpackable
	RegistryResponseKind() RegistryMessageKind
}

type InfoReq struct{}

type InfoResp struct {
	NumBlockServices  uint32
	NumFailureDomains uint32
	Capacity          uint64
	Available         uint64
	Blocks            uint64
}

type RegistryReq struct{}

type AddrsInfo struct {
	Addr1 IpPort
	Addr2 IpPort
}
type RegistryResp struct {
	Addrs AddrsInfo
}
type RegisterRegistryReq struct {
	ReplicaId ReplicaId
	Location  Location
	IsLeader  bool
	Addrs     AddrsInfo
	Bootstrap bool // indicating requestor still does not see enough replicas to form a quorum
}

type RegisterRegistryResp struct{}

type FullRegistryInfo struct {
	Id         ReplicaId
	LocationId Location
	IsLeader   bool
	Addrs      AddrsInfo
	LastSeen   TernTime
}

type AllRegistryReplicasReq struct {}

type AllRegistryReplicasResp struct {
	Replicas []FullRegistryInfo
}

type RegisterRegistryInstanceReplicaResp struct{}

type DecommissionBlockServiceReq struct {
	Id BlockServiceId
}

type DecommissionBlockServiceResp struct{}

type SetBlockServiceFlagsReq struct {
	Id        BlockServiceId
	Flags     BlockServiceFlags
	FlagsMask uint8
}

type SetBlockServiceFlagsResp struct{}

type AllBlockServicesDeprecatedReq struct{}

type AllBlockServicesDeprecatedResp struct {
	BlockServices []BlockServiceDeprecatedInfo
}

type LocalChangedBlockServicesReq struct {
	ChangedSince TernTime
}

type LocalChangedBlockServicesResp struct {
	LastChange    TernTime
	BlockServices []BlockService
}

type ChangedBlockServicesAtLocationReq struct {
	LocationId   Location
	ChangedSince TernTime
}

type ChangedBlockServicesAtLocationResp struct {
	LastChange    TernTime
	BlockServices []BlockService
}

type RegisterBlockServiceInfo struct {
	Id             BlockServiceId
	LocationId     Location
	Addrs          AddrsInfo
	StorageClass   StorageClass
	FailureDomain  FailureDomain
	SecretKey      [16]byte
	Flags          BlockServiceFlags
	FlagsMask      uint8
	CapacityBytes  uint64
	AvailableBytes uint64
	Blocks         uint64 // how many blocks we have
	Path           string
}

type FullBlockServiceInfo struct {
	Id             BlockServiceId
	LocationId     Location
	Addrs          AddrsInfo
	StorageClass   StorageClass
	FailureDomain  FailureDomain
	SecretKey      [16]byte
	Flags          BlockServiceFlags
	CapacityBytes  uint64
	AvailableBytes uint64
	Blocks         uint64 // how many blocks we have
	FirstSeen	   TernTime
	LastSeen	   TernTime
	LastInfoChange TernTime // change to read/write status or addresses
	HasFiles       bool
	Path           string
}

type RegisterBlockServicesReq struct {
	BlockServices []RegisterBlockServiceInfo
}

type RegisterBlockServicesResp struct{}

type UpdateBlockServicePathReq struct {
	Id      BlockServiceId
	NewPath string
}

type UpdateBlockServicePathResp struct{}

type EraseDecommissionedBlockReq struct {
	BlockServiceId BlockServiceId
	BlockId        BlockId
	Certificate    [8]byte
}

type EraseDecommissionedBlockResp struct {
	Proof [8]byte
}

type RegisterShardReq struct {
	Shrid    ShardReplicaId
	IsLeader bool
	Addrs    AddrsInfo
	Location Location
}

type RegisterShardResp struct{}

// Depending on which registry is asked it always returns local shards
type LocalShardsReq struct{}

type ShardInfo struct {
	Addrs    AddrsInfo
	LastSeen TernTime
}

type LocalShardsResp struct {
	// Always 256 length. If we don't have info for some shards, the ShardInfo
	// is zeroed.
	Shards []ShardInfo
}

type ShardsAtLocationReq struct {
	LocationId Location
}

type ShardsAtLocationResp struct {
	// Always 256 length. If we don't have info for some shards, the ShardInfo
	// is zeroed.
	Shards []ShardInfo
}

// Gets the block services a shard should use to write
type ShardBlockServicesReq struct {
	ShardId ShardId
}

type BlockServiceInfoShort struct {
	LocationId    Location
	FailureDomain FailureDomain
	Id            BlockServiceId
	StorageClass  StorageClass
}

type ShardBlockServicesResp struct {
	BlockServices []BlockServiceInfoShort
}

type AllShardsReq struct{}

type FullShardInfo struct {
	Id         ShardReplicaId
	IsLeader   bool
	Addrs      AddrsInfo
	LastSeen   TernTime
	LocationId Location
}

type AllShardsResp struct {
	Shards []FullShardInfo
}

type MoveShardLeaderReq struct {
	Shrid    ShardReplicaId
	Location Location
}

type MoveShardLeaderResp struct{}

type ClearShardInfoReq struct {
	Shrid    ShardReplicaId
	Location Location
}

type ClearShardInfoResp struct{}

type RegisterCdcReq struct {
	Replica  ReplicaId
	Location Location
	IsLeader bool
	Addrs    AddrsInfo
}

type RegisterCdcResp struct{}

type LocalCdcReq struct{}

type LocalCdcResp struct {
	Addrs    AddrsInfo
	LastSeen TernTime
}

type CdcAtLocationReq struct {
	LocationId Location
}

type CdcAtLocationResp struct {
	Addrs    AddrsInfo
	LastSeen TernTime
}

type AllCdcReq struct{}

type CdcInfo struct {
	ReplicaId  ReplicaId
	LocationId Location
	IsLeader   bool
	Addrs      AddrsInfo
	LastSeen   TernTime
}

type AllCdcResp struct {
	Replicas []CdcInfo
}

type MoveCdcLeaderReq struct {
	Replica  ReplicaId
	Location Location
}

type MoveCdcLeaderResp struct{}

type ClearCdcInfoReq struct {
	Replica  ReplicaId
	Location Location
}

type ClearCdcInfoResp struct{}

type CreateLocationReq struct {
	Id   Location
	Name string
}

type CreateLocationResp struct{}

type RenameLocationReq struct {
	Id   Location
	Name string
}

type RenameLocationResp struct{}

type LocationsReq struct{}

type LocationInfo struct {
	Id   Location
	Name string
}
type LocationsResp struct {
	Locations []LocationInfo
}

type CdcReplicasDEPRECATEDReq struct{}

type CdcReplicasDEPRECATEDResp struct {
	// Always 5 length. If we don't have info for some replicas, the AddrsInfo
	// is zeroed.
	Replicas []AddrsInfo
}

type ShardBlockServicesDEPRECATEDReq struct {
	ShardId ShardId
}

type ShardBlockServicesDEPRECATEDResp struct {
	BlockServices []BlockServiceId
}

// --------------------------------------------------------------------
// block service requests/responses

type BlocksRequest interface {
	bincode.Packable
	bincode.Unpackable
	BlocksRequestKind() BlocksMessageKind
}

type BlocksResponse interface {
	bincode.Packable
	bincode.Unpackable
	BlocksResponseKind() BlocksMessageKind
}

type EraseBlockReq struct {
	BlockId     BlockId
	Certificate [8]byte
}

type EraseBlockResp struct {
	Proof [8]byte
}

type FetchBlockReq struct {
	BlockId BlockId
	Offset  uint32
	Count   uint32
}

// Followed by data
type FetchBlockResp struct{}

// Offset needs to be multiple of 4K and count as well
type FetchBlockWithCrcReq struct {
	FileId   InodeId
	BlockId  BlockId
	BlockCrc Crc
	Offset   uint32
	Count    uint32
}

// Followed by data with CRC inserted after every 4K
type FetchBlockWithCrcResp struct{}

type WriteBlockReq struct {
	BlockId     BlockId
	Crc         Crc
	Size        uint32
	Certificate [8]byte
}

// Note that, given the way the write req/resp works, even if
// there's an error immediately the block service will still
// consume the required amount and _then_ send the error back.
type WriteBlockResp struct {
	Proof [8]byte
}

type TestWriteReq struct {
	Size uint64
}

type TestWriteResp struct{}

type CheckBlockReq struct {
	BlockId BlockId
	Size    uint32
	Crc     Crc
}

type CheckBlockResp struct{}

// --------------------------------------------------------------------
// Distributed log requests/responses

type LogRequest interface {
	bincode.Packable
	bincode.Unpackable
	LogRequestKind() LogMessageKind
}

type LogResponse interface {
	bincode.Packable
	bincode.Unpackable
	LogResponseKind() LogMessageKind
}

type LogWriteReq struct {
	Token        LeaderToken
	LastReleased LogIdx
	Idx          LogIdx
	Value        bincode.Blob
}

type LogWriteResp struct {
	Result TernError
}

type ReleaseReq struct {
	Token        LeaderToken
	LastReleased LogIdx
}

type ReleaseResp struct {
	Result TernError
}

type LogReadReq struct {
	Idx LogIdx
}

type LogReadResp struct {
	Result TernError
	Value  bincode.Blob
}

type NewLeaderReq struct {
	NomineeToken LeaderToken
}

type NewLeaderResp struct {
	Result       TernError
	LastReleased LogIdx
}

type NewLeaderConfirmReq struct {
	NomineeToken LeaderToken
	ReleasedIdx  LogIdx
}

type NewLeaderConfirmResp struct {
	Result TernError
}

type LogRecoveryReadReq struct {
	NomineeToken LeaderToken
	Idx          LogIdx
}

type LogRecoveryReadResp struct {
	Result TernError
	Value  bincode.Blob
}

type LogRecoveryWriteReq struct {
	NomineeToken LeaderToken
	Idx          LogIdx
	Value        bincode.Blob
}

type LogRecoveryWriteResp struct {
	Result TernError
}
