// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package openstate

import (
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Filehandles name the published inode a client looked up. A client's
// opens of a handle record what its I/O uses (State.dataID): a writer's
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

type ViewKey struct {
	ClientID uint64
	FileID   vfs.InodeID
}

type publishedAlias struct {
	dataID  vfs.InodeID
	expires time.Time
}

type ViewStore struct {
	mu      sync.Mutex
	aliases map[ViewKey]publishedAlias
	// writers records the staging each client wrote through a handle, so
	// COMMIT, which has no stateid, still finds a failed writer's tombstone
	// after its CLOSE. Entries live as long as the process, like the
	// tombstones (bounded by maxTombstones).
	writers   map[ViewKey][]vfs.InodeID
	writerIDs []ViewKey
}

func (vs *ViewStore) AddWriter(key ViewKey, dataID vfs.InodeID) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	if vs.writers == nil {
		vs.writers = make(map[ViewKey][]vfs.InodeID)
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

func (vs *ViewStore) WritersOf(key ViewKey) []vfs.InodeID {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	return append([]vfs.InodeID(nil), vs.writers[key]...)
}

func (vs *ViewStore) Set(key ViewKey, dataID vfs.InodeID, expires time.Time) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	if vs.aliases == nil {
		vs.aliases = make(map[ViewKey]publishedAlias)
	}
	vs.aliases[key] = publishedAlias{dataID: dataID, expires: expires}
}

func (vs *ViewStore) Get(key ViewKey) (vfs.InodeID, bool) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	alias, ok := vs.aliases[key]
	return alias.dataID, ok
}

// sweep drops aliases which have expired and whose handle no client of
// this nfsd still has open.
func (vs *ViewStore) Sweep(now time.Time, open func(ViewKey) bool) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	for key, alias := range vs.aliases {
		if now.After(alias.expires) && !open(key) {
			delete(vs.aliases, key)
		}
	}
}
