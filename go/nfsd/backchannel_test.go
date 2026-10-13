// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

type cbSecOffer uint8

const (
	offerNone cbSecOffer = iota
	offerSys
	offerGSS
)

// backCreateSessionBody is a CREATE_SESSION with flags and the given
// callback credentials, in order.
func backCreateSessionBody(clientID uint64, seq, flags uint32, offers ...cbSecOffer) []byte {
	return backCreateSessionBodyLimits(clientID, seq, flags, maxRPCMessage, 16, offers...)
}

// backCreateSessionBodyLimits is backCreateSessionBody with the back
// channel's request size and slot count given.
func backCreateSessionBodyLimits(clientID uint64, seq, flags, backReq, backReqs uint32, offers ...cbSecOffer) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		cw := w.AppendArgarray_CreateSession()
		cw.SetCsaClientid(clientID)
		cw.SetCsaSequence(seq)
		cw.SetCsaFlags(flags)
		attrs := func(a nfs4.ChannelAttrs4Writer, req, reqs uint32) []byte {
			a.SetCaMaxrequestsize(req)
			a.SetCaMaxresponsesize(maxRPCMessage)
			a.SetCaMaxresponsesizeCached(maxSessionReplyCached)
			a.SetCaMaxoperations(maxCompoundOperations)
			a.SetCaMaxrequests(reqs)
			return a.Finish()
		}
		cw.Resume(attrs(cw.StartCsaForeChanAttrs(), maxRPCMessage, 16))
		cw.Resume(attrs(cw.StartCsaBackChanAttrs(), backReq, backReqs))
		cw.SetCsaCbProgram(0x40000001)
		for _, offer := range offers {
			switch offer {
			case offerNone:
				cw.AppendCsaSecParms_None()
			case offerSys:
				aw := cw.AppendCsaSecParms_Sys()
				aw.SetStamp(7)
				aw.SetMachinename([]byte("cb-host"))
				aw.SetUid(1000)
				aw.SetGid(100)
				gw := aw.StartGids()
				aw.Resume(gw.Finish())
				cw.Resume(aw.Finish())
			case offerGSS:
				gw := cw.AppendCsaSecParms_RpcsecGss()
				gw.SetGcbpService(1)
				hw := gw.StartGcbpHandleFromServer()
				gw.Resume(hw.SetData([]byte("s")).Finish())
				hw = gw.StartGcbpHandleFromClient()
				gw.Resume(hw.SetData([]byte("c")).Finish())
				cw.Resume(gw.Finish())
			}
		}
		w.Resume(cw.Finish())
	})
}

func createWith(t *testing.T, conn net.Conn, xid *uint32, body []byte) (uint32, nfs4.CREATESESSION4resok) {
	t.Helper()
	reply, err := sendRawRPC(conn, *xid, procCompound, body)
	*xid++
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status() != nfs4.NFS4_OK {
		return res.Status(), nfs4.CREATESESSION4resok{}
	}
	iter := res.Resarray()
	iter.Next()
	return nfs4.NFS4_OK, iter.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok()
}

// The callback credential is checked before the creation slot, and only
// when a backchannel is requested.
func TestCreateSessionCallbackSecurity(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID, seq, status, err := exchangeID(conn, xid, "cb-client", [8]byte{3})
	xid++
	if err != nil || status != nfs4.NFS4_OK {
		t.Fatalf("EXCHANGE_ID %s %v", nfs4.Nfsstat4Name(status), err)
	}
	back := uint32(nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN)
	for _, c := range []struct {
		name   string
		offers []cbSecOffer
		want   uint32
	}{
		{"only RPCSEC_GSS", []cbSecOffer{offerGSS}, nfs4.NFS4ERR_NOENT},
		{"none offered", nil, nfs4.NFS4ERR_INVAL},
	} {
		if got, _ := createWith(t, conn, &xid, backCreateSessionBody(clientID, seq, back, c.offers...)); got != c.want {
			t.Fatalf("%s: %s, want %s", c.name, nfs4.Nfsstat4Name(got), nfs4.Nfsstat4Name(c.want))
		}
	}
	// The slot was not consumed: the same sequence now succeeds, choosing
	// the first usable entry in the client's order.
	got, ok := createWith(t, conn, &xid, backCreateSessionBody(clientID, seq, back, offerGSS, offerSys, offerNone))
	if got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(got))
	}
	if ok.CsrFlags()&nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN == 0 {
		t.Fatal("requested backchannel not granted")
	}
	sess := srv.sessions.lookup(readSessionID(ok.CsrSessionid()))
	if sess == nil || !sess.backRequested || sess.cbProgram != 0x40000001 || sess.cbSec.flavor != authSys {
		t.Fatalf("session callback state %+v", sess)
	}
	// stamp, machine name with its padding, uid, gid and no groups.
	want := binary.BigEndian.AppendUint32(nil, 7)
	want = binary.BigEndian.AppendUint32(want, 7)
	want = append(want, "cb-host\x00"...)
	want = binary.BigEndian.AppendUint32(want, 1000)
	want = binary.BigEndian.AppendUint32(want, 100)
	want = binary.BigEndian.AppendUint32(want, 0)
	if !bytes.Equal(sess.cbSec.body, want) {
		t.Fatalf("AUTH_SYS callback credential %x, want %x verbatim", sess.cbSec.body, want)
	}

	// Without CONN_BACK_CHAN no credential is needed. (The granted
	// backchannel's CB_NULL now arrives on conn, so use another.)
	conn = dial(t, addr)
	defer conn.Close()
	clientID, seq, _, _ = exchangeID(conn, xid, "cb-less-client", [8]byte{4})
	xid++
	if got, _ := createWith(t, conn, &xid, backCreateSessionBody(clientID, seq, 0)); got != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION without a backchannel %s", nfs4.Nfsstat4Name(got))
	}
}

// A session that asked for no backchannel: BIND refuses the back
// direction and gives FORE for FORE_OR_BOTH, BACKCHANNEL_CTL is INVAL, and
// SET_SSV is INVAL.
func TestBackchannelOperationsWithoutBackchannel(t *testing.T) {
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	ts := connectionSession(t, srv)
	conn := &rpcConnection{}
	for _, dir := range []uint32{nfs4.CDFC4_BACK, nfs4.CDFC4_BACK_OR_BOTH} {
		_, res := admissionRequest(t, srv, admissionBind(ts.id, dir, false), 1, conn)
		admissionStatus(t, res, nfs4.NFS4ERR_INVAL, nfs4.OP_BIND_CONN_TO_SESSION)
	}
	_, res := admissionRequest(t, srv, admissionBind(ts.id, nfs4.CDFC4_FORE_OR_BOTH, false), 2, conn)
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_BIND_CONN_TO_SESSION)
	it := res.Resarray()
	it.Next()
	if dir := it.Resarray().Value().AsBINDCONNTOSESSION4resEntry().Value().AsBINDCONNTOSESSION4resok().BctsrDir(); dir != nfs4.CDFS4_FORE {
		t.Fatalf("BIND direction %d", dir)
	}
	if conn.carriesBack(ts.id) {
		t.Fatal("BIND made a back association")
	}

	ctl := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		bw := w.AppendArgarray_BackchannelCtl()
		bw.SetBcaCbProgram(1)
		bw.AppendBcaSecParms_None()
		w.Resume(bw.Finish())
	}), ts, 0, 1, true)
	_, res = admissionRequest(t, srv, ctl, 3, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_INVAL, nfs4.OP_SEQUENCE, nfs4.OP_BACKCHANNEL_CTL)

	ssv := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		sw := w.AppendArgarray_SetSsv()
		ow := sw.StartSsaSsv()
		sw.Resume(ow.SetData([]byte("ssv")).Finish())
		ow = sw.StartSsaDigest()
		sw.Resume(ow.SetData([]byte("digest")).Finish())
		w.Resume(sw.Finish())
	}), ts, 0, 2, true)
	_, res = admissionRequest(t, srv, ssv, 4, nil)
	admissionStatus(t, res, nfs4.NFS4ERR_INVAL, nfs4.OP_SEQUENCE, nfs4.OP_SET_SSV)
}

// A REPLY from the client with no callback waiting is dropped and counted;
// the connection carries on.
func TestStrayRepliesDropped(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	reply := binary.BigEndian.AppendUint32(nil, 77)
	reply = binary.BigEndian.AppendUint32(reply, rpcReply)
	reply = binary.BigEndian.AppendUint32(reply, 0) // MSG_ACCEPTED
	reply = binary.BigEndian.AppendUint32(reply, authNone)
	reply = binary.BigEndian.AppendUint32(reply, 0)
	reply = binary.BigEndian.AppendUint32(reply, acceptSuccess)
	for range 3 {
		if err := writeFrame(conn, reply); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sendRawRPC(conn, 78, procNull, nil); err != nil {
		t.Fatalf("NULL after stray replies: %v", err)
	}
	if got := srv.backchannel.strayReplies.Load(); got != 3 {
		t.Fatalf("stray replies %d, want 3", got)
	}
}

// A connection holds at most one CB_COMPOUND per session it carries
// callbacks for and one CB_NULL; replies are delivered once, by XID.
func TestCallbackMapsBounded(t *testing.T) {
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	c := &rpcConnection{send: func([]byte) bool { return true }}
	a, b := connectionSession(t, srv).id, connectionSession(t, srv).id
	call := func(xid uint32, id SessionID, null bool) *cbCall {
		return newCBCall(xid, id, null)
	}
	if c.registerCall(call(1, a, false)) {
		t.Fatal("CB_COMPOUND for a session the connection does not carry")
	}
	if !c.bindBack(srv, a) || !c.bindBack(srv, b) {
		t.Fatal("bindBack")
	}
	null := call(2, SessionID{}, true)
	if !c.registerCall(null) || c.registerCall(call(3, SessionID{}, true)) {
		t.Fatal("CB_NULL bound")
	}
	first := call(4, a, false)
	if !c.registerCall(first) || c.registerCall(call(5, a, false)) || !c.registerCall(call(6, b, false)) {
		t.Fatal("one CB_COMPOUND per session")
	}
	if c.registerCall(call(4, b, false)) {
		t.Fatal("XID reused")
	}
	if c.outstandingCalls() != 3 {
		t.Fatalf("outstanding %d", c.outstandingCalls())
	}
	frame := binary.BigEndian.AppendUint32(nil, 4)
	frame = binary.BigEndian.AppendUint32(frame, rpcReply)
	if !c.deliverReply(frame) || c.deliverReply(frame) {
		t.Fatal("reply delivered other than once")
	}
	if got := <-first.reply; !bytes.Equal(got, frame) {
		t.Fatal("delivered frame")
	}
	c.cancelCall(null)
	if c.outstandingCalls() != 1 || !c.registerCall(call(7, SessionID{}, true)) {
		t.Fatal("cancel did not free the CB_NULL")
	}
	for len(c.backSessions()) < maxConnectionSessions {
		if !c.bindBack(srv, connectionSession(t, srv).id) {
			t.Fatalf("bindBack %d", len(c.backSessions()))
		}
	}
	if c.bindBack(srv, connectionSession(t, srv).id) {
		t.Fatal("back associations past the bound")
	}
	// A retired session's back association is pruned on the next
	// reservation, freeing its place.
	srv.sessions.destroySession(a)
	if !c.bindBack(srv, connectionSession(t, srv).id) {
		t.Fatal("a retired session's back association was not pruned")
	}
}
