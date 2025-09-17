// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// See wyhash.h, this used to be using FFI, but I had not
// realized that bits.Mul64 existed, and also that FFI is
// pretty slow in Go.
package wyhash

import (
	"math/bits"
	"unsafe"
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
	if len(p) == 0 {
		return 0, nil
	}
	bytes := uintptr(unsafe.Pointer(&p[0]))
	end := bytes + uintptr(len(p))
	unalignedEnd := (bytes - 1 + 8) & ^uintptr(7)
	for ; bytes < unalignedEnd; bytes++ {
		*(*uint8)(unsafe.Pointer(bytes)) = uint8(r.Uint64())
	}
	for ; bytes+8 <= end; bytes += 8 {
		*(*uint64)(unsafe.Pointer(bytes)) = r.Uint64()
	}
	for ; bytes < end; bytes++ {
		*(*uint8)(unsafe.Pointer(bytes)) = uint8(r.Uint64())
	}
	return len(p), nil
}
