// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"fmt"
	"net"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

func (c *Client) checkDeletedEdge(
	logger *log.Logger,
	dirId msgs.InodeId,
	targetId msgs.InodeId,
	name string,
	creationTime msgs.TernTime,
	owned bool,
) bool {
	// First we check the edge we expect to have moved away
	req := msgs.FullReadDirReq{
		DirId:     dirId,
		Flags:     msgs.FULL_READ_DIR_BACKWARDS | msgs.FULL_READ_DIR_CURRENT | msgs.FULL_READ_DIR_SAME_NAME,
		StartName: name,
		Limit:     2,
	}
	resp := msgs.FullReadDirResp{}
	err := c.ShardRequest(logger, dirId.Shard(), &req, &resp)
	if err != nil {
		logger.Info("failed to get snapshot edge (err %v), giving up and returning original error", err)
		return false
	}
	if len(resp.Results) != 2 {
		logger.Info("expected 2 snapshot edges but got %v, giving up and returning original error", len(resp.Results))
		return false
	}
	oldEdge := resp.Results[1]
	if oldEdge.Current || oldEdge.TargetId.Extra() != owned || oldEdge.TargetId.Id() != targetId || oldEdge.CreationTime != creationTime {
		logger.Info("got mismatched snapshot edge (%+v), giving up and returning original error", oldEdge)
		return false
	}
	deleteEdge := resp.Results[0]
	if deleteEdge.TargetId.Id() != msgs.NULL_INODE_ID {
		logger.Info("expected deletion edge but got %+v, giving up and returning original error", deleteEdge)
		return false
	}
	return true
}

func (c *Client) checkNewEdgeAfterRename(
	logger *log.Logger,
	dirId msgs.InodeId,
	targetId msgs.InodeId,
	name string,
	creationTime *msgs.TernTime,
) bool {
	// Then we check the target edge
	lookupResp := msgs.LookupResp{}
	err := c.ShardRequest(logger, dirId.Shard(), &msgs.LookupReq{DirId: dirId, Name: name}, &lookupResp)
	if err != nil {
		logger.Info("failed to get current edge (err %v), giving up and returning original error", err)
		return false
	}
	if lookupResp.TargetId != targetId {
		logger.Info("got mismatched current target (%v), giving up and returning original error", lookupResp.TargetId)
		return false
	}
	*creationTime = lookupResp.CreationTime
	return true
}

func (c *Client) checkRepeatedShardRequestError(
	logger *log.Logger,
	// these are already filled in by now
	reqBody msgs.ShardRequest,
	resp msgs.ShardResponse,
	respErr msgs.TernError,
) msgs.TernError {
	switch reqBody := reqBody.(type) {
	case *msgs.SameDirectoryRenameReq:
		if respErr == msgs.EDGE_NOT_FOUND {
			// Happens when a request succeeds, and then the response gets lost.
			// We try to apply some heuristics to let this slide. See convo following
			// <https://eulergamma.slack.com/archives/C03PCJMGAAC/p1673547512380409>.
			//
			// Specifically, check that the last snapshot edge is what we expect if
			// we had  just moved it, and that the target edge also exists.
			logger.Info("following up on EDGE_NOT_FOUND after repeated SameDirectoryRenameReq %+v", reqBody)
			if !c.checkDeletedEdge(logger, reqBody.DirId, reqBody.TargetId, reqBody.OldName, reqBody.OldCreationTime, false) {
				return respErr
			}
			// Then we check the target edge, and update creation time
			respBody := resp.(*msgs.SameDirectoryRenameResp)
			if !c.checkNewEdgeAfterRename(logger, reqBody.DirId, reqBody.TargetId, reqBody.NewName, &respBody.NewCreationTime) {
				return respErr
			}
			logger.Info("recovered from EDGE_NOT_FOUND, will fill in creation time")
			return 0
		}
	case *msgs.SoftUnlinkFileReq:
		if respErr == msgs.EDGE_NOT_FOUND {
			logger.Info("following up on EDGE_NOT_FOUND after repeated SoftUnlinkFileReq %+v", reqBody)
			if !c.checkDeletedEdge(logger, reqBody.OwnerId, reqBody.FileId, reqBody.Name, reqBody.CreationTime, true) {
				return respErr
			}
			return 0
		}
	}
	return respErr
}

func (c *Client) shardRequestInternal(
	logger *log.Logger,
	shid msgs.ShardId,
	reqBody msgs.ShardRequest,
	// Result will be written in here. If an error is returned, no guarantees
	// are made regarding the contents of `respBody`.
	respBody msgs.ShardResponse,
	dontWait bool,
) error {
	msgKind := reqBody.ShardRequestKind()
	if !dontWait && msgKind != respBody.ShardResponseKind() {
		panic(fmt.Errorf("mismatching req %T and resp %T", reqBody, respBody))
	}
	mtu := clientMtu
	switch r := reqBody.(type) {
	case *msgs.ReadDirReq:
		r.Mtu = mtu
	case *msgs.LocalFileSpansReq:
		r.Mtu = mtu
	case *msgs.FileSpansReq:
		r.Mtu = mtu
	case *msgs.VisitDirectoriesReq:
		r.Mtu = mtu
	case *msgs.VisitFilesReq:
		r.Mtu = mtu
	case *msgs.VisitTransientFilesReq:
		r.Mtu = mtu
	case *msgs.FullReadDirReq:
		r.Mtu = mtu
	}
	var counters *ReqCounters
	if c.counters != nil {
		counters = &c.counters.Shard[uint8(msgKind)][uint8(shid)]
	}
	return c.metadataRequest(logger, int16(shid), reqBody, respBody, counters, dontWait)
}

func (c *Client) ShardRequestDontWait(
	logger *log.Logger,
	shid msgs.ShardId,
	reqBody msgs.ShardRequest,
) error {
	return c.shardRequestInternal(logger, shid, reqBody, nil, true)
}

// This function will set the mtu field for requests that have it with whatever is in `SetMTU`
func (c *Client) ShardRequest(
	logger *log.Logger,
	shid msgs.ShardId,
	reqBody msgs.ShardRequest,
	// Result will be written in here. If an error is returned, no guarantees
	// are made regarding the contents of `respBody`.
	respBody msgs.ShardResponse,
) error {
	return c.shardRequestInternal(logger, shid, reqBody, respBody, false)
}

func createShardSocket(shid msgs.ShardId, ip [4]byte, port uint16) (*net.UDPConn, error) {
	socket, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: ip[:], Port: int(port)})
	if err != nil {
		return nil, fmt.Errorf("could not create shard %v socket at %v:%v: %w", shid, ip, port, err)
	}
	return socket, nil
}
