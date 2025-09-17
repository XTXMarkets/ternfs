// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package cbcmac

import (
	"crypto/cipher"
	"fmt"
)

// Intended to be used with AES-128
func CBCMAC(cipher cipher.Block, data []byte) [8]byte {
	if cipher.BlockSize() != 16 {
		panic(fmt.Errorf("expecting block size 16, got %d", cipher.BlockSize()))
	}
	block := [16]byte{}
	for i := 0; i < len(data); i += 16 {
		for j := 0; j < 16 && i+j < len(data); j++ {
			block[j] ^= data[i+j]
		}
		cipher.Encrypt(block[:], block[:])
	}
	var mac [8]byte
	copy(mac[:], block[:8])
	return mac
}
