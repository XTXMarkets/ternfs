// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"os"
	"sync"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
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

// EXCHANGE_ID's server owner and scope come from the staging directory's
// server-id: a restart on the same directory, or on a copy of it, is the
// same server, and an empty directory is a new one.
func TestExchangeIDServerOwnerFollowsStaging(t *testing.T) {
	owner := func(dir string) []byte {
		t.Helper()
		store, err := staging.NewLocalStore(dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer staging.CloseFiles(t, store)
		srv, err := NewServer(vfs.NewLocalTernVFS(t.TempDir()), store, nil)
		if err != nil {
			t.Fatal(err)
		}
		useTestWireEpoch(srv)
		_, res := admissionRequest(t, srv, exchangeIDBody("owner", [8]byte{1}), 1, testConnection(t))
		admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_EXCHANGE_ID)
		it := res.Resarray()
		it.Next()
		ex := it.Resarray().Value().AsEXCHANGEID4resEntry().Value().AsEXCHANGEID4resok()
		major := bytes.Clone(ex.EirServerOwner().SoMajorId())
		if want := "ternfs-nfsd/" + store.ServerID().String(); string(major) != want {
			t.Fatalf("server owner %q, want %q", major, want)
		}
		if scope := ex.EirServerScope(); !bytes.Equal(scope, major) {
			t.Fatalf("server scope %q, want the owner %q", scope, major)
		}
		return major
	}
	dir := t.TempDir()
	first := owner(dir)
	if again := owner(dir); !bytes.Equal(again, first) {
		t.Fatalf("restart changed the server owner from %q to %q", first, again)
	}
	moved := t.TempDir()
	if err := os.CopyFS(moved, os.DirFS(dir)); err != nil {
		t.Fatal(err)
	}
	if got := owner(moved); !bytes.Equal(got, first) {
		t.Fatalf("moved staging has server owner %q, want %q", got, first)
	}
	if wiped := owner(t.TempDir()); bytes.Equal(wiped, first) {
		t.Fatalf("an empty staging directory kept the server owner %q", wiped)
	}
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

// Expiry decided while a CREATE_SESSION is past its slot reservation and
// before its commit: the commit sees the gate and inserts nothing.
func TestCreateSessionRacingExpiry(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	c := newCBTestClient(t, addr)
	clientID, seq := exchangeOn(t, c, 1, "lease1-race")
	createdSession(t, c.compound(2, backCreateSessionBody(clientID, seq, 0)))
	inc := incarnationForTest(t, srv, clientID)
	srv.sessions.mu.Lock()
	srv.sessions.groups[inc].lease = gateExpired
	srv.sessions.mu.Unlock()
	before := srv.sessions.accounting()
	if got := c.compound(3, backCreateSessionBody(clientID, seq+1, 0)).Status(); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION racing expiry %s, want STALE_CLIENTID", nfs4.Nfsstat4Name(got))
	}
	if after := srv.sessions.accounting(); after.Sessions != before.Sessions {
		t.Fatal("a session was inserted into the expired incarnation")
	}
}

// An expired client recovers by registering again. Its wire record went
// with its lease, so CREATE_SESSION on it gets STALE_CLIENTID, and
// EXCHANGE_ID does not hand back the expired record as warm: it gives a
// new clientid and incarnation, whose CREATE_SESSION and first SEQUENCE
// succeed. The old session stays BADSESSION.
func TestExpiredRegistrationRecovers(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	c := newCBTestClient(t, addr)
	oldWire, seq := exchangeOn(t, c, 1, "expired-registration")
	oldID, _ := createdSession(t, c.compound(2, backCreateSessionBody(oldWire, seq, 0)))
	inc := incarnationForTest(t, srv, oldWire)
	awaitRequestHoldsReleased(t, srv)
	afterLeaseForTest(srv)
	srv.runLeaseSweep()
	if srv.sessions.lookup(oldID) != nil {
		t.Fatal("expiry did not retire the session")
	}
	if res := c.compound(4, backCreateSessionBody(oldWire, seq+1, 0)); res.Status() != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("new CREATE_SESSION %s, want STALE_CLIENTID", nfs4.Nfsstat4Name(res.Status()))
	}
	newWire, newSeq := exchangeOn(t, c, 5, "expired-registration")
	if newWire == oldWire {
		t.Fatal("EXCHANGE_ID returned the expired registration as warm")
	}
	newID, _ := createdSession(t, c.compound(6, backCreateSessionBody(newWire, newSeq, 0)))
	if newInc := incarnationForTest(t, srv, newWire); newInc == inc {
		t.Fatal("the expired incarnation was adopted again")
	}
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
