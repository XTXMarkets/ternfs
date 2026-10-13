// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"os"
	"slices"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sidecar"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func (s *Server) stagingOwnedBy(
	fileID vfs.InodeID,
	state openState,
) staging.File {
	meta, ok := s.stagingStore.GetMeta(fileID)
	if !ok || meta.ClientID != state.owner.clientID ||
		meta.NFSStateID != state.id && state.dataID != fileID {
		return nil
	}
	return s.stagingStore.Get(fileID)
}

func (s *Server) directStaging(fileID vfs.InodeID) staging.File {
	return s.stagingStore.Get(fileID)
}

func (s *Server) stagingTargetBusy(dirID vfs.InodeID, name string) (bool, error) {
	return s.retireInactiveStagingTarget(dirID, name)
}

// The caller holds the pathname lock. Visit every writer, including inactive
// siblings of an active writer, before capturing roles for a mutation.
func (s *Server) retireInactiveStagingTarget(dirID vfs.InodeID, name string) (bool, error) {
	activeWriter := false
	for id, meta := range s.stagingStore.FindTargets(dirID, name) {
		if meta.Retired {
			continue
		}
		active, err := s.retireIfInactiveWriter(id, meta)
		if err != nil {
			return true, err
		}
		activeWriter = activeWriter || active
	}
	return activeWriter, nil
}

// retireIfInactiveWriter retires one writer whose open is gone. The check and
// the retirement run under one destruction hold on its client; a writer
// whose client a confirmation protects counts as active and is kept.
func (s *Server) retireIfInactiveWriter(id vfs.InodeID, meta sidecar.Meta) (bool, error) {
	end, ok := s.beginDestroy(meta.ClientID)
	if !ok {
		return true, nil
	}
	defer end()
	if meta.ClientID != 0 {
		active, err := s.clients.HasOpen(meta.ClientID, meta.NFSStateID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return true, err
		}
		if active {
			return true, nil
		}
	}
	if err := s.retireStagingLocked(id, meta); err != nil && !errors.Is(err, staging.ErrRemoved) {
		return true, err
	}
	return false, nil
}

// Only IDs and final namespace roles are captured. Data checkpoints may keep
// advancing while the mutation runs and must never be restored from a snapshot.
type namespaceWriter struct {
	id     vfs.InodeID
	target *mutationTarget // nil means detach on success
}

func (s *Server) namespaceWriters(dirID vfs.InodeID, name string, target *mutationTarget) []namespaceWriter {
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
		if err == nil || errors.Is(err, staging.ErrRemoved) {
			continue
		}
		// A failed save may already have installed the guard. Clear every
		// attempted writer in memory, including this one, before returning.
		for _, attempted := range writers[:i+1] {
			if clearErr := s.stagingStore.SetGuard(attempted.id, false); clearErr != nil && !errors.Is(clearErr, staging.ErrRemoved) {
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
		case vfs.MutationApplied, vfs.MutationDetachOnly:
			if writer.target == nil {
				err = s.stagingStore.Detach(writer.id)
			} else {
				err = s.stagingStore.Retarget(writer.id, writer.target.dirID, writer.target.name)
			}
		case vfs.MutationNotApplied:
			err = s.stagingStore.SetGuard(writer.id, false)
		case vfs.MutationUnknown:
			// We cannot learn whether this mutation applied. Fail the captured
			// writer rather than guess a target or replay an uncertain request.
			err = s.stagingStore.Quarantine(writer.id)
		}
		if err != nil && !errors.Is(err, staging.ErrRemoved) {
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
	if len(writers) > 0 && outcome == vfs.MutationUnknown {
		s.log.Warn("namespace outcome unknown; captured writers failed", "err", backendErr)
		return nfs4.NFS4ERR_IO
	}
	return s.errToNFS(backendErr)
}

// lockStagingTarget waits for namespace operations on a writer's publication
// target. A rename may move the target while we wait, so check it again.
func (s *Server) lockStagingTarget(fileID vfs.InodeID) (sidecar.Meta, bool, func()) {
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

// recoveredStagingTarget selects the staging a recovering OPEN adopts. Taking
// another incarnation's staging after its lease expired is destructive for
// that incarnation, so its destruction hold is taken before the lease check
// and returned in release, for the caller to call once the adoption is done.
// While a confirmation protects that incarnation, the OPEN gets DELAY.
// release is never nil.
func (s *Server) recoveredStagingTarget(
	dirID vfs.InodeID,
	name string,
	clientID uint64,
	openOwner string,
	write bool,
) (_ vfs.InodeID, _ sidecar.Meta, _ bool, release func(), err error) {
	holds := make(map[vfs.InodeID]func()) // by candidate staging
	release = func() {
		for id, end := range holds {
			end()
			delete(holds, id)
		}
	}
	// The holds stay this function's until it returns successfully: an
	// error or a panic (in a backend call or anywhere else) releases them.
	handedOff := false
	defer func() {
		if !handedOff {
			release()
		}
	}()
	key, err := s.clients.stagingRecoveryKey(clientID)
	if err != nil {
		return 0, sidecar.Meta{}, false, release, err
	}
	var foundID vfs.InodeID
	var foundMeta sidecar.Meta
	// Writers on one client share staging, so any of the client's open
	// owners may reclaim it after a server restart. Reclaim after lease
	// expiry, by a new incarnation, still requires the original owner.
	for id, meta := range s.stagingStore.FindTargets(dirID, name) {
		if meta.DirID != dirID || meta.FileName != name ||
			meta.ReadOnly == write {
			continue
		}
		if meta.ClientID != clientID && meta.OwnerKnown && meta.OpenOwner != openOwner {
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
			end, ok := s.beginDestroy(meta.ClientID)
			if !ok {
				// The other incarnation's expiry waits for its confirmation.
				return 0, sidecar.Meta{}, false, release, nfs4.Error(nfs4.NFS4ERR_DELAY)
			}
			holds[id] = end
			// The expiry decision is the gate's (canonical r3 §8); this
			// OPEN holds the pathname lock, so local expiry is the sweeper's.
			expired, err := s.decideExpired(meta.ClientID, false)
			if err != nil {
				return 0, sidecar.Meta{}, false, release, err
			}
			if !expired {
				end()
				delete(holds, id)
				continue
			}
		}
		// Never guess between multiple recovered sessions and hand one
		// writer another's data.
		if foundID != 0 {
			return 0, sidecar.Meta{}, false, release, nfs4.Error(nfs4.NFS4ERR_EXPIRED)
		}
		foundID, foundMeta = id, meta
	}
	// Only the selected staging can still hold one: every other expired
	// candidate made this return an error above.
	handedOff = true
	return foundID, foundMeta, foundID != 0, release, nil
}

// claimFHRecoveryCandidate preselects, without locks, the staged writer a
// writable CLAIM_FH of fileID recovers when this nfsd has no name for it
// (canonical r3 §6): typically the first OPEN after a restart. A candidate
// is a writable, linked, healthy staging entry of this incarnation whose
// stateid an earlier process issued, or another incarnation's with this
// client's RecoveryKey whose lease has expired, as recoveredStagingTarget
// accepts. A known owner must match. Its name must currently hold fileID.
//
// Exactly one candidate is returned with NFS4_OK; none gives STALE, as for
// any handle this nfsd cannot name, and more than one gives EXPIRED. The
// caller opens at the candidate's name, still as CLAIM_FH, and checks under
// the pathname lock that recoveredStagingTarget selects exactly this entry.
func (s *Server) claimFHRecoveryCandidate(
	fileID vfs.InodeID,
	clientID uint64,
	openOwner string,
) (vfs.InodeID, sidecar.Meta, uint32) {
	var key [32]byte
	keyRead := false
	var foundID vfs.InodeID
	var foundMeta sidecar.Meta
	for id, meta := range s.stagingStore.Entries() {
		if meta.ReadOnly || meta.Unlinked || s.stagingStore.Failed(id) {
			continue
		}
		if meta.OwnerKnown && meta.OpenOwner != openOwner {
			continue
		}
		crossIncarnation := meta.ClientID != clientID
		if !crossIncarnation {
			if !s.opens.canRecover(meta.NFSStateID) {
				continue
			}
		} else {
			if !meta.OwnerKnown {
				continue
			}
			if !keyRead {
				var err error
				if key, err = s.clients.stagingRecoveryKey(clientID); err != nil {
					return 0, sidecar.Meta{}, clientStoreErrToNFS(err)
				}
				keyRead = true
			}
			if key == ([32]byte{}) || meta.RecoveryKey != key {
				continue
			}
		}
		current, err := s.fs.Lookup(meta.DirID, meta.FileName)
		if err != nil {
			if s.errToNFS(err) == nfs4.NFS4ERR_NOENT {
				continue
			}
			return 0, sidecar.Meta{}, s.errToNFS(err)
		}
		if current != fileID {
			continue
		}
		if crossIncarnation {
			expired, status := s.recoveryLeaseExpired(meta.ClientID)
			if status != nfs4.NFS4_OK {
				return 0, sidecar.Meta{}, status
			}
			if !expired {
				continue
			}
		}
		if foundID != 0 {
			// Never guess which writer's data the client meant.
			return 0, sidecar.Meta{}, nfs4.NFS4ERR_EXPIRED
		}
		foundID, foundMeta = id, meta
	}
	if foundID == 0 {
		return 0, sidecar.Meta{}, nfs4.NFS4ERR_STALE
	}
	return foundID, foundMeta, nfs4.NFS4_OK
}

// recoveryLeaseExpired checks another incarnation's lease for preselection,
// through the same expiry decision as recoveredStagingTarget. The decision
// may publish its expired marker, so it holds a destruction hold, released
// when the check returns; a confirmation protecting the incarnation gives
// DELAY.
func (s *Server) recoveryLeaseExpired(clientID uint64) (bool, uint32) {
	end, ok := s.beginDestroy(clientID)
	if !ok {
		return false, nfs4.NFS4ERR_DELAY
	}
	defer end()
	expired, err := s.decideExpired(clientID, false)
	if err != nil {
		return false, clientStoreErrToNFS(err)
	}
	return expired, nfs4.NFS4_OK
}

// sameRecoveryCandidate reports whether recoveredStagingTarget, under the
// pathname lock, selected exactly the preselected staging, unchanged in its
// stateid, incarnation and owner, and still linked and healthy.
func (s *Server) sameRecoveryCandidate(
	wantID vfs.InodeID, want sidecar.Meta,
	gotID vfs.InodeID, got sidecar.Meta, found bool,
) bool {
	return found && gotID == wantID &&
		got.NFSStateID == want.NFSStateID && got.ClientID == want.ClientID &&
		got.OwnerKnown == want.OwnerKnown && got.OpenOwner == want.OpenOwner &&
		got.DirID == want.DirID && got.FileName == want.FileName &&
		!got.Unlinked && !s.stagingStore.Failed(gotID)
}

func (s *Server) exclusiveStagingTarget(
	dirID vfs.InodeID,
	name string,
	publishedID vfs.InodeID,
	owner openOwnerKey,
	verifier [8]byte,
) (vfs.InodeID, sidecar.Meta, bool) {
	for id, meta := range s.stagingStore.FindTargets(dirID, name) {
		// Another publication at this name invalidates the old create verifier.
		if meta.BaseID == publishedID &&
			meta.Exclusive && meta.Verifier == verifier &&
			meta.ClientID == owner.clientID &&
			meta.OwnerKnown && meta.OpenOwner == owner.owner {
			return id, meta, true
		}
	}
	return 0, sidecar.Meta{}, false
}
