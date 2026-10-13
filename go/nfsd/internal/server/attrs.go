// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Supported attribute bitmasks.
const (
	supportedAttrs0 = (1 << nfs4.FATTR4_SUPPORTED_ATTRS) |
		(1 << nfs4.FATTR4_TYPE) |
		(1 << nfs4.FATTR4_FH_EXPIRE_TYPE) |
		(1 << nfs4.FATTR4_CHANGE) |
		(1 << nfs4.FATTR4_SIZE) |
		(1 << nfs4.FATTR4_LINK_SUPPORT) |
		(1 << nfs4.FATTR4_SYMLINK_SUPPORT) |
		(1 << nfs4.FATTR4_NAMED_ATTR) |
		(1 << nfs4.FATTR4_FSID) |
		(1 << nfs4.FATTR4_UNIQUE_HANDLES) |
		(1 << nfs4.FATTR4_LEASE_TIME) |
		(1 << nfs4.FATTR4_RDATTR_ERROR) |
		(1 << nfs4.FATTR4_CANSETTIME) |
		(1 << nfs4.FATTR4_CASE_INSENSITIVE) |
		(1 << nfs4.FATTR4_CASE_PRESERVING) |
		(1 << nfs4.FATTR4_CHOWN_RESTRICTED) |
		(1 << nfs4.FATTR4_FILEHANDLE) |
		(1 << nfs4.FATTR4_FILEID) |
		(1 << nfs4.FATTR4_FILES_AVAIL) |
		(1 << nfs4.FATTR4_FILES_FREE) |
		(1 << nfs4.FATTR4_FILES_TOTAL) |
		(1 << nfs4.FATTR4_HOMOGENEOUS) |
		(1 << nfs4.FATTR4_MAXFILESIZE) |
		(1 << nfs4.FATTR4_MAXLINK) |
		(1 << nfs4.FATTR4_MAXNAME) |
		(1 << nfs4.FATTR4_MAXREAD) |
		(1 << nfs4.FATTR4_MAXWRITE)

	supportedAttrs1 = (1 << (nfs4.FATTR4_MODE - 32)) |
		(1 << (nfs4.FATTR4_NO_TRUNC - 32)) |
		(1 << (nfs4.FATTR4_NUMLINKS - 32)) |
		(1 << (nfs4.FATTR4_OWNER - 32)) |
		(1 << (nfs4.FATTR4_OWNER_GROUP - 32)) |
		(1 << (nfs4.FATTR4_RAWDEV - 32)) |
		(1 << (nfs4.FATTR4_SPACE_AVAIL - 32)) |
		(1 << (nfs4.FATTR4_SPACE_FREE - 32)) |
		(1 << (nfs4.FATTR4_SPACE_TOTAL - 32)) |
		(1 << (nfs4.FATTR4_SPACE_USED - 32)) |
		(1 << (nfs4.FATTR4_TIME_ACCESS - 32)) |
		(1 << (nfs4.FATTR4_TIME_DELTA - 32)) |
		(1 << (nfs4.FATTR4_TIME_METADATA - 32)) |
		(1 << (nfs4.FATTR4_TIME_MODIFY - 32)) |
		(1 << (nfs4.FATTR4_MOUNTED_ON_FILEID - 32))

	// SUPPATTR_EXCLCREAT reports the attributes EXCLUSIVE4_1 may set. It is
	// empty: clients set attributes after an exclusive create with SETATTR.
	supportedAttrs2 = 1 << (nfs4.FATTR4_SUPPATTR_EXCLCREAT - 64)

	// Settable, write-only attributes. These have no GETATTR value (they are
	// not in supportedAttrs* and never encoded), but must be advertised in
	// FATTR4_SUPPORTED_ATTRS or clients won't send them in SETATTR. SIZE is
	// already in supportedAttrs0 since it is both readable and settable.
	settableAttrs0 uint32 = 0
	settableAttrs1 uint32 = (1 << (nfs4.FATTR4_TIME_ACCESS_SET - 32)) |
		(1 << (nfs4.FATTR4_TIME_MODIFY_SET - 32))

	// SETATTR-only attributes cannot be requested by GETATTR or compared by
	// VERIFY/NVERIFY. RDATTR_ERROR may be returned by GETATTR, but cannot be
	// compared.
	invalidGetattrAttrs0 uint32 = 0
	invalidGetattrAttrs1        = settableAttrs1
	invalidVerifyAttrs0         = 1 << nfs4.FATTR4_RDATTR_ERROR
	invalidVerifyAttrs1         = settableAttrs1

	writableAttrs0 uint32 = (1 << nfs4.FATTR4_SIZE) |
		(1 << nfs4.FATTR4_ACL) |
		(1 << nfs4.FATTR4_ARCHIVE) |
		(1 << nfs4.FATTR4_HIDDEN)
	writableAttrs1 uint32 = (1 << (nfs4.FATTR4_MIMETYPE - 32)) |
		(1 << (nfs4.FATTR4_MODE - 32)) |
		(1 << (nfs4.FATTR4_OWNER - 32)) |
		(1 << (nfs4.FATTR4_OWNER_GROUP - 32)) |
		(1 << (nfs4.FATTR4_SYSTEM - 32)) |
		(1 << (nfs4.FATTR4_TIME_ACCESS_SET - 32)) |
		(1 << (nfs4.FATTR4_TIME_BACKUP - 32)) |
		(1 << (nfs4.FATTR4_TIME_CREATE - 32)) |
		(1 << (nfs4.FATTR4_TIME_MODIFY_SET - 32))

	// Fixed FSID for the entire export.
	fsidMajor uint64 = 0x7E4F
	fsidMinor uint64 = 0

	// maxReadWrite is the advertised and enforced max READ/WRITE transfer size.
	maxReadWrite = 1 << 20
)

// attrMask holds the three bitmap4 words used by NFSv4.1 attributes.
type attrMask [3]uint32

// parseBitmap extracts up to three 32-bit words from a Bitmap4. Later words
// name no attribute nfsd knows.
func parseBitmap(bm nfs4.Bitmap4) attrMask {
	var mask attrMask
	for i := 0; i < len(mask) && i < int(bm.Count()); i++ {
		mask[i] = bm.Data(i)
	}
	return mask
}

// appendBitmap writes all three words of mask. READDIR predicts reply
// sizes from this fixed width.
func appendBitmap(w *nfs4.Bitmap4Writer, mask attrMask) {
	for _, word := range mask {
		w.AppendData(word)
	}
}

func validateGetattrMask(mask attrMask) uint32 {
	if mask[0]&invalidGetattrAttrs0 != 0 || mask[1]&invalidGetattrAttrs1 != 0 {
		return nfs4.NFS4ERR_INVAL
	}
	return nfs4.NFS4_OK
}

func validateVerifyMask(mask attrMask) uint32 {
	if mask[0]&invalidVerifyAttrs0 != 0 || mask[1]&invalidVerifyAttrs1 != 0 {
		return nfs4.NFS4ERR_INVAL
	}
	if mask[0]&^uint32(supportedAttrs0) != 0 ||
		mask[1]&^uint32(supportedAttrs1) != 0 ||
		mask[2]&^uint32(supportedAttrs2) != 0 {
		return nfs4.NFS4ERR_ATTRNOTSUPP
	}
	return nfs4.NFS4_OK
}

func validateCreateAttrs(attrs nfs4.Fattr4) uint32 {
	mask := parseBitmap(attrs.Attrmask())
	if mask[0]&^writableAttrs0 != 0 || mask[1]&^writableAttrs1 != 0 || mask[2] != 0 {
		return nfs4.NFS4ERR_INVAL
	}
	supported0 := (supportedAttrs0 | settableAttrs0) & writableAttrs0
	supported1 := (supportedAttrs1 | settableAttrs1) & writableAttrs1
	if mask[0]&^supported0 != 0 || mask[1]&^supported1 != 0 {
		return nfs4.NFS4ERR_ATTRNOTSUPP
	}
	return nfs4.NFS4_OK
}

func createAttrSize(attrs nfs4.Fattr4) (*uint64, uint32) {
	mask := parseBitmap(attrs.Attrmask())
	if mask[0]&(1<<nfs4.FATTR4_SIZE) == 0 {
		return nil, nfs4.NFS4_OK
	}
	data := attrs.AttrVals().Data()
	if len(data) < 8 {
		return nil, nfs4.NFS4ERR_BADXDR
	}
	size := binary.BigEndian.Uint64(data[:8])
	if size > 1<<63-1 {
		return nil, nfs4.NFS4ERR_FBIG
	}
	return &size, nfs4.NFS4_OK
}

// inodeNFSType converts an InodeID type to NFSv4 type constant.
func inodeNFSType(id vfs.InodeID) uint32 {
	switch id.Type() {
	case vfs.InodeTypeDir:
		return 2 // NF4DIR
	case vfs.InodeTypeSymlink:
		return 5 // NF4LNK
	default:
		return 1 // NF4REG
	}
}

// inodeNFSMode returns the NFS permission mode for an inode.
func inodeNFSMode(id vfs.InodeID) uint32 {
	switch id.Type() {
	case vfs.InodeTypeDir:
		return 0755
	case vfs.InodeTypeSymlink:
		return 0777
	default:
		return 0644
	}
}

// encodeAttrs encodes the requested attributes into an XDR byte buffer.
// Attributes MUST be encoded in order of their bit position.
func encodeAttrs(mask attrMask, id vfs.InodeID, ni vfs.NodeInfo) []byte {
	return appendAttrs(make([]byte, 0, 256), mask, id, ni)
}

// appendAttrs appends encodeAttrs's encoding to buf. For a given mask the
// encoding has a fixed length, so a buffer of that capacity never grows.
func appendAttrs(buf []byte, mask attrMask, id vfs.InodeID, ni vfs.NodeInfo) []byte {

	// Word 0 attributes (bits 0-31).
	if mask[0]&(1<<nfs4.FATTR4_SUPPORTED_ATTRS) != 0 {
		// Advertise readable + settable attrs so clients will send SETATTR for
		// the write-only ones (e.g. TIME_*_SET); these are never encoded as
		// values below.
		buf = binary.BigEndian.AppendUint32(buf, 3)
		buf = binary.BigEndian.AppendUint32(buf, supportedAttrs0|settableAttrs0)
		buf = binary.BigEndian.AppendUint32(buf, supportedAttrs1|settableAttrs1)
		buf = binary.BigEndian.AppendUint32(buf, supportedAttrs2)
	}
	if mask[0]&(1<<nfs4.FATTR4_TYPE) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, inodeNFSType(id))
	}
	if mask[0]&(1<<nfs4.FATTR4_FH_EXPIRE_TYPE) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.FH4_PERSISTENT)
	}
	if mask[0]&(1<<nfs4.FATTR4_CHANGE) != 0 {
		change := ni.Change
		if change == 0 {
			change = uint64(ni.Mtime.UnixNano())
		}
		buf = binary.BigEndian.AppendUint64(buf, change)
	}
	if mask[0]&(1<<nfs4.FATTR4_SIZE) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, ni.Size)
	}
	if mask[0]&(1<<nfs4.FATTR4_LINK_SUPPORT) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.FALSE)
	}
	if mask[0]&(1<<nfs4.FATTR4_SYMLINK_SUPPORT) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.TRUE)
	}
	if mask[0]&(1<<nfs4.FATTR4_NAMED_ATTR) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.FALSE)
	}
	if mask[0]&(1<<nfs4.FATTR4_FSID) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, fsidMajor)
		buf = binary.BigEndian.AppendUint64(buf, fsidMinor)
	}
	if mask[0]&(1<<nfs4.FATTR4_UNIQUE_HANDLES) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.TRUE)
	}
	if mask[0]&(1<<nfs4.FATTR4_LEASE_TIME) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, uint32(clientstore.LeaseTime/time.Second))
	}
	if mask[0]&(1<<nfs4.FATTR4_RDATTR_ERROR) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.NFS4_OK)
	}
	if mask[0]&(1<<nfs4.FATTR4_CANSETTIME) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.TRUE)
	}
	if mask[0]&(1<<nfs4.FATTR4_CASE_INSENSITIVE) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.FALSE)
	}
	if mask[0]&(1<<nfs4.FATTR4_CASE_PRESERVING) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.TRUE)
	}
	if mask[0]&(1<<nfs4.FATTR4_CHOWN_RESTRICTED) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.TRUE)
	}
	if mask[0]&(1<<nfs4.FATTR4_FILEHANDLE) != 0 {
		// opaque<NFS4_FHSIZE>: length + data
		buf = binary.BigEndian.AppendUint32(buf, 8)
		buf = append(buf, vfs.InodeIDToFH(id)...)
	}
	if mask[0]&(1<<nfs4.FATTR4_FILEID) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, id.Fileid())
	}
	if mask[0]&(1<<nfs4.FATTR4_FILES_AVAIL) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<40)
	}
	if mask[0]&(1<<nfs4.FATTR4_FILES_FREE) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<40)
	}
	if mask[0]&(1<<nfs4.FATTR4_FILES_TOTAL) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<40)
	}
	if mask[0]&(1<<nfs4.FATTR4_HOMOGENEOUS) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.TRUE)
	}
	if mask[0]&(1<<nfs4.FATTR4_MAXFILESIZE) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<63-1)
	}
	if mask[0]&(1<<nfs4.FATTR4_MAXLINK) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, 1) // no hard links
	}
	if mask[0]&(1<<nfs4.FATTR4_MAXNAME) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, uint32(maxTernNameLength))
	}
	if mask[0]&(1<<nfs4.FATTR4_MAXREAD) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, maxReadWrite)
	}
	if mask[0]&(1<<nfs4.FATTR4_MAXWRITE) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, maxReadWrite)
	}

	// Word 1 attributes (bits 32-63).
	if mask[1]&(1<<(nfs4.FATTR4_MODE-32)) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, inodeNFSMode(id))
	}
	if mask[1]&(1<<(nfs4.FATTR4_NO_TRUNC-32)) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, nfs4.TRUE)
	}
	if mask[1]&(1<<(nfs4.FATTR4_NUMLINKS-32)) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, 1)
	}
	if mask[1]&(1<<(nfs4.FATTR4_OWNER-32)) != 0 {
		buf = encodeString(buf, "nobody")
	}
	if mask[1]&(1<<(nfs4.FATTR4_OWNER_GROUP-32)) != 0 {
		buf = encodeString(buf, "nogroup")
	}
	if mask[1]&(1<<(nfs4.FATTR4_RAWDEV-32)) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, 0)
		buf = binary.BigEndian.AppendUint32(buf, 0)
	}
	if mask[1]&(1<<(nfs4.FATTR4_SPACE_AVAIL-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<50)
	}
	if mask[1]&(1<<(nfs4.FATTR4_SPACE_FREE-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<50)
	}
	if mask[1]&(1<<(nfs4.FATTR4_SPACE_TOTAL-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<50)
	}
	if mask[1]&(1<<(nfs4.FATTR4_SPACE_USED-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, ni.Size)
	}
	if mask[1]&(1<<(nfs4.FATTR4_TIME_ACCESS-32)) != 0 {
		sec, nsec := timeToNFS(ni.Atime)
		buf = binary.BigEndian.AppendUint64(buf, uint64(sec))
		buf = binary.BigEndian.AppendUint32(buf, nsec)
	}
	if mask[1]&(1<<(nfs4.FATTR4_TIME_DELTA-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 0)
		buf = binary.BigEndian.AppendUint32(buf, 1)
	}
	if mask[1]&(1<<(nfs4.FATTR4_TIME_METADATA-32)) != 0 {
		ctime := ni.Ctime
		if ctime.IsZero() {
			ctime = ni.Mtime
		}
		sec, nsec := timeToNFS(ctime)
		buf = binary.BigEndian.AppendUint64(buf, uint64(sec))
		buf = binary.BigEndian.AppendUint32(buf, nsec)
	}
	if mask[1]&(1<<(nfs4.FATTR4_TIME_MODIFY-32)) != 0 {
		sec, nsec := timeToNFS(ni.Mtime)
		buf = binary.BigEndian.AppendUint64(buf, uint64(sec))
		buf = binary.BigEndian.AppendUint32(buf, nsec)
	}
	if mask[1]&(1<<(nfs4.FATTR4_MOUNTED_ON_FILEID-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, id.Fileid())
	}
	if mask[2]&(1<<(nfs4.FATTR4_SUPPATTR_EXCLCREAT-64)) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, 0) // empty bitmap4
	}
	return buf
}

// encodeString appends an XDR string (length + data + pad to 4 bytes).
func encodeString(buf []byte, s string) []byte {
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(s)))
	buf = append(buf, s...)
	// Pad to 4-byte boundary.
	if pad := (4 - len(s)%4) % 4; pad > 0 {
		buf = append(buf, make([]byte, pad)...)
	}
	return buf
}

// stat supplies one attribute view to GETATTR, VERIFY and READDIR. Published
// handles never resolve to another open's private staging file.
func (s *Server) stat(id vfs.InodeID) (vfs.NodeInfo, error) {
	if sf := s.stagingStore.Get(id); sf != nil {
		return sf.Stat(), nil
	}
	return s.fs.Stat(id)
}

// readdirEntry holds attributes used to calculate exact XDR sizes.
type readdirEntry struct {
	vfs.DirEntry
	respMask attrMask
	attrData []byte // nil means Stat failed; encode empty attrs
}

// readdirEntryXDRSize returns the exact XDR byte count for one entry,
// including the leading present=TRUE(4) that precedes it in the list.
//
//	present(4) + cookie(8) + name_len(4) + name_padded + bitmap_count(4)
//	+ 3*bitmap_word(12) + attrvals_len(4) + attrdata + nextentry_present(4)
//
// The nextentry_present at the end is this entry's "next" pointer (TRUE or
// FALSE depending on whether another entry follows); its 4 bytes are included
// here so that each entry accounts for all the bytes it contributes.
func readdirEntryXDRSize(re *readdirEntry) int {
	namePadded := (len(re.Name) + 3) &^ 3
	var attrsSize int
	if re.attrData != nil {
		// bitmap: count(4) + 3 words(12) = 16; attrvals: len(4) + data
		attrsSize = 4 + 4*len(re.respMask) + 4 + len(re.attrData)
	} else {
		// Stat failed: empty bitmap count(4)=0 + empty attrvals len(4)=0
		attrsSize = 4 + 4
	}
	// present(4) + cookie(8) + name_len(4) + name_padded + attrs + nextentry(4)
	return 4 + 8 + 4 + namePadded + attrsSize + 4
}

// readdirEntryDirSize returns the directory-information bytes for dircount:
// cookie(8) + name XDR (len(4) + padded data).
func readdirEntryDirSize(re *readdirEntry) int {
	namePadded := (len(re.Name) + 3) &^ 3
	return 8 + 4 + namePadded
}

// Overhead within READDIR4resok that is always present:
// cookieverf(8) + entries_present_or_terminal_FALSE(4) + eof(4).
const readdirResokOverhead = 16

// dirlistWriter streams entries into a dirlist4. Each entry's nextentry
// discriminant is written with it, so the caller must know whether another
// entry follows.
type dirlistWriter struct {
	dir     nfs4.Dirlist4Writer
	ent     nfs4.Entry4Writer
	entries int
	name    []byte // reused for each entry's name
}

func (d *dirlistWriter) entry(re *readdirEntry, more bool) {
	if d.entries == 0 {
		d.ent = d.dir.SetEntries_True()
	}
	d.entries++
	d.ent.SetCookie(re.NameHash)
	d.name = append(d.name[:0], re.Name...)
	nameW := d.ent.StartName()
	d.ent.Resume(nameW.SetData(d.name).Finish())
	writeEntryAttrs(&d.ent, re)
	if more {
		d.ent = d.ent.SetNextentry_True()
		return
	}
	d.ent.SetNextentry_Default(nfs4.FALSE)
	d.dir.Resume(d.ent.Finish())
}

func (d *dirlistWriter) finish(eof bool) []byte {
	if d.entries == 0 {
		d.dir.SetEntries_Default(nfs4.FALSE)
	}
	if eof {
		d.dir.SetEof(nfs4.TRUE)
	} else {
		d.dir.SetEof(nfs4.FALSE)
	}
	return d.dir.Finish()
}

// encodeDirEntries writes directory entries into a Dirlist4Writer.
func encodeDirEntries(dirW *nfs4.Dirlist4Writer, entries []readdirEntry, eof bool) {
	d := dirlistWriter{dir: *dirW}
	for i := range entries {
		d.entry(&entries[i], i+1 < len(entries))
	}
	dirW.Resume(d.finish(eof))
}

func writeEntryAttrs(entW *nfs4.Entry4Writer, re *readdirEntry) {
	faw := entW.StartAttrs()
	if re.attrData == nil {
		entW.Resume(writeFattr(&faw, nil, nil))
		return
	}
	entW.Resume(writeFattr(&faw, re.respMask[:], re.attrData))
}

// writeFattr writes the supplied bitmap words and encoded attribute values.
func writeFattr(faw *nfs4.Fattr4Writer, mask []uint32, data []byte) []byte {
	bmW := faw.StartAttrmask()
	for _, word := range mask {
		bmW.AppendData(word)
	}
	faw.Resume(bmW.Finish())
	alW := faw.StartAttrVals()
	faw.Resume(alW.SetData(data).Finish())
	return faw.Finish()
}
