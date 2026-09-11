// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"github.com/XTXMarkets/ternfs/go/cleanup"
)

func NewScrub() Command {
	scrubCmd := flag.NewFlagSet("scrub", flag.ExitOnError)
	scrubRun := func(runtime *Runtime) {
		l := runtime.Log
		stats := cleanup.ScrubState{}
		if err := cleanup.ScrubFilesInAllShards(l, runtime.getClient(), &cleanup.ScrubOptions{NumWorkersPerShard: 10}, nil, &stats); err != nil {
			panic(err)
		}

	}
	return Command{
		Flags: scrubCmd,
		Run:   scrubRun,
	}
}
