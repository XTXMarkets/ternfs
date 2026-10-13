// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// downgradeResult sends OPEN_DOWNGRADE and returns its status and stateid.
func downgradeResult(t *testing.T, conn net.Conn, xid *uint32, fh []byte, sid [16]byte, access, deny uint32) (uint32, [16]byte) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		putfhOp(w, fh)
		dw := w.AppendArgarray_OpenDowngrade()
		setStateid(dw.OpenStateid(), sid)
		dw.SetShareAccess(access)
		dw.SetShareDeny(deny)
	})
	*xid++
	var out [16]byte
	if res.Status() != nfs4.NFS4_OK {
		return res.Status(), out
	}
	iter := res.Resarray()
	iter.Next()
	iter.Next()
	got := iter.Resarray().Value().AsOPENDOWNGRADE4resEntry().Value().AsOPENDOWNGRADE4resok().OpenStateid()
	binary.BigEndian.PutUint32(out[:4], got.Seqid())
	for i := range 12 {
		out[4+i] = got.Other(i)
	}
	return nfs4.NFS4_OK, out
}

func openGrantFixture(t *testing.T, content string) (*Server, net.Conn, *uint32, uint64, []byte) {
	t.Helper()
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	t.Cleanup(cleanup)
	conn := dial(t, addr)
	t.Cleanup(func() { conn.Close() })
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	sid, fh := openCreateFile(t, conn, &xid, clientID, "f")
	writeFileAt(t, conn, &xid, fh, sid, 0, []byte(content))
	closeFile(t, conn, &xid, fh, sid)
	return srv, conn, &xid, clientID, lookupFH(t, conn, &xid, "f")
}

// publishedData reads the published file from the backend: the client
// itself would see its own writer's staging.
func publishedData(t *testing.T, srv *Server) string {
	t.Helper()
	id, err := srv.fs.Lookup(srv.fs.RootID(), "f")
	if err != nil {
		t.Fatal(err)
	}
	data, err := srv.fs.ReadAll(id)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// OPEN of R then W by one owner gives BOTH. Downgrading to R publishes the
// writer; downgrading to W keeps it. Each success advances the seqid, and
// the old seqid is then OLD_STATEID.
func TestOpenDowngradeFromSeparateGrants(t *testing.T) {
	t.Run("to-read-publishes", func(t *testing.T) {
		srv, conn, xid, clientID, fh := openGrantFixture(t, "old")
		if status, _, _ := openByHandle(t, conn, xid, clientID, "o", fh, nfs4.OPEN4_SHARE_ACCESS_READ); status != nfs4.NFS4_OK {
			t.Fatal(nfs4.Nfsstat4Name(status))
		}
		status, both, _ := openByHandle(t, conn, xid, clientID, "o", fh, nfs4.OPEN4_SHARE_ACCESS_WRITE)
		if status != nfs4.NFS4_OK {
			t.Fatal(nfs4.Nfsstat4Name(status))
		}
		writeFileAt(t, conn, xid, fh, both, 0, []byte("new"))
		status, read := downgradeResult(t, conn, xid, fh, both, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE)
		if status != nfs4.NFS4_OK {
			t.Fatalf("downgrade to READ %s", nfs4.Nfsstat4Name(status))
		}
		if binary.BigEndian.Uint32(read[:4]) != binary.BigEndian.Uint32(both[:4])+1 {
			t.Fatalf("seqid %d after %d", binary.BigEndian.Uint32(read[:4]), binary.BigEndian.Uint32(both[:4]))
		}
		if got := publishedData(t, srv); got != "new" {
			t.Fatalf("published %q after downgrade to READ", got)
		}
		if got := stateidOpStatus(t, conn, xid, "READ", fh, both); got != nfs4.NFS4ERR_OLD_STATEID {
			t.Fatalf("old seqid %s", nfs4.Nfsstat4Name(got))
		}
		if got := stateidOpStatus(t, conn, xid, "WRITE", fh, read); got != nfs4.NFS4ERR_OPENMODE {
			t.Fatalf("WRITE after downgrade to READ %s", nfs4.Nfsstat4Name(got))
		}
		closeFile(t, conn, xid, fh, read)
	})
	t.Run("to-write-keeps-writer", func(t *testing.T) {
		srv, conn, xid, clientID, fh := openGrantFixture(t, "old")
		openByHandle(t, conn, xid, clientID, "o", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
		_, both, _ := openByHandle(t, conn, xid, clientID, "o", fh, nfs4.OPEN4_SHARE_ACCESS_WRITE)
		writeFileAt(t, conn, xid, fh, both, 0, []byte("abc"))
		status, write := downgradeResult(t, conn, xid, fh, both, nfs4.OPEN4_SHARE_ACCESS_WRITE, nfs4.OPEN4_SHARE_DENY_NONE)
		if status != nfs4.NFS4_OK {
			t.Fatalf("downgrade to WRITE %s", nfs4.Nfsstat4Name(status))
		}
		if got := publishedData(t, srv); got != "old" {
			t.Fatalf("downgrade to WRITE published %q", got)
		}
		writeFileAt(t, conn, xid, fh, write, 3, []byte("def"))
		// READ with a write-only open stays allowed.
		if got := stateidOpStatus(t, conn, xid, "READ", fh, write); got != nfs4.NFS4_OK {
			t.Fatalf("READ with a write-only open %s", nfs4.Nfsstat4Name(got))
		}
		closeFile(t, conn, xid, fh, write)
		if got := publishedData(t, srv); got != "abcdef" {
			t.Fatalf("published %q", got)
		}
	})
}

// A downgrade target must be a valid share value, a subset of the open's
// grants, and the union of the recorded values inside it.
func TestOpenDowngradeInvalidTargets(t *testing.T) {
	_, conn, xid, clientID, fh := openGrantFixture(t, "old")
	status, both, _ := openByHandle(t, conn, xid, clientID, "both", fh, nfs4.OPEN4_SHARE_ACCESS_BOTH)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	status, read, _ := openByHandle(t, conn, xid, clientID, "read", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	for _, c := range []struct {
		name         string
		sid          [16]byte
		access, deny uint32
	}{
		{"BOTH to READ, never granted alone", both, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE},
		{"BOTH to WRITE, never granted alone", both, nfs4.OPEN4_SHARE_ACCESS_WRITE, nfs4.OPEN4_SHARE_DENY_NONE},
		{"upgrade", read, nfs4.OPEN4_SHARE_ACCESS_BOTH, nfs4.OPEN4_SHARE_DENY_NONE},
		{"no access", both, 0, nfs4.OPEN4_SHARE_DENY_NONE},
		{"bad access", both, 4, nfs4.OPEN4_SHARE_DENY_NONE},
		{"bad deny", both, nfs4.OPEN4_SHARE_ACCESS_BOTH, 4},
		{"deny never requested", both, nfs4.OPEN4_SHARE_ACCESS_BOTH, nfs4.OPEN4_SHARE_DENY_READ},
	} {
		if got, _ := downgradeResult(t, conn, xid, fh, c.sid, c.access, c.deny); got != nfs4.NFS4ERR_INVAL {
			t.Errorf("%s: %s, want INVAL", c.name, nfs4.Nfsstat4Name(got))
		}
	}
	// The same value changes nothing but the seqid.
	status, same := downgradeResult(t, conn, xid, fh, both, nfs4.OPEN4_SHARE_ACCESS_BOTH|nfs4.OPEN4_SHARE_ACCESS_WANT_NO_DELEG, nfs4.OPEN4_SHARE_DENY_NONE)
	if status != nfs4.NFS4_OK {
		t.Fatalf("same-value downgrade %s", nfs4.Nfsstat4Name(status))
	}
	if binary.BigEndian.Uint32(same[:4]) != binary.BigEndian.Uint32(both[:4])+1 {
		t.Fatal("same-value downgrade kept the seqid")
	}
	if got, _ := downgradeResult(t, conn, xid, fh, both, nfs4.OPEN4_SHARE_ACCESS_BOTH, nfs4.OPEN4_SHARE_DENY_NONE); got != nfs4.NFS4ERR_OLD_STATEID {
		t.Fatalf("downgrade with the old seqid %s", nfs4.Nfsstat4Name(got))
	}
}

// A writer recovered from an earlier process, downgraded to READ, publishes
// and continues as a live read open under the same stateid.
func TestOpenDowngradeRecoveredWriterToRead(t *testing.T) {
	restarted := restartWithStagedWrite(t, "recovered-downgrade.txt", []byte("staged data"))
	conn, xid := restarted.conn, restarted.xid
	status, read := downgradeResult(t, conn, &xid, restarted.fh, restarted.stateid, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE)
	if status != nfs4.NFS4_OK {
		t.Fatalf("recovered downgrade to READ %s", nfs4.Nfsstat4Name(status))
	}
	var id StateID
	copy(id[:], read[4:])
	if !restarted.srv.opens.live(id) {
		t.Fatal("recovered open did not become a live read open")
	}
	if data, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "recovered-downgrade.txt"), 0, 64); string(data) != "staged data" {
		t.Fatalf("published %q", data)
	}
	if got := stateidOpStatus(t, conn, &xid, "READ", restarted.fh, read); got != nfs4.NFS4_OK {
		t.Fatalf("READ through the adopted open %s", nfs4.Nfsstat4Name(got))
	}
	closeFile(t, conn, &xid, restarted.fh, read)
}
