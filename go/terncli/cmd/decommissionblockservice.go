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

func NewDecommissionBlockservice() Command {
	decommissionBlockserviceCmd := flag.NewFlagSet("decommission-blockservice", flag.ExitOnError)
	decommissionBlockserviceId := decommissionBlockserviceCmd.Int64("id", 0, "Block service id")
	decommissionBlockserviceForce := decommissionBlockserviceCmd.Bool("force", false, "If the rate-limited decommission endpoint refuses (AUTO_DECOMMISSION_FORBIDDEN or AUTO_DECOMMISSION_RATE_LIMITED), bypass it by directly setting the DECOMMISSIONED flag via SetBlockServiceFlags.")
	decommissionBlockserviceRun := func(runtime *Runtime) {
		l := runtime.Log
		registryAddress := runtime.RegistryAddress
		if *decommissionBlockserviceId == 0 {
			fmt.Fprintf(os.Stderr, "must provide -id\n")
			os.Exit(2)
		}
		bsId := msgs.BlockServiceId(*decommissionBlockserviceId)
		l.Info("decommissioning block service %v using dedicated rate-limited endpoint", bsId)
		_, err := client.RegistryRequest(l, nil, *registryAddress, &msgs.DecommissionBlockServiceReq{
			Id: bsId,
		})
		if err == nil {
			return
		}
		if err == msgs.AUTO_DECOMMISSION_FORBIDDEN || err == msgs.AUTO_DECOMMISSION_RATE_LIMITED {
			if *decommissionBlockserviceForce {
				l.Info("registry refused decommission (%v); forcing DECOMMISSIONED flag via SetBlockServiceFlags", err)
				conn := client.MakeRegistryConn(l, nil, *registryAddress, 1)
				defer conn.Close()
				if _, err = conn.Request(&msgs.SetBlockServiceFlagsReq{
					Id:        bsId,
					Flags:     msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED,
					FlagsMask: uint8(msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED),
				}); err == nil {
					return
				}
			} else {
				l.Info("registry refused decommission (%v); not forcing because -force is not set", err)
			}
		}
		panic(err)
	}
	return Command{
		Flags: decommissionBlockserviceCmd,
		Run:   decommissionBlockserviceRun,
	}
}
