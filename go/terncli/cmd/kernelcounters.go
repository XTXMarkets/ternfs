// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func NewKernelCounters() Command {
	kernelCountersCmd := flag.NewFlagSet("kernel-counters", flag.ExitOnError)
	kernelCountersRun := func(runtime *Runtime) {
		l := runtime.Log
		{
			header, err := parseKernelMetricsHeader("shard")
			if err != nil {
				panic(err)
			}
			counters, err := parseShardKernelCounters(header)
			if err != nil {
				panic(err)
			}
			for _, c := range counters {
				l.Info("%v: Success=%v Attempts=%v Timeouts=%v Failures=%v NetFailures=%v", msgs.ShardMessageKind(c.Kind), c.Success, c.Attempts, c.Timeouts, c.Failures, c.NetFailures)
			}
		}
		{
			header, err := parseKernelMetricsHeader("cdc")
			if err != nil {
				panic(err)
			}
			counters, err := parseCDCKernelCounters(header)
			if err != nil {
				panic(err)
			}
			for _, c := range counters {
				l.Info("%v: Success=%v Attempts=%v Timeouts=%v Failures=%v NetFailures=%v", msgs.CDCMessageKind(c.Kind), c.Success, c.Attempts, c.Timeouts, c.Failures, c.NetFailures)
			}
		}
	}
	return Command{
		Flags: kernelCountersCmd,
		Run:   kernelCountersRun,
	}
}
