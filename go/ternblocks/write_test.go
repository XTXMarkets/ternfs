// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestNoSpaceResponse(t *testing.T) {
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client, err := net.DialTCP("tcp4", nil, listener.Addr().(*net.TCPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server, err := listener.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client.SetDeadline(time.Now().Add(5 * time.Second))
	server.SetDeadline(time.Now().Add(5 * time.Second))

	bs := &blockService{}
	bs.storage.Store(&blockServiceStorage{reported: storageSpace{1000, 800}})
	logger := testLogger(t)
	var lastError error
	err = &os.PathError{Op: "write", Path: "tmp.block", Err: syscall.ENOSPC}
	if handleRequestError(logger, map[msgs.BlockServiceId]*blockService{1: bs}, nil, server, &lastError, 1, msgs.WRITE_BLOCK, err) {
		t.Fatal("must close a failed write connection, whose body may be unread")
	}
	if !errors.Is(lastError, syscall.ENOSPC) {
		t.Fatalf("lost ENOSPC: %v", lastError)
	}
	var response struct {
		Version uint32
		Kind    uint8
		Error   uint16
	}
	if err := binary.Read(client, binary.LittleEndian, &response); err != nil {
		t.Fatal(err)
	}
	if response.Version != msgs.BLOCKS_RESP_PROTOCOL_VERSION || response.Kind != msgs.ERROR || msgs.TernError(response.Error) != msgs.INTERNAL_ERROR {
		t.Fatalf("unexpected protocol response: %+v", response)
	}
	if bs.storage.Load().reported.available != 0 || bs.noSpaceErrors.Load() != 1 {
		t.Fatal("ENOSPC did not update capacity and monitoring")
	}
	if bs.ioErrors != 0 {
		t.Fatal("ENOSPC should not count as device failure")
	}
}

func TestFailedWriteCleanup(t *testing.T) {
	if os.Getenv("TERNBLOCKS_TEST_WRITE_LIMIT") != "1" {
		// Restrict only a subprocess, so the test cannot affect unrelated
		// writes. A short write followed by EFBIG exercises the same cleanup
		// path as ENOSPC without filling a real filesystem.
		cmd := exec.Command(os.Args[0], "-test.run=^TestFailedWriteCleanup$")
		cmd.Env = append(os.Environ(), "TERNBLOCKS_TEST_WRITE_LIMIT=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("write failure subprocess: %v\n%s", err, output)
		}
		return
	}
	dir := t.TempDir()
	signal.Ignore(syscall.SIGXFSZ)
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 1, Max: 1}); err != nil {
		t.Fatal(err)
	}
	var written uint64
	if _, err := writeBufToTemp(&written, dir, []byte("block data")); !errors.Is(err, syscall.EFBIG) {
		t.Fatalf("expected failed disk write, got %v", err)
	}
	if written != 0 {
		t.Fatal("failed write counted as written bytes")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed write leaked temporary files: %v", entries)
	}
	// Startup writes must also return disk-full-style errors, not panic
	// and take down every block service hosted by the process.
	logger := testLogger(t)
	if _, err := retrieveOrCreateKey(logger, dir); !errors.Is(err, syscall.EFBIG) {
		t.Fatalf("expected key write error, got %v", err)
	}
}

func TestSuccessfulWrite(t *testing.T) {
	dir := t.TempDir()
	var written uint64
	data := []byte("block data")
	name, err := writeBufToTemp(&written, filepath.Join(dir, "blocks"), data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) || written != uint64(len(data)) {
		t.Fatalf("successful write: data=%q, bytes=%d", got, written)
	}
}
