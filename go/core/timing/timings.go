// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package timing

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"time"
)

type Timings struct {
	histo Histogram
	// Actual data
	startedAt time.Time
	bins      []uint64
}

func (t1 *Timings) Merge(t2 *Timings) {
	if !reflect.DeepEqual(t1.histo, t2.histo) {
		panic(fmt.Errorf("different histos, %+v vs %+v", t1.histo, t2.histo))
	}
	if t1.startedAt.After(t2.startedAt) {
		t1.startedAt = t2.startedAt
	}
	for i := 0; i < len(t1.bins); i++ {
		t1.bins[i] += t2.bins[i]
	}
}

type HistogramBin struct {
	UpperBound time.Duration
	Count      uint64
}

func (t *Timings) Histogram() []HistogramBin {
	bins := make([]HistogramBin, len(t.bins))
	for i, upperBound := range t.histo.Bins() {
		bins[i].Count = t.bins[i]
		bins[i].UpperBound = time.Duration(upperBound)
	}
	return bins
}

func NewTimings(bins int, firstUpperBound time.Duration, growth float64) *Timings {
	if firstUpperBound < 1 {
		panic(fmt.Errorf("non-positive first upper bound %d", firstUpperBound))
	}
	timings := Timings{
		bins:  make([]uint64, bins),
		histo: *NewHistogram(bins, uint64(firstUpperBound), growth),
	}
	timings.Reset()
	return &timings
}

func (t *Timings) Reset() {
	for i := range t.bins {
		atomic.StoreUint64(&t.bins[i], 0)
	}
	t.startedAt = time.Now()
}

func (t *Timings) Add(d time.Duration) {
	inanos := d.Nanoseconds()
	if inanos < 0 {
		return
	}
	atomic.AddUint64(&t.bins[t.histo.WhichBin(uint64(d))], 1)
}

// In these aggregates we're conservative (pick the upper bound)

func (t *Timings) TotalTime() time.Duration {
	d := time.Duration(0)
	for _, bin := range t.Histogram() {
		d += bin.UpperBound * time.Duration(bin.Count)
	}
	return d
}

func (t *Timings) Count() uint64 {
	x := uint64(0)
	for _, bin := range t.Histogram() {
		x += bin.Count
	}
	return x
}

func (t *Timings) Mean() time.Duration {
	bins := t.Histogram()
	totalCount := uint64(0)
	for _, bin := range bins {
		totalCount += bin.Count
	}
	x := float64(0)
	for _, bin := range bins {
		x += float64(bin.UpperBound) * (float64(bin.Count) / float64(totalCount))
	}
	return time.Duration(x)
}

func (t *Timings) P(target float64) time.Duration {
	bins := t.Histogram()
	totalCount := uint64(0)
	for _, bin := range bins {
		totalCount += bin.Count
	}
	p := float64(0)
	for _, bin := range bins {
		p += float64(bin.Count) / float64(totalCount)
		if p >= target {
			return bin.UpperBound
		}
	}
	panic("impossible")
}

func (t *Timings) Median() time.Duration {
	return t.P(0.5)
}
