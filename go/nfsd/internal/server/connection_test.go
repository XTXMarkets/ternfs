// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func rpcCallForTest(xid, procedure uint32, body []byte) []byte {
	call := make([]byte, 40)
	for i, value := range []uint32{xid, rpcCall, rpcVersion, nfsProg, nfsVersion, procedure} {
		binary.BigEndian.PutUint32(call[i*4:], value)
	}
	return append(call, body...)
}

// connectionSession is a session created directly in a server, so raw-frame
// tests can send compounds without a client handshake.
func connectionSession(t *testing.T, srv *Server) *testSession {
	t.Helper()
	// Compounds on the server may start lease workers; stop them before
	// the test's directories go.
	t.Cleanup(srv.clients.StopLeaseWorkers)
	// Its client is not in the store: give it a fixed live lease.
	srv.clients.PinLeaseForTest(1)
	srv.sessions.mu.Lock()
	defer srv.sessions.mu.Unlock()
	sess := &session{
		clientID:  1,
		slots:     make([]sessionSlot, maxSessionSlots),
		maxOps:    maxCompoundOperations,
		maxReq:    maxRPCMessage,
		maxResp:   maxRPCMessage,
		maxCached: maxSessionReplyCached,
	}
	sess.id = srv.sessions.newSessionIDLocked(sess.clientID)
	srv.sessions.insertSessionLocked(sess)
	return newTestSession(sess.id, sess.clientID, maxSessionSlots)
}

// sessionCallForTest uses a slot per xid, so calls which are in flight
// together do not share a slot.
func sessionCallForTest(ts *testSession, xid uint32, body []byte) []byte {
	slot := xid % uint32(len(ts.seqs))
	ts.mu.Lock()
	ts.seqs[slot]++
	seq := ts.seqs[slot]
	ts.mu.Unlock()
	return rpcCallForTest(xid, procCompound, withSequence(body, ts, slot, seq))
}

func statCallForTest(ts *testSession, xid uint32) []byte {
	return sessionCallForTest(ts, xid, buildCompoundBody([]byte(fmt.Sprint(xid)), func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		attr := w.AppendArgarray_Getattr()
		bits := attr.StartAttrRequest()
		bits.AppendData(1 << nfs4.FATTR4_TYPE)
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
	if err := conn.SetReadDeadline(time.Now().Add(testutil.ChannelTimeout)); err != nil {
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
		res, ok := nfs4.ReadCOMPOUND4res(reply[24:])
		if !ok || res.Status() != nfs4.NFS4_OK {
			t.Fatalf("COMPOUND failed: %x", reply)
		}
	}
	return binary.BigEndian.Uint32(reply)
}

type statGateVFS struct {
	vfs.TernVFS
	*testutil.Gate
	active    atomic.Int32
	maxActive atomic.Int32
}

func (fs *statGateVFS) Stat(id vfs.InodeID) (vfs.NodeInfo, error) {
	active := fs.active.Add(1)
	defer fs.active.Add(-1)
	for old := fs.maxActive.Load(); active > old; old = fs.maxActive.Load() {
		if fs.maxActive.CompareAndSwap(old, active) {
			break
		}
	}
	fs.Wait()
	return fs.TernVFS.Stat(id)
}

func newConnectionTestServer(t *testing.T, limit int) (*Server, *statGateVFS) {
	t.Helper()
	fs := &statGateVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		Gate: testutil.NewGate()}
	fs.All = true
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.MaxInFlightPerConn = limit
	srv.writeTimeout = time.Second
	useTestWireEpoch(srv) // Some tests call handleConn directly.
	return srv, fs
}

func TestConnectionConcurrentCompounds(t *testing.T) {
	for _, limit := range []int{1, 2} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			srv, fs := newConnectionTestServer(t, limit)
			ts := connectionSession(t, srv)
			fs.All = false
			addr, cleanup := serveTestServer(t, srv)
			defer cleanup()
			defer testutil.CloseSignal(fs.Release)
			conn := dial(t, addr)
			defer conn.Close()
			writeCallForTest(t, conn, statCallForTest(ts, 1))
			testutil.AwaitSignal(t, fs.Entered, "slow stat")
			writeCallForTest(t, conn, statCallForTest(ts, 2))
			if limit == 1 {
				conn.SetReadDeadline(time.Now().Add(25 * time.Millisecond))
				if _, err := readFrame(conn); err == nil {
					t.Fatal("reply bypassed limit of one")
				}
				testutil.CloseSignal(fs.Release)
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
				testutil.CloseSignal(fs.Release)
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
	ts := connectionSession(t, srv)
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	defer testutil.CloseSignal(fs.Release)
	conn := dial(t, addr)
	defer conn.Close()
	for i := 1; i <= count; i++ {
		writeCallForTest(t, conn, statCallForTest(ts, uint32(i)))
	}
	for i := 0; i < limit; i++ {
		testutil.AwaitSignal(t, fs.Entered, "admitted stat")
	}
	testutil.AssertBlocked(t, fs.Entered, "admission beyond the limit")
	testutil.CloseSignal(fs.Release)
	seen := make(map[uint32]bool)
	for i := 0; i < count; i++ {
		xid := readXIDForTest(t, conn)
		if xid < 1 || xid > count || seen[xid] {
			t.Fatalf("unexpected or duplicate xid %d", xid)
		}
		seen[xid] = true
	}
	if fs.maxActive.Load() > limit || fs.Calls.Load() != count {
		t.Fatalf("maximum=%d calls=%d", fs.maxActive.Load(), fs.Calls.Load())
	}
}

type mkdirGateVFS struct {
	vfs.TernVFS
	*testutil.Gate
}

func (fs *mkdirGateVFS) Mkdir(dir vfs.InodeID, name string) (vfs.InodeID, error) {
	if name == "once" {
		fs.Wait()
	}
	return fs.TernVFS.Mkdir(dir, name)
}

func TestConnectionInFlightDuplicates(t *testing.T) {
	for _, conflicting := range []bool{false, true} {
		t.Run(fmt.Sprint(conflicting), func(t *testing.T) {
			fs := &mkdirGateVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
				Gate: testutil.NewGate()}
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
			defer testutil.CloseSignal(fs.Release)
			conn := dial(t, addr)
			defer conn.Close()
			body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
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
			call := sessionCallForTest(connectionSession(t, srv), 1, body)
			writeCallForTest(t, conn, call)
			testutil.AwaitSignal(t, fs.Entered, "directory creation")
			if conflicting {
				writeCallForTest(t, conn, rpcCallForTest(1, procNull, nil))
				conn.SetReadDeadline(time.Now().Add(testutil.ChannelTimeout))
				if _, err := readFrame(conn); err == nil {
					t.Fatal("conflicting XID did not close connection")
				}
			} else {
				writeCallForTest(t, conn, call)
				writeCallForTest(t, conn, rpcCallForTest(2, procNull, nil))
				if xid := readXIDForTest(t, conn); xid != 2 {
					t.Fatalf("NULL barrier = %d", xid)
				}
				testutil.CloseSignal(fs.Release)
				if xid := readXIDForTest(t, conn); xid != 1 {
					t.Fatalf("original reply = %d", xid)
				}
				writeCallForTest(t, conn, rpcCallForTest(3, procNull, nil))
				if xid := readXIDForTest(t, conn); xid != 3 {
					t.Fatalf("duplicate produced another reply: %d", xid)
				}
			}
			if fs.Calls.Load() != 1 {
				t.Fatalf("duplicate mutation executions = %d", fs.Calls.Load())
			}
		})
	}
}

func TestConnectionHalfCloseDrainsReply(t *testing.T) {
	srv, fs := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	addr, cleanup := serveTestServer(t, srv)
	defer cleanup()
	defer testutil.CloseSignal(fs.Release)
	conn := dial(t, addr)
	defer conn.Close()
	writeCallForTest(t, conn, statCallForTest(ts, 1))
	testutil.AwaitSignal(t, fs.Entered, "stat before half-close")
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	testutil.CloseSignal(fs.Release)
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
			ts := connectionSession(t, srv)
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
			defer testutil.CloseSignal(fs.Release)
			writeCallForTest(t, client, statCallForTest(ts, 1))
			testutil.AwaitSignal(t, fs.Entered, "active stat")
			if mode == "write-timeout" {
				writeCallForTest(t, client, statCallForTest(ts, 2))
				testutil.AwaitSignal(t, fs.Entered, "second active stat")
			}
			if mode == "disconnect" {
				client.Close()
			}
			if mode == "read-timeout" {
				time.Sleep(60 * time.Millisecond)
			}
			testutil.CloseSignal(fs.Release)
			if mode == "read-timeout" {
				if xid := readXIDForTest(t, client); xid != 1 {
					t.Fatalf("reply after read timeout = %d", xid)
				}
			}
			testutil.AwaitSignal(t, done, "connection handler exit")
			// With no reader on the pipe, the first frame's header write
			// times out. Later workers must skip their writes altogether.
			if mode == "write-timeout" && conn.writes.Load() != 1 {
				t.Fatalf("writes after failure = %d", conn.writes.Load())
			}
		})
	}
}
