// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cleanup

import (
	"fmt"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
	lrecover "github.com/XTXMarkets/ternfs/go/core/recover"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"sync"
	"sync/atomic"
)

type DestructFilesStats struct {
	VisitedFiles     uint64
	DestructedFiles  uint64
	DestructedSpans  uint64
	SkippedFiles     uint64
	SkippedSpans     uint64
	DestructedBlocks uint64
	FailedFiles      uint64
	Cycles           [256]uint32
}

type DestructFilesState struct {
	Stats             DestructFilesStats
	WorkersQueuesSize [256]uint64
	Cursors           [256]msgs.InodeId
}

type CouldNotEraseBlocksInBlockServices []msgs.BlockServiceId

func (c CouldNotEraseBlocksInBlockServices) Error() string {
	return fmt.Sprintf("could not erase blocks in block services: %+v", []msgs.BlockServiceId(c))
}

func DestructFile(
	log *log.Logger,
	c *client.Client,
	stats *DestructFilesStats,
	id msgs.InodeId,
	deadline msgs.TernTime,
	cookie [8]byte,
) error {
	log.Debug("%v: destructing file, cookie=%v", id, cookie)
	// we've already checked this, but it might have expired
	// while in the queue
	now := msgs.Now()
	if now < deadline {
		log.Debug("%v: deadline not expired (deadline=%v, now=%v), not destructing", id, deadline, now)
		atomic.AddUint64(&stats.SkippedFiles, 1)
		return nil
	}

	initReq := msgs.RemoveSpanInitiateReq{
		FileId: id,
		Cookie: cookie,
	}
	initResp := msgs.RemoveSpanInitiateResp{}
	certifyReq := msgs.RemoveSpanCertifyReq{
		FileId: id,
		Cookie: cookie,
	}
	certifyResp := msgs.RemoveSpanCertifyResp{}
	atomic.AddUint64(&stats.VisitedFiles, 1)
	for {
		err := c.ShardRequest(log, id.Shard(), &initReq, &initResp)
		if err == msgs.FILE_EMPTY {
			break // TODO: kinda ugly to rely on this for control flow...
		}
		if err != nil {
			return fmt.Errorf("%v: could not initiate span removal: %w", id, err)
		}
		couldNotReachBlockServices := []msgs.BlockServiceId{}
		if len(initResp.Blocks) > 0 {
			certifyReq.ByteOffset = initResp.ByteOffset
			certifyReq.Proofs = make([]msgs.BlockProof, len(initResp.Blocks))
			var proof [8]byte
			for i := range initResp.Blocks {
				block := &initResp.Blocks[i]
				if block.BlockServiceFlags.HasAny(msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED) {
					proof, err = c.EraseDecommissionedBlock(block)
					if err != nil {
						return err
					}
				} else {
					// There's no point trying to erase blocks for stale block services -- they're
					// almost certainly temporarly offline, and we'll be stuck forever since in GC we run
					// with infinite timeout. Just skip.
					if block.BlockServiceFlags.HasAny(msgs.TERNFS_BLOCK_SERVICE_STALE) {
						log.Debug("skipping block %v in file %v since its block service %v is stale", block.BlockId, id, block.BlockServiceId)
						couldNotReachBlockServices = append(couldNotReachBlockServices, block.BlockServiceId)
						continue
					}
					proof, err = c.EraseBlock(log, block)
					if err != nil {
						if errIsTolerable(c, block.BlockServiceId, err) {
							log.Info("tolerable erase failure for block %v in block service %v while destructing file %v: %v", block.BlockId, block.BlockServiceId, id, err)
						} else {
							log.RaiseAlert("could not erase block in block service %v while destructing file %v: %v", block.BlockServiceId, id, err)
						}
						couldNotReachBlockServices = append(couldNotReachBlockServices, block.BlockServiceId)
						continue
					}
				}
				certifyReq.Proofs[i].BlockId = block.BlockId
				certifyReq.Proofs[i].Proof = proof
				atomic.AddUint64(&stats.DestructedBlocks, 1)
			}
			if len(couldNotReachBlockServices) == 0 {
				err = c.ShardRequest(log, id.Shard(), &certifyReq, &certifyResp)
				if err != nil {
					return fmt.Errorf("%v: could not certify span removal %+v: %w", id, certifyReq, err)
				}
				atomic.AddUint64(&stats.DestructedSpans, 1)
			} else {
				atomic.AddUint64(&stats.SkippedSpans, 1)
				// We need to return early -- we won't make progress in the file because
				// we haven't removed the span.
				return CouldNotEraseBlocksInBlockServices(couldNotReachBlockServices)
			}
		}
	}
	err := c.ShardRequest(log, id.Shard(), &msgs.RemoveInodeReq{Id: id}, &msgs.RemoveInodeResp{})
	if err != nil {
		return fmt.Errorf("%v: could not remove transient file inode after removing spans: %w", id, err)
	}
	atomic.AddUint64(&stats.DestructedFiles, 1)
	return nil
}

type destructFileRequest struct {
	id       msgs.InodeId
	deadline msgs.TernTime
	cookie   [8]byte
}

func destructFilesWorker(
	log *log.Logger,
	c *client.Client,
	stats *DestructFilesState,
	shid msgs.ShardId,
	workersChan chan *destructFileRequest,
	terminateChan chan any,
) {
	for {
		req, ok := <-workersChan
		if !ok {
			log.Debug("destruct files worker terminating")
			return
		}
		atomic.StoreUint64(&stats.WorkersQueuesSize[shid], uint64(len(workersChan)))
		if err := DestructFile(log, c, &stats.Stats, req.id, req.deadline, req.cookie); err != nil {
			// this is OK, we'll get there eventually
			_, isCouldNotReach := err.(CouldNotEraseBlocksInBlockServices)
			if isCouldNotReach {
				log.Debug("could not reach block services when destructing %v, ignoring: %+v", req.id, err)
			} else {
				log.Info("could not destruct file %v, terminating: %v", req.id, err)
				select {
				case terminateChan <- err:
				default:
				}
				// we don't return here we wait for our channel to be closed
			}
		}
	}
}

func destructFilesScraper(
	log *log.Logger,
	c *client.Client,
	opts *DestructFilesOptions,
	state *DestructFilesState,
	terminateChan chan<- any,
	shid msgs.ShardId,
	workerChan chan<- *destructFileRequest,
) {
	// regardless how we exit we want to close worker channels
	defer close(workerChan)
	req := &msgs.VisitTransientFilesReq{
		BeginId: state.Cursors[shid],
	}
	resp := &msgs.VisitTransientFilesResp{}
	for {
		log.Debug("visiting files with %+v", req)
		err := c.ShardRequest(log, shid, req, resp)
		if err != nil {
			log.Info("could not visit transient files: %v", err)
			select {
			case terminateChan <- err:
			default:
			}
			return
		}
		now := msgs.Now()
		for ix := range resp.Files {
			file := &resp.Files[ix]
			if opts.NumInstances > 1 && (uint64(file.Id)>>8)%opts.NumInstances != opts.InstanceIdx {
				continue
			}
			if now < file.DeadlineTime {
				log.Debug("%v: deadline not expired (deadline=%v, now=%v), not destructing", file.Id, file.DeadlineTime, now)
				continue
			}
			workerChan <- &destructFileRequest{
				id:       file.Id,
				deadline: file.DeadlineTime,
				cookie:   file.Cookie,
			}
		}
		state.Cursors[shid] = resp.NextId
		req.BeginId = resp.NextId
		if req.BeginId == 0 {
			log.Debug("file scraping done for shard %v", shid)
			return
		}
	}
}

type DestructFilesOptions struct {
	NumWorkersPerShard int
	WorkersQueueSize   int
	// To scale destruct-files horizontally, run multiple terngc instances with
	// the same NumInstances but distinct InstanceIdx in [0, NumInstances). Each
	// instance only destructs files where (fileId>>8) % NumInstances == InstanceIdx,
	// matching the migrator's fileId sharding.
	NumInstances uint64
	InstanceIdx  uint64
}

func DestructFiles(
	log *log.Logger,
	c *client.Client,
	opts *DestructFilesOptions,
	stats *DestructFilesState,
	shid msgs.ShardId,
) error {
	if opts.NumWorkersPerShard <= 0 {
		panic(fmt.Errorf("the number of workers should be positive, got %v", opts.NumWorkersPerShard))
	}
	if opts.NumInstances > 0 && opts.InstanceIdx >= opts.NumInstances {
		panic(fmt.Errorf("instance index %v should be less than the number of instances %v", opts.InstanceIdx, opts.NumInstances))
	}
	terminateChan := make(chan any, 1)
	defer close(terminateChan)
	workersChan := make(chan *destructFileRequest, opts.WorkersQueueSize)

	log.Info("destructing files in shard %v", shid)

	go func() {
		defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
		destructFilesScraper(log, c, opts, stats, terminateChan, shid, workersChan)
	}()

	go func() {
		var workersWg sync.WaitGroup
		workersWg.Add(opts.NumWorkersPerShard)
		for j := 0; j < opts.NumWorkersPerShard; j++ {
			go func() {
				defer workersWg.Done()
				defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
				destructFilesWorker(log, c, stats, shid, workersChan, terminateChan)
			}()
		}
		workersWg.Wait()
		terminateChan <- nil
	}()

	var err error
	for {
		workerErr := <-terminateChan
		if workerErr != nil {
			// remember first error
			if err == nil {
				err = workerErr.(error)
			}
			continue
		}
		break
	}

	log.Info("all workers terminated, we're done with shard %v", shid)
	atomic.AddUint32(&stats.Stats.Cycles[shid], 1)
	if err != nil {
		log.Info("could not destruct files in shard %v: %v", shid, err)
	}
	return err
}

func DestructFilesInAllShards(
	log *log.Logger,
	c *client.Client,
	opts *DestructFilesOptions,
	stats *DestructFilesState,
) error {
	terminateChan := make(chan any, 1)

	var wg sync.WaitGroup
	wg.Add(256)
	for i := 0; i < 256; i++ {
		shid := msgs.ShardId(i)
		go func() {
			defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
			if err := DestructFiles(log, c, opts, stats, shid); err != nil {
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
		log.Info("could not destruct files: %v", err)
		return err.(error)
	}
}
