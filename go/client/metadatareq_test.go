// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestMetadataRequestProcessorSurvivesErrors(t *testing.T) {
	for _, api := range []string{"shard", "cdc"} {
		for _, failure := range []string{"pack", "send", "unpack", "trailing_bytes"} {
			t.Run(api+"/"+failure, func(t *testing.T) {
				sendErr := &net.OpError{Op: "write", Net: "udp", Err: io.ErrClosedPipe}
				attempts := 0
				c, logger := metadataRetryClient(t, DefaultShardTimeout, metadataTestAddrs, func(packet []byte) ([]byte, error) {
					attempts++
					reply := metadataRetryReply(packet, 0)
					if attempts == 1 {
						switch failure {
						case "send":
							return nil, sendErr
						case "unpack":
							reply = reply[:14]
						case "trailing_bytes":
							reply = append(reply, 0)
						}
					}
					return reply, nil
				})
				name := "file"
				if failure == "pack" {
					name = strings.Repeat("x", 256)
				}
				err := receiveMetadata(t, testMetadataRequest(c, logger, api, name))
				if err == nil {
					t.Fatalf("%s failure was reported as success", failure)
				}
				switch failure {
				case "pack":
					if !strings.Contains(err.Error(), "exceeds maximum") {
						t.Fatalf("unexpected packing error: %v", err)
					}
				case "send":
					if err != sendErr {
						t.Fatalf("send error = %v, want original error %v", err, sendErr)
					}
				default:
					if err != msgs.MALFORMED_RESPONSE {
						t.Fatalf("unpack error = %v, want MALFORMED_RESPONSE", err)
					}
				}
				if err := receiveMetadata(t, testMetadataRequest(c, logger, api, "healthy")); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestMetadataAttemptTimeout(t *testing.T) {
	for _, api := range []string{"shard", "cdc"} {
		t.Run(api, func(t *testing.T) {
			timeouts := timing.NewReqTimeouts(time.Hour, time.Hour, time.Second, 2, 0)
			c := &Client{shardTimeout: timeouts, cdcTimeout: timeouts}
			c.clientMetadata.incoming = make(chan *metadataProcessorRequest, 1)
			logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
			done := testMetadataRequest(c, logger, api, "file")
			req := receiveMetadata(t, c.clientMetadata.incoming)
			if req.timeout <= 0 || req.timeout > timeouts.Overall {
				t.Errorf("queued attempt timeout = %v, want at most %v", req.timeout, timeouts.Overall)
			}
			req.respCh <- &metadataProcessorResponse{err: msgs.NOT_AUTHORISED}
			if err := receiveMetadata(t, done); err != msgs.NOT_AUTHORISED {
				t.Fatalf("error = %v, want NOT_AUTHORISED", err)
			}
		})
	}
}

func TestWriteRegistryRequestReturnsPackErrors(t *testing.T) {
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	err := writeRegistryRequest(logger, io.Discard, &msgs.UpdateBlockServicePathReq{NewPath: strings.Repeat("x", 256)})
	if err == nil {
		t.Fatal("oversized registry request packed successfully")
	}
}
