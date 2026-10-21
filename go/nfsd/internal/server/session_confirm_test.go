// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"errors"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// confirmServer is a served nfsd over the edge-aware fault VFS. P is the
// shim's client with a staged write open; N is the same owner rebooted, a
// pending incarnation awaiting CREATE_SESSION, driven by raw calls so the
// shim keeps using P's session.
type confirmServer struct {
	fs           *clientstore.ConfirmFaultVFS
	srv          *Server
	conn         net.Conn
	xid          uint32
	clock        atomic.Int64
	wireP, wireN uint64
	p, n         uint64
	identity     vfs.InodeID
	state        [16]byte
	fh           []byte
}

func newConfirmServer(t *testing.T) *confirmServer {
	t.Helper()
	f := &confirmServer{fs: clientstore.NewConfirmFaultVFS(t), xid: 1}
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.srv, err = NewServer(f.fs, staging, nil); err != nil {
		t.Fatal(err)
	}
	f.clock.Store(time.Now().UnixNano())
	f.srv.clients.SetClock(func() time.Time { return time.Unix(0, f.clock.Load()) })
	addr, stop := serveTestServer(t, f.srv)
	t.Cleanup(stop)
	f.conn = dial(t, addr)
	t.Cleanup(func() { f.conn.Close() })
	f.wireP = setupClient(t, f.conn, &f.xid)
	f.p = incarnationForTest(t, f.srv, f.wireP)
	f.state, f.fh = openCreateFile(t, f.conn, &f.xid, f.wireP, "pfile")
	writeFileAt(t, f.conn, &f.xid, f.fh, f.state, 0, []byte("P's bytes"))
	wire, seq, status, err := exchangeID(f.conn, f.xid, "test-client", [8]byte{9})
	f.xid++
	if err != nil || status != nfs4.NFS4_OK || seq != 1 {
		t.Fatalf("EXCHANGE_ID for N %s %v", nfs4.Nfsstat4Name(status), err)
	}
	f.wireN, f.n = wire, incarnationForTest(t, f.srv, wire)
	if f.identity, err = f.fs.TernVFS.LookupParent(vfs.InodeID(f.n)); err != nil {
		t.Fatal(err)
	}
	f.srv.waitForClientGC()
	return f
}

func (f *confirmServer) advance(d time.Duration) { f.clock.Add(int64(d)) }

func (f *confirmServer) arm(w clientstore.WriteID, fault clientstore.ConfirmFault, hook func()) {
	f.fs.Set(w, fault, hook)
	f.fs.Mu.Lock()
	f.fs.Armed = true
	f.fs.Mu.Unlock()
}

// disarm stops recording new temps, so the armed fault stays with N's
// attempt and other clients' writes are unaffected.
func (f *confirmServer) disarm() {
	f.fs.Mu.Lock()
	f.fs.Armed = false
	f.fs.Mu.Unlock()
}

// rawCreate sends CREATE_SESSION without registering a shim session.
func (f *confirmServer) rawCreate(t *testing.T, wire uint64, seq uint32) (uint32, SessionID) {
	t.Helper()
	reply, err := sendRawRPC(f.conn, f.xid, procCompound, createSessionBody(wire, seq))
	f.xid++
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status() != nfs4.NFS4_OK {
		return res.Status(), SessionID{}
	}
	it := res.Resarray()
	it.Next()
	return nfs4.NFS4_OK, readSessionID(it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok().CsrSessionid())
}

func (f *confirmServer) rawStatus(t *testing.T, body []byte) uint32 {
	t.Helper()
	reply, err := sendRawRPC(f.conn, f.xid, procCompound, body)
	f.xid++
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	return res.Status()
}

// pSequence sends one SEQUENCE on P's session. A refused one leaves its
// slot sequence for the next attempt.
func (f *confirmServer) pSequence(t *testing.T) uint32 {
	t.Helper()
	ts := testSessionForClient(f.conn, f.wireP)
	slot, seq := ts.acquire()
	defer ts.release(slot)
	status := sequenceStatusOf(t, f, withSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, slot, seq))
	if status != nfs4.NFS4_OK {
		ts.mu.Lock()
		ts.seqs[slot]--
		ts.mu.Unlock()
	}
	return status
}

func sequenceStatusOf(t *testing.T, f *confirmServer, body []byte) uint32 {
	t.Helper()
	reply, err := sendRawRPC(f.conn, f.xid, procCompound, body)
	f.xid++
	if err != nil {
		t.Fatal(err)
	}
	return sequenceStatus(reply)
}

func (f *confirmServer) pSession(t *testing.T) SessionID {
	return testSessionForClient(f.conn, f.wireP).id
}

func (f *confirmServer) present(t *testing.T, dir vfs.InodeID, name string) bool {
	t.Helper()
	_, err := f.fs.TernVFS.Lookup(dir, name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

func (f *confirmServer) pMarker(t *testing.T) bool {
	var sid nfs4.StateID
	copy(sid[:], f.state[4:])
	return f.present(t, vfs.InodeID(f.p), clientstore.ActiveOpenName(sid))
}

func (f *confirmServer) pStaging(t *testing.T) staging.Meta {
	t.Helper()
	meta, ok := f.srv.stagingStore.GetMeta(stagingIDForTest(t, f.srv.stagingStore, f.state))
	if !ok {
		t.Fatal("P's staging is gone")
	}
	return meta
}

func (f *confirmServer) nSessions() int {
	ss := f.srv.sessions
	ss.mu.Lock()
	defer ss.mu.Unlock()
	n := 0
	for _, sess := range ss.sessions {
		if sess.clientID == f.n {
			n++
		}
	}
	return n
}

// The confirmation of N over P while it is unresolved: N and P are
// protected, P's admissions wait, N's record cannot be destroyed or
// superseded, and another client is unaffected. A proved rollback reopens P;
// the client's next attempt then commits and replaces P.
func TestConfirmationUnresolvedAdmission(t *testing.T) {
	f := newConfirmServer(t)
	f.arm(clientstore.WriteD, clientstore.FaultFenceErr, nil)
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("CREATE_SESSION with an unfenceable D %s", nfs4.Nfsstat4Name(got))
	}
	f.disarm()
	if !f.srv.protected(f.n) || !f.srv.protected(f.p) {
		t.Fatal("N and P are not protected")
	}
	if got := f.pSequence(t); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("P's SEQUENCE %s", nfs4.Nfsstat4Name(got))
	}
	if got := f.rawStatus(t, admissionBind(f.pSession(t), nfs4.CDFC4_FORE, false)); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("P's BIND %s", nfs4.Nfsstat4Name(got))
	}
	if got, _ := f.rawCreate(t, f.wireP, testSessionForClient(f.conn, f.wireP).nextCSSeq); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("P's CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	for _, wire := range []uint64{f.wireN, f.wireP} {
		if got := f.rawStatus(t, buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
			w.AppendArgarray_DestroyClientid().SetDcaClientid(wire)
		})); got != nfs4.NFS4ERR_CLIENTID_BUSY {
			t.Fatalf("DESTROY_CLIENTID %#x %s", wire, nfs4.Nfsstat4Name(got))
		}
	}
	if _, _, status, _ := exchangeID(f.conn, f.xid, "test-client", [8]byte{10}); status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("EXCHANGE_ID for the identity %s", nfs4.Nfsstat4Name(status))
	}
	f.xid++
	// The same sequence retries the fence; it still fails.
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("retry %s", nfs4.Nfsstat4Name(got))
	}

	// Another client opens, writes and closes normally meanwhile.
	other := dial(t, f.conn.RemoteAddr().String())
	defer other.Close()
	xid := uint32(5000)
	ts, err := createTestSession(other, xid, "other-client", [8]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	xid++
	state, fh := openCreateFile(t, other, &xid, ts.clientID, "otherfile")
	writeFileAt(t, other, &xid, fh, state, 0, []byte("other"))
	closeFile(t, other, &xid, fh, state)
	if got, _ := readFileData(t, other, &xid, fh, 0, 16); string(got) != "other" {
		t.Fatalf("other client's file %q", got)
	}

	// Fenced, D never applied and confirmed is still P through Sp: the
	// retry rolls back and P reopens.
	f.fs.Set(clientstore.WriteD, clientstore.FaultNone, nil)
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("rolled-back retry %s", nfs4.Nfsstat4Name(got))
	}
	if f.srv.protected(f.n) || f.srv.protected(f.p) {
		t.Fatal("rollback kept the protection")
	}
	if got := f.pSequence(t); got != nfs4.NFS4_OK {
		t.Fatalf("P's SEQUENCE after rollback %s", nfs4.Nfsstat4Name(got))
	}
	// The next attempt commits: P is replaced and purged, N has its session.
	status, id := f.rawCreate(t, f.wireN, 1)
	if status != nfs4.NFS4_OK || f.srv.sessions.lookup(id) == nil {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(status))
	}
	if got := f.pSequence(t); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("P's SEQUENCE after replacement %s", nfs4.Nfsstat4Name(got))
	}
	if f.pMarker(t) || f.present(t, vfs.InodeID(f.n), clientstore.RebootName) {
		t.Fatal("P not purged")
	}
	if got := identityConfirmed(t, f); got != vfs.InodeID(f.n) {
		t.Fatalf("confirmed %d", got)
	}
	// The held creation's file went to N's session, and no other is left.
	sessionFilesMatchSessions(t, f.srv)
}

func identityConfirmed(t *testing.T, f *confirmServer) vfs.InodeID {
	t.Helper()
	id, _, err := f.srv.clients.ReadIdentitySymlink(f.identity, clientstore.ConfirmedName)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// stateid4For gives the wire Stateid4 of a 16-byte stateid.
func stateid4For(t *testing.T, stateid [16]byte) nfs4.Stateid4 {
	t.Helper()
	args, ok := nfs4.ReadCOMPOUND4args(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		rw := w.AppendArgarray_Read()
		setStateid(rw.Stateid(), stateid)
		rw.SetCount(1)
	}))
	it := args.Argarray()
	if !ok || !it.Next() {
		t.Fatal("READ arguments")
	}
	return it.Argarray().Value().AsREAD4args().Stateid()
}

// While a confirmation is unresolved, no destructive path touches N or P:
// lease expiry, expired staging, the expired marker, client GC,
// requireActiveOpen expiry, recovered-staging retirement on READ and CLOSE,
// the expired-lease rule for another incarnation's staging, and inactive
// writer retirement. Time alone does not settle it. Once it settles, expiry
// proceeds.
func TestConfirmationProtectsDestructivePaths(t *testing.T) {
	f := newConfirmServer(t)
	srv := f.srv
	var sid nfs4.StateID
	copy(sid[:], f.state[4:])
	stagingID := stagingIDForTest(t, srv.stagingStore, f.state)
	meta := f.pStaging(t)

	// A recovered writer of P: staging from an earlier process's stateid.
	base, err := f.fs.TernVFS.CreateFile(f.fs.RootID(), "rfile", nil)
	if err != nil {
		t.Fatal(err)
	}
	recovered, cookie, err := f.fs.TernVFS.ConstructFile(f.fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	old := srv.opens.NewStateID()
	old[0] ^= 0xff // another epoch
	if _, err := srv.stagingStore.Create(recovered, staging.Meta{DirID: f.fs.RootID(), FileName: "rfile",
		BaseID: base, ClientID: f.p, NFSStateID: old, TernCookie: cookie}); err != nil {
		t.Fatal(err)
	}
	// Staging of P that N's RecoveryKey may claim once P's lease expires.
	keyed, keyedCookie, err := f.fs.TernVFS.ConstructFile(f.fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	key, err := srv.clients.StagingRecoveryKey(f.n)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.stagingStore.Create(keyed, staging.Meta{DirID: f.fs.RootID(), FileName: "kfile",
		ClientID: f.p, NFSStateID: srv.opens.NewStateID(), TernCookie: keyedCookie,
		OwnerKnown: true, OpenOwner: "owner", RecoveryKey: key}); err != nil {
		t.Fatal(err)
	}

	f.arm(clientstore.WriteD, clientstore.FaultFenceErr, nil)
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	f.disarm()
	// P's lease and the confirming claim expire, and P's open marker goes.
	f.advance(3 * clientstore.LeaseTime)
	if err := f.fs.TernVFS.Remove(vfs.InodeID(f.p), clientstore.ActiveOpenName(sid)); err != nil {
		t.Fatal(err)
	}

	srv.runLeaseSweep() // expiry, expired staging and settlement all run
	if !srv.protected(f.n) || !srv.protected(f.p) {
		t.Fatal("lease expiry settled an unresolved confirmation")
	}
	if f.present(t, vfs.InodeID(f.p), clientstore.ExpiredName) {
		t.Fatal("expired marker published for P")
	}
	// A lease reader finds P's lease dead but writes no marker: only
	// decideExpired publishes one, and the sweep above refused
	// (README.md#expiry-decision).
	if expired, err := srv.clients.IsLeaseExpired(f.p); err != nil || !expired || f.present(t, vfs.InodeID(f.p), clientstore.ExpiredName) {
		t.Fatalf("P's lease read %t %v, want dead with no marker", expired, err)
	}
	if recheck, err := srv.collectClientSafely(vfs.InodeID(f.n)); err != nil || !recheck {
		t.Fatalf("client GC ran: %t %v", recheck, err)
	}
	st := &compoundState{currentID: base, currentIDSet: true, clientID: f.p}
	fileID, _ := vfs.FHToInodeID(f.fh)
	if _, status := srv.lookupDurableOpen(f.p, stateid4For(t, f.state), fileID); status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("requireActiveOpen expiry %s", nfs4.Nfsstat4Name(status))
	}
	var oldWire [16]byte
	copy(oldWire[4:], old[:])
	if _, status := srv.lookupDurableOpen(f.p, stateid4For(t, oldWire), base); status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("recovered READ %s", nfs4.Nfsstat4Name(status))
	}
	if _, status := srv.releaseWrite(stateid4For(t, oldWire), st, true, nil); status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("recovered CLOSE %s", nfs4.Nfsstat4Name(status))
	}
	if _, _, _, release, err := srv.recoveredStagingTarget(f.fs.RootID(), "kfile", f.n, "owner", true); nfs4.ErrorCode(err) != nfs4.NFS4ERR_DELAY {
		t.Fatalf("expired-lease recovery rule %v", err)
	} else {
		release()
	}
	unlock := srv.lockMutationTargets(mutationTarget{dirID: meta.DirID, name: meta.FileName})
	busy, err := srv.retireInactiveStagingTarget(meta.DirID, meta.FileName)
	unlock()
	if err != nil || !busy {
		t.Fatalf("inactive writer retirement %t %v", busy, err)
	}
	for _, id := range []vfs.InodeID{stagingID, recovered, keyed} {
		if m, ok := srv.stagingStore.GetMeta(id); !ok || m.Retired {
			t.Fatalf("staging %d retired", id)
		}
	}
	if len(srv.opens.ClientStates(f.p)) == 0 {
		t.Fatal("P's open state expired")
	}

	// Settled by the sweeper: D was never applied and confirmed is still
	// P through Sp. Protection ends, and expiry then proceeds normally.
	f.fs.Set(clientstore.WriteD, clientstore.FaultNone, nil)
	srv.runLeaseSweep()
	if srv.protected(f.n) || srv.protected(f.p) {
		t.Fatal("settled confirmation still protected")
	}
	srv.runLeaseSweep()
	if !f.present(t, vfs.InodeID(f.p), clientstore.ExpiredName) {
		t.Fatal("P's expiry did not resume")
	}
}

// An unresolved commit is applied once, whether the sweeper or the retried
// CREATE_SESSION settles it; the retry then replays the cached OK.
func TestConfirmationSettledOnce(t *testing.T) {
	f := newConfirmServer(t)
	f.arm(clientstore.WriteD, clientstore.FaultAppliedFail, nil)
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	f.fs.Set(clientstore.WriteD, clientstore.FaultNone, nil)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		f.srv.settleConfirmations()
	}()
	retry := dial(t, f.conn.RemoteAddr().String())
	defer retry.Close()
	reply, err := sendRawRPC(retry, 9000, procCompound, createSessionBody(f.wireN, 1))
	if err != nil {
		t.Fatal(err)
	}
	if res, err := parseCompoundReply(reply); err != nil || res.Status() != nfs4.NFS4_OK && res.Status() != nfs4.NFS4ERR_DELAY {
		t.Fatalf("concurrent retry %v %v", res.Status(), err)
	}
	wg.Wait()
	status, id := f.rawCreate(t, f.wireN, 1)
	if status != nfs4.NFS4_OK || f.nSessions() != 1 || f.srv.sessions.lookup(id) == nil {
		t.Fatalf("replay %s, %d sessions", nfs4.Nfsstat4Name(status), f.nSessions())
	}
	if f.srv.protected(f.n) || f.pMarker(t) || f.pSequence(t) != nfs4.NFS4ERR_BADSESSION {
		t.Fatal("commit not applied")
	}
}

// Without proof of commit or of an unchanged P, P is invalidated in this
// process only and N's record ends; P's durable state is kept. The same holds
// when P changes between preparation and the revalidation after closing.
func TestConfirmationConservative(t *testing.T) {
	check := func(t *testing.T, f *confirmServer) {
		t.Helper()
		if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("N's record survived: %s", nfs4.Nfsstat4Name(got))
		}
		if f.srv.protected(f.n) || f.srv.protected(f.p) {
			t.Fatal("conservative result kept the protection")
		}
		if got := f.pSequence(t); got != nfs4.NFS4ERR_BADSESSION {
			t.Fatalf("P's SEQUENCE %s", nfs4.Nfsstat4Name(got))
		}
		if !f.pMarker(t) || f.pStaging(t).Retired || f.present(t, vfs.InodeID(f.p), clientstore.ExpiredName) {
			t.Fatal("P's durable state changed")
		}
		// N's creation made its session file before confirming, and
		// removed it with the creation.
		noOrphanSessionFiles(t, f.srv)
	}
	t.Run("observed another incarnation", func(t *testing.T) {
		f := newConfirmServer(t)
		m, err := f.srv.clients.NewIncarnation(f.identity)
		if err != nil {
			t.Fatal(err)
		}
		f.arm(clientstore.WriteD, clientstore.FaultNotApplied, func() { pointConfirmed(t, f, m) })
		if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
		}
		check(t, f)
	})
	t.Run("P changed after preparation", func(t *testing.T) {
		f := newConfirmServer(t)
		f.fs.Mu.Lock()
		// Prepare reads N's record first, Commit second: change P's
		// symlink between them, after P is closed.
		f.fs.LookupHook.Dir, f.fs.LookupHook.Name, f.fs.LookupHook.Skip = vfs.InodeID(f.n), clientstore.ClientRecordName, 1
		f.fs.LookupHook.Fn = func() { pointConfirmed(t, f, vfs.InodeID(f.p)) }
		f.fs.Mu.Unlock()
		if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
		}
		if f.present(t, vfs.InodeID(f.n), f.srv.clients.ConfirmingName) {
			t.Fatal("write A made after the predecessor changed")
		}
		check(t, f)
	})
}

// pointConfirmed rewrites identity/confirmed, through the base VFS, as a new
// symlink to incarnation id.
func pointConfirmed(t *testing.T, f *confirmServer, id vfs.InodeID) {
	t.Helper()
	name, found, err := f.srv.clients.IncarnationName(f.identity, id)
	if err != nil || !found {
		t.Fatalf("incarnation name %v", err)
	}
	if _, err := f.fs.TernVFS.Symlink(f.identity, "t.hook", name); err != nil {
		t.Fatal(err)
	}
	if err := f.fs.TernVFS.Rename(f.identity, "t.hook", f.identity, clientstore.ConfirmedName); err != nil {
		t.Fatal(err)
	}
}

// A first confirmation has no predecessor. A D that never applied observes
// no confirmed pointer, which is conservative: the record ends.
func TestConfirmationFirstWithoutPredecessor(t *testing.T) {
	fs := clientstore.NewConfirmFaultVFS(t)
	srv := identityServer(t, fs)
	wire := identityExchange(t, srv, 1, "first", [8]byte{1}, false).clientID
	// Protection is keyed by incarnation; save it before the record ends.
	incarnation := incarnationForTest(t, srv, wire)
	fs.Set(clientstore.WriteD, clientstore.FaultNotApplied, nil)
	fs.Mu.Lock()
	fs.Armed = true
	fs.Mu.Unlock()
	if got, _ := createResult(t, srv, 1, wire, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	if wireRecords(srv) != 0 || srv.protected(incarnation) {
		t.Fatal("record or protection left")
	}
	// The client starts again and confirms.
	fs.Set(clientstore.WriteD, clientstore.FaultNone, nil)
	again := identityExchange(t, srv, 1, "first", [8]byte{1}, false)
	if got, _ := createResult(t, srv, 1, again.clientID, 1); got != nfs4.NFS4_OK {
		t.Fatalf("second attempt %s", nfs4.Nfsstat4Name(got))
	}
}

// A committed replacement whose purge fails keeps the reboot evidence; the
// next warm EXCHANGE_ID of N retries the purge of exactly P.
func TestConfirmationPurgeRetry(t *testing.T) {
	f := newConfirmServer(t)
	f.fs.Mu.Lock()
	f.fs.FailLeaseRemove = vfs.InodeID(f.p)
	f.fs.Mu.Unlock()
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	if !f.present(t, vfs.InodeID(f.n), clientstore.RebootName) {
		t.Fatal("reboot evidence lost after a failed purge")
	}
	if _, _, status, _ := exchangeID(f.conn, f.xid, "test-client", [8]byte{9}); status != nfs4.NFS4_OK {
		t.Fatalf("warm EXCHANGE_ID %s", nfs4.Nfsstat4Name(status))
	}
	f.xid++
	if f.present(t, vfs.InodeID(f.n), clientstore.RebootName) || f.present(t, vfs.InodeID(f.p), f.srv.clients.LeaseName) {
		t.Fatal("purge not retried")
	}
}

// N and P are protected from before the first write, while the initial
// CREATE_SESSION is still running: its retry and P's admissions wait.
func TestConfirmationProtectedWhileInFlight(t *testing.T) {
	f := newConfirmServer(t)
	entered, release := make(chan struct{}), make(chan struct{})
	defer testutil.CloseSignal(release)
	f.arm(clientstore.WriteD, clientstore.FaultNotApplied, func() {
		close(entered)
		<-release
	})
	first := dial(t, f.conn.RemoteAddr().String())
	defer first.Close()
	result := make(chan uint32, 1)
	go func() {
		reply, err := sendRawRPC(first, 9100, procCompound, createSessionBody(f.wireN, 1))
		if err != nil {
			result <- 0
			return
		}
		res, err := parseCompoundReply(reply)
		if err != nil {
			result <- 0
			return
		}
		result <- res.Status()
	}()
	testutil.AwaitSignal(t, entered, "write D in flight")
	if !f.srv.protected(f.n) || !f.srv.protected(f.p) {
		t.Fatal("not protected while the first write is in flight")
	}
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("retry while in flight %s", nfs4.Nfsstat4Name(got))
	}
	if got := f.pSequence(t); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("P's SEQUENCE while in flight %s", nfs4.Nfsstat4Name(got))
	}
	testutil.CloseSignal(release)
	// D never applied and confirmed is still P through Sp: rolled back.
	if got := testutil.AwaitValue(t, result, "first CREATE_SESSION"); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("first CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	if f.srv.protected(f.n) || f.pSequence(t) != nfs4.NFS4_OK {
		t.Fatal("rollback did not reopen P")
	}
}

// afterPrepare runs fn once, when CommitConfirmation rereads N's record: after
// Prepare and after P is closed, before any write.
func (f *confirmServer) afterPrepare(fn func()) {
	f.fs.Mu.Lock()
	defer f.fs.Mu.Unlock()
	f.fs.LookupHook.Dir, f.fs.LookupHook.Name, f.fs.LookupHook.Skip = vfs.InodeID(f.n), clientstore.ClientRecordName, 1
	f.fs.LookupHook.Fn = fn
}

func (f *confirmServer) setFailLookup(fail bool) {
	f.fs.Mu.Lock()
	f.fs.FailLookup = fail
	f.fs.Mu.Unlock()
}

// A read error before any write is no evidence that P is unchanged: P stays
// closed, N and P protected and N's slot held. Retries keep it so while the
// read fails. Once readable, the pointer decides: unchanged P rolls back
// and reopens; a pointer another process changed is conservative. Durable
// state is kept either way.
func TestConfirmationPreWriteReadError(t *testing.T) {
	for _, changed := range []bool{false, true} {
		name := "unchanged"
		if changed {
			name = "changed by another process"
		}
		t.Run(name, func(t *testing.T) {
			f := newConfirmServer(t)
			m, err := f.srv.clients.NewIncarnation(f.identity)
			if err != nil {
				t.Fatal(err)
			}
			f.afterPrepare(func() { f.setFailLookup(true) })
			if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
				t.Fatalf("CREATE_SESSION with an unreadable pointer %s", nfs4.Nfsstat4Name(got))
			}
			if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
				t.Fatalf("retry while unreadable %s", nfs4.Nfsstat4Name(got))
			}
			f.srv.runLeaseSweep()
			if !f.srv.protected(f.n) || !f.srv.protected(f.p) || f.pSequence(t) != nfs4.NFS4ERR_DELAY {
				t.Fatal("protection or P's closing lost on a read error")
			}
			if changed {
				pointConfirmed(t, f, m)
			}
			f.setFailLookup(false)
			if f.present(t, vfs.InodeID(f.n), f.srv.clients.ConfirmingName) {
				t.Fatal("a write was made")
			}
			if !changed {
				if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
					t.Fatalf("retry after the read recovers %s", nfs4.Nfsstat4Name(got))
				}
				if f.srv.protected(f.p) || f.pSequence(t) != nfs4.NFS4_OK {
					t.Fatal("unchanged P not reopened")
				}
				return
			}
			if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
				t.Fatalf("retry after another process changed P %s", nfs4.Nfsstat4Name(got))
			}
			if f.srv.protected(f.p) || f.pSequence(t) != nfs4.NFS4ERR_BADSESSION {
				t.Fatal("changed P not invalidated locally")
			}
			if !f.pMarker(t) || f.pStaging(t).Retired || f.present(t, vfs.InodeID(f.p), clientstore.ExpiredName) {
				t.Fatal("P's durable state changed")
			}
		})
	}
}

// Another process makes N not pending before the revalidation: nothing is
// written, P through Sp proves P unchanged, so P reopens and the reply is
// DELAY. The retry then finds N stale, and so does every later one.
func TestConfirmationPendingMoved(t *testing.T) {
	f := newConfirmServer(t)
	other, err := clientstore.NewStoreAt(f.fs.TernVFS, f.srv.clients.NFSDirID, f.srv.clients.DirID)
	if err != nil {
		t.Fatal(err)
	}
	f.afterPrepare(func() {
		if _, _, err := other.SetClientID([8]byte{11}, []byte("test-client"), clientstore.ClientOwner{}); err != nil {
			t.Error(err)
		}
	})
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	if f.srv.protected(f.p) || f.pSequence(t) != nfs4.NFS4_OK {
		t.Fatal("unchanged P not reopened")
	}
	for range 2 {
		if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("retry %s", nfs4.Nfsstat4Name(got))
		}
	}
	if identityConfirmed(t, f) != vfs.InodeID(f.p) || !f.pMarker(t) {
		t.Fatal("P changed")
	}
}

// Another process confirms N before the revalidation: committed elsewhere.
// N gets its session and P is replaced locally, but with no B this attempt
// has no authority to purge P.
func TestConfirmationCommittedElsewhere(t *testing.T) {
	f := newConfirmServer(t)
	f.afterPrepare(func() { pointConfirmed(t, f, vfs.InodeID(f.n)) })
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	if f.pSequence(t) != nfs4.NFS4ERR_BADSESSION || f.nSessions() != 1 {
		t.Fatal("commit not applied")
	}
	if !f.present(t, vfs.InodeID(f.p), f.srv.clients.LeaseName) || f.present(t, vfs.InodeID(f.n), clientstore.RebootName) {
		t.Fatal("P purged without B")
	}
}

// A panic while applying a committed result, before N's session is
// published, leaves the confirmation protected and resumable; a retry or the
// sweeper finishes it, with exactly one session. A panic after publication
// (in the purge) keeps OK and leaves the purge to its retry.
func TestConfirmationApplyPanic(t *testing.T) {
	for _, resume := range []string{"retry", "sweeper"} {
		t.Run("before publication/"+resume, func(t *testing.T) {
			f := newConfirmServer(t)
			// P's first removal is expireClientState's open marker.
			f.fs.Mu.Lock()
			f.fs.PanicRemove.Dir = vfs.InodeID(f.p)
			f.fs.Mu.Unlock()
			if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
				t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
			}
			if !f.srv.protected(f.n) || !f.srv.protected(f.p) || f.nSessions() != 0 {
				t.Fatal("interrupted application lost its protection or published")
			}
			if resume == "sweeper" {
				f.srv.runLeaseSweep()
				if f.srv.protected(f.n) {
					t.Fatal("sweeper did not resume")
				}
			}
			status, id := f.rawCreate(t, f.wireN, 1)
			if status != nfs4.NFS4_OK || f.nSessions() != 1 || f.srv.sessions.lookup(id) == nil {
				t.Fatalf("resumed %s with %d sessions", nfs4.Nfsstat4Name(status), f.nSessions())
			}
			if f.srv.protected(f.n) || f.pSequence(t) != nfs4.NFS4ERR_BADSESSION || f.pMarker(t) {
				t.Fatal("commit not fully applied")
			}
			if f.present(t, vfs.InodeID(f.n), clientstore.RebootName) {
				t.Fatal("purge not done")
			}
		})
	}
	t.Run("after publication", func(t *testing.T) {
		f := newConfirmServer(t)
		// Skip the open marker's removal; panic in the purge.
		f.fs.Mu.Lock()
		f.fs.PanicRemove.Dir, f.fs.PanicRemove.Skip = vfs.InodeID(f.p), 1
		f.fs.Mu.Unlock()
		status, id := f.rawCreate(t, f.wireN, 1)
		if status != nfs4.NFS4_OK || f.srv.protected(f.n) || f.nSessions() != 1 {
			t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(status))
		}
		if !f.present(t, vfs.InodeID(f.n), clientstore.RebootName) {
			t.Fatal("interrupted purge lost its evidence")
		}
		if got, replay := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4_OK || replay != id {
			t.Fatalf("replay %s", nfs4.Nfsstat4Name(got))
		}
		if _, _, status, _ := exchangeID(f.conn, f.xid, "test-client", [8]byte{9}); status != nfs4.NFS4_OK {
			t.Fatalf("warm EXCHANGE_ID %s", nfs4.Nfsstat4Name(status))
		}
		f.xid++
		if f.present(t, vfs.InodeID(f.n), clientstore.RebootName) || f.present(t, vfs.InodeID(f.p), f.srv.clients.LeaseName) {
			t.Fatal("purge not retried")
		}
	})
}

// gate blocks a hook until released, and reports entry.
type gate struct{ entered, release chan struct{} }

func newGate() gate { return gate{make(chan struct{}), make(chan struct{})} }
func (g gate) hold() {
	close(g.entered)
	<-g.release
}

// createAsync sends CREATE_SESSION for N on its own connection.
func (f *confirmServer) createAsync(t *testing.T) <-chan uint32 {
	t.Helper()
	return f.createAsyncFor(t, f.wireN)
}

// createAsyncFor sends sequence 1's CREATE_SESSION for wire on its own
// connection.
func (f *confirmServer) createAsyncFor(t *testing.T, wire uint64) <-chan uint32 {
	t.Helper()
	conn := dial(t, f.conn.RemoteAddr().String())
	t.Cleanup(func() { conn.Close() })
	result := make(chan uint32, 1)
	go func() {
		reply, err := sendRawRPC(conn, 9200, procCompound, createSessionBody(wire, 1))
		if err != nil {
			result <- 0
			return
		}
		res, err := parseCompoundReply(reply)
		if err != nil {
			result <- 0
			return
		}
		result <- res.Status()
	}()
	return result
}

// awaitInstalling waits until N's confirmation has installed its entry and
// waits for earlier destructive work.
func (f *confirmServer) awaitInstalling(t *testing.T) {
	t.Helper()
	for deadline := time.Now().Add(testutil.ChannelTimeout); !f.srv.protected(f.n); {
		if time.Now().After(deadline) {
			t.Fatal("confirmation never installed its protection")
		}
		time.Sleep(time.Millisecond)
	}
}

func notYet(t *testing.T, result <-chan uint32, what string) {
	t.Helper()
	time.Sleep(20 * time.Millisecond)
	select {
	case got := <-result:
		t.Fatalf("%s did not wait: %s", what, nfs4.Nfsstat4Name(got))
	default:
	}
}

// pausedCreate starts N's CREATE_SESSION and stops it at registration, after
// Prepare and after closing P, by holding the owner lock; resume lets it
// install its protection.
func (f *confirmServer) pausedCreate(t *testing.T) (result <-chan uint32, resume func()) {
	t.Helper()
	return f.pausedCreateFor(t, f.wireN)
}

// pausedCreateFor is pausedCreate for another record of the same owner.
func (f *confirmServer) pausedCreateFor(t *testing.T, wire uint64) (result <-chan uint32, resume func()) {
	t.Helper()
	unlock := f.srv.sessions.ownerLocks.Lock("test-client")
	var once sync.Once
	resume = func() { once.Do(unlock) }
	t.Cleanup(resume)
	result = f.createAsyncFor(t, wire)
	awaitOwnerWaiter(t, f.srv.sessions, "test-client")
	return result, resume
}

// Protection is ordered against destructive work admitted before it: a
// confirmation installs its entry, refusing new destructive work, then waits
// for each admitted action to finish completely before its first write.
// Each case admits the action while the confirmation is paused before
// registration, blocks the action midway, then lets registration proceed.
func TestProtectionOrdering(t *testing.T) {
	t.Run("client GC", func(t *testing.T) {
		f := newConfirmServer(t)
		result, resume := f.pausedCreate(t)
		g := newGate()
		defer testutil.CloseSignal(g.release)
		f.fs.Mu.Lock()
		f.fs.OnReaddir[f.identity] = g.hold
		f.fs.Mu.Unlock()
		collected := make(chan bool, 1)
		go func() {
			recheck, _ := f.srv.collectClientSafely(vfs.InodeID(f.n))
			collected <- recheck
		}()
		testutil.AwaitSignal(t, g.entered, "GC inside the identity")
		resume()
		f.awaitInstalling(t)
		notYet(t, result, "confirmation behind an admitted GC")
		// New GC of the identity is refused meanwhile.
		if recheck, err := f.srv.collectClientSafely(vfs.InodeID(f.n)); err != nil || !recheck {
			t.Fatalf("second GC %t %v", recheck, err)
		}
		// An EXCHANGE_ID arriving now waits for the owner and then sees
		// the confirmation.
		exchanged := make(chan uint32, 1)
		other := dial(t, f.conn.RemoteAddr().String())
		defer other.Close()
		go func() {
			_, _, status, _ := exchangeID(other, 9400, "test-client", [8]byte{13})
			exchanged <- status
		}()
		notYet(t, result, "confirmation")
		// Another client makes progress.
		unrelated := identityExchange(t, f.srv, 1, "other", [8]byte{1}, false)
		if got, _ := createResult(t, f.srv, 1, unrelated.clientID, 1); got != nfs4.NFS4_OK {
			t.Fatalf("other client %s", nfs4.Nfsstat4Name(got))
		}
		testutil.CloseSignal(g.release)
		if recheck := testutil.AwaitValue(t, collected, "admitted GC"); recheck {
			t.Fatal("admitted GC was cut short")
		}
		// It gets DELAY while the confirmation is protected, or runs after
		// it has finished; it never rewrites N's pending pointer under
		// protection, which would have failed the commit's pending check.
		if got := testutil.AwaitValue(t, exchanged, "EXCHANGE_ID during registration"); got != nfs4.NFS4ERR_DELAY && got != nfs4.NFS4_OK {
			t.Fatalf("EXCHANGE_ID during registration %s", nfs4.Nfsstat4Name(got))
		}
		if got := testutil.AwaitValue(t, result, "confirmation"); got != nfs4.NFS4_OK || identityConfirmed(t, f) != vfs.InodeID(f.n) {
			t.Fatalf("confirmation %s", nfs4.Nfsstat4Name(got))
		}
	})
	t.Run("expired marker", func(t *testing.T) {
		f := newConfirmServer(t)
		result, resume := f.pausedCreate(t)
		f.advance(3 * clientstore.LeaseTime)
		g := newGate()
		defer testutil.CloseSignal(g.release)
		f.fs.Mu.Lock()
		f.fs.CreateHook.Dir, f.fs.CreateHook.Name, f.fs.CreateHook.Fn = vfs.InodeID(f.p), clientstore.ExpiredName, g.hold
		f.fs.Mu.Unlock()
		expired := make(chan struct{})
		go func() { defer close(expired); f.srv.expireClientIfDead(f.p) }()
		testutil.AwaitSignal(t, g.entered, "marker write")
		resume()
		f.awaitInstalling(t)
		notYet(t, result, "confirmation behind an admitted expiry")
		if f.present(t, vfs.InodeID(f.n), f.srv.clients.ConfirmingName) {
			t.Fatal("confirmation wrote while the expiry was running")
		}
		testutil.CloseSignal(g.release)
		testutil.AwaitSignal(t, expired, "expiry")
		// The expiry finished completely before any confirmation write.
		if !f.present(t, vfs.InodeID(f.p), clientstore.ExpiredName) || len(f.srv.opens.ClientStates(f.p)) != 0 {
			t.Fatal("admitted expiry did not complete")
		}
		testutil.AwaitValue(t, result, "confirmation")
	})
	t.Run("staging retirement", func(t *testing.T) {
		f := newConfirmServer(t)
		var sid nfs4.StateID
		copy(sid[:], f.state[4:])
		meta := f.pStaging(t)
		if err := f.srv.clients.RemoveOpen(f.p, sid); err != nil {
			t.Fatal(err)
		}
		result, resume := f.pausedCreate(t)
		g := newGate()
		defer testutil.CloseSignal(g.release)
		f.fs.Mu.Lock()
		f.fs.LookupHook.Dir, f.fs.LookupHook.Name, f.fs.LookupHook.Fn = vfs.InodeID(f.p), clientstore.ActiveOpenName(sid), g.hold
		f.fs.Mu.Unlock()
		retired := make(chan bool, 1)
		go func() {
			unlock := f.srv.lockMutationTargets(mutationTarget{dirID: meta.DirID, name: meta.FileName})
			defer unlock()
			busy, _ := f.srv.retireInactiveStagingTarget(meta.DirID, meta.FileName)
			retired <- busy
		}()
		testutil.AwaitSignal(t, g.entered, "writer inspection")
		resume()
		f.awaitInstalling(t)
		notYet(t, result, "confirmation behind an admitted retirement")
		testutil.CloseSignal(g.release)
		if busy := testutil.AwaitValue(t, retired, "retirement"); busy || !f.pStaging(t).Retired {
			t.Fatal("admitted retirement did not complete")
		}
		testutil.AwaitValue(t, result, "confirmation")
	})
	t.Run("EXCHANGE_ID", func(t *testing.T) {
		f := newConfirmServer(t)
		g := newGate()
		defer testutil.CloseSignal(g.release)
		f.fs.Mu.Lock()
		// The registration reads the identity before rewriting pending.
		f.fs.OnReaddir[f.identity] = g.hold
		f.fs.Mu.Unlock()
		other := dial(t, f.conn.RemoteAddr().String())
		defer other.Close()
		exchanged := make(chan uint64, 1)
		go func() {
			wire, _, status, _ := exchangeID(other, 9300, "test-client", [8]byte{12})
			if status != nfs4.NFS4_OK {
				wire = 0
			}
			exchanged <- wire
		}()
		testutil.AwaitSignal(t, g.entered, "EXCHANGE_ID inside the store")
		result := f.createAsync(t)
		notYet(t, result, "confirmation behind an admitted EXCHANGE_ID")
		if f.srv.protected(f.n) {
			t.Fatal("protection installed while EXCHANGE_ID held the owner")
		}
		testutil.CloseSignal(g.release)
		if wire := testutil.AwaitValue(t, exchanged, "EXCHANGE_ID"); wire == 0 || wire == f.wireN {
			t.Fatalf("EXCHANGE_ID %#x", wire)
		}
		// The new registration superseded N before protection: nothing was
		// written for N, and N's record is gone.
		if got := testutil.AwaitValue(t, result, "confirmation"); got != nfs4.NFS4ERR_DELAY && got != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("superseded confirmation %s", nfs4.Nfsstat4Name(got))
		}
		if f.present(t, vfs.InodeID(f.n), f.srv.clients.ConfirmingName) || identityConfirmed(t, f) != vfs.InodeID(f.p) {
			t.Fatal("superseded N was written")
		}
		if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("superseded record %s", nfs4.Nfsstat4Name(got))
		}
		if f.srv.protected(f.n) || f.srv.protected(f.p) || f.pSequence(t) != nfs4.NFS4_OK {
			t.Fatal("P left closed or protected")
		}
	})
}

// A retry of a held confirmation uses the reservations it already owns: it
// settles and commits with the session-record budget saturated. It reserves
// its connection association before settling: on a full connection it gets
// RESOURCE with nothing settled; on another connection it commits and is
// associated there.
func TestHeldRetryAdmission(t *testing.T) {
	held := func(t *testing.T) *confirmServer {
		t.Helper()
		f := newConfirmServer(t)
		f.arm(clientstore.WriteD, clientstore.FaultAppliedFail, nil)
		if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
			t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
		}
		f.disarm()
		f.fs.Set(clientstore.WriteD, clientstore.FaultNone, nil)
		return f
	}
	t.Run("saturated record budget", func(t *testing.T) {
		f := held(t)
		ss := f.srv.sessions
		ss.mu.Lock()
		ss.recordLimit = ss.records + ss.pendingSessions
		ss.mu.Unlock()
		other := identityExchange(t, f.srv, 1, "other", [8]byte{1}, false)
		if got, _ := createResult(t, f.srv, 1, other.clientID, 1); got != nfs4.NFS4ERR_DELAY {
			t.Fatalf("new CREATE_SESSION at the cap %s", nfs4.Nfsstat4Name(got))
		}
		status, id := f.rawCreate(t, f.wireN, 1)
		if status != nfs4.NFS4_OK || f.srv.sessions.lookup(id) == nil || f.nSessions() != 1 || f.srv.protected(f.n) {
			t.Fatalf("held retry at the cap %s", nfs4.Nfsstat4Name(status))
		}
	})
	t.Run("full connection", func(t *testing.T) {
		f := held(t)
		other := identityExchange(t, f.srv, 1, "other", [8]byte{1}, false)
		full := &rpcConnection{}
		for seq := uint32(1); seq <= maxConnectionSessions; seq++ {
			if got := identityCallOn(t, f.srv, 1, full, createSessionBody(other.clientID, seq)).Status(); got != nfs4.NFS4_OK {
				t.Fatalf("filling the connection %s", nfs4.Nfsstat4Name(got))
			}
		}
		retry := func(conn *rpcConnection) (uint32, SessionID) {
			res := identityCallOn(t, f.srv, 1, conn, createSessionBody(f.wireN, 1))
			if res.Status() != nfs4.NFS4_OK {
				return res.Status(), SessionID{}
			}
			it := res.Resarray()
			it.Next()
			return nfs4.NFS4_OK, readSessionID(it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok().CsrSessionid())
		}
		if got, _ := retry(full); got != nfs4.NFS4ERR_RESOURCE {
			t.Fatalf("retry on a full connection %s", nfs4.Nfsstat4Name(got))
		}
		full.mu.Lock()
		size := len(full.sessions)
		full.mu.Unlock()
		if !f.srv.protected(f.n) || f.nSessions() != 0 || size != maxConnectionSessions {
			t.Fatal("refused retry settled, published or changed the connection")
		}
		fresh := &rpcConnection{}
		status, id := retry(fresh)
		if status != nfs4.NFS4_OK || !fresh.associated(id) || f.nSessions() != 1 || f.srv.protected(f.n) {
			t.Fatalf("retry on another connection %s", nfs4.Nfsstat4Name(status))
		}
		// The connection that has it replays at no cost; the full one
		// still has no room for it.
		if got, replay := retry(fresh); got != nfs4.NFS4_OK || replay != id {
			t.Fatalf("replay on the associated connection %s", nfs4.Nfsstat4Name(got))
		}
		if got, _ := retry(full); got != nfs4.NFS4ERR_RESOURCE {
			t.Fatalf("replay needing a new association %s", nfs4.Nfsstat4Name(got))
		}
	})
}

// replacementHeld reports the server's hint that N's purge is unfinished.
// The durable obligation is N's reboot symlink, checked separately.
func (f *confirmServer) replacementHeld() bool {
	return f.srv.hasPurgePending(f.n)
}

// panicNextRemoveInP arms a panic for the next Remove in P's directory,
// after client GC, which also removes there, has finished.
func (f *confirmServer) panicNextRemoveInP() {
	f.srv.waitForClientGC()
	f.fs.Mu.Lock()
	f.fs.PanicRemove.Dir, f.fs.PanicRemove.Skip = vfs.InodeID(f.p), 0
	f.fs.Mu.Unlock()
}

// A failed purge is retried by N's later CREATE_SESSION and warm
// EXCHANGE_ID. A panic in either retry is contained: the reply stays OK, the
// creation's reservations are released, and the obligation is kept for the
// next retry, which completes it.
func TestPurgeRetryPanic(t *testing.T) {
	f := newConfirmServer(t)
	ss := f.srv.sessions
	f.fs.Mu.Lock()
	f.fs.FailLeaseRemove = vfs.InodeID(f.p)
	f.fs.Mu.Unlock()
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4_OK || !f.replacementHeld() {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}

	// A later CREATE_SESSION retries the purge, which panics.
	f.panicNextRemoveInP()
	status, id := f.rawCreate(t, f.wireN, 2)
	if status != nfs4.NFS4_OK || ss.lookup(id) == nil {
		t.Fatalf("CREATE_SESSION with a panicking purge retry %s", nfs4.Nfsstat4Name(status))
	}
	if a := ss.accounting(); a.PendingSessions != 0 {
		t.Fatalf("reservation leaked %+v", a)
	}
	if !ss.closeForReplacement(f.n) {
		t.Fatal("creation hold leaked")
	}
	ss.reopen(f.n)
	if !f.replacementHeld() || !f.present(t, vfs.InodeID(f.n), clientstore.RebootName) {
		t.Fatal("purge obligation lost")
	}
	if got, replay := f.rawCreate(t, f.wireN, 2); got != nfs4.NFS4_OK || replay != id {
		t.Fatalf("replay %s", nfs4.Nfsstat4Name(got))
	}

	// A warm EXCHANGE_ID retries it, and panics too.
	f.panicNextRemoveInP()
	if wire, _, status, _ := exchangeID(f.conn, f.xid, "test-client", [8]byte{9}); status != nfs4.NFS4_OK || wire != f.wireN {
		t.Fatalf("warm EXCHANGE_ID with a panicking purge retry %s", nfs4.Nfsstat4Name(status))
	}
	f.xid++
	if !f.replacementHeld() || !f.present(t, vfs.InodeID(f.n), clientstore.RebootName) {
		t.Fatal("purge obligation lost")
	}

	// The next warm EXCHANGE_ID completes it.
	if _, _, status, _ := exchangeID(f.conn, f.xid, "test-client", [8]byte{9}); status != nfs4.NFS4_OK {
		t.Fatalf("warm EXCHANGE_ID %s", nfs4.Nfsstat4Name(status))
	}
	f.xid++
	if f.replacementHeld() || f.present(t, vfs.InodeID(f.n), clientstore.RebootName) || f.present(t, vfs.InodeID(f.p), f.srv.clients.LeaseName) {
		t.Fatal("purge not completed")
	}
}

// failPurgeOnce makes the next lease removal in P fail, as a purge of P
// would do.
func (f *confirmServer) failPurgeOnce() {
	f.srv.waitForClientGC()
	f.fs.Mu.Lock()
	f.fs.FailLeaseRemove = vfs.InodeID(f.p)
	f.fs.Mu.Unlock()
}

// commitWithFailedPurge confirms N over P with P's purge failing, and checks
// the durable obligation: N/reboot naming P, with P's lease kept.
func (f *confirmServer) commitWithFailedPurge(t *testing.T) {
	t.Helper()
	f.failPurgeOnce()
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	f.assertPurgePending(t)
}

func (f *confirmServer) assertPurgePending(t *testing.T) {
	t.Helper()
	target, found, err := f.srv.clients.ReadRebootSymlink(vfs.InodeID(f.n))
	if err != nil || !found || target != vfs.InodeID(f.p) || !f.present(t, vfs.InodeID(f.p), f.srv.clients.LeaseName) {
		t.Fatalf("purge obligation lost: reboot %d %t %v", target, found, err)
	}
}

func (f *confirmServer) assertPurged(t *testing.T) {
	t.Helper()
	if f.present(t, vfs.InodeID(f.n), clientstore.RebootName) || f.present(t, vfs.InodeID(f.p), f.srv.clients.LeaseName) {
		t.Fatal("purge not completed")
	}
}

// destroyN destroys N's sessions and wire record, as a client would before
// going away.
func (f *confirmServer) destroyN(t *testing.T) {
	t.Helper()
	ss := f.srv.sessions
	ss.mu.Lock()
	var ids []SessionID
	for id, sess := range ss.sessions {
		if sess.clientID == f.n {
			ids = append(ids, id)
		}
	}
	ss.mu.Unlock()
	for _, id := range ids {
		ss.destroySession(id)
	}
	if got := f.rawStatus(t, buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_DestroyClientid().SetDcaClientid(f.wireN)
	})); got != nfs4.NFS4_OK {
		t.Fatalf("DESTROY_CLIENTID %s", nfs4.Nfsstat4Name(got))
	}
}

// adoptN runs a cold EXCHANGE_ID of N's owner, which adopts the still
// confirmed N without any write.
func (f *confirmServer) adoptN(t *testing.T, conn net.Conn) uint64 {
	t.Helper()
	wire, seq, status, err := exchangeID(conn, f.xid, "test-client", [8]byte{9})
	f.xid++
	if err != nil || status != nfs4.NFS4_OK || seq != 1 || wire == f.wireN {
		t.Fatalf("cold EXCHANGE_ID %#x %d %s %v", wire, seq, nfs4.Nfsstat4Name(status), err)
	}
	return wire
}

// A committed replacement purge that failed is owned by its durable
// evidence, N/reboot. It outlives N's sessions and wire record, cold
// adoption (itself read-only) and a restart, and the next successful
// CREATE_SESSION of N, or a warm EXCHANGE_ID while the hint is held,
// completes it.
func TestPurgeOwnership(t *testing.T) {
	t.Run("after wire retirement", func(t *testing.T) {
		f := newConfirmServer(t)
		f.commitWithFailedPurge(t)
		f.destroyN(t)
		if !f.replacementHeld() {
			t.Fatal("server hint lost with the wire record")
		}
		wire := f.adoptN(t, f.conn)
		f.assertPurgePending(t) // cold adoption wrote nothing
		if got, _ := f.rawCreate(t, wire, 1); got != nfs4.NFS4_OK {
			t.Fatalf("CREATE_SESSION after adoption %s", nfs4.Nfsstat4Name(got))
		}
		f.assertPurged(t)
		if f.replacementHeld() {
			t.Fatal("hint kept after the purge")
		}
	})
	t.Run("warm EXCHANGE_ID after re-adoption", func(t *testing.T) {
		f := newConfirmServer(t)
		f.commitWithFailedPurge(t)
		f.destroyN(t)
		wire := f.adoptN(t, f.conn)
		f.failPurgeOnce()
		if got, _ := f.rawCreate(t, wire, 1); got != nfs4.NFS4_OK {
			t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
		}
		f.assertPurgePending(t)
		if got, _, status, _ := exchangeID(f.conn, f.xid, "test-client", [8]byte{9}); status != nfs4.NFS4_OK || got != wire {
			t.Fatalf("warm EXCHANGE_ID %s", nfs4.Nfsstat4Name(status))
		}
		f.xid++
		f.assertPurged(t)
	})
	t.Run("restart", func(t *testing.T) {
		f := newConfirmServer(t)
		f.commitWithFailedPurge(t)
		// A new process over the same store: no hint, only the marker.
		staging, err := staging.NewLocalStore(t.TempDir(), nil)
		if err != nil {
			t.Fatal(err)
		}
		restarted, err := NewServer(f.fs, staging, nil)
		if err != nil {
			t.Fatal(err)
		}
		addr, stop := serveTestServer(t, restarted)
		defer stop()
		conn := dial(t, addr)
		defer conn.Close()
		wire := f.adoptN(t, conn)
		f.assertPurgePending(t)
		reply, err := sendRawRPC(conn, f.xid, procCompound, createSessionBody(wire, 1))
		f.xid++
		if err != nil {
			t.Fatal(err)
		}
		if res, err := parseCompoundReply(reply); err != nil || res.Status() != nfs4.NFS4_OK {
			t.Fatalf("CREATE_SESSION after restart %v %v", res.Status(), err)
		}
		f.assertPurged(t)
	})
	t.Run("marker changed before the resumed purge", func(t *testing.T) {
		f := newConfirmServer(t)
		f.commitWithFailedPurge(t)
		// Another confirmation of N, in another process, recorded M as its
		// predecessor: the evidence now names M, which is what is purged.
		m, err := f.srv.clients.NewIncarnation(f.identity)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.fs.TernVFS.CreateFile(m, clientstore.ActiveOpenName(nfs4.StateID{2}), nil); err != nil {
			t.Fatal(err)
		}
		name, _, _ := f.srv.clients.IncarnationName(f.identity, m)
		if _, err := f.fs.TernVFS.Symlink(vfs.InodeID(f.n), "t.hook", "../"+name); err != nil {
			t.Fatal(err)
		}
		if err := f.fs.TernVFS.Rename(vfs.InodeID(f.n), "t.hook", vfs.InodeID(f.n), clientstore.RebootName); err != nil {
			t.Fatal(err)
		}
		if got, _ := f.rawCreate(t, f.wireN, 2); got != nfs4.NFS4_OK {
			t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
		}
		if f.present(t, m, clientstore.ActiveOpenName(nfs4.StateID{2})) || f.present(t, vfs.InodeID(f.n), clientstore.RebootName) {
			t.Fatal("the incarnation the evidence names was not purged")
		}
		if !f.present(t, vfs.InodeID(f.p), f.srv.clients.LeaseName) {
			t.Fatal("P purged without evidence naming it")
		}
	})
	t.Run("hint capacity", func(t *testing.T) {
		f := newConfirmServer(t)
		f.srv.confirmMu.Lock()
		f.srv.purgeLimit = 1
		f.srv.purgePending[12345] = struct{}{}
		f.srv.confirmMu.Unlock()
		f.commitWithFailedPurge(t)
		if f.replacementHeld() {
			t.Fatal("hint stored beyond its limit")
		}
		// Without the hint, warm EXCHANGE_ID makes no store call...
		if _, _, status, _ := exchangeID(f.conn, f.xid, "test-client", [8]byte{9}); status != nfs4.NFS4_OK {
			t.Fatalf("warm EXCHANGE_ID %s", nfs4.Nfsstat4Name(status))
		}
		f.xid++
		f.assertPurgePending(t)
		// ...and the next CREATE_SESSION completes the purge from the evidence.
		if got, _ := f.rawCreate(t, f.wireN, 2); got != nfs4.NFS4_OK {
			t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
		}
		f.assertPurged(t)
		f.srv.confirmMu.Lock()
		size := len(f.srv.purgePending)
		f.srv.confirmMu.Unlock()
		if size != 1 {
			t.Fatalf("%d hints", size)
		}
	})
	t.Run("repeated failures", func(t *testing.T) {
		f := newConfirmServer(t)
		f.commitWithFailedPurge(t)
		for seq := uint32(2); seq <= 3; seq++ {
			f.failPurgeOnce()
			if got, _ := f.rawCreate(t, f.wireN, seq); got != nfs4.NFS4_OK {
				t.Fatalf("CREATE_SESSION %d %s", seq, nfs4.Nfsstat4Name(got))
			}
			f.assertPurgePending(t)
			if !f.replacementHeld() {
				t.Fatal("hint dropped by a failed retry")
			}
		}
		if got, _ := f.rawCreate(t, f.wireN, 4); got != nfs4.NFS4_OK {
			t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
		}
		f.assertPurged(t)
	})
}

// rebootN registers R, the same owner rebooted again: a pending incarnation
// whose confirmation would replace N.
func (f *confirmServer) rebootN(t *testing.T) (wire, r uint64) {
	t.Helper()
	wire, seq, status, err := exchangeID(f.conn, f.xid, "test-client", [8]byte{10})
	f.xid++
	if err != nil || status != nfs4.NFS4_OK || seq != 1 {
		t.Fatalf("EXCHANGE_ID for R %s %v", nfs4.Nfsstat4Name(status), err)
	}
	f.srv.waitForClientGC()
	return wire, incarnationForTest(t, f.srv, wire)
}

func (f *confirmServer) destructionHolds() int {
	f.srv.confirmMu.Lock()
	defer f.srv.confirmMu.Unlock()
	return len(f.srv.destroying) + len(f.srv.destroyingIdentity)
}

// A resumed replacement purge is ordered with confirmation protection in
// this process. It holds destruction holds on N and on the target it
// selected, P, through the purge and the reboot edge's removal: a protected
// N or P defers it with nothing removed, and a confirmation registering
// while it runs waits for it to finish before its first write. The holds
// are released on error and panic.
func TestPurgeProtectionOrdering(t *testing.T) {
	t.Run("protection first", func(t *testing.T) {
		f := newConfirmServer(t)
		f.commitWithFailedPurge(t)
		wireR, _ := f.rebootN(t)
		// R's confirmation over N stays unresolved: N is protected.
		f.arm(clientstore.WriteD, clientstore.FaultFenceErr, nil)
		if got, _ := f.rawCreate(t, wireR, 1); got != nfs4.NFS4ERR_DELAY || !f.srv.protected(f.n) {
			t.Fatalf("R's CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
		}
		f.disarm()
		f.srv.resumeReplacementPurge(f.n)
		f.assertPurgePending(t)
		if !f.replacementHeld() || f.destructionHolds() != 0 {
			t.Fatal("deferred purge dropped its hint or kept a hold")
		}
		// R rolls back; N is no longer protected and the purge completes.
		f.fs.Set(clientstore.WriteD, clientstore.FaultNone, nil)
		f.srv.settleConfirmations()
		if f.srv.protected(f.n) || identityConfirmed(t, f) != vfs.InodeID(f.n) {
			t.Fatal("R did not roll back")
		}
		f.srv.resumeReplacementPurge(f.n)
		f.assertPurged(t)
		if f.replacementHeld() {
			t.Fatal("hint kept after the purge")
		}
	})
	t.Run("protected target", func(t *testing.T) {
		f := newConfirmServer(t)
		f.commitWithFailedPurge(t)
		// The incarnation the evidence names is protected by a confirmation
		// here, whatever its durable history.
		f.srv.confirmMu.Lock()
		f.srv.confirming[f.p] = &confirmation{}
		f.srv.confirmMu.Unlock()
		f.srv.resumeReplacementPurge(f.n)
		f.assertPurgePending(t)
		if !f.replacementHeld() || f.destructionHolds() != 0 {
			t.Fatal("deferred purge dropped its hint or kept a hold")
		}
		f.srv.confirmMu.Lock()
		delete(f.srv.confirming, f.p)
		f.srv.confirmMu.Unlock()
		f.srv.resumeReplacementPurge(f.n)
		f.assertPurged(t)
	})
	t.Run("admitted purge first", func(t *testing.T) {
		f := newConfirmServer(t)
		f.commitWithFailedPurge(t)
		wireR, r := f.rebootN(t)
		// R's confirmation has prepared and closed N, and waits to register.
		result, resume := f.pausedCreateFor(t, wireR)
		g := newGate()
		defer testutil.CloseSignal(g.release)
		f.fs.Mu.Lock()
		f.fs.OnReaddir[vfs.InodeID(f.p)] = g.hold
		f.fs.Mu.Unlock()
		purged := make(chan struct{})
		go func() { defer close(purged); f.srv.resumeReplacementPurge(f.n) }()
		testutil.AwaitSignal(t, g.entered, "purge of P")
		resume()
		f.awaitInstalling(t)
		notYet(t, result, "confirmation behind an admitted purge")
		// Protection is installed but not complete: registration waits for
		// the purge's holds on N and P before attaching to R's record.
		f.srv.confirmMu.Lock()
		holdN, holdP := f.srv.destroying[f.n], f.srv.destroying[f.p]
		f.srv.confirmMu.Unlock()
		f.srv.sessions.mu.Lock()
		registered := f.srv.sessions.wireRecordLocked(wireR).confirming != nil
		f.srv.sessions.mu.Unlock()
		if holdN != 1 || holdP != 1 || registered {
			t.Fatalf("registration completed past the admitted purge: holds %d %d, registered %t", holdN, holdP, registered)
		}
		if f.present(t, vfs.InodeID(r), f.srv.clients.ConfirmingName) || identityConfirmed(t, f) != vfs.InodeID(f.n) {
			t.Fatal("confirmation wrote while the purge was running")
		}
		testutil.CloseSignal(g.release)
		testutil.AwaitSignal(t, purged, "purge")
		// The admitted purge completed: P's state and N's reboot edge.
		f.assertPurged(t)
		if f.replacementHeld() {
			t.Fatal("hint kept after the purge")
		}
		if got := testutil.AwaitValue(t, result, "R's confirmation"); got != nfs4.NFS4_OK || identityConfirmed(t, f) != vfs.InodeID(r) {
			t.Fatalf("R's confirmation %s", nfs4.Nfsstat4Name(got))
		}
	})
	t.Run("error and panic release the holds", func(t *testing.T) {
		f := newConfirmServer(t)
		f.commitWithFailedPurge(t)
		f.failPurgeOnce()
		f.srv.resumeReplacementPurge(f.n)
		f.assertPurgePending(t)
		if !f.replacementHeld() || f.destructionHolds() != 0 {
			t.Fatal("failed purge dropped its hint or kept a hold")
		}
		f.panicNextRemoveInP()
		f.srv.resumeReplacementPurge(f.n)
		if !f.replacementHeld() || !f.present(t, vfs.InodeID(f.n), clientstore.RebootName) || f.destructionHolds() != 0 {
			t.Fatal("panicking purge dropped its obligation or kept a hold")
		}
		// No hold is left to wait for: R's confirmation registers and commits.
		wireR, r := f.rebootN(t)
		if got := testutil.AwaitValue(t, f.createAsyncFor(t, wireR), "R's confirmation"); got != nfs4.NFS4_OK || identityConfirmed(t, f) != vfs.InodeID(r) {
			t.Fatalf("R's confirmation %s", nfs4.Nfsstat4Name(got))
		}
	})
}

// recoveredStagingTarget owns its destruction holds until it hands them
// over: a panic after taking one releases it, and a later confirmation
// still registers.
func TestRecoveryHoldReleasedOnPanic(t *testing.T) {
	f := newConfirmServer(t)
	srv := f.srv
	keyed, cookie, err := f.fs.TernVFS.ConstructFile(f.fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	key, err := srv.clients.StagingRecoveryKey(f.n)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.stagingStore.Create(keyed, staging.Meta{DirID: f.fs.RootID(), FileName: "kfile",
		ClientID: f.p, NFSStateID: srv.opens.NewStateID(), TernCookie: cookie,
		OwnerKnown: true, OpenOwner: "owner", RecoveryKey: key}); err != nil {
		t.Fatal(err)
	}
	srv.waitForClientGC()
	// P's quiet period would answer before any read.
	endQuietForTest(srv)
	// decideExpired(P), after the hold, reads the identity's confirmed
	// pointer: panic there.
	f.fs.Mu.Lock()
	f.fs.LookupHook.Dir, f.fs.LookupHook.Name, f.fs.LookupHook.Skip = f.identity, clientstore.ConfirmedName, 0
	f.fs.LookupHook.Fn = func() { panic("injected panic after the recovery hold") }
	f.fs.Mu.Unlock()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("injected panic did not happen")
			}
		}()
		srv.recoveredStagingTarget(f.fs.RootID(), "kfile", f.n, "owner", true)
	}()
	srv.confirmMu.Lock()
	residual := len(srv.destroying) + len(srv.destroyingIdentity)
	srv.confirmMu.Unlock()
	if residual != 0 {
		t.Fatalf("%d destruction holds left after the panic", residual)
	}
	result := f.createAsync(t)
	if got := testutil.AwaitValue(t, result, "confirmation after the panic"); got != nfs4.NFS4_OK {
		t.Fatalf("confirmation %s", nfs4.Nfsstat4Name(got))
	}
}

// A rename that applies and then panics leaves the confirmation unresolved
// with its evidence; the retry settles it as committed.
func TestConfirmationPanicRetained(t *testing.T) {
	f := newConfirmServer(t)
	f.arm(clientstore.WriteD, clientstore.FaultAppliedPanic, nil)
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("CREATE_SESSION after a panic %s", nfs4.Nfsstat4Name(got))
	}
	if !f.srv.protected(f.n) {
		t.Fatal("panic dropped the protection")
	}
	f.fs.Set(clientstore.WriteD, clientstore.FaultNone, nil)
	if got, _ := f.rawCreate(t, f.wireN, 1); got != nfs4.NFS4_OK {
		t.Fatalf("retry %s", nfs4.Nfsstat4Name(got))
	}
	if f.nSessions() != 1 || f.pSequence(t) != nfs4.NFS4ERR_BADSESSION {
		t.Fatal("commit not applied once")
	}
	sessionFilesMatchSessions(t, f.srv)
}
