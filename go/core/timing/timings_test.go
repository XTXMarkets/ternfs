// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package timing

import (
	"github.com/XTXMarkets/ternfs/go/core/assert"
	"math"
	"testing"
	"time"
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

func timingsCount(bins []HistogramBin) uint64 {
	n := uint64(0)
	for _, bin := range bins {
		n += bin.Count
	}
	return n
}

// A snapshot has bins of its own: later adds to the original do not show in
// it, and merging into it does not change what was merged.
func TestTimingsSnapshotDetached(t *testing.T) {
	live := NewTimings(40, 10*time.Microsecond, 1.5)
	other := NewTimings(40, 10*time.Microsecond, 1.5)
	live.Add(time.Millisecond)
	other.Add(time.Second)
	snap := live.Snapshot()
	live.Add(time.Millisecond)
	snap.Merge(other)
	assert.Equal(t, timingsCount(snap.Histogram()), 2)  // its own count and the merged one
	assert.Equal(t, timingsCount(live.Histogram()), 2)  // its own adds only
	assert.Equal(t, timingsCount(other.Histogram()), 1) // the merge source is unchanged
}

// Histogram, Snapshot and Merge read bins that Add is updating; run under
// -race this checks the loads are atomic.
func TestTimingsReadsDuringAdd(t *testing.T) {
	live := NewTimings(40, 10*time.Microsecond, 1.5)
	const adds = 10000
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < adds; i++ {
			live.Add(time.Duration(i) * time.Microsecond)
		}
	}()
	for reading := true; reading; {
		select {
		case <-done:
			reading = false
		default:
		}
		snap := live.Snapshot()
		snap.Merge(live)
		_ = live.Histogram()
	}
	assert.Equal(t, timingsCount(live.Histogram()), adds)
}

func checkDurations(d1 time.Duration, d2 time.Duration) bool {
	return math.Abs(float64(d1-d2)/float64(d1+d2)) < 0.0001
}
