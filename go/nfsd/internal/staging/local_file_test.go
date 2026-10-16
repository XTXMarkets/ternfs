// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"errors"
	"os"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

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
