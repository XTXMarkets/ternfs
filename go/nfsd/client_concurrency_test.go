// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"io"
	"sync/atomic"
	"testing"
	"time"
)

type markerGateVFS struct {
	TernVFS
	name    string
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (fs *markerGateVFS) CreateFile(dir InodeID, name string, data io.Reader) (InodeID, error) {
	if name == fs.name && fs.calls.Add(1) == 1 {
		close(fs.entered)
		<-fs.release
	}
	return fs.TernVFS.CreateFile(dir, name, data)
}

func TestClientStoreIndependentMarkers(t *testing.T) {
	fs := &markerGateVFS{TernVFS: NewLocalTernVFS(t.TempDir()),
		name: activeOpenName(StateID{1}), entered: make(chan struct{}), release: make(chan struct{})}
	store, id := newConfirmedStoreClient(t, fs, []byte("client"), [8]byte{1}, clientOwner{})
	if err := store.MarkOpen(id, StateID{3}); err != nil {
		t.Fatal(err)
	}
	first := make(chan error, 1)
	go func() { first <- store.MarkOpen(id, StateID{1}) }()
	defer closeSignal(fs.release)
	awaitSignal(t, fs.entered, "blocked marker")
	other := make(chan error, 1)
	go func() {
		err := store.MarkOpen(id, StateID{2})
		if err == nil {
			err = store.RemoveOpen(id, StateID{3})
		}
		other <- err
	}()
	if err := awaitValue(t, other, "independent marker create and remove"); err != nil {
		t.Fatal(err)
	}
	duplicate := make(chan error, 1)
	go func() { duplicate <- store.MarkOpen(id, StateID{1}) }()
	closeSignal(fs.release)
	for _, result := range []chan error{first, duplicate} {
		if err := awaitValue(t, result, "marker creation"); err != nil {
			t.Fatal(err)
		}
	}
	if fs.calls.Load() != 1 {
		t.Fatalf("duplicate marker creates = %d", fs.calls.Load())
	}
	for _, sid := range []StateID{{1}, {2}} {
		if active, err := store.HasOpen(id, sid); err != nil || !active {
			t.Fatalf("open %v = %v, %v", sid, active, err)
		}
	}
	if active, err := store.HasOpen(id, StateID{3}); err != nil || active {
		t.Fatalf("removed open = %v, %v", active, err)
	}
}

func TestClientStoreRemoveWaitsForSameMarker(t *testing.T) {
	fs := &markerGateVFS{TernVFS: NewLocalTernVFS(t.TempDir()),
		name: activeOpenName(StateID{1}), entered: make(chan struct{}), release: make(chan struct{})}
	store, id := newConfirmedStoreClient(t, fs, []byte("client"), [8]byte{1}, clientOwner{})
	first := make(chan error, 1)
	go func() { first <- store.MarkOpen(id, StateID{1}) }()
	defer closeSignal(fs.release)
	awaitSignal(t, fs.entered, "marker creation")
	removed := make(chan error, 1)
	go func() { removed <- store.RemoveOpen(id, StateID{1}) }()
	select {
	case err := <-removed:
		t.Fatalf("removal overtook creation: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	closeSignal(fs.release)
	for _, result := range []chan error{first, removed} {
		if err := awaitValue(t, result, "marker operation"); err != nil {
			t.Fatal(err)
		}
	}
	// Read through a fresh cache as well as the local map, so a cached
	// removal cannot hide an orphaned durable marker.
	for i := 0; i < 2; i++ {
		if active, err := store.HasOpen(id, StateID{1}); err != nil || active {
			t.Fatalf("removed marker = %v, %v", active, err)
		}
		store.localClients.Delete(InodeID(id))
	}
}

func TestClientStoreMarkerExcludesLifecycle(t *testing.T) {
	for _, action := range []string{"confirm", "expire", "collect"} {
		t.Run(action, func(t *testing.T) {
			fs := &markerGateVFS{TernVFS: NewLocalTernVFS(t.TempDir()),
				name: activeOpenName(StateID{1}), entered: make(chan struct{}), release: make(chan struct{})}
			store, id := newConfirmedStoreClient(t, fs, []byte("client"), [8]byte{1}, clientOwner{})
			next, verifier, err := store.SetClientID([8]byte{2}, []byte("client"), clientOwner{})
			if err != nil {
				t.Fatal(err)
			}
			var now atomic.Int64
			now.Store(time.Now().UnixNano())
			store.now = func() time.Time { return time.Unix(0, now.Load()) }
			if err := store.Renew(id); err != nil {
				t.Fatal(err)
			}
			created := make(chan error, 1)
			go func() { created <- store.MarkOpen(id, StateID{1}) }()
			defer closeSignal(fs.release)
			awaitSignal(t, fs.entered, "marker creation")
			done := make(chan error, 1)
			go func() {
				switch action {
				case "confirm":
					_, err := store.ConfirmClientID(next, verifier, rpcPrincipal{})
					done <- err
				case "expire":
					now.Add(int64(2 * nfsLeaseTime))
					expired, err := store.ExpireIfLeaseDead(id)
					if err == nil && !expired {
						err = nfsError(NFS4ERR_SERVERFAULT)
					}
					done <- err
				case "collect":
					_, err := store.collectStaleForClientResult(InodeID(id))
					done <- err
				}
			}()
			select {
			case err := <-done:
				t.Fatalf("%s overtook marker creation: %v", action, err)
			case <-time.After(25 * time.Millisecond):
			}
			closeSignal(fs.release)
			if err := awaitValue(t, created, "marker"); err != nil {
				t.Fatal(err)
			}
			if err := awaitValue(t, done, action); err != nil {
				t.Fatal(err)
			}
			active, err := store.HasOpen(id, StateID{1})
			if err != nil || active != (action == "collect") {
				t.Fatalf("open after %s = %v, %v", action, active, err)
			}
		})
	}
}

type leaseGateVFS struct {
	TernVFS
	name    string
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (fs *leaseGateVFS) Rename(src InodeID, old string, dst InodeID, name string) error {
	if name == fs.name && fs.calls.Add(1) == 1 {
		close(fs.entered)
		<-fs.release
	}
	return fs.TernVFS.Rename(src, old, dst, name)
}

func TestClientStoreLeaseLockSurvivesCacheReplacement(t *testing.T) {
	fs := &leaseGateVFS{TernVFS: NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	store, id := newConfirmedStoreClient(t, fs, []byte("client"), [8]byte{1}, clientOwner{})
	fs.name = store.leaseName
	first := make(chan error, 1)
	go func() { first <- store.MarkOpen(id, StateID{1}) }()
	defer closeSignal(fs.release)
	awaitSignal(t, fs.entered, "lease renewal")
	// Invalidation may discard the cache while another shared holder is
	// still renewing. The next holder must use the same serialization lock.
	store.localClients.Delete(InodeID(id))
	second := make(chan error, 1)
	go func() { second <- store.MarkOpen(id, StateID{2}) }()
	select {
	case err := <-second:
		t.Fatalf("replacement cache bypassed lease lock: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if fs.calls.Load() != 1 {
		t.Fatal("lease writes overlapped across cache replacement")
	}
	closeSignal(fs.release)
	for _, ch := range []chan error{first, second} {
		if err := awaitValue(t, ch, "renewed open"); err != nil {
			t.Fatal(err)
		}
	}
}
