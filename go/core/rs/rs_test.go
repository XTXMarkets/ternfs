// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package rs

import (
	"math/rand"
	"sort"
	"testing"
	"xtx/ternfs/core/assert"
	"xtx/ternfs/core/parity"
)

func TestGet(t *testing.T) {
	parity := parity.MkParity(10, 4)
	rs := Get(parity)
	assert.Equal(t, parity, rs.Parity())
}

func TestComputeParity(t *testing.T) {
	rand := rand.New(rand.NewSource(0))
	maxBlockSize := 100
	buf := make([]byte, maxBlockSize*(16+16))
	for i := 0; i < 16*16*100; i++ {
		numData := int(2 + rand.Uint32()%(16-2))
		numParity := int(1 + rand.Uint32()%(16-1))
		blockSize := 1 + int(rand.Uint32()%100)
		data := buf[:blockSize*numData]
		blocks := make([][]byte, numData+numParity)
		for i := range blocks {
			blocks[i] = data[i*blockSize : (i+1)*blockSize]
		}
		rs := Get(parity.MkParity(uint8(numData), uint8(numParity)))
		rs.ComputeParityInto(blocks[:numData], blocks[numData:])
		// Verify that the first parity block is the XOR of all the data blocks
		expectedParity0 := make([]byte, blockSize)
		for i := 0; i < numData; i++ {
			for j := range blocks[i] {
				expectedParity0[j] ^= blocks[i][j]
			}
		}
		assert.EqualBytes(t, expectedParity0, blocks[numData])
		// Restore a random block
		{
			haveBlocks := make([]uint8, numData+numParity)
			for i := range haveBlocks {
				haveBlocks[i] = uint8(i)
			}
			for i := 0; i < numData+1; i++ {
				if i == numParity {
					break
				}
				j := i + 1 + int(rand.Uint32())%(len(haveBlocks)-i-1)
				haveBlocks[i], haveBlocks[j] = haveBlocks[j], haveBlocks[i]
			}
			wantBlock := haveBlocks[numData]
			haveBlocks = haveBlocks[:numData]
			sort.Slice(haveBlocks, func(i, j int) bool { return haveBlocks[i] < haveBlocks[j] })
			have := make([][]byte, numData)
			for i := range have {
				have[i] = blocks[haveBlocks[i]]
			}
			recoveredBlock := rs.Recover(haveBlocks, have, wantBlock)
			assert.EqualBytes(t, blocks[wantBlock], recoveredBlock)
		}
	}
}
