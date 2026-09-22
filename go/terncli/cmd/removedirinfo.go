// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func NewRemoveDirInfo() Command {
	removeDirInfoCmd := flag.NewFlagSet("remove-dir-info", flag.ExitOnError)
	removeDirInfoU64 := removeDirInfoCmd.Uint64("id", 0, "InodeId for the directory to unset the policy of.")
	removeDirInfoTag := removeDirInfoCmd.String("tag", "", "One of SNAPSHOT|SPAN|BLOCK")
	removeDirInfoRun := func(runtime *Runtime) {
		l := runtime.Log
		id := msgs.InodeId(*removeDirInfoU64)
		if err := runtime.getClient().RemoveDirectoryInfoEntry(l, id, msgs.DirInfoTagFromName(*removeDirInfoTag)); err != nil {
			panic(err)
		}
	}
	return Command{
		Flags: removeDirInfoCmd,
		Run:   removeDirInfoRun,
	}
}
