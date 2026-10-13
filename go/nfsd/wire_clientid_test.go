// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// testWireEpochs gives every test server its own epoch, so a server built
// over the same store as an earlier one (a restart) never reuses its
// clientids.
var testWireEpochs atomic.Uint32

func init() { testWireEpochs.Store(1 << 30) }

// useTestWireEpoch stands in for listen in tests that call handleConn
// directly. It leaves an epoch that is already set.
func useTestWireEpoch(srv *Server) {
	w := &srv.sessions.wireIDs
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.epoch == 0 {
		w.epoch = testWireEpochs.Add(1)
	}
}

// incarnationForTest maps a live wire clientid, as EXCHANGE_ID returned it,
// to the durable incarnation that ClientStore, open state and staging use.
func incarnationForTest(t *testing.T, srv *Server, wireID uint64) uint64 {
	t.Helper()
	srv.sessions.mu.Lock()
	defer srv.sessions.mu.Unlock()
	c := srv.sessions.wireRecordLocked(wireID)
	if c == nil {
		t.Fatalf("no live wire record %#x", wireID)
	}
	return c.incarnation
}

func wireTestServer(t *testing.T) *Server {
	t.Helper()
	srv, err := NewServer(vfs.NewLocalTernVFS(t.TempDir()), readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestWireEpochChosenAfterExclusiveBind(t *testing.T) {
	first := wireTestServer(t)
	if _, err := first.sessions.wireIDs.next(); !errors.Is(err, errWireEpochUnset) {
		t.Fatalf("clientid before listen: %v", err)
	}
	before := time.Now()
	ln, err := first.listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	firstEpoch := first.sessions.wireIDs.epoch
	if int64(firstEpoch) <= before.Unix() {
		t.Fatalf("epoch %d is not after the bind second %d", firstEpoch, before.Unix())
	}

	// While the endpoint is owned, another process cannot bind and so never
	// chooses an epoch.
	busy := wireTestServer(t)
	if busyLn, err := busy.listen(addr); err == nil {
		busyLn.Close()
		t.Fatal("second bind of an owned endpoint succeeded")
	}
	if _, err := busy.sessions.wireIDs.next(); !errors.Is(err, errWireEpochUnset) {
		t.Fatalf("failed bind chose an epoch: %v", err)
	}

	// The next owner binds at once, in the same second as the previous
	// owner's epoch, and still chooses a later epoch.
	ln.Close()
	next := wireTestServer(t)
	nextLn, err := next.listen(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer nextLn.Close()
	if nextEpoch := next.sessions.wireIDs.epoch; nextEpoch <= firstEpoch {
		t.Fatalf("next owner's epoch %d is not after %d", nextEpoch, firstEpoch)
	}
}

// An epoch that cannot be set stops startup and frees the endpoint.
func TestWireEpochFailureReleasesListener(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()
	srv := wireTestServer(t)
	useTestWireEpoch(srv) // A second start fails after the bind.
	if ln, err := srv.listen(addr); err == nil {
		ln.Close()
		t.Fatal("listen succeeded with an epoch already set")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("endpoint still bound after failed startup: %v", err)
	}
	ln.Close()
}

func TestWireEpochAfterWaitsForNextSecond(t *testing.T) {
	bound := time.Unix(100, 900_000_000)
	clock := time.Unix(100, 950_000_000)
	var slept []time.Duration
	epoch := wireEpochAfter(bound, func() time.Time { return clock }, func(d time.Duration) {
		slept = append(slept, d)
		clock = clock.Add(d)
	})
	if epoch != 101 || len(slept) != 1 || slept[0] != 50*time.Millisecond {
		t.Fatalf("epoch %d after sleeping %v", epoch, slept)
	}
	// A clock already past the bound second does not wait.
	clock = time.Unix(205, 0)
	slept = nil
	if epoch := wireEpochAfter(bound, func() time.Time { return clock }, func(d time.Duration) {
		slept = append(slept, d)
	}); epoch != 205 || slept != nil {
		t.Fatalf("epoch %d after sleeping %v", epoch, slept)
	}
}

func TestWireEpochRange(t *testing.T) {
	for _, epoch := range []int64{-1, 0, 1 << 32} {
		var w wireClientIDs
		if err := w.start(epoch); err == nil {
			t.Fatalf("epoch %d accepted", epoch)
		}
		if _, err := w.next(); !errors.Is(err, errWireEpochUnset) {
			t.Fatalf("rejected epoch %d issued a clientid: %v", epoch, err)
		}
	}
	var w wireClientIDs
	if err := w.start(0xffffffff); err != nil {
		t.Fatal(err)
	}
	if err := w.start(1); err == nil || w.epoch != 0xffffffff {
		t.Fatalf("epoch changed after start: %v %d", err, w.epoch)
	}
	if id, err := w.next(); err != nil || id != 0xffffffff<<32|1 {
		t.Fatalf("clientid %x %v", id, err)
	}
}

func TestWireClientIDsNeverWrap(t *testing.T) {
	var w wireClientIDs
	if err := w.start(7); err != nil {
		t.Fatal(err)
	}
	if id, err := w.next(); err != nil || id != 7<<32|1 {
		t.Fatalf("first clientid %x %v", id, err)
	}
	w.last = maxWireCounter - 1
	if id, err := w.next(); err != nil || id != 7<<32|0xffffffff {
		t.Fatalf("last clientid %x %v", id, err)
	}
	for range 2 {
		if id, err := w.next(); !errors.Is(err, errWireIDsExhausted) {
			t.Fatalf("clientid %x after exhaustion: %v", id, err)
		}
	}
}
