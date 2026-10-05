// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"

	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// confirmedLookupHold blocks the first Lookup of the confirmed pointer once
// armed: in HasOpen that is the first read of an operation's durable-open
// check.
type confirmedLookupHold struct {
	TernVFS
	mu      sync.Mutex
	armed   bool
	entered chan struct{}
	release chan struct{}
}

func (fs *confirmedLookupHold) Lookup(dir InodeID, name string) (InodeID, error) {
	fs.mu.Lock()
	hold := fs.armed && name == confirmedName
	if hold {
		fs.armed = false
	}
	fs.mu.Unlock()
	if hold {
		close(fs.entered)
		<-fs.release
	}
	return fs.TernVFS.Lookup(dir, name)
}

// A1: an admitted CLOSE whose durable-open check finds the durable lease dead
// gets EXPIRED within bounded time, and the client's other state is left
// alone while its logical lease is held: local expiry is the gate's
// decision. (Before, the operation expired the client itself, re-entering
// the staging pathname lock CLOSE holds.)
func TestActiveCompoundDoesNotExpireClientState(t *testing.T) {
	fs := &confirmedLookupHold{TernVFS: NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	staging, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	defer closeSignal(fs.release)
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	inc := incarnationForTest(t, srv, clientID)
	aSid, aFH := openCreateFile(t, conn, &xid, clientID, "a")
	writeFileAt(t, conn, &xid, aFH, aSid, 0, []byte("A's bytes"))
	bSid, bFH := openCreateFile(t, conn, &xid, clientID, "b")
	writeFileAt(t, conn, &xid, bFH, bSid, 0, []byte("B's bytes"))
	bStaging := stagingIDForTest(t, staging, bSid)

	// No more refreshes: the lease stays as published, then dies.
	srv.clients.stopLeaseWorkers()
	var skew atomic.Int64
	srv.clients.setClock(func() time.Time { return time.Now().Add(time.Duration(skew.Load())) })
	fs.mu.Lock()
	fs.armed = true
	fs.mu.Unlock()
	closed := make(chan uint32, 1)
	go func() {
		x := uint32(500)
		status, _, err := closeFileResultE(conn, &x, aFH, aSid)
		if err != nil {
			t.Error(err)
		}
		closed <- status
	}()
	awaitSignal(t, fs.entered, "CLOSE's durable-open check")
	skew.Store(int64(10 * leaseWriteTerm))
	closeSignal(fs.release)
	if got := awaitValue(t, closed, "CLOSE with a dead durable lease"); got != NFS4ERR_EXPIRED {
		t.Fatalf("CLOSE %s, want EXPIRED", Nfsstat4Name(got))
	}

	var bID StateID
	copy(bID[:], bSid[4:])
	if !srv.opens.live(bID) {
		t.Fatal("the operation expired the client's other open")
	}
	if m, ok := staging.GetMeta(bStaging); !ok || m.Retired {
		t.Fatal("the operation retired the client's other staging")
	}
	if _, err := fs.TernVFS.Lookup(InodeID(inc), activeOpenName(bID)); err != nil {
		t.Fatalf("the operation removed the other open's marker: %v", err)
	}
	// The gate decides: not while the logical lease is held...
	end, _ := srv.beginDestroy(inc)
	if expired, _ := srv.decideExpired(inc, true); expired {
		t.Fatal("expired during the quiet period")
	}
	end()
	// ...and once it is not, the sweeper's decision expires the client.
	endQuietForTest(srv)
	srv.sweepExpiredClientState()
	if srv.opens.live(bID) {
		t.Fatal("sweeper did not expire the dead client")
	}
	assertRetiredStaging(t, staging, bStaging)
}

// A11: the association quota counts distinct IDs. An ID committed by one
// token while another for it is still pending counts once, so the 64th
// distinct ID is admitted.
func TestAssociationQuotaCountsDistinctIDs(t *testing.T) {
	srv := admissionWritableServer(t, NewLocalTernVFS(t.TempDir()))
	conn := &rpcConnection{}
	for range maxConnectionSessions - 2 {
		if !conn.bind(srv, connectionSession(t, srv).id) {
			t.Fatal("bind")
		}
	}
	x := connectionSession(t, srv).id
	t1, ok1 := conn.reserve(srv, x)
	t2, ok2 := conn.reserve(srv, x)
	if !ok1 || !ok2 {
		t.Fatal("reserve X")
	}
	t1.commit()
	y, ok := conn.reserve(srv, connectionSession(t, srv).id)
	if !ok {
		t.Fatal("the 64th distinct ID was refused")
	}
	if _, ok := conn.reserve(srv, connectionSession(t, srv).id); ok {
		t.Fatal("a 65th distinct ID was admitted")
	}
	t2.abort()
	y.abort()
	if !conn.associated(x) {
		t.Fatal("aborting the pending token removed X's committed association")
	}
}

// A4: a stale refresher record's terminal cleanup is a destructive path. It
// takes the destruction hold before removing the lease, so while an
// unresolved confirmation protects the incarnation nothing is removed or
// retired; once the confirmation is settled, the cleanup runs.
func TestRefresherTerminalRespectsConfirmationProtection(t *testing.T) {
	f := newConfirmServer(t)
	srv := f.srv
	srv.clients.stopLeaseWorkers()
	f.arm(writeD, faultFenceErr, nil)
	if got, _ := f.rawCreate(t, f.wireN, 1); got != NFS4ERR_DELAY {
		t.Fatalf("CREATE_SESSION with an unfenceable D %s", Nfsstat4Name(got))
	}
	f.disarm()
	if !srv.protected(f.p) {
		t.Fatal("P is not protected")
	}
	if !f.present(t, InodeID(f.p), srv.clients.leaseName) {
		t.Fatal("P has no lease to protect")
	}
	pSession := f.pSession(t)
	// P's refresher finds it stale, as after D applied.
	setLeaseForTest(srv, f.p, func(r *leaseRefresher) { r.state = leaseRecordStale })
	leasePassForTest(t, srv, f.p)
	if !f.present(t, InodeID(f.p), srv.clients.leaseName) {
		t.Fatal("terminal cleanup removed P's lease while it was protected")
	}
	if srv.sessions.lookup(pSession) == nil || len(srv.opens.clientStates(f.p)) == 0 {
		t.Fatal("terminal cleanup retired P while it was protected")
	}
	// Settled: D never applied. The next pass after the backoff cleans up.
	f.fs.set(writeD, faultNone, nil)
	srv.runLeaseSweep()
	if srv.protected(f.p) {
		t.Fatal("still protected")
	}
	setLeaseForTest(srv, f.p, func(r *leaseRefresher) {
		r.state, r.notBefore = leaseRecordStale, time.Time{}
	})
	leasePassForTest(t, srv, f.p)
	if f.present(t, InodeID(f.p), srv.clients.leaseName) {
		t.Fatal("terminal cleanup did not remove the lease once permitted")
	}
	if srv.sessions.lookup(pSession) != nil {
		t.Fatal("terminal cleanup did not retire P's session once permitted")
	}
}

// A5: a recovered writer's downgrade that keeps WRITE installs its grant and
// generation as live state, so the old generation is OLD_STATEID and a
// later upgrade is INVAL; its staging is kept and its CLOSE publishes.
func TestRecoveredDowngradeKeepsItsGrant(t *testing.T) {
	name := "recovered-grant.txt"
	r := restartWithStagedWrite(t, name, []byte("staged data"))
	conn, xid := r.conn, r.xid
	stagingID := stagingIDForTest(t, r.staging, r.stateid)
	status, write := downgradeResult(t, conn, &xid, r.fh, r.stateid, OPEN4_SHARE_ACCESS_WRITE, OPEN4_SHARE_DENY_NONE)
	if status != NFS4_OK {
		t.Fatalf("recovered BOTH to WRITE %s", Nfsstat4Name(status))
	}
	var id StateID
	copy(id[:], write[4:])
	if !r.srv.opens.live(id) {
		t.Fatal("the downgrade installed no live state")
	}
	if got := stateidOpStatus(t, conn, &xid, "WRITE", r.fh, r.stateid); got != NFS4ERR_OLD_STATEID {
		t.Fatalf("the old generation %s, want OLD_STATEID", Nfsstat4Name(got))
	}
	if got, _ := downgradeResult(t, conn, &xid, r.fh, write, OPEN4_SHARE_ACCESS_BOTH, OPEN4_SHARE_DENY_NONE); got != NFS4ERR_INVAL {
		t.Fatalf("WRITE to BOTH %s, want INVAL", Nfsstat4Name(got))
	}
	status, write2 := downgradeResult(t, conn, &xid, r.fh, write, OPEN4_SHARE_ACCESS_WRITE, OPEN4_SHARE_DENY_NONE)
	if status != NFS4_OK {
		t.Fatalf("same-value downgrade %s", Nfsstat4Name(status))
	}
	if m, ok := r.staging.GetMeta(stagingID); !ok || m.Retired {
		t.Fatal("the recovered staging was retired")
	}
	writeFileAt(t, conn, &xid, r.fh, write2, uint64(len("staged data")), []byte(" and more"))
	closeFile(t, conn, &xid, r.fh, write2)
	data, err := readPublished(r.fs, name)
	if err != nil || string(data) != "staged data and more" {
		t.Fatalf("published %q %v", data, err)
	}
}

// A5 collision: when the recovered writer's owner already has a live open
// of the file, a downgrade to READ cannot continue as that owner's read
// open. It gets DELAY before publication, keeping both opens, and succeeds
// once the other open is closed. It never falls through to CLOSE.
func TestRecoveredDowngradeCollisionWaits(t *testing.T) {
	name := "recovered-collision.txt"
	r := restartWithStagedWrite(t, name, []byte("staged data"))
	conn, xid := r.conn, r.xid
	stagingID := stagingIDForTest(t, r.staging, r.stateid)
	status, live, _ := openByHandle(t, conn, &xid, r.clientID, "test-owner-"+name, r.fh, OPEN4_SHARE_ACCESS_READ)
	if status != NFS4_OK {
		t.Fatalf("the owner's live OPEN %s", Nfsstat4Name(status))
	}
	before, _ := readPublished(r.fs, name)
	if got, _ := downgradeResult(t, conn, &xid, r.fh, r.stateid, OPEN4_SHARE_ACCESS_READ, OPEN4_SHARE_DENY_NONE); got != NFS4ERR_DELAY {
		t.Fatalf("colliding recovered downgrade %s, want DELAY", Nfsstat4Name(got))
	}
	var recovered, liveID StateID
	copy(recovered[:], r.stateid[4:])
	copy(liveID[:], live[4:])
	if after, _ := readPublished(r.fs, name); string(after) != string(before) {
		t.Fatal("the refused downgrade published")
	}
	if m, ok := r.staging.GetMeta(stagingID); !ok || m.Retired || m.NFSStateID != recovered {
		t.Fatal("the refused downgrade changed the recovered staging")
	}
	if !r.srv.opens.live(liveID) || r.srv.opens.live(recovered) {
		t.Fatal("the refused downgrade changed the open state")
	}
	if open, err := r.srv.clients.HasOpen(r.clientID, recovered); err != nil || !open {
		t.Fatalf("the recovered writer's marker %t %v", open, err)
	}
	closeFile(t, conn, &xid, r.fh, live)
	status, read := downgradeResult(t, conn, &xid, r.fh, r.stateid, OPEN4_SHARE_ACCESS_READ, OPEN4_SHARE_DENY_NONE)
	if status != NFS4_OK {
		t.Fatalf("retried downgrade %s", Nfsstat4Name(status))
	}
	if data, _ := readPublished(r.fs, name); string(data) != "staged data" {
		t.Fatalf("published %q", data)
	}
	closeFile(t, conn, &xid, r.fh, read)
}

func readPublished(fs TernVFS, name string) ([]byte, error) {
	id, err := fs.Lookup(fs.RootID(), name)
	if err != nil {
		return nil, err
	}
	return fs.ReadAll(id)
}

// addrOfTestServer finds a served test server's address in the registry.
func addrOfTestServer(t *testing.T, srv *Server) string {
	t.Helper()
	var addr string
	testServers.Range(func(k, v any) bool {
		if v.(*Server) == srv {
			addr = k.(string)
			return false
		}
		return true
	})
	if addr == "" {
		t.Fatal("server not registered")
	}
	return addr
}

// exchangeOn registers a client over c and returns its clientid and next
// CREATE_SESSION sequence.
func exchangeOn(t *testing.T, c *cbTestClient, xid uint32, owner string) (uint64, uint32) {
	t.Helper()
	res := c.compound(xid, exchangeIDBody(owner, [8]byte{9}))
	if res.Status() != NFS4_OK {
		t.Fatalf("EXCHANGE_ID %s", Nfsstat4Name(res.Status()))
	}
	it := res.Resarray()
	it.Next()
	ex := it.Resarray().Value().AsEXCHANGEID4resEntry().Value().AsEXCHANGEID4resok()
	return ex.EirClientid(), ex.EirSequenceid()
}

func createdSession(t *testing.T, res COMPOUND4res) (SessionID, uint32) {
	t.Helper()
	if res.Status() != NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", Nfsstat4Name(res.Status()))
	}
	it := res.Resarray()
	it.Next()
	ok := it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok()
	return readSessionID(ok.CsrSessionid()), ok.CsrFlags()
}

// recallReplyFrame builds a CB_COMPOUND reply with the given compound
// status and raw result words.
func recallReplyFrame(xid, compound uint32, results ...[]byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, xid)
	b = binary.BigEndian.AppendUint32(b, rpcReply)
	b = binary.BigEndian.AppendUint32(b, 0) // MSG_ACCEPTED
	b = binary.BigEndian.AppendUint32(b, authNone)
	b = binary.BigEndian.AppendUint32(b, 0)
	b = binary.BigEndian.AppendUint32(b, acceptSuccess)
	b = binary.BigEndian.AppendUint32(b, compound)
	b = binary.BigEndian.AppendUint32(b, 0) // tag
	b = binary.BigEndian.AppendUint32(b, uint32(len(results)))
	for _, r := range results {
		b = append(b, r...)
	}
	return b
}

func seqResult(status uint32, id SessionID, seq uint32) []byte {
	b := binary.BigEndian.AppendUint32(nil, OP_CB_SEQUENCE)
	b = binary.BigEndian.AppendUint32(b, status)
	if status == NFS4_OK {
		b = append(b, id[:]...)
		for _, v := range []uint32{seq, 0, 0, 0} {
			b = binary.BigEndian.AppendUint32(b, v)
		}
	}
	return b
}

func recallResult(status uint32) []byte {
	return binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, OP_CB_RECALL_SLOT), status)
}

func groupQuiet(srv *Server, clientID uint64) time.Time {
	srv.sessions.mu.Lock()
	defer srv.sessions.mu.Unlock()
	if g := srv.sessions.groups[clientID]; g != nil {
		return g.quietUntil
	}
	return time.Time{}
}

// passCountVFS counts the refresher's calls: lease reads (one per pass's
// evidence), fences and writes. Fences of temps can be made to fail.
type passCountVFS struct {
	TernVFS
	mu                                  sync.Mutex
	leaseName                           string
	leaseReads, fences, writes, removes int
	failRename, failFence               bool
}

func (fs *passCountVFS) Lookup(dir InodeID, name string) (InodeID, error) {
	fs.mu.Lock()
	if name != "" && name == fs.leaseName {
		fs.leaseReads++
	}
	fs.mu.Unlock()
	return fs.TernVFS.Lookup(dir, name)
}
func (fs *passCountVFS) ConstructFile(dir InodeID) (InodeID, Cookie, error) {
	fs.mu.Lock()
	fs.writes++
	fs.mu.Unlock()
	return fs.TernVFS.ConstructFile(dir)
}
func (fs *passCountVFS) RenameEdge(src InodeID, old string, edge Edge, dst InodeID, name string) error {
	fs.mu.Lock()
	fs.writes++
	fail := fs.failRename && leaseTemp(old)
	fs.mu.Unlock()
	if fail {
		return errInjectedLease
	}
	return fs.TernVFS.RenameEdge(src, old, edge, dst, name)
}
func (fs *passCountVFS) RemoveEdge(dir InodeID, name string, edge Edge) error {
	fs.mu.Lock()
	temp := leaseTemp(name)
	if temp {
		fs.fences++
	}
	fs.removes++
	fail := fs.failFence && temp
	fs.mu.Unlock()
	if fail {
		return errInjectedLease
	}
	return fs.TernVFS.RemoveEdge(dir, name, edge)
}
func (fs *passCountVFS) Remove(dir InodeID, name string) error {
	fs.mu.Lock()
	fs.removes++
	fs.mu.Unlock()
	return fs.TernVFS.Remove(dir, name)
}
func (fs *passCountVFS) counts() (reads, fences, writes, removes int) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.leaseReads, fs.fences, fs.writes, fs.removes
}
func (fs *passCountVFS) reset() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.leaseReads, fs.fences, fs.writes, fs.removes = 0, 0, 0, 0
}

// A10: an active client's requests do not drive refresher passes. Each
// SEQUENCE notes the need, but while a refresh is scheduled a wake changes
// nothing.
func TestRequestsDoNotDriveRefresherPasses(t *testing.T) {
	fs := &passCountVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fs.leaseName = srv.clients.leaseName
	addr, stop := serveTestServer(t, srv)
	defer stop()
	c := newCBTestClient(t, addr)
	clientID, seq := exchangeOn(t, c, 1, "storm")
	id, _ := createdSession(t, c.compound(2, backCreateSessionBody(clientID, seq, 0)))
	ts := newTestSession(id, clientID, 16)
	body := buildCompoundBody(nil, func(*COMPOUND4argsWriter) {})
	// The first SEQUENCE turns need on; let its pass finish.
	if res := c.compound(3, withSequence(body, ts, 0, 1)); res.Status() != NFS4_OK {
		t.Fatalf("SEQUENCE %s", Nfsstat4Name(res.Status()))
	}
	time.Sleep(100 * time.Millisecond)
	fs.reset()
	for i := uint32(2); i <= 201; i++ {
		if res := c.compound(2+i, withSequence(body, ts, 0, i)); res.Status() != NFS4_OK {
			t.Fatalf("SEQUENCE %d %s", i, Nfsstat4Name(res.Status()))
		}
	}
	if reads, _, writes, _ := fs.counts(); reads > 1 || writes != 0 {
		t.Fatalf("200 SEQUENCEs drove %d passes and %d writes", reads, writes)
	}
}

// refresherWorkerForTest is refresherForTest with server hooks, so the
// record has a worker that wakes are delivered to.
func refresherWorkerForTest(t *testing.T, fs *passCountVFS) (*ClientStore, uint64, *leaseRefresher) {
	t.Helper()
	cs, clientID, r, _ := refresherForTest(t, fs)
	fs.leaseName = cs.leaseName
	// Real time: no worker runs yet, so the field can still be set.
	cs.now = time.Now
	var need atomic.Bool
	need.Store(true)
	cs.leaseNeed = func(uint64) bool { return need.Load() }
	cs.leaseSessions = func(uint64) bool { return true }
	t.Cleanup(cs.stopLeaseWorkers)
	return cs, clientID, r
}

// A10: a failed pass's backoff is a lower bound for settlement and writes,
// whatever wakes the worker. Verify requests are coalesced, at least
// leaseVerifySpacing apart, and before the deadline only read.
func TestWakesDoNotShortenBackoff(t *testing.T) {
	fs := &passCountVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	cs, clientID, r := refresherWorkerForTest(t, fs)
	// The publication's rename fails, and so does its fence.
	fs.mu.Lock()
	fs.failRename, fs.failFence = true, true
	fs.mu.Unlock()
	fs.reset()
	cs.startLeaseBootstrap(clientID)
	waitFor(t, "the failed publication", func() bool { a, _, _, _ := recordImage(r); return a })
	start := time.Now()
	_, fences0, writes0, _ := fs.counts()
	for range 50 {
		cs.noteLeaseNeed(clientID)
		cs.requestLeaseVerify(clientID)
		time.Sleep(20 * time.Millisecond)
	}
	elapsed := time.Since(start)
	reads, fences, writes, _ := fs.counts()
	r.mu.Lock()
	deadline := r.notBefore
	r.mu.Unlock()
	if !deadline.After(start) {
		t.Fatalf("no backoff deadline after the failure (elapsed %s)", elapsed)
	}
	if fences != fences0 || writes != writes0 {
		t.Fatalf("wakes drove %d fences and %d writes inside the backoff", fences-fences0, writes-writes0)
	}
	if maxPasses := int(elapsed/leaseVerifySpacing) + 2; reads > maxPasses {
		t.Fatalf("%d verify passes in %s", reads, elapsed)
	}
}

// A10: a verify-only pass reads and does not write or remove anything: not
// even another process's long-dead lease slot, which only an authorized
// pass prunes.
func TestVerifyPassIsReadOnly(t *testing.T) {
	fs := &passCountVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	cs, clientID, r := refresherWorkerForTest(t, fs)
	r.safePass(true) // publish a fresh lease
	dead, err := json.Marshal(durableLease{ExpiresUnixNano: time.Now().Add(-10 * nfsLeaseTime).UnixNano()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.TernVFS.CreateFile(InodeID(clientID), leasePrefix+"otherprocess", bytes.NewReader(dead)); err != nil {
		t.Fatal(err)
	}
	cs.leaseNeed = func(uint64) bool { return false }
	fs.reset()
	for range 20 {
		cs.requestLeaseVerify(clientID)
		time.Sleep(50 * time.Millisecond)
	}
	reads, fences, writes, removes := fs.counts()
	if reads == 0 || reads > 3 {
		t.Fatalf("%d verify passes for 20 requests in 1s", reads)
	}
	if fences != 0 || writes != 0 || removes != 0 {
		t.Fatalf("verify passes wrote: fences %d writes %d removes %d", fences, writes, removes)
	}
	if _, err := fs.TernVFS.Lookup(InodeID(clientID), leasePrefix+"otherprocess"); err != nil {
		t.Fatal("a verify pass removed another process's slot")
	}
	// An authorized pass, with our lease live, prunes it.
	r.safePass(true)
	if _, err := fs.TernVFS.Lookup(InodeID(clientID), leasePrefix+"otherprocess"); err == nil {
		t.Fatal("an authorized pass kept another process's long-dead slot")
	}
}

func setMtimeStatus(t *testing.T, conn net.Conn, xid *uint32, fh []byte, sid [16]byte, sec uint64) uint32 {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		putfhOp(w, fh)
		sw := w.AppendArgarray_Setattr()
		setStateid(sw.Stateid(), sid)
		aw := sw.StartObjAttributes()
		value := binary.BigEndian.AppendUint32(nil, SET_TO_CLIENT_TIME4)
		value = binary.BigEndian.AppendUint64(value, sec)
		value = binary.BigEndian.AppendUint32(value, 0)
		sw.Resume(finishTestFattr(aw, attrMask{0, 1 << (FATTR4_TIME_MODIFY_SET - 32)}, value))
		w.Resume(sw.Finish())
	})
	*xid++
	return res.Status()
}

// F-SETATTR: a timestamp-only SETATTR with another client's stateid gets
// BAD_STATEID and changes no times; the zero stateid still sets times.
func TestForeignTimestampSetattrRejected(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	connA := dial(t, addr)
	defer connA.Close()
	xidA := uint32(1)
	wireA := setupClient(t, connA, &xidA)
	sid, fh := openCreateFile(t, connA, &xidA, wireA, "times")
	writeFileAt(t, connA, &xidA, fh, sid, 0, []byte("A"))
	stagingID := stagingIDForTest(t, srv.stagingStore, sid)
	stagedBefore := srv.stagingStore.Get(stagingID).Stat().Mtime
	fileID, _ := fhToInodeID(fh)
	publishedBefore, err := srv.fs.Stat(fileID)
	if err != nil {
		t.Fatal(err)
	}
	connB := dial(t, addr)
	defer connB.Close()
	if _, err := createTestSession(connB, 1, "times-b", [8]byte{2}); err != nil {
		t.Fatal(err)
	}
	xidB := uint32(100)
	if got := setMtimeStatus(t, connB, &xidB, fh, sid, 1_000_000); got != NFS4ERR_BAD_STATEID {
		t.Fatalf("foreign timestamp SETATTR %s, want BAD_STATEID", Nfsstat4Name(got))
	}
	if got := srv.stagingStore.Get(stagingID).Stat().Mtime; !got.Equal(stagedBefore) {
		t.Fatal("the foreign SETATTR changed A's staged times")
	}
	if got, _ := srv.fs.Stat(fileID); !got.Mtime.Equal(publishedBefore.Mtime) {
		t.Fatal("the foreign SETATTR changed the published times")
	}
	if got := setMtimeStatus(t, connB, &xidB, fh, [16]byte{}, 2_000_000); got != NFS4_OK {
		t.Fatalf("zero-stateid timestamp SETATTR %s", Nfsstat4Name(got))
	}
	if got, _ := srv.fs.Stat(fileID); got.Mtime.Unix() != 2_000_000 {
		t.Fatalf("zero-stateid SETATTR left mtime %v", got.Mtime)
	}
}

// F-LEASE1: once the refresher has retired an incarnation here, its cached
// CREATE_SESSION still replays the same (dead) session, but a new sequence
// gets STALE_CLIENTID and creates nothing.
func TestNoNewSessionForExpiredIncarnation(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	c := newCBTestClient(t, addr)
	clientID, seq := exchangeOn(t, c, 1, "lease1")
	body := backCreateSessionBody(clientID, seq, 0)
	id, _ := createdSession(t, c.compound(2, body))
	inc := incarnationForTest(t, srv, clientID)
	srv.clients.stopLeaseWorkers()
	setLeaseForTest(srv, inc, func(r *leaseRefresher) { r.state = leaseRecordStale })
	leasePassForTest(t, srv, inc)
	if srv.sessions.lookup(id) != nil {
		t.Fatal("terminal pass did not retire the session")
	}
	before := srv.sessions.accounting()
	if replayed, _ := createdSession(t, c.compound(3, body)); replayed != id {
		t.Fatal("the cached CREATE_SESSION did not replay")
	}
	if res := c.compound(4, backCreateSessionBody(clientID, seq+1, 0)); res.Status() != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("new CREATE_SESSION %s, want STALE_CLIENTID", Nfsstat4Name(res.Status()))
	}
	after := srv.sessions.accounting()
	if after.Sessions != before.Sessions || after.Records != before.Records || after.PendingSessions != before.PendingSessions {
		t.Fatalf("refused CREATE_SESSION changed %+v to %+v", before, after)
	}
	// The slot did not advance: the old sequence still replays.
	if replayed, _ := createdSession(t, c.compound(5, body)); replayed != id {
		t.Fatal("the slot moved")
	}
}

// F-LEASE1, race: expiry decided while a CREATE_SESSION is past its slot
// reservation and before its commit; the commit sees it.
func TestCreateSessionRacingExpiry(t *testing.T) {
	fs := &confirmedLookupHold{TernVFS: NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	defer closeSignal(fs.release)
	c := newCBTestClient(t, addr)
	clientID, seq := exchangeOn(t, c, 1, "lease1-race")
	createdSession(t, c.compound(2, backCreateSessionBody(clientID, seq, 0)))
	inc := incarnationForTest(t, srv, clientID)
	fs.mu.Lock()
	fs.armed = true // CREATE_SESSION's IsConfirmed check pauses
	fs.mu.Unlock()
	srv.clients.stopLeaseWorkers()
	done := make(chan uint32, 1)
	go func() { done <- c.compound(3, backCreateSessionBody(clientID, seq+1, 0)).Status() }()
	awaitSignal(t, fs.entered, "CREATE_SESSION's confirmation check")
	srv.sessions.mu.Lock()
	srv.sessions.groups[inc].lease = gateExpired
	srv.sessions.mu.Unlock()
	before := srv.sessions.accounting()
	closeSignal(fs.release)
	if got := awaitValue(t, done, "CREATE_SESSION"); got != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION racing expiry %s, want STALE_CLIENTID", Nfsstat4Name(got))
	}
	if after := srv.sessions.accounting(); after.Sessions != before.Sessions {
		t.Fatal("a session was inserted into the expired incarnation")
	}
}

// F-C11: a lone writer whose publication failed (no other reader) gets
// errors on its later operations, but they touch only its own state: the
// client's other writer keeps its open, marker and staging, and publishes.
func TestLoneFailedWriterLeavesSiblingsAlone(t *testing.T) {
	fs := &readerSourceVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	staging, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	inc := incarnationForTest(t, srv, clientID)
	aSid, aFH := openCreateFile(t, conn, &xid, clientID, "a")
	writeFileAt(t, conn, &xid, aFH, aSid, 0, []byte("A"))
	bSid, bFH := openCreateFile(t, conn, &xid, clientID, "b")
	writeFileAt(t, conn, &xid, bFH, bSid, 0, []byte("B's bytes"))
	bStaging := stagingIDForTest(t, staging, bSid)
	fs.mu.Lock()
	fs.failLink = "a"
	fs.mu.Unlock()
	// The publication's error is the CLOSE's; the failed writer's open is
	// then inactive, so its later operations get EXPIRED (README).
	if status := closeFileStatus(t, conn, &xid, aFH, aSid); status != NFS4ERR_IO {
		t.Fatalf("A's CLOSE with a LinkFile error %s, want NFS4ERR_IO", Nfsstat4Name(status))
	}
	for _, op := range []string{"READ", "WRITE", "CLOSE"} {
		if got := stateidOpStatus(t, conn, &xid, op, aFH, aSid); got != NFS4ERR_EXPIRED {
			t.Errorf("%s on the failed writer %s, want NFS4ERR_EXPIRED", op, Nfsstat4Name(got))
		}
	}
	var bID StateID
	copy(bID[:], bSid[4:])
	if !srv.opens.live(bID) {
		t.Fatal("the failed writer's operations expired the sibling open")
	}
	if m, ok := staging.GetMeta(bStaging); !ok || m.Retired {
		t.Fatal("the failed writer's operations retired the sibling staging")
	}
	if _, err := fs.TernVFS.Lookup(InodeID(inc), activeOpenName(bID)); err != nil {
		t.Fatalf("the sibling's marker: %v", err)
	}
	closeFile(t, conn, &xid, bFH, bSid)
	if data, err := readPublished(fs.TernVFS, "b"); err != nil || string(data) != "B's bytes" {
		t.Fatalf("sibling published %q %v", data, err)
	}
}

// Renew contract: with server hooks the store's Renew does not run a pass
// (the worker owns every pass and a server's terminal steps take identity
// ownership under leaseLocks); it reads the record and writes nothing.
func TestRenewWithServerHooksDoesNotPass(t *testing.T) {
	fs := &passCountVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.clients.stopLeaseWorkers)
	fs.leaseName = srv.clients.leaseName
	clientID, confirm, err := srv.clients.SetClientID([8]byte{1}, []byte("renew"), clientOwner{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(clientID, confirm, rpcPrincipal{}); err != nil {
		t.Fatal(err)
	}
	publishLeaseForTest(t, srv, clientID)
	srv.clients.stopLeaseWorkers()
	// Due for a refresh, but still live.
	later := time.Now().Add(nfsLeaseRenewAfter)
	srv.clients.setClock(func() time.Time { return later })
	fs.reset()
	if err := srv.clients.Renew(clientID); err != nil {
		t.Fatalf("Renew %v", err)
	}
	if reads, fences, writes, _ := fs.counts(); reads != 0 || fences != 0 || writes != 0 {
		t.Fatalf("Renew ran a pass: reads %d fences %d writes %d", reads, fences, writes)
	}
}

// F-LEASE1 follow-through: after the refresher has retired a fleet-expired
// incarnation here, the client recovers by registering again, without
// waiting for a sweep. Its cached CREATE_SESSION replays the dead session,
// a new sequence gets STALE_CLIENTID, and EXCHANGE_ID does not hand back the
// expired record as warm: it gives a new clientid, whose CREATE_SESSION and
// first SEQUENCE succeed. The old session stays BADSESSION.
func TestExpiredRegistrationRecovers(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	c := newCBTestClient(t, addr)
	oldWire, seq := exchangeOn(t, c, 1, "expired-registration")
	body := backCreateSessionBody(oldWire, seq, 0)
	oldID, _ := createdSession(t, c.compound(2, body))
	inc := incarnationForTest(t, srv, oldWire)
	// Another process marked the client expired; this process's refresher
	// sees the marker and retires the incarnation's sessions.
	if err := srv.clients.publishExpiredMarker(inc); err != nil {
		t.Fatal(err)
	}
	srv.clients.stopLeaseWorkers()
	leasePassForTest(t, srv, inc)
	if srv.sessions.lookup(oldID) != nil {
		t.Fatal("the terminal pass did not retire the session")
	}
	if replayed, _ := createdSession(t, c.compound(3, body)); replayed != oldID {
		t.Fatal("the cached CREATE_SESSION did not replay")
	}
	if res := c.compound(4, backCreateSessionBody(oldWire, seq+1, 0)); res.Status() != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("new CREATE_SESSION %s, want STALE_CLIENTID", Nfsstat4Name(res.Status()))
	}
	newWire, newSeq := exchangeOn(t, c, 5, "expired-registration")
	if newWire == oldWire {
		t.Fatal("EXCHANGE_ID returned the expired registration as warm")
	}
	// Workers were stopped for the terminal pass, so CREATE_SESSION waits
	// out its bootstrap bound and the test publishes with one pass, as the
	// worker would.
	newID, _ := createdSession(t, c.compound(6, backCreateSessionBody(newWire, newSeq, 0)))
	newInc := incarnationForTest(t, srv, newWire)
	if newInc == inc {
		t.Fatal("the fleet-expired incarnation was adopted again")
	}
	srv.clients.refreshMu.Lock()
	r, ok := srv.clients.recordLocked(InodeID(newInc), true)
	srv.clients.refreshMu.Unlock()
	if !ok {
		t.Fatal("no record")
	}
	r.safePass(true)
	ts := newTestSession(newID, newWire, 16)
	empty := buildCompoundBody(nil, func(*COMPOUND4argsWriter) {})
	if res := c.compound(7, withSequence(empty, ts, 0, 1)); res.Status() != NFS4_OK {
		t.Fatalf("first SEQUENCE on the new session %s", Nfsstat4Name(res.Status()))
	}
	old := newTestSession(oldID, oldWire, 16)
	if res := c.compound(8, withSequence(empty, old, 0, 1)); res.Status() != NFS4ERR_BADSESSION {
		t.Fatalf("old session %s, want BADSESSION", Nfsstat4Name(res.Status()))
	}
}
