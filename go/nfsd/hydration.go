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

// Lease expiry revokes access, not stable data. Retired staging remains on
// disk for the original client to reclaim or an administrator to recover.
func (s *Server) retireStaging(id InodeID, meta StagingMeta) {
	if sf := s.stagingStore.Get(id); sf != nil {
		if err := sf.Retire(meta.ClientID, meta.NFSStateID); err != nil {
			s.log.Error("retain expired staging", "inode", id, "err", err)
		}
	}
}
