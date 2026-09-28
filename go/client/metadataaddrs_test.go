// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestMetadataRequestWaitsForAddresses(t *testing.T) {
	for _, api := range []string{"shard", "cdc"} {
		t.Run(api, func(t *testing.T) {
			c, logger := metadataRetryClient(t, DefaultShardTimeout, msgs.AddrsInfo{}, func(packet []byte) ([]byte, error) {
				return metadataRetryReply(packet, 0), nil
			})
			done := testMetadataRequest(c, logger, api, "file")
			select {
			case err := <-done:
				t.Fatalf("request finished before an address was known: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			c.SetAddrs(metadataTestAddrs, &[256]msgs.AddrsInfo{metadataTestAddrs})
			if err := receiveMetadata(t, done); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMetadataRequestTimesOutWithoutAddresses(t *testing.T) {
	for _, api := range []string{"shard", "cdc"} {
		t.Run(api, func(t *testing.T) {
			timeouts := *timing.NewReqTimeouts(10*time.Millisecond, 100*time.Millisecond, 40*time.Millisecond, 2, 0)
			c, logger := metadataRetryClient(t, timeouts, msgs.AddrsInfo{}, func([]byte) ([]byte, error) {
				t.Error("attempted a send without an address")
				return nil, net.ErrClosed
			})
			if err := receiveMetadata(t, testMetadataRequest(c, logger, api, "file")); err != msgs.TIMEOUT {
				t.Fatalf("error = %v, want TIMEOUT", err)
			}
		})
	}
}

func TestMetadataDontWaitWithoutAddresses(t *testing.T) {
	kinds := make(chan uint8, 4)
	c, logger := metadataRetryClient(t, DefaultShardTimeout, msgs.AddrsInfo{}, func(packet []byte) ([]byte, error) {
		kinds <- packet[12]
		return metadataRetryReply(packet, 0), nil
	})
	if err := c.ShardRequestDontWait(logger, 0, &msgs.SetTimeReq{Id: msgs.ROOT_DIR_INODE_ID}); err != nil {
		t.Fatal(err)
	}
	// A packing error behind SET_TIME confirms the sender processed it before discovery.
	if err := receiveMetadata(t, testMetadataRequest(c, logger, "shard", strings.Repeat("x", 256))); err == nil {
		t.Fatal("oversized request unexpectedly succeeded")
	}
	c.SetAddrs(metadataTestAddrs, &[256]msgs.AddrsInfo{metadataTestAddrs})
	if err := receiveMetadata(t, testMetadataRequest(c, logger, "shard", "healthy")); err != nil {
		t.Fatal(err)
	}
	if kind := receiveMetadata(t, kinds); kind != uint8(msgs.LOOKUP) {
		t.Fatalf("sent kind %v, want only LOOKUP", kind)
	}
	if len(kinds) != 0 {
		t.Fatal("sent an extra request")
	}
}
