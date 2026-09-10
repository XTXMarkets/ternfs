// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// When you want to traverse the filesystem, but you also want the
// filepath. We have some workers per shard, to try to parallelize
// the work nicely. However there is a work-stealing of sorts otherwise
// it's very easy to end in deadlocks.
package client

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

// ErrSkipSubtree, when returned from a Parwalk callback, tells Parwalk not to
// descend into the current entry. Inspired by filepath.SkipDir. Returning it
// for a non-directory entry is harmless -- there's nothing to skip.
var ErrSkipSubtree = errors.New("parwalk: skip subtree")

// ParwalkVisitFunc is invoked for every edge visited by a ParwalkPool. Callers
// that need cancellation inside the visit function should close over the context
// passed to Walk, WalkMany, or WalkFromInode.
type ParwalkVisitFunc func(
	parent msgs.InodeId,
	parentPath string,
	name string,
	creationTime msgs.TernTime,
	id msgs.InodeId,
	current bool,
	owned bool,
) error

// visitAndRecurse invokes the run's onEdge callback for one edge, and then,
// if the edge points at an owned directory, recurses into it -- inline on the
// current goroutine when the directory lives on the same shard, otherwise by
// scheduling it on the owning shard's workers.
func (run *parwalkRun) visitAndRecurse(
	homeShid msgs.ShardId,
	parent msgs.InodeId,
	parentPath string,
	name string,
	creationTime msgs.TernTime,
	id msgs.InodeId,
	current bool,
	owned bool,
) error {
	if err := run.cause(); err != nil {
		return err
	}
	cbErr := run.onEdge(
		parent,
		parentPath,
		name,
		creationTime,
		id,
		current,
		owned,
	)
	if errors.Is(cbErr, ErrSkipSubtree) {
		return nil
	}
	if cbErr != nil {
		return cbErr
	}
	// if it's not a directory, skip
	if id.Type() != msgs.DIRECTORY {
		return nil
	}
	// if it's not owned, skip
	if !owned && !run.Snapshot {
		return nil
	}
	fullPath := path.Join(parentPath, name)
	if parent != msgs.NULL_INODE_ID && homeShid == id.Shard() {
		// same shard, handle right now
		return run.process(homeShid, id, fullPath)
	}
	return run.schedule(homeShid, id, fullPath)
}

func (run *parwalkRun) process(
	homeShid msgs.ShardId,
	id msgs.InodeId,
	pathStr string,
) error {
	if err := run.cause(); err != nil {
		return err
	}
	if run.SnapshotLatest {
		return run.processSnapshotLatest(homeShid, id, pathStr)
	}
	if run.Snapshot {
		cursor := msgs.FullReadDirCursor{}
		for {
			if err := run.cause(); err != nil {
				return err
			}
			flags := msgs.FullReadDirFlags(0)
			if cursor.Current {
				flags = msgs.FULL_READ_DIR_CURRENT
			}
			resp, err := run.pool.client.fullReadDir(
				run.pool.log,
				id,
				flags,
				cursor.StartName,
				cursor.StartTime,
			)
			if err != nil {
				run.pool.log.Debug("failed to read dir %v at path %q, it might have been deleted in the meantime: %v", id, pathStr, err)
				return nil
			}
			for _, e := range resp.Results {
				if e.TargetId.Id() == msgs.NULL_INODE_ID { // no point looking at deletion edges
					continue
				}
				if err := run.visitAndRecurse(homeShid, id, pathStr, e.Name, e.CreationTime, e.TargetId.Id(), e.Current, e.Current || e.TargetId.Extra()); err != nil {
					return err
				}
			}
			if resp.Next.StartName == "" {
				break
			}
			cursor = resp.Next
		}
	} else {
		readReq := &msgs.ReadDirReq{
			DirId: id,
		}
		readResp := &msgs.ReadDirResp{}
		for {
			if err := run.cause(); err != nil {
				return err
			}
			if err := run.pool.client.ShardRequest(run.pool.log, id.Shard(), readReq, readResp); err != nil {
				run.pool.log.Debug("failed to read dir %v at path %q, it might have been deleted in the meantime: %v", id, pathStr, err)
				return nil
			}
			for _, e := range readResp.Results {
				if err := run.visitAndRecurse(homeShid, id, pathStr, e.Name, e.CreationTime, e.TargetId, true, true); err != nil {
					return err
				}
			}
			if readResp.NextHash == 0 {
				break
			}
			readReq.StartHash = readResp.NextHash
		}
	}
	return nil
}

// processSnapshotLatest reads all snapshot edges of a directory newest-first
// and keeps, per name, the newest edge whose target is not NULL.
func (run *parwalkRun) processSnapshotLatest(
	homeShid msgs.ShardId,
	id msgs.InodeId,
	pathStr string,
) error {
	type latestEdge struct {
		targetId     msgs.InodeId
		creationTime msgs.TernTime
		current      bool
		owned        bool
	}
	latest := map[string]latestEdge{}
	cursor := msgs.FullReadDirCursor{}
	for {
		if err := run.cause(); err != nil {
			return err
		}
		flags := msgs.FULL_READ_DIR_BACKWARDS
		if cursor.Current {
			flags |= msgs.FULL_READ_DIR_CURRENT
		}
		resp, err := run.pool.client.fullReadDir(
			run.pool.log,
			id,
			flags,
			cursor.StartName,
			cursor.StartTime,
		)
		if err != nil {
			run.pool.log.Debug("failed to read dir %v at path %q, it might have been deleted in the meantime: %v", id, pathStr, err)
			return nil
		}
		for _, e := range resp.Results {
			if e.TargetId.Id() == msgs.NULL_INODE_ID {
				continue
			}
			if _, seen := latest[e.Name]; seen {
				continue
			}
			latest[e.Name] = latestEdge{
				targetId:     e.TargetId.Id(),
				creationTime: e.CreationTime,
				current:      e.Current,
				owned:        e.Current || e.TargetId.Extra(),
			}
		}
		if resp.Next.StartName == "" {
			break
		}
		cursor = resp.Next
	}
	for name, e := range latest {
		if err := run.visitAndRecurse(homeShid, id, pathStr, name, e.creationTime, e.targetId, e.current, e.owned); err != nil {
			return err
		}
	}
	return nil
}

type ParwalkOptions struct {
	Snapshot bool
	// SnapshotLatest, when true, iterates snapshot edges backwards (newest
	// first) and invokes the callback only for the newest non-null edge per
	// name. NULL (deletion) edges are ignored. Implies Snapshot=true. Useful
	// for walking a deleted/historical subtree.
	SnapshotLatest bool
}

// normalized returns the options as a value, folding SnapshotLatest's implied
// Snapshot into it.
func (options *ParwalkOptions) normalized() ParwalkOptions {
	if options == nil {
		return ParwalkOptions{}
	}
	normalized := *options
	normalized.Snapshot = normalized.Snapshot || normalized.SnapshotLatest
	return normalized
}

// parwalkSeed is one root edge of a walk: the edge leading from the root's
// parent to the root itself.
type parwalkSeed struct {
	parentId     msgs.InodeId
	parentPath   string
	name         string
	creationTime msgs.TernTime
	rootId       msgs.InodeId
}

func parwalkRunWithSeeds(
	pool *ParwalkPool,
	ctx context.Context,
	options ParwalkOptions,
	onEdge ParwalkVisitFunc,
	seeds []parwalkSeed,
) error {
	prime := func(run *parwalkRun) error {
		for _, seed := range seeds {
			if err := run.visitAndRecurse(
				0,
				seed.parentId,
				seed.parentPath,
				seed.name,
				seed.creationTime,
				seed.rootId,
				true,
				true,
			); err != nil {
				return err
			}
		}
		return nil
	}
	return pool.run(ctx, prime, onEdge, options)
}

// Walk traverses one path using the shared workers.
func (pool *ParwalkPool) Walk(
	ctx context.Context,
	options *ParwalkOptions,
	root string,
	onEdge ParwalkVisitFunc,
) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	rootId, creationTime, parentId, err :=
		pool.client.ResolvePathWithParent(pool.log, root)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", root, err)
	}
	return parwalkRunWithSeeds(pool, ctx, options.normalized(), onEdge, []parwalkSeed{
		{
			parentId:     parentId,
			parentPath:   path.Dir(root),
			name:         path.Base(root),
			creationTime: creationTime,
			rootId:       rootId,
		},
	})
}

// WalkFromInode is like Walk but takes an inode id as the root instead
// of a path. `rootPath` is a cookie passed through to the callback as the
// parentPath of the root's children; it is not resolved against the
// filesystem, so callers may pass any string meaningful to their callback
// (e.g. a destination path when walking a deleted subtree to reconstruct it
// somewhere else).
func (pool *ParwalkPool) WalkFromInode(
	ctx context.Context,
	options *ParwalkOptions,
	rootId msgs.InodeId,
	rootPath string,
	onEdge ParwalkVisitFunc,
) error {
	return parwalkRunWithSeeds(pool, ctx, options.normalized(), onEdge, []parwalkSeed{
		{
			parentId:   msgs.NULL_INODE_ID,
			parentPath: path.Dir(rootPath),
			name:       path.Base(rootPath),
			rootId:     rootId,
		},
	})
}

// WalkMany walks several roots through a single shared 256×WorkersPerShard
// goroutine pool. Roots are resolved up front (so a typo in any root fails
// fast) and then seeded one after another into the same pool, so the workers
// servicing one shard process edges from every root concurrently rather than
// idling between roots.
//
// The visit contract is identical to Walk; callers that want to know
// which root an edge belongs to can derive it from the parentPath argument.
func (pool *ParwalkPool) WalkMany(
	ctx context.Context,
	options *ParwalkOptions,
	roots []string,
	onEdge ParwalkVisitFunc,
) error {
	seeds := make([]parwalkSeed, 0, len(roots))
	for _, root := range roots {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		rootId, creationTime, parentId, err :=
			pool.client.ResolvePathWithParent(pool.log, root)
		if err != nil {
			return fmt.Errorf("resolve %q: %w", root, err)
		}
		seeds = append(seeds, parwalkSeed{
			parentId:     parentId,
			parentPath:   path.Dir(root),
			name:         path.Base(root),
			creationTime: creationTime,
			rootId:       rootId,
		})
	}
	return parwalkRunWithSeeds(pool, ctx, options.normalized(), onEdge, seeds)
}
