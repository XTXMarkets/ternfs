// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package vfs

import "testing"

// The cache never holds more than its bound, evicts the least recently used
// entry, and counts a get as a use.
func TestLRUBound(t *testing.T) {
	c := newLRU[int, int](3)
	for i := range 3 {
		c.put(i, i*10)
	}
	if _, ok := c.get(0); !ok { // 0 is now the most recent
		t.Fatal("entry 0 missing")
	}
	c.put(3, 30) // evicts 1, the least recently used
	if _, ok := c.get(1); ok {
		t.Fatal("least recently used entry kept")
	}
	for _, k := range []int{0, 2, 3} {
		if v, ok := c.get(k); !ok || v != k*10 {
			t.Fatalf("entry %d = %d %t", k, v, ok)
		}
	}
	c.put(2, 21) // an update does not grow the cache
	c.remove(0)
	if c.len() != 2 {
		t.Fatalf("len %d, want 2", c.len())
	}
	for i := range 100 {
		c.put(100+i, i)
	}
	if c.len() != 3 || len(c.items) != 3 {
		t.Fatalf("len %d, items %d, want 3", c.len(), len(c.items))
	}
}

// Both of the remote backend's caches are bounded.
func TestRemoteCachesBounded(t *testing.T) {
	r := NewRemoteTernVFS(nil, nil, nil)
	for i := range maxCachedParents + 10 {
		r.parents.put(InodeID(i), 1)
	}
	for i := range maxCachedReaders + 10 {
		r.CacheEmptyReader(InodeID(i))
	}
	if r.parents.len() != maxCachedParents || r.readers.len() != maxCachedReaders {
		t.Fatalf("parents %d, readers %d", r.parents.len(), r.readers.len())
	}
	if _, ok := r.CachedParent(0); ok {
		t.Fatal("oldest parent kept past the bound")
	}
	if !r.ReaderCached(InodeID(maxCachedReaders + 9)) {
		t.Fatal("newest reader evicted")
	}
}
