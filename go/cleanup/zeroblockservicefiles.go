// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cleanup

import (
	"fmt"
	"sync/atomic"
	"xtx/ternfs/client"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

type ZeroBlockServiceFilesStats struct {
	// zero block service stuff
	ZeroBlockServiceFilesRemoved uint64
}

func CollectZeroBlockServiceFiles(log *log.Logger, c *client.Client, stats *ZeroBlockServiceFilesStats) error {
	log.Info("starting to collect block services files")
	reqs := make([]msgs.RemoveZeroBlockServiceFilesReq, 256)
	resps := make([]msgs.RemoveZeroBlockServiceFilesResp, 256)
	for i := 0; ; i++ {
		allDone := true
		for shid := 0; shid < 256; shid++ {
			req := &reqs[shid]
			resp := &resps[shid]
			if i > 0 && req.StartBlockService == 0 && req.StartFile == msgs.NULL_INODE_ID {
				continue
			}
			allDone = false
			err := c.ShardRequest(log, msgs.ShardId(shid), req, resp)
			if err != nil {
				return fmt.Errorf("could not remove zero block services: %w", err)
			}
			req.StartBlockService = resp.NextBlockService
			req.StartFile = resp.NextFile
			atomic.AddUint64(&stats.ZeroBlockServiceFilesRemoved, resp.Removed)
		}
		if allDone {
			break
		}
	}
	log.Info("finished collecting zero block service files: %+v", stats)
	return nil
}
