// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"xtx/ternfs/cleanup"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/certificate"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/core/flags"
	"xtx/ternfs/core/log"
	"xtx/ternfs/core/timing"
	"xtx/ternfs/msgs"
	"xtx/ternfs/terncli/filesamples"
)

type commandSpec struct {
	flags *flag.FlagSet
	run   func()
}

var commands map[string]commandSpec

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
	fmt.Fprintf(os.Stderr, "Usage: %v %v\n\n", os.Args[0], strings.Join(commandsStrs, "|"))
	fmt.Fprintf(os.Stderr, "Global options:\n\n")
	flag.PrintDefaults()
}

func outputFullFileSizes(log *log.Logger, c *client.Client) {
	var examinedDirs uint64
	var examinedFiles uint64
	err := client.Parwalk(
		log,
		c,
		&client.ParwalkOptions{
			WorkersPerShard: 1,
		},
		"/",
		func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
			if id.Type() == msgs.DIRECTORY {
				if atomic.AddUint64(&examinedDirs, 1)%1000000 == 0 {
					log.Info("examined %v dirs, %v files", examinedDirs, examinedFiles)
				}
			} else {
				if atomic.AddUint64(&examinedFiles, 1)%1000000 == 0 {
					log.Info("examined %v dirs, %v files", examinedDirs, examinedFiles)
				}
			}

			if id.Type() == msgs.DIRECTORY {
				return nil
			}

			logicalSize := uint64(0)
			physicalSize := uint64(0)
			spansReq := &msgs.LocalFileSpansReq{
				FileId: id,
			}
			spansResp := &msgs.LocalFileSpansResp{}
			for {
				if err := c.ShardRequest(log, id.Shard(), spansReq, spansResp); err != nil {
					log.ErrorNoAlert("could not read spans for %v: %v", id, err)
					return err
				}
				for _, s := range spansResp.Spans {
					logicalSize += uint64(s.Header.Size)
					if s.Header.StorageClass != msgs.INLINE_STORAGE {
						body := s.Body.(*msgs.FetchedBlocksSpan)
						physicalSize += uint64(body.CellSize) * uint64(body.Parity.Blocks()) * uint64(body.Stripes)
					}
				}
				if spansResp.NextOffset == 0 {
					break
				}
				spansReq.ByteOffset = spansResp.NextOffset
			}
			fmt.Printf("%v,%q,%v,%v\n", id, path.Join(parentPath, name), logicalSize, physicalSize)
			return nil
		},
	)
	if err != nil {
		panic(err)
	}
}

func outputBriefFileSizes(log *log.Logger, c *client.Client) {
	// histogram
	histoBins := 256
	histo := timing.NewHistogram(histoBins, 1024, 1.1)
	var histoSizes [256][]uint64
	var wg sync.WaitGroup
	wg.Add(256)
	for i := 0; i < 256; i++ {
		shid := msgs.ShardId(i)
		histoSizes[i] = make([]uint64, histoBins)
		go func() {
			filesReq := msgs.VisitFilesReq{}
			filesResp := msgs.VisitFilesResp{}
			for {
				if err := c.ShardRequest(log, shid, &filesReq, &filesResp); err != nil {
					log.ErrorNoAlert("could not get files in shard %v: %v, terminating this shard, results will be incomplete", shid, err)
					return
				}
				for _, file := range filesResp.Ids {
					statResp := msgs.StatFileResp{}
					if err := c.ShardRequest(log, shid, &msgs.StatFileReq{Id: file}, &statResp); err != nil {
						log.ErrorNoAlert("could not stat file %v in shard %v: %v, results will be incomplete", file, shid, err)
						continue
					}
					bin := histo.WhichBin(statResp.Size)
					histoSizes[shid][bin] += statResp.Size
				}
				if filesResp.NextId == msgs.NULL_INODE_ID {
					log.Info("finished with shard %v", shid)
					break
				}
				filesReq.BeginId = filesResp.NextId
			}
			wg.Done()
		}()
	}
	wg.Wait()
	log.Info("finished with all shards, will now output")
	for i, upperBound := range histo.Bins() {
		size := uint64(0)
		for j := 0; j < 256; j++ {
			size += histoSizes[j][i]
		}
		fmt.Printf("%v,%v\n", upperBound, size)
	}
}

func formatSize(bytes uint64) string {
	bytesf := float64(bytes)
	if bytes == 0 {
		return "0"
	}
	if bytes < 1e6 {
		return fmt.Sprintf("%.2fKB", bytesf/1e3)
	}
	if bytes < 1e9 {
		return fmt.Sprintf("%.2fMB", bytesf/1e6)
	}
	if bytes < 1e12 {
		return fmt.Sprintf("%.2fGB", bytesf/1e9)
	}
	if bytes < 1e15 {
		return fmt.Sprintf("%.2fTB", bytesf/1e12)
	}
	return fmt.Sprintf("%.2fPB", bytesf/1e15)
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
	var mbClient *client.Client
	var clientMu sync.RWMutex

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
	defer func() {
		clientMu.Lock()
		if mbClient != nil {
			mbClient.Close()
		}
		clientMu.Unlock()
	}()
	getClient := func() *client.Client {
		clientMu.RLock()
		if mbClient != nil {
			clientMu.RUnlock()
			return mbClient
		}
		clientMu.RUnlock()
		clientMu.Lock()
		if mbClient != nil {
			clientMu.Unlock()
			return mbClient
		}
		if *registryAddress == "" {
			clientMu.Unlock()
			panic("You need to specify -registry (or -prod).\n")
		}
		var err error
		c, err := client.NewClient(l, nil, *registryAddress, localAddresses)
		if err != nil {
			clientMu.Unlock()
			panic(fmt.Errorf("could not create client: %v", err))
		}
		c.SetFetchBlockServices()
		mbClient = c
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
		mbClient.SetShardTimeouts(&shardTimeouts)
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
		mbClient.SetCDCTimeouts(&cdcTimeouts)
		if printTimeouts {
			l.Info("shard timeouts: %+v", shardTimeouts)
			l.Info("CDC timeouts: %+v", cdcTimeouts)
		}
		clientMu.Unlock()
		return mbClient
	}

	commands = make(map[string]commandSpec)

	collectCmd := flag.NewFlagSet("collect", flag.ExitOnError)
	collectDirIdU64 := collectCmd.Uint64("dir", 0, "Directory inode id to GC. If not present, they'll all be collected.")
	collectDirMinEdgeAge := collectCmd.Duration("min-edge-age", time.Hour, "Minimum age of edges to be collected")
	collectRun := func() {
		dirInfoCache := client.NewDirInfoCache()
		if *collectDirIdU64 == 0 {
			state := &cleanup.CollectDirectoriesState{}
			if err := cleanup.CollectDirectoriesInAllShards(l, getClient(), dirInfoCache, nil, &cleanup.CollectDirectoriesOpts{NumWorkersPerShard: 2, WorkersQueueSize: 100}, state, *collectDirMinEdgeAge); err != nil {
				panic(err)
			}
		} else {
			dirId := msgs.InodeId(*collectDirIdU64)
			if dirId.Type() != msgs.DIRECTORY {
				panic(fmt.Errorf("inode id %v is not a directory", dirId))
			}
			var stats cleanup.CollectDirectoriesStats
			if err := cleanup.CollectDirectory(l, getClient(), dirInfoCache, &stats, dirId, *collectDirMinEdgeAge); err != nil {
				panic(fmt.Errorf("could not collect %v, stats: %+v, err: %v", dirId, stats, err))
			}
			l.Info("finished collecting %v, stats: %+v", dirId, stats)
		}
	}
	commands["collect"] = commandSpec{
		flags: collectCmd,
		run:   collectRun,
	}

	destructCmd := flag.NewFlagSet("destruct", flag.ExitOnError)
	destructFileIdU64 := destructCmd.Uint64("file", 0, "Transient file id to destruct. If not present, they'll all be destructed.")
	destrutcFileShardId := destructCmd.Int("shard", -1, "Shard to destruct into. Will destruct all of them if -1.")
	destructFileCookieU64 := destructCmd.Uint64("cookie", 0, "Transient file cookie. Must be present if file is specified.")
	destructRun := func() {
		if *destructFileIdU64 == 0 {
			state := &cleanup.DestructFilesState{}
			opts := &cleanup.DestructFilesOptions{NumWorkersPerShard: 10, WorkersQueueSize: 100}
			if *destrutcFileShardId < 0 {
				if err := cleanup.DestructFilesInAllShards(l, getClient(), opts, state); err != nil {
					panic(err)
				}
			} else {
				if err := cleanup.DestructFiles(l, getClient(), opts, state, msgs.ShardId(*destrutcFileShardId)); err != nil {
					panic(err)
				}
			}
		} else {
			fileId := msgs.InodeId(*destructFileIdU64)
			if fileId.Type() == msgs.DIRECTORY {
				panic(fmt.Errorf("inode id %v is not a file/symlink", fileId))
			}
			stats := cleanup.DestructFilesStats{}
			var destructFileCookie [8]byte
			binary.LittleEndian.PutUint64(destructFileCookie[:], *destructFileCookieU64)
			if err := cleanup.DestructFile(l, getClient(), &stats, fileId, 0, destructFileCookie); err != nil {
				panic(fmt.Errorf("could not destruct %v, stats: %+v, err: %v", fileId, stats, err))
			}
			l.Info("finished destructing %v, stats: %+v", fileId, stats)
		}
	}
	commands["destruct"] = commandSpec{
		flags: destructCmd,
		run:   destructRun,
	}

	migrateCmd := flag.NewFlagSet("migrate", flag.ExitOnError)
	migrateId := migrateCmd.Int64("id", 0, "Block service id")
	migrateFailureDomain := migrateCmd.String("failure-domain", "", "Failure domain -- if this is used all block services in a given failure domain will be affected.")
	migrateFailureFlagStr := migrateCmd.String("flags", "0", "All block services with the given flags will be included. Use with care! Basically only really useful with DECOMMISSIONED.")
	migrateFailureNoFlagStr := migrateCmd.String("no-flags", "0", "Block services with the given flags will be excluded.")
	migrateFileIdU64 := migrateCmd.Uint64("file", 0, "File in which to migrate blocks. If not present, all files will be migrated.")
	migrateShard := migrateCmd.Int("shard", -1, "Shard to migrate into. If not present, all shards will be migrated")
	migrateRun := func() {
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
		blockServicesResp, err := client.RegistryRequest(l, nil, *registryAddress, &msgs.AllBlockServicesDeprecatedReq{})
		if err != nil {
			panic(err)
		}
		blockServices := blockServicesResp.(*msgs.AllBlockServicesDeprecatedResp)
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
					if err := cleanup.MigrateBlocksInAllShards(l, getClient(), &stats, progressReportAlert, blockServiceId); err != nil {
						panic(err)
					}
				} else if *migrateFileIdU64 != 0 {
					fileId := msgs.InodeId(*migrateFileIdU64)
					if err := cleanup.MigrateBlocksInFile(l, getClient(), &stats, progressReportAlert, blockServiceId, fileId); err != nil {
						panic(fmt.Errorf("error while migrating file %v away from block service %v: %v", fileId, blockServiceId, err))
					}
				} else {
					shid := msgs.ShardId(*migrateShard)
					if err := cleanup.MigrateBlocks(l, getClient(), &stats, progressReportAlert, shid, blockServiceId); err != nil {
						panic(err)
					}
				}
				l.Info("finished migrating blocks away from block service %v, stats so far: %+v", blockServiceId, stats)
			}
		}
		l.Info("finished migrating away from all block services, stats: %+v", stats)
		l.ClearNC(progressReportAlert)
	}
	commands["migrate"] = commandSpec{
		flags: migrateCmd,
		run:   migrateRun,
	}

	shardReqCmd := flag.NewFlagSet("shard-req", flag.ExitOnError)
	shardReqShard := shardReqCmd.Uint("shard", 0, "Shard to send the req too")
	shardReqKind := shardReqCmd.String("kind", "", "")
	shardReqReq := shardReqCmd.String("req", "", "Request body, in JSON")
	shardReqYes := shardReqCmd.Bool("yes", false, "Do not ask for confirmation")
	shardReqRun := func() {
		req, resp, err := msgs.MkShardMessage(*shardReqKind)
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal([]byte(*shardReqReq), &req); err != nil {
			panic(fmt.Errorf("could not decode shard req: %w", err))
		}
		shard := msgs.ShardId(*shardReqShard)
		fmt.Printf("Will send this request to shard %v: %T %+v\n", shard, req, req)
		if !*shardReqYes {
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
		}
		if err := getClient().ShardRequest(l, shard, req, resp); err != nil {
			panic(err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			panic(fmt.Errorf("could not encode response %+v to json: %w", resp, err))
		}
		os.Stdout.Write(out)
		fmt.Println()
	}
	commands["shard-req"] = commandSpec{
		flags: shardReqCmd,
		run:   shardReqRun,
	}

	cdcReqCmd := flag.NewFlagSet("cdc-req", flag.ExitOnError)
	cdcReqKind := cdcReqCmd.String("kind", "", "")
	cdcReqReq := cdcReqCmd.String("req", "", "Request body, in JSON")
	cdcReqRun := func() {
		req, resp, err := msgs.MkCDCMessage(*cdcReqKind)
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal([]byte(*cdcReqReq), &req); err != nil {
			panic(fmt.Errorf("could not decode cdc req: %w", err))
		}
		fmt.Printf("Will send this CDC request: %T %+v\n", req, req)
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
		if err := getClient().CDCRequest(l, req, resp); err != nil {
			panic(err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			panic(fmt.Errorf("could not encode response %+v to json: %w", resp, err))
		}
		os.Stdout.Write(out)
		fmt.Println()
	}
	commands["cdc-req"] = commandSpec{
		flags: cdcReqCmd,
		run:   cdcReqRun,
	}

	setDirInfoCmd := flag.NewFlagSet("set-dir-info", flag.ExitOnError)
	setDirInfoIdU64 := setDirInfoCmd.Uint64("id", 0, "InodeId for the directory to set the policy of.")
	setDirInfoIdTag := setDirInfoCmd.String("tag", "", "One of SNAPSHOT|SPAN|BLOCK|STRIPE")
	setDirInfoPolicy := setDirInfoCmd.String("body", "", "Policy, in JSON")
	setDirInfoRun := func() {
		entry := msgs.TagToDirInfoEntry(msgs.DirInfoTagFromName(*setDirInfoIdTag))
		if err := json.Unmarshal([]byte(*setDirInfoPolicy), entry); err != nil {
			panic(fmt.Errorf("could not decode directory info: %w", err))
		}
		id := msgs.InodeId(*setDirInfoIdU64)
		fmt.Printf("Will set dir info %v to directory %v:\n", entry.Tag(), id)
		fmt.Printf("%+v\n", entry)
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
		if err := getClient().MergeDirectoryInfo(l, id, entry); err != nil {
			panic(err)
		}
	}
	commands["set-dir-info"] = commandSpec{
		flags: setDirInfoCmd,
		run:   setDirInfoRun,
	}

	removeDirInfoCmd := flag.NewFlagSet("remove-dir-info", flag.ExitOnError)
	removeDirInfoU64 := removeDirInfoCmd.Uint64("id", 0, "InodeId for the directory to unset the policy of.")
	removeDirInfoTag := removeDirInfoCmd.String("tag", "", "One of SNAPSHOT|SPAN|BLOCK")
	removeDirInfoRun := func() {
		id := msgs.InodeId(*removeDirInfoU64)
		if err := getClient().RemoveDirectoryInfoEntry(l, id, msgs.DirInfoTagFromName(*removeDirInfoTag)); err != nil {
			panic(err)
		}
	}
	commands["remove-dir-info"] = commandSpec{
		flags: removeDirInfoCmd,
		run:   removeDirInfoRun,
	}

	cpIntoCmd := flag.NewFlagSet("cp-into", flag.ExitOnError)
	cpIntoInput := cpIntoCmd.String("i", "", "What to copy, if empty stdin.")
	cpIntoOut := cpIntoCmd.String("o", "", "Where to write the file to in TernFS")
	cpIntoRun := func() {
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
		fileId, err := getClient().CreateFile(l, bufPool, client.NewDirInfoCache(), path, input)
		if err != nil {
			panic(err)
		}
		l.Info("File created as %v", fileId)
	}
	commands["cp-into"] = commandSpec{
		flags: cpIntoCmd,
		run:   cpIntoRun,
	}

	cpOutofCmd := flag.NewFlagSet("cp-outof", flag.ExitOnError)
	cpOutofInput := cpOutofCmd.String("i", "", "What to copy from TernFS.")
	cpOutofId := cpOutofCmd.Uint64("id", 0, "The ID of the file to copy.") //
	cpOutofOut := cpOutofCmd.String("o", "", "Where to write the file to. Stdout if empty.")
	cpOutofRun := func() {
		out := os.Stdout
		if *cpOutofOut != "" {
			var err error
			//os.MkdirAll(filepath.Dir(*cpOutofOut), 0755)
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
			id, err = getClient().ResolvePath(l, *cpOutofInput)
			if err != nil {
				panic(err)
			}
		}

		bufPool := bufpool.NewBufPool()
		r, err := getClient().FetchFile(l, bufPool, id)
		if err != nil {
			panic(err)
		}
		if _, err := out.Write(r.Bytes()); err != nil {
			panic(err)
		}
		out.Close()
	}
	commands["cp-outof"] = commandSpec{
		flags: cpOutofCmd,
		run:   cpOutofRun,
	}

	blockReqCmd := flag.NewFlagSet("write-block-req", flag.ExitOnError)
	blockReqBlockId := blockReqCmd.Uint64("b", 0, "Block id")
	blockReqBlockService := blockReqCmd.Uint64("bs", 0, "Block service")
	blockReqFile := blockReqCmd.String("file", "", "")
	blockReqRun := func() {
		resp, err := client.RegistryRequest(l, nil, *registryAddress, &msgs.AllBlockServicesDeprecatedReq{})
		if err != nil {
			panic(err)
		}
		blockServices := resp.(*msgs.AllBlockServicesDeprecatedResp)
		var blockServiceInfo msgs.BlockServiceDeprecatedInfo
		for _, bsInfo := range blockServices.BlockServices {
			if bsInfo.Id == msgs.BlockServiceId(*blockReqBlockService) {
				blockServiceInfo = bsInfo
				break
			}
		}
		cipher, err := aes.NewCipher(blockServiceInfo.SecretKey[:])
		if err != nil {
			panic(err)
		}
		fileContents, err := ioutil.ReadFile(*blockReqFile)
		if err != nil {
			panic(err)
		}
		req := msgs.WriteBlockReq{
			BlockId: msgs.BlockId(*blockReqBlockId),
			Crc:     msgs.Crc(crc32c.Sum(0, fileContents)),
			Size:    uint32(len(fileContents)),
		}
		req.Certificate = certificate.BlockWriteCertificate(cipher, blockServiceInfo.Id, &req)
		l.Info("request: %+v", req)
	}
	commands["write-block-req"] = commandSpec{
		flags: blockReqCmd,
		run:   blockReqRun,
	}

	testBlockWriteCmd := flag.NewFlagSet("test-block-write", flag.ExitOnError)
	testBlockWriteBlockService := testBlockWriteCmd.String("bs", "", "Block service. If comma-separated, they'll be written in parallel to the specified ones.")
	testBlockWriteSize := testBlockWriteCmd.Uint("size", 0, "Size (must fit in u32)")
	testBlockWriteRun := func() {
		resp, err := client.RegistryRequest(l, nil, *registryAddress, &msgs.AllBlockServicesDeprecatedReq{})
		if err != nil {
			panic(err)
		}
		blockServices := resp.(*msgs.AllBlockServicesDeprecatedResp)
		bsInfos := []msgs.BlockServiceDeprecatedInfo{}
		for _, str := range strings.Split(*testBlockWriteBlockService, ",") {
			bsId, err := strconv.ParseUint(str, 0, 64)
			if err != nil {
				panic(err)
			}
			found := false
			for _, bsInfo := range blockServices.BlockServices {
				if bsInfo.Id == msgs.BlockServiceId(bsId) {
					bsInfos = append(bsInfos, bsInfo)
					found = true
					break
				}
			}
			if !found {
				panic(fmt.Errorf("could not find block service %q", str))
			}
		}
		conns := make([]*net.TCPConn, len(bsInfos))
		for i := 0; i < len(conns); i++ {
			conn, err := client.BlockServiceConnection(l, bsInfos[i].Addrs)
			if err != nil {
				panic(err)
			}
			conns[i] = conn
		}
		contents := make([]byte, *testBlockWriteSize)
		var wait sync.WaitGroup
		wait.Add(len(conns))
		t := time.Now()
		for i := 0; i < len(conns); i++ {
			conn := conns[i]
			bsId := bsInfos[i].Id
			go func() {
				thisErr := client.TestWrite(l, conn, bsId, bytes.NewReader(contents), uint64(len(contents)))
				if thisErr != nil {
					err = thisErr
				}
				wait.Done()
			}()
		}
		wait.Wait()
		elapsed := time.Since(t)
		if err != nil {
			panic(err)
		}
		l.Info("writing %v bytes to %v block services took %v (%fGB/s)", *testBlockWriteSize, len(conns), time.Since(t), (float64(*testBlockWriteSize*uint(len(conns)))/1e9)/elapsed.Seconds())
	}
	commands["test-block-write"] = commandSpec{
		flags: testBlockWriteCmd,
		run:   testBlockWriteRun,
	}

	blockserviceFlagsCmd := flag.NewFlagSet("blockservice-flags", flag.ExitOnError)
	blockserviceFlagsId := blockserviceFlagsCmd.Int64("id", 0, "Block service id")
	blockserviceFlagsFailureDomain := blockserviceFlagsCmd.String("failure-domain", "", "Failure domain -- if this is used all block services in a given failure domain will be affected.")
	blockserviceFlagsPathPrefix := blockserviceFlagsCmd.String("path-prefix", "", "Path prefix -- if this is used all block services with a given path prefix will be affected.")
	blockserviceFlagsSet := blockserviceFlagsCmd.String("set", "", "Flag to set")
	blockserviceFlagsUnset := blockserviceFlagsCmd.String("unset", "", "Flag to unset")
	blockserviceFlagsRun := func() {
		if *blockserviceFlagsSet != "" && *blockserviceFlagsUnset != "" {
			fmt.Fprintf(os.Stderr, "cannot use -set and -unset at the same time\n")
			os.Exit(2)
		}
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
			blockServicesResp, err := client.RegistryRequest(l, nil, *registryAddress, &msgs.AllBlockServicesDeprecatedReq{})
			if err != nil {
				panic(err)
			}
			blockServices := blockServicesResp.(*msgs.AllBlockServicesDeprecatedResp)
			for _, bs := range blockServices.BlockServices {
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
		var flag msgs.BlockServiceFlags
		var mask uint8
		if *blockserviceFlagsSet != "" {
			var err error
			flag, err = msgs.BlockServiceFlagFromName(*blockserviceFlagsSet)
			if err != nil {
				panic(err)
			}
			mask = uint8(flag)
		}
		if *blockserviceFlagsUnset != "" {
			flagMask, err := msgs.BlockServiceFlagFromName(*blockserviceFlagsUnset)
			if err != nil {
				panic(err)
			}
			mask = uint8(flagMask)
		}
		conn := client.MakeRegistryConn(l, nil, *registryAddress, 1)
		defer conn.Close()
		for _, bsId := range blockServiceIds {
			l.Info("setting flags %v with mask %v for block service %v", flag, msgs.BlockServiceFlags(mask), bsId)
			_, err := conn.Request(&msgs.SetBlockServiceFlagsReq{
				Id:        bsId,
				Flags:     flag,
				FlagsMask: mask,
			})
			if err != nil {
				panic(err)
			}
		}
	}
	commands["blockservice-flags"] = commandSpec{
		flags: blockserviceFlagsCmd,
		run:   blockserviceFlagsRun,
	}

	decommissionBlockserviceCmd := flag.NewFlagSet("decommission-blockservice", flag.ExitOnError)
	decommissionBlockserviceId := decommissionBlockserviceCmd.Int64("id", 0, "Block service id")
	decommissionBlockserviceRun := func() {
		if *decommissionBlockserviceId == 0 {
			fmt.Fprintf(os.Stderr, "must provide -id\n")
			os.Exit(2)
		}
		bsId := msgs.BlockServiceId(*decommissionBlockserviceId)
		l.Info("decommissioning block service %v using dedicated rate-limited endpoint", bsId)
		_, err := client.RegistryRequest(l, nil, *registryAddress, &msgs.DecommissionBlockServiceReq{
			Id: bsId,
		})
		if err != nil {
			panic(err)
		}
	}
	commands["decommission-blockservice"] = commandSpec{
		flags: decommissionBlockserviceCmd,
		run:   decommissionBlockserviceRun,
	}

	updateBlockservicePathCmd := flag.NewFlagSet("update-blockservice-path", flag.ExitOnError)
	updateBlockservicePathId := updateBlockservicePathCmd.Int64("id", 0, "Block service id")
	updateBlockserviceNewPath := updateBlockservicePathCmd.String("new-path", "", "New block service path")
	updateBlockservicePathRun := func() {
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
	commands["update-blockservice-path"] = commandSpec{
		flags: updateBlockservicePathCmd,
		run:   updateBlockservicePathRun,
	}

	fileSizesCmd := flag.NewFlagSet("file-sizes", flag.ExitOnError)
	fileSizesBrief := fileSizesCmd.Bool("brief", false, "")
	fileSizesRun := func() {
		if *fileSizesBrief {
			outputBriefFileSizes(l, getClient())
		} else {
			outputFullFileSizes(l, getClient())
		}
	}
	commands["file-sizes"] = commandSpec{
		flags: fileSizesCmd,
		run:   fileSizesRun,
	}

	countFilesCmd := flag.NewFlagSet("count-files", flag.ExitOnError)
	countFilesRun := func() {
		var wg sync.WaitGroup
		ch := make(chan any)
		wg.Add(256)
		var numFiles uint64
		var numReqs uint64
		startedAt := time.Now()
		for i := 0; i < 256; i++ {
			shid := msgs.ShardId(i)
			go func() {
				req := msgs.VisitFilesReq{}
				resp := msgs.VisitFilesResp{}
				for {
					if err := getClient().ShardRequest(l, shid, &req, &resp); err != nil {
						ch <- err
						return
					}
					atomic.AddUint64(&numFiles, uint64(len(resp.Ids)))
					if atomic.AddUint64(&numReqs, 1)%uint64(1_000_000) == 0 {
						l.Info("went through %v files, %v reqs (%0.2f files/s, %0.2f req/s)", numFiles, numReqs, float64(numFiles)/float64(time.Since(startedAt).Seconds()), float64(numReqs)/float64(time.Since(startedAt).Seconds()))
					}
					req.BeginId = resp.NextId
					if req.BeginId == 0 {
						break
					}
				}
				wg.Done()
			}()
		}
		go func() {
			wg.Wait()
			ch <- nil
		}()
		err := <-ch
		if err != nil {
			panic(err)
		}
		l.Info("found %v files", numFiles)
	}
	commands["count-files"] = commandSpec{
		flags: countFilesCmd,
		run:   countFilesRun,
	}

	duCmd := flag.NewFlagSet("du", flag.ExitOnError)
	duDir := duCmd.String("path", "/", "")
	duHisto := duCmd.String("histogram", "", "Filepath in which to write size histogram (in CSV) to")
	duPhysical := duCmd.Bool("physical", false, "Also measure physical space (slower)")
	duSnapshot := duCmd.Bool("snapshot", false, "Also count snapshot files")
	duWorkersPerSshard := duCmd.Int("workers-per-shard", 5, "")
	duLocation := duCmd.Uint("location", 0, "Location for which to report size")
	duPattern := duCmd.String("pattern", "", "If set only measure files matching this regex pattern")
	duRun := func() {
		re, err := regexp.Compile(*duPattern)
		if err != nil {
			fmt.Println("failed to compile regex pattern:", err)
			return
		}
		var numDirectories uint64
		var numFiles uint64
		var totalLogicalSize uint64
		var totalPhysicalSize uint64
		var numSnapshotFiles uint64
		var totalSnapshotLogicalSize uint64
		var totalSnapshotPhysicalSize uint64
		histogram := timing.NewHistogram(256, 255, 1.15) // max: ~900PB
		histoLogicalSizeBins := make([]uint64, 256)
		histoPhysicalSizeBins := make([]uint64, 256)
		histoCountBins := make([]uint64, 256)
		startedAt := time.Now()
		c := getClient()
		printReport := func() {
			if *duSnapshot {
				if *duPhysical {
					l.Info("went through %v files (%v current logical, %v current physical, %v snapshot logical, %v snapshot physical, %0.2f files/s), %v directories", numFiles, formatSize(totalLogicalSize), formatSize(totalPhysicalSize), formatSize(totalSnapshotLogicalSize), formatSize(totalSnapshotPhysicalSize), float64(numFiles)/float64(time.Since(startedAt).Seconds()), numDirectories)
				} else {
					l.Info("went through %v files (%v current, %v snapshot, %0.2f files/s), %v directories", numFiles, formatSize(totalLogicalSize), formatSize(totalSnapshotLogicalSize), float64(numFiles)/float64(time.Since(startedAt).Seconds()), numDirectories)
				}
			} else {
				if *duPhysical {
					l.Info("went through %v files (%v logical, %v physical, %0.2f files/s), %v directories", numFiles, formatSize(totalLogicalSize), formatSize(totalPhysicalSize), float64(numFiles)/float64(time.Since(startedAt).Seconds()), numDirectories)
				} else {
					l.Info("went through %v files (%v, %0.2f files/s), %v directories", numFiles, formatSize(totalLogicalSize), float64(numFiles)/float64(time.Since(startedAt).Seconds()), numDirectories)
				}
			}
		}
		err = client.Parwalk(
			l,
			c,
			&client.ParwalkOptions{
				WorkersPerShard: *duWorkersPerSshard,
				Snapshot:        *duSnapshot,
			},
			*duDir,
			func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
				if !owned {
					return nil
				}
				if id.Type() == msgs.DIRECTORY {
					atomic.AddUint64(&numDirectories, 1)
					return nil
				}
				fullPath := path.Join(parentPath, name)
				if !re.MatchString(fullPath) {
					return nil
				}
				atomic.AddUint64(&numFiles, 1)
				resp := msgs.StatFileResp{}
				if err := c.ShardRequest(l, id.Shard(), &msgs.StatFileReq{Id: id}, &resp); err != nil {
					return err
				}
				if current {
					atomic.AddUint64(&totalLogicalSize, resp.Size)
				} else {
					atomic.AddUint64(&totalSnapshotLogicalSize, resp.Size)
				}
				bin := histogram.WhichBin(resp.Size)
				atomic.AddUint64(&histoCountBins[bin], 1)
				atomic.AddUint64(&histoLogicalSizeBins[bin], resp.Size)
				if *duPhysical {
					fileSpansReq := msgs.FileSpansReq{
						FileId:     id,
						ByteOffset: 0,
					}
					fileSpansResp := msgs.FileSpansResp{}
					physicalSize := uint64(0)
					for {
						if err := c.ShardRequest(l, id.Shard(), &fileSpansReq, &fileSpansResp); err != nil {
							return err
						}
						for spanIx := range fileSpansResp.Spans {
							span := &fileSpansResp.Spans[spanIx]
							if span.Header.IsInline {
								continue
							}
							locBody := span.Body.(*msgs.FetchedLocations)
							for _, loc := range locBody.Locations {
								if uint(loc.LocationId) != *duLocation {
									continue
								}
								physicalSize += uint64(loc.CellSize) * uint64(loc.Parity.Blocks()) * uint64(loc.Stripes)
							}
						}
						if fileSpansResp.NextOffset == 0 {
							break
						}
						fileSpansReq.ByteOffset = fileSpansResp.NextOffset
					}
					if current {
						atomic.AddUint64(&totalPhysicalSize, physicalSize)
					} else {
						atomic.AddUint64(&totalSnapshotPhysicalSize, physicalSize)
					}
					atomic.AddUint64(&histoPhysicalSizeBins[bin], physicalSize)
				}
				var currFiles uint64
				if current {
					currFiles = atomic.AddUint64(&numFiles, 1)
				} else {
					currFiles = atomic.AddUint64(&numSnapshotFiles, 1)
				}
				if currFiles%uint64(1_000_000) == 0 {
					printReport()
				}
				return nil
			},
		)
		if err != nil {
			panic(err)
		}
		printReport()
		if *duHisto != "" {
			l.Info("writing size histogram to %q", *duHisto)
			histoCsvBuf := bytes.NewBuffer([]byte{})
			if *duPhysical {
				fmt.Fprintf(histoCsvBuf, "logical_upper_bound,file_count,total_logical_size,total_physical_size\n")
			} else {
				fmt.Fprintf(histoCsvBuf, "upper_bound,file_count,total_size\n")
			}
			for i, upperBound := range histogram.Bins() {
				if *duPhysical {
					fmt.Fprintf(histoCsvBuf, "%v,%v,%v,%v\n", upperBound, histoCountBins[i], histoLogicalSizeBins[i], histoPhysicalSizeBins[i])
				} else {
					fmt.Fprintf(histoCsvBuf, "%v,%v,%v\n", upperBound, histoCountBins[i], histoLogicalSizeBins[i])
				}
			}
			if err := os.WriteFile(*duHisto, histoCsvBuf.Bytes(), 0644); err != nil {
				l.ErrorNoAlert("could not write histo file %q, will print histogram here: %v", *duHisto, err)
				fmt.Print(histoCsvBuf.Bytes())
				panic(err)
			}
		}
	}
	commands["du"] = commandSpec{
		flags: duCmd,
		run:   duRun,
	}

	fileLocationsCmd := flag.NewFlagSet("file-locations", flag.ExitOnError)
	fileLocationsId := fileLocationsCmd.Uint64("id", 0, "ID of the file to query")
	fileLocationsRun := func() {
		id := msgs.InodeId(*fileLocationsId)
		c := getClient()
		fileSpansReq := msgs.FileSpansReq{
			FileId:     id,
			ByteOffset: 0,
		}
		fileSpansResp := msgs.FileSpansResp{}
		locationSize := make(map[msgs.Location]uint64)

		for {
			if err := c.ShardRequest(l, id.Shard(), &fileSpansReq, &fileSpansResp); err != nil {
				panic(err)
			}
			for spanIx := range fileSpansResp.Spans {
				span := &fileSpansResp.Spans[spanIx]
				if span.Header.IsInline {
					continue
				}
				locBody := span.Body.(*msgs.FetchedLocations)
				for _, loc := range locBody.Locations {
					locationSize[loc.LocationId] += uint64(loc.CellSize) * uint64(loc.Parity.Blocks()) * uint64(loc.Stripes)
				}
			}
			if fileSpansResp.NextOffset == 0 {
				break
			}
			fileSpansReq.ByteOffset = fileSpansResp.NextOffset
		}
		l.Info("Done fetching locations for file %v", id)
		for locId, size := range locationSize {
			l.Info("Location %v has size %v", locId, size)
		}
	}
	commands["file-locations"] = commandSpec{
		flags: fileLocationsCmd,
		run:   fileLocationsRun,
	}

	findCmd := flag.NewFlagSet("find", flag.ExitOnError)
	findDir := findCmd.String("path", "/", "")
	findName := findCmd.String("name", "", "Regex to match the name against.")
	findSnapshot := findCmd.Bool("snapshot", false, "If set, will search through snapshot directory entries too.")
	findOnlySnapshot := findCmd.Bool("only-snapshot", false, "If set, will return _only_ snapshot edges.")
	findOnlyOwned := findCmd.Bool("only-owned", false, "If true and -snapshot is set, only owned files will be searched.")
	findBeforeSpec := findCmd.String("before", "", "If set, only directory entries created before this duration/date will be searched.")
	findBlockId := findCmd.Uint64("block-id", 0, "If specified, only files which contain the given block will be returned.")
	findMinSize := findCmd.Uint64("min-size", 0, "If specified, only files of at least this size will be returned.")
	findCheckBlocks := findCmd.Bool("check-blocks", false, "If true check all blocks in file")
	findWorkersPerShard := findCmd.Int("workers-per-shard", 5, "")
	findRun := func() {
		re := regexp.MustCompile(`.*`)
		if *findName != "" {
			re = regexp.MustCompile(*findName)
		}
		findBefore := msgs.TernTime(^uint64(0))
		if *findBeforeSpec != "" {
			d, durErr := time.ParseDuration(*findBeforeSpec)
			if durErr != nil {
				t, tErr := time.Parse(time.RFC3339Nano, *findBeforeSpec)
				if tErr != nil {
					panic(fmt.Errorf("could not parse %q as duration or time: %v, %v", *findBeforeSpec, durErr, tErr))
				}
				findBefore = msgs.MakeTernTime(t)
			} else {
				findBefore = msgs.MakeTernTime(time.Now().Add(-d))
			}
		}
		c := getClient()
		err := client.Parwalk(
			l,
			c,
			&client.ParwalkOptions{
				WorkersPerShard: *findWorkersPerShard,
				Snapshot:        *findSnapshot,
			},
			*findDir,
			func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
				if !owned && *findOnlyOwned {
					return nil
				}
				if current && *findOnlySnapshot {
					return nil
				}
				if creationTime > findBefore {
					return nil
				}
				if !re.MatchString(name) {
					return nil
				}
				if *findMinSize > 0 {
					if id.Type() == msgs.DIRECTORY {
						return nil
					}
					statReq := msgs.StatFileReq{Id: id}
					statResp := msgs.StatFileResp{}
					if err := c.ShardRequest(l, id.Shard(), &statReq, &statResp); err != nil {
						if err == msgs.FILE_NOT_FOUND {
							// could get collected
							l.Info("file %q disappeared", path.Join(parentPath, name))
						} else {
							return err
						}
					}
					if statResp.Size < *findMinSize {
						return nil
					}
				}
				if *findBlockId != 0 {
					if id.Type() == msgs.DIRECTORY {
						return nil
					}
					req := msgs.LocalFileSpansReq{
						FileId: id,
					}
					resp := msgs.LocalFileSpansResp{}
					found := false
					for {
						if err := c.ShardRequest(l, id.Shard(), &req, &resp); err != nil {
							return err
						}
						for _, span := range resp.Spans {
							if span.Header.StorageClass == msgs.INLINE_STORAGE {
								continue
							}
							body := span.Body.(*msgs.FetchedBlocksSpan)
							for _, block := range body.Blocks {
								if block.BlockId == msgs.BlockId(*findBlockId) {
									found = true
									break
								}
							}
							if found {
								break
							}
						}
						req.ByteOffset = resp.NextOffset
						if req.ByteOffset == 0 || found {
							break
						}
					}
					if !found {
						return nil
					}
				}
				if *findCheckBlocks {
					if id.Type() == msgs.DIRECTORY {
						return nil
					}
					req := msgs.LocalFileSpansReq{
						FileId: id,
					}
					resp := msgs.LocalFileSpansResp{}
					for {
						if err := c.ShardRequest(l, id.Shard(), &req, &resp); err != nil {
							return err
						}
						for _, span := range resp.Spans {
							if span.Header.StorageClass == msgs.INLINE_STORAGE {
								continue
							}
							body := span.Body.(*msgs.FetchedBlocksSpan)
							for _, block := range body.Blocks {
								blockService := &resp.BlockServices[block.BlockServiceIx]
								if err := c.CheckBlock(l, blockService, block.BlockId, body.CellSize*uint32(body.Stripes), block.Crc); err != nil {
									l.ErrorNoAlert("while checking block %v in file %v got error %v", block.BlockId, path.Join(parentPath, name), err)
								}
							}
						}
						req.ByteOffset = resp.NextOffset
						if req.ByteOffset == 0 {
							break
						}
					}
				}
				l.Info("%v %q", id, path.Join(parentPath, name))
				return nil
			},
		)
		if err != nil {
			panic(err)
		}
	}
	commands["find"] = commandSpec{
		flags: findCmd,
		run:   findRun,
	}

	scrubFileCmd := flag.NewFlagSet("scrub-file", flag.ExitOnError)
	scrubFileId := scrubFileCmd.Uint64("id", 0, "The file to scrub")
	scrubFileRun := func() {
		file := msgs.InodeId(*scrubFileId)
		stats := &cleanup.ScrubState{}
		if err := cleanup.ScrubFile(l, getClient(), stats, file); err != nil {
			panic(err)
		}
		l.Info("scrub stats: %+v", stats)
	}
	commands["scrub-file"] = commandSpec{
		flags: scrubFileCmd,
		run:   scrubFileRun,
	}

	scrubCmd := flag.NewFlagSet("scrub", flag.ExitOnError)
	scrubRun := func() {
		stats := cleanup.ScrubState{}
		if err := cleanup.ScrubFilesInAllShards(l, getClient(), &cleanup.ScrubOptions{NumWorkersPerShard: 10}, nil, &stats); err != nil {
			panic(err)
		}

	}
	commands["scrub"] = commandSpec{
		flags: scrubCmd,
		run:   scrubRun,
	}

	kernelCountersCmd := flag.NewFlagSet("kernel-counters", flag.ExitOnError)
	kernelCountersRun := func() {
		{
			header, err := parseKernelMetricsHeader("shard")
			if err != nil {
				panic(err)
			}
			counters, err := parseShardKernelCounters(header)
			if err != nil {
				panic(err)
			}
			for _, c := range counters {
				l.Info("%v: Success=%v Attempts=%v Timeouts=%v Failures=%v NetFailures=%v", msgs.ShardMessageKind(c.Kind), c.Success, c.Attempts, c.Timeouts, c.Failures, c.NetFailures)
			}
		}
		{
			header, err := parseKernelMetricsHeader("cdc")
			if err != nil {
				panic(err)
			}
			counters, err := parseCDCKernelCounters(header)
			if err != nil {
				panic(err)
			}
			for _, c := range counters {
				l.Info("%v: Success=%v Attempts=%v Timeouts=%v Failures=%v NetFailures=%v", msgs.CDCMessageKind(c.Kind), c.Success, c.Attempts, c.Timeouts, c.Failures, c.NetFailures)
			}
		}
	}
	commands["kernel-counters"] = commandSpec{
		flags: kernelCountersCmd,
		run:   kernelCountersRun,
	}

	kernelLatenciesCmd := flag.NewFlagSet("kernel-latencies", flag.ExitOnError)
	kernelLatenciesRun := func() {
		p := func(mh *kernelMetricsHeader, l *kernelLatencies, target float64) time.Duration {
			totalCount := uint64(0)
			for _, bin := range l.LatencyBins {
				totalCount += bin
			}
			if totalCount == 0 {
				return 0
			}
			p := float64(0)
			for k := len(l.LatencyBins) - 1; k > 0; k-- {
				val := l.LatencyBins[k]
				p += float64(val) / float64(totalCount)
				if p >= target {
					return time.Duration(mh.UpperBoundValues[k])
				}
			}
			panic("impossible")
		}

		{
			header, err := parseKernelMetricsHeader("shard")
			if err != nil {
				panic(err)
			}
			latencies, err := parseShardKernelLatencies(header)
			if err != nil {
				panic(err)
			}
			for i := range latencies {
				l.Info("%v: p50=%v p90=%v p99=%v", msgs.ShardMessageKind(latencies[i].Kind), p(&header, &latencies[i], 0.5), p(&header, &latencies[i], 0.9), p(&header, &latencies[i], 0.99))
			}
		}
		{
			header, err := parseKernelMetricsHeader("cdc")
			if err != nil {
				panic(err)
			}
			latencies, err := parseCDCKernelLatencies(header)
			if err != nil {
				panic(err)
			}
			for i := range latencies {
				l.Info("%v: p50=%v p90=%v p99=%v", msgs.CDCMessageKind(latencies[i].Kind), p(&header, &latencies[i], 0.5), p(&header, &latencies[i], 0.9), p(&header, &latencies[i], 0.99))
			}
		}
	}
	commands["kernel-latencies"] = commandSpec{
		flags: kernelLatenciesCmd,
		run:   kernelLatenciesRun,
	}

	defragFileCmd := flag.NewFlagSet("defrag", flag.ExitOnError)
	defragFilePath := defragFileCmd.String("path", "", "The directory or file to defrag")
	defragFileFrom := defragFileCmd.String("from", "", "If present, will not defrag files pointed at by edges created before this time.")
	defragFileRun := func() {
		c := getClient()
		dirInfoCache := client.NewDirInfoCache()
		bufPool := bufpool.NewBufPool()
		stats := &cleanup.DefragStats{}
		alert := l.NewNCAlert(0)
		alert.SetAppType(log.XMON_NEVER)
		id, _, parent, err := c.ResolvePathWithParent(l, *defragFilePath)
		if err != nil {
			panic(err)
		}
		if id.Type() == msgs.DIRECTORY {
			var startTime msgs.TernTime
			if *defragFileFrom != "" {
				t, err := time.Parse(time.RFC3339Nano, *defragFileFrom)
				if err != nil {
					panic(err)
				}
				startTime = msgs.MakeTernTime(t)
			}
			options := cleanup.DefragOptions{
				WorkersPerShard: 5,
				StartFrom:       startTime,
			}
			if err := cleanup.DefragFiles(l, c, bufPool, dirInfoCache, stats, alert, &options, *defragFilePath); err != nil {
				panic(err)
			}
		} else {
			if *defragFileFrom != "" {
				panic(fmt.Errorf("cannot provide -from with a file -path"))
			}
			if err := cleanup.DefragFile(l, c, bufPool, dirInfoCache, stats, alert, parent, id, *defragFilePath); err != nil {
				panic(err)
			}
		}
		l.Info("defrag stats: %+v", stats)
	}
	commands["defrag"] = commandSpec{
		flags: defragFileCmd,
		run:   defragFileRun,
	}

	defragSpansCmd := flag.NewFlagSet("defrag-spans", flag.ExitOnError)
	defragSpansPath := defragSpansCmd.String("path", "", "The directory or file to defrag")
	defragSpansRun := func() {
		c := getClient()
		dirInfoCache := client.NewDirInfoCache()
		bufPool := bufpool.NewBufPool()
		stats := &cleanup.DefragSpansStats{}
		alert := l.NewNCAlert(0)
		alert.SetAppType(log.XMON_NEVER)
		if err := cleanup.DefragSpans(l, c, bufPool, dirInfoCache, stats, alert, *defragSpansPath); err != nil {
			panic(err)
		}
		l.Info("defrag stats: %+v", stats)
	}
	commands["defrag-spans"] = commandSpec{
		flags: defragSpansCmd,
		run:   defragSpansRun,
	}

	resurrectFileCmd := flag.NewFlagSet("resurrect", flag.ExitOnError)
	resurrectFilePath := resurrectFileCmd.String("path", "", "The file to resurrect")
	resurrectFileList := resurrectFileCmd.String("list", "", "File with files to resurrect (one per line)")
	resurrectFileWorkers := resurrectFileCmd.Int("workers", 256, "")
	resurrectFileRun := func() {
		if (*resurrectFilePath == "" && *resurrectFileList == "") || (*resurrectFilePath != "" && *resurrectFileList != "") {
			panic(fmt.Errorf("must provide -path or -list"))
		}
		if *resurrectFilePath == "" && *resurrectFileList == "" {
			panic(fmt.Errorf("must provide -path or -list"))
		}
		if *resurrectFileWorkers < 1 {
			panic(fmt.Errorf("workers must be > 0"))
		}
		c := getClient()
		t0 := time.Now()
		ch := make(chan string, *resurrectFileWorkers*4)
		var wg sync.WaitGroup
		wg.Add(*resurrectFileWorkers)
		for i := 0; i < *resurrectFileWorkers; i++ {
			go func() {
				defer wg.Done()
				for {
					p, more := <-ch
					if !more {
						return
					}
					dirId, err := c.ResolvePath(l, path.Dir(p))
					if err != nil {
						panic(err)
					}
					req := msgs.FullReadDirReq{
						DirId:     dirId,
						Flags:     msgs.FULL_READ_DIR_CURRENT | msgs.FULL_READ_DIR_BACKWARDS | msgs.FULL_READ_DIR_SAME_NAME,
						StartName: path.Base(p),
					}
					resp := msgs.FullReadDirResp{}
					if err := c.ShardRequest(l, dirId.Shard(), &req, &resp); err != nil {
						panic(err)
					}
					if len(resp.Results) < 2 {
						l.Info("%q: found < 2 edges, skipping: %+v", p, resp.Results)
						continue
					}
					// if we already have a current edge, no need to do anything
					if resp.Results[0].Current {
						l.Info("%q: a current edge already exists, skipping", p)
						continue
					}
					// otherwise, we expect a deleted edge, and then an owned edge
					if resp.Results[0].TargetId.Id() != msgs.NULL_INODE_ID {
						l.Info("%q: last edge is not a deletion edge, skipping: %+v", p, resp.Results[0])
						continue
					}
					if !resp.Results[1].TargetId.Extra() {
						l.Info("%q: second to last edge is not an owned edge, skipping: %+v", p, resp.Results[1])
						continue
					}
					// We've got everything we need, do the resurrection
					resurrectReq := msgs.SameDirectoryRenameSnapshotReq{
						TargetId:        resp.Results[1].TargetId.Id(),
						DirId:           dirId,
						OldName:         path.Base(p),
						OldCreationTime: resp.Results[1].CreationTime,
						NewName:         path.Base(p),
					}
					if err := c.ShardRequest(l, dirId.Shard(), &resurrectReq, &msgs.SameDirectoryRenameSnapshotResp{}); err != nil {
						panic(fmt.Errorf("could not resurrect %q: %w", p, err))
					}
					l.Info("%q: resurrected", p)
				}
			}()
		}
		if *resurrectFilePath != "" {
			ch <- *resurrectFilePath
		}
		seenFiles := 0
		if *resurrectFileList != "" {
			file, err := os.Open(*resurrectFileList)
			if err != nil {
				panic(err)
			}
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				if seenFiles%30_000 == 0 {
					l.Info("Went through %v files (%0.2f files/sec)", seenFiles, 1000.0*float64(seenFiles)/float64(time.Since(t0).Milliseconds()))
				}
				seenFiles++
				if strings.TrimSpace(scanner.Text()) == "" {
					continue
				}
				ch <- scanner.Text()
			}
		}
		close(ch)
		wg.Wait()
	}
	commands["resurrect"] = commandSpec{
		flags: resurrectFileCmd,
		run:   resurrectFileRun,
	}

	resolveSamplePathsCmd := flag.NewFlagSet("resolve-sample-paths", flag.ExitOnError)
	resolveSamplePathsRun := func() {
		resolver := filesamples.NewPathResolver(getClient(), l)
		resolver.ResolveFilePaths(os.Stdin, os.Stdout)
	}
	commands["resolve-sample-paths"] = commandSpec{
		flags: resolveSamplePathsCmd,
		run:   resolveSamplePathsRun,
	}

	flag.Parse()

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
	spec.flags.Parse(flag.Args()[1:])
	noRunawayArgs(spec.flags)

	spec.run()
}
