// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sessionstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// persistedServer starts a server over its own staging directory and
// returns it with the directory, so tests can read the session files.
func persistedServer(t *testing.T, fs vfs.TernVFS) (*Server, string, string) {
	t.Helper()
	stagingDir := t.TempDir()
	store, err := staging.NewLocalStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { staging.CloseFiles(t, store) })
	srv, err := NewServer(fs, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup := serveTestServer(t, srv)
	t.Cleanup(cleanup)
	return srv, addr, stagingDir
}

// sessionFileRecords reloads a session's file from disk and returns its
// header and every slot's newest record.
func sessionFileRecords(t *testing.T, stagingDir string, id SessionID) (sessionstore.Header, map[uint32]sessionstore.Record) {
	t.Helper()
	store, err := sessionstore.Open(filepath.Join(stagingDir, staging.SessionsDirName))
	if err != nil {
		t.Fatal(err)
	}
	files, problems, err := store.Load()
	if err != nil || len(problems) != 0 {
		t.Fatalf("load: %v %v", err, problems)
	}
	for _, f := range files {
		defer f.Close()
		if f.ID() != sessionstore.ID(id) {
			continue
		}
		h := f.Header()
		recs := map[uint32]sessionstore.Record{}
		for slot := uint32(0); slot < h.SlotCount; slot++ {
			if rec, ok := f.Slot(slot); ok {
				recs[slot] = rec
			}
		}
		return h, recs
	}
	t.Fatalf("no file for session %s", sessionText(id))
	return sessionstore.Header{}, nil
}

// A session gets a file with its negotiated layout and identity, and every
// completed request leaves its slot's record: the reply when cached, the
// uncached prefix otherwise, matching what the server holds in memory.
func TestSessionFileRecordsEveryRequest(t *testing.T) {
	root := t.TempDir()
	srv, addr, stagingDir := persistedServer(t, vfs.NewLocalTernVFS(root))
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	ts := testSessionForClient(conn, clientID)
	sess := srv.sessions.lookup(ts.id)
	if sess == nil || diskFile(sess) == nil {
		t.Fatal("session has no file")
	}

	h, recs := sessionFileRecords(t, stagingDir, ts.id)
	if h.SlotCount != uint32(len(sess.slots)) || h.AreaSize != slotAreaSize(sess.maxCached) {
		t.Fatalf("header %+v, session has %d slots, max cached %d", h, len(sess.slots), sess.maxCached)
	}
	rec, err := decodeSessionRecord(h.Session)
	if err != nil || rec.incarnation != sess.clientID || rec.wireClientID != clientID ||
		rec.fore.maxReqs != uint32(len(sess.slots)) || rec.fore.maxCached != sess.maxCached {
		t.Fatalf("session record %+v %v; want incarnation %d, wire %#x", rec, err, sess.clientID, clientID)
	}
	if len(recs) != 0 {
		t.Fatalf("%d slot records before any request", len(recs))
	}
	// The records counted are those of requests, not the file's creation.
	if got := srv.metrics.replayRecords.Load(); got != 0 {
		t.Fatalf("%d records counted before any request", got)
	}

	// A cached request (the shim caches every request) and its reply.
	state, fh := openCreateFile(t, conn, &xid, clientID, "saved")
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("data"))
	closeFile(t, conn, &xid, fh, state)
	_, recs = sessionFileRecords(t, stagingDir, ts.id)
	if len(recs) == 0 {
		t.Fatal("no slot records after requests")
	}
	for slotID, rec := range recs {
		slot := &sess.slots[slotID]
		slot.mu.Lock()
		seq, reply, digest, user := slot.seq, slot.reply, slot.digest, slot.user
		slot.mu.Unlock()
		if rec.Kind != sessionstore.KindSlot || rec.Seq != seq || !rec.CacheThis {
			t.Fatalf("slot %d record %+v, memory seq %d", slotID, rec, seq)
		}
		if !bytes.Equal(rec.Payload, reply) || rec.Digest != digest {
			t.Fatalf("slot %d record differs from the slot in memory", slotID)
		}
		if u, ok := decodeUser(rec.User); !ok || u != user {
			t.Fatalf("slot %d user %+v, want %+v", slotID, u, user)
		}
	}
}

// An uncached request records the SEQUENCE-only prefix that a replay gets,
// so the replay after a restart answers RETRY_UNCACHED_REP as today.
func TestSessionFileRecordsUncachedPrefix(t *testing.T) {
	srv := admissionWritableServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	stagingDir := srv.sessionFiles.Dir()
	addr, stop := serveTestServer(t, srv)
	defer stop()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	client := setupClient(t, conn, &xid)
	ts := testSessionForClient(conn, client)
	body := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	_, res := admissionRequest(t, srv, admissionSequence(body, ts, 0, 1, false), 1, nil)
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("uncached compound %s", nfs4.Nfsstat4Name(res.Status()))
	}
	_, recs := sessionFileRecords(t, filepath.Dir(stagingDir), ts.id)
	rec, ok := recs[0]
	if !ok || rec.CacheThis || rec.Seq != 1 {
		t.Fatalf("slot 0 record %+v %v", rec, ok)
	}
	sess := srv.sessions.lookup(ts.id)
	if !bytes.Equal(rec.Payload, sess.slots[0].reply) {
		t.Fatal("recorded prefix differs from the slot's")
	}
	if res, ok := nfs4.ReadCOMPOUND4res(rec.Payload); !ok || res.Status() != nfs4.NFS4ERR_RETRY_UNCACHED_REP {
		t.Fatalf("recorded prefix status %s", nfs4.Nfsstat4Name(res.Status()))
	}
}

// intentWatchVFS captures the slot records of a session while a namespace
// effect runs, which is when an intent must already be on disk.
type intentWatchVFS struct {
	vfs.TernVFS
	t          *testing.T
	stagingDir string
	id         SessionID
	// dir is the directory the test mutates; the client store's own
	// renames under /.nfs are not watched.
	dir  vfs.InodeID
	seen []sessionstore.Record
}

func (f *intentWatchVFS) capture(dir vfs.InodeID) {
	if dir != f.dir || f.id == (SessionID{}) {
		return
	}
	_, recs := sessionFileRecords(f.t, f.stagingDir, f.id)
	for _, rec := range recs {
		if rec.Kind == sessionstore.KindIntent {
			f.seen = append(f.seen, rec)
		}
	}
}

func (f *intentWatchVFS) RenameEdge(srcDir vfs.InodeID, oldName string, edge vfs.Edge, dstDir vfs.InodeID, newName string) error {
	f.capture(srcDir)
	return f.TernVFS.RenameEdge(srcDir, oldName, edge, dstDir, newName)
}

func (f *intentWatchVFS) RemoveEdge(dir vfs.InodeID, name string, edge vfs.Edge) error {
	f.capture(dir)
	return f.TernVFS.RemoveEdge(dir, name, edge)
}

func (f *intentWatchVFS) Mkdir(dir vfs.InodeID, name string) (vfs.InodeID, error) {
	f.capture(dir)
	return f.TernVFS.Mkdir(dir, name)
}

func (f *intentWatchVFS) Symlink(dir vfs.InodeID, name, target string) (vfs.InodeID, error) {
	f.capture(dir)
	return f.TernVFS.Symlink(dir, name, target)
}

// RENAME, REMOVE and CREATE write an intent naming what they are about to
// do before the effect runs; the completed request's record replaces it.
func TestIntentPrecedesNamespaceEffect(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "doc"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	watch := &intentWatchVFS{TernVFS: vfs.NewLocalTernVFS(root), t: t}
	srv, addr, stagingDir := persistedServer(t, watch)
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	ts := testSessionForClient(conn, clientID)
	rootID := srv.fs.RootID()
	watch.stagingDir, watch.id, watch.dir = stagingDir, ts.id, rootID
	docEdge, err := srv.fs.LookupEdge(rootID, "doc")
	if err != nil {
		t.Fatal(err)
	}

	renameFile(t, conn, &xid, "doc", "moved")
	mkdirIn(t, conn, &xid, "sub")
	symlinkIn(t, conn, &xid, "link", "moved")
	removeFile(t, conn, &xid, "moved")

	if len(watch.seen) != 4 {
		t.Fatalf("%d intents seen during effects, want 4", len(watch.seen))
	}
	// Each names its operation's index: SEQUENCE is 0, then PUTROOTFH and,
	// for RENAME, SAVEFH.
	want := []struct {
		op int
		in any
	}{
		{3, renameIntent{srcDir: rootID, srcName: "doc", src: docEdge, dstDir: rootID, dstName: "moved"}},
		{2, createIntent{dir: rootID, name: "sub", objType: nfs4.NF4DIR}},
		{2, createIntent{dir: rootID, name: "link", objType: nfs4.NF4LNK, target: "moved"}},
		{2, removeIntent{dir: rootID, name: "moved", edge: docEdge}},
	}
	for i, rec := range watch.seen {
		op, got, err := decodeIntent(rec.Payload)
		if err != nil || got != want[i].in || op != want[i].op {
			t.Fatalf("intent %d = %+v at %d %v, want %+v", i, got, op, err, want[i])
		}
	}
	// Once the requests completed, their slots hold completed records.
	_, recs := sessionFileRecords(t, stagingDir, ts.id)
	for slotID, rec := range recs {
		if rec.Kind != sessionstore.KindSlot {
			t.Fatalf("slot %d still holds %+v", slotID, rec)
		}
	}
	if got := srv.metrics.replayIntents.Load(); got != 4 {
		t.Fatalf("intents counted %d", got)
	}
}

// mkdirIn and symlinkIn send the CREATE compounds mkdirBody and symlinkBody
// build (session_restore_test.go) through the session shim.
func mkdirIn(t *testing.T, conn net.Conn, xid *uint32, name string) {
	t.Helper()
	sendBodyOK(t, conn, xid, mkdirBody(name), "CREATE dir "+name)
}

func symlinkIn(t *testing.T, conn net.Conn, xid *uint32, name, target string) {
	t.Helper()
	sendBodyOK(t, conn, xid, symlinkBody(name, target), "CREATE symlink "+name)
}

// sendBodyOK sends a prebuilt compound body through the session shim and
// requires NFS4_OK.
func sendBodyOK(t *testing.T, conn net.Conn, xid *uint32, body []byte, what string) {
	t.Helper()
	reply, err := sendRPC(conn, *xid, procCompound, body)
	*xid++
	if err != nil {
		t.Fatal(err)
	}
	nfsBody, err := parseRPCReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	res, ok := nfs4.ReadCOMPOUND4res(nfsBody)
	if !ok {
		t.Fatalf("%s: unparsable reply", what)
	}
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("%s: %s", what, nfs4.Nfsstat4Name(res.Status()))
	}
}

func finishEmptyCreateattrs(cw *nfs4.CREATE4argsWriter) {
	faw := cw.StartCreateattrs()
	bmW := faw.StartAttrmask()
	faw.Resume(bmW.Finish())
	alW := faw.StartAttrVals()
	faw.Resume(alW.SetData(nil).Finish())
	cw.Resume(faw.Finish())
}

// A destroyed session's file goes with it, as does the file of a session
// retired with its incarnation.
func TestSessionFileRemovedWithSession(t *testing.T) {
	srv, addr, _ := persistedServer(t, vfs.NewLocalTernVFS(t.TempDir()))
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	ts := testSessionForClient(conn, clientID)
	path := srv.sessionFiles.Path(sessionstore.ID(ts.id))
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
		op := w.AppendArgarray_DestroySession()
		writeSessionID(op.DsaSessionid(), ts.id)
	})
	xid++
	if res.Status() != nfs4.NFS4_OK {
		t.Fatalf("DESTROY_SESSION %s", nfs4.Nfsstat4Name(res.Status()))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("destroyed session's file: %v", err)
	}

	// A second session, retired with the incarnation.
	conn2 := dial(t, addr)
	defer conn2.Close()
	clientID2 := setupClient(t, conn2, &xid)
	ts2 := testSessionForClient(conn2, clientID2)
	path2 := srv.sessionFiles.Path(sessionstore.ID(ts2.id))
	if _, err := os.Stat(path2); err != nil {
		t.Fatal(err)
	}
	srv.retireIncarnationSessions(srv.sessions.lookup(ts2.id).clientID)
	if _, err := os.Stat(path2); !os.IsNotExist(err) {
		t.Fatalf("retired session's file: %v", err)
	}
	if n := srv.sessions.persistedSessions(); n != 0 {
		t.Fatalf("%d persisted sessions counted after retirement", n)
	}
}

// A completed request whose record does not fit its slot area, here an
// uncached reply with a 2 KiB tag on a session that caches no reply, takes
// the session's file away: the file would otherwise restore the slot as it
// was before the request. The session goes on in memory and is not
// restored after a restart.
func TestSessionFileGoneWhenReplyNotRecorded(t *testing.T) {
	f := newSmallCacheReplayFixture(t)
	path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	body := buildCompoundBody(bytes.Repeat([]byte("t"), 2048), func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	request := admissionSequence(body, f.ts, slot, seq, false)
	if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
		t.Fatalf("long-tag request %s", nfs4.Nfsstat4Name(got))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file kept after its record failed: %v", err)
	}
	if got := f.srv.metrics.replayWriteFailures.Load(); got != 1 {
		t.Fatalf("write failures %d", got)
	}
	if n := f.srv.MetricsSnapshot().ReplaySessionFiles; n != 0 {
		t.Fatalf("%d session files reported", n)
	}
	ping := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	next, nextSeq := f.ts.acquire()
	f.ts.release(next)
	if got := compoundStatus(t, f.sendRaw(admissionSequence(ping, f.ts, next, nextSeq, false))); got != nfs4.NFS4_OK {
		t.Fatalf("the session after its file went %s", nfs4.Nfsstat4Name(got))
	}
	f.restart()
	if got := sequenceStatus(f.sendRaw(request)); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("retransmission after restart %s, want BADSESSION", nfs4.Nfsstat4Name(got))
	}
}

// gatedUnlink stands in for the server's unlink of a session file: it
// signals entered, waits for release, then fails with err or unlinks.
type gatedUnlink struct {
	entered, release chan struct{}
	err              error
}

func newGatedUnlink(err error) *gatedUnlink {
	return &gatedUnlink{entered: make(chan struct{}, 1), release: make(chan struct{}), err: err}
}

func (g *gatedUnlink) unlink(f *sessionstore.File) error {
	select {
	case g.entered <- struct{}{}:
	default:
	}
	<-g.release
	if g.err != nil {
		return g.err
	}
	return f.Unlink()
}

// invalidationRace runs two REMOVEs on two slots of one session, a and b,
// while its file cannot be written. The first one's intent write fails, so
// the file is unlinked through gate; the second is sent while the unlink
// waits, and must not run its effect before the outcome is known. done, on
// a third slot, completed before. It returns both statuses and the
// requests.
func invalidationRace(t *testing.T, f *replayFixture, gate *gatedUnlink) (statusA, statusB uint32, requestA, requestB, requestDone []byte) {
	t.Helper()
	for _, name := range []string{"a", "b", "done"} {
		writeRoot(t, f.root, name, "old "+name)
	}
	// Each request keeps its slot, so the retransmissions after a restart
	// find them as they were.
	slotDone, seqDone := f.ts.acquire()
	slotA, seqA := f.ts.acquire()
	slotB, seqB := f.ts.acquire()
	t.Cleanup(func() {
		for _, slot := range []uint32{slotDone, slotA, slotB} {
			f.ts.release(slot)
		}
	})
	requestDone = withSequence(removeBody("done"), f.ts, slotDone, seqDone)
	if got := compoundStatus(t, f.sendRaw(requestDone)); got != nfs4.NFS4_OK {
		t.Fatalf("REMOVE done %s", nfs4.Nfsstat4Name(got))
	}
	sess := f.srv.sessions.lookup(f.ts.id)
	// Released on every path, so a failure here does not leave the
	// server's requests waiting on the gate at teardown.
	defer testutil.CloseSignal(gate.release)
	f.srv.unlinkSessionFile = gate.unlink
	// Every write to the file fails from here on.
	diskFile(sess).Close()

	send := func(wire []byte) <-chan []byte {
		conn := dial(t, f.addr)
		t.Cleanup(func() { conn.Close() })
		out := make(chan []byte, 1)
		go func() {
			reply, err := sendRawRPC(conn, 1, procCompound, wire)
			if err != nil {
				reply = nil
			}
			out <- reply
		}()
		return out
	}
	requestA = withSequence(removeBody("a"), f.ts, slotA, seqA)
	requestB = withSequence(removeBody("b"), f.ts, slotB, seqB)
	replyA := send(requestA)
	select {
	case <-gate.entered:
	case <-time.After(testutil.ChannelTimeout):
		t.Fatal("the failed intent write did not unlink the session file")
	}
	replyB := send(requestB)
	// B is admitted on its slot, so what it waits for now is the file.
	admitted := func() bool {
		slot := &sess.slots[slotB]
		slot.mu.Lock()
		defer slot.mu.Unlock()
		return slot.inUse && slot.seq == seqB
	}
	for deadline := time.Now().Add(testutil.ChannelTimeout); !admitted(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the other slot's REMOVE was not admitted")
		}
	}
	select {
	case <-replyB:
		t.Fatal("the other slot's REMOVE answered while the file's unlink was undecided")
	case <-time.After(100 * time.Millisecond):
	}
	if _, ok := readRoot(t, f.root, "b"); !ok {
		t.Fatal("the other slot's REMOVE ran while the file's unlink was undecided")
	}
	// Diagnostics and metrics do not wait for the file.
	reported := make(chan struct{})
	go func() {
		f.srv.MetricsSnapshot()
		f.srv.sessions.snapshotSessions(inspect.SessionFilter{})
		close(reported)
	}()
	select {
	case <-reported:
	case <-time.After(testutil.ChannelTimeout):
		t.Fatal("metrics waited for the session file's unlink")
	}
	testutil.CloseSignal(gate.release)
	status := func(reply <-chan []byte) uint32 {
		select {
		case r := <-reply:
			if r == nil {
				t.Fatal("no reply")
			}
			return compoundStatus(t, r)
		case <-time.After(testutil.ChannelTimeout):
			t.Fatal("no reply")
		}
		return 0
	}
	return status(replyA), status(replyB), requestA, requestB, requestDone
}

// When the unlink succeeds, the session goes on in memory: both REMOVEs
// run, and after a restart the session is gone.
func TestSessionFileUnlinkedBeforeOtherSlotsEffect(t *testing.T) {
	f := newReplayFixture(t)
	path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	statusA, statusB, requestA, _, _ := invalidationRace(t, f, newGatedUnlink(nil))
	if statusA != nfs4.NFS4_OK || statusB != nfs4.NFS4_OK {
		t.Fatalf("REMOVEs %s %s, want OK", nfs4.Nfsstat4Name(statusA), nfs4.Nfsstat4Name(statusB))
	}
	for _, name := range []string{"a", "b"} {
		if _, ok := readRoot(t, f.root, name); ok {
			t.Fatalf("%s not removed", name)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file: %v", err)
	}
	ping := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	if got := compoundStatus(t, f.send(ping, slot, seq)); got != nfs4.NFS4_OK {
		t.Fatalf("the session after its file went %s", nfs4.Nfsstat4Name(got))
	}
	f.restart()
	writeRoot(t, f.root, "a", "new a")
	if got := sequenceStatus(f.sendRaw(requestA)); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("retransmission after restart %s, want BADSESSION", nfs4.Nfsstat4Name(got))
	}
	if _, ok := readRoot(t, f.root, "a"); !ok {
		t.Fatal("the REMOVE ran again")
	}
}

// When the unlink fails, the file is still on disk and cannot record what
// the session does next: both REMOVEs get SERVERFAULT before their effect
// and the session is retired. The file holds no intent of either, so after
// a restart their retransmissions run, and the request completed before
// the failure is answered from the file.
func TestSessionFileUnlinkFailureRefusesEffects(t *testing.T) {
	f := newReplayFixture(t)
	path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	statusA, statusB, requestA, requestB, requestDone := invalidationRace(t, f, newGatedUnlink(errors.New("unlink refused")))
	if statusA != nfs4.NFS4ERR_SERVERFAULT || statusB != nfs4.NFS4ERR_SERVERFAULT {
		t.Fatalf("REMOVEs %s %s, want SERVERFAULT", nfs4.Nfsstat4Name(statusA), nfs4.Nfsstat4Name(statusB))
	}
	for _, name := range []string{"a", "b"} {
		if _, ok := readRoot(t, f.root, name); !ok {
			t.Fatalf("%s removed although its intent was not recorded", name)
		}
	}
	ping := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	if got := sequenceStatus(f.send(ping, slot, seq)); got != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("the session after the failed unlink %s, want BADSESSION", nfs4.Nfsstat4Name(got))
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file: %v", err)
	}
	f.restart()
	for _, request := range [][]byte{requestA, requestB} {
		if got := compoundStatus(t, f.sendRaw(request)); got != nfs4.NFS4_OK {
			t.Fatalf("retransmission after restart %s", nfs4.Nfsstat4Name(got))
		}
	}
	for _, name := range []string{"a", "b"} {
		if _, ok := readRoot(t, f.root, name); ok {
			t.Fatalf("%s not removed by its retransmission", name)
		}
	}
	writeRoot(t, f.root, "done", "new done")
	if got := compoundStatus(t, f.sendRaw(requestDone)); got != nfs4.NFS4_OK {
		t.Fatalf("retransmission of the earlier request %s", nfs4.Nfsstat4Name(got))
	}
	if data, ok := readRoot(t, f.root, "done"); !ok || data != "new done" {
		t.Fatalf("the earlier REMOVE ran again: %q %v", data, ok)
	}
	if got := f.srv.metrics.replayServed.Load(); got != 1 {
		t.Fatalf("replays served %d", got)
	}
}

// A request admitted before its session was retired holds a use of the
// session, and the file stays on disk until that use ends. An intent the
// request writes meanwhile is recorded like any other: a restart could
// restore the session from the file, and must find the effect's intent.
func TestRetiredSessionStillRecordsIntent(t *testing.T) {
	f := newReplayFixture(t)
	ss := f.srv.sessions
	sess := ss.lookup(f.ts.id)
	if !ss.useSession(sess) {
		t.Fatal("session not in use")
	}
	ss.retireSession(sess, false)
	path := f.srv.sessionFiles.Path(sessionstore.ID(f.ts.id))
	payload := removeIntent{dir: f.srv.fs.RootID(), name: "x", edge: vfs.Edge{ID: 7, CreationTime: 8}}.encode(2)
	st := &compoundState{session: sess, slot: &sess.slots[1], slotID: 1, seq: 1, opIndex: 2}
	if f.srv.persistIntent(st, payload) != nfs4.NFS4_OK {
		t.Fatal("intent refused on a retired session")
	}
	_, recs := sessionFileRecords(t, f.stagingDir, f.ts.id)
	if rec, ok := recs[1]; !ok || rec.Kind != sessionstore.KindIntent || !bytes.Equal(rec.Payload, payload) {
		t.Fatalf("slot 1 on disk: %+v %v, want the intent", rec, ok)
	}
	ss.releaseSessionUse(sess)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file after the last use: %v", err)
	}
}

// noOrphanSessionFiles checks that every file in the session store belongs
// to a session of the server that has it on disk.
func noOrphanSessionFiles(t *testing.T, srv *Server) {
	t.Helper()
	entries, err := os.ReadDir(srv.sessionFiles.Dir())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		found := false
		for _, sess := range srv.sessions.accountingSessions() {
			if sessionstore.ID(sess.id).String() == e.Name() && sess.file.onDisk.Load() {
				found = true
			}
		}
		if !found {
			t.Fatalf("session file %s belongs to no session", e.Name())
		}
	}
}

// sessionFilesMatchSessions checks that the session store holds a file for
// every session and for nothing else.
func sessionFilesMatchSessions(t *testing.T, srv *Server) {
	t.Helper()
	noOrphanSessionFiles(t, srv)
	for id, sess := range srv.sessions.accountingSessions() {
		if !sess.file.onDisk.Load() {
			t.Fatalf("session %s has no file", sessionText(id))
		}
	}
}

// createSecondSession registers another client on its own connection and
// starts its CREATE_SESSION, returning the client and the reply to come.
func createSecondSession(t *testing.T, f *replayFixture, identity string) (clientID uint64, csSeq uint32, reply <-chan []byte) {
	t.Helper()
	conn := dial(t, f.addr)
	t.Cleanup(func() { conn.Close() })
	clientID, csSeq, status, err := exchangeID(conn, 1, identity, [8]byte{})
	if err != nil || status != nfs4.NFS4_OK {
		t.Fatalf("EXCHANGE_ID %s %v", nfs4.Nfsstat4Name(status), err)
	}
	out := make(chan []byte, 1)
	go func() {
		r, err := sendRawRPC(conn, 2, procCompound, createSessionBody(clientID, csSeq))
		if err != nil {
			r = nil
		}
		out <- r
	}()
	return clientID, csSeq, out
}

// createSessionStatus waits for a CREATE_SESSION reply and returns its
// status and session.
func createSessionStatus(t *testing.T, reply <-chan []byte) (uint32, SessionID) {
	t.Helper()
	select {
	case r := <-reply:
		if r == nil {
			t.Fatal("no CREATE_SESSION reply")
		}
		res, err := parseCompoundReply(r)
		if err != nil {
			t.Fatal(err)
		}
		if res.Status() != nfs4.NFS4_OK {
			return res.Status(), SessionID{}
		}
		iter := res.Resarray()
		iter.Next()
		return nfs4.NFS4_OK, readSessionID(iter.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok().CsrSessionid())
	case <-time.After(testutil.ChannelTimeout):
		t.Fatal("no CREATE_SESSION reply")
	}
	return 0, SessionID{}
}

// within fails the test if fn does not return in time.
func within(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(testutil.ChannelTimeout):
		t.Fatalf("%s waited for a session file being created", what)
	}
}

// gatedCreate holds the server's session file creation until release.
func gatedCreate(f *replayFixture) (entered chan sessionstore.ID, release chan struct{}, calls *atomic.Int32) {
	entered, release, calls = make(chan sessionstore.ID, 4), make(chan struct{}), &atomic.Int32{}
	create := f.srv.newSessionFile
	f.srv.newSessionFile = func(id sessionstore.ID, h sessionstore.Header) (*sessionstore.File, error) {
		calls.Add(1)
		entered <- id
		<-release
		return create(id, h)
	}
	f.t.Cleanup(func() { testutil.CloseSignal(release) })
	return entered, release, calls
}

// A CREATE_SESSION's file is created without the session store's lock:
// while the creation waits on its disk, another session's requests, the
// metrics and the creation's own retransmission are answered, the last
// with DELAY and no second file.
func TestCreateSessionFileOutsideStoreLock(t *testing.T) {
	f := newReplayFixture(t)
	entered, release, calls := gatedCreate(f)
	clientID, csSeq, reply := createSecondSession(t, f, "second-client")
	var id sessionstore.ID
	select {
	case id = <-entered:
	case <-time.After(testutil.ChannelTimeout):
		t.Fatal("no session file created")
	}
	ping := buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) { w.AppendArgarray_Putrootfh() })
	slot, seq := f.ts.acquire()
	f.ts.release(slot)
	var pinged []byte
	within(t, "another session's SEQUENCE", func() {
		pinged, _ = sendRawRPC(f.conn, f.xid, procCompound, withSequence(ping, f.ts, slot, seq))
	})
	f.xid++
	if pinged == nil || compoundStatus(t, pinged) != nfs4.NFS4_OK {
		t.Fatal("another session's request failed")
	}
	within(t, "the metrics", func() { f.srv.MetricsSnapshot() })
	conn := dial(t, f.addr)
	defer conn.Close()
	again, err := sendRawRPC(conn, 3, procCompound, createSessionBody(clientID, csSeq))
	if err != nil {
		t.Fatal(err)
	}
	if res, err := parseCompoundReply(again); err != nil || res.Status() != nfs4.NFS4ERR_DELAY {
		t.Fatalf("retransmitted CREATE_SESSION %v %v, want DELAY", res.Status(), err)
	}
	testutil.CloseSignal(release)
	status, sid := createSessionStatus(t, reply)
	if status != nfs4.NFS4_OK || sessionstore.ID(sid) != id {
		t.Fatalf("CREATE_SESSION %s for %s, file made for %s", nfs4.Nfsstat4Name(status), sessionText(sid), id)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("%d session files created", n)
	}
	if sess := f.srv.sessions.lookup(sid); sess == nil || !sess.file.onDisk.Load() {
		t.Fatal("the new session does not have its file")
	}
	noOrphanSessionFiles(t, f.srv)
}

// A CREATE_SESSION whose client record is retired while its file is being
// created inserts nothing, and its file goes with the creation.
func TestCreateSessionRefusedAfterFileRemovesIt(t *testing.T) {
	f := newReplayFixture(t)
	entered, release, _ := gatedCreate(f)
	clientID, _, reply := createSecondSession(t, f, "second-client")
	var id sessionstore.ID
	select {
	case id = <-entered:
	case <-time.After(testutil.ChannelTimeout):
		t.Fatal("no session file created")
	}
	ss := f.srv.sessions
	within(t, "retiring the record", func() {
		ss.mu.Lock()
		ss.retireWireLocked(ss.clients[clientID])
		ss.mu.Unlock()
	})
	testutil.CloseSignal(release)
	if status, _ := createSessionStatus(t, reply); status == nfs4.NFS4_OK {
		t.Fatal("CREATE_SESSION of a retired record succeeded")
	}
	if _, err := os.Stat(f.srv.sessionFiles.Path(id)); !os.IsNotExist(err) {
		t.Fatalf("refused creation's file: %v", err)
	}
	if a := ss.accounting(); a.Sessions != 1 || a.PendingSessions != 0 {
		t.Fatalf("after the refused creation %+v", a)
	}
	noOrphanSessionFiles(t, f.srv)
}

// A session file whose creation fails may leave a file behind. When the
// leftover can be removed, the session is made in memory only; when it
// cannot, a restart could find it, and CREATE_SESSION fails instead.
func TestCreateSessionFileFailureLeftover(t *testing.T) {
	for _, removable := range []bool{true, false} {
		name := "unremovable"
		if removable {
			name = "removable"
		}
		t.Run(name, func(t *testing.T) {
			f := newReplayFixture(t)
			var path string
			f.srv.newSessionFile = func(id sessionstore.ID, h sessionstore.Header) (*sessionstore.File, error) {
				path = f.srv.sessionFiles.Path(id)
				if removable {
					if err := os.WriteFile(path, []byte("partial"), 0o600); err != nil {
						panic(err)
					}
				} else {
					// A directory that is not empty cannot be removed as
					// a file is, even by root.
					if err := os.MkdirAll(filepath.Join(path, "x"), 0o700); err != nil {
						panic(err)
					}
				}
				return nil, errors.New("no space left on device")
			}
			_, _, reply := createSecondSession(t, f, "second-client")
			status, sid := createSessionStatus(t, reply)
			a := f.srv.sessions.accounting()
			if removable {
				if status != nfs4.NFS4_OK {
					t.Fatalf("CREATE_SESSION %s", nfs4.Nfsstat4Name(status))
				}
				if sess := f.srv.sessions.lookup(sid); sess == nil || sess.file.onDisk.Load() {
					t.Fatal("session missing or with a file")
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("leftover: %v", err)
				}
			} else {
				if status != nfs4.NFS4ERR_SERVERFAULT {
					t.Fatalf("CREATE_SESSION %s, want SERVERFAULT", nfs4.Nfsstat4Name(status))
				}
				if a.Sessions != 1 || a.PendingSessions != 0 {
					t.Fatalf("after the failed creation %+v", a)
				}
			}
		})
	}
}

// sessionIDOf builds a session ID from its epoch, incarnation and counter.
func sessionIDOf(epoch uint32, incarnation uint64, counter uint32) SessionID {
	var id SessionID
	binary.BigEndian.PutUint32(id[0:4], epoch)
	binary.BigEndian.PutUint64(id[4:12], incarnation)
	binary.BigEndian.PutUint32(id[12:16], counter)
	return id
}

// Session IDs carry the issuing process's epoch, which is random, and a
// restored session keeps the ID an earlier process gave it. An ID of this
// process's epoch that it has seen, a restored session's or one named by a
// file it refused, is not given to a new session, also once that session
// is gone; and one that would need the counter past its last value is not
// given at all, with no credit kept.
func TestKnownSessionIDsAreNotReissued(t *testing.T) {
	t.Run("restored session retired", func(t *testing.T) {
		ss := newSessionStore(7)
		old := sessionIDOf(7, 42, 1)
		sess := &session{id: old, clientID: 42, restored: true}
		ss.mu.Lock()
		ss.insertSessionLocked(sess)
		ss.mu.Unlock()
		ss.retireSession(sess, false)
		id, ok := ss.reserveSessionRecord(42)
		if !ok {
			t.Fatal("no ID")
		}
		defer ss.releaseSessionRecord()
		if id == old {
			t.Fatal("a retired restored session's ID was given again")
		}
	})
	// A session file this process refuses at startup still names an ID.
	refused := func(t *testing.T, counter uint32) *Server {
		srv, _, _ := persistedServer(t, vfs.NewLocalTernVFS(t.TempDir()))
		id := sessionIDOf(srv.sessions.epoch, 0x2000000000009999, counter)
		rec := sessionRecord{incarnation: 0x2000000000009999, wireClientID: 7,
			fore: channelAttrs{maxReq: 1 << 20, maxResp: 1 << 20, maxCached: 64 << 10, maxOps: 16, maxReqs: 8}}
		file, err := srv.sessionFiles.Create(sessionstore.ID(id), sessionstore.Header{SlotCount: 8, AreaSize: slotAreaSize(64 << 10), Session: rec.encode()})
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Write(0, sessionstore.Record{Seq: 1, Kind: sessionstore.KindSlot, User: encodeUser(effectiveUser{})}); err != nil {
			t.Fatal(err)
		}
		file.Close()
		srv.restoreSessions()
		if _, err := os.Stat(srv.sessionFiles.Path(sessionstore.ID(id))); !os.IsNotExist(err) {
			t.Fatalf("refused file kept: %v", err)
		}
		return srv
	}
	t.Run("refused file", func(t *testing.T) {
		srv := refused(t, 5)
		id, ok := srv.sessions.reserveSessionRecord(0x2000000000009999)
		if !ok {
			t.Fatal("no ID")
		}
		defer srv.sessions.releaseSessionRecord()
		if got := binary.BigEndian.Uint32(id[12:16]); got <= 5 {
			t.Fatalf("new session counter %d, not past the refused file's 5", got)
		}
	})
	t.Run("last counter value known", func(t *testing.T) {
		ss := refused(t, ^uint32(0)).sessions
		if _, ok := ss.reserveSessionRecord(0x2000000000009999); ok {
			t.Fatal("an ID was given past the counter's last value")
		}
		ss.mu.Lock()
		next, pending := ss.next, ss.pendingSessions
		ss.mu.Unlock()
		if next != uint64(^uint32(0)) || pending != 0 {
			t.Fatalf("counter %d, pending %d", next, pending)
		}
	})
}
