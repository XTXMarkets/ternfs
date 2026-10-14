// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"sync"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// backSessionForTest gives a connectionSession a backchannel request and a
// sender, as CREATE_SESSION with CONN_BACK_CHAN would.
func backSessionForTest(t *testing.T, srv *Server) *session {
	t.Helper()
	sess := srv.sessions.lookup(connectionSession(t, srv).id)
	sess.backRequested = true
	sess.cb = newCBSender(srv, sess, 0x40000001, callbackSec{flavor: nfs4.AUTH_NONE})
	return sess
}

// testConnection is a transport whose writes vanish; its probes end when
// the test does.
func testConnection(t *testing.T) *rpcConnection {
	closed := make(chan struct{})
	t.Cleanup(func() { close(closed) })
	return &rpcConnection{send: func([]byte) bool { return true }, closed: closed}
}

// BIND_CONN_TO_SESSION over every existing association and requested
// direction. An acceptable existing association comes back unchanged, any
// other gets INVAL, invalid directions get INVAL, and nothing changes on a
// refusal.
func TestBindDirectionMatrix(t *testing.T) {
	type set struct{ fore, back bool }
	none, f, b, fb := set{}, set{fore: true}, set{back: true}, set{true, true}
	inval := set{} // marker for INVAL in the table
	want := map[uint32]map[set]set{
		nfs4.CDFC4_FORE:         {none: f, f: f, b: inval, fb: inval},
		nfs4.CDFC4_BACK:         {none: b, f: inval, b: b, fb: inval},
		nfs4.CDFC4_FORE_OR_BOTH: {none: fb, f: f, b: inval, fb: fb},
		nfs4.CDFC4_BACK_OR_BOTH: {none: fb, f: inval, b: b, fb: fb},
	}
	dirName := map[set]uint32{f: nfs4.CDFS4_FORE, b: nfs4.CDFS4_BACK, fb: nfs4.CDFS4_BOTH}
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	for _, dir := range []uint32{0, nfs4.CDFC4_FORE, nfs4.CDFC4_BACK, nfs4.CDFC4_FORE_OR_BOTH, 4, 5, 6, nfs4.CDFC4_BACK_OR_BOTH} {
		for _, existing := range []set{none, f, b, fb} {
			sess := backSessionForTest(t, srv)
			conn := testConnection(t)
			if existing.fore && !conn.bind(srv, sess.id) || existing.back && !conn.bindBack(srv, sess.id) {
				t.Fatal("setting up the existing association")
			}
			_, res := admissionRequest(t, srv, admissionBind(sess.id, dir, false), 1, conn)
			expect, valid := want[dir][existing]
			after := set{conn.carriesFore(sess.id), conn.carriesBack(sess.id)}
			if !valid || expect == inval {
				admissionStatus(t, res, nfs4.NFS4ERR_INVAL, nfs4.OP_BIND_CONN_TO_SESSION)
				if after != existing {
					t.Errorf("dir %d existing %+v: refusal changed the association to %+v", dir, existing, after)
				}
				continue
			}
			admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_BIND_CONN_TO_SESSION)
			it := res.Resarray()
			it.Next()
			got := it.Resarray().Value().AsBINDCONNTOSESSION4resEntry().Value().AsBINDCONNTOSESSION4resok().BctsrDir()
			if got != dirName[expect] || after != expect {
				t.Errorf("dir %d existing %+v: returned %d with %+v, want %+v", dir, existing, got, after, expect)
			}
		}
	}

	// A session that asked for no backchannel cannot gain one.
	plain := srv.sessions.lookup(connectionSession(t, srv).id)
	for dir, status := range map[uint32]uint32{nfs4.CDFC4_BACK: nfs4.NFS4ERR_INVAL, nfs4.CDFC4_BACK_OR_BOTH: nfs4.NFS4ERR_INVAL, nfs4.CDFC4_FORE_OR_BOTH: nfs4.NFS4_OK} {
		conn := testConnection(t)
		_, res := admissionRequest(t, srv, admissionBind(plain.id, dir, false), 1, conn)
		admissionStatus(t, res, status, nfs4.OP_BIND_CONN_TO_SESSION)
		if conn.carriesBack(plain.id) {
			t.Errorf("dir %d gave a back association", dir)
		}
	}

	// Full quotas: RESOURCE, and neither direction is taken.
	for _, full := range []string{"fore", "back"} {
		sess := backSessionForTest(t, srv)
		conn := testConnection(t)
		for range maxConnectionSessions {
			other := connectionSession(t, srv).id
			if full == "fore" && !conn.bind(srv, other) || full == "back" && !conn.bindBack(srv, other) {
				t.Fatal("filling the quota")
			}
		}
		_, res := admissionRequest(t, srv, admissionBind(sess.id, nfs4.CDFC4_FORE_OR_BOTH, false), 1, conn)
		admissionStatus(t, res, nfs4.NFS4ERR_RESOURCE, nfs4.OP_BIND_CONN_TO_SESSION)
		if conn.carriesFore(sess.id) || conn.carriesBack(sess.id) {
			t.Errorf("%s quota full: a partial association was made", full)
		}
	}
}

// A cached CREATE_SESSION replayed on another connection binds the
// backchannel there too, which then gets its own CB_NULL.
func TestCreateSessionReplayBindsBackchannel(t *testing.T) {
	_, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	first := newCBTestClient(t, addr)
	clientID, seq := exchangeOn(t, first, 1, "replay-back")
	body := backCreateSessionBody(clientID, seq, nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN, offerNone)
	id, flags := createdSession(t, first.compound(2, body))
	if flags&nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN == 0 {
		t.Fatal("no backchannel granted")
	}
	first.answerNull(t)
	second := newCBTestClient(t, addr)
	replayed, flags := createdSession(t, second.compound(3, body))
	if replayed != id || flags&nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN == 0 {
		t.Fatal("replay result differs")
	}
	second.answerNull(t)
}

// A held confirmation's retry on a second connection binds the granted
// backchannel to that connection.
func TestHeldCreateSessionRetryBindsBackchannel(t *testing.T) {
	f := newConfirmServer(t)
	body := backCreateSessionBody(f.wireN, 1, nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN, offerNone)
	// D applies but reports failure: held, and settled by the retry.
	f.arm(clientstore.WriteD, clientstore.FaultAppliedFail, nil)
	if got := f.rawStatus(t, body); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("unresolved CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	f.disarm()
	f.fs.Set(clientstore.WriteD, clientstore.FaultNone, nil)
	retry := newCBTestClient(t, addrOfTestServer(t, f.srv))
	_, flags := createdSession(t, retry.compound(4000, body))
	if flags&nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN == 0 {
		t.Fatal("no backchannel granted")
	}
	retry.answerNull(t)
}

// With the connection's back quota full, CREATE_SESSION asking for a
// backchannel gets RESOURCE before creating anything.
func TestCreateSessionBackQuotaBeforeEffects(t *testing.T) {
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	conn := testConnection(t)
	for range maxConnectionSessions {
		if !conn.bindBack(srv, connectionSession(t, srv).id) {
			t.Fatal("filling the back quota")
		}
	}
	_, res := admissionRequest(t, srv, exchangeIDBody("back-quota", [8]byte{8}), 1, conn)
	it := res.Resarray()
	it.Next()
	ex := it.Resarray().Value().AsEXCHANGEID4resEntry().Value().AsEXCHANGEID4resok()
	before := srv.sessions.accounting()
	_, res = admissionRequest(t, srv, backCreateSessionBody(ex.EirClientid(), ex.EirSequenceid(),
		nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN, offerNone), 2, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_RESOURCE, nfs4.OP_CREATE_SESSION)
	after := srv.sessions.accounting()
	if after.Sessions != before.Sessions || after.Records != before.Records ||
		after.PendingSessions != before.PendingSessions || after.GroupCredits != before.GroupCredits {
		t.Fatalf("refused CREATE_SESSION changed %+v to %+v", before, after)
	}
	// The slot was not consumed: without the backchannel it succeeds.
	_, res = admissionRequest(t, srv, backCreateSessionBody(ex.EirClientid(), ex.EirSequenceid(), 0), 3, conn)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_CREATE_SESSION)
}

// Once the refresher has retired an incarnation here, its cached
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
	srv.clients.StopLeaseWorkers()
	setLeaseForTest(srv, inc, func(r *clientstore.LeaseRefresher) { r.State = clientstore.LeaseRecordStale })
	leasePassForTest(t, srv, inc)
	if srv.sessions.lookup(id) != nil {
		t.Fatal("terminal pass did not retire the session")
	}
	before := srv.sessions.accounting()
	if replayed, _ := createdSession(t, c.compound(3, body)); replayed != id {
		t.Fatal("the cached CREATE_SESSION did not replay")
	}
	if res := c.compound(4, backCreateSessionBody(clientID, seq+1, 0)); res.Status() != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("new CREATE_SESSION %s, want STALE_CLIENTID", nfs4.Nfsstat4Name(res.Status()))
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

// Expiry decided while a CREATE_SESSION is past its slot
// reservation and before its commit; the commit sees it.
func TestCreateSessionRacingExpiry(t *testing.T) {
	fs := &confirmedLookupHold{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	defer testutil.CloseSignal(fs.release)
	c := newCBTestClient(t, addr)
	clientID, seq := exchangeOn(t, c, 1, "lease1-race")
	createdSession(t, c.compound(2, backCreateSessionBody(clientID, seq, 0)))
	inc := incarnationForTest(t, srv, clientID)
	fs.mu.Lock()
	fs.armed = true // CREATE_SESSION's IsConfirmed check pauses
	fs.mu.Unlock()
	srv.clients.StopLeaseWorkers()
	done := make(chan uint32, 1)
	go func() { done <- c.compound(3, backCreateSessionBody(clientID, seq+1, 0)).Status() }()
	testutil.AwaitSignal(t, fs.entered, "CREATE_SESSION's confirmation check")
	srv.sessions.mu.Lock()
	srv.sessions.groups[inc].lease = gateExpired
	srv.sessions.mu.Unlock()
	before := srv.sessions.accounting()
	testutil.CloseSignal(fs.release)
	if got := testutil.AwaitValue(t, done, "CREATE_SESSION"); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION racing expiry %s, want STALE_CLIENTID", nfs4.Nfsstat4Name(got))
	}
	if after := srv.sessions.accounting(); after.Sessions != before.Sessions {
		t.Fatal("a session was inserted into the expired incarnation")
	}
}

// After the refresher has retired a fleet-expired
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
	if err := srv.clients.PublishExpiredMarker(inc); err != nil {
		t.Fatal(err)
	}
	srv.clients.StopLeaseWorkers()
	leasePassForTest(t, srv, inc)
	if srv.sessions.lookup(oldID) != nil {
		t.Fatal("the terminal pass did not retire the session")
	}
	if replayed, _ := createdSession(t, c.compound(3, body)); replayed != oldID {
		t.Fatal("the cached CREATE_SESSION did not replay")
	}
	if res := c.compound(4, backCreateSessionBody(oldWire, seq+1, 0)); res.Status() != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("new CREATE_SESSION %s, want STALE_CLIENTID", nfs4.Nfsstat4Name(res.Status()))
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
	srv.clients.RefreshMu.Lock()
	r, ok := srv.clients.RecordLocked(vfs.InodeID(newInc), true)
	srv.clients.RefreshMu.Unlock()
	if !ok {
		t.Fatal("no record")
	}
	r.SafePass(true)
	ts := newTestSession(newID, newWire, 16)
	empty := buildCompoundBody(nil, func(*nfs4.COMPOUND4argsWriter) {})
	if res := c.compound(7, withSequence(empty, ts, 0, 1)); res.Status() != nfs4.NFS4_OK {
		t.Fatalf("first SEQUENCE on the new session %s", nfs4.Nfsstat4Name(res.Status()))
	}
	old := newTestSession(oldID, oldWire, 16)
	if res := c.compound(8, withSequence(empty, old, 0, 1)); res.Status() != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("old session %s, want BADSESSION", nfs4.Nfsstat4Name(res.Status()))
	}
}

// bindStatus runs one BIND_CONN_TO_SESSION on conn and returns its status
// and, for OK, the direction granted. It reports a parse failure as an
// error status rather than failing, so goroutines may call it.
func bindStatus(srv *Server, conn *rpcConnection, id SessionID, dir uint32, xid uint32) (status, granted uint32) {
	req, err := parseRPCCall(rpcCallForTest(xid, procCompound, admissionBind(id, dir, false)))
	if err != nil {
		return nfs4.NFS4ERR_SERVERFAULT, 0
	}
	req.connection = conn
	reply := bytes.Clone(srv.safeHandleCompound(req, "bind-race"))
	req.releaseReply()
	res, err := parseCompoundReply(reply)
	if err != nil {
		return nfs4.NFS4ERR_SERVERFAULT, 0
	}
	if res.Status() != nfs4.NFS4_OK {
		return res.Status(), 0
	}
	it := res.Resarray()
	it.Next()
	return nfs4.NFS4_OK, it.Resarray().Value().AsBINDCONNTOSESSION4resEntry().Value().AsBINDCONNTOSESSION4resok().BctsrDir()
}

// Two BINDs for one unassociated session on one connection, sent
// at once, are decided as if one ran after the other. The second sees the
// first's association, which it may not change: it gets INVAL, and the
// connection never ends with both directions from two requests that each
// asked for one.
func TestConcurrentBindsSerialize(t *testing.T) {
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	pairs := [][2]uint32{
		{nfs4.CDFC4_FORE, nfs4.CDFC4_BACK},
		{nfs4.CDFC4_FORE, nfs4.CDFC4_BACK_OR_BOTH},
		{nfs4.CDFC4_FORE_OR_BOTH, nfs4.CDFC4_BACK},
	}
	for _, pair := range pairs {
		for round := range 300 {
			sess := backSessionForTest(t, srv)
			conn := testConnection(t)
			var start sync.WaitGroup
			start.Add(1)
			var done sync.WaitGroup
			status := make([]uint32, 2)
			granted := make([]uint32, 2)
			for i, dir := range pair {
				done.Add(1)
				go func() {
					defer done.Done()
					start.Wait()
					status[i], granted[i] = bindStatus(srv, conn, sess.id, dir, uint32(i+1))
				}()
			}
			start.Done()
			done.Wait()
			fore, back := conn.carriesFore(sess.id), conn.carriesBack(sess.id)
			oks := 0
			for i := range pair {
				switch status[i] {
				case nfs4.NFS4_OK:
					oks++
					want := granted[i] == nfs4.CDFS4_BOTH && fore && back ||
						granted[i] == nfs4.CDFS4_FORE && fore && !back ||
						granted[i] == nfs4.CDFS4_BACK && !fore && back
					if !want {
						t.Fatalf("dirs %v round %d: request %d granted %d but the connection has fore=%v back=%v",
							pair, round, i, granted[i], fore, back)
					}
				case nfs4.NFS4ERR_INVAL:
				default:
					t.Fatalf("dirs %v round %d: request %d got %s", pair, round, i, nfs4.Nfsstat4Name(status[i]))
				}
			}
			if oks != 1 {
				t.Fatalf("dirs %v round %d: %d BINDs succeeded (%s, %s), fore=%v back=%v; want exactly one",
					pair, round, oks, nfs4.Nfsstat4Name(status[0]), nfs4.Nfsstat4Name(status[1]), fore, back)
			}
		}
	}
}

// leaseRemoveHold blocks the first RemoveEdge of the lease name once
// armed: the terminal pass's cleanup of our lease.
type leaseRemoveHold struct {
	vfs.TernVFS
	mu        sync.Mutex
	leaseName string
	armed     bool
	entered   chan struct{}
	release   chan struct{}
}

func (fs *leaseRemoveHold) RemoveEdge(dir vfs.InodeID, name string, edge vfs.Edge) error {
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

// Once the refresher knows an incarnation is terminal, a new
// CREATE_SESSION cannot commit, even while the terminal cleanup's store
// I/O is blocked. The cached CREATE_SESSION still replays, and the slot
// does not move.
func TestTerminalAdmissionBeforeCleanup(t *testing.T) {
	fs := &leaseRemoveHold{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fs.leaseName = srv.clients.LeaseName
	addr, stop := serveTestServer(t, srv)
	defer stop()
	defer testutil.CloseSignal(fs.release)
	c := newCBTestClient(t, addr)
	clientID, seq := exchangeOn(t, c, 1, "terminal-cleanup")
	body := backCreateSessionBody(clientID, seq, 0)
	id, _ := createdSession(t, c.compound(2, body))
	inc := incarnationForTest(t, srv, clientID)
	srv.clients.StopLeaseWorkers()
	if !fs.present(vfs.InodeID(inc)) {
		t.Fatal("no published lease to clean up")
	}
	setLeaseForTest(srv, inc, func(r *clientstore.LeaseRefresher) { r.State = clientstore.LeaseRecordStale })
	fs.mu.Lock()
	fs.armed = true
	fs.mu.Unlock()
	cs := srv.clients
	cs.RefreshMu.Lock()
	r, _ := cs.RecordLocked(vfs.InodeID(inc), true)
	cs.RefreshMu.Unlock()
	passed := make(chan struct{})
	go func() {
		defer close(passed)
		r.SafePass(false)
	}()
	testutil.AwaitSignal(t, fs.entered, "the terminal cleanup's RemoveEdge")

	before := srv.sessions.accounting()
	if res := c.compound(3, backCreateSessionBody(clientID, seq+1, 0)); res.Status() != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION during terminal cleanup %s, want STALE_CLIENTID", nfs4.Nfsstat4Name(res.Status()))
	}
	if after := srv.sessions.accounting(); after.Sessions != before.Sessions ||
		after.Records != before.Records || after.PendingSessions != before.PendingSessions {
		t.Fatalf("refused CREATE_SESSION changed %+v to %+v", before, after)
	}
	if replayed, _ := createdSession(t, c.compound(4, body)); replayed != id {
		t.Fatal("the cached CREATE_SESSION did not replay during cleanup")
	}

	testutil.CloseSignal(fs.release)
	testutil.AwaitSignal(t, passed, "the terminal pass")
	if fs.present(vfs.InodeID(inc)) {
		t.Fatal("the terminal pass left our lease")
	}
	if srv.sessions.lookup(id) != nil {
		t.Fatal("the terminal pass did not retire the session")
	}
	// After cleanup: still a replay of the dead session, and still no new one.
	if replayed, _ := createdSession(t, c.compound(5, body)); replayed != id {
		t.Fatal("the cached CREATE_SESSION did not replay after cleanup")
	}
	if res := c.compound(6, backCreateSessionBody(clientID, seq+1, 0)); res.Status() != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION after terminal cleanup %s, want STALE_CLIENTID", nfs4.Nfsstat4Name(res.Status()))
	}
}

func (fs *leaseRemoveHold) present(dir vfs.InodeID) bool {
	_, err := fs.TernVFS.Lookup(dir, fs.leaseName)
	return err == nil
}
