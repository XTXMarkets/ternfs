// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package certificate

import (
	"bytes"
	"crypto/cipher"
	"encoding/binary"
	"xtx/ternfs/core/cbcmac"
	"xtx/ternfs/msgs"
)

func BlockWriteCertificate(cipher cipher.Block, blockServiceId msgs.BlockServiceId, req *msgs.WriteBlockReq) [8]byte {
	w := bytes.NewBuffer([]byte{})
	binary.Write(w, binary.LittleEndian, uint64(blockServiceId))
	w.Write([]byte{'w'})
	binary.Write(w, binary.LittleEndian, uint64(req.BlockId))
	binary.Write(w, binary.LittleEndian, uint32(req.Crc))
	binary.Write(w, binary.LittleEndian, uint32(req.Size))
	return cbcmac.CBCMAC(cipher, w.Bytes())
}

func CheckBlockWriteCertificate(cipher cipher.Block, blockServiceId msgs.BlockServiceId, req *msgs.WriteBlockReq) ([8]byte, bool) {
	expectedMac := BlockWriteCertificate(cipher, blockServiceId, req)
	return expectedMac, expectedMac == req.Certificate
}

func BlockEraseCertificate(blockServiceId msgs.BlockServiceId, blockId msgs.BlockId, key cipher.Block) [8]byte {
	buf := bytes.NewBuffer([]byte{})
	// struct.pack_into('<QcQ', b, 0, block['block_service_id'], b'e', block['block_id'])
	binary.Write(buf, binary.LittleEndian, uint64(blockServiceId))
	buf.Write([]byte{'e'})
	binary.Write(buf, binary.LittleEndian, uint64(blockId))

	return cbcmac.CBCMAC(key, buf.Bytes())
}

func CheckBlockEraseCertificate(blockServiceId msgs.BlockServiceId, cipher cipher.Block, req *msgs.EraseBlockReq) ([8]byte, bool) {
	expectedMac := BlockEraseCertificate(blockServiceId, req.BlockId, cipher)
	return expectedMac, expectedMac == req.Certificate
}

func BlockEraseProof(blockServiceId msgs.BlockServiceId, blockId msgs.BlockId, key cipher.Block) [8]byte {
	buf := bytes.NewBuffer([]byte{})
	// struct.pack_into('<QcQ', b, 0, block['block_service_id'], b'E', block['block_id'])
	binary.Write(buf, binary.LittleEndian, uint64(blockServiceId))
	buf.Write([]byte{'E'})
	binary.Write(buf, binary.LittleEndian, uint64(blockId))

	return cbcmac.CBCMAC(key, buf.Bytes())
}

func CheckBlockEraseProof(blockServiceId msgs.BlockServiceId, cipher cipher.Block, req *msgs.EraseBlockReq) ([8]byte, bool) {
	expectedMac := BlockEraseProof(blockServiceId, req.BlockId, cipher)
	return expectedMac, expectedMac == req.Certificate
}
