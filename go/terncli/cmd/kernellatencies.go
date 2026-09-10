// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"time"
)

func NewKernelLatencies() Spec {
	kernelLatenciesCmd := flag.NewFlagSet("kernel-latencies", flag.ExitOnError)
	kernelLatenciesRun := func(runtime Runtime) {
		l := runtime.Log
		p := func(mh *kernelMetricsHeader, l *kernelLatencies, target float64) time.Duration {
			totalCount := uint64(0)
			for _, bin := range l.LatencyBins {
				totalCount += bin
			}
			if totalCount == 0 {
				return 0
			}
			p := float64(0)
			for k := len(l.LatencyBins) - 1; k > 0; k-- {
				val := l.LatencyBins[k]
				p += float64(val) / float64(totalCount)
				if p >= target {
					return time.Duration(mh.UpperBoundValues[k])
				}
			}
			panic("impossible")
		}

		{
			header, err := parseKernelMetricsHeader("shard")
			if err != nil {
				panic(err)
			}
			latencies, err := parseShardKernelLatencies(header)
			if err != nil {
				panic(err)
			}
			for i := range latencies {
				l.Info("%v: p50=%v p90=%v p99=%v", msgs.ShardMessageKind(latencies[i].Kind), p(&header, &latencies[i], 0.5), p(&header, &latencies[i], 0.9), p(&header, &latencies[i], 0.99))
			}
		}
		{
			header, err := parseKernelMetricsHeader("cdc")
			if err != nil {
				panic(err)
			}
			latencies, err := parseCDCKernelLatencies(header)
			if err != nil {
				panic(err)
			}
			for i := range latencies {
				l.Info("%v: p50=%v p90=%v p99=%v", msgs.CDCMessageKind(latencies[i].Kind), p(&header, &latencies[i], 0.5), p(&header, &latencies[i], 0.9), p(&header, &latencies[i], 0.99))
			}
		}
	}
	return Spec{
		Flags: kernelLatenciesCmd,
		Run:   kernelLatenciesRun,
	}
}
