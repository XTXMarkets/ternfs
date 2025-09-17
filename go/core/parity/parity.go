// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package parity

import (
	"encoding/json"
	"fmt"
)

type Parity uint8

func MkParity(dataBlocks uint8, parityBlocks uint8) Parity {
	if dataBlocks == 0 || dataBlocks >= 16 {
		panic(fmt.Errorf("bad data blocks %v", dataBlocks))
	}
	if parityBlocks >= 16 {
		panic(fmt.Errorf("bad parity blocks %v", parityBlocks))
	}
	if dataBlocks > 1 && parityBlocks == 0 {
		panic(fmt.Errorf("got %v data blocks but no parity blocks -- we do not support striping", dataBlocks))
	}
	return Parity(dataBlocks | (parityBlocks << 4))
}

func (parity Parity) DataBlocks() int {
	return int(parity) & 0x0F
}
func (parity Parity) ParityBlocks() int {
	return int(parity) >> 4
}
func (parity Parity) Blocks() int {
	return parity.DataBlocks() + parity.ParityBlocks()
}

func (parity Parity) String() string {
	return fmt.Sprintf("(%v,%v)", parity.DataBlocks(), parity.ParityBlocks())
}

func (p Parity) MarshalJSON() ([]byte, error) {
	return json.Marshal([]int{p.DataBlocks(), p.ParityBlocks()})
}

func (p *Parity) UnmarshalJSON(b []byte) error {
	var nums []uint8
	if err := json.Unmarshal(b, &nums); err != nil {
		return err
	}
	if len(nums) != 2 {
		return fmt.Errorf("expecting a list of 2 numbers for parity value, got %v", nums)
	}
	*p = MkParity(nums[0], nums[1])
	return nil
}
