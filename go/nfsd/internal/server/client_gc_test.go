// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func TestClientStoreGCIsBoundedAndRetryable(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	writer, err := clientstore.NewStore(baseFS)
	if err != nil {
		t.Fatal(err)
	}
	failingFS := &clientstore.RemoveFailureVFS{
		TernVFS: baseFS,
		Name:    clientstore.ClientRecordName,
	}
	collector, err := clientstore.NewStore(failingFS)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	collector.Now = func() time.Time { return base }
	owner := clientstore.ClientOwner{Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"}}

	var clientID uint64
	for i := 0; i < clientstore.MaxClientGCRemovals+3; i++ {
		clientID, _, err = writer.SetClientID(
			[8]byte{byte(i + 1)}, []byte("client"), owner)
		if err != nil {
			t.Fatal(err)
		}
	}
	identityID, err := baseFS.LookupParent(vfs.InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	if err := collector.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := baseFS.Lookup(vfs.InodeID(clientID), clientstore.GCCandidateName); !os.IsNotExist(err) {
		t.Fatalf("current incarnation has a GC candidate marker: %v", err)
	}

	collector.Now = func() time.Time {
		return base.Add(clientstore.ClientGCGrace + time.Second)
	}
	failingFS.Remaining = 1
	if err := collector.CollectStaleForClient(
		vfs.InodeID(clientID),
	); !errors.Is(err, clientstore.ErrInjectedRemove) {
		t.Fatalf("GC error = %v, want injected failure", err)
	}
	if got := countClientIncarnations(t, baseFS, identityID); got != clientstore.MaxClientGCRemovals+3 {
		t.Fatalf("incarnations after failed GC = %d, want %d",
			got, clientstore.MaxClientGCRemovals+3)
	}

	if err := collector.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if got := countClientIncarnations(t, baseFS, identityID); got != 3 {
		t.Fatalf("incarnations after bounded GC = %d, want 3", got)
	}
	if err := collector.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if got := countClientIncarnations(t, baseFS, identityID); got != 1 {
		t.Fatalf("incarnations after GC retry = %d, want 1", got)
	}
}

func TestServerSchedulesClientGC(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	srv.clients.SetClock(func() time.Time { return base })
	owner := clientstore.ClientOwner{Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"}}
	oldID, _, err := srv.clients.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	clientID, _, err := srv.clients.SetClientID(
		[8]byte{2}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.clients.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	srv.clients.SetClock(func() time.Time {
		return base.Add(clientstore.ClientGCGrace + time.Second)
	})

	for range 20 {
		srv.scheduleClientGC(clientID)
	}
	srv.waitForClientGC()

	if _, err := fs.Stat(vfs.InodeID(oldID)); !os.IsNotExist(err) {
		t.Fatalf("scheduled GC did not remove stale incarnation: %v", err)
	}
	if _, err := fs.Stat(vfs.InodeID(clientID)); err != nil {
		t.Fatalf("scheduled GC removed current incarnation: %v", err)
	}
	srv.clientGCMu.Lock()
	running := srv.clientGCRunning
	pending := len(srv.clientGCPending)
	srv.clientGCMu.Unlock()
	if running || pending != 0 {
		t.Fatalf("client GC did not drain: running=%v pending=%d", running, pending)
	}
}

func TestConfirmedClientGCRechecksAfterLease(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	srv.clients.SetClock(func() time.Time { return base })
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)

	oldID := incarnationForTest(t, srv, setupClientWithVerifier(t, conn, &xid, [8]byte{1}))
	srv.waitForClientGC()
	newID, confirm := requestClientID(
		t, conn, &xid, "test-client", [8]byte{2})
	srv.waitForClientGC()
	if status := confirmClientID(
		t, conn, &xid, newID, confirm,
	); status != nfs4.NFS4_OK {
		t.Fatalf("SETCLIENTID_CONFIRM status = %s",
			nfs4.Nfsstat4Name(status))
	}
	srv.waitForClientGC()

	if _, err := fs.Lookup(
		vfs.InodeID(oldID), clientstore.GCCandidateName,
	); err != nil {
		t.Fatalf("confirmation did not schedule first GC phase: %v", err)
	}
	srv.clients.SetClock(func() time.Time {
		return base.Add(clientstore.ClientGCGrace + time.Second)
	})
	srv.runLeaseSweep()
	srv.waitForClientGC()
	if _, err := fs.Stat(vfs.InodeID(oldID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("periodic GC recheck retained stale incarnation: %v", err)
	}
}

func TestScheduleClientGCRestartsForPendingWork(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	clientID, _, err := srv.clients.SetClientID(
		[8]byte{1}, []byte("client"), clientstore.ClientOwner{})
	if err != nil {
		t.Fatal(err)
	}
	identityID, err := fs.LookupParent(vfs.InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	unlock := srv.clients.LockIdentity(identityID)
	srv.clientGCMu.Lock()
	srv.clientGCPending[vfs.InodeID(clientID)] = struct{}{}
	srv.clientGCRunning = false
	srv.clientGCMu.Unlock()

	srv.scheduleClientGC(clientID)
	srv.clientGCMu.Lock()
	running := srv.clientGCRunning
	srv.clientGCMu.Unlock()
	unlock()
	if !running {
		t.Fatal("pending client GC did not restart a stopped worker")
	}
	srv.waitForClientGC()
}

func TestClientGCContinuesAfterPanic(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	fs := &clientstore.LookupFailureVFS{TernVFS: baseFS}
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	var clientIDs []vfs.InodeID
	for i := byte(1); i <= 2; i++ {
		clientID, _, err := srv.clients.SetClientID(
			[8]byte{i}, []byte{byte('a' + i)}, clientstore.ClientOwner{})
		if err != nil {
			t.Fatal(err)
		}
		clientIDs = append(clientIDs, vfs.InodeID(clientID))
	}
	fs.Name = clientstore.ConfirmedName
	fs.PanicRemaining = 1
	srv.clientGCMu.Lock()
	for _, clientID := range clientIDs {
		srv.clientGCPending[clientID] = struct{}{}
	}
	srv.clientGCRunning = true
	srv.clientGCDone = make(chan struct{})
	srv.clientGCMu.Unlock()

	srv.runClientGC()
	srv.clientGCMu.Lock()
	running := srv.clientGCRunning
	pending := len(srv.clientGCPending)
	srv.clientGCMu.Unlock()
	if running || pending != 0 {
		t.Fatalf("GC after panic: running=%t pending=%d, want false, 0",
			running, pending)
	}
}
