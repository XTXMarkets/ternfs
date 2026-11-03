// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func createSessionBodyWith(clientID uint64, seq, flags, maxRequest uint32) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		cw := w.AppendArgarray_CreateSession()
		cw.SetCsaClientid(clientID)
		cw.SetCsaSequence(seq)
		cw.SetCsaFlags(flags)
		attrs := func(a nfs4.ChannelAttrs4Writer) []byte {
			a.SetCaMaxrequestsize(maxRequest)
			a.SetCaMaxresponsesize(maxRPCMessage)
			a.SetCaMaxresponsesizeCached(maxSessionReplyCached)
			a.SetCaMaxoperations(maxCompoundOperations)
			a.SetCaMaxrequests(16)
			return a.Finish()
		}
		cw.Resume(attrs(cw.StartCsaForeChanAttrs()))
		cw.Resume(attrs(cw.StartCsaBackChanAttrs()))
		cw.SetCsaCbProgram(0x40000000)
		cw.AppendCsaSecParms_None()
		w.Resume(cw.Finish())
	})
}

// createResult runs CREATE_SESSION as uid and returns its status and, on
// success, the session ID.
func createResult(t *testing.T, srv *Server, uid uint32, clientID uint64, seq uint32) (uint32, SessionID) {
	t.Helper()
	res := identityCall(t, srv, uid, createSessionBody(clientID, seq))
	if res.Status() != nfs4.NFS4_OK {
		return res.Status(), SessionID{}
	}
	it := res.Resarray()
	it.Next()
	return nfs4.NFS4_OK, readSessionID(it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok().CsrSessionid())
}

func creationSlot(srv *Server, clientID uint64) (seq uint32, cached, busy bool) {
	srv.sessions.mu.Lock()
	defer srv.sessions.mu.Unlock()
	c := srv.sessions.wireRecordLocked(clientID)
	return c.csSeq, c.csCached, c.csBusy
}

func sequenceStatusForTest(t *testing.T, srv *Server, id SessionID) uint32 {
	t.Helper()
	ts := newTestSession(id, 0, 1)
	_, res := admissionRequest(t, srv, admissionSequence(buildCompoundBody(nil, nil), ts, 0, 1, true), 1, nil)
	return res.Status()
}

// The CREATE_SESSION outcome table (README.md#create_session-outcomes).
func TestCreateSessionOutcomeTable(t *testing.T) {
	srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	ex := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	wire := ex.clientID
	unchanged := func(what string) {
		t.Helper()
		if seq, cached, busy := creationSlot(srv, wire); seq != 0 || cached || busy {
			t.Fatalf("%s changed the slot: %d %t %t", what, seq, cached, busy)
		}
	}
	// Validation and sequence errors leave the slot alone and cache nothing.
	for _, c := range []struct {
		name string
		body []byte
		want uint32
	}{
		{"flags", createSessionBodyWith(wire, 1, 0x80, maxRPCMessage), nfs4.NFS4ERR_INVAL},
		{"too small", createSessionBodyWith(wire, 1, 0, minSessionRequest-1), nfs4.NFS4ERR_TOOSMALL},
		{"ahead", createSessionBody(wire, 2), nfs4.NFS4ERR_SEQ_MISORDERED},
		{"slot sequence with nothing cached", createSessionBody(wire, 0), nfs4.NFS4ERR_SEQ_MISORDERED},
		{"unknown clientid", createSessionBody(wire+1000, 1), nfs4.NFS4ERR_STALE_CLIENTID},
	} {
		if got := identityCall(t, srv, 1, c.body).Status(); got != c.want {
			t.Fatalf("%s: %s, want %s", c.name, nfs4.Nfsstat4Name(got), nfs4.Nfsstat4Name(c.want))
		}
		unchanged(c.name)
	}
	// A failed reservation is DELAY, before the slot.
	srv.sessions.mu.Lock()
	srv.sessions.recordLimit = srv.sessions.records
	srv.sessions.mu.Unlock()
	if got, _ := createResult(t, srv, 1, wire, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("reservation failure %s", nfs4.Nfsstat4Name(got))
	}
	unchanged("reservation failure")
	srv.sessions.mu.Lock()
	srv.sessions.recordLimit = maxSessionRecords
	srv.sessions.mu.Unlock()

	// Another principal on an unconfirmed record: CLID_INUSE, cached. Its
	// replay returns it to anyone; the next sequence proceeds.
	for _, uid := range []uint32{2, 2, 1} {
		if got, _ := createResult(t, srv, uid, wire, 1); got != nfs4.NFS4ERR_CLID_INUSE {
			t.Fatalf("uid %d CREATE_SESSION 1 %s", uid, nfs4.Nfsstat4Name(got))
		}
	}
	if seq, cached, busy := creationSlot(srv, wire); seq != 1 || !cached || busy {
		t.Fatalf("CLID_INUSE slot %d %t %t", seq, cached, busy)
	}
	status, first := createResult(t, srv, 1, wire, 2)
	if status != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION 2 %s", nfs4.Nfsstat4Name(status))
	}
	// Replay returns the cached result; misordering does not overwrite it.
	for _, seq := range []uint32{1, 4, 2} {
		got, id := createResult(t, srv, 1, wire, seq)
		if seq == 2 {
			if got != nfs4.NFS4_OK || id != first {
				t.Fatalf("replay %s %x, want %x", nfs4.Nfsstat4Name(got), id, first)
			}
		} else if got != nfs4.NFS4ERR_SEQ_MISORDERED {
			t.Fatalf("CREATE_SESSION %d %s", seq, nfs4.Nfsstat4Name(got))
		}
	}
	if a := srv.sessions.accounting(); a.Sessions != 1 {
		t.Fatalf("replays created sessions: %+v", a)
	}
	// A confirmed record serves any principal under SP4_NONE.
	if got, id := createResult(t, srv, 2, wire, 3); got != nfs4.NFS4_OK || id == first {
		t.Fatalf("CREATE_SESSION 3 by another principal %s", nfs4.Nfsstat4Name(got))
	}
	if got := identityExchange(t, srv, 1, "client", [8]byte{1}, false); got.clientID != wire || got.seq != 4 {
		t.Fatalf("warm EXCHANGE_ID %+v", got)
	}
}

// identityCallOn is identityCall on a transport connection.
func identityCallOn(t *testing.T, srv *Server, uid uint32, conn *rpcConnection, body []byte) nfs4.COMPOUND4res {
	t.Helper()
	req, err := parseRPCCall(rpcCallForTest(1, procCompound, body))
	if err != nil {
		t.Fatal(err)
	}
	req.credFlavor, req.credBody, req.connection = nfs4.AUTH_SYS, authSysCredForTest(uid), conn
	reply := srv.safeHandleCompound(req, "identity-test")
	res, err := parseCompoundReply(append([]byte(nil), reply...))
	req.releaseReply()
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// The creation sequence wraps with uint32 arithmetic.
func TestCreateSessionSequenceWraps(t *testing.T) {
	srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
	if got, _ := createResult(t, srv, 1, wire, 1); got != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(got))
	}
	srv.sessions.mu.Lock()
	srv.sessions.wireRecordLocked(wire).csSeq = 0xfffffffe
	srv.sessions.mu.Unlock()
	if got := identityExchange(t, srv, 1, "client", [8]byte{1}, false); got.seq != 0xffffffff {
		t.Fatalf("warm EXCHANGE_ID sequence %#x", got.seq)
	}
	for _, seq := range []uint32{0xffffffff, 0} {
		if got, _ := createResult(t, srv, 1, wire, seq); got != nfs4.NFS4_OK {
			t.Fatalf("CREATE_SESSION %#x %s", seq, nfs4.Nfsstat4Name(got))
		}
	}
	if got := identityExchange(t, srv, 1, "client", [8]byte{1}, false); got.seq != 1 {
		t.Fatalf("warm EXCHANGE_ID after wrap %#x", got.seq)
	}
}

// While a CREATE_SESSION owns the slot: the same sequence waits, the cached
// sequence replays, any other is misordered and DESTROY_CLIENTID is busy.
// Exactly one session results.
func TestCreateSessionConcurrentSameSequence(t *testing.T) {
	srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	fs := gateCreations(t, srv)
	wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
	status, previous := createResult(t, srv, 1, wire, 1)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	fs.armed.Store(true)
	type result struct {
		status uint32
		id     SessionID
	}
	running := make(chan result, 1)
	go func() {
		status, id := createResult(t, srv, 1, wire, 2)
		running <- result{status, id}
	}()
	testutil.AwaitSignal(t, fs.entered, "CREATE_SESSION 2 in progress")
	if got, _ := createResult(t, srv, 1, wire, 2); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("same sequence while running %s", nfs4.Nfsstat4Name(got))
	}
	if got, id := createResult(t, srv, 1, wire, 1); got != nfs4.NFS4_OK || id != previous {
		t.Fatalf("cached sequence while running %s", nfs4.Nfsstat4Name(got))
	}
	if got, _ := createResult(t, srv, 1, wire, 3); got != nfs4.NFS4ERR_SEQ_MISORDERED {
		t.Fatalf("other sequence while running %s", nfs4.Nfsstat4Name(got))
	}
	if got := identityDestroy(t, srv, wire); got != nfs4.NFS4ERR_CLIENTID_BUSY {
		t.Fatalf("DESTROY_CLIENTID while running %s", nfs4.Nfsstat4Name(got))
	}
	testutil.CloseSignal(fs.release)
	r := testutil.AwaitValue(t, running, "CREATE_SESSION 2")
	if r.status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(r.status))
	}
	if got, id := createResult(t, srv, 1, wire, 2); got != nfs4.NFS4_OK || id != r.id {
		t.Fatalf("replay of 2 %s", nfs4.Nfsstat4Name(got))
	}
	if a := srv.sessions.accounting(); a.Sessions != 2 {
		t.Fatalf("sessions %+v", a)
	}
}

// An unconfirmed record whose CREATE_SESSION is in progress is not
// reclaimed at the record cap.
func TestCreateSessionInProgressRecordNotReclaimed(t *testing.T) {
	local, dir := vfs.NewLocalTernVFS(t.TempDir()), t.TempDir()
	first := identityServerAt(t, local, dir)
	p := identityExchange(t, first, 1, "client", [8]byte{1}, false)
	if got, _ := createResult(t, first, 1, p.clientID, 1); got != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(got))
	}
	srv := identityServerAt(t, local, dir)
	fs := gateCreations(t, srv)
	var clock atomic.Int64
	start := time.Now()
	clock.Store(start.UnixNano())
	srv.SetClock(func() time.Time { return time.Unix(0, clock.Load()) })
	adopted := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	fs.armed.Store(true)
	running := make(chan uint32, 1)
	go func() {
		status, _ := createResult(t, srv, 1, adopted.clientID, 1)
		running <- status
	}()
	testutil.AwaitSignal(t, fs.entered, "adopted confirmation")
	clock.Store(start.Add(leaseTime).UnixNano())
	srv.sessions.wireLimit = 1
	if got := identityExchange(t, srv, 1, "other", [8]byte{1}, false); got.status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("EXCHANGE_ID at the cap %s", nfs4.Nfsstat4Name(got.status))
	}
	testutil.CloseSignal(fs.release)
	if got := testutil.AwaitValue(t, running, "adopted CREATE_SESSION"); got != nfs4.NFS4_OK {
		t.Fatalf("adopted CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
}

// A replay of a cached outcome creates nothing, so it reserves no session
// record or serial: it works with the budget saturated. Here a first
// confirmation commits with the record limit 1 and its reply is lost; the
// client's retry returns exactly that session, and nothing else is
// allocated. A destroyed session's replay at the limit still returns its
// result without rebuilding it.
func TestCachedReplayAtRecordLimit(t *testing.T) {
	srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	ss := srv.sessions
	wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
	ss.mu.Lock()
	ss.recordLimit = 1
	ss.mu.Unlock()
	status, committed := createResult(t, srv, 1, wire, 1)
	if status != nfs4.NFS4_OK {
		t.Fatalf("first CREATE_SESSION %s", nfs4.Nfsstat4Name(status))
	}
	if a := ss.accounting(); a.Sessions != 1 || a.Records != 1 || a.PendingSessions != 0 {
		t.Fatalf("first commit %+v", a)
	}
	conn := &rpcConnection{}
	res := identityCallOn(t, srv, 1, conn, createSessionBody(wire, 1))
	it := res.Resarray()
	if res.Status() != nfs4.NFS4_OK || !it.Next() ||
		readSessionID(it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok().CsrSessionid()) != committed {
		t.Fatalf("retry after the lost reply %s", nfs4.Nfsstat4Name(res.Status()))
	}
	if !conn.associated(committed) {
		t.Fatal("replay not associated")
	}
	if a := ss.accounting(); a.Sessions != 1 || a.Records != 1 || a.PendingSessions != 0 {
		t.Fatalf("replay allocated %+v", a)
	}
	// A genuinely new creation is still refused at the limit.
	if got, _ := createResult(t, srv, 1, wire, 2); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("new CREATE_SESSION at the limit %s", nfs4.Nfsstat4Name(got))
	}
	// A destroyed session's replay returns its result and rebuilds nothing.
	if !ss.destroySession(committed) {
		t.Fatal("DESTROY_SESSION")
	}
	ss.mu.Lock()
	ss.recordLimit = ss.records + ss.pendingSessions
	ss.mu.Unlock()
	fresh := &rpcConnection{}
	if got := identityCallOn(t, srv, 1, fresh, createSessionBody(wire, 1)).Status(); got != nfs4.NFS4_OK ||
		fresh.associated(committed) || ss.lookup(committed) != nil {
		t.Fatalf("destroyed replay %s", nfs4.Nfsstat4Name(got))
	}
}

// With session serials exhausted, cached outcomes still replay (OK and
// CLID_INUSE) and misordering is still reported, while a new creation is
// refused before its slot.
func TestCachedReplayAtSerialExhaustion(t *testing.T) {
	srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	ss := srv.sessions
	wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
	status, first := createResult(t, srv, 1, wire, 1)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	other := identityExchange(t, srv, 1, "other", [8]byte{1}, false).clientID
	if got, _ := createResult(t, srv, 2, other, 1); got != nfs4.NFS4ERR_CLID_INUSE {
		t.Fatalf("CREATE_SESSION by another principal %s", nfs4.Nfsstat4Name(got))
	}
	ss.mu.Lock()
	ss.next = uint64(^uint32(0))
	ss.mu.Unlock()
	if got, id := createResult(t, srv, 1, wire, 1); got != nfs4.NFS4_OK || id != first {
		t.Fatalf("replay at serial exhaustion %s", nfs4.Nfsstat4Name(got))
	}
	if got, _ := createResult(t, srv, 2, other, 1); got != nfs4.NFS4ERR_CLID_INUSE {
		t.Fatalf("cached CLID_INUSE at serial exhaustion %s", nfs4.Nfsstat4Name(got))
	}
	if got, _ := createResult(t, srv, 1, wire, 5); got != nfs4.NFS4ERR_SEQ_MISORDERED {
		t.Fatalf("misordered at serial exhaustion %s", nfs4.Nfsstat4Name(got))
	}
	if got, _ := createResult(t, srv, 1, wire, 2); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("new CREATE_SESSION at serial exhaustion %s", nfs4.Nfsstat4Name(got))
	}
	if seq, cached, busy := creationSlot(srv, wire); seq != 1 || !cached || busy {
		t.Fatalf("refused creation changed the slot: %d %t %t", seq, cached, busy)
	}
	if a := ss.accounting(); a.Sessions != 1 || a.PendingSessions != 0 {
		t.Fatalf("accounting %+v", a)
	}
}

// DESTROY_CLIENTID's earlier checks run without the session-store lock, and
// CREATE_SESSION does not take the owner lock. Its final step re-checks the
// creation slot and the incarnation's sessions under the retirement lock, so
// a creator that started, or finished, after the earlier checks is never
// destroyed.
func TestDestroyClientIDFinalCheckSeesCreation(t *testing.T) {
	srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	fs := gateCreations(t, srv)
	ss := srv.sessions
	wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
	status, first := createResult(t, srv, 1, wire, 1)
	if status != nfs4.NFS4_OK || !ss.destroySession(first) {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(status))
	}
	// The record is idle: DESTROY_CLIENTID's earlier checks would pass now.
	ss.mu.Lock()
	c := ss.wireRecordLocked(wire)
	g := ss.groups[c.incarnation]
	idle := !c.csBusy && (g == nil || g.members == 0)
	ss.mu.Unlock()
	if !idle {
		t.Fatal("record not idle")
	}

	// A creator takes the slot before the destruction's final step.
	fs.armed.Store(true)
	created := make(chan uint32, 1)
	go func() {
		status, _ := createResult(t, srv, 1, wire, 2)
		created <- status
	}()
	testutil.AwaitSignal(t, fs.entered, "CREATE_SESSION in progress")
	if got := ss.destroyClientRecord(c); got != nfs4.NFS4ERR_CLIENTID_BUSY {
		t.Fatalf("destruction while creating %s", nfs4.Nfsstat4Name(got))
	}
	testutil.CloseSignal(fs.release)
	if got := testutil.AwaitValue(t, created, "CREATE_SESSION"); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	// The creator has inserted its session.
	if got := ss.destroyClientRecord(c); got != nfs4.NFS4ERR_CLIENTID_BUSY {
		t.Fatalf("destruction after creation %s", nfs4.Nfsstat4Name(got))
	}
	if got := identityExchange(t, srv, 1, "client", [8]byte{1}, false); got.clientID != wire || !got.confirmed {
		t.Fatalf("record destroyed: %+v", got)
	}
}

// At the connection's association quota, replaying a session the
// connection already has needs no new association; a new session is
// refused there, before the slot.
func TestCreateSessionReplayAtConnectionQuota(t *testing.T) {
	srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
	conn := &rpcConnection{}
	var last SessionID
	for seq := uint32(1); seq <= maxConnectionSessions; seq++ {
		res := identityCallOn(t, srv, 1, conn, createSessionBody(wire, seq))
		if res.Status() != nfs4.NFS4_OK {
			t.Fatalf("CREATE_SESSION %d %s", seq, nfs4.Nfsstat4Name(res.Status()))
		}
		it := res.Resarray()
		it.Next()
		last = readSessionID(it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok().CsrSessionid())
	}
	replay := identityCallOn(t, srv, 1, conn, createSessionBody(wire, maxConnectionSessions))
	if replay.Status() != nfs4.NFS4_OK || !conn.associated(last) {
		t.Fatalf("replay of an associated session at the quota %s", nfs4.Nfsstat4Name(replay.Status()))
	}
	if got := identityCallOn(t, srv, 1, conn, createSessionBody(wire, maxConnectionSessions+1)).Status(); got != nfs4.NFS4ERR_RESOURCE {
		t.Fatalf("new session at the quota %s", nfs4.Nfsstat4Name(got))
	}
	if seq, cached, busy := creationSlot(srv, wire); seq != maxConnectionSessions || !cached || busy {
		t.Fatalf("refused creation changed the slot: %d %t %t", seq, cached, busy)
	}
	// Another connection can still create it.
	if got := identityCallOn(t, srv, 1, &rpcConnection{}, createSessionBody(wire, maxConnectionSessions+1)).Status(); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION on another connection %s", nfs4.Nfsstat4Name(got))
	}
}
