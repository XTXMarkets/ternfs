// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// Sessions end with their nfsd process (canonical r3 §6). A retransmission
// of a request the earlier process completed gets BADSESSION from the
// restarted one and does not run again; BIND_CONN_TO_SESSION and a new
// request on the old session get BADSESSION too, and nothing is rebuilt.
func TestRestartRetransmissionGetsBadSession(t *testing.T) {
	root := t.TempDir()
	newServer := func() (*Server, string, func()) {
		staging, err := NewLocalStagingStore(t.TempDir(), nil)
		if err != nil {
			t.Fatal(err)
		}
		srv, err := NewServer(NewLocalTernVFS(root), staging, nil)
		if err != nil {
			t.Fatal(err)
		}
		addr, stop := serveTestServer(t, srv)
		return srv, addr, stop
	}
	victim := filepath.Join(root, "victim")
	if err := os.WriteFile(victim, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, addr, stop := newServer()
	conn := dial(t, addr)
	xid := uint32(1)
	wire := setupClient(t, conn, &xid)
	ts := testSessionForClient(conn, wire)
	remove := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rw := w.AppendArgarray_Remove()
		tw := rw.StartTarget()
		rw.Resume(tw.SetData([]byte("victim")).Finish())
		w.Resume(rw.Finish())
	})
	slot, seq := ts.acquire()
	request := withSequence(remove, ts, slot, seq)
	reply, err := sendRawRPC(conn, xid, procCompound, request)
	ts.release(slot)
	xid++
	if err != nil {
		t.Fatal(err)
	}
	if res, err := parseCompoundReply(reply); err != nil || res.Status() != nfs4.NFS4_OK {
		t.Fatalf("REMOVE before restart %v", err)
	}
	conn.Close()
	stop()

	// The name is in use again when the retransmission arrives.
	if err := os.WriteFile(victim, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv, addr, stop := newServer()
	defer stop()
	conn = dial(t, addr)
	defer conn.Close()
	reply, err = sendRawRPC(conn, xid, procCompound, request)
	xid++
	if err != nil {
		t.Fatal(err)
	}
	if got := sequenceStatus(reply); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("retransmission after restart %s", nfs4.Nfsstat4Name(got))
	}
	if data, err := os.ReadFile(victim); err != nil || string(data) != "second" {
		t.Fatalf("retransmitted REMOVE ran again: %q %v", data, err)
	}
	// A new request on the old session, and BIND, get BADSESSION too.
	next := withSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, slot, seq+1)
	if reply, err := sendRawRPC(conn, xid, procCompound, next); err != nil || sequenceStatus(reply) != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("SEQUENCE on the old session %s %v", nfs4.Nfsstat4Name(sequenceStatus(reply)), err)
	}
	xid++
	res, err := parseCompoundReply(must(sendRawRPC(conn, xid, procCompound, admissionBind(ts.id, nfs4.CDFC4_FORE, false))))
	xid++
	if err != nil || res.Status() != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("BIND_CONN_TO_SESSION on the old session %s %v", nfs4.Nfsstat4Name(res.Status()), err)
	}
	if a := srv.sessions.accounting(); a.Sessions != 0 || a.Records != 0 || a.Groups != 0 {
		t.Fatalf("old session rebuilt state %+v", a)
	}
}

// The approved restart path for a staged writer, on the wire: the old
// clientid is stale; EXCHANGE_ID gives a new clientid with sequence 1;
// CREATE_SESSION 1 adopts the still-confirmed incarnation; CLAIM_PREVIOUS
// gets NO_GRACE; a writable CLAIM_FH of the held handle recovers the
// writer; CLOSE publishes the data written before and after the restart.
func TestRestartRawRecoverySequence(t *testing.T) {
	f := newRestartedWriter(t, "file", false, nil)
	if _, status, err := createSession(f.conn, f.xid, f.oldWire, 2); err != nil || status != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION with the old clientid %s %v", nfs4.Nfsstat4Name(status), err)
	}
	f.xid++
	wire, seq, status, err := exchangeID(f.conn, f.xid, "test-client", [8]byte{})
	f.xid++
	if err != nil || status != nfs4.NFS4_OK || seq != 1 || wire == f.oldWire {
		t.Fatalf("EXCHANGE_ID %#x seq %d %s %v", wire, seq, nfs4.Nfsstat4Name(status), err)
	}
	if _, status, err := createSession(f.conn, f.xid, wire, 1); err != nil || status != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION 1 %s %v", nfs4.Nfsstat4Name(status), err)
	}
	f.xid++
	f.wire = wire
	if got := incarnationForTest(t, f.srv, wire); got != f.incarnation {
		t.Fatalf("new clientid bound incarnation %d, want the adopted %d", got, f.incarnation)
	}
	res := sendCompound(t, f.conn, f.xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(f.fh).Finish())
		w.Resume(pw.Finish())
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner().SetClientid(wire)
		ow.Resume(ownerW.SetOwner([]byte(f.owner)).Finish())
		ow.SetOpenhow_Default(nfs4.OPEN4_NOCREATE)
		ow.SetClaim_Previous()
		w.Resume(ow.Finish())
	})
	f.xid++
	if res.Status() != nfs4.NFS4ERR_NO_GRACE {
		t.Fatalf("CLAIM_PREVIOUS %s", nfs4.Nfsstat4Name(res.Status()))
	}
	status, sid, fh := f.claimFH(t, f.owner)
	if status != nfs4.NFS4_OK || !bytes.Equal(fh, f.fh) {
		t.Fatalf("CLAIM_FH %s", nfs4.Nfsstat4Name(status))
	}
	more := []byte(" and after")
	writeFileAt(t, f.conn, &f.xid, f.fh, sid, uint64(len(f.data)), more)
	closeFile(t, f.conn, &f.xid, f.fh, sid)
	want := append(append([]byte(nil), f.data...), more...)
	if got, err := os.ReadFile(filepath.Join(f.root, f.name)); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("published %q %v, want %q", got, err, want)
	}
}
