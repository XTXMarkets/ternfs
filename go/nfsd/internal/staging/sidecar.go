// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/byterange"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// The only sidecar format this nfsd reads or writes. NFS8 is the NFSv4.1
// server: one writer per client and file, found by stateid. Earlier formats
// belong to the NFSv4.0 server and make startup fail; drain or remove the
// staging directory before upgrading.
const (
	formatMagic = "NFS8"
	MetaVersion = 8
)

// FormatError reports a sidecar written by an incompatible nfsd.
type FormatError struct {
	path  string
	magic string
}

func (e FormatError) Error() string {
	return fmt.Sprintf("staging sidecar %s has format %q, want %q; drain or remove the staging directory written by the older nfsd",
		e.path, e.magic, formatMagic)
}

// Meta contains the state needed to complete CLOSE after a restart.
type Meta struct {
	DirID           vfs.InodeID  // directory to link the file into on CLOSE
	FileName        string       // name in directory
	TernCookie      vfs.Cookie   // cookie from VFS ConstructFile
	NFSStateID      nfs4.StateID // random, returned to NFS client as stateid "other"
	ClientID        uint64       // owning client
	OpenOwner       string       // separates recovered writers belonging to the same client
	OwnerKnown      bool         // an empty owner is valid
	RecoveryKey     [32]byte     // stable client identity, boot verifier and principal
	Unlinked        bool         // detached from its publication target
	Guarded         bool         // interrupted namespace operations quarantine at startup
	Retired         bool         // lease expired; retain acknowledged data for recovery
	ReadOnly        bool         // CREATE may initialize size/times with read-only share access
	Exclusive       bool         // EXCLUSIVE4 create
	Verifier        [8]byte      // EXCLUSIVE4 create verifier
	BaseID          vfs.InodeID  // immutable file being replaced, or a create's placeholder; zero for none
	BaseSize        uint64
	Size            uint64
	Dirty           byterange.Set
	MetadataChanged bool
	Attrs           vfs.NodeInfo // Size is stored separately
	Version         uint8
}

// Sidecar file format (binary, big-endian):
//   [8]  DirID
//   [8]  TernCookie
//   [12] NFSStateID
//   [2]  FileNameLen
//   [N]  FileName (UTF-8)
//   [8]  ClientID
//   [4]  "NFS8"
//   [8]  BaseID
//   [8]  BaseSize
//   [8]  LogicalSize
//   [4]  DirtyRangeCount
//   [16 * count] Dirty ranges as start/end pairs
//   [8] Change
//   [8] Mtime, [8] Atime, [8] Ctime (Unix nanoseconds)
//   [1] flags: bit 0 MetadataChanged, bit 1 OwnerKnown, bit 2 ReadOnly,
//       bit 3 Retired, bit 4 Exclusive, bit 5 Unlinked, bit 6 Guarded
//   [4] OpenOwnerLen, [N] OpenOwner (opaque bytes)
//   [32] RecoveryKey
//   [8] EXCLUSIVE4 verifier

func SaveMeta(path string, meta Meta) error {
	nameBytes := []byte(meta.FileName)
	buf := make([]byte, 8+8+12+2+len(nameBytes)+8+4+8+8+8+4+
		16*len(meta.Dirty)+32+1+4+len(meta.OpenOwner)+
		len(meta.RecoveryKey)+len(meta.Verifier))
	binary.BigEndian.PutUint64(buf[0:8], uint64(meta.DirID))
	copy(buf[8:16], meta.TernCookie[:])
	copy(buf[16:28], meta.NFSStateID[:])
	binary.BigEndian.PutUint16(buf[28:30], uint16(len(nameBytes)))
	copy(buf[30:], nameBytes)
	off := 30 + len(nameBytes)
	binary.BigEndian.PutUint64(buf[off:off+8], meta.ClientID)
	off += 8
	copy(buf[off:off+4], formatMagic)
	off += 4
	binary.BigEndian.PutUint64(buf[off:off+8], uint64(meta.BaseID))
	off += 8
	binary.BigEndian.PutUint64(buf[off:off+8], meta.BaseSize)
	off += 8
	binary.BigEndian.PutUint64(buf[off:off+8], meta.Size)
	off += 8
	binary.BigEndian.PutUint32(buf[off:off+4], uint32(len(meta.Dirty)))
	off += 4
	for _, r := range meta.Dirty {
		binary.BigEndian.PutUint64(buf[off:off+8], r.Start)
		binary.BigEndian.PutUint64(buf[off+8:off+16], r.End)
		off += 16
	}
	binary.BigEndian.PutUint64(buf[off:off+8], meta.Attrs.Change)
	for i, stamp := range []time.Time{meta.Attrs.Mtime, meta.Attrs.Atime, meta.Attrs.Ctime} {
		binary.BigEndian.PutUint64(buf[off+8+i*8:off+16+i*8], uint64(stamp.UnixNano()))
	}
	off += 32
	if meta.MetadataChanged {
		buf[off] = 1
	}
	if meta.OwnerKnown {
		buf[off] |= 2
	}
	if meta.ReadOnly {
		buf[off] |= 4
	}
	if meta.Retired {
		buf[off] |= 8
	}
	if meta.Exclusive {
		buf[off] |= 16
	}
	if meta.Unlinked {
		buf[off] |= 32
	}
	if meta.Guarded {
		buf[off] |= 64
	}
	off++
	binary.BigEndian.PutUint32(buf[off:off+4], uint32(len(meta.OpenOwner)))
	copy(buf[off+4:], meta.OpenOwner)
	off += 4 + len(meta.OpenOwner)
	copy(buf[off:], meta.RecoveryKey[:])
	copy(buf[off+len(meta.RecoveryKey):], meta.Verifier[:])
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if n, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return err
	} else if n != len(buf) {
		tmp.Close()
		return io.ErrShortWrite
	}
	// Persist the new file before replacing the last checkpoint's name.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return syncDir(dir)
}

func LoadMeta(path string) (Meta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Meta{}, err
	}
	if len(data) < 30 {
		return Meta{}, fmt.Errorf("meta file too short")
	}
	var meta Meta
	meta.DirID = vfs.InodeID(binary.BigEndian.Uint64(data[0:8]))
	copy(meta.TernCookie[:], data[8:16])
	copy(meta.NFSStateID[:], data[16:28])
	nameLen := int(binary.BigEndian.Uint16(data[28:30]))
	nameEnd := 30 + nameLen
	if len(data) < nameEnd {
		return Meta{}, fmt.Errorf("meta file truncated")
	}
	meta.FileName = string(data[30:nameEnd])
	if len(data) < nameEnd+12 {
		return Meta{}, fmt.Errorf("meta file truncated")
	}
	meta.ClientID = binary.BigEndian.Uint64(data[nameEnd : nameEnd+8])
	off := nameEnd + 8
	magic := string(data[off : off+4])
	if magic != formatMagic {
		if strings.HasPrefix(magic, "NFS") {
			return Meta{}, FormatError{path: path, magic: magic}
		}
		return Meta{}, fmt.Errorf("unknown meta file extension")
	}
	off += 4
	if len(data) < off+20 {
		return Meta{}, fmt.Errorf("meta file truncated")
	}
	meta.BaseID = vfs.InodeID(binary.BigEndian.Uint64(data[off : off+8]))
	off += 8
	meta.BaseSize = binary.BigEndian.Uint64(data[off : off+8])
	off += 8
	if len(data) < off+8 {
		return Meta{}, fmt.Errorf("meta file truncated")
	}
	meta.Size = binary.BigEndian.Uint64(data[off : off+8])
	meta.Version = MetaVersion
	off += 8
	if len(data) < off+4 {
		return Meta{}, fmt.Errorf("meta file truncated")
	}
	count := int(binary.BigEndian.Uint32(data[off : off+4]))
	off += 4
	if count > (len(data)-off)/16 {
		return Meta{}, fmt.Errorf("invalid dirty range count")
	}
	for range count {
		start := binary.BigEndian.Uint64(data[off : off+8])
		end := binary.BigEndian.Uint64(data[off+8 : off+16])
		if start >= end {
			return Meta{}, fmt.Errorf("invalid dirty range")
		}
		if len(meta.Dirty) != 0 && start <= meta.Dirty[len(meta.Dirty)-1].End {
			return Meta{}, fmt.Errorf("dirty ranges are not normalized")
		}
		meta.Dirty = append(meta.Dirty, byterange.Range{Start: start, End: end})
		off += 16
	}
	if len(data)-off < 37 {
		return Meta{}, fmt.Errorf("truncated staging attributes")
	}
	meta.Attrs.Change = binary.BigEndian.Uint64(data[off : off+8])
	if meta.Attrs.Change != 0 {
		meta.Attrs.Mtime = time.Unix(0, int64(binary.BigEndian.Uint64(data[off+8:off+16])))
		meta.Attrs.Atime = time.Unix(0, int64(binary.BigEndian.Uint64(data[off+16:off+24])))
		meta.Attrs.Ctime = time.Unix(0, int64(binary.BigEndian.Uint64(data[off+24:off+32])))
	}
	off += 32
	if data[off]&^byte(127) != 0 {
		return Meta{}, fmt.Errorf("invalid metadata change flag")
	}
	meta.MetadataChanged = data[off]&1 != 0
	meta.OwnerKnown = data[off]&2 != 0
	meta.ReadOnly = data[off]&4 != 0
	meta.Retired = data[off]&8 != 0
	meta.Exclusive = data[off]&16 != 0
	meta.Unlinked = data[off]&32 != 0
	meta.Guarded = data[off]&64 != 0
	off++
	ownerLen := uint64(binary.BigEndian.Uint32(data[off : off+4]))
	off += 4
	// The recovery key and the EXCLUSIVE4 verifier close every sidecar.
	tailLen := uint64(len(meta.RecoveryKey) + len(meta.Verifier))
	if uint64(len(data)-off) != ownerLen+tailLen {
		return Meta{}, fmt.Errorf("invalid staging owner length")
	}
	meta.OpenOwner = string(data[off : off+int(ownerLen)])
	off += int(ownerLen)
	copy(meta.RecoveryKey[:], data[off:off+len(meta.RecoveryKey)])
	off += len(meta.RecoveryKey)
	copy(meta.Verifier[:], data[off:off+len(meta.Verifier)])
	off += len(meta.Verifier)
	if off != len(data) {
		return Meta{}, fmt.Errorf("trailing staging metadata")
	}
	if meta.Size > math.MaxInt64 || meta.BaseSize > math.MaxInt64 {
		return Meta{}, fmt.Errorf("staging size exceeds supported offset")
	}
	return meta, nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
