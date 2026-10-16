// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"sync"
	"sync/atomic"
	"time"
)

type parwalkManyOpts struct {
	numRoots        int
	dirsPerRoot     int
	filesPerDir     int
	workersPerShard int
}

// parwalkManyTest creates several disjoint root subtrees, walks them all
// through a single ParwalkPool.WalkMany call, and checks:
//
//  1. Every owned file inserted is reported exactly once.
//  2. Every reported file lives under one of the seeded roots.
//  3. While the walk is in flight, more than one of the 256 shard channels
//     is being drained concurrently — i.e. roots share the worker pool
//     instead of being processed strictly one root at a time.
func parwalkManyTest(
	logger *log.Logger,
	registryAddress string,
	counters *client.ClientCounters,
	opts *parwalkManyOpts,
) {
	c := newTestClient(logger, registryAddress, counters)
	defer c.Close()

	roots := make([]string, opts.numRoots)
	expected := make(map[msgs.InodeId]string)
	expectedNames := make(map[string]struct{})

	for ri := 0; ri < opts.numRoots; ri++ {
		rootName := fmt.Sprintf("parwalkmany-%d", ri)
		mkRoot := &msgs.MakeDirectoryResp{}
		if err := c.CDCRequest(logger, &msgs.MakeDirectoryReq{OwnerId: msgs.ROOT_DIR_INODE_ID, Name: rootName}, mkRoot); err != nil {
			panic(err)
		}
		roots[ri] = "/" + rootName

		for di := 0; di < opts.dirsPerRoot; di++ {
			dirName := fmt.Sprintf("d-%d", di)
			mkDir := &msgs.MakeDirectoryResp{}
			if err := c.CDCRequest(logger, &msgs.MakeDirectoryReq{OwnerId: mkRoot.Id, Name: dirName}, mkDir); err != nil {
				panic(err)
			}
			for fi := 0; fi < opts.filesPerDir; fi++ {
				fileName := fmt.Sprintf("f-%d", fi)
				cf := msgs.ConstructFileResp{}
				if err := c.ShardRequest(logger, mkDir.Id.Shard(), &msgs.ConstructFileReq{Type: msgs.FILE}, &cf); err != nil {
					panic(err)
				}
				if err := c.ShardRequest(logger, mkDir.Id.Shard(), &msgs.LinkFileReq{FileId: cf.Id, Cookie: cf.Cookie, OwnerId: mkDir.Id, Name: fileName}, &msgs.LinkFileResp{}); err != nil {
					panic(err)
				}
				fullName := fmt.Sprintf("/%s/%s/%s", rootName, dirName, fileName)
				expected[cf.Id] = fullName
				expectedNames[fullName] = struct{}{}
			}
		}
	}

	// Concurrency observability: the parwalk callback runs on a shard
	// goroutine. Track how many distinct shards are *simultaneously*
	// inside the callback. If WalkMany shares a pool, this tops 1
	// the moment a second root is seeded and there's any work left for
	// the first root.
	var (
		seenMu       sync.Mutex
		seen         = make(map[msgs.InodeId]int)
		seenNames    = make(map[string]int)
		activeShards [256]int32
		maxConc      int32
	)

	cb := func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
		shid := id.Shard()
		atomic.AddInt32(&activeShards[shid], 1)
		// Recompute max distinct active shards. Coarse — we just count
		// non-zero slots. This isn't a tight bound but it surfaces the
		// "did roots actually overlap on the worker pool" signal.
		//
		// The callback body is microseconds long, so two shard workers
		// landing in it at the same instant is pure scheduler luck —
		// which made the >=2 assertion below flaky. Until we've actually
		// observed an overlap, linger briefly while still marked active so
		// a worker draining another shard has a chance to join the window.
		// If the pool were truly serial nothing else is ever active, the
		// poll exits immediately, and the assertion still fails as intended.
		var conc int32
		deadline := time.Now().Add(time.Second)
		for {
			conc = 0
			for i := range activeShards {
				if atomic.LoadInt32(&activeShards[i]) > 0 {
					conc++
				}
			}
			if conc >= 2 || atomic.LoadInt32(&maxConc) >= 2 || time.Now().After(deadline) {
				break
			}
			time.Sleep(time.Millisecond)
		}
		for {
			cur := atomic.LoadInt32(&maxConc)
			if conc <= cur || atomic.CompareAndSwapInt32(&maxConc, cur, conc) {
				break
			}
		}

		if id.Type() == msgs.FILE && owned && current {
			seenMu.Lock()
			seen[id]++
			seenNames[parentPath+"/"+name]++
			seenMu.Unlock()
		}

		atomic.AddInt32(&activeShards[shid], -1)
		return nil
	}

	pool := client.NewParwalkPool(logger, c, opts.workersPerShard)
	defer pool.Close()
	if err := pool.WalkMany(
		context.Background(),
		&client.ParwalkOptions{},
		roots,
		cb,
	); err != nil {
		panic(err)
	}

	// Every expected file seen exactly once.
	for id, name := range expected {
		got, ok := seen[id]
		if !ok {
			panic(fmt.Errorf("file %v (%s) never visited by WalkMany", id, name))
		}
		if got != 1 {
			panic(fmt.Errorf("file %v (%s) visited %d times, want 1", id, name, got))
		}
		if seenNames[name] != 1 {
			panic(fmt.Errorf("path %s visited %d times, want 1", name, seenNames[name]))
		}
	}
	// Every visited file is one we created (no leakage from elsewhere
	// in the test fs).
	if len(seen) != len(expected) {
		panic(fmt.Errorf("WalkMany visited %d files, expected %d", len(seen), len(expected)))
	}
	for name := range seenNames {
		if _, want := expectedNames[name]; !want {
			panic(fmt.Errorf("WalkMany visited unexpected path %s", name))
		}
	}

	// Sanity: with multiple roots and >1 worker per shard, we expect to
	// have observed at least 2 shard goroutines running concurrently in
	// the callback at some point. If this fails, WalkMany is
	// effectively serial — which would defeat the whole point.
	if opts.numRoots > 1 && opts.dirsPerRoot*opts.filesPerDir > 1 && atomic.LoadInt32(&maxConc) < 2 {
		panic(fmt.Errorf("WalkMany never had >=2 shards active concurrently (max=%d); pool not shared across roots", maxConc))
	}
	logger.Info("parwalkMany visited %d files across %d roots; max concurrent shards observed = %d",
		len(seen), len(roots), atomic.LoadInt32(&maxConc))
}
