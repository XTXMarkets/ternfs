// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package divide32

import "math/bits"

type Divisor struct {
	d uint32
	m uint64
}

func NewDivisor(divisor uint32) Divisor {
	return Divisor{d: divisor, m: (^uint64(0) / uint64(divisor)) + 1}
}

func (d *Divisor) Mod(quotient uint32) uint32 {
	x, _ := bits.Mul64(d.m*uint64(quotient), uint64(d.d))
	return uint32(x)
}

func (d *Divisor) Div(quotient uint32) uint32 {
	x, _ := bits.Mul64(d.m, uint64(quotient))
	return uint32(x)
}
