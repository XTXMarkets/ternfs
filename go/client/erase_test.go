// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestStartEraseBlockRetries(t *testing.T) {
	logFile, err := os.CreateTemp(t.TempDir(), "client-log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	logger := log.NewLogger(logFile, &log.LoggerOptions{})

	blockTimeout := timing.NewReqTimeouts(time.Millisecond, time.Millisecond, time.Second, 2, 0)
	c := &Client{blockTimeout: blockTimeout}
	c.eraseBlockProcessors.init("erase", &c.blockTimeout, msgs.AddrsInfo{})
	c.eraseBlockProcessors.blockServiceBits = 5

	block := &msgs.RemoveSpanInitiateBlockInfo{
		BlockServiceAddrs: msgs.AddrsInfo{
			Addr1: msgs.IpPort{Port: 1},
		},
		BlockServiceId: 42,
		BlockId:        123,
	}
	processor := &blocksProcessor{
		reqChan: make(chan *clientBlockRequest, 1),
	}
	c.eraseBlockProcessors.processors.Store(
		blocksProcessorKey{
			blockServiceKey: uint64(block.BlockServiceId) & 31,
			addrs:           block.BlockServiceAddrs,
		},
		processor,
	)

	completionChan := make(chan *BlockCompletion, 1)
	if err := c.StartEraseBlock(logger, block, "extra", completionChan); err != nil {
		t.Fatal(err)
	}

	firstReq := receiveBlockRequest(t, processor.reqChan)
	firstReq.resp.completionChan <- &BlockCompletion{
		Resp:  firstReq.resp.resp,
		Extra: firstReq.resp.extra,
		Error: syscall.ECONNRESET,
	}

	secondReq := receiveBlockRequest(t, processor.reqChan)
	expectedProof := [8]byte{1, 2, 3}
	secondReq.resp.resp.(*msgs.EraseBlockResp).Proof = expectedProof
	secondReq.resp.completionChan <- &BlockCompletion{
		Resp:  secondReq.resp.resp,
		Extra: secondReq.resp.extra,
	}

	select {
	case completion := <-completionChan:
		if completion.Error != nil {
			t.Fatalf("unexpected completion error: %v", completion.Error)
		}
		if completion.Extra != "extra" {
			t.Fatalf("unexpected completion extra: %v", completion.Extra)
		}
		if proof := completion.Resp.(*msgs.EraseBlockResp).Proof; proof != expectedProof {
			t.Fatalf("unexpected proof: %v", proof)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for erase completion")
	}

	select {
	case req := <-processor.reqChan:
		t.Fatalf("unexpected additional erase request: %+v", req)
	case <-time.After(10 * time.Millisecond):
	}
}

func receiveBlockRequest(t *testing.T, requests <-chan *clientBlockRequest) *clientBlockRequest {
	t.Helper()
	select {
	case req := <-requests:
		return req
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for block request")
		return nil
	}
}
