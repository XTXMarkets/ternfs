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

func (c *Client) checkRepeatedCDCRequestError(
	logger *log.Logger,
	// these are already filled in by now
	reqBody msgs.CDCRequest,
	resp msgs.CDCResponse,
	respErr msgs.TernError,
) msgs.TernError {
	switch reqBody := reqBody.(type) {
	case *msgs.RenameDirectoryReq:
		// We repeat the request, but the previous had actually gone through:
		// we need to check if we haven't created the thing already.
		if respErr == msgs.EDGE_NOT_FOUND {
			// Happens when a request succeeds, and then the response gets lost.
			// We try to apply some heuristics to let this slide. See convo following
			// <https://eulergamma.slack.com/archives/C03PCJMGAAC/p1673547512380409>.
			//
			// Specifically, check that the last snapshot edge is what we expect if
			// we had  just moved it, and that the target edge also exists.
			logger.Info("following up on EDGE_NOT_FOUND after repeated RenameDirectoryReq %+v", reqBody)
			if !c.checkDeletedEdge(logger, reqBody.OldOwnerId, reqBody.TargetId, reqBody.OldName, reqBody.OldCreationTime, false) {
				return respErr
			}
			// Then we check the target edge, and update creation time
			respBody := resp.(*msgs.RenameDirectoryResp)
			if !c.checkNewEdgeAfterRename(logger, reqBody.NewOwnerId, reqBody.TargetId, reqBody.NewName, &respBody.CreationTime) {
				return respErr
			}
			logger.Info("recovered from EDGE_NOT_FOUND, will fill in creation time")
			return 0
		}
	// in a decent language this branch and the previous could be merged
	case *msgs.RenameFileReq:
		if respErr == msgs.EDGE_NOT_FOUND {
			logger.Info("following up on EDGE_NOT_FOUND after repeated RenameFileReq %+v", reqBody)
			if !c.checkDeletedEdge(logger, reqBody.OldOwnerId, reqBody.TargetId, reqBody.OldName, reqBody.OldCreationTime, false) {
				return respErr
			}
			// Then we check the target edge, and update creation time
			respBody := resp.(*msgs.RenameFileResp)
			if !c.checkNewEdgeAfterRename(logger, reqBody.NewOwnerId, reqBody.TargetId, reqBody.NewName, &respBody.CreationTime) {
				return respErr
			}
			logger.Info("recovered from EDGE_NOT_FOUND, will fill in creation time")
			return 0
		}
	case *msgs.SoftUnlinkDirectoryReq:
		if respErr == msgs.EDGE_NOT_FOUND {
			logger.Info("following up on EDGE_NOT_FOUND after repeated SoftUnlinkDirectoryReq %+v", reqBody)
			// Note that here we expect a non-owned edge, since we're deleting a directory.
			if !c.checkDeletedEdge(logger, reqBody.OwnerId, reqBody.TargetId, reqBody.Name, reqBody.CreationTime, false) {
				return respErr
			}
			return 0
		}
	}
	return respErr
}

func (c *Client) CDCRequest(
	logger *log.Logger,
	reqBody msgs.CDCRequest,
	// Result will be written in here. If an error is returned, no guarantees
	// are made regarding the contents of `respBody`.
	respBody msgs.CDCResponse,
) error {
	msgKind := reqBody.CDCRequestKind()
	if msgKind != respBody.CDCResponseKind() {
		panic(fmt.Errorf("mismatching req %T and resp %T", reqBody, respBody))
	}
	var counters *ReqCounters
	if c.counters != nil {
		counters = c.counters.CDC[uint8(msgKind)]
	}
	return c.metadataRequest(logger, -1, reqBody, respBody, counters, false)
}

func createCDCSocket(ip [4]byte, port uint16) (*net.UDPConn, error) {
	socket, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: ip[:], Port: int(port)})
	if err != nil {
		return nil, fmt.Errorf("could not create CDC socket: %w", err)
	}
	return socket, nil
}
