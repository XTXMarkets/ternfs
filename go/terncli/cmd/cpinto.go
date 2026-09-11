// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"io"
	"os"
	"path/filepath"
)

func NewCpInto() Command {
	cpIntoCmd := flag.NewFlagSet("cp-into", flag.ExitOnError)
	cpIntoInput := cpIntoCmd.String("i", "", "What to copy, if empty stdin.")
	cpIntoOut := cpIntoCmd.String("o", "", "Where to write the file to in TernFS")
	cpIntoRun := func(runtime *Runtime) {
		l := runtime.Log
		path := filepath.Clean("/" + *cpIntoOut)
		var input io.Reader
		if *cpIntoInput == "" {
			input = os.Stdin
		} else {
			var err error
			input, err = os.Open(*cpIntoInput)
			if err != nil {
				panic(err)
			}
		}
		bufPool := bufpool.NewBufPool()
		fileId, err := runtime.getClient().CreateFile(l, bufPool, client.NewDirInfoCache(), path, input)
		if err != nil {
			panic(err)
		}
		l.Info("File created as %v", fileId)
	}
	return Command{
		Flags: cpIntoCmd,
		Run:   cpIntoRun,
	}
}
