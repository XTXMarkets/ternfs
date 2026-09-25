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
	"os"
	"testing"
	"time"
)

func exerciseVisibleCreation(t *testing.T, addr, name string, reverse bool) {
	t.Helper()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	size := uint64(5)
	creator, creatorFH := openCreateFileWithSize(t, conn, &xid, clientID, name, &size)
	emptyFH := lookupFH(t, conn, &xid, name)
	if bytes.Equal(emptyFH, creatorFH) {
		t.Fatal("creator must have a separate private handle")
	}
	checkSize := func(fh []byte, want uint64) {
		t.Helper()
		attrs := mutableAttrs(t, conn, &xid, fh)
		if got := binary.BigEndian.Uint64(attrs[8:16]); got != want {
			t.Fatalf("size of %x = %d, want %d", fh, got, want)
		}
	}
	check := func(fh []byte, want string) {
		t.Helper()
		data, eof := readFileData(t, conn, &xid, fh, 0, 1024)
		if string(data) != want || !eof {
			t.Fatalf("READ = (%q, %t), want (%q, true)", data, eof, want)
		}
	}
	checkSize(emptyFH, 0)
	checkSize(creatorFH, size)
	writeFileAt(t, conn, &xid, creatorFH, creator, 0, []byte("first"))
	check(emptyFH, "")
	check(lookupFH(t, conn, &xid, name), "")
	// A repeated guarded create now sees the published empty name.
	status, _, _, _ := openFileForOwner(
		t, conn, &xid, clientID, "guarded", 1, name,
		OPEN4_SHARE_ACCESS_BOTH, true, true,
	)
	if status != NFS4ERR_EXIST {
		t.Fatalf("GUARDED create = %s, want EXIST", Nfsstat4Name(status))
	}
	status, reader, readerFH, _ := openFileForOwner(
		t, conn, &xid, clientID, "reader", 1, name,
		OPEN4_SHARE_ACCESS_READ, false, false,
	)
	if status != NFS4_OK {
		t.Fatal(Nfsstat4Name(status))
	}
	reader = confirmOpenState(t, conn, &xid, readerFH, 2, reader)
	if !bytes.Equal(readerFH, emptyFH) {
		t.Fatal("reader did not open the empty published version")
	}
	status, writer, writerFH, _ := openFileForOwner(
		t, conn, &xid, clientID, "writer", 1, name,
		OPEN4_SHARE_ACCESS_BOTH, false, false,
	)
	if status != NFS4_OK {
		t.Fatal(Nfsstat4Name(status))
	}
	writer = confirmOpenState(t, conn, &xid, writerFH, 2, writer)
	if bytes.Equal(writerFH, creatorFH) || bytes.Equal(writerFH, emptyFH) {
		t.Fatal("second writer did not get independent staging")
	}
	checkSize(writerFH, 0)
	writeFileAt(t, conn, &xid, writerFH, writer, 0, []byte("second"))
	check(creatorFH, "first")
	check(writerFH, "second")
	check(readerFH, "")
	firstWant, lastWant := "first", "second"
	if reverse {
		creator, writer = writer, creator
		creatorFH, writerFH = writerFH, creatorFH
		firstWant, lastWant = lastWant, firstWant
	}
	closeFile(t, conn, &xid, creatorFH, creator)
	check(lookupFH(t, conn, &xid, name), firstWant)
	check(readerFH, "")
	check(writerFH, lastWant)
	closeFile(t, conn, &xid, writerFH, writer)
	check(lookupFH(t, conn, &xid, name), lastWant)
	check(readerFH, "")
	closeFile(t, conn, &xid, readerFH, reader)
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
		TernVFS: NewLocalTernVFS(t.TempDir()),
		blockingVFSGate: blockingVFSGate{
			firstEntered: make(chan struct{}),
			releaseFirst: make(chan struct{}),
		},
	}
	defer closeSignal(fs.releaseFirst)
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
			firstConn, &seq, clientID, "first", 1, "guarded",
			OPEN4_SHARE_ACCESS_BOTH, true, true,
		)
		if err != nil {
			status = NFS4ERR_SERVERFAULT
		}
		results <- status
	}()
	awaitSignal(t, fs.firstEntered, "guarded create allocating its empty inode")
	go func() {
		seq := uint32(200)
		status, _, _, _, err := openFileForOwnerE(
			secondConn, &seq, clientID, "second", 1, "guarded",
			OPEN4_SHARE_ACCESS_READ, true, true,
		)
		if err != nil {
			status = NFS4ERR_SERVERFAULT
		}
		results <- status
	}()
	closeSignal(fs.releaseFirst)
	first := awaitValue(t, results, "first guarded create result")
	second := awaitValue(t, results, "second guarded create result")
	if !(first == NFS4_OK && second == NFS4ERR_EXIST ||
		first == NFS4ERR_EXIST && second == NFS4_OK) {
		t.Fatalf("guarded creates returned %s and %s", Nfsstat4Name(first), Nfsstat4Name(second))
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
	srv.clients.now = func() time.Time { return now }
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
	fs := NewLocalTernVFS(t.TempDir())
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
		t, conn, &xid, clientID, "readonly", &size, OPEN4_SHARE_ACCESS_READ,
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
	res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
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
	if res.Status() != NFS4ERR_OPENMODE {
		t.Fatalf("recovered read-only WRITE = %s", Nfsstat4Name(res.Status()))
	}
	closeFile(t, conn, &xid, fh, state)
	data, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "readonly"), 0, 1024)
	if !bytes.Equal(data, make([]byte, 5)) {
		t.Fatalf("initial CREATE size was lost: %q", data)
	}
	data, _ = readFileData(t, conn, &xid, emptyFH, 0, 1024)
	if len(data) != 0 {
		t.Fatal("original empty version changed")
	}
}

type visibilityFailureVFS struct {
	*LocalTernVFS
	failure    string
	constructs int
}

func (fs *visibilityFailureVFS) ConstructFile(dirID InodeID) (InodeID, Cookie, error) {
	fs.constructs++
	if fs.failure == "staging" && fs.constructs == 2 {
		return 0, Cookie{}, errors.New("injected staging construction failure")
	}
	return fs.LocalTernVFS.ConstructFile(dirID)
}

func (fs *visibilityFailureVFS) CreateFile(dirID InodeID, name string, data io.Reader) (InodeID, error) {
	if fs.failure == "marker" && isActiveOpenName(name) {
		return 0, errors.New("injected open marker failure")
	}
	return fs.LocalTernVFS.CreateFile(dirID, name, data)
}

func (fs *visibilityFailureVFS) LinkFile(id InodeID, cookie Cookie, dirID InodeID, name string, data io.Reader) error {
	if fs.failure == "link" && data == nil {
		return errors.New("injected empty publication failure")
	}
	return fs.LocalTernVFS.LinkFile(id, cookie, dirID, name, data)
}

func TestFailedCreationDoesNotPublishEmptyVersion(t *testing.T) {
	for _, failure := range []string{"staging", "marker", "link"} {
		t.Run(failure, func(t *testing.T) {
			fs := &visibilityFailureVFS{
				LocalTernVFS: NewLocalTernVFS(t.TempDir()),
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
				t, conn, &xid, clientID, "creator", 1, "failed",
				OPEN4_SHARE_ACCESS_BOTH, true, false,
			)
			if status == NFS4_OK {
				t.Fatal("injected failure did not fail OPEN")
			}
			if _, err := fs.Lookup(fs.RootID(), "failed"); !os.IsNotExist(err) {
				t.Fatalf("failed creation exposed a pathname: %v", err)
			}
			fs.mu.RLock()
			transients := len(fs.transient)
			fs.mu.RUnlock()
			if len(store.Entries()) != 0 || transients != 0 {
				t.Fatal("failed creation leaked staging or transient inodes")
			}
			if count := activeOpenMarkerCount(t, srv, clientID); count != 0 {
				t.Fatalf("failed creation left %d open markers", count)
			}
		})
	}
}
