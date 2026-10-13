// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func TestOpenExclusive4(t *testing.T) {
	root := t.TempDir()
	srv, addr, cleanup := startTestServerWithServer(t, root)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	verifier := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	status, state, fh, flags := openExclusiveFile(t, conn, &xid, clientID, "owner", "file", verifier)
	if status != nfs4.NFS4_OK || flags&nfs4.OPEN4_RESULT_CONFIRM != 0 {
		t.Fatalf("create: %s, flags=%x", nfs4.Nfsstat4Name(status), flags)
	}
	retransmitNext(conn)
	status, replayState, replayFH, _ := openExclusiveFile(t, conn, &xid, clientID, "owner", "file", verifier)
	if status != nfs4.NFS4_OK || replayState != state || !bytes.Equal(replayFH, fh) {
		t.Fatalf("OPEN replay changed its result: %s", nfs4.Nfsstat4Name(status))
	}
	publishedFH := lookupFH(t, conn, &xid, "file")
	if !bytes.Equal(publishedFH, fh) {
		t.Fatal("exclusive writer does not hold the published empty filehandle")
	}
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("private"))
	if data, err := os.ReadFile(filepath.Join(root, "file")); err != nil || len(data) != 0 {
		t.Fatalf("published private bytes before CLOSE: %q, %v", data, err)
	}
	status, _, _, _ = openExclusiveFile(t, conn, &xid, clientID, "owner", "file", [8]byte{9})
	if status != nfs4.NFS4ERR_EXIST {
		t.Fatalf("changed verifier: %s", nfs4.Nfsstat4Name(status))
	}
	status, dup, dupFH, _ := openExclusiveFile(t, conn, &xid, clientID, "owner", "file", verifier)
	if status != nfs4.NFS4_OK || !bytes.Equal(dup[4:], state[4:]) || !bytes.Equal(dupFH, fh) ||
		binary.BigEndian.Uint32(dup[:4]) <= binary.BigEndian.Uint32(state[:4]) {
		t.Fatalf("duplicate create: %s, state=%x, fh=%x", nfs4.Nfsstat4Name(status), dup, dupFH)
	}
	state = dup
	if len(srv.stagingStore.Entries()) != 1 {
		t.Fatal("duplicate create allocated another writer")
	}
	status, _, _, _ = openExclusiveFile(t, conn, &xid, clientID, "other-owner", "file", verifier)
	if status != nfs4.NFS4ERR_EXIST {
		t.Fatalf("another owner reused verifier: %s", nfs4.Nfsstat4Name(status))
	}
	if data, _ := readFileData(t, conn, &xid, fh, 0, 1024); string(data) != "private" {
		t.Fatalf("duplicate create lost private data: %q", data)
	}
	status, _ = closeFileResult(t, conn, &xid, fh, state)
	if status != nfs4.NFS4_OK {
		t.Fatalf("CLOSE: %s", nfs4.Nfsstat4Name(status))
	}
	if data, err := os.ReadFile(filepath.Join(root, "file")); err != nil || string(data) != "private" {
		t.Fatalf("published data: %q, %v", data, err)
	}
	status, _, _, _ = openExclusiveFile(t, conn, &xid, clientID, "owner", "file", verifier)
	if status != nfs4.NFS4ERR_EXIST {
		t.Fatalf("create after publication: %s", nfs4.Nfsstat4Name(status))
	}
	_, _ = openCreateFile(t, conn, &xid, clientID, "ordinary")
	status, _, _, _ = openExclusiveFile(t, conn, &xid, clientID, "owner", "ordinary", verifier)
	if status != nfs4.NFS4ERR_EXIST {
		t.Fatalf("create over ordinary writer: %s", nfs4.Nfsstat4Name(status))
	}
}

func TestExclusiveVerifierDoesNotFollowReplacement(t *testing.T) {
	addr, cleanup := startTestServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	verifier := [8]byte{1}
	status, state, fh, _ := openExclusiveFile(t, conn, &xid, clientID, "exclusive", "file", verifier)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	// Writers on one client share staging, so the replacement comes from
	// another client.
	otherConn := dial(t, addr)
	defer otherConn.Close()
	otherID, confirm := requestClientID(t, otherConn, &xid, "replacement-client", [8]byte{2})
	if status := confirmClientID(t, otherConn, &xid, otherID, confirm); status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	status, other, otherFH, _ := openFileForOwner(t, otherConn, &xid, otherID, "replacement", "file",
		nfs4.OPEN4_SHARE_ACCESS_BOTH, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	writeFileAt(t, otherConn, &xid, otherFH, other, 0, []byte("replacement"))
	closeFile(t, otherConn, &xid, otherFH, other)
	status, _, _, _ = openExclusiveFile(t, conn, &xid, clientID, "exclusive", "file", verifier)
	if status != nfs4.NFS4ERR_EXIST {
		t.Fatalf("verifier matched a replacement: %s", nfs4.Nfsstat4Name(status))
	}
	if data, _ := readFileData(t, conn, &xid, lookupFH(t, conn, &xid, "file"), 0, 1024); string(data) != "replacement" {
		t.Fatalf("retry altered the replacement: %q", data)
	}
	if status, _ := closeFileResult(t, conn, &xid, fh, state); status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
}

func TestExclusiveCreateRecovery(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
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
	conn := dial(t, addr)
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	verifier := [8]byte{8, 7, 6}
	status, state, fh, _ := openExclusiveFile(t, conn, &xid, clientID, "owner", "file", verifier)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	writeFileAt(t, conn, &xid, fh, state, 0, []byte("checkpoint"))
	conn.Close()
	cleanup()
	closeLocalStagingFiles(t, store)

	recovered, err := NewLocalStagingStore(stagingDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeLocalStagingFiles(t, recovered)
	srv, err = NewServer(fs, recovered, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, cleanup = serveTestServer(t, srv)
	defer cleanup()
	conn = dial(t, addr)
	defer conn.Close()
	status, _, _, _ = openExclusiveFile(t, conn, &xid, clientID, "owner", "file", [8]byte{9})
	if status != nfs4.NFS4ERR_EXIST {
		t.Fatalf("changed verifier after restart: %s", nfs4.Nfsstat4Name(status))
	}
	status, state, restoredFH, flags := openExclusiveFile(t, conn, &xid, clientID, "owner", "file", verifier)
	if status != nfs4.NFS4_OK || !bytes.Equal(restoredFH, fh) {
		t.Fatalf("recovery: %s, fh=%x, want %x", nfs4.Nfsstat4Name(status), restoredFH, fh)
	}
	_ = flags
	if data, _ := readFileData(t, conn, &xid, fh, 0, 1024); string(data) != "checkpoint" {
		t.Fatalf("recovered data = %q", data)
	}
	if status, _ := closeFileResult(t, conn, &xid, fh, state); status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
}

func TestExclusiveStagingCheckpoint(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := vfs.MakeInodeID(vfs.InodeTypeFile, 17)
	meta := StagingMeta{
		DirID: vfs.MakeInodeID(vfs.InodeTypeDir, 1), FileName: "file",
		BaseID: vfs.MakeInodeID(vfs.InodeTypeFile, 18), BaseSize: 10,
		ClientID: 1, NFSStateID: StateID{1}, OpenOwner: "", OwnerKnown: true,
		Exclusive: true, Verifier: [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
	}
	stage, err := store.Create(id, meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.Write(2, []byte("changed")); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1_700_000_000, 123)
	if err := stage.SetTime(&stamp, &stamp); err != nil {
		t.Fatal(err)
	}
	if err := stage.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := store.Rebind(id, 2, StateID{2}); err != nil {
		t.Fatal(err)
	}
	closeLocalStagingFiles(t, store)
	loaded, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeLocalStagingFiles(t, loaded)
	got, ok := loaded.GetMeta(id)
	if !ok || got.version != stagingMetaVersion || !got.Exclusive || got.Verifier != meta.Verifier ||
		got.BaseID != meta.BaseID || got.ClientID != 2 || got.NFSStateID != (StateID{2}) ||
		!got.OwnerKnown || got.OpenOwner != "" || !got.Attrs.Mtime.Equal(stamp) ||
		got.Size != 10 || len(got.Dirty) != 1 || got.Dirty[0] != (byteRange{2, 9}) {
		t.Fatalf("recovered checkpoint: %+v", got)
	}
}

func TestExclusiveSidecarRoundTrip(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file.meta")
	meta := StagingMeta{FileName: "file", OwnerKnown: true, OpenOwner: "owner"}
	if err := saveStagingMeta(file, meta); err != nil {
		t.Fatal(err)
	}
	plain, err := loadStagingMeta(file)
	if err != nil || plain.version != stagingMetaVersion || plain.Exclusive ||
		plain.Verifier != ([8]byte{}) {
		t.Fatalf("ordinary sidecar: %+v, %v", plain, err)
	}
	meta.Exclusive = true
	meta.Verifier = [8]byte{1, 2, 3}
	meta.RecoveryKey = [32]byte{4, 5, 6}
	if err := saveStagingMeta(file, meta); err != nil {
		t.Fatal(err)
	}
	exclusive, err := loadStagingMeta(file)
	if err != nil || !exclusive.Exclusive ||
		exclusive.Verifier != meta.Verifier ||
		exclusive.RecoveryKey != meta.RecoveryKey ||
		exclusive.OpenOwner != meta.OpenOwner {
		t.Fatalf("exclusive sidecar: %+v, %v", exclusive, err)
	}
	encoded, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, removed := range []int{1, 8} {
		if err := os.WriteFile(file, encoded[:len(encoded)-removed], 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadStagingMeta(file); err == nil {
			t.Fatalf("accepted sidecar missing %d verifier bytes", removed)
		}
	}
}

type exclusiveTimeVFS struct {
	vfs.TernVFS
	fail bool
}

func (fs *exclusiveTimeVFS) SetTime(id vfs.InodeID, mtime, atime *time.Time) error {
	if fs.fail {
		return os.ErrPermission
	}
	return fs.TernVFS.SetTime(id, mtime, atime)
}

func TestExclusiveSetattr(t *testing.T) {
	for _, failTimes := range []bool{false, true} {
		t.Run(map[bool]string{false: "times", true: "published-before-time-error"}[failTimes], func(t *testing.T) {
			root := t.TempDir()
			fs := &exclusiveTimeVFS{TernVFS: vfs.NewLocalTernVFS(root), fail: failTimes}
			store, err := NewLocalStagingStore(t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer closeLocalStagingFiles(t, store)
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
			status, state, fh, _ := openExclusiveFile(t, conn, &xid, clientID, "owner", "file", [8]byte{1})
			if status != nfs4.NFS4_OK {
				t.Fatal(nfs4.Nfsstat4Name(status))
			}
			writeFileAt(t, conn, &xid, fh, state, 0, []byte("payload"))
			atime, mtime := time.Unix(1_600_000_000, 123), time.Unix(1_700_000_000, 456)
			mask := attrMask{1 << nfs4.FATTR4_SIZE,
				1<<(nfs4.FATTR4_MODE-32) | 1<<(nfs4.FATTR4_TIME_ACCESS_SET-32) | 1<<(nfs4.FATTR4_TIME_MODIFY_SET-32)}
			data := binary.BigEndian.AppendUint64(nil, 4)
			data = binary.BigEndian.AppendUint32(data, 0644)
			for _, stamp := range []time.Time{atime, mtime} {
				data = binary.BigEndian.AppendUint32(data, nfs4.SET_TO_CLIENT_TIME4)
				data = binary.BigEndian.AppendUint64(data, uint64(stamp.Unix()))
				data = binary.BigEndian.AppendUint32(data, uint32(stamp.Nanosecond()))
			}
			res := sendCompound(t, conn, xid, func(w *nfs4.COMPOUND4argsWriter) {
				pw := w.AppendArgarray_Putfh()
				pw.Resume(pw.StartObject().SetData(fh).Finish())
				w.Resume(pw.Finish())
				sw := w.AppendArgarray_Setattr()
				setStateid(sw.Stateid(), state)
				sw.Resume(finishTestFattr(sw.StartObjAttributes(), mask, data))
				w.Resume(sw.Finish())
			})
			xid++
			iter := expectOK(t, res)
			nextOp(t, &iter)
			if got := parseBitmap(nextOp(t, &iter).Value().AsSETATTR4res().Attrsset()); got != mask {
				t.Fatalf("attrsset=%v, want %v", got, mask)
			}
			attrs := mutableAttrs(t, conn, &xid, fh)
			getTime := func(off int) time.Time {
				return time.Unix(int64(binary.BigEndian.Uint64(attrs[off:off+8])),
					int64(binary.BigEndian.Uint32(attrs[off+8:off+12])))
			}
			if binary.BigEndian.Uint64(attrs[8:16]) != 4 ||
				!getTime(16).Equal(atime) || !getTime(40).Equal(mtime) {
				t.Fatalf("staged attrs=%x", attrs)
			}
			// This succeeds even when the backend rejects the post-publication times.
			if status, _ := closeFileResult(t, conn, &xid, fh, state); status != nfs4.NFS4_OK {
				t.Fatalf("CLOSE: %s", nfs4.Nfsstat4Name(status))
			}
			info, err := os.Stat(filepath.Join(root, "file"))
			if err != nil {
				t.Fatal(err)
			}
			if !failTimes {
				st := info.Sys().(*syscall.Stat_t)
				if !info.ModTime().Equal(mtime) || !time.Unix(st.Atim.Sec, st.Atim.Nsec).Equal(atime) {
					t.Fatalf("published timestamps: %+v", info)
				}
			}
			if data, err := os.ReadFile(filepath.Join(root, "file")); err != nil || string(data) != "payl" {
				t.Fatalf("published bytes=%q, %v", data, err)
			}
		})
	}
}
