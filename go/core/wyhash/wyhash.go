// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// See wyhash.h, this used to be using FFI, but I had not
// realized that bits.Mul64 existed, and also that FFI is
// pretty slow in Go.
package wyhash

import (
	"encoding/binary"
	"math/bits"
)

type Rand struct {
	State uint64
}

func New(seed uint64) *Rand {
	rand := Rand{State: seed}
	return &rand
}

func (r *Rand) Uint64() uint64 {
	r.State += 0x60bee2bee120fc15
	tmpHi, tmpLo := bits.Mul64(r.State, 0xa3b195354a39b70d)
	m1 := tmpHi ^ tmpLo
	tmpHi, tmpLo = bits.Mul64(m1, 0x1b03738712fad5c9)
	m2 := tmpHi ^ tmpLo
	return m2
}

func (r *Rand) Uint32() uint32 {
	return uint32(r.Uint64())
}

func (r *Rand) Float64() float64 {
	return float64(r.Uint64()&((1<<53)-1)) / float64(uint64(1<<53))
}

func (r *Rand) Read(p []byte) (int, error) {
	n := len(p)
	for len(p) >= 8 {
		binary.LittleEndian.PutUint64(p, r.Uint64())
		p = p[8:]
	}
	for i := range p {
		p[i] = uint8(r.Uint64())
	}
	return n, nil
}
