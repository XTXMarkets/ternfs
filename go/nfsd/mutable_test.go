// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var mutableAttrMask = attrMask{
	1<<FATTR4_CHANGE | 1<<FATTR4_SIZE,
	1<<(FATTR4_TIME_ACCESS-32) | 1<<(FATTR4_TIME_METADATA-32) | 1<<(FATTR4_TIME_MODIFY-32),
}

func mutableAttrs(t *testing.T, conn net.Conn, xid *uint32, fh []byte) []byte {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(mutableAttrMask[0])
		bw.AppendData(mutableAttrMask[1])
		gw.Resume(bw.Finish())
		w.Resume(gw.Finish())
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	g := nextOp(t, &iter).Value().AsGETATTR4resEntry().Value().AsGETATTR4resok()
	return append([]byte(nil), getAttrData(t, g)...)
}

func verifyMutableAttrs(t *testing.T, conn net.Conn, xid *uint32, fh, attrs []byte) {
	t.Helper()
	for _, negate := range []bool{false, true} {
		res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			pw.Resume(pw.StartObject().SetData(fh).Finish())
			w.Resume(pw.Finish())
			if negate {
				vw := w.AppendArgarray_Nverify()
				vw.Resume(finishTestFattr(vw.StartObjAttributes(), mutableAttrMask, attrs))
				w.Resume(vw.Finish())
			} else {
				vw := w.AppendArgarray_Verify()
				vw.Resume(finishTestFattr(vw.StartObjAttributes(), mutableAttrMask, attrs))
				w.Resume(vw.Finish())
			}
		})
		*xid++
		want := uint32(NFS4_OK)
		if negate {
			want = NFS4ERR_SAME
		}
		if res.Status() != want {
			t.Fatalf("NVERIFY=%t status %s, want %s", negate, Nfsstat4Name(res.Status()), Nfsstat4Name(want))
		}
	}
}

// Used by both the local and real-TernFS suites. Every session uses a distinct
// owner, including two writers from the same client.
// exercisePrivateMutableWriters checks writer isolation across clients.
// Writers on one client share a staging version, as they share that client's
// page cache; another client's writer stays private until its CLOSE. A
// reader keeps the version it opened.
func exercisePrivateMutableWriters(t *testing.T, addr, name string, reverse bool) {
	t.Helper()
	conn := dial(t, addr)
	defer conn.Close()
	otherConn := dial(t, addr)
	defer otherConn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	otherID, otherConfirm := requestClientID(t, otherConn, &xid, "other-"+name, [8]byte{2})
	if status := confirmClientID(t, otherConn, &xid, otherID, otherConfirm); status != NFS4_OK {
		t.Fatal(Nfsstat4Name(status))
	}
	createdState, createdFH := openCreateFile(t, conn, &xid, clientID, name)
	writeFileAt(t, conn, &xid, createdFH, createdState, 0, []byte("original"))
	closeFile(t, conn, &xid, createdFH, createdState)
	open := func(c net.Conn, client uint64, owner string, access uint32) ([16]byte, []byte) {
		status, sid, fh, _ := openFileForOwner(t, c, &xid, client, owner, name, access, false, false)
		if status != NFS4_OK {
			t.Fatalf("OPEN %s: %s", owner, Nfsstat4Name(status))
		}
		return sid, fh
	}
	reader, baseFH := open(otherConn, otherID, "reader", OPEN4_SHARE_ACCESS_READ)
	first, firstFH := open(conn, clientID, "first", OPEN4_SHARE_ACCESS_BOTH)
	second, secondFH := open(otherConn, otherID, "second", OPEN4_SHARE_ACCESS_BOTH)
	if !bytes.Equal(firstFH, baseFH) || !bytes.Equal(secondFH, baseFH) {
		t.Fatal("write OPEN did not return the published filehandle")
	}
	baseAttrs := mutableAttrs(t, conn, &xid, lookupFH(t, conn, &xid, name))
	writeFileAt(t, conn, &xid, firstFH, first, 0, []byte("FIRST"))
	writeFileAt(t, otherConn, &xid, secondFH, second, 8, []byte("SECOND"))
	after := mutableAttrs(t, conn, &xid, firstFH)
	if binary.BigEndian.Uint64(after) == binary.BigEndian.Uint64(baseAttrs) {
		t.Fatal("same-size overwrite did not change CHANGE")
	}
	if !bytes.Equal(after, mutableAttrs(t, conn, &xid, firstFH)) {
		t.Fatal("GETATTR changed attributes without a mutation")
	}
	verifyMutableAttrs(t, conn, &xid, firstFH, after)
	// A third client sees the published attributes.
	viewer := dial(t, addr)
	defer viewer.Close()
	viewerID, viewerConfirm := requestClientID(t, viewer, &xid, "viewer-"+name, [8]byte{3})
	if status := confirmClientID(t, viewer, &xid, viewerID, viewerConfirm); status != NFS4_OK {
		t.Fatal(Nfsstat4Name(status))
	}
	publishedAttrs := mutableAttrs(t, viewer, &xid, baseFH)
	if !bytes.Equal(baseAttrs[:16], publishedAttrs[:16]) || !bytes.Equal(baseAttrs[28:], publishedAttrs[28:]) {
		t.Fatal("writer changed published attributes")
	}
	readWith := func(c net.Conn, fh []byte, sid [16]byte, want string) {
		t.Helper()
		res := sendCompound(t, c, xid, func(w *COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			pw.Resume(pw.StartObject().SetData(fh).Finish())
			w.Resume(pw.Finish())
			rw := w.AppendArgarray_Read()
			setStateid(rw.Stateid(), sid)
			rw.SetCount(1024)
		})
		xid++
		iter := expectOK(t, res)
		nextOp(t, &iter)
		got := nextOp(t, &iter).Value().AsREAD4resEntry().Value().AsREAD4resok().Data()
		if string(got) != want {
			t.Fatalf("READ = %q, want %q", got, want)
		}
	}
	check := func(c net.Conn, fh []byte, want string) {
		t.Helper()
		got, eof := readFileData(t, c, &xid, fh, 0, 1024)
		if string(got) != want || !eof {
			t.Fatalf("READ %x = (%q, %t), want (%q, true)", fh, got, eof, want)
		}
	}
	check(viewer, baseFH, "original")
	readWith(otherConn, baseFH, reader, "original")
	readWith(conn, firstFH, first, "FIRSTnal")
	readWith(otherConn, secondFH, second, "originalSECOND")
	// A second writer on the first client joins its staging.
	joined, _ := open(conn, clientID, "joined", OPEN4_SHARE_ACCESS_BOTH)
	readWith(conn, firstFH, joined, "FIRSTnal")
	closeFile(t, conn, &xid, firstFH, joined)

	firstConn, secondConn := conn, otherConn
	firstWant, secondWant := "FIRSTnal", "originalSECOND"
	if reverse {
		first, second = second, first
		firstConn, secondConn = secondConn, firstConn
		firstWant, secondWant = secondWant, firstWant
	}
	closeFile(t, firstConn, &xid, baseFH, first)
	check(viewer, lookupFH(t, viewer, &xid, name), firstWant)
	readWith(otherConn, baseFH, reader, "original")
	readWith(secondConn, baseFH, second, secondWant)
	closeFile(t, secondConn, &xid, baseFH, second)
	check(viewer, lookupFH(t, viewer, &xid, name), secondWant)
	// Each closing client sees its own publication through the handle.
	check(secondConn, baseFH, secondWant)
	readWith(otherConn, baseFH, reader, "original")
	closeFile(t, otherConn, &xid, baseFH, reader)
}

// Writers on one client share staging. After a restart any of the client's
// open owners reclaims it, and the last CLOSE publishes both owners' writes.
func TestRecoveredSharedWriterAcrossOpenOwners(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := NewLocalTernVFS(root)
	dir := t.TempDir()
	store, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	conn := dial(t, addr)
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	open := func(owner string) ([16]byte, []byte) {
		t.Helper()
		status, sid, fh, _ := openFileForOwner(t, conn, &xid, clientID, owner, "file", OPEN4_SHARE_ACCESS_BOTH, false, false)
		if status != NFS4_OK {
			t.Fatalf("OPEN %s: %s", owner, Nfsstat4Name(status))
		}
		return sid, fh
	}
	first, fh := open("")
	second, _ := open("second-owner")
	writeFileAt(t, conn, &xid, fh, first, 0, []byte("ONE"))
	writeFileAt(t, conn, &xid, fh, second, 3, []byte("TWO"))
	if len(store.Entries()) != 1 {
		t.Fatalf("one client's writers used %d staging files", len(store.Entries()))
	}
	conn.Close()
	cleanup()
	// Close descriptors without deleting durable sidecars, simulating restart.
	for _, entry := range store.files {
		entry.file.prepareRemove()
	}
	recovered, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err = NewServer(fs, recovered, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup = serveTestServer(t, srv)
	defer cleanup()
	conn = dial(t, addr)
	defer conn.Close()
	second, _ = open("second-owner")
	first, _ = open("")
	got, _ := readFileData(t, conn, &xid, fh, 0, 1024)
	if string(got) != "ONETWOal" {
		t.Fatalf("recovered %q, want %q", got, "ONETWOal")
	}
	closeFile(t, conn, &xid, fh, second)
	closeFile(t, conn, &xid, fh, first)
	got, _ = readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "file"), 0, 1024)
	if string(got) != "ONETWOal" || stagingTargetBusyForTest(recovered, fs.RootID(), "file") {
		t.Fatalf("last recovered CLOSE did not publish and release staging: %q", got)
	}
}

func TestPrivateMutableWriters(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			addr, cleanup := startTestServer(t, t.TempDir())
			defer cleanup()
			exercisePrivateMutableWriters(t, addr, "private.txt", reverse)
		})
	}
}

func TestStagingCheckpointRetry(t *testing.T) {
	for _, mutation := range []string{"overwrite", "extend", "truncate", "timestamps"} {
		t.Run(mutation, func(t *testing.T) {
			dir := t.TempDir()
			base := []byte("original")
			store, id, stage := createOverlayStage(t, dir, MakeInodeID(InodeTypeFile, 10), base)
			defer store.Remove(id)
			if err := stage.Write(0, []byte("O")); err != nil {
				t.Fatal(err)
			}
			if err := stage.Sync(); err != nil {
				t.Fatal(err)
			}
			want := "Original"
			var err error
			switch mutation {
			case "overwrite":
				err = stage.Write(0, []byte("UPDATED!"))
				want = "UPDATED!"
			case "extend":
				err = stage.Write(8, []byte("XYZ"))
				want = "OriginalXYZ"
			case "truncate":
				err = stage.SetSize(3)
				want = "Ori"
			case "timestamps":
				stamp := time.Unix(1600000000, 123)
				err = stage.SetTime(&stamp, &stamp)
			}
			if err != nil {
				t.Fatal(err)
			}
			wantAttrs := encodeAttrs(mutableAttrMask, id, stage.Stat())
			sf := stage.(*localStagingFile)
			metaPath := sf.metaPath
			sf.metaPath = filepath.Join(dir, "missing", "file.meta")
			if err := stage.Sync(); err == nil {
				t.Fatal("checkpoint unexpectedly succeeded")
			}
			sf.metaPath = metaPath
			if err := stage.Sync(); err != nil {
				t.Fatalf("checkpoint retry: %v", err)
			}
			recovered, err := NewLocalStagingStore(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer recovered.Remove(id)
			buf := make([]byte, len(want))
			rs := recovered.Get(id)
			if _, _, err := rs.Read(0, buf, sliceBaseReader(base, nil)); err != nil {
				t.Fatal(err)
			}
			if string(buf) != want {
				t.Fatalf("recovered %q, want %q", buf, want)
			}
			if !bytes.Equal(encodeAttrs(mutableAttrMask, id, rs.Stat()), wantAttrs) {
				t.Fatal("checkpoint did not preserve writer attributes")
			}
		})
	}
}

// lookupFH looks up a name in the root directory and returns the file handle.
func lookupFH(t *testing.T, conn net.Conn, xid *uint32, name string) []byte {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte(name)).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
		w.AppendArgarray_Getfh()
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	fh := append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	return fh
}

// readFileData reads a file by handle and returns the data.
func readFileData(t *testing.T, conn net.Conn, xid *uint32, fh []byte, offset uint64, count uint32) (data []byte, eof bool) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		pfW := w.AppendArgarray_Putfh()
		buf := pfW.StartObject().SetData(fh).Finish()
		pfW.Resume(buf)
		buf = pfW.Finish()
		w.Resume(buf)
		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(offset)
		rw.SetCount(count)
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	readRes := nextOp(t, &iter).Value().AsREAD4resEntry()
	if readRes.Disc() != NFS4_OK {
		t.Fatalf("READ status = %s", Nfsstat4Name(readRes.Disc()))
	}
	readOK := readRes.Value().AsREAD4resok()
	return readOK.Data(), readOK.Eof() != 0
}
