// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package blockservice

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"xtx/ternfs/msgs"

	"golang.org/x/sys/unix"
)

type blockReader struct {
	*pageCrcReader
	bs *BlockService
}

func newBlockReader(bs *BlockService, path string, offset uint32, count uint32, readAhead bool, stripCrc bool) (*blockReader, error) {
	bs.logger.Debug("fetching block at path %v", path)
	f, err := os.Open(path)
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	if os.IsNotExist(err) {
		return nil, msgs.BLOCK_NOT_FOUND
	}

	if errors.Is(err, syscall.ENODATA) {
		atomic.AddUint64(&bs.IoAttempts, 1)
		atomic.AddUint64(&bs.IoErrors, 1)
		bs.logger.RaiseHardwareEvent(strings.Split(bs.Path, ":")[0], bs.Id.String(),
			fmt.Sprintf("could not open block %v, got ENODATA, this probably means that the block/disk is gone", path))
		return nil, syscall.EIO
	}

	if err != nil {
		return nil, err
	}

	if offset%msgs.TERN_PAGE_SIZE != 0 {
		bs.logger.Warn("trying to read from offset other than page boundary for path %v", path)
		return nil, msgs.BLOCK_FETCH_OUT_OF_BOUNDS
	}
	if count%msgs.TERN_PAGE_SIZE != 0 {
		bs.logger.Warn("trying to read count not multiple of page size for path %v", path)
		return nil, msgs.BLOCK_FETCH_OUT_OF_BOUNDS
	}

	fileOffset := int64(offset / msgs.TERN_PAGE_SIZE * msgs.TERN_PAGE_WITH_CRC_SIZE)
	var pos int64
	if pos, err = f.Seek(fileOffset, 0); err != nil {
		return nil, err
	}
	if pos != fileOffset {
		return nil, msgs.BLOCK_FETCH_OUT_OF_BOUNDS
	}
	adviseCount := int64(count / msgs.TERN_PAGE_SIZE * msgs.TERN_PAGE_WITH_CRC_SIZE)
	if readAhead {
		var stat unix.Stat_t
		if err := unix.Fstat(int(f.Fd()), &stat); err == nil {
			adviseCount = stat.Size - fileOffset
		}
		unix.Fadvise(int(f.Fd()), fileOffset, adviseCount, unix.FADV_SEQUENTIAL|unix.FADV_WILLNEED)
	}

	bs.wg.Add(1)
	if bs.readC != nil {
		bs.readC <- struct{}{}
	}
	reader := NewPageCrcReader(f, stripCrc)
	f = nil
	return &blockReader{
		pageCrcReader: reader,
		bs:            bs,
	}, nil
}

func (r *blockReader) Read(p []byte) (int, error) {
	atomic.AddUint64(&r.bs.IoAttempts, 1)
	read, err := r.pageCrcReader.Read(p)
	if err != nil && err != io.EOF {
		atomic.AddUint64(&r.bs.IoErrors, 1)
	}
	return read, err
}

func (r *blockReader) Close() error {
	if r.bs.readC != nil {
		<-r.bs.readC
	}
	r.bs.wg.Done()
	return r.pageCrcReader.Close()
}
