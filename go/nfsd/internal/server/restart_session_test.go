// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// A restart onto a fresh staging disk, with no session files, loses the
// session: the retransmission gets BADSESSION and does not run again, and
// so do a new request on the old session and a BIND. With the staging disk
// kept, the session survives (session_restore_test.go).
func TestRestartRetransmissionGetsBadSession(t *testing.T) {
	root := t.TempDir()
	newServer := func() (*Server, string, func()) {
		staging, err := staging.NewLocalStore(t.TempDir(), nil)
		if err != nil {
			t.Fatal(err)
		}
		srv, err := NewServer(vfs.NewLocalTernVFS(root), staging, nil)
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
