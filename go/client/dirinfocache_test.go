// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"testing"
	"xtx/ternfs/core/assert"
	"xtx/ternfs/msgs"
)

func TestLRU(t *testing.T) {
	lru := &dirInfoCacheLru{}
	lru.init(3)

	null := msgs.NULL_INODE_ID

	// sanity
	assert.EqualSlice[msgs.InodeId](t, lru.toSlice(), []msgs.InodeId{null, null, null})
	assert.EqualSlice[msgs.InodeId](t, lru.toReverseSlice(), []msgs.InodeId{null, null, null})

	// from middle to front, after: 1, 0, 2
	{
		slot := int32(1)
		lru.slots[slot].id = msgs.InodeId(1)
		lru.remove(slot)
		lru.addToFront(slot)

		assert.EqualSlice[msgs.InodeId](t, lru.toSlice(), []msgs.InodeId{msgs.InodeId(1), null, null})
		assert.EqualSlice[msgs.InodeId](t, lru.toReverseSlice(), []msgs.InodeId{null, null, msgs.InodeId(1)})
	}

	// from front to front, after: 1, 0, 2
	{
		slot := int32(1)
		lru.slots[slot].id = msgs.InodeId(2)
		lru.remove(slot)
		lru.addToFront(slot)

		assert.EqualSlice[msgs.InodeId](t, lru.toSlice(), []msgs.InodeId{msgs.InodeId(2), null, null})
		assert.EqualSlice[msgs.InodeId](t, lru.toReverseSlice(), []msgs.InodeId{null, null, msgs.InodeId(2)})
	}

	// from back to front, after: 2, 1, 0
	{
		slot := int32(2)
		lru.slots[slot].id = msgs.InodeId(3)
		lru.remove(slot)
		lru.addToFront(slot)

		assert.EqualSlice[msgs.InodeId](t, lru.toSlice(), []msgs.InodeId{msgs.InodeId(3), msgs.InodeId(2), null})
		assert.EqualSlice[msgs.InodeId](t, lru.toReverseSlice(), []msgs.InodeId{null, msgs.InodeId(2), msgs.InodeId(3)})
	}
}
