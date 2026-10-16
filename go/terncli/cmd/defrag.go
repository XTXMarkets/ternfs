// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/cleanup"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"time"
)

func NewDefrag() Command {
	defragFileCmd := flag.NewFlagSet("defrag", flag.ExitOnError)
	defragFilePath := defragFileCmd.String("path", "", "The directory or file to defrag")
	defragFileFrom := defragFileCmd.String("from", "", "If present, will not defrag files pointed at by edges created before this time.")
	defragFileRun := func(runtime *Runtime) {
		l := runtime.Log
		c := runtime.getClient()
		dirInfoCache := client.NewDirInfoCache()
		bufPool := bufpool.NewBufPool()
		stats := &cleanup.DefragStats{}
		alert := l.NewNCAlert(0)
		alert.SetAppType(log.XMON_NEVER)
		id, _, parent, err := c.ResolvePathWithParent(l, *defragFilePath)
		if err != nil {
			panic(err)
		}
		if id.Type() == msgs.DIRECTORY {
			var startTime msgs.TernTime
			if *defragFileFrom != "" {
				t, err := time.Parse(time.RFC3339Nano, *defragFileFrom)
				if err != nil {
					panic(err)
				}
				startTime = msgs.MakeTernTime(t)
			}
			options := cleanup.DefragOptions{
				WorkersPerShard: 5,
				StartFrom:       startTime,
			}
			if err := cleanup.DefragFiles(l, c, bufPool, dirInfoCache, stats, alert, &options, *defragFilePath); err != nil {
				panic(err)
			}
		} else {
			if *defragFileFrom != "" {
				panic(fmt.Errorf("cannot provide -from with a file -path"))
			}
			if err := cleanup.DefragFile(l, c, bufPool, dirInfoCache, stats, alert, parent, id, *defragFilePath); err != nil {
				panic(err)
			}
		}
		l.Info("defrag stats: %+v", stats)
	}
	return Command{
		Flags: defragFileCmd,
		Run:   defragFileRun,
	}
}
