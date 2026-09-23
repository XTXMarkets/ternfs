// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestNamespaceChangePreservesDataCheckpoint(t *testing.T) {
	store, id, stage := createOverlayStage(t, t.TempDir(), 42, []byte("original bytes"))
	defer store.Remove(id)
	sf := stage.(*localStagingFile)
	if err := sf.Write(0, []byte("saved")); err != nil {
		t.Fatal(err)
	}
	if err := sf.Sync(); err != nil {
		t.Fatal(err)
	}
	before, err := loadStagingMeta(sf.metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := sf.Write(8, []byte("new")); err != nil {
		t.Fatal(err)
	}
	// FindTargets exposes live ranges. Neither guard nor final namespace
	// checkpoints may treat that snapshot as a durable data checkpoint.
	_ = store.FindTargets(before.DirID, before.FileName)
	if err := store.SetGuard(id, true); err != nil {
		t.Fatal(err)
	}
	guarded, err := loadStagingMeta(sf.metaPath)
	if err != nil {
		t.Fatal(err)
	}
	before.Guarded = true
	if !reflect.DeepEqual(guarded, before) {
		t.Fatalf("guard checkpoint = %+v, want %+v", guarded, before)
	}
	// A COMMIT during the mutation must survive its final checkpoint.
	if err := sf.Sync(); err != nil {
		t.Fatal(err)
	}
	committed, err := loadStagingMeta(sf.metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Retarget(id, before.DirID, "renamed"); err != nil {
		t.Fatal(err)
	}
	final, err := loadStagingMeta(sf.metaPath)
	if err != nil {
		t.Fatal(err)
	}
	committed.Guarded = false
	committed.FileName = "renamed"
	if !reflect.DeepEqual(final, committed) {
		t.Fatalf("final checkpoint = %+v, want %+v", final, committed)
	}
	if len(store.FindTargets(before.DirID, before.FileName)) != 0 ||
		len(store.FindTargets(before.DirID, "renamed")) != 1 {
		t.Fatal("retarget did not move the writer's index")
	}
}

func TestNamespaceFinalCheckpointFailureRepairsOnSync(t *testing.T) {
	store, id, stage := createOverlayStage(t, t.TempDir(), 42, []byte("original"))
	defer store.Remove(id)
	sf := stage.(*localStagingFile)
	if err := store.SetGuard(id, true); err != nil {
		t.Fatal(err)
	}
	path := sf.metaPath
	// Fail the final metadata save without changing the already durable guard.
	sf.metaPath = filepath.Join(path, "not-a-directory")
	if err := store.Retarget(id, 1, "new-name"); err == nil {
		t.Fatal("final checkpoint unexpectedly succeeded")
	}
	sf.metaPath = path
	meta, _ := store.GetMeta(id)
	if meta.Guarded || meta.FileName != "new-name" || store.Failed(id) {
		t.Fatalf("known outcome was not kept usable: %+v", meta)
	}
	if len(store.FindTargets(1, "new-name")) != 1 {
		t.Fatal("failed save did not reindex the final target")
	}
	disk, err := loadStagingMeta(path)
	if err != nil || !disk.Guarded {
		t.Fatalf("old guard missing: %+v, %v", disk, err)
	}
	// No new data or attributes: checkpointDirty must force the repair.
	if err := sf.Sync(); err != nil {
		t.Fatal(err)
	}
	disk, err = loadStagingMeta(path)
	if err != nil || disk.Guarded || disk.FileName != "new-name" {
		t.Fatalf("Sync did not repair namespace checkpoint: %+v, %v", disk, err)
	}
}

func TestNamespaceQuarantineKeepsTombstone(t *testing.T) {
	store, id, stage := createOverlayStage(t, t.TempDir(), 42, []byte("original"))
	meta, _ := store.GetMeta(id)
	if err := store.SetGuard(id, true); err != nil {
		t.Fatal(err)
	}
	if err := store.Quarantine(id); err != nil {
		t.Fatal(err)
	}
	store.Remove(id)
	if !store.Failed(id) || store.Get(id) != nil ||
		len(store.FindTargets(meta.DirID, meta.FileName)) != 0 {
		t.Fatal("cleanup lost the tombstone or exposed the failed writer")
	}
	if _, ok := store.GetMeta(id); ok {
		t.Fatal("failed metadata is still recoverable")
	}
	if _, err := store.Create(id, meta); !errors.Is(err, errStagingRemoved) {
		t.Fatalf("Create revived failed inode: %v", err)
	}
	if err := stage.Write(0, []byte("late")); !errors.Is(err, errStagingRemoved) {
		t.Fatalf("previously obtained reference still writes: %v", err)
	}
	if err := stage.Sync(); !errors.Is(err, errStagingRemoved) {
		t.Fatalf("previously obtained reference still commits: %v", err)
	}
}

func TestNamespaceGuardRecoveryPreservesUncheckpointedTail(t *testing.T) {
	dir := t.TempDir()
	store, id, stage := createOverlayStage(t, dir, 42, []byte("base"))
	sf := stage.(*localStagingFile)
	if err := sf.Write(4, []byte("tail")); err != nil {
		t.Fatal(err)
	}
	if err := store.SetGuard(id, true); err != nil {
		t.Fatal(err)
	}
	if err := sf.f.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Get(id) != nil || len(recovered.Entries()) != 0 {
		t.Fatal("guarded staging registered on restart")
	}
	files, err := filepath.Glob(filepath.Join(dir, "quarantine", "*", "*.staging"))
	if err != nil || len(files) != 1 {
		t.Fatalf("quarantined data = %v, %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil || len(data) != 8 || string(data[4:]) != "tail" {
		t.Fatalf("recovery truncated the quarantined data: %q, %v", data, err)
	}
}

func TestNamespaceMutationClassification(t *testing.T) {
	remote := &RemoteTernVFS{}
	local := &LocalTernVFS{}
	for _, op := range []uint32{OP_REMOVE, OP_RENAME} {
		for _, err := range []error{msgs.TIMEOUT, msgs.EDGE_IS_LOCKED, msgs.NAME_IS_LOCKED,
			msgs.MALFORMED_RESPONSE, msgs.CANNOT_OVERRIDE_NAME, msgs.NOT_AUTHORISED,
			msgs.DIRECTORY_NOT_EMPTY, errors.New("lost reply"), syscall.EEXIST} {
			if got := remote.ClassifyMutation(op, err); got != mutationUnknown {
				t.Fatalf("remote classification of %v = %v", err, got)
			}
		}
		for _, err := range []error{msgs.EDGE_NOT_FOUND, msgs.MISMATCHING_CREATION_TIME} {
			want := mutationUnknown
			if op == OP_REMOVE {
				want = mutationDetachOnly
			}
			if got := remote.ClassifyMutation(op, err); got != want {
				t.Fatalf("remote edge-gone classification = %v, want %v", got, want)
			}
		}
		if remote.ClassifyMutation(op, nil) != mutationApplied ||
			local.ClassifyMutation(op, nil) != mutationApplied {
			t.Fatal("nil mutation was not applied")
		}
		if local.ClassifyMutation(op, syscall.EISDIR) != mutationNotApplied ||
			local.ClassifyMutation(op, errors.New("lost reply")) != mutationUnknown {
			t.Fatal("local classification does not distinguish syscall rejection")
		}
	}
}
