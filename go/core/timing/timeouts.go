// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package timing

import (
	"fmt"
	"time"
	"xtx/ternfs/core/wyhash"
)

type ReqTimeouts struct {
	Initial time.Duration // the first timeout
	Max     time.Duration // the max timeout -- 0 for never
	Overall time.Duration // the max overall waiting time for a request
	Growth  float64
	Jitter  float64 // in percentage, e.g. 0.10 for a 10% jitter
	rand    wyhash.Rand
}

func NewReqTimeouts(initial time.Duration, max time.Duration, overall time.Duration, growth float64, jitter float64) *ReqTimeouts {
	if initial <= 0 {
		panic(fmt.Errorf("initial (%v) <= 0", initial))
	}
	if max <= 0 {
		panic(fmt.Errorf("max (%v) <= 0", max))
	}
	if overall < 0 {
		panic(fmt.Errorf("overall (%v) < 0", overall))
	}
	if growth <= 1.0 {
		panic(fmt.Errorf("growth (%v) <= 1.0", growth))
	}
	if jitter < 0.0 {
		panic(fmt.Errorf("jitter (%v) < 0.0", jitter))
	}
	return &ReqTimeouts{
		Initial: initial,
		Max:     max,
		Overall: overall,
		Growth:  growth,
		Jitter:  jitter,
		rand:    *wyhash.New(0),
	}
}

// If 0, time's up.
func (r *ReqTimeouts) NextNow(startedAt time.Time, now time.Time) time.Duration {
	elapsed := now.Sub(startedAt)
	if r.Overall > 0 && elapsed >= r.Overall {
		return time.Duration(0)
	}
	g := r.Growth + r.Growth*r.Jitter*(r.rand.Float64()-0.5)
	timeout := r.Initial + startedAt.Add(time.Duration(float64(elapsed/1000000)*g)*1000000).Sub(now) // compute in milliseconds to avoid inf
	if timeout > r.Max {
		timeout = r.Max
	}
	return timeout
}

func (r *ReqTimeouts) Next(startedAt time.Time) time.Duration {
	return r.NextNow(startedAt, time.Now())
}

// useful to fail immediately, so we can fall back to other block services
var NoTimeouts = ReqTimeouts{
	Overall: time.Nanosecond,
}
