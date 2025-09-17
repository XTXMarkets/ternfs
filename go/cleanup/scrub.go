// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cleanup

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"xtx/ternfs/cleanup/scratch"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/log"
	lrecover "xtx/ternfs/core/recover"
	"xtx/ternfs/core/timing"
	"xtx/ternfs/msgs"
)

type ScrubState struct {
	Migrate              MigrateStats
	CheckedBlocks        uint64
	CheckedBytes         uint64
	WorkersQueuesSize    [256]uint64
	CheckQueuesSize      [256]uint64
	DecommissionedBlocks uint64
	Cursors              [256]msgs.InodeId
	Cycles               [256]uint32
}

type ScrubOptions struct {
	NumWorkersPerShard int // how many goroutienes should be sending check request to the block services
	WorkersQueueSize   int
}

func badBlockError(err error) bool {
	return err == msgs.BAD_BLOCK_CRC || err == msgs.BLOCK_NOT_FOUND || err == msgs.BLOCK_IO_ERROR_FILE
}

func scrubFileInternal(
	log *log.Logger,
	c *client.Client,
	bufPool *bufpool.BufPool,
	stats *ScrubState,
	timeStats *timeStats,
	progressReportAlert *log.XmonNCAlert,
	scratchFile scratch.ScratchFile,
	file msgs.InodeId,
) error {
	log.Debug("starting to scrub file %v", file)

	badBlock := func(blockService *msgs.BlockService, blockSize uint32, block *msgs.FetchedBlock) (bool, error) {
		err := c.CheckBlock(log, blockService, block.BlockId, blockSize, block.Crc)
		if badBlockError(err) {
			log.ErrorNoAlert("found bad block, block service %v, block %v: %v", blockService.Id, block.BlockId, err)
			return true, nil
		}
		return false, err
	}
	return migrateBlocksInFileGeneric(log, c, bufPool, &stats.Migrate, timeStats, progressReportAlert, "scrubbed", badBlock, scratchFile, file)
}

type scrubRequest struct {
	file         msgs.InodeId
	blockService msgs.BlockService
	block        msgs.BlockId
	size         uint32
	crc          msgs.Crc
}

func scrubWorker(
	log *log.Logger,
	c *client.Client,
	opts *ScrubOptions,
	stats *ScrubState,
	rateLimit *timing.RateLimit,
	shid msgs.ShardId,
	workerChan chan *scrubRequest,
	terminateChan chan any,
	scratchFile scratch.ScratchFile,
	scrubbingMu *sync.Mutex,
	migratingFiles map[msgs.InodeId]struct{},
	migratingFilesMu *sync.RWMutex,
) {
	bufPool := bufpool.NewBufPool()
	blockNotFoundAlert := log.NewNCAlert(0)
	defer log.ClearNC(blockNotFoundAlert)
	for {
		req, ok := <-workerChan
		if !ok {
			log.Debug("worker for shard %v terminating", shid)
			return
		}
		migratingFilesMu.RLock()
		_, ok = migratingFiles[req.file]
		migratingFilesMu.RUnlock()
		if ok {
			// no point in checking this block as file is being migrated and all blocks will be checked in process
			continue
		}
		// If we don't expect the block to be read, do not even bother, it might
		// be some known problem (i.e. NR because of temporary maintenance)
		if !req.blockService.Flags.CanRead() {
			log.Debug("skipping block %v in file %v since given its block service %v flags %v", req.block, req.file, req.blockService.Id, req.blockService.Flags)
			continue
		}
		if rateLimit != nil {
			rateLimit.Acquire()
		}
		atomic.StoreUint64(&stats.WorkersQueuesSize[shid], uint64(len(workerChan)))
		if req.blockService.Flags.HasAny(msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED) {
			atomic.AddUint64(&stats.DecommissionedBlocks, 1)
			continue
		}
		err := c.CheckBlock(log, &req.blockService, req.block, req.size, req.crc)
		if badBlockError(err) {
			if !migrateFileOnError(log, c, stats, shid, terminateChan, scratchFile, migratingFiles, migratingFilesMu, req, err, bufPool, blockNotFoundAlert) {
				return
			}
		} else if err == msgs.BLOCK_IO_ERROR_DEVICE {
			// This is almost certainly a broken server. We want migration to take
			// care of this -- otherwise the scrubber will be stuck on tons of
			// these errors.
			log.Info("got IO error for file %v (block %v, block service %v), ignoring: %v", req.file, req.block, req.blockService.Id, err)
		} else if err != nil {
			log.Info("could not check block %v for file %v, will terminate: %v", req.block, req.file, err)
			select {
			case terminateChan <- err:
			default:
			}
			return
		} else {
			atomic.AddUint64(&stats.CheckedBlocks, 1)
			atomic.AddUint64(&stats.CheckedBytes, uint64(req.size))
		}
	}
}

func migrateFileOnError(
	log *log.Logger,
	c *client.Client,
	stats *ScrubState,
	shid msgs.ShardId,
	terminateChan chan any,
	scratchFile scratch.ScratchFile,
	migratingFiles map[msgs.InodeId]struct{},
	migratingFilesMu *sync.RWMutex,
	req *scrubRequest,
	err error,
	bufPool *bufpool.BufPool,
	blockNotFoundAlert *log.XmonNCAlert,
) bool {
	migratingFilesMu.Lock()
	_, ok := migratingFiles[req.file]
	migratingFiles[req.file] = struct{}{}
	migratingFilesMu.Unlock()
	if ok {
		// file already being migrated, nothing to do
		return true
	}
	defer func() {
		migratingFilesMu.Lock()
		delete(migratingFiles, req.file)
		migratingFilesMu.Unlock()
	}()

	atomic.AddUint64(&stats.CheckedBlocks, 1)
	if err == msgs.BAD_BLOCK_CRC {
		atomic.AddUint64(&stats.CheckedBytes, uint64(req.size))
	}
	for attempts := 1; ; attempts++ {
		if err := scrubFileInternal(log, c, bufPool, stats, nil, nil, scratchFile, req.file); err != nil {
			if err == msgs.BLOCK_NOT_FOUND {
				log.RaiseNC(blockNotFoundAlert, "could not migrate blocks in file %v after %v attempts because a block was not found in it. this is probably due to conflicts with other migrations or scrubbing. will retry in one second.", req.file, attempts)
				time.Sleep(time.Second)
			} else {
				log.Info("could not scrub file %v, will terminate: %v", req.file, err)
				select {
				case terminateChan <- err:
				default:
				}
				return false
			}
		} else {
			log.ClearNC(blockNotFoundAlert)
			break
		}
	}
	return true
}

func scrubScraper(
	log *log.Logger,
	c *client.Client,
	stats *ScrubState,
	shid msgs.ShardId,
	terminateChan chan any,
	workerChan chan *scrubRequest,
) {
	fileReq := &msgs.VisitFilesReq{BeginId: stats.Cursors[shid]}
	fileResp := &msgs.VisitFilesResp{}
	for {
		if err := c.ShardRequest(log, msgs.ShardId(shid), fileReq, fileResp); err != nil {
			log.Info("could not get files: %v", err)
			select {
			case terminateChan <- err:
			default:
			}
			return
		}
		log.Debug("will migrate %d files", len(fileResp.Ids))
		for _, file := range fileResp.Ids {
			spansReq := msgs.LocalFileSpansReq{FileId: file}
			spansResp := msgs.LocalFileSpansResp{}
			for {
				if err := c.ShardRequest(log, file.Shard(), &spansReq, &spansResp); err != nil {
					log.Info("could not get spans: %v", err)
					select {
					case terminateChan <- err:
					default:
					}
					return
				}
				for _, span := range spansResp.Spans {
					if span.Header.StorageClass == msgs.INLINE_STORAGE {
						continue
					}
					body := span.Body.(*msgs.FetchedBlocksSpan)
					for _, block := range body.Blocks {
						size := body.CellSize * uint32(body.Stripes)
						blockService := spansResp.BlockServices[block.BlockServiceIx]
						workerChan <- &scrubRequest{
							file:         file,
							blockService: blockService,
							block:        block.BlockId,
							size:         size,
							crc:          block.Crc,
						}
					}
				}
				spansReq.ByteOffset = spansResp.NextOffset
				if spansReq.ByteOffset == 0 {
					break
				}
			}
		}
		stats.Cursors[shid] = fileResp.NextId
		fileReq.BeginId = fileResp.NextId
		if fileReq.BeginId == 0 {
			// this will terminate all the workers
			log.Debug("file scraping done for shard %v, terminating workers", shid)
			atomic.AddUint32(&stats.Cycles[shid], 1)
			close(workerChan)
			return
		}
	}
}

func ScrubFile(
	log *log.Logger,
	c *client.Client,
	stats *ScrubState,
	file msgs.InodeId,
) error {
	bufPool := bufpool.NewBufPool()
	scratchFile := scratch.NewScratchFile(log, c, file.Shard(), fmt.Sprintf("scrub file %v", file))
	defer scratchFile.Close()

	return scrubFileInternal(log, c, bufPool, stats, nil, nil, scratchFile, file)
}

func ScrubFiles(
	log *log.Logger,
	c *client.Client,
	opts *ScrubOptions,
	rateLimit *timing.RateLimit,
	stats *ScrubState,
	shid msgs.ShardId,
) error {
	if opts.NumWorkersPerShard <= 0 {
		panic(fmt.Errorf("the number of senders should be positive, got %v", opts.NumWorkersPerShard))
	}
	log.Info("starting to scrub files for shard %v", shid)
	terminateChan := make(chan any, 1)
	sendChan := make(chan *scrubRequest, opts.WorkersQueueSize)

	go func() {
		defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
		scrubScraper(log, c, stats, shid, terminateChan, sendChan)
	}()

	var workersWg sync.WaitGroup
	workersWg.Add(opts.NumWorkersPerShard)
	var scrubbingMu sync.Mutex
	migratingFiles := map[msgs.InodeId]struct{}{}
	migratingFilesMu := sync.RWMutex{}

	for i := 0; i < opts.NumWorkersPerShard; i++ {
		go func() {
			defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
			scratchFile := scratch.NewScratchFile(log, c, shid, fmt.Sprintf("scrubbing shard %v worked %d", shid, i))
			defer scratchFile.Close()
			scrubWorker(log, c, opts, stats, rateLimit, shid, sendChan, terminateChan, scratchFile, &scrubbingMu, migratingFiles, &migratingFilesMu)
			workersWg.Done()
		}()
	}
	go func() {
		workersWg.Wait()
		log.Info("all workers terminated for shard %v, we're done", shid)
		select {
		case terminateChan <- nil:
		default:
		}
	}()

	err := <-terminateChan
	if err == nil {
		return nil
	} else {
		log.Info("could not scrub files: %v", err)
		return err.(error)
	}
}

func ScrubFilesInAllShards(
	log *log.Logger,
	c *client.Client,
	opts *ScrubOptions,
	rateLimit *timing.RateLimit,
	state *ScrubState,
) error {
	terminateChan := make(chan any, 1)

	var wg sync.WaitGroup
	wg.Add(256)
	for i := 0; i < 256; i++ {
		shid := msgs.ShardId(i)
		go func() {
			defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
			if err := ScrubFiles(log, c, opts, rateLimit, state, shid); err != nil {
				panic(err)
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		log.Info("scrubbing terminated in all shards")
		terminateChan <- nil
	}()

	err := <-terminateChan
	if err == nil {
		return nil
	} else {
		log.Info("could not scrub files: %v", err)
		return err.(error)
	}
}
