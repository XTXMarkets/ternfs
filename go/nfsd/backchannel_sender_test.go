// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// cbTestClient is a client connection that also serves callbacks: its
// reader separates the server's CALLs from replies to its own requests.
type cbTestClient struct {
	t       *testing.T
	conn    net.Conn
	mu      sync.Mutex
	replies map[uint32]chan []byte
	calls   chan []byte
	writeMu sync.Mutex
	// order records frame arrivals: a reply's XID, or 0 for a callback.
	order []uint32
	// program is the callback program the client registered; every CALL
	// is checked against it (cbCallHeaderError).
	program uint32
}

// cbCallHeaderError checks a callback CALL as the client's callback
// service would: ONC RPC version 2, the session's callback program, program
// version 1 (NFS_CB in the official XDR), CB_NULL or CB_COMPOUND, and for
// CB_COMPOUND minor version 1 with callback_ident 0.
func cbCallHeaderError(call []byte, program uint32) string {
	if len(call) < 40 {
		return "short CALL"
	}
	field := func(off int) uint32 { return binary.BigEndian.Uint32(call[off:]) }
	switch {
	case field(8) != rpcVersion:
		return fmt.Sprintf("rpcvers %d", field(8))
	case field(12) != program:
		return fmt.Sprintf("program %#x, want %#x", field(12), program)
	case field(16) != 1:
		return fmt.Sprintf("program version %d, want 1", field(16))
	case field(20) != cbProcNull && field(20) != cbProcCompound:
		return fmt.Sprintf("procedure %d", field(20))
	}
	if field(20) == cbProcCompound {
		args := cbArgs(call)
		if len(args) < 12 {
			return "short CB_COMPOUND"
		}
		tag := int((binary.BigEndian.Uint32(args) + 3) &^ 3)
		if len(args) < 4+tag+8 || binary.BigEndian.Uint32(args[4+tag:]) != 1 ||
			binary.BigEndian.Uint32(args[8+tag:]) != 0 {
			return "CB_COMPOUND minor version or callback_ident"
		}
	}
	return ""
}

func newCBTestClient(t *testing.T, addr string) *cbTestClient {
	t.Helper()
	c := &cbTestClient{t: t, conn: dial(t, addr), replies: make(map[uint32]chan []byte),
		calls: make(chan []byte, 64), program: 0x40000001}
	t.Cleanup(func() { c.conn.Close() })
	go func() {
		for {
			frame, err := readFrame(c.conn)
			if err != nil {
				return
			}
			if binary.BigEndian.Uint32(frame[4:8]) == rpcCall {
				c.mu.Lock()
				c.order = append(c.order, 0)
				program := c.program
				c.mu.Unlock()
				if msg := cbCallHeaderError(frame, program); msg != "" {
					t.Errorf("invalid callback CALL: %s", msg)
				}
				c.calls <- frame
				continue
			}
			xid := binary.BigEndian.Uint32(frame[0:4])
			c.mu.Lock()
			c.order = append(c.order, xid)
			ch := c.replies[xid]
			delete(c.replies, xid)
			c.mu.Unlock()
			if ch != nil {
				ch <- frame
			}
		}
	}()
	return c
}

func (c *cbTestClient) write(frame []byte) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := writeFrame(c.conn, frame); err != nil {
		c.t.Fatal(err)
	}
}

func (c *cbTestClient) compound(xid uint32, body []byte) nfs4.COMPOUND4res {
	c.t.Helper()
	ch := make(chan []byte, 1)
	c.mu.Lock()
	c.replies[xid] = ch
	c.mu.Unlock()
	c.write(rpcCallForTest(xid, procCompound, body))
	reply := testutil.AwaitValue(c.t, ch, "reply")
	res, err := parseCompoundReply(reply)
	if err != nil {
		c.t.Fatal(err)
	}
	return res
}

// nextCall returns the next callback CALL, or false after wait.
func (c *cbTestClient) nextCall(wait time.Duration) ([]byte, bool) {
	select {
	case call := <-c.calls:
		return call, true
	case <-time.After(wait):
		return nil, false
	}
}

// cbProc is a CALL's procedure.
func cbProc(call []byte) uint32 { return binary.BigEndian.Uint32(call[20:24]) }

// cbArgs returns a CALL's arguments, after its credential and verifier.
func cbArgs(call []byte) []byte {
	off := 28
	off += 4 + int((binary.BigEndian.Uint32(call[off:])+3)&^3)
	off += 4
	off += 4 + int((binary.BigEndian.Uint32(call[off:])+3)&^3)
	return call[off:]
}

// cbRecall decodes a CB_COMPOUND's CB_SEQUENCE and CB_RECALL_SLOT.
func cbRecall(t *testing.T, call []byte) (id SessionID, seq, slot, target uint32) {
	t.Helper()
	args := cbArgs(call)
	if cbProc(call) != cbProcCompound || binary.BigEndian.Uint32(args[4:]) != 1 ||
		binary.BigEndian.Uint32(args[12:]) != 2 || binary.BigEndian.Uint32(args[16:]) != nfs4.OP_CB_SEQUENCE {
		t.Fatalf("not a CB_SEQUENCE+CB_RECALL_SLOT compound: %x", args)
	}
	copy(id[:], args[20:36])
	seq, slot = binary.BigEndian.Uint32(args[36:]), binary.BigEndian.Uint32(args[40:])
	if binary.BigEndian.Uint32(args[56:]) != nfs4.OP_CB_RECALL_SLOT {
		t.Fatal("second operation is not CB_RECALL_SLOT")
	}
	return id, seq, slot, binary.BigEndian.Uint32(args[60:])
}

func (c *cbTestClient) answer(call, body []byte) {
	r := binary.BigEndian.AppendUint32(nil, binary.BigEndian.Uint32(call[0:4]))
	r = binary.BigEndian.AppendUint32(r, rpcReply)
	r = binary.BigEndian.AppendUint32(r, 0) // MSG_ACCEPTED
	r = binary.BigEndian.AppendUint32(r, authNone)
	r = binary.BigEndian.AppendUint32(r, 0)
	r = binary.BigEndian.AppendUint32(r, acceptSuccess)
	c.write(append(r, body...))
}

// answerSequence replies to a CB_COMPOUND with CB_SEQUENCE status and, for
// OK, the session, sequence and slot it named.
func (c *cbTestClient) answerSequence(t *testing.T, call []byte, status uint32) {
	t.Helper()
	id, seq, slot, _ := cbRecall(t, call)
	b := binary.BigEndian.AppendUint32(nil, status)
	b = binary.BigEndian.AppendUint32(b, 0) // tag
	if status != nfs4.NFS4_OK {
		b = binary.BigEndian.AppendUint32(b, 1)
		b = binary.BigEndian.AppendUint32(b, nfs4.OP_CB_SEQUENCE)
		c.answer(call, binary.BigEndian.AppendUint32(b, status))
		return
	}
	b = binary.BigEndian.AppendUint32(b, 2)
	b = binary.BigEndian.AppendUint32(b, nfs4.OP_CB_SEQUENCE)
	b = binary.BigEndian.AppendUint32(b, nfs4.NFS4_OK)
	b = append(b, id[:]...)
	b = binary.BigEndian.AppendUint32(b, seq)
	b = binary.BigEndian.AppendUint32(b, slot)
	b = binary.BigEndian.AppendUint32(b, 0)
	b = binary.BigEndian.AppendUint32(b, 0)
	b = binary.BigEndian.AppendUint32(b, nfs4.OP_CB_RECALL_SLOT)
	c.answer(call, binary.BigEndian.AppendUint32(b, nfs4.NFS4_OK))
}

// answerNull answers the next callback, which must be CB_NULL.
func (c *cbTestClient) answerNull(t *testing.T) {
	t.Helper()
	call, ok := c.nextCall(testutil.ChannelTimeout)
	if !ok || cbProc(call) != cbProcNull {
		t.Fatalf("expected CB_NULL, got %v", ok)
	}
	c.answer(call, nil)
}

type cbSessionFixture struct {
	srv      *Server
	addr     string
	client   *cbTestClient
	ts       *testSession
	clientID uint64
	xid      uint32
	slotSeq  uint32
}

// newCBSession creates a session with a backchannel on a new connection.
// The CB_NULL that follows is left for the test.
func newCBSession(t *testing.T) *cbSessionFixture {
	t.Helper()
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	t.Cleanup(cleanup)
	return newCBSessionOn(t, srv, addr)
}

// newCBSessionOn is newCBSession on a server the test built.
func newCBSessionOn(t *testing.T, srv *Server, addr string) *cbSessionFixture {
	t.Helper()
	return newCBSessionLimits(t, srv, addr, maxRPCMessage, 16)
}

// newCBSessionLimits is newCBSessionOn with the back channel's request size
// and slot count given.
func newCBSessionLimits(t *testing.T, srv *Server, addr string, backReq, backReqs uint32) *cbSessionFixture {
	t.Helper()
	f := &cbSessionFixture{srv: srv, addr: addr, client: newCBTestClient(t, addr), xid: 1}
	res := f.client.compound(f.next(), exchangeIDBody("cb-sender", [8]byte{5}))
	it := res.Resarray()
	it.Next()
	ex := it.Resarray().Value().AsEXCHANGEID4resEntry().Value().AsEXCHANGEID4resok()
	f.clientID = ex.EirClientid()
	createXID := f.next()
	res = f.client.compound(createXID, backCreateSessionBodyLimits(f.clientID, ex.EirSequenceid(),
		nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN, backReq, backReqs, offerNone))
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(res.Status()))
	}
	f.client.mu.Lock()
	for _, xid := range f.client.order {
		if xid == createXID {
			break
		}
		if xid == 0 {
			t.Error("a callback arrived before the CREATE_SESSION reply")
		}
	}
	f.client.mu.Unlock()
	it = res.Resarray()
	it.Next()
	ok := it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok()
	if ok.CsrFlags()&nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN == 0 {
		t.Fatal("backchannel not granted")
	}
	f.ts = newTestSession(readSessionID(ok.CsrSessionid()), f.clientID, ok.CsrForeChanAttrs().CaMaxrequests())
	awaitLeaseProven(t, srv, incarnationForTest(t, srv, f.clientID))
	return f
}

func (f *cbSessionFixture) next() uint32 { f.xid++; return f.xid }

func (f *cbSessionFixture) session(t *testing.T) *session {
	t.Helper()
	sess := f.srv.sessions.lookup(f.ts.id)
	if sess == nil {
		t.Fatal("session gone")
	}
	return sess
}

// sequence sends SEQUENCE alone on slot 0 and returns its flags and target.
func (f *cbSessionFixture) sequence(t *testing.T) (flags, target uint32) {
	t.Helper()
	f.slotSeq++
	body := buildCompoundBody(nil, func(*nfs4.COMPOUND4argsWriter) {})
	res := f.client.compound(f.next(), withSequence(body, f.ts, 0, f.slotSeq))
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("SEQUENCE %s", nfs4.Nfsstat4Name(res.Status()))
	}
	it := res.Resarray()
	it.Next()
	ok := it.Resarray().Value().AsSEQUENCE4resEntry().Value().AsSEQUENCE4resok()
	return ok.SrStatusFlags(), ok.SrTargetHighestSlotid()
}

// pressure puts the reply cache under pressure, or relieves it, by a
// phantom charge, with enough idle sessions that this one is above its
// share.
func (f *cbSessionFixture) pressure(t *testing.T, on bool) {
	t.Helper()
	ss := f.srv.sessions
	charge := int(maxSessionReplyCached + cachedReplyOverhead)
	phantom := 10 * charge
	ss.cacheMu.Lock()
	if on {
		ss.cachedBytes += phantom
		ss.cacheLimit = ss.cachedBytes + 2*charge
	} else {
		ss.cachedBytes -= phantom
		ss.cacheLimit = maxSessionCacheBytes
	}
	ss.cacheMu.Unlock()
	if on {
		for len(f.srv.sessions.accountingSessions()) < 16 {
			connectionSession(t, f.srv)
		}
	}
}

func (ss *sessionStore) accountingSessions() map[SessionID]*session {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	out := make(map[SessionID]*session, len(ss.sessions))
	for id, sess := range ss.sessions {
		out[id] = sess
	}
	return out
}

func shortCallbackTimeout(t *testing.T, d time.Duration) {
	old, oldBackoff := cbTimeout, cbDelayBackoff
	cbTimeout, cbDelayBackoff = d, time.Millisecond
	t.Cleanup(func() { cbTimeout, cbDelayBackoff = old, oldBackoff })
}

// CB_NULL probes the new back connection; its reply marks the path up and
// its loss raises the path-down flags, which a late reply clears.
func TestCBNullMarksPath(t *testing.T) {
	shortCallbackTimeout(t, 100*time.Millisecond)
	f := newCBSession(t)
	call, ok := f.client.nextCall(testutil.ChannelTimeout)
	if !ok || cbProc(call) != cbProcNull || binary.BigEndian.Uint32(call[12:]) != 0x40000001 {
		t.Fatal("no CB_NULL to the callback program")
	}
	if flags, _ := f.sequence(t); flags != 0 {
		t.Fatalf("flags %#x while probing", flags)
	}
	time.Sleep(3 * cbTimeout)
	if flags, _ := f.sequence(t); flags != nfs4.SEQ4_STATUS_CB_PATH_DOWN_SESSION|nfs4.SEQ4_STATUS_CB_PATH_DOWN {
		t.Fatalf("flags %#x after a lost CB_NULL", flags)
	}
	f.client.answer(call, nil)
	waitFor(t, "path up", func() bool { flags, _ := f.sequence(t); return flags == 0 })
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(testutil.ChannelTimeout); !cond(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// Under reply-cache pressure a session above its share gets half its
// slots as target, and one CB_RECALL_SLOT on slot 0 with sequence 1. The
// target is restored below half full, and a later episode recalls again
// with the next sequence. An OK reply renews the lease.
func TestCBRecallSlotOncePerPressure(t *testing.T) {
	f := newCBSession(t)
	f.client.answerNull(t)
	highest := uint32(len(f.session(t).slots) - 1)
	f.pressure(t, true)
	if _, target := f.sequence(t); target != highest/2 {
		t.Fatalf("target %d under pressure, want %d", target, highest/2)
	}
	call, ok := f.client.nextCall(testutil.ChannelTimeout)
	if !ok {
		t.Fatal("no CB_RECALL_SLOT")
	}
	id, seq, slot, target := cbRecall(t, call)
	if id != f.ts.id || seq != 1 || slot != 0 || target != highest/2 {
		t.Fatalf("recall %x seq %d slot %d target %d", id, seq, slot, target)
	}
	before := time.Now()
	f.client.answerSequence(t, call, nfs4.NFS4_OK)
	f.sequence(t)
	if _, again := f.client.nextCall(100 * time.Millisecond); again {
		t.Fatal("recalled twice in one episode")
	}
	inc := incarnationForTest(t, f.srv, f.clientID)
	f.srv.sessions.mu.Lock()
	quiet := f.srv.sessions.groups[inc].quietUntil
	f.srv.sessions.mu.Unlock()
	if quiet.Before(before.Add(nfsLeaseTime - time.Second)) {
		t.Fatalf("callback did not renew: quiet until %s", quiet)
	}

	f.pressure(t, false)
	if _, target := f.sequence(t); target != highest {
		t.Fatalf("target %d after pressure, want %d", target, highest)
	}
	f.pressure(t, true)
	f.sequence(t)
	call, ok = f.client.nextCall(testutil.ChannelTimeout)
	if !ok {
		t.Fatal("no recall in the second episode")
	}
	if _, seq, _, _ := cbRecall(t, call); seq != 2 {
		t.Fatalf("second recall sequence %d, want 2", seq)
	}
	f.client.answerSequence(t, call, nfs4.NFS4_OK)
	f.pressure(t, false)
}

// An unanswered call is not resent while its connection is up. After a
// disconnect the same sequence goes out on another back connection, or
// waits for a BIND.
func TestCBResendOnlyAfterDisconnect(t *testing.T) {
	shortCallbackTimeout(t, 50*time.Millisecond)
	t.Run("other-connection", func(t *testing.T) {
		f := newCBSession(t)
		f.client.answerNull(t)
		second := newCBTestClient(t, f.addr)
		res := second.compound(1000, admissionBind(f.ts.id, nfs4.CDFC4_BACK_OR_BOTH, false))
		if res.Status() != nfs4.NFS4_OK {
			t.Fatalf("BIND %s", nfs4.Nfsstat4Name(res.Status()))
		}
		it := res.Resarray()
		it.Next()
		if dir := it.Resarray().Value().AsBINDCONNTOSESSION4resEntry().Value().AsBINDCONNTOSESSION4resok().BctsrDir(); dir != nfs4.CDFS4_BOTH {
			t.Fatalf("BIND direction %d", dir)
		}
		second.answerNull(t)
		f.pressure(t, true)
		f.sequence(t)
		call, ok := f.client.nextCall(testutil.ChannelTimeout)
		if !ok {
			t.Fatal("no recall")
		}
		_, seq, _, _ := cbRecall(t, call)
		if _, resent := f.client.nextCall(4 * cbTimeout); resent {
			t.Fatal("resent on the same connection while it is up")
		}
		if _, resent := second.nextCall(10 * time.Millisecond); resent {
			t.Fatal("resent on another connection before a disconnect")
		}
		f.client.conn.Close()
		call, ok = second.nextCall(testutil.ChannelTimeout)
		if !ok {
			t.Fatal("not resent after the disconnect")
		}
		if _, again, _, _ := cbRecall(t, call); again != seq {
			t.Fatalf("resent with sequence %d, want %d", again, seq)
		}
		second.answerSequence(t, call, nfs4.NFS4_OK)
		f.pressure(t, false)
	})
	t.Run("bind", func(t *testing.T) {
		f := newCBSession(t)
		f.client.answerNull(t)
		f.pressure(t, true)
		f.sequence(t)
		call, ok := f.client.nextCall(testutil.ChannelTimeout)
		if !ok {
			t.Fatal("no recall")
		}
		_, seq, _, _ := cbRecall(t, call)
		f.client.conn.Close()
		third := newCBTestClient(t, f.addr)
		if res := third.compound(2000, admissionBind(f.ts.id, nfs4.CDFC4_BACK, false)); res.Status() != nfs4.NFS4_OK {
			t.Fatalf("BIND %s", nfs4.Nfsstat4Name(res.Status()))
		}
		for {
			call, ok = third.nextCall(testutil.ChannelTimeout)
			if !ok {
				t.Fatal("not resent after the BIND")
			}
			if cbProc(call) == cbProcNull {
				third.answer(call, nil)
				continue
			}
			break
		}
		if _, again, _, _ := cbRecall(t, call); again != seq {
			t.Fatalf("resent with sequence %d, want %d", again, seq)
		}
		third.answerSequence(t, call, nfs4.NFS4_OK)
		f.pressure(t, false)
	})
}

// An actual DELAY reply is retried with the same sequence, up to three
// times; MISORDERED faults the backchannel.
func TestCBSequenceReplies(t *testing.T) {
	shortCallbackTimeout(t, time.Second)
	t.Run("delay", func(t *testing.T) {
		f := newCBSession(t)
		f.client.answerNull(t)
		f.pressure(t, true)
		f.sequence(t)
		var seqs []uint32
		for {
			call, ok := f.client.nextCall(200 * time.Millisecond)
			if !ok {
				break
			}
			_, seq, _, _ := cbRecall(t, call)
			seqs = append(seqs, seq)
			f.client.answerSequence(t, call, nfs4.NFS4ERR_DELAY)
		}
		if len(seqs) != 1+cbMaxDelays {
			t.Fatalf("sent %d times, want %d", len(seqs), 1+cbMaxDelays)
		}
		for _, seq := range seqs {
			if seq != 1 {
				t.Fatalf("retried with sequences %v", seqs)
			}
		}
		f.pressure(t, false)
	})
	t.Run("misordered", func(t *testing.T) {
		f := newCBSession(t)
		f.client.answerNull(t)
		f.pressure(t, true)
		f.sequence(t)
		call, ok := f.client.nextCall(testutil.ChannelTimeout)
		if !ok {
			t.Fatal("no recall")
		}
		f.client.answerSequence(t, call, nfs4.NFS4ERR_SEQ_MISORDERED)
		waitFor(t, "BACKCHANNEL_FAULT", func() bool {
			flags, _ := f.sequence(t)
			return flags&nfs4.SEQ4_STATUS_BACKCHANNEL_FAULT != 0
		})
		f.pressure(t, false)
		f.pressure(t, true)
		f.sequence(t)
		if _, more := f.client.nextCall(100 * time.Millisecond); more {
			t.Fatal("sent after a fault")
		}
		f.pressure(t, false)
	})
}

// DESTROY_SESSION cancels an outstanding callback; the session's record
// credit returns only once the sender has exited and removed its entry, and
// a late reply is then a stray.
func TestCBWorkerHoldsSessionUntilExit(t *testing.T) {
	f := newCBSession(t)
	f.client.answerNull(t)
	f.pressure(t, true)
	f.sequence(t)
	call, ok := f.client.nextCall(testutil.ChannelTimeout)
	if !ok {
		t.Fatal("no recall")
	}
	f.pressure(t, false)
	sess := f.session(t)
	records := f.srv.sessions.accounting().Records
	res := f.client.compound(f.next(), withSequence(admissionDestroy(f.ts.id), f.ts, 0, f.slotSeq+1))
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("DESTROY_SESSION %s", nfs4.Nfsstat4Name(res.Status()))
	}
	waitFor(t, "sender exit", func() bool {
		sess.cb.mu.Lock()
		defer sess.cb.mu.Unlock()
		return !sess.cb.running
	})
	waitFor(t, "record credit", func() bool { return f.srv.sessions.accounting().Records < records })
	strays := f.srv.backchannel.strayReplies.Load()
	f.client.answerSequence(t, call, nfs4.NFS4_OK)
	waitFor(t, "late reply dropped", func() bool { return f.srv.backchannel.strayReplies.Load() == strays+1 })
}

// Callback renewal under the gate: live and deciding renew (deciding also
// makes expiry give way); submitted and expired do not.
func TestCallbackRenewalUnderGate(t *testing.T) {
	ss := newSessionStore(1)
	now := time.Now()
	ss.now = func() time.Time { return now }
	for _, c := range []struct {
		name    string
		lease   leaseGateState
		phase   markerPhase
		renews  bool
		arbiter bool
	}{
		{"live", gateLive, markerNone, true, false},
		{"deciding", gateChecking, markerDeciding, true, true},
		{"submitted", gateChecking, markerSubmitted, false, false},
		{"expired", gateExpired, markerNone, false, false},
	} {
		ss.mu.Lock()
		g, _ := ss.groupLocked(7)
		g.lease, g.markerPhase, g.renewedDuringCheck, g.quietUntil = c.lease, c.phase, false, time.Time{}
		ss.mu.Unlock()
		ss.callbackRenew(7, now)
		ss.mu.Lock()
		renewed := !g.quietUntil.IsZero()
		if renewed != c.renews || g.renewedDuringCheck != c.arbiter {
			t.Errorf("%s: renewed %t, arbitration %t", c.name, renewed, g.renewedDuringCheck)
		}
		if renewed && !g.quietUntil.Equal(now.Add(nfsLeaseTime)) {
			t.Errorf("%s: quiet until %s", c.name, g.quietUntil)
		}
		ss.mu.Unlock()
	}
}

// expiryGateVFS blocks the lease observation's directory read, and can
// fail the expired marker's creation.
type expiryGateVFS struct {
	vfs.TernVFS
	mu         sync.Mutex
	dir        vfs.InodeID
	entered    chan struct{}
	release    chan struct{}
	failMarker bool
}

func (fs *expiryGateVFS) Readdir(dir vfs.InodeID, start uint64) ([]vfs.DirEntry, uint64, error) {
	fs.mu.Lock()
	block := fs.dir != 0 && dir == fs.dir
	if block {
		fs.dir = 0
	}
	fs.mu.Unlock()
	if block {
		close(fs.entered)
		<-fs.release
	}
	return fs.TernVFS.Readdir(dir, start)
}

func (fs *expiryGateVFS) CreateFile(dir vfs.InodeID, name string, data io.Reader) (vfs.InodeID, error) {
	fs.mu.Lock()
	fail := fs.failMarker && name == expiredName
	fs.mu.Unlock()
	if fail {
		return 0, errInjectedExpiry
	}
	return fs.TernVFS.CreateFile(dir, name, data)
}

var errInjectedExpiry = errors.New("injected marker failure")

// A callback reply while expiry observes the lease makes the decision give
// way and renews; callbacks keep the client live here although its durable
// lease is dead. Once the marker is submitted the client expires, whether
// the marker write succeeds or not.
func TestCallbackArbitratesExpiry(t *testing.T) {
	for _, c := range []string{"reply-during-check", "no-reply", "marker-error"} {
		t.Run(c, func(t *testing.T) {
			fs := &expiryGateVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
			srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(srv.clients.stopLeaseWorkers)
			clientID, confirm, err := srv.clients.SetClientID([8]byte{1}, []byte("cb-expiry"), clientOwner{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := srv.clients.ConfirmClientID(clientID, confirm, rpcPrincipal{}); err != nil {
				t.Fatal(err)
			}
			publishLeaseForTest(t, srv, clientID)
			srv.clients.stopLeaseWorkers()
			// The durable lease is long dead.
			later := time.Now().Add(10 * leaseWriteTerm)
			srv.clients.setClock(func() time.Time { return later })
			fs.mu.Lock()
			fs.dir, fs.entered, fs.release = vfs.InodeID(clientID), make(chan struct{}), make(chan struct{})
			fs.failMarker = c == "marker-error"
			fs.mu.Unlock()
			srv.sessions.mu.Lock()
			srv.sessions.groupLocked(clientID)
			srv.sessions.mu.Unlock()
			decided := make(chan bool, 1)
			go func() {
				expired, _ := srv.decideExpired(clientID, false)
				decided <- expired
			}()
			testutil.AwaitSignal(t, fs.entered, "lease observation")
			if c == "reply-during-check" {
				srv.sessions.callbackRenew(clientID, later)
			}
			close(fs.release)
			expired := testutil.AwaitValue(t, decided, "expiry decision")
			if want := c != "reply-during-check"; expired != want {
				t.Fatalf("expired %t, want %t", expired, want)
			}
			_, markerErr := fs.TernVFS.Lookup(vfs.InodeID(clientID), expiredName)
			if marked := markerErr == nil; marked != (c == "no-reply") {
				t.Fatalf("marker present %t", marked)
			}
			if c != "reply-during-check" {
				return
			}
			// Renewed: the client stays live locally while the durable
			// lease is dead.
			if expired, err := srv.decideExpired(clientID, false); expired || err != nil {
				t.Fatalf("renewed client decided %t %v", expired, err)
			}
		})
	}
}
