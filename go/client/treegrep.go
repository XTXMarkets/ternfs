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
	"time"

	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"golang.org/x/sync/errgroup"
)

// TreeGrepOptions describes one current-tree regular-expression search.
type TreeGrepOptions struct {
	Roots       []string
	Parwalk     ParwalkOptions
	Pattern     *regexp.Regexp
	MaxFileSize uint64
	MaxLineSize int
	// SearchBinary bypasses the default text detection on each file's prefix.
	SearchBinary     bool
	ProgressInterval time.Duration
	// Filter is called for every walked edge. Returning false excludes the
	// entry; excluding a directory also prunes its subtree.
	Filter func(string, msgs.InodeId) (bool, error)
}

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
	FilesSkippedBinary uint64
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

type treeGrepRun struct {
	group     *errgroup.Group
	ctx       context.Context
	options   *TreeGrepOptions
	callbacks TreeGrepCallbacks
	// serializes the callbacks, and guards stats
	mutex sync.Mutex
	stats TreeGrepStats
}

func (run *treeGrepRun) emit(callback func() error) error {
	run.mutex.Lock()
	defer run.mutex.Unlock()
	if err := context.Cause(run.ctx); err != nil {
		return err
	}
	return callback()
}

func (run *treeGrepRun) reportProgress(cancel context.CancelCauseFunc) error {
	return run.emit(func() error {
		err := run.callbacks.Progress(run.stats)
		if err != nil {
			cancel(err)
		}
		return err
	})
}

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
// Files identified as binary are counted as skipped unless SearchBinary is set.
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

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	group, grepCtx := errgroup.WithContext(ctx)
	run := &treeGrepRun{
		group:     group,
		ctx:       grepCtx,
		options:   options,
		callbacks: callbacks,
	}

	var progress sync.WaitGroup
	if callbacks.Progress != nil && options.ProgressInterval > 0 {
		progress.Go(func() {
			ticker := time.NewTicker(options.ProgressInterval)
			defer ticker.Stop()
			for {
				select {
				case <-grepCtx.Done():
					return
				case <-ticker.C:
					if err := run.reportProgress(cancel); err != nil {
						return
					}
				}
			}
		})
	}

	group.Go(func() error {
		parwalkOptions := options.Parwalk
		parwalkOptions.FailOnReadDirError = true
		return ParwalkMany(
			grepCtx,
			grep.log,
			grep.client,
			&parwalkOptions,
			options.Roots,
			func(walkCtx context.Context, _ msgs.InodeId, parentPath string, name string, _ msgs.TernTime, id msgs.InodeId, _ bool, _ bool) error {
				return grep.visit(run, walkCtx, path.Join(parentPath, name), id)
			},
		)
	})
	// Wait cancels grepCtx on its way out, which stops the progress reports.
	err := group.Wait()
	progress.Wait()
	if err == nil {
		err = context.Cause(ctx)
	}
	return run.stats, err
}

func (grep *TreeGrep) visit(run *treeGrepRun, walkCtx context.Context, fullPath string, id msgs.InodeId) error {
	if run.options.Filter != nil {
		include, err := run.options.Filter(fullPath, id)
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
	select {
	case <-walkCtx.Done():
		return context.Cause(walkCtx)
	case grep.sem <- struct{}{}:
	}
	run.mutex.Lock()
	run.stats.FilesFound++
	run.mutex.Unlock()
	run.group.Go(func() error {
		defer func() { <-grep.sem }()
		return grep.grepFile(run, fullPath, id)
	})
	return nil
}

func (grep *TreeGrep) grepFile(
	run *treeGrepRun,
	filePath string,
	id msgs.InodeId,
) error {
	var emitErr error
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
		run.options.SearchBinary,
		func(match GrepMatch) error {
			emitErr = run.emit(func() error {
				run.stats.MatchesFound++
				if run.callbacks.Match == nil {
					return nil
				}
				return run.callbacks.Match(match)
			})
			return emitErr
		},
	)
	run.mutex.Lock()
	defer run.mutex.Unlock()
	run.stats.BytesScanned += bytesScanned
	switch {
	case emitErr != nil:
		return emitErr
	case run.ctx.Err() != nil:
		// cancelled part way through the file
	case err == nil:
		run.stats.FilesScanned++
	case errors.Is(err, ErrGrepFileTooLarge):
		run.stats.FilesSkippedBySize++
	case errors.Is(err, ErrGrepBinaryFile):
		run.stats.FilesSkippedBinary++
	default:
		run.stats.FileErrors++
		if run.callbacks.FileError != nil {
			return run.callbacks.FileError(TreeGrepFileError{Path: filePath, Inode: id, Err: err})
		}
	}
	return nil
}
