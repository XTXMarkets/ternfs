// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"os"
)

func NewUpdateBlockservicePath() Command {
	updateBlockservicePathCmd := flag.NewFlagSet("update-blockservice-path", flag.ExitOnError)
	updateBlockservicePathId := updateBlockservicePathCmd.Int64("id", 0, "Block service id")
	updateBlockserviceNewPath := updateBlockservicePathCmd.String("new-path", "", "New block service path")
	updateBlockservicePathRun := func(runtime *Runtime) {
		l := runtime.Log
		registryAddress := runtime.RegistryAddress
		if *updateBlockservicePathId == 0 {
			fmt.Fprintf(os.Stderr, "must provide -id\n")
			os.Exit(2)
		}
		if *updateBlockserviceNewPath == "" {
			fmt.Fprintf(os.Stderr, "must provide -new-path\n")
			os.Exit(2)
		}
		bsId := msgs.BlockServiceId(*updateBlockservicePathId)
		l.Info("setting path to %s for block service %v", *updateBlockserviceNewPath, bsId)
		_, err := client.RegistryRequest(l, nil, *registryAddress, &msgs.UpdateBlockServicePathReq{
			Id:      bsId,
			NewPath: *updateBlockserviceNewPath,
		})
		if err != nil {
			panic(err)
		}
	}
	return Command{
		Flags: updateBlockservicePathCmd,
		Run:   updateBlockservicePathRun,
	}
}
