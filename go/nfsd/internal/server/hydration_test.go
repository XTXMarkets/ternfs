// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func TestRemoveCancelsQueuedHydration(t *testing.T) {
	store, id, sf := staging.CreateOverlayStage(t, t.TempDir(), vfs.MakeInodeID(vfs.InodeTypeFile, 1), []byte("base"))
	srv := &Server{hydrationSlots: make(chan struct{}, 1)}
	srv.hydrationSlots <- struct{}{}
	entered := make(chan struct{})
	sf.StartHydration(func(cancel <-chan struct{}, id vfs.InodeID, offset uint64, dest []byte) (int, bool, error) {
		close(entered)
		return srv.readBaseBackground(cancel, id, offset, dest)
	})
	testutil.AwaitSignal(t, entered, "queued hydration")
	done := make(chan struct{})
	go func() { store.Remove(id); close(done) }()
	testutil.AwaitSignal(t, done, "cancelled queued hydration")
}
