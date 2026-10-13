// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

// moveSessionForTest gives an inserted session another incarnation and ID.
func moveSessionForTest(ss *sessionStore, sess *session, clientID uint64, id SessionID) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.sessions, sess.id)
	if g := ss.groups[sess.clientID]; g != nil {
		g.members--
		ss.releaseGroupLocked(g)
	}
	g, ok := ss.groupLocked(clientID)
	if !ok {
		panic("no group credit")
	}
	sess.clientID, sess.id = clientID, id
	ss.sessions[id] = sess
	g.members++
}

func waitForAccounting(t *testing.T, ss *sessionStore, what string, cond func(replyAccounting) bool) {
	t.Helper()
	for deadline := time.Now().Add(testChannelTimeout); ; {
		a := ss.accounting()
		if cond(a) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: %+v", what, a)
		}
		time.Sleep(time.Millisecond)
	}
}

// A replay holds its incarnation like any admitted request, until its reply
// is written, even after its session is destroyed.
func TestGroupHoldLastsThroughReplayTransport(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	body := buildCompoundBody(nil, func(w *COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	wire := admissionSequence(body, ts, 0, 1, true)
	_, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, NFS4_OK, OP_SEQUENCE, OP_PUTROOTFH)
	if a := srv.sessions.accounting(); a.ActiveRequests != 0 || a.ProvisionalRequests != 0 || a.Groups != 1 {
		t.Fatalf("completed request kept a hold %+v", a)
	}

	server, client := net.Pipe()
	gate := &reviewBlockedWriteConn{Conn: server, entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan struct{})
	go func() { defer close(done); srv.handleConn(gate) }()
	defer func() {
		closeSignal(gate.release)
		client.Close()
		awaitSignal(t, done, "transport cleanup")
	}()
	writeCallForTest(t, client, rpcCallForTest(2, procCompound, wire))
	awaitSignal(t, gate.entered, "stalled replay")
	if a := srv.sessions.accounting(); a.ActiveRequests != 1 {
		t.Fatalf("replay has no active hold %+v", a)
	}
	if srv.sessions.closeForReplacement(ts.clientID) {
		t.Fatal("replacement overtook a replay")
	}
	srv.sessions.destroySession(ts.id)
	if a := srv.sessions.accounting(); a.Sessions != 0 || a.Groups != 1 || a.ActiveRequests != 1 {
		t.Fatalf("detached replay lost its group %+v", a)
	}
	closeSignal(gate.release)
	client.SetReadDeadline(time.Now().Add(testChannelTimeout))
	if _, err := readFrame(client); err != nil {
		t.Fatal(err)
	}
	// The group then keeps only its gate, for the quiet period.
	waitForAccounting(t, srv.sessions, "hold after the reply", func(a replyAccounting) bool {
		return a.ActiveRequests == 0 && a.Groups == 1
	})
	endQuietForTest(srv)
	if a := srv.sessions.accounting(); a.Groups != 0 {
		t.Fatalf("group outlived its quiet period %+v", a)
	}
}

// A reply that is never written still releases its hold: the connection's
// failure ends the request.
func TestGroupHoldReleasedOnTransportFailure(t *testing.T) {
	for _, mode := range []string{"write-timeout", "write-panic"} {
		t.Run(mode, func(t *testing.T) {
			srv, _ := newConnectionTestServer(t, 2)
			ts := connectionSession(t, srv)
			srv.idleTimeout = 0
			srv.writeTimeout = 25 * time.Millisecond
			server, client := net.Pipe()
			defer client.Close()
			defer server.Close()
			conn := &replyConn{Conn: server, panicWrite: mode == "write-panic"}
			done := make(chan struct{})
			go func() { srv.handleConn(conn); close(done) }()
			body := buildCompoundBody(nil, func(w *COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
			writeCallForTest(t, client, rpcCallForTest(1, procCompound, admissionSequence(body, ts, 0, 1, true)))
			// The client never reads, so the reply cannot be delivered.
			awaitSignal(t, done, "failed connection")
			if conn.writes.Load() == 0 {
				t.Fatal("reply was never attempted")
			}
			if a := srv.sessions.accounting(); a.ActiveRequests != 0 || a.ProvisionalRequests != 0 {
				t.Fatalf("failed transport kept its hold %+v", a)
			}
			if !srv.sessions.closeForReplacement(ts.clientID) {
				t.Fatal("released hold still blocks replacement")
			}
			srv.sessions.reopen(ts.clientID)
		})
	}
}

// A compound that panics after admission retires its session, and its hold
// keeps the detached session's group until the reply is done.
func TestGroupHoldReleasedAfterCompoundPanic(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "victim"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	srv := admissionWritableServer(t, &admissionPanicVFS{TernVFS: NewLocalTernVFS(root)})
	ts := connectionSession(t, srv)
	req, err := parseRPCCall(rpcCallForTest(1, procCompound, admissionSequence(
		buildCompoundBody(nil, func(w *COMPOUND4argsWriter) {
			w.AppendArgarray_Putrootfh()
			admissionRemove(w, "victim")
		}), ts, 0, 1, true)))
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseCompoundReply(srv.safeHandleCompound(req, "panic"))
	if err != nil {
		t.Fatal(err)
	}
	admissionStatus(t, res, NFS4ERR_SERVERFAULT, OP_SEQUENCE, OP_PUTROOTFH, OP_REMOVE)
	if a := srv.sessions.accounting(); a.Sessions != 0 || a.Groups != 1 || a.ActiveRequests != 1 {
		t.Fatalf("panicked request released its hold before the reply %+v", a)
	}
	req.releaseReply()
	// Its release renews the logical lease: the group's gate stays for
	// the quiet period, then goes.
	if a := srv.sessions.accounting(); a.ActiveRequests != 0 || a.Groups != 1 {
		t.Fatalf("panicked request kept its hold %+v", a)
	}
	endQuietForTest(srv)
	if a := srv.sessions.accounting(); a.Groups != 0 {
		t.Fatalf("group outlived its quiet period %+v", a)
	}
}

// Replacement waits for running requests, and new requests on a closing
// incarnation wait for the replacement, under one lock.
func TestGroupClosingAndReplacedAdmission(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	ss := srv.sessions
	inc := ts.clientID
	body := buildCompoundBody(nil, nil)

	seq := srv.startSequence(ReadCOMPOUND4argsForTest(t, admissionSequence(body, ts, 0, 1, true)))
	if seq.status != NFS4_OK {
		t.Fatalf("admission %s", Nfsstat4Name(seq.status))
	}
	if ss.closeForReplacement(inc) {
		t.Fatal("replacement overtook an admitted request")
	}
	seq.finish(nil)
	if ss.closeForReplacement(inc) {
		t.Fatal("replacement overtook a request whose reply is unsent")
	}
	seq.hold.release()
	endCreate, status := ss.beginCreate(inc)
	if status != NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", Nfsstat4Name(status))
	}
	if ss.closeForReplacement(inc) {
		t.Fatal("replacement overtook a CREATE_SESSION")
	}
	endCreate()
	if !ss.closeForReplacement(inc) {
		t.Fatal("idle incarnation cannot be replaced")
	}

	// Closing: SEQUENCE, BIND and CREATE_SESSION wait; nothing changes.
	slot := &ss.lookup(ts.id).slots[1]
	before := admissionImage(slot)
	next := admissionSequence(body, ts, 1, 1, true)
	if seq := srv.startSequence(ReadCOMPOUND4argsForTest(t, next)); seq.status != NFS4ERR_DELAY {
		t.Fatalf("SEQUENCE while closing %s", Nfsstat4Name(seq.status))
	}
	if after := admissionImage(slot); !reflect.DeepEqual(after, before) {
		t.Fatal("waiting admission changed its slot")
	}
	if a := ss.accounting(); a.ProvisionalRequests != 0 || a.ActiveRequests != 0 {
		t.Fatalf("waiting admission kept a hold %+v", a)
	}
	if status := ss.admissionStatus(inc); status != NFS4ERR_DELAY {
		t.Fatalf("BIND while closing %s", Nfsstat4Name(status))
	}
	if _, status := ss.beginCreate(inc); status != NFS4ERR_DELAY {
		t.Fatalf("CREATE_SESSION while closing %s", Nfsstat4Name(status))
	}
	ss.reopen(inc)
	seq = srv.startSequence(ReadCOMPOUND4argsForTest(t, next))
	if seq.status != NFS4_OK {
		t.Fatalf("SEQUENCE after reopen %s", Nfsstat4Name(seq.status))
	}
	seq.finish(nil)
	seq.hold.release()

	// Replaced: the incarnation's sessions are finished.
	if !ss.closeForReplacement(inc) {
		t.Fatal("reopened incarnation cannot be replaced")
	}
	ss.replaced(inc)
	if seq := srv.startSequence(ReadCOMPOUND4argsForTest(t, admissionSequence(body, ts, 2, 1, true))); seq.status != NFS4ERR_BADSESSION {
		t.Fatalf("SEQUENCE after replacement %s", Nfsstat4Name(seq.status))
	}
	if _, status := ss.beginCreate(inc); status != NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION after replacement %s", Nfsstat4Name(status))
	}
}

type confirmedLookupGateVFS struct {
	TernVFS
	armed            atomic.Bool
	entered, release chan struct{}
}

func (fs *confirmedLookupGateVFS) Lookup(dir InodeID, name string) (InodeID, error) {
	if name == confirmedName && fs.armed.CompareAndSwap(true, false) {
		close(fs.entered)
		<-fs.release
	}
	return fs.TernVFS.Lookup(dir, name)
}

// A standalone CREATE_SESSION blocked in the store keeps replacement out
// until it has inserted its session.
func TestGroupStandaloneCreateSessionExcludesReplacement(t *testing.T) {
	fs := &confirmedLookupGateVFS{TernVFS: NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	srv := admissionWritableServer(t, fs)
	addr, stop := serveTestServer(t, srv)
	defer stop()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	client := setupClient(t, conn, &xid)
	ts := testSessionForClient(conn, client)

	fs.armed.Store(true)
	defer closeSignal(fs.release)
	result := make(chan COMPOUND4res, 1)
	go func() {
		_, res := admissionRequest(t, srv, createSessionBody(client, ts.nextCSSeq), 100, nil)
		result <- res
	}()
	incarnation := incarnationForTest(t, srv, client)
	awaitSignal(t, fs.entered, "CREATE_SESSION store check")
	if srv.sessions.closeForReplacement(incarnation) {
		t.Fatal("replacement overtook an in-progress CREATE_SESSION")
	}
	closeSignal(fs.release)
	admissionStatus(t, awaitValue(t, result, "CREATE_SESSION"), NFS4_OK, OP_CREATE_SESSION)
	if !srv.sessions.closeForReplacement(incarnation) {
		t.Fatal("finished CREATE_SESSION still blocks replacement")
	}
	srv.sessions.reopen(incarnation)
}

// registerForTest runs EXCHANGE_ID and CREATE_SESSION number seq.
func registerForTest(t *testing.T, srv *Server, owner string, seq uint32) (uint64, *testSession) {
	t.Helper()
	ex := identityExchange(t, srv, 1, owner, [8]byte{1}, false)
	if ex.status != NFS4_OK {
		t.Fatalf("EXCHANGE_ID %s", Nfsstat4Name(ex.status))
	}
	return ex.clientID, createForTest(t, srv, ex.clientID, seq)
}

func createForTest(t *testing.T, srv *Server, clientID uint64, seq uint32) *testSession {
	t.Helper()
	res := identityCall(t, srv, 1, createSessionBody(clientID, seq))
	if res.Status() != NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", Nfsstat4Name(res.Status()))
	}
	it := res.Resarray()
	it.Next()
	ok := it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok()
	return newTestSession(readSessionID(ok.CsrSessionid()), clientID, ok.CsrForeChanAttrs().CaMaxrequests())
}

// heldReply sends one call on its own connection whose reply write stalls
// until the returned function is called; that function then reads the reply.
func heldReply(t *testing.T, srv *Server, call []byte) func() {
	t.Helper()
	server, client := net.Pipe()
	gate := &reviewBlockedWriteConn{Conn: server, entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan struct{})
	go func() { defer close(done); srv.handleConn(gate) }()
	t.Cleanup(func() {
		closeSignal(gate.release)
		client.Close()
		awaitSignal(t, done, "held transport cleanup")
	})
	writeCallForTest(t, client, call)
	awaitSignal(t, gate.entered, "held reply")
	return func() {
		closeSignal(gate.release)
		client.SetReadDeadline(time.Now().Add(testChannelTimeout))
		if _, err := readFrame(client); err != nil {
			t.Fatal(err)
		}
	}
}

// A group keeps its credit until its last request's reply is done, after its
// sessions, session records and wire record are gone; nothing else can use
// that credit meanwhile.
func TestGroupCreditHeldUntilFinalHold(t *testing.T) {
	srv := identityServer(t, NewLocalTernVFS(t.TempDir()))
	ss := srv.sessions
	wire, ts := registerForTest(t, srv, "held", 1)
	incarnation := incarnationForTest(t, srv, wire)
	second := createForTest(t, srv, wire, 2)
	_, other := registerForTest(t, srv, "other", 1)

	body := buildCompoundBody(nil, func(w *COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	replayed := admissionSequence(body, ts, 1, 1, true)
	_, res := admissionRequest(t, srv, replayed, 1, nil)
	admissionStatus(t, res, NFS4_OK, OP_SEQUENCE, OP_PUTROOTFH)
	finishNormal := heldReply(t, srv, rpcCallForTest(2, procCompound, admissionSequence(body, ts, 0, 1, true)))
	finishReplay := heldReply(t, srv, rpcCallForTest(3, procCompound, replayed))
	if a := ss.accounting(); a.ActiveRequests != 2 || a.GroupCredits != 2 {
		t.Fatalf("held replies %+v", a)
	}
	ss.mu.Lock()
	ss.groupLimit = ss.groupCredits // No spare credit.
	ss.mu.Unlock()

	// Replacement drops every session of the incarnation and its wire
	// record, and only those.
	ss.dropClient(incarnation)
	if ss.lookup(ts.id) != nil || ss.lookup(second.id) != nil || ss.lookup(other.id) == nil {
		t.Fatal("dropClient did not drop exactly the incarnation's sessions")
	}
	if a := ss.accounting(); a.Sessions != 1 || a.Records != 1 || wireRecords(srv) != 1 ||
		a.Groups != 2 || a.GroupCredits != 2 || a.ActiveRequests != 2 {
		t.Fatalf("after session and record removal %+v, %d wire records", a, wireRecords(srv))
	}
	assertNoGroupCredit := func(when string) {
		t.Helper()
		if got := identityExchange(t, srv, 1, "new", [8]byte{1}, false); got.status != NFS4ERR_DELAY {
			t.Fatalf("EXCHANGE_ID %s: %s", when, Nfsstat4Name(got.status))
		}
		if _, status := ss.beginCreate(incarnation + 1); status != NFS4ERR_DELAY {
			t.Fatalf("CREATE_SESSION %s: %s", when, Nfsstat4Name(status))
		}
		if ss.closeForReplacement(incarnation + 1) {
			t.Fatalf("replacement %s took a group", when)
		}
		if a := ss.accounting(); a.GroupCredits != 2 || wireRecords(srv) != 1 {
			t.Fatalf("failed reservation %s changed state %+v", when, a)
		}
	}
	assertNoGroupCredit("while both replies are held")

	finishNormal()
	waitForAccounting(t, ss, "normal reply done", func(a replyAccounting) bool { return a.ActiveRequests == 1 })
	assertNoGroupCredit("while the replay is held")

	finishReplay()
	waitForAccounting(t, ss, "final reply done", func(a replyAccounting) bool {
		return a.ActiveRequests == 0 && a.Groups == 1 && a.GroupCredits == 1
	})
	if got := identityExchange(t, srv, 1, "new", [8]byte{1}, false); got.status != NFS4_OK {
		t.Fatalf("EXCHANGE_ID after the final release %s", Nfsstat4Name(got.status))
	}
}

// BIND_CONN_TO_SESSION waits while its incarnation is being replaced and
// fails once it has been.
func TestGroupBindWhileReplacing(t *testing.T) {
	srv := identityServer(t, NewLocalTernVFS(t.TempDir()))
	wire, ts := registerForTest(t, srv, "bind", 1)
	incarnation := incarnationForTest(t, srv, wire)
	bind := func() uint32 {
		t.Helper()
		_, res := admissionRequest(t, srv, admissionBind(ts.id, CDFC4_FORE, false), 1, &rpcConnection{})
		return res.Status()
	}
	if got := bind(); got != NFS4_OK {
		t.Fatalf("BIND %s", Nfsstat4Name(got))
	}
	if !srv.sessions.closeForReplacement(incarnation) {
		t.Fatal("cannot close")
	}
	if got := bind(); got != NFS4ERR_DELAY {
		t.Fatalf("BIND while closing %s", Nfsstat4Name(got))
	}
	srv.sessions.reopen(incarnation)
	if got := bind(); got != NFS4_OK {
		t.Fatalf("BIND after reopen %s", Nfsstat4Name(got))
	}
	if !srv.sessions.closeForReplacement(incarnation) {
		t.Fatal("cannot close again")
	}
	srv.sessions.replaced(incarnation)
	if got := bind(); got != NFS4ERR_BADSESSION {
		t.Fatalf("BIND after replacement %s", Nfsstat4Name(got))
	}
}
