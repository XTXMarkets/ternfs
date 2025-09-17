// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cleanup

import (
	"testing"
	"time"
	"xtx/ternfs/core/assert"
	"xtx/ternfs/msgs"
)

func inodeId(id uint64, extra bool) msgs.InodeIdExtra {
	return msgs.MakeInodeIdExtra(msgs.MakeInodeId(msgs.FILE, 0, id), extra)
}

func date(day int) msgs.TernTime {
	return msgs.MakeTernTime(time.Date(2021, time.January, day, 0, 0, 0, 0, time.UTC))
}

func TestDeleteAll(t *testing.T) {
	edges := []msgs.Edge{
		{
			TargetId:     inodeId(1, false),
			NameHash:     0, // unneeded
			Name:         "f",
			CreationTime: date(1),
		},
		{
			TargetId:     msgs.InodeIdExtra(msgs.NULL_INODE_ID),
			NameHash:     0, // uneeded
			Name:         "f",
			CreationTime: date(2),
		},
	}
	assert.Equal(t, 2, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterVersions: msgs.ActiveDeleteAfterVersions(0)}, date(3), edges, 0))
	assert.Equal(t, 2, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(0)}, date(3), edges, 0))
}

func TestDeleteAfterVersions(t *testing.T) {
	edges := []msgs.Edge{
		{
			TargetId:     inodeId(1, false),
			NameHash:     0, // unneeded
			Name:         "f",
			CreationTime: date(1),
		},
		{
			TargetId:     inodeId(2, false),
			NameHash:     0, // unneeded
			Name:         "f",
			CreationTime: date(3),
		},
		{
			TargetId:     msgs.InodeIdExtra(msgs.NULL_INODE_ID),
			NameHash:     0, // unneeded
			Name:         "f",
			CreationTime: date(5),
		},
		{
			TargetId:     inodeId(3, false),
			NameHash:     0, // unneeded
			Name:         "f",
			CreationTime: date(7),
		},
	}
	assert.Equal(t, 0, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterVersions: msgs.ActiveDeleteAfterVersions(10)}, msgs.Now(), edges, 0))
	assert.Equal(t, 3, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterVersions: msgs.ActiveDeleteAfterVersions(1)}, msgs.Now(), edges, 0))
	assert.Equal(t, 1, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterVersions: msgs.ActiveDeleteAfterVersions(2)}, msgs.Now(), edges, 0)) // delete does not count
	/*
		// this falls on day 6, so between the delete and the last create. everything
		// apart from the last create should be deleted.
		assert.Equal(t, 3, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(24 * time.Hour * 2)}, date(8), edges))
		// this falls on day 4, so between the second create and the deleted. the
		// delete is deleted too (can't be kept hanging)
		assert.Equal(t, 3, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(24 * time.Hour * 4)}, date(8), edges))
		// this falls between the first and second create.
		assert.Equal(t, 1, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(24 * time.Hour * 6)}, date(8), edges))
		// this is well before anything
		assert.Equal(t, 0, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(48 * time.Hour * 10)}, date(8), edges))
		// doesn't crash with empty edges
		assert.Equal(t, 0, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(48 * time.Hour * 10)}, date(8), make([]msgs.Edge, 0)))
	*/
}

func TestDeleteAfterTime1(t *testing.T) {
	// This is the most common case. A file gets created, then deleted.
	edges := []msgs.Edge{
		{
			TargetId:     inodeId(1, true),
			NameHash:     0, // unneeded
			Name:         "f",
			CreationTime: date(1),
		},
		{
			TargetId:     msgs.InodeIdExtra(msgs.NULL_INODE_ID),
			NameHash:     0, // unneeded
			Name:         "f",
			CreationTime: date(3),
		},
	}
	// This falls on day 2, note that the edge owning the file is expired, but we still don't delete anything
	assert.Equal(t, 0, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(24 * time.Hour * 2)}, date(4), edges, 0))
	// Once the deletion edge expires, _then_ we delete
	assert.Equal(t, 2, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(24 * time.Hour * 2)}, date(10), edges, 0))
}

func TestDeleteAfterTime2(t *testing.T) {
	// Same as test 1, but with an override
	edges := []msgs.Edge{
		{
			TargetId:     inodeId(1, true),
			NameHash:     0, // unneeded
			Name:         "f",
			CreationTime: date(1),
		},
		{
			TargetId:     inodeId(2, true),
			NameHash:     0, // unneeded
			Name:         "f",
			CreationTime: date(3),
			Current:      true,
		},
	}
	// This falls on day 2, note that the edge owning the file is expired, but we still don't delete anything
	assert.Equal(t, 0, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(24 * time.Hour * 2)}, date(4), edges, time.Duration(0)))
	// Once the deletion edge expires, _then_ we delete, but only the snapshot
	// edge.
	assert.Equal(t, 1, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(24 * time.Hour * 2)}, date(10), edges, time.Duration(0)))
}

func TestDeleteAfterTime3(t *testing.T) {
	// This is the case where we only have one non-owned edge, very uncommon
	edges := []msgs.Edge{
		{
			TargetId:     inodeId(1, false),
			NameHash:     0, // unneeded
			Name:         "f",
			CreationTime: date(1),
		},
	}
	// This falls on day 2, note that the edge owning the file is expired, but we still don't delete anything
	assert.Equal(t, 1, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(24 * time.Hour * 2)}, date(4), edges, time.Duration(0)))
}

func TestDontDeleteCurrent(t *testing.T) {
	edges := []msgs.Edge{
		{
			TargetId:     inodeId(1, true),
			NameHash:     0,
			Name:         "f",
			CreationTime: date(1),
			Current:      true,
		},
	}
	assert.Equal(t, 0, edgesToRemove(msgs.NULL_INODE_ID, &msgs.SnapshotPolicy{DeleteAfterTime: msgs.ActiveDeleteAfterTime(time.Hour)}, date(5), edges, time.Duration(0)))
}
