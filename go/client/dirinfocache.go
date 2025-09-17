// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"bytes"
	"fmt"
	"math/bits"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
	"xtx/ternfs/msgs"
)

type dirInfoKey struct {
	id  msgs.InodeId
	tag msgs.DirectoryInfoTag
}

const (
	dirInfoCacheBuckets        = 128   // must be a power of two
	dirInfoCacheTargetHeapSize = 300e6 // 300MB
)

type dirInfoCacheLruSlot struct {
	id   msgs.InodeId
	prev int32
	next int32
}

type dirInfoCacheInheritedFrom struct {
	id       msgs.InodeId
	cachedAt msgs.TernTime
	lruSlot  int32
}

type dirInfoCacheLru struct {
	slots []dirInfoCacheLruSlot
	first int32 // most recently used
	last  int32 // least recently used
}

func (lru *dirInfoCacheLru) init(slots int32) {
	lru.first = 0
	lru.last = slots - 1
	lru.slots = make([]dirInfoCacheLruSlot, slots)
	for j := int32(0); j < slots; j++ {
		lru.slots[j].next = j + 1
		if lru.slots[j].next >= slots {
			lru.slots[j].next = 0
		}
		lru.slots[j].prev = j - 1
		if lru.slots[j].prev < 0 {
			lru.slots[j].prev = slots - 1
		}
	}
}

func (lru *dirInfoCacheLru) remove(slot int32) {
	lru.slots[lru.slots[slot].prev].next = lru.slots[slot].next
	lru.slots[lru.slots[slot].next].prev = lru.slots[slot].prev

	if slot == lru.first {
		lru.first = lru.slots[slot].next
	}
	if slot == lru.last {
		lru.last = lru.slots[slot].prev
	}
}

func (lru *dirInfoCacheLru) addToFront(slot int32) {
	lru.slots[slot].next = lru.first
	lru.slots[slot].prev = lru.slots[lru.first].prev

	lru.slots[lru.slots[lru.first].prev].next = slot
	lru.slots[lru.first].prev = slot

	lru.first = slot
}

func (lru *dirInfoCacheLru) toSlice() []msgs.InodeId {
	ids := []msgs.InodeId{}
	slot := lru.first
	for {
		ids = append(ids, lru.slots[slot].id)
		if lru.last == slot {
			break
		}
		slot = lru.slots[slot].next
	}
	return ids
}

func (lru *dirInfoCacheLru) toReverseSlice() []msgs.InodeId {
	ids := []msgs.InodeId{}
	slot := lru.last
	for {
		ids = append(ids, lru.slots[slot].id)
		if lru.first == slot {
			break
		}
		slot = lru.slots[slot].prev
	}
	return ids
}

type dirInfoCacheBucket struct {
	mu    sync.Mutex
	lru   dirInfoCacheLru
	cache map[dirInfoKey]dirInfoCacheInheritedFrom
}

type cachedDirInfoEntry struct {
	cachedAt msgs.TernTime
	entry    msgs.IsDirectoryInfoEntry
	packed   [256]byte // for easy comparison
}

type DirInfoCache struct {
	// This changes extremely infrequently, and has very few members
	// -- just the directories that have a policy set, which is few.
	policies   map[dirInfoKey]*cachedDirInfoEntry
	policiesMu sync.RWMutex

	buckets [dirInfoCacheBuckets]dirInfoCacheBucket

	lookups uint64
	hits    uint64
}

func dirInfoCachePickBucket(id msgs.InodeId) uint64 {
	hi, _ := bits.Mul64(uint64(id), 0xda942042e4dd58b5)
	return hi & (dirInfoCacheBuckets - 1)
}

func NewDirInfoCache() *DirInfoCache {
	elements := int32(dirInfoCacheTargetHeapSize / (unsafe.Sizeof(dirInfoKey{}) + unsafe.Sizeof(dirInfoCacheInheritedFrom{}) + unsafe.Sizeof(dirInfoCacheLruSlot{})))
	elementsPerBucket := (elements + dirInfoCacheBuckets - 1) / dirInfoCacheBuckets

	c := &DirInfoCache{
		policies: make(map[dirInfoKey]*cachedDirInfoEntry),
	}
	for i := 0; i < dirInfoCacheBuckets; i++ {
		c.buckets[i].cache = make(map[dirInfoKey]dirInfoCacheInheritedFrom)
		c.buckets[i].lru.init(elementsPerBucket)
	}

	return c
}

// If it couldn't be found, will return NULL_INODE_ID.
// Otherwise that's where it was inherited from.
func (env *DirInfoCache) LookupCachedDirInfoEntry(dirId msgs.InodeId, entry msgs.IsDirectoryInfoEntry) msgs.InodeId {
	key := dirInfoKey{id: dirId, tag: entry.Tag()}
	now := msgs.Now()
	atomic.AddUint64(&env.lookups, 1)

	// Check if we have a cached thing at all
	var inheritedFrom msgs.InodeId
	{
		bucket := &env.buckets[dirInfoCachePickBucket(dirId)]

		bucket.mu.Lock()
		v, found := bucket.cache[key]

		if !found {
			bucket.mu.Unlock()
			return msgs.NULL_INODE_ID
		}

		if time.Duration(int64(now)-int64(v.cachedAt)) > time.Hour {
			bucket.mu.Unlock()
			return msgs.NULL_INODE_ID
		}

		// put at the front
		bucket.lru.remove(v.lruSlot)
		bucket.lru.addToFront(v.lruSlot)

		bucket.mu.Unlock()

		inheritedFrom = v.id
	}

	// lookup policy
	{
		env.policiesMu.RLock()
		v, found := env.policies[dirInfoKey{id: inheritedFrom, tag: entry.Tag()}]
		env.policiesMu.RUnlock()

		if !found {
			panic(fmt.Errorf("could not find policy for id=%v tag=%v", inheritedFrom, entry.Tag()))
		}

		if time.Duration(int64(now)-int64(v.cachedAt)) > time.Hour {
			return msgs.NULL_INODE_ID
		}

		switch entry.Tag() {
		case msgs.SNAPSHOT_POLICY_TAG:
			*(entry.(*msgs.SnapshotPolicy)) = *(v.entry.(*msgs.SnapshotPolicy))
		case msgs.BLOCK_POLICY_TAG:
			*(entry.(*msgs.BlockPolicy)) = *(v.entry.(*msgs.BlockPolicy))
		case msgs.SPAN_POLICY_TAG:
			*(entry.(*msgs.SpanPolicy)) = *(v.entry.(*msgs.SpanPolicy))
		case msgs.STRIPE_POLICY_TAG:
			*(entry.(*msgs.StripePolicy)) = *(v.entry.(*msgs.StripePolicy))
		default:
			panic(fmt.Errorf("bad entry tag %v", entry.Tag()))
		}

		atomic.AddUint64(&env.hits, 1)
		return inheritedFrom
	}
}

// This must be called before UpdateInheritedFrom.
func (env *DirInfoCache) UpdatePolicy(dirId msgs.InodeId, entry msgs.IsDirectoryInfoEntry) {
	// pack
	buf := bytes.NewBuffer(make([]byte, 256)[:1])
	if err := entry.Pack(buf); err != nil {
		panic(fmt.Errorf("could not pack policy: %v", err))
	}
	if len(buf.Bytes()) > 256 {
		panic(fmt.Errorf("overrun packed policy %+v, len=%v %+v", entry, len(buf.Bytes()), buf.Bytes()))
	}
	var packed [256]byte
	copy(packed[:], buf.Bytes())

	// update, if necessary
	key := dirInfoKey{id: dirId, tag: entry.Tag()}
	env.policiesMu.RLock()
	cachedEntry, found := env.policies[key]
	env.policiesMu.RUnlock()

	now := msgs.Now()
	if !found || cachedEntry.packed != packed { // unlikely
		env.policiesMu.Lock()
		env.policies[key] = &cachedDirInfoEntry{
			entry:    entry,
			packed:   packed,
			cachedAt: now,
		}
		env.policiesMu.Unlock()
	} else {
		atomic.StoreUint64((*uint64)(&cachedEntry.cachedAt), uint64(now))
	}
}

func (env *DirInfoCache) UpdateInheritedFrom(dirId msgs.InodeId, tag msgs.DirectoryInfoTag, inheritedFrom msgs.InodeId) {
	now := msgs.Now()

	bucket := &env.buckets[dirInfoCachePickBucket(dirId)]
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// kick out last element, and use it to store the current one
	slot := bucket.lru.last
	bucket.lru.remove(slot)

	// remove all cached entries for the kicked out directory
	if bucket.lru.slots[slot].id != msgs.NULL_INODE_ID {
		for _, tag := range msgs.DirectoryInfoTags {
			delete(bucket.cache, dirInfoKey{id: bucket.lru.slots[slot].id, tag: tag})
		}
	}

	// add new element as the first element
	bucket.lru.slots[slot].id = dirId
	bucket.lru.addToFront(slot)

	// update map
	bucket.cache[dirInfoKey{id: dirId, tag: tag}] = dirInfoCacheInheritedFrom{
		id:       inheritedFrom,
		cachedAt: now,
		lruSlot:  slot,
	}
}

func (env *DirInfoCache) Hits() uint64 {
	return env.hits
}

func (env *DirInfoCache) Lookups() uint64 {
	return env.lookups
}
