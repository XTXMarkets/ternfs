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

var mutableAttrMask = [2]uint32{
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
func exercisePrivateMutableWriters(t *testing.T, addr, name string, reverse bool) {
	t.Helper()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	createdState, createdFH := openCreateFile(t, conn, &xid, clientID, name)
	writeFileAt(t, conn, &xid, createdFH, createdState, 0, []byte("original"))
	closeFile(t, conn, &xid, createdFH, createdState)
	open := func(owner string, access uint32) ([16]byte, []byte) {
		status, sid, fh, flags := openFileForOwner(t, conn, &xid, clientID, owner, 1, name, access, false, false)
		if status != NFS4_OK {
			t.Fatalf("OPEN %s: %s", owner, Nfsstat4Name(status))
		}
		if flags&OPEN4_RESULT_CONFIRM != 0 {
			sid = confirmOpenState(t, conn, &xid, fh, 2, sid)
		}
		return sid, fh
	}
	reader, baseFH := open("reader", OPEN4_SHARE_ACCESS_READ)
	first, firstFH := open("first", OPEN4_SHARE_ACCESS_BOTH)
	second, secondFH := open("second", OPEN4_SHARE_ACCESS_BOTH)
	if bytes.Equal(firstFH, secondFH) || bytes.Equal(firstFH, baseFH) || bytes.Equal(secondFH, baseFH) {
		t.Fatal("writers and published reader must have distinct handles")
	}
	baseAttrs := mutableAttrs(t, conn, &xid, baseFH)
	firstAttrs := mutableAttrs(t, conn, &xid, firstFH)
	writeFileAt(t, conn, &xid, firstFH, first, 0, []byte("FIRST"))
	writeFileAt(t, conn, &xid, secondFH, second, 8, []byte("SECOND"))
	after := mutableAttrs(t, conn, &xid, firstFH)
	if binary.BigEndian.Uint64(after) == binary.BigEndian.Uint64(firstAttrs) {
		t.Fatal("same-size overwrite did not change CHANGE")
	}
	if !bytes.Equal(after, mutableAttrs(t, conn, &xid, firstFH)) {
		t.Fatal("GETATTR changed attributes without a mutation")
	}
	verifyMutableAttrs(t, conn, &xid, firstFH, after)
	verifyMutableAttrs(t, conn, &xid, secondFH, mutableAttrs(t, conn, &xid, secondFH))
	publishedAttrs := mutableAttrs(t, conn, &xid, baseFH)
	// Hydration can update the local backend's access time; data, size,
	// modification time and change token must remain published attributes.
	if !bytes.Equal(baseAttrs[:16], publishedAttrs[:16]) || !bytes.Equal(baseAttrs[28:], publishedAttrs[28:]) {
		t.Fatal("writer changed published attributes")
	}
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rw := w.AppendArgarray_Readdir()
		rw.SetDircount(4096)
		rw.SetMaxcount(8192)
		bw := rw.StartAttrRequest()
		bw.AppendData(mutableAttrMask[0])
		bw.AppendData(mutableAttrMask[1])
		rw.Resume(bw.Finish())
		w.Resume(rw.Finish())
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	dir := nextOp(t, &iter).Value().AsREADDIR4resEntry().Value().AsREADDIR4resok().Reply()
	found := false
	if dir.EntriesPresent() == TRUE {
		entry := dir.Entries().AsEntry4()
		for {
			if string(entry.Name().Data()) == name {
				found = true
				attrs := entry.Attrs().AttrVals().Data()
				if len(attrs) < 16 || !bytes.Equal(attrs[:16], publishedAttrs[:16]) {
					t.Fatal("READDIR exposes staged size or change token")
				}
			}
			if entry.NextentryPresent() != TRUE {
				break
			}
			entry = entry.Nextentry().AsEntry4()
		}
	}
	if !found {
		t.Fatal("published file missing from READDIR")
	}
	check := func(fh []byte, want string) {
		t.Helper()
		got, eof := readFileData(t, conn, &xid, fh, 0, 1024)
		if string(got) != want || !eof {
			t.Fatalf("READ %x = (%q, %t), want (%q, true)", fh, got, eof, want)
		}
	}
	check(baseFH, "original")
	check(lookupFH(t, conn, &xid, name), "original")
	check(firstFH, "FIRSTnal")
	check(secondFH, "originalSECOND")
	// A stateid for a published reader cannot be used on a private handle.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(firstFH).Finish())
		w.Resume(pw.Finish())
		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(binary.BigEndian.Uint32(reader[:4]))
		for i := range 12 {
			rw.Stateid().SetOther(i, reader[4+i])
		}
		rw.SetCount(1024)
	})
	xid++
	if res.Status() != NFS4ERR_BAD_STATEID {
		t.Fatalf("reader state on writer handle: %s", Nfsstat4Name(res.Status()))
	}
	firstWant, secondWant := "FIRSTnal", "originalSECOND"
	if reverse {
		first, second = second, first
		firstFH, secondFH = secondFH, firstFH
		firstWant, secondWant = secondWant, firstWant
	}
	closeFile(t, conn, &xid, firstFH, first)
	check(lookupFH(t, conn, &xid, name), firstWant)
	check(baseFH, "original")
	check(secondFH, secondWant)
	closeFile(t, conn, &xid, secondFH, second)
	check(lookupFH(t, conn, &xid, name), secondWant)
	check(firstFH, firstWant)
	check(baseFH, "original")
	closeFile(t, conn, &xid, baseFH, reader)
}

func TestRecoveredPrivateWritersUseOpenOwner(t *testing.T) {
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
		status, sid, fh, flags := openFileForOwner(t, conn, &xid, clientID, owner, 1, "file", OPEN4_SHARE_ACCESS_BOTH, false, false)
		if status != NFS4_OK {
			t.Fatalf("OPEN %s: %s", owner, Nfsstat4Name(status))
		}
		if flags&OPEN4_RESULT_CONFIRM != 0 {
			sid = confirmOpenState(t, conn, &xid, fh, 2, sid)
		}
		return sid, fh
	}
	first, firstFH := open("")
	second, secondFH := open("second-owner")
	writeFileAt(t, conn, &xid, firstFH, first, 0, []byte("ONE"))
	writeFileAt(t, conn, &xid, secondFH, second, 0, []byte("TWO"))
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
	// Recover in reverse order. Both share a client and target name.
	second, gotSecondFH := open("second-owner")
	first, gotFirstFH := open("")
	if !bytes.Equal(firstFH, gotFirstFH) || !bytes.Equal(secondFH, gotSecondFH) {
		t.Fatal("recovery rebound a writer to another session")
	}
	for _, test := range []struct {
		sid  [16]byte
		fh   []byte
		want string
	}{{second, secondFH, "TWOginal"}, {first, firstFH, "ONEginal"}} {
		got, _ := readFileData(t, conn, &xid, test.fh, 0, 1024)
		if string(got) != test.want {
			t.Fatalf("recovered %q, want %q", got, test.want)
		}
		closeFile(t, conn, &xid, test.fh, test.sid)
	}
	got, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "file"), 0, 1024)
	if string(got) != "ONEginal" || stagingTargetBusyForTest(recovered, fs.RootID(), "file") {
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
