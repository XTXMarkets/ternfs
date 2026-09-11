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

// TreeGrepOptions describes one current-tree regular-expression search.
type TreeGrepOptions struct {
	Roots []string
	// Parwalk configures the walk of the roots. Its Context is ignored:
	// the walk is cancelled through the context passed to Grep.
	Parwalk          ParwalkOptions
	Pattern          *regexp.Regexp
	MaxFileSize      uint64
	MaxLineSize      int
	ProgressInterval time.Duration
	// Filter is called for every walked edge. Returning false excludes the
	// entry; excluding a directory also prunes its subtree.
	Filter func(string, msgs.InodeId) (bool, error)
}

// TreeGrepFileError reports a file which could not be completely searched.
type TreeGrepFileError struct {
	Path  string
	Inode msgs.InodeId
	Err   error
}

// TreeGrepStats is a point-in-time search progress snapshot.
type TreeGrepStats struct {
	FilesFound         uint64
	FilesScanned       uint64
	BytesScanned       uint64
	MatchesFound       uint64
	FilesSkippedBySize uint64
	FileErrors         uint64
}

// TreeGrepCallbacks receives serialized output. Returning an error cancels the
// search.
type TreeGrepCallbacks struct {
	Match     func(GrepMatch) error
	FileError func(TreeGrepFileError) error
	Progress  func(TreeGrepStats) error
}

// TreeGrep runs GrepFile on every file under a set of directories. It holds
// a limit on concurrent file reads. Each Grep call walks with its own Parwalk
// workers, and concurrent Grep calls share the same file-read budget.
type TreeGrep struct {
	log     *log.Logger
	client  *Client
	bufPool *bufpool.BufPool
	sem     chan struct{}
}

type treeGrepCounters struct {
	filesFound         atomic.Uint64
	filesScanned       atomic.Uint64
	bytesScanned       atomic.Uint64
	matchesFound       atomic.Uint64
	filesSkippedBySize atomic.Uint64
	fileErrors         atomic.Uint64
}

func (counters *treeGrepCounters) snapshot() TreeGrepStats {
	return TreeGrepStats{
		FilesFound:         counters.filesFound.Load(),
		FilesScanned:       counters.filesScanned.Load(),
		BytesScanned:       counters.bytesScanned.Load(),
		MatchesFound:       counters.matchesFound.Load(),
		FilesSkippedBySize: counters.filesSkippedBySize.Load(),
		FileErrors:         counters.fileErrors.Load(),
	}
}

type treeGrepRun struct {
	ctx       context.Context
	cancel    context.CancelCauseFunc
	options   *TreeGrepOptions
	callbacks TreeGrepCallbacks
	counters  treeGrepCounters
	pending   sync.WaitGroup
	emitMutex sync.Mutex
}

func (run *treeGrepRun) emit(callback func() error) error {
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

// NewTreeGrep creates a tree grep using Parwalk for traversal.
// grepWorkers bounds concurrent file reads across all Grep calls.
func NewTreeGrep(
	log *log.Logger,
	client *Client,
	grepWorkers int,
) *TreeGrep {
	if grepWorkers < 1 {
		panic(fmt.Errorf("grepWorkers=%d < 1", grepWorkers))
	}
	return &TreeGrep{
		log:     log,
		client:  client,
		bufPool: bufpool.NewBufPool(),
		sem:     make(chan struct{}, grepWorkers),
	}
}

// Grep walks the roots and searches regular files. Individual file failures
// are reported through FileError and do not stop the remaining search.
//
// Acquiring the file-read budget blocks inside the walk callback, so the walk
// does not run ahead of the file reads.
func (grep *TreeGrep) Grep(
	ctx context.Context,
	options *TreeGrepOptions,
	callbacks TreeGrepCallbacks,
) (TreeGrepStats, error) {
	if options == nil {
		return TreeGrepStats{}, errors.New("tree grep options are required")
	}
	if len(options.Roots) == 0 {
		return TreeGrepStats{}, errors.New("tree grep requires at least one root")
	}
	if options.Pattern == nil {
		return TreeGrepStats{}, errors.New("tree grep pattern is required")
	}

	grepCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	run := &treeGrepRun{
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

	parwalkOptions := options.Parwalk
	parwalkOptions.Context = grepCtx
	walkErr := ParwalkMany(
		grep.log,
		grep.client,
		&parwalkOptions,
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

func (grep *TreeGrep) grepFile(
	run *treeGrepRun,
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

func (grep *TreeGrep) publishFileError(
	run *treeGrepRun,
	filePath string,
	id msgs.InodeId,
	err error,
) {
	run.counters.fileErrors.Add(1)
	_ = run.emit(func() error {
		if run.callbacks.FileError == nil {
			return nil
		}
		return run.callbacks.FileError(TreeGrepFileError{
			Path:  filePath,
			Inode: id,
			Err:   err,
		})
	})
}
