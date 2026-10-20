// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/openstate"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// clientWriter returns the staging inode of clientID's most recent write
// open of fileID, or fileID itself. Restart-recovered writers present
// their staging inode as the handle.
func (s *Server) clientWriter(clientID uint64, fileID vfs.InodeID) vfs.InodeID {
	for _, state := range s.opens.ClientOpens(clientID, fileID) {
		if state.Writes() {
			return state.DataID
		}
	}
	return fileID
}

// clientView returns the inode clientID sees through handle fileID.
func (s *Server) clientView(clientID uint64, fileID vfs.InodeID) vfs.InodeID {
	opens := s.opens.ClientOpens(clientID, fileID)
	for _, state := range opens {
		if state.Writes() {
			return state.DataID
		}
	}
	if dataID, ok := s.views.Get(openstate.ViewKey{ClientID: clientID, FileID: fileID}); ok {
		return dataID
	}
	if len(opens) > 0 {
		return opens[0].DataID
	}
	return fileID
}

// publishedAs records that clientID's CLOSE published dataID over fileID.
func (s *Server) publishedAs(clientID uint64, fileID, dataID vfs.InodeID) {
	if fileID == dataID {
		return
	}
	s.aliasMu.Lock()
	defer s.aliasMu.Unlock()
	expires := s.clients.Now().Add(clientstore.LeaseTime)
	s.views.Set(openstate.ViewKey{ClientID: clientID, FileID: fileID}, dataID, expires)
	// Persisted so that the client's view survives an nfsd restart; see
	// alias_log.go.
	if err := s.aliases.append(aliasRecord{ClientID: clientID, FileID: uint64(fileID), DataID: uint64(dataID), Expires: expires}); err != nil {
		s.log.Warn("publication aliases: append", "err", err)
	}
}

// viewHeld reports that the alias's client still has its handle open,
// which keeps the alias past its expiry.
func (s *Server) viewHeld(key openstate.ViewKey) bool {
	return len(s.opens.ClientOpens(key.ClientID, key.FileID)) > 0
}

func (s *Server) sweepViews() {
	s.views.Sweep(s.clients.Now(), s.viewHeld)
	s.compactAliases()
}

// failedWriter reports whether a writer clientID used through fileID has
// failed. Operations without a usable stateid report its failure.
func (s *Server) failedWriter(clientID uint64, fileID vfs.InodeID) bool {
	for _, id := range s.views.WritersOf(openstate.ViewKey{ClientID: clientID, FileID: fileID}) {
		if s.stagingStore.Failed(id) {
			return true
		}
	}
	return false
}

// nameOf returns the directory and name at which fileID is currently
// linked, if this nfsd has seen it there.
func (s *Server) nameOf(fileID vfs.InodeID) (vfs.InodeID, string, bool) {
	target, ok := s.names.lookup(fileID)
	if !ok {
		return 0, "", false
	}
	current, err := s.fs.Lookup(target.dirID, target.name)
	if err != nil || current != fileID {
		return 0, "", false
	}
	return target.dirID, target.name, true
}
