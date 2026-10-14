// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package keyedlock

import (
	"sync"
)

type lockEntry struct {
	mu   sync.Mutex
	refs int
}

// Locker is a mutex per key. Entries exist only while a key is held or
// awaited. The zero value is ready to use.
type Locker[K comparable] struct {
	mu      sync.Mutex
	entries map[K]*lockEntry
}

func (l *Locker[K]) ref(key K) *lockEntry {
	l.mu.Lock()
	if l.entries == nil {
		l.entries = make(map[K]*lockEntry)
	}
	entry := l.entries[key]
	if entry == nil {
		entry = &lockEntry{}
		l.entries[key] = entry
	}
	entry.refs++
	l.mu.Unlock()
	return entry
}

func (l *Locker[K]) unref(key K, entry *lockEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry.refs--
	if entry.refs == 0 {
		delete(l.entries, key)
	}
}

func (l *Locker[K]) Lock(key K) func() {
	entry := l.ref(key)
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		l.unref(key, entry)
	}
}

func (l *Locker[K]) TryLock(key K) (func(), bool) {
	entry := l.ref(key)
	if !entry.mu.TryLock() {
		l.unref(key, entry)
		return nil, false
	}
	return func() {
		entry.mu.Unlock()
		l.unref(key, entry)
	}, true
}

// Refs returns the number of holders and waiters for key. For tests.
func (l *Locker[K]) Refs(key K) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry := l.entries[key]; entry != nil {
		return entry.refs
	}
	return 0
}

// Contended reports whether any key has a waiter. For tests.
func (l *Locker[K]) Contended() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, entry := range l.entries {
		if entry.refs > 1 {
			return true
		}
	}
	return false
}
