// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func assertRetiredStaging(t *testing.T, store StagingStore, id InodeID) {
	t.Helper()
	meta, ok := store.GetMeta(id)
	if !ok || !meta.Retired || store.Get(id) == nil {
		t.Fatal("expired open did not retain its recoverable staging")
	}
	if store.TargetBusy(meta.DirID, meta.FileName) {
		t.Fatal("retired staging still reserves its pathname")
	}
}

func TestExpiredWriterRecoversAcknowledgedData(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, restart := range []bool{false, true} {
			t.Run(map[bool]string{false: "new", true: "existing"}[existing]+
				map[bool]string{false: "/same-server", true: "/restart"}[restart], func(t *testing.T) {
				fs := NewLocalTernVFS(t.TempDir())
				if existing {
					if _, err := fs.CreateFile(fs.RootID(), "file", bytes.NewBufferString("original")); err != nil {
						t.Fatal(err)
					}
				}
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
				defer func() { cleanup() }()
				conn := dial(t, addr)
				defer func() { conn.Close() }()
				xid := uint32(1)
				clientID := setupClient(t, conn, &xid)
				status, state, fh, _ := openFileForOwner(t, conn, &xid, clientID,
					"recovering-writer", 1, "file", OPEN4_SHARE_ACCESS_BOTH, !existing, false)
				if status != NFS4_OK {
					t.Fatal(Nfsstat4Name(status))
				}
				state = confirmOpenState(t, conn, &xid, fh, 2, state)
				writeFileAt(t, conn, &xid, fh, state, 0, []byte("UPDATED!"))
				id, _ := fhToInodeID(fh)
				srv.waitForClientGC()
				now := time.Now().Add(2 * nfsLeaseTime)
				srv.clients.now = func() time.Time { return now }
				srv.runLeaseSweep()
				assertRetiredStaging(t, store, id)
				if restart {
					conn.Close()
					cleanup()
					closeLocalStagingFiles(t, store)
					store, err = NewLocalStagingStore(stagingDir, nil)
					if err != nil {
						t.Fatal(err)
					}
					srv, err = NewServer(fs, store, nil)
					if err != nil {
						t.Fatal(err)
					}
					srv.clients.now = func() time.Time { return now }
					addr, cleanup = serveTestServer(t, srv)
					conn = dial(t, addr)
				}
				newClient := setupClient(t, conn, &xid)
				if newClient == clientID {
					t.Fatal("expired lease reused its old clientid")
				}
				srv.waitForClientGC()
				status, recovered, recoveredFH, _ := openFileForOwner(t, conn, &xid, newClient,
					"recovering-writer", 1, "file", OPEN4_SHARE_ACCESS_BOTH, false, false)
				if status != NFS4_OK {
					t.Fatal(Nfsstat4Name(status))
				}
				if !bytes.Equal(fh, recoveredFH) {
					t.Fatal("recovery changed the writer's filehandle")
				}
				recovered = confirmOpenState(t, conn, &xid, recoveredFH, 2, recovered)
				got, _ := readFileData(t, conn, &xid, recoveredFH, 0, 32)
				if string(got) != "UPDATED!" {
					t.Fatalf("recovery lost acknowledged data: %q", got)
				}
				closeFile(t, conn, &xid, recoveredFH, recovered)
				got, _ = readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "file"), 0, 32)
				if string(got) != "UPDATED!" {
					t.Fatalf("published recovered data = %q", got)
				}
			})
		}
	}
}

func TestStagingRecoveryKeySeparatesBootsAndPrincipals(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	cs, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	owner := clientOwner{principal: rpcPrincipal{flavor: authSys, body: "one"}}
	key := func(identity string, verifier byte, principal string) [32]byte {
		owner.principal.body = principal
		id, _, err := cs.SetClientID([8]byte{verifier}, []byte(identity), owner)
		if err != nil {
			t.Fatal(err)
		}
		k, err := cs.stagingRecoveryKey(id)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	original := key("client", 1, "one")
	if original != key("client", 1, "one") {
		t.Fatal("incarnation changed recovery identity")
	}
	for _, other := range [][32]byte{
		key("client", 2, "one"), key("client", 1, "two"), key("other", 1, "one"),
	} {
		if original == other {
			t.Fatal("recovery identity crossed a client, boot or principal boundary")
		}
	}
}

func TestRemoveCancelsQueuedHydration(t *testing.T) {
	store, id, sf := createOverlayStage(t, t.TempDir(), MakeInodeID(InodeTypeFile, 1), []byte("base"))
	srv := &Server{hydrationSlots: make(chan struct{}, 1)}
	srv.hydrationSlots <- struct{}{}
	entered := make(chan struct{})
	sf.StartHydration(func(cancel <-chan struct{}, id InodeID, offset uint64, dest []byte) (int, bool, error) {
		close(entered)
		return srv.readBaseBackground(cancel, id, offset, dest)
	})
	awaitSignal(t, entered, "queued hydration")
	done := make(chan struct{})
	go func() { store.Remove(id); close(done) }()
	awaitSignal(t, done, "cancelled queued hydration")
}

func TestInvalidStagingIsQuarantined(t *testing.T) {
	for _, format := range []string{"missing", "truncated", "NFS2", "NFS3"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			store, id, sf := createOverlayStage(t, dir, MakeInodeID(InodeTypeFile, 1), []byte("base"))
			if err := sf.Write(0, []byte("keep")); err != nil {
				t.Fatal(err)
			}
			if err := sf.Sync(); err != nil {
				t.Fatal(err)
			}
			metaPath := sf.(*localStagingFile).metaPath
			data, err := os.ReadFile(metaPath)
			if err != nil {
				t.Fatal(err)
			}
			closeLocalStagingFiles(t, store)
			switch format {
			case "missing":
				err = os.Remove(metaPath)
			case "truncated":
				err = os.WriteFile(metaPath, data[:8], 0600)
			default:
				off := 30 + int(binary.BigEndian.Uint16(data[28:30])) + 8
				copy(data[off:], format)
				err = os.WriteFile(metaPath, data, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			recovered, err := NewLocalStagingStore(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			if recovered.Get(id) != nil || len(recovered.Entries()) != 0 {
				t.Fatal("invalid staging was registered")
			}
			files, err := filepath.Glob(filepath.Join(dir, "quarantine", "*", "*.staging"))
			if err != nil || len(files) != 1 {
				t.Fatalf("quarantine files = %v, err = %v", files, err)
			}
			got, err := os.ReadFile(files[0])
			if err != nil || string(got) != "keep" {
				t.Fatalf("quarantined data = %q, err = %v", got, err)
			}
		})
	}
}

type closeTimeErrorVFS struct {
	*LocalTernVFS
}

func (fs *closeTimeErrorVFS) SetTime(InodeID, *time.Time, *time.Time) error {
	return errors.New("injected timestamp failure")
}

func TestPublishedCloseSurvivesTimestampFailure(t *testing.T) {
	fs := &closeTimeErrorVFS{NewLocalTernVFS(t.TempDir())}
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
	state, fh := openCreateFile(t, conn, &xid, clientID, "file")
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("published"))
	for range 2 {
		if status := closeFileWithSeqStatus(t, conn, &xid, fh, state, 3); status != NFS4_OK {
			t.Fatalf("CLOSE/replay = %s", Nfsstat4Name(status))
		}
	}
	if len(store.Entries()) != 0 || activeOpenMarkerCount(t, srv, clientID) != 0 {
		t.Fatal("committed CLOSE retained state")
	}
	got, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "file"), 0, 32)
	if string(got) != "published" {
		t.Fatalf("published data = %q", got)
	}
}

type metadataOnlyVFS struct {
	*LocalTernVFS
	reads atomic.Int32
	links atomic.Int32
}

func (fs *metadataOnlyVFS) Read(InodeID, uint64, []byte) (int, bool, error) {
	fs.reads.Add(1)
	return 0, false, errors.New("metadata-only CLOSE must not read data")
}

func (fs *metadataOnlyVFS) LinkFile(InodeID, Cookie, InodeID, string, io.Reader) error {
	fs.links.Add(1)
	return errors.New("metadata-only CLOSE must not publish data")
}

func TestMetadataOnlyCloseSkipsDataIO(t *testing.T) {
	for _, concurrentPublish := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-version", true: "newer-version"}[concurrentPublish], func(t *testing.T) {
			base := NewLocalTernVFS(t.TempDir())
			published, err := base.CreateFile(base.RootID(), "file", bytes.NewBufferString("original"))
			if err != nil {
				t.Fatal(err)
			}
			fs := &metadataOnlyVFS{LocalTernVFS: base}
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
			state, fh := openWriteFile(t, conn, &xid, clientID, "file")
			id, _ := fhToInodeID(fh)
			stamp := time.Unix(1_600_000_000, 0)
			if err := store.Get(id).SetTime(&stamp, nil); err != nil {
				t.Fatal(err)
			}
			if err := store.Get(id).Sync(); err != nil {
				t.Fatal(err)
			}
			want := "original"
			if concurrentPublish {
				want = "newer"
				published, err = base.CreateFile(base.RootID(), "file", bytes.NewBufferString(want))
				if err != nil {
					t.Fatal(err)
				}
			}
			closeFile(t, conn, &xid, fh, state)
			current, err := base.Lookup(base.RootID(), "file")
			if err != nil || current != published {
				t.Fatalf("CLOSE changed published inode: %v, err=%v", current, err)
			}
			info, err := base.Stat(current)
			if err != nil || !info.Mtime.Equal(stamp) {
				t.Fatalf("published attributes = %+v, err=%v", info, err)
			}
			got, err := base.ReadAll(current)
			if err != nil || string(got) != want {
				t.Fatalf("published data = %q, err=%v", got, err)
			}
			if fs.reads.Load() != 0 || fs.links.Load() != 0 {
				t.Fatal("timestamp-only CLOSE performed data IO")
			}
		})
	}
}

func createSizeStatus(t *testing.T, conn net.Conn, xid *uint32, clientID, size uint64) uint32 {
	t.Helper()
	res := sendCompound(t, conn, *xid, func(w *COMPOUND4argsWriter) {
		w.AppendArgarray_Putrootfh()
		ow := w.AppendArgarray_Open()
		ow.SetSeqid(1)
		ow.SetShareAccess(OPEN4_SHARE_ACCESS_BOTH)
		ow.SetShareDeny(OPEN4_SHARE_DENY_NONE)
		owner := ow.StartOwner().SetClientid(clientID).SetOwner([]byte("size-error"))
		ow.Resume(owner.Finish())
		create := ow.SetOpenhow_Create()
		attrs := create.SetValue_Unchecked4()
		data := binary.BigEndian.AppendUint64(nil, size)
		create.Resume(finishTestFattr(attrs, [2]uint32{1 << FATTR4_SIZE}, data))
		ow.Resume(create.Finish())
		claim := ow.SetClaim_Null()
		ow.Resume(claim.SetData([]byte("file")).Finish())
		w.Resume(ow.Finish())
	})
	*xid++
	return res.Status()
}

type sizeErrorStore struct{ StagingStore }
type sizeErrorFile struct{ StagingFile }

func (s sizeErrorStore) Get(id InodeID) StagingFile {
	sf := s.StagingStore.Get(id)
	if sf == nil {
		return nil
	}
	return sizeErrorFile{sf}
}
func (sizeErrorFile) SetSize(uint64) error { return errors.New("injected size failure") }

func TestOpenSizeErrorReplay(t *testing.T) {
	for _, ioFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "FBIG", true: "IO"}[ioFailure], func(t *testing.T) {
			store, err := NewLocalStagingStore(t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			var ss StagingStore = store
			size, want := uint64(1<<63), uint32(NFS4ERR_FBIG)
			if ioFailure {
				ss, size, want = sizeErrorStore{store}, 1, NFS4ERR_IO
			}
			srv, err := NewServer(NewLocalTernVFS(t.TempDir()), ss, nil)
			if err != nil {
				t.Fatal(err)
			}
			addr, cleanup := serveTestServer(t, srv)
			defer cleanup()
			conn := dial(t, addr)
			defer conn.Close()
			xid := uint32(1)
			clientID := setupClient(t, conn, &xid)
			for range 2 {
				if got := createSizeStatus(t, conn, &xid, clientID, size); got != want {
					t.Fatalf("OPEN/replay = %s, want %s", Nfsstat4Name(got), Nfsstat4Name(want))
				}
			}
		})
	}
}

func TestStaleLeaseCheckCannotRetireRecoveredWriter(t *testing.T) {
	store, id, sf := createOverlayStage(t, t.TempDir(), MakeInodeID(InodeTypeFile, 1), []byte("base"))
	defer store.Remove(id)
	old, _ := store.GetMeta(id)
	if err := store.Rebind(id, 42, StateID{3}); err != nil {
		t.Fatal(err)
	}
	if err := sf.Retire(old.ClientID, old.NFSStateID); err != nil {
		t.Fatal(err)
	}
	meta, _ := store.GetMeta(id)
	if meta.Retired || !store.TargetBusy(meta.DirID, meta.FileName) {
		t.Fatal("stale lease check retired the recovered writer")
	}
}
