// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

/*
#include <errno.h>
#include <linux/fs.h>
#include <stdint.h>
#include <sys/ioctl.h>

// Stable XFS ioctl ABI from linux's fs/xfs/libxfs/xfs_fs.h. Keep these
// definitions here so building ternblocks does not require xfsprogs headers.
struct tern_xfs_geom_v1 {
	uint32_t blocksize, rtextsize, agblocks, agcount;
	uint32_t logblocks, sectsize, inodesize, imaxpct;
	uint64_t datablocks, rtblocks, rtextents, logstart;
	unsigned char uuid[16];
	uint32_t sunit, swidth;
	int32_t version;
	uint32_t flags, logsectsize, rtsectsize, dirblocksize;
};
struct tern_xfs_counts {
	uint64_t freedata, freertx, freeino, allocino;
};

static int tern_xfs_storage(int fd, struct tern_xfs_geom_v1 *geom,
		struct tern_xfs_counts *counts) {
	struct fsxattr attr;
	if (ioctl(fd, FS_IOC_FSGETXATTR, &attr) < 0) {
		return -errno;
	}
	// Check the block directory, not secret.key: the key may have been
	// created before realtime inheritance was enabled.
	if (!(attr.fsx_xflags & FS_XFLAG_RTINHERIT)) {
		return 0;
	}
	if (ioctl(fd, _IOR('X', 100, struct tern_xfs_geom_v1), geom) < 0) {
		return -errno;
	}
	if (geom->rtblocks == 0) {
		return 0;
	}
	if (ioctl(fd, _IOR('X', 113, struct tern_xfs_counts), counts) < 0) {
		return -errno;
	}
	return 1;
}
*/
import "C"

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func readXfsStorage(blockDir string, storage *blockServiceStorage) error {
	dir, err := os.Open(blockDir)
	if err != nil {
		return err
	}
	defer dir.Close()
	var geom C.struct_tern_xfs_geom_v1
	var counts C.struct_tern_xfs_counts
	result := C.tern_xfs_storage(C.int(dir.Fd()), &geom, &counts)
	if result < 0 {
		return fmt.Errorf("query XFS space for %s: %w", blockDir, syscall.Errno(-result))
	}
	if result == 0 {
		// This service writes ordinary files, even if the filesystem also
		// has a realtime device. Account the directory's data-device space.
		var statfs unix.Statfs_t
		if err := unix.Fstatfs(int(dir.Fd()), &statfs); err != nil {
			return err
		}
		storage.data = storageSpace{
			capacity:  statfs.Blocks * uint64(statfs.Bsize),
			available: statfs.Bavail * uint64(statfs.Bsize),
		}
		return nil
	}
	blockSize := uint64(geom.blocksize)
	metadataBlocks := uint64(geom.datablocks)
	if geom.logstart != 0 {
		// Like statfs, exclude the internal log from usable data space.
		metadataBlocks = subtractReserve(metadataBlocks, uint64(geom.logblocks))
	}
	freeMetadataBlocks := uint64(counts.freedata)
	if freeMetadataBlocks > metadataBlocks {
		// FSCOUNTS subtracts XFS's own reservations using unsigned
		// arithmetic. At exhaustion this can underflow; fail closed.
		freeMetadataBlocks = 0
	}
	storage.xfsRealtime = true
	storage.xfsMetadata = storageSpace{
		capacity:  metadataBlocks * blockSize,
		available: freeMetadataBlocks * blockSize,
	}
	storage.data = storageSpace{
		capacity:  uint64(geom.rtblocks) * blockSize,
		available: uint64(counts.freertx) * uint64(geom.rtextsize) * blockSize,
	}
	return nil
}
