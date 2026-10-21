// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/openstate"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

type readErrorVFS struct {
	*vfs.LocalTernVFS
	failID vfs.InodeID
}

func (v *readErrorVFS) Read(
	id vfs.InodeID,
	offset uint64,
	dest []byte,
) (int, bool, error) {
	if id == v.failID {
		return 0, false, syscall.EIO
	}
	return v.LocalTernVFS.Read(id, offset, dest)
}

type panicOnceReadVFS struct {
	*vfs.LocalTernVFS
	panic bool
}

type mutableBlockingLinkVFS struct {
	*vfs.LocalTernVFS
	once    sync.Once
	started chan struct{}
	release chan struct{}
	mu      sync.Mutex
	links   int
}

type markerCheckingStagingStore struct {
	staging.Store
	clients *clientstore.Store
	checked bool
}

func (s *markerCheckingStagingStore) Rebind(
	id vfs.InodeID,
	clientID uint64,
	stateID nfs4.StateID,
) error {
	active, err := s.clients.HasOpen(clientID, stateID)
	if err != nil {
		return err
	}
	if !active {
		return errors.New("staging rebound before durable open marker")
	}
	s.checked = true
	return s.Store.Rebind(id, clientID, stateID)
}

func (v *mutableBlockingLinkVFS) LinkFile(
	fileID vfs.InodeID,
	cookie vfs.Cookie,
	dirID vfs.InodeID,
	name string,
	data io.Reader,
) error {
	if clientstore.LeaseTemp(name) {
		return v.LocalTernVFS.LinkFile(fileID, cookie, dirID, name, data)
	}
	v.mu.Lock()
	v.links++
	v.mu.Unlock()
	v.once.Do(func() {
		close(v.started)
		<-v.release
	})
	return v.LocalTernVFS.LinkFile(
		fileID, cookie, dirID, name, data,
	)
}

func (v *mutableBlockingLinkVFS) linkCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.links
}

func (v *panicOnceReadVFS) Read(
	id vfs.InodeID,
	offset uint64,
	dest []byte,
) (int, bool, error) {
	if v.panic {
		v.panic = false
		panic("injected read panic")
	}
	return v.LocalTernVFS.Read(id, offset, dest)
}

func startTestServer(
	t *testing.T,
	dir string,
) (addr string, cleanup func()) {
	t.Helper()
	_, addr, cleanup = startTestServerWithServer(t, dir)
	return addr, cleanup
}

func startTestServerWithServer(
	t *testing.T,
	dir string,
) (srv *Server, addr string, cleanup func()) {
	t.Helper()
	fs := vfs.NewLocalTernVFS(dir)
	stagingDir := t.TempDir()
	ss, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err = NewServer(fs, ss, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup = serveTestServer(t, srv)
	return srv, addr, cleanup
}

func serveTestServer(t *testing.T, srv *Server) (addr string, cleanup func()) {
	t.Helper()
	useTestWireEpoch(srv)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	conns := make(map[net.Conn]struct{})
	var wg sync.WaitGroup
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns[conn] = struct{}{}
			wg.Add(1)
			mu.Unlock()
			go func() {
				defer func() {
					mu.Lock()
					delete(conns, conn)
					mu.Unlock()
					wg.Done()
				}()
				srv.handleConn(conn)
			}()
		}
	}()
	registerTestServer(ln.Addr().String(), srv)
	var once sync.Once
	return ln.Addr().String(), func() {
		once.Do(func() {
			unregisterTestServer(ln.Addr().String())
			ln.Close()
			mu.Lock()
			for conn := range conns {
				conn.Close()
			}
			mu.Unlock()
			wg.Wait()
			srv.clients.StopLeaseWorkers()
			srv.waitForClientGC()
		})
	}
}

func (s *Server) waitForClientGC() {
	s.clientGCMu.Lock()
	done := s.clientGCDone
	s.clientGCMu.Unlock()
	if done != nil {
		<-done
	}
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
	if proc == procCompound {
		return sendSessionRPC(conn, xid, body)
	}
	return sendRawRPC(conn, xid, proc, body)
}

func sendRawRPC(conn net.Conn, xid uint32, proc uint32, body []byte) ([]byte, error) {
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

func parseRPCReply(reply []byte) ([]byte, error) {
	if len(reply) < 24 {
		return nil, fmt.Errorf("reply too short: %d bytes", len(reply))
	}
	acceptStat := binary.BigEndian.Uint32(reply[20:24])
	if acceptStat != 0 {
		return nil, fmt.Errorf("accept_stat = %d, want SUCCESS", acceptStat)
	}
	return reply[24:], nil
}

func mustParseRPCReply(t *testing.T, reply []byte) []byte {
	t.Helper()
	body, err := parseRPCReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func buildCompoundBody(tag []byte, build func(w *nfs4.COMPOUND4argsWriter)) []byte {
	var body []byte
	w := nfs4.StartCOMPOUND4args(body)
	tagW := w.StartTag()
	body = tagW.SetData(tag).Finish()
	w.Resume(body)
	w.SetMinorversion(nfsMinorVersion)

	if build != nil {
		build(&w)
	}

	return w.Finish()
}

// sendCompound builds a compound, sends it, and returns the parsed COMPOUND4res.
func sendCompound(t *testing.T, conn net.Conn, xid uint32, build func(w *nfs4.COMPOUND4argsWriter)) nfs4.COMPOUND4res {
	t.Helper()
	body := buildCompoundBody(nil, build)
	reply, err := sendRPC(conn, xid, procCompound, body)
	if err != nil {
		t.Fatal(err)
	}
	nfsBody, err := parseRPCReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	res, ok := nfs4.ReadCOMPOUND4res(nfsBody)
	if !ok {
		t.Fatal("failed to parse COMPOUND4res")
	}
	return res
}

func TestEmptyCompound(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	const xid = 12153
	reply, err := sendRPC(conn, xid, procCompound, buildCompoundBody([]byte("empty"), nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.BigEndian.Uint32(reply[:4]); got != xid {
		t.Fatalf("reply xid = %d, want %d", got, xid)
	}

	res, ok := nfs4.ReadCOMPOUND4res(mustParseRPCReply(t, reply))
	if !ok {
		t.Fatal("failed to parse COMPOUND4res")
	}
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("status = %d, want NFS4_OK", res.Status())
	}
	if got := string(res.Tag().Data()); got != "empty" {
		t.Fatalf("tag = %q, want %q", got, "empty")
	}
	if res.ResarrayCount() != 0 {
		t.Fatalf("resarray count = %d, want 0", res.ResarrayCount())
	}
}

func TestRPCPrincipalIgnoresAuthSysCallerFields(t *testing.T) {
	authSysBody := func(stamp uint32, machine string, uid uint32) []byte {
		body := binary.BigEndian.AppendUint32(nil, stamp)
		body = binary.BigEndian.AppendUint32(body, uint32(len(machine)))
		body = append(body, machine...)
		body = append(body, make([]byte, (4-len(machine)%4)%4)...)
		body = binary.BigEndian.AppendUint32(body, uid)
		body = binary.BigEndian.AppendUint32(body, 100)
		body = binary.BigEndian.AppendUint32(body, 2)
		body = binary.BigEndian.AppendUint32(body, 10)
		body = binary.BigEndian.AppendUint32(body, 20)
		return body
	}
	first := &rpcRequest{
		credFlavor: nfs4.AUTH_SYS,
		credBody:   authSysBody(1, "first-host", 1000),
	}
	second := &rpcRequest{
		credFlavor: nfs4.AUTH_SYS,
		credBody:   authSysBody(2, "second-host", 1000),
	}
	if first.principal() != second.principal() {
		t.Fatal("AUTH_SYS timestamp or machine name changed the RPC principal")
	}
	differentUID := &rpcRequest{
		credFlavor: nfs4.AUTH_SYS,
		credBody:   authSysBody(2, "second-host", 1001),
	}
	if first.principal() == differentUID.principal() {
		t.Fatal("different AUTH_SYS uid produced the same RPC principal")
	}
	differentFlavor := &rpcRequest{
		credFlavor: nfs4.AUTH_NONE,
		credBody:   first.credBody,
	}
	if first.principal() == differentFlavor.principal() {
		t.Fatal("different credential flavor produced the same RPC principal")
	}
	shortFirst := (&rpcRequest{
		credFlavor: nfs4.AUTH_SYS,
		credBody:   []byte{1, 2, 3},
	}).principal()
	shortSecond := (&rpcRequest{
		credFlavor: nfs4.AUTH_SYS,
		credBody:   []byte{1, 2, 4},
	}).principal()
	if shortFirst == shortSecond {
		t.Fatal("short AUTH_SYS bodies collapsed to one RPC principal")
	}
}

func TestCompoundOperationLimit(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	body := buildCompoundBody([]byte("too-many"), func(w *nfs4.COMPOUND4argsWriter) {
		for range maxCompoundOperations + 1 {
			w.AppendArgarray_Putrootfh()
		}
	})
	reply, err := sendRPC(conn, 1, procCompound, body)
	if err != nil {
		t.Fatal(err)
	}
	res, ok := nfs4.ReadCOMPOUND4res(mustParseRPCReply(t, reply))
	if !ok {
		t.Fatal("failed to parse COMPOUND4res")
	}
	if res.Status() != nfs4.NFS4ERR_TOO_MANY_OPS {
		t.Fatalf("status = %d, want NFS4ERR_TOO_MANY_OPS", res.Status())
	}
	if got := string(res.Tag().Data()); got != "too-many" {
		t.Fatalf("tag = %q, want %q", got, "too-many")
	}
	if res.ResarrayCount() != 0 {
		t.Fatalf("resarray count = %d, want 0", res.ResarrayCount())
	}
}

// expectOK checks compound status is NFS4_OK and returns the resarray iterator.
func expectOK(t *testing.T, res nfs4.COMPOUND4res) nfs4.NfsResop4EntryIter {
	t.Helper()
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("compound status = %d, want NFS4_OK", res.Status())
	}
	return res.Resarray()
}

// nextOp advances the iterator and returns the entry.
func nextOp(t *testing.T, iter *nfs4.NfsResop4EntryIter) nfs4.NfsResop4Entry {
	t.Helper()
	if !iter.Next() {
		t.Fatal("expected another op in resarray")
	}
	return iter.Resarray()
}

// getAttrData extracts attribute values from a GETATTR4resok fattr4.
// Uses Fattr4.AttrVals().Data() which requires correct codegen for
// sequential variable-size field getters (bitmap4 then attrlist4).
func getAttrData(t *testing.T, getattrOk nfs4.GETATTR4resok) []byte {
	t.Helper()
	return getattrOk.ObjAttributes().AttrVals().Data()
}

func setVerifier(verifier nfs4.Verifier4, data [8]byte) {
	for i, b := range data {
		verifier.SetData(i, b)
	}
}

// requestClientID runs EXCHANGE_ID. The returned confirm value carries the
// CREATE_SESSION sequence id for confirmClientID.
func requestClientID(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	identity string,
	verifier [8]byte,
) (uint64, [8]byte) {
	t.Helper()
	clientID, seq, status, err := exchangeID(conn, *xid, identity, verifier)
	*xid++
	if err != nil {
		t.Fatal(err)
	}
	if status != nfs4.NFS4_OK {
		t.Fatalf("EXCHANGE_ID status = %s", nfs4.Nfsstat4Name(status))
	}
	testIdentities.set(clientID, identity, verifier)
	return clientID, encodeConfirm(seq)
}

// confirmClientID runs CREATE_SESSION, which confirms the clientid.
func confirmClientID(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	clientid uint64,
	confirm [8]byte,
) uint32 {
	t.Helper()
	ts, status, err := createSession(conn, *xid, clientid, decodeConfirm(confirm))
	*xid++
	if err != nil {
		t.Fatal(err)
	}
	if ts != nil {
		ts.identity, ts.verifier = testIdentities.get(clientid)
	}
	return status
}

// setupClient runs EXCHANGE_ID + CREATE_SESSION with a zero verifier.
func setupClient(t *testing.T, conn net.Conn, xid *uint32) uint64 {
	t.Helper()
	return setupClientWithVerifier(t, conn, xid, [8]byte{})
}

func setupClientWithVerifier(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	verifier [8]byte,
) uint64 {
	t.Helper()
	clientid, confirm := requestClientID(
		t, conn, xid, "test-client", verifier)
	if status := confirmClientID(
		t, conn, xid, clientid, confirm,
	); status != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION status = %s", nfs4.Nfsstat4Name(status))
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
	mustParseRPCReply(t, reply)
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

	res := sendCompound(t, conn, 2, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1<<nfs4.FATTR4_TYPE | 1<<nfs4.FATTR4_FILEID)
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
	if entry.Disc() != nfs4.OP_PUTROOTFH {
		t.Fatalf("op[0] = %d, want OP_PUTROOTFH", entry.Disc())
	}
	if entry.Value().AsPUTROOTFH4res().Status() != nfs4.NFS4_OK {
		t.Fatal("PUTROOTFH failed")
	}

	// GETATTR
	entry = nextOp(t, &iter)
	if entry.Disc() != nfs4.OP_GETATTR {
		t.Fatalf("op[1] = %d, want OP_GETATTR", entry.Disc())
	}
	getattrRes := entry.Value().AsGETATTR4resEntry()
	if getattrRes.Disc() != nfs4.NFS4_OK {
		t.Fatalf("GETATTR status = %d", getattrRes.Disc())
	}
}

func TestPutpubfh(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putpubfh()
		w.AppendArgarray_Getfh()
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTPUBFH
	entry := nextOp(t, &iter)
	getfhRes := entry.Value().AsGETFH4resEntry()
	if getfhRes.Disc() != nfs4.NFS4_OK {
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

	res := sendCompound(t, conn, 3, func(w *nfs4.COMPOUND4argsWriter) {
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
	if getfhRes.Disc() != nfs4.NFS4_OK {
		t.Fatalf("GETFH status = %d", getfhRes.Disc())
	}
	fhData := getfhRes.Value().AsGETFH4resok().Object().Data()
	if len(fhData) != 8 {
		t.Fatalf("GETFH returned FH of %d bytes, want 8", len(fhData))
	}

	// READ — validate data and EOF.
	entry = nextOp(t, &iter)
	readRes := entry.Value().AsREAD4resEntry()
	if readRes.Disc() != nfs4.NFS4_OK {
		t.Fatalf("READ status = %d", readRes.Disc())
	}
	readOk := readRes.Value().AsREAD4resok()
	data := readOk.Data()
	if string(data) != "Hello, NFS!" {
		t.Fatalf("READ data = %q, want %q", data, "Hello, NFS!")
	}
	if readOk.Eof() != nfs4.TRUE {
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	if readOk.Eof() != nfs4.FALSE {
		t.Fatalf("READ eof = TRUE, want FALSE (not at end of file)")
	}

	// Read past end — should get remaining data with EOF.
	res = sendCompound(t, conn, 2, func(w *nfs4.COMPOUND4argsWriter) {
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
	if readOk.Eof() != nfs4.TRUE {
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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	if readOk.Eof() != nfs4.TRUE {
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

	res := sendCompound(t, conn, 4, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		rdw := w.AppendArgarray_Readdir()
		rdw.SetCookie(0)
		rdw.SetDircount(4096)
		rdw.SetMaxcount(8192)
		bmW := rdw.StartAttrRequest()
		bmW.AppendData(1<<nfs4.FATTR4_TYPE | 1<<nfs4.FATTR4_FILEID)
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
	if readdirRes.Disc() != nfs4.NFS4_OK {
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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	if readdirRes.Disc() != nfs4.NFS4_OK {
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	res = sendCompound(t, conn, 2, func(w *nfs4.COMPOUND4argsWriter) {
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData([]byte{1, 2, 3, 4}).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
	})

	if res.Status() != nfs4.NFS4ERR_BADHANDLE {
		t.Fatalf("compound status = %d, want NFS4ERR_BADHANDLE (%d)", res.Status(), nfs4.NFS4ERR_BADHANDLE)
	}
}

func TestPutfhRejectsClientStoreHandles(t *testing.T) {
	dir := t.TempDir()
	srv, addr, cleanup := startTestServerWithServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientID := incarnationForTest(t, srv, setupClient(t, conn, &xid))
	recordID, err := srv.fs.Lookup(vfs.InodeID(clientID), clientstore.ClientRecordName)
	if err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string]vfs.InodeID{
		"nfs directory":     srv.clients.NFSDirID,
		"clients directory": srv.clients.DirID,
		"clientid":          vfs.InodeID(clientID),
		"client record":     recordID,
	} {
		t.Run(name, func(t *testing.T) {
			fh := binary.BigEndian.AppendUint64(nil, uint64(id))
			res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
				pw := w.AppendArgarray_Putfh()
				buf := pw.StartObject().SetData(fh).Finish()
				pw.Resume(buf)
				w.Resume(pw.Finish())
			})
			xid++
			if res.Status() != nfs4.NFS4ERR_BADHANDLE {
				t.Fatalf("PUTFH status = %s, want NFS4ERR_BADHANDLE",
					nfs4.Nfsstat4Name(res.Status()))
			}
		})
	}
}

func TestPutfhAllowsUncachedClientStoreFileHandles(t *testing.T) {
	dir := t.TempDir()
	firstFS := vfs.NewLocalTernVFS(dir)
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	store, clientID := clientstore.NewConfirmedStoreClient(
		t, firstFS, []byte("client"), [8]byte{1}, owner)
	stateID := nfs4.StateID{1}
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}
	recordID, err := firstFS.Lookup(
		vfs.InodeID(clientID), clientstore.ClientRecordName)
	if err != nil {
		t.Fatal(err)
	}
	leaseID, err := firstFS.Lookup(
		vfs.InodeID(clientID), store.LeaseName)
	if err != nil {
		t.Fatal(err)
	}
	markerID, err := firstFS.Lookup(
		vfs.InodeID(clientID), clientstore.ActiveOpenName(stateID))
	if err != nil {
		t.Fatal(err)
	}
	externalID, err := firstFS.CreateFile(
		firstFS.RootID(), "ordinary", bytes.NewReader([]byte("data")))
	if err != nil {
		t.Fatal(err)
	}

	secondFS := vfs.NewLocalTernVFS(dir)
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(secondFS, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []vfs.InodeID{
		recordID, leaseID, markerID, externalID,
	} {
		secondFS.ForgetParent(id)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	for name, id := range map[string]vfs.InodeID{
		"record": recordID,
		"lease":  leaseID,
		"marker": markerID,
	} {
		t.Run(name, func(t *testing.T) {
			res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
				pw := w.AppendArgarray_Putfh()
				buf := pw.StartObject().SetData(
					vfs.InodeIDToFH(id)).Finish()
				pw.Resume(buf)
				w.Resume(pw.Finish())
			})
			xid++
			if res.Status() != nfs4.NFS4_OK {
				t.Fatalf("PUTFH status = %s, want NFS4_OK",
					nfs4.Nfsstat4Name(res.Status()))
			}
		})
	}
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(
			vfs.InodeIDToFH(recordID)).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())
		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(4096)
	})
	xid++
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("uncached client record READ = %s, want NFS4_OK",
			nfs4.Nfsstat4Name(res.Status()))
	}
	res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(
			vfs.InodeIDToFH(externalID)).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())
	})
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("ordinary PUTFH status = %s, want NFS4_OK",
			nfs4.Nfsstat4Name(res.Status()))
	}
}

func TestLocalTernVFSCreateFileReplacesExistingFile(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	firstID, err := fs.CreateFile(
		fs.RootID(), "record", bytes.NewReader([]byte("first")))
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := fs.CreateFile(
		fs.RootID(), "record", bytes.NewReader([]byte("second")))
	if err != nil {
		t.Fatal(err)
	}
	if secondID == firstID {
		t.Fatalf("replacement retained inode %d", firstID)
	}
	if _, err := fs.Stat(firstID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replaced inode still resolves: %v", err)
	}
	if got, err := fs.ReadAll(secondID); err != nil {
		t.Fatal(err)
	} else if string(got) != "second" {
		t.Fatalf("replacement data = %q, want second", got)
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Restorefh()
	})

	if res.Status() != nfs4.NFS4ERR_RESTOREFH {
		t.Fatalf("status = %d, want NFS4ERR_RESTOREFH (%d)", res.Status(), nfs4.NFS4ERR_RESTOREFH)
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	if readlinkRes.Disc() != nfs4.NFS4_OK {
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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		aw := w.AppendArgarray_Access()
		aw.SetAccess(nfs4.ACCESS4_READ | nfs4.ACCESS4_LOOKUP)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	accessRes := entry.Value().AsACCESS4resEntry()
	if accessRes.Disc() != nfs4.NFS4_OK {
		t.Fatalf("ACCESS status = %d", accessRes.Disc())
	}
	accessOk := accessRes.Value().AsACCESS4resok()
	requested := uint32(nfs4.ACCESS4_READ | nfs4.ACCESS4_LOOKUP)
	if accessOk.Supported() != requested {
		t.Fatalf("supported = %d, want %d", accessOk.Supported(), requested)
	}
	if accessOk.Access() != requested {
		t.Fatalf("access = %d, want %d", accessOk.Access(), requested)
	}
}

func TestAccessMasksUnsupportedBitsByType(t *testing.T) {
	const allAccess = nfs4.ACCESS4_READ | nfs4.ACCESS4_LOOKUP | nfs4.ACCESS4_MODIFY |
		nfs4.ACCESS4_EXTEND | nfs4.ACCESS4_DELETE | nfs4.ACCESS4_EXECUTE
	const unknownAccess = 0x40

	tests := []struct {
		name       string
		object     string
		wantAccess uint32
	}{
		{
			name:       "directory",
			object:     "dir",
			wantAccess: allAccess &^ nfs4.ACCESS4_EXECUTE,
		},
		{
			name:       "file",
			object:     "file",
			wantAccess: allAccess &^ (nfs4.ACCESS4_LOOKUP | nfs4.ACCESS4_DELETE),
		},
		{
			name:       "symlink",
			object:     "link",
			wantAccess: allAccess &^ (nfs4.ACCESS4_LOOKUP | nfs4.ACCESS4_DELETE),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "dir"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("file", filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				lw := w.AppendArgarray_Lookup()
				nw := lw.StartObjname()
				buf := nw.SetData([]byte(test.object)).Finish()
				lw.Resume(buf)
				w.Resume(lw.Finish())

				aw := w.AppendArgarray_Access()
				aw.SetAccess(allAccess | unknownAccess)
			})
			iter := expectOK(t, res)
			nextOp(t, &iter)
			nextOp(t, &iter)
			entry := nextOp(t, &iter)
			ok := entry.Value().AsACCESS4resEntry().
				Value().AsACCESS4resok()
			if ok.Supported() != test.wantAccess {
				t.Errorf("supported = %#x, want %#x",
					ok.Supported(), test.wantAccess)
			}
			if ok.Access() != test.wantAccess {
				t.Errorf("access = %#x, want %#x",
					ok.Access(), test.wantAccess)
			}
		})
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
	stateid, fh := openReadFile(t, conn, &xid, clientid, "file.txt")

	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())
		rw := w.AppendArgarray_Read()
		setStateid(rw.Stateid(), stateid)
		rw.SetOffset(0)
		rw.SetCount(1024)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	entry := nextOp(t, &iter)
	readOk := entry.Value().AsREAD4resEntry().Value().AsREAD4resok()
	if string(readOk.Data()) != "content" {
		t.Fatalf("READ = %q, want %q", readOk.Data(), "content")
	}
	closeFile(t, conn, &xid, fh, stateid)
}

func addConfirmedOpenForTest(
	t *testing.T,
	srv *Server,
	clientID uint64,
	owner string,
	fileID vfs.InodeID,
	write bool,
) openstate.State {
	t.Helper()
	state, status := openstate.AddOpenForTest(srv.opens,
		openstate.OwnerKey{ClientID: clientID, Owner: owner},
		fileID, write, nfs4.StateID{},
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("OPEN state setup = %s", nfs4.Nfsstat4Name(status))
	}
	if err := srv.clients.MarkOpen(clientID, state.ID); err != nil {
		t.Fatal(err)
	}
	return state
}

func newClientStoreTestServer(
	t *testing.T,
) (*clientstore.LookupFailureVFS, *Server, net.Conn, *uint32, uint64) {
	t.Helper()
	fs := &clientstore.LookupFailureVFS{
		TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
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
	conn := dial(t, addr)
	t.Cleanup(func() {
		conn.Close()
		cleanup()
	})
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	return fs, srv, conn, &xid, clientID
}

func TestLookupDurableOpenStorageErrorPreservesClientState(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	fs := &clientstore.LookupFailureVFS{TernVFS: baseFS}
	stagingStore, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, stagingStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	srv.clients.SetClock(func() time.Time { return base })
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := srv.clients.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	publishLeaseForTest(t, srv, clientID)
	readerID, _, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	reader := addConfirmedOpenForTest(
		t, srv, clientID, "reader", readerID, false)
	writerID, cookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	writer := addConfirmedOpenForTest(
		t, srv, clientID, "writer", writerID, true)
	if _, err := stagingStore.Create(writerID, staging.Meta{
		DirID:      fs.RootID(),
		FileName:   "writer",
		TernCookie: cookie,
		NFSStateID: writer.ID,
		ClientID:   clientID,
	}); err != nil {
		t.Fatal(err)
	}

	srv.clients.StopLeaseWorkers()
	fs.Name = clientstore.ConfirmedName
	fs.Remaining = 1
	_, stateid := nfs4.StartStateid4(nil)
	writeStateID(stateid, reader.Generation, reader.ID)
	if _, status := srv.lookupDurableOpen(
		clientID, stateid, readerID,
	); status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("lookup status = %s, want NFS4ERR_DELAY",
			nfs4.Nfsstat4Name(status))
	}
	if _, status := srv.opens.Lookup(
		writer.Owner.ClientID, writer.ID, writer.Generation, writerID,
	); status != nfs4.NFS4_OK {
		t.Fatalf("unrelated state after lookup error = %s, want NFS4_OK",
			nfs4.Nfsstat4Name(status))
	}
	if stagingStore.Get(writerID) == nil {
		t.Fatal("lookup error removed unrelated staging")
	}
	if _, err := baseFS.Lookup(
		vfs.InodeID(clientID), clientstore.ActiveOpenName(writer.ID),
	); err != nil {
		t.Fatalf("lookup error removed unrelated marker: %v", err)
	}
}

func TestOwnerOperationsPreserveStateOnClientStoreError(t *testing.T) {
	t.Run("close", func(t *testing.T) {
		fs, srv, conn, xid, clientID := newClientStoreTestServer(t)
		stateid, fh := openCreateFile(
			t, conn, xid, clientID, "close.txt")
		// The worker's evidence read would consume the injected failure.
		srv.clients.StopLeaseWorkers()
		fs.Name = clientstore.ConfirmedName
		fs.Remaining = 1
		if status := closeFileStatus(
			t, conn, xid, fh, stateid); status != nfs4.NFS4ERR_DELAY {
			t.Fatalf("CLOSE with store error = %s, want DELAY",
				nfs4.Nfsstat4Name(status))
		}
		if status := closeFileStatus(
			t, conn, xid, fh, stateid); status != nfs4.NFS4_OK {
			t.Fatalf("retried CLOSE = %s, want OK",
				nfs4.Nfsstat4Name(status))
		}
	})

}

func TestExpireIfLeaseDeadKeepsClientWithoutSlot(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := srv.clients.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	if _, status := openstate.AddOpenForTest(srv.opens,
		openstate.OwnerKey{ClientID: clientID, Owner: "owner"},
		vfs.MakeInodeID(vfs.InodeTypeFile, 1), false, nfs4.StateID{},
	); status != nfs4.NFS4_OK {
		t.Fatalf("OPEN = %s", nfs4.Nfsstat4Name(status))
	}

	expired, err := srv.clients.ExpireIfLeaseDead(clientID)
	if err != nil {
		t.Fatal(err)
	}
	if expired {
		t.Fatal("client with an in-flight first OPEN was expired")
	}
}

// A collected incarnation directory has no leases or markers left, so the
// sweep must drop any process-local state which still names it.
func TestExpireIfLeaseDeadExpiresCollectedIncarnation(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := srv.clients.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	publishLeaseForTest(t, srv, clientID)
	if err := srv.clients.MarkOpen(clientID, nfs4.StateID{1}); err != nil {
		t.Fatal(err)
	}

	identityID, err := fs.LookupParent(vfs.InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	name, found, err := srv.clients.IncarnationName(
		identityID, vfs.InodeID(clientID))
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("confirmed incarnation is missing from its identity")
	}
	if err := srv.clients.RemoveIncarnation(
		identityID, vfs.InodeID(clientID), name,
	); err != nil {
		t.Fatal(err)
	}

	expired, err := srv.clients.ExpireIfLeaseDead(clientID)
	if err != nil {
		t.Fatal(err)
	}
	if !expired {
		t.Fatal("collected incarnation was not expired")
	}
	for _, cached := range srv.clients.CachedClientIDs() {
		if cached == clientID {
			t.Fatal("collected incarnation remains in the local client cache")
		}
	}
	if err := srv.clients.MarkOpen(
		clientID, nfs4.StateID{2},
	); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("MarkOpen on collected incarnation = %v, want NFS4ERR_STALE_CLIENTID",
			err)
	}
	// The sweep itself must tolerate the missing incarnation.
	srv.sweepExpiredClientState()
}

func TestStagingSweepKeepsClientWithoutLeaseSlot(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	stagingStore, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, stagingStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	srv.startedAt = base
	srv.clients.SetClock(func() time.Time {
		return base.Add(clientstore.LeaseTime + time.Second)
	})
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := srv.clients.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	fileID, cookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stagingStore.Create(fileID, staging.Meta{
		DirID:      fs.RootID(),
		FileName:   "opening.txt",
		TernCookie: cookie,
		NFSStateID: nfs4.StateID{1},
		ClientID:   clientID,
	}); err != nil {
		t.Fatal(err)
	}

	srv.removeExpiredStaging()
	if stagingStore.Get(fileID) == nil {
		t.Fatal("staging sweep removed an in-flight first OPEN")
	}
}

func TestServerLeaseSweepExpiresAbandonedStaging(t *testing.T) {
	baseFS := vfs.NewLocalTernVFS(t.TempDir())
	fs := &clientstore.RemoveFailureVFS{TernVFS: baseFS}
	stagingStore, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, stagingStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	srv.clients.SetClock(func() time.Time { return base })
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := srv.clients.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	publishLeaseForTest(t, srv, clientID)
	// The refresher's terminal steps would also expire the client's state
	// once the marker is noted; only the sweep's own retry is under test.
	srv.clients.StopLeaseWorkers()
	fileID, cookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	state := addConfirmedOpenForTest(
		t, srv, clientID, "writer", fileID, true)
	if _, err := stagingStore.Create(fileID, staging.Meta{
		DirID:      fs.RootID(),
		FileName:   "abandoned.txt",
		TernCookie: cookie,
		NFSStateID: state.ID,
		ClientID:   clientID,
	}); err != nil {
		t.Fatal(err)
	}

	srv.clients.SetClock(func() time.Time {
		return base.Add(clientstore.LeaseWriteTerm + time.Second)
	})
	fs.Name = clientstore.ActiveOpenName(state.ID)
	fs.Remaining = 1
	srv.sweepExpiredClientState()
	if stagingStore.Get(fileID) == nil {
		t.Fatal("marker cleanup failure removed staging")
	}
	if _, status := srv.opens.Lookup(
		state.Owner.ClientID, state.ID, state.Generation, fileID,
	); status != nfs4.NFS4_OK {
		t.Fatalf("marker cleanup failure removed state = %s",
			nfs4.Nfsstat4Name(status))
	}
	srv.sweepExpiredClientState()
	assertRetiredStaging(t, stagingStore, fileID)
	if _, status := srv.opens.Lookup(
		state.Owner.ClientID, state.ID, state.Generation, fileID,
	); status != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("lease sweep state = %s, want NFS4ERR_EXPIRED",
			nfs4.Nfsstat4Name(status))
	}
	if _, err := fs.Lookup(
		vfs.InodeID(clientID), clientstore.ActiveOpenName(state.ID),
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lease sweep retained active-open marker: %v", err)
	}
	if active, err := srv.clients.HasOpen(
		clientID, state.ID,
	); err != nil {
		t.Fatal(err)
	} else if active {
		t.Fatal("expired fleet marker remained active")
	}
}

func TestServerLeaseSweepKeepsClientRenewedByAnotherOwner(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	stagingStore, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, stagingStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	srv.clients.SetClock(func() time.Time { return base })
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := srv.clients.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	publishLeaseForTest(t, srv, clientID)
	writerID, cookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	writer := addConfirmedOpenForTest(
		t, srv, clientID, "writer", writerID, true)
	if _, err := stagingStore.Create(writerID, staging.Meta{
		DirID:      fs.RootID(),
		FileName:   "active.txt",
		TernCookie: cookie,
		NFSStateID: writer.ID,
		ClientID:   clientID,
	}); err != nil {
		t.Fatal(err)
	}
	readerID, _, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	reader := addConfirmedOpenForTest(
		t, srv, clientID, "reader", readerID, false)

	srv.clients.SetClock(func() time.Time {
		return base.Add(clientstore.LeaseRenewAfter + time.Second)
	})
	if active, err := srv.clients.HasOpen(
		clientID, reader.ID,
	); err != nil {
		t.Fatal(err)
	} else if !active {
		t.Fatal("second owner could not renew the client lease")
	}
	srv.clients.SetClock(func() time.Time {
		return base.Add(clientstore.LeaseTime + 10*time.Second)
	})
	srv.sweepExpiredClientState()
	if stagingStore.Get(writerID) == nil {
		t.Fatal("lease sweep removed staging for a renewed client")
	}
	if _, status := srv.opens.Lookup(
		writer.Owner.ClientID, writer.ID, writer.Generation, writerID,
	); status != nfs4.NFS4_OK {
		t.Fatalf("renewed writer state = %s, want NFS4_OK",
			nfs4.Nfsstat4Name(status))
	}
}

func TestNewServerDelaysExpiredRecoveredStagingRemoval(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	stagingDir := t.TempDir()
	stagingStore, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := clientstore.NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	clients.SetClock(func() time.Time { return base })
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := clients.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	fileID, cookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	stateID := nfs4.StateID{1}
	if err := clients.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}
	if _, err := stagingStore.Create(fileID, staging.Meta{
		DirID:      fs.RootID(),
		FileName:   "expired.txt",
		TernCookie: cookie,
		NFSStateID: stateID,
		ClientID:   clientID,
	}); err != nil {
		t.Fatal(err)
	}
	liveID, liveConfirm, err := clients.SetClientID(
		[8]byte{2}, []byte("live-client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients.ConfirmClientID(
		liveID, liveConfirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	clients.SetClock(func() time.Time { return base.Add(250 * time.Second) })
	liveStateID := nfs4.StateID{2}
	if err := clients.MarkOpen(liveID, liveStateID); err != nil {
		t.Fatal(err)
	}
	liveFileID, liveCookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stagingStore.Create(liveFileID, staging.Meta{
		DirID:      fs.RootID(),
		FileName:   "live.txt",
		TernCookie: liveCookie,
		NFSStateID: liveStateID,
		ClientID:   liveID,
	}); err != nil {
		t.Fatal(err)
	}
	staging.CloseFiles(t, stagingStore)
	recovered, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, recovered, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.startedAt = base.Add(200 * time.Second)
	srv.clients.SetClock(func() time.Time { return srv.startedAt })
	srv.removeExpiredStaging()
	if recovered.Get(fileID) == nil {
		t.Fatal("startup grace removed expired staging early")
	}
	srv.clients.SetClock(func() time.Time {
		return srv.startedAt.Add(clientstore.LeaseTime + time.Second)
	})
	srv.removeExpiredStaging()
	assertRetiredStaging(t, recovered, fileID)
	if recovered.Get(liveFileID) == nil {
		t.Fatal("startup removed staging with a live lease")
	}
}

func TestClientStoreGCKeepsConfirmedIncarnation(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	store, err := clientstore.NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	store.Now = func() time.Time { return base }
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := store.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	stateID := nfs4.StateID{1}
	if err := store.MarkOpen(clientID, stateID); err != nil {
		t.Fatal(err)
	}
	store.Now = func() time.Time {
		return base.Add(clientstore.LeaseTime + time.Second)
	}
	if err := store.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	store.Now = func() time.Time {
		return base.Add(clientstore.LeaseTime + clientstore.ClientGCGrace + 2*time.Second)
	}
	if err := store.CollectStaleForClient(vfs.InodeID(clientID)); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(vfs.InodeID(clientID)); err != nil {
		t.Fatalf("confirmed incarnation was collected: %v", err)
	}
	if confirmed, err := store.IsConfirmed(clientID); err != nil {
		t.Fatal(err)
	} else if !confirmed {
		t.Fatal("confirmed clientid lost its pointer")
	}
}

func countClientIncarnations(
	t *testing.T,
	fs vfs.TernVFS,
	identityID vfs.InodeID,
) int {
	t.Helper()
	var count int
	var cursor uint64
	for {
		entries, next, err := fs.Readdir(identityID, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name, clientstore.IncarnationPrefix) {
				count++
			}
		}
		if next == 0 {
			return count
		}
		cursor = next
	}
}

func TestClientStoreRecognizesConfirmedClientAfterRestart(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	first, err := clientstore.NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	clientID, confirm, err := first.SetClientID(
		[8]byte{}, []byte("test-client"), clientstore.ClientOwner{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.ConfirmClientID(
		clientID, confirm, clientstore.RPCPrincipal{},
	); err != nil {
		t.Fatal(err)
	}

	second, err := clientstore.NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := second.IsConfirmed(clientID)
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed {
		t.Fatal("confirmed client was stale after ClientStore restart")
	}
}

func TestSetattr(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	if entry.Value().AsSETATTR4res().Status() != nfs4.NFS4_OK {
		t.Fatal("SETATTR failed")
	}
}

// RFC 8881 section 18 removes these NFSv4.0 operations.
func TestRemovedOperationsNotSupported(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	for _, test := range []struct {
		name  string
		build func(w *nfs4.COMPOUND4argsWriter)
	}{
		{"RELEASE_LOCKOWNER", func(w *nfs4.COMPOUND4argsWriter) {
			rlw := w.AppendArgarray_ReleaseLockowner()
			loW := rlw.StartLockOwner()
			loW = loW.SetClientid(clientid)
			loW = loW.SetOwner([]byte("test-lock-owner"))
			rlw.Resume(loW.Finish())
			w.Resume(rlw.Finish())
		}},
		{"RENEW", func(w *nfs4.COMPOUND4argsWriter) {
			w.AppendArgarray_Renew().SetClientid(clientid)
		}},
		{"OPEN_CONFIRM", func(w *nfs4.COMPOUND4argsWriter) {
			w.AppendArgarray_Putrootfh()
			w.AppendArgarray_OpenConfirm()
		}},
	} {
		res := sendCompound(t, conn, xid, test.build)
		xid++
		if res.Status() != nfs4.NFS4ERR_NOTSUPP {
			t.Errorf("%s status = %s, want NFS4ERR_NOTSUPP",
				test.name, nfs4.Nfsstat4Name(res.Status()))
		}
	}
}

// Every compound must start with SEQUENCE or a session-management operation.
func TestCompoundRequiresSequence(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
	})
	reply, err := sendRawRPC(conn, 1, procCompound, body)
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status() != nfs4.NFS4ERR_OP_NOT_IN_SESSION {
		t.Fatalf("status = %s, want NFS4ERR_OP_NOT_IN_SESSION",
			nfs4.Nfsstat4Name(res.Status()))
	}
}

func TestIllegalOp(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Illegal()
	})

	if res.Status() != nfs4.NFS4ERR_OP_ILLEGAL {
		t.Fatalf("status = %d, want NFS4ERR_OP_ILLEGAL (%d)", res.Status(), nfs4.NFS4ERR_OP_ILLEGAL)
	}
}

func TestUnknownOp(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// Empty tag, minor version one, one operation, and undefined operation zero.
	body := make([]byte, 16)
	binary.BigEndian.PutUint32(body[4:8], nfsMinorVersion)
	binary.BigEndian.PutUint32(body[8:12], 1)

	reply, err := sendRPC(conn, 1, procCompound, body)
	if err != nil {
		t.Fatal(err)
	}
	res, ok := nfs4.ReadCOMPOUND4res(mustParseRPCReply(t, reply))
	if !ok {
		t.Fatal("failed to parse COMPOUND4res")
	}
	if res.Status() != nfs4.NFS4ERR_OP_ILLEGAL {
		t.Fatalf("status = %s, want NFS4ERR_OP_ILLEGAL", nfs4.Nfsstat4Name(res.Status()))
	}
	if res.ResarrayCount() != 1 {
		t.Fatalf("resarray count = %d, want 1", res.ResarrayCount())
	}
	iter := res.Resarray()
	entry := nextOp(t, &iter)
	if entry.Disc() != nfs4.OP_ILLEGAL {
		t.Fatalf("result operation = %s, want OP_ILLEGAL", nfs4.NfsOpnum4Name(entry.Disc()))
	}
	if status := entry.Value().AsILLEGAL4res().Status(); status != nfs4.NFS4ERR_OP_ILLEGAL {
		t.Fatalf("operation status = %s, want NFS4ERR_OP_ILLEGAL", nfs4.Nfsstat4Name(status))
	}
}

func TestLookupNonExistent(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("nonexistent.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
	})

	if res.Status() != nfs4.NFS4ERR_NOENT {
		t.Fatalf("status = %d, want NFS4ERR_NOENT (%d)", res.Status(), nfs4.NFS4ERR_NOENT)
	}
	// Should have 2 ops: PUTROOTFH (success) + LOOKUP (failure).
	if res.ResarrayCount() != 2 {
		t.Fatalf("resarray count = %d, want 2", res.ResarrayCount())
	}
}

func TestLongNamesRejected(t *testing.T) {
	longName := make([]byte, maxTernNameLength+1)
	for i := range longName {
		longName[i] = 'a'
	}

	tests := []struct {
		name  string
		build func(*nfs4.COMPOUND4argsWriter)
	}{
		{
			name: "lookup",
			build: func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				lw := w.AppendArgarray_Lookup()
				nw := lw.StartObjname()
				buf := nw.SetData(longName).Finish()
				lw.Resume(buf)
				w.Resume(lw.Finish())
			},
		},
		{
			name: "create",
			build: func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				cw := w.AppendArgarray_Create()
				cw.SetObjtype_Nf4dir()
				nw := cw.StartObjname()
				buf := nw.SetData(longName).Finish()
				cw.Resume(buf)
				faw := cw.StartCreateattrs()
				bmw := faw.StartAttrmask()
				buf = bmw.Finish()
				faw.Resume(buf)
				avw := faw.StartAttrVals()
				buf = avw.SetData(nil).Finish()
				faw.Resume(buf)
				cw.Resume(faw.Finish())
				w.Resume(cw.Finish())
			},
		},
		{
			name: "remove",
			build: func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				rw := w.AppendArgarray_Remove()
				tw := rw.StartTarget()
				buf := tw.SetData(longName).Finish()
				rw.Resume(buf)
				w.Resume(rw.Finish())
			},
		},
		{
			name: "rename source",
			build: func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				w.AppendArgarray_Savefh()
				rw := w.AppendArgarray_Rename()
				ow := rw.StartOldname()
				buf := ow.SetData(longName).Finish()
				rw.Resume(buf)
				nw := rw.StartNewname()
				buf = nw.SetData([]byte("new")).Finish()
				rw.Resume(buf)
				w.Resume(rw.Finish())
			},
		},
		{
			name: "rename target",
			build: func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				w.AppendArgarray_Savefh()
				rw := w.AppendArgarray_Rename()
				ow := rw.StartOldname()
				buf := ow.SetData([]byte("old")).Finish()
				rw.Resume(buf)
				nw := rw.StartNewname()
				buf = nw.SetData(longName).Finish()
				rw.Resume(buf)
				w.Resume(rw.Finish())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, test.build)
			if res.Status() != nfs4.NFS4ERR_NAMETOOLONG {
				t.Fatalf("status = %s, want NFS4ERR_NAMETOOLONG",
					nfs4.Nfsstat4Name(res.Status()))
			}
		})
	}
}

func TestRenameInvalidNamesRejected(t *testing.T) {
	tests := []struct {
		name       string
		oldName    []byte
		newName    []byte
		wantStatus uint32
	}{
		{"empty source", nil, []byte("new"), nfs4.NFS4ERR_INVAL},
		{"empty target", []byte("old"), nil, nfs4.NFS4ERR_INVAL},
		{"dot source", []byte("."), []byte("new"), nfs4.NFS4ERR_BADNAME},
		{"dot target", []byte("old"), []byte("."), nfs4.NFS4ERR_BADNAME},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				w.AppendArgarray_Savefh()
				rw := w.AppendArgarray_Rename()
				ow := rw.StartOldname()
				buf := ow.SetData(test.oldName).Finish()
				rw.Resume(buf)
				nw := rw.StartNewname()
				buf = nw.SetData(test.newName).Finish()
				rw.Resume(buf)
				w.Resume(rw.Finish())
			})
			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
	}
}

func TestOpenInvalidNamesRejected(t *testing.T) {
	longName := make([]byte, maxTernNameLength+1)
	for i := range longName {
		longName[i] = 'a'
	}

	tests := []struct {
		name       string
		component  []byte
		wantStatus uint32
	}{
		{"empty", nil, nfs4.NFS4ERR_INVAL},
		{"too long", longName, nfs4.NFS4ERR_NAMETOOLONG},
		{"dot", []byte("."), nfs4.NFS4ERR_BADNAME},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			xid := uint32(1)
			clientID := setupClient(t, conn, &xid)
			res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				ow := w.AppendArgarray_Open()
				ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_READ)
				ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
				owner := ow.StartOwner()
				owner = owner.SetClientid(clientID)
				owner = owner.SetOwner([]byte("name-test"))
				buf := owner.Finish()
				ow.Resume(buf)
				ow.SetOpenhow_Default(nfs4.OPEN4_NOCREATE)
				claim := ow.SetClaim_Null()
				buf = claim.SetData(test.component).Finish()
				ow.Resume(buf)
				w.Resume(ow.Finish())
			})
			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
	}
}

func TestEmptyComponentsRejected(t *testing.T) {
	tests := []struct {
		name  string
		build func(*nfs4.COMPOUND4argsWriter)
	}{
		{
			name: "lookup",
			build: func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				lw := w.AppendArgarray_Lookup()
				nw := lw.StartObjname()
				buf := nw.SetData(nil).Finish()
				lw.Resume(buf)
				w.Resume(lw.Finish())
			},
		},
		{
			name: "remove",
			build: func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				rw := w.AppendArgarray_Remove()
				tw := rw.StartTarget()
				buf := tw.SetData(nil).Finish()
				rw.Resume(buf)
				w.Resume(rw.Finish())
			},
		},
		{
			name: "secinfo",
			build: func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				sw := w.AppendArgarray_Secinfo()
				nw := sw.StartName()
				buf := nw.SetData(nil).Finish()
				sw.Resume(buf)
				w.Resume(sw.Finish())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, test.build)
			if res.Status() != nfs4.NFS4ERR_INVAL {
				t.Fatalf("status = %s, want NFS4ERR_INVAL",
					nfs4.Nfsstat4Name(res.Status()))
			}
		})
	}
}

func TestLookuppAtRootRejected(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Lookupp()
	})
	if res.Status() != nfs4.NFS4ERR_NOENT {
		t.Fatalf("status = %s, want NFS4ERR_NOENT",
			nfs4.Nfsstat4Name(res.Status()))
	}
}

func TestSecinfoRequiresExistingName(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		sw := w.AppendArgarray_Secinfo()
		nw := sw.StartName()
		buf := nw.SetData([]byte("missing")).Finish()
		sw.Resume(buf)
		w.Resume(sw.Finish())
	})
	if res.Status() != nfs4.NFS4ERR_NOENT {
		t.Fatalf("status = %s, want NFS4ERR_NOENT",
			nfs4.Nfsstat4Name(res.Status()))
	}
}

func TestCreateInvalidFieldsRejected(t *testing.T) {
	maxSymlinkTarget := bytes.Repeat([]byte{'x'}, maxTernBytesLength)
	longSymlinkTarget := bytes.Repeat([]byte{'x'}, maxTernBytesLength+1)
	tests := []struct {
		name          string
		objname       []byte
		symlink       bool
		symlinkTarget []byte
		wantStatus    uint32
	}{
		{
			name:       "empty object name",
			objname:    nil,
			wantStatus: nfs4.NFS4ERR_INVAL,
		},
		{
			name:          "symlink target",
			objname:       []byte("link"),
			symlink:       true,
			symlinkTarget: nil,
			wantStatus:    nfs4.NFS4ERR_INVAL,
		},
		{
			name:          "maximum symlink target",
			objname:       []byte("link"),
			symlink:       true,
			symlinkTarget: maxSymlinkTarget,
			wantStatus:    nfs4.NFS4_OK,
		},
		{
			name:          "long symlink target",
			objname:       []byte("link"),
			symlink:       true,
			symlinkTarget: longSymlinkTarget,
			wantStatus:    nfs4.NFS4ERR_NAMETOOLONG,
		},
		{
			name:       "dot",
			objname:    []byte("."),
			wantStatus: nfs4.NFS4ERR_BADNAME,
		},
		{
			name:       "dot dot",
			objname:    []byte(".."),
			wantStatus: nfs4.NFS4ERR_BADNAME,
		},
		{
			name:       "slash",
			objname:    []byte("foo/bar"),
			wantStatus: nfs4.NFS4ERR_BADNAME,
		},
		{
			name:       "nul",
			objname:    []byte{'f', 'o', 'o', 0, 'b', 'a', 'r'},
			wantStatus: nfs4.NFS4ERR_BADNAME,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				cw := w.AppendArgarray_Create()
				if test.symlink {
					ltw := cw.SetObjtype_Nf4lnk()
					buf := ltw.SetData(test.symlinkTarget).Finish()
					cw.Resume(buf)
				} else {
					cw.SetObjtype_Nf4dir()
				}
				nw := cw.StartObjname()
				buf := nw.SetData(test.objname).Finish()
				cw.Resume(buf)
				faw := cw.StartCreateattrs()
				bmw := faw.StartAttrmask()
				buf = bmw.Finish()
				faw.Resume(buf)
				avw := faw.StartAttrVals()
				buf = avw.SetData(nil).Finish()
				faw.Resume(buf)
				cw.Resume(faw.Finish())
				w.Resume(cw.Finish())
			})
			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
	}
}

func TestCreateTypeAndAttributesValidated(t *testing.T) {
	tests := []struct {
		name       string
		objType    uint32
		mask       attrMask
		wantStatus uint32
	}{
		{
			name:       "regular file type",
			objType:    nfs4.NF4REG,
			wantStatus: nfs4.NFS4ERR_BADTYPE,
		},
		{
			name:       "read-only attribute",
			objType:    nfs4.NF4DIR,
			mask:       attrMask{1 << nfs4.FATTR4_LINK_SUPPORT, 0},
			wantStatus: nfs4.NFS4ERR_INVAL,
		},
		{
			name:       "unsupported writable attribute",
			objType:    nfs4.NF4DIR,
			mask:       attrMask{1 << nfs4.FATTR4_ACL, 0},
			wantStatus: nfs4.NFS4ERR_ATTRNOTSUPP,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				cw := w.AppendArgarray_Create()
				cw.SetObjtype_Default(test.objType)
				nw := cw.StartObjname()
				buf := nw.SetData([]byte("new")).Finish()
				cw.Resume(buf)
				faw := cw.StartCreateattrs()
				buf = finishTestFattr(faw, test.mask, nil)
				cw.Resume(buf)
				w.Resume(cw.Finish())
			})
			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
	}
}

func TestCreateExistingNameRejected(t *testing.T) {
	tests := []struct {
		name    string
		objType uint32
		setup   func(string) error
	}{
		{
			name:    "directory",
			objType: nfs4.NF4DIR,
			setup: func(path string) error {
				return os.Mkdir(path, 0755)
			},
		},
		{
			name:    "symlink",
			objType: nfs4.NF4LNK,
			setup: func(path string) error {
				return os.Symlink("target", path)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := test.setup(filepath.Join(dir, "existing")); err != nil {
				t.Fatal(err)
			}
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				cw := w.AppendArgarray_Create()
				if test.objType == nfs4.NF4LNK {
					tw := cw.SetObjtype_Nf4lnk()
					buf := tw.SetData([]byte("target")).Finish()
					cw.Resume(buf)
				} else {
					cw.SetObjtype_Nf4dir()
				}
				nw := cw.StartObjname()
				buf := nw.SetData([]byte("existing")).Finish()
				cw.Resume(buf)
				faw := cw.StartCreateattrs()
				buf = finishTestFattr(faw, attrMask{}, nil)
				cw.Resume(buf)
				w.Resume(cw.Finish())
			})
			if res.Status() != nfs4.NFS4ERR_EXIST {
				t.Fatalf("status = %s, want NFS4ERR_EXIST",
					nfs4.Nfsstat4Name(res.Status()))
			}
		})
	}
}

func TestOpenCreateAttributesValidated(t *testing.T) {
	tests := []struct {
		name       string
		mask       attrMask
		attrData   []byte
		wantStatus uint32
	}{
		{
			name:       "read-only attribute",
			mask:       attrMask{1 << nfs4.FATTR4_LINK_SUPPORT, 0},
			wantStatus: nfs4.NFS4ERR_INVAL,
		},
		{
			name:       "unsupported writable attribute",
			mask:       attrMask{1 << nfs4.FATTR4_ACL, 0},
			wantStatus: nfs4.NFS4ERR_ATTRNOTSUPP,
		},
		{
			name:       "size missing data",
			mask:       attrMask{1 << nfs4.FATTR4_SIZE, 0},
			wantStatus: nfs4.NFS4ERR_BADXDR,
		},
		{
			name:       "size exceeds signed backend range",
			mask:       attrMask{1 << nfs4.FATTR4_SIZE, 0},
			attrData:   []byte{0x80, 0, 0, 0, 0, 0, 0, 0},
			wantStatus: nfs4.NFS4ERR_FBIG,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()
			xid := uint32(1)
			clientID := setupClient(t, conn, &xid)

			res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				ow := w.AppendArgarray_Open()
				ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
				ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
				owner := ow.StartOwner()
				owner = owner.SetClientid(clientID)
				owner = owner.SetOwner([]byte("attr-test"))
				buf := owner.Finish()
				ow.Resume(buf)
				chw := ow.SetOpenhow_Create()
				faw := chw.SetValue_Unchecked4()
				buf = finishTestFattr(faw, test.mask, test.attrData)
				chw.Resume(buf)
				ow.Resume(chw.Finish())
				claim := ow.SetClaim_Null()
				buf = claim.SetData([]byte("new")).Finish()
				ow.Resume(buf)
				w.Resume(ow.Finish())
			})
			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
	}
}

func TestCreateFromNonDirectoryRejected(t *testing.T) {
	tests := []struct {
		name  string
		setup func(string) error
	}{
		{
			name: "file",
			setup: func(path string) error {
				return os.WriteFile(path, []byte("data"), 0644)
			},
		},
		{
			name: "symlink",
			setup: func(path string) error {
				return os.Symlink("target", path)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := test.setup(filepath.Join(dir, "parent")); err != nil {
				t.Fatal(err)
			}
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				lw := w.AppendArgarray_Lookup()
				nw := lw.StartObjname()
				buf := nw.SetData([]byte("parent")).Finish()
				lw.Resume(buf)
				w.Resume(lw.Finish())

				cw := w.AppendArgarray_Create()
				cw.SetObjtype_Nf4dir()
				nw = cw.StartObjname()
				buf = nw.SetData([]byte("child")).Finish()
				cw.Resume(buf)
				faw := cw.StartCreateattrs()
				bmw := faw.StartAttrmask()
				buf = bmw.Finish()
				faw.Resume(buf)
				avw := faw.StartAttrVals()
				buf = avw.SetData(nil).Finish()
				faw.Resume(buf)
				cw.Resume(faw.Finish())
				w.Resume(cw.Finish())
			})
			if res.Status() != nfs4.NFS4ERR_NOTDIR {
				t.Fatalf("status = %s, want NFS4ERR_NOTDIR",
					nfs4.Nfsstat4Name(res.Status()))
			}
		})
	}
}

func TestCurrentFilehandleTypeValidation(t *testing.T) {
	tests := []struct {
		name       string
		object     string
		operation  string
		wantStatus uint32
	}{
		{"lookup from file", "file", "lookup", nfs4.NFS4ERR_NOTDIR},
		{"lookup from symlink", "link", "lookup", nfs4.NFS4ERR_SYMLINK},
		{"lookupp from file", "file", "lookupp", nfs4.NFS4ERR_NOTDIR},
		{"lookupp from symlink", "link", "lookupp", nfs4.NFS4ERR_SYMLINK},
		{"commit directory", "dir", "commit", nfs4.NFS4ERR_ISDIR},
		{"commit symlink", "link", "commit", nfs4.NFS4ERR_INVAL},
		{"read directory", "dir", "read", nfs4.NFS4ERR_ISDIR},
		{"read symlink", "link", "read", nfs4.NFS4ERR_INVAL},
		{"write directory", "dir", "write", nfs4.NFS4ERR_ISDIR},
		{"write symlink", "link", "write", nfs4.NFS4ERR_INVAL},
		{"readdir file", "file", "readdir", nfs4.NFS4ERR_NOTDIR},
		{"readdir symlink", "link", "readdir", nfs4.NFS4ERR_SYMLINK},
		{"readlink file", "file", "readlink", nfs4.NFS4ERR_INVAL},
		{"readlink directory", "dir", "readlink", nfs4.NFS4ERR_INVAL},
		{"remove from file", "file", "remove", nfs4.NFS4ERR_NOTDIR},
		{"remove from symlink", "link", "remove", nfs4.NFS4ERR_SYMLINK},
		{"secinfo from file", "file", "secinfo", nfs4.NFS4ERR_NOTDIR},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, "dir"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("file", filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}

			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				lw := w.AppendArgarray_Lookup()
				nw := lw.StartObjname()
				buf := nw.SetData([]byte(test.object)).Finish()
				lw.Resume(buf)
				w.Resume(lw.Finish())

				switch test.operation {
				case "lookup":
					lw := w.AppendArgarray_Lookup()
					nw := lw.StartObjname()
					buf := nw.SetData([]byte("child")).Finish()
					lw.Resume(buf)
					w.Resume(lw.Finish())
				case "lookupp":
					w.AppendArgarray_Lookupp()
				case "commit":
					w.AppendArgarray_Commit()
				case "read":
					rw := w.AppendArgarray_Read()
					rw.SetCount(1)
				case "write":
					ww := w.AppendArgarray_Write()
					ww = ww.SetData(nil)
					w.Resume(ww.Finish())
				case "readdir":
					rw := w.AppendArgarray_Readdir()
					rw.SetDircount(1024)
					rw.SetMaxcount(1024)
					bw := rw.StartAttrRequest()
					buf := bw.Finish()
					rw.Resume(buf)
					w.Resume(rw.Finish())
				case "readlink":
					w.AppendArgarray_Readlink()
				case "remove":
					rw := w.AppendArgarray_Remove()
					tw := rw.StartTarget()
					buf := tw.SetData([]byte("child")).Finish()
					rw.Resume(buf)
					w.Resume(rw.Finish())
				case "secinfo":
					sw := w.AppendArgarray_Secinfo()
					nw := sw.StartName()
					buf := nw.SetData([]byte("child")).Finish()
					sw.Resume(buf)
					w.Resume(sw.Finish())
				default:
					t.Fatalf("unknown operation %q", test.operation)
				}
			})
			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
	}
}

func TestRenameDirectoryFilehandlesValidated(t *testing.T) {
	tests := []struct {
		name       string
		badSource  bool
		object     string
		wantStatus uint32
	}{
		{"file source", true, "file", nfs4.NFS4ERR_NOTDIR},
		{"symlink source", true, "link", nfs4.NFS4ERR_SYMLINK},
		{"file target", false, "file", nfs4.NFS4ERR_NOTDIR},
		{"symlink target", false, "link", nfs4.NFS4ERR_SYMLINK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("file", filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				if test.badSource {
					lw := w.AppendArgarray_Lookup()
					nw := lw.StartObjname()
					buf := nw.SetData([]byte(test.object)).Finish()
					lw.Resume(buf)
					w.Resume(lw.Finish())
					w.AppendArgarray_Savefh()
					w.AppendArgarray_Putrootfh()
				} else {
					w.AppendArgarray_Savefh()
					lw := w.AppendArgarray_Lookup()
					nw := lw.StartObjname()
					buf := nw.SetData([]byte(test.object)).Finish()
					lw.Resume(buf)
					w.Resume(lw.Finish())
				}

				rw := w.AppendArgarray_Rename()
				ow := rw.StartOldname()
				buf := ow.SetData([]byte("old")).Finish()
				rw.Resume(buf)
				nw := rw.StartNewname()
				buf = nw.SetData([]byte("new")).Finish()
				rw.Resume(buf)
				w.Resume(rw.Finish())
			})
			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
	}
}

func TestOpenFilehandleTypesValidated(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		target     string
		wantStatus uint32
	}{
		{"file current filehandle", "file", "child", nfs4.NFS4ERR_NOTDIR},
		{"symlink current filehandle", "link", "child", nfs4.NFS4ERR_SYMLINK},
		{"directory target", "", "dir", nfs4.NFS4ERR_ISDIR},
		{"symlink target", "", "link", nfs4.NFS4ERR_SYMLINK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, "dir"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("file", filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}

			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()
			xid := uint32(1)
			clientID := setupClient(t, conn, &xid)

			res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				if test.current != "" {
					lw := w.AppendArgarray_Lookup()
					nw := lw.StartObjname()
					buf := nw.SetData([]byte(test.current)).Finish()
					lw.Resume(buf)
					w.Resume(lw.Finish())
				}

				ow := w.AppendArgarray_Open()
				ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_READ)
				ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
				owner := ow.StartOwner()
				owner = owner.SetClientid(clientID)
				owner = owner.SetOwner([]byte("type-test"))
				buf := owner.Finish()
				ow.Resume(buf)
				ow.SetOpenhow_Default(nfs4.OPEN4_NOCREATE)
				claim := ow.SetClaim_Null()
				buf = claim.SetData([]byte(test.target)).Finish()
				ow.Resume(buf)
				w.Resume(ow.Finish())
			})
			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
	}
}

func TestNoFilehandle(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// GETATTR without setting a filehandle first.
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << nfs4.FATTR4_TYPE)
		buf := bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	if res.Status() != nfs4.NFS4ERR_NOFILEHANDLE {
		t.Fatalf("status = %d, want NFS4ERR_NOFILEHANDLE (%d)", res.Status(), nfs4.NFS4ERR_NOFILEHANDLE)
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("nonexistent")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		w.AppendArgarray_Getfh()
	})

	if res.Status() != nfs4.NFS4ERR_NOENT {
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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << nfs4.FATTR4_TYPE)
		buf := bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	getattrRes := entry.Value().AsGETATTR4resEntry()
	if getattrRes.Disc() != nfs4.NFS4_OK {
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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("sized.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << nfs4.FATTR4_SIZE)
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
		bw.AppendData(1 << (nfs4.FATTR4_MODE - 32))
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
	res = sendCompound(t, conn, 2, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(0)
		bw.AppendData(1 << (nfs4.FATTR4_MODE - 32))
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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("link")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << nfs4.FATTR4_TYPE)
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
		res := sendCompound(t, conn, uint32(i+1), func(w *nfs4.COMPOUND4argsWriter) {
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

	// The entire file exceeds the negotiated reply cache. This read-only
	// compound explicitly requests an uncached response.
	setupXID := uint32(1)
	setupClient(t, conn, &setupXID)
	ts := testSessionForConn(conn)
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
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
	slot, seq := ts.acquire()
	defer ts.release(slot)
	wire := admissionSequence(body, ts, slot, seq, false)
	reply, err := sendRawRPC(conn, setupXID, procCompound, wire)
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseCompoundReply(withoutSequenceResult(reply))
	if err != nil {
		t.Fatal(err)
	}

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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << nfs4.FATTR4_SUPPORTED_ATTRS)
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
	// supported_attrs is bitmap4: count(4) + 3 words(12) = 16 bytes.
	if len(attrData) < 16 {
		t.Fatalf("attr data too short for supported_attrs: %d bytes", len(attrData))
	}
	count := binary.BigEndian.Uint32(attrData[:4])
	if count != 3 {
		t.Fatalf("supported_attrs bitmap count = %d, want 3", count)
	}
	if word2 := binary.BigEndian.Uint32(attrData[12:16]); word2 != supportedAttrs2 {
		t.Fatalf("supported_attrs[2] = %#x, want %#x", word2, supportedAttrs2)
	}
	word0 := binary.BigEndian.Uint32(attrData[4:8])
	word1 := binary.BigEndian.Uint32(attrData[8:12])
	// SUPPORTED_ATTRS advertises readable + settable (write-only) attrs.
	wantWord0 := supportedAttrs0 | settableAttrs0
	wantWord1 := supportedAttrs1 | settableAttrs1
	if word0 != wantWord0 {
		t.Fatalf("supported_attrs[0] = %#x, want %#x", word0, wantWord0)
	}
	if word1 != wantWord1 {
		t.Fatalf("supported_attrs[1] = %#x, want %#x", word1, wantWord1)
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()

		lw := w.AppendArgarray_Lookup()
		nw := lw.StartObjname()
		buf := nw.SetData([]byte("multi.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1<<nfs4.FATTR4_TYPE | 1<<nfs4.FATTR4_SIZE | 1<<nfs4.FATTR4_CHANGE | 1<<nfs4.FATTR4_FILEID)
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

func TestGetattrAttributeMaskValidation(t *testing.T) {
	tests := []struct {
		name       string
		mask       attrMask
		wantStatus uint32
	}{
		{"write-only time_access_set", attrMask{0, 1 << (nfs4.FATTR4_TIME_ACCESS_SET - 32)}, nfs4.NFS4ERR_INVAL},
		{"rdattr_error", attrMask{1 << nfs4.FATTR4_RDATTR_ERROR, 0}, nfs4.NFS4_OK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				gw := w.AppendArgarray_Getattr()
				bw := gw.StartAttrRequest()
				bw.AppendData(test.mask[0])
				if test.mask[1] != 0 {
					bw.AppendData(test.mask[1])
				}
				buf := bw.Finish()
				gw.Resume(buf)
				w.Resume(gw.Finish())
			})

			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
	}
}

func TestVerifyAttributeMaskValidation(t *testing.T) {
	tests := []struct {
		name       string
		nverify    bool
		mask       attrMask
		values     []byte
		wantStatus uint32
	}{
		{"verify type", false, attrMask{1 << nfs4.FATTR4_TYPE, 0}, binary.BigEndian.AppendUint32(nil, nfs4.NF4DIR), nfs4.NFS4_OK},
		{"nverify type", true, attrMask{1 << nfs4.FATTR4_TYPE, 0}, binary.BigEndian.AppendUint32(nil, nfs4.NF4REG), nfs4.NFS4_OK},
		{"verify rdattr_error", false, attrMask{1 << nfs4.FATTR4_RDATTR_ERROR, 0}, make([]byte, 4), nfs4.NFS4ERR_INVAL},
		{"nverify rdattr_error", true, attrMask{1 << nfs4.FATTR4_RDATTR_ERROR, 0}, make([]byte, 4), nfs4.NFS4ERR_INVAL},
		{"verify unsupported acl", false, attrMask{1 << nfs4.FATTR4_ACL, 0}, make([]byte, 4), nfs4.NFS4ERR_ATTRNOTSUPP},
		{"nverify unsupported acl", true, attrMask{1 << nfs4.FATTR4_ACL, 0}, make([]byte, 4), nfs4.NFS4ERR_ATTRNOTSUPP},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()

				if test.nverify {
					vw := w.AppendArgarray_Nverify()
					faw := vw.StartObjAttributes()
					buf := finishTestFattr(faw, test.mask, test.values)
					vw.Resume(buf)
					w.Resume(vw.Finish())
				} else {
					vw := w.AppendArgarray_Verify()
					faw := vw.StartObjAttributes()
					buf := finishTestFattr(faw, test.mask, test.values)
					vw.Resume(buf)
					w.Resume(vw.Finish())
				}
			})

			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
	}
}

func finishTestFattr(w nfs4.Fattr4Writer, mask attrMask, values []byte) []byte {
	bw := w.StartAttrmask()
	bw.AppendData(mask[0])
	if mask[1] != 0 {
		bw.AppendData(mask[1])
	}
	buf := bw.Finish()
	w.Resume(buf)
	aw := w.StartAttrVals()
	buf = aw.SetData(values).Finish()
	w.Resume(buf)
	return w.Finish()
}

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	openStateid, fh := openCreateFile(
		t, conn, &xid, clientid, "newfile.txt",
	)

	// WRITE to the file.
	writeData := []byte("hello world from NFS write!")
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		ww := w.AppendArgarray_Write()
		setStateid(ww.Stateid(), openStateid)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(2) // FILE_SYNC4
		ww = ww.SetData(writeData)
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	entry := nextOp(t, &iter)
	writeRes := entry.Value().AsWRITE4resEntry()
	if writeRes.Disc() != nfs4.NFS4_OK {
		t.Fatalf("WRITE status = %d", writeRes.Disc())
	}
	writeOk := writeRes.Value().AsWRITE4resok()
	if writeOk.Count() != uint32(len(writeData)) {
		t.Fatalf("WRITE count = %d, want %d", writeOk.Count(), len(writeData))
	}
	if writeOk.Committed() != fileSync4 {
		t.Fatalf("WRITE committed = %d, want FILE_SYNC4", writeOk.Committed())
	}

	// The anonymous stateid is valid for WRITE as well as READ.
	anonymousData := []byte(" anonymous")
	res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())

		ww := w.AppendArgarray_Write()
		ww.Stateid().SetSeqid(0)
		ww = ww.SetOffset(uint64(len(writeData)))
		ww = ww.SetStable(2)
		ww = ww.SetData(anonymousData)
		w.Resume(ww.Finish())
	})
	xid++
	expectOK(t, res)
	writeData = append(writeData, anonymousData...)

	closeFile(t, conn, &xid, fh, openStateid)

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
	openStateid, fh := openCreateFile(
		t, conn, &xid, clientid, "staged.txt",
	)

	// Write some data using the open stateid.
	writeData := []byte("staged content here!")
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		ww := w.AppendArgarray_Write()
		setStateid(ww.Stateid(), openStateid)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(2)
		ww = ww.SetData(writeData)
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	// GETATTR should reflect the staged size, not the empty VFS file.
	res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)

		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << nfs4.FATTR4_SIZE)
		buf = bw.Finish()
		gw.Resume(buf)
		buf = gw.Finish()
		w.Resume(buf)
	})
	xid++

	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	entry := nextOp(t, &iter)
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
	res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
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
	if readRes.Disc() != nfs4.NFS4_OK {
		t.Fatalf("READ status = %s", nfs4.Nfsstat4Name(readRes.Disc()))
	}
	readData := readRes.Value().AsREAD4resok().Data()
	if string(readData) != string(writeData) {
		t.Fatalf("READ data = %q, want %q", readData, writeData)
	}

	closeFile(t, conn, &xid, fh, openStateid)
}

func TestOpenExistingForWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openWriteFile(
		t, conn, &xid, clientid, "existing.txt",
	)
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		t.Fatal(err)
	}
	fhID, ok := vfs.FHToInodeID(fh)
	if !ok {
		t.Fatalf("invalid filehandle %x", fh)
	}
	if want := vfs.MakeInodeID(vfs.InodeTypeFile, stat.Ino); fhID != want {
		t.Fatalf("write-open filehandle = %v, want published inode %v",
			fhID, want)
	}
	writeFileAt(t, conn, &xid, fh, stateid, 3, []byte("XYZ"))

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("published file changed before CLOSE: %q", data)
	}

	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(0)
		rw.SetOffset(0)
		rw.SetCount(1024)
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	readEntry := nextOp(t, &iter)
	readData := readEntry.Value().AsREAD4resEntry().
		Value().AsREAD4resok().Data()
	if string(readData) != "oriXYZal" {
		t.Fatalf("anonymous READ = %q, want staged data %q",
			readData, "oriXYZal")
	}

	res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())

		rw := w.AppendArgarray_Read()
		rw.Stateid().SetSeqid(binary.BigEndian.Uint32(stateid[0:4]))
		for i := 0; i < 12; i++ {
			rw.Stateid().SetOther(i, stateid[4+i])
		}
		rw.SetOffset(0)
		rw.SetCount(1024)
	})
	xid++
	iter = expectOK(t, res)
	nextOp(t, &iter)
	readEntry = nextOp(t, &iter)
	readData = readEntry.Value().AsREAD4resEntry().
		Value().AsREAD4resok().Data()
	if string(readData) != "oriXYZal" {
		t.Fatalf("staged READ = %q, want %q", readData, "oriXYZal")
	}

	closeFile(t, conn, &xid, fh, stateid)
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "oriXYZal" {
		t.Fatalf("published file = %q, want %q", data, "oriXYZal")
	}
}

func TestHydrationFailureDiscardsReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	local := vfs.NewLocalTernVFS(dir)
	failID, err := local.Lookup(local.RootID(), "existing.txt")
	if err != nil {
		t.Fatal(err)
	}
	fs := &readErrorVFS{LocalTernVFS: local, failID: failID}
	stagingStore, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, stagingStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openWriteFile(
		t, conn, &xid, clientid, "existing.txt",
	)
	writeFileAt(t, conn, &xid, fh, stateid, 0, []byte("changed"))
	if status := closeFileStatus(
		t, conn, &xid, fh, stateid); status != nfs4.NFS4ERR_IO {
		t.Fatalf("CLOSE status = %s, want NFS4ERR_IO",
			nfs4.Nfsstat4Name(status))
	}

	if staging.TargetBusy(stagingStore, fs.RootID(), "existing.txt") {
		t.Fatal("failed hydration left the publication target reserved")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("published file changed after hydration failure: %q", data)
	}
}

func TestConcurrentClosePublishesOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := &mutableBlockingLinkVFS{
		LocalTernVFS: vfs.NewLocalTernVFS(dir),
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
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openWriteFile(
		t, conn, &xid, clientid, "existing.txt",
	)
	writeFileAt(t, conn, &xid, fh, stateid, 0, []byte("changed"))

	// A retransmission on the same slot while CLOSE publishes is told to
	// wait; once CLOSE finishes it gets the cached reply.
	statuses := make(chan uint32, 1)
	go func() {
		xid := uint32(100)
		statuses <- closeFileStatus(t, conn, &xid, fh, stateid)
	}()
	<-fs.started
	closeConn := dial(t, addr)
	defer closeConn.Close()
	ts := testSessionForConn(conn)
	retransmit := func(xid uint32) uint32 {
		ts.mu.Lock()
		slot, seq := ts.lastSlot, ts.lastSeq
		ts.mu.Unlock()
		body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			pw.Resume(pw.StartObject().SetData(fh).Finish())
			w.Resume(pw.Finish())
			caw := w.AppendArgarray_Close()
			setStateid(caw.OpenStateid(), stateid)
		})
		reply, err := sendRawRPC(closeConn, xid, procCompound,
			withSequence(body, ts, slot, seq))
		if err != nil {
			t.Fatal(err)
		}
		res, err := parseCompoundReply(reply)
		if err != nil {
			t.Fatal(err)
		}
		return res.Status()
	}
	if status := retransmit(200); status != nfs4.NFS4ERR_DELAY {
		t.Fatalf("retransmitted CLOSE during publication = %s, want NFS4ERR_DELAY",
			nfs4.Nfsstat4Name(status))
	}
	close(fs.release)
	if status := <-statuses; status != nfs4.NFS4_OK {
		t.Fatalf("CLOSE status = %s, want NFS4_OK", nfs4.Nfsstat4Name(status))
	}
	if status := retransmit(201); status != nfs4.NFS4_OK {
		t.Fatalf("retransmitted CLOSE = %s, want cached NFS4_OK",
			nfs4.Nfsstat4Name(status))
	}
	if got := fs.linkCount(); got != 1 {
		t.Fatalf("LinkFile calls = %d, want 1", got)
	}
}

func TestBackgroundReadHasBoundedForegroundYield(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := vfs.NewLocalTernVFS(dir)
	id, err := fs.Lookup(fs.RootID(), "file")
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		fs:             fs,
		hydrationSlots: make(chan struct{}, 1),
	}
	srv.foregroundReads.Store(1)
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 4)
		n, _, err := srv.readBaseBackground(nil, id, 0, buf)
		if err == nil && (n != len(buf) || string(buf) != "data") {
			err = fmt.Errorf("read = (%d, %q), want (4, %q)",
				n, buf, "data")
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("background read starved behind sustained foreground reads")
	}
}

func TestBackgroundReadReleasesSlotAfterPanic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	local := vfs.NewLocalTernVFS(dir)
	id, err := local.Lookup(local.RootID(), "file")
	if err != nil {
		t.Fatal(err)
	}
	fs := &panicOnceReadVFS{LocalTernVFS: local, panic: true}
	srv := &Server{
		fs:             fs,
		hydrationSlots: make(chan struct{}, 1),
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("background read did not panic")
			}
		}()
		_, _, _ = srv.readBaseBackground(nil, id, 0, make([]byte, 4))
	}()

	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 4)
		n, _, err := srv.readBaseBackground(nil, id, 0, buf)
		if err == nil && (n != len(buf) || string(buf) != "data") {
			err = fmt.Errorf("read = (%d, %q), want (4, %q)",
				n, buf, "data")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("hydration slot leaked after read panic")
	}
}

func TestMutableGetattrReportsStagedSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "existing.txt"), []byte("original"), 0644,
	); err != nil {
		t.Fatal(err)
	}
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openWriteFile(
		t, conn, &xid, clientid, "existing.txt",
	)
	writeFileAt(t, conn, &xid, fh, stateid, 8, []byte("XYZ"))

	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())
		gw := w.AppendArgarray_Getattr()
		bw := gw.StartAttrRequest()
		bw.AppendData(1 << nfs4.FATTR4_SIZE)
		buf = bw.Finish()
		gw.Resume(buf)
		w.Resume(gw.Finish())
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	getattr := nextOp(t, &iter).Value().AsGETATTR4resEntry().
		Value().AsGETATTR4resok()
	data := getAttrData(t, getattr)
	if len(data) < 8 {
		t.Fatal("GETATTR size data is truncated")
	}
	if size := binary.BigEndian.Uint64(data[:8]); size != 11 {
		t.Fatalf("GETATTR size = %d, want staged size 11", size)
	}
	closeFile(t, conn, &xid, fh, stateid)
}

func TestReadCloseIgnoresOtherWriterStaging(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "existing.txt"), []byte("original"), 0644,
	); err != nil {
		t.Fatal(err)
	}
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	readerClient := setupClient(t, conn, &xid)
	writerClient, confirm := requestClientID(
		t, conn, &xid, "writer-client", [8]byte{},
	)
	if status := confirmClientID(
		t, conn, &xid, writerClient, confirm,
	); status != nfs4.NFS4_OK {
		t.Fatal("writer CREATE_SESSION failed")
	}
	readerState, readerFH := openReadFile(
		t, conn, &xid, readerClient, "existing.txt",
	)
	writerState, writerFH := openWriteFile(
		t, conn, &xid, writerClient, "existing.txt",
	)
	writeFileAt(
		t, conn, &xid, writerFH, writerState, 0, []byte("changed"),
	)

	if status := closeFileStatus(
		t, conn, &xid, readerFH, readerState); status != nfs4.NFS4_OK {
		t.Fatalf("reader CLOSE status = %s, want NFS4_OK",
			nfs4.Nfsstat4Name(status))
	}
	closeFile(t, conn, &xid, writerFH, writerState)
}

func TestClientRebootDoesNotDiscardOtherWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := vfs.NewLocalTernVFS(dir)
	stagingStore, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, stagingStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	readerClient := setupClient(t, conn, &xid)
	writerClient, confirm := requestClientID(
		t, conn, &xid, "writer-client", [8]byte{},
	)
	if status := confirmClientID(
		t, conn, &xid, writerClient, confirm,
	); status != nfs4.NFS4_OK {
		t.Fatal("writer CREATE_SESSION failed")
	}
	_, _ = openReadFile(
		t, conn, &xid, readerClient, "existing.txt",
	)
	writerState, writerFH := openWriteFile(
		t, conn, &xid, writerClient, "existing.txt",
	)
	writeFileAt(
		t, conn, &xid, writerFH, writerState, 0, []byte("changed"),
	)

	rebootedClient, rebootConfirm := requestClientID(
		t, conn, &xid, "test-client", [8]byte{1},
	)
	if status := confirmClientID(
		t, conn, &xid, rebootedClient, rebootConfirm,
	); status != nfs4.NFS4_OK {
		t.Fatalf("reader reboot confirmation = %s", nfs4.Nfsstat4Name(status))
	}
	if !staging.TargetBusy(stagingStore, fs.RootID(), "existing.txt") {
		t.Fatal("reader reboot discarded another client's staging")
	}
	closeFile(t, conn, &xid, writerFH, writerState)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "changedl" {
		t.Fatalf("published data = %q, want %q", data, "changedl")
	}
}

func TestOpenExistingForWriteWithoutChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openWriteFile(
		t, conn, &xid, clientid, "existing.txt",
	)
	closeFile(t, conn, &xid, fh, stateid)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("file changed after no-op write open: %q", data)
	}
	if !os.SameFile(before, after) {
		t.Fatal("no-op write open replaced the published file")
	}
}

func TestExpiredMutableOpenReleasesTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := vfs.NewLocalTernVFS(dir)
	stagingStore, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, stagingStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	_, _ = openWriteFile(
		t, conn, &xid, clientid, "existing.txt",
	)
	srv.waitForClientGC()
	now := time.Now().Add(2 * clientstore.LeaseTime)
	srv.clients.SetClock(func() time.Time { return now })

	unlock := srv.lockMutationTargets(mutationTarget{
		dirID: fs.RootID(),
		name:  "existing.txt",
	})
	busy, err := srv.stagingTargetBusy(
		fs.RootID(), "existing.txt",
	)
	unlock()
	if err != nil {
		t.Fatal(err)
	}
	if busy || staging.TargetBusy(stagingStore, fs.RootID(), "existing.txt") {
		t.Fatal("expired mutable open still reserves its target")
	}
}

func TestRecoveredMutableOpenCanBeReplayedByClient(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "existing.txt"), []byte("original"), 0644,
	); err != nil {
		t.Fatal(err)
	}
	fs := vfs.NewLocalTernVFS(dir)
	stagingDir := t.TempDir()
	staging1, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv1, err := NewServer(fs, staging1, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr1, cleanup1 := serveTestServer(t, srv1)
	conn1 := dial(t, addr1)
	xid := uint32(1)
	clientid := setupClient(t, conn1, &xid)
	originalState, originalFH := openWriteFile(
		t, conn1, &xid, clientid, "existing.txt",
	)
	writeFileAt(
		t, conn1, &xid, originalFH, originalState, 0, []byte("changed"),
	)
	conn1.Close()
	cleanup1()

	staging2, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkedStaging := &markerCheckingStagingStore{
		Store: staging2,
	}
	srv2, err := NewServer(fs, checkedStaging, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkedStaging.clients = srv2.clients
	if !staging.TargetBusy(staging2, fs.RootID(), "existing.txt") {
		t.Fatal("live recovered staging was discarded during startup")
	}
	addr2, cleanup2 := serveTestServer(t, srv2)
	defer cleanup2()
	conn2 := dial(t, addr2)
	defer conn2.Close()

	replayedState, replayedFH := openWriteFile(
		t, conn2, &xid, clientid, "existing.txt",
	)
	closeFile(t, conn2, &xid, replayedFH, replayedState)
	if !checkedStaging.checked {
		t.Fatal("recovered staging was not rebound")
	}
	data, err := os.ReadFile(filepath.Join(dir, "existing.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "changedl" {
		t.Fatalf("recovered publication = %q, want %q", data, "changedl")
	}
}

func TestRecoveredNewFileCanBeReopenedWithoutCreate(t *testing.T) {
	dir := t.TempDir()
	fs := vfs.NewLocalTernVFS(dir)
	stagingDir := t.TempDir()
	staging1, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv1, err := NewServer(fs, staging1, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr1, cleanup1 := serveTestServer(t, srv1)
	conn1 := dial(t, addr1)
	xid := uint32(1)
	clientid := setupClient(t, conn1, &xid)
	originalState, originalFH := openCreateFile(
		t, conn1, &xid, clientid, "new.txt",
	)
	writeFileAt(
		t, conn1, &xid, originalFH, originalState, 0, []byte("contents"),
	)
	conn1.Close()
	cleanup1()

	staging2, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv2, err := NewServer(fs, staging2, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr2, cleanup2 := serveTestServer(t, srv2)
	defer cleanup2()
	conn2 := dial(t, addr2)
	defer conn2.Close()

	replayedState, replayedFH := openWriteFile(
		t, conn2, &xid, clientid, "new.txt",
	)
	closeFile(t, conn2, &xid, replayedFH, replayedState)
	data, err := os.ReadFile(filepath.Join(dir, "new.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "contents" {
		t.Fatalf("recovered publication = %q, want %q", data, "contents")
	}
}

func TestRecoveredMutableOpenKeepsOriginalBase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := vfs.NewLocalTernVFS(dir)
	stagingDir := t.TempDir()
	staging1, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv1, err := NewServer(fs, staging1, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr1, cleanup1 := serveTestServer(t, srv1)
	conn1 := dial(t, addr1)
	xid := uint32(1)
	clientid := setupClient(t, conn1, &xid)
	stateid, fh := openWriteFile(
		t, conn1, &xid, clientid, "existing.txt",
	)
	writeFileAt(t, conn1, &xid, fh, stateid, 0, []byte("changed"))
	_, oldMeta, ok := staging.FindTarget(staging1, fs.RootID(), "existing.txt")
	if !ok {
		t.Fatal("staging metadata not found")
	}
	conn1.Close()
	cleanup1()

	id, cookie, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.LinkFile(id, cookie, fs.RootID(), "existing.txt", strings.NewReader("new base")); err != nil {
		t.Fatal(err)
	}
	currentID, err := fs.Lookup(fs.RootID(), "existing.txt")
	if err != nil {
		t.Fatal(err)
	}
	if currentID == oldMeta.BaseID {
		t.Fatal("test replacement reused the original inode")
	}
	staging2, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv2, err := NewServer(fs, staging2, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr2, cleanup2 := serveTestServer(t, srv2)
	defer cleanup2()
	conn2 := dial(t, addr2)
	defer conn2.Close()

	recoveredState, recoveredFH := openWriteFile(t, conn2, &xid, clientid, "existing.txt")
	// Reopening by name returns the handle the name has now; the recovered
	// staging keeps its own base.
	if !bytes.Equal(recoveredFH, vfs.InodeIDToFH(currentID)) {
		t.Fatal("recovered OPEN did not return the current published handle")
	}
	closeFile(t, conn2, &xid, recoveredFH, recoveredState)
	if staging.TargetBusy(staging2, fs.RootID(), "existing.txt") {
		t.Fatal("recovered publication left staging registered")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "changedl" {
		t.Fatalf("recovered publication = %q, want changedl", data)
	}
}

func TestRecoveredMissingBaseCreatesFreshFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := vfs.NewLocalTernVFS(dir)
	stagingDir := t.TempDir()
	staging1, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv1, err := NewServer(fs, staging1, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr1, cleanup1 := serveTestServer(t, srv1)
	conn1 := dial(t, addr1)
	xid := uint32(1)
	clientid := setupClient(t, conn1, &xid)
	stateid, fh := openWriteFile(
		t, conn1, &xid, clientid, "existing.txt",
	)
	writeFileAt(t, conn1, &xid, fh, stateid, 0, []byte("changed"))
	conn1.Close()
	cleanup1()

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	staging2, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv2, err := NewServer(fs, staging2, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr2, cleanup2 := serveTestServer(t, srv2)
	defer cleanup2()
	conn2 := dial(t, addr2)
	defer conn2.Close()

	oldStagingID, _, ok := staging.FindTarget(staging2,
		fs.RootID(), "existing.txt",
	)
	if !ok {
		t.Fatal("recovered staging target not found")
	}
	newState, newFH := openCreateFile(
		t, conn2, &xid, clientid, "existing.txt",
	)
	newID := stagingIDForTest(t, staging2, newState)
	newMeta, found := staging2.GetMeta(newID)
	if !found || newMeta.BaseID == 0 || newMeta.BaseSize != 0 || newMeta.Size != 0 {
		t.Fatal("fresh create did not allocate staging over a new empty version")
	}
	// The local backend's filesystem can immediately reuse the scrapped
	// transient's inode number; its replacement session must still be fresh.
	if oldStagingID != newID && staging2.Get(oldStagingID) != nil {
		t.Fatal("fresh create retained stale replacement staging")
	}
	writeFileAt(t, conn2, &xid, newFH, newState, 0, []byte("fresh"))
	closeFile(t, conn2, &xid, newFH, newState)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fresh" {
		t.Fatalf("fresh create data = %q, want %q", data, "fresh")
	}
}

func TestMutableOpenPreservesMetadataOnlySetattr(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openWriteFile(
		t, conn, &xid, clientid, "existing.txt",
	)
	mtime := time.Unix(1_600_000_000, 0)
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())

		saw := w.AppendArgarray_Setattr()
		saw.Stateid().SetSeqid(binary.BigEndian.Uint32(stateid[0:4]))
		for i := 0; i < 12; i++ {
			saw.Stateid().SetOther(i, stateid[4+i])
		}
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(0)
		bmW.AppendData(1 << (nfs4.FATTR4_TIME_MODIFY_SET - 32))
		buf = bmW.Finish()
		faw.Resume(buf)
		attrData := make([]byte, 16)
		binary.BigEndian.PutUint32(attrData[0:4], nfs4.SET_TO_CLIENT_TIME4)
		binary.BigEndian.PutUint64(attrData[4:12], uint64(mtime.Unix()))
		buf = faw.StartAttrVals().SetData(attrData).Finish()
		faw.Resume(buf)
		saw.Resume(faw.Finish())
		w.Resume(saw.Finish())
	})
	xid++
	expectOK(t, res)
	closeFile(t, conn, &xid, fh, stateid)

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("metadata-only update replaced file contents")
	}
	if !after.ModTime().Equal(mtime) {
		t.Fatalf("mtime = %v, want %v", after.ModTime(), mtime)
	}
}

func TestMutableOpenBlocksDirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "existing.txt"), []byte("original"), 0644,
	); err != nil {
		t.Fatal(err)
	}

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openWriteFile(
		t, conn, &xid, clientid, "existing.txt",
	)

	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		cw := w.AppendArgarray_Create()
		cw.SetObjtype_Nf4dir()
		buf := cw.StartObjname().SetData([]byte("existing.txt")).Finish()
		cw.Resume(buf)
		faw := cw.StartCreateattrs()
		bmW := faw.StartAttrmask()
		buf = bmW.Finish()
		faw.Resume(buf)
		buf = faw.StartAttrVals().SetData(nil).Finish()
		faw.Resume(buf)
		cw.Resume(faw.Finish())
		w.Resume(cw.Finish())
	})
	xid++
	if res.Status() != nfs4.NFS4ERR_FILE_OPEN {
		t.Fatalf("CREATE status = %s, want NFS4ERR_FILE_OPEN",
			nfs4.Nfsstat4Name(res.Status()))
	}

	closeFile(t, conn, &xid, fh, stateid)
}

func TestTruncateExistingFileThenRegrow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("abcdefghij"), 0644); err != nil {
		t.Fatal(err)
	}

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openWriteFile(
		t, conn, &xid, clientid, "existing.txt",
	)
	setFileSize(t, conn, &xid, fh, stateid, 5)
	setFileSize(t, conn, &xid, fh, stateid, 8)
	closeFile(t, conn, &xid, fh, stateid)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{'a', 'b', 'c', 'd', 'e', 0, 0, 0}
	if !reflect.DeepEqual(data, want) {
		t.Fatalf("file data = %v, want %v", data, want)
	}
}

func TestOpenUncheckedTruncatesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("old contents"), 0644); err != nil {
		t.Fatal(err)
	}

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	size := uint64(0)
	stateid, fh := openCreateFileWithSize(
		t, conn, &xid, clientid, "existing.txt", &size,
	)
	closeFile(t, conn, &xid, fh, stateid)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("file size = %d, want 0", info.Size())
	}
}

func TestCreateDirectory(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	// PUTROOTFH + CREATE(NF4DIR, "subdir")
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	if createRes.Disc() != nfs4.NFS4_OK {
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	if removeRes.Disc() != nfs4.NFS4_OK {
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	if renameRes.Disc() != nfs4.NFS4_OK {
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

func TestRenameToSameNameIsNoOp(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "same"), 0755); err != nil {
		t.Fatal(err)
	}
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Savefh()
		rw := w.AppendArgarray_Rename()
		ow := rw.StartOldname()
		buf := ow.SetData([]byte("same")).Finish()
		rw.Resume(buf)
		nw := rw.StartNewname()
		buf = nw.SetData([]byte("same")).Finish()
		rw.Resume(buf)
		w.Resume(rw.Finish())
	})
	iter := expectOK(t, res)
	nextOp(t, &iter)
	nextOp(t, &iter)
	ok := nextOp(t, &iter).Value().AsRENAME4resEntry().
		Value().AsRENAME4resok()
	if got := ok.SourceCinfo(); got.Before() != got.After() {
		t.Errorf("source cinfo changed: before=%d after=%d",
			got.Before(), got.After())
	}
	if got := ok.TargetCinfo(); got.Before() != got.After() {
		t.Errorf("target cinfo changed: before=%d after=%d",
			got.Before(), got.After())
	}
}

func TestDelegpurgeNotSupported(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		dw := w.AppendArgarray_Delegpurge()
		dw.SetClientid(1)
	})

	if res.Status() != nfs4.NFS4ERR_NOTSUPP {
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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Savefh()

		lw := w.AppendArgarray_Link()
		nw := lw.StartNewname()
		buf := nw.SetData([]byte("link.txt")).Finish()
		lw.Resume(buf)
		buf = lw.Finish()
		w.Resume(buf)
	})

	if res.Status() != nfs4.NFS4ERR_NOTSUPP {
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
	if createRes.Disc() != nfs4.NFS4_OK {
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

	// NFSv4.0 and NFSv4.2 are both rejected by default.
	for _, minor := range []uint32{0, 2} {
		testMinorVersionMismatch(t, conn, minor)
	}
}

func TestMinorVersion2WhenAllowed(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	srv.AllowMinorVersion2 = true
	conn := dial(t, addr)
	defer conn.Close()
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
	})
	binary.BigEndian.PutUint32(body[4:8], 2)
	res, err := parseCompoundReply(must(sendRawRPC(conn, 1, procCompound, body)))
	if err != nil {
		t.Fatal(err)
	}
	// Accepted as a minor version, so the missing SEQUENCE is what fails.
	if res.Status() != nfs4.NFS4ERR_OP_NOT_IN_SESSION {
		t.Fatalf("minor version 2 = %s", nfs4.Nfsstat4Name(res.Status()))
	}
	testMinorVersionMismatch(t, conn, 0)
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func testMinorVersionMismatch(t *testing.T, conn net.Conn, minor uint32) {
	t.Helper()
	var body []byte
	w := nfs4.StartCOMPOUND4args(body)
	tagW := w.StartTag()
	body = tagW.SetData(nil).Finish()
	w.Resume(body)
	w.SetMinorversion(minor)
	w.AppendArgarray_Putrootfh()
	body = w.Finish()

	reply, err := sendRPC(conn, 1, procCompound, body)
	if err != nil {
		t.Fatal(err)
	}
	nfsBody := mustParseRPCReply(t, reply)
	res, ok := nfs4.ReadCOMPOUND4res(nfsBody)
	if !ok {
		t.Fatal("failed to parse COMPOUND4res")
	}
	if res.Status() != nfs4.NFS4ERR_MINOR_VERS_MISMATCH {
		t.Fatalf("status = %s, want NFS4ERR_MINOR_VERS_MISMATCH",
			nfs4.Nfsstat4Name(res.Status()))
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
	openStateid, fh := openCreateFile(
		t, conn, &xid, clientid, "empty.txt",
	)
	closeFile(t, conn, &xid, fh, openStateid)

	// Verify the empty file was created on disk.
	data, err := os.ReadFile(filepath.Join(dir, "empty.txt"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("file content = %q, want empty", data)
	}
}

// activeOpenMarkerCount counts the o.* markers of the incarnation behind a
// wire clientid.
func activeOpenMarkerCount(
	t *testing.T,
	srv *Server,
	wireID uint64,
) int {
	t.Helper()
	incarnation := vfs.InodeID(incarnationForTest(t, srv, wireID))
	count := 0
	var cursor uint64
	for {
		entries, next, err := srv.fs.Readdir(incarnation, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if clientstore.IsActiveOpenName(entry.Name) {
				count++
			}
		}
		if next == 0 {
			return count
		}
		if next == cursor {
			t.Fatal("marker listing cursor did not advance")
		}
		cursor = next
	}
}

func setStateid(dst nfs4.Stateid4, stateid [16]byte) {
	dst.SetSeqid(binary.BigEndian.Uint32(stateid[:4]))
	for i := range 12 {
		dst.SetOther(i, stateid[4+i])
	}
}

func openFileForOwner(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	clientID uint64,
	owner string,
	filename string,
	access uint32,
	create bool,
	guarded bool,
) (status uint32, stateid [16]byte, fh []byte, rflags uint32) {
	t.Helper()
	status, stateid, fh, rflags, err := openFileForOwnerE(
		conn, xid, clientID, owner, filename, access, create, guarded)
	if err != nil {
		t.Fatal(err)
	}
	return status, stateid, fh, rflags
}

func openFileForOwnerE(
	conn net.Conn,
	xid *uint32,
	clientID uint64,
	owner string,
	filename string,
	access uint32,
	create bool,
	guarded bool,
) (status uint32, stateid [16]byte, fh []byte, rflags uint32, err error) {
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(access)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientID)
		ownerW = ownerW.SetOwner([]byte(owner))
		buf := ownerW.Finish()
		ow.Resume(buf)
		if create {
			chw := ow.SetOpenhow_Create()
			var faw nfs4.Fattr4Writer
			if guarded {
				faw = chw.SetValue_Guarded4()
			} else {
				faw = chw.SetValue_Unchecked4()
			}
			bmW := faw.StartAttrmask()
			buf = bmW.Finish()
			faw.Resume(buf)
			alW := faw.StartAttrVals()
			buf = alW.SetData(nil).Finish()
			faw.Resume(buf)
			chw.Resume(faw.Finish())
			ow.Resume(chw.Finish())
		} else {
			ow.SetOpenhow_Default(nfs4.OPEN4_NOCREATE)
		}
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte(filename)).Finish()
		ow.Resume(buf)
		w.Resume(ow.Finish())
		w.AppendArgarray_Getfh()
	})
	reply, err := sendRPC(conn, *xid, procCompound, body)
	*xid++
	if err != nil {
		return 0, stateid, nil, 0, err
	}
	nfsBody, err := parseRPCReply(reply)
	if err != nil {
		return 0, stateid, nil, 0, err
	}
	res, ok := nfs4.ReadCOMPOUND4res(nfsBody)
	if !ok {
		return 0, stateid, nil, 0,
			errors.New("failed to parse COMPOUND4res")
	}
	if res.Status() != nfs4.NFS4_OK {
		return res.Status(), stateid, nil, 0, nil
	}
	iter := res.Resarray()
	if !iter.Next() || !iter.Next() {
		return 0, stateid, nil, 0, errors.New(
			"OPEN response is missing PUTROOTFH or OPEN")
	}
	openEntry := iter.Resarray()
	openOK := openEntry.Value().AsOPEN4resEntry().Value().AsOPEN4resok()
	sid := openOK.Stateid()
	binary.BigEndian.PutUint32(stateid[:4], sid.Seqid())
	for i := 0; i < 12; i++ {
		stateid[4+i] = sid.Other(i)
	}
	if !iter.Next() {
		return 0, stateid, nil, 0, errors.New(
			"OPEN response is missing GETFH")
	}
	fhEntry := iter.Resarray()
	fh = append([]byte(nil), fhEntry.Value().AsGETFH4resEntry().
		Value().AsGETFH4resok().Object().Data()...)
	return nfs4.NFS4_OK, stateid, fh, openOK.Rflags(), nil
}

// openReadFile opens a file for read and returns the open stateid and filehandle.
func openReadFile(t *testing.T, conn net.Conn, xid *uint32, clientid uint64, filename string) (stateid [16]byte, fh []byte) {
	t.Helper()
	status, stateid, fh, _ := openFileForOwner(
		t, conn, xid, clientid, "test-owner-"+filename, filename,
		nfs4.OPEN4_SHARE_ACCESS_READ, false, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("OPEN status = %s", nfs4.Nfsstat4Name(status))
	}
	return stateid, fh
}

// openWriteFile opens an existing file for write and returns its replacement
// stateid and filehandle.
func openWriteFile(t *testing.T, conn net.Conn, xid *uint32, clientid uint64, filename string) (stateid [16]byte, fh []byte) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner-" + filename))
		buf := ownerW.Finish()
		ow.Resume(buf)
		ow.SetOpenhow_Default(nfs4.OPEN4_NOCREATE)
		cw := ow.SetClaim_Null()
		buf = cw.SetData([]byte(filename)).Finish()
		ow.Resume(buf)
		w.Resume(ow.Finish())
		w.AppendArgarray_Getfh()
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	entry := nextOp(t, &iter)
	openRes := entry.Value().AsOPEN4resEntry()
	if openRes.Disc() != nfs4.NFS4_OK {
		t.Fatalf("OPEN for write status = %s", nfs4.Nfsstat4Name(openRes.Disc()))
	}
	sid := openRes.Value().AsOPEN4resok().Stateid()
	binary.BigEndian.PutUint32(stateid[0:4], sid.Seqid())
	for i := 0; i < 12; i++ {
		stateid[4+i] = sid.Other(i)
	}
	entry = nextOp(t, &iter)
	fh = append([]byte(nil), entry.Value().AsGETFH4resEntry().
		Value().AsGETFH4resok().Object().Data()...)
	return stateid, fh
}

// openCreateFile opens a new file for write and returns the open stateid and filehandle.
func openCreateFile(t *testing.T, conn net.Conn, xid *uint32, clientid uint64, filename string) (stateid [16]byte, fh []byte) {
	return openCreateFileWithSize(
		t, conn, xid, clientid, filename, nil,
	)
}

func openCreateFileWithSize(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	clientid uint64,
	filename string,
	size *uint64,
) (stateid [16]byte, fh []byte) {
	return openCreateFileWithSizeAndAccess(
		t, conn, xid, clientid, filename, size, nfs4.OPEN4_SHARE_ACCESS_BOTH,
	)
}

func openCreateFileWithSizeAndAccess(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	clientid uint64,
	filename string,
	size *uint64,
	access uint32,
) (stateid [16]byte, fh []byte) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(access)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner-" + filename))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		faw := chw.SetValue_Unchecked4()
		bmW := faw.StartAttrmask()
		if size != nil {
			bmW.AppendData(1 << nfs4.FATTR4_SIZE)
		}
		buf = bmW.Finish()
		faw.Resume(buf)
		alW := faw.StartAttrVals()
		var attrData []byte
		if size != nil {
			attrData = make([]byte, 8)
			binary.BigEndian.PutUint64(attrData, *size)
		}
		buf = alW.SetData(attrData).Finish()
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
		if size != nil {
			gw := w.AppendArgarray_Getattr()
			bw := gw.StartAttrRequest()
			bw.AppendData(1 << nfs4.FATTR4_SIZE)
			buf = bw.Finish()
			gw.Resume(buf)
			w.Resume(gw.Finish())
		}
		w.AppendArgarray_Getfh()
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	entry := nextOp(t, &iter)
	openRes := entry.Value().AsOPEN4resEntry()
	if openRes.Disc() != nfs4.NFS4_OK {
		t.Fatalf("OPEN CREATE status = %s", nfs4.Nfsstat4Name(openRes.Disc()))
	}
	openOk := openRes.Value().AsOPEN4resok()
	sid := openOk.Stateid()
	binary.BigEndian.PutUint32(stateid[0:4], sid.Seqid())
	for i := 0; i < 12; i++ {
		stateid[4+i] = sid.Other(i)
	}
	if size != nil {
		entry = nextOp(t, &iter)
		getattr := entry.Value().AsGETATTR4resEntry().
			Value().AsGETATTR4resok()
		data := getAttrData(t, getattr)
		if len(data) < 8 {
			t.Fatal("OPEN GETATTR size data is truncated")
		}
		if got := binary.BigEndian.Uint64(data[:8]); got != *size {
			t.Fatalf("OPEN GETATTR size = %d, want %d", got, *size)
		}
	}
	entry = nextOp(t, &iter)
	fh = append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)
	return stateid, fh
}

func writeFileAt(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	fh []byte,
	stateid [16]byte,
	offset uint64,
	data []byte,
) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())

		ww := w.AppendArgarray_Write()
		ww.Stateid().SetSeqid(binary.BigEndian.Uint32(stateid[0:4]))
		for i := 0; i < 12; i++ {
			ww.Stateid().SetOther(i, stateid[4+i])
		}
		ww = ww.SetOffset(offset)
		ww = ww.SetStable(fileSync4)
		ww = ww.SetData(data)
		w.Resume(ww.Finish())
	})
	*xid++
	expectOK(t, res)
}

func setFileSize(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	fh []byte,
	stateid [16]byte,
	size uint64,
) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())

		saw := w.AppendArgarray_Setattr()
		saw.Stateid().SetSeqid(binary.BigEndian.Uint32(stateid[0:4]))
		for i := 0; i < 12; i++ {
			saw.Stateid().SetOther(i, stateid[4+i])
		}
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(1 << nfs4.FATTR4_SIZE)
		buf = bmW.Finish()
		faw.Resume(buf)
		attrData := make([]byte, 8)
		binary.BigEndian.PutUint64(attrData, size)
		buf = faw.StartAttrVals().SetData(attrData).Finish()
		faw.Resume(buf)
		saw.Resume(faw.Finish())
		w.Resume(saw.Finish())
	})
	*xid++
	expectOK(t, res)
}

// closeFile sends CLOSE for the given filehandle and stateid.
func closeFile(t *testing.T, conn net.Conn, xid *uint32, fh []byte, stateid [16]byte) {
	t.Helper()
	status, _ := closeFileResult(t, conn, xid, fh, stateid)
	if status != nfs4.NFS4_OK {
		t.Fatalf("CLOSE status = %s", nfs4.Nfsstat4Name(status))
	}
}

func closeFileResult(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	fh []byte,
	stateid [16]byte,
) (uint32, [16]byte) {
	t.Helper()
	status, result, err := closeFileResultE(
		conn, xid, fh, stateid)
	if err != nil {
		t.Fatal(err)
	}
	return status, result
}

func closeFileStatus(
	t *testing.T,
	conn net.Conn,
	xid *uint32,
	fh []byte,
	stateid [16]byte,
) uint32 {
	t.Helper()
	status, _ := closeFileResult(t, conn, xid, fh, stateid)
	return status
}

func closeFileResultE(
	conn net.Conn,
	xid *uint32,
	fh []byte,
	stateid [16]byte,
) (uint32, [16]byte, error) {
	var result [16]byte
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		caw := w.AppendArgarray_Close()
		setStateid(caw.OpenStateid(), stateid)
	})
	reply, err := sendRPC(conn, *xid, procCompound, body)
	*xid++
	if err != nil {
		return 0, result, err
	}
	nfsBody, err := parseRPCReply(reply)
	if err != nil {
		return 0, result, err
	}
	res, ok := nfs4.ReadCOMPOUND4res(nfsBody)
	if !ok {
		return 0, result, errors.New("failed to parse COMPOUND4res")
	}
	if res.Status() == nfs4.NFS4_OK {
		iter := res.Resarray()
		if !iter.Next() || !iter.Next() {
			return 0, result, errors.New(
				"CLOSE response is missing PUTFH or CLOSE")
		}
		sid := iter.Resarray().Value().AsCLOSE4resEntry().
			Value().AsStateid4()
		binary.BigEndian.PutUint32(result[:4], sid.Seqid())
		for i := range 12 {
			result[4+i] = sid.Other(i)
		}
	}
	return res.Status(), result, nil
}

// collectReaddirNames issues READDIR calls to collect all names in a directory.
func collectReaddirNames(t *testing.T, conn net.Conn, xid *uint32, dirFH []byte) []string {
	t.Helper()
	var allNames []string
	cookie := uint64(0)
	var cookieVerf [8]byte
	for {
		res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
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
			bmW.AppendData(1 << nfs4.FATTR4_TYPE)
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
		if readdirRes.Disc() != nfs4.NFS4_OK {
			t.Fatalf("READDIR status = %s", nfs4.Nfsstat4Name(readdirRes.Disc()))
		}
		okRes := readdirRes.Value().AsREADDIR4resok()
		// Save cookieverf for next request.
		cv := okRes.Cookieverf()
		for i := 0; i < 8; i++ {
			cookieVerf[i] = cv.Data(i)
		}
		reply := okRes.Reply()
		if reply.EntriesPresent() == nfs4.TRUE {
			entOpt := reply.Entries()
			for {
				ent := entOpt.AsEntry4()
				allNames = append(allNames, string(ent.Name().Data()))
				cookie = ent.Cookie()
				if ent.NextentryPresent() != nfs4.TRUE {
					break
				}
				entOpt = ent.Nextentry()
			}
		}
		if reply.Eof() == nfs4.TRUE {
			break
		}
	}
	return allNames
}

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
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
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
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rdw := w.AppendArgarray_Readdir()
		rdw.SetCookie(0)
		rdw.SetDircount(4096)
		rdw.SetMaxcount(8192)
		bmW := rdw.StartAttrRequest()
		bmW.AppendData(1 << nfs4.FATTR4_TYPE)
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
	res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
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
		bmW.AppendData(1 << nfs4.FATTR4_TYPE)
		buf := bmW.Finish()
		rdw.Resume(buf)
		buf = rdw.Finish()
		w.Resume(buf)
	})
	xid++

	// Compound should report the READDIR error.
	if res.Status() != nfs4.NFS4ERR_NOT_SAME {
		t.Fatalf("expected NFS4ERR_NOT_SAME, got %s", nfs4.Nfsstat4Name(res.Status()))
	}
}

func TestReaddirReservedCookies(t *testing.T) {
	for _, cookie := range []uint64{1, 2} {
		t.Run(fmt.Sprintf("cookie_%d", cookie), func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				rw := w.AppendArgarray_Readdir()
				rw.SetCookie(cookie)
				rw.SetDircount(4096)
				rw.SetMaxcount(8192)
				bw := rw.StartAttrRequest()
				buf := bw.Finish()
				rw.Resume(buf)
				w.Resume(rw.Finish())
			})
			if res.Status() != nfs4.NFS4ERR_BAD_COOKIE {
				t.Fatalf("status = %s, want NFS4ERR_BAD_COOKIE",
					nfs4.Nfsstat4Name(res.Status()))
			}
		})
	}
}

func TestReaddirWriteOnlyAttributesRejected(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rw := w.AppendArgarray_Readdir()
		rw.SetDircount(4096)
		rw.SetMaxcount(8192)
		bw := rw.StartAttrRequest()
		bw.AppendData(0)
		bw.AppendData(1 << (nfs4.FATTR4_TIME_ACCESS_SET - 32))
		buf := bw.Finish()
		rw.Resume(buf)
		w.Resume(rw.Finish())
	})
	if res.Status() != nfs4.NFS4ERR_INVAL {
		t.Fatalf("status = %s, want NFS4ERR_INVAL",
			nfs4.Nfsstat4Name(res.Status()))
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rdw := w.AppendArgarray_Readdir()
		rdw.SetCookie(0)
		rdw.SetDircount(4096)
		rdw.SetMaxcount(10) // Way too small.
		bmW := rdw.StartAttrRequest()
		bmW.AppendData(1 << nfs4.FATTR4_TYPE)
		buf := bmW.Finish()
		rdw.Resume(buf)
		buf = rdw.Finish()
		w.Resume(buf)
	})

	if res.Status() != nfs4.NFS4ERR_TOOSMALL {
		t.Fatalf("expected NFS4ERR_TOOSMALL, got %s", nfs4.Nfsstat4Name(res.Status()))
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
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
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

func TestReaddirNewFileVisibleBeforeClose(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("x"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Get root FH.
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Getfh()
	})
	xid++
	iter := expectOK(t, res)
	nextOp(t, &iter)
	entry := nextOp(t, &iter)
	rootFH := append([]byte(nil), entry.Value().AsGETFH4resEntry().Value().AsGETFH4resok().Object().Data()...)

	// The new pathname is published empty while its creator keeps staging.
	stateid, fh := openCreateFile(t, conn, &xid, clientid, "newfile.txt")
	emptyFH := lookupFH(t, conn, &xid, "newfile.txt")

	names := collectReaddirNames(t, conn, &xid, rootFH)
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"existing.txt", "newfile.txt"}) {
		t.Fatalf("READDIR before CLOSE = %v", names)
	}

	// An unchanged CLOSE keeps the already-published empty file.
	closeFile(t, conn, &xid, fh, stateid)
	if got := lookupFH(t, conn, &xid, "newfile.txt"); !bytes.Equal(got, emptyFH) {
		t.Fatal("unchanged creator replaced the empty published version")
	}

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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
		bmW.AppendData(1 << (nfs4.FATTR4_TIME_MODIFY_SET - 32))
		buf = bmW.Finish()
		faw.Resume(buf)
		// Attr data: SET_TO_CLIENT_TIME4(1) + nfstime4(sec:8 + nsec:4)
		attrData := make([]byte, 4+8+4)
		binary.BigEndian.PutUint32(attrData[0:4], nfs4.SET_TO_CLIENT_TIME4)
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
	if entry.Value().AsSETATTR4res().Status() != nfs4.NFS4_OK {
		t.Fatalf("SETATTR status = %s", nfs4.Nfsstat4Name(entry.Value().AsSETATTR4res().Status()))
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

	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
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
		bmW.AppendData(1 << (nfs4.FATTR4_MODE - 32)) // mode is not supported
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

	if res.Status() != nfs4.NFS4ERR_ATTRNOTSUPP {
		t.Fatalf("expected NFS4ERR_ATTRNOTSUPP, got %s", nfs4.Nfsstat4Name(res.Status()))
	}
}

func TestSetattrValidation(t *testing.T) {
	invalidTime := make([]byte, 16)
	binary.BigEndian.PutUint32(invalidTime[0:4], nfs4.SET_TO_CLIENT_TIME4)
	binary.BigEndian.PutUint32(invalidTime[12:16], 1_000_000_000)

	maxSize := make([]byte, 8)
	binary.BigEndian.PutUint64(maxSize, ^uint64(0))

	tests := []struct {
		name       string
		mask       attrMask
		attrData   []byte
		wantStatus uint32
	}{
		{
			name:       "read-only attribute",
			mask:       attrMask{1 << nfs4.FATTR4_SUPPORTED_ATTRS, 0},
			wantStatus: nfs4.NFS4ERR_INVAL,
		},
		{
			name:       "mode missing data",
			mask:       attrMask{0, 1 << (nfs4.FATTR4_MODE - 32)},
			wantStatus: nfs4.NFS4ERR_BADXDR,
		},
		{
			name:       "mode trailing data",
			mask:       attrMask{0, 1 << (nfs4.FATTR4_MODE - 32)},
			attrData:   make([]byte, 8),
			wantStatus: nfs4.NFS4ERR_BADXDR,
		},
		{
			name: "invalid time nanoseconds",
			mask: attrMask{
				0,
				1 << (nfs4.FATTR4_TIME_MODIFY_SET - 32),
			},
			attrData:   invalidTime,
			wantStatus: nfs4.NFS4ERR_INVAL,
		},
		{
			name:       "size exceeds signed backend range",
			mask:       attrMask{1 << nfs4.FATTR4_SIZE, 0},
			attrData:   maxSize,
			wantStatus: nfs4.NFS4ERR_FBIG,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			addr, cleanup := startTestServer(t, dir)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()

			res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()

				saw := w.AppendArgarray_Setattr()
				saw.Stateid().SetSeqid(0)
				faw := saw.StartObjAttributes()
				bmW := faw.StartAttrmask()
				bmW.AppendData(test.mask[0])
				if test.mask[1] != 0 {
					bmW.AppendData(test.mask[1])
				}
				buf := bmW.Finish()
				faw.Resume(buf)
				alW := faw.StartAttrVals()
				buf = alW.SetData(test.attrData).Finish()
				faw.Resume(buf)
				buf = faw.Finish()
				saw.Resume(buf)
				w.Resume(saw.Finish())
			})

			if res.Status() != test.wantStatus {
				t.Fatalf("status = %s, want %s",
					nfs4.Nfsstat4Name(res.Status()),
					nfs4.Nfsstat4Name(test.wantStatus))
			}
		})
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
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		ww := w.AppendArgarray_Write()
		setStateid(ww.Stateid(), stateid)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(2)
		ww = ww.SetData(writeData)
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	// SETATTR to truncate to 10 bytes.
	res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		saw := w.AppendArgarray_Setattr()
		setStateid(saw.Stateid(), stateid)
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(1 << nfs4.FATTR4_SIZE)
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
	if entry.Value().AsSETATTR4res().Status() != nfs4.NFS4_OK {
		t.Fatalf("SETATTR status = %s", nfs4.Nfsstat4Name(entry.Value().AsSETATTR4res().Status()))
	}

	// The anonymous stateid is also valid for a size-changing SETATTR.
	res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())

		saw := w.AppendArgarray_Setattr()
		saw.Stateid().SetSeqid(0)
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(1 << nfs4.FATTR4_SIZE)
		buf = bmW.Finish()
		faw.Resume(buf)
		attrData := make([]byte, 8)
		binary.BigEndian.PutUint64(attrData, 5)
		buf = faw.StartAttrVals().SetData(attrData).Finish()
		faw.Resume(buf)
		saw.Resume(faw.Finish())
		w.Resume(saw.Finish())
	})
	xid++
	expectOK(t, res)

	// Close and verify.
	closeFile(t, conn, &xid, fh, stateid)

	data, err := os.ReadFile(filepath.Join(dir, "trunc.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 5 {
		t.Fatalf("file size = %d, want 5", len(data))
	}
}

func TestSetattrSizeRequiresWriteOpen(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openReadFile(t, conn, &xid, clientid, "file.txt")

	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())

		saw := w.AppendArgarray_Setattr()
		setStateid(saw.Stateid(), stateid)
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(1 << nfs4.FATTR4_SIZE)
		buf = bmW.Finish()
		faw.Resume(buf)
		attrData := make([]byte, 8)
		binary.BigEndian.PutUint64(attrData, 1)
		buf = faw.StartAttrVals().SetData(attrData).Finish()
		faw.Resume(buf)
		saw.Resume(faw.Finish())
		w.Resume(saw.Finish())
	})
	if res.Status() != nfs4.NFS4ERR_OPENMODE {
		t.Fatalf("SETATTR size status = %s, want NFS4ERR_OPENMODE",
			nfs4.Nfsstat4Name(res.Status()))
	}
}

func openExclusiveFile(
	t *testing.T, conn net.Conn, xid *uint32, clientid uint64,
	owner string, filename string, verifier [8]byte,
) (status uint32, stateid [16]byte, fh []byte, rflags uint32) {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte(owner))
		buf := ownerW.Finish()
		ow.Resume(buf)
		chw := ow.SetOpenhow_Create()
		verf := chw.SetValue_Exclusive4()
		for i := range verifier {
			verf.SetData(i, verifier[i])
		}
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
	if res.Status() != nfs4.NFS4_OK {
		return res.Status(), stateid, nil, 0
	}
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTROOTFH
	openOK := nextOp(t, &iter).Value().AsOPEN4resEntry().Value().AsOPEN4resok()
	sid := openOK.Stateid()
	binary.BigEndian.PutUint32(stateid[:4], sid.Seqid())
	for i := range 12 {
		stateid[4+i] = sid.Other(i)
	}
	fh = append([]byte(nil), nextOp(t, &iter).Value().AsGETFH4resEntry().
		Value().AsGETFH4resok().Object().Data()...)
	return nfs4.NFS4_OK, stateid, fh, openOK.Rflags()
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

	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_READ)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientid)
		ownerW = ownerW.SetOwner([]byte("test-owner"))
		buf := ownerW.Finish()
		ow.Resume(buf)
		ow.SetOpenhow_Default(nfs4.OPEN4_NOCREATE)
		ow.SetClaim_Previous()
		buf = ow.Finish()
		w.Resume(buf)
	})
	xid++

	if res.Status() != nfs4.NFS4ERR_NO_GRACE {
		t.Fatalf("expected NFS4ERR_NO_GRACE, got %s", nfs4.Nfsstat4Name(res.Status()))
	}
}

func TestCloseReplay(t *testing.T) {
	dir := t.TempDir()
	srv, addr, cleanup := startTestServerWithServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)

	// Create and close a file.
	stateid, fh := openCreateFile(t, conn, &xid, clientid, "replay.txt")
	status, first := closeFileResult(
		t, conn, &xid, fh, stateid)
	if status != nfs4.NFS4_OK {
		t.Fatalf("CLOSE status = %s", nfs4.Nfsstat4Name(status))
	}

	// Verify file exists.
	if _, err := os.Stat(filepath.Join(dir, "replay.txt")); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	// Retransmit CLOSE on the same slot. The reply cache answers it.
	retransmitNext(conn)
	status, replay := closeFileResult(
		t, conn, &xid, fh, stateid)
	if status != nfs4.NFS4_OK {
		t.Fatalf("CLOSE replay: expected NFS4_OK, got %s", nfs4.Nfsstat4Name(status))
	}
	if replay != first {
		t.Fatalf("CLOSE replay stateid = %x, want %x", replay, first)
	}
	var localStateID nfs4.StateID
	copy(localStateID[:], stateid[4:])
	if _, err := srv.fs.Lookup(
		vfs.InodeID(clientid), clientstore.ActiveOpenName(localStateID),
	); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("CLOSE replay retained the open marker: %v", err)
	}
}

func TestReopenReusesState(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "file.txt"), []byte("data"), 0644,
	); err != nil {
		t.Fatal(err)
	}
	srv, addr, cleanup := startTestServerWithServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	status, first, fh, flags := openFileForOwner(
		t, conn, &xid, clientID, "shared-owner", "file.txt",
		nfs4.OPEN4_SHARE_ACCESS_READ, false, false,
	)
	if status != nfs4.NFS4_OK || flags&nfs4.OPEN4_RESULT_CONFIRM != 0 {
		t.Fatalf("first OPEN status = %s, rflags = %#x",
			nfs4.Nfsstat4Name(status), flags)
	}
	if got := binary.BigEndian.Uint32(first[:4]); got != 1 {
		t.Fatalf("first OPEN generation = %d, want 1", got)
	}

	status, upgraded, upgradedFH, flags := openFileForOwner(
		t, conn, &xid, clientID, "shared-owner", "file.txt",
		nfs4.OPEN4_SHARE_ACCESS_READ, false, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("second OPEN status = %s", nfs4.Nfsstat4Name(status))
	}

	if !bytes.Equal(first[4:], upgraded[4:]) {
		t.Fatalf("second OPEN stateid other = %x, want %x",
			upgraded[4:], first[4:])
	}
	if got := binary.BigEndian.Uint32(upgraded[:4]); got != 2 {
		t.Fatalf("second OPEN generation = %d, want 2", got)
	}
	if !bytes.Equal(upgradedFH, fh) {
		t.Fatal("second OPEN returned a different filehandle")
	}
	// Read opens are process-local: no marker, however often reopened.
	if got := activeOpenMarkerCount(t, srv, clientID); got != 0 {
		t.Fatalf("active-open markers after read reopen = %d, want 0", got)
	}
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())
		rw := w.AppendArgarray_Read()
		setStateid(rw.Stateid(), first)
		rw.SetOffset(0)
		rw.SetCount(1)
	})
	xid++
	if res.Status() != nfs4.NFS4ERR_OLD_STATEID {
		t.Fatalf("READ with generation 1 = %s, want NFS4ERR_OLD_STATEID",
			nfs4.Nfsstat4Name(res.Status()))
	}
	if status := closeFileStatus(
		t, conn, &xid, fh, upgraded); status != nfs4.NFS4_OK {
		t.Fatalf("CLOSE status = %s", nfs4.Nfsstat4Name(status))
	}
	if got := activeOpenMarkerCount(t, srv, clientID); got != 0 {
		t.Fatalf("active-open markers after CLOSE = %d, want 0", got)
	}
}

func TestReadCloseAfterFileRemoval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	stateid, fh := openReadFile(t, conn, &xid, clientID, "file.txt")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if status := closeFileStatus(
		t, conn, &xid, fh, stateid); status != nfs4.NFS4_OK {
		t.Fatalf("CLOSE after removal = %s, want NFS4_OK",
			nfs4.Nfsstat4Name(status))
	}
}

func TestWriteCloseWithoutStagingExpiresAndDropsState(t *testing.T) {
	dir := t.TempDir()
	srv, addr, cleanup := startTestServerWithServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	stateid, fh := openCreateFile(
		t, conn, &xid, clientID, "lost.txt")
	srv.stagingStore.Remove(stagingIDForTest(t, srv.stagingStore, stateid))
	if status := closeFileStatus(
		t, conn, &xid, fh, stateid); status != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("CLOSE without staging = %s, want NFS4ERR_EXPIRED",
			nfs4.Nfsstat4Name(status))
	}
	if status := closeFileStatus(
		t, conn, &xid, fh, stateid); status != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("replayed CLOSE without staging = %s, want NFS4ERR_EXPIRED",
			nfs4.Nfsstat4Name(status))
	}
	if got := activeOpenMarkerCount(t, srv, clientID); got != 0 {
		t.Fatalf("active-open markers after expired CLOSE = %d, want 0", got)
	}
}

type blockingVFSGate struct {
	mu           sync.Mutex
	calls        int
	firstEntered chan struct{}
	releaseFirst chan struct{}
}

func (gate *blockingVFSGate) enter() error {
	gate.mu.Lock()
	gate.calls++
	call := gate.calls
	gate.mu.Unlock()
	if call == 1 {
		close(gate.firstEntered)
		select {
		case <-gate.releaseFirst:
		case <-time.After(testutil.ChannelTimeout):
			return errors.New("timed out waiting to release VFS gate")
		}
	}
	return nil
}

func (gate *blockingVFSGate) callCount() int {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	return gate.calls
}

type blockingConstructVFS struct {
	vfs.TernVFS
	blockingVFSGate
}

func (fs *blockingConstructVFS) ConstructFile(
	dirID vfs.InodeID,
) (vfs.InodeID, vfs.Cookie, error) {
	if clientStoreDir(fs.TernVFS, dirID) {
		return fs.TernVFS.ConstructFile(dirID)
	}
	if err := fs.enter(); err != nil {
		return 0, vfs.Cookie{}, err
	}
	return fs.TernVFS.ConstructFile(dirID)
}

func TestConcurrentOpenSharesOwnerState(t *testing.T) {
	fs := &blockingConstructVFS{
		TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		blockingVFSGate: blockingVFSGate{
			firstEntered: make(chan struct{}),
			releaseFirst: make(chan struct{}),
		},
	}
	defer testutil.CloseSignal(fs.releaseFirst)
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
	firstConn := dial(t, addr)
	defer firstConn.Close()
	secondConn := dial(t, addr)
	defer secondConn.Close()
	xid := uint32(1)
	clientID := setupClient(t, firstConn, &xid)

	type result struct {
		status  uint32
		stateid [16]byte
		fh      []byte
		err     error
	}
	results := make(chan result, 2)
	open := func(conn net.Conn, xid uint32) {
		status, stateid, fh, _, err := openFileForOwnerE(
			conn, &xid, clientID, "shared-owner", "race.txt",
			nfs4.OPEN4_SHARE_ACCESS_BOTH, true, false,
		)
		results <- result{
			status: status, stateid: stateid, fh: fh, err: err,
		}
	}
	go open(firstConn, 100)
	testutil.AwaitSignal(t, fs.firstEntered, "first OPEN entering ConstructFile")
	go open(secondConn, 200)
	testutil.CloseSignal(fs.releaseFirst)
	first := testutil.AwaitValue(t, results, "first concurrent OPEN result")
	second := testutil.AwaitValue(t, results, "second concurrent OPEN result")

	if first.err != nil || second.err != nil {
		t.Fatalf("concurrent OPEN errors = %v, %v", first.err, second.err)
	}
	if first.status != nfs4.NFS4_OK || second.status != nfs4.NFS4_OK {
		t.Fatalf("concurrent OPEN statuses = %s, %s",
			nfs4.Nfsstat4Name(first.status), nfs4.Nfsstat4Name(second.status))
	}
	// Two OPENs by one owner on different session slots are separate
	// requests. Each creates its own writer; the second is serialized on
	// the pathname and finds the first's placeholder.
	if first.stateid == second.stateid {
		t.Fatal("concurrent write OPENs shared a stateid")
	}
}

type failingLinkVFS struct {
	vfs.TernVFS

	mu        sync.Mutex
	remaining int
}

type closeResult struct {
	status  uint32
	stateid [16]byte
	err     error
}

func (fs *failingLinkVFS) LinkFile(
	fileID vfs.InodeID,
	cookie vfs.Cookie,
	dirID vfs.InodeID,
	name string,
	data io.Reader,
) error {
	if data == nil || clientstore.LeaseTemp(name) {
		return fs.TernVFS.LinkFile(fileID, cookie, dirID, name, data)
	}
	fs.mu.Lock()
	if fs.remaining > 0 {
		fs.remaining--
		fs.mu.Unlock()
		// A definite local syscall rejection, before any publication effect.
		// Unknown submitted outcomes have separate terminal-session tests.
		return syscall.EIO
	}
	fs.mu.Unlock()
	return fs.TernVFS.LinkFile(fileID, cookie, dirID, name, data)
}

type blockingFailingLinkVFS struct {
	vfs.TernVFS
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (fs *blockingFailingLinkVFS) LinkFile(
	fileID vfs.InodeID,
	cookie vfs.Cookie,
	dirID vfs.InodeID,
	name string,
	data io.Reader,
) error {
	if data == nil || clientstore.LeaseTemp(name) {
		return fs.TernVFS.LinkFile(fileID, cookie, dirID, name, data)
	}
	fs.once.Do(func() { close(fs.entered) })
	select {
	case <-fs.release:
	case <-time.After(testutil.ChannelTimeout):
		return errors.New("timed out waiting to release LinkFile")
	}
	return errors.New("injected LinkFile failure")
}

type notifyingStagingStore struct {
	staging.Store

	mu       sync.Mutex
	calls    int
	notifyAt int
	notified chan struct{}
}

func (store *notifyingStagingStore) GetMeta(
	id vfs.InodeID,
) (staging.Meta, bool) {
	store.mu.Lock()
	store.calls++
	if store.calls == store.notifyAt {
		close(store.notified)
	}
	store.mu.Unlock()
	return store.Store.GetMeta(id)
}

func (store *notifyingStagingStore) notifyNextGetMeta() <-chan struct{} {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.calls = 0
	store.notifyAt = 1
	store.notified = make(chan struct{})
	return store.notified
}

func TestWaitingCloseRechecksStagingMeta(t *testing.T) {
	fs := &blockingFailingLinkVFS{
		TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	defer testutil.CloseSignal(fs.release)
	localStaging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	staging := &notifyingStagingStore{Store: localStaging}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	firstConn := dial(t, addr)
	defer firstConn.Close()
	secondConn := dial(t, addr)
	defer secondConn.Close()

	xid := uint32(1)
	clientID := setupClient(t, firstConn, &xid)
	status, stateid, fh, _ := openFileForOwner(
		t, firstConn, &xid, clientID, "shared-owner", "race.txt",
		nfs4.OPEN4_SHARE_ACCESS_BOTH, true, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("OPEN status = %s", nfs4.Nfsstat4Name(status))
	}

	writeFileAt(t, firstConn, &xid, fh, stateid, 0, []byte("pending"))
	firstResult := make(chan closeResult, 1)
	go func() {
		closeXID := uint32(100)
		status, result, err := closeFileResultE(
			firstConn, &closeXID, fh, stateid)
		firstResult <- closeResult{
			status: status, stateid: result, err: err,
		}
	}()
	testutil.AwaitSignal(t, fs.entered, "first CLOSE entering LinkFile")
	secondRead := srv.waitForOpenFileWaiter(clientID, fh)
	secondResult := make(chan closeResult, 1)
	go func() {
		closeXID := uint32(200)
		status, result, err := closeFileResultE(
			secondConn, &closeXID, fh, stateid)
		secondResult <- closeResult{
			status: status, stateid: result, err: err,
		}
	}()
	testutil.AwaitSignal(t, secondRead, "second CLOSE waiting for the first")
	testutil.CloseSignal(fs.release)

	first := testutil.AwaitValue(t, firstResult, "first racing CLOSE result")
	second := testutil.AwaitValue(t, secondResult, "second racing CLOSE result")
	if first.err != nil || second.err != nil {
		t.Fatalf("CLOSE errors = %v, %v", first.err, second.err)
	}
	if first.status != nfs4.NFS4ERR_IO {
		t.Fatalf("first CLOSE = %s, want NFS4ERR_IO",
			nfs4.Nfsstat4Name(first.status))
	}
	if second.status != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("second CLOSE = %s, want NFS4ERR_EXPIRED",
			nfs4.Nfsstat4Name(second.status))
	}
	if n := srv.opens.StateCount(); n != 0 {
		t.Fatalf("states after lost write = %d, want 0", n)
	}
}

func TestFailedCloseReplaysFromSlot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "read.txt"), []byte("read"), 0644,
	); err != nil {
		t.Fatal(err)
	}
	fs := &failingLinkVFS{
		TernVFS:   vfs.NewLocalTernVFS(dir),
		remaining: 1,
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
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	status, stateid, fh, _ := openFileForOwner(
		t, conn, &xid, clientID, "shared-owner", "write.txt",
		nfs4.OPEN4_SHARE_ACCESS_BOTH, true, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("OPEN status = %s", nfs4.Nfsstat4Name(status))
	}
	writeFileAt(t, conn, &xid, fh, stateid, 0, []byte("pending"))

	if status := closeFileStatus(
		t, conn, &xid, fh, stateid); status != nfs4.NFS4ERR_IO {
		t.Fatalf("failed CLOSE status = %s, want NFS4ERR_IO",
			nfs4.Nfsstat4Name(status))
	}
	// The failed publication ends the open: its staging is discarded and
	// the client will not close it again.
	if n := srv.opens.StateCount(); n != 0 {
		t.Fatalf("%d opens after the failed CLOSE, want 0", n)
	}
	if n := len(staging.Entries()); n != 0 {
		t.Fatalf("%d staging entries after the failed CLOSE, want 0", n)
	}
	retransmitNext(conn)
	if status := closeFileStatus(
		t, conn, &xid, fh, stateid); status != nfs4.NFS4ERR_IO {
		t.Fatalf("failed CLOSE replay = %s, want NFS4ERR_IO",
			nfs4.Nfsstat4Name(status))
	}
	status, _, _, flags := openFileForOwner(
		t, conn, &xid, clientID, "shared-owner", "read.txt",
		nfs4.OPEN4_SHARE_ACCESS_READ, false, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("OPEN after failed CLOSE = %s, want NFS4_OK",
			nfs4.Nfsstat4Name(status))
	}
	_ = flags
}

type blockingLinkVFS struct {
	vfs.TernVFS
	blockingVFSGate
}

func (fs *blockingLinkVFS) LinkFile(
	fileID vfs.InodeID,
	cookie vfs.Cookie,
	dirID vfs.InodeID,
	name string,
	data io.Reader,
) error {
	if data == nil || clientstore.LeaseTemp(name) {
		return fs.TernVFS.LinkFile(fileID, cookie, dirID, name, data)
	}
	if err := fs.enter(); err != nil {
		return err
	}
	return fs.TernVFS.LinkFile(fileID, cookie, dirID, name, data)
}

func TestConcurrentClosePublishesAtMostOnce(t *testing.T) {
	fs := &blockingLinkVFS{
		TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		blockingVFSGate: blockingVFSGate{
			firstEntered: make(chan struct{}),
			releaseFirst: make(chan struct{}),
		},
	}
	defer testutil.CloseSignal(fs.releaseFirst)
	localStaging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	staging := &notifyingStagingStore{Store: localStaging}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	firstConn := dial(t, addr)
	defer firstConn.Close()
	secondConn := dial(t, addr)
	defer secondConn.Close()

	xid := uint32(1)
	clientID := setupClient(t, firstConn, &xid)
	status, stateid, fh, _ := openFileForOwner(
		t, firstConn, &xid, clientID, "shared-owner", "race.txt",
		nfs4.OPEN4_SHARE_ACCESS_BOTH, true, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatalf("OPEN status = %s", nfs4.Nfsstat4Name(status))
	}
	writeFileAt(t, firstConn, &xid, fh, stateid, 0, []byte("pending"))

	results := make(chan closeResult, 1)
	secondResults := make(chan closeResult, 1)
	go func() {
		closeXID := uint32(100)
		status, result, err := closeFileResultE(
			firstConn, &closeXID, fh, stateid)
		results <- closeResult{
			status: status, stateid: result, err: err,
		}
	}()
	testutil.AwaitSignal(t, fs.firstEntered, "first CLOSE entering LinkFile")
	secondRequestEntered := srv.waitForOpenFileWaiter(clientID, fh)
	go func() {
		closeXID := uint32(200)
		status, result, err := closeFileResultE(
			secondConn, &closeXID, fh, stateid)
		secondResults <- closeResult{
			status: status, stateid: result, err: err,
		}
	}()
	testutil.AwaitSignal(t, secondRequestEntered, "second CLOSE waiting for the first")
	testutil.CloseSignal(fs.releaseFirst)
	first := testutil.AwaitValue(t, results, "first concurrent CLOSE result")
	second := testutil.AwaitValue(t, secondResults, "second concurrent CLOSE result")

	if first.err != nil || second.err != nil {
		t.Fatalf("concurrent CLOSE errors = %v, %v", first.err, second.err)
	}
	// Different slots are different requests: the second CLOSE waits for
	// the first and then finds the stateid closed.
	if first.status != nfs4.NFS4_OK || second.status != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("concurrent CLOSE statuses = %s, %s",
			nfs4.Nfsstat4Name(first.status), nfs4.Nfsstat4Name(second.status))
	}
	if calls := fs.callCount(); calls != 1 {
		t.Fatalf("LinkFile calls = %d, want 1", calls)
	}
}

func TestCloseUnknownStateid(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "file.txt"), []byte("data"), 0644,
	); err != nil {
		t.Fatal(err)
	}
	srv, addr, cleanup := startTestServerWithServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	fileID, err := srv.fs.Lookup(srv.fs.RootID(), "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	fh := binary.BigEndian.AppendUint64(nil, uint64(fileID))
	var unknown [16]byte
	binary.BigEndian.PutUint32(unknown[:4], 1)
	for i := 4; i < len(unknown); i++ {
		unknown[i] = 0xa5
	}

	xid := uint32(1)
	status := closeFileStatus(t, conn, &xid, fh, unknown)
	if status != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("expected NFS4ERR_BAD_STATEID, got %s", nfs4.Nfsstat4Name(status))
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
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
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

	if res.Status() != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("WRITE with bad stateid: status = %s, want NFS4ERR_BAD_STATEID",
			nfs4.Nfsstat4Name(res.Status()))
	}
}

// TestWriteSpecialStateid verifies that WRITE accepts the anonymous special
// stateid (all zeros) on a file already open for write, matching opRead's
// existing isSpecialStateID exemption. Clients — including pynfs's own
// nfs4lib.write_file default and the Linux kernel client's zero_stateid
// fallback — may legitimately omit a real stateid on WRITE.
func TestWriteSpecialStateid(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openCreateFile(
		t, conn, &xid, clientid, "specialwrite.txt")

	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())
		ww := w.AppendArgarray_Write()
		sid := ww.Stateid()
		sid.SetSeqid(0)
		for i := 0; i < 12; i++ {
			sid.SetOther(i, 0) // anonymous special stateid
		}
		ww = ww.SetOffset(0)
		ww = ww.SetStable(fileSync4)
		ww = ww.SetData([]byte("hello"))
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++

	expectOK(t, res)
	closeFile(t, conn, &xid, fh, stateid)
	data, err := os.ReadFile(filepath.Join(dir, "specialwrite.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("file content = %q, want %q", data, "hello")
	}
}

// TestSetattrSizeSpecialStateid verifies that SETATTR(SIZE) accepts the
// anonymous special stateid on a file already open for write, matching
// opRead's and opWrite's isSpecialStateID exemption. pynfs's own
// nfs4lib.setattr default stateid is the anonymous one.
func TestSetattrSizeSpecialStateid(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid := setupClient(t, conn, &xid)
	stateid, fh := openCreateFile(
		t, conn, &xid, clientid, "specialsetattr.txt")

	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())
		saw := w.AppendArgarray_Setattr()
		sid := saw.Stateid()
		sid.SetSeqid(0)
		for i := 0; i < 12; i++ {
			sid.SetOther(i, 0) // anonymous special stateid
		}
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(1 << nfs4.FATTR4_SIZE)
		buf = bmW.Finish()
		faw.Resume(buf)
		attrData := make([]byte, 8)
		binary.BigEndian.PutUint64(attrData, 10)
		alW := faw.StartAttrVals()
		buf = alW.SetData(attrData).Finish()
		faw.Resume(buf)
		buf = faw.Finish()
		saw.Resume(buf)
		buf = saw.Finish()
		w.Resume(buf)
	})
	xid++

	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("SETATTR(SIZE) with special stateid: status = %s, want NFS4_OK",
			nfs4.Nfsstat4Name(res.Status()))
	}
	closeFile(t, conn, &xid, fh, stateid)
	info, err := os.Stat(filepath.Join(dir, "specialsetattr.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 10 {
		t.Fatalf("file size = %d, want 10", info.Size())
	}
}

// TestWriteAndSetattrSizeNoStagingSameStatus checks that WRITE and SETATTR
// size with the anonymous stateid on a file that has no staging on this nfsd
// both report NFS4ERR_OPENMODE, the same status as a read-only open.
func TestWriteAndSetattrSizeNoStagingSameStatus(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "nostaging.txt"), []byte("content"), 0644)
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	setupClient(t, conn, &xid)
	fh := lookupFH(t, conn, &xid, "nostaging.txt")

	anonymous := func(sid nfs4.Stateid4) {
		sid.SetSeqid(0)
		for i := 0; i < 12; i++ {
			sid.SetOther(i, 0)
		}
	}

	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		ww := w.AppendArgarray_Write()
		anonymous(ww.Stateid())
		ww = ww.SetOffset(0)
		ww = ww.SetStable(unstable4)
		ww = ww.SetData([]byte("X"))
		w.Resume(ww.Finish())
	})
	xid++
	if res.Status() != nfs4.NFS4ERR_OPENMODE {
		t.Fatalf("WRITE without staging: status = %s, want NFS4ERR_OPENMODE",
			nfs4.Nfsstat4Name(res.Status()))
	}

	res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		saw := w.AppendArgarray_Setattr()
		anonymous(saw.Stateid())
		faw := saw.StartObjAttributes()
		bmW := faw.StartAttrmask()
		bmW.AppendData(1 << nfs4.FATTR4_SIZE)
		faw.Resume(bmW.Finish())
		attrData := make([]byte, 8)
		binary.BigEndian.PutUint64(attrData, 1)
		faw.Resume(faw.StartAttrVals().SetData(attrData).Finish())
		saw.Resume(faw.Finish())
		w.Resume(saw.Finish())
	})
	xid++
	if res.Status() != nfs4.NFS4ERR_OPENMODE {
		t.Fatalf("SETATTR size without staging: status = %s, want NFS4ERR_OPENMODE",
			nfs4.Nfsstat4Name(res.Status()))
	}
}

func TestSetattrSizeRejectsNonRegularFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "directory"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("destination", filepath.Join(dir, "symlink")); err != nil {
		t.Fatal(err)
	}
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	for _, tc := range []struct {
		name   string
		status uint32
	}{
		{"directory", nfs4.NFS4ERR_ISDIR},
		{"symlink", nfs4.NFS4ERR_INVAL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fh := lookupFH(t, conn, &xid, tc.name)
			res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
				pw := w.AppendArgarray_Putfh()
				pw.Resume(pw.StartObject().SetData(fh).Finish())
				w.Resume(pw.Finish())
				saw := w.AppendArgarray_Setattr()
				setStateid(saw.Stateid(), [16]byte{})
				faw := saw.StartObjAttributes()
				bmW := faw.StartAttrmask()
				bmW.AppendData(1 << nfs4.FATTR4_SIZE)
				faw.Resume(bmW.Finish())
				data := binary.BigEndian.AppendUint64(nil, 0)
				faw.Resume(faw.StartAttrVals().SetData(data).Finish())
				saw.Resume(faw.Finish())
				w.Resume(saw.Finish())
			})
			xid++
			if res.Status() != tc.status {
				t.Fatalf("SETATTR size on %s: status = %s, want %s",
					tc.name, nfs4.Nfsstat4Name(res.Status()), nfs4.Nfsstat4Name(tc.status))
			}
		})
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
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		fhW := pw.StartObject()
		buf := fhW.SetData(fh).Finish()
		pw.Resume(buf)
		buf = pw.Finish()
		w.Resume(buf)
		ww := w.AppendArgarray_Write()
		setStateid(ww.Stateid(), stateid)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(0) // UNSTABLE4
		ww = ww.SetData([]byte("commit test"))
		buf = ww.Finish()
		w.Resume(buf)
	})
	xid++
	expectOK(t, res)

	// COMMIT and check the write verifier is non-zero.
	res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
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
	fs := vfs.NewLocalTernVFS(dir)
	ss := staging.ReadOnlyStore{}
	srv, err := NewServer(fs, ss, nil)
	if err != nil {
		t.Fatal(err)
	}
	useTestWireEpoch(srv)

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
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
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

	if res.Status() != nfs4.NFS4ERR_ROFS {
		t.Fatalf("expected NFS4ERR_ROFS, got %s", nfs4.Nfsstat4Name(res.Status()))
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
	res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lock()
		lw.SetLocker_False()
		w.Resume(lw.Finish())
	})
	if res.Status() != nfs4.NFS4ERR_NOTSUPP {
		t.Fatalf("LOCK: expected NFS4ERR_NOTSUPP, got %s", nfs4.Nfsstat4Name(res.Status()))
	}

	// LOCKT — variable-size writer.
	res = sendCompound(t, conn, 2, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lockt()
		ow := lw.StartOwner()
		ow.SetOwner([]byte("x"))
		lw.Resume(ow.Finish())
		w.Resume(lw.Finish())
	})
	if res.Status() != nfs4.NFS4ERR_NOTSUPP {
		t.Fatalf("LOCKT: expected NFS4ERR_NOTSUPP, got %s", nfs4.Nfsstat4Name(res.Status()))
	}

	// LOCKU — fixed-size, no finish needed.
	res = sendCompound(t, conn, 3, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Locku()
	})
	if res.Status() != nfs4.NFS4ERR_NOTSUPP {
		t.Fatalf("LOCKU: expected NFS4ERR_NOTSUPP, got %s", nfs4.Nfsstat4Name(res.Status()))
	}
}

type restartedStagedWrite struct {
	rootDir    string
	stagingDir string
	fs         vfs.TernVFS
	srv        *Server
	staging    *staging.LocalStore
	clientID   uint64
	conn       net.Conn
	xid        uint32
	stateid    [16]byte
	fh         []byte
}

func restartWithStagedWrite(
	t *testing.T,
	name string,
	initial []byte,
) restartedStagedWrite {
	t.Helper()
	rootDir, stagingDir := t.TempDir(), t.TempDir()
	fs := vfs.NewLocalTernVFS(rootDir)
	firstStaging, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewServer(fs, firstStaging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stopFirst := serveTestServer(t, first)
	conn := dial(t, addr)
	xid := uint32(1)
	wireID := setupClient(t, conn, &xid)
	clientID := incarnationForTest(t, first, wireID)
	stateid, fh := openCreateFile(
		t, conn, &xid, wireID, name)
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		buf := pw.StartObject().SetData(fh).Finish()
		pw.Resume(buf)
		w.Resume(pw.Finish())
		ww := w.AppendArgarray_Write()
		setStateid(ww.Stateid(), stateid)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(fileSync4)
		ww = ww.SetData(initial)
		w.Resume(ww.Finish())
	})
	xid++
	expectOK(t, res)
	fileID := stagingIDForTest(t, firstStaging, stateid)
	conn.Close()
	stopFirst()
	staging.CloseFiles(t, firstStaging)

	recoveredStaging, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	meta, found := recoveredStaging.GetMeta(fileID)
	if !found {
		t.Fatal("staging sidecar was not recovered")
	}
	if meta.ClientID != clientID {
		t.Fatalf("recovered clientid = %d, want %d",
			meta.ClientID, clientID)
	}
	second, err := NewServer(fs, recoveredStaging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stopSecond := serveTestServer(t, second)
	conn = dial(t, addr)
	t.Cleanup(func() {
		conn.Close()
		stopSecond()
	})

	return restartedStagedWrite{
		rootDir:    rootDir,
		stagingDir: stagingDir,
		fs:         fs,
		srv:        second,
		staging:    recoveredStaging,
		clientID:   clientID,
		conn:       conn,
		xid:        100,
		stateid:    stateid,
		fh:         fh,
	}
}

func TestRecoveredStagedWriteWithDeadLeaseExpires(t *testing.T) {
	restarted := restartWithStagedWrite(
		t, "expired-lease.txt", []byte("staged data"))
	restarted.srv.clients.SetClock(func() time.Time {
		return time.Now().Add(2 * clientstore.LeaseTime)
	})
	res := sendCompound(
		t, restarted.conn, restarted.xid,
		func(w *nfs4.COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			buf := pw.StartObject().SetData(restarted.fh).Finish()
			pw.Resume(buf)
			w.Resume(pw.Finish())
			rw := w.AppendArgarray_Read()
			setStateid(rw.Stateid(), restarted.stateid)
			rw.SetOffset(0)
			rw.SetCount(1)
		})
	// With its lease dead the client registers a new incarnation, whose
	// session the old stateid does not belong to
	// (../openstate/README.md#stateid-scope). The
	// predecessor's staging is retired by its expiry, not by the READ.
	if res.Status() != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("recovered READ with dead lease = %s, want BAD_STATEID",
			nfs4.Nfsstat4Name(res.Status()))
	}
	fileID := stagingIDForTest(t, restarted.staging, restarted.stateid)
	if m, ok := restarted.staging.GetMeta(fileID); !ok || m.Retired {
		t.Fatal("a foreign READ retired the staging")
	}
	restarted.srv.runLeaseSweep()
	assertRetiredStaging(t, restarted.staging, fileID)
}

func TestStagedWriteOperationsAfterServerRestart(t *testing.T) {
	for _, operation := range []string{"close", "write", "read", "setattr-size"} {
		t.Run(operation, func(t *testing.T) {
			initial := []byte("before restart")
			restarted := restartWithStagedWrite(
				t, operation+".txt", initial)

			expected := append([]byte(nil), initial...)
			if operation != "close" {
				res := sendCompound(
					t, restarted.conn, restarted.xid,
					func(w *nfs4.COMPOUND4argsWriter) {
						pw := w.AppendArgarray_Putfh()
						buf := pw.StartObject().SetData(restarted.fh).Finish()
						pw.Resume(buf)
						w.Resume(pw.Finish())
						switch operation {
						case "write":
							appended := []byte(" and after")
							expected = append(expected, appended...)
							ww := w.AppendArgarray_Write()
							setStateid(ww.Stateid(), restarted.stateid)
							ww = ww.SetOffset(uint64(len(initial)))
							ww = ww.SetStable(fileSync4)
							ww = ww.SetData(appended)
							w.Resume(ww.Finish())
						case "read":
							rw := w.AppendArgarray_Read()
							setStateid(rw.Stateid(), restarted.stateid)
							rw.SetOffset(0)
							rw.SetCount(1024)
						case "setattr-size":
							expected = expected[:6]
							saw := w.AppendArgarray_Setattr()
							setStateid(saw.Stateid(), restarted.stateid)
							faw := saw.StartObjAttributes()
							bmW := faw.StartAttrmask()
							bmW.AppendData(1 << nfs4.FATTR4_SIZE)
							buf = bmW.Finish()
							faw.Resume(buf)
							attrData := make([]byte, 8)
							binary.BigEndian.PutUint64(
								attrData, uint64(len(expected)))
							alW := faw.StartAttrVals()
							buf = alW.SetData(attrData).Finish()
							faw.Resume(buf)
							saw.Resume(faw.Finish())
							w.Resume(saw.Finish())
						}
					})
				restarted.xid++
				iter := expectOK(t, res)
				nextOp(t, &iter)
				entry := nextOp(t, &iter)
				if operation == "read" {
					got := entry.Value().AsREAD4resEntry().
						Value().AsREAD4resok().Data()
					if !bytes.Equal(got, expected) {
						t.Fatalf("recovered READ = %q, want %q", got, expected)
					}
				}
			}

			status, first := closeFileResult(
				t, restarted.conn, &restarted.xid, restarted.fh,
				restarted.stateid)
			if status != nfs4.NFS4_OK {
				t.Fatalf("recovered CLOSE = %s", nfs4.Nfsstat4Name(status))
			}
			if operation == "close" {
				retransmitNext(restarted.conn)
				status, replay := closeFileResult(
					t, restarted.conn, &restarted.xid, restarted.fh,
					restarted.stateid)
				if status != nfs4.NFS4_OK {
					t.Fatalf("recovered CLOSE replay = %s",
						nfs4.Nfsstat4Name(status))
				}
				if replay != first {
					t.Fatalf("recovered CLOSE replay stateid = %x, want %x",
						replay, first)
				}
			}
			got, err := os.ReadFile(
				filepath.Join(restarted.rootDir, operation+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, expected) {
				t.Fatalf("recovered file data = %q, want %q", got, expected)
			}
			entries, err := stagingFiles(restarted.stagingDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("recovered CLOSE retained staging files: %v", entries)
			}
			var localStateID nfs4.StateID
			copy(localStateID[:], restarted.stateid[4:])
			if _, err := restarted.fs.Lookup(
				vfs.InodeID(restarted.clientID), clientstore.ActiveOpenName(localStateID),
			); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("recovered CLOSE retained the open marker: %v", err)
			}
		})
	}
}

func TestRecoveredStagedWriteRequiresActiveMarker(t *testing.T) {
	for _, operation := range []string{"read", "close"} {
		t.Run(operation, func(t *testing.T) {
			restarted := restartWithStagedWrite(
				t, operation+".txt", []byte("staged data"))
			var stateID nfs4.StateID
			copy(stateID[:], restarted.stateid[4:])
			store, err := clientstore.NewStore(restarted.fs)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.RemoveOpen(
				restarted.clientID, stateID,
			); err != nil {
				t.Fatal(err)
			}

			var status uint32
			switch operation {
			case "read":
				res := sendCompound(
					t, restarted.conn, restarted.xid,
					func(w *nfs4.COMPOUND4argsWriter) {
						pw := w.AppendArgarray_Putfh()
						buf := pw.StartObject().
							SetData(restarted.fh).Finish()
						pw.Resume(buf)
						w.Resume(pw.Finish())
						rw := w.AppendArgarray_Read()
						setStateid(rw.Stateid(), restarted.stateid)
						rw.SetOffset(0)
						rw.SetCount(1)
					})
				status = res.Status()
			case "close":
				status = closeFileStatus(
					t, restarted.conn, &restarted.xid,
					restarted.fh, restarted.stateid)
			}
			if status != nfs4.NFS4ERR_EXPIRED {
				t.Fatalf("%s status = %s, want NFS4ERR_EXPIRED",
					operation, nfs4.Nfsstat4Name(status))
			}
			entries, err := stagingFiles(restarted.stagingDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 2 {
				t.Fatalf("%s lost recoverable staging: %v", operation, entries)
			}
			id := stagingIDForTest(t, restarted.staging, restarted.stateid)
			assertRetiredStaging(t, restarted.staging, id)
		})
	}
}

func TestSetclientidRebootRetiresStagingFiles(t *testing.T) {
	rootDir := t.TempDir()
	stagingDir := t.TempDir()
	fs := vfs.NewLocalTernVFS(rootDir)
	staging, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	stateid, fh := openCreateFile(
		t, conn, &xid, clientID, "reboot.txt")
	fileID := stagingIDForTest(t, staging, stateid)
	if staging.Get(fileID) == nil {
		t.Fatal("write OPEN did not create staging")
	}

	var rebootVerifier [8]byte
	for i := range rebootVerifier {
		rebootVerifier[i] = byte(i + 1)
	}
	setupClientWithVerifier(t, conn, &xid, rebootVerifier)

	assertRetiredStaging(t, staging, fileID)
	entries, err := stagingFiles(stagingDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("client reboot lost recoverable staging: %v", entries)
	}
	// The rebooted client is a new incarnation: its predecessor's stateid
	// is not its own (../openstate/README.md#stateid-scope).
	if status := closeFileStatus(
		t, conn, &xid, fh, stateid); status != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("CLOSE after reboot = %s, want NFS4ERR_BAD_STATEID",
			nfs4.Nfsstat4Name(status))
	}
}

func TestLocalStagingStoreRejectsPreMagicSidecars(t *testing.T) {
	// There are no deployed NFS servers, so nfsd reads one sidecar format.
	// Sidecars from before the format marker are quarantined, not loaded.
	meta := staging.Meta{
		DirID:      vfs.MakeInodeID(vfs.InodeTypeDir, 7),
		FileName:   "legacy.txt",
		TernCookie: vfs.Cookie{1, 2, 3},
		NFSStateID: nfs4.StateID{4, 5, 6},
		ClientID:   99,
	}
	name := []byte(meta.FileName)
	header := make([]byte, 30+len(name))
	binary.BigEndian.PutUint64(header[0:8], uint64(meta.DirID))
	copy(header[8:16], meta.TernCookie[:])
	copy(header[16:28], meta.NFSStateID[:])
	binary.BigEndian.PutUint16(header[28:30], uint16(len(name)))
	copy(header[30:], name)
	withClientID := append(append([]byte(nil), header...), make([]byte, 8)...)
	binary.BigEndian.PutUint64(withClientID[len(header):], meta.ClientID)

	for _, sidecar := range []struct {
		name    string
		encoded []byte
	}{
		{"no-clientid", header},
		{"clientid-only", withClientID},
	} {
		t.Run(sidecar.name, func(t *testing.T) {
			dir := t.TempDir()
			id := vfs.MakeInodeID(vfs.InodeTypeFile, 42)
			data := []byte("legacy staged data")
			if err := os.WriteFile(
				filepath.Join(dir, fmt.Sprintf("%016x.staging", uint64(id))),
				data, 0600,
			); err != nil {
				t.Fatal(err)
			}
			metaPath := filepath.Join(
				dir, fmt.Sprintf("%016x.meta", uint64(id)))
			if err := os.WriteFile(metaPath, sidecar.encoded, 0600); err != nil {
				t.Fatal(err)
			}
			store, err := staging.NewLocalStore(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer staging.CloseFiles(t, store)
			if _, found := store.GetMeta(id); found || store.Get(id) != nil {
				t.Fatal("pre-magic staging sidecar was loaded")
			}
			files, err := filepath.Glob(
				filepath.Join(dir, "quarantine", "*", "*.staging"))
			if err != nil || len(files) != 1 {
				t.Fatalf("quarantine files = %v, err = %v", files, err)
			}
			if got, err := os.ReadFile(files[0]); err != nil ||
				!bytes.Equal(got, data) {
				t.Fatalf("quarantined data = %q, err = %v", got, err)
			}
		})
	}
}

func TestLoadStagingMetaRejectsTrailingBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "staging.meta")
	meta := staging.Meta{
		DirID:      vfs.MakeInodeID(vfs.InodeTypeDir, 1),
		FileName:   "file.txt",
		TernCookie: vfs.Cookie{1},
		NFSStateID: nfs4.StateID{2},
		ClientID:   3,
	}
	if err := staging.SaveMeta(path, meta); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, 0), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := staging.LoadMeta(path); err == nil {
		t.Fatal("staging metadata with trailing bytes was accepted")
	}
}

func TestSetclientidReboot(t *testing.T) {
	dir := t.TempDir()
	srv, addr, cleanup := startTestServerWithServer(t, dir)
	fs := srv.fs
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientid1 := setupClient(t, conn, &xid)
	incarnation1 := incarnationForTest(t, srv, clientid1)
	stateid, fh := openCreateFile(
		t, conn, &xid, clientid1, "reboot.txt")

	other, err := clientstore.NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	clientid2, confirm2, err := other.SetClientID(
		[8]byte{1}, []byte("test-client"), clientstore.ClientOwner{})
	if err != nil {
		t.Fatal(err)
	}
	if clientid2 == incarnation1 {
		t.Fatal("client reboot reused clientid")
	}
	if replaced, err := other.ConfirmClientID(
		clientid2, confirm2, clientstore.RPCPrincipal{},
	); err != nil {
		t.Fatal(err)
	} else if replaced != incarnation1 {
		t.Fatalf("reboot replaced clientid %d, want %d", replaced, incarnation1)
	}

	t.Run("sequence", func(t *testing.T) {
		// The refresher's next pass finds the clientid replaced and retires
		// its sessions, so SEQUENCE refuses the call. Failed SEQUENCE
		// preserves the server slot, so the test client must not consume it.
		ts := testSessionForClient(conn, clientid1)
		leasePassForTest(t, srv, incarnation1)
		slot, seq := ts.acquire()
		defer ts.release(slot)
		body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
			w.AppendArgarray_Putrootfh()
		})
		reply, err := sendRawRPC(conn, xid, procCompound, withSequence(body, ts, slot, seq))
		xid++
		if err != nil {
			t.Fatal(err)
		}
		res, err := parseCompoundReply(reply)
		if err != nil {
			t.Fatal(err)
		}
		if res.Status() != nfs4.NFS4ERR_BADSESSION {
			t.Fatalf("SEQUENCE after remote reboot = %s, want NFS4ERR_BADSESSION",
				nfs4.Nfsstat4Name(res.Status()))
		}
		ts.mu.Lock()
		ts.seqs[slot]--
		ts.mu.Unlock()
	})

	t.Run("create session", func(t *testing.T) {
		ts := testSessionForClient(conn, clientid1)
		_, status, err := createSession(conn, xid, clientid1, ts.nextCSSeq)
		xid++
		if err != nil {
			t.Fatal(err)
		}
		if status != nfs4.NFS4ERR_STALE_CLIENTID {
			t.Fatalf("CREATE_SESSION after remote reboot = %s, want NFS4ERR_STALE_CLIENTID",
				nfs4.Nfsstat4Name(status))
		}
	})

	// The test client recovers like Linux: it registers again and uses a
	// session for the new incarnation. Old stateids are then another
	// incarnation's and report BAD_STATEID (../openstate/README.md#stateid-scope).
	t.Run("state operations", func(t *testing.T) {
		res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			buf := pw.StartObject().SetData(fh).Finish()
			pw.Resume(buf)
			w.Resume(pw.Finish())
			ww := w.AppendArgarray_Write()
			setStateid(ww.Stateid(), stateid)
			ww = ww.SetOffset(0)
			ww = ww.SetStable(fileSync4)
			ww = ww.SetData([]byte("stale"))
			w.Resume(ww.Finish())
		})
		xid++
		if res.Status() != nfs4.NFS4ERR_BAD_STATEID {
			t.Fatalf("WRITE after remote reboot = %s, want NFS4ERR_BAD_STATEID",
				nfs4.Nfsstat4Name(res.Status()))
		}
		fileID := stagingIDForTest(t, srv.stagingStore, stateid)
		assertRetiredStaging(t, srv.stagingStore, fileID)
		fhID, _ := vfs.FHToInodeID(fh)
		var localStateID nfs4.StateID
		copy(localStateID[:], stateid[4:])
		if _, status := srv.opens.Lookup(
			incarnation1,
			localStateID,
			binary.BigEndian.Uint32(stateid[:4]),
			fhID,
		); status != nfs4.NFS4ERR_EXPIRED {
			t.Fatalf("local open after remote reboot = %s, want NFS4ERR_EXPIRED",
				nfs4.Nfsstat4Name(status))
		}

		res = sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			buf := pw.StartObject().SetData(fh).Finish()
			pw.Resume(buf)
			w.Resume(pw.Finish())
			rw := w.AppendArgarray_Read()
			setStateid(rw.Stateid(), stateid)
			rw.SetOffset(0)
			rw.SetCount(1)
		})
		xid++
		if res.Status() != nfs4.NFS4ERR_BAD_STATEID {
			t.Fatalf("READ after remote reboot = %s, want NFS4ERR_BAD_STATEID",
				nfs4.Nfsstat4Name(res.Status()))
		}

		status := closeFileStatus(
			t, conn, &xid, fh, stateid)
		if status != nfs4.NFS4ERR_BAD_STATEID {
			t.Fatalf("CLOSE after remote reboot = %s, want NFS4ERR_BAD_STATEID",
				nfs4.Nfsstat4Name(status))
		}
	})
}

// TestReaddirEntryXDRSize verifies that the size prediction formula in
// readdirEntryXDRSize exactly matches the actual encoded XDR output for
// various attribute masks and name lengths.
func TestReaddirEntryXDRSize(t *testing.T) {
	id := vfs.MakeInodeID(vfs.InodeTypeFile, 42)
	ni := vfs.NodeInfo{Size: 1024}

	cases := []struct {
		name   string
		mask   attrMask
		fname  string
		statOK bool
	}{
		{"no_attrs", attrMask{0, 0}, "file.txt", true},
		{"type_only", attrMask{1 << nfs4.FATTR4_TYPE, 0}, "file.txt", true},
		{"size_and_type", attrMask{(1 << nfs4.FATTR4_TYPE) | (1 << nfs4.FATTR4_SIZE), 0}, "file.txt", true},
		{"full_attrs", attrMask{supportedAttrs0, supportedAttrs1}, "file.txt", true},
		{"short_name", attrMask{1 << nfs4.FATTR4_TYPE, 0}, "a", true},
		{"padded_name", attrMask{1 << nfs4.FATTR4_TYPE, 0}, "ab", true},  // 2 bytes, pads to 4
		{"exact_4", attrMask{1 << nfs4.FATTR4_TYPE, 0}, "abcd", true},    // 4 bytes, no pad
		{"needs_pad", attrMask{1 << nfs4.FATTR4_TYPE, 0}, "abcde", true}, // 5 bytes, pads to 8
		{"stat_failed", attrMask{supportedAttrs0, supportedAttrs1}, "file.txt", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var respMask attrMask
			respMask[0] = tc.mask[0] & supportedAttrs0
			respMask[1] = tc.mask[1] & supportedAttrs1

			re := readdirEntry{
				DirEntry: vfs.DirEntry{
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
			dirW := nfs4.StartDirlist4(nil)
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

func TestServerLeaseSweepExpiresButDoesNotRevokeConfirmedClient(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	srv.clients.SetClock(func() time.Time { return base })
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := srv.clients.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	publishLeaseForTest(t, srv, clientID)
	fileID, _, err := fs.ConstructFile(fs.RootID())
	if err != nil {
		t.Fatal(err)
	}
	state := addConfirmedOpenForTest(
		t, srv, clientID, "reader", fileID, false)

	srv.clients.SetClock(func() time.Time {
		return base.Add(clientstore.LeaseWriteTerm + time.Second)
	})
	srv.sweepExpiredClientState()
	if _, status := srv.opens.Lookup(
		state.Owner.ClientID, state.ID, state.Generation, fileID,
	); status != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("swept state = %s, want NFS4ERR_EXPIRED",
			nfs4.Nfsstat4Name(status))
	}

	if err := srv.clients.Renew(clientID); nfs4.ErrorCode(err) != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("RENEW after lease sweep = %v, want NFS4ERR_EXPIRED", err)
	}
	if err := srv.clients.MarkOpen(
		clientID, nfs4.StateID{2},
	); nfs4.ErrorCode(err) != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("OPEN marker after lease sweep = %v, want NFS4ERR_EXPIRED",
			err)
	}
	if confirmed, err := srv.clients.IsConfirmed(clientID); err != nil {
		t.Fatal(err)
	} else if !confirmed {
		t.Fatal("lease sweep revoked a clientid that is still confirmed")
	}
}

func TestServerLeaseSweepDropsRenewOnlyClientCache(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	srv.clients.SetClock(func() time.Time { return base })
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := srv.clients.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	// RENEW reads the refresher record, which the worker publishes, and
	// caches no client state, so a renew-only client leaves nothing for the
	// sweep to drop.
	publishLeaseForTest(t, srv, clientID)
	if err := srv.clients.Renew(clientID); err != nil {
		t.Fatal(err)
	}
	if got := srv.clients.CachedClientIDs(); len(got) != 0 {
		t.Fatalf("cached clients after RENEW = %v, want none", got)
	}

	srv.clients.SetClock(func() time.Time {
		return base.Add(clientstore.LeaseWriteTerm + time.Second)
	})
	srv.sweepExpiredClientState()
	if got := srv.clients.CachedClientIDs(); len(got) != 0 {
		t.Fatalf("sweep retained cache for a dead lease: %v", got)
	}
}

func TestClientStoreMarkOpenReusesFreshLease(t *testing.T) {
	fs := &clientstore.CountingVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	store, err := clientstore.NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	store.Now = func() time.Time { return base }
	owner := clientstore.ClientOwner{
		Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"},
	}
	clientID, confirm, err := store.SetClientID(
		[8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConfirmClientID(
		clientID, confirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}

	// The first OPEN has no cached lease and must write one.
	fs.Reset()
	if err := store.MarkOpen(clientID, nfs4.StateID{1}); err != nil {
		t.Fatal(err)
	}
	if fs.Renames != 1 {
		t.Fatalf("first MarkOpen lease writes = %d, want 1", fs.Renames)
	}
	if err := store.RemoveOpen(clientID, nfs4.StateID{1}); err != nil {
		t.Fatal(err)
	}

	// An open-close loop within the renewal window creates only the marker.
	for i := 2; i <= 4; i++ {
		store.Now = func() time.Time {
			return base.Add(time.Duration(i) * time.Second)
		}
		fs.Reset()
		stateID := nfs4.StateID{byte(i)}
		if err := store.MarkOpen(clientID, stateID); err != nil {
			t.Fatal(err)
		}
		if fs.CreateFiles != 1 || fs.Renames != 0 {
			t.Fatalf("MarkOpen %d writes: create=%d rename=%d, want marker only",
				i, fs.CreateFiles, fs.Renames)
		}
		if err := store.RemoveOpen(clientID, stateID); err != nil {
			t.Fatal(err)
		}
	}

	// Once the cached lease is a third of the way to expiry, OPEN renews it.
	store.Now = func() time.Time {
		return base.Add(clientstore.LeaseRenewAfter + time.Second)
	}
	fs.Reset()
	if err := store.MarkOpen(clientID, nfs4.StateID{5}); err != nil {
		t.Fatal(err)
	}
	if fs.Renames != 1 {
		t.Fatalf("aged MarkOpen lease writes = %d, want 1", fs.Renames)
	}

	// A replaced confirmed pointer forces a renewal even with a fresh lease.
	other, err := clientstore.NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	other.Now = store.Now
	rebootID, rebootConfirm, err := other.SetClientID(
		[8]byte{2}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.ConfirmClientID(
		rebootID, rebootConfirm, owner.Principal,
	); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOpen(clientID, nfs4.StateID{6}); nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("MarkOpen on replaced clientid = %v, want NFS4ERR_STALE_CLIENTID", err)
	}
}

func TestRunLeaseSweepContinuesAfterPanic(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	staging, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, staging, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	srv.clients.SetClock(func() time.Time { return base })
	owner := clientstore.ClientOwner{Principal: clientstore.RPCPrincipal{Flavor: nfs4.AUTH_SYS, Body: "owner"}}
	clientID, confirm, err := srv.clients.SetClientID([8]byte{1}, []byte("client"), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(clientID, confirm, owner.Principal); err != nil {
		t.Fatal(err)
	}
	publishLeaseForTest(t, srv, clientID)
	state := addConfirmedOpenForTest(t, srv, clientID, "owner",
		vfs.MakeInodeID(vfs.InodeTypeFile, 80), false)
	srv.clients.SetClock(func() time.Time { return base.Add(3 * clientstore.LeaseTime) })
	srv.startedAt = base
	srv.stagingStore = nil
	srv.runLeaseSweep()
	if _, status := srv.opens.Lookup(state.Owner.ClientID, state.ID, 0, state.FileID); status != nfs4.NFS4_OK {
		t.Fatal("panicking sweep unexpectedly reached client expiry")
	}
	srv.stagingStore = staging
	srv.runLeaseSweep()
	if _, status := srv.opens.Lookup(state.Owner.ClientID, state.ID, 0, state.FileID); status != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("later sweep did not expire the client: %s", nfs4.Nfsstat4Name(status))
	}
}
