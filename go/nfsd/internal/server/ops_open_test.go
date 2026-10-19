// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/openstate"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

type syncFailStore struct {
	staging.Store
	armed bool
}

type syncFailFile struct{ staging.File }

func (s *syncFailStore) Get(id vfs.InodeID) staging.File {
	f := s.Store.Get(id)
	if f != nil && s.armed {
		return &syncFailFile{f}
	}
	return f
}

func (f *syncFailFile) Sync() error { return syscall.EIO }

func openWithSize(w *nfs4.COMPOUND4argsWriter, client uint64, name, owner string, size uint64) {
	w.AppendArgarray_Putrootfh()
	op := w.AppendArgarray_Open()
	op.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
	op.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
	o := op.StartOwner()
	o.SetClientid(client)
	op.Resume(o.SetOwner([]byte(owner)).Finish())
	how := op.SetOpenhow_Create()
	attrs := how.SetValue_Unchecked4()
	mask := attrs.StartAttrmask()
	mask.AppendData(1 << nfs4.FATTR4_SIZE)
	attrs.Resume(mask.Finish())
	attrs.Resume(attrs.StartAttrVals().SetData(binary.BigEndian.AppendUint64(nil, size)).Finish())
	how.Resume(attrs.Finish())
	op.Resume(how.Finish())
	op.Resume(op.SetClaim_Null().SetData([]byte(name)).Finish())
	w.Resume(op.Finish())
	w.AppendArgarray_Getfh()
}

// An OPEN that joins another owner's writer and sets its size truncates the
// shared staging even when the sync fails; the IO error ends the session.
func TestJoinedOpenSizeSyncFailure(t *testing.T) {
	root := t.TempDir()
	stageDir := t.TempDir()
	base, err := staging.NewLocalStore(stageDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	store := &syncFailStore{Store: base}
	srv, err := NewServer(vfs.NewLocalTernVFS(root), store, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	client := setupClient(t, conn, &xid)
	sid, fh := openCreateFile(t, conn, &xid, client, "joined")
	writeFileAt(t, conn, &xid, fh, sid, 0, []byte("abcdefghij"))
	id := stagingIDForTest(t, base, sid)
	path := filepath.Join(stageDir, fmt.Sprintf("%016x.staging", uint64(id)))
	ts := testSessionForClient(conn, client)
	slot, seq := ts.acquire()
	defer ts.release(slot)
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { openWithSize(w, client, "joined", "second", 3) }), ts, slot, seq, true)
	store.armed = true
	send := func() nfs4.COMPOUND4res {
		t.Helper()
		reply, err := sendRawRPC(conn, xid, procCompound, wire)
		xid++
		if err != nil {
			t.Fatal(err)
		}
		res, err := parseCompoundReply(reply)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	admissionStatus(t, send(), nfs4.NFS4ERR_IO, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_OPEN)
	info, err := os.Stat(path)
	if err != nil || info.Size() != 3 {
		t.Fatalf("shared real staging not truncated: %v %v", info, err)
	}
	admissionStatus(t, send(), nfs4.NFS4ERR_BADSESSION, nfs4.OP_SEQUENCE)
}

// A definite OPEN failure before any effect is cached and replays without
// ending the session.
func TestDefiniteOpenErrorStaysReplayable(t *testing.T) {
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	client, confirm, err := srv.clients.SetClientID([8]byte{}, []byte("definite-open"), clientstore.ClientOwner{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = srv.clients.ConfirmClientID(client, confirm, clientstore.RPCPrincipal{}); err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	ts.clientID = client
	moveSessionForTest(srv.sessions, srv.sessions.lookup(ts.id), client, ts.id)
	srv.clients.PinLeaseForTest(client)
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		op := w.AppendArgarray_Open()
		op.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
		o := op.StartOwner()
		o.SetClientid(ts.clientID)
		op.Resume(o.SetOwner([]byte("rejected")).Finish())
		op.SetOpenhow_Default(nfs4.OPEN4_NOCREATE)
		op.Resume(op.SetClaim_Null().SetData([]byte("missing")).Finish())
		w.Resume(op.Finish())
	})
	wire := admissionSequence(body, ts, 0, 1, true)
	first, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_NOENT, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_OPEN)
	replay, res := admissionRequest(t, srv, wire, 2, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_NOENT, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_OPEN)
	if !bytes.Equal(first[24:], replay[24:]) || srv.sessions.lookup(ts.id) == nil {
		t.Fatal("definite pre-effect OPEN failure poisoned session")
	}
}

// confirmedLookupHold blocks the first Lookup of the confirmed pointer once
// armed: in HasOpen that is the first read of an operation's durable-open
// check.
type confirmedLookupHold struct {
	vfs.TernVFS
	mu      sync.Mutex
	armed   bool
	entered chan struct{}
	release chan struct{}
}

func (fs *confirmedLookupHold) Lookup(dir vfs.InodeID, name string) (vfs.InodeID, error) {
	fs.mu.Lock()
	hold := fs.armed && name == clientstore.ConfirmedName
	if hold {
		fs.armed = false
	}
	fs.mu.Unlock()
	if hold {
		close(fs.entered)
		<-fs.release
	}
	return fs.TernVFS.Lookup(dir, name)
}

// An admitted CLOSE whose durable-open check finds the durable lease dead
// gets EXPIRED within bounded time, and the client's other state is left
// alone while its logical lease is held: local expiry is the gate's
// decision. (Before, the operation expired the client itself, re-entering
// the staging pathname lock CLOSE holds.)
func TestActiveCompoundDoesNotExpireClientState(t *testing.T) {
	fs := &confirmedLookupHold{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}), release: make(chan struct{})}
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	defer testutil.CloseSignal(fs.release)
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	inc := incarnationForTest(t, srv, clientID)
	aSid, aFH := openCreateFile(t, conn, &xid, clientID, "a")
	writeFileAt(t, conn, &xid, aFH, aSid, 0, []byte("A's bytes"))
	bSid, bFH := openCreateFile(t, conn, &xid, clientID, "b")
	writeFileAt(t, conn, &xid, bFH, bSid, 0, []byte("B's bytes"))
	bStaging := stagingIDForTest(t, staging, bSid)

	// No more refreshes: the lease stays as published, then dies.
	srv.clients.StopLeaseWorkers()
	var skew atomic.Int64
	srv.clients.SetClock(func() time.Time { return time.Now().Add(time.Duration(skew.Load())) })
	fs.mu.Lock()
	fs.armed = true
	fs.mu.Unlock()
	closed := make(chan uint32, 1)
	go func() {
		x := uint32(500)
		status, _, err := closeFileResultE(conn, &x, aFH, aSid)
		if err != nil {
			t.Error(err)
		}
		closed <- status
	}()
	testutil.AwaitSignal(t, fs.entered, "CLOSE's durable-open check")
	skew.Store(int64(10 * clientstore.LeaseWriteTerm))
	testutil.CloseSignal(fs.release)
	if got := testutil.AwaitValue(t, closed, "CLOSE with a dead durable lease"); got != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("CLOSE %s, want EXPIRED", nfs4.Nfsstat4Name(got))
	}
	awaitRequestHoldsReleased(t, srv)

	var bID nfs4.StateID
	copy(bID[:], bSid[4:])
	if !srv.opens.Live(bID) {
		t.Fatal("the operation expired the client's other open")
	}
	if m, ok := staging.GetMeta(bStaging); !ok || m.Retired {
		t.Fatal("the operation retired the client's other staging")
	}
	if _, err := fs.TernVFS.Lookup(vfs.InodeID(inc), clientstore.ActiveOpenName(bID)); err != nil {
		t.Fatalf("the operation removed the other open's marker: %v", err)
	}
	// The gate decides: not while the logical lease is held...
	end, _ := srv.beginDestroy(inc)
	if expired, _ := srv.decideExpired(inc, true); expired {
		t.Fatal("expired during the quiet period")
	}
	end()
	// ...and once it is not, the sweeper's decision expires the client.
	endQuietForTest(srv)
	srv.sweepExpiredClientState()
	if srv.opens.Live(bID) {
		t.Fatal("sweeper did not expire the dead client")
	}
	assertRetiredStaging(t, staging, bStaging)
}

// A recovered writer's downgrade that keeps WRITE installs its grant and
// generation as live state, so the old generation is OLD_STATEID and a
// later upgrade is INVAL; its staging is kept and its CLOSE publishes.
func TestRecoveredDowngradeKeepsItsGrant(t *testing.T) {
	name := "recovered-grant.txt"
	r := restartWithStagedWrite(t, name, []byte("staged data"))
	conn, xid := r.conn, r.xid
	stagingID := stagingIDForTest(t, r.staging, r.stateid)
	status, write := downgradeResult(t, conn, &xid, r.fh, r.stateid, nfs4.OPEN4_SHARE_ACCESS_WRITE, nfs4.OPEN4_SHARE_DENY_NONE)
	if status != nfs4.NFS4_OK {
		t.Fatalf("recovered BOTH to WRITE %s", nfs4.Nfsstat4Name(status))
	}
	var id nfs4.StateID
	copy(id[:], write[4:])
	if !r.srv.opens.Live(id) {
		t.Fatal("the downgrade installed no live state")
	}
	if got := stateidOpStatus(t, conn, &xid, "WRITE", r.fh, r.stateid); got != nfs4.NFS4ERR_OLD_STATEID {
		t.Fatalf("the old generation %s, want OLD_STATEID", nfs4.Nfsstat4Name(got))
	}
	if got, _ := downgradeResult(t, conn, &xid, r.fh, write, nfs4.OPEN4_SHARE_ACCESS_BOTH, nfs4.OPEN4_SHARE_DENY_NONE); got != nfs4.NFS4ERR_INVAL {
		t.Fatalf("WRITE to BOTH %s, want INVAL", nfs4.Nfsstat4Name(got))
	}
	status, write2 := downgradeResult(t, conn, &xid, r.fh, write, nfs4.OPEN4_SHARE_ACCESS_WRITE, nfs4.OPEN4_SHARE_DENY_NONE)
	if status != nfs4.NFS4_OK {
		t.Fatalf("same-value downgrade %s", nfs4.Nfsstat4Name(status))
	}
	if m, ok := r.staging.GetMeta(stagingID); !ok || m.Retired {
		t.Fatal("the recovered staging was retired")
	}
	writeFileAt(t, conn, &xid, r.fh, write2, uint64(len("staged data")), []byte(" and more"))
	closeFile(t, conn, &xid, r.fh, write2)
	data, err := readPublished(r.fs, name)
	if err != nil || string(data) != "staged data and more" {
		t.Fatalf("published %q %v", data, err)
	}
}

// When the recovered writer's owner already has a live open
// of the file, a downgrade to READ cannot continue as that owner's read
// open. It gets DELAY before publication, keeping both opens, and succeeds
// once the other open is closed. It never falls through to CLOSE.
func TestRecoveredDowngradeCollisionWaits(t *testing.T) {
	name := "recovered-collision.txt"
	r := restartWithStagedWrite(t, name, []byte("staged data"))
	conn, xid := r.conn, r.xid
	stagingID := stagingIDForTest(t, r.staging, r.stateid)
	status, live, _ := openByHandle(t, conn, &xid, r.clientID, "test-owner-"+name, r.fh, nfs4.OPEN4_SHARE_ACCESS_READ)
	if status != nfs4.NFS4_OK {
		t.Fatalf("the owner's live OPEN %s", nfs4.Nfsstat4Name(status))
	}
	before, _ := readPublished(r.fs, name)
	if got, _ := downgradeResult(t, conn, &xid, r.fh, r.stateid, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE); got != nfs4.NFS4ERR_DELAY {
		t.Fatalf("colliding recovered downgrade %s, want DELAY", nfs4.Nfsstat4Name(got))
	}
	var recovered, liveID nfs4.StateID
	copy(recovered[:], r.stateid[4:])
	copy(liveID[:], live[4:])
	if after, _ := readPublished(r.fs, name); string(after) != string(before) {
		t.Fatal("the refused downgrade published")
	}
	if m, ok := r.staging.GetMeta(stagingID); !ok || m.Retired || m.NFSStateID != recovered {
		t.Fatal("the refused downgrade changed the recovered staging")
	}
	if !r.srv.opens.Live(liveID) || r.srv.opens.Live(recovered) {
		t.Fatal("the refused downgrade changed the open state")
	}
	if open, err := r.srv.clients.HasOpen(r.clientID, recovered); err != nil || !open {
		t.Fatalf("the recovered writer's marker %t %v", open, err)
	}
	closeFile(t, conn, &xid, r.fh, live)
	status, read := downgradeResult(t, conn, &xid, r.fh, r.stateid, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE)
	if status != nfs4.NFS4_OK {
		t.Fatalf("retried downgrade %s", nfs4.Nfsstat4Name(status))
	}
	if data, _ := readPublished(r.fs, name); string(data) != "staged data" {
		t.Fatalf("published %q", data)
	}
	closeFile(t, conn, &xid, r.fh, read)
}

// A lone writer whose publication failed (no other reader) gets
// errors on its later operations, but they touch only its own state: the
// client's other writer keeps its open, marker and staging, and publishes.
func TestLoneFailedWriterLeavesSiblingsAlone(t *testing.T) {
	fs := &readerSourceVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	inc := incarnationForTest(t, srv, clientID)
	aSid, aFH := openCreateFile(t, conn, &xid, clientID, "a")
	writeFileAt(t, conn, &xid, aFH, aSid, 0, []byte("A"))
	bSid, bFH := openCreateFile(t, conn, &xid, clientID, "b")
	writeFileAt(t, conn, &xid, bFH, bSid, 0, []byte("B's bytes"))
	bStaging := stagingIDForTest(t, staging, bSid)
	fs.mu.Lock()
	fs.failLink = "a"
	fs.mu.Unlock()
	// The publication's error is the CLOSE's; the failed writer's open is
	// then inactive, so its later operations get EXPIRED (README).
	if status := closeFileStatus(t, conn, &xid, aFH, aSid); status != nfs4.NFS4ERR_IO {
		t.Fatalf("A's CLOSE with a LinkFile error %s, want NFS4ERR_IO", nfs4.Nfsstat4Name(status))
	}
	for _, op := range []string{"READ", "WRITE", "CLOSE"} {
		if got := stateidOpStatus(t, conn, &xid, op, aFH, aSid); got != nfs4.NFS4ERR_EXPIRED {
			t.Errorf("%s on the failed writer %s, want NFS4ERR_EXPIRED", op, nfs4.Nfsstat4Name(got))
		}
	}
	var bID nfs4.StateID
	copy(bID[:], bSid[4:])
	if !srv.opens.Live(bID) {
		t.Fatal("the failed writer's operations expired the sibling open")
	}
	if m, ok := staging.GetMeta(bStaging); !ok || m.Retired {
		t.Fatal("the failed writer's operations retired the sibling staging")
	}
	if _, err := fs.TernVFS.Lookup(vfs.InodeID(inc), clientstore.ActiveOpenName(bID)); err != nil {
		t.Fatalf("the sibling's marker: %v", err)
	}
	closeFile(t, conn, &xid, bFH, bSid)
	if data, err := readPublished(fs.TernVFS, "b"); err != nil || string(data) != "B's bytes" {
		t.Fatalf("sibling published %q %v", data, err)
	}
}

func assertRetiredStaging(t *testing.T, store staging.Store, id vfs.InodeID) {
	t.Helper()
	meta, ok := store.GetMeta(id)
	if !ok || !meta.Retired || store.Get(id) == nil {
		t.Fatal("expired open did not retain its recoverable staging")
	}
	if staging.TargetBusy(store, meta.DirID, meta.FileName) {
		t.Fatal("retired staging still reserves its pathname")
	}
}

func TestExpiredWriterRecoversAcknowledgedData(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, restart := range []bool{false, true} {
			t.Run(map[bool]string{false: "new", true: "existing"}[existing]+
				map[bool]string{false: "/same-server", true: "/restart"}[restart], func(t *testing.T) {
				fs := vfs.NewLocalTernVFS(t.TempDir())
				if existing {
					if _, err := fs.CreateFile(fs.RootID(), "file", bytes.NewBufferString("original")); err != nil {
						t.Fatal(err)
					}
				}
				stagingDir := t.TempDir()
				store, err := staging.NewLocalStore(stagingDir, nil)
				if err != nil {
					t.Fatal(err)
				}
				srv, err := NewServer(fs, store, nil)
				if err != nil {
					t.Fatal(err)
				}
				addr, cleanup := serveTestServer(t, srv)
				defer func() { cleanup() }()
				conn := dial(t, addr)
				defer func() { conn.Close() }()
				xid := uint32(1)
				clientID := setupClient(t, conn, &xid)
				status, state, fh, _ := openFileForOwner(t, conn, &xid, clientID,
					"recovering-writer", "file", nfs4.OPEN4_SHARE_ACCESS_BOTH, !existing, false)
				if status != nfs4.NFS4_OK {
					t.Fatal(nfs4.Nfsstat4Name(status))
				}
				writeFileAt(t, conn, &xid, fh, state, 0, []byte("UPDATED!"))
				id := stagingIDForTest(t, srv.stagingStore, state)
				srv.waitForClientGC()
				awaitRequestHoldsReleased(t, srv)
				now := time.Now().Add(2 * clientstore.LeaseTime)
				srv.clients.SetClock(func() time.Time { return now })
				srv.runLeaseSweep()
				assertRetiredStaging(t, store, id)
				if restart {
					conn.Close()
					cleanup()
					staging.CloseFiles(t, store)
					store, err = staging.NewLocalStore(stagingDir, nil)
					if err != nil {
						t.Fatal(err)
					}
					srv, err = NewServer(fs, store, nil)
					if err != nil {
						t.Fatal(err)
					}
					srv.clients.SetClock(func() time.Time { return now })
					addr, cleanup = serveTestServer(t, srv)
					conn = dial(t, addr)
				}
				newClient := setupClient(t, conn, &xid)
				if newClient == clientID {
					t.Fatal("expired lease reused its old clientid")
				}
				srv.waitForClientGC()
				status, recovered, recoveredFH, _ := openFileForOwner(t, conn, &xid, newClient,
					"recovering-writer", "file", nfs4.OPEN4_SHARE_ACCESS_BOTH, false, false)
				if status != nfs4.NFS4_OK {
					t.Fatal(nfs4.Nfsstat4Name(status))
				}
				if !bytes.Equal(fh, recoveredFH) {
					t.Fatal("recovery changed the writer's filehandle")
				}
				got, _ := readFileData(t, conn, &xid, recoveredFH, 0, 32)
				if string(got) != "UPDATED!" {
					t.Fatalf("recovery lost acknowledged data: %q", got)
				}
				closeFile(t, conn, &xid, recoveredFH, recovered)
				got, _ = readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "file"), 0, 32)
				if string(got) != "UPDATED!" {
					t.Fatalf("published recovered data = %q", got)
				}
			})
		}
	}
}

type closeTimeErrorVFS struct {
	*vfs.LocalTernVFS
}

func (fs *closeTimeErrorVFS) SetTime(vfs.InodeID, *time.Time, *time.Time) error {
	return errors.New("injected timestamp failure")
}

func TestPublishedCloseSurvivesTimestampFailure(t *testing.T) {
	fs := &closeTimeErrorVFS{vfs.NewLocalTernVFS(t.TempDir())}
	store, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	state, fh := openCreateFile(t, conn, &xid, clientID, "file")
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("published"))
	for i := range 2 {
		if i == 1 {
			retransmitNext(conn)
		}
		if status := closeFileStatus(t, conn, &xid, fh, state); status != nfs4.NFS4_OK {
			t.Fatalf("CLOSE/replay = %s", nfs4.Nfsstat4Name(status))
		}
	}
	if len(store.Entries()) != 0 || activeOpenMarkerCount(t, srv, clientID) != 0 {
		t.Fatal("committed CLOSE retained state")
	}
	got, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "file"), 0, 32)
	if string(got) != "published" {
		t.Fatalf("published data = %q", got)
	}
}

type metadataOnlyVFS struct {
	*vfs.LocalTernVFS
	reads atomic.Int32
	links atomic.Int32
}

func (fs *metadataOnlyVFS) Read(vfs.InodeID, uint64, []byte) (int, bool, error) {
	fs.reads.Add(1)
	return 0, false, errors.New("metadata-only CLOSE must not read data")
}

func (fs *metadataOnlyVFS) LinkFile(id vfs.InodeID, cookie vfs.Cookie, dir vfs.InodeID, name string, data io.Reader) error {
	if clientstore.LeaseTemp(name) {
		return fs.LocalTernVFS.LinkFile(id, cookie, dir, name, data)
	}
	fs.links.Add(1)
	return errors.New("metadata-only CLOSE must not publish data")
}

func TestMetadataOnlyCloseSkipsDataIO(t *testing.T) {
	for _, concurrentPublish := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-version", true: "newer-version"}[concurrentPublish], func(t *testing.T) {
			base := vfs.NewLocalTernVFS(t.TempDir())
			published, err := base.CreateFile(base.RootID(), "file", bytes.NewBufferString("original"))
			if err != nil {
				t.Fatal(err)
			}
			fs := &metadataOnlyVFS{LocalTernVFS: base}
			store, err := staging.NewLocalStore(t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			srv, err := NewServer(fs, store, nil)
			if err != nil {
				t.Fatal(err)
			}
			addr, cleanup := serveTestServer(t, srv)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()
			xid := uint32(1)
			clientID := setupClient(t, conn, &xid)
			state, fh := openWriteFile(t, conn, &xid, clientID, "file")
			id := stagingIDForTest(t, store, state)
			stamp := time.Unix(1_600_000_000, 0)
			if err := store.Get(id).SetTime(&stamp, nil); err != nil {
				t.Fatal(err)
			}
			if err := store.Get(id).Sync(); err != nil {
				t.Fatal(err)
			}
			want := "original"
			if concurrentPublish {
				want = "newer"
				published, err = base.CreateFile(base.RootID(), "file", bytes.NewBufferString(want))
				if err != nil {
					t.Fatal(err)
				}
			}
			closeFile(t, conn, &xid, fh, state)
			current, err := base.Lookup(base.RootID(), "file")
			if err != nil || current != published {
				t.Fatalf("CLOSE changed published inode: %v, err=%v", current, err)
			}
			info, err := base.Stat(current)
			if err != nil || !info.Mtime.Equal(stamp) {
				t.Fatalf("published attributes = %+v, err=%v", info, err)
			}
			got, err := base.ReadAll(current)
			if err != nil || string(got) != want {
				t.Fatalf("published data = %q, err=%v", got, err)
			}
			if fs.reads.Load() != 0 || fs.links.Load() != 0 {
				t.Fatal("timestamp-only CLOSE performed data IO")
			}
		})
	}
}

func createSizeStatus(t *testing.T, conn net.Conn, xid *uint32, clientID, size uint64) uint32 {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		owner := ow.StartOwner().SetClientid(clientID).SetOwner([]byte("size-error"))
		ow.Resume(owner.Finish())
		create := ow.SetOpenhow_Create()
		attrs := create.SetValue_Unchecked4()
		data := binary.BigEndian.AppendUint64(nil, size)
		create.Resume(finishTestFattr(attrs, attrMask{1 << nfs4.FATTR4_SIZE}, data))
		ow.Resume(create.Finish())
		claim := ow.SetClaim_Null()
		ow.Resume(claim.SetData([]byte("file")).Finish())
		w.Resume(ow.Finish())
	})
	*xid++
	return res.Status()
}

type sizeErrorStore struct{ staging.Store }

type sizeErrorFile struct{ staging.File }

func (s sizeErrorStore) Get(id vfs.InodeID) staging.File {
	sf := s.Store.Get(id)
	if sf == nil {
		return nil
	}
	return sizeErrorFile{sf}
}

func (sizeErrorFile) SetSize(uint64) error { return errors.New("injected size failure") }

func TestOpenSizeErrorReplay(t *testing.T) {
	for _, ioFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "FBIG", true: "IO"}[ioFailure], func(t *testing.T) {
			store, err := staging.NewLocalStore(t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			var ss staging.Store = store
			size, want := uint64(1<<63), uint32(nfs4.NFS4ERR_FBIG)
			if ioFailure {
				ss, size, want = sizeErrorStore{store}, 1, nfs4.NFS4ERR_IO
			}
			srv, err := NewServer(vfs.NewLocalTernVFS(t.TempDir()), ss, nil)
			if err != nil {
				t.Fatal(err)
			}
			addr, cleanup := serveTestServer(t, srv)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()
			xid := uint32(1)
			clientID := setupClient(t, conn, &xid)
			for range 2 {
				if got := createSizeStatus(t, conn, &xid, clientID, size); got != want {
					t.Fatalf("OPEN/replay = %s, want %s", nfs4.Nfsstat4Name(got), nfs4.Nfsstat4Name(want))
				}
			}
		})
	}
}

func TestRecoveredRetiredCreateAppliesSize(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	state, fh := openCreateFile(t, conn, &xid, clientID, "file")
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("acknowledged"))
	srv.waitForClientGC()
	now := time.Now().Add(2 * clientstore.LeaseTime)
	srv.clients.SetClock(func() time.Time { return now })
	srv.runLeaseSweep()
	newClient := setupClient(t, conn, &xid)
	size := uint64(3)
	recovered, recoveredFH := openCreateFileWithSize(t, conn, &xid, newClient, "file", &size)
	if !bytes.Equal(fh, recoveredFH) {
		t.Fatal("CREATE did not rebind the retired file")
	}
	closeFile(t, conn, &xid, recoveredFH, recovered)
	got, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "file"), 0, 32)
	if string(got) != "ack" {
		t.Fatalf("recovered CREATE size was not applied: %q", got)
	}
}

// markerVFS counts the client store's open-marker mutations.
type markerVFS struct {
	*vfs.LocalTernVFS
	creates, removes atomic.Int64
}

func (v *markerVFS) CreateFile(dirID vfs.InodeID, name string, data io.Reader) (vfs.InodeID, error) {
	if clientstore.IsActiveOpenName(name) {
		v.creates.Add(1)
	}
	return v.LocalTernVFS.CreateFile(dirID, name, data)
}

func (v *markerVFS) Remove(dirID vfs.InodeID, name string) error {
	if clientstore.IsActiveOpenName(name) {
		v.removes.Add(1)
	}
	return v.LocalTernVFS.Remove(dirID, name)
}

func (v *markerVFS) RemoveEdge(dirID vfs.InodeID, name string, edge vfs.Edge) error {
	if clientstore.IsActiveOpenName(name) {
		v.removes.Add(1)
	}
	return v.LocalTernVFS.RemoveEdge(dirID, name, edge)
}

// A read-only open is process-local: its OPEN and CLOSE write nothing to
// the client store, which on TernFS saves three shard mutations per open.
// A write open is marked, and an owner's read open gets its marker when
// an OPEN for write upgrades it. The cached client lists only the markers.
func TestReadOpenHasNoDurableMarker(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := &markerVFS{LocalTernVFS: vfs.NewLocalTernVFS(dir)}
	ss, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, ss, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	incarnation := incarnationForTest(t, srv, clientID)
	markers := func(wantCreates, wantRemoves int64, wantLive int, step string) {
		t.Helper()
		if c, r := fs.creates.Load(), fs.removes.Load(); c != wantCreates || r != wantRemoves {
			t.Fatalf("%s: marker creates/removes = %d/%d, want %d/%d", step, c, r, wantCreates, wantRemoves)
		}
		if got := activeOpenMarkerCount(t, srv, clientID); got != wantLive {
			t.Fatalf("%s: live markers = %d, want %d", step, got, wantLive)
		}
		cached := -1
		if incs, _ := srv.clients.SnapshotIncarnations(vfs.InodeID(incarnation)); len(incs) == 1 && incs[0].Cached != nil {
			cached = incs[0].Cached.Opens
		}
		if cached != wantLive {
			t.Fatalf("%s: cached client's open markers = %d, want %d", step, cached, wantLive)
		}
	}

	status, reader, fh, _ := openFileForOwner(
		t, conn, &xid, clientID, "reader", "file.txt",
		nfs4.OPEN4_SHARE_ACCESS_READ, false, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("read OPEN %s", nfs4.Nfsstat4Name(status))
	}
	if data, eof := readFileData(t, conn, &xid, fh, 0, 16); string(data) != "data" || !eof {
		t.Fatalf("READ through the unmarked open = %q, %t", data, eof)
	}
	markers(0, 0, 0, "read open")
	closeFile(t, conn, &xid, fh, reader)
	markers(0, 0, 0, "read close")

	status, writer, fh, _ := openFileForOwner(
		t, conn, &xid, clientID, "writer", "file.txt",
		nfs4.OPEN4_SHARE_ACCESS_BOTH, false, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("write OPEN %s", nfs4.Nfsstat4Name(status))
	}
	markers(1, 0, 1, "write open")
	writeFileAt(t, conn, &xid, fh, writer, 0, []byte("DATA"))
	closeFile(t, conn, &xid, fh, writer)
	markers(1, 1, 0, "write close")

	status, first, fh, _ := openFileForOwner(
		t, conn, &xid, clientID, "upgrader", "file.txt",
		nfs4.OPEN4_SHARE_ACCESS_READ, false, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("read OPEN before upgrade %s", nfs4.Nfsstat4Name(status))
	}
	markers(1, 1, 0, "read open before upgrade")
	status, upgraded, upgradedFH, _ := openFileForOwner(
		t, conn, &xid, clientID, "upgrader", "file.txt",
		nfs4.OPEN4_SHARE_ACCESS_BOTH, false, false,
	)
	if status != nfs4.NFS4_OK || !bytes.Equal(first[4:], upgraded[4:]) || !bytes.Equal(fh, upgradedFH) {
		t.Fatalf("upgrade OPEN %s, stateid %x -> %x", nfs4.Nfsstat4Name(status), first[4:], upgraded[4:])
	}
	markers(2, 1, 1, "upgrade to write")
	writeFileAt(t, conn, &xid, fh, upgraded, 0, []byte("DATA2"))
	closeFile(t, conn, &xid, fh, upgraded)
	markers(2, 2, 0, "close of the upgraded open")
	if data, err := readPublished(fs, "file.txt"); err != nil || string(data) != "DATA2" {
		t.Fatalf("published %q %v", data, err)
	}
}

// A read open whose client's lease lapses gets EXPIRED, as a write open
// does, but expiring the client's state is the lease gate's decision: both
// opens keep their entries, and once the lease is renewed the same
// stateids work again.
func TestReadOpenOutlivesLapsedLease(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	srv, addr, cleanup := startTestServerWithServer(t, dir)
	defer cleanup()
	var clock atomic.Int64
	clock.Store(time.Now().UnixNano())
	srv.clients.SetClock(func() time.Time { return time.Unix(0, clock.Load()) })
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	incarnation := incarnationForTest(t, srv, clientID)
	status, reader, readFH, _ := openFileForOwner(
		t, conn, &xid, clientID, "reader", "file.txt",
		nfs4.OPEN4_SHARE_ACCESS_READ, false, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("read OPEN %s", nfs4.Nfsstat4Name(status))
	}
	writer, writeFH := openCreateFile(t, conn, &xid, clientID, "written")
	check := func(want uint32, step string) {
		t.Helper()
		for _, open := range []struct {
			name    string
			stateid [16]byte
			fh      []byte
		}{{"read", reader, readFH}, {"write", writer, writeFH}} {
			fileID, _ := vfs.FHToInodeID(open.fh)
			if _, got := srv.lookupDurableOpen(incarnation, stateid4For(t, open.stateid), fileID); got != want {
				t.Fatalf("%s: %s open %s, want %s", step, open.name, nfs4.Nfsstat4Name(got), nfs4.Nfsstat4Name(want))
			}
			var id nfs4.StateID
			copy(id[:], open.stateid[4:])
			if !srv.opens.Live(id) {
				t.Fatalf("%s: %s open lost its entry", step, open.name)
			}
		}
	}

	// Past this process's published lease and the durable one, with no
	// sweep to expire the client.
	srv.clients.StopLeaseWorkers()
	clock.Add(int64(clientstore.LeaseWriteTerm + time.Second))
	check(nfs4.NFS4ERR_EXPIRED, "lapsed lease")
	// The refresher publishes again, as it does for an active client once
	// its writes succeed.
	srv.clients.RefreshMu.Lock()
	r, ok := srv.clients.RecordLocked(vfs.InodeID(incarnation), true)
	srv.clients.RefreshMu.Unlock()
	if !ok {
		t.Fatal("no refresher record")
	}
	r.SafePass(true)
	check(nfs4.NFS4_OK, "renewed lease")
}

// A file created and written before an nfsd restart is closed through the
// handle the client holds, the empty placeholder published at its name,
// which the sidecar records as the staging's base. The retried CLOSE finds
// the recovered staging and publishes it. A same-directory rename meanwhile
// is followed, as for any open writer, and a REMOVE detaches the writer so
// its CLOSE discards.
func TestRestartedCreateClosesThroughPlaceholder(t *testing.T) {
	for _, namespace := range []string{"linked", "renamed", "removed"} {
		t.Run(namespace, func(t *testing.T) {
			fs := vfs.NewLocalTernVFS(t.TempDir())
			stagingDir := t.TempDir()
			store, err := staging.NewLocalStore(stagingDir, nil)
			if err != nil {
				t.Fatal(err)
			}
			srv, err := NewServer(fs, store, nil)
			if err != nil {
				t.Fatal(err)
			}
			addr, cleanup := serveTestServer(t, srv)
			conn := dial(t, addr)
			xid := uint32(1)
			clientID := setupClient(t, conn, &xid)
			state, fh := openCreateFile(t, conn, &xid, clientID, "saved.tmp")
			writeFileAt(t, conn, &xid, fh, state, 0, []byte("the whole save"))
			conn.Close()
			cleanup()
			staging.CloseFiles(t, store)

			recovered, err := staging.NewLocalStore(stagingDir, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer staging.CloseFiles(t, recovered)
			srv, err = NewServer(fs, recovered, nil)
			if err != nil {
				t.Fatal(err)
			}
			addr, cleanup = serveTestServer(t, srv)
			defer cleanup()
			conn = dial(t, addr)
			defer conn.Close()
			setupClient(t, conn, &xid)
			name := "saved.tmp"
			switch namespace {
			case "renamed":
				renameFile(t, conn, &xid, "saved.tmp", "elsewhere")
				name = "elsewhere"
			case "removed":
				removeFile(t, conn, &xid, "saved.tmp")
			}
			if status, _ := closeFileResult(t, conn, &xid, fh, state); status != nfs4.NFS4_OK {
				t.Fatalf("CLOSE after restart = %s", nfs4.Nfsstat4Name(status))
			}
			if n := len(recovered.Entries()); n != 0 {
				t.Fatalf("%d staging entries left after the CLOSE", n)
			}
			if namespace == "removed" {
				if _, err := readPublished(fs, "saved.tmp"); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("removed name still resolves: %v", err)
				}
				return
			}
			if data, err := readPublished(fs, name); err != nil || string(data) != "the whole save" {
				t.Fatalf("published at %s: %q %v", name, data, err)
			}
		})
	}
}

func removeFile(t *testing.T, conn net.Conn, xid *uint32, name string) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rw := w.AppendArgarray_Remove()
		rw.Resume(rw.StartTarget().SetData([]byte(name)).Finish())
		w.Resume(rw.Finish())
	})
	*xid++
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("REMOVE %s: %s", name, nfs4.Nfsstat4Name(res.Status()))
	}
}

func renameFile(t *testing.T, conn net.Conn, xid *uint32, from, to string) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Savefh()
		rw := w.AppendArgarray_Rename()
		rw.Resume(rw.StartOldname().SetData([]byte(from)).Finish())
		rw.Resume(rw.StartNewname().SetData([]byte(to)).Finish())
		w.Resume(rw.Finish())
	})
	*xid++
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("RENAME %s -> %s: %s", from, to, nfs4.Nfsstat4Name(res.Status()))
	}
}

// A client that has just published a new version through its write open
// keeps seeing that version through the handle it holds, even if nfsd
// restarts in between: the publication alias is kept in the staging
// directory and reloaded.
func TestPublicationAliasSurvivesRestart(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "doc"), []byte("version one"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs := vfs.NewLocalTernVFS(root)
	stagingDir := t.TempDir()
	store, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	conn := dial(t, addr)
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	oldFH := lookupFH(t, conn, &xid, "doc")
	state, fh := openWriteFile(t, conn, &xid, clientID, "doc")
	if !bytes.Equal(fh, oldFH) {
		t.Fatal("write open changed the handle")
	}
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("version two"))
	closeFile(t, conn, &xid, fh, state)
	if data, _ := readFileData(t, conn, &xid, oldFH, 0, 64); string(data) != "version two" {
		t.Fatalf("before restart, old handle reads %q", data)
	}
	conn.Close()
	cleanup()
	staging.CloseFiles(t, store)

	recovered, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer staging.CloseFiles(t, recovered)
	srv, err = NewServer(fs, recovered, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup = serveTestServer(t, srv)
	defer cleanup()
	conn = dial(t, addr)
	defer conn.Close()
	setupClient(t, conn, &xid)
	if data, _ := readFileData(t, conn, &xid, oldFH, 0, 64); string(data) != "version two" {
		t.Fatalf("after restart, old handle reads %q, want the version this client published", data)
	}
	// Sweeping with the alias expired drops it from memory and the file.
	srv.clients.SetClock(func() time.Time { return time.Now().Add(2 * clientstore.LeaseTime) })
	srv.sweepViews()
	if recs, err := srv.aliases.load(srv.clients.Now()); err != nil || len(recs) != 0 {
		t.Fatalf("after expiry the log holds %d records (%v)", len(recs), err)
	}
}

// A client that holds a handle open keeps its publication alias past the
// lease after the publication, and the log must keep it as well: a process
// starting before the next sweep loads the alias rather than showing the
// client the version it replaced. A read open under another owner holds
// the handle while a write open publishes over it.
func TestHeldPublicationAliasStaysInLog(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "doc"), []byte("version one"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer staging.CloseFiles(t, store)
	srv, err := NewServer(vfs.NewLocalTernVFS(root), store, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.clients.StopLeaseWorkers)
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	status, _, readFH, _ := openFileForOwner(t, conn, &xid, clientID, "reader", "doc", nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatalf("read OPEN = %s", nfs4.Nfsstat4Name(status))
	}
	state, fh := openWriteFile(t, conn, &xid, clientID, "doc")
	if !bytes.Equal(fh, readFH) {
		t.Fatal("write open changed the handle")
	}
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("version two"))
	closeFile(t, conn, &xid, fh, state)

	// Two leases on, the read open still holds the alias. Nothing else may
	// act on the jumped clock.
	srv.clients.StopLeaseWorkers()
	awaitRequestHoldsReleased(t, srv)
	later := time.Now().Add(2 * clientstore.LeaseTime)
	srv.clients.SetClock(func() time.Time { return later })
	srv.sweepViews()
	held := srv.views.Entries()
	if len(held) != 1 {
		t.Fatalf("%d aliases in memory after the sweep, want the held one", len(held))
	}
	recs, err := srv.aliases.load(later.Add(clientstore.LeaseRenewAfter))
	if err != nil || len(recs) != 1 || vfs.InodeID(recs[0].DataID) != held[0].DataID {
		t.Fatalf("a process starting before the next sweep loads %v (%v), want the held alias", recs, err)
	}
}

// A publication made while a sweep rewrites the alias log survives a
// restart. The rewrite renames a new file over the log, so an alias
// appended to the old file in the meantime would be lost until the next
// sweep; a process started in that time would not load it. The rewrite
// here is held at its rename while an alias is published, and the clock
// moves on meanwhile: the alias lasts a lease from when it is made, not
// from when its publication began to wait.
func TestAliasPublishedDuringRewriteSurvivesRestart(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	stagingDir := t.TempDir()
	store, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.clients.StopLeaseWorkers)
	entered, release := make(chan struct{}), make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	srv.aliases.rename = func(oldpath, newpath string) error {
		enterOnce.Do(func() { close(entered) })
		<-release
		return os.Rename(oldpath, newpath)
	}
	swept, published := make(chan struct{}), make(chan struct{})
	finish := func() {
		releaseOnce.Do(func() { close(release) })
		for _, done := range []chan struct{}{swept, published} {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("sweep or publication did not finish")
			}
		}
	}
	key := openstate.ViewKey{ClientID: 7, FileID: 0x1001}
	go func() {
		defer close(swept)
		srv.sweepViews()
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		close(published)
		finish()
		t.Fatal("the sweep's rewrite did not reach its rename")
	}
	go func() {
		defer close(published)
		srv.publishedAs(key.ClientID, key.FileID, 0x1002)
	}()
	// Give the publication time to append to the file about to be
	// replaced, if nothing makes it wait for the rewrite.
	select {
	case <-published:
	case <-time.After(100 * time.Millisecond):
	}
	srv.clients.StopLeaseWorkers()
	later := time.Now().Add(2 * clientstore.LeaseTime)
	srv.clients.SetClock(func() time.Time { return later })
	finish()
	if recs, err := srv.aliases.load(later); err != nil || len(recs) != 1 || recs[0].DataID != 0x1002 {
		t.Fatalf("at the time of the publication the log loads %v (%v), want its alias", recs, err)
	}
	staging.CloseFiles(t, store)

	recovered, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer staging.CloseFiles(t, recovered)
	restarted, err := NewServer(fs, recovered, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.clients.StopLeaseWorkers)
	if dataID, ok := restarted.views.Get(key); !ok || dataID != 0x1002 {
		t.Fatalf("after a restart the alias is %v %v, want the publication made during the rewrite", dataID, ok)
	}
}

// The Linux client's recovery after an nfsd restart, as the kill tests show
// it: the wire clientid is new, so every open is reclaimed with CLAIM_FH
// before the in-flight WRITEs are replayed with the new stateids, then
// COMMIT and CLOSE follow. For a file created just before the kill, with
// no WRITE yet acknowledged, the reclaimed open must adopt the recovered
// staging and its CLOSE must publish what was written after the restart.
func TestLinuxRecoveryOfFreshCreateAcrossRestart(t *testing.T) {
	for _, exclusive := range []bool{true, false} {
		t.Run(map[bool]string{true: "exclusive", false: "unchecked"}[exclusive], func(t *testing.T) {
			fs := vfs.NewLocalTernVFS(t.TempDir())
			stagingDir := t.TempDir()
			store, err := staging.NewLocalStore(stagingDir, nil)
			if err != nil {
				t.Fatal(err)
			}
			srv, err := NewServer(fs, store, nil)
			if err != nil {
				t.Fatal(err)
			}
			addr, cleanup := serveTestServer(t, srv)
			conn := dial(t, addr)
			xid := uint32(1)
			clientID := setupClient(t, conn, &xid)
			var fh []byte
			if exclusive {
				var status uint32
				status, _, fh, _ = openExclusiveFile(t, conn, &xid, clientID, "owner", "save.tmp", [8]byte{1, 2, 3})
				if status != nfs4.NFS4_OK {
					t.Fatal(nfs4.Nfsstat4Name(status))
				}
			} else {
				var status uint32
				status, _, fh, _ = openFileForOwner(t, conn, &xid, clientID, "owner", "save.tmp", nfs4.OPEN4_SHARE_ACCESS_BOTH, true, false)
				if status != nfs4.NFS4_OK {
					t.Fatal(nfs4.Nfsstat4Name(status))
				}
			}
			// Killed here: no WRITE reached the server.
			conn.Close()
			cleanup()
			staging.CloseFiles(t, store)

			recovered, err := staging.NewLocalStore(stagingDir, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer staging.CloseFiles(t, recovered)
			srv, err = NewServer(fs, recovered, nil)
			if err != nil {
				t.Fatal(err)
			}
			addr, cleanup = serveTestServer(t, srv)
			defer cleanup()
			conn = dial(t, addr)
			defer conn.Close()
			newClientID := setupClient(t, conn, &xid)
			status, sid, reclaimedFH := openByHandle(t, conn, &xid, newClientID, "owner", fh, nfs4.OPEN4_SHARE_ACCESS_BOTH)
			if status != nfs4.NFS4_OK || !bytes.Equal(reclaimedFH, fh) {
				t.Fatalf("CLAIM_FH reclaim = %s", nfs4.Nfsstat4Name(status))
			}
			if n := len(recovered.Entries()); n != 1 {
				t.Fatalf("%d staging entries after the reclaim, want the one adopted", n)
			}
			writeFileAt(t, conn, &xid, fh, sid, 0, []byte("written after the restart"))
			res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
				pw := w.AppendArgarray_Putfh()
				pw.Resume(pw.StartObject().SetData(fh).Finish())
				w.Resume(pw.Finish())
				w.AppendArgarray_Commit()
			})
			xid++
			if res.Status() != nfs4.NFS4_OK {
				t.Fatalf("COMMIT %s", nfs4.Nfsstat4Name(res.Status()))
			}
			if status, _ := closeFileResult(t, conn, &xid, fh, sid); status != nfs4.NFS4_OK {
				t.Fatalf("CLOSE %s", nfs4.Nfsstat4Name(status))
			}
			if data, err := readPublished(fs, "save.tmp"); err != nil || string(data) != "written after the restart" {
				t.Fatalf("published %q %v", data, err)
			}
			if n := len(recovered.Entries()); n != 0 {
				t.Fatalf("%d staging entries left after the CLOSE", n)
			}
		})
	}
}
