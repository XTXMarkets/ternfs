// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package timing

import (
	"math"
	"testing"
	"time"
	"xtx/ternfs/core/assert"
)

func TestTimingsBins(t *testing.T) {
	// use the same stuff we use in prod
	timings := NewTimings(40, 10*time.Microsecond, 1.5)
	// bins should be:
	// 0-10us           1.5^0
	// 10us-15us        1.5^1
	// 15us-22.5us
	// ...
	// 32762ms-49143ms  1.5^38
	// 49143ms-infty
	timings.Add(time.Microsecond)
	timings.Add(12 * time.Microsecond)
	timings.Add(40 * time.Second)
	timings.Add(100 * time.Second)

	totalCount := uint64(0)
	for i, bin := range timings.Histogram() {
		assert.True(t, i != 0 || bin.Count == 1, "timing in first bin")
		assert.True(t, i != 1 || bin.Count == 1, "timing in second bin")
		assert.True(t, i != 38 || bin.Count == 1, "timing in second to last bin")
		assert.True(t, i != 39 || bin.Count == 1, "timing in last bin")
		assert.True(t, (i < 2 || i >= 38) || bin.Count == 0, "nothing in other bins")
		totalCount += bin.Count
	}
}

func checkDurations(d1 time.Duration, d2 time.Duration) bool {
	return math.Abs(float64(d1-d2)/float64(d1+d2)) < 0.0001
}
