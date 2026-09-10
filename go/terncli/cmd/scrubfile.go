// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"github.com/XTXMarkets/ternfs/go/cleanup"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func NewScrubFile() SpecWithClient {
	scrubFileCmd := flag.NewFlagSet("scrub-file", flag.ExitOnError)
	scrubFileId := scrubFileCmd.Uint64("id", 0, "The file to scrub")
	scrubFileRun := func(runtime RuntimeWithClient) {
		l := runtime.Log
		file := msgs.InodeId(*scrubFileId)
		stats := &cleanup.ScrubState{}
		if err := cleanup.ScrubFile(l, runtime.Client, stats, file); err != nil {
			panic(err)
		}
		l.Info("scrub stats: %+v", stats)
	}
	return SpecWithClient{
		Flags: scrubFileCmd,
		Run:   scrubFileRun,
	}
}
