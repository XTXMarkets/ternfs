// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package vfs

import (
	"github.com/XTXMarkets/ternfs/go/msgs"
)

// Test hooks: nfsd tests inspect and perturb the backends' caches.

// CachedParent returns the parent cached for id.
func (t *RemoteTernVFS) CachedParent(id InodeID) (InodeID, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	parent, ok := t.parents[id]
	return parent, ok
}

// ForgetParent drops the parent cached for id.
func (t *RemoteTernVFS) ForgetParent(id InodeID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.parents, id)
}

// ReaderCached reports whether a file reader is cached for id.
func (t *RemoteTernVFS) ReaderCached(id InodeID) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.readers[msgs.InodeId(id)]
	return ok
}

// CacheEmptyReader caches an empty file reader for id.
func (t *RemoteTernVFS) CacheEmptyReader(id InodeID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.readers[msgs.InodeId(id)] = &cachedReader{}
}

// ForgetParent drops the parent recorded for id.
func (lfs *LocalTernVFS) ForgetParent(id InodeID) {
	lfs.mu.Lock()
	defer lfs.mu.Unlock()
	delete(lfs.parent, id)
}

// TransientCount returns the number of transient files not yet linked.
func (lfs *LocalTernVFS) TransientCount() int {
	lfs.mu.RLock()
	defer lfs.mu.RUnlock()
	return len(lfs.transient)
}
