// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"fmt"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// A backend that does not answer a GETATTR in time gets DELAY, not IO.
// Linux closes a read open with PUTFH GETATTR CLOSE and forgets the
// stateid when that compound fails with IO, so an IO answer would leave the
// open registered for as long as the client renews its lease. On DELAY the
// client retries the whole compound, and the CLOSE runs once the backend
// answers.
func TestGetattrBackendTimeoutDelaysCloseCompound(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, _ := f.writer(t, "doc", "saved")
	mustCloseStatus(t, f, fh, sid, nfs4.NFS4_OK, "publish")
	f.mustDrained(t, "after publish")

	status, rsid, rfh, _ := openFileForOwner(t, f.conn, f.xid, f.clientID, "reader", "doc", nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatalf("reader OPEN %s", nfs4.Nfsstat4Name(status))
	}
	if n := f.srv.opens.StateCount(); n != 1 {
		t.Fatalf("%d opens after the read OPEN, want 1", n)
	}

	closeCompound := func() (uint32, []uint32) {
		res := sendCompound(t, f.conn, *f.xid, func(w *nfs4.COMPOUND4argsWriter) {
			putfhOp(w, rfh)
			gw := w.AppendArgarray_Getattr()
			bw := gw.StartAttrRequest()
			bw.AppendData(1<<nfs4.FATTR4_CHANGE | 1<<nfs4.FATTR4_SIZE)
			bw.AppendData(1 << (nfs4.FATTR4_TIME_MODIFY - 32))
			gw.Resume(bw.Finish())
			w.Resume(gw.Finish())
			caw := w.AppendArgarray_Close()
			setStateid(caw.OpenStateid(), rsid)
		})
		*f.xid++
		var ops []uint32
		iter := res.Resarray()
		for iter.Next() {
			ops = append(ops, iter.Resarray().Value().Disc())
		}
		return res.Status(), ops
	}

	f.fs.set(func() { f.fs.statTimeouts = 1 })
	status, ops := closeCompound()
	if status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("close compound with the backend Stat timing out: %s (ops %v), want NFS4ERR_DELAY", nfs4.Nfsstat4Name(status), ops)
	}
	if n := f.srv.opens.StateCount(); n != 1 {
		t.Fatalf("%d opens after the delayed compound, want the read open kept", n)
	}

	status, ops = closeCompound()
	want := []uint32{nfs4.OP_PUTFH, nfs4.OP_GETATTR, nfs4.OP_CLOSE}
	if status != nfs4.NFS4_OK || fmt.Sprint(ops) != fmt.Sprint(want) {
		t.Fatalf("retried close compound: %s, ops %v, want NFS4_OK with %v", nfs4.Nfsstat4Name(status), ops, want)
	}
	f.mustDrained(t, "after the retried CLOSE")
}
