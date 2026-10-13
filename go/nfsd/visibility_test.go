// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// exerciseVisibleCreation checks that CREATE publishes an empty file at
// once. The creator's client sees its staged contents through the empty
// file's handle; other clients see the empty version until CLOSE.
func exerciseVisibleCreation(t *testing.T, addr, name string, reverse bool) {
	t.Helper()
	conn := dial(t, addr)
	defer conn.Close()
	otherConn := dial(t, addr)
	defer otherConn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	otherID, confirm := requestClientID(t, otherConn, &xid, "other-"+name, [8]byte{2})
	if status := confirmClientID(t, otherConn, &xid, otherID, confirm); status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	size := uint64(5)
	creator, creatorFH := openCreateFileWithSize(t, conn, &xid, clientID, name, &size)
	emptyFH := lookupFH(t, otherConn, &xid, name)
	if !bytes.Equal(emptyFH, creatorFH) {
		t.Fatal("creator's handle is not the published empty file")
	}
	checkSize := func(c net.Conn, fh []byte, want uint64) {
		t.Helper()
		attrs := mutableAttrs(t, c, &xid, fh)
		if got := binary.BigEndian.Uint64(attrs[8:16]); got != want {
			t.Fatalf("size of %x = %d, want %d", fh, got, want)
		}
	}
	check := func(c net.Conn, fh []byte, want string) {
		t.Helper()
		data, eof := readFileData(t, c, &xid, fh, 0, 1024)
		if string(data) != want || !eof {
			t.Fatalf("READ = (%q, %t), want (%q, true)", data, eof, want)
		}
	}
	checkSize(otherConn, emptyFH, 0)
	checkSize(conn, creatorFH, size)
	writeFileAt(t, conn, &xid, creatorFH, creator, 0, []byte("first"))
	check(otherConn, emptyFH, "")
	check(conn, creatorFH, "first")
	// A repeated guarded create now sees the published empty name.
	status, _, _, _ := openFileForOwner(
		t, otherConn, &xid, otherID, "guarded", name,
		nfs4.OPEN4_SHARE_ACCESS_BOTH, true, true,
	)
	if status != nfs4.NFS4ERR_EXIST {
		t.Fatalf("GUARDED create = %s, want EXIST", nfs4.Nfsstat4Name(status))
	}
	status, reader, readerFH, _ := openFileForOwner(
		t, otherConn, &xid, otherID, "reader", name,
		nfs4.OPEN4_SHARE_ACCESS_READ, false, false,
	)
	if status != nfs4.NFS4_OK || !bytes.Equal(readerFH, emptyFH) {
		t.Fatalf("reader OPEN = %s", nfs4.Nfsstat4Name(status))
	}
	status, writer, writerFH, _ := openFileForOwner(
		t, otherConn, &xid, otherID, "writer", name,
		nfs4.OPEN4_SHARE_ACCESS_BOTH, false, false,
	)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	checkSize(otherConn, writerFH, 0)
	writeFileAt(t, otherConn, &xid, writerFH, writer, 0, []byte("second"))
	check(conn, creatorFH, "first")
	check(otherConn, writerFH, "second")
	firstWant, lastWant := "first", "second"
	firstConn, lastConn := conn, otherConn
	if reverse {
		creator, writer = writer, creator
		firstConn, lastConn = lastConn, firstConn
		firstWant, lastWant = lastWant, firstWant
	}
	viewer := dial(t, addr)
	defer viewer.Close()
	viewerID, viewerConfirm := requestClientID(t, viewer, &xid, "viewer-"+name, [8]byte{3})
	if status := confirmClientID(t, viewer, &xid, viewerID, viewerConfirm); status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	closeFile(t, firstConn, &xid, emptyFH, creator)
	check(viewer, lookupFH(t, viewer, &xid, name), firstWant)
	check(viewer, emptyFH, "")
	check(lastConn, emptyFH, lastWant)
	closeFile(t, lastConn, &xid, emptyFH, writer)
	check(viewer, lookupFH(t, viewer, &xid, name), lastWant)
	check(viewer, emptyFH, "")
	closeFile(t, otherConn, &xid, readerFH, reader)
}

func TestVisibleCreation(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			addr, cleanup := startTestServer(t, t.TempDir())
			defer cleanup()
			exerciseVisibleCreation(t, addr, "visible.txt", reverse)
		})
	}
}

func TestConcurrentGuardedCreationPublishesOnce(t *testing.T) {
	fs := &blockingConstructVFS{
		TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		blockingVFSGate: blockingVFSGate{
			firstEntered: make(chan struct{}),
			releaseFirst: make(chan struct{}),
		},
	}
	defer testutil.CloseSignal(fs.releaseFirst)
	store, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(fs, store, nil)
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
	results := make(chan uint32, 2)
	go func() {
		seq := uint32(100)
		status, _, _, _, err := openFileForOwnerE(
			firstConn, &seq, clientID, "first", "guarded",
			nfs4.OPEN4_SHARE_ACCESS_BOTH, true, true,
		)
		if err != nil {
			status = nfs4.NFS4ERR_SERVERFAULT
		}
		results <- status
	}()
	testutil.AwaitSignal(t, fs.firstEntered, "guarded create allocating its empty inode")
	go func() {
		seq := uint32(200)
		status, _, _, _, err := openFileForOwnerE(
			secondConn, &seq, clientID, "second", "guarded",
			nfs4.OPEN4_SHARE_ACCESS_READ, true, true,
		)
		if err != nil {
			status = nfs4.NFS4ERR_SERVERFAULT
		}
		results <- status
	}()
	testutil.CloseSignal(fs.releaseFirst)
	first := testutil.AwaitValue(t, results, "first guarded create result")
	second := testutil.AwaitValue(t, results, "second guarded create result")
	if !(first == nfs4.NFS4_OK && second == nfs4.NFS4ERR_EXIST ||
		first == nfs4.NFS4ERR_EXIST && second == nfs4.NFS4_OK) {
		t.Fatalf("guarded creates returned %s and %s", nfs4.Nfsstat4Name(first), nfs4.Nfsstat4Name(second))
	}
	if fs.callCount() != 2 {
		t.Fatalf("constructed %d inodes, want one empty version and one staging inode", fs.callCount())
	}
}

func TestAbandonedCreationKeepsEmptyVersion(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	state, fh := openCreateFile(t, conn, &xid, clientID, "abandoned")
	baseFH := lookupFH(t, conn, &xid, "abandoned")
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("unpublished"))
	srv.waitForClientGC()
	now := time.Now().Add(2 * nfsLeaseTime)
	srv.clients.setClock(func() time.Time { return now })
	srv.runLeaseSweep()
	if stagingTargetBusyForTest(srv.stagingStore, srv.fs.RootID(), "abandoned") {
		t.Fatal("expired creator still holds staging")
	}
	if got := lookupFH(t, conn, &xid, "abandoned"); !bytes.Equal(got, baseFH) {
		t.Fatal("expiry replaced the empty version")
	}
	data, eof := readFileData(t, conn, &xid, baseFH, 0, 1024)
	if len(data) != 0 || !eof {
		t.Fatal("expiry published private contents")
	}
}

func TestReadOnlyCreationSurvivesRestart(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	stagingDir := t.TempDir()
	store, err := NewLocalStagingStore(stagingDir, nil)
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
	size := uint64(5)
	state, fh := openCreateFileWithSizeAndAccess(
		t, conn, &xid, clientID, "readonly", &size, nfs4.OPEN4_SHARE_ACCESS_READ,
	)
	emptyFH := lookupFH(t, conn, &xid, "readonly")
	conn.Close()
	cleanup()
	for _, entry := range store.files {
		entry.file.prepareRemove()
	}
	recovered, err := NewLocalStagingStore(stagingDir, nil)
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
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		ww := w.AppendArgarray_Write()
		setStateid(ww.Stateid(), state)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(fileSync4)
		ww = ww.SetData([]byte("X"))
		w.Resume(ww.Finish())
	})
	xid++
	if res.Status() != nfs4.NFS4ERR_OPENMODE {
		t.Fatalf("recovered read-only WRITE = %s", nfs4.Nfsstat4Name(res.Status()))
	}
	closeFile(t, conn, &xid, fh, state)
	data, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "readonly"), 0, 1024)
	if !bytes.Equal(data, make([]byte, 5)) {
		t.Fatalf("initial CREATE size was lost: %q", data)
	}
	if !bytes.Equal(emptyFH, fh) {
		t.Fatal("creator's handle is not the published empty file")
	}
}

type visibilityFailureVFS struct {
	*vfs.LocalTernVFS
	failure    string
	constructs int
}

func (fs *visibilityFailureVFS) ConstructFile(dirID vfs.InodeID) (vfs.InodeID, vfs.Cookie, error) {
	if clientStoreDir(fs.LocalTernVFS, dirID) {
		return fs.LocalTernVFS.ConstructFile(dirID)
	}
	fs.constructs++
	if fs.failure == "staging" && fs.constructs == 2 {
		return 0, vfs.Cookie{}, errors.New("injected staging construction failure")
	}
	return fs.LocalTernVFS.ConstructFile(dirID)
}

func (fs *visibilityFailureVFS) CreateFile(dirID vfs.InodeID, name string, data io.Reader) (vfs.InodeID, error) {
	if fs.failure == "marker" && isActiveOpenName(name) {
		return 0, errors.New("injected open marker failure")
	}
	return fs.LocalTernVFS.CreateFile(dirID, name, data)
}

func (fs *visibilityFailureVFS) LinkFile(id vfs.InodeID, cookie vfs.Cookie, dirID vfs.InodeID, name string, data io.Reader) error {
	if fs.failure == "link" && data == nil {
		return errors.New("injected empty publication failure")
	}
	return fs.LocalTernVFS.LinkFile(id, cookie, dirID, name, data)
}

func TestFailedCreationDoesNotPublishEmptyVersion(t *testing.T) {
	for _, failure := range []string{"staging", "marker", "link"} {
		t.Run(failure, func(t *testing.T) {
			fs := &visibilityFailureVFS{
				LocalTernVFS: vfs.NewLocalTernVFS(t.TempDir()),
				failure:      failure,
			}
			store, err := NewLocalStagingStore(t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			srv, err := NewServer(fs, store, nil)
			if err != nil {
				t.Fatal(err)
			}
			addr, cleanup := serveTestServer(t, srv)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()
			xid := uint32(1)
			clientID := setupClient(t, conn, &xid)
			srv.waitForClientGC()
			status, _, _, _ := openFileForOwner(
				t, conn, &xid, clientID, "creator", "failed",
				nfs4.OPEN4_SHARE_ACCESS_BOTH, true, false,
			)
			if status == nfs4.NFS4_OK {
				t.Fatal("injected failure did not fail OPEN")
			}
			if _, err := fs.Lookup(fs.RootID(), "failed"); !os.IsNotExist(err) {
				t.Fatalf("failed creation exposed a pathname: %v", err)
			}
			transients := fs.TransientCount()
			if len(store.Entries()) != 0 || transients != 0 {
				t.Fatal("failed creation leaked staging or transient inodes")
			}
			if count := activeOpenMarkerCount(t, srv, clientID); count != 0 {
				t.Fatalf("failed creation left %d open markers", count)
			}
		})
	}
}
