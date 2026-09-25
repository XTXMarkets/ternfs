// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func rpcCallForTest(xid, procedure uint32, body []byte) []byte {
	call := make([]byte, 40)
	for i, value := range []uint32{xid, rpcCall, rpcVersion, nfsProg, nfsVersion, procedure} {
		binary.BigEndian.PutUint32(call[i*4:], value)
	}
	return append(call, body...)
}

func statCallForTest(xid uint32) []byte {
	return rpcCallForTest(xid, procCompound, buildCompoundBody([]byte(fmt.Sprint(xid)), func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		attr := w.AppendArgarray_Getattr()
		bits := attr.StartAttrRequest()
		bits.AppendData(1 << FATTR4_TYPE)
		buf := bits.Finish()
		attr.Resume(buf)
		w.Resume(attr.Finish())
	}))
}

func writeCallForTest(t *testing.T, conn net.Conn, call []byte) {
	t.Helper()
	if err := writeFrame(conn, call); err != nil {
		t.Fatal(err)
	}
}

func readXIDForTest(t *testing.T, conn net.Conn) uint32 {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(testChannelTimeout)); err != nil {
		t.Fatal(err)
	}
	reply, err := readFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(reply) < 24 {
		t.Fatalf("short reply: %x", reply)
	}
	if binary.BigEndian.Uint32(reply[20:24]) != acceptSuccess {
		t.Fatalf("RPC failed: %x", reply)
	}
	if len(reply) > 24 {
		res, ok := ReadCOMPOUND4res(reply[24:])
		if !ok || res.Status() != NFS4_OK {
			t.Fatalf("COMPOUND failed: %x", reply)
		}
	}
	return binary.BigEndian.Uint32(reply)
}

type statGateVFS struct {
	TernVFS
	firstOnly bool
	entered   chan struct{}
	release   chan struct{}
	calls     atomic.Int32
	active    atomic.Int32
	maxActive atomic.Int32
}

func (fs *statGateVFS) Stat(id InodeID) (NodeInfo, error) {
	call := fs.calls.Add(1)
	active := fs.active.Add(1)
	defer fs.active.Add(-1)
	for old := fs.maxActive.Load(); active > old; old = fs.maxActive.Load() {
		if fs.maxActive.CompareAndSwap(old, active) {
			break
		}
	}
	if !fs.firstOnly || call == 1 {
		fs.entered <- struct{}{}
		<-fs.release
	}
	return fs.TernVFS.Stat(id)
}

func newConnectionTestServer(t *testing.T, limit int) (*Server, *statGateVFS) {
	t.Helper()
	fs := &statGateVFS{TernVFS: NewLocalTernVFS(t.TempDir()),
		entered: make(chan struct{}, 32), release: make(chan struct{})}
	srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.maxInFlightPerConn = limit
	srv.writeTimeout = time.Second
	return srv, fs
}

func TestConnectionConcurrentCompounds(t *testing.T) {
	for _, limit := range []int{1, 2} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			srv, fs := newConnectionTestServer(t, limit)
			fs.firstOnly = true
			addr, cleanup := serveTestServer(t, srv)
			defer cleanup()
			defer closeSignal(fs.release)
			conn := dial(t, addr)
			defer conn.Close()
			writeCallForTest(t, conn, statCallForTest(1))
			awaitSignal(t, fs.entered, "slow stat")
			writeCallForTest(t, conn, statCallForTest(2))
			if limit == 1 {
				conn.SetReadDeadline(time.Now().Add(25 * time.Millisecond))
				if _, err := readFrame(conn); err == nil {
					t.Fatal("reply bypassed limit of one")
				}
				closeSignal(fs.release)
				if xid := readXIDForTest(t, conn); xid != 1 {
					t.Fatalf("first reply = %d", xid)
				}
				if xid := readXIDForTest(t, conn); xid != 2 {
					t.Fatalf("second reply = %d", xid)
				}
			} else {
				if xid := readXIDForTest(t, conn); xid != 2 {
					t.Fatalf("fast reply = %d", xid)
				}
				closeSignal(fs.release)
				if xid := readXIDForTest(t, conn); xid != 1 {
					t.Fatalf("slow reply = %d", xid)
				}
			}
		})
	}
}

func TestConnectionAdmissionBound(t *testing.T) {
	const limit, count = 3, 12
	srv, fs := newConnectionTestServer(t, limit)
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	defer closeSignal(fs.release)
	conn := dial(t, addr)
	defer conn.Close()
	for i := 1; i <= count; i++ {
		writeCallForTest(t, conn, statCallForTest(uint32(i)))
	}
	for i := 0; i < limit; i++ {
		awaitSignal(t, fs.entered, "admitted stat")
	}
	select {
	case <-fs.entered:
		t.Fatal("admitted more handlers than the limit")
	case <-time.After(25 * time.Millisecond):
	}
	closeSignal(fs.release)
	seen := make(map[uint32]bool)
	for i := 0; i < count; i++ {
		xid := readXIDForTest(t, conn)
		if xid < 1 || xid > count || seen[xid] {
			t.Fatalf("unexpected or duplicate xid %d", xid)
		}
		seen[xid] = true
	}
	if fs.maxActive.Load() > limit || fs.calls.Load() != count {
		t.Fatalf("maximum=%d calls=%d", fs.maxActive.Load(), fs.calls.Load())
	}
}

type mkdirGateVFS struct {
	TernVFS
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (fs *mkdirGateVFS) Mkdir(dir InodeID, name string) (InodeID, error) {
	if name == "once" && fs.calls.Add(1) == 1 {
		close(fs.entered)
		<-fs.release
	}
	return fs.TernVFS.Mkdir(dir, name)
}

func TestConnectionInFlightDuplicates(t *testing.T) {
	for _, conflicting := range []bool{false, true} {
		t.Run(fmt.Sprint(conflicting), func(t *testing.T) {
			fs := &mkdirGateVFS{TernVFS: NewLocalTernVFS(t.TempDir()),
				entered: make(chan struct{}), release: make(chan struct{})}
			staging, err := NewLocalStagingStore(t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			srv, err := NewServer(fs, staging, nil)
			if err != nil {
				t.Fatal(err)
			}
			addr, cleanup := serveTestServer(t, srv)
			defer cleanup()
			defer closeSignal(fs.release)
			conn := dial(t, addr)
			defer conn.Close()
			body := buildCompoundBody(nil, func(w *COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				create := w.AppendArgarray_Create()
				create.SetObjtype_Nf4dir()
				name := create.StartObjname()
				buf := name.SetData([]byte("once")).Finish()
				create.Resume(buf)
				attrs := create.StartCreateattrs()
				mask := attrs.StartAttrmask()
				buf = mask.Finish()
				attrs.Resume(buf)
				values := attrs.StartAttrVals()
				buf = values.SetData(nil).Finish()
				attrs.Resume(buf)
				create.Resume(attrs.Finish())
				w.Resume(create.Finish())
			})
			call := rpcCallForTest(1, procCompound, body)
			writeCallForTest(t, conn, call)
			awaitSignal(t, fs.entered, "directory creation")
			if conflicting {
				writeCallForTest(t, conn, rpcCallForTest(1, procNull, nil))
				conn.SetReadDeadline(time.Now().Add(testChannelTimeout))
				if _, err := readFrame(conn); err == nil {
					t.Fatal("conflicting XID did not close connection")
				}
			} else {
				writeCallForTest(t, conn, call)
				writeCallForTest(t, conn, rpcCallForTest(2, procNull, nil))
				if xid := readXIDForTest(t, conn); xid != 2 {
					t.Fatalf("NULL barrier = %d", xid)
				}
				closeSignal(fs.release)
				if xid := readXIDForTest(t, conn); xid != 1 {
					t.Fatalf("original reply = %d", xid)
				}
				writeCallForTest(t, conn, rpcCallForTest(3, procNull, nil))
				if xid := readXIDForTest(t, conn); xid != 3 {
					t.Fatalf("duplicate produced another reply: %d", xid)
				}
			}
			if fs.calls.Load() != 1 {
				t.Fatalf("duplicate mutation executions = %d", fs.calls.Load())
			}
		})
	}
}

func TestConnectionHalfCloseDrainsReply(t *testing.T) {
	srv, fs := newConnectionTestServer(t, 2)
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	defer closeSignal(fs.release)
	conn := dial(t, addr)
	defer conn.Close()
	writeCallForTest(t, conn, statCallForTest(1))
	awaitSignal(t, fs.entered, "stat before half-close")
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	closeSignal(fs.release)
	if xid := readXIDForTest(t, conn); xid != 1 {
		t.Fatalf("drained reply = %d", xid)
	}
	if _, err := readFrame(conn); err != io.EOF {
		t.Fatalf("after drain = %v", err)
	}
}

type replyConn struct {
	net.Conn
	writes     atomic.Int32
	panicWrite bool
}

func (c *replyConn) Write(data []byte) (int, error) {
	c.writes.Add(1)
	if c.panicWrite {
		panic("injected reply panic")
	}
	return c.Conn.Write(data)
}

func TestConnectionTermination(t *testing.T) {
	for _, mode := range []string{"disconnect", "read-timeout", "write-timeout", "write-panic"} {
		t.Run(mode, func(t *testing.T) {
			srv, fs := newConnectionTestServer(t, 2)
			srv.idleTimeout = 0
			srv.writeTimeout = 25 * time.Millisecond
			if mode == "read-timeout" {
				srv.idleTimeout = 25 * time.Millisecond
				srv.writeTimeout = time.Second
			}
			server, client := net.Pipe()
			conn := &replyConn{Conn: server, panicWrite: mode == "write-panic"}
			done := make(chan struct{})
			go func() { srv.handleConn(conn); close(done) }()
			defer client.Close()
			defer server.Close()
			defer closeSignal(fs.release)
			writeCallForTest(t, client, statCallForTest(1))
			awaitSignal(t, fs.entered, "active stat")
			if mode == "write-timeout" {
				writeCallForTest(t, client, statCallForTest(2))
				awaitSignal(t, fs.entered, "second active stat")
			}
			if mode == "disconnect" {
				client.Close()
			}
			if mode == "read-timeout" {
				time.Sleep(60 * time.Millisecond)
			}
			closeSignal(fs.release)
			if mode == "read-timeout" {
				if xid := readXIDForTest(t, client); xid != 1 {
					t.Fatalf("reply after read timeout = %d", xid)
				}
			}
			awaitSignal(t, done, "connection handler exit")
			// With no reader on the pipe, the first frame's header write
			// times out. Later workers must skip their writes altogether.
			if mode == "write-timeout" && conn.writes.Load() != 1 {
				t.Fatalf("writes after failure = %d", conn.writes.Load())
			}
		})
	}
}
