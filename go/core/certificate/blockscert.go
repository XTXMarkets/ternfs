// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package certificate

import (
	"crypto/cipher"
	"encoding/binary"
	"xtx/ternfs/core/cbcmac"
	"xtx/ternfs/msgs"
)

func BlockWriteCertificate(cipher cipher.Block, blockServiceId msgs.BlockServiceId, blockId msgs.BlockId, crc msgs.Crc, size uint32) [8]byte {
	var buf [25]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(blockServiceId))
	buf[8] = 'w'
	binary.LittleEndian.PutUint64(buf[9:17], uint64(blockId))
	binary.LittleEndian.PutUint32(buf[17:21], uint32(crc))
	binary.LittleEndian.PutUint32(buf[21:25], size)
	return cbcmac.CBCMAC(cipher, buf[:])
}

func CheckBlockWriteCertificate(cipher cipher.Block, blockServiceId msgs.BlockServiceId, req *msgs.WriteBlockReq) ([8]byte, bool) {
	expectedMac := BlockWriteCertificate(cipher, blockServiceId, req.BlockId, req.Crc, req.Size)
	return expectedMac, expectedMac == req.Certificate
}

func BlockWriteProof(cipher cipher.Block, blockServiceId msgs.BlockServiceId, blockId msgs.BlockId) [8]byte {
	var buf [17]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(blockServiceId))
	buf[8] = 'W'
	binary.LittleEndian.PutUint64(buf[9:17], uint64(blockId))
	return cbcmac.CBCMAC(cipher, buf[:])
}

func BlockEraseCertificate(blockServiceId msgs.BlockServiceId, blockId msgs.BlockId, key cipher.Block) [8]byte {
	var buf [17]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(blockServiceId))
	buf[8] = 'e'
	binary.LittleEndian.PutUint64(buf[9:17], uint64(blockId))
	return cbcmac.CBCMAC(key, buf[:])
}

func CheckBlockEraseCertificate(blockServiceId msgs.BlockServiceId, cipher cipher.Block, req *msgs.EraseBlockReq) ([8]byte, bool) {
	expectedMac := BlockEraseCertificate(blockServiceId, req.BlockId, cipher)
	return expectedMac, expectedMac == req.Certificate
}

func BlockEraseProof(blockServiceId msgs.BlockServiceId, blockId msgs.BlockId, key cipher.Block) [8]byte {
	var buf [17]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(blockServiceId))
	buf[8] = 'E'
	binary.LittleEndian.PutUint64(buf[9:17], uint64(blockId))
	return cbcmac.CBCMAC(key, buf[:])
}

func CheckBlockEraseProof(blockServiceId msgs.BlockServiceId, cipher cipher.Block, req *msgs.EraseBlockReq) ([8]byte, bool) {
	expectedMac := BlockEraseProof(blockServiceId, req.BlockId, cipher)
	return expectedMac, expectedMac == req.Certificate
}
