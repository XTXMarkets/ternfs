// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package blockservice

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/certificate"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"

	"golang.org/x/sys/unix"
)

type BlockServiceStats struct {
	BlocksWritten  uint64
	BytesWritten   uint64
	BlocksErased   uint64
	BlocksFetched  uint64
	BytesFetched   uint64
	BlocksChecked  uint64
	BytesChecked   uint64
	BlocksNotFound uint64
	IoErrors       uint64
	IoAttempts     uint64
}

type BlockServiceOptions struct {
	BufferSize            int
	MaxConcurrentWrites   int
	MaxConcurrentReads    int
	FutureCutoff          time.Duration
	PastCutoff            time.Duration
	ReservedCapacityBytes uint64
}

type BlockService struct {
	msgs.RegisterBlockServiceInfo
	BlockServiceStats
	DevId string

	options *BlockServiceOptions

	logger    *log.Logger
	bufPool   *bufpool.BufPool
	localPath string

	secretFile *os.File
	cipher     cipher.Block

	toDecommission atomic.Bool
	active         atomic.Bool

	stopC       chan struct{}
	eraseCheckC chan struct{}
	writeC      chan struct{}
	readC       chan struct{}
	wg          sync.WaitGroup
}

const SECRET_FILE_NAME = "secret.key"
const INFO_REFRESH_PERIOD = time.Minute * 5
const MAX_OBJECT_SIZE uint32 = 100 << 20 // 100MB

func OpenBlockService(logger *log.Logger, blockServiceOptions *BlockServiceOptions, bufPool *bufpool.BufPool, blockServiceInfo msgs.RegisterBlockServiceInfo, devId string) *BlockService {
	localPath := blockServiceInfo.Path
	if pathSegments := strings.Split(blockServiceInfo.Path, ":"); len(pathSegments) > 1 {
		localPath = pathSegments[1]
	}

	var writeC chan struct{}
	if blockServiceOptions.MaxConcurrentWrites > 0 {
		writeC = make(chan struct{}, blockServiceOptions.MaxConcurrentWrites)
	}

	var readC chan struct{}
	if blockServiceOptions.MaxConcurrentReads > 0 {
		readC = make(chan struct{}, blockServiceOptions.MaxConcurrentReads)
	}

	blockService := &BlockService{
		RegisterBlockServiceInfo: blockServiceInfo,
		DevId:                    devId,
		options:                  blockServiceOptions,
		logger:                   logger,
		bufPool:                  bufPool,
		localPath:                localPath,
		stopC:                    make(chan struct{}),
		eraseCheckC:              make(chan struct{}, 1),
		writeC:                   writeC,
		readC:                    readC,
	}

	cipherBlock, err := aes.NewCipher(blockService.SecretKey[:])
	if err != nil {
		logger.Warn("failed creating cipher for block service %v, err: %v", blockService.Id, err)
	} else {
		blockService.cipher = cipherBlock
	}

	migrateWithCrcDir(logger, localPath)
	blockService.startInfoUpdater()
	return blockService
}

func (b *BlockService) Close() {
	close(b.stopC)
	b.wg.Wait()
	if b.secretFile != nil {
		if err := syscall.Flock(int(b.secretFile.Fd()), syscall.LOCK_UN); err != nil {
			b.logger.Warn("failed unlocking secret file for block service %v, err: %v", b.Id, err)
		}
		if err := b.secretFile.Close(); err != nil {
			b.logger.Warn("failed closing secret file for block service %v, err: %v", b.Id, err)
		}
		b.secretFile = nil
	}
}

func (b *BlockService) ToDecommission() bool {
	return b.toDecommission.Load()
}

func (b *BlockService) Active() bool {
	return b.active.Load()
}

func (b *BlockService) Cipher() cipher.Block {
	return b.cipher
}

func (b *BlockService) LocalPath() string {
	return b.localPath
}

// GetBlockReader returns a reader that validates page CRCs and optionally strips them.
// Used for FetchBlock (strip CRC) and CheckBlock (verify CRC).
func (b *BlockService) GetBlockReader(blockId msgs.BlockId, offset uint32, count uint32, readAhead bool, stripCrc bool) (*blockReader, error) {
	if err := b.acquireWgIfActive(); err != nil {
		return nil, err
	}
	defer b.wg.Done()
	atomic.AddUint64(&b.BlocksFetched, 1)
	atomic.AddUint64(&b.BytesFetched, uint64(count))
	return newBlockReader(b, path.Join(b.localPath, blockId.Path()), offset, count, readAhead, stripCrc)
}

// GetBlockFileForFetch returns a raw file handle for zero-copy transfer via conn.ReadFrom.
// The file is seeked to the correct offset. Caller must close the file and call
// ReleaseBlockFile when done.
// Returns the file, the number of bytes to read, and any error.
func (b *BlockService) GetBlockFileForFetch(blockId msgs.BlockId, offset uint32, count uint32, readAhead bool) (*os.File, int64, error) {
	if err := b.acquireWgIfActive(); err != nil {
		return nil, 0, err
	}
	defer b.wg.Done()

	if offset%msgs.TERN_PAGE_SIZE != 0 {
		b.logger.RaiseAlert("trying to read from offset other than page boundary")
		return nil, 0, msgs.BLOCK_FETCH_OUT_OF_BOUNDS
	}
	if count%msgs.TERN_PAGE_SIZE != 0 {
		b.logger.RaiseAlert("trying to read count which is not a multiple of page size")
		return nil, 0, msgs.BLOCK_FETCH_OUT_OF_BOUNDS
	}

	pageCount := count / msgs.TERN_PAGE_SIZE
	offsetPageCount := offset / msgs.TERN_PAGE_SIZE
	blockPath := path.Join(b.localPath, blockId.Path())
	b.logger.Debug("fetching block id %v (%v -> %v) at path %v", blockId, offset, count, blockPath)

	f, err := os.Open(blockPath)
	if errors.Is(err, syscall.ENODATA) {
		b.logger.RaiseHardwareEvent(strings.Split(b.Path, ":")[0], b.Id.String(),
			fmt.Sprintf("could not open block %v, got ENODATA, this probably means that the block/disk is gone", blockPath))
		return nil, 0, syscall.EIO
	}
	if os.IsNotExist(err) {
		b.logger.ErrorNoAlert("could not find block to fetch at path %v", blockPath)
		return nil, 0, msgs.BLOCK_NOT_FOUND
	}
	if err != nil {
		return nil, 0, err
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	filePageCount := uint32(fi.Size()) / msgs.TERN_PAGE_WITH_CRC_SIZE
	if offsetPageCount+pageCount > filePageCount {
		f.Close()
		b.logger.RaiseAlert("malformed request for block %v. requested read at [%d - %d] but stored block size is %d",
			blockId, offset, offset+count, filePageCount*msgs.TERN_PAGE_SIZE)
		return nil, 0, msgs.BLOCK_FETCH_OUT_OF_BOUNDS
	}

	fileOffset := int64(offsetPageCount * msgs.TERN_PAGE_WITH_CRC_SIZE)
	byteCount := int64(pageCount * msgs.TERN_PAGE_WITH_CRC_SIZE)

	if readAhead {
		unix.Fadvise(int(f.Fd()), fileOffset, fi.Size(), unix.FADV_SEQUENTIAL|unix.FADV_WILLNEED)
	}

	if _, err := f.Seek(fileOffset, 0); err != nil {
		f.Close()
		return nil, 0, err
	}

	atomic.AddUint64(&b.BlocksFetched, 1)
	atomic.AddUint64(&b.BytesFetched, uint64(count))

	if b.readC != nil {
		b.readC <- struct{}{}
	}
	b.wg.Add(1)
	return f, byteCount, nil
}

// ReleaseBlockFile releases concurrency control acquired by GetBlockFileForFetch.
func (b *BlockService) ReleaseBlockFile() {
	if b.readC != nil {
		<-b.readC
	}
	b.wg.Done()
}

func (b *BlockService) TestWrite(size uint32, dataReader io.Reader) error {
	if err := b.acquireWgIfActive(); err != nil {
		return err
	}
	defer b.wg.Done()

	if size > MAX_OBJECT_SIZE {
		return msgs.BLOCK_TOO_BIG
	}

	if b.writeC != nil {
		b.writeC <- struct{}{}
		defer func() { <-b.writeC }()
	}

	atomic.AddUint64(&b.IoAttempts, 1)
	f, err := os.CreateTemp(b.localPath, "tmp.")
	if err != nil {
		atomic.AddUint64(&b.IoErrors, 1)
		return err
	}
	tmpName := f.Name()
	defer func() {
		f.Close()
		os.Remove(tmpName)
	}()

	w := NewPageCrcWriter(f)
	atomic.AddUint64(&b.IoAttempts, 1)
	buf := b.bufPool.Get(b.options.BufferSize)
	defer b.bufPool.Put(buf)
	written, err := io.CopyBuffer(w, io.LimitReader(dataReader, int64(size)), buf.Bytes())
	atomic.AddUint64(&b.BytesWritten, uint64(written))
	if err != nil {
		atomic.AddUint64(&b.IoErrors, 1)
		return err
	}
	if err = f.Sync(); err != nil {
		atomic.AddUint64(&b.IoErrors, 1)
		return err
	}
	return nil
}

func (b *BlockService) WriteBlock(blockId msgs.BlockId, cert [8]byte, expectedCrc msgs.Crc, size uint32, dataReader io.Reader) ([8]byte, error) {
	var proof [8]byte
	if err := b.acquireWgIfActive(); err != nil {
		return proof, err
	}
	defer b.wg.Done()
	filePath := path.Join(b.localPath, blockId.Path())
	b.logger.Debug("writing block %v at path %v", blockId, filePath)

	if size > MAX_OBJECT_SIZE {
		return proof, msgs.BLOCK_TOO_BIG
	}

	expectedCert := certificate.BlockWriteCertificate(b.cipher, b.Id, blockId, expectedCrc, size)
	if cert != expectedCert {
		b.logger.Warn("bad certificate for block %v: %v != %v", blockId, cert, expectedCert)
		return proof, msgs.BAD_CERTIFICATE
	}

	now := time.Now()
	pastCutoff := now.Add(-b.options.PastCutoff)
	futureCutoff := now.Add(b.options.FutureCutoff)
	blockTime := msgs.TernTime(uint64(blockId)).Time()

	if blockTime.Before(pastCutoff) {
		b.logger.Info("block %v too old for write: %v < %v", blockId, blockTime, pastCutoff)
		return proof, msgs.BLOCK_TOO_OLD_FOR_WRITE
	}

	if blockTime.After(futureCutoff) {
		panic(fmt.Errorf("block %v is in the future! (now=%v, futureCutoff=%v)", blockId, now, futureCutoff))
	}

	if b.writeC != nil {
		b.writeC <- struct{}{}
		defer func() { <-b.writeC }()
	}

	atomic.AddUint64(&b.IoAttempts, 1)
	if err := os.Mkdir(path.Dir(filePath), 0777); err != nil && !os.IsExist(err) {
		atomic.AddUint64(&b.IoErrors, 1)
		return proof, err
	}
	atomic.AddUint64(&b.IoAttempts, 1)
	f, err := os.CreateTemp(b.localPath, "tmp.")
	if err != nil {
		atomic.AddUint64(&b.IoErrors, 1)
		return proof, err
	}
	tmpName := f.Name()
	defer func() {
		if f != nil {
			f.Close()
		}
		if err != nil {
			os.Remove(tmpName)
		}
	}()

	w := NewPageCrcWriter(f)
	atomic.AddUint64(&b.IoAttempts, 1)
	written, err := io.CopyN(w, dataReader, int64(size))
	atomic.AddUint64(&b.BytesWritten, uint64(written))
	if err != nil {
		atomic.AddUint64(&b.IoErrors, 1)
		return proof, err
	}
	actualCrc, err := w.GetCrc()
	if err != nil {
		return proof, err
	}
	if msgs.Crc(actualCrc) != expectedCrc {
		err = msgs.BAD_BLOCK_CRC
		return proof, err
	}
	atomic.AddUint64(&b.IoAttempts, 1)
	if err = f.Sync(); err != nil {
		atomic.AddUint64(&b.IoErrors, 1)
		return proof, err
	}

	if err = f.Close(); err != nil {
		return proof, err
	}
	f = nil

	// Check again after write — file transfer may have taken a while
	now = time.Now()
	pastCutoff = now.Add(-b.options.PastCutoff)
	if blockTime.Before(pastCutoff) {
		b.logger.Info("block %v too old for write: %v < %v", blockId, blockTime, pastCutoff)
		return proof, msgs.BLOCK_TOO_OLD_FOR_WRITE
	}

	atomic.AddUint64(&b.IoAttempts, 1)
	err = moveFileAndSyncDir(tmpName, filePath)
	if err != nil {
		atomic.AddUint64(&b.IoErrors, 1)
		return proof, err
	}

	atomic.AddUint64(&b.BlocksWritten, 1)
	proof = certificate.BlockWriteProof(b.cipher, b.Id, blockId)
	return proof, nil
}

func (b *BlockService) CheckBlock(blockId msgs.BlockId, expectedSize uint32, crc msgs.Crc) error {
	if err := b.acquireWgIfActive(); err != nil {
		return err
	}
	defer b.wg.Done()

	b.eraseCheckC <- struct{}{}
	defer func() { <-b.eraseCheckC }()

	blockPath := path.Join(b.localPath, blockId.Path())
	b.logger.Debug("checking block %v at path %v", blockId, blockPath)

	atomic.AddUint64(&b.BlocksChecked, 1)
	atomic.AddUint64(&b.BytesChecked, uint64(expectedSize))

	f, err := newBlockReader(b, blockPath, 0, expectedSize, true, true)
	if err != nil {
		return err
	}
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	buf := b.bufPool.Get(b.options.BufferSize)
	defer b.bufPool.Put(buf)
	written, err := io.CopyBuffer(io.Discard, io.LimitReader(f, int64(expectedSize)), buf.Bytes())
	if err != nil && err != io.EOF {
		return err
	}

	if written != int64(expectedSize) {
		return msgs.BAD_BLOCK_CRC
	}

	if err = f.Close(); err != nil {
		f = nil
		return err
	}
	if readCrc, err := f.pageCrcReader.GetCrc(); err != nil || readCrc != uint32(crc) {
		return msgs.BAD_BLOCK_CRC
	}
	f = nil
	return nil
}

func (b *BlockService) EraseBlock(blockId msgs.BlockId, cert [8]byte) ([8]byte, error) {
	var proof [8]byte
	if err := b.acquireWgIfActive(); err != nil {
		return proof, err
	}
	defer b.wg.Done()

	expectedCert := certificate.BlockEraseCertificate(b.Id, blockId, b.cipher)
	if expectedCert != cert {
		b.logger.RaiseAlert("bad MAC, got %v, expected %v", cert, expectedCert)
		return proof, msgs.BAD_CERTIFICATE
	}

	now := time.Now()
	pastCutoff := now.Add(-b.options.PastCutoff)
	blockTime := msgs.TernTime(uint64(blockId)).Time()

	if blockTime.After(pastCutoff) {
		return proof, msgs.BLOCK_TOO_RECENT_FOR_DELETION
	}

	b.eraseCheckC <- struct{}{}
	defer func() { <-b.eraseCheckC }()
	atomic.AddUint64(&b.IoAttempts, 1)

	blockPath := path.Join(b.localPath, blockId.Path())
	b.logger.Debug("deleting block %v at path %v", blockId, blockPath)
	err := eraseFileIfExistsAndSyncDir(blockPath)
	if err != nil {
		atomic.AddUint64(&b.IoErrors, 1)
		return proof, err
	}
	atomic.AddUint64(&b.BlocksErased, 1)
	proof = certificate.BlockEraseProof(b.Id, blockId, b.cipher)
	return proof, nil
}

func (b *BlockService) acquireWgIfActive() error {
	b.wg.Add(1)
	if b.active.Load() {
		return nil
	}
	b.wg.Done()
	return msgs.BLOCK_SERVICE_NOT_FOUND
}

func (b *BlockService) startInfoUpdater() {
	b.wg.Add(1)
	go func(b *BlockService) {
		defer b.wg.Done()
		defer b.active.Store(false)
		ticker := time.NewTicker(INFO_REFRESH_PERIOD)
		for {
			succeed := b.checkSecret()
			succeed = succeed && b.updateCapacity()
			succeed = succeed && b.countBlocks()
			b.toDecommission.Store(!succeed)

			b.active.Store(b.secretFile != nil && b.cipher != nil)
			select {
			case <-b.stopC:
				return
			case <-ticker.C:
			}
		}
	}(b)
}

func (b *BlockService) checkSecret() bool {
	var err error
	secretFile := b.secretFile

	if b.secretFile == nil {
		keyFilePath := path.Join(b.localPath, SECRET_FILE_NAME)
		secretFile, err = os.Open(keyFilePath)
		if err != nil {
			b.logger.Warn("could not open secret file for block service %v, path: %v, err: %v", b.Id, keyFilePath, err)
			return false
		}
		if err := syscall.Flock(int(secretFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			b.logger.Warn("could not lock secret file for block service %v, path: %v, err: %v", b.Id, keyFilePath, err)
			return false
		}
	}
	defer func() {
		if b.secretFile != nil {
			return
		}
		if err := syscall.Flock(int(secretFile.Fd()), syscall.LOCK_UN); err != nil {
			b.logger.Warn("failed unlocking secret file for block service %v, err: %v", b.Id, err)
		}
		if err := secretFile.Close(); err != nil {
			b.logger.Warn("failed closing secret file for block service %v, err: %v", b.Id, err)
		}
	}()

	if _, err = secretFile.Seek(0, 0); err != nil {
		b.logger.Warn("could not seek secret file for block service %v err: %v", b.Id, err)
		return false
	}
	var key [16]byte
	var read int
	read, err = secretFile.Read(key[:])
	if err != nil {
		b.logger.Warn("could not read secret file for block service %v err: %v", b.Id, err)
		return false
	}
	if read != len(key) {
		b.logger.Warn("truncated secret file for block service %v length: %v", b.Id, read)
		return false
	}
	expectedKeyCrc := crc32c.Sum(0, key[:])
	var actualKeyCrc uint32
	if err := binary.Read(secretFile, binary.LittleEndian, &actualKeyCrc); err != nil {
		b.logger.Warn("could not read secret file for block service %v err: %v", b.Id, err)
		return false
	}
	if expectedKeyCrc != actualKeyCrc {
		b.logger.Warn("crc mismatch in secret file for block service %v", b.Id)
		return false
	}
	blockServiceId := BlockServiceIdFromKey(key)
	if blockServiceId != b.Id {
		b.logger.Warn("blockServiceId mismatch in secret file for block service %v blockService in file: %v", b.Id, blockServiceId)
		return false
	}
	for i := range key {
		if b.SecretKey[i] != key[i] {
			b.logger.Warn("key mismatch in secret file for block service %v", b.Id)
			return false
		}
	}
	b.secretFile = secretFile
	return true
}

func (b *BlockService) updateCapacity() bool {
	var statfs unix.Statfs_t
	if err := unix.Statfs(path.Join(b.localPath, SECRET_FILE_NAME), &statfs); err != nil {
		b.logger.Warn("could not update capacity for block service %v, err: %v", b.Id, err)
		return false
	}
	capacityBytes := statfs.Blocks * uint64(statfs.Bsize)
	availableBytes := statfs.Bavail * uint64(statfs.Bsize)

	capacityBytes -= min(capacityBytes, b.options.ReservedCapacityBytes)
	availableBytes -= min(availableBytes, b.options.ReservedCapacityBytes)

	atomic.StoreUint64(&b.CapacityBytes, capacityBytes)
	atomic.StoreUint64(&b.AvailableBytes, availableBytes)
	return true
}

func (b *BlockService) countBlocks() bool {
	blocks, err := CountBlocks(b.localPath)
	if err != nil {
		b.logger.Warn("could not count blocks for block service %v, err: %v", b.Id, err)
		return false
	}
	atomic.StoreUint64(&b.Blocks, blocks)
	return true
}

func BlockServiceIdFromKey(secretKey [16]byte) msgs.BlockServiceId {
	return msgs.BlockServiceId(binary.LittleEndian.Uint64(secretKey[:8]) & uint64(0x7FFFFFFFFFFFFFFF))
}

func CreateSecret(log *log.Logger, secretPath string) *[16]byte {
	var key [16]byte

	log.Info("creating new secret key at %s", secretPath)
	if _, err := crand.Read(key[:]); err != nil {
		log.Warn("failed creating secret at %s, err: %v", secretPath, err)
		return nil
	}

	var err error
	var keyFile *os.File
	if keyFile, err = os.OpenFile(secretPath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0600); err != nil {
		log.Warn("failed creating new secret file at %s, err: %v", secretPath, err)
		return nil
	}
	defer func() {
		if err := keyFile.Close(); err != nil {
			log.Warn("failed closing secret file at %s, err: %v", secretPath, err)
		}
	}()

	if _, err := keyFile.Seek(0, 0); err != nil {
		log.Warn("failed seeking in secret file at %s, err: %v", secretPath, err)
		return nil
	}
	if _, err := keyFile.Write(key[:]); err != nil {
		log.Warn("failed creating secret at %s, err: %v", secretPath, err)
		return nil
	}
	keyCrc := crc32c.Sum(0, key[:])
	if err := binary.Write(keyFile, binary.LittleEndian, keyCrc); err != nil {
		log.Warn("failed creating secret at %s, err: %v", secretPath, err)
		return nil
	}
	if err = keyFile.Sync(); err != nil {
		log.Warn("failed syncing secret at %s, err: %v", secretPath, err)
		return nil
	}
	dir, err := os.Open(filepath.Dir(secretPath))
	if err != nil {
		log.Warn("failed opening secret dir at %s, err: %v", secretPath, err)
		return nil
	}
	defer func() {
		if err := dir.Close(); err != nil {
			log.Warn("failed closing secret file dir at %s, err: %v", secretPath, err)
		}
	}()
	if err = dir.Sync(); err != nil {
		log.Warn("failed syncing secret dir at %s, err: %v", secretPath, err)
		return nil
	}

	return &key
}

// migrateWithCrcDir moves all subdirectories from with_crc/ up one level.
// This is a one-time migration for production data that stored blocks under with_crc/.
func migrateWithCrcDir(logger *log.Logger, localPath string) {
	withCrcPath := path.Join(localPath, "with_crc")
	entries, err := os.ReadDir(withCrcPath)
	if err != nil {
		return
	}

	logger.Info("migrating with_crc directory for %v", localPath)
	for _, entry := range entries {
		src := path.Join(withCrcPath, entry.Name())
		dst := path.Join(localPath, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			if os.IsExist(err) {
				continue
			}
			logger.Warn("failed migrating %v to %v: %v", src, dst, err)
			return
		}
	}
	if err := os.Remove(withCrcPath); err != nil {
		logger.Warn("failed removing empty with_crc directory %v: %v", withCrcPath, err)
	} else {
		logger.Info("with_crc migration complete for %v", localPath)
	}
}
