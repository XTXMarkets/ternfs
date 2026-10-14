// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// The lease gate: a read error returns it to live with nothing published;
// an active or quiet group is never expired; expiry refuses admission.
func TestLeaseGateDecisions(t *testing.T) {
	fs := &admissionLeaseVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.clients.StopLeaseWorkers)
	clientID, confirm, err := srv.clients.SetClientID([8]byte{1}, []byte("gate"), clientstore.ClientOwner{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(clientID, confirm, clientstore.RPCPrincipal{}); err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	moveSessionForTest(srv.sessions, sess, clientID, sess.id)
	srv.clients.PinLeaseForTest(clientID)
	srv.clients.SetClock(func() time.Time { return time.Now().Add(10 * clientstore.LeaseTime) })
	gate := func() leaseGateState {
		srv.sessions.mu.Lock()
		defer srv.sessions.mu.Unlock()
		return srv.sessions.groups[clientID].lease
	}

	// Active: no decision, no read.
	hold, status := srv.sessions.holdRequest(sess)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	hold.activate()
	fs.fail = true
	if expired, err := srv.decideExpired(clientID, true); expired || err != nil {
		t.Fatalf("active client decided %t %v", expired, err)
	}
	// Released: quiet until clientstore.LeaseTime after the reply.
	hold.release()
	srv.sessions.mu.Lock()
	quiet := srv.sessions.groups[clientID].quietUntil
	srv.sessions.mu.Unlock()
	if quiet.Before(srv.sessions.now().Add(clientstore.LeaseTime - time.Second)) {
		t.Fatalf("quiet until %s", quiet)
	}
	if expired, _ := srv.decideExpired(clientID, true); expired {
		t.Fatal("quiet client expired")
	}
	endQuietForTest(srv)
	// A read error: live again, no marker, state kept.
	if expired, err := srv.decideExpired(clientID, true); expired || err == nil {
		t.Fatalf("read error decided %t %v", expired, err)
	}
	if gate() != gateLive {
		t.Fatal("read error left the gate deciding")
	}
	fs.fail = false
	if _, err := fs.Lookup(vfs.InodeID(clientID), clientstore.ExpiredName); err == nil {
		t.Fatal("marker published on a read error")
	}
	if srv.sessions.lookup(ts.id) == nil {
		t.Fatal("read error retired the session")
	}

	// Expired: admission is refused.
	srv.sessions.mu.Lock()
	srv.sessions.groups[clientID].lease = gateExpired
	srv.sessions.mu.Unlock()
	if _, status := srv.sessions.holdRequest(sess); status != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("admission after expiry %s", nfs4.Nfsstat4Name(status))
	}
	srv.sessions.mu.Lock()
	srv.sessions.groups[clientID].lease = gateChecking
	srv.sessions.mu.Unlock()
	if _, status := srv.sessions.holdRequest(sess); status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("admission while deciding %s", nfs4.Nfsstat4Name(status))
	}
}

// SEQUENCE never calls the store: admission and its DELAY both answer from
// memory while every backend call blocks.
func TestSequenceAdmissionDoesNoStoreIO(t *testing.T) {
	fs := &clientstore.TrapVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()), Release: make(chan struct{})}
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	body := buildCompoundBody(nil, func(*nfs4.COMPOUND4argsWriter) {})
	fs.Mu.Lock()
	fs.Armed = true
	fs.Mu.Unlock()
	defer close(fs.Release)
	done := make(chan uint32, 2)
	go func() {
		_, res := admissionRequest(t, srv, admissionSequence(body, ts, 0, 1, true), 1, nil)
		done <- res.Status()
		setLeaseForTest(srv, sess.clientID, func(r *clientstore.LeaseRefresher) { r.Proven = 0 })
		_, res = admissionRequest(t, srv, admissionSequence(body, ts, 0, 2, true), 2, nil)
		done <- res.Status()
	}()
	for _, want := range []uint32{nfs4.NFS4_OK, nfs4.NFS4ERR_DELAY} {
		if got := testutil.AwaitValue(t, done, "SEQUENCE with a blocked store"); got != want {
			t.Fatalf("SEQUENCE %s, want %s", nfs4.Nfsstat4Name(got), nfs4.Nfsstat4Name(want))
		}
	}
	fs.Mu.Lock()
	defer fs.Mu.Unlock()
	if len(fs.Calls) != 0 {
		t.Fatalf("SEQUENCE called the store: %v", fs.Calls)
	}
}

// A replay renews: its commit activates a hold like a new request.
func TestSequenceReplayRenews(t *testing.T) {
	srv, err := NewServer(vfs.NewLocalTernVFS(t.TempDir()), staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	body := buildCompoundBody(nil, func(*nfs4.COMPOUND4argsWriter) {})
	wire := admissionSequence(body, ts, 0, 1, true)
	admissionRequest(t, srv, wire, 1, nil)
	endQuietForTest(srv)
	admissionRequest(t, srv, wire, 2, nil)
	srv.sessions.mu.Lock()
	defer srv.sessions.mu.Unlock()
	g := srv.sessions.groups[sess.clientID]
	if g == nil || !g.quietUntil.After(time.Now()) {
		t.Fatal("replay did not renew")
	}
}

// An idle client's first SEQUENCE gets DELAY while a verify reads its
// lease; the worker's read-only pass then admits the retry.
func TestIdleClientResumesAfterVerify(t *testing.T) {
	fs := &clientstore.RefresherFaultVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	conn := dial(t, addr)
	defer conn.Close()
	ts, err := createTestSession(conn, 1, "idle-client", [8]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	clientID := incarnationForTest(t, srv, ts.clientID)
	setLeaseForTest(srv, clientID, func(r *clientstore.LeaseRefresher) { r.VerifiedAt = 0 })
	fs.Set(func() { fs.Constructs = 0 })
	body := buildCompoundBody(nil, func(*nfs4.COMPOUND4argsWriter) {})
	reply, err := sendRawRPC(conn, 2, procCompound, withSequence(body, ts, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := sequenceStatus(reply); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("idle SEQUENCE %s, want DELAY", nfs4.Nfsstat4Name(got))
	}
	for deadline := time.Now().Add(testutil.ChannelTimeout); ; time.Sleep(time.Millisecond) {
		reply, err = sendRawRPC(conn, 3, procCompound, withSequence(body, ts, 0, 1))
		if err != nil {
			t.Fatal(err)
		}
		if sequenceStatus(reply) == nfs4.NFS4_OK {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("retry %s", nfs4.Nfsstat4Name(sequenceStatus(reply)))
		}
	}
	fs.Mu.Lock()
	defer fs.Mu.Unlock()
	if fs.Constructs != 0 {
		t.Fatal("the verify wrote a lease")
	}
}

// CREATE_SESSION replies once the worker has published the client's first
// lease, so a client that treats DELAY as failure (libnfs at mount) gets OK
// from its first SEQUENCE.
func TestCreateSessionWaitsForFirstPublication(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	clientID, seq, status, err := exchangeID(conn, 1, "first-sequence", [8]byte{6})
	if err != nil || status != nfs4.NFS4_OK {
		t.Fatalf("EXCHANGE_ID %s %v", nfs4.Nfsstat4Name(status), err)
	}
	reply, err := sendRawRPC(conn, 2, procCompound, backCreateSessionBody(clientID, seq, 0))
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseCompoundReply(reply)
	if err != nil || res.Status() != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %v %v", res.Status(), err)
	}
	it := res.Resarray()
	it.Next()
	ok := it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok()
	if !srv.clients.LeaseProven(vfs.InodeID(incarnationForTest(t, srv, clientID))) {
		t.Fatal("CREATE_SESSION replied before the first publication")
	}
	ts := newTestSession(readSessionID(ok.CsrSessionid()), clientID, ok.CsrForeChanAttrs().CaMaxrequests())
	body := buildCompoundBody(nil, func(*nfs4.COMPOUND4argsWriter) {})
	reply, err = sendRawRPC(conn, 3, procCompound, withSequence(body, ts, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := sequenceStatus(reply); got != nfs4.NFS4_OK {
		t.Fatalf("first SEQUENCE %s", nfs4.Nfsstat4Name(got))
	}
}

// Local expiry retires the client's sessions itself, cancelling their
// callbacks and returning their credits, even when no expired marker could
// be written. It does not claim fleet expiry.
func TestLocalExpiryDrainsSessionsWithoutMarker(t *testing.T) {
	fs := &expiryGateVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()), failMarker: true}
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	before := srv.sessions.accounting()
	f := newCBSessionOn(t, srv, addr)
	f.client.answerNull(t)
	sess := f.session(t)
	inc := sess.clientID
	f.pressure(t, true)
	f.sequence(t)
	if _, ok := f.client.nextCall(testutil.ChannelTimeout); !ok {
		t.Fatal("no recall outstanding")
	}
	f.pressure(t, false)

	// The durable lease dies; the marker write fails.
	srv.clients.StopLeaseWorkers()
	later := time.Now().Add(10 * clientstore.LeaseWriteTerm)
	srv.clients.SetClock(func() time.Time { return later })
	endQuietForTest(srv)
	held := srv.sessions.accounting()
	end, ok := srv.beginDestroy(inc)
	if !ok {
		t.Fatal("protected")
	}
	expired, err := srv.decideExpired(inc, true)
	end()
	if !expired || err != nil {
		t.Fatalf("decideExpired %t %v", expired, err)
	}
	if _, err := fs.TernVFS.Lookup(vfs.InodeID(inc), clientstore.ExpiredName); err == nil {
		t.Fatal("the marker exists; the fixture should have failed it")
	}
	if srv.sessions.lookup(sess.id) != nil {
		t.Fatal("expired client's session still attached")
	}
	waitFor(t, "callback sender exit", func() bool {
		sess.cb.mu.Lock()
		defer sess.cb.mu.Unlock()
		return !sess.cb.running
	})
	// The session's record is returned only once its sender has exited;
	// its cached replies go with it.
	waitFor(t, "session credits", func() bool {
		a := srv.sessions.accounting()
		return a.Sessions == held.Sessions-1 && a.Records == held.Records-1 && a.CacheBytes < held.CacheBytes
	})
	_ = before
	srv.sessions.mu.Lock()
	for _, other := range srv.sessions.sessions {
		if other.clientID == inc {
			t.Error("a session of the expired client remains")
		}
	}
	srv.sessions.mu.Unlock()
	srv.clients.RefreshMu.Lock()
	r := srv.clients.Refreshers[vfs.InodeID(inc)]
	srv.clients.RefreshMu.Unlock()
	if r != nil {
		r.Mu.Lock()
		state := r.State
		r.Mu.Unlock()
		if state == clientstore.LeaseRecordFleetExpired {
			t.Fatal("local expiry recorded fleet expiry without a marker")
		}
	}
	// Later sweeps change nothing more and stay bounded.
	srv.sweepExpiredClientState()
	if _, err := fs.TernVFS.Lookup(vfs.InodeID(inc), clientstore.ExpiredName); err == nil {
		t.Fatal("marker appeared")
	}
}

// A stale refresher record's terminal cleanup is a destructive path. It
// takes the destruction hold before removing the lease, so while an
// unresolved confirmation protects the incarnation nothing is removed or
// retired; once the confirmation is settled, the cleanup runs.
func TestRefresherTerminalRespectsConfirmationProtection(t *testing.T) {
	f := newConfirmServer(t)
	srv := f.srv
	srv.clients.StopLeaseWorkers()
	f.arm(clientstore.WriteD, clientstore.FaultFenceErr, nil)
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("CREATE_SESSION with an unfenceable D %s", nfs4.Nfsstat4Name(got))
	}
	f.disarm()
	if !srv.protected(f.p) {
		t.Fatal("P is not protected")
	}
	if !f.present(t, vfs.InodeID(f.p), srv.clients.LeaseName) {
		t.Fatal("P has no lease to protect")
	}
	pSession := f.pSession(t)
	// P's refresher finds it stale, as after D applied.
	setLeaseForTest(srv, f.p, func(r *clientstore.LeaseRefresher) { r.State = clientstore.LeaseRecordStale })
	leasePassForTest(t, srv, f.p)
	if !f.present(t, vfs.InodeID(f.p), srv.clients.LeaseName) {
		t.Fatal("terminal cleanup removed P's lease while it was protected")
	}
	if srv.sessions.lookup(pSession) == nil || len(srv.opens.ClientStates(f.p)) == 0 {
		t.Fatal("terminal cleanup retired P while it was protected")
	}
	// Settled: D never applied. The next pass after the backoff cleans up.
	f.fs.Set(clientstore.WriteD, clientstore.FaultNone, nil)
	srv.runLeaseSweep()
	if srv.protected(f.p) {
		t.Fatal("still protected")
	}
	setLeaseForTest(srv, f.p, func(r *clientstore.LeaseRefresher) {
		r.State, r.NotBefore = clientstore.LeaseRecordStale, time.Time{}
	})
	leasePassForTest(t, srv, f.p)
	if f.present(t, vfs.InodeID(f.p), srv.clients.LeaseName) {
		t.Fatal("terminal cleanup did not remove the lease once permitted")
	}
	if srv.sessions.lookup(pSession) != nil {
		t.Fatal("terminal cleanup did not retire P's session once permitted")
	}
}

func groupQuiet(srv *Server, clientID uint64) time.Time {
	srv.sessions.mu.Lock()
	defer srv.sessions.mu.Unlock()
	if g := srv.sessions.groups[clientID]; g != nil {
		return g.quietUntil
	}
	return time.Time{}
}

// An active client's requests do not drive refresher passes. Each
// SEQUENCE notes the need, but while a refresh is scheduled a wake changes
// nothing.
func TestRequestsDoNotDriveRefresherPasses(t *testing.T) {
	fs := &clientstore.PassCountVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fs.LeaseName = srv.clients.LeaseName
	addr, stop := serveTestServer(t, srv)
	defer stop()
	c := newCBTestClient(t, addr)
	clientID, seq := exchangeOn(t, c, 1, "storm")
	id, _ := createdSession(t, c.compound(2, backCreateSessionBody(clientID, seq, 0)))
	ts := newTestSession(id, clientID, 16)
	body := buildCompoundBody(nil, func(*nfs4.COMPOUND4argsWriter) {})
	// The first SEQUENCE turns need on; let its pass finish.
	if res := c.compound(3, withSequence(body, ts, 0, 1)); res.Status() != nfs4.NFS4_OK {
		t.Fatalf("SEQUENCE %s", nfs4.Nfsstat4Name(res.Status()))
	}
	time.Sleep(100 * time.Millisecond)
	fs.Reset()
	for i := uint32(2); i <= 201; i++ {
		if res := c.compound(2+i, withSequence(body, ts, 0, i)); res.Status() != nfs4.NFS4_OK {
			t.Fatalf("SEQUENCE %d %s", i, nfs4.Nfsstat4Name(res.Status()))
		}
	}
	if reads, _, writes, _ := fs.Counts(); reads > 1 || writes != 0 {
		t.Fatalf("200 SEQUENCEs drove %d passes and %d writes", reads, writes)
	}
}

// A failed pass's backoff is a lower bound for settlement and writes,
// whatever wakes the worker. Verify requests are coalesced, at least
// clientstore.LeaseVerifySpacing apart, and before the deadline only read.
func TestWakesDoNotShortenBackoff(t *testing.T) {
	fs := &clientstore.PassCountVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	cs, clientID, r := clientstore.RefresherWorkerForTest(t, fs)
	// The publication's rename fails, and so does its fence.
	fs.Mu.Lock()
	fs.FailRename, fs.FailFence = true, true
	fs.Mu.Unlock()
	fs.Reset()
	cs.StartLeaseBootstrap(clientID)
	waitFor(t, "the failed publication", func() bool { a, _, _, _ := clientstore.RecordImage(r); return a })
	start := time.Now()
	_, fences0, writes0, _ := fs.Counts()
	for range 50 {
		cs.NoteLeaseNeed(clientID)
		cs.RequestLeaseVerify(clientID)
		time.Sleep(20 * time.Millisecond)
	}
	elapsed := time.Since(start)
	reads, fences, writes, _ := fs.Counts()
	r.Mu.Lock()
	deadline := r.NotBefore
	r.Mu.Unlock()
	if !deadline.After(start) {
		t.Fatalf("no backoff deadline after the failure (elapsed %s)", elapsed)
	}
	if fences != fences0 || writes != writes0 {
		t.Fatalf("wakes drove %d fences and %d writes inside the backoff", fences-fences0, writes-writes0)
	}
	if maxPasses := int(elapsed/clientstore.LeaseVerifySpacing) + 2; reads > maxPasses {
		t.Fatalf("%d verify passes in %s", reads, elapsed)
	}
}

// With server hooks the store's Renew does not run a pass
// (the worker owns every pass and a server's terminal steps take identity
// ownership under leaseLocks); it reads the record and writes nothing.
func TestRenewWithServerHooksDoesNotPass(t *testing.T) {
	fs := &clientstore.PassCountVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.clients.StopLeaseWorkers)
	fs.LeaseName = srv.clients.LeaseName
	clientID, confirm, err := srv.clients.SetClientID([8]byte{1}, []byte("renew"), clientstore.ClientOwner{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(clientID, confirm, clientstore.RPCPrincipal{}); err != nil {
		t.Fatal(err)
	}
	publishLeaseForTest(t, srv, clientID)
	srv.clients.StopLeaseWorkers()
	// Due for a refresh, but still live.
	later := time.Now().Add(clientstore.LeaseRenewAfter)
	srv.clients.SetClock(func() time.Time { return later })
	fs.Reset()
	if err := srv.clients.Renew(clientID); err != nil {
		t.Fatalf("Renew %v", err)
	}
	if reads, fences, writes, _ := fs.Counts(); reads != 0 || fences != 0 || writes != 0 {
		t.Fatalf("Renew ran a pass: reads %d fences %d writes %d", reads, fences, writes)
	}
}

// fenceClockVFS is passCountVFS that also records when the first temp was
// removed.
type fenceClockVFS struct {
	*clientstore.PassCountVFS
	at time.Time
}

func (fs *fenceClockVFS) RemoveEdge(dir vfs.InodeID, name string, edge vfs.Edge) error {
	if clientstore.LeaseTemp(name) {
		fs.Mu.Lock()
		if fs.at.IsZero() {
			fs.at = time.Now()
		}
		fs.Mu.Unlock()
	}
	return fs.PassCountVFS.RemoveEdge(dir, name, edge)
}

func (fs *fenceClockVFS) firstFence() time.Time {
	fs.Mu.Lock()
	defer fs.Mu.Unlock()
	return fs.at
}

// Verify requests do not run maintenance. An unresolved LINK's
// temp appears after a successful publication, so no failure deadline is
// active; the next maintenance pass is the obligation recheck. Verifies
// before it only read: the orphan is not consumed and the deadline does not
// move. When it is due, maintenance runs with no further request.
func TestVerifyDoesNotRunMaintenance(t *testing.T) {
	counts := &clientstore.PassCountVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	fs := &fenceClockVFS{PassCountVFS: counts}
	cs, clientID, r, _ := clientstore.RefresherForTest(t, fs)
	counts.LeaseName = cs.LeaseName
	cs.Now = time.Now
	cs.LeaseNeed = func(uint64) bool { return true }
	cs.LeaseSessions = func(uint64) bool { return true }
	t.Cleanup(cs.StopLeaseWorkers)
	dir := vfs.InodeID(clientID)

	// A LINK whose outcome was unknown: its orphan is recorded, unresolved,
	// and its temp is not visible yet.
	inode, cookie, err := counts.TernVFS.ConstructFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	unlock := cs.LeaseLocks.Lock(r.Incarnation)
	o, ok := r.TakeOrphan()
	if !ok {
		unlock()
		t.Fatal("no orphan credit")
	}
	r.Mu.Lock()
	o.Inode, o.Phase = inode, clientstore.OrphanUnresolved
	r.Mu.Unlock()
	unlock()

	// A maintenance pass publishes successfully: no failure deadline, and
	// the orphan is an obligation to recheck at the next maintenance.
	cs.StartLeaseBootstrap(clientID)
	waitFor(t, "the publication", func() bool { _, _, proven, _ := clientstore.RecordImage(r); return proven > 0 })
	waitFor(t, "the pass to schedule", func() bool {
		r.Mu.Lock()
		defer r.Mu.Unlock()
		return !r.MaintainAt.IsZero() && r.NotBefore.IsZero() && !r.InCall
	})
	// Bring the scheduled recheck in, so the test need not wait
	// leaseBackoffMax; verifies must neither move nor run it.
	deadline := time.Now().Add(2500 * time.Millisecond)
	r.Mu.Lock()
	r.MaintainAt = deadline
	r.Mu.Unlock()

	// The LINK applies late: the orphan's temp is now visible.
	data, _ := json.Marshal(clientstore.DurableLease{ExpiresUnixNano: time.Now().Add(clientstore.LeaseWriteTerm).UnixNano()})
	if err := counts.TernVFS.LinkFile(inode, cookie, dir, o.Temp, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	counts.Reset()
	for time.Until(deadline) > 300*time.Millisecond {
		cs.RequestLeaseVerify(clientID)
		time.Sleep(20 * time.Millisecond)
	}
	reads, fences, writes, removes := counts.Counts()
	_, orphans, _, _ := clientstore.RecordImage(r)
	r.Mu.Lock()
	moved := !r.MaintainAt.Equal(deadline)
	r.Mu.Unlock()
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
	waitFor(t, "maintenance at its deadline", func() bool { _, n, _, _ := clientstore.RecordImage(r); return n == 0 })
	if at := fs.firstFence(); at.Before(deadline) {
		t.Fatalf("the orphan was removed %s before the maintenance deadline", deadline.Sub(at))
	}
}

// A verify that newly finds the client fleet-expired schedules
// the terminal steps, which then run as a maintenance pass: the record is
// finished and our lease removed, without waiting for the next refresh.
func TestVerifySchedulesTerminalWork(t *testing.T) {
	fs := &clientstore.PassCountVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	cs, clientID, r := clientstore.RefresherWorkerForTest(t, fs)
	cs.StartLeaseBootstrap(clientID)
	waitFor(t, "the publication", func() bool { _, _, proven, _ := clientstore.RecordImage(r); return proven > 0 })
	if err := cs.PublishExpiredMarker(clientID); err != nil {
		t.Fatal(err)
	}
	cs.RequestLeaseVerify(clientID)
	waitFor(t, "the terminal steps", func() bool {
		r.Mu.Lock()
		defer r.Mu.Unlock()
		return r.Finished && r.State == clientstore.LeaseRecordFleetExpired
	})
	if _, err := fs.TernVFS.Lookup(vfs.InodeID(clientID), cs.LeaseName); err == nil {
		t.Fatal("the terminal steps left our lease")
	}
}

// An expiry this process notes (noteFleetExpired) makes the
// terminal steps due at once, rather than at the next scheduled refresh.
func TestNotedExpiryRunsTerminalSteps(t *testing.T) {
	fs := &clientstore.PassCountVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	cs, clientID, r := clientstore.RefresherWorkerForTest(t, fs)
	cs.StartLeaseBootstrap(clientID)
	waitFor(t, "the publication", func() bool { _, _, proven, _ := clientstore.RecordImage(r); return proven > 0 })
	waitFor(t, "the next refresh to be scheduled", func() bool {
		r.Mu.Lock()
		defer r.Mu.Unlock()
		return time.Until(r.MaintainAt) > testutil.ChannelTimeout
	})
	cs.NoteFleetExpired(clientID)
	waitFor(t, "the terminal steps", func() bool {
		r.Mu.Lock()
		defer r.Mu.Unlock()
		return r.Finished
	})
}

// leaseRecordForTest returns the incarnation's record, or nil.
func leaseRecordForTest(srv *Server, inc uint64) *clientstore.LeaseRefresher {
	cs := srv.clients
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	return cs.Refreshers[vfs.InodeID(inc)]
}

func destroyClientBody(clientID uint64) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_DestroyClientid().SetDcaClientid(clientID)
	})
}

// A client that bootstraps and then goes (DESTROY_SESSION,
// DESTROY_CLIENTID) leaves an idle, unreferenced record whose worker is
// waiting. The sweeper's wake reclaims it at once, so with one record credit
// the next client registers and creates its session, cycle after cycle,
// without waiting out the idle deadline.
func TestIdleRecordReclaimedForNextClient(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	srv.clients.RefreshMu.Lock()
	srv.clients.RefresherLimit = 1
	srv.clients.RefreshMu.Unlock()
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
			r.Mu.Lock()
			defer r.Mu.Unlock()
			return r.IdleWaiting && r.Proven > 0 && !r.InCall
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

// A record is not reclaimed while its incarnation's
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
		r.Mu.Lock()
		defer r.Mu.Unlock()
		return !r.InCall && time.Until(r.MaintainAt) > 5*time.Second
	})
	deadlines := func() [3]any {
		r.Mu.Lock()
		defer r.Mu.Unlock()
		return [3]any{r.MaintainAt, r.NotBefore, r.Backoff}
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
	release, ok := srv.clients.ReserveLeaseRecord(inc)
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

// The reclamation wake does not touch an obligation. A record with
// an unsettled publication attempt in its backoff, and nothing else
// referencing it, is kept through repeated sweeps with no fence or write
// before its deadline, and its deadline does not move. After the deadline
// the attempt is fenced, and only then is the record freed.
func TestIdleReclaimKeepsObligations(t *testing.T) {
	counts := &clientstore.PassCountVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	fs := &fenceClockVFS{PassCountVFS: counts}
	cs, clientID := clientstore.NewConfirmedStoreClient(t, fs, []byte("client"), [8]byte{1}, clientstore.ClientOwner{})
	counts.LeaseName = cs.LeaseName
	cs.Now = time.Now
	var need atomic.Bool
	need.Store(true)
	cs.LeaseNeed = func(uint64) bool { return need.Load() }
	cs.LeaseSessions = func(uint64) bool { return false }
	t.Cleanup(cs.StopLeaseWorkers)
	release, ok := cs.ReserveLeaseRecord(clientID)
	if !ok {
		t.Fatal("no record credit")
	}
	cs.RefreshMu.Lock()
	r := cs.Refreshers[vfs.InodeID(clientID)]
	cs.RefreshMu.Unlock()
	// The publication's rename fails, and so does the first fence.
	counts.Mu.Lock()
	counts.FailRename, counts.FailFence = true, true
	counts.Mu.Unlock()
	cs.NoteLeaseNeed(clientID)
	waitFor(t, "the failed publication", func() bool {
		r.Mu.Lock()
		defer r.Mu.Unlock()
		return r.Attempt != nil && !r.NotBefore.IsZero() && !r.InCall
	})
	r.Mu.Lock()
	deadline, backoff := r.NotBefore, r.Backoff
	r.Mu.Unlock()
	// Nothing references the record but its attempt.
	need.Store(false)
	release()
	counts.Mu.Lock()
	counts.FailRename, counts.FailFence = false, false
	counts.Mu.Unlock()
	_, fences0, writes0, _ := counts.Counts()
	for time.Until(deadline) > 500*time.Millisecond {
		cs.SweepLeaseRecords()
		time.Sleep(50 * time.Millisecond)
	}
	_, fences, writes, _ := counts.Counts()
	attempt, _, _, _ := clientstore.RecordImage(r)
	r.Mu.Lock()
	moved := !r.NotBefore.Equal(deadline) || r.Backoff != backoff
	r.Mu.Unlock()
	if fences != fences0 || writes != writes0 {
		t.Fatalf("sweeps inside the backoff drove %d fences and %d writes", fences-fences0, writes-writes0)
	}
	cs.RefreshMu.Lock()
	kept := cs.Refreshers[vfs.InodeID(clientID)] == r
	cs.RefreshMu.Unlock()
	if !attempt || !kept {
		t.Fatalf("the sweeps released the obligation: attempt %v, record kept %v", attempt, kept)
	}
	if moved {
		t.Fatal("the sweeps moved the backoff")
	}
	// After the deadline the worker fences the attempt, then frees the record.
	for limit := time.Now().Add(deadline.Sub(time.Now()) + testutil.ChannelTimeout); ; time.Sleep(10 * time.Millisecond) {
		cs.RefreshMu.Lock()
		freed := cs.Refreshers[vfs.InodeID(clientID)] == nil
		cs.RefreshMu.Unlock()
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
	if attempt, _, _, _ := clientstore.RecordImage(r); attempt {
		t.Fatal("freed with its attempt unsettled")
	}
}
