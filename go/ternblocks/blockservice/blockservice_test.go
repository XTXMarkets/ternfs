// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package blockservice

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xtx/ternfs/core/certificate"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"

	"xtx/ternfs/core/bufpool"
)

func testLogger() *log.Logger {
	return log.NewLogger(os.Stdout, &log.LoggerOptions{Level: log.DEBUG, Syslog: false, PrintQuietAlerts: true})
}

// createTestService creates a BlockService with a secret file on disk and waits for it to become active.
func createTestService(t *testing.T) *BlockService {
	t.Helper()
	tmpDir := t.TempDir()
	logger := testLogger()
	pool := bufpool.NewBufPool()

	secretPath := filepath.Join(tmpDir, SECRET_FILE_NAME)
	key := CreateSecret(logger, secretPath)
	if key == nil {
		t.Fatal("Failed to create secret")
	}

	id := BlockServiceIdFromKey(*key)
	bsInfo := msgs.RegisterBlockServiceInfo{
		Id:        id,
		Path:      "test:" + tmpDir,
		SecretKey: *key,
	}

	bs := OpenBlockService(logger, &BlockServiceOptions{
		BufferSize:   1024 * 1024,
		FutureCutoff: time.Minute,
		PastCutoff:   time.Minute,
		EraseCutoff:  time.Minute,
	}, pool, bsInfo, "dev1")

	deadline := time.Now().Add(5 * time.Second)
	for !bs.Active() {
		if time.Now().After(deadline) {
			bs.Close()
			t.Fatal("Timed out waiting for block service to become active")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return bs
}

// writeTestBlockDirect writes block data directly to disk (bypassing WriteBlock).
func writeTestBlockDirect(t *testing.T, bs *BlockService, id msgs.BlockId, data []byte) {
	t.Helper()
	blockPath := filepath.Join(bs.localPath, id.Path())
	if err := os.MkdirAll(filepath.Dir(blockPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blockPath, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestBlockServiceLifecycle(t *testing.T) {
	bs := createTestService(t)
	defer bs.Close()

	if bs.Id == msgs.BlockServiceId(0) {
		t.Error("Invalid service ID generated")
	}
	if !bs.Active() {
		t.Error("Service should be active")
	}
}

func TestWriteBlock(t *testing.T) {
	bs := createTestService(t)
	defer bs.Close()

	testData := make([]byte, 10*int(msgs.TERN_PAGE_SIZE))
	if _, err := rand.Read(testData); err != nil {
		t.Fatal(err)
	}

	t.Run("SuccessfulWrite", func(t *testing.T) {
		blockID := msgs.BlockId(msgs.Now())
		crc := msgs.Crc(crc32c.Sum(0, testData))
		size := uint32(len(testData))
		writeCert := certificate.BlockWriteCertificate(bs.cipher, bs.Id, blockID, crc, size)
		proof, err := bs.WriteBlock(blockID, writeCert, crc, size, bytes.NewReader(testData))
		if err != nil {
			t.Fatalf("WriteBlock failed: %v", err)
		}

		expectedProof := certificate.BlockWriteProof(bs.cipher, bs.Id, blockID)
		if proof != expectedProof {
			t.Errorf("Invalid write proof. Got %v, expected %v", proof, expectedProof)
		}

		blockPath := filepath.Join(bs.localPath, blockID.Path())
		if _, err := os.Stat(blockPath); err != nil {
			t.Errorf("Block file not created: %v", err)
		}
	})

	t.Run("BadBlockCRC", func(t *testing.T) {
		blockID := msgs.BlockId(msgs.Now())
		badCrc := msgs.Crc(0xDEADBEEF)
		size := uint32(len(testData))
		writeCert := certificate.BlockWriteCertificate(bs.cipher, bs.Id, blockID, badCrc, size)

		_, err := bs.WriteBlock(blockID, writeCert, badCrc, size, bytes.NewReader(testData))
		if !errors.Is(err, msgs.BAD_BLOCK_CRC) {
			t.Errorf("Expected BAD_BLOCK_CRC error, got: %v", err)
		}
	})

	t.Run("BadWriteCertificate", func(t *testing.T) {
		blockID := msgs.BlockId(msgs.Now())
		badCert := [8]byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE}
		_, err := bs.WriteBlock(blockID, badCert, msgs.Crc(0), 100, bytes.NewReader(make([]byte, 100)))
		if !errors.Is(err, msgs.BAD_CERTIFICATE) {
			t.Errorf("Expected BAD_CERTIFICATE error, got: %v", err)
		}
	})

	t.Run("BlockTooBig", func(t *testing.T) {
		blockID := msgs.BlockId(msgs.Now())
		crc := msgs.Crc(0)
		size := MAX_OBJECT_SIZE + 1
		writeCert := certificate.BlockWriteCertificate(bs.cipher, bs.Id, blockID, crc, size)
		_, err := bs.WriteBlock(blockID, writeCert, crc, size, bytes.NewReader(make([]byte, size)))
		if !errors.Is(err, msgs.BLOCK_TOO_BIG) {
			t.Errorf("Expected BLOCK_TOO_BIG error, got: %v", err)
		}
	})

	t.Run("BlockTooOldForWrite", func(t *testing.T) {
		blockID := msgs.BlockId(msgs.MakeTernTime(time.Now().Add(-2 * time.Hour)))
		crc := msgs.Crc(0)
		size := uint32(100)
		writeCert := certificate.BlockWriteCertificate(bs.cipher, bs.Id, blockID, crc, size)
		_, err := bs.WriteBlock(blockID, writeCert, crc, size, bytes.NewReader(make([]byte, size)))
		if !errors.Is(err, msgs.BLOCK_TOO_OLD_FOR_WRITE) {
			t.Errorf("Expected BLOCK_TOO_OLD_FOR_WRITE error, got: %v", err)
		}
	})

	t.Run("TempFileCleanupOnError", func(t *testing.T) {
		blockID := msgs.BlockId(msgs.Now())
		crc := msgs.Crc(crc32c.Sum(0, testData))
		size := uint32(len(testData))
		writeCert := certificate.BlockWriteCertificate(bs.cipher, bs.Id, blockID, crc, size)

		// Reader that returns no data — will cause CRC mismatch
		_, _ = bs.WriteBlock(blockID, writeCert, crc, size, io.LimitReader(bytes.NewReader(nil), 0))

		tmpFiles, _ := filepath.Glob(filepath.Join(bs.localPath, "tmp.*"))
		if len(tmpFiles) > 0 {
			t.Errorf("Temporary files not cleaned up: %v", tmpFiles)
		}
	})
}

func TestBlockOperations(t *testing.T) {
	bs := createTestService(t)
	defer bs.Close()

	testData := make([]byte, 10*int(msgs.TERN_PAGE_SIZE))
	if _, err := rand.Read(testData); err != nil {
		t.Fatal(err)
	}

	// Pre-compute block data with CRCs for direct writes
	var crcBuf bytes.Buffer
	writer := NewPageCrcWriter(&crcBuf)
	writer.Write(testData)
	blockData := crcBuf.Bytes()

	t.Run("WriteAndReadBlock", func(t *testing.T) {
		blockID := msgs.BlockId(12345)
		writeTestBlockDirect(t, bs, blockID, blockData)

		reader, err := bs.GetBlockReader(blockID, 0, uint32(len(testData)), true, true)
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()

		buf := make([]byte, len(testData))
		_, err = io.ReadFull(reader, buf)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(testData, buf) {
			t.Error("Read data doesn't match written data")
		}
	})

	t.Run("CheckBlock", func(t *testing.T) {
		blockID := msgs.BlockId(12345)
		writeTestBlockDirect(t, bs, blockID, blockData)

		err := bs.CheckBlock(blockID, uint32(len(testData)), msgs.Crc(crc32c.Sum(0, testData)))
		if err != nil {
			t.Errorf("Block check failed: %v", err)
		}
	})

	t.Run("CheckCorruptedBlock", func(t *testing.T) {
		blockID := msgs.BlockId(12345)
		writeTestBlockDirect(t, bs, blockID, blockData)

		corruptBlockPath := filepath.Join(bs.localPath, blockID.Path())
		data, err := os.ReadFile(corruptBlockPath)
		if err != nil {
			t.Fatal(err)
		}
		data[0] ^= 1
		if err := os.WriteFile(corruptBlockPath, data, 0644); err != nil {
			t.Fatal(err)
		}

		err = bs.CheckBlock(blockID, uint32(len(testData)), msgs.Crc(crc32c.Sum(0, testData)))
		if err == nil {
			t.Error("CheckBlock did not detect corrupted block")
		}
		if !errors.Is(err, msgs.BAD_BLOCK_CRC) {
			t.Errorf("CheckBlock returned unexpected error: %v", err)
		}
	})

	t.Run("EraseBlock", func(t *testing.T) {
		blockID := msgs.BlockId(12345)
		writeTestBlockDirect(t, bs, blockID, blockData)

		cert := certificate.BlockEraseCertificate(bs.Id, blockID, bs.cipher)
		proof, err := bs.EraseBlock(blockID, cert)
		if err != nil {
			t.Fatal(err)
		}

		if proof != certificate.BlockEraseProof(bs.Id, blockID, bs.cipher) {
			t.Errorf("EraseBlock returned incorrect proof: %v", proof)
		}

		_, err = os.Stat(filepath.Join(bs.localPath, blockID.Path()))
		if !os.IsNotExist(err) {
			t.Error("Block file not deleted")
		}
	})

	t.Run("EraseBlockInvalidCertificate", func(t *testing.T) {
		blockID := msgs.BlockId(12345)
		writeTestBlockDirect(t, bs, blockID, blockData)

		badBlockID := msgs.BlockId(99999)
		badCert := certificate.BlockEraseCertificate(bs.Id, badBlockID, bs.cipher)

		_, err := bs.EraseBlock(blockID, badCert)
		if err != msgs.BAD_CERTIFICATE {
			t.Errorf("Expected BAD_CERTIFICATE error, got: %v", err)
		}

		if _, err := os.Stat(filepath.Join(bs.localPath, blockID.Path())); err != nil {
			t.Error("Block should not have been deleted")
		}
	})

	t.Run("GetBlockFileForFetch", func(t *testing.T) {
		blockID := msgs.BlockId(12345)
		writeTestBlockDirect(t, bs, blockID, blockData)

		f, byteCount, err := bs.GetBlockFileForFetch(blockID, 0, uint32(len(testData)), false)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			f.Close()
			bs.ReleaseBlockFile()
		}()

		expectedByteCount := int64(len(blockData))
		if byteCount != expectedByteCount {
			t.Errorf("Expected byte count %d, got %d", expectedByteCount, byteCount)
		}

		buf := make([]byte, byteCount)
		_, err = io.ReadFull(f, buf)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(buf, blockData) {
			t.Error("File data doesn't match written data")
		}
	})
}

func TestCapacityTracking(t *testing.T) {
	bs := createTestService(t)
	defer bs.Close()

	t.Run("InitialCapacity", func(t *testing.T) {
		if !bs.updateCapacity() {
			t.Error("Failed to update capacity")
		}
		if bs.CapacityBytes == 0 {
			t.Error("Invalid capacity reporting")
		}
	})

	t.Run("BlockCount", func(t *testing.T) {
		testData := make([]byte, int(msgs.TERN_PAGE_SIZE))
		if _, err := rand.Read(testData); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		writer := NewPageCrcWriter(&buf)
		writer.Write(testData)
		blockData := buf.Bytes()

		const BLOCK_COUNT = 10
		for i := range BLOCK_COUNT {
			blockID := msgs.BlockId(12345 + i)
			writeTestBlockDirect(t, bs, blockID, blockData)
		}

		if !bs.countBlocks() {
			t.Error("Failed to count blocks")
		}
		if bs.Blocks != BLOCK_COUNT {
			t.Errorf("Incorrect block count: expected %d, got %d", BLOCK_COUNT, bs.Blocks)
		}
	})
}

func TestMigrateWithCrcDir(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()

	// Create with_crc directory with subdirectories
	withCrcPath := filepath.Join(tmpDir, "with_crc")
	os.MkdirAll(filepath.Join(withCrcPath, "00"), 0755)
	os.MkdirAll(filepath.Join(withCrcPath, "01"), 0755)
	os.WriteFile(filepath.Join(withCrcPath, "00", "testblock"), []byte("data"), 0644)

	migrateWithCrcDir(logger, tmpDir)

	// Verify subdirectories were moved up
	if _, err := os.Stat(filepath.Join(tmpDir, "00", "testblock")); err != nil {
		t.Errorf("Block not migrated: %v", err)
	}
	if _, err := os.Stat(withCrcPath); !os.IsNotExist(err) {
		t.Error("with_crc directory should have been removed")
	}
}
