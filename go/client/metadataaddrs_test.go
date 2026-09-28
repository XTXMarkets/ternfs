// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"encoding/binary"
	"net"
	"os"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/bincode"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestMetadataRequestProcessorHoldsUnaddressedRequests(t *testing.T) {
	cm := clientMetadata{
		client:   &Client{},
		incoming: make(chan *metadataProcessorRequest),
		inFlight: make(chan *metadataProcessorRequest, 1),
	}
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	go cm.processRequests(logger)
	defer close(cm.incoming)

	responses := make(chan *metadataProcessorResponse, 1)
	cm.incoming <- &metadataProcessorRequest{
		requestId: 1,
		timeout:   time.Second,
		shard:     0,
		req:       &msgs.LookupReq{DirId: msgs.ROOT_DIR_INODE_ID, Name: "x"},
		resp:      &msgs.LookupResp{},
		respCh:    responses,
	}
	select {
	case req := <-cm.inFlight:
		if req.requestId != 1 || req.deadline.IsZero() {
			t.Fatalf("in-flight request id %v deadline %v", req.requestId, req.deadline)
		}
	case response := <-responses:
		t.Fatalf("unaddressed request failed immediately: %v", response.err)
	case <-time.After(time.Second):
		t.Fatal("unaddressed request was not held in flight")
	}
}

// A request issued before the registry has supplied any shard addresses must
// wait for them, not fail or come back as an empty success.
func TestShardRequestWaitsForAddresses(t *testing.T) {
	shard, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer shard.Close()
	const targetID = msgs.InodeId(0x2000000000001234)
	go func() {
		buf := make([]byte, msgs.DEFAULT_UDP_MTU)
		for {
			n, from, err := shard.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n < 4+8+1 {
				continue
			}
			body, err := bincode.Pack(&msgs.LookupResp{TargetId: targetID})
			if err != nil {
				panic(err)
			}
			resp := binary.LittleEndian.AppendUint32(nil, msgs.SHARD_RESP_PROTOCOL_VERSION)
			resp = append(resp, buf[4:4+8+1]...) // request id and kind
			resp = append(resp, body...)
			shard.WriteToUDP(resp, from)
		}
	}()

	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	c, err := NewClientDirectNoAddrs(logger, msgs.AddrsInfo{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.clientMetadata.close()

	type result struct {
		resp msgs.LookupResp
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var r result
		r.err = c.ShardRequest(logger, 0, &msgs.LookupReq{DirId: msgs.ROOT_DIR_INODE_ID, Name: ".nfs"}, &r.resp)
		done <- r
	}()

	select {
	case r := <-done:
		t.Fatalf("request finished before any address was known: resp %+v err %v", r.resp, r.err)
	case <-time.After(300 * time.Millisecond):
	}

	var shardAddrs [256]msgs.AddrsInfo
	shardAddrs[0].Addr1 = msgs.IpPort{Addrs: [4]byte{127, 0, 0, 1}, Port: uint16(shard.LocalAddr().(*net.UDPAddr).Port)}
	c.SetAddrs(msgs.AddrsInfo{}, &shardAddrs)

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("request failed after address arrived: %v", r.err)
		}
		if r.resp.TargetId != targetID {
			t.Fatalf("target id = %v, want %v", r.resp.TargetId, targetID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request did not complete after address arrived")
	}
}

// Without any address the request gives up with TIMEOUT once its budget is
// spent, rather than returning the EINVAL from sending to 0.0.0.0:0.
func TestShardRequestTimesOutWithoutAddresses(t *testing.T) {
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	c, err := NewClientDirectNoAddrs(logger, msgs.AddrsInfo{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.clientMetadata.close()
	timeouts := DefaultShardTimeout
	timeouts.Overall = 500 * time.Millisecond
	c.SetShardTimeouts(&timeouts)

	err = c.ShardRequest(logger, 0, &msgs.LookupReq{DirId: msgs.ROOT_DIR_INODE_ID, Name: ".nfs"}, &msgs.LookupResp{})
	if err != msgs.TIMEOUT {
		t.Fatalf("err = %v, want %v", err, msgs.TIMEOUT)
	}
}
