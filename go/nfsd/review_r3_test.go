// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// leaseRemoveHold blocks the first RemoveEdge of the lease name once
// armed: the terminal pass's cleanup of our lease.
type leaseRemoveHold struct {
	TernVFS
	mu        sync.Mutex
	leaseName string
	armed     bool
	entered   chan struct{}
	release   chan struct{}
}

func (fs *leaseRemoveHold) RemoveEdge(dir InodeID, name string, edge Edge) error {
	fs.mu.Lock()
	hold := fs.armed && name == fs.leaseName
	if hold {
		fs.armed = false
	}
	fs.mu.Unlock()
	if hold {
		close(fs.entered)
		<-fs.release
	}
	return fs.TernVFS.RemoveEdge(dir, name, edge)
}

// R2-TERMINAL: once the refresher knows an incarnation is terminal, a new
// CREATE_SESSION cannot commit, even while the terminal cleanup's store
// I/O is blocked. The cached CREATE_SESSION still replays, and the slot
// does not move.
func TestTerminalAdmissionBeforeCleanup(t *testing.T) {
	fs := &leaseRemoveHold{TernVFS: NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fs.leaseName = srv.clients.leaseName
	addr, stop := serveTestServer(t, srv)
	defer stop()
	defer closeSignal(fs.release)
	c := newCBTestClient(t, addr)
	clientID, seq := exchangeOn(t, c, 1, "terminal-cleanup")
	body := backCreateSessionBody(clientID, seq, 0)
	id, _ := createdSession(t, c.compound(2, body))
	inc := incarnationForTest(t, srv, clientID)
	srv.clients.stopLeaseWorkers()
	if !fs.present(InodeID(inc)) {
		t.Fatal("no published lease to clean up")
	}
	setLeaseForTest(srv, inc, func(r *leaseRefresher) { r.state = leaseRecordStale })
	fs.mu.Lock()
	fs.armed = true
	fs.mu.Unlock()
	cs := srv.clients
	cs.refreshMu.Lock()
	r, _ := cs.recordLocked(InodeID(inc), true)
	cs.refreshMu.Unlock()
	passed := make(chan struct{})
	go func() {
		defer close(passed)
		r.safePass(false)
	}()
	awaitSignal(t, fs.entered, "the terminal cleanup's RemoveEdge")

	before := srv.sessions.accounting()
	if res := c.compound(3, backCreateSessionBody(clientID, seq+1, 0)); res.Status() != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION during terminal cleanup %s, want STALE_CLIENTID", Nfsstat4Name(res.Status()))
	}
	if after := srv.sessions.accounting(); after.Sessions != before.Sessions ||
		after.Records != before.Records || after.PendingSessions != before.PendingSessions {
		t.Fatalf("refused CREATE_SESSION changed %+v to %+v", before, after)
	}
	if replayed, _ := createdSession(t, c.compound(4, body)); replayed != id {
		t.Fatal("the cached CREATE_SESSION did not replay during cleanup")
	}

	closeSignal(fs.release)
	awaitSignal(t, passed, "the terminal pass")
	if fs.present(InodeID(inc)) {
		t.Fatal("the terminal pass left our lease")
	}
	if srv.sessions.lookup(id) != nil {
		t.Fatal("the terminal pass did not retire the session")
	}
	// After cleanup: still a replay of the dead session, and still no new one.
	if replayed, _ := createdSession(t, c.compound(5, body)); replayed != id {
		t.Fatal("the cached CREATE_SESSION did not replay after cleanup")
	}
	if res := c.compound(6, backCreateSessionBody(clientID, seq+1, 0)); res.Status() != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION after terminal cleanup %s, want STALE_CLIENTID", Nfsstat4Name(res.Status()))
	}
}

func (fs *leaseRemoveHold) present(dir InodeID) bool {
	_, err := fs.TernVFS.Lookup(dir, fs.leaseName)
	return err == nil
}

// fenceClockVFS is passCountVFS that also records when the first temp was
// removed.
type fenceClockVFS struct {
	*passCountVFS
	at time.Time
}

func (fs *fenceClockVFS) RemoveEdge(dir InodeID, name string, edge Edge) error {
	if leaseTemp(name) {
		fs.mu.Lock()
		if fs.at.IsZero() {
			fs.at = time.Now()
		}
		fs.mu.Unlock()
	}
	return fs.passCountVFS.RemoveEdge(dir, name, edge)
}

func (fs *fenceClockVFS) firstFence() time.Time {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.at
}

// R2-VERIFY: verify requests do not run maintenance. An unresolved LINK's
// temp appears after a successful publication, so no failure deadline is
// active; the next maintenance pass is the obligation recheck. Verifies
// before it only read: the orphan is not consumed and the deadline does not
// move. When it is due, maintenance runs with no further request.
func TestVerifyDoesNotRunMaintenance(t *testing.T) {
	counts := &passCountVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	fs := &fenceClockVFS{passCountVFS: counts}
	cs, clientID, r, _ := refresherForTest(t, fs)
	counts.leaseName = cs.leaseName
	cs.now = time.Now
	cs.leaseNeed = func(uint64) bool { return true }
	cs.leaseSessions = func(uint64) bool { return true }
	t.Cleanup(cs.stopLeaseWorkers)
	dir := InodeID(clientID)

	// A LINK whose outcome was unknown: its orphan is recorded, unresolved,
	// and its temp is not visible yet.
	inode, cookie, err := counts.TernVFS.ConstructFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	unlock := cs.leaseLocks.lock(r.incarnation)
	o, ok := r.takeOrphan()
	if !ok {
		unlock()
		t.Fatal("no orphan credit")
	}
	r.mu.Lock()
	o.inode, o.phase = inode, orphanUnresolved
	r.mu.Unlock()
	unlock()

	// A maintenance pass publishes successfully: no failure deadline, and
	// the orphan is an obligation to recheck at the next maintenance.
	cs.startLeaseBootstrap(clientID)
	waitFor(t, "the publication", func() bool { _, _, proven, _ := recordImage(r); return proven > 0 })
	waitFor(t, "the pass to schedule", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return !r.maintainAt.IsZero() && r.notBefore.IsZero() && !r.inCall
	})
	// Bring the scheduled recheck in, so the test need not wait
	// leaseBackoffMax; verifies must neither move nor run it.
	deadline := time.Now().Add(2500 * time.Millisecond)
	r.mu.Lock()
	r.maintainAt = deadline
	r.mu.Unlock()

	// The LINK applies late: the orphan's temp is now visible.
	data, _ := json.Marshal(durableLease{ExpiresUnixNano: time.Now().Add(leaseWriteTerm).UnixNano()})
	if err := counts.TernVFS.LinkFile(inode, cookie, dir, o.temp, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	counts.reset()
	for time.Until(deadline) > 300*time.Millisecond {
		cs.requestLeaseVerify(clientID)
		time.Sleep(20 * time.Millisecond)
	}
	reads, fences, writes, removes := counts.counts()
	_, orphans, _, _ := recordImage(r)
	r.mu.Lock()
	moved := !r.maintainAt.Equal(deadline)
	r.mu.Unlock()
	if fences != 0 || writes != 0 || removes != 0 || orphans != 1 {
		t.Fatalf("verifies before the maintenance deadline: fences %d writes %d removes %d, orphans %d",
			fences, writes, removes, orphans)
	}
	if reads == 0 {
		t.Fatal("no verify pass ran")
	}
	if moved {
		t.Fatal("a verify pass moved the maintenance deadline")
	}
	// Due: the orphan is consumed, with no further request.
	waitFor(t, "maintenance at its deadline", func() bool { _, n, _, _ := recordImage(r); return n == 0 })
	if at := fs.firstFence(); at.Before(deadline) {
		t.Fatalf("the orphan was removed %s before the maintenance deadline", deadline.Sub(at))
	}
}

// R2-VERIFY: a verify that newly finds the client fleet-expired schedules
// the terminal steps, which then run as a maintenance pass: the record is
// finished and our lease removed, without waiting for the next refresh.
func TestVerifySchedulesTerminalWork(t *testing.T) {
	fs := &passCountVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	cs, clientID, r := refresherWorkerForTest(t, fs)
	cs.startLeaseBootstrap(clientID)
	waitFor(t, "the publication", func() bool { _, _, proven, _ := recordImage(r); return proven > 0 })
	if err := cs.publishExpiredMarker(clientID); err != nil {
		t.Fatal(err)
	}
	cs.requestLeaseVerify(clientID)
	waitFor(t, "the terminal steps", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.finished && r.state == leaseRecordFleetExpired
	})
	if _, err := fs.TernVFS.Lookup(InodeID(clientID), cs.leaseName); err == nil {
		t.Fatal("the terminal steps left our lease")
	}
}

// R2-VERIFY: an expiry this process notes (noteFleetExpired) makes the
// terminal steps due at once, rather than at the next scheduled refresh.
func TestNotedExpiryRunsTerminalSteps(t *testing.T) {
	fs := &passCountVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	cs, clientID, r := refresherWorkerForTest(t, fs)
	cs.startLeaseBootstrap(clientID)
	waitFor(t, "the publication", func() bool { _, _, proven, _ := recordImage(r); return proven > 0 })
	waitFor(t, "the next refresh to be scheduled", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return time.Until(r.maintainAt) > testChannelTimeout
	})
	cs.noteFleetExpired(clientID)
	waitFor(t, "the terminal steps", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.finished
	})
}
