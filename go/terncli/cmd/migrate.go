// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/cleanup"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"os"
	"time"
)

func NewMigrate() Command {
	migrateCmd := flag.NewFlagSet("migrate", flag.ExitOnError)
	migrateId := migrateCmd.Int64("id", 0, "Block service id")
	migrateFailureDomain := migrateCmd.String("failure-domain", "", "Failure domain -- if this is used all block services in a given failure domain will be affected.")
	migrateFailureFlagStr := migrateCmd.String("flags", "0", "All block services with the given flags will be included. Use with care! Basically only really useful with DECOMMISSIONED.")
	migrateFailureNoFlagStr := migrateCmd.String("no-flags", "0", "Block services with the given flags will be excluded.")
	migrateFileIdU64 := migrateCmd.Uint64("file", 0, "File in which to migrate blocks. If not present, all files will be migrated.")
	migrateShard := migrateCmd.Int("shard", -1, "Shard to migrate into. If not present, all shards will be migrated")
	migrateRun := func(runtime *Runtime) {
		l := runtime.Log
		registryAddress := runtime.RegistryAddress
		yesFlags, err := msgs.BlockServiceFlagsFromUnion(*migrateFailureFlagStr)
		if err != nil {
			panic(err)
		}
		noFlags, err := msgs.BlockServiceFlagsFromUnion(*migrateFailureNoFlagStr)
		if err != nil {
			panic(err)
		}
		if yesFlags&noFlags != 0 {
			fmt.Fprintf(os.Stderr, "Can't provide the same flag both in -flags and -no-flags\n")
			os.Exit(2)
		}
		l.Info("requesting block services")
		blockServicesResp, err := client.RegistryRequest(l, nil, registryAddress, &msgs.ChangedBlockServicesReq{})
		if err != nil {
			panic(err)
		}
		blockServices := blockServicesResp.(*msgs.ChangedBlockServicesResp)
		blockServicesToMigrate := make(map[string]*[]msgs.BlockServiceId) // by failure domain
		numBlockServicesToMigrate := 0
		for _, bs := range blockServices.BlockServices {
			if bs.Id == msgs.BlockServiceId(*migrateId) || bs.FailureDomain.String() == *migrateFailureDomain || (bs.Flags&yesFlags != 0 && bs.Flags&noFlags == 0) {
				numBlockServicesToMigrate++
				bss := blockServicesToMigrate[bs.FailureDomain.String()]
				if bss == nil {
					bss = &[]msgs.BlockServiceId{}
					blockServicesToMigrate[bs.FailureDomain.String()] = bss
				}
				*bss = append(*bss, bs.Id)
			}
		}
		if len(blockServicesToMigrate) == 0 {
			panic(fmt.Errorf("could not get any block service ids with failure domain %v, id %v, yes flags %v, no flags %v", migrateFailureDomain, msgs.BlockServiceId(*migrateId), yesFlags, noFlags))
		}
		if *migrateShard != -1 && *migrateFileIdU64 != 0 {
			fmt.Fprintf(os.Stderr, "You passed in both -shard and -file, not sure what to do.\n")
			os.Exit(2)
		}
		if *migrateShard > 255 {
			fmt.Fprintf(os.Stderr, "Invalid shard %v.\n", *migrateShard)
			os.Exit(2)
		}
		l.Info("will migrate in %v block services:", numBlockServicesToMigrate)
		for failureDomain, bss := range blockServicesToMigrate {
			for _, blockServiceId := range *bss {
				l.Info("%v, %v", failureDomain, blockServiceId)
			}
		}
		for {
			var action string
			fmt.Printf("Proceed? y/n ")
			fmt.Scanln(&action)
			if action == "y" {
				break
			}
			if action == "n" {
				fmt.Printf("BYE\n")
				os.Exit(0)
			}
		}
		stats := cleanup.MigrateStats{}
		progressReportAlert := l.NewNCAlert(10 * time.Second)
		for failureDomain, bss := range blockServicesToMigrate {
			for _, blockServiceId := range *bss {
				l.Info("migrating block service %v, %v", blockServiceId, failureDomain)
				if *migrateFileIdU64 == 0 && *migrateShard < 0 {
					if err := cleanup.MigrateBlocksInAllShards(l, runtime.getClient(), &stats, progressReportAlert, blockServiceId); err != nil {
						panic(err)
					}
				} else if *migrateFileIdU64 != 0 {
					fileId := msgs.InodeId(*migrateFileIdU64)
					if err := cleanup.MigrateBlocksInFile(l, runtime.getClient(), &stats, progressReportAlert, blockServiceId, fileId); err != nil {
						panic(fmt.Errorf("error while migrating file %v away from block service %v: %v", fileId, blockServiceId, err))
					}
				} else {
					shid := msgs.ShardId(*migrateShard)
					if err := cleanup.MigrateBlocks(l, runtime.getClient(), &stats, progressReportAlert, shid, blockServiceId); err != nil {
						panic(err)
					}
				}
				l.Info("finished migrating blocks away from block service %v, stats so far: %+v", blockServiceId, stats)
			}
		}
		l.Info("finished migrating away from all block services, stats: %+v", stats)
		l.ClearNC(progressReportAlert)
	}
	return Command{
		Flags: migrateCmd,
		Run:   migrateRun,
	}
}
