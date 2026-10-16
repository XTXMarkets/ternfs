// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestStorageAccounting(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		data, metadata           storageSpace
		realtime                 bool
		dataReserve, metaReserve uint64
		want                     storageSpace
	}{
		{"ordinary filesystem", storageSpace{1000, 600}, storageSpace{}, false, 100, 100, storageSpace{900, 500}},
		{"metadata limits", storageSpace{1000, 800}, storageSpace{100, 30}, true, 0, 0, storageSpace{1000, 300}},
		{"HDD limits", storageSpace{1000, 200}, storageSpace{100, 80}, true, 0, 0, storageSpace{1000, 200}},
		{"both reserves", storageSpace{1100, 900}, storageSpace{110, 40}, true, 100, 10, storageSpace{1000, 300}},
		{"metadata at reserve", storageSpace{1000, 800}, storageSpace{100, 10}, true, 0, 10, storageSpace{1000, 0}},
		{"metadata below reserve", storageSpace{1000, 800}, storageSpace{100, 9}, true, 0, 10, storageSpace{1000, 0}},
		{"metadata reserve exceeds capacity", storageSpace{1000, 800}, storageSpace{100, 100}, true, 0, 101, storageSpace{1000, 0}},
		{"no metadata capacity", storageSpace{1000, 800}, storageSpace{}, true, 0, 0, storageSpace{1000, 0}},
		{"HDD at reserve", storageSpace{1000, 100}, storageSpace{100, 80}, true, 100, 0, storageSpace{900, 0}},
		{"HDD reserve exceeds capacity", storageSpace{1000, 800}, storageSpace{100, 80}, true, 1001, 0, storageSpace{}},
		{"round down", storageSpace{1000, 800}, storageSpace{3, 1}, true, 0, 0, storageSpace{1000, 333}},
		{"large devices", storageSpace{1 << 60, 1 << 59}, storageSpace{1 << 50, 1 << 48}, true, 0, 0, storageSpace{1 << 60, 1 << 58}},
		{"maximum capacity", storageSpace{math.MaxUint64, math.MaxUint64}, storageSpace{math.MaxUint64, math.MaxUint64}, true, 0, 0, storageSpace{math.MaxUint64, math.MaxUint64}},
		{"clamp inconsistent free counts", storageSpace{1000, 1100}, storageSpace{100, 110}, true, 0, 0, storageSpace{1000, 1000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage := blockServiceStorage{data: tc.data, xfsMetadata: tc.metadata, xfsRealtime: tc.realtime}
			storage.account(tc.dataReserve, tc.metaReserve)
			if storage.reported != tc.want {
				t.Fatalf("reported %+v, want %+v", storage.reported, tc.want)
			}
			if storage.data != tc.data || storage.xfsMetadata != tc.metadata {
				t.Fatal("accounting modified raw monitoring counters")
			}
		})
	}
}

func TestCapacityRefreshBeforeRegistration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "secret.key"), make([]byte, 20), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "with_crc"), 0700); err != nil {
		t.Fatal(err)
	}
	bs := &blockService{
		path: dir,
		// Simulate an existing service restored from the registry.
		cachedInfo: msgs.RegisterBlockServiceInfo{Blocks: 123, CapacityBytes: 1, AvailableBytes: 1},
	}
	logger := testLogger(t)
	if err := initBlockServicesInfo(&env{}, logger, 0, msgs.AddrsInfo{}, [16]byte{}, map[msgs.BlockServiceId]*blockService{1: bs}, 0, 0); err != nil {
		t.Fatal(err)
	}
	storage := bs.storage.Load()
	if storage == nil || storage.reported.capacity <= 1 {
		t.Fatalf("startup did not refresh capacity: %+v", storage)
	}
	if bs.cachedInfo.Blocks != 123 {
		t.Fatal("startup discarded registry block count")
	}
	if err := updateBlockServiceInfoCapacity(logger, bs, math.MaxUint64, 0); err != nil {
		t.Fatal(err)
	}
	if bs.storage.Load().reported != (storageSpace{}) {
		t.Fatal("capacity refresh did not apply reserve")
	}
	bs.path = filepath.Join(dir, "missing")
	if err := updateBlockServiceInfoCapacity(logger, bs, 0, 0); !os.IsNotExist(err) {
		t.Fatalf("missing filesystem: got %v", err)
	}
	if bs.storage.Load() != nil {
		t.Fatal("failed refresh retained stale writable capacity")
	}
}

func testLogger(t *testing.T) *log.Logger {
	t.Helper()
	out, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { out.Close() })
	return log.NewLogger(out, &log.LoggerOptions{})
}

func TestWriteSpaceExhaustionAndRecovery(t *testing.T) {
	bs := &blockService{}
	if err := bs.checkWriteSpace(); err != msgs.INTERNAL_ERROR {
		t.Fatalf("unknown capacity should reject writes, got %v", err)
	}
	storage := &blockServiceStorage{
		data: storageSpace{1000, 800}, xfsMetadata: storageSpace{100, 10}, xfsRealtime: true,
	}
	storage.account(0, 10)
	bs.storage.Store(storage)
	if err := bs.checkWriteSpace(); !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("metadata at reserve should reject writes, got %v", err)
	}
	recovered := *storage
	recovered.xfsMetadata.available = 80
	recovered.account(0, 10)
	bs.storage.Store(&recovered)
	if err := bs.checkWriteSpace(); err != nil {
		t.Fatalf("recovered service rejected write: %v", err)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() { bs.recordNoSpace() })
	}
	wg.Wait()
	if err := bs.checkWriteSpace(); !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("ENOSPC did not stop further writes: %v", err)
	}
	if recovered.reported.available == 0 {
		t.Fatal("ENOSPC mutated a published snapshot")
	}
	bs.storage.Store(&recovered)
	if err := bs.checkWriteSpace(); err != nil {
		t.Fatalf("new capacity sample did not allow recovery: %v", err)
	}
}
