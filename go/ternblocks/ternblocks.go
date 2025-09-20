// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/binary"
	"errors"

	"flag"
	"fmt"
	"io"
	"math/rand"
	mrand "math/rand"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/cbcmac"
	"xtx/ternfs/core/certificate"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/core/flags"
	"xtx/ternfs/core/log"
	lrecover "xtx/ternfs/core/recover"
	"xtx/ternfs/core/timing"
	"xtx/ternfs/core/wyhash"
	"xtx/ternfs/msgs"

	"golang.org/x/sys/unix"
)

// #include <unistd.h>
// #include <fcntl.h>
// #include <errno.h>
// #include <stdio.h>
// #include <stdlib.h>
// #include <string.h>
// #include <sys/syscall.h>
//
// struct linux_dirent {
//     unsigned long  d_ino;
//     off_t          d_off;
//     unsigned short d_reclen;
//     char           d_name[];
// };
//
// // If negative, it's an error code.
// ssize_t count_blocks(char* base_path) {
//     int base_fd = -1;
//     int dir_fd = -1;
//     ssize_t blocks = 0;
//     base_fd = open(base_path, O_RDONLY|O_DIRECTORY);
//     if (base_fd < 0) {
//         fprintf(stderr, "could not open %s: %d\n", base_path, errno);
//         blocks = -errno;
//         goto out;
//     }
//     char buf[1024];
//     for (int i = 0; i < 256; i++) {
//         char dir_path[3];
//         snprintf(dir_path, 3, "%02x", i);
//         if (dir_fd >= 0) { close(dir_fd); }
//         dir_fd = openat(base_fd, dir_path, O_RDONLY|O_DIRECTORY);
//         if (dir_fd < 0) {
//             if (errno == ENOENT) {
//                 continue;
//             }
//             fprintf(stderr, "could not open dir %s/%s: %d\n", base_path, dir_path, errno);
//             blocks = -errno;
//             goto out;
//         }
//         for (;;) {
//             long read = syscall(SYS_getdents, dir_fd, buf, sizeof(buf));
//             if (read < 0) {
//                 fprintf(stderr, "could not read direntries in %s/%s: %d\n", base_path, dir_path, errno);
//                 blocks = -errno;
//                 goto out;
//             }
//             if (read == 0) { break; }
//             long bpos = 0;
//             while( bpos < read) {
//                 struct linux_dirent *entry = (struct linux_dirent *)(buf + bpos);
//                 bpos += entry->d_reclen;
//                 if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0 ||
//                     strncmp(entry->d_name, "tmp.", 4) == 0) {
//                         // we only want to count blocks and ignore . and .. and tmp.* files
//                         continue;
//                 }
//                 blocks++;
//             }
//         }
//     }
// out:
//     if (dir_fd >= 0) { close(dir_fd); }
//     if (base_fd >= 0) { close(base_fd); }
//     return blocks;
// }
import "C"

type blockServiceStats struct {
	blocksWritten            uint64
	bytesWritten             uint64
	blocksErased             uint64
	blocksFetched            uint64
	bytesFetched             uint64
	blocksChecked            uint64
	bytesChecked             uint64
	blocksConverted          uint64
	blockConversionDiscarded uint64
}
type env struct {
	bufPool        *bufpool.BufPool
	stats          map[msgs.BlockServiceId]*blockServiceStats
	counters       map[msgs.BlocksMessageKind]*timing.Timings
	eraseLocks     map[msgs.BlockServiceId]*sync.Mutex
	registryConn   *client.RegistryConn
	failureDomain  string
	hostname       string
	pathPrefix     string
	readWholeFile  bool
	ioAlertPercent uint8
}

func BlockWriteProof(blockServiceId msgs.BlockServiceId, blockId msgs.BlockId, key cipher.Block) [8]byte {
	buf := bytes.NewBuffer([]byte{})
	// struct.pack_into('<QcQ', b, 0,  block_service_id, b'W', block_id)
	binary.Write(buf, binary.LittleEndian, uint64(blockServiceId))
	buf.Write([]byte{'W'})
	binary.Write(buf, binary.LittleEndian, uint64(blockId))
	return cbcmac.CBCMAC(key, buf.Bytes())
}

func raiseAlertAndHardwareEvent(logger *log.Logger, hostname string, blockServiceId string, msg string) {
	logger.RaiseHardwareEvent(hostname, blockServiceId, msg)
}

func blockServiceIdFromKey(secretKey [16]byte) msgs.BlockServiceId {
	// we don't really care about leaking part or all of the key -- the whole key business is
	// to defend against bugs, not malicious agents.
	//
	// That said, we prefer to encode the knowledge of how to generate block service ids once,
	// in the block service.
	//
	// Also, we remove the highest bit for the sake of SQLite, at least for now
	return msgs.BlockServiceId(binary.LittleEndian.Uint64(secretKey[:8]) & uint64(0x7FFFFFFFFFFFFFFF))
}

// Do this in one go in C to minimize syscall overhead in golang,
// also I think the busy loop with syscalls in them was starving
// the goroutines waiting on network stuff, which in turn caused
// xmon connections to be dropped.
//
// Note that `findRunnable` picks up network stuff last:
// <https://github.com/golang/go/blob/1d45a7ef560a76318ed59dfdb178cecd58caf948/src/runtime/proc.go#L3222-L3242>
func countBlocks(basePath string) (uint64, error) {
	cBasePath := C.CString(basePath)
	defer C.free(unsafe.Pointer(cBasePath))
	blocks := C.count_blocks(cBasePath)
	if blocks < 0 {
		return 0, syscall.Errno(-blocks)
	}
	return uint64(blocks), nil
}

func updateBlockServiceInfoCapacity(
	_ *log.Logger,
	blockService *blockService,
	reservedStorage uint64,
) error {
	var statfs unix.Statfs_t
	if err := unix.Statfs(path.Join(blockService.path, "secret.key"), &statfs); err != nil {
		return err
	}

	capacityBytes := statfs.Blocks * uint64(statfs.Bsize)
	if capacityBytes < reservedStorage {
		capacityBytes = 0
	} else {
		capacityBytes -= reservedStorage
	}
	availableBytes := statfs.Bavail * uint64(statfs.Bsize)
	if availableBytes < reservedStorage {
		availableBytes = 0
	} else {
		availableBytes -= reservedStorage
	}
	blockService.cachedInfo.CapacityBytes = capacityBytes
	blockService.cachedInfo.AvailableBytes = availableBytes
	return nil
}

// either updates `blockService`, or returns an error.
func updateBlockServiceInfoBlocks(
	log *log.Logger,
	blockService *blockService,
) error {
	t := time.Now()
	log.Info("starting to count blocks for %v", blockService.cachedInfo.Id)
	blocksWithCrc, err := countBlocks(path.Join(blockService.path, "with_crc"))
	if err != nil {
		return err
	}

	blockService.cachedInfo.Blocks = blocksWithCrc
	log.Info("done counting blocks for %v in %v. (blocks: %d)", blockService.cachedInfo.Id, time.Since(t), blocksWithCrc)
	return nil
}

func initBlockServicesInfo(
	env *env,
	log *log.Logger,
	locationId msgs.Location,
	addrs msgs.AddrsInfo,
	failureDomain [16]byte,
	blockServices map[msgs.BlockServiceId]*blockService,
	reservedStorage uint64,
) error {
	log.Info("initializing block services info")
	var wg sync.WaitGroup
	wg.Add(len(blockServices))
	alert := log.NewNCAlert(0)
	log.RaiseNC(alert, "getting info for %v block services", len(blockServices))
	for id, bs := range blockServices {
		bs.cachedInfo.LocationId = locationId
		bs.cachedInfo.Id = id
		bs.cachedInfo.Addrs = addrs
		bs.cachedInfo.SecretKey = bs.key
		bs.cachedInfo.StorageClass = bs.storageClass
		bs.cachedInfo.FailureDomain.Name = failureDomain
		if len(env.pathPrefix) > 0 {
			bs.cachedInfo.Path = fmt.Sprintf("%s:%s", env.pathPrefix, bs.path)
		} else {
			bs.cachedInfo.Path = bs.path
		}
		closureBs := bs
		go func() {
			// only update if it isn't filled it in already from registry
			if closureBs.cachedInfo.Blocks == 0 {
				if err := updateBlockServiceInfoCapacity(log, closureBs, reservedStorage); err != nil {
					panic(err)
				}
				if err := updateBlockServiceInfoBlocks(log, closureBs); err != nil {
					panic(err)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	log.ClearNC(alert)
	return nil
}

var minimumRegisterInterval time.Duration = time.Second * 60
var maximumRegisterInterval time.Duration = minimumRegisterInterval * 2
var variantRegisterInterval time.Duration = maximumRegisterInterval - minimumRegisterInterval

func registerPeriodically(
	log *log.Logger,
	blockServices map[msgs.BlockServiceId]*blockService,
	env *env,
) {
	req := msgs.RegisterBlockServicesReq{}
	alert := log.NewNCAlert(10 * time.Second)
	for {
		req.BlockServices = req.BlockServices[:0]
		for _, bs := range blockServices {
			if bs.couldNotUpdateInfoBlocks || bs.couldNotUpdateInfoCapacity {
				continue
			}
			req.BlockServices = append(req.BlockServices, bs.cachedInfo)
		}
		log.Trace("registering with %+v", req)
		_, err := env.registryConn.Request(&req)
		if err != nil {
			log.RaiseNC(alert, "could not register block services with %+v: %v", env.registryConn.RegistryAddress(), err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		log.ClearNC(alert)
		waitFor := minimumRegisterInterval + time.Duration(mrand.Uint64()%uint64(variantRegisterInterval.Nanoseconds()))
		log.Info("registered with %v (%v alive), waiting %v", env.registryConn.RegistryAddress(), len(blockServices), waitFor)
		time.Sleep(waitFor)
	}
}

func updateBlockServiceInfoBlocksForever(
	log *log.Logger,
	blockServices map[msgs.BlockServiceId]*blockService,
) {
	for {
		for _, bs := range blockServices {
			if err := updateBlockServiceInfoBlocks(log, bs); err != nil {
				bs.couldNotUpdateInfoBlocks = true
				log.RaiseNC(&bs.couldNotUpdateInfoBlocksAlert, "could not count blocks for block service %v: %v", bs.cachedInfo.Id, err)
			} else {
				bs.couldNotUpdateInfoBlocks = false
				log.ClearNC(&bs.couldNotUpdateInfoBlocksAlert)
			}
		}
		time.Sleep(time.Minute) // so that we won't busy loop in tests etc
	}
}

func updateBlockServiceInfoCapacityForever(
	log *log.Logger,
	blockServices map[msgs.BlockServiceId]*blockService,
	reservedStorage uint64,
) {
	for {
		for _, bs := range blockServices {
			if err := updateBlockServiceInfoCapacity(log, bs, reservedStorage); err != nil {
				bs.couldNotUpdateInfoCapacity = true
				log.RaiseNC(&bs.couldNotUpdateInfoCapacityAlert, "could not get capacity for block service %v: %v", bs.cachedInfo.Id, err)
			} else {
				bs.couldNotUpdateInfoCapacity = false
				log.ClearNC(&bs.couldNotUpdateInfoCapacityAlert)
			}
		}
		time.Sleep(10 * time.Second) // so that we won't busy loop in tests etc
	}
}

func checkEraseCertificate(log *log.Logger, blockServiceId msgs.BlockServiceId, cipher cipher.Block, req *msgs.EraseBlockReq) error {
	expectedMac, good := certificate.CheckBlockEraseCertificate(blockServiceId, cipher, req)
	if !good {
		log.RaiseAlert("bad MAC, got %v, expected %v", req.Certificate, expectedMac)
		return msgs.BAD_CERTIFICATE
	}
	return nil
}

func eraseBlock(log *log.Logger, env *env, blockServiceId msgs.BlockServiceId, basePath string, blockId msgs.BlockId) error {
	m := env.eraseLocks[blockServiceId]
	m.Lock()
	defer m.Unlock()
	blockPath := path.Join(basePath, blockId.Path())
	log.Debug("deleting block %v at path %v", blockId, blockPath)
	err := eraseFileIfExistsAndSyncDir(blockPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	atomic.AddUint64(&env.stats[blockServiceId].blocksErased, 1)
	return nil
}

func writeBlocksResponse(log *log.Logger, w io.Writer, resp msgs.BlocksResponse) error {
	log.Trace("writing response %T %+v", resp, resp)
	buf := bytes.NewBuffer([]byte{})
	if err := binary.Write(buf, binary.LittleEndian, msgs.BLOCKS_RESP_PROTOCOL_VERSION); err != nil {
		return err
	}
	if _, err := buf.Write([]byte{uint8(resp.BlocksResponseKind())}); err != nil {
		return err
	}
	if err := resp.Pack(buf); err != nil {
		return err
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

func writeBlocksResponseError(log *log.Logger, w io.Writer, err msgs.TernError) error {
	log.Debug("writing blocks error %v", err)
	buf := bytes.NewBuffer([]byte{})
	if err := binary.Write(buf, binary.LittleEndian, msgs.BLOCKS_RESP_PROTOCOL_VERSION); err != nil {
		return err
	}
	if _, err := buf.Write([]byte{msgs.ERROR}); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(err)); err != nil {
		return err
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

type newToOldReadConverter struct {
	log           *log.Logger
	r             io.Reader
	b             []byte
	totalRead     int
	bytesInBuffer int
}

func (c *newToOldReadConverter) Read(p []byte) (int, error) {
	var read int = 0
	for len(p) > 0 {
		c.log.Debug("{len_p: %d, read, %d, bytesInBuffer: %d, totalRead %d}", len(p), read, c.bytesInBuffer, c.totalRead)
		readFromFile, err := c.r.Read(c.b[c.bytesInBuffer:])
		if err != nil && err != io.EOF {
			return read, err
		}
		c.bytesInBuffer += readFromFile
		if c.bytesInBuffer == 0 && err == io.EOF {
			return read, io.EOF
		}
		offsetInBuffer := 0
		for offsetInBuffer < c.bytesInBuffer && len(p) > 0 {
			toCopy := c.bytesInBuffer - offsetInBuffer
			if toCopy > len(p) {
				toCopy = len(p)
			}
			offSetInPage := c.totalRead % int(msgs.TERN_PAGE_WITH_CRC_SIZE)
			availableInPage := int(msgs.TERN_PAGE_SIZE) - offSetInPage
			if toCopy > availableInPage {
				toCopy = availableInPage
			}
			c.log.Debug("{toCopy: %d, offSetInPage: %d, availableInPage: %d, offsetInBuffer: %d, bytesInBuffer: %d}", toCopy, offSetInPage, availableInPage, offsetInBuffer, c.bytesInBuffer)
			copy(p, c.b[offsetInBuffer:offsetInBuffer+toCopy])
			c.totalRead += toCopy
			read += toCopy
			p = p[toCopy:]
			offsetInBuffer += toCopy
			offSetInPage += toCopy
			if offSetInPage == int(msgs.TERN_PAGE_SIZE) {
				if c.bytesInBuffer-offsetInBuffer < 4 {
					break
				}
				c.totalRead += 4
				offsetInBuffer += 4
			}
		}
		copy(c.b, c.b[offsetInBuffer:c.bytesInBuffer])
		c.bytesInBuffer -= offsetInBuffer
	}
	c.log.Debug("{len_p: %d, read: %d", len(p), read)
	return read, nil
}

func sendFetchBlock(log *log.Logger, env *env, blockServiceId msgs.BlockServiceId, basePath string, blockId msgs.BlockId, offset uint32, count uint32, conn *net.TCPConn, withCrc bool) error {
	if offset%msgs.TERN_PAGE_SIZE != 0 {
		log.RaiseAlert("trying to read from offset other than page boundary")
		return msgs.BLOCK_FETCH_OUT_OF_BOUNDS
	}
	if count%msgs.TERN_PAGE_SIZE != 0 {
		log.RaiseAlert("trying to read count which is not a multiple of page size")
		return msgs.BLOCK_FETCH_OUT_OF_BOUNDS
	}
	pageCount := count / msgs.TERN_PAGE_SIZE
	offsetPageCount := offset / msgs.TERN_PAGE_SIZE
	blockPath := path.Join(basePath, blockId.Path())
	log.Debug("fetching block id %v (%v -> %v) at path %v", blockId, offset, count, blockPath)
	f, err := os.Open(blockPath)

	if errors.Is(err, syscall.ENODATA) {
		// see <https://internal-repo/issues/106>
		raiseAlertAndHardwareEvent(log, env.hostname, blockServiceId.String(),
			fmt.Sprintf("could not open block %v, got ENODATA, this probably means that the block/disk is gone", blockPath))
		// return io error, downstream code will pick it up
		return syscall.EIO
	}

	if os.IsNotExist(err) {
		log.ErrorNoAlert("could not find block to fetch at path %v. Request from client: %v", blockPath, conn.RemoteAddr())
		return msgs.BLOCK_NOT_FOUND
	}

	if err != nil {
		return err
	}
	defer func() {
		if f != nil {
			f.Close()
		}
	}()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	preReadSize := fi.Size()
	filePageCount := uint32(fi.Size()) / msgs.TERN_PAGE_WITH_CRC_SIZE
	if offsetPageCount+pageCount > filePageCount {
		log.RaiseAlert("malformed request for block %v. requested read at [%d - %d] but stored block size is %d", blockId, offset, offset+count, filePageCount*msgs.TERN_PAGE_SIZE)
		return msgs.BLOCK_FETCH_OUT_OF_BOUNDS
	}
	if !env.readWholeFile {
		preReadSize = int64(pageCount) * int64(msgs.TERN_PAGE_WITH_CRC_SIZE)
	}
	var reader io.ReadSeeker = f

	if withCrc {
		offset = offsetPageCount * msgs.TERN_PAGE_WITH_CRC_SIZE
		count = pageCount * msgs.TERN_PAGE_WITH_CRC_SIZE
		unix.Fadvise(int(f.Fd()), int64(offset), preReadSize, unix.FADV_SEQUENTIAL|unix.FADV_WILLNEED)

		if _, err := reader.Seek(int64(offset), 0); err != nil {
			return err
		}
		var resp msgs.BlocksResponse = &msgs.FetchBlockResp{}
		if withCrc {
			resp = &msgs.FetchBlockWithCrcResp{}
		}

		if err := writeBlocksResponse(log, conn, resp); err != nil {
			return err
		}
		lf := io.LimitedReader{
			R: reader,
			N: int64(count),
		}

		read, err := conn.ReadFrom(&lf)
		if err != nil {
			return err
		}
		if read != int64(count) {
			log.RaiseAlert("expected to read at least %v bytes, but only got %v for file %q", count, read, blockPath)
			return msgs.INTERNAL_ERROR
		}
	} else {
		// the only remaining case is that we have a file in new format and client wants old format
		offset = offsetPageCount * msgs.TERN_PAGE_WITH_CRC_SIZE
		unix.Fadvise(int(f.Fd()), int64(offset), preReadSize, unix.FADV_SEQUENTIAL|unix.FADV_WILLNEED)
		if _, err := reader.Seek(int64(offset), 0); err != nil {
			return err
		}
		if err := writeBlocksResponse(log, conn, &msgs.FetchBlockResp{}); err != nil {
			return err
		}
		buf := env.bufPool.Get(1 << 20)
		defer env.bufPool.Put(buf)
		converter := newToOldReadConverter{
			log:           log,
			r:             reader,
			b:             buf.Bytes(),
			totalRead:     0,
			bytesInBuffer: 0,
		}
		lf := io.LimitedReader{
			R: &converter,
			N: int64(count),
		}
		read, err := conn.ReadFrom(&lf)
		if err != nil {
			return err
		}
		if read != int64(count) {
			log.RaiseAlert("expected to read at least %v bytes, but only got %v for file %q", count, read, blockPath)
			return msgs.INTERNAL_ERROR
		}
	}

	s := env.stats[blockServiceId]
	atomic.AddUint64(&s.blocksFetched, 1)
	atomic.AddUint64(&s.bytesFetched, uint64(count))
	return nil
}

func getPhysicalBlockSize(path string) (int, error) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return 0, err
	}
	return int(fs.Bsize), nil
}

func checkBlock(log *log.Logger, env *env, blockServiceId msgs.BlockServiceId, basePath string, blockId msgs.BlockId, expectedSize uint32, crc msgs.Crc, conn *net.TCPConn) error {
	blockPath := path.Join(basePath, blockId.Path())
	log.Debug("checking block id %v at path %v", blockId, blockPath)

	// we try to open no crc first as this file is deleted only after file with crc is created
	// if it doesn't exist in both places then it really is deleted
	f, err := os.Open(blockPath)

	if errors.Is(err, syscall.ENODATA) {
		// see <https://internal-repo/issues/106>
		raiseAlertAndHardwareEvent(log, env.failureDomain, blockServiceId.String(),
			fmt.Sprintf("could not open block %v, got ENODATA, this probably means that the block/disk is gone", blockPath))
		// return io error, downstream code will pick it up
		return syscall.EIO
	}
	if os.IsNotExist(err) {
		return msgs.BLOCK_NOT_FOUND
	}
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		log.ErrorNoAlert("could not read file %v : %v", blockPath, err)
		return err
	}
	s := env.stats[blockServiceId]
	atomic.AddUint64(&s.blocksChecked, 1)
	atomic.AddUint64(&s.bytesChecked, uint64(expectedSize))

	if uint32(fi.Size())%msgs.TERN_PAGE_WITH_CRC_SIZE != 0 {
		log.ErrorNoAlert("size %v for block %v, not multiple of TERN_PAGE_WITH_CRC_SIZE", uint32(fi.Size()), blockPath)
		return msgs.BAD_BLOCK_CRC
	}
	actualDataSize := (uint32(fi.Size()) / msgs.TERN_PAGE_WITH_CRC_SIZE) * msgs.TERN_PAGE_SIZE
	if actualDataSize != expectedSize {
		log.ErrorNoAlert("size %v for block %v, not equal to expected size %v", actualDataSize, blockPath, expectedSize)
		return msgs.BAD_BLOCK_CRC
	}
	bufPtr := env.bufPool.Get(1 << 20)
	defer env.bufPool.Put(bufPtr)
	err = verifyCrcReader(log, bufPtr.Bytes(), f, crc)

	if errors.Is(err, syscall.ENODATA) {
		// see <https://internal-repo/issues/106>
		raiseAlertAndHardwareEvent(log, env.failureDomain, blockServiceId.String(),
			fmt.Sprintf("could not open block %v, got ENODATA, this probably means that the block/disk is gone", blockPath))
		// return io error, downstream code will pick it up
		return syscall.EIO
	}
	if os.IsNotExist(err) {
		return msgs.BLOCK_NOT_FOUND
	}
	if err != nil {
		return err
	}
	if err := writeBlocksResponse(log, conn, &msgs.CheckBlockResp{}); err != nil {
		return err
	}
	return nil
}

func checkWriteCertificate(log *log.Logger, cipher cipher.Block, blockServiceId msgs.BlockServiceId, req *msgs.WriteBlockReq) error {
	expectedMac, good := certificate.CheckBlockWriteCertificate(cipher, blockServiceId, req)
	if !good {
		log.Debug("mac computed for %v %v %v %v", blockServiceId, req.BlockId, req.Crc, req.Size)
		log.RaiseAlert("bad MAC, got %v, expected %v", req.Certificate, expectedMac)
		return msgs.BAD_CERTIFICATE
	}
	return nil
}

func writeToBuf(log *log.Logger, env *env, reader io.Reader, size int64) (*bufpool.Buf, error) {
	readBufPtr := env.bufPool.Get(1 << 20)
	defer env.bufPool.Put(readBufPtr)
	readBuffer := readBufPtr.Bytes()
	var err error

	writeButPtr := env.bufPool.Get(int(size / int64(msgs.TERN_PAGE_SIZE) * int64(msgs.TERN_PAGE_WITH_CRC_SIZE)))
	writeBuffer := writeButPtr.Bytes()
	defer func() {
		if err != nil {
			env.bufPool.Put(writeButPtr)
		}
	}()
	readerHasMoreData := true
	dataInReadBuffer := 0
	dataInWriteBuffer := 0
	endReadOffset := len(readBuffer)
	for readerHasMoreData && size > int64(dataInReadBuffer) {
		if int64(endReadOffset) > size {
			endReadOffset = int(size)
		}

		read, err := reader.Read(readBuffer[dataInReadBuffer:endReadOffset])
		if err != nil {
			if err == io.EOF {
				readerHasMoreData = false
				err = nil
			} else {
				return nil, err
			}
		}
		dataInReadBuffer += read
		if dataInReadBuffer < int(msgs.TERN_PAGE_SIZE) {
			continue
		}
		availablePages := dataInReadBuffer / int(msgs.TERN_PAGE_SIZE)
		for i := 0; i < availablePages; i++ {
			page := readBuffer[i*int(msgs.TERN_PAGE_SIZE) : (i+1)*int(msgs.TERN_PAGE_SIZE)]
			dataInWriteBuffer += copy(writeBuffer[dataInWriteBuffer:], page)
			pageCRC := crc32c.Sum(0, page)
			binary.LittleEndian.PutUint32(writeBuffer[dataInWriteBuffer:dataInWriteBuffer+4], pageCRC)
			dataInWriteBuffer += 4
		}
		size -= int64(availablePages) * int64(msgs.TERN_PAGE_SIZE)
		dataInReadBuffer = copy(readBuffer[:], readBuffer[availablePages*int(msgs.TERN_PAGE_SIZE):dataInReadBuffer])
	}
	if !readerHasMoreData && (size-int64(dataInReadBuffer) > 0) {
		log.Debug("failed converting block, reached EOF in input stream, missing %d bytes", size-int64(dataInReadBuffer))
		err = io.EOF
		return nil, err
	}
	if dataInReadBuffer != 0 || size != 0 {
		log.Debug("failed converting block, unexpected data size. left in read buffer %d, remaining size %d", dataInReadBuffer, size)
		err = msgs.BAD_BLOCK_CRC
		return nil, err
	}
	if dataInWriteBuffer != len(writeBuffer) {
		log.Debug("failed converting block, unexpected write buffer size. expected %d, got %d", len(writeBuffer), dataInWriteBuffer)
		err = msgs.BAD_BLOCK_CRC
		return nil, err
	}
	return writeButPtr, nil
}

func writeBlockInternal(
	log *log.Logger,
	env *env,
	reader io.LimitedReader,
	blockServiceId msgs.BlockServiceId,
	expectedCrc msgs.Crc,
	blockId msgs.BlockId,
	filePath string,
) error {
	// We don't check CRC here, we fully check tmpFile after it has been written and synced
	bufPtr, err := writeToBuf(log, env, reader.R, reader.N)
	if err != nil {
		return err
	}
	defer env.bufPool.Put(bufPtr)

	tmpFile, err := writeBufToTemp(&env.stats[blockServiceId].bytesWritten, path.Dir(filePath), bufPtr.Bytes())
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)
	readBufPtr := env.bufPool.Get(1 << 20)
	defer env.bufPool.Put(readBufPtr)
	err = verifyCrcFile(log, readBufPtr.Bytes(), tmpFile, int64(len(bufPtr.Bytes())), expectedCrc)
	if err != nil {
		log.ErrorNoAlert("failed writing block %v in blockservice %v with error : %v", blockId, blockServiceId, err)
		return err
	}
	err = moveFileAndSyncDir(tmpFile, filePath)
	if err != nil {
		log.ErrorNoAlert("failed writing block %v in blockservice %v with error : %v", blockId, blockServiceId, err)
		return err
	}
	return nil
}

func writeBlock(
	log *log.Logger,
	env *env,
	blockServiceId msgs.BlockServiceId, cipher cipher.Block, basePath string,
	blockId msgs.BlockId, expectedCrc msgs.Crc, size uint32, conn *net.TCPConn,
) error {
	filePath := path.Join(basePath, blockId.Path())
	log.Debug("writing block %v at path %v", blockId, basePath)

	err := writeBlockInternal(log, env, io.LimitedReader{R: conn, N: int64(size)}, blockServiceId, expectedCrc, blockId, filePath)
	if err != nil {
		return err
	}

	log.Debug("writing proof")
	if err := writeBlocksResponse(log, conn, &msgs.WriteBlockResp{Proof: BlockWriteProof(blockServiceId, blockId, cipher)}); err != nil {
		return err
	}
	atomic.AddUint64(&env.stats[blockServiceId].blocksWritten, 1)
	return nil
}

func testWrite(
	log *log.Logger, env *env, blockServiceId msgs.BlockServiceId, basePath string, size uint64, conn *net.TCPConn,
) error {
	filePath := path.Join(basePath, fmt.Sprintf("tmp.test-write%d", rand.Int63()))
	defer os.Remove(filePath)
	err := writeBlockInternal(log, env, io.LimitedReader{R: conn, N: int64(size)}, blockServiceId, 0, msgs.BlockId(0), filePath)
	if err != nil {
		return err
	}

	if err := writeBlocksResponse(log, conn, &msgs.TestWriteResp{}); err != nil {
		return err
	}
	return nil
}

const PAST_CUTOFF time.Duration = 22 * time.Hour
const DEFAULT_FUTURE_CUTOFF time.Duration = 1 * time.Hour
const WRITE_FUTURE_CUTOFF time.Duration = 5 * time.Minute

const MAX_OBJECT_SIZE uint32 = 100 << 20

// The bool is whether we should keep going
func handleRequestError(
	log *log.Logger,
	blockServices map[msgs.BlockServiceId]*blockService,
	deadBlockServices map[msgs.BlockServiceId]deadBlockService,
	conn *net.TCPConn,
	lastError *error,
	blockServiceId msgs.BlockServiceId,
	req msgs.BlocksMessageKind,
	err error,
) bool {
	defer func() {
		*lastError = err
	}()

	if err == io.EOF {
		log.Debug("got EOF from %v, terminating", conn.RemoteAddr())
		return false
	}

	rootErr := err
	for {
		unwrapped := errors.Unwrap(rootErr)
		if unwrapped == nil {
			break
		}
		rootErr = unwrapped
	}

	if netErr, ok := rootErr.(net.Error); ok && netErr.Timeout() {
		log.Debug("got timeout from %v, terminating", conn.RemoteAddr())
		return false
	}
	if sysErr, ok := rootErr.(syscall.Errno); ok {
		if sysErr == syscall.EPIPE {
			log.Info("got broken pipe error from %v, terminating", conn.RemoteAddr())
			return false
		}
		if sysErr == syscall.ECONNRESET {
			log.Info("got connection reset error from %v, terminating", conn.RemoteAddr())
			return false
		}
	}

	if errors.Is(err, syscall.EIO) && blockServiceId != 0 {
		blockService := blockServices[blockServiceId]
		if blockService.couldNotUpdateInfoBlocks || blockService.couldNotUpdateInfoCapacity {
			err = msgs.BLOCK_IO_ERROR_DEVICE
		} else {
			err = msgs.BLOCK_IO_ERROR_FILE
		}
		atomic.AddUint64(&blockService.ioErrors, 1)
		log.ErrorNoAlert("got unxpected IO error %v from %v for req kind %v, block service %v, will return %v, previous error: %v", err, conn.RemoteAddr(), req, blockServiceId, err, *lastError)
		writeBlocksResponseError(log, conn, err.(msgs.TernError))
		return false
	}

	// In general, refuse to service requests for block services that we
	// don't have. In the case of checking a dead block, we don't emit
	// an alert, since it's expected when the scrubber is running.
	// Similarly, reading blocks from old block services can happen if
	// the cached span structure is used in the kmod.
	if _, isDead := deadBlockServices[blockServiceId]; isDead && (req == msgs.CHECK_BLOCK || req == msgs.FETCH_BLOCK || req == msgs.FETCH_BLOCK_WITH_CRC) {
		log.Info("got fetch/check block request for dead block service %v", blockServiceId)
		if ternErr, isTernErr := err.(msgs.TernError); isTernErr {
			if err := writeBlocksResponseError(log, conn, ternErr); err != nil {
				log.Info("could not write response error to %v, will terminate connection: %v", conn.RemoteAddr(), err)
				return false
			}
		}
		return true
	}

	// we always raise an alert since this is almost always bad news in the block service
	if !errors.Is(err, syscall.ENOSPC) && err != msgs.BLOCK_SERVICE_NOT_FOUND && err != msgs.BLOCK_NOT_FOUND {
		log.RaiseAlertStack("", 1, "got unexpected error %v from %v for req kind %v, block service %v, previous error %v", err, conn.RemoteAddr(), req, blockServiceId, *lastError)
	}

	if ternErr, isTernErr := err.(msgs.TernError); isTernErr {
		if err := writeBlocksResponseError(log, conn, ternErr); err != nil {
			log.Info("could not write response error to %v, will terminate connection: %v", conn.RemoteAddr(), err)
			return false
		}
		// Keep the connection around using a whitelist of conditions where we know
		// that the stream is safe. Right now I just added one case which I know
		// is safe, we can add others conservatively in the future if we wish to.
		safeError := false
		safeError = safeError || ((req == msgs.CHECK_BLOCK || req == msgs.FETCH_BLOCK || req == msgs.FETCH_BLOCK_WITH_CRC) && ternErr == msgs.BLOCK_NOT_FOUND)
		if safeError {
			log.Info("preserving connection from %v after err %v", conn.RemoteAddr(), err)
			return true
		} else {
			log.Info("not preserving connection from %v after err %v", conn.RemoteAddr(), err)
			return false
		}
	} else {
		// attempt to say goodbye, ignore errors
		writeBlocksResponseError(log, conn, msgs.INTERNAL_ERROR)
		log.Info("tearing down connection from %v after internal error %v", conn.RemoteAddr(), err)
		return false
	}
}

type deadBlockService struct{}

func readBlocksRequest(
	log *log.Logger,
	r io.Reader,
) (msgs.BlockServiceId, msgs.BlocksRequest, error) {
	var protocol uint32
	if err := binary.Read(r, binary.LittleEndian, &protocol); err != nil {
		return 0, nil, err
	}
	if protocol != msgs.BLOCKS_REQ_PROTOCOL_VERSION {
		log.RaiseAlert("bad blocks protocol, expected %v, got %v", msgs.BLOCKS_REQ_PROTOCOL_VERSION, protocol)
		return 0, nil, msgs.MALFORMED_REQUEST
	}
	var blockServiceId uint64
	if err := binary.Read(r, binary.LittleEndian, &blockServiceId); err != nil {
		return 0, nil, err
	}
	var kindByte [1]byte
	if _, err := io.ReadFull(r, kindByte[:]); err != nil {
		return 0, nil, err
	}
	kind := msgs.BlocksMessageKind(kindByte[0])
	var req msgs.BlocksRequest
	switch kind {
	case msgs.ERASE_BLOCK:
		req = &msgs.EraseBlockReq{}
	case msgs.FETCH_BLOCK:
		req = &msgs.FetchBlockReq{}
	case msgs.FETCH_BLOCK_WITH_CRC:
		req = &msgs.FetchBlockWithCrcReq{}
	case msgs.WRITE_BLOCK:
		req = &msgs.WriteBlockReq{}
	case msgs.TEST_WRITE:
		req = &msgs.TestWriteReq{}
	case msgs.CHECK_BLOCK:
		req = &msgs.CheckBlockReq{}
	default:
		log.RaiseAlert("bad blocks request kind %v", kind)
		return 0, nil, msgs.MALFORMED_REQUEST
	}
	if err := req.Unpack(r); err != nil {
		return 0, nil, err
	}
	return msgs.BlockServiceId(blockServiceId), req, nil
}

// The bool tells us whether we should keep going
func handleSingleRequest(
	log *log.Logger,
	env *env,
	_ chan any,
	lastError *error,
	blockServices map[msgs.BlockServiceId]*blockService,
	deadBlockServices map[msgs.BlockServiceId]deadBlockService,
	conn *net.TCPConn,
	futureCutoff time.Duration,
	connectionTimeout time.Duration,
) bool {
	if connectionTimeout != 0 {
		conn.SetReadDeadline(time.Now().Add(connectionTimeout))
	}
	blockServiceId, req, err := readBlocksRequest(log, conn)
	if err != nil {
		return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, 0, 0, err)
	}
	kind := req.BlocksRequestKind()
	t := time.Now()
	defer func() {
		env.counters[kind].Add(time.Since(t))
	}()
	log.Debug("servicing request of type %T from %v", req, conn.RemoteAddr())
	log.Trace("req %+v", req)
	defer log.Debug("serviced request of type %T from %v", req, conn.RemoteAddr())
	if connectionTimeout != 0 {
		// Reset timeout, with default settings this will give
		// the request a minute to complete, given that the max
		// block size is 10MiB, that is ~0.17MiB/s, so it should
		// be plenty of time unless something is wrong.
		//
		// If we didn't reset this (or just remove the timeout)
		// the previous timeout might very well trip the request
		// because it might have been almost expired.
		conn.SetDeadline(time.Now().Add(connectionTimeout))
	}
	blockService, found := blockServices[blockServiceId]
	if !found {
		// In general, refuse to service requests for block services that we
		// don't have.
		return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, msgs.BLOCK_SERVICE_NOT_FOUND)
	}
	atomic.AddUint64(&blockService.requests, 1)
	switch whichReq := req.(type) {
	case *msgs.EraseBlockReq:
		if err := checkEraseCertificate(log, blockServiceId, blockService.cipher, whichReq); err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		cutoffTime := msgs.TernTime(uint64(whichReq.BlockId)).Time().Add(futureCutoff)
		now := time.Now()
		if now.Before(cutoffTime) {
			log.ErrorNoAlert("block %v is too recent to be deleted (now=%v, cutoffTime=%v)", whichReq.BlockId, now, cutoffTime)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, msgs.BLOCK_TOO_RECENT_FOR_DELETION)
		}
		if err := eraseBlock(log, env, blockServiceId, blockService.path, whichReq.BlockId); err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}

		resp := msgs.EraseBlockResp{
			Proof: certificate.BlockEraseProof(blockServiceId, whichReq.BlockId, blockService.cipher),
		}
		if err := writeBlocksResponse(log, conn, &resp); err != nil {
			log.Info("could not send blocks response to %v: %v", conn.RemoteAddr(), err)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
	case *msgs.FetchBlockReq:
		if err := sendFetchBlock(log, env, blockServiceId, blockService.path, whichReq.BlockId, whichReq.Offset, whichReq.Count, conn, false); err != nil {
			log.Info("could not send block response to %v: %v", conn.RemoteAddr(), err)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
	case *msgs.FetchBlockWithCrcReq:
		if err := sendFetchBlock(log, env, blockServiceId, blockService.path, whichReq.BlockId, whichReq.Offset, whichReq.Count, conn, true); err != nil {
			log.Info("could not send block response to %v: %v", conn.RemoteAddr(), err)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
	case *msgs.WriteBlockReq:
		pastCutoffTime := msgs.TernTime(uint64(whichReq.BlockId)).Time().Add(-PAST_CUTOFF)
		futureCutoffTime := msgs.TernTime(uint64(whichReq.BlockId)).Time().Add(WRITE_FUTURE_CUTOFF)
		now := time.Now()
		if now.Before(pastCutoffTime) {
			panic(fmt.Errorf("block %v is in the future! (now=%v, pastCutoffTime=%v)", whichReq.BlockId, now, pastCutoffTime))
		}
		if now.After(futureCutoffTime) {
			log.ErrorNoAlert("block %v is too old to be written (now=%v, futureCutoffTime=%v)", whichReq.BlockId, now, futureCutoffTime)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, msgs.BLOCK_TOO_OLD_FOR_WRITE)
		}
		if err := checkWriteCertificate(log, blockService.cipher, blockServiceId, whichReq); err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		if whichReq.Size > MAX_OBJECT_SIZE {
			log.RaiseAlert("block %v exceeds max object size: %v > %v", whichReq.BlockId, whichReq.Size, MAX_OBJECT_SIZE)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, msgs.BLOCK_TOO_BIG)
		}
		if err := writeBlock(log, env, blockServiceId, blockService.cipher, blockService.path, whichReq.BlockId, whichReq.Crc, whichReq.Size, conn); err != nil {
			log.Info("could not write block: %v", err)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
	case *msgs.CheckBlockReq:
		if err := checkBlock(log, env, blockServiceId, blockService.path, whichReq.BlockId, whichReq.Size, whichReq.Crc, conn); err != nil {
			log.Info("checking block failed, conn %v, err %v", conn.RemoteAddr(), err)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
	case *msgs.TestWriteReq:
		if err := testWrite(log, env, blockServiceId, blockService.path, whichReq.Size, conn); err != nil {
			log.Info("could not perform test write: %v", err)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
	default:
		return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, fmt.Errorf("bad request type %T", req))
	}
	return true
}

func handleRequest(
	log *log.Logger,
	env *env,
	terminateChan chan any,
	blockServices map[msgs.BlockServiceId]*blockService,
	deadBlockServices map[msgs.BlockServiceId]deadBlockService,
	conn *net.TCPConn,
	futureCutoff time.Duration,
	connectionTimeout time.Duration,
) {
	defer conn.Close()

	var lastError error

	for {
		keepGoing := handleSingleRequest(log, env, terminateChan, &lastError, blockServices, deadBlockServices, conn, futureCutoff, connectionTimeout)
		if !keepGoing {
			return
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %v DIRECTORY STORAGE_CLASS...\n\n", os.Args[0])
	description := `
For each directory/storage class pair specified we'll have one block
service. The block service id for each will be automatically generated
when running for the first time. The failure domain will be the same.

The intention is that a single blockservice process will service
a storage node.

Options:`
	description = strings.TrimSpace(description)
	fmt.Fprintln(os.Stderr, description)
	flag.PrintDefaults()
}

func retrieveOrCreateKey(log *log.Logger, dir string) [16]byte {
	var err error
	var keyFile *os.File
	keyFilePath := path.Join(dir, "secret.key")
	keyFile, err = os.OpenFile(keyFilePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not open key file %v: %v\n", keyFilePath, err)
		os.Exit(1)
	}
	keyFile.Seek(0, 0)
	if err := syscall.Flock(int(keyFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fmt.Fprintf(os.Stderr, "could not lock key file %v: %v\n", keyFilePath, err)
		os.Exit(1)
	}
	var key [16]byte
	var read int
	read, err = keyFile.Read(key[:])
	if err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "could not read key file %v: %v\n", keyFilePath, err)
		os.Exit(1)
	}
	if err == io.EOF {
		log.Info("creating new secret key")
		if _, err := crand.Read(key[:]); err != nil {
			panic(err)
		}
		if _, err := keyFile.Write(key[:]); err != nil {
			panic(err)
		}
		keyCrc := crc32c.Sum(0, key[:])
		if err := binary.Write(keyFile, binary.LittleEndian, keyCrc); err != nil {
			panic(err)
		}
		log.Info("creating directory structure")
		if err := os.Mkdir(path.Join(dir, "with_crc"), 0755); err != nil && !os.IsExist(err) {
			panic(fmt.Errorf("failed to create folder %s error: %v", path.Join(dir, "with_crc"), err))
		}
	} else if read != 16 {
		panic(fmt.Errorf("short secret key (%v rather than 16 bytes)", read))
	} else {
		expectedKeyCrc := crc32c.Sum(0, key[:])
		var actualKeyCrc uint32
		if err := binary.Read(keyFile, binary.LittleEndian, &actualKeyCrc); err != nil {
			panic(err)
		}
		if expectedKeyCrc != actualKeyCrc {
			panic(fmt.Errorf("expected crc %v, got %v", msgs.Crc(expectedKeyCrc), msgs.Crc(actualKeyCrc)))
		}
	}
	return key
}

type diskStats struct {
	readMs       uint64
	writeMs      uint64
	weightedIoMs uint64
}

func getDiskStats(log *log.Logger, statsPath string) (map[string]diskStats, error) {
	file, err := os.Open(statsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	ret := make(map[string]diskStats)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 11 {
			log.RaiseAlert("malformed disk entry in %v: %v", statsPath, line)
			continue
		}
		devId := fmt.Sprintf("%s:%s", fields[0], fields[1])
		// https://www.kernel.org/doc/html/v5.4/admin-guide/iostats.html
		readMs, err := strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse reads ms from %s", line)
		}
		writeMs, err := strconv.ParseUint(fields[10], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse write ms from %s", line)
		}
		weightedIoMs, err := strconv.ParseUint(fields[11], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse weighted IO ms from %s", line)
		}

		ret[devId] = diskStats{
			readMs:       readMs,
			writeMs:      writeMs,
			weightedIoMs: weightedIoMs,
		}
	}
	return ret, nil
}

func raiseAlerts(log *log.Logger, env *env, blockServices map[msgs.BlockServiceId]*blockService) {
	for {
		for bsId, bs := range blockServices {
			ioErrors := bs.lastIoErrors
			requests := bs.lastRequests
			bs.lastIoErrors = atomic.LoadUint64(&bs.ioErrors)
			bs.lastRequests = atomic.LoadUint64(&bs.requests)
			ioErrors = bs.lastIoErrors - ioErrors
			requests = bs.lastRequests - requests
			if requests*uint64(env.ioAlertPercent) < ioErrors*100 {
				log.RaiseNC(&bs.ioErrorsAlert, "block service %v had %v ioErrors from %v requests in the last 5 minutes which is over the %d%% threshold", bsId, ioErrors, requests, env.ioAlertPercent)
				log.Info("decommissioning block service %v", bs.cachedInfo.Id)
				env.registryConn.Request(&msgs.DecommissionBlockServiceReq{Id: bs.cachedInfo.Id})
			} else {
				log.ClearNC(&bs.ioErrorsAlert)
			}
		}
		time.Sleep(5 * time.Minute)
	}
}

func sendMetrics(l *log.Logger, env *env, influxDB *log.InfluxDB, blockServices map[msgs.BlockServiceId]*blockService, failureDomain string) {
	metrics := log.MetricsBuilder{}
	rand := wyhash.New(mrand.Uint64())
	alert := l.NewNCAlert(10 * time.Second)
	failureDomainEscaped := strings.ReplaceAll(failureDomain, " ", "-")
	for {
		diskMetrics, err := getDiskStats(l, "/proc/diskstats")
		if err != nil {
			l.RaiseAlert("failed reading diskstats: %v", err)
		}
		l.Info("sending metrics")
		metrics.Reset()
		now := time.Now()
		for bsId, bsStats := range env.stats {
			metrics.Measurement("eggsfs_blocks_write")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.FieldU64("bytes", bsStats.bytesWritten)
			metrics.FieldU64("blocks", bsStats.blocksWritten)
			metrics.Timestamp(now)

			metrics.Measurement("eggsfs_blocks_read")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.FieldU64("bytes", bsStats.bytesFetched)
			metrics.FieldU64("blocks", bsStats.blocksFetched)
			metrics.Timestamp(now)

			metrics.Measurement("eggsfs_blocks_erase")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.FieldU64("blocks", bsStats.blocksErased)
			metrics.Timestamp(now)

			metrics.Measurement("eggsfs_blocks_check")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.FieldU64("blocks", bsStats.blocksChecked)
			metrics.FieldU64("bytes", bsStats.bytesChecked)
			metrics.Timestamp(now)
		}
		for bsId, bsInfo := range blockServices {
			metrics.Measurement("eggsfs_blocks_storage")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.Tag("storageclass", bsInfo.storageClass.String())
			metrics.FieldU64("capacity", bsInfo.cachedInfo.CapacityBytes)
			metrics.FieldU64("available", bsInfo.cachedInfo.AvailableBytes)
			metrics.FieldU64("blocks", bsInfo.cachedInfo.Blocks)
			metrics.FieldU64("io_errors", bsInfo.ioErrors)
			dm, found := diskMetrics[bsInfo.devId]
			if found {
				metrics.FieldU64("read_ms", dm.readMs)
				metrics.FieldU64("write_ms", dm.writeMs)
				metrics.FieldU64("weighted_io_ms", dm.weightedIoMs)
			}
			metrics.Timestamp(now)
		}
		err = influxDB.SendMetrics(metrics.Payload())
		if err == nil {
			l.ClearNC(alert)
			sleepFor := time.Minute + time.Duration(rand.Uint64() & ^(uint64(1)<<63))%time.Minute
			l.Info("metrics sent, sleeping for %v", sleepFor)
			time.Sleep(sleepFor)
		} else {
			l.RaiseNC(alert, "failed to send metrics, will try again in a second: %v", err)
			time.Sleep(time.Second)
		}
	}
}

type blockService struct {
	path                            string
	devId                           string
	key                             [16]byte
	cipher                          cipher.Block
	storageClass                    msgs.StorageClass
	cachedInfo                      msgs.RegisterBlockServiceInfo
	couldNotUpdateInfoBlocks        bool
	couldNotUpdateInfoBlocksAlert   log.XmonNCAlert
	couldNotUpdateInfoCapacity      bool
	couldNotUpdateInfoCapacityAlert log.XmonNCAlert
	ioErrorsAlert                   log.XmonNCAlert
	ioErrors                        uint64
	requests                        uint64
	lastIoErrors                    uint64
	lastRequests                    uint64
}

func getMountsInfo(log *log.Logger, mountsPath string) (map[string]string, error) {
	file, err := os.Open(mountsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	ret := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		mountFields := strings.Fields(line)
		if len(mountFields) < 11 {
			log.RaiseAlert("malformed mount in %v: %v", mountsPath, line)
			continue
		}
		path := mountFields[4]
		// should be major:minor of the mounted disk
		ret[path] = mountFields[2]
	}
	return ret, nil
}

func main() {
	flag.Usage = usage
	failureDomainStr := flag.String("failure-domain", "", "Failure domain")
	hostname := flag.String("hostname", "", "Hostname (for hardware event reporting)")
	pathPrefixStr := flag.String("path-prefix", "", "We filter our block service not only by failure domain but also by path prefix")

	futureCutoff := flag.Duration("future-cutoff", DEFAULT_FUTURE_CUTOFF, "")
	var addresses flags.StringArrayFlags
	flag.Var(&addresses, "addr", "Addresses (up to two) to bind to, and that will be advertised to registry.")
	verbose := flag.Bool("verbose", false, "")
	xmon := flag.String("xmon", "", "Xmon address (empty for no xmon)")
	trace := flag.Bool("trace", false, "")
	logFile := flag.String("log-file", "", "If empty, stdout")
	registryAddress := flag.String("registry", "", "Registry address (host:port).")
	hardwareEventAddress := flag.String("hardwareevent", "", "Server address (host:port) to send hardware events to OR empty for no event logging")
	profileFile := flag.String("profile-file", "", "")
	syslog := flag.Bool("syslog", false, "")
	connectionTimeout := flag.Duration("connection-timeout", 10*time.Minute, "")
	reservedStorage := flag.Uint64("reserved-storage", 100<<30, "How many bytes to reserve and under-report capacity")
	influxDBOrigin := flag.String("influx-db-origin", "", "Base URL to InfluxDB endpoint")
	influxDBOrg := flag.String("influx-db-org", "", "InfluxDB org")
	influxDBBucket := flag.String("influx-db-bucket", "", "InfluxDB bucket")
	locationId := flag.Uint("location", 10000, "Location ID")
	readWholeFile := flag.Bool("read-whole-file", false, "")
	ioAlertPercent := flag.Uint("io-alert-percent", 10, "Threshold percent of I/O errors over which we alert")
	registryConnectionTimeout := flag.Duration("registry-connection-timeout", 10*time.Second, "")

	flag.Parse()
	flagErrors := false
	if flag.NArg()%2 != 0 {
		fmt.Fprintf(os.Stderr, "Malformed directory/storage class pairs.\n\n")
		flagErrors = true
	}
	if flag.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "Expected at least one block service.\n\n")
		flagErrors = true
	}

	if *registryAddress == "" {
		fmt.Fprintf(os.Stderr, "You need to specify -registry.\n")
		flagErrors = true
	}

	if len(addresses) == 0 || len(addresses) > 2 {
		fmt.Fprintf(os.Stderr, "at least one -addr and no more than two needs to be provided\n")
		flagErrors = true
	}

	if *locationId > 255 {
		fmt.Fprintf(os.Stderr, "Provide valid location id\n")
		flagErrors = true
	}
	if *ioAlertPercent > 100 {
		fmt.Fprintf(os.Stderr, "io-alert-percent should not be above 100\n")
		flagErrors = true
	}

	if *failureDomainStr == "" {
		fmt.Fprintf(os.Stderr, "failure-domain can not be empty\n")
		flagErrors = true
	}

	if *pathPrefixStr == "" {
		*pathPrefixStr = *failureDomainStr
	}

	if *hardwareEventAddress != "" && *hostname == "" {
		fmt.Fprintf(os.Stderr, "-hostname must be provided if you need hardware event reporting\n")
		flagErrors = true
	}

	var influxDB *log.InfluxDB
	if *influxDBOrigin == "" {
		if *influxDBOrg != "" || *influxDBBucket != "" {
			fmt.Fprintf(os.Stderr, "Either all or none of the -influx-db flags must be passed\n")
			flagErrors = true
		}
	} else {
		if *influxDBOrg == "" || *influxDBBucket == "" {
			fmt.Fprintf(os.Stderr, "Either all or none of the -influx-db flags must be passed\n")
			flagErrors = true
		}
		influxDB = &log.InfluxDB{
			Origin: *influxDBOrigin,
			Org:    *influxDBOrg,
			Bucket: *influxDBBucket,
		}
	}

	if flagErrors {
		usage()
		os.Exit(2)
	}

	ownIp1, port1, err := flags.ParseIPV4Addr(addresses[0])
	if err != nil {
		panic(err)
	}
	var ownIp2 [4]byte
	var port2 uint16
	if len(addresses) == 2 {
		ownIp2, port2, err = flags.ParseIPV4Addr(addresses[1])
		if err != nil {
			panic(err)
		}
	}

	var failureDomain [16]byte
	if copy(failureDomain[:], []byte(*failureDomainStr)) != len(*failureDomainStr) {
		fmt.Fprintf(os.Stderr, "Failure domain too long -- must be at most 16 characters: %v\n\n", *failureDomainStr)
		usage()
		os.Exit(2)
	}

	// create all directories first, we might need them for the log output
	for i := 0; i < flag.NArg(); i += 2 {
		dir := flag.Args()[i]
		if err := os.Mkdir(dir, 0777); err != nil && !os.IsExist(err) {
			panic(fmt.Errorf("could not create data dir %v", dir))
		}
	}

	logOut := os.Stdout
	if *logFile != "" {
		var err error
		logOut, err = os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not open log file %v: %v\n", *logFile, err)
			os.Exit(1)
		}
		defer logOut.Close()
	}
	level := log.INFO
	if *verbose {
		level = log.DEBUG
	}
	if *trace {
		level = log.TRACE
	}
	l := log.NewLogger(logOut, &log.LoggerOptions{
		Level:                  level,
		Syslog:                 *syslog,
		XmonAddr:               *xmon,
		HardwareEventServerURL: *hardwareEventAddress,
		AppInstance:            "eggsblocks",
		AppType:                "restech_eggsfs.daytime",
		PrintQuietAlerts:       true,
	})

	if *profileFile != "" {
		f, err := os.Create(*profileFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open profile file %v", *profileFile)
			os.Exit(1)
		}
		pprof.StartCPUProfile(f)
		stopCpuProfile := func() {
			l.Info("stopping cpu profile")
			pprof.StopCPUProfile()
		}
		defer stopCpuProfile()
		// Save CPU profile if we get killed by a signal
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGILL, syscall.SIGTRAP, syscall.SIGABRT, syscall.SIGSTKFLT, syscall.SIGSYS)
		go func() {
			sig := <-signalChan
			signal.Stop(signalChan)
			stopCpuProfile()
			syscall.Kill(syscall.Getpid(), sig.(syscall.Signal))
		}()
	}

	l.Info("Running block service with options:")
	l.Info("  locationId = %v", *locationId)
	l.Info("  failureDomain = %v", *failureDomainStr)
	l.Info("  pathPrefix = %v", *pathPrefixStr)
	l.Info("  futureCutoff = %v", *futureCutoff)
	l.Info("  addr = '%v'", addresses)
	l.Info("  logLevel = %v", level)
	l.Info("  logFile = '%v'", *logFile)
	l.Info("  registryAddress = '%v'", *registryAddress)
	l.Info("  connectionTimeout = %v", *connectionTimeout)
	l.Info("  reservedStorage = %v", *reservedStorage)
	l.Info("  registryConnectionTimeout = %v", *registryConnectionTimeout)

	bufPool := bufpool.NewBufPool()
	env := &env{
		bufPool:        bufPool,
		stats:          make(map[msgs.BlockServiceId]*blockServiceStats),
		eraseLocks:     make(map[msgs.BlockServiceId]*sync.Mutex),
		readWholeFile:  *readWholeFile,
		failureDomain:  *failureDomainStr,
		pathPrefix:     *pathPrefixStr,
		ioAlertPercent: uint8(*ioAlertPercent),
		registryConn:   client.MakeRegistryConn(l, nil, *registryAddress, 1),
	}

	mountsInfo, err := getMountsInfo(l, "/proc/self/mountinfo")
	if err != nil {
		l.RaiseAlert("Disk stats for mounted paths will not be collected due to failure collecting mount info: %v", err)
	}

	blockServices := make(map[msgs.BlockServiceId]*blockService)
	for i := 0; i < flag.NArg(); i += 2 {
		dir := flag.Args()[i]
		storageClass := msgs.StorageClassFromString(flag.Args()[i+1])
		if storageClass == msgs.EMPTY_STORAGE || storageClass == msgs.INLINE_STORAGE {
			fmt.Fprintf(os.Stderr, "Storage class cannot be EMPTY/INLINE")
			os.Exit(2)
		}
		key := retrieveOrCreateKey(l, dir)
		id := blockServiceIdFromKey(key)
		cipher, err := aes.NewCipher(key[:])
		if err != nil {
			panic(fmt.Errorf("could not create AES-128 key: %w", err))
		}
		devId, found := mountsInfo[dir]
		if !found {
			devId = ""
		}
		blockServices[id] = &blockService{
			path:                            dir,
			devId:                           devId,
			key:                             key,
			cipher:                          cipher,
			storageClass:                    storageClass,
			couldNotUpdateInfoBlocksAlert:   *l.NewNCAlert(time.Second),
			couldNotUpdateInfoCapacityAlert: *l.NewNCAlert(time.Second),
			ioErrorsAlert:                   *l.NewNCAlert(time.Second),
		}
	}
	for id, blockService := range blockServices {
		l.Info("block service %v at %v, storage class %v", id, blockService.path, blockService.storageClass)
	}

	if len(blockServices) != flag.NArg()/2 {
		panic(fmt.Errorf("duplicate block services"))
	}

	// Now ask registry for block services we _had_ before. We need to know this to honor
	// erase block requests for old block services safely.
	deadBlockServices := make(map[msgs.BlockServiceId]deadBlockService)
	{
		var registryBlockServices []msgs.BlockServiceDeprecatedInfo
		{
			alert := l.NewNCAlert(0)
			l.RaiseNC(alert, "fetching block services")

			resp, err := env.registryConn.Request(&msgs.AllBlockServicesDeprecatedReq{})
			if err != nil {
				panic(fmt.Errorf("could not request block services from registry: %v", err))
			}
			l.ClearNC(alert)
			registryBlockServices = resp.(*msgs.AllBlockServicesDeprecatedResp).BlockServices
		}
		for i := range registryBlockServices {
			bs := &registryBlockServices[i]
			ourBs, weHaveBs := blockServices[bs.Id]
			sameFailureDomain := bs.FailureDomain.Name == failureDomain
			if len(env.pathPrefix) > 0 {
				pathParts := strings.Split(bs.Path, ":")
				if len(pathParts) == 2 {
					sameFailureDomain = pathParts[0] == env.pathPrefix
				}
			}
			isDecommissioned := (bs.Flags & msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED) != 0
			// No disagreement on failure domain with registry (otherwise we could end up with
			// a split brain scenario where two eggsblocks processes assume control of two dead
			// block services)
			if weHaveBs && !sameFailureDomain {
				panic(fmt.Errorf("we have block service %v, and we're failure domain %v, but registry thinks it should be failure domain %v. If you've moved this block service, change the failure domain on registry", bs.Id, failureDomain, bs.FailureDomain))
			}
			// block services in the same failure domain, which we do not have, must be
			// decommissioned
			if !weHaveBs && sameFailureDomain {
				if !isDecommissioned {
					panic(fmt.Errorf("registry has block service %v for our failure domain %v, but we don't have this block service, and it is not decommissioned. If the block service is dead, mark it as decommissioned", bs.Id, failureDomain))
				}
				deadBlockServices[bs.Id] = deadBlockService{}
			}
			// we can't have a decommissioned block service
			if weHaveBs && isDecommissioned {
				l.ErrorNoAlert("We have block service %v, which is decommissioned according to registry. We will treat it as if it doesn't exist.", bs.Id)
				delete(blockServices, bs.Id)
				deadBlockServices[bs.Id] = deadBlockService{}
			}
			// fill in information from registry, if it's recent enough
			if weHaveBs && time.Since(bs.LastSeen.Time()) < maximumRegisterInterval*2 {
				// everything else is filled in by initBlockServicesInfo
				ourBs.cachedInfo = msgs.RegisterBlockServiceInfo{
					CapacityBytes:  bs.CapacityBytes,
					AvailableBytes: bs.AvailableBytes,
					Blocks:         bs.Blocks,
				}
			}
		}
	}

	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}

	listener1, err := lc.Listen(context.Background(), "tcp4", fmt.Sprintf("%v:%v", net.IP(ownIp1[:]), port1))

	if err != nil {
		panic(err)
	}
	defer listener1.Close()

	l.Info("running 1 on %v", listener1.Addr())
	actualPort1 := uint16(listener1.Addr().(*net.TCPAddr).Port)

	var listener2 net.Listener
	var actualPort2 uint16
	if len(addresses) == 2 {
		listener2, err = lc.Listen(context.Background(), "tcp4", fmt.Sprintf("%v:%v", net.IP(ownIp2[:]), port2))
		if err != nil {
			panic(err)
		}
		defer listener2.Close()

		l.Info("running 2 on %v", listener2.Addr())
		actualPort2 = uint16(listener2.Addr().(*net.TCPAddr).Port)
	}

	initBlockServicesInfo(env, l, msgs.Location(*locationId), msgs.AddrsInfo{Addr1: msgs.IpPort{Addrs: ownIp1, Port: actualPort1}, Addr2: msgs.IpPort{Addrs: ownIp2, Port: actualPort2}}, failureDomain, blockServices, *reservedStorage)
	l.Info("finished updating block service info, will now start")

	terminateChan := make(chan any)

	for bsId := range blockServices {
		env.stats[bsId] = &blockServiceStats{}
		env.eraseLocks[bsId] = &sync.Mutex{}
	}
	for bsId := range deadBlockServices {
		env.stats[bsId] = &blockServiceStats{}
		env.eraseLocks[bsId] = &sync.Mutex{}
	}
	env.counters = make(map[msgs.BlocksMessageKind]*timing.Timings)
	for _, k := range msgs.AllBlocksMessageKind {
		env.counters[k] = timing.NewTimings(40, 100*time.Microsecond, 1.5)
	}

	go func() {
		defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
		registerPeriodically(l, blockServices, env)
	}()

	go func() {
		defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
		updateBlockServiceInfoBlocksForever(l, blockServices)
	}()

	go func() {
		defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
		updateBlockServiceInfoCapacityForever(l, blockServices, *reservedStorage)
	}()

	if influxDB != nil {
		go func() {
			defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
			sendMetrics(l, env, influxDB, blockServices, *failureDomainStr)
		}()
	}

	go func() {
		defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
		raiseAlerts(l, env, blockServices)
	}()

	go func() {
		defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
		for {
			conn, err := listener1.Accept()
			l.Trace("new conn %+v", conn)
			if err != nil {
				terminateChan <- err
				return
			}
			go func() {
				defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
				handleRequest(l, env, terminateChan, blockServices, deadBlockServices, conn.(*net.TCPConn), *futureCutoff, *connectionTimeout)
			}()
		}
	}()
	if listener2 != nil {
		go func() {
			defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
			for {
				conn, err := listener2.Accept()
				l.Trace("new conn %+v", conn)
				if err != nil {
					terminateChan <- err
					return
				}
				go func() {
					defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
					handleRequest(l, env, terminateChan, blockServices, deadBlockServices, conn.(*net.TCPConn), *futureCutoff, *connectionTimeout)
				}()
			}
		}()
	}

	{
		err := <-terminateChan
		if err != nil {
			panic(err)
		}
	}
}

func eraseFileIfExistsAndSyncDir(path string) error {
	err := os.Remove(path)
	if err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func writeBufToTemp(statBytes *uint64, basePath string, buf []byte) (string, error) {
	if err := os.Mkdir(basePath, 0777); err != nil && !os.IsExist(err) {
		return "", err
	}
	f, err := os.CreateTemp(basePath, "tmp.")
	if err != nil {
		return "", err
	}
	tmpName := f.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpName)
		}
		f.Close()
	}()
	bufSize := len(buf)
	for len(buf) > 0 {
		n, err := f.Write(buf)
		if err != nil {
			return "", err
		}
		buf = buf[n:]
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	atomic.AddUint64(statBytes, uint64(bufSize))
	return tmpName, err
}

func verifyCrcFile(log *log.Logger, readBuffer []byte, path string, expectedSize int64, expectedCrc msgs.Crc) error {
	f, err := os.Open(path)
	if err != nil {
		log.Debug("failed opening file %s with error: %v", path, err)
		return err
	}
	defer f.Close()
	fs, err := f.Stat()
	if err != nil {
		log.Debug("failed stating file %s with error: %v", path, err)
		return err
	}
	if fs.Size() != expectedSize {
		log.Debug("failed file size when checking crc for file %s. expected size: %d size %d", path, expectedSize, fs.Size())
		return msgs.BAD_BLOCK_CRC
	}
	return verifyCrcReader(log, readBuffer, f, expectedCrc)
}

func verifyCrcReader(log *log.Logger, readBuffer []byte, r io.Reader, expectedCrc msgs.Crc) error {
	cursor := uint32(0)
	remainingData := 0
	actualCrc := uint32(0)
	processChunkSize := int(msgs.TERN_PAGE_WITH_CRC_SIZE)
	if len(readBuffer) < processChunkSize {
		readBuffer = make([]byte, processChunkSize)
	}
	readerHasMoreData := true
	for readerHasMoreData {
		read, err := r.Read(readBuffer[remainingData:])
		if err != nil {
			if err == io.EOF {
				err = nil
				readerHasMoreData = false
			} else {
				log.Debug("failed reading source while checking crc. error: %v", err)
				return err
			}
		}
		remainingData += read
		cursor += uint32(read)
		if remainingData < int(msgs.TERN_PAGE_WITH_CRC_SIZE) {
			continue
		}
		numAvailableChunks := remainingData / processChunkSize
		for i := 0; i < numAvailableChunks; i++ {
			actualPageCrc := crc32c.Sum(0, readBuffer[i*processChunkSize:i*processChunkSize+int(msgs.TERN_PAGE_SIZE)])
			storedPageCrc := binary.LittleEndian.Uint32(readBuffer[i*processChunkSize+int(msgs.TERN_PAGE_SIZE) : i*processChunkSize+int(msgs.TERN_PAGE_WITH_CRC_SIZE)])
			if storedPageCrc != actualPageCrc {
				log.Debug("failed checking crc. incorrect page crc at offset %d, expected %v, got %v", cursor-uint32(remainingData)+uint32(i*processChunkSize), msgs.Crc(storedPageCrc), msgs.Crc(actualPageCrc))
				return msgs.BAD_BLOCK_CRC
			}
			actualCrc = crc32c.Append(actualCrc, actualPageCrc, int(msgs.TERN_PAGE_SIZE))
		}
		copy(readBuffer[:], readBuffer[numAvailableChunks*processChunkSize:remainingData])
		remainingData -= numAvailableChunks * processChunkSize
	}
	if actualCrc != uint32(expectedCrc) {
		log.Debug("failed checking crc. invalid block crc. expected %v, got %v", expectedCrc, msgs.Crc(actualCrc))
		return msgs.BAD_BLOCK_CRC
	}
	if remainingData > 0 {
		log.Debug("failed checking crc. unexpected data left")
		return msgs.BAD_BLOCK_CRC
	}
	return nil
}

func moveFileAndSyncDir(src, dst string) error {
	err := os.Rename(src, dst)
	if err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(dst))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
