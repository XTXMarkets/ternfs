// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func TestSealPreservesFinalDataUntilRemoval(t *testing.T) {
	store, id, sf := CreateOverlayStage(t, t.TempDir(), vfs.MakeInodeID(vfs.InodeTypeFile, 1), []byte("base"))
	defer store.Remove(id)
	version := sf.DataVersion()
	if err := sf.Write(0, []byte("next")); err != nil {
		t.Fatal(err)
	}
	if sf.Seal(version) {
		t.Fatal("sealed a different data version")
	}
	if err := sf.SetSize(3); err != nil {
		t.Fatalf("failed seal stopped a later size change: %v", err)
	}
	if !sf.Seal(sf.DataVersion()) {
		t.Fatal("could not seal the current version")
	}
	if err := sf.Write(0, []byte("lost")); !errors.Is(err, ErrRemoved) {
		t.Fatalf("WRITE after seal: %v", err)
	}
	if err := sf.SetSize(0); !errors.Is(err, ErrRemoved) {
		t.Fatalf("size change after seal: %v", err)
	}
	now := time.Now()
	if err := sf.SetTime(&now, nil); !errors.Is(err, ErrRemoved) {
		t.Fatalf("time change after seal: %v", err)
	}
	if err := sf.Sync(); err != nil {
		t.Fatalf("sync before removal: %v", err)
	}
	data := make([]byte, 3)
	if n, _, err := sf.Read(0, data, nil); err != nil || n != 3 || string(data) != "nex" {
		t.Fatalf("read before removal: %q, %d, %v", data, n, err)
	}
}

func TestStaleLeaseCheckCannotRetireRecoveredWriter(t *testing.T) {
	store, id, sf := CreateOverlayStage(t, t.TempDir(), vfs.MakeInodeID(vfs.InodeTypeFile, 1), []byte("base"))
	defer store.Remove(id)
	old, _ := store.GetMeta(id)
	if err := store.Rebind(id, 42, nfs4.StateID{3}); err != nil {
		t.Fatal(err)
	}
	if err := sf.Retire(old.ClientID, old.NFSStateID); err != nil {
		t.Fatal(err)
	}
	meta, _ := store.GetMeta(id)
	if meta.Retired || !TargetBusy(store, meta.DirID, meta.FileName) {
		t.Fatal("stale lease check retired the recovered writer")
	}
}

func TestRetiredStagingReleasesDescriptorAndReopensOnRecovery(t *testing.T) {
	dir := t.TempDir()
	store, id, sf := CreateOverlayStage(t, dir, vfs.MakeInodeID(vfs.InodeTypeFile, 1), []byte("base"))
	if err := sf.Write(0, []byte("kept")); err != nil {
		t.Fatal(err)
	}
	meta, _ := store.GetMeta(id)
	if err := sf.Retire(meta.ClientID, meta.NFSStateID); err != nil {
		t.Fatal(err)
	}
	if _, err := sf.(*LocalFile).DataFile().Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("retired file still has an open descriptor: %v", err)
	}
	recovered, err := NewLocalStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Remove(id)
	recoveredFile := recovered.Get(id).(*LocalFile)
	if _, err := recoveredFile.DataFile().Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("restart reopened a retired descriptor: %v", err)
	}
	if err := recovered.Rebind(id, 42, nfs4.StateID{3}); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 4)
	if _, _, err := recoveredFile.Read(0, data, nil); err != nil || string(data) != "kept" {
		t.Fatalf("recovered data = %q, err = %v", data, err)
	}
}
