// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type namespaceProbeVFS struct {
	TernVFS
	mutations atomic.Int32
	lookups   atomic.Int32
	before    func()
}

func (fs *namespaceProbeVFS) LookupEdge(dir InodeID, name string) (Edge, error) {
	fs.lookups.Add(1)
	return fs.TernVFS.LookupEdge(dir, name)
}

func (fs *namespaceProbeVFS) RemoveEdge(dir InodeID, name string, edge Edge) error {
	fs.mutations.Add(1)
	if fs.before != nil {
		fs.before()
	}
	return fs.TernVFS.RemoveEdge(dir, name, edge)
}

func (fs *namespaceProbeVFS) RenameEdge(dir InodeID, name string, edge Edge, dst InodeID, newName string) error {
	fs.mutations.Add(1)
	if fs.before != nil {
		fs.before()
	}
	return fs.TernVFS.RenameEdge(dir, name, edge, dst, newName)
}

func TestNamespacePreparationFailureKeepsWritersUsable(t *testing.T) {
	for _, installed := range []bool{false, true} {
		t.Run(fmt.Sprintf("installed=%t", installed), func(t *testing.T) {
			f := newNamespaceFixture(t)
			fs := &namespaceProbeVFS{TernVFS: f.fs}
			f.server.fs = fs
			// Source and destination writers exercise rollback of a successful
			// guard as well as the writer whose own guard failed.
			state, fh := openWriteFile(t, f.conn, &f.xid, f.clientID, "target")
			dstID, _ := fhToInodeID(fh)
			sf := f.store.Get(dstID).(*localStagingFile)
			path := sf.metaPath
			if installed {
				f.server.stagingStore = &installedGuardFailureStore{StagingStore: f.store, id: dstID}
			} else {
				sf.metaPath = filepath.Join(f.disk, "missing", "checkpoint.meta")
			}
			if got := renameNamespaceFile(t, f, "source", "target"); got != NFS4ERR_IO {
				t.Fatalf("preparation failure = %s", Nfsstat4Name(got))
			}
			sf.metaPath = path
			if fs.mutations.Load() != 0 {
				t.Fatal("preparation failure submitted a backend mutation")
			}
			for _, id := range []InodeID{f.id, dstID} {
				meta, ok := f.store.GetMeta(id)
				if !ok || meta.Guarded || meta.Unlinked || f.store.Failed(id) {
					t.Fatalf("preparation failure excluded a writer: %+v", meta)
				}
				if err := f.store.Get(id).Sync(); err != nil {
					t.Fatal(err)
				}
				disk, err := loadStagingMeta(f.store.Get(id).(*localStagingFile).metaPath)
				if err != nil || disk.Guarded {
					t.Fatalf("rollback was not repaired: %+v, %v", disk, err)
				}
			}
			closeFile(t, f.conn, &f.xid, fh, state)
			assertRejectedNamespaceWriter(t, f)
		})
	}
}

type installedGuardFailureStore struct {
	StagingStore
	id InodeID
}

func (store *installedGuardFailureStore) SetGuard(id InodeID, guarded bool) error {
	err := store.StagingStore.SetGuard(id, guarded)
	if err == nil && id == store.id && guarded {
		return errors.New("guard installed but directory sync failed")
	}
	return err
}

func TestNamespaceFinalSaveFailureKeepsBackendResult(t *testing.T) {
	for _, operation := range []string{"remove", "rename", "replace"} {
		t.Run(operation, func(t *testing.T) {
			f := newNamespaceFixture(t)
			sf := f.store.Get(f.id).(*localStagingFile)
			path := sf.metaPath
			f.server.fs = &namespaceProbeVFS{TernVFS: f.fs, before: func() {
				sf.mu.Lock()
				sf.metaPath = filepath.Join(f.disk, "missing", "checkpoint.meta")
				sf.mu.Unlock()
			}}
			if got := namespaceOperation(t, f, operation); got != NFS4_OK {
				t.Fatalf("known outcome changed to %s", Nfsstat4Name(got))
			}
			sf.metaPath = path
			meta, ok := f.store.GetMeta(f.id)
			if !ok || meta.Guarded || f.store.Failed(f.id) {
				t.Fatalf("final save failure excluded the writer: %+v", meta)
			}
			disk, err := loadStagingMeta(path)
			if err != nil || !disk.Guarded {
				t.Fatalf("failed save lost durable guard: %+v, %v", disk, err)
			}
			if got := commitNamespaceStatus(t, f); got != NFS4_OK {
				t.Fatalf("repair COMMIT = %s", Nfsstat4Name(got))
			}
			disk, err = loadStagingMeta(path)
			if err != nil || disk.Guarded || disk.Unlinked != meta.Unlinked || disk.FileName != meta.FileName {
				t.Fatalf("COMMIT did not repair final metadata: %+v, %v", disk, err)
			}
			closeFile(t, f.conn, &f.xid, f.fh, f.state)
			switch operation {
			case "remove":
				if _, err := os.Stat(filepath.Join(f.root, "source")); !os.IsNotExist(err) {
					t.Fatalf("CLOSE resurrected removed name: %v", err)
				}
			case "rename":
				data, err := os.ReadFile(filepath.Join(f.root, "target"))
				if err != nil || string(data) != "private data" {
					t.Fatalf("renamed writer did not publish: %q, %v", data, err)
				}
			case "replace":
				data, err := os.ReadFile(filepath.Join(f.root, "source"))
				if err != nil || string(data) != "replacement" {
					t.Fatalf("detached writer overwrote replacement: %q, %v", data, err)
				}
			}
		})
	}
}

func TestNamespaceFinalDirectorySyncFailureIsSafe(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory read permissions")
	}
	f := newNamespaceFixture(t)
	fs := &namespaceProbeVFS{TernVFS: f.fs, before: func() {
		// Renaming a sidecar needs write+execute. Opening its directory
		// for the final fsync also needs read permission.
		if err := os.Chmod(f.disk, 0300); err != nil {
			t.Fatal(err)
		}
	}}
	f.server.fs = fs
	defer os.Chmod(f.disk, 0700)
	if got := renameNamespaceFile(t, f, "source", "target"); got != NFS4_OK {
		t.Fatalf("installed final checkpoint returned %s", Nfsstat4Name(got))
	}
	if err := os.Chmod(f.disk, 0700); err != nil {
		t.Fatal(err)
	}
	meta, _ := f.store.GetMeta(f.id)
	if meta.Guarded || meta.FileName != "target" || !f.store.Get(f.id).(*localStagingFile).checkpointDirty {
		t.Fatalf("installed final checkpoint was not retained for repair: %+v", meta)
	}
	if got := commitNamespaceStatus(t, f); got != NFS4_OK {
		t.Fatal(Nfsstat4Name(got))
	}
	f.restart(t)
	closeFile(t, f.conn, &f.xid, f.fh, f.state)
	data, err := os.ReadFile(filepath.Join(f.root, "target"))
	if err != nil || string(data) != "private data" {
		t.Fatalf("recovered known target = %q, %v", data, err)
	}
}

type pausedFailureCheckStore struct {
	StagingStore
	id               InodeID
	paused           atomic.Bool
	entered, release chan struct{}
}

func (store *pausedFailureCheckStore) Failed(id InodeID) bool {
	failed := store.StagingStore.Failed(id)
	if id == store.id && store.paused.CompareAndSwap(false, true) {
		close(store.entered)
		<-store.release
	}
	return failed
}

func TestFailedWriterCloseDoesNotExpireSiblingDuringLookup(t *testing.T) {
	f := newNamespaceFixture(t)
	siblingState, siblingFH := openWriteFile(t, f.conn, &f.xid, f.clientID, "target")
	siblingID, _ := fhToInodeID(siblingFH)
	store := &pausedFailureCheckStore{StagingStore: f.store, id: f.id,
		entered: make(chan struct{}), release: make(chan struct{})}
	f.server.stagingStore = store
	readConn := dial(t, f.conn.RemoteAddr().String())
	defer readConn.Close()
	done := make(chan uint32, 1)
	go func() {
		res := sendCompound(t, readConn, 1000, func(w *COMPOUND4argsWriter) {
			pw := w.AppendArgarray_Putfh()
			pw.Resume(pw.StartObject().SetData(f.fh).Finish())
			w.Resume(pw.Finish())
			rw := w.AppendArgarray_Read()
			setStateid(rw.Stateid(), f.state)
			rw.SetCount(1)
		})
		done <- res.Status()
	}()
	awaitSignal(t, store.entered, "READ local stateid lookup")
	// The READ has copied valid local state and observed no tombstone.
	// Fail W1, then let CLOSE remove its marker before READ checks it.
	f.server.fs = &namespaceFaultVFS{TernVFS: f.fs, err: errors.New("lost reply")}
	if got := removeNamespaceFile(t, f, "source"); got != NFS4ERR_IO {
		t.Fatal(Nfsstat4Name(got))
	}
	if got := closeFileWithSeqStatus(t, f.conn, &f.xid, f.fh, f.state, 3); got != NFS4ERR_EXPIRED {
		t.Fatal(Nfsstat4Name(got))
	}
	close(store.release)
	if got := awaitValue(t, done, "READ after failed CLOSE"); got != NFS4ERR_IO {
		t.Fatalf("racing READ = %s", Nfsstat4Name(got))
	}
	meta, ok := f.store.GetMeta(siblingID)
	if !ok || meta.Retired {
		t.Fatal("failed writer expired its healthy sibling")
	}
	if active, err := f.server.clients.HasOpen(f.clientID, meta.NFSStateID); err != nil || !active {
		t.Fatalf("sibling marker was revoked: %t, %v", active, err)
	}
	writeFileAt(t, f.conn, &f.xid, siblingFH, siblingState, 0, []byte("healthy sibling"))
	closeFile(t, f.conn, &f.xid, siblingFH, siblingState)
}

func TestNamespaceGuardRestartMakesNoRecoveryCalls(t *testing.T) {
	for _, applied := range []bool{false, true} {
		t.Run(fmt.Sprintf("applied=%t", applied), func(t *testing.T) {
			f := newNamespaceFixture(t)
			if err := f.store.SetGuard(f.id, true); err != nil {
				t.Fatal(err)
			}
			if applied {
				if err := f.fs.Rename(f.fs.RootID(), "source", f.fs.RootID(), "target"); err != nil {
					t.Fatal(err)
				}
			}
			probe := &namespaceProbeVFS{TernVFS: f.fs}
			f.backend = probe
			f.restart(t)
			assertQuarantined(t, f, f.id)
			if probe.mutations.Load() != 0 || probe.lookups.Load() != 0 {
				t.Fatal("startup tried to resolve or replay the guarded mutation")
			}
		})
	}
}

func TestStagingStartupExcludesEveryBadEntry(t *testing.T) {
	for _, problem := range []string{"corrupt", "foreign magic", "quarantine failure", "unreadable sidecar", "truncate failure", "retired detached"} {
		t.Run(problem, func(t *testing.T) {
			if problem == "unreadable sidecar" && os.Geteuid() == 0 {
				t.Skip("root bypasses unreadable sidecar permissions")
			}
			dir := t.TempDir()
			store, id, stage := createOverlayStage(t, dir, 42, []byte("original"))
			meta, _ := store.GetMeta(id)
			path := stage.(*localStagingFile).metaPath
			if err := stage.Write(0, []byte("private!")); err != nil {
				t.Fatal(err)
			}
			if err := stage.Sync(); err != nil {
				t.Fatal(err)
			}
			if err := stage.(*localStagingFile).f.Close(); err != nil {
				t.Fatal(err)
			}
			meta, _ = loadStagingMeta(path)
			switch problem {
			case "corrupt", "quarantine failure":
				if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
					t.Fatal(err)
				}
				if problem == "quarantine failure" {
					if err := os.WriteFile(filepath.Join(dir, stagingQuarantineDirName), []byte("blocked"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "foreign magic":
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				data = bytes.Replace(data, []byte("NFS6"), []byte("NFS7"), 1)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "unreadable sidecar":
				if err := os.Chmod(path, 0000); err != nil {
					t.Fatal(err)
				}
			case "truncate failure":
				dataPath := stage.(*localStagingFile).f.Name()
				if err := os.Rename(dataPath, dataPath+".preserved"); err != nil {
					t.Fatal(err)
				}
				if err := syscall.Mkfifo(dataPath, 0600); err != nil {
					t.Fatal(err)
				}
			case "retired detached":
				meta.Unlinked, meta.Retired = true, true
				if err := saveStagingMeta(path, meta); err != nil {
					t.Fatal(err)
				}
			}
			healthy := id + 1
			healthyFile, err := store.Create(healthy, StagingMeta{DirID: meta.DirID, FileName: "healthy"})
			if err != nil {
				t.Fatal(err)
			}
			if err := healthyFile.(*localStagingFile).f.Close(); err != nil {
				t.Fatal(err)
			}
			recovered, err := NewLocalStagingStore(dir, nil)
			if err != nil {
				t.Fatalf("per-entry failure prevented startup: %v", err)
			}
			defer recovered.Remove(healthy)
			if recovered.Get(id) != nil || recovered.Get(healthy) == nil {
				t.Fatal("startup did not isolate just the bad entry")
			}
			srv, err := NewServer(NewLocalTernVFS(t.TempDir()), recovered, nil)
			if err != nil {
				t.Fatalf("excluded entry prevented server startup: %v", err)
			}
			srv.waitForClientGC()
			if problem == "quarantine failure" {
				data, err := os.ReadFile(stage.(*localStagingFile).f.Name())
				if err != nil || string(data) != "private!" {
					t.Fatalf("failed quarantine discarded data: %q, %v", data, err)
				}
			}
		})
	}
}

func TestNamespaceSidecarStrictDecode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint.meta")
	meta := StagingMeta{FileName: "file", Unlinked: true, Guarded: true}
	if err := saveStagingMeta(path, meta); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Fixed NFS6 layout, with no dirty ranges in this checkpoint.
	flagOffset := 30 + len(meta.FileName) + 8 + 4 + 8 + 8 + 8 + 4 + 32
	for _, kind := range []string{"NFS5", "NFS7", "trailer", "unknown flag", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			bad := append([]byte(nil), data...)
			switch kind {
			case "NFS5", "NFS7":
				magic := 30 + int(binary.BigEndian.Uint16(bad[28:30])) + 8
				copy(bad[magic:], kind)
			case "trailer":
				bad = append(bad, 0)
			case "unknown flag":
				bad[flagOffset] |= 128
			case "truncated":
				bad = bad[:len(bad)-1]
			}
			if err := os.WriteFile(path, bad, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadStagingMeta(path); err == nil {
				t.Fatal("incompatible checkpoint was accepted")
			}
		})
	}
}

func TestNamespacePreflightRejectionDoesNotGuard(t *testing.T) {
	f := newNamespaceFixture(t)
	if _, err := f.fs.Mkdir(f.fs.RootID(), "directory"); err != nil {
		t.Fatal(err)
	}
	probe := &namespaceProbeVFS{TernVFS: f.fs}
	f.server.fs = probe
	sf := f.store.Get(f.id).(*localStagingFile)
	path := sf.metaPath
	// A misplaced guard attempt would fail with IO instead of the preflight
	// EXIST response, even if the operation later tried to clear its guard.
	sf.metaPath = filepath.Join(f.disk, "missing", "checkpoint.meta")
	defer func() { sf.metaPath = path }()
	if got := renameNamespaceFile(t, f, "source", "directory"); got != NFS4ERR_EXIST {
		t.Fatalf("type rejection = %s", Nfsstat4Name(got))
	}
	if probe.mutations.Load() != 0 {
		t.Fatal("type rejection submitted a mutation")
	}
	sf.metaPath = path
	meta, _ := loadStagingMeta(f.store.Get(f.id).(*localStagingFile).metaPath)
	if meta.Guarded || meta.Unlinked {
		t.Fatalf("type rejection changed the sidecar: %+v", meta)
	}
	assertRejectedNamespaceWriter(t, f)
}

func TestUnknownNamespaceQuarantineFailureSurvivesRestart(t *testing.T) {
	f := newNamespaceFixture(t)
	blocker := filepath.Join(f.disk, stagingQuarantineDirName)
	if err := os.WriteFile(blocker, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	f.server.fs = &namespaceFaultVFS{TernVFS: f.fs, err: errors.New("lost reply"), apply: true}
	if got := removeNamespaceFile(t, f, "source"); got != NFS4ERR_IO || !f.store.Failed(f.id) {
		t.Fatalf("unknown outcome did not fail writer: %s", Nfsstat4Name(got))
	}
	f.restart(t) // still cannot move it, but must start and exclude it
	if f.store.Get(f.id) != nil {
		t.Fatal("failed quarantine allowed recovery")
	}
	if err := os.Rename(blocker, blocker+".blocker"); err != nil {
		t.Fatal(err)
	}
	f.restart(t)
	assertQuarantined(t, f, f.id)
}

func TestQuarantineSyncFailureLeavesSourceFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory read permissions")
	}
	f := newNamespaceFixture(t)
	if err := f.store.SetGuard(f.id, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.disk, 0300); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(f.disk, 0700)
	// The directory permits moves but not the fsync which makes the new
	// quarantine path durable. Neither source may move until that succeeds.
	if err := f.store.Quarantine(f.id); err == nil {
		t.Fatal("quarantine succeeded without syncing its parent")
	}
	if err := os.Chmod(f.disk, 0700); err != nil {
		t.Fatal(err)
	}
	dataPath := filepath.Join(f.disk, fmt.Sprintf("%016x.staging", uint64(f.id)))
	data, err := os.ReadFile(dataPath)
	if err != nil || string(data) != "private data" {
		t.Fatalf("source data moved before destination was durable: %q, %v", data, err)
	}
	meta, err := loadStagingMeta(filepath.Join(f.disk, fmt.Sprintf("%016x.meta", uint64(f.id))))
	if err != nil || !meta.Guarded || !f.store.Failed(f.id) {
		t.Fatalf("failed quarantine lost its guard or tombstone: %+v, %v", meta, err)
	}
	f.restart(t)
	assertQuarantined(t, f, f.id)
}

func TestNamespacePrivateIODoesNotWaitForGuard(t *testing.T) {
	f := newNamespaceFixture(t)
	fs := blockingNamespaceRemove{TernVFS: f.fs, entered: make(chan struct{}), release: make(chan struct{})}
	f.server.fs = fs
	removeF := *f
	removeF.conn = dial(t, f.conn.RemoteAddr().String())
	defer removeF.conn.Close()
	done := make(chan uint32, 1)
	go func() { done <- removeNamespaceFile(t, &removeF, "source") }()
	awaitSignal(t, fs.entered, "guarded REMOVE")
	f.conn.SetDeadline(time.Now().Add(time.Second))
	writeFileAt(t, f.conn, &f.xid, f.fh, f.state, 0, []byte("new private data"))
	if got := commitNamespaceStatus(t, f); got != NFS4_OK {
		t.Fatal(Nfsstat4Name(got))
	}
	data, _ := readFileData(t, f.conn, &f.xid, f.fh, 0, 128)
	if string(data) != "new private data" {
		t.Fatalf("guarded private read = %q", data)
	}
	close(fs.release)
	if got := awaitValue(t, done, "completed REMOVE"); got != NFS4_OK {
		t.Fatal(Nfsstat4Name(got))
	}
}

func TestFailedNamespaceWriterIOAndStateidValidation(t *testing.T) {
	for _, operation := range []string{"read", "write", "size", "time"} {
		for _, stateKind := range []string{"ordinary", "special", "invalid"} {
			t.Run(operation+"/"+stateKind, func(t *testing.T) {
				f := newNamespaceFixture(t)
				f.server.fs = &namespaceFaultVFS{TernVFS: f.fs, err: errors.New("lost reply")}
				if got := removeNamespaceFile(t, f, "source"); got != NFS4ERR_IO {
					t.Fatal(Nfsstat4Name(got))
				}
				state := f.state
				want := uint32(NFS4ERR_IO)
				switch stateKind {
				case "special":
					state = [16]byte{}
				case "invalid":
					state[15] ^= 128
					if operation != "time" {
						want = NFS4ERR_BAD_STATEID
					}
				}
				res := sendCompound(t, f.conn, f.xid, func(w *COMPOUND4argsWriter) {
					pw := w.AppendArgarray_Putfh()
					pw.Resume(pw.StartObject().SetData(f.fh).Finish())
					w.Resume(pw.Finish())
					switch operation {
					case "read":
						rw := w.AppendArgarray_Read()
						setStateid(rw.Stateid(), state)
						rw.SetCount(1)
					case "write":
						ww := w.AppendArgarray_Write()
						setStateid(ww.Stateid(), state)
						ww = ww.SetStable(fileSync4)
						w.Resume(ww.SetData([]byte("x")).Finish())
					default:
						sw := w.AppendArgarray_Setattr()
						setStateid(sw.Stateid(), state)
						aw := sw.StartObjAttributes()
						bw := aw.StartAttrmask()
						var data []byte
						if operation == "size" {
							bw.AppendData(1 << FATTR4_SIZE)
							data = binary.BigEndian.AppendUint64(nil, 1)
						} else {
							bw.AppendData(0)
							bw.AppendData(1 << (FATTR4_TIME_MODIFY_SET - 32))
							data = binary.BigEndian.AppendUint32(nil, SET_TO_SERVER_TIME4)
						}
						aw.Resume(bw.Finish())
						aw.Resume(aw.StartAttrVals().SetData(data).Finish())
						sw.Resume(aw.Finish())
						w.Resume(sw.Finish())
					}
				})
				if got := res.Status(); got != want {
					t.Fatalf("%s = %s, want %s", operation, Nfsstat4Name(got), Nfsstat4Name(want))
				}
			})
		}
	}
}
