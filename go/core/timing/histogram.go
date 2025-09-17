// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package timing

import (
	"fmt"
	"math"
)

type Histogram struct {
	growth              float64 // the growth factor for each histogram bin, must be > 1
	invLogGrowth        float64 // 1/ln(factor)
	firstUpperBound     uint64  // the upper bound of the first bin of the histogram
	growthDivUpperBound float64 // growth/firstUpperBound
	bins                int
}

func NewHistogram(bins int, firstUpperBound uint64, growth float64) *Histogram {
	if bins < 1 {
		panic(fmt.Errorf("non-positive bins %d", bins))
	}
	if growth <= 1.0 {
		panic(fmt.Errorf("growth %v <= 1.0", growth))
	}
	return &Histogram{
		bins:                bins,
		growth:              growth,
		invLogGrowth:        1.0 / math.Log(growth),
		firstUpperBound:     firstUpperBound,
		growthDivUpperBound: growth / float64(firstUpperBound),
	}
}

func (t *Histogram) WhichBin(x uint64) int {
	// bin = floor(log_growth(t*growth/firstUpperBound))
	//     = floor(log(t*growthDivUpperBound) * invLogGrowth)
	bin := int(math.Log(float64(x)*t.growthDivUpperBound) * t.invLogGrowth)
	if bin < 0 {
		bin = 0
	}
	if bin >= t.bins {
		bin = t.bins - 1
	}
	return bin
}

// returns all the upper bounds
func (t *Histogram) Bins() []uint64 {
	bins := make([]uint64, t.bins)
	upperBound := float64(t.firstUpperBound)
	for i := 0; i < t.bins; i++ {
		bins[i] = uint64(upperBound)
		upperBound *= t.growth
	}
	return bins
}
