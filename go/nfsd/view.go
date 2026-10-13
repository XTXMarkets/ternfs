// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Filehandles name the published inode a client looked up. A client's
// opens of a handle record what its I/O uses (openState.dataID): a writer's
// staging, or the version a reader opened. Operations without a stateid
// (GETATTR, COMMIT, special-stateid READ) use the client's view of the
// handle: its most recent writer, else the version its own CLOSE published.
//
// When a CLOSE publishes a replacement, the closing client's handle keeps
// showing the replacement until the client has no opens of it and one lease
// has passed. Close-to-open revalidation within that time sees the client's
// own write through the handle it holds; a later LOOKUP finds the
// replacement's own handle. The alias is process-local and bounded by
// recent publications; after it lapses the handle names its original,
// immutable version again.

type viewKey struct {
	clientID uint64
	fileID   vfs.InodeID
}

type publishedAlias struct {
	dataID  vfs.InodeID
	expires time.Time
}

type viewStore struct {
	mu      sync.Mutex
	aliases map[viewKey]publishedAlias
	// writers records the staging each client wrote through a handle, so
	// COMMIT, which has no stateid, still finds a failed writer's tombstone
	// after its CLOSE. Entries live as long as the process, like the
	// tombstones (bounded by maxTombstones).
	writers   map[viewKey][]vfs.InodeID
	writerIDs []viewKey
}

func (vs *viewStore) addWriter(key viewKey, dataID vfs.InodeID) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	if vs.writers == nil {
		vs.writers = make(map[viewKey][]vfs.InodeID)
	}
	for _, id := range vs.writers[key] {
		if id == dataID {
			return
		}
	}
	if _, ok := vs.writers[key]; !ok {
		vs.writerIDs = append(vs.writerIDs, key)
		if len(vs.writerIDs) > maxTombstones {
			delete(vs.writers, vs.writerIDs[0])
			vs.writerIDs = vs.writerIDs[1:]
		}
	}
	vs.writers[key] = append(vs.writers[key], dataID)
	if len(vs.writers[key]) > 8 {
		vs.writers[key] = vs.writers[key][1:]
	}
}

func (vs *viewStore) writersOf(key viewKey) []vfs.InodeID {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	return append([]vfs.InodeID(nil), vs.writers[key]...)
}

func (vs *viewStore) set(key viewKey, dataID vfs.InodeID, expires time.Time) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	if vs.aliases == nil {
		vs.aliases = make(map[viewKey]publishedAlias)
	}
	vs.aliases[key] = publishedAlias{dataID: dataID, expires: expires}
}

func (vs *viewStore) get(key viewKey) (vfs.InodeID, bool) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	alias, ok := vs.aliases[key]
	return alias.dataID, ok
}

// sweep drops aliases which have expired and whose handle no client of
// this nfsd still has open.
func (vs *viewStore) sweep(now time.Time, open func(viewKey) bool) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	for key, alias := range vs.aliases {
		if now.After(alias.expires) && !open(key) {
			delete(vs.aliases, key)
		}
	}
}

// clientWriter returns the staging inode of clientID's most recent write
// open of fileID, or fileID itself. Restart-recovered writers present
// their staging inode as the handle.
func (s *Server) clientWriter(clientID uint64, fileID vfs.InodeID) vfs.InodeID {
	for _, state := range s.opens.clientOpens(clientID, fileID) {
		if state.writes() {
			return state.dataID
		}
	}
	return fileID
}

// clientView returns the inode clientID sees through handle fileID.
func (s *Server) clientView(clientID uint64, fileID vfs.InodeID) vfs.InodeID {
	opens := s.opens.clientOpens(clientID, fileID)
	for _, state := range opens {
		if state.writes() {
			return state.dataID
		}
	}
	if dataID, ok := s.views.get(viewKey{clientID, fileID}); ok {
		return dataID
	}
	if len(opens) > 0 {
		return opens[0].dataID
	}
	return fileID
}

// publishedAs records that clientID's CLOSE published dataID over fileID.
func (s *Server) publishedAs(clientID uint64, fileID, dataID vfs.InodeID) {
	if fileID == dataID {
		return
	}
	s.views.set(viewKey{clientID, fileID}, dataID, s.clients.now().Add(nfsLeaseTime))
}

func (s *Server) sweepViews() {
	s.views.sweep(s.clients.now(), func(key viewKey) bool {
		return len(s.opens.clientOpens(key.clientID, key.fileID)) > 0
	})
}

// failedWriter reports whether a writer clientID used through fileID has
// failed. Operations without a usable stateid report its failure.
func (s *Server) failedWriter(clientID uint64, fileID vfs.InodeID) bool {
	for _, id := range s.views.writersOf(viewKey{clientID, fileID}) {
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
