// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

import (
	"io"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

type markerGateVFS struct {
	vfs.TernVFS
	*testutil.Gate
	name string
}

func (fs *markerGateVFS) CreateFile(dir vfs.InodeID, name string, data io.Reader) (vfs.InodeID, error) {
	if name == fs.name {
		fs.Wait()
	}
	return fs.TernVFS.CreateFile(dir, name, data)
}

type markerLookupGateVFS struct {
	vfs.TernVFS
	*testutil.Gate
	name string
}

func (fs *markerLookupGateVFS) Lookup(dir vfs.InodeID, name string) (vfs.InodeID, error) {
	id, err := fs.TernVFS.Lookup(dir, name)
	if name == fs.name {
		fs.Wait()
	}
	return id, err
}

type leaseGateVFS struct {
	vfs.TernVFS
	*testutil.Gate
	name string
}

func (fs *leaseGateVFS) Rename(src vfs.InodeID, old string, dst vfs.InodeID, name string) error {
	if name == fs.name {
		fs.Wait()
	}
	return fs.TernVFS.Rename(src, old, dst, name)
}

func (fs *leaseGateVFS) RenameEdge(src vfs.InodeID, old string, edge vfs.Edge, dst vfs.InodeID, name string) error {
	if name == fs.name {
		fs.Wait()
	}
	return fs.TernVFS.RenameEdge(src, old, edge, dst, name)
}

func TestClientStoreLeaseLockSurvivesCacheReplacement(t *testing.T) {
	fs := &leaseGateVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		Gate: testutil.NewGate()}
	store, id := NewConfirmedStoreClient(t, fs, []byte("client"), [8]byte{1}, ClientOwner{})
	fs.name = store.LeaseName
	// Due for a refresh, so the first MarkOpen publishes.
	base := time.Now()
	store.Now = func() time.Time { return base.Add(LeaseRenewAfter + time.Second) }
	first := make(chan error, 1)
	go func() { first <- store.NoteOpen(id, nfs4.StateID{1}) }()
	defer testutil.CloseSignal(fs.Release)
	testutil.AwaitSignal(t, fs.Entered, "lease renewal")
	// Invalidation may discard the cache while another shared holder is
	// still renewing. The next holder must use the same serialization lock.
	store.LocalClients.Delete(vfs.InodeID(id))
	second := make(chan error, 1)
	go func() { second <- store.NoteOpen(id, nfs4.StateID{2}) }()
	testutil.AssertBlocked(t, second, "lease renewal after cache replacement")
	if fs.Calls.Load() != 1 {
		t.Fatal("lease writes overlapped across cache replacement")
	}
	testutil.CloseSignal(fs.Release)
	for _, ch := range []chan error{first, second} {
		if err := testutil.AwaitValue(t, ch, "renewed open"); err != nil {
			t.Fatal(err)
		}
	}
}
