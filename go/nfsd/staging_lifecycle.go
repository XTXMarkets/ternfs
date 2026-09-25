// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"os"
	"slices"
)

func (s *Server) stagingOwnedBy(
	fileID InodeID,
	state openState,
) StagingFile {
	meta, ok := s.stagingStore.GetMeta(fileID)
	if !ok || meta.NFSStateID != state.id ||
		meta.ClientID != state.owner.clientID {
		return nil
	}
	return s.stagingStore.Get(fileID)
}

func (s *Server) directStaging(fileID InodeID) StagingFile {
	return s.stagingStore.Get(fileID)
}

func (s *Server) stagingTargetBusy(dirID InodeID, name string) (bool, error) {
	return s.retireInactiveStagingTarget(dirID, name)
}

// The caller holds the pathname lock. Visit every writer, including inactive
// siblings of an active writer, before capturing roles for a mutation.
func (s *Server) retireInactiveStagingTarget(dirID InodeID, name string) (bool, error) {
	activeWriter := false
	for id, meta := range s.stagingStore.FindTargets(dirID, name) {
		if meta.Retired {
			continue
		}
		if meta.ClientID != 0 {
			active, err := s.clients.HasOpen(meta.ClientID, meta.NFSStateID)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return true, err
			}
			if active {
				activeWriter = true
				continue
			}
		}
		if err := s.retireStagingLocked(id, meta); err != nil && !errors.Is(err, errStagingRemoved) {
			return true, err
		}
	}
	return activeWriter, nil
}

// Only IDs and final namespace roles are captured. Data checkpoints may keep
// advancing while the mutation runs and must never be restored from a snapshot.
type namespaceWriter struct {
	id     InodeID
	target *mutationTarget // nil means detach on success
}

func (s *Server) namespaceWriters(dirID InodeID, name string, target *mutationTarget) []namespaceWriter {
	var writers []namespaceWriter
	for id := range s.stagingStore.FindTargets(dirID, name) {
		writers = append(writers, namespaceWriter{id: id, target: target})
	}
	slices.SortFunc(writers, func(a, b namespaceWriter) int {
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return 0
	})
	return writers
}

func (s *Server) guardNamespaceWriters(writers []namespaceWriter) error {
	for i, writer := range writers {
		err := s.stagingStore.SetGuard(writer.id, true)
		if err == nil || errors.Is(err, errStagingRemoved) {
			continue
		}
		// A failed save may already have installed the guard. Clear every
		// attempted writer in memory, including this one, before returning.
		for _, attempted := range writers[:i+1] {
			if clearErr := s.stagingStore.SetGuard(attempted.id, false); clearErr != nil && !errors.Is(clearErr, errStagingRemoved) {
				// Keep the writer usable: the next Sync repairs a stale disk guard,
				// or startup quarantines it if the process crashes before repair.
				s.log.Warn("cannot checkpoint cleared namespace guard", "inode", attempted.id, "err", clearErr)
			}
		}
		return err
	}
	return nil
}

// Writer disposition is separate from the reply: REMOVE edge-gone errors
// detach the captured writers but still return NOENT. Known outcomes survive
// final checkpoint failures; an unknown outcome fails only these writers.
func (s *Server) finishNamespaceWriters(writers []namespaceWriter, op uint32, backendErr error) uint32 {
	outcome := s.fs.ClassifyMutation(op, backendErr)
	for _, writer := range writers {
		var err error
		switch outcome {
		case mutationApplied, mutationDetachOnly:
			if writer.target == nil {
				err = s.stagingStore.Detach(writer.id)
			} else {
				err = s.stagingStore.Retarget(writer.id, writer.target.dirID, writer.target.name)
			}
		case mutationNotApplied:
			err = s.stagingStore.SetGuard(writer.id, false)
		case mutationUnknown:
			// We cannot learn whether this mutation applied. Fail the captured
			// writer rather than guess a target or replay an uncertain request.
			err = s.stagingStore.Quarantine(writer.id)
		}
		if err != nil && !errors.Is(err, errStagingRemoved) {
			// A known target stays usable despite a failed save: Sync repairs
			// the sidecar, and a surviving disk guard makes restart conservative.
			s.log.Warn("namespace staging bookkeeping failed", "inode", writer.id, "err", err)
		}
		meta, ok := s.stagingStore.GetMeta(writer.id)
		if ok && meta.Unlinked && meta.Retired {
			if err := s.stagingStore.Quarantine(writer.id); err != nil {
				s.log.Warn("quarantine detached retired writer", "inode", writer.id, "err", err)
			}
		}
	}
	if len(writers) > 0 && outcome == mutationUnknown {
		s.log.Warn("namespace outcome unknown; captured writers failed", "err", backendErr)
		return NFS4ERR_IO
	}
	return s.errToNFS(backendErr)
}

// lockStagingTarget waits for namespace operations on a writer's publication
// target. A rename may move the target while we wait, so check it again.
func (s *Server) lockStagingTarget(fileID InodeID) (StagingMeta, bool, func()) {
	for {
		meta, ok := s.stagingStore.GetMeta(fileID)
		if !ok || meta.Unlinked {
			return meta, ok, func() {}
		}
		target := mutationTarget{dirID: meta.DirID, name: meta.FileName}
		unlock := s.lockMutationTargets(target)
		current, ok := s.stagingStore.GetMeta(fileID)
		if !ok || current.Unlinked {
			unlock()
			return current, ok, func() {}
		}
		if current.DirID == target.dirID && current.FileName == target.name {
			return current, true, unlock
		}
		unlock()
	}
}

func (s *Server) recoveredStagingTarget(
	dirID InodeID,
	name string,
	clientID uint64,
	openOwner string,
	write bool,
) (InodeID, StagingMeta, bool, error) {
	key, err := s.clients.stagingRecoveryKey(clientID)
	if err != nil {
		return 0, StagingMeta{}, false, err
	}
	var foundID InodeID
	var foundMeta StagingMeta
	for id, meta := range s.stagingStore.FindTargets(dirID, name) {
		if meta.DirID != dirID || meta.FileName != name ||
			(meta.OwnerKnown && meta.OpenOwner != openOwner) ||
			meta.ReadOnly == write {
			continue
		}
		if meta.ClientID == clientID {
			if !s.opens.canRecover(meta.NFSStateID) {
				continue
			}
		} else {
			if !meta.OwnerKnown || key == ([32]byte{}) || meta.RecoveryKey != key {
				continue
			}
			expired, err := s.clients.IsLeaseExpired(meta.ClientID)
			if err != nil {
				return 0, StagingMeta{}, false, err
			}
			if !expired {
				continue
			}
		}
		// Never guess between multiple recovered sessions and hand one
		// writer another's data.
		if foundID != 0 {
			return 0, StagingMeta{}, false, nfsError(NFS4ERR_EXPIRED)
		}
		foundID, foundMeta = id, meta
	}
	return foundID, foundMeta, foundID != 0, nil
}

func (s *Server) exclusiveStagingTarget(
	dirID InodeID,
	name string,
	publishedID InodeID,
	owner openOwnerKey,
	verifier [8]byte,
) (InodeID, StagingMeta, bool) {
	for id, meta := range s.stagingStore.FindTargets(dirID, name) {
		// Another publication at this name invalidates the old create verifier.
		if meta.BaseID == publishedID &&
			meta.Exclusive && meta.Verifier == verifier &&
			meta.ClientID == owner.clientID &&
			meta.OwnerKnown && meta.OpenOwner == owner.owner {
			return id, meta, true
		}
	}
	return 0, StagingMeta{}, false
}
