// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bincode"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestShardReqSnapshotReplica(t *testing.T) {
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	sock, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer sock.Close()
	sock.SetReadDeadline(time.Now().Add(5 * time.Second))
	done := make(chan error, 1)
	go func() {
		packet := make([]byte, 1024)
		n, addr, err := sock.ReadFromUDP(packet)
		if err != nil {
			done <- err
			return
		}
		packet = packet[:n]
		if n < 13 || binary.LittleEndian.Uint32(packet) != msgs.SHARD_REQ_PROTOCOL_VERSION || packet[12] != byte(msgs.SHARD_SNAPSHOT) {
			done <- fmt.Errorf("unexpected shard packet: %x", packet)
			return
		}
		var req msgs.ShardSnapshotReq
		if err := bincode.Unpack(packet[13:], &req); err != nil || req.SnapshotId != 42 {
			done <- fmt.Errorf("snapshot request = %+v, error = %v", req, err)
			return
		}
		reply := binary.LittleEndian.AppendUint32(nil, msgs.SHARD_RESP_PROTOCOL_VERSION)
		reply = append(reply, packet[4:13]...)
		_, err = sock.WriteToUDP(reply, addr)
		done <- err
	}()
	c, err := client.NewClientDirectNoAddrs(logger, msgs.AddrsInfo{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	timeouts := client.DefaultShardTimeout
	timeouts.Overall = time.Second
	c.SetShardTimeouts(&timeouts)
	target := msgs.MakeShardReplicaId(7, 0)
	c.SetShardReplicaAddrs(map[msgs.ShardReplicaId]msgs.AddrsInfo{
		target: {Addr1: msgs.IpPort{Addrs: [4]byte{127, 0, 0, 1}, Port: uint16(sock.LocalAddr().(*net.UDPAddr).Port)}},
	})
	command := NewShardReq()
	if err := command.Flags.Parse([]string{"-shard", "7", "-replica", "0", "-kind", "SHARD_SNAPSHOT", "-req", `{"SnapshotId":42}`, "-yes"}); err != nil {
		t.Fatal(err)
	}
	command.Run(NewRuntime(logger, "", func() *client.Client { return c }))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
