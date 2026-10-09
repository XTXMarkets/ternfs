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
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func testMetadataRequest(c *Client, logger *log.Logger, api, name string) error {
	if api == "cdc" {
		return c.CDCRequest(logger, &msgs.MakeDirectoryReq{
			OwnerId: msgs.ROOT_DIR_INODE_ID, Name: name,
		}, &msgs.MakeDirectoryResp{})
	}
	return c.ShardRequest(logger, 0, &msgs.LookupReq{
		DirId: msgs.ROOT_DIR_INODE_ID, Name: name,
	}, &msgs.LookupResp{})
}

func TestMetadataRequestErrors(t *testing.T) {
	sendErr := &net.OpError{Op: "write", Net: "udp", Err: io.ErrClosedPipe}
	for _, api := range []string{"shard", "cdc"} {
		for _, tc := range []struct {
			name    string
			replies []error
			want    error
		}{
			{"success", []error{nil}, nil},
			{"protocol", []error{msgs.NOT_AUTHORISED}, msgs.NOT_AUTHORISED},
			{"send", []error{sendErr}, sendErr},
			{"ordinary", []error{io.ErrUnexpectedEOF}, io.ErrUnexpectedEOF},
			{"malformed", []error{msgs.MALFORMED_RESPONSE}, msgs.MALFORMED_RESPONSE},
			{"timeout_then_success", []error{msgs.TIMEOUT, nil}, nil},
			{"timeout_then_protocol", []error{msgs.TIMEOUT, msgs.NOT_AUTHORISED}, msgs.NOT_AUTHORISED},
			{"timeout_then_send", []error{msgs.TIMEOUT, sendErr}, sendErr},
			{"timeout_then_ordinary", []error{msgs.TIMEOUT, io.ErrUnexpectedEOF}, io.ErrUnexpectedEOF},
			{"timeout_then_malformed", []error{msgs.TIMEOUT, msgs.MALFORMED_RESPONSE}, msgs.MALFORMED_RESPONSE},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				shardTimeout, cdcTimeout := DefaultShardTimeout, DefaultCDCTimeout
				c := &Client{shardTimeout: &shardTimeout, cdcTimeout: &cdcTimeout}
				c.clientMetadata.incoming = make(chan *metadataProcessorRequest)
				logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
				done := make(chan struct{})
				var ids []uint64
				go func() {
					defer close(done)
					for req := range c.clientMetadata.incoming {
						err := error(msgs.INTERNAL_ERROR)
						if len(ids) < len(tc.replies) {
							err = tc.replies[len(ids)]
						}
						ids = append(ids, req.requestId)
						req.respCh <- &metadataProcessorResponse{requestId: req.requestId, err: err}
					}
				}()
				err := testMetadataRequest(c, logger, api, "file")
				close(c.clientMetadata.incoming)
				<-done
				if err != tc.want {
					t.Fatalf("error = %v, want original error %v", err, tc.want)
				}
				if len(ids) != len(tc.replies) {
					t.Fatalf("sent %d requests, want %d", len(ids), len(tc.replies))
				}
				for _, id := range ids {
					if id != ids[0] {
						t.Fatal("retry changed the request ID")
					}
				}
			})
		}
	}
}

func TestMetadataRequestProcessorErrors(t *testing.T) {
	for _, api := range []string{"shard", "cdc"} {
		for _, failure := range []string{"pack", "send", "unpack", "trailing_bytes"} {
			t.Run(api+"/"+failure, func(t *testing.T) {
				shardTimeout, cdcTimeout := DefaultShardTimeout, DefaultCDCTimeout
				c := &Client{shardTimeout: &shardTimeout, cdcTimeout: &cdcTimeout}
				c.clientMetadata = clientMetadata{client: c, incoming: make(chan *metadataProcessorRequest)}
				logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
				name := "file"
				switch failure {
				case "pack":
					name = strings.Repeat("x", 256)
				case "send":
					sock, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
					if err != nil {
						t.Fatal(err)
					}
					sock.Close()
					c.clientMetadata.sock = sock
				}
				done := make(chan struct{})
				go func() {
					defer close(done)
					if failure != "unpack" && failure != "trailing_bytes" {
						c.clientMetadata.processRequests(logger)
						return
					}
					req := <-c.clientMetadata.incoming
					protocol, kind := msgs.SHARD_RESP_PROTOCOL_VERSION, uint8(msgs.LOOKUP)
					if api == "cdc" {
						protocol, kind = msgs.CDC_RESP_PROTOCOL_VERSION, uint8(msgs.MAKE_DIRECTORY)
					}
					// Lookup and MakeDirectory responses both contain two
					// eight-byte fields. Exercise short and overlong bodies.
					bodySize := 1
					if failure == "trailing_bytes" {
						bodySize = 17
					}
					body := make([]byte, 4+8+1+bodySize)
					c.clientMetadata.parseResponse(logger, req, &rawMetadataResponse{
						protocol: protocol, requestId: req.requestId, kind: kind,
						respLen: len(body), buf: &body,
					}, false)
				}()
				err := testMetadataRequest(c, logger, api, name)
				close(c.clientMetadata.incoming)
				<-done
				if err == nil {
					t.Fatalf("%s failure was reported as success", failure)
				}
				switch failure {
				case "pack":
					if !strings.Contains(err.Error(), "exceeds maximum") {
						t.Fatalf("unexpected packing error: %v", err)
					}
				case "send":
					if _, ok := err.(*net.OpError); !ok {
						t.Fatalf("send error has type %T, want *net.OpError", err)
					}
				case "unpack", "trailing_bytes":
					if err != msgs.MALFORMED_RESPONSE {
						t.Fatalf("unpack error = %v, want %v", err, msgs.MALFORMED_RESPONSE)
					}
				}
			})
		}
	}
}

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
