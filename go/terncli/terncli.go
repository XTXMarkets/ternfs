// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/flags"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
	terncmd "github.com/XTXMarkets/ternfs/go/terncli/cmd"
)

var commands map[string]terncmd.Command

func newCommands() map[string]terncmd.Command {
	result := make(map[string]terncmd.Command)
	for _, command := range []terncmd.Command{
		terncmd.NewCollect(),
		terncmd.NewDestruct(),
		terncmd.NewMigrate(),
		terncmd.NewShardReq(),
		terncmd.NewRegistryReq(),
		terncmd.NewCdcReq(),
		terncmd.NewSetDirInfo(),
		terncmd.NewRemoveDirInfo(),
		terncmd.NewCpInto(),
		terncmd.NewCpOutof(),
		terncmd.NewWriteBlockReq(),
		terncmd.NewTestBlockWrite(),
		terncmd.NewBlockserviceFlags(),
		terncmd.NewDecommissionBlockservice(),
		terncmd.NewUpdateBlockservicePath(),
		terncmd.NewFileSizes(),
		terncmd.NewCountFiles(),
		terncmd.NewDu(),
		terncmd.NewFileLocations(),
		terncmd.NewEstimateFileAge(),
		terncmd.NewFind(),
		terncmd.NewRm(),
		terncmd.NewScrubFile(),
		terncmd.NewScrub(),
		terncmd.NewKernelCounters(),
		terncmd.NewKernelLatencies(),
		terncmd.NewDefrag(),
		terncmd.NewDefragSpans(),
		terncmd.NewResurrect(),
		terncmd.NewResurrectSubtree(),
		terncmd.NewResolveSamplePaths(),
		terncmd.NewTagFiles(),
	} {
		result[command.FlagSet().Name()] = command
	}
	return result
}

func noRunawayArgs(flag *flag.FlagSet) {
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Unexpected extra arguments %v\n", flag.Args())
		os.Exit(2)
	}
}

func usage() {
	commandsStrs := []string{}
	for c := range commands {
		commandsStrs = append(commandsStrs, c)
	}
	slices.Sort(commandsStrs)
	fmt.Fprintf(os.Stderr, "Usage: %v <command> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Commands:\n")
	for _, cmd := range commandsStrs {
		fmt.Fprintf(os.Stderr, "  %s\n", cmd)
	}
	fmt.Fprintf(os.Stderr, "\nGlobal options:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	registryAddress := flag.String("registry", "", "Registry address (host:port).")
	var addresses flags.StringArrayFlags
	flag.Var(&addresses, "addr", "Local addresses (up to two) to connect from.")
	mtu := flag.String("mtu", "", "MTU to use, either an integer or \"max\"")
	shardInitialTimeout := flag.Duration("shard-initial-timeout", 0, "")
	shardMaxTimeout := flag.Duration("shard-max-timeout", 0, "")
	shardOverallTimeout := flag.Duration("shard-overall-timeout", -1, "")
	cdcInitialTimeout := flag.Duration("cdc-initial-timeout", 0, "")
	cdcMaxTimeout := flag.Duration("cdc-max-timeout", 0, "")
	cdcOverallTimeout := flag.Duration("cdc-overall-timeout", -1, "")
	verbose := flag.Bool("verbose", false, "")
	trace := flag.Bool("trace", false, "")

	var l *log.Logger

	commands = newCommands()

	flag.Parse()

	var localAddresses msgs.AddrsInfo
	if len(addresses) > 0 {
		ownIp1, port1, err := flags.ParseIPV4Addr(addresses[0])
		if err != nil {
			panic(err)
		}
		localAddresses.Addr1 = msgs.IpPort{Addrs: ownIp1, Port: port1}
		var ownIp2 [4]byte
		var port2 uint16
		if len(addresses) == 2 {
			ownIp2, port2, err = flags.ParseIPV4Addr(addresses[1])
			if err != nil {
				panic(err)
			}
		}
		localAddresses.Addr2 = msgs.IpPort{Addrs: ownIp2, Port: port2}
	}

	if *mtu != "" {
		if *mtu == "max" {
			client.SetMTU(msgs.MAX_UDP_MTU)
		} else {
			mtuU, err := strconv.ParseUint(*mtu, 0, 16)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not parse mtu: %v", err)
				os.Exit(2)
			}
			client.SetMTU(mtuU)
		}
	}

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "No command provided.\n\n")
		flag.Usage()
		os.Exit(2)
	}

	level := log.INFO
	if *verbose {
		level = log.DEBUG
	}
	if *trace {
		level = log.TRACE
	}
	l = log.NewLogger(os.Stderr, &log.LoggerOptions{Level: level})

	spec, found := commands[flag.Args()[0]]
	if !found {
		fmt.Fprintf(os.Stderr, "Bad subcommand %v provided.\n\n", flag.Args()[0])
		flag.Usage()
		os.Exit(2)
	}
	spec.FlagSet().Parse(flag.Args()[1:])
	noRunawayArgs(spec.FlagSet())

	var ternClient *client.Client
	var clientOnce sync.Once
	getClient := func() *client.Client {
		clientOnce.Do(func() {
			if *registryAddress == "" {
				panic("You need to specify -registry.\n")
			}
			var err error
			ternClient, err = client.NewClient(
				l,
				nil,
				*registryAddress,
				localAddresses,
			)
			if err != nil {
				panic(fmt.Errorf("could not create client: %v", err))
			}
			ternClient.SetFetchBlockServices()

			shardTimeouts := client.DefaultShardTimeout
			printTimeouts := false
			if *shardInitialTimeout > 0 {
				printTimeouts = true
				shardTimeouts.Initial = *shardInitialTimeout
			}
			if *shardMaxTimeout > 0 {
				printTimeouts = true
				shardTimeouts.Max = *shardMaxTimeout
			}
			if *shardOverallTimeout >= 0 {
				printTimeouts = true
				shardTimeouts.Overall = *shardOverallTimeout
			}
			ternClient.SetShardTimeouts(&shardTimeouts)
			cdcTimeouts := client.DefaultCDCTimeout
			if *cdcInitialTimeout > 0 {
				printTimeouts = true
				cdcTimeouts.Initial = *cdcInitialTimeout
			}
			if *cdcMaxTimeout > 0 {
				printTimeouts = true
				cdcTimeouts.Max = *cdcMaxTimeout
			}
			if *cdcOverallTimeout >= 0 {
				printTimeouts = true
				cdcTimeouts.Overall = *cdcOverallTimeout
			}
			ternClient.SetCDCTimeouts(&cdcTimeouts)
			if printTimeouts {
				l.Info("shard timeouts: %+v", shardTimeouts)
				l.Info("CDC timeouts: %+v", cdcTimeouts)
			}
		})
		return ternClient
	}
	defer func() {
		if ternClient != nil {
			ternClient.Close()
		}
	}()

	runtime := terncmd.NewRuntime(l, registryAddress, getClient)
	spec.Run(runtime)
}
