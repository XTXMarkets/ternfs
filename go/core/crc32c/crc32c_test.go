// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package crc32c

import (
	"math/rand"
	"testing"
	"xtx/ternfs/core/assert"
)

func TestBasic(t *testing.T) {
	assert.Equal(t, uint32(0x6c0ec068), Sum(0, []byte("bazzer\n")))
}

func TestAppend(t *testing.T) {
	r := rand.New(rand.NewSource(0))

	for i := 0; i < 1000; i++ {
		s1 := make([]byte, r.Uint32()%100)
		rand.Read(s1)
		s2 := make([]byte, r.Uint32()%100)
		rand.Read(s2)
		assert.Equal(
			t,
			Sum(0, append(s1, s2...)),
			Append(Sum(0, s1), Sum(0, s2), len(s2)),
		)
	}
}

func TestXor(t *testing.T) {
	r := rand.New(rand.NewSource(0))

	for i := 0; i < 1000; i++ {
		l := r.Uint32() % 100
		s1 := make([]byte, l)
		rand.Read(s1)
		s2 := make([]byte, l)
		rand.Read(s2)
		xor := append([]byte{}, s1...)
		for i := range xor {
			xor[i] ^= s2[i]
		}
		assert.Equal(
			t,
			Sum(0, xor),
			Xor(Sum(0, s1), Sum(0, s2), int(l)),
		)
	}
}

func TestZeroExtend(t *testing.T) {
	r := rand.New(rand.NewSource(0))

	for i := 0; i < 1000; i++ {
		s := make([]byte, r.Uint32()%100)
		r.Read(s)
		zeros := make([]byte, r.Uint32()%100)
		assert.Equal(
			t,
			Sum(0, append(s, zeros...)),
			ZeroExtend(Sum(0, s), len(zeros)),
		)
	}
}

func TestZeroContract(t *testing.T) {
	r := rand.New(rand.NewSource(0))

	for i := 0; i < 1000; i++ {
		s := make([]byte, r.Uint32()%100)
		r.Read(s)
		zeros := make([]byte, r.Uint32()%100)
		assert.Equal(
			t,
			Sum(0, s),
			ZeroExtend(Sum(0, append(s, zeros...)), -len(zeros)),
		)
	}
}

func TestEmptySum(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	for i := 0; i < 100; i++ {
		s := make([]byte, r.Uint32()%100)
		r.Read(s)
		crc := Sum(0, s)
		assert.Equal(t, crc, Sum(crc, []byte{}))
	}
}
