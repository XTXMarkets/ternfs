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
	stagingID, ok := s.stagingStore.ResolveID(fileID)
	if !ok || stagingID != fileID {
		return nil
	}
	return s.stagingStore.Get(fileID)
}

func (s *Server) stagingTargetBusy(
	dirID InodeID,
	name string,
) (bool, error) {
	for {
		id, meta, ok := s.stagingStore.FindTarget(dirID, name)
		if !ok {
			return false, nil
		}

		if meta.ClientID != 0 {
			active, err := s.clients.HasOpen(meta.ClientID, meta.NFSStateID)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return true, err
			}
			if active {
				return true, nil
			}
		}

		s.log.Info("discarding stale staging file",
			"inode", id,
			"clientid", meta.ClientID,
			"target", name)
		s.discardStaging(id)
	}
}

func (s *Server) recoveredStagingTarget(
	dirID InodeID,
	name string,
	clientID uint64,
) (InodeID, StagingMeta, bool) {
	id, meta, ok := s.stagingStore.FindTarget(dirID, name)
	if !ok || meta.ClientID != clientID ||
		!s.opens.canRecover(meta.NFSStateID) {
		return 0, StagingMeta{}, false
	}
	return id, meta, true
}

func (s *Server) reapStaleStaging() {
	for id, meta := range s.stagingStore.Entries() {
		if meta.ClientID == 0 {
			continue
		}
		active, err := s.clients.HasOpen(meta.ClientID, meta.NFSStateID)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				s.log.Warn("check recovered staging lease",
					"inode", id, "err", err)
			}
			continue
		}
		if !active {
			s.discardStaging(id)
		}
	}
}
