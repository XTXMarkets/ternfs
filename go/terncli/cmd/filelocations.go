// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func NewFileLocations() Command {
	fileLocationsCmd := flag.NewFlagSet("file-locations", flag.ExitOnError)
	fileLocationsId := fileLocationsCmd.Uint64("id", 0, "ID of the file to query")
	fileLocationsRun := func(runtime *Runtime) {
		l := runtime.Log
		id := msgs.InodeId(*fileLocationsId)
		c := runtime.Client()
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
	return Command{
		Flags: fileLocationsCmd,
		Run:   fileLocationsRun,
	}
}
