// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientrecords"
)

// testServers maps a served test server's address to the server.
var testServers sync.Map

func registerTestServer(addr string, srv *Server) { testServers.Store(addr, srv) }
func unregisterTestServer(addr string)            { testServers.Delete(addr) }

// awaitRequestHoldsReleased waits until no request holds its incarnation
// group. The server releases a request's hold, which starts the quiet
// period, after writing the reply (releaseReply), so a test that has
// received a reply may still race the release; a lease sweep run in that
// window does not expire the client, as intended for a live request.
func awaitRequestHoldsReleased(t *testing.T, srv *Server) {
	t.Helper()
	waitFor(t, "request holds to be released", func() bool {
		ss := srv.sessions
		ss.mu.Lock()
		defer ss.mu.Unlock()
		for _, g := range ss.groups {
			if g.active > 0 || g.provisional > 0 {
				return false
			}
		}
		return true
	})
}

// endQuietForTest ends every group's quiet period and forgets the groups
// kept only by it: a group outlives its last request until quietUntil
// (README.md#lease-gate).
func endQuietForTest(srv *Server) {
	ss := srv.sessions
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for _, g := range ss.groups {
		g.quietUntil = time.Time{}
	}
	ss.pruneGroupsLocked()
}

// afterLeaseForTest moves the server's clock past the startup lease and the
// quiet period of every request so far, so the next sweep expires every
// client without a hold.
func afterLeaseForTest(srv *Server) {
	later := time.Now().Add(3 * leaseTime)
	srv.SetClock(func() time.Time { return later })
}

// groupQuiet is the end of an incarnation's quiet period, zero without one.
func groupQuiet(srv *Server, clientID uint64) time.Time {
	srv.sessions.mu.Lock()
	defer srv.sessions.mu.Unlock()
	if g := srv.sessions.groups[clientID]; g != nil {
		return g.quietUntil
	}
	return time.Time{}
}

// confirmForTest records a confirmed client for owner, as its first
// CREATE_SESSION does, and returns its incarnation.
func confirmForTest(t *testing.T, srv *Server, owner string, verifier [8]byte, principal clientrecords.Principal) uint64 {
	t.Helper()
	rec, err := srv.records.Confirm([]byte(owner), verifier, principal, clientrecords.NewIncarnation())
	if err != nil {
		t.Fatal(err)
	}
	return uint64(rec.Incarnation)
}
