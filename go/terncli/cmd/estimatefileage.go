// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"math"
)

func NewEstimateFileAge() Command {
	estimateFileAgeCmd := flag.NewFlagSet("estimate-file-age", flag.ExitOnError)
	estimateFileAgeId := estimateFileAgeCmd.Uint64("id", 0, "ID of the file to estimage age for")
	estimateFileAgePath := estimateFileAgeCmd.String("path", "", "Path of the file to estimate age for (alternative to -id)")
	estimateFileAgeRun := func(runtime *Runtime) {
		l := runtime.Log
		c := runtime.Client()
		id := msgs.InodeId(*estimateFileAgeId)
		if id == 0 {
			if *estimateFileAgePath == "" {
				panic(fmt.Errorf("either -id or -path must be specified"))
			}
			var err error
			if id, err = c.ResolvePath(l, *estimateFileAgePath); err != nil {
				panic(fmt.Errorf("could not resolve path %v: %v", *estimateFileAgePath, err))
			}
		}
		if id.Type() != msgs.FILE {
			panic(fmt.Errorf("inode id %v is not a file", id))
		}
		fileSpansReq := msgs.FileSpansReq{
			FileId:     id,
			ByteOffset: 0,
		}
		fileSpansResp := msgs.FileSpansResp{}
		oldestBlock := uint64(math.MaxUint64)
		oldestByLoc := map[msgs.Location]uint64{}
		var locOrder []msgs.Location

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
					for _, block := range loc.Blocks {
						oldestBlock = min(oldestBlock, uint64(block.BlockId))
						if _, seen := oldestByLoc[loc.LocationId]; !seen {
							oldestByLoc[loc.LocationId] = math.MaxUint64
							locOrder = append(locOrder, loc.LocationId)
						}
						oldestByLoc[loc.LocationId] = min(oldestByLoc[loc.LocationId], uint64(block.BlockId))
					}
				}
			}
			if fileSpansResp.NextOffset == 0 {
				break
			}
			fileSpansReq.ByteOffset = fileSpansResp.NextOffset
		}
		if oldestBlock == math.MaxUint64 {
			statFileReq := msgs.StatFileReq{Id: id}
			statFileResp := msgs.StatFileResp{}
			if err := c.ShardRequest(l, id.Shard(), &statFileReq, &statFileResp); err != nil {
				panic(err)
			}
			mtime := statFileResp.Mtime
			l.Info("File %v has no blocks to use in file age estimation, returning mtime %v", id, msgs.TernTime(mtime))
			return
		} else {
			l.Info("Estimated file age %v, %v", id, msgs.TernTime(oldestBlock))
			for _, loc := range locOrder {
				l.Info("  location %v: %v", loc, msgs.TernTime(oldestByLoc[loc]))
			}
		}
	}

	return Command{
		Flags: estimateFileAgeCmd,
		Run:   estimateFileAgeRun,
	}
}
