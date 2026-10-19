// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/cleanup"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/wyhash"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

// Decommissions a block service, and checks that once the migrator is done
// nothing references it anymore: not the file we migrated, not a deleted file
// GC hasn't got to yet, and not the migrator's own scratch files.
// Decommissioning can't be undone, so this has to run after the tests that
// need all the block services.
func migrateDecommissionedTest(log *log.Logger, registryAddress string, counters *client.ClientCounters) {
	c := newTestClient(log, registryAddress, counters)
	defer c.Close()
	bufPool := bufpool.NewBufPool()

	const size = 20 << 20
	const dataSeed = 42
	fileId, _ := createFile(log, c, client.NewDirInfoCache(), msgs.ROOT_DIR_INODE_ID, 1<<20, "migrate-decommissioned", size, dataSeed, bufPool)
	// take the block service with the most blocks of the file
	blocksIn := make(map[msgs.BlockServiceId]uint64)
	bytesIn := make(map[msgs.BlockServiceId]uint64)
	var location msgs.Location
	spansReq := msgs.FileSpansReq{FileId: fileId}
	for {
		spansResp := msgs.FileSpansResp{}
		shardReq(log, c, fileId.Shard(), &spansReq, &spansResp)
		for _, span := range spansResp.Spans {
			for _, loc := range span.Body.(*msgs.FetchedLocations).Locations {
				location = loc.LocationId
				for _, block := range loc.Blocks {
					bs := spansResp.BlockServices[block.BlockServiceIx].Id
					blocksIn[bs]++
					bytesIn[bs] += uint64(loc.CellSize) * uint64(loc.Stripes)
				}
			}
		}
		if spansResp.NextOffset == 0 {
			break
		}
		spansReq.ByteOffset = spansResp.NextOffset
	}
	var blockServiceId msgs.BlockServiceId
	for bs, n := range blocksIn {
		if n > blocksIn[blockServiceId] {
			blockServiceId = bs
		}
	}
	log.Info("block service %v has %v blocks of file %v", blockServiceId, blocksIn[blockServiceId], fileId)
	replaceHealthyNeighbor(log, c, bufPool, fileId, blockServiceId)
	deletedFile, deletedBytes := createExpiredTransientFileOn(log, c, blockServiceId)

	log.Info("decommissioning block service %v", blockServiceId)
	if _, err := client.RegistryRequest(log, nil, registryAddress, &msgs.SetBlockServiceFlagsReq{
		Id:        blockServiceId,
		Flags:     msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED,
		FlagsMask: uint8(msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED),
	}); err != nil {
		panic(err)
	}
	waitForDecommissioned(log, c, fileId, blockServiceId)

	migrator := cleanup.Migrator(log, c, &cleanup.MigratorOptions{NumMigrators: 1, NumParallelFiles: 1, LocationId: location, ReplaceDecommissioned: true})
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		migrator.Run()
	}()
	migrator.UpdateScheduled([]msgs.BlockServiceId{blockServiceId})
	migrateStats := <-migrator.MigrationFinishedStats()
	migrator.Stop()
	wg.Wait()
	log.Info("migration stats: %+v", migrateStats)

	// the deleted file has one block there
	if migrateStats.MigratedBlocks != blocksIn[blockServiceId]+1 {
		panic(fmt.Errorf("migrator replaced %v blocks, expected %v", migrateStats.MigratedBlocks, blocksIn[blockServiceId]+1))
	}
	if expected := bytesIn[blockServiceId] + deletedBytes; migrateStats.MigratedBytes != expected {
		panic(fmt.Errorf("migrator replaced %v bytes, expected %v", migrateStats.MigratedBytes, expected))
	}
	// deleting is up to GC
	if err := c.ShardRequest(log, deletedFile.Shard(), &msgs.StatTransientFileReq{Id: deletedFile}, &msgs.StatTransientFileResp{}); err != nil {
		panic(fmt.Errorf("expected deleted file %v to still be there, got %v", deletedFile, err))
	}
	for shid := 0; shid < 256; shid++ {
		filesResp := msgs.BlockServiceFilesResp{}
		shardReq(log, c, msgs.ShardId(shid), &msgs.BlockServiceFilesReq{BlockServiceId: blockServiceId}, &filesResp)
		if len(filesResp.FileIds) > 0 {
			panic(fmt.Errorf("block service %v still has file %v in shard %v after migration", blockServiceId, filesResp.FileIds[0], shid))
		}
	}
	actualData := readFile(log, bufPool, c, fileId, size)
	defer bufPool.Put(actualData)
	expectedData := bufPool.Get(size)
	defer bufPool.Put(expectedData)
	wyhash.New(dataSeed).Read(expectedData.Bytes())
	checkFileData(fileId, 0, size, actualData.Bytes(), expectedData.Bytes())
}

// Exercise ReplaceBlock on a healthy sibling of the drive we'll decommission.
// That drive is excluded from replacement placement because it's already in
// the span, so its expected block and byte counts remain unchanged.
func replaceHealthyNeighbor(log *log.Logger, c *client.Client, bufPool *bufpool.BufPool, fileId msgs.InodeId, futureDecommissioned msgs.BlockServiceId) {
	req := msgs.FileSpansReq{FileId: fileId}
	for {
		resp := msgs.FileSpansResp{}
		shardReq(log, c, fileId.Shard(), &req, &resp)
		for _, span := range resp.Spans {
			for _, location := range span.Body.(*msgs.FetchedLocations).Locations {
				hasVictim := false
				for _, block := range location.Blocks {
					hasVictim = hasVictim || resp.BlockServices[block.BlockServiceIx].Id == futureDecommissioned
				}
				if !hasVictim {
					continue
				}
				for _, block := range location.Blocks {
					bs := &resp.BlockServices[block.BlockServiceIx]
					if bs.Id == futureDecommissioned {
						continue
					}
					init := msgs.ReplaceBlockInitiateResp{}
					shardReq(log, c, fileId.Shard(), &msgs.ReplaceBlockInitiateReq{
						FileId: fileId, ByteOffset: span.Header.ByteOffset, OldBlockId: block.BlockId,
					}, &init)
					blockSize := location.CellSize * uint32(location.Stripes)
					var data client.BlockReader
					data.New(bufPool, 0, blockSize, nil)
					defer func() { bufPool.Put(data.AcquireBuf()) }()
					if err := c.FetchBlock(log, nil, bs, block.BlockId, 0, blockSize, &data); err != nil {
						panic(err)
					}
					proof, err := c.WriteBlock(log, nil, &init.NewBlock, bytes.NewReader(data.Buf().Bytes()), blockSize, block.Crc)
					if err != nil {
						panic(err)
					}
					certify := msgs.ReplaceBlockCertifyResp{}
					shardReq(log, c, fileId.Shard(), &msgs.ReplaceBlockCertifyReq{
						FileId: fileId, ByteOffset: span.Header.ByteOffset, OldBlockId: block.BlockId,
						HolderId: init.HolderId, HolderCookie: init.HolderCookie,
						NewBlock: msgs.BlockProof{BlockId: init.NewBlock.BlockId, Proof: proof},
					}, &certify)
					if !certify.Replaced {
						panic("healthy block was not replaced")
					}
					holder := msgs.FileSpansResp{}
					shardReq(log, c, fileId.Shard(), &msgs.FileSpansReq{FileId: init.HolderId}, &holder)
					if len(holder.Spans) != 1 {
						panic("expected one span holding the old healthy block")
					}
					held := holder.Spans[0].Body.(*msgs.FetchedLocations).Locations
					if len(held) != 1 || len(held[0].Blocks) != 1 || held[0].Blocks[0].BlockId != block.BlockId ||
						holder.BlockServices[held[0].Blocks[0].BlockServiceIx].Id != bs.Id {
						panic("holder did not retain the old healthy block for GC")
					}
					log.Info("replaced healthy block %v in file %v; old block is in holder %v", block.BlockId, fileId, init.HolderId)
					return
				}
			}
		}
		if resp.NextOffset == 0 {
			panic("could not find a healthy sibling block to replace")
		}
		req.ByteOffset = resp.NextOffset
	}
}

// Creates a transient file with a block in the given block service and scraps it,
// so that its deadline has passed, like a deleted file waiting for GC.
func createExpiredTransientFileOn(log *log.Logger, c *client.Client, blockServiceId msgs.BlockServiceId) (msgs.InodeId, uint64) {
	dirInfoCache := client.NewDirInfoCache()
	var spanPolicy msgs.SpanPolicy
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, msgs.ROOT_DIR_INODE_ID, &spanPolicy); err != nil {
		panic(err)
	}
	var blockPolicies msgs.BlockPolicy
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, msgs.ROOT_DIR_INODE_ID, &blockPolicies); err != nil {
		panic(err)
	}
	var stripePolicy msgs.StripePolicy
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, msgs.ROOT_DIR_INODE_ID, &stripePolicy); err != nil {
		panic(err)
	}
	constructResp := msgs.ConstructFileResp{}
	shardReq(log, c, 0, &msgs.ConstructFileReq{Type: msgs.FILE, Note: "deleted"}, &constructResp)
	const spanSize = 1 << 20
	// block placement is up to the shard, so add spans until one lands where we want
	for offset := uint64(0); offset < 100*spanSize; offset += spanSize {
		data := make([]byte, spanSize) // CreateSpan may grow it
		if _, err := c.CreateSpan(log, []msgs.BlacklistEntry{}, &spanPolicy, &blockPolicies, &stripePolicy, constructResp.Id, msgs.NULL_INODE_ID, constructResp.Cookie, offset, spanSize, &data); err != nil {
			panic(err)
		}
		spansResp := msgs.FileSpansResp{}
		shardReq(log, c, 0, &msgs.FileSpansReq{FileId: constructResp.Id, ByteOffset: offset}, &spansResp)
		for _, span := range spansResp.Spans {
			for _, location := range span.Body.(*msgs.FetchedLocations).Locations {
				for _, block := range location.Blocks {
					if spansResp.BlockServices[block.BlockServiceIx].Id == blockServiceId {
						shardReq(log, c, 0, &msgs.ScrapTransientFileReq{Id: constructResp.Id, Cookie: constructResp.Cookie}, &msgs.ScrapTransientFileResp{})
						log.Info("created expired transient file %v with a block in %v", constructResp.Id, blockServiceId)
						return constructResp.Id, uint64(location.CellSize) * uint64(location.Stripes)
					}
				}
			}
		}
	}
	panic(fmt.Errorf("could not get a block in %v into transient file %v", blockServiceId, constructResp.Id))
}

// Waits until the file's shard reports the block service as decommissioned.
func waitForDecommissioned(log *log.Logger, c *client.Client, file msgs.InodeId, blockServiceId msgs.BlockServiceId) {
	for start := time.Now(); time.Since(start) < time.Minute; time.Sleep(100 * time.Millisecond) {
		spansReq := msgs.FileSpansReq{FileId: file}
		for {
			spansResp := msgs.FileSpansResp{}
			shardReq(log, c, file.Shard(), &spansReq, &spansResp)
			for _, bs := range spansResp.BlockServices {
				if bs.Id == blockServiceId && bs.Flags.HasAny(msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED) {
					return
				}
			}
			if spansResp.NextOffset == 0 {
				break
			}
			spansReq.ByteOffset = spansResp.NextOffset
		}
	}
	panic(fmt.Errorf("shard did not see block service %v as decommissioned", blockServiceId))
}
