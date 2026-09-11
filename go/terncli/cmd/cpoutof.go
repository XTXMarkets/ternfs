// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"os"
)

func NewCpOutof() Command {
	cpOutofCmd := flag.NewFlagSet("cp-outof", flag.ExitOnError)
	cpOutofInput := cpOutofCmd.String("i", "", "What to copy from TernFS.")
	cpOutofId := cpOutofCmd.Uint64("id", 0, "The ID of the file to copy.")
	cpOutofOut := cpOutofCmd.String("o", "", "Where to write the file to. Stdout if empty.")
	cpOutofRun := func(runtime *Runtime) {
		l := runtime.Log
		out := os.Stdout
		if *cpOutofOut != "" {
			var err error

			os.Remove(*cpOutofOut)
			out, err = os.Create(*cpOutofOut)
			if err != nil {
				panic(err)
			}
		}
		var id msgs.InodeId
		if *cpOutofId != 0 && *cpOutofInput != "" {
			panic("Cannot specify both -i and -id")
		}
		if *cpOutofId != 0 {
			id = msgs.InodeId(*cpOutofId)
		} else {
			var err error
			id, err = runtime.getClient().ResolvePath(l, *cpOutofInput)
			if err != nil {
				panic(err)
			}
		}

		bufPool := bufpool.NewBufPool()
		r, err := runtime.getClient().FetchFile(l, bufPool, id)
		if err != nil {
			panic(err)
		}
		if _, err := out.Write(r.Bytes()); err != nil {
			panic(err)
		}
		out.Close()
	}
	return Command{
		Flags: cpOutofCmd,
		Run:   cpOutofRun,
	}
}
