// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

func admissionSequence(body []byte, ts *testSession, slot, seq uint32, cache bool) []byte {
	wire := withSequence(body, ts, slot, seq)
	countOff, _, _ := compoundArgLayout(wire)
	if !cache {
		binary.BigEndian.PutUint32(wire[countOff+36:], nfs4.FALSE)
	}
	return wire
}

func admissionRequest(t *testing.T, srv *Server, wire []byte, xid uint32, conn *rpcConnection) ([]byte, nfs4.COMPOUND4res) {
	t.Helper()
	req, err := parseRPCCall(rpcCallForTest(xid, procCompound, wire))
	if err != nil {
		t.Fatal(err)
	}
	req.connection = conn
	reply := srv.safeHandleCompound(req, "admission-test")
	// The test consumes/copies the transport output, then ends server ownership.
	copyReply := bytes.Clone(reply)
	reply = nil
	req.releaseReply()
	reply = copyReply
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatalf("reply %x: %v", reply, err)
	}
	return reply, res
}

func admissionStatus(t *testing.T, res nfs4.COMPOUND4res, want uint32, ops ...uint32) {
	t.Helper()
	if res.Status() != want {
		t.Fatalf("status %s, want %s", nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(want))
	}
	var got []uint32
	iter := res.Resarray()
	for iter.Next() {
		got = append(got, iter.Resarray().Disc())
	}
	if !reflect.DeepEqual(got, ops) {
		t.Fatalf("operations %v, want %v", got, ops)
	}
}

type admissionSlotImage struct {
	seq                     uint32
	occupied, inUse, cached bool
	reply                   []byte
	digest                  [32]byte
	user                    effectiveUser
	reserved                int
}

func admissionImage(slot *sessionSlot) admissionSlotImage {
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return admissionSlotImage{slot.seq, slot.occupied, slot.inUse, slot.cached, bytes.Clone(slot.reply), slot.digest, slot.user, slot.reserved}
}

type admissionLeaseVFS struct {
	TernVFS
	fail bool
}

func (fs *admissionLeaseVFS) Lookup(id InodeID, name string) (InodeID, error) {
	if fs.fail {
		return 0, syscall.EIO
	}
	return fs.TernVFS.Lookup(id, name)
}

func TestAdmissionFailedSequencePreservesCache(t *testing.T) {
	cases := []string{"bad-session", "bad-slot", "bad-highest", "highest-below-slot", "misordered", "too-many-ops", "full-request", "response", "cache", "quota", "association", "renewal", "missing-client"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			fs := &admissionLeaseVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
			srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			ts := connectionSession(t, srv)
			sess := srv.sessions.lookup(ts.id)
			body := buildCompoundBody([]byte("prior"), func(w *nfs4.COMPOUND4argsWriter) {
				if name != "quota" {
					w.AppendArgarray_Putrootfh()
				}
			})
			original := admissionSequence(body, ts, 0, 1, name != "quota")
			reply, res := admissionRequest(t, srv, original, 1, nil)
			priorOps := []uint32{nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH}
			if name == "quota" {
				priorOps = priorOps[:1]
			}
			admissionStatus(t, res, nfs4.NFS4_OK, priorOps...)
			before := admissionImage(&sess.slots[0])
			reserved := srv.sessions.cachedBytes
			next := admissionSequence(body, ts, 0, 2, true)
			off, _, _ := compoundArgLayout(next)
			want := uint32(nfs4.NFS4ERR_BADSESSION)
			conn := (*rpcConnection)(nil)
			restore := func() {}
			switch name {
			case "bad-session":
				next[off+4+4] ^= 1
			case "bad-slot":
				binary.BigEndian.PutUint32(next[off+28:], maxSessionSlots)
				want = nfs4.NFS4ERR_BADSLOT
			case "bad-highest":
				binary.BigEndian.PutUint32(next[off+32:], maxSessionSlots)
				want = nfs4.NFS4ERR_BAD_HIGH_SLOT
			case "highest-below-slot":
				binary.BigEndian.PutUint32(next[off+28:], 1)
				binary.BigEndian.PutUint32(next[off+32:], 0)
				want = nfs4.NFS4ERR_BAD_HIGH_SLOT
			case "misordered":
				binary.BigEndian.PutUint32(next[off+24:], 4)
				want = nfs4.NFS4ERR_SEQ_MISORDERED
			case "too-many-ops":
				sess.maxOps = 1
				want = nfs4.NFS4ERR_TOO_MANY_OPS
				restore = func() { sess.maxOps = maxCompoundOperations }
			case "full-request":
				sess.maxReq = uint32(len(next))
				want = nfs4.NFS4ERR_REQ_TOO_BIG
				restore = func() { sess.maxReq = maxRPCMessage }
			case "response":
				sess.maxResp = 87
				want = nfs4.NFS4ERR_REP_TOO_BIG
				restore = func() { sess.maxResp = maxRPCMessage }
			case "cache":
				sess.maxCached = 87
				want = nfs4.NFS4ERR_REP_TOO_BIG_TO_CACHE
				restore = func() { sess.maxCached = maxSessionReplyCached }
			case "quota":
				// Model other sessions having consumed the remaining capacity.
				// Replacing this uncached prefix requires a larger reservation.
				srv.sessions.cachedBytes = maxSessionCacheBytes
				want = nfs4.NFS4ERR_RESOURCE
				restore = func() { srv.sessions.cachedBytes = reserved }
			case "association":
				conn = &rpcConnection{sessions: make(map[SessionID]struct{})}
				for i := 0; i < maxConnectionSessions; i++ {
					other := connectionSession(t, srv)
					conn.sessions[other.id] = struct{}{}
				}
				want = nfs4.NFS4ERR_RESOURCE
			case "renewal":
				// The lease is not proven: DELAY from memory, with the
				// store failing too.
				setLeaseForTest(srv, sess.clientID, func(r *leaseRefresher) { r.proven = 0 })
				fs.fail = true
				want = nfs4.NFS4ERR_DELAY
				restore = func() { fs.fail = false; srv.clients.pinLeaseForTest(sess.clientID) }
			case "missing-client":
				setLeaseForTest(srv, sess.clientID, func(r *leaseRefresher) { r.state = leaseRecordStale })
				want = nfs4.NFS4ERR_BADSESSION
				restore = func() {
					setLeaseForTest(srv, sess.clientID, func(r *leaseRefresher) { r.state = leaseRecordLive })
				}
			}
			_, res = admissionRequest(t, srv, next, 2, conn)
			admissionStatus(t, res, want, nfs4.OP_SEQUENCE)
			if got := admissionImage(&sess.slots[0]); !reflect.DeepEqual(got, before) {
				t.Fatalf("failed admission changed cache: before %+v after %+v", before, got)
			}
			restore()
			if srv.sessions.cachedBytes != reserved {
				t.Fatalf("reply reservation changed %d to %d", reserved, srv.sessions.cachedBytes)
			}
			replay, res := admissionRequest(t, srv, original, 3, nil)
			admissionStatus(t, res, nfs4.NFS4_OK, priorOps...)
			if !bytes.Equal(reply[24:], replay[24:]) {
				t.Fatal("failed admission changed subsequent replay")
			}
		})
	}
}

func admissionAuthCall(xid uint32, body []byte, stamp uint32, host string, uid, gid uint32, groups []uint32) []byte {
	cred := binary.BigEndian.AppendUint32(nil, stamp)
	cred = binary.BigEndian.AppendUint32(cred, uint32(len(host)))
	cred = append(cred, host...)
	for len(cred)%4 != 0 {
		cred = append(cred, 0)
	}
	for _, n := range []uint32{uid, gid, uint32(len(groups))} {
		cred = binary.BigEndian.AppendUint32(cred, n)
	}
	for _, n := range groups {
		cred = binary.BigEndian.AppendUint32(cred, n)
	}
	call := rpcCallForTest(xid, procCompound, nil)[:24]
	call = binary.BigEndian.AppendUint32(call, authSys)
	call = binary.BigEndian.AppendUint32(call, uint32(len(cred)))
	call = append(call, cred...)
	call = binary.BigEndian.AppendUint32(call, authNone)
	call = binary.BigEndian.AppendUint32(call, 0)
	return append(call, body...)
}

func TestAdmissionReplayEffectiveUserAndDigest(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 4)
	ts := connectionSession(t, srv)
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, 0, 1, true)
	send := func(xid, stamp uint32, host string, uid, gid uint32, groups []uint32, body []byte) ([]byte, nfs4.COMPOUND4res) {
		t.Helper()
		req, err := parseRPCCall(admissionAuthCall(xid, body, stamp, host, uid, gid, groups))
		if err != nil {
			t.Fatal(err)
		}
		reply := srv.safeHandleCompound(req, "test")
		owned := bytes.Clone(reply)
		reply = nil
		req.releaseReply()
		reply = owned
		res, err := parseCompoundReply(reply)
		if err != nil {
			t.Fatal(err)
		}
		return reply, res
	}
	original, res := send(1, 10, "host-one", 100, 200, []uint32{4, 3, 4, 200}, wire)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	replay, res := send(1001, 20, "another-host", 100, 200, []uint32{3, 4}, wire)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	if !bytes.Equal(original[24:], replay[24:]) || binary.BigEndian.Uint32(replay) != 1001 {
		t.Fatal("normalized user/new-XID replay changed")
	}
	for _, test := range []struct {
		name     string
		uid, gid uint32
		groups   []uint32
		body     []byte
	}{
		{"uid", 101, 200, []uint32{3, 4}, wire}, {"gid", 100, 201, []uint32{3, 4}, wire}, {"groups", 100, 200, []uint32{3, 5}, wire},
		{"body", 100, 200, []uint32{3, 4}, admissionSequence(buildCompoundBody([]byte("different"), func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, 0, 1, true)},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := admissionImage(&srv.sessions.lookup(ts.id).slots[0])
			_, res := send(2, 1, "host", test.uid, test.gid, test.groups, test.body)
			admissionStatus(t, res, nfs4.NFS4ERR_SEQ_FALSE_RETRY, nfs4.OP_SEQUENCE)
			if !reflect.DeepEqual(before, admissionImage(&srv.sessions.lookup(ts.id).slots[0])) {
				t.Fatal("false retry changed cache")
			}
		})
	}
	_, res = admissionRequest(t, srv, wire, 3, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_SEQ_FALSE_RETRY, nfs4.OP_SEQUENCE)
}

func TestAdmissionUncachedReplayPrefix(t *testing.T) {
	for _, following := range []bool{false, true} {
		t.Run(map[bool]string{false: "sequence-only", true: "following-operation"}[following], func(t *testing.T) {
			srv, _ := newConnectionTestServer(t, 2)
			ts := connectionSession(t, srv)
			wire := admissionSequence(buildCompoundBody([]byte("tag"), func(w *nfs4.COMPOUND4argsWriter) {
				if following {
					w.AppendArgarray_Putrootfh()
				}
			}), ts, 0, 1, false)
			_, first := admissionRequest(t, srv, wire, 1, nil)
			_, replay := admissionRequest(t, srv, wire, 2, nil)
			if following {
				admissionStatus(t, first, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
				admissionStatus(t, replay, nfs4.NFS4ERR_RETRY_UNCACHED_REP, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
			} else {
				admissionStatus(t, first, nfs4.NFS4_OK, nfs4.OP_SEQUENCE)
				admissionStatus(t, replay, nfs4.NFS4_OK, nfs4.OP_SEQUENCE)
			}
			if string(replay.Tag().Data()) != "tag" {
				t.Fatal("uncached replay lost tag")
			}
			it := replay.Resarray()
			it.Next()
			if it.Resarray().Value().AsSEQUENCE4resEntry().Disc() != nfs4.NFS4_OK {
				t.Fatal("SEQUENCE not retained")
			}
			if following {
				it.Next()
				if binary.BigEndian.Uint32(it.Resarray().Value().Bytes()) != nfs4.NFS4ERR_RETRY_UNCACHED_REP {
					t.Fatal("wrong uncached result")
				}
			}
		})
	}
}

func TestAdmissionSequenceWrap(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	slot := &sess.slots[0]
	slot.seq = math.MaxUint32
	slot.occupied = true
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, 0, 0, true)
	first, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	replay, res := admissionRequest(t, srv, wire, 2, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	if slot.seq != 0 || !bytes.Equal(first[24:], replay[24:]) {
		t.Fatal("wrap was not admitted once")
	}
}

func TestAdmissionExecutingDuplicateAcrossConnections(t *testing.T) {
	srv, fs := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	defer closeSignal(fs.release)
	a, b := dial(t, addr), dial(t, addr)
	defer a.Close()
	defer b.Close()
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		attr := w.AppendArgarray_Getattr()
		bits := attr.StartAttrRequest()
		bits.AppendData(1 << nfs4.FATTR4_TYPE)
		attr.Resume(bits.Finish())
		w.Resume(attr.Finish())
	})
	wire := admissionSequence(body, ts, 0, 1, true)
	writeCallForTest(t, a, rpcCallForTest(1, procCompound, wire))
	awaitSignal(t, fs.entered, "original stat")
	before := admissionImage(&srv.sessions.lookup(ts.id).slots[0])
	b.SetDeadline(time.Now().Add(testChannelTimeout))
	reply, err := sendRawRPC(b, 2, procCompound, wire)
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	admissionStatus(t, res, nfs4.NFS4ERR_DELAY, nfs4.OP_SEQUENCE)
	if !reflect.DeepEqual(before, admissionImage(&srv.sessions.lookup(ts.id).slots[0])) {
		t.Fatal("executing duplicate changed slot")
	}
	if fs.calls.Load() != 1 {
		t.Fatal("executing duplicate reached backend")
	}
	closeSignal(fs.release)
	a.SetDeadline(time.Now().Add(testChannelTimeout))
	first, err := readFrame(a)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := sendRawRPC(b, 3, procCompound, wire)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first[24:], replay[24:]) || fs.calls.Load() != 1 {
		t.Fatal("completed cross-connection replay reexecuted")
	}
}

func TestAdmissionRPCPaddingAndCompleteSize(t *testing.T) {
	body := buildCompoundBody([]byte("tag"), func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	call := rpcCallForTest(1, procCompound, nil)[:24]
	for _, opaque := range [][]byte{{1, 2, 3}, {4}} {
		call = binary.BigEndian.AppendUint32(call, 99)
		call = binary.BigEndian.AppendUint32(call, uint32(len(opaque)))
		call = append(call, opaque...)
		for len(call)%4 != 0 {
			call = append(call, 0)
		}
	}
	call = append(call, body...)
	req, err := parseRPCCall(call)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(req.body, body) || req.fullSize() != len(call) || len(req.credBody) != 3 || len(req.verfBody) != 1 {
		t.Fatal("RPC opaque padding/size wrong")
	}
	if _, err := parseRPCCall(call[:45]); err == nil {
		t.Fatal("missing verifier padding accepted")
	}
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	wire := admissionSequence(body, ts, 0, 1, true)
	call = admissionAuthCall(1, wire, 10, "abc", 1, 2, []uint32{3})
	req, err = parseRPCCall(call)
	if err != nil {
		t.Fatal(err)
	}
	sess := srv.sessions.lookup(ts.id)
	sess.maxReq = uint32(len(call) - 1)
	reply := srv.safeHandleCompound(req, "test")
	owned := bytes.Clone(reply)
	reply = nil
	req.releaseReply()
	reply = owned
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	admissionStatus(t, res, nfs4.NFS4ERR_REQ_TOO_BIG, nfs4.OP_SEQUENCE)
	if sess.slots[0].occupied {
		t.Fatal("full RPC rejection consumed slot")
	}
	sess.maxReq = uint32(len(call))
	reply = srv.safeHandleCompound(req, "test")
	owned = bytes.Clone(reply)
	reply = nil
	req.releaseReply()
	reply = owned
	res, err = parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
}

func admissionBind(id SessionID, dir uint32, rdma bool) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		op := w.AppendArgarray_BindConnToSession()
		writeSessionID(op.BctsaSessid(), id)
		op.SetBctsaDir(dir)
		if rdma {
			op.SetBctsaUseConnInRdmaMode(nfs4.TRUE)
		}
	})
}
func admissionDestroy(id SessionID) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		op := w.AppendArgarray_DestroySession()
		writeSessionID(op.DsaSessionid(), id)
	})
}
func admissionJoin(bodies ...[]byte) []byte {
	out := bytes.Clone(bodies[0])
	off, _, _ := compoundArgLayout(out)
	count := binary.BigEndian.Uint32(out[off:])
	for _, body := range bodies[1:] {
		at, _, _ := compoundArgLayout(body)
		count += binary.BigEndian.Uint32(body[at:])
		out = append(out, body[at+4:]...)
	}
	binary.BigEndian.PutUint32(out[off:], count)
	return out
}

func TestAdmissionSP4NoneAndMaintenancePositions(t *testing.T) {
	srv := admissionWritableServer(t, NewLocalTernVFS(t.TempDir()))
	ts := connectionSession(t, srv)
	conn := &rpcConnection{}
	_, res := admissionRequest(t, srv, admissionDestroy(ts.id), 1, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_CONN_NOT_BOUND_TO_SESSION, nfs4.OP_DESTROY_SESSION)
	for _, dir := range []uint32{nfs4.CDFC4_BACK, nfs4.CDFC4_BACK_OR_BOTH, 99} {
		_, res = admissionRequest(t, srv, admissionBind(ts.id, dir, false), 2, conn)
		admissionStatus(t, res, nfs4.NFS4ERR_INVAL, nfs4.OP_BIND_CONN_TO_SESSION)
	}
	_, res = admissionRequest(t, srv, admissionBind(ts.id, nfs4.CDFC4_FORE, true), 2, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_INVAL, nfs4.OP_BIND_CONN_TO_SESSION)
	if conn.associated(ts.id) {
		t.Fatal("failed BIND associated transport")
	}
	// A different supported principal can bind this SP4_NONE session.
	req, err := parseRPCCall(admissionAuthCall(3, admissionBind(ts.id, nfs4.CDFC4_FORE_OR_BOTH, false), 1, "other", 123, 456, []uint32{789}))
	if err != nil {
		t.Fatal(err)
	}
	req.connection = conn
	reply := srv.safeHandleCompound(req, "test")
	owned := bytes.Clone(reply)
	reply = nil
	req.releaseReply()
	reply = owned
	res, err = parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_BIND_CONN_TO_SESSION)
	it := res.Resarray()
	it.Next()
	if it.Resarray().Value().AsBINDCONNTOSESSION4resEntry().Value().AsBINDCONNTOSESSION4resok().BctsrDir() != nfs4.CDFS4_FORE || !conn.associated(ts.id) {
		t.Fatal("SP4_NONE binding")
	}
	ordinary := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	_, res = admissionRequest(t, srv, admissionJoin(admissionBind(ts.id, nfs4.CDFC4_FORE, false), ordinary), 4, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_NOT_ONLY_OP, nfs4.OP_BIND_CONN_TO_SESSION)
	_, res = admissionRequest(t, srv, admissionJoin(exchangeIDBody("bad-position", [8]byte{}), ordinary), 5, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_NOT_ONLY_OP, nfs4.OP_EXCHANGE_ID)
	_, res = admissionRequest(t, srv, admissionJoin(createSessionBody(999, 1), ordinary), 6, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_NOT_ONLY_OP, nfs4.OP_CREATE_SESSION)
	_, res = admissionRequest(t, srv, admissionJoin(admissionDestroy(ts.id), ordinary), 7, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_NOT_ONLY_OP, nfs4.OP_DESTROY_SESSION)
	// Self-destruction must be final; the preceding SEQUENCE remains successful.
	wire := admissionSequence(admissionJoin(admissionDestroy(ts.id), ordinary), ts, 0, 1, true)
	_, res = admissionRequest(t, srv, wire, 8, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_NOT_ONLY_OP, nfs4.OP_SEQUENCE, nfs4.OP_DESTROY_SESSION)
	if srv.sessions.lookup(ts.id) == nil {
		t.Fatal("position rejection destroyed session")
	}
	// Automatic association on SEQUENCE allows self-destruction on a new connection.
	automatic := &rpcConnection{}
	wire = admissionSequence(admissionDestroy(ts.id), ts, 0, 2, true)
	_, res = admissionRequest(t, srv, wire, 9, automatic)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_DESTROY_SESSION)
	if srv.sessions.lookup(ts.id) != nil || srv.sessions.cachedBytes != 0 {
		t.Fatal("self-destruction/cache release")
	}
}

func TestAdmissionMaintenanceExplicitOtherClient(t *testing.T) {
	srv := admissionWritableServer(t, NewLocalTernVFS(t.TempDir()))
	ts := connectionSession(t, srv)
	conn := &rpcConnection{}
	wire := admissionSequence(exchangeIDBody("other-client", [8]byte{}), ts, 0, 1, true)
	_, res := admissionRequest(t, srv, wire, 1, conn)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_EXCHANGE_ID)
	it := res.Resarray()
	it.Next()
	it.Next()
	exchange := it.Resarray().Value().AsEXCHANGEID4resEntry().Value().AsEXCHANGEID4resok()
	if exchange.EirClientid() == ts.clientID {
		t.Fatal("test requires another explicit client")
	}
	wire = admissionSequence(createSessionBody(exchange.EirClientid(), exchange.EirSequenceid()), ts, 0, 2, true)
	_, res = admissionRequest(t, srv, wire, 2, conn)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_CREATE_SESSION)
	it = res.Resarray()
	it.Next()
	it.Next()
	other := readSessionID(it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok().CsrSessionid())
	if !conn.associated(other) {
		t.Fatal("CREATE_SESSION did not associate its transport")
	}
	// Destruction of another associated session need not be the last operation.
	wire = admissionSequence(admissionJoin(admissionDestroy(other), buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })), ts, 0, 3, true)
	_, res = admissionRequest(t, srv, wire, 3, conn)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_DESTROY_SESSION, nfs4.OP_PUTROOTFH)
	// DESTROY_CLIENTID uses the live session set even for an explicit other CID.
	client := exchange.EirClientid()
	otherTS := connectionSession(t, srv)
	moveSessionForTest(srv.sessions, srv.sessions.lookup(otherTS.id), incarnationForTest(t, srv, client), otherTS.id)
	destroyClient := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_DestroyClientid().SetDcaClientid(client) })
	wire = admissionSequence(destroyClient, ts, 0, 4, true)
	_, res = admissionRequest(t, srv, wire, 4, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_CLIENTID_BUSY, nfs4.OP_SEQUENCE, nfs4.OP_DESTROY_CLIENTID)
	_, res = admissionRequest(t, srv, admissionJoin(destroyClient, exchangeIDBody("illegal", [8]byte{})), 5, conn)
	admissionStatus(t, res, nfs4.NFS4ERR_NOT_ONLY_OP, nfs4.OP_DESTROY_CLIENTID)
}

func TestAdmissionSequencePosition(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	empty := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {})
	wire := admissionSequence(admissionSequence(empty, ts, 0, 1, true), ts, 0, 1, true)
	_, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_SEQUENCE_POS, nfs4.OP_SEQUENCE, nfs4.OP_SEQUENCE)
}

func TestAdmissionUncachedFalseRetryAndMalformedUser(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	body := buildCompoundBody([]byte("uncached"), func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	wire := admissionSequence(body, ts, 0, 1, false)
	_, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	before := admissionImage(&srv.sessions.lookup(ts.id).slots[0])
	changed := admissionSequence(buildCompoundBody([]byte("another-tag"), func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, 0, 1, false)
	_, res = admissionRequest(t, srv, changed, 2, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_SEQ_FALSE_RETRY, nfs4.OP_SEQUENCE)
	for _, test := range []struct {
		name   string
		flavor uint32
		cred   []byte
	}{
		{"nonempty-auth-none", authNone, []byte{1}}, {"truncated-auth-sys", authSys, make([]byte, 8)}, {"unsupported-flavor", 99, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			req, err := parseRPCCall(rpcCallForTest(3, procCompound, admissionSequence(body, ts, 0, 2, false)))
			if err != nil {
				t.Fatal(err)
			}
			req.credFlavor, req.credBody = test.flavor, test.cred
			reply := srv.safeHandleCompound(req, "test")
			owned := bytes.Clone(reply)
			reply = nil
			req.releaseReply()
			reply = owned
			res, err := parseCompoundReply(reply)
			if err != nil {
				t.Fatal(err)
			}
			admissionStatus(t, res, nfs4.NFS4ERR_WRONGSEC, nfs4.OP_SEQUENCE)
			if !reflect.DeepEqual(before, admissionImage(&srv.sessions.lookup(ts.id).slots[0])) {
				t.Fatal("malformed user changed admitted slot")
			}
		})
	}
	_, res = admissionRequest(t, srv, wire, 4, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_RETRY_UNCACHED_REP, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH)
	if !reflect.DeepEqual(before, admissionImage(&srv.sessions.lookup(ts.id).slots[0])) {
		t.Fatal("uncached retry changed cache")
	}
}

func TestAdmissionDestroyClientWithOpenAndNoSession(t *testing.T) {
	srv, addr, stop := startTestServerWithServer(t, t.TempDir())
	defer stop()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	client := setupClient(t, conn, &xid)
	state, fh := openCreateFile(t, conn, &xid, client, "busy")
	ts := testSessionForClient(conn, client)
	send := func(body []byte) nfs4.COMPOUND4res {
		t.Helper()
		reply, err := sendRawRPC(conn, xid, procCompound, body)
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
	admissionStatus(t, send(admissionDestroy(ts.id)), nfs4.NFS4_OK, nfs4.OP_DESTROY_SESSION)
	destroy := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_DestroyClientid().SetDcaClientid(client) })
	admissionStatus(t, send(destroy), nfs4.NFS4ERR_CLIENTID_BUSY, nfs4.OP_DESTROY_CLIENTID)
	if len(srv.opens.clientStates(incarnationForTest(t, srv, client))) != 1 {
		t.Fatal("busy rejection discarded live open")
	}
	replacement, status, err := createSession(conn, xid, client, ts.nextCSSeq)
	xid++
	if err != nil || status != nfs4.NFS4_OK {
		t.Fatalf("new session %s %v", nfs4.Nfsstat4Name(status), err)
	}
	closeFile(t, conn, &xid, fh, state)
	admissionStatus(t, send(admissionDestroy(replacement.id)), nfs4.NFS4_OK, nfs4.OP_DESTROY_SESSION)
	admissionStatus(t, send(destroy), nfs4.NFS4_OK, nfs4.OP_DESTROY_CLIENTID)
}

func TestAdmissionUncachedSetattrErrorShape(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		op := w.AppendArgarray_Setattr()
		attrs := op.StartObjAttributes()
		mask := attrs.StartAttrmask()
		attrs.Resume(mask.Finish())
		attrs.Resume(attrs.StartAttrVals().SetData(nil).Finish())
		op.Resume(attrs.Finish())
		w.Resume(op.Finish())
	})
	wire := admissionSequence(body, ts, 0, 1, false)
	_, res := admissionRequest(t, srv, wire, 1, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_NOFILEHANDLE, nfs4.OP_SEQUENCE, nfs4.OP_SETATTR)
	reply, res := admissionRequest(t, srv, wire, 2, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_RETRY_UNCACHED_REP, nfs4.OP_SEQUENCE, nfs4.OP_SETATTR)
	it := res.Resarray()
	it.Next()
	it.Next()
	attrs := it.Resarray().Value().AsSETATTR4res()
	if attrs.Status() != nfs4.NFS4ERR_RETRY_UNCACHED_REP || attrs.Attrsset().Count() != 0 || len(reply) != 92 {
		t.Fatal("SETATTR error missing empty attrsset/prefix")
	}
}
