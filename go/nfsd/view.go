// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"sync"

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
	s.views.Set(openstate.ViewKey{ClientID: clientID, FileID: fileID}, dataID, s.clients.Now().Add(clientstore.LeaseTime))
}

func (s *Server) sweepViews() {
	s.views.Sweep(s.clients.Now(), func(key openstate.ViewKey) bool {
		return len(s.opens.ClientOpens(key.ClientID, key.FileID)) > 0
	})
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

// names remembers where this nfsd last saw a file, for opens by
// filehandle. A write needs the name it will publish at; TernFS cannot
// find a file's name from its inode. Entries are hints, checked against
// the directory before use, and bounded.
type nameCache struct {
	mu      sync.Mutex
	entries map[vfs.InodeID]mutationTarget
	order   []vfs.InodeID
}

const maxNameCache = 1 << 16

func (nc *nameCache) remember(id vfs.InodeID, dirID vfs.InodeID, name string) {
	if id.Type() == vfs.InodeTypeDir {
		return
	}
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if nc.entries == nil {
		nc.entries = make(map[vfs.InodeID]mutationTarget)
	}
	if _, ok := nc.entries[id]; !ok {
		nc.order = append(nc.order, id)
		if len(nc.order) > maxNameCache {
			delete(nc.entries, nc.order[0])
			nc.order = nc.order[1:]
		}
	}
	nc.entries[id] = mutationTarget{dirID: dirID, name: name}
}

func (nc *nameCache) lookup(id vfs.InodeID) (mutationTarget, bool) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	target, ok := nc.entries[id]
	return target, ok
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
