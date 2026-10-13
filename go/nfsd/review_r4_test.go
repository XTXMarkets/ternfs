// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// leaseRecordForTest returns the incarnation's record, or nil.
func leaseRecordForTest(srv *Server, inc uint64) *leaseRefresher {
	cs := srv.clients
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	return cs.refreshers[InodeID(inc)]
}

func destroyClientBody(clientID uint64) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_DestroyClientid().SetDcaClientid(clientID)
	})
}

// R3-IDLE: a client that bootstraps and then goes (DESTROY_SESSION,
// DESTROY_CLIENTID) leaves an idle, unreferenced record whose worker is
// waiting. The sweeper's wake reclaims it at once, so with one record credit
// the next client registers and creates its session, cycle after cycle,
// without waiting out the idle deadline.
func TestIdleRecordReclaimedForNextClient(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	srv.clients.refreshMu.Lock()
	srv.clients.refresherLimit = 1
	srv.clients.refreshMu.Unlock()
	xid := uint32(1)
	for cycle := range 3 {
		c := newCBTestClient(t, addr)
		clientID, seq := exchangeOn(t, c, xid, fmt.Sprintf("idle-%d", cycle))
		res := c.compound(xid+1, backCreateSessionBody(clientID, seq, 0))
		if res.Status() != nfs4.NFS4_OK {
			t.Fatalf("cycle %d: CREATE_SESSION %s with no other client", cycle, nfs4.Nfsstat4Name(res.Status()))
		}
		id, _ := createdSession(t, res)
		inc := incarnationForTest(t, srv, clientID)
		r := leaseRecordForTest(srv, inc)
		waitFor(t, "the worker to wait idle", func() bool {
			r.mu.Lock()
			defer r.mu.Unlock()
			return r.idleWaiting && r.proven > 0 && !r.inCall
		})
		if res := c.compound(xid+2, admissionDestroy(id)); res.Status() != nfs4.NFS4_OK {
			t.Fatalf("cycle %d: DESTROY_SESSION %s", cycle, nfs4.Nfsstat4Name(res.Status()))
		}
		if res := c.compound(xid+3, destroyClientBody(clientID)); res.Status() != nfs4.NFS4_OK {
			t.Fatalf("cycle %d: DESTROY_CLIENTID %s", cycle, nfs4.Nfsstat4Name(res.Status()))
		}
		srv.runLeaseSweep()
		waitFor(t, fmt.Sprintf("cycle %d: the idle record to be reclaimed", cycle), func() bool {
			return leaseRecordForTest(srv, inc) == nil
		})
		xid += 4
	}
}

// R3-IDLE controls: a record is not reclaimed while its incarnation's
// logical lease is quiet (need), or while a CREATE_SESSION reservation
// holds it, and the sweeper's wake changes none of its deadlines. Once
// both end, it is reclaimed promptly. The client itself is kept (only the
// session is destroyed), so its gate and quiet period remain.
func TestIdleReclaimWaitsForQuietAndReservation(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	c := newCBTestClient(t, addr)
	clientID, seq := exchangeOn(t, c, 1, "idle-quiet")
	id, _ := createdSession(t, c.compound(2, backCreateSessionBody(clientID, seq, 0)))
	ts := newTestSession(id, clientID, 16)
	body := buildCompoundBody(nil, func(*nfs4.COMPOUND4argsWriter) {})
	if res := c.compound(3, withSequence(body, ts, 0, 1)); res.Status() != nfs4.NFS4_OK {
		t.Fatalf("SEQUENCE %s", nfs4.Nfsstat4Name(res.Status()))
	}
	if res := c.compound(4, admissionDestroy(id)); res.Status() != nfs4.NFS4_OK {
		t.Fatalf("DESTROY_SESSION %s", nfs4.Nfsstat4Name(res.Status()))
	}
	inc := incarnationForTest(t, srv, clientID)
	if !srv.sessions.leaseNeed(inc) {
		t.Fatal("no quiet period after the SEQUENCE")
	}
	r := leaseRecordForTest(srv, inc)
	// Wait until its next maintenance (the refresh the quiet period needs)
	// is scheduled ahead, so no pass is due during the checks.
	waitFor(t, "the worker to schedule its refresh", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return !r.inCall && time.Until(r.maintainAt) > 5*time.Second
	})
	deadlines := func() [3]any {
		r.mu.Lock()
		defer r.mu.Unlock()
		return [3]any{r.maintainAt, r.notBefore, r.backoff}
	}
	before := deadlines()
	held := func(what string) {
		t.Helper()
		srv.runLeaseSweep()
		time.Sleep(200 * time.Millisecond)
		if leaseRecordForTest(srv, inc) != r {
			t.Fatalf("the record was reclaimed while %s", what)
		}
		if after := deadlines(); after != before {
			t.Fatalf("the sweeper's wake moved the deadlines from %v to %v", before, after)
		}
	}
	held("its logical lease is quiet")
	release, ok := srv.clients.reserveLeaseRecord(inc)
	if !ok {
		t.Fatal("no record for the reservation")
	}
	endQuietForTest(srv)
	if srv.sessions.leaseNeed(inc) {
		t.Fatal("still needed after the quiet period")
	}
	held("a CREATE_SESSION reservation holds it")
	release()
	srv.runLeaseSweep()
	waitFor(t, "the record to be reclaimed", func() bool { return leaseRecordForTest(srv, inc) == nil })
}

// R3-IDLE: the reclamation wake does not touch an obligation. A record with
// an unsettled publication attempt in its backoff, and nothing else
// referencing it, is kept through repeated sweeps with no fence or write
// before its deadline, and its deadline does not move. After the deadline
// the attempt is fenced, and only then is the record freed.
func TestIdleReclaimKeepsObligations(t *testing.T) {
	counts := &passCountVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	fs := &fenceClockVFS{passCountVFS: counts}
	cs, clientID := newConfirmedStoreClient(t, fs, []byte("client"), [8]byte{1}, clientOwner{})
	counts.leaseName = cs.leaseName
	cs.now = time.Now
	var need atomic.Bool
	need.Store(true)
	cs.leaseNeed = func(uint64) bool { return need.Load() }
	cs.leaseSessions = func(uint64) bool { return false }
	t.Cleanup(cs.stopLeaseWorkers)
	release, ok := cs.reserveLeaseRecord(clientID)
	if !ok {
		t.Fatal("no record credit")
	}
	cs.refreshMu.Lock()
	r := cs.refreshers[InodeID(clientID)]
	cs.refreshMu.Unlock()
	// The publication's rename fails, and so does the first fence.
	counts.mu.Lock()
	counts.failRename, counts.failFence = true, true
	counts.mu.Unlock()
	cs.noteLeaseNeed(clientID)
	waitFor(t, "the failed publication", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.attempt != nil && !r.notBefore.IsZero() && !r.inCall
	})
	r.mu.Lock()
	deadline, backoff := r.notBefore, r.backoff
	r.mu.Unlock()
	// Nothing references the record but its attempt.
	need.Store(false)
	release()
	counts.mu.Lock()
	counts.failRename, counts.failFence = false, false
	counts.mu.Unlock()
	_, fences0, writes0, _ := counts.counts()
	for time.Until(deadline) > 500*time.Millisecond {
		cs.sweepLeaseRecords()
		time.Sleep(50 * time.Millisecond)
	}
	_, fences, writes, _ := counts.counts()
	attempt, _, _, _ := recordImage(r)
	r.mu.Lock()
	moved := !r.notBefore.Equal(deadline) || r.backoff != backoff
	r.mu.Unlock()
	if fences != fences0 || writes != writes0 {
		t.Fatalf("sweeps inside the backoff drove %d fences and %d writes", fences-fences0, writes-writes0)
	}
	cs.refreshMu.Lock()
	kept := cs.refreshers[InodeID(clientID)] == r
	cs.refreshMu.Unlock()
	if !attempt || !kept {
		t.Fatalf("the sweeps released the obligation: attempt %v, record kept %v", attempt, kept)
	}
	if moved {
		t.Fatal("the sweeps moved the backoff")
	}
	// After the deadline the worker fences the attempt, then frees the record.
	for limit := time.Now().Add(deadline.Sub(time.Now()) + testChannelTimeout); ; time.Sleep(10 * time.Millisecond) {
		cs.refreshMu.Lock()
		freed := cs.refreshers[InodeID(clientID)] == nil
		cs.refreshMu.Unlock()
		if freed {
			break
		}
		if time.Now().After(limit) {
			t.Fatal("the record was not freed after its attempt settled")
		}
	}
	if at := fs.firstFence(); at.IsZero() || at.Before(deadline) {
		t.Fatalf("the attempt was fenced at %v, before the deadline %v", at, deadline)
	}
	if attempt, _, _, _ := recordImage(r); attempt {
		t.Fatal("freed with its attempt unsettled")
	}
}
