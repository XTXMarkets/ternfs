// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cleanup

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

type ZeroBlockServiceFilesStats struct {
	// zero block service stuff
	ZeroBlockServiceFilesRemoved uint64
	LastCycleDurationNs          uint64
}

func CollectZeroBlockServiceFiles(log *log.Logger, c *client.Client, stats *ZeroBlockServiceFilesStats) error {
	log.Info("starting to collect block services files")
	start := time.Now()
	var wg sync.WaitGroup
	errs := make([]error, 256)
	for shid := 0; shid < 256; shid++ {
		wg.Add(1)
		go func(shid int) {
			defer wg.Done()
			var req msgs.RemoveZeroBlockServiceFilesReq
			var resp msgs.RemoveZeroBlockServiceFilesResp
			for i := 0; ; i++ {
				if i > 0 && req.StartBlockService == 0 && req.StartFile == msgs.NULL_INODE_ID {
					break
				}
				if err := c.ShardRequest(log, msgs.ShardId(shid), &req, &resp); err != nil {
					errs[shid] = fmt.Errorf("could not remove zero block services in shard %v: %w", shid, err)
					return
				}
				req.StartBlockService = resp.NextBlockService
				req.StartFile = resp.NextFile
				atomic.AddUint64(&stats.ZeroBlockServiceFilesRemoved, resp.Removed)
			}
		}(shid)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	atomic.StoreUint64(&stats.LastCycleDurationNs, uint64(time.Since(start).Nanoseconds()))
	log.Info("finished collecting zero block service files: %+v", stats)
	return nil
}
