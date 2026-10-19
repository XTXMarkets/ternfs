// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// A session ended by a panic keeps charging the replies its slots still hold.
func TestTerminalSessionKeepsSlotRepliesCharged(t *testing.T) {
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

func heldCompoundReply(t *testing.T, srv *Server, wire []byte, xid uint32) (*rpcRequest, []byte) {
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

// A replay whose reply is still held keeps the old reply generation charged:
// its successor cannot use that credit, and destroying the session leaves the
// charge until the replay is released.
func TestHeldReplayKeepsOldReplyCharged(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	wire := admissionSequence(body, ts, 0, 1, true)
	_, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	old := sess.slots[0].buffer
	oldCharge := old.charge
	req, held := heldCompoundReply(t, srv, wire, 2)
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

// An uncached reply stays charged as work until it is released, even after
// its session is destroyed.
func TestUncachedOutputChargedUntilReleased(t *testing.T) {
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
	req, output := heldCompoundReply(t, srv, uncached, 3)
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

type blockedWriteConn struct {
	net.Conn
	entered, release chan struct{}
	once             sync.Once
}

func (c *blockedWriteConn) Write(p []byte) (int, error) {
	c.once.Do(func() { close(c.entered); <-c.release })
	return c.Conn.Write(p)
}

// A reply stalled in the transport stays charged until it is written, even
// after its session is destroyed.
func TestStalledTransportKeepsOutputCharged(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	server, client := net.Pipe()
	gate := &blockedWriteConn{Conn: server, entered: make(chan struct{}), release: make(chan struct{})}
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

// CREATE_SESSION without a free session record or session id, and a request
// without work capacity, are refused before any effect.
func TestSessionAndWorkCapacityBeforeEffects(t *testing.T) {
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

// A request that does not fit the reply-work budget waits for releases
// rather than losing its connection; one whose connection closes while it
// waits gives up. A request larger than the whole budget is refused at once.
func TestReplyWorkWaitsForCapacity(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	client := setupClient(t, conn, &xid)
	ts := testSessionForClient(conn, client)
	ss := srv.sessions
	request := func(slot, seq uint32) *rpcRequest {
		wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, slot, seq, false)
		req, err := parseRPCCall(rpcCallForTest(xid, procCompound, wire))
		if err != nil {
			t.Fatal(err)
		}
		xid++
		return req
	}
	first := request(0, 1)
	if !first.acquireReplyWork(srv) {
		t.Fatal("first admission")
	}
	ss.cacheMu.Lock()
	ss.workLimit = ss.workBytes // Exactly one request fits.
	ss.cacheMu.Unlock()
	defer func() {
		ss.cacheMu.Lock()
		ss.workLimit = maxReplyWorkBytes
		ss.cacheMu.Unlock()
	}()

	second := request(1, 1)
	admitted := make(chan bool, 1)
	go func() { admitted <- second.acquireReplyWork(srv) }()
	testutil.AssertBlocked(t, admitted, "second request while the budget is full")
	first.releaseReply()
	if !testutil.AwaitValue(t, admitted, "second request after a release") {
		t.Fatal("second request refused after the release")
	}

	closed := make(chan struct{})
	third := request(2, 1)
	third.connection = &rpcConnection{closed: closed}
	go func() { admitted <- third.acquireReplyWork(srv) }()
	testutil.AssertBlocked(t, admitted, "third request while the budget is full")
	close(closed)
	ss.wakeWorkWaiters()
	if testutil.AwaitValue(t, admitted, "third request after its connection closed") {
		t.Fatal("a request on a closed connection was admitted")
	}
	second.releaseReply()

	ss.cacheMu.Lock()
	ss.workLimit = 1
	ss.cacheMu.Unlock()
	if request(3, 1).acquireReplyWork(srv) {
		t.Fatal("a request larger than the whole budget was admitted")
	}
}

// The reply-work charge follows what a compound can return, not the session's
// maximum response: a WRITE of 1 MiB reserves a few hundred KiB, a READ of
// 1 MiB reserves the full envelope, and a compound with READDIR keeps the
// session's response size because its result is sized from the remainder.
func TestReplyWorkChargeFollowsOperationBounds(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	client := setupClient(t, conn, &xid)
	ts := testSessionForClient(conn, client)
	charge := func(body func(w *nfs4.COMPOUND4argsWriter)) int {
		t.Helper()
		wire := admissionSequence(buildCompoundBody(nil, body), ts, 0, 1, false)
		req, err := parseRPCCall(rpcCallForTest(xid, procCompound, wire))
		if err != nil {
			t.Fatal(err)
		}
		xid++
		if !req.acquireReplyWork(srv) {
			t.Fatal("admission")
		}
		c := req.replyWork.charge
		req.releaseReply()
		return c
	}
	data := make([]byte, 1<<20)
	write := charge(func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ww := w.AppendArgarray_Write()
		ww.Stateid().SetSeqid(0)
		for i := 0; i < 12; i++ {
			ww.Stateid().SetOther(i, 0)
		}
		ww = ww.SetOffset(0)
		ww = ww.SetStable(0)
		ww = ww.SetData(data)
		w.Resume(ww.Finish())
	})
	read := charge(func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(1 << 20)
	})
	readdir := charge(func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		r := w.AppendArgarray_Readdir()
		r.SetMaxcount(1 << 20)
		r.SetDircount(1 << 20)
		mask := r.StartAttrRequest()
		r.Resume(mask.Finish())
		w.Resume(r.Finish())
	})
	if write > 8*(96<<10) {
		t.Fatalf("WRITE compound charged %d bytes, want a few hundred KiB", write)
	}
	if read < 8*(1<<20) {
		t.Fatalf("READ compound charged %d bytes, want at least the 1 MiB payload eightfold", read)
	}
	if readdir < 8*(1<<20) {
		t.Fatalf("READDIR compound charged %d bytes, want the session's response size eightfold", readdir)
	}
}
