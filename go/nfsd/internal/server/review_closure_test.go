// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func TestReviewN08R1PopulatedTerminalSlotsRemainCharged(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "victim"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	srv := admissionWritableServer(t, &admissionPanicVFS{TernVFS: vfs.NewLocalTernVFS(root)})
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	for slot := uint32(0); slot < 3; slot++ {
		wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, slot, 1, true)
		_, res := admissionRequest(t, srv, wire, slot+1, nil)
		admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	}
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh(); admissionRemove(w, "victim") }), ts, 3, 1, true)
	_, res := admissionRequest(t, srv, wire, 10, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_SERVERFAULT, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_REMOVE)
	retained := 0
	for i := range sess.slots {
		retained += cap(sess.slots[i].reply)
	}
	if srv.sessions.cachedBytes < retained {
		t.Fatalf("uncharged terminal arrays: retained=%d charged=%d", retained, srv.sessions.cachedBytes)
	}
	_, res = admissionRequest(t, srv, wire, 11, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_BADSESSION, nfs4.OP_SEQUENCE)
}

func reviewEmptySetattr(w *nfs4.COMPOUND4argsWriter) {
	op := w.AppendArgarray_Setattr()
	attrs := op.StartObjAttributes()
	mask := attrs.StartAttrmask()
	attrs.Resume(mask.Finish())
	attrs.Resume(attrs.StartAttrVals().SetData(nil).Finish())
	op.Resume(attrs.Finish())
	w.Resume(op.Finish())
}
func TestReviewN08R2GetfhFollowingErrorBoundary(t *testing.T) {
	for _, limit := range []uint32{140, 152} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			root := t.TempDir()
			fs := vfs.NewLocalTernVFS(root)
			srv := admissionWritableServer(t, fs)
			ts := connectionSession(t, srv)
			srv.sessions.lookup(ts.id).maxCached = limit
			wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				admissionCreate(w, "born")
				w.AppendArgarray_Getfh()
				reviewEmptySetattr(w)
			}), ts, 0, 1, true)
			reply, res := admissionRequest(t, srv, wire, 1, nil)
			iter := res.Resarray()
			iter.Next()
			iter.Next()
			iter.Next()
			create := iter.Resarray().Value().AsCREATE4resEntry()
			if create.Disc() == nfs4.NFS4_OK {
				if !iter.Next() || iter.Resarray().Disc() != nfs4.OP_GETFH || iter.Resarray().Value().AsGETFH4resEntry().Disc() != nfs4.NFS4_OK {
					t.Fatalf("CREATE ran without promised GETFH: status=%s bytes=%d", nfs4.Nfsstat4Name(res.Status()), len(reply))
				}
			} else {
				if _, err := fs.Lookup(fs.RootID(), "born"); !os.IsNotExist(err) {
					t.Fatalf("pre-effect rejection created object: %v", err)
				}
			}
			replay, _ := admissionRequest(t, srv, wire, 2, nil)
			if !bytes.Equal(reply[24:], replay[24:]) {
				t.Fatal("truthful prefix changed on replay")
			}
		})
	}
}
func TestReviewN08R2VariableBeforeGetfh(t *testing.T) {
	root := t.TempDir()
	fs := vfs.NewLocalTernVFS(root)
	srv := admissionWritableServer(t, fs)
	ts := connectionSession(t, srv)
	srv.sessions.lookup(ts.id).maxCached = 160
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		c := w.AppendArgarray_Create()
		c.Resume(c.SetObjtype_Nf4lnk().SetData(bytes.Repeat([]byte("x"), 128)).Finish())
		c.Resume(c.StartObjname().SetData([]byte("link")).Finish())
		a := c.StartCreateattrs()
		m := a.StartAttrmask()
		a.Resume(m.Finish())
		a.Resume(a.StartAttrVals().SetData(nil).Finish())
		c.Resume(a.Finish())
		w.Resume(c.Finish())
		w.AppendArgarray_Readlink()
		w.AppendArgarray_Getfh()
	}), ts, 0, 1, true)
	_, res := admissionRequest(t, srv, wire, 1, nil)
	if _, err := fs.Lookup(fs.RootID(), "link"); !os.IsNotExist(err) {
		t.Fatalf("unsafe variable path created link: %v, status=%s", err, nfs4.Nfsstat4Name(res.Status()))
	}
	admissionStatus(t, res, nfs4.NFS4ERR_UNSAFE_COMPOUND, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_CREATE)
}

func TestReviewN08R3UncachedRequiredErrors(t *testing.T) {
	for _, opcode := range []uint32{9999, nfs4.OP_OPEN_CONFIRM, nfs4.OP_RENEW, nfs4.OP_RELEASE_LOCKOWNER, nfs4.OP_SETCLIENTID_CONFIRM} {
		t.Run(fmt.Sprint(opcode), func(t *testing.T) {
			srv, _ := newConnectionTestServer(t, 2)
			ts := connectionSession(t, srv)
			body := buildCompoundBody(nil, nil)
			binary.BigEndian.PutUint32(body[8:], 1)
			body = binary.BigEndian.AppendUint32(body, opcode)
			switch opcode {
			case nfs4.OP_OPEN_CONFIRM:
				body = append(body, make([]byte, 20)...)
			case nfs4.OP_RENEW:
				body = append(body, make([]byte, 8)...)
			case nfs4.OP_RELEASE_LOCKOWNER:
				body = append(body, make([]byte, 12)...)
			case nfs4.OP_SETCLIENTID_CONFIRM:
				body = append(body, make([]byte, 16)...)
			}
			wire := admissionSequence(body, ts, 0, 1, false)
			reply, res := admissionRequest(t, srv, wire, 1, nil)
			expected, disc := uint32(nfs4.NFS4ERR_NOTSUPP), opcode
			if !knownOperation(opcode) {
				expected, disc = nfs4.NFS4ERR_OP_ILLEGAL, nfs4.OP_ILLEGAL
			}
			admissionStatus(t, res, expected, nfs4.OP_SEQUENCE, disc)
			before := admissionImage(&srv.sessions.lookup(ts.id).slots[0])
			retry, res := admissionRequest(t, srv, wire, 1001, nil)
			admissionStatus(t, res, expected, nfs4.OP_SEQUENCE, disc)
			if !bytes.Equal(reply[24:], retry[24:]) || !reflect.DeepEqual(before, admissionImage(&srv.sessions.lookup(ts.id).slots[0])) {
				t.Fatal("mandatory error replay changed identity/prefix")
			}
		})
	}
}

type reviewSyncFailStore struct {
	staging.Store
	armed bool
}
type reviewSyncFailFile struct{ staging.File }

func (s *reviewSyncFailStore) Get(id vfs.InodeID) staging.File {
	f := s.Store.Get(id)
	if f != nil && s.armed {
		return &reviewSyncFailFile{f}
	}
	return f
}
func (f *reviewSyncFailFile) Sync() error { return syscall.EIO }
func reviewOpenSize(w *nfs4.COMPOUND4argsWriter, client uint64, name, owner string, size uint64) {
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
func TestReviewN08R4JoinedOpenSizeSyncFailure(t *testing.T) {
	root := t.TempDir()
	stageDir := t.TempDir()
	base, err := staging.NewLocalStore(stageDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewSyncFailStore{Store: base}
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
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { reviewOpenSize(w, client, "joined", "second", 3) }), ts, slot, seq, true)
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

func reviewHeldReply(t *testing.T, srv *Server, wire []byte, xid uint32) (*rpcRequest, []byte) {
	t.Helper()
	req, err := parseRPCCall(rpcCallForTest(xid, procCompound, wire))
	if err != nil {
		t.Fatal(err)
	}
	reply := srv.safeHandleCompound(req, "held-reply")
	if _, err := parseCompoundReply(reply); err != nil {
		req.releaseReply()
		t.Fatal(err)
	}
	return req, reply
}
func TestReviewN08R1ReplaySuccessorDestructionOwnership(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	wire := admissionSequence(body, ts, 0, 1, true)
	_, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	old := sess.slots[0].buffer
	oldCharge := old.charge
	req, held := reviewHeldReply(t, srv, wire, 2)
	defer req.releaseReply()
	if a := srv.sessions.accounting(); a.CacheBuffers != 1 || a.WorkReplies != 1 || a.WorkBytes < cap(held) || req.replyWork.outputCapacity != cap(held) {
		t.Fatalf("held replay accounting %+v", a)
	}
	// A pinned old generation cannot fund its successor's cache allocation.
	srv.sessions.cacheLimit = int(sess.maxCached) + cachedReplyOverhead
	before := admissionImage(&sess.slots[0])
	next := admissionSequence(body, ts, 0, 2, true)
	_, res = admissionRequest(t, srv, next, 3, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_RESOURCE, nfs4.OP_SEQUENCE)
	if !reflect.DeepEqual(before, admissionImage(&sess.slots[0])) {
		t.Fatal("overlap quota failure changed slot")
	}
	srv.sessions.cacheLimit = maxSessionCacheBytes
	_, res = admissionRequest(t, srv, next, 4, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	if a := srv.sessions.accounting(); a.CacheBuffers != 2 || a.CacheBytes != oldCharge+sess.slots[0].buffer.charge || a.WorkReplies != 1 {
		t.Fatalf("successor lost old view charge %+v", a)
	}
	srv.sessions.destroySession(ts.id)
	if a := srv.sessions.accounting(); a.CacheBuffers != 1 || a.CacheBytes != oldCharge || a.WorkReplies != 1 {
		t.Fatalf("destruction lost replay ownership %+v", a)
	}
	if old.data == nil || len(held) == 0 {
		t.Fatal("replay view prematurely discarded")
	}
	held = nil
	req.releaseReply()
	if a := srv.sessions.accounting(); a.CacheBuffers != 0 || a.CacheBytes != 0 || a.WorkBytes != 0 || a.WorkReplies != 0 {
		t.Fatalf("final-use charge not released %+v", a)
	}
}

func TestReviewN08R1UncachedOutputAndExclusiveTransfer(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	first := admissionSequence(body, ts, 0, 1, true)
	_, res := admissionRequest(t, srv, first, 1, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	srv.sessions.cacheLimit = int(sess.maxCached) + cachedReplyOverhead // Exclusive credit fits.
	second := admissionSequence(body, ts, 0, 2, true)
	_, res = admissionRequest(t, srv, second, 2, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	uncached := admissionSequence(body, ts, 0, 3, false)
	req, output := reviewHeldReply(t, srv, uncached, 3)
	defer req.releaseReply()
	if a := srv.sessions.accounting(); a.WorkReplies != 1 || a.WorkBytes < cap(output) || a.CacheBytes < cap(sess.slots[0].reply)+cachedReplyOverhead {
		t.Fatalf("uncached output missing charge %+v", a)
	}
	srv.sessions.destroySession(ts.id)
	if a := srv.sessions.accounting(); a.CacheBytes != 0 || a.WorkReplies != 1 || a.WorkBytes < cap(output) {
		t.Fatalf("uncached output released during destruction %+v", a)
	}
	output = nil
	req.releaseReply()
	if a := srv.sessions.accounting(); a.WorkBytes != 0 || a.WorkReplies != 0 {
		t.Fatalf("work leak %+v", a)
	}
}

type reviewBlockedWriteConn struct {
	net.Conn
	entered, release chan struct{}
	once             sync.Once
}

func (c *reviewBlockedWriteConn) Write(p []byte) (int, error) {
	c.once.Do(func() { close(c.entered); <-c.release })
	return c.Conn.Write(p)
}
func TestReviewN08R1StalledTransportRetainsOutput(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	server, client := net.Pipe()
	gate := &reviewBlockedWriteConn{Conn: server, entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan struct{})
	go func() { defer close(done); srv.handleConn(gate) }()
	defer func() {
		testutil.CloseSignal(gate.release)
		client.Close()
		server.Close()
		testutil.AwaitSignal(t, done, "transport worker cleanup")
	}()
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	wire := admissionSequence(body, ts, 0, 1, false)
	writeCallForTest(t, client, rpcCallForTest(1, procCompound, wire))
	testutil.AwaitSignal(t, gate.entered, "stalled response")
	if a := srv.sessions.accounting(); a.WorkReplies != 1 || a.WorkBytes == 0 {
		t.Fatalf("stalled output not charged %+v", a)
	}
	next := admissionSequence(body, ts, 0, 2, true)
	_, res := admissionRequest(t, srv, next, 2, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	srv.sessions.destroySession(ts.id)
	if a := srv.sessions.accounting(); a.CacheBytes != 0 || a.WorkReplies != 1 || a.WorkBytes == 0 {
		t.Fatalf("stalled output released before write %+v", a)
	}
	testutil.CloseSignal(gate.release)
	client.SetReadDeadline(time.Now().Add(testutil.ChannelTimeout))
	if _, err := readFrame(client); err != nil {
		t.Fatal(err)
	}
	client.Close()
	testutil.AwaitSignal(t, done, "completed transport")
	if a := srv.sessions.accounting(); a.WorkBytes != 0 || a.WorkReplies != 0 {
		t.Fatalf("completed output retains charge %+v", a)
	}
}

// Retired sessions, including ones carrying an earlier epoch, stay
// BADSESSION for SEQUENCE and BIND without any record kept for them: with no
// revival there is nothing to deny (canonical r3 §6). Fresh sessions are
// unaffected.
func TestReviewN08R1RetiredSessionIDsStayBadSession(t *testing.T) {
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	addr, stop := serveTestServer(t, srv)
	defer stop()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	client := setupClient(t, conn, &xid)
	ss := srv.sessions
	var retired []SessionID
	for i := 0; i < 12; i++ {
		ts := connectionSession(t, srv)
		sess := ss.lookup(ts.id)
		id := sess.id
		binary.BigEndian.PutUint32(id[:4], ss.epoch^0x80000000)
		binary.BigEndian.PutUint64(id[4:12], client)
		moveSessionForTest(ss, sess, client, id)
		retired = append(retired, sess.id)
		ss.retireSession(sess, true)
		ss.destroySession(sess.id)
		if a := ss.accounting(); a.Sessions != 1 || a.CacheBytes != 0 {
			t.Fatalf("retired sessions retained %+v", a)
		}
	}
	for i, id := range retired {
		ts := newTestSession(id, client, maxSessionSlots)
		if seq := srv.startSequence(ReadCOMPOUND4argsForTest(t, admissionSequence(buildCompoundBody(nil, nil), ts, 0, 1, true))); seq.status != nfs4.NFS4ERR_BADSESSION {
			t.Fatalf("SEQUENCE on a retired ID %s", nfs4.Nfsstat4Name(seq.status))
		}
		_, res := admissionRequest(t, srv, admissionBind(id, nfs4.CDFC4_FORE, false), uint32(100+i), &rpcConnection{})
		admissionStatus(t, res, nfs4.NFS4ERR_BADSESSION, nfs4.OP_BIND_CONN_TO_SESSION)
	}
	if a := ss.accounting(); a.Sessions != 1 || a.Records != 1 {
		t.Fatalf("retired IDs rebuilt state %+v", a)
	}
	current := testSessionForClient(conn, client)
	_, status, err := createSession(conn, xid, client, current.nextCSSeq)
	if err != nil || status != nfs4.NFS4_OK {
		t.Fatalf("current-epoch creation blocked %s %v", nfs4.Nfsstat4Name(status), err)
	}
}

func TestReviewN08R1SessionAndWorkCapacityBeforeEffects(t *testing.T) {
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	addr, stop := serveTestServer(t, srv)
	defer stop()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	client := setupClient(t, conn, &xid)
	ts := testSessionForClient(conn, client)
	ss := srv.sessions
	ss.recordLimit = 1
	// A failed pre-slot reservation is DELAY and leaves the creation slot
	// unchanged (canonical r3 §5.1).
	_, res := admissionRequest(t, srv, createSessionBody(client, ts.nextCSSeq), 100, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_DELAY, nfs4.OP_CREATE_SESSION)
	if a := ss.accounting(); a.Sessions != 1 || a.PendingSessions != 0 {
		t.Fatalf("record admission changed state %+v", a)
	}
	sess := ss.lookup(ts.id)
	before := admissionImage(&sess.slots[0])
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, 0, 1, true)
	req, err := parseRPCCall(rpcCallForTest(1, procCompound, wire))
	if err != nil {
		t.Fatal(err)
	}
	ss.workLimit = 1
	if reply := srv.safeHandleCompound(req, "work-exhausted"); reply != nil {
		req.releaseReply()
		t.Fatal("work capacity overload produced unreserved output")
	}
	if !reflect.DeepEqual(before, admissionImage(&sess.slots[0])) {
		t.Fatal("work admission consumed slot")
	}
	if a := ss.accounting(); a.WorkBytes != 0 || a.WorkReplies != 0 {
		t.Fatalf("failed work admission charged %+v", a)
	}
	ss.workLimit = maxReplyWorkBytes
	ss.recordLimit = clientstore.MaxSessionRecords
	ss.mu.Lock()
	ss.next = uint64(^uint32(0))
	ss.mu.Unlock()
	_, res = admissionRequest(t, srv, createSessionBody(client, ts.nextCSSeq), 101, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_DELAY, nfs4.OP_CREATE_SESSION)
	if a := ss.accounting(); a.PendingSessions != 0 || a.Sessions != 1 {
		t.Fatalf("serial exhaustion changed records %+v", a)
	}
}

func TestReviewN08R2OpenFollowingGetfhBoundary(t *testing.T) {
	for _, limit := range []uint32{164, 176} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			root := t.TempDir()
			srv := admissionWritableServer(t, vfs.NewLocalTernVFS(root))
			addr, stop := serveTestServer(t, srv)
			defer stop()
			conn := dial(t, addr)
			defer conn.Close()
			xid := uint32(1)
			client := setupClient(t, conn, &xid)
			ts := testSessionForClient(conn, client)
			sess := srv.sessions.lookup(ts.id)
			sess.maxCached = limit
			wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				admissionOpen(w, client, "created")
				w.AppendArgarray_Getfh()
				reviewEmptySetattr(w)
			}), ts, 0, 1, true)
			reply, res := admissionRequest(t, srv, wire, 100, nil)
			it := res.Resarray()
			it.Next()
			it.Next()
			it.Next()
			open := it.Resarray().Value().AsOPEN4resEntry()
			if open.Disc() == nfs4.NFS4_OK {
				if !it.Next() || it.Resarray().Value().AsGETFH4resEntry().Disc() != nfs4.NFS4_OK {
					t.Fatal("OPEN lost admitted handle")
				}
			} else {
				if len(srv.stagingStore.Entries()) != 0 {
					t.Fatal("rejected OPEN installed staging")
				}
				if _, err := srv.fs.Lookup(srv.fs.RootID(), "created"); !os.IsNotExist(err) {
					t.Fatalf("rejected OPEN created file %v", err)
				}
			}
			replay, _ := admissionRequest(t, srv, wire, 101, nil)
			if !bytes.Equal(reply[24:], replay[24:]) {
				t.Fatal("OPEN prefix changed")
			}
		})
	}
}
func TestReviewN08R4DefiniteOpenErrorStaysReplayable(t *testing.T) {
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

// reviewPagedVFS serves a large directory in MTU-bounded pages, like
// RemoteTernVFS: each page decodes fresh names, and every entry has a name of
// the maximum length.
type reviewPagedVFS struct {
	vfs.TernVFS
	entries int
	sample  func()
	stats   atomic.Int32
}

func (fs *reviewPagedVFS) Readdir(dir vfs.InodeID, start uint64) ([]vfs.DirEntry, uint64, error) {
	if dir != fs.RootID() {
		return fs.TernVFS.Readdir(dir, start)
	}
	fs.sample()
	per := msgs.MAX_UDP_MTU / (8 + 8 + 1 + maxTernNameLength + 8)
	first := max(int(start), 3)
	var page []vfs.DirEntry
	for hash := first; hash < first+per && hash < fs.entries+3; hash++ {
		name := fmt.Sprintf("%0*d", maxTernNameLength, hash)
		page = append(page, vfs.DirEntry{Name: name, ID: vfs.MakeInodeID(vfs.InodeTypeFile, uint64(hash)), NameHash: uint64(hash)})
	}
	next := uint64(first + per)
	if int(next) >= fs.entries+3 {
		next = 0
	}
	return page, next, nil
}
func (fs *reviewPagedVFS) Stat(id vfs.InodeID) (vfs.NodeInfo, error) {
	if id.Type() != vfs.InodeTypeFile {
		return fs.TernVFS.Stat(id)
	}
	if fs.stats.Add(1)%16 == 0 {
		fs.sample()
	}
	return vfs.NodeInfo{Size: uint64(id)}, nil
}

func TestReviewN08R1aReaddirPreparationWithinReservation(t *testing.T) {
	fs := &reviewPagedVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()), entries: 12000}
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	sess.maxResp = 1 << 20
	readdir := func(cookie uint64) func(w *nfs4.COMPOUND4argsWriter) {
		return func(w *nfs4.COMPOUND4argsWriter) {
			w.AppendArgarray_Putrootfh()
			r := w.AppendArgarray_Readdir()
			r.SetCookie(cookie)
			r.SetMaxcount(1 << 20)
			r.SetDircount(1 << 20)
			bits := r.StartAttrRequest()
			bits.AppendData(supportedAttrs0)
			bits.AppendData(supportedAttrs1)
			bits.AppendData(supportedAttrs2)
			r.Resume(bits.Finish())
			w.Resume(r.Finish())
		}
	}
	wire := admissionSequence(buildCompoundBody(nil, readdir(0)), ts, 0, 1, false)
	req, err := parseRPCCall(rpcCallForTest(1, procCompound, wire))
	if err != nil {
		t.Fatal(err)
	}
	if !req.acquireReplyWork(srv) {
		t.Fatal("work admission")
	}
	var stats runtime.MemStats
	live := func() int {
		runtime.GC()
		runtime.ReadMemStats(&stats)
		return int(stats.HeapAlloc)
	}
	// Peak retained heap during preparation, against the reservation the call
	// actually holds. Samples follow a full collection, so only reachable
	// allocations count.
	base, peak := live(), 0
	fs.sample = func() { peak = max(peak, live()-base) }
	reply := srv.safeHandleCompound(req, "readdir-peak")
	work := req.replyWork.charge
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	reply = nil
	req.releaseReply()
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_READDIR)
	if peak == 0 || peak > work {
		t.Fatalf("READDIR preparation retained %d bytes, reservation %d", peak, work)
	}
	// The READDIR-specific obligation is the writer plus its explicit allowance;
	// it does not rely on the multiplier's general slack.
	if allowance := int(sess.maxResp) + readdirPrepareBytes; peak > allowance+allowance/8 {
		t.Fatalf("READDIR preparation retained %d bytes, allowance %d", peak, allowance)
	}
	t.Logf("peak %d, writer+allowance %d, reservation %d", peak, int(sess.maxResp)+readdirPrepareBytes, work)

	// The streamed page is the exact largest prefix and continues correctly.
	it := res.Resarray()
	it.Next()
	it.Next()
	it.Next()
	ok := it.Resarray().Value().AsREADDIR4resEntry().Value().AsREADDIR4resok()
	entrySize := 4 + 8 + 4 + maxTernNameLength + 1 + 16 + 4 + 312 + 4
	want := (min(1<<20, int(sess.maxResp)-rpcAcceptedHeaderSize-8-60-8-8) - readdirResokOverhead) / entrySize
	got, last := 0, uint64(0)
	if ok.Reply().Eof() != nfs4.FALSE {
		t.Fatal("partial page reported EOF")
	}
	for e := ok.Reply().Entries(); e.Disc() == nfs4.TRUE; {
		entry := e.AsEntry4()
		if entry.Cookie() != uint64(got+3) || len(entry.Name().Data()) != maxTernNameLength {
			t.Fatalf("entry %d cookie %d", got, entry.Cookie())
		}
		last = entry.Cookie()
		got++
		e = entry.Nextentry()
	}
	if got < want-1 || got > want {
		t.Fatalf("page has %d entries, want about %d", got, want)
	}
	wire = admissionSequence(buildCompoundBody(nil, readdir(last)), ts, 0, 2, false)
	_, res = admissionRequest(t, srv, wire, 2, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_READDIR)
	it = res.Resarray()
	it.Next()
	it.Next()
	it.Next()
	ok = it.Resarray().Value().AsREADDIR4resEntry().Value().AsREADDIR4resok()
	if e := ok.Reply().Entries(); e.Disc() != nfs4.TRUE || e.AsEntry4().Cookie() != last+1 {
		t.Fatal("continuation did not resume after the last cookie")
	}
	if a := srv.sessions.accounting(); a.WorkBytes != 0 || a.WorkReplies != 0 {
		t.Fatalf("READDIR work not released %+v", a)
	}
}

func TestReviewN08R1bClientRetirementHoldsRecords(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ss := srv.sessions
	const client, count = 77, 64
	ss.recordLimit = count
	srv.clients.PinLeaseForTest(client)
	var batch []*session
	ss.mu.Lock()
	for i := 0; i < count; i++ {
		sess := &session{clientID: client, slots: make([]sessionSlot, maxSessionSlots),
			maxOps: maxCompoundOperations, maxReq: maxRPCMessage, maxResp: maxRPCMessage,
			maxCached: maxSessionReplyCached}
		sess.id = ss.newSessionIDLocked(client)
		ss.insertSessionLocked(sess)
		batch = append(batch, sess)
	}
	ss.mu.Unlock()
	// One continuing user: an admitted request still running on the last session.
	user := batch[count-1]
	ts := newTestSession(user.id, client, maxSessionSlots)
	seq := srv.startSequence(ReadCOMPOUND4argsForTest(t, admissionSequence(buildCompoundBody(nil, nil), ts, 1, 1, true)))
	if seq.status != nfs4.NFS4_OK || seq.slot == nil {
		t.Fatalf("admission %s", nfs4.Nfsstat4Name(seq.status))
	}
	// Pause every session's slot so the bulk retirement stalls wherever it starts.
	for _, sess := range batch {
		sess.slots[0].mu.Lock()
	}
	done := make(chan struct{})
	go func() {
		ss.dropClient(client)
		close(done)
	}()
	waitFor := func(what string, cond func(replyAccounting) bool) replyAccounting {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); ; {
			a := ss.accounting()
			if cond(a) {
				return a
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: %+v", what, a)
			}
			time.Sleep(time.Millisecond)
		}
	}
	// The batch is detached from lookup yet its records stay counted.
	waitFor("batch detached", func(a replyAccounting) bool { return a.Sessions == 0 })
	testutil.AssertBlocked(t, done, "client-wide retirement")
	if a := ss.accounting(); a.Records != count {
		t.Fatalf("stalled batch returned record credit %+v", a)
	}
	if _, ok := ss.reserveSessionRecord(client + 1); ok {
		t.Fatal("new record admitted against a stalled retiring batch")
	}
	if seq := srv.startSequence(ReadCOMPOUND4argsForTest(t, admissionSequence(buildCompoundBody(nil, nil),
		newTestSession(SessionID{0xff}, client, maxSessionSlots), 0, 1, true))); seq.status != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("unknown session %s", nfs4.Nfsstat4Name(seq.status))
	}
	// Retirement drains in map order. Each released slot can return at most its
	// own session's record, and never the continuing user's.
	for i, sess := range batch {
		time.Sleep(time.Millisecond)
		if a := ss.accounting(); a.Records < count-i || a.Records > count {
			t.Fatalf("%d of %d drained, records %+v", i, count, a)
		}
		sess.slots[0].mu.Unlock()
	}
	<-done
	// The admitted request still owns its drained session's record.
	if a := ss.accounting(); a.Records != 1 || a.Sessions != 0 || a.Groups != 1 || a.ActiveRequests != 1 {
		t.Fatalf("continuing user lost its record or group %+v", a)
	}
	ss.recordLimit = 1
	if _, ok := ss.reserveSessionRecord(client + 1); ok {
		t.Fatal("record admitted while a request still uses its retired session")
	}
	seq.finish(nil)
	if a := ss.accounting(); a.Records != 0 || a.CacheBytes != 0 || a.CacheBuffers != 0 || a.Groups != 1 || a.GroupCredits != 1 {
		t.Fatalf("retirement did not release %+v", a)
	}
	// The transport's hold outlives the slot's use, and keeps its group
	// credit, until the reply is done.
	seq.hold.release()
	if a := ss.accounting(); a.Groups != 0 || a.GroupCredits != 0 || a.ActiveRequests != 0 {
		t.Fatalf("group outlived its last request %+v", a)
	}
	if _, ok := ss.reserveSessionRecord(client + 1); !ok {
		t.Fatal("released record capacity not reusable")
	}
	ss.releaseSessionRecord()
	// A retired session cannot be re-admitted after its last use ended.
	if seq := srv.startSequence(ReadCOMPOUND4argsForTest(t, admissionSequence(buildCompoundBody(nil, nil), ts, 2, 1, true))); seq.status != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("retired session admitted %s", nfs4.Nfsstat4Name(seq.status))
	}
}

func ReadCOMPOUND4argsForTest(t *testing.T, wire []byte) nfs4.SEQUENCE4args {
	t.Helper()
	args, ok := nfs4.ReadCOMPOUND4args(wire)
	it := args.Argarray()
	if !ok || !it.Next() {
		t.Fatal("SEQUENCE arguments")
	}
	return it.Argarray().Value().AsSEQUENCE4args()
}
