// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestWriteRegistryRequestReturnsPackErrors(t *testing.T) {
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	err := writeRegistryRequest(logger, io.Discard, &msgs.UpdateBlockServicePathReq{
		NewPath: strings.Repeat("x", 256),
	})
	if err == nil {
		t.Fatal("oversized registry request packed successfully")
	}
}

func TestMetadataRequestProcessorSurvivesPackErrors(t *testing.T) {
	cm := clientMetadata{
		client:   &Client{},
		incoming: make(chan *metadataProcessorRequest),
		inFlight: make(chan *metadataProcessorRequest, 1),
	}
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	done := make(chan struct{})
	go func() {
		cm.processRequests(logger)
		close(done)
	}()

	responses := make(chan *metadataProcessorResponse, 2)
	for requestID := uint64(1); requestID <= 2; requestID++ {
		cm.incoming <- &metadataProcessorRequest{
			requestId: requestID,
			timeout:   time.Second,
			shard:     0,
			req: &msgs.LookupReq{
				DirId: msgs.ROOT_DIR_INODE_ID,
				Name:  strings.Repeat("x", 256),
			},
			resp:   &msgs.LookupResp{},
			respCh: responses,
		}
	}
	close(cm.incoming)

	for requestID := uint64(1); requestID <= 2; requestID++ {
		response := <-responses
		if response.requestId != requestID {
			t.Fatalf("request id = %d, want %d", response.requestId, requestID)
		}
		if response.err == nil {
			t.Fatalf("request %d unexpectedly packed", requestID)
		}
	}
	<-done
}
