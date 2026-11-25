// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

// Merging per-shard counters builds a new total and leaves the shards alone.
// It used to start from a copy of shard 0 that shared its bins, so every
// merge added the other shards into shard 0 again: one request on shard 7
// was reported as 1, 2, 3 on successive merges.
func TestMergeReqCountersLeavesShardsUnchanged(t *testing.T) {
	counters := NewClientCounters()
	shards := counters.Shard[uint8(msgs.LOOKUP)]
	shards[7].Timings.Add(200 * time.Microsecond)
	shards[7].Attempts++
	for i := 0; i < 3; i++ {
		merged := MergeReqCounters(shards[:])
		if merged.Timings.Count() != 1 || merged.Attempts != 1 {
			t.Fatalf("merge %d: count %d attempts %d, want 1 and 1", i, merged.Timings.Count(), merged.Attempts)
		}
	}
	if shards[0].Timings.Count() != 0 || shards[0].Attempts != 0 {
		t.Fatalf("shard 0 changed: count %d attempts %d", shards[0].Timings.Count(), shards[0].Attempts)
	}
	if shards[7].Timings.Count() != 1 {
		t.Fatalf("shard 7 count %d", shards[7].Timings.Count())
	}
}

// Merging reads counters that requests are updating, as the client does
// with atomic adds; run under -race this checks the reads are atomic.
func TestMergeReqCountersDuringRequests(t *testing.T) {
	counters := NewClientCounters()
	shards := counters.Shard[uint8(msgs.LOOKUP)]
	const requests = 10000
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < requests; i++ {
			c := &shards[i%len(shards)]
			atomic.AddUint64(&c.Attempts, 1)
			c.Timings.Add(time.Duration(i) * time.Microsecond)
		}
	}()
	for merging := true; merging; {
		select {
		case <-done:
			merging = false
		default:
		}
		MergeReqCounters(shards[:])
	}
	merged := MergeReqCounters(shards[:])
	if merged.Timings.Count() != requests || merged.Attempts != requests {
		t.Fatalf("count %d attempts %d, want %d", merged.Timings.Count(), merged.Attempts, requests)
	}
}
