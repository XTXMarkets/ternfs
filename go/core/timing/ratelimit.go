// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package timing

import (
	"fmt"
	"sync/atomic"
	"time"
)

type RateLimit struct {
	available uint64
	max       uint64
	wake      chan struct{}
	ticker    time.Ticker
	terminate chan struct{}
}

type RateLimitOpts struct {
	RefillInterval time.Duration
	BucketSize     uint64
	Refill         uint64
}

func NewRateLimit(opts0 *RateLimitOpts) *RateLimit {
	opts := *opts0
	if opts.BucketSize < 1 || opts.RefillInterval < 1 || opts.Refill < 1 {
		panic(fmt.Errorf("bad options: %+v", opts))
	}
	rl := &RateLimit{
		available: opts.Refill,
		max:       opts.BucketSize,
		wake:      make(chan struct{}),
		ticker:    *time.NewTicker(opts.RefillInterval),
		terminate: make(chan struct{}),
	}
	go func() {
		for {
			select {
			case <-rl.terminate:
				return
			case <-rl.ticker.C:
				for {
					avail := atomic.LoadUint64(&rl.available)
					newAvail := avail + opts.Refill
					if newAvail > rl.max {
						newAvail = rl.max
					}
					if atomic.CompareAndSwapUint64(&rl.available, avail, newAvail) {
						break
					}
				}
				select {
				case rl.wake <- struct{}{}:
				default:
				}
			}
		}
	}()
	return rl
}

func (rl *RateLimit) Close() {
	rl.ticker.Stop()
	rl.terminate <- struct{}{}
}

func (rl *RateLimit) Acquire() {
	// The below is racy (we might miss the wakeup), but we don't
	// care, we'll get the next wakeup. We don't care about pauses
	// so much here. We'd really like futex.
	for {
		for {
			avail := atomic.LoadUint64(&rl.available)
			if avail > 0 {
				if atomic.CompareAndSwapUint64(&rl.available, avail, avail-1) {
					return
				}
			} else {
				break
			}
		}
		// we need to wait for the thing to be replenished
		<-rl.wake
		// wake other waiters, if any
		select {
		case rl.wake <- struct{}{}:
		default:
		}
	}
}
