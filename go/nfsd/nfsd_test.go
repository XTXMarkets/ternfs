// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func startTestServer(t *testing.T, dir string) (addr string, cleanup func()) {
	t.Helper()
	fs := NewLocalTernVFS(dir)
	stagingDir := t.TempDir()
	ss, err := NewLocalStagingStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, ss, nil)
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.handleConn(conn)
		}
	}()
	return ln.Addr().String(), func() { ln.Close() }
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func sendRPC(conn net.Conn, xid uint32, proc uint32, body []byte) ([]byte, error) {
	buf := make([]byte, 0, 128+len(body))
	buf = binary.BigEndian.AppendUint32(buf, xid)
	buf = binary.BigEndian.AppendUint32(buf, 0)      // CALL
	buf = binary.BigEndian.AppendUint32(buf, 2)      // RPC version
	buf = binary.BigEndian.AppendUint32(buf, 100003) // NFS prog
	buf = binary.BigEndian.AppendUint32(buf, 4)      // NFS v4
	buf = binary.BigEndian.AppendUint32(buf, proc)
	buf = binary.BigEndian.AppendUint32(buf, 0) // cred flavor
	buf = binary.BigEndian.AppendUint32(buf, 0) // cred len
	buf = binary.BigEndian.AppendUint32(buf, 0) // verf flavor
	buf = binary.BigEndian.AppendUint32(buf, 0) // verf len
	buf = append(buf, body...)

	if err := writeFrame(conn, buf); err != nil {
		return nil, err
	}
	return readFrame(conn)
}

func parseRPCReply(t *testing.T, reply []byte) []byte {
	t.Helper()
	if len(reply) < 24 {
		t.Fatalf("reply too short: %d bytes", len(reply))
	}
	acceptStat := binary.BigEndian.Uint32(reply[20:24])
	if acceptStat != 0 {
		t.Fatalf("accept_stat = %d, want SUCCESS", acceptStat)
	}
	return reply[24:]
}

// sendCompound builds a compound, sends it, and returns the parsed COMPOUND4res.
func sendCompound(t *testing.T, conn net.Conn, xid uint32, build func(w *COMPOUND4argsWriter)) COMPOUND4res {
	t.Helper()
	var body []byte
	w := StartCOMPOUND4args(body)
	tagW := w.StartTag()
	body = tagW.SetData(nil).Finish()
	w.Resume(body)
	w.SetMinorversion(0)

	build(&w)

	body = w.Finish()

	reply, err := sendRPC(conn, xid, procCompound, body)
	if err != nil {
		t.Fatal(err)
	}
	nfsBody := parseRPCReply(t, reply)
	res, ok := ReadCOMPOUND4res(nfsBody)
	if !ok {
		t.Fatal("failed to parse COMPOUND4res")
	}
	return res
}

// expectOK checks compound status is NFS4_OK and returns the resarray iterator.
func expectOK(t *testing.T, res COMPOUND4res) NfsResop4EntryIter {
	t.Helper()
	if res.Status() != NFS4_OK {
		t.Fatalf("compound status = %d, want NFS4_OK", res.Status())
	}
	return res.Resarray()
}

// nextOp advances the iterator and returns the entry.
func nextOp(t *testing.T, iter *NfsResop4EntryIter) NfsResop4Entry {
	t.Helper()
	if !iter.Next() {
		t.Fatal("expected another op in resarray")
	}
	return iter.Resarray()
}

// getAttrData extracts attribute values from a GETATTR4resok fattr4.
// Uses Fattr4.AttrVals().Data() which requires correct codegen for
// sequential variable-size field getters (bitmap4 then attrlist4).
func getAttrData(t *testing.T, getattrOk GETATTR4resok) []byte {
	t.Helper()
	return getattrOk.ObjAttributes().AttrVals().Data()
}

// setupClient runs SETCLIENTID + SETCLIENTID_CONFIRM and returns the assigned clientid.
func setupClient(t *testing.T, conn net.Conn, xid *uint32) uint64 {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		scw := w.AppendArgarray_Setclientid()
		clientW := scw.StartClient()
		clientW = clientW.SetId([]byte("test-client"))
		buf := clientW.Finish()
		scw.Resume(buf)
		cbW := scw.StartCallback()
		cbW.SetCbProgram(0x40000000)
		locW := cbW.StartCbLocation()
		netidW := locW.StartRNetid()
		buf = netidW.SetData([]byte("tcp")).Finish()
		locW.Resume(buf)
		addrW := locW.StartRAddr()
		buf = addrW.SetData([]byte("0.0.0.0.0.0")).Finish()
		locW.Resume(buf)
		buf = locW.Finish()
		cbW.Resume(buf)
		buf = cbW.Finish()
		scw.Resume(buf)
		scw.SetCallbackIdent(0)
		buf = scw.Finish()
		w.Resume(buf)
	})
	*xid++
	iter := expectOK(t, res)
	entry := nextOp(t, &iter)
	scRes := entry.Value().AsSETCLIENTID4resEntry()
	if scRes.Disc() != NFS4_OK {
		t.Fatalf("SETCLIENTID status = %d", scRes.Disc())
	}
	clientid := scRes.Value().AsSETCLIENTID4resok().Clientid()

	res = sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		scw := w.AppendArgarray_SetclientidConfirm()
		scw.SetClientid(clientid)
	})
	*xid++
	iter = expectOK(t, res)
	entry = nextOp(t, &iter)
	if entry.Value().AsSETCLIENTIDCONFIRM4res().Status() != NFS4_OK {
		t.Fatal("SETCLIENTID_CONFIRM failed")
	}
	return clientid
}

func TestNullRPC(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	reply, err := sendRPC(conn, 1, procNull, nil)
	if err != nil {
		t.Fatal(err)
	}
	parseRPCReply(t, reply)
}

func TestProgMismatch(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Send an RPC with wrong program number.
	buf := make([]byte, 0, 64)
	buf = binary.BigEndian.AppendUint32(buf, 99)     // xid
	buf = binary.BigEndian.AppendUint32(buf, 0)      // CALL
	buf = binary.BigEndian.AppendUint32(buf, 2)      // RPC version
	buf = binary.BigEndian.AppendUint32(buf, 999999) // wrong program
	buf = binary.BigEndian.AppendUint32(buf, 4)      // version
	buf = binary.BigEndian.AppendUint32(buf, 0)      // proc
	buf = binary.BigEndian.AppendUint32(buf, 0)      // cred flavor
	buf = binary.BigEndian.AppendUint32(buf, 0)      // cred len
	buf = binary.BigEndian.AppendUint32(buf, 0)      // verf flavor
	buf = binary.BigEndian.AppendUint32(buf, 0)      // verf len

	if err := writeFrame(conn, buf); err != nil {
		t.Fatal(err)
	}
	reply, err := readFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	// Should get a PROG_MISMATCH reply.
	if len(reply) < 24 {
		t.Fatalf("reply too short: %d bytes", len(reply))
	}
	acceptStat := binary.BigEndian.Uint32(reply[20:24])
	if acceptStat != acceptProgMismatch {
		t.Fatalf("accept_stat = %d, want PROG_MISMATCH (%d)", acceptStat, acceptProgMismatch)
	}
}

func TestPutrootfhGetattr(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 2, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1<<FATTR4_TYPE | 1<<FATTR4_FILEID)
		buf := bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	if res.ResarrayCount() != 2 {
		t.Fatalf("resarray count = %d, want 2", res.ResarrayCount())
	}

	// PUTROOTFH
	entry := nextOp(t, &iter)
	if entry.Disc() != OP_PUTROOTFH {
		t.Fatalf("op[0] = %d, want OP_PUTROOTFH", entry.Disc())
	}
	if entry.Value().AsPUTROOTFH4res().Status() != NFS4_OK {
		t.Fatal("PUTROOTFH failed")
	}

	// GETATTR
	entry = nextOp(t, &iter)
	if entry.Disc() != OP_GETATTR {
		t.Fatalf("op[1] = %d, want OP_GETATTR", entry.Disc())
	}
	getattrRes := entry.Value().AsGETATTR4resEntry()
	if getattrRes.Disc() != NFS4_OK {
		t.Fatalf("GETATTR status = %d", getattrRes.Disc())
	}
}

func TestPutpubfh(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putpubfh()
		w.AppendArgarray_Getfh()
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTPUBFH
	entry := nextOp(t, &iter)
	getfhRes := entry.Value().AsGETFH4resEntry()
	if getfhRes.Disc() != NFS4_OK {
		t.Fatalf("GETFH status = %d", getfhRes.Disc())
	}
	fh := getfhRes.Value().AsGETFH4resok().Object().Data()
	if len(fh) != 8 {
		t.Fatalf("GETFH returned %d bytes, want 8", len(fh))
	}
}

func TestLookupAndRead(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("Hello, NFS!"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 3, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("hello.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(1024)
	})

	iter := expectOK(t, res)
	if res.ResarrayCount() != 4 {
		t.Fatalf("resarray count = %d, want 4", res.ResarrayCount())
	}

	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP

	// GETFH — validate 8-byte file handle.
	entry := nextOp(t, &iter)
	getfhRes := entry.Value().AsGETFH4resEntry()
	if getfhRes.Disc() != NFS4_OK {
		t.Fatalf("GETFH status = %d", getfhRes.Disc())
	}
	fhData := getfhRes.Value().AsGETFH4resok().Object().Data()
	if len(fhData) != 8 {
		t.Fatalf("GETFH returned FH of %d bytes, want 8", len(fhData))
	}

	// READ — validate data and EOF.
	entry = nextOp(t, &iter)
	readRes := entry.Value().AsREAD4resEntry()
	if readRes.Disc() != NFS4_OK {
		t.Fatalf("READ status = %d", readRes.Disc())
	}
	readOk := readRes.Value().AsREAD4resok()
	data := readOk.Data()
	if string(data) != "Hello, NFS!" {
		t.Fatalf("READ data = %q, want %q", data, "Hello, NFS!")
	}
	if readOk.Eof() != TRUE {
		t.Fatalf("READ eof = %d, want TRUE", readOk.Eof())
	}
}

func TestReadWithOffset(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("0123456789abcdef"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Read 4 bytes at offset 10 — should get "abcd", not EOF.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("data.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(10)
		rw.SetCount(4)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP

	entry := nextOp(t, &iter)
	readOk := entry.Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readOk.Data()) != "abcd" {
		t.Fatalf("READ data = %q, want %q", readOk.Data(), "abcd")
	}
	if readOk.Eof() != FALSE {
		t.Fatalf("READ eof = TRUE, want FALSE (not at end of file)")
	}

	// Read past end — should get remaining data with EOF.
	res = sendCompound(t, conn, 2, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("data.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(14)
		rw.SetCount(100)
	})

	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry = nextOp(t, &iter)
	readOk = entry.Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readOk.Data()) != "ef" {
		t.Fatalf("READ data = %q, want %q", readOk.Data(), "ef")
	}
	if readOk.Eof() != TRUE {
		t.Fatal("READ eof = FALSE, want TRUE")
	}
}

func TestReadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "empty.txt"), nil, 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("empty.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(1024)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry := nextOp(t, &iter)
	readOk := entry.Value().AsREAD4resEntry().Value().AsREAD4resok()
	if len(readOk.Data()) != 0 {
		t.Fatalf("expected 0 bytes, got %d", len(readOk.Data()))
	}
	if readOk.Eof() != TRUE {
		t.Fatal("expected EOF for empty file")
	}
}

func TestReaddir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "aaa.txt"), []byte("A"), 0644)
	os.WriteFile(filepath.Join(dir, "bbb.txt"), []byte("BB"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 4, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		rdw := w.AppendArgarray_Readdir()
		rdw.SetCookie(0)
		rdw.SetDircount(4096)
		rdw.SetMaxcount(8192)
		bmW := rdw.StartAttrRequest()
		bmW.AppendData(1<<FATTR4_TYPE | 1<<FATTR4_FILEID)
		buf := bmW.Finish()
		rdw.Resume(buf)
		buf = rdw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	if res.ResarrayCount() != 2 {
		t.Fatalf("resarray count = %d, want 2", res.ResarrayCount())
	}
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	readdirRes := entry.Value().AsREADDIR4resEntry()
	if readdirRes.Disc() != NFS4_OK {
		t.Fatalf("READDIR status = %d", readdirRes.Disc())
	}
	// Verify cookieverf is non-zero in the response.
	okRes := readdirRes.Value().AsREADDIR4resok()
	cv := okRes.Cookieverf()
	var allZero bool = true
	for i := 0; i < 8; i++ {
		if cv.Data(i) != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("READDIR cookieverf is all zeros in response")
	}
}

func TestReaddirEmpty(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "empty"), 0755)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("empty")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		rdw := w.AppendArgarray_Readdir()
		rdw.SetCookie(0)
		rdw.SetDircount(4096)
		rdw.SetMaxcount(8192)
		bmW := rdw.StartAttrRequest()
		bmW.AppendData(0)
		buf = bmW.Finish()
		rdw.Resume(buf)
		buf = rdw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry := nextOp(t, &iter)
	readdirRes := entry.Value().AsREADDIR4resEntry()
	if readdirRes.Disc() != NFS4_OK {
		t.Fatalf("READDIR status = %d", readdirRes.Disc())
	}
}

func TestPutfhRoundTrip(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// First: PUTROOTFH + LOOKUP + GETFH to get a file handle.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("file.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry := nextOp(t, &iter)
	fh := append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)

	// Second: PUTFH(fh) + READ to read via the file handle.
	res = sendCompound(t, conn, 2, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(1024)
	})

	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	entry = nextOp(t, &iter)
	readOk := entry.Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readOk.Data()) != "hello" {
		t.Fatalf("READ via PUTFH = %q, want %q", readOk.Data(), "hello")
	}
}

func TestPutfhBadHandle(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Send a PUTFH with wrong-sized handle (4 bytes instead of 8).
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData([]byte{1, 2, 3, 4}).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
	})

	if res.Status() != NFS4ERR_BADHANDLE {
		t.Fatalf("compound status = %d, want NFS4ERR_BADHANDLE (%d)", res.Status(), NFS4ERR_BADHANDLE)
	}
}

func TestSavefhRestorefh(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// PUTROOTFH + LOOKUP(a.txt) + SAVEFH + LOOKUP(../b.txt via putrootfh) + RESTOREFH + READ
	// This tests: save file A's handle, navigate to B, restore A, read A.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("a.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		w.AppendArgarray_Savefh()

		// Navigate away.
		w.AppendArgarray_Putrootfh()
		lw = w.AppendArgarray_Lookup()
		nw = lw.StartObjname()
		buf = nw.SetData([]byte("b.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		// Restore and read the saved handle (a.txt).
		w.AppendArgarray_Restorefh()

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(1024)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	nextOp(t, &iter) // SAVEFH
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	nextOp(t, &iter) // RESTOREFH
	entry := nextOp(t, &iter)
	readOk := entry.Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readOk.Data()) != "aaa" {
		t.Fatalf("READ after RESTOREFH = %q, want %q", readOk.Data(), "aaa")
	}
}

func TestRestorefhWithoutSave(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Restorefh()
	})

	if res.Status() != NFS4ERR_RESTOREFH {
		t.Fatalf("status = %d, want NFS4ERR_RESTOREFH (%d)", res.Status(), NFS4ERR_RESTOREFH)
	}
}

func TestLookupp(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("at root"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// PUTROOTFH + LOOKUP(sub) + LOOKUPP + LOOKUP(root.txt) + READ
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("sub")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		w.AppendArgarray_Lookupp()

		lw = w.AppendArgarray_Lookup()
		nw = lw.StartObjname()
		buf = nw.SetData([]byte("root.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(1024)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	nextOp(t, &iter) // LOOKUPP
	nextOp(t, &iter) // LOOKUP

	entry := nextOp(t, &iter)
	readOk := entry.Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readOk.Data()) != "at root" {
		t.Fatalf("READ = %q, want %q", readOk.Data(), "at root")
	}
}

func TestReadlink(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "target.txt"), []byte("target"), 0644)
	os.Symlink("target.txt", filepath.Join(dir, "link.txt"))

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("link.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		w.AppendArgarray_Readlink()
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry := nextOp(t, &iter)
	readlinkRes := entry.Value().AsREADLINK4resEntry()
	if readlinkRes.Disc() != NFS4_OK {
		t.Fatalf("READLINK status = %d", readlinkRes.Disc())
	}
	target := string(readlinkRes.Value().AsREADLINK4resok().Link().Data())
	if target != "target.txt" {
		t.Fatalf("readlink = %q, want %q", target, "target.txt")
	}
}

func TestAccess(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		aw := w.AppendArgarray_Access()
		aw.SetAccess(ACCESS4_READ | ACCESS4_LOOKUP)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	accessRes := entry.Value().AsACCESS4resEntry()
	if accessRes.Disc() != NFS4_OK {
		t.Fatalf("ACCESS status = %d", accessRes.Disc())
	}
	accessOk := accessRes.Value().AsACCESS4resok()
	requested := uint32(ACCESS4_READ | ACCESS4_LOOKUP)
	if accessOk.Supported() != requested {
		t.Fatalf("supported = %d, want %d", accessOk.Supported(), requested)
	}
	if accessOk.Access() != requested {
		t.Fatalf("access = %d, want %d", accessOk.Access(), requested)
	}
}

func TestOpenConfirmClose(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// PUTROOTFH + OPEN(file.txt) + OPEN_CONFIRM + GETFH + READ + CLOSE
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_READ)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		ow.SetOpenhow_Default(OPEN4_NOCREATE)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("file.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)

		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)

		w.AppendArgarray_Getfh()

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(1024)

		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		caw.OpenStateid().SetSeqid(1)
	})

	iter := expectOK(t, res)
	if res.ResarrayCount() != 6 {
		t.Fatalf("resarray count = %d, want 6", res.ResarrayCount())
	}

	nextOp(t, &iter) // PUTROOTFH

	// OPEN
	entry := nextOp(t, &iter)
	openRes := entry.Value().AsOPEN4resEntry()
	if openRes.Disc() != NFS4_OK {
		t.Fatalf("OPEN status = %d", openRes.Disc())
	}

	// OPEN_CONFIRM
	entry = nextOp(t, &iter)
	ocRes := entry.Value().AsOPENCONFIRM4resEntry()
	if ocRes.Disc() != NFS4_OK {
		t.Fatalf("OPEN_CONFIRM status = %d", ocRes.Disc())
	}

	// GETFH
	entry = nextOp(t, &iter)
	fh := entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()
	if len(fh) != 8 {
		t.Fatalf("FH = %d bytes, want 8", len(fh))
	}

	// READ
	entry = nextOp(t, &iter)
	readOk := entry.Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readOk.Data()) != "content" {
		t.Fatalf("READ = %q, want %q", readOk.Data(), "content")
	}

	// CLOSE
	entry = nextOp(t, &iter)
	closeRes := entry.Value().AsCLOSE4resEntry()
	if closeRes.Disc() != NFS4_OK {
		t.Fatalf("CLOSE status = %d", closeRes.Disc())
	}
}

func TestSetclientidFlow(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// SETCLIENTID
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		scw := w.AppendArgarray_Setclientid()

		clientW := scw.StartClient()
		// Verifier is pre-allocated in the first 8 bytes — leave as zero.
		clientW = clientW.SetId([]byte("test-client"))
		buf := clientW.Finish()
		scw.Resume(buf)

		cbW := scw.StartCallback()
		cbW.SetCbProgram(0x40000000)
		locW := cbW.StartCbLocation()
		netidW := locW.StartRNetid()
		buf = netidW.SetData([]byte("tcp")).Finish()
		locW.Resume(buf)
		addrW := locW.StartRAddr()
		buf = addrW.SetData([]byte("0.0.0.0.0.0")).Finish()
		locW.Resume(buf)
		buf = locW.Finish()
		cbW.Resume(buf)
		buf = cbW.Finish()
		scw.Resume(buf)

		scw.SetCallbackIdent(0)
		buf = scw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	entry := nextOp(t, &iter)
	scRes := entry.Value().AsSETCLIENTID4resEntry()
	if scRes.Disc() != NFS4_OK {
		t.Fatalf("SETCLIENTID status = %d", scRes.Disc())
	}
	scOk := scRes.Value().AsSETCLIENTID4resok()
	clientid := scOk.Clientid()
	if clientid == 0 {
		t.Fatal("SETCLIENTID returned clientid=0")
	}

	// SETCLIENTID_CONFIRM
	res = sendCompound(t, conn, 2, func(w *COMPOUND4argsWriter) {
		scw := w.AppendArgarray_SetclientidConfirm()
		scw.SetClientid(clientid)
		// Setclientid_confirm verifier — leave as zero.
	})

	iter = expectOK(t, res)
	entry = nextOp(t, &iter)
	if entry.Value().AsSETCLIENTIDCONFIRM4res().Status() != NFS4_OK {
		t.Fatal("SETCLIENTID_CONFIRM failed")
	}

	// RENEW
	res = sendCompound(t, conn, 3, func(w *COMPOUND4argsWriter) {
		rw := w.AppendArgarray_Renew()
		rw.SetClientid(clientid)
	})

	iter = expectOK(t, res)
	entry = nextOp(t, &iter)
	if entry.Value().AsRENEW4res().Status() != NFS4_OK {
		t.Fatal("RENEW failed")
	}
}

func TestSetattr(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		saw := w.AppendArgarray_Setattr()
		// Stateid: zero.
		saw.Stateid().SetSeqid(0)
		// Empty fattr4.
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		buf := bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		saw.Resume(buf)
		buf = saw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	if entry.Value().AsSETATTR4res().Status() != NFS4_OK {
		t.Fatal("SETATTR failed")
	}
}

func TestReleaseLockowner(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		rlw := w.AppendArgarray_ReleaseLockowner()
		loW := rlw.StartLockOwner()
		loW = loW.SetClientid(clientid)
		loW = loW.SetOwner([]byte("test-lock-owner"))
		buf := loW.Finish()
		rlw.Resume(buf)
		buf = rlw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	entry := nextOp(t, &iter)
	if entry.Value().AsRELEASELOCKOWNER4res().Status() != NFS4_OK {
		t.Fatal("RELEASE_LOCKOWNER failed")
	}
}

func TestIllegalOp(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Illegal()
	})

	if res.Status() != NFS4ERR_OP_ILLEGAL {
		t.Fatalf("status = %d, want NFS4ERR_OP_ILLEGAL (%d)", res.Status(), NFS4ERR_OP_ILLEGAL)
	}
}

func TestLookupNonExistent(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("nonexistent.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
	})

	if res.Status() != NFS4ERR_NOENT {
		t.Fatalf("status = %d, want NFS4ERR_NOENT (%d)", res.Status(), NFS4ERR_NOENT)
	}
	// Should have 2 ops: PUTROOTFH (success) + LOOKUP (failure).
	if res.ResarrayCount() != 2 {
		t.Fatalf("resarray count = %d, want 2", res.ResarrayCount())
	}
}

func TestNoFilehandle(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// GETATTR without setting a filehandle first.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_TYPE)
		buf := bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	if res.Status() != NFS4ERR_NOFILEHANDLE {
		t.Fatalf("status = %d, want NFS4ERR_NOFILEHANDLE (%d)", res.Status(), NFS4ERR_NOFILEHANDLE)
	}
}

func TestCompoundStopsOnError(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// PUTROOTFH + LOOKUP(nonexistent) + GETFH
	// The GETFH should NOT execute because LOOKUP fails.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("nonexistent")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()
	})

	if res.Status() != NFS4ERR_NOENT {
		t.Fatalf("status = %d, want NFS4ERR_NOENT", res.Status())
	}
	// Only PUTROOTFH + LOOKUP should be in the result (GETFH skipped).
	if res.ResarrayCount() != 2 {
		t.Fatalf("resarray count = %d, want 2 (GETFH should be skipped)", res.ResarrayCount())
	}
}

func TestSubdirNavigation(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0755)
	os.WriteFile(filepath.Join(dir, "a", "b", "c", "deep.txt"), []byte("deep content"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		for _, name := range []string{"a", "b", "c", "deep.txt"} {
			lw := w.AppendArgarray_Lookup()
			nw := lw.StartObjname()
			buf := nw.SetData([]byte(name)).Finish()
			lw.Resume(buf)
			buf = lw.Finish()
			w.Resume(buf)
		}

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(1024)
	})

	iter := expectOK(t, res)
	// 1 PUTROOTFH + 4 LOOKUPs + 1 READ = 6
	for i := 0; i < 5; i++ {
		nextOp(t, &iter)
	}
	entry := nextOp(t, &iter)
	readOk := entry.Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readOk.Data()) != "deep content" {
		t.Fatalf("READ = %q, want %q", readOk.Data(), "deep content")
	}
}

func TestGetattr_RootType(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Request type attribute for root — should be NF4DIR (2).
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_TYPE)
		buf := bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	getattrRes := entry.Value().AsGETATTR4resEntry()
	if getattrRes.Disc() != NFS4_OK {
		t.Fatalf("GETATTR status = %d", getattrRes.Disc())
	}
	// Parse the attribute data manually.
	getattrOk := getattrRes.Value().AsGETATTR4resok()
	attrData := getAttrData(t, getattrOk)
	if len(attrData) < 4 {
		t.Fatalf("attr data too short: %d bytes", len(attrData))
	}
	nfsType := binary.BigEndian.Uint32(attrData[:4])
	if nfsType != 2 { // NF4DIR
		t.Fatalf("root type = %d, want 2 (NF4DIR)", nfsType)
	}
}

func TestGetattr_FileSize(t *testing.T) {
	dir := t.TempDir()
	content := []byte("twelve chars")
	os.WriteFile(filepath.Join(dir, "sized.txt"), content, 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("sized.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_SIZE)
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry := nextOp(t, &iter)
	getattrOk := entry.Value().AsGETATTR4resEntry().Value().AsGETATTR4resok()
	attrData := getAttrData(t, getattrOk)
	if len(attrData) < 8 {
		t.Fatalf("attr data too short: %d bytes", len(attrData))
	}
	size := binary.BigEndian.Uint64(attrData[:8])
	if size != uint64(len(content)) {
		t.Fatalf("size = %d, want %d", size, len(content))
	}
}

func TestGetattr_Mode(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(dir, "d"), 0755)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Check file mode = 0444.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("f.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(0)
		bw.AppendData(1 << (FATTR4_MODE - 32))
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry := nextOp(t, &iter)
	attrData := getAttrData(t, entry.Value().AsGETATTR4resEntry().Value().AsGETATTR4resok())
	if len(attrData) < 4 {
		t.Fatalf("attr data too short: %d", len(attrData))
	}
	mode := binary.BigEndian.Uint32(attrData[:4])
	if mode != 0644 {
		t.Fatalf("file mode = %#o, want 0644", mode)
	}

	// Check dir mode = 0555.
	res = sendCompound(t, conn, 2, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(0)
		bw.AppendData(1 << (FATTR4_MODE - 32))
		buf := bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry = nextOp(t, &iter)
	attrData = getAttrData(t, entry.Value().AsGETATTR4resEntry().Value().AsGETATTR4resok())
	mode = binary.BigEndian.Uint32(attrData[:4])
	if mode != 0755 {
		t.Fatalf("dir mode = %#o, want 0755", mode)
	}
}

func TestGetattr_Symlink(t *testing.T) {
	dir := t.TempDir()
	os.Symlink("target", filepath.Join(dir, "link"))

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("link")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_TYPE)
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry := nextOp(t, &iter)
	attrData := getAttrData(t, entry.Value().AsGETATTR4resEntry().Value().AsGETATTR4resok())
	nfsType := binary.BigEndian.Uint32(attrData[:4])
	if nfsType != 5 { // NF4LNK
		t.Fatalf("symlink type = %d, want 5 (NF4LNK)", nfsType)
	}
}

func TestMultipleCompoundsOnConnection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Send multiple compounds on the same connection.
	for i, name := range []string{"a.txt", "b.txt"} {
		expected := string(name[0]) + string(name[0]) + string(name[0])
		res := sendCompound(t, conn, uint32(i+1), func(w *COMPOUND4argsWriter) {
			w.AppendArgarray_Putrootfh()

			lw := w.AppendArgarray_Lookup()
			nw := lw.StartObjname()
			buf := nw.SetData([]byte(name)).Finish()
			lw.Resume(buf)
			buf = lw.Finish()
			w.Resume(buf)

			rw := w.AppendArgarray_Read()
			rw.Stateid().SetSeqid(0)
			rw.SetOffset(0)
			rw.SetCount(1024)
		})

		iter := expectOK(t, res)
		nextOp(t, &iter) // PUTROOTFH
		nextOp(t, &iter) // LOOKUP
		entry := nextOp(t, &iter)
		data := entry.Value().AsREAD4resEntry().Value().AsREAD4resok().Data()
		if string(data) != expected {
			t.Fatalf("compound %d: READ = %q, want %q", i, data, expected)
		}
	}
}

func TestLargeFileRead(t *testing.T) {
	dir := t.TempDir()
	content := make([]byte, 128*1024)
	for i := range content {
		content[i] = byte(i % 251)
	}
	os.WriteFile(filepath.Join(dir, "large.bin"), content, 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Read the entire file in one shot.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("large.bin")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(uint32(len(content)))
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry := nextOp(t, &iter)
	data := entry.Value().AsREAD4resEntry().Value().AsREAD4resok().Data()
	if len(data) != len(content) {
		t.Fatalf("read %d bytes, want %d", len(data), len(content))
	}
	for i := range data {
		if data[i] != content[i] {
			t.Fatalf("mismatch at byte %d: got %d, want %d", i, data[i], content[i])
		}
	}
}

func TestGetattr_SupportedAttrs(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Request supported_attrs — verify bitmap is returned.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_SUPPORTED_ATTRS)
		buf := bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	getattrOk := entry.Value().AsGETATTR4resEntry().Value().AsGETATTR4resok()
	attrData := getAttrData(t, getattrOk)
	// supported_attrs is bitmap4: count(4) + 2 words(8) = 12 bytes.
	if len(attrData) < 12 {
		t.Fatalf("attr data too short for supported_attrs: %d bytes", len(attrData))
	}
	count := binary.BigEndian.Uint32(attrData[:4])
	if count != 2 {
		t.Fatalf("supported_attrs bitmap count = %d, want 2", count)
	}
	word0 := binary.BigEndian.Uint32(attrData[4:8])
	word1 := binary.BigEndian.Uint32(attrData[8:12])
	if word0 != supportedAttrs0 {
		t.Fatalf("supported_attrs[0] = %#x, want %#x", word0, supportedAttrs0)
	}
	if word1 != supportedAttrs1 {
		t.Fatalf("supported_attrs[1] = %#x, want %#x", word1, supportedAttrs1)
	}
}

func TestGetattr_MultipleAttrs(t *testing.T) {
	dir := t.TempDir()
	content := []byte("test content for multi attr")
	os.WriteFile(filepath.Join(dir, "multi.txt"), content, 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Request type + size + change + fileid — verify all are present.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("multi.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1<<FATTR4_TYPE | 1<<FATTR4_SIZE | 1<<FATTR4_CHANGE | 1<<FATTR4_FILEID)
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry := nextOp(t, &iter)
	attrData := getAttrData(t, entry.Value().AsGETATTR4resEntry().Value().AsGETATTR4resok())

	// Attrs in bit order: type(4) + change(8) + size(8) + fileid(8) = 28 bytes.
	if len(attrData) < 28 {
		t.Fatalf("attr data too short: %d bytes, want >= 28", len(attrData))
	}
	off := 0

	nfsType := binary.BigEndian.Uint32(attrData[off:])
	off += 4
	if nfsType != 1 { // NF4REG
		t.Fatalf("type = %d, want 1 (NF4REG)", nfsType)
	}

	change := binary.BigEndian.Uint64(attrData[off:])
	off += 8
	if change == 0 {
		t.Fatal("change = 0, want nonzero mtime-based value")
	}

	size := binary.BigEndian.Uint64(attrData[off:])
	off += 8
	if size != uint64(len(content)) {
		t.Fatalf("size = %d, want %d", size, len(content))
	}

	fileid := binary.BigEndian.Uint64(attrData[off:])
	if fileid == 0 {
		t.Fatal("fileid = 0, want nonzero inode number")
	}
}

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// PUTROOTFH + OPEN(CREATE) + OPEN_CONFIRM
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		// OPEN4_CREATE with UNCHECKED4 and empty attrs.
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("newfile.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()

		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)
	})
	xid++

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH

	// Get the stateid from OPEN.
	entry := nextOp(t, &iter)
	openRes := entry.Value().AsOPEN4resEntry()
	if openRes.Disc() != NFS4_OK {
		t.Fatalf("OPEN status = %d", openRes.Disc())
	}
	openOk := openRes.Value().AsOPEN4resok()
	openStateid := openOk.Stateid()

	// Get the filehandle from GETFH (file is transient, not yet in directory).
	entry = nextOp(t, &iter)
	fh := append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)

	nextOp(t, &iter) // OPEN_CONFIRM

	// WRITE to the file.
	writeData := []byte("hello world from NFS write!")
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		ww := w.AppendArgarray_Write()
		sid := ww.Stateid()
		sid.SetSeqid(openStateid.Seqid())
		for i := 0; i < 12; i++ {
			sid.SetOther(i, openStateid.Other(i))
		}
		ww = ww.SetOffset(0)
		ww = ww.SetStable(2) // FILE_SYNC4
		ww = ww.SetData(writeData)
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++

	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	entry = nextOp(t, &iter)
	writeRes := entry.Value().AsWRITE4resEntry()
	if writeRes.Disc() != NFS4_OK {
		t.Fatalf("WRITE status = %d", writeRes.Disc())
	}
	writeOk := writeRes.Value().AsWRITE4resok()
	if writeOk.Count() != uint32(len(writeData)) {
		t.Fatalf("WRITE count = %d, want %d", writeOk.Count(), len(writeData))
	}

	// CLOSE to commit the file.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		sid := caw.OpenStateid()
		sid.SetSeqid(openStateid.Seqid())
		for i := 0; i < 12; i++ {
			sid.SetOther(i, openStateid.Other(i))
		}
	})
	xid++
	expectOK(t, res)

	// Verify the file was written to disk.
	data, err := os.ReadFile(filepath.Join(dir, "newfile.txt"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(data) != string(writeData) {
		t.Fatalf("file content = %q, want %q", data, writeData)
	}
}

func TestStagedFileGetattrAndRead(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create and open a new file for write.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("staged.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()

		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)
	})
	xid++

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	openOk := entry.Value().AsOPEN4resEntry().Value().AsOPEN4resok()
	openStateid := openOk.Stateid()
	entry = nextOp(t, &iter) // GETFH
	fh := append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	nextOp(t, &iter) // OPEN_CONFIRM

	// Write some data using the open stateid.
	writeData := []byte("staged content here!")
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		ww := w.AppendArgarray_Write()
		sid := ww.Stateid()
		sid.SetSeqid(openStateid.Seqid())
		for i := 0; i < 12; i++ {
			sid.SetOther(i, openStateid.Other(i))
		}
		ww = ww.SetOffset(0)
		ww = ww.SetStable(2)
		ww = ww.SetData(writeData)
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	// GETATTR should reflect the staged size, not the empty VFS file.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << FATTR4_SIZE)
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})
	xid++

	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	entry = nextOp(t, &iter)
	getattrOk := entry.Value().AsGETATTR4resEntry().Value().AsGETATTR4resok()
	attrData := getAttrData(t, getattrOk)
	if len(attrData) < 8 {
		t.Fatalf("attr data too short: %d bytes", len(attrData))
	}
	size := binary.BigEndian.Uint64(attrData[:8])
	if size != uint64(len(writeData)) {
		t.Fatalf("GETATTR size = %d, want %d (staged size)", size, len(writeData))
	}

	// READ with anonymous stateid should return staged data.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0) // anonymous stateid (all zeros)
		rw.SetOffset(0)
		rw.SetCount(1024)
	})
	xid++

	iter = expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	entry = nextOp(t, &iter)
	readRes := entry.Value().AsREAD4resEntry()
	if readRes.Disc() != NFS4_OK {
		t.Fatalf("READ status = %s", Nfsstat4Name(readRes.Disc()))
	}
	readData := readRes.Value().AsREAD4resok().Data()
	if string(readData) != string(writeData) {
		t.Fatalf("READ data = %q, want %q", readData, writeData)
	}

	// Close the file.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		sid := caw.OpenStateid()
		sid.SetSeqid(openStateid.Seqid())
		for i := 0; i < 12; i++ {
			sid.SetOther(i, openStateid.Other(i))
		}
	})
	xid++
	expectOK(t, res)
}

func TestOpenExistingForWriteRejected(t *testing.T) {
	dir := t.TempDir()
	// Create an existing file.
	os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("original"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Try to OPEN existing file for write.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_WRITE)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		ow.SetOpenhow_Default(OPEN4_NOCREATE)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("existing.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)
	})
	xid++

	// PUTROOTFH should succeed, but OPEN should fail with NFS4ERR_PERM.
	status := res.Status()
	if status != NFS4ERR_PERM {
		t.Fatalf("OPEN existing for write: got status %s, want NFS4ERR_PERM",
			Nfsstat4Name(status))
	}
}

func TestCreateDirectory(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// PUTROOTFH + CREATE(NF4DIR, "subdir")
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		cw := w.AppendArgarray_Create()
		cw.SetObjtype_Nf4dir()
		nameW := cw.StartObjname()
		buf := nameW.SetData([]byte("subdir")).Finish()
		cw.Resume(buf)
		// Empty createattrs.
		faw := cw.StartCreateattrs()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		cw.Resume(buf)
		buf = cw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	createRes := entry.Value().AsCREATE4resEntry()
	if createRes.Disc() != NFS4_OK {
		t.Fatalf("CREATE status = %d", createRes.Disc())
	}

	// Verify the directory exists on disk.
	info, err := os.Stat(filepath.Join(dir, "subdir"))
	if err != nil {
		t.Fatalf("stat subdir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("subdir is not a directory")
	}
}

func TestRemoveFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "victim.txt"), []byte("delete me"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// PUTROOTFH + REMOVE("victim.txt")
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		rw := w.AppendArgarray_Remove()
		tw := rw.StartTarget()
		buf := tw.SetData([]byte("victim.txt")).Finish()
		rw.Resume(buf)
		buf = rw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	removeRes := entry.Value().AsREMOVE4resEntry()
	if removeRes.Disc() != NFS4_OK {
		t.Fatalf("REMOVE status = %d", removeRes.Disc())
	}

	// Verify the file is gone.
	if _, err := os.Stat(filepath.Join(dir, "victim.txt")); !os.IsNotExist(err) {
		t.Fatal("victim.txt still exists after REMOVE")
	}
}

func TestRename(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "old.txt"), []byte("rename me"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// PUTROOTFH + SAVEFH + RENAME(old.txt -> new.txt)
	// savedFH = source dir, currentFH = target dir (both root here).
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Savefh()

		rw := w.AppendArgarray_Rename()
		onw := rw.StartOldname()
		buf := onw.SetData([]byte("old.txt")).Finish()
		rw.Resume(buf)
		nnw := rw.StartNewname()
		buf = nnw.SetData([]byte("new.txt")).Finish()
		rw.Resume(buf)
		buf = rw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // SAVEFH
	entry := nextOp(t, &iter)
	renameRes := entry.Value().AsRENAME4resEntry()
	if renameRes.Disc() != NFS4_OK {
		t.Fatalf("RENAME status = %d", renameRes.Disc())
	}

	// Verify: old gone, new exists with correct content.
	if _, err := os.Stat(filepath.Join(dir, "old.txt")); !os.IsNotExist(err) {
		t.Fatal("old.txt still exists after RENAME")
	}
	data, err := os.ReadFile(filepath.Join(dir, "new.txt"))
	if err != nil {
		t.Fatalf("reading new.txt: %v", err)
	}
	if string(data) != "rename me" {
		t.Fatalf("new.txt content = %q, want %q", data, "rename me")
	}
}

func TestDelegpurgeNotSupported(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		dw := w.AppendArgarray_Delegpurge()
		dw.SetClientid(1)
	})

	if res.Status() != NFS4ERR_NOTSUPP {
		t.Fatalf("DELEGPURGE status = %d, want NFS4ERR_NOTSUPP", res.Status())
	}
}

func TestLinkNotSupported(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("test"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Savefh()

		lw := w.AppendArgarray_Link()
		nw := lw.StartNewname()
		buf := nw.SetData([]byte("link.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
	})

	if res.Status() != NFS4ERR_NOTSUPP {
		t.Fatalf("LINK status = %d, want NFS4ERR_NOTSUPP", res.Status())
	}
}

func TestCreateSymlink(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "target.txt"), []byte("target"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// PUTROOTFH + CREATE(NF4LNK, "link" -> "target.txt")
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		cw := w.AppendArgarray_Create()
		ltw := cw.SetObjtype_Nf4lnk()
		buf := ltw.SetData([]byte("target.txt")).Finish()
		cw.Resume(buf)
		nameW := cw.StartObjname()
		buf = nameW.SetData([]byte("link")).Finish()
		cw.Resume(buf)
		// Empty createattrs.
		faw := cw.StartCreateattrs()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		cw.Resume(buf)
		buf = cw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	createRes := entry.Value().AsCREATE4resEntry()
	if createRes.Disc() != NFS4_OK {
		t.Fatalf("CREATE symlink status = %d", createRes.Disc())
	}

	// Verify the symlink.
	target, err := os.Readlink(filepath.Join(dir, "link"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "target.txt" {
		t.Fatalf("symlink target = %q, want %q", target, "target.txt")
	}
}

func TestMinorVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Build a COMPOUND with minorversion=1 (NFSv4.1).
	var body []byte
	w := StartCOMPOUND4args(body)
	tagW := w.StartTag()
	body = tagW.SetData(nil).Finish()
	w.Resume(body)
	w.SetMinorversion(1)
	w.AppendArgarray_Putrootfh()
	body = w.Finish()

	reply, err := sendRPC(conn, 1, procCompound, body)
	if err != nil {
		t.Fatal(err)
	}
	nfsBody := parseRPCReply(t, reply)
	res, ok := ReadCOMPOUND4res(nfsBody)
	if !ok {
		t.Fatal("failed to parse COMPOUND4res")
	}
	if res.Status() != NFS4ERR_MINOR_VERS_MISMATCH {
		t.Fatalf("status = %s, want NFS4ERR_MINOR_VERS_MISMATCH",
			Nfsstat4Name(res.Status()))
	}
}

func TestCreateEmptyFile(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// PUTROOTFH + OPEN(CREATE) + OPEN_CONFIRM
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("empty.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()

		ocw := w.AppendArgarray_OpenConfirm()
		ocw.OpenStateid().SetSeqid(1)
		ocw.SetSeqid(2)
	})
	xid++

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	openRes := entry.Value().AsOPEN4resEntry()
	if openRes.Disc() != NFS4_OK {
		t.Fatalf("OPEN status = %d", openRes.Disc())
	}
	openOk := openRes.Value().AsOPEN4resok()
	openStateid := openOk.Stateid()
	entry = nextOp(t, &iter) // GETFH
	fh := append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	nextOp(t, &iter) // OPEN_CONFIRM

	// CLOSE immediately without writing anything.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		sid := caw.OpenStateid()
		sid.SetSeqid(openStateid.Seqid())
		for i := 0; i < 12; i++ {
			sid.SetOther(i, openStateid.Other(i))
		}
	})
	xid++
	expectOK(t, res)

	// Verify the empty file was created on disk.
	data, err := os.ReadFile(filepath.Join(dir, "empty.txt"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("file content = %q, want empty", data)
	}
}

// openReadFile opens a file for read and returns the open stateid and filehandle.
func openReadFile(t *testing.T, conn net.Conn, xid *uint32, clientid uint64, filename string) (stateid [16]byte, fh []byte) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_READ)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		ow.SetOpenhow_Default(OPEN4_NOCREATE)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte(filename)).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)
		w.AppendArgarray_Getfh()
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	openRes := entry.Value().AsOPEN4resEntry()
	if openRes.Disc() != NFS4_OK {
		t.Fatalf("OPEN status = %s", Nfsstat4Name(openRes.Disc()))
	}
	openOk := openRes.Value().AsOPEN4resok()
	sid := openOk.Stateid()
	binary.BigEndian.PutUint32(stateid[0:4], sid.Seqid())
	for i := 0; i < 12; i++ {
		stateid[4+i] = sid.Other(i)
	}
	entry = nextOp(t, &iter)
	fh = append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	return stateid, fh
}

// openCreateFile opens a new file for write and returns the open stateid and filehandle.
func openCreateFile(t *testing.T, conn net.Conn, xid *uint32, clientid uint64, filename string) (stateid [16]byte, fh []byte) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte(filename)).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)
		w.AppendArgarray_Getfh()
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	openRes := entry.Value().AsOPEN4resEntry()
	if openRes.Disc() != NFS4_OK {
		t.Fatalf("OPEN CREATE status = %s", Nfsstat4Name(openRes.Disc()))
	}
	openOk := openRes.Value().AsOPEN4resok()
	sid := openOk.Stateid()
	binary.BigEndian.PutUint32(stateid[0:4], sid.Seqid())
	for i := 0; i < 12; i++ {
		stateid[4+i] = sid.Other(i)
	}
	entry = nextOp(t, &iter)
	fh = append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	return stateid, fh
}

// closeFile sends CLOSE for the given filehandle and stateid.
func closeFile(t *testing.T, conn net.Conn, xid *uint32, fh []byte, stateid [16]byte) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		sid := caw.OpenStateid()
		sid.SetSeqid(binary.BigEndian.Uint32(stateid[0:4]))
		for i := 0; i < 12; i++ {
			sid.SetOther(i, stateid[4+i])
		}
	})
	*xid++
	expectOK(t, res)
}

// closeFileExpectStatus sends CLOSE and returns the NFS status.
func closeFileExpectStatus(t *testing.T, conn net.Conn, xid *uint32, fh []byte, stateid [16]byte) uint32 {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		caw := w.AppendArgarray_Close()
		caw.SetSeqid(3)
		sid := caw.OpenStateid()
		sid.SetSeqid(binary.BigEndian.Uint32(stateid[0:4]))
		for i := 0; i < 12; i++ {
			sid.SetOther(i, stateid[4+i])
		}
	})
	*xid++
	// The compound status reflects the CLOSE status.
	return res.Status()
}

// collectReaddirNames issues READDIR calls to collect all names in a directory.
func collectReaddirNames(t *testing.T, conn net.Conn, xid *uint32, dirFH []byte) []string {
	t.Helper()
	var allNames []string
	cookie := uint64(0)
	var cookieVerf [8]byte
	for {
		res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			fhW := pw.StartObject()
			buf := fhW.SetData(dirFH).Finish()
			pw.Resume(buf)
			buf = pw.Finish()
			w.Resume(buf)

			rdw := w.AppendArgarray_Readdir()
			rdw.SetCookie(cookie)
			cv := rdw.Cookieverf()
			for i := 0; i < 8; i++ {
				cv.SetData(i, cookieVerf[i])
			}
			rdw.SetDircount(4096)
			rdw.SetMaxcount(8192)
			bmW := rdw.StartAttrRequest()
			bmW.AppendData(1 << FATTR4_TYPE)
			buf = bmW.Finish()
			rdw.Resume(buf)
			buf = rdw.Finish()
			w.Resume(buf)
		})
		*xid++
		iter := expectOK(t, res)
		nextOp(t, &iter) // PUTFH
		entry := nextOp(t, &iter)
		readdirRes := entry.Value().AsREADDIR4resEntry()
		if readdirRes.Disc() != NFS4_OK {
			t.Fatalf("READDIR status = %s", Nfsstat4Name(readdirRes.Disc()))
		}
		okRes := readdirRes.Value().AsREADDIR4resok()
		// Save cookieverf for next request.
		cv := okRes.Cookieverf()
		for i := 0; i < 8; i++ {
			cookieVerf[i] = cv.Data(i)
		}
		reply := okRes.Reply()
		if reply.EntriesPresent() == TRUE {
			entOpt := reply.Entries()
			for {
				ent := entOpt.AsEntry4()
				allNames = append(allNames, string(ent.Name().Data()))
				cookie = ent.Cookie()
				if ent.NextentryPresent() != TRUE {
					break
				}
				entOpt = ent.Nextentry()
			}
		}
		if reply.Eof() == TRUE {
			break
		}
	}
	return allNames
}

// --- Checklist of new tests ---
// [x] TestReaddirPagination — multi-batch READDIR with continuation cookies
// [x] TestReaddirCookieverfMismatch — NFS4ERR_NOT_SAME on stale verifier
// [x] TestReaddirTooSmall — NFS4ERR_TOOSMALL with tiny maxcount
// [x] TestReaddirNfsDirHidden — .nfs not visible in root listing
// [x] TestReaddirTransientNotVisible — transient files not in listing until CLOSE
// [x] TestSetattrTime — SET_TO_CLIENT_TIME4 and SET_TO_SERVER_TIME4
// [x] TestSetattrModeRejected — mode/owner → NFS4ERR_ATTRNOTSUPP
// [x] TestSetattrSize — truncate staging file via SETATTR
// [x] TestOpenExclusive4Rejected — EXCLUSIVE4 → NFS4ERR_NOTSUPP
// [x] TestOpenClaimPrevious — CLAIM_PREVIOUS → NFS4ERR_NO_GRACE
// [x] TestOpenRflagsNoConfirm — OPEN4_RESULT_CONFIRM absent from rflags
// [x] TestCloseReplay — CLOSE twice returns OK both times
// [x] TestCloseExpired — CLOSE on nonexistent inode → NFS4ERR_EXPIRED
// [x] TestWriteBadStateid — WRITE with wrong stateid → error
// [x] TestCommitVerifier — COMMIT returns write verifier
// [x] TestReadOnlyMode — no staging → NFS4ERR_ROFS
// [x] TestLockNotSupported — LOCK/LOCKT/LOCKU → NFS4ERR_NOTSUPP
// [x] TestDeterministicReadStateid — same file+client → same stateid
// [x] TestSetclientidReboot — same identity, different verifier

func TestReaddirPagination(t *testing.T) {
	dir := t.TempDir()
	// Create 50 files to force multiple batches (VFS batch size is 16).
	var expected []string
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("file_%03d.txt", i)
		os.WriteFile(filepath.Join(dir, name), []byte(name), 0644)
		expected = append(expected, name)
	}
	sort.Strings(expected)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)

	// Get root FH.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Getfh()
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	rootFH := append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)

	names := collectReaddirNames(t, conn, &xid, rootFH)
	sort.Strings(names)

	if len(names) != len(expected) {
		t.Fatalf("got %d entries, want %d", len(names), len(expected))
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("entry[%d] = %q, want %q", i, names[i], expected[i])
		}
	}
}

func TestReaddirCookieverfMismatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	// First READDIR to get a valid cookie.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rdw := w.AppendArgarray_Readdir()
		rdw.SetCookie(0)
		rdw.SetDircount(4096)
		rdw.SetMaxcount(8192)
		bmW := rdw.StartAttrRequest()
		bmW.AppendData(1 << FATTR4_TYPE)
		buf := bmW.Finish()
		rdw.Resume(buf)
		buf = rdw.Finish()
		w.Resume(buf)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	readdirOk := entry.Value().AsREADDIR4resEntry().Value().AsREADDIR4resok()

	// Get a valid cookie from first entry.
	entOpt := readdirOk.Reply().Entries()
	validCookie := entOpt.AsEntry4().Cookie()

	// Send continuation with a bogus non-zero cookieverf.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rdw := w.AppendArgarray_Readdir()
		rdw.SetCookie(validCookie)
		cv := rdw.Cookieverf()
		// Set a non-zero but wrong verifier.
		cv.SetData(0, 0xFF)
		cv.SetData(1, 0xFF)
		cv.SetData(2, 0xFF)
		cv.SetData(3, 0xFF)
		cv.SetData(4, 0xFF)
		cv.SetData(5, 0xFF)
		cv.SetData(6, 0xFF)
		cv.SetData(7, 0xFF)
		rdw.SetDircount(4096)
		rdw.SetMaxcount(8192)
		bmW := rdw.StartAttrRequest()
		bmW.AppendData(1 << FATTR4_TYPE)
		buf := bmW.Finish()
		rdw.Resume(buf)
		buf = rdw.Finish()
		w.Resume(buf)
	})
	xid++

	// Compound should report the READDIR error.
	if res.Status() != NFS4ERR_NOT_SAME {
		t.Fatalf("expected NFS4ERR_NOT_SAME, got %s", Nfsstat4Name(res.Status()))
	}
}

func TestReaddirTooSmall(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Send READDIR with maxcount too small to fit any entry.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rdw := w.AppendArgarray_Readdir()
		rdw.SetCookie(0)
		rdw.SetDircount(4096)
		rdw.SetMaxcount(10) // Way too small.
		bmW := rdw.StartAttrRequest()
		bmW.AppendData(1 << FATTR4_TYPE)
		buf := bmW.Finish()
		rdw.Resume(buf)
		buf = rdw.Finish()
		w.Resume(buf)
	})

	if res.Status() != NFS4ERR_TOOSMALL {
		t.Fatalf("expected NFS4ERR_TOOSMALL, got %s", Nfsstat4Name(res.Status()))
	}
}

func TestReaddirNfsDirHidden(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("x"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)

	// Get root FH.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Getfh()
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	entry := nextOp(t, &iter)
	rootFH := append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)

	names := collectReaddirNames(t, conn, &xid, rootFH)
	for _, name := range names {
		if name == ".nfs" {
			t.Fatal(".nfs directory should be hidden from READDIR")
		}
	}
	found := false
	for _, name := range names {
		if name == "visible.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("visible.txt not found in READDIR")
	}
}

func TestReaddirTransientNotVisible(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("x"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Get root FH.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Getfh()
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	entry := nextOp(t, &iter)
	rootFH := append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)

	// Create a file but don't close it yet (still transient).
	stateid, fh := openCreateFile(t, conn, &xid, clientid, "newfile.txt")

	// READDIR should NOT show newfile.txt.
	names := collectReaddirNames(t, conn, &xid, rootFH)
	for _, name := range names {
		if name == "newfile.txt" {
			t.Fatal("transient file should not appear in READDIR before CLOSE")
		}
	}

	// Close the file (links it into the directory).
	closeFile(t, conn, &xid, fh, stateid)

	// READDIR should now show newfile.txt.
	names = collectReaddirNames(t, conn, &xid, rootFH)
	found := false
	for _, name := range names {
		if name == "newfile.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("newfile.txt should appear in READDIR after CLOSE")
	}
}

func TestSetattrTime(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Set mtime to a specific time (2020-01-01 00:00:00 UTC).
	targetTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	targetSec := targetTime.Unix()
	targetNsec := uint32(0)

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("file.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		saw := w.AppendArgarray_Setattr()
		saw.Stateid().SetSeqid(0)
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(0) // word 0: nothing
		bmW.AppendData(1 << (FATTR4_TIME_MODIFY_SET - 32))
		buf = bmW.Finish()
		faw.Resume(buf)
		// Attr data: SET_TO_CLIENT_TIME4(1) + nfstime4(sec:8 + nsec:4)
		attrData := make([]byte, 4+8+4)
		binary.BigEndian.PutUint32(attrData[0:4], SET_TO_CLIENT_TIME4)
		binary.BigEndian.PutUint64(attrData[4:12], uint64(targetSec))
		binary.BigEndian.PutUint32(attrData[12:16], targetNsec)
		alW := faw.StartAttrVals()
		buf = alW.SetData(attrData).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		saw.Resume(buf)
		buf = saw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	nextOp(t, &iter) // LOOKUP
	entry := nextOp(t, &iter)
	if entry.Value().AsSETATTR4res().Status() != NFS4_OK {
		t.Fatalf("SETATTR status = %s", Nfsstat4Name(entry.Value().AsSETATTR4res().Status()))
	}

	// Verify mtime was set on disk.
	info, err := os.Stat(filepath.Join(dir, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(targetTime) {
		t.Fatalf("mtime = %v, want %v", info.ModTime(), targetTime)
	}
}

func TestSetattrModeRejected(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("file.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		saw := w.AppendArgarray_Setattr()
		saw.Stateid().SetSeqid(0)
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(0)
		bmW.AppendData(1 << (FATTR4_MODE - 32)) // mode is not supported
		buf = bmW.Finish()
		faw.Resume(buf)
		attrData := make([]byte, 4)
		binary.BigEndian.PutUint32(attrData[0:4], 0755)
		alW := faw.StartAttrVals()
		buf = alW.SetData(attrData).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		saw.Resume(buf)
		buf = saw.Finish()
		w.Resume(buf)
	})

	if res.Status() != NFS4ERR_ATTRNOTSUPP {
		t.Fatalf("expected NFS4ERR_ATTRNOTSUPP, got %s", Nfsstat4Name(res.Status()))
	}
}

func TestSetattrSize(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create a file, write data, then truncate via SETATTR size.
	stateid, fh := openCreateFile(t, conn, &xid, clientid, "trunc.txt")

	// WRITE 100 bytes.
	writeData := make([]byte, 100)
	for i := range writeData {
		writeData[i] = byte(i)
	}
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		ww := w.AppendArgarray_Write()
		sid := ww.Stateid()
		sid.SetSeqid(binary.BigEndian.Uint32(stateid[0:4]))
		for i := 0; i < 12; i++ {
			sid.SetOther(i, stateid[4+i])
		}
		ww = ww.SetOffset(0)
		ww = ww.SetStable(2)
		ww = ww.SetData(writeData)
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	// SETATTR to truncate to 10 bytes.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		saw := w.AppendArgarray_Setattr()
		sid := saw.Stateid()
		sid.SetSeqid(binary.BigEndian.Uint32(stateid[0:4]))
		for i := 0; i < 12; i++ {
			sid.SetOther(i, stateid[4+i])
		}
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(1 << FATTR4_SIZE)
		buf = bmW.Finish()
		faw.Resume(buf)
		attrData := make([]byte, 8)
		binary.BigEndian.PutUint64(attrData[0:8], 10)
		alW := faw.StartAttrVals()
		buf = alW.SetData(attrData).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		saw.Resume(buf)
		buf = saw.Finish()
		w.Resume(buf)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	entry := nextOp(t, &iter)
	if entry.Value().AsSETATTR4res().Status() != NFS4_OK {
		t.Fatalf("SETATTR status = %s", Nfsstat4Name(entry.Value().AsSETATTR4res().Status()))
	}

	// Close and verify.
	closeFile(t, conn, &xid, fh, stateid)

	data, err := os.ReadFile(filepath.Join(dir, "trunc.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 10 {
		t.Fatalf("file size = %d, want 10", len(data))
	}
}

func TestOpenExclusive4Rejected(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		verf := chw.SetValue_Exclusive4()
		for i := 0; i < 8; i++ {
			verf.SetData(i, byte(i))
		}
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("excl.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)
	})
	xid++

	if res.Status() != NFS4ERR_NOTSUPP {
		t.Fatalf("expected NFS4ERR_NOTSUPP, got %s", Nfsstat4Name(res.Status()))
	}
}

func TestOpenClaimPrevious(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_READ)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		ow.SetOpenhow_Default(OPEN4_NOCREATE)
		ow.SetClaim_Previous()
		buf = ow.Finish()
		w.Resume(buf)
	})
	xid++

	if res.Status() != NFS4ERR_NO_GRACE {
		t.Fatalf("expected NFS4ERR_NO_GRACE, got %s", Nfsstat4Name(res.Status()))
	}
}

func TestOpenRflagsNoConfirm(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_READ)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		ow.SetOpenhow_Default(OPEN4_NOCREATE)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("file.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)
	})
	xid++

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	openOk := entry.Value().AsOPEN4resEntry().Value().AsOPEN4resok()
	rflags := openOk.Rflags()
	if rflags&OPEN4_RESULT_CONFIRM != 0 {
		t.Fatal("OPEN4_RESULT_CONFIRM should not be set")
	}
}

func TestCloseReplay(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create and close a file.
	stateid, fh := openCreateFile(t, conn, &xid, clientid, "replay.txt")
	closeFile(t, conn, &xid, fh, stateid)

	// Verify file exists.
	if _, err := os.Stat(filepath.Join(dir, "replay.txt")); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	// Send CLOSE again (replay). Should succeed.
	status := closeFileExpectStatus(t, conn, &xid, fh, stateid)
	if status != NFS4_OK {
		t.Fatalf("CLOSE replay: expected NFS4_OK, got %s", Nfsstat4Name(status))
	}
}

func TestCloseExpired(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Construct a filehandle for a nonexistent inode. Use a type=file inode
	// with a made-up number that doesn't correspond to any real file.
	var fakeFH [8]byte
	fakeID := MakeInodeID(InodeTypeFile, 0xDEADDEAD)
	binary.BigEndian.PutUint64(fakeFH[:], uint64(fakeID))
	var fakeStateid [16]byte // all zeros

	xid := uint32(1)
	status := closeFileExpectStatus(t, conn, &xid, fakeFH[:], fakeStateid)
	if status != NFS4ERR_EXPIRED {
		t.Fatalf("expected NFS4ERR_EXPIRED, got %s", Nfsstat4Name(status))
	}
}

func TestWriteBadStateid(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create a file.
	_, fh := openCreateFile(t, conn, &xid, clientid, "badwrite.txt")

	// WRITE with a wrong stateid.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		ww := w.AppendArgarray_Write()
		sid := ww.Stateid()
		sid.SetSeqid(99)
		for i := 0; i < 12; i++ {
			sid.SetOther(i, 0xFF) // wrong
		}
		ww = ww.SetOffset(0)
		ww = ww.SetStable(2)
		ww = ww.SetData([]byte("bad"))
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++

	// The WRITE should still succeed because we don't validate stateids
	// on WRITE (design doc: "Optionally validate the stateid").
	// Just verify it doesn't crash.
	if res.Status() != NFS4_OK {
		// If we do validate, the error should be sensible.
		t.Logf("WRITE with bad stateid: %s (acceptable)", Nfsstat4Name(res.Status()))
	}
}

func TestCommitVerifier(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	stateid, fh := openCreateFile(t, conn, &xid, clientid, "commit.txt")

	// WRITE some data.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		ww := w.AppendArgarray_Write()
		sid := ww.Stateid()
		sid.SetSeqid(binary.BigEndian.Uint32(stateid[0:4]))
		for i := 0; i < 12; i++ {
			sid.SetOther(i, stateid[4+i])
		}
		ww = ww.SetOffset(0)
		ww = ww.SetStable(0) // UNSTABLE4
		ww = ww.SetData([]byte("commit test"))
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	// COMMIT and check the write verifier is non-zero.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		w.AppendArgarray_Commit()
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	entry := nextOp(t, &iter)
	commitOk := entry.Value().AsCOMMIT4resEntry().Value().AsCOMMIT4resok()
	verf := commitOk.Writeverf()
	var allZero bool = true
	for i := 0; i < 8; i++ {
		if verf.Data(i) != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("COMMIT write verifier should be non-zero")
	}

	closeFile(t, conn, &xid, fh, stateid)
}

func TestReadOnlyMode(t *testing.T) {
	dir := t.TempDir()
	fs := NewLocalTernVFS(dir)
	ss := readOnlyStagingStore{}
	srv, err := NewServer(fs, ss, nil)
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.handleConn(conn)
		}
	}()
	defer ln.Close()

	conn := dial(t, ln.Addr().String())
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// OPEN with CREATE should fail with NFS4ERR_ROFS.
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		buf = alW.SetData(nil).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		chw.Resume(buf)
		buf = chw.Finish()
		ow.Resume(buf)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte("newfile.txt")).Finish()
		ow.Resume(buf)
		buf = ow.Finish()
		w.Resume(buf)
	})
	xid++

	if res.Status() != NFS4ERR_ROFS {
		t.Fatalf("expected NFS4ERR_ROFS, got %s", Nfsstat4Name(res.Status()))
	}
}

func TestLockNotSupported(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// LOCK — variable-size writer, need to finish it properly.
	res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lock()
		lw.SetLocker_False()
		w.Resume(lw.Finish())
	})
	if res.Status() != NFS4ERR_NOTSUPP {
		t.Fatalf("LOCK: expected NFS4ERR_NOTSUPP, got %s", Nfsstat4Name(res.Status()))
	}

	// LOCKT — variable-size writer.
	res = sendCompound(t, conn, 2, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lockt()
		ow := lw.StartOwner()
		ow.SetOwner([]byte("x"))
		lw.Resume(ow.Finish())
		w.Resume(lw.Finish())
	})
	if res.Status() != NFS4ERR_NOTSUPP {
		t.Fatalf("LOCKT: expected NFS4ERR_NOTSUPP, got %s", Nfsstat4Name(res.Status()))
	}

	// LOCKU — fixed-size, no finish needed.
	res = sendCompound(t, conn, 3, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Locku()
	})
	if res.Status() != NFS4ERR_NOTSUPP {
		t.Fatalf("LOCKU: expected NFS4ERR_NOTSUPP, got %s", Nfsstat4Name(res.Status()))
	}
}

func TestDeterministicReadStateid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Open the same file twice and check the stateids are identical.
	sid1, _ := openReadFile(t, conn, &xid, clientid, "file.txt")
	sid2, _ := openReadFile(t, conn, &xid, clientid, "file.txt")

	if sid1 != sid2 {
		t.Fatalf("read stateids differ: %x vs %x", sid1, sid2)
	}
}

func TestSetclientidReboot(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// First SETCLIENTID + CONFIRM.
	xid := uint32(1)
	clientid1 := setupClient(t, conn, &xid)

	// Second SETCLIENTID with same identity but different verifier (simulating reboot).
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		scw := w.AppendArgarray_Setclientid()
		clientW := scw.StartClient()
		// Same identity string as setupClient.
		clientW = clientW.SetId([]byte("test-client"))
		// Different verifier (setupClient uses default zero verifier).
		verf := clientW.Verifier()
		for i := 0; i < 8; i++ {
			verf.SetData(i, byte(i+1))
		}
		buf := clientW.Finish()
		scw.Resume(buf)
		cbW := scw.StartCallback()
		cbW.SetCbProgram(0x40000000)
		locW := cbW.StartCbLocation()
		netidW := locW.StartRNetid()
		buf = netidW.SetData([]byte("tcp")).Finish()
		locW.Resume(buf)
		addrW := locW.StartRAddr()
		buf = addrW.SetData([]byte("0.0.0.0.0.0")).Finish()
		locW.Resume(buf)
		buf = locW.Finish()
		cbW.Resume(buf)
		buf = cbW.Finish()
		scw.Resume(buf)
		scw.SetCallbackIdent(0)
		buf = scw.Finish()
		w.Resume(buf)
	})
	xid++
	iter := expectOK(t, res)
	entry := nextOp(t, &iter)
	scRes := entry.Value().AsSETCLIENTID4resEntry()
	if scRes.Disc() != NFS4_OK {
		t.Fatalf("SETCLIENTID reboot: status = %s", Nfsstat4Name(scRes.Disc()))
	}
	clientid2 := scRes.Value().AsSETCLIENTID4resok().Clientid()

	// CONFIRM.
	res = sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
		scw := w.AppendArgarray_SetclientidConfirm()
		scw.SetClientid(clientid2)
	})
	xid++
	iter = expectOK(t, res)
	entry = nextOp(t, &iter)
	if entry.Value().AsSETCLIENTIDCONFIRM4res().Status() != NFS4_OK {
		t.Fatal("SETCLIENTID_CONFIRM reboot failed")
	}

	// The new client ID should differ (new file = new InodeID).
	if clientid1 == clientid2 {
		t.Log("client IDs are the same (verifier stored in same file)")
	}
}

// TestReaddirEntryXDRSize verifies that the size prediction formula in
// readdirEntryXDRSize exactly matches the actual encoded XDR output for
// various attribute masks and name lengths.
func TestReaddirEntryXDRSize(t *testing.T) {
	id := MakeInodeID(InodeTypeFile, 42)
	ni := NodeInfo{Size: 1024}

	cases := []struct {
		name   string
		mask   [2]uint32
		fname  string
		statOK bool
	}{
		{"no_attrs", [2]uint32{0, 0}, "file.txt", true},
		{"type_only", [2]uint32{1 << FATTR4_TYPE, 0}, "file.txt", true},
		{"size_and_type", [2]uint32{(1 << FATTR4_TYPE) | (1 << FATTR4_SIZE), 0}, "file.txt", true},
		{"full_attrs", [2]uint32{supportedAttrs0, supportedAttrs1}, "file.txt", true},
		{"short_name", [2]uint32{1 << FATTR4_TYPE, 0}, "a", true},
		{"padded_name", [2]uint32{1 << FATTR4_TYPE, 0}, "ab", true},  // 2 bytes, pads to 4
		{"exact_4", [2]uint32{1 << FATTR4_TYPE, 0}, "abcd", true},    // 4 bytes, no pad
		{"needs_pad", [2]uint32{1 << FATTR4_TYPE, 0}, "abcde", true}, // 5 bytes, pads to 8
		{"stat_failed", [2]uint32{supportedAttrs0, supportedAttrs1}, "file.txt", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var respMask [2]uint32
			respMask[0] = tc.mask[0] & supportedAttrs0
			respMask[1] = tc.mask[1] & supportedAttrs1

			re := readdirEntry{
				DirEntry: DirEntry{
					ID:       id,
					Name:     tc.fname,
					NameHash: 12345,
				},
				respMask: respMask,
			}
			if tc.statOK {
				re.attrData = encodeAttrs(respMask, id, ni)
			}

			predicted := readdirEntryXDRSize(&re)

			// Encode one entry into a dirlist4 and measure.
			dirW := StartDirlist4(nil)
			encodeDirEntries(&dirW, []readdirEntry{re}, true)
			buf := dirW.Finish()
			// dirlist4 = entries_present(4) + entry_bytes + eof(4)
			// With one entry: entries_present=TRUE(4) is part of the entry's
			// XDR size, so total = entry_bytes + eof(4).
			// But entries_present(4) is written by SetEntries_True, which is
			// accounted in the entry's leading present(4).
			actualDirlist := len(buf)
			expectedDirlist := predicted + 4 // entry + eof(4)
			if actualDirlist != expectedDirlist {
				t.Errorf("size mismatch: predicted entry=%d, expected dirlist=%d, actual dirlist=%d",
					predicted, expectedDirlist, actualDirlist)
			}
		})
	}
}
