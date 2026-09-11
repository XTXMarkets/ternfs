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
	"strings"
)

func NewBlockserviceFlags() Command {
	blockserviceFlagsCmd := flag.NewFlagSet("blockservice-flags", flag.ExitOnError)
	blockserviceFlagsId := blockserviceFlagsCmd.Int64("id", 0, "Block service id")
	blockserviceFlagsFailureDomain := blockserviceFlagsCmd.String("failure-domain", "", "Failure domain -- if this is used all block services in a given failure domain will be affected.")
	blockserviceFlagsPathPrefix := blockserviceFlagsCmd.String("path-prefix", "", "Path prefix -- if this is used all block services with a given path prefix will be affected.")
	blockserviceFlagsSet := blockserviceFlagsCmd.String("set", "", "Flag to set")
	blockserviceFlagsUnset := blockserviceFlagsCmd.String("unset", "", "Flag to unset")
	blockserviceFlagsRun := func(runtime *Runtime) {
		l := runtime.Log
		registryAddress := runtime.RegistryAddress
		var flagsToSet, flagsToClear msgs.BlockServiceFlags
		var err error
		if *blockserviceFlagsSet != "" {
			flagsToSet, err = msgs.BlockServiceFlagsFromUnion(*blockserviceFlagsSet)
			if err != nil {
				panic(err)
			}
		}
		if *blockserviceFlagsUnset != "" {
			flagsToClear, err = msgs.BlockServiceFlagsFromUnion(*blockserviceFlagsUnset)
			if err != nil {
				panic(err)
			}
		}
		if flagsToClear&msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED != msgs.TERNFS_BLOCK_SERVICE_EMPTY {
			fmt.Fprintf(os.Stderr, "cannot unset DECOMMISSIONED flag\n")
			os.Exit(2)
		}
		if flagsToSet&flagsToClear != msgs.TERNFS_BLOCK_SERVICE_EMPTY {
			fmt.Fprintf(os.Stderr, "there can not be intersection between -set and -unset\n")
			os.Exit(2)
		}
		flag := flagsToSet
		mask := flagsToSet | flagsToClear
		filterCount := 0
		if *blockserviceFlagsId != 0 {
			filterCount++
		}
		if *blockserviceFlagsFailureDomain != "" {
			filterCount++
		}
		if *blockserviceFlagsPathPrefix != "" {
			filterCount++
		}
		if filterCount != 1 {
			fmt.Fprintf(os.Stderr, "must provide exactly one of -id, -failure-domain, or -path-prefix\n")
			os.Exit(2)
		}
		blockServiceIds := []msgs.BlockServiceId{}
		if *blockserviceFlagsId != 0 {
			blockServiceIds = append(blockServiceIds, msgs.BlockServiceId(*blockserviceFlagsId))
		}
		if *blockserviceFlagsFailureDomain != "" || *blockserviceFlagsPathPrefix != "" {
			l.Info("requesting block services")
			blockServicesResp, err := client.RegistryRequest(l, nil, *registryAddress, &msgs.ChangedBlockServicesReq{})
			if err != nil {
				panic(err)
			}
			blockServices := blockServicesResp.(*msgs.ChangedBlockServicesResp)
			for _, bs := range blockServices.BlockServices {
				if bs.Flags&msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED != 0 {
					continue
				}
				if bs.FailureDomain.String() == *blockserviceFlagsFailureDomain {
					blockServiceIds = append(blockServiceIds, bs.Id)
				}
				if strings.Split(bs.Path, ":")[0] == *blockserviceFlagsPathPrefix {
					blockServiceIds = append(blockServiceIds, bs.Id)
				}
			}
			if len(blockServiceIds) == 0 {
				if *blockserviceFlagsPathPrefix != "" {
					panic(fmt.Errorf("could not get any block service ids for path prefix %v", blockserviceFlagsPathPrefix))
				} else if *blockserviceFlagsFailureDomain != "" {
					panic(fmt.Errorf("could not get any block service ids for failure domain %v or ", blockserviceFlagsFailureDomain))
				}
			}
		}
		conn := client.MakeRegistryConn(l, nil, *registryAddress, 1)
		defer conn.Close()
		for _, bsId := range blockServiceIds {
			l.Info("setting flags %v with mask %v for block service %v", flag, mask, bsId)
			_, err := conn.Request(&msgs.SetBlockServiceFlagsReq{
				Id:        bsId,
				Flags:     flag,
				FlagsMask: uint8(mask),
			})
			if err != nil {
				panic(err)
			}
		}
	}
	return Command{
		Flags: blockserviceFlagsCmd,
		Run:   blockserviceFlagsRun,
	}
}
