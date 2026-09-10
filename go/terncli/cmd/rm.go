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
	"path"
	"regexp"
	"sync/atomic"
	"time"
)

func NewRm() SpecWithClient {
	rmCmd := flag.NewFlagSet("rm", flag.ExitOnError)
	rmPath := rmCmd.String("path", "", "The directory path to delete files from (recursively).")
	rmFilter := rmCmd.String("filter", "", "Optional regex to match against the full file path. Only matching files will be deleted.")
	rmWorkersPerShard := rmCmd.Int("workers-per-shard", 5, "Number of parallel workers per shard.")
	rmDryRun := rmCmd.Bool("dry-run", false, "If set, will only print the files that would be deleted without actually deleting them.")
	rmRun := func(runtime RuntimeWithClient) {
		l := runtime.Log
		if *rmPath == "" {
			fmt.Fprintf(os.Stderr, "must provide -path\n")
			os.Exit(2)
		}
		re := regexp.MustCompile(`.*`)
		if *rmFilter != "" {
			var err error
			re, err = regexp.Compile(*rmFilter)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to compile regex filter: %v\n", err)
				os.Exit(2)
			}
		}
		if !*rmDryRun {
			filterMsg := ""
			if *rmFilter != "" {
				filterMsg = fmt.Sprintf(" matching filter %q", *rmFilter)
			}
			fmt.Printf("Will recursively delete all files under %q%s.\n", *rmPath, filterMsg)
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
		c := runtime.Client
		var numDeleted uint64
		var numSkipped uint64
		var numErrors uint64
		startedAt := time.Now()
		err := client.Parwalk(
			l,
			c,
			&client.ParwalkOptions{
				WorkersPerShard: *rmWorkersPerShard,
			},
			*rmPath,
			func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
				if id.Type() == msgs.DIRECTORY {
					return nil
				}
				fullPath := path.Join(parentPath, name)
				if !re.MatchString(fullPath) {
					atomic.AddUint64(&numSkipped, 1)
					return nil
				}
				if *rmDryRun {
					l.Info("[dry-run] would delete %v %q", id, fullPath)
					atomic.AddUint64(&numDeleted, 1)
					return nil
				}
				unlinkReq := msgs.SoftUnlinkFileReq{
					OwnerId:      parent,
					FileId:       id,
					Name:         name,
					CreationTime: creationTime,
				}
				if err := c.ShardRequest(l, parent.Shard(), &unlinkReq, &msgs.SoftUnlinkFileResp{}); err != nil {
					l.ErrorNoAlert("could not delete %q (%v): %v", fullPath, id, err)
					atomic.AddUint64(&numErrors, 1)
					return nil
				}
				deleted := atomic.AddUint64(&numDeleted, 1)
				if deleted%100000 == 0 {
					l.Info("deleted %v files, skipped %v, errors %v (%0.2f files/s)", deleted, numSkipped, numErrors, float64(deleted)/time.Since(startedAt).Seconds())
				}
				return nil
			},
		)
		if err != nil {
			panic(err)
		}
		l.Info("rm finished: deleted %v files, skipped %v, errors %v", numDeleted, numSkipped, numErrors)
	}
	return SpecWithClient{
		Flags: rmCmd,
		Run:   rmRun,
	}
}
