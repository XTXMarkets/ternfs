// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sidecar"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func scopedStateid(seqid uint32, id nfs4.StateID) [16]byte {
	var wire [16]byte
	binary.BigEndian.PutUint32(wire[:4], seqid)
	copy(wire[4:], id[:])
	return wire
}

func putfhOp(w *nfs4.COMPOUND4argsWriter, fh []byte) {
	pw := w.AppendArgarray_Putfh()
	pw.Resume(pw.StartObject().SetData(fh).Finish())
	w.Resume(pw.Finish())
}

// stateidOpStatus sends one stateid operation on fh and returns its status.
func stateidOpStatus(t *testing.T, conn net.Conn, xid *uint32, op string, fh []byte, sid [16]byte) uint32 {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		putfhOp(w, fh)
		switch op {
		case "READ":
			rw := w.AppendArgarray_Read()
			setStateid(rw.Stateid(), sid)
			rw.SetOffset(0)
			rw.SetCount(64)
		case "WRITE":
			ww := w.AppendArgarray_Write()
			setStateid(ww.Stateid(), sid)
			ww = ww.SetOffset(0)
			ww = ww.SetStable(fileSync4)
			ww = ww.SetData([]byte("foreign"))
			w.Resume(ww.Finish())
		case "SETATTR":
			sw := w.AppendArgarray_Setattr()
			setStateid(sw.Stateid(), sid)
			aw := sw.StartObjAttributes()
			sw.Resume(finishTestFattr(aw, attrMask{1 << nfs4.FATTR4_SIZE}, binary.BigEndian.AppendUint64(nil, 0)))
			w.Resume(sw.Finish())
		case "SETATTR-TIMES":
			sw := w.AppendArgarray_Setattr()
			setStateid(sw.Stateid(), sid)
			aw := sw.StartObjAttributes()
			value := binary.BigEndian.AppendUint32(nil, nfs4.SET_TO_CLIENT_TIME4)
			value = binary.BigEndian.AppendUint64(value, 1_000_000)
			value = binary.BigEndian.AppendUint32(value, 0)
			sw.Resume(finishTestFattr(aw, attrMask{0, 1 << (nfs4.FATTR4_TIME_MODIFY_SET - 32)}, value))
			w.Resume(sw.Finish())
		case "CLOSE":
			cw := w.AppendArgarray_Close()
			setStateid(cw.OpenStateid(), sid)
		case "DOWNGRADE-READ", "DOWNGRADE-WRITE":
			dw := w.AppendArgarray_OpenDowngrade()
			setStateid(dw.OpenStateid(), sid)
			access := uint32(nfs4.OPEN4_SHARE_ACCESS_READ)
			if op == "DOWNGRADE-WRITE" {
				access = nfs4.OPEN4_SHARE_ACCESS_WRITE
			}
			dw.SetShareAccess(access)
			dw.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		case "FREE_STATEID":
			fw := w.AppendArgarray_FreeStateid()
			setStateid(fw.FsaStateid(), sid)
		case "TEST_STATEID":
			tw := w.AppendArgarray_TestStateid()
			setStateid(tw.AppendTsStateids(), sid)
			w.Resume(tw.Finish())
		default:
			t.Fatalf("unknown op %s", op)
		}
	})
	*xid++
	if op != "TEST_STATEID" {
		return res.Status()
	}
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("TEST_STATEID %s", nfs4.Nfsstat4Name(res.Status()))
	}
	iter := res.Resarray()
	iter.Next()
	iter.Next()
	ok := iter.Resarray().Value().AsTESTSTATEID4resEntry().Value().AsTESTSTATEID4resok()
	if ok.TsrStatusCodesCount() != 1 {
		t.Fatal("TEST_STATEID result count")
	}
	return binary.BigEndian.Uint32(ok[4:])
}

// Client B presenting client A's live, recovered or expired stateid gets
// BAD_STATEID from every stateid operation, and A's opens, staging and
// markers survive (canonical r3 §10).
func TestForeignStateidsAreBadStateid(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	connA := dial(t, addr)
	defer connA.Close()
	xidA := uint32(1)
	wireA := setupClient(t, connA, &xidA)
	incA := incarnationForTest(t, srv, wireA)

	liveSid, liveFH := openCreateFile(t, connA, &xidA, wireA, "a-live")
	writeFileAt(t, connA, &xidA, liveFH, liveSid, 0, []byte("A's bytes"))
	expSid, expFH := openCreateFile(t, connA, &xidA, wireA, "a-expired")
	var expID nfs4.StateID
	copy(expID[:], expSid[4:])
	srv.opens.Expire(expID)

	// A write open from an earlier process, recovered through its staging.
	root := srv.fs.RootID()
	recStaging, cookie, err := srv.fs.ConstructFile(root)
	if err != nil {
		t.Fatal(err)
	}
	recID := nfs4.StateID{1, 2, 3, 4, 0, 0, 0, 0, 0, 0, 0, 9}
	if binary.BigEndian.Uint32(recID[:4]) == srv.opens.Epoch {
		recID[0]++
	}
	if _, err := srv.stagingStore.Create(recStaging, sidecar.Meta{DirID: root, FileName: "a-recovered",
		TernCookie: cookie, NFSStateID: recID, ClientID: incA}); err != nil {
		t.Fatal(err)
	}
	if err := srv.clients.MarkOpen(incA, recID); err != nil {
		t.Fatal(err)
	}
	recSid, recFH := scopedStateid(1, recID), vfs.InodeIDToFH(recStaging)

	connB := dial(t, addr)
	defer connB.Close()
	if _, err := createTestSession(connB, 1, "client-b", [8]byte{2}); err != nil {
		t.Fatal(err)
	}
	xidB := uint32(100)
	for _, target := range []struct {
		name string
		fh   []byte
		sid  [16]byte
	}{
		{"live", liveFH, liveSid},
		{"recovered", recFH, recSid},
		{"expired", expFH, expSid},
	} {
		for _, op := range []string{"READ", "WRITE", "SETATTR", "SETATTR-TIMES", "CLOSE", "DOWNGRADE-READ", "DOWNGRADE-WRITE", "TEST_STATEID", "FREE_STATEID"} {
			if got := stateidOpStatus(t, connB, &xidB, op, target.fh, target.sid); got != nfs4.NFS4ERR_BAD_STATEID {
				t.Errorf("B's %s of A's %s stateid = %s, want BAD_STATEID", op, target.name, nfs4.Nfsstat4Name(got))
			}
		}
	}

	// A's state survived.
	var liveID nfs4.StateID
	copy(liveID[:], liveSid[4:])
	liveFile, _ := vfs.FHToInodeID(liveFH)
	state, status := srv.opens.Lookup(incA, liveID, 0, liveFile)
	if status != nfs4.NFS4_OK {
		t.Fatalf("A's live open %s", nfs4.Nfsstat4Name(status))
	}
	for _, id := range []vfs.InodeID{state.DataID, recStaging} {
		if m, ok := srv.stagingStore.GetMeta(id); !ok || m.Retired {
			t.Fatalf("A's staging %d retired", id)
		}
	}
	for _, id := range []nfs4.StateID{liveID, recID} {
		if open, err := srv.clients.HasOpen(incA, id); err != nil || !open {
			t.Fatalf("A's marker %x: %t %v", id, open, err)
		}
	}
	for _, check := range []struct {
		fh   []byte
		sid  [16]byte
		want uint32
	}{
		{liveFH, liveSid, nfs4.NFS4_OK},
		{recFH, recSid, nfs4.NFS4_OK},
		{expFH, expSid, nfs4.NFS4ERR_EXPIRED},
	} {
		if got := stateidOpStatus(t, connA, &xidA, "TEST_STATEID", check.fh, check.sid); got != check.want {
			t.Fatalf("A's TEST_STATEID %s, want %s", nfs4.Nfsstat4Name(got), nfs4.Nfsstat4Name(check.want))
		}
	}
	if data, _ := readFileData(t, connA, &xidA, liveFH, 0, 64); string(data) != "A's bytes" {
		t.Fatalf("A's data %q", data)
	}

	// FREE_STATEID: A's own expired stateid is forgotten; an unknown one is
	// BAD_STATEID; a live one is LOCKS_HELD.
	if got := stateidOpStatus(t, connA, &xidA, "FREE_STATEID", expFH, expSid); got != nfs4.NFS4_OK {
		t.Fatalf("A's FREE_STATEID of its expired stateid %s", nfs4.Nfsstat4Name(got))
	}
	if got := stateidOpStatus(t, connA, &xidA, "TEST_STATEID", expFH, expSid); got != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("freed stateid %s", nfs4.Nfsstat4Name(got))
	}
	unknown := scopedStateid(1, nfs4.StateID{9, 9, 9, 9})
	if got := stateidOpStatus(t, connA, &xidA, "FREE_STATEID", liveFH, unknown); got != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("FREE_STATEID of an unknown stateid %s", nfs4.Nfsstat4Name(got))
	}
	if got := stateidOpStatus(t, connA, &xidA, "FREE_STATEID", liveFH, liveSid); got != nfs4.NFS4ERR_LOCKS_HELD {
		t.Fatalf("FREE_STATEID of a live stateid %s", nfs4.Nfsstat4Name(got))
	}
}
