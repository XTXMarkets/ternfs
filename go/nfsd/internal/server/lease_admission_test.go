// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
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

// Association tokens: a commit by BIND survives another request's abort,
// a pending-only failure leaves nothing, and pending tokens count toward
// the quota so SEQUENCE gets RESOURCE before any renewal work.
func TestAssociationTokens(t *testing.T) {
	srv, err := NewServer(vfs.NewLocalTernVFS(t.TempDir()), staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	conn := &rpcConnection{}
	pending, ok := conn.reserve(srv, ts.id)
	if !ok || !pending.new {
		t.Fatal("reserve")
	}
	if !conn.bind(srv, ts.id) {
		t.Fatal("bind")
	}
	pending.abort()
	if !conn.associated(ts.id) {
		t.Fatal("abort removed BIND's association")
	}
	other := connectionSession(t, srv)
	failed, _ := conn.reserve(srv, other.id)
	failed.abort()
	failed.abort()
	conn.mu.Lock()
	if _, ok := conn.sessions[other.id]; ok || len(conn.pending) != 0 {
		conn.mu.Unlock()
		t.Fatal("pending-only failure left an association")
	}
	conn.mu.Unlock()

	// Fill the quota with pending tokens.
	var tokens []*associationToken
	for len(tokens) < maxConnectionSessions-1 {
		s := connectionSession(t, srv)
		tok, ok := conn.reserve(srv, s.id)
		if !ok {
			t.Fatalf("reserve %d refused", len(tokens))
		}
		tokens = append(tokens, tok)
	}
	if _, ok := conn.reserve(srv, other.id); ok {
		t.Fatal("pending tokens not counted against the quota")
	}
	// SEQUENCE on the full connection: RESOURCE, with no verify requested.
	sess := srv.sessions.lookup(other.id)
	setLeaseForTest(srv, sess.clientID, func(r *clientstore.LeaseRefresher) { r.Proven = 0 })
	body := buildCompoundBody(nil, func(*nfs4.COMPOUND4argsWriter) {})
	_, res := admissionRequest(t, srv, admissionSequence(body, other, 0, 1, true), 1, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_RESOURCE, nfs4.OP_SEQUENCE)
	setLeaseForTest(srv, sess.clientID, func(r *clientstore.LeaseRefresher) {
		if r.Verify {
			t.Error("RESOURCE admission requested a verify")
		}
	})
	for _, tok := range tokens {
		tok.abort()
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if len(conn.pending) != 0 || len(conn.sessions) != 1 {
		t.Fatalf("pending %d sessions %d", len(conn.pending), len(conn.sessions))
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
