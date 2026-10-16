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
