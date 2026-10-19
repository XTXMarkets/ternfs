// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// A served compound shows up in the counters: one count per operation and
// status, a latency observation per operation, and the bytes a READ returned.
func TestMetricsCountServedOperations(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	srv, addr, cleanup := startTestServerWithServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	status, reader, fh, _ := openFileForOwner(t, conn, &xid, clientID, "reader", "file.txt",
		nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	readFileData(t, conn, &xid, fh, 0, 16)
	closeFile(t, conn, &xid, fh, reader)

	snap := srv.MetricsSnapshot()
	count := func(op, status string) uint64 {
		for _, c := range snap.Ops {
			if c.Op == op && c.Status == status {
				return c.Count
			}
		}
		return 0
	}
	for _, op := range []string{"OP_OPEN", "OP_READ", "OP_CLOSE", "OP_SEQUENCE"} {
		if count(op, "NFS4_OK") == 0 {
			t.Errorf("%s not counted: %+v", op, snap.Ops)
		}
	}
	if snap.Reads != 1 || snap.ReadBytes != 4 {
		t.Errorf("reads=%d bytes=%d, want 1 and 4", snap.Reads, snap.ReadBytes)
	}
	if snap.Compounds == 0 || snap.Connections != 1 || snap.ConnAccepted != 1 {
		t.Errorf("compounds=%d connections=%d accepted=%d", snap.Compounds, snap.Connections, snap.ConnAccepted)
	}
	if snap.Opens != 0 {
		t.Errorf("opens=%d after CLOSE", snap.Opens)
	}
	var read *OpLatencySnapshot
	for i := range snap.Latency {
		if snap.Latency[i].Op == "OP_READ" {
			read = &snap.Latency[i]
		}
	}
	if read == nil || read.Count != 1 || len(read.Cumulative) != len(OpLatencyBounds)+1 ||
		read.Cumulative[len(read.Cumulative)-1] != 1 {
		t.Fatalf("READ latency %+v", read)
	}
}

// Observations land in the bucket of the first bound at or above them, and
// the snapshot's cumulative counts are non-decreasing and end at the total.
func TestMetricsLatencyBuckets(t *testing.T) {
	m := newMetrics()
	m.observeOp(nfs4.OP_WRITE, nfs4.NFS4_OK, 200*time.Microsecond) // <= 0.0005
	m.observeOp(nfs4.OP_WRITE, nfs4.NFS4_OK, 3*time.Millisecond)   // <= 0.005
	m.observeOp(nfs4.OP_WRITE, nfs4.NFS4_OK, 2*time.Minute)        // overflow
	l := m.latency[nfs4.OP_WRITE]
	if l.buckets[0] != 1 || l.buckets[2] != 1 || l.buckets[len(OpLatencyBounds)] != 1 || l.count != 3 {
		t.Fatalf("buckets %v count %d", l.buckets, l.count)
	}
}

type opKey struct{ op, status string }

// opTotals reads the cumulative operation and latency counts, as exported.
func opTotals(srv *Server) (map[opKey]uint64, map[string]uint64) {
	snap := srv.MetricsSnapshot()
	ops, latency := make(map[opKey]uint64), make(map[string]uint64)
	for _, c := range snap.Ops {
		ops[opKey{c.Op, c.Status}] = c.Count
	}
	for _, l := range snap.Latency {
		latency[l.Op] = l.Count
	}
	return ops, latency
}

// opDelta runs one request and returns the operation and latency counts it
// added.
func opDelta(srv *Server, run func()) (map[opKey]uint64, map[string]uint64) {
	ops0, latency0 := opTotals(srv)
	run()
	ops, latency := opTotals(srv)
	for k, n := range ops0 {
		if ops[k] -= n; ops[k] == 0 {
			delete(ops, k)
		}
	}
	for k, n := range latency0 {
		if latency[k] -= n; latency[k] == 0 {
			delete(latency, k)
		}
	}
	return ops, latency
}

// replyResults counts the results in a reply by operation and status. Every
// result begins with its opcode and status.
func replyResults(res nfs4.COMPOUND4res) map[opKey]uint64 {
	out := make(map[opKey]uint64)
	iter := res.Resarray()
	for iter.Next() {
		entry := iter.Resarray()
		out[opKey{nfs4.NfsOpnum4Name(entry.Disc()), nfs4.Nfsstat4Name(binary.BigEndian.Uint32(entry[4:8]))}]++
	}
	return out
}

// assertCountedOnce checks that a request counted each result in its reply
// exactly once, and one latency observation per result.
func assertCountedOnce(t *testing.T, res nfs4.COMPOUND4res, ops map[opKey]uint64, latency map[string]uint64) {
	t.Helper()
	want := replyResults(res)
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("counted %v, reply has %v", ops, want)
	}
	wantLatency := make(map[string]uint64)
	for k, n := range want {
		wantLatency[k.op] += n
	}
	if !reflect.DeepEqual(latency, wantLatency) {
		t.Fatalf("latency counts %v, want %v", latency, wantLatency)
	}
}

func requireResult(t *testing.T, res nfs4.COMPOUND4res, op, status uint32) {
	t.Helper()
	if replyResults(res)[opKey{nfs4.NfsOpnum4Name(op), nfs4.Nfsstat4Name(status)}] == 0 {
		t.Fatalf("reply lacks %s %s: %v", nfs4.NfsOpnum4Name(op), nfs4.Nfsstat4Name(status), replyResults(res))
	}
}

// An opcode the XDR does not define is counted as OP_ILLEGAL, as its result
// says on the wire, so a client cannot add series or invalid tag values.
func TestMetricsUnknownOpcodeIsIllegal(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	body := buildCompoundBody(nil, nil)
	binary.BigEndian.PutUint32(body[8:], 1)
	body = binary.BigEndian.AppendUint32(body, 9999)
	var res nfs4.COMPOUND4res
	ops, latency := opDelta(srv, func() { _, res = admissionRequest(t, srv, admissionSequence(body, ts, 0, 1, false), 1, nil) })
	requireResult(t, res, nfs4.OP_ILLEGAL, nfs4.NFS4ERR_OP_ILLEGAL)
	assertCountedOnce(t, res, ops, latency)

	// Sessionless, undefined operation zero.
	body = make([]byte, 16)
	binary.BigEndian.PutUint32(body[4:8], nfsMinorVersion)
	binary.BigEndian.PutUint32(body[8:12], 1)
	ops, latency = opDelta(srv, func() { _, res = admissionRequest(t, srv, body, 2, nil) })
	requireResult(t, res, nfs4.OP_ILLEGAL, nfs4.NFS4ERR_OP_ILLEGAL)
	assertCountedOnce(t, res, ops, latency)

	srv.metrics.observeOp(123456, nfs4.NFS4ERR_OP_ILLEGAL, time.Millisecond)
	snap := srv.MetricsSnapshot()
	for _, c := range snap.Ops {
		if strings.HasPrefix(c.Op, "nfs_opnum4(") {
			t.Fatalf("raw opcode counted: %+v", snap.Ops)
		}
	}
	for _, l := range snap.Latency {
		if strings.HasPrefix(l.Op, "nfs_opnum4(") {
			t.Fatalf("raw opcode timed: %+v", snap.Latency)
		}
	}
}

// An operation refused before it runs because its result would not fit the
// response or cache limit is counted with that status. SEQUENCE and the
// failed result fit in 88 bytes; a GETFH result does not.
func TestMetricsCountResponseLimitRefusals(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprint(cached), func(t *testing.T) {
			srv, _ := newConnectionTestServer(t, 2)
			ts := connectionSession(t, srv)
			want := uint32(nfs4.NFS4ERR_REP_TOO_BIG)
			srv.sessions.mu.Lock()
			sess := srv.sessions.sessions[ts.id]
			if cached {
				sess.maxCached = 88
				want = nfs4.NFS4ERR_REP_TOO_BIG_TO_CACHE
			} else {
				sess.maxResp = 88
			}
			srv.sessions.mu.Unlock()
			wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Getfh() }), ts, 0, 1, cached)
			var res nfs4.COMPOUND4res
			ops, latency := opDelta(srv, func() { _, res = admissionRequest(t, srv, wire, 1, nil) })
			admissionStatus(t, res, want, nfs4.OP_SEQUENCE, nfs4.OP_GETFH)
			requireResult(t, res, nfs4.OP_GETFH, want)
			assertCountedOnce(t, res, ops, latency)
		})
	}
}

// Every other refusal before execution is counted once, with the results
// before it.
func TestMetricsCountPreDispatchErrors(t *testing.T) {
	putroot := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	for _, test := range []struct {
		name       string
		op, status uint32
		wire       func(ts *testSession) []byte
		flavor     uint32
		cred       []byte
	}{
		{name: "unsupported-flavor", op: nfs4.OP_SEQUENCE, status: nfs4.NFS4ERR_WRONGSEC, flavor: 99,
			wire: func(ts *testSession) []byte { return admissionSequence(putroot, ts, 0, 1, false) }},
		{name: "truncated-auth-sys", op: nfs4.OP_SEQUENCE, status: nfs4.NFS4ERR_WRONGSEC, flavor: nfs4.AUTH_SYS, cred: make([]byte, 8),
			wire: func(ts *testSession) []byte { return admissionSequence(putroot, ts, 0, 1, false) }},
		{name: "not-in-session", op: nfs4.OP_PUTROOTFH, status: nfs4.NFS4ERR_OP_NOT_IN_SESSION,
			wire: func(*testSession) []byte { return putroot }},
		{name: "sequence-position", op: nfs4.OP_SEQUENCE, status: nfs4.NFS4ERR_SEQUENCE_POS,
			wire: func(ts *testSession) []byte {
				second := admissionSequence(buildCompoundBody(nil, nil), ts, 1, 1, false)
				return admissionSequence(admissionJoin(putroot, second), ts, 0, 1, false)
			}},
		{name: "maintenance-position", op: nfs4.OP_DESTROY_SESSION, status: nfs4.NFS4ERR_NOT_ONLY_OP,
			wire: func(ts *testSession) []byte {
				return admissionSequence(admissionJoin(admissionDestroy(ts.id), putroot), ts, 0, 1, false)
			}},
		{name: "handle-obligation", op: nfs4.OP_CREATE, status: nfs4.NFS4ERR_UNSAFE_COMPOUND,
			wire: func(ts *testSession) []byte {
				return admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
					w.AppendArgarray_Putrootfh()
					admissionCreate(w, "dir")
					w.AppendArgarray_Readlink()
					w.AppendArgarray_Getfh()
				}), ts, 0, 1, false)
			}},
	} {
		t.Run(test.name, func(t *testing.T) {
			srv, _ := newConnectionTestServer(t, 2)
			ts := connectionSession(t, srv)
			req, err := parseRPCCall(rpcCallForTest(1, procCompound, test.wire(ts)))
			if err != nil {
				t.Fatal(err)
			}
			if test.flavor != 0 {
				req.credFlavor, req.credBody = test.flavor, test.cred
			}
			var res nfs4.COMPOUND4res
			ops, latency := opDelta(srv, func() {
				reply := bytes.Clone(srv.safeHandleCompound(req, "test"))
				req.releaseReply()
				if res, err = parseCompoundReply(reply); err != nil {
					t.Fatal(err)
				}
			})
			requireResult(t, res, test.op, test.status)
			assertCountedOnce(t, res, ops, latency)
		})
	}
}

type statPanicVFS struct{ vfs.TernVFS }

func (fs statPanicVFS) Stat(id vfs.InodeID) (vfs.NodeInfo, error) {
	panic("injected panic in Stat")
}

// A panic while an operation runs, before it has a result, is counted once
// as that operation's SERVERFAULT.
func TestMetricsCountPanicDuringOperation(t *testing.T) {
	srv, err := NewServer(statPanicVFS{vfs.NewLocalTernVFS(t.TempDir())}, staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	useTestWireEpoch(srv)
	ts := connectionSession(t, srv)
	var res nfs4.COMPOUND4res
	ops, latency := opDelta(srv, func() {
		_, res = admissionRequest(t, srv, statCallForTest(ts, 1)[40:], 1, nil)
	})
	if res.Status() != nfs4.NFS4ERR_SERVERFAULT {
		t.Fatalf("status %s, want SERVERFAULT", nfs4.Nfsstat4Name(res.Status()))
	}
	assertCountedOnce(t, res, ops, latency)
}

// panicOnDone is a log handler that panics once on the "COMPOUND done"
// record, which compoundBody logs after its last result is pending and
// before the reply is finished.
type panicOnDone struct {
	armed, fired *atomic.Bool
}

func (h panicOnDone) Enabled(context.Context, slog.Level) bool { return true }
func (h panicOnDone) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h panicOnDone) WithGroup(string) slog.Handler            { return h }
func (h panicOnDone) Handle(_ context.Context, r slog.Record) error {
	if r.Message == "COMPOUND done" && h.armed.CompareAndSwap(true, false) {
		h.fired.Store(true)
		panic("injected panic finishing the compound")
	}
	return nil
}

// A panic after the last operation's result is pending replaces that result
// with SERVERFAULT, and it is counted once, as SERVERFAULT, not with the
// status it had. The results before it are counted once each.
func TestMetricsCountLatePanicReplacesResult(t *testing.T) {
	for _, test := range []struct {
		name   string
		op     uint32
		before uint32 // the result's status before the panic
		build  func(w *nfs4.COMPOUND4argsWriter)
	}{
		{"success", nfs4.OP_GETFH, nfs4.NFS4_OK, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Getfh() }},
		{"error", nfs4.OP_LOOKUP, nfs4.NFS4ERR_NOENT, func(w *nfs4.COMPOUND4argsWriter) { admissionLookup(w, "missing") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var armed, fired atomic.Bool
			srv, err := NewServer(vfs.NewLocalTernVFS(t.TempDir()), staging.ReadOnlyStore{}, slog.New(panicOnDone{&armed, &fired}))
			if err != nil {
				t.Fatal(err)
			}
			useTestWireEpoch(srv)
			ts := connectionSession(t, srv)
			wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
				w.AppendArgarray_Putrootfh()
				test.build(w)
			}), ts, 0, 1, false)
			var res nfs4.COMPOUND4res
			armed.Store(true)
			ops, latency := opDelta(srv, func() { _, res = admissionRequest(t, srv, wire, 1, nil) })
			if !fired.Load() {
				t.Fatal("the COMPOUND done record was not logged; the injection point moved")
			}
			admissionStatus(t, res, nfs4.NFS4ERR_SERVERFAULT, nfs4.OP_SEQUENCE, nfs4.OP_PUTROOTFH, test.op)
			requireResult(t, res, nfs4.OP_SEQUENCE, nfs4.NFS4_OK)
			requireResult(t, res, nfs4.OP_PUTROOTFH, nfs4.NFS4_OK)
			requireResult(t, res, test.op, nfs4.NFS4ERR_SERVERFAULT)
			assertCountedOnce(t, res, ops, latency)
			if n := ops[opKey{nfs4.NfsOpnum4Name(test.op), nfs4.Nfsstat4Name(test.before)}]; n != 0 {
				t.Fatalf("replaced result also counted as %s", nfs4.Nfsstat4Name(test.before))
			}
			if latency[nfs4.NfsOpnum4Name(test.op)] != 1 {
				t.Fatalf("%s latency counted %d times", nfs4.NfsOpnum4Name(test.op), latency[nfs4.NfsOpnum4Name(test.op)])
			}
		})
	}
}

// A replayed reply counts its SEQUENCE only: the cached results were
// counted when they ran.
func TestMetricsReplayCountsSequenceOnly(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ts := connectionSession(t, srv)
	wire := admissionSequence(buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() }), ts, 0, 1, true)
	var res nfs4.COMPOUND4res
	ops, latency := opDelta(srv, func() { _, res = admissionRequest(t, srv, wire, 1, nil) })
	assertCountedOnce(t, res, ops, latency)
	ops, latency = opDelta(srv, func() { _, res = admissionRequest(t, srv, wire, 2, nil) })
	requireResult(t, res, nfs4.OP_PUTROOTFH, nfs4.NFS4_OK)
	want := map[opKey]uint64{{"OP_SEQUENCE", "NFS4_OK"}: 1}
	if !reflect.DeepEqual(ops, want) || !reflect.DeepEqual(latency, map[string]uint64{"OP_SEQUENCE": 1}) {
		t.Fatalf("replay counted %v, latency %v; want SEQUENCE once", ops, latency)
	}
}
