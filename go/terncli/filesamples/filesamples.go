// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package filesamples

import (
	"encoding/csv"
	"fmt"
	"io"
	"path"
	"strconv"
	"sync"
	"xtx/ternfs/client"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

type PathResolver interface {
	// Given an inode ID and the name of a file, returns the full path to the file.
	Resolve(inode msgs.InodeId, filename string) (string, error)

	// Given an input of `sample-files` output, resolves the full path for every sample,
	// and writes enriched samples to output.
	ResolveFilePaths(input io.Reader, output io.Writer)
}

// Returns a thread-safe PathResolver that can be used concurrently in multiple goroutines.
func NewPathResolver(cl *client.Client, logger *log.Logger) PathResolver {
	return &resolver{
		ternClient: cl,
		logger:     logger,
		inodeToDir: make(map[msgs.InodeId]string),
		lock:       sync.RWMutex{},
	}
}

type resolver struct {
	ternClient *client.Client
	logger     *log.Logger
	// Mapping of inode ID to directory name. Used to avoid duplicate lookups for the same inode.
	inodeToDir map[msgs.InodeId]string
	// Used to handle concurrent access to resolver internal data.
	lock sync.RWMutex
}

func (r *resolver) Resolve(ownerInode msgs.InodeId, filename string) (string, error) {
	filepath := filename
	currentDir := ownerInode
	for {
		// Get the next owner inode.
		statReq := msgs.StatDirectoryReq{
			Id: currentDir,
		}
		statResp := msgs.StatDirectoryResp{}
		if err := r.ternClient.ShardRequest(r.logger, currentDir.Shard(), &statReq, &statResp); err != nil {
			return "", fmt.Errorf("StatDirectoryReq to shard %v for inode %v failed: %w", currentDir.Shard(), currentDir, err)
		}
		owner := statResp.Owner
		// If we're at the top level, then we're done.
		if currentDir == msgs.ROOT_DIR_INODE_ID {
			return path.Join("/", filepath), nil
		}
		// If we've found a null inode ID, then we won't be able to chase things any further.
		if owner == msgs.NULL_INODE_ID {
			return filepath, nil
		}
		// Get the name of the next node locally if possible.
		if dirName, exists := r.getDirName(currentDir); exists {
			filepath = path.Join(dirName, filepath)
			currentDir = owner
			continue
		}
		dirName, err := r.getNameFromShard(owner, currentDir)
		if err != nil {
			return "", fmt.Errorf("failed to lookup directory name from shard: %w", err)
		}
		r.setDirName(currentDir, dirName)
		filepath = path.Join(dirName, filepath)
		currentDir = owner
	}
}

func (r *resolver) ResolveFilePaths(input io.Reader, output io.Writer) {
	reader := csv.NewReader(input)
	writer := csv.NewWriter(output)
	// Start the workers going.
	numWorkers := 100
	workQueue := make(chan []string, numWorkers)
	outputQueue := make(chan []string, 1000)
	wg := sync.WaitGroup{}
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				sample, ok := <-workQueue
				if !ok {
					return
				}
				filename := sample[0]
				owner, err := strconv.ParseUint(sample[1], 0, 63)
				if err != nil {
					panic(fmt.Errorf("Couldn't parse owner inode (%v): %v", sample[1], err))
				}
				path, err := r.Resolve(msgs.InodeId(owner), filename)
				if err != nil {
					r.logger.ErrorNoAlert("Failed to resolve file path: %v", err)
				} else {
					sample[0] = path // overwrite filename with resolved path
				}
				outputQueue <- sample
			}
		}()
	}
	go func() {
		for {
			sample, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(fmt.Errorf("error reading csv: %v", err))
			}
			workQueue <- sample
		}
		r.logger.Info("Finished reading input")
		close(workQueue)
		wg.Wait()
		close(outputQueue)
	}()
	processed := 0
	for sample := range outputQueue {
		err := writer.Write(sample)
		if err != nil {
			panic(fmt.Errorf("error writing csv: %v", err))
		}
		processed += 1
		if processed%10000 == 0 {
			r.logger.Info("%d samples processed", processed)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		panic(fmt.Errorf("Couldn't flush the results: %v", err))
	}
}

// Queries the shards to list the contents of the parent directory and get the name of the target inode.
func (r *resolver) getNameFromShard(parentDir msgs.InodeId, target msgs.InodeId) (string, error) {
	readDirReq := msgs.FullReadDirReq{
		DirId: parentDir,
	}
	for {
		readDirResp := msgs.FullReadDirResp{}
		if err := r.ternClient.ShardRequest(r.logger, parentDir.Shard(), &readDirReq, &readDirResp); err != nil {
			return "", fmt.Errorf("FullReadDirReq to shard failed: %w", err)
		}
		for _, result := range readDirResp.Results {
			if result.TargetId.Id() != target {
				continue
			}
			return result.Name, nil
		}
		// If the cursor doesn't have a start name, then we've seen everything.
		if readDirResp.Next.StartName == "" {
			break
		}

		readDirReq.StartTime = readDirResp.Next.StartTime
		readDirReq.StartName = readDirResp.Next.StartName
		if readDirResp.Next.Current {
			readDirReq.Flags = msgs.FULL_READ_DIR_CURRENT
		}
	}

	return "", fmt.Errorf("failed to find directory name")

}

func (r *resolver) getDirName(inode msgs.InodeId) (string, bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	val, exists := r.inodeToDir[inode]
	return val, exists
}

func (r *resolver) setDirName(inode msgs.InodeId, name string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.inodeToDir[inode] = name
}
