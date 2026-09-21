// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestStagingRecoveryRemovesTemporaryMetadata(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, ".4000000000000064.meta.tmp-stale")
	if err := os.WriteFile(tempPath, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocalStagingStore(dir, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("temporary metadata still exists: %v", err)
	}
}

func createOverlayStage(
	t *testing.T,
	dir string,
	baseID InodeID,
	base []byte,
) (*LocalStagingStore, InodeID, StagingFile) {
	t.Helper()
	store, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := MakeInodeID(InodeTypeFile, 100)
	stage, err := store.Create(id, StagingMeta{
		DirID:    MakeInodeID(InodeTypeDir, 1),
		FileName: "file",
		BaseID:   baseID,
		BaseSize: uint64(len(base)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, id, stage
}

func sliceBaseReader(base []byte, calls *[]byteRange) stagingBaseReader {
	return func(_ InodeID, offset uint64, dest []byte) (int, bool, error) {
		end := min(offset+uint64(len(dest)), uint64(len(base)))
		if calls != nil {
			*calls = append(*calls, byteRange{start: offset, end: end})
		}
		if offset >= uint64(len(base)) {
			return 0, true, nil
		}
		n := copy(dest, base[offset:end])
		return n, end == uint64(len(base)), nil
	}
}

func TestStagingOverlayReadAndCache(t *testing.T) {
	base := []byte("abcdefghij")
	baseID := MakeInodeID(InodeTypeFile, 10)
	_, _, stage := createOverlayStage(t, t.TempDir(), baseID, base)

	if err := stage.Write(3, []byte("XYZ")); err != nil {
		t.Fatal(err)
	}
	var calls []byteRange
	buf := make([]byte, len(base))
	n, eof, err := stage.Read(0, buf, sliceBaseReader(base, &calls))
	if err != nil {
		t.Fatal(err)
	}
	if n != len(buf) || !eof || string(buf) != "abcXYZghij" {
		t.Fatalf("read = (%d, %t, %q), want (10, true, %q)",
			n, eof, buf, "abcXYZghij")
	}
	wantCalls := []byteRange{{start: 0, end: 3}, {start: 6, end: 10}}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("base reads = %#v, want %#v", calls, wantCalls)
	}

	failReader := func(InodeID, uint64, []byte) (int, bool, error) {
		return 0, false, errors.New("unexpected base read")
	}
	clear(buf)
	n, eof, err = stage.Read(0, buf, failReader)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(buf) || !eof || string(buf) != "abcXYZghij" {
		t.Fatalf("cached read = (%d, %t, %q)", n, eof, buf)
	}
}

func TestStagingCacheDoesNotOverwriteDirtyData(t *testing.T) {
	base := []byte("abcdefghij")
	baseID := MakeInodeID(InodeTypeFile, 10)
	_, _, stage := createOverlayStage(t, t.TempDir(), baseID, base)

	if err := stage.Write(3, []byte("XYZ")); err != nil {
		t.Fatal(err)
	}
	if err := stage.Cache(0, base); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len(base))
	n, eof, err := stage.Read(0, buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(buf) || !eof || string(buf) != "abcXYZghij" {
		t.Fatalf("read = (%d, %t, %q)", n, eof, buf)
	}
}

func TestStagingFinishRetriesFailedBackgroundHydration(t *testing.T) {
	base := []byte("abcdefghij")
	baseID := MakeInodeID(InodeTypeFile, 10)
	_, _, stage := createOverlayStage(t, t.TempDir(), baseID, base)
	if err := stage.Write(3, []byte("XYZ")); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	failed := make(chan struct{})
	reader := func(
		_ InodeID,
		offset uint64,
		dest []byte,
	) (int, bool, error) {
		if calls.Add(1) == 1 {
			close(failed)
			return 0, false, errors.New("injected read failure")
		}
		end := min(offset+uint64(len(dest)), uint64(len(base)))
		n := copy(dest, base[offset:end])
		return n, end == uint64(len(base)), nil
	}
	stage.StartHydration(hydrationReader(reader))
	<-failed
	if err := stage.FinishHydration(reader); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, len(base))
	if _, _, err := stage.Read(0, buf, nil); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "abcXYZghij" {
		t.Fatalf("hydrated read = %q, want %q", buf, "abcXYZghij")
	}
}

func TestStagingHydrationSkipsFullyDirtyBase(t *testing.T) {
	base := []byte("abcdefghij")
	baseID := MakeInodeID(InodeTypeFile, 10)
	_, _, stage := createOverlayStage(t, t.TempDir(), baseID, base)
	if err := stage.Write(0, []byte("0123456789")); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	reader := func(
		InodeID,
		uint64,
		[]byte,
	) (int, bool, error) {
		calls.Add(1)
		return 0, false, errors.New("unexpected base read")
	}
	stage.StartHydration(hydrationReader(reader))
	if err := stage.FinishHydration(reader); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatalf("base reads = %d, want 0", calls.Load())
	}
}

func TestStagingFinishHydratesInParallel(t *testing.T) {
	const baseSize = 4 * stagingHydrationChunk
	baseID := MakeInodeID(InodeTypeFile, 10)
	_, _, stage := createOverlayStage(
		t, t.TempDir(), baseID, make([]byte, baseSize),
	)
	var active atomic.Int32
	var maximum atomic.Int32
	reader := func(
		_ InodeID,
		_ uint64,
		dest []byte,
	) (int, bool, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			seen := maximum.Load()
			if current <= seen || maximum.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		clear(dest)
		return len(dest), false, nil
	}
	if err := stage.FinishHydration(reader); err != nil {
		t.Fatal(err)
	}
	if maximum.Load() < 2 {
		t.Fatalf("maximum concurrent base reads = %d, want at least 2",
			maximum.Load())
	}
}

func TestStagingRemoveCancelsHydration(t *testing.T) {
	const baseSize = 3 * stagingHydrationChunk
	baseID := MakeInodeID(InodeTypeFile, 10)
	store, id, stage := createOverlayStage(
		t, t.TempDir(), baseID, make([]byte, baseSize),
	)

	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	reader := func(
		_ InodeID,
		_ uint64,
		dest []byte,
	) (int, bool, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		clear(dest)
		return len(dest), false, nil
	}
	stage.StartHydration(hydrationReader(reader))
	<-started
	done := make(chan struct{})
	go func() {
		store.Remove(id)
		close(done)
	}()
	close(release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("staging removal did not cancel hydration")
	}
	if calls.Load() != 1 {
		t.Fatalf("base reads after cancellation = %d, want 1", calls.Load())
	}
}

func TestStagingReadRaceReturnsRemoved(t *testing.T) {
	base := []byte("abcdefghij")
	baseID := MakeInodeID(InodeTypeFile, 10)
	store, id, stage := createOverlayStage(
		t, t.TempDir(), baseID, base,
	)
	started := make(chan struct{})
	release := make(chan struct{})
	reader := func(
		_ InodeID,
		offset uint64,
		dest []byte,
	) (int, bool, error) {
		close(started)
		<-release
		end := min(offset+uint64(len(dest)), uint64(len(base)))
		n := copy(dest, base[offset:end])
		return n, end == uint64(len(base)), nil
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := stage.Read(0, make([]byte, len(base)), reader)
		done <- err
	}()
	<-started
	store.Remove(id)
	close(release)
	if err := <-done; !errors.Is(err, errStagingRemoved) {
		t.Fatalf("racing read error = %v, want %v",
			err, errStagingRemoved)
	}
}

func TestStagingTruncateThenRegrowReturnsZeros(t *testing.T) {
	base := []byte("abcdefghij")
	baseID := MakeInodeID(InodeTypeFile, 10)
	_, _, stage := createOverlayStage(t, t.TempDir(), baseID, base)

	if err := stage.SetSize(5); err != nil {
		t.Fatal(err)
	}
	if err := stage.SetSize(8); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 8)
	n, eof, err := stage.Read(0, buf, sliceBaseReader(base, nil))
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{'a', 'b', 'c', 'd', 'e', 0, 0, 0}
	if n != len(buf) || !eof || !reflect.DeepEqual(buf, want) {
		t.Fatalf("read = (%d, %t, %v), want (8, true, %v)",
			n, eof, buf, want)
	}
}

func TestStagingDirtyRangesSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	base := []byte("abcdefghij")
	baseID := MakeInodeID(InodeTypeFile, 10)
	_, id, stage := createOverlayStage(t, dir, baseID, base)
	if err := stage.Write(3, []byte("XYZ")); err != nil {
		t.Fatal(err)
	}
	if err := stage.Sync(); err != nil {
		t.Fatal(err)
	}

	recovered, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	recoveredStage := recovered.Get(id)
	if recoveredStage == nil {
		t.Fatal("staging file was not recovered")
	}
	meta, ok := recovered.GetMeta(id)
	if !ok {
		t.Fatal("staging metadata was not recovered")
	}
	wantDirty := byteRangeSet{{start: 3, end: 6}}
	if meta.BaseID != baseID || meta.BaseSize != uint64(len(base)) ||
		!reflect.DeepEqual(meta.Dirty, wantDirty) {
		t.Fatalf("recovered metadata = %#v", meta)
	}

	buf := make([]byte, len(base))
	if _, _, err := recoveredStage.Read(
		0, buf, sliceBaseReader(base, nil),
	); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "abcXYZghij" {
		t.Fatalf("recovered read = %q", buf)
	}
}

func TestStagingDropsUncommittedChangesOnRestart(t *testing.T) {
	dir := t.TempDir()
	base := []byte("abcdefghij")
	baseID := MakeInodeID(InodeTypeFile, 10)
	_, id, stage := createOverlayStage(t, dir, baseID, base)
	if err := stage.Write(3, []byte("XYZ")); err != nil {
		t.Fatal(err)
	}
	if err := stage.Write(uint64(len(base)), []byte("extra")); err != nil {
		t.Fatal(err)
	}

	recovered, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	recoveredStage := recovered.Get(id)
	if recoveredStage == nil {
		t.Fatal("staging file was not recovered")
	}
	if size, ok := recovered.StagedSize(id); !ok || size != uint64(len(base)) {
		t.Fatalf("recovered size = (%d, %t), want (%d, true)",
			size, ok, len(base))
	}
	meta, ok := recovered.GetMeta(id)
	if !ok {
		t.Fatal("staging metadata was not recovered")
	}
	if len(meta.Dirty) != 0 {
		t.Fatalf("recovered dirty ranges = %#v, want none", meta.Dirty)
	}

	buf := make([]byte, len(base))
	if _, _, err := recoveredStage.Read(
		0, buf, sliceBaseReader(base, nil),
	); err != nil {
		t.Fatal(err)
	}
	if string(buf) != string(base) {
		t.Fatalf("recovered read = %q, want %q", buf, base)
	}
}

func TestStagingRetainsUncommittedNewFileOnRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := MakeInodeID(InodeTypeFile, 100)
	stage, err := store.Create(id, StagingMeta{
		DirID:    MakeInodeID(InodeTypeDir, 1),
		FileName: "new",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("uncommitted new file")
	if err := stage.Write(0, want); err != nil {
		t.Fatal(err)
	}

	recovered, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	recoveredStage := recovered.Get(id)
	if recoveredStage == nil {
		t.Fatal("staging file was not recovered")
	}
	if size, ok := recovered.StagedSize(id); !ok || size != uint64(len(want)) {
		t.Fatalf("recovered size = (%d, %t), want (%d, true)",
			size, ok, len(want))
	}
	got := make([]byte, len(want))
	n, eof, err := recoveredStage.Read(0, got, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(want) || !eof || !reflect.DeepEqual(got, want) {
		t.Fatalf("recovered read = (%d, %t, %q), want (%d, true, %q)",
			n, eof, got, len(want), want)
	}
}

func TestStagingAllowsIndependentReplacementsOfSameBase(t *testing.T) {
	store, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	baseID := MakeInodeID(InodeTypeFile, 10)
	firstID := MakeInodeID(InodeTypeFile, 100)
	secondID := MakeInodeID(InodeTypeFile, 101)
	meta := StagingMeta{
		DirID:    MakeInodeID(InodeTypeDir, 1),
		FileName: "file",
		BaseID:   baseID,
		BaseSize: 10,
	}
	if _, err := store.Create(firstID, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(secondID, meta); err != nil {
		t.Fatalf("second replacement failed: %v", err)
	}

	store.Remove(firstID)
	if _, err := store.Create(secondID, meta); err != nil {
		t.Fatalf("replacement after close failed: %v", err)
	}
}

func TestStagingKeepsBaseFilehandlePrivate(t *testing.T) {
	store, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	baseID := MakeInodeID(InodeTypeFile, 10)
	stagingID := MakeInodeID(InodeTypeFile, 100)
	stage, err := store.Create(stagingID, StagingMeta{
		DirID:    MakeInodeID(InodeTypeDir, 1),
		FileName: "file",
		BaseID:   baseID,
		BaseSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.Get(baseID) != nil {
		t.Fatal("published base handle exposes private staging")
	}
	if store.Get(baseID) != nil {
		t.Fatal("base handle resolves to a writer")
	}
	store.Remove(baseID)
	if store.Get(stagingID) != stage {
		t.Fatal("removing the base handle discarded private staging")
	}
	store.Remove(stagingID)

}

func TestStagingRebindSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	baseID := MakeInodeID(InodeTypeFile, 10)
	stagingID := MakeInodeID(InodeTypeFile, 100)
	if _, err := store.Create(stagingID, StagingMeta{
		DirID:    MakeInodeID(InodeTypeDir, 1),
		FileName: "file",
		BaseID:   baseID,
		BaseSize: 10,
		ClientID: 1,
	}); err != nil {
		t.Fatal(err)
	}
	stateID := StateID{1, 2, 3}
	if err := store.Rebind(stagingID, 2, stateID); err != nil {
		t.Fatal(err)
	}

	recovered, err := NewLocalStagingStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	meta, ok := recovered.GetMeta(stagingID)
	if !ok {
		t.Fatal("rebound staging was not recovered")
	}
	if meta.ClientID != 2 || meta.NFSStateID != stateID {
		t.Fatalf("recovered owner = (%d, %x), want (2, %x)",
			meta.ClientID, meta.NFSStateID, stateID)
	}
}

func hydrationReader(reader stagingBaseReader) stagingHydrationReader {
	return func(_ <-chan struct{}, id InodeID, offset uint64, dest []byte) (int, bool, error) {
		return reader(id, offset, dest)
	}
}

func findStagingTarget(store StagingStore, dirID InodeID, name string) (InodeID, StagingMeta, bool) {
	for id, meta := range store.FindTargets(dirID, name) {
		return id, meta, true
	}
	return 0, StagingMeta{}, false
}
