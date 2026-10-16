// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

func BenchmarkDiskIOAdmission(b *testing.B) {
	for _, serviceCount := range []int{30, 106} {
		b.Run(fmt.Sprintf("services=%d", serviceCount), func(b *testing.B) {
			opts := defaultDiskIOLimitOptions()
			opts.waitTimeout = 0
			// Avoid benchmarking overload rejection when several benchmark
			// goroutines happen to select the same serialized erase slot.
			opts.perService[diskErase] = 128
			ids := make([]msgs.BlockServiceId, serviceCount)
			for i := range ids {
				ids[i] = msgs.BlockServiceId(i + 1)
			}
			l := newDiskIOLimiter(opts, ids)
			b.Cleanup(l.close)
			kinds := [...]msgs.BlocksMessageKind{msgs.FETCH_BLOCK, msgs.WRITE_BLOCK, msgs.ERASE_BLOCK}
			var workers atomic.Uint64
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				service := int(workers.Add(1)-1) % serviceCount
				class := service % len(kinds)
				for pb.Next() {
					_, err := l.run(msgs.BlockServiceId(service+1), kinds[class], time.Time{}, func() bool { return true })
					if err != nil {
						b.Fatal(err)
					}
					service = (service + 1) % serviceCount
					class = (class + 1) % len(kinds)
				}
			})
		})
	}
}
