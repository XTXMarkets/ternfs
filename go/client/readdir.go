// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"fmt"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

// DirEdgesCursor identifies the first unread edge in one directory edge
// section. The zero value identifies the start of the section.
type DirEdgesCursor struct {
	StartName string
	StartTime msgs.TernTime
}

// DirEdgesPage is one MTU-bounded page of directory edges. Next is nil at the
// end of the requested section.
type DirEdgesPage struct {
	Next    *DirEdgesCursor
	Results []msgs.Edge
}

func (c *Client) fullReadDir(
	logger *log.Logger,
	dirId msgs.InodeId,
	flags msgs.FullReadDirFlags,
	startName string,
	startTime msgs.TernTime,
) (*msgs.FullReadDirResp, error) {
	if flags&msgs.FULL_READ_DIR_CURRENT != 0 && startTime != 0 {
		return nil, fmt.Errorf("FULL_READ_DIR_CURRENT requires a zero start time")
	}
	if flags&msgs.FULL_READ_DIR_SAME_NAME != 0 && startName == "" {
		return nil, fmt.Errorf("FULL_READ_DIR_SAME_NAME requires a start name")
	}
	req := &msgs.FullReadDirReq{
		DirId:     dirId,
		Flags:     flags,
		StartName: startName,
		StartTime: startTime,
	}
	resp := &msgs.FullReadDirResp{}
	if err := c.ShardRequest(logger, dirId.Shard(), req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func dirEdgesPage(
	resp *msgs.FullReadDirResp,
	current bool,
	cursor DirEdgesCursor,
) (*DirEdgesPage, error) {
	end := len(resp.Results)
	for i, edge := range resp.Results {
		if edge.Current != current {
			end = i
			break
		}
	}
	page := &DirEdgesPage{Results: resp.Results[:end]}
	if end != len(resp.Results) ||
		resp.Next.StartName == "" ||
		resp.Next.Current != current {
		return page, nil
	}
	next := DirEdgesCursor{
		StartName: resp.Next.StartName,
		StartTime: resp.Next.StartTime,
	}
	if current && next.StartTime != 0 {
		return nil, fmt.Errorf("current directory edge cursor has a snapshot time")
	}
	if next == cursor {
		return nil, fmt.Errorf("FULL_READ_DIR cursor did not advance")
	}
	page.Next = &next
	return page, nil
}

func (c *Client) readDirEdges(
	logger *log.Logger,
	dirId msgs.InodeId,
	current bool,
	cursor DirEdgesCursor,
) (*DirEdgesPage, error) {
	if cursor.StartName == "" && cursor.StartTime != 0 {
		return nil, fmt.Errorf("directory edge cursor has a time without a name")
	}
	if current && cursor.StartTime != 0 {
		return nil, fmt.Errorf("current directory edge cursor has a snapshot time")
	}
	flags := msgs.FullReadDirFlags(0)
	if current {
		flags = msgs.FULL_READ_DIR_CURRENT
	}
	resp, err := c.fullReadDir(
		logger,
		dirId,
		flags,
		cursor.StartName,
		cursor.StartTime,
	)
	if err != nil {
		return nil, err
	}
	return dirEdgesPage(resp, current, cursor)
}

// ReadCurrentDirEdgesPage reads one MTU-bounded page of current directory edges.
func ReadCurrentDirEdgesPage(
	logger *log.Logger,
	client *Client,
	dirId msgs.InodeId,
	cursor DirEdgesCursor,
) (*DirEdgesPage, error) {
	return client.readDirEdges(logger, dirId, true, cursor)
}

// ReadRetainedDirEdgesPage reads one MTU-bounded page of retained directory
// edges. Tombstones and sentinel values are preserved for the caller to
// interpret.
func ReadRetainedDirEdgesPage(
	logger *log.Logger,
	client *Client,
	dirId msgs.InodeId,
	cursor DirEdgesCursor,
) (*DirEdgesPage, error) {
	return client.readDirEdges(logger, dirId, false, cursor)
}

// WalkDirNameHistory walks the current and retained edges for one name,
// newest first. Tombstones and sentinel values are preserved.
func WalkDirNameHistory(
	logger *log.Logger,
	client *Client,
	dirId msgs.InodeId,
	name string,
	visit func([]msgs.Edge) error,
) error {
	baseFlags := msgs.FULL_READ_DIR_BACKWARDS |
		msgs.FULL_READ_DIR_SAME_NAME
	cursor := msgs.FullReadDirCursor{
		Current:   true,
		StartName: name,
	}
	for {
		flags := baseFlags
		if cursor.Current {
			flags |= msgs.FULL_READ_DIR_CURRENT
		}
		resp, err := client.fullReadDir(
			logger,
			dirId,
			flags,
			cursor.StartName,
			cursor.StartTime,
		)
		if err != nil {
			return err
		}
		if len(resp.Results) > 0 {
			if err := visit(resp.Results); err != nil {
				return err
			}
		}
		if resp.Next.StartName == "" {
			return nil
		}
		if resp.Next == cursor {
			return fmt.Errorf(
				"FULL_READ_DIR cursor did not advance for directory %s",
				dirId,
			)
		}
		cursor = resp.Next
	}
}
