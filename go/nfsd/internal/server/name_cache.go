// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"sync"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

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
