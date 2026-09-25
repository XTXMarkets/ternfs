// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"os"
	"time"
)

const maxBackgroundReadYield = 10 * time.Millisecond
const maxConcurrentHydrations = 2

func (s *Server) readBaseForeground(
	fileID InodeID,
	offset uint64,
	dest []byte,
) (int, bool, error) {
	s.foregroundReads.Add(1)
	defer s.foregroundReads.Add(-1)
	return s.fs.Read(fileID, offset, dest)
}

func (s *Server) readBaseBackground(
	cancel <-chan struct{},
	fileID InodeID,
	offset uint64,
	dest []byte,
) (int, bool, error) {
	// Give foreground reads a short head start, capped so a steady stream
	// of READs cannot starve background hydration. Both waits are cancellable.
	deadline := time.Now().Add(maxBackgroundReadYield)
	for s.foregroundReads.Load() != 0 && time.Now().Before(deadline) {
		timer := time.NewTimer(time.Millisecond)
		select {
		case <-cancel:
			timer.Stop()
			return 0, false, context.Canceled
		case <-timer.C:
		}
	}
	select {
	case <-cancel:
		return 0, false, context.Canceled
	case s.hydrationSlots <- struct{}{}:
	}
	defer func() { <-s.hydrationSlots }()
	select {
	case <-cancel:
		return 0, false, context.Canceled
	default:
	}
	return s.fs.Read(fileID, offset, dest)
}

func (s *Server) startHydration(sf StagingFile) {
	baseID, _ := sf.Base()
	if baseID != 0 {
		sf.StartHydration(s.readBaseBackground)
	}
}

func (s *Server) setStagingSize(sf StagingFile, size uint64) error {
	if err := sf.SetSize(size); err != nil {
		return err
	}
	if sf.Dirty() {
		s.startHydration(sf)
	}
	return sf.Sync()
}

func (s *Server) discardStaging(id InodeID) {
	meta, ok := s.stagingStore.GetMeta(id)
	s.stagingStore.Remove(id)
	if !ok {
		return
	}
	if err := s.fs.ScrapFile(id, meta.TernCookie); err != nil &&
		!os.IsNotExist(err) {
		s.log.Warn("scrap transient staging file", "inode", id, "err", err)
	}
	if meta.ClientID != 0 {
		if err := s.clients.RemoveOpen(meta.ClientID, meta.NFSStateID); err != nil &&
			!os.IsNotExist(err) {
			s.log.Warn("remove discarded staging open",
				"clientid", meta.ClientID, "err", err)
		}
	}
}

// Recheck ownership under the target lock: the lease snapshot may predate a
// reclaim or a namespace operation which detached this writer.
func (s *Server) retireStaging(id InodeID, meta StagingMeta) {
	current, ok, unlock := s.lockStagingTarget(id)
	defer unlock()
	if !ok || current.ClientID != meta.ClientID || current.NFSStateID != meta.NFSStateID {
		return
	}
	s.retireStagingLocked(id, current)
}

func (s *Server) retireStagingLocked(id InodeID, meta StagingMeta) {
	if meta.Unlinked {
		if meta.Retired {
			if err := s.stagingStore.Quarantine(id); err != nil {
				s.log.Warn("quarantine retired staging", "inode", id, "err", err)
			}
		} else {
			s.discardStaging(id)
		}
		return
	}
	if sf := s.stagingStore.Get(id); sf != nil {
		if err := sf.Retire(meta.ClientID, meta.NFSStateID); err != nil {
			s.log.Error("retain expired staging", "inode", id, "err", err)
		}
	}
}
