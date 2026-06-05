// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
)

// Supported attribute bitmasks.
const (
	supportedAttrs0 = (1 << FATTR4_SUPPORTED_ATTRS) |
		(1 << FATTR4_TYPE) |
		(1 << FATTR4_FH_EXPIRE_TYPE) |
		(1 << FATTR4_CHANGE) |
		(1 << FATTR4_SIZE) |
		(1 << FATTR4_LINK_SUPPORT) |
		(1 << FATTR4_SYMLINK_SUPPORT) |
		(1 << FATTR4_NAMED_ATTR) |
		(1 << FATTR4_FSID) |
		(1 << FATTR4_UNIQUE_HANDLES) |
		(1 << FATTR4_LEASE_TIME) |
		(1 << FATTR4_RDATTR_ERROR) |
		(1 << FATTR4_CANSETTIME) |
		(1 << FATTR4_CASE_INSENSITIVE) |
		(1 << FATTR4_CASE_PRESERVING) |
		(1 << FATTR4_CHOWN_RESTRICTED) |
		(1 << FATTR4_FILEHANDLE) |
		(1 << FATTR4_FILEID) |
		(1 << FATTR4_FILES_AVAIL) |
		(1 << FATTR4_FILES_FREE) |
		(1 << FATTR4_FILES_TOTAL) |
		(1 << FATTR4_HOMOGENEOUS) |
		(1 << FATTR4_MAXFILESIZE) |
		(1 << FATTR4_MAXLINK) |
		(1 << FATTR4_MAXNAME) |
		(1 << FATTR4_MAXREAD) |
		(1 << FATTR4_MAXWRITE)

	supportedAttrs1 = (1 << (FATTR4_MODE - 32)) |
		(1 << (FATTR4_NO_TRUNC - 32)) |
		(1 << (FATTR4_NUMLINKS - 32)) |
		(1 << (FATTR4_OWNER - 32)) |
		(1 << (FATTR4_OWNER_GROUP - 32)) |
		(1 << (FATTR4_RAWDEV - 32)) |
		(1 << (FATTR4_SPACE_AVAIL - 32)) |
		(1 << (FATTR4_SPACE_FREE - 32)) |
		(1 << (FATTR4_SPACE_TOTAL - 32)) |
		(1 << (FATTR4_SPACE_USED - 32)) |
		(1 << (FATTR4_TIME_ACCESS - 32)) |
		(1 << (FATTR4_TIME_DELTA - 32)) |
		(1 << (FATTR4_TIME_METADATA - 32)) |
		(1 << (FATTR4_TIME_MODIFY - 32)) |
		(1 << (FATTR4_MOUNTED_ON_FILEID - 32))

	// Fixed FSID for the entire export.
	fsidMajor uint64 = 0x7E4F
	fsidMinor uint64 = 0

	// maxReadWrite is the advertised and enforced max READ/WRITE transfer size.
	maxReadWrite = 1 << 20
)

// parseBitmap extracts up to two 32-bit words from a Bitmap4 into a fixed array.
func parseBitmap(bm Bitmap4) [2]uint32 {
	var mask [2]uint32
	if bm.Count() > 0 {
		mask[0] = bm.Data(0)
	}
	if bm.Count() > 1 {
		mask[1] = bm.Data(1)
	}
	return mask
}

// inodeNFSType converts an InodeID type to NFSv4 type constant.
func inodeNFSType(id InodeID) uint32 {
	switch id.Type() {
	case InodeTypeDir:
		return 2 // NF4DIR
	case InodeTypeSymlink:
		return 5 // NF4LNK
	default:
		return 1 // NF4REG
	}
}

// inodeNFSMode returns the NFS permission mode for an inode.
func inodeNFSMode(id InodeID) uint32 {
	switch id.Type() {
	case InodeTypeDir:
		return 0755
	case InodeTypeSymlink:
		return 0777
	default:
		return 0644
	}
}

// encodeAttrs encodes the requested attributes into an XDR byte buffer.
// Attributes MUST be encoded in order of their bit position.
func encodeAttrs(mask [2]uint32, id InodeID, ni NodeInfo) []byte {
	buf := make([]byte, 0, 256)

	// Word 0 attributes (bits 0-31).
	if mask[0]&(1<<FATTR4_SUPPORTED_ATTRS) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, 2)
		buf = binary.BigEndian.AppendUint32(buf, supportedAttrs0)
		buf = binary.BigEndian.AppendUint32(buf, supportedAttrs1)
	}
	if mask[0]&(1<<FATTR4_TYPE) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, inodeNFSType(id))
	}
	if mask[0]&(1<<FATTR4_FH_EXPIRE_TYPE) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, FH4_PERSISTENT)
	}
	if mask[0]&(1<<FATTR4_CHANGE) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, uint64(ni.Mtime.UnixNano()))
	}
	if mask[0]&(1<<FATTR4_SIZE) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, ni.Size)
	}
	if mask[0]&(1<<FATTR4_LINK_SUPPORT) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, FALSE)
	}
	if mask[0]&(1<<FATTR4_SYMLINK_SUPPORT) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, TRUE)
	}
	if mask[0]&(1<<FATTR4_NAMED_ATTR) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, FALSE)
	}
	if mask[0]&(1<<FATTR4_FSID) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, fsidMajor)
		buf = binary.BigEndian.AppendUint64(buf, fsidMinor)
	}
	if mask[0]&(1<<FATTR4_UNIQUE_HANDLES) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, TRUE)
	}
	if mask[0]&(1<<FATTR4_LEASE_TIME) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, 90) // 90 seconds
	}
	if mask[0]&(1<<FATTR4_RDATTR_ERROR) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, NFS4_OK)
	}
	if mask[0]&(1<<FATTR4_CANSETTIME) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, TRUE)
	}
	if mask[0]&(1<<FATTR4_CASE_INSENSITIVE) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, FALSE)
	}
	if mask[0]&(1<<FATTR4_CASE_PRESERVING) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, TRUE)
	}
	if mask[0]&(1<<FATTR4_CHOWN_RESTRICTED) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, TRUE)
	}
	if mask[0]&(1<<FATTR4_FILEHANDLE) != 0 {
		// opaque<NFS4_FHSIZE>: length + data
		buf = binary.BigEndian.AppendUint32(buf, 8)
		buf = append(buf, inodeIDToFH(id)...)
	}
	if mask[0]&(1<<FATTR4_FILEID) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, id.Fileid())
	}
	if mask[0]&(1<<FATTR4_FILES_AVAIL) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<40)
	}
	if mask[0]&(1<<FATTR4_FILES_FREE) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<40)
	}
	if mask[0]&(1<<FATTR4_FILES_TOTAL) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<40)
	}
	if mask[0]&(1<<FATTR4_HOMOGENEOUS) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, TRUE)
	}
	if mask[0]&(1<<FATTR4_MAXFILESIZE) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<63-1)
	}
	if mask[0]&(1<<FATTR4_MAXLINK) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, 1) // no hard links
	}
	if mask[0]&(1<<FATTR4_MAXNAME) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, 255)
	}
	if mask[0]&(1<<FATTR4_MAXREAD) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, maxReadWrite)
	}
	if mask[0]&(1<<FATTR4_MAXWRITE) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, maxReadWrite)
	}

	// Word 1 attributes (bits 32-63).
	if mask[1]&(1<<(FATTR4_MODE-32)) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, inodeNFSMode(id))
	}
	if mask[1]&(1<<(FATTR4_NO_TRUNC-32)) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, TRUE)
	}
	if mask[1]&(1<<(FATTR4_NUMLINKS-32)) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, 1)
	}
	if mask[1]&(1<<(FATTR4_OWNER-32)) != 0 {
		buf = encodeString(buf, "nobody")
	}
	if mask[1]&(1<<(FATTR4_OWNER_GROUP-32)) != 0 {
		buf = encodeString(buf, "nogroup")
	}
	if mask[1]&(1<<(FATTR4_RAWDEV-32)) != 0 {
		buf = binary.BigEndian.AppendUint32(buf, 0)
		buf = binary.BigEndian.AppendUint32(buf, 0)
	}
	if mask[1]&(1<<(FATTR4_SPACE_AVAIL-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<50)
	}
	if mask[1]&(1<<(FATTR4_SPACE_FREE-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<50)
	}
	if mask[1]&(1<<(FATTR4_SPACE_TOTAL-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 1<<50)
	}
	if mask[1]&(1<<(FATTR4_SPACE_USED-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, ni.Size)
	}
	if mask[1]&(1<<(FATTR4_TIME_ACCESS-32)) != 0 {
		sec, nsec := timeToNFS(ni.Atime)
		buf = binary.BigEndian.AppendUint64(buf, uint64(sec))
		buf = binary.BigEndian.AppendUint32(buf, nsec)
	}
	if mask[1]&(1<<(FATTR4_TIME_DELTA-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, 0)
		buf = binary.BigEndian.AppendUint32(buf, 1)
	}
	if mask[1]&(1<<(FATTR4_TIME_METADATA-32)) != 0 {
		// ctime = mtime (TernFS has no separate ctime)
		sec, nsec := timeToNFS(ni.Mtime)
		buf = binary.BigEndian.AppendUint64(buf, uint64(sec))
		buf = binary.BigEndian.AppendUint32(buf, nsec)
	}
	if mask[1]&(1<<(FATTR4_TIME_MODIFY-32)) != 0 {
		sec, nsec := timeToNFS(ni.Mtime)
		buf = binary.BigEndian.AppendUint64(buf, uint64(sec))
		buf = binary.BigEndian.AppendUint32(buf, nsec)
	}
	if mask[1]&(1<<(FATTR4_MOUNTED_ON_FILEID-32)) != 0 {
		buf = binary.BigEndian.AppendUint64(buf, id.Fileid())
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

// stagedSizes maps InodeID to the staged size for files being written.
// Passed through the dir entry encoding chain so READDIR reflects staged sizes.
type stagedSizes map[InodeID]uint64

// readdirEntry holds a directory entry with pre-computed attribute data
// so that exact XDR sizes can be calculated before encoding.
type readdirEntry struct {
	DirEntry
	respMask [2]uint32
	attrData []byte // nil means Stat failed; encode empty attrs
}

// prepareReaddirEntries pre-computes attribute data for each directory entry.
func prepareReaddirEntries(entries []DirEntry, reqMask [2]uint32, vfs TernVFS, ss stagedSizes) []readdirEntry {
	var respMask [2]uint32
	respMask[0] = reqMask[0] & supportedAttrs0
	respMask[1] = reqMask[1] & supportedAttrs1

	result := make([]readdirEntry, len(entries))
	for i, e := range entries {
		result[i].DirEntry = e
		result[i].respMask = respMask
		ni, err := vfs.Stat(e.ID)
		if err != nil {
			continue // attrData stays nil → empty attrs
		}
		if sz, ok := ss[e.ID]; ok {
			ni.Size = sz
		}
		result[i].attrData = encodeAttrs(respMask, e.ID, ni)
	}
	return result
}

// readdirEntryXDRSize returns the exact XDR byte count for one entry,
// including the leading present=TRUE(4) that precedes it in the list.
//
//	present(4) + cookie(8) + name_len(4) + name_padded + bitmap_count(4)
//	+ 2*bitmap_word(8) + attrvals_len(4) + attrdata + nextentry_present(4)
//
// The nextentry_present at the end is this entry's "next" pointer (TRUE or
// FALSE depending on whether another entry follows); its 4 bytes are included
// here so that each entry accounts for all the bytes it contributes.
func readdirEntryXDRSize(re *readdirEntry) int {
	namePadded := (len(re.Name) + 3) &^ 3
	var attrsSize int
	if re.attrData != nil {
		// bitmap: count(4) + 2 words(8) = 12; attrvals: len(4) + data
		attrsSize = 12 + 4 + len(re.attrData)
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

// encodeDirEntries writes directory entries into a Dirlist4Writer.
func encodeDirEntries(dirW *Dirlist4Writer, entries []readdirEntry, eof bool) {
	if len(entries) == 0 {
		dirW.SetEntries_Default(FALSE)
	} else {
		entW := dirW.SetEntries_True()
		writeEntryChain(&entW, dirW, entries, 0)
	}
	if eof {
		dirW.SetEof(TRUE)
	} else {
		dirW.SetEof(FALSE)
	}
}

func writeEntryChain(entW *Entry4Writer, dirW *Dirlist4Writer, entries []readdirEntry, idx int) {
	re := &entries[idx]
	entW.SetCookie(re.NameHash)

	nameW := entW.StartName()
	buf := nameW.SetData([]byte(re.Name)).Finish()
	entW.Resume(buf)

	writeEntryAttrs(entW, re)

	if idx+1 < len(entries) {
		nextW := entW.SetNextentry_True()
		writeEntryChain(&nextW, dirW, entries, idx+1)
	} else {
		entW.SetNextentry_Default(FALSE)
		buf = entW.Finish()
		dirW.Resume(buf)
	}
}

func writeEntryAttrs(entW *Entry4Writer, re *readdirEntry) {
	faw := entW.StartAttrs()
	if re.attrData == nil {
		writeEmptyAttrs(&faw, entW)
		return
	}
	bmW := faw.StartAttrmask()
	bmW.AppendData(re.respMask[0])
	bmW.AppendData(re.respMask[1])
	buf := bmW.Finish()
	faw.Resume(buf)
	alW := faw.StartAttrVals()
	buf = alW.SetData(re.attrData).Finish()
	faw.Resume(buf)
	buf = faw.Finish()
	entW.Resume(buf)
}

func writeEmptyAttrs(faw *Fattr4Writer, entW *Entry4Writer) {
	bmW := faw.StartAttrmask()
	buf := bmW.Finish()
	faw.Resume(buf)
	alW := faw.StartAttrVals()
	buf = alW.SetData(nil).Finish()
	faw.Resume(buf)
	buf = faw.Finish()
	entW.Resume(buf)
}
