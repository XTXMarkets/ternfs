// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"errors"
	"io"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// QuarantineDirName is the subdirectory of the staging directory which
// holds checkpoints whose metadata sidecar would not decode. Recovery moves
// them aside rather than deleting them; nfsd inspect lists them.
const QuarantineDirName = "quarantine"

var ErrRemoved = errors.New("staging file was removed")

type stagingTarget struct {
	dirID vfs.InodeID
	name  string
}

type stagingHydrationReader func(cancel <-chan struct{}, fileID vfs.InodeID, offset uint64, dest []byte) (int, bool, error)

type BaseReader func(
	fileID vfs.InodeID,
	offset uint64,
	dest []byte,
) (n int, eof bool, err error)

// File is the interface for locally staged file data during NFS writes.
// The store owns the file lifecycle; there is no Close method.
type File interface {
	Stat() vfs.NodeInfo
	SetTime(mtime, atime *time.Time) error
	SetSize(size uint64) error
	Write(offset uint64, data []byte) error
	Read(offset uint64, dest []byte, readBase BaseReader) (n int, eof bool, err error)
	Cache(offset uint64, data []byte) error
	Base() (id vfs.InodeID, size uint64)
	Dirty() bool
	DataChanged() bool
	Retire(clientID uint64, stateID nfs4.StateID) error
	StartHydration(readBase stagingHydrationReader)
	FinishHydration(readBase BaseReader) error
	Sync() error                    // persist to stable storage
	Reader() (io.ReadSeeker, error) // returns a reader positioned at offset 0
}

// Store manages staging files keyed by InodeID.
// All staging queries go through this interface — the NFS server
// does not maintain its own staging caches.
type Store interface {
	// ReadOnly returns true if no staging directory is configured.
	ReadOnly() bool
	// Create creates a new staging file with associated metadata.
	Create(id vfs.InodeID, meta Meta) (File, error)
	// Get returns the staging file for the given inode, or nil.
	Get(id vfs.InodeID) File
	// GetMeta returns the metadata sidecar for the given inode.
	GetMeta(id vfs.InodeID) (Meta, bool)
	// FindTargets returns the staging sessions indexed by dirID/name.
	FindTargets(dirID vfs.InodeID, name string) map[vfs.InodeID]Meta
	// Entries returns a snapshot of all staging metadata keyed by staging inode.
	Entries() map[vfs.InodeID]Meta
	// Rebind updates the owning client and stateid of a recovered staging file.
	Rebind(id vfs.InodeID, clientID uint64, stateID nfs4.StateID) error
	// Namespace updates preserve the current durable data checkpoint.
	SetGuard(id vfs.InodeID, guarded bool) error
	Detach(id vfs.InodeID) error
	Retarget(id vfs.InodeID, dirID vfs.InodeID, name string) error
	// Quarantine excludes an entry for the process lifetime and preserves its files.
	Quarantine(id vfs.InodeID) error
	Failed(id vfs.InodeID) bool
	// Remove closes and removes the staging file and sidecar for the given inode.
	Remove(id vfs.InodeID)
	// StagedSize returns the staged size for the given inode.
	// Returns (0, false) if the inode has no staging file.
	StagedSize(id vfs.InodeID) (uint64, bool)
}
