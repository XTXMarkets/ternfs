// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

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

// The association quota counts distinct IDs. An ID committed by one
// token while another for it is still pending counts once, so the 64th
// distinct ID is admitted.
func TestAssociationQuotaCountsDistinctIDs(t *testing.T) {
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
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
