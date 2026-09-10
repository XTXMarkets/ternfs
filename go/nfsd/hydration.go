// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"os"
	"runtime"
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
	fileID InodeID,
	offset uint64,
	dest []byte,
) (int, bool, error) {
	deadline := time.Now().Add(maxBackgroundReadYield)
	for s.foregroundReads.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	s.hydrationSlots <- struct{}{}
	defer func() {
		<-s.hydrationSlots
		runtime.Gosched()
	}()
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
	stagingID, resolved := s.stagingStore.ResolveID(id)
	meta, ok := s.stagingStore.GetMeta(id)
	s.stagingStore.Remove(id)
	if !ok {
		return
	}
	if !resolved {
		stagingID = id
	}
	if err := s.fs.ScrapFile(stagingID, meta.TernCookie); err != nil &&
		!os.IsNotExist(err) {
		s.log.Warn("scrap transient staging file", "inode", stagingID, "err", err)
	}
	if meta.ClientID != 0 {
		if err := s.clients.RemoveOpen(meta.ClientID, meta.NFSStateID); err != nil &&
			!os.IsNotExist(err) {
			s.log.Warn("remove discarded staging open",
				"clientid", meta.ClientID, "err", err)
		}
	}
}
