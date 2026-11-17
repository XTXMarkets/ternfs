// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"fmt"
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
	data, err := readAll(srv.fs, id)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Separate READ and WRITE opens by one owner have distinct handles, staging
// and grants. DOWNGRADE cannot acquire the reader's grant; a same-value WRITE
// downgrade advances the writer's state without publishing it.
func TestOpenDowngradeWithIndependentReadOpen(t *testing.T) {
	t.Run("to-read-is-invalid", func(t *testing.T) {
		srv, conn, xid, clientID, _ := openGrantFixture(t, "old")
		status, reader, readerFH, _ := openFileForOwner(t, conn, xid, clientID, "o", "f", nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
		if status != nfs4.NFS4_OK {
			t.Fatal(nfs4.Nfsstat4Name(status))
		}
		status, writer, writerFH, _ := openFileForOwner(t, conn, xid, clientID, "o", "f", nfs4.OPEN4_SHARE_ACCESS_WRITE, false, false)
		if status != nfs4.NFS4_OK {
			t.Fatal(nfs4.Nfsstat4Name(status))
		}
		writeFileAt(t, conn, xid, writerFH, writer, 0, []byte("new"))
		if got, _ := downgradeResult(t, conn, xid, writerFH, writer, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE); got != nfs4.NFS4ERR_INVAL {
			t.Fatalf("downgrade to READ %s, want INVAL", nfs4.Nfsstat4Name(got))
		}
		if got := publishedData(t, srv); got != "old" {
			t.Fatalf("published %q after invalid downgrade", got)
		}
		if status, data := readStateid(t, conn, xid, readerFH, reader); status != nfs4.NFS4_OK || data != "old" {
			t.Fatalf("independent reader %s %q, want old", nfs4.Nfsstat4Name(status), data)
		}
		if status, data := readStateid(t, conn, xid, writerFH, writer); status != nfs4.NFS4_OK || data != "new" {
			t.Fatalf("writer after invalid downgrade %s %q, want new", nfs4.Nfsstat4Name(status), data)
		}
		closeFile(t, conn, xid, writerFH, writer)
		if got := publishedData(t, srv); got != "new" {
			t.Fatalf("published %q after writer CLOSE", got)
		}
		closeFile(t, conn, xid, readerFH, reader)
	})
	t.Run("to-write-keeps-writer", func(t *testing.T) {
		srv, conn, xid, clientID, _ := openGrantFixture(t, "old")
		status, reader, readerFH, _ := openFileForOwner(t, conn, xid, clientID, "o", "f", nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
		if status != nfs4.NFS4_OK {
			t.Fatal(nfs4.Nfsstat4Name(status))
		}
		status, writer, writerFH, _ := openFileForOwner(t, conn, xid, clientID, "o", "f", nfs4.OPEN4_SHARE_ACCESS_WRITE, false, false)
		if status != nfs4.NFS4_OK {
			t.Fatal(nfs4.Nfsstat4Name(status))
		}
		writeFileAt(t, conn, xid, writerFH, writer, 0, []byte("abc"))
		status, write := downgradeResult(t, conn, xid, writerFH, writer, nfs4.OPEN4_SHARE_ACCESS_WRITE, nfs4.OPEN4_SHARE_DENY_NONE)
		if status != nfs4.NFS4_OK {
			t.Fatalf("downgrade to WRITE %s", nfs4.Nfsstat4Name(status))
		}
		if got := publishedData(t, srv); got != "old" {
			t.Fatalf("downgrade to WRITE published %q", got)
		}
		if status, data := readStateid(t, conn, xid, readerFH, reader); status != nfs4.NFS4_OK || data != "old" {
			t.Fatalf("independent reader %s %q, want old", nfs4.Nfsstat4Name(status), data)
		}
		writeFileAt(t, conn, xid, writerFH, write, 3, []byte("def"))
		// READ with a write-only open stays allowed.
		if got := stateidOpStatus(t, conn, xid, "READ", writerFH, write); got != nfs4.NFS4_OK {
			t.Fatalf("READ with a write-only open %s", nfs4.Nfsstat4Name(got))
		}
		closeFile(t, conn, xid, writerFH, write)
		if got := publishedData(t, srv); got != "abcdef" {
			t.Fatalf("published %q", got)
		}
		closeFile(t, conn, xid, readerFH, reader)
	})
}

// A downgrade target must be a valid share value, a subset of the open's
// grants, and the union of the recorded values inside it.
func TestOpenDowngradeInvalidTargets(t *testing.T) {
	_, conn, xid, clientID, _ := openGrantFixture(t, "old")
	status, both, writerFH, _ := openFileForOwner(t, conn, xid, clientID, "both", "f", nfs4.OPEN4_SHARE_ACCESS_BOTH, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	status, read, readerFH, _ := openFileForOwner(t, conn, xid, clientID, "read", "f", nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	for _, c := range []struct {
		name         string
		fh           []byte
		sid          [16]byte
		access, deny uint32
	}{
		{"BOTH to READ, never granted alone", writerFH, both, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE},
		{"BOTH to WRITE, never granted alone", writerFH, both, nfs4.OPEN4_SHARE_ACCESS_WRITE, nfs4.OPEN4_SHARE_DENY_NONE},
		{"upgrade", readerFH, read, nfs4.OPEN4_SHARE_ACCESS_BOTH, nfs4.OPEN4_SHARE_DENY_NONE},
		{"no access", writerFH, both, 0, nfs4.OPEN4_SHARE_DENY_NONE},
		{"bad access", writerFH, both, 4, nfs4.OPEN4_SHARE_DENY_NONE},
		{"bad deny", writerFH, both, nfs4.OPEN4_SHARE_ACCESS_BOTH, 4},
		{"deny never requested", writerFH, both, nfs4.OPEN4_SHARE_ACCESS_BOTH, nfs4.OPEN4_SHARE_DENY_READ},
	} {
		if got, _ := downgradeResult(t, conn, xid, c.fh, c.sid, c.access, c.deny); got != nfs4.NFS4ERR_INVAL {
			t.Errorf("%s: %s, want INVAL", c.name, nfs4.Nfsstat4Name(got))
		}
	}
	// The same value changes nothing but the seqid.
	status, same := downgradeResult(t, conn, xid, writerFH, both, nfs4.OPEN4_SHARE_ACCESS_BOTH|nfs4.OPEN4_SHARE_ACCESS_WANT_NO_DELEG, nfs4.OPEN4_SHARE_DENY_NONE)
	if status != nfs4.NFS4_OK {
		t.Fatalf("same-value downgrade %s", nfs4.Nfsstat4Name(status))
	}
	if binary.BigEndian.Uint32(same[:4]) != binary.BigEndian.Uint32(both[:4])+1 {
		t.Fatal("same-value downgrade kept the seqid")
	}
	if got, _ := downgradeResult(t, conn, xid, writerFH, both, nfs4.OPEN4_SHARE_ACCESS_BOTH, nfs4.OPEN4_SHARE_DENY_NONE); got != nfs4.NFS4ERR_OLD_STATEID {
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
	var id nfs4.StateID
	copy(id[:], read[4:])
	if !restarted.srv.opens.Live(id) {
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

// Removing the writer's name does not merge its grants with an independent
// read open. A write-only open cannot downgrade to READ; its original state
// still reads its private image and its CLOSE discards the detached staging.
func TestDetachedDowngradeRejectsIndependentReadGrant(t *testing.T) {
	for _, write := range []bool{true, false} {
		t.Run(fmt.Sprintf("written=%t", write), func(t *testing.T) {
			f := newPublishFixture(t)
			sid, fh, _ := f.writer(t, "doc", "base")
			closeFile(t, f.conn, f.xid, fh, sid)
			status, reader, readerFH, _ := openFileForOwner(t, f.conn, f.xid, f.clientID, "o", "doc", nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
			if status != nfs4.NFS4_OK {
				t.Fatalf("reader OPEN %s", nfs4.Nfsstat4Name(status))
			}
			status, writer, writerFH, _ := openFileForOwner(t, f.conn, f.xid, f.clientID, "o", "doc", nfs4.OPEN4_SHARE_ACCESS_WRITE, false, false)
			if status != nfs4.NFS4_OK {
				t.Fatalf("writer OPEN %s", nfs4.Nfsstat4Name(status))
			}
			want := "base"
			if write {
				writeFileAt(t, f.conn, f.xid, writerFH, writer, 0, []byte("BA"))
				want = "BAse"
			}
			conn, xid, _ := f.otherClient(t)
			removeFile(t, conn, xid, "doc")
			if got, _ := downgradeResult(t, f.conn, f.xid, writerFH, writer, nfs4.OPEN4_SHARE_ACCESS_READ, nfs4.OPEN4_SHARE_DENY_NONE); got != nfs4.NFS4ERR_INVAL {
				t.Fatalf("downgrade of the detached writer %s, want INVAL", nfs4.Nfsstat4Name(got))
			}
			if status, data := readStateid(t, f.conn, f.xid, writerFH, writer); status != nfs4.NFS4_OK || data != want {
				t.Fatalf("READ after the invalid downgrade %s %q, want %q", nfs4.Nfsstat4Name(status), data, want)
			}
			if got := closeFileStatus(t, f.conn, f.xid, writerFH, writer); got != nfs4.NFS4_OK {
				t.Fatalf("writer CLOSE %s", nfs4.Nfsstat4Name(got))
			}
			if got := closeFileStatus(t, f.conn, f.xid, readerFH, reader); got != nfs4.NFS4_OK {
				t.Fatalf("reader CLOSE %s", nfs4.Nfsstat4Name(got))
			}
			if n := len(f.store.Entries()); n != 0 {
				t.Fatalf("%d staging entries after CLOSE", n)
			}
		})
	}
}

// A separate reader has the published placeholder, not the writer's private
// staging. A detached writer's CLOSE discards its staging and leaves that
// reader on the placeholder.
func TestDetachedCloseKeepsReaderOnPublishedImage(t *testing.T) {
	f := newPublishFixture(t)
	sid, fh, stagingID := f.writer(t, "doc", "granted")
	status, reader, readerFH, _ := openFileForOwner(t, f.conn, f.xid, f.clientID, "reader", "doc", nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatalf("reader OPEN %s", nfs4.Nfsstat4Name(status))
	}
	conn, xid, _ := f.otherClient(t)
	removeFile(t, conn, xid, "doc")
	if got := closeFileStatus(t, f.conn, f.xid, fh, sid); got != nfs4.NFS4_OK {
		t.Fatalf("writer CLOSE %s", nfs4.Nfsstat4Name(got))
	}
	if status, data := readStateid(t, f.conn, f.xid, readerFH, reader); status != nfs4.NFS4_OK || data != "" {
		t.Fatalf("reader after the writer's CLOSE %s %q, want the empty placeholder", nfs4.Nfsstat4Name(status), data)
	}
	if meta, ok := f.store.GetMeta(stagingID); ok {
		t.Fatalf("detached staging remains: %+v", meta)
	}
	if got := closeFileStatus(t, f.conn, f.xid, readerFH, reader); got != nfs4.NFS4_OK {
		t.Fatalf("reader CLOSE %s", nfs4.Nfsstat4Name(got))
	}
	if n := len(f.store.Entries()); n != 0 {
		t.Fatalf("%d staging entries after the reader's CLOSE", n)
	}
}
