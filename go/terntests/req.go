// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"io"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/log"
	"xtx/ternfs/core/wyhash"
	"xtx/ternfs/msgs"
)

type edge struct {
	name     string
	targetId msgs.InodeId
}

type fullEdge struct {
	current      bool
	name         string
	targetId     msgs.InodeId
	creationTime msgs.TernTime
}

func shardReq(log *log.Logger, client *client.Client, shid msgs.ShardId, reqBody msgs.ShardRequest, respBody msgs.ShardResponse) {
	err := client.ShardRequest(log, shid, reqBody, respBody)
	if err != nil {
		panic(err)
	}
}

func cdcReq(log *log.Logger, client *client.Client, reqBody msgs.CDCRequest, respBody msgs.CDCResponse) {
	err := client.CDCRequest(log, reqBody, respBody)
	if err != nil {
		panic(err)
	}
}

type noopReader struct{}

func (noopReader) Read(bs []byte) (int, error) {
	return len(bs), nil
}

func createFile(
	log *log.Logger,
	client *client.Client,
	dirInfoCache *client.DirInfoCache,
	dirId msgs.InodeId,
	spanSize uint32,
	name string,
	size uint64,
	dataSeed uint64,
	bufPool *bufpool.BufPool,
) (id msgs.InodeId, creationTime msgs.TernTime) {
	// construct
	constructReq := msgs.ConstructFileReq{
		Type: msgs.FILE,
		Note: "",
	}
	constructResp := msgs.ConstructFileResp{}
	shardReq(log, client, dirId.Shard(), &constructReq, &constructResp)
	rand := wyhash.New(dataSeed)
	if size > 0 {
		var spanPolicy msgs.SpanPolicy
		if _, err := client.ResolveDirectoryInfoEntry(log, dirInfoCache, dirId, &spanPolicy); err != nil {
			panic(err)
		}
		var blockPolicies msgs.BlockPolicy
		if _, err := client.ResolveDirectoryInfoEntry(log, dirInfoCache, dirId, &blockPolicies); err != nil {
			panic(err)
		}
		var stripePolicy msgs.StripePolicy
		if _, err := client.ResolveDirectoryInfoEntry(log, dirInfoCache, dirId, &stripePolicy); err != nil {
			panic(err)
		}
		// add spans
		for offset := uint64(0); offset < size; offset += uint64(spanSize) {
			thisSpanSize := spanSize
			if uint32(size-offset) < thisSpanSize {
				thisSpanSize = uint32(size - offset)
			}
			spanBuf := bufPool.Get(int(thisSpanSize))
			rand.Read(spanBuf.Bytes())
			_, err := client.CreateSpan(log, []msgs.BlacklistEntry{}, &spanPolicy, &blockPolicies, &stripePolicy, constructResp.Id, msgs.NULL_INODE_ID, constructResp.Cookie, offset, uint32(thisSpanSize), spanBuf.BytesPtr())
			bufPool.Put(spanBuf)
			if err != nil {
				panic(err)
			}
		}
	}
	// link
	linkReq := msgs.LinkFileReq{
		FileId:  constructResp.Id,
		Cookie:  constructResp.Cookie,
		OwnerId: dirId,
		Name:    name,
	}
	linkResp := msgs.LinkFileResp{}
	shardReq(log, client, dirId.Shard(), &linkReq, &linkResp)
	return constructResp.Id, linkResp.CreationTime
}

func readFile(log *log.Logger, bufPool *bufpool.BufPool, client *client.Client, id msgs.InodeId, size uint64) *bufpool.Buf {
	buf := bufPool.Get(int(size))
	r, err := client.ReadFile(log, bufPool, id)
	if err != nil {
		panic(err)
	}
	defer r.Close()
	cursor := 0
	for {
		read, err := r.Read(buf.Bytes()[cursor:])
		if err == io.EOF {
			if cursor != len(buf.Bytes()) {
				panic(fmt.Errorf("expected file read of size %v for file %v, got %v", len(buf.Bytes()), id, cursor))
			}
			break
		}
		if err != nil {
			panic(err)
		}
		cursor += read
	}
	return buf
}

func readDir(log *log.Logger, client *client.Client, dir msgs.InodeId) []edge {
	req := msgs.ReadDirReq{
		DirId:     dir,
		StartHash: 0,
	}
	resp := msgs.ReadDirResp{}
	edges := []edge{}
	for {
		shardReq(log, client, dir.Shard(), &req, &resp)
		for _, result := range resp.Results {
			edges = append(edges, edge{
				name:     result.Name,
				targetId: result.TargetId,
			})
		}
		req.StartHash = resp.NextHash
		if req.StartHash == 0 {
			break
		}
	}
	return edges
}

func fullReadDir(log *log.Logger, client *client.Client, dirId msgs.InodeId) []fullEdge {
	req := msgs.FullReadDirReq{
		DirId: msgs.ROOT_DIR_INODE_ID,
	}
	resp := msgs.FullReadDirResp{}
	edges := []fullEdge{}
	for {
		shardReq(log, client, dirId.Shard(), &req, &resp)
		for _, result := range resp.Results {
			edges = append(edges, fullEdge{
				name:         result.Name,
				targetId:     result.TargetId.Id(),
				creationTime: result.CreationTime,
				current:      result.Current,
			})
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
	return edges
}
