// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"context"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

const maxBackgroundReadYield = 10 * time.Millisecond
const maxConcurrentHydrations = 2

func (s *Server) readBaseForeground(
	fileID vfs.InodeID,
	offset uint64,
	dest []byte,
) (int, bool, error) {
	s.foregroundReads.Add(1)
	defer s.foregroundReads.Add(-1)
	return s.fs.Read(fileID, offset, dest)
}

func (s *Server) readBaseBackground(
	cancel <-chan struct{},
	fileID vfs.InodeID,
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

func (s *Server) startHydration(sf staging.File) {
	baseID, _ := sf.Base()
	if baseID != 0 {
		sf.StartHydration(s.readBaseBackground)
	}
}

func (s *Server) setStagingSize(sf staging.File, size uint64) error {
	if err := sf.SetSize(size); err != nil {
		return err
	}
	if sf.Dirty() {
		s.startHydration(sf)
	}
	return sf.Sync()
}

// discardStaging scraps an unpublished staging's transient, best effort,
// and removes it.
func (s *Server) discardStaging(id vfs.InodeID) {
	_ = s.discardStagingFenced(id, false)
}

// fenceAndDiscardStaging is discardStaging for a staging whose candidate
// may still link: a transient error scrapping it keeps the staging and is
// returned.
func (s *Server) fenceAndDiscardStaging(id vfs.InodeID) error {
	return s.discardStagingFenced(id, true)
}

func (s *Server) discardStagingFenced(id vfs.InodeID, fence bool) error {
	meta, ok := s.stagingStore.GetMeta(id)
	if ok {
		var c staging.Candidate
		if sf := s.stagingStore.Get(id); sf != nil {
			c = sf.Candidate()
		}
		if err := s.scrapTransient(id, unpublished(id, meta, c)); err != nil && fence && c.ID != 0 {
			return err
		}
	}
	s.stagingStore.Remove(id)
	return nil
}

// Recheck ownership under the target lock: the lease snapshot may predate a
// reclaim or a namespace operation which detached this writer.
func (s *Server) retireStaging(id vfs.InodeID, meta staging.Meta) {
	current, ok, unlock := s.lockStagingTarget(id)
	defer unlock()
	if !ok || current.ClientID != meta.ClientID || current.NFSStateID != meta.NFSStateID {
		return
	}
	s.retireStagingLocked(id, current)
}

// retireStagingLocked retires the staging of an open whose lease ended,
// fencing its candidate first, best effort, unless an earlier retirement
// did.
func (s *Server) retireStagingLocked(id vfs.InodeID, meta staging.Meta) error {
	if meta.Unlinked {
		if meta.Retired {
			if err := s.stagingStore.Quarantine(id); err != nil {
				s.log.Warn("quarantine retired staging", "inode", id, "err", err)
				return err
			}
		} else {
			s.discardStaging(id)
		}
		return nil
	}
	if sf := s.stagingStore.Get(id); sf != nil {
		if c := sf.Candidate(); c.ID != 0 && !meta.Retired {
			_ = s.scrapTransient(id, c)
		}
		if err := sf.Retire(meta.ClientID, meta.NFSStateID); err != nil {
			s.log.Error("retain expired staging", "inode", id, "err", err)
			return err
		}
	}
	return nil
}
