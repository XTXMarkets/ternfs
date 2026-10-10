// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

// RecursiveGrepOptions describes one current-tree regular-expression search.
type RecursiveGrepOptions struct {
	Roots            []string
	Pattern          *regexp.Regexp
	MaxFileSize      uint64
	MaxLineSize      int
	ProgressInterval time.Duration
	// Filter is called for every walked edge. Returning false excludes the
	// entry; excluding a directory also prunes its subtree.
	Filter func(string, msgs.InodeId) (bool, error)
}

// RecursiveGrepFileError reports a file which could not be completely searched.
type RecursiveGrepFileError struct {
	Path  string
	Inode msgs.InodeId
	Err   error
}

// RecursiveGrepStats is a point-in-time search progress snapshot.
type RecursiveGrepStats struct {
	FilesFound         uint64
	FilesScanned       uint64
	BytesScanned       uint64
	MatchesFound       uint64
	FilesSkippedBySize uint64
	FileErrors         uint64
}

// RecursiveGrepCallbacks receives serialized output. Returning an error cancels the
// search.
type RecursiveGrepCallbacks struct {
	Match     func(GrepMatch) error
	FileError func(RecursiveGrepFileError) error
	Progress  func(RecursiveGrepStats) error
}

// RecursiveGrep pairs a shared ParwalkPool with a shared limit on concurrent file
// reads. Concurrent Grep calls share the same file-read budget.
type RecursiveGrep struct {
	parwalk *ParwalkPool
	log     *log.Logger
	client  *Client
	bufPool *bufpool.BufPool
	sem     chan struct{}
}

type recursiveGrepCounters struct {
	filesFound         atomic.Uint64
	filesScanned       atomic.Uint64
	bytesScanned       atomic.Uint64
	matchesFound       atomic.Uint64
	filesSkippedBySize atomic.Uint64
	fileErrors         atomic.Uint64
}

func (counters *recursiveGrepCounters) snapshot() RecursiveGrepStats {
	return RecursiveGrepStats{
		FilesFound:         counters.filesFound.Load(),
		FilesScanned:       counters.filesScanned.Load(),
		BytesScanned:       counters.bytesScanned.Load(),
		MatchesFound:       counters.matchesFound.Load(),
		FilesSkippedBySize: counters.filesSkippedBySize.Load(),
		FileErrors:         counters.fileErrors.Load(),
	}
}

type recursiveGrepRun struct {
	ctx       context.Context
	cancel    context.CancelCauseFunc
	options   *RecursiveGrepOptions
	callbacks RecursiveGrepCallbacks
	counters  recursiveGrepCounters
	pending   sync.WaitGroup
	emitMutex sync.Mutex
}

func (run *recursiveGrepRun) emit(callback func() error) error {
	run.emitMutex.Lock()
	defer run.emitMutex.Unlock()
	if err := context.Cause(run.ctx); err != nil {
		return err
	}
	if err := callback(); err != nil {
		run.cancel(err)
		return err
	}
	return nil
}

// NewRecursiveGrep creates a recursive grep using parwalk for traversal.
// grepWorkers bounds concurrent file reads across all Grep calls.
func NewRecursiveGrep(parwalk *ParwalkPool, grepWorkers int) *RecursiveGrep {
	if parwalk == nil {
		panic("nil ParwalkPool")
	}
	if grepWorkers < 1 {
		panic(fmt.Errorf("grepWorkers=%d < 1", grepWorkers))
	}
	return &RecursiveGrep{
		parwalk: parwalk,
		log:     parwalk.log,
		client:  parwalk.client,
		bufPool: bufpool.NewBufPool(),
		sem:     make(chan struct{}, grepWorkers),
	}
}

// Grep walks the roots and searches regular files. Individual file failures
// are reported through FileError and do not stop the remaining search.
//
// Acquiring the file-read budget blocks inside the walk callback. The callback
// holds a ParwalkPool shard worker while blocked, so another walk sharing that
// pool may stall if it needs the same shard. Parwalk's inline scheduling
// fallback prevents deadlock, but callers sharing a pool should account for
// this loss of metadata concurrency.
func (grep *RecursiveGrep) Grep(
	ctx context.Context,
	options *RecursiveGrepOptions,
	callbacks RecursiveGrepCallbacks,
) (RecursiveGrepStats, error) {
	if options == nil {
		return RecursiveGrepStats{}, errors.New("recursive grep options are required")
	}
	if len(options.Roots) == 0 {
		return RecursiveGrepStats{}, errors.New("recursive grep requires at least one root")
	}
	if options.Pattern == nil {
		return RecursiveGrepStats{}, errors.New("recursive grep pattern is required")
	}

	grepCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	run := &recursiveGrepRun{
		ctx:       grepCtx,
		cancel:    cancel,
		options:   options,
		callbacks: callbacks,
	}

	done := make(chan struct{})
	var progressDone <-chan struct{}
	if callbacks.Progress != nil && options.ProgressInterval > 0 {
		finished := make(chan struct{})
		progressDone = finished
		go func() {
			defer close(finished)
			ticker := time.NewTicker(options.ProgressInterval)
			defer ticker.Stop()
			for {
				select {
				case <-grepCtx.Done():
					return
				case <-done:
					return
				case <-ticker.C:
					_ = run.emit(func() error {
						return callbacks.Progress(run.counters.snapshot())
					})
				}
			}
		}()
	}

	walkErr := grep.parwalk.WalkMany(
		grepCtx,
		&ParwalkOptions{},
		options.Roots,
		func(
			_ msgs.InodeId,
			parentPath string,
			name string,
			_ msgs.TernTime,
			id msgs.InodeId,
			_ bool,
			_ bool,
		) error {
			fullPath := path.Join(parentPath, name)
			if options.Filter != nil {
				include, err := options.Filter(fullPath, id)
				if err != nil {
					return err
				}
				if !include {
					return ErrSkipSubtree
				}
			}
			if id.Type() != msgs.FILE {
				return nil
			}
			run.pending.Add(1)
			select {
			case <-grepCtx.Done():
				run.pending.Done()
				return context.Cause(grepCtx)
			case grep.sem <- struct{}{}:
				run.counters.filesFound.Add(1)
				go func() {
					defer func() { <-grep.sem }()
					defer run.pending.Done()
					grep.grepFile(run, fullPath, id)
				}()
				return nil
			}
		},
	)
	run.pending.Wait()
	close(done)
	if progressDone != nil {
		<-progressDone
	}

	stats := run.counters.snapshot()
	if err := context.Cause(grepCtx); err != nil {
		return stats, err
	}
	return stats, walkErr
}

func (grep *RecursiveGrep) grepFile(
	run *recursiveGrepRun,
	filePath string,
	id msgs.InodeId,
) {
	bytesScanned, err := GrepFile(
		run.ctx,
		grep.log,
		grep.client,
		grep.bufPool,
		filePath,
		id,
		run.options.Pattern,
		run.options.MaxFileSize,
		run.options.MaxLineSize,
		func(match GrepMatch) error {
			return run.emit(func() error {
				run.counters.matchesFound.Add(1)
				if run.callbacks.Match == nil {
					return nil
				}
				return run.callbacks.Match(match)
			})
		},
	)
	run.counters.bytesScanned.Add(bytesScanned)
	switch {
	case err == nil:
		run.counters.filesScanned.Add(1)
	case errors.Is(err, ErrGrepFileTooLarge):
		run.counters.filesSkippedBySize.Add(1)
	case context.Cause(run.ctx) == nil:
		grep.publishFileError(run, filePath, id, err)
	}
}

func (grep *RecursiveGrep) publishFileError(
	run *recursiveGrepRun,
	filePath string,
	id msgs.InodeId,
	err error,
) {
	run.counters.fileErrors.Add(1)
	_ = run.emit(func() error {
		if run.callbacks.FileError == nil {
			return nil
		}
		return run.callbacks.FileError(RecursiveGrepFileError{
			Path:  filePath,
			Inode: id,
			Err:   err,
		})
	})
}
