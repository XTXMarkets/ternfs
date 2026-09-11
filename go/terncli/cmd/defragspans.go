// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"github.com/XTXMarkets/ternfs/go/cleanup"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/log"
)

func NewDefragSpans() Command {
	defragSpansCmd := flag.NewFlagSet("defrag-spans", flag.ExitOnError)
	defragSpansPath := defragSpansCmd.String("path", "", "The directory or file to defrag")
	defragSpansRun := func(runtime *Runtime) {
		l := runtime.Log
		c := runtime.Client()
		dirInfoCache := client.NewDirInfoCache()
		bufPool := bufpool.NewBufPool()
		stats := &cleanup.DefragSpansStats{}
		alert := l.NewNCAlert(0)
		alert.SetAppType(log.XMON_NEVER)
		if err := cleanup.DefragSpans(l, c, bufPool, dirInfoCache, stats, alert, *defragSpansPath); err != nil {
			panic(err)
		}
		l.Info("defrag stats: %+v", stats)
	}
	return Command{
		Flags: defragSpansCmd,
		Run:   defragSpansRun,
	}
}
