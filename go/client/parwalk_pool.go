// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

// ErrParwalkPoolClosed is returned when a walk is submitted after the pool
// was closed.
var ErrParwalkPoolClosed = errors.New("parwalk pool is closed")

type parwalkReq struct {
	run  *parwalkRun
	id   msgs.InodeId
	path string
}

// ParwalkPool owns shard-aware directory workers shared by concurrent walks.
type ParwalkPool struct {
	log       *log.Logger
	client    *Client
	chans     []chan parwalkReq
	workers   sync.WaitGroup
	active    sync.WaitGroup
	mutex     sync.Mutex
	closed    bool
	closeOnce sync.Once
}

type parwalkRun struct {
	ParwalkOptions
	pool    *ParwalkPool
	ctx     context.Context
	cancel  context.CancelCauseFunc
	onEdge  ParwalkVisitFunc
	pending sync.WaitGroup
}

// NewParwalkPool creates a shard-aware worker pool. Close must be called after
// the final walk.
func NewParwalkPool(
	logger *log.Logger,
	client *Client,
	workersPerShard int,
) *ParwalkPool {
	if workersPerShard < 1 {
		panic(fmt.Errorf("workersPerShard=%d < 1", workersPerShard))
	}
	pool := &ParwalkPool{
		log:    logger,
		client: client,
		chans:  make([]chan parwalkReq, 256),
	}
	for i := range pool.chans {
		pool.chans[i] = make(chan parwalkReq, 10_000)
		shid := msgs.ShardId(i)
		for range workersPerShard {
			pool.workers.Add(1)
			go pool.worker(shid, pool.chans[i])
		}
	}
	return pool
}

func (pool *ParwalkPool) worker(
	homeShid msgs.ShardId,
	requests <-chan parwalkReq,
) {
	defer pool.workers.Done()
	for req := range requests {
		if context.Cause(req.run.ctx) == nil {
			if err := req.run.process(homeShid, req.id, req.path); err != nil {
				req.run.fail(err)
			}
		}
		req.run.pending.Done()
	}
}

// Close waits for active walks and then stops the workers. Callers must cancel
// their walk contexts before closing a pool with active work.
func (pool *ParwalkPool) Close() {
	pool.closeOnce.Do(func() {
		pool.mutex.Lock()
		pool.closed = true
		pool.mutex.Unlock()

		pool.active.Wait()
		for _, requests := range pool.chans {
			close(requests)
		}
		pool.workers.Wait()
	})
}

// run executes one walk on the pool's shared workers. prime seeds the walk
// (typically one visitAndRecurse call per root); it runs before the walk is
// awaited, so seeds that fail immediately still report through the returned
// error.
func (pool *ParwalkPool) run(
	ctx context.Context,
	prime func(*parwalkRun) error,
	onEdge ParwalkVisitFunc,
	options ParwalkOptions,
) error {
	runCtx, cancel := context.WithCancelCause(ctx)
	run := &parwalkRun{
		ParwalkOptions: options,
		pool:           pool,
		ctx:            runCtx,
		cancel:         cancel,
		onEdge:         onEdge,
	}

	pool.mutex.Lock()
	if pool.closed {
		pool.mutex.Unlock()
		cancel(ErrParwalkPoolClosed)
		return ErrParwalkPoolClosed
	}
	pool.active.Add(1)
	pool.mutex.Unlock()

	defer func() {
		run.cancel(nil)
		pool.active.Done()
	}()

	if err := prime(run); err != nil {
		run.fail(err)
	}
	run.pending.Wait()
	return context.Cause(run.ctx)
}

func (run *parwalkRun) cause() error {
	return context.Cause(run.ctx)
}

func (run *parwalkRun) fail(err error) {
	if err != nil {
		run.cancel(err)
	}
}

func (run *parwalkRun) schedule(
	homeShid msgs.ShardId,
	id msgs.InodeId,
	pathStr string,
) error {
	if err := context.Cause(run.ctx); err != nil {
		return err
	}
	req := parwalkReq{run: run, id: id, path: pathStr}
	run.pending.Add(1)
	select {
	case <-run.ctx.Done():
		run.pending.Done()
		return context.Cause(run.ctx)
	case run.pool.chans[id.Shard()] <- req:
		return nil
	default:
		err := run.process(homeShid, id, pathStr)
		run.pending.Done()
		return err
	}
}
