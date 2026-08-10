// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

const (
	maxSymlinkResolutionHops = 40
)

type symlinkJob struct {
	id       msgs.InodeId
	fullPath string
}

type symlinkFilesystem interface {
	readlink(msgs.InodeId) (string, error)
	lookup(msgs.InodeId, string) (msgs.InodeId, error)
}

type ternSymlinkFilesystem struct {
	client  *client.Client
	log     *log.Logger
	bufPool *bufpool.BufPool
}

func (fs *ternSymlinkFilesystem) readlink(id msgs.InodeId) (string, error) {
	r, err := fs.client.ReadFile(fs.log, fs.bufPool, id)
	if err != nil {
		return "", err
	}
	defer r.Close()
	target, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(target), nil
}

func (fs *ternSymlinkFilesystem) lookup(dir msgs.InodeId, name string) (msgs.InodeId, error) {
	resp := msgs.LookupResp{}
	if err := fs.client.ShardRequest(fs.log, dir.Shard(), &msgs.LookupReq{
		DirId: dir,
		Name:  name,
	}, &resp); err != nil {
		return msgs.NULL_INODE_ID, err
	}
	return resp.TargetId, nil
}

func symlinkIsDangling(fs symlinkFilesystem, linkPath string, linkId msgs.InodeId) (bool, error) {
	target, err := fs.readlink(linkId)
	if err != nil {
		return false, fmt.Errorf("readlink: %w", err)
	}
	if target == "" {
		return true, nil
	}
	if !path.IsAbs(target) {
		target = path.Dir(linkPath) + "/" + target
	}
	return pathIsMissing(fs, target)
}

func FirstSymlinkMatch(
	rules []*Rule,
	path string,
	size uint64,
	atime, mtime, now msgs.TernTime,
	targetIsDangling func() (bool, error),
) (*Rule, error) {
	var (
		resolved bool
		dangling bool
	)
	for _, rule := range rules {
		if !rule.Matches(path, size, atime, mtime, now) {
			continue
		}
		if rule.SymlinkTarget == "" {
			return rule, nil
		}
		if !resolved {
			var err error
			dangling, err = targetIsDangling()
			if err != nil {
				return nil, err
			}
			resolved = true
		}
		if (rule.SymlinkTarget == "dangling") == dangling {
			return rule, nil
		}
	}
	return nil, nil
}

func pathIsMissing(fs symlinkFilesystem, target string) (bool, error) {
	originalTarget := target
	pending := strings.Split(target, "/")
	dirStack := []msgs.InodeId{msgs.ROOT_DIR_INODE_ID}
	hops := 0

	for len(pending) > 0 {
		segment := pending[0]
		pending = pending[1:]
		switch segment {
		case "", ".":
			continue
		case "..":
			if len(dirStack) > 1 {
				dirStack = dirStack[:len(dirStack)-1]
			}
			continue
		}

		currentDir := dirStack[len(dirStack)-1]
		id, err := fs.lookup(currentDir, segment)
		if err != nil {
			if isMissingPathError(err) {
				return true, nil
			}
			return false, err
		}

		if id.Type() == msgs.SYMLINK {
			hops++
			if hops > maxSymlinkResolutionHops {
				return false, fmt.Errorf("more than %d symlink hops resolving %q", maxSymlinkResolutionHops, originalTarget)
			}
			linkTarget, err := fs.readlink(id)
			if err != nil {
				if isMissingPathError(err) {
					return true, nil
				}
				return false, err
			}
			if linkTarget == "" {
				return true, nil
			}
			if path.IsAbs(linkTarget) {
				dirStack = dirStack[:1]
			}
			pending = append(strings.Split(linkTarget, "/"), pending...)
			continue
		}

		if len(pending) == 0 {
			return false, nil
		}
		if id.Type() != msgs.DIRECTORY {
			return true, nil
		}
		dirStack = append(dirStack, id)
	}
	return false, nil
}

func isMissingPathError(err error) bool {
	return errors.Is(err, msgs.FILE_NOT_FOUND) ||
		errors.Is(err, msgs.DIRECTORY_NOT_FOUND) ||
		errors.Is(err, msgs.NAME_NOT_FOUND) ||
		errors.Is(err, msgs.EDGE_NOT_FOUND)
}

func symlinkRow(job symlinkJob, resp msgs.StatFileResp, rule *Rule) string {
	return fmt.Sprintf(
		"%s\t%d\t%d\t%d\t%s\t%s",
		job.id.String(),
		resp.Size,
		uint64(resp.Atime),
		uint64(resp.Mtime),
		rule.Name,
		job.fullPath,
	)
}
