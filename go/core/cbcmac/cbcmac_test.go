// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package cbcmac

import (
	"crypto/aes"
	"testing"
	"xtx/ternfs/core/assert"
)

// Sanity check to ensure that the block cipher we're using is the correct one
func TestAES(t *testing.T) {
	k := []byte("\x2b\x7e\x15\x16\x28\xae\xd2\xa6\xab\xf7\x15\x88\x09\xcf\x4f\x3c")
	cipher, err := aes.NewCipher(k)
	if err != nil {
		panic(err)
	}
	plaintext := []byte("\x32\x43\xf6\xa8\x88\x5a\x30\x8d\x31\x31\x98\xa2\xe0\x37\x07\x34")
	ciphertext := make([]byte, 16)
	cipher.Encrypt(ciphertext, plaintext)
	assert.Equal(t, "\x39\x25\x84\x1d\x02\xdc\x09\xfb\xdc\x11\x85\x97\x19\x6a\x0b\x32", string(ciphertext))
}

// Just a sanity check vs python
func TestMAC(t *testing.T) {
	k := []byte("\x2b\x7e\x15\x16\x28\xae\xd2\xa6\xab\xf7\x15\x88\x09\xcf\x4f\x3c")
	cipher, err := aes.NewCipher(k)
	if err != nil {
		panic(err)
	}
	plaintext := []byte("\x32\x43\xf6\xa8\x88\x5a\x30\x8d\x31\x31\x98\xa2\xe0\x37\x07\x34")
	mac := CBCMAC(cipher, plaintext)
	assert.Equal(t, "9%\x84\x1d\x02\xdc\t\xfb", string(mac[:]))
}
