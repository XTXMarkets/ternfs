// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package vfs

import (
	"encoding/binary"
	"io"
	"time"
)

// InodeID is a 64-bit inode identifier. Bits 62-61 encode the inode type
// (matching TernFS InodeId layout).
type InodeID uint64

const (
	InodeTypeDir     = 1
	InodeTypeFile    = 2
	InodeTypeSymlink = 3
)

// NFSDirName is the directory under the root that holds nfsd's own state.
const NFSDirName = ".nfs"

func (id InodeID) Type() uint64   { return (uint64(id) >> 61) & 0x3 }
func (id InodeID) Fileid() uint64 { return uint64(id) & ((1 << 61) - 1) }

func MakeInodeID(typ uint64, ino uint64) InodeID {
	return InodeID((typ << 61) | (ino & ((1 << 61) - 1)))
}

// Cookie is an opaque 8-byte token returned by ConstructFile. It must be
// passed to all subsequent operations on a transient file (matching TernFS
// semantics where the cookie prevents other clients from interfering with
// an in-progress write).
type Cookie [8]byte

// Edge identifies a directory entry as LookupEdge observed it: the inode it
// names and the creation time of the entry itself. TernFS pins a mutation to
// both. The pin does not establish the outcome of a retransmitted request.
type Edge struct {
	ID           InodeID
	CreationTime uint64
}

type MutationOutcome uint8

const (
	MutationApplied MutationOutcome = iota
	MutationDetachOnly
	MutationNotApplied
	MutationUnknown
)

// NodeInfo holds metadata returned by Stat.
type NodeInfo struct {
	Size   uint64    // 0 for directories
	Mtime  time.Time // modification time
	Atime  time.Time // access time (same as Mtime for directories in TernFS)
	Ctime  time.Time // defaults to Mtime for immutable backend inodes
	Change uint64    // defaults to Mtime.UnixNano for immutable backend inodes
}

// DirEntry is one entry from a directory listing.
type DirEntry struct {
	Name     string
	ID       InodeID
	NameHash uint64
}

// TernVFS is the filesystem abstraction layer. This matches the operations
// available from TernFS closely enough for a real implementation later.
type TernVFS interface {
	RootID() InodeID

	// Stat returns metadata for a file, directory, or symlink.
	Stat(id InodeID) (NodeInfo, error)

	// Lookup finds a child by name in a directory.
	Lookup(dirID InodeID, name string) (InodeID, error)

	// LookupParent returns the parent directory of the given inode.
	LookupParent(id InodeID) (InodeID, error)

	// Readdir lists directory entries starting at the given name hash.
	// Returns entries and the NextHash continuation cursor (0 = EOF).
	// Matches TernFS ReadDirReq semantics.
	Readdir(dirID InodeID, startHash uint64) ([]DirEntry, uint64, error)

	// Read reads file data into dest, returning bytes read and EOF flag.
	Read(fileID InodeID, offset uint64, dest []byte) (n int, eof bool, err error)

	// ReadAll reads a small internal file without retaining reader state.
	ReadAll(fileID InodeID) ([]byte, error)

	// Readlink reads the target of a symlink.
	Readlink(fileID InodeID) (string, error)

	// Mkdir creates a directory. Returns the new directory's InodeID.
	Mkdir(dirID InodeID, name string) (InodeID, error)

	// Symlink creates a symlink. Returns the new symlink's InodeID.
	Symlink(dirID InodeID, name string, target string) (InodeID, error)

	// ConstructFile creates a transient file (not yet visible in any
	// directory). Returns the file's InodeID and a Cookie that must be
	// provided for subsequent operations on the file. dirID is the
	// intended target directory (used for shard selection in TernFS).
	// Matches TernFS ConstructFileReq semantics.
	ConstructFile(dirID InodeID) (InodeID, Cookie, error)

	// ScrapFile discards an unlinked transient file.
	ScrapFile(fileID InodeID, cookie Cookie) error

	// LinkFile links a transient file into a directory, making it visible.
	// The correct cookie (from ConstructFile) must be provided. data is the
	// file content; in real TernFS the data was already written via AddSpan,
	// but LocalTernVFS writes it at link time.
	LinkFile(fileID InodeID, cookie Cookie, dirID InodeID, name string, data io.Reader) error

	// CreateFile creates or replaces a regular file with the given data.
	// This matches TernFS LinkFile semantics. It is used for internal
	// bookkeeping files, not NFS file creation.
	CreateFile(dirID InodeID, name string, data io.Reader) (InodeID, error)

	// Remove removes a file, directory, or symlink by name from a directory.
	// A removed file remains readable by inode until it is collected.
	Remove(dirID InodeID, name string) error

	// Rename moves/renames a directory entry. A replaced file remains
	// readable by inode until it is collected.
	Rename(srcDirID InodeID, srcName string, dstDirID InodeID, dstName string) error

	// LookupEdge finds a child by name and returns the entry it was found
	// through, so that a later mutation can be pinned to it.
	LookupEdge(dirID InodeID, name string) (Edge, error)

	// RemoveEdge removes the pinned entry and returns the raw backend error.
	RemoveEdge(dirID InodeID, name string, edge Edge) error

	// RenameEdge moves the entry only while srcName still refers to edge, with
	// the same error contract as RemoveEdge.
	RenameEdge(srcDirID InodeID, srcName string, edge Edge, dstDirID InodeID, dstName string) error

	// ClassifyMutation describes the entire submitted call, including retries.
	// The original error is mapped to the NFS reply separately.
	ClassifyMutation(op uint32, err error) MutationOutcome

	// SetTime sets the mtime and/or atime of a file or directory.
	// A nil pointer means "don't change this field."
	SetTime(id InodeID, mtime *time.Time, atime *time.Time) error
}

// InodeIDToFH converts an InodeID to an 8-byte NFS file handle.
func InodeIDToFH(id InodeID) []byte {
	var fh [8]byte
	binary.BigEndian.PutUint64(fh[:], uint64(id))
	return fh[:]
}

// FHToInodeID converts an 8-byte NFS file handle back to an InodeID.
func FHToInodeID(fh []byte) (InodeID, bool) {
	if len(fh) != 8 {
		return 0, false
	}
	return InodeID(binary.BigEndian.Uint64(fh)), true
}
