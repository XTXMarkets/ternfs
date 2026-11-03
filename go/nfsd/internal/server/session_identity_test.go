// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientrecords"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
	"reflect"
)

func authSysCredForTest(uid uint32) []byte {
	body := binary.BigEndian.AppendUint32(nil, 1) // stamp
	body = binary.BigEndian.AppendUint32(body, 0) // machine name
	body = binary.BigEndian.AppendUint32(body, uid)
	body = binary.BigEndian.AppendUint32(body, uid)
	return binary.BigEndian.AppendUint32(body, 0) // groups
}

// identityCall runs one sessionless compound as AUTH_SYS uid.
func identityCall(t *testing.T, srv *Server, uid uint32, body []byte) nfs4.COMPOUND4res {
	t.Helper()
	req, err := parseRPCCall(rpcCallForTest(1, procCompound, body))
	if err != nil {
		t.Fatal(err)
	}
	req.credFlavor, req.credBody = nfs4.AUTH_SYS, authSysCredForTest(uid)
	reply := srv.safeHandleCompound(req, "identity-test")
	res, err := parseCompoundReply(append([]byte(nil), reply...))
	req.releaseReply()
	if err != nil {
		t.Fatal(err)
	}
	return res
}

type exchangeResult struct {
	status, seq uint32
	clientID    uint64
	confirmed   bool
}

func identityExchange(t *testing.T, srv *Server, uid uint32, owner string, verifier [8]byte, update bool) exchangeResult {
	t.Helper()
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		ew := w.AppendArgarray_ExchangeId()
		cw := ew.StartEiaClientowner()
		setVerifier(cw.CoVerifier(), verifier)
		ew.Resume(cw.SetCoOwnerid([]byte(owner)).Finish())
		flags := uint32(0)
		if update {
			flags = nfs4.EXCHGID4_FLAG_UPD_CONFIRMED_REC_A
		}
		ew.SetEiaFlags(flags)
		ew.SetEiaStateProtect_None()
		w.Resume(ew.Finish())
	})
	res := identityCall(t, srv, uid, body)
	if res.Status() != nfs4.NFS4_OK {
		return exchangeResult{status: res.Status()}
	}
	it := res.Resarray()
	it.Next()
	ok := it.Resarray().Value().AsEXCHANGEID4resEntry().Value().AsEXCHANGEID4resok()
	return exchangeResult{status: nfs4.NFS4_OK, clientID: ok.EirClientid(), seq: ok.EirSequenceid(),
		confirmed: ok.EirFlags()&nfs4.EXCHGID4_FLAG_CONFIRMED_R != 0}
}

func identityCreate(t *testing.T, srv *Server, uid uint32, clientID uint64, seq uint32) uint32 {
	t.Helper()
	return identityCall(t, srv, uid, createSessionBody(clientID, seq)).Status()
}

func identityDestroy(t *testing.T, srv *Server, clientID uint64) uint32 {
	t.Helper()
	return identityCall(t, srv, 0, buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_DestroyClientid().SetDcaClientid(clientID)
	})).Status()
}

// identityServer is a server on fs with a staging directory of its own.
func identityServer(t *testing.T, fs vfs.TernVFS) *Server {
	t.Helper()
	return identityServerAt(t, fs, t.TempDir())
}

// identityServerAt is a server on fs and the staging directory dir: a
// second one on the same directory is the same server started again.
func identityServerAt(t *testing.T, fs vfs.TernVFS, dir string) *Server {
	t.Helper()
	store, err := staging.NewLocalStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.CloseAll)
	srv, err := NewServer(fs, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	useTestWireEpoch(srv)
	return srv
}

// recordsForTest lists the server's client records.
func recordsForTest(srv *Server) []clientrecords.Record { return srv.records.List() }

func wireRecords(srv *Server) int {
	srv.sessions.mu.Lock()
	defer srv.sessions.mu.Unlock()
	return len(srv.sessions.clients)
}

// The EXCHANGE_ID case table, RFC 8881 section 18.35.5, over wire records.
func TestExchangeIDCases(t *testing.T) {
	srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	epoch := uint64(srv.sessions.wireIDs.epoch)

	// Case 7: an update needs a confirmed record.
	if got := identityExchange(t, srv, 1, "client", [8]byte{1}, true); got.status != nfs4.NFS4ERR_NOENT {
		t.Fatalf("update without record %s", nfs4.Nfsstat4Name(got.status))
	}
	// Case 1: a fresh wire clientid, not confirmed, creation sequence 1.
	first := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	if first.status != nfs4.NFS4_OK || first.confirmed || first.seq != 1 || first.clientID>>32 != epoch {
		t.Fatalf("cold EXCHANGE_ID %+v", first)
	}
	incarnation := incarnationForTest(t, srv, first.clientID)
	if incarnation == first.clientID {
		t.Fatal("wire clientid is the incarnation")
	}
	// Case 4: an unconfirmed record is replaced, and its clientid retired.
	second := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	if second.status != nfs4.NFS4_OK || second.clientID == first.clientID || second.confirmed || second.seq != 1 {
		t.Fatalf("EXCHANGE_ID over unconfirmed record %+v", second)
	}
	if got := identityCreate(t, srv, 1, first.clientID, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION on superseded clientid %s", nfs4.Nfsstat4Name(got))
	}
	if got := identityDestroy(t, srv, first.clientID); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("DESTROY_CLIENTID on superseded clientid %s", nfs4.Nfsstat4Name(got))
	}
	if wireRecords(srv) != 1 {
		t.Fatalf("%d wire records", wireRecords(srv))
	}
	for seq := uint32(1); seq <= 3; seq++ {
		if got := identityCreate(t, srv, 1, second.clientID, seq); got != nfs4.NFS4_OK {
			t.Fatalf("CREATE_SESSION %d %s", seq, nfs4.Nfsstat4Name(got))
		}
	}
	confirmedIncarnation := incarnationForTest(t, srv, second.clientID)

	// Case 2: same record and sequence, CONFIRMED_R, nothing written.
	before := recordsForTest(srv)
	warm := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	if warm.status != nfs4.NFS4_OK || warm.clientID != second.clientID || !warm.confirmed || warm.seq != 4 {
		t.Fatalf("warm EXCHANGE_ID %+v", warm)
	}
	if !reflect.DeepEqual(recordsForTest(srv), before) {
		t.Fatal("warm EXCHANGE_ID wrote a client record")
	}
	// Case 6 returns the record unchanged; cases 8 and 9 refuse.
	if got := identityExchange(t, srv, 1, "client", [8]byte{1}, true); got.status != nfs4.NFS4_OK ||
		got.clientID != second.clientID || !got.confirmed || got.seq != 4 {
		t.Fatalf("update with the same verifier %+v", got)
	}
	if got := identityExchange(t, srv, 1, "client", [8]byte{2}, true); got.status != nfs4.NFS4ERR_NOT_SAME {
		t.Fatalf("update with a new verifier %s", nfs4.Nfsstat4Name(got.status))
	}
	if got := identityExchange(t, srv, 2, "client", [8]byte{1}, true); got.status != nfs4.NFS4ERR_PERM {
		t.Fatalf("update by another principal %s", nfs4.Nfsstat4Name(got.status))
	}

	// Case 5: a rebooted client gets a new record and a new incarnation; the
	// confirmed record stays until confirmation replaces it.
	reboot := identityExchange(t, srv, 1, "client", [8]byte{3}, false)
	if reboot.status != nfs4.NFS4_OK || reboot.clientID == second.clientID || reboot.confirmed || reboot.seq != 1 {
		t.Fatalf("rebooted client %+v", reboot)
	}
	if incarnationForTest(t, srv, reboot.clientID) == confirmedIncarnation {
		t.Fatal("reboot adopted the old incarnation")
	}
	if got := identityExchange(t, srv, 1, "client", [8]byte{1}, false); got.clientID != second.clientID {
		t.Fatalf("confirmed record lost before confirmation %+v", got)
	}

	// Clientids never issued or from another epoch are stale.
	for _, id := range []uint64{epoch<<32 | 0xffff, (epoch+1)<<32 | 1, confirmedIncarnation} {
		if got := identityCreate(t, srv, 1, id, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("CREATE_SESSION %#x %s", id, nfs4.Nfsstat4Name(got))
		}
		if got := identityDestroy(t, srv, id); got != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("DESTROY_CLIENTID %#x %s", id, nfs4.Nfsstat4Name(got))
		}
	}
	// DESTROY_CLIENTID retires the record; its clientid is then stale.
	if got := identityDestroy(t, srv, reboot.clientID); got != nfs4.NFS4_OK {
		t.Fatalf("DESTROY_CLIENTID %s", nfs4.Nfsstat4Name(got))
	}
	if got := identityCreate(t, srv, 1, reboot.clientID, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION after DESTROY_CLIENTID %s", nfs4.Nfsstat4Name(got))
	}
	// Destroying the confirmed client removes its client record too, once
	// its sessions are gone.
	srv.retireIncarnationSessions(confirmedIncarnation)
	if got := identityDestroy(t, srv, second.clientID); got != nfs4.NFS4_OK {
		t.Fatalf("DESTROY_CLIENTID of the confirmed client %s", nfs4.Nfsstat4Name(got))
	}
	if recs := recordsForTest(srv); len(recs) != 0 {
		t.Fatalf("client records after DESTROY_CLIENTID: %+v", recs)
	}
}

// Each process issues its own clientids for the same incarnation. After a
// restart, a cold EXCHANGE_ID adopts the incarnation its client record
// confirms without writing, and no process accepts another's clientid.
func TestExchangeIDProcessesKeepIndependentClientIDs(t *testing.T) {
	local, dir := vfs.NewLocalTernVFS(t.TempDir()), t.TempDir()
	first := identityServerAt(t, local, dir)
	a := identityExchange(t, first, 1, "client", [8]byte{1}, false)
	if a.status != nfs4.NFS4_OK || identityCreate(t, first, 1, a.clientID, 1) != nfs4.NFS4_OK {
		t.Fatalf("first process registration %+v", a)
	}
	incarnation := incarnationForTest(t, first, a.clientID)

	second := identityServerAt(t, local, dir)
	before := recordsForTest(second)
	b := identityExchange(t, second, 1, "client", [8]byte{1}, false)
	if b.status != nfs4.NFS4_OK || b.confirmed || b.seq != 1 || b.clientID == a.clientID ||
		b.clientID>>32 == a.clientID>>32 {
		t.Fatalf("second process EXCHANGE_ID %+v, first %#x", b, a.clientID)
	}
	if got := incarnationForTest(t, second, b.clientID); got != incarnation {
		t.Fatalf("second process bound incarnation %d, want %d", got, incarnation)
	}
	if !reflect.DeepEqual(recordsForTest(second), before) {
		t.Fatal("adoption wrote a client record")
	}
	for _, check := range []struct {
		srv *Server
		id  uint64
	}{{second, a.clientID}, {first, b.clientID}} {
		if got := identityCreate(t, check.srv, 1, check.id, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("CREATE_SESSION with another process's clientid %s", nfs4.Nfsstat4Name(got))
		}
		if got := identityDestroy(t, check.srv, check.id); got != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("DESTROY_CLIENTID with another process's clientid %s", nfs4.Nfsstat4Name(got))
		}
	}
	// The adopted record confirms by checking the client record.
	if got := identityCreate(t, second, 1, b.clientID, 1); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION on adopted record %s", nfs4.Nfsstat4Name(got))
	}
	if got := identityExchange(t, second, 1, "client", [8]byte{1}, false); !got.confirmed || got.clientID != b.clientID || got.seq != 2 {
		t.Fatalf("warm EXCHANGE_ID after adoption %+v", got)
	}
}

// Wire records are bounded, the allocator never wraps, and a record ends
// with its client.
func TestExchangeIDBoundsAndRetirement(t *testing.T) {
	srv := identityServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	// Client GC reads the clock from its own goroutine.
	var clock atomic.Int64
	setNow := func(t time.Time) { clock.Store(t.UnixNano()) }
	created := time.Now()
	setNow(created)
	srv.SetClock(func() time.Time { return time.Unix(0, clock.Load()) })
	confirmed := identityExchange(t, srv, 1, "confirmed", [8]byte{1}, false)
	if identityCreate(t, srv, 1, confirmed.clientID, 1) != nfs4.NFS4_OK {
		t.Fatal("registration")
	}

	// The cap counts every live record. Only records that waited a lease
	// interval for CREATE_SESSION are reclaimed.
	srv.sessions.wireLimit = 2
	setNow(created.Add(-leaseTime / 2))
	waiting := identityExchange(t, srv, 1, "waiting", [8]byte{1}, false)
	setNow(created)
	if got := identityExchange(t, srv, 1, "new", [8]byte{1}, false); got.status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("EXCHANGE_ID at the cap %s", nfs4.Nfsstat4Name(got.status))
	}
	setNow(created.Add(leaseTime / 2))
	admitted := identityExchange(t, srv, 1, "new", [8]byte{1}, false)
	if admitted.status != nfs4.NFS4_OK || wireRecords(srv) != 2 {
		t.Fatalf("EXCHANGE_ID after reclamation %+v, %d records", admitted, wireRecords(srv))
	}
	if got := identityCreate(t, srv, 1, waiting.clientID, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION on reclaimed record %s", nfs4.Nfsstat4Name(got))
	}
	if got := identityCreate(t, srv, 1, confirmed.clientID, 2); got != nfs4.NFS4_OK {
		t.Fatalf("confirmed record reclaimed: %s", nfs4.Nfsstat4Name(got))
	}
	srv.sessions.wireLimit = maxSessionRecords

	// Exhaustion: no new record, but existing ones keep working.
	srv.sessions.wireIDs.mu.Lock()
	srv.sessions.wireIDs.last = maxWireCounter
	srv.sessions.wireIDs.mu.Unlock()
	if got := identityExchange(t, srv, 1, "late", [8]byte{1}, false); got.status != nfs4.NFS4ERR_SERVERFAULT {
		t.Fatalf("EXCHANGE_ID after exhaustion %s", nfs4.Nfsstat4Name(got.status))
	}
	if got := identityExchange(t, srv, 1, "confirmed", [8]byte{1}, false); got.status != nfs4.NFS4_OK ||
		got.clientID != confirmed.clientID || got.seq != 3 {
		t.Fatalf("warm EXCHANGE_ID after exhaustion %+v", got)
	}
	if got := identityCreate(t, srv, 1, admitted.clientID, 1); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION after exhaustion %s", nfs4.Nfsstat4Name(got))
	}
	srv.sessions.wireIDs.mu.Lock()
	srv.sessions.wireIDs.last = 100
	srv.sessions.wireIDs.mu.Unlock()

	// Expiry ends the record: the client registers again as a new client.
	setNow(created.Add(leaseTime/2 + 3*leaseTime))
	srv.runLeaseSweep()
	if got := identityCreate(t, srv, 1, confirmed.clientID, 3); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION on expired client %s", nfs4.Nfsstat4Name(got))
	}
	again := identityExchange(t, srv, 1, "confirmed", [8]byte{1}, false)
	if again.status != nfs4.NFS4_OK || again.clientID == confirmed.clientID || again.confirmed || again.seq != 1 {
		t.Fatalf("EXCHANGE_ID after expiry %+v", again)
	}
}

// A DESTROY_CLIENTID of one wire record never destroys a newer record of
// the same incarnation.
func TestDestroyClientIDRetiresOnlyItsRecord(t *testing.T) {
	local, dir := vfs.NewLocalTernVFS(t.TempDir()), t.TempDir()
	first := identityServerAt(t, local, dir)
	p := identityExchange(t, first, 1, "client", [8]byte{1}, false)
	if identityCreate(t, first, 1, p.clientID, 1) != nfs4.NFS4_OK {
		t.Fatal("registration")
	}
	srv := identityServerAt(t, local, dir)
	ss := srv.sessions
	// A: an unconfirmed cold adoption of P, with no sessions or opens.
	a := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	ss.mu.Lock()
	recordA := ss.wireRecordLocked(a.clientID)
	ss.mu.Unlock()
	// B: a second adoption supersedes A.
	b := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	if b.status != nfs4.NFS4_OK || b.clientID == a.clientID || incarnationForTest(t, srv, b.clientID) != recordA.incarnation {
		t.Fatalf("second adoption %+v", b)
	}
	if got := identityDestroy(t, srv, a.clientID); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("DESTROY_CLIENTID of a superseded record %s", nfs4.Nfsstat4Name(got))
	}
	// The same holds for a destruction already past its checks.
	if got := ss.destroyClientRecord(recordA); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("stale record destruction %s", nfs4.Nfsstat4Name(got))
	}
	if wireRecords(srv) != 1 || len(recordsForTest(srv)) != 1 {
		t.Fatalf("%d wire records, client records %+v", wireRecords(srv), recordsForTest(srv))
	}
	if got := identityCreate(t, srv, 1, b.clientID, 1); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION on the newer record %s", nfs4.Nfsstat4Name(got))
	}
}

// holdWriteStateForTest gives the incarnation a staged writer, the write
// state that keeps its identity from another principal.
func holdWriteStateForTest(t *testing.T, srv *Server, fs vfs.TernVFS, incarnation uint64) {
	t.Helper()
	id, cookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.stagingStore.Create(id, staging.Meta{DirID: fs.RootID(), FileName: "held",
		TernCookie: cookie, NFSStateID: srv.opens.NewStateID(), ClientID: incarnation}); err != nil {
		t.Fatal(err)
	}
}

// The principal check: another principal registering a client's identity
// gets CLID_INUSE while the client has write state and a live lease, and
// takes the identity over once the lease has expired. The takeover replaces
// the incarnation, whose writers and sessions go, and records the new
// principal.
func TestPrincipalCheck(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	srv := identityServer(t, fs)
	a := identityExchange(t, srv, 1, "shared", [8]byte{1}, false)
	status, sessionA := createResult(t, srv, 1, a.clientID, 1)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	incA := incarnationForTest(t, srv, a.clientID)
	// Without write state the identity is not in use: another principal
	// may register it.
	early := identityExchange(t, srv, 2, "shared", [8]byte{2}, false)
	if early.status != nfs4.NFS4_OK {
		t.Fatalf("EXCHANGE_ID by another principal without write state %s", nfs4.Nfsstat4Name(early.status))
	}
	holdWriteStateForTest(t, srv, fs, incA)
	if got := identityExchange(t, srv, 2, "shared", [8]byte{3}, false); got.status != nfs4.NFS4ERR_CLID_INUSE {
		t.Fatalf("EXCHANGE_ID by another principal %s, want CLID_INUSE", nfs4.Nfsstat4Name(got.status))
	}
	// The registration made before the write state is checked again at
	// confirmation.
	if got, _ := createResult(t, srv, 2, early.clientID, 1); got != nfs4.NFS4ERR_CLID_INUSE {
		t.Fatalf("CREATE_SESSION of the earlier registration %s, want CLID_INUSE", nfs4.Nfsstat4Name(got))
	}
	if !srv.clientLive(incA) || srv.sessions.lookup(sessionA) == nil {
		t.Fatal("CLID_INUSE disturbed the client")
	}
	// Once the lease has run out, the second principal takes over.
	endQuietForTest(srv)
	afterLeaseForTest(srv)
	b := identityExchange(t, srv, 2, "shared", [8]byte{2}, false)
	if b.status != nfs4.NFS4_OK {
		t.Fatalf("EXCHANGE_ID after the lease %s", nfs4.Nfsstat4Name(b.status))
	}
	if got, _ := createResult(t, srv, 2, b.clientID, 1); got != nfs4.NFS4_OK {
		t.Fatalf("takeover CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	recs := recordsForTest(srv)
	if len(recs) != 1 || uint64(recs[0].Incarnation) != incarnationForTest(t, srv, b.clientID) ||
		recs[0].Principal == (clientrecords.Principal{}) || recs[0].Purging != 0 {
		t.Fatalf("records after the takeover %+v", recs)
	}
	if srv.sessions.lookup(sessionA) != nil || srv.clientLive(incA) {
		t.Fatal("the replaced incarnation kept its session or stayed live")
	}
}

// A rebooted client, a new verifier from the same principal, replaces its
// incarnation at confirmation: the record names the new one, the old one's
// sessions go, and it is never live again.
func TestRebootReplacesIncarnation(t *testing.T) {
	local, dir := vfs.NewLocalTernVFS(t.TempDir()), t.TempDir()
	srv := identityServerAt(t, local, dir)
	old := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	_, oldSession := createResult(t, srv, 1, old.clientID, 1)
	oldInc := incarnationForTest(t, srv, old.clientID)
	reboot := identityExchange(t, srv, 1, "client", [8]byte{2}, false)
	status, newSession := createResult(t, srv, 1, reboot.clientID, 1)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	newInc := incarnationForTest(t, srv, reboot.clientID)
	recs := recordsForTest(srv)
	if len(recs) != 1 || uint64(recs[0].Incarnation) != newInc || recs[0].Verifier != [8]byte{2} || recs[0].Purging != 0 {
		t.Fatalf("records after the reboot %+v", recs)
	}
	if srv.sessions.lookup(oldSession) != nil || srv.clientLive(oldInc) || !srv.clientLive(newInc) {
		t.Fatal("the old incarnation survived its client's reboot")
	}
	if got := sequenceStatusForTest(t, srv, oldSession); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("SEQUENCE on the old session %s", nfs4.Nfsstat4Name(got))
	}
	// A restart keeps the new incarnation and nothing of the old one.
	restarted := identityServerAt(t, local, dir)
	if !restarted.clientLive(newInc) || restarted.clientLive(oldInc) {
		t.Fatal("restart changed which incarnation is live")
	}
	_ = newSession
}

// A start finishes what a crash left in a record: a reboot's purge, so the
// replaced incarnation is gone and the new one live, and an expiry, whose
// record is removed.
func TestRestartFinishesInterruptedTransitions(t *testing.T) {
	local, dir := vfs.NewLocalTernVFS(t.TempDir()), t.TempDir()
	first := identityServerAt(t, local, dir)
	principal := clientrecords.Principal{Flavor: nfs4.AUTH_SYS, Body: "owner"}
	old := confirmForTest(t, first, "rebooted", [8]byte{1}, principal)
	// The crash came after the reboot's record and before its purge.
	rebooted, err := first.records.Confirm([]byte("rebooted"), [8]byte{2}, principal, clientrecords.NewIncarnation())
	if err != nil || rebooted.Purging != clientrecords.Incarnation(old) {
		t.Fatalf("reboot record %+v %v", rebooted, err)
	}
	// And after another client's expiry record, before its removal.
	expired := confirmForTest(t, first, "expired", [8]byte{1}, principal)
	if err := first.records.Expire(clientrecords.Incarnation(expired)); err != nil {
		t.Fatal(err)
	}
	srv := identityServerAt(t, local, dir)
	recs := recordsForTest(srv)
	if len(recs) != 1 || recs[0].Incarnation != rebooted.Incarnation || recs[0].Purging != 0 {
		t.Fatalf("records after the restart %+v", recs)
	}
	if srv.clientLive(old) || srv.clientLive(expired) || !srv.clientLive(uint64(rebooted.Incarnation)) {
		t.Fatal("restart left the wrong incarnations live")
	}
}
