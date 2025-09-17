// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

// Goes through all the _current_ files, and checks that the span block allocation
// is coherent with the span policy. Very similar to migration, hence it shares
// some datatypes.
package cleanup

import (
	"fmt"
	"path"
	"sync/atomic"
	"time"
	"xtx/ternfs/cleanup/scratch"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

type DefragStats struct {
	AnalyzedFiles                uint64
	AnalyzedLogicalSize          uint64
	AnalyzedBlocks               uint64
	DefraggedSpans               uint64
	DefraggedLogicalBytes        uint64
	DefraggedBlocksBefore        uint64
	DefraggedPhysicalBytesBefore uint64
	DefraggedBlocksAfter         uint64
	DefraggedPhysicalBytesAfter  uint64
}

func defragPrintStatsLastReport(log *log.Logger, c *client.Client, stats *DefragStats, timeStats *timeStats, progressReportAlert *log.XmonNCAlert, lastReport int64, now int64) {
	timeSinceStart := time.Duration(now - atomic.LoadInt64(&timeStats.startedAt))
	physicalDeltaMB := (float64(stats.DefraggedPhysicalBytesAfter) - float64(stats.DefraggedPhysicalBytesBefore)) / 1e6
	physicalDeltaMBs := 1000.0 * physicalDeltaMB / float64(timeSinceStart.Milliseconds())
	log.RaiseNC(
		progressReportAlert,
		"looked at %v files %v blocks, logical size %0.2fTB, defragged %v spans, logical size %0.2fTB; %v blocks %0.2fTB -> %v blocks (%+d) %0.2fTB (%+0.2fTB), %+0.2fMB/s",
		stats.AnalyzedFiles, stats.AnalyzedBlocks, float64(stats.AnalyzedLogicalSize)/1e12,
		stats.DefraggedSpans, float64(stats.DefraggedLogicalBytes)/1e12,
		stats.DefraggedBlocksBefore, float64(stats.DefraggedPhysicalBytesBefore)/1e12,
		stats.DefraggedBlocksAfter, int64(stats.DefraggedBlocksAfter)-int64(stats.DefraggedBlocksBefore),
		float64(stats.DefraggedPhysicalBytesAfter)/1e12, physicalDeltaMB/1e6, physicalDeltaMBs,
	)
	timeStats.lastReportAt = now
}

func defragFileInternal(
	log *log.Logger,
	c *client.Client,
	bufPool *bufpool.BufPool,
	dirInfoCache *client.DirInfoCache,
	stats *DefragStats,
	progressReportAlert *log.XmonNCAlert,
	timeStats *timeStats,
	parent msgs.InodeId,
	fileId msgs.InodeId,
	filePath string,
	minSpanSize uint32,
	storageClass msgs.StorageClass,
) error {
	defer func() {
		lastReportAt := atomic.LoadInt64(&timeStats.lastReportAt)
		now := time.Now().UnixNano()
		if (now - lastReportAt) > time.Minute.Nanoseconds() {
			if atomic.CompareAndSwapInt64(&timeStats.lastReportAt, lastReportAt, now) {
				defragPrintStatsLastReport(log, c, stats, timeStats, progressReportAlert, lastReportAt, now)
			}
		}
	}()
	atomic.AddUint64(&stats.AnalyzedFiles, 1)
	var spanPolicy msgs.SpanPolicy
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, parent, &spanPolicy); err != nil {
		return err
	}
	var blockPolicy msgs.BlockPolicy
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, parent, &blockPolicy); err != nil {
		return err
	}
	var stripePolicy msgs.StripePolicy
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, parent, &stripePolicy); err != nil {
		return err
	}
	fileSpansReq := msgs.LocalFileSpansReq{
		FileId:     fileId,
		ByteOffset: 0,
	}
	fileSpansResp := msgs.LocalFileSpansResp{}
	sf := scratch.NewScratchFile(log, c, fileId.Shard(), fmt.Sprintf("defrag %v", fileId))
	for {
		if err := c.ShardRequest(log, fileId.Shard(), &fileSpansReq, &fileSpansResp); err != nil {
			return err
		}
		for spanIx := range fileSpansResp.Spans {
			span := &fileSpansResp.Spans[spanIx]
			atomic.AddUint64(&stats.AnalyzedLogicalSize, uint64(span.Header.Size))
			if span.Header.StorageClass == msgs.INLINE_STORAGE {
				continue
			}
			if span.Header.Size < minSpanSize {
				continue
			}
			if storageClass != msgs.EMPTY_STORAGE && storageClass != span.Header.StorageClass {
				continue
			}
			body := span.Body.(*msgs.FetchedBlocksSpan)
			atomic.AddUint64(&stats.AnalyzedBlocks, uint64(body.Parity.Blocks()))
			actualSpanParams := &client.SpanParameters{
				Parity:       body.Parity,
				Stripes:      body.Stripes,
				StorageClass: span.Header.StorageClass,
				CellSize:     body.CellSize,
			}
			desiredSpanParams := client.ComputeSpanParameters(&spanPolicy, &blockPolicy, &stripePolicy, span.Header.Size)
			if *actualSpanParams == *desiredSpanParams {
				continue
			}
			log.Debug("replacing path=%q file=%v offset=%v before=%v after=%v", filePath, fileId, span.Header.ByteOffset, actualSpanParams, desiredSpanParams)
			atomic.AddUint64(&stats.DefraggedSpans, 1)
			atomic.AddUint64(&stats.DefraggedLogicalBytes, uint64(span.Header.Size))
			atomic.AddUint64(&stats.DefraggedBlocksBefore, uint64(body.Parity.Blocks()))
			atomic.AddUint64(&stats.DefraggedBlocksAfter, uint64(desiredSpanParams.Parity.Blocks()))
			atomic.AddUint64(&stats.DefraggedPhysicalBytesBefore, uint64(body.Parity.Blocks())*uint64(body.Stripes)*uint64(body.CellSize))
			atomic.AddUint64(&stats.DefraggedPhysicalBytesAfter, uint64(desiredSpanParams.Parity.Blocks())*uint64(desiredSpanParams.Stripes)*uint64(desiredSpanParams.CellSize))
			// read span
			spanBuf, err := c.FetchSpan(log, bufPool, fileId, &client.SpanWithBlockServices{BlockServices: fileSpansResp.BlockServices, Span: span})
			if err != nil {
				return err
			}
			defer bufPool.Put(spanBuf)
			crc := crc32c.Sum(0, spanBuf.Bytes())
			if msgs.Crc(crc) != span.Header.Crc {
				panic(fmt.Errorf("expected crc %v, got %v", span.Header.Crc, msgs.Crc(crc)))
			}
			lockedSF, err := sf.Lock()
			if err != nil {
				return err
			}

			// create new span in scratch file
			scratchOffset := lockedSF.Size()
			createdBlocks, err := c.CreateSpan(log, []msgs.BlacklistEntry{}, &spanPolicy, &blockPolicy, &stripePolicy, lockedSF.FileId(), fileId, lockedSF.Cookie(), lockedSF.Size(), span.Header.Size, spanBuf.BytesPtr())
			if err != nil {
				lockedSF.ClearOnUnlock(fmt.Sprintf("failed to create span %v", err))
				lockedSF.Unlock()
				return err
			}
			lockedSF.AddSize(uint64(span.Header.Size))
			// swap them
			swapReq := msgs.SwapSpansReq{
				FileId1:     fileId,
				ByteOffset1: span.Header.ByteOffset,
				Blocks1:     []msgs.BlockId{},
				FileId2:     lockedSF.FileId(),
				ByteOffset2: scratchOffset,
				Blocks2:     createdBlocks,
			}
			for _, block := range body.Blocks {
				swapReq.Blocks1 = append(swapReq.Blocks1, block.BlockId)
			}
			if err := c.ShardRequest(log, fileId.Shard(), &swapReq, &msgs.SwapSpansResp{}); err != nil {
				lockedSF.ClearOnUnlock(fmt.Sprintf("failed to swap spans %v", err))
				lockedSF.Unlock()
				return err
			}
			lockedSF.Unlock()
		}
		if fileSpansResp.NextOffset == 0 {
			break
		}
		fileSpansReq.ByteOffset = fileSpansResp.NextOffset
	}
	log.Debug("finished migrating file %v, %v files migrated so far", fileId, stats.AnalyzedFiles)
	return nil
}

func DefragFile(
	log *log.Logger,
	client *client.Client,
	bufPool *bufpool.BufPool,
	dirInfoCache *client.DirInfoCache,
	stats *DefragStats,
	progressReportAlert *log.XmonNCAlert,
	parent msgs.InodeId,
	fileId msgs.InodeId,
	filePath string,
) error {
	timeStats := newTimeStats()
	return defragFileInternal(log, client, bufPool, dirInfoCache, stats, progressReportAlert, timeStats, parent, fileId, filePath, 0, 0)
}

type DefragOptions struct {
	WorkersPerShard int
	StartFrom       msgs.TernTime
	MinSpanSize     uint32
	StorageClass    msgs.StorageClass // EMPTY = no filter
}

func DefragFiles(
	log *log.Logger,
	c *client.Client,
	bufPool *bufpool.BufPool,
	dirInfoCache *client.DirInfoCache,
	stats *DefragStats,
	progressReportAlert *log.XmonNCAlert,
	options *DefragOptions,
	root string,
) error {
	timeStats := newTimeStats()
	return client.Parwalk(
		log, c, &client.ParwalkOptions{WorkersPerShard: options.WorkersPerShard}, root,
		func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
			if id.Type() == msgs.DIRECTORY {
				return nil
			}
			if creationTime < options.StartFrom {
				return nil
			}
			err := defragFileInternal(
				log, c, bufPool, dirInfoCache, stats, progressReportAlert, timeStats, parent, id, path.Join(parentPath, name), options.MinSpanSize, options.StorageClass,
			)
			// keep defragging
			if err != nil {
				log.RaiseAlert("could not read file %q (%v): %v", path.Join(parentPath, name), id, err)
			}
			return nil
		},
	)
}

type DefragSpansStats struct {
	AnalyzedFiles       uint64
	AnalyzedLogicalSize uint64
	ReplacedLogicalSize uint64
}

// Replaces a file with another, identical one.
func defragFileReplace(
	log *log.Logger,
	c *client.Client,
	bufPool *bufpool.BufPool,
	dirInfoCache *client.DirInfoCache,
	stats *DefragSpansStats,
	progressReportAlert *log.XmonNCAlert,
	timeStats *timeStats,
	parent msgs.InodeId,
	fileId msgs.InodeId,
	filePath string,
) error {
	defer func() {
		lastReportAt := atomic.LoadInt64(&timeStats.lastReportAt)
		now := time.Now().UnixNano()
		if (now - lastReportAt) > time.Minute.Nanoseconds() {
			if atomic.CompareAndSwapInt64(&timeStats.lastReportAt, lastReportAt, now) {
				timeSinceStart := time.Duration(now - atomic.LoadInt64(&timeStats.startedAt))
				analyzedMB := float64(stats.AnalyzedLogicalSize) / 1e6
				analyzedMBs := 1000.0 * analyzedMB / float64(timeSinceStart.Milliseconds())
				log.RaiseNC(
					progressReportAlert,
					"looked at %v files, logical size %0.2fTB (%0.2fMB/s), replaced %0.2fTB",
					stats.AnalyzedFiles, float64(stats.AnalyzedLogicalSize)/1e12, analyzedMBs, float64(stats.ReplacedLogicalSize)/1e12,
				)
			}
		}
	}()
	var spanPolicy msgs.SpanPolicy
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, parent, &spanPolicy); err != nil {
		return err
	}
	atomic.AddUint64(&stats.AnalyzedFiles, 1)
	shouldReplace := false
	fileSize := uint64(0)
	{
		largestSpanSize := spanPolicy.Entries[len(spanPolicy.Entries)-1].MaxSize
		fileSpansReq := msgs.LocalFileSpansReq{
			FileId:     fileId,
			ByteOffset: 0,
		}
		fileSpansResp := msgs.LocalFileSpansResp{}
		for {
			if err := c.ShardRequest(log, fileId.Shard(), &fileSpansReq, &fileSpansResp); err != nil {
				return err
			}
			for spanIx := range fileSpansResp.Spans {
				span := &fileSpansResp.Spans[spanIx]
				fileSize += uint64(span.Header.Size)
				shouldReplace = shouldReplace || (span.Header.Size > largestSpanSize) // span is too big
				lastSpan := (spanIx == len(fileSpansResp.Spans)-1) && (fileSpansResp.NextOffset == 0)
				shouldReplace = shouldReplace || (!lastSpan && span.Header.Size < largestSpanSize) // span is too small
			}
			if fileSpansResp.NextOffset == 0 {
				break
			}
			fileSpansReq.ByteOffset = fileSpansResp.NextOffset
		}
	}
	atomic.AddUint64(&stats.AnalyzedLogicalSize, fileSize)
	if !shouldReplace {
		return nil
	}
	// TODO it would be a lot nicer for this to be atomic, with some custom shard
	// operation.
	fileContents, err := c.ReadFile(log, bufPool, fileId)
	if err != nil {
		return err
	}
	defer fileContents.Close()
	if _, err := c.CreateFile(log, bufPool, dirInfoCache, filePath, fileContents); err != nil {
		return err
	}
	atomic.AddUint64(&stats.ReplacedLogicalSize, fileSize)
	return nil
}

func DefragSpans(
	log *log.Logger,
	c *client.Client,
	bufPool *bufpool.BufPool,
	dirInfoCache *client.DirInfoCache,
	stats *DefragSpansStats,
	progressReportAlert *log.XmonNCAlert,
	root string,
) error {
	timeStats := newTimeStats()
	return client.Parwalk(
		log, c, &client.ParwalkOptions{WorkersPerShard: 5}, root,
		func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
			if id.Type() == msgs.DIRECTORY {
				return nil
			}
			err := defragFileReplace(
				log, c, bufPool, dirInfoCache, stats, progressReportAlert, timeStats, parent, id, path.Join(parentPath, name),
			)
			// keep defragging
			if err != nil {
				log.RaiseAlert("could not defrag file %q (%v): %v", path.Join(parentPath, name), id, err)
			}
			return nil
		},
	)
}
