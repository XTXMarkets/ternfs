// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"xtx/ternfs/client"
	"xtx/ternfs/core/log"
	lrecover "xtx/ternfs/core/recover"
	"xtx/ternfs/core/wyhash"
	"xtx/ternfs/msgs"
)

type createInode struct {
	id           msgs.InodeId
	creationTime msgs.TernTime
}

func parallelDirsTest(
	log *log.Logger,
	registryAddress string,
	counters *client.ClientCounters,
) {
	numRootDirs := 10
	numThreads := 1000
	numThreadsDigits := int(math.Ceil(math.Log10(float64(numThreads))))
	actionsPerThread := 100

	entityName := func(tid int, i int) string {
		return fmt.Sprintf(fmt.Sprintf("%%0%dd-%%d", numThreadsDigits), tid, i)
	}

	client, err := client.NewClient(log, nil, registryAddress, msgs.AddrsInfo{})
	if err != nil {
		panic(err)
	}
	client.SetCounters(counters)

	log.Info("creating root dirs")
	rootDirs := []msgs.InodeId{}
	for i := 0; i < numRootDirs; i++ {
		resp := &msgs.MakeDirectoryResp{}
		if err := client.CDCRequest(log, &msgs.MakeDirectoryReq{OwnerId: msgs.ROOT_DIR_INODE_ID, Name: fmt.Sprintf("%v", i)}, resp); err != nil {
			panic(err)
		}
		rootDirs = append(rootDirs, resp.Id)
	}

	terminateChan := make(chan any)

	log.Info("creating directories in parallel")
	var wg sync.WaitGroup
	wg.Add(numThreads)
	done := uint64(0)
	inodes := make([]map[string]createInode, numThreads)
	for i := 0; i < numThreads; i++ {
		tid := i
		inodes[tid] = make(map[string]createInode)
		go func() {
			defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
			rand := wyhash.New(uint64(tid))
			for i := 0; i < actionsPerThread; i++ {
				which := rand.Float64()
				// we mostly issue creates since only one dir rename
				// can exist at the same time.
				if len(inodes[tid]) < 2 || which < 0.7 { // create dir
					ownerIx := int(rand.Uint32()) % len(rootDirs)
					owner := rootDirs[ownerIx]
					resp := &msgs.MakeDirectoryResp{}
					if err := client.CDCRequest(log, &msgs.MakeDirectoryReq{OwnerId: owner, Name: entityName(tid, i)}, resp); err != nil {
						panic(err)
					}
					fullName := fmt.Sprintf("%v/%v", ownerIx, entityName(tid, i))
					log.Debug("creating %q", fullName)
					inodes[tid][fullName] = createInode{
						id:           resp.Id,
						creationTime: resp.CreationTime,
					}
				} else if which < 0.85 { // create file
					ownerIx := int(rand.Uint32()) % len(rootDirs)
					owner := rootDirs[ownerIx]
					fileResp := msgs.ConstructFileResp{}
					if err := client.ShardRequest(log, owner.Shard(), &msgs.ConstructFileReq{Type: msgs.FILE}, &fileResp); err != nil {
						panic(err)
					}
					linkResp := msgs.LinkFileResp{}
					if err := client.ShardRequest(log, owner.Shard(), &msgs.LinkFileReq{FileId: fileResp.Id, Cookie: fileResp.Cookie, OwnerId: owner, Name: entityName(tid, i)}, &linkResp); err != nil {
						panic(err)
					}
					fullName := fmt.Sprintf("%v/%v", ownerIx, entityName(tid, i))
					log.Debug("creating file %q", fullName)
					inodes[tid][fullName] = createInode{
						id:           fileResp.Id,
						creationTime: linkResp.CreationTime,
					}
				} else { // rename
					// just pick the first one to rename
					var oldFullName string
					var targetId msgs.InodeId
					var oldCreationTime msgs.TernTime
					for tmpDirName, x := range inodes[tid] {
						oldFullName = tmpDirName
						targetId = x.id
						oldCreationTime = x.creationTime
						break
					}
					// parse the string
					var oldOwner msgs.InodeId
					var oldName string
					{
						parts := strings.Split(oldFullName, "/")
						oldOwnerIx, err := strconv.Atoi(parts[0])
						if err != nil {
							panic(err)
						}
						oldOwner = rootDirs[oldOwnerIx]
						oldName = parts[1]
					}
					// pick a new owner, different than the previous one
					var newOwnerIx int
					var newOwner msgs.InodeId
					for {
						newOwnerIx = int(rand.Uint32()) % len(rootDirs)
						newOwner = rootDirs[newOwnerIx]
						if newOwner != oldOwner {
							break
						}
					}
					// do the rename
					if targetId.Type() == msgs.DIRECTORY {
						req := &msgs.RenameDirectoryReq{
							TargetId:        targetId,
							OldOwnerId:      oldOwner,
							OldName:         oldName,
							OldCreationTime: oldCreationTime,
							NewOwnerId:      newOwner,
							NewName:         entityName(tid, i),
						}
						newFullName := fmt.Sprintf("%v/%v", newOwnerIx, entityName(tid, i))
						log.Debug("renaming dir %q to %q", oldFullName, newFullName)
						resp := &msgs.RenameDirectoryResp{}
						if err := client.CDCRequest(log, req, resp); err != nil {
							panic(err)
						}
						delete(inodes[tid], oldFullName)
						inodes[tid][newFullName] = createInode{
							id:           targetId,
							creationTime: resp.CreationTime,
						}
					} else {
						req := &msgs.RenameFileReq{
							TargetId:        targetId,
							OldOwnerId:      oldOwner,
							OldName:         oldName,
							OldCreationTime: oldCreationTime,
							NewOwnerId:      newOwner,
							NewName:         entityName(tid, i),
						}
						newFullName := fmt.Sprintf("%v/%v", newOwnerIx, entityName(tid, i))
						log.Debug("renaming file %q to %q", oldFullName, newFullName)
						resp := &msgs.RenameFileResp{}
						if err := client.CDCRequest(log, req, resp); err != nil {
							panic(err)
						}
						delete(inodes[tid], oldFullName)
						inodes[tid][newFullName] = createInode{
							id:           targetId,
							creationTime: resp.CreationTime,
						}
					}
				}
				if atomic.AddUint64(&done, 1)%256 == 0 {
					log.Info("went through %v/%v actions", atomic.LoadUint64(&done), numThreads*actionsPerThread)
				}
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		terminateChan <- nil
	}()

	log.Info("waiting for everything to be done")
	{
		err := <-terminateChan
		if err != nil {
			panic(err)
		}
	}

	log.Info("checking")
	expectedDirs := make(map[string]struct{})
	for i := 0; i < numThreads; i++ {
		for d := range inodes[i] {
			expectedDirs[d] = struct{}{}
		}
	}
	actualDirs := make(map[string]struct{})
	for i, id := range rootDirs {
		req := &msgs.ReadDirReq{DirId: id}
		resp := &msgs.ReadDirResp{}
		for {
			if err := client.ShardRequest(log, id.Shard(), req, resp); err != nil {
				panic(err)
			}
			for _, d := range resp.Results {
				actualDirs[fmt.Sprintf("%v/%v", i, d.Name)] = struct{}{}
			}
			req.StartHash = resp.NextHash
			if req.StartHash == 0 {
				break
			}
		}
	}
	someErrors := false
	for d := range expectedDirs {
		if _, found := actualDirs[d]; !found {
			someErrors = true
			log.Info("expected to find dir %q, but did not", d)
		}
	}
	for d := range actualDirs {
		if _, found := expectedDirs[d]; !found {
			someErrors = true
			log.Info("found dir %q, but did not expect it", d)
		}
	}
	if someErrors {
		panic(fmt.Errorf("mismatching directories, check logs"))
	}
	log.Info("finished checking")
}
