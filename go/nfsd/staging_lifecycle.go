// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"os"
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
	for id, meta := range s.stagingStore.FindTargets(dirID, name) {
		if meta.ClientID != 0 {
			active, err := s.clients.HasOpen(meta.ClientID, meta.NFSStateID)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return true, err
			}
			if active {
				return true, nil
			}
		}
		s.retireStaging(id, meta)
	}
	return false, nil
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
		// Legacy sidecars have no open-owner identity. Never guess between
		// multiple recovered sessions and hand one writer another's data.
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
