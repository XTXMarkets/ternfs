// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/bincode"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func snapshotRequest(c *Client, logger *log.Logger, shrid msgs.ShardReplicaId) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- c.ShardReplicaRequest(logger, shrid, &msgs.ShardSnapshotReq{SnapshotId: 42}, &msgs.ShardSnapshotResp{})
	}()
	return done
}

func snapshotReply(packet []byte) []byte {
	reply := binary.LittleEndian.AppendUint32(nil, msgs.SHARD_RESP_PROTOCOL_VERSION)
	return append(reply, packet[4:13]...)
}

func TestShardReplicaRequestRouting(t *testing.T) {
	for _, replica := range []msgs.ReplicaId{0, 4} {
		t.Run(msgs.MakeShardReplicaId(0, replica).String(), func(t *testing.T) {
			timeouts := *timing.NewReqTimeouts(10*time.Millisecond, 100*time.Millisecond, time.Second, 2, 0)
			target := msgs.MakeShardReplicaId(0, replica)
			replicaAddrs := msgs.AddrsInfo{
				Addr1: msgs.IpPort{Addrs: [4]byte{127, 0, 0, 2}, Port: 100},
				Addr2: msgs.IpPort{Addrs: [4]byte{127, 0, 0, 3}, Port: 101},
			}
			type sentRequest struct {
				id   uint64
				addr string
				kind msgs.ShardMessageKind
			}
			sends := make(chan sentRequest, 8)
			attempts := 0
			c, logger := metadataRoutingClient(t, timeouts, metadataTestAddrs, func(packet []byte, addr *net.UDPAddr) ([]byte, error) {
				kind := msgs.ShardMessageKind(packet[12])
				sends <- sentRequest{binary.LittleEndian.Uint64(packet[4:]), addr.String(), kind}
				if kind == msgs.SHARD_SNAPSHOT {
					var req msgs.ShardSnapshotReq
					if err := bincode.Unpack(packet[13:], &req); err != nil || req.SnapshotId != 42 {
						t.Errorf("snapshot request = %+v, unpack error = %v", req, err)
					}
					attempts++
					if attempts == 1 {
						return nil, syscall.EHOSTUNREACH
					}
					return snapshotReply(packet), nil
				}
				return metadataRetryReply(packet, 0), nil
			})
			c.SetShardReplicaAddrs(map[msgs.ShardReplicaId]msgs.AddrsInfo{target: replicaAddrs})
			if err := receiveMetadata(t, snapshotRequest(c, logger, target)); err != nil {
				t.Fatal(err)
			}
			a, b := receiveMetadata(t, sends), receiveMetadata(t, sends)
			if a.id != b.id || a.addr == b.addr {
				t.Fatalf("retry did not retain request ID and alternate replica addresses: %+v, %+v", a, b)
			}
			for _, send := range []sentRequest{a, b} {
				if send.addr != "127.0.0.2:100" && send.addr != "127.0.0.3:101" {
					t.Fatalf("snapshot sent outside selected replica: %+v", send)
				}
			}
			if err := receiveMetadata(t, testMetadataRequest(c, logger, "shard", "file")); err != nil {
				t.Fatal(err)
			}
			if send := receiveMetadata(t, sends); send.addr != "127.0.0.1:1" || send.kind != msgs.LOOKUP {
				t.Fatalf("ordinary request did not use leader: %+v", send)
			}
		})
	}
}

func TestShardReplicaRequestRefreshesAddresses(t *testing.T) {
	timeouts := *timing.NewReqTimeouts(30*time.Millisecond, 100*time.Millisecond, time.Second, 2, 0)
	target := msgs.MakeShardReplicaId(7, 2)
	oldAddrs := msgs.AddrsInfo{Addr1: msgs.IpPort{Addrs: [4]byte{127, 0, 0, 1}, Port: 100}}
	// Some replicas only advertise their second interface.
	newAddrs := msgs.AddrsInfo{Addr2: msgs.IpPort{Addrs: [4]byte{127, 0, 0, 2}, Port: 200}}
	sends := make(chan string, 8)
	releaseFirst := make(chan struct{})
	c, logger := metadataRoutingClient(t, timeouts, metadataTestAddrs, func(packet []byte, addr *net.UDPAddr) ([]byte, error) {
		sends <- addr.String()
		if addr.Port == 100 {
			receiveMetadata(t, releaseFirst)
			return nil, nil // Lost response.
		}
		return snapshotReply(packet), nil
	})
	c.SetShardReplicaAddrs(map[msgs.ShardReplicaId]msgs.AddrsInfo{target: oldAddrs})
	done := snapshotRequest(c, logger, target)
	if addr := receiveMetadata(t, sends); addr != "127.0.0.1:100" {
		t.Fatal(addr)
	}
	c.SetShardReplicaAddrs(map[msgs.ShardReplicaId]msgs.AddrsInfo{target: newAddrs})
	close(releaseFirst)
	if err := receiveMetadata(t, done); err != nil {
		t.Fatal(err)
	}
	if addr := receiveMetadata(t, sends); addr != "127.0.0.2:200" {
		t.Fatal(addr)
	}
}

func TestShardReplicaRequestMissingAddress(t *testing.T) {
	timeouts := *timing.NewReqTimeouts(10*time.Millisecond, 30*time.Millisecond, 80*time.Millisecond, 2, 0)
	c, logger := metadataRoutingClient(t, timeouts, metadataTestAddrs, func(_ []byte, addr *net.UDPAddr) ([]byte, error) {
		t.Errorf("missing replica fell back to %v", addr)
		return nil, nil
	})
	target := msgs.MakeShardReplicaId(0, 0)
	if err := receiveMetadata(t, snapshotRequest(c, logger, target)); err != msgs.TIMEOUT {
		t.Fatalf("error = %v, want TIMEOUT", err)
	}
}

func TestShardReplicaRequestWaitsForDiscovery(t *testing.T) {
	c, logger := metadataRoutingClient(t, DefaultShardTimeout, metadataTestAddrs, func(packet []byte, addr *net.UDPAddr) ([]byte, error) {
		return snapshotReply(packet), nil
	})
	target := msgs.MakeShardReplicaId(1, 2)
	done := snapshotRequest(c, logger, target)
	select {
	case err := <-done:
		t.Fatalf("request completed before discovery: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	c.SetShardReplicaAddrs(map[msgs.ShardReplicaId]msgs.AddrsInfo{target: metadataTestAddrs})
	if err := receiveMetadata(t, done); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshShardReplicas(t *testing.T) {
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	c := &Client{
		registryConn:      &RegistryConn{log: logger, reqChan: make(chan shuckReq)},
		blockServicesLock: &sync.RWMutex{},
	}
	leaders := make([]msgs.ShardInfo, 256)
	replicas := make([]msgs.FullShardInfo, 0, 257)
	for shard := range leaders {
		leaders[shard].Addrs = metadataTestAddrs
		replicas = append(replicas, msgs.FullShardInfo{
			Id: msgs.MakeShardReplicaId(msgs.ShardId(shard), 1), IsLeader: true, Addrs: metadataTestAddrs,
		})
	}
	target := msgs.MakeShardReplicaId(0, 0)
	followerAddrs := msgs.AddrsInfo{Addr1: msgs.IpPort{Addrs: [4]byte{127, 0, 0, 2}, Port: 2}}
	replicas = append(replicas, msgs.FullShardInfo{Id: target, Addrs: followerAddrs})

	refresh := func(reply shuckResp, legacy bool) {
		t.Helper()
		done := make(chan error, 1)
		go func() { done <- c.refreshAddrs() }()
		req := receiveMetadata(t, c.registryConn.reqChan)
		if _, ok := req.req.(*msgs.LocalShardReplicasReq); !ok {
			t.Fatalf("discovery request = %T", req.req)
		}
		req.respC <- reply
		if legacy {
			req = receiveMetadata(t, c.registryConn.reqChan)
			if _, ok := req.req.(*msgs.LocalShardsReq); !ok {
				t.Fatalf("legacy request = %T", req.req)
			}
			req.respC <- shuckResp{resp: &msgs.LocalShardsResp{Shards: leaders}}
		}
		req = receiveMetadata(t, c.registryConn.reqChan)
		if _, ok := req.req.(*msgs.LocalCdcReq); !ok {
			t.Fatalf("CDC request = %T", req.req)
		}
		req.respC <- shuckResp{resp: &msgs.LocalCdcResp{Addrs: metadataTestAddrs}}
		if err := receiveMetadata(t, done); err != nil {
			t.Fatal(err)
		}
	}
	refresh(shuckResp{resp: &msgs.LocalShardReplicasResp{Shards: replicas}}, false)
	if !c.clientReady.Load() || c.shardAddrs(0)[0].Port != 1 || c.shardReplicaAddrs(target)[0].Port != 2 {
		t.Fatal("discovery did not populate leader and follower addresses")
	}
	refresh(shuckResp{resp: &msgs.LocalShardReplicasResp{Shards: replicas[:256]}}, false)
	if c.shardReplicaAddrs(target)[0].Port != 0 {
		t.Fatal("removed replica retained stale addresses")
	}
	refresh(shuckResp{err: io.EOF}, true)
	if !c.clientReady.Load() || c.shardAddrs(0)[0].Port != 1 {
		t.Fatal("legacy discovery failed")
	}
}

func TestShardReplicaSnapshotError(t *testing.T) {
	c, logger := metadataRoutingClient(t, DefaultShardTimeout, metadataTestAddrs, func(packet []byte, _ *net.UDPAddr) ([]byte, error) {
		return metadataRetryReply(packet, msgs.NOT_AUTHORISED), nil
	})
	target := msgs.MakeShardReplicaId(0, 1)
	c.SetShardReplicaAddrs(map[msgs.ShardReplicaId]msgs.AddrsInfo{target: metadataTestAddrs})
	if err := receiveMetadata(t, snapshotRequest(c, logger, target)); !errors.Is(err, msgs.NOT_AUTHORISED) {
		t.Fatalf("snapshot error = %v", err)
	}
}
