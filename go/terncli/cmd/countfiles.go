// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"sync"
	"sync/atomic"
	"time"
)

func NewCountFiles() SpecWithClient {
	countFilesCmd := flag.NewFlagSet("count-files", flag.ExitOnError)
	countFilesRun := func(runtime RuntimeWithClient) {
		l := runtime.Log
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
					if err := runtime.Client.ShardRequest(l, shid, &req, &resp); err != nil {
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
	return SpecWithClient{
		Flags: countFilesCmd,
		Run:   countFilesRun,
	}
}
