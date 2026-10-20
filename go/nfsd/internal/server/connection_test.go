// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"syscall"
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
	sess.id, _ = srv.sessions.newSessionIDLocked(sess.clientID)
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
	return newConnectionTestServerWithLog(t, limit, nil)
}

func newConnectionTestServerWithLog(t *testing.T, limit int, logger *slog.Logger) (*Server, *statGateVFS) {
	t.Helper()
	fs := &statGateVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()),
		Gate: testutil.NewGate()}
	fs.All = true
	srv, err := NewServer(fs, staging.ReadOnlyStore{}, logger)
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
			// The reader logs "client disconnected" after it has recorded EOF
			// as the cause; a failed reply write logs "RPC reply failed".
			logs := newLogSignals("client disconnected", "RPC reply failed")
			srv, fs := newConnectionTestServerWithLog(t, 2, slog.New(logs))
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
				// Release the stat only once EOF is the recorded cause, so the
				// drained reply's write fails second.
				testutil.AwaitSignal(t, logs["client disconnected"].ch, "EOF recorded as the cause")
			}
			if mode == "read-timeout" {
				// The idle timeout closes the connection before the drain,
				// while the stat is still held, so its reply is never written.
				awaitServerClose(t, client, "read timeout")
			}
			testutil.CloseSignal(fs.Release)
			testutil.AwaitSignal(t, done, "connection handler exit")
			// With no reader on the pipe, the first frame's header write
			// times out. Later workers must skip their writes altogether.
			if mode == "write-timeout" && conn.writes.Load() != 1 {
				t.Fatalf("writes after failure = %d", conn.writes.Load())
			}
			// After a disconnect the drained reply's write fails (on the
			// pipe, already at its write deadline), after EOF was recorded.
			if mode == "disconnect" && !logs.logged("RPC reply failed") {
				t.Fatal("the drained reply's write did not fail")
			}
			// Disconnect reads EOF and then fails to write the drained
			// reply: the first cause is kept.
			assertConnectionEnds(t, srv, map[string]string{
				"disconnect": "client", "read-timeout": "idle", "write-timeout": "server", "write-panic": "server",
			}[mode])
		})
	}
}

// logSignals is a log handler that closes a message's channel the first time
// a record with that message is logged, at any level.
type logSignals map[string]*logSignal

type logSignal struct {
	ch   chan struct{}
	once sync.Once
}

func newLogSignals(messages ...string) logSignals {
	s := make(logSignals)
	for _, m := range messages {
		s[m] = &logSignal{ch: make(chan struct{})}
	}
	return s
}

func (s logSignals) logged(message string) bool {
	select {
	case <-s[message].ch:
		return true
	default:
		return false
	}
}

func (s logSignals) Enabled(context.Context, slog.Level) bool { return true }
func (s logSignals) WithAttrs([]slog.Attr) slog.Handler       { return s }
func (s logSignals) WithGroup(string) slog.Handler            { return s }
func (s logSignals) Handle(_ context.Context, r slog.Record) error {
	if sig := s[r.Message]; sig != nil {
		sig.once.Do(func() { close(sig.ch) })
	}
	return nil
}

// awaitServerClose reads on the client side until the server closes the
// connection. A reply fails the test, and so does the client's own read
// deadline expiring, which is not a close.
func awaitServerClose(t *testing.T, client net.Conn, description string) {
	t.Helper()
	if err := client.SetReadDeadline(time.Now().Add(testutil.ChannelTimeout)); err != nil {
		// A pipe refuses a deadline once the server has closed its end,
		// which is the close being waited for. It holds no unread reply:
		// a pipe write completes only when it is read.
		if errors.Is(err, io.ErrClosedPipe) {
			return
		}
		t.Fatal(err)
	}
	frame, err := readFrame(client)
	if err == nil {
		t.Fatalf("%s: reply %x", description, frame)
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("%s: the server did not close the connection", description)
	}
}

// A server that has already closed its end, before the client starts
// waiting, counts as closed.
func TestAwaitServerCloseAlreadyClosed(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	awaitServerClose(t, client, "server closed before waiting")
}

// assertConnectionEnds checks that every connection the server accepted,
// all of whose handlers have returned, ended exactly once, all with cause.
func assertConnectionEnds(t *testing.T, srv *Server, cause string) {
	t.Helper()
	s := srv.MetricsSnapshot()
	got := map[string]uint64{
		"client": s.ConnClosedByClient, "server": s.ConnClosedByServer, "idle": s.ConnIdleTimeouts,
		"read": s.ConnReadErrors, "parse": s.ParseErrors,
	}
	want := map[string]uint64{"client": 0, "server": 0, "idle": 0, "read": 0, "parse": 0}
	want[cause] = s.ConnAccepted
	if s.ConnAccepted == 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("%d connections accepted, ends %v, want all %s", s.ConnAccepted, got, cause)
	}
}

// pinReplyWork holds one request's reply work and lowers the budget to it,
// so further requests of the same shape wait for reply work. release frees
// the held work; it runs at cleanup if the test has not called it.
func pinReplyWork(t *testing.T, srv *Server, ts *testSession) (release func()) {
	t.Helper()
	req, err := parseRPCCall(statCallForTest(ts, 1000))
	if err != nil {
		t.Fatal(err)
	}
	if !req.acquireReplyWork(srv) {
		t.Fatal("pinning request refused")
	}
	ss := srv.sessions
	ss.cacheMu.Lock()
	ss.workLimit = ss.workBytes
	ss.cacheMu.Unlock()
	var once sync.Once
	release = func() { once.Do(req.releaseReply) }
	t.Cleanup(release)
	return release
}

// awaitWorkWaiters waits until n requests are waiting for reply work.
func awaitWorkWaiters(t *testing.T, srv *Server, n int64) {
	t.Helper()
	deadline := time.Now().Add(testutil.ChannelTimeout)
	for srv.metrics.workWaiters.Load() != n {
		if time.Now().After(deadline) {
			t.Fatalf("%d requests waiting for reply work, want %d", srv.metrics.workWaiters.Load(), n)
		}
		time.Sleep(time.Millisecond)
	}
}

// failingConn injects connection failures. Once the peer has closed, Read
// returns readErr in place of the pipe's error. SetReadDeadline fails from
// its failDeadline'th call. It counts writes.
type failingConn struct {
	net.Conn
	readErr      error
	failDeadline int32
	deadlines    atomic.Int32
	writes       atomic.Int32
}

func (c *failingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if err != nil && c.readErr != nil {
		err = c.readErr
	}
	return n, err
}

func (c *failingConn) SetReadDeadline(t time.Time) error {
	if c.failDeadline > 0 && c.deadlines.Add(1) >= c.failDeadline {
		return errors.New("injected SetReadDeadline failure")
	}
	return c.Conn.SetReadDeadline(t)
}

func (c *failingConn) Write(p []byte) (int, error) {
	c.writes.Add(1)
	return c.Conn.Write(p)
}

// A read failure other than EOF, or an RPC that does not parse, closes the
// connection before the drain: requests waiting for reply work give up at
// once instead of holding their frames until the budget frees, nothing
// executes or is written, and the connection is counted once, by its
// cause, not once per refused waiter.
func TestConnectionFatalReadReleasesWaiters(t *testing.T) {
	for _, test := range []struct {
		name, cause string
		readErr     error
		deadline    int32
		malformed   bool
	}{
		{"idle-timeout", "idle", nil, 0, false},
		{"reset", "client", syscall.ECONNRESET, 0, false},
		{"read-error", "read", errors.New("injected read failure"), 0, false},
		{"set-read-deadline", "read", nil, 2, false},
		{"parse-error", "parse", nil, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			srv, fs := newConnectionTestServer(t, 4)
			testutil.CloseSignal(fs.Release) // Stat is counted, never held.
			ts := connectionSession(t, srv)
			pinReplyWork(t, srv, ts)
			waiters := int64(2)
			if test.name == "idle-timeout" {
				srv.idleTimeout = 50 * time.Millisecond
			}
			if test.deadline > 0 {
				waiters = 1 // The second deadline fails before a second frame is read.
			}
			server, client := net.Pipe()
			defer client.Close()
			conn := &failingConn{Conn: server, readErr: test.readErr, failDeadline: test.deadline}
			done := make(chan struct{})
			go func() { srv.handleConn(conn); close(done) }()
			for xid := uint32(1); xid <= uint32(waiters); xid++ {
				writeCallForTest(t, client, statCallForTest(ts, xid))
			}
			if test.deadline == 0 {
				awaitWorkWaiters(t, srv, waiters)
			}
			if test.readErr != nil {
				client.Close() // The server's Read fails with readErr.
			}
			if test.malformed {
				// A CALL too short to parse. The client stays connected.
				writeCallForTest(t, client, []byte{0, 0, 0, 9, 0, 0, 0, 0})
			}
			// The budget is still held: only the close can release the waiters.
			testutil.AwaitSignal(t, done, "connection handler exit with the budget held")
			if n := srv.metrics.workWaiters.Load(); n != 0 {
				t.Fatalf("%d requests still waiting for reply work", n)
			}
			if fs.Calls.Load() != 0 || conn.writes.Load() != 0 {
				t.Fatalf("after the failure %d requests executed and %d writes were made", fs.Calls.Load(), conn.writes.Load())
			}
			assertConnectionEnds(t, srv, test.cause)
		})
	}
}

// A clean EOF may be a half-close: requests waiting for reply work are
// still admitted once the budget frees, and answered, before the connection
// closes. It is counted once, as closed by the client.
func TestConnectionEOFDrainsWaiters(t *testing.T) {
	srv, fs := newConnectionTestServer(t, 4)
	testutil.CloseSignal(fs.Release)
	ts := connectionSession(t, srv)
	release := pinReplyWork(t, srv, ts)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if conn, err := ln.Accept(); err == nil {
			srv.handleConn(conn)
		}
	}()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	writeCallForTest(t, client, statCallForTest(ts, 1))
	writeCallForTest(t, client, statCallForTest(ts, 2))
	awaitWorkWaiters(t, srv, 2)
	if err := client.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	testutil.AssertBlocked(t, done, "connection handler after a half-close")
	release()
	replies := map[uint32]bool{readXIDForTest(t, client): true, readXIDForTest(t, client): true}
	if !replies[1] || !replies[2] {
		t.Fatalf("replies %v, want 1 and 2", replies)
	}
	if _, err := readFrame(client); err != io.EOF {
		t.Fatalf("after the replies: %v", err)
	}
	testutil.AwaitSignal(t, done, "connection handler exit")
	assertConnectionEnds(t, srv, "client")
}

// A request larger than the whole reply-work budget is refused and the
// connection closed: one server close, nothing else.
func TestConnectionRefusedRequestEndsOnce(t *testing.T) {
	srv, fs := newConnectionTestServer(t, 2)
	testutil.CloseSignal(fs.Release)
	ts := connectionSession(t, srv)
	srv.SetReplyWorkBudget(1)
	server, client := net.Pipe()
	defer client.Close()
	done := make(chan struct{})
	go func() { srv.handleConn(server); close(done) }()
	writeCallForTest(t, client, statCallForTest(ts, 1))
	awaitServerClose(t, client, "refused request")
	testutil.AwaitSignal(t, done, "connection handler exit")
	if fs.Calls.Load() != 0 {
		t.Fatal("the refused request executed")
	}
	assertConnectionEnds(t, srv, "server")
}

// Admission is decided when work would be charged. A request whose
// connection has closed is refused even when capacity is free, both when
// capacity frees while it waits and when it arrives after the close; an
// open connection with free capacity is admitted, as a control.
func TestReplyWorkRefusesClosedConnection(t *testing.T) {
	type state struct {
		bytes, replies int
		waits, nanos   uint64
		compounds      uint64
	}
	read := func(srv *Server) state {
		ss := srv.sessions
		ss.cacheMu.Lock()
		defer ss.cacheMu.Unlock()
		return state{ss.workBytes, ss.workReplies, srv.metrics.workWaits.Load(), srv.metrics.workWaitNanos.Load(), srv.metrics.compounds.Load()}
	}
	setup := func(t *testing.T) (*Server, *statGateVFS, *testSession) {
		srv, fs := newConnectionTestServer(t, 2)
		testutil.CloseSignal(fs.Release)
		return srv, fs, connectionSession(t, srv)
	}
	request := func(t *testing.T, ts *testSession, xid uint32, closed chan struct{}) *rpcRequest {
		req, err := parseRPCCall(statCallForTest(ts, xid))
		if err != nil {
			t.Fatal(err)
		}
		req.connection = &rpcConnection{closed: closed}
		return req
	}

	t.Run("closed-while-waiting-with-capacity-freed", func(t *testing.T) {
		srv, fs, ts := setup(t)
		release := pinReplyWork(t, srv, ts)
		closed := make(chan struct{})
		req := request(t, ts, 1, closed)
		before := read(srv)
		reply := make(chan []byte, 1)
		go func() { reply <- srv.handleRPC(req, "test") }()
		awaitWorkWaiters(t, srv, 1)
		// Close the connection and free capacity together, so the waiter
		// wakes to both at once.
		ss := srv.sessions
		ss.cacheMu.Lock()
		close(closed)
		ss.workLimit = maxReplyWorkBytes
		ss.workFreed.Broadcast()
		ss.cacheMu.Unlock()
		if r := testutil.AwaitValue(t, reply, "refused request"); r != nil {
			t.Fatal("a request on a closed connection was admitted and answered")
		}
		after := read(srv)
		if req.replyWork != nil || after.bytes != before.bytes || after.replies != before.replies {
			t.Fatalf("refused request holds work: lease %v, bytes %d -> %d, replies %d -> %d",
				req.replyWork != nil, before.bytes, after.bytes, before.replies, after.replies)
		}
		if srv.metrics.workWaiters.Load() != 0 || after.waits != before.waits+1 || after.nanos <= before.nanos {
			t.Fatalf("waiters %d, waits %d -> %d, wait nanos %d -> %d",
				srv.metrics.workWaiters.Load(), before.waits, after.waits, before.nanos, after.nanos)
		}
		if fs.Calls.Load() != 0 || after.compounds != before.compounds {
			t.Fatal("the refused request executed")
		}
		release()
	})

	t.Run("already-closed-with-free-capacity", func(t *testing.T) {
		srv, fs, ts := setup(t)
		closed := make(chan struct{})
		close(closed)
		req := request(t, ts, 1, closed)
		before := read(srv)
		if srv.handleRPC(req, "test") != nil {
			t.Fatal("a request on a closed connection was admitted and answered")
		}
		after := read(srv)
		if req.replyWork != nil || after != before || srv.metrics.workWaiters.Load() != 0 || fs.Calls.Load() != 0 {
			t.Fatalf("refused request changed state %+v -> %+v, lease %v, calls %d",
				before, after, req.replyWork != nil, fs.Calls.Load())
		}
	})

	t.Run("open-with-free-capacity", func(t *testing.T) {
		srv, fs, ts := setup(t)
		req := request(t, ts, 1, make(chan struct{}))
		before := read(srv)
		if srv.handleRPC(req, "test") == nil {
			t.Fatal("a request on an open connection was refused")
		}
		req.releaseReply()
		if fs.Calls.Load() != 1 || read(srv).compounds != before.compounds+1 {
			t.Fatalf("the admitted request did not execute: calls %d", fs.Calls.Load())
		}
	})
}

// An idle timeout closes the connection while a cached mutation is still
// executing. The mutation completes once and installs its reply, which a
// retry on another connection gets from the slot without repeating it.
func TestConnectionTimeoutKeepsExecutingEffect(t *testing.T) {
	fs := &mkdirGateVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()), Gate: testutil.NewGate()}
	srv := admissionWritableServer(t, fs)
	srv.idleTimeout = 50 * time.Millisecond
	ts := connectionSession(t, srv)
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		admissionCreate(w, "once")
	}), ts, 0, 1, true)
	server, client := net.Pipe()
	defer client.Close()
	defer testutil.CloseSignal(fs.Release)
	done := make(chan struct{})
	go func() { srv.handleConn(server); close(done) }()
	writeCallForTest(t, client, rpcCallForTest(1, procCompound, wire))
	testutil.AwaitSignal(t, fs.Entered, "directory creation")
	// The timeout closes the connection with the mutation still held. If it
	// does not, the bounded read fails the test and the deferred release
	// lets the handler drain.
	awaitServerClose(t, client, "idle timeout during the mutation")
	testutil.AssertBlocked(t, done, "connection handler while the mutation runs")
	testutil.CloseSignal(fs.Release)
	testutil.AwaitSignal(t, done, "connection handler exit")
	assertConnectionEnds(t, srv, "idle")
	_, res := admissionRequest(t, srv, wire, 2, &rpcConnection{})
	admissionStatus(t, res, nfs4.NFS4_OK, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, nfs4.OP_CREATE)
	if fs.Calls.Load() != 1 {
		t.Fatalf("directory creation ran %d times", fs.Calls.Load())
	}
}
