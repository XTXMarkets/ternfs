// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"math/bits"
	"path"
	"syscall"

	"github.com/XTXMarkets/ternfs/go/msgs"
	"golang.org/x/sys/unix"
)

type storageSpace struct {
	capacity  uint64
	available uint64
}

func subtractReserve(n, reserve uint64) uint64 {
	if n <= reserve {
		return 0
	}
	return n - reserve
}

func (s storageSpace) reserve(bytes uint64) storageSpace {
	return storageSpace{
		capacity:  subtractReserve(s.capacity, bytes),
		available: subtractReserve(min(s.available, s.capacity), bytes),
	}
}

// Published as one immutable snapshot for registration, metrics and write admission.
type blockServiceStorage struct {
	data                storageSpace
	xfsMetadata         storageSpace
	xfsRealtime         bool
	reservedXfsMetadata uint64
	reported            storageSpace
}

func (s *blockServiceStorage) account(reservedStorage, reservedXfsMetadata uint64) {
	s.reported = s.data.reserve(reservedStorage)
	if !s.xfsRealtime {
		return
	}
	s.reservedXfsMetadata = reservedXfsMetadata
	metadata := s.xfsMetadata.reserve(reservedXfsMetadata)
	if metadata.capacity == 0 {
		s.reported.available = 0
		return
	}
	// Express the metadata free proportion in data-device bytes. Use a 128-bit
	// product so large HDDs do not overflow, and round down conservatively.
	hi, lo := bits.Mul64(s.reported.capacity, metadata.available)
	metadataAvailable, _ := bits.Div64(hi, lo, metadata.capacity)
	s.reported.available = min(s.reported.available, metadataAvailable)
}

func readBlockServiceStorage(basePath string) (*blockServiceStorage, error) {
	var statfs unix.Statfs_t
	if err := unix.Statfs(path.Join(basePath, "secret.key"), &statfs); err != nil {
		return nil, err
	}
	storage := &blockServiceStorage{
		data: storageSpace{
			capacity:  statfs.Blocks * uint64(statfs.Bsize),
			available: statfs.Bavail * uint64(statfs.Bsize),
		},
	}
	if statfs.Type == unix.XFS_SUPER_MAGIC {
		if err := readXfsStorage(path.Join(basePath, "with_crc"), storage); err != nil {
			return nil, err
		}
	}
	return storage, nil
}

func (bs *blockService) checkWriteSpace() error {
	storage := bs.storage.Load()
	if storage == nil {
		return msgs.INTERNAL_ERROR
	}
	if storage.reported.available == 0 {
		return syscall.ENOSPC
	}
	return nil
}

func (bs *blockService) recordNoSpace() {
	// Stop advertising writable space after an actual ENOSPC, even if it
	// arrived between samples. The next successful sample allows recovery.
	for {
		storage := bs.storage.Load()
		if storage == nil || storage.reported.available == 0 {
			return
		}
		exhausted := *storage
		exhausted.reported.available = 0
		if bs.storage.CompareAndSwap(storage, &exhausted) {
			return
		}
	}
}
