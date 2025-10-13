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
	lrecover "xtx/ternfs/core/recover"
	"xtx/ternfs/core/timing"
	"xtx/ternfs/msgs"
)

type CollectDirectoriesStats struct {
	VisitedDirectories    uint64
	VisitedEdges          uint64
	CollectedEdges        uint64
	DestructedDirectories uint64
	Cycles                [256]uint32
}

type CollectDirectoriesState struct {
	Stats             CollectDirectoriesStats
	WorkersQueuesSize [256]uint64
	Cursors           [256]msgs.InodeId
}

func HardUnlinkFile(
	log *log.Logger,
	c *client.Client,
	dirId msgs.InodeId,
	fileId msgs.InodeId,
	name string,
	creationTime msgs.TernTime,
) (err error) {
	if dirId.Shard() == fileId.Shard() {
		// same shard, we can delete directly. We also know that this is not a directory (it's an
		// owned, but snapshot edge)
		log.Debug("%v: removing owned snapshot edge fileId=%v name=%q creationTime=%v", dirId, fileId, name, creationTime)
		req := msgs.SameShardHardFileUnlinkReq{
			OwnerId:      dirId,
			TargetId:     fileId,
			Name:         name,
			CreationTime: creationTime,
		}
		err = c.ShardRequest(log, dirId.Shard(), &req, &msgs.SameShardHardFileUnlinkResp{})
	} else {
		// different shard, we need to go through the CDC
		log.Debug("%v: removing cross-shard owned snapshot edge fileId=%v name=%q creationTime=%v", dirId, fileId, name, creationTime)
		req := msgs.CrossShardHardUnlinkFileReq{
			OwnerId:      dirId,
			TargetId:     fileId,
			Name:         name,
			CreationTime: creationTime,
		}
		err = c.CDCRequest(log, &req, &msgs.CrossShardHardUnlinkFileResp{})
	}
	return err
}

// returns whether all the edges were removed
func applyPolicy(
	log *log.Logger,
	c *client.Client,
	stats *CollectDirectoriesStats,
	dirId msgs.InodeId,
	policy *msgs.SnapshotPolicy,
	edges []msgs.Edge,
	minEdgeAge time.Duration,
) (bool, error) {
	log.Debug("%v: about to apply policy %+v for name %s", dirId, policy, edges[0].Name)
	atomic.AddUint64(&stats.VisitedEdges, uint64(len(edges)))
	now := msgs.Now()
	toCollect := edgesToRemove(dirId, policy, now, edges, minEdgeAge)
	log.Debug("%v: will remove %d edges out of %d", dirId, toCollect, len(edges))
	for _, edge := range edges[:toCollect] {
		if edge.Current {
			panic(fmt.Errorf("unexpected current edge to %v when trying to apply policy on dir %v", edge.TargetId, dirId))
		}
		var err error
		if edge.TargetId.Extra() { // the extra bit on a snapshot edge means owned
			if edge.TargetId.Id().Type() == msgs.DIRECTORY {
				panic(fmt.Errorf("unexpected owned directory owner=%v edge=%+v", dirId, edge))
			}
			err = HardUnlinkFile(log, c, dirId, edge.TargetId.Id(), edge.Name, edge.CreationTime)
		} else {
			// non-owned edge, we can just kill it without worrying about much.
			log.Debug("%v: removing non-owned edge %+v", dirId, edge)
			req := msgs.RemoveNonOwnedEdgeReq{
				DirId:        dirId,
				TargetId:     edge.TargetId.Id(),
				Name:         edge.Name,
				CreationTime: edge.CreationTime,
			}
			err = c.ShardRequest(log, dirId.Shard(), &req, &msgs.RemoveNonOwnedEdgeResp{})
		}
		if err != nil && err != msgs.FILE_NOT_FOUND {
			return false, fmt.Errorf("error while collecting edge %+v in directory %v: %w", edge, dirId, err)
		}
		atomic.AddUint64(&stats.CollectedEdges, 1)
	}
	return toCollect == len(edges), nil
}

func CollectDirectory(log *log.Logger, c *client.Client, dirInfoCache *client.DirInfoCache, stats *CollectDirectoriesStats, dirId msgs.InodeId, minEdgeAge time.Duration) error {
	log.Debug("%v: collecting", dirId)
	atomic.AddUint64(&stats.VisitedDirectories, 1)

	policy := &msgs.SnapshotPolicy{}
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, dirId, policy); err != nil {
		return err
	}
	dirEdges := make(map[string][]msgs.Edge)
	req := msgs.FullReadDirReq{
		DirId: dirId,
	}
	resp := msgs.FullReadDirResp{}
	for {
		err := c.ShardRequest(log, dirId.Shard(), &req, &resp)
		if err != nil {
			return err
		}
		log.Debug("%v: got %d edges in response", dirId, len(resp.Results))
		for _, result := range resp.Results {
			if _, found := dirEdges[result.Name]; !found {
				dirEdges[result.Name] = []msgs.Edge{}
			}
			dirEdges[result.Name] = append(dirEdges[result.Name], result)
		}
		if resp.Next.StartName == "" {
			break
		}
		req.Flags = 0
		if resp.Next.Current {
			req.Flags = msgs.FULL_READ_DIR_CURRENT
		}
		req.StartName = resp.Next.StartName
		req.StartTime = resp.Next.StartTime
	}
	hasEdges := false
	for _, edges := range dirEdges {
		allRemoved, err := applyPolicy(log, c, stats, dirId, policy, edges, minEdgeAge)
		if err != nil {
			return err
		}
		hasEdges = hasEdges || !allRemoved
	}
	if !hasEdges && dirId != msgs.ROOT_DIR_INODE_ID {
		statReq := msgs.StatDirectoryReq{
			Id: dirId,
		}
		statResp := msgs.StatDirectoryResp{}
		err := c.ShardRequest(log, dirId.Shard(), &statReq, &statResp)
		if err != nil {
			return fmt.Errorf("error while stat'ing directory: %w", err)
		}
		if statResp.Owner == msgs.NULL_INODE_ID {
			log.Debug("%v: removing directory inode, since it has no edges and no owner", dirId)
			req := msgs.HardUnlinkDirectoryReq{
				DirId: dirId,
			}
			err := c.CDCRequest(log, &req, &msgs.HardUnlinkDirectoryResp{})
			if err != nil {
				return fmt.Errorf("error while trying to remove directory inode: %w", err)
			}
			atomic.AddUint64(&stats.DestructedDirectories, 1)
		}
	}
	return nil
}

func collectDirectoriesWorker(
	log *log.Logger,
	c *client.Client,
	dirInfoCache *client.DirInfoCache,
	rateLimit *timing.RateLimit,
	stats *CollectDirectoriesState,
	shid msgs.ShardId,
	workersChan chan msgs.InodeId,
	terminateChan chan any,
	minEdgeAge time.Duration,
) {
	for {
		if rateLimit != nil {
			rateLimit.Acquire()
		}
		dir, ok := <-workersChan
		if !ok {
			log.Debug("collect directories worker for shard %v terminated", shid)
			return
		}
		log.Debug("received worker request for shard %v len=%v cap=%v", shid, len(workersChan), cap(workersChan))
		atomic.StoreUint64(&stats.WorkersQueuesSize[shid], uint64(len(workersChan)))
		if err := CollectDirectory(log, c, dirInfoCache, &stats.Stats, dir, minEdgeAge); err != nil {
			log.Info("could not destruct directory %v, terminating: %v", dir, err)
			select {
			case terminateChan <- err:
			default:
			}
			return
		}
	}
}

func collectDirectoriesScraper(
	log *log.Logger,
	c *client.Client,
	state *CollectDirectoriesState,
	shid msgs.ShardId,
	workerChan chan msgs.InodeId,
	terminateChan chan any,
) {
	req := &msgs.VisitDirectoriesReq{
		BeginId: state.Cursors[shid],
	}
	resp := &msgs.VisitDirectoriesResp{}
	for {
		err := c.ShardRequest(log, shid, req, resp)
		if err != nil {
			select {
			case terminateChan <- fmt.Errorf("could not visit directories: %w", err):
			default:
			}
			return
		}
		for _, id := range resp.Ids {
			workerChan <- id
		}
		state.Cursors[shid] = resp.NextId
		req.BeginId = resp.NextId
		if req.BeginId == 0 {
			log.Debug("directory scraping done for shard %v, terminating workers", shid)
			close(workerChan)
			return
		}
	}
}

type CollectDirectoriesOpts struct {
	NumWorkersPerShard int
	WorkersQueueSize   int
}

func CollectDirectories(
	log *log.Logger,
	c *client.Client,
	dirInfoCache *client.DirInfoCache,
	rateLimit *timing.RateLimit,
	opts *CollectDirectoriesOpts,
	state *CollectDirectoriesState,
	shid msgs.ShardId,
	minEdgeAge time.Duration,
) error {
	log.Info("starting to collect directories in shard %v", shid)

	if opts.NumWorkersPerShard <= 0 {
		panic(fmt.Errorf("bad NumWorkersPerShard %v", opts.NumWorkersPerShard))
	}

	terminateChan := make(chan any, 1)
	workerChan := make(chan msgs.InodeId, opts.WorkersQueueSize)

	go func() {
		defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
		collectDirectoriesScraper(log, c, state, shid, workerChan, terminateChan)
	}()

	var workersWg sync.WaitGroup
	workersWg.Add(opts.NumWorkersPerShard)
	for j := 0; j < opts.NumWorkersPerShard; j++ {
		go func() {
			defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
			collectDirectoriesWorker(log, c, dirInfoCache, rateLimit, state, shid, workerChan, terminateChan, minEdgeAge)
			workersWg.Done()
		}()
	}
	go func() {
		workersWg.Wait()
		log.Info("all workers terminated for shard %v, we're done", shid)
		atomic.AddUint32(&state.Stats.Cycles[shid], 1)
		terminateChan <- nil
	}()

	err := <-terminateChan
	if err == nil {
		return nil
	} else {
		log.Info("could not collect directories in shard %v: %v", shid, err)
		return err.(error)
	}
}

func CollectDirectoriesInAllShards(
	log *log.Logger,
	c *client.Client,
	dirInfoCache *client.DirInfoCache,
	rateLimit *timing.RateLimit,
	opts *CollectDirectoriesOpts,
	state *CollectDirectoriesState,
	minEdgeAge time.Duration,
) error {
	terminateChan := make(chan any, 1)

	var wg sync.WaitGroup
	wg.Add(256)
	for i := 0; i < 256; i++ {
		shid := msgs.ShardId(i)
		go func() {
			defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
			if err := CollectDirectories(log, c, dirInfoCache, rateLimit, opts, state, shid, minEdgeAge); err != nil {
				panic(err)
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		terminateChan <- nil
	}()

	err := <-terminateChan
	if err == nil {
		return nil
	} else {
		log.Info("could not collect directories: %v", err)
		return err.(error)
	}
}
