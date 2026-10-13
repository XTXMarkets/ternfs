// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func openByHandle(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	clientID uint64,
	owner string,
	fh []byte,
	access uint32,
) (uint32, [16]byte, []byte) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(access)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientID)
		ow.Resume(ownerW.SetOwner([]byte(owner)).Finish())
		ow.SetOpenhow_Default(nfs4.OPEN4_NOCREATE)
		ow.SetClaim_Fh()
		w.Resume(ow.Finish())
		w.AppendArgarray_Getfh()
	})
	*xid++
	var stateid [16]byte
	if res.Status() != nfs4.NFS4_OK {
		return res.Status(), stateid, nil
	}
	iter := expectOK(t, res)
	nextOp(t, &iter)
	sid := nextOp(t, &iter).Value().AsOPEN4resEntry().Value().AsOPEN4resok().Stateid()
	binary.BigEndian.PutUint32(stateid[:4], sid.Seqid())
	for i := range 12 {
		stateid[4+i] = sid.Other(i)
	}
	got := nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()
	return nfs4.NFS4_OK, stateid, append([]byte(nil), got...)
}

func TestOpenClaimFH(t *testing.T) {
	addr, cleanup := startTestServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	state, fh := openCreateFile(t, conn, &xid, clientID, "file")
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("first"))
	closeFile(t, conn, &xid, fh, state)
	fh = lookupFH(t, conn, &xid, "file")

	status, reader, readerFH := openByHandle(t, conn, &xid, clientID, "reader", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
	if status != nfs4.NFS4_OK || !bytes.Equal(readerFH, fh) {
		t.Fatalf("read CLAIM_FH = %s", nfs4.Nfsstat4Name(status))
	}
	status, writer, writerFH := openByHandle(t, conn, &xid, clientID, "writer", fh, nfs4.OPEN4_SHARE_ACCESS_BOTH)
	if status != nfs4.NFS4_OK || !bytes.Equal(writerFH, fh) {
		t.Fatalf("write CLAIM_FH = %s", nfs4.Nfsstat4Name(status))
	}
	writeFileAt(t, conn, &xid, fh, writer, 0, []byte("SECOND"))
	closeFile(t, conn, &xid, fh, writer)
	if data, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "file"), 0, 64); string(data) != "SECOND" {
		t.Fatalf("published %q", data)
	}
	closeFile(t, conn, &xid, fh, reader)

	// After the closing client's alias lapses, the old handle names the
	// replaced version, whose name now belongs to another inode. A write
	// open by handle then reports STALE, and the client looks up the name.
	other := dial(t, addr)
	defer other.Close()
	otherID, confirm := requestClientID(t, other, &xid, "other", [8]byte{2})
	if status := confirmClientID(t, other, &xid, otherID, confirm); status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	if status, _, _ := openByHandle(t, other, &xid, otherID, "writer", fh, nfs4.OPEN4_SHARE_ACCESS_BOTH); status != nfs4.NFS4ERR_STALE {
		t.Fatalf("write CLAIM_FH of replaced file = %s, want STALE", nfs4.Nfsstat4Name(status))
	}
	if status, sid, _ := openByHandle(t, other, &xid, otherID, "reader", fh, nfs4.OPEN4_SHARE_ACCESS_READ); status != nfs4.NFS4_OK {
		t.Fatalf("read CLAIM_FH of replaced file = %s", nfs4.Nfsstat4Name(status))
	} else {
		if data, _ := readFileData(t, other, &xid, fh, 0, 64); string(data) != "first" {
			t.Fatalf("replaced version reads %q", data)
		}
		closeFile(t, other, &xid, fh, sid)
	}
}

// rsync --inplace: one owner reads the file, then opens it for write, which
// upgrades its open. The writes must publish.
func TestOpenUpgradePublishesWrites(t *testing.T) {
	addr, cleanup := startTestServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	state, fh := openCreateFile(t, conn, &xid, clientID, "target")
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("old target\n"))
	closeFile(t, conn, &xid, fh, state)
	fh = lookupFH(t, conn, &xid, "target")
	status, reader, _ := openByHandle(t, conn, &xid, clientID, "owner", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	status, upgraded, _ := openByHandle(t, conn, &xid, clientID, "owner", fh, nfs4.OPEN4_SHARE_ACCESS_BOTH)
	if status != nfs4.NFS4_OK || !bytes.Equal(upgraded[4:], reader[4:]) {
		t.Fatalf("upgrade = %s", nfs4.Nfsstat4Name(status))
	}
	current := upgraded
	binary.BigEndian.PutUint32(current[:4], 0)
	writeFileAt(t, conn, &xid, fh, current, 0, []byte("replacement from rsync --inplace\n"))
	// Linux closes the write by downgrading to the read it still holds.
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		dw := w.AppendArgarray_OpenDowngrade()
		setStateid(dw.OpenStateid(), current)
		dw.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_READ)
		dw.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
	})
	xid++
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("OPEN_DOWNGRADE = %s", nfs4.Nfsstat4Name(res.Status()))
	}
	if data, _ := readFileData(t, conn, &xid, fh, 0, 64); string(data) != "replacement from rsync --inplace\n" {
		t.Fatalf("read after downgrade %q", data)
	}
	closeFile(t, conn, &xid, fh, current)
	if data, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "target"), 0, 64); string(data) != "replacement from rsync --inplace\n" {
		t.Fatalf("published %q", data)
	}
}

func downgradeForTest(t *testing.T, conn net.Conn, xid *uint32, fh []byte, sid [16]byte, access uint32) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		dw := w.AppendArgarray_OpenDowngrade()
		setStateid(dw.OpenStateid(), sid)
		dw.SetShareAccess(access)
		dw.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
	})
	*xid++
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("OPEN_DOWNGRADE = %s", nfs4.Nfsstat4Name(res.Status()))
	}
}

// The rsync --inplace sequence seen from Linux: write open, read open by the
// same owner, read, downgrade to write, truncate, write, close.
func TestRsyncInPlaceSequence(t *testing.T) {
	addr, cleanup := startTestServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	state, fh := openCreateFile(t, conn, &xid, clientID, "target")
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("old target\n"))
	closeFile(t, conn, &xid, fh, state)
	fh = lookupFH(t, conn, &xid, "target")
	status, sid, _ := openByHandle(t, conn, &xid, clientID, "owner", fh, nfs4.OPEN4_SHARE_ACCESS_WRITE)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	status, sid, _ = openByHandle(t, conn, &xid, clientID, "owner", fh, nfs4.OPEN4_SHARE_ACCESS_READ)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	readFileData(t, conn, &xid, fh, 0, 64)
	downgradeForTest(t, conn, &xid, fh, sid, nfs4.OPEN4_SHARE_ACCESS_WRITE)
	cur := sid
	binary.BigEndian.PutUint32(cur[:4], 0)
	setSizeForTest(t, conn, &xid, fh, cur, 33)
	writeFileAt(t, conn, &xid, fh, cur, 0, []byte("replacement from rsync --inplace\n"))
	closeFile(t, conn, &xid, fh, cur)
	if data, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "target"), 0, 64); string(data) != "replacement from rsync --inplace\n" {
		t.Fatalf("published %q", data)
	}
}

func setSizeForTest(t *testing.T, conn net.Conn, xid *uint32, fh []byte, sid [16]byte, size uint64) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		sw := w.AppendArgarray_Setattr()
		setStateid(sw.Stateid(), sid)
		aw := sw.StartObjAttributes()
		sw.Resume(finishTestFattr(aw, attrMask{1 << nfs4.FATTR4_SIZE}, binary.BigEndian.AppendUint64(nil, size)))
		w.Resume(sw.Finish())
	})
	*xid++
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("SETATTR size = %s", nfs4.Nfsstat4Name(res.Status()))
	}
}

// OPEN of a file waits for another client's publishing CLOSE of it, and
// opens what the CLOSE published.
func TestOpenWaitsForPublishingClose(t *testing.T) {
	root := t.TempDir()
	fs := &mutableBlockingLinkVFS{
		LocalTernVFS: vfs.NewLocalTernVFS(root),
		started:      make(chan struct{}),
		release:      make(chan struct{}),
	}
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	writer := dial(t, addr)
	defer writer.Close()
	reader := dial(t, addr)
	defer reader.Close()
	xid := uint32(1)
	writerID := setupClient(t, writer, &xid)
	readerID, confirm := requestClientID(t, reader, &xid, "reader", [8]byte{2})
	if status := confirmClientID(t, reader, &xid, readerID, confirm); status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	state, fh := openWriteFile(t, writer, &xid, writerID, "file")
	writeFileAt(t, writer, &xid, fh, state, 0, []byte("new"))
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		xid := uint32(1000)
		closeFile(t, writer, &xid, fh, state)
	}()
	<-fs.started
	opened := make(chan []byte, 1)
	go func() {
		xid := uint32(2000)
		status, sid, gotFH, _ := openFileForOwner(t, reader, &xid, readerID, "reader", "file",
			nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
		if status != nfs4.NFS4_OK {
			t.Error(nfs4.Nfsstat4Name(status))
			opened <- nil
			return
		}
		data, _ := readFileData(t, reader, &xid, gotFH, 0, 64)
		closeFile(t, reader, &xid, gotFH, sid)
		opened <- data
	}()
	select {
	case <-opened:
		t.Fatal("OPEN did not wait for the publishing CLOSE")
	case <-time.After(50 * time.Millisecond):
	}
	close(fs.release)
	<-closed
	if data := <-opened; string(data) != "new" {
		t.Fatalf("OPEN after CLOSE read %q, want %q", data, "new")
	}
}
