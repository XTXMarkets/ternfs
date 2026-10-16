// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"encoding/hex"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Diagnostics snapshots of the staging store, in the inspect API's types.
// A staging file's lock is only tried, since disk operations hold it
// across I/O; a busy file is reported locked. The server composes them.

// The API's text for ids; see inspect.Scalar and inspect.ID.
func inodeText(id vfs.InodeID) string    { return inspect.Scalar(inspect.KindInode, uint64(id)) }
func incarnationText(id uint64) string   { return inspect.Scalar(inspect.KindIncarnation, id) }
func stateidText(id nfs4.StateID) string { return inspect.ID(inspect.KindStateid, id[:]) }

func (sf *LocalFile) snapshot(id vfs.InodeID, target stagingTarget) inspect.StagingEntry {
	out := inspect.StagingEntry{FileID: inodeText(id), DirID: inodeText(target.dirID), FileName: target.name}
	sf.hydrateMu.Lock()
	done := sf.hydrateDone
	sf.hydrateMu.Unlock()
	if done != nil {
		select {
		case <-done:
		default:
			out.Hydrating = true
		}
	}
	if !sf.mu.TryLock() {
		out.Locked = true
		return out
	}
	defer sf.mu.Unlock()
	meta := sf.meta
	out.Incarnation = incarnationText(meta.ClientID)
	out.Stateid = stateidText(meta.NFSStateID)
	out.OwnerHex = hex.EncodeToString([]byte(meta.OpenOwner))
	out.OwnerKnown = meta.OwnerKnown
	out.Size = sf.size
	if meta.BaseID != 0 {
		out.BaseID = inodeText(meta.BaseID)
	}
	out.BaseSize = meta.BaseSize
	out.DirtyRanges = len(sf.dirty)
	for _, r := range sf.dirty {
		out.DirtyBytes += r.End - r.Start
	}
	out.HydratedRanges = len(sf.hydrated)
	out.Unlinked, out.Guarded, out.Retired = meta.Unlinked, meta.Guarded, meta.Retired
	out.ReadOnly, out.Exclusive = meta.ReadOnly, meta.Exclusive
	out.MetadataChanged = sf.metadataChanged
	out.CheckpointDirty = sf.checkpointDirty
	out.Removed = sf.removed
	return out
}

// Diagnostics: every entry as /v1/staging reports it, and the failed
// tombstones. Each file's lock is only tried, since it is held across disk
// I/O; a busy file reports its identity and target only.
func (s *LocalStore) Snapshot() ([]inspect.StagingEntry, []string) {
	type picked struct {
		id     vfs.InodeID
		entry  *localStagingEntry
		target stagingTarget
	}
	s.mu.Lock()
	var files []picked
	failed := []string{}
	for id, entry := range s.files {
		if entry.failed {
			failed = append(failed, inodeText(id))
			continue
		}
		files = append(files, picked{id, entry, entry.target})
	}
	s.mu.Unlock()
	records := make([]inspect.StagingEntry, 0, len(files))
	for _, p := range files {
		records = append(records, p.entry.file.snapshot(p.id, p.target))
	}
	return records, failed
}

// SnapshotStateid finds the entry recorded for stateid id (StateidSource).
func (s *LocalStore) SnapshotStateid(id nfs4.StateID) StateidSnapshot {
	s.mu.Lock()
	files := make(map[vfs.InodeID]*LocalFile)
	for fileID, entry := range s.files {
		if !entry.failed {
			files[fileID] = entry.file
		}
	}
	s.mu.Unlock()
	var out StateidSnapshot
	for fileID, file := range files {
		if !file.mu.TryLock() {
			out.Incomplete = true
			continue
		}
		matches := file.meta.NFSStateID == id
		clientID := file.meta.ClientID
		file.mu.Unlock()
		if matches {
			return StateidSnapshot{FileID: fileID, ClientID: clientID, Found: true}
		}
	}
	return out
}

type StateidSnapshot struct {
	FileID     vfs.InodeID
	ClientID   uint64
	Found      bool
	Incomplete bool
}

// StateidSource is optional diagnostics support. It must not wait on
// a file lock held across I/O. A negative result is incomplete if any
// potentially matching entry could not be inspected.
type StateidSource interface {
	SnapshotStateid(nfs4.StateID) StateidSnapshot
}

// Dir returns the staging directory.
func (s *LocalStore) Dir() string { return s.dir }
