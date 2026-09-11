// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

func NewResurrectSubtree() Command {
	resurrectSubtreeCmd := flag.NewFlagSet("resurrect-subtree", flag.ExitOnError)
	resurrectSubtreeSrcId := resurrectSubtreeCmd.Uint64("src-id", 0, "Inode id of the deleted source directory (required)")
	resurrectSubtreeDst := resurrectSubtreeCmd.String("dst", "", "Destination path in TernFS; must not yet exist (required)")
	resurrectSubtreeWorkersPerShard := resurrectSubtreeCmd.Int("workers-per-shard", 4, "Directory-walk concurrency per shard (see Parwalk)")
	resurrectSubtreeFileWorkers := resurrectSubtreeCmd.Int("file-workers", 16, "Concurrency for copying file contents")
	resurrectSubtreeDryRun := resurrectSubtreeCmd.Bool("dry-run", false, "Log what would happen without creating anything")
	resurrectSubtreeRun := func(runtime *Runtime) {
		l := runtime.Log
		if *resurrectSubtreeSrcId == 0 {
			panic(fmt.Errorf("-src-id is required"))
		}
		if *resurrectSubtreeDst == "" {
			panic(fmt.Errorf("-dst is required"))
		}
		if *resurrectSubtreeWorkersPerShard < 1 {
			panic(fmt.Errorf("-workers-per-shard must be > 0"))
		}
		if *resurrectSubtreeFileWorkers < 1 {
			panic(fmt.Errorf("-file-workers must be > 0"))
		}
		srcRootId := msgs.InodeId(*resurrectSubtreeSrcId)
		if srcRootId.Type() != msgs.DIRECTORY {
			panic(fmt.Errorf("-src-id %v is not a directory inode", srcRootId))
		}
		c := runtime.Client()

		// 1. Verify source is a deleted directory.
		srcStat := msgs.StatDirectoryResp{}
		if err := c.ShardRequest(l, srcRootId.Shard(), &msgs.StatDirectoryReq{Id: srcRootId}, &srcStat); err != nil {
			panic(fmt.Errorf("could not stat source directory %v: %w", srcRootId, err))
		}
		if srcStat.Owner != msgs.NULL_INODE_ID {
			panic(fmt.Errorf("source directory %v is not deleted (owner=%v); resurrect-subtree only operates on snapshot directories", srcRootId, srcStat.Owner))
		}

		// 2. Resolve destination parent and create the root destination dir.
		dstRootPath := filepath.Clean("/" + *resurrectSubtreeDst)
		if dstRootPath == "/" {
			panic(fmt.Errorf("-dst must not be the filesystem root"))
		}
		dstParentPath := path.Dir(dstRootPath)
		dstBase := path.Base(dstRootPath)
		dstParentId, err := c.ResolvePath(l, dstParentPath)
		if err != nil {
			panic(fmt.Errorf("could not resolve destination parent %q: %w", dstParentPath, err))
		}

		// Refuse to proceed if the destination already exists.
		lookupResp := msgs.LookupResp{}
		if err := c.ShardRequest(l, dstParentId.Shard(), &msgs.LookupReq{DirId: dstParentId, Name: dstBase}, &lookupResp); err == nil {
			panic(fmt.Errorf("destination %q already exists as inode %v", dstRootPath, lookupResp.TargetId))
		}

		makeDir := func(parent msgs.InodeId, name string) msgs.InodeId {
			if *resurrectSubtreeDryRun {
				return 0
			}
			resp := msgs.MakeDirectoryResp{}
			if err := c.CDCRequest(l, &msgs.MakeDirectoryReq{OwnerId: parent, Name: name}, &resp); err != nil {
				panic(fmt.Errorf("could not make directory %q under %v: %w", name, parent, err))
			}
			return resp.Id
		}

		var dstRootId msgs.InodeId
		if *resurrectSubtreeDryRun {
			l.Info("[dry-run] would mkdir %q under %v", dstBase, dstParentId)
		} else {
			dstRootId = makeDir(dstParentId, dstBase)
			l.Info("created destination root %q as inode %v", dstRootPath, dstRootId)
		}

		// 3. Worker pool for file copies, shared across all shard walkers.
		type fileCopyJob struct {
			srcFileId msgs.InodeId
			dstPath   string
		}
		var copiedFiles uint64
		var createdDirs uint64
		t0 := time.Now()
		fileJobs := make(chan fileCopyJob, *resurrectSubtreeFileWorkers*4)
		var fileWg sync.WaitGroup
		for i := 0; i < *resurrectSubtreeFileWorkers; i++ {
			fileWg.Add(1)
			go func() {
				defer fileWg.Done()
				bufPool := bufpool.NewBufPool()
				cache := client.NewDirInfoCache()
				for job := range fileJobs {
					buf, err := c.FetchFile(l, bufPool, job.srcFileId)
					if err != nil {
						panic(fmt.Errorf("could not fetch file %v for %q: %w", job.srcFileId, job.dstPath, err))
					}
					if _, err := c.CreateFile(l, bufPool, cache, job.dstPath, bytes.NewReader(buf.Bytes())); err != nil {
						panic(fmt.Errorf("could not create file %q: %w", job.dstPath, err))
					}
					bufPool.Put(buf)
					n := atomic.AddUint64(&copiedFiles, 1)
					if n%1000 == 0 {
						elapsed := time.Since(t0).Seconds()
						l.Info("copied %v files (%0.1f files/s), %v directories created",
							n, float64(n)/elapsed, atomic.LoadUint64(&createdDirs))
					}
				}
			}()
		}

		// 4. Walk the source subtree via ParwalkFromInode in SnapshotLatest mode:
		//    for each name, only the newest non-deletion snapshot edge is
		//    visited; deletion edges mask older entries for that name.
		//    We keep a src-dir-id -> dst-dir-id map so child callbacks know
		//    which freshly-created destination directory to drop their
		//    entries under. The store happens inside the callback before
		//    Parwalk descends, so children always find their parent.
		var dstDirIds sync.Map
		dstDirIds.Store(srcRootId, dstRootId)
		err = client.ParwalkFromInode(
			l,
			c,
			&client.ParwalkOptions{
				WorkersPerShard: *resurrectSubtreeWorkersPerShard,
				SnapshotLatest:  true,
			},
			srcRootId,
			dstRootPath,
			func(parent msgs.InodeId, parentPath string, name string, creationTime msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
				if parent == msgs.NULL_INODE_ID {
					// Root: already created outside the walk.
					return nil
				}
				dstParentAny, ok := dstDirIds.Load(parent)
				if !ok {
					return fmt.Errorf("no destination mapping for source parent %v", parent)
				}
				dstParentDirId := dstParentAny.(msgs.InodeId)
				childDstPath := path.Join(parentPath, name)
				if id.Type() == msgs.DIRECTORY {
					// Edge-level "owned" is unreliable for directories (a
					// renamed dir leaves a non-owned snapshot behind but is
					// still live elsewhere). Stat the target: restore only if
					// it's itself orphaned.
					childStat := msgs.StatDirectoryResp{}
					if err := c.ShardRequest(l, id.Shard(), &msgs.StatDirectoryReq{Id: id}, &childStat); err != nil {
						l.ErrorNoAlert("could not stat directory %v at %q, skipping subtree: %v", id, childDstPath, err)
						return client.ErrSkipSubtree
					}
					if childStat.Owner != msgs.NULL_INODE_ID {
						l.Info("skipping %q: directory %v still owned by %v", childDstPath, id, childStat.Owner)
						return client.ErrSkipSubtree
					}
					var childDstId msgs.InodeId
					if *resurrectSubtreeDryRun {
						l.Info("[dry-run] would mkdir %q (from deleted dir %v)", childDstPath, id)
					} else {
						childDstId = makeDir(dstParentDirId, name)
						atomic.AddUint64(&createdDirs, 1)
					}
					dstDirIds.Store(id, childDstId)
					return nil
				}
				// File entry.
				if !owned {
					l.Info("skipping %q: file %v is not owned by this directory", childDstPath, id)
					return nil
				}
				if *resurrectSubtreeDryRun {
					l.Info("[dry-run] would copy file %v -> %q", id, childDstPath)
					atomic.AddUint64(&copiedFiles, 1)
					return nil
				}
				fileJobs <- fileCopyJob{srcFileId: id, dstPath: childDstPath}
				return nil
			},
		)
		close(fileJobs)
		fileWg.Wait()
		if err != nil {
			panic(err)
		}
		l.Info("resurrect-subtree done: %v files copied, %v directories created in %v",
			atomic.LoadUint64(&copiedFiles), atomic.LoadUint64(&createdDirs), time.Since(t0))
	}
	return Command{
		Flags: resurrectSubtreeCmd,
		Run:   resurrectSubtreeRun,
	}
}
