// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// When you want to traverse the filesystem, but you also want the
// filepath. We have some workers per shard, to try to parallelize
// the work nicely. However there is a work-stealing of sorts otherwise
// it's very easy to end in deadlocks.
package client

import (
	"errors"
	"fmt"
	"path"
	"sync"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

// ErrSkipSubtree, when returned from a Parwalk callback, tells Parwalk not to
// descend into the current entry. Inspired by filepath.SkipDir. Returning it
// for a non-directory entry is harmless -- there's nothing to skip.
var ErrSkipSubtree = errors.New("parwalk: skip subtree")

type parwarlkReq struct {
	id   msgs.InodeId
	path string
}

type parwalkEnv struct {
	wg             sync.WaitGroup
	chans          []chan parwarlkReq
	client         *Client
	snapshot       bool
	snapshotLatest bool
	callback       func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error
}

func (env *parwalkEnv) visit(
	log *log.Logger,
	homeShid msgs.ShardId,
	parent msgs.InodeId,
	parentPath string,
	name string,
	creationTime msgs.TernTime,
	id msgs.InodeId,
	current bool,
	owned bool,
) error {
	cbErr := env.callback(parent, parentPath, name, creationTime, id, current, owned)
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
	if !owned && !env.snapshot {
		return nil
	}
	fullPath := path.Join(parentPath, name)
	if parent != msgs.NULL_INODE_ID && homeShid == id.Shard() {
		// same shard, handle right now
		env.process(log, homeShid, id, fullPath)
	} else {
		req := parwarlkReq{
			id:   id,
			path: fullPath,
		}
		select {
		// pass to other shard
		case env.chans[id.Shard()] <- req:
			env.wg.Add(1)
		// queue is full, do it yourself
		default:
			env.process(log, homeShid, id, fullPath)
		}
	}
	return nil
}

func (env *parwalkEnv) process(
	log *log.Logger,
	homeShid msgs.ShardId,
	id msgs.InodeId,
	pathStr string,
) error {
	if env.snapshotLatest {
		return env.processSnapshotLatest(log, homeShid, id, pathStr)
	}
	if env.snapshot {
		cursor := msgs.FullReadDirCursor{}
		for {
			flags := msgs.FullReadDirFlags(0)
			if cursor.Current {
				flags = msgs.FULL_READ_DIR_CURRENT
			}
			resp, err := env.client.fullReadDir(
				log,
				id,
				flags,
				cursor.StartName,
				cursor.StartTime,
			)
			if err != nil {
				log.Debug("failed to read dir %v at path %q, it might have been deleted in the meantime: %v", id, pathStr, err)
				return nil
			}
			for _, e := range resp.Results {
				if e.TargetId.Id() == msgs.NULL_INODE_ID { // no point looking at deletion edges
					continue
				}
				if err := env.visit(log, homeShid, id, pathStr, e.Name, e.CreationTime, e.TargetId.Id(), e.Current, e.Current || e.TargetId.Extra()); err != nil {
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
			if err := env.client.ShardRequest(log, id.Shard(), readReq, readResp); err != nil {
				log.Debug("failed to read dir %v at path %q, it might have been deleted in the meantime: %v", id, pathStr, err)
				return nil
			}
			for _, e := range readResp.Results {
				if err := env.visit(log, homeShid, id, pathStr, e.Name, e.CreationTime, e.TargetId, true, true); err != nil {
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
func (env *parwalkEnv) processSnapshotLatest(
	log *log.Logger,
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
		flags := msgs.FULL_READ_DIR_BACKWARDS
		if cursor.Current {
			flags |= msgs.FULL_READ_DIR_CURRENT
		}
		resp, err := env.client.fullReadDir(
			log,
			id,
			flags,
			cursor.StartName,
			cursor.StartTime,
		)
		if err != nil {
			log.Debug("failed to read dir %v at path %q, it might have been deleted in the meantime: %v", id, pathStr, err)
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
		if err := env.visit(log, homeShid, id, pathStr, name, e.creationTime, e.targetId, e.current, e.owned); err != nil {
			return err
		}
	}
	return nil
}

type ParwalkOptions struct {
	WorkersPerShard int
	Snapshot        bool
	// SnapshotLatest, when true, iterates snapshot edges backwards (newest
	// first) and invokes the callback only for the newest non-null edge per
	// name. NULL (deletion) edges are ignored. Implies Snapshot=true. Useful
	// for walking a deleted/historical subtree.
	SnapshotLatest bool
}

// Parwalk traverses the subtree rooted at `root` (a path), invoking callback
// for each edge it visits. See ParwalkFromInode for a variant that starts
// from an inode id (useful when the root has no live path, e.g. a deleted
// directory).
func Parwalk(
	log *log.Logger,
	client *Client,
	options *ParwalkOptions,
	root string,
	callback func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error,
) error {
	rootId, creationTime, parentId, err := client.ResolvePathWithParent(log, root)
	if err != nil {
		return err
	}
	return parwalkRunWithSeeds(log, client, options, callback, []parwalkSeed{
		{parentId: parentId, parentPath: path.Dir(root), name: path.Base(root), creationTime: creationTime, rootId: rootId},
	})
}

// ParwalkFromInode is like Parwalk but takes an inode id as the root instead
// of a path. `rootPath` is a cookie passed through to the callback as the
// parentPath of the root's children; it is not resolved against the
// filesystem, so callers may pass any string meaningful to their callback
// (e.g. a destination path when walking a deleted subtree to reconstruct it
// somewhere else).
func ParwalkFromInode(
	log *log.Logger,
	client *Client,
	options *ParwalkOptions,
	rootId msgs.InodeId,
	rootPath string,
	callback func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error,
) error {
	return parwalkRunWithSeeds(log, client, options, callback, []parwalkSeed{
		{parentId: msgs.NULL_INODE_ID, parentPath: path.Dir(rootPath), name: path.Base(rootPath), creationTime: 0, rootId: rootId},
	})
}

// ParwalkMany walks several roots through a single shared 256×WorkersPerShard
// goroutine pool. Roots are resolved up front (so a typo in any root fails
// fast) and then seeded one after another into the same pool, so the workers
// servicing one shard process edges from every root concurrently rather than
// idling between roots.
//
// The callback contract is identical to Parwalk; callers that want to know
// which root an edge belongs to can derive it from the parentPath argument.
func ParwalkMany(
	log *log.Logger,
	client *Client,
	options *ParwalkOptions,
	roots []string,
	callback func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error,
) error {
	seeds := make([]parwalkSeed, 0, len(roots))
	for _, root := range roots {
		rootId, creationTime, parentId, err := client.ResolvePathWithParent(log, root)
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
	return parwalkRunWithSeeds(log, client, options, callback, seeds)
}

type parwalkSeed struct {
	parentId     msgs.InodeId
	parentPath   string
	name         string
	creationTime msgs.TernTime
	rootId       msgs.InodeId
}

func parwalkRunWithSeeds(
	log *log.Logger,
	client *Client,
	options *ParwalkOptions,
	callback func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error,
	seeds []parwalkSeed,
) error {
	if options.WorkersPerShard < 1 {
		panic(fmt.Errorf("workersPerShard=%d < 1", options.WorkersPerShard))
	}
	env := parwalkEnv{
		chans:          make([]chan parwarlkReq, 256),
		client:         client,
		callback:       callback,
		snapshot:       options.Snapshot || options.SnapshotLatest,
		snapshotLatest: options.SnapshotLatest,
	}
	for i := 0; i < 256; i++ {
		env.chans[i] = make(chan parwarlkReq, 10_000)
	}
	errChan := make(chan error, 1)
	for i := 0; i < 256; i++ {
		shid := msgs.ShardId(i)
		ch := env.chans[shid]
		for j := 0; j < options.WorkersPerShard; j++ {
			go func() {
				for {
					req, more := <-ch
					if !more {
						return
					}
					if err := env.process(log, shid, req.id, req.path); err != nil {
						for _, ch := range env.chans {
							close(ch)
						}
						select {
						case errChan <- err:
						default:
						}
						return
					}
					env.wg.Done()
				}
			}()
		}
	}
	for _, s := range seeds {
		if err := env.visit(log, 0, s.parentId, s.parentPath, s.name, s.creationTime, s.rootId, true, true); err != nil {
			return err
		}
	}
	go func() {
		env.wg.Wait()
		errChan <- nil
	}()
	return <-errChan
}
