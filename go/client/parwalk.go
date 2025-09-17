// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// When you want to traverse the filesystem, but you also want the
// filepath. We have some workers per shard, to try to parallelize
// the work nicely. However there is a work-stealing of sorts otherwise
// it's very easy to end in deadlocks.
package client

import (
	"fmt"
	"path"
	"sync"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

type parwarlkReq struct {
	id   msgs.InodeId
	path string
}

type parwalkEnv struct {
	wg       sync.WaitGroup
	chans    []chan parwarlkReq
	client   *Client
	snapshot bool
	callback func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error
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
	if err := env.callback(parent, parentPath, name, creationTime, id, current, owned); err != nil {
		return err
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
	path string,
) error {
	if env.snapshot {
		req := &msgs.FullReadDirReq{
			DirId: id,
		}
		resp := &msgs.FullReadDirResp{}
		for {
			if err := env.client.ShardRequest(log, id.Shard(), req, resp); err != nil {
				log.Debug("failed to read dir %v at path %q, it might have been deleted in the meantime: %v", id, path, err)
				return nil
			}
			for _, e := range resp.Results {
				if e.TargetId.Id() == msgs.NULL_INODE_ID { // no point looking at deletion edges
					continue
				}
				if err := env.visit(log, homeShid, id, path, e.Name, e.CreationTime, e.TargetId.Id(), e.Current, e.Current || e.TargetId.Extra()); err != nil {
					return err
				}
			}
			if resp.Next.StartName == "" {
				break
			}
			req.Flags = 0
			if resp.Next.Current {
				req.Flags = msgs.FULL_READ_DIR_CURRENT
			}
			req.StartName = resp.Next.StartName
			req.StartTime = resp.Next.StartTime
		}
	} else {
		readReq := &msgs.ReadDirReq{
			DirId: id,
		}
		readResp := &msgs.ReadDirResp{}
		for {
			if err := env.client.ShardRequest(log, id.Shard(), readReq, readResp); err != nil {
				log.Debug("failed to read dir %v at path %q, it might have been deleted in the meantime: %v", id, path, err)
				return nil
			}
			for _, e := range readResp.Results {
				if err := env.visit(log, homeShid, id, path, e.Name, e.CreationTime, e.TargetId, true, true); err != nil {
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

type ParwalkOptions struct {
	WorkersPerShard int
	Snapshot        bool
}

func Parwalk(
	log *log.Logger,
	client *Client,
	options *ParwalkOptions,
	root string,
	callback func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error,
) error {
	if options.WorkersPerShard < 1 {
		panic(fmt.Errorf("workersPerShard=%d < 1", options.WorkersPerShard))
	}
	// compute
	env := parwalkEnv{
		chans:    make([]chan parwarlkReq, 256),
		client:   client,
		callback: callback,
		snapshot: options.Snapshot,
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
	rootId, creationTime, parentId, err := client.ResolvePathWithParent(log, root)
	if err != nil {
		return err
	}
	if err := env.visit(log, 0, parentId, path.Dir(root), path.Base(root), creationTime, rootId, true, true); err != nil {
		return err
	}
	go func() {
		env.wg.Wait()
		errChan <- nil
	}()
	return <-errChan
}
