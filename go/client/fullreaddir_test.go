// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"strings"
	"testing"
	"xtx/ternfs/msgs"
)

func TestDirEdgesPageStopsBeforeCurrentSection(t *testing.T) {
	retained := msgs.Edge{
		TargetId:     msgs.MakeInodeIdExtra(msgs.MakeInodeId(msgs.FILE, 1, 10), true),
		NameHash:     1,
		Name:         "retained",
		CreationTime: 10,
	}
	current := msgs.Edge{
		Current:  true,
		TargetId: msgs.MakeInodeIdExtra(msgs.MakeInodeId(msgs.FILE, 1, 11), false),
		NameHash: 2,
		Name:     "current",
	}
	page, err := dirEdgesPage(
		&msgs.FullReadDirResp{Results: []msgs.Edge{retained, current}},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Results) != 1 || page.Results[0] != retained {
		t.Fatalf("results = %#v, want %#v", page.Results, []msgs.Edge{retained})
	}
	if page.Next != nil {
		t.Fatalf("next = %#v, want nil", page.Next)
	}
}

func TestSemanticDirEdgeAPIsRejectInvalidArguments(t *testing.T) {
	_, err := (&Client{}).ReadCurrentDirEdgesPage(
		nil,
		msgs.ROOT_DIR_INODE_ID,
		DirEdgesCursor{StartName: "name", StartTime: 1},
	)
	if err == nil || !strings.Contains(err.Error(), "snapshot time") {
		t.Fatalf("ReadCurrentDirEdgesPage() error = %v", err)
	}

	err = (&Client{}).WalkDirNameHistory(
		nil,
		msgs.ROOT_DIR_INODE_ID,
		"",
		func([]msgs.Edge) error { return nil },
	)
	if err == nil || err.Error() !=
		"FULL_READ_DIR_SAME_NAME requires a start name" {
		t.Fatalf("WalkDirNameHistory() error = %v", err)
	}
}
