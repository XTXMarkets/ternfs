// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// countingWriteVFS counts mutations that changed the store.
type countingWriteVFS struct {
	vfs.TernVFS
	writes atomic.Int64
}

func (fs *countingWriteVFS) count(err error) error {
	if err == nil {
		fs.writes.Add(1)
	}
	return err
}
func (fs *countingWriteVFS) Mkdir(dir vfs.InodeID, name string) (vfs.InodeID, error) {
	id, err := fs.TernVFS.Mkdir(dir, name)
	return id, fs.count(err)
}
func (fs *countingWriteVFS) Symlink(dir vfs.InodeID, name, target string) (vfs.InodeID, error) {
	id, err := fs.TernVFS.Symlink(dir, name, target)
	return id, fs.count(err)
}
func (fs *countingWriteVFS) CreateFile(dir vfs.InodeID, name string, data io.Reader) (vfs.InodeID, error) {
	id, err := fs.TernVFS.CreateFile(dir, name, data)
	return id, fs.count(err)
}

// The refresher's own lease writes (its temps and lease.<nfsdID>) run in
// the background for every client and are not counted.
func (fs *countingWriteVFS) LinkFile(id vfs.InodeID, cookie vfs.Cookie, dir vfs.InodeID, name string, data io.Reader) error {
	if leaseTemp(name) {
		return fs.TernVFS.LinkFile(id, cookie, dir, name, data)
	}
	return fs.count(fs.TernVFS.LinkFile(id, cookie, dir, name, data))
}
func (fs *countingWriteVFS) Remove(dir vfs.InodeID, name string) error {
	return fs.count(fs.TernVFS.Remove(dir, name))
}
func (fs *countingWriteVFS) RemoveEdge(dir vfs.InodeID, name string, edge vfs.Edge) error {
	if leaseTemp(name) || strings.HasPrefix(name, leasePrefix) {
		return fs.TernVFS.RemoveEdge(dir, name, edge)
	}
	return fs.count(fs.TernVFS.RemoveEdge(dir, name, edge))
}
func (fs *countingWriteVFS) Rename(src vfs.InodeID, name string, dst vfs.InodeID, newName string) error {
	return fs.count(fs.TernVFS.Rename(src, name, dst, newName))
}
func (fs *countingWriteVFS) RenameEdge(src vfs.InodeID, name string, edge vfs.Edge, dst vfs.InodeID, newName string) error {
	if strings.HasPrefix(newName, leasePrefix) {
		return fs.TernVFS.RenameEdge(src, name, edge, dst, newName)
	}
	return fs.count(fs.TernVFS.RenameEdge(src, name, edge, dst, newName))
}

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
	req.credFlavor, req.credBody = authSys, authSysCredForTest(uid)
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

func identityServer(t *testing.T, fs vfs.TernVFS) *Server {
	t.Helper()
	srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.waitForClientGC)
	t.Cleanup(srv.clients.stopLeaseWorkers)
	useTestWireEpoch(srv)
	return srv
}

func wireRecords(srv *Server) int {
	srv.sessions.mu.Lock()
	defer srv.sessions.mu.Unlock()
	return len(srv.sessions.clients)
}

// The EXCHANGE_ID case table, RFC 8881 section 18.35.5, over wire records.
func TestExchangeIDCases(t *testing.T) {
	fs := &countingWriteVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	srv := identityServer(t, fs)
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
	srv.waitForClientGC()
	before := fs.writes.Load()
	warm := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	if warm.status != nfs4.NFS4_OK || warm.clientID != second.clientID || !warm.confirmed || warm.seq != 4 {
		t.Fatalf("warm EXCHANGE_ID %+v", warm)
	}
	srv.waitForClientGC()
	if fs.writes.Load() != before {
		t.Fatal("warm EXCHANGE_ID wrote to the store")
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
}

// Each process issues its own clientids for the same durable incarnation. A
// cold EXCHANGE_ID adopts a confirmed incarnation without writing, and no
// process accepts another's clientid.
func TestExchangeIDProcessesKeepIndependentClientIDs(t *testing.T) {
	local := vfs.NewLocalTernVFS(t.TempDir())
	first := identityServer(t, local)
	a := identityExchange(t, first, 1, "client", [8]byte{1}, false)
	if a.status != nfs4.NFS4_OK || identityCreate(t, first, 1, a.clientID, 1) != nfs4.NFS4_OK {
		t.Fatalf("first process registration %+v", a)
	}
	incarnation := incarnationForTest(t, first, a.clientID)
	first.waitForClientGC()

	fs := &countingWriteVFS{TernVFS: local}
	second := identityServer(t, fs)
	second.waitForClientGC()
	before := fs.writes.Load()
	b := identityExchange(t, second, 1, "client", [8]byte{1}, false)
	second.waitForClientGC()
	if b.status != nfs4.NFS4_OK || b.confirmed || b.seq != 1 || b.clientID == a.clientID ||
		b.clientID>>32 == a.clientID>>32 {
		t.Fatalf("second process EXCHANGE_ID %+v, first %#x", b, a.clientID)
	}
	if got := incarnationForTest(t, second, b.clientID); got != incarnation {
		t.Fatalf("second process bound incarnation %d, want %d", got, incarnation)
	}
	if fs.writes.Load() != before {
		t.Fatal("adoption wrote to the store")
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
	// The adopted record confirms by checking the durable state.
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
	srv.clients.setClock(func() time.Time { return time.Unix(0, clock.Load()) })
	confirmed := identityExchange(t, srv, 1, "confirmed", [8]byte{1}, false)
	if identityCreate(t, srv, 1, confirmed.clientID, 1) != nfs4.NFS4_OK {
		t.Fatal("registration")
	}

	// The cap counts every live record. Only records that waited a lease
	// interval for CREATE_SESSION are reclaimed.
	srv.sessions.wireLimit = 2
	setNow(created.Add(-nfsLeaseTime / 2))
	waiting := identityExchange(t, srv, 1, "waiting", [8]byte{1}, false)
	setNow(created)
	if got := identityExchange(t, srv, 1, "new", [8]byte{1}, false); got.status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("EXCHANGE_ID at the cap %s", nfs4.Nfsstat4Name(got.status))
	}
	setNow(created.Add(nfsLeaseTime / 2))
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

	// Expiry ends the record: the client starts again from the store.
	setNow(created.Add(nfsLeaseTime/2 + 3*nfsLeaseTime))
	srv.runLeaseSweep()
	if got := identityCreate(t, srv, 1, confirmed.clientID, 3); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION on expired client %s", nfs4.Nfsstat4Name(got))
	}
	again := identityExchange(t, srv, 1, "confirmed", [8]byte{1}, false)
	if again.status != nfs4.NFS4_OK || again.clientID == confirmed.clientID || again.confirmed || again.seq != 1 {
		t.Fatalf("EXCHANGE_ID after expiry %+v", again)
	}
}

// awaitOwnerWaiter waits until a second request queues on an owner's lock.
func awaitOwnerWaiter(t *testing.T, ss *sessionStore, owner string) {
	t.Helper()
	for deadline := time.Now().Add(testChannelTimeout); ; {
		refs := ss.ownerLocks.Refs(owner)
		if refs == 2 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("owner lock refs %d", refs)
		}
		time.Sleep(time.Millisecond)
	}
}

// Two EXCHANGE_IDs for one owner register in order. The second waits for
// the first to publish its record, then supersedes it, so the last reply's
// clientid is usable. Without that order, the first could publish a pending
// incarnation the second had already replaced, leaving neither usable.
func TestExchangeIDSameOwnerRegistersInOrder(t *testing.T) {
	fs := &confirmedLookupGateVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	srv := identityServer(t, fs)
	defer closeSignal(fs.release)
	fs.armed.Store(true)
	first, second := make(chan exchangeResult, 1), make(chan exchangeResult, 1)
	go func() { first <- identityExchange(t, srv, 1, "client", [8]byte{1}, false) }()
	awaitSignal(t, fs.entered, "first registration in the store")
	go func() { second <- identityExchange(t, srv, 1, "client", [8]byte{1}, false) }()
	awaitOwnerWaiter(t, srv.sessions, "client")
	select {
	case got := <-second:
		t.Fatalf("second registration overtook the first: %+v", got)
	default:
	}
	closeSignal(fs.release)
	a := awaitValue(t, first, "first registration")
	b := awaitValue(t, second, "second registration")
	if a.status != nfs4.NFS4_OK || b.status != nfs4.NFS4_OK || a.clientID == b.clientID {
		t.Fatalf("registrations %+v, %+v", a, b)
	}
	if got := identityCreate(t, srv, 1, a.clientID, 1); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION on superseded registration %s", nfs4.Nfsstat4Name(got))
	}
	if got := identityCreate(t, srv, 1, b.clientID, 1); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION on latest registration %s", nfs4.Nfsstat4Name(got))
	}
}

// A DESTROY_CLIENTID admitted for one record never destroys a newer record
// of the same incarnation.
func TestDestroyClientIDRetiresOnlyItsRecord(t *testing.T) {
	local := vfs.NewLocalTernVFS(t.TempDir())
	first := identityServer(t, local)
	p := identityExchange(t, first, 1, "client", [8]byte{1}, false)
	if identityCreate(t, first, 1, p.clientID, 1) != nfs4.NFS4_OK {
		t.Fatal("registration")
	}
	fs := &confirmedLookupGateVFS{TernVFS: local, entered: make(chan struct{}), release: make(chan struct{})}
	srv := identityServer(t, fs)
	ss := srv.sessions
	defer closeSignal(fs.release)
	// A: an unconfirmed cold adoption of P, with no sessions or opens.
	a := identityExchange(t, srv, 1, "client", [8]byte{1}, false)
	ss.mu.Lock()
	recordA := ss.wireRecordLocked(a.clientID)
	ss.mu.Unlock()

	// A second adoption of P is in the store when DESTROY_CLIENTID(A) arrives.
	// Client GC also reads the confirmed pointer; let it finish first so the
	// gate catches the adoption.
	srv.waitForClientGC()
	fs.armed.Store(true)
	exchanged := make(chan exchangeResult, 1)
	go func() { exchanged <- identityExchange(t, srv, 1, "client", [8]byte{1}, false) }()
	awaitSignal(t, fs.entered, "adoption in the store")
	destroyed := make(chan uint32, 1)
	go func() { destroyed <- identityDestroy(t, srv, a.clientID) }()
	awaitOwnerWaiter(t, ss, "client")
	closeSignal(fs.release)
	b := awaitValue(t, exchanged, "second adoption")
	if b.status != nfs4.NFS4_OK || b.clientID == a.clientID || incarnationForTest(t, srv, b.clientID) != recordA.incarnation {
		t.Fatalf("second adoption %+v", b)
	}
	if got := awaitValue(t, destroyed, "delayed DESTROY_CLIENTID"); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("DESTROY_CLIENTID of a superseded record %s", nfs4.Nfsstat4Name(got))
	}
	// The same holds for a destruction already past its checks.
	if got := ss.destroyClientRecord(recordA); got != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("stale record destruction %s", nfs4.Nfsstat4Name(got))
	}
	if wireRecords(srv) != 1 {
		t.Fatalf("%d wire records", wireRecords(srv))
	}
	if got := identityCreate(t, srv, 1, b.clientID, 1); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION on the newer record %s", nfs4.Nfsstat4Name(got))
	}
}

// Deciding whether EXCHANGE_ID may adopt a confirmed incarnation only reads
// the store, and follows SetClientID's predicate: no expired marker, and
// either no lease or a live one.
func TestClientStoreExchangeIDAdoptionObservation(t *testing.T) {
	const verifier = 1
	type setup struct {
		name                string
		leases              map[string]time.Duration // slot name -> expiry relative to now
		marker, readError   bool
		otherPrincipal      bool
		adopt               bool
		wantErr, wantInUse  bool
		zeroWrites          bool
		keepSlots, noMarker bool
	}
	for _, test := range []setup{
		{name: "live and old", leases: map[string]time.Duration{"lease.live": nfsLeaseTime, "lease.old": -2 * nfsLeaseTime},
			adopt: true, zeroWrites: true, keepSlots: true, noMarker: true},
		{name: "absent", adopt: true, zeroWrites: true, noMarker: true},
		{name: "all dead", leases: map[string]time.Duration{"lease.dead": -2 * nfsLeaseTime},
			keepSlots: true, noMarker: true},
		{name: "expired marker", leases: map[string]time.Duration{"lease.live": nfsLeaseTime}, marker: true,
			keepSlots: true},
		{name: "read error", leases: map[string]time.Duration{"lease.live": nfsLeaseTime}, readError: true,
			wantErr: true, zeroWrites: true, keepSlots: true, noMarker: true},
		{name: "other principal", leases: map[string]time.Duration{"lease.live": nfsLeaseTime}, otherPrincipal: true,
			wantErr: true, wantInUse: true, zeroWrites: true, keepSlots: true, noMarker: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			failing := &lookupFailureVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
			fs := &countingWriteVFS{TernVFS: failing}
			store, id := newConfirmedStoreClient(t, fs, []byte("adopt"), [8]byte{verifier}, clientOwner{})
			now := time.Now()
			store.now = func() time.Time { return now }
			incarnation := vfs.InodeID(id)
			for name, expiry := range test.leases {
				if _, err := store.createJSON(incarnation, name, durableLease{ExpiresUnixNano: now.Add(expiry).UnixNano()}); err != nil {
					t.Fatal(err)
				}
			}
			if test.marker {
				if err := store.ensureMarker(incarnation, expiredName); err != nil {
					t.Fatal(err)
				}
			}
			if test.otherPrincipal {
				// A live open with another principal makes the identity in use.
				if err := store.ensureMarker(incarnation, activeOpenName(nfs4.StateID{1})); err != nil {
					t.Fatal(err)
				}
			}
			if test.readError {
				failing.mu.Lock()
				failing.name, failing.remaining = expiredName, 1
				failing.mu.Unlock()
			}
			owner := clientOwner{}
			if test.otherPrincipal {
				owner.principal = rpcPrincipal{flavor: authSys, body: "other"}
			}
			before := fs.writes.Load()
			got, _, adopted, err := store.ExchangeID([8]byte{verifier}, []byte("adopt"), owner)
			switch {
			case test.wantErr:
				var inUse clientInUseError
				if err == nil || errors.As(err, &inUse) != test.wantInUse {
					t.Fatalf("err %v", err)
				}
			case err != nil:
				t.Fatal(err)
			case adopted != test.adopt || (got == id) != test.adopt:
				t.Fatalf("adopted %t, incarnation %d (confirmed %d)", adopted, got, id)
			}
			if test.zeroWrites && fs.writes.Load() != before {
				t.Fatalf("%d writes", fs.writes.Load()-before)
			}
			if test.keepSlots {
				for name := range test.leases {
					if _, err := failing.TernVFS.Lookup(incarnation, name); err != nil {
						t.Fatalf("observation removed %s: %v", name, err)
					}
				}
			}
			if test.noMarker {
				if _, err := failing.TernVFS.Lookup(incarnation, expiredName); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("observation published an expired marker: %v", err)
				}
			}
		})
	}
}
