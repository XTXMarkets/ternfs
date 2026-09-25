// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type replayParentGateVFS struct {
	TernVFS
	id      atomic.Uint64
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (fs *replayParentGateVFS) LookupParent(id InodeID) (InodeID, error) {
	if id == InodeID(fs.id.Load()) && fs.calls.Add(1) == 1 {
		close(fs.entered)
		<-fs.release
	}
	return fs.TernVFS.LookupParent(id)
}

func TestOpenReplayCannotRecreateClosedMarker(t *testing.T) {
	base := NewLocalTernVFS(t.TempDir())
	if _, err := base.CreateFile(base.RootID(), "file", strings.NewReader("data")); err != nil {
		t.Fatal(err)
	}
	fs := &replayParentGateVFS{TernVFS: base,
		entered: make(chan struct{}), release: make(chan struct{})}
	store, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	defer closeSignal(fs.release)
	a, b := dial(t, addr), dial(t, addr)
	defer a.Close()
	defer b.Close()
	xid := uint32(1)
	clientID := setupClient(t, a, &xid)
	status, sid, fh, _, err := openFileForOwnerE(a, &xid, clientID, "owner", 1,
		"file", OPEN4_SHARE_ACCESS_READ, false, false)
	if err != nil || status != NFS4_OK {
		t.Fatalf("OPEN = %d, %v", status, err)
	}
	confirmOpenState(t, a, &xid, fh, 2, sid)
	status, sid, fh, _, err = openFileForOwnerE(a, &xid, clientID, "owner", 3,
		"file", OPEN4_SHARE_ACCESS_READ, false, false)
	if err != nil || status != NFS4_OK {
		t.Fatalf("second OPEN = %d, %v", status, err)
	}
	// The populated client cache makes IsConfirmed use its fast path, so
	// this gate is in MarkOpen, before it acquires the identity lock.
	srv.waitForClientGC()
	fs.id.Store(clientID)
	replayed := make(chan error, 1)
	go func() {
		status, _, _, _, err := openFileForOwnerE(a, &xid, clientID, "owner", 3,
			"file", OPEN4_SHARE_ACCESS_READ, false, false)
		if err == nil && status != NFS4_OK {
			err = nfsError(status)
		}
		replayed <- err
	}()
	awaitSignal(t, fs.entered, "replay before marker identity lock")
	closed := make(chan closeResult, 1)
	go func() {
		closeXID := uint32(100)
		status, _, err := closeFileWithSeqResultE(b, &closeXID, fh, sid, 4)
		closed <- closeResult{status: status, err: err}
	}()
	select {
	case r := <-closed:
		t.Fatalf("CLOSE overtook replay marker renewal: %d, %v", r.status, r.err)
	case <-time.After(25 * time.Millisecond):
	}
	closeSignal(fs.release)
	if err := awaitValue(t, replayed, "OPEN replay"); err != nil {
		t.Fatal(err)
	}
	if r := awaitValue(t, closed, "CLOSE"); r.err != nil || r.status != NFS4_OK {
		t.Fatalf("CLOSE = %d, %v", r.status, r.err)
	}
	if got := activeOpenMarkerCount(t, srv, clientID); got != 0 {
		t.Fatalf("replay left %d orphan markers", got)
	}
}
