// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/openstate"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sessionstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// namespaceFaultSwitch makes the namespace effects fail, as the local
// backend fails them (NotApplied), while on. before and after, when set,
// run once around the next effect that reaches the backend; after reports
// whether it is done or waits for a later effect.
type namespaceFaultSwitch struct {
	vfs.TernVFS
	on atomic.Bool

	mu     sync.Mutex
	before func()
	after  func() bool
	// lookup, when set, runs before every LookupEdge.
	lookup func(name string)
}

func (f *namespaceFaultSwitch) LookupEdge(dir vfs.InodeID, name string) (vfs.Edge, error) {
	f.mu.Lock()
	lookup := f.lookup
	f.mu.Unlock()
	if lookup != nil {
		lookup(name)
	}
	return f.TernVFS.LookupEdge(dir, name)
}

func (f *namespaceFaultSwitch) effect(do func() error) error {
	if f.on.Load() {
		return os.ErrPermission
	}
	f.mu.Lock()
	before := f.before
	f.before = nil
	f.mu.Unlock()
	if before != nil {
		before()
	}
	err := do()
	f.mu.Lock()
	after := f.after
	f.mu.Unlock()
	if after != nil && after() {
		f.mu.Lock()
		f.after = nil
		f.mu.Unlock()
	}
	return err
}

func (f *namespaceFaultSwitch) RenameEdge(srcDir vfs.InodeID, oldName string, edge vfs.Edge, dstDir vfs.InodeID, newName string) error {
	return f.effect(func() error { return f.TernVFS.RenameEdge(srcDir, oldName, edge, dstDir, newName) })
}

func (f *namespaceFaultSwitch) RemoveEdge(dir vfs.InodeID, name string, edge vfs.Edge) error {
	return f.effect(func() error { return f.TernVFS.RemoveEdge(dir, name, edge) })
}

func (f *namespaceFaultSwitch) Mkdir(dir vfs.InodeID, name string) (id vfs.InodeID, err error) {
	err = f.effect(func() error { id, err = f.TernVFS.Mkdir(dir, name); return err })
	return id, err
}

func (f *namespaceFaultSwitch) Symlink(dir vfs.InodeID, name, target string) (id vfs.InodeID, err error) {
	err = f.effect(func() error { id, err = f.TernVFS.Symlink(dir, name, target); return err })
	return id, err
}

// replayFixture is a client with a session on a server whose staging
// directory, and so whose session files, survive a restart.
type replayFixture struct {
	t          *testing.T
	root       string
	stagingDir string
	fs         *namespaceFaultSwitch
	store      *staging.LocalStore
	srv        *Server
	addr       string
	stop       func()
	conn       net.Conn
	xid        uint32
	wire       uint64
	ts         *testSession
	// wrapStore, when set, wraps the staging store each server is given.
	wrapStore func(*staging.LocalStore) staging.Store
	// killed is the session file as fileAtEffect found it.
	killedMu sync.Mutex
	killed   *keptFile
}

// keptFile is a session file's content, or its absence.
type keptFile struct {
	data []byte
	gone bool
}

func newReplayFixture(t *testing.T) *replayFixture {
	t.Helper()
	f := newReplayServer(t)
	f.wire = setupClient(t, f.conn, &f.xid)
	f.ts = testSessionForClient(f.conn, f.wire)
	return f
}

// newSmallCacheReplayFixture is newReplayFixture with a session whose fore
// channel caches no reply, which the protocol allows.
func newSmallCacheReplayFixture(t *testing.T) *replayFixture {
	t.Helper()
	f := newReplayServer(t)
	clientID, seq, status, err := exchangeID(f.conn, f.xid, "small-cache-client", [8]byte{})
	f.xid++
	if err != nil || status != nfs4.NFS4_OK {
		t.Fatalf("EXCHANGE_ID %s %v", nfs4.Nfsstat4Name(status), err)
	}
	ts, status, err := createSessionCached(f.conn, f.xid, clientID, seq, 0)
	f.xid++
	if err != nil || status != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %s %v", nfs4.Nfsstat4Name(status), err)
	}
	f.wire, f.ts = clientID, ts
	return f
}

func newReplayServer(t *testing.T) *replayFixture {
	t.Helper()
	f := &replayFixture{t: t, root: t.TempDir(), stagingDir: t.TempDir(), xid: 1}
	f.fs = &namespaceFaultSwitch{TernVFS: vfs.NewLocalTernVFS(f.root)}
	f.start()
	return f
}

func (f *replayFixture) start() {
	t := f.t
	store, err := staging.NewLocalStore(f.stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.store = store
	var serverStore staging.Store = store
	if f.wrapStore != nil {
		serverStore = f.wrapStore(store)
	}
	srv, err := NewServer(f.fs, serverStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.srv = srv
	addr, stop := serveTestServer(t, srv)
	f.addr, f.stop = addr, stop
	f.conn = dial(t, addr)
	t.Cleanup(func() {
		f.conn.Close()
		if f.stop != nil {
			f.stop()
		}
	})
}

// restart kills the server and starts another over the same filesystem
// and staging directory. The client does not register again.
func (f *replayFixture) restart() {
	f.conn.Close()
	f.stop()
	f.stop = nil
	staging.CloseFiles(f.t, f.store)
	f.killedMu.Lock()
	killed := f.killed
	f.killed = nil
	f.killedMu.Unlock()
	path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	switch {
	case killed == nil:
	case killed.gone:
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			f.t.Fatal(err)
		}
	default:
		if err := os.WriteFile(path, killed.data, 0o600); err != nil {
			f.t.Fatal(err)
		}
	}
	f.fs.on.Store(false)
	f.start()
}

// fileAtEffect keeps the session file as it is when the n-th namespace
// effect from now returns, and restart puts it back: the new process finds
// the records the process had written by then and none written after. Only
// the session file is kept, so this stands for a kill at that effect only
// if nothing after it reaches the filesystem. With stop, every later effect
// fails before reaching it, until the restart; without, the effect must be
// the request's last.
func (f *replayFixture) fileAtEffect(n int, stop bool) {
	path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()
	f.fs.after = func() bool {
		if n--; n > 0 {
			return false
		}
		f.keepFile(path)
		if stop {
			f.fs.on.Store(true)
		}
		return true
	}
}

// keepFile keeps the session file at path as it is now, or its absence,
// for restart to put back.
func (f *replayFixture) keepFile(path string) {
	kept := &keptFile{}
	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		kept.gone = true
	case err != nil:
		panic(err)
	default:
		kept.data = data
	}
	f.killedMu.Lock()
	f.killed = kept
	f.killedMu.Unlock()
}

// diskFile is the session's open file.
func diskFile(sess *session) *sessionstore.File {
	sess.file.mu.RLock()
	defer sess.file.mu.RUnlock()
	return sess.file.f
}

// send sends body on the session with the given slot and seqid and
// returns the reply, retrying DELAY, which a restored session's first
// requests get while its lease is published.
func (f *replayFixture) send(body []byte, slot, seq uint32) []byte {
	f.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		reply, err := sendRawRPC(f.conn, f.xid, procCompound, withSequence(body, f.ts, slot, seq))
		f.xid++
		if err != nil {
			f.t.Fatal(err)
		}
		if sequenceStatus(reply) != nfs4.NFS4ERR_DELAY || time.Now().After(deadline) {
			return reply
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// sendRaw sends exactly wire and returns the reply, retrying DELAY.
func (f *replayFixture) sendRaw(wire []byte) []byte {
	f.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		reply, err := sendRawRPC(f.conn, f.xid, procCompound, wire)
		f.xid++
		if err != nil {
			f.t.Fatal(err)
		}
		if sequenceStatus(reply) != nfs4.NFS4ERR_DELAY || time.Now().After(deadline) {
			return reply
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ping sends a request on the session, so that it has recorded one and a
// restart restores it.
func (f *replayFixture) ping() {
	f.t.Helper()
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	if got := compoundStatus(f.t, f.send(body, slot, seq)); got != nfs4.NFS4_OK {
		f.t.Fatalf("PUTROOTFH %s", nfs4.Nfsstat4Name(got))
	}
}

func compoundStatus(t *testing.T, reply []byte) uint32 {
	t.Helper()
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	return res.Status()
}

// leaveIntent makes a completed request's slot hold the given intent as
// its newest record, which is the state a process leaves when it dies
// between the shard committing the effect and the reply being recorded:
// the intent is written to the slot's other area and the reply's record is
// torn. The record before the request, which a restart does not read, is
// gone rather than kept.
func (f *replayFixture) leaveIntent(slotID, seq uint32, intent []byte) {
	f.t.Helper()
	sess := f.srv.sessions.lookup(f.ts.id)
	slot := &sess.slots[slotID]
	slot.mu.Lock()
	digest, user := slot.digest, slot.user
	slot.mu.Unlock()
	file := diskFile(sess)
	if err := file.Write(slotID, sessionstore.Record{Seq: seq, Kind: sessionstore.KindIntent,
		Digest: digest, User: encodeUser(user), Payload: intent}); err != nil {
		f.t.Fatal(err)
	}
	tearSessionRecord(f.t, file, slotID, seq, sessionstore.KindSlot)
}

func removeBody(name string) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rw := w.AppendArgarray_Remove()
		rw.Resume(rw.StartTarget().SetData([]byte(name)).Finish())
		w.Resume(rw.Finish())
	})
}

func renameBody(from, to string) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Savefh()
		rw := w.AppendArgarray_Rename()
		rw.Resume(rw.StartOldname().SetData([]byte(from)).Finish())
		rw.Resume(rw.StartNewname().SetData([]byte(to)).Finish())
		w.Resume(rw.Finish())
	})
}

func mkdirBody(name string) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		cw := w.AppendArgarray_Create()
		cw.SetObjtype_Nf4dir()
		cw.Resume(cw.StartObjname().SetData([]byte(name)).Finish())
		finishEmptyCreateattrs(&cw)
		w.Resume(cw.Finish())
		w.AppendArgarray_Getfh()
	})
}

func symlinkBody(name, target string) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		cw := w.AppendArgarray_Create()
		cw.Resume(cw.SetObjtype_Nf4lnk().SetData([]byte(target)).Finish())
		cw.Resume(cw.StartObjname().SetData([]byte(name)).Finish())
		finishEmptyCreateattrs(&cw)
		w.Resume(cw.Finish())
		w.AppendArgarray_Getfh()
	})
}

func writeRoot(t *testing.T, root, name, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readRoot(t *testing.T, root, name string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data), true
}

// A request retransmitted after a restart gets its original reply from
// the session file and does not run again; the session goes on taking
// new requests and connections.
func TestRestartReplaysCachedReply(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "victim", "first")
	slot, seq := f.ts.acquire()
	request := withSequence(removeBody("victim"), f.ts, slot, seq)
	f.ts.release(slot)
	first := f.sendRaw(request)
	if got := compoundStatus(t, first); got != nfs4.NFS4_OK {
		t.Fatalf("REMOVE before restart %s", nfs4.Nfsstat4Name(got))
	}
	f.restart()
	// The name is in use again when the retransmission arrives.
	writeRoot(t, f.root, "victim", "second")

	reply := f.sendRaw(request)
	if got := compoundStatus(t, reply); got != nfs4.NFS4_OK {
		t.Fatalf("retransmission after restart %s", nfs4.Nfsstat4Name(got))
	}
	if data, ok := readRoot(t, f.root, "victim"); !ok || data != "second" {
		t.Fatalf("retransmitted REMOVE ran again: %q %v", data, ok)
	}
	// The replay is the original bytes, not a re-execution with new
	// change_info.
	if !bytes.Equal(withoutSequenceResult(first)[24:], withoutSequenceResult(reply)[24:]) {
		t.Fatal("replayed reply differs from the original")
	}
	if got := f.srv.metrics.replayServed.Load(); got != 1 {
		t.Fatalf("replays served %d", got)
	}
	// New requests and a new connection binding are served.
	next := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	if got := compoundStatus(t, f.send(next, slot, seq+1)); got != nfs4.NFS4_OK {
		t.Fatalf("new request on the restored session %s", nfs4.Nfsstat4Name(got))
	}
	res, err := parseCompoundReply(must(sendRawRPC(f.conn, f.xid, procCompound, admissionBind(f.ts.id, nfs4.CDFC4_FORE, false))))
	f.xid++
	if err != nil || res.Status() != nfs4.NFS4_OK {
		t.Fatalf("BIND_CONN_TO_SESSION on the restored session %s %v", nfs4.Nfsstat4Name(res.Status()), err)
	}
	if a := f.srv.sessions.accounting(); a.Sessions != 1 || a.Groups != 1 {
		t.Fatalf("restored state %+v", a)
	}
	if got := f.srv.metrics.replaySessionsRestored.Load(); got != 1 {
		t.Fatalf("sessions restored %d", got)
	}
}

// A retransmission to a slot that holds an intent but no reply runs again,
// and the intended RENAME or REMOVE is decided against the filesystem:
// applied, so it is reported done without running; not applied, so it runs
// now, on the intent already on the slot.
func TestRestartDecidesRequestWithIntent(t *testing.T) {
	type step struct {
		name    string
		prepare func(f *replayFixture)
		body    func(f *replayFixture) []byte
		intent  func(f *replayFixture) []byte
		check   func(f *replayFixture) // after the replay
	}
	rootID := func(f *replayFixture) vfs.InodeID { return f.srv.fs.RootID() }
	edgeOf := func(f *replayFixture, name string) vfs.Edge {
		edge, err := f.srv.fs.LookupEdge(rootID(f), name)
		if err != nil {
			f.t.Fatal(err)
		}
		return edge
	}
	steps := []step{
		{
			name:    "rename",
			prepare: func(f *replayFixture) { writeRoot(t, f.root, "a", "payload") },
			body:    func(f *replayFixture) []byte { return renameBody("a", "b") },
			intent: func(f *replayFixture) []byte {
				return renameIntent{srcDir: rootID(f), srcName: "a", src: edgeOf(f, "a"), dstDir: rootID(f), dstName: "b"}.encode(3)
			},
			check: func(f *replayFixture) {
				if _, ok := readRoot(t, f.root, "a"); ok {
					t.Fatal("a still exists")
				}
				if data, ok := readRoot(t, f.root, "b"); !ok || data != "payload" {
					t.Fatalf("b = %q %v", data, ok)
				}
			},
		},
		{
			name:    "remove",
			prepare: func(f *replayFixture) { writeRoot(t, f.root, "gone", "x") },
			body:    func(f *replayFixture) []byte { return removeBody("gone") },
			intent: func(f *replayFixture) []byte {
				return removeIntent{dir: rootID(f), name: "gone", edge: edgeOf(f, "gone")}.encode(2)
			},
			check: func(f *replayFixture) {
				if _, ok := readRoot(t, f.root, "gone"); ok {
					t.Fatal("gone still exists")
				}
			},
		},
	}
	for _, st := range steps {
		for _, applied := range []bool{true, false} {
			name := st.name + "/not applied"
			if applied {
				name = st.name + "/applied"
			}
			t.Run(name, func(t *testing.T) {
				f := newReplayFixture(t)
				if st.prepare != nil {
					st.prepare(f)
				}
				intent := st.intent(f)
				slot, seq := f.ts.acquire()
				request := withSequence(st.body(f), f.ts, slot, seq)
				f.ts.release(slot)
				// The effect runs, or fails before running, in the first
				// process; either way the process dies before the reply
				// record, leaving the intent.
				f.fs.on.Store(!applied)
				reply := f.sendRaw(request)
				f.fs.on.Store(false)
				if got := compoundStatus(t, reply); (got == nfs4.NFS4_OK) != applied {
					t.Fatalf("first attempt %s", nfs4.Nfsstat4Name(got))
				}
				f.leaveIntent(slot, seq, intent)
				f.restart()

				reply = f.sendRaw(request)
				if got := compoundStatus(t, reply); got != nfs4.NFS4_OK {
					t.Fatalf("retransmission after restart %s", nfs4.Nfsstat4Name(got))
				}
				st.check(f)
				decided := f.srv.metrics.replayDecidedApplied.Load()
				if applied && decided != 1 || !applied && decided != 0 {
					t.Fatalf("decided applied %d", decided)
				}
				// Run now, it uses the intent already on the slot.
				if got := f.srv.metrics.replayIntents.Load(); got != 0 {
					t.Fatalf("%d intents written by the replay", got)
				}
				// The decided request is recorded; a second retransmission
				// is an ordinary replay and decides nothing again.
				if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
					t.Fatalf("second retransmission %s", nfs4.Nfsstat4Name(got))
				}
				if got := f.srv.metrics.replayDecidedApplied.Load(); got != decided {
					t.Fatalf("second retransmission decided again: %d", got)
				}
			})
		}
	}
}

// A RENAME whose source is gone and whose destination holds something else
// cannot be decided: the replay gets DEADSESSION, never a second rename.
func TestRestartUndecidableRenameIsDeadSession(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "a", "payload")
	rootID := f.srv.fs.RootID()
	edge, err := f.srv.fs.LookupEdge(rootID, "a")
	if err != nil {
		t.Fatal(err)
	}
	slot, seq := f.ts.acquire()
	request := withSequence(renameBody("a", "b"), f.ts, slot, seq)
	f.ts.release(slot)
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(got))
	}
	f.leaveIntent(slot, seq, renameIntent{srcDir: rootID, srcName: "a", src: edge, dstDir: rootID, dstName: "b"}.encode(3))
	// Before the restart, b is replaced by another file. The old inode is
	// kept alive at c so the local backend cannot reuse its number.
	if err := os.Rename(filepath.Join(f.root, "b"), filepath.Join(f.root, "c")); err != nil {
		t.Fatal(err)
	}
	writeRoot(t, f.root, "b", "someone else's")
	f.restart()
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4ERR_DEADSESSION {
		t.Fatalf("undecidable replay %s, want DEADSESSION", nfs4.Nfsstat4Name(got))
	}
	if data, ok := readRoot(t, f.root, "b"); !ok || data != "someone else's" {
		t.Fatalf("b = %q %v", data, ok)
	}
	if got := f.srv.metrics.replayUndecidable.Load(); got != 1 {
		t.Fatalf("undecidable %d", got)
	}
}

// A restored slot accepts a later sequence number than it recorded, since
// the request that advanced it may not have been recorded; a stale one is
// still misordered, and a different request on the recorded sequence is a
// false retry.
func TestRestartRestoredSlotSequenceRules(t *testing.T) {
	f := newReplayFixture(t)
	ping := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	other := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh(); w.AppendArgarray_Getfh() })
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	for i := uint32(0); i < 3; i++ {
		if got := compoundStatus(t, f.send(ping, slot, seq+i)); got != nfs4.NFS4_OK {
			t.Fatal(nfs4.Nfsstat4Name(got))
		}
	}
	last := seq + 2
	f.restart()
	if got := sequenceStatus(f.send(other, slot, last)); got != nfs4.NFS4ERR_SEQ_FALSE_RETRY {
		t.Fatalf("different request on the recorded seqid %s", nfs4.Nfsstat4Name(got))
	}
	if got := sequenceStatus(f.send(ping, slot, last-1)); got != nfs4.NFS4ERR_SEQ_MISORDERED {
		t.Fatalf("stale seqid %s", nfs4.Nfsstat4Name(got))
	}
	if got := compoundStatus(t, f.send(ping, slot, last+3)); got != nfs4.NFS4_OK {
		t.Fatalf("later seqid on a restored slot %s", nfs4.Nfsstat4Name(got))
	}
	// Used once, the slot follows the ordinary rules.
	if got := sequenceStatus(f.send(ping, slot, last+5)); got != nfs4.NFS4ERR_SEQ_MISORDERED {
		t.Fatalf("skipped seqid after the slot was used %s", nfs4.Nfsstat4Name(got))
	}
	// An untouched restored slot takes any first seqid.
	free, _ := f.ts.acquire()
	f.ts.release(free)
	if got := compoundStatus(t, f.send(ping, free, 7)); got != nfs4.NFS4_OK {
		t.Fatalf("first request on an unused restored slot %s", nfs4.Nfsstat4Name(got))
	}
}

// A request sent without cachethis replays as RETRY_UNCACHED_REP after a
// restart, as it does within one process.
func TestRestartReplaysUncachedPrefix(t *testing.T) {
	f := newReplayFixture(t)
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	wire := admissionSequence(body, f.ts, slot, seq, false)
	if got := compoundStatus(t, f.sendRaw(wire)); got != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(got))
	}
	f.restart()
	if got := compoundStatus(t, f.sendRaw(wire)); got != nfs4.NFS4ERR_RETRY_UNCACHED_REP {
		t.Fatalf("uncached replay after restart %s", nfs4.Nfsstat4Name(got))
	}
}

// The Linux client's writes across a restart: a CLOSE answered before the
// kill is replayed from the cache, and a CLOSE never sent before it runs on
// the restored session with its old stateid through the recovered writer.
// Each file is published once and staging is left empty.
func TestRestartReplaysCloseAndClosesPendingWriter(t *testing.T) {
	f := newReplayFixture(t)
	closedState, closedFH := openCreateFile(t, f.conn, &f.xid, f.wire, "closed")
	writeFileAt(t, f.conn, &f.xid, closedFH, closedState, 0, []byte("closed before"))
	closeBody := func(fh []byte, stateid [16]byte) []byte {
		return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			pw.Resume(pw.StartObject().SetData(fh).Finish())
			w.Resume(pw.Finish())
			caw := w.AppendArgarray_Close()
			setStateid(caw.OpenStateid(), stateid)
		})
	}
	// The slot stays held so that nothing else advances it before the
	// retransmission.
	slot, seq := f.ts.acquire()
	defer f.ts.release(slot)
	closeRequest := withSequence(closeBody(closedFH, closedState), f.ts, slot, seq)
	if got := compoundStatus(t, f.sendRaw(closeRequest)); got != nfs4.NFS4_OK {
		t.Fatalf("CLOSE before restart %s", nfs4.Nfsstat4Name(got))
	}
	pendingState, pendingFH := openCreateFile(t, f.conn, &f.xid, f.wire, "pending")
	writeFileAt(t, f.conn, &f.xid, pendingFH, pendingState, 0, []byte("pending at the kill"))
	f.restart()

	if got := compoundStatus(t, f.sendRaw(closeRequest)); got != nfs4.NFS4_OK {
		t.Fatalf("CLOSE retransmitted after restart %s", nfs4.Nfsstat4Name(got))
	}
	slot2, seq2 := f.ts.acquire()
	f.ts.release(slot2)
	if got := compoundStatus(t, f.send(closeBody(pendingFH, pendingState), slot2, seq2)); got != nfs4.NFS4_OK {
		t.Fatalf("CLOSE of the pending writer after restart %s", nfs4.Nfsstat4Name(got))
	}
	for name, want := range map[string]string{"closed": "closed before", "pending": "pending at the kill"} {
		if data, ok := readRoot(t, f.root, name); !ok || data != want {
			t.Fatalf("%s = %q %v", name, data, ok)
		}
	}
	if n := len(f.store.Entries()); n != 0 {
		t.Fatalf("%d staging entries left", n)
	}
}

// A session file whose incarnation is not the confirmed one is removed at
// startup and no session is built from it.
func TestRestartDropsSessionOfUnknownIncarnation(t *testing.T) {
	stagingDir := t.TempDir()
	store, err := sessionstore.Open(filepath.Join(stagingDir, staging.SessionsDirName))
	if err != nil {
		t.Fatal(err)
	}
	id := sessionstore.ID{1, 2, 3}
	rec := sessionRecord{incarnation: 0x2000000000009999, wireClientID: 7,
		fore: channelAttrs{maxReq: 1 << 20, maxResp: 1 << 20, maxCached: 64 << 10, maxOps: 16, maxReqs: 8}}
	file, err := store.Create(id, sessionstore.Header{SlotCount: 8, AreaSize: slotAreaSize(64 << 10), Session: rec.encode()})
	if err != nil {
		t.Fatal(err)
	}
	// A recorded request, so that the incarnation is what refuses it.
	if err := file.Write(0, sessionstore.Record{Seq: 1, Kind: sessionstore.KindSlot, User: encodeUser(effectiveUser{})}); err != nil {
		t.Fatal(err)
	}
	file.Close()
	local, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer staging.CloseFiles(t, local)
	srv, err := NewServer(vfs.NewLocalTernVFS(t.TempDir()), local, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.clients.StopLeaseWorkers()
	if _, err := os.Stat(store.Path(id)); !os.IsNotExist(err) {
		t.Fatalf("file of an unknown incarnation kept: %v", err)
	}
	if a := srv.sessions.accounting(); a.Sessions != 0 || a.Groups != 0 {
		t.Fatalf("state built from it %+v", a)
	}
}

// A session whose recorded replies the reply cache cannot hold is not
// restored at all: a slot without its reply would run a completed request
// again, so the client gets BADSESSION and registers afresh. The new
// process holds no session of the incarnation, so it publishes no lease
// for it.
func TestRestartRefusesSessionWhenReplyCacheFull(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "victim", "first")
	slot, seq := f.ts.acquire()
	request := withSequence(removeBody("victim"), f.ts, slot, seq)
	f.ts.release(slot)
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(got))
	}
	incarnation := f.srv.sessions.lookup(f.ts.id).clientID
	path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	limit := replyCacheLimit
	replyCacheLimit = cachedReplyOverhead // room for nothing
	defer func() { replyCacheLimit = limit }()
	f.restart()
	f.srv.sessions.cacheMu.Lock()
	f.srv.sessions.cacheLimit = limit
	f.srv.sessions.cacheMu.Unlock()
	if a := f.srv.sessions.accounting(); a.Sessions != 0 {
		t.Fatalf("session restored with a full cache: %+v", a)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("refused session's file kept: %v", err)
	}
	// Startup waits for the leases it publishes, so one would be there now.
	if _, err := f.srv.fs.Lookup(vfs.InodeID(incarnation), f.srv.clients.LeaseName); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lease of an incarnation with no restored session: %v", err)
	}
	writeRoot(t, f.root, "victim", "second")
	reply, err := sendRawRPC(f.conn, f.xid, procCompound, request)
	f.xid++
	if err != nil {
		t.Fatal(err)
	}
	if got := sequenceStatus(reply); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("retransmission to the refused session %s, want BADSESSION", nfs4.Nfsstat4Name(got))
	}
	if data, ok := readRoot(t, f.root, "victim"); !ok || data != "second" {
		t.Fatalf("REMOVE ran again: %q %v", data, ok)
	}
}

// A restored session whose client never returns is retired when the
// incarnation's durable lease, last written by the earlier process, runs
// out: the sweep expires the client and the session's file goes with it.
func TestRestartAbandonedSessionExpiresWithLease(t *testing.T) {
	f := newReplayFixture(t)
	path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	f.ping()
	f.restart()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("restored session's file: %v", err)
	}
	if a := f.srv.sessions.accounting(); a.Sessions != 1 {
		t.Fatalf("after restart %+v", a)
	}
	f.srv.clients.StopLeaseWorkers()
	later := time.Now().Add(10 * clientstore.LeaseWriteTerm)
	f.srv.clients.SetClock(func() time.Time { return later })
	f.srv.runLeaseSweep()
	if a := f.srv.sessions.accounting(); a.Sessions != 0 {
		t.Fatalf("after the sweep %+v", a)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("abandoned session's file: %v", err)
	}
}

// The Linux client's writes across two restarts, with its session kept:
// data written and acknowledged UNSTABLE before the first kill, COMMIT in
// the second process. Nothing of the unstable data is checkpointed, so the
// COMMIT answers with the new process's verifier; the client sees it differ
// from the one its writes got and sends them again, then commits again.
// That second COMMIT must sync the recovered writer the resent WRITE used;
// otherwise nothing is checkpointed and the third process, closing the
// file, publishes it empty.
func TestRestartCommitSyncsRecoveredWriter(t *testing.T) {
	f := newReplayFixture(t)
	state, fh := openCreateFile(t, f.conn, &f.xid, f.wire, "large")
	data := bytes.Repeat([]byte("unstable "), 4096)
	firstVerifier := writeUnstableAt(t, f.conn, &f.xid, fh, state, 0, data)
	f.restart()
	commitBody := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		w.AppendArgarray_Commit()
	})
	commit := func() [8]byte {
		slot, seq := f.ts.acquire()
		f.ts.release(slot)
		reply := f.send(commitBody, slot, seq)
		res, err := parseCompoundReply(reply)
		if err != nil || res.Status() != nfs4.NFS4_OK {
			t.Fatalf("COMMIT %s %v", nfs4.Nfsstat4Name(res.Status()), err)
		}
		iter := res.Resarray()
		iter.Next() // SEQUENCE
		iter.Next() // PUTFH
		iter.Next()
		verf := iter.Resarray().Value().AsCOMMIT4resEntry().Value().AsCOMMIT4resok().Writeverf()
		var out [8]byte
		for i := range out {
			out[i] = verf.Data(i)
		}
		return out
	}
	if v := commit(); v == firstVerifier {
		t.Fatal("COMMIT after the restart kept the earlier process's write verifier")
	}
	// The client resends the unstable data and commits again.
	writeUnstableAt(t, f.conn, &f.xid, fh, state, 0, data)
	commit()
	f.restart()
	closeBody := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		caw := w.AppendArgarray_Close()
		setStateid(caw.OpenStateid(), state)
	})
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	if got := compoundStatus(t, f.send(closeBody, slot, seq)); got != nfs4.NFS4_OK {
		t.Fatalf("CLOSE after the second restart %s", nfs4.Nfsstat4Name(got))
	}
	if got, ok := readRoot(t, f.root, "large"); !ok || got != string(data) {
		t.Fatalf("published %d bytes, want %d", len(got), len(data))
	}
	if n := len(f.store.Entries()); n != 0 {
		t.Fatalf("%d staging entries left", n)
	}
}

// A WRITE with an old stateid after a restart adopts the recovered writer
// as a live open: later requests find it as any other open, and the data
// written before and after the restart is published together. The WRITE
// carries seqid 0, as the Linux 4.1 client sends it for I/O, so the open's
// generation is learnt from the CLOSE.
func TestRestartWriteAdoptsRecoveredWriter(t *testing.T) {
	f := newReplayFixture(t)
	state, fh := openCreateFile(t, f.conn, &f.xid, f.wire, "doc")
	writeFileAt(t, f.conn, &f.xid, fh, state, 0, []byte("before "))
	f.restart()
	current := state
	copy(current[:4], []byte{0, 0, 0, 0})
	writeBody := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		ww := w.AppendArgarray_Write()
		setStateid(ww.Stateid(), current)
		ww = ww.SetOffset(uint64(len("before ")))
		ww = ww.SetStable(fileSync4)
		ww = ww.SetData([]byte("after"))
		w.Resume(ww.Finish())
	})
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	if got := compoundStatus(t, f.send(writeBody, slot, seq)); got != nfs4.NFS4_OK {
		t.Fatalf("WRITE after restart %s", nfs4.Nfsstat4Name(got))
	}
	fileID, _ := vfs.FHToInodeID(fh)
	if !f.srv.opens.Live(extractStateID(stateidOf(state))) {
		t.Fatal("old stateid not adopted as a live open")
	}
	if writers := f.srv.views.WritersOf(openstate.ViewKey{ClientID: f.srv.sessions.lookup(f.ts.id).clientID, FileID: fileID}); len(writers) != 1 {
		t.Fatalf("writers of the handle after adoption: %v", writers)
	}
	if got := f.srv.metrics.replaySessionsRestored.Load(); got != 1 {
		t.Fatalf("sessions restored %d", got)
	}
	closeBody := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		caw := w.AppendArgarray_Close()
		setStateid(caw.OpenStateid(), state)
	})
	slot, seq = f.ts.acquire()
	f.ts.release(slot)
	if got := compoundStatus(t, f.send(closeBody, slot, seq)); got != nfs4.NFS4_OK {
		t.Fatalf("CLOSE %s", nfs4.Nfsstat4Name(got))
	}
	if got, ok := readRoot(t, f.root, "doc"); !ok || got != "before after" {
		t.Fatalf("published %q %v", got, ok)
	}
}

// stateidOf builds a wire stateid from a test's 16-byte form.
func stateidOf(stateid [16]byte) nfs4.Stateid4 {
	_, sid := nfs4.StartStateid4(nil)
	setStateid(sid, stateid)
	return sid
}

// writeUnstableAt is writeFileAt with UNSTABLE4, as the Linux client writes:
// the data reaches the staging file but nothing is checkpointed until a
// COMMIT or a stable write. It returns the reply's write verifier.
func writeUnstableAt(t *testing.T, conn net.Conn, xid *uint32, fh []byte, stateid [16]byte, offset uint64, data []byte) [8]byte {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		ww := w.AppendArgarray_Write()
		setStateid(ww.Stateid(), stateid)
		ww = ww.SetOffset(offset)
		ww = ww.SetStable(unstable4)
		ww = ww.SetData(data)
		w.Resume(ww.Finish())
	})
	*xid++
	iter := expectOK(t, res)
	nextOp(t, &iter) // PUTFH
	verf := nextOp(t, &iter).Value().AsWRITE4resEntry().Value().AsWRITE4resok().Writeverf()
	var out [8]byte
	for i := range out {
		out[i] = verf.Data(i)
	}
	return out
}

// The first SEQUENCE on a restored session is served, not delayed: the
// Linux client answers DELAY on the lone SEQUENCE its state manager sends
// after a reconnect by waiting 15 s with every other request held.
func TestRestartFirstSequenceIsNotDelayed(t *testing.T) {
	f := newReplayFixture(t)
	f.ping()
	f.restart()
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	lone := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {})
	reply, err := sendRawRPC(f.conn, f.xid, procCompound, withSequence(lone, f.ts, slot, seq))
	f.xid++
	if err != nil {
		t.Fatal(err)
	}
	if got := sequenceStatus(reply); got != nfs4.NFS4_OK {
		t.Fatalf("first SEQUENCE after restart %s", nfs4.Nfsstat4Name(got))
	}
}

// A CLOSE whose LinkFile ran in the earlier process, which died before
// removing the staging, finds the file already at its name when it runs
// again and reports success instead of failing the publication it did.
func TestRestartCloseAfterPublishedLinkIsIdempotent(t *testing.T) {
	f := newReplayFixture(t)
	state, fh := openCreateFile(t, f.conn, &f.xid, f.wire, "doc")
	writeFileAt(t, f.conn, &f.xid, fh, state, 0, []byte("published once"))
	stagingID := stagingIDForTest(t, f.store, state)
	// Keep the staging and its sidecar as they were before the CLOSE.
	keep := map[string][]byte{}
	for _, suffix := range []string{".staging", ".meta"} {
		name := fmt.Sprintf("%016x%s", uint64(stagingID), suffix)
		data, err := os.ReadFile(filepath.Join(f.stagingDir, name))
		if err != nil {
			t.Fatal(err)
		}
		keep[name] = data
	}
	closeFile(t, f.conn, &f.xid, fh, state)
	if got, ok := readRoot(t, f.root, "doc"); !ok || got != "published once" {
		t.Fatalf("published %q %v", got, ok)
	}
	// The process died between LinkFile and removing the staging.
	f.conn.Close()
	f.stop()
	f.stop = nil
	staging.CloseFiles(f.t, f.store)
	for name, data := range keep {
		if err := os.WriteFile(filepath.Join(f.stagingDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	f.start()
	// The first CLOSE also removed the open's durable marker, which the
	// dying process would not have reached; put it back.
	if err := f.srv.clients.MarkOpen(f.srv.sessions.lookup(f.ts.id).clientID, extractStateID(stateidOf(state))); err != nil {
		t.Fatal(err)
	}
	// The client's CLOSE, run again as a new request with the old stateid.
	closeBody := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
		caw := w.AppendArgarray_Close()
		setStateid(caw.OpenStateid(), state)
	})
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	if got := compoundStatus(t, f.send(closeBody, slot, seq)); got != nfs4.NFS4_OK {
		t.Fatalf("CLOSE after the restart %s", nfs4.Nfsstat4Name(got))
	}
	if got, ok := readRoot(t, f.root, "doc"); !ok || got != "published once" {
		t.Fatalf("after the second CLOSE %q %v", got, ok)
	}
	if n := len(f.store.Entries()); n != 0 {
		t.Fatalf("%d staging entries left", n)
	}
}

// A session that caches no reply still records the intent of a RENAME of
// two names of the longest length TernFS takes. Its process dies after the
// rename; meanwhile a new file appears at the old name. The retransmission
// decides the rename happened and does not move the new file over it.
func TestRestartLongRenameOnSmallCacheSession(t *testing.T) {
	f := newSmallCacheReplayFixture(t)
	from, to := strings.Repeat("a", maxTernNameLength), strings.Repeat("b", maxTernNameLength)
	writeRoot(t, f.root, from, "payload")
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	request := admissionSequence(renameBody(from, to), f.ts, slot, seq, false)
	f.fileAtEffect(1, false)
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatalf("RENAME %s", nfs4.Nfsstat4Name(got))
	}
	f.restart()
	writeRoot(t, f.root, from, "someone else's")
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatalf("retransmission after restart %s", nfs4.Nfsstat4Name(got))
	}
	if data, ok := readRoot(t, f.root, to); !ok || data != "payload" {
		t.Fatalf("destination = %q %v, want the renamed file", data, ok)
	}
	if data, ok := readRoot(t, f.root, from); !ok || data != "someone else's" {
		t.Fatalf("source = %q %v, want the new file", data, ok)
	}
	if got := f.srv.metrics.replayDecidedApplied.Load(); got != 1 {
		t.Fatalf("decided applied %d", got)
	}
}

// replyResult is the operation and status of a reply's last result.
func replyResult(t *testing.T, reply []byte) (op, status uint32) {
	t.Helper()
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	iter := res.Resarray()
	for iter.Next() {
		entry := iter.Resarray()
		op = entry.Disc()
	}
	return op, res.Status()
}

func removeTwoBody(first, second string) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		for _, name := range []string{first, second} {
			rw := w.AppendArgarray_Remove()
			rw.Resume(rw.StartTarget().SetData([]byte(name)).Finish())
			w.Resume(rw.Finish())
		}
	})
}

// A compound with two REMOVEs: the second's intent would replace the
// first's on the slot, so the session's file goes instead. Killed after
// both, the session is not restored and nothing runs again: a file put at
// the first name since is kept.
func TestRestartSecondProtectedOperationRemovesFile(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "x", "old x")
	writeRoot(t, f.root, "y", "old y")
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	request := withSequence(removeTwoBody("x", "y"), f.ts, slot, seq)
	f.fileAtEffect(2, false)
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatalf("REMOVE x, y %s", nfs4.Nfsstat4Name(got))
	}
	f.restart()
	writeRoot(t, f.root, "x", "new x")
	if got := sequenceStatus(f.sendRaw(request)); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("retransmission %s, want BADSESSION", nfs4.Nfsstat4Name(got))
	}
	if data, ok := readRoot(t, f.root, "x"); !ok || data != "new x" {
		t.Fatalf("x = %q %v, want the new file", data, ok)
	}
}

// The same compound killed after its first REMOVE, before the second
// reaches the filesystem: the slot holds the first's intent. Run again, the
// first is decided applied, leaving the file put at its name since alone,
// and the second runs, its own intent replaced by removing the file.
func TestRestartDecidesFirstOfTwoProtectedOperations(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "x", "old x")
	writeRoot(t, f.root, "y", "old y")
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	request := withSequence(removeTwoBody("x", "y"), f.ts, slot, seq)
	f.fileAtEffect(1, true)
	f.sendRaw(request) // The second REMOVE fails: the process is dead.
	if _, ok := readRoot(t, f.root, "y"); !ok {
		t.Fatal("y removed after the kill")
	}
	f.restart()
	writeRoot(t, f.root, "x", "new x")
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatalf("retransmission %s", nfs4.Nfsstat4Name(got))
	}
	if data, ok := readRoot(t, f.root, "x"); !ok || data != "new x" {
		t.Fatalf("x = %q %v, want the new file", data, ok)
	}
	if _, ok := readRoot(t, f.root, "y"); ok {
		t.Fatal("y not removed")
	}
	if got := f.srv.metrics.replayDecidedApplied.Load(); got != 1 {
		t.Fatalf("decided applied %d", got)
	}
	if _, err := os.Stat(f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))); !os.IsNotExist(err) {
		t.Fatalf("session file after the second REMOVE: %v", err)
	}
}

// A REMOVE in a directory a LOOKUP resolves: killed after it, and the name
// then resolves to another directory with the same entry. The intent names
// the old directory, so the replay is DEADSESSION and the new entry is kept.
func TestRestartChangedLookupFailsClosed(t *testing.T) {
	f := newReplayFixture(t)
	if err := os.Mkdir(filepath.Join(f.root, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRoot(t, f.root, "d/x", "old")
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		lw.Resume(lw.StartObjname().SetData([]byte("d")).Finish())
		w.Resume(lw.Finish())
		rw := w.AppendArgarray_Remove()
		rw.Resume(rw.StartTarget().SetData([]byte("x")).Finish())
		w.Resume(rw.Finish())
	})
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	request := withSequence(body, f.ts, slot, seq)
	f.fileAtEffect(1, false)
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatalf("REMOVE %s", nfs4.Nfsstat4Name(got))
	}
	f.restart()
	if err := os.Rename(filepath.Join(f.root, "d"), filepath.Join(f.root, "d.old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(f.root, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRoot(t, f.root, "d/x", "another directory's x")
	if op, got := replyResult(t, f.sendRaw(request)); op != nfs4.OP_REMOVE || got != nfs4.NFS4ERR_DEADSESSION {
		t.Fatalf("retransmission %s at %s, want DEADSESSION at REMOVE", nfs4.Nfsstat4Name(got), nfs4.NfsOpnum4Name(op))
	}
	if data, ok := readRoot(t, f.root, "d/x"); !ok || data != "another directory's x" {
		t.Fatalf("d/x = %q %v, want the other directory's", data, ok)
	}
}

// An intent that does not decode, an empty one included, or that names
// SEQUENCE's index, protects nothing it can name: every mutating operation
// of the replay is DEADSESSION.
func TestRestartMalformedIntentFailsClosed(t *testing.T) {
	for _, c := range []struct {
		name   string
		intent func(f *replayFixture) []byte
	}{
		{"undecodable", func(*replayFixture) []byte { return append(intentHeader(intentRemove, 2), 0xff) }},
		{"empty", func(*replayFixture) []byte { return nil }},
		{"index of SEQUENCE", func(f *replayFixture) []byte {
			return removeIntent{dir: f.srv.fs.RootID(), name: "victim"}.encode(0)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newReplayFixture(t)
			writeRoot(t, f.root, "victim", "old")
			slot, seq := f.ts.acquire()
			request := withSequence(removeBody("victim"), f.ts, slot, seq)
			f.ts.release(slot)
			if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
				t.Fatal(nfs4.Nfsstat4Name(got))
			}
			f.leaveIntent(slot, seq, c.intent(f))
			f.restart()
			writeRoot(t, f.root, "victim", "new")
			if op, got := replyResult(t, f.sendRaw(request)); op != nfs4.OP_REMOVE || got != nfs4.NFS4ERR_DEADSESSION {
				t.Fatalf("retransmission %s at %s, want DEADSESSION at REMOVE", nfs4.Nfsstat4Name(got), nfs4.NfsOpnum4Name(op))
			}
			if data, ok := readRoot(t, f.root, "victim"); !ok || data != "new" {
				t.Fatalf("victim = %q %v, want the new file", data, ok)
			}
			if got := f.srv.metrics.replayUndecidable.Load(); got != 1 {
				t.Fatalf("undecidable %d", got)
			}
		})
	}
}

// A RENAME across directories, the source by its handle and the
// destination by LOOKUP, killed after the rename. Then the destination
// directory is moved aside and the source directory put at its name, so
// the replay's two directories are one and the names the same. That is a
// RENAME of a name onto itself, which succeeds at once, but the intent
// names two directories: DEADSESSION, and nothing moves.
func TestRestartRenameOntoItselfAfterLookupChangeFailsClosed(t *testing.T) {
	f := newReplayFixture(t)
	for _, dir := range []string{"src", "dst"} {
		if err := os.Mkdir(filepath.Join(f.root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeRoot(t, f.root, "src/a", "payload")
	srcID, err := f.srv.fs.Lookup(f.srv.fs.RootID(), "src")
	if err != nil {
		t.Fatal(err)
	}
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(vfs.InodeIDToFH(srcID)).Finish())
		w.Resume(pw.Finish())
		w.AppendArgarray_Savefh()
		w.AppendArgarray_Putrootfh()
		lw := w.AppendArgarray_Lookup()
		lw.Resume(lw.StartObjname().SetData([]byte("dst")).Finish())
		w.Resume(lw.Finish())
		rw := w.AppendArgarray_Rename()
		rw.Resume(rw.StartOldname().SetData([]byte("a")).Finish())
		rw.Resume(rw.StartNewname().SetData([]byte("a")).Finish())
		w.Resume(rw.Finish())
	})
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	request := withSequence(body, f.ts, slot, seq)
	f.fileAtEffect(1, false)
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatalf("RENAME %s", nfs4.Nfsstat4Name(got))
	}
	f.restart()
	if err := os.Rename(filepath.Join(f.root, "dst"), filepath.Join(f.root, "dst.old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(f.root, "src"), filepath.Join(f.root, "dst")); err != nil {
		t.Fatal(err)
	}
	writeRoot(t, f.root, "dst/a", "new a")
	if op, got := replyResult(t, f.sendRaw(request)); op != nfs4.OP_RENAME || got != nfs4.NFS4ERR_DEADSESSION {
		t.Fatalf("retransmission %s at %s, want DEADSESSION at RENAME", nfs4.Nfsstat4Name(got), nfs4.NfsOpnum4Name(op))
	}
	if got := f.srv.metrics.replayUndecidable.Load(); got != 1 {
		t.Fatalf("undecidable %d", got)
	}
	for name, want := range map[string]string{"dst/a": "new a", "dst.old/a": "payload"} {
		if data, ok := readRoot(t, f.root, name); !ok || data != want {
			t.Fatalf("%s = %q %v, want %q", name, data, ok, want)
		}
	}
}

// A REMOVE run again and found not applied runs on its restored intent. If
// another client replaces the edge between the decision and the effect,
// the intent no longer describes what the REMOVE would remove: DEADSESSION,
// and the new file is kept.
func TestRestartReplayedRemoveOfReplacedEdgeFailsClosed(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "x", "old")
	edge, err := f.srv.fs.LookupEdge(f.srv.fs.RootID(), "x")
	if err != nil {
		t.Fatal(err)
	}
	slot, seq := f.ts.acquire()
	request := withSequence(removeBody("x"), f.ts, slot, seq)
	f.ts.release(slot)
	f.fs.on.Store(true) // The process dies before the effect.
	f.sendRaw(request)
	f.fs.on.Store(false)
	f.leaveIntent(slot, seq, removeIntent{dir: f.srv.fs.RootID(), name: "x", edge: edge}.encode(2))
	f.restart()
	// The decision's lookup finds the old edge; before the REMOVE's own,
	// another client replaces the file.
	lookups := 0
	f.fs.mu.Lock()
	f.fs.lookup = func(name string) {
		if name != "x" {
			return
		}
		if lookups++; lookups == 2 {
			// The old file is kept at another name, so the local backend
			// cannot give the new one its inode number.
			if err := os.Rename(filepath.Join(f.root, "x"), filepath.Join(f.root, "x.old")); err != nil {
				panic(err)
			}
			if err := os.WriteFile(filepath.Join(f.root, "x"), []byte("new"), 0o644); err != nil {
				panic(err)
			}
		}
	}
	f.fs.mu.Unlock()
	if op, got := replyResult(t, f.sendRaw(request)); op != nfs4.OP_REMOVE || got != nfs4.NFS4ERR_DEADSESSION {
		t.Fatalf("retransmission %s at %s, want DEADSESSION at REMOVE", nfs4.Nfsstat4Name(got), nfs4.NfsOpnum4Name(op))
	}
	if data, ok := readRoot(t, f.root, "x"); !ok || data != "new" {
		t.Fatalf("x = %q %v, want the new file", data, ok)
	}
}

// A REMOVE run again and found not applied, on a session whose file was
// poisoned meanwhile: its restored intent stands for it, but no protected
// effect is permitted once the file is poisoned. SERVERFAULT, x is kept.
func TestRestartReplayedRemoveOnPoisonedFileIsRefused(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "x", "old")
	edge, err := f.srv.fs.LookupEdge(f.srv.fs.RootID(), "x")
	if err != nil {
		t.Fatal(err)
	}
	slot, seq := f.ts.acquire()
	request := withSequence(removeBody("x"), f.ts, slot, seq)
	f.ts.release(slot)
	f.fs.on.Store(true)
	f.sendRaw(request)
	f.fs.on.Store(false)
	f.leaveIntent(slot, seq, removeIntent{dir: f.srv.fs.RootID(), name: "x", edge: edge}.encode(2))
	f.restart()
	// Another slot's failed write could not unlink the file.
	f.srv.unlinkSessionFile = func(*sessionstore.File) error { return errors.New("unlink refused") }
	if state, _ := f.srv.sessions.lookup(f.ts.id).file.invalidate(); state != filePoisoned {
		t.Fatalf("file state %d", state)
	}
	if op, got := replyResult(t, f.sendRaw(request)); op != nfs4.OP_REMOVE || got != nfs4.NFS4ERR_SERVERFAULT {
		t.Fatalf("retransmission %s at %s, want SERVERFAULT at REMOVE", nfs4.Nfsstat4Name(got), nfs4.NfsOpnum4Name(op))
	}
	if data, ok := readRoot(t, f.root, "x"); !ok || data != "old" {
		t.Fatalf("x = %q %v, want it kept", data, ok)
	}
}

// A compound with a mutating operation before the one an intent would
// protect, here an empty SETATTR before a REMOVE, leaves no intent to
// decide: its session's file goes before the REMOVE. Killed after it, the
// session is not restored, the retransmission gets BADSESSION and a file
// put at the REMOVE's name since is kept.
func TestRestartMutationBeforeIntentNotRestored(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "x", "old x")
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		emptySetattr(w)
		rw := w.AppendArgarray_Remove()
		rw.Resume(rw.StartTarget().SetData([]byte("x")).Finish())
		w.Resume(rw.Finish())
	})
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	request := withSequence(body, f.ts, slot, seq)
	f.fileAtEffect(1, false)
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatalf("SETATTR, REMOVE %s", nfs4.Nfsstat4Name(got))
	}
	f.restart()
	writeRoot(t, f.root, "x", "new x")
	if got := sequenceStatus(f.sendRaw(request)); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("retransmission %s, want BADSESSION", nfs4.Nfsstat4Name(got))
	}
	if data, ok := readRoot(t, f.root, "x"); !ok || data != "new x" {
		t.Fatalf("x = %q %v, want the new file", data, ok)
	}
}

// A CREATE run again with its intent restored is DEADSESSION whatever is at
// the name, and changes nothing there: an object may be someone else's, and
// a name absent may be one this CREATE made and someone removed since.
func TestRestartPendingCreateFailsClosed(t *testing.T) {
	marker := func(f *replayFixture, dir string) {
		writeRoot(t, f.root, dir+"/marker", "mine")
	}
	for _, c := range []struct {
		name    string
		body    []byte
		before  func(f *replayFixture) // in the first process, before its Mkdir
		after   func(f *replayFixture) // after the restart, before the replay
		first   uint32                 // the first attempt's status
		present bool                   // the name after the replay
		marked  bool                   // sub holds a marker the replay must keep
	}{
		{name: "made", body: mkdirBody("sub"), after: func(f *replayFixture) { marker(f, "sub") }, first: nfs4.NFS4_OK, present: true, marked: true},
		{name: "made then removed", body: mkdirBody("sub"), after: func(f *replayFixture) {
			if err := os.Remove(filepath.Join(f.root, "sub")); err != nil {
				t.Fatal(err)
			}
		}, first: nfs4.NFS4_OK},
		{name: "someone else's", body: mkdirBody("sub"), before: func(f *replayFixture) {
			if err := os.Mkdir(filepath.Join(f.root, "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
			marker(f, "sub")
		}, first: nfs4.NFS4ERR_EXIST, present: true, marked: true},
		{name: "symlink made", body: symlinkBody("link", "elsewhere"), first: nfs4.NFS4_OK, present: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newReplayFixture(t)
			slot, seq := f.ts.acquire()
			f.ts.release(slot)
			request := withSequence(c.body, f.ts, slot, seq)
			if c.before != nil {
				f.fs.mu.Lock()
				f.fs.before = func() { c.before(f) }
				f.fs.mu.Unlock()
			}
			f.fileAtEffect(1, false)
			if got := compoundStatus(t, f.sendRaw(request)); got != c.first {
				t.Fatalf("first attempt %s", nfs4.Nfsstat4Name(got))
			}
			f.restart()
			if c.after != nil {
				c.after(f)
			}
			before, _ := os.ReadDir(f.root)
			if op, got := replyResult(t, f.sendRaw(request)); op != nfs4.OP_CREATE || got != nfs4.NFS4ERR_DEADSESSION {
				t.Fatalf("retransmission %s at %s, want DEADSESSION at CREATE", nfs4.Nfsstat4Name(got), nfs4.NfsOpnum4Name(op))
			}
			after, _ := os.ReadDir(f.root)
			if len(after) != len(before) {
				t.Fatalf("the replay changed the directory: %d entries, then %d", len(before), len(after))
			}
			name := "sub"
			if c.name == "symlink made" {
				name = "link"
			}
			if _, err := os.Lstat(filepath.Join(f.root, name)); (err == nil) != c.present {
				t.Fatalf("%s after the replay: %v", name, err)
			}
			if c.marked {
				if data, ok := readRoot(t, f.root, "sub/marker"); !ok || data != "mine" {
					t.Fatalf("sub/marker = %q %v", data, ok)
				}
			}
		})
	}
}

// A session file of another format version is not restored: its records
// mean something else to this process. It is removed at startup, and a
// retransmission gets BADSESSION and does not run.
func TestRestartRefusesOtherFormatVersion(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "victim", "old")
	slot, seq := f.ts.acquire()
	request := withSequence(removeBody("victim"), f.ts, slot, seq)
	f.ts.release(slot)
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(got))
	}
	path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	f.leaveIntent(slot, seq, removeIntent{dir: f.srv.fs.RootID(), name: "victim"}.encode(2))
	f.conn.Close()
	f.stop()
	f.stop = nil
	staging.CloseFiles(t, f.store)
	setSessionFileVersion(t, path, 4)
	f.start()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file of another version kept: %v", err)
	}
	writeRoot(t, f.root, "victim", "new")
	if got := sequenceStatus(f.sendRaw(request)); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("retransmission %s, want BADSESSION", nfs4.Nfsstat4Name(got))
	}
	if data, ok := readRoot(t, f.root, "victim"); !ok || data != "new" {
		t.Fatalf("victim = %q %v, want the new file", data, ok)
	}
}

// setSessionFileVersion rewrites a session file's format version, with a
// header checksum to match.
func setSessionFileVersion(t *testing.T, path string, version uint16) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint16(data[4:6], version)
	off := 18 + int(binary.BigEndian.Uint32(data[14:18]))
	binary.BigEndian.PutUint32(data[off:], crc32.Checksum(data[:off], crc32.MakeTable(crc32.Castagnoli)))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// A REMOVE completes, and the process dies while writing the record of its
// reply, which is left torn. The slot still holds the REMOVE's intent, so
// the retransmission after the restart decides it applied and leaves alone
// the file put at the name since.
func TestRestartTornReplyKeepsIntent(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "victim", "old")
	slot, seq := f.ts.acquire()
	request := withSequence(removeBody("victim"), f.ts, slot, seq)
	f.ts.release(slot)
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(got))
	}
	tearSessionRecord(t, diskFile(f.srv.sessions.lookup(f.ts.id)), slot, seq, sessionstore.KindSlot)
	f.restart()
	writeRoot(t, f.root, "victim", "new")
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatalf("retransmission %s", nfs4.Nfsstat4Name(got))
	}
	if data, ok := readRoot(t, f.root, "victim"); !ok || data != "new" {
		t.Fatalf("victim = %q %v, want the new file", data, ok)
	}
	if got := f.srv.metrics.replayDecidedApplied.Load(); got != 1 {
		t.Fatalf("decided applied %d", got)
	}
}

// tearSessionRecord damages the slot area of a session file holding the
// record of seq and kind, as a write cut short leaves it. Slot areas follow
// a 4096-byte header (sessionstore's layout).
func tearSessionRecord(t *testing.T, f *sessionstore.File, slot, seq uint32, kind sessionstore.Kind) {
	t.Helper()
	fd, err := os.OpenFile(f.Path(), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	size := int64(f.Header().AreaSize)
	for area := int64(0); area < 2; area++ {
		off := 4096 + (int64(slot)*2+area)*size
		fixed := make([]byte, 12)
		if _, err := fd.ReadAt(fixed, off); err != nil {
			t.Fatal(err)
		}
		if string(fixed[0:4]) == "NFSR" && binary.BigEndian.Uint32(fixed[4:8]) == seq && sessionstore.Kind(fixed[8]) == kind {
			if _, err := fd.WriteAt([]byte{^fixed[11]}, off+11); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no record of seq %d kind %d on slot %d", seq, kind, slot)
}

// A session that never recorded a request has nothing a retransmission
// could need, and may be one whose CREATE_SESSION reply the client never
// got. Of two sessions of one client, only the one used before the restart
// is restored; the other's file is removed at startup.
func TestRestartSkipsSessionThatRecordedNothing(t *testing.T) {
	f := newReplayFixture(t)
	used := f.ts.id
	f.ping()
	clientID, csSeq, status, err := exchangeID(f.conn, f.xid, "test-client", [8]byte{})
	f.xid++
	if err != nil || status != nfs4.NFS4_OK {
		t.Fatalf("EXCHANGE_ID %s %v", nfs4.Nfsstat4Name(status), err)
	}
	unused, status, err := createSession(f.conn, f.xid, clientID, csSeq)
	f.xid++
	if err != nil || status != nfs4.NFS4_OK {
		t.Fatalf("CREATE_SESSION %s %v", nfs4.Nfsstat4Name(status), err)
	}
	unusedPath := f.srv.sessionFiles.Path(sessionstore.ID(unused.id))
	if _, err := os.Stat(unusedPath); err != nil {
		t.Fatalf("unused session's file: %v", err)
	}
	f.restart()
	if f.srv.sessions.lookup(used) == nil {
		t.Fatal("the used session was not restored")
	}
	if f.srv.sessions.lookup(unused.id) != nil {
		t.Fatal("the session that recorded nothing was restored")
	}
	if _, err := os.Stat(unusedPath); !os.IsNotExist(err) {
		t.Fatalf("file of the session that recorded nothing: %v", err)
	}
}

// cbRestartServer starts servers, one after another, over one filesystem
// and staging directory, for backchannel sessions that survive a restart.
func cbRestartServer(t *testing.T) func() (*Server, string, func()) {
	root, stagingDir := t.TempDir(), t.TempDir()
	var store *staging.LocalStore
	return func() (*Server, string, func()) {
		if store != nil {
			staging.CloseFiles(t, store)
		}
		var err error
		if store, err = staging.NewLocalStore(stagingDir, nil); err != nil {
			t.Fatal(err)
		}
		srv, err := NewServer(vfs.NewLocalTernVFS(root), store, nil)
		if err != nil {
			t.Fatal(err)
		}
		addr, stop := serveTestServer(t, srv)
		t.Cleanup(stop)
		return srv, addr, stop
	}
}

// A session's callback sequence is not kept on disk, so the session leaves
// the disk before its first CB_COMPOUND. A restored session that never sent
// one starts at sequence 1, as its client expects; once it has sent one,
// its file is gone, before the call reaches the client, and a restart does
// not restore it with its sequence back at 1.
func TestCallbackRemovesSessionFile(t *testing.T) {
	start := cbRestartServer(t)
	srv, addr, stop := start()
	f := newCBSessionOn(t, srv, addr)
	f.client.answerNull(t)
	f.sequence(t)
	path := srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	f.client.conn.Close()
	stop()

	f.srv, f.addr, stop = start()
	if f.srv.sessions.lookup(f.ts.id) == nil {
		t.Fatal("session not restored")
	}
	f.client = newCBTestClient(t, f.addr)
	if res := f.client.compound(f.next(), admissionBind(f.ts.id, nfs4.CDFC4_BACK_OR_BOTH, false)); res.Status() != nfs4.NFS4_OK {
		t.Fatalf("BIND %s", nfs4.Nfsstat4Name(res.Status()))
	}
	f.client.answerNull(t)
	f.pressure(t, true)
	f.sequence(t)
	call, ok := f.client.nextCall(testutil.ChannelTimeout)
	if !ok {
		t.Fatal("no recall")
	}
	if _, seq, _, _ := cbRecall(t, call); seq != 1 {
		t.Fatalf("restored session's first recall at sequence %d, want 1", seq)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file when its first callback arrived: %v", err)
	}
	f.client.answerSequence(t, call, nfs4.NFS4_OK)
	f.sequence(t)
	f.pressure(t, false)
	f.client.conn.Close()
	stop()

	f.srv, f.addr, _ = start()
	if f.srv.sessions.lookup(f.ts.id) != nil {
		t.Fatal("a session that sent a callback was restored")
	}
}

// When the session's file cannot be removed, no callback is sent: the
// file would restore the session with its callback sequence back at 1.
// The session is retired.
func TestCallbackRefusedWhenSessionFileStays(t *testing.T) {
	f := newCBSession(t)
	f.client.answerNull(t)
	f.sequence(t)
	path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	f.srv.unlinkSessionFile = func(*sessionstore.File) error { return errors.New("unlink refused") }
	f.pressure(t, true)
	defer f.pressure(t, false)
	f.sequence(t)
	if call, ok := f.client.nextCall(500 * time.Millisecond); ok {
		t.Fatalf("callback sent with the session file on disk: proc %d", cbProc(call))
	}
	deadline := time.Now().Add(testutil.ChannelTimeout)
	for f.srv.sessions.lookup(f.ts.id) != nil {
		if time.Now().After(deadline) {
			t.Fatal("session not retired")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file: %v", err)
	}
}

func backchannelCtlBody(program uint32) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		bw := w.AppendArgarray_BackchannelCtl()
		bw.SetBcaCbProgram(program)
		bw.AppendBcaSecParms_None()
		w.Resume(bw.Finish())
	})
}

// BACKCHANNEL_CTL changes what a restored session would call back with,
// which its file does not record: the file goes before the change, and the
// session is not restored. If the file cannot go, the change is refused.
func TestBackchannelCtlRemovesSessionFile(t *testing.T) {
	program := func(sess *session) uint32 {
		sess.cb.mu.Lock()
		defer sess.cb.mu.Unlock()
		return sess.cb.program
	}
	t.Run("removed", func(t *testing.T) {
		f := newReplayFixture(t)
		path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
		slot, seq := f.ts.acquire()
		f.ts.release(slot)
		if got := compoundStatus(t, f.send(backchannelCtlBody(0x40000002), slot, seq)); got != nfs4.NFS4_OK {
			t.Fatalf("BACKCHANNEL_CTL %s", nfs4.Nfsstat4Name(got))
		}
		if got := program(f.srv.sessions.lookup(f.ts.id)); got != 0x40000002 {
			t.Fatalf("program %#x", got)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("session file after BACKCHANNEL_CTL: %v", err)
		}
		f.restart()
		if f.srv.sessions.lookup(f.ts.id) != nil {
			t.Fatal("restored after BACKCHANNEL_CTL")
		}
	})
	t.Run("file stays", func(t *testing.T) {
		f := newReplayFixture(t)
		sess := f.srv.sessions.lookup(f.ts.id)
		before := program(sess)
		f.srv.unlinkSessionFile = func(*sessionstore.File) error { return errors.New("unlink refused") }
		slot, seq := f.ts.acquire()
		f.ts.release(slot)
		if op, got := replyResult(t, f.send(backchannelCtlBody(0x40000002), slot, seq)); op != nfs4.OP_BACKCHANNEL_CTL || got != nfs4.NFS4ERR_SERVERFAULT {
			t.Fatalf("BACKCHANNEL_CTL %s at %s, want SERVERFAULT", nfs4.Nfsstat4Name(got), nfs4.NfsOpnum4Name(op))
		}
		if got := program(sess); got != before {
			t.Fatalf("program changed to %#x", got)
		}
		if f.srv.sessions.lookup(f.ts.id) != nil {
			t.Fatal("session not retired")
		}
	})
}

// metaGateStore holds, once armed, the first GetMeta of one staging inode
// made while adopting a recovered writer, until released. A WRITE with a
// recovered writer's stateid makes that call after all its recovery checks
// and before the adoption takes any lock.
type metaGateStore struct {
	*staging.LocalStore
	mu               sync.Mutex
	armed            vfs.InodeID
	entered, release chan struct{}
}

func (g *metaGateStore) GetMeta(id vfs.InodeID) (staging.Meta, bool) {
	g.mu.Lock()
	hold := g.armed != 0 && g.armed == id && calledFrom("adoptRecoveredWriter")
	if hold {
		g.armed = 0
	}
	g.mu.Unlock()
	if hold {
		close(g.entered)
		<-g.release
	}
	return g.LocalStore.GetMeta(id)
}

// calledFrom reports whether a function of this package named name is on
// the calling goroutine's stack.
func calledFrom(name string) bool {
	pcs := make([]uintptr, 64)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.Function, ".(*Server)."+name) {
			return true
		}
		if !more {
			return false
		}
	}
}

// Two WRITEs after a restart use one recovered writer's stateid, on two
// slots: one at the writer's generation and one at the next, which the
// client never had. The second passes the recovery checks first; the first
// then adopts the writer and writes. The live open then rejects the
// second's generation, and the second must fail with BAD_STATEID, as it
// does when it comes after, rather than write over the first.
func TestRestartAdoptionRejectsOtherGenerationMeanwhile(t *testing.T) {
	gate := &metaGateStore{entered: make(chan struct{}), release: make(chan struct{})}
	f := newReplayServer(t)
	f.wrapStore = func(store *staging.LocalStore) staging.Store {
		gate.LocalStore = store
		return gate
	}
	f.conn.Close()
	f.stop()
	f.stop = nil
	staging.CloseFiles(t, f.store)
	f.start()
	f.wire = setupClient(t, f.conn, &f.xid)
	f.ts = testSessionForClient(f.conn, f.wire)
	defer testutil.CloseSignal(gate.release)

	state, fh := openCreateFile(t, f.conn, &f.xid, f.wire, "doc")
	writeFileAt(t, f.conn, &f.xid, fh, state, 0, []byte("seed"))
	dataID := stagingIDForTest(t, f.store, state)
	id := extractStateID(stateidOf(state))
	generation := binary.BigEndian.Uint32(state[:4])
	other := state
	binary.BigEndian.PutUint32(other[:4], generation+1)
	f.restart()
	if f.srv.opens.Live(id) || !f.srv.opens.CanRecover(id) {
		t.Fatal("writer not left to recover")
	}
	writeBody := func(sid [16]byte, data string) []byte {
		return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			pw.Resume(pw.StartObject().SetData(fh).Finish())
			w.Resume(pw.Finish())
			ww := w.AppendArgarray_Write()
			setStateid(ww.Stateid(), sid)
			ww = ww.SetOffset(0)
			ww = ww.SetStable(fileSync4)
			ww = ww.SetData([]byte(data))
			w.Resume(ww.Finish())
		})
	}
	otherSlot, otherSeq := f.ts.acquire()
	defer f.ts.release(otherSlot)
	ownSlot, ownSeq := f.ts.acquire()
	defer f.ts.release(ownSlot)

	gate.mu.Lock()
	gate.armed = dataID
	gate.mu.Unlock()
	conn := dial(t, f.addr)
	defer conn.Close()
	otherReply := make(chan []byte, 1)
	go func() {
		reply, err := sendRawRPC(conn, 1, procCompound, withSequence(writeBody(other, "EVIL"), f.ts, otherSlot, otherSeq))
		if err != nil {
			reply = nil
		}
		otherReply <- reply
	}()
	testutil.AwaitSignal(t, gate.entered, "the other generation's WRITE about to adopt the writer")
	if f.srv.opens.Live(id) {
		t.Fatal("writer adopted before the WRITE at its generation")
	}
	if got := compoundStatus(t, f.send(writeBody(state, "GOOD"), ownSlot, ownSeq)); got != nfs4.NFS4_OK {
		t.Fatalf("WRITE at the writer's generation %s", nfs4.Nfsstat4Name(got))
	}
	if !f.srv.opens.Live(id) {
		t.Fatal("WRITE at the writer's generation did not adopt it")
	}
	testutil.CloseSignal(gate.release)
	var reply []byte
	select {
	case reply = <-otherReply:
	case <-time.After(testutil.ChannelTimeout):
		t.Fatal("the other generation's WRITE did not finish")
	}
	if reply == nil {
		t.Fatal("no reply")
	}
	buf := make([]byte, 4)
	if n, _, err := f.store.Get(dataID).Read(0, buf, nil); err != nil || n != len(buf) || string(buf) != "GOOD" {
		t.Fatalf("staging holds %q (%d, %v), want the first WRITE's", buf, n, err)
	}
	if got := compoundStatus(t, reply); got != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("WRITE at the other generation %s, want BAD_STATEID", nfs4.Nfsstat4Name(got))
	}
	// The same WRITE after the adoption, with nothing between.
	slot, seq := f.ts.acquire()
	defer f.ts.release(slot)
	if got := compoundStatus(t, f.send(writeBody(other, "NEXT"), slot, seq)); got != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("later WRITE at the other generation %s, want BAD_STATEID", nfs4.Nfsstat4Name(got))
	}
}

// stagingKillStore calls kill, once armed, right after the staging store
// removes a staging file: a CLOSE that has published. Nothing in the
// request after that reaches the filesystem, so keeping the session file
// there stands for a kill before the reply is recorded.
type stagingKillStore struct {
	*staging.LocalStore
	mu   sync.Mutex
	kill func()
}

func (k *stagingKillStore) Remove(id vfs.InodeID) {
	k.LocalStore.Remove(id)
	k.mu.Lock()
	kill := k.kill
	k.kill = nil
	k.mu.Unlock()
	if kill != nil {
		kill()
	}
}

// newWrappedReplayFixture is newReplayFixture whose servers get the
// staging store through wrap.
func newWrappedReplayFixture(t *testing.T, wrap func(*staging.LocalStore) staging.Store) *replayFixture {
	t.Helper()
	f := newReplayServer(t)
	f.wrapStore = wrap
	f.conn.Close()
	f.stop()
	f.stop = nil
	staging.CloseFiles(t, f.store)
	f.start()
	f.wire = setupClient(t, f.conn, &f.xid)
	f.ts = testSessionForClient(f.conn, f.wire)
	return f
}

// openWriteCloseBody opens name for writing, writes data and closes it in
// one compound, with the current stateid, after a REMOVE of remove if set.
func openWriteCloseBody(clientID uint64, remove, name, data string) []byte {
	current := [16]byte{0, 0, 0, 1}
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		if remove != "" {
			rw := w.AppendArgarray_Remove()
			rw.Resume(rw.StartTarget().SetData([]byte(remove)).Finish())
			w.Resume(rw.Finish())
		}
		ow := w.AppendArgarray_Open()
		ow.SetShareAccess(nfs4.OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(nfs4.OPEN4_SHARE_DENY_NONE)
		ownerW := ow.StartOwner()
		ownerW = ownerW.SetClientid(clientID)
		ow.Resume(ownerW.SetOwner([]byte("mixed-owner")).Finish())
		ow.SetOpenhow_Default(nfs4.OPEN4_NOCREATE)
		cw := ow.SetClaim_Null()
		ow.Resume(cw.SetData([]byte(name)).Finish())
		w.Resume(ow.Finish())
		ww := w.AppendArgarray_Write()
		setStateid(ww.Stateid(), current)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(fileSync4)
		ww = ww.SetData([]byte(data))
		w.Resume(ww.Finish())
		caw := w.AppendArgarray_Close()
		setStateid(caw.OpenStateid(), current)
	})
}

// A compound with more than one mutating operation, here OPEN, WRITE and
// CLOSE of a file, alone or after a REMOVE, holds no intent for most of
// them. Killed after its CLOSE published and before its reply was
// recorded, it must not run again over a file published at the name since:
// the session file goes before its second mutating operation, so the
// retransmission gets BADSESSION and the new file is kept.
func TestRestartMultiMutationCompoundNotRestored(t *testing.T) {
	for _, remove := range []string{"", "x"} {
		name := "open write close"
		if remove != "" {
			name = "remove then " + name
		}
		t.Run(name, func(t *testing.T) {
			kill := &stagingKillStore{}
			f := newWrappedReplayFixture(t, func(store *staging.LocalStore) staging.Store {
				kill.LocalStore = store
				return kill
			})
			writeRoot(t, f.root, "doc", "old")
			if remove != "" {
				writeRoot(t, f.root, remove, "old x")
			}
			// An earlier request on the same slot, completed and recorded.
			slot, seq := f.ts.acquire()
			defer f.ts.release(slot)
			ping := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
			if got := compoundStatus(t, f.send(ping, slot, seq)); got != nfs4.NFS4_OK {
				t.Fatal(nfs4.Nfsstat4Name(got))
			}
			request := withSequence(openWriteCloseBody(f.wire, remove, "doc", "stale"), f.ts, slot, seq+1)
			path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
			kill.mu.Lock()
			kill.kill = func() { f.keepFile(path) }
			kill.mu.Unlock()
			if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
				t.Fatalf("compound %s", nfs4.Nfsstat4Name(got))
			}
			if data, _ := readRoot(t, f.root, "doc"); data != "stale" {
				t.Fatalf("doc = %q after the compound", data)
			}
			if n := len(f.store.Entries()); n != 0 {
				t.Fatalf("%d staging entries at the kill", n)
			}
			f.restart()
			// Another client publishes a new version at the name.
			root := f.srv.fs.RootID()
			id, cookie, err := f.srv.fs.ConstructFile(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.srv.fs.LinkFile(id, cookie, root, "doc", strings.NewReader("replacement")); err != nil {
				t.Fatal(err)
			}
			reply := f.sendRaw(request)
			if data, _ := readRoot(t, f.root, "doc"); data != "replacement" {
				t.Fatalf("doc = %q after the retransmission, want the replacement", data)
			}
			if got := sequenceStatus(reply); got != nfs4.NFS4ERR_BADSESSION {
				t.Fatalf("retransmission %s, want BADSESSION", nfs4.Nfsstat4Name(got))
			}
		})
	}
}

// When the session file cannot be removed before a compound's second
// mutating operation, that operation is refused before its effect.
func TestSecondMutationRefusedWhenSessionFileStays(t *testing.T) {
	f := newReplayFixture(t)
	writeRoot(t, f.root, "doc", "old")
	f.srv.unlinkSessionFile = func(*sessionstore.File) error { return errors.New("unlink refused") }
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	if op, got := replyResult(t, f.send(openWriteCloseBody(f.wire, "", "doc", "stale"), slot, seq)); op != nfs4.OP_WRITE || got != nfs4.NFS4ERR_SERVERFAULT {
		t.Fatalf("compound %s at %s, want SERVERFAULT at WRITE", nfs4.Nfsstat4Name(got), nfs4.NfsOpnum4Name(op))
	}
	if data, _ := readRoot(t, f.root, "doc"); data != "old" {
		t.Fatalf("doc = %q", data)
	}
	if f.srv.sessions.lookup(f.ts.id) != nil {
		t.Fatal("session not retired")
	}
}

// closeGateStore holds, once armed, the first GetMeta of one staging inode
// made by releaseWrite as it takes its locks: after a CLOSE found no live
// open and chose the recovered writer's sidecar.
type closeGateStore struct{ metaGateStore }

func (g *closeGateStore) GetMeta(id vfs.InodeID) (staging.Meta, bool) {
	g.mu.Lock()
	hold := g.armed != 0 && g.armed == id && calledFrom("releaseWrite") && calledFrom("lockStagingTarget")
	if hold {
		g.armed = 0
	}
	g.mu.Unlock()
	if hold {
		close(g.entered)
		<-g.release
	}
	return g.LocalStore.GetMeta(id)
}

// A CLOSE of a recovered writer and a WRITE with its stateid, on two
// slots. The CLOSE finds no live open; the WRITE then adopts the writer.
// The CLOSE, once it holds its locks, must close the live open, not the
// sidecar's writer, so no open is left without its staging.
func TestRestartRecoveredCloseAfterConcurrentAdoption(t *testing.T) {
	gate := &closeGateStore{metaGateStore{entered: make(chan struct{}), release: make(chan struct{})}}
	f := newWrappedReplayFixture(t, func(store *staging.LocalStore) staging.Store {
		gate.LocalStore = store
		return gate
	})
	defer testutil.CloseSignal(gate.release)
	state, fh := openCreateFile(t, f.conn, &f.xid, f.wire, "doc")
	writeFileAt(t, f.conn, &f.xid, fh, state, 0, []byte("seed"))
	dataID := stagingIDForTest(t, f.store, state)
	id := extractStateID(stateidOf(state))
	f.restart()
	handle := func(w *nfs4.COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(fh).Finish())
		w.Resume(pw.Finish())
	}
	writeBody := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		handle(w)
		ww := w.AppendArgarray_Write()
		setStateid(ww.Stateid(), state)
		ww = ww.SetOffset(0)
		ww = ww.SetStable(fileSync4)
		ww = ww.SetData([]byte("GOOD"))
		w.Resume(ww.Finish())
	})
	closeBody := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		handle(w)
		caw := w.AppendArgarray_Close()
		setStateid(caw.OpenStateid(), state)
	})
	closeSlot, closeSeq := f.ts.acquire()
	defer f.ts.release(closeSlot)
	writeSlot, writeSeq := f.ts.acquire()
	defer f.ts.release(writeSlot)
	gate.mu.Lock()
	gate.armed = dataID
	gate.mu.Unlock()
	conn := dial(t, f.addr)
	defer conn.Close()
	closeReply := make(chan []byte, 1)
	go func() {
		reply, err := sendRawRPC(conn, 1, procCompound, withSequence(closeBody, f.ts, closeSlot, closeSeq))
		if err != nil {
			reply = nil
		}
		closeReply <- reply
	}()
	testutil.AwaitSignal(t, gate.entered, "the CLOSE about to take its locks")
	if got := compoundStatus(t, f.send(writeBody, writeSlot, writeSeq)); got != nfs4.NFS4_OK {
		t.Fatalf("WRITE %s", nfs4.Nfsstat4Name(got))
	}
	if !f.srv.opens.Live(id) {
		t.Fatal("the WRITE did not adopt the writer")
	}
	testutil.CloseSignal(gate.release)
	var reply []byte
	select {
	case reply = <-closeReply:
	case <-time.After(testutil.ChannelTimeout):
		t.Fatal("the CLOSE did not finish")
	}
	if reply == nil || compoundStatus(t, reply) != nfs4.NFS4_OK {
		t.Fatal("CLOSE failed")
	}
	if data, _ := readRoot(t, f.root, "doc"); data != "GOOD" {
		t.Fatalf("published %q", data)
	}
	if n := len(f.store.Entries()); n != 0 {
		t.Fatalf("%d staging entries left", n)
	}
	if f.srv.opens.Live(id) {
		t.Fatal("the adopted open outlived the CLOSE")
	}
	if marked, err := f.srv.clients.HasOpen(f.srv.sessions.lookup(f.ts.id).clientID, id); err != nil || marked {
		t.Fatalf("open marker after the CLOSE: %v %v", marked, err)
	}
}
