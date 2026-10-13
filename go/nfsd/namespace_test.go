// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

type namespaceFaultVFS struct {
	TernVFS
	err    error
	apply  bool
	remote bool
}

func (fs *namespaceFaultVFS) fault(mutate func() error) error {
	if fs.apply {
		if err := mutate(); err != nil {
			return err
		}
	}
	return fs.err
}

func (fs *namespaceFaultVFS) RemoveEdge(dir InodeID, name string, edge Edge) error {
	return fs.fault(func() error { return fs.TernVFS.RemoveEdge(dir, name, edge) })
}

func (fs *namespaceFaultVFS) RenameEdge(dir InodeID, name string, edge Edge, dst InodeID, newName string) error {
	return fs.fault(func() error { return fs.TernVFS.RenameEdge(dir, name, edge, dst, newName) })
}

func (fs *namespaceFaultVFS) ClassifyMutation(op uint32, err error) mutationOutcome {
	if fs.remote {
		return (&RemoteTernVFS{}).ClassifyMutation(op, err)
	}
	return fs.TernVFS.ClassifyMutation(op, err)
}

func commitNamespaceStatus(t *testing.T, f *namespaceFixture) uint32 {
	t.Helper()
	res := sendCompound(t, f.conn, f.xid, func(w *COMPOUND4argsWriter) {
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(f.fh).Finish())
		w.Resume(pw.Finish())
		w.AppendArgarray_Commit()
	})
	f.xid++
	return res.Status()
}

func namespaceOperation(t *testing.T, f *namespaceFixture, operation string) uint32 {
	t.Helper()
	switch operation {
	case "remove":
		return removeNamespaceFile(t, f, "source")
	case "rename":
		return renameNamespaceFile(t, f, "source", "target")
	case "replace":
		return renameNamespaceFile(t, f, "target", "source")
	default:
		t.Fatalf("unknown operation %q", operation)
		return NFS4ERR_IO
	}
}

func TestNamespaceErrorOutcomes(t *testing.T) {
	for _, operation := range []string{"remove", "rename", "replace"} {
		for _, tc := range []struct {
			name     string
			err      error
			apply    bool
			remote   bool
			status   uint32
			failed   bool
			unlinked bool
		}{
			{"local rejection", os.ErrPermission, false, false, NFS4ERR_ACCESS, false, false},
			{"lost reply", errors.New("lost reply"), true, false, NFS4ERR_IO, true, false},
			{"send failure", errors.New("send failure"), false, false, NFS4ERR_IO, true, false},
			{"submitted rejection", msgs.NOT_AUTHORISED, false, true, NFS4ERR_IO, true, false},
			{"malformed response", msgs.MALFORMED_RESPONSE, true, true, NFS4ERR_IO, true, false},
			{"edge gone", msgs.EDGE_NOT_FOUND, true, true, NFS4ERR_NOENT, false, true},
			{"edge replaced", msgs.MISMATCHING_CREATION_TIME, true, true, NFS4ERR_NOENT, false, true},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				f := newNamespaceFixture(t)
				f.server.fs = &namespaceFaultVFS{TernVFS: f.fs, err: tc.err, apply: tc.apply, remote: tc.remote}
				if operation != "remove" && tc.unlinked {
					tc.status, tc.failed, tc.unlinked = NFS4ERR_IO, true, false
				}
				if got := namespaceOperation(t, f, operation); got != tc.status {
					t.Fatalf("%s = %s, want %s", operation, Nfsstat4Name(got), Nfsstat4Name(tc.status))
				}
				if tc.failed {
					assertQuarantined(t, f, f.id)
					if !f.store.Failed(f.id) {
						t.Fatal("unknown outcome lost the tombstone")
					}
					if got := commitNamespaceStatus(t, f); got != NFS4ERR_IO {
						t.Fatalf("failed COMMIT = %s", Nfsstat4Name(got))
					}
					if got := closeFileWithSeqStatus(t, f.conn, &f.xid, f.fh, f.state, 3); got != NFS4ERR_EXPIRED {
						t.Fatalf("failed CLOSE = %s", Nfsstat4Name(got))
					}
					if got := commitNamespaceStatus(t, f); got != NFS4ERR_IO {
						t.Fatalf("COMMIT after failed CLOSE = %s", Nfsstat4Name(got))
					}
					return
				}
				if !tc.unlinked {
					assertRejectedNamespaceWriter(t, f)
					return
				}
				meta, ok := f.store.GetMeta(f.id)
				if !ok || !meta.Unlinked || meta.Guarded {
					t.Fatalf("edge-gone writer was not detached: %+v", meta)
				}
				writeFileAt(t, f.conn, &f.xid, f.fh, f.state, 0, []byte("detached bytes"))
				closeFile(t, f.conn, &f.xid, f.fh, f.state)
				if _, err := os.Stat(filepath.Join(f.root, "source")); !os.IsNotExist(err) {
					t.Fatalf("detached writer published: %v", err)
				}
			})
		}
	}
}

func TestRenameStagedSourceAndDestination(t *testing.T) {
	for _, destinationFirst := range []bool{false, true} {
		for _, restart := range []bool{false, true} {
			t.Run(fmt.Sprintf("destination-first=%t/restart=%t", destinationFirst, restart), func(t *testing.T) {
				f := newNamespaceFixture(t)
				state, fh := openWriteFile(t, f.conn, &f.xid, f.clientID, "target")
				destinationID, _ := fhToInodeID(fh)
				writeFileAt(t, f.conn, &f.xid, f.fh, f.state, 0, []byte("source writer"))
				writeFileAt(t, f.conn, &f.xid, fh, state, 0, []byte("target writer"))
				if status := renameNamespaceFile(t, f, "source", "target"); status != NFS4_OK {
					t.Fatalf("RENAME = %s", Nfsstat4Name(status))
				}
				src, _ := f.store.GetMeta(f.id)
				dst, _ := f.store.GetMeta(destinationID)
				if src.Unlinked || src.Guarded || src.FileName != "target" || !dst.Unlinked || dst.Guarded {
					t.Fatalf("wrong writer dispositions: source=%+v destination=%+v", src, dst)
				}
				if restart {
					f.restart(t)
				}
				if destinationFirst {
					closeFile(t, f.conn, &f.xid, fh, state)
					closeFile(t, f.conn, &f.xid, f.fh, f.state)
				} else {
					closeFile(t, f.conn, &f.xid, f.fh, f.state)
					closeFile(t, f.conn, &f.xid, fh, state)
				}
				data, err := os.ReadFile(filepath.Join(f.root, "target"))
				if err != nil || string(data) != "source writer" {
					t.Fatalf("destination CLOSE overwrote source: %q, %v", data, err)
				}
				if _, err := os.Stat(filepath.Join(f.root, "source")); !os.IsNotExist(err) {
					t.Fatalf("source CLOSE recreated its old name: %v", err)
				}
			})
		}
	}
}

type namespaceFixture struct {
	root, disk string
	fs         *LocalTernVFS
	backend    TernVFS // serves in place of fs when set, including after restart
	store      *LocalStagingStore
	server     *Server
	conn       net.Conn
	stop       func()
	xid        uint32
	clientID   uint64
	id         InodeID
	state      [16]byte
	fh         []byte
}

type blockingNamespaceRemove struct {
	TernVFS
	entered, release chan struct{}
}

func (fs blockingNamespaceRemove) RemoveEdge(dirID InodeID, name string, edge Edge) error {
	if name == "source" {
		close(fs.entered)
		<-fs.release
	}
	return fs.TernVFS.RemoveEdge(dirID, name, edge)
}

func newNamespaceFixture(t *testing.T) *namespaceFixture {
	t.Helper()
	f := openNamespaceFixture(t)
	writeFileAt(t, f.conn, &f.xid, f.fh, f.state, 0, []byte("private data"))
	return f
}

func openNamespaceFixture(t *testing.T) *namespaceFixture {
	t.Helper()
	f := &namespaceFixture{root: t.TempDir(), disk: t.TempDir(), xid: 1}
	for name, data := range map[string]string{"source": "original", "target": "replacement"} {
		if err := os.WriteFile(filepath.Join(f.root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	f.fs = NewLocalTernVFS(f.root)
	f.start(t)
	t.Cleanup(func() { f.close(t) })
	f.clientID = setupClient(t, f.conn, &f.xid)
	f.state, f.fh = openWriteFile(t, f.conn, &f.xid, f.clientID, "source")
	f.id, _ = fhToInodeID(f.fh)
	return f
}

func removeNamespaceFile(t *testing.T, f *namespaceFixture, name string) uint32 {
	t.Helper()
	res := sendCompound(t, f.conn, f.xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		rw := w.AppendArgarray_Remove()
		rw.Resume(rw.StartTarget().SetData([]byte(name)).Finish())
		w.Resume(rw.Finish())
	})
	f.xid++
	return res.Status()
}

func renameNamespaceFile(t *testing.T, f *namespaceFixture, from, to string) uint32 {
	t.Helper()
	res := sendCompound(t, f.conn, f.xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Savefh()
		rw := w.AppendArgarray_Rename()
		rw.Resume(rw.StartOldname().SetData([]byte(from)).Finish())
		rw.Resume(rw.StartNewname().SetData([]byte(to)).Finish())
		w.Resume(rw.Finish())
	})
	f.xid++
	return res.Status()
}

func expireNamespaceClient(t *testing.T, f *namespaceFixture) {
	t.Helper()
	f.server.waitForClientGC()
	now := time.Now().Add(2 * nfsLeaseTime)
	f.server.clients.now = func() time.Time { return now }
	f.server.runLeaseSweep()
}

func assertQuarantined(t *testing.T, f *namespaceFixture, id InodeID) {
	t.Helper()
	if f.store.Get(id) != nil {
		t.Fatal("quarantined writer is still indexed")
	}
	matches, _ := filepath.Glob(filepath.Join(f.disk, stagingQuarantineDirName,
		fmt.Sprintf("%016x-*", uint64(id)), fmt.Sprintf("%016x.staging", uint64(id))))
	if len(matches) != 1 {
		t.Fatalf("quarantined staging files = %v", matches)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil || string(data) != "private data" {
		t.Fatalf("quarantined bytes = %q, %v", data, err)
	}
}

func assertRejectedNamespaceWriter(t *testing.T, f *namespaceFixture) {
	t.Helper()
	meta, _ := f.store.GetMeta(f.id)
	if meta.Guarded || meta.Unlinked || meta.FileName != "source" {
		t.Fatalf("rejection did not restore writer: %+v", meta)
	}
	data, _ := readFileData(t, f.conn, &f.xid, f.fh, 0, 128)
	if string(data) != "private data" {
		t.Fatalf("rejection lost private bytes: %q", data)
	}
	closeFile(t, f.conn, &f.xid, f.fh, f.state)
	data, err := os.ReadFile(filepath.Join(f.root, "source"))
	if err != nil || string(data) != "private data" {
		t.Fatalf("rejected operation prevented publication: %q, %v", data, err)
	}
}

func TestRemoveOpenStaging(t *testing.T) {
	f := newNamespaceFixture(t)
	if status := removeNamespaceFile(t, f, "source"); status != NFS4_OK {
		t.Fatalf("REMOVE returned %s", Nfsstat4Name(status))
	}
	meta, _ := f.store.GetMeta(f.id)
	if !meta.Unlinked || meta.Guarded ||
		len(f.store.FindTargets(f.fs.RootID(), "source")) != 0 {
		t.Fatalf("writer was not detached: %+v", meta)
	}
	// Recreate the pathname before closing the old descriptor.
	status, state, fh, _ := openFileForOwner(t, f.conn, &f.xid, f.clientID,
		"recreated", 1, "source", OPEN4_SHARE_ACCESS_BOTH, true, false)
	if status != NFS4_OK {
		t.Fatalf("recreate returned %s", Nfsstat4Name(status))
	}
	state = confirmOpenState(t, f.conn, &f.xid, fh, 2, state)
	writeFileAt(t, f.conn, &f.xid, fh, state, 0, []byte("new pathname"))
	closeFile(t, f.conn, &f.xid, fh, state)
	writeFileAt(t, f.conn, &f.xid, f.fh, f.state, 0, []byte("old descriptor"))
	data, _ := readFileData(t, f.conn, &f.xid, f.fh, 0, 128)
	if string(data) != "old descriptor" {
		t.Fatalf("unlinked handle read = %q", data)
	}
	closeFile(t, f.conn, &f.xid, f.fh, f.state)
	data, err := os.ReadFile(filepath.Join(f.root, "source"))
	if err != nil || string(data) != "new pathname" {
		t.Fatalf("old CLOSE overwrote recreated name: %q, %v", data, err)
	}
}

func TestDetachedWriterReadsBaseSnapshot(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprintf("replace=%t", replace), func(t *testing.T) {
			f := openNamespaceFixture(t)
			var status uint32
			if replace {
				status = renameNamespaceFile(t, f, "target", "source")
			} else {
				status = removeNamespaceFile(t, f, "source")
			}
			if status != NFS4_OK {
				t.Fatalf("detach returned %s", Nfsstat4Name(status))
			}
			meta, _ := f.store.GetMeta(f.id)
			if !meta.Unlinked || len(meta.Dirty) != 0 {
				t.Fatalf("detach changed the staged data: %+v", meta)
			}
			for range 2 {
				data, _ := readFileData(t, f.conn, &f.xid, f.fh, 0, 128)
				if string(data) != "original" {
					t.Fatalf("detached read = %q", data)
				}
				f.restart(t)
			}
			closeFile(t, f.conn, &f.xid, f.fh, f.state)
		})
	}
}

func TestRetiredWritersFollowNamespaceChanges(t *testing.T) {
	for _, operation := range []string{"remove", "rename", "replace"} {
		t.Run(operation, func(t *testing.T) {
			f := newNamespaceFixture(t)
			expireNamespaceClient(t, f)
			meta, _ := f.store.GetMeta(f.id)
			if !meta.Retired {
				t.Fatal("fixture writer did not retire")
			}
			var status uint32
			switch operation {
			case "remove":
				status = removeNamespaceFile(t, f, "source")
			case "rename":
				status = renameNamespaceFile(t, f, "source", "target")
			case "replace":
				status = renameNamespaceFile(t, f, "target", "source")
			}
			if status != NFS4_OK {
				t.Fatalf("%s returned %s", operation, Nfsstat4Name(status))
			}
			if operation != "rename" {
				assertQuarantined(t, f, f.id)
				f.restart(t)
				if f.store.Get(f.id) != nil {
					t.Fatal("quarantined retired writer returned on restart")
				}
				return
			}
			meta, _ = f.store.GetMeta(f.id)
			if meta.FileName != "target" || meta.Guarded {
				t.Fatalf("retired writer stayed at the old name: %+v", meta)
			}
			clientID := setupClient(t, f.conn, &f.xid)
			status, state, fh, _ := openFileForOwner(t, f.conn, &f.xid, clientID,
				"test-owner-source", 1, "target", OPEN4_SHARE_ACCESS_BOTH, false, false)
			if status != NFS4_OK {
				t.Fatalf("reclaim returned %s", Nfsstat4Name(status))
			}
			state = confirmOpenState(t, f.conn, &f.xid, fh, 2, state)
			closeFile(t, f.conn, &f.xid, fh, state)
			if _, err := os.Stat(filepath.Join(f.root, "source")); !os.IsNotExist(err) {
				t.Fatalf("reclaim resurrected old source: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(f.root, "target"))
			if err != nil || string(data) != "private data" {
				t.Fatalf("reclaimed data = %q, %v", data, err)
			}
		})
	}
}

func TestExpiredDetachedWritersAreCollected(t *testing.T) {
	for _, retiredBeforeRestart := range []bool{false, true} {
		t.Run(fmt.Sprintf("already-retired=%t", retiredBeforeRestart), func(t *testing.T) {
			f := newNamespaceFixture(t)
			if status := removeNamespaceFile(t, f, "source"); status != NFS4_OK {
				t.Fatal(Nfsstat4Name(status))
			}
			if retiredBeforeRestart {
				meta, _ := f.store.GetMeta(f.id)
				if err := f.store.Get(f.id).Retire(meta.ClientID, meta.NFSStateID); err != nil {
					t.Fatal(err)
				}
				f.restart(t)
			}
			expireNamespaceClient(t, f)
			for _, suffix := range []string{".staging", ".meta"} {
				_, err := os.Stat(filepath.Join(f.disk, fmt.Sprintf("%016x%s", uint64(f.id), suffix)))
				if !os.IsNotExist(err) {
					t.Fatalf("expired detached %s survived: %v", suffix, err)
				}
			}
			// A writer which was retired before it was detached was headed for
			// quarantine; a crash before that must not turn into deletion.
			if retiredBeforeRestart {
				assertQuarantined(t, f, f.id)
			}
			f.restart(t)
			if f.store.Get(f.id) != nil {
				t.Fatal("collected writer returned")
			}
		})
	}
}

func TestDetachedCloseDoesNotLockOldName(t *testing.T) {
	f := newNamespaceFixture(t)
	if status := removeNamespaceFile(t, f, "source"); status != NFS4_OK {
		t.Fatal(Nfsstat4Name(status))
	}
	unlock := f.server.lockMutationTargets(mutationTarget{dirID: f.fs.RootID(), name: "source"})
	defer unlock()
	f.conn.SetDeadline(time.Now().Add(time.Second))
	closeFile(t, f.conn, &f.xid, f.fh, f.state)
}

func TestAbandonedOpenWaitsForNamespaceChange(t *testing.T) {
	f := newNamespaceFixture(t)
	status, _, fh, _ := openFileForOwner(t, f.conn, &f.xid, f.clientID,
		"unconfirmed", 1, "source", OPEN4_SHARE_ACCESS_BOTH, false, false)
	if status != NFS4_OK {
		t.Fatal(Nfsstat4Name(status))
	}
	abandonedID, _ := fhToInodeID(fh)
	fs := blockingNamespaceRemove{TernVFS: f.fs, entered: make(chan struct{}), release: make(chan struct{})}
	f.server.fs = fs
	removeF := *f
	removeF.conn = dial(t, f.conn.RemoteAddr().String())
	defer removeF.conn.Close()
	removeF.xid = 1000
	removed := make(chan uint32, 1)
	go func() { removed <- removeNamespaceFile(t, &removeF, "source") }()
	awaitSignal(t, fs.entered, "backend REMOVE")
	type openResult struct {
		status uint32
		err    error
	}
	opened := make(chan openResult, 1)
	go func() {
		status, _, _, _, err := openFileForOwnerE(f.conn, &f.xid, f.clientID,
			"unconfirmed", 2, "next", OPEN4_SHARE_ACCESS_BOTH, true, false)
		opened <- openResult{status, err}
	}()
	select {
	case result := <-opened:
		close(fs.release)
		<-removed
		t.Fatalf("abandoned-open cleanup did not wait: %+v", result)
	case <-time.After(50 * time.Millisecond):
	}
	close(fs.release)
	if status := <-removed; status != NFS4_OK {
		t.Fatal(Nfsstat4Name(status))
	}
	result := <-opened
	if result.err != nil || result.status != NFS4_OK {
		t.Fatalf("replacement OPEN = %+v", result)
	}
	if f.store.Get(abandonedID) != nil {
		t.Fatal("abandoned detached writer leaked")
	}
	for _, suffix := range []string{".meta", ".staging"} {
		_, err := os.Stat(filepath.Join(f.disk, fmt.Sprintf("%016x%s", uint64(abandonedID), suffix)))
		if !os.IsNotExist(err) {
			t.Fatalf("abandoned sidecar/data remained: %v", err)
		}
	}
}

func (f *namespaceFixture) start(t *testing.T) {
	t.Helper()
	var err error
	f.store, err = NewLocalStagingStore(f.disk, nil)
	if err != nil {
		t.Fatal(err)
	}
	backend := TernVFS(f.fs)
	if f.backend != nil {
		backend = f.backend
	}
	f.server, err = NewServer(backend, f.store, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, f.server)
	f.stop = stop
	f.conn = dial(t, addr)
}

func (f *namespaceFixture) close(t *testing.T) {
	t.Helper()
	f.conn.Close()
	f.stop()
	closeLocalStagingFiles(t, f.store)
}

func (f *namespaceFixture) restart(t *testing.T) {
	t.Helper()
	f.close(t)
	f.start(t)
}

func renameNamespaceIntoDirectory(t *testing.T, f *namespaceFixture) uint32 {
	t.Helper()
	dirFH := lookupFH(t, f.conn, &f.xid, "other")
	res := sendCompound(t, f.conn, f.xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		w.AppendArgarray_Savefh()
		pw := w.AppendArgarray_Putfh()
		pw.Resume(pw.StartObject().SetData(dirFH).Finish())
		w.Resume(pw.Finish())
		rw := w.AppendArgarray_Rename()
		rw.Resume(rw.StartOldname().SetData([]byte("source")).Finish())
		rw.Resume(rw.StartNewname().SetData([]byte("moved")).Finish())
		w.Resume(rw.Finish())
	})
	f.xid++
	return res.Status()
}

func TestRetiredSourceDoesNotBlockRename(t *testing.T) {
	for _, destination := range []string{"absent", "retired", "active", "rejected", "checkpoint failure"} {
		t.Run(destination, func(t *testing.T) {
			f := newNamespaceFixture(t)
			expireNamespaceClient(t, f)
			dir, err := f.fs.Mkdir(f.fs.RootID(), "other")
			if err != nil {
				t.Fatal(err)
			}
			var destinationID InodeID
			if destination == "rejected" {
				if _, err := f.fs.Mkdir(dir, "moved"); err != nil {
					t.Fatal(err)
				}
			} else if destination != "absent" {
				base, err := f.fs.CreateFile(dir, "moved", strings.NewReader("displaced"))
				if err != nil {
					t.Fatal(err)
				}
				var cookie Cookie
				destinationID, cookie, err = f.fs.ConstructFile(dir)
				if err != nil {
					t.Fatal(err)
				}
				clientID := f.clientID
				if destination == "active" {
					clientID = setupClient(t, f.conn, &f.xid)
				}
				stateID := f.server.opens.newStateID()
				sf, err := f.store.Create(destinationID, StagingMeta{
					DirID: dir, FileName: "moved", BaseID: base, BaseSize: 9,
					ClientID: clientID, NFSStateID: stateID, TernCookie: cookie,
				})
				if err != nil {
					t.Fatal(err)
				}
				if destination == "active" {
					if err := f.server.clients.MarkOpen(clientID, stateID); err != nil {
						t.Fatal(err)
					}
				} else if err := sf.Retire(clientID, stateID); err != nil {
					t.Fatal(err)
				}
			}
			var savedMetaPath string
			if destination == "checkpoint failure" {
				sf := f.store.Get(destinationID).(*localStagingFile)
				savedMetaPath = sf.metaPath
				sf.metaPath = filepath.Join(f.disk, "missing", "destination.meta")
			}
			status := renameNamespaceIntoDirectory(t, f)
			if savedMetaPath != "" {
				f.store.Get(destinationID).(*localStagingFile).metaPath = savedMetaPath
			}
			if destination == "rejected" || destination == "checkpoint failure" {
				if status == NFS4_OK || status == NFS4ERR_FILE_OPEN {
					t.Fatalf("failed rename returned %s", Nfsstat4Name(status))
				}
				for _, id := range []InodeID{f.id, destinationID} {
					if id == 0 {
						continue
					}
					meta, ok := f.store.GetMeta(id)
					if !ok || meta.Guarded || meta.Unlinked || !meta.Retired {
						t.Fatalf("failed rename changed retired writer: %+v, %t", meta, ok)
					}
				}
				if _, err := os.Stat(filepath.Join(f.root, "source")); err != nil {
					t.Fatalf("failed rename lost source: %v", err)
				}
				return
			}
			if status != NFS4_OK {
				t.Fatalf("retired-only source blocked rename: %s", Nfsstat4Name(status))
			}
			assertQuarantined(t, f, f.id)
			if destinationID != 0 {
				meta, ok := f.store.GetMeta(destinationID)
				if destination == "active" {
					if !ok || !meta.Unlinked || meta.Guarded {
						t.Fatalf("active destination was not detached: %+v, %t", meta, ok)
					}
				} else if ok {
					t.Fatal("retired destination staging was retained")
				}
			}
			data, err := os.ReadFile(filepath.Join(f.root, "other", "moved"))
			if err != nil || string(data) != "original" {
				t.Fatalf("moved file = %q, %v", data, err)
			}
			if _, err := os.Stat(filepath.Join(f.root, "source")); !os.IsNotExist(err) {
				t.Fatalf("source reappeared: %v", err)
			}
			f.restart(t)
			if f.store.Get(f.id) != nil {
				t.Fatal("quarantined source writer returned on restart")
			}
		})
	}
}

func TestActiveSourceStillBlocksCrossDirectoryRename(t *testing.T) {
	f := newNamespaceFixture(t)
	expireNamespaceClient(t, f)
	if _, err := f.fs.Mkdir(f.fs.RootID(), "other"); err != nil {
		t.Fatal(err)
	}
	clientID := setupClient(t, f.conn, &f.xid)
	status, state, fh, _ := openFileForOwner(t, f.conn, &f.xid, clientID,
		"live-source", 1, "source", OPEN4_SHARE_ACCESS_BOTH, false, false)
	if status != NFS4_OK {
		t.Fatal(Nfsstat4Name(status))
	}
	state = confirmOpenState(t, f.conn, &f.xid, fh, 2, state)
	if status := renameNamespaceIntoDirectory(t, f); status != NFS4ERR_FILE_OPEN {
		t.Fatalf("live source returned %s, want FILE_OPEN", Nfsstat4Name(status))
	}
	meta, _ := f.store.GetMeta(f.id)
	if !meta.Retired || meta.Guarded || meta.Unlinked {
		t.Fatalf("rejected move changed retired sibling: %+v", meta)
	}
	closeFile(t, f.conn, &f.xid, fh, state)
	if status := renameNamespaceIntoDirectory(t, f); status != NFS4_OK {
		t.Fatalf("closed source still blocked rename: %s", Nfsstat4Name(status))
	}
}

func TestRenameOpenStaging(t *testing.T) {
	f := newNamespaceFixture(t)
	// A writer based on a different published version must also follow the
	// pathname. This occurs when one writer publishes while another is open.
	other, err := f.fs.CreateFile(f.fs.RootID(), "source", nil)
	if err != nil {
		t.Fatal(err)
	}
	status, state, fh, _ := openFileForOwner(t, f.conn, &f.xid, f.clientID,
		"second-owner", 1, "source", OPEN4_SHARE_ACCESS_BOTH, false, false)
	if status != NFS4_OK {
		t.Fatalf("second OPEN returned %s", Nfsstat4Name(status))
	}
	state = confirmOpenState(t, f.conn, &f.xid, fh, 2, state)
	second, _ := fhToInodeID(fh)
	meta, _ := f.store.GetMeta(second)
	if meta.BaseID != other {
		t.Fatal("fixture did not open a different base inode")
	}
	if status := renameNamespaceFile(t, f, "source", "target"); status != NFS4_OK {
		t.Fatalf("RENAME returned %s", Nfsstat4Name(status))
	}
	for _, id := range []InodeID{f.id, second} {
		meta, _ := f.store.GetMeta(id)
		if meta.Guarded || meta.FileName != "target" {
			t.Fatalf("writer did not follow rename: %+v", meta)
		}
	}
	writeFileAt(t, f.conn, &f.xid, f.fh, f.state, 0, []byte("first version"))
	writeFileAt(t, f.conn, &f.xid, fh, state, 0, []byte("last version"))
	f.restart(t)
	closeFile(t, f.conn, &f.xid, f.fh, f.state)
	closeFile(t, f.conn, &f.xid, fh, state)
	data, err := os.ReadFile(filepath.Join(f.root, "target"))
	if err != nil || string(data) != "last version" {
		t.Fatalf("last writer did not publish at new name: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(f.root, "source")); !os.IsNotExist(err) {
		t.Fatalf("old name reappeared: %v", err)
	}
}

func TestCloseWaitsForStagedRename(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "existing.txt"), []byte("original"), 0644,
	); err != nil {
		t.Fatal(err)
	}
	baseFS := NewLocalTernVFS(dir)
	fs := &blockingRenameVFS{
		TernVFS: baseFS,
		dstName: "renamed.txt",
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
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
	conn := dial(t, addr)
	defer conn.Close()

	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	stateID, fh := openWriteFile(
		t, conn, &xid, clientID, "existing.txt",
	)
	writeFileAt(t, conn, &xid, fh, stateID, 0, []byte("updated!"))

	renameDone := make(chan uint32, 1)
	go func() {
		renameConn := dial(t, addr)
		defer renameConn.Close()
		res := sendCompound(t, renameConn, 100, func(w *COMPOUND4argsWriter) {
			w.AppendArgarray_Putrootfh()
			w.AppendArgarray_Savefh()
			rw := w.AppendArgarray_Rename()
			buf := rw.StartOldname().SetData([]byte("existing.txt")).Finish()
			rw.Resume(buf)
			buf = rw.StartNewname().SetData([]byte("renamed.txt")).Finish()
			rw.Resume(buf)
			w.Resume(rw.Finish())
		})
		renameDone <- res.Status()
	}()
	<-fs.entered

	closeDone := make(chan uint32, 1)
	go func() {
		closeConn := dial(t, addr)
		defer closeConn.Close()
		closeXID := uint32(200)
		closeDone <- closeFileWithSeqStatus(
			t, closeConn, &closeXID, fh, stateID, 3,
		)
	}()
	select {
	case status := <-closeDone:
		close(fs.release)
		t.Fatalf("CLOSE completed during RENAME with status %s",
			Nfsstat4Name(status))
	case <-time.After(100 * time.Millisecond):
	}
	close(fs.release)

	if status := <-renameDone; status != NFS4_OK {
		t.Fatalf("RENAME status = %s, want NFS4_OK",
			Nfsstat4Name(status))
	}
	if status := <-closeDone; status != NFS4_OK {
		t.Fatalf("CLOSE status = %s, want NFS4_OK",
			Nfsstat4Name(status))
	}
	if _, err := os.Stat(filepath.Join(dir, "existing.txt")); !os.IsNotExist(err) {
		t.Fatalf("old name exists after CLOSE: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "renamed.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "updated!" {
		t.Fatalf("renamed file content = %q, want %q", data, "updated!")
	}
}

func TestRenameOverOpenStaging(t *testing.T) {
	f := newNamespaceFixture(t)
	if status := renameNamespaceFile(t, f, "target", "source"); status != NFS4_OK {
		t.Fatalf("RENAME returned %s", Nfsstat4Name(status))
	}
	meta, _ := f.store.GetMeta(f.id)
	if !meta.Unlinked || meta.Guarded {
		t.Fatalf("destination writer not detached: %+v", meta)
	}
	writeFileAt(t, f.conn, &f.xid, f.fh, f.state, 0, []byte("old writer"))
	f.restart(t)
	closeFile(t, f.conn, &f.xid, f.fh, f.state)
	data, err := os.ReadFile(filepath.Join(f.root, "source"))
	if err != nil || string(data) != "replacement" {
		t.Fatalf("detached writer overwrote replacement: %q, %v", data, err)
	}
}
