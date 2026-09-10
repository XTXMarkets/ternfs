// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"context"
	"testing"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestParwalkWalkFromInode(t *testing.T) {
	pool := NewParwalkPool(nil, nil, 1)
	defer pool.Close()

	file := msgs.MakeInodeId(msgs.FILE, 1, 1)
	var called bool
	err := pool.WalkFromInode(
		context.Background(),
		&ParwalkOptions{},
		file,
		"/some/file.txt",
		func(
			parent msgs.InodeId,
			parentPath string,
			name string,
			_ msgs.TernTime,
			id msgs.InodeId,
			current bool,
			owned bool,
		) error {
			called = true
			if parent != msgs.NULL_INODE_ID ||
				parentPath != "/some" ||
				name != "file.txt" ||
				id != file ||
				!current ||
				!owned {
				t.Fatalf(
					"callback got parent=%v parentPath=%q name=%q id=%v current=%v owned=%v",
					parent,
					parentPath,
					name,
					id,
					current,
					owned,
				)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("callback was not called")
	}
}
