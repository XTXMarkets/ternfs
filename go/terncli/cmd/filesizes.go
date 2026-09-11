// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"path"
	"sync"
	"sync/atomic"
)

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

func NewFileSizes() Command {
	fileSizesCmd := flag.NewFlagSet("file-sizes", flag.ExitOnError)
	fileSizesBrief := fileSizesCmd.Bool("brief", false, "")
	fileSizesRun := func(runtime *Runtime) {
		l := runtime.Log
		if *fileSizesBrief {
			outputBriefFileSizes(l, runtime.Client())
		} else {
			outputFullFileSizes(l, runtime.Client())
		}
	}
	return Command{
		Flags: fileSizesCmd,
		Run:   fileSizesRun,
	}
}
