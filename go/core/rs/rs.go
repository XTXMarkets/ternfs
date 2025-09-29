// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package rs

import (
	"fmt"
	"unsafe"
	"xtx/ternfs/core/parity"
)

// #cgo LDFLAGS: -L${SRCDIR}/../../../cpp/build/alpine/rs -lrs
// #include "../../../cpp/rs/rs.h"
//
// void set_ptr(uint8_t** ptrs, size_t i, uint8_t* ptr) {
//     ptrs[i] = ptr;
// }
import "C"

type Rs struct {
	r *C.struct_rs
}

func Get(parity parity.Parity) *Rs {
	if parity.DataBlocks() < 2 {
		panic(fmt.Errorf("bad parity, expected at least 2 data blocks, got %v", parity))
	}
	r := &Rs{}
	r.r = C.rs_get(C.uchar(parity))
	return r
}

func (r *Rs) Parity() parity.Parity {
	return parity.Parity(C.rs_parity(r.r))
}

func (r *Rs) ComputeParity(data [][]byte) [][]byte {
	parity := make([][]byte, r.Parity().ParityBlocks())
	blockSize := len(data[0])
	for i := range parity {
		parity[i] = make([]byte, blockSize)
	}
	r.ComputeParityInto(data, parity)
	return parity
}

// Returns block size
func (r *Rs) checkBlockSize(data [][]byte, parity [][]byte) int {
	if len(data) != r.Parity().DataBlocks() {
		panic(fmt.Errorf("bad number of data blocks, expected %v, got %v", r.Parity().DataBlocks(), len(data)))
	}
	blockSize := len(data[0])
	for i := range data {
		if len(data[i]) != blockSize {
			panic(fmt.Errorf("differing data block size, expected %v, got %v in block %v", blockSize, len(data[i]), i))
		}
	}
	if parity != nil {
		if len(parity) != r.Parity().ParityBlocks() {
			panic(fmt.Errorf("bad number of parity blocks, expected %v, got %v", r.Parity().ParityBlocks(), len(parity)))
		}
		for i := range parity {
			if len(parity[i]) != blockSize {
				panic(fmt.Errorf("differing parity block size, expected %v, got %v in block %v", blockSize, len(parity[i]), i))
			}
		}
	}
	return blockSize
}

func (r *Rs) ComputeParityInto(data [][]byte, parity [][]byte) {
	blockSize := r.checkBlockSize(data, parity)
	// prepare C structs
	dataPtrs := (**C.uchar)(C.malloc(C.size_t(uintptr(r.Parity().DataBlocks()) * unsafe.Sizeof((*C.uchar)(nil)))))
	for i := range data {
		C.set_ptr(dataPtrs, C.ulong(i), (*C.uchar)(&data[i][0]))
	}
	parityPtrs := (**C.uchar)(C.malloc(C.size_t(uintptr(r.Parity().ParityBlocks()) * unsafe.Sizeof((*C.uchar)(nil)))))
	for i := range parity {
		C.set_ptr(parityPtrs, C.ulong(i), (*C.uchar)(&parity[i][0]))
	}
	// go for it
	C.rs_compute_parity(r.r, C.ulong(blockSize), dataPtrs, parityPtrs)
}

func (r *Rs) Recover(
	haveBlocks uint32,
	blocks [][]byte,
	wantBlock uint8,
) []byte {
	block := make([]byte, len(blocks[0]))
	r.RecoverInto(haveBlocks, blocks, wantBlock, block)
	return block
}

func (r *Rs) RecoverInto(
	haveBlocks uint32, // bitmap
	blocks [][]byte,
	wantBlock uint8,
	block []byte,
) {
	blockSize := r.checkBlockSize(blocks, nil)
	if len(block) != blockSize {
		panic(fmt.Errorf("differing block size, expected %v, got %v", blockSize, len(block)))
	}
	for blockIx := range uint8(32) {
		if haveBlocks&(uint32(1)<<blockIx) == 0 {
			continue
		}
		if int(blockIx) >= r.Parity().Blocks() {
			panic(fmt.Errorf("blockIx=%d >= %d", blockIx, r.Parity().Blocks()))
		}
		if blockIx == wantBlock {
			panic(fmt.Errorf("blockIx=%d == wantBlock=%d", blockIx, wantBlock))
		}
	}
	blocksPtrs := (**C.uchar)(C.malloc(C.size_t(uintptr(r.Parity().DataBlocks()) * unsafe.Sizeof((*C.uchar)(nil)))))
	for i := range blocks {
		C.set_ptr(blocksPtrs, C.ulong(i), (*C.uchar)(&blocks[i][0]))
	}
	C.rs_recover(r.r, C.ulong(blockSize), C.uint(haveBlocks), blocksPtrs, C.uint(1)<<wantBlock, (*C.uchar)(&block[0]))
}
