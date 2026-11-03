// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
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

	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientrecords"
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
// shared staging even when the sync fails. The IO error is the OPEN's
// reply, recorded and replayed like any other; the session lives.
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
	admissionStatus(t, send(), nfs4.NFS4ERR_IO, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_OPEN)
}

// A definite OPEN failure before any effect is cached and replays without
// ending the session.
func TestDefiniteOpenErrorStaysReplayable(t *testing.T) {
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	client := confirmForTest(t, srv, "definite-open", [8]byte{}, clientrecords.Principal{})
	ts := connectionSession(t, srv)
	ts.clientID = client
	moveSessionForTest(srv.sessions, srv.sessions.lookup(ts.id), client, ts.id)
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

// An admitted CLOSE whose client is no longer live gets EXPIRED, and the
// client's other state is left alone while its logical lease is held:
// local expiry is the gate's decision. (Before, the operation expired the
// client itself, re-entering the staging pathname lock CLOSE holds.)
func TestActiveCompoundDoesNotExpireClientState(t *testing.T) {
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(vfs.NewLocalTernVFS(t.TempDir()), staging, nil)
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
	writeFileAt(t, conn, &xid, aFH, aSid, 0, []byte("A's bytes"))
	bSid, bFH := openCreateFile(t, conn, &xid, clientID, "b")
	writeFileAt(t, conn, &xid, bFH, bSid, 0, []byte("B's bytes"))
	bStaging := stagingIDForTest(t, staging, bSid)

	// The record goes, as if the client had been destroyed elsewhere.
	if err := srv.records.Remove(clientrecords.Incarnation(inc)); err != nil {
		t.Fatal(err)
	}
	if got, _, err := closeFileResultE(conn, &xid, aFH, aSid); err != nil || got != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("CLOSE %s %v, want EXPIRED", nfs4.Nfsstat4Name(got), err)
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
	// The gate decides: not while the logical lease is held...
	if expired, _ := srv.decideExpired(inc, true); expired {
		t.Fatal("expired during the quiet period")
	}
	// ...and once it is not, the sweeper's decision expires the client.
	endQuietForTest(srv)
	afterLeaseForTest(srv)
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
	if !r.srv.clientLive(r.clientID) {
		t.Fatal("the refused downgrade expired the recovered writer's client")
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
	aSid, aFH := openCreateFile(t, conn, &xid, clientID, "a")
	writeFileAt(t, conn, &xid, aFH, aSid, 0, []byte("A"))
	bSid, bFH := openCreateFile(t, conn, &xid, clientID, "b")
	writeFileAt(t, conn, &xid, bFH, bSid, 0, []byte("B's bytes"))
	bStaging := stagingIDForTest(t, staging, bSid)
	fs.mu.Lock()
	fs.failLink = "a"
	fs.mu.Unlock()
	// The publication's error is the CLOSE's; the failed writer's open is
	// then expired, so its later operations get EXPIRED (README.md#stateid-checks).
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
				awaitRequestHoldsReleased(t, srv)
				now := time.Now().Add(2 * leaseTime)
				srv.SetClock(func() time.Time { return now })
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
					srv.SetClock(func() time.Time { return now })
					addr, cleanup = serveTestServer(t, srv)
					conn = dial(t, addr)
				}
				newClient := setupClient(t, conn, &xid)
				if newClient == clientID {
					t.Fatal("expired lease reused its old clientid")
				}
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
	if len(store.Entries()) != 0 {
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
	now := time.Now().Add(2 * leaseTime)
	srv.SetClock(func() time.Time { return now })
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
	srv.SetClock(func() time.Time { return time.Now().Add(2 * leaseTime) })
	srv.sweepViews()
	if recs, err := srv.aliases.load(srv.now()); err != nil || len(recs) != 0 {
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
	awaitRequestHoldsReleased(t, srv)
	later := time.Now().Add(2 * leaseTime)
	srv.SetClock(func() time.Time { return later })
	srv.sweepViews()
	held := srv.views.Entries()
	if len(held) != 1 {
		t.Fatalf("%d aliases in memory after the sweep, want the held one", len(held))
	}
	recs, err := srv.aliases.load(later.Add(leaseSweepInterval))
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
	later := time.Now().Add(2 * leaseTime)
	srv.SetClock(func() time.Time { return later })
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
	if dataID, ok := restarted.views.Get(key); !ok || dataID != 0x1002 {
		t.Fatalf("after a restart the alias is %v %v, want the publication made during the rewrite", dataID, ok)
	}
}

// The alias log is read up to its first malformed line, which a torn
// append leaves when the process dies in the middle of it. At startup the
// aliases before that line are installed and none after it.
func TestAliasLogLoadsUpToMalformedLine(t *testing.T) {
	stagingDir := t.TempDir()
	expires := time.Now().Add(leaseTime)
	before := aliasRecord{ClientID: 7, FileID: 0x1001, DataID: 0x1002, Expires: expires}
	after := aliasRecord{ClientID: 7, FileID: 0x2001, DataID: 0x2002, Expires: expires}
	var log []byte
	for _, rec := range []aliasRecord{before, after} {
		line, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		log = append(log, line...)
		log = append(log, '\n')
		if rec == before {
			log = append(log, `{"client":7,"fi`+"\n"...)
		}
	}
	path := filepath.Join(stagingDir, staging.AliasesDirName, aliasLogName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, log, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer staging.CloseFiles(t, store)
	srv, err := NewServer(vfs.NewLocalTernVFS(t.TempDir()), store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dataID, ok := srv.views.Get(openstate.ViewKey{ClientID: 7, FileID: 0x1001}); !ok || dataID != 0x1002 {
		t.Fatalf("alias before the malformed line = %v %v, want it installed", dataID, ok)
	}
	if dataID, ok := srv.views.Get(openstate.ViewKey{ClientID: 7, FileID: 0x2001}); ok {
		t.Fatalf("alias after the malformed line installed as %v", dataID)
	}
}

// The Linux client's recovery when its session is lost across an nfsd
// restart (no session file, or an expired lease): the wire clientid is
// new, so every open is reclaimed with CLAIM_FH before the in-flight WRITEs
// are replayed with the new stateids, then COMMIT and CLOSE follow. For a
// file created just before the kill, with
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

// publishFaultVFS is a backend whose shard stops answering or refuses; its
// faults are switched under its mutex, never by replacing the backend
// under a running server.
// linkFault, when set, runs each LinkFile of a user file in place of the
// link: it may apply it with link, and returns what the LinkFile reports.
// scrapTimeouts and statTimeouts answer that many ScrapFile and Stat calls
// with TIMEOUT, and onSetTime runs before each SetTime. Once counting is
// on it records, in order, the constructions, links and scraps the backend
// carried out.
type publishFaultVFS struct {
	vfs.TernVFS
	mu            sync.Mutex
	linkFault     func(n int, id vfs.InodeID, cookie vfs.Cookie, link func() error) error
	scrapTimeouts int
	statTimeouts  int
	readTimeouts  int // Read calls still to time out
	// The faults below hold while set: Lookup of lookupTimeout, SetTime,
	// ConstructFile in the root, and RemoveEdge and RenameEdge, which do
	// not apply and are classified as on the remote backend.
	lookupTimeout string
	setTimeErr    error
	constructErr  error
	namespaceErr  error
	onScrap       func(id vfs.InodeID) error // runs before each scrap; an error is the scrap's answer
	onSetTime     func(id vfs.InodeID)
	counting      bool
	links         int
	events        []string
	// root and keep, when set, keep a scrapped transient's inode
	// allocated (keepScrapped).
	root, keep string
}

// newLocalPublishVFS is a publishFaultVFS over a LocalTernVFS at root.
func newLocalPublishVFS(t *testing.T, root string) *publishFaultVFS {
	return &publishFaultVFS{TernVFS: vfs.NewLocalTernVFS(root), root: root, keep: t.TempDir()}
}

// keepScrapped links a transient aside before it is scrapped. TernFS never
// reuses an inode ID; LocalTernVFS would give the next transient the
// scrapped one's inode number, which hides a candidate that is not fresh.
func (fs *publishFaultVFS) keepScrapped(id vfs.InodeID) {
	if fs.root == "" {
		return
	}
	dir := filepath.Clean(fs.root) + ".nfsd/transients" // LocalTernVFS's private transients
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Ino == id.Fileid() {
			_ = os.Link(filepath.Join(dir, e.Name()), filepath.Join(fs.keep, e.Name()))
		}
	}
}

func (fs *publishFaultVFS) note(kind string, id vfs.InodeID) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.counting {
		fs.events = append(fs.events, fmt.Sprintf("%s %d", kind, id))
	}
}

// count returns how many recorded events are of kind.
func (fs *publishFaultVFS) count(kind string) int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	n := 0
	for _, e := range fs.events {
		if len(e) > len(kind) && e[:len(kind)+1] == kind+" " {
			n++
		}
	}
	return n
}

func (fs *publishFaultVFS) LinkFile(id vfs.InodeID, cookie vfs.Cookie, dir vfs.InodeID, name string, data io.Reader) error {
	if data == nil {
		return fs.TernVFS.LinkFile(id, cookie, dir, name, data)
	}
	fs.mu.Lock()
	fs.links++
	n, fault := fs.links, fs.linkFault
	fs.mu.Unlock()
	link := func() error {
		err := fs.TernVFS.LinkFile(id, cookie, dir, name, data)
		if err == nil {
			fs.note("link", id)
		}
		return err
	}
	if fault == nil {
		return link()
	}
	return fault(n, id, cookie, link)
}

// timeoutLinks answers the next n links with TIMEOUT, after applying them
// when apply is set.
func (fs *publishFaultVFS) timeoutLinks(n int, apply bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	base := fs.links
	fs.linkFault = func(i int, _ vfs.InodeID, _ vfs.Cookie, link func() error) error {
		if i > base+n {
			return link()
		}
		if apply {
			if err := link(); err != nil {
				return err
			}
		}
		return msgs.TIMEOUT
	}
}

func (fs *publishFaultVFS) Stat(id vfs.InodeID) (vfs.NodeInfo, error) {
	fs.mu.Lock()
	timeout := fs.statTimeouts > 0
	if timeout {
		fs.statTimeouts--
	}
	fs.mu.Unlock()
	if timeout {
		return vfs.NodeInfo{}, msgs.TIMEOUT
	}
	return fs.TernVFS.Stat(id)
}

func (fs *publishFaultVFS) faults() (string, error, error, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.lookupTimeout, fs.setTimeErr, fs.constructErr, fs.namespaceErr
}

func (fs *publishFaultVFS) Lookup(dir vfs.InodeID, name string) (vfs.InodeID, error) {
	if lookup, _, _, _ := fs.faults(); lookup != "" && lookup == name {
		return 0, msgs.TIMEOUT
	}
	return fs.TernVFS.Lookup(dir, name)
}

func (fs *publishFaultVFS) RemoveEdge(dir vfs.InodeID, name string, edge vfs.Edge) error {
	if _, _, _, err := fs.faults(); err != nil {
		return err
	}
	return fs.TernVFS.RemoveEdge(dir, name, edge)
}

func (fs *publishFaultVFS) RenameEdge(dir vfs.InodeID, name string, edge vfs.Edge, dst vfs.InodeID, newName string) error {
	if _, _, _, err := fs.faults(); err != nil {
		return err
	}
	return fs.TernVFS.RenameEdge(dir, name, edge, dst, newName)
}

func (fs *publishFaultVFS) ClassifyMutation(op uint32, err error) vfs.MutationOutcome {
	if _, _, _, injected := fs.faults(); injected != nil && errors.Is(err, injected) {
		return (&vfs.RemoteTernVFS{}).ClassifyMutation(op, err)
	}
	return fs.TernVFS.ClassifyMutation(op, err)
}

// onLink sets linkFault.
func (fs *publishFaultVFS) onLink(fault func(n int, id vfs.InodeID, cookie vfs.Cookie, link func() error) error) {
	fs.set(func() { fs.linkFault = fault })
}

// set changes the faults under the mutex.
func (fs *publishFaultVFS) set(change func()) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	change()
}

func (fs *publishFaultVFS) Read(id vfs.InodeID, offset uint64, dest []byte) (int, bool, error) {
	fs.mu.Lock()
	timeout := fs.readTimeouts > 0
	if timeout {
		fs.readTimeouts--
	}
	fs.mu.Unlock()
	if timeout {
		return 0, false, msgs.TIMEOUT
	}
	return fs.TernVFS.Read(id, offset, dest)
}

func (fs *publishFaultVFS) ConstructFile(dir vfs.InodeID) (vfs.InodeID, vfs.Cookie, error) {
	if _, _, err, _ := fs.faults(); err != nil && dir == fs.RootID() {
		return 0, vfs.Cookie{}, err
	}
	id, cookie, err := fs.TernVFS.ConstructFile(dir)
	if err == nil && dir == fs.RootID() {
		fs.note("construct", id)
	}
	return id, cookie, err
}

func (fs *publishFaultVFS) ScrapFile(id vfs.InodeID, cookie vfs.Cookie) error {
	fs.mu.Lock()
	timeout := fs.scrapTimeouts > 0
	if timeout {
		fs.scrapTimeouts--
	}
	hook := fs.onScrap
	fs.mu.Unlock()
	if timeout {
		return msgs.TIMEOUT
	}
	if hook != nil {
		if err := hook(id); err != nil {
			return err
		}
	}
	fs.keepScrapped(id)
	err := fs.TernVFS.ScrapFile(id, cookie)
	if err == nil {
		fs.note("scrap", id)
	}
	return err
}

func (fs *publishFaultVFS) SetTime(id vfs.InodeID, mtime, atime *time.Time) error {
	fs.mu.Lock()
	hook, injected := fs.onSetTime, fs.setTimeErr
	fs.mu.Unlock()
	if hook != nil {
		hook(id)
	}
	if injected != nil {
		return injected
	}
	return fs.TernVFS.SetTime(id, mtime, atime)
}

type publishFixture struct {
	fs       *publishFaultVFS
	store    staging.Store
	srv      *Server
	addr     string
	conn     net.Conn
	xid      *uint32
	clientID uint64
}

func newPublishFixture(t *testing.T) *publishFixture {
	t.Helper()
	return newPublishFixtureWithStore(t, func(s staging.Store) staging.Store { return s })
}

// newPublishFixtureWithStore is newPublishFixture with the server's store
// wrapped.
func newPublishFixtureWithStore(t *testing.T, wrap func(staging.Store) staging.Store) *publishFixture {
	t.Helper()
	fs := newLocalPublishVFS(t, t.TempDir())
	local, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, wrap(local), nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	t.Cleanup(stop)
	xid := uint32(1)
	f := &publishFixture{fs: fs, store: local, srv: srv, addr: addr, conn: dial(t, addr), xid: &xid}
	t.Cleanup(func() { f.conn.Close() })
	f.clientID = setupClient(t, f.conn, f.xid)
	return f
}

// mustDrained fails unless no open and no staging is left.
func (f *publishFixture) mustDrained(t *testing.T, when string) {
	t.Helper()
	if n := f.srv.opens.StateCount(); n != 0 || len(f.store.Entries()) != 0 {
		t.Fatalf("%d opens, %d staging entries %s", n, len(f.store.Entries()), when)
	}
}

// writer opens name for write, creating it, writes data and starts
// counting backend calls. It returns the open, the handle and the staging.
func (f *publishFixture) writer(t *testing.T, name, data string) ([16]byte, []byte, vfs.InodeID) {
	t.Helper()
	sid, fh := openCreateFile(t, f.conn, f.xid, f.clientID, name)
	writeFileAt(t, f.conn, f.xid, fh, sid, 0, []byte(data))
	f.fs.mu.Lock()
	f.fs.counting = true
	f.fs.mu.Unlock()
	return sid, fh, stagingIDForTest(t, f.store, sid)
}

// reader opens name for READ as another owner of the client.
func (f *publishFixture) reader(t *testing.T, name string) [16]byte {
	t.Helper()
	status, sid, _, _ := openFileForOwner(t, f.conn, f.xid, f.clientID, "reader", name, nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatalf("reader OPEN %s", nfs4.Nfsstat4Name(status))
	}
	return sid
}

// sendBody sends a compound body through the session shim and returns the
// reply.
func sendBody(t *testing.T, conn net.Conn, xid *uint32, body []byte) []byte {
	t.Helper()
	reply, err := sendRPC(conn, *xid, procCompound, body)
	*xid++
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

// readStateid reads through sid and returns the status and data.
func readStateid(t *testing.T, conn net.Conn, xid *uint32, fh []byte, sid [16]byte) (uint32, string) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		putfhOp(w, fh)
		rw := w.AppendArgarray_Read()
		setStateid(rw.Stateid(), sid)
		rw.SetOffset(0)
		rw.SetCount(64)
	})
	*xid++
	if res.Status() != nfs4.NFS4_OK {
		return res.Status(), ""
	}
	iter := res.Resarray()
	iter.Next()
	iter.Next()
	return nfs4.NFS4_OK, string(iter.Resarray().Value().AsREAD4resEntry().Value().AsREAD4resok().Data())
}

// otherClient connects a second client.
func (f *publishFixture) otherClient(t *testing.T) (net.Conn, *uint32, uint64) {
	t.Helper()
	conn := dial(t, f.addr)
	t.Cleanup(func() { conn.Close() })
	xid := uint32(1)
	id, confirm := requestClientID(t, conn, &xid, "other-client", [8]byte{2})
	if status := confirmClientID(t, conn, &xid, id, confirm); status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	return conn, &xid, id
}

// stageWrite writes into a staging as a WRITE with a special stateid does,
// while a CLOSE holds its locks.
func stageWrite(t *testing.T, store staging.Store, id vfs.InodeID, offset uint64, data string) {
	t.Helper()
	sf := store.Get(id)
	if sf == nil {
		t.Error("no staging to write")
		return
	}
	if err := sf.Write(offset, []byte(data)); err != nil {
		t.Error(err)
	}
	if err := sf.Sync(); err != nil {
		t.Error(err)
	}
}

func mustPublished(t *testing.T, fs vfs.TernVFS, name, want string) vfs.InodeID {
	t.Helper()
	if data, err := readPublished(fs, name); err != nil || string(data) != want {
		t.Fatalf("%s holds %q (%v), want %q", name, data, err, want)
	}
	id, err := fs.Lookup(fs.RootID(), name)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustCloseStatus(t *testing.T, f *publishFixture, fh []byte, sid [16]byte, want uint32, what string) {
	t.Helper()
	if got := closeFileStatus(t, f.conn, f.xid, fh, sid); got != want {
		t.Fatalf("%s: CLOSE %s, want %s", what, nfs4.Nfsstat4Name(got), nfs4.Nfsstat4Name(want))
	}
}

// A CLOSE whose publication the backend does not answer loses nothing: it
// keeps the open and the staging, records the candidate in the sidecar and
// answers DELAY, as many times as the backend stays silent, here also for
// the retry's resolution of the candidate. Once the backend answers, the
// retry scraps the candidate, which proves it was never linked and keeps
// it from ever linking, and only then publishes through a fresh transient.
func TestClosePublicationTimeoutDelaysAndRetries(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "saved")
	meta, _ := f.store.GetMeta(stagingID)
	f.fs.timeoutLinks(1, false)
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "unanswered link")
	if n := f.srv.opens.StateCount(); n != 1 {
		t.Fatalf("%d opens after the delayed CLOSE, want the open kept", n)
	}
	if saved, ok := f.store.GetMeta(stagingID); !ok || saved.Candidate.ID != stagingID {
		t.Fatalf("sidecar candidate %+v (%t), want the staging's transient", saved.Candidate, ok)
	}
	mustPublished(t, f.fs.TernVFS, "doc", "")
	f.fs.set(func() { f.fs.scrapTimeouts = 1 })
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "unanswered resolution")
	if n := f.srv.metrics.publishDelays.Load(); n != 2 {
		t.Fatalf("publish delays = %d, want 2", n)
	}
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry")
	published := mustPublished(t, f.fs.TernVFS, "doc", "saved")
	f.fs.mu.Lock()
	events := f.fs.events
	f.fs.mu.Unlock()
	want := []string{fmt.Sprintf("scrap %d", stagingID), fmt.Sprintf("construct %d", published), fmt.Sprintf("link %d", published)}
	if fmt.Sprint(events) != fmt.Sprint(want) {
		t.Fatalf("backend calls %v, want %v", events, want)
	}
	// The abandoned transient can never be linked now.
	if err := f.fs.TernVFS.LinkFile(stagingID, meta.TernCookie, f.fs.RootID(), "doc", bytes.NewReader([]byte("late"))); err == nil {
		t.Fatal("the fenced candidate linked late")
	}
	f.mustDrained(t, "after the publishing CLOSE")
}

// A link that applied before the backend's answer was lost: the retry
// proves it linked, by ScrapFile reporting not-exist and Stat succeeding,
// and publishes nothing twice. A Stat with no answer delays once more.
func TestClosePublicationAppliedButUnansweredPublishesOnce(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "saved")
	f.fs.timeoutLinks(1, true)
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "applied, unanswered link")
	f.fs.set(func() { f.fs.statTimeouts = 1 })
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "unanswered Stat")
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry")
	if published := mustPublished(t, f.fs.TernVFS, "doc", "saved"); published != stagingID {
		t.Fatalf("published inode %v, want the staging %v", published, stagingID)
	}
	if c, l, sc := f.fs.count("construct"), f.fs.count("link"), f.fs.count("scrap"); c != 0 || l != 1 || sc != 0 {
		t.Fatalf("constructs %d links %d scraps %d, want 0, 1, 0", c, l, sc)
	}
	f.mustDrained(t, "after the publishing CLOSE")
}

// A permanent publication error keeps its answer: the candidate is fenced,
// the open ends, the staging goes, and the CLOSE reports the error
// (TestFailedCloseReplaysFromSlot pins the replay).
func TestClosePublicationPermanentErrorStillFails(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "saved")
	f.fs.onLink(func(int, vfs.InodeID, vfs.Cookie, func() error) error { return errors.New("injected link rejection") })
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_IO, "rejected link")
	if got := f.fs.count("scrap"); got != 1 {
		t.Fatalf("%d scraps, want the candidate %v fenced", got, stagingID)
	}
	f.mustDrained(t, "after the failed CLOSE, want none")
}

// A WRITE acknowledged after a submission that applied and timed out is
// published by the retry, through a fresh transient.
func TestCloseRetryPublishesWriteAfterAppliedLink(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "saved")
	f.fs.timeoutLinks(1, true)
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "applied, unanswered link")
	writeFileAt(t, f.conn, f.xid, fh, sid, 0, []byte("later"))
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry")
	mustPublished(t, f.fs.TernVFS, "doc", "later")
	if got := f.fs.count("construct"); got != 1 {
		t.Fatalf("%d constructions, want one fresh candidate", got)
	}
}

// A retry whose earlier attempt applied and was then replaced by another
// client's save completes without publishing the older bytes again.
func TestCloseRetryAfterForeignPublicationDoesNotRepublish(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "mine")
	f.fs.timeoutLinks(1, true)
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "applied, unanswered link")
	conn, xid, other := f.otherClient(t)
	osid, ofh := openWriteFile(t, conn, xid, other, "doc")
	writeFileAt(t, conn, xid, ofh, osid, 0, []byte("theirs"))
	closeFile(t, conn, xid, ofh, osid)
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry")
	mustPublished(t, f.fs.TernVFS, "doc", "theirs")
	if got := f.fs.count("construct"); got != 1 {
		t.Fatalf("%d constructions, want only the other client's: %v", got, f.fs.events)
	}
}

// Times set after a submission that applied do not change the data
// version, so the retry publishes nothing again; the CLOSE still restores
// the times on the published file. A guard for the data version.
func TestCloseRetryAfterTimesOnlySetattrDoesNotRepublish(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "saved")
	f.fs.timeoutLinks(1, true)
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "applied, unanswered link")
	if got := setMtimeStatus(t, f.conn, f.xid, fh, sid, 1_000_000); got != nfs4.NFS4_OK {
		t.Fatalf("SETATTR mtime %s", nfs4.Nfsstat4Name(got))
	}
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry")
	if published := mustPublished(t, f.fs.TernVFS, "doc", "saved"); published != stagingID {
		t.Fatalf("published %v, want the linked candidate %v", published, stagingID)
	}
	if got := f.fs.count("construct"); got != 0 {
		t.Fatalf("%d constructions after a times-only change", got)
	}
	if info, err := f.fs.TernVFS.Stat(stagingID); err != nil || info.Mtime.Unix() != 1_000_000 {
		t.Fatalf("published mtime %v %v", info.Mtime, err)
	}
}

// LinkFile reporting not-exist for every candidate, as for a directory
// removed natively, ends the CLOSE with NOENT after one fresh candidate.
func TestCloseLinkNotExistEndsAfterOneFreshCandidate(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "saved")
	f.fs.onLink(func(int, vfs.InodeID, vfs.Cookie, func() error) error { return os.ErrNotExist })
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_NOENT, "links not-exist")
	if c, l := f.fs.count("construct"), f.links(); c != 1 || l != 2 {
		t.Fatalf("constructs %d links %d, want 1 and 2", c, l)
	}
	f.mustDrained(t, "after the failed CLOSE")
}

func (f *publishFixture) links() int {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()
	return f.fs.links
}

// A staging past its transient deadline, whose LinkFile reports not-exist
// without linking, publishes through a fresh candidate in the same CLOSE.
func TestCloseStagingPastDeadlinePublishesFresh(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "saved")
	f.fs.onLink(func(n int, _ vfs.InodeID, _ vfs.Cookie, link func() error) error {
		if n == 1 {
			return os.ErrNotExist
		}
		return link()
	})
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "stale staging")
	mustPublished(t, f.fs.TernVFS, "doc", "saved")
	// LocalTernVFS can reuse a scrapped transient's inode number, so count.
	if got := f.fs.count("construct"); got != 1 || f.fs.count("scrap") != 1 {
		t.Fatalf("backend calls %v, want the staging scrapped and one fresh candidate", f.fs.events)
	}
}

// A FILE_SYNC WRITE during the fresh candidate's upload, whose link then
// reports not-exist although it applied, gives DELAY; the retry publishes
// the WRITE.
func TestCloseWriteDuringFreshUploadDelays(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "saved")
	f.fs.onLink(func(n int, _ vfs.InodeID, _ vfs.Cookie, link func() error) error {
		switch n {
		case 1:
			return os.ErrNotExist
		case 2:
			if err := link(); err != nil {
				return err
			}
			stageWrite(t, f.store, stagingID, 0, "SAVED")
			return os.ErrNotExist
		}
		return link()
	})
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "WRITE during the upload")
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry")
	mustPublished(t, f.fs.TernVFS, "doc", "SAVED")
	if got := f.fs.count("construct"); got != 2 {
		t.Fatalf("%d constructions, want one per attempt", got)
	}
}

// A WRITE with a special stateid while the publication restores the times
// gives DELAY, and the retry publishes it.
func TestCloseWriteDuringSetTimeDelays(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "saved")
	var once sync.Once
	f.fs.set(func() {
		f.fs.onSetTime = func(vfs.InodeID) { once.Do(func() { stageWrite(t, f.store, stagingID, 0, "SAVED") }) }
	})
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "WRITE during SetTime")
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry")
	mustPublished(t, f.fs.TernVFS, "doc", "SAVED")
}

// A timestamp-only CLOSE also checks that no WRITE landed while it ran: one
// during its SetTime gives DELAY, and the retry publishes the data.
func TestTimestampOnlyCloseWriteDuringSetTimeDelays(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "saved")
	closeFile(t, f.conn, f.xid, fh, sid)
	sid, fh = openWriteFile(t, f.conn, f.xid, f.clientID, "doc")
	stagingID := stagingIDForTest(t, f.store, sid)
	if got := setMtimeStatus(t, f.conn, f.xid, fh, sid, 1_000_000); got != nfs4.NFS4_OK {
		t.Fatalf("SETATTR mtime %s", nfs4.Nfsstat4Name(got))
	}
	var once sync.Once
	f.fs.set(func() {
		f.fs.onSetTime = func(vfs.InodeID) { once.Do(func() { stageWrite(t, f.store, stagingID, 0, "SAVED") }) }
	})
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "WRITE during SetTime")
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry")
	mustPublished(t, f.fs.TernVFS, "doc", "SAVED")
}

// A link refused while the name is locked applied nothing: DELAY, and the
// retry publishes.
func TestCloseLockedLinkDelays(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "saved")
	f.fs.onLink(func(n int, _ vfs.InodeID, _ vfs.Cookie, link func() error) error {
		if n == 1 {
			return nfs4.Error(nfs4.NFS4ERR_LOCKED)
		}
		return link()
	})
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "locked name")
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry")
	mustPublished(t, f.fs.TernVFS, "doc", "saved")
}

// The candidate belongs to the staging, not to a request: a DELAYed CLOSE
// is finished by the owner's CLOSE under a newer generation, after a
// reopen, or by its OPEN_DOWNGRADE. A guard: the candidate is kept by
// staging, not by stateid.
func TestDelayedPublicationFinishedByOwnersCurrentStateid(t *testing.T) {
	for _, finish := range []string{"CLOSE", "OPEN_DOWNGRADE"} {
		t.Run(finish, func(t *testing.T) {
			f := newPublishFixture(t)
			sid, fh, _ := f.writer(t, "doc", "saved")
			f.fs.timeoutLinks(1, false)
			mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "unanswered link")
			// The same owner opens again: one stateid, a newer generation.
			status, newer, _ := openByHandle(t, f.conn, f.xid, f.clientID, "test-owner-doc", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
			if status != nfs4.NFS4_OK || newer == sid {
				t.Fatalf("reopen %s %x", nfs4.Nfsstat4Name(status), newer)
			}
			if finish == "CLOSE" {
				mustCloseStatus(t, f, fh, newer, nfs4.NFS4_OK, "newer generation")
			} else if got, _ := downgradeResult(t, f.conn, f.xid, fh, newer, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE); got != nfs4.NFS4_OK {
				t.Fatalf("OPEN_DOWNGRADE %s", nfs4.Nfsstat4Name(got))
			}
			mustPublished(t, f.fs.TernVFS, "doc", "saved")
			if got := f.fs.count("construct"); got != 1 {
				t.Fatalf("%d constructions, want one fresh candidate", got)
			}
		})
	}
}

// A timestamp-only CLOSE whose Lookup or SetTime gets no answer gives DELAY
// with nothing changed; a permanent error ends the writer: the readers go
// back to the published file, the staging is discarded and the open ends.
func TestTimestampOnlyCloseBackendErrors(t *testing.T) {
	for _, fault := range []string{"Lookup timeout", "SetTime timeout", "SetTime rejected"} {
		t.Run(fault, func(t *testing.T) {
			f := newPublishFixture(t)
			sid, fh, _ := f.writer(t, "doc", "saved")
			closeFile(t, f.conn, f.xid, fh, sid)
			published := mustPublished(t, f.fs.TernVFS, "doc", "saved")
			sid, fh = openWriteFile(t, f.conn, f.xid, f.clientID, "doc")
			if got := setMtimeStatus(t, f.conn, f.xid, fh, sid, 1_000_000); got != nfs4.NFS4_OK {
				t.Fatalf("SETATTR mtime %s", nfs4.Nfsstat4Name(got))
			}
			f.reader(t, "doc")
			f.fs.set(func() {
				switch fault {
				case "Lookup timeout":
					f.fs.lookupTimeout = "doc"
				case "SetTime timeout":
					f.fs.setTimeErr = msgs.TIMEOUT
				case "SetTime rejected":
					f.fs.setTimeErr = os.ErrPermission
				}
			})
			if fault == "SetTime rejected" {
				mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_ACCESS, fault)
				if n := len(f.store.Entries()); n != 0 {
					t.Fatalf("%d staging entries after the ended writer", n)
				}
				states := f.srv.opens.ClientStates(incarnationForTest(t, f.srv, f.clientID))
				if len(states) != 1 || states[0].Writes() || states[0].DataID != published {
					t.Fatalf("opens after the ended writer %+v, want the reader on %v", states, published)
				}
				return
			}
			mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, fault)
			if n := len(f.store.Entries()); n != 1 {
				t.Fatalf("%d staging entries after the delay, want the staging kept", n)
			}
			f.fs.set(func() { f.fs.lookupTimeout, f.fs.setTimeErr = "", nil })
			mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry")
			if info, err := f.fs.Stat(published); err != nil || info.Mtime.Unix() != 1_000_000 {
				t.Fatalf("published mtime %v %v", info.Mtime, err)
			}
		})
	}
}

// setMtimeStatus sets the mtime through stateid and returns the status.
func setMtimeStatus(t *testing.T, conn net.Conn, xid *uint32, fh []byte, sid [16]byte, seconds uint64) uint32 {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		putfhOp(w, fh)
		sw := w.AppendArgarray_Setattr()
		setStateid(sw.Stateid(), sid)
		aw := sw.StartObjAttributes()
		value := binary.BigEndian.AppendUint32(nil, nfs4.SET_TO_CLIENT_TIME4)
		value = binary.BigEndian.AppendUint64(value, seconds)
		value = binary.BigEndian.AppendUint32(value, 0)
		sw.Resume(finishTestFattr(aw, attrMask{0, 1 << (nfs4.FATTR4_TIME_MODIFY_SET - 32)}, value))
		w.Resume(sw.Finish())
	})
	*xid++
	return res.Status()
}

// A failed publication fences its candidate before failing the staging for
// its readers: the scrap runs while the staging is still healthy.
func TestFailedPublicationFencesBeforeQuarantine(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "saved")
	reader := f.reader(t, "doc")
	fencedHealthy := false
	f.fs.set(func() {
		f.fs.onScrap = func(id vfs.InodeID) error {
			if id == stagingID {
				fencedHealthy = !f.store.Failed(stagingID)
			}
			return nil
		}
	})
	f.fs.onLink(func(int, vfs.InodeID, vfs.Cookie, func() error) error { return errors.New("injected link rejection") })
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_IO, "rejected link")
	if !fencedHealthy {
		t.Fatal("the candidate was not fenced before the quarantine")
	}
	if status, _ := readStateid(t, f.conn, f.xid, fh, reader); status != nfs4.NFS4ERR_IO {
		t.Fatalf("reader of the failed staging %s, want IO", nfs4.Nfsstat4Name(status))
	}
}

// REMOVE and RENAME of a name whose writer has a pending candidate scrap
// it before they look the name up, so that a late link cannot bring the
// name back or land at the old name. A scrap with no answer gives DELAY
// and changes nothing.
func TestNamespaceChangeFencesPendingCandidate(t *testing.T) {
	for _, op := range []string{"REMOVE", "RENAME"} {
		t.Run(op, func(t *testing.T) {
			f := newPublishFixture(t)
			sid, fh, stagingID := f.writer(t, "doc", "saved")
			f.fs.timeoutLinks(1, false)
			mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "unanswered link")
			body := removeBody("doc")
			if op == "RENAME" {
				body = renameBody("doc", "moved")
			}
			f.fs.set(func() { f.fs.scrapTimeouts = 1 })
			if got := compoundStatus(t, sendBody(t, f.conn, f.xid, body)); got != nfs4.NFS4ERR_DELAY {
				t.Fatalf("%s with an unanswered fence %s, want DELAY", op, nfs4.Nfsstat4Name(got))
			}
			if meta, ok := f.store.GetMeta(stagingID); !ok || meta.Guarded || meta.Unlinked || meta.FileName != "doc" {
				t.Fatalf("delayed %s changed the writer: %+v %t", op, meta, ok)
			}
			if got := compoundStatus(t, sendBody(t, f.conn, f.xid, body)); got != nfs4.NFS4_OK {
				t.Fatalf("%s %s", op, nfs4.Nfsstat4Name(got))
			}
			if got := f.fs.count("scrap"); got != 1 {
				t.Fatalf("%d scraps, want the candidate fenced", got)
			}
		})
	}
}

// A writer quarantined by a namespace change of unknown outcome has its
// pending candidate scrapped first.
func TestQuarantineFencesPendingCandidate(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "saved")
	f.fs.timeoutLinks(1, false)
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "unanswered link")
	f.fs.set(func() { f.fs.namespaceErr = msgs.TIMEOUT })
	if got := compoundStatus(t, sendBody(t, f.conn, f.xid, removeBody("doc"))); got != nfs4.NFS4ERR_IO {
		t.Fatalf("REMOVE of unknown outcome %s, want IO", nfs4.Nfsstat4Name(got))
	}
	f.fs.set(func() { f.fs.namespaceErr = nil })
	if !f.store.Failed(stagingID) {
		t.Fatal("the writer was not quarantined")
	}
	// Once, before the REMOVE's lookup.
	if got := f.fs.count("scrap"); got != 1 {
		t.Fatalf("%d scraps reached the backend", got)
	}
}

// A reader of the writer's staging reads what the CLOSE published through
// a fresh transient, also a READ that chose the staging before the CLOSE
// removed it.
func TestReaderReadsFreshPublication(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "saved")
	reader := f.reader(t, "doc")
	f.fs.onLink(func(n int, _ vfs.InodeID, _ vfs.Cookie, link func() error) error {
		if n == 1 {
			return os.ErrNotExist
		}
		return link()
	})
	gate := &getGateStore{Store: f.store, armed: stagingID, entered: make(chan struct{}), release: make(chan struct{})}
	defer testutil.CloseSignal(gate.release)
	f.srv.stagingStore = gate
	// The READ goes on a second connection of the same client.
	conn := dial(t, f.addr)
	defer conn.Close()
	xid := uint32(9000)
	if id := setupClient(t, conn, &xid); id != f.clientID {
		t.Fatalf("second connection is client %d, want %d", id, f.clientID)
	}
	read := make(chan string, 1)
	go func() {
		status, data := readStateid(t, conn, &xid, fh, reader)
		if status != nfs4.NFS4_OK {
			t.Errorf("READ racing the CLOSE %s", nfs4.Nfsstat4Name(status))
		}
		read <- data
	}()
	testutil.AwaitSignal(t, gate.entered, "the READ holding the staging")
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "fresh publication")
	testutil.CloseSignal(gate.release)
	if data := testutil.AwaitValue(t, read, "the READ"); data != "saved" {
		t.Fatalf("READ that chose the staging read %q", data)
	}
	if published := mustPublished(t, f.fs.TernVFS, "doc", "saved"); published == stagingID {
		t.Fatal("published the staging itself, not a fresh candidate")
	}
	if _, data := readStateid(t, f.conn, f.xid, fh, reader); data != "saved" {
		t.Fatalf("reader read %q after the publication", data)
	}
}

// getGateStore holds, once armed, a READ's Get of one staging inode, with
// the staging it found, until released.
type getGateStore struct {
	staging.Store
	mu               sync.Mutex
	armed            vfs.InodeID
	entered, release chan struct{}
}

func (g *getGateStore) Get(id vfs.InodeID) staging.File {
	sf := g.Store.Get(id)
	g.mu.Lock()
	hold := g.armed != 0 && g.armed == id && calledFrom("opRead")
	if hold {
		g.armed = 0
	}
	g.mu.Unlock()
	if hold {
		close(g.entered)
		<-g.release
	}
	return sf
}

// delayedWriter creates name as owner "a", writes data and gets DELAY from
// its CLOSE, whose link the backend does not answer; the open is then
// closing.
func (f *publishFixture) delayedWriter(t *testing.T, name, data string) ([16]byte, []byte, vfs.InodeID) {
	t.Helper()
	sid, fh, stagingID := f.writer(t, name, data)
	f.fs.timeoutLinks(1, false)
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "unanswered link")
	f.fs.onLink(nil)
	return sid, fh, stagingID
}

// A writer whose CLOSE answered DELAY, for its link, its base hydration or
// its timestamp-only update, which libnfs gives up, is not handed a later
// writer's staging: another owner joining it is bound to the sidecar, and
// its CLOSE ends the staging, publishing, updating times or failing, and
// expires the closing open. A truncating OPEN afterwards finds no writer
// left on a removed staging.
func TestDelayedCloseJoinedByAnotherOwner(t *testing.T) {
	for _, ending := range []string{"publish", "hydration", "times", "fail"} {
		t.Run(ending, func(t *testing.T) {
			f := newPublishFixture(t)
			var first [16]byte
			var fh []byte
			var stagingID vfs.InodeID
			switch ending {
			case "hydration":
				sid, wfh, _ := f.writer(t, "doc", "0123456789")
				closeFile(t, f.conn, f.xid, wfh, sid)
				first, fh = openWriteFile(t, f.conn, f.xid, f.clientID, "doc")
				stagingID = stagingIDForTest(t, f.store, first)
				f.fs.set(func() { f.fs.readTimeouts = 1 << 20 })
				writeFileAt(t, f.conn, f.xid, fh, first, 0, []byte("AB"))
				mustCloseStatus(t, f, fh, first, nfs4.NFS4ERR_DELAY, "unanswered hydration")
				f.fs.set(func() { f.fs.readTimeouts = 0 })
			case "times":
				sid, wfh, _ := f.writer(t, "doc", "base")
				closeFile(t, f.conn, f.xid, wfh, sid)
				first, fh = openWriteFile(t, f.conn, f.xid, f.clientID, "doc")
				stagingID = stagingIDForTest(t, f.store, first)
				if got := setMtimeStatus(t, f.conn, f.xid, fh, first, 1_000_000); got != nfs4.NFS4_OK {
					t.Fatalf("SETATTR %s", nfs4.Nfsstat4Name(got))
				}
				f.fs.set(func() { f.fs.lookupTimeout = "doc" })
				mustCloseStatus(t, f, fh, first, nfs4.NFS4ERR_DELAY, "unanswered Lookup")
				f.fs.set(func() { f.fs.lookupTimeout = "" })
			default:
				first, fh, stagingID = f.delayedWriter(t, "doc", "first")
			}
			status, joiner, _, _ := openFileForOwner(t, f.conn, f.xid, f.clientID, "b", "doc", nfs4.OPEN4_SHARE_ACCESS_BOTH, false, false)
			if status != nfs4.NFS4_OK {
				t.Fatalf("joining OPEN %s", nfs4.Nfsstat4Name(status))
			}
			if meta, ok := f.store.GetMeta(stagingID); !ok || meta.NFSStateID != extractStateID(stateidOf(joiner)) || meta.OpenOwner != "b" {
				t.Fatalf("sidecar %+v %t, want it bound to the joiner", meta, ok)
			}
			want := uint32(nfs4.NFS4_OK)
			switch ending {
			case "publish", "hydration":
				writeFileAt(t, f.conn, f.xid, fh, joiner, 0, []byte("SECOND"))
			case "fail":
				f.fs.onLink(func(int, vfs.InodeID, vfs.Cookie, func() error) error { return errors.New("injected link rejection") })
				want = nfs4.NFS4ERR_IO
			}
			mustCloseStatus(t, f, fh, joiner, want, "joiner's CLOSE")
			f.fs.onLink(nil)
			switch ending {
			case "publish":
				mustPublished(t, f.fs.TernVFS, "doc", "SECOND")
			case "hydration":
				mustPublished(t, f.fs.TernVFS, "doc", "SECOND6789")
			case "times":
				if info, err := f.fs.TernVFS.Stat(mustPublished(t, f.fs.TernVFS, "doc", "base")); err != nil || info.Mtime.Unix() != 1_000_000 {
					t.Fatalf("published mtime %v %v", info.Mtime, err)
				}
			}
			if f.srv.opens.Live(extractStateID(stateidOf(first))) {
				t.Fatal("the closing open outlived the staging")
			}
			mustCloseStatus(t, f, fh, first, nfs4.NFS4ERR_EXPIRED, "closing open's retry")
			res := sendCompound(t, f.conn, *f.xid, func(w *nfs4.COMPOUND4argsWriter) { openWithSize(w, f.clientID, "doc", "c", 0) })
			*f.xid++
			if res.Status() != nfs4.NFS4_OK {
				t.Fatalf("truncating OPEN %s", nfs4.Nfsstat4Name(res.Status()))
			}
		})
	}
}

// A detached staging whose remaining writer CLOSEs is not handed to an
// open whose own CLOSE answered DELAY: that open is expired and the image
// discarded.
func TestDetachedStagingNotHandedToClosingWriter(t *testing.T) {
	f := newPublishFixture(t)
	first, fh, _ := f.delayedWriter(t, "doc", "first")
	status, joiner, _, _ := openFileForOwner(t, f.conn, f.xid, f.clientID, "b", "doc", nfs4.OPEN4_SHARE_ACCESS_BOTH, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatalf("joining OPEN %s", nfs4.Nfsstat4Name(status))
	}
	conn, xid, _ := f.otherClient(t)
	removeFile(t, conn, xid, "doc")
	mustCloseStatus(t, f, fh, joiner, nfs4.NFS4_OK, "joiner's CLOSE of the detached staging")
	if n := len(f.store.Entries()); n != 0 {
		t.Fatalf("%d staging entries, want the image discarded", n)
	}
	if f.srv.opens.Live(extractStateID(stateidOf(first))) {
		t.Fatal("the closing open kept the image")
	}
}

// A truncating OPEN that would join a writer whose staging failed answers
// IO, as an operation on that writer does.
func TestTruncatingJoinOfFailedWriterIsIO(t *testing.T) {
	f := newPublishFixture(t)
	f.writer(t, "doc", "saved")
	f.fs.set(func() { f.fs.namespaceErr = msgs.TIMEOUT })
	if got := compoundStatus(t, sendBody(t, f.conn, f.xid, removeBody("doc"))); got != nfs4.NFS4ERR_IO {
		t.Fatalf("REMOVE of unknown outcome %s, want IO", nfs4.Nfsstat4Name(got))
	}
	f.fs.set(func() { f.fs.namespaceErr = nil })
	res := sendCompound(t, f.conn, *f.xid, func(w *nfs4.COMPOUND4argsWriter) { openWithSize(w, f.clientID, "doc", "c", 0) })
	*f.xid++
	if res.Status() != nfs4.NFS4ERR_IO {
		t.Fatalf("truncating OPEN joining the failed writer %s, want IO", nfs4.Nfsstat4Name(res.Status()))
	}
}

// A read-only OPEN4_CREATE that sets a size owns its staging: its CLOSE
// publishes the initialized file and leaves no staging.
func TestReadOnlyCreateWithSizePublishes(t *testing.T) {
	f := newPublishFixture(t)
	size := uint64(5)
	sid, fh := openCreateFileWithSizeAndAccess(t, f.conn, f.xid, f.clientID, "lock", &size, nfs4.OPEN4_SHARE_ACCESS_READ)
	closeFile(t, f.conn, f.xid, fh, sid)
	mustPublished(t, f.fs.TernVFS, "lock", "\x00\x00\x00\x00\x00")
	if n := len(f.store.Entries()); n != 0 {
		t.Fatalf("%d staging entries after the creator's CLOSE", n)
	}
}

// replaceOnLookupVFS replaces one name with new content right after the
// first Lookup of it returns, as a CLOSE publishing over it then would.
type replaceOnLookupVFS struct {
	vfs.TernVFS
	name string
	once sync.Once
}

func (fs *replaceOnLookupVFS) Lookup(dir vfs.InodeID, name string) (vfs.InodeID, error) {
	id, err := fs.TernVFS.Lookup(dir, name)
	if name == fs.name {
		fs.once.Do(func() {
			if _, err := fs.TernVFS.CreateFile(dir, name, bytes.NewReader([]byte("replaced"))); err != nil {
				panic(err)
			}
		})
	}
	return id, err
}

// A read-only OPEN4_CREATE of an existing name that finds the name
// replaced once it holds the file lock opens again, after releasing its
// pathname lock, instead of waiting on it.
func TestReadOnlyCreateReopenAfterReplacement(t *testing.T) {
	root := t.TempDir()
	writeRoot(t, root, "doc", "original")
	fs := &replaceOnLookupVFS{TernVFS: vfs.NewLocalTernVFS(root), name: "doc"}
	_, addr, cleanup := startTestServerWithVFS(t, fs)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	done := make(chan uint32, 1)
	go func() {
		status, _, _, _ := openFileForOwner(t, conn, &xid, clientID, "o", "doc", nfs4.OPEN4_SHARE_ACCESS_READ, true, false)
		done <- status
	}()
	if got := testutil.AwaitValue(t, done, "the read-only OPEN4_CREATE"); got != nfs4.NFS4_OK {
		t.Fatalf("OPEN %s", nfs4.Nfsstat4Name(got))
	}
}

// A timestamp-only CLOSE points the staging's readers back at their own
// file, the version they opened, even when another client has published
// at the name since: they do not move to that version. A guard.
func TestTimestampOnlyCloseRetargetsReadersToTheirFile(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "base")
	closeFile(t, f.conn, f.xid, fh, sid)
	fh = lookupFH(t, f.conn, f.xid, "doc")
	_, w, _ := openByHandle(t, f.conn, f.xid, f.clientID, "w", fh, nfs4.OPEN4_SHARE_ACCESS_WRITE)
	_, r, _ := openByHandle(t, f.conn, f.xid, f.clientID, "r", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
	if got := setMtimeStatus(t, f.conn, f.xid, fh, w, 1_000_000); got != nfs4.NFS4_OK {
		t.Fatalf("SETATTR %s", nfs4.Nfsstat4Name(got))
	}
	conn, xid, other := f.otherClient(t)
	osid, ofh := openWriteFile(t, conn, xid, other, "doc")
	writeFileAt(t, conn, xid, ofh, osid, 0, []byte("OTHER"))
	closeFile(t, conn, xid, ofh, osid)
	mustCloseStatus(t, f, fh, w, nfs4.NFS4_OK, "timestamp-only")
	if status, data := readStateid(t, f.conn, f.xid, fh, r); status != nfs4.NFS4_OK || data != "base" {
		t.Fatalf("reader after the timestamp-only CLOSE %s %q, want the version it opened", nfs4.Nfsstat4Name(status), data)
	}
}

// Times set without a staging of their own, with the zero stateid or
// through a read open that borrows its client's writer, while that
// writer's CLOSE publishes wait for the CLOSE, which holds the file lock
// from reading the staging's times to removing it, and then reach the
// published file.
func TestStagingLessTimesWaitForWritersClose(t *testing.T) {
	for _, borrowed := range []bool{false, true} {
		t.Run(fmt.Sprintf("borrowed=%v", borrowed), func(t *testing.T) {
			f := newPublishFixture(t)
			sid, fh, _ := f.writer(t, "doc", "saved")
			var stateid [16]byte
			if borrowed {
				stateid = f.reader(t, "doc")
			}
			other := dial(t, f.addr)
			defer other.Close()
			oxid := uint32(1)
			setupClient(t, other, &oxid)
			gate := testutil.NewGate()
			defer testutil.CloseSignal(gate.Release)
			f.fs.set(func() { f.fs.onSetTime = func(vfs.InodeID) { gate.Wait() } })
			closed := make(chan uint32, 1)
			go func() {
				xid := uint32(7000)
				closed <- closeFileStatus(t, f.conn, &xid, fh, sid)
			}()
			testutil.AwaitSignal(t, gate.Entered, "the CLOSE restoring the times")
			set := make(chan uint32, 1)
			go func() { set <- setMtimeStatus(t, other, &oxid, fh, stateid, 1_000_000) }()
			testutil.AssertBlocked(t, set, "SETATTR behind the CLOSE")
			testutil.CloseSignal(gate.Release)
			if got := testutil.AwaitValue(t, closed, "the CLOSE"); got != nfs4.NFS4_OK {
				t.Fatalf("CLOSE %s", nfs4.Nfsstat4Name(got))
			}
			if got := testutil.AwaitValue(t, set, "the SETATTR"); got != nfs4.NFS4_OK {
				t.Fatalf("SETATTR %s", nfs4.Nfsstat4Name(got))
			}
			if info, err := f.fs.Stat(mustPublished(t, f.fs.TernVFS, "doc", "saved")); err != nil || info.Mtime.Unix() != 1_000_000 {
				t.Fatalf("published mtime %v %v", info.Mtime, err)
			}
		})
	}
}

// A candidate the attempt's own resolution has scrapped is not fenced
// again: when every link reports not-exist, the CLOSE ends with NOENT even
// if a further scrap would get no answer. A guard.
func TestCloseResolvedCandidateIsNotFencedAgain(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "saved")
	f.fs.onLink(func(int, vfs.InodeID, vfs.Cookie, func() error) error { return os.ErrNotExist })
	scraps := 0
	f.fs.set(func() {
		f.fs.onScrap = func(vfs.InodeID) error {
			if scraps++; scraps > 2 {
				return msgs.TIMEOUT
			}
			return nil
		}
	})
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_NOENT, "links not-exist")
	if n := len(f.store.Entries()); n != 0 {
		t.Fatalf("%d staging entries after the failed CLOSE", n)
	}
}

// bindFailStore refuses the Binds refuse picks.
type bindFailStore struct {
	staging.Store
	refuse func(owner string, readOnly bool) bool
}

func (s bindFailStore) Bind(id vfs.InodeID, clientID uint64, stateID nfs4.StateID, owner string, readOnly bool) error {
	if s.refuse(owner, readOnly) {
		return errors.New("injected bind failure")
	}
	return s.Store.Bind(id, clientID, stateID, owner, readOnly)
}

// A detached staging whose hand-over to its reader cannot be saved fails
// for that reader, which then gets IO rather than NOENT.
func TestDetachedHandoffFailureFailsReader(t *testing.T) {
	f := newPublishFixtureWithStore(t, func(s staging.Store) staging.Store {
		return bindFailStore{s, func(string, bool) bool { return true }}
	})
	sid, fh, _ := f.writer(t, "doc", "granted")
	reader := f.reader(t, "doc")
	conn, oxid, _ := f.otherClient(t)
	removeFile(t, conn, oxid, "doc")
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "writer's CLOSE of the detached staging")
	if status, data := readStateid(t, f.conn, f.xid, fh, reader); status != nfs4.NFS4ERR_IO {
		t.Fatalf("reader after the failed hand-over %s %q, want IO", nfs4.Nfsstat4Name(status), data)
	}
}

// A detached staging is not handed to a read open whose CLOSE answered
// DELAY: its client may have given it up. A guard.
func TestDetachedStagingNotHandedToClosingReader(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "granted")
	reader := f.reader(t, "doc")
	f.srv.opens.MarkClosing(extractStateID(stateidOf(reader)))
	conn, xid, _ := f.otherClient(t)
	removeFile(t, conn, xid, "doc")
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "writer's CLOSE of the detached staging")
	if n := len(f.store.Entries()); n != 0 {
		t.Fatalf("%d staging entries, want the image discarded", n)
	}
}

// A writer retired because its client is gone, here replaced by a reboot,
// has its pending candidate scrapped, by a CREATE at its name.
func TestRetiredWriterCandidateIsFenced(t *testing.T) {
	f := newPublishFixture(t)
	_, _, stagingID := f.delayedWriter(t, "doc", "first")
	f.clientID = setupClientWithVerifier(t, f.conn, f.xid, [8]byte{9})
	if got := compoundStatus(t, sendBody(t, f.conn, f.xid, mkdirBody("doc"))); got != nfs4.NFS4ERR_EXIST {
		t.Fatalf("CREATE over the name %s, want EXIST", nfs4.Nfsstat4Name(got))
	}
	if meta, ok := f.store.GetMeta(stagingID); !ok || !meta.Retired {
		t.Fatalf("writer %+v %t, want it retired", meta, ok)
	}
	if got := f.fs.count("scrap"); got != 1 {
		t.Fatalf("%d scraps, want the pending candidate fenced", got)
	}
}

// A joining OPEN that fails after binding the sidecar to itself gives the
// sidecar back to the closing open. A guard.
func TestFailedJoinRestoresClosingOwner(t *testing.T) {
	f := newPublishFixture(t)
	first, _, stagingID := f.delayedWriter(t, "doc", "first")
	// Occupy the stateid the joining OPEN will be given, so that its
	// opens.Open fails after the bind.
	next := f.srv.opens.NewStateID()
	binary.BigEndian.PutUint64(next[4:], binary.BigEndian.Uint64(next[4:])+1)
	if _, err := f.srv.opens.Open(openstate.OwnerKey{ClientID: 1, Owner: "squatter"}, 1, 1, openstate.Grant{Access: nfs4.OPEN4_SHARE_ACCESS_READ}, next); err != nil {
		t.Fatal(err)
	}
	if status, _, _, _ := openFileForOwner(t, f.conn, f.xid, f.clientID, "b", "doc", nfs4.OPEN4_SHARE_ACCESS_BOTH, false, false); status == nfs4.NFS4_OK {
		t.Fatal("joining OPEN succeeded on an occupied stateid")
	}
	if meta, ok := f.store.GetMeta(stagingID); !ok || meta.NFSStateID != extractStateID(stateidOf(first)) || meta.OpenOwner != "test-owner-doc" {
		t.Fatalf("sidecar %+v %t, want it the closing open's again", meta, ok)
	}
}

// A clean CLOSE leaves no transient behind once its reply is written. A
// guard.
func TestCleanCloseLeavesNoTransient(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "saved")
	closeFile(t, f.conn, f.xid, fh, sid)
	sid, fh = openWriteFile(t, f.conn, f.xid, f.clientID, "doc")
	closeFile(t, f.conn, f.xid, fh, sid)
	awaitRequestHoldsReleased(t, f.srv)
	if n := f.fs.TernVFS.(*vfs.LocalTernVFS).TransientCount(); n != 0 {
		t.Fatalf("%d transients after the clean CLOSE", n)
	}
}

// A first link that applies, with a WRITE during its upload, and then
// answers an error that is neither the backend's silence nor not-exist:
// resolution finds it linked but stale, and the CLOSE publishes the WRITE
// through one fresh transient.
func TestAppliedLinkErrorWithWriteDuringUploadPublishesFresh(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "saved")
	f.fs.onLink(func(n int, _ vfs.InodeID, _ vfs.Cookie, link func() error) error {
		if n != 1 {
			return link()
		}
		if err := link(); err != nil {
			return err
		}
		stageWrite(t, f.store, stagingID, 0, "LATER")
		return errors.New("injected error after the link applied")
	})
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "stale applied link")
	mustPublished(t, f.fs.TernVFS, "doc", "LATER")
	if got := f.fs.count("construct"); got != 1 {
		t.Fatalf("%d constructions, want one fresh candidate", got)
	}
}

// A fresh candidate's link that applies but answers an error that is
// neither the backend's silence nor not-exist is resolved as linked: the
// CLOSE reports the publication.
func TestFreshLinkAppliedWithErrorIsPublication(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "saved")
	f.fs.onLink(func(n int, _ vfs.InodeID, _ vfs.Cookie, link func() error) error {
		switch n {
		case 1:
			return msgs.TIMEOUT
		case 2:
			if err := link(); err != nil {
				return err
			}
			return errors.New("injected error after the link applied")
		}
		return link()
	})
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4ERR_DELAY, "unanswered link")
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "fresh link applied with an error")
	mustPublished(t, f.fs.TernVFS, "doc", "saved")
}

// A detached writer downgraded to READ whose image cannot be bound to it
// reads IO, not NOENT or other bytes.
func TestDetachedDowngradeBindFailureReadsIO(t *testing.T) {
	f := newPublishFixtureWithStore(t, func(s staging.Store) staging.Store {
		return bindFailStore{s, func(_ string, readOnly bool) bool { return readOnly }}
	})
	s0, fh0, _ := f.writer(t, "doc", "base-version")
	closeFile(t, f.conn, f.xid, fh0, s0)
	fh := lookupFH(t, f.conn, f.xid, "doc")
	openByHandle(t, f.conn, f.xid, f.clientID, "o", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
	_, sid, _ := openByHandle(t, f.conn, f.xid, f.clientID, "o", fh, nfs4.OPEN4_SHARE_ACCESS_WRITE)
	writeFileAt(t, f.conn, f.xid, fh, sid, 0, []byte("GRANTED"))
	conn, oxid, _ := f.otherClient(t)
	removeFile(t, conn, oxid, "doc")
	status, read := downgradeResult(t, f.conn, f.xid, fh, sid, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE)
	if status != nfs4.NFS4_OK {
		t.Fatalf("downgrade %s", nfs4.Nfsstat4Name(status))
	}
	if status, data := readStateid(t, f.conn, f.xid, fh, read); status != nfs4.NFS4ERR_IO {
		t.Fatalf("READ after the failed bind %s %q, want IO", nfs4.Nfsstat4Name(status), data)
	}
}

// A join whose bind fails answers IO and leaves the joining owner's
// existing marked open, and its marker, alone.
func TestJoinBindFailureKeepsExistingOpen(t *testing.T) {
	f := newPublishFixtureWithStore(t, func(s staging.Store) staging.Store {
		return bindFailStore{s, func(owner string, _ bool) bool { return owner == "b" }}
	})
	s0, fh0, _ := f.writer(t, "doc", "base")
	closeFile(t, f.conn, f.xid, fh0, s0)
	fh := lookupFH(t, f.conn, f.xid, "doc")
	// b: READ, then WRITE, then a clean downgrade: a marked read open.
	openByHandle(t, f.conn, f.xid, f.clientID, "b", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
	_, bw, _ := openByHandle(t, f.conn, f.xid, f.clientID, "b", fh, nfs4.OPEN4_SHARE_ACCESS_WRITE)
	status, bread := downgradeResult(t, f.conn, f.xid, fh, bw, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE)
	if status != nfs4.NFS4_OK {
		t.Fatalf("b's downgrade %s", nfs4.Nfsstat4Name(status))
	}
	_, a, _ := openByHandle(t, f.conn, f.xid, f.clientID, "a", fh, nfs4.OPEN4_SHARE_ACCESS_BOTH)
	writeFileAt(t, f.conn, f.xid, fh, a, 0, []byte("AAAA"))
	f.fs.timeoutLinks(1, false)
	mustCloseStatus(t, f, fh, a, nfs4.NFS4ERR_DELAY, "a's unanswered link")
	f.fs.onLink(nil)
	if status, _, _ := openByHandle(t, f.conn, f.xid, f.clientID, "b", fh, nfs4.OPEN4_SHARE_ACCESS_WRITE); status != nfs4.NFS4ERR_IO {
		t.Fatalf("join with a failed bind %s, want IO", nfs4.Nfsstat4Name(status))
	}
	awaitRequestHoldsReleased(t, f.srv)
	if status, data := readStateid(t, f.conn, f.xid, fh, bread); status != nfs4.NFS4_OK || data != "base" {
		t.Fatalf("b's existing open after the failed join %s %q", nfs4.Nfsstat4Name(status), data)
	}
}

// An OPEN_DOWNGRADE that answers DELAY leaves its open ordinary: its
// client keeps the open and retries.
func TestDelayedDowngradeIsNotClosing(t *testing.T) {
	f := newPublishFixture(t)
	s0, fh0, _ := f.writer(t, "doc", "base")
	closeFile(t, f.conn, f.xid, fh0, s0)
	fh := lookupFH(t, f.conn, f.xid, "doc")
	openByHandle(t, f.conn, f.xid, f.clientID, "o", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
	_, sid, _ := openByHandle(t, f.conn, f.xid, f.clientID, "o", fh, nfs4.OPEN4_SHARE_ACCESS_WRITE)
	writeFileAt(t, f.conn, f.xid, fh, sid, 0, []byte("NEW"))
	f.fs.timeoutLinks(1, false)
	if status, _ := downgradeResult(t, f.conn, f.xid, fh, sid, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE); status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("downgrade %s, want DELAY", nfs4.Nfsstat4Name(status))
	}
	if f.srv.opens.Closing(extractStateID(stateidOf(sid))) {
		t.Fatal("a DELAYed downgrade marked its open closing")
	}
}

// closeWriteGate holds a WRITE after it selected staging, and CLOSE just
// before its final version check or just before removal.
type closeWriteGate struct {
	staging.Store
	holdWrite, holdSeal, holdRemove atomic.Bool
	writeEntered, writeRelease      chan struct{}
	closeEntered, closeRelease      chan struct{}
}

type closeWriteFile struct {
	staging.File
	gate *closeWriteGate
}

func (g *closeWriteGate) Get(id vfs.InodeID) staging.File {
	file := g.Store.Get(id)
	if file == nil {
		return nil
	}
	return &closeWriteFile{File: file, gate: g}
}

func (f *closeWriteFile) Write(offset uint64, data []byte) error {
	if f.gate.holdWrite.CompareAndSwap(true, false) {
		close(f.gate.writeEntered)
		<-f.gate.writeRelease
	}
	return f.File.Write(offset, data)
}

func (f *closeWriteFile) Seal(version uint64) bool {
	if f.gate.holdSeal.CompareAndSwap(true, false) {
		close(f.gate.closeEntered)
		<-f.gate.closeRelease
	}
	return f.File.Seal(version)
}

func (g *closeWriteGate) Remove(id vfs.InodeID) {
	if g.holdRemove.CompareAndSwap(true, false) {
		close(g.closeEntered)
		<-g.closeRelease
	}
	g.Store.Remove(id)
}

// A stable WRITE that has selected staging must either reach the final
// publication or fail. It cannot succeed after CLOSE's final version check
// and then have its bytes discarded.
func TestCloseFinalVersionCheckOrdersStableWrites(t *testing.T) {
	for _, ending := range []string{"upload", "clean", "times"} {
		for _, before := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/before-check=%t", ending, before), func(t *testing.T) {
				gate := &closeWriteGate{
					writeEntered: make(chan struct{}), writeRelease: make(chan struct{}),
					closeEntered: make(chan struct{}), closeRelease: make(chan struct{}),
				}
				f := newPublishFixtureWithStore(t, func(store staging.Store) staging.Store {
					gate.Store = store
					return gate
				})
				sid, fh, _ := f.writer(t, "doc", "old")
				if ending != "upload" {
					closeFile(t, f.conn, f.xid, fh, sid)
					fh = lookupFH(t, f.conn, f.xid, "doc")
					status, writer, _ := openByHandle(t, f.conn, f.xid, f.clientID, "writer", fh, nfs4.OPEN4_SHARE_ACCESS_WRITE)
					if status != nfs4.NFS4_OK {
						t.Fatal(nfs4.Nfsstat4Name(status))
					}
					sid = writer
					if ending == "times" {
						if got := setMtimeStatus(t, f.conn, f.xid, fh, sid, 123); got != nfs4.NFS4_OK {
							t.Fatal(nfs4.Nfsstat4Name(got))
						}
					}
				}
				conn := dial(t, f.addr)
				defer conn.Close()
				xid := uint32(9000)
				setupClient(t, conn, &xid)
				defer testutil.CloseSignal(gate.writeRelease)
				defer testutil.CloseSignal(gate.closeRelease)
				gate.holdWrite.Store(true)
				gate.holdSeal.Store(before)
				gate.holdRemove.Store(!before)
				written := make(chan uint32, 1)
				go func() {
					reply := sendBody(t, conn, &xid, writeBody(fh, sid, "new"))
					written <- compoundStatus(t, reply)
				}()
				testutil.AwaitSignal(t, gate.writeEntered, "WRITE that already selected staging")
				closed := make(chan uint32, 1)
				go func() { closed <- closeFileStatus(t, f.conn, f.xid, fh, sid) }()
				testutil.AwaitSignal(t, gate.closeEntered, "CLOSE finalization")
				testutil.CloseSignal(gate.writeRelease)
				writeStatus := testutil.AwaitValue(t, written, "stable WRITE reply")
				testutil.CloseSignal(gate.closeRelease)
				closeStatus := testutil.AwaitValue(t, closed, "CLOSE reply")
				if before {
					if writeStatus != nfs4.NFS4_OK || closeStatus != nfs4.NFS4ERR_DELAY {
						t.Fatalf("WRITE %s, CLOSE %s; want OK, DELAY", nfs4.Nfsstat4Name(writeStatus), nfs4.Nfsstat4Name(closeStatus))
					}
					mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "retry after concurrent write")
					mustPublished(t, f.fs, "doc", "new")
				} else {
					if writeStatus != nfs4.NFS4ERR_IO || closeStatus != nfs4.NFS4_OK {
						t.Fatalf("WRITE %s, CLOSE %s; want IO, OK", nfs4.Nfsstat4Name(writeStatus), nfs4.Nfsstat4Name(closeStatus))
					}
					mustPublished(t, f.fs, "doc", "old")
				}
			})
		}
	}
}
