// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package timing

import (
	"sync"
	"testing"
	"time"
)

func TestSharedReqTimeouts(t *testing.T) {
	timeouts := NewReqTimeouts(time.Millisecond, time.Second, time.Minute, 1.5, 0.1)
	start := time.Now()
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			for i := range 1000 {
				next := timeouts.NextNow(start, start.Add(time.Duration(i)*time.Millisecond))
				if next <= 0 || next > timeouts.Max {
					t.Errorf("timeout = %v, want positive and at most %v", next, timeouts.Max)
				}
			}
		})
	}
	workers.Wait()
}
