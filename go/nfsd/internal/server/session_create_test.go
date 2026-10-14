// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
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

// The CREATE_SESSION outcome table, canonical r3 §5.1.
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
	srv.sessions.recordLimit = clientstore.MaxSessionRecords
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

// A destroyed session's cached creation result still replays, and the
// session stays destroyed.
func TestCreateSessionReplayOfDestroyedSession(t *testing.T) {
	srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
	status, id := createResult(t, srv, 1, wire, 1)
	if status != nfs4.NFS4_OK || !srv.sessions.destroySession(id) {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(status))
	}
	conn := &rpcConnection{}
	res := identityCallOn(t, srv, 1, conn, createSessionBody(wire, 1))
	it := res.Resarray()
	if res.Status() != nfs4.NFS4_OK || !it.Next() ||
		readSessionID(it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok().CsrSessionid()) != id {
		t.Fatalf("replay %s", nfs4.Nfsstat4Name(res.Status()))
	}
	if srv.sessions.lookup(id) != nil || conn.associated(id) {
		t.Fatal("replay rebuilt or associated the destroyed session")
	}
	if got := sequenceStatusForTest(t, srv, id); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("SEQUENCE on the destroyed session %s", nfs4.Nfsstat4Name(got))
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
	fs := &confirmedLookupGateVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	srv := identityServer(t, fs)
	defer testutil.CloseSignal(fs.release)
	wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
	status, previous := createResult(t, srv, 1, wire, 1)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	srv.waitForClientGC()
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
	testutil.AwaitSignal(t, fs.entered, "CREATE_SESSION 2 checking the store")
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
	local := vfs.NewLocalTernVFS(t.TempDir())
	first := identityServer(t, local)
	p := identityExchange(t, first, 1, "client", [8]byte{1}, false)
	if got, _ := createResult(t, first, 1, p.clientID, 1); got != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(got))
	}
	fs := &confirmedLookupGateVFS{TernVFS: local, entered: make(chan struct{}), release: make(chan struct{})}
	srv := identityServer(t, fs)
	defer testutil.CloseSignal(fs.release)
	var clock atomic.Int64
	start := time.Now()
	clock.Store(start.UnixNano())
	srv.clients.SetClock(func() time.Time { return time.Unix(0, clock.Load()) })
	adopted := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	srv.waitForClientGC()
	fs.armed.Store(true)
	running := make(chan uint32, 1)
	go func() {
		status, _ := createResult(t, srv, 1, adopted.clientID, 1)
		running <- status
	}()
	testutil.AwaitSignal(t, fs.entered, "adopted confirmation")
	clock.Store(start.Add(clientstore.LeaseTime).UnixNano())
	srv.sessions.wireLimit = 1
	if got := identityExchange(t, srv, 1, "other", [8]byte{1}, false); got.status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("EXCHANGE_ID at the cap %s", nfs4.Nfsstat4Name(got.status))
	}
	testutil.CloseSignal(fs.release)
	if got := testutil.AwaitValue(t, running, "adopted CREATE_SESSION"); got != nfs4.NFS4_OK {
		t.Fatalf("adopted CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
}

// leaseRenameFailureVFS fails the publication renames into lease slots
// while armed.
type leaseRenameFailureVFS struct {
	vfs.TernVFS
	armed atomic.Bool
}

func (fs *leaseRenameFailureVFS) RenameEdge(src vfs.InodeID, name string, edge vfs.Edge, dst vfs.InodeID, newName string) error {
	if fs.armed.Load() && strings.HasPrefix(newName, clientstore.LeasePrefix) {
		return clientstore.ErrInjectedLookup
	}
	return fs.TernVFS.RenameEdge(src, name, edge, dst, newName)
}

// A committed CREATE_SESSION starts bootstrap, the client's first durable
// publication, and never becomes an error (canonical r3 §9.1). Until the
// publication SEQUENCE gets DELAY; a store that proves the client expired
// makes the session unusable while its cached result still replays.
func TestCreateSessionBootstrap(t *testing.T) {
	t.Run("publication failing", func(t *testing.T) {
		fs := &leaseRenameFailureVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
		srv := identityServer(t, fs)
		wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
		incarnation := incarnationForTest(t, srv, wire)
		fs.armed.Store(true)
		status, id := createResult(t, srv, 1, wire, 1)
		if status != nfs4.NFS4_OK || srv.sessions.lookup(id) == nil {
			t.Fatalf("CREATE_SESSION with failing publication %s", nfs4.Nfsstat4Name(status))
		}
		leasePassForTest(t, srv, incarnation)
		if got := sequenceStatusForTest(t, srv, id); got != nfs4.NFS4ERR_DELAY {
			t.Fatalf("SEQUENCE before the first publication %s", nfs4.Nfsstat4Name(got))
		}
		if got, replay := createResult(t, srv, 1, wire, 1); got != nfs4.NFS4_OK || replay != id {
			t.Fatalf("replay %s", nfs4.Nfsstat4Name(got))
		}
		// The store recovers: the attempt is fenced, then published.
		fs.armed.Store(false)
		leasePassForTest(t, srv, incarnation)
		awaitLeaseProven(t, srv, incarnation)
		if got := sequenceStatusForTest(t, srv, id); got != nfs4.NFS4_OK {
			t.Fatalf("SEQUENCE after the publication %s", nfs4.Nfsstat4Name(got))
		}
	})
	t.Run("expired", func(t *testing.T) {
		local := vfs.NewLocalTernVFS(t.TempDir())
		srv := identityServer(t, local)
		wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
		incarnation := vfs.InodeID(incarnationForTest(t, srv, wire))
		// Another process expired the client; the store check still passes,
		// so the creation commits and bootstrap then finds the marker.
		srv.waitForClientGC()
		if _, err := local.CreateFile(incarnation, clientstore.ExpiredName, nil); err != nil {
			t.Fatal(err)
		}
		status, id := createResult(t, srv, 1, wire, 1)
		if status != nfs4.NFS4_OK {
			t.Fatalf("committed CREATE_SESSION became %s", nfs4.Nfsstat4Name(status))
		}
		leasePassForTest(t, srv, uint64(incarnation))
		if srv.sessions.lookup(id) != nil {
			t.Fatal("expired client's session stayed usable")
		}
		if got := sequenceStatusForTest(t, srv, id); got != nfs4.NFS4ERR_BADSESSION {
			t.Fatalf("SEQUENCE %s", nfs4.Nfsstat4Name(got))
		}
		if got, replay := createResult(t, srv, 1, wire, 1); got != nfs4.NFS4_OK || replay != id {
			t.Fatalf("committed result lost: %s", nfs4.Nfsstat4Name(got))
		}
	})
	t.Run("record credit", func(t *testing.T) {
		srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
		wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
		srv.clients.RefreshMu.Lock()
		srv.clients.RefresherLimit = len(srv.clients.Refreshers)
		srv.clients.RefreshMu.Unlock()
		if got, _ := createResult(t, srv, 1, wire, 1); got != nfs4.NFS4ERR_DELAY {
			t.Fatalf("CREATE_SESSION without a refresher credit %s", nfs4.Nfsstat4Name(got))
		}
		if seq, cached, busy := creationSlot(srv, wire); seq != 0 || cached || busy {
			t.Fatalf("refused creation changed the slot: %d %t %t", seq, cached, busy)
		}
	})
}

// A replay of a cached outcome creates nothing, so it reserves no session
// record or serial: it works with the budget saturated. Here the sweeper
// commits a first confirmation (no predecessor) with the record limit 1
// before the client's retry; the retry returns exactly that session, and
// nothing else is allocated. A destroyed session's replay at the limit still
// returns its result without rebuilding it.
func TestCachedReplayAtRecordLimit(t *testing.T) {
	fs := clientstore.NewConfirmFaultVFS(t)
	srv := identityServer(t, fs)
	ss := srv.sessions
	wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
	ss.mu.Lock()
	ss.recordLimit = 1
	ss.mu.Unlock()
	fs.Set(clientstore.WriteD, clientstore.FaultAppliedFail, nil)
	fs.Mu.Lock()
	fs.Armed = true
	fs.Mu.Unlock()
	if got, _ := createResult(t, srv, 1, wire, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("CREATE_SESSION with an unfenceable D %s", nfs4.Nfsstat4Name(got))
	}
	fs.Set(clientstore.WriteD, clientstore.FaultNone, nil)
	srv.settleConfirmations()
	var committed SessionID
	ss.mu.Lock()
	for id := range ss.sessions {
		committed = id
	}
	busy := ss.wireRecordLocked(wire).csBusy
	ss.mu.Unlock()
	if a := ss.accounting(); a.Sessions != 1 || a.Records != 1 || a.PendingSessions != 0 || busy {
		t.Fatalf("sweeper commit %+v, slot busy %t", a, busy)
	}
	conn := &rpcConnection{}
	res := identityCallOn(t, srv, 1, conn, createSessionBody(wire, 1))
	it := res.Resarray()
	if res.Status() != nfs4.NFS4_OK || !it.Next() ||
		readSessionID(it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok().CsrSessionid()) != committed {
		t.Fatalf("retry after the sweeper's commit %s", nfs4.Nfsstat4Name(res.Status()))
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
	fs := &confirmedLookupGateVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	srv := identityServer(t, fs)
	defer testutil.CloseSignal(fs.release)
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
	srv.waitForClientGC()
	fs.armed.Store(true)
	created := make(chan uint32, 1)
	go func() {
		status, _ := createResult(t, srv, 1, wire, 2)
		created <- status
	}()
	testutil.AwaitSignal(t, fs.entered, "CREATE_SESSION in the store")
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

// leaseLinkPanicVFS panics in the LINK of lease temps while armed.
type leaseLinkPanicVFS struct {
	vfs.TernVFS
	armed atomic.Bool
}

func (fs *leaseLinkPanicVFS) LinkFile(id vfs.InodeID, cookie vfs.Cookie, dir vfs.InodeID, name string, data io.Reader) error {
	if strings.HasPrefix(name, clientstore.TempPrefix) && fs.armed.CompareAndSwap(true, false) {
		panic("injected publication panic")
	}
	return fs.TernVFS.LinkFile(id, cookie, dir, name, data)
}

// A panic in the bootstrap publication is contained in the refresher: the
// committed OK stands, in the reply and its replay, every hold and charge is
// released, and a later pass publishes.
func TestCreateSessionBootstrapPanicKeepsOK(t *testing.T) {
	fs := &leaseLinkPanicVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	srv := identityServer(t, fs)
	ss := srv.sessions
	wire := identityExchange(t, srv, 1, "client", [8]byte{1}, false).clientID
	incarnation := incarnationForTest(t, srv, wire)
	fs.armed.Store(true)
	status, id := createResult(t, srv, 1, wire, 1)
	if status != nfs4.NFS4_OK || ss.lookup(id) == nil {
		t.Fatalf("CREATE_SESSION with a publication panic %s", nfs4.Nfsstat4Name(status))
	}
	leasePassForTest(t, srv, incarnation)
	if fs.armed.Load() {
		t.Fatal("publication did not reach the injected panic")
	}
	if got, replay := createResult(t, srv, 1, wire, 1); got != nfs4.NFS4_OK || replay != id {
		t.Fatalf("replay after a publication panic %s", nfs4.Nfsstat4Name(got))
	}
	// The creation's group hold is gone.
	if !ss.closeForReplacement(incarnation) {
		t.Fatal("publication panic left the creation in progress")
	}
	ss.reopen(incarnation)
	leasePassForTest(t, srv, incarnation)
	awaitLeaseProven(t, srv, incarnation)
	ss.destroySession(id)
	if a := ss.accounting(); a.Records != 0 || a.Sessions != 0 || a.ActiveRequests != 0 || a.ProvisionalRequests != 0 {
		t.Fatalf("publication panic leaked %+v", a)
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
