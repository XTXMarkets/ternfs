// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// clientStoreDir reports whether dir is inside the client store (.nfs), where
// the refresher constructs its temps.
func clientStoreDir(fs vfs.TernVFS, dir vfs.InodeID) bool {
	nfsDir, err := fs.Lookup(fs.RootID(), vfs.NFSDirName)
	if err != nil {
		return false
	}
	for i := 0; i < 4 && dir != 0 && dir != fs.RootID(); i++ {
		if dir == nfsDir {
			return true
		}
		if dir, err = fs.LookupParent(dir); err != nil {
			return false
		}
	}
	return false
}

// testServers maps a served test server's address to the server, so the
// session shim can wait out the first-publication DELAY after
// CREATE_SESSION without consuming a slot.
var testServers sync.Map

func registerTestServer(addr string, srv *Server) { testServers.Store(addr, srv) }
func unregisterTestServer(addr string)            { testServers.Delete(addr) }

// awaitShimLease waits, for a session the shim created on conn, until its
// server has published the client's lease: the DELAY a real client's first
// SEQUENCE may get (canonical r3 §9.1). Unknown servers are not waited for.
func awaitShimLease(conn net.Conn, wireID uint64) {
	value, ok := testServers.Load(conn.RemoteAddr().String())
	if !ok {
		return
	}
	srv := value.(*Server)
	srv.sessions.mu.Lock()
	c := srv.sessions.wireRecordLocked(wireID)
	srv.sessions.mu.Unlock()
	if c == nil {
		return
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if srv.clients.LeaseProven(vfs.InodeID(c.incarnation)) {
			return
		}
	}
}

// setLeaseForTest changes an incarnation's refresher record in place.
func setLeaseForTest(srv *Server, clientID uint64, mutate func(r *clientstore.LeaseRefresher)) {
	cs := srv.clients
	cs.RefreshMu.Lock()
	r, _ := cs.RecordLocked(vfs.InodeID(clientID), true)
	cs.RefreshMu.Unlock()
	r.Mu.Lock()
	defer r.Mu.Unlock()
	mutate(r)
}

// endQuietForTest ends every group's quiet period and forgets the groups
// kept only by it: a group outlives its last request until quietUntil
// (canonical r3 §8).
func endQuietForTest(srv *Server) {
	ss := srv.sessions
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for _, g := range ss.groups {
		g.quietUntil = time.Time{}
	}
	ss.pruneGroupsLocked()
}

// leasePassForTest runs one refresher pass for an incarnation in the test's
// goroutine, as its worker would, without forcing a refresh.
func leasePassForTest(t *testing.T, srv *Server, clientID uint64) {
	t.Helper()
	cs := srv.clients
	cs.RefreshMu.Lock()
	r, ok := cs.RecordLocked(vfs.InodeID(clientID), true)
	cs.RefreshMu.Unlock()
	if !ok {
		t.Fatal("no refresher record credit")
	}
	r.SafePass(false)
}

// awaitLeaseProven waits until this process has published the incarnation's
// lease, as a client's first SEQUENCE after CREATE_SESSION waits out DELAY.
func awaitLeaseProven(t *testing.T, srv *Server, clientID uint64) {
	t.Helper()
	for deadline := time.Now().Add(testutil.ChannelTimeout); !srv.clients.LeaseProven(vfs.InodeID(clientID)); {
		if time.Now().After(deadline) {
			t.Fatalf("lease of %d never published", clientID)
		}
		time.Sleep(time.Millisecond)
	}
}

// publishLeaseForTest has the worker publish a store-confirmed client's
// lease, as a CREATE_SESSION's bootstrap does, for tests that confirm a
// client through the store and then call MarkOpen.
func publishLeaseForTest(t *testing.T, srv *Server, clientID uint64) {
	t.Helper()
	// With no session, the record is kept by a reservation for the test.
	release, ok := srv.clients.ReserveLeaseRecord(clientID)
	if !ok {
		t.Fatal("no refresher record credit")
	}
	t.Cleanup(release)
	srv.clients.StartLeaseBootstrap(clientID)
	awaitLeaseProven(t, srv, clientID)
}
