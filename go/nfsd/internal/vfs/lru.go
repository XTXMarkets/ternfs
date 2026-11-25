// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package vfs

import "container/list"

// lru is a map bounded to max entries that evicts the least recently used
// one. The caller serializes access.
type lru[K comparable, V any] struct {
	max   int
	items map[K]*list.Element
	order *list.List // front is the most recent
}

type lruEntry[K comparable, V any] struct {
	key K
	val V
}

func newLRU[K comparable, V any](max int) *lru[K, V] {
	return &lru[K, V]{max: max, items: make(map[K]*list.Element), order: list.New()}
}

func (c *lru[K, V]) get(key K) (V, bool) {
	e, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToFront(e)
	return e.Value.(*lruEntry[K, V]).val, true
}

func (c *lru[K, V]) put(key K, val V) {
	if e, ok := c.items[key]; ok {
		e.Value.(*lruEntry[K, V]).val = val
		c.order.MoveToFront(e)
		return
	}
	c.items[key] = c.order.PushFront(&lruEntry[K, V]{key: key, val: val})
	if c.order.Len() > c.max {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*lruEntry[K, V]).key)
	}
}

func (c *lru[K, V]) remove(key K) {
	if e, ok := c.items[key]; ok {
		c.order.Remove(e)
		delete(c.items, key)
	}
}

func (c *lru[K, V]) len() int { return c.order.Len() }
